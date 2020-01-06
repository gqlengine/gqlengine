// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"context"
	"fmt"
	"reflect"

	"github.com/karfield/graphql"
)

type Engine struct {
	initialized    bool
	enableTracing  bool
	schema         graphql.Schema
	query          *graphql.Object
	mutation       *graphql.Object
	subscription   *graphql.Object
	types          map[reflect.Type]graphql.Type
	idTypes        map[reflect.Type]struct{}
	argConfigs     map[reflect.Type]graphql.FieldConfigArgument
	reqCtx         map[reflect.Type]struct{}
	respCtx        map[reflect.Type]struct{}
	objResolvers   map[reflect.Type]objectResolvers
	batchResolvers map[reflect.Type]objectResolvers

	resultCheckers     []resolveResultChecker
	inputFieldCheckers []fieldChecker
	objFieldCheckers   []fieldChecker
}

type Options struct {
	Tracing bool
}

func NewEngine(options Options) *Engine {
	engine := &Engine{
		enableTracing:  options.Tracing,
		types:          map[reflect.Type]graphql.Type{},
		idTypes:        map[reflect.Type]struct{}{},
		argConfigs:     map[reflect.Type]graphql.FieldConfigArgument{},
		reqCtx:         map[reflect.Type]struct{}{},
		respCtx:        map[reflect.Type]struct{}{},
		objResolvers:   map[reflect.Type]objectResolvers{},
		batchResolvers: map[reflect.Type]objectResolvers{},
	}
	engine.resultCheckers = []resolveResultChecker{
		asBuiltinScalarResult,
		engine.asObjectResult,
		engine.asIdResult,
		engine.asEnumResult,
		engine.asCustomScalarResult,
	}
	engine.inputFieldCheckers = []fieldChecker{
		asBuiltinScalar,
		engine.asIdField,
		engine.asEnumField,
		engine.asCustomScalarField,
		engine.asInputField,
	}
	engine.objFieldCheckers = []fieldChecker{
		asBuiltinScalar,
		engine.asIdField,
		engine.asEnumField,
		engine.asObjectField,
		//engine.asInterfaceField, fixme: add support for interface field
		engine.asCustomScalarField,
	}
	engine.initBuiltinTypes()
	return engine
}

func (engine *Engine) Init() (err error) {
	if engine.initialized {
		return
	}

	engine.finalizeObjectResolvers()

	var extensions []graphql.Extension
	if engine.enableTracing {
		extensions = append(extensions, &tracingExtension{})
	}

	engine.schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:        engine.query,
		Mutation:     engine.mutation,
		Subscription: engine.subscription,
		Extensions:   extensions,
	})
	return
}

func (engine *Engine) Schema() graphql.Schema {
	if !engine.initialized {
		panic("engine not initialized yet!")
	}
	return engine.schema
}

func (engine *Engine) AddQuery(name string, description string, resolve interface{}) error {
	if engine.query == nil {
		engine.query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: graphql.Fields{},
		})
	}
	resolver, err := engine.analysisResolver("", resolve)
	if err != nil {
		return err
	}
	if resolver.out == nil {
		return fmt.Errorf("missing result of resolver")
	}
	var args graphql.FieldConfigArgument
	if resolver.args != nil {
		args = engine.argConfigs[resolver.argsInfo.baseType]
	}
	typ := engine.types[resolver.outInfo.baseType]
	if resolver.out.Kind() == reflect.Slice {
		typ = graphql.NewList(typ)
	}
	engine.query.AddFieldConfig(name, &graphql.Field{
		Description: description,
		Args:        args,
		Type:        typ,
		Resolve:     resolver.fn,
	})
	return nil
}

