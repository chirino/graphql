package resolvers

type dynamicResolver struct {
}

func DynamicResolverFactory() Resolver {
	return &dynamicResolver{}
}

func (this *dynamicResolver) Resolve(request *ResolveRequest, next Resolution) Resolution {
	resolver := MetadataResolver.Resolve(request, next)
	if resolver != nil {
		return resolver
	}
	resolver = MethodResolver.Resolve(request, next)
	if resolver != nil {
		return resolver
	}
	resolver = FieldResolver.Resolve(request, next)
	if resolver != nil {
		return resolver
	}
	resolver = MapResolver.Resolve(request, next)
	if resolver != nil {
		return resolver
	}
	return next
}
