package httpgql_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/httpgql"
	"github.com/stretchr/testify/assert"
)

type testStream struct {
	responses chan *graphql.Response
}

func (t testStream) Close() {
}
func (t testStream) Responses() <-chan *graphql.Response {
	return t.responses
}

func TestClientServeGraphQLStream(t *testing.T) {

	s := httptest.NewServer(&httpgql.Handler{
		ServeGraphQLStream: func(request *graphql.Request) graphql.ResponseStream {
			result := make(chan *graphql.Response, 2)
			result <- &graphql.Response{
				Data: json.RawMessage(`{"hello":"world"}`),
			}
			result <- &graphql.Response{
				Data: json.RawMessage(`{"bye":"world"}`),
			}
			close(result)
			return result
		},
	})

	defer s.Close()
	client := httpgql.NewClient(s.URL)
	rs := client.ServeGraphQLStream(&graphql.Request{Query: "{hello}"})
	response := <-rs
	assert.Equal(t, `{"hello":"world"}`, string(response.Data))
	response = <-rs
	assert.Equal(t, `{"bye":"world"}`, string(response.Data))
	response = <-rs
	assert.Nil(t, response)
}
