// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"fmt"
	"reflect"

	"github.com/karfield/graphql"
)

type ID interface {
	GraphQLID()
}

var _idType = reflect.TypeOf((*ID)(nil)).Elem()

func (engine *Engine) collectIdType(baseType reflect.Type) {
	typ := baseType
	if baseType.Kind() == reflect.Ptr {
		typ = baseType.Elem()
	}
	switch typ.Kind() {
	case reflect.Uint64, reflect.Uint, reflect.Uint32,
		reflect.Int64, reflect.Int, reflect.Int32,
		reflect.String:
	default:
		panic(fmt.Errorf("%s cannot be used as an ID", typ.String()))
	}

	if _, ok := engine.idTypes[baseType]; !ok {
		engine.idTypes[baseType] = struct{}{}
	}
}

func (engine *Engine) asIdField(field reflect.StructField) graphql.Type {
	isId, isArray, baseType := implementsOf(field.Type, _idType)
	if !isId {
		return nil
	}

	engine.collectIdType(baseType)
	if isArray {
		return graphql.NewNonNull(graphql.ID)
	} else {
		return graphql.ID
	}
}

func (engine *Engine) asIdResult(out reflect.Type) reflect.Type {
	isId, _, baseType := implementsOf(out, _idType)
	if !isId {
		return nil
	}
	engine.collectIdType(baseType)
	return baseType
}
