package main

import (
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
	prometheus.MustRegister(labsTotal)
	prometheus.MustRegister(labsActive)
	prometheus.MustRegister(packetsTotal)
	prometheus.MustRegister(failuresTotal)
	prometheus.MustRegister(latencyHistogram)
	prometheus.MustRegister(agentsActive)
}

// Record metrics for a lab start
func RecordLabStart() {
	labsTotal.WithLabelValues("started").Inc()
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
