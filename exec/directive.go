package exec

import (
	"reflect"

	"github.com/chirino/graphql/errors"
	"github.com/chirino/graphql/internal/exec/packer"
	"github.com/chirino/graphql/schema"
)

func SkipByDirective(l schema.DirectiveList, vars map[string]interface{}) (bool, *errors.QueryError) {
	if d := l.Get("skip"); d != nil {
		return evaluateIf(d, vars)
	}
	if d := l.Get("include"); d != nil {
		b, err := evaluateIf(d, vars)
		if err != nil {
			return false, err
		}
		return !b, nil
	}
	return false, nil
}

func evaluateIf(d *schema.Directive, vars map[string]interface{}) (bool, *errors.QueryError) {
	arg := d.Args.MustGet("if")
	if arg == nil {
		return false, errors.New("@skip if argument missing").WithLocations(d.Name.Loc).WithStack()
	}
	p := packer.ValuePacker{ValueType: reflect.TypeOf(false)}
	v, err := p.Pack(arg.Evaluate(vars))
	if err != nil {
		return false, errors.New(err.Error()).WithLocations(d.Name.Loc).WithStack()
	}
	return v.Bool(), nil
}
