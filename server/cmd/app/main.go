package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"web-analyzer/internal/application"
	"web-analyzer/internal/config"
	infrafetcher "web-analyzer/internal/infrastructure/fetcher"
	"web-analyzer/internal/infrastructure/linkchecker"
	infraparser "web-analyzer/internal/infrastructure/parser"
	httphandler "web-analyzer/internal/interfaces/http"
)

func main() {
	cfg := config.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Boot the global link-checker worker pool.
	checker := linkchecker.New(linkchecker.CheckerConfig{
		MaxWorkers:    cfg.LinkCheckerMaxWorkers,
		JobBufferSize: cfg.LinkCheckerJobBuffer,
		Timeout:       cfg.LinkCheckerTimeout,
		Retries:       cfg.LinkCheckerRetries,
	})
	checker.Start(ctx)

	// sem is a counting semaphore — at most 10 concurrent analyses.
	sem := make(chan struct{}, cfg.MaxConcurrentJobs)

	store := application.NewJobStore()
	store.StartReaper(ctx, cfg.JobTTL)

	// Wire infrastructure implementations to application ports.
	uc := &application.AnalyzePageUseCase{
		Fetcher: infrafetcher.NewWithTimeout(cfg.PageFetchTimeout),
		Parser:  infraparser.New(),
		Checker: checker,
	}

	handler := httphandler.NewHandler(store, uc, sem, ctx)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("web-analyzer ready on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}

// Compile-time interface satisfaction checks.
var _ application.Fetcher = (*infrafetcher.Fetcher)(nil)
var _ application.Parser = (*infraparser.Parser)(nil)
var _ application.LinkChecker = (*linkchecker.GlobalLinkChecker)(nil)
