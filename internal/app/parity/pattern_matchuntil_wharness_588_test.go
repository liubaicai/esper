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

// patternMatchUntilWHarness588ExpectedRecord pins one listener record:
// the leg (0..51, statement S<leg>), the delivery instant in epoch
// millis (the trigger event's external-clock time, shared by timer
// expiries attributed to the upcoming bucket) and the flattened rows
// as "tag:id" pairs — "tag:~" marks the vacant null cell Java renders
// as {"state":"null"}, "tag:[...]" a match-until collected array and
// an empty row the untagged S44 delivery.
type patternMatchUntilWHarness588ExpectedRecord struct {
	leg  int
	atMS int64
	rows [][]string
}

// Records are pinned in the Go trace's emitted order: the Go engine
// dispatches one send's completions in deployment order (S0..S51) with
// advance-phase timer expiries delivered first; the -diff normalizer
// sorts both traces into (case, time, statement) buckets because
// Esper's cross-statement dispatch order inside a send is unspecified.
var patternMatchUntilWHarness588ExpectedRecords = []patternMatchUntilWHarness588ExpectedRecord{
	{2, 1000, [][]string{{"a:A1", "b:~"}}},
	{6, 1000, [][]string{{"a:A1", "d:~"}}},
	{10, 1000, [][]string{{"a:[A1]"}}},
	{11, 1000, [][]string{{"a:[A1]"}}},
	{22, 1000, [][]string{{"a:A1", "b:~"}}},
	{40, 2000, [][]string{{"b:B1", "d:~"}}},
	{41, 2000, [][]string{{"b:B1", "d:~"}}},
	{42, 2000, [][]string{{"b:B1", "d:~"}}},
	{45, 2000, [][]string{{"a:[A1]", "b:[B1]"}}},
	{33, 3000, [][]string{{"b:[B1]"}}},
	{34, 3000, [][]string{{"b:[B1]"}}},
	{49, 3000, [][]string{{"a:[A1]", "b:[B1]", "c:C1"}}},
	{17, 4000, [][]string{{"b:[B1,B2]", "g:~"}}},
	{40, 4000, [][]string{{"b:B2", "d:~"}}},
	{48, 4000, [][]string{{"a:[A1]", "b:[B1,B2]"}}},
	{8, 5000, [][]string{{"a:[A1,A2]"}}},
	{9, 5000, [][]string{{"a:[A1,A2]"}}},
	{14, 5000, [][]string{{"a:[A1,A2]", "b:[B1,B2]"}}},
	{25, 5000, [][]string{{"a:A2", "b:[B1,B2]"}}},
	{29, 5000, [][]string{{"a:A2", "b:[B1,B2]"}}},
	{46, 5000, [][]string{{"a:[A1,A2]"}}},
	{0, 6000, [][]string{{"a:[A2]"}}},
	{1, 6000, [][]string{{"a:[A1,A2]"}}},
	{5, 6000, [][]string{{"a:[A1,A2]", "b:[B1,B2]", "d:D1", "g:~"}}},
	{47, 6000, [][]string{{"a:[A1,A2]", "d:D1"}}},
	{39, 7000, [][]string{{"d:[D1]"}}},
	{32, 8000, [][]string{{"b:[B1,B2]"}}},
	{38, 9000, [][]string{{"b:[B1,B2]", "d:[D1,D2]"}}},
	{13, 10000, [][]string{{"b:[B1,B2,B3]"}}},
	{36, 10000, [][]string{{"b:[B1,B2,B3]", "d:~"}}},
	{40, 10000, [][]string{{"b:B3", "d:[D1,D2]"}}},
	{43, 11000, [][]string{{"a:[A1,A2]"}}},
	{18, 11000, [][]string{{"b:[B1,B2,B3]", "g:G1"}}},
	{19, 11000, [][]string{{"b:[B1,B2,B3]", "g:G1"}}},
	{20, 11000, [][]string{{"b:[B1,B2]", "g:G1"}}},
	{21, 11000, [][]string{{"b:[B1]", "g:G1"}}},
	{23, 11000, [][]string{{"b:[B1,B2,B3]", "g:G1"}}},
	{30, 11000, [][]string{{"b:[B1,B2,B3]"}}},
	{31, 11000, [][]string{{"b:[B1,B2]"}}},
	{50, 11000, [][]string{{"a:[A1,A2]", "b:[B1,B2,B3]", "g:G1"}}},
	{44, 12000, [][]string{{}}},
	{3, 12000, [][]string{{"b:[B1,B2,B3]"}}},
	{4, 12000, [][]string{{"a:[A1,A2]", "b:[B1,B2,B3]", "d:D3"}}},
	{35, 12000, [][]string{{"b:[B2,B3]", "c:C1", "d:D3"}}},
	{37, 12000, [][]string{{"b:~", "d:[D1,D2,D3]"}}},
}

