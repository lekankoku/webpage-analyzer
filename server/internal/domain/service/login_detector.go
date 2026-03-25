package service

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// loginSignals are the substrings checked (case-insensitively) against a form's
// action, id, class, and visible text content.
var loginSignals = []string{"login", "sign in", "signin", "anmelden", "log in"}

// DetectLoginForm returns true when the document appears to contain a login form.
//
// Primary rule: any <form> that contains an <input type="password"> AND whose
// action/id/class/text matches a login signal.
//
// Fallback rule: if <input type="password"> exists anywhere on the page without a
// matching form context, still return true — real-world markup is often messy.
func DetectLoginForm(doc *goquery.Document) bool {
	hasPasswordInput := doc.Find(`input[type="password"]`).Length() > 0
	if !hasPasswordInput {
		return false
	}

	found := false
	doc.Find("form").EachWithBreak(func(_ int, form *goquery.Selection) bool {
		if form.Find(`input[type="password"]`).Length() == 0 {
			return true // continue
		}
		action, _ := form.Attr("action")
		id, _ := form.Attr("id")
		class, _ := form.Attr("class")
		text := form.Text()
		combined := strings.ToLower(action + " " + id + " " + class + " " + text)

		for _, sig := range loginSignals {
			if strings.Contains(combined, sig) {
				found = true
				return false // break
			}
		}
		return true
	})

	if found {
		return true
	}

	// Fallback: password input present but no matching form context found.
	return hasPasswordInput
}
