// Package server provides an HTTP server mode for comic conversion.
// It exposes REST API endpoints for submitting, monitoring, and
// configuring conversions.
package server
import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/converter"
)

// Config holds server configuration.
type Config struct {
	Addr            string        // listen address, e.g., ":8080"
	MaxConcurrent   int           // max simultaneous conversions
	AllowLocalPaths bool          // allow local file paths as input
	ShutdownTimeout time.Duration // graceful shutdown timeout
}

// Server is the HTTP server for comic conversion.
type Server struct {
	cfg    Config
	queue  *JobQueue
	server *http.Server
	mux    *http.ServeMux
}

// New creates a new Server with the given config.
func New(cfg Config) *Server {
	s := &Server{
		cfg:   cfg,
		queue: NewJobQueue(cfg.MaxConcurrent),
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes registers all API endpoints.
func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/profiles", s.handleProfiles)
	s.registerConvertRoutes()
}

// Start begins listening and serving requests.
func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.mux,
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleHealth returns the server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleProfiles returns the list of available device profiles.
func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := converter.NewProfiles()
	profileList := make([]map[string]interface{}, 0)
	for _, p := range profiles {
		profileList = append(profileList, map[string]interface{}{
			"code":        p.Code,
			"description": p.Description,
			"width":       p.Width,
			"height":      p.Height,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profileList)
}

// serveHTTP wraps the mux for potential middleware.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
