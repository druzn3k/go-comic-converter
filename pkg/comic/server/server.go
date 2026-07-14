// Package server provides an HTTP server mode for comic conversion.
// It exposes REST API endpoints for submitting, monitoring, and
// configuring conversions.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/converter"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epub"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
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
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Server with the given config and parent context.
// It launches MaxConcurrent worker goroutines to process queued jobs.
func New(ctx context.Context, cfg Config) *Server {
	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		cfg:    cfg,
		queue:  NewJobQueue(cfg.MaxConcurrent),
		mux:    http.NewServeMux(),
		ctx:    ctx,
		cancel: cancel,
	}
	s.routes()
	for i := 0; i < cfg.MaxConcurrent; i++ {
		go s.runWorker(ctx)
	}
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

// Shutdown gracefully stops the server and cancels worker goroutines.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.server.Shutdown(ctx)
}

// runWorker processes queued jobs, acquiring a concurrency slot from the queue.
func (s *Server) runWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case s.queue.sem <- struct{}{}:
		}

		job := s.queue.NextPending()
		if job == nil {
			<-s.queue.sem
			continue
		}

		job.SendProgress("processing")

		opts := epuboptions.EPUBOptions{Input: job.Opts}
		// EPUB conversion handles the full pipeline
		err := epub.New(opts).Write(ctx)

		if job.Cleanup != nil {
			job.Cleanup()
		}
		job.Done(err)
		<-s.queue.sem
	}
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
