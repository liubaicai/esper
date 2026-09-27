package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// infraNWTableLateIndex560JavaTraceFixture writes the Java-side trace the
// diff assertions run against. When the unit's Java trace has been landed
// next to the scenario the checked-in file wins; before that the fixture is
// the same 16 records assembled from the runner's pinned constants (record
// shapes verified 0-diff against the oracle run for
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c).
func infraNWTableLateIndex560JavaTraceFixture(t *testing.T, mutate func(*compat.Trace)) string {
	t.Helper()
	checkedIn := filepath.Join("..", "..", "..", "testdata", "parity",
		infraNWTableLateIndex560ID+".trace.json")
	if _, err := os.Stat(checkedIn); err == nil {
		return writeJavaTraceFixtureFromTrace(t, checkedIn, mutate)
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: infraNWTableLateIndex560ID}
	epoch := "1970-01-01T00:00:00Z"
	deploy := func(caseName, statement string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "deployed", Statement: statement,
			Sequence: 1, Time: epoch,
		})
	}
	lateRows := func() []compat.ResultRecord {
		return []compat.ResultRecord{
			{Kind: "row", Fields: map[string]any{
				"theString": "B2", "intPrimitive": json.Number("1"),
			}},
			{Kind: "row", Fields: map[string]any{
				"theString": "B2", "intPrimitive": json.Number("2"),
			}},
		}
	}
	sceneTwoRows := func() []compat.ResultRecord {
		rows := make([]compat.ResultRecord, 0, 3)
		for _, f2 := range []int64{-4, -3, -2} {
			rows = append(rows, compat.ResultRecord{
				Kind: "row",
				Fields: map[string]any{
					"f1": "E1",
					"f2": json.Number(fmt.Sprintf("%d", f2)),
					"f3": ">E1<",
					"f4": "?E1?",
				},
			})
		}
		return rows
	}
	for _, caseName := range infraNWTableLateIndex560Cases {
		deploy(caseName, "create")
		deploy(caseName, "insert")
		deploy(caseName, "index")
		var rows []compat.ResultRecord
		if strings.HasPrefix(caseName, "scene-two") {
			rows = sceneTwoRows()
		} else {
			rows = lateRows()
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "snapshot", Statement: "select",
			Time: epoch, New: rows,
		})
	}
	if len(trace.Records) != 16 {
		t.Fatalf("fixture records = %d, want 16", len(trace.Records))
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

// assertInfraNWTableLateIndex560Trace pins the 16-record late-index trace:
// per case three deployed markers (sequence 1, in order create, insert,
// index) and the single ordered FAF snapshot — {B2,1},{B2,2} for the
// late-create cases and the three E1 rows in f2 order {-4,-3,-2} for the
// scene-two cases. Milestone and undeploy steps emit no records.
func assertInfraNWTableLateIndex560Trace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if trace.ID != infraNWTableLateIndex560ID || trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace identity = %q/%q", trace.ID, trace.Version)
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	for _, caseName := range infraNWTableLateIndex560Cases {
		records := byCase[caseName]
		if len(records) != 4 {
			t.Fatalf("case %q records = %d, want 4", caseName, len(records))
		}
		var deployed []string
		var snapshots map[string][]compat.ResultRecord = map[string][]compat.ResultRecord{}
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
			default:
				t.Fatalf("case %q unexpected operation %q", caseName, record.Operation)
			}
		}
		wantDeployed := []string{"create", "insert", "index"}
		if !reflect.DeepEqual(deployed, wantDeployed) {
			t.Fatalf("case %q deployed order = %v", caseName, deployed)
		}
		rows := snapshots["select"]
		if strings.HasPrefix(caseName, "scene-two") {
			if len(rows) != 3 {
				t.Fatalf("case %q select row count = %d, want 3", caseName, len(rows))
			}
			for index, want := range []int64{-4, -3, -2} {
				row := rows[index]
				if row.Kind != "row" || len(row.Fields) != 4 ||
					row.Fields["f1"] != "E1" || row.Fields["f3"] != ">E1<" || row.Fields["f4"] != "?E1?" {
					t.Fatalf("case %q select row %d = %#v", caseName, index, row)
				}
				if !infraNWTableLateIndex560IntEquals(row.Fields["f2"], want) {
					t.Fatalf("case %q select row %d f2 = %#v, want %d", caseName, index, row.Fields["f2"], want)
				}
			}
		} else {
			if len(rows) != 2 {
				t.Fatalf("case %q select row count = %d, want 2", caseName, len(rows))
			}
			for index, want := range []int64{1, 2} {
				row := rows[index]
				if row.Kind != "row" || len(row.Fields) != 2 || row.Fields["theString"] != "B2" {
					t.Fatalf("case %q select row %d = %#v", caseName, index, row)
				}
				if !infraNWTableLateIndex560IntEquals(row.Fields["intPrimitive"], want) {
					t.Fatalf("case %q select row %d intPrimitive = %#v, want %d",
						caseName, index, row.Fields["intPrimitive"], want)
				}
			}
		}
	}
}

