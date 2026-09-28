package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunViewExprBatchAgg569ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: one deployed marker and three batch-flush deliveries for
// aggregation-ungrouped; one deployed marker and five deliveries for
// aggregation-groupwin; three deployed markers (one module) and one flush
// delivery for aggregation-on-delete; three deployed markers, one flush
// delivery and two ordered snapshots for named-window-delete.
func TestRunViewExprBatchAgg569ScenarioReplay(t *testing.T) {
	scenario := loadViewExprBatchAgg569ScenarioForTest(t)
	trace, err := runViewExprBatchAgg569Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprBatchAgg569ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 20 {
		t.Fatalf("records = %d, want 20", len(trace.Records))
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
	if byCase["aggregation-ungrouped"] != 4 || byCase["aggregation-groupwin"] != 6 ||
		byCase["aggregation-on-delete"] != 4 || byCase["named-window-delete"] != 6 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 8 || byOp["listener"] != 10 || byOp["snapshot"] != 2 {
		t.Fatalf("operation counts = %v", byOp)
	}

	// aggregation-ungrouped: each flush posts the whole accumulated batch
	// as new and the prior batch as old.
	var ungrouped []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "aggregation-ungrouped" {
			ungrouped = append(ungrouped, record)
		}
	}
	if len(ungrouped) != 3 {
		t.Fatalf("ungrouped listeners = %d", len(ungrouped))
	}
	if got := ungrouped[0].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("E3 new rows = %v", got)
	}
	if len(ungrouped[0].Old) != 0 {
		t.Fatalf("E3 old rows = %v", ungrouped[0].Old)
	}
	if got := ungrouped[1].New; len(got) != 1 || got[0].Fields["theString"] != "E4" {
		t.Fatalf("E4 new rows = %v", got)
	}
	if got := ungrouped[1].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("E4 old rows = %v", got)
	}
	if got := ungrouped[2].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E5" || got[1].Fields["theString"] != "E6" ||
		got[2].Fields["theString"] != "E7" {
		t.Fatalf("E7 new rows = %v", got)
	}
	if got := ungrouped[2].Old; len(got) != 1 || got[0].Fields["theString"] != "E4" {
		t.Fatalf("E7 old rows = %v", got)
	}

	// aggregation-groupwin: only the triggering group flushes; the
	// group's prior batch leaves as old data.
	var groupwin []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "aggregation-groupwin" {
			groupwin = append(groupwin, record)
		}
	}
	if len(groupwin) != 5 {
		t.Fatalf("groupwin listeners = %d", len(groupwin))
	}
	if got := groupwin[0].New; len(got) != 4 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E4" ||
		got[2].Fields["theString"] != "E5" || got[3].Fields["theString"] != "E6" {
		t.Fatalf("E6 new rows = %v", got)
	}
	if len(groupwin[0].Old) != 0 || len(groupwin[1].Old) != 0 {
		t.Fatalf("first group flushes must have no old rows: %v %v",
			groupwin[0].Old, groupwin[1].Old)
	}
	if got := groupwin[1].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E3" ||
		got[2].Fields["theString"] != "E8" {
		t.Fatalf("E8 new rows = %v", got)
	}
	if got := groupwin[2].New; len(got) != 1 || got[0].Fields["theString"] != "E10" {
		t.Fatalf("E10 new rows = %v", got)
	}
	if got := groupwin[2].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E3" ||
		got[2].Fields["theString"] != "E8" {
		t.Fatalf("E10 old rows = %v", got)
	}
	if got := groupwin[3].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E7" || got[1].Fields["theString"] != "E9" ||
		got[2].Fields["theString"] != "E11" {
		t.Fatalf("E11 new rows = %v", got)
	}
	if got := groupwin[3].Old; len(got) != 4 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E4" ||
		got[2].Fields["theString"] != "E5" || got[3].Fields["theString"] != "E6" {
		t.Fatalf("E11 old rows = %v", got)
	}
	if got := groupwin[4].New; len(got) != 1 || got[0].Fields["theString"] != "E12" {
		t.Fatalf("E12 new rows = %v", got)
	}
	if got := groupwin[4].Old; len(got) != 1 || got[0].Fields["theString"] != "E10" {
		t.Fatalf("E12 old rows = %v", got)
	}

	// aggregation-on-delete: silent accumulation and a silent delete,
	// then E4/1 flushes new {E1,E3,E4} with no old rows.
	var onDelete []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "aggregation-on-delete" {
			onDelete = append(onDelete, record)
		}
	}
	if len(onDelete) != 1 {
		t.Fatalf("aggregation-on-delete listeners = %d", len(onDelete))
	}
	if got := onDelete[0].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E3" ||
		got[2].Fields["theString"] != "E4" {
		t.Fatalf("E4 flush rows = %v", got)
	}
	if len(onDelete[0].Old) != 0 {
		t.Fatalf("E4 old rows = %v", onDelete[0].Old)
	}

	// named-window-delete: iterator snapshots bracket the silent A(E2)
	// delete and E5 flushes the four-row pending batch.
	var nwDelete []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "named-window-delete" {
			nwDelete = append(nwDelete, record)
		}
	}
	if len(nwDelete) != 1 {
		t.Fatalf("named-window-delete listeners = %d", len(nwDelete))
	}
	if got := nwDelete[0].New; len(got) != 4 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E3" ||
		got[2].Fields["theString"] != "E4" || got[3].Fields["theString"] != "E5" {
		t.Fatalf("E5 flush rows = %v", got)
	}
	if len(nwDelete[0].Old) != 0 {
		t.Fatalf("E5 old rows = %v", nwDelete[0].Old)
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
}

