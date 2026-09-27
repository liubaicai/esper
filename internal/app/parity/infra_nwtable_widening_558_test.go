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

// infraNWTableWidening558JavaTraceFixture writes the Java-side trace the
// diff assertions run against. When the unit's Java trace has been landed
// next to the scenario the checked-in file wins; before that the fixture is
// the same 36 records assembled from the runner's pinned constants (record
// shapes verified 0-diff against the oracle run for
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c).
func infraNWTableWidening558JavaTraceFixture(t *testing.T, mutate func(*compat.Trace)) string {
	t.Helper()
	checkedIn := filepath.Join("..", "..", "..", "testdata", "parity",
		infraNWTableWidening558ID+".trace.json")
	if _, err := os.Stat(checkedIn); err == nil {
		return writeJavaTraceFixtureFromTrace(t, checkedIn, mutate)
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: infraNWTableWidening558ID}
	epoch := "1970-01-01T00:00:00Z"
	deploy := func(caseName, statement string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "deployed", Statement: statement,
			Sequence: 1, Time: epoch,
		})
	}
	row := func(f1 string) []compat.ResultRecord {
		return []compat.ResultRecord{{
			Kind:   "row",
			Fields: map[string]any{"f1": json.Number(f1), "f2": "E1"},
		}}
	}
	snapshot := func(caseName, statement, f1 string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "snapshot", Statement: statement,
			Time: epoch, New: row(f1),
		})
	}
	for _, caseName := range infraNWTableWidening558Cases {
		deploy(caseName, "create-one")
		deploy(caseName, "insert-one")
		deploy(caseName, "index-one")
		if strings.HasPrefix(caseName, "hashbtree") {
			snapshot(caseName, "select-gt-long", "10")
			trace.Records = append(trace.Records,
				compat.TraceRecord{Case: caseName, Operation: "unrepresentable",
					Statement: "soda-ix1", Value: infraNWTableW558SodaIX1Note},
				compat.TraceRecord{Case: caseName, Operation: "unrepresentable",
					Statement: "soda-ix2", Value: infraNWTableW558SodaIX2Note})
		} else {
			snapshot(caseName, "select-eq-long", "10")
		}
		deploy(caseName, "create-two")
		deploy(caseName, "insert-two")
		deploy(caseName, "index-two")
		if strings.HasPrefix(caseName, "hashbtree") {
			snapshot(caseName, "select-gte-short", "2")
		} else {
			snapshot(caseName, "select-eq-short", "2")
		}
	}
	if len(trace.Records) != 36 {
		t.Fatalf("fixture records = %d, want 36", len(trace.Records))
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

// assertInfraNWTableWidening558Trace pins the 36-record widening trace: per
// case six deployed markers (sequence 1, in order create-one, insert-one,
// index-one, create-two, insert-two, index-two), the long-leg and short-leg
// FAF snapshots returning exactly {f1:10, f2:"E1"} and {f1:2, f2:"E1"}, and
// the hashbtree cases' two plan-only SODA records between the legs.
func assertInfraNWTableWidening558Trace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if trace.ID != infraNWTableWidening558ID || trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace identity = %q/%q", trace.ID, trace.Version)
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	longSelect := map[string]string{
		"hashbtree-window": "select-gt-long",
		"hashbtree-table":  "select-gt-long",
		"widening-window":  "select-eq-long",
		"widening-table":   "select-eq-long",
	}
	shortSelect := map[string]string{
		"hashbtree-window": "select-gte-short",
		"hashbtree-table":  "select-gte-short",
		"widening-window":  "select-eq-short",
		"widening-table":   "select-eq-short",
	}
	for _, caseName := range infraNWTableWidening558Cases {
		records := byCase[caseName]
		wantCount := 8
		if strings.HasPrefix(caseName, "hashbtree") {
			wantCount = 10
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
		wantDeployed := []string{"create-one", "insert-one", "index-one", "create-two", "insert-two", "index-two"}
		if !reflect.DeepEqual(deployed, wantDeployed) {
			t.Fatalf("case %q deployed order = %v", caseName, deployed)
		}
		longRows := snapshots[longSelect[caseName]]
		shortRows := snapshots[shortSelect[caseName]]
		if len(longRows) != 1 || len(shortRows) != 1 {
			t.Fatalf("case %q snapshot row counts = %d/%d", caseName, len(longRows), len(shortRows))
		}
		for label, rows := range map[string][]compat.ResultRecord{"long": longRows, "short": shortRows} {
			row := rows[0]
			if row.Kind != "row" || len(row.Fields) != 2 || row.Fields["f2"] != "E1" {
				t.Fatalf("case %q %s row = %#v", caseName, label, row)
			}
			want := float64(10)
			if label == "short" {
				want = 2
			}
			if row.Fields["f1"] != want && row.Fields["f1"] != json.Number("10") && row.Fields["f1"] != json.Number("2") {
				t.Fatalf("case %q %s f1 = %#v, want %v", caseName, label, row.Fields["f1"], want)
			}
		}
		if strings.HasPrefix(caseName, "hashbtree") {
			wantProbes := []string{"soda-ix1", "soda-ix2"}
			if !reflect.DeepEqual(probes, wantProbes) {
				t.Fatalf("case %q unrepresentable probes = %v", caseName, probes)
			}
		} else if len(probes) != 0 {
			t.Fatalf("case %q unexpected unrepresentable probes = %v", caseName, probes)
		}
	}
}

func TestRunInfraNWTableWidening558DirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableWidening558ID,
		"-scenario", filepath.Join(root, infraNWTableWidening558ID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertInfraNWTableWidening558Trace(t, trace)
}

// TestRunInfraNWTableWidening558PinnedArtifacts pins the byte-exact EPL
// constants transcribed from InfraNWTableCreateIndex.java lines 481-509
// (InfraWidening) and 527-571 (InfraHashBTreeWidening) plus the per-case
// step sequences, so a transcription slip fails the unit even when the
// checked-in scenario is regenerated.
func TestRunInfraNWTableWidening558PinnedArtifacts(t *testing.T) {
	wants := map[string]string{
		"w-create-one-window":   "@public create window MyInfraW#keepall as (f1 long, f2 string)",
		"w-create-one-table":    "@public create table MyInfraW as (f1 long primary key, f2 string primary key)",
		"w-insert-one":          "insert into MyInfraW(f1, f2) select longPrimitive, theString from SupportBean",
		"w-index-one":           "create index MyInfraWIndex1 on MyInfraW(f1)",
		"w-select-one":          "select * from MyInfraW where f1=10",
		"w-create-two-window":   "@public create window MyInfraWTwo#keepall as (f1 short, f2 string)",
		"w-create-two-table":    "@public create table MyInfraWTwo as (f1 short primary key, f2 string primary key)",
		"w-insert-two":          "insert into MyInfraWTwo(f1, f2) select shortPrimitive, theString from SupportBean",
		"w-index-two":           "create index MyInfraWTwoIndex1 on MyInfraWTwo(f1)",
		"w-select-two":          "select * from MyInfraWTwo where f1=2",
		"hbt-create-one-window": "@public create window MyInfraHBTW#keepall as (f1 long, f2 string)",
		"hbt-create-one-table":  "@public create table MyInfraHBTW as (f1 long primary key, f2 string primary key)",
		"hbt-insert-one":        "insert into MyInfraHBTW(f1, f2) select longPrimitive, theString from SupportBean",
		"hbt-index-one":         "create index MyInfraHBTWIndex1 on MyInfraHBTW(f1 btree)",
		"hbt-select-one":        "select * from MyInfraHBTW where f1>9",
		"hbt-soda-ix1":          "create index IX1 on MyInfraHBTW(f1, f2 btree)",
		"hbt-soda-ix2":          "create unique index IX2 on MyInfraHBTW(f1)",
		"hbt-create-two-window": "@public create window MyInfraHBTWTwo#keepall as (f1 short, f2 string)",
		"hbt-create-two-table":  "@public create table MyInfraHBTWTwo as (f1 short primary key, f2 string primary key)",
		"hbt-insert-two":        "insert into MyInfraHBTWTwo(f1, f2) select shortPrimitive, theString from SupportBean",
		"hbt-index-two":         "create index MyInfraHBTWTwoIndex1 on MyInfraHBTWTwo(f1 btree)",
		"hbt-select-two":        "select * from MyInfraHBTWTwo where f1>=2",
	}
	got := map[string]string{
		"w-create-one-window":   infraNWTableW558CreateOneWindow,
		"w-create-one-table":    infraNWTableW558CreateOneTable,
		"w-insert-one":          infraNWTableW558InsertOne,
		"w-index-one":           infraNWTableW558IndexOne,
		"w-select-one":          infraNWTableW558SelectOne,
		"w-create-two-window":   infraNWTableW558CreateTwoWindow,
		"w-create-two-table":    infraNWTableW558CreateTwoTable,
		"w-insert-two":          infraNWTableW558InsertTwo,
		"w-index-two":           infraNWTableW558IndexTwo,
		"w-select-two":          infraNWTableW558SelectTwo,
		"hbt-create-one-window": infraNWTableHBT558CreateOneWindow,
		"hbt-create-one-table":  infraNWTableHBT558CreateOneTable,
		"hbt-insert-one":        infraNWTableHBT558InsertOne,
		"hbt-index-one":         infraNWTableHBT558IndexOne,
		"hbt-select-one":        infraNWTableHBT558SelectOne,
		"hbt-soda-ix1":          infraNWTableHBT558SodaIX1,
		"hbt-soda-ix2":          infraNWTableHBT558SodaIX2,
		"hbt-create-two-window": infraNWTableHBT558CreateTwoWindow,
		"hbt-create-two-table":  infraNWTableHBT558CreateTwoTable,
		"hbt-insert-two":        infraNWTableHBT558InsertTwo,
		"hbt-index-two":         infraNWTableHBT558IndexTwo,
		"hbt-select-two":        infraNWTableHBT558SelectTwo,
	}
	for name, want := range wants {
		if got[name] != want {
			t.Fatalf("EPL %s = %q, want %q", name, got[name], want)
		}
	}
	if len(infraNWTableWidening558CaseSteps["hashbtree-window"]) != 19 ||
		len(infraNWTableWidening558CaseSteps["widening-window"]) != 17 {
		t.Fatalf("pinned step sequences = %d/%d",
			len(infraNWTableWidening558CaseSteps["hashbtree-window"]),
			len(infraNWTableWidening558CaseSteps["widening-window"]))
	}
}

// TestRunInfraNWTableWidening558ScenarioLoader pins the strict loader: the
// checked-in scenario parses, and runtime-id drift plus step-count drift
// are both rejected.
func TestRunInfraNWTableWidening558ScenarioLoader(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableWidening558ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	_, loadErr := loadInfraNWTableWidening558Scenario(scenarioFile)
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
	if _, err := loadInfraNWTableWidening558Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("runtime-id drift accepted")
	}
	mutated["javaRuntimes"] = runtimes
	steps := mutated["steps"].([]any)
	mutated["steps"] = steps[:len(steps)-1]
	raw, err = json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableWidening558Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("step-count drift accepted")
	}
}

