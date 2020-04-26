package trace

import (
	"context"
	"fmt"

	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/qerrors"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
)

type TraceQueryFinishFunc func(qerrors.ErrorList)
type TraceFieldFinishFunc func(*qerrors.Error)

type Tracer interface {
	TraceQuery(ctx context.Context, queryString string, operationName string, variables interface{}, varTypes map[string]*introspection.Type) (context.Context, TraceQueryFinishFunc)
	TraceField(ctx context.Context, label, typeName, fieldName string, trivial bool, args map[string]interface{}) (context.Context, TraceFieldFinishFunc)
}

type OpenTracingTracer struct{}

func (OpenTracingTracer) TraceQuery(ctx context.Context, queryString string, operationName string, variables interface{}, varTypes map[string]*introspection.Type) (context.Context, TraceQueryFinishFunc) {
	span, spanCtx := opentracing.StartSpanFromContext(ctx, "GraphQL request")
	span.SetTag("graphql.query", queryString)

	if operationName != "" {
		span.SetTag("graphql.operationName", operationName)
	}

	if variables != nil {
		span.LogFields(log.Object("graphql.variables", variables))
	}

	return spanCtx, func(errs qerrors.ErrorList) {
		if len(errs) > 0 {
			msg := errs[0].Error()
			if len(errs) > 1 {
				msg += fmt.Sprintf(" (and %d more errors)", len(errs)-1)
			}
			ext.Error.Set(span, true)
			span.SetTag("graphql.error", msg)
		}
		span.Finish()
	}
}

func (OpenTracingTracer) TraceField(ctx context.Context, label, typeName, fieldName string, trivial bool, args map[string]interface{}) (context.Context, TraceFieldFinishFunc) {
	if trivial {
		return ctx, noop
	}

	span, spanCtx := opentracing.StartSpanFromContext(ctx, label)
	span.SetTag("graphql.type", typeName)
	span.SetTag("graphql.field", fieldName)
	for name, value := range args {
		span.SetTag("graphql.args."+name, value)
	}

	return spanCtx, func(err *qerrors.Error) {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("graphql.error", err.Error())
		}
		span.Finish()
	}
}

func noop(*qerrors.Error) {}

type NoopTracer struct{}

func (NoopTracer) TraceQuery(ctx context.Context, queryString string, operationName string, variables interface{}, varTypes map[string]*introspection.Type) (context.Context, TraceQueryFinishFunc) {
	return ctx, func(errs qerrors.ErrorList) {}
}

func (NoopTracer) TraceField(ctx context.Context, label, typeName, fieldName string, trivial bool, args map[string]interface{}) (context.Context, TraceFieldFinishFunc) {
	return ctx, func(err *qerrors.Error) {}
}
