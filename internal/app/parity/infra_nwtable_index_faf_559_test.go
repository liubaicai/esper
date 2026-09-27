package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// infraNWTableIndexFAF559JavaTraceFixture writes the Java-side trace the
// diff assertions run against. When the unit's Java trace has been landed
// next to the scenario the checked-in file wins; before that the fixture is
// the same 28 records assembled from the runner's pinned constants (record
// shapes verified 0-diff against the oracle run for
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c).
func infraNWTableIndexFAF559JavaTraceFixture(t *testing.T, mutate func(*compat.Trace)) string {
	t.Helper()
	checkedIn := filepath.Join("..", "..", "..", "testdata", "parity",
		infraNWTableIndexFAF559ID+".trace.json")
	if _, err := os.Stat(checkedIn); err == nil {
		return writeJavaTraceFixtureFromTrace(t, checkedIn, mutate)
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: infraNWTableIndexFAF559ID}
	epoch := "1970-01-01T00:00:00Z"
	deploy := func(caseName, statement string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "deployed", Statement: statement,
			Sequence: 1, Time: epoch,
		})
	}
	row := func() []compat.ResultRecord {
		return []compat.ResultRecord{{
			Kind: "row",
			Fields: map[string]any{
				"f1": "E1", "f2": json.Number("-2"), "f3": ">E1<", "f4": "?E1?",
			},
		}}
	}
	snapshot := func(caseName, statement string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "snapshot", Statement: statement,
			Time: epoch, New: row(),
		})
	}
	for _, caseName := range infraNWTableIndexFAF559Cases {
		deploy(caseName, "create")
		deploy(caseName, "insert")
		deploy(caseName, "index")
		snapshot(caseName, "select-f3")
		snapshot(caseName, "select-f3-f2")
		snapshot(caseName, "select-full")
		if strings.HasPrefix(caseName, "composite") {
			trace.Records = append(trace.Records,
				compat.TraceRecord{Case: caseName, Operation: "unrepresentable",
					Statement: "undeploy-index-one", Value: infraNWTableCI559UndeployIndexNote},
				compat.TraceRecord{Case: caseName, Operation: "unrepresentable",
					Statement: "soda-index-two", Value: infraNWTableCI559SodaIndexTwoNote})
		}
	}
	if len(trace.Records) != 28 {
		t.Fatalf("fixture records = %d, want 28", len(trace.Records))
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

// assertInfraNWTableIndexFAF559Trace pins the 28-record index-faf trace:
// per case three deployed markers (sequence 1, in order create, insert,
// index), the three f3-leading FAF snapshots each returning exactly
// {f1:E1, f2:-2, f3:>E1<, f4:?E1?}, and the composite cases' two plan-only
// SODA-tail records. Milestone and undeploy steps emit no records.
func assertInfraNWTableIndexFAF559Trace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if trace.ID != infraNWTableIndexFAF559ID || trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace identity = %q/%q", trace.ID, trace.Version)
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	for _, caseName := range infraNWTableIndexFAF559Cases {
		records := byCase[caseName]
		wantCount := 6
		if strings.HasPrefix(caseName, "composite") {
			wantCount = 8
		}
		if len(records) != wantCount {
			t.Fatalf("case %q records = %d, want %d", caseName, len(records), wantCount)
		}
		var deployed []string
		var snapshots map[string][]compat.ResultRecord = map[string][]compat.ResultRecord{}
		var probes []string
		for _, record := range records {
			switch record.Operation {
			case "deployed":
				if record.Sequence != 1 || record.Time != "1970-01-01T00:00:00Z" {
					t.Fatalf("case %q deployed record %#v", caseName, record)
				}
				deployed = append(deployed, record.Statement)
			case "snapshot":
				if record.Sequence != 0 || record.Time != "1970-01-01T00:00:00Z" {
					t.Fatalf("case %q snapshot record %#v", caseName, record)
				}
				snapshots[record.Statement] = record.New
			case "unrepresentable":
				if record.Sequence != 0 || record.Time != "" {
					t.Fatalf("case %q unrepresentable record %#v", caseName, record)
				}
				probes = append(probes, record.Statement)
			default:
				t.Fatalf("case %q unexpected operation %q", caseName, record.Operation)
			}
		}
		wantDeployed := []string{"create", "insert", "index"}
		if !reflect.DeepEqual(deployed, wantDeployed) {
			t.Fatalf("case %q deployed order = %v", caseName, deployed)
		}
		for _, statement := range []string{"select-f3", "select-f3-f2", "select-full"} {
			rows := snapshots[statement]
			if len(rows) != 1 {
				t.Fatalf("case %q %s row count = %d, want 1", caseName, statement, len(rows))
			}
			row := rows[0]
			if row.Kind != "row" || len(row.Fields) != 4 ||
				row.Fields["f1"] != "E1" || row.Fields["f3"] != ">E1<" || row.Fields["f4"] != "?E1?" {
				t.Fatalf("case %q %s row = %#v", caseName, statement, row)
			}
			if row.Fields["f2"] != float64(-2) && row.Fields["f2"] != json.Number("-2") && row.Fields["f2"] != int64(-2) {
				t.Fatalf("case %q %s f2 = %#v, want -2", caseName, statement, row.Fields["f2"])
			}
		}
		if strings.HasPrefix(caseName, "composite") {
			wantProbes := []string{"undeploy-index-one", "soda-index-two"}
			if !reflect.DeepEqual(probes, wantProbes) {
				t.Fatalf("case %q unrepresentable probes = %v", caseName, probes)
			}
		} else if len(probes) != 0 {
			t.Fatalf("case %q unexpected unrepresentable probes = %v", caseName, probes)
		}
	}
}

