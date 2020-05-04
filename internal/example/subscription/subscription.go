package main

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/graphiql"
	"github.com/chirino/graphql/httpgql"
	"github.com/chirino/graphql/resolvers"
)

type root struct {
	Test string `json:"test"`
}

func (m *root) Hello(ctx resolvers.ExecutionContext, args struct{ Duration int }) {
	go func() {
		counter := args.Duration
		for {
			select {
			case <-ctx.GetContext().Done():
				// We could close any resources held open for the subscription here.
				return
			case <-time.After(time.Duration(args.Duration) * time.Millisecond):
				// every few duration ms.. fire a subscription event.
				ctx.FireSubscriptionEvent(reflect.ValueOf(fmt.Sprintf("Hello: %d", counter)))
				counter += args.Duration
			}
		}
	}()
}

func main() {
	engine := graphql.New()
	engine.Root = &root{Test: "Hi!"}
	err := engine.Schema.Parse(`
        schema {
            query: MyQuery
            subscription: MySubscription
        }
        type MyQuery {
            test: String
        }
        type MySubscription {
            hello(duration: Int!): String
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
