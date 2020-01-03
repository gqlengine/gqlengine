// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
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

func (engine *Engine) collectInput(baseType reflect.Type) graphql.Type {
	if input, ok := engine.types[baseType]; ok {
		if input != nil {
			return input
		}
		panic("loop-referred input object")
	}
	structType := baseType
	if baseType.Kind() == reflect.Ptr {
		structType = baseType.Elem()
	}

	fields := graphql.InputObjectConfigFieldMap{}
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)

		var fieldType graphql.Type
		if scalar := asBuiltinScalar(f); scalar != nil {
			fieldType = scalar
		} else if id := engine.asIdField(f); id != nil {
			fieldType = id
		} else if input := engine.asInputField(f); input != nil {
			fieldType = input
		} else if enum := engine.asEnumField(f); enum != nil {
			fieldType = enum
		} else if scalar := engine.asCustomScalarField(f); scalar != nil {
			fieldType = scalar
		} else {
			panic(fmt.Errorf("unsupported field type for input: %s", f.Type.String()))
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

	input := newPrototype(baseType).(Input)
	name := structType.Name()
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

	engine.types[baseType] = d
	return d
}

func (engine *Engine) asInputField(field reflect.StructField) graphql.Type {
	isInput, isArray, baseType := implementsOf(field.Type, _inputType)
	if !isInput {
		return nil
	}
	var gtype = engine.collectInput(baseType)
	if isArray {
		gtype = graphql.NewList(gtype)
	}
	return gtype
}
