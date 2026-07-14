package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// handleConvert submits a new conversion job.
// POST /api/convert
// Accepts multipart/form-data with "file" and "options" fields,
// or application/json with "input" and "options" fields.
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	switch {
	case contentType == "application/json":
		s.handleConvertJSON(w, r)
	default:
		s.handleConvertMultipart(w, r)
	}
}

// handleConvertJSON handles JSON request body conversion.
func (s *Server) handleConvertJSON(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input   string                 `json:"input"`
		Options map[string]interface{} `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if !s.cfg.AllowLocalPaths {
		http.Error(w, `{"error":"local paths not allowed"}`, http.StatusForbidden)
		return
	}

	job, err := s.queue.Submit(req.Input)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID})
}

// handleConvertMultipart handles multipart file upload conversion.
func (s *Server) handleConvertMultipart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(500 << 20); err != nil { // 500MB max
		http.Error(w, `{"error":"file too large"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save uploaded file to temp dir
	tmpDir, err := os.MkdirTemp("", "comic-convert-*")
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}

	tmpPath := filepath.Join(tmpDir, header.Filename)
	dst, err := os.Create(tmpPath)
	if err != nil {
		http.Error(w, `{"error":"server error"}`, http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}
	dst.Close()

	job, err := s.queue.Submit(tmpPath)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	// Cleanup runs after the worker finishes processing the job
	job.mu.Lock()
	job.Cleanup = func() { os.RemoveAll(tmpDir) }
	job.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"job_id": job.ID})
}

// handleProgress streams progress events via SSE.
// GET /api/progress/{id}
func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	job, ok := s.queue.Get(jobID)
	if !ok {
		http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case progress, ok := <-job.Progress:
			if !ok {
				return
			}
			// SSE format: "data: {...}\n\n"
			if _, err := io.WriteString(w, "data: "+progress+"\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-job.DoneCh():
			// Send final event
			status := "completed"
			if job.Result != nil {
				status = "failed"
			}
			final := `{"type":"done","status":"` + status + `"}`
			io.WriteString(w, "data: "+final+"\n\n")
			flusher.Flush()
			return
		case <-r.Context().Done():
			return
		}
	}
}

// registerConvertRoutes adds conversion and progress routes.
func (s *Server) registerConvertRoutes() {
	s.mux.HandleFunc("POST /api/convert", s.handleConvert)
	s.mux.HandleFunc("GET /api/progress/{id}", s.handleProgress)
}
