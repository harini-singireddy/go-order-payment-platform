package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics is a middleware that records request count and latency,
// labeled by service name, method, path, and status code.
func Metrics(service string) func(http.Handler) http.Handler {
	reqs := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "opp",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests.",
	}, []string{"service", "method", "path", "status"})

	dur := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "opp",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"service", "method", "path", "status"})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r)
			elapsed := time.Since(start).Seconds()
			status := strconv.Itoa(rec.status)

			reqs.WithLabelValues(service, r.Method, r.URL.Path, status).Inc()
			dur.WithLabelValues(service, r.Method, r.URL.Path, status).Observe(elapsed)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
