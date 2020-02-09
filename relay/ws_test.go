package relay_test

import (
    "fmt"
    "github.com/chirino/graphql"
    "github.com/chirino/graphql/relay"
    "log"
    "net/http"
    "testing"
)

type query struct {
    Name string `json:"name"`
}

func (q *query) Hello() string { return "Hello, " + q.Name }

func NoTestUpgrade(t *testing.T) {
    engine := graphql.New()
    engine.Root = &query{
        Name: "World!",
    }
    err := engine.Schema.Parse(`
        schema {
            query: Query
        }
        type Query {
            name: String!
            hello: String!
        }
    `)
    if err != nil {
        log.Fatal(err)
    }

    http.Handle("/graphql", &relay.Handler{Engine: engine})
    fmt.Println("WS GraphQL service running at http://localhost:8080/graphql")
    http.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
        w.Header().Set("Content-Type", "text/html")
        w.Write([]byte(`


<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="robots" content="noindex" />
  <meta name="referrer" content="origin" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>SWAPI GraphQL API</title>
  <style>
    body {
      height: 100vh;
      margin: 0;
      overflow: hidden;
    }
    #splash {
      color: #333;
      display: flex;
      flex-direction: column;
      font-family: system, -apple-system, "San Francisco", ".SFNSDisplay-Regular", "Segoe UI", Segoe, "Segoe WP", "Helvetica Neue", helvetica, "Lucida Grande", arial, sans-serif;
      height: 100vh;
      justify-content: center;
      text-align: center;
    }
  </style>
  <link rel="icon" href="favicon.ico">
  <link type="text/css" href="//unpkg.com/graphiql@0.17.0/graphiql.css" rel="stylesheet" />
</head>
<body>
  <div id="splash">
    Loading&hellip;
  </div>
  <script src="//cdn.jsdelivr.net/es6-promise/4.0.5/es6-promise.auto.min.js"></script>
  <script src="//cdn.jsdelivr.net/react/15.4.2/react.min.js"></script>
  <script src="//cdn.jsdelivr.net/react/15.4.2/react-dom.min.js"></script>
  <script src="//unpkg.com/graphiql@0.17.0/graphiql.min.js"></script>
  <script src="//unpkg.com/subscriptions-transport-ws@0.9.16/browser/client.js"></script>
  <script src="//unpkg.com/graphiql-subscriptions-fetcher@0.0.2/browser/client.js"></script>

  <script>
      // Parse the search string to get url parameters.
      var search = window.location.search;
      var parameters = {};
      search.substr(1).split('&').forEach(function (entry) {
        var eq = entry.indexOf('=');
        if (eq >= 0) {
          parameters[decodeURIComponent(entry.slice(0, eq))] =
            decodeURIComponent(entry.slice(eq + 1));
        }
      });

      // if variables was provided, try to format it.
      if (parameters.variables) {
        try {
          parameters.variables =
            JSON.stringify(JSON.parse(parameters.variables), null, 2);
        } catch (e) {
          // Do nothing, we want to display the invalid JSON as a string, rather
          // than present an error.
        }
      }

      // When the query and variables string is edited, update the URL bar so
      // that it can be easily shared
      function onEditQuery(newQuery) {
        parameters.query = newQuery;
        updateURL();
      }
      function onEditVariables(newVariables) {
        parameters.variables = newVariables;
        updateURL();
      }
      function onEditOperationName(newOperationName) {
        parameters.operationName = newOperationName;
        updateURL();
      }
      function updateURL() {
        var newSearch = '?' + Object.keys(parameters).filter(function (key) {
          return Boolean(parameters[key]);
        }).map(function (key) {
          return encodeURIComponent(key) + '=' +
            encodeURIComponent(parameters[key]);
        }).join('&');
        history.replaceState(null, null, newSearch);
      }


    //let subscriptionsClient = new window.SubscriptionsTransportWs.SubscriptionClient('ws://localhost:8080/graphql', {
    let subscriptionsClient = new window.SubscriptionsTransportWs.SubscriptionClient('ws://localhost:3031/graphql', {
        reconnect: true
    });

    function graphQLFetcher(graphQLParams) {
        return subscriptionsClient.request(graphQLParams);
    };

      // Render <GraphiQL /> into the body.
      ReactDOM.render(
        React.createElement(GraphiQL, {
          fetcher: graphQLFetcher,
          query: parameters.query,
          variables: parameters.variables,
          operationName: parameters.operationName,
          onEditQuery: onEditQuery,
          onEditVariables: onEditVariables,
          onEditOperationName: onEditOperationName
        }),
        document.body,
      );
  </script>
</body>
</html>
`,
/*
`
<!--
 *  Copyright (c) Facebook, Inc.
 *  All rights reserved.
 *
 *  This source code is licensed under the license found in the
 *  LICENSE file in the root directory of this source tree.
-->
<!DOCTYPE html>
<html>
<head>
    <style>
        body {
            height: 100%;
            margin: 0;
            width: 100%;
            overflow: hidden;
        }
        #graphiql {
            height: 100vh;
        }
    </style>

    <!--
      This GraphiQL example depends on Promise and fetch, which are available in
      modern browsers, but can be "polyfilled" for older browsers.
      GraphiQL itself depends on React DOM.
      If you do not want to rely on a CDN, you can host these files locally or
      include them directly in your favored resource bunder.
    -->
	<script src="//cdn.jsdelivr.net/npm/es6-promise@4.2.6/dist/es6-promise.auto.min.js"></script>
    <script src="//cdnjs.cloudflare.com/ajax/libs/fetch/2.0.4/fetch.min.js"></script>
    <script src="//cdnjs.cloudflare.com/ajax/libs/react/16.8.4/umd/react.production.min.js"></script>
    <script src="//cdnjs.cloudflare.com/ajax/libs/react-dom/16.8.4/umd/react-dom.production.min.js"></script>

    <!--
      These two files can be found in the npm module, however you may wish to
      copy them directly into your environment, or perhaps include them in your
      favored resource bundler.
     -->
    <link rel="stylesheet" href="//cdn.jsdelivr.net/npm/graphiql@0.13.0/graphiql.css" />
    <script src="//cdn.jsdelivr.net/npm/graphiql@0.13.0/graphiql.js"></script>
    <script src="//unpkg.com/subscriptions-transport-ws@0.9.16/browser/client.js"></script>
    <script src="//unpkg.com/graphiql-subscriptions-fetcher@0.0.2/browser/client.js"></script>

</head>
<body>
<div id="graphiql">Loading...</div>
<script>


            // Parse the search string to get url parameters.
    var search = window.location.search;
    var parameters = {};
    search.substr(1).split('&').forEach(function (entry) {
        var eq = entry.indexOf('=');
        if (eq >= 0) {
            parameters[decodeURIComponent(entry.slice(0, eq))] =
                    decodeURIComponent(entry.slice(eq + 1));
        }
    });

    // if variables was provided, try to format it.
    if (parameters.variables) {
        try {
            parameters.variables =
                    JSON.stringify(JSON.parse(parameters.variables), null, 2);
        } catch (e) {
            // Do nothing, we want to display the invalid JSON as a string, rather
            // than present an error.
        }
    }

    // When the query and variables string is edited, update the URL bar so
    // that it can be easily shared
    function onEditQuery(newQuery) {
        parameters.query = newQuery;
        updateURL();
    }

    function onEditVariables(newVariables) {
        parameters.variables = newVariables;
        updateURL();
    }

    function onEditOperationName(newOperationName) {
        parameters.operationName = newOperationName;
        updateURL();
    }

    function updateURL() {
        var newSearch = '?' + Object.keys(parameters).filter(function (key) {
            return Boolean(parameters[key]);
        }).map(function (key) {
            return encodeURIComponent(key) + '=' +
                    encodeURIComponent(parameters[key]);
        }).join('&');
        history.replaceState(null, null, newSearch);
    }

    let subscriptionsClient = new window.SubscriptionsTransportWs.SubscriptionClient('ws://localhost:3031/graphql', {
    //let subscriptionsClient = new window.SubscriptionsTransportWs.SubscriptionClient('ws://localhost:8080/graphql', {
        reconnect: true
    });

    function graphQLFetcher(graphQLParams) {
        return subscriptionsClient.request(graphQLParams);
    };

    //// Defines a GraphQL fetcher using the fetch API. You're not required to
    //// use fetch, and could instead implement graphQLFetcher however you like,
    //// as long as it returns a Promise or Observable.
    //function graphQLFetcher(graphQLParams) {
    //    return fetch('/graphql', {
    //        method: 'post',
    //        headers: {
    //            'Accept': 'application/json',
    //            'Content-Type': 'application/json',
    //        },
    //        body: JSON.stringify(graphQLParams),
    //    }).then(function (response) {
    //        return response.text();
    //    }).then(function (responseBody) {
    //        try {
    //            return JSON.parse(responseBody);
    //        } catch (error) {
    //            return responseBody;
    //        }
    //    });
    //}
    let myCustomFetcher = window.GraphiQLSubscriptionsFetcher.graphQLFetcher(subscriptionsClient, graphQLFetcher);

    // Render <GraphiQL /> into the body.
    // See the README in the top level of this module to learn more about
    // how you can customize GraphiQL by providing different values or
    // additional child elements.
    ReactDOM.render(
            React.createElement(GraphiQL, {
                fetcher: myCustomFetcher,
                query: parameters.query,
                variables: parameters.variables,
                operationName: parameters.operationName,
                onEditQuery: onEditQuery,
                onEditVariables: onEditVariables,
                onEditOperationName: onEditOperationName
            }),
            document.getElementById('graphiql')
    );
</script>
</body>
</html>
`*/
))
    })
    fmt.Println("GraphiQL UI running at http://localhost:8080/")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
