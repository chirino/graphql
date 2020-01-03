package graphql_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chirino/graphql"
	"github.com/chirino/graphql/resolvers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
	"time"
)

const schemaText = `
schema {
	query: Query
}
type Query {
	person: Person
	dog: Dog
	animal: Animal
	alien: Alien
}
type Animal {
	relativeAge: Int
}
type Person {
	name: String
	spouse: Person
	pets: [Dog]
	age: Int
	relativeAge: Int
}
type Dog {
	name: String
	age: Int
	dogYears: Int
	relativeAge: Int
}
type Alien {
	composition: String
	shape: String
	pet: Dog
}

`

type Animal interface {
	RelativeAge() int
}
type QueryStruct struct {
	Person *PersonStruct          `json:"person"`
	Dog    *DogStruct             `json:"dog"`
	Animal Animal                 `json:"animal"`
	Alien  map[string]interface{} `json:"alien"`
}
type PersonStruct struct {
	Name   *string       `json:"name"`
	Spouse *PersonStruct `json:"spouse"`
	Pets   []*DogStruct  `json:"pets"`
	Age    int           `json:"age"`
}
type DogStruct struct {
	Name *string       `json:"name"`
	Mate *DogStruct    `json:"mate"`
	Age  int           `json:"age"`
	Tag  string        `json:"tag"`
	Hero *PersonStruct `json:"hero"`
}

func (this *PersonStruct) RelativeAge() int {
	return this.Age
}

func (this *DogStruct) RelativeAge() int {
	return this.Age * 7
}
func (this *DogStruct) DogYears() int {
	return this.Age * 7
}

func (this *DogStruct) SetTag(args *struct{ Value *string }) string {
	this.Tag = *args.Value
	return "ok"
}

func (this *DogStruct) SetHero(args *struct{ Value *PersonStruct }) string {
	this.Hero = args.Value
	return "ok"
}

func createQueryData() *QueryStruct {
	return &QueryStruct{
		Person: &PersonStruct{
			Name: p("Hiram"),
			Age:  35,
			Spouse: &PersonStruct{
				Name: p("Ana"),
				Age:  37,
			},
			Pets: []*DogStruct{
				&DogStruct{Name: p("Ginger"), Age: 4},
				&DogStruct{Name: p("Cameron"), Age: 7,},
			},
		},
		Dog: &DogStruct{
			Name: p("Ginger"),
			Age:  4,
		},
		Animal: &PersonStruct{
			Name: p("Ana"),
			Age:  37,
		},
		Alien: map[string]interface{}{
			"composition": "carbon & silicon",
			"shape":       "weird",
			"pet": &DogStruct{
				Name: p("Alf"),
				Age:  4,
			},
		},
	}
}

func p(s string) *string {
	return &s
}

func TestStructResolver(t *testing.T) {

	engine := graphql.New()
	err := engine.Schema.Parse(schemaText)
	assert.NoError(t, err)

	assertQuery(t, engine, `{ person { name } }`,
		`{"data":{"person":{"name":"Hiram"}}}`)

	assertQuery(t, engine,
		`{ person { name } }`,
		`{"data":{"person":{"name":"Hiram"}}}`)
}

func TestInterfaceResolver(t *testing.T) {
	engine := graphql.New()
	err := engine.Schema.Parse(schemaText)
	assert.NoError(t, err)

	data := createQueryData()
	assertRequestString(t, engine,
		`{"query":"{ dog { dogYears } }"}`,
		`{"data":{"dog":{"dogYears":28}}}`, data)

	//assertGraphQL(t, engine,
	//	`{"query":"{ dog { relativeAge } }"}`,
	//	`{"data":{"dog":{"relativeAge":28}}}`)
	//assertGraphQL(t, engine,
	//	`{"query":"{ animal { relativeAge } }"}`,
	//	`{"data":{"animal":{"relativeAge":37}}}`)
}

func TestInputArgs(t *testing.T) {
	engine := graphql.New()
	err := engine.Schema.Parse(`
schema {
	query: Query
}
type Query {
	dog: Dog
}
type Dog {
	tag: String
	setTag(value: String): String
}
`)
	assert.NoError(t, err)

	data := createQueryData()
	assertQuery(t, engine,
		`{ dog { setTag(value: "test") } }`,
		`{"data":{"dog":{"setTag":"ok"}}}`, data)

	assertQuery(t, engine,
		`{ dog { tag } }`,
		`{"data":{"dog":{"tag":"test"}}}`, data)

	assertQuery(t, engine,
		`{ dog { setTag(value: "test2") } }`,
		`{"data":{"dog":{"setTag":"ok"}}}`, data)

	assertQuery(t, engine,
		`{ dog { tag } }`,
		`{"data":{"dog":{"tag":"test2"}}}`, data)
}

