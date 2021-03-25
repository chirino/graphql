package schema

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"text/scanner"

	"github.com/chirino/graphql/internal/lexer"
	"github.com/chirino/graphql/qerrors"
)

type Description = lexer.Description
type Location = lexer.Location

func NewDescription(text string) Description {
	return Description{
		Text: text,
	}
}

// Schema represents a GraphQL service's collective type system capabilities.
// A schema is defined in terms of the types and directives it supports as well as the root
// operation types for each kind of operation: `query`, `mutation`, and `subscription`.
//
// For a more formal definition, read the relevant section in the specification:
//
// http://facebook.github.io/graphql/draft/#sec-Schema
type Schema struct {

	// These are directives applied to the schema.
	Directives DirectiveList

	// EntryPoints determines the place in the type system where `query`, `mutation`, and
	// `subscription` operations begin.
	//
	// http://facebook.github.io/graphql/draft/#sec-Root-Operation-Types
	//
	// NOTE: The specification refers to this concept as "Root Operation Types".
	// TODO: Rename the `EntryPoints` field to `RootOperationTypes` to align with spec terminology.
	EntryPoints map[OperationType]NamedType

	// Types are the fundamental unit of any GraphQL schema.
	// There are six kinds of named types, and two wrapping types.
	//
	// http://facebook.github.io/graphql/draft/#sec-Types
	Types map[string]NamedType

	// TODO: Type extensions?
	// http://facebook.github.io/graphql/draft/#sec-Type-Extensions

	// Directives are used to annotate various parts of a GraphQL document as an indicator that they
	// should be evaluated differently by a validator, executor, or client tool such as a code
	// generator.
	//
	// http://facebook.github.io/graphql/draft/#sec-Type-System.Directives
	DeclaredDirectives map[string]*DirectiveDecl

	EntryPointNames map[OperationType]string
}

type HasDirectives interface {
	Type
	GetDirectives() DirectiveList
}

type Formatter interface {
	WriteTo(out io.StringWriter)
}

func FormatterToString(s Formatter) string {
	buf := &bytes.Buffer{}
	s.WriteTo(buf)
	return buf.String()
}

type Type interface {
	Kind() string
	String() string
	AddIfMissing(to *Schema, from *Schema)
	Formatter
}

type List struct {
	OfType Type
}

type NonNull struct {
	OfType Type
}

type TypeName struct {
	Name    string
	NameLoc Location
}

// Resolve a named type in the schema by its name.
func (s *Schema) Resolve(name string) Type {
	return s.Types[name]
}

// NamedType represents a type with a name.
//
// http://facebook.github.io/graphql/draft/#NamedType
type NamedType interface {
	Type
	TypeName() string
	Description() string
}

// Scalar types represent primitive leaf values (e.g. a string or an integer) in a GraphQL type
// system.
//
// GraphQL responses take the form of a hierarchical tree; the leaves on these trees are GraphQL
// scalars.
//
// http://facebook.github.io/graphql/draft/#sec-Scalars
type Scalar struct {
	Name       string
	Desc       Description
	Directives DirectiveList
	// TODO: Add a list of directives?
}

// Object types represent a list of named fields, each of which yield a value of a specific type.
//
// GraphQL queries are hierarchical and composed, describing a tree of information.
// While Scalar types describe the leaf values of these hierarchical types, Objects describe the
// intermediate levels.
//
// http://facebook.github.io/graphql/draft/#sec-Objects
type Object struct {
	Name       string
	Interfaces InterfaceList
	Fields     FieldList `json:"fields"`
	Desc       Description
	Directives DirectiveList

	InterfaceNames []string
}

// Interface types represent a list of named fields and their arguments.
//
// GraphQL objects can then implement these interfaces which requires that the object type will
// define all fields defined by those interfaces.
//
// http://facebook.github.io/graphql/draft/#sec-Interfaces
type Interface struct {
	Name          string
	PossibleTypes []*Object
	Fields        FieldList // NOTE: the spec refers to this as `FieldsDefinition`.
	Desc          Description
	Directives    DirectiveList
}

type InterfaceList []*Interface

