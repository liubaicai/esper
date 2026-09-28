package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunViewExprUDFPrev572ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the
// oracle produces: one deployed marker, two flush deliveries and two
// udf observations for udf-batch; one deployed marker plus the single
// 3-row flush for prev-batch; one deployed marker, three deliveries
// (two retained keeps plus the mass-expiry delivery) and two udf
// observations for udf-window; one deployed marker plus two per-row
// deliveries for prev-window.
func TestRunViewExprUDFPrev572ScenarioReplay(t *testing.T) {
	scenario := loadViewExprUDFPrev572ScenarioForTest(t)
	trace, err := runViewExprUDFPrev572Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprUDFPrev572ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 16 {
		t.Fatalf("records = %d, want 16", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var listeners []compat.TraceRecord
	var observations []compat.TraceRecord
	for _, record := range trace.Records {
		byCase[record.Case]++
		byOp[record.Operation]++
		switch record.Operation {
		case "listener":
			listeners = append(listeners, record)
		case "observation":
			observations = append(observations, record)
		}
	}
	if byCase["udf-batch"] != 5 || byCase["prev-batch"] != 2 ||
		byCase["udf-window"] != 6 || byCase["prev-window"] != 3 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 4 || byOp["listener"] != 8 || byOp["observation"] != 4 {
		t.Fatalf("operation counts = %v", byOp)
	}

	// udf-batch: result=true flushes each single-row batch; plain
	// `select *` is istream only, so the prior batch's rstream row
	// never reaches the listener (E2's delivery is new-only). The
	// batch discriminant is expiryCount 0 at both observations —
	// expr_batch evaluates the arriving event only.
	var udfBatch []compat.TraceRecord
	var udfBatchObs []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "udf-batch" {
			udfBatch = append(udfBatch, record)
		}
	}
	for _, record := range observations {
		if record.Case == "udf-batch" {
			udfBatchObs = append(udfBatchObs, record)
		}
	}
	if len(udfBatch) != 2 {
		t.Fatalf("udf-batch listeners = %d", len(udfBatch))
	}
	if got := udfBatch[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E1 flush new rows = %v", got)
	}
	if len(udfBatch[0].Old) != 0 {
		t.Fatalf("E1 flush old rows = %v", udfBatch[0].Old)
	}
	if got := udfBatch[1].New; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 flush new rows = %v", got)
	}
	if len(udfBatch[1].Old) != 0 {
		t.Fatalf("E2 flush old rows = %v", udfBatch[1].Old)
	}
	if len(udfBatchObs) != 2 {
		t.Fatalf("udf-batch observations = %d", len(udfBatchObs))
	}
	first := udfBatchObs[0].Value.(map[string]any)
	if first["key"] != "E1" || first["expiryCount"] != int64(0) || first["viewref"] != true {
		t.Fatalf("first udf-batch observation = %v", first)
	}
	second := udfBatchObs[1].Value.(map[string]any)
	if second["key"] != "E3" || second["expiryCount"] != int64(0) || second["viewref"] != true {
		t.Fatalf("second udf-batch observation = %v", second)
	}
	// prev-batch: E1/E2 stay silent and E3 flushes the whole batch in
	// one delivery; prev evaluates per output row inside the flush.
	var prevBatch []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "prev-batch" {
			prevBatch = append(prevBatch, record)
		}
	}
	if len(prevBatch) != 1 {
		t.Fatalf("prev-batch listeners = %d", len(prevBatch))
	}
	if got := prevBatch[0].New; len(got) != 3 ||
		!reflect.DeepEqual(got[0].Fields["val0"], map[string]any{"state": "null"}) ||
		got[1].Fields["val0"] != "E1" || got[2].Fields["val0"] != "E2" {
		t.Fatalf("prev-batch flush rows = %v", got)
	}
	if len(prevBatch[0].Old) != 0 {
		t.Fatalf("prev-batch flush old rows = %v", prevBatch[0].Old)
	}
	// udf-window: E1/E2 are retained as new-only deliveries; the
	// result=false flip at E3 expires both retained rows and then the
	// arriving row itself — the mass-expiry rstream never surfaces on
	// the istream statement, so the delivery is new-only while the
	// observation reads expiryCount 2 (the window discriminant
	// against the batch case's 0).
	var udfWindow []compat.TraceRecord
	var udfWindowObs []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "udf-window" {
			udfWindow = append(udfWindow, record)
		}
	}
	for _, record := range observations {
		if record.Case == "udf-window" {
			udfWindowObs = append(udfWindowObs, record)
		}
	}
	if len(udfWindow) != 3 {
		t.Fatalf("udf-window listeners = %d", len(udfWindow))
	}
	if got := udfWindow[0].New; len(got) != 1 || got[0].Fields["theString"] != "E1" {
		t.Fatalf("E1 delivery new rows = %v", got)
	}
	if len(udfWindow[0].Old) != 0 || len(udfWindow[1].Old) != 0 {
		t.Fatalf("retained deliveries old rows = %v / %v", udfWindow[0].Old, udfWindow[1].Old)
	}
	if got := udfWindow[1].New; len(got) != 1 || got[0].Fields["theString"] != "E2" {
		t.Fatalf("E2 delivery new rows = %v", got)
	}
	if got := udfWindow[2].New; len(got) != 1 || got[0].Fields["theString"] != "E3" {
		t.Fatalf("expiry delivery new rows = %v", got)
	}
	if len(udfWindow[2].Old) != 0 {
		t.Fatalf("expiry delivery old rows = %v", udfWindow[2].Old)
	}
	if len(udfWindowObs) != 2 {
		t.Fatalf("udf-window observations = %d", len(udfWindowObs))
	}
	firstWindow := udfWindowObs[0].Value.(map[string]any)
	if firstWindow["key"] != "E1" || firstWindow["expiryCount"] != int64(0) || firstWindow["viewref"] != true {
		t.Fatalf("first udf-window observation = %v", firstWindow)
	}
	secondWindow := udfWindowObs[1].Value.(map[string]any)
	if secondWindow["key"] != "E3" || secondWindow["expiryCount"] != int64(2) || secondWindow["viewref"] != true {
		t.Fatalf("second udf-window observation = %v", secondWindow)
	}
	// prev-window: per-row deliveries under the never-expiring
	// expr(true) keep; prev reads the prior retained row.
	var prevWindow []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "prev-window" {
			prevWindow = append(prevWindow, record)
		}
	}
	if len(prevWindow) != 2 {
		t.Fatalf("prev-window listeners = %d", len(prevWindow))
	}
	if got := prevWindow[0].New; len(got) != 1 ||
		!reflect.DeepEqual(got[0].Fields["val0"], map[string]any{"state": "null"}) {
		t.Fatalf("prev-window E1 row = %v", got)
	}
	if got := prevWindow[1].New; len(got) != 1 || got[0].Fields["val0"] != "E1" {
		t.Fatalf("prev-window E2 row = %v", got)
	}
	if len(prevWindow[0].Old) != 0 || len(prevWindow[1].Old) != 0 {
		t.Fatalf("prev-window old rows = %v / %v", prevWindow[0].Old, prevWindow[1].Old)
	}
}

