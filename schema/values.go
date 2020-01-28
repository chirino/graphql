package schema

import (
    "github.com/chirino/graphql/errors"
    "github.com/chirino/graphql/internal/lexer"
)

// http://facebook.github.io/graphql/draft/#InputValueDefinition
type InputValue struct {
    Name    lexer.Ident
    Type    Type
    Default Literal
    Desc    *lexer.Description
    Loc     errors.Location
    TypeLoc    errors.Location
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
    p.Desc = l.ConsumeDescription()
    p.Name = l.ConsumeIdentWithLoc()
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
    Name  lexer.Ident
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

func ParseArguments(l *lexer.Lexer) ArgumentList {
    var args ArgumentList
    l.ConsumeToken('(')
    for l.Peek() != ')' {
        name := l.ConsumeIdentWithLoc()
        l.ConsumeToken(':')
        value := ParseLiteral(l, false)
        args = append(args, Argument{Name: name, Value: value})
    }
    l.ConsumeToken(')')
    return args
}
