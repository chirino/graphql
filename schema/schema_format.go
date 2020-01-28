package schema

import (
    "bytes"
    "fmt"
    "github.com/chirino/graphql/text"
    "io"
    "reflect"
    "sort"
    "strconv"
    "strings"
)

func (t *List) WriteSchemaFormat(out io.StringWriter)    { panic("unsupported") }
func (t *NonNull) WriteSchemaFormat(out io.StringWriter) { panic("unsupported") }
func (*TypeName) WriteSchemaFormat(out io.StringWriter)  { panic("unsupported") }

func (s *Schema) String() string {
    buf := &bytes.Buffer{}
    s.WriteSchemaFormat(buf)
    return buf.String()
}
func (s *Schema) WriteSchemaFormat(out io.StringWriter) {

    for _, entry := range mapToSortedArray(s.DeclaredDirectives) {
        value := entry.Value.(*DirectiveDecl)
        if isBuiltIn(value) {
            continue
        }
        value.WriteSchemaFormat(out)
    }

    for _, entry := range mapToSortedArray(s.Types) {
        value := entry.Value.(NamedType)
        if isBuiltIn(value) {
            continue
        }
        value.WriteSchemaFormat(out)
    }

    out.WriteString("schema {\n")
    for _, entry := range mapToSortedArray(s.EntryPoints) {
        key := entry.Key.(string)
        value := entry.Value.(NamedType)
        out.WriteString(fmt.Sprintf("  %s: %s\n", key, value.TypeName()))
    }
    out.WriteString("}\n")
}

func (t *Scalar) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("scalar ")
    out.WriteString(t.Name)
    out.WriteString("\n")
}

func (t *DirectiveDecl) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("directive @")
    out.WriteString(t.Name)
    t.Args.WriteSchemaFormat(out)
    out.WriteString(" on ")
    for i, loc := range t.Locs {
        if i != 0 {
            out.WriteString(" | ")
        }
        out.WriteString(loc)
    }
    out.WriteString("\n")
}

func (t *Object) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("type ")
    out.WriteString(t.Name)
    if len(t.Interfaces) > 0 {
        out.WriteString(" implements")
        for i, intf := range t.Interfaces {
            if i != 0 {
                out.WriteString(" &")
            }
            out.WriteString(" ")
            out.WriteString(intf.Name)
        }
        out.WriteString(" ")
        t.Directives.WriteSchemaFormat(out)
    }
    if len(t.Directives) > 0 {
        out.WriteString(" ")
        t.Directives.WriteSchemaFormat(out)
    }
    out.WriteString(" {\n")
    for _, f := range t.Fields {
        i := &indent{}
        f.WriteSchemaFormat(i)
        i.WriteString("\n")
        i.Done(out)
    }
    out.WriteString("}\n")
}

type indent struct {
    bytes.Buffer
}

func (i indent) Done(out io.StringWriter) {
    out.WriteString(text.Indent(i.String(), "  "))
}

func (t *Interface) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("interface ")
    out.WriteString(t.Name)
    out.WriteString(" {\n")
    for _, f := range t.Fields {
        i := &indent{}
        f.WriteSchemaFormat(i)
        i.WriteString("\n")
        i.Done(out)
    }
    out.WriteString("}\n")
}
func (t *Union) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("union ")
    out.WriteString(t.Name)
    out.WriteString(" = ")
    for i, f := range t.typeNames {
        if i != 0 {
            out.WriteString(" | ")
        }
        out.WriteString(f)
    }
    out.WriteString("\n")
}

func (t *Enum) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("enum ")
    out.WriteString(t.Name)
    out.WriteString(" {\n")
    for _, f := range t.Values {
        i := &indent{}
        f.WriteSchemaFormat(i)
        i.WriteString("\n")
        i.Done(out)
    }
    out.WriteString("}\n")
}

func (t *InputObject) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString("input ")
    out.WriteString(t.Name)
    out.WriteString(" {\n")
    for _, f := range t.Fields {
        i := &indent{}
        f.WriteSchemaFormat(i)
        i.WriteString("\n")
        i.Done(out)
    }
    out.WriteString("}\n")
}

