package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-redis/redis"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type pageContent struct {
	Vars       map[string]*envVar
	Hostname   string
	Hits       int
	RedisHost  string
	RedisError string
	Cmd        string
	ConfFile   string
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
var configFile = "/etc/kad/config.yml"
var pc = pageContent{
	Vars: make(map[string]*envVar),
	Hits: 0,
	Cmd:  "",
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

	// read command
	pc.Cmd = strings.Join(os.Args, " ")

	// detect redis
	pc.RedisHost = os.Getenv("REDIS_SERVER")

}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func liveHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func addHit() error {
	if pc.RedisHost == "" {
		// Use pc variable
		pc.Hits = pc.Hits + 1

	} else {
		// use redis
		client := redis.NewClient(&redis.Options{
			Addr: pc.RedisHost,
		})

		defer client.Close()

		hits, err := client.Incr("hits").Result()
		if err != nil {
			return fmt.Errorf("Unable to inc hits in redis: %s", err)
		}
		pc.Hits = int(hits)

	}

	pageHits.Set(float64(pc.Hits))

	return nil
}

func readConfig() {
	// read config file
	if content, err := ioutil.ReadFile(configFile); err != nil {
		log.Printf("Unable to read config file %s: %s", configFile, err)
	} else {
		pc.ConfFile = string(content)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	err = addHit()
	if err != nil {
		log.Printf("Redis error: %e", err)
		pc.RedisError = err.Error()
	} else {
		pc.RedisError = ""
	}

	// update config file context
	readConfig()

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
	r.HandleFunc("/check/live", liveHandler)
	r.HandleFunc("/check/ready", readyHandler)
	r.Handle("/metrics", promhttp.Handler())

	// log requests
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)

	log.Printf("Listening on %s\n", listen)
	log.Fatal(http.ListenAndServe(listen, loggedRouter))

}
