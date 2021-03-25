package validation

import (
	"fmt"

	"github.com/chirino/graphql/internal/scanner"
	"github.com/chirino/graphql/qerrors"
	uperrors "github.com/graph-gophers/graphql-go/errors"

	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/chirino/graphql/schema"
)

type varSet map[*schema.InputValue]struct{}

type selectionPair struct{ a, b schema.Selection }

type fieldInfo struct {
	sf     *schema.Field
	parent schema.NamedType
}

type context struct {
	schema           *schema.Schema
	doc              *schema.QueryDocument
	errs             qerrors.ErrorList
	opErrs           map[*schema.Operation]qerrors.ErrorList
	usedVars         map[*schema.Operation]varSet
	fieldMap         map[*schema.FieldSelection]fieldInfo
	overlapValidated map[selectionPair]struct{}
	maxDepth         int
}

func (c *context) addErr(loc uperrors.Location, rule string, format string, a ...interface{}) {
	c.addErrMultiLoc([]uperrors.Location{loc}, rule, format, a...)
}

func (c *context) addErrMultiLoc(locs []uperrors.Location, rule string, format string, a ...interface{}) {
	c.errs = append(c.errs, (&qerrors.Error{
		QueryError: &uperrors.QueryError {
			Message:   fmt.Sprintf(format, a...),
			Locations: locs,
			Rule:      rule,
		},
	}).WithStack())
}

type opContext struct {
	*context
	ops []*schema.Operation
}

func newContext(s *schema.Schema, doc *schema.QueryDocument, maxDepth int) *context {
	return &context{
		schema:           s,
		doc:              doc,
		opErrs:           make(map[*schema.Operation]qerrors.ErrorList),
		usedVars:         make(map[*schema.Operation]varSet),
		fieldMap:         make(map[*schema.FieldSelection]fieldInfo),
		overlapValidated: make(map[selectionPair]struct{}),
		maxDepth:         maxDepth,
	}
}

func Validate(s *schema.Schema, doc *schema.QueryDocument, maxDepth int) qerrors.ErrorList {
	c := newContext(s, doc, maxDepth)

	opNames := make(nameSet)
	fragUsedBy := make(map[*schema.FragmentDecl][]*schema.Operation)
	for _, op := range doc.Operations {
		c.usedVars[op] = make(varSet)
		opc := &opContext{c, []*schema.Operation{op}}

		// Check if max depth is exceeded, if it's set. If max depth is exceeded,
		// don't continue to validate the document and exit early.
		if validateMaxDepth(opc, op.Selections, 1) {
			return c.errs
		}

		if op.Name == "" && len(doc.Operations) != 1 {
			c.addErr(op.Loc, "LoneAnonymousOperation", "This anonymous operation must be the only defined operation.")
		}
		if op.Name != "" {
			validateName(c, opNames, op.Name, op.NameLoc, "UniqueOperationNames", "operation")
		}

		validateDirectives(opc, strings.ToUpper(string(op.Type)), op.Directives)

		varNames := make(nameSet)
		for _, v := range op.Vars {
			validateName(c, varNames, v.Name, v.NameLoc, "UniqueVariableNames", "variable")

			t := resolveType(c, v.Type)
			if !canBeInput(t) {
				c.addErr(v.TypeLoc, "VariablesAreInputTypes", "Variable %q cannot be non-input type %q.", v.Name, t)
			}

			if v.Default != nil {
				validateLiteral(opc, v.Default)

				if t != nil {
					if nn, ok := t.(*schema.NonNull); ok {
						c.addErr(v.Default.Location(), "DefaultValuesOfCorrectType", "Variable %q of type %q is required and will not use the default value. Perhaps you meant to use type %q.", v.Name, t, nn.OfType)
					}

					if ok, reason := validateValueType(opc, v.Default, t); !ok {
						c.addErr(v.Default.Location(), "DefaultValuesOfCorrectType", "Variable %q of type %q has invalid default value %s.\n%s", v.Name, t, v.Default, reason)
					}
				}
			}
		}

		entryPoint := s.EntryPoints[op.Type]
		validateSelectionSet(opc, op.Selections, entryPoint)

		fragUsed := make(map[*schema.FragmentDecl]struct{})
		markUsedFragments(c, op.Selections, fragUsed)
		for frag := range fragUsed {
			fragUsedBy[frag] = append(fragUsedBy[frag], op)
		}
	}

	fragNames := make(nameSet)
	fragVisited := make(map[*schema.FragmentDecl]struct{})
	for _, frag := range doc.Fragments {
		opc := &opContext{c, fragUsedBy[frag]}

		validateName(c, fragNames, frag.Name, frag.NameLoc, "UniqueFragmentNames", "fragment")
		validateDirectives(opc, "FRAGMENT_DEFINITION", frag.Directives)

		t := unwrapType(resolveType(c, &frag.On))
		// continue even if t is nil
		if t != nil && !canBeFragment(t) {
			c.addErr(frag.On.NameLoc, "FragmentsOnCompositeTypes", "Fragment %q cannot condition on non composite type %q.", frag.Name, t)
			continue
		}

		validateSelectionSet(opc, frag.Selections, t)

		if _, ok := fragVisited[frag]; !ok {
			detectFragmentCycle(c, frag.Selections, fragVisited, nil, map[string]int{frag.Name: 0})
		}
	}

	for _, frag := range doc.Fragments {
		if len(fragUsedBy[frag]) == 0 {
			c.addErr(frag.Loc, "NoUnusedFragments", "Fragment %q is never used.", frag.Name)
		}
	}

	for _, op := range doc.Operations {
		c.errs = append(c.errs, c.opErrs[op]...)

		opUsedVars := c.usedVars[op]
		for _, v := range op.Vars {
			if _, ok := opUsedVars[v]; !ok {
				opSuffix := ""
				if op.Name != "" {
					opSuffix = fmt.Sprintf(" in operation %q", op.Name)
				}
				c.addErr(v.Loc, "NoUnusedVariables", "Variable %q is never used%s.", v.Name, opSuffix)
			}
		}
	}

	return c.errs
}

