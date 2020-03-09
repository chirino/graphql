package resolvers

import "reflect"

///////////////////////////////////////////////////////////////////////
//
// FieldResolverFactory resolves fields using struct fields on the parent
// value.
//
///////////////////////////////////////////////////////////////////////
type fieldResolver byte

const FieldResolver = fieldResolver(0)

func (this fieldResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
	parentValue := dereference(request.Parent)
	if parentValue.Kind() != reflect.Struct {
		return next
	}
	childValue, found := getChildField(&parentValue, request.Field.Name)
	if !found {
		return next
	}
	return func() (reflect.Value, error) {
		return *childValue, nil
	}
}

func dereference(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface {
		value = value.Elem()
	}
	return value
}
