package deprecated

import (
    "context"
    "github.com/chirino/graphql"

    "github.com/chirino/graphql/errors"
    "github.com/chirino/graphql/internal/validation"
    "github.com/chirino/graphql/log"
    "github.com/chirino/graphql/query"
    "github.com/chirino/graphql/trace"
)

// ParseSchema parses a GraphQL schema and attaches the given root resolver. It returns an error if
// the Go type signature of the resolvers does not match the schema. If nil is passed as the
// resolver, then the schema can not be executed, but it may be inspected (e.g. with ToJSON).
func ParseSchema(schemaString string, resolver interface{}, opts ...SchemaOpt) (*Schema, error) {

	engine := graphql.New()
	err := engine.Schema.Parse(schemaString)

	if err != nil {
		return nil, err
	}
	s := &Schema{
		Engine: engine,
	}
	for _, opt := range opts {
		opt(s)
	}
	if resolver != nil {
		s.resolver = resolver
	}

	//s := &Schema{
	//	schema:           schema.New(),
	//	maxParallelism:   10,
	//	tracer:           trace.OpenTracingTracer{},
	//	validationTracer: trace.NoopValidationTracer{},
	//	logger:           &log.DefaultLogger{},
	//}
	//for _, opt := range opts {
	//	opt(s)
	//}
	//
	//if err := s.schema.Parse(schemaString); err != nil {
	//	return nil, err
	//}
	//
	//if resolver != nil {
	//	r, err := resolvable.ApplyResolver(s.schema, resolver)
	//	if err != nil {
	//		return nil, err
	//	}
	//	s.res = r
	//}

	return s, nil
}

// MustParseSchema calls ParseSchema and panics on error.
func MustParseSchema(schemaString string, resolver interface{}, opts ...SchemaOpt) *Schema {
	s, err := ParseSchema(schemaString, resolver, opts...)
	if err != nil {
		panic(err)
	}
	return s
}

// Schema represents a GraphQL schema with an optional resolver.
type Schema struct {
	//schema *schema.Schema
	//res    *resolvable.Schema
	//
	//maxDepth         int
	//maxParallelism   int
	//tracer           trace.Tracer
	//validationTracer trace.ValidationTracer
	//logger           log.Logger

	Engine   *graphql.Engine
	resolver interface{}
}

// SchemaOpt is an option to pass to ParseSchema or MustParseSchema.
type SchemaOpt func(*Schema)

// MaxDepth specifies the maximum field nesting depth in a query. The default is 0 which disables max depth checking.
func MaxDepth(n int) SchemaOpt {
	return func(s *Schema) {
		s.Engine.MaxDepth = n
	}
}

// MaxParallelism specifies the maximum number of resolvers per request allowed to run in parallel. The default is 10.
func MaxParallelism(n int) SchemaOpt {
	return func(s *Schema) {
		s.Engine.MaxParallelism = n
	}
}

// Tracer is used to trace queries and fields. It defaults to trace.OpenTracingTracer.
func Tracer(tracer trace.Tracer) SchemaOpt {
	return func(s *Schema) {
		s.Engine.Tracer = tracer
	}
}

// ValidationTracer is used to trace validation errors. It defaults to trace.NoopValidationTracer.
func ValidationTracer(tracer trace.ValidationTracer) SchemaOpt {
	return func(s *Schema) {
		s.Engine.ValidationTracer = tracer
	}
}

// Logger is used to log panics during query execution. It defaults to exec.DefaultLogger.
func Logger(logger log.Logger) SchemaOpt {
	return func(s *Schema) {
		s.Engine.Logger = logger
	}
}

// Validate validates the given query with the schema.
func (s *Schema) Validate(queryString string) []*errors.QueryError {
	doc, qErr := query.Parse(queryString)
	if qErr != nil {
		return []*errors.QueryError{qErr}
	}

	return validation.Validate(s.Engine.Schema, doc, s.Engine.MaxDepth)
}

// Exec executes the given query with the schema's resolver. It panics if the schema was created
// without a resolver. If the context get cancelled, no further resolvers will be called and a
// the context error will be returned as soon as possible (not immediately).
func (s *Schema) Exec(ctx context.Context, queryString string, operationName string, variables map[string]interface{}) *graphql.EngineResponse {
	if s.resolver == nil {
		panic("schema created without resolver, can not exec")
	}
	return s.Engine.ExecuteOne(&graphql.EngineRequest{
		Context:       ctx,
		Root:          s.resolver,
		Query:         queryString,
		OperationName: operationName,
		Variables:     variables,
	})
}

//func (s *Schema) exec(ctx context.Context, queryString string, operationName string, variables map[string]interface{}, res *resolvable.Schema) *Response {
//	doc, qErr := query.Parse(queryString)
//	if qErr != nil {
//		return &Response{Errors: []*errors.QueryError{qErr}}
//	}
//
//	validationFinish := s.validationTracer.TraceValidation()
//	errs := validation.Validate(s.schema, doc, s.maxDepth)
//	validationFinish(errs)
//	if len(errs) != 0 {
//		return &Response{Errors: errs}
//	}
//
//	op, err := getOperation(doc, operationName)
//	if err != nil {
//		return &Response{Errors: []*errors.QueryError{errors.Errorf("%s", err)}}
//	}
//
//	r := &exec.Request{
//		Request: selected.Request{
//			Doc:    doc,
//			Vars:   variables,
//			Schema: s.schema,
//		},
//		Limiter: make(chan struct{}, s.maxParallelism),
//		Tracer:  s.tracer,
//		Logger:  s.logger,
//	}
//	varTypes := make(map[string]*introspection.Type)
//	for _, v := range op.Vars {
//		t, err := common.ResolveType(v.Type, s.schema.Resolve)
//		if err != nil {
//			return &Response{Errors: []*errors.QueryError{err}}
//		}
//		varTypes[v.Name.Name] = introspection.WrapType(t)
//	}
//	traceCtx, finish := s.tracer.TraceQuery(ctx, queryString, operationName, variables, varTypes)
//	data, errs := r.Execute(traceCtx, res, op)
//	finish(errs)
//
//	return &Response{
//		Data:   data,
//		Errors: errs,
//	}
//}
