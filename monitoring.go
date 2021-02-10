package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
)

var pageHits = prometheus.NewSummary(prometheus.SummaryOpts{
	Name: "page_hits",
	Help: "Number of page visits",
})

var httpRequestTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of requests served by this instance",
	},
	[]string{"method", "endpoint"},
)

var httpDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "http_requests_duration",
		Help: "Time took to serve request",
	},
	[]string{"method", "endpoint"},
)

var randomMet = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "random",
		Help: "Random metrics with many labels",
	},
	[]string{"rn"},
)

func init() {
	err := prometheus.Register(pageHits)
	if err != nil {
		log.Printf("Unable to register pageHits: %s", err)
	}

	err = prometheus.Register(httpRequestTotal)
	if err != nil {
		log.Printf("Unable to register pageHits: %s", err)
	}

	err = prometheus.Register(httpDuration)
	if err != nil {
		log.Printf("Unable to register pageHits: %s", err)
	}

	err = prometheus.Register(randomMet)
	if err != nil {
		log.Printf("Unable to register pageHits: %s", err)
	}
}