// TestRunPatternMatchUntilWHarness588ScenarioReplay replays the
// scenario and pins the complete record surface: 45 listener records
// across the 52 legs — the timer legs S39/S43/S44 attributed to the
// E1/G1/D3 buckets whose advance expired them, the silent legs
// S7/S12/S15/S16/S24/S26/S27/S28 contributing nothing, S51's start
// fire not delivered to the listener, and the terminator's own tag
// absent from S30..S34's wildcard rows.
func TestRunPatternMatchUntilWHarness588ScenarioReplay(t *testing.T) {
	scenario := loadPatternMatchUntilWHarness588ScenarioForTest(t)
	trace, err := runPatternMatchUntilWHarness588Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternMatchUntilWHarness588ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternMatchUntilWHarness588ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternMatchUntilWHarness588ExpectedRecords), trace.Records)
	}
	for index, want := range patternMatchUntilWHarness588ExpectedRecords {
		record := trace.Records[index]
		wantStatement := patternMatchUntilWHarness588LegName(want.leg)
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
		got := patternMatchUntilWHarness588FlattenRows(t, record.New)
		if !reflect.DeepEqual(got, want.rows) {
			t.Fatalf("record %d (leg %d) rows = %v, want %v", index, want.leg, got, want.rows)
		}
	}
}

// patternMatchUntilWHarness588FlattenRows flattens each normalized new
// row into "tag:id" strings; an unbound tag renders "tag:~" for the
// {"state":"null"} marker, a collected match-until tag renders
// "tag:[id,id]", and the untagged S44 delivery renders an empty row.
func patternMatchUntilWHarness588FlattenRows(t *testing.T, rows []compat.ResultRecord) [][]string {
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
			case []any:
				ids := make([]string, 0, len(value))
				for _, element := range value {
					bean, ok := element.(map[string]any)
					if !ok {
						t.Fatalf("row %d tag %s element %v", rowIndex, tag, element)
					}
					beanFields, ok := bean["fields"].(map[string]any)
					if !ok {
						t.Fatalf("row %d tag %s element has no fields: %v", rowIndex, tag, bean)
					}
					ids = append(ids, fmt.Sprintf("%v", beanFields["id"]))
				}
				flatten = append(flatten, fmt.Sprintf("%s:[%v]", tag, joinIds(ids)))
			default:
				t.Fatalf("row %d tag %s unexpected value %v", rowIndex, tag, value)
			}
		}
		flattened = append(flattened, flatten)
	}
	return flattened
}

