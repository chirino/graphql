package schema

import (
	"encoding/json"

	"github.com/chirino/graphql/internal/lexer"
	uperrors "github.com/graph-gophers/graphql-go/errors"
)

// http://facebook.github.io/graphql/draft/#InputValueDefinition
type InputValue struct {
	Loc        uperrors.Location
	Name       string
	NameLoc    Location
	Type       Type
	TypeLoc    uperrors.Location
	Default    Literal
	Desc       Description
	Directives DirectiveList
}

type InputValueList []*InputValue

func (l InputValueList) Get(name string) *InputValue {
	for _, v := range l {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func ParseInputValue(l *lexer.Lexer) *InputValue {
	p := &InputValue{}
	p.Loc = l.Location()
	p.Desc = l.ConsumeDescription()
	p.Name, p.NameLoc = l.ConsumeIdentInternWithLoc()
	l.ConsumeToken(':')
	p.TypeLoc = l.Location()
	p.Type = ParseType(l)
	if l.Peek() == '=' {
		l.ConsumeToken('=')
		p.Default = ParseLiteral(l, true)
	}
	p.Directives = ParseDirectives(l)
	return p
}

type Argument struct {
	Name    string
	NameLoc Location
	Value   Literal
}

type ArgumentList []Argument

func (l ArgumentList) Get(name string) (Literal, bool) {
	for _, arg := range l {
		if arg.Name == name {
			return arg.Value, true
		}
	}
	return nil, false
}

func (l ArgumentList) MustGet(name string) Literal {
	value, ok := l.Get(name)
	if !ok {
		panic("argument not found")
	}
	return value
}

func (l ArgumentList) Value(vars map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(l))
	for _, v := range l {
		result[v.Name] = v.Value.Evaluate(vars)
	}
	return result
}

// ScanValue stores the values of the argument list
// in the value pointed to by v. If v is nil or not a pointer,
// ScanValue returns an error.
func (l ArgumentList) ScanValue(vars map[string]interface{}, v interface{}) error {
	value := l.Value(vars)
	marshaled, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(marshaled, v)
}

func ParseArguments(l *lexer.Lexer) ArgumentList {
	var args ArgumentList
	l.ConsumeToken('(')
	for l.Peek() != ')' {
		arg := Argument{}
		arg.Name, arg.NameLoc = l.ConsumeIdentInternWithLoc()
		l.ConsumeToken(':')
		arg.Value = ParseLiteral(l, false)
		args = append(args, arg)
	}
	l.ConsumeToken(')')
	return args
}
