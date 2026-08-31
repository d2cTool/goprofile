package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingOK struct{}

func (pingOK) Ping(context.Context) error { return nil }

type pingFail struct{}

func (pingFail) Ping(context.Context) error { return errors.New("down") }

func TestHealthOK(t *testing.T) {
	h := &HealthHandler{DB: pingOK{}, S3: pingOK{}, Kafka: pingOK{}}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("%v", body)
	}
}

func TestHealthFail(t *testing.T) {
	h := &HealthHandler{DB: pingOK{}, S3: pingFail{}, Kafka: pingOK{}}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}
