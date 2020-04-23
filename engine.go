package graphql

import (
	"context"
	"encoding/json"

	"github.com/chirino/graphql/errors"
	"github.com/chirino/graphql/internal/exec"
	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/internal/validation"
	"github.com/chirino/graphql/log"
	"github.com/chirino/graphql/query"
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
	result := engine.ExecuteOne(&EngineRequest{
		Query: introspection.Query,
	})
	return result.Data, result.Error()
}

func (engine *Engine) Exec(ctx context.Context, result interface{}, query string, args ...interface{}) error {
	variables := map[string]interface{}{}
	for i := 0; i+1 < len(args); i += 2 {
		variables[args[i].(string)] = args[i+1]
	}
	response := engine.ExecuteOne(&EngineRequest{
		Context:   ctx,
		Query:     query,
		Variables: variables,
	})

	if result != nil && response != nil {
		switch result := result.(type) {
		case *[]byte:
			*result = response.Data
		case *string:
			*result = string(response.Data)
		default:
			err := json.Unmarshal(response.Data, result)
			if err != nil {
				return errors.Multi(err, response.Error())
			}
		}
	}
	return response.Error()
}

// Execute the given request.
func (engine *Engine) ExecuteOne(request *EngineRequest) *EngineResponse {
	stream, err := engine.Execute(request)
	if err != nil {
		return &EngineResponse{
			Errors: errors.AsArray(err),
		}
	}
	defer stream.Close()
	if stream.IsSubscription {
		return &EngineResponse{
			Errors: errors.AsArray(errors.Errorf("ExecuteOne method does not support getting results from subscriptions")),
		}
	}
	return stream.Next()
}

func (engine *Engine) Execute(request *EngineRequest) (*ResponseStream, error) {
	doc, qErr := query.Parse(request.Query)
	if qErr != nil {
		return nil, qErr
	}

	validationFinish := engine.ValidationTracer.TraceValidation()
	errs := validation.Validate(engine.Schema, doc, engine.MaxDepth)
	validationFinish(errs)
	if len(errs) != 0 {
		return nil, errors.AsMulti(errs)
	}

	op, err := doc.GetOperation(request.OperationName)
	if err != nil {
		return nil, err
	}

	varTypes := make(map[string]*introspection.Type)
	for _, v := range op.Vars {
		t, err := schema.ResolveType(v.Type, engine.Schema.Resolve)
		if err != nil {
			return nil, err
		}
		varTypes[v.Name.Text] = introspection.WrapType(t)
	}

	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	ctx = cancelCtx

	traceContext, finish := engine.Tracer.TraceQuery(ctx, request.Query, request.OperationName, request.Variables, varTypes)
	responses := make(chan *EngineResponse, 1)

	variables, err := request.UnmarshalVariables()
	if err != nil {
		return nil, err
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
		Handler: func(d json.RawMessage, e []*errors.QueryError) {
			responses <- &EngineResponse{
				Data:   d,
				Errors: e,
			}
		},
	}

	sub, err := r.Execute()
	if err != nil {
		return nil, err
	}
	finish(errs)
	return &ResponseStream{
		IsSubscription: sub,
		Cancel:         cancelFunc,
		Responses:      responses,
	}, nil
}