// validates the query doesn't go deeper than maxDepth (if set). Returns whether
// or not query validated max depth to avoid excessive recursion.
func validateMaxDepth(c *opContext, sels []schema.Selection, depth int) bool {
	// maxDepth checking is turned off when maxDepth is 0
	if c.maxDepth == 0 {
		return false
	}

	exceededMaxDepth := false

	for _, sel := range sels {
		switch sel := sel.(type) {
		case *schema.FieldSelection:
			if depth > c.maxDepth {
				exceededMaxDepth = true
				c.addErr(sel.AliasLoc, "MaxDepthExceeded", "Field %q has depth %d that exceeds max depth %d", sel.Name, depth, c.maxDepth)
				continue
			}
			exceededMaxDepth = exceededMaxDepth || validateMaxDepth(c, sel.Selections, depth+1)
		case *schema.InlineFragment:
			// Depth is not checked because inline fragments resolve to other fields which are checked.
			// Depth is not incremented because inline fragments have the same depth as neighboring fields
			exceededMaxDepth = exceededMaxDepth || validateMaxDepth(c, sel.Selections, depth)
		case *schema.FragmentSpread:
			// Depth is not checked because fragments resolve to other fields which are checked.
			frag := c.doc.Fragments.Get(sel.Name)
			if frag == nil {
				// In case of unknown fragment (invalid request), ignore max depth evaluation
				c.addErr(sel.Loc, "MaxDepthEvaluationError", "Unknown fragment %q. Unable to evaluate depth.", sel.Name)
				continue
			}
			// Depth is not incremented because fragments have the same depth as surrounding fields
			exceededMaxDepth = exceededMaxDepth || validateMaxDepth(c, frag.Selections, depth)
		}
	}

	return exceededMaxDepth
}

func validateSelectionSet(c *opContext, sels []schema.Selection, t schema.NamedType) {
	for _, sel := range sels {
		validateSelection(c, sel, t)
	}

	for i, a := range sels {
		for _, b := range sels[i+1:] {
			c.validateOverlap(a, b, nil, nil)
		}
	}
}

