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
//   - ord 0 PatternMatchUntilSimple / ord 2 PatternSelectArray / ord 3
//     PatternUseFilter / ord 5 PatternArrayFunctionRepeat — covered by
//     pattern-matchuntil-untimed-585.
//   - ord 1 PatternOp — the 52-case W-harness over the timed mixed event
//     set, tracked as its own follow-up unit.
//   - ord 6 PatternExpressionBounds — variable/substitution dynamic
//     bounds tracked under the dynamic-bounds unit.
//   - ords 7/8 PatternBoundRepeatWithNot + PatternInvalid — covered by
//     pattern-matchuntil-static-586.
//   - manifest dispositions are main-agent owned.

// TestRunPatternMatchUntilRepeatTags587ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record surface
// the oracle produces: ONE case carrying THREE sequential deploy legs
// and exactly 3 listener records — leg 1's {a:[A1,A2], b:[B(A1),B(A2)]}
// at the epoch instant, leg 2's {e1:[2x'2'], e2:[4x'3']} recorded at
// t=10000 (the e2 until-timer expires at t=7000 inside the
// advanceTime(10000) jump, which stamps the delivery at the target —
// Esper's until is a SUCCESSFUL terminator), and leg 3's {A:[10,20],
// B:[10,10], C:[10,10]} at t=15000 after the A[0].intPrimitive
// correlation admits both B and C pairs.
func TestRunPatternMatchUntilRepeatTags587ScenarioReplay(t *testing.T) {
	scenario := loadPatternMatchUntilRepeatTags587ScenarioForTest(t)
	trace, err := runPatternMatchUntilRepeatTags587Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternMatchUntilRepeatTags587ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 3 {
		t.Fatalf("records = %d, want 3", len(trace.Records))
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
	letterBean := func(id string) map[string]any {
		return map[string]any{"kind": "row", "fields": map[string]any{"id": id}}
	}

	// Leg 1: one listener record on s0 at the epoch instant carrying the
	// completed [2]-repeat's a/b tag arrays; the b predicates correlate
	// per iteration so b[0].id == a[0].id and b[1].id == a[1].id.
	record := trace.Records[0]
	if record.Case != "repeat-use-tags" || record.Operation != "listener" ||
		record.Statement != "s0" {
		t.Fatalf("record 0 = %+v", record)
	}
	if record.Sequence != 1 {
		t.Fatalf("record 0 sequence = %d, want 1", record.Sequence)
	}
	if record.Time != "1970-01-01T00:00:00Z" {
		t.Fatalf("record 0 time = %q", record.Time)
	}
	if len(record.Old) != 0 {
		t.Fatalf("record 0 should be new-only: %v", record.Old)
	}
	wantLeg1 := map[string]any{
		"a": []any{letterBean("A1"), letterBean("A2")},
		"b": []any{letterBean("A1"), letterBean("A2")},
	}
	if fields := recordRowFields(record); !reflect.DeepEqual(fields, wantLeg1) {
		t.Fatalf("record 0 fields = %v, want %v", fields, wantLeg1)
	}

	// Leg 2: ONE listener record at t=10000 — the e1 until-timer at 5000
	// completes its [2:] repeat with the two '2's, arming e2; the e2
	// until-timer expires at t=7000 mid-jump, and the delivery records
	// at the advanceTime(10000) target. Java asserts nothing on this
	// leg, so the delivery must match the oracle verbatim.
	record = trace.Records[1]
	if record.Case != "repeat-use-tags" || record.Operation != "listener" ||
		record.Statement != "s0" {
		t.Fatalf("record 1 = %+v", record)
	}
	if record.Sequence != 1 {
		t.Fatalf("record 1 sequence = %d, want 1 (fresh listener per deploy)", record.Sequence)
	}
	if record.Time != "1970-01-01T00:00:10Z" {
		t.Fatalf("record 1 time = %q", record.Time)
	}
	if len(record.Old) != 0 {
		t.Fatalf("record 1 should be new-only: %v", record.Old)
	}
	wantLeg2 := map[string]any{
		"e1": []any{supportBean("2", 0), supportBean("2", 0)},
		"e2": []any{supportBean("3", 0), supportBean("3", 0), supportBean("3", 0), supportBean("3", 0)},
	}
	if fields := recordRowFields(record); !reflect.DeepEqual(fields, wantLeg2) {
		t.Fatalf("record 1 fields = %v, want %v", fields, wantLeg2)
	}

	// Leg 3: one listener record at t=15000 — B and C correlate on
	// A[0].intPrimitive (10), the FIRST element of the completed
	// A-repeat, not the nearest A value 20.
	record = trace.Records[2]
	if record.Case != "repeat-use-tags" || record.Operation != "listener" ||
		record.Statement != "s0" {
		t.Fatalf("record 2 = %+v", record)
	}
	if record.Sequence != 1 {
		t.Fatalf("record 2 sequence = %d, want 1 (fresh listener per deploy)", record.Sequence)
	}
	if record.Time != "1970-01-01T00:00:15Z" {
		t.Fatalf("record 2 time = %q", record.Time)
	}
	if len(record.Old) != 0 {
		t.Fatalf("record 2 should be new-only: %v", record.Old)
	}
	wantLeg3 := map[string]any{
		"A": []any{supportBean("1", 10), supportBean("1", 20)},
		"B": []any{supportBean("2", 10), supportBean("2", 10)},
		"C": []any{supportBean("3", 10), supportBean("3", 10)},
	}
	if fields := recordRowFields(record); !reflect.DeepEqual(fields, wantLeg3) {
		t.Fatalf("record 2 fields = %v, want %v", fields, wantLeg3)
	}
}

// TestRunPatternMatchUntilRepeatTags587PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — one
// case block, three s0 deploys (one per leg), eighteen sends, four
// advance-time steps and three undeploy-all markers.
func TestRunPatternMatchUntilRepeatTags587PinnedArtifacts(t *testing.T) {
	scenario := loadPatternMatchUntilRepeatTags587ScenarioForTest(t)
	if scenario.ID != patternMatchUntilRepeatTags587ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 29 {
		t.Fatalf("steps = %d, want 29", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "repeat-use-tags" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, advances, undeploys int
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
	if cases != 1 || deploys != 3 || sends != 18 || advances != 4 || undeploys != 3 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternMatchUntilRepeatTags587RejectsMalformedRawScenario
// verifies the loader pins the byte-exact discriminants: leg 1's
// `SupportBean_A()` empty parens and unquoted `id=a.id`, leg 2's `[2:]`
// open ranges and `until timer:interval` arms, leg 3's double space
// after `pattern [` and the `A[0]` first-element index, plus send
// payload and advance-time instant pinning.
func TestRunPatternMatchUntilRepeatTags587RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternMatchUntilRepeatTags587ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"empty parens dropped", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`a=SupportBean_A() ->`),
				[]byte(`a=SupportBean_A ->`), 1)
		}},
		{"correlation quoted", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`b=SupportBean_B(id=a.id)`),
				[]byte(`b=SupportBean_B(id='a.id')`), 1)
		}},
		{"leg2 range bound", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`[2:]e1=SupportBean(theString='2')`),
				[]byte(`[2]e1=SupportBean(theString='2')`), 1)
		}},
		{"leg2 until arm drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(5))->([2:]`),
				[]byte(`timer:interval(4))->([2:]`), 1)
		}},
		{"leg3 double space collapsed", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`pattern [ every [2] A=`),
				[]byte(`pattern [every [2] A=`), 1)
		}},
		{"leg3 index drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`intPrimitive=A[0].intPrimitive)-> [2] C`),
				[]byte(`intPrimitive=A[1].intPrimitive)-> [2] C`), 1)
		}},
		{"advance-time instant", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"op": "advance-time", "case": "repeat-use-tags", "at": "1970-01-01T00:00:05Z"}`),
				[]byte(`{"op": "advance-time", "case": "repeat-use-tags", "at": "1970-01-01T00:00:04Z"}`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"op": "send", "case": "repeat-use-tags", "eventType": "SupportBean", "payload": {"theString": "1", "intPrimitive": 20}}`),
				[]byte(`{"op": "send", "case": "repeat-use-tags", "eventType": "SupportBean", "payload": {"theString": "1", "intPrimitive": 10}}`), 1)
		}},
		{"send order", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"op": "send", "case": "repeat-use-tags", "eventType": "SupportBean_A", "payload": {"id": "A1"}}`),
				[]byte(`{"op": "send", "case": "repeat-use-tags", "eventType": "SupportBean_B", "payload": {"id": "A1"}}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternMatchUntilRepeatTags587Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternMatchUntilRepeatTags587RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports: ord 4 owns runtime eb238a… with static id
// java-b2c644fc2f9603bd8568.
func TestRunPatternMatchUntilRepeatTags587RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{4}
	wantStatic := []string{"java-b2c644fc2f9603bd8568"}
	if len(patternMatchUntilRepeatTags587CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternMatchUntilRepeatTags587CaseSpecs))
	}
	for index, spec := range patternMatchUntilRepeatTags587CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != index {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	if !reflect.DeepEqual(patternMatchUntilRepeatTags587JavaStaticIDs, wantStatic) {
		t.Fatalf("static ids = %v, want %v", patternMatchUntilRepeatTags587JavaStaticIDs, wantStatic)
	}
	if !reflect.DeepEqual(patternMatchUntilRepeatTags587JavaRuntimeIDs,
		[]string{"java-runtime-eb238a96331acf6fc30b"}) {
		t.Fatalf("runtime ids = %v", patternMatchUntilRepeatTags587JavaRuntimeIDs)
	}
}

func patternMatchUntilRepeatTags587ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-matchuntil-repeattags-587.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternMatchUntilRepeatTags587ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternMatchUntilRepeatTags587ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer func() { _ = file.Close() }()
	scenario, err := loadPatternMatchUntilRepeatTags587Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
