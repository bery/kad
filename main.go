package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var listen = ":5000"
var hits = 0

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

type pageContent struct {
	Vars      map[string]string
	Hostname  string
	Hits      int
	RedisHost string
}

var rootPage = `
<html>
<meta charset="utf-8">

<head>
<title>Kubernetes app demo</title>
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css"
integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous">
<style>
td {
	font-size: 70%;
	}
a {
	font-weight: bold;
}
</style>
</head>

<body>
<div class="container">
<div class="row">

<div class="col-sm-6">

{{ if .Hits }}
<div class="alert alert-info">This worker returned page <strong>{{ .Hits }}</strong> times.</div>
{{ end }}


<div class="alert alert-info">Metrics exported at <a href="/metrics">/metrics</a></div>


{{ if .RedisHost }}
<div class="alert alert-info">Redis host is <code>{{ .RedisHost }}</code></div>
{{ else }}
<div class="alert alert-info">Redis server not used.</div>
{{ end }}

</div>

<div class="col-sm-6">
<table class="table">
<thead>
<tr><th>Variable name</th><th>Value</th></tr>
</thead>
<tbody>
{{ range $name, $value := .Vars }}
<tr><td>{{ $name }}</td><td>{{ $value }}</td></tr>
{{ end }}
</tbody>
</table>
</div>

</div> <!-- row -->
</div> <!-- container -->

</body>
</html>
`

var pageHits = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "page_hits",
	Help: "Number of page visits",
})

func init() {
	err := prometheus.Register(pageHits)
	if err != nil {
		log.Printf("Unable to register pageHits: %s", err)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	hits = hits + 1
	pageHits.Set(float64(hits))

	pc := pageContent{
		Vars: make(map[string]string),
		Hits: hits,
	}

	// read environment variables
	for _, v := range os.Environ() {
		pair := strings.Split(v, "=")
		pc.Vars[pair[0]] = pair[1]
	}

	// read hostname
	pc.Hostname, err = os.Hostname()
	if err != nil {
		log.Printf("Unable to read hostname: %s", err)
	}

	// render template
	t, err := template.New("tpl").Parse(rootPage)
	if err != nil {
		log.Printf("Unable to parse template: %s", err)
	}
	err = t.Execute(w, pc)
	if err != nil {
		log.Printf("Unable to execute template: %s", err)
	}
}

func main() {
	r := mux.NewRouter()

	// register handlers
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/health", healthHandler)
	r.Handle("/metrics", promhttp.Handler())

	// log requests
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	log.Printf("Listening on %s\n", listen)
	log.Fatal(http.ListenAndServe(listen, loggedRouter))

}
