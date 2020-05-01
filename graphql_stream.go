package graphql

import "github.com/chirino/graphql/qerrors"

type ResponseStream interface {
	Close()
	Responses() <-chan *Response
}

type errStream <-chan *Response

func (e errStream) Close() {
}
func (e errStream) Responses() <-chan *Response {
	return e
}

func NewErrStream(err error) ResponseStream {
	rc := make(chan *Response, 1)
	rc <- NewResponse().AddError(err)
	close(rc)
	return errStream(rc)
}

type StreamingHandler interface {
	ServeGraphQLStream(request *Request) ResponseStream
}

type ServeGraphQLStreamFunc func(request *Request) ResponseStream

func (f ServeGraphQLStreamFunc) ServeGraphQLStream(request *Request) ResponseStream {
	return f(request)
}

func (f ServeGraphQLStreamFunc) ServeGraphQL(request *Request) *Response {
	stream := f(request)
	defer stream.Close()
	response := <-stream.Responses()
	if response == nil {
		response = &Response{}
		response.AddError(qerrors.New("response stream closed.").WithStack())
	}
	return response
}
