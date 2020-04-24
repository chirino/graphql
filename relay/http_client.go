package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/errors"
)

type Client struct {
	URL        string
	HTTPClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{
		URL: url,
	}
}

func (client *Client) ServeGraphQL(request *graphql.Request) *graphql.Response {
	c := client.HTTPClient
	if c == nil {
		c = &http.Client{}
	}

	body, err := json.Marshal(request)
	if err != nil {
		return &graphql.Response{
			Errors: errors.AsArray(err),
		}
	}

	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.URL, bytes.NewReader(body))
	if err != nil {
		return &graphql.Response{
			Errors: errors.AsArray(err),
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return &graphql.Response{
			Errors: errors.AsArray(err),
		}
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "application/json" {
		engineResponse := graphql.Response{}
		err = json.NewDecoder(resp.Body).Decode(&engineResponse)
		if err != nil {
			return &graphql.Response{
				Errors: errors.AsArray(err),
			}
		}
		return &engineResponse
	}

	return &graphql.Response{
		Errors: errors.AsArray(errors.Errorf("invalid content type: %s", contentType)),
	}
}

// TODO: to support subscriptions, we would need to implmeent the following using websockets..
//func (*Client) Stream(engine *Engine) Execute(request *EngineRequest) (*ResponseStream, error) {
//}
