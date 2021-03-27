package customtypes

import (
	"fmt"
	"time"

	graphql "github.com/graph-gophers/graphql-go"
)

type Time graphql.Time

// UnmarshalGraphQL is a custom unmarshaler for Time
//
// This function will be called whenever you use the
// time scalar as an input
func (t *Time) UnmarshalGraphQL(input interface{}) error {
	switch input := input.(type) {
	case time.Time:
		t.Time = input
		return nil
	case string:
		var err error
		t.Time, err = time.Parse(time.RFC3339, input)
		return err
	case int:
		t.Time = time.Unix(int64(input), 0)
		return nil
	case float64:
		t.Time = time.Unix(int64(input), 0)
		return nil
	default:
		return fmt.Errorf("wrong type")
	}
}
