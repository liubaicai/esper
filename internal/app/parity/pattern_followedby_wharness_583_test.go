package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/liubaicai/esper/internal/compat"
)

// Siblings excluded from this slice (no scenario steps, no trace):
//   - ord 2 PatternFollowedByTimer java-runtime-4759bc801b8c0be6c10a —
//     timer:within + statement-where over SupportCallEvent (separate
//     micro-unit per the contract split).
//   - ords 1/6/9 covered by pattern-followedby-timernot-581, ords 3/4/5
//     by pattern-followedby-rfid-580, ords 7/8 by
//     pattern-followedby-chain-582.
//   - the harness's COMPILE_TO_MODEL, COMPILE_TO_EPL and
//     USE_EPL_AND_CONSUME_NOCHECK replay styles are compile-text/consume
//     surfaces outside this runner; manifest dispositions are
//     main-agent owned.

// patternFollowedByWHarness583ExpectedRecord pins one listener record:
// the leg (1..16), the delivery instant in epoch millis (the trigger
// event's external-clock time) and the flattened rows as "tag:id"
// pairs — "d:~" marks the vacant null cell Java renders as
// {"state":"null"}. Row order is the delivery order; duplicated rows
// (legs 10/15/16) are normative multiset multiplicity.
type patternFollowedByWHarness583ExpectedRecord struct {
	leg  int
	atMS int64
	rows [][]string
}

