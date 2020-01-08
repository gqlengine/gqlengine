// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/karfield/graphql"
	"github.com/valyala/fasthttp"
)

type RequestContext interface {
	GraphQLContextFromHTTPRequest(r *http.Request) error
}

type FastRequestContext interface {
	RequestContext
	GraphQLContextFromFastHTTPRequest(ctx *fasthttp.RequestCtx) error
}

type ResponseContext interface {
	GraphQLContextToHTTPResponse(w http.ResponseWriter) error
}

type FastResponseContext interface {
	ResponseContext
	GraphQLContextToFastHTTPResponse(ctx *fasthttp.RequestCtx) error
}

var (
	_requestContextType  = reflect.TypeOf((*RequestContext)(nil)).Elem()
	_responseContextType = reflect.TypeOf((*ResponseContext)(nil)).Elem()
)

type contextBuilder struct {
	unwrappedInfo
}

func (c *contextBuilder) build(params graphql.ResolveParams) (reflect.Value, error) {
	ctxVal := params.Context.Value(c.ptrType)
	if ctxVal == nil {
		ctxVal = params.Context.Value(c.baseType)
	}
	return reflect.ValueOf(ctxVal), nil
}

func (engine *Engine) asContextArgument(p reflect.Type) (*contextBuilder, error) {
	isCtx, info, err := implementsOf(p, _requestContextType)
	if err != nil {
		return nil, err
	}
	if !isCtx {
		return nil, nil
	}
	if info.array {
		return nil, fmt.Errorf("context object('%s') should not be a slice/array", p.String())
	}

	if _, ok := engine.reqCtx[info.baseType]; !ok {
		engine.reqCtx[info.baseType] = info.implType
	}

	return &contextBuilder{
		unwrappedInfo: info,
	}, nil
}

func (engine *Engine) asContextMerger(p reflect.Type) (bool, error) {
	isCtx, info, err := implementsOf(p, _responseContextType)
	if err != nil {
		return false, err
	}
	if !isCtx {
		return false, nil
	}
	if info.array {
		return false, fmt.Errorf("response context result('%s') should not be a slice", p.String())
	}

	if _, ok := engine.respCtx[info.baseType]; !ok {
		engine.respCtx[info.baseType] = info.implType
	}

	return true, nil
}

func (engine *Engine) handleRequestContexts(r *http.Request) (context.Context, error) {
	ctx := r.Context()
	var errs []error
	for reqCtxType, reqCtxImplType := range engine.reqCtx {
		req := newPrototype(reqCtxImplType).(RequestContext)
		err := req.GraphQLContextFromHTTPRequest(r)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ctx = context.WithValue(ctx, reqCtxType, req)
	}
	if len(errs) > 0 {
		s := "multiple request context handling errors: "
		for i, err := range errs {
			if i > 0 {
				s += ";"
			}
			s += err.Error()
		}
		return nil, errors.New(s)
	}
	return ctx, nil
}

func (engine *Engine) handleFastHttpRequestContexts(r *fasthttp.RequestCtx) (context.Context, error) {
	var ctx context.Context = r
	var errs []error
	for reqCtxType, reqCtxImplType := range engine.reqCtx {
		req := newPrototype(reqCtxImplType).(FastRequestContext)
		err := req.GraphQLContextFromFastHTTPRequest(r)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ctx = context.WithValue(ctx, reqCtxType, req)
	}
	if len(errs) > 0 {
		s := "multiple request context handling errors: "
		for i, err := range errs {
			if i > 0 {
				s += ";"
			}
			s += err.Error()
		}
		return nil, errors.New(s)
	}
	return ctx, nil
}

func (engine *Engine) finalizeContexts(ctx context.Context, w http.ResponseWriter) error {
	var errs []error
	for ctxType := range engine.respCtx {
		val := ctx.Value(ctxType)
		if val != nil {
			respCtx := val.(ResponseContext)
			err := respCtx.GraphQLContextToHTTPResponse(w)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		s := "finalize contexts errors: "
		for i, err := range errs {
			if i > 0 {
				s += ";"
			}
			s += err.Error()
		}
		return errors.New(s)
	}
	return nil
}

func (engine *Engine) finalizeContextsWithFastHTTP(ctx context.Context, r *fasthttp.RequestCtx) error {
	var errs []error
	for ctxType := range engine.respCtx {
		val := ctx.Value(ctxType)
		if val != nil {
			respCtx := val.(FastResponseContext)
			err := respCtx.GraphQLContextToFastHTTPResponse(r)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		s := "finalize contexts errors: "
		for i, err := range errs {
			if i > 0 {
				s += ";"
			}
			s += err.Error()
		}
		return errors.New(s)
	}
	return nil
}
