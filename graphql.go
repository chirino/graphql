package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/chirino/graphql/errors"
	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/schema"
)

type Request struct {
	Context       context.Context `json:"-"`
	Query         string          `json:"query,omitempty"`
	OperationName string          `json:"operationName,omitempty"`
	// Variables can be set to a json.RawMessage or a map[string]interface{}
	Variables interface{} `json:"variables,omitempty"`
}

// Response represents a typical response of a GraphQL server. It may be encoded to JSON directly or
// it may be further processed to a custom response type, for example to include custom error data.
type Response struct {
	Data       json.RawMessage        `json:"data,omitempty"`
	Errors     []*errors.QueryError   `json:"errors,omitempty"`
	Extensions interface{}            `json:"extensions,omitempty"`
	Details    map[string]interface{} `json:"-"`
}

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
				return errors.Multi(err, response.Error())
			}
		}
	}
	return response.Error()
}

func (r *Request) VariablesAsMap() (map[string]interface{}, error) {
	if r.Variables == nil {
		return nil, nil
	}
	switch variables := r.Variables.(type) {
	case map[string]interface{}:
		return variables, nil
	case json.RawMessage:
		if len(variables) == 0 {
			return nil, nil
		}
		x := map[string]interface{}{}
		err := json.Unmarshal(variables, &x)
		if err != nil {
			return nil, err
		}
		return x, nil
	}
	return nil, fmt.Errorf("unsupported type: %s", reflect.TypeOf(r.Variables))
}

func (r *Request) VariablesAsJson() (json.RawMessage, error) {
	if r.Variables == nil {
		return nil, nil
	}
	switch variables := r.Variables.(type) {
	case map[string]interface{}:
		return json.Marshal(variables)
	case json.RawMessage:
		return variables, nil
	}
	return nil, fmt.Errorf("unsupported type: %s", reflect.TypeOf(r.Variables))
}

func (r *Response) Error() error {
	errs := []error{}
	for _, err := range r.Errors {
		errs = append(errs, err)
	}
	return errors.Multi(errs...)
}

func (r *Response) String() string {
	return fmt.Sprintf("{Data: %s, Errors: %v}", string(r.Data), r.Errors)
}
