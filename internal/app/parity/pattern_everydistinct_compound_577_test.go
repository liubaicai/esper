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
//     semantic than the fixed-interval ords 8-12.
// Ords 4/6 (repeat pair with tag-array projections), ord 13 (dual-distinct
// multi-fire) and ord 16 (int-array bean) are follow-on slices; ords 5/7
// already live under case.pattern-every.

// TestRunPatternEveryDistinctCompound577ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record surface
// the oracle produces: 34 listener deliveries across the ten leg cases —
// 4 per over-and leg (composite key), 4 per over-or leg (coalesced sum
// key), 3 per over-not leg (falsification respawns with an empty key set
// so A4 refires), 2 per over-followed-by leg (sum key) and 4 per
// within-followed-by leg (one waiting branch per fresh key). Each timed
// leg shares its ord's no-expiry sequence because no clock op exists.
func TestRunPatternEveryDistinctCompound577ScenarioReplay(t *testing.T) {
	scenario := loadPatternEveryDistinctCompound577ScenarioForTest(t)
	trace, err := runPatternEveryDistinctCompound577Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternEveryDistinctCompound577ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 34 {
		t.Fatalf("records = %d, want 34", len(trace.Records))
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		if record.Operation != "listener" {
			t.Fatalf("unexpected operation %q", record.Operation)
		}
		if record.Statement != "s0" {
			t.Fatalf("unexpected statement %q", record.Statement)
		}
		if len(record.Old) != 0 {
			t.Fatalf("delivery should be new-only: %v", record.Old)
		}
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	beanField := func(record compat.TraceRecord, tag string) (map[string]any, bool) {
		if len(record.New) != 1 {
			return nil, false
		}
		row, ok := record.New[0].Fields[tag].(map[string]any)
		if !ok {
			return nil, false
		}
		fields, ok := row["fields"].(map[string]any)
		return fields, ok
	}
	// assertFires checks the delivered (a.theString, b.theString) pairs per
	// case; a "" entry pins the null marker on the non-firing side.
	assertFires := func(cases []string, wants [][2]string) {
		t.Helper()
		for _, name := range cases {
			deliveries := byCase[name]
			if len(deliveries) != len(wants) {
				t.Fatalf("%s deliveries = %d, want %d", name, len(deliveries), len(wants))
			}
			for index, want := range wants {
				record := deliveries[index]
				if len(record.New) != 1 {
					t.Fatalf("%s delivery %d = %v", name, index, record.New)
				}
				aFields, aOK := beanField(record, "a")
				bFields, bOK := beanField(record, "b")
				if want[0] != "" {
					if !aOK || aFields["theString"] != want[0] {
						t.Fatalf("%s delivery %d a = %v, want a.theString=%s",
							name, index, record.New, want[0])
					}
				} else if aOK {
					t.Fatalf("%s delivery %d should carry no bound a tag: %v",
						name, index, record.New)
				}
				if want[1] != "" {
					if !bOK || bFields["theString"] != want[1] {
						t.Fatalf("%s delivery %d b = %v, want b.theString=%s",
							name, index, record.New, want[1])
					}
				} else if marker, isMarker := record.New[0].Fields["b"].(map[string]any); !isMarker || marker["state"] != "null" {
					t.Fatalf("%s delivery %d should carry the b null marker: %v",
						name, index, record.New)
				}
			}
		}
	}
	assertFires([]string{"over-and", "over-and-expiry"},
		[][2]string{{"A1", "B1"}, {"A3", "B3"}, {"A4", "B4"}, {"A6", "B6"}})
	assertFires([]string{"over-or", "over-or-expiry"},
		[][2]string{{"A1", ""}, {"", "B1"}, {"", "B4"}, {"", "B5"}})
	assertFires([]string{"over-followed-by", "over-followed-by-expiry"},
		[][2]string{{"A1", "B1"}, {"A4", "B4"}})
	assertFires([]string{"within-followed-by", "within-followed-by-expiry"},
		[][2]string{{"A1", "B2"}, {"A3", "B3"}, {"A2", "B5"}, {"A6", "B7"}})

	// ord 10's `and not` projects only tag a: A1 fires key 1, A3 fires
	// key 2 and A4 fires key 1 AGAIN — the B1 falsification respawned the
	// attempt with an empty key set (EvalEveryDistinctStateNode
	// evaluateFalse), which is the discriminant this slice exists to pin.
	for _, name := range []string{"over-not", "over-not-expiry"} {
		deliveries := byCase[name]
		if len(deliveries) != 3 {
			t.Fatalf("%s deliveries = %d, want 3", name, len(deliveries))
		}
		for index, want := range []string{"A1", "A3", "A4"} {
			aFields, ok := beanField(deliveries[index], "a")
			if !ok || aFields["theString"] != want {
				t.Fatalf("%s delivery %d = %v, want a.theString=%s",
					name, index, deliveries[index].New, want)
			}
			if _, hasB := deliveries[index].New[0].Fields["b"]; hasB {
				t.Fatalf("%s delivery %d should not carry a b tag (and not is untagged): %v",
					name, index, deliveries[index].New)
			}
		}
	}
}

// TestRunPatternEveryDistinctCompound577PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts.
func TestRunPatternEveryDistinctCompound577PinnedArtifacts(t *testing.T) {
	scenario := loadPatternEveryDistinctCompound577ScenarioForTest(t)
	if scenario.ID != patternEveryDistinctCompound577ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 138 {
		t.Fatalf("steps = %d, want 138", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "over-and" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, advances, sends, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
		case "send":
			sends++
		case "advance-time":
			advances++
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 10 || deploys != 10 || undeploys != 10 || sends != 108 || advances != 0 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternEveryDistinctCompound577RejectsMalformedRawScenario
// verifies the loader pins the step sequence: a mutated EPL or mutated
// send payload is rejected.
func TestRunPatternEveryDistinctCompound577RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternEveryDistinctCompound577ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(a.intPrimitive, b.intPrimitive, 1 hour)"),
				[]byte("every-distinct(a.intPrimitive, b.intPrimitive, 2 hour)"), 1)
		}},
		{"expiry literal", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(a.intPrimitive, 2 hours 1 minute)"),
				[]byte("every-distinct(a.intPrimitive, 2 hours)"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "B3", "intPrimitive": -8`),
				[]byte(`"theString": "B3", "intPrimitive": -9`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternEveryDistinctCompound577Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternEveryDistinctCompound577RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports: both legs of each ord share their execution's runtimeId,
// execution name and ordinal.
func TestRunPatternEveryDistinctCompound577RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{8, 8, 9, 9, 10, 10, 11, 11, 12, 12}
	wantRuntimeIndex := []int{0, 0, 1, 1, 2, 2, 3, 3, 4, 4}
	if len(patternEveryDistinctCompound577CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternEveryDistinctCompound577CaseSpecs))
	}
	for index, spec := range patternEveryDistinctCompound577CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternEveryDistinctCompound577JavaStaticIDs {
		if id != "java-073091a0b42ca8dd974f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternEveryDistinctCompound577ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-everydistinct-compound-577.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternEveryDistinctCompound577ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternEveryDistinctCompound577ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternEveryDistinctCompound577Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
