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
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Registries map[string]Registry `json:"registries"`
	Network    NetworkConfig       `json:"network"`
}

type Registry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type NetworkConfig struct {
	Proxy ProxyConfig `json:"proxy"`
}

type ProxyConfig struct {
	Mode string `json:"mode"`
	URL  string `json:"url"`
}

const defaultRegistryName = "xiaodonghu"
const defaultRegistryURL = "https://raw.githubusercontent.com/huxiaodong07/skills/main/registry/registry-index.json"

type Index struct {
	Schema      int     `json:"schema"`
	GeneratedAt string  `json:"generated_at"`
	Plugins     []Plugin `json:"plugins"`
	Skills      []Skill `json:"skills"`
}

type Plugin struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Repository  string   `json:"repository"`
	Source      string   `json:"source"`
	Skills      []string `json:"skills"`
	Registry    string   `json:"-"`
}

type CatalogItem struct {
	Type        string
	Name        string
	Version     string
	Description string
	Skill       Skill
	Plugin      Plugin
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

type BinLinks struct {
	Commands map[string]BinLink `json:"commands"`
}

type BinLink struct {
	Skill   string `json:"skill"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Target  string `json:"target"`
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
	case "config":
		return configCmd(args[1:])
	case "registry":
		return registryCmd(args[1:])
	case "plugin":
		return pluginCmd(args[1:])
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
	fmt.Println(`skills - skill package manager

Usage:
  skills config list
  skills config get proxy
  skills config set proxy <system|none|url>
  skills registry add <name> <index-url>
  skills registry list
  skills registry remove <name>
  skills search [query]
  skills info <name|skill:name|plugin:name>
  skills install <name[@version]> [--version <version>] [--yes]
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

func configCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand required: list, get, set")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: skills config list")
		}
		fmt.Printf("proxy\t%s\n", formatProxyConfig(cfg.Network.Proxy))
	case "get":
		if len(args) != 2 {
			return errors.New("usage: skills config get <key>")
		}
		switch args[1] {
		case "proxy":
			fmt.Println(formatProxyConfig(cfg.Network.Proxy))
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
	case "set":
		if len(args) != 3 {
			return errors.New("usage: skills config set proxy <system|none|url>")
		}
		switch args[1] {
		case "proxy":
			proxy, err := parseProxyConfig(args[2])
			if err != nil {
				return err
			}
			cfg.Network.Proxy = proxy
		default:
			return fmt.Errorf("unknown config key %q", args[1])
		}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("config set: proxy %s\n", formatProxyConfig(cfg.Network.Proxy))
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
	return nil
}

func pluginCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("plugin subcommand required: list, info, install")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: skills plugin list")
		}
		plugins, err := loadAllPlugins()
		if err != nil {
			return err
		}
		if len(plugins) == 0 {
			fmt.Println("no plugins available")
			return nil
		}
		for _, p := range plugins {
			fmt.Printf("%s\t%s\t%s\n", p.Name, p.Version, p.Description)
		}
	case "info":
		if len(args) != 2 {
			return errors.New("usage: skills plugin info <name>")
		}
		p, err := findPlugin(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Name: %s\n", p.Name)
		fmt.Printf("Version: %s\n", p.Version)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("Repository: %s\n", p.Repository)
		fmt.Printf("Registry: %s\n", p.Registry)
		fmt.Println("Skills:")
		for _, name := range p.Skills {
			fmt.Printf("  %s\n", name)
		}
	case "install":
		if len(args) == 1 {
			return errors.New("usage: skills plugin install <name> [--yes]")
		}
		name := args[1]
		yes := false
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--yes", "-y":
				yes = true
			default:
				return fmt.Errorf("unknown plugin install option %q", args[i])
			}
		}
		p, err := findPlugin(name)
		if err != nil {
			return err
		}
		if len(p.Skills) == 0 {
			return fmt.Errorf("plugin %s has no skills", p.Name)
		}
		fmt.Printf("Plugin: %s\nVersion: %s\nSkills: %s\n", p.Name, p.Version, strings.Join(p.Skills, ", "))
		for _, skillName := range p.Skills {
			if err := installSkillByName(skillName, "", yes); err != nil {
				return fmt.Errorf("install plugin %s skill %s: %w", p.Name, skillName, err)
			}
		}
		fmt.Printf("installed plugin %s with %d skills\n", p.Name, len(p.Skills))
	default:
		return fmt.Errorf("unknown plugin subcommand %q", args[0])
	}
	return nil
}