// TestRunViewExprBatchAgg569PinnedArtifacts pins the scenario file's Java
// identity plus the step sequence the loader accepts.
func TestRunViewExprBatchAgg569PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprBatchAgg569ScenarioForTest(t)
	if scenario.ID != viewExprBatchAgg569ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 56 {
		t.Fatalf("steps = %d, want 56", len(scenario.Steps))
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
	if deploys[0].Epl != viewExprBatchAgg569EPLAggUngrouped ||
		deploys[1].Epl != viewExprBatchAgg569EPLAggGroupwin ||
		deploys[2].Epl != viewExprBatchAgg569EPLNWCreateAgg ||
		deploys[3].Epl != viewExprBatchAgg569EPLNWInsert ||
		deploys[4].Epl != viewExprBatchAgg569EPLNWDelete ||
		deploys[5].Epl != viewExprBatchAgg569EPLNWCreateCount ||
		deploys[6].Epl != viewExprBatchAgg569EPLNWInsert ||
		deploys[7].Epl != viewExprBatchAgg569EPLNWDelete {
		t.Fatalf("deploy EPLs are not pinned")
	}
}

// TestRunViewExprBatchAgg569RejectsMalformedRawScenario verifies the
// loader pins the step sequence: a mutated EPL, reordered step or mutated
// payload is rejected.
func TestRunViewExprBatchAgg569RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprBatchAgg569ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"statement": "s0", "epl": "@name('s0') select irstream theString from SupportBean#expr_batch(sum(intPrimitive) > 100)"`),
				[]byte(`"statement": "s0", "epl": "@name('s0') select irstream theString from SupportBean#expr_batch(sum(intPrimitive) > 101)"`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"theString": "E12", "intPrimitive": 1, "longPrimitive": 102`), []byte(`"theString": "E12", "intPrimitive": 1, "longPrimitive": 101`), 1)
		}},
		{"delete id", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"eventType": "SupportBean_A", "payload": {"id": "E2"}`), []byte(`"eventType": "SupportBean_A", "payload": {"id": "E3"}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprBatchAgg569Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprBatchAgg569RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprBatchAgg569RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprBatchAgg569CaseSpecs) != len(viewExprBatchAgg569JavaRuntimeIDs) ||
		len(viewExprBatchAgg569CaseSpecs) != len(viewExprBatchAgg569JavaExecutions) ||
		len(viewExprBatchAgg569CaseSpecs) != len(viewExprBatchAgg569JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprBatchAgg569CaseSpecs))
	}
	for index, spec := range viewExprBatchAgg569CaseSpecs {
		if spec.runtimeID != viewExprBatchAgg569JavaRuntimeIDs[index] ||
			spec.execution != viewExprBatchAgg569JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprBatchAgg569JavaStaticIDs {
		if id != "java-20551a17cb2af08c67fc" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func viewExprBatchAgg569ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-batch-agg-569.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprBatchAgg569ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprBatchAgg569ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprBatchAgg569Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
