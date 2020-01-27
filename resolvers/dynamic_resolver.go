package resolvers

type dynamicResolver struct {
}

func DynamicResolverFactory() Resolver {
    return &dynamicResolver{}
}

func (this *dynamicResolver) Resolve(request *ResolveRequest) Resolution {
    resolver := MetadataResolverFactory.Resolve(request)
    if resolver != nil {
        return resolver
    }
    resolver = MethodResolverFactory.Resolve(request)
    if resolver != nil {
        return resolver
    }
    resolver = FieldResolverFactory.Resolve(request)
    if resolver != nil {
        return resolver
    }
    resolver = MapResolverFactory.Resolve(request)
    if resolver != nil {
        return resolver
    }
    return nil
}


