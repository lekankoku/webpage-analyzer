package application

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"web-analyzer/internal/domain/model"
)

// JobStatus represents the lifecycle state of an analysis job.
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// Job holds the runtime state for a single analysis request.
type Job struct {
	ID            string
	Status        JobStatus
	Result        *model.AnalysisResult // non-nil after SetDone
	Error         string                // non-empty after SetFailed
	CreatedAt     time.Time
	TerminalEvent *SSEEvent // set by Publish when a result/error event is delivered
}

// JobStore is a thread-safe in-memory store for jobs and their SSE subscribers.
type JobStore struct {
	mu          sync.RWMutex
	jobs        map[string]*Job
	subscribers map[string][]chan SSEEvent // jobID → active subscriber channels
}

// NewJobStore returns an initialised, empty JobStore.
func NewJobStore() *JobStore {
	return &JobStore{
		jobs:        make(map[string]*Job),
		subscribers: make(map[string][]chan SSEEvent),
	}
}

// Create generates a new UUID job in the pending state and stores it.
func (s *JobStore) Create() *Job {
	job := &Job{
		ID:        uuid.New().String(),
		Status:    JobPending,
		CreatedAt: time.Now(),
	}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
	return job
}

// Get retrieves a job by ID. Returns (nil, false) if not found.
func (s *JobStore) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

// SetRunning transitions a job to the running state.
func (s *JobStore) SetRunning(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobRunning
	}
}

// SetDone transitions a job to done and stores the result.
func (s *JobStore) SetDone(id string, result *model.AnalysisResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobDone
		j.Result = result
	}
}

// SetFailed transitions a job to failed and stores the error message.
func (s *JobStore) SetFailed(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobFailed
		j.Error = errMsg
	}
}

// Subscribe registers a new SSE subscriber for jobID and returns:
//   - ch: a read-only channel that will receive SSE events
//   - unsub: call to remove the subscriber (use when client disconnects early)
//
// If a terminal event (result/error) has already been published for this job,
// ch immediately contains that event and is closed — range over it and return.
func (s *JobStore) Subscribe(id string) (<-chan SSEEvent, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan SSEEvent, 50)

	job, ok := s.jobs[id]
	if !ok {
		// Unknown job — immediately close so the handler can return 404.
		close(ch)
		return ch, func() {}
	}

	// Replay stored terminal event for late subscribers.
	if job.TerminalEvent != nil {
		ch <- *job.TerminalEvent
		close(ch)
		return ch, func() {}
	}

	s.subscribers[id] = append(s.subscribers[id], ch)

	unsub := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		subs := s.subscribers[id]
		for i, sub := range subs {
			if sub == ch {
				s.subscribers[id] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
	return ch, unsub
}

// Publish sends event to all active subscribers of jobID.
// After a terminal event ("result" or "error"), all subscriber channels are
// closed so range-based readers exit cleanly.
func (s *JobStore) Publish(id string, event SSEEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.subscribers[id] {
		select {
		case ch <- event:
		default:
			// Subscriber channel full — drop to avoid blocking the publisher.
		}
	}

	if event.Type == "result" || event.Type == "error" {
		// Store terminal event so late subscribers can replay it.
		if job, ok := s.jobs[id]; ok {
			e := event
			job.TerminalEvent = &e
			if event.Type == "result" {
				job.Status = JobDone
				if r, ok := event.Data.(*model.AnalysisResult); ok {
					job.Result = r
				}
			} else {
				job.Status = JobFailed
			}
		}
		for _, ch := range s.subscribers[id] {
			close(ch)
		}
		delete(s.subscribers, id)
	}
}

// StartReaper launches a background goroutine that deletes jobs older than ttl.
// It stops when ctx is cancelled (e.g. on graceful shutdown).
func (s *JobStore) StartReaper(ctx context.Context, ttl time.Duration) {
	go func() {
		ticker := time.NewTicker(ttl / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.reap(ttl)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *JobStore) reap(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-ttl)
	for id, job := range s.jobs {
		if job.CreatedAt.Before(cutoff) {
			delete(s.jobs, id)
			delete(s.subscribers, id)
		}
	}
}
