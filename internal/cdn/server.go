package cdn

import (
	"net/http"
	"strings"

	"github.com/vixa/cdn/internal/config"
	"github.com/vixa/cdn/internal/storage"
)

type Server struct {
	storage       *storage.Storage
	configManager *config.ConfigManager
}

func NewServer(storage *storage.Storage, cm *config.ConfigManager) *Server {
	return &Server{
		storage:       storage,
		configManager: cm,
	}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimPrefix(host, "https://")

		domainFolder, _, ok := s.configManager.GetDomainByFQDN(host)
		if !ok {
			s.serveNotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) < 2 {
			s.serveNotFound(w, r)
			return
		}

		category := parts[0]
		filename := parts[1]

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
			w.Header().Set("Access-Control-Max-Age", "86400")
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			s.serveNotFound(w, r)
			return
		}

		data, contentType, err := s.storage.GetFile(domainFolder, category, filename)
		if err != nil || data == nil {
			s.serveNotFound(w, r)
			return
		}

		etag := storage.GenerateETag(data)
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if r.Header.Get("Origin") != "" {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		}

		if r.Method != http.MethodHead {
			w.Write(data)
		}
	})
}

func (s *Server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.NotFound(w, r)
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}
