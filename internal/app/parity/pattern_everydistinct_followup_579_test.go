package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Siblings excluded from this slice (no scenario steps, no trace):
//   - ord 14 PatternInvalid java-runtime-2542b81b75a973a3cc38 is
//     compile-error-only (three tryInvalid messages; the typed Go API has
//     no EPL-text compile path to pin the verbatim diagnostics).
// Ords 0-2, 4-12 and 16 already live under pattern-everydistinct-576 /
// pattern-everydistinct-compound-577 / pattern-everydistinct-nested-578.

// TestRunPatternEveryDistinctFollowup579ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record surface
// the oracle produces: 20 listener deliveries across the five cases —
// 4 per over-filter leg (the unqualified-key distinct delivers E1/E3/E4/
// E8 while the dup keys stay silent), 5 per followedby-with-distinct leg
// (per-branch right keysets let B1/B2 both fire on the A1 branch, the
// left-dup A2 spawns no branch, and B7 fans out TWO rows {A1,B7},{A3,B7}
// inside ONE delivery — the multi-fire discriminant: 5 listener calls
// carrying 6 rows) and 2 for month-scoped (the key survives one
// millisecond before the calendar-month mark and E1(4) refires at
// exactly 2002-03-01T09:00:00). Each timed leg shares its ord's
// no-expiry sequence because no expiry falls inside the send window.
func TestRunPatternEveryDistinctFollowup579ScenarioReplay(t *testing.T) {
	scenario := loadPatternEveryDistinctFollowup579ScenarioForTest(t)
	trace, err := runPatternEveryDistinctFollowup579Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternEveryDistinctFollowup579ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 20 {
		t.Fatalf("records = %d, want 20", len(trace.Records))
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
	// beanRow reads one bound tag's delivered bean row fields.
	beanRow := func(record compat.TraceRecord, rowIndex int, tag string) map[string]any {
		row, ok := record.New[rowIndex].Fields[tag].(map[string]any)
		if !ok {
			return nil
		}
		fields, ok := row["fields"].(map[string]any)
		if !ok {
			return nil
		}
		return fields
	}

	// ord 3 delivers the captured bean row keyed on theString.
	assertBeans := func(cases []string, wants []string) {
		t.Helper()
		for _, name := range cases {
			deliveries := byCase[name]
			if len(deliveries) != len(wants) {
				t.Fatalf("%s deliveries = %d, want %d", name, len(deliveries), len(wants))
			}
			for index, want := range wants {
				if len(deliveries[index].New) != 1 {
					t.Fatalf("%s delivery %d = %v", name, index, deliveries[index].New)
				}
				fields := beanRow(deliveries[index], 0, "a")
				if fields == nil || fields["theString"] != want {
					t.Fatalf("%s delivery %d = %v, want a.theString=%s",
						name, index, deliveries[index].New, want)
				}
			}
		}
	}
	assertBeans([]string{"over-filter", "over-filter-expiry"},
		[]string{"E1", "E3", "E4", "E8"})

	// ord 13 delivers {a,b} pairs; the B7 fan-out carries two rows in one
	// delivery (any-order, so collect the a multiset).
	assertPairs := func(cases []string) {
		t.Helper()
		for _, name := range cases {
			deliveries := byCase[name]
			if len(deliveries) != 5 {
				t.Fatalf("%s deliveries = %d, want 5", name, len(deliveries))
			}
			wantSingle := [][2]string{
				{"A1", "B1"}, {"A1", "B2"}, {"A1", "B4"}, {"A3", "B5"},
			}
			for index, want := range wantSingle {
				if len(deliveries[index].New) != 1 {
					t.Fatalf("%s delivery %d = %v, want one row",
						name, index, deliveries[index].New)
				}
				aFields := beanRow(deliveries[index], 0, "a")
				bFields := beanRow(deliveries[index], 0, "b")
				if aFields == nil || bFields == nil ||
					aFields["theString"] != want[0] || bFields["theString"] != want[1] {
					t.Fatalf("%s delivery %d = %v, want {%s,%s}",
						name, index, deliveries[index].New, want[0], want[1])
				}
			}
			// deliveries[0-3] are the single-row fires; deliveries[4] is
			// the B7 two-row fan-out — one listener update carrying both
			// retained-branch completions {A1,B7} and {A3,B7} any-order.
			fanout := deliveries[4]
			if len(fanout.New) != 2 {
				t.Fatalf("%s B7 fan-out = %v, want two rows", name, fanout.New)
			}
			seen := map[string]bool{}
			for row := range 2 {
				aFields := beanRow(fanout, row, "a")
				bFields := beanRow(fanout, row, "b")
				if aFields == nil || bFields == nil || bFields["theString"] != "B7" {
					t.Fatalf("%s B7 row %d = %v", name, row, fanout.New)
				}
				aString, _ := aFields["theString"].(string)
				seen[aString] = true
			}
			if !seen["A1"] || !seen["A3"] {
				t.Fatalf("%s B7 rows = %v, want {A1,B7} and {A3,B7} any-order",
					name, fanout.New)
			}
		}
	}
	assertPairs([]string{"followedby-with-distinct", "followedby-with-distinct-expiry"})

	// ord 15 delivers the bean row with both asserted fields
	// (a.theString,a.intPrimitive): {E1,1} before the month mark and
	// {E1,4} at exactly 2002-03-01T09:00:00.
	monthDeliveries := byCase["month-scoped"]
	if len(monthDeliveries) != 2 {
		t.Fatalf("month-scoped deliveries = %d, want 2", len(monthDeliveries))
	}
	for index, want := range []struct {
		theString    string
		intPrimitive int
	}{
		{"E1", 1},
		{"E1", 4},
	} {
		if len(monthDeliveries[index].New) != 1 {
			t.Fatalf("month-scoped delivery %d = %v", index, monthDeliveries[index].New)
		}
		fields := beanRow(monthDeliveries[index], 0, "a")
		if fields == nil || fields["theString"] != want.theString ||
			fields["intPrimitive"] != want.intPrimitive {
			t.Fatalf("month-scoped delivery %d = %v, want {%s,%d}",
				index, monthDeliveries[index].New, want.theString, want.intPrimitive)
		}
	}
}

// TestRunPatternEveryDistinctFollowup579PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts.
func TestRunPatternEveryDistinctFollowup579PinnedArtifacts(t *testing.T) {
	scenario := loadPatternEveryDistinctFollowup579ScenarioForTest(t)
	if scenario.ID != patternEveryDistinctFollowup579ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 58 {
		t.Fatalf("steps = %d, want 58", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "over-filter" {
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
	if cases != 5 || deploys != 5 || undeploys != 5 || sends != 40 || advances != 3 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternEveryDistinctFollowup579RejectsMalformedRawScenario
// verifies the loader pins the step sequence: a mutated EPL, send payload
// or advance-time instant is rejected.
func TestRunPatternEveryDistinctFollowup579RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternEveryDistinctFollowup579ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"unqualified key", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(intPrimitive) a=SupportBean"),
				[]byte("every-distinct(a.intPrimitive) a=SupportBean"), 1)
		}},
		{"expiry spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(intPrimitive,2 minutes)"),
				[]byte("every-distinct(intPrimitive, 2 minutes)"), 1)
		}},
		{"left expiry on the wrong leg", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(a.intPrimitive) a=SupportBean(theString like 'A%')"),
				[]byte("every-distinct(a.intPrimitive, 1 day) a=SupportBean(theString like 'A%')"), 1)
		}},
		{"right-side expiry mutation", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(b.intPrimitive) b=SupportBean"),
				[]byte("every-distinct(b.intPrimitive, 1 day) b=SupportBean"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "B7", "intPrimitive": 3`),
				[]byte(`"theString": "B7", "intPrimitive": 2`), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("2002-03-01T08:59:59.999Z"),
				[]byte("2002-03-01T09:00:00.001Z"), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternEveryDistinctFollowup579Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternEveryDistinctFollowup579RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports: both legs of ords 3/13 share their execution's runtimeId,
// execution name and ordinal.
func TestRunPatternEveryDistinctFollowup579RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{3, 3, 13, 13, 15}
	wantRuntimeIndex := []int{0, 0, 1, 1, 2}
	if len(patternEveryDistinctFollowup579CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternEveryDistinctFollowup579CaseSpecs))
	}
	for index, spec := range patternEveryDistinctFollowup579CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternEveryDistinctFollowup579JavaStaticIDs {
		if id != "java-073091a0b42ca8dd974f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternEveryDistinctFollowup579ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-everydistinct-followup-579.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternEveryDistinctFollowup579ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternEveryDistinctFollowup579ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternEveryDistinctFollowup579Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
