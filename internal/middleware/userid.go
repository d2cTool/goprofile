package middleware

import (
	"context"
	"net/http"

	"github.com/d2cTool/goprofile/internal/domain"
)

type ctxKey int

const userIDKey ctxKey = 1

const HeaderUserID = "X-User-ID"

func UserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(HeaderUserID)
		if err := domain.ValidateUserID(userID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "Invalid user id",
				"details": "X-User-ID header is required",
			})
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
	})
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(mustJSON(v))
}
