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
	_contextType         = reflect.TypeOf((*context.Context)(nil)).Elem()
)

type contextBuilder struct {
	unwrappedInfo
	ctx bool
}

func (c *contextBuilder) build(params graphql.ResolveParams) (reflect.Value, error) {
	if c.ctx {
		return reflect.ValueOf(params.Context), nil
	}
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
	originalCtx := false
	if !isCtx {
		info, err = unwrap(p)
		if err != nil {
			return nil, err
		}
		if info.implType == _contextType || info.baseType == _contextType || info.ptrType == _contextType {
			originalCtx = true
		} else {
			return nil, nil
		}
	}
	if info.array {
		return nil, fmt.Errorf("context object('%s') should not be a slice/array", p.String())
	}

	if _, ok := engine.reqCtx[info.baseType]; !ok {
		engine.reqCtx[info.baseType] = info.implType
	}

	return &contextBuilder{
		unwrappedInfo: info,
		ctx:           originalCtx,
	}, nil
}

func (engine *Engine) asContextMerger(p reflect.Type) (bool, *unwrappedInfo, error) {
	isCtx, info, err := implementsOf(p, _responseContextType)
	if err != nil {
		return false, &info, err
	}
	if !isCtx {
		return false, &info, nil
	}
	if info.array {
		return false, &info, fmt.Errorf("response context result('%s') should not be a slice", p.String())
	}

	if _, ok := engine.respCtx[info.baseType]; !ok {
		engine.respCtx[info.baseType] = info.implType
	}

	return true, &info, nil
}

func (engine *Engine) handleRequestContexts(r *http.Request) (context.Context, error) {
	ctx := context.Background() // fixme: default is keep alive, need to support maximum time for each link
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

type contextResultBuilder struct {
	info *unwrappedInfo
}

func (c *contextResultBuilder) isResultBuilder() {}

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
