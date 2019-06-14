package main

import (
	"net/http"

	meta "cloud.google.com/go/compute/metadata"
)

func getProjectID() *string {

	// then metadata
	mc := meta.NewClient(&http.Client{Transport: userAgentTransport{
		userAgent: "event-maker",
		base:      http.DefaultTransport,
	}})

	p, err := mc.ProjectID()
	if err != nil {
		logger.Printf("Error creating metadata client: %v", err)
	}

	return &p
}

// GCP Metadata
// https://godoc.org/cloud.google.com/go/compute/metadata#example-NewClient
type userAgentTransport struct {
	userAgent string
	base      http.RoundTripper
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.userAgent)
	return t.base.RoundTrip(req)
}
