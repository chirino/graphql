package resolvers

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/chirino/graphql/schema"
)

type ExecutionContext interface {
	GetRoot() interface{}
	FireSubscriptionEvent(value reflect.Value)
	GetSchema() *schema.Schema
	GetContext() context.Context
	GetLimiter() *chan byte
	HandlePanic(selectionPath []string) error
	GetQuery() string
	GetDocument() *schema.QueryDocument
	GetOperation() *schema.Operation
	GetVars() map[string]interface{}
}

type ResolveRequest struct {
	Context          context.Context
	ExecutionContext ExecutionContext
	ParentResolve    *ResolveRequest
	SelectionPath    func() []string
	ParentType       schema.Type
	Parent           reflect.Value
	Field            *schema.Field
	Args             map[string]interface{}
	Selection        *schema.FieldSelection
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

// When RawMessage is the result of a Resolution, for a field, no sub field resolution performed.
type RawMessage json.RawMessage

type ValueWithContext struct {
	Value   reflect.Value
	Context context.Context
}
