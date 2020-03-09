package resolvers

type nilResolver byte

var NilResolver = nilResolver(0)

func (this nilResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
	return nil
}
