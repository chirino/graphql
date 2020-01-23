package api

import (
    "encoding/json"
    "fmt"
    "github.com/chirino/graphql/resolvers"
    "github.com/pkg/errors"
    "net/http"
    "net/url"
    "reflect"
    "strings"
)

func (factory resolverFactory) CreateResolver(request *resolvers.ResolveRequest) resolvers.Resolver {
    if request.ParentType == nil {
        return nil
    }
    if factory.options.QueryType == request.ParentType.String() {
        if operation := factory.queryFields[request.Field.Name]; operation != nil {
            return factory.resolve(request, operation)
        }
    } else if factory.options.MutationType == request.ParentType.String() {
        if operation := factory.mutationFields[request.Field.Name]; operation != nil {
            return factory.resolve(request, operation)
        }
    }
    return nil
}

func (factory resolverFactory) resolve(gqlRequest *resolvers.ResolveRequest, operation *operation) resolvers.Resolver {
    path := operation.path
    query := url.Values{}
    headers := http.Header{}
    for _, param := range operation.definition.Parameters {
        param := param.Value
        qlid := sanitizeGraphQLID(param.Name)
        value, found := gqlRequest.Args[qlid]
        switch param.In {
        case "path":
            if !found { // all path params are required.
                panic("required path parameter not set: " + qlid)
            }
            strings.ReplaceAll(path, fmt.Sprintf("{%s}", param.Name), fmt.Sprintf("%s", value))

        case "query":
            if param.Required && !found {
                panic("required query parameter not set: " + qlid)
            }
            if found {
                query.Add(param.Name, fmt.Sprintf("%s", value))
            }
        case "header":
            if param.Required && !found {
                panic("required header parameter not set: " + qlid)
            }
            if found {
                headers.Add(param.Name, fmt.Sprintf("%s", value))
            }
        case "cookie":
            // TODO: consider how to best handle these...
        }
    }

    u, err := url.Parse(factory.options.BaseUrl + path)
    if err != nil {
        panic(err)
    }
    u.RawQuery = query.Encode()

    return func() (reflect.Value, error) {
        client := http.Client{}
        request, err := http.NewRequest(operation.method, u.String(), nil)
        if err != nil {
            return reflect.Value{}, errors.WithStack(err)
        }

        // TODO: If we are proxying.. we should fill this in from the original request...
        //headers.Set("X-Forwarded-Proto", "http")
        //headers.Set("X-Forwarded-Host", "localhost")
        //headers.Del("X-Forwarded-For")
        request.Header = headers

        resp, err := client.Do(request)
        if err != nil {
            return reflect.Value{}, errors.WithStack(err)
        }
        defer resp.Body.Close()

        status := fmt.Sprintf("%d", resp.StatusCode)
        if status == operation.status {
            var result map[string]interface{}
            err := json.NewDecoder(resp.Body).Decode(&result)
            if err != nil {
                return reflect.Value{}, errors.WithStack(err)
            }
            return reflect.ValueOf(result), nil
        } else {
            // error case....
            return reflect.Value{}, errors.New("failure status code: "+status)
        }
    }
}
