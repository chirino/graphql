package relay_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/internal/example/starwars"
	"github.com/chirino/graphql/relay"
)

func TestEngineAPIServeHTTP(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/some/path/here", strings.NewReader(`{"query":"{ hero { name } }", "operationName":"", "variables": null}`))

	engine := graphql.New()
	err := engine.Schema.Parse(starwars.Schema)

	require.NoError(t, err)
	engine.Root = &starwars.Resolver{}
	h := relay.Handler{ServeGraphQLStream: engine.ServeGraphQLStream}

	h.ServeHTTP(w, r)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, `{"data":{"hero":{"name":"R2-D2"}}}
`, w.Body.String())
}

func TestClientServeGraphQL(t *testing.T) {

	s := httptest.NewServer(&relay.Handler{
		ServeGraphQL: func(request *graphql.Request) *graphql.Response {
			return &graphql.Response{
				Data: json.RawMessage(`{"hello":"world"}`),
			}
		},
	})
	defer s.Close()
	client := relay.NewClient(s.URL)
	response := client.ServeGraphQL(&graphql.Request{Query: "{hello}"})
	assert.Equal(t, `{"hello":"world"}`, string(response.Data))
}

type testStream struct {
	responses chan *graphql.Response
}

func (t testStream) Close() {
}
func (t testStream) Responses() <-chan *graphql.Response {
	return t.responses
}

func TestClientServeGraphQLStream(t *testing.T) {

	s := httptest.NewServer(&relay.Handler{
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
	client := relay.NewClient(s.URL)
	rs := client.ServeGraphQLStream(&graphql.Request{Query: "{hello}"})
	response := <-rs
	assert.Equal(t, `{"hello":"world"}`, string(response.Data))
	response = <-rs
	assert.Equal(t, `{"bye":"world"}`, string(response.Data))
	response = <-rs
	assert.Nil(t, response)
}
