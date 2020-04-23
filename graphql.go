package graphql

import (
	"github.com/chirino/graphql/internal/introspection"
	"github.com/chirino/graphql/schema"
)

func GetSchema(api StandardAPI) (*schema.Schema, error) {
	resp := api(&EngineRequest{
		Query: introspection.Query,
	})
	if resp.Error() != nil {
		return nil, resp.Error()
	}
	return introspection.NewSchema(resp.Data)
}
