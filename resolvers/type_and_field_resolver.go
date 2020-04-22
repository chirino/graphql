package resolvers

type TypeAndFieldResolver map[typeAndFieldResolverKey]Func

type typeAndFieldResolverKey struct {
	typeName string
	field    string
}

func (r TypeAndFieldResolver) Set(typeName string, field string, f Func) {
	r[typeAndFieldResolverKey{typeName: typeName, field: field}] = f
}

func (r TypeAndFieldResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
	if request.ParentType == nil {
		return next
	}
	f := r[typeAndFieldResolverKey{
		typeName: request.ParentType.String(),
		field:    request.Field.Name,
	}]
	if f == nil {
		return next
	}
	return f(request, next)
}
