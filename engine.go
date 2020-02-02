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

	"github.com/karfield/graphql"
)

const (
	DefaultMultipartParsingBufferSize = 10 * 1024 * 1024
)

type Engine struct {
	initialized  bool
	opts         Options
	schema       graphql.Schema
	query        *graphql.Object
	mutation     *graphql.Object
	subscription *graphql.Object
	types        map[reflect.Type]graphql.Type
	idTypes      map[reflect.Type]struct{}
	argConfigs   map[reflect.Type]graphql.FieldConfigArgument
	reqCtx       map[reflect.Type]reflect.Type
	respCtx      map[reflect.Type]reflect.Type

	resultCheckers        []resolveResultChecker
	inputFieldCheckers    []fieldChecker
	objFieldCheckers      []fieldChecker
	authSubscriptionToken func(authToken string) (context.Context, error)

	chainBuilders []chainBuilder
	tags          map[string]*tagEntries
}

type Options struct {
	Tracing                    bool
	WsSubProtocol              string
	Tags                       bool
	MultipartParsingBufferSize int64
}

func NewEngine(options Options) *Engine {
	if options.MultipartParsingBufferSize == 0 {
		options.MultipartParsingBufferSize = DefaultMultipartParsingBufferSize
	}
	engine := &Engine{
		opts:       options,
		types:      map[reflect.Type]graphql.Type{},
		idTypes:    map[reflect.Type]struct{}{},
		argConfigs: map[reflect.Type]graphql.FieldConfigArgument{},
		reqCtx:     map[reflect.Type]reflect.Type{},
		respCtx:    map[reflect.Type]reflect.Type{},
		tags:       map[string]*tagEntries{},
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
		asUploadScalar,
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

	if len(engine.chainBuilders) > 0 {
		for _, b := range engine.chainBuilders {
			if err := b.build(engine); err != nil {
				return err
			}
		}
	}

	if engine.opts.Tags {
		engine.enableQueryTags()
	}

	var extensions []graphql.Extension
	if engine.opts.Tracing {
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

type chainBuilder interface {
	build(engine *Engine) error
}

type QueryBuilder interface {
	Name(name string) QueryBuilder
	Description(desc string) QueryBuilder
	Tags(tags ...string) QueryBuilder
	WrapWith(fn interface{}) QueryBuilder
}

type _query struct {
	name    string
	resolve interface{}
	desc    string
	tags    []string
}

func (q *_query) build(engine *Engine) error {
	if q.tags == nil {
		return engine.AddQuery(q.resolve, q.name, q.desc)
	}
	return engine.AddQuery(q.resolve, q.name, q.desc, q.tags...)
}

func (q *_query) Name(name string) QueryBuilder        { q.name = name; return q }
func (q *_query) Description(desc string) QueryBuilder { q.desc = desc; return q }
func (q *_query) Tags(tags ...string) QueryBuilder     { q.tags = tags; return q }
func (q *_query) WrapWith(fn interface{}) QueryBuilder {
	newResolveFn, err := BeforeResolve(q.resolve, fn)
	if err != nil {
		panic(err)
	}
	q.resolve = newResolveFn
	return q
}

func (engine *Engine) NewQuery(resolve interface{}) QueryBuilder {
	q := &_query{resolve: resolve, name: getEntryFuncName(resolve)}
	engine.chainBuilders = append(engine.chainBuilders, q)
	return q
}

func (engine *Engine) AddQuery(resolve interface{}, name string, description string, tags ...string) error {
	if resolve == nil {
		return fmt.Errorf("missing resolve funtion")
	}
	if name == "" {
		name = getEntryFuncName(resolve)
		if name == "" {
			return fmt.Errorf("requires a query field name")
		}
	}
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
	engine.addTags(tagQuery, name, tags)
	return nil
}

type MutationBuilder interface {
	Name(name string) MutationBuilder
	Description(desc string) MutationBuilder
	Tags(tags ...string) MutationBuilder
	WrapWith(fn interface{}) MutationBuilder
}

type _mutation struct {
	name    string
	desc    string
	resolve interface{}
	tags    []string
}

func (m *_mutation) build(engine *Engine) error {
	if m.tags == nil {
		return engine.AddMutation(m.resolve, m.name, m.desc)
	}
	return engine.AddMutation(m.resolve, m.name, m.desc, m.tags...)
}

func (m *_mutation) Name(name string) MutationBuilder        { m.name = name; return m }
func (m *_mutation) Description(desc string) MutationBuilder { m.desc = desc; return m }
func (m *_mutation) Tags(tags ...string) MutationBuilder     { m.tags = tags; return m }
func (m *_mutation) WrapWith(fn interface{}) MutationBuilder {
	newResolveFn, err := BeforeResolve(m.resolve, fn)
	if err != nil {
		panic(err)
	}
	m.resolve = newResolveFn
	return m
}

func (engine *Engine) NewMutation(resolve interface{}) MutationBuilder {
	m := &_mutation{resolve: resolve, name: getEntryFuncName(resolve)}
	engine.chainBuilders = append(engine.chainBuilders, m)
	return m
}

func (engine *Engine) AddMutation(resolve interface{}, name string, description string, tags ...string) error {
	if resolve == nil {
		return fmt.Errorf("missing resolve funtion")
	}
	if name == "" {
		name = getEntryFuncName(resolve)
		if name == "" {
			return fmt.Errorf("requires a mutation field name")
		}
	}
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

	engine.addTags(tagMutation, name, tags)
	return nil
}

type SubscriptionBuilder interface {
	Name(name string) SubscriptionBuilder
	Description(desc string) SubscriptionBuilder
	OnUnsubscribed(unsubscribed interface{}) SubscriptionBuilder
	Tags(tags ...string) SubscriptionBuilder
	WrapWith(fn interface{}) SubscriptionBuilder
}

type _subscriptionBuilder struct {
	name           string
	desc           string
	onSubscribed   interface{}
	onUnsubscribed interface{}
	tags           []string
}

func (s *_subscriptionBuilder) build(engine *Engine) error {
	if s.tags == nil {
		return engine.AddSubscription(s.onSubscribed, s.onUnsubscribed, s.name, s.desc)
	}
	return engine.AddSubscription(s.onSubscribed, s.onUnsubscribed, s.name, s.desc, s.tags...)
}

func (s *_subscriptionBuilder) Name(name string) SubscriptionBuilder        { s.name = name; return s }
func (s *_subscriptionBuilder) Description(desc string) SubscriptionBuilder { s.desc = desc; return s }
func (s *_subscriptionBuilder) Tags(tags ...string) SubscriptionBuilder     { s.tags = tags; return s }
func (s *_subscriptionBuilder) OnUnsubscribed(unsubscribed interface{}) SubscriptionBuilder {
	s.onUnsubscribed = unsubscribed
	return s
}
func (s *_subscriptionBuilder) WrapWith(fn interface{}) SubscriptionBuilder {
	newInitPrototypeFn, err := BeforeResolve(s.onSubscribed, fn)
	if err != nil {
		panic(err)
	}
	s.onSubscribed = newInitPrototypeFn
	return s
}

func (engine *Engine) NewSubscription(onSubscribed interface{}) SubscriptionBuilder {
	s := &_subscriptionBuilder{name: getEntryFuncName(onSubscribed), onSubscribed: onSubscribed}
	engine.chainBuilders = append(engine.chainBuilders, s)
	return s
}

func (engine *Engine) AddSubscription(onSubscribed, onUnsubscribed interface{}, name string, description string, tags ...string) error {
	if onSubscribed == nil {
		return fmt.Errorf("missing onSubscribed() funtion")
	}
	if name == "" {
		name = getEntryFuncName(onSubscribed)
		if name == "" {
			return fmt.Errorf("requires a subscription field name")
		}
	}
	if engine.subscription == nil {
		engine.subscription = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: graphql.Fields{},
		})
	}
	handler, err := engine.checkSubscriptionHandler(onSubscribed, onUnsubscribed)
	if err != nil {
		return err
	}
	engine.subscription.AddFieldConfig(name, &graphql.Field{
		Description: description,
		Args:        handler.args,
		Type:        handler.result,
		Resolve:     graphql.ResolveFieldWithContext(handler.resolve),
	})
	engine.addTags(tagSubscription, name, tags)
	return nil
}

func (engine *Engine) AddSubscriptionAuthentication(auth func(authToken string) (context.Context, error)) {
	engine.authSubscriptionToken = auth
}
