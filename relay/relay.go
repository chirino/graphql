package relay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	graphql "github.com/chirino/graphql"
	"github.com/chirino/graphql/customtypes"
	"github.com/chirino/graphql/errors"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"sync"
)

func MarshalID(kind string, spec interface{}) customtypes.ID {
	d, err := json.Marshal(spec)
	if err != nil {
		panic(fmt.Errorf("relay.MarshalID: %s", err))
	}
	return customtypes.ID(base64.URLEncoding.EncodeToString(append([]byte(kind+":"), d...)))
}

func UnmarshalKind(id customtypes.ID) string {
	s, err := base64.URLEncoding.DecodeString(string(id))
	if err != nil {
		return ""
	}
	i := strings.IndexByte(string(s), ':')
	if i == -1 {
		return ""
	}
	return string(s[:i])
}

func UnmarshalSpec(id customtypes.ID, v interface{}) error {
	s, err := base64.URLEncoding.DecodeString(string(id))
	if err != nil {
		return err
	}
	i := strings.IndexByte(string(s), ':')
	if i == -1 {
		return errors.Errorf("invalid graphql.ID")
	}
	return json.Unmarshal([]byte(s[i+1:]), v)
}

type Handler struct {
	Engine *graphql.Engine
}

type OperationMessage struct {
	Id      interface{}     `json:"id,omitempty"`
	Type    string          `json:"type,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (h *Handler) Upgrade(w http.ResponseWriter, r *http.Request) {

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	header := http.Header{}

	subprotocol := r.Header.Get("Sec-WebSocket-Protocol")
	switch subprotocol {
	case "graphql-ws":
		fallthrough
	case "graphql-subscriptions":
		header.Set("Sec-WebSocket-Protocol", subprotocol)
	}

	conn, _ := upgrader.Upgrade(w, r, header) // error ignored for sake of simplicity
	defer conn.Close()

	op := OperationMessage{}
	err := conn.ReadJSON(&op)
	if err != nil {
		fmt.Printf("websocket read failure\n")
		return
	}
	if op.Type != "connection_init" {
		fmt.Printf("protocol violation: expected an init message, but received: %v\n", op.Type)
		return
	}

	// websocket connections do not support concurrent write access.. protect with a mutex.
	mu := sync.Mutex{}
	writeJSON := func(json interface{}) error {
		mu.Lock()
		err := conn.WriteJSON(json)
		mu.Unlock()
		return err
	}

	writeJSON(OperationMessage{Type: "connection_ack"})
	streams := map[interface{}]*graphql.ResponseStream{}

	for {

		msg := OperationMessage{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			fmt.Printf("websocket read failure\n")
			return
		}

		switch msg.Type {
		case "start":

			var request graphql.EngineRequest
			err := json.Unmarshal(msg.Payload, &request)
			if err != nil {
				fmt.Printf("could not read payload: %v\n", err)
				return
			}

			ctx := withValue(withValue(r.Context(), "net/http.ResponseWriter", w), "*net/http.Request", r)
			request.Context = ctx
			stream, err := h.Engine.Execute(&request)

			if err != nil {
				r := graphql.EngineResponse{Errors: errors.AsArray(err)}
				payload, err := json.Marshal(r)
				if err != nil {
					panic(fmt.Sprintf("could not marshal payload: %v\n", err))
				}
				writeJSON(OperationMessage{Type: "error", Id: msg.Id, Payload: json.RawMessage(payload)})
				return
			}

			if stream.IsSubscription {
				// save it.. so that client can later cancel it...
				streams[msg.Id] = stream

				// Start a goroutine ot handle the events....
				go func() {
					for {
						r := stream.Next()
						if r != nil {
							payload, err := json.Marshal(r)
							if err != nil {
								panic(fmt.Sprintf("could not marshal payload: %v\n", err))
							}
							writeJSON(OperationMessage{Type: "data", Id: msg.Id, Payload: json.RawMessage(payload)})
						} else {
							writeJSON(OperationMessage{Type: "complete", Id: msg.Id})
						}
					}
				}()

			} else {

				r := stream.Next()
				payload, err := json.Marshal(r)
				if err != nil {
					fmt.Println(r)
					stream.Close()
					panic(fmt.Sprintf("could not marshal payload: %v\n", err))
				}
				writeJSON(OperationMessage{Type: "data", Id: msg.Id, Payload: json.RawMessage(payload)})
				writeJSON(OperationMessage{Type: "complete", Id: msg.Id})
				stream.Close()
			}

		case "stop":
			stream := streams[msg.Id]
			if stream != nil {
				stream.Close()
				delete(streams, msg.Id)
			}
		}
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	upgrade := r.Header.Get("Upgrade")
	if upgrade == "websocket" {
		h.Upgrade(w, r)
		return
	}

	var request graphql.EngineRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Attach the response and request to the context, in case a resolver wants to
	// work at the the http level.
	ctx := withValue(withValue(r.Context(), "net/http.ResponseWriter", w), "*net/http.Request", r)
	request.Context = ctx
	reponse := h.Engine.ExecuteOne(&request)
	responseJSON, err := json.Marshal(reponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

func withValue(ctx context.Context, key string, v interface{}) context.Context {
	return context.WithValue(ctx, key, v)
}
