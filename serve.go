package gqlengine

import (
	"encoding/json"
	"net/http"

	"github.com/karfield/graphql"

	"github.com/valyala/fasthttp"
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

func (engine *Engine) ServeFastHTTP(ctx *fasthttp.RequestCtx) {

}
