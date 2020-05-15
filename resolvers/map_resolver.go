package resolvers

import "reflect"

///////////////////////////////////////////////////////////////////////
//
// MapResolverFactory resolves fields using entries in a map
//
///////////////////////////////////////////////////////////////////////
type mapResolver byte

const MapResolver = mapResolver(0)

func (this mapResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {

	parentValue := Dereference(request.Parent)
	if parentValue.Kind() != reflect.Map || parentValue.Type().Key().Kind() != reflect.String {
		return next
	}
	field := reflect.ValueOf(request.Field.Name)
	value := parentValue.MapIndex(field)
	if !value.IsValid() {
		value = reflect.Zero(parentValue.Type().Elem())
	}

	return func() (reflect.Value, error) {
		return value, nil
	}
}
