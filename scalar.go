// Copyright 2020 Karfield Technology. Ltd (凯斐德科技（杭州）有限公司)
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
	"reflect"

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

func (engine *Engine) collectCustomScalar(info *unwrappedInfo) graphql.Type {
	if s, ok := engine.types[info.baseType]; ok {
		return s
	}

	scalar := newPrototype(info.baseType).(Scalar)

	name := info.baseType.Name()
	if v, ok := scalar.(NameAlterableScalar); ok {
		name = v.GraphQLScalarName()
	}

	var literalParsing graphql.ParseLiteralFn
	if _, ok := scalar.(ScalarWithASTParsing); ok {
		literalParsing = func(valueAST ast.Value) interface{} {
			s := newPrototype(info.baseType).(ScalarWithASTParsing)
			s.GraphQLScalarParseLiteral(valueAST)
			return s
		}
	} else {
		literalParsing = func(valueAST ast.Value) interface{} {
			s := newPrototype(info.baseType).(Scalar)
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
			}
			return nil
		},
		ParseValue: func(value interface{}) interface{} {
			s := newPrototype(info.baseType).(Scalar)
			s.GraphQLScalarParseValue(value)
			return s
		},
		ParseLiteral: literalParsing,
	})
	engine.types[info.baseType] = d
	return d
}

func (engine *Engine) asCustomScalarField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isScalar, info, err := implementsOf(field.Type, scalarType)
	if err != nil {
		return nil, &info, err
	}
	if !isScalar {
		return nil, &info, nil
	}
	gtype := engine.collectCustomScalar(&info)
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