// infraNWTableLateIndex560IntEquals compares a decoded trace field value
// against the pinned integral value across the number representations the
// JSON loader yields (json.Number, float64, int64).
func infraNWTableLateIndex560IntEquals(value any, want int64) bool {
	switch typed := value.(type) {
	case json.Number:
		return typed == json.Number(strconv.FormatInt(want, 10))
	case float64:
		return typed == float64(want)
	case int64:
		return typed == want
	case int:
		return int64(typed) == want
	}
	return false
}

func TestRunInfraNWTableLateIndex560DirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableLateIndex560ID,
		"-scenario", filepath.Join(root, infraNWTableLateIndex560ID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertInfraNWTableLateIndex560Trace(t, trace)
}

// TestRunInfraNWTableLateIndex560PinnedArtifacts pins the byte-exact EPL
// constants transcribed from InfraNWTableCreateIndex.java lines 342-364
// (InfraLateCreate) and 389-410 (InfraLateCreateSceneTwo) plus the per-case
// step sequences, so a transcription slip fails the unit even when the
// checked-in scenario is regenerated.
func TestRunInfraNWTableLateIndex560PinnedArtifacts(t *testing.T) {
	wants := map[string]string{
		"lc-create-window":  "@Name('Create') @public create window MyInfra.win:keepall() as SupportBean",
		"lc-create-table":   "@Name('Create') @public create table MyInfra(theString string primary key, intPrimitive int primary key)",
		"lc-insert":         "@Name('Insert') insert into MyInfra select theString, intPrimitive from SupportBean",
		"lc-index":          "@Name('Index') create index MyInfra_IDX on MyInfra(theString)",
		"lc-select":         "select * from MyInfra where theString = 'B2' order by intPrimitive asc",
		"two-create-window": "@public create window MyInfraLC#keepall as (f1 string, f2 int, f3 string, f4 string)",
		"two-create-table":  "@public create table MyInfraLC as (f1 string primary key, f2 int primary key, f3 string primary key, f4 string primary key)",
		"two-insert":        "insert into MyInfraLC(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean",
		"two-index":         "create index MyInfraLCIndex on MyInfraLC(f2, f3, f1)",
		"two-select":        "select * from MyInfraLC where f3='>E1<' order by f2 asc",
	}
	got := map[string]string{
		"lc-create-window":  infraNWTableLC560CreateWindow,
		"lc-create-table":   infraNWTableLC560CreateTable,
		"lc-insert":         infraNWTableLC560Insert,
		"lc-index":          infraNWTableLC560Index,
		"lc-select":         infraNWTableLC560Select,
		"two-create-window": infraNWTableLC560TwoCreateWindow,
		"two-create-table":  infraNWTableLC560TwoCreateTable,
		"two-insert":        infraNWTableLC560TwoInsert,
		"two-index":         infraNWTableLC560TwoIndex,
		"two-select":        infraNWTableLC560TwoSelect,
	}
	for name, want := range wants {
		if got[name] != want {
			t.Fatalf("EPL %s = %q, want %q", name, got[name], want)
		}
	}
	for _, caseName := range infraNWTableLateIndex560Cases {
		if len(infraNWTableLateIndex560CaseSteps[caseName]) != 11 {
			t.Fatalf("case %q pinned steps = %d, want 11",
				caseName, len(infraNWTableLateIndex560CaseSteps[caseName]))
		}
	}
}

// TestRunInfraNWTableLateIndex560ScenarioLoader pins the strict loader: the
// checked-in scenario parses, and runtime-id drift plus step-count drift
// are both rejected.
func TestRunInfraNWTableLateIndex560ScenarioLoader(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableLateIndex560ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	_, loadErr := loadInfraNWTableLateIndex560Scenario(scenarioFile)
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
	if _, err := loadInfraNWTableLateIndex560Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("runtime-id drift accepted")
	}
	mutated["javaRuntimes"] = runtimes
	steps := mutated["steps"].([]any)
	mutated["steps"] = steps[:len(steps)-1]
	raw, err = json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableLateIndex560Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("step-count drift accepted")
	}
}

func TestRunInfraNWTableLateIndex560DiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := infraNWTableLateIndex560JavaTraceFixture(t, func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), infraNWTableLateIndex560ID+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableLateIndex560ID + "-diff",
		"-scenario", filepath.Join(root, infraNWTableLateIndex560ID+".json"),
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
	if evidence.JavaCommit != infraNWTableLateIndex560JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableLateIndex560JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableLateIndex560JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableLateIndex560JavaExecutions) {
		t.Fatalf("Java metadata = %#v", evidence)
	}
	assertInfraNWTableLateIndex560Trace(t, evidence.JavaTrace)
	assertInfraNWTableLateIndex560Trace(t, evidence.GoTrace)
}

