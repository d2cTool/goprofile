package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/d2cTool/goprofile/internal/config"
	"github.com/d2cTool/goprofile/internal/handlers"
	"github.com/d2cTool/goprofile/internal/middleware"
)

func NewRouter(cfg config.Config, avatars *handlers.AvatarHandler, health *handlers.HealthHandler, web *handlers.WebHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(cfg.CORSOrigins))

	r.Get("/health", health.ServeHTTP)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimit(cfg.RateLimitRPS))

		r.Route("/api/v1", func(r chi.Router) {
			r.Route("/avatars", func(r chi.Router) {
				r.With(middleware.UploadRateLimit(), middleware.UserID).Post("/", avatars.Upload)
				r.Get("/{avatar_id}", avatars.GetByID)
				r.Get("/{avatar_id}/metadata", avatars.Metadata)
				r.With(middleware.UserID).Delete("/{avatar_id}", avatars.DeleteByID)
			})
			r.Route("/users/{user_id}", func(r chi.Router) {
				r.Get("/avatar", avatars.GetByUser)
				r.Get("/avatars", avatars.ListByUser)
				r.With(middleware.UserID).Delete("/avatar", avatars.DeleteByUser)
			})
		})

		r.Route("/web", func(r chi.Router) {
			r.Get("/upload", web.UploadPage)
			r.With(middleware.UploadRateLimit()).Post("/upload", web.UploadForm)
			r.Get("/gallery/{user_id}", web.GalleryPage)
			r.Handle("/static/*", http.StripPrefix("/web/", http.HandlerFunc(web.Static)))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/upload", http.StatusFound)
	})

	return r
}
