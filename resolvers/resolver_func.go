package resolvers

func Func(fn func(request *ResolveRequest, next Resolution) Resolution) Resolver {
    return &resolverFunc{fn}
}

type resolverFunc struct {
    apply func(request *ResolveRequest, next Resolution) Resolution
}

func (this *resolverFunc) Resolve(request *ResolveRequest, next Resolution) Resolution {
    return this.apply(request, next)
}
