package resolvers

import "reflect"

///////////////////////////////////////////////////////////////////////
//
// FieldResolverFactory resolves fields using struct fields on the parent
// value.
//
///////////////////////////////////////////////////////////////////////
type FieldResolverFactory struct{}

func (this *FieldResolverFactory) CreateResolver(request *ResolveRequest) Resolver {
    parentValue := dereference(request.Parent)
    if parentValue.Kind() != reflect.Struct {
        return nil
    }
    childValue, found := getChildField(&parentValue, request.Field.Name)
    if !found {
        return nil
    }
    return func() (reflect.Value, error) {
        return *childValue, nil
    }
}

func dereference(value reflect.Value) reflect.Value {
    for ; value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface; {
        value = value.Elem()
    }
    return value
}

