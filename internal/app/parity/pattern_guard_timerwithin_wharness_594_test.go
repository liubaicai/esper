package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/liubaicai/esper/internal/compat"
)

// patternGuardTimerWithinWHarness594ExpectedRecord pins one listener
// record: the leg (0..33, statement S<leg>), the delivery instant in
// epoch millis and the flattened rows as "tag:id" pairs — "tag:~"
// marks the unbound-tag null cell Java renders as {"state":"null"}.
type patternGuardTimerWithinWHarness594ExpectedRecord struct {
	leg  int
	atMS int64
	rows [][]string
}

// Records are pinned in the Go trace's emitted order: the Go engine
// dispatches one send's completions in deployment order (S0..S33)
// with guard expiries inside the fused advance-before-send resolving
// before the event (an exclusive deadline lands exactly on the send
// instant kills the leg first). The -diff normalizer sorts both
// traces into (case, time, statement) buckets because Esper's
// cross-statement dispatch order inside a send is unspecified, and
// within a batch the rows compare as a multiset.
var patternGuardTimerWithinWHarness594ExpectedRecords = []patternGuardTimerWithinWHarness594ExpectedRecord{
	// B1@2000: the single-filter and every-guarded-within legs take B1;
	// S0 (within 2 sec, deadline == instant) and S21/S27's expired b
	// sides stay silent; S30's or completes with d unbound.
	{1, 2000, [][]string{{"b:B1"}}},
	{7, 2000, [][]string{{"b:B1"}}},
	{8, 2000, [][]string{{"b:B1"}}},
	{9, 2000, [][]string{{"b:B1"}}},
	{10, 2000, [][]string{{"b:B1"}}},
	{11, 2000, [][]string{{"b:B1"}}},
	{12, 2000, [][]string{{"b:B1"}}},
	{13, 2000, [][]string{{"b:B1"}}},
	{14, 2000, [][]string{{"b:B1"}}},
	{15, 2000, [][]string{{"b:B1"}}},
	{30, 2000, [][]string{{"b:B1", "d:~"}}},
	// B2@4000: the retained-every-within legs S8/S14 fire their second
	// B, S9/S10/S15's serial every retries match B2, and S11/S12's
	// nested every emits one row per live inner instance.
	{8, 4000, [][]string{{"b:B2"}}},
	{9, 4000, [][]string{{"b:B2"}}},
	{10, 4000, [][]string{{"b:B2"}}},
	{11, 4000, [][]string{{"b:B2"}, {"b:B2"}}},
	{12, 4000, [][]string{{"b:B2"}, {"b:B2"}}},
	{13, 4000, [][]string{{"b:B2"}}},
	{14, 4000, [][]string{{"b:B2"}}},
	{15, 4000, [][]string{{"b:B2"}}},
	// D1@6000: the sequence/and legs pair their retained B matches
	// with D1; S17/S20/S24's boundary deadlines died inside the
	// advance first; S31's or reports d only (b expired at the B1
	// instant) and S22's B1 instance expired at exactly 6000 so only
	// the B2 instance pairs D1.
	{16, 6000, [][]string{{"b:B1", "d:D1"}}},
	{18, 6000, [][]string{{"b:B1", "d:D1"}}},
	{19, 6000, [][]string{{"b:B1", "d:D1"}}},
	{22, 6000, [][]string{{"b:B2", "d:D1"}}},
	{23, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	{25, 6000, [][]string{{"b:B2", "d:D1"}}},
	{26, 6000, [][]string{{"b:B1", "d:D1"}}},
	{29, 6000, [][]string{{"b:B1", "d:D1"}}},
	{31, 6000, [][]string{{"b:~", "d:D1"}}},
	{32, 6000, [][]string{{"b:B1", "d:D1"}, {"b:B2", "d:D1"}}},
	// D2@9000: the and-of-every legs pair both retained Bs with D2.
	{23, 9000, [][]string{{"b:B1", "d:D2"}, {"b:B2", "d:D2"}}},
	{32, 9000, [][]string{{"b:B1", "d:D2"}, {"b:B2", "d:D2"}}},
	// B3@10000: the SODA leg S3 (10.001d double literal), the
	// long-window legs and the nested-every accumulations fire on
	// B3; S18/S29's every-and respawn retains D2 for the {B3,D2}
	// pair while S32 cross-products B3 against both retained Ds.
	{3, 10000, [][]string{{"b:B3"}}},
	{4, 10000, [][]string{{"b:B3"}}},
	{9, 10000, [][]string{{"b:B3"}}},
	{10, 10000, [][]string{{"b:B3"}}},
	{11, 10000, [][]string{{"b:B3"}, {"b:B3"}, {"b:B3"}, {"b:B3"}}},
	{12, 10000, [][]string{{"b:B3"}, {"b:B3"}, {"b:B3"}, {"b:B3"}}},
	{13, 10000, [][]string{{"b:B3"}}},
	{15, 10000, [][]string{{"b:B3"}}},
	{18, 10000, [][]string{{"b:B3", "d:D2"}}},
	{29, 10000, [][]string{{"b:B3", "d:D2"}}},
	{32, 10000, [][]string{{"b:B3", "d:D1"}, {"b:B3", "d:D2"}}},
	// D3@12000: the surviving sequence and and legs close on D3.
	{22, 12000, [][]string{{"b:B3", "d:D3"}}},
	{23, 12000, [][]string{{"b:B1", "d:D3"}, {"b:B2", "d:D3"}, {"b:B3", "d:D3"}}},
	{25, 12000, [][]string{{"b:B3", "d:D3"}}},
	{26, 12000, [][]string{{"b:B3", "d:D3"}}},
	{32, 12000, [][]string{{"b:B1", "d:D3"}, {"b:B2", "d:D3"}, {"b:B3", "d:D3"}}},
}

// TestRunPatternGuardTimerWithinWHarness594ScenarioReplay replays the
// scenario and pins the complete record surface: 47 listener records
// across the 34 legs — the eleven boundary-exclusive silent legs
// (S0, S2, S5, S6, S17, S20, S21, S24, S27, S28, S33) contribute
// nothing, S30's or wins with d unbound and S31's or reports only d.
func TestRunPatternGuardTimerWithinWHarness594ScenarioReplay(t *testing.T) {
	scenario := loadPatternGuardTimerWithinWHarness594ScenarioForTest(t)
	trace, err := runPatternGuardTimerWithinWHarness594Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternGuardTimerWithinWHarness594ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternGuardTimerWithinWHarness594ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternGuardTimerWithinWHarness594ExpectedRecords), trace.Records)
	}
	for index, want := range patternGuardTimerWithinWHarness594ExpectedRecords {
		record := trace.Records[index]
		wantStatement := patternGuardTimerWithinWHarness594LegName(want.leg)
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

// TestRunPatternGuardTimerWithinWHarness594RejectsMalformedRawScenario
// pins the negative path of the loader validation: every flipped
// byte-discriminant must be refused instead of silently drifting from
// the pinned oracle text or step contract.
func TestRunPatternGuardTimerWithinWHarness594RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternGuardTimerWithinWHarness594ScenarioPath(t))
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
		{"within interval drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:within(2 sec)`),
				[]byte(`timer:within(3 sec)`), 1)
		}},
		{"within spacing drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:within (2000 msec)`),
				[]byte(`timer:within (2000  msec)`), 1)
		}},
		{"soda literal drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:within(10.001d)`),
				[]byte(`timer:within(10.002d)`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mutated := mutate.mutate(raw)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("mutation %s produced no change", mutate.name)
			}
			if _, err := loadPatternGuardTimerWithinWHarness594Scenario(bytes.NewReader(mutated)); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternGuardTimerWithinWHarness594PinnedArtifacts pins the
// scenario file's Java identity plus the step sequence the loader
// accepts — one case block, ONE deploy-all step, twelve fused
// advance-before-send steps and one undeploy-all — and the 34 leg
// EPLs byte-exact.
func TestRunPatternGuardTimerWithinWHarness594PinnedArtifacts(t *testing.T) {
	scenario := loadPatternGuardTimerWithinWHarness594ScenarioForTest(t)
	if scenario.ID != patternGuardTimerWithinWHarness594ID {
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
	if len(patternGuardTimerWithinWHarness594CaseSpecs) != 1 ||
		len(patternGuardTimerWithinWHarness594CaseSpecs[0].epls) != 34 {
		t.Fatalf("case specs = %+v", patternGuardTimerWithinWHarness594CaseSpecs)
	}
	for leg, epl := range patternGuardTimerWithinWHarness594CaseSpecs[0].epls {
		want := patternGuardTimerWithinWHarness594StatementText(leg, patternGuardTimerWithinWHarness594LegAtoms[leg])
		if epl != want {
			t.Fatalf("epl %d = %q, want %q", leg, epl, want)
		}
	}
}

// TestRunPatternGuardTimerWithinWHarness594RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path
// reports plus the per-execution static id: the single w-harness case
// owns ord 0's java-runtime-bb8113cb979826cff927 and static
// java-0dec801a426fed297402.
func TestRunPatternGuardTimerWithinWHarness594RuntimeIDMappingMatchesScenario(t *testing.T) {
	spec := patternGuardTimerWithinWHarness594CaseSpecs[0]
	if spec.ordinal != 0 || spec.runtimeIndex != 0 {
		t.Fatalf("case mapping = ordinal %d runtimeIndex %d", spec.ordinal, spec.runtimeIndex)
	}
	if !reflect.DeepEqual(patternGuardTimerWithinWHarness594JavaRuntimeIDs,
		[]string{"java-runtime-bb8113cb979826cff927"}) {
		t.Fatalf("runtime ids = %v", patternGuardTimerWithinWHarness594JavaRuntimeIDs)
	}
	if !reflect.DeepEqual(patternGuardTimerWithinWHarness594JavaStaticIDs,
		[]string{"java-0dec801a426fed297402"}) {
		t.Fatalf("static ids = %v", patternGuardTimerWithinWHarness594JavaStaticIDs)
	}
	if !reflect.DeepEqual(patternGuardTimerWithinWHarness594JavaExecutions,
		[]string{"PatternOp"}) {
		t.Fatalf("executions = %v", patternGuardTimerWithinWHarness594JavaExecutions)
	}
}

func patternGuardTimerWithinWHarness594ScenarioPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "parity", patternGuardTimerWithinWHarness594ID+".json")
}

func loadPatternGuardTimerWithinWHarness594ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternGuardTimerWithinWHarness594ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer func() { _ = file.Close() }()
	scenario, err := loadPatternGuardTimerWithinWHarness594Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
