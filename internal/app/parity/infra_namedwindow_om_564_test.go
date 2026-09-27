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

// TestRunInfraNWOM564ScenarioReplay replays the checked-in scenario through
// the Go runner and pins the record surface the oracle produces: five
// deployed markers and three toEPL pins plus listener deliveries for the
// compile case, five deployed markers and four toEPL pins plus listener
// deliveries for the om case, and the single toEPL pin for
// create-table-syntax.
func TestRunInfraNWOM564ScenarioReplay(t *testing.T) {
	scenario := loadInfraNWOM564ScenarioForTest(t)
	trace, err := runInfraNWOM564Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion || trace.ID != infraNWOM564ID {
		t.Fatalf("trace identity = %v/%v", trace.Version, trace.ID)
	}
	if len(trace.Records) != 62 {
		t.Fatalf("records = %d, want 62", len(trace.Records))
	}
	byCase := map[string]int{}
	byOp := map[string]int{}
	var compileSelectNew, compileSelectOld []compat.ResultRecord
	var omSelectNew []compat.ResultRecord
	var compileOnSelectRows [][]compat.ResultRecord
	var omOnSelectRows [][]compat.ResultRecord
	for _, record := range trace.Records {
		byCase[record.Case]++
		byOp[record.Operation]++
		switch {
		case record.Case == "compile" && record.Operation == "listener" && record.Statement == "select":
			compileSelectNew = append(compileSelectNew, record.New...)
			compileSelectOld = append(compileSelectOld, record.Old...)
		case record.Case == "om" && record.Operation == "listener" && record.Statement == "select":
			omSelectNew = append(omSelectNew, record.New...)
		case record.Case == "compile" && record.Operation == "listener" && record.Statement == "onselect":
			compileOnSelectRows = append(compileOnSelectRows, record.New)
		case record.Case == "om" && record.Operation == "listener" && record.Statement == "onselect":
			omOnSelectRows = append(omOnSelectRows, record.New)
		}
	}
	wantCase := map[string]int{"compile": 32, "om": 29, "create-table-syntax": 1}
	if len(byCase) != len(wantCase) {
		t.Fatalf("cases = %v", byCase)
	}
	for name, want := range wantCase {
		if byCase[name] != want {
			t.Fatalf("case %q records = %d, want %d", name, byCase[name], want)
		}
	}
	wantOp := map[string]int{"deployed": 10, "unrepresentable": 8, "listener": 44}
	for op, want := range wantOp {
		if byOp[op] != want {
			t.Fatalf("operation %q records = %d, want %d", op, byOp[op], want)
		}
	}
	// The compile select consumer emits irstream rows: new E1/E2/E3 and the
	// null-value boundary bean (value projected as null), old E1/E2 and BND;
	// the null-key bean is filtered out.
	if len(compileSelectNew) != 4 || len(compileSelectOld) != 3 {
		t.Fatalf("compile select deliveries = %d new / %d old", len(compileSelectNew), len(compileSelectOld))
	}
	// The om select consumer filters on value: the null-key bean emits
	// {null,198} while the null-value bean stays silent.
	if len(omSelectNew) != 5 {
		t.Fatalf("om select new deliveries = %d, want 5", len(omSelectNew))
	}
	// The unconditional compile on-select fires exactly once (B2 over the
	// single retained E3 row); the correlated om on-select fires once for
	// the B(id=E3) trigger.
	if len(compileOnSelectRows) != 1 || len(omOnSelectRows) != 1 {
		t.Fatalf("onselect deliveries = %d compile / %d om",
			len(compileOnSelectRows), len(omOnSelectRows))
	}
}

// TestRunInfraNWOM564PinnedArtifacts pins the scenario file's Java identity
// plus the step sequence the loader accepts.
func TestRunInfraNWOM564PinnedArtifacts(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWOM564ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	scenario, loadErr := loadInfraNWOM564Scenario(scenarioFile)
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
		"javaRuntimes":  infraNWOM564JavaRuntimeIDs,
		"javaNames":     infraNWOM564JavaExecutions,
		"javaStaticIds": infraNWOM564JavaStaticIDs,
		"javaFlags":     infraNWOM564JavaFlags,
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
	if len(scenario.Steps) != 63 {
		t.Fatalf("steps = %d, want 63", len(scenario.Steps))
	}
}

// TestRunInfraNWOM564RejectsMalformedRawScenario verifies the loader pins
// the step sequence: a mutated EPL, reordered step or wrong probe label is
// rejected.
func TestRunInfraNWOM564RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWOM564ID+".json")
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
			steps[1].(map[string]any)["epl"] = "create window MyWindow#keepall as (key string)"
		}},
		{"wrong-case", func(m map[string]any) {
			steps := m["steps"].([]any)
			steps[2].(map[string]any)["statement"] = "bogus"
		}},
		{"trailing-step", func(m map[string]any) {
			steps := m["steps"].([]any)
			m["steps"] = append(steps, map[string]any{"op": "undeploy-all", "case": "compile"})
		}},
		{"unrepresentable-note-drift", func(m map[string]any) {
			steps := m["steps"].([]any)
			steps[3].(map[string]any)["expectError"] = "bogus note"
		}},
		{"send-payload-drift", func(m map[string]any) {
			steps := m["steps"].([]any)
			steps[10].(map[string]any)["payload"] = map[string]any{"theString": "E9", "longBoxed": 9}
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
		if _, err := loadInfraNWOM564Scenario(bytes.NewReader(buf)); err == nil {
			t.Fatalf("malformed scenario %q accepted", mutation.name)
		}
	}
}

// TestRunInfraNWOM564RuntimeIDMappingMatchesScenario pins the
// case→runtime/execution/ordinal mapping the -diff path reports.
func TestRunInfraNWOM564RuntimeIDMappingMatchesScenario(t *testing.T) {
	scenario := loadInfraNWOM564ScenarioForTest(t)
	if len(infraNWOM564Cases) != len(infraNWOM564JavaRuntimeIDs) ||
		len(infraNWOM564Cases) != len(infraNWOM564JavaExecutions) ||
		len(infraNWOM564Cases) != len(infraNWOM564JavaStaticIDs) {
		t.Fatal("case/runtime/execution/static-id tables have divergent lengths")
	}
	cases := map[string]int{}
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			cases[step.Case]++
		}
	}
	if len(cases) != len(infraNWOM564Cases) {
		t.Fatalf("scenario carries %d cases, want %d", len(cases), len(infraNWOM564Cases))
	}
	for _, name := range infraNWOM564Cases {
		if cases[name] != 1 {
			t.Fatalf("case %q markers = %d", name, cases[name])
		}
	}
	if !strings.HasPrefix(infraNWOM564JavaRuntimeIDs[0], "java-runtime-") {
		t.Fatal("runtime id table is not pinned")
	}
}

func loadInfraNWOM564ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioFile, err := os.Open(filepath.Join(root, infraNWOM564ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := scenarioFile.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	scenario, err := loadInfraNWOM564Scenario(scenarioFile)
	if err != nil {
		t.Fatalf("checked-in scenario rejected: %v", err)
	}
	return scenario
}
