package graphql

import (
	"encoding/json"
	"fmt"

	"github.com/chirino/graphql/qerrors"
)

// Response represents a typical response of a GraphQL server. It may be encoded to JSON directly or
// it may be further processed to a custom response type, for example to include custom error data.
type Response struct {
	Data       json.RawMessage        `json:"data,omitempty"`
	Errors     ErrorList              `json:"errors,omitempty"`
	Extensions interface{}            `json:"extensions,omitempty"`
	Details    map[string]interface{} `json:"-"`
}

func NewResponse() *Response {
	return &Response{}
}

func (r *Response) Error() error {
	return r.Errors.Error()
}

func (r *Response) String() string {
	return fmt.Sprintf("{Data: %s, Errors: %v}", string(r.Data), r.Errors)
}

func (r *Response) AddError(err error) *Response {
	r.Errors = qerrors.AppendErrors(r.Errors, err)
	return r
}
