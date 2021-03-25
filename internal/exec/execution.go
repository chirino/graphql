package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/chirino/graphql/exec"
	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/internal/linkedmap"
	"github.com/chirino/graphql/log"
	"github.com/chirino/graphql/qerrors"
	"github.com/chirino/graphql/resolvers"
	"github.com/chirino/graphql/schema"
	"github.com/chirino/graphql/trace"
	uperrors "github.com/graph-gophers/graphql-go/errors"
)

type Execution struct {
	Query     string
	Vars      map[string]interface{}
	Schema    *schema.Schema
	Doc       *schema.QueryDocument
	Operation *schema.Operation
	limiter   chan byte
	Tracer    trace.Tracer
	Logger    log.Logger
	Root      interface{}
	VarTypes  map[string]*introspection.Type
	Context   context.Context
	Resolver  resolvers.Resolver
	Mu        sync.Mutex

	rootFields     *linkedmap.LinkedMap
	data           *bytes.Buffer
	errs           qerrors.ErrorList
	MaxParallelism int

	subMu                     sync.Mutex
	FireSubscriptionEventFunc func(d json.RawMessage, e qerrors.ErrorList)
	FireSubscriptionCloseFunc func()
	TryCast                   func(value reflect.Value, toType string) (v reflect.Value, ok bool)
}

func (this *Execution) GetRoot() interface{} {
	return this.Root
}

func (this *Execution) GetQuery() string {
	return this.Query
}

func (this *Execution) GetVars() map[string]interface{} {
	return this.Vars
}

func (this *Execution) GetDocument() *schema.QueryDocument {
	return this.Doc
}

func (this *Execution) GetOperation() *schema.Operation {
	return this.Operation
}

func (this *Execution) GetSchema() *schema.Schema {
	return this.Schema
}

func (this *Execution) GetContext() context.Context {
	return this.Context
}
func (this *Execution) GetLimiter() *chan byte {
	return &this.limiter
}
func (this *Execution) HandlePanic(path []string) error {
	if value := recover(); value != nil {
		this.Logger.LogPanic(this.Context, value)
		err := makePanicError(value)
		err.PathStr = path
		return err
	}
	return nil
}

func makePanicError(value interface{}) *qerrors.Error {
	return qerrors.New("graphql: panic occurred: %v", value)
}

type ExecutionResult struct {
	Errors qerrors.ErrorList
	Data   *bytes.Buffer
}

type SelectionResolver struct {
	parent     *SelectionResolver
	field      *schema.FieldSelection
	Resolution resolvers.Resolution
	selections []schema.Selection
}

func (this *SelectionResolver) Path() []string {
	if this == nil {
		return []string{}
	}
	if this.parent == nil {
		return []string{this.field.Alias}
	}
	return append(this.parent.Path(), this.field.Alias)
}

