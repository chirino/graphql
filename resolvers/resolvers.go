package resolvers

import (
	"context"
	"github.com/chirino/graphql/internal/query"
	"github.com/chirino/graphql/schema"
	"reflect"
)

type ExecutionContext interface {
	FireSubscriptionEvent(value reflect.Value)
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
	// Resolve allows you to inspect a ResolveRequest to see if your resolver can resolve it.
	// If you can resolve it, return a new Resolution that computes the value of the field.
	// If you don't know how to resolve that request, return next.
	//
	// The next variable hold the Resolution of the previous resolver, this allows your resolver
	// to filter it's results.  next may be nil if no resolution has been found yet.
	Resolve(request *ResolveRequest, next Resolution) Resolution
}
