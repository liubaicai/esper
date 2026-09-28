package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunViewExprBatchCore570ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: two deployed markers and five batch-flush deliveries across
// the two newest-oldest phases; one deployed marker and three flush
// deliveries for length-batch; one deployed marker and two flush
// deliveries for time-batch; one deployed marker and three flush
// deliveries for event-prop-batch.
func TestRunViewExprBatchCore570ScenarioReplay(t *testing.T) {
	scenario := loadViewExprBatchCore570ScenarioForTest(t)
	trace, err := runViewExprBatchCore570Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprBatchCore570ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 18 {
		t.Fatalf("records = %d, want 18", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var listeners []compat.TraceRecord
	var deployed []compat.TraceRecord
	for _, record := range trace.Records {
		byCase[record.Case]++
		byOp[record.Operation]++
		switch record.Operation {
		case "listener":
			listeners = append(listeners, record)
		case "deployed":
			deployed = append(deployed, record)
		}
	}
	if byCase["newest-oldest"] != 7 || byCase["length-batch"] != 4 ||
		byCase["time-batch"] != 3 || byCase["event-prop-batch"] != 4 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 5 || byOp["listener"] != 13 {
		t.Fatalf("operation counts = %v", byOp)
	}
	// The second newest-oldest deployment keeps the per-case sequence
	// counters: the include phase's deployed marker continues at 2.
	if len(deployed) != 5 || deployed[0].Sequence != 1 || deployed[1].Sequence != 2 {
		t.Fatalf("newest-oldest deployed sequences = %v", deployed)
	}

	// newest-oldest exclude-trigger-event phase: the flushed batch is
	// the accumulation before the triggering send.
	var newestOldest []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "newest-oldest" {
			newestOldest = append(newestOldest, record)
		}
	}
	if len(newestOldest) != 5 {
		t.Fatalf("newest-oldest listeners = %d", len(newestOldest))
	}
	if got := newestOldest[0].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" {
		t.Fatalf("exclude E3 new rows = %v", got)
	}
	if len(newestOldest[0].Old) != 0 {
		t.Fatalf("exclude E3 old rows = %v", newestOldest[0].Old)
	}
	if got := newestOldest[1].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("exclude E4 new rows = %v", got)
	}
	if got := newestOldest[1].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" {
		t.Fatalf("exclude E4 old rows = %v", got)
	}
	if got := newestOldest[2].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" ||
		got[2].Fields["theString"] != "E6" {
		t.Fatalf("exclude E7 new rows = %v", got)
	}
	if got := newestOldest[2].Old; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("exclude E7 old rows = %v", got)
	}
	// newest-oldest include-trigger-event phase: the triggering send is
	// part of the batch it fired on.
	if got := newestOldest[3].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("include E3 new rows = %v", got)
	}
	if len(newestOldest[3].Old) != 0 {
		t.Fatalf("include E3 old rows = %v", newestOldest[3].Old)
	}
	if got := newestOldest[4].New; len(got) != 4 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" ||
		got[2].Fields["theString"] != "E6" || got[3].Fields["theString"] != "E7" {
		t.Fatalf("include E7 new rows = %v", got)
	}
	if got := newestOldest[4].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("include E7 old rows = %v", got)
	}

	// length-batch: every third send flushes the triple, the prior
	// batch leaves as old data.
	var lengthBatch []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "length-batch" {
			lengthBatch = append(lengthBatch, record)
		}
	}
	if len(lengthBatch) != 3 {
		t.Fatalf("length-batch listeners = %d", len(lengthBatch))
	}
	if got := lengthBatch[0].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("E3 new rows = %v", got)
	}
	if len(lengthBatch[0].Old) != 0 {
		t.Fatalf("E3 old rows = %v", lengthBatch[0].Old)
	}
	if got := lengthBatch[1].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" ||
		got[2].Fields["theString"] != "E6" {
		t.Fatalf("E6 new rows = %v", got)
	}
	if got := lengthBatch[1].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" {
		t.Fatalf("E6 old rows = %v", got)
	}
	if got := lengthBatch[2].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E7" || got[1].Fields["theString"] != "E8" ||
		got[2].Fields["theString"] != "E9" {
		t.Fatalf("E9 new rows = %v", got)
	}
	if got := lengthBatch[2].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" ||
		got[2].Fields["theString"] != "E6" {
		t.Fatalf("E9 old rows = %v", got)
	}

	// time-batch: the lone advances to t=3100 and t=5101 deliver
	// nothing; only E5@3100 and E8@5101 flush.
	var timeBatch []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "time-batch" {
			timeBatch = append(timeBatch, record)
		}
	}
	if len(timeBatch) != 2 {
		t.Fatalf("time-batch listeners = %d", len(timeBatch))
	}
	if timeBatch[0].Time != "1970-01-01T00:00:03.100Z" {
		t.Fatalf("E5 flush time = %q", timeBatch[0].Time)
	}
	if got := timeBatch[0].New; len(got) != 5 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" || got[3].Fields["theString"] != "E4" ||
		got[4].Fields["theString"] != "E5" {
		t.Fatalf("E5 new rows = %v", got)
	}
	if len(timeBatch[0].Old) != 0 {
		t.Fatalf("E5 old rows = %v", timeBatch[0].Old)
	}
	if timeBatch[1].Time != "1970-01-01T00:00:05.101Z" {
		t.Fatalf("E8 flush time = %q", timeBatch[1].Time)
	}
	if got := timeBatch[1].New; len(got) != 3 ||
		got[0].Fields["theString"] != "E6" || got[1].Fields["theString"] != "E7" ||
		got[2].Fields["theString"] != "E8" {
		t.Fatalf("E8 new rows = %v", got)
	}
	if got := timeBatch[1].Old; len(got) != 5 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" ||
		got[2].Fields["theString"] != "E3" || got[3].Fields["theString"] != "E4" ||
		got[4].Fields["theString"] != "E5" {
		t.Fatalf("E8 old rows = %v", got)
	}

	// event-prop-batch: the silent E3/-1 is retained in the pending
	// batch and rides E4's flush under the val0 projection.
	var eventProp []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "event-prop-batch" {
			eventProp = append(eventProp, record)
		}
	}
	if len(eventProp) != 3 {
		t.Fatalf("event-prop-batch listeners = %d", len(eventProp))
	}
	if got := eventProp[0].New; len(got) != 1 || got[0].Fields["val0"] != "E1" {
		t.Fatalf("E1 new rows = %v", got)
	}
	if len(eventProp[0].Old) != 0 {
		t.Fatalf("E1 old rows = %v", eventProp[0].Old)
	}
	if got := eventProp[1].New; len(got) != 1 || got[0].Fields["val0"] != "E2" {
		t.Fatalf("E2 new rows = %v", got)
	}
	if got := eventProp[1].Old; len(got) != 1 || got[0].Fields["val0"] != "E1" {
		t.Fatalf("E2 old rows = %v", got)
	}
	if got := eventProp[2].New; len(got) != 2 ||
		got[0].Fields["val0"] != "E3" || got[1].Fields["val0"] != "E4" {
		t.Fatalf("E4 new rows = %v", got)
	}
	if got := eventProp[2].Old; len(got) != 1 || got[0].Fields["val0"] != "E2" {
		t.Fatalf("E4 old rows = %v", got)
	}
}

