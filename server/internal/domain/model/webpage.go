package model

import "net/url"

// WebPage is the parsed domain representation of a fetched HTML document.
// Headings maps heading tag names ("h1"…"h6") to their occurrence counts.
type WebPage struct {
	BaseURL  *url.URL
	HTML     string
	Title    string
	Headings map[string]int
	Links    []Link
}
