// Copyright 2020 凯斐德科技（杭州）有限公司 (Karfield Technology, ltd.)
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

type IsGraphQLInput struct{}

var (
	_inputType          = reflect.TypeOf((*Input)(nil)).Elem()
	_isGraphQLInputType = reflect.TypeOf(IsGraphQLInput{})
)

type inputLazyConfig struct {
	fields     graphql.InputObjectConfigFieldMap
	pluginData map[string]interface{}
	pluginErr  map[string][]error
}

func (c *inputLazyConfig) addPluginError(name string, err error) {
	if c.pluginErr == nil {
		c.pluginErr = map[string][]error{}
	}
	c.pluginErr[name] = append(c.pluginErr[name], err)
}

func (engine *Engine) callPluginsOnCheckingInputObject(config *inputLazyConfig, do func(pluginData interface{}, plugin Plugin) error) {
	engine.callPluginsSafely(func(name string, plugin Plugin) error {
		return do(config.pluginData[name], plugin)
	}, func(name string, err error) {
		config.addPluginError(name, err)
	})
}

func (engine *Engine) unwrapInputFields(baseType reflect.Type, config *inputLazyConfig, depth int) error {
	for i := 0; i < baseType.NumField(); i++ {
		f := baseType.Field(i)

		if isIgnored(&f) || isMatchedFieldType(f.Type, _isGraphQLInputType) {
			continue
		}

		if isEmptyStructField(&f) {
			engine.callPluginsOnCheckingInputObject(config, func(pluginData interface{}, plugin Plugin) error {
				return plugin.CheckInputObjectEmbeddedFieldTags(pluginData, &f)
			})
			continue
		}

		if f.Anonymous {
			embeddedInfo, err := unwrap(f.Type)
			if err != nil {
				return fmt.Errorf("check input field '%s' failure: %E", baseType.String(), err)
			}
			if embeddedInfo.array {
				return fmt.Errorf("embedded input field type should be struct, not slice")
			}
			if err := engine.unwrapInputFields(embeddedInfo.baseType, config, depth+1); err != nil {
				return err
			}
			continue
		}

		fieldType, _, err := checkField(&f, engine.inputFieldCheckers, "input field")
		if err != nil {
			return err
		}
		if fieldType == nil {
			return fmt.Errorf("unsupported field type for input: %s", f.Type.String())
		}
		name := fieldName(&f)
		value, err := defaultValue(&f)
		if err != nil {
			panic(err)
		}
		fc := &graphql.InputObjectFieldConfig{
			Description:  desc(&f),
			Type:         fieldType,
			DefaultValue: value,
		}

		engine.callPluginsOnCheckingInputObject(config, func(pluginData interface{}, plugin Plugin) error {
			return plugin.CheckInputObjectField(pluginData, name, fieldType, &f.Tag, f.Type)
		})
		config.fields[name] = fc
	}
	return nil
}

func (engine *Engine) collectInput(info *unwrappedInfo, tag *reflect.StructTag) (*graphql.InputObject, error) {
	if input, ok := engine.types[info.baseType]; ok {
		if input != nil {
			return input.(*graphql.InputObject), nil
		}
		return nil, fmt.Errorf("loop-referred input object %s", info.baseType.String())
	}

	config := inputLazyConfig{fields: graphql.InputObjectConfigFieldMap{}}

	engine.callPluginsSafely(func(name string, plugin Plugin) error {
		if config.pluginData == nil {
			config.pluginData = map[string]interface{}{}
		}
		config.pluginData[name] = plugin.BeforeCheckInputStruct(info.baseType)
		return nil
	}, func(name string, err error) {
		config.addPluginError(name, err)
	})

	if err := engine.unwrapInputFields(info.baseType, &config, 0); err != nil {
		return nil, err
	}

	var input Input
	if tag == nil {
		input = newPrototype(info.implType).(Input)
	}

	name := info.baseType.Name()
	if input != nil {
		if rename, ok := input.(NameAlterableInput); ok {
			name = rename.GraphQLInputName()
		}
	}

	var parseValue graphql.ParseInputValueFn
	if input != nil {
		if customParse, ok := input.(CustomParserInput); ok {
			parseValue = customParse.GraphQLInputParseValue
		}
	}

	description := ""
	if input != nil {
		description = input.GraphQLInputDescription()
	}
	if tag != nil {
		if s := tag.Get(gqlName); s != "" {
			name = s
		}
		if s := tag.Get(gqlDesc); s != "" {
			description = s
		}
	}

	engine.callPluginOnMethod(info.implType, func(method reflect.Method, prototype reflect.Value) {
		engine.callPluginsOnCheckingInputObject(&config, func(pluginData interface{}, plugin Plugin) error {
			return plugin.MatchAndCallInputObjectMethod(pluginData, method, prototype)
		})
	})

	d := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:        name,
		Description: description,
		Fields:      config.fields,
		ParseValue:  parseValue,
	})

	engine.callPluginsOnCheckingInputObject(&config, func(pluginData interface{}, plugin Plugin) error {
		return plugin.AfterCheckInputStruct(pluginData, d)
	})

	engine.types[info.baseType] = d
	return d, nil
}

func (engine *Engine) asInputField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isInput, info, err := implementsOf(field.Type, _inputType)
	if err != nil {
		return nil, &info, err
	}
	var tag *reflect.StructTag
	if !isInput {
		info, err = unwrap(field.Type)
		if err != nil {
			return nil, &info, err
		}
		idx, t := findBaseTypeFieldTag(info.baseType, _isGraphQLInputType)
		if idx < 0 {
			return nil, &info, nil
		}
		tag = &t
	}
	gtype, err := engine.collectInput(&info, tag)
	if err != nil {
		return nil, &info, err
	}
	return wrapType(field, gtype, info.array), &info, nil
}

func (engine *Engine) RegisterInput(prototype interface{}) (*graphql.InputObject, error) {
	typ := reflect.TypeOf(prototype)
	isInput, info, err := implementsOf(typ, _inputType)
	if err != nil {
		return nil, err
	}
	if !isInput {
		return nil, nil
	}
	return engine.collectInput(&info, nil)
}
