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

// patternTimerIntervalWHarness592ExpectedRecord pins one listener
// record: the leg (0..30, statement S<leg>), the delivery instant in
// epoch millis (the trigger event's external-clock time, shared by
// timer expiries attributed to the upcoming bucket) and the flattened
// rows as "tag:id" pairs — "tag:~" marks the unbound-tag null cell
// Java renders as {"state":"null"} and an empty row an untagged
// timer-observer fire.
type patternTimerIntervalWHarness592ExpectedRecord struct {
	leg  int
	atMS int64
	rows [][]string
}

// Records are pinned in the Go trace's emitted order: the Go engine
// dispatches one send's completions in deployment order (S0..S30)
// with advance-phase timer expiries delivered inside the preceding
// advance; the -diff normalizer sorts both traces into (case, time,
// statement) buckets because Esper's cross-statement dispatch order
// inside a send is unspecified, and within a batch the rows compare
// as a multiset.
var patternTimerIntervalWHarness592ExpectedRecords = []patternTimerIntervalWHarness592ExpectedRecord{
	// B1@2000: the S20 1.001 or-timer and the S2/S1/S0 timers expired
	// inside this advance (the Go engine drains advance-phase timer
	// callbacks in reverse deployment order); S12's pre-armed zero
	// timer and S14's same-send zero timer complete with B1; S21's
	// b wins its or.
	{20, 2000, [][]string{{"b:~"}}},
	{2, 2000, [][]string{{}}},
	{1, 2000, [][]string{{}}},
	{0, 2000, [][]string{{}}},
	{12, 2000, [][]string{{"b:B1"}}},
	{14, 2000, [][]string{{"b:B1"}}},
	{21, 2000, [][]string{{"b:B1"}}},
	// C1@3000: B1's S15/S13 armed timers and the S5/S4/S3 timers
	// expired inside this advance.
	{15, 3000, [][]string{{"b:B1"}}},
	{13, 3000, [][]string{{"b:B1"}}},
	{5, 3000, [][]string{{}}},
	{4, 3000, [][]string{{}}},
	{3, 3000, [][]string{{}}},
	// B2@4000: the S25 and-timer, B1's S16 timer, S7's first every
	// expiry and S6's 3.001-second timer expired inside this advance;
	// S9/S10's timers armed b just in time and the S28 filtered b
	// completes the long-dead and.
	{25, 4000, [][]string{{"b:B1"}}},
	{16, 4000, [][]string{{"b:B1"}}},
	{7, 4000, [][]string{{}}},
	{6, 4000, [][]string{{}}},
	{9, 4000, [][]string{{"b:B2"}}},
	{10, 4000, [][]string{{"b:B2"}}},
	{28, 4000, [][]string{{"b:B2"}}},
	// A2@5000: the S26 and-timer and S8's first every expiry.
	{26, 5000, [][]string{{"b:B1"}}},
	{8, 5000, [][]string{{}}},
	// D1@6000: both every-followed-by legs pair B1 with D1 — S19's
	// 2.000 timer expired at this exact advance instant.
	{18, 6000, [][]string{{"b:B1", "d:D1"}}},
	{19, 6000, [][]string{{"b:B1", "d:D1"}}},
	// F1@8000: the S23 7.500 or-timer and S7's re-armed every expiry.
	{23, 8000, [][]string{{}}},
	{7, 8000, [][]string{{}}},
	// D2@9000: the S22 8.500 or-timer expired inside this advance
	// leaving b unbound; S17's 6.000 timer armed d inside the E1
	// advance and completes on the D2 send.
	{22, 9000, [][]string{{"b:~"}}},
	{17, 9000, [][]string{{"b:B1", "d:D2"}}},
	// B3@10000: S8's re-armed every expiry and S11's timer-armed b.
	{8, 10000, [][]string{{}}},
	{11, 10000, [][]string{{"b:B3"}}},
	// G1@11000: the g atom wins S24's or.
	{24, 11000, [][]string{{"g:G1"}}},
	// D3@12000: S7's third re-armed expiry and S19's respawned B3
	// instance whose 2.000 timer expired at this exact instant.
	{7, 12000, [][]string{{}}},
	{19, 12000, [][]string{{"b:B3", "d:D3"}}},
}

// TestRunPatternTimerIntervalWHarness592ScenarioReplay replays the
// scenario and pins the complete record surface: 32 listener records
// across the 31 legs — the advance-phase timer expiries attributed
// to the upcoming send's bucket, the every-interval re-arms at
// advance instants (S7's B2/F1/D3, S8's A2/B3), the silent legs
// S27/S29/S30 contributing nothing, and the or-timer wins of
// S20/S22 leaving their b tags unbound.
func TestRunPatternTimerIntervalWHarness592ScenarioReplay(t *testing.T) {
	scenario := loadPatternTimerIntervalWHarness592ScenarioForTest(t)
	trace, err := runPatternTimerIntervalWHarness592Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternTimerIntervalWHarness592ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternTimerIntervalWHarness592ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternTimerIntervalWHarness592ExpectedRecords), trace.Records)
	}
	for index, want := range patternTimerIntervalWHarness592ExpectedRecords {
		record := trace.Records[index]
		wantStatement := patternTimerIntervalWHarness592LegName(want.leg)
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
		got := patternTimerIntervalWHarness592FlattenRows(t, record.New)
		if !reflect.DeepEqual(got, want.rows) {
			t.Fatalf("record %d (leg %d) rows = %v, want %v", index, want.leg, got, want.rows)
		}
	}
}

