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
	Schema         *schema.Schema
	MaxDepth       int
	MaxParallelism int
	Tracer         trace.Tracer
	Logger         log.Logger
	Resolver       resolvers.Resolver
	Root           interface{}
	// Validate can be set to nil to disable validation.
	Validate func(doc *schema.QueryDocument, maxDepth int) error
	// OnRequest is called after the query is parsed but before the request is validated.
	OnRequestHook func(request *Request, doc *schema.QueryDocument, op *schema.Operation) error
}

func CreateEngine(schema string) (*Engine, error) {
	engine := New()
	err := engine.Schema.Parse(schema)
	return engine, err
}

func New() *Engine {
	e := &Engine{
		Schema:         schema.New(),
		Tracer:         trace.NoopTracer{},
		MaxParallelism: 10,
		MaxDepth:       50,
		Logger:         &log.DefaultLogger{},
		Resolver:       resolvers.DynamicResolverFactory(),
	}
	e.Validate = e.validate
	return e
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

	doc := &schema.QueryDocument{}
	err := doc.Parse(request.Query)
	if err != nil {
		return NewErrStream(err)
	}

	op, err := doc.GetOperation(request.OperationName)
	if err != nil {
		return NewErrStream(err)
	}

	if engine.OnRequestHook != nil {
		err := engine.OnRequestHook(request, doc, op)
		if err != nil {
			return NewErrStream(err)
		}
	}

	if engine.Validate != nil {
		err = engine.Validate(doc, engine.MaxDepth)
		if err != nil {
			return NewErrStream(err)
		}
	}

	varTypes := make(map[string]*introspection.Type)
	for _, v := range op.Vars {
		t, err := schema.ResolveType(v.Type, engine.Schema.Resolve)
		if err != nil {
			return NewErrStream(err)
		}
		varTypes[v.Name] = introspection.WrapType(t)
	}

	ctx := request.GetContext()
	traceContext, traceResponse, traceFinish := engine.Tracer.TraceQuery(ctx, request.Query, request.OperationName, request.Variables, varTypes)

	variables, err := request.VariablesAsMap()
	if err != nil {
		return NewErrStream(err)
	}

	responses := make(chan *Response, 1)
	r := exec.Execution{
		Context:        traceContext,
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
		FireSubscriptionEventFunc: func(d json.RawMessage, e qerrors.ErrorList) {
			responses <- &Response{
				Data:   d,
				Errors: e,
			}
			traceResponse(e)
		},
		FireSubscriptionCloseFunc: func() {
			close(responses)
			doc.Close()
			traceFinish()
		},
	}

	err = r.Execute()
	if err != nil {
		return NewErrStream(err)
	}
	return responses
}

func (engine *Engine) validate(doc *schema.QueryDocument, maxDepth int) error {
	errs := validation.Validate(engine.Schema, doc, maxDepth)
	if len(errs) != 0 {
		return errs.Error()
	}
	return nil
}
