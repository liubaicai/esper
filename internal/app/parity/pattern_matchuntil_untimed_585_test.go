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
//   - ords 6/7 PatternExpressionBounds/PatternBoundRepeatWithNot —
//     variable bounds + timer/not semantics tracked under pattern-basic.
//   - ord 8 PatternInvalid — compile-only probes.
//   - manifest dispositions are main-agent owned.

// TestRunPatternMatchUntilUntimed585ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: exactly 9 listener deliveries — one per firing leg
// (1 simple + 2 select-array + 5 use-filter + 1 array-function-repeat) —
// all at the epoch instant with per-deployment sequence 1 (each leg
// redeploys s0). The discriminant rows: ord 2 leg 1 carries the exact-order
// a array plus the element beans with a[2]/a[2].id null, and ord 5
// reports {length:3,l2:3} from TagCount.
func TestRunPatternMatchUntilUntimed585ScenarioReplay(t *testing.T) {
	scenario := loadPatternMatchUntilUntimed585ScenarioForTest(t)
	trace, err := runPatternMatchUntilUntimed585Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternMatchUntilUntimed585ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 9 {
		t.Fatalf("records = %d, want 9", len(trace.Records))
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
	nullMarker := map[string]any{"state": "null"}
	beanA := func(id string) map[string]any {
		return map[string]any{"kind": "row", "fields": map[string]any{"id": id}}
	}
	supportBean := func(theString string, intPrimitive float64) map[string]any {
		return map[string]any{"kind": "row", "fields": map[string]any{
			"theString": theString, "intPrimitive": intPrimitive}}
	}
	wantCases := []string{
		"simple",
		"select-array", "select-array",
		"use-filter", "use-filter", "use-filter", "use-filter", "use-filter",
		"array-function-repeat",
	}
	for index, record := range trace.Records {
		if record.Case != wantCases[index] || record.Operation != "listener" || record.Statement != "s0" {
			t.Fatalf("record %d = %+v", index, record)
		}
		if record.Sequence != 1 {
			t.Fatalf("record %d sequence = %d, want 1 (per-deployment listener)", index, record.Sequence)
		}
		// The engine clock never advances — every delivery is stamped at
		// the epoch, matching the oracle's initialize(0L) runtime.
		if record.Time != "1970-01-01T00:00:00Z" {
			t.Fatalf("record %d time = %q", index, record.Time)
		}
		if len(record.Old) != 0 {
			t.Fatalf("delivery %d should be new-only: %v", index, record.Old)
		}
	}

	// ord 0 simple: ONE row {c0:A1,c1:A2,c2:B1}; the trailing A1/B1 pair
	// stays silent, which the record count already pins.
	fields := recordRowFields(trace.Records[0])
	wantSimple := map[string]any{"c0": "A1", "c1": "A2", "c2": "B1"}
	if !reflect.DeepEqual(fields, wantSimple) {
		t.Fatalf("simple fields = %v, want %v", fields, wantSimple)
	}

	// ord 2 leg 1: explicit projection — a exact-order array, a0/a1 beans,
	// a2+a2Id null, a0Id/a1Id strings, b bean.
	fields = recordRowFields(trace.Records[1])
	wantLeg1 := map[string]any{
		"a":    []any{beanA("A1"), beanA("A2")},
		"b":    beanA("B1"),
		"a0":   beanA("A1"),
		"a0Id": "A1",
		"a1":   beanA("A2"),
		"a1Id": "A2",
		"a2":   nullMarker,
		"a2Id": nullMarker,
	}
	if !reflect.DeepEqual(fields, wantLeg1) {
		t.Fatalf("select-array leg 1 fields = %v, want %v", fields, wantLeg1)
	}

	// ord 2 leg 2: select * fragments {a:[A1,A2],b:B1}.
	fields = recordRowFields(trace.Records[2])
	wantLeg2 := map[string]any{
		"a": []any{beanA("A1"), beanA("A2")},
		"b": beanA("B1"),
	}
	if !reflect.DeepEqual(fields, wantLeg2) {
		t.Fatalf("select-array leg 2 fields = %v, want %v", fields, wantLeg2)
	}

	// ord 3 legs: each fires ONCE — concat c=CA1A2B1, equals c=(A2,10),
	// in c=(A3,5), not-in c=(A6,5), between a=[(A1,5),(A2,8)] b=(B1,-1)
	// c=(E3,5).
	wantUseFilter := []map[string]any{
		{"a": []any{beanA("A1"), beanA("A2")}, "b": beanA("B1"), "c": beanA("CA1A2B1")},
		{"a": []any{beanA("A1"), beanA("A2")}, "b": beanA("B1"), "c": supportBean("A2", 10)},
		{"a": []any{beanA("A1"), beanA("A2"), beanA("A3")}, "b": beanA("B1"), "c": supportBean("A3", 5)},
		{"a": []any{beanA("A1"), beanA("A2"), beanA("A3")}, "b": beanA("B1"), "c": supportBean("A6", 5)},
		{"a": []any{supportBean("A1", 5), supportBean("A2", 8)}, "b": supportBean("B1", -1), "c": supportBean("E3", 5)},
	}
	for leg, want := range wantUseFilter {
		fields = recordRowFields(trace.Records[3+leg])
		if !reflect.DeepEqual(fields, want) {
			t.Fatalf("use-filter leg %d fields = %v, want %v", leg, fields, want)
		}
	}

	// ord 5: TagCount maps arrayLength/getLength — {length:3,l2:3}.
	fields = recordRowFields(trace.Records[8])
	wantRepeat := map[string]any{"length": float64(3), "l2": float64(3)}
	if !reflect.DeepEqual(fields, wantRepeat) {
		t.Fatalf("array-function-repeat fields = %v, want %v", fields, wantRepeat)
	}
}

// TestRunPatternMatchUntilUntimed585PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — four
// case blocks, nine s0 deploys (1+2+5+1 legs), forty-four sends, nine
// undeploy-alls and zero advance-time steps.
func TestRunPatternMatchUntilUntimed585PinnedArtifacts(t *testing.T) {
	scenario := loadPatternMatchUntilUntimed585ScenarioForTest(t)
	if scenario.ID != patternMatchUntilUntimed585ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 66 {
		t.Fatalf("steps = %d, want 66", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "simple" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, advances, undeploys int
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
		case "advance-time":
			advances++
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 4 || deploys != 9 || sends != 44 || advances != 0 || undeploys != 9 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
	wantOrder := []string{"simple", "select-array", "use-filter", "array-function-repeat"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("case order = %v, want %v", order, wantOrder)
	}
}

// TestRunPatternMatchUntilUntimed585RejectsMalformedRawScenario verifies
// the loader pins the byte-exact discriminants: the capital @Name of ord
// 0 lowered to @name, a space inserted after `in` in the ord 3 in-leg,
// the concat filter's spacing collapsed, and a mutated send payload.
func TestRunPatternMatchUntilUntimed585RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternMatchUntilUntimed585ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"capital @Name lowered", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`@Name('s0') select a[0].theString`),
				[]byte(`@name('s0') select a[0].theString`), 1)
		}},
		{"in spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`theString in(a[2].id)`),
				[]byte(`theString in (a[2].id)`), 1)
		}},
		{"concat spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`(id = ('C' || a[0].id || a[1].id || b.id))`),
				[]byte(`(id=('C' || a[0].id || a[1].id || b.id))`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"op": "send", "case": "use-filter", "eventType": "SupportBean_C", "payload": {"id": "CA1A2B1"}}`),
				[]byte(`{"op": "send", "case": "use-filter", "eventType": "SupportBean_C", "payload": {"id": "CA1A2B2"}}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternMatchUntilUntimed585Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternMatchUntilUntimed585RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// ords 0/2/3/5 own runtimes 93ea4a…/bd06e4…/4b8c99…/602d43… with the
// per-execution static ids (ord 0 keeps the file-level
// java-2169accb9d43755e87e4 shared with the deduplicated inventory row).
func TestRunPatternMatchUntilUntimed585RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{0, 2, 3, 5}
	wantStatic := []string{
		"java-2169accb9d43755e87e4",
		"java-c393b406de406e3ffd1f",
		"java-bf3211b9b2822d0cbbf4",
		"java-7a5adcaeaa460c4641cb",
	}
	if len(patternMatchUntilUntimed585CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternMatchUntilUntimed585CaseSpecs))
	}
	for index, spec := range patternMatchUntilUntimed585CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != index {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	if !reflect.DeepEqual(patternMatchUntilUntimed585JavaStaticIDs, wantStatic) {
		t.Fatalf("static ids = %v, want %v", patternMatchUntilUntimed585JavaStaticIDs, wantStatic)
	}
}

func patternMatchUntilUntimed585ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-matchuntil-untimed-585.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternMatchUntilUntimed585ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternMatchUntilUntimed585ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternMatchUntilUntimed585Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
