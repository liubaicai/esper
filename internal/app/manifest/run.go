// Package manifest implements the manifest command.
package manifest

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/liubaicai/esper/internal/compat"
)

const defaultJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// Run executes the manifest command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("manifest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sourceRoot := flags.String("source", `D:\Code\soc\esper`, "Esper Java source root")
	commit := flags.String("commit", defaultJavaCommit, "pinned Java commit")
	out := flags.String("out", "", "output JSON path; stdout when empty")
	if err := flags.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}

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
		return fail(stderr, err)
	}
	compat.SortManifestCases(manifest.Cases)
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fail(stderr, err)
	}
	encoded = append(encoded, '\n')
	if *out == "" {
		if _, err := stdout.Write(encoded); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}
