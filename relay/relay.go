package relay

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "github.com/chirino/graphql/customtypes"
    "github.com/gorilla/websocket"
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

type OperationMessage struct {
    Id      interface{}     `json:"id"`
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
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
    conn.WriteJSON(OperationMessage{Type: "connection_ack"})

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
            err := json.Unmarshal(msg.Payload, request)
            if err != nil {
                fmt.Printf("could not read payload: %v\n", err)
                return
            }

            ctx := withValue(withValue(r.Context(), "net/http.ResponseWriter", w), "*net/http.Request", r)
            response := h.Engine.Execute(ctx, &request, nil)

            data, err := json.Marshal(response)
            if err != nil {
                fmt.Printf("could not marshal payload: %v\n", err)
                return
            }

            resultType := "data"
            if len(response.Errors) > 0 {
                resultType = "error"
            }
            conn.WriteJSON(OperationMessage{Type: resultType, Id: msg.Id, Payload: json.RawMessage(data)})
            conn.WriteJSON(OperationMessage{Type: "complete", Id: msg.Id})

        case "stop":

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
