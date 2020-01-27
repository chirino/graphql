package schema

import (
    "fmt"
    "io"
    "text/scanner"

    "github.com/chirino/graphql/errors"
)

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
    EntryPoints map[string]NamedType

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

    entryPointNames map[string]string
    objects         []*Object
    unions          []*Union
    enums           []*Enum
}

type HasDirectives interface {
    GetDirectives() DirectiveList
}

type Type interface {
    Kind() string
    String() string
    WriteSchemaFormat(out io.StringWriter)
}

type List struct {
    OfType Type
}

type NonNull struct {
    OfType Type
}

type TypeName struct {
    Ident
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
    Desc       string
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
    Desc       string
    Directives DirectiveList

    interfaceNames []string
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
    Desc          string
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
    Desc          string
    typeNames     []string
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
    Desc       string
    Directives DirectiveList
}

// EnumValue types are unique values that may be serialized as a string: the name of the
// represented value.
//
// http://facebook.github.io/graphql/draft/#EnumValueDefinition
type EnumValue struct {
    Name       string
    Directives DirectiveList
    Desc       string
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
    Desc       string
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
    Desc string
    Locs []string
    Args InputValueList
}

func (*List) Kind() string        { return "LIST" }
func (*NonNull) Kind() string     { return "NON_NULL" }
func (*TypeName) Kind() string    { panic("TypeName needs to be resolved to actual type") }
func (*Scalar) Kind() string      { return "SCALAR" }
func (*Object) Kind() string      { return "OBJECT" }
func (*Interface) Kind() string   { return "INTERFACE" }
func (*Union) Kind() string       { return "UNION" }
func (*Enum) Kind() string        { return "ENUM" }
func (*InputObject) Kind() string { return "INPUT_OBJECT" }

func (t *List) String() string        { return "[" + t.OfType.String() + "]" }
func (t *NonNull) String() string     { return t.OfType.String() + "!" }
func (*TypeName) String() string      { panic("TypeName needs to be resolved to actual type") }
func (t *Scalar) String() string      { return t.Name }
func (t *Object) String() string      { return t.Name }
func (t *Interface) String() string   { return t.Name }
func (t *Union) String() string       { return t.Name }
func (t *Enum) String() string        { return t.Name }
func (t *InputObject) String() string { return t.Name }

func (t *Scalar) TypeName() string      { return t.Name }
func (t *Object) TypeName() string      { return t.Name }
func (t *Interface) TypeName() string   { return t.Name }
func (t *Union) TypeName() string       { return t.Name }
func (t *Enum) TypeName() string        { return t.Name }
func (t *InputObject) TypeName() string { return t.Name }

func (t *Scalar) Description() string      { return t.Desc }
func (t *Object) Description() string      { return t.Desc }
func (t *Interface) Description() string   { return t.Desc }
func (t *Union) Description() string       { return t.Desc }
func (t *Enum) Description() string        { return t.Desc }
func (t *InputObject) Description() string { return t.Desc }

func (t *Schema) GetDirectives() DirectiveList    { return t.Directives }
func (t *Object) GetDirectives() DirectiveList    { return t.Directives }
func (t *Field) GetDirectives() DirectiveList     { return t.Directives }
func (t *EnumValue) GetDirectives() DirectiveList { return t.Directives }

// TODO:
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
    Desc       string `json:"desc"`
}

