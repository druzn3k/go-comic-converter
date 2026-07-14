package server

import (
	"errors"
	"testing"
	"time"
)

func TestJobQueueSubmit(t *testing.T) {
	q := NewJobQueue(2)
	job, err := q.Submit("test")
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if job.ID == "" {
		t.Error("expected non-empty job ID")
	}
	if job.Status != "queued" {
		t.Errorf("expected 'queued', got %q", job.Status)
	}
	if job.Opts != "test" {
		t.Errorf("expected opts 'test', got %v", job.Opts)
	}
}

func TestJobQueueGet(t *testing.T) {
	q := NewJobQueue(2)
	submitted, _ := q.Submit("test")
	got, ok := q.Get(submitted.ID)
	if !ok {
		t.Fatal("expected to find job by ID")
	}
	if got.ID != submitted.ID {
		t.Errorf("expected ID %q, got %q", submitted.ID, got.ID)
	}
}

func TestJobQueueGetNotFound(t *testing.T) {
	q := NewJobQueue(2)
	_, ok := q.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent job")
	}
}

func TestJobQueueMaxConcurrent(t *testing.T) {
	q := NewJobQueue(1)
	// Submit first job (occupies the slot)
	j1, _ := q.Submit("job1")
	// Start processing j1 (acquires semaphore)
	j1.Acquire()
	defer j1.Release()

	// Second submit should still succeed (queue, not block)
	j2, err := q.Submit("job2")
	if err != nil {
		t.Fatalf("Submit job2 failed: %v", err)
	}
	if j2.Status != "queued" {
		t.Errorf("expected 'queued', got %q", j2.Status)
	}
}

func TestJobLifecycle(t *testing.T) {
	q := NewJobQueue(5)
	job, _ := q.Submit("test")

	// queued → processing
	job.Status = "processing"
	got, _ := q.Get(job.ID)
	if got.Status != "processing" {
		t.Errorf("expected 'processing', got %q", got.Status)
	}

	// processing → completed
	job.Status = "completed"
	got, _ = q.Get(job.ID)
	if got.Status != "completed" {
		t.Errorf("expected 'completed', got %q", got.Status)
	}
}

func TestJobQueueConcurrency(t *testing.T) {
	q := NewJobQueue(10)
	done := make(chan struct{})
	go func() {
		for range 50 {
			_, _ = q.Submit("job")
		}
		close(done)
	}()
	// Concurrent reads
	for range 50 {
		_, _ = q.Get("nonexistent")
	}
	<-done
}

func TestJobDoneChannel(t *testing.T) {
	q := NewJobQueue(2)
	job, _ := q.Submit("test")

	// Signal done
	job.Done(nil)

	select {
	case <-job.DoneCh():
		// expected
	case <-time.After(time.Second):
		t.Fatal("expected DoneCh to be closed")
	}

	if job.Status != "completed" {
		t.Errorf("expected 'completed', got %q", job.Status)
	}
}

func TestJobDoneWithError(t *testing.T) {
	q := NewJobQueue(2)
	job, _ := q.Submit("test")
	expectedErr := errors.New("conversion failed")

	job.Done(expectedErr)

	select {
	case <-job.DoneCh():
	default:
		t.Fatal("expected DoneCh to be closed")
	}

	if job.Status != "failed" {
		t.Errorf("expected 'failed', got %q", job.Status)
	}
	if job.Result != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, job.Result)
	}
}
