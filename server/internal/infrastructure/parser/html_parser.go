package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"web-analyzer/internal/domain/model"
)

// Parser extracts a WebPage domain model from raw HTML.
type Parser struct{}

// New returns a ready-to-use Parser.
func New() *Parser { return &Parser{} }

// Parse converts rawHTML (fetched from pageURL) into a WebPage domain model.
// pageURL is the final URL after any HTTP redirects and is used when no <base> tag is present.
func (p *Parser) Parse(rawHTML string, pageURL *url.URL) (*model.WebPage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("html parser: %w", err)
	}

	page := &model.WebPage{
		HTML:     rawHTML,
		Headings: make(map[string]int),
	}

	// Resolve the effective base URL: <base href="..."> takes precedence over pageURL.
	page.BaseURL = resolveBase(doc, pageURL)

	// Extract <title>.
	page.Title = strings.TrimSpace(doc.Find("title").First().Text())

	// Count heading elements h1–h6.
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		if count := doc.Find(tag).Length(); count > 0 {
			page.Headings[tag] = count
		}
	}

	// Collect all <a href="..."> raw values (skip anchors without href).
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		raw, exists := s.Attr("href")
		if !exists || raw == "" {
			return
		}
		page.Links = append(page.Links, model.Link{Raw: raw})
	})

	return page, nil
}

// resolveBase returns the effective base URL for link resolution.
// The first valid <base href="..."> wins; otherwise pageURL is used as-is.
func resolveBase(doc *goquery.Document, pageURL *url.URL) *url.URL {
	var base *url.URL
	doc.Find("base[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return true
		}
		parsed, err := url.Parse(href)
		if err != nil {
			return true
		}
		// Resolve the base href against the page URL to handle relative <base> tags.
		resolved := pageURL.ResolveReference(parsed)
		base = resolved
		return false // use first valid <base>
	})
	if base != nil {
		return base
	}
	return pageURL
}
