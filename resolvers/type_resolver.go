package resolvers

type TypeResolver map[string]Resolver

func (this TypeResolver) Set(typeName string, factory Resolver) {
    this[typeName] = factory
}

func (this TypeResolver) Resolve(request *ResolveRequest) Resolution {
    if request.ParentType == nil {
        return nil
    }
    resolverFunc := this[request.ParentType.String()]
    if resolverFunc == nil {
        return nil
    }
    return resolverFunc.Resolve(request)
}