func validateSelection(c *opContext, sel schema.Selection, t schema.NamedType) {
	switch sel := sel.(type) {
	case *schema.FieldSelection:
		validateDirectives(c, "FIELD", sel.Directives)

		fieldName := sel.Name
		var f *schema.Field
		switch fieldName {
		case "__typename":
			f = &schema.Field{
				Name: "__typename",
				Type: c.schema.Types["String"],
			}
		case "__schema":
			f = &schema.Field{
				Name: "__schema",
				Type: c.schema.Types["__Schema"],
			}
		case "__type":
			f = &schema.Field{
				Name: "__type",
				Args: schema.InputValueList{
					&schema.InputValue{
						Name: "name",
						Type: &schema.NonNull{OfType: c.schema.Types["String"]},
					},
				},
				Type: c.schema.Types["__Type"],
			}
		default:
			f = fields(t).Get(fieldName)
			if f == nil && t != nil {
				suggestion := makeSuggestion("Did you mean", fields(t).Names(), fieldName)
				c.addErr(sel.AliasLoc, "FieldsOnCorrectType", "Cannot query field %q on type %q.%s", fieldName, t, suggestion)
			}
		}
		c.fieldMap[sel] = fieldInfo{sf: f, parent: t}

		sel.Schema = &schema.FieldSchema{
			Field:  f,
			Parent: t,
		}

		validateArgumentLiterals(c, sel.Arguments)
		if f != nil {
			validateArgumentTypes(c, sel.Arguments, f.Args, sel.AliasLoc,
				func() string { return fmt.Sprintf("field %q of type %q", fieldName, t) },
				func() string { return fmt.Sprintf("Field %q", fieldName) },
			)
		}

		var ft schema.Type
		if f != nil {
			ft = f.Type
			sf := hasSubfields(ft)
			if sf && sel.Selections == nil {
				c.addErr(sel.AliasLoc, "ScalarLeafs", "Field %q of type %q must have a selection of subfields. Did you mean \"%s { ... }\"?", fieldName, ft, fieldName)
			}
			if !sf && sel.Selections != nil {
				c.addErr(sel.SelectionSetLoc, "ScalarLeafs", "Field %q must not have a selection since type %q has no subfields.", fieldName, ft)
			}
		}
		if sel.Selections != nil {
			validateSelectionSet(c, sel.Selections, unwrapType(ft))
		}

	case *schema.InlineFragment:
		validateDirectives(c, "INLINE_FRAGMENT", sel.Directives)
		if sel.On.Name != "" {
			fragTyp := unwrapType(resolveType(c.context, &sel.On))
			if fragTyp != nil && !compatible(t, fragTyp) {
				c.addErr(sel.Loc, "PossibleFragmentSpreads", "Fragment cannot be spread here as objects of type %q can never be of type %q.", t, fragTyp)
			}
			t = fragTyp
			// continue even if t is nil
		}
		if t != nil && !canBeFragment(t) {
			c.addErr(sel.On.NameLoc, "FragmentsOnCompositeTypes", "Fragment cannot condition on non composite type %q.", t)
			return
		}
		validateSelectionSet(c, sel.Selections, unwrapType(t))

	case *schema.FragmentSpread:
		validateDirectives(c, "FRAGMENT_SPREAD", sel.Directives)
		frag := c.doc.Fragments.Get(sel.Name)
		if frag == nil {
			c.addErr(sel.NameLoc, "KnownFragmentNames", "Unknown fragment %q.", sel.Name)
			return
		}
		fragTyp := c.schema.Types[frag.On.Name]
		if !compatible(t, fragTyp) {
			c.addErr(sel.Loc, "PossibleFragmentSpreads", "Fragment %q cannot be spread here as objects of type %q can never be of type %q.", frag.Name, t, fragTyp)
		}

	default:
		panic("unreachable")
	}
}

func compatible(a, b schema.Type) bool {
	for _, pta := range possibleTypes(a) {
		for _, ptb := range possibleTypes(b) {
			if pta == ptb {
				return true
			}
		}
	}
	return false
}

func possibleTypes(t schema.Type) []*schema.Object {
	switch t := t.(type) {
	case *schema.Object:
		return []*schema.Object{t}
	case *schema.Interface:
		return t.PossibleTypes
	case *schema.Union:
		return t.PossibleTypes
	default:
		return nil
	}
}

func markUsedFragments(c *context, sels []schema.Selection, fragUsed map[*schema.FragmentDecl]struct{}) {
	for _, sel := range sels {
		switch sel := sel.(type) {
		case *schema.FieldSelection:
			if sel.Selections != nil {
				markUsedFragments(c, sel.Selections, fragUsed)
			}

		case *schema.InlineFragment:
			markUsedFragments(c, sel.Selections, fragUsed)

		case *schema.FragmentSpread:
			frag := c.doc.Fragments.Get(sel.Name)
			if frag == nil {
				return
			}

			if _, ok := fragUsed[frag]; ok {
				return
			}
			fragUsed[frag] = struct{}{}
			markUsedFragments(c, frag.Selections, fragUsed)

		default:
			panic("unreachable")
		}
	}
}

