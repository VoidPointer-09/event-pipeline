package metrics

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	Processed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "events_processed_total",
		Help: "Total number of events processed",
	})
	DLQCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dlq_messages_total",
		Help: "Total number of messages sent to DLQ",
	})
	DBLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "db_latency_seconds",
		Help:    "Database operation latency",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(Processed, DLQCount, DBLatency)
}

// Serve starts a /metrics endpoint on the given addr (e.g., :2112)
func Serve() {
	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = ":2112"
	}
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(addr, nil)
}
