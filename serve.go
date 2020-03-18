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
	"encoding/json"
	"net/http"
	"sync"

	"github.com/karfield/graphql/gqlerrors"

	"github.com/gobwas/ws"

	"github.com/karfield/graphql"
)

func handleContextError(err error, w http.ResponseWriter) *graphql.Result {
	if err != nil {
		if ctxErr, ok := err.(ContextError); ok {
			w.WriteHeader(ctxErr.StatusCode())
			return &graphql.Result{
				Errors: []gqlerrors.FormattedError{{
					Message:    ctxErr.Error(),
					Extensions: ctxErr.Extensions(),
				}},
			}
		}
		if extErr, ok := err.(gqlerrors.ExtendedError); ok {
			w.WriteHeader(http.StatusBadRequest)
			return &graphql.Result{
				Errors: []gqlerrors.FormattedError{{
					Message:    extErr.Error(),
					Extensions: extErr.Extensions(),
				}},
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		return &graphql.Result{
			Errors: []gqlerrors.FormattedError{{
				Message: err.Error(),
			}},
		}
	}
	return nil
}

func (engine *Engine) doGraphqlRequest(w http.ResponseWriter, r *http.Request, opt *RequestOptions) *graphql.Result {
	ctx, err := engine.handleRequestContexts(r)
	if r := handleContextError(err, w); r != nil {
		return r
	}
	result, ctx := graphql.Do(graphql.Params{
		Schema:         engine.schema,
		Context:        ctx,
		RequestString:  opt.Query,
		VariableValues: opt.Variables,
		OperationName:  opt.OperationName,
	})
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			if r := handleContextError(err, w); r != nil {
				return r
			}
		}
	}
	if err := engine.finalizeContexts(ctx, w); err != nil {
		if r := handleContextError(err, w); r != nil {
			return r
		}
	}
	return result
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fixCors(w, r)
	opts := engine.newRequestOptions(r)
	if len(opts) == 1 {
		result := engine.doGraphqlRequest(w, r, opts[0])
		if err := json.NewEncoder(w).Encode(result); err != nil {
		}
	} else if len(opts) > 1 {
		results := make([]*graphql.Result, len(opts))
		wg := sync.WaitGroup{}
		wg.Add(len(opts))
		for i, opt := range opts {
			go func(i int, opt *RequestOptions) {
				results[i] = engine.doGraphqlRequest(w, r, opt)
				wg.Done()
			}(i, opt)
		}
		wg.Wait()
		if err := json.NewEncoder(w).Encode(results); err != nil {
		}
	} else {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{})
	}
}

func (engine *Engine) ServeWebsocket(w http.ResponseWriter, r *http.Request) {
	upgrader := ws.HTTPUpgrader{
		Protocol: func(s string) bool {
			if engine.opts.WsSubProtocol != "" {
				return s == engine.opts.WsSubProtocol
			}
			return s == "graphql-ws"
		},
	}
	//conn, _, _, err := ws.UpgradeHTTP(r, w)
	conn, _, _, err := upgrader.Upgrade(r, w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ctx, err := engine.handleRequestContexts(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	go engine.handleWs(conn, ctx)
}
