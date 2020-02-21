package resolvers

import "github.com/chirino/graphql/schema"

///////////////////////////////////////////////////////////////////////
//
// DirectiveResolver gets applied to fields or types that have a given
// directive
//
///////////////////////////////////////////////////////////////////////
type DirectiveResolver struct {
    Directive string
    Create    func(request *ResolveRequest, next Resolution, args map[string]interface{}) Resolution
}

func (factory DirectiveResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
    if args := matchingArgList(request, factory.Directive); args != nil {
        args := args.Value(nil)
        return factory.Create(request, next, args)
    }
    return next
}

func matchingArgList(request *ResolveRequest, directive string) schema.ArgumentList {
    for _, d := range request.Field.Directives {
        if d.Name.Text == directive {
            return d.Args
        }
    }
    if parentType, ok := request.ParentType.(schema.HasDirectives); ok {
        for _, d := range parentType.GetDirectives() {
            if d.Name.Text == directive {
                return d.Args
            }
        }
    }
    return nil
}
