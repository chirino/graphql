package api

import (
    "bytes"
    "fmt"
    "github.com/chirino/graphql/resolvers"
    "github.com/chirino/graphql/schema"
    "github.com/getkin/kin-openapi/openapi3"
    "github.com/kr/text"
    "github.com/pkg/errors"
    "os"
    "sort"
    "strings"
    "text/template"
)

type operation struct {
    definition *openapi3.Operation
    status     string
    fieldName  string
    method     string
    path       string
}

type resolverFactory struct {
    options        ApiResolverOptions
    queryFields    map[string]*operation
    mutationFields map[string]*operation
}

var _ resolvers.ResolverFactory = &resolverFactory{}

func NewResolverFactory(doc *openapi3.Swagger, options ApiResolverOptions) (resolvers.ResolverFactory, string, error) {
    result := resolverFactory{options: options}
    result.queryFields = make(map[string]*operation)
    result.mutationFields = make(map[string]*operation)
    if result.options.Logs == nil {
        result.options.Logs = os.Stderr
    }
    queryMethods := map[string]bool{"GET": true, "HEAD": true}
    for path, v := range doc.Paths {
        for method, o := range v.Operations() {
            fieldName := sanitizeGraphQLID(path)
            if o.OperationID != "" {
                fieldName = sanitizeGraphQLID(o.OperationID)
            }
            operation := &operation{definition: o, fieldName: fieldName, method: method, path: path}
            if queryMethods[method] {
                result.queryFields[fieldName] = operation
            } else {
                result.mutationFields[fieldName] = operation
            }
        }
    }

    schema, err := result.schema()
    if err != nil {
        return nil, "", err
    }
    return result, schema, nil
}

