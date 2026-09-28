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
//   - ord 0 PatternOpWHarness — covered by pattern-followedby-wharness-583.
//   - ords 1/6/9 PatternFollowedByWithNot/PatternFollowedNotEvery/
//     PatternFollowedOrPermFalse — covered by
//     pattern-followedby-timernot-581.
//   - ords 3/4/5 PatternMemoryRFIDEvent/PatternRFIDZoneExit/
//     PatternRFIDZoneEnter — covered by pattern-followedby-rfid-580.
//   - ords 7/8 PatternFollowedEveryMultiple/PatternFilterGreaterThen —
//     covered by pattern-followedby-chain-582.
//   - manifest dispositions are main-agent owned.

// TestRunPatternFollowedByTimer584ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: exactly 2 listener deliveries for the single timer case,
// both at the epoch instant (no clock ops — the Java execution performs
// no sendTimer/advanceTime calls and the ~83-day timer:within guard
// never binds). The first delivery carries ONE row {A:e1,B:e2}; the
// second is the discriminant — ONE invocation carrying TWO rows
// [{A:e1,B:e3},{A:e2,B:e3}], e3's 38100ms start falling inside both
// armed windows ([0:41200] and [24100:65400]). Every delivered row
// projects BOTH bound tags as the full five-property SupportCallEvent
// fragment — callId/source/dest/startTime/endTime — matching the Java
// assertSame bean-identity assertion under select *.
func TestRunPatternFollowedByTimer584ScenarioReplay(t *testing.T) {
	scenario := loadPatternFollowedByTimer584ScenarioForTest(t)
	trace, err := runPatternFollowedByTimer584Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternFollowedByTimer584ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(trace.Records))
	}
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
	e1 := map[string]any{"callId": float64(2000002601), "source": "18",
		"dest": "123456789014795", "startTime": float64(0), "endTime": float64(41200)}
	e2 := map[string]any{"callId": float64(2000002607), "source": "20",
		"dest": "123456789014795", "startTime": float64(24100), "endTime": float64(65400)}
	e3 := map[string]any{"callId": float64(2000002610), "source": "22",
		"dest": "123456789014795", "startTime": float64(38100), "endTime": float64(78900)}
	want := []struct {
		sequence uint64
		rows     [][2]map[string]any // per row: {A fragment, B fragment}
	}{
		{1, [][2]map[string]any{{e1, e2}}},
		{2, [][2]map[string]any{{e1, e3}, {e2, e3}}},
	}
	for index, record := range trace.Records {
		if record.Case != "timer" || record.Operation != "listener" || record.Statement != "s0" {
			t.Fatalf("record %d = %+v", index, record)
		}
		if record.Sequence != want[index].sequence {
			t.Fatalf("record %d sequence = %d, want %d", index, record.Sequence, want[index].sequence)
		}
		// The engine clock never advances — every delivery is stamped at
		// the epoch, matching the oracle's initialize(0L) runtime.
		if record.Time != "1970-01-01T00:00:00Z" {
			t.Fatalf("record %d time = %q", index, record.Time)
		}
		if len(record.Old) != 0 {
			t.Fatalf("delivery %d should be new-only: %v", index, record.Old)
		}
		rows := recordRows(record)
		if len(rows) != len(want[index].rows) {
			t.Fatalf("record %d rows = %v, want %d", index, rows, len(want[index].rows))
		}
		for rowIndex, wantPair := range want[index].rows {
			for tagIndex, tag := range [2]string{"A", "B"} {
				fields := tagFields(rows, rowIndex, tag)
				if fields == nil {
					t.Fatalf("record %d row %d = %v, want %s fragment", index, rowIndex, rows, tag)
				}
				if !reflect.DeepEqual(fields, wantPair[tagIndex]) {
					t.Fatalf("record %d row %d tag %s fields = %v, want %v",
						index, rowIndex, tag, fields, wantPair[tagIndex])
				}
			}
		}
	}
}

// TestRunPatternFollowedByTimer584PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — one
// case block, one s0 deploy, three SupportCallEvent sends, zero
// advance-time steps and one undeploy-all.
func TestRunPatternFollowedByTimer584PinnedArtifacts(t *testing.T) {
	scenario := loadPatternFollowedByTimer584ScenarioForTest(t)
	if scenario.ID != patternFollowedByTimer584ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 6 {
		t.Fatalf("steps = %d, want 6", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "timer" {
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
	if cases != 1 || deploys != 1 || undeploys != 1 || sends != 3 || advances != 0 {
		t.Fatalf("cases/deploys/sends/advances/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, sends, advances, undeploys)
	}
}

// TestRunPatternFollowedByTimer584RejectsMalformedRawScenario verifies
// the loader pins the step sequence: the byte-exact discriminant
// mutations — a space inserted between "]" and the statement-level
// `where`, the space dropped inside `timer:within (7200000)`, a mutated
// range predicate — and a mutated send payload are rejected.
func TestRunPatternFollowedByTimer584RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternFollowedByTimer584ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"statement-where spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`where timer:within (7200000)]where B.source != A.source`),
				[]byte(`where timer:within (7200000)] where B.source != A.source`), 1)
		}},
		{"timer-within spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`where timer:within (7200000)]where`),
				[]byte(`where timer:within(7200000)]where`), 1)
		}},
		{"range predicate", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`startTime in [A.startTime:A.endTime]`),
				[]byte(`startTime in [A.startTime:A.endTime)`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"callId": 2000002610, "source": "22", "dest": "123456789014795", "startTime": 38100, "endTime": 78900}`),
				[]byte(`"payload": {"callId": 2000002610, "source": "18", "dest": "123456789014795", "startTime": 38100, "endTime": 78900}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternFollowedByTimer584Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternFollowedByTimer584RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// the single timer case owns ord 2's
// java-runtime-4759bc801b8c0be6c10a and the per-execution static id
// java-b96c718a6895cd0d80de (the deduplicated inventory id
// java-089b2086c9945dff918f is not carried — the per-runtime row keeps
// the per-execution static-manifest id).
func TestRunPatternFollowedByTimer584RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{2}
	wantRuntimeIndex := []int{0}
	if len(patternFollowedByTimer584CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternFollowedByTimer584CaseSpecs))
	}
	for index, spec := range patternFollowedByTimer584CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternFollowedByTimer584JavaStaticIDs {
		if id != "java-b96c718a6895cd0d80de" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternFollowedByTimer584ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-followedby-timer-584.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternFollowedByTimer584ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternFollowedByTimer584ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternFollowedByTimer584Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
