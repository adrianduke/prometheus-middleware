package prometheusmiddleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Test_InstrumentGorillaMux(t *testing.T) {
	recorder := httptest.NewRecorder()

	middleware := NewPrometheusMiddleware(Opts{})

	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	r.Use(middleware.InstrumentHandlerDuration)

	ts := httptest.NewServer(r)
	defer ts.Close()

	req1, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Error(err)
	}
	req2, err := http.NewRequest("GET", ts.URL+"/metrics", nil)
	if err != nil {
		t.Error(err)
	}

	r.ServeHTTP(recorder, req1)
	r.ServeHTTP(recorder, req2)
	body := recorder.Body.String()
	if !strings.Contains(body, startedName) {
		t.Errorf("body does not contain request total entry '%s'", startedName)
	}
	if !strings.Contains(body, latencyName) {
		t.Errorf("body does not contain request duration entry '%s'", latencyName)
	}
	if !strings.Contains(body, completedName) {
		t.Errorf("body does not contain request total entry '%s'", completedName)
	}
}
