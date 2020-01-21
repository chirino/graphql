package resolvers

import (
    "fmt"
    "github.com/chirino/graphql/internal/introspection"
    "github.com/chirino/graphql/schema"
    "reflect"
)

///////////////////////////////////////////////////////////////////////
//
// MetadataResolverFactory resolves fields using schema metadata
//
///////////////////////////////////////////////////////////////////////
type MetadataResolverFactory struct{}

func (this *MetadataResolverFactory) CreateResolver(request *ResolveRequest) Resolver {
    s := request.Context.GetSchema()
    switch request.Field.Name {
    case "__typename":
        return func() (reflect.Value, error) {

            switch schemaType := request.ParentType.(type) {
            case *schema.Union:
                for _, pt := range schemaType.PossibleTypes {
                    if _, ok := TryCastFunction(request.Parent, pt.Name); ok {
                        return reflect.ValueOf(pt.Name), nil
                    }
                }
            case *schema.Interface:
                for _, pt := range schemaType.PossibleTypes {
                    if _, ok := TryCastFunction(request.Parent, pt.Name); ok {
                        return reflect.ValueOf(pt.Name), nil
                    }
                }
            default:
                return reflect.ValueOf(schemaType.String()), nil
            }
            return reflect.ValueOf(""), nil
        }

    case "__schema":
        return func() (reflect.Value, error) {
            return reflect.ValueOf(introspection.WrapSchema(s)), nil
        }

    case "__type":
        return func() (reflect.Value, error) {
            t, ok := s.Types[request.Args["name"].(string)]
            if !ok {
                return reflect.Value{}, fmt.Errorf("Could not find the type")
            }
            return reflect.ValueOf(introspection.WrapType(t)), nil
        }
    }
    return nil
}

