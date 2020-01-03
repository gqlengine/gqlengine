// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
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

var interfaceType = reflect.TypeOf((*Interface)(nil)).Elem()

func (engine *Engine) collectInterface(p reflect.Type, prototype Interface) (*graphql.Interface, bool) {
	isInterface, isArray, baseType := implementsOf(p, interfaceType)
	if !isInterface {
		return nil, isArray
	}

	if i, ok := engine.types[baseType]; ok {
		return i.(*graphql.Interface), isArray
	}

	structType := baseType
	if baseType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	if prototype == nil {
		pv := reflect.New(structType)
		if baseType.Kind() != reflect.Ptr {
			pv = pv.Elem()
		}
		prototype = pv.Interface().(Interface)
	}

	name := structType.Name()
	if p, ok := prototype.(NameAlterableInterface); ok {
		name = p.GraphQLInterfaceName()
	}

	intf := graphql.NewInterface(graphql.InterfaceConfig{
		Name:        name,
		Description: prototype.GraphQLInterfaceDescription(),
		Fields:      graphql.Fields{},
	})

	engine.types[baseType] = intf

	fieldsConfig := objectFieldLazyConfig{
		fields: map[string]objectField{},
	}
	engine.objectFields(structType, &fieldsConfig)

	for name, f := range fieldsConfig.fields {
		intf.AddFieldConfig(name, &graphql.Field{
			Name:              f.name,
			Description:       f.desc,
			DeprecationReason: f.deprecated,
			Type:              f.typ,
			// FIXME: need to support args
		})
	}

	return intf, isArray
}

func (engine *Engine) asInterfaceFromPrototype(prototype Interface) *graphql.Interface {
	i, _ := engine.collectInterface(reflect.TypeOf(prototype), prototype)
	return i
}
