// Copyright 2020 Karfield Technology. Ltd (凯斐德科技（杭州）有限公司)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gqlengine

import (
	"fmt"
	"reflect"

	"github.com/karfield/graphql"
)

type Input interface {
	GraphQLInputDescription() string
}

type NameAlterableInput interface {
	Input
	GraphQLInputName() string
}

type CustomParserInput interface {
	Input
	GraphQLInputParseValue(map[string]interface{}) interface{}
}

var _inputType = reflect.TypeOf((*Input)(nil)).Elem()

func (engine *Engine) collectInput(info *unwrappedInfo) (graphql.Type, error) {
	if input, ok := engine.types[info.baseType]; ok {
		if input != nil {
			return input, nil
		}
		return nil, fmt.Errorf("loop-referred input object %s", info.baseType.String())
	}
	fields := graphql.InputObjectConfigFieldMap{}
	for i := 0; i < info.baseType.NumField(); i++ {
		f := info.baseType.Field(i)

		fieldType, _, err := checkField(&f, engine.inputFieldCheckers, "input field")
		if err != nil {
			return nil, err
		}
		if fieldType == nil {
			return nil, fmt.Errorf("unsupported field type for input: %s", f.Type.String())
		}
		name := fieldName(&f)
		value, err := defaultValue(&f)
		if err != nil {
			panic(err)
		}
		fields[name] = &graphql.InputObjectFieldConfig{
			Description:  desc(&f),
			Type:         fieldType,
			DefaultValue: value,
		}
	}

	input := newPrototype(info.implType).(Input)
	name := info.baseType.Name()
	if rename, ok := input.(NameAlterableInput); ok {
		name = rename.GraphQLInputName()
	}

	var parseValue graphql.ParseInputValueFn
	if customParse, ok := input.(CustomParserInput); ok {
		parseValue = customParse.GraphQLInputParseValue
	}

	d := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:        name,
		Description: input.GraphQLInputDescription(),
		Fields:      fields,
		ParseValue:  parseValue,
	})

	engine.types[info.baseType] = d
	return d, nil
}

func (engine *Engine) asInputField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isInput, info, err := implementsOf(field.Type, _inputType)
	if err != nil {
		return nil, &info, err
	}
	if !isInput {
		return nil, &info, nil
	}
	gtype, err := engine.collectInput(&info)
	if err != nil {
		return nil, &info, err
	}
	return wrapType(field, gtype, info.array), &info, nil
}
