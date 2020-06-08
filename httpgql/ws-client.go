package httpgql

import (
	"context"
	"encoding/json"
	"fmt"
	url2 "net/url"
	"time"

	"github.com/chirino/graphql"
	"github.com/gorilla/websocket"
)

func (client *Client) Exec(ctx context.Context, result interface{}, query string, args ...interface{}) error {
	return graphql.Exec(client.ServeGraphQL, ctx, result, query, args...)
}

func (client *Client) ServeGraphQLStream(request *graphql.Request) graphql.ResponseStream {
	url := client.URL

	client.mu.Lock()
	c := client.connections[url]
	client.mu.Unlock()

	if c == nil {
		c = &wsConnection{
			client:          client,
			url:             url,
			serviceCommands: make(chan interface{}),
			streams:         map[int64]*wsOperation{},
			idleFlag:        false,
		}

		wsUrl, err := ToWsURL(client.URL)
		if err != nil {
			return graphql.NewErrStream(err)
		}

		headers := client.RequestHeader.Clone()
		headers.Set("Sec-WebSocket-Protocol", "graphql-ws")

		c.websocket, _, err = websocket.DefaultDialer.Dial(wsUrl, headers)
		if err != nil {
			return graphql.NewErrStream(err)
		}

		err = c.websocket.WriteJSON(OperationMessage{Type: "connection_init"})
		if err != nil {
			c.websocket.Close()
			return graphql.NewErrStream(err)
		}

		op := OperationMessage{}
		err = c.websocket.ReadJSON(&op)
		if err != nil {
			c.websocket.Close()
			return graphql.NewErrStream(err)
		}
		if op.Type != "connection_ack" {
			c.websocket.Close()
			return graphql.NewErrStream(fmt.Errorf("protocol violation: expected an init message, but received: %v\n", op.Type))
		}
		go service(c)

		client.mu.Lock()
		client.connections[url] = c
		client.mu.Unlock()
	}
	return c.ServeGraphQLStream(request)
}

func ToWsURL(url string) (string, error) {
	parsed, err := url2.Parse(url)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}
	wsUrl := parsed.String()
	return wsUrl, nil
}

type wsConnection struct {
	client          *Client
	url             string
	serviceCommands chan interface{}
	websocket       *websocket.Conn

	// these fields should only be mutated by the service* methods.
	nextStreamId int64
	streams      map[int64]*wsOperation
	idleFlag     bool
	err          error
}

func (c *wsConnection) Close() {
	c.serviceCommands <- "close"
	close(c.serviceCommands)
}

func (c *wsConnection) ServeGraphQLStream(request *graphql.Request) graphql.ResponseStream {
	stream := &wsOperation{
		request:         request,
		responseChannel: make(chan *graphql.Response, 1),
	}
	c.serviceCommands <- stream
	return stream.responseChannel
}

func service(c *wsConnection) {
	ticker := time.NewTicker(60 * time.Second)
	defer func() {
		c.websocket.Close()
		ticker.Stop()
	}()

	go func() {

		// This is the read side goroutine
		defer func() {
			close(c.serviceCommands)
			c.client.mu.Lock()
			if c.client.connections[c.url] == c {
				delete(c.client.connections, c.url)
			}
			c.client.mu.Unlock()
		}()

		for {
			o := OperationMessage{}
			err := c.websocket.ReadJSON(&o)
			if err != nil {
				c.serviceCommands <- err
				return
			}
			c.serviceCommands <- o
		}
	}()

	for {
		select {
		case command := <-c.serviceCommands:
			switch command := command.(type) {
			case *wsOperation:
				c.idleFlag = false
				serviceOpenOperation(c, command)
			case closeOperation:
				c.idleFlag = false
				serviceCloseOperation(c, command)
			case OperationMessage:
				c.idleFlag = false
				serviceRead(c, command)
			case error:
				serviceError(c, command)
				return
			case string:
				if command == "close" {
					serviceClose(c)
					return
				}
			}

			//case "shutdown":
			//	log.Println("interrupt")
			//
			//	// Cleanly close the connection by sending a close message and then
			//	// waiting (with timeout) for the server to close the connection.
			//	err := c.websocket.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			//	if err != nil {
			//		log.Println("write close:", err)
			//		return
			//	}
			//	select {
			//	case <-c.commands:
			//	case <-time.After(time.Second):
			//	}
			//	return

		case <-ticker.C:
			if c.idleFlag && len(c.streams) == 0 {

			}
			c.idleFlag = true
		}
	}

}

func serviceClose(c *wsConnection) {
	err := c.websocket.WriteJSON(OperationMessage{
		Type: "connection_terminate",
	})
	if err != nil {
		serviceError(c, err)
		return
	}
	err = c.websocket.Close()
	if err != nil {
		serviceError(c, err)
		return
	}
}

type wsOperation struct {
	id              int64
	request         *graphql.Request
	responseChannel chan *graphql.Response
}

func serviceOpenOperation(c *wsConnection, stream *wsOperation) {
	payload, err := json.Marshal(stream.request)
	if err != nil {
		stream.responseChannel <- graphql.NewResponse().AddError(err)
		close(stream.responseChannel)
		return
	}
	stream.request = nil
	streamId := c.nextStreamId
	c.nextStreamId += 1
	err = c.websocket.WriteJSON(OperationMessage{
		Id:      streamId,
		Type:    "start",
		Payload: payload,
	})
	if err != nil {
		stream.responseChannel <- graphql.NewResponse().AddError(err)
		close(stream.responseChannel)
		return
	}
	stream.id = streamId
	c.streams[streamId] = stream
}

type closeOperation struct {
	stream *wsOperation
}

func serviceCloseOperation(c *wsConnection, command closeOperation) {
	id := command.stream.id
	if c.streams[id] == nil {
		return // it was already closed...
	}

	// let the server know we want it to close the stream.
	// it wil respond with a complete.
	c.websocket.WriteJSON(OperationMessage{
		Id:   id,
		Type: "stop",
	})
}

func serviceError(c *wsConnection, err error) {
	for _, s := range c.streams {
		r := &graphql.Response{}
		r.AddError(err)
		s.responseChannel <- r
		close(s.responseChannel)
	}
	c.streams = map[int64]*wsOperation{}
	c.err = err
}

func serviceRead(c *wsConnection, command OperationMessage) {

	switch command.Type {
	case "data":
		id := command.Id.(float64)
		stream := serviceOperation(c, int64(id))
		if stream == nil {
			return
		}
		response := &graphql.Response{}
		err := json.Unmarshal(command.Payload, &response)
		if err != nil {
			response.AddError(err)
		}
		stream.responseChannel <- response

	case "complete":
		id := command.Id.(float64)
		stream := serviceOperation(c, int64(id))
		if stream == nil {
			return
		}
		delete(c.streams, stream.id)
		close(stream.responseChannel)

	case "ka":
		// keep alive.
	case "connection_ack":
		// server accepted the connection
	case "connection_error":
		serviceError(c, fmt.Errorf("graphql connection error: %s", string(command.Payload)))
	}
}

func serviceOperation(c *wsConnection, id int64) *wsOperation {
	stream := c.streams[id]
	if stream == nil {
		serviceError(c, fmt.Errorf("invalid operation id received: %v", id))
		return nil
	}
	return stream
}
