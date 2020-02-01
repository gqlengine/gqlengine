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
	"context"
	"fmt"
	"reflect"
	"runtime/debug"

	"github.com/karfield/graphql/gqlerrors"

	"github.com/karfield/graphql"
)

type resolverArgumentBuilder interface {
	build(params graphql.ResolveParams) (reflect.Value, error)
}

type resolverResultBuilder interface {
	isResultBuilder()
}

type (
	returnedResultBuilder int
	errorResultBuilder    int
)

func (r returnedResultBuilder) isResultBuilder() {}
func (r errorResultBuilder) isResultBuilder()    {}

type resolver struct {
	fn             graphql.ResolveFieldWithContext
	fnPrototype    reflect.Value
	args           reflect.Type
	argsInfo       *unwrappedInfo
	argConfig      graphql.FieldConfigArgument
	argBuilders    []resolverArgumentBuilder
	source         reflect.Type
	sourceInfo     *unwrappedInfo
	out            reflect.Type
	outInfo        *unwrappedInfo
	resultBuilders []resolverResultBuilder
	isBatch        bool
}

func (r resolver) buildArgs(p graphql.ResolveParams) ([]reflect.Value, error) {
	args := make([]reflect.Value, len(r.argBuilders))
	for i, ab := range r.argBuilders {
		arg, err := ab.build(p)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}
	return args, nil
}

func (r resolver) buildResults(ctx context.Context, outs []reflect.Value) (interface{}, context.Context, error) {
	var (
		result interface{}
		err    error
	)

	for i, res := range outs {
		switch b := r.resultBuilders[i].(type) {
		case returnedResultBuilder:
			result = res.Interface()
		case *contextResultBuilder:
			ctx = context.WithValue(ctx, b.info.baseType, res.Interface())
		case errorResultBuilder:
			if !res.IsNil() {
				err = res.Interface().(error)
			}
		}
	}
	return result, ctx, err
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

type (
	resolveResultChecker func(p reflect.Type) (*unwrappedInfo, error)
)

func (engine *Engine) analysisResolver(fieldName string, resolve interface{}) (*resolver, error) {
	resolveFn := reflect.ValueOf(resolve)
	resolveFnType := resolveFn.Type()
	if resolveFnType.Kind() != reflect.Func {
		panic("resolve prototype should be a function")
	}

	resolver := resolver{}

	argumentBuilders := make([]resolverArgumentBuilder, resolveFnType.NumIn())
	returnTypes := make([]resolverResultBuilder, resolveFnType.NumOut())

	for i := 0; i < resolveFnType.NumIn(); i++ {
		in := resolveFnType.In(i)
		var builder resolverArgumentBuilder
		if argsBuilder, info, err := engine.asArguments(in); err != nil || argsBuilder != nil {
			if err != nil {
				return nil, err
			}
			builder = argsBuilder
			if resolver.args != nil {
				return nil, fmt.Errorf("more than one 'arguments' parameter[%d]", i)
			}
			resolver.args = in
			resolver.argsInfo = info
		} else if ctxBuilder, err := engine.asContextArgument(in); err != nil || ctxBuilder != nil {
			if err != nil {
				return nil, err
			}
			builder = ctxBuilder
		} else if objSource, info, err := engine.asObjectSource(in); err != nil || objSource != nil {
			if err != nil {
				return nil, err
			}
			builder = objSource
			if resolver.source == nil {
				resolver.source = in
			} else {
				return nil, fmt.Errorf("more than one source argument[%d]: '%s'", i, in)
			}
			resolver.isBatch = info.array
			resolver.sourceInfo = info
		} else if selBuilder, err := engine.asFieldSelection(in); err != nil || selBuilder != nil {
			if err != nil {
				return nil, err
			}
			builder = selBuilder
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
			returnType resolverResultBuilder
			outInfo    *unwrappedInfo
		)
		if isContext, info, err := engine.asContextMerger(out); isContext {
			returnType = &contextResultBuilder{info: info}
		} else if err != nil {
			return nil, err
		} else if engine.asErrorResult(out) {
			returnType = errorResultBuilder(0)
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

			for _, check := range engine.resultCheckers {
				if info, err := check(out); err != nil {
					return nil, err
				} else if info != nil {
					outInfo = info
					break
				}
			}

			if outInfo == nil {
				return nil, fmt.Errorf("unsupported resolve result[%d] '%s'", i, out)
			}

			returnType = returnedResultBuilder(0)
		}

		returnTypes[i] = returnType
		if outInfo != nil {
			if resolver.out != nil {
				return nil, fmt.Errorf("more than one result[%d] '%s'", i, out)
			}
			resolver.out = out
			resolver.outInfo = outInfo
		}
	}

	resolver.argBuilders = argumentBuilders
	resolver.resultBuilders = returnTypes
	resolveFnValue := reflect.ValueOf(resolve)
	resolver.fnPrototype = resolveFnValue
	resolver.fn = func(p graphql.ResolveParams) (result interface{}, ctx context.Context, ferr error) {
		ctx = p.Context
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				if err, ok := r.(error); ok {
					ferr = err
				} else {
					ferr = gqlerrors.InternalError(fmt.Sprintf("%v", r))
				}
			}
		}()
		args, err := resolver.buildArgs(p)
		if err != nil {
			ferr = err
			return
		}
		results := resolveFnValue.Call(args)
		result, ctx, ferr = resolver.buildResults(p.Context, results)
		return
	}

	return &resolver, nil
}
