package resolvers

type Func func(request *ResolveRequest, next Resolution) Resolution

func (f Func) Resolve(request *ResolveRequest, next Resolution) Resolution {
	return f(request, next)
}
