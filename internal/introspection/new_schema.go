// retrospection converts from an introspection result to a Schema
package introspection

import (
	"encoding/json"
	"fmt"

	"github.com/chirino/graphql/errors"
	"github.com/chirino/graphql/schema"
)

type named struct {
	Name string `json:"name"`
}
type typeRef struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	OfType *typeRef `json:"ofType"`
}

// A Directive provides a way to describe alternate runtime execution and type validation behavior in a GraphQL document.
//
// In some cases, you need to provide options to alter GraphQL's execution behavior
// in ways field arguments will not suffice, such as conditionally including or
// skipping a field. Directives provide this by describing additional information
// to the executor.
type directive struct {
	Name        string       `json:"name"`
	Description *string      `json:"description"`
	Locations   []string     `json:"locations"`
	Args        []inputValue `json:"args"`
}

// Arguments provided to Fields or Directives and the input fields of an
// InputObject are represented as Input Values which describe their type and
// optionally a default value.
type inputValue struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Type         typeRef `json:"type"`
	DefaultValue *string `json:"defaultValue"`
}

// Object and Interface types are described by a list of Fields, each of which has
// a name, potentially a list of arguments, and a return type.
type field struct {
	Name              string       `json:"name"`
	Description       *string      `json:"description"`
	Args              []inputValue `json:"args"`
	Type              typeRef      `json:"type"`
	IsDeprecated      bool         `json:"isDeprecated"`
	DeprecationReason *string      `json:"deprecationReason"`
}

// One possible value for a given Enum. Enum values are unique values, not a
// placeholder for a string or numeric value. However an Enum value is returned in
// a JSON response as a string.
type enumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

type fullType struct {
	Kind          *string      `json:"kind"`
	Name          string       `json:"name"`
	Description   *string      `json:"description"`
	Fields        []field      `json:"fields"`
	InputFields   []inputValue `json:"inputFields"`
	Interfaces    []typeRef    `json:"interfaces"`
	EnumValues    []enumValue  `json:"enumValues"`
	PossibleTypes []typeRef    `json:"possibleTypes"`
}

// A GraphQL Schema defines the capabilities of a GraphQL server. It exposes all
// available types and directives on the server, as well as the entry points for
// query, mutation, and subscription operations.
type schemaType struct {
	QueryType        named       `json:"queryType"`
	MutationType     *named      `json:"mutationType"`
	SubscriptionType *named      `json:"subscriptionType"`
	Types            []fullType  `json:"types"`
	Directives       []directive `json:"directives"`
}

type introspectionData struct {
	Schema *schemaType `json:"__schema"`
}

const Query = `
 query IntrospectionQuery {
   __schema {
     queryType { name }
     mutationType { name }
     subscriptionType { name }
     types {
       ...FullType
     }
     directives {
       name
       description
       locations
       args {
         ...InputValue
       }
     }
   }
 }
 fragment FullType on __Type {
   kind
   name
   description
   fields(includeDeprecated: true) {
     name
     description
     args {
       ...InputValue
     }
     type {
       ...TypeRef
     }
     isDeprecated
     deprecationReason
   }
   inputFields {
     ...InputValue
   }
   interfaces {
     ...TypeRef
   }
   enumValues(includeDeprecated: true) {
     name
     description
     isDeprecated
     deprecationReason
   }
   possibleTypes {
     ...TypeRef
   }
 }
 fragment InputValue on __InputValue {
   name
   description
   type { ...TypeRef }
   defaultValue
 }
 fragment TypeRef on __Type {
   kind
   name
   ofType {
     kind
     name
     ofType {
       kind
       name
       ofType {
         kind
         name
         ofType {
           kind
           name
           ofType {
             kind
             name
             ofType {
               kind
               name
               ofType {
                 kind
                 name
               }
             }
           }
         }
       }
     }
   }
 }
`

