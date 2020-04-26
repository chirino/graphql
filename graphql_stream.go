package graphql

import (
	"context"

	"github.com/chirino/graphql/qerrors"
)

type ResponseStream struct {
	Cancel          context.CancelFunc
	Responses       chan *Response
	IsSubscription  bool
	ResponseCounter int
}

type StreamingHandler interface {
	ServeGraphQLStream(request *Request) (*ResponseStream, error)
}

type ServeGraphQLStreamFunc func(request *Request) (*ResponseStream, error)

func (f ServeGraphQLStreamFunc) ServeGraphQLStream(request *Request) (*ResponseStream, error) {
	return f(request)
}

func (f ServeGraphQLStreamFunc) ServeGraphQL(request *Request) *Response {
	stream, err := f(request)
	if err != nil {
		return NewResponse().AddError(err)
	}
	defer stream.Close()
	if stream.IsSubscription {
		return NewResponse().AddError(qerrors.Errorf(
			"ExecuteOne method does not support getting results from subscriptions",
		))
	}
	return stream.Next()
}

func (qr *ResponseStream) Next() *Response {
	if !qr.IsSubscription && qr.ResponseCounter > 0 {
		return nil
	}
	response := <-qr.Responses
	if response != nil {
		qr.ResponseCounter += 1
	}
	return response
}

func (qr *ResponseStream) Close() {
	close(qr.Responses)
	qr.Cancel()
}
