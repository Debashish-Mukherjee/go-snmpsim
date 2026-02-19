package main

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Prometheus metrics
	labsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snmpsim_labs_total",
			Help: "Total number of labs created",
		},
		[]string{"status"},
	)

	labsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "snmpsim_labs_active",
			Help: "Number of active (running) labs",
		},
		[]string{},
	)

	packetsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snmpsim_packets_total",
			Help: "Total SNMP packets processed",
		},
		[]string{"method", "lab_id"},
	)

	failuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snmpsim_failures_total",
			Help: "Total SNMP operation failures",
		},
		[]string{"reason", "lab_id"},
	)

	latencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "snmpsim_latency_seconds",
			Help:    "SNMP operation latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "lab_id"},
	)

	agentsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "snmpsim_agents_active",
			Help: "Number of active virtual agents",
		},
		[]string{"lab_id"},
	)
)

func initMetrics() {
	if err := prometheus.Register(labsTotal); err != nil {
		log.Printf("labsTotal already registered: %v", err)
	}
	if err := prometheus.Register(labsActive); err != nil {
		log.Printf("labsActive already registered: %v", err)
	}
	if err := prometheus.Register(packetsTotal); err != nil {
		log.Printf("packetsTotal already registered: %v", err)
	}
	if err := prometheus.Register(failuresTotal); err != nil {
		log.Printf("failuresTotal already registered: %v", err)
	}
	if err := prometheus.Register(latencyHistogram); err != nil {
		log.Printf("latencyHistogram already registered: %v", err)
	}
	if err := prometheus.Register(agentsActive); err != nil {
		log.Printf("agentsActive already registered: %v", err)
	}

	// Initialize baseline series so dashboards do not show empty panels.
	labsTotal.WithLabelValues("created").Add(0)
	labsActive.WithLabelValues().Set(0)
	packetsTotal.WithLabelValues("GET", "labs").Add(0)
	failuresTotal.WithLabelValues("none", "none").Add(0)
	agentsActive.WithLabelValues("global").Set(0)
}

// Record metrics for a lab creation
func RecordLabCreated() {
	labsTotal.WithLabelValues("created").Inc()
}

// Record metrics for a lab start
func RecordLabStart() {
	labsActive.WithLabelValues().Add(1)
}

// Record metrics for a lab stop
func RecordLabStop() {
	labsActive.WithLabelValues().Add(-1)
}

// Record SNMP packet processing
func RecordPacket(method, labID string) {
	packetsTotal.WithLabelValues(method, labID).Inc()
}

// Record SNMP failure
func RecordFailure(reason, labID string) {
	failuresTotal.WithLabelValues(reason, labID).Inc()
}

// Record operation latency
func RecordLatency(method, labID string, seconds float64) {
	latencyHistogram.WithLabelValues(method, labID).Observe(seconds)
}

// Update active agent count
func UpdateActiveAgents(labID string, count int) {
	agentsActive.WithLabelValues(labID).Set(float64(count))
}
