package api

import (
    "fmt"
    "github.com/chirino/graphql/resolvers"
    "github.com/chirino/graphql/schema"
    "github.com/getkin/kin-openapi/openapi3"
    "github.com/pkg/errors"
    "os"
    "reflect"
    "sort"
    "strings"
)

type ResolverHook struct {
    graphType  string
    graphField string
}

type Converter func(value reflect.Value, err error) (reflect.Value, error)

type resolverFactory struct {
    next       resolvers.ResolverFactory
    options    ApiResolverOptions
    resolvers  map[string]resolvers.ResolverFactory
    converters map[string]Converter
}

var _ resolvers.ResolverFactory = &resolverFactory{}

func NewResolverFactory(doc *openapi3.Swagger, options ApiResolverOptions) (resolvers.ResolverFactory, string, error) {
    result := &resolverFactory{options: options}
    result.next = resolvers.DynamicResolverFactory()
    result.resolvers = make(map[string]resolvers.ResolverFactory)
    result.converters = make(map[string]Converter)

    if result.options.Logs == nil {
        result.options.Logs = os.Stderr
    }
    queryMethods := map[string]bool{"GET": true, "HEAD": true}

    draftSchema := schema.New()
    err := draftSchema.Parse(`
        directive @openapi(ref: String) on OBJECT | FIELD_DEFINITION | INPUT_FIELD_DEFINITION | INPUT_OBJECT
    `)
    if err != nil {
        return nil, "", err
    }

    refCache := map[string]interface{}{}
    for path, v := range doc.Paths {
        for method, operation := range v.Operations() {
            if queryMethods[method] {
                err := result.addRootField(draftSchema, options.QueryType, operation, refCache, method, path)
                if err != nil {
                    fmt.Fprintf(result.options.Logs, "could not map api endpoint '%s %s': %s\n", method, path, err)
                }
            } else {
                err := result.addRootField(draftSchema, options.MutationType, operation, refCache, method, path)
                if err != nil {
                    fmt.Fprintf(result.options.Logs, "could not map api endpoint '%s %s': %s\n", method, path, err)
                }
            }
        }
    }

    // Sort the type fields since we generated them by mutating..
    // which leads to then being in a random order based on the random order
    // they are received from the openapi doc.
    for _, t := range draftSchema.Types {
        if t, ok := t.(*schema.Object); ok {
            sort.Slice(t.Fields, func(i, j int) bool {
                return t.Fields[i].Name < t.Fields[j].Name
            })
        }
        if t, ok := t.(*schema.InputObject); ok {
            sort.Slice(t.Values, func(i, j int) bool {
                return t.Values[i].Name.Name < t.Values[j].Name.Name
            })
        }
    }

    return result, draftSchema.String(), nil
}

func (factory resolverFactory) addRootField(draftSchema *schema.Schema, rootType string, operation *openapi3.Operation, refCache map[string]interface{}, method string, path string) error {

    fieldName := sanitizeName(path)
    if operation.OperationID != "" {
        fieldName = sanitizeName(operation.OperationID)
    }

    typePath := rootType + "/" + capitalizeFirstLetter(fieldName)

    field := description(operation.Description + "\n\\\n\\\n**endpoint:** `" + method + " " + path + "`")
    field += fieldName
    field += "("

    generated := map[string]string{}
    argNames := map[string]bool{}
    addComma := false
    if operation.RequestBody != nil {
        content := operation.RequestBody.Value.Content.Get("application/json")
        if content != nil {
            argName := makeUnique(argNames, "body")
            field += argName
            field += ": "
            fieldType, err := factory.addGraphQLType(generated, content.Schema, typePath+"/body", refCache, true)
            if err != nil {
                fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: required parameter '%s' type cannot be converted: %s\n", rootType, fieldName, "body", err)
                return nil
            }
            field += requiredType(fieldType, true)
            addComma = true
        }
    }

    if len(operation.Parameters) > 0 {
        for i, param := range operation.Parameters {
            if addComma {
                field += ",\n"
            } else {
                field += "\n"
            }
            field += description(param.Value.Description)
            argName := makeUnique(argNames, sanitizeName(param.Value.Name))
            field += argName
            field += ": "
            fieldType, err := factory.addGraphQLType(generated, param.Value.Schema, fmt.Sprintf("%s/Arg/%d", typePath, i), refCache, true)
            if err != nil {
                if param.Value.Required {
                    fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: required parameter '%s' type cannot be converted: %s\n", rootType, fieldName, param.Value.Name, err)
                    return nil
                } else {
                    fmt.Fprintf(factory.options.Logs, "dropping optional %s.%s field parameter: parameter '%s' type cannot be converted: %s\n", rootType, fieldName, param.Value.Name, err)
                    continue
                }
            }
            field += requiredType(fieldType, param.Value.Required)
            addComma = true
        }
    }

    field += ")"
    field += ": "

    for status, response := range operation.Responses {
        content := response.Value.Content.Get("application/json")
        if strings.HasPrefix(status, "2") && content != nil {
            qlType, err := factory.addGraphQLType(generated, content.Schema, fmt.Sprintf("%s/DefaultResponse", typePath), refCache, false)
            if err != nil {
                fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: result type cannot be converted: %s\n", rootType, fieldName, err)
                return nil
            }
            field += qlType

            gql := fmt.Sprintf(`type %s @graphql(alter:"add") { %s }`, rootType, field)
            for _, g := range generated {
                gql += "\n " + g
            }
            err = draftSchema.Parse(gql)
            if err != nil {
                return err
            }

            factory.resolvers[rootType+":"+fieldName] = resolvers.Func(func(request *resolvers.ResolveRequest) resolvers.Resolver {
                return factory.resolve(request, operation, method, path, status)
            })

            return nil
        }
    }
    fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: graphql result type could not be determined\n", rootType, fieldName)
    return nil
}

