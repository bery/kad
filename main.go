package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

type pageContent struct {
	Vars           map[string]*envVar
	Hostname       string
	Hits           int
	RedisHost      string
	RedisError     string
	Cmd            string
	ConfFile       string
	ConfigFilePath string
	Help           string
	Ready          bool
	Color          string
	Resources      Resources

	Request        *http.Request
	KubernetesHost string
}

type envVar struct {
	Name      string
	Value     string
	Dangerous bool
}

type Resources struct {
	Pods        []v1.Pod
	Services    []v1.Service
	Deployments []apps_v1.Deployment
	ReplicaSets []apps_v1.ReplicaSet
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
	Vars:           make(map[string]*envVar),
	Hits:           0,
	Cmd:            "",
	ConfigFilePath: configFile,
}

var (
	checkReady = true
	readyFile  = "/tmp/notready"

	exit      = make(chan error)
	exitDelay = 15 * time.Second
)

func init() {
	var err error

	colorFlag := flag.String("color", "", "background color")
	flag.String("user", "", "fake parameter")
	flag.Parse()

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

	// setup color
	if *colorFlag != "" {
		pc.Color = *colorFlag
	}
	if v := os.Getenv("COLOR"); v != "" && pc.Color == "" {
		pc.Color = v
	}
	if pc.Color == "" {
		pc.Color = "#ffffff"
	}

	// detect redis
	pc.RedisHost = os.Getenv("REDIS_SERVER")

}

func responseTime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		took := time.Since(start).Seconds()

		// set duration
		httpDuration.With(
			prometheus.Labels{
				"method":   r.Method,
				"endpoint": r.URL.String(),
			}).Observe(took)

		// increase cout
		httpRequestTotal.With(
			prometheus.Labels{
				"method":   r.Method,
				"endpoint": r.URL.String(),
			}).Add(1)
	})
}

func isReady() bool {
	_, err := os.Stat(readyFile)

	return err != nil
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

func main() {
	r := mux.NewRouter()

	adminRouter := mux.NewRouter()

	// register handlers
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/heavy", heavyHandler)
	r.HandleFunc("/slow", slowHandler)
	r.HandleFunc("/check/live", liveHandler)
	r.HandleFunc("/check/ready", readyHandler)
	r.HandleFunc("/kubernetes/delete/{type}/{name}", kubernetesDeleteHandler)
	r.Handle("/metrics", promhttp.Handler())

	adminRouter.HandleFunc("/action/terminate", terminateHandler)
	adminRouter.HandleFunc("/check/live", liveHandler)
	adminRouter.HandleFunc("/check/ready", readyHandler)

	// log requests
	loggedRouter := handlers.LoggingHandler(os.Stdout, responseTime(r))
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
