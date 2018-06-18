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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type pageContent struct {
	Vars      map[string]*envVar
	Hostname  string
	Hits      int
	RedisHost string
}

type envVar struct {
	Name      string
	Value     string
	Dangerous bool
}

func (e *envVar) detect() {
	dv := strings.ToLower(e.Name + e.Value)

	e.Dangerous = strings.Contains(dv, "pass") ||
		strings.Contains(dv, "user") ||
		strings.Contains(dv, "key")
}

var listen = ":5000"
var pc = pageContent{
	Vars: make(map[string]*envVar),
	Hits: 0,
}

func init() {
	var err error
	// read environment variables
	for _, v := range os.Environ() {
		pair := strings.Split(v, "=")

		p := envVar{Name: pair[0], Value: pair[1]}
		p.detect()
		pc.Vars[pair[0]] = &p
	}

	// read hostname
	pc.Hostname, err = os.Hostname()
	if err != nil {
		log.Printf("Unable to read hostname: %s", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	pc.Hits = pc.Hits + 1
	pageHits.Set(float64(pc.Hits))

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
