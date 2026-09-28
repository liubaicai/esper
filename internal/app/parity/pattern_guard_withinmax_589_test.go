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

// patternGuardWithinMax589ExpectedRecord pins one listener record: the
// leg (0..16, statement S<leg>), the delivery instant in epoch millis
// (the trigger event's external-clock time, shared by deadline
// expiries attributed to the upcoming bucket) and the flattened rows
// as "tag:id" pairs.
type patternGuardWithinMax589ExpectedRecord struct {
	leg  int
	atMS int64
	rows [][]string
}

// Records are pinned in the Go trace's emitted order: the Go engine
// dispatches one send's completions in deployment order (S0..S16) with
// advance-phase expiries processed inside the preceding advance; the
// -diff normalizer sorts both traces into (case, time, statement)
// buckets because Esper's cross-statement dispatch order inside a send
// is unspecified, and within a batch the rows compare as a multiset.
var patternGuardWithinMax589ExpectedRecords = []patternGuardWithinMax589ExpectedRecord{
	// B1@2000: the S0/S2 deadlines already expired inside this advance;
	// S4's cap-0 and S11's cap-0 suppress their completions.
	{1, 2000, [][]string{{"b:B1"}}},
	{5, 2000, [][]string{{"b:B1"}}},
	{6, 2000, [][]string{{"b:B1"}}},
	{7, 2000, [][]string{{"b:B1"}}},
	{8, 2000, [][]string{{"b:B1"}}},
	{9, 2000, [][]string{{"b:B1"}}},
	{10, 2000, [][]string{{"b:B1"}}},
	// B2@4000: S5 already spent its cap-1 completion at B1; the
	// every-inside-guard legs S7..S10 fire their per-instance rows.
	{6, 4000, [][]string{{"b:B2"}}},
	{7, 4000, [][]string{{"b:B2"}}},
	{8, 4000, [][]string{{"b:B2"}}},
	{9, 4000, [][]string{{"b:B2"}}},
	{10, 4000, [][]string{{"b:B2"}}},
	// D1@6000: B1's S12 instance died at its exact 4000 deadline inside
	// this advance; the every-d legs pair both open b spawns with D1.
	{12, 6000, [][]string{{"b:B2", "d:D1"}}},
	{13, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{14, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{15, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{16, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	// D2@9000.
	{13, 9000, [][]string{{"b:B1", "d:D2"}, {"b:B2", "d:D2"}}},
	{14, 9000, [][]string{{"b:B1", "d:D2"}, {"b:B2", "d:D2"}}},
	{15, 9000, [][]string{{"b:B1", "d:D2"}, {"b:B2", "d:D2"}}},
	// B3@10000: the 2.001s-respawned S7 instance and the per-instance
	// caps of S8..S10 all still cover B3.
	{3, 10000, [][]string{{"b:B3"}}},
	{7, 10000, [][]string{{"b:B3"}}},
	{8, 10000, [][]string{{"b:B3"}}},
	{9, 10000, [][]string{{"b:B3"}}},
	{10, 10000, [][]string{{"b:B3"}}},
	// D3@12000: B3's S12 instance binds D3; the cap-2 S15 every-d
	// instances for B1/B2 already spent both completions, so only the
	// B3 spawn fires; S16's cap-1 instances likewise kept only D1.
	{12, 12000, [][]string{{"b:B3", "d:D3"}}},
	{13, 12000, [][]string{{"b:B1", "d:D3"}, {"b:B2", "d:D3"}, {"b:B3", "d:D3"}}},
	{14, 12000, [][]string{{"b:B1", "d:D3"}, {"b:B2", "d:D3"}, {"b:B3", "d:D3"}}},
	{15, 12000, [][]string{{"b:B3", "d:D3"}}},
	{16, 12000, [][]string{{"b:B3", "d:D3"}}},
}

// TestRunPatternGuardWithinMax589ScenarioReplay replays the scenario
// and pins the complete record surface: 30 listener records across the
// 17 legs — the deadline-exact expiries attributed to the upcoming
// send's bucket, the silent legs S0/S2/S4/S11 contributing nothing,
// and the guard-over-every vs guard-inside-every nesting distinction
// separating S4..S6's global caps from S7..S11's per-instance caps.
func TestRunPatternGuardWithinMax589ScenarioReplay(t *testing.T) {
	scenario := loadPatternGuardWithinMax589ScenarioForTest(t)
	trace, err := runPatternGuardWithinMax589Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternGuardWithinMax589ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternGuardWithinMax589ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternGuardWithinMax589ExpectedRecords), trace.Records)
	}
	for index, want := range patternGuardWithinMax589ExpectedRecords {
		record := trace.Records[index]
		wantStatement := patternGuardWithinMax589LegName(want.leg)
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
		got := patternGuardWithinMax589FlattenRows(t, record.New)
		if !reflect.DeepEqual(got, want.rows) {
			t.Fatalf("record %d (leg %d) rows = %v, want %v", index, want.leg, got, want.rows)
		}
	}
}

// patternGuardWithinMax589FlattenRows flattens each normalized new row
// into "tag:id" strings; every projected tag here is a single bean
// fragment.
func patternGuardWithinMax589FlattenRows(t *testing.T, rows []compat.ResultRecord) [][]string {
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
			value, ok := decoded[tag].(map[string]any)
			if !ok {
				t.Fatalf("row %d tag %s unexpected value %v", rowIndex, tag, decoded[tag])
			}
			beanFields, ok := value["fields"].(map[string]any)
			if !ok {
				t.Fatalf("row %d tag %s has no fields: %v", rowIndex, tag, value)
			}
			flatten = append(flatten, fmt.Sprintf("%s:%v", tag, beanFields["id"]))
		}
		flattened = append(flattened, flatten)
	}
	return flattened
}

