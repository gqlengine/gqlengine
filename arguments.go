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
	"github.com/mitchellh/mapstructure"
)

type Arguments interface {
	GraphQLArguments()
}

type IsGraphQLArguments struct{}

var (
	_argumentsType      = reflect.TypeOf((*Arguments)(nil)).Elem()
	_isGraphQLArguments = reflect.TypeOf(IsGraphQLArguments{})
)

type argumentsBuilder struct {
	ptr bool
	typ reflect.Type
}

func unmarshalArguments(params graphql.ResolveParams, requirePtr bool, typ reflect.Type) (reflect.Value, error) {
	val := reflect.New(typ)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           val.Interface(),
		WeaklyTypedInput: true,
		TagName:          "json",
	})
	if err == nil {
		err = decoder.Decode(params.Args)
	}
	if err != nil {
		return reflect.Value{}, fmt.Errorf("unmarshal arguments failed: %E", err)
	}
	if !requirePtr {
		return val.Elem(), nil
	}
	return val, nil
}

func (a argumentsBuilder) build(params graphql.ResolveParams) (reflect.Value, error) {
	return unmarshalArguments(params, a.ptr, a.typ)
}

type fieldChecker func(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error)

type argsLazyConfig struct {
	args graphql.FieldConfigArgument
}

func (engine *Engine) unwrapArgsFields(baseType reflect.Type, config *argsLazyConfig) error {
	for i := 0; i < baseType.NumField(); i++ {
		f := baseType.Field(i)
		if isIgnored(&f) || isMatchedFieldType(f.Type, _isGraphQLArguments) {
			continue
		}
		if f.Anonymous {
			// embedded
			embeddedInfo, err := unwrap(f.Type)
			if err != nil {
				return fmt.Errorf("check argument '%s' failure: %E", baseType.String(), err)
			}
			if embeddedInfo.array {
				return fmt.Errorf("embedded arguments type should be struct, not slice")
			}
			err = engine.unwrapArgsFields(embeddedInfo.baseType, config)
			if err != nil {
				return err
			}
			continue
		}

		gType, _, err := checkField(&f, engine.inputFieldCheckers, "argument")
		if err != nil {
			return err
		}
		if gType == nil {
			return fmt.Errorf("unsupported type '%s' for argument[%d] '%s'", baseType.Name(), i, f.Name)
		}

		name := fieldName(&f)
		value, err := defaultValue(&f)
		if err != nil {
			return err
		}

		config.args[name] = &graphql.ArgumentConfig{
			Type:         gType,
			DefaultValue: value,
			Description:  desc(&f),
		}
	}
	return nil
}

func (engine *Engine) collectFieldArgumentConfig(baseType reflect.Type) (graphql.FieldConfigArgument, error) {
	if _, ok := engine.argConfigs[baseType]; ok {
		return nil, nil
	}
	config := argsLazyConfig{
		args: graphql.FieldConfigArgument{},
	}
	if err := engine.unwrapArgsFields(baseType, &config); err != nil {
		return nil, err
	}
	engine.argConfigs[baseType] = config.args
	return config.args, nil
}

func (engine *Engine) asArguments(arg reflect.Type) (*argumentsBuilder, *unwrappedInfo, error) {
	isArg, info, err := implementsOf(arg, _argumentsType)
	if err != nil {
		return nil, &info, err
	}
	if info.array {
		return nil, &info, fmt.Errorf("arguments object should not be a slice/array")
	}
	if !isArg {
		info, err = unwrap(arg)
		if err != nil {
			return nil, &info, err
		}
		idx, _ := findBaseTypeFieldTag(info.baseType, _isGraphQLArguments)
		if idx < 0 {
			return nil, &info, nil
		}
	}
	_, err = engine.collectFieldArgumentConfig(info.baseType)
	if err != nil {
		return nil, &info, err
	}
	return &argumentsBuilder{
		ptr: arg.Kind() == reflect.Ptr,
		typ: info.baseType,
	}, &info, nil
}
