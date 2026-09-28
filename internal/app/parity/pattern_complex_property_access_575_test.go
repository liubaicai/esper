package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Ords 3-4 (PatternIndexedValuePropOM java-runtime-e0804e3e156cf0ee3656,
// PatternIndexedValuePropCompile java-runtime-4364d60de0353a761ba1) replay
// the identical followed-by pattern through the SODA object-model and
// eplToModelCompileDeploy front ends; the fluent Go API has no OM/compile
// front ends, so both join the manifest as intentionally-different with
// the same expectations covered by ord 2 (SODA/compile-text precedent).

// TestRunPatternComplexPropertyAccess575ScenarioReplay replays the
// checked-in scenario through the Go runner and pins the record surface
// the oracle produces: 11 listener deliveries across eighteen cases — the
// eight firing ord 0 spellings, two ord 1 captures, one ord 2
// followed-by.
func TestRunPatternComplexPropertyAccess575ScenarioReplay(t *testing.T) {
	scenario := loadPatternComplexPropertyAccess575ScenarioForTest(t)
	trace, err := runPatternComplexPropertyAccess575Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternComplexPropertyAccess575ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	wantCounts := map[string]int{
		"mapped-key":              1,
		"indexed-1-eq-2":          1,
		"array-1-eq-20":           1,
		"array-1-in-range":        1,
		"nested-value":            1,
		"nested-nested-value":     1,
		"combined-indexed-mapped": 1,
		"combined-array-mapped":   1,
		"indexed-filter-prop":     2,
		"indexed-value-prop":      1,
	}
	byCase := map[string]int{}
	byCaseSeq := map[string]uint64{}
	records := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		if record.Operation != "listener" || record.Statement != "s0" {
			t.Fatalf("unexpected record %+v", record)
		}
		byCase[record.Case]++
		byCaseSeq[record.Case]++
		if record.Sequence != byCaseSeq[record.Case] {
			t.Fatalf("case %q sequence = %d, want %d", record.Case, record.Sequence, byCaseSeq[record.Case])
		}
		if len(record.New) != 1 {
			t.Fatalf("case %q record %d new rows = %v", record.Case, record.Sequence, record.New)
		}
		records[record.Case] = append(records[record.Case], record)
	}
	for name, want := range wantCounts {
		if byCase[name] != want {
			t.Fatalf("case %q records = %d, want %d", name, byCase[name], want)
		}
	}
	if len(trace.Records) != 11 {
		t.Fatalf("records = %d, want 11", len(trace.Records))
	}

	// Each ord 0 fire carries the captured tagged event under s; pin the
	// discriminators of the default beans that drove the match. Go events
	// surface schema values (*string for nullable fields); the
	// canonicalized trace renders them like the oracle's strings.
	captured := func(caseName string, index int, tag string) map[string]any {
		t.Helper()
		fields, ok := records[caseName][index].New[0].Fields[tag].(map[string]any)["fields"].(map[string]any)
		if !ok {
			t.Fatalf("case %q record %d tag %q has no event row", caseName, index, tag)
		}
		return fields
	}
	stringField := func(fields map[string]any, name string) string {
		t.Helper()
		value, ok := fields[name].(*string)
		if !ok || value == nil {
			t.Fatalf("field %q = %v, want string", name, fields[name])
		}
		return *value
	}
	for _, name := range []string{
		"mapped-key", "indexed-1-eq-2", "array-1-eq-20", "array-1-in-range",
		"nested-value", "nested-nested-value",
	} {
		fields := captured(name, 0, "s")
		if got := stringField(fields, "simpleProperty"); got != "simple" {
			t.Fatalf("case %q captured simpleProperty = %v", name, got)
		}
		indexed, ok := fields["indexed"].([]int)
		if !ok || len(indexed) != 2 || indexed[0] != 1 || indexed[1] != 2 {
			t.Fatalf("case %q captured indexed = %v", name, fields["indexed"])
		}
	}
	for _, name := range []string{"combined-indexed-mapped", "combined-array-mapped"} {
		fields := captured(name, 0, "s")
		indexed, ok := fields["indexed"].([]*patternComplexPropertyAccess575CombinedNestedOne)
		if !ok || len(indexed) != 4 || indexed[3] != nil {
			t.Fatalf("case %q captured indexed = %v", name, fields["indexed"])
		}
	}

	// Ord 1: every-atom filter captures the two indexed[0]=3 events;
	// {indexed:[6]} stays silent.
	first := captured("indexed-filter-prop", 0, "a")
	if got := first["indexed"]; !reflect.DeepEqual(got, []int{3, 4}) {
		t.Fatalf("indexed-filter-prop first captured indexed = %v", got)
	}
	second := captured("indexed-filter-prop", 1, "a")
	if got := second["indexed"]; !reflect.DeepEqual(got, []int{3}) {
		t.Fatalf("indexed-filter-prop second captured indexed = %v", got)
	}

	// Ord 2: the single followed-by delivery pairs the first and third
	// sends; the simpleProperty markers pin which {3} event each tag
	// captured (Java's assertSame has no Go counterpart).
	a := captured("indexed-value-prop", 0, "a")
	if got := stringField(a, "simpleProperty"); got != "eventOne" {
		t.Fatalf("indexed-value-prop a = %v", got)
	}
	b := captured("indexed-value-prop", 0, "b")
	if got := stringField(b, "simpleProperty"); got != "eventTwo" {
		t.Fatalf("indexed-value-prop b = %v", got)
	}
}

