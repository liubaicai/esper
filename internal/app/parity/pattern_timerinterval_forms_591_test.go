package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Siblings excluded from this slice (manifest dispositions are
// main-agent owned):
//   - ord 0 PatternOp — the 26-case W-harness over the timed mixed event
//     set.
//   - ord 4 PatternIntervalSpecExpressionWithProperty — already
//     differential-verified (case.pattern-observer-timer-interval-
//     expression, java-runtime-36d18111e663f599bc14).
//   - ord 7 PatternIntervalSpecExpressionWithPropertyArray — the
//     [2] a=SupportBean -> timer:interval(a[0]+a[1] seconds) leg is
//     already Go-covered
//     (TestPatternTimerIntervalExpressionUsesCapturedPropertyArray).

// TestRunPatternTimerIntervalForms591ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record surface
// the oracle produces: exactly 5 listener deliveries — one per case —
// each carrying ONE new row with empty fields (select * over a tagless
// timer match) stamped at the firing instant: 62000ms for the four
// fixed forms and the 2002-03-01T09:00:00Z calendar boundary for
// month-scoped.
func TestRunPatternTimerIntervalForms591ScenarioReplay(t *testing.T) {
	scenario := loadPatternTimerIntervalForms591ScenarioForTest(t)
	trace, err := runPatternTimerIntervalForms591Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternTimerIntervalForms591ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 5 {
		t.Fatalf("records = %d, want 5", len(trace.Records))
	}
	wantTimes := map[string]string{
		"interval-spec":               patternTimerIntervalForms591Fire,
		"interval-spec-variables":     patternTimerIntervalForms591Fire,
		"interval-spec-expression":    patternTimerIntervalForms591Fire,
		"interval-spec-prepared-stmt": patternTimerIntervalForms591Fire,
		"month-scoped":                patternTimerIntervalForms591FireM,
	}
	for index, record := range trace.Records {
		spec := patternTimerIntervalForms591CaseSpecs[index]
		if record.Case != spec.name || record.Operation != "listener" || record.Statement != "s0" {
			t.Fatalf("record %d = %+v", index, record)
		}
		if record.Sequence != 1 {
			t.Fatalf("record %d sequence = %d, want 1", index, record.Sequence)
		}
		if record.Time != wantTimes[spec.name] {
			t.Fatalf("record %d time = %q, want %q", index, record.Time, wantTimes[spec.name])
		}
		if len(record.Old) != 0 {
			t.Fatalf("delivery %d should be new-only: %v", index, record.Old)
		}
		if len(record.New) != 1 {
			t.Fatalf("record %d has %d new rows, want 1", index, len(record.New))
		}
		row := record.New[0]
		if row.Kind != "row" || len(row.Fields) != 0 {
			t.Fatalf("record %d row = %+v, want one empty-fields row", index, row)
		}
	}
}

// TestRunPatternTimerIntervalForms591PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts — five
// case blocks, nine deploys (the four create-variable legs plus five s0
// statements), fifteen advance-time probes (arm, silent and firing per
// case), five undeploy-alls and zero send/set-variable steps.
func TestRunPatternTimerIntervalForms591PinnedArtifacts(t *testing.T) {
	scenario := loadPatternTimerIntervalForms591ScenarioForTest(t)
	if scenario.ID != patternTimerIntervalForms591ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 34 {
		t.Fatalf("steps = %d, want 34", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "interval-spec" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, advances, undeploys, parameterized int
	order := []string{}
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
			order = append(order, step.Case)
		case "deploy":
			deploys++
			if len(step.Payload) > 0 {
				parameterized++
			}
		case "advance-time":
			advances++
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 5 || deploys != 9 || advances != 15 || undeploys != 5 || parameterized != 1 {
		t.Fatalf("cases/deploys/advances/undeploys/parameterized = %d/%d/%d/%d/%d",
			cases, deploys, advances, undeploys, parameterized)
	}
	wantOrder := []string{
		"interval-spec", "interval-spec-variables", "interval-spec-expression",
		"interval-spec-prepared-stmt", "month-scoped",
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("case order = %v, want %v", order, wantOrder)
	}
}

// TestRunPatternTimerIntervalForms591RejectsMalformedRawScenario
// verifies the loader pins the byte-exact discriminants: the literal
// form's spacing, the create-variable initializers, the expression's
// operator precedence spelling, the prepared-stmt casts and deploy
// payload, the calendar form's month keyword and the pinned advance
// instants.
func TestRunPatternTimerIntervalForms591RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternTimerIntervalForms591ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"literal spacing", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(1 minute 2 seconds)`),
				[]byte(`timer:interval(1 minute  2 seconds)`), 1)
		}},
		{"variable initializer", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`@public create variable double M_isv=1`),
				[]byte(`@public create variable double M_isv=2`), 1)
		}},
		{"expression precedence", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(MOne*60+SOne seconds)`),
				[]byte(`timer:interval(MOne*(60+SOne) seconds)`), 1)
		}},
		{"prepared-stmt cast", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(?::int minute ?::int seconds)`),
				[]byte(`timer:interval(?::long minute ?::long seconds)`), 1)
		}},
		{"prepared-stmt payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": [1, 2]`),
				[]byte(`"payload": [2, 1]`), 1)
		}},
		{"calendar unit", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:interval(1 month)`),
				[]byte(`timer:interval(28 days)`), 1)
		}},
		{"silent probe instant", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:01:01.999Z"`),
				[]byte(`"at": "1970-01-01T00:01:02Z"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternTimerIntervalForms591Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternTimerIntervalForms591RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// ords 1/2/3/5/6 own runtimes d5ad6a…/bbb75d…/97c5e6…/ea394f…/28fc7f…
// with the shared static id java-1422565b568236b2bfea.
func TestRunPatternTimerIntervalForms591RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{1, 2, 3, 5, 6}
	if len(patternTimerIntervalForms591CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternTimerIntervalForms591CaseSpecs))
	}
	for index, spec := range patternTimerIntervalForms591CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != index {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	wantStatic := []string{
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
		"java-1422565b568236b2bfea",
	}
	if !reflect.DeepEqual(patternTimerIntervalForms591JavaStaticIDs, wantStatic) {
		t.Fatalf("static ids = %v, want %v", patternTimerIntervalForms591JavaStaticIDs, wantStatic)
	}
}

func patternTimerIntervalForms591ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-timer-interval-forms-591.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternTimerIntervalForms591ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternTimerIntervalForms591ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternTimerIntervalForms591Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
