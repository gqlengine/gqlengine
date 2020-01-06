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

func (engine *Engine) collectInput(baseType reflect.Type) (graphql.Type, error) {
	if input, ok := engine.types[baseType]; ok {
		if input != nil {
			return input, nil
		}
		return nil, fmt.Errorf("loop-referred input object %s", baseType.String())
	}
	structType := baseType
	if baseType.Kind() == reflect.Ptr {
		structType = baseType.Elem()
	}

	fields := graphql.InputObjectConfigFieldMap{}
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)

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
	gtype, err := engine.collectInput(info.baseType)
	if err != nil {
		return nil, &info, err
	}
	if info.array {
		gtype = graphql.NewList(gtype)
	}
	return gtype, &info, nil
}
