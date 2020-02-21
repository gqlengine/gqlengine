package gqlengine

import (
	"reflect"
	"strings"

	"github.com/karfield/graphql"
)

type FieldSelection interface {
	IsSelected(fieldNames ...string) bool
}

var _fieldSelectionType = reflect.TypeOf((*FieldSelection)(nil)).Elem()

type fieldSelectionSet struct {
	*graphql.ResolveInfo
}

func (f *fieldSelectionSet) IsSelected(fieldNames ...string) bool {
	for i := 0; i < len(fieldNames); i++ {
		name := fieldNames[i]
		if strings.HasPrefix(name, f.FieldName) {
			continue
		}
		if !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "*/") {
			fieldNames[i] = "*/" + name
		}
	}
	return f.IsFieldSelected(fieldNames...)
}

type fieldSelectionBuilder struct{}

func (f *fieldSelectionBuilder) build(params graphql.ResolveParams) (reflect.Value, error) {
	var s FieldSelection = &fieldSelectionSet{&params.Info}
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
