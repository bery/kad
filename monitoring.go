package main

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

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
