package api

import (
    "encoding/json"
    "fmt"
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/resolvers"
    "io"
    "net/url"
    "os"
)

type URL struct {
    *url.URL
}

func (u *URL) UnmarshalJSON(b []byte) error {
    url, err := url.Parse(fmt.Sprintf("%s", b[1:len(b)-1]))
    if err != nil {
        return err
    }
    u.URL = url
    return nil
}

func (u *URL) MarshalJSON() ([]byte, error) {
    return json.Marshal(u.String())
}

type EndpointOptions struct {
    URL            URL
    BearerToken    string
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

    doc, err := LoadOpenApiDoc(o.Openapi)
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
