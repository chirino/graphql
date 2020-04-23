package main

import (
	"fmt"
	"github.com/chirino/graphql/internal/deprecated"
	"github.com/chirino/graphql/relay"
	"github.com/friendsofgo/graphiql"
	"log"
	"net/http"

	"github.com/chirino/graphql/internal/example/starwars"
)

var schema *deprecated.Schema

func init() {
	schema = deprecated.MustParseSchema(starwars.Schema, &starwars.Resolver{})
}

func main() {
	http.Handle("/query", &relay.Handler{ServeGraphQLStream: schema.Engine.ServeGraphQLStream})
	graphiql, _ := graphiql.NewGraphiqlHandler("/query")
	http.Handle("/", graphiql)

	fmt.Println("GraphiQL UI running at http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
