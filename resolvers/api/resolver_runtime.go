package api

import (
    "bytes"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "github.com/chirino/graphql/resolvers"
    "github.com/getkin/kin-openapi/openapi3"
    "github.com/pkg/errors"
    "io"
    "io/ioutil"
    "net/http"
    "net/url"
    "reflect"
    "strings"
)

func (factory *resolverFactory) convert(request *resolvers.ResolveRequest, next resolvers.Resolver) resolvers.Resolver {
    fieldType := request.Field.Type.String()
    if converter, ok := factory.resultConverters[fieldType]; ok {
        return func() (value reflect.Value, err error) {
            return converter(next())
        }
    }
    return next
}

func (factory *resolverFactory) CreateResolver(request *resolvers.ResolveRequest) resolvers.Resolver {
    key := request.ParentType.String() + ":" + request.Field.Name
    if r, ok := factory.resolvers[key]; ok {
        resolver := r.CreateResolver(request)
        if resolver != nil {
            return factory.convert(request, resolver)
        }
    }

    // We need these one to traverse the json results that are held as maps...
    resolver := resolvers.MapResolverFactory.CreateResolver(request)
    if resolver != nil {
        return factory.convert(request, resolver)
    }
    // And this one to handle Additional properties conversions.
    resolver = resolvers.FieldResolverFactory.CreateResolver(request)
    if resolver != nil {
        return factory.convert(request, resolver)
    }

    return nil
}

func (factory resolverFactory) resolve(gqlRequest *resolvers.ResolveRequest, operation *openapi3.Operation, method string, path string, status string) resolvers.Resolver {
    return func() (reflect.Value, error) {

        query := url.Values{}
        headers := http.Header{}

        ctx := gqlRequest.Context.GetContext()
        if severRequest := ctx.Value("*net/http.Request"); severRequest != nil {
            if serverRequest, ok := severRequest.(*http.Request); ok {
                // Let's act like a proxy and pass through all the headers...
                headers = serverRequest.Header.Clone()

                // And add set some proxy headers.
                //headers.Add("X-Forwarded-Proto", serverRequest.URL.Scheme)
                headers.Add("X-Forwarded-Host", serverRequest.Host)
                headers.Add("X-Forwarded-For", serverRequest.RemoteAddr)

                cookie, err := serverRequest.Cookie("Authorization")
                if err == nil && cookie.Value != "" {
                    headers.Set("Authorization", cookie.Value)
                }
            }
        }

        if factory.options.APIBase.BearerToken != "" && headers.Get("Authorization") == "" {
            headers.Set("Authorization", "Bearer "+factory.options.APIBase.BearerToken)
        }

        for _, param := range operation.Parameters {
            param := param.Value
            qlid := sanitizeName(param.Name)
            value, found := gqlRequest.Args[qlid]
            switch param.In {
            case "path":
                if !found { // all path params are required.
                    panic("required path parameter not set: " + qlid)
                }
                path = strings.ReplaceAll(path, fmt.Sprintf("{%s}", param.Name), fmt.Sprintf("%s", value))

            case "query":
                if param.Required && !found {
                    panic("required query parameter not set: " + qlid)
                }
                if found {
                    query.Set(param.Name, fmt.Sprintf("%s", value))
                }
            case "header":
                if param.Required && !found {
                    panic("required header parameter not set: " + qlid)
                }
                if found {
                    headers.Set(param.Name, fmt.Sprintf("%s", value))
                }
            case "cookie":
                // TODO: consider how to best handle these...
            }
        }

        headers.Set("Content-Type", "application/json")
        headers.Set("Accept", "application/json")

        apiURL, err := url.Parse(factory.options.APIBase.URL)
        if err != nil {
            return reflect.Value{}, errors.WithStack(err)
        }

        apiURL.Path += path
        apiURL.RawQuery = query.Encode()

        client := factory.options.APIBase.Client
        if client == nil {
            client = &http.Client{Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: factory.options.APIBase.InsecureClient},
            }}
        }

        var body io.Reader = nil
        if operation.RequestBody != nil {
            content := operation.RequestBody.Value.Content.Get("application/json")
            if content != nil {

                v, err := factory.inputConverters.Convert(gqlRequest.Field.Args.Get("body").Type, gqlRequest.Args["body"])
                if err != nil {
                    return reflect.Value{}, errors.WithStack(err)
                }

                data, err := json.Marshal(v)
                if err != nil {
                    return reflect.Value{}, errors.WithStack(err)
                }
                body = bytes.NewReader(data)
            }
        }

        request, err := http.NewRequestWithContext(ctx, method, apiURL.String(), body)
        if err != nil {
            return reflect.Value{}, errors.WithStack(err)
        }

        if operation.RequestBody != nil {
            content := operation.RequestBody.Value.Content.Get("application/json")
            if content != nil {
            }
        }

        request.Header = headers
        resp, err := client.Do(request)
        if err != nil {
            return reflect.Value{}, errors.WithStack(err)
        }
        defer resp.Body.Close()

        statusCode := fmt.Sprintf("%d", resp.StatusCode)
        if status == statusCode {
            var result map[string]interface{}
            err := json.NewDecoder(resp.Body).Decode(&result)
            if err != nil {
                return reflect.Value{}, errors.WithStack(err)
            }
            return reflect.ValueOf(result), nil
        } else {
            // error case....
            all, _ := ioutil.ReadAll(resp.Body)
            return reflect.Value{}, errors.New("http request status code: " + statusCode + ", body: " + string(all))
        }
    }
}
