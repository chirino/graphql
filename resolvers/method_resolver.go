package resolvers

import (
	"github.com/chirino/graphql/internal/exec/packer"
	"reflect"
)

///////////////////////////////////////////////////////////////////////
//
// MethodResolverFactory resolves fields using the method
// implemented by a receiver type.
//
///////////////////////////////////////////////////////////////////////
type methodResolver byte

const MethodResolver = methodResolver(0)

func (this methodResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
	if !request.Parent.IsValid() {
		return nil
	}
	childMethod := getChildMethod(&request.Parent, request.Field.Name)
	if childMethod == nil {
		return next
	}

	var structPacker *packer.StructPacker = nil
	if childMethod.argumentsType != nil {
		p := packer.NewBuilder()
		sp, err := p.MakeStructPacker(request.Field.Args, *childMethod.argumentsType)
		if err != nil {
			return nil
		}
		structPacker = sp
		p.Finish()
	}

	return func() (reflect.Value, error) {
		var in []reflect.Value
		if childMethod.hasContext {
			in = append(in, reflect.ValueOf(request.ExecutionContext.GetContext()))
		}
		if childMethod.hasExecutionContext {
			in = append(in, reflect.ValueOf(request.ExecutionContext))
		}

		if childMethod.argumentsType != nil {

			argValue, err := structPacker.Pack(request.Args)
			if err != nil {
				return reflect.Value{}, err
			}
			in = append(in, argValue)

		}
		result := request.Parent.Method(childMethod.Index).Call(in)
		if childMethod.hasError && !result[1].IsNil() {
			return reflect.Value{}, result[1].Interface().(error)
		}
		if len(result) > 0 {
			return result[0], nil
		} else {
			return reflect.ValueOf(nil), nil
		}

	}
}
