// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
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

func (engine *Engine) collectEnum(baseType reflect.Type) graphql.Type {
	if d, ok := engine.types[baseType]; ok {
		return d
	}
	typ := baseType
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	v := reflect.New(typ)
	if baseType.Kind() != reflect.Ptr {
		v = v.Elem()
	}
	enum := v.Interface().(Enum)

	values := graphql.EnumValueConfigMap{}
	for valName, def := range enum.GraphQLEnumValues() {
		values[valName] = &graphql.EnumValueConfig{
			Value:       def.Value,
			Description: def.Description,
		}
	}

	name := typ.Name()
	if rename, ok := enum.(NameAlterableEnum); ok {
		name = rename.GraphQLEnumName()
	}

	d := graphql.NewEnum(graphql.EnumConfig{
		Name:        name,
		Description: enum.GraphQLEnumDescription(),
		Values:      values,
	})
	engine.types[baseType] = d
	return d
}

func (engine *Engine) asEnumField(field reflect.StructField) graphql.Type {
	isEnum, isArray, baseType := implementsOf(field.Type, enumType)
	if !isEnum {
		return nil
	}
	var gType = engine.collectEnum(baseType)
	if isArray {
		gType = graphql.NewList(gType)
	}
	return gType
}

func (engine *Engine) asEnumResult(out reflect.Type) reflect.Type {
	isEnum, _, baseType := implementsOf(out, enumType)
	if !isEnum {
		return baseType
	}
	engine.collectEnum(baseType)
	return baseType
}
