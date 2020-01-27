package resolvers

import "github.com/chirino/graphql/schema"

///////////////////////////////////////////////////////////////////////
//
// directiveResolverFactory gets applied to fields that have a given directive
//
///////////////////////////////////////////////////////////////////////
type DirectiveResolver struct {
    Directive string
    Create    func(args schema.ArgumentList, request *ResolveRequest) Resolution
}

func (factory DirectiveResolver) Resolve(request *ResolveRequest) Resolution {
    for _, d := range request.Field.Directives {
        if d.Name.Text == factory.Directive {
            return factory.Create(d.Args, request)
        }
    }
    return nil
}

// ResolverFilter
///////////////////////////////////////////////////////////////////////
//
// DirectiveFilter gets applied to fields that have a given directive
//
///////////////////////////////////////////////////////////////////////
type DirectiveFilter struct {
    Directive       string
    DirectiveFilter func(args schema.ArgumentList, next Resolution) Resolution
}

func (factory DirectiveFilter) Filter(request *ResolveRequest, resolution Resolution) Resolution {
    // Check to see if it's applied as a field level directive:
    resolution = factory.filter(request.Field.Directives, resolution)

    // Also check to see if it's applied as an object level directive:
    switch t := request.Field.Type.(type) {
    case *schema.Object:
        resolution = factory.filter(t.Directives, resolution)
    }
    return resolution
}

func (factory DirectiveFilter) filter(directives schema.DirectiveList, resolution Resolution) Resolution {
    for _, d := range directives {
        if d.Name.Text == factory.Directive {
            return factory.DirectiveFilter(d.Args, resolution)
        }
    }
    return resolution
}
