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
type MethodResolverFactory struct{}

func (this *MethodResolverFactory) CreateResolver(request *ResolveRequest) Resolver {
    childMethod := getChildMethod(&request.Parent, request.Field.Name)
    if childMethod == nil {
        return nil
    }

    var structPacker *packer.StructPacker = nil
    if childMethod.argumentsType != nil {
        p := packer.NewBuilder()
        defer p.Finish()
        sp, err := p.MakeStructPacker(request.Field.Args, *childMethod.argumentsType)
        if err != nil {
            return nil
        }
        structPacker = sp
    }

    return func() (reflect.Value, error) {
        var in []reflect.Value
        if childMethod.hasContext {
            in = append(in, reflect.ValueOf(request.Context.GetContext()))
        }
        if childMethod.hasExecutionContext {
            in = append(in, reflect.ValueOf(request.Context))
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
        return result[0], nil

    }
}

