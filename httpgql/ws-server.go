package httpgql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/chirino/graphql"
	"github.com/gorilla/websocket"
)

type wsStream struct {
	cancel    context.CancelFunc
	responses graphql.ResponseStream
}

func Upgrade(w http.ResponseWriter, r *http.Request, streamingHandlerFunc graphql.ServeGraphQLStreamFunc) {

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
	streams := map[interface{}]wsStream{}
	conn, _ := upgrader.Upgrade(w, r, header) // error ignored for sake of simplicity
	defer func() {
		mu.Lock()
		for _, stream := range streams {
			stream.cancel()
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
			return
		}

		switch msg.Type {
		case "start":

			var request graphql.Request
			err := json.Unmarshal(msg.Payload, &request)
			if err != nil {
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, "net/http.ResponseWriter", w)
			ctx = context.WithValue(ctx, "*net/http.Request", r)

			stream := wsStream{}
			request.Context, stream.cancel = context.WithCancel(ctx)
			stream.responses = streamingHandlerFunc(&request)

			// save it.. so that client can later cancel it...
			mu.Lock()
			streams[msg.Id] = stream
			mu.Unlock()

			// Start a goroutine ot handle the events....
			go func() {
				for {
					r := <-stream.responses
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
						stream.cancel()
						return
					}
				}
			}()

		case "stop":
			mu.Lock()
			stream, ok := streams[msg.Id]
			mu.Unlock()
			if ok {
				stream.cancel()
			}
		}
	}
}
