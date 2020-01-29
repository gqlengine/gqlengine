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
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gobwas/ws"

	"github.com/karfield/graphql"
)

func (engine *Engine) doGraphqlRequest(w http.ResponseWriter, r *http.Request, opt *RequestOptions) *graphql.Result {
	ctx, err := engine.handleRequestContexts(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	result, ctx := graphql.Do(graphql.Params{
		Schema:         engine.schema,
		Context:        ctx,
		RequestString:  opt.Query,
		VariableValues: opt.Variables,
		OperationName:  opt.OperationName,
	})
	if err := engine.finalizeContexts(ctx, w); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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
