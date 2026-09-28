package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// The bundle's two compile-error-only siblings enter the manifest as
// intentionally-different rows (no scenario steps, no trace — the typed Go
// API has no EPL-text compile path, so the verbatim Java diagnostics are
// recorded here and the representative Go checks stay the structural
// Build rejections in internal/esper/view_expression_parity_test.go):
//
//   - ViewExpressionWindow ord 5 ViewExpressionWindowInvalid
//     java-runtime-984f59c809d70e02b3da:
//     `select * from SupportBean#expr(1)` ->
//     "Failed to validate data window declaration: Invalid return value
//     for expiry expression, expected a boolean return value but received
//     int [select * from SupportBean#expr(1)]"
//     `select * from SupportBean#expr((select * from SupportBean#lastevent))` ->
//     "Failed to validate data window declaration: Invalid expiry
//     expression: Sub-select, previous or prior functions are not
//     supported in this context [select * from
//     SupportBean#expr((select * from SupportBean#lastevent))]"
//   - ViewExpressionBatch ord 4 ViewExpressionBatchInvalid
//     java-runtime-ad02b23eda34efdfd83e: the same two probes with
//     #expr_batch, plus
//     `select * from SupportBean#expr_batch(null < 0)` ->
//     "Failed to validate data window declaration: Invalid parameter
//     expression 0 for Expression-batch view: Failed to validate view
//     parameter expression 'null<0': Null-type value is not allow for
//     relational operator" (Java's `is not allow` typo verbatim;
//     unrepresentable in the typed Go API).

// TestRunViewExprWinTime573ScenarioReplay replays the checked-in scenario
// through the Go runner and pins the record surface the oracle produces:
// one deployed marker, eight listener deliveries (one per send — every
// send is retained, with evictions riding the remove stream at E5, E7 and
// E8) and eight in-order iterator snapshots.
func TestRunViewExprWinTime573ScenarioReplay(t *testing.T) {
	scenario := loadViewExprWinTime573ScenarioForTest(t)
	trace, err := runViewExprWinTime573Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprWinTime573ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 17 {
		t.Fatalf("records = %d, want 17", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var listeners, snapshots []compat.TraceRecord
	for _, record := range trace.Records {
		byCase[record.Case]++
		byOp[record.Operation]++
		switch record.Operation {
		case "listener":
			listeners = append(listeners, record)
		case "snapshot":
			snapshots = append(snapshots, record)
		}
	}
	if byCase["time-window"] != 17 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 1 || byOp["listener"] != 8 || byOp["snapshot"] != 8 {
		t.Fatalf("operation counts = %v", byOp)
	}
	if len(listeners) != 8 || len(snapshots) != 8 {
		t.Fatalf("listeners/snapshots = %d/%d", len(listeners), len(snapshots))
	}
	// E1..E4 and E6 deliver new-only (no evictions); E5 evicts {E1}, E7
	// evicts the t=1500-tied pair {E2,E3} in ONE delivery and E8
	// mass-expires {E4,E5,E6,E7}.
	for index, name := range []string{"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8"} {
		record := listeners[index]
		if record.Case != "time-window" || record.Statement != "s0" || record.Sequence != uint64(index+1) {
			t.Fatalf("listener %d = %+v", index, record)
		}
		if len(record.New) != 1 || record.New[0].Fields["theString"] != name {
			t.Fatalf("listener %d new rows = %v", index, record.New)
		}
	}
	for _, index := range []int{0, 1, 2, 3, 5} {
		if len(listeners[index].Old) != 0 {
			t.Fatalf("listener %d should be new-only: %v", index, listeners[index].Old)
		}
	}
	if got := listeners[4].Old; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E5 old rows = %v", got)
	}
	if got := listeners[6].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E3" {
		t.Fatalf("E7 double-old rows = %v", got)
	}
	if got := listeners[7].Old; len(got) != 4 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" ||
		got[2].Fields["theString"] != "E6" || got[3].Fields["theString"] != "E7" {
		t.Fatalf("E8 mass-expiry rows = %v", got)
	}
	// Per-send iterator pins (the post-E2 snapshot is the stronger pin the
	// Java assertions leave out): {E1}, {E1,E2}, {E1,E2,E3}, {E1..E4},
	// {E2..E5}, {E2..E6}, {E4..E7}, {E8}.
	want := [][]string{
		{"E1"},
		{"E1", "E2"},
		{"E1", "E2", "E3"},
		{"E1", "E2", "E3", "E4"},
		{"E2", "E3", "E4", "E5"},
		{"E2", "E3", "E4", "E5", "E6"},
		{"E4", "E5", "E6", "E7"},
		{"E8"},
	}
	for index, rows := range want {
		record := snapshots[index]
		if len(record.New) != len(rows) {
			t.Fatalf("snapshot %d rows = %v, want %v", index, record.New, rows)
		}
		for rowIndex, name := range rows {
			if record.New[rowIndex].Fields["theString"] != name {
				t.Fatalf("snapshot %d row %d = %v, want %q", index, rowIndex,
					record.New[rowIndex].Fields["theString"], name)
			}
		}
	}
	// Lazy expiry on send: the advanceTime(10000) step between the E6 and
	// E8 sends posts no listener record and no eviction — the mass-expiry
	// rides the E8 send, and the snapshot right before E8 still saw
	// {E4,E5,E6,E7}... i.e. the E8 listener record is the only old-data
	// delivery after E7.
}

