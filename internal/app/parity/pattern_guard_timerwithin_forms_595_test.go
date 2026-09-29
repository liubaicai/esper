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

// patternGuardTimerWithinForms595ExpectedRecord pins one listener
// record: the case, the per-case sequence, the delivery instant and
// the flattened field rows. `nil` rows mark an empty-payload row
// (untagged atoms — Java renders zero fields).
type patternGuardTimerWithinForms595ExpectedRecord struct {
	caseName string
	sequence uint64
	at       string
	fields   []map[string]string
}

var patternGuardTimerWithinForms595ExpectedRecords = []patternGuardTimerWithinForms595ExpectedRecord{
	// Ords 1-3: fire at t=0 and t=93784004ms, silent at the exclusive
	// deadline. Identical timing whether the period is literal,
	// variable-resolved or substitution-bound.
	{"interval-10-min", 1, "1970-01-01T00:00:00Z", []map[string]string{{}}},
	{"interval-10-min", 2, "1970-01-02T02:03:04.004Z", []map[string]string{{}}},
	{"interval-10-min-variable", 1, "1970-01-01T00:00:00Z", []map[string]string{{}}},
	{"interval-10-min-variable", 2, "1970-01-02T02:03:04.004Z", []map[string]string{{}}},
	{"interval-prepared", 1, "1970-01-01T00:00:00Z", []map[string]string{{}}},
	{"interval-prepared", 2, "1970-01-02T02:03:04.004Z", []map[string]string{{}}},
	// Ord 4: one {id} row per matching send; the advance to 3000 kills
	// the guarded (every b) subtree so post-deadline E4 is silent.
	{"within-from-expression", 1, "1970-01-01T00:00:02Z", []map[string]string{{"id": "E2"}}},
	{"within-from-expression", 2, "1970-01-01T00:00:02.999Z", []map[string]string{{"id": "E3"}}},
	// Ord 5: the E4 branch spawned inside the 6000 advance sees E5.
	{"pattern-not-followed-by", 1, "1970-01-01T00:00:06Z", []map[string]string{{}}},
	// Ord 6: two rounds, each arm-at-Feb-01, fire E1 + E2@deadline-1ms,
	// silent at the exact boundary. The per-deploy listener resets the
	// sequence each round like the Java oracle's fresh TraceWriter.
	{"may-max-month", 1, "2002-02-01T09:00:00Z", []map[string]string{{}}},
	{"may-max-month", 2, "2002-03-01T08:59:59.999Z", []map[string]string{{}}},
	{"may-max-month", 1, "2002-02-01T09:00:00Z", []map[string]string{{}}},
	{"may-max-month", 2, "2002-03-01T08:59:59.999Z", []map[string]string{{}}},
}

// TestRunPatternGuardTimerWithinForms595ScenarioReplay replays the
// scenario and pins the complete record surface: thirteen listener
// records across six cases — three boundary-exclusive silence points
// (period for ords 1-3, the 3000ms post-deadline for ord 4, both
// boundary sends of ord 6) deliver nothing.
func TestRunPatternGuardTimerWithinForms595ScenarioReplay(t *testing.T) {
	scenario := loadPatternGuardTimerWithinForms595ScenarioForTest(t)
	trace, err := runPatternGuardTimerWithinForms595Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternGuardTimerWithinForms595ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != len(patternGuardTimerWithinForms595ExpectedRecords) {
		t.Fatalf("records = %d, want %d: %v",
			len(trace.Records), len(patternGuardTimerWithinForms595ExpectedRecords), trace.Records)
	}
	for index, want := range patternGuardTimerWithinForms595ExpectedRecords {
		record := trace.Records[index]
		if record.Case != want.caseName || record.Operation != "listener" || record.Statement != "s0" {
			t.Fatalf("record %d = %+v, want case %q s0", index, record, want.caseName)
		}
		if record.Sequence != want.sequence {
			t.Fatalf("record %d sequence = %d, want %d", index, record.Sequence, want.sequence)
		}
		if record.Time != want.at {
			t.Fatalf("record %d time = %q, want %q", index, record.Time, want.at)
		}
		if len(record.Old) != 0 {
			t.Fatalf("record %d delivery should be new-only: %v", index, record.Old)
		}
		if len(record.New) != len(want.fields) {
			t.Fatalf("record %d new rows = %d, want %d", index, len(record.New), len(want.fields))
		}
		for row, fields := range want.fields {
			got := map[string]string{}
			for name, value := range record.New[row].Fields {
				if str, ok := value.(string); ok {
					got[name] = str
				}
			}
			if !reflect.DeepEqual(got, fields) {
				t.Fatalf("record %d row %d fields = %v, want %v", index, row, got, fields)
			}
		}
	}
}

