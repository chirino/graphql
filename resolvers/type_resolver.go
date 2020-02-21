package resolvers

type TypeResolver map[string]Resolver

func (this TypeResolver) Set(typeName string, factory Resolver) {
    this[typeName] = factory
}

func (this TypeResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
    if request.ParentType == nil {
        return next
    }
    resolverFunc := this[request.ParentType.String()]
    if resolverFunc == nil {
        return next
    }
    return resolverFunc.Resolve(request, next)
}

