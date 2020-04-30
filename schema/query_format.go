package schema

import (
	"io"
)

func (s *QueryDocument) String() string {
	return FormatterToString(s)
}

func (s *QueryDocument) WriteTo(out io.StringWriter) {
	for _, value := range s.Operations {
		value.WriteTo(out)
		out.WriteString("\n")
	}
	for _, value := range s.Fragments {
		value.WriteTo(out)
		out.WriteString("\n")
	}
}

func (o *FragmentDecl) WriteTo(out io.StringWriter) {
	out.WriteString("fragment")
	if o.Name != "" {
		out.WriteString(" ")
		out.WriteString(o.Name)
	}
	out.WriteString(" on ")
	out.WriteString(o.On.Name)
	o.Directives.WriteTo(out)
	o.Selections.WriteTo(out)
}

func (o *Operation) WriteTo(out io.StringWriter) {
	out.WriteString(string(o.Type))
	if o.Name != "" {
		out.WriteString(" ")
		out.WriteString(o.Name)
	}
	o.Directives.WriteTo(out)
	o.Vars.WriteTo(out)
	o.Selections.WriteTo(out)
}

func (o SelectionList) WriteTo(out io.StringWriter) {
	if len(o) > 0 {
		out.WriteString(" {\n")
		i := &indent{}
		for _, value := range o {
			value.WriteTo(i)
		}
		i.Done(out)
		out.WriteString("}")
	}
}

func (t *FieldSelection) WriteTo(out io.StringWriter) {
	out.WriteString(t.Alias)
	if t.Name != t.Alias {
		out.WriteString(":")
		out.WriteString(t.Name)
	}
	t.Arguments.WriteTo(out)
	t.Directives.WriteTo(out)
	t.Selections.WriteTo(out)
	out.WriteString("\n")
}

func (t *FragmentSpread) WriteTo(out io.StringWriter) {
	out.WriteString("...")
	out.WriteString(t.Name)
	t.Directives.WriteTo(out)
	out.WriteString("\n")
}

func (t *InlineFragment) WriteTo(out io.StringWriter) {
	out.WriteString("... on ")
	out.WriteString(t.On.Name)
	t.Directives.WriteTo(out)
	t.Selections.WriteTo(out)
	out.WriteString("\n")
}
