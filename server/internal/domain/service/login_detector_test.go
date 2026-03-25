package service_test

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"web-analyzer/internal/domain/service"
)

func parseDoc(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parseDoc: %v", err)
	}
	return doc
}

func TestLoginDetector_FormWithPasswordAndLoginAction(t *testing.T) {
	doc := parseDoc(t, `<html><body>
		<form action="/login">
			<input type="password" name="pass">
		</form>
	</body></html>`)
	if !service.DetectLoginForm(doc) {
		t.Error("expected HasLoginForm=true: form with password input and login action")
	}
}

func TestLoginDetector_FormWithPasswordNoSignal_FallbackTrue(t *testing.T) {
	// Fallback rule: any <input type="password"> anywhere → true
	doc := parseDoc(t, `<html><body>
		<form action="/submit">
			<input type="password" name="pass">
		</form>
	</body></html>`)
	if !service.DetectLoginForm(doc) {
		t.Error("expected HasLoginForm=true: fallback rule — password input present")
	}
}

func TestLoginDetector_FormWithNoPasswordInput_False(t *testing.T) {
	doc := parseDoc(t, `<html><body>
		<form action="/login">
			<input type="text" name="user">
		</form>
	</body></html>`)
	if service.DetectLoginForm(doc) {
		t.Error("expected HasLoginForm=false: no password input")
	}
}

func TestLoginDetector_NoForm_False(t *testing.T) {
	doc := parseDoc(t, `<html><body>
		<p>No form here</p>
	</body></html>`)
	if service.DetectLoginForm(doc) {
		t.Error("expected HasLoginForm=false: no form element")
	}
}

func TestLoginDetector_AnmeldenInFormText_True(t *testing.T) {
	doc := parseDoc(t, `<html><body>
		<form>
			<label>Anmelden</label>
			<input type="password" name="pass">
		</form>
	</body></html>`)
	if !service.DetectLoginForm(doc) {
		t.Error("expected HasLoginForm=true: 'anmelden' in form text")
	}
}

func TestLoginDetector_SignInInFormClass_True(t *testing.T) {
	doc := parseDoc(t, `<html><body>
		<form class="sign-in-form">
			<input type="password" name="pass">
		</form>
	</body></html>`)
	if !service.DetectLoginForm(doc) {
		t.Error("expected HasLoginForm=true: 'sign-in' in form class (contains 'sign in' signal)")
	}
}
