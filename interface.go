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

type Interface interface {
	GraphQLInterfaceDescription() string
}

type NameAlterableInterface interface {
	Interface
	GraphQLInterfaceName() string
}

type IsGraphQLInterface struct{}

var _isGraphQLInterfaceType = reflect.TypeOf(IsGraphQLInterface{})

type interfaceConfig struct {
	model     reflect.Type
	prototype interface{}
	typ       *graphql.Interface
}

func (engine *Engine) PreRegisterInterface(interfacePrototype, modelPrototype interface{}) error {
	interfaceType := unwrapForKind(interfacePrototype, reflect.Interface)
	if interfaceType == nil {
		return fmt.Errorf("PreRegisterInterfacePrototype(): first argument should be an interface")
	}
	modelType := unwrapForKind(modelPrototype, reflect.Struct)
	if modelType == nil {
		return fmt.Errorf("PreRegisterInterfacePrototype(): second argument should be a struct")
	}

	if _, ok := engine.interfaces[interfaceType]; ok {
		return nil
	}

	intf := graphql.Interface{}
	engine.types[interfaceType] = &intf

	name := interfaceType.Name()
	description := ""
	if ifPp, ok := modelPrototype.(Interface); ok {
		description = ifPp.GraphQLInterfaceDescription()
		if ifPp, ok := ifPp.(NameAlterableInterface); ok {
			name = ifPp.GraphQLInterfaceName()
		}
	} else {
		iterateStructTypeFields(modelType, func(field *reflect.StructField) {
			if isMatchedFieldType(field.Type, _isGraphQLInterfaceType) {
				if s := field.Tag.Get(gqlName); s != "" {
					name = s
				}
				description = field.Tag.Get(gqlDesc)
			}
		})
	}

	err := graphql.InitInterface(&intf, graphql.InterfaceConfig{
		Name:        name,
		Fields:      graphql.Fields{},
		Description: description,
		ResolveType: func(p graphql.ResolveTypeParams) *graphql.Object {
			pt := reflect.TypeOf(p.Value)
			if pt.Implements(interfaceType) {
				if info, err := unwrap(pt); err == nil {
					if typ, ok := engine.types[info.baseType]; ok {
						if obj, ok := typ.(*graphql.Object); ok {
							return obj
						}
					}
				}
			}
			return nil
		},
	})
	if err != nil {
		return err
	}

	engine.interfaces[interfaceType] = interfaceConfig{
		typ:   &intf,
		model: modelType,
	}

	return nil
}

func (engine *Engine) scanObjectImplementedInterfaces(info *unwrappedInfo) (interfaces graphql.Interfaces) {
	for ifType, ifConfig := range engine.interfaces {
		if info.implType.Implements(ifType) ||
			(info.baseType != info.implType && info.baseType.Implements(ifType)) ||
			(info.ptrType != nil && info.ptrType != info.implType && info.ptrType.Implements(ifType)) {
			interfaces = append(interfaces, ifConfig.typ)
		}
	}
	return
}

func (engine *Engine) completeInterfaceFields() error {
	for _, ifConfig := range engine.interfaces {

		fieldsConfig := objectFieldLazyConfig{
			fields:     map[string]*objectField{},
			pluginData: map[string]interface{}{},
		}
		err := engine.unwrapObjectFields(ifConfig.model, &fieldsConfig, true, 0)
		if err != nil {
			return fmt.Errorf("check interface '%s' failed: %E", ifConfig.model.Name(), err)
		}

		for name, f := range fieldsConfig.fields {
			ifConfig.typ.AddFieldConfig(name, &graphql.Field{
				Name:              f.name,
				Args:              f.args,
				Description:       f.desc,
				DeprecationReason: f.deprecated,
				Type:              f.typ,
			})
		}
	}
	return nil
}

func (engine *Engine) asInterfaceResult(p reflect.Type) (*unwrappedInfo, error) {
	_, intfType := unwrapInterface(p)
	if intfType == nil {
		return nil, nil
	}
	if _, ok := engine.interfaces[intfType]; ok {
		return &unwrappedInfo{
			ptrType:  intfType,
			implType: intfType,
			baseType: intfType,
		}, nil
	}
	return nil, nil
}

func (engine *Engine) asInterfaceField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	array, intfType := unwrapInterface(field.Type)
	if intfType == nil {
		return nil, nil, nil
	}
	if ifConfig, ok := engine.interfaces[intfType]; ok {
		return wrapType(field, ifConfig.typ, array), &unwrappedInfo{
			ptrType:  intfType,
			implType: intfType,
			baseType: intfType,
		}, nil
	}
	return nil, nil, nil
}