func TestRunInfraNWTableIndexFAF559DirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableIndexFAF559ID,
		"-scenario", filepath.Join(root, infraNWTableIndexFAF559ID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertInfraNWTableIndexFAF559Trace(t, trace)
}

// TestRunInfraNWTableIndexFAF559PinnedArtifacts pins the byte-exact EPL
// constants transcribed from InfraNWTableCreateIndex.java lines 430-457
// (InfraCompositeIndex) and 645-672 (InfraMultikeyIndexFAF) plus the
// per-case step sequences, so a transcription slip fails the unit even
// when the checked-in scenario is regenerated.
func TestRunInfraNWTableIndexFAF559PinnedArtifacts(t *testing.T) {
	wants := map[string]string{
		"ci-create-window": "@public create window MyInfraCI#keepall as (f1 string, f2 int, f3 string, f4 string)",
		"ci-create-table":  "@public create table MyInfraCI as (f1 string primary key, f2 int, f3 string, f4 string)",
		"ci-insert":        "insert into MyInfraCI(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean",
		"ci-index":         "@name('indexOne') create index MyInfraCIIndex on MyInfraCI(f2, f3, f1)",
		"ci-select-f3":     "select * from MyInfraCI where f3='>E1<'",
		"ci-select-f3-f2":  "select * from MyInfraCI where f3='>E1<' and f2=-2",
		"ci-select-full":   "select * from MyInfraCI where f3='>E1<' and f2=-2 and f1='E1'",
		"ci-soda-two":      "create index MyInfraCIIndexTwo on MyInfraCI(f2, f3, f1)",
		"mk-create-window": "@public create window MyInfra.win:keepall() as (f1 string, f2 int, f3 string, f4 string)",
		"mk-create-table":  "@public create table MyInfra as (f1 string primary key, f2 int, f3 string, f4 string)",
		"mk-insert":        "insert into MyInfra(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean",
		"mk-index":         "create index MyInfraIndex on MyInfra(f2, f3, f1)",
		"mk-select-f3":     "select * from MyInfra where f3='>E1<'",
		"mk-select-f3-f2":  "select * from MyInfra where f3='>E1<' and f2=-2",
		"mk-select-full":   "select * from MyInfra where f3='>E1<' and f2=-2 and f1='E1'",
	}
	got := map[string]string{
		"ci-create-window": infraNWTableCI559CreateWindow,
		"ci-create-table":  infraNWTableCI559CreateTable,
		"ci-insert":        infraNWTableCI559Insert,
		"ci-index":         infraNWTableCI559Index,
		"ci-select-f3":     infraNWTableCI559SelectF3,
		"ci-select-f3-f2":  infraNWTableCI559SelectF3F2,
		"ci-select-full":   infraNWTableCI559SelectFull,
		"ci-soda-two":      infraNWTableCI559SodaIndexTwo,
		"mk-create-window": infraNWTableMK559CreateWindow,
		"mk-create-table":  infraNWTableMK559CreateTable,
		"mk-insert":        infraNWTableMK559Insert,
		"mk-index":         infraNWTableMK559Index,
		"mk-select-f3":     infraNWTableMK559SelectF3,
		"mk-select-f3-f2":  infraNWTableMK559SelectF3F2,
		"mk-select-full":   infraNWTableMK559SelectFull,
	}
	for name, want := range wants {
		if got[name] != want {
			t.Fatalf("EPL %s = %q, want %q", name, got[name], want)
		}
	}
	if len(infraNWTableIndexFAF559CaseSteps["composite-window"]) != 13 ||
		len(infraNWTableIndexFAF559CaseSteps["multikey-window"]) != 11 {
		t.Fatalf("pinned step sequences = %d/%d",
			len(infraNWTableIndexFAF559CaseSteps["composite-window"]),
			len(infraNWTableIndexFAF559CaseSteps["multikey-window"]))
	}
}

