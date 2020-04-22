package relay

import (
	"bytes"
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
	client := &Client{
		URL: url,
	}
	return client
}

func (client *Client) Post(request *graphql.EngineRequest) *graphql.EngineResponse {
	c := client.HTTPClient
	if c == nil {
		c = &http.Client{}
	}

	body, err := json.Marshal(request)
	if err != nil {
		return &graphql.EngineResponse{
			Errors: errors.AsArray(err),
		}
	}

	req, err := http.NewRequestWithContext(request.Context, http.MethodPost, client.URL, bytes.NewReader(body))
	if err != nil {
		return &graphql.EngineResponse{
			Errors: errors.AsArray(err),
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return &graphql.EngineResponse{
			Errors: errors.AsArray(err),
		}
	}
	defer resp.Body.Close()

	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		return &graphql.EngineResponse{
			Errors: errors.AsArray(errors.Errorf("http status code: %d", resp.StatusCode)),
		}
	}

	engineResponse := graphql.EngineResponse{}
	err = json.NewDecoder(resp.Body).Decode(&engineResponse)
	if err != nil {
		return &graphql.EngineResponse{
			Errors: errors.AsArray(err),
		}
	}

	return &engineResponse
}

// TODO: to support subscriptions, we would need to implmeent the following using websockets..
//func (*Client) Stream(engine *Engine) Execute(request *EngineRequest) (*ResponseStream, error) {
//}
