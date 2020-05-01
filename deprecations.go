package graphql

// Deprecated: use Request instead
type EngineRequest = Request

// Deprecated: use Response instead
type EngineResponse = Response

// Deprecated: use ServeGraphQL instead.
func (engine *Engine) ExecuteOne(request *EngineRequest) *EngineResponse {
	stream := ServeGraphQLStreamFunc(engine.ServeGraphQLStream)
	return stream.ServeGraphQL(request)
}

// Deprecated: use ServeGraphQLStream instead.
func (engine *Engine) Execute(request *EngineRequest) (ResponseStream, error) {
	return engine.ServeGraphQLStream(request), nil
}
