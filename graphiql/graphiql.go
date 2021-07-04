package graphiql

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

type Handler struct {
	url      *url.URL
	ws       bool
	template *template.Template
}

func New(urlPath string, ws bool) *Handler {
	html := `
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
{{ if .WebSocket }}
  <script src="//unpkg.com/subscriptions-transport-ws@0.9.16/browser/client.js"></script>
  <script src="//unpkg.com/graphiql-subscriptions-fetcher@0.0.2/browser/client.js"></script>
{{ end }}

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

{{ if .WebSocket }}
      let subscriptionsClient = new window.SubscriptionsTransportWs.SubscriptionClient('{{.URL}}', {
        reconnect: true
      });
      
      function graphQLFetcher(graphQLParams) {
        return subscriptionsClient.request(graphQLParams);

      };
{{ else }}

      // Defines a GraphQL fetcher using the fetch API. You're not required to
      // use fetch, and could instead implement graphQLFetcher however you like,
      // as long as it returns a Promise or Observable.
      function graphQLFetcher(graphQLParams) {
          // This example expects a GraphQL server at the path /graphql.
          // Change this to point wherever you host your GraphQL server.
          return fetch('{{.URL}}', {
              method: 'post',
              headers: {
                  'Accept': 'application/json',
                  'Content-Type': 'application/json',
              },
              body: JSON.stringify(graphQLParams),
          }).then(function (response) {
              return response.text();
          }).then(function (responseBody) {
              try {
                  return JSON.parse(responseBody);
              } catch (error) {
                  return responseBody;
              }
          });
      }
{{ end }}

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
`
	u, err := url.Parse(urlPath)
	if err != nil {
		panic(err)
	}

	t, err := template.New("index.html").Parse(html)
	if err != nil {
		panic(err)
	}
	return &Handler{
		url:      u,
		ws:       ws,
		template: t,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := *h.url

	scheme, host, _ := originalSchemeHostClient(r)
	if u.Scheme == "" {
		u.Scheme = scheme
	}
	if u.Host == "" {
		u.Host = host
	}

	if h.ws {
		switch u.Scheme {
		case "http":
			u.Scheme = "ws"
		case "https":
			u.Scheme = "wss"
		}
	}

	w.Header().Set("Content-Type", "text/html")
	buf := &bytes.Buffer{}
	err := h.template.Execute(buf, struct {
		WebSocket bool
		URL       string
	}{
		WebSocket: h.ws,
		URL:       u.String(),
	})
	if err != nil {
		panic(err)
	}
	w.Write(buf.Bytes())
}

func originalSchemeHostClient(r *http.Request) (scheme string, host string, client string) {

	h := r.Header.Get("Forwarded")
	if h != "" {
		for _, kv := range strings.Split(h, ";") {
			if pair := strings.SplitN(kv, "=", 2); len(pair) == 2 {
				switch strings.ToLower(pair[0]) {
				case "for":
					client = pair[1]
				case "host":
					host = pair[1]
				case "proto":
					scheme = pair[1]
				}
			}
		}
	}

	if scheme == "" {
		scheme = r.Header.Get("X-Forwarded-Proto")
	}
	if host == "" {
		host = r.Header.Get("X-Forwarded-Host")
	}
	if client == "" {
		client = r.Header.Get("X-Forwarded-For")
	}

	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if host == "" {
		host = r.Host
	}
	if client == "" {
		client = r.RemoteAddr
	}
	return
}