// Records are pinned in Java delivery order: Esper dispatches one
// event's completions to statement listeners in REVERSE deployment
// order, so the legs run newest-first (16,15,...) inside each send.
var patternFollowedByWHarness583ExpectedRecords = []patternFollowedByWHarness583ExpectedRecord{
	// B1 at t=2000: legs 16/15 fire the a=A1 branch first, then legs
	// 6/5/2/1 complete at spawn with vacant d.
	{16, 2000, [][]string{{"a:A1", "b:B1"}}},
	{15, 2000, [][]string{{"a:A1", "b:B1"}}},
	{6, 2000, [][]string{{"b:B1", "d:~"}}},
	{5, 2000, [][]string{{"b:B1", "d:~"}}},
	{2, 2000, [][]string{{"b:B1", "d:~"}}},
	{1, 2000, [][]string{{"b:B1", "d:~"}}},
	// B2 at t=4000: legs 16/15 fire the a=A1 branch again.
	{16, 4000, [][]string{{"a:A1", "b:B2"}}},
	{15, 4000, [][]string{{"a:A1", "b:B2"}}},
	// A2 at t=5000: the three a_1->b->a_2 chain legs complete once.
	{14, 5000, [][]string{{"a_1:A1", "a_2:A2", "b:B1"}}},
	{13, 5000, [][]string{{"a_1:A1", "a_2:A2", "b:B1"}}},
	{11, 5000, [][]string{{"a_1:A1", "a_2:A2", "b:B1"}}},
	// D1 at t=6000.
	{10, 6000, [][]string{{"b:B1", "d:D1"}}},
	{9, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{8, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{7, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{4, 6000, [][]string{{"b:B1", "d:D1"}}},
	{3, 6000, [][]string{{"b:B1", "d:D1"}}},
	{2, 6000, [][]string{{"b:B1", "d:D1"}}},
	{1, 6000, [][]string{{"b:B1", "d:D1"}}},
	// D2 at t=9000.
	{10, 9000, [][]string{{"b:B1", "d:D2"}}},
	{7, 9000, [][]string{{"b:B1", "d:D2"}, {"b:B2", "d:D2"}}},
	{3, 9000, [][]string{{"b:B1", "d:D2"}}},
	// B3 at t=10000: legs 16/15 fire the duplicated tails.
	{16, 10000, [][]string{{"a:A1", "b:B3"}, {"a:A2", "b:B3"}, {"a:A2", "b:B3"}}},
	{15, 10000, [][]string{{"a:A1", "b:B3"}, {"a:A2", "b:B3"}, {"a:A2", "b:B3"}, {"a:A2", "b:B3"}}},
	// D3 at t=12000.
	{10, 12000, [][]string{{"b:B1", "d:D3"}, {"b:B3", "d:D3"}, {"b:B3", "d:D3"}}},
	{9, 12000, [][]string{{"b:B3", "d:D3"}}},
	{8, 12000, [][]string{{"b:B3", "d:D3"}}},
	{7, 12000, [][]string{{"b:B1", "d:D3"}, {"b:B2", "d:D3"}, {"b:B3", "d:D3"}}},
	{3, 12000, [][]string{{"b:B1", "d:D3"}}},
}

// TestRunPatternFollowedByWHarness583ScenarioReplay replays the
// scenario and pins the complete record surface the oracle produces:
// 29 listener records across the sixteen legs in Java delivery order
// (Esper dispatches one send's completions to listeners newest-
// statement-first, which the runner mirrors by reversing each send's
// deliveries) — legs 1/2/5/6 firing AT B1 (t=2000) with the vacant d
// cell rendered {"state":"null"}, the three chain legs at A2 (t=5000),
// leg 12 contributing zero records, and the duplicated multiset rows
// of legs 10 (two B3/D3 rows at t=12000), 15 (three A2/B3 rows at
// t=10000) and 16 (two A2/B3 rows).
func TestRunPatternFollowedByWHarness583ScenarioReplay(t *testing.T) {
	scenario := loadPatternFollowedByWHarness583ScenarioForTest(t)
	trace, err := runPatternFollowedByWHarness583Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternFollowedByWHarness583ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternFollowedByWHarness583ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternFollowedByWHarness583ExpectedRecords), trace.Records)
	}
	for index, want := range patternFollowedByWHarness583ExpectedRecords {
		record := trace.Records[index]
		wantStatement := patternFollowedByWHarness583LegName(patternFollowedByWHarness583LegAtoms[want.leg-1])
		if record.Case != "w-harness" || record.Operation != "listener" || record.Statement != wantStatement {
			t.Fatalf("record %d = %+v, want leg %d statement %q",
				index, record, want.leg, wantStatement)
		}
		wantTime := time.UnixMilli(want.atMS).UTC()
		if record.Time != compat.FormatTraceTime(wantTime) {
			t.Fatalf("record %d time = %q, want %q", index, record.Time, compat.FormatTraceTime(wantTime))
		}
		if len(record.Old) != 0 {
			t.Fatalf("record %d delivery should be new-only: %v", index, record.Old)
		}
		got := patternFollowedByWHarness583FlattenRows(t, record.New)
		if !reflect.DeepEqual(got, want.rows) {
			t.Fatalf("record %d (leg %d) rows = %v, want %v", index, want.leg, got, want.rows)
		}
	}
}

