package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleConvertMultipart(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.cbz")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("test file content")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	writer.Close()

	s := New(context.Background(), Config{MaxConcurrent: 1, AllowLocalPaths: true})
	req := httptest.NewRequest("POST", "/api/convert", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["job_id"] == "" {
		t.Error("expected job_id in response")
	}
}

func TestHandleConvertJSON(t *testing.T) {
	jsonBody := `{"input": "/tmp/test.cbz", "options": {"profile": "SR"}}`

	s := New(context.Background(), Config{MaxConcurrent: 1, AllowLocalPaths: true})
	req := httptest.NewRequest("POST", "/api/convert", bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["job_id"] == "" {
		t.Error("expected job_id in response")
	}
}

func TestNextPending(t *testing.T) {
	q := NewJobQueue(4)
	jobA, _ := q.Submit("A")
	jobB, _ := q.Submit("B")
	jobC, _ := q.Submit("C")

	a := q.NextPending()
	b := q.NextPending()
	c := q.NextPending()

	if a == nil || b == nil || c == nil {
		t.Fatal("expected three non-nil jobs from NextPending")
	}

	ids := map[string]bool{a.ID: true, b.ID: true, c.ID: true}
	if len(ids) != 3 {
		t.Error("expected 3 distinct jobs from NextPending")
	}
	for _, id := range []string{jobA.ID, jobB.ID, jobC.ID} {
		if !ids[id] {
			t.Errorf("job %s not returned by NextPending", id)
		}
	}
}

func TestNextPendingEmpty(t *testing.T) {
	q := NewJobQueue(2)
	j := q.NextPending()
	if j != nil {
		t.Errorf("expected nil from empty queue, got %v", j)
	}
}

func TestNextPendingSkipsProcessing(t *testing.T) {
	q := NewJobQueue(2)
	q.Submit("test")

	first := q.NextPending()
	if first == nil {
		t.Fatal("expected first job from NextPending")
	}
	if first.Status != "processing" {
		t.Errorf("expected status 'processing', got %q", first.Status)
	}

	second := q.NextPending()
	if second != nil {
		t.Errorf("expected nil on second NextPending with no queued jobs, got %v", second)
	}
}

func TestSendProgress(t *testing.T) {
	q := NewJobQueue(2)
	job, _ := q.Submit("test")

	job.SendProgress("test message")

	select {
	case msg := <-job.Progress:
		if msg != "test message" {
			t.Errorf("expected 'test message', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for progress message")
	}
}

func TestSendProgressBlocking(t *testing.T) {
	q := NewJobQueue(2)
	job, _ := q.Submit("test")

	// Fill the progress channel to capacity (100)
	for range 100 {
		job.Progress <- "msg"
	}

	if len(job.Progress) != 100 {
		t.Fatalf("expected channel full (100), got %d", len(job.Progress))
	}

	// SendProgress must be non-blocking even when channel is full
	done := make(chan bool, 1)
	go func() {
		job.SendProgress("overflow") // should be dropped, not block
		done <- true
	}()

	select {
	case <-done:
		// success — SendProgress returned without blocking
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SendProgress blocked on full channel")
	}

	// Verify the overflow message was dropped (channel still full)
	if len(job.Progress) != 100 {
		t.Errorf("expected channel still full (100) after overflow, got %d", len(job.Progress))
	}
}

func TestJobDone(t *testing.T) {
	q := NewJobQueue(2)
	job, _ := q.Submit("test")

	job.Done(nil)

	if job.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", job.Status)
	}

	select {
	case <-job.DoneCh():
		// channel closed as expected
	default:
		t.Error("expected DoneCh to be closed after Done(nil)")
	}
}

func TestJobDoneError(t *testing.T) {
	q := NewJobQueue(2)
	job, _ := q.Submit("test")

	testErr := errors.New("conversion failed")
	job.Done(testErr)

	if job.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", job.Status)
	}
	if job.Result != testErr {
		t.Errorf("expected Result %v, got %v", testErr, job.Result)
	}

	select {
	case <-job.DoneCh():
		// channel closed as expected
	default:
		t.Error("expected DoneCh to be closed after Done(error)")
	}
}

func TestConcurrencyLimit(t *testing.T) {
	q := NewJobQueue(2)

	q.Submit("A")
	q.Submit("B")
	q.Submit("C")

	// Acquire the queue's semaphore to capacity
	q.sem <- struct{}{}
	q.sem <- struct{}{}

	if len(q.sem) != cap(q.sem) {
		t.Fatalf("expected sem full (len=%d, cap=%d)", len(q.sem), cap(q.sem))
	}

	// A third acquisition must block
	acquired := make(chan bool, 1)
	go func() {
		q.sem <- struct{}{}
		acquired <- true
	}()

	select {
	case <-acquired:
		t.Error("should not have acquired third sem slot immediately")
	case <-time.After(50 * time.Millisecond):
		// expected — third acquisition is blocked
	}

	// Release one slot; third acquisition should now succeed
	<-q.sem

	select {
	case <-acquired:
		// now succeeded after release
	case <-time.After(time.Second):
		t.Fatal("third acquisition should have succeeded after releasing a slot")
	}

	// Cleanup remaining slots
	<-q.sem
	<-q.sem
}