// TestRunViewExprBatchCore570PinnedArtifacts pins the scenario file's Java
// identity plus the step sequence the loader accepts.
func TestRunViewExprBatchCore570PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprBatchCore570ScenarioForTest(t)
	if scenario.ID != viewExprBatchCore570ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 61 {
		t.Fatalf("steps = %d, want 61", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "newest-oldest" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var deploys []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "deploy" {
			deploys = append(deploys, step)
		}
	}
	if len(deploys) != 5 {
		t.Fatalf("deploys = %d, want 5", len(deploys))
	}
	if deploys[0].Epl != viewExprBatchCore570EPLNewestOldestExclude ||
		deploys[1].Epl != viewExprBatchCore570EPLNewestOldestInclude ||
		deploys[2].Epl != viewExprBatchCore570EPLLengthBatch ||
		deploys[3].Epl != viewExprBatchCore570EPLTimeBatch ||
		deploys[4].Epl != viewExprBatchCore570EPLEventProp {
		t.Fatalf("deploy EPLs are not pinned")
	}
}

// TestRunViewExprBatchCore570RejectsMalformedRawScenario verifies the
// loader pins the step sequence: a mutated EPL, mutated send payload or
// mutated advance-time instant is rejected.
func TestRunViewExprBatchCore570RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprBatchCore570ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"statement": "s0", "epl": "@name('s0') select irstream * from SupportBean#expr_batch(current_count >= 3, true)"`),
				[]byte(`"statement": "s0", "epl": "@name('s0') select irstream * from SupportBean#expr_batch(current_count >= 4, true)"`), 1)
		}},
		{"include flag", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"epl": "@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, false)"`),
				[]byte(`"epl": "@name('s0') select irstream * from SupportBean#expr_batch(newest_event.intPrimitive != oldest_event.intPrimitive, true)"`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"theString": "E9", "intPrimitive": 9`), []byte(`"theString": "E9", "intPrimitive": 8`), 1)
		}},
		{"negative send", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"theString": "E3", "intPrimitive": -1`), []byte(`"theString": "E3", "intPrimitive": 1`), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"at": "1970-01-01T00:00:03.100Z"`), []byte(`"at": "1970-01-01T00:00:03.200Z"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprBatchCore570Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprBatchCore570RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprBatchCore570RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprBatchCore570CaseSpecs) != len(viewExprBatchCore570JavaRuntimeIDs) ||
		len(viewExprBatchCore570CaseSpecs) != len(viewExprBatchCore570JavaExecutions) ||
		len(viewExprBatchCore570CaseSpecs) != len(viewExprBatchCore570JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprBatchCore570CaseSpecs))
	}
	for index, spec := range viewExprBatchCore570CaseSpecs {
		if spec.runtimeID != viewExprBatchCore570JavaRuntimeIDs[index] ||
			spec.execution != viewExprBatchCore570JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprBatchCore570JavaStaticIDs {
		if id != "java-20551a17cb2af08c67fc" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func viewExprBatchCore570ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-batch-core-570.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprBatchCore570ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprBatchCore570ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprBatchCore570Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
