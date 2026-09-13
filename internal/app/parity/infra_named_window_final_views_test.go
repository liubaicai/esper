package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

func TestRunInfraNamedWindowFinalViewsDirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWFVId,
		"-scenario", filepath.Join(root, infraNWFVId+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertInfraNWFVTrace(t, trace)
}

func TestRunInfraNamedWindowFinalViewsDiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	evidencePath := filepath.Join(t.TempDir(), infraNWFVId+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWFVId + "-diff",
		"-scenario", filepath.Join(root, infraNWFVId+".json"),
		"-java-trace", filepath.Join(root, infraNWFVId+".trace.json"),
		"-evidence", evidencePath,
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("diff exit code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("passing diff wrote stdout = %q", stdout.String())
	}
	evidence, err := loadDifferentialEvidenceFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
	if err := infraNWFVCheckJavaMetadata(evidence.JavaCommit, evidence.JavaRuntimeIDs,
		evidence.JavaSourceFiles, evidence.JavaExecutions); err != nil {
		t.Fatalf("Java metadata: %v", err)
	}
	assertInfraNWFVTrace(t, evidence.JavaTrace)
	assertInfraNWFVTrace(t, evidence.GoTrace)
}

func TestRunInfraNamedWindowFinalViewsDiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// Record 0 is the first S1 pattern fire: value 2.
			name: "pattern-first-value-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[0].New[0].Fields["value"] = json.Number("99")
			},
		},
		{
			// Record 1 is the re-armed S1 fire: sequence must stay 2.
			name: "pattern-second-sequence-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[1].Sequence = 9
			},
		},
		{
			// Record 2 is the S2 fire that quits the or-expression.
			name: "pattern-s2-key-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[2].New[0].Fields["key"] = "S9"
			},
		},
		{
			// Record 4 is the first post-delete snapshot: E2 must be gone.
			name: "ttl-first-delete-loss",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New = append(trace.Records[4].New, trace.Records[3].New[1])
			},
		},
		{
			// Record 6 is the post-expiry snapshot: only E4 survives t=1000.
			name: "ttl-expiry-time-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[6].Time = "1970-01-01T00:00:02Z"
			},
		},
		{
			name: "record-count-short",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:7]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := writeJavaTraceFixtureFromEvidence(t,
				filepath.Join(root, infraNWFVId+".evidence.json"), test.mutate)
			evidencePath := filepath.Join(t.TempDir(), infraNWFVId+".evidence.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", infraNWFVId + "-diff",
				"-scenario", filepath.Join(root, infraNWFVId+".json"),
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation %q unexpectedly passed; stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
			evidence, err := loadDifferentialEvidenceFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation %q evidence = %#v", test.name, evidence)
			}
		})
	}
}

func TestRunInfraNamedWindowFinalViewsCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTrace, err := loadTraceFile(filepath.Join(root, infraNWFVId+".trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	goTrace, err := loadTraceFile(filepath.Join(root, infraNWFVId+".go.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := loadDifferentialEvidenceFile(filepath.Join(root, infraNWFVId+".evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if differences := compat.DiffTraces(goTrace, evidence.GoTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from checked-in Go trace: %#v", differences)
	}
	if err := infraNWFVCheckJavaMetadata(evidence.JavaCommit, evidence.JavaRuntimeIDs,
		evidence.JavaSourceFiles, evidence.JavaExecutions); err != nil {
		t.Fatalf("checked-in Java metadata: %v", err)
	}
	assertInfraNWFVTrace(t, javaTrace)
	assertInfraNWFVTrace(t, goTrace)

	scenarioPath := filepath.Join(root, infraNWFVId+".json")
	scenarioData, err := os.ReadFile(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	var rawScenario struct {
		Version string        `json:"version"`
		ID      string        `json:"id"`
		Steps   []compat.Step `json:"steps"`
	}
	if err := json.Unmarshal(scenarioData, &rawScenario); err != nil {
		t.Fatal(err)
	}
	scenario := compat.Scenario{Version: rawScenario.Version, ID: rawScenario.ID, Steps: rawScenario.Steps}
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

	canonicalEvidence, err := compat.NewDifferentialEvidence(
		infraNWFVJavaCommit,
		infraNWFVJavaRuntimeIDs,
		infraNWFVJavaSources,
		infraNWFVJavaExecutions,
		scenario, javaTrace, goTrace)
	if err != nil {
		t.Fatal(err)
	}
	if canonicalEvidence.Status != "passing" || len(canonicalEvidence.Differences) != 0 {
		t.Fatalf("checked-in traces are not a passing comparison: %#v", canonicalEvidence.Differences)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWFVId,
		"-scenario", scenarioPath,
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	replayed, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, replayed); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
	assertInfraNWFVTrace(t, replayed)
}

func TestRunInfraNamedWindowFinalViewsRejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWFVId+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "cases")
	mutated, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(t.TempDir(), infraNWFVId+".json")
	if err := os.WriteFile(scenarioPath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWFVId,
		"-scenario", scenarioPath,
	}, &stdout, &stderr); code == 0 {
		t.Fatalf("malformed scenario unexpectedly passed; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunInfraNamedWindowFinalViewsRuntimeIDMappingMatchesScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWFVId+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var scenario struct {
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
	}
	if err := json.Unmarshal(data, &scenario); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scenario.JavaRuntimes, infraNWFVJavaRuntimeIDs) {
		t.Fatalf("scenario javaRuntimes = %v, want %v", scenario.JavaRuntimes, infraNWFVJavaRuntimeIDs)
	}
	if !reflect.DeepEqual(scenario.JavaNames, infraNWFVJavaExecutions) {
		t.Fatalf("scenario javaNames = %v, want %v", scenario.JavaNames, infraNWFVJavaExecutions)
	}
}

