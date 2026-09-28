package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Siblings excluded from this slice (no scenario steps, no trace):
//   - ord 1 PatternOp — the 52-case W-harness over the timed mixed event
//     set (unbounded/exact/tight/loose/open bounds, shared-event
//     terminator-wins, or/and/sequence compositions, timer terminators).
//   - ord 4 PatternRepeatUseTags — every-repeated correlated sequences
//     plus timer:interval chains needing a clock harness.
//   - ord 6 PatternExpressionBounds — variable/substitution dynamic
//     bounds tracked under a separate dynamic-bounds unit.
//   - ord 8 legs {1,4,5,6,7,8,11,12,13} — the nine Java-only probes
//     (zero-valued literals, [4:6]-without-until, a[0].id own/follow-on
//     filter refs, non-numeric bound expressions) carry no steps and no
//     records; see the scenario description.
//   - manifest dispositions are main-agent owned.

// TestRunPatternMatchUntilStatic586ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: exactly 6 records — two listener deliveries for ord 7
// ({e:[(A,1),(A,2)]} then {e:[(A,5),(A,6)]} at the epoch instant with
// per-deployment sequences 1 and 2) and four compile-error records for
// the ord 8 probes carrying the pinned Java message prefixes.
func TestRunPatternMatchUntilStatic586ScenarioReplay(t *testing.T) {
	scenario := loadPatternMatchUntilStatic586ScenarioForTest(t)
	trace, err := runPatternMatchUntilStatic586Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternMatchUntilStatic586ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 6 {
		t.Fatalf("records = %d, want 6", len(trace.Records))
	}

	recordRowFields := func(record compat.TraceRecord) map[string]any {
		data, err := json.Marshal(record.New)
		if err != nil {
			t.Fatalf("marshal new rows: %v", err)
		}
		var rows []map[string]any
		if err := json.Unmarshal(data, &rows); err != nil {
			t.Fatalf("decode new rows: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("record %v has %d new rows, want 1", record, len(rows))
		}
		fields, ok := rows[0]["fields"].(map[string]any)
		if !ok {
			t.Fatalf("record %v row has no fields: %v", record, rows[0])
		}
		return fields
	}
	supportBean := func(theString string, intPrimitive float64) map[string]any {
		return map[string]any{"kind": "row", "fields": map[string]any{
			"theString": theString, "intPrimitive": intPrimitive}}
	}

	// ord 7: two listener records on s0 at the epoch instant; the first
	// pair completes at A(2), B(4) cancels only the pending partial and
	// A(5)/A(6) complete the re-armed pair.
	wantPairs := [][]any{
		{supportBean("A", 1), supportBean("A", 2)},
		{supportBean("A", 5), supportBean("A", 6)},
	}
	for index, want := range wantPairs {
		record := trace.Records[index]
		if record.Case != "bound-repeat-with-not" || record.Operation != "listener" ||
			record.Statement != "s0" {
			t.Fatalf("record %d = %+v", index, record)
		}
		if record.Sequence != uint64(index+1) {
			t.Fatalf("record %d sequence = %d, want %d", index, record.Sequence, index+1)
		}
		if record.Time != "1970-01-01T00:00:00Z" {
			t.Fatalf("record %d time = %q", index, record.Time)
		}
		if len(record.Old) != 0 {
			t.Fatalf("delivery %d should be new-only: %v", index, record.Old)
		}
		fields := recordRowFields(record)
		wantFields := map[string]any{"e": want}
		if !reflect.DeepEqual(fields, wantFields) {
			t.Fatalf("record %d fields = %v, want %v", index, fields, wantFields)
		}
	}

	// ord 8: four compile-error records pinning the Java message
	// prefixes; compile-error records carry no time field.
	wantProbes := []struct {
		statement string
		value     string
	}{
		{"inverted-bounds", patternMatchUntilStatic586InvertedBoundsError},
		{"negative-bounds", patternMatchUntilStatic586NegativeBoundsError},
		{"tag-redeclared-across-until", patternMatchUntilStatic586TagAcrossUntilError},
		{"tag-reused-inside-nested-until", patternMatchUntilStatic586TagNestedUntilError},
	}
	for index, want := range wantProbes {
		record := trace.Records[2+index]
		if record.Case != "invalid" || record.Operation != "compile-error" ||
			record.Statement != want.statement {
			t.Fatalf("record %d = %+v", 2+index, record)
		}
		if record.Time != "" || len(record.New) != 0 || len(record.Old) != 0 {
			t.Fatalf("record %d should carry no time/rows: %+v", 2+index, record)
		}
		if record.Value != want.value {
			t.Fatalf("record %d value = %v, want %q", 2+index, record.Value, want.value)
		}
	}
}

// TestRunPatternMatchUntilStatic586PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — two
// case blocks, one s0 deploy, six sends, four build-error probes, one
// undeploy-all and zero advance-time steps.
func TestRunPatternMatchUntilStatic586PinnedArtifacts(t *testing.T) {
	scenario := loadPatternMatchUntilStatic586ScenarioForTest(t)
	if scenario.ID != patternMatchUntilStatic586ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 14 {
		t.Fatalf("steps = %d, want 14", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "bound-repeat-with-not" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, buildErrors, advances, undeploys int
	order := []string{}
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
			order = append(order, step.Case)
		case "deploy":
			deploys++
		case "send":
			sends++
		case "build-error":
			buildErrors++
		case "advance-time":
			advances++
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 2 || deploys != 1 || sends != 6 || buildErrors != 4 || advances != 0 || undeploys != 1 {
		t.Fatalf("cases/deploys/sends/build-errors/advances/undeploys = %d/%d/%d/%d/%d/%d",
			cases, deploys, sends, buildErrors, advances, undeploys)
	}
	wantOrder := []string{"bound-repeat-with-not", "invalid"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("case order = %v, want %v", order, wantOrder)
	}
}

// TestRunPatternMatchUntilStatic586RejectsMalformedRawScenario verifies
// the loader pins the byte-exact discriminants: ord 7's `e = ` spacing
// collapsed, the `and not` guard dropped to `and`, an ord 8 probe EPL
// mutated, a build-error expectError prefix drifted, and a send payload
// mutated.
func TestRunPatternMatchUntilStatic586RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternMatchUntilStatic586ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"e tag spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`e = SupportBean(theString='A')`),
				[]byte(`e=SupportBean(theString='A')`), 1)
		}},
		{"and not guard dropped", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`and not SupportBean(theString='B')`),
				[]byte(`and SupportBean(theString='B')`), 1)
		}},
		{"probe epl mutated", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`select * from pattern[[10:4] SupportBean_A]`),
				[]byte(`select * from pattern[[10:5] SupportBean_A]`), 1)
		}},
		{"probe expectError drifted", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`lower bounds value '10' is higher then higher bounds '4'`),
				[]byte(`lower bounds value '10' is higher then higher bounds '5'`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"op": "send", "case": "bound-repeat-with-not", "eventType": "SupportBean", "payload": {"theString": "B", "intPrimitive": 4}}`),
				[]byte(`{"op": "send", "case": "bound-repeat-with-not", "eventType": "SupportBean", "payload": {"theString": "B", "intPrimitive": 5}}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternMatchUntilStatic586Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternMatchUntilStatic586RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// ords 7/8 own runtimes f10e94…/50cec2… with the per-execution static
// ids.
func TestRunPatternMatchUntilStatic586RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{7, 8}
	wantStatic := []string{
		"java-ada27e5f94fecc9ffce2",
		"java-256196856cfafb3c17b7",
	}
	if len(patternMatchUntilStatic586CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternMatchUntilStatic586CaseSpecs))
	}
	for index, spec := range patternMatchUntilStatic586CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != index {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	if !reflect.DeepEqual(patternMatchUntilStatic586JavaStaticIDs, wantStatic) {
		t.Fatalf("static ids = %v, want %v", patternMatchUntilStatic586JavaStaticIDs, wantStatic)
	}
}

func patternMatchUntilStatic586ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-matchuntil-static-586.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternMatchUntilStatic586ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternMatchUntilStatic586ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternMatchUntilStatic586Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
