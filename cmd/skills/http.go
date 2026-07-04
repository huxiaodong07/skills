package main

import (
	"crypto/tls"
	"net/http"
	"os"
)

func internalHTTPClient() *http.Client {
	return internalHTTPClientWithTLS(false)
}

func internalHTTPClientWithTLS(insecureSkipVerify bool) *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy: nil,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // explicit CLI flag for internal self-signed GitLab.
		},
	}}
}

func addRegistryAuth(req *http.Request) {
	for _, name := range []string{"SKILLS_REGISTRY_TOKEN", "GITLAB_API_PAT", "GITLAB_TOKEN"} {
		if value := os.Getenv(name); value != "" {
			req.Header.Set("PRIVATE-TOKEN", value)
			return
		}
	}
	if value := os.Getenv("CI_JOB_TOKEN"); value != "" {
		req.Header.Set("JOB-TOKEN", value)
	}
}
