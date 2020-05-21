package httpgql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/chirino/graphql"
)

type Handler struct {
	ServeGraphQL       graphql.ServeGraphQLFunc
	ServeGraphQLStream graphql.ServeGraphQLStreamFunc
	MaxRequestSizeBytes int64
	Indent             string
}

type OperationMessage struct {
	Id      interface{}     `json:"id,omitempty"`
	Type    string          `json:"type,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	handlerFunc := h.ServeGraphQL
	streamingHandlerFunc := h.ServeGraphQLStream

	if handlerFunc == nil && streamingHandlerFunc != nil {
		handlerFunc = streamingHandlerFunc.ServeGraphQL
	}

	if handlerFunc == nil {
		panic("either HandlerFunc or StreamingHandlerFunc must be configured")
	}

	if streamingHandlerFunc != nil {
		u := strings.ToLower(r.Header.Get("Upgrade"))
		if u == "websocket" {
			Upgrade(w, r, streamingHandlerFunc)
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

		// Fallback to using query parameters
		if request.Query == "" {
			request.Query = r.URL.Query().Get("query")
		}
		if request.Variables == nil {
			request.Variables = json.RawMessage(r.URL.Query().Get("variables"))
		}
		if request.OperationName == "" {
			request.OperationName = r.URL.Query().Get("operationName")
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
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", h.Indent)
	err := encoder.Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
