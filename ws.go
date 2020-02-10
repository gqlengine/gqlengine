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
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/karfield/graphql"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

const (
	// Constants for operation message types
	gqlConnectionInit      = "connection_init"
	gqlConnectionAck       = "connection_ack"
	gqlConnectionKeepAlive = "ka"
	gqlConnectionError     = "connection_error"
	gqlConnectionTerminate = "connection_terminate"
	gqlStart               = "start"
	gqlData                = "data"
	gqlError               = "error"
	gqlComplete            = "complete"
	gqlStop                = "stop"

	// Timeout for outgoing messages
	writeTimeout = 10 * time.Second
)

// wsMessage represents a GraphQL WebSocket message.
type wsMessage struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type wsCtxKey struct{}
type wsDataKey struct{}

type nilData struct{}

type subscriptionFeedback struct {
	engine         *Engine
	id             string
	mu             sync.Mutex
	encoder        *json.Encoder
	finalize       func()
	result         *unwrappedInfo
	w              *wsutil.Writer
	originalCtx    context.Context
	requestString  string
	operationName  string
	variableValues map[string]interface{}
}

func (s *subscriptionFeedback) Available() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encoder != nil
}

func (s *subscriptionFeedback) close() {
	s.mu.Lock()
	if s.finalize != nil {
		s.finalize()
	}
	s.finalize = nil
	s.encoder = nil
	s.mu.Unlock()
}

type subSetupCtxKey struct{}
type subInitResult struct {
	err       error
	hasResult bool
	finalize  func()
}

func (s *subscriptionFeedback) send(data interface{}) error {
	if data == nil {
		data = nilData{}
	}
	result, _ := graphql.Do(graphql.Params{
		Context:        context.WithValue(s.originalCtx, wsDataKey{}, data),
		Schema:         s.engine.schema,
		RequestString:  s.requestString,
		OperationName:  s.operationName,
		VariableValues: s.variableValues,
	})
	jsonData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	msg := wsMessage{
		ID:      s.id,
		Type:    gqlData,
		Payload: jsonData,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.encoder != nil {
		if err := s.encoder.Encode(msg); err != nil {
			return err
		}
		return s.w.Flush()
	}
	return fmt.Errorf("ws channel(#%s) closed", s.id)
}

func (engine *Engine) handleWs(conn net.Conn, ctx context.Context) {
	mu := sync.Mutex{}
	sessions := map[string]*subscriptionFeedback{}

	defer func() {
		mu.Lock()
		for _, s := range sessions {
			s.close()
		}
		mu.Unlock()
		_ = conn.Close()
	}()

loop:
	for {
		r := wsutil.NewReader(conn, ws.StateServerSide)
		decoder := json.NewDecoder(r)

		hdr, err := r.NextFrame()
		if err != nil {
			return
		}
		if hdr.OpCode == ws.OpClose {
			return
		}

		op := wsMessage{}
		if err := decoder.Decode(&op); err != nil {
			return
		}

		w := wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText)
		encoder := json.NewEncoder(w)

		message := func(typ string, payload interface{}) error {
			data, _ := json.Marshal(payload)
			err := encoder.Encode(wsMessage{
				ID:      op.ID,
				Type:    typ,
				Payload: data,
			})
			if err != nil {
				return err
			}
			return w.Flush()
		}

		switch op.Type {
		case gqlConnectionInit:
			auth := struct {
				AuthToken string `json:"authToken"`
			}{}
			err = json.Unmarshal(op.Payload, &auth)
			if err != nil {
				message(gqlConnectionError, err.Error())
				return
			}

			if engine.authSubscriptionToken != nil {
				ctx, err = engine.authSubscriptionToken(auth.AuthToken)
				if err != nil {
					message(gqlConnectionError, err.Error())
					return
				}
			}

			_ = message(gqlConnectionAck, nil)
			_ = message(gqlConnectionKeepAlive, nil)

		case gqlConnectionTerminate:
			return

		case gqlStart:
			payload := struct {
				Query         string                 `json:"query"`
				Variables     map[string]interface{} `json:"variables"`
				OperationName string                 `json:"operationName"`
			}{}
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				message(gqlError, err.Error())
				continue
			}

			fb := &subscriptionFeedback{
				engine:         engine,
				id:             op.ID,
				encoder:        encoder,
				w:              w,
				originalCtx:    ctx,
				requestString:  payload.Query,
				operationName:  payload.OperationName,
				variableValues: payload.Variables,
			}
			ctx = context.WithValue(ctx, wsCtxKey{}, fb)

			result, ctx := graphql.Do(graphql.Params{
				Schema:         engine.schema,
				Context:        ctx,
				RequestString:  payload.Query,
				OperationName:  payload.OperationName,
				VariableValues: payload.Variables,
			})

			hasResult := false
			if subCtx := ctx.Value(subSetupCtxKey{}); subCtx != nil {
				// do nothing
				r := subCtx.(*subInitResult)
				if r.err != nil {
					_ = message(gqlError, r.err.Error())
				} else {
					fb.w = w
					fb.finalize = r.finalize
					mu.Lock()
					sessions[op.ID] = fb
					mu.Unlock()
					hasResult = r.hasResult
				}
			}

			if hasResult {
				_ = encoder.Encode(result)
				_ = w.Flush()
			}

		case gqlStop:
			payload := struct {
				ID string `json:"id"`
			}{}
			if op.Payload != nil {
				if err := json.Unmarshal(op.Payload, &payload); err != nil {
					_ = message(gqlError, err.Error())
				}
			} else {
				//_ = message(gqlError, "missing payload")
			}

			if payload.ID != "" {
				mu.Lock()
				if s, ok := sessions[payload.ID]; ok {
					s.close()
					delete(sessions, payload.ID)
				}
				mu.Unlock()

				// tell client no more messages from this ID
				_ = message(gqlComplete, fmt.Sprintf(`{"id": "%s"}`, payload.ID))
			} else {
				break loop
			}

		default:

		}
	}

	for _, s := range sessions {
		var finalize func()
		s.mu.Lock()
		if s.finalize != nil {
			finalize = s.finalize
			s.finalize = nil
		}
		finalize()
	}
}
