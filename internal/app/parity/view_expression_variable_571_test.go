package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunViewExprVar571ScenarioReplay replays the checked-in scenario
// through the Go runner and pins the record surface the oracle produces:
// one deployed marker plus three flush deliveries for dynamic-time-batch;
// one deployed marker plus five flush deliveries for variable-batch; one
// deployed marker, four listener deliveries (the E1 keep, the old-only
// expel, the E2 self-expiring pair and the retained E3) and five ordered
// snapshots for variable-window; one deployed marker, five listener
// deliveries (the lazy E2 eviction at the 6000 advance included) and
// four ordered snapshots for dynamic-time-window.
func TestRunViewExprVar571ScenarioReplay(t *testing.T) {
	scenario := loadViewExprVar571ScenarioForTest(t)
	trace, err := runViewExprVar571Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprVar571ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 30 {
		t.Fatalf("records = %d, want 30", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var listeners []compat.TraceRecord
	var snapshots []compat.TraceRecord
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
	if byCase["dynamic-time-batch"] != 4 || byCase["variable-batch"] != 6 ||
		byCase["variable-window"] != 10 || byCase["dynamic-time-window"] != 10 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 4 || byOp["listener"] != 17 || byOp["snapshot"] != 9 {
		t.Fatalf("operation counts = %v", byOp)
	}

	// dynamic-time-batch: the SIZE=500 shrink plus the 1901 advance
	// flushes the pending {E1,E2} WITHOUT a new send; E5 and E8 flush on
	// arriving sends under the strict `>` trigger.
	var dynamicBatch []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "dynamic-time-batch" {
			dynamicBatch = append(dynamicBatch, record)
		}
	}
	if len(dynamicBatch) != 3 {
		t.Fatalf("dynamic-time-batch listeners = %d", len(dynamicBatch))
	}
	if dynamicBatch[0].Time != "1970-01-01T00:00:01.901Z" {
		t.Fatalf("shrink flush time = %q", dynamicBatch[0].Time)
	}
	if got := dynamicBatch[0].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" {
		t.Fatalf("shrink flush new rows = %v", got)
	}
	if len(dynamicBatch[0].Old) != 0 {
		t.Fatalf("shrink flush old rows = %v", dynamicBatch[0].Old)
	}
	if dynamicBatch[1].Time != "1970-01-01T00:00:02.500Z" {
		t.Fatalf("E5 flush time = %q", dynamicBatch[1].Time)
	}
	if got := dynamicBatch[1].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E3" || got[1].Fields["theString"] != "E4" ||
		got[2].Fields["theString"] != "E5" {
		t.Fatalf("E5 flush new rows = %v", got)
	}
	if got := dynamicBatch[1].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" {
		t.Fatalf("E5 flush old rows = %v", got)
	}
	if dynamicBatch[2].Time != "1970-01-01T00:00:04.100Z" {
		t.Fatalf("E8 flush time = %q", dynamicBatch[2].Time)
	}
	if got := dynamicBatch[2].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E6" || got[1].Fields["theString"] != "E7" ||
		got[2].Fields["theString"] != "E8" {
		t.Fatalf("E8 flush new rows = %v", got)
	}
	if got := dynamicBatch[2].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E3" || got[1].Fields["theString"] != "E4" ||
		got[2].Fields["theString"] != "E5" {
		t.Fatalf("E8 flush old rows = %v", got)
	}

	// variable-batch: POST toggles gate the flush; the 1001/2001
	// advances flush the pending batch without a send and each arrival
	// while POST=true closes its own batch.
	var variableBatch []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "variable-batch" {
			variableBatch = append(variableBatch, record)
		}
	}
	if len(variableBatch) != 5 {
		t.Fatalf("variable-batch listeners = %d", len(variableBatch))
	}
	if variableBatch[0].Time != "1970-01-01T00:00:01.001Z" {
		t.Fatalf("POST-true flush time = %q", variableBatch[0].Time)
	}
	if got := variableBatch[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("1001 flush new rows = %v", got)
	}
	if len(variableBatch[0].Old) != 0 {
		t.Fatalf("1001 flush old rows = %v", variableBatch[0].Old)
	}
	if got := variableBatch[1].New; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 flush new rows = %v", got)
	}
	if got := variableBatch[1].Old; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E2 flush old rows = %v", got)
	}
	if got := variableBatch[2].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("E3 flush new rows = %v", got)
	}
	if got := variableBatch[2].Old; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E3 flush old rows = %v", got)
	}
	if variableBatch[3].Time != "1970-01-01T00:00:02.001Z" {
		t.Fatalf("2001 flush time = %q", variableBatch[3].Time)
	}
	if got := variableBatch[3].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" {
		t.Fatalf("2001 flush new rows = %v", got)
	}
	if got := variableBatch[3].Old; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("2001 flush old rows = %v", got)
	}
	if got := variableBatch[4].New; len(got) != 1 || got[0].Fields["theString"] != "E6" {
		t.Fatalf("E6 flush new rows = %v", got)
	}
	if got := variableBatch[4].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" {
		t.Fatalf("E6 flush old rows = %v", got)
	}

	// variable-window: the old-only {E1} expel rides the 1001 advance
	// after KEEP=false, E2 self-expires as an {E2}/{E2} pair while KEEP
	// stays false, and E3 is retained once KEEP=true is restored.
	var variableWindow []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "variable-window" {
			variableWindow = append(variableWindow, record)
		}
	}
	if len(variableWindow) != 4 {
		t.Fatalf("variable-window listeners = %d", len(variableWindow))
	}
	if got := variableWindow[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E1 delivery new rows = %v", got)
	}
	if variableWindow[1].Time != "1970-01-01T00:00:01.001Z" {
		t.Fatalf("expel time = %q", variableWindow[1].Time)
	}
	if len(variableWindow[1].New) != 0 {
		t.Fatalf("expel new rows = %v", variableWindow[1].New)
	}
	if got := variableWindow[1].Old; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("expel old rows = %v", got)
	}
	if got := variableWindow[2].New; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 pair new rows = %v", got)
	}
	if got := variableWindow[2].Old; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 pair old rows = %v", got)
	}
	if got := variableWindow[3].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("E3 delivery new rows = %v", got)
	}
	if len(variableWindow[3].Old) != 0 {
		t.Fatalf("E3 delivery old rows = %v", variableWindow[3].Old)
	}
	// Snapshot sequence: {E1}, {E1} (the KEEP=false flip alone does not
	// evict), empty, empty (E2 self-expired), {E3}.
	var windowSnapshots []compat.TraceRecord
	for _, record := range snapshots {
		if record.Case == "variable-window" {
			windowSnapshots = append(windowSnapshots, record)
		}
	}
	if len(windowSnapshots) != 5 {
		t.Fatalf("variable-window snapshots = %d", len(windowSnapshots))
	}
	if got := windowSnapshots[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("first snapshot = %v", got)
	}
	if got := windowSnapshots[1].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("post-flip snapshot = %v", got)
	}
	if len(windowSnapshots[2].New) != 0 || len(windowSnapshots[3].New) != 0 {
		t.Fatalf("expired snapshots = %v / %v", windowSnapshots[2].New, windowSnapshots[3].New)
	}
	if got := windowSnapshots[4].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("final snapshot = %v", got)
	}

	// dynamic-time-window: the strict `<` keep expires E1 when E2 lands
	// at 2000 and the SIZE=2000 shrink lazily evicts E2 on the 6000
	// advance before E4 arrives.
	var dynamicWindow []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "dynamic-time-window" {
			dynamicWindow = append(dynamicWindow, record)
		}
	}
	if len(dynamicWindow) != 5 {
		t.Fatalf("dynamic-time-window listeners = %d", len(dynamicWindow))
	}
	if got := dynamicWindow[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E1 delivery new rows = %v", got)
	}
	if got := dynamicWindow[1].New; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 delivery new rows = %v", got)
	}
	if got := dynamicWindow[1].Old; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E2 delivery old rows = %v", got)
	}
	if got := dynamicWindow[2].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("E3 delivery new rows = %v", got)
	}
	if dynamicWindow[3].Time != "1970-01-01T00:00:06Z" {
		t.Fatalf("E2 eviction time = %q", dynamicWindow[3].Time)
	}
	if len(dynamicWindow[3].New) != 0 {
		t.Fatalf("E2 eviction new rows = %v", dynamicWindow[3].New)
	}
	if got := dynamicWindow[3].Old; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 eviction old rows = %v", got)
	}
	if got := dynamicWindow[4].New; len(got) != 1 || got[0].Fields["theString"] != "E4" {
		t.Fatalf("E4 delivery new rows = %v", got)
	}
	var dynSnapshots []compat.TraceRecord
	for _, record := range snapshots {
		if record.Case == "dynamic-time-window" {
			dynSnapshots = append(dynSnapshots, record)
		}
	}
	if len(dynSnapshots) != 4 {
		t.Fatalf("dynamic-time-window snapshots = %d", len(dynSnapshots))
	}
	if got := dynSnapshots[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E1 snapshot = %v", got)
	}
	if got := dynSnapshots[1].New; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 snapshot = %v", got)
	}
	if got := dynSnapshots[2].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E3" {
		t.Fatalf("E3 snapshot = %v", got)
	}
	if got := dynSnapshots[3].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E3" || got[1].Fields["theString"] != "E4" {
		t.Fatalf("E4 snapshot = %v", got)
	}
}

