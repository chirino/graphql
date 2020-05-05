package resolvers

type TypeAndFieldResolver map[TypeAndFieldKey]Func

type TypeAndFieldKey struct {
	Type  string
	Field string
}

func (r TypeAndFieldResolver) Set(typeName string, field string, f Func) {
	r[TypeAndFieldKey{Type: typeName, Field: field}] = f
}

func (r TypeAndFieldResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
	if request.ParentType == nil {
		return next
	}
	f := r[TypeAndFieldKey{
		Type:  request.ParentType.String(),
		Field: request.Field.Name,
	}]
	if f == nil {
		return next
	}
	return f(request, next)
}
