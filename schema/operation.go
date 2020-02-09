package schema

type OperationType string

const (
    Query        OperationType = "query"
    Mutation                   = "mutation"
    Subscription               = "subscription"
)

