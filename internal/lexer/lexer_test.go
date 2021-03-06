package lexer_test

import (
	"testing"

	"github.com/chirino/graphql/internal/lexer"
	"github.com/stretchr/testify/assert"
)

func TestConsumeDescription(t *testing.T) {
	lex := lexer.Get(`
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
		lexer.Description{Text: "Comment line 1", ShowType: lexer.ShowStringDescription, Loc: lexer.Location{Line: 3, Column: 1}},
		lex.ConsumeDescription())
}

func TestConsumeBlockDescription(t *testing.T) {
	lex := lexer.Get(`
"""
Comment line 1
"""
type Hello {
	world: String!
}`)
	lex.Consume()
	assert.Equal(t,
		lexer.Description{Text: "\nComment line 1\n", ShowType: lexer.ShowBlockDescription, Loc: lexer.Location{Line: 2, Column: 1}},
		lex.ConsumeDescription())
}
