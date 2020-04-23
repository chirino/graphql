package introspection_test

import (
	"testing"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/internal/example/starwars"
	"github.com/chirino/graphql/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSchema(t *testing.T) {
	engine := graphql.New()
	engine.Schema.Parse(starwars.Schema)
	s, err := graphql.GetSchema(engine.ServeGraphQL)
	require.NoError(t, err)

	// Lets modify the type of description so that they display the same...
	for _, sc := range engine.Schema.Types {
		var desc *schema.Description = nil
		switch sc := sc.(type) {
		case *schema.Scalar:
			desc = sc.Desc
		case *schema.Object:
			desc = sc.Desc
		case *schema.Interface:
			desc = sc.Desc
		case *schema.Union:
			desc = sc.Desc
		case *schema.Enum:
			desc = sc.Desc
		case *schema.InputObject:
			desc = sc.Desc
		}
		if desc != nil {
			desc.BlockString = false
		}
	}

	assert.Equal(t, engine.Schema.String(), s.String())
}
