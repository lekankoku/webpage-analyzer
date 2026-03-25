package service_test

import (
	"net/url"
	"testing"

	"web-analyzer/internal/domain/service"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("mustParse(%q): %v", raw, err)
	}
	return u
}

// ── NormalizeURL tests (Step 3) ──────────────────────────────────────────────

func TestNormalizeURL_RelativePath(t *testing.T) {
	base := mustParse(t, "https://example.com")
	got, skip, err := service.NormalizeURL("/about", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}
	if got != "https://example.com/about" {
		t.Errorf("got %q, want %q", got, "https://example.com/about")
	}
}

func TestNormalizeURL_FragmentStripped(t *testing.T) {
	base := mustParse(t, "https://example.com")
	got, skip, err := service.NormalizeURL("https://example.com/page#section", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}
	if got != "https://example.com/page" {
		t.Errorf("got %q, want %q", got, "https://example.com/page")
	}
}

func TestNormalizeURL_FragmentOnly_Skipped(t *testing.T) {
	base := mustParse(t, "https://example.com")
	got, skip, err := service.NormalizeURL("#anchor", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Fatal("expected skip=true for fragment-only link")
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestNormalizeURL_SchemeNormalized(t *testing.T) {
	base := mustParse(t, "https://example.com")
	got, skip, err := service.NormalizeURL("HTTP://EXAMPLE.COM/path", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}
	if got != "http://example.com/path" {
		t.Errorf("got %q, want %q", got, "http://example.com/path")
	}
}

func TestNormalizeURL_DefaultPortStripped(t *testing.T) {
	base := mustParse(t, "https://example.com")
	got, skip, err := service.NormalizeURL("https://example.com:443/path", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}
	if got != "https://example.com/path" {
		t.Errorf("got %q, want %q", got, "https://example.com/path")
	}
}

func TestNormalizeURL_BaseTagRespected(t *testing.T) {
	// base here is the resolved <base> tag href, not the page URL
	base := mustParse(t, "https://other.com")
	got, skip, err := service.NormalizeURL("/foo", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false")
	}
	if got != "https://other.com/foo" {
		t.Errorf("got %q, want %q", got, "https://other.com/foo")
	}
}

func TestNormalizeURL_InvalidURL_ReturnsError(t *testing.T) {
	base := mustParse(t, "https://example.com")
	// A URL with a null byte is rejected by url.Parse
	got, skip, err := service.NormalizeURL("http://exam\x00ple.com/", base)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	if skip {
		t.Fatal("expected skip=false for invalid URL (error path)")
	}
	if got != "" {
		t.Errorf("expected empty string on error, got %q", got)
	}
}

// ── ClassifyLink tests (Step 4) ──────────────────────────────────────────────

func TestClassifyLink_SameHost_IsInternal(t *testing.T) {
	base := mustParse(t, "https://example.com")
	link := mustParse(t, "https://example.com/page")
	if !service.ClassifyLink(link, base) {
		t.Error("same host should be internal")
	}
}

func TestClassifyLink_DifferentHost_IsExternal(t *testing.T) {
	base := mustParse(t, "https://example.com")
	link := mustParse(t, "https://other.com/page")
	if service.ClassifyLink(link, base) {
		t.Error("different host should be external")
	}
}

func TestClassifyLink_Subdomain_IsExternal(t *testing.T) {
	base := mustParse(t, "https://example.com")
	link := mustParse(t, "https://sub.example.com/page")
	if service.ClassifyLink(link, base) {
		t.Error("subdomain should be external")
	}
}

func TestClassifyLink_WwwVariant_IsInternal(t *testing.T) {
	base := mustParse(t, "https://example.com")
	link := mustParse(t, "https://www.example.com/page")
	if !service.ClassifyLink(link, base) {
		t.Error("www variant of same host should be internal")
	}
}
