# graphql [![GoDoc](https://godoc.org/github.com/chirino/graphql?status.svg)](https://godoc.org/github.com/chirino/graphql)
<!-- [![Sourcegraph](https://sourcegraph.com/github.com/chirino/graphql/-/badge.svg)](https://sourcegraph.com/github.com/chirino/graphql?badge) --> 

The goal of this project is to provide full support of the [GraphQL draft specification](https://facebook.github.io/graphql/draft) with a set of idiomatic, easy to use Go packages.

This project is still under heavy development and APIs are almost certainly subject to change.

## Features

- minimal API
- support for `context.Context`
- support for the `OpenTracing` standard
- schema type-checking against resolvers
- custom resolvers
- built int resolvers against maps, struct fields, interface methods
- resolve against external APIs that have an openapi spec
- handles panics in resolvers
- parallel execution of resolvers
- modifying schemas and generating new schema documents
- relay compatible http interface

## (Some) Documentation

### Basic Sample

```go
package main

import (
    "fmt"
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/relay"
    "github.com/friendsofgo/graphiql"
    "log"
    "net/http"
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

    addr := ":8080"
    http.Handle("/graphql", &relay.Handler{Engine: engine})
    fmt.Println("GraphQL service running at http://localhost" + addr + "/graphql")

    graphiql, _ := graphiql.NewGraphiqlHandler("/graphql")
    http.Handle("/graphiql", graphiql)
    fmt.Println("GraphiQL UI running at http://localhost" + addr + "/graphiql")
    
    log.Fatal(http.ListenAndServe(addr, nil))
}
```

To test:
```sh
$ curl -XPOST -d '{"query": "{ hello }"}' localhost:8080/query
```

### Resolvers

Resolvers implement accessing that data selected by the GraphQL query or mutations.

When a graphql field is access, we the graphql engine tries using the following resolver factories until one of them
is able to create a resolver for the field:
1. `resolvers.MetadataResolverFactory`: Resolves the `__typename`, `__schema`, and `__type` metadata fields.
2. `resolvers.MethodResolverFactory`: Resolves the field using exported functions implemented on the current struct
3. `resolvers.FieldResolverFactory`: Resolves the field using exported fields on the current struct
4. `resolvers.MapResolverFactory`: Resolves the field using map entries on the current map.

If no resolver factory can resolve the GraphQL field, then it results in a GraphQL error.

#### Resolving fields using `resolvers.MethodResolverFactory`

The method name has to be [exported](https://golang.org/ref/spec#Exported_identifiers) and match the field's name
in a non-case-sensitive way.

The method has up to two arguments:

- Optional `context.Context` argument or `resolvers.ExecutionContext`.
- Mandatory `*struct { ... }` argument if the corresponding GraphQL field has arguments. The names of the struct
fields have to be [exported](https://golang.org/ref/spec#Exported_identifiers) and have to match the names of the
GraphQL arguments in a non-case-sensitive way.

The method has up to two results:

- The GraphQL field's value as determined by the resolver.
- Optional `error` result.

Example for a simple resolver method:

```go
func (r *helloWorldResolver) Hello() string {
    return "Hello world!"
}
```

The following signature is also allowed:

```go
func (r *helloWorldResolver) Hello(ctx resolvers.ExecutionContext) (string, error) {
    return "Hello world!", nil
}
```


#### Resolving fields using `resolvers.FieldResolverFactory`

The method name has to be [exported](https://golang.org/ref/spec#Exported_identifiers). The GraphQL field's name
must match the struct field name or the name in a `json:""` annotation.

Example of a simple resolver struct:

```go
type Query struct {
    Name   string
    Length float64  `json:"length"`
}
```

Could be access with the following GraphQL type:

```graphql
type Query {
    Name: String!
    length: Float!
}
```

#### Resolving fields using `resolvers.MapResolverFactory`

You must use maps with string keys.

Example of a simple resolver map:

```go
query := map[string]interface{}{
    "Name": "Millennium Falcon",
    "length": 34.37,
}
```

Could be access with the following GraphQL type:

```graphql
type Query {
    Name: String
    length: Float
}
```

### Custom Resolvers

You can change the resolver factories used by the engine and implement custom resolver factories if needed.

Changing the default list of resolver factories so that only metadata and method based resolvers are used:
```go
engine, err := graphql.CreateEngine(starwars.Schema)
engine.ResolverFactory = &resolvers.ResolverFactoryList{
        &resolvers.MetadataResolverFactory{},
        &resolvers.MethodResolverFactory{},
}
```

You can also implement a custom resolver factory.  Here's an example resolver factory, that would resolve all `foo` GraphQL
fields to the value `"bar"`:

```go
type MyFooResolverFactory struct{}
func (this *MyFooResolverFactory) CreateResolver(request *resolvers.ResolveRequest) resolvers.Resolver {
    if request.Field.Name!="foo" {
        return nil // Lets only handle the foo fields:
    }
    return func() (reflect.Value, error) {
        return reflect.ValueOf("bar"), nil
    }
}
```

or you could do like:

```go
myFooResolverFactory := resolvers.FuncResolverFactory{ func (request *resolvers.ResolveRequest) resolvers.Resolver {
    if request.Field.Name!="foo" {
        return nil // Lets only handle the foo fields:
    }
    return func() (reflect.Value, error) {
        return reflect.ValueOf("bar"), nil
    }
}}
```

If you only want to apply a custom resolver for a specific GraphQL type, you can do it like:
```go
myTypeResolverFactory := resolvers.TypeResolverFactory {
    "Query": func (request *resolvers.ResolveRequest) resolvers.Resolver {
        if request.Field.Name!="foo" {
            return nil // Lets only handle the foo fields:
        }
        return func() (reflect.Value, error) {
            return reflect.ValueOf("bar"), nil
        }
    },
}
```

### Async Resolvers

If resolvers are going to fetch data from multiple remote systems, you will want to resolve those async so that
each fetch is done in parallel.  You can enable async resolution in your custom resolver using the following pattern:

```go
type MyHttpResolverFactory struct{}
func (this *MyHttpResolverFactory) CreateResolver(request *resolvers.ResolveRequest) resolvers.Resolver {

    if request.Field.Name!="google" {
        return nil // Lets only handle the google field:
    }

    httpClient := &http.Client{}
    req, err := http.NewRequest("GET", "http://google.com", nil)
    if err!=nil {
        panic(err)
    }

    return request.RunAsync(func() (reflect.Value, error) {
        resp, err := httpClient.Do(req)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()

        data, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            return nil, err
        }

        return reflect.ValueOf(string(data)), nil
    })
}
```

### History / Credits

This is a fork of the http://github.com/graph-gophers/graphql-go project.  

