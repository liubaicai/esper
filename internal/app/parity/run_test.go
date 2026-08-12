package parity

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReplaysCheckedInScenario(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "stage1-length-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-scenario", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version":"esper-parity/v1"`) || !strings.Contains(stdout.String(), `"statement":"parity-stage1"`) {
		t.Fatalf("trace output = %s", stdout.String())
	}
}

func TestRunHelpSucceeds(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, stderr = %q", code, stderr.String())
	}
}
