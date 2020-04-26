package schema

import (
	"encoding/json"

	"github.com/chirino/graphql/internal/lexer"
	"github.com/chirino/graphql/qerrors"
)

type Ident lexer.Ident
type Description lexer.Description

func (d *Description) String() string {
	if d == nil {
		return ""
	}
	return d.Text
}

// http://facebook.github.io/graphql/draft/#InputValueDefinition
type InputValue struct {
	Name       Ident
	Type       Type
	Default    Literal
	Desc       *Description
	Loc        qerrors.Location
	TypeLoc    qerrors.Location
	Directives DirectiveList
}

type InputValueList []*InputValue

func (l InputValueList) Get(name string) *InputValue {
	for _, v := range l {
		if v.Name.Text == name {
			return v
		}
	}
	return nil
}

func ParseInputValue(l *lexer.Lexer) *InputValue {
	p := &InputValue{}
	p.Loc = l.Location()
	p.Desc = toDescription(l.ConsumeDescription())
	p.Name = Ident(l.ConsumeIdentWithLoc())
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

func toDescription(description *lexer.Description) *Description {
	if description == nil {
		return nil
	}
	d := Description(*description)
	return &d
}

type Argument struct {
	Name  Ident
	Value Literal
}

type ArgumentList []Argument

func (l ArgumentList) Get(name string) (Literal, bool) {
	for _, arg := range l {
		if arg.Name.Text == name {
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
		result[v.Name.Text] = v.Value.Evaluate(vars)
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
		name := Ident(l.ConsumeIdentWithLoc())
		l.ConsumeToken(':')
		value := ParseLiteral(l, false)
		args = append(args, Argument{Name: name, Value: value})
	}
	l.ConsumeToken(')')
	return args
}
