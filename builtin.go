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

func asBuiltinScalar(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	info, err := unwrap(field.Type)
	if err != nil {
		return nil, &info, err
	}

	var scalar graphql.Type
	if info.baseType.PkgPath() == "" {
		// builtin
		switch info.baseType.Kind() {
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
		switch info.baseType.String() {
		case "time.Time":
			scalar = graphql.DateTime
		case "time.Duration":
			scalar = Duration
		}
	}

	if scalar == nil {
		return nil, &info, nil
	}

	if info.array {
		scalar = graphql.NewList(scalar)
	}

	if isRequired(field) {
		scalar = graphql.NewNonNull(scalar)
	}

	return scalar, &info, nil
}

func asBuiltinScalarResult(p reflect.Type) (*unwrappedInfo, error) {
	info, err := unwrap(p)
	if err != nil {
		return &info, err
	}
	if info.baseType.PkgPath() == "" {
		// builtin
		if info.baseType.Kind() != reflect.Struct {
			return &info, nil
		}
	} else {
		switch info.baseType.String() {
		case "time.Time":
			return &info, nil
		case "time.Duration":
			return &info, nil
		}
	}
	return nil, nil
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
