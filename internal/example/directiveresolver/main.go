package main

import (
	"fmt"
	"github.com/chirino/graphql"
	"github.com/chirino/graphql/graphiql"
	"github.com/chirino/graphql/relay"
	"github.com/chirino/graphql/resolvers"
	"log"
	"net/http"
	"reflect"
)

type query struct {
	Name string `json:"name"`
}

func main() {
	engine := graphql.New()
	engine.Root = &query{
		Name: "Hiram",
	}
	err := engine.Schema.Parse(`
        directive @static(prefix: String) on FIELD
        schema {
            query: Query
        }
        type Query { 
            name: String! @static(prefix: "Hello ")
        }
    `)

	// Lets register a resolver that is applied to fields with the @static directive
	engine.Resolver = resolvers.List(engine.Resolver, resolvers.DirectiveResolver{
		Directive: "static",
		Create: func(request *resolvers.ResolveRequest, next resolvers.Resolution, args map[string]interface{}) resolvers.Resolution {

			// This resolver just filters the next result bu applying a prefix..
			// so we need the next resolver to be valid.
			if next == nil {
				return nil
			}
			return func() (reflect.Value, error) {
				value, err := next()
				if err != nil {
					return value, err
				}
				v := fmt.Sprintf("%s%s", args["prefix"], value.String())
				return reflect.ValueOf(v), nil
			}
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/graphql", &relay.Handler{Engine: engine})
	fmt.Println("GraphQL service running at http://localhost:8080/graphql")

	http.Handle("/", graphiql.New("ws://localhost:8080/graphql", true))
	fmt.Println("GraphiQL UI running at http://localhost:8080/")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
