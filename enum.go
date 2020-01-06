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
	enum := newPrototype(baseType).(Enum)

	values := graphql.EnumValueConfigMap{}
	for valName, def := range enum.GraphQLEnumValues() {
		values[valName] = &graphql.EnumValueConfig{
			Value:       def.Value,
			Description: def.Description,
		}
	}

	name := baseType.Name()
	if baseType.Kind() == reflect.Ptr {
		name = baseType.Elem().Name()
	}
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

func (engine *Engine) asEnumField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isEnum, info, err := implementsOf(field.Type, enumType)
	if err != nil {
		return nil, &info, err
	}
	if !isEnum {
		return nil, &info, nil
	}
	var gType = engine.collectEnum(info.baseType)
	if info.array {
		gType = graphql.NewList(gType)
	}
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
	engine.collectEnum(info.baseType)
	return &info, nil
}
