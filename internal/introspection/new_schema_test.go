package introspection_test

import (
	"testing"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/internal/example/starwars"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSchema(t *testing.T) {
	engine := graphql.New()
	engine.Schema.Parse(starwars.Schema)
	s, err := graphql.GetSchema(engine.ServeGraphQL)
	require.NoError(t, err)
	assert.Equal(t, engine.Schema.String(), s.String())
}
