package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func publishCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: skills publish <skill-root> --gitlab-url <url> --project-id <id> [--dist dist] [--token-env CI_JOB_TOKEN] [--insecure-skip-tls-verify]")
	}
	root := args[0]
	dist := "dist"
	gitlabURL := ""
	projectID := ""
	tokenEnv := "CI_JOB_TOKEN"
	insecureSkipTLSVerify := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--dist":
			if i+1 >= len(args) {
				return errors.New("--dist requires a value")
			}
			dist = args[i+1]
			i++
		case "--gitlab-url":
			if i+1 >= len(args) {
				return errors.New("--gitlab-url requires a value")
			}
			gitlabURL = args[i+1]
			i++
		case "--project-id":
			if i+1 >= len(args) {
				return errors.New("--project-id requires a value")
			}
			projectID = args[i+1]
			i++
		case "--token-env":
			if i+1 >= len(args) {
				return errors.New("--token-env requires a value")
			}
			tokenEnv = args[i+1]
			i++
		case "--insecure-skip-tls-verify":
			insecureSkipTLSVerify = true
		default:
			return fmt.Errorf("unknown publish argument %q", args[i])
		}
	}
	if gitlabURL == "" {
		return errors.New("--gitlab-url is required")
	}
	if projectID == "" {
		return errors.New("--project-id is required")
	}
	token := os.Getenv(tokenEnv)
	if token == "" {
		return fmt.Errorf("environment variable %s is not set", tokenEnv)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	skill, err := readLocalSkill(rootAbs)
	if err != nil {
		return err
	}
	distAbs := dist
	if !filepath.IsAbs(distAbs) {
		distAbs = filepath.Join(rootAbs, dist)
	}
	pattern := filepath.Join(distAbs, fmt.Sprintf("%s-%s.*", skill.Name, skill.Version))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no package files found matching %s", pattern)
	}
	apiBase := normalizeGitLabAPI(gitlabURL)
	for _, file := range files {
		if err := uploadGenericPackageFile(apiBase, projectID, skill.Name, skill.Version, file, tokenEnv, token, insecureSkipTLSVerify); err != nil {
			return err
		}
		fmt.Printf("uploaded %s\n", filepath.Base(file))
	}
	return nil
}

func normalizeGitLabAPI(raw string) string {
	base := strings.TrimRight(raw, "/")
	if strings.HasSuffix(base, "/api/v4") {
		return base
	}
	return base + "/api/v4"
}

func uploadGenericPackageFile(apiBase, projectID, packageName, version, path, tokenEnv, token string, insecureSkipTLSVerify bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	uri := fmt.Sprintf("%s/projects/%s/packages/generic/%s/%s/%s",
		apiBase,
		url.PathEscape(projectID),
		url.PathEscape(packageName),
		url.PathEscape(version),
		url.PathEscape(filepath.Base(path)),
	)
	req, err := http.NewRequest(http.MethodPut, uri, file)
	if err != nil {
		return err
	}
	if tokenEnv == "CI_JOB_TOKEN" {
		req.Header.Set("JOB-TOKEN", token)
	} else {
		req.Header.Set("PRIVATE-TOKEN", token)
	}
	resp, err := internalHTTPClientWithTLS(insecureSkipTLSVerify).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("upload %s failed: HTTP %s", filepath.Base(path), resp.Status)
	}
	return nil
}

