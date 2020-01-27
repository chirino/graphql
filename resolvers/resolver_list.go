package resolvers

///////////////////////////////////////////////////////////////////////
//
// ResolverFactoryList uses a list of other resolvers to resolve
// requests.  First resolver that matches wins.
//
///////////////////////////////////////////////////////////////////////
type ResolverList []Resolver

func List(factories ...Resolver) *ResolverList {
    list := ResolverList(factories)
    return &list
}

func (this *ResolverList) Add(factory Resolver) {
    *this = append(*this, factory)
}

func (this *ResolverList) Resolve(request *ResolveRequest) Resolution {
    for _, f := range *this {
        resolver := f.Resolve(request)
        if resolver != nil {
            return resolver
        }
    }
    return nil
}
