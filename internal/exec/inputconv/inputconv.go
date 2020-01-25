package inputconv

import (
    "fmt"
    "github.com/chirino/graphql/schema"
)

type TypeConverters map[string]func(t schema.Type, value interface{}) (interface{}, error)

func (tc TypeConverters) Convert(t schema.Type, value interface{}) (interface{}, error) {

    // Dive into nested children first using recursion...
    switch t := t.(type) {
    case *schema.Scalar: // no children...
    case *schema.NonNull:
        cv, err := tc.Convert(t.OfType, value)
        if err != nil {
            return nil, err
        }
        value = cv
    case *schema.List:
        value := value.([]interface{})
        for i, cv := range value {
            cv, err := tc.Convert(t.OfType, cv)
            if err != nil {
                return nil, err
            }
            value[i] = cv
        }
    case *schema.InputObject:
        value := value.(map[string]interface{})
        converted := make(map[string]interface{}, len(value))
        for _, field := range t.Values {
            fieldName := field.Name.Name
            cv := value[fieldName]
            cv, err := tc.Convert(field.Type, cv)
            if err != nil {
                return nil, err
            }
            converted[fieldName] = cv
        }
        value = converted
    default:
        panic(fmt.Sprintf("convert not implemented for type %T: ", t))
    }

    // then convert values on the way out...
    converter := tc[t.String()]
    if converter != nil {
        v, err := converter(t, value)
        if err != nil {
            return nil, err
        }
        value = v
    }
    return value, nil
}
