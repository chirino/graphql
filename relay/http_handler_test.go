package relay_test

import (
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
