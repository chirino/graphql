package schema

import "strings"

type OperationType string

const (
	InvalidOperation OperationType = ""
	Query                          = "query"
	Mutation                       = "mutation"
	Subscription                   = "subscription"
)

func GetOperationType(v string) OperationType {
	switch strings.ToLower(v) {
	case string(Query):
		return Query
	case string(Mutation):
		return Mutation
	case string(Subscription):
		return Subscription
	default:
		return InvalidOperation
	}
}
