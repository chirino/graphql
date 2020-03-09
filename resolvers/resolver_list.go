package resolvers

///////////////////////////////////////////////////////////////////////
//
// ResolverFactoryList uses a list of other resolvers to resolve
// requests.  Last resolver wins.
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

func (this *ResolverList) Resolve(request *ResolveRequest, next Resolution) Resolution {
	for _, f := range *this {
		next = f.Resolve(request, next)
	}
	return next
}
