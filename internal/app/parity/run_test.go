package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
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

func TestRunContextHashDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixture(t, func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-hash.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-hash-segmented.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-hash-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextHashDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c2"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixture(t, test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-hash.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-hash-segmented.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-hash-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunFilterWindowAggregateDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "filter-window-aggregate-output.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "filter-window-aggregate.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "filter-window-aggregate-output.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "filter-window-aggregate-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunFilterWindowAggregateDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["total"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["total"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "filter-window-aggregate-output.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "filter-window-aggregate.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "filter-window-aggregate-output.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "filter-window-aggregate-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextHashDiffRejectsMissingJavaTrace(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-hash-segmented.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-hash-diff",
		"-scenario", scenarioPath,
		"-java-trace", filepath.Join(t.TempDir(), "missing-java-trace.json"),
	}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "missing-java-trace.json") {
		t.Fatalf("missing Java trace exit code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunContextHashDiffRejectsTraceIdentityMismatch(t *testing.T) {
	javaTracePath := writeJavaTraceFixture(t, func(trace *compat.Trace) {
		trace.ID = "different-scenario"
	})
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-hash-segmented.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-hash-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
	}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "identity does not match") {
		t.Fatalf("identity mismatch exit code=%d stderr=%q", code, stderr.String())
	}
}

func writeJavaTraceFixture(t *testing.T, mutate func(*compat.Trace)) string {
	t.Helper()
	return writeJavaTraceFixtureFromEvidence(t, filepath.Join("..", "..", "..", "testdata", "parity", "context-hash-segmented.evidence.json"), mutate)
}

func writeJavaTraceFixtureFromEvidence(t *testing.T, evidenceFixture string, mutate func(*compat.Trace)) string {
	t.Helper()
	file, err := os.Open(evidenceFixture)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	mutate(&evidence.JavaTrace)
	javaTrace, err := json.Marshal(evidence.JavaTrace)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "java-trace.json")
	if err := os.WriteFile(path, javaTrace, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeJavaTraceFixtureFromTrace(t *testing.T, traceFixture string, mutate func(*compat.Trace)) string {
	t.Helper()
	file, err := os.Open(traceFixture)
	if err != nil {
		t.Fatal(err)
	}
	trace, loadErr := compat.LoadTrace(file)
	closeErr := file.Close()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	mutate(&trace)
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "java-trace.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunJoinLengthWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "join-length-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "join-length-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "join-length-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "join-length-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunJoinLengthWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["amount"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["amount"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "join-length-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "join-length-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "join-length-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "join-length-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunOutputPolicyDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "output-policy-iterator.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "output-policy.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "output-policy-iterator.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "output-policy-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunOutputPolicyDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["total"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["total"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "output-policy-iterator.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "output-policy.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "output-policy-iterator.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "output-policy-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunPatternTimerDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "pattern-timer-interval.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "pattern-timer.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "pattern-timer-interval.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "pattern-timer-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunPatternTimerDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["id"] = "MUTATED"
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["id"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:13Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "pattern-timer-interval.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "pattern-timer.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "pattern-timer-interval.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "pattern-timer-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubqueryDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subquery-length-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subquery.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subquery-length-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subquery-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubqueryDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["value"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["value"] = json.Number("1")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subquery-length-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subquery.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subquery-length-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subquery-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunNamedWindowMutationDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "named-window-mutation.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "named-window-mutation.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "named-window-mutation.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "named-window-mutation-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunNamedWindowMutationDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["intPrimitive"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New, trace.Records[0].Old = trace.Records[0].Old, trace.Records[0].New
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "named-window-mutation.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "named-window-mutation.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "named-window-mutation.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "named-window-mutation-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunTableMutationDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "table-mutation.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "table-mutation.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "table-mutation.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "table-mutation-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunTableMutationDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["value"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New, trace.Records[2].Old = trace.Records[2].Old, trace.Records[2].New
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "table-mutation.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "table-mutation.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "table-mutation.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "table-mutation-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunVariableDeployDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "variable-deploy.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "variable-deploy.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "variable-deploy.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "variable-deploy-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunVariableDeployDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["var1RTC"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "statement-mix",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].Statement, trace.Records[3].Statement = trace.Records[3].Statement, trace.Records[2].Statement
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "variable-deploy.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "variable-deploy.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "variable-deploy.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "variable-deploy-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextOutputDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-output-termination.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-output.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-output-termination.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-output-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextOutputDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "2002-05-01T08:01:00Z"
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c2"] = fields["c1"]
				delete(fields, "c1")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-output-termination.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-output.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-output-termination.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-output-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunDeploymentRestartDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "deployment-restart-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "deployment-restart.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "deployment-restart-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "deployment-restart-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunDeploymentRestartDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["s0volume"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "case-mix",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case, trace.Records[4].Case = trace.Records[4].Case, trace.Records[0].Case
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "deployment-restart-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "deployment-restart.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "deployment-restart-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "deployment-restart-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunHighCardinalityDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "high-cardinality-context.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "high-cardinality.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "high-cardinality-context.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "high-cardinality-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunHighCardinalityDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["col1"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "partition-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[40].Partitions[0].Key = "key:WRONG"
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "high-cardinality-context.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "high-cardinality.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "high-cardinality-context.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "high-cardinality-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "time-window-long-running.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "time-window-long-running.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["value"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New, trace.Records[3].Old = trace.Records[3].Old, trace.Records[3].New
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "time-window-long-running.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "time-window-long-running.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunDataflowConnectorDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "dataflow-connector-output.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "dataflow-connector.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "dataflow-connector-output.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "dataflow-connector-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunDataflowConnectorDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["p1"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["p2"] = fields["p1"]
				delete(fields, "p1")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "dataflow-connector-output.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "dataflow-connector.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "dataflow-connector-output.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "dataflow-connector-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunOutputAfterDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "output-after-last.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "output-after.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "output-after-last.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "output-after-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunOutputAfterDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["thesum"] = json.Number("999")
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records, trace.Records[0])
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["total"] = fields["thesum"]
				delete(fields, "thesum")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "output-after-last.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "output-after.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "output-after-last.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "output-after-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New, trace.Records[0].Old = trace.Records[0].Old, trace.Records[0].New
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputEverySortedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every-sorted.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-every-sorted.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every-sorted.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-every-sorted-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputEverySortedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-null-placeholder",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Old[4].Fields["c2"] = json.Number("40")
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records, trace.Records[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every-sorted.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-every-sorted.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-every-sorted.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-every-sorted-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputLastDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-last.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-last-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputLastDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[2].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-null-placeholder",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Old[2].Fields["c2"] = json.Number("40")
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records, trace.Records[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-last.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-last-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputLastSortedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-sorted.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-last-sorted.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-sorted.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-last-sorted-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputLastSortedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[2].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-null-placeholder",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Old[3].Fields["c2"] = json.Number("40")
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records, trace.Records[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-sorted.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-last-sorted.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-sorted.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-last-sorted-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputFirstDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-first.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-first-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputFirstDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].Old[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "expiry-new-rows",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New = nil
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-first.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-first-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputFirstSortedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-sorted.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-first-sorted.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-sorted.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-first-sorted-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputFirstSortedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].Old[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New[0], trace.Records[7].New[1] = trace.Records[7].New[1], trace.Records[7].New[0]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-sorted.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-first-sorted.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-sorted.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-first-sorted-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputSnapshotOrderLimitDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot-order-limit.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-snapshot-order-limit.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot-order-limit.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-snapshot-order-limit-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputSnapshotOrderLimitDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[1].Fields["c1"] = json.Number("999")
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0], trace.Records[0].New[1] = trace.Records[0].New[1], trace.Records[0].New[0]
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = append(trace.Records[0].New, trace.Records[0].New[0])
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x0"] = fields["c0"]
				delete(fields, "c0")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot-order-limit.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-snapshot-order-limit.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot-order-limit.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-snapshot-order-limit-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputSnapshotDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-snapshot.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-snapshot-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputSnapshotDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0], trace.Records[5].New[1] = trace.Records[5].New[1], trace.Records[5].New[0]
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].New = append(trace.Records[6].New, trace.Records[6].New[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-snapshot.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-snapshot.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-snapshot-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputLastMarketDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-market.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-last-market.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-market.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-last-market-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputLastMarketDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[1], trace.Records[5].New[2] = trace.Records[5].New[2], trace.Records[5].New[1]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Old[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-market.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-last-market.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-last-market.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-last-market-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputFirstMarketDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-market.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-first-market.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-market.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-first-market-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputFirstMarketDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].New[0], trace.Records[10].New[1] = trace.Records[10].New[1], trace.Records[10].New[0]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].Old[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-market.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-first-market.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-market.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-first-market-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputNoLimitMarketDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-no-limit-market.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-no-limit-market.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-no-limit-market.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-no-limit-market-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputNoLimitMarketDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[11].New[1], trace.Records[11].New[2] = trace.Records[11].New[2], trace.Records[11].New[1]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].Old[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-no-limit-market.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-no-limit-market.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-no-limit-market.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-no-limit-market-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputDefaultMarketDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-default-market.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-default-market.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-default-market.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-default-market-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputDefaultMarketDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[2], trace.Records[5].New[3] = trace.Records[5].New[3], trace.Records[5].New[2]
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Old[0].Fields["sum(price)"] = float64(999)
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-default-market.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-default-market.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-default-market.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-default-market-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputAllDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-all.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-all-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputAllDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[1], trace.Records[4].New[2] = trace.Records[4].New[2], trace.Records[4].New[1]
			},
		},
		{
			name: "empty-group-rows",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New = trace.Records[4].New[:2]
				trace.Records[4].Old = trace.Records[4].Old[:2]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-all.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-all-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputAllSortedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all-sorted.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-all-sorted.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all-sorted.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-all-sorted-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputAllSortedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[2], trace.Records[4].New[3] = trace.Records[4].New[3], trace.Records[4].New[2]
			},
		},
		{
			name: "empty-group-rows",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New = trace.Records[4].New[:4]
				trace.Records[4].Old = trace.Records[4].Old[:4]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all-sorted.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-all-sorted.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-all-sorted.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-all-sorted-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupOutputFirstHavingDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-having.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-output-first-having.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-having.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-output-first-having-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRollupOutputFirstHavingDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "old-equals-new",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old[0].Fields["c2"] = json.Number("999")
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].New = append(trace.Records[6].New, trace.Records[6].New[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:02Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-having.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-output-first-having.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-output-first-having.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-output-first-having-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateDefaultDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-default.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-default.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-default.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-default-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateDefaultDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["mySum"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0], trace.Records[0].New[1] = trace.Records[0].New[1], trace.Records[0].New[0]
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New = append(trace.Records[1].New, trace.Records[1].New[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-default.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-default.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-default.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-default-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateLastDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-last.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-last-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateLastDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["mySum"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0], trace.Records[1].New[1] = trace.Records[1].New[1], trace.Records[1].New[0]
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = append(trace.Records[0].New, trace.Records[0].New[0])
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-last.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-last-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateNoOutputDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-no-output.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-no-output.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-no-output.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-no-output-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateNoOutputDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["mySum"] = float64(999)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = append(trace.Records[0].New, trace.Records[0].New[0])
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["total"] = fields["mySum"]
				delete(fields, "mySum")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-no-output.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-no-output.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-no-output.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-no-output-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunMatchRecognizeDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "match-recognize-simple.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "match-recognize.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "match-recognize-simple.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "match-recognize-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunMatchRecognizeDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["a_string"] = "WRONG"
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c0"] = fields["a_string"]
				delete(fields, "a_string")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "match-recognize-simple.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "match-recognize.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "match-recognize-simple.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "match-recognize-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunUnidirectionalJoinDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "unidirectional-aggregate-join.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "unidirectional-join.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "unidirectional-aggregate-join.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "unidirectional-join-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunUnidirectionalJoinDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["cnt"] = int64(99)
			},
		},
		{
			name: "old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old[0].Fields["cnt"] = int64(8)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c0"] = fields["symbol"]
				delete(fields, "symbol")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "unidirectional-aggregate-join.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "unidirectional-join.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "unidirectional-aggregate-join.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "unidirectional-join-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunOutputFirstHavingDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "output-first-having.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "output-first-having.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "output-first-having.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "output-first-having-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunOutputFirstHavingDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["doublePrimitive"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[4].New[0].Fields
				fields["c0"] = fields["val0"]
				delete(fields, "val0")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].Time = "1970-01-01T00:00:02Z"
			},
		},
		{
			name: "case-order",
			mutate: func(trace *compat.Trace) {
				timeRecord := trace.Records[4]
				copy(trace.Records[1:5], trace.Records[0:4])
				trace.Records[0] = timeRecord
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "output-first-having.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "output-first-having.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "output-first-having.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "output-first-having-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeyedSubqueryDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-keyed-subquery.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-keyed-subquery.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-keyed-subquery.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-keyed-subquery-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeyedSubqueryDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["val0"] = "WRONG"
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["val0"] = "not-null"
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c0"] = fields["theString"]
				delete(fields, "theString")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-keyed-subquery.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-keyed-subquery.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-keyed-subquery.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-keyed-subquery-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRowRecogAggregationDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rowrecog-aggregation.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rowrecog-aggregation.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rowrecog-aggregation.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rowrecog-aggregation-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunRowRecogAggregationDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["maxb"] = 99
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["firstb"] = "not-null"
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "snapshot-row-order",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[3].New
				fields[0], fields[1] = fields[1], fields[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c0"] = fields["a_string"]
				delete(fields, "a_string")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rowrecog-aggregation.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rowrecog-aggregation.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rowrecog-aggregation.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rowrecog-aggregation-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetGroupedTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-grouped-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-grouped-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-grouped-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-grouped-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetGroupedTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c0"] = fields["symbol"]
				delete(fields, "symbol")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "expiry-record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-grouped-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-grouped-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-grouped-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-grouped-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetRowPerGroupSimpleDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-row-per-group-simple.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-row-per-group-simple.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-row-per-group-simple.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-row-per-group-simple-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetRowPerGroupSimpleDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = 99
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["c0"] = fields["c0"]
				delete(fields, "c0")
				fields["x0"] = "E1"
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "group-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = "WRONG"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-row-per-group-simple.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-row-per-group-simple.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-row-per-group-simple.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-row-per-group-simple-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "expiry-old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateLastTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-last-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-last-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateLastTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "expiry-old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "expiry-old-order",
			mutate: func(trace *compat.Trace) {
				for index := len(trace.Records) - 1; index >= 0; index-- {
					if len(trace.Records[index].Old) == 3 {
						old := trace.Records[index].Old
						old[0], old[1] = old[1], old[0]
						return
					}
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-last-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-last-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-last-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateFirstTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-first-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-first-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-first-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-first-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateFirstTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "pure-expiry-new-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "expiry-new-value",
			mutate: func(trace *compat.Trace) {
				last := trace.Records[len(trace.Records)-1]
				last.New[0].Fields["sum(price)"] = float64(99)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-first-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-first-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-first-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-first-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateSnapshotTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-snapshot-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-snapshot-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-snapshot-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-snapshot-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateSnapshotTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[1].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "snapshot-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[len(trace.Records)-1].New = trace.Records[len(trace.Records)-1].New[:len(trace.Records[len(trace.Records)-1].New)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-snapshot-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-snapshot-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-snapshot-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-snapshot-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateAllEventsDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-events.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-all-events.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-events.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-all-events-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateAllEventsDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["mySum"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0], trace.Records[1].New[1] = trace.Records[1].New[1], trace.Records[1].New[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["mySum"]
				delete(fields, "mySum")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "re-emitted-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New = trace.Records[1].New[:len(trace.Records[1].New)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-events.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-all-events.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-events.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-all-events-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetHavingEveryEventsDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-having-every-events.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-having-every-events.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-having-every-events.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-having-every-events-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetHavingEveryEventsDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sumprice"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0], trace.Records[1].New[1] = trace.Records[1].New[1], trace.Records[1].New[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sumprice"]
				delete(fields, "sumprice")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "expiry-old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[len(trace.Records)-1].Old = trace.Records[len(trace.Records)-1].Old[:len(trace.Records[len(trace.Records)-1].Old)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-having-every-events.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-having-every-events.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-having-every-events.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-having-every-events-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateMaxTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-max-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-max-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-max-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-max-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateMaxTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["maxVol"] = float64(99)
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["maxVol"]
				delete(fields, "maxVol")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:00Z"
			},
		},
		{
			name: "old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old = trace.Records[0].Old[:1]
			},
		},
		{
			name: "old-null-to-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old[0].Fields["maxVol"] = float64(1)
			},
		},
		{
			name: "new-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = trace.Records[0].New[:1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-max-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-max-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-max-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-max-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateJoinDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-join.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-join-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateJoinDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[2], trace.Records[3] = trace.Records[3], trace.Records[2]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "pure-expiry-new-value",
			mutate: func(trace *compat.Trace) {
				last := trace.Records[len(trace.Records)-1]
				last.New[0].Fields["sum(price)"] = float64(99)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-join.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-join-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateAllTimeWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-time-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-all-time-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-time-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-all-time-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateAllTimeWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0], trace.Records[1].New[1] = trace.Records[1].New[1], trace.Records[1].New[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "old-value",
			mutate: func(trace *compat.Trace) {
				record := trace.Records[5]
				record.Old[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "re-emitted-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New = trace.Records[2].New[:len(trace.Records[2].New)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-time-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-all-time-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-time-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-all-time-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateAllHavingDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-having.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-all-having.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-having.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-all-having-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateAllHavingDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[3] = trace.Records[3], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["sum(price)"]
				delete(fields, "sum(price)")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "expiry-old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].Old = nil
			},
		},
		{
			name: "expiry-new-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["sum(price)"] = float64(99)
			},
		},
		{
			name: "suppressed-expiry-inserted",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records, trace.Records[3])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-having.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-all-having.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-all-having.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-all-having-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateJoinEventsDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-events.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-join-events.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-events.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-join-events-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateJoinEventsDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["mySum"] = float64(99)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0], trace.Records[5].New[1] = trace.Records[5].New[1], trace.Records[5].New[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["mySum"]
				delete(fields, "mySum")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "all-reemit-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New = trace.Records[3].New[:len(trace.Records[3].New)-1]
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-events.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-join-events.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-events.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-join-events-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateJoinSortWindowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-sort-window.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-join-sort-window.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-sort-window.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-join-sort-window-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateJoinSortWindowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["maxVol"] = float64(99)
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["maxVol"]
				delete(fields, "maxVol")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:00Z"
			},
		},
		{
			name: "new-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = trace.Records[0].New[:1]
			},
		},
		{
			name: "old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old = nil
			},
		},
		{
			name: "old-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old[0].Fields["maxVol"] = float64(2)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-sort-window.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-join-sort-window.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-join-sort-window.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-join-sort-window-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateMultikeyDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-multikey.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-multikey.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-multikey.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-multikey-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateMultikeyDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["thesum"] = int64(99)
			},
		},
		{
			name: "group-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["longPrimitive"] = int64(9)
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["x"] = fields["intPrimitive"]
				delete(fields, "intPrimitive")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:00Z"
			},
		},
		{
			name: "new-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = trace.Records[0].New[:2]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[0].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:3]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].Case = "multikey-last"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-multikey.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-multikey.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-multikey.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-multikey-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateGroupOutputDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-group-output.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-group-output.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-group-output.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-group-output-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateGroupOutputDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["intp"] = int64(99)
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[2].New[0].Fields
				fields["x"] = fields["theString"]
				delete(fields, "theString")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:00Z"
			},
		},
		{
			name: "new-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = trace.Records[0].New[:2]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[2].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:12]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[12].Case = "first-simple"
			},
		},
		{
			name: "old-row-added",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Old = append(trace.Records[0].Old, compat.ResultRecord{
					Kind:   "row",
					Fields: map[string]any{"intp": int64(31), "theString": "E3"},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-group-output.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-group-output.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-group-output.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-group-output-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateLimitSnapshotDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-limit-snapshot.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-limit-snapshot.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-limit-snapshot.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-limit-snapshot-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateLimitSnapshotDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["sumprice"] = float64(99)
			},
		},
		{
			name: "boundary-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New = trace.Records[1].New[1:]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[1].New[0].Fields
				fields["x"] = fields["symbol"]
				delete(fields, "symbol")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Time = "1970-01-01T00:00:09Z"
			},
		},
		{
			name: "new-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New = trace.Records[1].New[:4]
			},
		},
		{
			name: "row-order",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[1].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:5]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].Case = "limit-snapshot"
			},
		},
		{
			name: "old-row-added",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Old = append(trace.Records[1].Old, compat.ResultRecord{
					Kind:   "row",
					Fields: map[string]any{"sumprice": float64(34), "symbol": "s0", "volume": int64(1)},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-limit-snapshot.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-limit-snapshot.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-limit-snapshot.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-limit-snapshot-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateFilteredDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-filtered.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-filtered.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-filtered.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-filtered-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateFilteredDiffRejectsTraceMutations(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-filtered.evidence.json"),
		func(trace *compat.Trace) {
			trace.Records[2].New[0].Fields["pct"] = json.Number("999")
		})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-filtered.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-filtered.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-filtered-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("mutated aggregate filtered trace unexpectedly passed")
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if output.Status != "different" || len(output.Differences) == 0 {
		t.Fatalf("mutation evidence = %#v", output)
	}
}

func TestRunExprCoreBitwiseDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-bitwise.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-bitwise.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-bitwise.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-bitwise-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreBitwiseDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["myFourthProperty"] = json.Number("8")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["wrongProperty"] = fields["myFirstProperty"]
				delete(fields, "myFirstProperty")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-bitwise.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-bitwise.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-bitwise.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-bitwise-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated bitwise trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunExprCoreLogicalDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-logical.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-logical.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-logical.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-logical-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestExprCoreLogicalCheckedInEvidenceMatchesIndependentTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-logical.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-logical.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreLogicalJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreLogicalJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreLogicalJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreLogicalJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from independent trace: %#v", differences)
	}

	scenarioPath := filepath.Join(root, "expr-core-logical.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-logical", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreLogicalDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = false
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["wrongProperty"] = fields["c0"]
				delete(fields, "c0")
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New[0].Fields["c0"] = true
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-logical.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-logical.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-logical.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-logical-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated logical trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunExprCoreCoalesceDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-coalesce.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-coalesce.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-coalesce.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-coalesce-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestExprCoreCoalesceCheckedInEvidenceMatchesIndependentTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-coalesce.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-coalesce.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreCoalesceJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreCoalesceJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreCoalesceJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreCoalesceJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from independent trace: %#v", differences)
	}

	scenarioPath := filepath.Join(root, "expr-core-coalesce.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-coalesce", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreCoalesceDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{{
		name: "bean-value",
		mutate: func(trace *compat.Trace) {
			trace.Records[0].New[0].Fields["myString"] = "wrong"
		},
	}, {
		name: "numeric-value",
		mutate: func(trace *compat.Trace) {
			trace.Records[2].New[0].Fields["result"] = json.Number("99")
		},
	}, {
		name: "null-state",
		mutate: func(trace *compat.Trace) {
			trace.Records[14].New[0].Fields["c0"] = true
		},
	}, {
		name: "record-order",
		mutate: func(trace *compat.Trace) {
			trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
		},
	}, {
		name: "record-removal",
		mutate: func(trace *compat.Trace) {
			trace.Records = trace.Records[:len(trace.Records)-1]
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-coalesce.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-coalesce.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-coalesce.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-coalesce-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated coalesce trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunExprCoreRelOpDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-relop.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-relop.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-relop.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-relop-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreRelOpRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-relop.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreRelOpCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreRelOpScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed relop scenario unexpectedly replayed")
	}
}

func TestExprCoreRelOpCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-relop.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-relop.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreRelOpJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreRelOpJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreRelOpJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreRelOpJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}

	scenarioPath := filepath.Join(root, "expr-core-relop.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-relop", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreRelOpDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = true
			},
		},
		{
			name: "big-number-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].New[0].Fields["c1"] = true
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[27].New[0].Fields["c0"] = true
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-relop.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-relop.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-relop.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-relop-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated relop trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreLikeRegexpDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-like-regexp.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-like-regexp.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-like-regexp.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-like-regexp-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreLikeRegexpRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-like-regexp.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreLikeRegexpCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreLikeRegexpScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed like/regexp scenario unexpectedly replayed")
	}
}

func TestExprCoreLikeRegexpCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-like-regexp.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-like-regexp.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreLikeRegexpJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreLikeRegexpJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreLikeRegexpJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreLikeRegexpJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}

	scenarioPath := filepath.Join(root, "expr-core-like-regexp.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-like-regexp", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreLikeRegexpDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = true
			},
		},
		{
			name: "full-match-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[16].New[0].Fields["c0"] = true
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c0"] = true
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-like-regexp.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-like-regexp.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-like-regexp.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-like-regexp-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated like/regexp trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreInBetweenDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-in-between.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-in-between.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-in-between.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-in-between-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreInBetweenRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-in-between.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreInBetweenCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreInBetweenScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed IN/BETWEEN scenario unexpectedly replayed")
	}

	payloadMalformed := scenario
	payloadMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	payloadMalformed.Steps[1].Payload = json.RawMessage(`{"doubleBoxed":9}`)
	if _, err := runExprCoreInBetweenScenario(context.Background(), payloadMalformed); err == nil {
		t.Fatal("mutated IN/BETWEEN payload unexpectedly replayed")
	}
}

func TestExprCoreInBetweenCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-in-between.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-in-between.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreInBetweenJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreInBetweenJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreInBetweenJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreInBetweenJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if len(javaTrace.Records) != 164 {
		t.Fatalf("checked-in Java trace records = %d, want 164", len(javaTrace.Records))
	}
	statementCounts := map[string]int{}
	for _, record := range javaTrace.Records {
		statementCounts[record.Statement]++
	}
	if !reflect.DeepEqual(statementCounts, map[string]int{"s0": 159, "s1": 1, "s2": 4}) {
		t.Fatalf("checked-in statement counts = %#v", statementCounts)
	}

	scenarioPath := filepath.Join(root, "expr-core-in-between.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-in-between", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreInBetweenDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = true
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["c0"] = true
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "statement-lifecycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[159].Statement = "s0"
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-in-between.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-in-between.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-in-between.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-in-between-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated IN/BETWEEN trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreEqualsIsDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-equals-is.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-equals-is.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-equals-is.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-equals-is-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreEqualsIsRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-equals-is.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreEqualsIsCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreEqualsIsScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed equality scenario unexpectedly replayed")
	}

	payloadMalformed := scenario
	payloadMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	payloadMalformed.Steps[1].Payload = json.RawMessage(`{"intPrimitive":9,"longPrimitive":1}`)
	if _, err := runExprCoreEqualsIsScenario(context.Background(), payloadMalformed); err == nil {
		t.Fatal("mutated equality payload unexpectedly replayed")
	}
}

func TestExprCoreEqualsIsCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-equals-is.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-equals-is.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreEqualsIsJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreEqualsIsJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreEqualsIsJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreEqualsIsJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if len(javaTrace.Records) != 8 {
		t.Fatalf("checked-in Java trace records = %d, want 8", len(javaTrace.Records))
	}
	statementCounts := map[string]int{}
	for _, record := range javaTrace.Records {
		statementCounts[record.Statement]++
	}
	if !reflect.DeepEqual(statementCounts, map[string]int{"s0": 8}) {
		t.Fatalf("checked-in statement counts = %#v", statementCounts)
	}

	scenarioPath := filepath.Join(root, "expr-core-equals-is.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-equals-is", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreEqualsIsDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "coercion-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = false
			},
		},
		{
			name: "array-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["c6"] = true
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].New[0].Fields["c0"] = true
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "case-lifecycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].Case = "equals-coercion"
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-equals-is.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-equals-is.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-equals-is.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-equals-is-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated equality trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreCaseDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-case.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-case.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-case.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-case-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreCaseRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-case.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreCaseScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed CASE scenario unexpectedly replayed")
	}

	payloadMalformed := scenario
	payloadMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	payloadMalformed.Steps[1].Payload = json.RawMessage(`{"symbol":"CSCO","volume":4001,"price":0}`)
	if _, err := runExprCoreCaseScenario(context.Background(), payloadMalformed); err == nil {
		t.Fatal("mutated CASE payload unexpectedly replayed")
	}

	metadataMalformed := scenario
	metadataMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	metadataMalformed.Steps[1].Case = "case-with-else"
	if _, err := runExprCoreCaseScenario(context.Background(), metadataMalformed); err == nil {
		t.Fatal("CASE send metadata unexpectedly replayed")
	}
}

func TestExprCoreCaseCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-case.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-case.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreCaseJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreCaseJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreCaseJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreCaseJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if len(javaTrace.Records) != 9 {
		t.Fatalf("checked-in Java trace records = %d, want 9", len(javaTrace.Records))
	}
	statementCounts := map[string]int{}
	for _, record := range javaTrace.Records {
		statementCounts[record.Statement]++
	}
	if !reflect.DeepEqual(statementCounts, map[string]int{"s0": 9}) {
		t.Fatalf("checked-in statement counts = %#v", statementCounts)
	}

	scenarioPath := filepath.Join(root, "expr-core-case.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-case", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreCaseDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "searched-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["p1"] = float64(61)
			},
		},
		{
			name: "simple-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["c0"] = float64(5)
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[2], trace.Records[3] = trace.Records[3], trace.Records[2]
			},
		},
		{
			name: "case-lifecycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].Case = "case-with-else"
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-case.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-case.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-case.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-case-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated CASE trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreInstanceOfDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-instanceof.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-instanceof.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-instanceof.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-instanceof-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreInstanceOfRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-instanceof.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreInstanceOfCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreInstanceOfScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed instanceof scenario unexpectedly replayed")
	}

	payloadMalformed := scenario
	payloadMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	payloadMalformed.Steps[10].Payload = json.RawMessage(`{"itemType":"float32","itemValue":101}`)
	if _, err := runExprCoreInstanceOfScenario(context.Background(), payloadMalformed); err == nil {
		t.Fatal("mutated instanceof payload unexpectedly replayed")
	}

	metadataMalformed := scenario
	metadataMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	metadataMalformed.Steps[1].Case = "instanceof-simple"
	if _, err := runExprCoreInstanceOfScenario(context.Background(), metadataMalformed); err == nil {
		t.Fatal("instanceof send metadata unexpectedly replayed")
	}
}

func TestExprCoreInstanceOfCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-instanceof.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-instanceof.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreInstanceOfJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreInstanceOfJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreInstanceOfJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreInstanceOfJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if len(javaTrace.Records) != 17 {
		t.Fatalf("checked-in Java trace records = %d, want 17", len(javaTrace.Records))
	}
	statementCounts := map[string]int{}
	for _, record := range javaTrace.Records {
		statementCounts[record.Statement]++
	}
	if !reflect.DeepEqual(statementCounts, map[string]int{"s0": 17}) {
		t.Fatalf("checked-in statement counts = %#v", statementCounts)
	}

	scenarioPath := filepath.Join(root, "expr-core-instanceof.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-instanceof", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreInstanceOfDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "dynamic-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New[0].Fields["t2"] = false
			},
		},
		{
			name: "hierarchy-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[12].New[0].Fields["t3"] = true
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "case-lifecycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].Case = "instanceof-simple"
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-instanceof.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-instanceof.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-instanceof.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-instanceof-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated instanceof trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreTypeNameDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-type-name.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-type-name.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-type-name.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-type-name-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreTypeNameRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-type-name.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	malformed := scenario
	malformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	malformed.Steps[1], malformed.Steps[2] = malformed.Steps[2], malformed.Steps[1]
	if _, err := runExprCoreTypeNameScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed type-name scenario unexpectedly replayed")
	}
	payloadMalformed := scenario
	payloadMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	payloadMalformed.Steps[1].Payload = json.RawMessage(`{"shape":"insidearr"}`)
	if _, err := runExprCoreTypeNameScenario(context.Background(), payloadMalformed); err == nil {
		t.Fatal("mutated type-name payload unexpectedly replayed")
	}
	metadataMalformed := scenario
	metadataMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	metadataMalformed.Steps[1].Case = "type-name-fragment-object-array"
	if _, err := runExprCoreTypeNameScenario(context.Background(), metadataMalformed); err == nil {
		t.Fatal("type-name send metadata unexpectedly replayed")
	}
}

func TestExprCoreTypeNameCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-type-name.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-type-name.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreTypeNameJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreTypeNameJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreTypeNameJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreTypeNameJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if len(javaTrace.Records) != 18 {
		t.Fatalf("checked-in Java trace records = %d, want 18", len(javaTrace.Records))
	}
	statementCounts := map[string]int{}
	caseCounts := map[string]int{}
	for _, record := range javaTrace.Records {
		statementCounts[record.Statement]++
		caseCounts[record.Case]++
	}
	if !reflect.DeepEqual(statementCounts, map[string]int{"s0": 18}) {
		t.Fatalf("checked-in statement counts = %#v", statementCounts)
	}
	for _, caseName := range exprCoreTypeNameCaseOrder {
		if caseCounts[caseName] != 3 {
			t.Fatalf("checked-in case %q count = %d, want 3", caseName, caseCounts[caseName])
		}
	}
	scenarioPath := filepath.Join(root, "expr-core-type-name.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-type-name", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreTypeNameDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "fragment-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["t0"] = "WrongSchema"
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "case-lifecycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].Case = exprCoreTypeNameCaseOrder[0]
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-type-name.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-type-name.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-type-name.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-type-name-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated type-name trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreExistsCastDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-exists-cast.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-exists-cast.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-exists-cast.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-exists-cast-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreExistsCastRejectsMalformedScenarioShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-exists-cast.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(file)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	caseMarkers := make([]compat.Step, 0, len(exprCoreExistsCastCaseOrder))
	var sends []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseMarkers = append(caseMarkers, step)
		} else {
			sends = append(sends, step)
		}
	}
	malformed := scenario
	malformed.Steps = append(caseMarkers, sends...)
	if _, err := runExprCoreExistsCastScenario(context.Background(), malformed); err == nil {
		t.Fatal("malformed exists/cast scenario unexpectedly replayed")
	}

	payloadMalformed := scenario
	payloadMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	payloadMalformed.Steps[3].Payload = json.RawMessage(`{"shape":"complex"}`)
	if _, err := runExprCoreExistsCastScenario(context.Background(), payloadMalformed); err == nil {
		t.Fatal("mutated exists/cast payload unexpectedly replayed")
	}

	metadataMalformed := scenario
	metadataMalformed.Steps = append([]compat.Step(nil), scenario.Steps...)
	metadataMalformed.Steps[3].Case = "exists-inner"
	if _, err := runExprCoreExistsCastScenario(context.Background(), metadataMalformed); err == nil {
		t.Fatal("exists/cast send metadata unexpectedly replayed")
	}
}

func TestExprCoreExistsCastCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "expr-core-exists-cast.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "expr-core-exists-cast.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.JavaCommit != exprCoreExistsCastJavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, exprCoreExistsCastJavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, exprCoreExistsCastJavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, exprCoreExistsCastJavaExecutions) {
		t.Fatalf("checked-in evidence Java metadata = %#v", evidence)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if len(javaTrace.Records) != 54 {
		t.Fatalf("checked-in Java trace records = %d, want 54", len(javaTrace.Records))
	}
	statementCounts := map[string]int{}
	caseCounts := map[string]int{}
	for _, record := range javaTrace.Records {
		statementCounts[record.Statement]++
		caseCounts[record.Case]++
	}
	if !reflect.DeepEqual(statementCounts, map[string]int{"s0": 54}) {
		t.Fatalf("checked-in statement counts = %#v", statementCounts)
	}
	wantCaseCounts := map[string]int{
		"exists-simple": 1, "exists-inner": 5, "exists-om": 3, "exists-compile": 3,
		"cast-simple": 2, "cast-simple-more-types": 1, "cast-as-parse": 1, "cast-double-null-om": 6,
		"cast-interface": 5, "cast-string-and-null": 6, "cast-boolean": 3, "cast-w-static-type": 1,
		"cast-bigdecimal-bigint": 8, "cast-warray": 2, "cast-warray-soda": 2, "cast-generic": 2,
		"cast-dates-base": 1, "cast-dates-java8": 1, "cast-dates-constant": 1,
	}
	if !reflect.DeepEqual(caseCounts, wantCaseCounts) {
		t.Fatalf("checked-in case counts = %#v", caseCounts)
	}

	scenarioPath := filepath.Join(root, "expr-core-exists-cast.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-core-exists-cast", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestRunExprCoreExistsCastDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = false
			},
		},
		{
			name: "cast-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[14].New[0].Fields["c3"] = "x"
			},
		},
		{
			name: "null-shaped-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["t0"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[2], trace.Records[3] = trace.Records[3], trace.Records[2]
			},
		},
		{
			name: "case-lifecycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].Case = "exists-simple"
			},
		},
		{
			name: "string-and-null-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[30].New[0].Fields["t0"] = "77.777"
			},
		},
		{
			name: "boolean-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[33].New[0].Fields["t1"] = false
			},
		},
		{
			name: "static-type-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[36].New[0].Fields["byteVal"] = "11"
			},
		},
		{
			name: "record-removal",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "bigdecimal-exact-decimal",
			mutate: func(trace *compat.Trace) {
				trace.Records[37].New[0].Fields["c0"] = "2.5"
			},
		},
		{
			name: "bigdecimal-bigint-truncate",
			mutate: func(trace *compat.Trace) {
				trace.Records[38].New[0].Fields["c1"] = "155"
			},
		},
		{
			name: "bigdecimal-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[44].New[0].Fields["c0"] = "0"
			},
		},
		{
			name: "cast-interface-bean-token",
			mutate: func(trace *compat.Trace) {
				trace.Records[22].New[0].Fields["t0"] = "ISupportDImpl"
			},
		},
		{
			name: "warray-string-array-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[45].New[0].Fields["c0"] = []any{"b"}
			},
		},
		{
			name: "warray-support-bean-token",
			mutate: func(trace *compat.Trace) {
				trace.Records[47].New[0].Fields["c4"] = []any{"SupportBean(E1,1)"}
			},
		},
		{
			name: "warray-null-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[46].New[0].Fields["c2"] = []any{"7"}
			},
		},
		{
			name: "warray-3dim-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[48].New[0].Fields["c7"] = []any{[]any{[]any{"1"}}}
			},
		},
		{
			name: "generic-optional-token",
			mutate: func(trace *compat.Trace) {
				trace.Records[49].New[0].Fields["listOfOptionalInteger"] = []any{"Optional[11]"}
			},
		},
		{
			name: "generic-map-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[49].New[0].Fields["mapOfStringAndInteger"] = map[string]any{"k": "21"}
			},
		},
		{
			name: "generic-nested-list-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[49].New[0].Fields["listArray2DimOfString"] = []any{[]any{[]any{"z"}}}
			},
		},
		{
			name: "dates-base-epoch",
			mutate: func(trace *compat.Trace) {
				trace.Records[51].New[0].Fields["c0"] = int64(1273449600001)
			},
		},
		{
			name: "dates-base-month",
			mutate: func(trace *compat.Trace) {
				trace.Records[51].New[0].Fields["c6"] = "5"
			},
		},
		{
			name: "dates-java8-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[52].New[0].Fields["c2"] = "2010-05-10T14:15:17"
			},
		},
		{
			name: "dates-constant-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[53].New[0].Fields["c0"] = int64(1044057600001)
			},
		},
		{
			name: "generic-null-cell",
			mutate: func(trace *compat.Trace) {
				trace.Records[50].New[0].Fields["listOfString"] = []any{"a"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-exists-cast.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-exists-cast.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-exists-cast.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-exists-cast-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated exists/cast trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", evidence)
			}
		})
	}
}

func TestRunExprCoreCurrentTimestampDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-timestamp.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-current-timestamp.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-timestamp.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-current-timestamp-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreCurrentTimestampDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["t2"] = json.Number("1001")
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["timestamp"] = fields["current_timestamp()"]
				delete(fields, "current_timestamp()")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-timestamp.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-current-timestamp.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-timestamp.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-current-timestamp-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated current-timestamp trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunExprCoreCurrentEvaluationContextDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-evaluation-context.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-core-current-evaluation-context.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-evaluation-context.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-core-current-evaluation-context-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunExprCoreCurrentEvaluationContextDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "metadata-value",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields["c0"].(map[string]any)
				fields["statementName"] = "wrong"
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["runtime"] = fields["c2"]
				delete(fields, "c2")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "1970-01-01T00:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-evaluation-context.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-core-current-evaluation-context.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-core-current-evaluation-context.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-core-current-evaluation-context-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated current-evaluation-context trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunExprDTBetweenDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromTrace(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "dt-between.trace.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "dt-between.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "dt-between.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-dt-between-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestExprDTBetweenCheckedInEvidenceMatchesIndependentTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	traceFile, err := os.Open(filepath.Join(root, "dt-between.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	javaTrace, err := compat.LoadTrace(traceFile)
	closeErr := traceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	evidenceFile, err := os.Open(filepath.Join(root, "dt-between.evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(evidenceFile)
	closeErr = evidenceFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from independent trace: %#v", differences)
	}

	scenarioPath := filepath.Join(root, "dt-between.json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := compat.LoadScenario(scenarioFile)
	closeErr = scenarioFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	scenarioJSON, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	evidenceScenarioJSON, err := json.Marshal(evidence.Scenario)
	if err != nil {
		t.Fatal(err)
	}
	var scenarioValue, evidenceScenarioValue any
	if err := json.Unmarshal(scenarioJSON, &scenarioValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(evidenceScenarioJSON, &evidenceScenarioValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenarioValue, evidenceScenarioValue) {
		t.Fatal("checked-in evidence scenario differs from checked-in scenario")
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "expr-dt-between", "-scenario", scenarioPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	goTrace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, goTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
}

func TestDecodeExprDTBetweenPayloadPreservesNullAndMissingFields(t *testing.T) {
	decode := func(payload string) map[string]any {
		t.Helper()
		value, err := decodeExprDTBetweenPayload(compat.Step{
			Op:        "send",
			EventType: "SupportTimeStartEndA",
			Payload:   json.RawMessage(payload),
		})
		if err != nil {
			t.Fatal(err)
		}
		event, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("decoded event type = %T, want map[string]any", value)
		}
		return event
	}

	explicitNull := decode("{\"key\":\"N1\",\"start\":null,\"duration\":0}")
	missing := decode("{\"key\":\"N2\",\"duration\":0}")
	if value, exists := explicitNull["longdateStart"]; !exists || value != nil {
		t.Fatalf("explicit null field = (%#v, %t), want present nil", value, exists)
	}
	if _, exists := missing["longdateStart"]; exists {
		t.Fatal("missing date-time field was materialized in the decoded event")
	}
}

func TestRunExprDTBetweenDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["val0"] = false
			},
		},
		{
			name: "record-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[0].New[0].Fields
				fields["matched"] = fields["val0"]
				delete(fields, "val0")
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Time = "2002-05-30T09:00:01Z"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromTrace(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "dt-between.trace.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "dt-between.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "dt-between.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-dt-between-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutated date-time between trace unexpectedly passed")
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunResultSetAggregateCountSumDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-count-sum.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-count-sum.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-count-sum.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "resultset-aggregate-count-sum-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunResultSetAggregateCountSumDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["countAll"] = int64(99)
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "old-row-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].Old = trace.Records[5].Old[1:]
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].New[0].Fields["mysum"] = int64(100)
			},
		},
		{
			name: "distinct-expiry",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[1].Fields["countDistVol"] = int64(2)
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[25].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "field-name",
			mutate: func(trace *compat.Trace) {
				fields := trace.Records[21].New[0].Fields
				fields["x"] = fields["theString"]
				delete(fields, "theString")
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:33]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[33].Case = "count-one-view"
			},
		},
		{
			name: "snapshot-row-added",
			mutate: func(trace *compat.Trace) {
				trace.Records[25].New = append(trace.Records[25].New, compat.ResultRecord{
					Kind:   "row",
					Fields: map[string]any{"mysum": float64(7), "theString": "C"},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-count-sum.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "resultset-aggregate-count-sum.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "resultset-aggregate-count-sum.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "resultset-aggregate-count-sum-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectAggregatedInExistsAnyAllDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-in-exists-any-all.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-aggregated-in-exists-any-all.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-in-exists-any-all.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-aggregated-in-exists-any-all-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectAggregatedInExistsAnyAllDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = true
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = false
			},
		},
		{
			name: "empty-set-quantifier",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["c0"] = true
			},
		},
		{
			name: "having-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[13].New[0].Fields["c0"] = false
			},
		},
		{
			name: "grouped-empty-in",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].New[0].Fields["c0"] = true
			},
		},
		{
			name: "faf-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[42].New[0].Fields["c0"] = true
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:46]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[46].Case = "grouped-exists"
			},
		},
		{
			name: "time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].Time = "1970-01-01T00:00:01Z"
			},
		},
		{
			name: "sequence",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].Sequence = 99
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-in-exists-any-all.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-aggregated-in-exists-any-all.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-in-exists-any-all.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-aggregated-in-exists-any-all-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectAggregatedSingleValueDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-single-value.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-aggregated-single-value.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-single-value.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-aggregated-single-value-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectAggregatedSingleValueDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c1"] = int64(31)
			},
		},
		{
			name: "null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = int64(0)
			},
		},
		{
			name: "correlated-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[24].New[0].Fields["mycount"] = int64(1)
			},
		},
		{
			name: "filtered-event",
			mutate: func(trace *compat.Trace) {
				trace.Records[30].New[0].Fields["p00"] = "T2"
			},
		},
		{
			name: "having-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[37].New[0].Fields["c0"] = int64(22)
			},
		},
		{
			name: "grouped-scalar",
			mutate: func(trace *compat.Trace) {
				trace.Records[44].New[0].Fields["c0"] = "E1"
			},
		},
		{
			name: "table-having",
			mutate: func(trace *compat.Trace) {
				trace.Records[53].New[0].Fields["c0"] = int64(106)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:56]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[56].Case = "ungrouped-table-having"
			},
		},
		{
			name: "sequence",
			mutate: func(trace *compat.Trace) {
				trace.Records[40].Sequence = 99
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-single-value.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-aggregated-single-value.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-single-value.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-aggregated-single-value-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectInDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-in.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-in.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-in.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-in-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectInDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "value",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["value"] = false
			},
		},
		{
			name: "empty-set-in",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["value"] = true
			},
		},
		{
			name: "eviction-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[13].New[0].Fields["id"] = int64(12)
			},
		},
		{
			name: "expression-projection",
			mutate: func(trace *compat.Trace) {
				trace.Records[24].New[0].Fields["value"] = true
			},
		},
		{
			name: "nullable-match",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].New[0].Fields["id"] = int64(3)
			},
		},
		{
			name: "coercion",
			mutate: func(trace *compat.Trace) {
				trace.Records[27].New[0].Fields["longBoxed"] = int64(97)
			},
		},
		{
			name: "index-shape",
			mutate: func(trace *compat.Trace) {
				trace.Records[35].New[0].Fields["c0"] = "v9"
			},
		},
		{
			name: "not-in-null-row",
			mutate: func(trace *compat.Trace) {
				trace.Records[41].New[0].Fields["intBoxed"] = int64(1)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:51]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[51].Case = "in-select"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-in.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-in.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-in.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-in-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermTemporalFixedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-temporal-fixed.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-temporal-fixed.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-temporal-fixed.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-temporal-fixed-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermTemporalFixedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "correlated-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c3"] = int64(99)
			},
		},
		{
			name: "pattern-sum",
			mutate: func(trace *compat.Trace) {
				for _, record := range trace.Records {
					if record.Case == "pattern-pattern" {
						record.New[0].Fields["c2"] = int64(42)
						break
					}
				}
			},
		},
		{
			name: "every-second-null-state",
			mutate: func(trace *compat.Trace) {
				for _, record := range trace.Records {
					if record.Case == "every-second" {
						record.New[0].Fields["intBoxed"] = int64(7)
						break
					}
				}
			},
		},
		{
			name: "daily-join-time-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[38].New[0].Fields["col3"] = int64(2)
			},
		},
		{
			name: "daily-pattern-time-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:41]
			},
		},
		{
			name: "turned-off-mid-deploy",
			mutate: func(trace *compat.Trace) {
				for _, record := range trace.Records {
					if record.Case == "turned-off" && record.Statement == "B" {
						record.New[0].Fields["theString"] = "mutated"
						break
					}
				}
			},
		},
		{
			name: "turned-on-timer",
			mutate: func(trace *compat.Trace) {
				for _, record := range trace.Records {
					if record.Case == "turned-on" && record.Statement == "B" {
						record.New[0].Fields["intPrimitive"] = int64(3)
						break
					}
				}
			},
		},
		{
			name: "snapshot-removed-record",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:64]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[64].Case = "daily-agg-ungrouped"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-temporal-fixed.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-temporal-fixed.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-temporal-fixed.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-temporal-fixed-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermCorrelatedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-correlated.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-correlated.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-correlated.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-correlated-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermCorrelatedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "pattern-tag-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = int64(11)
			},
		},
		{
			name: "ender-tag-null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c2"] = int64(10)
			},
		},
		{
			name: "timer-termination-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["c1"] = int64(21)
			},
		},
		{
			name: "timer-null-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["c2"] = int64(10)
			},
		},
		{
			name: "filter-row-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["theString"] = "mutated"
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:3]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].Case = "pattern-pattern-correlated"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-correlated.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-correlated.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-correlated.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-correlated-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermWithNowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-with-now.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-with-now.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-with-now.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-with-now-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermWithNowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "termination-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["cnt"] = int64(4)
			},
		},
		{
			name: "empty-cycle-zero",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["cnt"] = int64(1)
			},
		},
		{
			name: "pattern-cycle-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["cnt"] = int64(2)
			},
		},
		{
			name: "pattern-empty-cycle",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["cnt"] = int64(1)
			},
		},
		{
			name: "timer-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].Time = "2002-05-01T00:00:39Z"
			},
		},
		{
			name: "now-no-end-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New[0].Fields["cnt"] = int64(5)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:10]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].Case = "start-stop-now"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-with-now.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-with-now.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-with-now.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-with-now-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermOverlapDurationDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-overlap-duration.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-overlap-duration.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-overlap-duration.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-overlap-duration-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermOverlapDurationDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "cron-partition-sum",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c2"] = int64(19)
			},
		},
		{
			name: "cron-multi-row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0], trace.Records[2].New[1] = trace.Records[2].New[1], trace.Records[2].New[0]
			},
		},
		{
			name: "cron-partition-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[2].Fields["c2"] = int64(11)
			},
		},
		{
			name: "cron-timer-time",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].Time = "2002-05-01T08:04:59.999Z"
			},
		},
		{
			name: "two-context-partition-row",
			mutate: func(trace *compat.Trace) {
				trace.Records[9].New[1].Fields["c3"] = "SB03"
			},
		},
		{
			name: "two-context-sum",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].New[0].Fields["c2"] = int64(15)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:10]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].Case = "crontab-minute"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-overlap-duration.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-overlap-duration.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-overlap-duration.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-overlap-duration-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermKeyedAggregationDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-keyed-aggregation.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-keyed-aggregation.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-keyed-aggregation.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-keyed-aggregation-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermKeyedAggregationDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "partition-isolation",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c1"] = int64(101)
			},
		},
		{
			name: "group-sum",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["c1"] = int64(200)
			},
		},
		{
			name: "group-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c0"] = "G3"
			},
		},
		{
			name: "reinit-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["c1"] = int64(999)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:5]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-keyed-aggregation.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-keyed-aggregation.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-keyed-aggregation.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-keyed-aggregation-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermFilterPatternEndDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-pattern-end.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-filter-pattern-end.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-pattern-end.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-filter-pattern-end-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermFilterPatternEndDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "sequence-end-routing",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["id"] = int64(101)
			},
		},
		{
			name: "terminated-partition-silence",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["id"] = int64(301)
			},
		},
		{
			name: "start-tag-projection",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["c3"] = "S0_2"
			},
		},
		{
			name: "correlated-id-filter",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["c2"] = int64(2)
			},
		},
		{
			name: "multi-row-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0], trace.Records[5].New[1] = trace.Records[5].New[1], trace.Records[5].New[0]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:6]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].Case = "filter-and-pattern"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-pattern-end.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-filter-pattern-end.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-pattern-end.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-filter-pattern-end-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermFilterOperatorsDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-operators.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-filter-operators.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-operators.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-filter-operators-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func recordsOfCase(trace *compat.Trace, caseName string) []compat.TraceRecord {
	var selected []compat.TraceRecord
	for _, record := range trace.Records {
		if record.Case == caseName {
			selected = append(selected, record)
		}
	}
	return selected
}

