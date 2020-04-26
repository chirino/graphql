package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/chirino/graphql"
	"github.com/chirino/graphql/qerrors"
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
	if contentType == "application/json" {
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return response.AddError(err)
		}
		return response
	}

	return response.AddError(qerrors.Errorf("invalid content type: %s", contentType))
}

// TODO: to support subscriptions, we would need to implmeent the following using websockets..
//func (*Client) Stream(engine *Engine) Execute(request *EngineRequest) (*ResponseStream, error) {
//}