func detectFragmentCycle(c *context, sels []schema.Selection, fragVisited map[*schema.FragmentDecl]struct{}, spreadPath []*schema.FragmentSpread, spreadPathIndex map[string]int) {
	for _, sel := range sels {
		detectFragmentCycleSel(c, sel, fragVisited, spreadPath, spreadPathIndex)
	}
}

func detectFragmentCycleSel(c *context, sel schema.Selection, fragVisited map[*schema.FragmentDecl]struct{}, spreadPath []*schema.FragmentSpread, spreadPathIndex map[string]int) {
	switch sel := sel.(type) {
	case *schema.FieldSelection:
		if sel.Selections != nil {
			detectFragmentCycle(c, sel.Selections, fragVisited, spreadPath, spreadPathIndex)
		}

	case *schema.InlineFragment:
		detectFragmentCycle(c, sel.Selections, fragVisited, spreadPath, spreadPathIndex)

	case *schema.FragmentSpread:
		frag := c.doc.Fragments.Get(sel.Name)
		if frag == nil {
			return
		}

		spreadPath = append(spreadPath, sel)
		if i, ok := spreadPathIndex[frag.Name]; ok {
			cyclePath := spreadPath[i:]
			via := ""
			if len(cyclePath) > 1 {
				names := make([]string, len(cyclePath)-1)
				for i, frag := range cyclePath[:len(cyclePath)-1] {
					names[i] = frag.Name
				}
				via = " via " + strings.Join(names, ", ")
			}

			locs := make([]uperrors.Location, len(cyclePath))
			for i, frag := range cyclePath {
				locs[i] = frag.Loc
			}
			c.addErrMultiLoc(locs, "NoFragmentCycles", "Cannot spread fragment %q within itself%s.", frag.Name, via)
			return
		}

		if _, ok := fragVisited[frag]; ok {
			return
		}
		fragVisited[frag] = struct{}{}

		spreadPathIndex[frag.Name] = len(spreadPath)
		detectFragmentCycle(c, frag.Selections, fragVisited, spreadPath, spreadPathIndex)
		delete(spreadPathIndex, frag.Name)

	default:
		panic("unreachable")
	}
}

func (c *context) validateOverlap(a, b schema.Selection, reasons *[]string, locs *[]uperrors.Location) {
	if a == b {
		return
	}

	if _, ok := c.overlapValidated[selectionPair{a, b}]; ok {
		return
	}
	c.overlapValidated[selectionPair{a, b}] = struct{}{}
	c.overlapValidated[selectionPair{b, a}] = struct{}{}

	switch a := a.(type) {
	case *schema.FieldSelection:
		switch b := b.(type) {
		case *schema.FieldSelection:
			if b.AliasLoc.Before(a.AliasLoc) {
				a, b = b, a
			}
			if reasons2, locs2 := c.validateFieldOverlap(a, b); len(reasons2) != 0 {
				locs2 = append(locs2, a.AliasLoc, b.AliasLoc)
				if reasons == nil {
					c.addErrMultiLoc(locs2, "OverlappingFieldsCanBeMerged", "Fields %q conflict because %s. Use different aliases on the fields to fetch both if this was intentional.", a.Alias, strings.Join(reasons2, " and "))
					return
				}
				for _, r := range reasons2 {
					*reasons = append(*reasons, fmt.Sprintf("subfields %q conflict because %s", a.Alias, r))
				}
				*locs = append(*locs, locs2...)
			}

		case *schema.InlineFragment:
			for _, sel := range b.Selections {
				c.validateOverlap(a, sel, reasons, locs)
			}

		case *schema.FragmentSpread:
			if frag := c.doc.Fragments.Get(b.Name); frag != nil {
				for _, sel := range frag.Selections {
					c.validateOverlap(a, sel, reasons, locs)
				}
			}

		default:
			panic("unreachable")
		}

	case *schema.InlineFragment:
		for _, sel := range a.Selections {
			c.validateOverlap(sel, b, reasons, locs)
		}

	case *schema.FragmentSpread:
		if frag := c.doc.Fragments.Get(a.Name); frag != nil {
			for _, sel := range frag.Selections {
				c.validateOverlap(sel, b, reasons, locs)
			}
		}

	default:
		panic("unreachable")
	}
}

