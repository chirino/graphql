package resolvers

func Func(fn func(request *ResolveRequest) Resolver) ResolverFactory {
    return &resolverFactoryFunc{fn}
}

type resolverFactoryFunc struct {
    apply func(request *ResolveRequest) Resolver
}

func (this *resolverFactoryFunc) CreateResolver(request *ResolveRequest) Resolver {
    return this.apply(request)
}
