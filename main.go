package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

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
var listenAdmin = ":5001"
var configFile = "/etc/kad/config.yml"
var pc = pageContent{
	Vars: make(map[string]*envVar),
	Hits: 0,
	Cmd:  "",
}

var checkReady = true

var exit = make(chan error)
var exitDelay = 15 * time.Second

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
	if checkReady {
		fmt.Fprintf(w, "OK")
	} else {
		http.Error(w, "NOT ready", http.StatusNotFound)
	}
}

func liveHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func terminateHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Terminating on request from %s", r.RemoteAddr)
	log.Printf("Reporting this instance as NOT ready")
	checkReady = false
	fmt.Fprintf(w, "OK")

	go func() {
		time.Sleep(exitDelay)

		exit <- nil
	}()
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

// make heavu computation
func heavyHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Starting heavy load")

	go func() {
		f, err := os.Open(os.DevNull)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		n := runtime.NumCPU()
		runtime.GOMAXPROCS(n)

		for i := 0; i < n; i++ {
			go func() {
				for {
					fmt.Fprintf(f, ".")
				}
			}()
		}
	}()

	time.Sleep(3 * time.Second)
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

	adminRouter := mux.NewRouter()

	// register handlers
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/heavy", heavyHandler)
	r.HandleFunc("/check/live", liveHandler)
	r.HandleFunc("/check/ready", readyHandler)
	r.Handle("/metrics", promhttp.Handler())

	adminRouter.HandleFunc("/action/terminate", terminateHandler)

	// log requests
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	loggedAdminRouter := handlers.LoggingHandler(os.Stdout, adminRouter)

	go func() {
		log.Printf("Listening on %s\n", listen)
		if err := http.ListenAndServe(listen, loggedRouter); err != nil {
			log.Printf("Server failed with: %s", err)
		}
	}()

	go func() {
		log.Printf("Listening admin interface on %s\n", listenAdmin)
		if err := http.ListenAndServe(listenAdmin, loggedAdminRouter); err != nil {
			log.Printf("Admin server failed with: %s", err)
		}
	}()

	select {
	case err := <-exit:
		if err != nil {
			log.Printf("Terminating with error: %s", err)
		}

	}

}
