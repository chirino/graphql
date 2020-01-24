package relay

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/chirino/graphql/customtypes"
    "net/http"
    "strings"

    graphql "github.com/chirino/graphql"
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
		return errors.New("invalid graphql.ID")
	}
	return json.Unmarshal([]byte(s[i+1:]), v)
}

type Handler struct {
	Engine *graphql.Engine
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request graphql.EngineRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var response *graphql.EngineResponse = nil
	// Attach the response and request to the context, in case a resolver wants to
	// work at the the http level.
	ctx := withValue(withValue(r.Context(), "net/http.ResponseWriter", w), "*net/http.Request", r)
	response = h.Engine.Execute(ctx, &request, nil)
	responseJSON, err := json.Marshal(response)
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