package query

import (
	"fmt"
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
	Type       OperationType
	Name       schema.Ident
	Vars       schema.InputValueList
	Selections []Selection
	Directives schema.DirectiveList
	Loc        errors.Location
}

type OperationType string

const (
	Query        OperationType = "QUERY"
	Mutation                   = "MUTATION"
	Subscription               = "SUBSCRIPTION"
)

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
	l := schema.NewLexer(queryString)

	var doc *Document
	err := l.CatchSyntaxError(func() { doc = parseDocument(l) })
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func parseDocument(l *schema.Lexer) *Document {
	d := &Document{}
	l.Consume()
	for l.Peek() != scanner.EOF {
		if l.Peek() == '{' {
			op := &Operation{Type: Query, Loc: l.Location()}
			op.Selections = parseSelectionSet(l)
			d.Operations = append(d.Operations, op)
			continue
		}

		loc := l.Location()
		switch x := l.ConsumeIdent(); x {
		case "query":
			op := parseOperation(l, Query)
			op.Loc = loc
			d.Operations = append(d.Operations, op)

		case "mutation":
			d.Operations = append(d.Operations, parseOperation(l, Mutation))

		case "subscription":
			d.Operations = append(d.Operations, parseOperation(l, Subscription))

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

func parseOperation(l *schema.Lexer, opType OperationType) *Operation {
	op := &Operation{Type: opType}
	op.Name.Loc = l.Location()
	if l.Peek() == scanner.Ident {
		op.Name = l.ConsumeIdentWithLoc()
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

func parseFragment(l *schema.Lexer) *FragmentDecl {
	f := &FragmentDecl{}
	f.Name = l.ConsumeIdentWithLoc()
	l.ConsumeKeyword("on")
	f.On = schema.TypeName{Ident: l.ConsumeIdentWithLoc()}
	f.Directives = schema.ParseDirectives(l)
	f.Selections = parseSelectionSet(l)
	return f
}

func parseSelectionSet(l *schema.Lexer) []Selection {
	var sels []Selection
	l.ConsumeToken('{')
	for l.Peek() != '}' {
		sels = append(sels, parseSelection(l))
	}
	l.ConsumeToken('}')
	return sels
}

func parseSelection(l *schema.Lexer) Selection {
	if l.Peek() == '.' {
		return parseSpread(l)
	}
	return parseField(l)
}

func parseField(l *schema.Lexer) *Field {
	f := &Field{}
	f.Alias = l.ConsumeIdentWithLoc()
	f.Name = f.Alias
	if l.Peek() == ':' {
		l.ConsumeToken(':')
		f.Name = l.ConsumeIdentWithLoc()
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

func parseSpread(l *schema.Lexer) Selection {
	loc := l.Location()
	l.ConsumeToken('.')
	l.ConsumeToken('.')
	l.ConsumeToken('.')

	f := &InlineFragment{Loc: loc}
	if l.Peek() == scanner.Ident {
		ident := l.ConsumeIdentWithLoc()
		if ident.Text != "on" {
			fs := &FragmentSpread{
				Name: ident,
				Loc:  loc,
			}
			fs.Directives = schema.ParseDirectives(l)
			return fs
		}
		f.On = schema.TypeName{Ident: l.ConsumeIdentWithLoc()}
	}
	f.Directives = schema.ParseDirectives(l)
	f.Selections = parseSelectionSet(l)
	return f
}
