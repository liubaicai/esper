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
//   - ords 1/6/9 PatternFollowedByWithNot/PatternFollowedNotEvery/
//     PatternFollowedOrPermFalse — covered by
//     pattern-followedby-timernot-581.
//   - manifest dispositions are main-agent owned.

// TestRunPatternFollowedByChain582ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: exactly 1 listener delivery across the two cases — the ord 7
// every-multiple discriminant (ONE invocation carrying TWO rows, the A1
// branch first then A2, both paired with the shared B1/C1/D1 tail and
// projecting the a/b/c/d fragments under select *). Ord 8
// filter-greater-then contributes ZERO records: both deploy phases —
// b.intPrimitive <= a.intPrimitive and the operand-flipped
// a.intPrimitive >= b.intPrimitive — are silent for the E1(10), E2(11)
// pair because the correlated second leg evaluates against the captured
// a event (ESPER-411).
func TestRunPatternFollowedByChain582ScenarioReplay(t *testing.T) {
	scenario := loadPatternFollowedByChain582ScenarioForTest(t)
	trace, err := runPatternFollowedByChain582Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternFollowedByChain582ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(trace.Records))
	}
	record := trace.Records[0]
	if record.Case != "every-multiple" || record.Operation != "listener" || record.Statement != "s0" {
		t.Fatalf("record = %+v", record)
	}
	if record.Sequence != 1 {
		t.Fatalf("record sequence = %d, want 1", record.Sequence)
	}
	if len(record.Old) != 0 {
		t.Fatalf("delivery should be new-only: %v", record.Old)
	}
	// ONE invocation carrying TWO rows, A1's branch first.
	data, err := json.Marshal(record.New)
	if err != nil {
		t.Fatalf("marshal new rows: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("decode new rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("new rows = %v, want 2 rows", rows)
	}
	wantIDs := [][4]string{{"A1", "B1", "C1", "D1"}, {"A2", "B1", "C1", "D1"}}
	for rowIndex, want := range wantIDs {
		fields, ok := rows[rowIndex]["fields"].(map[string]any)
		if !ok {
			t.Fatalf("row %d has no fields: %v", rowIndex, rows[rowIndex])
		}
		for tagIndex, tag := range [4]string{"a", "b", "c", "d"} {
			fragment, ok := fields[tag].(map[string]any)
			if !ok {
				t.Fatalf("row %d tag %s missing: %v", rowIndex, tag, fields)
			}
			beanFields, ok := fragment["fields"].(map[string]any)
			if !ok {
				t.Fatalf("row %d tag %s has no fields: %v", rowIndex, tag, fragment)
			}
			if beanFields["id"] != want[tagIndex] {
				t.Fatalf("row %d tag %s id = %v, want %s", rowIndex, tag, beanFields["id"], want[tagIndex])
			}
		}
	}
}

// TestRunPatternFollowedByChain582PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — two
// case blocks, three deploys (ord 8's single case deploys twice under
// undeploy-all), nine sends and three undeploy-alls.
func TestRunPatternFollowedByChain582PinnedArtifacts(t *testing.T) {
	scenario := loadPatternFollowedByChain582ScenarioForTest(t)
	if scenario.ID != patternFollowedByChain582ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 17 {
		t.Fatalf("steps = %d, want 17", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "every-multiple" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
		case "send":
			sends++
		case "undeploy-all":
			undeploys++
		default:
			t.Fatalf("unexpected op %q", step.Op)
		}
	}
	if cases != 2 || deploys != 3 || sends != 9 || undeploys != 3 {
		t.Fatalf("cases/deploys/sends/undeploys = %d/%d/%d/%d",
			cases, deploys, sends, undeploys)
	}
}

// TestRunPatternFollowedByChain582RejectsMalformedRawScenario verifies
// the loader pins the step sequence: a mutated EPL or send payload is
// rejected. The ord 8 mutations hit both discriminants — the spaceless
// `pattern[` phase-1 bracket versus the spaced `pattern [` phase-2
// bracket, and the operand-order flip (b.X <= a.X vs a.X >= b.X) — plus
// ord 7's chain order and payloads.
func TestRunPatternFollowedByChain582RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternFollowedByChain582ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"phase-1 bracket spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"epl": "@name('s0') select * from pattern[every`),
				[]byte(`"epl": "@name('s0') select * from pattern [every`), 1)
		}},
		{"phase-2 bracket spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"epl": "@name('s0') select * from pattern [every a=SupportBean -> b=SupportBean(a.`),
				[]byte(`"epl": "@name('s0') select * from pattern[every a=SupportBean -> b=SupportBean(a.`), 1)
		}},
		{"phase-1 operand order", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"epl": "@name('s0') select * from pattern[every a=SupportBean -> b=SupportBean(b.intPrimitive <= a.intPrimitive)]"`),
				[]byte(`"epl": "@name('s0') select * from pattern[every a=SupportBean -> b=SupportBean(a.intPrimitive <= b.intPrimitive)]"`), 1)
		}},
		{"phase-2 operand order", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"epl": "@name('s0') select * from pattern [every a=SupportBean -> b=SupportBean(a.intPrimitive >= b.intPrimitive)]"`),
				[]byte(`"epl": "@name('s0') select * from pattern [every a=SupportBean -> b=SupportBean(b.intPrimitive >= a.intPrimitive)]"`), 1)
		}},
		{"every-multiple chain tail", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("b=SupportBean_B -> c=SupportBean_C -> d=SupportBean_D"),
				[]byte("b=SupportBean_B -> d=SupportBean_D -> c=SupportBean_C"), 1)
		}},
		{"every-multiple send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"id": "A2"}`),
				[]byte(`"payload": {"id": "A3"}`), 1)
		}},
		{"filter-greater-then send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"theString": "E2", "intPrimitive": 11}`),
				[]byte(`"payload": {"theString": "E2", "intPrimitive": 9}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternFollowedByChain582Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternFollowedByChain582RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// each case owns its execution's runtimeId and ordinal, and ord 8's two
// deploy phases share that single runtimeId like the Java execution.
func TestRunPatternFollowedByChain582RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{7, 8}
	wantRuntimeIndex := []int{0, 1}
	if len(patternFollowedByChain582CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternFollowedByChain582CaseSpecs))
	}
	for index, spec := range patternFollowedByChain582CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternFollowedByChain582JavaStaticIDs {
		if id != "java-089b2086c9945dff918f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
	if !reflect.DeepEqual(patternFollowedByChain582CaseSpecs[1].epls,
		[]string{patternFollowedByChain582LessOrEqualEPL, patternFollowedByChain582GreaterOrEqEPL}) {
		t.Fatalf("filter-greater-then epls = %v", patternFollowedByChain582CaseSpecs[1].epls)
	}
}

func patternFollowedByChain582ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-followedby-chain-582.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat scenario: %v", err)
	}
	return path
}

func loadPatternFollowedByChain582ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternFollowedByChain582ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer func() { _ = file.Close() }()
	scenario, err := loadPatternFollowedByChain582Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
