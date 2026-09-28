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
//   - ord 0 PatternOpWHarness java-runtime-* — the shared sixteen-case
//     followed-by list over the mixed event set (separate harness slice).
//   - ord 2 PatternFollowedByTimer java-runtime-* — timer:within + where
//     over SupportCallEvent (separate harness slice).
//   - ords 3/4/5 PatternMemoryRFIDEvent/PatternRFIDZoneExit/
//     PatternRFIDZoneEnter — covered by pattern-followedby-rfid-580.
//   - ords 7/8 PatternFollowedEveryMultiple/PatternFilterGreaterThen —
//     no-timer chains; future slice; manifest dispositions are
//     main-agent owned.

// TestRunPatternFollowedByTimerNot581ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record surface
// the oracle produces: 4 listener deliveries across the three cases —
// 2 for with-not (A1 at t=10000 and A4 at t=50000; correlated
// B(A2)/C(A3) cancels keep A2/A3 silent while non-correlated B(B4)/C(A5)
// do not touch A4's branch), 1 for not-every carrying TWO rows in ONE
// invocation (the Java getNewDataList().size()==1 + get(0).length==2
// batching pin — both one-second branches expire together at t=1000),
// and 1 for or-perm-false (the E sent at t=1000 fires alone at t=11000
// after the right alternative's not-timer died permanently at t=11000).
// Every delivered row carries the bound tag's bean fragment — `a`, `A`
// or `s` — with SupportBean's default theString rendering the null
// marker.
func TestRunPatternFollowedByTimerNot581ScenarioReplay(t *testing.T) {
	scenario := loadPatternFollowedByTimerNot581ScenarioForTest(t)
	trace, err := runPatternFollowedByTimerNot581Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternFollowedByTimerNot581ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 4 {
		t.Fatalf("records = %d, want 4", len(trace.Records))
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
	// recordRows decodes one record's new rows into plain JSON values so
	// field comparisons ignore map ordering exactly like the -diff path.
	recordRows := func(record compat.TraceRecord) []any {
		data, err := json.Marshal(record.New)
		if err != nil {
			t.Fatalf("marshal new rows: %v", err)
		}
		var rows []any
		if err := json.Unmarshal(data, &rows); err != nil {
			t.Fatalf("decode new rows: %v", err)
		}
		return rows
	}
	tagFields := func(rows []any, rowIndex int, tag string) map[string]any {
		row, ok := rows[rowIndex].(map[string]any)
		if !ok {
			return nil
		}
		fields, ok := row["fields"].(map[string]any)
		if !ok {
			return nil
		}
		fragment, ok := fields[tag].(map[string]any)
		if !ok {
			return nil
		}
		beanFields, ok := fragment["fields"].(map[string]any)
		if !ok {
			return nil
		}
		return beanFields
	}
	assertDeliveries := func(name string, tag string, wantTimes []string, wantRows [][]map[string]any) {
		t.Helper()
		deliveries := byCase[name]
		if len(deliveries) != len(wantTimes) {
			t.Fatalf("%s deliveries = %d, want %d", name, len(deliveries), len(wantTimes))
		}
		for index, delivery := range deliveries {
			if delivery.Time != wantTimes[index] {
				t.Fatalf("%s delivery %d time = %q, want %q", name, index, delivery.Time, wantTimes[index])
			}
			if delivery.Sequence != uint64(index+1) {
				t.Fatalf("%s delivery %d sequence = %d", name, index, delivery.Sequence)
			}
			rows := recordRows(delivery)
			if len(rows) != len(wantRows[index]) {
				t.Fatalf("%s delivery %d rows = %v, want %d", name, index, rows, len(wantRows[index]))
			}
			for rowIndex, want := range wantRows[index] {
				fields := tagFields(rows, rowIndex, tag)
				if fields == nil {
					t.Fatalf("%s delivery %d row %d = %v, want %s fragment", name, index, rowIndex, rows, tag)
				}
				if !reflect.DeepEqual(fields, want) {
					t.Fatalf("%s delivery %d row %d fields = %v, want %v", name, index, rowIndex, fields, want)
				}
			}
		}
	}
	assertDeliveries("with-not", "a",
		[]string{"1970-01-01T00:00:10Z", "1970-01-01T00:00:50Z"},
		[][]map[string]any{
			{{"id": "A1"}},
			{{"id": "A4"}},
		})
	// ord 6's discriminant: ONE record whose new rows carry both armed
	// branches — two default SupportBean fragments with theString null.
	assertDeliveries("not-every", "A",
		[]string{"1970-01-01T00:00:01Z"},
		[][]map[string]any{{
			{"intPrimitive": float64(0), "theString": map[string]any{"state": "null"}},
			{"intPrimitive": float64(0), "theString": map[string]any{"state": "null"}},
		}})
	assertDeliveries("or-perm-false", "s",
		[]string{"1970-01-01T00:00:11Z"},
		[][]map[string]any{
			{{"intPrimitive": float64(0), "theString": "E"}},
		})
}

// TestRunPatternFollowedByTimerNot581PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts.
func TestRunPatternFollowedByTimerNot581PinnedArtifacts(t *testing.T) {
	scenario := loadPatternFollowedByTimerNot581ScenarioForTest(t)
	if scenario.ID != patternFollowedByTimerNot581ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 36 {
		t.Fatalf("steps = %d, want 36", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "with-not" {
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
	if cases != 3 || deploys != 3 || undeploys != 3 || sends != 11 || advances != 16 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternFollowedByTimerNot581RejectsMalformedRawScenario
// verifies the loader pins the step sequence: a mutated EPL, advance
// instant or send payload is rejected. The or-perm-false mutations hit
// both discriminants — the literal `)or(` and the bare `10` interval —
// and the with-not mutation constantizes the correlated id=a.id cancel.
func TestRunPatternFollowedByTimerNot581RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternFollowedByTimerNot581ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"with-not timer length", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("timer:interval(10 seconds)"),
				[]byte("timer:interval(10 second)"), 1)
		}},
		{"with-not cancel correlation", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("SupportBean_B(id=a.id)"),
				[]byte("SupportBean_B(id='A2')"), 1)
		}},
		{"or-perm-false or spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("theString='C1'))or(SupportBean(theString='C2'"),
				[]byte("theString='C1')) or (SupportBean(theString='C2'"), 1)
		}},
		{"or-perm-false bare interval", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("not timer:interval(10))]"),
				[]byte("not timer:interval(10 seconds))]"), 1)
		}},
		{"advance instant", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:10.999Z"`),
				[]byte(`"at": "1970-01-01T00:00:10.998Z"`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"id": "B4"}`),
				[]byte(`"payload": {"id": "A4"}`), 1)
		}},
		{"support bean payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"theString": "E", "intPrimitive": 0}`),
				[]byte(`"payload": {"theString": "C1", "intPrimitive": 0}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternFollowedByTimerNot581Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternFollowedByTimerNot581RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports: each of the three cases owns its execution's runtimeId and
// ordinal.
func TestRunPatternFollowedByTimerNot581RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{1, 6, 9}
	wantRuntimeIndex := []int{0, 1, 2}
	if len(patternFollowedByTimerNot581CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternFollowedByTimerNot581CaseSpecs))
	}
	for index, spec := range patternFollowedByTimerNot581CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternFollowedByTimerNot581JavaStaticIDs {
		if id != "java-089b2086c9945dff918f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternFollowedByTimerNot581ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-followedby-timernot-581.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternFollowedByTimerNot581ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternFollowedByTimerNot581ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternFollowedByTimerNot581Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
