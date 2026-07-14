package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	s := New(Config{MaxConcurrent: 2})
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestProfilesEndpoint(t *testing.T) {
	s := New(Config{MaxConcurrent: 2})
	req := httptest.NewRequest("GET", "/api/profiles", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var profiles []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&profiles); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(profiles) == 0 {
		t.Error("expected at least one profile")
	}

	// Check a known profile exists
	found := false
	for _, p := range profiles {
		if p["code"] == "SR" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'SR' profile in list")
	}
}

func TestNewServerConfig(t *testing.T) {
	s := New(Config{
		Addr:          ":8080",
		MaxConcurrent: 3,
	})
	if s.cfg.Addr != ":8080" {
		t.Errorf("expected ':8080', got %q", s.cfg.Addr)
	}
	if s.cfg.MaxConcurrent != 3 {
		t.Errorf("expected 3, got %d", s.cfg.MaxConcurrent)
	}
}

func TestServerStartShutdown(t *testing.T) {
	s := New(Config{Addr: ":0", MaxConcurrent: 1})

	// Initialize server before starting
	s.server = &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.ListenAndServe()
	}()

	// Give it a moment to start, then shut down
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	// Verify server stopped
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server to stop")
	}
}