// New initializes an instance of Schema.
func New() *Schema {
    s := &Schema{
        entryPointNames:    make(map[string]string),
        Types:              make(map[string]NamedType),
        DeclaredDirectives: make(map[string]*DirectiveDecl),
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
    l := NewLexer(schemaString)

    err := l.CatchSyntaxError(func() { parseSchema(s, l) })
    if err != nil {
        return err
    }

    for _, t := range s.Types {
        if err := resolveNamedType(s, t); err != nil {
            return err
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

    s.EntryPoints = make(map[string]NamedType)
    for key, name := range s.entryPointNames {
        t, ok := s.Types[name]
        if !ok {
            if !ok {
                return errors.Errorf("type %q not found", name)
            }
        }
        s.EntryPoints[key] = t
    }

    for _, obj := range s.objects {
        obj.Interfaces = make([]*Interface, len(obj.interfaceNames))
        for i, intfName := range obj.interfaceNames {
            t, ok := s.Types[intfName]
            if !ok {
                return errors.Errorf("interface %q not found", intfName)
            }
            intf, ok := t.(*Interface)
            if !ok {
                return errors.Errorf("type %q is not an interface", intfName)
            }
            obj.Interfaces[i] = intf
            intf.PossibleTypes = append(intf.PossibleTypes, obj)
        }
    }

    for _, union := range s.unions {
        union.PossibleTypes = make([]*Object, len(union.typeNames))
        for i, name := range union.typeNames {
            t, ok := s.Types[name]
            if !ok {
                return errors.Errorf("object type %q not found", name)
            }
            obj, ok := t.(*Object)
            if !ok {
                return errors.Errorf("type %q is not an object", name)
            }
            union.PossibleTypes[i] = obj
        }
    }

    for _, enum := range s.enums {
        for _, value := range enum.Values {
            if err := resolveDirectives(s, value.Directives); err != nil {
                return err
            }
        }
    }

    return nil
}

func (s *Schema) VisitDirective(name string, visitor func (directive *Directive, on HasDirectives, parent NamedType) error) error {
    d := s.GetDirectives().Get(name)
    if d!=nil {
        err := visitor(d, s, nil)
        if err != nil {
            return err
        }
    }
    for _, t := range s.Types {
        if t, ok :=t.(HasDirectives); ok {
            d := t.GetDirectives().Get(name)
            if d!=nil {
                err := visitor(d, t, nil)
                if err != nil {
                    return err
                }
            }
        }
        switch t := t.(type) {
        case *Object:
            for _, f := range t.Fields {
                d := f.GetDirectives().Get(name)
                if d!=nil {
                    err := visitor(d, f, t)
                    if err != nil {
                        return err
                    }
                }
            }
        case *InputObject:
            for _, f := range t.Fields {
                d := f.GetDirectives().Get(name)
                if d!=nil {
                    err := visitor(d, f, t)
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
        dirName := d.Name.Text
        dd, ok := s.DeclaredDirectives[dirName]
        if !ok {
            return errors.Errorf("directive %q not found", dirName)
        }
        for _, arg := range d.Args {
            if dd.Args.Get(arg.Name.Text) == nil {
                return errors.Errorf("invalid argument %q for directive %q", arg.Name.Text, dirName)
            }
        }
        for _, arg := range dd.Args {
            if _, ok := d.Args.Get(arg.Name.Text); !ok {
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

func parseSchema(s *Schema, l *Lexer) {
    l.Consume()

    for l.Peek() != scanner.EOF {
        desc := l.DescComment()
        switch x := l.ConsumeIdent(); x {

        case "schema":
            s.Directives = ParseDirectives(l)
            l.ConsumeToken('{')
            for l.Peek() != '}' {
                name := l.ConsumeIdent()
                l.ConsumeToken(':')
                typ := l.ConsumeIdent()
                s.entryPointNames[name] = typ
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
                            s.objects = append(s.objects, obj)
                        } else {
                            existing, ok := existing.(*Object)
                            if !ok {
                                l.SyntaxError(fmt.Sprintf("Cannot update %s, it was a %s", obj.Name, existing.Kind()))
                            }
                            existing.Directives = append(existing.Directives, obj.Directives...)
                            existing.Fields = append(existing.Fields, obj.Fields...)
                            existing.Desc = existing.Desc + "\n" + obj.Desc
                            existing.interfaceNames = append(existing.interfaceNames, obj.interfaceNames...)
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
                                return obj.Directives.Get(d.Name.Text) == nil
                            })
                            existing.Fields = existing.Fields.Select(func(d *Field) bool {
                                return obj.Fields.Get(d.Name) == nil
                            })
                            existing.Interfaces = existing.Interfaces.Select(func(d *Interface) bool {
                                return obj.Interfaces.Get(d.Name) == nil
                            })
                            existing.interfaceNames = StringListSelect(existing.interfaceNames, func(d string) bool {
                                return StringListGet(obj.interfaceNames, d) == nil
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
                            s.objects = append(s.objects, obj)
                        }
                    default:
                        panic(`@graphql if value must be: "missing"`)
                    }
                } else {
                    panic(`@graphql alter or if arguments must be provided`)
                }

            } else {
                s.Types[obj.Name] = obj
                s.objects = append(s.objects, obj)
            }

        case "interface":
            iface := parseInterfaceDef(l)
            iface.Desc = desc
            s.Types[iface.Name] = iface

        case "union":
            union := parseUnionDef(l)
            union.Desc = desc
            s.Types[union.Name] = union
            s.unions = append(s.unions, union)

        case "enum":
            enum := parseEnumDef(l)
            enum.Desc = desc
            s.Types[enum.Name] = enum
            s.enums = append(s.enums, enum)

        case "input":
            input := parseInputDef(l)
            input.Desc = desc
            s.Types[input.Name] = input

        case "scalar":
            name := l.ConsumeIdent()
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

func parseObjectDef(l *Lexer) *Object {
    object := &Object{Name: l.ConsumeIdent()}

    if l.PeekKeyword("implements") {
        l.Consume()
        if l.Peek() == '&' {
            l.ConsumeToken('&')
        }
        for {
            object.interfaceNames = append(object.interfaceNames, l.ConsumeIdent())
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

func parseInterfaceDef(l *Lexer) *Interface {
    i := &Interface{Name: l.ConsumeIdent()}
    i.Directives = ParseDirectives(l)
    l.ConsumeToken('{')
    i.Fields = parseFieldsDef(l)
    l.ConsumeToken('}')

    return i
}

func parseUnionDef(l *Lexer) *Union {
    union := &Union{Name: l.ConsumeIdent()}
    union.Directives = ParseDirectives(l)
    l.ConsumeToken('=')
    union.typeNames = []string{l.ConsumeIdent()}
    for l.Peek() == '|' {
        l.ConsumeToken('|')
        union.typeNames = append(union.typeNames, l.ConsumeIdent())
    }

    return union
}

func parseInputDef(l *Lexer) *InputObject {
    i := &InputObject{}
    i.Name = l.ConsumeIdent()
    i.Directives = ParseDirectives(l)
    l.ConsumeToken('{')
    for l.Peek() != '}' {
        i.Fields = append(i.Fields, ParseInputValue(l))
    }
    l.ConsumeToken('}')
    return i
}

func parseEnumDef(l *Lexer) *Enum {
    enum := &Enum{Name: l.ConsumeIdent()}
    enum.Directives = ParseDirectives(l)
    l.ConsumeToken('{')
    for l.Peek() != '}' {
        v := &EnumValue{
            Desc:       l.DescComment(),
            Name:       l.ConsumeIdent(),
            Directives: ParseDirectives(l),
        }

        enum.Values = append(enum.Values, v)
    }
    l.ConsumeToken('}')
    return enum
}

func parseDirectiveDef(l *Lexer) *DirectiveDecl {
    l.ConsumeToken('@')
    d := &DirectiveDecl{Name: l.ConsumeIdent()}

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
        loc := l.ConsumeIdent()
        d.Locs = append(d.Locs, loc)
        if l.Peek() != '|' {
            break
        }
        l.ConsumeToken('|')
    }
    return d
}

func parseFieldsDef(l *Lexer) FieldList {
    var fields FieldList
    for l.Peek() != '}' {
        f := &Field{}
        f.Desc = l.DescComment()
        f.Name = l.ConsumeIdent()
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
