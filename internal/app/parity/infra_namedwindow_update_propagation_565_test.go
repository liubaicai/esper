package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunInfraNWUP565ScenarioReplay replays the checked-in scenario through
// the Go runner and pins the record surface the oracle produces: listener
// deliveries for update-namedwindow (including the redeploy-barrier
// replays), the on-update trigger batches plus iterator snapshots for
// onupdate-multidispatch, the four output-snapshot ticks for
// outputrate-snapshot and the six any-order window snapshots for
// removestream-chain.
func TestRunInfraNWUP565ScenarioReplay(t *testing.T) {
	scenario := loadInfraNWUP565ScenarioForTest(t)
	trace, err := runInfraNWUP565Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion || trace.ID != infraNWUP565Id {
		t.Fatalf("trace identity = %v/%v", trace.Version, trace.ID)
	}
	if len(trace.Records) != 37 {
		t.Fatalf("records = %d, want 37", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var insertRows, windowRows, s0Rows [][]compat.ResultRecord
	var updateNewTotals []any
	var snapshotBatch [][]compat.ResultRecord
	for _, record := range trace.Records {
		byCase[record.Case]++
		byOp[record.Operation]++
		switch {
		case record.Operation == "listener" && record.Statement == "insert":
			insertRows = append(insertRows, record.New)
		case record.Operation == "listener" && record.Statement == "window":
			windowRows = append(windowRows, record.New)
		case record.Operation == "listener" && record.Statement == "s0" && record.Case == "update-namedwindow":
			s0Rows = append(s0Rows, record.New)
		case record.Operation == "listener" && record.Statement == "upd":
			for _, row := range record.New {
				updateNewTotals = append(updateNewTotals, row.Fields["total"])
			}
		case record.Operation == "listener" && record.Statement == "s0" && record.Case == "outputrate-snapshot":
			snapshotBatch = append(snapshotBatch, record.New)
		}
	}
	wantCase := map[string]int{
		"update-namedwindow":     16,
		"onupdate-multidispatch": 11,
		"outputrate-snapshot":    4,
		"removestream-chain":     6,
	}
	if len(byCase) != len(wantCase) {
		t.Fatalf("cases = %v", byCase)
	}
	for name, want := range wantCase {
		if byCase[name] != want {
			t.Fatalf("case %q records = %d, want %d", name, byCase[name], want)
		}
	}
	wantOp := map[string]int{"listener": 23, "snapshot": 14}
	for op, want := range wantOp {
		if byOp[op] != want {
			t.Fatalf("op %q records = %d, want %d", op, byOp[op], want)
		}
	}
	// Copy-on-write: every insert delivery (initial send plus the two
	// barrier replays) carries the pre-update {E1,oldvalue} while the
	// create-window statement sees {E1,newvalue}.
	if len(insertRows) != 3 || len(windowRows) != 3 {
		t.Fatalf("insert/window deliveries = %d/%d, want 3/3", len(insertRows), len(windowRows))
	}
	for index := range insertRows {
		if len(insertRows[index]) != 1 || insertRows[index][0].Fields["p0"] != "E1" || insertRows[index][0].Fields["p1"] != "oldvalue" {
			t.Fatalf("insert record %d = %v, want pre-update {E1,oldvalue}", index, insertRows[index])
		}
		if len(windowRows[index]) != 1 || windowRows[index][0].Fields["p1"] != "newvalue" {
			t.Fatalf("window record %d = %v, want post-update {E1,newvalue}", index, windowRows[index])
		}
	}
	// The second update-istream rewrites the routed MyOtherStream row.
	if len(s0Rows) != 1 || s0Rows[0][0].Fields["p0"] != "a" || s0Rows[0][0].Fields["p1"] != "b" {
		t.Fatalf("s0 rows = %v, want single {a,b}", s0Rows)
	}
	// The preemptive on-update accumulates totals 9, 8, 7 over the retained
	// value-3 row before each insert lands.
	wantTotals := []any{float64(9), float64(8), float64(7)}
	if len(updateNewTotals) != len(wantTotals) {
		t.Fatalf("upd new totals = %v, want %v", updateNewTotals, wantTotals)
	}
	for index, want := range wantTotals {
		if updateNewTotals[index] != want {
			t.Fatalf("upd new totals = %v, want %v", updateNewTotals, wantTotals)
		}
	}
	// The t=3000 snapshot re-emits the identical group rows: snapshots two
	// and three carry the same {A,2}/{B,2} rows.
	if len(snapshotBatch) != 4 {
		t.Fatalf("outputrate snapshots = %d, want 4", len(snapshotBatch))
	}
	if len(snapshotBatch[1]) != 2 || len(snapshotBatch[2]) != 2 {
		t.Fatalf("outputrate snapshot rows = %v", snapshotBatch)
	}
	equalRows := func(left, right []compat.ResultRecord) bool {
		l, _ := json.Marshal(left)
		r, _ := json.Marshal(right)
		return bytes.Equal(l, r)
	}
	if !equalRows(snapshotBatch[1], snapshotBatch[2]) {
		t.Fatalf("t=2000 vs t=3000 snapshots differ: %v vs %v", snapshotBatch[1], snapshotBatch[2])
	}
	if len(snapshotBatch[3]) != 3 {
		t.Fatalf("final snapshot rows = %v, want 3", snapshotBatch[3])
	}
}

// TestRunInfraNWUP565PinnedArtifacts pins the scenario file's Java identity
// plus the step sequence the loader accepts.
func TestRunInfraNWUP565PinnedArtifacts(t *testing.T) {
	scenario := loadInfraNWUP565ScenarioForTest(t)
	if scenario.ID != infraNWUP565Id {
		t.Fatalf("id = %q", scenario.ID)
	}
	// update-namedwindow: 36 steps (16 deploys across the three barrier
	// replays plus onselect x3/oninsert x2/update-other/s0, three undeploy-all
	// barriers, 8 sends including replays, final undeploy-all).
	wantSteps := 36
	wantSteps += 5 + 4*3 + 1           // onupdate-multidispatch: 5 deploys, 4x(send+2 snapshots), undeploy-all
	wantSteps += 2 + 1 + 1 + 6 + 4 + 1 // outputrate-snapshot: create+insert, advance(0), s0, 6 sends, 4 advances, undeploy-all
	wantSteps += 6 + 5 + 6 + 1         // removestream-chain: 6 deploys, 5 sends, 6 snapshots, undeploy-all
	wantSteps += 4                     // case markers
	if len(scenario.Steps) != wantSteps {
		t.Fatalf("steps = %d, want %d", len(scenario.Steps), wantSteps)
	}
	cases := 0
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			cases++
		}
	}
	if cases != 4 {
		t.Fatalf("case markers = %d, want 4", cases)
	}
}

