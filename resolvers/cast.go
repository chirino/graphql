package resolvers

import (
	"reflect"
	"strings"
)

func normalizeMethodName(method string) string {
	//method = strings.Replace(method, "_", "", -1)
	method = strings.ToLower(method)
	return method
}

var castMethodCache Cache

func TryCastFunction(parentValue reflect.Value, toType string) (reflect.Value, bool) {
	var key struct {
		fromType reflect.Type
		toType   string
	}
	key.fromType = parentValue.Type()
	key.toType = toType

	methodIndex := childMethodTypeCache.GetOrElseUpdate(key, func() interface{} {
		needle := normalizeMethodName("To" + toType)
		for methodIndex := 0; methodIndex < key.fromType.NumMethod(); methodIndex++ {
			method := normalizeMethodName(key.fromType.Method(methodIndex).Name)
			if needle == method {
				if key.fromType.Method(methodIndex).Type.NumIn() != 1 {
					continue
				}
				if key.fromType.Method(methodIndex).Type.NumOut() != 2 {
					continue
				}
				if key.fromType.Method(methodIndex).Type.Out(1) != reflect.TypeOf(true) {
					continue
				}
				return methodIndex
			}
		}
		return -1
	}).(int)
	if methodIndex == -1 {
		return reflect.Value{}, false
	}
	out := parentValue.Method(methodIndex).Call(nil)
	return out[0], out[1].Bool()
}