func (c *context) validateFieldOverlap(a, b *schema.FieldSelection) ([]string, []uperrors.Location) {
	if a.Alias != b.Alias {
		return nil, nil
	}

	if asf := c.fieldMap[a].sf; asf != nil {
		if bsf := c.fieldMap[b].sf; bsf != nil {
			if !typesCompatible(asf.Type, bsf.Type) {
				return []string{fmt.Sprintf("they return conflicting types %s and %s", asf.Type, bsf.Type)}, nil
			}
		}
	}

	at := c.fieldMap[a].parent
	bt := c.fieldMap[b].parent
	if at == nil || bt == nil || at == bt {
		if a.Name != b.Name {
			return []string{fmt.Sprintf("%s and %s are different fields", a.Name, b.Name)}, nil
		}

		if argumentsConflict(a.Arguments, b.Arguments) {
			return []string{"they have differing arguments"}, nil
		}
	}

	var reasons []string
	var locs []uperrors.Location
	for _, a2 := range a.Selections {
		for _, b2 := range b.Selections {
			c.validateOverlap(a2, b2, &reasons, &locs)
		}
	}
	return reasons, locs
}

func argumentsConflict(a, b schema.ArgumentList) bool {
	if len(a) != len(b) {
		return true
	}
	for _, argA := range a {
		valB, ok := b.Get(argA.Name)
		if !ok || !reflect.DeepEqual(argA.Value.Evaluate(nil), valB.Evaluate(nil)) {
			return true
		}
	}
	return false
}

func fields(t schema.Type) schema.FieldList {
	switch t := t.(type) {
	case *schema.Object:
		return t.Fields
	case *schema.Interface:
		return t.Fields
	default:
		return nil
	}
}

func unwrapType(t schema.Type) schema.NamedType {
	if t == nil {
		return nil
	}
	for {
		switch t2 := t.(type) {
		case schema.NamedType:
			return t2
		case *schema.List:
			t = t2.OfType
		case *schema.NonNull:
			t = t2.OfType
		default:
			panic("unreachable")
		}
	}
}

func resolveType(c *context, t schema.Type) schema.Type {
	t2, err := schema.ResolveType(t, c.schema.Resolve)
	if err != nil {
		c.errs = append(c.errs, err)
	}
	return t2
}

func validateDirectives(c *opContext, loc string, directives schema.DirectiveList) {
	directiveNames := make(nameSet)
	for _, d := range directives {
		dirName := d.Name
		validateNameCustomMsg(c.context, directiveNames, d.Name, d.NameLoc, "UniqueDirectivesPerLocation", func() string {
			return fmt.Sprintf("The directive %q can only be used once at this location.", dirName)
		})

		validateArgumentLiterals(c, d.Args)

		dd, ok := c.schema.DeclaredDirectives[dirName]
		if !ok {
			c.addErr(d.NameLoc, "KnownDirectives", "Unknown directive %q.", dirName)
			continue
		}

		locOK := false
		for _, allowedLoc := range dd.Locs {
			if loc == allowedLoc {
				locOK = true
				break
			}
		}
		if !locOK {
			c.addErr(d.NameLoc, "KnownDirectives", "Directive %q may not be used on %s.", dirName, loc)
		}

		validateArgumentTypes(c, d.Args, dd.Args, d.NameLoc,
			func() string { return fmt.Sprintf("directive %q", "@"+dirName) },
			func() string { return fmt.Sprintf("Directive %q", "@"+dirName) },
		)
	}
}

type nameSet map[string]uperrors.Location

func validateName(c *context, set nameSet, name string, loc schema.Location, rule string, kind string) {
	validateNameCustomMsg(c, set, name, loc, rule, func() string {
		return fmt.Sprintf("There can be only one %s named %q.", kind, name)
	})
}

func validateNameCustomMsg(c *context, set nameSet, name string, loc schema.Location, rule string, msg func() string) {
	if loc, ok := set[name]; ok {
		c.addErrMultiLoc([]uperrors.Location{loc, loc}, rule, msg())
		return
	}
	set[name] = loc
}

