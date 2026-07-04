package main

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"os"
	"time"
)

func internalHTTPClient() (*http.Client, error) {
	return internalHTTPClientWithTLS(false)
}

func internalHTTPClientWithTLS(insecureSkipVerify bool) (*http.Client, error) {
	proxyFunc, err := configuredProxyFunc()
	if err != nil {
		return nil, err
	}
	return &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{
		Proxy: proxyFunc,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // explicit CLI flag for internal self-signed GitLab.
		},
	}}, nil
}

func configuredProxyFunc() (func(*http.Request) (*url.URL, error), error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	proxy, err := effectiveProxyConfig(cfg.Network.Proxy)
	if err != nil {
		return nil, err
	}
	switch proxy.Mode {
	case "system":
		return http.ProxyFromEnvironment, nil
	case "none":
		return nil, nil
	case "custom":
		u, err := url.Parse(proxy.URL)
		if err != nil {
			return nil, err
		}
		return http.ProxyURL(u), nil
	default:
		return http.ProxyFromEnvironment, nil
	}
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