// assertInfraNWFVTrace pins the 8-record final-slice trace: three ordered
// pattern fires on s0, then five any-order TTL iterator snapshots of win.
func assertInfraNWFVTrace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if trace.Version != compat.ScenarioVersion || trace.ID != infraNWFVId {
		t.Fatalf("trace identity = %q/%q", trace.Version, trace.ID)
	}
	type line struct {
		caseName  string
		operation string
		statement string
		sequence  uint64
		time      string
		newRows   string
		newCount  int
	}
	render := func(rows []compat.ResultRecord) string {
		parts := make([]string, 0, len(rows))
		for _, row := range rows {
			fields := make([]string, 0, len(row.Fields))
			for name, value := range row.Fields {
				fields = append(fields, name+":"+stringifyInfraNWFVField(value))
			}
			sort.Strings(fields)
			parts = append(parts, strings.Join(fields, ","))
		}
		sort.Strings(parts)
		return strings.Join(parts, "|")
	}
	expected := []line{
		{"pattern", "listener", "s0", 1, "1970-01-01T00:00:00Z", "key:S1,value:2", 1},
		{"pattern", "listener", "s0", 2, "1970-01-01T00:00:00Z", "key:S1,value:3", 1},
		{"pattern", "listener", "s0", 3, "1970-01-01T00:00:00Z", "key:S2,value:4", 1},
		{"ttl-delete", "snapshot", "win", 0, "1970-01-01T00:00:00Z", "theString:E1|theString:E2|theString:E3|theString:E4", 4},
		{"ttl-delete", "snapshot", "win", 0, "1970-01-01T00:00:00.500Z", "theString:E1|theString:E3|theString:E4", 3},
		{"ttl-delete", "snapshot", "win", 0, "1970-01-01T00:00:00.500Z", "theString:E3|theString:E4", 2},
		{"ttl-delete", "snapshot", "win", 0, "1970-01-01T00:00:01Z", "theString:E4", 1},
		{"ttl-delete", "snapshot", "win", 0, "1970-01-01T00:00:02Z", "", 0},
	}
	if len(trace.Records) != len(expected) {
		t.Fatalf("trace records = %d, want %d", len(trace.Records), len(expected))
	}
	for index, want := range expected {
		got := trace.Records[index]
		gotNew := render(got.New)
		if got.Case != want.caseName || got.Operation != want.operation ||
			got.Statement != want.statement || got.Sequence != want.sequence ||
			got.Time != want.time || gotNew != want.newRows || len(got.New) != want.newCount {
			t.Fatalf("record %d = %s|%s|%s|%d|%s|%s(%d), want %s|%s|%s|%d|%s|%s(%d)", index,
				got.Case, got.Operation, got.Statement, got.Sequence, got.Time,
				gotNew, len(got.New),
				want.caseName, want.operation, want.statement, want.sequence, want.time,
				want.newRows, want.newCount)
		}
		if len(got.Old) != 0 {
			t.Fatalf("record %d has old rows = %#v, want none", index, got.Old)
		}
	}
}

func stringifyInfraNWFVField(value any) string {
	switch typed := value.(type) {
	case json.Number:
		return typed.String()
	case string:
		return typed
	default:
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprint(value), "\n", ""), "\r", ""))
	}
}

// infraNWFVCheckJavaMetadata verifies differential evidence carries the pinned
// Java commit, runtime IDs, source files and executions.
func infraNWFVCheckJavaMetadata(javaCommit string, runtimeIDs, sourceFiles, executions []string) error {
	if javaCommit != infraNWFVJavaCommit {
		return fmt.Errorf("Java commit = %q, want %q", javaCommit, infraNWFVJavaCommit)
	}
	if !reflect.DeepEqual(runtimeIDs, infraNWFVJavaRuntimeIDs) {
		return fmt.Errorf("Java runtime IDs = %v, want %v", runtimeIDs, infraNWFVJavaRuntimeIDs)
	}
	if !reflect.DeepEqual(sourceFiles, infraNWFVJavaSources) {
		return fmt.Errorf("Java source files = %v, want %v", sourceFiles, infraNWFVJavaSources)
	}
	if !reflect.DeepEqual(executions, infraNWFVJavaExecutions) {
		return fmt.Errorf("Java executions = %v, want %v", executions, infraNWFVJavaExecutions)
	}
	return nil
}
