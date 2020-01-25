package api

import (
    "crypto/tls"
    "encoding/json"
    "fmt"
    "github.com/getkin/kin-openapi/openapi2"
    "github.com/getkin/kin-openapi/openapi2conv"
    "github.com/getkin/kin-openapi/openapi3"
    "github.com/pkg/errors"
    "io/ioutil"
    "net/http"
    "net/url"
    "regexp"
)

func LoadOpenApiV2orV3Doc(docLocation EndpointOptions) (*openapi3.Swagger, error) {

    var data, err = readURL(docLocation)
    if err != nil {
        return nil, err
    }

    // Lets detect if it's openapi 2 or 3
    doc := struct {
        Swagger string `json:"swagger,omitempty"`
        OpenAPI string `json:"openapi,omitempty"`
    }{}

    err = json.Unmarshal(data, &doc)
    if err != nil {
        return nil, errors.Wrap(err, "could not detect openapi version")
    }


    location, err := url.Parse(docLocation.URL)
    if err != nil {
        return nil, err
    }

    if doc.Swagger != "" || doc.OpenAPI == "" {
        // Lets load it up as openapi v2 and convert to v3
        var swagger2 openapi2.Swagger
        err := json.Unmarshal(data, &swagger2)
        if err != nil {
            return nil, errors.WithStack(err)
        }

        apiDoc, err := openapi2conv.ToV3Swagger(&swagger2)
        if err != nil {
            return nil, errors.WithStack(err)
        }

        err = openapi3.NewSwaggerLoader().ResolveRefsIn(apiDoc, location)
        if err != nil {
            return nil, errors.WithStack(err)
        }
        enrichApiDoc(apiDoc)
        return apiDoc, nil
    } else {
        // It should be a v3 document already..
        apiDoc, err := openapi3.NewSwaggerLoader().LoadSwaggerFromDataWithPath(data, location)
        if err != nil {
            return nil, errors.WithStack(err)
        }
        enrichApiDoc(apiDoc)
        return apiDoc, nil
    }
}

var pathVariableRegex = regexp.MustCompile("{(.+?)}")

//
// enrichApiDoc does some pre-processing of the API document to make them more consistent:
// 1: Creates a operation parameter for path parameter if it's missing.
//
func enrichApiDoc(doc *openapi3.Swagger) {
    // Lets make sure there are path parameters defined for the path variable bits..
    for path, v := range doc.Paths {
        for _, operation := range v.Operations() {

            definedPathParams := map[string]bool{}
            for _, param := range operation.Parameters {
                if param.Value.In == "path" {
                    definedPathParams[param.Value.Name] = true
                }
            }
            for _, match := range pathVariableRegex.FindAllStringSubmatch(path, -1) {
                name := match[1]
                if !definedPathParams[name] {
                    operation.Parameters = append(operation.Parameters, &openapi3.ParameterRef{
                        Value: &openapi3.Parameter{
                            Name:     name,
                            In:       "path",
                            Required: true,
                            Schema: &openapi3.SchemaRef{
                                Value: &openapi3.Schema{
                                    Type: "string",
                                },
                            },
                        },
                    })
                }
            }

            // Find all the
        }
    }
}

func readURL(endpointOptions EndpointOptions) ([]byte, error) {
    location, err := url.Parse(endpointOptions.URL)
    if err != nil {
        return nil, err
    }

    if location.Scheme != "" && location.Host != "" {
        client := http.Client{Transport: &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: endpointOptions.InsecureClient},
        }}
        request, err := http.NewRequest("GET", location.String(), nil)
        if err != nil {
            return nil, errors.WithStack(err)
        }
        if endpointOptions.BearerToken != "" {
            request.Header.Add("Authorization", "Bearer "+endpointOptions.BearerToken)
        }
        resp, err := client.Do(request)
        if err != nil {
            return nil, errors.WithStack(err)
        }
        defer resp.Body.Close()
        data, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            return nil, errors.WithStack(err)
        }
        return data, nil
    }
    if location.Scheme != "" || location.Host != "" || location.RawQuery != "" {
        return nil, fmt.Errorf("Unsupported URI: '%s'", location.String())
    }
    data, err := ioutil.ReadFile(location.Path)
    if err != nil {
        return nil, err
    }
    return data, nil
}
