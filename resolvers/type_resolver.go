package resolvers

type TypeResolverFactory map[string]ResolverFactory

func (this TypeResolverFactory) Set(typeName string, factory ResolverFactory) {
    this[typeName] = factory
}

func (this TypeResolverFactory) CreateResolver(request *ResolveRequest) Resolver {
    if request.ParentType == nil {
        return nil
    }
    resolverFunc := this[request.ParentType.String()]
    if resolverFunc == nil {
        return nil
    }
    return resolverFunc.CreateResolver(request)
}

