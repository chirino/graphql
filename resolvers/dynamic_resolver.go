package resolvers

type dynamicResolverFactory struct {
}

func DynamicResolverFactory() ResolverFactory {
    return &dynamicResolverFactory{}
}

func (this *dynamicResolverFactory) CreateResolver(request *ResolveRequest) Resolver {
    resolver := (&MetadataResolverFactory{}).CreateResolver(request)
    if resolver != nil {
        return resolver
    }
    resolver = (&MethodResolverFactory{}).CreateResolver(request)
    if resolver != nil {
        return resolver
    }
    resolver = (&FieldResolverFactory{}).CreateResolver(request)
    if resolver != nil {
        return resolver
    }
    resolver = (&MapResolverFactory{}).CreateResolver(request)
    if resolver != nil {
        return resolver
    }
    return nil
}


