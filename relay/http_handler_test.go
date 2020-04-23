package relay_test

import (
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"

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

	if w.Code != 200 {
		t.Fatalf("Expected status code 200, got %d.", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("Invalid content-type. Expected [application/json], but instead got [%s]", contentType)
	}

	expectedResponse := `{"data":{"hero":{"name":"R2-D2"}}}`
	actualResponse := w.Body.String()
	if expectedResponse != actualResponse {
		t.Fatalf("Invalid response. Expected [%s], but instead got [%s]", expectedResponse, actualResponse)
	}
}