func validateArgumentTypes(c *opContext, args schema.ArgumentList, argDecls schema.InputValueList, loc uperrors.Location, owner1, owner2 func() string) {
	for _, selArg := range args {
		arg := argDecls.Get(selArg.Name)
		if arg == nil {
			c.addErr(selArg.NameLoc, "KnownArgumentNames", "Unknown argument %q on %s.", selArg.Name, owner1())
			continue
		}
		value := selArg.Value
		if ok, reason := validateValueType(c, value, arg.Type); !ok {
			c.addErr(value.Location(), "ArgumentsOfCorrectType", "Argument %q has invalid value %s.\n%s", arg.Name, value, reason)
		}
	}
	for _, decl := range argDecls {
		if _, ok := decl.Type.(*schema.NonNull); ok {
			if _, ok := args.Get(decl.Name); !ok {
				c.addErr(loc, "ProvidedNonNullArguments", "%s argument %q of type %q is required but not provided.", owner2(), decl.Name, decl.Type)
			}
		}
	}
}

func validateArgumentLiterals(c *opContext, args schema.ArgumentList) {
	argNames := make(nameSet)
	for _, arg := range args {
		validateName(c.context, argNames, arg.Name, arg.NameLoc, "UniqueArgumentNames", "argument")
		validateLiteral(c, arg.Value)
	}
}

func validateLiteral(c *opContext, l schema.Literal) {
	switch l := l.(type) {
	case *schema.ObjectLit:
		fieldNames := make(nameSet)
		for _, f := range l.Fields {
			validateName(c.context, fieldNames, f.Name, f.NameLoc, "UniqueInputFieldNames", "input field")
			validateLiteral(c, f.Value)
		}
	case *schema.ListLit:
		for _, entry := range l.Entries {
			validateLiteral(c, entry)
		}
	case *schema.Variable:
		for _, op := range c.ops {
			v := op.Vars.Get(l.String())
			if v == nil {
				byOp := ""
				if op.Name != "" {
					byOp = fmt.Sprintf(" by operation %q", op.Name)
				}
				c.opErrs[op] = append(c.opErrs[op], (&qerrors.Error{
					QueryError: &uperrors.QueryError {
						Message:   fmt.Sprintf("Variable %q is not defined%s.", l.String(), byOp),
						Locations: []uperrors.Location{l.Loc, op.Loc},
						Rule:      "NoUndefinedVariables",
					},
				}).WithStack())
				continue
			}
			c.usedVars[op][v] = struct{}{}
		}
	}
}

func validateValueType(c *opContext, v schema.Literal, t schema.Type) (bool, string) {
	if v, ok := v.(*schema.Variable); ok {
		for _, op := range c.ops {
			if v2 := op.Vars.Get(v.String()); v2 != nil {
				t2, err := schema.ResolveType(v2.Type, c.schema.Resolve)
				if _, ok := t2.(*schema.NonNull); !ok && v2.Default != nil {
					t2 = &schema.NonNull{OfType: t2}
				}
				if err == nil && !typeCanBeUsedAs(t2, t) {
					c.addErrMultiLoc([]uperrors.Location{v2.Loc, v.Loc}, "VariablesInAllowedPosition", "Variable %q of type %q used in position expecting type %q.", v.String(), t2, t)
				}
			}
		}
		return true, ""
	}

	if nn, ok := t.(*schema.NonNull); ok {
		if isNull(v) {
			return false, fmt.Sprintf("Expected %q, found null.", t)
		}
		t = nn.OfType
	}
	if isNull(v) {
		return true, ""
	}

	switch t := t.(type) {
	case *schema.Scalar, *schema.Enum:
		if lit, ok := v.(*schema.BasicLit); ok {
			if validateBasicLit(lit, t) {
				return true, ""
			}
		}

	case *schema.List:
		list, ok := v.(*schema.ListLit)
		if !ok {
			return validateValueType(c, v, t.OfType) // single value instead of list
		}
		for i, entry := range list.Entries {
			if ok, reason := validateValueType(c, entry, t.OfType); !ok {
				return false, fmt.Sprintf("In element #%d: %s", i, reason)
			}
		}
		return true, ""

	case *schema.InputObject:
		v, ok := v.(*schema.ObjectLit)
		if !ok {
			return false, fmt.Sprintf("Expected %q, found not an object.", t)
		}
		for _, f := range v.Fields {
			name := f.Name
			iv := t.Fields.Get(name)
			if iv == nil {
				return false, fmt.Sprintf("In field %q: Unknown field.", name)
			}
			if ok, reason := validateValueType(c, f.Value, iv.Type); !ok {
				return false, fmt.Sprintf("In field %q: %s", name, reason)
			}
		}
		for _, iv := range t.Fields {
			found := false
			for _, f := range v.Fields {
				if f.Name == iv.Name {
					found = true
					break
				}
			}
			if !found {
				if _, ok := iv.Type.(*schema.NonNull); ok && iv.Default == nil {
					return false, fmt.Sprintf("In field %q: Expected %q, found null.", iv.Name, iv.Type)
				}
			}
		}
		return true, ""
	}

	return false, fmt.Sprintf("Expected type %q, found %s.", t, v)
}

