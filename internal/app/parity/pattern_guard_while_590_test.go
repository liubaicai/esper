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

// patternGuardWhile590ExpectedRecord pins one trace record: the case,
// the record kind (listener or compile-error), the statement label,
// the delivery instant in epoch millis (the trigger event's
// external-clock time for pattern-op; the engine start instant for the
// unclocked cases) and the flattened new rows as "tag:value" pairs.
type patternGuardWhile590ExpectedRecord struct {
	caseName    string
	operation   string
	statement   string
	atMS        int64
	rows        [][]string
	expectError string
}

// Records are pinned in the Go trace's emitted order: the Go engine
// dispatches one send's completions in deployment order (S0..S4); the
// -diff normalizer sorts both traces into (case, time, statement)
// buckets because Esper's cross-statement dispatch order inside a send
// is unspecified, and within a batch the rows compare as a multiset.
var patternGuardWhile590ExpectedRecords = []patternGuardWhile590ExpectedRecord{
	// simple: E1 and E2 pass the `a.theString like 'E%'` guard; the X
	// match falsifies it permanently so E3/X never fire.
	{"simple", "listener", "s0", 0, [][]string{{"c0:E1"}}, ""},
	{"simple", "listener", "s0", 0, [][]string{{"c0:E2"}}, ""},
	// pattern-op B1@2000: S4's guard falsifies on the very first match
	// and stays silent; the other legs pass B1.
	{"pattern-op", "listener", "S0", 2000, [][]string{{"a:A1", "b:B1"}}, ""},
	{"pattern-op", "listener", "S1", 2000, [][]string{{"a:A1", "b:B1"}}, ""},
	{"pattern-op", "listener", "S2", 2000, [][]string{{"b:B1"}}, ""},
	{"pattern-op", "listener", "S3", 2000, [][]string{{"b:B1"}}, ""},
	// pattern-op B2@4000: S0's `!= 'B2'` guard already quit at this
	// match; S1..S3 pass B2 and quit at B3.
	{"pattern-op", "listener", "S1", 4000, [][]string{{"a:A1", "b:B2"}}, ""},
	{"pattern-op", "listener", "S2", 4000, [][]string{{"b:B2"}}, ""},
	{"pattern-op", "listener", "S3", 4000, [][]string{{"b:B2"}}, ""},
	// pattern-variable: B1 completes both live every-a branches in ONE
	// listener batch; the runtimeSetVariable flip falsifies every live
	// branch so A3/A4/B2 stay silent.
	{"pattern-variable", "listener", "s0", 0,
		[][]string{{"a:A1", "b:B1"}, {"a:A2", "b:B1"}}, ""},
	// pattern-invalid: the two build-error probes record the pinned
	// Java compile-error messages.
	{"pattern-invalid", "compile-error", "non-boolean-literal-guard", 0, nil,
		patternGuardWhile590InvalidLiteralError},
	{"pattern-invalid", "compile-error", "unresolvable-property-guard", 0, nil,
		patternGuardWhile590InvalidPropertyError},
}

// TestRunPatternGuardWhile590ScenarioReplay replays the scenario and
// pins the complete record surface: twelve records across the four
// cases — two s0 fires for simple (the first falsifying X match quits
// the while-guard permanently), seven leg fires for pattern-op (S4's
// zero fires are the quit-vs-skip discriminant), the two-row B1 batch
// for pattern-variable (per-branch falsification after the variable
// flip keeps A3/A4/B2 silent) and the two compile-error records for
// pattern-invalid.
func TestRunPatternGuardWhile590ScenarioReplay(t *testing.T) {
	scenario := loadPatternGuardWhile590ScenarioForTest(t)
	trace, err := runPatternGuardWhile590Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternGuardWhile590ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternGuardWhile590ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternGuardWhile590ExpectedRecords), trace.Records)
	}
	for index, want := range patternGuardWhile590ExpectedRecords {
		record := trace.Records[index]
		if record.Case != want.caseName || record.Operation != want.operation ||
			record.Statement != want.statement {
			t.Fatalf("record %d = %+v, want case %q operation %q statement %q",
				index, record, want.caseName, want.operation, want.statement)
		}
		if want.operation == "compile-error" {
			if record.Value != want.expectError {
				t.Fatalf("record %d value = %v, want %q", index, record.Value, want.expectError)
			}
			continue
		}
		wantTime := time.UnixMilli(want.atMS).UTC()
		if record.Time != compat.FormatTraceTime(wantTime) {
			t.Fatalf("record %d time = %q, want %q", index, record.Time, compat.FormatTraceTime(wantTime))
		}
		if len(record.Old) != 0 {
			t.Fatalf("record %d delivery should be new-only: %v", index, record.Old)
		}
		got := patternGuardWhile590FlattenRows(t, record.New)
		if !reflect.DeepEqual(got, want.rows) {
			t.Fatalf("record %d (%s/%s) rows = %v, want %v", index,
				want.caseName, want.statement, got, want.rows)
		}
	}
}

// patternGuardWhile590FlattenRows flattens each normalized new row
// into "tag:value" strings: c0 is a plain scalar while the pattern-op
// and pattern-variable tags project bean fragments (id / theString).
func patternGuardWhile590FlattenRows(t *testing.T, rows []compat.ResultRecord) [][]string {
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
			value := decoded[tag]
			if bean, ok := value.(map[string]any); ok {
				beanFields, ok := bean["fields"].(map[string]any)
				if !ok {
					t.Fatalf("row %d tag %s has no fields: %v", rowIndex, tag, value)
				}
				if id, ok := beanFields["id"]; ok {
					flatten = append(flatten, fmt.Sprintf("%s:%v", tag, id))
					continue
				}
				if name, ok := beanFields["theString"]; ok {
					flatten = append(flatten, fmt.Sprintf("%s:%v", tag, name))
					continue
				}
				t.Fatalf("row %d tag %s has neither id nor theString: %v",
					rowIndex, tag, beanFields)
			}
			flatten = append(flatten, fmt.Sprintf("%s:%v", tag, value))
		}
		flattened = append(flattened, flatten)
	}
	return flattened
}