func (factory resolverFactory) addGraphQLType(generated map[string]string, sf *openapi3.SchemaRef, path string, refCache map[string]interface{}, inputType bool) (string, error) {
    if sf.Value == nil {
        panic("a schema reference was not resolved.")
    }

    cacheKey := "o:" + sf.Ref
    if inputType {
        cacheKey = "i:" + sf.Ref
    }
    if sf.Ref != "" {
        if v, ok := refCache[cacheKey]; ok {
            if v, ok := v.(string); ok {
                return v, nil
            }
            return "", v.(error)
        }
    }

    switch sf.Value.Type {
    case "string":
        return "String", nil
    case "integer":
        return "Int", nil
    case "number":
        return "Float", nil
    case "boolean":
        return "Boolean", nil
    case "array":
        nestedType, err := factory.addGraphQLType(generated, sf.Value.Items, path, refCache, inputType)
        if err != nil {
            return "", err
        }
        return fmt.Sprintf("[%s]", nestedType), nil
    case "object":

        typeName := path
        if sf.Ref != "" {
            typeName = strings.TrimPrefix(sf.Ref, "#/components/schemas/")
        }
        if inputType {
            typeName += "Input"
        } else {
            typeName += "Result"
        }
        typeName = sanitizeName(typeName)

        if len(sf.Value.Properties) == 0 && sf.Value.AdditionalProperties != nil {
            nestedType, err := factory.addGraphQLType(generated, sf.Value.AdditionalProperties, path, refCache, inputType)
            if err != nil {
                return "", err
            }
            wrapper, err := factory.addPropWrapper(generated, nestedType, inputType)
            if err != nil {
                return "", err
            }
            return fmt.Sprintf("[%s!]", wrapper), nil
        }

        vars := map[string]interface{}{}
        vars["Description"] = description(sf.Value.Description)
        vars["Name"] = typeName
        fields := []string{}

        // In case a type is recursive.. lets stick it in the cache now before we try to resolve it's fields..
        refCache[cacheKey] = typeName

        for name, ref := range sf.Value.Properties {
            field := description(ref.Value.Description)
            fieldType, err := factory.addGraphQLType(generated, ref, path+"/"+capitalizeFirstLetter(name), refCache, inputType)
            if err != nil {
                fmt.Fprintf(factory.options.Logs, "dropping openapi field '%s' from graphql type '%s': %s\n", name, typeName, err)
                continue
            }
            field += sanitizeName(name) + ": " + fieldType
            fields = append(fields, field)
        }

        if len(fields) == 0 {
            err := errors.New(fmt.Sprintf("graphql type '%s' would have no fields", typeName))
            refCache[cacheKey] = err
            return "", err
        }

        vars["Fields"] = fields
        vars["Ref"] = sf.Ref
        vars["Type"] = "type"
        if inputType {
            vars["Type"] = "input"
        }
        gql, err := renderTemplate(vars,
            `
{{.Description}}
{{.Type}} {{.Name}} {
{{- range $k, $field :=  .Fields }}
{{$field}}
{{- end }}
}
`, )
        if err != nil {
            refCache[cacheKey] = err
            return "", err
        }
        generated[typeName] = gql
        refCache[cacheKey] = typeName
        return typeName, nil

    default:
        err := errors.New(fmt.Sprintf("cannot convert to a graphql type '%s' ", sf.Value.Type))
        refCache[cacheKey] = err
        return "", err

    }

}

func (factory *resolverFactory) addPropWrapper(generated map[string]string, nestedType string, inputType bool) (string, error) {
    nestedTypeLong := toTypeName(nestedType)
    graphType := "type"
    name := nestedTypeLong + "ResultProp"
    if inputType {
        graphType = "input"
        name = nestedTypeLong + "InputProp"
    }
    gql := fmt.Sprintf(`
        %s
        %s %s {
            key: String!
            value: %s
        }
    `, description(`A property entry`),
        graphType, name,
        nestedType)
    generated[name] = gql

    // Lets register a converter for this type....
    factory.converters["["+name+"!]"] = func(value reflect.Value, err error) (reflect.Value, error) {
        if err != nil {
            return value, err
        }
        if value.IsNil() {
            return value, err
        }
        m := value.Interface().(map[string]interface{})
        if m == nil {
            return value, err
        }

        type Prop struct {
            Key   string      `json:"key"`
            Value interface{} `json:"value"`
        }
        props := make([]Prop, len(m))
        i := 0
        for k, v := range m {
            props[i] = Prop{Key: k, Value: v}
            i++
        }
        return reflect.ValueOf(props), nil
    }
    return name, nil
}

func toTypeName(v string) string {
    if strings.HasSuffix(v, "!") {
        return toTypeName(strings.TrimSuffix(v, "!")) + "NN"
    }
    if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
        return toTypeName(strings.TrimSuffix(strings.TrimPrefix(v, "["), "]")) + "Array"
    }
    return v
}
