package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunInfraNWTableCreateDDL563ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: one deployed module marker plus a types assert and an
// iterator snapshot per create-DDL leg, four SODA plan-only records, one
// window deployed marker and five invalid plan-only records for the
// advanced-syntax case.
func TestRunInfraNWTableCreateDDL563ScenarioReplay(t *testing.T) {
	scenario := loadInfraNWTableCreateDDL563ScenarioForTest(t)
	trace, err := runInfraNWTableCreateDDL563Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion || trace.ID != infraNWTableCreateDDL563ID {
		t.Fatalf("trace identity = %v/%v", trace.Version, trace.ID)
	}
	if len(trace.Records) != 16 {
		t.Fatalf("records = %d, want 16", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var typesValues []any
	var snapshotRows [][]compat.ResultRecord
	for _, record := range trace.Records {
		byCase[record.Case]++
		byOp[record.Operation]++
		switch record.Operation {
		case "types":
			typesValues = append(typesValues, record.Value)
		case "snapshot":
			snapshotRows = append(snapshotRows, record.New)
		}
	}
	wantCase := map[string]int{"generic-col-window": 3, "generic-col-table": 3, "index-syntax": 10}
	if len(byCase) != 3 {
		t.Fatalf("cases = %v", byCase)
	}
	for name, want := range wantCase {
		if byCase[name] != want {
			t.Fatalf("case %q records = %d, want %d", name, byCase[name], want)
		}
	}
	wantOp := map[string]int{"deployed": 3, "types": 2, "snapshot": 2, "unrepresentable": 9}
	for op, want := range wantOp {
		if byOp[op] != want {
			t.Fatalf("op %q records = %d, want %d", op, byOp[op], want)
		}
	}
	// The types records pin all eight SupportGenericColUtil columns with
	// the canonical Java type strings and the descriptor flags.
	for _, value := range typesValues {
		entries, ok := value.([]map[string]any)
		if !ok || len(entries) != 8 {
			t.Fatalf("types value = %#v", value)
		}
		if entries[0]["name"] != "listOfString" || entries[0]["type"] != "java.util.List<String>" ||
			entries[0]["indexed"] != true || entries[0]["mapped"] != false {
			t.Fatalf("types[0] = %#v", entries[0])
		}
		if entries[1]["type"] != "java.util.List<Optional<Integer>>" {
			t.Fatalf("types[1] = %#v", entries[1])
		}
		if entries[2]["mapped"] != true || entries[2]["indexed"] != false {
			t.Fatalf("map column flags = %#v", entries[2])
		}
	}
	// Each iterator snapshot carries the single merged row with all eight
	// generic columns materialized.
	for _, rows := range snapshotRows {
		if len(rows) != 1 {
			t.Fatalf("snapshot rows = %d, want 1", len(rows))
		}
		for _, name := range infraNWTableCreateDDL563SnapshotFields {
			if _, ok := rows[0].Fields[name]; !ok {
				t.Fatalf("snapshot row missing %q: %#v", name, rows[0].Fields)
			}
		}
	}
}

// TestRunInfraNWTableCreateDDL563PinnedArtifacts pins the scenario file's
// Java identity plus the step sequence the loader accepts.
func TestRunInfraNWTableCreateDDL563PinnedArtifacts(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableCreateDDL563ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, loadErr := loadInfraNWTableCreateDDL563Scenario(scenarioFile)
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
	for name, want := range map[string]any{
		"javaRuntimes":  infraNWTableCreateDDL563JavaRuntimeIDs,
		"javaNames":     infraNWTableCreateDDL563JavaExecutions,
		"javaStaticIds": infraNWTableCreateDDL563JavaStaticIDs,
		"javaFlags":     infraNWTableCreateDDL563JavaFlags,
	} {
		var raw []string
		buf, _ := json.Marshal(mutated[name])
		_ = json.Unmarshal(buf, &raw)
		if len(raw) != len(want.([]string)) {
			t.Fatalf("%v length = %d, want %d", name, len(raw), len(want.([]string)))
		}
		for index := range raw {
			if raw[index] != want.([]string)[index] {
				t.Fatalf("%v[%d] = %q, want %q", name, index, raw[index], want.([]string)[index])
			}
		}
	}
	if len(scenario.Steps) != 27 {
		t.Fatalf("steps = %d, want 27", len(scenario.Steps))
	}
}

// TestRunInfraNWTableCreateDDL563RejectsMalformedRawScenario verifies the
// loader pins the step sequence: a mutated EPL, reordered step or wrong
// probe label is rejected.
func TestRunInfraNWTableCreateDDL563RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableCreateDDL563ID+".json")
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"epl-drift", func(m map[string]any) {
			steps := m["steps"].([]any)
			steps[1].(map[string]any)["epl"] = "create schema MyInputEvent(x int)"
		}},
		{"wrong-case", func(m map[string]any) {
			steps := m["steps"].([]any)
			steps[2].(map[string]any)["statement"] = "bogus"
		}},
		{"trailing-step", func(m map[string]any) {
			steps := m["steps"].([]any)
			m["steps"] = append(steps, map[string]any{"op": "undeploy-all", "case": "generic-col-window"})
		}},
		{"unrepresentable-note-drift", func(m map[string]any) {
			steps := m["steps"].([]any)
			steps[24].(map[string]any)["expectError"] = "bogus note"
		}},
	} {
		var mutated map[string]any
		if err := json.Unmarshal(data, &mutated); err != nil {
			t.Fatal(err)
		}
		mutation.mutate(mutated)
		buf, err := json.Marshal(mutated)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loadInfraNWTableCreateDDL563Scenario(bytes.NewReader(buf)); err == nil {
			t.Fatalf("malformed scenario %q accepted", mutation.name)
		}
	}
}

// TestRunInfraNWTableCreateDDL563RuntimeIDMappingMatchesScenario pins the
// case→runtime/execution/ordinal mapping the -diff path reports.
func TestRunInfraNWTableCreateDDL563RuntimeIDMappingMatchesScenario(t *testing.T) {
	scenario := loadInfraNWTableCreateDDL563ScenarioForTest(t)
	if len(infraNWTableCreateDDL563Cases) != len(infraNWTableCreateDDL563JavaRuntimeIDs) ||
		len(infraNWTableCreateDDL563Cases) != len(infraNWTableCreateDDL563JavaExecutions) ||
		len(infraNWTableCreateDDL563Cases) != len(infraNWTableCreateDDL563JavaStaticIDs) {
		t.Fatal("case/runtime/execution/static-id tables have divergent lengths")
	}
	cases := map[string]int{}
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			cases[step.Case]++
		}
	}
	if len(cases) != len(infraNWTableCreateDDL563Cases) {
		t.Fatalf("scenario carries %d cases, want %d", len(cases), len(infraNWTableCreateDDL563Cases))
	}
	for _, name := range infraNWTableCreateDDL563Cases {
		if cases[name] != 1 {
			t.Fatalf("case %q markers = %d", name, cases[name])
		}
	}
	if !strings.HasPrefix(infraNWTableCreateDDL563JavaRuntimeIDs[0], "java-runtime-") {
		t.Fatal("runtime id table is not pinned")
	}
}

func loadInfraNWTableCreateDDL563ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioFile, err := os.Open(filepath.Join(root, infraNWTableCreateDDL563ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := scenarioFile.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	scenario, err := loadInfraNWTableCreateDDL563Scenario(scenarioFile)
	if err != nil {
		t.Fatalf("checked-in scenario rejected: %v", err)
	}
	return scenario
}
