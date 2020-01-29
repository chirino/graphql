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

    http.Handle("/graphql", &relay.Handler{Engine: engine})
    fmt.Println("GraphQL service running at http://localhost:8080/graphql")

    graphiql, _ := graphiql.NewGraphiqlHandler("/graphql")
    http.Handle("/graphiql", graphiql)
    fmt.Println("GraphiQL UI running at http://localhost:8080/graphiql")

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

To test:
```sh
$ curl -XPOST -d '{"query": "{ hello }"}' localhost:8080/query
```

Or open `http://localhost:8080/graphiql` and use the graphiql UI.

### Example Walk Through

* Step 1: `engine := graphql.New()` : Create the GraphQL engine
* Step 2: `engine.Root = ...`: Associate a root object that the resolvers will operate against with the  call.  
If you use custom resolvers this may not be required, but all the default resolvers, navigate this Root object. 
* Step 3:  `err := engine.Schema.Parse(...)`: Configure the engine to know what the valid graphql operations that can 
be performed using the [`GraphQL schema language`](https://graphql.org/learn/schema/#type-language).
* Step 4:  `http.Handle("/graphql", &relay.Handler{Engine: engine})`: Use a relay http interface to access the GraphQL 
engine. 
* Step 5:  `log.Fatal(http.ListenAndServe(":8080", nil))`: Start the http server

The example also starts a `http://localhost:8080/graphiql` which is optional. It gives you a nice UI interface
to introspect your graphql endpoint and test it out.

### Schema Updates

You can call `err := engine.Schema.Parse(...)` multiple times to evolve the schema. By default any types
redeclared will replace the previous definition.  You can use engine.Schema.String() to get the schema back in the 
[`GraphQL schema language`](https://graphql.org/learn/schema/#type-language).

You can use a `@graphql(alter:"add")` directive to add fields, directives, or interfaces to previously defined type.

For example, lets say your  previously parsed:
```graphql
type Person {
    name: String
}
```

If you then subsequently parse:
```graphql
type Person @graphql(alter:"add") {
    children: [Person!]
}
```

then the resulting schema would be:
```graphql
type Person {
    name: String
    children: [Person!]
}
```

Similarly you can use `@graphql(alter:"drop")` to remove fields, directives, or interfaces from a type.

### Resolvers

Resolvers implement accessing that data selected by the GraphQL query or mutations.

When a graphql field is access, we the graphql engine tries using each of the following resolvers until one of them
is able to resolve field:
1. `resolvers.MetadataResolver`: Resolves the `__typename`, `__schema`, and `__type` metadata fields.
2. `resolvers.MethodResolver`: Resolves the field using exported functions implemented on the current struct
3. `resolvers.FieldResolver`: Resolves the field using exported fields on the current struct
4. `resolvers.MapResolver`: Resolves the field using map entries on the current map.

If no resolver can resolve the GraphQL field, then it results in a GraphQL error.

#### Resolving fields using `resolvers.MethodResolver`

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

#### Resolving fields using `resolvers.FieldResolver`

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

#### Resolving fields using `resolvers.MapResolver`

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

You can change the resolvers used by the engine and implement custom resolvers if needed.

Changing the default list of resolvers so that only metadata and method based resolvers are used:
```go
engine.ResolverFactory = &resolvers.ResolverList{
    resolvers.MetadataResolver,
    resolvers.MethodResolver,
}
```

You can also implement a custom resolver.  Here's an example resolver, that would resolve all `foo` GraphQL
fields to the value `"bar"`:

```go
type MyFooResolver struct{}
func (this *MyFooResolver) Resolve(request *resolvers.ResolveRequest) resolvers.Resolution {
    if request.Field.Name!="foo" {
        return nil // Lets only handle the foo fields:
    }
    return func() (reflect.Value, error) {
        return reflect.ValueOf("bar"), nil
    }
}
```

### Async Resolvers

If resolvers are going to fetch data from multiple remote systems, you will want to resolve those async so that
each fetch is done in parallel.  You can enable async resolution in your custom resolver using the following pattern:

```go
type MyHttpResolver struct{}
func (this *MyHttpResolver) Resolve(request *resolvers.ResolveRequest) resolvers.Resolution {

    if request.Field.Name!="google" {
        return nil // Lets only handle the google field:
    }

    return request.RunAsync(func() (reflect.Value, error) {
        httpClient := &http.Client{}
        req, err := http.NewRequest("GET", "http://google.com", nil)
        if err!=nil {
            return nil, error
        }
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

## License

[BSD](./LICENSE)

## Development

* We love [pull requests](https://github.com/chirino/graphql/pulls)
* Having problems? Open an [issue](https://github.com/chirino/graphql/issues)
* This project is written in [Go](https://golang.org/).  It should work on any platform where go is supported.
* 

## Future Work

* setup ci jobs to validate PRs
* provide better hooks to implement custom directives so you can do things like configuring/selecting resolvers
  using directives.  Or using directives to drive schema generation.
* implement the GraphQL websocket interface
* implement subscriptions
* provide [dataloader](https://github.com/graphql/dataloader) functionality 

### Related Projects 

* [graphql-4-apis](https://github.com/chirino/graphql-4-apis)

### History / Credits

This is a fork of the http://github.com/graph-gophers/graphql-go project.  

