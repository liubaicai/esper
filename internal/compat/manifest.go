package compat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const ManifestVersion = "esper-manifest/v1"

type Manifest struct {
	Version    string         `json:"version"`
	SourceRoot string         `json:"sourceRoot"`
	JavaCommit string         `json:"javaCommit"`
	Discovery  string         `json:"discovery"`
	Cases      []ManifestCase `json:"cases"`
}

type ManifestCase struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	SourceFile     string   `json:"sourceFile"`
	Package        string   `json:"package,omitempty"`
	OuterClass     string   `json:"outerClass,omitempty"`
	ExecutionClass string   `json:"executionClass"`
	ExecutionName  string   `json:"executionName,omitempty"`
	Suite          string   `json:"suite,omitempty"`
	Flags          []string `json:"flags,omitempty"`
	Discovery      string   `json:"discovery"`
}

var (
	packagePattern   = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	executionPattern = regexp.MustCompile(`(?s)(?:class|interface)\s+([A-Za-z0-9_$]+)[^{]{0,400}?implements\s+RegressionExecution`)
	namePattern      = regexp.MustCompile(`(?s)\bname\s*\(\s*\)\s*\{[^{}]{0,300}?return\s+"([^"]+)"`)
	flagPattern      = regexp.MustCompile(`RegressionFlag\.([A-Z]+)`)
)

// ScanRegressionSource performs a deliberately conservative static scan. It
// is a candidate inventory only; RegressionRunner instrumentation must later
// enumerate actual parameterized executions and replace/verify these cases.
func ScanRegressionSource(relativePath, source string) []ManifestCase {
	packageName := ""
	if match := packagePattern.FindStringSubmatch(source); len(match) == 2 {
		packageName = match[1]
	}
	outerClass := filepath.Base(relativePath)
	outerClass = strings.TrimSuffix(outerClass, filepath.Ext(outerClass))
	flags := uniqueSorted(flagPattern.FindAllStringSubmatch(source, -1))
	matches := executionPattern.FindAllStringSubmatchIndex(source, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make([]ManifestCase, 0, len(matches))
	for ordinal, match := range matches {
		className := source[match[2]:match[3]]
		executionClass := className
		if className != outerClass {
			executionClass = outerClass + "$" + className
		}
		name := ""
		segmentStart := match[0]
		segmentEnd := len(source)
		if ordinal+1 < len(matches) {
			segmentEnd = matches[ordinal+1][0]
		}
		if nameMatch := namePattern.FindStringSubmatch(source[segmentStart:segmentEnd]); len(nameMatch) == 2 {
			name = nameMatch[1]
		}
		idInput := relativePath + "\x00" + className + fmt.Sprintf("\x00%d", ordinal)
		digest := sha256.Sum256([]byte(idInput))
		result = append(result, ManifestCase{
			ID:             "java-" + hex.EncodeToString(digest[:])[:20],
			Kind:           "regression-execution",
			SourceFile:     filepath.ToSlash(relativePath),
			Package:        packageName,
			OuterClass:     outerClass,
			ExecutionClass: packageName + "." + executionClass,
			ExecutionName:  name,
			Flags:          append([]string(nil), flags...),
			Discovery:      "static-candidate",
		})
	}
	return result
}

func uniqueSorted(matches [][]string) []string {
	set := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			set[match[1]] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func SortManifestCases(cases []ManifestCase) {
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].SourceFile != cases[j].SourceFile {
			return cases[i].SourceFile < cases[j].SourceFile
		}
		return cases[i].ID < cases[j].ID
	})
}
