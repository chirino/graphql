package schema

import (
	"github.com/chirino/graphql/internal/lexer"
	"github.com/chirino/graphql/qerrors"
	uperrors "github.com/graph-gophers/graphql-go/errors"
)

func DeepestType(t Type) Type {
	switch t := (t).(type) {
	case *NonNull:
		return DeepestType(t.OfType)
	case *List:
		return DeepestType(t.OfType)
	}
	return t
}

func OfType(t Type) Type {
	switch t := (t).(type) {
	case *NonNull:
		return t.OfType
	case *List:
		return t.OfType
	default:
		return nil
	}
}

func ParseType(l *lexer.Lexer) Type {
	t := parseNullType(l)
	if l.Peek() == '!' {
		l.ConsumeToken('!')
		return &NonNull{OfType: t}
	}
	return t
}

func parseNullType(l *lexer.Lexer) Type {
	if l.Peek() == '[' {
		l.ConsumeToken('[')
		ofType := ParseType(l)
		l.ConsumeToken(']')
		return &List{OfType: ofType}
	}

	name := parseTypeName(l)
	return &name
}

type Resolver func(name string) Type

func ResolveType(t Type, resolver Resolver) (Type, *qerrors.Error) {
	switch t := t.(type) {
	case *List:
		ofType, err := ResolveType(t.OfType, resolver)
		if err != nil {
			return nil, err
		}
		return &List{OfType: ofType}, nil
	case *NonNull:
		ofType, err := ResolveType(t.OfType, resolver)
		if err != nil {
			return nil, err
		}
		return &NonNull{OfType: ofType}, nil
	case *TypeName:
		refT := resolver(t.Name)
		if refT == nil {
			err := qerrors.New("Unknown type %q.", t.Name)
			err.Rule = "KnownTypeNames"
			err.Locations = []uperrors.Location{t.NameLoc}
			return nil, err
		}
		return refT, nil
	default:
		return t, nil
	}
}
