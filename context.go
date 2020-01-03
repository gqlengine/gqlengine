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
	typ reflect.Type
}

func (c *contextBuilder) build(params graphql.ResolveParams) (interface{}, error) {
	ctxVal := params.Context.Value(c.typ)
	return ctxVal, nil
}

func (engine *Engine) asContextArgument(p reflect.Type) *contextBuilder {
	isCtx, isArray, baseType := implementsOf(p, _requestContextType)
	if !isCtx {
		return nil
	}
	if isArray {
		panic("request context argument should not be a slice")
	}

	if _, ok := engine.reqCtx[baseType]; !ok {
		engine.reqCtx[baseType] = struct{}{}
	}

	return &contextBuilder{
		typ: baseType,
	}
}

func (engine *Engine) asContextMerger(p reflect.Type) (bool, error) {
	isCtx, isArray, baseType := implementsOf(p, _responseContextType)
	if !isCtx {
		return false, nil
	}
	if isArray {
		return false, fmt.Errorf("response context result should not be a slice")
	}

	if _, ok := engine.respCtx[baseType]; !ok {
		engine.respCtx[baseType] = struct{}{}
	}

	return true, nil
}

func (engine *Engine) handleRequestContexts(r *http.Request) (context.Context, error) {
	ctx := r.Context()
	var errs []error
	for reqCtxType := range engine.reqCtx {
		reqCtx := reflect.New(reqCtxType)
		req := reqCtx.Interface().(RequestContext)
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
	for reqCtxType := range engine.reqCtx {
		reqCtx := reflect.New(reqCtxType)
		req := reqCtx.Interface().(FastRequestContext)
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