func (l InterfaceList) Get(name string) *Interface {
	for _, d := range l {
		if d.Name == name {
			return d
		}
	}
	return nil

}
func (l InterfaceList) Select(keep func(d *Interface) bool) InterfaceList {
	rc := InterfaceList{}
	for _, d := range l {
		if keep(d) {
			rc = append(rc, d)
		}
	}
	return rc
}

func StringListGet(l []string, name string) *string {
	for _, d := range l {
		if d == name {
			return &d
		}
	}
	return nil
}

func StringListSelect(l []string, keep func(d string) bool) []string {
	rc := []string{}
	for _, d := range l {
		if keep(d) {
			rc = append(rc, d)
		}
	}
	return rc
}

// Union types represent objects that could be one of a list of GraphQL object types, but provides no
// guaranteed fields between those types.
//
// They also differ from interfaces in that object types declare what interfaces they implement, but
// are not aware of what unions contain them.
//
// http://facebook.github.io/graphql/draft/#sec-Unions
type Union struct {
	Name          string
	PossibleTypes []*Object // NOTE: the spec refers to this as `UnionMemberTypes`.
	Desc          Description
	TypeNames     []string
	Directives    DirectiveList
}

// Enum types describe a set of possible values.
//
// Like scalar types, Enum types also represent leaf values in a GraphQL type system.
//
// http://facebook.github.io/graphql/draft/#sec-Enums
type Enum struct {
	Name       string
	Values     []*EnumValue // NOTE: the spec refers to this as `EnumValuesDefinition`.
	Desc       Description
	Directives DirectiveList
}

// EnumValue types are unique values that may be serialized as a string: the name of the
// represented value.
//
// http://facebook.github.io/graphql/draft/#EnumValueDefinition
type EnumValue struct {
	Name       string
	Directives DirectiveList
	Desc       Description
	// TODO: Add a list of directives?
}

// InputObject types define a set of input fields; the input fields are either scalars, enums, or
// other input objects.
//
// This allows arguments to accept arbitrarily complex structs.
//
// http://facebook.github.io/graphql/draft/#sec-Input-Objects
type InputObject struct {
	Name       string
	Desc       Description
	Fields     InputValueList
	Directives DirectiveList
}

// FieldsList is a list of an Object's Fields.
//
// http://facebook.github.io/graphql/draft/#FieldsDefinition
type FieldList []*Field

