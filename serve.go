// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"encoding/json"
	"net/http"

	"github.com/gobwas/ws"

	"github.com/karfield/graphql"
)

func (engine *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fixCors(w, r)
	opts := newRequestOptions(r)
	if opts != nil {
		ctx, err := engine.handleRequestContexts(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		result, ctx := graphql.Do(graphql.Params{
			Schema:         engine.schema,
			Context:        ctx,
			RequestString:  opts.Query,
			VariableValues: opts.Variables,
			OperationName:  opts.OperationName,
		})
		if err := engine.finalizeContexts(ctx, w); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			// FIXME: do with response failure
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
