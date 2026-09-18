package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// contextDeclaredExpressionCheckJavaMetadata verifies differential evidence
// carries the pinned Java commit, runtime IDs, source file, and execution
// names for the ContextWDeclaredExpression ords 0/1/2 unit.
func contextDeclaredExpressionCheckJavaMetadata(javaCommit string, runtimeIDs, sourceFiles, executions []string) error {
	if javaCommit != contextDeclaredExpressionJavaCommit {
		return fmt.Errorf("Java commit = %q, want %q", javaCommit, contextDeclaredExpressionJavaCommit)
	}
	if !reflect.DeepEqual(runtimeIDs, contextDeclaredExpressionJavaRuntimeIDs) {
		return fmt.Errorf("Java runtime IDs = %v, want %v", runtimeIDs, contextDeclaredExpressionJavaRuntimeIDs)
	}
	if !reflect.DeepEqual(sourceFiles, contextDeclaredExpressionSources) {
		return fmt.Errorf("Java source files = %v, want %v", sourceFiles, contextDeclaredExpressionSources)
	}
	if !reflect.DeepEqual(executions, contextDeclaredExpressionJavaExecutions) {
		return fmt.Errorf("Java executions = %v, want %v", executions, contextDeclaredExpressionJavaExecutions)
	}
	return nil
}

// assertContextDeclaredExpressionTrace pins the full normalized record
// sequence: deploy/deployed bookkeeping plus the two category-case listener
// rows (n/xnx/n and p/xpx/p) and the wfilter pattern row (c0=1, c1=2).
func assertContextDeclaredExpressionTrace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if trace.Version != compat.ScenarioVersion || trace.ID != contextDeclaredExpressionID {
		t.Fatalf("trace identity = %q/%q", trace.Version, trace.ID)
	}
	type line struct {
		caseName  string
		operation string
		statement string
		sequence  uint64
		newRows   string
	}
	expected := []line{
		{"simple", "deployed", "ctx", 1, ""},
		{"simple", "deployed", "expr-1", 1, ""},
		{"simple", "deployed", "expr-2", 1, ""},
		{"simple", "deployed", "s0", 1, ""},
		{"simple", "listener", "s0", 1, `[{"kind":"row","fields":{"c0":"n","c1":"xnx","c2":"n"}}]`},
		{"simple", "listener", "s0", 2, `[{"kind":"row","fields":{"c0":"p","c1":"xpx","c2":"p"}}]`},
		{"alias", "deployed", "ctx", 1, ""},
		{"alias", "deployed", "expr-1", 1, ""},
		{"alias", "deployed", "expr-2", 1, ""},
		{"alias", "deployed", "s0", 1, ""},
		{"alias", "listener", "s0", 1, `[{"kind":"row","fields":{"c0":"n","c1":"xnx","c2":"n"}}]`},
		{"alias", "listener", "s0", 2, `[{"kind":"row","fields":{"c0":"p","c1":"xpx","c2":"p"}}]`},
		{"wfilter", "deployed", "expr", 1, ""},
		{"wfilter", "deployed", "ctx", 1, ""},
		{"wfilter", "deployed", "s0", 1, ""},
		{"wfilter", "listener", "s0", 1, `[{"kind":"row","fields":{"c0":1,"c1":2}}]`},
	}
	if len(trace.Records) != len(expected) {
		t.Fatalf("trace records = %d, want %d", len(trace.Records), len(expected))
	}
	for index, want := range expected {
		record := trace.Records[index]
		newRows := ""
		if record.New != nil {
			encoded, err := json.Marshal(record.New)
			if err != nil {
				t.Fatal(err)
			}
			newRows = string(encoded)
		}
		got := line{
			caseName:  record.Case,
			operation: record.Operation,
			statement: record.Statement,
			sequence:  record.Sequence,
			newRows:   newRows,
		}
		if got != want {
			t.Fatalf("record %d = %#v, want %#v", index, got, want)
		}
	}
}

func TestRunContextDeclaredExpressionDirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", contextDeclaredExpressionID,
		"-scenario", filepath.Join(root, contextDeclaredExpressionID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertContextDeclaredExpressionTrace(t, trace)
}

func TestRunContextDeclaredExpressionDiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	evidencePath := filepath.Join(t.TempDir(), contextDeclaredExpressionID+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", contextDeclaredExpressionID + "-diff",
		"-scenario", filepath.Join(root, contextDeclaredExpressionID+".json"),
		"-java-trace", filepath.Join(root, contextDeclaredExpressionID+".trace.json"),
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
	if err := contextDeclaredExpressionCheckJavaMetadata(evidence.JavaCommit, evidence.JavaRuntimeIDs,
		evidence.JavaSourceFiles, evidence.JavaExecutions); err != nil {
		t.Fatalf("Java metadata: %v", err)
	}
	assertContextDeclaredExpressionTrace(t, evidence.JavaTrace)
	assertContextDeclaredExpressionTrace(t, evidence.GoTrace)
}

func TestRunContextDeclaredExpressionDiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// Record 4 is simple's first listener row: the category label
			// must be "n" for intPrimitive=-2.
			name: "simple-label-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[4].New[0].Fields["c0"] = "p"
			},
		},
		{
			// Record 10 is alias's first listener row: the concat expression
			// must wrap the label in x...x.
			name: "alias-concat-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[10].New[0].Fields["c1"] = "nxn"
			},
		},
		{
			// Record 15 is wfilter's only listener row: e1 binds the
			// initiating x event (intPrimitive=1).
			name: "wfilter-e1-drift",
			mutate: func(trace *compat.Trace) {
				trace.Records[15].New[0].Fields["c0"] = 2
			},
		},
		{
			// The wfilter listener row must exist exactly once: dropping it
			// removes the only evidence the initiating event reached the
			// partition's pattern.
			name: "wfilter-row-missing",
			mutate: func(trace *compat.Trace) {
				trace.Records = append(trace.Records[:15], trace.Records[16:]...)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTrace, err := loadTraceFile(filepath.Join(root, contextDeclaredExpressionID+".trace.json"))
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&javaTrace)
			mutatedPath := filepath.Join(t.TempDir(), contextDeclaredExpressionID+".trace.json")
			data, err := json.Marshal(javaTrace)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(mutatedPath, data, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := Run([]string{
				"-mode", contextDeclaredExpressionID + "-diff",
				"-scenario", filepath.Join(root, contextDeclaredExpressionID+".json"),
				"-java-trace", mutatedPath,
				"-evidence", filepath.Join(t.TempDir(), "evidence.json"),
			}, &stdout, &stderr); code == 0 {
				t.Fatalf("mutated trace %q passed the diff", test.name)
			}
		})
	}
}

func TestRunContextDeclaredExpressionCheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTrace, err := loadTraceFile(filepath.Join(root, contextDeclaredExpressionID+".trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	goTrace, err := loadTraceFile(filepath.Join(root, contextDeclaredExpressionID+".go.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := loadDifferentialEvidenceFile(filepath.Join(root, contextDeclaredExpressionID+".evidence.json"))
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
		t.Fatalf("checked-in evidence Go trace differs from evidence Go trace: %#v", differences)
	}
	if err := contextDeclaredExpressionCheckJavaMetadata(evidence.JavaCommit, evidence.JavaRuntimeIDs,
		evidence.JavaSourceFiles, evidence.JavaExecutions); err != nil {
		t.Fatalf("checked-in Java metadata: %v", err)
	}
	assertContextDeclaredExpressionTrace(t, javaTrace)
	assertContextDeclaredExpressionTrace(t, goTrace)

	scenarioPath := filepath.Join(root, contextDeclaredExpressionID+".json")
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
		contextDeclaredExpressionJavaCommit,
		contextDeclaredExpressionJavaRuntimeIDs,
		contextDeclaredExpressionSources,
		contextDeclaredExpressionJavaExecutions,
		scenario, javaTrace, evidence.GoTrace)
	if err != nil {
		t.Fatal(err)
	}
	if canonicalEvidence.Status != "passing" || len(canonicalEvidence.Differences) != 0 {
		t.Fatalf("checked-in Java trace is not a passing comparison: %#v", canonicalEvidence.Differences)
	}

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", contextDeclaredExpressionID,
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
	assertContextDeclaredExpressionTrace(t, replayed)
}

func TestRunContextDeclaredExpressionRejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, contextDeclaredExpressionID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "top-level-extra", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"steps": [`), []byte(`"extra": 0, "steps": [`), 1)
		}},
		{name: "unknown-op", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"op": "undeploy-all"`), []byte(`"op": "close-all"`), 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), contextDeclaredExpressionID+".json")
			if err := os.WriteFile(path, test.mutate(data), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"-mode", contextDeclaredExpressionID, "-scenario", path}, &stdout, &stderr); code == 0 {
				t.Fatalf("malformed scenario %q was accepted", test.name)
			}
		})
	}
}

func TestRunContextDeclaredExpressionRuntimeIDMappingMatchesScenario(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", contextDeclaredExpressionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaFlags    []string `json:"javaFlags"`
		Cases        []struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		} `json:"cases"`
		Steps []struct {
			Op        string `json:"op"`
			Case      string `json:"case"`
			Statement string `json:"statement"`
			EventType string `json:"eventType"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document.Version != compat.ScenarioVersion || document.ID != contextDeclaredExpressionID ||
		document.Description != contextDeclaredExpressionDescription ||
		document.JavaCommit != contextDeclaredExpressionJavaCommit ||
		document.JavaSource != contextDeclaredExpressionSource {
		t.Fatalf("scenario identity = %q/%q/%q/%q/%q", document.Version, document.ID,
			document.Description, document.JavaCommit, document.JavaSource)
	}
	if !reflect.DeepEqual(document.JavaRuntimes, contextDeclaredExpressionJavaRuntimeIDs) {
		t.Fatalf("scenario javaRuntimes = %v, want %v", document.JavaRuntimes, contextDeclaredExpressionJavaRuntimeIDs)
	}
	if !reflect.DeepEqual(document.JavaNames, contextDeclaredExpressionJavaExecutions) {
		t.Fatalf("scenario javaNames = %v, want %v", document.JavaNames, contextDeclaredExpressionJavaExecutions)
	}
	if !reflect.DeepEqual(document.JavaFlags, contextDeclaredExpressionJavaFlags) {
		t.Fatalf("scenario javaFlags = %v, want %v", document.JavaFlags, contextDeclaredExpressionJavaFlags)
	}
	if len(document.Cases) != len(contextDeclaredExpressionCases) {
		t.Fatalf("scenario cases = %d, want %d", len(document.Cases), len(contextDeclaredExpressionCases))
	}
	for index, entry := range document.Cases {
		if entry.Case != contextDeclaredExpressionCases[index] ||
			entry.Ordinal != contextDeclaredExpressionOrdinals[index] ||
			entry.RuntimeID != contextDeclaredExpressionJavaRuntimeIDs[index] ||
			entry.ExecutionName != contextDeclaredExpressionJavaExecutions[index] ||
			entry.Observation != contextDeclaredExpressionCaseObservations[index] ||
			entry.EPL != contextDeclaredExpressionCaseEPLs[index] {
			t.Fatalf("scenario case %d = %#v", index, entry)
		}
	}
	// Step order per case: case marker, then the pinned case steps.
	offset := 0
	for _, caseName := range contextDeclaredExpressionCases {
		if offset >= len(document.Steps) {
			t.Fatalf("missing case %q", caseName)
		}
		if document.Steps[offset].Op != "case" || document.Steps[offset].Case != caseName {
			t.Fatalf("step %d is not the %q case marker", offset, caseName)
		}
		offset++
		for _, want := range contextDeclaredExpressionCaseSteps[caseName] {
			if offset >= len(document.Steps) {
				t.Fatalf("case %q ran out of steps", caseName)
			}
			step := document.Steps[offset]
			parts := strings.SplitN(want, "|", 3)
			if step.Op != parts[0] || step.Statement != parts[1] || step.EventType != parts[2] {
				t.Fatalf("case %q step %d = %s|%s|%s, want %s", caseName, offset,
					step.Op, step.Statement, step.EventType, want)
			}
			offset++
		}
	}
	if offset != len(document.Steps) {
		t.Fatalf("scenario has %d trailing steps", len(document.Steps)-offset)
	}
}
