package resolvers

type dynamicResolver struct {
}

func DynamicResolverFactory() Resolver {
    return &dynamicResolver{}
}

func (this *dynamicResolver) Resolve(request *ResolveRequest) Resolution {
    resolver := MetadataResolver.Resolve(request)
    if resolver != nil {
        return resolver
    }
    resolver = MethodResolver.Resolve(request)
    if resolver != nil {
        return resolver
    }
    resolver = FieldResolver.Resolve(request)
    if resolver != nil {
        return resolver
    }
    resolver = MapResolver.Resolve(request)
    if resolver != nil {
        return resolver
    }
    return nil
}


