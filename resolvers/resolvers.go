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


type Resolver func() (reflect.Value, error)

type ResolverFactory interface {
	CreateResolver(request *ResolveRequest) Resolver
}

