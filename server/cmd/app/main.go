package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"web-analyzer/internal/application"
	"web-analyzer/internal/infrastructure/linkchecker"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	checker := linkchecker.New(linkchecker.CheckerConfig{
		MaxWorkers:    100,
		JobBufferSize: 500,
		Timeout:       10 * time.Second,
		Retries:       2,
	})
	checker.Start(ctx)

	// sem is a buffered channel used as a counting semaphore — at most 10 concurrent analyses.
	sem := make(chan struct{}, 10)
	_ = sem // handlers registered in Step 12

	store := application.NewJobStore()
	store.StartReaper(ctx, time.Hour)

	mux := http.NewServeMux()
	// HTTP handlers registered in Step 12.

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Println("web-analyzer ready on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}
