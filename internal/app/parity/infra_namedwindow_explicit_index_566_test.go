package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunInfraNWIdx566ScenarioReplay replays the checked-in scenario through
// the Go runner and pins the record surface the oracle produces: the
// deployed markers plus idx-props unrepresentable record and any-order
// window snapshot for named-window-index; the deployed markers, s0/s1/s2
// listener deliveries, index-count pins, getter unrepresentable records and
// the two phase-end 101-row snapshots for late-start-index.
func TestRunInfraNWIdx566ScenarioReplay(t *testing.T) {
	scenario := loadInfraNWIdx566ScenarioForTest(t)
	trace, err := runInfraNWIdx566Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != infraNWIdx566ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 33 {
		t.Fatalf("records = %d, want 33", len(trace.Records))
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
	if byCase["named-window-index"] != 5 || byCase["late-start-index"] != 28 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 12 || byOp["unrepresentable"] != 9 ||
		byOp["index-count"] != 5 || byOp["snapshot"] != 3 || byOp["listener"] != 4 {
		t.Fatalf("operation counts = %v", byOp)
	}
	// named-window-index snapshot: unique last-wins dedup over theString.
	if len(snapshots[0].New) != 3 {
		t.Fatalf("named-window-index snapshot rows = %d", len(snapshots[0].New))
	}
	// s0 fires once per S0 send while deployed (both phase-A sends).
	if listeners[0].Statement != "s0" || listeners[1].Statement != "s0" ||
		listeners[2].Statement != "s1" || listeners[3].Statement != "s2" {
		t.Fatalf("listener statements = %v", listeners)
	}
	joinFields := listeners[0].New[0].Fields
	aw, ok := joinFields["aw"].(map[string]any)
	if !ok {
		t.Fatalf("s0 row aw fragment = %T", joinFields["aw"])
	}
	awFields, ok := aw["fields"].(map[string]any)
	if !ok || awFields["id"] != -1 || awFields["p00"] != "x" {
		t.Fatalf("s0 aw row = %v", aw)
	}
	if scalar := listeners[2].New[0].Fields["id"]; scalar != -1 {
		t.Fatalf("s1 scalar id = %v", scalar)
	}
}

// TestRunInfraNWIdx566PinnedArtifacts pins the scenario file's Java identity
// plus the step sequence the loader accepts.
func TestRunInfraNWIdx566PinnedArtifacts(t *testing.T) {
	scenario := loadInfraNWIdx566ScenarioForTest(t)
	if scenario.ID != infraNWIdx566ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 256 {
		t.Fatalf("steps = %d, want 256", len(scenario.Steps))
	}
	cases := map[string]int{}
	for _, step := range scenario.Steps {
		cases[step.Case]++
	}
	if cases["named-window-index"] != 15 || cases["late-start-index"] != 241 {
		t.Fatalf("steps per case = %v", cases)
	}
}

// TestRunInfraNWIdx566RejectsMalformedRawScenario verifies the loader pins
// the step sequence: a mutated EPL, reordered step or wrong statement label
// is rejected.
func TestRunInfraNWIdx566RejectsMalformedRawScenario(t *testing.T) {
	scenario := loadInfraNWIdx566ScenarioForTest(t)
	mutated := append([]compat.Step(nil), scenario.Steps...)
	mutated[1].Epl = "@name('window') create window MyWindowOne#keepall as SupportBean"
	strict := compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: mutated}
	if err := validateInfraNWIdx566Scenario(strict); err == nil {
		t.Fatal("mutated window EPL was accepted")
	}
	mutated[1] = scenario.Steps[1]
	mutated[4].Statement = "idx"
	if err := validateInfraNWIdx566Scenario(compat.Scenario{
		Version: scenario.Version, ID: scenario.ID, Steps: mutated,
	}); err == nil {
		t.Fatal("mutated deployed label was accepted")
	}
}

// TestRunInfraNWIdx566RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunInfraNWIdx566RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(infraNWIdx566CaseSpecs) != len(infraNWIdx566JavaRuntimeIDs) ||
		len(infraNWIdx566CaseSpecs) != len(infraNWIdx566JavaExecutions) ||
		len(infraNWIdx566CaseSpecs) != len(infraNWIdx566JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(infraNWIdx566CaseSpecs))
	}
	for index, spec := range infraNWIdx566CaseSpecs {
		if spec.runtimeID != infraNWIdx566JavaRuntimeIDs[index] ||
			spec.execution != infraNWIdx566JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	if infraNWIdx566JavaStaticIDs[0] != "java-152a3c2771c531a8841c" ||
		infraNWIdx566JavaStaticIDs[1] != "java-a81367bdfb722afec857" {
		t.Fatalf("static ids = %v", infraNWIdx566JavaStaticIDs)
	}
}

func infraNWIdx566ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"infra-namedwindow-explicit-index-566.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadInfraNWIdx566ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(infraNWIdx566ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadInfraNWIdx566Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
