package gqlengine

import (
	"reflect"
	"runtime/debug"

	"github.com/karfield/graphql"
)

type Plugin interface {
	BeforeCheckArgumentsStruct(baseType reflect.Type) interface{}
	CheckArgumentsEmbeddedField(pluginData interface{}, field *reflect.StructField) error
	CheckArgument(pluginData interface{}, name string, typ graphql.Type, tag *reflect.StructTag, goType reflect.Type, defaultValue interface{}) error
	MatchAndCallArgumentsMethod(pluginData interface{}, method reflect.Method, prototype reflect.Value) error
	AfterCheckArgumentsStruct(pluginData interface{}) error

	BeforeCheckObjectStruct(baseType reflect.Type) interface{}
	CheckObjectEmbeddedFieldTags(pluginData interface{}, field *reflect.StructField) error
	CheckObjectField(pluginData interface{}, name string, typ graphql.Type, tag *reflect.StructTag, goType reflect.Type) error
	MatchAndCallObjectMethod(pluginData interface{}, method reflect.Method, prototype reflect.Value) error
	AfterCheckObjectStruct(pluginData interface{}, obj *graphql.Object) error

	BeforeCheckInputStruct(baseType reflect.Type) interface{}
	CheckInputObjectEmbeddedFieldTags(pluginData interface{}, field *reflect.StructField) error
	CheckInputObjectField(pluginData interface{}, name string, typ graphql.Type, tag *reflect.StructTag, goType reflect.Type) error
	MatchAndCallInputObjectMethod(pluginData interface{}, method reflect.Method, prototype reflect.Value) error
	AfterCheckInputStruct(pluginData interface{}, input *graphql.InputObject) error

	CheckQueryOperation(operation string, arguments reflect.Type, typ reflect.Type)
	CheckMutationOperation(operation string, arguments reflect.Type, typ reflect.Type)
}

type pluginWrapper struct {
	name   string
	plugin Plugin
}

func (engine *Engine) RegisterPlugin(name string, plugin Plugin) {
	engine.plugins = append(engine.plugins, pluginWrapper{name, plugin})
}

func (engine *Engine) callPluginsSafely(do func(name string, plugin Plugin) error, handleError func(name string, err error)) {
	if len(engine.plugins) == 0 {
		return
	}
	for _, w := range engine.plugins {
		(func(wrapper *pluginWrapper) {
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); ok {
						if handleError != nil {
							handleError(wrapper.name, err)
						}
					}
					if engine.opts.Debug {
						debug.PrintStack()
					}
				}
			}()

			if err := do(wrapper.name, wrapper.plugin); err != nil {
				if handleError != nil {
					handleError(wrapper.name, err)
				}
			}
		})(&w)
	}
}

func (engine *Engine) callPluginOnMethod(implType reflect.Type, call func(method reflect.Method, prototype reflect.Value)) {
	if len(engine.plugins) > 0 {
		if numField := implType.NumMethod(); numField > 0 {
			p := newPrototype(implType)
			pv := reflect.ValueOf(p)
			for i := 0; i < numField; i++ {
				method := implType.Method(i)
				call(method, pv)
			}
		}
	}
}
