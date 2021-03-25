package schema

import (
	"fmt"
	"sync"
	"text/scanner"

	"github.com/chirino/graphql/internal/lexer"
	uperrors "github.com/graph-gophers/graphql-go/errors"
)

type QueryDocument struct {
	Operations OperationList
	Fragments  FragmentList
}

type OperationList []*Operation

func (l OperationList) Get(name string) *Operation {
	for _, f := range l {
		if f.Name == name {
			return f
		}
	}
	return nil
}

type FragmentList []*FragmentDecl

func (l FragmentList) Get(name string) *FragmentDecl {
	for _, f := range l {
		if f.Name == name {
			return f
		}
	}
	return nil
}

type SelectionList []Selection

type Operation struct {
	Type       OperationType
	Name       string
	NameLoc    Location
	Vars       InputValueList
	Selections SelectionList
	Directives DirectiveList
	Loc        uperrors.Location
}

type Fragment struct {
	On         TypeName
	Selections SelectionList
}

type FragmentDecl struct {
	Fragment
	Name       string
	NameLoc    Location
	Directives DirectiveList
	Loc        uperrors.Location
}

type Selection interface {
	Formatter
	GetSelections(doc *QueryDocument) SelectionList
	SetSelections(doc *QueryDocument, v SelectionList)
	Location() uperrors.Location
}

type FieldSelection struct {
	Alias           string
	AliasLoc        Location
	Name            string
	NameLoc         Location
	Arguments       ArgumentList
	Directives      DirectiveList
	Selections      SelectionList
	SelectionSetLoc uperrors.Location
	Schema          *FieldSchema
	Extension       interface{}
}

type FieldSchema struct {
	Field  *Field
	Parent NamedType
}

type InlineFragment struct {
	Fragment
	Directives DirectiveList
	Loc        uperrors.Location
}

type FragmentSpread struct {
	Name       string
	NameLoc    Location
	Directives DirectiveList
	Loc        uperrors.Location
}

func (t *Operation) SetSelections(doc *QueryDocument, v SelectionList)      { t.Selections = v }
func (t *FieldSelection) SetSelections(doc *QueryDocument, v SelectionList) { t.Selections = v }
func (t *InlineFragment) SetSelections(doc *QueryDocument, v SelectionList) { t.Selections = v }
func (t *FragmentSpread) SetSelections(doc *QueryDocument, v SelectionList) {
	frag := doc.Fragments.Get(t.Name)
	if frag == nil {
		return
	}
	frag.Selections = v
}

func (t Operation) GetSelections(doc *QueryDocument) SelectionList      { return t.Selections }
func (t FieldSelection) GetSelections(doc *QueryDocument) SelectionList { return t.Selections }
func (t InlineFragment) GetSelections(doc *QueryDocument) SelectionList { return t.Selections }
func (t FragmentSpread) GetSelections(doc *QueryDocument) SelectionList {
	frag := doc.Fragments.Get(t.Name)
	if frag == nil {
		return nil
	}
	return frag.Selections
}

func (t Operation) Location() uperrors.Location      { return t.Loc }
func (t FieldSelection) Location() uperrors.Location { return t.NameLoc }
func (t InlineFragment) Location() uperrors.Location { return t.Loc }
func (t FragmentSpread) Location() uperrors.Location { return t.Loc }

func (doc *QueryDocument) ParseWithDescriptions(queryString string) error {
	l := lexer.Get(queryString)
	err := l.CatchSyntaxError(func() { parseDocument(l, doc) })
	lexer.Put(l)
	return err
}

func (doc *QueryDocument) Parse(queryString string) error {
	l := lexer.Get(queryString)
	l.SkipDescriptions = true
	err := l.CatchSyntaxError(func() { parseDocument(l, doc) })
	lexer.Put(l)
	return err
}

func parseDocument(l *lexer.Lexer, d *QueryDocument) {
	l.Consume()
	for l.Peek() != scanner.EOF {
		if l.Peek() == '{' {
			op := &Operation{Type: Query, Loc: l.Location()}
			op.Selections = parseSelectionSet(l)
			d.Operations = append(d.Operations, op)
			continue
		}

		loc := l.Location()
		switch x := l.ConsumeIdentIntern(); x {
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
}

func parseOperation(l *lexer.Lexer, opType OperationType) *Operation {
	op := &Operation{Type: opType}
	op.Loc = l.Location()
	if l.Peek() == scanner.Ident {
		op.Name, op.NameLoc = l.ConsumeIdentInternWithLoc()
	}
	op.Directives = ParseDirectives(l)
	if l.Peek() == '(' {
		l.ConsumeToken('(')
		for l.Peek() != ')' {
			loc := l.Location()
			l.ConsumeToken('$')
			iv := ParseInputValue(l)
			iv.Name = "$" + iv.Name
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
	f.Name, f.NameLoc = l.ConsumeIdentInternWithLoc()
	l.ConsumeKeyword("on")
	f.On = parseTypeName(l)
	f.Directives = ParseDirectives(l)
	f.Selections = parseSelectionSet(l)
	return f
}

func parseSelectionSet(l *lexer.Lexer) SelectionList {
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

var fieldPool = sync.Pool{
	New: func() interface{} { return new(FieldSelection) },
}

func returnFieldsToPool(sl SelectionList) {
	for _, s := range sl {
		switch s := s.(type) {
		case *FieldSelection:
			returnFieldsToPool(s.Selections)
			*s = FieldSelection{}
			fieldPool.Put(s)
		case *InlineFragment:
			returnFieldsToPool(s.Selections)
		}
	}
}

func (d *QueryDocument) Close() {
	for _, o := range d.Operations {
		returnFieldsToPool(o.Selections)
	}
	d.Operations = OperationList{}
	for _, f := range d.Fragments {
		returnFieldsToPool(f.Selections)
	}
	d.Fragments = FragmentList{}
}

func parseField(l *lexer.Lexer) *FieldSelection {
	f := fieldPool.Get().(*FieldSelection)
	f.Alias, f.AliasLoc = l.ConsumeIdentInternWithLoc()
	f.Name = f.Alias
	if l.Peek() == ':' {
		l.ConsumeToken(':')
		f.Name, f.NameLoc = l.ConsumeIdentInternWithLoc()
	}
	if l.Peek() == '(' {
		f.Arguments = ParseArguments(l)
	}
	f.Directives = ParseDirectives(l)
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
		ident, identLoc := l.ConsumeIdentInternWithLoc()
		if ident != "on" {
			fs := &FragmentSpread{
				Name:    ident,
				NameLoc: identLoc,
				Loc:     loc,
			}
			fs.Directives = ParseDirectives(l)
			return fs
		}
		f.On = parseTypeName(l)
	}
	f.Directives = ParseDirectives(l)
	f.Selections = parseSelectionSet(l)
	return f
}

func (d *QueryDocument) GetOperation(operationName string) (*Operation, error) {
	if len(d.Operations) == 0 {
		return nil, fmt.Errorf("no operations in query document")
	}

	if operationName == "" {
		if len(d.Operations) > 1 {
			return nil, fmt.Errorf("more than one operation in query document and no operation name given")
		}
		for _, op := range d.Operations {
			return op, nil // return the one and only operation
		}
	}

	op := d.Operations.Get(operationName)
	if op == nil {
		return nil, fmt.Errorf("no operation with name %q", operationName)
	}
	return op, nil
}