// patternFollowedByWHarness583FlattenRows flattens each normalized new
// row into "tag:id" strings; an unbound tag renders "tag:~" for the
// {"state":"null"} marker the Java oracle emits for the vacant cell.
func patternFollowedByWHarness583FlattenRows(t *testing.T, rows []compat.ResultRecord) [][]string {
	t.Helper()
	flattened := make([][]string, 0, len(rows))
	for rowIndex, row := range rows {
		if row.Kind != "row" {
			t.Fatalf("row %d kind = %q", rowIndex, row.Kind)
		}
		fields, err := json.Marshal(row.Fields)
		if err != nil {
			t.Fatalf("marshal row %d: %v", rowIndex, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(fields, &decoded); err != nil {
			t.Fatalf("decode row %d: %v", rowIndex, err)
		}
		// Deterministic tag order: the pinned tags per row.
		tags := make([]string, 0, len(decoded))
		for tag := range decoded {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		flatten := make([]string, 0, len(tags))
		for _, tag := range tags {
			switch value := decoded[tag].(type) {
			case map[string]any:
				if value["state"] == "null" {
					flatten = append(flatten, tag+":~")
					continue
				}
				beanFields, ok := value["fields"].(map[string]any)
				if !ok {
					t.Fatalf("row %d tag %s has no fields: %v", rowIndex, tag, value)
				}
				flatten = append(flatten, fmt.Sprintf("%s:%v", tag, beanFields["id"]))
			default:
				t.Fatalf("row %d tag %s unexpected value %v", rowIndex, tag, value)
			}
		}
		flattened = append(flattened, flatten)
	}
	return flattened
}

// TestRunPatternFollowedByWHarness583PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — one
// case block, thirteen advance-times (ON_START=0 plus one per event),
// sixteen deploys, twelve sends and one undeploy-all.
func TestRunPatternFollowedByWHarness583PinnedArtifacts(t *testing.T) {
	scenario := loadPatternFollowedByWHarness583ScenarioForTest(t)
	if scenario.ID != patternFollowedByWHarness583ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 43 {
		t.Fatalf("steps = %d, want 43", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "w-harness" {
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
		default:
			t.Fatalf("unexpected op %q", step.Op)
		}
	}
	if cases != 1 || deploys != 16 || sends != 12 || advances != 13 || undeploys != 1 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternFollowedByWHarness583RejectsMalformedRawScenario
// verifies the loader pins the step sequence: a mutated atom (dropped
// `()`, changed followed-by edge, dropped dup-triggering structure), a
// mutated advance-time instant or a mutated send payload is rejected.
func TestRunPatternFollowedByWHarness583RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternFollowedByWHarness583ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"leg 13 dropped parens marker", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("b=SupportBean_B() -> a_2=SupportBean_A())"),
				[]byte("b=SupportBean_B -> a_2=SupportBean_A())"), 1)
		}},
		{"leg 2 bounded edge", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("b=SupportBean_B -[1000]> (d=SupportBean_D"),
				[]byte("b=SupportBean_B -[100]> (d=SupportBean_D"), 1)
		}},
		{"leg 15 double-space atom", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("every ( every a=SupportBean_A -> every b=SupportBean_B)"),
				[]byte("every (every a=SupportBean_A -> every b=SupportBean_B)"), 1)
		}},
		{"advance-time instant", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:06Z"`),
				[]byte(`"at": "1970-01-01T00:00:06.500Z"`), 1)
		}},
		{"send payload id", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"eventType": "SupportBean_D", "payload": {"id": "D2"}`),
				[]byte(`"eventType": "SupportBean_D", "payload": {"id": "D9"}`), 1)
		}},
		{"deploy statement name", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"statement": "name--b=SupportBean_B -> d=SupportBean_D"`),
				[]byte(`"statement": "name--b=SupportBean_B -> every d=SupportBean_D"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternFollowedByWHarness583Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternFollowedByWHarness583RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports plus the per-execution static id: the single w-harness case
// owns ord 0's java-runtime-d896ea164e3cebd5c27e and static
// java-971bf7dfac84756cdd0a.
func TestRunPatternFollowedByWHarness583RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{0}
	if len(patternFollowedByWHarness583CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternFollowedByWHarness583CaseSpecs))
	}
	for index, spec := range patternFollowedByWHarness583CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != index {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternFollowedByWHarness583JavaStaticIDs {
		if id != "java-971bf7dfac84756cdd0a" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
	if len(patternFollowedByWHarness583CaseSpecs[0].epls) != 16 {
		t.Fatalf("w-harness epls = %d, want 16", len(patternFollowedByWHarness583CaseSpecs[0].epls))
	}
}

func patternFollowedByWHarness583ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-followedby-wharness-583.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat scenario: %v", err)
	}
	return path
}

func loadPatternFollowedByWHarness583ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternFollowedByWHarness583ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer func() { _ = file.Close() }()
	scenario, err := loadPatternFollowedByWHarness583Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
