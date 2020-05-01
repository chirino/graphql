package relay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/customtypes"
	"github.com/chirino/graphql/qerrors"
	"github.com/gorilla/websocket"
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
		return qerrors.Errorf("invalid graphql.ID")
	}
	return json.Unmarshal([]byte(s[i+1:]), v)
}

type Handler struct {

	// Engine points at the graphql.Engine requests will be issued against.
	//
	// Deprecated: Engine exists for historical compatibility  and should not be used.
	// set HandlerFunc or StreamingHandlerFunc instead.
	Engine *graphql.Engine

	ServeGraphQL       graphql.ServeGraphQLFunc
	ServeGraphQLStream graphql.ServeGraphQLStreamFunc

	MaxRequestSizeBytes int64
}

type OperationMessage struct {
	Id      interface{}     `json:"id,omitempty"`
	Type    string          `json:"type,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func upgrade(streamingHandlerFunc graphql.ServeGraphQLStreamFunc, w http.ResponseWriter, r *http.Request) {

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

	mu := sync.Mutex{}
	streams := map[interface{}]graphql.ResponseStream{}
	conn, _ := upgrader.Upgrade(w, r, header) // error ignored for sake of simplicity
	defer func() {
		mu.Lock()
		for _, stream := range streams {
			stream.Close()
		}
		mu.Unlock()
		conn.Close()
	}()

	// websocket connections do not support concurrent write access.. protect with a mutex.
	writeJSON := func(json interface{}) error {
		mu.Lock()
		err := conn.WriteJSON(json)
		mu.Unlock()
		return err
	}

	op := OperationMessage{}
	err := conn.ReadJSON(&op)
	if err != nil {
		fmt.Printf("websocket read failure\n")
		return
	}
	if op.Type != "connection_init" {
		r := graphql.NewResponse().AddError(fmt.Errorf("protocol violation: expected an init message, but received: %v", op.Type))
		payload, _ := json.Marshal(r)
		writeJSON(OperationMessage{Type: "connection_error", Payload: payload})
		return
	}

	writeJSON(OperationMessage{Type: "connection_ack"})
	for {

		msg := OperationMessage{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			fmt.Printf("websocket read failure\n")
			return
		}

		switch msg.Type {
		case "start":

			var request graphql.Request
			err := json.Unmarshal(msg.Payload, &request)
			if err != nil {
				fmt.Printf("could not read payload: %v\n", err)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, "net/http.ResponseWriter", w)
			ctx = context.WithValue(ctx, "*net/http.Request", r)

			request.Context = ctx
			stream := streamingHandlerFunc(&request)

			// save it.. so that client can later cancel it...
			mu.Lock()
			streams[msg.Id] = stream
			mu.Unlock()

			// Start a goroutine ot handle the events....
			go func() {
				for {
					r := <-stream.Responses()
					if r != nil {
						payload, err := json.Marshal(r)
						if err != nil {
							panic(fmt.Sprintf("could not marshal payload: %v\n", err))
						}
						writeJSON(OperationMessage{Type: "data", Id: msg.Id, Payload: payload})
					} else {

						mu.Lock()
						delete(streams, msg.Id)
						mu.Unlock()

						writeJSON(OperationMessage{Type: "complete", Id: msg.Id})
						stream.Close()
						return
					}
				}
			}()

		case "stop":
			mu.Lock()
			stream := streams[msg.Id]
			mu.Unlock()
			if stream != nil {
				stream.Close()
			}
		}
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	handlerFunc := h.ServeGraphQL
	streamingHandlerFunc := h.ServeGraphQLStream

	if streamingHandlerFunc == nil && h.Engine != nil {
		streamingHandlerFunc = h.Engine.ServeGraphQLStream
	}
	if handlerFunc == nil && streamingHandlerFunc != nil {
		handlerFunc = streamingHandlerFunc.ServeGraphQL
	}

	if handlerFunc == nil {
		panic("either HandlerFunc or StreamingHandlerFunc must be configured")
	}

	if streamingHandlerFunc != nil {
		u := strings.ToLower(r.Header.Get("Upgrade"))
		if u == "websocket" {
			upgrade(streamingHandlerFunc, w, r)
			return
		}
	}

	defer r.Body.Close()
	var request graphql.Request

	switch r.Method {
	case http.MethodGet:
		request.Query = r.URL.Query().Get("query")
		request.Variables = json.RawMessage(r.URL.Query().Get("variables"))
		request.OperationName = r.URL.Query().Get("operationName")
	case http.MethodPost:

		reader := r.Body.(io.Reader)
		if h.MaxRequestSizeBytes > 0 {
			reader = io.LimitReader(reader, h.MaxRequestSizeBytes)
		}

		if err := json.NewDecoder(reader).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Attach the response and request to the context, in case a resolver wants to
	// work at the the http level.
	ctx := r.Context()
	ctx = context.WithValue(ctx, "net/http.ResponseWriter", w)
	ctx = context.WithValue(ctx, "*net/http.Request", r)

	request.Context = ctx
	response := handlerFunc(&request)

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
