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

var _argumentsType = reflect.TypeOf((*Arguments)(nil)).Elem()

type argumentsBuilder struct {
	typ reflect.Type
}

func (a argumentsBuilder) build(params graphql.ResolveParams) (interface{}, error) {
	val := reflect.New(a.typ)
	err := mapstructure.Decode(params.Args, val.Interface())
	if err != nil {
		return nil, fmt.Errorf("unmarshal arguments failed: %E", err)
	}
	if a.typ.Kind() == reflect.Ptr {
		return val.Interface(), nil
	}
	return val.Elem().Interface(), nil
}

func (engine *Engine) collectFieldArgumentConfig(baseType reflect.Type) error {
	if _, ok := engine.argConfigs[baseType]; ok {
		return nil
	}

	structType := baseType
	if baseType.Kind() == reflect.Ptr {
		structType = baseType.Elem()
	}

	defs := graphql.FieldConfigArgument{}
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)

		var gType graphql.Type
		if scalar := asBuiltinScalar(f); scalar != nil {
			gType = scalar
		} else if id := engine.asIdField(f); id != nil {
			gType = id
		} else if input := engine.asInputField(f); input != nil {
			gType = input
		} else if enum := engine.asEnumField(f); enum != nil {
			gType = enum
		} else if scalar := engine.asCustomScalarField(f); scalar != nil {
			gType = scalar
		} else {
			// FIXME: no more input field
			return fmt.Errorf("argument config '%s' has unsupported field[%d] (type: %s}", baseType.Name(), i, f.Name)
		}

		if isRequired(&f) {
			gType = graphql.NewNonNull(gType)
		}

		name := fieldName(&f)
		value, err := defaultValue(&f)
		if err != nil {
			return err
		}

		defs[name] = &graphql.ArgumentConfig{
			Type:         gType,
			DefaultValue: value,
			Description:  desc(&f),
		}
	}

	engine.argConfigs[baseType] = defs
	return nil
}

func (engine *Engine) asArguments(arg reflect.Type) *argumentsBuilder {
	isArg, isArray, baseType := implementsOf(arg, _argumentsType)
	if isArray || !isArg {
		return nil
	}
	err := engine.collectFieldArgumentConfig(baseType)
	if err != nil {
		panic(err)
	}
	return &argumentsBuilder{
		typ: baseType,
	}
}
