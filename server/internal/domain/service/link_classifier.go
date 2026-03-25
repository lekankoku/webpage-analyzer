package service

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeURL resolves and normalises a raw href value against base.
// base is the effective base URL: the <base> tag href if present, otherwise the page URL.
//
// Returns (normalised, skip, err).
//   - skip=true: fragment-only link — must not be counted or checked.
//   - err!=nil: malformed URL — caller should mark the link inaccessible.
func NormalizeURL(raw string, base *url.URL) (string, bool, error) {
	if strings.HasPrefix(raw, "#") {
		return "", true, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("parse %q: %w", raw, err)
	}

	resolved := base.ResolveReference(parsed)

	resolved.Fragment = ""

	resolved.Scheme = strings.ToLower(resolved.Scheme)
	resolved.Host = strings.ToLower(resolved.Host)

	host := resolved.Hostname()
	port := resolved.Port()
	if (resolved.Scheme == "http" && port == "80") ||
		(resolved.Scheme == "https" && port == "443") {
		resolved.Host = host
	}

	if resolved.Scheme == "" || resolved.Host == "" {
		return "", false, fmt.Errorf("invalid URL %q: missing scheme or host", raw)
	}

	return resolved.String(), false, nil
}

// ClassifyLink returns true when link points to the same host as base (internal).
// The www. prefix is stripped before comparing, so www.example.com == example.com.
// Subdomains other than www are treated as external.
func ClassifyLink(link, base *url.URL) bool {
	return normalizeHost(link.Hostname()) == normalizeHost(base.Hostname())
}

func normalizeHost(h string) string {
	return strings.TrimPrefix(strings.ToLower(h), "www.")
}
