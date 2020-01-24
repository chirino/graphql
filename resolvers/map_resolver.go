package resolvers

import "reflect"

///////////////////////////////////////////////////////////////////////
//
// MapResolverFactory resolves fields using entries in a map
//
///////////////////////////////////////////////////////////////////////
type mapResolverFactory byte
const MapResolverFactory = mapResolverFactory(0)

func (this mapResolverFactory) CreateResolver(request *ResolveRequest) Resolver {
    parentValue := dereference(request.Parent)
    if parentValue.Kind() != reflect.Map || parentValue.Type().Key().Kind() != reflect.String {
        return nil
    }

    return func() (reflect.Value, error) {
        field := reflect.ValueOf(request.Field.Name)
        value := parentValue.MapIndex(field)
        if !value.IsValid() {
            return reflect.Zero(parentValue.Type().Elem()), nil
        }
        if value.Interface() == nil {
            return value, nil
        }
        value = reflect.ValueOf(value.Interface())
        return value, nil
    }
}

