package query

import (
	"fmt"
	"github.com/chirino/graphql/internal/lexer"
	"github.com/chirino/graphql/schema"
	"text/scanner"

	"github.com/chirino/graphql/errors"
)

type Document struct {
	Operations OperationList
	Fragments  FragmentList
}

type OperationList []*Operation

func (l OperationList) Get(name string) *Operation {
	for _, f := range l {
		if f.Name.Text == name {
			return f
		}
	}
	return nil
}

type FragmentList []*FragmentDecl

func (l FragmentList) Get(name string) *FragmentDecl {
	for _, f := range l {
		if f.Name.Text == name {
			return f
		}
	}
	return nil
}

type Operation struct {
	Type       schema.OperationType
	Name       schema.Ident
	Vars       schema.InputValueList
	Selections []Selection
	Directives schema.DirectiveList
	Loc        errors.Location
}

type Fragment struct {
	On         schema.TypeName
	Selections []Selection
}

type FragmentDecl struct {
	Fragment
	Name       schema.Ident
	Directives schema.DirectiveList
	Loc        errors.Location
}

type Selection interface {
	isSelection()
}

type Field struct {
	Alias           schema.Ident
	Name            schema.Ident
	Arguments       schema.ArgumentList
	Directives      schema.DirectiveList
	Selections      []Selection
	SelectionSetLoc errors.Location
	Schema          *FieldSchema
}

type FieldSchema struct {
	Field  *schema.Field
	Parent schema.NamedType
}

type InlineFragment struct {
	Fragment
	Directives schema.DirectiveList
	Loc        errors.Location
}

type FragmentSpread struct {
	Name       schema.Ident
	Directives schema.DirectiveList
	Loc        errors.Location
}

func (Field) isSelection()          {}
func (InlineFragment) isSelection() {}
func (FragmentSpread) isSelection() {}

func Parse(queryString string) (*Document, *errors.QueryError) {
	l := lexer.NewLexer(queryString)

	var doc *Document
	err := l.CatchSyntaxError(func() { doc = parseDocument(l) })
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func parseDocument(l *lexer.Lexer) *Document {
	d := &Document{}
	l.Consume()
	for l.Peek() != scanner.EOF {
		if l.Peek() == '{' {
			op := &Operation{Type: schema.Query, Loc: l.Location()}
			op.Selections = parseSelectionSet(l)
			d.Operations = append(d.Operations, op)
			continue
		}

		loc := l.Location()
		switch x := l.ConsumeIdent(); x {
		case "query":
			op := parseOperation(l, schema.Query)
			op.Loc = loc
			d.Operations = append(d.Operations, op)

		case "mutation":
			d.Operations = append(d.Operations, parseOperation(l, schema.Mutation))

		case "subscription":
			d.Operations = append(d.Operations, parseOperation(l, schema.Subscription))

		case "fragment":
			frag := parseFragment(l)
			frag.Loc = loc
			d.Fragments = append(d.Fragments, frag)

		default:
			l.SyntaxError(fmt.Sprintf(`unexpected %q, expecting "fragment"`, x))
		}
	}
	return d
}

func parseOperation(l *lexer.Lexer, opType schema.OperationType) *Operation {
	op := &Operation{Type: opType}
	op.Name.Loc = l.Location()
	if l.Peek() == scanner.Ident {
		op.Name = schema.Ident(l.ConsumeIdentWithLoc())
	}
	op.Directives = schema.ParseDirectives(l)
	if l.Peek() == '(' {
		l.ConsumeToken('(')
		for l.Peek() != ')' {
			loc := l.Location()
			l.ConsumeToken('$')
			iv := schema.ParseInputValue(l)
			iv.Loc = loc
			op.Vars = append(op.Vars, iv)
		}
		l.ConsumeToken(')')
	}
	op.Selections = parseSelectionSet(l)
	return op
}

func parseFragment(l *lexer.Lexer) *FragmentDecl {
	f := &FragmentDecl{}
	f.Name = schema.Ident(l.ConsumeIdentWithLoc())
	l.ConsumeKeyword("on")
	f.On = schema.TypeName{Ident: schema.Ident(l.ConsumeIdentWithLoc())}
	f.Directives = schema.ParseDirectives(l)
	f.Selections = parseSelectionSet(l)
	return f
}

func parseSelectionSet(l *lexer.Lexer) []Selection {
	var sels []Selection
	l.ConsumeToken('{')
	for l.Peek() != '}' {
		sels = append(sels, parseSelection(l))
	}
	l.ConsumeToken('}')
	return sels
}

func parseSelection(l *lexer.Lexer) Selection {
	if l.Peek() == '.' {
		return parseSpread(l)
	}
	return parseField(l)
}

func parseField(l *lexer.Lexer) *Field {
	f := &Field{}
	f.Alias = schema.Ident(l.ConsumeIdentWithLoc())
	f.Name = f.Alias
	if l.Peek() == ':' {
		l.ConsumeToken(':')
		f.Name = schema.Ident(l.ConsumeIdentWithLoc())
	}
	if l.Peek() == '(' {
		f.Arguments = schema.ParseArguments(l)
	}
	f.Directives = schema.ParseDirectives(l)
	if l.Peek() == '{' {
		f.SelectionSetLoc = l.Location()
		f.Selections = parseSelectionSet(l)
	}
	return f
}

func parseSpread(l *lexer.Lexer) Selection {
	loc := l.Location()
	l.ConsumeToken('.')
	l.ConsumeToken('.')
	l.ConsumeToken('.')

	f := &InlineFragment{Loc: loc}
	if l.Peek() == scanner.Ident {
		ident := schema.Ident(l.ConsumeIdentWithLoc())
		if ident.Text != "on" {
			fs := &FragmentSpread{
				Name: ident,
				Loc:  loc,
			}
			fs.Directives = schema.ParseDirectives(l)
			return fs
		}
		f.On = schema.TypeName{Ident: schema.Ident(l.ConsumeIdentWithLoc())}
	}
	f.Directives = schema.ParseDirectives(l)
	f.Selections = parseSelectionSet(l)
	return f
}
