package graphql

// GetSchemaIntrospectionJSON returns the JSON that describes the Schema
// in the introspection format expected by GraphiQL
func (engine *Engine) GetSchemaIntrospectionJSON() ([]byte, error) {
    result := engine.ExecuteOne(&EngineRequest{
        Query: introspectionQuery,
    })
    return result.Data, result.Error()
}

const introspectionQuery = `
 query {
   __schema {
     queryType { name }
     mutationType { name }
     subscriptionType { name }
     types {
       ...FullType
     }
     directives {
       name
       description
       locations
       args {
         ...InputValue
       }
     }
   }
 }
 fragment FullType on __Type {
   kind
   name
   description
   fields(includeDeprecated: true) {
     name
     description
     args {
       ...InputValue
     }
     type {
       ...TypeRef
     }
     isDeprecated
     deprecationReason
   }
   inputFields {
     ...InputValue
   }
   interfaces {
     ...TypeRef
   }
   enumValues(includeDeprecated: true) {
     name
     description
     isDeprecated
     deprecationReason
   }
   possibleTypes {
     ...TypeRef
   }
 }
 fragment InputValue on __InputValue {
   name
   description
   type { ...TypeRef }
   defaultValue
 }
 fragment TypeRef on __Type {
   kind
   name
   ofType {
     kind
     name
     ofType {
       kind
       name
       ofType {
         kind
         name
         ofType {
           kind
           name
           ofType {
             kind
             name
             ofType {
               kind
               name
               ofType {
                 kind
                 name
               }
             }
           }
         }
       }
     }
   }
 }
`
