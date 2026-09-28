package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Sibling note: ords 1/2/3/5/6 live in the 591 forms runner, ord 0 in
// the 592 W-harness and ord 4 was already differential-verified; this
// file pins ord 7 (java-runtime-6155a2b0181e2169f2a4).

// TestRunPatternTimerIntervalPropertyArray593ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record
// surface: exactly ONE listener delivery carrying ONE new row
// {a0id:"E1", a1id:"E2"} stamped at the 15000ms firing instant — the
// [2]-tag array supplies both ids and the 3+2-second arming delay.
func TestRunPatternTimerIntervalPropertyArray593ScenarioReplay(t *testing.T) {
	scenario := loadPatternTimerIntervalPropertyArray593ScenarioForTest(t)
	trace, err := runPatternTimerIntervalPropertyArray593Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternTimerIntervalPropertyArray593ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(trace.Records))
	}
	record := trace.Records[0]
	if record.Case != patternTimerIntervalPropertyArray593Case || record.Operation != "listener" ||
		record.Statement != "s0" || record.Sequence != 1 {
		t.Fatalf("record = %+v", record)
	}
	if record.Time != patternTimerIntervalPropertyArray593Fire {
		t.Fatalf("record time = %q, want %q", record.Time, patternTimerIntervalPropertyArray593Fire)
	}
	if len(record.Old) != 0 {
		t.Fatalf("delivery should be new-only: %v", record.Old)
	}
	if len(record.New) != 1 {
		t.Fatalf("record has %d new rows, want 1", len(record.New))
	}
	row := record.New[0]
	if row.Kind != "row" || row.Fields["a0id"] != "E1" || row.Fields["a1id"] != "E2" {
		t.Fatalf("record row = %+v, want {a0id:E1,a1id:E2}", row)
	}
}

// TestRunPatternTimerIntervalPropertyArray593PinnedArtifacts pins the
// scenario's Java identity plus the ordered step sequence the loader
// accepts: the arm advance, one s0 deploy, the two t=10000 sends, the
// silent and firing advance probes and undeploy-all.
func TestRunPatternTimerIntervalPropertyArray593PinnedArtifacts(t *testing.T) {
	scenario := loadPatternTimerIntervalPropertyArray593ScenarioForTest(t)
	if scenario.ID != patternTimerIntervalPropertyArray593ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 8 {
		t.Fatalf("steps = %d, want 8", len(scenario.Steps))
	}
	var cases, deploys, advances, sends, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
			if step.Epl != patternTimerIntervalPropertyArray593EPL {
				t.Fatalf("deploy epl = %q", step.Epl)
			}
		case "advance-time":
			advances++
		case "send":
			sends++
			if step.At != patternTimerIntervalPropertyArray593Send {
				t.Fatalf("send at = %q", step.At)
			}
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 1 || deploys != 1 || advances != 3 || sends != 2 || undeploys != 1 {
		t.Fatalf("cases/deploys/advances/sends/undeploys = %d/%d/%d/%d/%d",
			cases, deploys, advances, sends, undeploys)
	}
	// Step order: arm, deploy, E1, E2, silent probe, fire, undeploy.
	ops := []string{}
	for _, step := range scenario.Steps {
		ops = append(ops, step.Op)
	}
	want := []string{"case", "advance-time", "deploy", "send", "send", "advance-time", "advance-time", "undeploy-all"}
	for index := range want {
		if ops[index] != want[index] {
			t.Fatalf("step %d op = %q, want %q", index, ops[index], want[index])
		}
	}
}

// TestRunPatternTimerIntervalPropertyArray593RejectsMalformedRawScenario
// verifies the loader pins the byte-exact discriminants: the EPL's
// repeated-tag expression, the tag-array indexing, the send payload
// values (E1/E2 and 3/2) and the advance instants (arm/send/silent/fire).
func TestRunPatternTimerIntervalPropertyArray593RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternTimerIntervalPropertyArray593ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"interval expression", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(a[0].intPrimitive+a[1].intPrimitive seconds)`),
				[]byte(`timer:interval(a[0].intPrimitive-a[1].intPrimitive seconds)`), 1)
		}},
		{"tag-array index", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`a[1].theString as a1id`),
				[]byte(`a[0].theString as a1id`), 1)
		}},
		{"E1 payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"theString": "E1", "intPrimitive": 3}`),
				[]byte(`{"theString": "E1", "intPrimitive": 4}`), 1)
		}},
		{"E2 payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`{"theString": "E2", "intPrimitive": 2}`),
				[]byte(`{"theString": "E1", "intPrimitive": 2}`), 1)
		}},
		{"silent probe instant", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:14.999Z"`),
				[]byte(`"at": "1970-01-01T00:00:15Z"`), 1)
		}},
		{"send instant", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:10Z"`),
				[]byte(`"at": "1970-01-01T00:00:11Z"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternTimerIntervalPropertyArray593Scenario(
				bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternTimerIntervalPropertyArray593RuntimeIDMapping pins the
// case->runtime/execution/ordinal mapping the -diff path reports: ord 7
// owns runtime java-runtime-6155a2b0181e2169f2a4 with the shared static
// id java-1422565b568236b2bfea.
func TestRunPatternTimerIntervalPropertyArray593RuntimeIDMapping(t *testing.T) {
	if len(patternTimerIntervalPropertyArray593JavaRuntimeIDs) != 1 ||
		patternTimerIntervalPropertyArray593JavaRuntimeIDs[0] != "java-runtime-6155a2b0181e2169f2a4" {
		t.Fatalf("runtime ids = %v", patternTimerIntervalPropertyArray593JavaRuntimeIDs)
	}
	if len(patternTimerIntervalPropertyArray593JavaExecutions) != 1 ||
		patternTimerIntervalPropertyArray593JavaExecutions[0] != "PatternIntervalSpecExpressionWithPropertyArray" {
		t.Fatalf("executions = %v", patternTimerIntervalPropertyArray593JavaExecutions)
	}
	if len(patternTimerIntervalPropertyArray593JavaStaticIDs) != 1 ||
		patternTimerIntervalPropertyArray593JavaStaticIDs[0] != "java-1422565b568236b2bfea" {
		t.Fatalf("static ids = %v", patternTimerIntervalPropertyArray593JavaStaticIDs)
	}
}

func patternTimerIntervalPropertyArray593ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-timer-interval-property-array-593.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternTimerIntervalPropertyArray593ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternTimerIntervalPropertyArray593ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternTimerIntervalPropertyArray593Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
