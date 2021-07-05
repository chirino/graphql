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
<html>
<head>
  <title>GraphiQL</title>
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
  <script crossorigin src="https://unpkg.com/react@16/umd/react.development.js"></script>
  <script crossorigin src="https://unpkg.com/react-dom@16/umd/react-dom.development.js"></script>
  <link href="https://unpkg.com/graphiql/graphiql.min.css" rel="stylesheet" />
  <script crossorigin src="https://unpkg.com/graphiql/graphiql.min.js"></script>
</head>
<body>
  <div id="graphiql">Loading...</div>
  <script>
  {{- if .WebSocket }}
    let subscriptionsClient = new window.SubscriptionsTransportWs.SubscriptionClient('{{.URL}}', {
  	reconnect: true
    }); 
    let fetcher = subscriptionsClient.request;
  {{- else }}
    let fetcher = GraphiQL.createFetcher({url: '{{.URL}}'})
  {{- end }}
    ReactDOM.render(
  	  React.createElement(GraphiQL, { 
  	  	fetcher: fetcher,
  	    headerEditorEnabled: true,
  	  	shouldPersistHeaders: true,	
  	  }),
  	  document.getElementById('graphiql'),
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
