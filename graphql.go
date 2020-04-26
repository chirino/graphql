package graphql

import (
	"context"
	"encoding/json"

	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/schema"
)

type Handler interface {
	ServeGraphQL(request *Request) *Response
}
type ServeGraphQLFunc func(request *Request) *Response

func (f ServeGraphQLFunc) ServeGraphQL(request *Request) *Response {
	return f(request)
}

func GetSchema(serveGraphQL ServeGraphQLFunc) (*schema.Schema, error) {
	json, err := GetSchemaIntrospectionJSON(serveGraphQL)
	if err != nil {
		return nil, err
	}
	return introspection.NewSchema(json)
}

func GetSchemaIntrospectionJSON(serveGraphQL ServeGraphQLFunc) ([]byte, error) {
	result := serveGraphQL(&Request{
		Query: introspection.Query,
	})
	return result.Data, result.Error()
}

func Exec(serveGraphQL ServeGraphQLFunc, ctx context.Context, result interface{}, query string, args ...interface{}) error {
	variables := map[string]interface{}{}
	for i := 0; i+1 < len(args); i += 2 {
		variables[args[i].(string)] = args[i+1]
	}
	response := serveGraphQL(&Request{
		Context:   ctx,
		Query:     query,
		Variables: variables,
	})

	if result != nil && response != nil {
		switch result := result.(type) {
		case *[]byte:
			*result = response.Data
		case *string:
			*result = string(response.Data)
		default:
			err := json.Unmarshal(response.Data, result)
			if err != nil {
				response.AddError(err)
			}
		}
	}
	return response.Error()
}
