package gqltesting

import (
	"encoding/json"
	"github.com/chirino/graphql"
	"github.com/stretchr/testify/assert"
	"testing"
)

func AssertQuery(t *testing.T, engine *graphql.Engine, query string, expected string) {
	request := graphql.Request{}
	request.Query = query
	AssertRequest(t, engine, request, expected)
}

func AssertRequestString(t *testing.T, engine *graphql.Engine, req string, expected string) {
	request := graphql.Request{}
	jsonUnmarshal(t, req, &request)
	AssertRequest(t, engine, request, expected)
}

func AssertRequest(t *testing.T, engine *graphql.Engine, request graphql.Request, expected string) {
	response := engine.ServeGraphQL(&request)
	actual := jsonMarshal(t, response)
	assert.Equal(t, expected, actual)
}

func jsonMarshal(t *testing.T, value interface{}) string {
	data, err := json.Marshal(value)
	assert.NoError(t, err)
	return string(data)
}

func jsonUnmarshal(t *testing.T, from string, target interface{}) {
	err := json.Unmarshal([]byte(from), target)
	assert.NoError(t, err)
}
