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

type Object interface {
	GraphQLObjectDescription() string
}

type NameAlterableObject interface {
	Object
	GraphQLObjectName() string
}

type ImplementedObject interface {
	Object
	GraphQLObjectInterfaces() []Interface
}

type ObjectDelegation interface {
	Object
	GraphQLObjectDelegation() interface{}
}

type IsGraphQLObject struct{}

var (
	_objectType          = reflect.TypeOf((*Object)(nil)).Elem()
	_isGraphQLObjectType = reflect.TypeOf(IsGraphQLObject{})
)

type objectSourceBuilder struct {
	unwrappedInfo
}

func (o *objectSourceBuilder) build(params graphql.ResolveParams) (reflect.Value, error) {
	return reflect.ValueOf(params.Source), nil
}

type objectResolvers map[string]graphql.FieldResolveFn

func (engine *Engine) objectFields(baseType reflect.Type, config *objectFieldLazyConfig, asInterface bool) error {
	for i := 0; i < baseType.NumField(); i++ {
		f := baseType.Field(i)

		if isIgnored(&f) {
			continue
		}
		if asInterface && isMatchedFieldType(f.Type, _isGraphQLInterfaceType) {
			continue
		}
		if !asInterface && isMatchedFieldType(f.Type, _isGraphQLObjectType) {
			continue
		}

		if f.Anonymous {
			// embedded
			embeddedInfo, err := unwrap(f.Type)
			if err != nil {
				return fmt.Errorf("check object field '%s' failure: %E", baseType.String(), err)
			}
			if embeddedInfo.array {
				return fmt.Errorf("embedded object type should be struct")
			}
			err = engine.objectFields(embeddedInfo.baseType, config, false)
			if err != nil {
				return err
			}
			continue
		}

		fieldType, _, err := checkField(&f, engine.objFieldCheckers, "object field")
		if err != nil {
			return err
		}
		if fieldType == nil {
			panic(fmt.Errorf("unsupported field type: %s", f.Type.String()))
		}

		config.fields[fieldName(&f)] = objectField{
			typ:        fieldType,
			desc:       desc(&f),
			deprecated: deprecatedReason(&f),
		}
	}
	return nil
}

type objectField struct {
	name       string
	typ        graphql.Type
	desc       string
	deprecated string
}

type objectFieldLazyConfig struct {
	fields map[string]objectField
}

func (c *objectFieldLazyConfig) getFields(obj reflect.Type, engine *Engine) graphql.FieldsThunk {
	return func() graphql.Fields {
		fields := graphql.Fields{}
		for name, config := range c.fields {
			f := &graphql.Field{
				Name:              config.name,
				Description:       config.desc,
				Type:              config.typ,
				DeprecationReason: config.deprecated,
			}
			fields[name] = f
		}
		return fields
	}
}

func (engine *Engine) collectObject(info *unwrappedInfo, desc string) (graphql.Type, error) {
	if obj, ok := engine.types[info.baseType]; ok {
		return obj, nil
	}

	var prototype Object
	if desc == "" {
		prototype = newPrototype(info.implType).(Object)
	}

	name := info.baseType.Name()
	if prototype != nil {
		if rename, ok := prototype.(NameAlterableObject); ok {
			name = rename.GraphQLObjectName()
		}
	}

	baseType := info.baseType
	if prototype != nil {
		if delegated, ok := prototype.(ObjectDelegation); ok {
			objPrototype := delegated.GraphQLObjectDelegation()
			delegatedType := reflect.TypeOf(objPrototype)
			info, err := unwrap(delegatedType)
			if err != nil {
				return nil, fmt.Errorf("collect delegated object failure %E", err)
			}
			if info.array {
				return nil, fmt.Errorf("delegated prototype should not be non-struct")
			}
			if info.baseType.Kind() != reflect.Struct {
				return nil, fmt.Errorf("delegated type of '%s' should be an object but '%s'",
					baseType.Name(), delegatedType.String())
			}
			baseType = info.baseType
		}
	}

	fieldsConfig := objectFieldLazyConfig{
		fields: map[string]objectField{},
	}
	err := engine.objectFields(baseType, &fieldsConfig, false)
	if err != nil {
		return nil, err
	}

	var intfs graphql.Interfaces
	if prototype != nil {
		if impl, ok := prototype.(ImplementedObject); ok {
			for _, intfPrototype := range impl.GraphQLObjectInterfaces() {
				intf, err := engine.asInterfaceFromPrototype(intfPrototype)
				if err != nil {
					return nil, fmt.Errorf("check type '%s' implemented infterface '%s' failed %E",
						info.baseType.Name(), reflect.TypeOf(intfPrototype).Name(), err)
				}
				intfs = append(intfs, intf)
			}
		}
	}

	if prototype != nil {
		desc = prototype.GraphQLObjectDescription()
	}

	object := graphql.NewObject(graphql.ObjectConfig{
		Name:        name,
		Description: desc,
		Fields:      fieldsConfig.getFields(info.baseType, engine),
		Interfaces:  intfs,
	})
	engine.types[info.baseType] = object

	return object, nil
}

func (engine *Engine) asObject(p reflect.Type) (typ graphql.Type, info unwrappedInfo, err error) {
	var isObj bool
	isObj, info, err = implementsOf(p, _objectType)
	if err != nil {
		return
	}
	var description string
	if !isObj {
		info, err = unwrap(p)
		if err != nil {
			return
		}
		fieldIdx, tag := findBaseTypeFieldTag(info.baseType, _isGraphQLObjectType)
		if fieldIdx < 0 {
			return
		}
		description = tag.Get(gqlDesc)
		if description == "" {
			err = fmt.Errorf("mark %s as 'IsGraphQLObject' but missing 'gqlDesc' tag", p.String())
			return
		}
	}
	typ, err = engine.collectObject(&info, description)
	return
}

func (engine *Engine) asObjectSource(p reflect.Type) (resolverArgumentBuilder, *unwrappedInfo, error) {
	_, info, err := engine.asObject(p)
	if err != nil {
		return nil, &info, err
	}
	return &objectSourceBuilder{
		unwrappedInfo: info,
	}, &info, nil
}

func (engine *Engine) asObjectResult(p reflect.Type) (*unwrappedInfo, error) {
	_, info, err := engine.asObject(p)
	if err != nil {
		return &info, err
	}
	return &info, nil
}

func (engine *Engine) asObjectField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	typ, info, err := engine.asObject(field.Type)
	if err != nil {
		return nil, &info, err
	}
	return wrapType(field, typ, info.array), &info, nil
}
