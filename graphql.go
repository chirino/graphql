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

type EngineRequest struct {
	Context       context.Context `json:"-"`
	Query         string          `json:"query,omitempty"`
	OperationName string          `json:"operationName,omitempty"`
	// Variables can be set to a json.RawMessage or a map[string]interface{}
	Variables interface{} `json:"variables,omitempty"`
}

// Response represents a typical response of a GraphQL server. It may be encoded to JSON directly or
// it may be further processed to a custom response type, for example to include custom error data.
type EngineResponse struct {
	Data       json.RawMessage      `json:"data,omitempty"`
	Errors     []*errors.QueryError `json:"errors,omitempty"`
	Extensions interface{}          `json:"extensions,omitempty"`
}

type ResponseStream struct {
	Cancel          context.CancelFunc
	Responses       chan *EngineResponse
	IsSubscription  bool
	ResponseCounter int
}

type StandardAPI func(request *EngineRequest) *EngineResponse
type StreamingAPI func(request *EngineRequest) (*ResponseStream, error)

func GetSchema(api StandardAPI) (*schema.Schema, error) {
	resp := api(&EngineRequest{
		Query: introspection.Query,
	})
	if resp.Error() != nil {
		return nil, resp.Error()
	}
	return introspection.NewSchema(resp.Data)
}

func (r *EngineRequest) UnmarshalVariables() (map[string]interface{}, error) {
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

func (r *EngineResponse) Error() error {
	errs := []error{}
	for _, err := range r.Errors {
		errs = append(errs, err)
	}
	return errors.Multi(errs...)
}

func (r *EngineResponse) String() string {
	return fmt.Sprintf("{Data: %s, Errors: %v}", string(r.Data), r.Errors)
}

func (qr *ResponseStream) Next() *EngineResponse {
	if !qr.IsSubscription && qr.ResponseCounter > 0 {
		return nil
	}
	response := <-qr.Responses
	if response != nil {
		qr.ResponseCounter += 1
	}
	return response
}

func (qr *ResponseStream) Close() {
	close(qr.Responses)
	qr.Cancel()
}
