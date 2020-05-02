package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

type Request struct {
	// Optional Context to use on the request.  Required if caller wants to cancel long-running \
	// operations like subscriptions.
	Context context.Context `json:"-"`
	// Query is the graphql query document to execute
	Query string `json:"query,omitempty"`
	// OperationName is the name of the operation in the query document to execute
	OperationName string `json:"operationName,omitempty"`
	// Variables can be set to a json.RawMessage or a map[string]interface{}
	Variables interface{} `json:"variables,omitempty"`
}

func (r Request) GetContext() (ctx context.Context) {
	ctx = r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return
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