// TestRunInfraNWTableIndexFAF559ScenarioLoader pins the strict loader: the
// checked-in scenario parses, and runtime-id drift plus step-count drift
// are both rejected.
func TestRunInfraNWTableIndexFAF559ScenarioLoader(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableIndexFAF559ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	_, loadErr := loadInfraNWTableIndexFAF559Scenario(scenarioFile)
	closeErr := scenarioFile.Close()
	if loadErr != nil {
		t.Fatalf("checked-in scenario rejected: %v", loadErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	var mutated map[string]any
	if err := json.Unmarshal(data, &mutated); err != nil {
		t.Fatal(err)
	}
	runtimes := mutated["javaRuntimes"].([]any)
	mutated["javaRuntimes"] = []any{"java-runtime-mutated", runtimes[1], runtimes[2], runtimes[3]}
	raw, err := json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableIndexFAF559Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("runtime-id drift accepted")
	}
	mutated["javaRuntimes"] = runtimes
	steps := mutated["steps"].([]any)
	mutated["steps"] = steps[:len(steps)-1]
	raw, err = json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableIndexFAF559Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("step-count drift accepted")
	}
}

func TestRunInfraNWTableIndexFAF559DiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := infraNWTableIndexFAF559JavaTraceFixture(t, func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), infraNWTableIndexFAF559ID+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableIndexFAF559ID + "-diff",
		"-scenario", filepath.Join(root, infraNWTableIndexFAF559ID+".json"),
		"-java-trace", javaTracePath,
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
	if evidence.JavaCommit != infraNWTableIndexFAF559JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableIndexFAF559JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableIndexFAF559JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableIndexFAF559JavaExecutions) {
		t.Fatalf("Java metadata = %#v", evidence)
	}
	assertInfraNWTableIndexFAF559Trace(t, evidence.JavaTrace)
	assertInfraNWTableIndexFAF559Trace(t, evidence.GoTrace)
}

func TestRunInfraNWTableIndexFAF559DiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// The composite f3-leading probe pins f3='>E1<'; a drifted
			// concat value means the insert-into feed lost the '>'||'<'
			// decoration.
			name: "concat-value-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "composite-window" && rec.Statement == "select-f3" && len(rec.New) > 0 {
						rec.New[0].Fields["f3"] = "E1"
						return
					}
				}
				panic("no select-f3 snapshot record")
			},
		},
		{
			// The multikey full-predicate probe pins {E1,-2,>E1<,?E1?};
			// drifting the projected int key breaks the row.
			name: "multikey-int-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "multikey-table" && rec.Statement == "select-full" && len(rec.New) > 0 {
						rec.New[0].Fields["f2"] = json.Number("-4")
						return
					}
				}
				panic("no select-full snapshot record")
			},
		},
		{
			// The SODA IX2 probe must stay the plan-only unrepresentable
			// record; retyping it to a snapshot silently downgrades the
			// probe.
			name: "soda-probe-retyped",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					if trace.Records[index].Statement == "soda-index-two" {
						trace.Records[index].Operation = "snapshot"
						return
					}
				}
				panic("no soda-index-two record")
			},
		},
		{
			name: "deployed-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "record-count-short",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := infraNWTableIndexFAF559JavaTraceFixture(t, test.mutate)
			evidencePath := filepath.Join(t.TempDir(), infraNWTableIndexFAF559ID+".evidence.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", infraNWTableIndexFAF559ID + "-diff",
				"-scenario", filepath.Join(root, infraNWTableIndexFAF559ID+".json"),
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

// TestRunInfraNWTableIndexFAF559CheckedInEvidenceMatchesTraceAndReplay
// verifies the landed Java trace, Go trace and evidence against the
// runner's pinned constants; the test is inert until the unit's
// differential artifacts are checked in next to the scenario.
func TestRunInfraNWTableIndexFAF559CheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := filepath.Join(root, infraNWTableIndexFAF559ID+".trace.json")
	goTracePath := filepath.Join(root, infraNWTableIndexFAF559ID+".go.trace.json")
	evidencePath := filepath.Join(root, infraNWTableIndexFAF559ID+".evidence.json")
	for _, path := range []string{javaTracePath, goTracePath, evidencePath} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("differential artifacts not checked in yet: %v", err)
		}
	}
	javaTrace, err := loadTraceFile(javaTracePath)
	if err != nil {
		t.Fatal(err)
	}
	goTrace, err := loadTraceFile(goTracePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := loadDifferentialEvidenceFile(evidencePath)
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
	if evidence.JavaCommit != infraNWTableIndexFAF559JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableIndexFAF559JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableIndexFAF559JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableIndexFAF559JavaExecutions) {
		t.Fatalf("checked-in Java metadata = %#v", evidence)
	}
	assertInfraNWTableIndexFAF559Trace(t, javaTrace)
	assertInfraNWTableIndexFAF559Trace(t, goTrace)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableIndexFAF559ID,
		"-scenario", filepath.Join(root, infraNWTableIndexFAF559ID+".json"),
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
	assertInfraNWTableIndexFAF559Trace(t, replayed)
}

