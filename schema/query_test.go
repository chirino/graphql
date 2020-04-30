package schema_test

import (
	"testing"

	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoLossOnQueryFormating(t *testing.T) {
	q1 := &schema.QueryDocument{}
	err := q1.Parse(introspection.Query)
	require.NoError(t, err)

	q2 := &schema.QueryDocument{}
	err = q2.Parse(q1.String())
	require.NoError(t, err)

	assert.Equal(t, q1.String(), q2.String())
}
