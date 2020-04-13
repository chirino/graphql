package exec

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "github.com/chirino/graphql/errors"
    "github.com/chirino/graphql/internal/exec/packer"
    "github.com/chirino/graphql/internal/introspection"
    "github.com/chirino/graphql/internal/linkedmap"
    "github.com/chirino/graphql/log"
    "github.com/chirino/graphql/query"
    "github.com/chirino/graphql/resolvers"
    "github.com/chirino/graphql/schema"
    "github.com/chirino/graphql/trace"
    "reflect"
    "sync"
)

type Execution struct {
	Schema    *schema.Schema
	Vars      map[string]interface{}
	Doc       *query.Document
	Operation *query.Operation
	limiter   chan byte
	Tracer    trace.Tracer
	Logger    log.Logger
	Root      interface{}
	VarTypes  map[string]*introspection.Type
	Context   context.Context
	Resolver  resolvers.Resolver
	Mu        sync.Mutex

	subMu          sync.Mutex
	rootFields     *linkedmap.LinkedMap
	data           *bytes.Buffer
	errs           []*errors.QueryError
	MaxParallelism int
	Handler        func(data json.RawMessage, errors []*errors.QueryError)
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
		err.Path = path
		return err
	}
	return nil
}

func makePanicError(value interface{}) *errors.QueryError {
	return errors.Errorf("graphql: panic occurred: %v", value)
}

type ExecutionResult struct {
	Errors []*errors.QueryError
	Data   *bytes.Buffer
}

type SelectionResolver struct {
	parent     *SelectionResolver
	field      *query.Field
	Resolution resolvers.Resolution
	selections []query.Selection
}

func (this *SelectionResolver) Path() []string {
	if this == nil {
		return []string{}
	}
	if this.parent == nil {
		return []string{this.field.Alias.Text}
	}
	return append(this.parent.Path(), this.field.Alias.Text)
}

func (this *Execution) resolveFields(parentSelectionResolver *SelectionResolver, selectionResolvers *linkedmap.LinkedMap, parentValue reflect.Value, parentType schema.Type, selections []query.Selection) {
	for _, selection := range selections {
		switch field := selection.(type) {
		case *query.Field:
			if this.skipByDirective(field.Directives) {
				continue
			}

			var sr *SelectionResolver = nil
			x := selectionResolvers.Get(field.Alias.Text)
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
					evaluatedArguments[arg.Name.Text] = arg.Value.Evaluate(this.Vars)
				}

				resolveRequest := &resolvers.ResolveRequest{
					Context:       this,
					ParentType:    typeName,
					Parent:        parentValue,
					Field:         field.Schema.Field,
					Args:          evaluatedArguments,
					Selection:     field,
					SelectionPath: sr.Path,
				}
				resolution := this.Resolver.Resolve(resolveRequest, nil)

				if resolution == nil {
					this.AddError((&errors.QueryError{
						Message: "No resolver found",
						Path:    append(parentSelectionResolver.Path(), field.Alias.Text),
					}).WithStack())
				} else {
					sr.Resolution = resolution
					selectionResolvers.Set(field.Alias.Text, sr)
				}
			} else {
				// field previously resolved, but fragment is adding more child field selections.
				sr.selections = append(sr.selections, field.Selections...)
			}

		case *query.InlineFragment:
			if this.skipByDirective(field.Directives) {
				continue
			}

			fragment := &field.Fragment
			this.CreateSelectionResolversForFragment(parentSelectionResolver, fragment, parentType, parentValue, selectionResolvers)

		case *query.FragmentSpread:
			if this.skipByDirective(field.Directives) {
				continue
			}
			fragment := &this.Doc.Fragments.Get(field.Name.Text).Fragment
			this.CreateSelectionResolversForFragment(parentSelectionResolver, fragment, parentType, parentValue, selectionResolvers)
		}
	}
}

func (this *Execution) CreateSelectionResolversForFragment(parentSelectionResolver *SelectionResolver, fragment *query.Fragment, parentType schema.Type, parentValue reflect.Value, selectionResolvers *linkedmap.LinkedMap) {
	if fragment.On.Text != "" && fragment.On.Text != parentType.String() {
		castType := this.Schema.Types[fragment.On.Text]
		if casted, ok := resolvers.TryCastFunction(parentValue, fragment.On.Text); ok {
			this.resolveFields(parentSelectionResolver, selectionResolvers, casted, castType, fragment.Selections)
		}
	} else {
		this.resolveFields(parentSelectionResolver, selectionResolvers, parentValue, parentType, fragment.Selections)
	}
}

