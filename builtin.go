// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"reflect"
	"time"

	"github.com/karfield/graphql"
	"github.com/karfield/graphql/language/ast"
)

var (
	_builtinTypes = []struct {
		v interface{}
		t graphql.Type
	}{
		{int(0), graphql.Int},
		{int8(0), graphql.Int},
		{int16(0), graphql.Int},
		{int32(0), graphql.Int},
		{int64(0), graphql.Int},
		{uint(0), graphql.Int},
		{uint8(0), graphql.Int},
		{uint16(0), graphql.Int},
		{uint32(0), graphql.Int},
		{uint64(0), graphql.Int},
		{"", graphql.String},
		{float32(0), graphql.Float},
		{float64(0), graphql.Float},
		{false, graphql.Boolean},
		{time.Time{}, graphql.DateTime},
		{time.Duration(0), Duration},
	}
	builtinTypeMap = map[reflect.Type]graphql.Type{}
)

func (engine *Engine) initBuiltinTypes() {
	for _, builtinType := range _builtinTypes {
		bt := reflect.TypeOf(builtinType.v)
		engine.types[bt] = builtinType.t
		builtinTypeMap[bt] = builtinType.t
	}
}

func asBuiltinScalar(field reflect.StructField) (scalar graphql.Type) {
	baseType, isArray, _ := unwrap(field.Type)

	if baseType.PkgPath() == "" {
		// builtin
		switch baseType.Kind() {
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Uint, reflect.Uint64, reflect.Uint32:
			scalar = graphql.Int
		case reflect.Int8, reflect.Int16, reflect.Uint8, reflect.Uint16:
			scalar = graphql.Int
		case reflect.Float32, reflect.Float64:
			scalar = graphql.Float
		case reflect.Bool:
			scalar = graphql.Boolean
		case reflect.String:
			scalar = graphql.String
		default:
		}
	} else {
		switch baseType.String() {
		case "time.Time":
			scalar = graphql.DateTime
		case "time.Duration":
			scalar = Duration
		}
	}

	if scalar == nil {
		return
	}

	if isArray {
		scalar = graphql.NewList(scalar)
	}

	if isRequired(&field) {
		scalar = graphql.NewNonNull(scalar)
	}

	return
}

func asBuiltinScalarResult(baseType reflect.Type) reflect.Type {
	baseType, _, _ = unwrap(baseType)
	if baseType.PkgPath() == "" {
		// builtin
		switch baseType.Kind() {
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Uint, reflect.Uint64, reflect.Uint32:
			return baseType
		case reflect.Int8, reflect.Int16, reflect.Uint8, reflect.Uint16:
			return baseType
		case reflect.Float32, reflect.Float64:
			return baseType
		case reflect.Bool:
			return baseType
		case reflect.String:
			return baseType
		default:
		}
	} else {
		switch baseType.String() {
		case "time.Time":
			return baseType
		case "time.Duration":
			return baseType
		}
	}
	return nil
}

var Void = graphql.NewScalar(graphql.ScalarConfig{
	Name:         "Void",
	Description:  "void",
	Serialize:    func(value interface{}) interface{} { return "0" },
	ParseValue:   func(value interface{}) interface{} { return 0 },
	ParseLiteral: func(valueAST ast.Value) interface{} { return 0 },
})

var Duration = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "Duration",
	Description: "time duration",
	Serialize: func(value interface{}) interface{} {
		switch value := value.(type) {
		case time.Duration:
			return value.String()
		case *time.Duration:
			return value.String()
		}
		return nil
	},
	ParseValue: func(value interface{}) interface{} {
		switch value := value.(type) {
		case string:
			d, _ := time.ParseDuration(value)
			return d
		case *string:
			d, _ := time.ParseDuration(*value)
			return d
		}
		return nil
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			d, _ := time.ParseDuration(valueAST.Value)
			return d
		}
		return nil
	},
})
