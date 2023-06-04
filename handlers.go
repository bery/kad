package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	ctx, span := tracer.Start(r.Context(), "heavy")
	defer span.End()

	err = addHit()
	if err != nil {
		log.Printf("Redis error: %e", err)
		pc.RedisError = err.Error()
		span.RecordError(err)
	} else {
		pc.RedisError = ""
		pc.RedisPath = redisPath()
	}

	// check failure probability
	if pc.FailureProbability > 0 {
		if rf := rand.Float64(); rf <= pc.FailureProbability {
			es := fmt.Sprintf("Failing due to probability set to %.2f, got %.2f. Retry your request.", pc.FailureProbability, rf)
			log.Printf("Request failure probabilty applied on request")
			http.Error(w, es, http.StatusBadGateway)
			return
		}
	}

	// check ready file
	pc.Ready = isReady()

	// store request
	pc.Request = r

	// read remote addr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		pc.RemoteAddr = host
	}

	// headers
	pc.Headers = []Header{}
	for k, v := range r.Header {
		va := strings.Join(v, " ")
		ha := Header{Name: k, Value: va}
		pc.Headers = append(pc.Headers, ha)
	}

	// update config file context
	readConfig()

	// read resources from kubernetes
	if err := readResources(ctx); err != nil {
		pc.KubernetesError = err.Error()
	}

	pc.PersistentFiles = readPersistentFiles()

	// render template
	t, err := template.New("tpl").Parse(rootPage)
	if err != nil {
		span.RecordError(err)
		log.Printf("Unable to parse template: %s", err)
	}
	err = t.Execute(w, pc)
	if err != nil {
		span.RecordError(err)
		log.Printf("Unable to execute template: %s", err)
	}
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	if !isReady() {
		http.Error(w, fmt.Sprintf("NOT ready, %s exists", readyFile), http.StatusNotFound)
	} else if checkReady {
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

// make heavy computation
func heavyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "heavy")
	defer span.End()

	fmt.Fprintf(w, "Starting heavy load")

	go func() {
		_, span := tracer.Start(ctx, "heavy-goroutines")
		defer span.End()

		f, err := os.Open(os.DevNull)
		if err != nil {
			span.RecordError(err)
			panic(err)
		}
		defer f.Close()

		n := runtime.NumCPU()
		runtime.GOMAXPROCS(n)
		span.SetAttributes(
			attribute.Int("goroutines", n),
		)

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

// make slow response
func slowHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "sleep")
	time.Sleep(3 * time.Second)
	defer span.End()

	fmt.Fprintf(w, "Executed slow load\n")
}

// return hostname
func hostnameHandler(w http.ResponseWriter, r *http.Request) {
	hn, err := os.Hostname()

	if c := os.Getenv("CLUSTER"); c != "" {
		hn = fmt.Sprintf("%s/%s", c, hn)
	}
	if c := os.Getenv("VES_IO_SITENAME"); c != "" {
		hn = fmt.Sprintf("%s/%s", c, hn)
	}
	if c := os.Getenv("VES_IO_REGION"); c != "" {
		hn = fmt.Sprintf("%s/%s", c, hn)
	}

	if err != nil {
		fmt.Fprintf(w, "Failed reading hostname: %s", err)
	} else {
		fmt.Fprint(w, hn)
	}
}

type MalwareData struct {
	Secret map[string]string
	Env    map[string]string
}

func readSecrets(ictx context.Context) (map[string]string, error) {
	ctx, span := tracer.Start(ictx, "readSecrets")
	defer span.End()

	cs, err := getClientset()
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// list pods
	l, err := cs.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	r := map[string]string{}
	for _, s := range l.Items {
		bn, err := json.Marshal(s)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		r[fmt.Sprintf("%s/%s", s.GetNamespace(), s.GetName())] = string(bn)
	}

	return r, nil
}

func getMalwareData(ctx context.Context) (*MalwareData, error) {
	md := &MalwareData{
		Secret: map[string]string{},
		Env:    map[string]string{},
	}

	// read env
	for _, v := range os.Environ() {
		pair := strings.Split(v, "=")
		md.Env[pair[0]] = pair[1]
	}

	// read secrets
	s, err := readSecrets(ctx)
	if err != nil {
		log.Printf("Unable to list secrets: %s", err)
	} else {
		md.Secret = s
	}

	return md, nil
}

func malwareHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "malware")
	defer span.End()

	dm, err := getMalwareData(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	d, err := json.Marshal(dm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	fmt.Fprint(w, string(d))
}
