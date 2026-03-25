package service_test

import (
	"testing"

	"web-analyzer/internal/domain/service"
)

func TestDetectHTMLVersion_HTML5_Lowercase(t *testing.T) {
	html := `<!DOCTYPE html><html><head></head><body></body></html>`
	got := service.DetectHTMLVersion(html)
	if got != "HTML5" {
		t.Errorf("got %q, want %q", got, "HTML5")
	}
}

func TestDetectHTMLVersion_HTML5_Uppercase(t *testing.T) {
	html := `<!doctype HTML><html><head></head><body></body></html>`
	got := service.DetectHTMLVersion(html)
	if got != "HTML5" {
		t.Errorf("got %q, want %q", got, "HTML5")
	}
}

func TestDetectHTMLVersion_HTML401Strict(t *testing.T) {
	html := `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01 Strict//EN"
	"http://www.w3.org/TR/html4/strict.dtd"><html></html>`
	got := service.DetectHTMLVersion(html)
	if got != "HTML 4.01 Strict" {
		t.Errorf("got %q, want %q", got, "HTML 4.01 Strict")
	}
}

func TestDetectHTMLVersion_XHTML10(t *testing.T) {
	html := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"
	"http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"><html></html>`
	got := service.DetectHTMLVersion(html)
	if got != "XHTML 1.0" {
		t.Errorf("got %q, want %q", got, "XHTML 1.0")
	}
}

func TestDetectHTMLVersion_NoDoctype_Unknown(t *testing.T) {
	html := `<html><head></head><body><p>No doctype</p></body></html>`
	got := service.DetectHTMLVersion(html)
	if got != "Unknown" {
		t.Errorf("got %q, want %q", got, "Unknown")
	}
}
