package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/qerrors"
	"github.com/gorilla/websocket"
)

type Client struct {
	URL           string
	HTTPClient    *http.Client
	connections   map[string]*connection
	requestHeader http.Header
	mu            sync.Mutex
}

func NewClient(url string) *Client {
	return &Client{
		URL:           url,
		connections:   map[string]*connection{},
		requestHeader: http.Header{},
	}
}

func (client *Client) ServeGraphQL(request *graphql.Request) *graphql.Response {
	c := client.HTTPClient
	if c == nil {
		c = &http.Client{}
	}

	response := graphql.NewResponse()
	body, err := json.Marshal(request)
	if err != nil {
		return response.AddError(err)
	}

	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.URL, bytes.NewReader(body))
	if err != nil {
		return response.AddError(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return response.AddError(err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return response.AddError(err)
		}
		return response
	}

	return response.AddError(qerrors.Errorf("invalid content type: %s", contentType))
}

func (client *Client) ServeGraphQLStream(request *graphql.Request) graphql.ResponseStream {
	url := client.URL

	client.mu.Lock()
	c := client.connections[url]
	client.mu.Unlock()

	if c == nil {
		c = &connection{
			client:          client,
			url:             url,
			serviceCommands: make(chan interface{}),
			streams:         map[int64]*stream{},
			idleFlag:        false,
		}

		parsed, err := neturl.Parse(client.URL)
		if err != nil {
			return graphql.NewErrStream(err)
		}
		switch parsed.Scheme {
		case "http":
			parsed.Scheme = "ws"
		case "https":
			parsed.Scheme = "wss"
		}
		wsUrl := parsed.String()

		headers := client.requestHeader.Clone()
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

type connection struct {
	client          *Client
	url             string
	serviceCommands chan interface{}
	websocket       *websocket.Conn

	// these fields should only be mutated by the service* methods.
	nextStreamId int64
	streams      map[int64]*stream
	idleFlag     bool
	err          error
}

func (c *connection) Close() {
	c.serviceCommands <- "close"
	close(c.serviceCommands)
}

func (c *connection) ServeGraphQLStream(request *graphql.Request) graphql.ResponseStream {
	stream := &stream{
		connection:      c,
		request:         request,
		responseChannel: make(chan *graphql.Response, 1),
	}

	// request the stream, and wait for the result...
	c.serviceCommands <- stream
	return stream
}

func service(c *connection) {
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
			case *stream:
				c.idleFlag = false
				serviceOpenStream(c, command)
			case closeStream:
				c.idleFlag = false
				serviceCloseStream(c, command)
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

func serviceClose(c *connection) {
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

type stream struct {
	connection      *connection
	id              int64
	request         *graphql.Request
	responseChannel chan *graphql.Response
}

func (s *stream) Close() {
	s.connection.serviceCommands <- closeStream{stream: s}
}

func (s *stream) Responses() <-chan *graphql.Response {
	return s.responseChannel
}

func serviceOpenStream(c *connection, stream *stream) {
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

type closeStream struct {
	stream *stream
}

func serviceCloseStream(c *connection, command closeStream) {
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

func serviceError(c *connection, err error) {
	for _, s := range c.streams {
		r := &graphql.Response{}
		r.AddError(err)
		s.responseChannel <- r
		close(s.responseChannel)
	}
	c.streams = map[int64]*stream{}
	c.err = err
}

func serviceRead(c *connection, command OperationMessage) {

	switch command.Type {
	case "data":
		id := command.Id.(float64)
		stream := serviceStream(c, int64(id))
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
		stream := serviceStream(c, int64(id))
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

func serviceStream(c *connection, id int64) *stream {
	stream := c.streams[id]
	if stream == nil {
		serviceError(c, fmt.Errorf("invalid operation id received: %v", id))
		return nil
	}
	return stream
}