func TestObjectInputArgs(t *testing.T) {
	engine := graphql.New()
	err := engine.Schema.Parse(`
schema {
	query: Query
}
type Query {
	dog: Dog
}
type Person {
	name: String
	age: Int
}
input PersonInput {
	name: String
	age: Int!
}
type Dog {
	hero: Person
	setHero(value: PersonInput): String
}
`)
	assert.NoError(t, err)

	data := createQueryData()
	assertQuery(t, engine,
		`{ dog { setHero(value: { name: "Hiram", age: 21 } ) } }`,
		`{"data":{"dog":{"setHero":"ok"}}}`, data)
	assertQuery(t, engine,
		`{ dog { hero { name }} }`,
		`{"data":{"dog":{"hero":{"name":"Hiram"}}}}`, data)

}

func TestMapResolver(t *testing.T) {
	engine := graphql.New()
	err := engine.Schema.Parse(schemaText)
	assert.NoError(t, err)

	assertRequestString(t, engine,
		`{"query":"{ alien { shape } }"}`,
		`{"data":{"alien":{"shape":"weird"}}}`)

	assertRequestString(t, engine,
		`{"query":"{ alien { pet { name } } }"}`,
		`{"data":{"alien":{"pet":{"name":"Alf"}}}}`)

}

func TestCustomTypeResolver(t *testing.T) {
	engine := graphql.New()
	err := engine.Schema.Parse(schemaText)

	require.NoError(t, err)

	engine.ResolverFactory = &resolvers.ResolverFactoryList{
		// First try out custom resolver...
		&resolvers.TypeResolverFactory{
			"Alien": func(request *resolvers.ResolveRequest) resolvers.Resolver {
				// Only interested in changing result of shape...
				if request.Field.Name == "shape" {
					return func() (reflect.Value, error) {
						return reflect.ValueOf("changed"), nil
					}

				}
				return nil
			},
		},

		// then use the default resolvers
		engine.ResolverFactory,
	}

	assertRequestString(t, engine,
		`{"query":"{ alien { shape, composition } }"}`,
		`{"data":{"alien":{"shape":"changed","composition":"carbon \u0026 silicon"}}}`)

}

func TestCustomAsyncResolvers(t *testing.T) {
	schema := `
		schema {
			query: Query
		}
		type Query {
			f1 : String
			f2 : String
			f3 : String
			f4 : String
		}
	`

	engine := graphql.New()
	err := engine.Schema.Parse(schema)
	require.NoError(t, err)
	engine.ResolverFactory = &resolvers.ResolverFactoryList{
		// First try out custom resolver...
		&resolvers.FuncResolverFactory{
			func(request *resolvers.ResolveRequest) resolvers.Resolver {
				return func() (reflect.Value, error) {
					time.Sleep(1 * time.Second)
					return reflect.ValueOf(request.Field.Name), nil
				}
			},
		},
		engine.ResolverFactory,
	}

	benchmark := testing.Benchmark(func(b *testing.B) {
		response := engine.Execute(context.TODO(), &graphql.EngineRequest{Query: "{f1,f2,f3,f4}"}, nil)
		assert.Equal(t, 0, len(response.Errors))
	})
	assert.True(t, benchmark.T.Seconds() > 3)
	assert.True(t, benchmark.T.Seconds() < 5)

	fmt.Println()
	engine = graphql.New()
	err = engine.Schema.Parse(schema)
	require.NoError(t, err)
	engine.ResolverFactory = &resolvers.ResolverFactoryList{
		// First try out custom resolver...
		&resolvers.FuncResolverFactory{
			func(request *resolvers.ResolveRequest) resolvers.Resolver {
				// Use request.RunAsync to signal that the resolution will run async:
				return request.RunAsync(func() (reflect.Value, error) {
					time.Sleep(1 * time.Second)
					return reflect.ValueOf(request.Field.Name), nil
				})
			},
		},
		engine.ResolverFactory,
	}
	benchmark = testing.Benchmark(func(b *testing.B) {
		response := engine.Execute(context.TODO(), &graphql.EngineRequest{Query: "{f1,f2,f3,f4}"}, nil)
		assert.Equal(t, 0, len(response.Errors))
	})
	assert.True(t, benchmark.T.Seconds() > 0)
	assert.True(t, benchmark.T.Seconds() < 2)

}

func assertQuery(t *testing.T, engine *graphql.Engine, query string, expected string, roots ...interface{}) {
	request := graphql.EngineRequest{}
	request.Query = query
	assertRequest(t, engine, request, expected, roots...)
}

func assertRequestString(t *testing.T, engine *graphql.Engine, req string, expected string, roots ...interface{}) {
	request := graphql.EngineRequest{}
	jsonUnmarshal(t, req, &request)
	assertRequest(t, engine, request, expected, roots...)
}

func assertRequest(t *testing.T, engine *graphql.Engine, request graphql.EngineRequest, expected string, roots ...interface{}) {
	var root interface{}
	if len(roots) > 0 {
		root = roots[0]
	} else {
		root = createQueryData()
	}
	response := engine.Execute(context.TODO(), &request, root)
	actual := jsonMarshal(t, response)
	assert.Equal(t, expected, actual)
}

func jsonMarshal(t *testing.T, value interface{}) string {
	data, err := json.Marshal(value)
	assert.NoError(t, err)
	return string(data)
}

func jsonUnmarshal(t *testing.T, from string, target interface{}) {
	err := json.Unmarshal([]byte(from), target)
	assert.NoError(t, err)
}
