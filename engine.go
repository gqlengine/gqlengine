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
	Tracing       bool
	WsSubProtocol string
}

func NewEngine(options Options) *Engine {
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
	Description(desc string) QueryBuilder
	Tags(tags ...string) QueryBuilder
}

type _query struct {
	name    string
	resolve interface{}
	desc    string
	tags    []string
}

func (q *_query) build(engine *Engine) error {
	if q.tags == nil {
		return engine.AddQuery(q.name, q.desc, q.resolve)
	}
	return engine.AddQuery(q.name, q.desc, q.resolve, q.tags...)
}

func (q *_query) Description(desc string) QueryBuilder { q.desc = desc; return q }
func (q *_query) Tags(tags ...string) QueryBuilder     { q.tags = tags; return q }

func (engine *Engine) NewQuery(name string, resolve interface{}) QueryBuilder {
	q := &_query{name: name, resolve: resolve}
	engine.chainBuilders = append(engine.chainBuilders, q)
	return q
}

func (engine *Engine) AddQuery(name string, description string, resolve interface{}, tags ...string) error {
	if name == "" {
		return fmt.Errorf("requires an operation name")
	}
	if resolve == nil {
		return fmt.Errorf("missing resolve funtion")
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
	Description(desc string) MutationBuilder
	Tags(tags ...string) MutationBuilder
}

type _mutation struct {
	name    string
	desc    string
	resolve interface{}
	tags    []string
}

func (m *_mutation) build(engine *Engine) error {
	if m.tags == nil {
		return engine.AddMutation(m.name, m.desc, m.resolve)
	}
	return engine.AddMutation(m.name, m.desc, m.resolve, m.tags...)
}
func (m *_mutation) Description(desc string) MutationBuilder { m.desc = desc; return m }
func (m *_mutation) Tags(tags ...string) MutationBuilder     { m.tags = tags; return m }
func (engine *Engine) NewMutation(name string, resolve interface{}) MutationBuilder {
	m := &_mutation{name: name, resolve: resolve}
	engine.chainBuilders = append(engine.chainBuilders, m)
	return m
}

func (engine *Engine) AddMutation(name string, description string, resolve interface{}, tags ...string) error {
	if name == "" {
		return fmt.Errorf("requires an operation name")
	}
	if resolve == nil {
		return fmt.Errorf("missing resolve funtion")
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
	Description(desc string) PaginationQuery
	Tags(tags ...string) PaginationQuery
	TotalResolver(resolve interface{}) PaginationQuery
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
		return engine.AddPaginationQuery(p.name, p.description, p.resolveList, p.resolveTotal)
	}
	return engine.AddPaginationQuery(p.name, p.description, p.resolveList, p.resolveTotal, p.tags...)
}
func (p *_paginationQuery) Description(desc string) PaginationQuery { p.description = desc; return p }
func (p *_paginationQuery) Tags(tags ...string) PaginationQuery     { p.tags = tags; return p }
func (p *_paginationQuery) TotalResolver(resolve interface{}) PaginationQuery {
	p.resolveTotal = resolve
	return p
}

func (engine *Engine) NewPaginationQuery(name string, resolve interface{}) PaginationQuery {
	p := &_paginationQuery{name: name, resolveList: resolve}
	engine.chainBuilders = append(engine.chainBuilders, p)
	return p
}

func (engine *Engine) AddPaginationQuery(name, description string, resolveList, resolveTotal interface{}, tags ...string) error {
	if name == "" {
		return fmt.Errorf("requires an operation name")
	}
	if resolveList == nil {
		return fmt.Errorf("missing resolveList() funtion")
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
	Description(desc string) SubscriptionBuilder
	OnUnsubscribed(unsubscribed interface{}) SubscriptionBuilder
	Tags(tags ...string) SubscriptionBuilder
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
		return engine.AddSubscription(s.name, s.desc, s.onSubscribed, s.onUnsubscribed)
	}
	return engine.AddSubscription(s.name, s.desc, s.onSubscribed, s.onUnsubscribed, s.tags...)
}

func (s *_subscriptionBuilder) Description(desc string) SubscriptionBuilder { s.desc = desc; return s }
func (s *_subscriptionBuilder) Tags(tags ...string) SubscriptionBuilder     { s.tags = tags; return s }
func (s *_subscriptionBuilder) OnUnsubscribed(unsubscribed interface{}) SubscriptionBuilder {
	s.onUnsubscribed = unsubscribed
	return s
}

func (engine *Engine) NewSubscription(name string, onSubscribed interface{}) SubscriptionBuilder {
	s := &_subscriptionBuilder{name: name, onSubscribed: onSubscribed}
	engine.chainBuilders = append(engine.chainBuilders, s)
	return s
}

func (engine *Engine) AddSubscription(name string, description string, onSubscribed, onUnsubscribed interface{}, tags ...string) error {
	if name == "" {
		return fmt.Errorf("requires an operation name")
	}
	if onSubscribed == nil {
		return fmt.Errorf("missing onSubscribed() funtion")
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
