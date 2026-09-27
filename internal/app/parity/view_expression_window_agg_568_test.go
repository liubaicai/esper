package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunViewExprWin568ScenarioReplay replays the checked-in scenario
// through the Go runner and pins the record surface the oracle produces:
// one deployed marker, nine listener deliveries (including both
// self-expiring inserts) and nine ordered iterator snapshots for
// aggregation-ungrouped; one deployed marker and six listener deliveries
// for aggregation-groupwin; three deployed markers (one module), four
// listener deliveries and two ordered snapshots for named-window-delete;
// three deployed markers and five listener deliveries for
// aggregation-on-delete.
func TestRunViewExprWin568ScenarioReplay(t *testing.T) {
	scenario := loadViewExprWin568ScenarioForTest(t)
	trace, err := runViewExprWin568Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprWin568ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 43 {
		t.Fatalf("records = %d, want 43", len(trace.Records))
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
	if byCase["aggregation-ungrouped"] != 19 || byCase["aggregation-groupwin"] != 7 ||
		byCase["named-window-delete"] != 9 || byCase["aggregation-on-delete"] != 8 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 8 || byOp["listener"] != 24 || byOp["snapshot"] != 11 {
		t.Fatalf("operation counts = %v", byOp)
	}

	// aggregation-ungrouped: E3/11 and E4/12 self-expire (each send posts
	// its own event on both streams while the iterator stays empty), E8/6
	// evicts {E5,E6} and E9/9 evicts {E7,E8}.
	var ungrouped []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "aggregation-ungrouped" {
			ungrouped = append(ungrouped, record)
		}
	}
	if len(ungrouped) != 9 {
		t.Fatalf("ungrouped listeners = %d", len(ungrouped))
	}
	if got := ungrouped[2].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("E3 new rows = %v", got)
	}
	if got := ungrouped[2].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E3" {
		t.Fatalf("E3 old rows = %v", got)
	}
	if got := ungrouped[3].New; len(got) != 1 || got[0].Fields["theString"] != "E4" {
		t.Fatalf("E4 new rows = %v", got)
	}
	if got := ungrouped[3].Old; len(got) != 1 || got[0].Fields["theString"] != "E4" {
		t.Fatalf("E4 old rows = %v", got)
	}
	if got := ungrouped[7].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E5" || got[1].Fields["theString"] != "E6" {
		t.Fatalf("E8 old rows = %v", got)
	}
	if got := ungrouped[8].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E7" || got[1].Fields["theString"] != "E8" {
		t.Fatalf("E9 old rows = %v", got)
	}
	var ungroupedSnaps []compat.TraceRecord
	for _, record := range snapshots {
		if record.Case == "aggregation-ungrouped" {
			ungroupedSnaps = append(ungroupedSnaps, record)
		}
	}
	if len(ungroupedSnaps) != 9 {
		t.Fatalf("ungrouped snapshots = %d", len(ungroupedSnaps))
	}
	if len(ungroupedSnaps[2].New) != 0 || len(ungroupedSnaps[3].New) != 0 {
		t.Fatalf("post-E3/post-E4 snapshots must be empty: %v %v",
			ungroupedSnaps[2].New, ungroupedSnaps[3].New)
	}
	if got := ungroupedSnaps[6].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E5" || got[1].Fields["theString"] != "E6" ||
		got[2].Fields["theString"] != "E7" {
		t.Fatalf("post-E7 snapshot = %v", got)
	}
	if got := ungroupedSnaps[8].New; len(got) != 1 || got[0].Fields["theString"] != "E9" {
		t.Fatalf("post-E9 snapshot = %v", got)
	}

	// aggregation-groupwin: E5/2/6 evicts the whole group-2 pair {E2,E4}
	// and E6/1/2 evicts the group-1 head {E1}.
	var groupwin []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "aggregation-groupwin" {
			groupwin = append(groupwin, record)
		}
	}
	if len(groupwin) != 6 {
		t.Fatalf("groupwin listeners = %d", len(groupwin))
	}
	if got := groupwin[4].New; len(got) != 1 || got[0].Fields["theString"] != "E5" {
		t.Fatalf("groupwin E5 new rows = %v", got)
	}
	if got := groupwin[4].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E4" {
		t.Fatalf("groupwin E5 old rows = %v", got)
	}
	if got := groupwin[5].Old; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("groupwin E6 old rows = %v", got)
	}

	// named-window-delete: the A(E2) trigger posts old {E2} between the
	// {E1,E2,E3} and {E1,E3} iterator pins.
	var nwDelete []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "named-window-delete" {
			nwDelete = append(nwDelete, record)
		}
	}
	if len(nwDelete) != 4 {
		t.Fatalf("named-window-delete listeners = %d", len(nwDelete))
	}
	if got := nwDelete[3]; len(got.New) != 0 || len(got.Old) != 1 ||
		got.Old[0].Fields["theString"] != "E2" {
		t.Fatalf("named-window-delete delete delivery = %v", got)
	}
	var nwSnaps []compat.TraceRecord
	for _, record := range snapshots {
		if record.Case == "named-window-delete" {
			nwSnaps = append(nwSnaps, record)
		}
	}
	if len(nwSnaps) != 2 {
		t.Fatalf("named-window-delete snapshots = %d", len(nwSnaps))
	}
	if got := nwSnaps[0].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("pre-delete snapshot = %v", got)
	}
	if got := nwSnaps[1].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E3" {
		t.Fatalf("post-delete snapshot = %v", got)
	}

	// aggregation-on-delete: A(E2) posts the old-only delete, then E4/2
	// posts new {E4} while the re-evaluated keep predicate pops {E1}.
	var onDelete []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "aggregation-on-delete" {
			onDelete = append(onDelete, record)
		}
	}
	if len(onDelete) != 5 {
		t.Fatalf("aggregation-on-delete listeners = %d", len(onDelete))
	}
	if got := onDelete[2]; len(got.New) != 0 || len(got.Old) != 1 ||
		got.Old[0].Fields["theString"] != "E2" {
		t.Fatalf("on-delete trigger delivery = %v", got)
	}
	if got := onDelete[4].New; len(got) != 1 || got[0].Fields["theString"] != "E4" {
		t.Fatalf("E4 new rows = %v", got)
	}
	if got := onDelete[4].Old; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E4 old rows = %v", got)
	}
}

