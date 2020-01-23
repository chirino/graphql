package api

import (
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/resolvers"
    "io"
    "io/ioutil"
)

type ApiResolverOptions struct {
    BaseUrl      string
    QueryType    string
    MutationType string
    Logs         io.Writer
}

func MountApi(engine *graphql.Engine, docLocation string, options ...ApiResolverOptions) error {
    doc, err := LoadOpenApiDoc(docLocation)
    if err != nil {
        return err
    }

    o := ApiResolverOptions{
        QueryType:    "Query",
        MutationType: "Mutation",
        Logs:         ioutil.Discard,
    }
    for _, option := range options {
        if option.Logs != nil {
            o.Logs = option.Logs
        }
        if option.BaseUrl != "" {
            o.BaseUrl = option.BaseUrl
        }
        if option.QueryType != "" {
            o.QueryType = option.QueryType
        }
        if option.MutationType != "" {
            o.MutationType = option.MutationType
        }
    }
    resolver, schema, err := NewResolverFactory(doc, o)
    if err != nil {
        return err
    }
    engine.Schema.Parse(schema)
    engine.ResolverFactory = resolvers.List(resolver, engine.ResolverFactory)
    return nil
}
