package inputconv

import (
	"fmt"
	"github.com/chirino/graphql/schema"
)

type TypeConverters map[string]func(t schema.Type, value interface{}) (interface{}, error)

func (tc TypeConverters) Convert(t schema.Type, value interface{}, path string) (interface{}, error) {
	var result interface{} = nil
	// Dive into nested children first using recursion...
	switch t := t.(type) {
	case *schema.Scalar: // no children...
		result = value
	case *schema.NonNull:
		if value == nil {
			panic(fmt.Sprintf("Expecting non null value, but got one. (field path: %s)", path))
		}
		cv, err := tc.Convert(t.OfType, value, path)
		if err != nil {
			return nil, err
		}
		result = cv
	case *schema.List:
		if value == nil {
			break
		}
		value := value.([]interface{})
		for i, cv := range value {
			cv, err := tc.Convert(t.OfType, cv, path)
			if err != nil {
				return nil, err
			}
			value[i] = cv
		}
		result = value
	case *schema.InputObject:
		if value == nil {
			break
		}
		value := value.(map[string]interface{})
		converted := make(map[string]interface{}, len(value))
		for _, field := range t.Fields {
			fieldName := field.Name.Text
			cv := value[fieldName]
			cv, err := tc.Convert(field.Type, cv, path+"/"+fieldName)
			if err != nil {
				return nil, err
			}
			if cv != nil {
				converted[fieldName] = cv
			}
		}
		result = converted
	default:
		panic(fmt.Sprintf("convert not implemented for type %T: ", t))
	}
	// then convert values on the way out...
	converter := tc[t.String()]
	if converter != nil {
		v, err := converter(t, result)
		if err != nil {
			return nil, err
		}
		result = v
	}
	return result, nil
}
