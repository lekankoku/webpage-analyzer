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
	infrafetcher "web-analyzer/internal/infrastructure/fetcher"
	"web-analyzer/internal/infrastructure/linkchecker"
	infraparser "web-analyzer/internal/infrastructure/parser"
	httphandler "web-analyzer/internal/interfaces/http"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Boot the global link-checker worker pool.
	checker := linkchecker.New(linkchecker.CheckerConfig{
		MaxWorkers:    100,
		JobBufferSize: 500,
		Timeout:       10 * time.Second,
		Retries:       2,
	})
	checker.Start(ctx)

	// sem is a counting semaphore — at most 10 concurrent analyses.
	sem := make(chan struct{}, 10)

	store := application.NewJobStore()
	store.StartReaper(ctx, time.Hour)

	// Wire infrastructure implementations to application ports.
	uc := &application.AnalyzePageUseCase{
		Fetcher: &fetcherAdapter{f: infrafetcher.New()},
		Parser:  infraparser.New(),
		Checker: checker,
	}

	handler := httphandler.NewHandler(store, uc, sem, ctx)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

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

// fetcherAdapter bridges infrastructure/fetcher to the application.Fetcher port.
// The infrastructure Fetcher returns *fetcher.Result; the application port expects
// *application.FetchResult — identical fields, different package types.
type fetcherAdapter struct {
	f *infrafetcher.Fetcher
}

func (a *fetcherAdapter) Fetch(ctx context.Context, rawURL string) (*application.FetchResult, error) {
	r, err := a.f.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return &application.FetchResult{HTML: r.HTML, FinalURL: r.FinalURL}, nil
}

// Compile-time interface satisfaction checks.
var _ application.Fetcher = (*fetcherAdapter)(nil)
var _ application.Parser = (*infraparser.Parser)(nil)
var _ application.LinkChecker = (*linkchecker.GlobalLinkChecker)(nil)
