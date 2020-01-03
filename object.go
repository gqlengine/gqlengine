// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
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

var objectType = reflect.TypeOf((*Object)(nil)).Elem()

type objectSourceBuilder struct {
}

func (o *objectSourceBuilder) build(params graphql.ResolveParams) (interface{}, error) {
	return params.Source, nil
}

type objectResolvers map[string]graphql.FieldResolveFn

func (engine *Engine) objectFields(structType reflect.Type, config *objectFieldLazyConfig) {
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)

		if f.Anonymous {
			// embedded
			embeddedType, isArray, _ := unwrap(f.Type)
			if isArray {
				panic("embedded object type should be struct")
			}
			engine.objectFields(embeddedType, config)
			continue
		}

		var fieldType graphql.Type
		if scalar := asBuiltinScalar(f); scalar != nil {
			fieldType = scalar
		} else if id := engine.asIdField(f); id != nil {
			fieldType = id
		} else if enum := engine.asEnumField(f); enum != nil {
			fieldType = enum
		} else if scalar := engine.asCustomScalarField(f); scalar != nil {
			fieldType = scalar
		} else {
			objType, _, _ := unwrap(f.Type)
			if objType == structType {
				panic("same field type and object type in loop")
			}
			if obj := engine.asObjectField(f); obj != nil {
				fieldType = obj
			}
		}

		if fieldType == nil {
			panic(fmt.Errorf("unsupported field type: %s", f.Type.String()))
		}

		config.fields[fieldName(&f)] = objectField{
			typ:            fieldType,
			desc:           desc(&f),
			needBeResolved: needBeResolved(&f),
			deprecated:     deprecatedReason(&f),
		}
	}
}

type objectField struct {
	name           string
	typ            graphql.Type
	desc           string
	needBeResolved bool
	deprecated     string
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
			if config.needBeResolved {
				if resolvers, ok := engine.objResolvers[obj]; ok {
					f.Resolve = resolvers[name]
				}
			}
			fields[name] = f
		}
		return fields
	}
}

func (engine *Engine) collectObject(baseType reflect.Type) graphql.Type {
	if obj, ok := engine.types[baseType]; ok {
		return obj
	}
	structType := baseType
	if baseType.Kind() == reflect.Ptr {
		structType = baseType.Elem()
	}

	prototypeVal := reflect.New(structType)
	if baseType.Kind() != reflect.Ptr {
		prototypeVal = prototypeVal.Elem()
	}
	prototype := prototypeVal.Interface().(Object)

	name := structType.Name()
	if rename, ok := prototype.(NameAlterableObject); ok {
		name = rename.GraphQLObjectName()
	}

	if delegated, ok := prototype.(ObjectDelegation); ok {
		objPrototype := delegated.GraphQLObjectDelegation()
		d, isArray, _ := unwrap(reflect.TypeOf(objPrototype))
		if isArray {
			panic("delegated prototype should not be non-struct")
		}
		structType = d
	}

	fieldsConfig := objectFieldLazyConfig{
		fields: map[string]objectField{},
	}
	engine.objectFields(structType, &fieldsConfig)

	var intfs graphql.Interfaces
	if impl, ok := prototype.(ImplementedObject); ok {
		for _, intfPrototype := range impl.GraphQLObjectInterfaces() {
			intfs = append(intfs, engine.asInterfaceFromPrototype(intfPrototype))
		}
	}

	preassigned := graphql.NewObject(graphql.ObjectConfig{
		Name:        name,
		Description: prototype.GraphQLObjectDescription(),
		Fields:      fieldsConfig.getFields(baseType, engine),
		Interfaces:  intfs,
	})
	engine.types[baseType] = preassigned

	return preassigned
}

func (engine *Engine) asObjectSource(p reflect.Type) (resolverArgumentBuilder, bool, reflect.Type) {
	isObj, isArray, baseType := implementsOf(p, objectType)
	if !isObj {
		return nil, false, nil
	}
	engine.collectObject(baseType)
	return &objectSourceBuilder{}, isArray, baseType
}

func (engine *Engine) asObjectResult(p reflect.Type) reflect.Type {
	isObj, _, baseType := implementsOf(p, objectType)
	if !isObj {
		return baseType
	}
	engine.collectObject(baseType)
	return baseType
}

func (engine *Engine) asObjectField(field reflect.StructField) graphql.Type {
	isObj, isArray, baseType := implementsOf(field.Type, objectType)
	if !isObj {
		return nil
	}
	typ := engine.collectObject(baseType)
	if isArray {
		typ = graphql.NewList(typ)
	}
	return typ
}
