// Package server provides an HTTP server mode for comic conversion.
// It exposes REST API endpoints for submitting, monitoring, and
// configuring conversions.
package server

import (
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
)

// Job represents a single conversion job in the queue.
type Job struct {
	ID        string
	Opts      string // serialized comic.Options or path
	Status    string // "queued", "processing", "completed", "failed"
	Progress  chan string
	Result    error
	CreatedAt time.Time
	Cleanup   func() // called after job completes, may be nil

	doneCh chan struct{}
	sem    chan struct{} // nil for queued jobs, non-nil for processing
	mu     sync.Mutex
}

func (j *Job) Acquire() {
	j.sem = make(chan struct{}, 1)
	j.sem <- struct{}{}
}

func (j *Job) Release() {
	if j.sem != nil {
		<-j.sem
	}
}

// DoneCh returns a channel that is closed when the job completes.
func (j *Job) DoneCh() <-chan struct{} {
	return j.doneCh
}

// Done marks the job as completed (or failed) and closes the done channel.
func (j *Job) Done(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		j.Status = "failed"
		j.Result = err
	} else {
		j.Status = "completed"
	}
	close(j.doneCh)
}

// JobQueue manages async conversion jobs with concurrency limiting.
type JobQueue struct {
	maxConcurrent int
	sem           chan struct{}
	jobs          sync.Map
}

// NewJobQueue creates a job queue with the given concurrency limit.
func NewJobQueue(maxConcurrent int) *JobQueue {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &JobQueue{
		maxConcurrent: maxConcurrent,
		sem:           make(chan struct{}, maxConcurrent),
	}
}

// Submit adds a new job to the queue.
func (q *JobQueue) Submit(opts string) (*Job, error) {
	uid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:        uid.String(),
		Opts:      opts,
		Status:    "queued",
		Progress:  make(chan string, 100),
		CreatedAt: time.Now().UTC(),
		doneCh:    make(chan struct{}),
	}
	q.jobs.Store(job.ID, job)
	return job, nil
}

// Get retrieves a job by ID.
func (q *JobQueue) Get(id string) (*Job, bool) {
	v, ok := q.jobs.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Job), true
}

// NextPending returns the first queued job and atomically marks it as "processing".
// Returns nil if no queued job is found.
func (q *JobQueue) NextPending() *Job {
	var found *Job
	q.jobs.Range(func(key, value any) bool {
		job := value.(*Job)
		job.mu.Lock()
		if job.Status == "queued" {
			job.Status = "processing"
			found = job
		}
		job.mu.Unlock()
		return found == nil // stop iterating once found
	})
	return found
}

// SendProgress sends a progress message to the job's Progress channel without blocking.
// If the channel is full, the message is dropped.
func (j *Job) SendProgress(msg string) {
	select {
	case j.Progress <- msg:
	default:
	}
}
