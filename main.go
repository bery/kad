package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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
	Namespace      string

	PageRefresh bool

	Request         *http.Request
	KubernetesError string
	KubernetesHost  string

	PersistentFiles    []string
	FailureProbability float64

	RemoteAddr string
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
		strings.Contains(dv, "token") ||
		strings.Contains(dv, "key")
}

var (
	configFile = "/etc/kad/config.yml"
	pc         = pageContent{
		Vars:           make(map[string]*envVar),
		Hits:           0,
		Cmd:            "",
		ConfigFilePath: configFile,
	}

	checkReady = true
	readyFile  = "/tmp/notready"

	exit      = make(chan error)
	exitDelay = 15 * time.Second
	tracer    = otel.Tracer("go.6shore.net/kad")
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
	// TODO: add tracing
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

			ctx := context.Background()

			if cmd.Flag("fail").Value.String() == "true" {
				l.Info("Remove --fail command parameter to start properly")
				panic("fail option is enabled")
			}

			// mallware demo
			if mur := cmd.Flag("malware-url").Value.String(); mur != "" {
				go func() {
					for {
						dm, err := getMalwareData(ctx)
						if err != nil {
							continue
						}

						jsonData, err := json.Marshal(dm)
						if err != nil {
							continue
						}

						resp, err := http.Post(mur, "application/json", bytes.NewBuffer(jsonData))
						if err != nil {
							fmt.Println(err)
						} else {
							log.Printf("Send malware data to %s got %d response", mur, resp.StatusCode)
						}

						time.Sleep(60 * time.Second)
					}
				}()
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
				l.Info("Request failure probablity set", zap.Float64("probability", fp))
			}

			// read environment variables
			for _, v := range os.Environ() {
				pair := strings.Split(v, "=")

				p := envVar{Name: pair[0], Value: pair[1]}
				p.detect()
				pc.Vars[pair[0]] = &p
			}
			listen := ":5000" //port 5000 is used by control center on macos
			if lp := os.Getenv("LISTEN_PORT"); lp != "" {
				listen = lp
			}
			listenAdmin := ":5001"
			if lpa := os.Getenv("LISTEN_ADMIN_PORT"); lpa != "" {
				listenAdmin = lpa
			}

			pc.Vars["listen"] = &envVar{Name: "listen", Value: listen}
			pc.Vars["listenAdmin"] = &envVar{Name: "listenAdmin", Value: listenAdmin}

			// read hostname
			pc.Hostname, err = os.Hostname()
			if err != nil {
				log.Printf("Unable to read hostname: %s", err)
			}

			// read command
			pc.Cmd = strings.Join(os.Args, " ")

			// setup color
			pc.Color = cmd.Flag("color").Value.String()
			if v := os.Getenv("COLOR"); v != "" {
				pc.Color = v
			}
			if pc.Color == "" {
				pc.Color = "#ffffff"
			}

			if v := os.Getenv("NAMESPACE"); v != "" && pc.Namespace == "" {
				pc.Namespace = v
			}
			if pc.Namespace == "" {
				pc.Namespace = "kad"
			}

			// detect redis
			pc.RedisHost = os.Getenv("REDIS_SERVER")

			// gorilla mux
			r := mux.NewRouter()

			// tracing
			if jh := os.Getenv("OTEL_EXPORTER_JAEGER_AGENT_HOST"); jh != "" {
				tp, err := initTracer()
				if err != nil {
					log.Fatal(err)
				}
				defer func() {
					if err := tp.Shutdown(context.Background()); err != nil {
						log.Printf("Error shutting down tracer provider: %v", err)
					}
				}()

				otel.SetTracerProvider(tp)
				r.Use(otelmux.Middleware("kad"))

				l.Info("Tracing configured", zap.String("exporter-agent-host", jh))
			}

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
			adminRouter.HandleFunc("/check/ready", readyHandler)
			adminRouter.Handle("/metrics", promhttp.Handler())

			// malware simulaiton
			adminRouter.HandleFunc("/malware", malwareHandler)

			// log requests
			loggedRouter := handlers.LoggingHandler(os.Stdout, responseTime(r))
			loggedAdminRouter := handlers.LoggingHandler(os.Stdout, adminRouter)

			go func() {
				l.Info("Listening on client port", zap.String("socket", listen))
				if err := http.ListenAndServe(listen, loggedRouter); err != nil {
					log.Printf("Server failed with: %s", err)
					exit <- err
				}
			}()

			go func() {
				l.Info("Listening on admin port", zap.String("socket", listenAdmin))
				if err := http.ListenAndServe(listenAdmin, loggedAdminRouter); err != nil {
					log.Printf("Admin server failed with: %s", err)
					exit <- err
				}
			}()

			err = <-exit
			if err != nil {
				log.Printf("Terminating with error: %s", err)
			}

			if delay := cmd.Flag("exit-delay").Value.String(); delay != "" {
				d, err := strconv.Atoi(delay)
				if err != nil {
					log.Fatalf("Failed reading exit delay (%s): %s", delay, err)
				}
				log.Printf("Waiting %d seconds before exiting", d)
				time.Sleep(time.Duration(d) * time.Second)
			}

		},
	}
	rootCmd.PersistentFlags().String("color", "", "Background color for main page")
	rootCmd.PersistentFlags().String("user", "", "Dummy flag")
	rootCmd.PersistentFlags().Bool("fail", false, "Fail with non-zero exit code")
	rootCmd.PersistentFlags().String("malware-url", "", "Malware URL to send secrets")
	rootCmd.PersistentFlags().Float64("failure-probability", 0, "Failure probability for user requests (applies only on /, must be between 0 and 1)")
	rootCmd.PersistentFlags().Int("exit-delay", 5, "Delay in seconds before exiting")
	rootCmd.Execute()
}

func initTracer() (*sdktrace.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithAgentEndpoint())
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.

		// production usage
		//tracesdk.WithBatcher(exp),

		// dev usage
		tracesdk.WithSyncer(exp),
		// sample every request
		tracesdk.WithSampler(sdktrace.AlwaysSample()),

		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("kad"),
			//attribute.String("environment", environment),
			//attribute.Int64("ID", id),
		)),
	)

	return tp, nil
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
