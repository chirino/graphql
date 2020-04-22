package exec

import (
	"fmt"
	"reflect"

	qerrors "github.com/chirino/graphql/errors"
	"github.com/chirino/graphql/query"
	"github.com/chirino/graphql/schema"
)

type FieldSelectionContext struct {
	Schema        *schema.Schema
	QueryDocument *query.Document
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
	Selection *query.Field
	Field     *schema.Field
}

func (c FieldSelectionContext) Apply(selections []query.Selection) (result []FieldSelection, errs []*qerrors.QueryError) {

	selectedFieldAliases := map[string]bool{}
	for _, selection := range selections {
		switch selection := selection.(type) {
		case *query.Field:
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

			field := fields.Get(selection.Name.Text)
			if field == nil {
				errs = append(errs, (&qerrors.QueryError{
					Message: fmt.Sprintf("field '%s' not found on '%s': ", selection.Name.Text, c.OnType.String()),
				}).WithStack())
			} else {
				if !selectedFieldAliases[selection.Alias.Text] {
					result = append(result, FieldSelection{
						OnType:    c.OnType,
						Selection: selection,
						Field:     field,
					})
				}
			}

		case *query.InlineFragment:
			skip, err := SkipByDirective(selection.Directives, c.Vars)
			if err != nil {
				errs = append(errs, err)
			}
			if skip {
				continue
			}

			rs, es := c.applyFragment(selection.Fragment)
			for _, r := range rs {
				if !selectedFieldAliases[r.Selection.Alias.Text] {
					result = append(result, r)
				}
			}
			errs = append(errs, es...)

		case *query.FragmentSpread:
			skip, err := SkipByDirective(selection.Directives, c.Vars)
			if err != nil {
				errs = append(errs, err)
			}
			if skip {
				continue
			}

			rs, es := c.applyFragment(c.QueryDocument.Fragments.Get(selection.Name.Text).Fragment)
			for _, r := range rs {
				if !selectedFieldAliases[r.Selection.Alias.Text] {
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

func (c FieldSelectionContext) applyFragment(fragment query.Fragment) ([]FieldSelection, []*qerrors.QueryError) {
	if fragment.On.Text != "" && fragment.On.Text != c.OnType.String() {

		castType := c.Schema.Types[fragment.On.Text]
		if c.CanCast == nil || !c.CanCast(castType) {
			return []FieldSelection{}, []*qerrors.QueryError{}
		}

		castedContext := c
		castedContext.OnType = castType
		return castedContext.Apply(fragment.Selections)
	} else {
		return c.Apply(fragment.Selections)
	}
}
