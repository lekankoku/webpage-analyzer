package model

// AnalysisResult is the final output of a completed (or partially completed) analysis.
// Partial is true when link checking was interrupted before all links were verified;
// in that case InaccessibleLinks and UnverifiedLinks may be understated.
type AnalysisResult struct {
	HTMLVersion       string         `json:"html_version"`
	Title             string         `json:"title"`
	Headings          map[string]int `json:"headings"`
	InternalLinks     int            `json:"internal_links"`
	ExternalLinks     int            `json:"external_links"`
	InaccessibleLinks int            `json:"inaccessible_links"`
	UnverifiedLinks   int            `json:"unverified_links"`
	HasLoginForm      bool           `json:"has_login_form"`
	Partial           bool           `json:"partial,omitempty"`
}