func searchCmd(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: skills search [query]")
	}
	items, err := loadCatalog()
	if err != nil {
		return err
	}
	q := ""
	if len(args) == 1 {
		q = strings.ToLower(args[0])
	}
	found := 0
	for _, item := range items {
		if q == "" || matchesCatalogItem(item, q) {
			fmt.Printf("%s\t%s\t%s\t%s\n", item.Type, item.Name, item.Version, item.Description)
			found++
		}
	}
	if found == 0 {
		fmt.Println("no matching items")
	}
	return nil
}

func infoCmd(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: skills info <name|skill:name|plugin:name>")
	}
	item, err := resolveCatalogItem(args[0])
	if err != nil {
		return err
	}
	switch item.Type {
	case "plugin":
		p := item.Plugin
		fmt.Println("Type: plugin")
		fmt.Printf("Name: %s\n", p.Name)
		fmt.Printf("Version: %s\n", p.Version)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("Repository: %s\n", p.Repository)
		fmt.Printf("Registry: %s\n", p.Registry)
		fmt.Println("Skills:")
		for _, name := range p.Skills {
			fmt.Printf("  %s\n", name)
		}
	case "skill":
		s := item.Skill
		fmt.Println("Type: skill")
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
	}
	return nil
}

func installCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: skills install <name[@version]> [--version <version>] [--yes]")
	}
	rawName, version := splitNameVersion(args[0])
	nameType, name := splitTypedName(rawName)
	if name == "" || strings.HasPrefix(name, "-") {
		return errors.New("usage: skills install <name[@version]> [--version <version>] [--yes]")
	}
	yes := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--yes", "-y":
			yes = true
		case "--version":
			if i+1 >= len(args) {
				return errors.New("install --version requires a value")
			}
			if version != "" {
				return errors.New("install version specified more than once")
			}
			i++
			version = args[i]
			if version == "" || strings.HasPrefix(version, "-") {
				return errors.New("install --version requires a value")
			}
		default:
			return fmt.Errorf("unknown install option %q", args[i])
		}
	}
	if nameType == "skill" {
		return installSkillByName(name, version, yes)
	}
	if nameType == "plugin" {
		if version != "" {
			return errors.New("plugin install does not support --version yet")
		}
		return installPluginByName(name, yes)
	}
	item, err := resolveCatalogItem(rawName)
	if err != nil {
		return err
	}
	if item.Type == "plugin" {
		if version != "" {
			return errors.New("plugin install does not support --version yet")
		}
		return installPlugin(item.Plugin, yes)
	}
	v, err := chooseVersion(item.Skill, version)
	if err != nil {
		return err
	}
	return installResolvedSkill(item.Skill, v, yes)
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
	if err := removeBinLinks(name); err != nil {
		return err
	}
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
	fmt.Printf("command bin: %s\n", commandBinDir())
	fmt.Printf("command bin on PATH: %v\n", commandBinOnPATH())
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	effectiveProxy, err := effectiveProxyConfig(cfg.Network.Proxy)
	if err != nil {
		return err
	}
	fmt.Printf("proxy: %s\n", formatProxyConfig(effectiveProxy))
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

func hxdHomeDir() string {
	if v := os.Getenv("HXD_SKILLS_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "hxdSkills")
}

func commandBinDir() string { return filepath.Join(hxdHomeDir(), "bin") }
func linksPath() string     { return filepath.Join(hxdHomeDir(), "links.json") }

func configPath() string { return filepath.Join(configDir(), "config.json") }
func lockPath() string   { return filepath.Join(installDir(), "skills.lock") }

func loadConfig() (Config, error) {
	b, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := Config{}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Registries == nil {
		cfg.Registries = map[string]Registry{}
	}
	cfg.Network.Proxy = normalizeProxyConfig(cfg.Network.Proxy)
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Registries: map[string]Registry{
			defaultRegistryName: {Name: defaultRegistryName, URL: defaultRegistryURL},
		},
		Network: NetworkConfig{Proxy: ProxyConfig{Mode: "system"}},
	}
}

func normalizeProxyConfig(proxy ProxyConfig) ProxyConfig {
	if proxy.Mode == "" {
		proxy.Mode = "system"
	}
	if proxy.Mode != "system" && proxy.Mode != "none" && proxy.Mode != "custom" {
		proxy.Mode = "system"
		proxy.URL = ""
	}
	if proxy.Mode != "custom" {
		proxy.URL = ""
	}
	return proxy
}

