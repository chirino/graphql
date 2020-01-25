package api

import (
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/resolvers"
    "io"
    "os"
)

// EndpointOptions defines how to access an endpoint URL
type EndpointOptions struct {
    // URL is the endpoint or endpoint base path that will be accessed.
    URL            string
    // BearerToken is the Authentication Bearer token that will added to the request headers.
    BearerToken    string
    // InsecureClient allows the client request to connect to TLS servers that do not have a valid certificate.
    InsecureClient bool
}

type ApiResolverOptions struct {
    Openapi      EndpointOptions
    APIBase      EndpointOptions
    QueryType    string
    MutationType string
    Logs         io.Writer
}

func MountApi(engine *graphql.Engine, option ApiResolverOptions) error {
    o := ApiResolverOptions{
        QueryType:    "Query",
        MutationType: "Mutation",
        Logs:         os.Stderr,
    }
    if option.Logs != nil {
        o.Logs = option.Logs
    }
    if option.QueryType != "" {
        o.QueryType = option.QueryType
    }
    if option.MutationType != "" {
        o.MutationType = option.MutationType
    }
    o.Openapi = option.Openapi
    o.APIBase = option.APIBase

    doc, err := LoadOpenApiV2orV3Doc(o.Openapi)
    if err != nil {
        return err
    }

    resolver, schema, err := NewResolverFactory(doc, o)
    if err != nil {
        return err
    }
    engine.Schema.Parse(schema)
    engine.ResolverFactory = resolvers.List(resolver, engine.ResolverFactory)
    return nil
}
