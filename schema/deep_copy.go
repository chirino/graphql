package schema

func (from *QueryDocument) DeepCopy() *QueryDocument {
	if from == nil {
		return nil
	}
	return &QueryDocument{
		Operations: from.Operations.DeepCopy(),
		Fragments:  from.Fragments.DeepCopy(),
	}
}

func (from *Operation) DeepCopy() *Operation {
	if from == nil {
		return nil
	}
	to := *from
	to.Vars = from.Vars.DeepCopy()
	to.Selections = from.Selections.DeepCopy()
	to.Directives = from.Directives.DeepCopy()
	return &to
}

func (from *Directive) DeepCopy() *Directive {
	if from == nil {
		return nil
	}
	to := *from
	to.Args = from.Args.DeepCopy()
	return &to
}

func (from Argument) DeepCopy() Argument {
	to := from
	to.Value = DeepCopyLiteral(from.Value)
	return to
}

func DeepCopyLiteral(from Literal) Literal {
	if from == nil {
		return nil
	}
	switch from := from.(type) {
	case *BasicLit:
		return from.DeepCopy()
	case *ListLit:
		return from.DeepCopy()
	case *NullLit:
		return from.DeepCopy()
	case *ObjectLit:
		return from.DeepCopy()
	case *Variable:
		return from.DeepCopy()
	default:
		panic("unreachable")
	}
}

func (from *BasicLit) DeepCopy() Literal {
	if from == nil {
		return nil
	}
	to := *from
	return &to
}
func (from *ListLit) DeepCopy() Literal {
	if from == nil {
		return nil
	}
	to := *from
	to.Entries = make([]Literal, len(from.Entries))
	for i, v := range from.Entries {
		to.Entries[i] = DeepCopyLiteral(v)
	}
	return &to
}
func (from *NullLit) DeepCopy() Literal {
	if from == nil {
		return nil
	}
	to := *from
	return &to
}
func (from *ObjectLit) DeepCopy() Literal {
	if from == nil {
		return nil
	}
	to := *from
	to.Fields = make([]*ObjectLitField, len(from.Fields))
	for i, v := range from.Fields {
		to.Fields[i] = v.DeepCopy()
	}
	return &to
}

func (from *ObjectLitField) DeepCopy() *ObjectLitField {
	if from == nil {
		return nil
	}
	to := *from
	to.Value = DeepCopyLiteral(from.Value)
	return &to
}

func (from *Variable) DeepCopy() Literal {
	if from == nil {
		return nil
	}
	to := *from
	return &to
}

func (from *FieldSelection) DeepCopy() Selection {
	if from == nil {
		return nil
	}
	to := *from
	to.Arguments = from.Arguments.DeepCopy()
	to.Directives = from.Directives.DeepCopy()
	to.Selections = from.Selections.DeepCopy()
	return &to
}

func (from *FragmentSpread) DeepCopy() Selection {
	if from == nil {
		return nil
	}
	to := *from
	to.Directives = from.Directives.DeepCopy()
	return &to
}

func (from *InlineFragment) DeepCopy() Selection {
	if from == nil {
		return nil
	}
	to := *from
	to.Directives = from.Directives.DeepCopy()
	to.Selections = from.Selections.DeepCopy()
	return &to
}

func (from *InputValue) DeepCopy() *InputValue {
	if from == nil {
		return nil
	}
	to := *from
	to.Directives = from.Directives.DeepCopy()
	to.Default = DeepCopyLiteral(from.Default)
	return &to
}

func (from *FragmentDecl) DeepCopy() *FragmentDecl {
	if from == nil {
		return nil
	}
	to := *from
	to.Directives = from.Directives.DeepCopy()
	to.Selections = from.Selections.DeepCopy()
	return &to
}

func DeepCopySelection(from Selection) Selection {
	if from == nil {
		return nil
	}
	switch from := from.(type) {
	case *FieldSelection:
		return from.DeepCopy()
	case *InlineFragment:
		return from.DeepCopy()
	case *FragmentSpread:
		return from.DeepCopy()
	default:
		panic("unreachable")
	}
}

func (from SelectionList) DeepCopy() (to SelectionList) {
	if len(from) == 0 {
		return
	}
	to = make(SelectionList, len(from))
	for i, v := range from {
		to[i] = DeepCopySelection(v)
	}
	return
}

func (from InputValueList) DeepCopy() (to InputValueList) {
	if len(from) == 0 {
		return
	}
	to = make(InputValueList, len(from))
	for i, v := range from {
		to[i] = v.DeepCopy()
	}
	return
}

func (from FragmentList) DeepCopy() (to FragmentList) {
	if len(from) == 0 {
		return
	}
	to = make(FragmentList, len(from))
	for i, v := range from {
		to[i] = v.DeepCopy()
	}
	return

}

func (from OperationList) DeepCopy() (to OperationList) {
	if len(from) == 0 {
		return
	}
	to = make(OperationList, len(from))
	for i, v := range from {
		to[i] = v.DeepCopy()
	}
	return
}

func (from ArgumentList) DeepCopy() (to ArgumentList) {
	if len(from) == 0 {
		return
	}
	to = make(ArgumentList, len(from))
	for i, v := range from {
		to[i] = v.DeepCopy()
	}
	return
}

func (from DirectiveList) DeepCopy() (to DirectiveList) {
	if len(from) == 0 {
		return
	}
	to = make(DirectiveList, len(from))
	for i, v := range from {
		to[i] = v.DeepCopy()
	}
	return
}
