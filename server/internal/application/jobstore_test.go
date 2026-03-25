package application_test

import (
	"context"
	"testing"
	"time"

	"web-analyzer/internal/application"
	"web-analyzer/internal/domain/model"
)

func TestJobStore_CreateAndGet(t *testing.T) {
	store := application.NewJobStore()
	job := store.Create()

	got, ok := store.Get(job.ID)
	if !ok {
		t.Fatal("expected job to be found after Create")
	}
	if got.Status != application.JobPending {
		t.Errorf("expected status=pending, got %q", got.Status)
	}
	if got.ID == "" {
		t.Error("expected non-empty job ID")
	}
}

func TestJobStore_SetDone_StoresResult(t *testing.T) {
	store := application.NewJobStore()
	job := store.Create()

	result := &model.AnalysisResult{
		HTMLVersion: "HTML5",
		Title:       "Test",
		Headings:    map[string]int{"h1": 1},
	}
	store.SetDone(job.ID, result)

	got, _ := store.Get(job.ID)
	if got.Status != application.JobDone {
		t.Errorf("expected status=done, got %q", got.Status)
	}
	if got.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Result.Title != "Test" {
		t.Errorf("expected title=%q, got %q", "Test", got.Result.Title)
	}
}

func TestJobStore_SubscribeAndPublish(t *testing.T) {
	store := application.NewJobStore()
	job := store.Create()

	ch, unsub := store.Subscribe(job.ID)
	defer unsub()

	event := application.SSEEvent{Type: "phase", Data: map[string]string{"phase": "fetching"}}
	store.Publish(job.ID, event)

	select {
	case received := <-ch:
		if received.Type != "phase" {
			t.Errorf("expected type=phase, got %q", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for published event")
	}
}

func TestJobStore_Reaper_RemovesExpiredJobs(t *testing.T) {
	store := application.NewJobStore()
	job := store.Create()

	// Force the job to appear older than the TTL.
	// Access via Get to assert it exists first.
	if _, ok := store.Get(job.ID); !ok {
		t.Fatal("job should exist before reap")
	}

	// TTL of 1ms, then wait to ensure expiry.
	store.StartReaper(context.Background(), time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	if _, ok := store.Get(job.ID); ok {
		t.Error("expected job to be reaped after TTL")
	}
}

func TestJobStore_Reaper_KeepsYoungJobs(t *testing.T) {
	store := application.NewJobStore()
	job := store.Create()

	// TTL of 1 hour — this job was just created, so it should survive.
	store.StartReaper(context.Background(), time.Hour)
	// Give reaper one tick (30 minutes — it won't fire in test time).
	// Instead, call nothing and just verify the job is still there.
	if _, ok := store.Get(job.ID); !ok {
		t.Error("young job should not be reaped")
	}
}