// TestRunPatternComplexPropertyAccess575PinnedArtifacts pins the
// scenario's Java identity and the step shape the loader accepts.
func TestRunPatternComplexPropertyAccess575PinnedArtifacts(t *testing.T) {
	scenario := loadPatternComplexPropertyAccess575ScenarioForTest(t)
	if scenario.ID != patternComplexPropertyAccess575ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 92 {
		t.Fatalf("steps = %d, want 92", len(scenario.Steps))
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
	if cases != 18 || deploys != 18 || undeploys != 18 || sends != 38 {
		t.Fatalf("cases/deploys/sends/undeploys = %d/%d/%d/%d", cases, deploys, sends, undeploys)
	}
}

// TestRunPatternComplexPropertyAccess575RejectsMalformedRawScenario
// verifies the loader pins the step sequence: a mutated EPL, mutated send
// payload or mutated event type is rejected.
func TestRunPatternComplexPropertyAccess575RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternComplexPropertyAccess575ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("arrayProperty[1] in (10:30)"),
				[]byte("arrayProperty[1] in (10:29)"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"simpleProperty": "eventOne"`),
				[]byte(`"simpleProperty": "eventThree"`), 1)
		}},
		{"event type", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"eventType": "SupportBeanCombinedProps"`),
				[]byte(`"eventType": "SupportBeanComplexProps"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternComplexPropertyAccess575Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternComplexPropertyAccess575RuntimeIDMappingMatchesScenario
// pins the case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunPatternComplexPropertyAccess575RuntimeIDMappingMatchesScenario(t *testing.T) {
	for _, spec := range patternComplexPropertyAccess575CaseSpecs {
		if spec.ordinal != spec.runtimeIndex {
			t.Fatalf("case %q ordinal = %d, runtimeIndex = %d", spec.name, spec.ordinal, spec.runtimeIndex)
		}
	}
	wantStatic := []string{"java-be2858d5e4c76bbf7f85", "java-ec97467eeb93b4ff0e67", "java-9504034cfc1c929363d8"}
	for index, id := range patternComplexPropertyAccess575JavaStaticIDs {
		if id != wantStatic[index] {
			t.Fatalf("static id %d = %q, want %q", index, id, wantStatic[index])
		}
	}
	if len(patternComplexPropertyAccess575JavaRuntimeIDs) != 3 ||
		len(patternComplexPropertyAccess575JavaExecutions) != 3 {
		t.Fatalf("java identity lengths = %d/%d",
			len(patternComplexPropertyAccess575JavaRuntimeIDs), len(patternComplexPropertyAccess575JavaExecutions))
	}
}

func patternComplexPropertyAccess575ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-complex-property-access-575.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternComplexPropertyAccess575ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternComplexPropertyAccess575ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternComplexPropertyAccess575Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