func (this *Execution) resolveFields(ctx context.Context, parentSelectionResolver *SelectionResolver, selectionResolvers *linkedmap.LinkedMap, parentValue reflect.Value, parentType schema.Type, selections []schema.Selection) {
	for _, selection := range selections {
		switch field := selection.(type) {
		case *schema.FieldSelection:
			if this.skipByDirective(field.Directives) {
				continue
			}
			if field.Schema == nil {
				continue
			}

			var sr *SelectionResolver = nil
			x := selectionResolvers.Get(field.Alias)
			if x != nil {
				sr = x.(*SelectionResolver)
			} else {
				sr = &SelectionResolver{}
				sr.field = field
				sr.parent = parentSelectionResolver
			}

			if sr.Resolution == nil {

				// This field has not been resolved yet..
				sr.selections = field.Selections

				typeName := parentType
				evaluatedArguments := make(map[string]interface{}, len(field.Arguments))
				for _, arg := range field.Arguments {
					evaluatedArguments[arg.Name] = arg.Value.Evaluate(this.Vars)
				}
				resolveRequest := &resolvers.ResolveRequest{
					Context:          ctx,
					ExecutionContext: this,
					ParentType:       typeName,
					Parent:           parentValue,
					Field:            field.Schema.Field,
					Args:             evaluatedArguments,
					Selection:        field,
					SelectionPath:    sr.Path,
				}
				resolution := this.Resolver.Resolve(resolveRequest, nil)

				if resolution == nil {
					this.AddError((&qerrors.Error{
						QueryError: &uperrors.QueryError {
							Message: "No resolver found",
						},
						PathStr:    append(parentSelectionResolver.Path(), field.Alias),
					}).WithStack())
				} else {
					sr.Resolution = resolution
					selectionResolvers.Set(field.Alias, sr)
				}
			} else {
				// field previously resolved, but fragment is adding more child field selections.
				sr.selections = append(sr.selections, field.Selections...)
			}

		case *schema.InlineFragment:
			if this.skipByDirective(field.Directives) {
				continue
			}

			fragment := &field.Fragment
			this.CreateSelectionResolversForFragment(ctx, parentSelectionResolver, fragment, parentType, parentValue, selectionResolvers)

		case *schema.FragmentSpread:
			if this.skipByDirective(field.Directives) {
				continue
			}
			fragment := &this.Doc.Fragments.Get(field.Name).Fragment
			this.CreateSelectionResolversForFragment(ctx, parentSelectionResolver, fragment, parentType, parentValue, selectionResolvers)
		}
	}
}

func (this *Execution) CreateSelectionResolversForFragment(ctx context.Context, parentSelectionResolver *SelectionResolver, fragment *schema.Fragment, parentType schema.Type, parentValue reflect.Value, selectionResolvers *linkedmap.LinkedMap) {
	if fragment.On.Name != "" && fragment.On.Name != parentType.String() {
		castType := this.Schema.Types[fragment.On.Name]
		if casted, ok := this.TryCast(parentValue, fragment.On.Name); ok {
			this.resolveFields(ctx, parentSelectionResolver, selectionResolvers, casted, castType, fragment.Selections)
		}
	} else {
		this.resolveFields(ctx, parentSelectionResolver, selectionResolvers, parentValue, parentType, fragment.Selections)
	}
}

func (this *Execution) Execute() error {

	rootType := this.Schema.EntryPoints[this.Operation.Type]
	rootValue := reflect.ValueOf(this.Root)
	rootFields := linkedmap.CreateLinkedMap(len(this.Operation.Selections))

	// async processing can start when the field is selected... apply limit here...
	this.errs = qerrors.ErrorList{}
	this.limiter = make(chan byte, this.MaxParallelism)
	this.limiter <- 1
	this.resolveFields(this.Context, nil, rootFields, rootValue, rootType, this.Operation.Selections)

	if this.Operation.Type == schema.Subscription {

		// subs are kinda special... we need to setup a subscription to an event,
		// and execute it every time the event is triggered...
		if len(this.Operation.Selections) != 1 {
			return qerrors.New("you can only select 1 field in a subscription")
		}

		if len(rootFields.ValuesByKey) != 1 {
			// We may not have resolved it correctly..
			return this.errs.Error()
		}
		// This code path interact closely with with FireSubscriptionEvent method.
		this.rootFields = rootFields
		selected := rootFields.First.Value.(*SelectionResolver)
		_, err := selected.Resolution() // This should start go routines to fire events via FireSubscriptionEvent
		if err != nil {
			return err
		}
		return nil

	} else {

		// This is the first execution goroutine.
		// TODO: this may need to move for the subscription case..
		this.data = &bytes.Buffer{}
		defer func() { <-this.limiter }()
		this.recursiveExecute(this.Context, nil, rootFields)
		this.FireSubscriptionEventFunc(json.RawMessage(this.data.Bytes()), this.errs)
		this.FireSubscriptionClose()
		return nil
	}
}