// TestRunViewExprUDFPrev572PinnedArtifacts pins the scenario file's
// Java identity plus the step sequence the loader accepts.
func TestRunViewExprUDFPrev572PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprUDFPrev572ScenarioForTest(t)
	if scenario.ID != viewExprUDFPrev572ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 35 {
		t.Fatalf("steps = %d, want 35", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "udf-batch" {
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
	if deploys[0].Epl != viewExprUDFPrev572EPLUDFBatch ||
		deploys[1].Epl != viewExprUDFPrev572EPLPrevBatch ||
		deploys[2].Epl != viewExprUDFPrev572EPLUDFWindow ||
		deploys[3].Epl != viewExprUDFPrev572EPLPrevWindow {
		t.Fatalf("deploy EPLs are not pinned")
	}
	var toggles, udfSnapshots int
	for _, step := range scenario.Steps {
		if step.Op == "set-variable" {
			toggles++
			if step.Name != viewExprUDFPrev572UDFResultName {
				t.Fatalf("set-variable name = %q", step.Name)
			}
		}
		if step.Op == "snapshot" {
			udfSnapshots++
			if step.Mode != viewExprUDFPrev572SnapshotUDF {
				t.Fatalf("snapshot mode = %q", step.Mode)
			}
		}
	}
	if toggles != 4 || udfSnapshots != 4 {
		t.Fatalf("udf toggles/snapshots = %d/%d, want 4/4", toggles, udfSnapshots)
	}
}

// TestRunViewExprUDFPrev572RejectsMalformedRawScenario verifies the
// loader pins the step sequence: a mutated EPL, mutated send payload,
// mutated udf-result toggle or mutated snapshot mode is rejected.
func TestRunViewExprUDFPrev572RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprUDFPrev572ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl udf argument", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`udf(theString, view_reference, expired_count)`),
				[]byte(`udf(theString, view_reference, current_count)`), 1)
		}},
		{"epl trigger", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`#expr_batch(current_count > 2)`),
				[]byte(`#expr_batch(current_count >= 2)`), 1)
		}},
		{"epl keep", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`SupportBean#expr(true)`),
				[]byte(`SupportBean#expr(false)`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "E3", "intPrimitive": 3`),
				[]byte(`"theString": "E3", "intPrimitive": 4`), 1)
		}},
		{"udf-result toggle", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"name": "udf-result", "payload": false`),
				[]byte(`"name": "udf-result", "payload": true`), 1)
		}},
		{"snapshot mode", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"mode": "udf"`),
				[]byte(`"mode": "ordered"`), 1)
		}},
		{"extra send", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"op": "undeploy-all", "case": "prev-batch"}`),
				[]byte(`{"op": "send", "case": "prev-batch", "eventType": "SupportBean", "payload": {"theString": "E4", "intPrimitive": 4}}, {"op": "undeploy-all", "case": "prev-batch"}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprUDFPrev572Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprUDFPrev572RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprUDFPrev572RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprUDFPrev572CaseSpecs) != len(viewExprUDFPrev572JavaRuntimeIDs) ||
		len(viewExprUDFPrev572CaseSpecs) != len(viewExprUDFPrev572JavaExecutions) ||
		len(viewExprUDFPrev572CaseSpecs) != len(viewExprUDFPrev572JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprUDFPrev572CaseSpecs))
	}
	for index, spec := range viewExprUDFPrev572CaseSpecs {
		if spec.runtimeID != viewExprUDFPrev572JavaRuntimeIDs[index] ||
			spec.execution != viewExprUDFPrev572JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprUDFPrev572JavaStaticIDs[:2] {
		if id != "java-20551a17cb2af08c67fc" {
			t.Fatalf("batch static id %d = %q", index, id)
		}
	}
	for index, id := range viewExprUDFPrev572JavaStaticIDs[2:] {
		if id != "java-06e6b1f6c905b8f12b82" {
			t.Fatalf("window static id %d = %q", index, id)
		}
	}
}

func viewExprUDFPrev572ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-udf-prev-572.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprUDFPrev572ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprUDFPrev572ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprUDFPrev572Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
