package graphql

import (
	"context"
	"encoding/json"

	"github.com/chirino/graphql/internal/exec"
	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/internal/validation"
	"github.com/chirino/graphql/log"
	"github.com/chirino/graphql/qerrors"
	"github.com/chirino/graphql/resolvers"
	"github.com/chirino/graphql/schema"
	"github.com/chirino/graphql/trace"
)

type Engine struct {
	Schema           *schema.Schema
	MaxDepth         int
	MaxParallelism   int
	Tracer           trace.Tracer
	ValidationTracer trace.ValidationTracer
	Logger           log.Logger
	Resolver         resolvers.Resolver
	Root             interface{}
}

func CreateEngine(schema string) (*Engine, error) {
	engine := New()
	err := engine.Schema.Parse(schema)
	return engine, err
}

func New() *Engine {
	return &Engine{
		Schema:           schema.New(),
		Tracer:           trace.NoopTracer{},
		MaxParallelism:   10,
		MaxDepth:         50,
		ValidationTracer: trace.NoopValidationTracer{},
		Logger:           &log.DefaultLogger{},
		Resolver:         resolvers.DynamicResolverFactory(),
	}
}

func (engine *Engine) GetSchemaIntrospectionJSON() ([]byte, error) {
	return GetSchemaIntrospectionJSON(engine.ServeGraphQL)
}

func (engine *Engine) Exec(ctx context.Context, result interface{}, query string, args ...interface{}) error {
	return Exec(engine.ServeGraphQL, ctx, result, query, args...)
}

func (engine *Engine) ServeGraphQL(request *Request) *Response {
	return ServeGraphQLStreamFunc(engine.ServeGraphQLStream).ServeGraphQL(request)
}

type responseStream struct {
	cancel    context.CancelFunc
	responses chan *Response
}

func (r responseStream) Close() {
	r.cancel()
}

func (r responseStream) Responses() <-chan *Response {
	return r.responses
}

func (r responseStream) CloseWithErr(err error) responseStream {
	r.responses <- NewResponse().AddError(err)
	close(r.responses)
	return r
}

func (engine *Engine) ServeGraphQLStream(request *Request) ResponseStream {
	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	ctx = cancelCtx

	stream := responseStream{
		cancel:    cancelFunc,
		responses: make(chan *Response, 1),
	}

	doc := &schema.QueryDocument{}
	qErr := doc.Parse(request.Query)
	if qErr != nil {
		return stream.CloseWithErr(qErr)
	}

	validationFinish := engine.ValidationTracer.TraceValidation()
	errs := validation.Validate(engine.Schema, doc, engine.MaxDepth)
	validationFinish(errs)
	if len(errs) != 0 {
		return stream.CloseWithErr(errs.Error())
	}

	op, err := doc.GetOperation(request.OperationName)
	if err != nil {
		return stream.CloseWithErr(err)
	}

	varTypes := make(map[string]*introspection.Type)
	for _, v := range op.Vars {
		t, err := schema.ResolveType(v.Type, engine.Schema.Resolve)
		if err != nil {
			return stream.CloseWithErr(err)
		}
		varTypes[v.Name] = introspection.WrapType(t)
	}

	traceContext, finish := engine.Tracer.TraceQuery(ctx, request.Query, request.OperationName, request.Variables, varTypes)

	variables, err := request.VariablesAsMap()
	if err != nil {
		return stream.CloseWithErr(err)
	}

	r := exec.Execution{
		Query:          request.Query,
		Vars:           variables,
		Schema:         engine.Schema,
		Tracer:         engine.Tracer,
		Logger:         engine.Logger,
		Resolver:       engine.Resolver,
		Doc:            doc,
		Operation:      op,
		VarTypes:       varTypes,
		MaxParallelism: engine.MaxParallelism,
		Root:           engine.Root,
		Context:        traceContext,
		FireSubscriptionEventFunc: func(d json.RawMessage, e qerrors.ErrorList) {
			stream.responses <- &Response{
				Data:   d,
				Errors: e,
			}
		},
		FireSubscriptionCloseFunc: func() {
			close(stream.responses)
		},
	}

	err = r.Execute()
	if err != nil {
		return stream.CloseWithErr(err)
	}
	finish(errs)
	return stream
}
