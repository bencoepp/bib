package checks

import (
	"bib/internal/capcheck"
	"context"
	"net/http"
	"time"
)

type CloudEnvironmentChecker struct{}

func (c CloudEnvironmentChecker) ID() capcheck.CapabilityID { return "cloud_environment" }
func (c CloudEnvironmentChecker) Description() string {
	return "Heuristically detects major cloud metadata endpoints (AWS, GCP, Azure)."
}

func (c CloudEnvironmentChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "Cloud environment",
		Details: map[string]any{},
	}
	type meta struct {
		Name string
		URL  string
		Head bool
	}
	targets := []meta{
		{"aws", "http://169.254.169.254/latest/meta-data/", true},
		{"gcp", "http://169.254.169.254/computeMetadata/v1/", true},
		{"azure", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", true},
	}
	client := &http.Client{Timeout: 400 * time.Millisecond}
	detected := []string{}
	for _, t := range targets {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
		if t.Name == "gcp" {
			req.Header.Set("Metadata-Flavor", "Google")
		}
		if t.Name == "azure" {
			req.Header.Set("Metadata", "true")
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				detected = append(detected, t.Name)
			}
		}
	}
	res.Details["detected"] = detected
	if len(detected) > 0 {
		res.Supported = true
	} else {
		res.Error = "no cloud metadata endpoint reachable"
	}
	return res
}
