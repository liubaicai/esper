package compat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

const SourceManifestVersion = "esper-source-tests/v1"

type SourceTestManifest struct {
	Version    string           `json:"version"`
	SourceRoot string           `json:"sourceRoot"`
	JavaCommit string           `json:"javaCommit"`
	Discovery  string           `json:"discovery"`
	Counts     map[string]int   `json:"counts"`
	Cases      []SourceTestCase `json:"cases"`
}

type SourceTestCase struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Module     string `json:"module,omitempty"`
	SourceFile string `json:"sourceFile"`
	ClassName  string `json:"className"`
	Discovery  string `json:"discovery"`
}

func ScanJavaSourceTests(root string, commit string) (SourceTestManifest, error) {
	manifest := SourceTestManifest{
		Version:    SourceManifestVersion,
		SourceRoot: filepath.Clean(root),
		JavaCommit: commit,
		Discovery:  "static-source-file; runtime-test-mapping-required",
		Counts:     make(map[string]int),
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".java") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativeSlash := filepath.ToSlash(relative)
		kind, include := sourceTestKind(relativeSlash)
		if !include {
			return nil
		}
		className := strings.TrimSuffix(filepath.Base(relativeSlash), ".java")
		module := moduleFromPath(relativeSlash)
		idInput := relativeSlash + "\x00" + kind + "\x00" + className
		digest := sha256.Sum256([]byte(idInput))
		manifest.Cases = append(manifest.Cases, SourceTestCase{
			ID:         "source-" + hex.EncodeToString(digest[:])[:20],
			Kind:       kind,
			Module:     module,
			SourceFile: relativeSlash,
			ClassName:  className,
			Discovery:  "static-source-file",
		})
		manifest.Counts[kind]++
		return nil
	})
	if err != nil {
		return SourceTestManifest{}, fmt.Errorf("compat: scan Java source tests: %w", err)
	}
	sort.Slice(manifest.Cases, func(i, j int) bool { return manifest.Cases[i].SourceFile < manifest.Cases[j].SourceFile })
	return manifest, nil
}

func sourceTestKind(relative string) (string, bool) {
	relative = strings.TrimPrefix(filepath.ToSlash(relative), "./")
	if strings.HasPrefix(relative, "examples/") {
		if strings.Contains(relative, "/src/test/java/") {
			return "example-test", true
		}
		if strings.Contains(relative, "/src/main/java/") {
			return "example-source", true
		}
		return "", false
	}
	if strings.HasPrefix(relative, "regression-run/") && strings.Contains(relative, "/src/test/java/") {
		return "regression-entry", true
	}
	if strings.Contains(relative, "/src/test/java/") {
		if strings.HasPrefix(relative, "esperio/") || strings.HasPrefix(relative, "esperio-") {
			return "esperio-unit", true
		}
		return "core-unit", true
	}
	return "", false
}

func moduleFromPath(relative string) string {
	parts := strings.Split(relative, "/")
	if len(parts) == 0 {
		return ""
	}
	for index, part := range parts {
		if part == "src" && index > 0 {
			return parts[index-1]
		}
	}
	return ""
}
