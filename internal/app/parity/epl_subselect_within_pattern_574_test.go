package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Ord 0 EPLSubselectInvalid (java-runtime-57f90dfd1960035b1517) is
// compile-only and joins the manifest as intentionally-different: the
// typed Go API has no EPL-text compile path, so the verbatim Java
// diagnostics cannot be pinned in a trace. Byte-exact probes:
//
//	select * from SupportBean_S0(exists (select * from SupportBean_S1))
//	-> "Failed to validate subquery number 1 querying SupportBean_S1:
//	   Subqueries require one or more views to limit the stream,
//	   consider declaring a length or time window […]"
//	select * from SupportBean_S0(exists (select * from
//	   MyWindowInvalid#lastevent))
//	-> "Failed to validate subquery number 1 querying MyWindowInvalid:
//	   Consuming statements to a named window cannot declare a data
//	   window view onto the named window […]"
//	select * from SupportBean_S0(id in ((select p00 from
//	   MyWindowInvalid)))
//	-> "Failed to validate filter expression 'id in (subselect_1)':
//	   Implicit conversion not allowed: Cannot coerce types Integer
//	   and String […]"
//
// The Go-side representative checks live in
// internal/esper/pattern_subquery_invalid_test.go and assert the same
// three Build rejections (unwindowed exists, data window onto a named
// window, Integer/String coercion) as structural errors.
//

// TestRunEplSubselectWithinPattern574ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: 19 listener deliveries across nine cases
// (3+3+1+3+1+2+2+2+2).
func TestRunEplSubselectWithinPattern574ScenarioReplay(t *testing.T) {
	scenario := loadEplSubselectWithinPattern574ScenarioForTest(t)
	trace, err := runEplSubselectWithinPattern574Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != eplSubselectWithinPattern574ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	wantCounts := map[string]int{
		"correlated-pattern-exists":     3,
		"correlated-filter-exists":      3,
		"correlated-followed-by-scalar": 1,
		"aggregation":                   3,
		"named-window-udf":              1,
		"noalias-pattern-lastevent":     2,
		"noalias-filter-lastevent":      2,
		"noalias-filter-named-window":   2,
		"noalias-pattern-named-window":  2,
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
		records[record.Case] = append(records[record.Case], record)
	}
	for name, want := range wantCounts {
		if byCase[name] != want {
			t.Fatalf("case %q records = %d, want %d", name, byCase[name], want)
		}
	}
	if len(trace.Records) != 19 {
		t.Fatalf("records = %d, want 19", len(trace.Records))
	}
	// myid pins.
	myid := func(caseName string, index int) any {
		t.Helper()
		rec := records[caseName][index]
		if len(rec.New) != 1 {
			t.Fatalf("case %q record %d new rows = %v", caseName, index, rec.New)
		}
		return rec.New[0].Fields["myid"]
	}
	if got := myid("correlated-followed-by-scalar", 0); got != "Y+B" {
		t.Fatalf("followed-by-scalar myid = %v, want Y+B", got)
	}
	for _, name := range []string{"correlated-pattern-exists", "correlated-filter-exists"} {
		for index, want := range []any{5, 6, 9} {
			if got := myid(name, index); got != want {
				t.Fatalf("case %q myid %d = %v, want %v", name, index, got, want)
			}
		}
	}
	for _, name := range []string{
		"noalias-pattern-lastevent", "noalias-filter-lastevent",
		"noalias-filter-named-window", "noalias-pattern-named-window",
	} {
		for index, want := range []any{5, 10} {
			if got := myid(name, index); got != want {
				t.Fatalf("case %q myid %d = %v, want %v", name, index, got, want)
			}
		}
	}
	// Aggregation select * rows carry every S0 property (unset ones null);
	// pin only the discriminator ids.
	for index, want := range []int{1, 4, 13} {
		rec := records["aggregation"][index]
		if len(rec.New) != 1 {
			t.Fatalf("aggregation record %d rows = %v", index, rec.New)
		}
		if got := rec.New[0].Fields["id"]; got != want {
			t.Fatalf("aggregation record %d id = %v, want %v", index, got, want)
		}
	}
}

// TestRunEplSubselectWithinPattern574PinnedArtifacts pins the scenario's
// Java identity and the step shape the loader accepts.
func TestRunEplSubselectWithinPattern574PinnedArtifacts(t *testing.T) {
	scenario := loadEplSubselectWithinPattern574ScenarioForTest(t)
	if scenario.ID != eplSubselectWithinPattern574ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 108 {
		t.Fatalf("steps = %d, want 108", len(scenario.Steps))
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
	if cases != 9 || deploys != 9 || undeploys != 9 || sends != 81 {
		t.Fatalf("cases/deploys/sends/undeploys = %d/%d/%d/%d", cases, deploys, sends, undeploys)
	}
}

// TestRunEplSubselectWithinPattern574RejectsMalformedRawScenario verifies
// the loader pins the step sequence: a mutated EPL, mutated send payload
// or mutated event type is rejected.
func TestRunEplSubselectWithinPattern574RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(eplSubselectWithinPattern574ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"epl", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("SupportBean_S1#keepall"),
				[]byte("SupportBean_S1#lastevent"), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"p00": "C"`),
				[]byte(`"p00": "B"`), 1)
		}},
		{"event type", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"eventType": "SupportBean_S2"`),
				[]byte(`"eventType": "SupportBean_S1"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadEplSubselectWithinPattern574Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunEplSubselectWithinPattern574RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports.
func TestRunEplSubselectWithinPattern574RuntimeIDMappingMatchesScenario(t *testing.T) {
	for _, spec := range eplSubselectWithinPattern574CaseSpecs {
		if spec.ordinal != spec.runtimeIndex+1 {
			t.Fatalf("case %q ordinal = %d, runtimeIndex = %d", spec.name, spec.ordinal, spec.runtimeIndex)
		}
	}
	for _, id := range eplSubselectWithinPattern574JavaStaticIDs {
		if id != "java-495107e31d1fe86086ab" {
			t.Fatalf("static id = %q", id)
		}
	}
	if len(eplSubselectWithinPattern574JavaRuntimeIDs) != 4 ||
		len(eplSubselectWithinPattern574JavaExecutions) != 4 {
		t.Fatalf("java identity lengths = %d/%d",
			len(eplSubselectWithinPattern574JavaRuntimeIDs), len(eplSubselectWithinPattern574JavaExecutions))
	}
}

func eplSubselectWithinPattern574ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"epl-subselect-within-pattern-574.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadEplSubselectWithinPattern574ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(eplSubselectWithinPattern574ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadEplSubselectWithinPattern574Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
