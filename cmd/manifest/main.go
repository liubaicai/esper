package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/liubaicai/esper/compat"
)

func main() {
	sourceRoot := flag.String("source", `D:\Code\soc\esper`, "Esper Java source root")
	commit := flag.String("commit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c", "pinned Java commit")
	out := flag.String("out", "", "output JSON path; stdout when empty")
	flag.Parse()

	manifest := compat.Manifest{
		Version:    compat.ManifestVersion,
		SourceRoot: filepath.Clean(*sourceRoot),
		JavaCommit: *commit,
		Discovery:  "static-candidate; runtime-enumeration-required",
	}
	err := filepath.WalkDir(*sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".java") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(*sourceRoot, path)
		if err != nil {
			return err
		}
		manifest.Cases = append(manifest.Cases, compat.ScanRegressionSource(relative, string(data))...)
		return nil
	})
	if err != nil {
		fail(err)
	}
	compat.SortManifestCases(manifest.Cases)
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fail(err)
	}
	encoded = append(encoded, '\n')
	if *out == "" {
		_, _ = os.Stdout.Write(encoded)
		return
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
