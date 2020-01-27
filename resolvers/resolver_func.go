package resolvers

func Func(fn func(request *ResolveRequest) Resolution) Resolver {
    return &resolverFunc{fn}
}

type resolverFunc struct {
    apply func(request *ResolveRequest) Resolution
}

func (this *resolverFunc) Resolve(request *ResolveRequest) Resolution {
    return this.apply(request)
}
