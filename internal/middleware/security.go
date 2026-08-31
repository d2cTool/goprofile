package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

func CORS(origins []string) func(http.Handler) http.Handler {
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-User-ID"},
		ExposedHeaders:   []string{"ETag", "Cache-Control", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}

func RateLimit(rps int) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 20
	}
	return httprate.Limit(
		rps,
		time.Second,
		httprate.WithKeyFuncs(httprate.KeyByIP, httprate.KeyByEndpoint),
	)
}

func UploadRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		10,
		time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if id := r.Header.Get(HeaderUserID); id != "" {
				return id, nil
			}
			return httprate.KeyByIP(r)
		}),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "Too many upload requests",
			})
		}),
	)
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
