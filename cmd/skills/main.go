package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Registries map[string]Registry `json:"registries"`
}

type Registry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Index struct {
	Schema      int     `json:"schema"`
	GeneratedAt string  `json:"generated_at"`
	Skills      []Skill `json:"skills"`
}

type Skill struct {
	Name        string       `json:"name"`
	Latest      string       `json:"latest"`
	Description string       `json:"description"`
	Repository  string       `json:"repository"`
	ProjectID   int          `json:"project_id"`
	Tags        []string     `json:"tags"`
	Auth        Auth         `json:"auth"`
	Permissions Permissions  `json:"permissions"`
	Versions    []VersionRef `json:"versions"`
	Registry    string       `json:"-"`
}

type Auth struct {
	Mode string `json:"mode"`
	Doc  string `json:"doc"`
}

type Permissions struct {
	Network     bool `json:"network"`
	Filesystem  bool `json:"filesystem"`
	Process     bool `json:"process"`
	Destructive bool `json:"destructive"`
}

type VersionRef struct {
	Version     string `json:"version"`
	PackageURL  string `json:"package_url"`
	ManifestURL string `json:"manifest_url"`
	SHA256      string `json:"sha256"`
}

type LockFile struct {
	Installed []InstalledSkill `json:"installed"`
}

type InstalledSkill struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Registry    string `json:"registry"`
	SHA256      string `json:"sha256"`
	InstalledAt string `json:"installed_at"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return nil
	}
	switch args[0] {
	case "registry":
		return registryCmd(args[1:])
	case "search":
		return searchCmd(args[1:])
	case "info":
		return infoCmd(args[1:])
	case "install":
		return installCmd(args[1:])
	case "pack":
		return packCmd(args[1:])
	case "publish":
		return publishCmd(args[1:])
	case "list":
		return listCmd(args[1:])
	case "remove":
		return removeCmd(args[1:])
	case "doctor":
		return doctorCmd(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printHelp() {
	fmt.Println(`skills - internal skill package manager

Usage:
  skills registry add <name> <index-url>
  skills registry list
  skills registry remove <name>
  skills search <query>
  skills info <name>
  skills install <name[@version]> [--yes]
  skills pack <skill-root> [--out dist]
  skills publish <skill-root> --gitlab-url <url> --project-id <id> [--dist dist] [--token-env CI_JOB_TOKEN] [--insecure-skip-tls-verify]
  skills list
  skills remove <name> --yes
  skills doctor