func joinIds(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

// TestRunPatternMatchUntilWHarness588RejectsMalformedRawScenario
// pins the negative path of the new loader validation: every flipped
// byte-discriminant must be refused instead of silently drifting from
// the pinned oracle text or step contract.
func TestRunPatternMatchUntilWHarness588RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternMatchUntilWHarness588ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"leg count drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"ordinal": 1`),
				[]byte(`"ordinal": 2`), 1)
		}},
		{"deploy statement drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"statement": "all"`),
				[]byte(`"statement": "s0"`), 1)
		}},
		{"send time drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:07Z"`),
				[]byte(`"at": "1970-01-01T00:00:06Z"`), 1)
		}},
		{"send payload drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"id": "D3"`),
				[]byte(`"id": "D4"`), 1)
		}},
		{"atom bound drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`[2:2] a=SupportBean_A`),
				[]byte(`[2:3] a=SupportBean_A`), 1)
		}},
		{"until guard drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`SupportBean_B until not SupportBean_B`),
				[]byte(`SupportBean_B until not SupportBean_C`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mutated := mutate.mutate(raw)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("mutation %s produced no change", mutate.name)
			}
			if _, err := loadPatternMatchUntilWHarness588Scenario(bytes.NewReader(mutated)); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternMatchUntilWHarness588PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts —
// one case block, ONE deploy-all step, twelve fused advance-before-
// send steps and one undeploy-all — and the 52 leg EPLs byte-exact.
func TestRunPatternMatchUntilWHarness588PinnedArtifacts(t *testing.T) {
	scenario := loadPatternMatchUntilWHarness588ScenarioForTest(t)
	if scenario.ID != patternMatchUntilWHarness588ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 15 {
		t.Fatalf("steps = %d, want 15", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "w-harness" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
			if step.Statement != "all" {
				t.Fatalf("deploy statement = %q", step.Statement)
			}
		case "send":
			sends++
		case "undeploy-all":
			undeploys++
		default:
			t.Fatalf("unexpected op %q", step.Op)
		}
	}
	if cases != 1 || deploys != 1 || sends != 12 || undeploys != 1 {
		t.Fatalf("cases/deploys/sends/undeploys = %d/%d/%d/%d",
			cases, deploys, sends, undeploys)
	}
	// The loader byte-validates the case's epls against the leg atoms;
	// pin the spec table itself.
	if len(patternMatchUntilWHarness588CaseSpecs) != 1 ||
		len(patternMatchUntilWHarness588CaseSpecs[0].epls) != 52 {
		t.Fatalf("case specs = %+v", patternMatchUntilWHarness588CaseSpecs)
	}
	for leg, epl := range patternMatchUntilWHarness588CaseSpecs[0].epls {
		want := patternMatchUntilWHarness588StatementText(leg, patternMatchUntilWHarness588LegAtoms[leg])
		if epl != want {
			t.Fatalf("epl %d = %q, want %q", leg, epl, want)
		}
	}
}

// TestRunPatternMatchUntilWHarness588RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports plus the per-execution static id: the single w-harness case
// owns ord 1's java-runtime-0383244f373c8a0ffc8b and static
// java-d4cb4534ca508be87595.
func TestRunPatternMatchUntilWHarness588RuntimeIDMappingMatchesScenario(t *testing.T) {
	spec := patternMatchUntilWHarness588CaseSpecs[0]
	if spec.ordinal != 1 || spec.runtimeIndex != 0 {
		t.Fatalf("case mapping = ordinal %d runtimeIndex %d", spec.ordinal, spec.runtimeIndex)
	}
	if !reflect.DeepEqual(patternMatchUntilWHarness588JavaRuntimeIDs,
		[]string{"java-runtime-0383244f373c8a0ffc8b"}) {
		t.Fatalf("runtime ids = %v", patternMatchUntilWHarness588JavaRuntimeIDs)
	}
	if !reflect.DeepEqual(patternMatchUntilWHarness588JavaStaticIDs,
		[]string{"java-d4cb4534ca508be87595"}) {
		t.Fatalf("static ids = %v", patternMatchUntilWHarness588JavaStaticIDs)
	}
	if !reflect.DeepEqual(patternMatchUntilWHarness588JavaExecutions,
		[]string{"PatternOp"}) {
		t.Fatalf("executions = %v", patternMatchUntilWHarness588JavaExecutions)
	}
}

func patternMatchUntilWHarness588ScenarioPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "parity", patternMatchUntilWHarness588ID+".json")
}

func loadPatternMatchUntilWHarness588ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternMatchUntilWHarness588ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternMatchUntilWHarness588Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
