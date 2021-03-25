package exec

import (
	"reflect"

	qerrors2 "github.com/chirino/graphql/qerrors"
	"github.com/chirino/graphql/schema"
)

type FieldSelectionContext struct {
	Schema        *schema.Schema
	QueryDocument *schema.QueryDocument
	// Used to provide path context in errors
	Path []string
	// The current type that is being selected against
	OnType schema.Type
	// CanCast is optional.  Used to evaluate if the current OnType can be casted.
	CanCast func(castTo schema.Type) bool
	// Vars are used to evaluate @skip directives
	Vars map[string]interface{}
}

type FieldSelection struct {
	OnType    schema.Type
	Selection *schema.FieldSelection
	Field     *schema.Field
}

func (c FieldSelectionContext) Apply(selections []schema.Selection) (result []FieldSelection, errs qerrors2.ErrorList) {
	c.OnType = schema.DeepestType(c.OnType)
	selectedFieldAliases := map[string]bool{}
	for _, selection := range selections {
		switch selection := selection.(type) {
		case *schema.FieldSelection:
			skip, err := SkipByDirective(selection.Directives, c.Vars)
			if err != nil {
				errs = append(errs, err)
			}
			if skip {
				continue
			}

			fields := schema.FieldList{}
			switch o := c.OnType.(type) {
			case *schema.Object:
				fields = o.Fields
			case *schema.Interface:
				fields = o.Fields
			default:
				panic("unexpected value type: " + reflect.TypeOf(c.OnType).String())
			}

			field := fields.Get(selection.Name)
			if field == nil {
				errs = append(errs, qerrors2.New("field '%s' not found on '%s': ", selection.Name, c.OnType.String()))
			} else {
				selection.Schema = &schema.FieldSchema{
					Field:  field,
					Parent: c.OnType.(schema.NamedType),
				}
				if !selectedFieldAliases[selection.Alias] {
					result = append(result, FieldSelection{
						OnType:    c.OnType,
						Selection: selection,
						Field:     field,
					})
				}
			}

		case *schema.InlineFragment:
			skip, err := SkipByDirective(selection.Directives, c.Vars)
			if err != nil {
				errs = append(errs, err)
			}
			if skip {
				continue
			}

			rs, es := c.applyFragment(selection.Fragment)
			for _, r := range rs {
				if !selectedFieldAliases[r.Selection.Alias] {
					result = append(result, r)
				}
			}
			errs = append(errs, es...)

		case *schema.FragmentSpread:
			skip, err := SkipByDirective(selection.Directives, c.Vars)
			if err != nil {
				errs = append(errs, err)
			}
			if skip {
				continue
			}

			rs, es := c.applyFragment(c.QueryDocument.Fragments.Get(selection.Name).Fragment)
			for _, r := range rs {
				if !selectedFieldAliases[r.Selection.Alias] {
					result = append(result, r)
				}
			}
			errs = append(errs, es...)

		default:
			panic("unexpected selection type: " + reflect.TypeOf(selection).String())
		}
	}
	return
}

func (c FieldSelectionContext) applyFragment(fragment schema.Fragment) ([]FieldSelection, qerrors2.ErrorList) {
	if fragment.On.Name != "" && fragment.On.Name != c.OnType.String() {

		castType := c.Schema.Types[fragment.On.Name]
		if c.CanCast == nil || !c.CanCast(castType) {
			return []FieldSelection{}, qerrors2.ErrorList{}
		}

		castedContext := c
		castedContext.OnType = castType
		return castedContext.Apply(fragment.Selections)
	} else {
		return c.Apply(fragment.Selections)
	}
}