// TestRunInfraNWUP565RejectsMalformedRawScenario verifies the loader pins
// the step sequence: a mutated EPL, reordered step or wrong statement label
// is rejected.
func TestRunInfraNWUP565RejectsMalformedRawScenario(t *testing.T) {
	data, err := os.ReadFile(infraNWUP565ScenarioPath(t))
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name    string
		replace string
		with    string
	}{
		{"mutated EPL", `"epl": "update istream AWindow set p1='newvalue'"`, `"epl": "update istream AWindow set p1='other'"`},
		{"wrong statement label", `"statement": "route-w2"`, `"statement": "route-w4"`},
		{"mutated payload", `"theString": "E3"`, `"theString": "E9"`},
		{"wrong time", `"at": "1970-01-01T00:00:03Z"`, `"at": "1970-01-01T00:00:05Z"`},
	}
	for _, mutation := range mutations {
		mutated := bytes.Replace(data, []byte(mutation.replace), []byte(mutation.with), 1)
		if bytes.Equal(data, mutated) {
			t.Fatalf("%s: pattern %q not found", mutation.name, mutation.replace)
		}
		if _, err := loadInfraNWUP565Scenario(bytes.NewReader(mutated)); err == nil {
			t.Fatalf("%s: mutated scenario accepted", mutation.name)
		}
	}
}

// TestRunInfraNWUP565RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunInfraNWUP565RuntimeIDMappingMatchesScenario(t *testing.T) {
	for index, spec := range infraNWUP565CaseSpecs {
		if spec.runtimeID != infraNWUP565JavaRuntimeIDs[index] {
			t.Fatalf("runtime ids = %v", infraNWUP565JavaRuntimeIDs)
		}
		if spec.execution != infraNWUP565JavaExecutions[index] {
			t.Fatalf("executions = %v", infraNWUP565JavaExecutions)
		}
		if spec.epl[spec.deploys[0]] == "" {
			t.Fatalf("case %q first deploy has no EPL pin", spec.name)
		}
	}
	staticIDs := []string{
		"java-082395e7cb9dbac98bea",
		"java-608c6908a0d53bf60fb5",
		"java-3ffa81dff07fbd6b6a84",
		"java-5d1931fb11b4c00a1ecc",
	}
	for index := range infraNWUP565CaseSpecs {
		if infraNWUP565JavaStaticIDs[index] != staticIDs[index] {
			t.Fatalf("static ids = %v", infraNWUP565JavaStaticIDs)
		}
	}
}

func infraNWUP565ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity", infraNWUP565Id+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadInfraNWUP565ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(infraNWUP565ScenarioPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scenario, err := loadInfraNWUP565Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
