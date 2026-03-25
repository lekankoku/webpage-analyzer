package service

import (
	"strings"
)

// doctypeRules maps ordered substring markers to their canonical HTML version names.
// Ordering matters: more specific patterns must come before more general ones.
var doctypeRules = []struct {
	marker  string
	version string
}{
	{"HTML 4.01 Strict", "HTML 4.01 Strict"},
	{"HTML 4.01 Transitional", "HTML 4.01 Transitional"},
	{"XHTML 1.1", "XHTML 1.1"},
	{"XHTML 1.0", "XHTML 1.0"},
}

// DetectHTMLVersion extracts the HTML version from the raw document string by
// inspecting the doctype declaration in the first 512 bytes.
//
// Detection table (case-insensitive):
//   - "<!DOCTYPE html>" with no public identifier → HTML5
//   - "HTML 4.01 Strict" in doctype              → HTML 4.01 Strict
//   - "HTML 4.01 Transitional" in doctype         → HTML 4.01 Transitional
//   - "XHTML 1.0" in doctype                      → XHTML 1.0
//   - "XHTML 1.1" in doctype                      → XHTML 1.1
//   - No doctype found                             → Unknown
func DetectHTMLVersion(rawHTML string) string {
	// Inspect only the head of the document — doctype must appear first.
	head := rawHTML
	if len(head) > 512 {
		head = head[:512]
	}
	upper := strings.ToUpper(head)

	idx := strings.Index(upper, "<!DOCTYPE")
	if idx == -1 {
		return "Unknown"
	}

	doctypeEnd := strings.Index(upper[idx:], ">")
	if doctypeEnd == -1 {
		return "Unknown"
	}
	doctype := upper[idx : idx+doctypeEnd+1]

	// Check specific legacy doctypes first.
	for _, rule := range doctypeRules {
		if strings.Contains(doctype, strings.ToUpper(rule.marker)) {
			return rule.version
		}
	}

	// "<!DOCTYPE html>" with no extra tokens → HTML5.
	// After uppercasing, a minimal HTML5 doctype is "<!DOCTYPE HTML>".
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(doctype, "<!DOCTYPE"), ">"))
	if trimmed == "HTML" {
		return "HTML5"
	}

	return "Unknown"
}
