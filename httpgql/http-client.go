package httpgql

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/qerrors"
)

type Client struct {
	URL           string
	HTTPClient    *http.Client
	connections   map[string]*wsConnection
	RequestHeader http.Header
	mu            sync.Mutex
}

func NewClient(url string) *Client {
	return &Client{
		URL:           url,
		connections:   map[string]*wsConnection{},
		RequestHeader: http.Header{},
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

	req, err := http.NewRequestWithContext(request.GetContext(), http.MethodPost, client.URL, bytes.NewReader(body))
	if err != nil {
		return response.AddError(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	for k, h := range client.RequestHeader {
		req.Header[k] = h
	}

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

	preview, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1024))
	return response.AddError(qerrors.New("invalid content type: %s", contentType).WithCause(errors.New(string(preview))))
}
