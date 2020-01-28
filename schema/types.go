package schema

import (
    "github.com/chirino/graphql/errors"
    "github.com/chirino/graphql/internal/lexer"
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

	return &TypeName{Ident: l.ConsumeIdentWithLoc()}
}

type Resolver func(name string) Type

func ResolveType(t Type, resolver Resolver) (Type, *errors.QueryError) {
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
		refT := resolver(t.Text)
		if refT == nil {
			err := errors.Errorf("Unknown type %q.", t.Text)
			err.Rule = "KnownTypeNames"
			err.Locations = []errors.Location{t.Loc}
			return nil, err
		}
		return refT, nil
	default:
		return t, nil
	}
}