func validateBasicLit(v *schema.BasicLit, t schema.Type) bool {
	switch t := t.(type) {
	case *schema.Scalar:
		switch t.Name {
		case "Int":
			if v.Type != scanner.Int {
				return false
			}
			f, err := strconv.ParseFloat(v.Text, 64)
			if err != nil {
				panic(err)
			}
			return f >= math.MinInt32 && f <= math.MaxInt32
		case "Float":
			return v.Type == scanner.Int || v.Type == scanner.Float
		case "String":
			return v.Type == scanner.String || v.Type == scanner.BlockString
		case "Boolean":
			return v.Type == scanner.Ident && (v.Text == "true" || v.Text == "false")
		case "ID":
			return v.Type == scanner.Int || v.Type == scanner.String || v.Type == scanner.BlockString
		default:
			//TODO: Type-check against expected type by Unmarshalling
			return true
		}

	case *schema.Enum:
		if v.Type != scanner.Ident {
			return false
		}
		for _, option := range t.Values {
			if option.Name == v.Text {
				return true
			}
		}
		return false
	}

	return false
}

func canBeFragment(t schema.Type) bool {
	switch t.(type) {
	case *schema.Object, *schema.Interface, *schema.Union:
		return true
	default:
		return false
	}
}

func canBeInput(t schema.Type) bool {
	switch t := t.(type) {
	case *schema.InputObject, *schema.Scalar, *schema.Enum:
		return true
	case *schema.List:
		return canBeInput(t.OfType)
	case *schema.NonNull:
		return canBeInput(t.OfType)
	default:
		return false
	}
}

func hasSubfields(t schema.Type) bool {
	switch t := t.(type) {
	case *schema.Object, *schema.Interface, *schema.Union:
		return true
	case *schema.List:
		return hasSubfields(t.OfType)
	case *schema.NonNull:
		return hasSubfields(t.OfType)
	default:
		return false
	}
}

func isLeaf(t schema.Type) bool {
	switch t.(type) {
	case *schema.Scalar, *schema.Enum:
		return true
	default:
		return false
	}
}

func isNull(lit interface{}) bool {
	_, ok := lit.(*schema.NullLit)
	return ok
}

func typesCompatible(a, b schema.Type) bool {
	al, aIsList := a.(*schema.List)
	bl, bIsList := b.(*schema.List)
	if aIsList || bIsList {
		return aIsList && bIsList && typesCompatible(al.OfType, bl.OfType)
	}

	ann, aIsNN := a.(*schema.NonNull)
	bnn, bIsNN := b.(*schema.NonNull)
	if aIsNN || bIsNN {
		return aIsNN && bIsNN && typesCompatible(ann.OfType, bnn.OfType)
	}

	if isLeaf(a) || isLeaf(b) {
		return a == b
	}

	return true
}

func typeCanBeUsedAs(t, as schema.Type) bool {
	nnT, okT := t.(*schema.NonNull)
	if okT {
		t = nnT.OfType
	}

	nnAs, okAs := as.(*schema.NonNull)
	if okAs {
		as = nnAs.OfType
		if !okT {
			return false // nullable can not be used as non-null
		}
	}

	if t == as {
		return true
	}

	if lT, ok := t.(*schema.List); ok {
		if lAs, ok := as.(*schema.List); ok {
			return typeCanBeUsedAs(lT.OfType, lAs.OfType)
		}
	}
	return false
}
