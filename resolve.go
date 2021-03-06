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
	argsConfig     graphql.FieldConfigArgument
	argBuilders    []resolverArgumentBuilder
	out            reflect.Type
	outInfo        *unwrappedInfo
	resultBuilders []resolverResultBuilder
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

func (engine *Engine) analysisResolver(resolve interface{}, opName string, query bool) (*resolver, error) {
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
		if argsBuilder, fieldArgsConfig, info, err := engine.asArguments(in); err != nil || argsBuilder != nil {
			if err != nil {
				return nil, err
			}
			builder = argsBuilder
			if resolver.args != nil {
				return nil, fmt.Errorf("more than one 'arguments' parameter[%d]", i)
			}
			resolver.args = in
			resolver.argsInfo = info
			resolver.argsConfig = fieldArgsConfig
		} else if ctxBuilder, err := engine.asContextArgument(in); err != nil || ctxBuilder != nil {
			if err != nil {
				return nil, err
			}
			builder = ctxBuilder
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
				if engine.opts.Debug {
					debug.PrintStack()
				}
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

	engine.callPluginsSafely(func(name string, plugin Plugin) error {
		var (
			argBaseType reflect.Type
			outBaseType reflect.Type
		)
		if resolver.argsInfo != nil {
			argBaseType = resolver.argsInfo.baseType
		}
		if resolver.outInfo != nil {
			outBaseType = resolver.outInfo.baseType
		}
		if query {
			plugin.CheckQueryOperation(opName, argBaseType, outBaseType)
		} else {
			plugin.CheckMutationOperation(opName, argBaseType, outBaseType)
		}
		return nil
	}, nil)

	return &resolver, nil
}
