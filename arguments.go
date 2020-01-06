// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
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
	ptr bool
	typ reflect.Type
}

func unmarshalArguments(params graphql.ResolveParams, requirePtr bool, typ reflect.Type) (interface{}, error) {
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
		return nil, fmt.Errorf("unmarshal arguments failed: %E", err)
	}
	if !requirePtr {
		return val.Elem().Interface(), nil
	}
	return val.Interface(), nil
}

func (a argumentsBuilder) build(params graphql.ResolveParams) (interface{}, error) {
	return unmarshalArguments(params, a.ptr, a.typ)
}

type fieldChecker func(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error)

func (engine *Engine) collectFieldArgumentConfig(baseType reflect.Type) error {
	if _, ok := engine.argConfigs[baseType]; ok {
		return nil
	}

	defs := graphql.FieldConfigArgument{}
	for i := 0; i < baseType.NumField(); i++ {
		f := baseType.Field(i)

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

		defs[name] = &graphql.ArgumentConfig{
			Type:         gType,
			DefaultValue: value,
			Description:  desc(&f),
		}
	}

	engine.argConfigs[baseType] = defs
	return nil
}

func (engine *Engine) asArguments(arg reflect.Type) (*argumentsBuilder, *unwrappedInfo, error) {
	isArg, info, err := implementsOf(arg, _argumentsType)
	if err != nil {
		return nil, &info, err
	}
	if !isArg {
		return nil, &info, nil
	}
	if info.array {
		return nil, &info, fmt.Errorf("arguments object should not be a slice/array")
	}
	err = engine.collectFieldArgumentConfig(info.baseType)
	if err != nil {
		return nil, &info, err
	}
	return &argumentsBuilder{
		ptr: arg.Kind() == reflect.Ptr,
		typ: arg,
	}, &info, nil
}
