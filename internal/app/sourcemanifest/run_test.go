package sourcemanifest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBuildsSourceManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "runtime", "src", "test", "java", "example", "RuntimeTest.java")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("class RuntimeTest {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-source", root, "-commit", "test-commit"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"core-unit": 1`) || !strings.Contains(stdout.String(), `"className": "RuntimeTest"`) {
		t.Fatalf("source manifest output = %s", stdout.String())
	}
}

func TestRunHelpSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr = %q", code, stderr.String())
	}
}
