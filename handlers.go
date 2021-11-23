package main

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	err = addHit()
	if err != nil {
		log.Printf("Redis error: %e", err)
		pc.RedisError = err.Error()
	} else {
		pc.RedisError = ""
		pc.RedisPath = redisPath()
	}

	// check failure probability
	if pc.FailureProbability > 0 {
		if rf := rand.Float64(); rf <= pc.FailureProbability {
			//es := fmt.Sprintf("Failing due to probability set to %.2f, got %.2f. Retry your request.", pc.FailureProbability, rf)

			log.Printf("Request failure probabilty applied on request")

			//http.Error(w, es, http.StatusInternalServerError)
			return
		}
	}

	// check ready file
	pc.Ready = isReady()

	// store request
	pc.Request = r

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
	if err := readResources(); err != nil {
		pc.KubernetesError = err.Error()
	}

	pc.PersistentFiles = readPersistentFiles()

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

// make slow response
func slowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(3 * time.Second)
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