// patternTimerIntervalWHarness592FlattenRows flattens each normalized
// new row into "tag:id" strings; an unbound tag renders "tag:~" for
// the {"state":"null"} marker and the untagged timer-observer fires
// render an empty row.
func patternTimerIntervalWHarness592FlattenRows(t *testing.T, rows []compat.ResultRecord) [][]string {
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
				t.Fatalf("row %d tag %s unexpected value %v", rowIndex, tag, decoded[tag])
			}
		}
		flattened = append(flattened, flatten)
	}
	return flattened
}

// TestRunPatternTimerIntervalWHarness592RejectsMalformedRawScenario
// pins the negative path of the new loader validation: every flipped
// byte-discriminant must be refused instead of silently drifting from
// the pinned oracle text or step contract.
func TestRunPatternTimerIntervalWHarness592RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternTimerIntervalWHarness592ScenarioPath(t))
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
		{"interval drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(2999 milliseconds)`),
				[]byte(`timer:interval(2998 milliseconds)`), 1)
		}},
		{"within spacing drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:within (3.000)`),
				[]byte(`timer:within(3.000)`), 1)
		}},
		{"or drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`b=SupportBean_B() or timer:interval(1.001)`),
				[]byte(`b=SupportBean_B() or timer:interval(1.002)`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mutated := mutate.mutate(raw)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("mutation %s produced no change", mutate.name)
			}
			if _, err := loadPatternTimerIntervalWHarness592Scenario(bytes.NewReader(mutated)); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternTimerIntervalWHarness592PinnedArtifacts pins the
// scenario file's Java identity plus the step sequence the loader
// accepts — one case block, ONE deploy-all step, twelve fused
// advance-before-send steps and one undeploy-all — and the 31 leg
// EPLs byte-exact.
func TestRunPatternTimerIntervalWHarness592PinnedArtifacts(t *testing.T) {
	scenario := loadPatternTimerIntervalWHarness592ScenarioForTest(t)
	if scenario.ID != patternTimerIntervalWHarness592ID {
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
	if len(patternTimerIntervalWHarness592CaseSpecs) != 1 ||
		len(patternTimerIntervalWHarness592CaseSpecs[0].epls) != 31 {
		t.Fatalf("case specs = %+v", patternTimerIntervalWHarness592CaseSpecs)
	}
	for leg, epl := range patternTimerIntervalWHarness592CaseSpecs[0].epls {
		want := patternTimerIntervalWHarness592StatementText(leg, patternTimerIntervalWHarness592LegAtoms[leg])
		if epl != want {
			t.Fatalf("epl %d = %q, want %q", leg, epl, want)
		}
	}
}

// TestRunPatternTimerIntervalWHarness592RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports plus the per-execution static id: the single w-harness case
// owns ord 0's java-runtime-9b41fb5951301c0de979 and static
// java-1422565b568236b2bfea.
func TestRunPatternTimerIntervalWHarness592RuntimeIDMappingMatchesScenario(t *testing.T) {
	spec := patternTimerIntervalWHarness592CaseSpecs[0]
	if spec.ordinal != 0 || spec.runtimeIndex != 0 {
		t.Fatalf("case mapping = ordinal %d runtimeIndex %d", spec.ordinal, spec.runtimeIndex)
	}
	if !reflect.DeepEqual(patternTimerIntervalWHarness592JavaRuntimeIDs,
		[]string{"java-runtime-9b41fb5951301c0de979"}) {
		t.Fatalf("runtime ids = %v", patternTimerIntervalWHarness592JavaRuntimeIDs)
	}
	if !reflect.DeepEqual(patternTimerIntervalWHarness592JavaStaticIDs,
		[]string{"java-1422565b568236b2bfea"}) {
		t.Fatalf("static ids = %v", patternTimerIntervalWHarness592JavaStaticIDs)
	}
	if !reflect.DeepEqual(patternTimerIntervalWHarness592JavaExecutions,
		[]string{"PatternOp"}) {
		t.Fatalf("executions = %v", patternTimerIntervalWHarness592JavaExecutions)
	}
}

func patternTimerIntervalWHarness592ScenarioPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "parity", patternTimerIntervalWHarness592ID+".json")
}

func loadPatternTimerIntervalWHarness592ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternTimerIntervalWHarness592ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternTimerIntervalWHarness592Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
