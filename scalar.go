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
	if info.array {
		gtype = graphql.NewList(gtype)
	}
	return gtype, &info, nil
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