func (engine *Engine) AddMutation(name string, description string, resolve interface{}) error {
	if engine.mutation == nil {
		engine.mutation = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: graphql.Fields{},
		})
	}
	resolver, err := engine.analysisResolver("", resolve)
	if err != nil {
		return err
	}
	var typ graphql.Type = Void
	if resolver.out != nil {
		typ = engine.types[resolver.outInfo.baseType]
		if resolver.out.Kind() == reflect.Slice {
			typ = graphql.NewList(typ)
		}
	}
	var args graphql.FieldConfigArgument
	if resolver.args != nil {
		args = engine.argConfigs[resolver.argsInfo.baseType]
	}
	engine.mutation.AddFieldConfig(name, &graphql.Field{
		Description: description,
		Args:        args,
		Type:        typ,
		Resolve:     resolver.fn,
	})
	return nil
}

func (engine *Engine) AddSubscription(name string, description string, resolve interface{}) error {
	if engine.subscription == nil {
		engine.subscription = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: graphql.Fields{},
		})
	}
	resolver, err := engine.analysisResolver("", resolve)
	if err != nil {
		return err
	}
	if resolver.out == nil {
		return fmt.Errorf("missing result of resolver")
	}
	var args graphql.FieldConfigArgument
	if resolver.args != nil {
		args = engine.argConfigs[resolver.argsInfo.baseType]
	}
	typ := engine.types[resolver.outInfo.baseType]
	if resolver.out.Kind() == reflect.Slice {
		typ = graphql.NewList(typ)
	}
	engine.subscription.AddFieldConfig(name, &graphql.Field{
		Description: description,
		Args:        args,
		Type:        typ,
		Resolve:     resolver.fn,
	})
	return nil
}

func (engine *Engine) AddResolver(field string, resolve interface{}) error {
	resolver, err := engine.analysisResolver(field, resolve)
	if err != nil {
		return err
	}
	if resolver.isBatch {
		if resolvers, ok := engine.batchResolvers[resolver.source]; ok {
			resolvers[field] = resolver.fn
		}
	} else {
		if resolvers, ok := engine.objResolvers[resolver.source]; ok {
			resolvers[field] = resolver.fn
		}
	}
	return nil
}

func (engine *Engine) AddPaginationQuery(name, description string, resolveList, resolveTotal interface{}) error {
	listResolver, err := engine.analysisResolver("", resolveList)
	if err != nil {
		return err
	}
	totalResolver, err := engine.analysisResolver("", resolveTotal)
	if err != nil {
		return err
	}
	//if listResolver.args != totalResolver.args {
	//	return fmt.Errorf("total resolver arguments not match with list resolver")
	//}
	if listResolver.out.Kind() != reflect.Slice {
		return fmt.Errorf("list resolver should return slice of results")
	}
	if totalResolver.out.Kind() != reflect.Int {
		return fmt.Errorf("total resolver should return a int value")
	}

	argConfigs := engine.argConfigs[listResolver.args]

	if engine.query == nil {
		engine.query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: graphql.Fields{},
		})
	}

	engine.query.AddFieldConfig(name, &graphql.Field{
		Description: description,
		Args:        argConfigs,
		Type:        engine.makePaginationQueryResultObject(listResolver.outInfo.baseType),
		Resolve: graphql.ResolveFieldWithContext(func(p graphql.ResolveParams) (interface{}, context.Context, error) {
			ctx := p.Context
			args, err := listResolver.buildArgs(p)
			if err != nil {
				return nil, ctx, err
			}
			totalFnArgs := args
			if totalResolver.args != listResolver.args {
				totalFnArgs, err = totalResolver.buildArgs(p)
				if err != nil {
					return nil, ctx, err
				}
			}

			pagination := getPaginationFromParams(p)

			listResults := listResolver.fnPrototype.Call(args)
			results, ctx, err := listResolver.buildResults(ctx, listResults)
			if err != nil {
				return nil, ctx, err
			}

			totalResults := totalResolver.fnPrototype.Call(totalFnArgs)
			total, _, err := totalResolver.buildResults(ctx, totalResults)
			if err != nil {
				return nil, ctx, err
			}

			return PaginationQueryResult{
				Page:  pagination.Page,
				List:  results,
				Total: getInt(total),
			}, ctx, err
		}),
	})
	return nil
}

func (engine *Engine) finalizeObjectResolvers() {
	// FIXME
}