// TestRunViewExprWinTime573PinnedArtifacts pins the scenario file's Java
// identity plus the step sequence the loader accepts.
func TestRunViewExprWinTime573PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprWinTime573ScenarioForTest(t)
	if scenario.ID != viewExprWinTime573ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 28 {
		t.Fatalf("steps = %d, want 28", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "time-window" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var deploys []compat.Step
	var advances, sends, snapshots int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "deploy":
			deploys = append(deploys, step)
		case "advance-time":
			advances++
		case "send":
			sends++
		case "snapshot":
			snapshots++
		}
	}
	if len(deploys) != 1 || deploys[0].Epl != viewExprWinTime573EPL {
		t.Fatalf("deploys = %+v", deploys)
	}
	if advances != 8 || sends != 8 || snapshots != 8 {
		t.Fatalf("advances/sends/snapshots = %d/%d/%d", advances, sends, snapshots)
	}
	// No advance-time step may sit between the E2 and E3 sends — that gap
	// is what gives E3 the same t=1500 arrival timestamp as E2 and makes
	// E7's double-old delivery the discriminant.
	e2, e3 := -1, -1
	for index, step := range scenario.Steps {
		if step.Op == "send" {
			if step.EventType == "SupportBean" {
				if bytes.Contains(step.Payload, []byte(`"theString": "E2"`)) {
					e2 = index
				}
				if bytes.Contains(step.Payload, []byte(`"theString": "E3"`)) {
					e3 = index
				}
			}
		}
	}
	if e2 < 0 || e3 < 0 {
		t.Fatalf("E2/E3 sends not found: %d/%d", e2, e3)
	}
	for index := e2 + 1; index < e3; index++ {
		if scenario.Steps[index].Op == "advance-time" {
			t.Fatalf("advance-time between the E2 and E3 sends breaks the t=1500 tie: step %d", index)
		}
	}
}

// TestRunViewExprWinTime573RejectsMalformedRawScenario verifies the
// loader pins the step sequence: a mutated EPL, mutated advance target or
// mutated send payload is rejected.
func TestRunViewExprWinTime573RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprWinTime573ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("oldest_timestamp > newest_timestamp - 2000"),
				[]byte("newest_timestamp - oldest_timestamp > 2000"), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data, []byte("00:00:03.499"), []byte("00:00:03.500"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "E3", "intPrimitive": 3`),
				[]byte(`"theString": "E3", "intPrimitive": 4`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprWinTime573Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprWinTime573RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprWinTime573RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprWinTime573CaseSpecs) != len(viewExprWinTime573JavaRuntimeIDs) ||
		len(viewExprWinTime573CaseSpecs) != len(viewExprWinTime573JavaExecutions) ||
		len(viewExprWinTime573CaseSpecs) != len(viewExprWinTime573JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprWinTime573CaseSpecs))
	}
	for index, spec := range viewExprWinTime573CaseSpecs {
		if spec.runtimeID != viewExprWinTime573JavaRuntimeIDs[index] ||
			spec.execution != viewExprWinTime573JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprWinTime573JavaStaticIDs {
		if id != "java-06e6b1f6c905b8f12b82" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
	if viewExprWinTime573CaseSpecs[0].ordinal != 3 {
		t.Fatalf("time-window ordinal = %d, want 3", viewExprWinTime573CaseSpecs[0].ordinal)
	}
}

func viewExprWinTime573ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-window-time-573.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprWinTime573ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprWinTime573ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprWinTime573Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
