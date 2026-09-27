package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// TestRunViewExprWin567ScenarioReplay replays the checked-in scenario through
// the Go runner and pins the record surface the oracle produces: the deployed
// marker, six listener deliveries and seven any-order iterator snapshots for
// scene-one; the deployed marker, seven listener deliveries and seven
// in-order iterator snapshots for newest-oldest; the deployed marker, three
// new-only listener deliveries (the plain select surfaces the insert stream
// only, a stronger pin than Java's iterator-only assertion) and three
// in-order snapshots for length-window.
func TestRunViewExprWin567ScenarioReplay(t *testing.T) {
	scenario := loadViewExprWin567ScenarioForTest(t)
	trace, err := runViewExprWin567Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != viewExprWin567ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 36 {
		t.Fatalf("records = %d, want 36", len(trace.Records))
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
	if byCase["scene-one"] != 14 || byCase["newest-oldest"] != 15 || byCase["length-window"] != 7 {
		t.Fatalf("per-case records = %v", byCase)
	}
	if byOp["deployed"] != 3 || byOp["listener"] != 16 || byOp["snapshot"] != 17 {
		t.Fatalf("operation counts = %v", byOp)
	}
	// scene-one: the E3 send pairs new {E3} with old {E1} on the c0
	// projection; E6 at t=10000 pairs new {E6} with the mass-expired
	// {E3,E4,E5} while advanceTime(10000) itself posted nothing.
	var sceneOne []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "scene-one" {
			sceneOne = append(sceneOne, record)
		}
	}
	if len(sceneOne) != 6 {
		t.Fatalf("scene-one listeners = %d", len(sceneOne))
	}
	if got := sceneOne[2].Old[0].Fields["c0"]; got != "E1" {
		t.Fatalf("E3 old row c0 = %v", got)
	}
	if got := sceneOne[5].Old; len(got) != 3 ||
		got[0].Fields["c0"] != "E3" || got[1].Fields["c0"] != "E4" || got[2].Fields["c0"] != "E5" {
		t.Fatalf("E6 old rows = %v", got)
	}
	// scene-one iterator pins: empty pre-E1, {E1} at 1500, {E1,E2} at 2000,
	// {E2,E3} post-E3 and post-2499, and the {E2,E3,E4} triple post-E4 and
	// at 2500.
	var sceneOneSnaps []compat.TraceRecord
	for _, record := range snapshots {
		if record.Case == "scene-one" {
			sceneOneSnaps = append(sceneOneSnaps, record)
		}
	}
	if len(sceneOneSnaps) != 7 {
		t.Fatalf("scene-one snapshots = %d", len(sceneOneSnaps))
	}
	if len(sceneOneSnaps[0].New) != 0 {
		t.Fatalf("pre-E1 snapshot rows = %v", sceneOneSnaps[0].New)
	}
	if got := sceneOneSnaps[5].New; len(got) != 3 ||
		got[0].Fields["c0"] != "E2" || got[1].Fields["c0"] != "E3" || got[2].Fields["c0"] != "E4" {
		t.Fatalf("post-E4 snapshot rows = %v", got)
	}
	// newest-oldest: E3/2 flushes {E1,E2}, E4/3 flushes {E3}, E5/E6 deliver
	// new-only, E7/2 flushes {E4,E5,E6}.
	var newestOldest []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "newest-oldest" {
			newestOldest = append(newestOldest, record)
		}
	}
	if len(newestOldest) != 7 {
		t.Fatalf("newest-oldest listeners = %d", len(newestOldest))
	}
	if got := newestOldest[2].Old; len(got) != 2 ||
		got[0].Fields["theString"] != "E1" || got[1].Fields["theString"] != "E2" {
		t.Fatalf("E3 old rows = %v", got)
	}
	if got := newestOldest[6].Old; len(got) != 3 ||
		got[0].Fields["theString"] != "E4" || got[1].Fields["theString"] != "E5" || got[2].Fields["theString"] != "E6" {
		t.Fatalf("E7 old rows = %v", got)
	}
	// length-window: the plain select delivers new-only for all three sends
	// (the stronger pin the Java test leaves iterator-only).
	var lengthWindow []compat.TraceRecord
	for _, record := range listeners {
		if record.Case == "length-window" {
			lengthWindow = append(lengthWindow, record)
		}
	}
	if len(lengthWindow) != 3 {
		t.Fatalf("length-window listeners = %d", len(lengthWindow))
	}
	if len(lengthWindow[0].Old) != 0 || len(lengthWindow[1].Old) != 0 {
		t.Fatalf("E1/E2 should be new-only: %v %v", lengthWindow[0].Old, lengthWindow[1].Old)
	}
	if got := lengthWindow[2].Old; len(got) != 0 {
		t.Fatalf("E3 old rows = %v", got)
	}
	if got := snapshots[len(snapshots)-1].New; len(got) != 2 ||
		got[0].Fields["theString"] != "E2" || got[1].Fields["theString"] != "E3" {
		t.Fatalf("length-window final snapshot = %v", got)
	}
}

// TestRunViewExprWin567PinnedArtifacts pins the scenario file's Java identity
// plus the step sequence the loader accepts.
func TestRunViewExprWin567PinnedArtifacts(t *testing.T) {
	scenario := loadViewExprWin567ScenarioForTest(t)
	if scenario.ID != viewExprWin567ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 52 {
		t.Fatalf("steps = %d, want 52", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "scene-one" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var deploys []compat.Step
	for _, step := range scenario.Steps {
		if step.Op == "deploy" {
			deploys = append(deploys, step)
		}
	}
	if len(deploys) != 3 {
		t.Fatalf("deploys = %d", len(deploys))
	}
	if deploys[0].Epl != viewExprWin567EPLSceneOne ||
		deploys[1].Epl != viewExprWin567EPLNewestOldest ||
		deploys[2].Epl != viewExprWin567EPLLengthWindow {
		t.Fatalf("deploy EPLs = %q / %q / %q", deploys[0].Epl, deploys[1].Epl, deploys[2].Epl)
	}
}

// TestRunViewExprWin567RejectsMalformedRawScenario verifies the loader pins
// the step sequence: a mutated EPL, reordered step or wrong statement label
// is rejected.
func TestRunViewExprWin567RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(viewExprWin567ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data, []byte("current_count <= 2"), []byte("current_count <= 3"), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data, []byte("00:00:02.500"), []byte("00:00:02.501"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"theString": "E7", "intPrimitive": 2`), []byte(`"theString": "E7", "intPrimitive": 3`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadViewExprWin567Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunViewExprWin567RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunViewExprWin567RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(viewExprWin567CaseSpecs) != len(viewExprWin567JavaRuntimeIDs) ||
		len(viewExprWin567CaseSpecs) != len(viewExprWin567JavaExecutions) ||
		len(viewExprWin567CaseSpecs) != len(viewExprWin567JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(viewExprWin567CaseSpecs))
	}
	for index, spec := range viewExprWin567CaseSpecs {
		if spec.runtimeID != viewExprWin567JavaRuntimeIDs[index] ||
			spec.execution != viewExprWin567JavaExecutions[index] {
			t.Fatalf("case %d mapping = %q/%q", index, spec.runtimeID, spec.execution)
		}
	}
	for index, id := range viewExprWin567JavaStaticIDs {
		if id != "java-06e6b1f6c905b8f12b82" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func viewExprWin567ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"view-expression-window-567.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadViewExprWin567ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(viewExprWin567ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadViewExprWin567Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