func (t *EnumValue) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString(t.Name)
    t.Directives.WriteSchemaFormat(out)
}

func (t *Field) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString(t.Name)
    t.Args.WriteSchemaFormat(out)
    out.WriteString(":")
    out.WriteString(t.Type.String())
    t.Directives.WriteSchemaFormat(out)
}

func (t DirectiveList) WriteSchemaFormat(out io.StringWriter) {
    if len(t) > 0 {
        for i, d := range t {
            if i != 0 {
                out.WriteString(", ")
            }
            d.WriteSchemaFormat(out)
        }
    }
}

func (t *Directive) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString("@")
    out.WriteString(t.Name.Text)
    t.Args.WriteSchemaFormat(out)
}

func (t ArgumentList) WriteSchemaFormat(out io.StringWriter) {
    if len(t) > 0 {
        out.WriteString("(")
        for i, v := range t {
            if i != 0 {
                out.WriteString(", ")
            }
            v.WriteSchemaFormat(out)
        }
        out.WriteString(")")
    }
}

func (t *Argument) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString(t.Name.Text)
    out.WriteString(":")
    t.Value.WriteSchemaFormat(out)
}

func (t InputValueList) WriteSchemaFormat(out io.StringWriter) {
    if len(t) > 0 {
        indented := false
        out.WriteString("(")
        for i, v := range t {
            if i != 0 {
                out.WriteString(", ")
            }

            b := bytes.Buffer{}
            v.WriteSchemaFormat(&b)
            arg := b.String()

            if strings.Contains(arg, "\n") {
                i := &indent{}
                i.WriteString("\n")
                i.WriteString(arg)
                i.Done(out)
                indented = true
            } else {
                out.WriteString(arg)
            }
        }
        if indented {
            out.WriteString("\n")
        }
        out.WriteString(")")
    }
}

func (t *InputValue) WriteSchemaFormat(out io.StringWriter) {
    writeDescription(out, t.Desc)
    out.WriteString(t.Name.Text)
    out.WriteString(":")
    out.WriteString(t.Type.String())
    if t.Default != nil {
        out.WriteString("=")
        t.Default.WriteSchemaFormat(out)
    }
}

func (lit *BasicLit) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString(lit.Text)
}
func (lit *ListLit) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString("[")
    for i, v := range lit.Entries {
        if i != 0 {
            out.WriteString(", ")
        }
        v.WriteSchemaFormat(out)
    }
    out.WriteString("]")
}
func (lit *NullLit) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString("null")
}
func (lit *ObjectLit) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString("{")
    for i, v := range lit.Fields {
        if i != 0 {
            out.WriteString(", ")
        }
        out.WriteString(v.Name.Text)
        out.WriteString(": ")
        v.Value.WriteSchemaFormat(out)
    }
    out.WriteString("}")
}
func (lit *Variable) WriteSchemaFormat(out io.StringWriter) {
    out.WriteString("$")
    out.WriteString(lit.Name)
}

func writeDescription(out io.StringWriter, desc *Description) {
    if desc != nil && desc.Text != "" {
        // desc := desc.Text
        if desc.BlockString {
            out.WriteString(`"""`)
            out.WriteString(desc.Text)
            out.WriteString(`"""`+"\n")
        } else {
            out.WriteString(strconv.Quote(desc.Text))
            out.WriteString("\n")
        }
    }
}

type entry struct {
    Key   interface{}
    Value interface{}
}

func mapToSortedArray(m interface{}) []entry {
    var result []entry
    iter := reflect.ValueOf(m).MapRange()
    for iter.Next() {
        result = append(result, entry{
            Key:   iter.Key().Interface(),
            Value: iter.Value().Interface(),
        })
    }
    sort.Slice(result, func(i, j int) bool {
        key1 := fmt.Sprintf("%s", result[i].Key)
        key2 := fmt.Sprintf("%s", result[j].Key)
        return key1 < key2
    })
    return result
}
func isBuiltIn(m interface{}) bool {
    for _, t := range Meta.Types {
        if m == t {
            return true
        }
    }
    for _, d := range Meta.DeclaredDirectives {
        if m == d {
            return true
        }
    }
    return false
}