// This is called by subscription resolvers to send out a subscription event.
func (this *Execution) FireSubscriptionEvent(value reflect.Value, err error) {
	if this.rootFields == nil {
		panic("the FireSubscriptionEvent method should only be called when triggering events for subscription fields")
	}

	// Protect against a resolver firing concurrent events at us.. we only will process one at
	// at time.
	this.subMu.Lock()
	defer this.subMu.Unlock()

	// this.FireSubscriptionEventFunc is nil once the sub has been closed by the resolver.
	if this.FireSubscriptionEventFunc == nil {
		return
	}

	// Change the resolution function to that it returns the fired event
	selected := this.rootFields.First.Value.(*SelectionResolver)
	selected.Resolution = func() (reflect.Value, error) {
		return value, nil
	}
	this.data = &bytes.Buffer{}
	this.errs = qerrors.ErrorList{}

	if err != nil {
		this.errs = qerrors.AppendErrors(this.errs, err)
	} else {
		this.limiter = make(chan byte, this.MaxParallelism)
		this.limiter <- 1
		defer func() { <-this.limiter }()
		this.recursiveExecute(this.Context, nil, this.rootFields)
	}

	this.FireSubscriptionEventFunc(this.data.Bytes(), this.errs)
}

func (this *Execution) FireSubscriptionClose() {
	this.subMu.Lock()
	defer this.subMu.Unlock()
	this.FireSubscriptionCloseFunc()
	this.FireSubscriptionEventFunc = nil
	this.FireSubscriptionCloseFunc = nil
}

func (this *Execution) recursiveExecute(ctx context.Context, parentSelection *SelectionResolver, selectedFields *linkedmap.LinkedMap) { // (parentSelection *SelectionResolver, parentValue reflect.Value, parentType schema.Type, selections []query.Selection) ExecutionResult {

	this.data.WriteByte('{')
	writeComma := false
	for entry := selectedFields.First; entry != nil; entry = entry.Next {

		offset := this.data.Len()
		if writeComma {
			this.data.WriteByte(',')
		}

		// apply the resolver before the field is written, since it could
		// fail, and we don't want to write field that resulted in a resolver error.
		selected := entry.Value.(*SelectionResolver)
		err := this.executeSelected(ctx, parentSelection, selected)
		if err != nil {
			// undo any (likely partial) writes that we performed
			this.data.Truncate(offset)
			this.AddError(err)
			continue
		}
		writeComma = true
	}
	this.data.WriteByte('}')
}

var rawMessageType = reflect.TypeOf(resolvers.RawMessage{})
var valueWithContextType = reflect.TypeOf(resolvers.ValueWithContext{})

func (this *Execution) executeSelected(ctx context.Context, parentSelection *SelectionResolver, selected *SelectionResolver) (result *qerrors.Error) {

	defer func() {
		if value := recover(); value != nil {
			this.Logger.LogPanic(this.Context, value)
			err := makePanicError(value)
			err.PathStr = selected.Path()
			result = err
		}
	}()

	childValue, err := selected.Resolution()
	if err != nil {
		return qerrors.WrapError(err, err.Error()).WithPath(selected.Path()...).WithStack()
	}

	if childValue.IsValid() {
		if childValue.Type() == rawMessageType {
			message := childValue.Interface().(resolvers.RawMessage)
			// if it's wrapped, then unwrap...
			if message[0] == '{' {
				message = message[1 : len(message)-1]
			}
			this.data.Write(message)
			return
		}

		if childValue.Type() == valueWithContextType {
			vwc := childValue.Interface().(resolvers.ValueWithContext)
			childValue = vwc.Value
			ctx = vwc.Context
		}
	}

	field := selected.field

	this.data.WriteByte('"')
	this.data.WriteString(selected.field.Alias)
	this.data.WriteByte('"')
	this.data.WriteByte(':')

	childType, nonNullType := unwrapNonNull(field.Schema.Field.Type)
	valid := childValue.IsValid()

	if !valid || ((childValue.Kind() == reflect.Ptr || childValue.Kind() == reflect.Interface) && childValue.IsNil()) {
		if nonNullType {
			return (&qerrors.Error{
				QueryError: &uperrors.QueryError {
					Message: "ResolverFactory produced a nil value for a Non Null type",
				},
				PathStr:    selected.Path(),
			}).WithStack()
		} else {
			this.data.WriteString("null")
			return
		}
	}

	// Are we a leaf node?
	if selected.selections == nil {
		this.writeLeaf(childValue, selected, childType)
	} else {

		switch childType := childType.(type) {
		case *schema.List:
			this.writeList(*childType, childValue, selected, func(elementType schema.Type, element reflect.Value) {
				selectedFields := linkedmap.CreateLinkedMap(len(this.Operation.Selections))
				this.resolveFields(ctx, selected, selectedFields, element, elementType, selected.selections)
				this.recursiveExecute(ctx, selected, selectedFields)
			})
		case *schema.Object, *schema.Interface, *schema.Union:
			selectedFields := linkedmap.CreateLinkedMap(len(this.Operation.Selections))
			this.resolveFields(ctx, selected, selectedFields, childValue, childType, selected.selections)
			this.recursiveExecute(ctx, selected, selectedFields)
		}
	}
	return
}