func TestRunInfraNWTableLateIndex560DiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// The late-create probe pins order by intPrimitive asc; an
			// out-of-order Java trace must fail positionally.
			name: "late-order-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "late-window" && rec.Statement == "select" && len(rec.New) == 2 {
						rec.New[0], rec.New[1] = rec.New[1], rec.New[0]
						return
					}
				}
				panic("no late-window select snapshot record")
			},
		},
		{
			// The scene-two probe pins f3='>E1<'; a drifted concat value
			// means the insert-into feed lost the '>'||'<' decoration.
			name: "concat-value-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "scene-two-table" && rec.Statement == "select" && len(rec.New) > 0 {
						rec.New[0].Fields["f3"] = "E1"
						return
					}
				}
				panic("no scene-two-table select snapshot record")
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
			javaTracePath := infraNWTableLateIndex560JavaTraceFixture(t, test.mutate)
			evidencePath := filepath.Join(t.TempDir(), infraNWTableLateIndex560ID+".evidence.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", infraNWTableLateIndex560ID + "-diff",
				"-scenario", filepath.Join(root, infraNWTableLateIndex560ID+".json"),
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

// TestRunInfraNWTableLateIndex560CheckedInEvidenceMatchesTraceAndReplay
// verifies the landed Java trace, Go trace and evidence against the
// runner's pinned constants; the test is inert until the unit's
// differential artifacts are checked in next to the scenario.
func TestRunInfraNWTableLateIndex560CheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := filepath.Join(root, infraNWTableLateIndex560ID+".trace.json")
	goTracePath := filepath.Join(root, infraNWTableLateIndex560ID+".go.trace.json")
	evidencePath := filepath.Join(root, infraNWTableLateIndex560ID+".evidence.json")
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
	if evidence.JavaCommit != infraNWTableLateIndex560JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableLateIndex560JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableLateIndex560JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableLateIndex560JavaExecutions) {
		t.Fatalf("checked-in Java metadata = %#v", evidence)
	}
	assertInfraNWTableLateIndex560Trace(t, javaTrace)
	assertInfraNWTableLateIndex560Trace(t, goTrace)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableLateIndex560ID,
		"-scenario", filepath.Join(root, infraNWTableLateIndex560ID+".json"),
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
	assertInfraNWTableLateIndex560Trace(t, replayed)
}

func TestRunInfraNWTableLateIndex560RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableLateIndex560ID+".json"))
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
			return bytes.Replace(data, []byte(`{"op": "deploy", "case": "late-window", "statement": "create"`),
				[]byte(`{"op": "deploy", "case": "late-window", "statement": "create", "extra": 0`), 1)
		}},
		{name: "case-runtime-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"runtimeId": "java-runtime-ae0b11741e8c17c55271"`),
				[]byte(`"runtimeId": "java-runtime-wrong"`), 1)
		}},
		{name: "epl-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`select * from MyInfraLC where f3='>E1<' order by f2 asc`),
				[]byte(`select * from MyInfraLC where f3='>E2<' order by f2 asc`), 1)
		}},
		{name: "payload-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"intPrimitive": -2`),
				[]byte(`"intPrimitive": -5`), 1)
		}},
		{name: "snapshot-mode-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"statement": "select", "mode": "ordered"`),
				[]byte(`"statement": "select", "mode": "any"`), 1)
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
				"-mode", infraNWTableLateIndex560ID,
				"-scenario", scenarioPath,
			}, &stdout, &stderr); code == 0 {
				t.Fatalf("malformed scenario %q unexpectedly replayed: stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunInfraNWTableLateIndex560RuntimeIDMappingMatchesScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableLateIndex560ID+".json"))
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
	if !reflect.DeepEqual(raw.JavaRuntimes, infraNWTableLateIndex560JavaRuntimeIDs) ||
		!reflect.DeepEqual(raw.JavaNames, infraNWTableLateIndex560JavaExecutions) ||
		!reflect.DeepEqual(raw.JavaStaticIDs, infraNWTableLateIndex560JavaStaticIDs) ||
		!reflect.DeepEqual(raw.JavaFlags, infraNWTableLateIndex560JavaFlags) {
		t.Fatalf("scenario Java identity = %#v", raw)
	}
	for index, definition := range raw.Cases {
		if definition.Case != infraNWTableLateIndex560Cases[index] ||
			definition.Ordinal != infraNWTableLateIndex560Ordinals[index] ||
			definition.RuntimeID != infraNWTableLateIndex560JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableLateIndex560JavaExecutions[index] {
			t.Fatalf("case %d mapping = %#v", index, definition)
		}
	}
}
