package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LocalSkill struct {
	Schema      int               `json:"schema"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Entry       string            `json:"entry"`
	Auth        Auth              `json:"auth"`
	Permissions Permissions       `json:"permissions"`
	Tags        []string          `json:"tags"`
	Platforms   []string          `json:"platforms"`
	Language    string            `json:"language"`
	Maintainers []string          `json:"maintainers"`
	License     string            `json:"license"`
	Extra       map[string]string `json:"-"`
}

type PackageManifest struct {
	Schema      int            `json:"schema"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Entry       string         `json:"entry"`
	GeneratedAt string         `json:"generated_at"`
	Files       []ManifestFile `json:"files"`
}

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func packCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: skills pack <skill-root> [--out dist]")
	}
	root := args[0]
	outDir := "dist"
	for i := 1; i < len(args); i++ {
		if args[i] == "--out" {
			if i+1 >= len(args) {
				return errors.New("--out requires a value")
			}
			outDir = args[i+1]
			i++
			continue
		}
		return fmt.Errorf("unknown pack argument %q", args[i])
	}
	return packSkill(root, outDir)
}

func packSkill(root, outDir string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	skill, err := readLocalSkill(rootAbs)
	if err != nil {
		return err
	}
	files, err := collectPackageFiles(rootAbs)
	if err != nil {
		return err
	}
	outAbs := outDir
	if !filepath.IsAbs(outAbs) {
		outAbs = filepath.Join(rootAbs, outDir)
	}
	if err := os.MkdirAll(outAbs, 0o755); err != nil {
		return err
	}
	base := fmt.Sprintf("%s-%s", skill.Name, skill.Version)
	packagePath := filepath.Join(outAbs, base+".skillpack.tar.gz")
	manifestPath := filepath.Join(outAbs, base+".manifest.json")
	shaPath := filepath.Join(outAbs, base+".sha256")
	manifest, err := buildPackageManifest(rootAbs, skill, files)
	if err != nil {
		return err
	}
	if err := writeJSON(manifestPath, manifest); err != nil {
		return err
	}
	if err := writeTarGz(rootAbs, packagePath, files); err != nil {
		return err
	}
	pkgHash, err := fileSHA256(packagePath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(shaPath, []byte(fmt.Sprintf("%s  %s\n", pkgHash, filepath.Base(packagePath))), 0o644); err != nil {
		return err
	}
	fmt.Printf("created %s\n", packagePath)
	fmt.Printf("created %s\n", manifestPath)
	fmt.Printf("created %s\n", shaPath)
	return nil
}

func readLocalSkill(root string) (LocalSkill, error) {
	if !exists(filepath.Join(root, "SKILL.md")) {
		return LocalSkill{}, errors.New("skill root missing SKILL.md")
	}
	if !exists(filepath.Join(root, "skill.json")) {
		return LocalSkill{}, errors.New("skill root missing skill.json")
	}
	b, err := os.ReadFile(filepath.Join(root, "skill.json"))
	if err != nil {
		return LocalSkill{}, err
	}
	var skill LocalSkill
	if err := json.Unmarshal(b, &skill); err != nil {
		return LocalSkill{}, err
	}
	if skill.Schema != 1 {
		return LocalSkill{}, fmt.Errorf("unsupported skill schema %d", skill.Schema)
	}
	if !validSkillName(skill.Name) {
		return LocalSkill{}, fmt.Errorf("invalid skill name %q", skill.Name)
	}
	if !validSemver(skill.Version) {
		return LocalSkill{}, fmt.Errorf("invalid skill version %q", skill.Version)
	}
	if skill.Entry != "SKILL.md" {
		return LocalSkill{}, errors.New("skill entry must be SKILL.md")
	}
	if skill.Auth.Mode != "external" || skill.Auth.Doc == "" {
		return LocalSkill{}, errors.New("skill auth.mode must be external and auth.doc is required")
	}
	return skill, nil
}

func collectPackageFiles(root string) ([]string, error) {
	var files []string
	excludedDirs := map[string]bool{
		".git":   true,
		"dist":   true,
		"build":  true,
		".cache": true,
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if d.IsDir() && excludedDirs[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") || rel == ".." || filepath.IsAbs(rel) {
			return fmt.Errorf("unsafe package path %s", rel)
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func buildPackageManifest(root string, skill LocalSkill, files []string) (PackageManifest, error) {
	manifest := PackageManifest{
		Schema:      1,
		Name:        skill.Name,
		Version:     skill.Version,
		Entry:       "SKILL.md",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			return PackageManifest{}, err
		}
		hash, err := fileSHA256(path)
		if err != nil {
			return PackageManifest{}, err
		}
		manifest.Files = append(manifest.Files, ManifestFile{Path: rel, SHA256: hash, Size: info.Size()})
	}
	return manifest, nil
}

func writeTarGz(root, dest string, files []string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func validSkillName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' && i > 0:
		default:
			return false
		}
	}
	return true
}

func validSemver(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
