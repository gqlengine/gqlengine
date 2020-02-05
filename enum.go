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
	"reflect"

	"github.com/karfield/graphql"
)

type EnumValue struct {
	Value       interface{}
	Description string
}

type EnumValueMapping map[string]EnumValue

type Enum interface {
	GraphQLEnumDescription() string
	GraphQLEnumValues() EnumValueMapping
}

type NameAlterableEnum interface {
	Enum
	GraphQLEnumName() string
}

var enumType = reflect.TypeOf((*Enum)(nil)).Elem()

func (engine *Engine) collectEnum(info *unwrappedInfo) *graphql.Enum {
	if d, ok := engine.types[info.baseType]; ok {
		return d.(*graphql.Enum)
	}
	enum := newPrototype(info.implType).(Enum)

	values := graphql.EnumValueConfigMap{}
	for valName, def := range enum.GraphQLEnumValues() {
		values[valName] = &graphql.EnumValueConfig{
			Value:       def.Value,
			Description: def.Description,
		}
	}

	name := info.baseType.Name()
	if rename, ok := enum.(NameAlterableEnum); ok {
		name = rename.GraphQLEnumName()
	}

	d := graphql.NewEnum(graphql.EnumConfig{
		Name:        name,
		Description: enum.GraphQLEnumDescription(),
		Values:      values,
	})
	engine.types[info.baseType] = d
	return d
}

func (engine *Engine) asEnumField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isEnum, info, err := implementsOf(field.Type, enumType)
	if err != nil {
		return nil, &info, err
	}
	if !isEnum {
		return nil, &info, nil
	}
	var gType graphql.Type = engine.collectEnum(&info)
	gType = wrapType(field, gType, info.array)
	return gType, &info, nil
}

func (engine *Engine) asEnumResult(out reflect.Type) (*unwrappedInfo, error) {
	isEnum, info, err := implementsOf(out, enumType)
	if err != nil {
		return nil, err
	}
	if !isEnum {
		return nil, nil
	}
	engine.collectEnum(&info)
	return &info, nil
}

func (engine *Engine) registerEnum(typ reflect.Type) (*graphql.Enum, error) {
	isEnum, info, err := implementsOf(typ, enumType)
	if err != nil {
		return nil, err
	}
	if !isEnum {
		return nil, nil
	}
	return engine.collectEnum(&info), nil
}

func (engine *Engine) RegisterEnum(prototype interface{}) (*graphql.Enum, error) {
	typ := reflect.TypeOf(prototype)
	return engine.registerEnum(typ)
}
