package application

import (
	"context"
	"sync"
	"time"
)

// JobStatus represents the lifecycle state of an analysis job.
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// Job holds runtime state for a single analysis request.
// Result and Error are nil/empty until the job reaches a terminal state.
type Job struct {
	ID        string
	Status    JobStatus
	CreatedAt time.Time
}

// JobStore is a thread-safe in-memory store for analysis jobs and their SSE subscribers.
// Full implementation (Create, Get, SetDone, Subscribe, Publish) is added in Step 11.
type JobStore struct {
	mu          sync.RWMutex
	jobs        map[string]*Job
	subscribers map[string][]chan SSEEvent
}

// NewJobStore returns an initialised, empty JobStore.
func NewJobStore() *JobStore {
	return &JobStore{
		jobs:        make(map[string]*Job),
		subscribers: make(map[string][]chan SSEEvent),
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
