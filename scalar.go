// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"reflect"

	"github.com/karfield/graphql"
	"github.com/karfield/graphql/language/ast"
)

type Scalar interface {
	GraphQLScalarSerialize(value interface{}) interface{}
	GraphQLScalarParseValue(value interface{}) interface{}
	GraphQLScalarDescription() string
}

type ScalarWithASTParsing interface {
	Scalar
	GraphQLScalarParseLiteral(valueAST ast.Value) interface{}
}

type NameAlterableScalar interface {
	Scalar
	GraphQLScalarName() string
}

var scalarType = reflect.TypeOf((*Scalar)(nil)).Elem()

func (engine *Engine) collectCustomScalar(baseType reflect.Type) graphql.Type {
	if s, ok := engine.types[baseType]; ok {
		return s
	}

	scalar := newPrototype(baseType).(Scalar)

	name := baseType.Name()
	if baseType.Kind() == reflect.Ptr {
		name = baseType.Elem().Name()
	}
	if v, ok := scalar.(NameAlterableScalar); ok {
		name = v.GraphQLScalarName()
	}

	var literalParsing graphql.ParseLiteralFn
	if astParsing, ok := scalar.(ScalarWithASTParsing); ok {
		literalParsing = astParsing.GraphQLScalarParseLiteral
	} else {
		literalParsing = func(valueAST ast.Value) interface{} {
			return scalar.GraphQLScalarParseValue(valueAST.GetValue())
		}
	}

	d := graphql.NewScalar(graphql.ScalarConfig{
		Name:         name,
		Description:  scalar.GraphQLScalarDescription(),
		Serialize:    scalar.GraphQLScalarSerialize,
		ParseValue:   scalar.GraphQLScalarParseValue,
		ParseLiteral: literalParsing,
	})
	engine.types[baseType] = d
	return d
}

func (engine *Engine) asCustomScalarField(field reflect.StructField) graphql.Type {
	isScalar, isArray, baseType := implementsOf(field.Type, scalarType)
	if !isScalar {
		return nil
	}
	gtype := engine.collectCustomScalar(baseType)
	if isArray {
		gtype = graphql.NewList(gtype)
	}
	return gtype
}

func (engine *Engine) asCustomScalarResult(out reflect.Type) reflect.Type {
	isScalar, _, baseType := implementsOf(out, scalarType)
	if isScalar {
		engine.collectCustomScalar(baseType)
		return baseType
	}
	return nil
}
