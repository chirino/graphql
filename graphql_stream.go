package graphql

import (
	"context"

	"github.com/chirino/graphql/qerrors"
)

type ResponseStream = <-chan *Response

func NewErrStream(err error) ResponseStream {
	rc := make(chan *Response, 1)
	rc <- NewResponse().AddError(err)
	close(rc)
	return rc
}

type StreamingHandler interface {
	ServeGraphQLStream(request *Request) ResponseStream
}

type ServeGraphQLStreamFunc func(request *Request) ResponseStream

func (f ServeGraphQLStreamFunc) ServeGraphQLStream(request *Request) ResponseStream {
	return f(request)
}

func (f ServeGraphQLStreamFunc) ServeGraphQL(request *Request) *Response {
	ctx, cancel := context.WithCancel(request.GetContext())
	requestCp := *request
	requestCp.Context = ctx

	stream := f(request)
	request.GetContext()

	defer cancel()
	response := <-stream
	if response == nil {
		response = &Response{}
		response.AddError(qerrors.New("response stream closed.").WithStack())
	}
	return response
}