// TestRunViewExprVar571PinnedArtifacts pins the scenario file's Java
// identity plus the step sequence the loader accepts.
func TestRunViewExprVar571PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprVar571ScenarioForTest(t)
	if scenario.ID != viewExprVar571ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 77 {
		t.Fatalf("steps = %d, want 77", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "dynamic-time-batch" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var deploys []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "deploy" {
			deploys = append(deploys, step)
		}
	}
	if len(deploys) != 4 {
		t.Fatalf("deploys = %d, want 4", len(deploys))
	}
	if deploys[0].Epl != viewExprVar571EPLDynamicTimeBatch ||
		deploys[1].Epl != viewExprVar571EPLVariableBatch ||
		deploys[2].Epl != viewExprVar571EPLVariableWindow ||
		deploys[3].Epl != viewExprVar571EPLDynamicTimeWindow {
		t.Fatalf("deploy EPLs are not pinned")
	}
}

// TestRunViewExprVar571RejectsMalformedRawScenario verifies the loader
// pins the step sequence: a mutated EPL, mutated send payload, mutated
// advance-time instant, mutated variable assignment or mutated snapshot
// mode is rejected.
func TestRunViewExprVar571RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprVar571ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl operator", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`SupportBean#expr_batch(newest_timestamp - oldest_timestamp > SIZE);\n"`),
				[]byte(`SupportBean#expr_batch(newest_timestamp - oldest_timestamp >= SIZE);\n"`), 1)
		}},
		{"epl trailing newline", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`SupportBean#expr_batch(POST);\n"`),
				[]byte(`SupportBean#expr_batch(POST)"`), 1)
		}},
		{"epl missing semicolon", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`SupportBean#expr(newest_timestamp - oldest_timestamp < SIZE)"`),
				[]byte(`SupportBean#expr(newest_timestamp - oldest_timestamp < SIZE);"`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "E8", "intPrimitive": 0`),
				[]byte(`"theString": "E8", "intPrimitive": 1`), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:01.901Z"`),
				[]byte(`"at": "1970-01-01T00:00:01.902Z"`), 1)
		}},
		{"set-variable value", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"name": "SIZE", "payload": 500`),
				[]byte(`"name": "SIZE", "payload": 501`), 1)
		}},
		{"set-variable name", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"statement": "s0", "name": "KEEP"`),
				[]byte(`"statement": "s0", "name": "KEPT"`), 1)
		}},
		{"snapshot mode", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"mode": "ordered"`),
				[]byte(`"mode": "any"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprVar571Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprVar571RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprVar571RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprVar571CaseSpecs) != len(viewExprVar571JavaRuntimeIDs) ||
		len(viewExprVar571CaseSpecs) != len(viewExprVar571JavaExecutions) ||
		len(viewExprVar571CaseSpecs) != len(viewExprVar571JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprVar571CaseSpecs))
	}
	for index, spec := range viewExprVar571CaseSpecs {
		if spec.runtimeID != viewExprVar571JavaRuntimeIDs[index] ||
			spec.execution != viewExprVar571JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprVar571JavaStaticIDs[:2] {
		if id != "java-20551a17cb2af08c67fc" {
			t.Fatalf("batch static id %d = %q", index, id)
		}
	}
	for index, id := range viewExprVar571JavaStaticIDs[2:] {
		if id != "java-06e6b1f6c905b8f12b82" {
			t.Fatalf("window static id %d = %q", index, id)
		}
	}
}

func viewExprVar571ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-variable-571.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprVar571ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprVar571ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprVar571Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
