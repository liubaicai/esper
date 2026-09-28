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
//   - ord 13 PatternFollowedByWithDistinct java-runtime-* is a dual-distinct
//     multi-fire execution — a follow-on slice.
//   - ord 14 PatternInvalid java-runtime-2542b81b75a973a3cc38 is
//     compile-error-only (three tryInvalid messages; the typed Go API has
//     no EPL-text compile path to pin the verbatim diagnostics).
//   - ord 15 PatternMonthScoped java-runtime-d211795c3ecad71c297a needs
//     calendar-month expiry (EveryDistinctForCalendar) — a different
//     semantic than the fixed-interval legs here.
// Ords 0-2 and 8-12 already live under pattern-everydistinct-576 /
// pattern-everydistinct-compound-577.

// TestRunPatternEveryDistinctNested578ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: 22 listener deliveries across the nine cases — 1 per
// repeat-over-distinct leg (the distinct-inside-repeat match delivers the
// tag array {a[0]=E1, a[1]=E3}), 2 per timer-within-over-distinct leg
// (E1/E3 fire before sendTimer(11000) kills the guard), 2 per
// everydistinct-over-repeat leg (the post-completion a[0] key suppresses
// the would-be key-1 dup {E3,E4}), 4 per everydistinct-over-timerwithin
// leg (E1/E3 fire, E4-E9 are swallowed while a guarded branch lives,
// E10/E12 refire after every branch has quit — the within-quit
// keyset-reset discriminant) and 4 for multikey-w-array (content-equal
// arrays dedup while empty and null are their own keys). Each timed leg
// shares its ord's no-expiry sequence because no expiry falls inside the
// send window.
func TestRunPatternEveryDistinctNested578ScenarioReplay(t *testing.T) {
	scenario := loadPatternEveryDistinctNested578ScenarioForTest(t)
	trace, err := runPatternEveryDistinctNested578Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternEveryDistinctNested578ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 22 {
		t.Fatalf("records = %d, want 22", len(trace.Records))
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
	// singleRow reads the bound tag as one bean row; arrayRows reads it as
	// the repeat tag array.
	singleRow := func(record compat.TraceRecord, tag string) map[string]any {
		row, ok := record.New[0].Fields[tag].(map[string]any)
		if !ok {
			return nil
		}
		fields, ok := row["fields"].(map[string]any)
		if !ok {
			return nil
		}
		return fields
	}
	arrayRows := func(record compat.TraceRecord, tag string) []map[string]any {
		rows, ok := record.New[0].Fields[tag].([]any)
		if !ok {
			return nil
		}
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			row, ok := item.(map[string]any)
			if !ok {
				return nil
			}
			fields, ok := row["fields"].(map[string]any)
			if !ok {
				return nil
			}
			out = append(out, fields)
		}
		return out
	}

	// ords 4/6 deliver the two-element tag array a.
	assertTagArray := func(cases []string, wants [][][2]any) {
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
				rows := arrayRows(deliveries[index], "a")
				if len(rows) != len(want) {
					t.Fatalf("%s delivery %d tag array = %v, want %d elements",
						name, index, deliveries[index].New, len(want))
				}
				for element, pair := range want {
					if rows[element]["theString"] != pair[0] || rows[element]["intPrimitive"] != pair[1] {
						t.Fatalf("%s delivery %d a[%d] = %v, want {%v, %v}",
							name, index, element, rows[element], pair[0], pair[1])
					}
				}
			}
		}
	}
	assertTagArray([]string{"repeat-over-distinct", "repeat-over-distinct-expiry"},
		[][][2]any{{{"E1", 1}, {"E3", 2}}})
	assertTagArray([]string{"everydistinct-over-repeat", "everydistinct-over-repeat-expiry"},
		[][][2]any{{{"E1", 1}, {"E2", 1}}, {{"E5", 2}, {"E6", 1}}})

	// ords 5/7 deliver the captured bean row.
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
				fields := singleRow(deliveries[index], "a")
				if fields == nil || fields["theString"] != want {
					t.Fatalf("%s delivery %d = %v, want a.theString=%s",
						name, index, deliveries[index].New, want)
				}
			}
		}
	}
	assertBeans([]string{"timer-within-over-distinct", "timer-within-over-distinct-expiry"},
		[]string{"E1", "E3"})
	assertBeans([]string{"everydistinct-over-timerwithin", "everydistinct-over-timerwithin-expiry"},
		[]string{"E1", "E3", "E10", "E12"})

	// ord 16 delivers the SupportEventWithIntArray bean row: {id, array}
	// with array as the JSON element list or the null marker.
	deliveries := byCase["multikey-w-array"]
	if len(deliveries) != 4 {
		t.Fatalf("multikey-w-array deliveries = %d, want 4", len(deliveries))
	}
	for index, want := range []struct {
		id    string
		array []int
	}{
		{"E1", []int{1, 2}},
		{"E3", []int{1}},
		{"E4", []int{}},
	} {
		fields := singleRow(deliveries[index], "a")
		if fields == nil || fields["id"] != want.id {
			t.Fatalf("multikey-w-array delivery %d = %v, want a.id=%s",
				index, deliveries[index].New, want.id)
		}
		array, ok := fields["array"].([]int)
		if !ok || len(array) != len(want.array) {
			t.Fatalf("multikey-w-array delivery %d array = %v, want %v",
				index, fields["array"], want.array)
		}
		for element, item := range want.array {
			if array[element] != item {
				t.Fatalf("multikey-w-array delivery %d array[%d] = %v, want %v",
					index, element, array[element], item)
			}
		}
	}
	fields := singleRow(deliveries[3], "a")
	if fields == nil || fields["id"] != "E5" {
		t.Fatalf("multikey-w-array delivery 3 = %v, want a.id=E5", deliveries[3].New)
	}
	// The Java null int[] surfaces as a typed-nil Go []int, which JSON
	// marshals as null — matching the oracle's Json.NULL rendering.
	array, isSlice := fields["array"].([]int)
	if !isSlice || array != nil {
		t.Fatalf("multikey-w-array delivery 3 array = %v, want the JSON null (nil slice)", fields["array"])
	}
}

