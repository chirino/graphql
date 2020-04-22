package schema

func (t *Schema) AddIfMissing(to *Schema, from *Schema) {
	if t != from {
		panic("receiver should match the from argument")
	}
	for k, v := range t.EntryPoints {
		if to.EntryPoints[k] == nil {
			to.EntryPoints[k] = v
		}
	}
	for k, v := range t.DeclaredDirectives {
		if to.DeclaredDirectives[k] == nil {
			to.DeclaredDirectives[k] = v
		}
	}
	for k, v := range t.Types {
		if to.Types[k] == nil {
			to.Types[k] = v
		}
	}
}

func (t *InputObject) AddIfMissing(to *Schema, from *Schema) {
	if to.Types[t.Name] == nil {
		to.Types[t.Name] = t
		t.Directives.AddIfMissing(to, from)
		t.Fields.AddIfMissing(to, from)
	}
}

func (t DirectiveList) AddIfMissing(to *Schema, from *Schema) {
	for _, d := range t {
		k := d.Name.Text
		v := from.DeclaredDirectives[k]
		if to.DeclaredDirectives[k] == nil {
			to.DeclaredDirectives[k] = v
		}
	}
}

func (t *Object) AddIfMissing(to *Schema, from *Schema) {
	if to.Types[t.Name] == nil {
		to.Types[t.Name] = t
		t.Directives.AddIfMissing(to, from)
		t.Fields.AddIfMissing(to, from)
	}
}

func (t FieldList) AddIfMissing(to *Schema, from *Schema) {
	for _, t := range t {
		t.AddIfMissing(to, from)
	}
}

func (t *Field) AddIfMissing(to *Schema, from *Schema) {
	t.Directives.AddIfMissing(to, from)
	t.Type.AddIfMissing(to, from)
	t.Args.AddIfMissing(to, from)
}

func (t InputValueList) AddIfMissing(to *Schema, from *Schema) {
	for _, t := range t {
		t.AddIfMissing(to, from)
	}
}

func (t *InputValue) AddIfMissing(to *Schema, from *Schema) {
	t.Type.AddIfMissing(to, from)
	t.Directives.AddIfMissing(to, from)
}

func (t *List) AddIfMissing(to *Schema, from *Schema) {
	t.OfType.AddIfMissing(to, from)
}
func (t *NonNull) AddIfMissing(to *Schema, from *Schema) {
	t.OfType.AddIfMissing(to, from)
}
func (t *TypeName) AddIfMissing(to *Schema, from *Schema) {
}
func (t *Scalar) AddIfMissing(to *Schema, from *Schema) {
	if to.Types[t.Name] == nil {
		to.Types[t.Name] = t
	}
}
func (t *Interface) AddIfMissing(to *Schema, from *Schema) {
	if to.Types[t.Name] == nil {
		to.Types[t.Name] = t
		t.Directives.AddIfMissing(to, from)
		t.Fields.AddIfMissing(to, from)
	}
}
func (t *Union) AddIfMissing(to *Schema, from *Schema) {
	if to.Types[t.Name] == nil {
		to.Types[t.Name] = t
		t.Directives.AddIfMissing(to, from)
		for _, t := range t.PossibleTypes {
			t.AddIfMissing(to, from)
		}
	}
}
func (t *Enum) AddIfMissing(to *Schema, from *Schema) {
	if to.Types[t.Name] == nil {
		to.Types[t.Name] = t
		t.Directives.AddIfMissing(to, from)
		for _, t := range t.Values {
			t.AddIfMissing(to, from)
		}
	}
}
func (t *EnumValue) AddIfMissing(to *Schema, from *Schema) {
	t.Directives.AddIfMissing(to, from)
}
