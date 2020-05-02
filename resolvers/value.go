package resolvers

import "reflect"

// MapValue allows you to convert map a resolved result to something new.
func MapValue(mapper func(value reflect.Value) reflect.Value) Resolver {
	return Func(func(request *ResolveRequest, next Resolution) Resolution {
		if next == nil {
			return nil
		}
		return func() (value reflect.Value, err error) {
			value, err = next()
			if err != nil {
				return
			}
			value = mapper(value)
			return
		}
	})
}

// Sniff allows you see resolution requests, but does not impact them
func Sniff(sniffer func(request *ResolveRequest, next Resolution)) Resolver {
	return Func(func(request *ResolveRequest, next Resolution) Resolution {
		sniffer(request, next)
		return next
	})
}