// recordIndicesOfCase returns the indices in trace.Records whose case label
// matches. Scalar-field mutations must go through the trace slice directly
// because recordsOfCase returns value copies.
func recordIndicesOfCase(trace *compat.Trace, caseName string) []int {
	var indices []int
	for i := range trace.Records {
		if trace.Records[i].Case == caseName {
			indices = append(indices, i)
		}
	}
	return indices
}

func TestRunContextInitTermFilterOperatorsDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "equality-value",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-eq-intboxed-lhs")
				records[0].New[0].Fields["c2"] = "S02"
			},
		},
		{
			name: "greater-boundary",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-gt-intboxed")
				records[0].New[0].Fields["c1"] = int64(9)
			},
		},
		{
			name: "is-not-null-match",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-isnot-intboxed-lhs")
				last := records[len(records)-1]
				for i := range trace.Records {
					if trace.Records[i].Case == last.Case && trace.Records[i].Sequence == last.Sequence {
						trace.Records = append(trace.Records[:i], trace.Records[i+1:]...)
						return
					}
				}
				t.Fatal("is-not-null-match record not found")
			},
		},
		{
			name: "boolean-partition-attribution",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-boolean-arithmetic")
				last := records[len(records)-1]
				last.New[0].Fields["c2"] = "S01"
			},
		},
		{
			name: "boolean-multi-row-order",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-boolean-arithmetic")
				multi := records[1]
				multi.New[0], multi.New[1] = multi.New[1], multi.New[0]
			},
		},
		{
			name: "straight-select-unbound-tag",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-pattern-straight-select")
				records[0].New[0].Fields["c1"] = int64(2)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "op-ne-intboxed-lhs"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-operators.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-filter-operators.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-filter-operators.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-filter-operators-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermOutputClauseDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-output-clause.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-output-clause.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-output-clause.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-output-clause-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermOutputClauseDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "every2-group-sum",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-all-every2-terminated")
				records[0].New[0].Fields["c2"] = int64(2)
			},
		},
		{
			name: "termination-group-order",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-all-every2-terminated")
				records[1].New[0], records[1].New[1] = records[1].New[1], records[1].New[0]
			},
		},
		{
			name: "termination-silence",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "op-all-every2-terminated")
				trace.Records[indices[1]].Case = ""
			},
		},
		{
			name: "when-condition-boundary",
			mutate: func(trace *compat.Trace) {
				records := recordsOfCase(trace, "op-when-expr-when-terminated")
				records[1].New[0].Fields["c0"] = "E2"
			},
		},
		{
			name: "termination-only-condition",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "op-only-when-terminated")
				trace.Records[indices[0]].New = trace.Records[indices[0]].New[:1]
			},
		},
		{
			name: "start-empty-record",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "op-when-set-variable")
				trace.Records[indices[0]].Case = ""
			},
		},
		{
			name: "variable-value",
			mutate: func(trace *compat.Trace) {
				for i := range trace.Records {
					if trace.Records[i].Case == "op-when-set-variable" && trace.Records[i].Operation == "variable" && trace.Records[i].Name == "myvar" {
						trace.Records[i].Value = int64(0)
					}
				}
			},
		},
		{
			name: "termination-assignment",
			mutate: func(trace *compat.Trace) {
				for i := range trace.Records {
					if trace.Records[i].Case == "op-only-terminated-set" && trace.Records[i].Operation == "variable" {
						trace.Records[i].Value = int64(9)
					}
				}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "op-only-when-terminated"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-output-clause.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-output-clause.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-output-clause.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-output-clause-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextStartEndCorrelatedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-start-end-correlated.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-start-end-correlated.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-start-end-correlated.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-start-end-correlated-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextStartEndCorrelatedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "filter-end-value",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "start-end-pattern-filter")
				trace.Records[indices[0]].New[0].Fields["theString"] = "E2"
			},
		},
		{
			name: "filter-end-silence",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "start-end-pattern-filter")
				trace.Records[indices[0]].Case = ""
			},
		},
		{
			name: "or-end-bound-tag",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "start-end-or-correlated")
				trace.Records[indices[0]].New[0].Fields["theString"] = "SB2"
			},
		},
		{
			name: "or-end-unbound-tag",
			mutate: func(trace *compat.Trace) {
				// A partition started by S1 must not terminate on S2(id=a.id)
				// with an unbound a tag; force the third record (SB4) to
				// disappear by relabeling the preceding end event.
				indices := recordIndicesOfCase(trace, "start-end-or-correlated")
				trace.Records[indices[1]].Case = ""
			},
		},
		{
			name: "initiated-correlation",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "init-term-pattern-correlated")
				trace.Records[indices[0]].New[0].Fields["p00"] = "X"
			},
		},
		{
			name: "initiated-termination",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "init-term-pattern-correlated")
				trace.Records[indices[3]].Case = ""
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "start-end-or-correlated"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-start-end-correlated.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-start-end-correlated.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-start-end-correlated.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-start-end-correlated-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermDurationDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-duration.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-duration.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-duration.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-duration-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermInclusiveEqualsDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-inclusive-equals.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-inclusive-equals.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-inclusive-equals.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-inclusive-equals-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermInclusiveEqualsDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "inclusive-start-event-routed",
			mutate: func(trace *compat.Trace) {
				// The start event E1@0 is analyzed through the partition
				// (@Inclusive routes the match): the termination last row at
				// 10000 is E1/3 (replaced by E1@8000), not E1/1. A
				// non-inclusive runtime that drops the routed start event
				// would emit the first retained event instead.
				indices := recordIndicesOfCase(trace, "pattern-inclusion")
				fields := trace.Records[indices[0]].New[0].Fields
				fields["intPrimitive"] = 1
			},
		},
		{
			name: "distinct-expiry-at-10100",
			mutate: func(trace *compat.Trace) {
				// E1@10100 must start a NEW partition (key expired at 10000)
				// and E1/5 is the last row at 20100. If every-distinct never
				// expired, the second E1 partition would not exist and E1/5
				// would not be emitted.
				indices := recordIndicesOfCase(trace, "pattern-inclusion")
				trace.Records[indices[2]].Case = ""
			},
		},
		{
			name: "distinct-swallow-at-8000",
			mutate: func(trace *compat.Trace) {
				// E1@8000 is analyzed by the first partition (theString=E1
				// matches) so the last row at 10000 is E1/3, and E1@8000 must
				// NOT start a second partition (key E1 seen at 0 within 10s).
				indices := recordIndicesOfCase(trace, "pattern-inclusion")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.Time = "1970-01-01T00:00:08Z"
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"theString": "E1", "intPrimitive": 3, "longPrimitive": map[string]any{"state": "null"}}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "multi-event-order",
			mutate: func(trace *compat.Trace) {
				// The routed start events must be S0 then S1 (tag order): the
				// inner pattern completes and fires. If the routing were
				// reversed (S1 first), the sequence could not complete.
				indices := recordIndicesOfCase(trace, "pattern-inclusion-multi")
				fields := trace.Records[indices[0]].New[0].Fields
				fields["a_id"] = 20
				fields["b_id"] = 10
			},
		},
		{
			name: "multi-event-silence",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "pattern-inclusion-multi")
				trace.Records[indices[0]].Case = ""
			},
		},
		{
			name: "straight-equals-sum",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "filter-straight-equals")
				trace.Records[indices[0]].New[0].Fields["c1"] = 3
			},
		},
		{
			name: "straight-equals-partition-split",
			mutate: func(trace *compat.Trace) {
				// I2 (intPrimitive=3) starts a second partition: E4/15 sums to
				// 29 in partition 2 while partition 1 (sb.intPrimitive=2)
				// stays at 9 for E3/2. Dropping the second partition would
				// change the last record to 12.
				indices := recordIndicesOfCase(trace, "filter-straight-equals")
				trace.Records[indices[4]].New[0].Fields["c1"] = 12
			},
		},
		{
			name: "straight-equals-like-filter",
			mutate: func(trace *compat.Trace) {
				// Only theString like "I%" initiates: E1(-1,-2) produces no
				// partition. A like-filter bug that matched everything would
				// analyze E1 into a partition and change the first sum.
				indices := recordIndicesOfCase(trace, "filter-straight-equals")
				trace.Records[indices[0]].New[0].Fields["c1"] = -2
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "filter-straight-equals"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-inclusive-equals.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-inclusive-equals.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-inclusive-equals.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-inclusive-equals-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermDurationDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "start-after-zero-end-instant",
			mutate: func(trace *compat.Trace) {
				// E3 at 60000 must be dropped: the end fires at the exact end
				// instant and the re-armed start only fires at the next
				// advance. Restoring the boundary-inclusive Go behavior
				// (partition [0,60000] with E3 analyzed) would add a record.
				indices := recordIndicesOfCase(trace, "start-after-zero")
				record := trace.Records[indices[len(indices)-1]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"c0": "E3", "c1": 3}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "same-event-not-included-terminator",
			mutate: func(trace *compat.Trace) {
				// The terminating event (intPrimitive=11) must not enter the
				// aggregate: min/max/sum/avg stay 10. A terminator-included
				// bug would snapshot {10,11,21,10.5}.
				indices := recordIndicesOfCase(trace, "same-event-not-included")
				fields := trace.Records[indices[0]].New[0].Fields
				fields["c2"] = 11
				fields["c3"] = 21
				fields["c4"] = 10.5
			},
		},
		{
			name: "same-event-included-avg",
			mutate: func(trace *compat.Trace) {
				// The insert-into terminator must not consume the analyzed
				// event: avg is 10.5 (10+11)/2, not 10.
				indices := recordIndicesOfCase(trace, "same-event-included")
				trace.Records[indices[0]].New[0].Fields["c4"] = 10
			},
		},
		{
			name: "filter-all-terminated-broadcast",
			mutate: func(trace *compat.Trace) {
				// S1 must terminate all partitions: E4 after S1 produces no
				// rows. A single-partition termination bug would keep the
				// second partition alive and emit a third record.
				indices := recordIndicesOfCase(trace, "filter-all-terminated")
				record := trace.Records[indices[len(indices)-1]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"c1": 11}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "filter-after-1-min-initiating-property",
			mutate: func(trace *compat.Trace) {
				// context.sb0.p00 must project the initiating event: SB01 not
				// G1's theString.
				indices := recordIndicesOfCase(trace, "filter-after-1-min")
				trace.Records[indices[0]].New[0].Fields["c3"] = "G2"
			},
		},
		{
			name: "filter-after-1-min-duration-split",
			mutate: func(trace *compat.Trace) {
				// Both partitions receive G4: a duration end that only ends
				// one partition would drop the {G4,4,SB02} row.
				indices := recordIndicesOfCase(trace, "filter-after-1-min")
				row := trace.Records[indices[2]].New
				row = row[:1]
				trace.Records[indices[2]].New = row
			},
		},
		{
			name: "pattern-interval-zero-end-instant",
			mutate: func(trace *compat.Trace) {
				// E3 at 180000 must be dropped: the one-shot OR start fires
				// once at deploy and the 60-second duration ends at 180000.
				indices := recordIndicesOfCase(trace, "pattern-interval-zero")
				record := trace.Records[indices[len(indices)-1]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"c0": "E3", "c1": 34}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "pattern-interval-zero-or-requeue",
			mutate: func(trace *compat.Trace) {
				// The terminal interval(0) OR branch must quit the OR and
				// kill the every-child: E3 at 180000 stays unanalyzed. A
				// keep-every-alive OR bug would restart a partition at
				// 180000 and emit a third record.
				indices := recordIndicesOfCase(trace, "pattern-interval-zero")
				record := trace.Records[indices[len(indices)-1]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"c0": "E3", "c1": 4}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "cal-month-scoped-end-instant",
			mutate: func(trace *compat.Trace) {
				// E3 at 2002-03-01T09:00:00 must be dropped: the calendar
				// month end fires at exactly one AddDate(0,1,0) month.
				indices := recordIndicesOfCase(trace, "cal-month-scoped")
				record := trace.Records[indices[len(indices)-1]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"theString": "E3", "intPrimitive": 3}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "same-event-not-included"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-duration.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-duration.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-duration.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-duration-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermPartitionSelectionDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-partition-selection.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-partition-selection.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-partition-selection.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-partition-selection-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermPartitionSelectionDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "snapshot-sum",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "partition-selection")
				record := trace.Records[indices[6]] // snapshot record
				record.New[0].Fields["c3"] = 5
				trace.Records[indices[6]] = record
			},
		},
		{
			name: "snapshot-partition-key",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "partition-selection")
				record := trace.Records[indices[6]]
				record.Partitions[0].Key = "start:999"
				trace.Records[indices[6]] = record
			},
		},
		{
			name: "by-id-selector-filter",
			mutate: func(trace *compat.Trace) {
				// The ids selector must drop partition 0: restoring a
				// by-key selection that keeps both partitions would add rows.
				indices := recordIndicesOfCase(trace, "partition-selection")
				record := trace.Records[indices[7]] // by-id selector
				record.New = append(record.New,
					compat.ResultRecord{Kind: "row", Fields: map[string]any{"c0": 0, "c1": "S0_1", "c2": "E1", "c3": 6}},
					compat.ResultRecord{Kind: "row", Fields: map[string]any{"c0": 0, "c1": "S0_1", "c2": "E2", "c3": 10}},
					compat.ResultRecord{Kind: "row", Fields: map[string]any{"c0": 0, "c1": "S0_1", "c2": "E3", "c3": 201}})
				trace.Records[indices[7]] = record
			},
		},
		{
			name: "filtered-selector-value",
			mutate: func(trace *compat.Trace) {
				// The initiating-event filtered selector matches only
				// S0_2; a selector matching the wrong property value would
				// return no rows for this snapshot.
				indices := recordIndicesOfCase(trace, "partition-selection")
				record := trace.Records[indices[8]] // filtered S0_2 selector
				record.New = nil
				trace.Records[indices[8]] = record
			},
		},
		{
			name: "always-false-partition-observation",
			mutate: func(trace *compat.Trace) {
				// The always-false filtered selector still visits every
				// partition; dropping the observed partition list hides a
				// selector that short-circuits before inspecting all
				// partitions.
				indices := recordIndicesOfCase(trace, "partition-selection")
				record := trace.Records[indices[9]] // always-false filtered selector
				record.Partitions = []compat.PartitionRecord{{ID: 0, Key: "start:1000", Properties: map[string]any{"startTime": 1000, "endTime": nil, "initiating.p00": "S0_1"}}}
				trace.Records[indices[9]] = record
			},
		},
		{
			name: "selector-error-category",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "partition-selection")
				record := trace.Records[len(trace.Records)-1]
				record.Value = "some-other-error"
				trace.Records[len(trace.Records)-1] = record
				_ = indices
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-partition-selection.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-partition-selection.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-partition-selection.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-partition-selection-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextInitTermPrevPriorDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-prev-prior.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-init-term-prev-prior.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-prev-prior.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-init-term-prev-prior-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextInitTermPrevPriorDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "prev-value",
			mutate: func(trace *compat.Trace) {
				// col1 after E2 is the previous event's theString (E1); a
				// prev bug returning the current event would read E2.
				indices := recordIndicesOfCase(trace, "prev-prior")
				record := trace.Records[indices[2]]
				record.New[0].Fields["col1"] = "E2"
				trace.Records[indices[2]] = record
			},
		},
		{
			name: "prevwindow-order",
			mutate: func(trace *compat.Trace) {
				// prevwindow returns newest-to-oldest ([E2,E1]); reversing
				// the order breaks the window projection.
				indices := recordIndicesOfCase(trace, "prev-prior")
				record := trace.Records[indices[2]]
				rows := record.New[0].Fields["col2"].([]any)
				rows[0], rows[1] = rows[1], rows[0]
				record.New[0].Fields["col2"] = rows
				trace.Records[indices[2]] = record
			},
		},
		{
			name: "prevtail-value",
			mutate: func(trace *compat.Trace) {
				// col3 is the oldest event (E1 for both rows); a tail bug
				// reading the newest event would emit E2.
				indices := recordIndicesOfCase(trace, "prev-prior")
				record := trace.Records[indices[2]]
				record.New[0].Fields["col3"] = "E2"
				trace.Records[indices[2]] = record
			},
		},
		{
			name: "prior-offset",
			mutate: func(trace *compat.Trace) {
				// prior(1) is the immediately preceding event (E1 after E2);
				// an off-by-one reading two events back would emit null.
				indices := recordIndicesOfCase(trace, "prev-prior")
				record := trace.Records[indices[2]]
				record.New[0].Fields["col4"] = map[string]any{"state": "null"}
				trace.Records[indices[2]] = record
			},
		},
		{
			name: "daily-window-close",
			mutate: func(trace *compat.Trace) {
				// The partition ends at 17:00: the snapshot after the close
				// has no rows. A window that never closes would retain E1/E2.
				indices := recordIndicesOfCase(trace, "prev-prior")
				record := trace.Records[indices[3]]
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"col1": "E1", "col3": "E1", "col4": "E1", "col5": 3}}}
				trace.Records[indices[3]] = record
			},
		},
		{
			name: "fresh-partition-prev",
			mutate: func(trace *compat.Trace) {
				// The second day starts a fresh partition: prev/prior are
				// null for E3. A retained-window bug would report E2.
				indices := recordIndicesOfCase(trace, "prev-prior")
				record := trace.Records[indices[4]]
				record.New[0].Fields["col1"] = "E2"
				record.New[0].Fields["col4"] = "E2"
				trace.Records[indices[4]] = record
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-prev-prior.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-init-term-prev-prior.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-init-term-prev-prior.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-init-term-prev-prior-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedViewDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-view.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-view.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-view.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-view-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedViewDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "prevwindow-order",
			mutate: func(trace *compat.Trace) {
				// length(2) prevwindow is newest-to-oldest: [G1,11],[G1,10].
				// Reversing the order would be an off-by-one window read.
				indices := recordIndicesOfCase(trace, "view-scene-one")
				record := trace.Records[indices[2]]
				rows := record.New[0].Fields["pw"].([]any)
				rows[0], rows[1] = rows[1], rows[0]
				record.New[0].Fields["pw"] = rows
				trace.Records[indices[2]] = record
			},
		},
		{
			name: "evicted-prevwindow-null",
			mutate: func(trace *compat.Trace) {
				// The evicted row's prevwindow is null; a window read that
				// leaks the retained view would populate it.
				indices := recordIndicesOfCase(trace, "view-scene-one")
				record := trace.Records[indices[5]]
				record.Old[0].Fields["pw"] = []any{map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": 10, "theString": "G1"}}}
				trace.Records[indices[5]] = record
			},
		},
		{
			name: "partition-isolation",
			mutate: func(trace *compat.Trace) {
				// G2's first row has a single-element window; a key leak from
				// G1's partition would show [G1,10],[G2,20].
				indices := recordIndicesOfCase(trace, "view-scene-one")
				record := trace.Records[indices[1]]
				record.New[0].Fields["pw"] = []any{
					map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": 10, "theString": "G1"}},
					map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": 20, "theString": "G2"}},
				}
				trace.Records[indices[1]] = record
			},
		},
		{
			name: "lastevent-replacement",
			mutate: func(trace *compat.Trace) {
				// lastevent replacement emits new G1/2 with old G1/1; a
				// retention bug that keeps both would drop the old row.
				indices := recordIndicesOfCase(trace, "view-scene-two")
				record := trace.Records[indices[2]]
				record.Old = nil
				trace.Records[indices[2]] = record
			},
		},
		{
			name: "snapshot-partition-key",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "view-scene-one")
				record := trace.Records[indices[3]]
				record.Partitions[0].Key = "key:G3"
				trace.Records[indices[3]] = record
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "view-scene-two"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-view.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-view.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-view.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-view-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedTermByFilterDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-by-filter.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-term-by-filter.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-by-filter.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-term-by-filter-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedTermByFilterDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "termination-restart",
			mutate: func(trace *compat.Trace) {
				// A terminated partition restarts at 1: the row after
				// A/-1 is cnt=1. A no-restart bug would keep counting 3.
				indices := recordIndicesOfCase(trace, "term-by-filter")
				trace.Records[indices[2]].New[0].Fields["cnt"] = 3
			},
		},
		{
			name: "negative-event-silence",
			mutate: func(trace *compat.Trace) {
				// The terminating A/-1 event produces no output; a
				// termination bug that analyzes it would add a row.
				indices := recordIndicesOfCase(trace, "term-by-filter")
				record := trace.Records[indices[1]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"theString": "A", "cnt": 3}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "key-isolation",
			mutate: func(trace *compat.Trace) {
				// B's first row counts only B's partition; a key leak from
				// A would report cnt=3.
				indices := recordIndicesOfCase(trace, "term-by-filter")
				trace.Records[indices[3]].New[0].Fields["cnt"] = 3
			},
		},
		{
			name: "absent-key-negative",
			mutate: func(trace *compat.Trace) {
				// C/-1 has no partition and produces nothing; an
				// auto-initiation bug would emit a row.
				record := trace.Records[len(trace.Records)-1]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"theString": "C", "cnt": 0}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-by-filter.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-term-by-filter.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-by-filter.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-term-by-filter-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedWInitTermEndEventDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-end-event.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-w-init-term-end-event.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-end-event.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-w-init-term-end-event-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedWInitTermEndEventDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "missing-endevent",
			mutate: func(trace *compat.Trace) {
				// The terminating event must be projected into c1; a
				// buffered-at-arrival row would carry missing.
				indices := recordIndicesOfCase(trace, "w-init-term-end-event")
				trace.Records[indices[0]].New[0].Fields["c1"] = map[string]any{"state": "missing"}
			},
		},
		{
			name: "wrong-startevent",
			mutate: func(trace *compat.Trace) {
				// c0 is the initiating event; a snapshot row of the
				// terminating event itself would carry intPrimitive=0.
				indices := recordIndicesOfCase(trace, "w-init-term-end-event")
				trace.Records[indices[0]].New[0].Fields["c0"] = map[string]any{
					"kind": "row", "fields": map[string]any{"theString": "A", "intPrimitive": 0},
				}
			},
		},
		{
			name: "terminating-event-analyzed",
			mutate: func(trace *compat.Trace) {
				// The terminating event does not enter the statement; a
				// bug that analyzes it would emit an extra row.
				indices := recordIndicesOfCase(trace, "w-init-term-end-event")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"c0": map[string]any{"kind": "row", "fields": map[string]any{"theString": "A", "intPrimitive": 0}},
					"c1": map[string]any{"kind": "row", "fields": map[string]any{"theString": "A", "intPrimitive": 0}},
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "partition-isolation",
			mutate: func(trace *compat.Trace) {
				// B's partition starts from its own initiating event; a
				// key leak would reuse A's.
				indices := recordIndicesOfCase(trace, "w-init-term-end-event")
				trace.Records[indices[1]].New[0].Fields["c0"] = map[string]any{
					"kind": "row", "fields": map[string]any{"theString": "A", "intPrimitive": 1},
				}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-end-event.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-w-init-term-end-event.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-end-event.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-w-init-term-end-event-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedTermEventSelectDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-event-select.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-term-event-select.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-event-select.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-term-event-select-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedTermEventSelectDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "missing-term",
			mutate: func(trace *compat.Trace) {
				// The term column must carry the terminating event; a
				// missing projection would show null.
				indices := recordIndicesOfCase(trace, "term-event-select")
				trace.Records[indices[0]].New[0].Fields["term"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "window-holds-terminating-event",
			mutate: func(trace *compat.Trace) {
				// The snapshot observes the state before the terminating
				// event: alert stays A. A bug that lets the terminating
				// event into the firstevent window would project B.
				indices := recordIndicesOfCase(trace, "term-event-select")
				trace.Records[indices[0]].New[0].Fields["alert"] = "B"
			},
		},
		{
			name: "mid-events-analyzed",
			mutate: func(trace *compat.Trace) {
				// The two null-alert events produce no output; a bug that
				// analyzes them would add rows.
				indices := recordIndicesOfCase(trace, "term-event-select")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
					"userId": "U1", "alert": "A",
					"term": map[string]any{"kind": "row", "fields": map[string]any{"userId": "U1", "alert": "A"}},
				}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "partition-isolation",
			mutate: func(trace *compat.Trace) {
				// U2's term must be its own terminating event, not U1's.
				indices := recordIndicesOfCase(trace, "term-event-select")
				trace.Records[indices[1]].New[0].Fields["term"] = map[string]any{
					"kind": "row", "fields": map[string]any{"userId": "U1", "alert": "B"},
				}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-event-select.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-term-event-select.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-term-event-select.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-term-event-select-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedWInitTermPatternAsNameDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-pattern-as-name.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-w-init-term-pattern-as-name.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-pattern-as-name.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-w-init-term-pattern-as-name-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedWInitTermPatternAsNameDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "wrong-start-boxed",
			mutate: func(trace *compat.Trace) {
				// c0 is the initiating event's intBoxed (10); a snapshot
				// that projects the pattern match would yield 20.
				indices := recordIndicesOfCase(trace, "w-init-term-pattern-as-name")
				trace.Records[indices[0]].New[0].Fields["c0"] = 20
			},
		},
		{
			name: "wrong-end-boxed",
			mutate: func(trace *compat.Trace) {
				// c1 is the end-pattern match's intBoxed (20); a stale tag
				// would yield 10.
				indices := recordIndicesOfCase(trace, "w-init-term-pattern-as-name")
				trace.Records[indices[0]].New[0].Fields["c1"] = 10
			},
		},
		{
			name: "mid-event-analyzed",
			mutate: func(trace *compat.Trace) {
				// The mid event (intPrimitive=0) produces no output; a bug
				// that analyzes it would add a row.
				indices := recordIndicesOfCase(trace, "w-init-term-pattern-as-name")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"c0": 99, "c1": 20}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "partition-isolation",
			mutate: func(trace *compat.Trace) {
				// B's pattern match carries its own intBoxed=20; a leak
				// from A would be indistinguishable here, so corrupt the
				// initiating boxed value instead.
				indices := recordIndicesOfCase(trace, "w-init-term-pattern-as-name")
				trace.Records[indices[1]].New[0].Fields["c0"] = 21
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-pattern-as-name.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-w-init-term-pattern-as-name.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-w-init-term-pattern-as-name.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-w-init-term-pattern-as-name-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedMultikeyWArrayOfPrimitiveDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-of-primitive.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-multikey-w-array-of-primitive.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-of-primitive.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-multikey-w-array-of-primitive-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedMultikeyWArrayOfPrimitiveDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "array-content-merge",
			mutate: func(trace *compat.Trace) {
				// [1,2] and [1] partitions must stay isolated; a key bug
				// merging them would show 22 after E12.
				indices := recordIndicesOfCase(trace, "multikey-w-array-of-primitive")
				trace.Records[indices[7]].New[0].Fields["thesum"] = 22
			},
		},
		{
			name: "empty-vs-null-merge",
			mutate: func(trace *compat.Trace) {
				// Empty [] and null are distinct keys: E13 sums into the
				// empty partition (36). Merging null+empty would give 37.
				indices := recordIndicesOfCase(trace, "multikey-w-array-of-primitive")
				trace.Records[indices[8]].New[0].Fields["thesum"] = 37
			},
		},
		{
			name: "null-key-shared",
			mutate: func(trace *compat.Trace) {
				// E5 and E10 share the null partition (34); a per-event
				// partition bug would keep E10 at 20.
				indices := recordIndicesOfCase(trace, "multikey-w-array-of-primitive")
				trace.Records[indices[5]].New[0].Fields["thesum"] = 20
			},
		},
		{
			name: "array-value-mismatch",
			mutate: func(trace *compat.Trace) {
				// E2 accumulates into the [1,2] partition (21); a value
				// misrouting bug would show 11.
				indices := recordIndicesOfCase(trace, "multikey-w-array-of-primitive")
				trace.Records[indices[1]].New[0].Fields["thesum"] = 11
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-of-primitive.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-multikey-w-array-of-primitive.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-of-primitive.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-multikey-w-array-of-primitive-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedMultikeyWArrayTwoFieldDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-two-field.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-multikey-w-array-two-field.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-two-field.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-multikey-w-array-two-field-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedMultikeyWArrayTwoFieldDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "id-isolation",
			mutate: func(trace *compat.Trace) {
				// G1 and G2 with equal arrays are separate partitions; a
				// bug dropping the id key would merge G2[1,2] into G1.
				indices := recordIndicesOfCase(trace, "multikey-w-array-two-field")
				trace.Records[indices[3]].New[0].Fields["thesum"] = 3
			},
		},
		{
			name: "array-distinguishes-same-id",
			mutate: func(trace *compat.Trace) {
				// G1/[1,2] and G1/[1] accumulate independently: G1/[1]
				// stays 3 after G1/[1,2]=15, then 21.
				indices := recordIndicesOfCase(trace, "multikey-w-array-two-field")
				trace.Records[indices[5]].New[0].Fields["thesum"] = 19
			},
		},
		{
			name: "array-content-merge",
			mutate: func(trace *compat.Trace) {
				// G2/[1,2] accumulates 2 then 12; a content-merge bug
				// would give 11.
				indices := recordIndicesOfCase(trace, "multikey-w-array-two-field")
				trace.Records[indices[3]].New[0].Fields["thesum"] = 11
			},
		},
		{
			name: "accumulation-order",
			mutate: func(trace *compat.Trace) {
				// G1/[1,2] goes 1 then 16 (1+15); a fresh-partition bug
				// would give 15.
				indices := recordIndicesOfCase(trace, "multikey-w-array-two-field")
				trace.Records[indices[4]].New[0].Fields["thesum"] = 15
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-two-field.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-multikey-w-array-two-field.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multikey-w-array-two-field.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-multikey-w-array-two-field-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedMatchRecognizeDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-match-recognize.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-match-recognize.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-match-recognize.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-match-recognize-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedMatchRecognizeDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "partition-crossing",
			mutate: func(trace *compat.Trace) {
				// A's A-B match must use A's own events (10,20); a
				// cross-partition bug would pair A/1/10 with B/2/40.
				indices := recordIndicesOfCase(trace, "match-recognize")
				trace.Records[indices[0]].New[0].Fields["b"] = 40
			},
		},
		{
			name: "wrong-measure-a",
			mutate: func(trace *compat.Trace) {
				// a is the A-row longPrimitive (10); a bug reading the B
				// row would give 20.
				indices := recordIndicesOfCase(trace, "match-recognize")
				trace.Records[indices[0]].New[0].Fields["a"] = 20
			},
		},
		{
			name: "fresh-partition-state",
			mutate: func(trace *compat.Trace) {
				// The second A partition cycle (50,60) starts fresh; a
				// stale-state bug would emit on the wrong event or reuse
				// prior values.
				indices := recordIndicesOfCase(trace, "match-recognize")
				trace.Records[indices[2]].New[0].Fields["a"] = 10
			},
		},
		{
			name: "define-predicate",
			mutate: func(trace *compat.Trace) {
				// B requires intPrimitive=2; a buggy predicate matching
				// any event would pair A/1/10 with B/1/30.
				indices := recordIndicesOfCase(trace, "match-recognize")
				trace.Records[indices[0]].New[0].Fields["b"] = 30
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-match-recognize.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-match-recognize.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-match-recognize.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-match-recognize-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedNullKeysDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-null-keys.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-null-keys.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-null-keys.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-null-keys-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedNullKeysDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "null-key-shared",
			mutate: func(trace *compat.Trace) {
				// Two null-key events share one partition (cnt 1,2); a
				// per-event partition bug would keep cnt at 1.
				indices := recordIndicesOfCase(trace, "null-single-key")
				trace.Records[indices[1]].New[0].Fields["cnt"] = 1
			},
		},
		{
			name: "null-vs-present",
			mutate: func(trace *compat.Trace) {
				// A's partition starts at 1; a null-merge bug would count
				// into the null partition (3).
				indices := recordIndicesOfCase(trace, "null-single-key")
				trace.Records[indices[2]].New[0].Fields["cnt"] = 3
			},
		},
		{
			name: "multi-key-null-shared",
			mutate: func(trace *compat.Trace) {
				// (A,null,1) twice shares a partition (1,2); a null-boxed
				// bug splitting by pointer would keep cnt at 1.
				indices := recordIndicesOfCase(trace, "null-key-multi-key")
				trace.Records[indices[1]].New[0].Fields["cnt"] = 1
			},
		},
		{
			name: "multi-key-present-distinct",
			mutate: func(trace *compat.Trace) {
				// (A,10,1) is a separate partition (cnt 1); merging into
				// (A,null,1) would give 3.
				indices := recordIndicesOfCase(trace, "null-key-multi-key")
				trace.Records[indices[2]].New[0].Fields["cnt"] = 3
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-null-keys.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-null-keys.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-null-keys.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-null-keys-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedPatternDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-pattern.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-pattern-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedPatternDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "partition-crossing",
			mutate: func(trace *compat.Trace) {
				// G2's match must pair G2/20 with G2/21; a cross-partition
				// bug would pair G1/10 with G2/21.
				indices := recordIndicesOfCase(trace, "pattern-correlated-every")
				trace.Records[indices[0]].New[0].Fields["a"] = map[string]any{
					"kind": "row", "fields": map[string]any{"theString": "G1", "intPrimitive": 10},
				}
			},
		},
		{
			name: "single-b-wait",
			mutate: func(trace *compat.Trace) {
				// Left-leg every spawns a b-wait per a-event: after
				// G2/21 matched a=G2/20, G2/22 chains to a=G2/21. A
				// single-wait bug (only the first a) would pair G2/22
				// with a=G2/10 instead.
				indices := recordIndicesOfCase(trace, "pattern-correlated-every")
				trace.Records[indices[2]].New[0].Fields["a"] = map[string]any{
					"kind": "row", "fields": map[string]any{"theString": "G2", "intPrimitive": 10},
				}
			},
		},
		{
			name: "correlation-offset",
			mutate: func(trace *compat.Trace) {
				// b requires intPrimitive = a.intPrimitive + 1; an offset
				// bug would emit a=b pairs with equal values.
				indices := recordIndicesOfCase(trace, "pattern-correlated-every")
				trace.Records[indices[1]].New[0].Fields["b"] = map[string]any{
					"kind": "row", "fields": map[string]any{"theString": "G1", "intPrimitive": 10},
				}
			},
		},
		{
			name: "pattern-stopped-after-match",
			mutate: func(trace *compat.Trace) {
				// The every-left keeps the pattern alive: G2/23 chains
				// after G2/22. A pattern-stopped bug would drop later
				// matches.
				indices := recordIndicesOfCase(trace, "pattern-correlated-every")
				trace.Records = trace.Records[:indices[2]]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-pattern.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-pattern-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedPriorDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-prior.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-prior.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-prior.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-prior-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedPriorDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "cross-partition-prior",
			mutate: func(trace *compat.Trace) {
				// G1/11's prior is G1's own 10, not G2's 20; a shared
				// history bug would show 20.
				indices := recordIndicesOfCase(trace, "prior-per-partition")
				trace.Records[indices[2]].New[0].Fields["val1"] = 20
			},
		},
		{
			name: "first-event-prior-null",
			mutate: func(trace *compat.Trace) {
				// The first event of each partition has null prior; a
				// self-inclusion bug would show 10.
				indices := recordIndicesOfCase(trace, "prior-per-partition")
				trace.Records[indices[0]].New[0].Fields["val1"] = 10
			},
		},
		{
			name: "prior-offset",
			mutate: func(trace *compat.Trace) {
				// prior(1) reads the immediately previous event (11's
				// prior is 10); an offset bug would read 12 itself.
				indices := recordIndicesOfCase(trace, "prior-per-partition")
				trace.Records[indices[2]].New[0].Fields["val1"] = 11
			},
		},
		{
			name: "chained-prior",
			mutate: func(trace *compat.Trace) {
				// G2/22's prior is G2/21 (21); a stale-history bug would
				// repeat 20.
				indices := recordIndicesOfCase(trace, "prior-per-partition")
				trace.Records[indices[5]].New[0].Fields["val1"] = 20
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-prior.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-prior.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-prior.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-prior-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedSelectorDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-selector.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-selector.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-selector.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-selector-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedSelectorDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "wrong-key-projection",
			mutate: func(trace *compat.Trace) {
				// c0 is the partition key (E1); a key bug would project
				// the wrong value.
				indices := recordIndicesOfCase(trace, "selector-key1")
				trace.Records[indices[0]].New[0].Fields["c0"] = "E2"
			},
		},
		{
			name: "per-partition-sum",
			mutate: func(trace *compat.Trace) {
				// E2's sum accumulates only E2's events (41); a shared
				// accumulator would show 51.
				indices := recordIndicesOfCase(trace, "selector-key1")
				trace.Records[indices[2]].New[0].Fields["c1"] = 51
			},
		},
		{
			name: "snapshot-rows",
			mutate: func(trace *compat.Trace) {
				// The snapshot holds one row per partition; a bug would
				// drop the E1 row.
				indices := recordIndicesOfCase(trace, "selector-key1")
				snapshot := trace.Records[indices[3]]
				snapshot.New = snapshot.New[1:]
				trace.Records[indices[3]] = snapshot
			},
		},
		{
			name: "snapshot-partition-key",
			mutate: func(trace *compat.Trace) {
				indices := recordIndicesOfCase(trace, "selector-key1")
				partitions := trace.Records[indices[3]].Partitions
				partitions[0].Key = "key:E2"
				trace.Records[indices[3]].Partitions = partitions
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-selector.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-selector.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-selector.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-selector-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedJoinDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-join-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedJoinDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "fan-out-missing",
			mutate: func(trace *compat.Trace) {
				// S0(20) joins G2's partition; a missing fan-out would
				// emit nothing and the record would not exist.
				indices := recordIndicesOfCase(trace, "join-per-partition")
				trace.Records = trace.Records[:indices[0]]
			},
		},
		{
			name: "late-partition-sees-early-s0",
			mutate: func(trace *compat.Trace) {
				// G3 created after S0(30) must NOT join it; a bug that
				// retro-fills would add {G3,30,30}.
				indices := recordIndicesOfCase(trace, "join-per-partition")
				record := trace.Records[indices[1]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"sb.theString": "G3", "sb.intPrimitive": 30, "s0.id": 30,
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "join-condition",
			mutate: func(trace *compat.Trace) {
				// G1/30 joins S0(30); an off-by-one would pair G1/30 with
				// S0(20).
				indices := recordIndicesOfCase(trace, "join-per-partition")
				trace.Records[indices[1]].New[0].Fields["s0.id"] = 20
			},
		},
		{
			name: "per-partition-state",
			mutate: func(trace *compat.Trace) {
				// G1's join uses only G1's window: S0(20) does not join
				// G1/10 (10 != 20) — a shared-window bug would emit it.
				indices := recordIndicesOfCase(trace, "join-per-partition")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"sb.theString": "G1", "sb.intPrimitive": 10, "s0.id": 20,
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-join-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedAdditionalFiltersDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-additional-filters.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-additional-filters.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-additional-filters.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-additional-filters-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedAdditionalFiltersDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "filter-rejection",
			mutate: func(trace *compat.Trace) {
				// B1/-1 and S0(-2) fail the per-stream filters and create
				// no partition; a bug that accepts them would add records.
				indices := recordIndicesOfCase(trace, "additional-filters")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
					"col1": map[string]any{"state": "null"}, "col2": map[string]any{"state": "null"},
				}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "per-stream-key",
			mutate: func(trace *compat.Trace) {
				// S0(2,"S0") partitions by p00 (S0 partition); a key bug
				// mixing streams would misroute and produce different sums.
				indices := recordIndicesOfCase(trace, "additional-filters")
				trace.Records[indices[0]].New[0].Fields["col2"] = 0
			},
		},
		{
			name: "match-tag-aggregate",
			mutate: func(trace *compat.Trace) {
				// S1 partition's col1 accumulates only SB matches (10); a
				// tag bug counting S0 matches would show 13.
				indices := recordIndicesOfCase(trace, "additional-filters")
				trace.Records[indices[2]].New[0].Fields["col1"] = 13
			},
		},
		{
			name: "per-partition-aggregate",
			mutate: func(trace *compat.Trace) {
				// S0 partition col1 is 9 after SB(S0,9); a shared
				// accumulator would show 19.
				indices := recordIndicesOfCase(trace, "additional-filters")
				trace.Records[indices[3]].New[0].Fields["col1"] = 19
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-additional-filters.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-additional-filters.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-additional-filters.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-additional-filters-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedMultiStatementFilterCountDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multi-statement-filter-count.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-multi-statement-filter-count.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multi-statement-filter-count.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-multi-statement-filter-count-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedMultiStatementFilterCountDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "cross-statement-routing",
			mutate: func(trace *compat.Trace) {
				// s0 receives only S0 events; a routing bug feeding SB
				// events into s0 would emit extra s0 records.
				indices := recordIndicesOfCase(trace, "multi-statement")
				record := trace.Records[indices[3]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"col1": 5}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "s0-partition-sum",
			mutate: func(trace *compat.Trace) {
				// s0's S0 partition sums 10+4+7=21; a shared accumulator
				// would include S1's 8.
				indices := recordIndicesOfCase(trace, "multi-statement")
				trace.Records[indices[5]].New[0].Fields["col1"] = 29
			},
		},
		{
			name: "s1-partition-sum",
			mutate: func(trace *compat.Trace) {
				// s1's S0 partition sums 5+9=14; a cross-partition bug
				// would include S2's 6.
				indices := recordIndicesOfCase(trace, "multi-statement")
				trace.Records[indices[6]].New[0].Fields["col1"] = 20
			},
		},
		{
			name: "statement-label",
			mutate: func(trace *compat.Trace) {
				// s1 records must carry s1; a label bug would tag s0.
				indices := recordIndicesOfCase(trace, "multi-statement")
				trace.Records[indices[3]].Statement = "s0"
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multi-statement-filter-count.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-multi-statement-filter-count.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-multi-statement-filter-count.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-multi-statement-filter-count-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedJoinMultitypeMultifieldDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-multitype-multifield.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join-multitype-multifield.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-multitype-multifield.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-join-multitype-multifield-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedJoinMultitypeMultifieldDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "key2-projection",
			mutate: func(trace *compat.Trace) {
				// c6 is the partition's second key (1); a key bug would
				// project the first key.
				indices := recordIndicesOfCase(trace, "join-multitype-multifield")
				trace.Records[indices[0]].New[0].Fields["c6"] = "G2"
			},
		},
		{
			name: "cross-stream-key",
			mutate: func(trace *compat.Trace) {
				// G2/1's join pairs SB(G2,1) with S0(1,G2); a key mixing
				// bug would pair across (G2,2).
				indices := recordIndicesOfCase(trace, "join-multitype-multifield")
				trace.Records[indices[0]].New[0].Fields["c3"] = 2
			},
		},
		{
			name: "per-partition-window",
			mutate: func(trace *compat.Trace) {
				// G1/1's join uses only G1/1's windows; a shared-window
				// bug would leak G2 rows.
				indices := recordIndicesOfCase(trace, "join-multitype-multifield")
				trace.Records[indices[2]].New[0].Fields["c1"] = "G2"
			},
		},
		{
			name: "empty-side-silence",
			mutate: func(trace *compat.Trace) {
				// The first four events create single-sided partitions
				// with no join output; a bug would emit rows.
				indices := recordIndicesOfCase(trace, "join-multitype-multifield")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
					"c1": "G1", "c2": 1, "c3": map[string]any{"state": "null"}, "c4": map[string]any{"state": "null"}, "c5": "G1", "c6": 1,
				}}}
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-multitype-multifield.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join-multitype-multifield.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-multitype-multifield.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-join-multitype-multifield-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedPatternSceneTwoDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-scene-two.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-pattern-scene-two.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-scene-two.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-pattern-scene-two-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedPatternSceneTwoDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "cross-partition-routing",
			mutate: func(trace *compat.Trace) {
				// S0(20,"G2") routes to the G2 partition and matches
				// a=G2/20; a fan-out bug pairing with G1's a would emit
				// {G1,10,20,G2}.
				indices := recordIndicesOfCase(trace, "pattern-scene-two")
				trace.Records[indices[0]].New[0].Fields["c0"] = "G1"
			},
		},
		{
			name: "correlation",
			mutate: func(trace *compat.Trace) {
				// b requires id = a.intPrimitive; an offset bug would
				// pair S0(0) with a=G1/10.
				indices := recordIndicesOfCase(trace, "pattern-scene-two")
				trace.Records[indices[0]].New[0].Fields["c2"] = 0
			},
		},
		{
			name: "consumed-a",
			mutate: func(trace *compat.Trace) {
				// After G2's match, the second S0(20) has no pending a;
				// a re-fire bug would emit a duplicate.
				indices := recordIndicesOfCase(trace, "pattern-scene-two")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"c0": "G2", "c1": 20, "c2": 20, "c3": "G2",
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "partner-key",
			mutate: func(trace *compat.Trace) {
				// S0(10,"G1") routes to the G1 partition by p00; a key
				// bug would route it elsewhere and miss the match.
				indices := recordIndicesOfCase(trace, "pattern-scene-two")
				trace.Records = trace.Records[:indices[0]]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-scene-two.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-pattern-scene-two.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-scene-two.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-pattern-scene-two-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedJoinWhereClauseOnPartitionKeyDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-where-clause-on-partition-key.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join-where-clause-on-partition-key.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-where-clause-on-partition-key.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-join-where-clause-on-partition-key-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedJoinWhereClauseOnPartitionKeyDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "where-clause",
			mutate: func(trace *compat.Trace) {
				// The where clause admits only the Test partition; a bug
				// dropping it would emit E2's join too.
				indices := recordIndicesOfCase(trace, "join-where-partition-key")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"sb.theString": "E2", "sb.intPrimitive": 20, "s0.id": 1, "s0.p00": "S0",
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "fan-out",
			mutate: func(trace *compat.Trace) {
				// S0(1) fans out to both partitions; a missing fan-out
				// would produce no record at all.
				indices := recordIndicesOfCase(trace, "join-where-partition-key")
				trace.Records = trace.Records[:indices[0]]
			},
		},
		{
			name: "partition-key-value",
			mutate: func(trace *compat.Trace) {
				// The join row carries SB(Test,10); a wrong window value
				// would carry 20.
				indices := recordIndicesOfCase(trace, "join-where-partition-key")
				trace.Records[indices[0]].New[0].Fields["sb.intPrimitive"] = 20
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-where-clause-on-partition-key.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join-where-clause-on-partition-key.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-where-clause-on-partition-key.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-join-where-clause-on-partition-key-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedJoinRemoveStreamDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-remove-stream.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join-remove-stream.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-remove-stream.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-join-remove-stream-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedJoinRemoveStreamDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "missing-removal",
			mutate: func(trace *compat.Trace) {
				// The expiry of session 3's Start must emit the removal; a
				// window-expiry bug would drop it.
				indices := recordIndicesOfCase(trace, "join-remove-stream")
				trace.Records = trace.Records[:indices[0]]
			},
		},
		{
			name: "complete-session-leak",
			mutate: func(trace *compat.Trace) {
				// Complete sessions (Start+Middle+End) produce no removal;
				// an outer-join bug would emit their expiry.
				indices := recordIndicesOfCase(trace, "join-remove-stream")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"pageNameA": "Start", "sessionIdA": "0", "pageNameB": map[string]any{"state": "null"}, "pageNameC": "End",
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "insert-replacement-leak",
			mutate: func(trace *compat.Trace) {
				// Java's outer join emits no removal when an intermediate
				// null-padded row is replaced; an insert-phase record
				// would be a leak.
				indices := recordIndicesOfCase(trace, "join-remove-stream")
				record := trace.Records[indices[0]]
				record.Sequence++
				record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: map[string]any{
					"pageNameA": "Start", "sessionIdA": "0", "pageNameB": map[string]any{"state": "null"}, "pageNameC": map[string]any{"state": "null"},
				}})
				trace.Records = append(trace.Records, record)
			},
		},
		{
			name: "wrong-partner",
			mutate: func(trace *compat.Trace) {
				// The removal carries End as the C side; a wrong window
				// would show Middle.
				indices := recordIndicesOfCase(trace, "join-remove-stream")
				trace.Records[indices[0]].New[0].Fields["pageNameC"] = "Middle"
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].Case = "other"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-remove-stream.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-join-remove-stream.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-join-remove-stream.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-join-remove-stream-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunContextKeySegmentedPatternFilterDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-filter.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-pattern-filter.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-filter.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "context-key-segmented-pattern-filter-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunContextKeySegmentedPatternFilterDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "spurious-match",
			mutate: func(trace *compat.Trace) {
				// The pattern can never match within a partition; a bug
				// that pairs F1 with X1 would emit a record.
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case: "pattern-filter", Operation: "listener", Statement: "s0", Sequence: 1,
					Time: "1970-01-01T00:00:00Z",
					New: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
						"a_theString": "F1", "b_theString": "X1",
					}}},
				})
			},
		},
		{
			name: "cross-partition-match",
			mutate: func(trace *compat.Trace) {
				// A cross-partition leak pairing F1 (F1 partition) with
				// X1 (X1 partition) would emit a record.
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case: "pattern-filter", Operation: "listener", Statement: "s0", Sequence: 1,
					Time: "1970-01-01T00:00:00Z",
					New: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
						"a_theString": "F1", "b_theString": "X1",
					}}},
				})
			},
		},
		{
			name: "loose-predicate",
			mutate: func(trace *compat.Trace) {
				// A loosened event2 predicate (any event) would match
				// F1->F1; that must be detected.
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case: "pattern-filter", Operation: "listener", Statement: "s0", Sequence: 1,
					Time: "1970-01-01T00:00:00Z",
					New: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
						"a_theString": "F1", "b_theString": "F1",
					}}},
				})
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				// No-op for an empty trace: append then drop keeps identity;
				// instead corrupt the case list by adding a bogus record.
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case: "pattern-filter", Operation: "listener", Statement: "s0", Sequence: 1,
					Time: "1970-01-01T00:00:00Z",
					New:  []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"a_theString": "x"}}},
				})
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case: "other", Operation: "listener", Statement: "s0", Sequence: 1,
					Time: "1970-01-01T00:00:00Z",
					New:  []compat.ResultRecord{{Kind: "row", Fields: map[string]any{"a_theString": "x"}}},
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-filter.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "context-key-segmented-pattern-filter.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "context-key-segmented-pattern-filter.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "context-key-segmented-pattern-filter-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectUnfilteredDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-unfiltered.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-unfiltered.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-unfiltered.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-unfiltered-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectUnfilteredDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "expression-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["value"] = "ab"
			},
		},
		{
			name: "expression-concat",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["value"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "unlimited-stream-multi-row-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["idS1"] = int64(10)
			},
		},
		{
			name: "length-window-single-row",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New[0].Fields["idS1"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "self-subselect-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].New[0].Fields["value"] = int64(1)
			},
		},
		{
			name: "stream-prior-newest",
			mutate: func(trace *compat.Trace) {
				trace.Records[36].New[0].Fields["idS1"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "two-subq-select",
			mutate: func(trace *compat.Trace) {
				trace.Records[39].New[0].Fields["idS1_0"] = int64(0)
			},
		},
		{
			name: "join-unfiltered",
			mutate: func(trace *compat.Trace) {
				trace.Records[44].New[0].Fields["idS3"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:45]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[45].Case = "expression"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-unfiltered.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-unfiltered.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-unfiltered.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-unfiltered-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectQuantifiedDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-quantified.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-quantified.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-quantified.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-quantified-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectQuantifiedDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "relational-all-empty-true",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["g"] = false
			},
		},
		{
			name: "relational-all-equal-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["ge"] = false
			},
		},
		{
			name: "relational-all-strict-both",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["le"] = true
			},
		},
		{
			name: "relational-all-om-fresh-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[7].New[0].Fields["g"] = false
			},
		},
		{
			name: "relational-some-empty-false",
			mutate: func(trace *compat.Trace) {
				trace.Records[8].New[0].Fields["ge"] = true
			},
		},
		{
			name: "relational-some-any-vs-all",
			mutate: func(trace *compat.Trace) {
				trace.Records[13].New[0].Fields["le"] = false
			},
		},
		{
			name: "equals-not-equals-all-mixed-set",
			mutate: func(trace *compat.Trace) {
				trace.Records[19].New[0].Fields["nneq"] = false
			},
		},
		{
			name: "equals-any-or-some-synonym",
			mutate: func(trace *compat.Trace) {
				trace.Records[24].New[0].Fields["r3"] = false
			},
		},
		{
			name: "relational-null-empty-all",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].New[0].Fields["vall"] = false
			},
		},
		{
			name: "relational-null-any-false-dominates",
			mutate: func(trace *compat.Trace) {
				trace.Records[32].New[0].Fields["vany"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "equals-in-null-empty-in",
			mutate: func(trace *compat.Trace) {
				trace.Records[33].New[0].Fields["isin"] = true
			},
		},
		{
			name: "equals-in-null-e6-neall",
			mutate: func(trace *compat.Trace) {
				trace.Records[38].New[0].Fields["neall"] = true
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[21].Case = "relational-all"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-quantified.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-quantified.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-quantified.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-quantified-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupDimensionalityDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "rollup-dimensionality.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "rollup-dimensionality.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-dimensionality.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "rollup-dimensionality-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunInfraNwTableFafJoinDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "infra-nwtable-faf-join.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "infra-nwtable-faf-join.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "infra-nwtable-faf-join.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "infra-nwtable-faf-join-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunInfraNwTableFafJoinDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "join-result-row-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New = trace.Records[0].New[:0]
			},
		},
		{
			name: "join-product-value-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["WinProduct.productId"] = "Product2"
			},
		},
		{
			name: "map-representation-row-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[8].New[0].Fields["WinProduct.productId"] = "WRONG"
			},
		},
		{
			name: "avro-representation-row-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[16].New = nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "infra-nwtable-faf-join.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "infra-nwtable-faf-join.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "infra-nwtable-faf-join.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "infra-nwtable-faf-join-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunViewLengthBatchDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "view-length-batch.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "view-length-batch.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "view-length-batch.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "view-length-batch-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunViewUniqueDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "view-unique.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "view-unique.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "view-unique.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "view-unique-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunViewUniqueDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "listener-alias-value-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[14].New[0].Fields["c0"] = "WRONG"
			},
		},
		{
			name: "unique-evicted-old-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[18].Old = nil
			},
		},
		{
			name: "snapshot-row-value-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].New[0].Fields["c1"] = 99
			},
		},
		{
			name: "two-windows-deferred-statement-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[32].New = nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "view-unique.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "view-unique.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "view-unique.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "view-unique-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunExprFilterOptimizableDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "expr-filter-optimizable.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "expr-filter-optimizable.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-filter-optimizable.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "expr-filter-optimizable-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunExprFilterOptimizableDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "filter-fire-lost",
			mutate: func(trace *compat.Trace) {
				for i := range trace.Records {
					if trace.Records[i].Case == "in-and-not-in-multivalue" && trace.Records[i].Sequence == 2 {
						trace.Records[i].New = nil
						return
					}
				}
				trace.Records[2].New = nil
			},
		},
		{
			name: "observation-value-drift",
			mutate: func(trace *compat.Trace) {
				for i := range trace.Records {
					if trace.Records[i].Case == "method-invocation-context" && trace.Records[i].Name == "runtimeURI" {
						trace.Records[i].Value = "wrong-runtime"
						return
					}
				}
				trace.Records[9].Value = "wrong-runtime"
			},
		},
		{
			name: "pattern-tag-row-drift",
			mutate: func(trace *compat.Trace) {
				for i := range trace.Records {
					if trace.Records[i].Case == "pattern-udf" {
						fields, ok := trace.Records[i].New[0].Fields["b"].(map[string]any)
						if ok {
							fields["theString"] = "WRONG"
						}
						return
					}
				}
			},
		},
		{
			name: "deploy-time-constant-listener-lost",
			mutate: func(trace *compat.Trace) {
				for i := range trace.Records {
					if trace.Records[i].Case == "deploy-time-constant" && trace.Records[i].Statement == "s0" && trace.Records[i].Sequence == 2 {
						trace.Records[i].New = nil
						return
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "expr-filter-optimizable.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "expr-filter-optimizable.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "expr-filter-optimizable.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "expr-filter-optimizable-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunEplOtherWildcardAdditionalDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-wildcard-additional.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "epl-other-wildcard-additional.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-wildcard-additional.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "epl-other-wildcard-additional-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunRollupDimensionalityDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// int[] rollup dimensions compare by content: [4,5] is its own
			// group with cnt 1.
			name: "warray-content-key-collapsed",
			mutate: func(trace *compat.Trace) {
				trace.Records[68].New[0].Fields["array"] = []any{int64(1), int64(2)}
			},
		},
		{
			// Named-window cube delete-all must null the sums in new data.
			name: "nw-cube-delete-sums-retained",
			mutate: func(trace *compat.Trace) {
				trace.Records[101].New[3].Fields["c2"] = int64(1000)
			},
		},
		{
			// Grouped on-select emits row-per-group plus the overall level.
			name: "onselect-overall-row-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[107].New = trace.Records[107].New[:3]
			},
		},
		{
			// Output-when-terminated flushes exactly the partition groups.
			name: "out-when-term-flip",
			mutate: func(trace *compat.Trace) {
				trace.Records[109].New[0].Fields["c1"] = int64(5)
			},
		},
		{
			// window(*) renders each group's own events.
			name: "mixed-access-window-leak",
			mutate: func(trace *compat.Trace) {
				trace.Records[130].New[0].Fields["c0"] = int64(6)
			},
		},
		{
			// Non-boxed short sums preserve Integer/Double/Long output types.
			name: "non-boxed-type-widened",
			mutate: func(trace *compat.Trace) {
				trace.Records[132].Value.(map[string]any)["c2"] = "Double"
			},
		},
		{
			// Computed case-when keys null plain fields at the overall level.
			name: "groupby-computation-frame-null-dropped",
			mutate: func(trace *compat.Trace) {
				trace.Records[136].New[1].Fields["c0"] = int64(10)
			},
		},
		{
			name: "rollup-2dim-subtotal",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["c2"] = int64(999)
			},
		},
		{
			name: "rollup-2dim-overall-key-null-padding",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[2].Fields["c0"] = "E2"
			},
		},
		{
			name: "rollup-1dim-cube-equivalence",
			mutate: func(trace *compat.Trace) {
				trace.Records[12].New[0].Fields["c1"] = int64(11)
			},
		},
		{
			name: "unenclosed-nested-key-retained",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].New[0].Fields["c0"] = nil
			},
		},
		{
			name: "rollup-3dim-overall-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].New[3].Fields["c3"] = int64(2)
			},
		},
		{
			name: "rollup-3dim-gs-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[33].New[0].Fields["c1"] = nil
			},
		},
		{
			name: "rollup-3dim-join-prime",
			mutate: func(trace *compat.Trace) {
				trace.Records[39].New[0].Fields["c0"] = "E2"
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[19].Case = "unbound-rollup-1dim-cube"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-dimensionality.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-dimensionality.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-dimensionality.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-dimensionality-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunRollupDimensionalityCubeMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "cube-unenclosed-retained-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[51].New[0].Fields["c0"] = nil
			},
		},
		{
			name: "cube-4dim-mask-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[62].New[1].Fields["c0"] = nil
			},
		},
		{
			name: "cube-4dim-cross-accumulation",
			mutate: func(trace *compat.Trace) {
				trace.Records[52].New[15].Fields["c4"] = int64(6000)
			},
		},
		{
			name: "bound-rollup-expiry-null-sum",
			mutate: func(trace *compat.Trace) {
				trace.Records[56].New[1].Fields["c2"] = int64(400)
			},
		},
		{
			name: "batch-flush-old-null-aggregates",
			mutate: func(trace *compat.Trace) {
				trace.Records[64].Old[0].Fields["c2"] = int64(100)
			},
		},
		{
			name: "batch-flush-new-accumulation",
			mutate: func(trace *compat.Trace) {
				trace.Records[65].New[0].Fields["c2"] = int64(300)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "rollup-dimensionality.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "rollup-dimensionality.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "rollup-dimensionality.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "rollup-dimensionality-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunOrderBySimpleDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "orderby-simple.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "orderby-simple.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "orderby-simple.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "orderby-simple-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunOrderBySimpleDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "orderby-asc-wrong-symbol",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["symbol"] = "WRONG"
			},
		},
		{
			name: "orderby-desc-wrong-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["symbol"] = "KGB"
			},
		},
		{
			name: "multikey-tie-break",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["symbol"] = "WRONG"
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "orderby-simple.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "orderby-simple.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "orderby-simple.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "orderby-simple-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectFilteredDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-filtered.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-filtered.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-filtered.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-filtered-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunSubselectFilteredDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "having-empty-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["c0"] = 0
			},
		},
		{
			name: "having-where-excluded",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["c0"] = 11
			},
		},
		{
			name: "filter-vs-where-exclusion",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].New[0].Fields["c0"] = 20
			},
		},
		{
			name: "null-on-multiple",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].New[0].Fields["ids1"] = 2
			},
		},
		{
			name: "two-column-match",
			mutate: func(trace *compat.Trace) {
				trace.Records[16].New[0].Fields["ids1"] = 2
			},
		},
		{
			name: "range-boundary",
			mutate: func(trace *compat.Trace) {
				trace.Records[17].New[0].Fields["ids1"] = "E2"
			},
		},
		{
			name: "joined-correlation",
			mutate: func(trace *compat.Trace) {
				trace.Records[20].New[0].Fields["ids1"] = 2
			},
		},
		{
			name: "multikey-empty-array-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[25].New[0].Fields["value"] = "MA3"
			},
		},
		{
			name: "multikey-null-array-key",
			mutate: func(trace *compat.Trace) {
				trace.Records[27].New[0].Fields["value"] = "MA1"
			},
		},
		{
			name: "multikey-null-on-multiple",
			mutate: func(trace *compat.Trace) {
				trace.Records[28].New[0].Fields["value"] = "MA1"
			},
		},
		{
			name: "multikey-scalar-equality",
			mutate: func(trace *compat.Trace) {
				trace.Records[32].New[0].Fields["value"] = "MA3"
			},
		},
		{
			name: "coercion-cross-type-match",
			mutate: func(trace *compat.Trace) {
				trace.Records[39].New[0].Fields["ids0"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "coercion-near-miss-rejected",
			mutate: func(trace *compat.Trace) {
				trace.Records[44].New[0].Fields["ids0"] = -3
			},
		},
		{
			name: "back-coercion-double-precision",
			mutate: func(trace *compat.Trace) {
				trace.Records[56].New[0].Fields["ids0"] = -3
			},
		},
		{
			name: "coercion-predicate-order-invariant",
			mutate: func(trace *compat.Trace) {
				trace.Records[49].New[0].Fields["ids0"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "join-filtered-gate-match",
			mutate: func(trace *compat.Trace) {
				trace.Records[63].New[0].Fields["s2p20"] = "qx"
			},
		},
		{
			name: "join-filtered-prior-unfiltered",
			mutate: func(trace *compat.Trace) {
				trace.Records[64].New[0].Fields["s2p20Prior"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "join-filtered-prev-window",
			mutate: func(trace *compat.Trace) {
				trace.Records[66].New[0].Fields["s2p20Prev"] = "qx"
			},
		},
		{
			name: "where-previous-unfiltered-window",
			mutate: func(trace *compat.Trace) {
				trace.Records[71].New[0].Fields["value"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "where-previous-anchor-offset",
			mutate: func(trace *compat.Trace) {
				trace.Records[72].New[0].Fields["value"] = 1
			},
		},
		{
			name: "same-event-trigger-snapshot",
			mutate: func(trace *compat.Trace) {
				trace.Records[76].New[0].Fields["events1"] = map[string]any{
					"kind": "row",
					"fields": map[string]any{
						"id":  map[string]any{"state": "null"},
						"p10": "Y",
					},
				}
			},
		},
		{
			name: "wildcard-cross-stream-prior",
			mutate: func(trace *compat.Trace) {
				trace.Records[79].New[0].Fields["events1"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "multi-stream-partial-correlation",
			mutate: func(trace *compat.Trace) {
				trace.Records[83].New[0].Fields["ids0"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "multi-stream-full-correlation",
			mutate: func(trace *compat.Trace) {
				trace.Records[88].New[0].Fields["ids0"] = 99
			},
		},
		{
			name: "multi-stream-scene-two-positive-hit",
			mutate: func(trace *compat.Trace) {
				trace.Records[91].New[0].Fields["ids0"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "multi-stream-join-gate",
			mutate: func(trace *compat.Trace) {
				trace.Records[80].New[0].Fields["ids0"] = 99
			},
		},
		{
			name: "scene-one-old-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[94].Old[0].Fields["s0price"] = 999.0
			},
		},
		{
			name: "where-2-or-gate",
			mutate: func(trace *compat.Trace) {
				trace.Records[95].New[0].Fields["id"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "mixmax-low-window",
			mutate: func(trace *compat.Trace) {
				trace.Records[99].New[0].Fields["low"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "prior-dedupe-gate",
			mutate: func(trace *compat.Trace) {
				trace.Records[102].New[0].Fields["b"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[21].Case = "where-constant-range"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-filtered.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-filtered.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-filtered.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-filtered-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectMulticolumnDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multicolumn.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-multicolumn.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multicolumn.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-multicolumn-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectMulticolumnDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "multicolumn-agg-empty-window-count",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["s1totals"] = map[string]any{"v1": int64(1)}
			},
		},
		{
			name: "multicolumn-agg-sum-value",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["s1totals"] = map[string]any{"v1": int64(1), "v2": int64(201)}
			},
		},
		{
			name: "columns-uncorrelated-empty-subrow",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["subrow"] = map[string]any{"state": "null"}
			},
		},
		{
			name: "columns-uncorrelated-lastevent",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New[0].Fields["subrow"] = map[string]any{"v1": "E2", "v2": int64(20)}
			},
		},
		{
			name: "columns-uncorrelated-om-fresh-state",
			mutate: func(trace *compat.Trace) {
				trace.Records[9].New[0].Fields["subrow"] = map[string]any{}
			},
		},
		{
			name: "correlated-empty-all-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].New[0].Fields["subrow"] = map[string]any{"v1": int64(0)}
			},
		},
		{
			name: "correlated-sum-plus-one",
			mutate: func(trace *compat.Trace) {
				trace.Records[11].New[0].Fields["subrow"] = map[string]any{"v1": int64(10), "v2": int64(10)}
			},
		},
		{
			name: "correlated-window-array",
			mutate: func(trace *compat.Trace) {
				trace.Records[12].New[0].Fields["subrow"] = map[string]any{"v3": []any{int64(10)}}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:12]
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].Case = "multicolumn-agg"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multicolumn.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-multicolumn.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multicolumn.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-multicolumn-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectAggregatedMultirowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-multirow.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-aggregated-multirow.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-multirow.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-aggregated-multirow-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectAggregatedMultirowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "nodatawindow-empty-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["subq"] = []any{}
			},
		},
		{
			name: "nodatawindow-second-group-sum",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["subq"] = []any{map[string]any{"c0": "G1", "c1": int64(10)}}
			},
		},
		{
			name: "context-partition-inner-first-row",
			mutate: func(trace *compat.Trace) {
				trace.Records[52].New[0].Fields["subq"] = map[string]any{}
			},
		},
		{
			name: "whaving-single-group",
			mutate: func(trace *compat.Trace) {
				trace.Records[58].New[0].Fields["subq"] = map[string]any{"c0": "E2", "c1": int64(12)}
			},
		},
		{
			name: "indexshare-array-key-sum",
			mutate: func(trace *compat.Trace) {
				trace.Records[67].New[0].Fields["e1"] = []any{map[string]any{"c0": []any{int64(1), int64(2)}, "c1": int64(22)}, map[string]any{"c0": []any{int64(1)}, "c1": int64(41)}}
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-multirow.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-aggregated-multirow.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-aggregated-multirow.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-aggregated-multirow-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation %s unexpectedly passed", test.name)
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			var output compat.DifferentialEvidence
			if err := json.Unmarshal(data, &output); err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectDirectMultirowDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multirow.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "subselect-multirow.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multirow.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-multirow-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	var output compat.DifferentialEvidence
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestRunSubselectDirectMultirowDiffAcceptsUnderlyingOrderMutation(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multirow.evidence.json"),
		func(trace *compat.Trace) {
			rows := trace.Records[5].New[0].Fields["val"].([]any)
			rows[0], rows[1] = rows[1], rows[0]
		})
	evidencePath := filepath.Join(t.TempDir(), "subselect-multirow.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multirow.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "subselect-multirow-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if output.Status != "passing" || len(output.Differences) != 0 || stdout.Len() != 0 {
		t.Fatalf("evidence=%s stdout=%q", data, stdout.String())
	}
}

func TestNormalizeSubselectDirectMultirowUnderlyingTraceSortsRows(t *testing.T) {
	trace := compat.Trace{
		Version: "esper-parity/v1",
		ID:      "subselect-multirow",
		Records: []compat.TraceRecord{{
			Case: "multirow-underlying-correlated",
			New: []compat.ResultRecord{{
				Kind: "row",
				Fields: map[string]any{
					"val": []any{
						map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": int64(30), "theString": "T2"}},
						map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": int64(20), "theString": "T2"}},
					},
				},
			}},
		}},
	}
	normalized := normalizeSubselectDirectMultirowUnderlyingTrace(trace)
	rows := normalized.Records[0].New[0].Fields["val"].([]any)
	if rows[0].(map[string]any)["fields"].(map[string]any)["intPrimitive"] != int64(20) ||
		rows[1].(map[string]any)["fields"].(map[string]any)["intPrimitive"] != int64(30) {
		t.Fatalf("normalized rows = %#v", rows)
	}
}

func TestRunSubselectDirectMultirowDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "direct-window-values",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["val"] = []any{int64(5), int64(10), int64(15)}
			},
		},
		{
			name: "late-length-window",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New[0].Fields["val"] = []any{int64(5), int64(15), int64(6)}
			},
		},
		{
			name: "correlated-empty-null",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].New[0].Fields["val"] = []any{}
			},
		},
		{
			name: "underlying-field",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[4].New[0].Fields["val"].([]any)
				rows[0].(map[string]any)["fields"].(map[string]any)["intPrimitive"] = int64(11)
			},
		},
		{
			name: "case-label",
			mutate: func(trace *compat.Trace) {
				trace.Records[3].Case = "multirow-single-column"
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:5]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multirow.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-multirow.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-multirow.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-multirow-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation %s unexpectedly passed; stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunEplOtherDistinctDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "epl-other-distinct.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "epl-other-distinct-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunEplOtherDistinctDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "simple-column-duplicate-suppressed",
			mutate: func(trace *compat.Trace) {
				// Continuous keepall deliveries must re-emit duplicates.
				trace.Records[1].New = nil
			},
		},
		{
			name: "output-every-bundle-dedup-broken",
			mutate: func(trace *compat.Trace) {
				// Bundle dedup collapses the repeated (E1,1) row.
				trace.Records[7].New = append(trace.Records[7].New, trace.Records[7].New[0])
			},
		},
		{
			name: "batch-flush-wrong-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].New[0], trace.Records[10].New[1] = trace.Records[10].New[1], trace.Records[10].New[0]
			},
		},
		{
			name: "record-removed",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "epl-other-distinct.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "epl-other-distinct-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunEplOtherDistinctDiffRejectsMultikeyMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "array-content-key-collapsed",
			mutate: func(trace *compat.Trace) {
				// [1,2] and [2,1] are distinct array keys; collapsing one drops a row.
				trace.Records[13].New = trace.Records[13].New[:len(trace.Records[13].New)-1]
			},
		},
		{
			name: "faf-distinct-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].New = append(trace.Records[15].New, compat.ResultRecord{Kind: "row", Fields: map[string]any{"intOne": []any{int64(1), int64(2)}}})
			},
		},
		{
			name: "iterate-order-broken",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[17].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "on-select-vector-truncated",
			mutate: func(trace *compat.Trace) {
				trace.Records[19].New = trace.Records[19].New[:1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "epl-other-distinct.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "epl-other-distinct-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunEplOtherDistinctDiffRejectsSlice2Mutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "snapshot-threshold-ignored",
			mutate: func(trace *compat.Trace) {
				// Snapshot every 3 events must not fire before the threshold.
				trace.Records[23].New = nil
			},
		},
		{
			name: "snapshot-order-by-broken",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[28].New
				rows[0], rows[len(rows)-1] = rows[len(rows)-1], rows[0]
			},
		},
		{
			name: "subquery-membership-broken",
			mutate: func(trace *compat.Trace) {
				trace.Records[30].New = nil
			},
		},
		{
			name: "ondemand-faf-dedup-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[21].New = append(trace.Records[21].New, compat.ResultRecord{Kind: "row", Fields: map[string]any{"theString": "E1", "intPrimitive": int64(1)}})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "epl-other-distinct.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "epl-other-distinct-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunEplOtherDistinctDiffRejectsWildcardMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "wildcard-duplicate-not-suppressed",
			mutate: func(trace *compat.Trace) {
				// The third bean snapshot must not contain a third row.
				trace.Records[33].New = append(trace.Records[33].New, compat.ResultRecord{Kind: "row", Fields: map[string]any{"theString": "E1", "intPrimitive": int64(1)}})
			},
		},
		{
			name: "computed-column-key-collapsed",
			mutate: func(trace *compat.Trace) {
				// (1,3,8) and (1,3,3) are distinct rows via the computed columns.
				trace.Records[38].New = trace.Records[38].New[:1]
			},
		},
		{
			name: "map-wildcard-row-dropped",
			mutate: func(trace *compat.Trace) {
				trace.Records[41].New = nil
			},
		},
		{
			name: "soda-growth-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[35].New = trace.Records[34].New
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "epl-other-distinct.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "epl-other-distinct-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunEplOtherDistinctDiffRejectsFinaleMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "join-flush-order-broken",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[43].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "insert-route-dedup-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[46].New = append(trace.Records[46].New, compat.ResultRecord{Kind: "row", Fields: map[string]any{"theString": "E1", "intPrimitive": int64(1)}})
			},
		},
		{
			name: "second-flush-order-broken",
			mutate: func(trace *compat.Trace) {
				rows := trace.Records[44].New
				rows[0], rows[1] = rows[1], rows[0]
			},
		},
		{
			name: "pattern-invoked-marker-dropped",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records[:49], trace.Records[50:]...)
			},
		},
		{
			name: "pattern-two-payload-truncated",
			mutate: func(trace *compat.Trace) {
				trace.Records[50].New = trace.Records[50].New[:1]
			},
		},
		{
			name: "variant-intone-key-collapsed",
			mutate: func(trace *compat.Trace) {
				trace.Records[52].New = trace.Records[52].New[:1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "epl-other-distinct.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "epl-other-distinct.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "epl-other-distinct-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunSubselectUnfilteredDiffRejectsLifecycleMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "start-stop-first-matched-while-empty",
			mutate: func(trace *compat.Trace) {
				// First generation must not match before the S1 event arrives.
				trace.Records[49].New = append(trace.Records[49].New, compat.ResultRecord{Kind: "row", Fields: map[string]any{"id": int64(2)}})
			},
		},
		{
			name: "start-stop-second-window-carried-over",
			mutate: func(trace *compat.Trace) {
				// The second deployment starts with an empty subquery window.
				trace.Records[50].New = append(trace.Records[50].New, compat.ResultRecord{Kind: "row", Fields: map[string]any{"id": int64(2)}})
			},
		},
		{
			name: "custom-function-null-lost",
			mutate: func(trace *compat.Trace) {
				// Empty subquery window yields null idS1, not a number.
				trace.Records[51].New[0].Fields["idS1"] = int64(0)
			},
		},
		{
			name: "custom-function-value-wrong",
			mutate: func(trace *compat.Trace) {
				trace.Records[52].New[0].Fields["idS1"] = float64(8)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "subselect-unfiltered.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "subselect-unfiltered.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "subselect-unfiltered.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "subselect-unfiltered-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunStreamSelectorDiffRejectsMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "join-cross-product-broken",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].New = nil
			},
		},
		{
			name: "alias-identity-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["s0"] = "SupportBean[theString=WRONG,intPrimitive=15]"
			},
		},
		{
			name: "reverse-generation-wrong-stream",
			mutate: func(trace *compat.Trace) {
				trace.Records[9].New[0].Fields["szero"] = "{price=0.0;symbol=E1;volume=0}"
			},
		},
		{
			name: "config-istream-old-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].Old = nil
			},
		},
		{
			name: "config-rstream-expired-row-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[16].New = trace.Records[16].New[:0]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "stream-selector.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "stream-selector.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "stream-selector.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "stream-selector-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunVariablesOnsetSetDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "variables-onset-set.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "variables-onset-set.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "variables-onset-set.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "variables-onset-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunVariablesOnsetSetDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "simple-toggle-order-broken",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["c0"] = "true"
			},
		},
		{
			name: "filter-trigger-suppressed",
			mutate: func(trace *compat.Trace) {
				trace.Records[5].New = nil
			},
		},
		{
			name: "chained-order-broken",
			mutate: func(trace *compat.Trace) {
				// var3OND must see var1OND/var2OND after their updates: 3+4=7.
				trace.Records[7].New[0].Fields["var3OND"] = int64(5)
			},
		},
		{
			name: "duplicate-last-write-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[11].New[0].Fields["var1OD"] = "-1"
			},
		},
		{
			name: "coercion-old-stream-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records[26].Old = nil
			},
		},
		{
			// Grouped scalar subquery: a second array group must collapse the
			// assignment to null.
			name: "multikey-warray-second-group-summed",
			mutate: func(trace *compat.Trace) {
				trace.Records[33].Value = int64(46)
			},
		},
		{
			// Array-element writes mutate shared backing storage: [0,1,0].
			name: "array-at-index-element-write-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[37].Value = []any{int64(0), int64(0), int64(0)}
			},
		},
		{
			// Boxed arrays keep per-element nulls and box the written literal.
			name: "array-boxed-null-elements-collapsed",
			mutate: func(trace *compat.Trace) {
				trace.Records[45].New[0].Fields["c0"] = []any{int64(1), int64(1), int64(1)}
			},
		},
		{
			// Out-of-range index failures carry Java's exact root message.
			name: "array-invalid-overflow-message-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[46].Value = "Array length 3 less than index 11 for variable 'doublearray'"
			},
		},
		{
			// Call-form assignments mutate the variable value in place.
			name: "expression-swap-not-applied",
			mutate: func(trace *compat.Trace) {
				trace.Records[53].Value = map[string]any{"a": int64(1), "b": int64(10)}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "variables-onset-set.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "variables-onset-set.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "variables-onset-set.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "variables-onset-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}

func TestRunVariablesUseDiffWritesPassingEvidence(t *testing.T) {
	javaTracePath := writeJavaTraceFixtureFromEvidence(t,
		filepath.Join("..", "..", "..", "testdata", "parity", "variables-use.evidence.json"),
		func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), "variables-use.evidence.json")
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "variables-use.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"-mode", "variables-use-diff",
		"-scenario", scenarioPath,
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestRunVariablesUseDiffRejectsTraceMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			name: "variable-filter-match-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["theString"] = "WRONG"
			},
		},
		{
			name: "or-filter-second-var-lost",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["theString"] = "WRONG"
			},
		},
		{
			name: "same-module-const-flipped",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].New[0].Fields["c0"] = "false"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join("..", "..", "..", "testdata", "parity", "variables-use.evidence.json"),
				test.mutate)
			evidencePath := filepath.Join(t.TempDir(), "variables-use.evidence.json")
			scenarioPath := filepath.Join("..", "..", "..", "testdata", "parity", "variables-use.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", "variables-use-diff",
				"-scenario", scenarioPath,
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			output, err := compat.LoadDifferentialEvidence(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if output.Status != "different" || len(output.Differences) == 0 {
				t.Fatalf("mutation evidence = %#v", output)
			}
		})
	}
}
