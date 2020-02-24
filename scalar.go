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
	"unicode"

	"github.com/karfield/graphql"
	"github.com/karfield/graphql/language/ast"
)

type Scalar interface {
	GraphQLScalarSerialize() interface{}
	GraphQLScalarParseValue(value interface{})
	GraphQLScalarDescription() string
}

type ScalarWithASTParsing interface {
	Scalar
	GraphQLScalarParseLiteral(valueAST ast.Value)
}

type NameAlterableScalar interface {
	Scalar
	GraphQLScalarName() string
}

var scalarType = reflect.TypeOf((*Scalar)(nil)).Elem()

func (engine *Engine) collectCustomScalar(info *unwrappedInfo) (*graphql.Scalar, error) {
	if s, ok := engine.types[info.baseType]; ok {
		return s.(*graphql.Scalar), nil
	}
	if info.implType.Kind() == reflect.Ptr {
		pImpl := info.implType.Elem()
		if pImpl.Kind() == reflect.Struct {
			for i := 0; i < pImpl.NumField(); i++ {
				if !unicode.IsUpper(rune(pImpl.Field(i).Name[0])) {
					return nil, fmt.Errorf("struct-based scalar contains unexported field, may not be serialized")
				}
			}
		}
	}

	scalar := newPrototype(info.implType).(Scalar)

	name := info.baseType.Name()
	if v, ok := scalar.(NameAlterableScalar); ok {
		name = v.GraphQLScalarName()
	}

	var literalParsing graphql.ParseLiteralFn
	if _, ok := scalar.(ScalarWithASTParsing); ok {
		literalParsing = func(valueAST ast.Value) interface{} {
			s := newPrototype(info.implType).(ScalarWithASTParsing)
			s.GraphQLScalarParseLiteral(valueAST)
			return s
		}
	} else {
		literalParsing = func(valueAST ast.Value) interface{} {
			s := newPrototype(info.implType).(Scalar)
			s.GraphQLScalarParseValue(valueAST.GetValue())
			return s
		}
	}

	d := graphql.NewScalar(graphql.ScalarConfig{
		Name:        name,
		Description: scalar.GraphQLScalarDescription(),
		Serialize: func(value interface{}) interface{} {
			if s, ok := value.(Scalar); ok {
				return s.GraphQLScalarSerialize()
			} else {
				rv := reflect.ValueOf(value)
				if info.implType.Kind() == reflect.Ptr {
					if rv.Kind() != reflect.Ptr {
						v := reflect.New(rv.Type())
						switch rv.Kind() {
						case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
							v.Elem().SetInt(rv.Int())
						case reflect.String:
							v.Elem().SetString(rv.String())
						case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
							v.Elem().SetUint(rv.Uint())
						case reflect.Bool:
							v.Elem().SetBool(rv.Bool())
						case reflect.Float32, reflect.Float64:
							v.Elem().SetFloat(rv.Float())
						case reflect.Complex64, reflect.Complex128:
							v.Elem().SetComplex(rv.Complex())
						case reflect.Struct:
							s := v.Elem()
							for i := 0; i < rv.NumField(); i++ {
								if s.Field(i).CanSet() {
									s.Field(i).Set(rv.Field(i))
								} else {
									// fixme: do panic
								}
							}
						case reflect.Map:
							for _, k := range rv.MapKeys() {
								v.SetMapIndex(k, rv.MapIndex(k))
							}
						case reflect.Slice:
							v.SetLen(rv.Len())
							for i := 0; i < rv.Len(); i++ {
								v.Index(i).Set(rv.Index(i))
							}
						default:
							return nil
						}
						if s, ok := v.Interface().(Scalar); ok {
							return s.GraphQLScalarSerialize()
						}
					}
				} else {
					if rv.Kind() == reflect.Ptr {
						if s, ok := rv.Elem().Interface().(Scalar); ok {
							return s.GraphQLScalarSerialize()
						}
					}
				}
			}
			return nil
		},
		ParseValue: func(value interface{}) interface{} {
			s := newPrototype(info.implType).(Scalar)
			s.GraphQLScalarParseValue(value)
			return s
		},
		ParseLiteral: literalParsing,
	})
	engine.types[info.baseType] = d
	return d, nil
}

func (engine *Engine) asCustomScalarField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isScalar, info, err := implementsOf(field.Type, scalarType)
	if err != nil {
		return nil, &info, err
	}
	if !isScalar {
		return nil, &info, nil
	}
	gtype, err := engine.collectCustomScalar(&info)
	if err != nil {
		return nil, &info, err
	}
	return wrapType(field, gtype, info.array), &info, nil
}

func (engine *Engine) asCustomScalarResult(out reflect.Type) (*unwrappedInfo, error) {
	isScalar, info, err := implementsOf(out, scalarType)
	if err != nil {
		return nil, err
	}
	if !isScalar {
		return nil, nil
	}
	engine.collectCustomScalar(&info)
	return &info, nil
}

func (engine *Engine) registerScalar(typ reflect.Type) (*graphql.Scalar, error) {
	isScalar, info, err := implementsOf(typ, scalarType)
	if err != nil {
		return nil, err
	}
	if !isScalar {
		return nil, nil
	}
	return engine.collectCustomScalar(&info)
}

func (engine *Engine) RegisterScalar(prototype interface{}) (*graphql.Scalar, error) {
	return engine.registerScalar(reflect.TypeOf(prototype))
}
