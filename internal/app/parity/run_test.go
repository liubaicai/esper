package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