func parseProxyConfig(raw string) (ProxyConfig, error) {
	switch strings.ToLower(raw) {
	case "system":
		return ProxyConfig{Mode: "system"}, nil
	case "none":
		return ProxyConfig{Mode: "none"}, nil
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") && !strings.HasPrefix(raw, "socks5://") {
		return ProxyConfig{}, errors.New("proxy must be system, none, or a proxy URL")
	}
	return ProxyConfig{Mode: "custom", URL: raw}, nil
}

func effectiveProxyConfig(configured ProxyConfig) (ProxyConfig, error) {
	if mode := os.Getenv("SKILLS_PROXY_MODE"); mode != "" {
		if strings.EqualFold(mode, "custom") {
			url := os.Getenv("SKILLS_PROXY_URL")
			if url == "" {
				return ProxyConfig{}, errors.New("SKILLS_PROXY_MODE=custom requires SKILLS_PROXY_URL")
			}
			return parseProxyConfig(url)
		}
		return parseProxyConfig(mode)
	}
	if url := os.Getenv("SKILLS_PROXY_URL"); url != "" {
		return parseProxyConfig(url)
	}
	return normalizeProxyConfig(configured), nil
}

func formatProxyConfig(proxy ProxyConfig) string {
	proxy = normalizeProxyConfig(proxy)
	if proxy.Mode == "custom" {
		return proxy.URL
	}
	return proxy.Mode
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

func loadAllPlugins() ([]Plugin, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if len(cfg.Registries) == 0 {
		return nil, errors.New("no registries configured; use skills registry add <name> <index-url>")
	}
	var out []Plugin
	for _, name := range sortedRegistryNames(cfg) {
		reg := cfg.Registries[name]
		idx, err := fetchIndex(reg.URL)
		if err != nil {
			return nil, fmt.Errorf("registry %s: %w", name, err)
		}
		for _, p := range idx.Plugins {
			p.Registry = name
			out = append(out, p)
		}
	}
	return out, nil
}

func loadCatalog() ([]CatalogItem, error) {
	plugins, err := loadAllPlugins()
	if err != nil {
		return nil, err
	}
	skills, err := loadAllSkills()
	if err != nil {
		return nil, err
	}
	items := make([]CatalogItem, 0, len(plugins)+len(skills))
	for _, p := range plugins {
		items = append(items, CatalogItem{Type: "plugin", Name: p.Name, Version: p.Version, Description: p.Description, Plugin: p})
	}
	for _, s := range skills {
		items = append(items, CatalogItem{Type: "skill", Name: s.Name, Version: s.Latest, Description: s.Description, Skill: s})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Type < items[j].Type
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func fetchIndex(url string) (Index, error) {
	var r io.ReadCloser
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return Index{}, err
		}
		addRegistryAuth(req)
		client, err := internalHTTPClient()
		if err != nil {
			return Index{}, err
		}
		resp, err := client.Do(req)
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
	var available []string
	for _, s := range skills {
		available = append(available, s.Name)
		if s.Name == name {
			return s, nil
		}
	}
	sort.Strings(available)
	if len(available) == 0 {
		return Skill{}, fmt.Errorf("skill not found: %s; no skills available", name)
	}
	return Skill{}, fmt.Errorf("skill not found: %s; available: %s", name, strings.Join(available, ", "))
}

func findPlugin(name string) (Plugin, error) {
	plugins, err := loadAllPlugins()
	if err != nil {
		return Plugin{}, err
	}
	var available []string
	for _, p := range plugins {
		available = append(available, p.Name)
		if p.Name == name {
			return p, nil
		}
	}
	sort.Strings(available)
	if len(available) == 0 {
		return Plugin{}, fmt.Errorf("plugin not found: %s; no plugins available", name)
	}
	return Plugin{}, fmt.Errorf("plugin not found: %s; available: %s", name, strings.Join(available, ", "))
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

func matchesPlugin(p Plugin, q string) bool {
	if strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Description), q) {
		return true
	}
	for _, s := range p.Skills {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

func matchesCatalogItem(item CatalogItem, q string) bool {
	if strings.Contains(strings.ToLower(item.Type), q) || strings.Contains(strings.ToLower(item.Name), q) || strings.Contains(strings.ToLower(item.Description), q) {
		return true
	}
	if item.Type == "plugin" {
		return matchesPlugin(item.Plugin, q)
	}
	return matchesSkill(item.Skill, q)
}

func resolveCatalogItem(raw string) (CatalogItem, error) {
	nameType, name := splitTypedName(raw)
	if nameType == "skill" {
		s, err := findSkill(name)
		if err != nil {
			return CatalogItem{}, err
		}
		return CatalogItem{Type: "skill", Name: s.Name, Version: s.Latest, Description: s.Description, Skill: s}, nil
	}
	if nameType == "plugin" {
		p, err := findPlugin(name)
		if err != nil {
			return CatalogItem{}, err
		}
		return CatalogItem{Type: "plugin", Name: p.Name, Version: p.Version, Description: p.Description, Plugin: p}, nil
	}
	plugins, pErr := loadAllPlugins()
	if pErr != nil {
		return CatalogItem{}, pErr
	}
	skills, sErr := loadAllSkills()
	if sErr != nil {
		return CatalogItem{}, sErr
	}
	var matched []CatalogItem
	for _, p := range plugins {
		if p.Name == name {
			matched = append(matched, CatalogItem{Type: "plugin", Name: p.Name, Version: p.Version, Description: p.Description, Plugin: p})
		}
	}
	for _, s := range skills {
		if s.Name == name {
			matched = append(matched, CatalogItem{Type: "skill", Name: s.Name, Version: s.Latest, Description: s.Description, Skill: s})
		}
	}
	if len(matched) == 1 {
		return matched[0], nil
	}
	if len(matched) > 1 {
		var names []string
		for _, item := range matched {
			names = append(names, item.Type+":"+item.Name)
		}
		return CatalogItem{}, fmt.Errorf("ambiguous item %s; use %s", name, strings.Join(names, " or "))
	}
	return CatalogItem{}, fmt.Errorf("item not found: %s", name)
}

func chooseVersion(s Skill, version string) (VersionRef, error) {
	if version == "" {
		version = s.Latest
	}
	var available []string
	for _, v := range s.Versions {
		available = append(available, v.Version)
		if v.Version == version {
			return v, nil
		}
	}
	sort.Strings(available)
	if len(available) == 0 {
		return VersionRef{}, fmt.Errorf("version %s not found for %s; no versions available", version, s.Name)
	}
	return VersionRef{}, fmt.Errorf("version %s not found for %s; available: %s", version, s.Name, strings.Join(available, ", "))
}

func splitNameVersion(raw string) (string, string) {
	parts := strings.SplitN(raw, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return raw, ""
}

func splitTypedName(raw string) (string, string) {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return "", raw
	}
	if parts[0] == "skill" || parts[0] == "plugin" {
		return parts[0], parts[1]
	}
	return "", raw
}

func showInstallSummary(s Skill, v VersionRef) {
	fmt.Printf("Skill: %s\nVersion: %s\nSource: %s\n", s.Name, v.Version, s.Repository)
	fmt.Printf("Permissions: network=%v filesystem=%v process=%v destructive=%v\n", s.Permissions.Network, s.Permissions.Filesystem, s.Permissions.Process, s.Permissions.Destructive)
	fmt.Printf("Authentication: %s (%s)\n", s.Auth.Mode, s.Auth.Doc)
}

func installSkillByName(name, version string, yes bool) error {
	s, err := findSkill(name)
	if err != nil {
		return err
	}
	v, err := chooseVersion(s, version)
	if err != nil {
		return err
	}
	return installResolvedSkill(s, v, yes)
}

func installPluginByName(name string, yes bool) error {
	p, err := findPlugin(name)
	if err != nil {
		return err
	}
	return installPlugin(p, yes)
}

func installPlugin(p Plugin, yes bool) error {
	if len(p.Skills) == 0 {
		return fmt.Errorf("plugin %s has no skills", p.Name)
	}
	fmt.Printf("Plugin: %s\nVersion: %s\nSkills: %s\n", p.Name, p.Version, strings.Join(p.Skills, ", "))
	for _, skillName := range p.Skills {
		if err := installSkillByName(skillName, "", yes); err != nil {
			return fmt.Errorf("install plugin %s skill %s: %w", p.Name, skillName, err)
		}
	}
	fmt.Printf("installed plugin %s with %d skills\n", p.Name, len(p.Skills))
	return nil
}

func installResolvedSkill(s Skill, v VersionRef, yes bool) error {
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
	if err := checkBinConflicts(unpackDir, s.Name, yes); err != nil {
		return err
	}
	if exists(dest) {
		if err := removeBinLinks(s.Name); err != nil {
			return err
		}
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}
	if err := os.Rename(unpackDir, dest); err != nil {
		return err
	}
	if err := exposeSkillBin(dest, s.Name, v.Version, true); err != nil {
		return err
	}
	if err := updateLock(InstalledSkill{Name: s.Name, Version: v.Version, Registry: s.Registry, SHA256: v.SHA256, InstalledAt: time.Now().Format(time.RFC3339)}); err != nil {
		return err
	}
	fmt.Printf("installed %s@%s to %s\n", s.Name, v.Version, dest)
	return nil
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
	client, err := internalHTTPClient()
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
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

func loadBinLinks() (BinLinks, error) {
	links := BinLinks{Commands: map[string]BinLink{}}
	b, err := os.ReadFile(linksPath())
	if os.IsNotExist(err) {
		return links, nil
	}
	if err != nil {
		return links, err
	}
	if err := json.Unmarshal(b, &links); err != nil {
		return links, err
	}
	if links.Commands == nil {
		links.Commands = map[string]BinLink{}
	}
	return links, nil
}

func saveBinLinks(links BinLinks) error {
	if err := os.MkdirAll(hxdHomeDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(linksPath(), append(b, '\n'), 0o600)
}

func checkBinConflicts(skillDir, skillName string, overwrite bool) error {
	binDir := filepath.Join(skillDir, "bin")
	entries, err := os.ReadDir(binDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	links, err := loadBinLinks()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		target := filepath.Join(commandBinDir(), name)
		link, known := links.Commands[name]
		if exists(target) && (!known || link.Skill != skillName) && !overwrite {
			if known {
				return fmt.Errorf("command %s is already managed by skill %s; rerun with --yes to replace", name, link.Skill)
			}
			return fmt.Errorf("command %s already exists in %s; rerun with --yes to replace", name, commandBinDir())
		}
	}
	return nil
}

func exposeSkillBin(skillDir, skillName, version string, overwrite bool) error {
	binDir := filepath.Join(skillDir, "bin")
	entries, err := os.ReadDir(binDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(commandBinDir(), 0o755); err != nil {
		return err
	}
	links, err := loadBinLinks()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		source := filepath.Join(binDir, entry.Name())
		target := filepath.Join(commandBinDir(), entry.Name())
		link, known := links.Commands[entry.Name()]
		if exists(target) && (!known || link.Skill != skillName) && !overwrite {
			if known {
				return fmt.Errorf("command %s is already managed by skill %s; rerun with --yes to replace", entry.Name(), link.Skill)
			}
			return fmt.Errorf("command %s already exists in %s; rerun with --yes to replace", entry.Name(), commandBinDir())
		}
		if err := copyFile(source, target, info.Mode()); err != nil {
			return err
		}
		absSource, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		links.Commands[entry.Name()] = BinLink{Skill: skillName, Version: version, Source: absSource, Target: absTarget}
	}
	if err := saveBinLinks(links); err != nil {
		return err
	}
	if len(entries) > 0 {
		if err := ensureCommandBinOnPATH(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update PATH: %v\n", err)
		}
	}
	return nil
}

func removeBinLinks(skillName string) error {
	links, err := loadBinLinks()
	if err != nil {
		return err
	}
	changed := false
	for command, link := range links.Commands {
		if link.Skill != skillName {
			continue
		}
		if link.Target != "" {
			if err := os.Remove(link.Target); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		delete(links.Commands, command)
		changed = true
	}
	if changed {
		return saveBinLinks(links)
	}
	return nil
}

func copyFile(source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func ensureCommandBinOnPATH() error {
	if os.Getenv("SKILLS_SKIP_PATH_UPDATE") != "" {
		return nil
	}
	if commandBinOnPATH() {
		return nil
	}
	if runtime.GOOS != "windows" {
		fmt.Fprintf(os.Stderr, "warning: add %s to PATH to use installed commands directly\n", commandBinDir())
		return nil
	}
	script := fmt.Sprintf(`$dir = %q; $path = [Environment]::GetEnvironmentVariable("Path", "User"); if ([string]::IsNullOrWhiteSpace($path)) { [Environment]::SetEnvironmentVariable("Path", $dir, "User") } else { $parts = $path -split ";"; if ($parts -notcontains $dir) { [Environment]::SetEnvironmentVariable("Path", ($path.TrimEnd(";") + ";" + $dir), "User") } }`, commandBinDir())
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if err := cmd.Run(); err != nil {
		return err
	}
	current := os.Getenv("PATH")
	if current == "" {
		return os.Setenv("PATH", commandBinDir())
	}
	return os.Setenv("PATH", current+";"+commandBinDir())
}

func commandBinOnPATH() bool {
	target, err := filepath.Abs(commandBinDir())
	if err != nil {
		target = commandBinDir()
	}
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if runtime.GOOS == "windows" {
			if strings.EqualFold(abs, target) {
				return true
			}
			continue
		}
		if abs == target {
			return true
		}
	}
	return false
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

