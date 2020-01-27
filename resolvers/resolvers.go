package resolvers

import (
    "context"
    "github.com/chirino/graphql/internal/query"
    "github.com/chirino/graphql/schema"
    "reflect"
)

type ExecutionContext interface {
    GetSchema() *schema.Schema
    GetContext() context.Context
    GetLimiter() *chan byte
    HandlePanic(selectionPath []string) error
}

type ResolveRequest struct {
    Context       ExecutionContext
    ParentResolve *ResolveRequest
    SelectionPath func() []string
    ParentType    schema.Type
    Parent        reflect.Value
    Field         *schema.Field
    Args          map[string]interface{}
    Selection     *query.Field
}

type Resolution func() (reflect.Value, error)

type Resolver interface {
    Resolve(request *ResolveRequest) Resolution
}

type ResolutionFilter interface {
    Filter(request *ResolveRequest, next Resolution) Resolution
}

type noFilter byte

const NoFilter = noFilter(0)

func (noFilter) Filter(request *ResolveRequest, next Resolution) Resolution {
    return next
}
