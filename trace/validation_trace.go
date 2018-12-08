package trace

import (
	"github.com/chirino/graphql/errors"
)

type TraceValidationFinishFunc = TraceQueryFinishFunc

type ValidationTracer interface {
	TraceValidation() TraceValidationFinishFunc
}

type NoopValidationTracer struct{}

func (NoopValidationTracer) TraceValidation() TraceValidationFinishFunc {
	return func(errs []*errors.QueryError) {}
}
