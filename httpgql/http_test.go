package httpgql_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chirino/graphql/httpgql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/internal/example/starwars"
)

func TestEngineAPIServeHTTP(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/some/path/here", strings.NewReader(`{"query":"{ hero { name } }", "operationName":"", "variables": null}`))

	engine := graphql.New()
	err := engine.Schema.Parse(starwars.Schema)

	require.NoError(t, err)
	engine.Root = &starwars.Resolver{}
	h := httpgql.Handler{ServeGraphQLStream: engine.ServeGraphQLStream}

	h.ServeHTTP(w, r)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, `{"data":{"hero":{"name":"R2-D2"}}}
`, w.Body.String())
}

func TestClientServeGraphQL(t *testing.T) {

	s := httptest.NewServer(&httpgql.Handler{
		ServeGraphQL: func(request *graphql.Request) *graphql.Response {
			return &graphql.Response{
				Data: json.RawMessage(`{"hello":"world"}`),
			}
		},
	})
	defer s.Close()
	client := httpgql.NewClient(s.URL)
	response := client.ServeGraphQL(&graphql.Request{Query: "{hello}"})
	assert.Equal(t, `{"hello":"world"}`, string(response.Data))
}
