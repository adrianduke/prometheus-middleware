package prometheusmiddleware

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	dflBuckets = []float64{0.3, 1.0, 2.5, 5.0}
)

const (
	latencyName   = "http_request_duration_ms"
	startedName   = "http_request_started_total"
	completedName = "http_request_completed_total"
)

// Opts specifies options how to create new PrometheusMiddleware.
type Opts struct {
	// Buckets specifies an custom buckets to be used in request histograpm.
	Buckets []float64
}

// PrometheusMiddleware specifies the metrics that is going to be generated
type PrometheusMiddleware struct {
	latency   *prometheus.HistogramVec
	started   *prometheus.CounterVec
	completed *prometheus.CounterVec
}

// NewPrometheusMiddleware creates a new PrometheusMiddleware instance
func NewPrometheusMiddleware(opts Opts) *PrometheusMiddleware {
	var prometheusMiddleware PrometheusMiddleware

	buckets := opts.Buckets
	if len(buckets) == 0 {
		buckets = dflBuckets
	}

	prometheusMiddleware.latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    latencyName,
		Help:    "How long it took to process the request, partitioned by status code, method and HTTP path.",
		Buckets: buckets,
	},
		[]string{"code", "method", "path"},
	)

	if err := prometheus.Register(prometheusMiddleware.latency); err != nil {
		log.Println("prometheusMiddleware.latency was not registered:", err)
	}

	prometheusMiddleware.started = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: startedName,
		Help: "Total number of requests started on the http server.",
	},
		[]string{"method", "path"},
	)

	if err := prometheus.Register(prometheusMiddleware.started); err != nil {
		log.Println("prometheusMiddleware.started was not registered:", err)
	}

	prometheusMiddleware.completed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: completedName,
		Help: "Total number of requests completed on the http server.",
	},
		[]string{"code", "method", "path"},
	)

	if err := prometheus.Register(prometheusMiddleware.completed); err != nil {
		log.Println("prometheusMiddleware.completed was not registered:", err)
	}

	return &prometheusMiddleware
}

// InstrumentHandlerDuration is a middleware that wraps the http.Handler and it record
// how long the handler took to run, which path was called, and the status code.
// This method is going to be used with gorilla/mux.
func (p *PrometheusMiddleware) InstrumentHandlerDuration(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		method := sanitizeMethod(r.Method)
		p.started.WithLabelValues(method, path).Inc()

		begin := time.Now()

		delegate := &responseWriterDelegator{ResponseWriter: w}
		rw := delegate

		next.ServeHTTP(rw, r) // call original

		code := sanitizeCode(delegate.status)

		go p.completed.WithLabelValues(
			code,
			method,
			path,
		).Inc()

		go p.latency.WithLabelValues(
			code,
			method,
			path,
		).Observe(float64(time.Since(begin).Milliseconds()))
	})
}

type responseWriterDelegator struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

func (r *responseWriterDelegator) WriteHeader(code int) {
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseWriterDelegator) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

func sanitizeMethod(m string) string {
	return strings.ToLower(m)
}

func sanitizeCode(s int) string {
	return strconv.Itoa(s)
}
