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
