// Copyright 2020 Karfield Technology. Ltd (凯斐德科技（杭州）有限公司)
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
	initialized    bool
	opts           Options
	schema         graphql.Schema
	query          *graphql.Object
	mutation       *graphql.Object
	subscription   *graphql.Object
	types          map[reflect.Type]graphql.Type
	idTypes        map[reflect.Type]struct{}
	argConfigs     map[reflect.Type]graphql.FieldConfigArgument
	reqCtx         map[reflect.Type]reflect.Type
	respCtx        map[reflect.Type]reflect.Type
	objResolvers   map[reflect.Type]objectResolvers
	batchResolvers map[reflect.Type]objectResolvers

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
		opts:           options,
		types:          map[reflect.Type]graphql.Type{},
		idTypes:        map[reflect.Type]struct{}{},
		argConfigs:     map[reflect.Type]graphql.FieldConfigArgument{},
		reqCtx:         map[reflect.Type]reflect.Type{},
		respCtx:        map[reflect.Type]reflect.Type{},
		objResolvers:   map[reflect.Type]objectResolvers{},
		batchResolvers: map[reflect.Type]objectResolvers{},
		tags:           map[string]*tagEntries{},
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

func (engine *Engine) AddResolver(field string, resolve interface{}) error {
	if field == "" {
		return fmt.Errorf("requires the field name")
	}
	if resolve == nil {
		return fmt.Errorf("missing resolve funtion")
	}
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

type PaginationQuery interface {
	Name(name string) PaginationQuery
	Description(desc string) PaginationQuery
	Tags(tags ...string) PaginationQuery
	TotalResolver(resolve interface{}) PaginationQuery
	WrapWith(fn interface{}) PaginationQuery
}

type _paginationQuery struct {
	name         string
	description  string
	resolveList  interface{}
	resolveTotal interface{}
	tags         []string
}

func (p *_paginationQuery) build(engine *Engine) error {
	if p.tags == nil {
		return engine.AddPaginationQuery(p.resolveList, p.resolveTotal, p.name, p.description)
	}
	return engine.AddPaginationQuery(p.resolveList, p.resolveTotal, p.name, p.description, p.tags...)
}

func (p *_paginationQuery) Name(name string) PaginationQuery        { p.name = name; return p }
func (p *_paginationQuery) Description(desc string) PaginationQuery { p.description = desc; return p }
func (p *_paginationQuery) Tags(tags ...string) PaginationQuery     { p.tags = tags; return p }
func (p *_paginationQuery) TotalResolver(resolve interface{}) PaginationQuery {
	p.resolveTotal = resolve
	return p
}
func (p *_paginationQuery) WrapWith(fn interface{}) PaginationQuery {
	newResolveList, err := BeforeResolve(p.resolveList, fn)
	if err != nil {
		panic(err)
	}
	p.resolveList = newResolveList
	// fixme: wrap 'resolve total' too?
	return p
}

func (engine *Engine) NewPaginationQuery(resolve interface{}) PaginationQuery {
	p := &_paginationQuery{resolveList: resolve, name: getEntryFuncName(resolve)}
	engine.chainBuilders = append(engine.chainBuilders, p)
	return p
}

func (engine *Engine) AddPaginationQuery(resolveList, resolveTotal interface{}, name, description string, tags ...string) error {
	if resolveList == nil {
		return fmt.Errorf("missing resolveList() funtion")
	}
	if name == "" {
		name = getEntryFuncName(resolveList)
		if name == "" {
			return fmt.Errorf("requires a query field name")
		}
	}
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

	engine.addTags(tagQuery, name, tags)
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