func (this *Execution) skipByDirective(directives schema.DirectiveList) bool {
	skip, err := exec.SkipByDirective(directives, this.Vars)
	if err != nil {
		this.AddError(err)
	}
	return skip
}

func (this *Execution) writeList(listType schema.List, childValue reflect.Value, selectionResolver *SelectionResolver, writeElement func(elementType schema.Type, element reflect.Value)) {

	// Dereference pointers..
	for childValue.Kind() == reflect.Ptr {
		childValue = childValue.Elem()
	}
	// Dereference interfaces...
	for childValue.Kind() == reflect.Interface {
		childValue = childValue.Elem()
	}

	switch childValue.Kind() {
	case reflect.Slice, reflect.Array:
		l := childValue.Len()
		this.data.WriteByte('[')
		for i := 0; i < l; i++ {
			if i > 0 {
				this.data.WriteByte(',')
			}
			element := childValue.Index(i)
			switch elementType := listType.OfType.(type) {
			case *schema.List:
				this.writeList(*elementType, element, selectionResolver, writeElement)
			default:
				writeElement(elementType, element)
			}
		}
		this.data.WriteByte(']')
	default:
		i := childValue.Interface()
		fmt.Println(i)
		panic(fmt.Sprintf("Resolved object was not an array, it was a: %s", childValue.Type().String()))
	}
}

func (this *Execution) writeLeaf(childValue reflect.Value, selectionResolver *SelectionResolver, childType schema.Type) {
	switch childType := childType.(type) {
	case *schema.NonNull:
		if childValue.Kind() == reflect.Ptr && childValue.Elem().IsNil() {
			panic(qerrors.New("got nil for non-null %q", childType))
		} else {
			this.writeLeaf(childValue, selectionResolver, childType.OfType)
		}

	case *schema.Scalar:
		data, err := json.Marshal(childValue.Interface())
		if err != nil {
			panic(qerrors.New("could not marshal %v: %s", childValue, err))
		}
		this.data.Write(data)

	case *schema.Enum:

		// Deref the pointer.
		for childValue.Kind() == reflect.Ptr || childValue.Kind() == reflect.Interface {
			childValue = childValue.Elem()
		}

		this.data.WriteByte('"')
		this.data.WriteString(childValue.String())
		this.data.WriteByte('"')

	case *schema.List:
		this.writeList(*childType, childValue, selectionResolver, func(elementType schema.Type, element reflect.Value) {
			this.writeLeaf(element, selectionResolver, childType.OfType)
		})

	default:
		panic(fmt.Sprintf("Unknown type: %s", childType))
	}
}

func (r *Execution) AddError(err error) {
	if err != nil {
		var qe *qerrors.Error = nil
		switch err := err.(type) {
		case *qerrors.Error:
			qe = err
		default:
			qe = &qerrors.Error{
					QueryError: &uperrors.QueryError {
					Message: err.Error(),
				},
			}
		}
		r.Mu.Lock()
		r.errs = append(r.errs, qe)
		r.Mu.Unlock()
	}
}

func unwrapNonNull(t schema.Type) (schema.Type, bool) {
	if nn, ok := t.(*schema.NonNull); ok {
		return nn.OfType, true
	}
	return t, false
}
