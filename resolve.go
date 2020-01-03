package gqlengine

import (
	"context"
	"fmt"
	"reflect"

	"github.com/karfield/graphql"
)

type resolverArgumentBuilder interface {
	build(params graphql.ResolveParams) (interface{}, error)
}

const (
	returnResult = iota + 1
	returnError
	returnContext
)

type resolver struct {
	fn          graphql.ResolveFieldWithContext
	args        reflect.Type
	source      reflect.Type
	out         reflect.Type
	outBaseType reflect.Type
	isBatch     bool
}

func checkResultType(expected, actually reflect.Type) bool {
	// unwrap slice
	if expected.Kind() == reflect.Slice {
		if actually.Kind() != reflect.Slice {
			return false
		}
		expected = expected.Elem()
		actually = actually.Elem()
	} else if actually.Kind() == reflect.Slice {
		return false
	}

	if expected.Kind() == reflect.Ptr {
		expected = expected.Elem()
	}
	if actually.Kind() == reflect.Ptr {
		actually = actually.Elem()
	}

	return expected == actually
}

func (engine *Engine) analysisResolver(fieldName string, resolve interface{}) (*resolver, error) {
	resolveFn := reflect.ValueOf(resolve)
	resolveFnType := resolveFn.Type()
	if resolveFnType.Kind() != reflect.Func {
		panic("resolve prototype should be a function")
	}

	resolver := resolver{}

	argumentBuilders := make([]resolverArgumentBuilder, resolveFnType.NumIn())
	returnTypes := make([]int, resolveFnType.NumOut())

	for i := 0; i < resolveFnType.NumIn(); i++ {
		in := resolveFnType.In(i)
		var builder resolverArgumentBuilder
		if argsBuilder := engine.asArguments(in); argsBuilder != nil {
			builder = argsBuilder
			if resolver.args != nil {
				return nil, fmt.Errorf("more than one 'arguments' parameter[%d]", i)
			}
			resolver.args = in
		} else if ctxBuilder := engine.asContextArgument(in); ctxBuilder != nil {
			builder = ctxBuilder
		} else if objSource, isBatch, obj := engine.asObjectSource(in); objSource != nil {
			builder = objSource
			if resolver.source == nil {
				resolver.source = obj
			} else {
				return nil, fmt.Errorf("more than one source argument[%d]: '%s'", i, in)
			}
			resolver.isBatch = isBatch
		} else {
			return nil, fmt.Errorf("unsupported argument type [%d]: '%s'", i, in)
		}
		argumentBuilders[i] = builder
	}

	var sourceField *reflect.StructField
	if resolver.source != nil {
		if fieldName == "" {
			return nil, fmt.Errorf("unexpect source argument '%s'", resolver.source)
		}
		srcStructType := resolver.source
		if srcStructType.Kind() == reflect.Ptr {
			srcStructType = srcStructType.Elem()
		}
		for i := 0; i < srcStructType.NumField(); i++ {
			f := srcStructType.Field(i)
			if f.Name == fieldName {
				sourceField = &f
			}
		}
		if sourceField != nil {
			if !needBeResolved(sourceField) {
				return nil, fmt.Errorf("the field need not be resolved")
			}
		}
	}

	for i := 0; i < resolveFnType.NumOut(); i++ {
		out := resolveFnType.Out(i)
		var (
			returnType  int
			outBaseType reflect.Type
		)
		if isContext, err := engine.asContextMerger(out); isContext {
			returnType = returnContext
		} else if err != nil {
			return nil, err
		} else if engine.asErrorResult(out) {
			returnType = returnError
		} else {
			if resolver.isBatch {
				if out.Kind() != reflect.Slice {
					return nil, fmt.Errorf("expect slice of results, but '%s' in result[%d]", out, i)
				}
				out = out.Elem() // unwrap the slice
			}
			if sourceField != nil {
				// compare out with sourceField.Type
				if !checkResultType(sourceField.Type, out) {
					return nil, fmt.Errorf("result type('%d') of resolve function is not match with field('%s') type('%s') of object",
						out, sourceField.Name, sourceField.Type)
				}
			}

			if baseType := engine.asObjectResult(out); baseType != nil {
				outBaseType = baseType
			} else if baseType := asBuiltinScalarResult(out); baseType != nil {
				outBaseType = baseType
			} else if baseType := engine.asEnumResult(out); baseType != nil {
				outBaseType = baseType
			} else if baseType := engine.asIdResult(out); baseType != nil {
				outBaseType = baseType
			} else if baseType := engine.asCustomScalarResult(out); baseType != nil {
				outBaseType = baseType
			} else {
				return nil, fmt.Errorf("unsupported resolve result[%d] '%s'", i, out)
			}

			returnType = returnResult
		}

		returnTypes[i] = returnType
		if outBaseType != nil {
			if resolver.out != nil {
				return nil, fmt.Errorf("more than one result[%d] '%s'", i, out)
			}
			resolver.out = out
			resolver.outBaseType = outBaseType
		}
	}

	resolveFnValue := reflect.ValueOf(resolve)
	resolver.fn = func(p graphql.ResolveParams) (result interface{}, ctx context.Context, err error) {
		args := make([]reflect.Value, len(argumentBuilders))
		for i, ab := range argumentBuilders {
			arg, err := ab.build(p)
			if err != nil {
				return nil, p.Context, err
			}
			args[i] = reflect.ValueOf(arg)
		}
		results := resolveFnValue.Call(args)
		ctx = p.Context
		for i, r := range results {
			switch returnTypes[i] {
			case returnResult:
				result = r.Interface()
			case returnContext:
				ctx = context.WithValue(ctx, r.Type(), r.Interface())
			case returnError:
				if !r.IsNil() {
					err = r.Interface().(error)
				}
			}
		}
		return
	}

	return &resolver, nil
}