func NewSchema(introspection json.RawMessage) (*schema.Schema, error) {

	data := introspectionData{}
	err := json.Unmarshal(introspection, &data)
	if err != nil {
		return nil, err
	}

	s := schema.New()
	for _, d := range data.Schema.Directives {
		// skip over the meta types
		if schema.Meta.DeclaredDirectives[d.Name] != nil {
			continue
		}

		s.DeclaredDirectives[d.Name] = &schema.DirectiveDecl{
			Desc: desc(d.Description),
			Name: d.Name,
			Args: args(d.Args),
		}
	}
	for _, t := range data.Schema.Types {
		// skip over the meta types
		if schema.Meta.Types[t.Name] != nil {
			continue
		}
		if t.Kind == nil {
			return nil, errors.New("kind not set for type: " + t.Name)
		}
		switch *t.Kind {
		case "OBJECT":
			s.Types[t.Name] = &schema.Object{
				Desc:           desc(t.Description),
				Name:           t.Name,
				InterfaceNames: toSimpleNames(t.Interfaces),
				Fields:         toFields(t.Fields),
			}
		case "INTERFACE":
			s.Types[t.Name] = &schema.Interface{
				Desc:   desc(t.Description),
				Name:   t.Name,
				Fields: toFields(t.Fields),
			}
		case "SCALAR":
			s.Types[t.Name] = &schema.Scalar{
				Desc: desc(t.Description),
				Name: t.Name,
			}
		case "UNION":
			s.Types[t.Name] = &schema.Union{
				Name:      t.Name,
				Desc:      desc(t.Description),
				TypeNames: toSimpleNames(t.PossibleTypes),
			}
		case "ENUM":
			s.Types[t.Name] = &schema.Enum{
				Name:   t.Name,
				Desc:   desc(t.Description),
				Values: toEnumValues(t.EnumValues),
			}
		case "INPUT_OBJECT":
			s.Types[t.Name] = &schema.InputObject{
				Desc:   desc(t.Description),
				Name:   t.Name,
				Fields: toInputFields(t.InputFields),
			}
		default:
			return nil, fmt.Errorf("invalid kind: %s", *t.Kind)
		}
	}

	s.EntryPointNames[schema.Query] = data.Schema.QueryType.Name
	if data.Schema.MutationType != nil {
		s.EntryPointNames[schema.Mutation] = data.Schema.MutationType.Name
	}
	if data.Schema.SubscriptionType != nil {
		s.EntryPointNames[schema.Subscription] = data.Schema.SubscriptionType.Name
	}

	err = s.ResolveTypes()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func toEnumValues(values []enumValue) []*schema.EnumValue {
	r := []*schema.EnumValue{}
	for _, v := range values {
		r = append(r, &schema.EnumValue{
			Desc:       desc(v.Description),
			Name:       v.Name,
			Directives: directives(v.IsDeprecated, v.DeprecationReason),
		})
	}
	return r
}

func directives(deprecated bool, reason *string) schema.DirectiveList {
	return nil
}

func toSimpleNames(interfaces []typeRef) []string {
	r := []string{}
	if len(interfaces) > 0 {
		for _, i := range interfaces {
			r = append(r, i.Name)
		}
	}
	return r
}

func toFields(fields []field) schema.FieldList {
	rc := schema.FieldList{}
	for _, f := range fields {
		rc = append(rc, &schema.Field{
			Desc:       desc(f.Description),
			Name:       f.Name,
			Args:       args(f.Args),
			Type:       toType(f.Type),
			Directives: directives(f.IsDeprecated, f.DeprecationReason),
		})
	}
	return rc
}
func toInputFields(args []inputValue) schema.InputValueList {
	rc := schema.InputValueList{}
	for _, arg := range args {
		rc = append(rc, &schema.InputValue{
			Desc: desc(arg.Description),
			Name: schema.Ident{Text: arg.Name},
			Type: toType(arg.Type),
		})
	}
	return rc
}

func toLiteral(t typeRef, value *string) schema.Literal {
	if value == nil {
		return nil
	}
	//if t.Kind == "ENUM" {
	//}
	return &schema.BasicLit{
		Type: rune((*value)[0]),
		Text: *value,
	}
}

func args(args []inputValue) schema.InputValueList {
	rc := schema.InputValueList{}
	for _, arg := range args {
		rc = append(rc, &schema.InputValue{
			Desc:    desc(arg.Description),
			Name:    schema.Ident{Text: arg.Name},
			Type:    toType(arg.Type),
			Default: toLiteral(arg.Type, arg.DefaultValue),
		})
	}
	return rc
}

func toType(ref typeRef) schema.Type {
	switch ref.Kind {
	case "LIST":
		return &schema.List{OfType: toType(*ref.OfType)}
	case "NON_NULL":
		return &schema.NonNull{OfType: toType(*ref.OfType)}
	default:
		return &schema.TypeName{
			Ident: schema.Ident{Text: ref.Name},
		}
	}
}

func desc(description *string) *schema.Description {
	if description == nil {
		return nil
	}
	return &schema.Description{Text: *description}
}
