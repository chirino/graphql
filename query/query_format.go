package query

import (
	"bytes"
	"io"

	"github.com/chirino/graphql/schema"
	"github.com/chirino/graphql/text"
)

func (s *Document) String() string {
	return schema.FormatterToString(s)
}

func (s *Document) WriteTo(out io.StringWriter) {
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
	if o.Name.Text != "" {
		out.WriteString(" ")
		out.WriteString(o.Name.Text)
	}
	out.WriteString(" on ")
	out.WriteString(o.On.Text)
	o.Directives.WriteTo(out)
	o.Selections.WriteTo(out)
}

func (o *Operation) WriteTo(out io.StringWriter) {
	out.WriteString(string(o.Type))
	if o.Name.Text != "" {
		out.WriteString(" ")
		out.WriteString(o.Name.Text)
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

type indent struct {
	bytes.Buffer
}

func (i indent) Done(out io.StringWriter) {
	out.WriteString(text.Indent(i.String(), "  "))
}

func (t *Field) WriteTo(out io.StringWriter) {
	out.WriteString(t.Alias.Text)
	if t.Name.Text != t.Alias.Text {
		out.WriteString(":")
		out.WriteString(t.Name.Text)
	}
	t.Arguments.WriteTo(out)
	t.Directives.WriteTo(out)
	t.Selections.WriteTo(out)
	out.WriteString("\n")
}

func (t *FragmentSpread) WriteTo(out io.StringWriter) {
	out.WriteString("...")
	out.WriteString(t.Name.Text)
	t.Directives.WriteTo(out)
	out.WriteString("\n")
}

func (t *InlineFragment) WriteTo(out io.StringWriter) {
	out.WriteString("... on ")
	out.WriteString(t.On.Text)
	t.Directives.WriteTo(out)
	t.Selections.WriteTo(out)
	out.WriteString("\n")
}
