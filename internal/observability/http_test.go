package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHTTPRecordsMetrics(t *testing.T) {
	r := chi.NewRouter()
	r.Use(HTTP)
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	r.Handle("/metrics", MetricsHandler())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics %d", rec.Code)
	}
}

func TestSetupWithoutEndpoint(t *testing.T) {
	stop, err := Setup(t.Context(), "gophprofile-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := stop(t.Context()); err != nil {
		t.Fatal(err)
	}
}
