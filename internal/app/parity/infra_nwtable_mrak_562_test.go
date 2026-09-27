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

// infraNWTableMRAK562JavaTraceFixture writes the Java-side trace the diff
// assertions run against. When the unit's Java trace has been landed next to
// the scenario the checked-in file wins; before that the fixture is the same
// 20 records assembled from the runner's pinned constants (record shapes
// verified 0-diff against the oracle run for
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c).
func infraNWTableMRAK562JavaTraceFixture(t *testing.T, mutate func(*compat.Trace)) string {
	t.Helper()
	checkedIn := filepath.Join("..", "..", "..", "testdata", "parity",
		infraNWTableMRAK562ID+".trace.json")
	if _, err := os.Stat(checkedIn); err == nil {
		return writeJavaTraceFixtureFromTrace(t, checkedIn, mutate)
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: infraNWTableMRAK562ID}
	epoch := "1970-01-01T00:00:00Z"
	deploy := func(caseName, statement string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "deployed", Statement: statement,
			Sequence: 1, Time: epoch,
		})
	}
	row := func(id string) compat.ResultRecord {
		return compat.ResultRecord{Kind: "row", Fields: map[string]any{"id": id}}
	}
	rows := func(ids ...string) []compat.ResultRecord {
		result := make([]compat.ResultRecord, len(ids))
		for index, id := range ids {
			result[index] = row(id)
		}
		return result
	}
	snapshot := func(caseName, statement string, ids ...string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "snapshot", Statement: statement,
			Time: epoch, New: rows(ids...),
		})
	}
	for index, caseName := range infraNWTableMRAK562Cases {
		deploy(caseName, "create")
		deploy(caseName, "insert")
		deploy(caseName, "index")
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "unrepresentable",
			Statement: "index-kind", Value: infraNWTableMRAK562IndexKindNote,
		})
		snapshot(caseName, "select-q1-empty")
		snapshot(caseName, "select-q1-e1", "E1")
		snapshot(caseName, "select-q1-e2", "E1", "E2")
		snapshot(caseName, "select-q1-e3", "E1", "E2", "E3")
		snapshot(caseName, "select-q2", "E1", "E2", "E3")
		count := int64(index + 1)
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "index-count", Statement: "MyInfraMRAK",
			Sequence: 1, Time: epoch, Count: &count,
		})
	}
	if len(trace.Records) != 20 {
		t.Fatalf("fixture records = %d, want 20", len(trace.Records))
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

// assertInfraNWTableMRAK562Trace pins the 20-record mrak trace: per case
// three deployed markers (sequence 1, in order create, insert, index), the
// index-kind plan-only record, the five ordered FAF snapshots returning {},
// {E1}, {E1,E2}, {E1,E2,E3} and {E1,E2,E3} through the id projection, and
// the index-count tail (1 for the window, 2 for the table). Send, milestone
// and undeploy steps emit no records.
func assertInfraNWTableMRAK562Trace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if trace.ID != infraNWTableMRAK562ID || trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace identity = %q/%q", trace.ID, trace.Version)
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	wantSnapshots := map[string][]string{
		"select-q1-empty": {},
		"select-q1-e1":    {"E1"},
		"select-q1-e2":    {"E1", "E2"},
		"select-q1-e3":    {"E1", "E2", "E3"},
		"select-q2":       {"E1", "E2", "E3"},
	}
	for index, caseName := range infraNWTableMRAK562Cases {
		records := byCase[caseName]
		if len(records) != 10 {
			t.Fatalf("case %q records = %d, want 10", caseName, len(records))
		}
		var deployed []string
		snapshots := map[string][]string{}
		var probes []string
		var counts []int64
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
				ids := make([]string, 0, len(record.New))
				for _, row := range record.New {
					if row.Kind != "row" || len(row.Fields) != 1 {
						t.Fatalf("case %q %s row = %#v", caseName, record.Statement, row)
					}
					id, ok := row.Fields["id"].(string)
					if !ok {
						t.Fatalf("case %q %s id = %#v", caseName, record.Statement, row.Fields["id"])
					}
					ids = append(ids, id)
				}
				snapshots[record.Statement] = ids
			case "unrepresentable":
				if record.Sequence != 0 || record.Time != "" {
					t.Fatalf("case %q unrepresentable record %#v", caseName, record)
				}
				if record.Value != infraNWTableMRAK562IndexKindNote {
					t.Fatalf("case %q unrepresentable value = %#v", caseName, record.Value)
				}
				probes = append(probes, record.Statement)
			case "index-count":
				if record.Statement != "MyInfraMRAK" || record.Sequence != 1 ||
					record.Time != "1970-01-01T00:00:00Z" || record.Count == nil {
					t.Fatalf("case %q index-count record %#v", caseName, record)
				}
				counts = append(counts, *record.Count)
			default:
				t.Fatalf("case %q unexpected operation %q", caseName, record.Operation)
			}
		}
		wantDeployed := []string{"create", "insert", "index"}
		if !reflect.DeepEqual(deployed, wantDeployed) {
			t.Fatalf("case %q deployed order = %v", caseName, deployed)
		}
		for _, statement := range []string{"select-q1-empty", "select-q1-e1", "select-q1-e2", "select-q1-e3", "select-q2"} {
			if !reflect.DeepEqual(snapshots[statement], wantSnapshots[statement]) {
				t.Fatalf("case %q %s ids = %v, want %v", caseName, statement,
					snapshots[statement], wantSnapshots[statement])
			}
		}
		if !reflect.DeepEqual(probes, []string{"index-kind"}) {
			t.Fatalf("case %q unrepresentable probes = %v", caseName, probes)
		}
		wantCount := int64(index + 1)
		if !reflect.DeepEqual(counts, []int64{wantCount}) {
			t.Fatalf("case %q index counts = %v, want %d", caseName, counts, wantCount)
		}
	}
}

