package graphql_test

import (
    "context"
    "fmt"
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/internal/gqltesting"
    "github.com/chirino/graphql/resolvers"
    "github.com/chirino/graphql/schema"
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

func root() *QueryStruct {
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
    engine.Root = root()

    err := engine.Schema.Parse(schemaText)
    assert.NoError(t, err)

    gqltesting.AssertQuery(t, engine, `{ person { name } }`,
        `{"data":{"person":{"name":"Hiram"}}}`)

    gqltesting.AssertQuery(t, engine,
        `{ person { name } }`,
        `{"data":{"person":{"name":"Hiram"}}}`)
}

func TestInterfaceResolver(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(schemaText)
    assert.NoError(t, err)

    gqltesting.AssertRequestString(t, engine,
        `{"query":"{ dog { dogYears } }"}`,
        `{"data":{"dog":{"dogYears":28}}}`)

    //assertGraphQL(t, engine,
    //	`{"query":"{ dog { relativeAge } }"}`,
    //	`{"data":{"dog":{"relativeAge":28}}}`)
    //assertGraphQL(t, engine,
    //	`{"query":"{ animal { relativeAge } }"}`,
    //	`{"data":{"animal":{"relativeAge":37}}}`)
}

func TestInputArgs(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

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

    gqltesting.AssertQuery(t, engine,
        `{ dog { setTag(value: "test") } }`,
        `{"data":{"dog":{"setTag":"ok"}}}`)

    gqltesting.AssertQuery(t, engine,
        `{ dog { tag } }`,
        `{"data":{"dog":{"tag":"test"}}}`)

    gqltesting.AssertQuery(t, engine,
        `{ dog { setTag(value: "test2") } }`,
        `{"data":{"dog":{"setTag":"ok"}}}`)

    gqltesting.AssertQuery(t, engine,
        `{ dog { tag } }`,
        `{"data":{"dog":{"tag":"test2"}}}`)
}

func TestObjectInputArgs(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()
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

    gqltesting.AssertQuery(t, engine,
        `{ dog { setHero(value: { name: "Hiram", age: 21 } ) } }`,
        `{"data":{"dog":{"setHero":"ok"}}}`)
    gqltesting.AssertQuery(t, engine,
        `{ dog { hero { name }} }`,
        `{"data":{"dog":{"hero":{"name":"Hiram"}}}}`)

}

func TestMapResolver(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(schemaText)
    assert.NoError(t, err)

    gqltesting.AssertRequestString(t, engine,
        `{"query":"{ alien { shape } }"}`,
        `{"data":{"alien":{"shape":"weird"}}}`)

    gqltesting.AssertRequestString(t, engine,
        `{"query":"{ alien { pet { name } } }"}`,
        `{"data":{"alien":{"pet":{"name":"Alf"}}}}`)

}

func TestCustomTypeResolver(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(schemaText)

    require.NoError(t, err)

    engine.Resolver = &resolvers.ResolverList{
        // First try out custom resolver...
        &resolvers.TypeResolver{
            "Alien": resolvers.Func(func(request *resolvers.ResolveRequest) resolvers.Resolution {
                // Only interested in changing result of shape...
                if request.Field.Name == "shape" {
                    return func() (reflect.Value, error) {
                        return reflect.ValueOf("changed"), nil
                    }

                }
                return nil
            }),
        },

        // then use the default resolvers
        engine.Resolver,
    }

    gqltesting.AssertRequestString(t, engine,
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
    engine.Root = root()

    err := engine.Schema.Parse(schema)
    require.NoError(t, err)
    engine.Resolver = &resolvers.ResolverList{
        // First try out custom resolver...
        resolvers.Func(
            func(request *resolvers.ResolveRequest) resolvers.Resolution {
                return func() (reflect.Value, error) {
                    time.Sleep(1 * time.Second)
                    return reflect.ValueOf(request.Field.Name), nil
                }
            },
        ),
        engine.Resolver,
    }

    benchmark := testing.Benchmark(func(b *testing.B) {
        response := engine.Execute(context.TODO(), &graphql.EngineRequest{Query: "{f1,f2,f3,f4}"}, nil)
        assert.Equal(t, 0, len(response.Errors))
    })
    assert.True(t, benchmark.T.Seconds() > 3)
    assert.True(t, benchmark.T.Seconds() < 5)

    fmt.Println()
    engine = graphql.New()
    engine.Root = root()

    err = engine.Schema.Parse(schema)
    require.NoError(t, err)
    engine.Resolver = &resolvers.ResolverList{
        // First try out custom resolver...
        resolvers.Func(
            func(request *resolvers.ResolveRequest) resolvers.Resolution {
                // Use request.RunAsync to signal that the resolution will run async:
                return request.RunAsync(func() (reflect.Value, error) {
                    time.Sleep(1 * time.Second)
                    return reflect.ValueOf(request.Field.Name), nil
                })
            },
        ),
        engine.Resolver,
    }
    benchmark = testing.Benchmark(func(b *testing.B) {
        response := engine.Execute(context.TODO(), &graphql.EngineRequest{Query: "{f1,f2,f3,f4}"}, nil)
        assert.Equal(t, 0, len(response.Errors))
    })
    assert.True(t, benchmark.T.Seconds() > 0)
    assert.True(t, benchmark.T.Seconds() < 2)

}

func TestTypeDirectives(t *testing.T) {

    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(`
type Test @test(foo:"bar") {
	name: String
}
`)
    assert.NoError(t, err)
    object := engine.Schema.Types[`Test`].(*schema.Object)
    assert.Equal(t, len(object.Directives), 1)
}

func TestTypeRedeclaration(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(`
type Test {
	firstName: String
}
type Test {
	lastName: String
}
`)
    assert.NoError(t, err)
    object := engine.Schema.Types[`Test`].(*schema.Object)
    assert.Equal(t, 1, len(object.Fields), 1)
    assert.Equal(t, "lastName", object.Fields[0].Name)
}

func TestGraphqlAddDirective(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(`
type Test {
	firstName: String
}
type Test @graphql(alter:"add") {
	lastName: String
}
`)
    assert.NoError(t, err)
    object := engine.Schema.Types[`Test`].(*schema.Object)
    assert.Equal(t, 2, len(object.Fields))
    assert.Equal(t, "firstName", object.Fields[0].Name)
    assert.Equal(t, "lastName", object.Fields[1].Name)
}

func TestGraphqlDropDirective(t *testing.T) {
    engine := graphql.New()
    engine.Root = root()

    err := engine.Schema.Parse(`
type Test {
	firstName: String
	lastName: String
}
type Test @graphql(alter:"drop") {
	lastName: String
}
`)
    assert.NoError(t, err)
    object := engine.Schema.Types[`Test`].(*schema.Object)
    assert.Equal(t, 1, len(object.Fields))
    assert.Equal(t, "firstName", object.Fields[0].Name)
}

type MyMutation struct {
    name string
}

func (m*MyMutation) SetName(args struct{ Name string }) string {
    m.name = args.Name
    return "Hi "+m.name
}

func TestMutationStringArgs(t *testing.T) {
    engine := graphql.New()
    root := &MyMutation{}
    engine.Root = root

    err := engine.Schema.Parse(`
schema {
    mutation: MyMutation
}
type MyMutation {
	setName(name: String!): String
}
`)
    assert.NoError(t, err)
    result := ""
    err = engine.Exec(context.Background(), &result, `mutation{ setName(name: "Hiram") }`)
    assert.NoError(t, err)
    assert.Equal(t, `{"setName":"Hi Hiram"}`, result)
    assert.Equal(t, "Hiram", root.name)
}

func TestMutationBlockStringArgs(t *testing.T) {
    engine := graphql.New()
    root := &MyMutation{}
    engine.Root = root

    err := engine.Schema.Parse(`
schema {
    mutation: MyMutation
}
type MyMutation {
	setName(name: String!): String
}
`)
    require.NoError(t, err)
    result := ""
    err = engine.Exec(context.Background(), &result, `mutation{ setName(name: """Hiram""") }`)
    require.NoError(t, err)
    assert.Equal(t, `{"setName":"Hi Hiram"}`, result)
    assert.Equal(t, "Hiram", root.name)
}
