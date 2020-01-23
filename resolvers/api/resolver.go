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
        //if m.Match(k) {
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

    schema, err := result.Schema()
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

func (factory resolverFactory) Schema() (string, error) {
    s := schema.New()
    err := s.Parse(`
        directive @openapi(ref: String) on OBJECT | FIELD_DEFINITION | INPUT_FIELD_DEFINITION | INPUT_OBJECT
    `)
    if err != nil {
        return "", err
    }

    vars := map[string]interface{}{}
    vars["Name"] = factory.options.QueryType
    fields := []string{}

    refCache := map[string]interface{}{}

    path := factory.options.QueryType
outer:
    for _, o := range factory.queryFields {
        path := path + UpperFirst(o.fieldName)

        field := toDescription(o.definition.Description)
        field += o.fieldName
        if len(o.definition.Parameters) > 0 {
            field += "("
            for i, param := range o.definition.Parameters {
                if i != 0 {
                    field += ",\n"
                } else {
                    field += "\n"
                }
                field += toDescription(param.Value.Description)
                field += sanitizeGraphQLID(param.Value.Name)
                field += ": "
                fieldType, err := factory.addGraphQLType(s, param.Value.Schema, fmt.Sprintf("%s/Arg/%d", path, i), refCache)
                if err != nil {
                    fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: parameter '%s' type cannot be converted: %s\n", factory.options.QueryType, o.fieldName, param.Value.Name, err)
                    continue outer
                }
                field += requiredWrapper(fieldType, param.Value.Required)
            }
            field += ")"
        }
        field += ": "

        for status, response := range o.definition.Responses {
            content := response.Value.Content.Get("application/json")
            if strings.HasPrefix(status, "2") && content !=nil {
                o.status = status
                qlType, err := factory.addGraphQLType(s, content.Schema, fmt.Sprintf("%s/DefaultResponse", path), refCache)
                if err != nil {
                    fmt.Fprintf(factory.options.Logs, "dropping %s.%s field: result type cannot be converted: %s\n", factory.options.QueryType, o.fieldName, err)
                    continue outer
                }
                field += qlType
                fields = append(fields, field)
                break;
            }
        }
    }

    vars["Fields"] = fields
    gql, err := renderTemplate(vars, `
type {{.Name}} @graphql(alter:"add") {
{{- range $k, $field :=  .Fields }}

{{$field}}

{{- end }}
}
`)
    if err != nil {
        return "", err
    }
    err = s.Parse(gql)
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

    return s.String(), nil
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

func (factory resolverFactory) addGraphQLType(s *schema.Schema, sf *openapi3.SchemaRef, path string, refCache map[string]interface{}) (string, error) {
    if sf.Value == nil {
        panic("a schema reference was not resolved.")
    }

    if sf.Ref != "" {
        path = sf.Ref
    }
    if v, ok := refCache[path]; ok {
        if v, ok := v.(string); ok {
            return v, nil
        }
        return "", v.(error)
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
        nestedType, err := factory.addGraphQLType(s, sf.Value.Items, path, refCache)
        if err != nil {
            return "", err
        }
        return fmt.Sprintf("[%s]", nestedType), nil
    case "object":
        typeName := path
        if sf.Ref != "" {
            base := sanitizeGraphQLID(strings.TrimPrefix(sf.Ref, "#/components/schemas/"))
            for i := 0; ; i++ {
                name := base
                if i > 0 {
                    name = fmt.Sprintf("%s%d", base, i)
                }
                o := s.Types[name]
                if o != nil {
                    // found one, but is it a conflict?
                    if o, ok := o.(*schema.Object); ok {
                        d := o.Directives.Get("openapi")
                        if d != nil {
                            ref, _ := d.Args.Get("ref")
                            value := ref.Value(nil).(string)
                            if value == sf.Ref {

                                refCache[sf.Ref] = name
                                // Ding Ding.. it's to the same openapi type..
                                return name, nil
                            }
                        }
                    }
                    // looks like a conflict..
                    continue // to the next name attempt.
                } else {
                    // not found.. lets define it...
                    typeName = name
                    break
                }
            }
        } else {
            typeName = sanitizeGraphQLID(typeName)
        }

        vars := map[string]interface{}{}
        vars["Description"] = toDescription(sf.Value.Description)
        vars["Name"] = typeName
        fields := []string{}

        // In case a type is recursive.. lets stick it in the cache now before we try to resolve it's fields..
        refCache[path] = typeName
        for name, ref := range sf.Value.Properties {
            field := toDescription(ref.Value.Description)
            fieldType, err := factory.addGraphQLType(s, ref, path+"/"+UpperFirst(name), refCache)
            if err != nil {
                fmt.Fprintf(factory.options.Logs, "dropping openapi field '%s' from graphql type '%s': %s\n", name, typeName, err)
                continue
            }
            field += sanitizeGraphQLID(name) + ": " + fieldType
            fields = append(fields, field)
        }

        if len(fields) == 0 {
            err := errors.New(fmt.Sprintf("graphql type '%s' would have no fields", typeName))
            refCache[path] = err
            return "", err
        }

        vars["Fields"] = fields
        vars["Ref"] = sf.Ref
        gql, err := renderTemplate(vars,
            `
{{.Description}}

type {{.Name}} @graphql(alter:"add") @openapi(ref:"{{.Ref}}") {
{{- range $k, $field :=  .Fields }}
{{$field}}
{{- end }}
}
`, )
        if err != nil {
            refCache[path] = err
            return "", err
        }
        err = s.Parse(gql)
        if err != nil {
            refCache[path] = err
            return "", err
        }

        refCache[path] = typeName
        return typeName, nil
    default:
        err := errors.New(fmt.Sprintf("cannot convert to a graphql type '%s' ", sf.Value.Type))
        refCache[path] = err
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
