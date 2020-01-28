package schema_test

import (
    "github.com/chirino/graphql/errors"
    "github.com/chirino/graphql/schema"
    "github.com/stretchr/testify/assert"
    "testing"
)

func TestConsumeDescription(t *testing.T) {
    lex := schema.NewLexer(`
# comment
"Comment line 1"
,,,,,, # Commas are insignificant
type Hello {
	world: String!
}`)
    lex.Consume()
    //err := lex.CatchSyntaxError(lex.Consume)
    //require.NoError(t, err)
    assert.Equal(t,
        &schema.Description{Text: "Comment line 1", BlockString: false, Loc: errors.Location{Line: 3, Column: 1}},
        lex.ConsumeDescription())
}

func
TestConsumeBlockDescription(t *testing.T) {
    lex := schema.NewLexer(`
"""
Comment line 1
"""
type Hello {
	world: String!
}`)
    lex.Consume()
    assert.Equal(t,
        &schema.Description{Text: "\nComment line 1\n", BlockString: true, Loc: errors.Location{Line: 2, Column: 1}},
        lex.ConsumeDescription())
}