Authentication is external. skills does not store or inject tool tokens.`)
}

func registryCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("registry subcommand required: add, list, remove")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return errors.New("usage: skills registry add <name> <index-url>")
		}
		cfg.Registries[args[1]] = Registry{Name: args[1], URL: args[2]}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("registry added: %s %s\n", args[1], args[2])
	case "list":
		if len(cfg.Registries) == 0 {
			fmt.Println("no registries configured")
			return nil
		}
		names := sortedRegistryNames(cfg)
		for _, name := range names {
			fmt.Printf("%s\t%s\n", name, cfg.Registries[name].URL)
		}
	case "remove":
		if len(args) != 2 {
			return errors.New("usage: skills registry remove <name>")
		}
		delete(cfg.Registries, args[1])
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("registry removed: %s\n", args[1])
	default:
		return fmt.Errorf("unknown registry subcommand %q", args[0])
	}
	return nil
}

func searchCmd(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: skills search <query>")
	}
	skills, err := loadAllSkills()
	if err != nil {
		return err
	}
	q := strings.ToLower(args[0])
	found := 0
	for _, s := range skills {
		if matchesSkill(s, q) {
			fmt.Printf("%s\t%s\t%s\n", s.Name, s.Latest, s.Description)
			found++
		}
	}
	if found == 0 {
		fmt.Println("no matching skills")
	}
	return nil
}

func infoCmd(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: skills info <name>")
	}
	s, err := findSkill(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("Name: %s\n", s.Name)
	fmt.Printf("Latest: %s\n", s.Latest)
	fmt.Printf("Description: %s\n", s.Description)
	fmt.Printf("Repository: %s\n", s.Repository)
	fmt.Printf("Registry: %s\n", s.Registry)
	fmt.Printf("Auth: %s (%s)\n", s.Auth.Mode, s.Auth.Doc)
	fmt.Printf("Permissions: network=%v filesystem=%v process=%v destructive=%v\n", s.Permissions.Network, s.Permissions.Filesystem, s.Permissions.Process, s.Permissions.Destructive)
	fmt.Println("Versions:")
	for _, v := range s.Versions {
		fmt.Printf("  %s\t%s\n", v.Version, v.PackageURL)
	}
	return nil
}

func installCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: skills install <name[@version]> [--yes]")
	}
	name, version := splitNameVersion(args[0])
	yes := hasFlag(args[1:], "--yes")
	s, err := findSkill(name)
	if err != nil {
		return err
	}
	v, err := chooseVersion(s, version)
	if err != nil {
		return err
	}
	if v.SHA256 == "" {
		return fmt.Errorf("skill %s@%s has no sha256 in index", s.Name, v.Version)
	}
	showInstallSummary(s, v)
	dest := filepath.Join(installDir(), s.Name)
	if exists(dest) && !yes {
		return fmt.Errorf("%s already exists; rerun with --yes to replace", dest)
	}
	if err := os.MkdirAll(installDir(), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(installDir(), ".tmp-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	pkg := filepath.Join(tmp, filepath.Base(v.PackageURL))
	if err := downloadFile(v.PackageURL, pkg); err != nil {
		return err
	}
	actual, err := fileSHA256(pkg)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, v.SHA256) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", actual, v.SHA256)
	}
	unpackDir := filepath.Join(tmp, "unpack")
	if err := os.MkdirAll(unpackDir, 0o755); err != nil {
		return err
	}
	if err := untarGz(pkg, unpackDir); err != nil {
		return err
	}
	if err := validateSkillDir(unpackDir); err != nil {
		return err
	}
	if exists(dest) {
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}
	if err := os.Rename(unpackDir, dest); err != nil {
		return err
	}
	if err := updateLock(InstalledSkill{Name: s.Name, Version: v.Version, Registry: s.Registry, SHA256: v.SHA256, InstalledAt: time.Now().Format(time.RFC3339)}); err != nil {
		return err
	}
	fmt.Printf("installed %s@%s to %s\n", s.Name, v.Version, dest)
	return nil
}

func listCmd(args []string) error {
	lock, err := loadLock()
	if err != nil {
		return err
	}
	if len(lock.Installed) == 0 {
		fmt.Println("no skills installed")
		return nil
	}
	for _, s := range lock.Installed {
		fmt.Printf("%s\t%s\t%s\n", s.Name, s.Version, s.InstalledAt)
	}
	return nil
}

func removeCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: skills remove <name> --yes")
	}
	if !hasFlag(args[1:], "--yes") {
		return errors.New("remove requires --yes")
	}
	name := args[0]
	dest := filepath.Join(installDir(), name)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	lock, err := loadLock()
	if err != nil {
		return err
	}
	kept := lock.Installed[:0]
	for _, s := range lock.Installed {
		if s.Name != name {
			kept = append(kept, s)
		}
	}
	lock.Installed = kept
	if err := saveLock(lock); err != nil {
		return err
	}
	fmt.Printf("removed %s\n", name)
	return nil
}

func doctorCmd(args []string) error {
	fmt.Printf("config dir: %s\n", configDir())
	fmt.Printf("install dir: %s\n", installDir())
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	fmt.Printf("registries: %d\n", len(cfg.Registries))
	for _, name := range sortedRegistryNames(cfg) {
		fmt.Printf("  %s\t%s\n", name, cfg.Registries[name].URL)
	}
	return nil
}

func configDir() string {
	if v := os.Getenv("SKILLS_CONFIG_DIR"); v != "" {
		return v
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "skills")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "skills")
}

func installDir() string {
	if v := os.Getenv("SKILLS_INSTALL_DIR"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".agents", "skills")
}

func configPath() string { return filepath.Join(configDir(), "config.json") }
func lockPath() string   { return filepath.Join(installDir(), "skills.lock") }

func loadConfig() (Config, error) {
	cfg := Config{Registries: map[string]Registry{}}
	b, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Registries == nil {
		cfg.Registries = map[string]Registry{}
	}
	return cfg, nil
}

func saveConfig(cfg Config) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), append(b, '\n'), 0o600)
}

func loadAllSkills() ([]Skill, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if len(cfg.Registries) == 0 {
		return nil, errors.New("no registries configured; use skills registry add <name> <index-url>")
	}
	var out []Skill
	for _, name := range sortedRegistryNames(cfg) {
		reg := cfg.Registries[name]
		idx, err := fetchIndex(reg.URL)
		if err != nil {
			return nil, fmt.Errorf("registry %s: %w", name, err)
		}
		for _, s := range idx.Skills {
			s.Registry = name
			out = append(out, s)
		}
	}
	return out, nil
}

func fetchIndex(url string) (Index, error) {
	var r io.ReadCloser
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return Index{}, err
		}
		addRegistryAuth(req)
		resp, err := internalHTTPClient().Do(req)
		if err != nil {
			return Index{}, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			resp.Body.Close()
			return Index{}, fmt.Errorf("HTTP %s", resp.Status)
		}
		r = resp.Body
	} else {
		f, err := os.Open(url)
		if err != nil {
			return Index{}, err
		}
		r = f
	}
	defer r.Close()
	var idx Index
	if err := json.NewDecoder(r).Decode(&idx); err != nil {
		return Index{}, err
	}
	if idx.Schema != 1 {
		return Index{}, fmt.Errorf("unsupported index schema %d", idx.Schema)
	}
	return idx, nil
}

func findSkill(name string) (Skill, error) {
	skills, err := loadAllSkills()
	if err != nil {
		return Skill{}, err
	}
	for _, s := range skills {
		if s.Name == name {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("skill not found: %s", name)
}

func matchesSkill(s Skill, q string) bool {
	if strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(s.Description), q) {
		return true
	}
	for _, t := range s.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func chooseVersion(s Skill, version string) (VersionRef, error) {
	if version == "" {
		version = s.Latest
	}
	for _, v := range s.Versions {
		if v.Version == version {
			return v, nil
		}
	}
	return VersionRef{}, fmt.Errorf("version %s not found for %s", version, s.Name)
}

func splitNameVersion(raw string) (string, string) {
	parts := strings.SplitN(raw, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return raw, ""
}

func showInstallSummary(s Skill, v VersionRef) {
	fmt.Printf("Skill: %s\nVersion: %s\nSource: %s\n", s.Name, v.Version, s.Repository)
	fmt.Printf("Permissions: network=%v filesystem=%v process=%v destructive=%v\n", s.Permissions.Network, s.Permissions.Filesystem, s.Permissions.Process, s.Permissions.Destructive)
	fmt.Printf("Authentication: %s (%s)\n", s.Auth.Mode, s.Auth.Doc)
}

func downloadFile(url, dest string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		src, err := os.Open(url)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, src)
		return err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	addRegistryAuth(req)
	resp, err := internalHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download failed: HTTP %s", resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func untarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(h.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, "..") {
			return fmt.Errorf("unsafe path in package: %s", h.Name)
		}
		target := filepath.Join(dest, name)
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if absTarget != cleanDest && !strings.HasPrefix(absTarget, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in package: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported tar entry: %s", h.Name)
		}
	}
}

func validateSkillDir(dir string) error {
	if !exists(filepath.Join(dir, "SKILL.md")) {
		return errors.New("package missing SKILL.md")
	}
	if !exists(filepath.Join(dir, "skill.json")) {
		return errors.New("package missing skill.json")
	}
	return nil
}

func loadLock() (LockFile, error) {
	var lock LockFile
	b, err := os.ReadFile(lockPath())
	if os.IsNotExist(err) {
		return lock, nil
	}
	if err != nil {
		return lock, err
	}
	return lock, json.Unmarshal(b, &lock)
}

func saveLock(lock LockFile) error {
	if err := os.MkdirAll(installDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lockPath(), append(b, '\n'), 0o600)
}

func updateLock(item InstalledSkill) error {
	lock, err := loadLock()
	if err != nil {
		return err
	}
	for i := range lock.Installed {
		if lock.Installed[i].Name == item.Name {
			lock.Installed[i] = item
			return saveLock(lock)
		}
	}
	lock.Installed = append(lock.Installed, item)
	return saveLock(lock)
}

func sortedRegistryNames(cfg Config) []string {
	names := make([]string, 0, len(cfg.Registries))
	for name := range cfg.Registries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

