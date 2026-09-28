package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Siblings excluded from this slice (no scenario steps, no trace):
//   - ord 3 PatternEveryDistinctOverFilter java-runtime-7d52c9e689e3eeca65d7
//     replays the same legs twice, the second variant ending in
//     eplToModelCompileDeploy (SODA/compile-text precedent — the fluent Go
//     API has no eplToModel front end).
//   - ord 14 PatternInvalid java-runtime-2542b81b75a973a3cc38 is
//     compile-error-only (three tryInvalid messages; the typed Go API has
//     no EPL-text compile path to pin the verbatim diagnostics).
//   - ord 15 PatternMonthScoped java-runtime-d211795c3ecad71c297a needs
//     calendar-month expiry (EveryDistinctForCalendar) — a different
//     semantic than the fixed-interval ords 1-2.
// Ords 4-13 and 16 are compound-root/multi-key/special-type variants
// deferred to follow-on slices.

// TestRunPatternEveryDistinct576ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: 12 listener deliveries across the three cases — three c0
// deliveries for the dup-suppression sequence, four for the 5-second
// per-key expiry (including the boundary re-fire at first-seen+5000) and
// five for the 1-second intPrimitive-keyed expiry (select * projecting
// the captured bean).
func TestRunPatternEveryDistinct576ScenarioReplay(t *testing.T) {
	scenario := loadPatternEveryDistinct576ScenarioForTest(t)
	trace, err := runPatternEveryDistinct576Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternEveryDistinct576ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 12 {
		t.Fatalf("records = %d, want 12", len(trace.Records))
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		if record.Operation != "listener" {
			t.Fatalf("unexpected operation %q", record.Operation)
		}
		if record.Statement != "s0" {
			t.Fatalf("unexpected statement %q", record.Statement)
		}
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	simple := byCase["every-distinct-simple"]
	if len(simple) != 3 {
		t.Fatalf("simple deliveries = %d, want 3", len(simple))
	}
	for index, want := range []string{"E1", "E2", "E3"} {
		if len(simple[index].New) != 1 || simple[index].New[0].Fields["c0"] != want {
			t.Fatalf("simple delivery %d = %v, want c0=%s", index, simple[index].New, want)
		}
		if len(simple[index].Old) != 0 {
			t.Fatalf("simple delivery %d should be new-only: %v", index, simple[index].Old)
		}
	}
	wTime := byCase["every-distinct-w-time"]
	if len(wTime) != 4 {
		t.Fatalf("w-time deliveries = %d, want 4", len(wTime))
	}
	// E1@15000, E2@18000, E1@20000 (re-fire exactly at first-seen+5000 —
	// the t=19999 send stays swallowed), E3@20000.
	for index, want := range []string{"E1", "E2", "E1", "E3"} {
		if len(wTime[index].New) != 1 || wTime[index].New[0].Fields["c0"] != want {
			t.Fatalf("w-time delivery %d = %v, want c0=%s", index, wTime[index].New, want)
		}
	}
	// The boundary re-fire carries the t=20000 clock.
	if wTime[2].Time != "1970-01-01T00:00:20Z" {
		t.Fatalf("w-time boundary delivery time = %q", wTime[2].Time)
	}
	expiry := byCase["expire-seen-before-key"]
	if len(expiry) != 5 {
		t.Fatalf("expiry deliveries = %d, want 5", len(expiry))
	}
	// select * projects the captured tag `a` as the bean row; the Java
	// assertions pin a.theString on A1/A3 at t=0, A4/A5 at t=1000 (both
	// keys expired at first-seen+1000) and A7 at t=2000.
	for index, want := range []string{"A1", "A3", "A4", "A5", "A7"} {
		record := expiry[index]
		if len(record.New) != 1 {
			t.Fatalf("expiry delivery %d = %v", index, record.New)
		}
		a, ok := record.New[0].Fields["a"].(map[string]any)
		if !ok {
			t.Fatalf("expiry delivery %d field a is not a row: %v", index, record.New[0].Fields["a"])
		}
		fields, ok := a["fields"].(map[string]any)
		if !ok || fields["theString"] != want {
			t.Fatalf("expiry delivery %d = %v, want a.theString=%s", index, record.New, want)
		}
	}
}

// TestRunPatternEveryDistinct576PinnedArtifacts pins the scenario file's
// Java identity plus the step sequence the loader accepts.
func TestRunPatternEveryDistinct576PinnedArtifacts(t *testing.T) {
	scenario := loadPatternEveryDistinct576ScenarioForTest(t)
	if scenario.ID != patternEveryDistinct576ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 47 {
		t.Fatalf("steps = %d, want 47", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "every-distinct-simple" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, advances, sends, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
		case "advance-time":
			advances++
		case "send":
			sends++
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 3 || deploys != 3 || undeploys != 3 || sends != 29 || advances != 9 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternEveryDistinct576RejectsMalformedRawScenario verifies the
// loader pins the step sequence: a mutated EPL, mutated advance target or
// mutated send payload is rejected.
func TestRunPatternEveryDistinct576RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternEveryDistinct576ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(a.theString, 5 sec)"),
				[]byte("every-distinct(a.theString, 6 sec)"), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data, []byte("00:00:19.999"), []byte("00:00:20.000"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "A7", "intPrimitive": 2`),
				[]byte(`"theString": "A7", "intPrimitive": 3`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternEveryDistinct576Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternEveryDistinct576RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunPatternEveryDistinct576RuntimeIDMappingMatchesScenario(t *testing.T) {
	if len(patternEveryDistinct576CaseSpecs) != len(patternEveryDistinct576JavaRuntimeIDs) ||
		len(patternEveryDistinct576CaseSpecs) != len(patternEveryDistinct576JavaExecutions) ||
		len(patternEveryDistinct576CaseSpecs) != len(patternEveryDistinct576JavaStaticIDs) {
		t.Fatalf("case spec count = %d", len(patternEveryDistinct576CaseSpecs))
	}
	for index, spec := range patternEveryDistinct576CaseSpecs {
		if spec.runtimeID != patternEveryDistinct576JavaRuntimeIDs[index] ||
			spec.execution != patternEveryDistinct576JavaExecutions[index] ||
			spec.ordinal != index {
			t.Fatalf("case %d mapping = %q/%q/%d", index, spec.runtimeID, spec.execution, spec.ordinal)
		}
	}
	for index, id := range patternEveryDistinct576JavaStaticIDs {
		if id != "java-073091a0b42ca8dd974f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternEveryDistinct576ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-everydistinct-576.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternEveryDistinct576ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternEveryDistinct576ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternEveryDistinct576Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