// TestRunPatternGuardWhile590RejectsMalformedRawScenario pins the
// negative path of the new loader validation: every flipped
// byte-discriminant must be refused instead of silently drifting from
// the pinned oracle text or step contract.
func TestRunPatternGuardWhile590RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternGuardWhile590ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"case ordinal drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"ordinal": 1`),
				[]byte(`"ordinal": 5`), 1)
		}},
		{"deploy-all statement drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"statement": "all"`),
				[]byte(`"statement": "s0"`), 1)
		}},
		{"send advance drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-01T00:00:04Z"`),
				[]byte(`"at": "1970-01-01T00:00:05Z"`), 1)
		}},
		{"send payload drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "E3"`),
				[]byte(`"theString": "E4"`), 1)
		}},
		{"guard atom drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`while(b.id != 'B2')`),
				[]byte(`while(b.id != 'B9')`), 1)
		}},
		{"soda literal drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`while (b.id!=\"B3\")`),
				[]byte(`while (b.id!=\"B4\")`), 1)
		}},
		{"set-variable value drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": false`),
				[]byte(`"payload": true`), 1)
		}},
		{"build-error message drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`true or false (boolean) value`),
				[]byte(`true or false (bool) value`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mutated := mutate.mutate(raw)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("mutation %s produced no change", mutate.name)
			}
			if _, err := loadPatternGuardWhile590Scenario(bytes.NewReader(mutated)); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternGuardWhile590PinnedArtifacts pins the scenario file's
// Java identity plus the step sequence the loader accepts — four case
// blocks, four deploy steps (one "all" expander plus three labeled
// deploys), 23 sends, one set-variable, two build-error probes and
// three undeploy-all steps — and the pinned EPLs byte-exact.
func TestRunPatternGuardWhile590PinnedArtifacts(t *testing.T) {
	scenario := loadPatternGuardWhile590ScenarioForTest(t)
	if scenario.ID != patternGuardWhile590ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 37 {
		t.Fatalf("steps = %d, want 37", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "simple" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, setVariables, buildErrors, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
		case "send":
			sends++
		case "set-variable":
			setVariables++
			if step.Statement != "var" || step.Name != "myVariable" {
				t.Fatalf("set-variable = %+v", step)
			}
		case "build-error":
			buildErrors++
		case "undeploy-all":
			undeploys++
		default:
			t.Fatalf("unexpected op %q", step.Op)
		}
	}
	if cases != 4 || deploys != 4 || sends != 23 || setVariables != 1 ||
		buildErrors != 2 || undeploys != 3 {
		t.Fatalf("cases/deploys/sends/set-variables/build-errors/undeploys = %d/%d/%d/%d/%d/%d",
			cases, deploys, sends, setVariables, buildErrors, undeploys)
	}
	if len(patternGuardWhile590CaseSpecs) != 4 ||
		len(patternGuardWhile590CaseSpecs[1].epls) != 5 {
		t.Fatalf("case specs = %+v", patternGuardWhile590CaseSpecs)
	}
	for leg, epl := range patternGuardWhile590CaseSpecs[1].epls {
		want := patternGuardWhile590StatementText(leg, patternGuardWhile590LegAtoms[leg])
		if epl != want {
			t.Fatalf("epl %d = %q, want %q", leg, epl, want)
		}
	}
}

// TestRunPatternGuardWhile590RuntimeIDMappingMatchesScenario pins the
// case->runtime/execution/ordinal mapping the -diff path reports plus
// the shared per-execution static id java-2be6b5626499eeff834e.
func TestRunPatternGuardWhile590RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantRuntimes := []string{
		"java-runtime-aa6c31e987ec865a8cf2",
		"java-runtime-bf4b4c7d66f189e40a73",
		"java-runtime-8808f29a3bfdd475589f",
		"java-runtime-4416cc3367e38623077a",
	}
	wantExecutions := []string{
		"PatternGuardWhileSimple", "PatternOp", "PatternVariable", "PatternInvalid",
	}
	for index, spec := range patternGuardWhile590CaseSpecs {
		if spec.ordinal != index || spec.runtimeIndex != index {
			t.Fatalf("case %q mapping = ordinal %d runtimeIndex %d",
				spec.name, spec.ordinal, spec.runtimeIndex)
		}
	}
	if !reflect.DeepEqual(patternGuardWhile590JavaRuntimeIDs, wantRuntimes) {
		t.Fatalf("runtime ids = %v", patternGuardWhile590JavaRuntimeIDs)
	}
	if !reflect.DeepEqual(patternGuardWhile590JavaExecutions, wantExecutions) {
		t.Fatalf("executions = %v", patternGuardWhile590JavaExecutions)
	}
	static := "java-2be6b5626499eeff834e"
	for index, id := range patternGuardWhile590JavaStaticIDs {
		if id != static {
			t.Fatalf("static id %d = %q, want %q", index, id, static)
		}
	}
}

func patternGuardWhile590ScenarioPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "parity", patternGuardWhile590ID+".json")
}

func loadPatternGuardWhile590ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternGuardWhile590ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternGuardWhile590Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