// TestRunPatternEveryDistinctNested578PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts.
func TestRunPatternEveryDistinctNested578PinnedArtifacts(t *testing.T) {
	scenario := loadPatternEveryDistinctNested578ScenarioForTest(t)
	if scenario.ID != patternEveryDistinctNested578ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 112 {
		t.Fatalf("steps = %d, want 112", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "repeat-over-distinct" {
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
	if cases != 9 || deploys != 9 || undeploys != 9 || sends != 67 || advances != 18 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternEveryDistinctNested578RejectsMalformedRawScenario verifies
// the loader pins the step sequence: a mutated EPL, send payload or
// advance-time instant is rejected.
func TestRunPatternEveryDistinctNested578RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternEveryDistinctNested578ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("[[2] every-distinct(a.intPrimitive)"),
				[]byte("[[3] every-distinct(a.intPrimitive)"), 1)
		}},
		{"doubled key", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every-distinct(a[0].intPrimitive, a[0].intPrimitive, 1 hour)"),
				[]byte("every-distinct(a[0].intPrimitive, 1 hour)"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "E6", "intPrimitive": 1`),
				[]byte(`"theString": "E6", "intPrimitive": 2`), 1)
		}},
		{"array payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"id": "E3", "array": [1]`),
				[]byte(`"id": "E3", "array": [1, 2]`), 1)
		}},
		{"advance-time", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("1970-01-01T00:00:50.000Z"),
				[]byte("1970-01-01T00:00:51.000Z"), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternEveryDistinctNested578Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternEveryDistinctNested578RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// both legs of each ord share their execution's runtimeId, execution name
// and ordinal.
func TestRunPatternEveryDistinctNested578RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{4, 4, 5, 5, 6, 6, 7, 7, 16}
	wantRuntimeIndex := []int{0, 0, 1, 1, 2, 2, 3, 3, 4}
	if len(patternEveryDistinctNested578CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternEveryDistinctNested578CaseSpecs))
	}
	for index, spec := range patternEveryDistinctNested578CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternEveryDistinctNested578JavaStaticIDs {
		if id != "java-073091a0b42ca8dd974f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternEveryDistinctNested578ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-everydistinct-nested-578.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternEveryDistinctNested578ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternEveryDistinctNested578ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternEveryDistinctNested578Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