func (this *Execution) Execute() (bool, error) {

	rootType := this.Schema.EntryPoints[this.Operation.Type]
	rootValue := reflect.ValueOf(this.Root)
	rootFields := linkedmap.CreateLinkedMap(len(this.Operation.Selections))

	// async processing can start when the field is selected... apply limit here...
	this.errs = []*errors.QueryError{}
	this.limiter = make(chan byte, this.MaxParallelism)
	this.limiter <- 1
	this.resolveFields(nil, rootFields, rootValue, rootType, this.Operation.Selections)

	if this.Operation.Type == schema.Subscription {

		// subs are kinda special... we need to setup a subscription to an event,
		// and execute it every time the event is triggered...
		if len(this.Operation.Selections) != 1 {
			return true, errors.Errorf("you can only select 1 field in a subscription")
		}

		if len(rootFields.ValuesByKey) != 1 {
			// We may not have resolved it correctly..
			return true, errors.AsMulti(this.errs)
		}
		// This code path interact closely with with FireSubscriptionEvent method.
		this.rootFields = rootFields
		selected := rootFields.First.Value.(*SelectionResolver)
		_, err := selected.Resolution() // This should start go routines to fire events via FireSubscriptionEvent
		if err != nil {
			return true, err
		}
		return true, nil

	} else {

		// This is the first execution goroutine.
		// TODO: this may need to move for the subscription case..
		this.data = &bytes.Buffer{}
		defer func() { <-this.limiter }()
		this.recursiveExecute(nil, rootFields)
		this.Handler(json.RawMessage(this.data.Bytes()), this.errs)
		return false, nil
	}
}

// This is called by subscription resolvers to send out a subscription event.
func (this *Execution) FireSubscriptionEvent(value reflect.Value) {
	if this.rootFields == nil {
		panic("the FireSubscriptionEvent method should only be called when triggering events for subscription fields")
	}

	// Protect against a resolver firing concurrent events at us.. we only will process one at
	// at time.
	this.subMu.Lock()
	defer this.subMu.Unlock()

	selected := this.rootFields.First.Value.(*SelectionResolver)
	selected.Resolution = func() (reflect.Value, error) {
		return value, nil
	}
	this.data = &bytes.Buffer{}
	this.errs = []*errors.QueryError{}
	this.limiter = make(chan byte, this.MaxParallelism)
	this.limiter <- 1
	defer func() { <-this.limiter }()
	this.recursiveExecute(nil, this.rootFields)
	this.Handler(json.RawMessage(this.data.Bytes()), this.errs)
}

func (this *Execution) recursiveExecute(parentSelection *SelectionResolver, selectedFields *linkedmap.LinkedMap) { // (parentSelection *SelectionResolver, parentValue reflect.Value, parentType schema.Type, selections []query.Selection) ExecutionResult {

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
		err := this.executeSelected(parentSelection, selected)
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

func (this *Execution) executeSelected(parentSelection *SelectionResolver, selected *SelectionResolver) (result *errors.QueryError) {

	defer func() {
		if value := recover(); value != nil {
			this.Logger.LogPanic(this.Context, value)
			err := makePanicError(value)
			err.Path = selected.Path()
			result = err
		}
	}()

	resolver := selected.Resolution
	childValue, err := resolver()
	if err != nil {
		return (&errors.QueryError{
			Message:       err.Error(),
			Path:          selected.Path(),
			ResolverError: err,
		}).WithStack()
	}

	field := selected.field

	this.data.WriteByte('"')
	this.data.WriteString(selected.field.Alias.Text)
	this.data.WriteByte('"')
	this.data.WriteByte(':')

	childType, nonNullType := unwrapNonNull(field.Schema.Field.Type)
	if (childValue.Kind() == reflect.Ptr || childValue.Kind() == reflect.Interface) &&
		childValue.IsNil() {
		if nonNullType {
			return (&errors.QueryError{
				Message: "ResolverFactory produced a nil value for a Non Null type",
				Path:    selected.Path(),
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
				this.resolveFields(selected, selectedFields, element, elementType, selected.selections)
				this.recursiveExecute(selected, selectedFields)
			})
		case *schema.Object, *schema.Interface, *schema.Union:
			selectedFields := linkedmap.CreateLinkedMap(len(this.Operation.Selections))
			this.resolveFields(selected, selectedFields, childValue, childType, selected.selections)
			this.recursiveExecute(selected, selectedFields)
		}
	}
	return
}

func (this *Execution) skipByDirective(directives schema.DirectiveList) bool {
	if d := directives.Get("skip"); d != nil {
		p := packer.ValuePacker{ValueType: reflect.TypeOf(false)}
		v, err := p.Pack(d.Args.MustGet("if").Evaluate(this.Vars))
		if err != nil {
			this.AddError(errors.Errorf("%s", err))
		}
		if err == nil && v.Bool() {
			return true
		}
	}

	if d := directives.Get("include"); d != nil {
		p := packer.ValuePacker{ValueType: reflect.TypeOf(false)}
		v, err := p.Pack(d.Args.MustGet("if").Evaluate(this.Vars))
		if err != nil {
			this.AddError(errors.Errorf("%s", err))
		}
		if err == nil && !v.Bool() {
			return true
		}
	}
	return false
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
			panic(errors.Errorf("got nil for non-null %q", childType))
		} else {
			this.writeLeaf(childValue, selectionResolver, childType.OfType)
		}

	case *schema.Scalar:
		data, err := json.Marshal(childValue.Interface())
		if err != nil {
			panic(errors.Errorf("could not marshal %v: %s", childValue, err))
		}
		this.data.Write(data)

	case *schema.Enum:

		// Deref the pointer.
		for childValue.Kind() == reflect.Ptr {
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

func (r *Execution) AddError(err *errors.QueryError) {
	if err != nil {
		r.Mu.Lock()
		r.errs = append(r.errs, err)
		r.Mu.Unlock()
	}
}

func unwrapNonNull(t schema.Type) (schema.Type, bool) {
	if nn, ok := t.(*schema.NonNull); ok {
		return nn.OfType, true
	}
	return t, false
}
