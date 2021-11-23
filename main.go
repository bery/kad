package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.uber.org/zap"

	"github.com/go-redis/redis"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

type pageContent struct {
	Vars           map[string]*envVar
	Hostname       string
	Hits           int
	RedisHost      string
	RedisPath      string
	RedisError     string
	Cmd            string
	ConfFile       string
	ConfigFilePath string
	Help           string
	Ready          bool
	Color          string
	Resources      Resources
	Headers        []Header

	PageRefresh bool

	Request         *http.Request
	KubernetesError string
	KubernetesHost  string

	PersistentFiles    []string
	FailureProbability float64
}

type Header struct {
	Name  string
	Value string
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

var (
	listen      = ":5000"
	listenAdmin = ":5001"
	configFile  = "/etc/kad/config.yml"
	pc          = pageContent{
		Vars:           make(map[string]*envVar),
		Hits:           0,
		Cmd:            "",
		ConfigFilePath: configFile,
	}

	checkReady = true
	readyFile  = "/tmp/notready"

	exit      = make(chan error)
	exitDelay = 15 * time.Second
)

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

		// set random metrics
		rn := fmt.Sprintf("%d", rand.Intn(1000))
		randomMet.With(
			prometheus.Labels{
				"rn": rn,
			}).Add(1)

	})
}

func isReady() bool {
	_, err := os.Stat(readyFile)

	return err != nil
}

func redisPath() string {
	cluster := os.Getenv("CLUSTER")
	return fmt.Sprintf("hits-%s", cluster)
}

func addHit() error {
	if pc.RedisHost == "" {
		// Use pc variable
		pc.Hits = pc.Hits + 1

	} else {
		// use redis
		client := redis.NewClient(&redis.Options{
			Addr:         pc.RedisHost,
			DialTimeout:  300 * time.Millisecond,
			ReadTimeout:  300 * time.Millisecond,
			WriteTimeout: 300 * time.Millisecond,
		})

		defer client.Close()

		hits, err := client.Incr(redisPath()).Result()
		if err != nil {
			return fmt.Errorf("Unable to inc hits in redis: %s", err)
		}
		pc.Hits = int(hits)

	}

	pageHits.Observe(float64(pc.Hits))

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
	l, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer l.Sync()

	var rootCmd = &cobra.Command{
		Use: "kad",
		Run: func(cmd *cobra.Command, args []string) {
			var err error

			if cmd.Flag("fail").Value.String() == "true" {
				l.Info("Remove --fail command parameter to start properly")
				panic("fail option is enabled")
			}

			if fpr := cmd.Flag("failure-probability").Value.String(); fpr != "" {
				fp, err := strconv.ParseFloat(fpr, 64)
				if err != nil {
					log.Fatalf("Failed reading request failure probability (%s): %s", fpr, err)
				}
				if fp > 1 || fp < 0 {
					log.Fatal("Failure probabilty must be between 0 and 1")
				}

				pc.FailureProbability = fp
				l.Info("Request failure probablity set", zap.Float64("probabilty", fp))
			}

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
			pc.Color = cmd.Flag("color").Value.String()
			if v := os.Getenv("COLOR"); v != "" && pc.Color == "" {
				pc.Color = v
			}
			if pc.Color == "" {
				pc.Color = "#ffffff"
			}

			// detect redis
			pc.RedisHost = os.Getenv("REDIS_SERVER")

			r := mux.NewRouter()

			adminRouter := mux.NewRouter()

			// register handlers
			r.HandleFunc("/", rootHandler)
			r.HandleFunc("/heavy", heavyHandler)
			r.HandleFunc("/slow", slowHandler)
			r.HandleFunc("/hostname", hostnameHandler)
			r.HandleFunc("/check/live", liveHandler)
			r.HandleFunc("/check/ready", readyHandler)
			r.HandleFunc("/kubernetes/delete/{type}/{name}", kubernetesDeleteHandler)
			r.Handle("/metrics", promhttp.Handler())

			adminRouter.HandleFunc("/action/terminate", terminateHandler)
			adminRouter.HandleFunc("/check/live", liveHandler)
			adminRouter.HandleFunc("/check/ready", readyHandler)
			adminRouter.Handle("/metrics", promhttp.Handler())

			// log requests
			loggedRouter := handlers.LoggingHandler(os.Stdout, responseTime(r))
			loggedAdminRouter := handlers.LoggingHandler(os.Stdout, adminRouter)

			go func() {
				l.Info("Listening on client port", zap.String("socket", listen))
				if err := http.ListenAndServe(listen, loggedRouter); err != nil {
					log.Printf("Server failed with: %s", err)
				}
			}()

			go func() {
				l.Info("Listening on admin port", zap.String("socket", listenAdmin))
				if err := http.ListenAndServe(listenAdmin, loggedAdminRouter); err != nil {
					log.Printf("Admin server failed with: %s", err)
				}
			}()

			err = <-exit
			if err != nil {
				log.Printf("Terminating with error: %s", err)
			}

		},
	}
	rootCmd.PersistentFlags().String("color", "", "Background color for main page")
	rootCmd.PersistentFlags().Bool("fail", false, "Fail with non-zero exit code")
	rootCmd.PersistentFlags().Float64("failure-probability", 0, "Failure probability for user requests (applies only on /, must be between 0 and 1)")
	rootCmd.Execute()

}

func readPersistentFiles() []string {
	dataDir := os.Getenv("DATADIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	r := []string{}
	files, err := ioutil.ReadDir(dataDir)
	if err != nil {
		return r
	}

	for _, f := range files {
		if f.IsDir() {
			r = append(r, f.Name()+" (d)")
		} else {
			r = append(r, f.Name())
		}
	}

	return r
}
