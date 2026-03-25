package model

import "net/url"

// Link represents a single hyperlink found on a web page.
// Raw is the href value as-is from the HTML; Resolved is the normalised absolute URL.
type Link struct {
	Raw        string
	Resolved   *url.URL
	IsInternal bool
}