// TestRunViewExprWin568PinnedArtifacts pins the scenario file's Java
// identity plus the step sequence the loader accepts.
func TestRunViewExprWin568PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprWin568ScenarioForTest(t)
	if scenario.ID != viewExprWin568ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 59 {
		t.Fatalf("steps = %d, want 59", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "aggregation-ungrouped" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var deploys []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "deploy" {
			deploys = append(deploys, step)
		}
	}
	if len(deploys) != 8 {
		t.Fatalf("deploys = %d", len(deploys))
	}
	if deploys[0].Epl != viewExprWin568EPLAggUngrouped ||
		deploys[1].Epl != viewExprWin568EPLAggGroupwin ||
		deploys[2].Epl != viewExprWin568EPLNWCreateKeep ||
		deploys[3].Epl != viewExprWin568EPLNWInsert ||
		deploys[4].Epl != viewExprWin568EPLNWDelete ||
		deploys[5].Epl != viewExprWin568EPLNWCreateAgg ||
		deploys[6].Epl != viewExprWin568EPLNWInsert ||
		deploys[7].Epl != viewExprWin568EPLNWDelete {
		t.Fatalf("deploy EPLs are not pinned")
	}
}

// TestRunViewExprWin568RejectsMalformedRawScenario verifies the loader pins
// the step sequence: a mutated EPL, reordered step or mutated payload is
// rejected.
func TestRunViewExprWin568RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprWin568ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data, []byte("sum(intPrimitive) < 10"), []byte("sum(intPrimitive) < 11"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"theString": "E9", "intPrimitive": 9`), []byte(`"theString": "E9", "intPrimitive": 8`), 1)
		}},
		{"delete id", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"eventType": "SupportBean_A", "payload": {"id": "E2"}`), []byte(`"eventType": "SupportBean_A", "payload": {"id": "E3"}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprWin568Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprWin568RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprWin568RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprWin568CaseSpecs) != len(viewExprWin568JavaRuntimeIDs) ||
		len(viewExprWin568CaseSpecs) != len(viewExprWin568JavaExecutions) ||
		len(viewExprWin568CaseSpecs) != len(viewExprWin568JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprWin568CaseSpecs))
	}
	for index, spec := range viewExprWin568CaseSpecs {
		if spec.runtimeID != viewExprWin568JavaRuntimeIDs[index] ||
			spec.execution != viewExprWin568JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprWin568JavaStaticIDs {
		if id != "java-06e6b1f6c905b8f12b82" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func viewExprWin568ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-window-agg-568.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprWin568ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprWin568ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprWin568Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
