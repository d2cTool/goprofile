package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserIDMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserIDFrom(r.Context()) != "alice" {
			t.Fatalf("user %q", UserIDFrom(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := UserID(ok)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing header: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderUserID, "alice")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid header: %d", rec.Code)
	}
}
