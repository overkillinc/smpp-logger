package ui

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/overkillinc/smpp-logger/internal/logging"
)

// NewHandler returns an http.Handler that serves a simple UI and logs endpoint.
// It uses basic auth with the provided user/pass (defaults are in config).
func NewHandler(logger *logging.Logger, user, pass string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// simple HTML page that fetches /logs
		// authentication already enforced by middleware in this implementation
		tmpl := `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>smpp-logger UI</title>
<style>body{font-family:sans-serif;margin:1rem} pre{background:#111;color:#eee;padding:1rem}</style>
</head>
<body>
<h1>smpp-logger recent logs</h1>
<p>Showing last <span id="mins">5</span> minutes. <button onclick="dec()">-</button> <button onclick="inc()">+</button> <button onclick="refresh()">Refresh</button></p>
<pre id="logs">Loading...</pre>
<script>
let mins=5
function render(text){document.getElementById('logs').textContent = text}
function fetchLogs(){fetch('/logs?minutes='+mins,{credentials: 'same-origin'}).then(r=>r.text()).then(render).catch(e=>render('fetch error: '+e))}
function inc(){mins+=1;document.getElementById('mins').textContent=mins;fetchLogs()}
function dec(){if(mins>1){mins-=1;document.getElementById('mins').textContent=mins;fetchLogs()}}
function refresh(){fetchLogs()}
fetchLogs();setInterval(fetchLogs,15000)
</script>
</body>
</html>`
		t := template.Must(template.New("ui").Parse(tmpl))
		t.Execute(w, nil)
	})

	m.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		// Basic auth
		userReq, passReq, ok := r.BasicAuth()
		if !ok || userReq != user || passReq != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="smpp-logger"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		minutes := 5
		if v := r.URL.Query().Get("minutes"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				minutes = n
			}
		}
		lines := logger.Recent(minutes)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	})

	m.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Wrap with a middleware that requires basic auth for the root path as well
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// allow healthz without auth
		if r.URL.Path == "/healthz" {
			m.ServeHTTP(w, r)
			return
		}
		// require auth for everything else
		userReq, passReq, ok := r.BasicAuth()
		if !ok || userReq != user || passReq != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="smpp-logger"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		m.ServeHTTP(w, r)
	})
}