// TestRunPatternGuardTimerWithinForms595RejectsMalformedRawScenario
// pins the negative path of the loader validation: every mutation must
// be refused instead of silently drifting from the pinned oracle text
// or step contract.
func TestRunPatternGuardTimerWithinForms595RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternGuardTimerWithinForms595ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"ordinal drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"ordinal": 1`),
				[]byte(`"ordinal": 0`), 1)
		}},
		{"runtime id drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`java-runtime-f0649272cfb528ce731b`),
				[]byte(`java-runtime-f0649272cfb528ce731c`), 1)
		}},
		{"epl period drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`timer:within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)`),
				[]byte(`timer:within(1 days 2 hours 3 minutes 4 seconds 6 milliseconds)`), 1)
		}},
		{"send time drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "1970-01-02T02:03:04.004Z"`),
				[]byte(`"at": "1970-01-02T02:03:04.005Z"`), 1)
		}},
		{"send payload drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"theString": "E2"`),
				[]byte(`"theString": "E9"`), 1)
		}},
		{"deploy at drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"at": "2002-02-01T09:00:00Z"`),
				[]byte(`"at": "2002-02-01T10:00:00Z"`), 1)
		}},
		{"undeploy op drift", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"op": "undeploy-all"`),
				[]byte(`"op": "undeploy"`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mutated := mutate.mutate(raw)
			if bytes.Equal(mutated, raw) {
				t.Fatalf("mutation %s produced no change", mutate.name)
			}
			if _, err := loadPatternGuardTimerWithinForms595Scenario(bytes.NewReader(mutated)); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternGuardTimerWithinForms595PinnedArtifacts pins the
// scenario file's Java identity plus the step sequence the loader
// accepts — six case blocks, seven deploys (ord 6 deploys twice, both
// carrying the pre-deploy `at`), one undeploy-all per deploy and 23
// fused advance-before-send steps.
func TestRunPatternGuardTimerWithinForms595PinnedArtifacts(t *testing.T) {
	scenario := loadPatternGuardTimerWithinForms595ScenarioForTest(t)
	if scenario.ID != patternGuardTimerWithinForms595ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	var cases, deploys, sends, undeploys, deploysWithAt int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
			if step.Statement != "s0" {
				t.Fatalf("deploy statement = %q", step.Statement)
			}
			if step.At != "" {
				deploysWithAt++
			}
		case "send":
			sends++
			if step.At == "" {
				t.Fatalf("send step %d missing at", sends)
			}
		case "undeploy-all":
			undeploys++
		default:
			t.Fatalf("unexpected op %q", step.Op)
		}
	}
	if cases != 6 || deploys != 7 || sends != 23 || undeploys != 7 || deploysWithAt != 2 {
		t.Fatalf("cases/deploys/sends/undeploys/deploysAt = %d/%d/%d/%d/%d",
			cases, deploys, sends, undeploys, deploysWithAt)
	}
	wantSpecs := []string{
		"interval-10-min", "interval-10-min-variable", "interval-prepared",
		"within-from-expression", "pattern-not-followed-by", "may-max-month",
	}
	if len(patternGuardTimerWithinForms595CaseSpecs) != len(wantSpecs) {
		t.Fatalf("case specs = %d, want %d", len(patternGuardTimerWithinForms595CaseSpecs), len(wantSpecs))
	}
	for index, want := range wantSpecs {
		spec := patternGuardTimerWithinForms595CaseSpecs[index]
		if spec.name != want || spec.ordinal != index+1 || spec.runtimeIndex != index {
			t.Fatalf("spec %d = %+v, want name %q ordinal %d", index, spec, want, index+1)
		}
	}
}

// TestRunPatternGuardTimerWithinForms595RuntimeIDMappingMatchesScenario
// pins the case→runtime/execution/ordinal mapping the -diff path
// reports plus the shared static id: ordinals 1-6 own one runtime id
// each under static java-0dec801a426fed297402.
func TestRunPatternGuardTimerWithinForms595RuntimeIDMappingMatchesScenario(t *testing.T) {
	if !reflect.DeepEqual(patternGuardTimerWithinForms595JavaRuntimeIDs, []string{
		"java-runtime-f0649272cfb528ce731b",
		"java-runtime-2fe5370a7c1599bfb424",
		"java-runtime-66d65309509fac58bbfc",
		"java-runtime-36ef6f15f14a0e86ab93",
		"java-runtime-6a5e8128ae184e8a7249",
		"java-runtime-34555c4a9823a346d710",
	}) {
		t.Fatalf("runtime ids = %v", patternGuardTimerWithinForms595JavaRuntimeIDs)
	}
	if !reflect.DeepEqual(patternGuardTimerWithinForms595JavaExecutions, []string{
		"PatternInterval10Min",
		"PatternInterval10MinVariable",
		"PatternIntervalPrepared",
		"PatternWithinFromExpression",
		"PatternPatternNotFollowedBy",
		"PatternWithinMayMaxMonthScoped",
	}) {
		t.Fatalf("executions = %v", patternGuardTimerWithinForms595JavaExecutions)
	}
	if !reflect.DeepEqual(patternGuardTimerWithinForms595JavaStaticIDs,
		[]string{"java-0dec801a426fed297402"}) {
		t.Fatalf("static ids = %v", patternGuardTimerWithinForms595JavaStaticIDs)
	}
	if len(patternGuardTimerWithinForms595JavaFlags) != 0 {
		t.Fatalf("flags = %v", patternGuardTimerWithinForms595JavaFlags)
	}
	for index, spec := range patternGuardTimerWithinForms595CaseSpecs {
		if spec.runtimeIndex != index ||
			patternGuardTimerWithinForms595JavaRuntimeIDs[index] != patternGuardTimerWithinForms595JavaRuntimeIDs[spec.runtimeIndex] {
			t.Fatalf("spec %d mapping drift: %+v", index, spec)
		}
	}
}

// TestRunPatternGuardTimerWithinForms595EPLsByteExact pins each case's
// deployed statement text against the Java source lines byte-for-byte
// (ord 6 carries both rounds in deploy order).
func TestRunPatternGuardTimerWithinForms595EPLsByteExact(t *testing.T) {
	want := map[string][]string{
		"interval-10-min": {
			`@name('s0') select * from pattern [(every SupportBean) where timer:within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)]`,
		},
		"interval-10-min-variable": {
			`@name('s0') select * from pattern [(every SupportBean) where timer:within(D days H hours M minutes S seconds MS milliseconds)]`,
		},
		"interval-prepared": {
			`@name('s0') select * from pattern [(every SupportBean) where timer:within(?::int days ?::int hours ?::int minutes ?::int seconds ?::int milliseconds)]`,
		},
		"within-from-expression": {
			`@name('s0') select b.theString as id from pattern[a=SupportBean -> (every b=SupportBean) where timer:within(a.intPrimitive seconds)]`,
		},
		"pattern-not-followed-by": {
			`@name('s0') select * from pattern [ every(SupportBean -> (SupportMarketDataBean where timer:within(5 sec))) ]`,
		},
		"may-max-month": {
			`@name('s0') select * from pattern [(every SupportBean) where timer:within(1 month)]`,
			`@name('s0') select * from pattern [(every SupportBean) where timer:withinmax(1 month, 10)]`,
		},
	}
	for caseName, epls := range want {
		if !reflect.DeepEqual(patternGuardTimerWithinForms595EPLs(caseName), epls) {
			t.Fatalf("epls for %s = %v, want %v", caseName, patternGuardTimerWithinForms595EPLs(caseName), epls)
		}
	}
}

// TestRunPatternGuardTimerWithinForms595ArmOrder pins the two
// clock-ordering facts the unit exists to cover: a deploy step may
// carry `at` (advance-before-deploy, ord 6) and every send fuses
// advance-before-send (guard expiry at the send instant precedes the
// event delivery).
func TestRunPatternGuardTimerWithinForms595ArmOrder(t *testing.T) {
	scenario := loadPatternGuardTimerWithinForms595ScenarioForTest(t)
	var monthDeploys, otherDeploysWithAt int
	current := ""
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			current = step.Case
			continue
		}
		if step.Op == "deploy" && step.At != "" {
			if current == "may-max-month" {
				monthDeploys++
				if step.At != "2002-02-01T09:00:00Z" {
					t.Fatalf("ord6 deploy at = %q", step.At)
				}
			} else {
				otherDeploysWithAt++
			}
		}
	}
	if monthDeploys != 2 || otherDeploysWithAt != 0 {
		t.Fatalf("deploy-at usage = %d ord6 / %d other", monthDeploys, otherDeploysWithAt)
	}
	deadline := time.Date(2002, 3, 1, 9, 0, 0, 0, time.UTC)
	if arm := time.Date(2002, 2, 1, 9, 0, 0, 0, time.UTC); !arm.AddDate(0, 1, 0).Equal(deadline) {
		t.Fatalf("calendar month arithmetic drifted: %s", arm.AddDate(0, 1, 0))
	}
}

func patternGuardTimerWithinForms595ScenarioPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "parity", patternGuardTimerWithinForms595ID+".json")
}

func loadPatternGuardTimerWithinForms595ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternGuardTimerWithinForms595ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer func() { _ = file.Close() }()
	scenario, err := loadPatternGuardTimerWithinForms595Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
