package customtypes

import (
	"errors"
	"strconv"

	graphql "github.com/graph-gophers/graphql-go"
)

// ID represents GraphQL's "ID" scalar type. A custom type may be used instead.
type ID graphql.ID

func (id *ID) UnmarshalGraphQL(input interface{}) error {
	var err error
	switch input := input.(type) {
	case string:
		*id = ID(input)
	case int32:
		*id = ID(strconv.Itoa(int(input)))
	default:
		err = errors.New("wrong type")
	}
	return err
}
