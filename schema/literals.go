package schema

import (
	"sort"

	"github.com/chirino/graphql/internal/lexer"
	"github.com/chirino/graphql/internal/scanner"
	uperrors "github.com/graph-gophers/graphql-go/errors"

	"io"
	"strconv"
	"strings"
)

type Literal interface {
	Evaluate(vars map[string]interface{}) interface{}
	String() string
	Location() uperrors.Location
	WriteTo(out io.StringWriter)
}

type BasicLit struct {
	Type rune
	Text string
	Loc  uperrors.Location
}

func ToLiteral(v interface{}) Literal {
	switch v := v.(type) {
	case nil:
		return &NullLit{}
	case string:
		return &BasicLit{
			Type: scanner.String,
			Text: strconv.Quote(v),
		}
	case int:
		return &BasicLit{
			Type: scanner.Int,
			Text: strconv.FormatInt(int64(v), 10),
		}
	case int32:
		return &BasicLit{
			Type: scanner.Int,
			Text: strconv.FormatInt(int64(v), 10),
		}
	case int64:
		return &BasicLit{
			Type: scanner.Int,
			Text: strconv.FormatInt(v, 10),
		}
	case uint:
		return &BasicLit{
			Type: scanner.Int,
			Text: strconv.FormatUint(uint64(v), 10),
		}
	case uint32:
		return &BasicLit{
			Type: scanner.Int,
			Text: strconv.FormatUint(uint64(v), 10),
		}
	case uint64:
		return &BasicLit{
			Type: scanner.Int,
			Text: strconv.FormatUint(v, 10),
		}
	}
	return nil
}

func (lit *BasicLit) Evaluate(vars map[string]interface{}) interface{} {
	switch lit.Type {
	case scanner.Int:
		value, err := strconv.ParseInt(lit.Text, 10, 32)
		if err != nil {
			panic(err)
		}
		return int32(value)

	case scanner.Float:
		value, err := strconv.ParseFloat(lit.Text, 64)
		if err != nil {
			panic(err)
		}
		return value

	case scanner.String:
		value, err := strconv.Unquote(lit.Text)
		if err != nil {
			panic(err)
		}
		return value

	case scanner.BlockString:
		return lit.Text[3 : len(lit.Text)-3]

	case scanner.Ident:
		switch lit.Text {
		case "true":
			return true
		case "false":
			return false
		default:
			return lit.Text
		}

	default:
		panic("invalid literal")
	}
}

func (lit *BasicLit) String() string {
	return lit.Text
}

func (lit *BasicLit) Location() uperrors.Location {
	return lit.Loc
}

type ListLit struct {
	Entries []Literal
	Loc     uperrors.Location
}

func (lit *ListLit) Evaluate(vars map[string]interface{}) interface{} {
	entries := make([]interface{}, len(lit.Entries))
	for i, entry := range lit.Entries {
		entries[i] = entry.Evaluate(vars)
	}
	return entries
}

func (lit *ListLit) String() string {
	entries := make([]string, len(lit.Entries))
	for i, entry := range lit.Entries {
		entries[i] = entry.String()
	}
	return "[" + strings.Join(entries, ", ") + "]"
}

func (lit *ListLit) Location() uperrors.Location {
	return lit.Loc
}

type ObjectLit struct {
	Fields []*ObjectLitField
	Loc    uperrors.Location
}

type ObjectLitField struct {
	Name    string
	NameLoc Location
	Value   Literal
}

func (lit *ObjectLit) Evaluate(vars map[string]interface{}) interface{} {
	fields := make(map[string]interface{}, len(lit.Fields))
	for _, f := range lit.Fields {
		fields[f.Name] = f.Value.Evaluate(vars)
	}
	return fields
}

func (lit *ObjectLit) String() string {
	entries := make([]string, 0, len(lit.Fields))
	for _, f := range lit.Fields {
		entries = append(entries, f.Name+": "+f.Value.String())
	}
	return "{" + strings.Join(entries, ", ") + "}"
}

func (lit *ObjectLit) Location() uperrors.Location {
	return lit.Loc
}

type NullLit struct {
	Loc uperrors.Location
}

func (lit *NullLit) Evaluate(vars map[string]interface{}) interface{} {
	return nil
}

func (lit *NullLit) String() string {
	return "null"
}

func (lit *NullLit) Location() uperrors.Location {
	return lit.Loc
}

type Variable struct {
	Name string
	Loc  uperrors.Location
}

func (v Variable) Evaluate(vars map[string]interface{}) interface{} {
	return vars[v.Name]
}

func (v Variable) String() string {
	return "$" + v.Name
}

func (v *Variable) Location() uperrors.Location {
	return v.Loc
}

func ParseLiteral(l *lexer.Lexer, constOnly bool) Literal {
	loc := l.Location()
	switch l.Peek() {
	case '$':
		if constOnly {
			l.SyntaxError("variable not allowed")
			panic("unreachable")
		}
		l.ConsumeToken('$')
		return &Variable{l.ConsumeIdentIntern(), loc}

	case scanner.Int, scanner.Float, scanner.String, scanner.BlockString, scanner.Ident:
		lit := ConsumeLiteral(l)
		if lit.Type == scanner.Ident && lit.Text == "null" {
			return &NullLit{loc}
		}
		lit.Loc = loc
		return lit
	case '-':
		l.ConsumeToken('-')
		lit := ConsumeLiteral(l)
		lit.Text = "-" + lit.Text
		lit.Loc = loc
		return lit
	case '[':
		l.ConsumeToken('[')
		var list []Literal
		for l.Peek() != ']' {
			list = append(list, ParseLiteral(l, constOnly))
		}
		l.ConsumeToken(']')
		return &ListLit{list, loc}

	case '{':
		l.ConsumeToken('{')
		var fields []*ObjectLitField
		for l.Peek() != '}' {
			name, loc := l.ConsumeIdentInternWithLoc()
			l.ConsumeToken(':')
			value := ParseLiteral(l, constOnly)
			fields = append(fields, &ObjectLitField{Name: name, NameLoc: loc, Value: value})
		}
		l.ConsumeToken('}')
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].Name < fields[j].Name
		})
		return &ObjectLit{fields, loc}

	default:
		peek := l.Peek()
		l.SyntaxError("invalid value: " + string(peek))
		panic("unreachable")
	}
}

func ConsumeLiteral(l *lexer.Lexer) *BasicLit {
	return &BasicLit{
		Loc:  l.Location(),
		Type: l.Peek(),
		Text: l.ConsumeLiteral(),
	}
}
