package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/chirino/graphql/httpgql"
	"github.com/chirino/graphql/internal/deprecated"
	"github.com/friendsofgo/graphiql"

	"github.com/chirino/graphql/internal/example/starwars"
)

var schema *deprecated.Schema

func init() {
	schema = deprecated.MustParseSchema(starwars.Schema, &starwars.Resolver{})
}

func main() {
	http.Handle("/query", &httpgql.Handler{ServeGraphQLStream: schema.Engine.ServeGraphQLStream})
	graphiql, _ := graphiql.NewGraphiqlHandler("/query")
	http.Handle("/", graphiql)

	fmt.Println("GraphiQL UI running at http://localhost:8085/")
	log.Fatal(http.ListenAndServe(":8085", nil))
}