func TestRunInfraNWTableMRAK562DirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableMRAK562ID,
		"-scenario", filepath.Join(root, infraNWTableMRAK562ID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertInfraNWTableMRAK562Trace(t, trace)
}

// TestRunInfraNWTableMRAK562PinnedArtifacts pins the byte-exact EPL
// constants transcribed from InfraNWTableCreateIndex.java lines 586-620
// (InfraMultiRangeAndKey) plus the per-case step sequence length, so a
// transcription slip fails the unit even when the checked-in scenario is
// regenerated.
func TestRunInfraNWTableMRAK562PinnedArtifacts(t *testing.T) {
	wants := map[string]string{
		"create-window": "@name('create') @public create window MyInfraMRAK#keepall as SupportBeanRange",
		"create-table":  "@name('create') @public create table MyInfraMRAK(id string primary key, key string, keyLong long, rangeStartLong long primary key, rangeEndLong long primary key)",
		"insert-window": "insert into MyInfraMRAK select * from SupportBeanRange",
		"insert-table":  "on SupportBeanRange t0 merge MyInfraMRAK t1 where t0.id = t1.id when not matched then insert select id, key, keyLong, rangeStartLong, rangeEndLong",
		"index":         "create index idx1 on MyInfraMRAK(key hash, keyLong hash, rangeStartLong btree, rangeEndLong btree)",
		"query1":        "select * from MyInfraMRAK where rangeStartLong > 1 and rangeEndLong > 2 and keyLong=1 and key='K1' order by id asc",
		"query2":        "select * from MyInfraMRAK where rangeStartLong > 1 and rangeEndLong > 2 and keyLong=1 order by id asc",
	}
	got := map[string]string{
		"create-window": infraNWTableMRAK562CreateWindow,
		"create-table":  infraNWTableMRAK562CreateTable,
		"insert-window": infraNWTableMRAK562InsertWindow,
		"insert-table":  infraNWTableMRAK562InsertTable,
		"index":         infraNWTableMRAK562Index,
		"query1":        infraNWTableMRAK562Query1,
		"query2":        infraNWTableMRAK562Query2,
	}
	for name, want := range wants {
		if got[name] != want {
			t.Fatalf("EPL %s = %q, want %q", name, got[name], want)
		}
	}
	for _, caseName := range infraNWTableMRAK562Cases {
		if len(infraNWTableMRAK562CaseSteps[caseName]) != 17 {
			t.Fatalf("case %q pinned step sequence = %d, want 17",
				caseName, len(infraNWTableMRAK562CaseSteps[caseName]))
		}
	}
}

// TestRunInfraNWTableMRAK562ScenarioLoader pins the strict loader: the
// checked-in scenario parses, and runtime-id drift plus step-count drift
// are both rejected.
func TestRunInfraNWTableMRAK562ScenarioLoader(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableMRAK562ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	_, loadErr := loadInfraNWTableMRAK562Scenario(scenarioFile)
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
	mutated["javaRuntimes"] = []any{"java-runtime-mutated", runtimes[1]}
	raw, err := json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableMRAK562Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("runtime-id drift accepted")
	}
	mutated["javaRuntimes"] = runtimes
	steps := mutated["steps"].([]any)
	mutated["steps"] = steps[:len(steps)-1]
	raw, err = json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableMRAK562Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("step-count drift accepted")
	}
}

func TestRunInfraNWTableMRAK562DiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := infraNWTableMRAK562JavaTraceFixture(t, func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), infraNWTableMRAK562ID+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableMRAK562ID + "-diff",
		"-scenario", filepath.Join(root, infraNWTableMRAK562ID+".json"),
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
	if evidence.JavaCommit != infraNWTableMRAK562JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableMRAK562JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableMRAK562JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableMRAK562JavaExecutions) {
		t.Fatalf("Java metadata = %#v", evidence)
	}
	assertInfraNWTableMRAK562Trace(t, evidence.JavaTrace)
	assertInfraNWTableMRAK562Trace(t, evidence.GoTrace)
}

