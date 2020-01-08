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
type subscriptionFeedback struct {
	id       string
	mu       sync.Mutex
	encoder  *json.Encoder
	finalize func()
	result   *unwrappedInfo
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
	err      error
	finalize func()
}

func (s *subscriptionFeedback) send(data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encoder != nil {
		return s.encoder.Encode(wsMessage{
			ID:      s.id,
			Type:    gqlData,
			Payload: payload,
		})
	}
	return fmt.Errorf("ws channel(#%s) closed", s.id)
}

func message(id, typ string, payload interface{}) wsMessage {
	data, _ := json.Marshal(payload)
	return wsMessage{
		ID:      id,
		Type:    typ,
		Payload: data,
	}
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

	for {
		r := wsutil.NewReader(conn, ws.StateServerSide)
		w := wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText)
		decoder := json.NewDecoder(r)
		encoder := json.NewEncoder(w)

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

		switch op.Type {
		case gqlConnectionInit:
			auth := struct {
				AuthToken string `json:"authToken"`
			}{}
			err = json.Unmarshal(op.Payload, &auth)
			if err != nil {
				encoder.Encode(message(op.ID, gqlConnectionError, err.Error()))
				return
			}

			if engine.authSubscriptionToken != nil {
				ctx, err = engine.authSubscriptionToken(auth.AuthToken)
				if err != nil {
					encoder.Encode(message(op.ID, gqlConnectionError, err.Error()))
					return
				}
			}

			err = encoder.Encode(wsMessage{
				Type: gqlConnectionAck,
			})
			if err != nil {
				//encoder.Encode(message(op.ID, gqlConnectionError, err.Error()))
				return
			}

		case gqlConnectionTerminate:
			return

		case gqlStart:
			payload := struct {
				Query         string                 `json:"query"`
				Variables     map[string]interface{} `json:"variables"`
				OperationName string                 `json:"operationName"`
			}{}
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				_ = encoder.Encode(message(op.ID, gqlError, err.Error()))
				continue
			}

			fb := &subscriptionFeedback{
				id:      op.ID,
				encoder: encoder,
			}
			ctx = context.WithValue(ctx, wsCtxKey{}, fb)

			result, ctx := graphql.Do(graphql.Params{
				Schema:         engine.schema,
				Context:        ctx,
				RequestString:  payload.Query,
				OperationName:  payload.OperationName,
				VariableValues: payload.Variables,
			})

			if subCtx := ctx.Value(subSetupCtxKey{}); subCtx != nil {
				// do nothing
				r := subCtx.(*subInitResult)
				if r.err != nil {
					_ = encoder.Encode(message(op.ID, gqlError, r.err.Error()))
				} else {
					fb.finalize = r.finalize
					mu.Lock()
					sessions[op.ID] = fb
					mu.Unlock()
				}
			} else {
				_ = encoder.Encode(result)
			}

		case gqlStop:
			payload := struct {
				ID string `json:"id"`
			}{}
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				_ = encoder.Encode(message(op.ID, gqlError, err.Error()))
				continue
			}

			mu.Lock()
			if s, ok := sessions[payload.ID]; ok {
				s.close()
				delete(sessions, payload.ID)
			}
			mu.Unlock()

			// tell client no more messages from this ID
			_ = encoder.Encode(message(payload.ID, gqlComplete, fmt.Sprintf(`{"id": "%s"}`, payload.ID)))

		default:

		}
	}
}