// TestRunPatternGuardWithinMax589RejectsMalformedRawScenario pins the
// negative path of the new loader validation: every flipped
// byte-discriminant must be refused instead of silently drifting from
// the pinned oracle text or step contract.
func TestRunPatternGuardWithinMax589RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternGuardWithinMax589ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"leg ordinal drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"ordinal": 0`),
				[]byte(`"ordinal": 1`), 1)
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
		{"cap drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:withinmax(1999 msec,10)`),
				[]byte(`timer:withinmax(1999 msec,11)`), 1)
		}},
		{"every placement drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`every b=SupportBean_B where timer:withinmax(2.001, 4)`),
				[]byte(`(every b=SupportBean_B) where timer:withinmax(2.001, 4)`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mutated := mutate.mutate(raw)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("mutation %s produced no change", mutate.name)
			}
			if _, err := loadPatternGuardWithinMax589Scenario(bytes.NewReader(mutated)); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternGuardWithinMax589PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts —
// one case block, ONE deploy-all step, twelve fused advance-before-
// send steps and one undeploy-all — and the 17 leg EPLs byte-exact.
func TestRunPatternGuardWithinMax589PinnedArtifacts(t *testing.T) {
	scenario := loadPatternGuardWithinMax589ScenarioForTest(t)
	if scenario.ID != patternGuardWithinMax589ID {
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
	if len(patternGuardWithinMax589CaseSpecs) != 1 ||
		len(patternGuardWithinMax589CaseSpecs[0].epls) != 17 {
		t.Fatalf("case specs = %+v", patternGuardWithinMax589CaseSpecs)
	}
	for leg, epl := range patternGuardWithinMax589CaseSpecs[0].epls {
		want := patternGuardWithinMax589StatementText(leg, patternGuardWithinMax589LegAtoms[leg])
		if epl != want {
			t.Fatalf("epl %d = %q, want %q", leg, epl, want)
		}
	}
}

// TestRunPatternGuardWithinMax589RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports
// plus the per-execution static id: the single w-harness case owns ord
// 0's java-runtime-78d9e9fcf78678c48ff9 and static
// java-0c023cc183e0d8221a0b.
func TestRunPatternGuardWithinMax589RuntimeIDMappingMatchesScenario(t *testing.T) {
	spec := patternGuardWithinMax589CaseSpecs[0]
	if spec.ordinal != 0 || spec.runtimeIndex != 0 {
		t.Fatalf("case mapping = ordinal %d runtimeIndex %d", spec.ordinal, spec.runtimeIndex)
	}
	if !reflect.DeepEqual(patternGuardWithinMax589JavaRuntimeIDs,
		[]string{"java-runtime-78d9e9fcf78678c48ff9"}) {
		t.Fatalf("runtime ids = %v", patternGuardWithinMax589JavaRuntimeIDs)
	}
	if !reflect.DeepEqual(patternGuardWithinMax589JavaStaticIDs,
		[]string{"java-0c023cc183e0d8221a0b"}) {
		t.Fatalf("static ids = %v", patternGuardWithinMax589JavaStaticIDs)
	}
	if !reflect.DeepEqual(patternGuardWithinMax589JavaExecutions,
		[]string{"PatternGuardTimerWithinOrMax"}) {
		t.Fatalf("executions = %v", patternGuardWithinMax589JavaExecutions)
	}
}

func patternGuardWithinMax589ScenarioPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "parity", patternGuardWithinMax589ID+".json")
}

func loadPatternGuardWithinMax589ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternGuardWithinMax589ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternGuardWithinMax589Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