// Get iterates over the field list, returning a pointer-to-Field when the field name matches the
// provided `name` argument.
// Returns nil when no field was found by that name.
func (l FieldList) Get(name string) *Field {
	for _, f := range l {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// Names returns a string slice of the field names in the FieldList.
func (l FieldList) Names() []string {
	names := make([]string, len(l))
	for i, f := range l {
		names[i] = f.Name
	}
	return names
}

func (l FieldList) Select(keep func(d *Field) bool) FieldList {
	rc := FieldList{}
	for _, d := range l {
		if keep(d) {
			rc = append(rc, d)
		}
	}
	return rc
}

// http://facebook.github.io/graphql/draft/#sec-Type-System.Directives
type DirectiveDecl struct {
	Name string
	Desc Description
	Locs []string
	Args InputValueList
}

func (*Schema) Kind() string { return "SCHEMA" }
func (*Field) Kind() string  { return "FIELD_DEFINITION" }

func (*List) Kind() string         { return "LIST" }
func (*NonNull) Kind() string      { return "NON_NULL" }
func (*TypeName) Kind() string     { panic("TypeName needs to be resolved to actual type") }
func (*Scalar) Kind() string       { return "SCALAR" }
func (*Object) Kind() string       { return "OBJECT" }
func (*Interface) Kind() string    { return "INTERFACE" }
func (*Union) Kind() string        { return "UNION" }
func (*Enum) Kind() string         { return "ENUM" }
func (*InputObject) Kind() string  { return "INPUT_OBJECT" }
func (t *InputValue) Kind() string { return "INPUT_FIELD_DEFINITION" }
func (t *EnumValue) Kind() string  { return "ENUM_VALUE" }

func (s *Schema) String() string {
	return FormatterToString(s)
}
func (t *Field) String() string       { return t.Name }
func (t *List) String() string        { return "[" + t.OfType.String() + "]" }
func (t *NonNull) String() string     { return t.OfType.String() + "!" }
func (t *TypeName) String() string    { return t.Name }
func (t *Scalar) String() string      { return t.Name }
func (t *Object) String() string      { return t.Name }
func (t *Interface) String() string   { return t.Name }
func (t *Union) String() string       { return t.Name }
func (t *Enum) String() string        { return t.Name }
func (t *InputObject) String() string { return t.Name }
func (t *InputValue) String() string  { return t.Name }
func (t *EnumValue) String() string   { return t.Name }

func (t *Scalar) TypeName() string      { return t.Name }
func (t *Object) TypeName() string      { return t.Name }
func (t *Interface) TypeName() string   { return t.Name }
func (t *Union) TypeName() string       { return t.Name }
func (t *Enum) TypeName() string        { return t.Name }
func (t *InputObject) TypeName() string { return t.Name }

func (t *Scalar) Description() string      { return t.Desc.String() }
func (t *Object) Description() string      { return t.Desc.String() }
func (t *Interface) Description() string   { return t.Desc.String() }
func (t *Union) Description() string       { return t.Desc.String() }
func (t *Enum) Description() string        { return t.Desc.String() }
func (t *InputObject) Description() string { return t.Desc.String() }

func (t *Schema) GetDirectives() DirectiveList    { return t.Directives }
func (t *Object) GetDirectives() DirectiveList    { return t.Directives }
func (t *Field) GetDirectives() DirectiveList     { return t.Directives }
func (t *EnumValue) GetDirectives() DirectiveList { return t.Directives }

func (t *Scalar) GetDirectives() DirectiveList      { return t.Directives }
func (t *InputValue) GetDirectives() DirectiveList  { return t.Directives }
func (t *Interface) GetDirectives() DirectiveList   { return t.Directives }
func (t *Union) GetDirectives() DirectiveList       { return t.Directives }
func (t *Enum) GetDirectives() DirectiveList        { return t.Directives }
func (t *InputObject) GetDirectives() DirectiveList { return t.Directives }

// Field is a conceptual function which yields values.
// http://facebook.github.io/graphql/draft/#FieldDefinition
type Field struct {
	Name       string         `json:"name"`
	Args       InputValueList `json:"args"` // NOTE: the spec refers to this as `ArgumentsDefinition`.
	Type       Type
	Directives DirectiveList
	Desc       Description `json:"desc"`
	Extension  interface{} `json:"-"`
}

// New initializes an instance of Schema.
func New() *Schema {
	s := &Schema{
		EntryPointNames:    make(map[OperationType]string),
		Types:              make(map[string]NamedType),
		DeclaredDirectives: make(map[string]*DirectiveDecl),
		EntryPoints:        make(map[OperationType]NamedType),
	}
	for n, t := range Meta.Types {
		s.Types[n] = t
	}
	for n, d := range Meta.DeclaredDirectives {
		s.DeclaredDirectives[n] = d
	}
	return s
}

// Parse the schema string.
func (s *Schema) Parse(schemaString string) error {
	l := lexer.Get(schemaString)
	err := l.CatchSyntaxError(func() { parseSchema(s, l) })
	lexer.Put(l)
	if err != nil {
		return err
	}
	return s.ResolveTypes()
}

func (s *Schema) ResolveTypes() error {

	objects := []*Object{}
	unions := []*Union{}
	enums := []*Enum{}

	for _, t := range s.Types {
		if err := resolveNamedType(s, t); err != nil {
			return err
		}
		switch t := t.(type) {
		case *Object:
			sort.Slice(t.Fields, func(i, j int) bool {
				return t.Fields[i].Name < t.Fields[j].Name
			})
			sort.Slice(t.Directives, func(i, j int) bool {
				return t.Directives[i].Name < t.Directives[j].Name
			})
			sort.Slice(t.InterfaceNames, func(i, j int) bool {
				return t.InterfaceNames[i] < t.InterfaceNames[j]
			})
			objects = append(objects, t)

		case *Interface:
			sort.Slice(t.Directives, func(i, j int) bool {
				return t.Directives[i].Name < t.Directives[j].Name
			})
			sort.Slice(t.Fields, func(i, j int) bool {
				return t.Fields[i].Name < t.Fields[j].Name
			})

		case *InputObject:
			sort.Slice(t.Directives, func(i, j int) bool {
				return t.Directives[i].Name < t.Directives[j].Name
			})
			sort.Slice(t.Fields, func(i, j int) bool {
				return t.Fields[i].Name < t.Fields[j].Name
			})

		case *Union:
			sort.Slice(t.Directives, func(i, j int) bool {
				return t.Directives[i].Name < t.Directives[j].Name
			})
			sort.Slice(t.TypeNames, func(i, j int) bool {
				return t.TypeNames[i] < t.TypeNames[j]
			})
			unions = append(unions, t)
		case *Enum:
			sort.Slice(t.Directives, func(i, j int) bool {
				return t.Directives[i].Name < t.Directives[j].Name
			})
			sort.Slice(t.Values, func(i, j int) bool {
				return t.Values[i].Name < t.Values[j].Name
			})
			enums = append(enums, t)
		}
	}
	for _, d := range s.DeclaredDirectives {
		for _, arg := range d.Args {
			t, err := ResolveType(arg.Type, s.Resolve)
			if err != nil {
				return err
			}
			arg.Type = t
		}
	}

	for key, name := range s.EntryPointNames {
		t, ok := s.Types[name]
		if !ok {
			if !ok {
				return qerrors.New("type %q not found", name)
			}
		}
		s.EntryPoints[key] = t
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Name < objects[j].Name
	})
	for _, obj := range objects {
		obj.Interfaces = make([]*Interface, len(obj.InterfaceNames))
		for i, intfName := range obj.InterfaceNames {
			t, ok := s.Types[intfName]
			if !ok {
				return qerrors.New("interface %q not found", intfName)
			}
			intf, ok := t.(*Interface)
			if !ok {
				return qerrors.New("type %q is not an interface", intfName)
			}
			obj.Interfaces[i] = intf
			intf.PossibleTypes = append(intf.PossibleTypes, obj)
		}
	}
	for _, union := range unions {
		union.PossibleTypes = make([]*Object, len(union.TypeNames))
		for i, name := range union.TypeNames {
			t, ok := s.Types[name]
			if !ok {
				return qerrors.New("object type %q not found", name)
			}
			obj, ok := t.(*Object)
			if !ok {
				return qerrors.New("type %q is not an object", name)
			}
			union.PossibleTypes[i] = obj
		}
		sort.Slice(union.PossibleTypes, func(i, j int) bool {
			return union.PossibleTypes[i].Name < union.PossibleTypes[j].Name
		})
	}

	for _, enum := range enums {
		for _, value := range enum.Values {
			if err := resolveDirectives(s, value.Directives); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Schema) RenameTypes(renamer func(string) string) {
	newTypes := make(map[string]NamedType, len(s.Types))
	for oldName, t := range s.Types {

		// Don't rename built in types.
		if Meta.Types[oldName] != nil {
			newTypes[oldName] = t
			continue
		}

		newName := renamer(oldName)
		switch t := t.(type) {
		case *Object:
			t.Name = newName
			newTypes[newName] = t
		case *Enum:
			t.Name = newName
			newTypes[newName] = t
		case *Union:
			t.Name = newName
			newTypes[newName] = t
		case *Interface:
			t.Name = newName
			newTypes[newName] = t
		case *InputObject:
			t.Name = newName
			newTypes[newName] = t
		case *Scalar:
			t.Name = newName
			newTypes[newName] = t
		}
	}
	s.Types = newTypes
}

/**
 */
func (s *Schema) VisitDirective(directiveDecl *DirectiveDecl, visitor func(directive *Directive, parents ...HasDirectives) error) error {

	var searchSchema, searchScalar, searchObject, searchField, searchArgument, searchInterface, searchUnion,
		searchEnum, searchEnumValue, searchInputObject, searchInputField bool

	for _, loc := range directiveDecl.Locs {
		switch loc {
		case "SCHEMA":
			searchSchema = true
		case "SCALAR":
			searchScalar = true
		case "OBJECT":
			searchObject = true
		case "FIELD_DEFINITION":
			searchField = true
		case "ARGUMENT_DEFINITION":
			searchArgument = true
		case "INTERFACE":
			searchInterface = true
		case "UNION":
			searchUnion = true
		case "ENUM":
			searchEnum = true
		case "ENUM_VALUE":
			searchEnumValue = true
		case "INPUT_OBJECT":
			searchInputObject = true
		case "INPUT_FIELD_DEFINITION":
			searchInputField = true
		}
	}

	process := func(list DirectiveList, parents ...HasDirectives) error {
		d := list.Get(directiveDecl.Name)
		if d != nil {
			err := visitor(d, parents...)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if searchSchema {
		err := process(s.GetDirectives(), s)
		if err != nil {
			return err
		}
	}
	for _, t := range s.Types {

		switch t := t.(type) {
		case *Scalar:
			if searchScalar {
				err := process(t.GetDirectives(), t, s)
				if err != nil {
					return err
				}
			}

		case *Object:
			if searchObject {
				err := process(t.GetDirectives(), t, s)
				if err != nil {
					return err
				}
			}
			if searchField {
				for _, f := range t.Fields {
					err := process(f.GetDirectives(), f, t, s)
					if err != nil {
						return err
					}
				}
			}
			if searchArgument {
				for _, f := range t.Fields {
					for _, a := range f.Args {
						err := process(a.GetDirectives(), a, f, t, s)
						if err != nil {
							return err
						}
					}
				}
			}

		case *Interface:
			if searchInterface {
				err := process(t.GetDirectives(), t, s)
				if err != nil {
					return err
				}
			}
			if searchField {
				for _, f := range t.Fields {
					err := process(f.GetDirectives(), f, t, s)
					if err != nil {
						return err
					}
				}
			}
			if searchArgument {
				for _, f := range t.Fields {
					for _, a := range f.Args {
						err := process(a.GetDirectives(), a, f, t, s)
						if err != nil {
							return err
						}
					}
				}
			}

		case *Union:
			if searchUnion {
				err := process(t.GetDirectives(), t, s)
				if err != nil {
					return err
				}
			}

		case *Enum:
			if searchEnum {
				err := process(t.GetDirectives(), t, s)
				if err != nil {
					return err
				}
			}
			if searchEnumValue {
				for _, f := range t.Values {
					err := process(f.GetDirectives(), f, t, s)
					if err != nil {
						return err
					}
				}
			}

		case *InputObject:
			if searchInputObject {
				err := process(t.GetDirectives(), t, s)
				if err != nil {
					return err
				}
			}
			if searchInputField {
				for _, f := range t.Fields {
					err := process(f.GetDirectives(), f, t, s)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func resolveNamedType(s *Schema, t NamedType) error {
	switch t := t.(type) {
	case *Object:
		for _, f := range t.Fields {
			if err := resolveField(s, f); err != nil {
				return err
			}
		}
	case *Interface:
		for _, f := range t.Fields {
			if err := resolveField(s, f); err != nil {
				return err
			}
		}
	case *InputObject:
		if err := resolveInputObject(s, t.Fields); err != nil {
			return err
		}
	}
	return nil
}

func resolveField(s *Schema, f *Field) error {
	t, err := ResolveType(f.Type, s.Resolve)
	if err != nil {
		return err
	}
	f.Type = t
	if err := resolveDirectives(s, f.Directives); err != nil {
		return err
	}
	return resolveInputObject(s, f.Args)
}

func resolveDirectives(s *Schema, directives DirectiveList) error {
	for _, d := range directives {
		dirName := d.Name
		dd, ok := s.DeclaredDirectives[dirName]
		if !ok {
			return qerrors.New("directive %q not found", dirName)
		}
		for _, arg := range d.Args {
			if dd.Args.Get(arg.Name) == nil {
				return qerrors.New("invalid argument %q for directive %q", arg.Name, dirName)
			}
		}
		for _, arg := range dd.Args {
			if _, ok := d.Args.Get(arg.Name); !ok {
				d.Args = append(d.Args, Argument{Name: arg.Name, Value: arg.Default})
			}
		}
	}
	return nil
}

func resolveInputObject(s *Schema, values InputValueList) error {
	for _, v := range values {
		t, err := ResolveType(v.Type, s.Resolve)
		if err != nil {
			return err
		}
		v.Type = t
	}
	return nil
}

func parseSchema(s *Schema, l *lexer.Lexer) {
	l.Consume()

	for l.Peek() != scanner.EOF {
		desc := l.ConsumeDescription()
		switch x := l.ConsumeIdentIntern(); x {

		case "schema":
			s.Directives = ParseDirectives(l)
			l.ConsumeToken('{')
			for l.Peek() != '}' {
				name := OperationType(l.ConsumeKeyword(string(Query), string(Mutation), string(Subscription)))
				l.ConsumeToken(':')
				typ := l.ConsumeIdentIntern()
				s.EntryPointNames[name] = typ
			}
			l.ConsumeToken('}')

		case "type":
			obj := parseObjectDef(l)
			obj.Desc = desc

			d := obj.Directives.Get("graphql")
			if d != nil {

				// Drop this directive.. since we are processing it here.
				obj.Directives = obj.Directives.Select(func(d2 *Directive) bool {
					return d2 != d
				})

				if mode, ok := d.Args.Get("alter"); ok {
					switch mode.Evaluate(nil) {
					case "add":
						existing := s.Types[obj.Name]
						if existing == nil {
							s.Types[obj.Name] = obj
						} else {
							existing, ok := existing.(*Object)
							if !ok {
								l.SyntaxError(fmt.Sprintf("Cannot update %s, it was a %s", obj.Name, existing.Kind()))
							}
							existing.Directives = append(existing.Directives, obj.Directives...)
							existing.Fields = append(existing.Fields, obj.Fields...)
							if existing.Desc.ShowType == lexer.NoDescription {
								existing.Desc = obj.Desc
							} else if obj.Desc.ShowType != lexer.NoDescription {
								existing.Desc.Text += "\n" + obj.Desc.Text
							}
							existing.InterfaceNames = append(existing.InterfaceNames, obj.InterfaceNames...)
							existing.Interfaces = append(existing.Interfaces, obj.Interfaces...)
						}
					case "drop":
						existing := s.Types[obj.Name]
						if existing != nil {
							existing, ok := existing.(*Object)
							if !ok {
								l.SyntaxError(fmt.Sprintf("Cannot update %s, it was a %s", obj.Name, existing.Kind()))
							}
							existing.Directives = existing.Directives.Select(func(d *Directive) bool {
								return obj.Directives.Get(d.Name) == nil
							})
							existing.Fields = existing.Fields.Select(func(d *Field) bool {
								return obj.Fields.Get(d.Name) == nil
							})
							existing.Interfaces = existing.Interfaces.Select(func(d *Interface) bool {
								return obj.Interfaces.Get(d.Name) == nil
							})
							existing.InterfaceNames = StringListSelect(existing.InterfaceNames, func(d string) bool {
								return StringListGet(obj.InterfaceNames, d) == nil
							})
						}
					default:
						panic(`@graphql alter value must be one of: "add" or "drop"`)
					}
				} else if mode, ok := d.Args.Get("if"); ok {
					switch mode.Evaluate(nil) {
					case "missing":
						existing := s.Types[obj.Name]
						if existing == nil {
							s.Types[obj.Name] = obj
						}
					default:
						panic(`@graphql if value must be: "missing"`)
					}
				} else {
					panic(`@graphql alter or if arguments must be provided`)
				}

			} else {
				s.Types[obj.Name] = obj
			}

		case "interface":
			iface := parseInterfaceDef(l)
			iface.Desc = desc
			s.Types[iface.Name] = iface

		case "union":
			union := parseUnionDef(l)
			union.Desc = desc
			s.Types[union.Name] = union

		case "enum":
			enum := parseEnumDef(l)
			enum.Desc = desc
			s.Types[enum.Name] = enum

		case "input":
			input := parseInputDef(l)
			input.Desc = desc
			s.Types[input.Name] = input

		case "scalar":
			name := l.ConsumeIdentIntern()
			s.Types[name] = &Scalar{
				Name:       name,
				Desc:       desc,
				Directives: ParseDirectives(l),
			}

		case "directive":
			directive := parseDirectiveDef(l)
			directive.Desc = desc
			s.DeclaredDirectives[directive.Name] = directive

		default:
			// TODO: Add support for type extensions.
			l.SyntaxError(fmt.Sprintf(`unexpected %q, expecting "schema", "type", "enum", "interface", "union", "input", "scalar" or "directive"`, x))
		}
	}
}

func parseObjectDef(l *lexer.Lexer) *Object {
	object := &Object{Name: l.ConsumeIdentIntern()}

	if l.PeekKeyword("implements") {
		l.Consume()
		if l.Peek() == '&' {
			l.ConsumeToken('&')
		}
		for {
			object.InterfaceNames = append(object.InterfaceNames, l.ConsumeIdentIntern())
			if l.Peek() == '&' {
				l.ConsumeToken('&')
			} else if l.Peek() == '@' || l.Peek() == '{' {
				break
			}
		}
	}

	object.Directives = ParseDirectives(l)

	l.ConsumeToken('{')
	object.Fields = parseFieldsDef(l)
	l.ConsumeToken('}')

	return object
}

func parseTypeName(l *lexer.Lexer) TypeName {
	name, loc := l.ConsumeIdentInternWithLoc()
	return TypeName{Name: name, NameLoc: loc}
}

func parseInterfaceDef(l *lexer.Lexer) *Interface {
	i := &Interface{Name: l.ConsumeIdentIntern()}
	i.Directives = ParseDirectives(l)
	l.ConsumeToken('{')
	i.Fields = parseFieldsDef(l)
	l.ConsumeToken('}')

	return i
}

func parseUnionDef(l *lexer.Lexer) *Union {
	union := &Union{Name: l.ConsumeIdentIntern()}
	union.Directives = ParseDirectives(l)
	l.ConsumeToken('=')
	union.TypeNames = []string{l.ConsumeIdentIntern()}
	for l.Peek() == '|' {
		l.ConsumeToken('|')
		union.TypeNames = append(union.TypeNames, l.ConsumeIdentIntern())
	}

	return union
}

func parseInputDef(l *lexer.Lexer) *InputObject {
	i := &InputObject{}
	i.Name = l.ConsumeIdentIntern()
	i.Directives = ParseDirectives(l)
	l.ConsumeToken('{')
	for l.Peek() != '}' {
		i.Fields = append(i.Fields, ParseInputValue(l))
	}
	l.ConsumeToken('}')
	return i
}

func parseEnumDef(l *lexer.Lexer) *Enum {
	enum := &Enum{Name: l.ConsumeIdentIntern()}
	enum.Directives = ParseDirectives(l)
	l.ConsumeToken('{')
	for l.Peek() != '}' {
		v := &EnumValue{
			Desc:       l.ConsumeDescription(),
			Name:       l.ConsumeIdentIntern(),
			Directives: ParseDirectives(l),
		}

		enum.Values = append(enum.Values, v)
	}
	l.ConsumeToken('}')
	return enum
}

func parseDirectiveDef(l *lexer.Lexer) *DirectiveDecl {
	l.ConsumeToken('@')
	d := &DirectiveDecl{Name: l.ConsumeIdentIntern()}

	if l.Peek() == '(' {
		l.ConsumeToken('(')
		for l.Peek() != ')' {
			v := ParseInputValue(l)
			d.Args = append(d.Args, v)
		}
		l.ConsumeToken(')')
	}

	l.ConsumeKeyword("on")

	for {
		loc := l.ConsumeIdentIntern()
		d.Locs = append(d.Locs, loc)
		if l.Peek() != '|' {
			break
		}
		l.ConsumeToken('|')
	}
	return d
}

func parseFieldsDef(l *lexer.Lexer) FieldList {
	var fields FieldList
	for l.Peek() != '}' {
		f := &Field{}
		f.Desc = l.ConsumeDescription()
		f.Name = l.ConsumeIdentIntern()
		if l.Peek() == '(' {
			l.ConsumeToken('(')
			for l.Peek() != ')' {
				f.Args = append(f.Args, ParseInputValue(l))
			}
			l.ConsumeToken(')')
		}
		l.ConsumeToken(':')
		f.Type = ParseType(l)
		f.Directives = ParseDirectives(l)
		fields = append(fields, f)
	}
	return fields
}
