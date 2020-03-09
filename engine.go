package graphql

import (
	"github.com/chirino/graphql/log"
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
