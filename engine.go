// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"fmt"
	"reflect"

	"github.com/karfield/graphql"
)

type Engine struct {
	initialized    bool
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
}

func NewEngine() *Engine {
	engine := &Engine{
		types:          map[reflect.Type]graphql.Type{},
		idTypes:        map[reflect.Type]struct{}{},
		argConfigs:     map[reflect.Type]graphql.FieldConfigArgument{},
		reqCtx:         map[reflect.Type]struct{}{},
		respCtx:        map[reflect.Type]struct{}{},
		objResolvers:   map[reflect.Type]objectResolvers{},
		batchResolvers: map[reflect.Type]objectResolvers{},
	}
	engine.initBuiltinTypes()
	return engine
}

func (engine *Engine) Init() (err error) {
	if engine.initialized {
		return
	}

	engine.finalizeObjectResolvers()

	engine.schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:        engine.query,
		Mutation:     engine.mutation,
		Subscription: engine.subscription,
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
		args = engine.argConfigs[resolver.args]
	}
	typ := engine.types[resolver.outBaseType]
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
		typ = engine.types[resolver.outBaseType]
		if resolver.out.Kind() == reflect.Slice {
			typ = graphql.NewList(typ)
		}
	}
	var args graphql.FieldConfigArgument
	if resolver.args != nil {
		args = engine.argConfigs[resolver.args]
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
		args = engine.argConfigs[resolver.args]
	}
	typ := engine.types[resolver.outBaseType]
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

func (engine *Engine) finalizeObjectResolvers() {
	// FIXME
}
