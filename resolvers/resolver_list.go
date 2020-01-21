package resolvers

///////////////////////////////////////////////////////////////////////
//
// ResolverFactoryList uses a list of other resolvers to resolve
// requests.  First resolver that matches wins.
//
///////////////////////////////////////////////////////////////////////
type ResolverFactoryList []ResolverFactory

func List(factories ...ResolverFactory) *ResolverFactoryList {
    list := ResolverFactoryList(factories)
    return &list
}

func (this *ResolverFactoryList) Add(factory ResolverFactory) {
    *this = append(*this, factory)
}

func (this *ResolverFactoryList) CreateResolver(request *ResolveRequest) Resolver {
    for _, f := range *this {
        resolver := f.CreateResolver(request)
        if resolver != nil {
            return resolver
        }
    }
    return nil
}
