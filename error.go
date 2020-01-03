// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"reflect"
)

type Error interface {
	error
	GraphQLErrorExtension() string
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func (engine *Engine) asErrorResult(p reflect.Type) bool {
	return p.Implements(errorType)
}
