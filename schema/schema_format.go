package schema

import (
	"bytes"
	"fmt"
	"github.com/chirino/graphql/internal/lexer"
	"github.com/chirino/graphql/text"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func (t *List) WriteTo(out io.StringWriter) {
	out.WriteString("[")
	t.OfType.WriteTo(out)
	out.WriteString("]")
}
func (t *NonNull) WriteTo(out io.StringWriter) {
	out.WriteString("!")
	t.OfType.WriteTo(out)
}
func (t *TypeName) WriteTo(out io.StringWriter) {
	out.WriteString(t.Name)
}

func (s *Schema) WriteTo(out io.StringWriter) {

	for _, entry := range mapToSortedArray(s.DeclaredDirectives) {
		value := entry.Value.(*DirectiveDecl)
		if isBuiltIn(value) {
			continue
		}
		value.WriteTo(out)
	}

	for _, entry := range mapToSortedArray(s.Types) {
		value := entry.Value.(NamedType)
		if isBuiltIn(value) {
			continue
		}
		value.WriteTo(out)
	}

	out.WriteString("schema {\n")
	for _, entry := range mapToSortedArray(s.EntryPoints) {
		key := entry.Key.(OperationType)
		value := entry.Value.(NamedType)
		out.WriteString(fmt.Sprintf("  %s: %s\n", key, value.TypeName()))
	}
	out.WriteString("}\n")
}

func (t *Scalar) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString("scalar ")
	out.WriteString(t.Name)
	out.WriteString("\n")
}

func (t *DirectiveDecl) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString("directive @")
	out.WriteString(t.Name)
	t.Args.WriteTo(out)
	out.WriteString(" on ")
	for i, loc := range t.Locs {
		if i != 0 {
			out.WriteString(" | ")
		}
		out.WriteString(loc)
	}
	out.WriteString("\n")
}

func (t *Object) WriteTo(out io.StringWriter) {
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
		t.Directives.WriteTo(out)
	}
	if len(t.Directives) > 0 {
		out.WriteString(" ")
		t.Directives.WriteTo(out)
	}
	out.WriteString(" {\n")
	sort.Slice(t.Fields, func(i, j int) bool {
		return t.Fields[i].Name < t.Fields[j].Name
	})
	for _, f := range t.Fields {
		i := &indent{}
		f.WriteTo(i)
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

func (t *Interface) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString("interface ")
	out.WriteString(t.Name)
	out.WriteString(" {\n")

	sort.Slice(t.Fields, func(i, j int) bool {
		return t.Fields[i].Name < t.Fields[j].Name
	})
	for _, f := range t.Fields {
		i := &indent{}
		f.WriteTo(i)
		i.WriteString("\n")
		i.Done(out)
	}
	out.WriteString("}\n")
}
func (t *Union) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString("union ")
	out.WriteString(t.Name)
	out.WriteString(" = ")
	for i, f := range t.PossibleTypes {
		if i != 0 {
			out.WriteString(" | ")
		}
		out.WriteString(f.Name)
	}
	out.WriteString("\n")
}

func (t *Enum) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString("enum ")
	out.WriteString(t.Name)
	out.WriteString(" {\n")
	for _, f := range t.Values {
		i := &indent{}
		f.WriteTo(i)
		i.WriteString("\n")
		i.Done(out)
	}
	out.WriteString("}\n")
}

func (t *InputObject) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString("input ")
	out.WriteString(t.Name)
	out.WriteString(" {\n")

	sort.Slice(t.Fields, func(i, j int) bool {
		return t.Fields[i].Name < t.Fields[j].Name
	})
	for _, f := range t.Fields {
		i := &indent{}
		f.WriteTo(i)
		i.WriteString("\n")
		i.Done(out)
	}
	out.WriteString("}\n")
}

func (t *EnumValue) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString(t.Name)
	t.Directives.WriteTo(out)
}

func (t *Field) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString(t.Name)
	t.Args.WriteTo(out)
	out.WriteString(":")
	out.WriteString(t.Type.String())
	t.Directives.WriteTo(out)
}

func (t DirectiveList) WriteTo(out io.StringWriter) {
	if len(t) > 0 {
		for i, d := range t {
			if i != 0 {
				out.WriteString(", ")
			}
			d.WriteTo(out)
		}
	}
}

func (t *Directive) WriteTo(out io.StringWriter) {
	out.WriteString("@")
	out.WriteString(t.Name)
	t.Args.WriteTo(out)
}

func (t ArgumentList) WriteTo(out io.StringWriter) {
	if len(t) > 0 {
		out.WriteString("(")
		for i, v := range t {
			if i != 0 {
				out.WriteString(", ")
			}
			v.WriteTo(out)
		}
		out.WriteString(")")
	}
}

func (t *Argument) WriteTo(out io.StringWriter) {
	out.WriteString(t.Name)
	out.WriteString(":")
	t.Value.WriteTo(out)
}

func (t InputValueList) WriteTo(out io.StringWriter) {
	if len(t) > 0 {
		indented := false
		out.WriteString("(")
		for i, v := range t {
			if i != 0 {
				out.WriteString(", ")
			}

			b := bytes.Buffer{}
			v.WriteTo(&b)
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

func (t *InputValue) WriteTo(out io.StringWriter) {
	writeDescription(out, t.Desc)
	out.WriteString(t.Name)
	out.WriteString(":")
	out.WriteString(t.Type.String())
	if t.Default != nil {
		out.WriteString("=")
		t.Default.WriteTo(out)
	}
}

func (lit *BasicLit) WriteTo(out io.StringWriter) {
	out.WriteString(lit.Text)
}
func (lit *ListLit) WriteTo(out io.StringWriter) {
	out.WriteString("[")
	for i, v := range lit.Entries {
		if i != 0 {
			out.WriteString(", ")
		}
		v.WriteTo(out)
	}
	out.WriteString("]")
}
func (lit *NullLit) WriteTo(out io.StringWriter) {
	out.WriteString("null")
}
func (t *ObjectLit) WriteTo(out io.StringWriter) {
	out.WriteString("{")
	sort.Slice(t.Fields, func(i, j int) bool {
		return t.Fields[i].Name < t.Fields[j].Name
	})
	for i, v := range t.Fields {
		if i != 0 {
			out.WriteString(", ")
		}
		out.WriteString(v.Name)
		out.WriteString(": ")
		v.Value.WriteTo(out)
	}
	out.WriteString("}")
}
func (lit *Variable) WriteTo(out io.StringWriter) {
	out.WriteString("$")
	out.WriteString(lit.Name)
}

func writeDescription(out io.StringWriter, desc Description) {
	if desc.ShowType == lexer.NoDescription || desc.Text == "" {
		return
	}
	showType := desc.ShowType
	if showType == lexer.PossibleDescription && strings.Contains(desc.Text, "\n") {
		showType = lexer.ShowBlockDescription
	}
	if desc.ShowType == lexer.ShowBlockDescription {
		out.WriteString(`"""`)
		out.WriteString(desc.Text)
		out.WriteString(`"""` + "\n")
	} else {
		out.WriteString(strconv.Quote(desc.Text))
		out.WriteString("\n")
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
