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
