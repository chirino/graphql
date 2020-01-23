package api

import (
    "encoding/json"
    "fmt"
    "github.com/getkin/kin-openapi/openapi2"
    "github.com/getkin/kin-openapi/openapi2conv"
    "github.com/getkin/kin-openapi/openapi3"
    "github.com/pkg/errors"
    "io/ioutil"
    "net/http"
    "net/url"
)

func LoadOpenApiDoc(docLocation string) (*openapi3.Swagger, error) {
    path, err := url.Parse(docLocation)
    if err != nil {
        return nil, errors.Wrap(err, "invalid URI path to openapi document")
    }

    data, err := readUrl(path)

    // Lets detect if it's openapi 2 or 3
    doc := struct {
        Swagger string `json:"swagger,omitempty"`
        OpenAPI string `json:"openapi,omitempty"`
    }{}

    err = json.Unmarshal(data, &doc)
    if err != nil {
        return nil, errors.Wrap(err, "could not detect openapi version")
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

        err = openapi3.NewSwaggerLoader().ResolveRefsIn(apiDoc, path)
        if err != nil {
            return nil, errors.WithStack(err)
        }

        return apiDoc, err
    } else {
        // It should be a v3 document already..
        apiDoc, err := openapi3.NewSwaggerLoader().LoadSwaggerFromDataWithPath(data, path)
        if err != nil {
            return nil, errors.WithStack(err)
        }
        return apiDoc, err
    }
}

func readUrl(location *url.URL) ([]byte, error) {
    if location.Scheme != "" && location.Host != "" {
        resp, err := http.Get(location.String())
        if err != nil {
            return nil, err
        }
        data, err := ioutil.ReadAll(resp.Body)
        defer resp.Body.Close()
        if err != nil {
            return nil, err
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