func sanitizeGraphQLID(id string) string {
    // valid ids have match this regex: `/^[_a-zA-Z][_a-zA-Z0-9]*$/`
    if id == "" {
        return id
    }
    buf := []byte(id)
    c := buf[0]
    if !(('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '_') {
        buf[0] = '_'
    }
    for i := 1; i < len(buf); i++ {
        c = buf[i]
        if !(('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') || c == '_') {
            buf[i] = '_'
        }
    }
    return string(buf)
}

func (factory resolverFactory) schema() (string, error) {

    // We will be adding types to s
    s := schema.New()
    err := s.Parse(`
        directive @openapi(ref: String) on OBJECT | FIELD_DEFINITION | INPUT_FIELD_DEFINITION | INPUT_OBJECT
    `)
    if err != nil {
        return "", err
    }

    refCache := map[string]interface{}{}
    err = factory.addToSchema(s, factory.options.QueryType, factory.queryFields, refCache)
    if err != nil {
        return "", err
    }

    err = factory.addToSchema(s, factory.options.MutationType, factory.mutationFields, refCache)
    if err != nil {
        return "", err
    }

    // Sort the type fields since we generated them by mutating..
    // which leads to then being in a random order based on the random order
    // they are received from the openapi doc.
    for _, t := range s.Types {
        if t, ok := t.(*schema.Object); ok {
            sort.Slice(t.Fields, func(i, j int) bool {
                return t.Fields[i].Name < t.Fields[j].Name
            })
        }
    }

    // Lets get the graphql schema encoding for it...
    return s.String(), nil
}

func (factory resolverFactory) addToSchema(s *schema.Schema, rootType string, operations map[string]*operation, refCache map[string]interface{}) error {
    fields := []string{}
    path := rootType
outer:
    for _, o := range operations {
        path := path + "/" + UpperFirst(o.fieldName)

        field := toDescription(o.definition.Description)
        field += o.fieldName
        field += "("

        argNames := map[string]bool{}
        addComma := false
        if o.definition.RequestBody != nil {
            content := o.definition.RequestBody.Value.Content.Get("application/json")
            if content != nil {
                argName := makeUnique(argNames, sanitizeGraphQLID(strings.ToLower(o.method)))
                field += argName
                field += ": "
                fieldType, err := factory.addGraphQLType(s, content.Schema, path+"/Body", refCache, true)
                if err != nil {
                    fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: required parameter '%s' type cannot be converted: %s\n", rootType, o.fieldName, "body", err)
                    continue outer
                }
                field += requiredWrapper(fieldType, true)
                addComma = true
            }
        }

        if len(o.definition.Parameters) > 0 {
            for i, param := range o.definition.Parameters {
                if addComma {
                    field += ",\n"
                } else {
                    field += "\n"
                }
                field += toDescription(param.Value.Description)
                argName := makeUnique(argNames, sanitizeGraphQLID(param.Value.Name))
                field += argName
                field += ": "
                fieldType, err := factory.addGraphQLType(s, param.Value.Schema, fmt.Sprintf("%s/Arg/%d", path, i), refCache, true)
                if err != nil {
                    if param.Value.Required {
                        fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: required parameter '%s' type cannot be converted: %s\n", rootType, o.fieldName, param.Value.Name, err)
                        continue outer
                    } else {
                        fmt.Fprintf(factory.options.Logs, "dropping optional %s.%s field parameter: parameter '%s' type cannot be converted: %s\n", rootType, o.fieldName, param.Value.Name, err)
                        continue
                    }
                }
                field += requiredWrapper(fieldType, param.Value.Required)
                addComma = true
            }
        }

        field += ")"
        field += ": "

        for status, response := range o.definition.Responses {
            content := response.Value.Content.Get("application/json")
            if strings.HasPrefix(status, "2") && content != nil {
                o.status = status
                qlType, err := factory.addGraphQLType(s, content.Schema, fmt.Sprintf("%s/DefaultResponse", path), refCache, false)
                if err != nil {
                    fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: result type cannot be converted: %s\n", rootType, o.fieldName, err)
                    continue outer
                }
                field += qlType
                fields = append(fields, field)
                break
            }
        }
    }

    vars := map[string]interface{}{}
    vars["Name"] = rootType
    vars["Fields"] = fields
    gql, err := renderTemplate(vars, `
type {{.Name}} @graphql(alter:"add") {
{{- range $k, $field :=  .Fields }}

{{$field}}

{{- end }}
}
`)
    if err != nil {
        return err
    }
    err = s.Parse(gql)
    if err != nil {
        return err
    }
    return nil
}

func makeUnique(existing map[string]bool, name string) string {
    cur := name
    for i := 1; existing[cur]; i++ {
        cur = fmt.Sprintf("%s%d", name, i)
    }
    existing[cur] = true
    return cur
}

func requiredWrapper(qlType string, required bool) string {
    if required {
        return qlType + "!"
    }
    return qlType
}

func UpperFirst(name string) string {
    if name == "" {
        return ""
    }
    return strings.ToUpper(name[0:1]) + name[1:]
}

func (factory resolverFactory) addGraphQLType(s *schema.Schema, sf *openapi3.SchemaRef, path string, refCache map[string]interface{}, inputType bool) (string, error) {
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
        nestedType, err := factory.addGraphQLType(s, sf.Value.Items, path, refCache, inputType)
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
        typeName = sanitizeGraphQLID(typeName)

        vars := map[string]interface{}{}
        vars["Description"] = toDescription(sf.Value.Description)
        vars["Name"] = typeName
        fields := []string{}

        // In case a type is recursive.. lets stick it in the cache now before we try to resolve it's fields..
        refCache[cacheKey] = typeName

        for name, ref := range sf.Value.Properties {
            field := toDescription(ref.Value.Description)
            fieldType, err := factory.addGraphQLType(s, ref, path+"/"+UpperFirst(name), refCache, inputType)
            if err != nil {
                fmt.Fprintf(factory.options.Logs, "dropping openapi field '%s' from graphql type '%s': %s\n", name, typeName, err)
                continue
            }
            field += sanitizeGraphQLID(name) + ": " + fieldType
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
        err = s.Parse(gql)
        if err != nil {
            refCache[cacheKey] = err
            return "", err
        }

        refCache[cacheKey] = typeName
        return typeName, nil
    default:
        err := errors.New(fmt.Sprintf("cannot convert to a graphql type '%s' ", sf.Value.Type))
        refCache[cacheKey] = err
        return "", err

    }

}
func toDescription(desc string) string {
    if desc == "" {
        return ""
    }
    if !strings.HasSuffix(desc, "\n") {
        desc += "\n"
    }
    return text.Indent(desc, "# ")
}

func renderTemplate(variables interface{}, templateText string) (string, error) {
    buf := bytes.Buffer{}
    tmpl, err := template.New("template").Parse(templateText)
    if err != nil {
        return "", errors.WithStack(err)
    }
    err = tmpl.Execute(&buf, variables)
    if err != nil {
        return "", errors.WithStack(err)
    }
    result := buf.String()
    return result, nil
}