func TestRunInfraNWTableIndexFAF559RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableIndexFAF559ID+".json"))
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
		{name: "step-extra", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`{"op": "deploy", "case": "composite-window", "statement": "create"`),
				[]byte(`{"op": "deploy", "case": "composite-window", "statement": "create", "extra": 0`), 1)
		}},
		{name: "case-runtime-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"runtimeId": "java-runtime-78145a646789e9674a38"`),
				[]byte(`"runtimeId": "java-runtime-wrong"`), 1)
		}},
		{name: "epl-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`select * from MyInfraCI where f3='>E1<' and f2=-2 and f1='E1'`),
				[]byte(`select * from MyInfraCI where f3='>E1<' and f2=-2 and f1='E2'`), 1)
		}},
		{name: "payload-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"intPrimitive": -2`),
				[]byte(`"intPrimitive": -3`), 1)
		}},
		{name: "undeploy-key-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"statement": "undeploy-index-one", "epl": "indexOne"`),
				[]byte(`"statement": "undeploy-index-one", "epl": "indexTwo"`), 1)
		}},
		{name: "soda-note-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`EPL-object-model deploy has no Go boundary`),
				[]byte(`EPL-object-model deploy has a Go boundary`), 1)
		}},
		{name: "trailing-json", mutate: func(data []byte) []byte {
			return append(append([]byte(nil), data...), []byte("\n{}\n")...)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := test.mutate(data)
			if bytes.Equal(mutated, data) {
				t.Fatalf("raw mutation %q did not change scenario", test.name)
			}
			scenarioPath := filepath.Join(t.TempDir(), "scenario.json")
			if err := os.WriteFile(scenarioPath, mutated, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := Run([]string{
				"-mode", infraNWTableIndexFAF559ID,
				"-scenario", scenarioPath,
			}, &stdout, &stderr); code == 0 {
				t.Fatalf("malformed scenario %q unexpectedly replayed: stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunInfraNWTableIndexFAF559RuntimeIDMappingMatchesScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableIndexFAF559ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		JavaRuntimes  []string `json:"javaRuntimes"`
		JavaNames     []string `json:"javaNames"`
		JavaStaticIDs []string `json:"javaStaticIds"`
		JavaFlags     []string `json:"javaFlags"`
		Cases         []struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw.JavaRuntimes, infraNWTableIndexFAF559JavaRuntimeIDs) ||
		!reflect.DeepEqual(raw.JavaNames, infraNWTableIndexFAF559JavaExecutions) ||
		!reflect.DeepEqual(raw.JavaStaticIDs, infraNWTableIndexFAF559JavaStaticIDs) ||
		!reflect.DeepEqual(raw.JavaFlags, infraNWTableIndexFAF559JavaFlags) {
		t.Fatalf("scenario Java identity = %#v", raw)
	}
	for index, definition := range raw.Cases {
		if definition.Case != infraNWTableIndexFAF559Cases[index] ||
			definition.Ordinal != infraNWTableIndexFAF559Ordinals[index] ||
			definition.RuntimeID != infraNWTableIndexFAF559JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableIndexFAF559JavaExecutions[index] {
			t.Fatalf("case %d mapping = %#v", index, definition)
		}
	}
}
