package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/graphiql"
	"github.com/chirino/graphql/httpgql"
)

type query struct {
	Name string `json:"name"`
}

func (q *query) Hello() string { return "Hello, " + q.Name }

func main() {
	engine := graphql.New()
	engine.Root = &query{
		Name: "World!",
	}
	err := engine.Schema.Parse(`
        schema {
            query: Query
        }
        type Query {
            name: String!
            hello: String!
        }
    `)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/graphql", &httpgql.Handler{ServeGraphQLStream: engine.ServeGraphQLStream})
	fmt.Println("GraphQL service running at http://localhost:8080/graphql")

	http.Handle("/", graphiql.New("ws://localhost:8080/graphql", true))
	fmt.Println("GraphiQL UI running at http://localhost:8080/")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
