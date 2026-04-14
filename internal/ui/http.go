package ui

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/overkillinc/smpp-logger/internal/logging"
)

// NewHandler returns an http.Handler that serves a simple UI and logs endpoint.
// It uses basic auth with the provided user/pass (defaults are in config).
func NewHandler(logger *logging.Logger, user, pass string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// simple HTML page that fetches /logs and supports auto-refresh + filtering
		tmpl := `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>smpp-logger UI</title>
<style>body{font-family:sans-serif;margin:1rem} input[type=text]{width:40%;padding:0.25rem} pre{background:#111;color:#eee;padding:1rem;white-space:pre-wrap;word-break:break-word}</style>
</head>
<body>
<h1>smpp-logger recent logs</h1>
<div>
  <label>Minutes: <span id="mins">5</span></label>
  <button onclick="dec()">-</button>
  <button onclick="inc()">+</button>
  <button onclick="refresh()">Refresh</button>
</div>
<div style="margin-top:0.5rem">
  <label>Filter: <input id="filter" type="text" placeholder="substring to filter (case-insensitive)" /></label>
  <button onclick="applyFilter()">Apply</button>
  <label style="margin-left:1rem"><input id="autorefresh" type="checkbox" checked /> Auto-refresh</label>
</div>
<pre id="logs">Loading...</pre>
<script>
let mins = 5;
let filter = '';
let timer = null;
function render(text){ document.getElementById('logs').textContent = text }
function fetchLogs(){
  let url = '/logs?minutes=' + mins;
  if (filter && filter.trim() !== '') {
    url += '&q=' + encodeURIComponent(filter.trim());
  }
  fetch(url, {credentials: 'include'})
    .then(r => r.text())
    .then(render)
    .catch(e => render('fetch error: ' + e))
}
function inc(){ mins += 1; document.getElementById('mins').textContent = mins; fetchLogs() }
function dec(){ if (mins > 1) { mins -= 1; document.getElementById('mins').textContent = mins; fetchLogs() } }
function refresh(){ fetchLogs() }
function applyFilter(){ filter = document.getElementById('filter').value; fetchLogs() }
// trigger filter on Enter key
document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('filter').addEventListener('keyup', (e) => { if (e.key === 'Enter') applyFilter() })
  // auto-refresh handler
  const auto = document.getElementById('autorefresh')
  function schedule(){ if (timer) clearInterval(timer); if (auto.checked) timer = setInterval(fetchLogs, 5000) }
  auto.addEventListener('change', schedule)
  schedule()
  fetchLogs()
})
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

		q := strings.TrimSpace(r.URL.Query().Get("q"))
		ql := strings.ToLower(q)

		lines := logger.Recent(minutes)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, l := range lines {
			if q != "" {
				if !strings.Contains(strings.ToLower(l), ql) {
					continue
				}
			}
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
