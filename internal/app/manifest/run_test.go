package manifest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBuildsManifest(t *testing.T) {
	root := t.TempDir()
	source := `package example; class Case implements RegressionExecution {}`
	if err := os.WriteFile(filepath.Join(root, "Case.java"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-source", root, "-commit", "test-commit"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"javaCommit": "test-commit"`) || !strings.Contains(stdout.String(), `"executionClass": "example.Case"`) {
		t.Fatalf("manifest output = %s", stdout.String())
	}
}

func TestRunHelpSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr = %q", code, stderr.String())
	}
}