func TestRunInfraNWTableMRAK562DiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// The q1 probes pin the id-only projection; a drifted id means
			// the merge feed wrote a different row.
			name: "row-id-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "mrak-window" && rec.Statement == "select-q1-e2" && len(rec.New) > 1 {
						rec.New[1].Fields["id"] = "E9"
						return
					}
				}
				panic("no select-q1-e2 snapshot record")
			},
		},
		{
			// The key-less probe pins the same three rows; dropping a row
			// means the post-filter or ordering regressed.
			name: "q2-row-count",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "mrak-table" && rec.Statement == "select-q2" && len(rec.New) == 3 {
						rec.New = rec.New[:2]
						return
					}
				}
				panic("no select-q2 snapshot record")
			},
		},
		{
			// The index-kind record must stay the plan-only unrepresentable
			// note; retyping it to a snapshot silently downgrades the
			// divergence marker.
			name: "index-kind-retyped",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					if trace.Records[index].Statement == "index-kind" {
						trace.Records[index].Operation = "snapshot"
						return
					}
				}
				panic("no index-kind record")
			},
		},
		{
			// The table tail pins getIndexCount=2 (idx1 plus the implicit
			// primary-key descriptor); a drifted count means the composite
			// primary key was lost.
			name: "table-count-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "mrak-table" && rec.Operation == "index-count" {
						drifted := int64(1)
						rec.Count = &drifted
						return
					}
				}
				panic("no index-count record")
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
			javaTracePath := infraNWTableMRAK562JavaTraceFixture(t, test.mutate)
			evidencePath := filepath.Join(t.TempDir(), infraNWTableMRAK562ID+".evidence.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", infraNWTableMRAK562ID + "-diff",
				"-scenario", filepath.Join(root, infraNWTableMRAK562ID+".json"),
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

// TestRunInfraNWTableMRAK562CheckedInEvidenceMatchesTraceAndReplay verifies
// the landed Java trace, Go trace and evidence against the runner's pinned
// constants; the test is inert until the unit's differential artifacts are
// checked in next to the scenario.
func TestRunInfraNWTableMRAK562CheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := filepath.Join(root, infraNWTableMRAK562ID+".trace.json")
	goTracePath := filepath.Join(root, infraNWTableMRAK562ID+".go.trace.json")
	evidencePath := filepath.Join(root, infraNWTableMRAK562ID+".evidence.json")
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
	if evidence.JavaCommit != infraNWTableMRAK562JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableMRAK562JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableMRAK562JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableMRAK562JavaExecutions) {
		t.Fatalf("checked-in Java metadata = %#v", evidence)
	}
	assertInfraNWTableMRAK562Trace(t, javaTrace)
	assertInfraNWTableMRAK562Trace(t, goTrace)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableMRAK562ID,
		"-scenario", filepath.Join(root, infraNWTableMRAK562ID+".json"),
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
	assertInfraNWTableMRAK562Trace(t, replayed)
}

func TestRunInfraNWTableMRAK562RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableMRAK562ID+".json"))
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
			return bytes.Replace(data, []byte(`{"op": "deploy", "case": "mrak-window", "statement": "create"`),
				[]byte(`{"op": "deploy", "case": "mrak-window", "statement": "create", "extra": 0`), 1)
		}},
		{name: "case-runtime-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"runtimeId": "java-runtime-7242c335a588f1d6fa15"`),
				[]byte(`"runtimeId": "java-runtime-wrong"`), 1)
		}},
		{name: "epl-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`keyLong=1 and key='K1' order by id asc`),
				[]byte(`keyLong=1 and key='K2' order by id asc`), 1)
		}},
		{name: "payload-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"rangeEndLong": 3`),
				[]byte(`"rangeEndLong": 4`), 1)
		}},
		{name: "index-kind-note-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`the kind split is plan-only`),
				[]byte(`the kind split is replayed`), 1)
		}},
		{name: "index-count-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"of": "indexes", "count": 2`),
				[]byte(`"of": "indexes", "count": 1`), 1)
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
				"-mode", infraNWTableMRAK562ID,
				"-scenario", scenarioPath,
			}, &stdout, &stderr); code == 0 {
				t.Fatalf("malformed scenario %q unexpectedly replayed: stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunInfraNWTableMRAK562RuntimeIDMappingMatchesScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableMRAK562ID+".json"))
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
	if !reflect.DeepEqual(raw.JavaRuntimes, infraNWTableMRAK562JavaRuntimeIDs) ||
		!reflect.DeepEqual(raw.JavaNames, infraNWTableMRAK562JavaExecutions) ||
		!reflect.DeepEqual(raw.JavaStaticIDs, infraNWTableMRAK562JavaStaticIDs) ||
		!reflect.DeepEqual(raw.JavaFlags, infraNWTableMRAK562JavaFlags) {
		t.Fatalf("scenario Java identity = %#v", raw)
	}
	for index, definition := range raw.Cases {
		if definition.Case != infraNWTableMRAK562Cases[index] ||
			definition.Ordinal != infraNWTableMRAK562Ordinals[index] ||
			definition.RuntimeID != infraNWTableMRAK562JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableMRAK562JavaExecutions[index] {
			t.Fatalf("case %d mapping = %#v", index, definition)
		}
	}
}