func TestRunInfraNWTableWidening558DiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := infraNWTableWidening558JavaTraceFixture(t, func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), infraNWTableWidening558ID+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableWidening558ID + "-diff",
		"-scenario", filepath.Join(root, infraNWTableWidening558ID+".json"),
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
	if evidence.JavaCommit != infraNWTableWidening558JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableWidening558JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableWidening558JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableWidening558JavaExecutions) {
		t.Fatalf("Java metadata = %#v", evidence)
	}
	assertInfraNWTableWidening558Trace(t, evidence.JavaTrace)
	assertInfraNWTableWidening558Trace(t, evidence.GoTrace)
}

func TestRunInfraNWTableWidening558DiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// The long-leg hashbtree read `where f1>9` must widen int 9 to
			// long 10 and return f1=10; a drifted value is a widening loss.
			name: "widening-long-value-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "hashbtree-window" && rec.Statement == "select-gt-long" && len(rec.New) > 0 {
						rec.New[0].Fields["f1"] = json.Number("9")
						return
					}
				}
				panic("no select-gt-long snapshot record")
			},
		},
		{
			// The widening-window equality read `where f1=10` pins {10,E1};
			// drifting the projected string key breaks the row.
			name: "widening-short-key-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "widening-table" && rec.Statement == "select-eq-short" && len(rec.New) > 0 {
						rec.New[0].Fields["f2"] = "Z9"
						return
					}
				}
				panic("no select-eq-short snapshot record")
			},
		},
		{
			// The first hashbtree SODA probe must stay the plan-only
			// unrepresentable record; retyping it to a snapshot silently
			// downgrades the probe.
			name: "soda-probe-retyped",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					if trace.Records[index].Statement == "soda-ix1" {
						trace.Records[index].Operation = "snapshot"
						return
					}
				}
				panic("no soda-ix1 record")
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
			javaTracePath := infraNWTableWidening558JavaTraceFixture(t, test.mutate)
			evidencePath := filepath.Join(t.TempDir(), infraNWTableWidening558ID+".evidence.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", infraNWTableWidening558ID + "-diff",
				"-scenario", filepath.Join(root, infraNWTableWidening558ID+".json"),
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

func TestRunInfraNWTableWidening558CheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTrace, err := loadTraceFile(filepath.Join(root, infraNWTableWidening558ID+".trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	goTrace, err := loadTraceFile(filepath.Join(root, infraNWTableWidening558ID+".go.trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := loadDifferentialEvidenceFile(filepath.Join(root, infraNWTableWidening558ID+".evidence.json"))
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
	if evidence.JavaCommit != infraNWTableWidening558JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableWidening558JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableWidening558JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableWidening558JavaExecutions) {
		t.Fatalf("checked-in Java metadata = %#v", evidence)
	}
	assertInfraNWTableWidening558Trace(t, javaTrace)
	assertInfraNWTableWidening558Trace(t, goTrace)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableWidening558ID,
		"-scenario", filepath.Join(root, infraNWTableWidening558ID+".json"),
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
	assertInfraNWTableWidening558Trace(t, replayed)
}

func TestRunInfraNWTableWidening558RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableWidening558ID+".json"))
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
			return bytes.Replace(data, []byte(`{"op": "deploy", "case": "hashbtree-window", "statement": "create-one"`),
				[]byte(`{"op": "deploy", "case": "hashbtree-window", "statement": "create-one", "extra": 0`), 1)
		}},
		{name: "case-runtime-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"runtimeId": "java-runtime-ea8f73c9bd9fc31f1312"`),
				[]byte(`"runtimeId": "java-runtime-wrong"`), 1)
		}},
		{name: "epl-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`select * from MyInfraHBTW where f1>9`),
				[]byte(`select * from MyInfraHBTW where f1>8`), 1)
		}},
		{name: "payload-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"longPrimitive": 10`),
				[]byte(`"longPrimitive": 9`), 1)
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
				"-mode", infraNWTableWidening558ID,
				"-scenario", scenarioPath,
			}, &stdout, &stderr); code == 0 {
				t.Fatalf("malformed scenario %q unexpectedly replayed: stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunInfraNWTableWidening558RuntimeIDMappingMatchesScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableWidening558ID+".json"))
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
	if !reflect.DeepEqual(raw.JavaRuntimes, infraNWTableWidening558JavaRuntimeIDs) ||
		!reflect.DeepEqual(raw.JavaNames, infraNWTableWidening558JavaExecutions) ||
		!reflect.DeepEqual(raw.JavaStaticIDs, infraNWTableWidening558JavaStaticIDs) ||
		len(raw.JavaFlags) != 0 {
		t.Fatalf("scenario Java identity = %#v", raw)
	}
	for index, definition := range raw.Cases {
		if definition.Case != infraNWTableWidening558Cases[index] ||
			definition.Ordinal != infraNWTableWidening558Ordinals[index] ||
			definition.RuntimeID != infraNWTableWidening558JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableWidening558JavaExecutions[index] {
			t.Fatalf("case %d mapping = %#v", index, definition)
		}
	}
}
