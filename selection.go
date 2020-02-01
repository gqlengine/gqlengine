package gqlengine

import (
	"reflect"

	"github.com/karfield/graphql"
	"github.com/karfield/graphql/language/ast"
)

type FieldSelection interface {
	IsSelected(fieldNames ...string) bool
}

var _fieldSelectionType = reflect.TypeOf((*FieldSelection)(nil)).Elem()

type fieldSelectionSet struct {
	set map[string]*ast.Field
}

func (f *fieldSelectionSet) IsSelected(fieldNames ...string) bool {
	if len(fieldNames) == 0 {
		return false
	}
	for _, name := range fieldNames {
		if _, ok := f.set[name]; ok {
			return true
		}
	}
	return false
}

type fieldSelectionBuilder struct{}

func (f *fieldSelectionBuilder) build(params graphql.ResolveParams) (reflect.Value, error) {
	var s FieldSelection = &fieldSelectionSet{params.Info.FieldSelectionSet}
	return reflect.ValueOf(s), nil
}

func (engine *Engine) asFieldSelection(in reflect.Type) (*fieldSelectionBuilder, error) {
	unwrappedInfo, err := unwrap(in)
	if err != nil {
		return nil, err
	}
	if unwrappedInfo.array {
		return nil, nil
	}
	if unwrappedInfo.baseType.Kind() == reflect.Interface {
		if unwrappedInfo.baseType == _fieldSelectionType {
			return &fieldSelectionBuilder{}, nil
		}
	}
	return nil, nil
}
