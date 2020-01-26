package main

import (
    "fmt"
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/relay"
    "github.com/chirino/graphql/resolvers"
    "github.com/chirino/graphql/resolvers/api"
    "github.com/friendsofgo/graphiql"
    "github.com/ghodss/yaml"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "time"
)

func main() {
    if len(os.Args) != 2 {
        panic("Invalid usage, expecting: example {config.yaml}")
    }

    file, err := ioutil.ReadFile(os.Args[1])
    if err != nil {
        log.Fatalf("%+v", err)
    }

    config := api.ApiResolverOptions{}
    err = yaml.Unmarshal(file, &config)
    if err != nil {
        log.Fatalf("%+v", err)
    }
    config.QueryType = `QueryApi`
    config.MutationType = `MutationApi`
    config.Logs = ioutil.Discard

    engine := graphql.New()
    err = api.MountApi(engine, config)
    if err != nil {
        log.Fatalf("%+v", err)
    }

    err = engine.Schema.Parse(`
        type Query {
            # Access to the API
            api: QueryApi,
        }
        type Mutation {
            # Saves a Authorization Bearer token in a browser cookie that 
            # is then subsequently used when issuing requests to the API.
            login(token:String!): String
            # Clears the Authorization Bearer token previously stored in a browser cookie.
            logout(): String
            # Access to the API
            api: MutationApi,
        }
        schema {
            query: Query
            mutation: Mutation
        }
    `)
    if err != nil {
        log.Fatalf("%+v", err)
    }
    engine.Root = root(0)

    http.Handle("/graphql", &relay.Handler{Engine: engine})
    graphiql, _ := graphiql.NewGraphiqlHandler("/graphql")
    http.Handle("/graphiql", graphiql)

    addr := ":8080"
    fmt.Println("GraphQL service running at http://localhost" + addr + "/graphql")
    fmt.Println("GraphiQL UI running at http://localhost" + addr + "/graphiql")
    log.Fatal(http.ListenAndServe(addr, nil))
}

type root byte

func (root) Login(rctx resolvers.ExecutionContext, args struct{ Token string }) string {
    ctx := rctx.GetContext()
    if r := ctx.Value("net/http.ResponseWriter"); r != nil {
        if r, ok := r.(http.ResponseWriter); ok {
            http.SetCookie(r, &http.Cookie{
                Name:    "Authorization",
                Value:   "Bearer " + args.Token,
                Path:    "/",
                Expires: time.Now().Add(1 * time.Hour),
            })
        }
    }
    return "ok"
}

func (root) Logout(rctx resolvers.ExecutionContext) string {
    ctx := rctx.GetContext()
    if r := ctx.Value("net/http.ResponseWriter"); r != nil {
        if r, ok := r.(http.ResponseWriter); ok {
            http.SetCookie(r, &http.Cookie{
                Name:    "Authorization",
                Value:   "",
                Path:    "/",
                Expires: time.Now().Add(-10000 * time.Hour),
            })
        }
    }
    return "ok"
}

func (root) Api() string {
    return "ok"
}
