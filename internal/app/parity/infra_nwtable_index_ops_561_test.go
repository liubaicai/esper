package parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// infraNWTableIndexOps561JavaTraceFixture writes the Java-side trace the
// diff assertions run against. When the unit's Java trace has been landed
// next to the scenario the checked-in file wins; before that the fixture
// is the same 92 records assembled from the runner's pinned constants
// (record shapes verified 0-diff against the oracle run for
// 9e1b9f1cc9117fea4bf33ab043762c045d73839c).
func infraNWTableIndexOps561JavaTraceFixture(t *testing.T, mutate func(*compat.Trace)) string {
	t.Helper()
	checkedIn := filepath.Join("..", "..", "..", "testdata", "parity",
		infraNWTableIndexOps561ID+".trace.json")
	if _, err := os.Stat(checkedIn); err == nil {
		return writeJavaTraceFixtureFromTrace(t, checkedIn, mutate)
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: infraNWTableIndexOps561ID}
	epoch := "1970-01-01T00:00:00Z"
	deploy := func(caseName, statement string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "deployed", Statement: statement,
			Sequence: 1, Time: epoch,
		})
	}
	note := func(caseName, statement, value string) {
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "unrepresentable", Statement: statement,
			Value: value,
		})
	}
	mcmiRows := func() []compat.ResultRecord {
		return []compat.ResultRecord{
			{Kind: "row", Fields: map[string]any{
				"f1": "E1", "f2": json.Number("-2"), "f3": ">E1<", "f4": "?E1?",
			}},
		}
	}
	mcmiCase := func(caseName string) {
		for _, label := range []string{"create", "insert", "index-one", "index-two", "index-three"} {
			deploy(caseName, label)
		}
		for _, label := range []string{"select-f3", "select-f3-f2", "select-full",
			"select-f2", "select-f1", "select-all"} {
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "snapshot", Statement: label,
				Sequence: 0, Time: epoch, New: mcmiRows(),
			})
		}
	}
	onrCase := func(caseName string, namedWindow bool) {
		for _, label := range []string{"create", "insert", "index", "s0"} {
			deploy(caseName, label)
		}
		if namedWindow {
			note(caseName, "count-after-s0", infraNWTableONR561NoteCount1)
		} else {
			one := int64(2)
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "index-count", Statement: "MyInfraONR",
				Sequence: 1, Time: epoch, Count: &one,
			})
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "listener", Statement: "s0",
			Sequence: 1, Time: epoch,
			New: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
				"f1": "E1", "f2": json.Number("1"),
			}}},
		})
		deploy(caseName, "stmtTwo")
		if namedWindow {
			note(caseName, "count-after-two", infraNWTableONR561NoteCount1)
		} else {
			two := int64(2)
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "index-count", Statement: "MyInfraONR",
				Sequence: 2, Time: epoch, Count: &two,
			})
		}
		if namedWindow {
			note(caseName, "count-after-s0-undeploy", infraNWTableONR561NoteCount2)
		} else {
			three := int64(2)
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "index-count", Statement: "MyInfraONR",
				Sequence: 3, Time: epoch, Count: &three,
			})
		}
		fourth := int64(1)
		if !namedWindow {
			fourth = int64(2)
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "index-count", Statement: "MyInfraONR",
			Sequence: 4, Time: epoch, Count: &fourth,
		})
		for _, label := range []string{"cw", "cw-index", "on-select-a", "on-select-b"} {
			deploy(caseName, label)
		}
		note(caseName, "count-cw", infraNWTableONR561NoteFour)
	}
	invalidCase := func(caseName string, table bool) {
		errCtx := infraNWTableINV561ErrCtxNW
		errSend := infraNWTableINV561ErrSendNW
		if table {
			errCtx = infraNWTableINV561ErrCtxTBL
			errSend = infraNWTableINV561ErrSendTBL
		}
		for _, label := range []string{"create", "index", "context-one", "context-two", "create-ctx"} {
			deploy(caseName, label)
		}
		note(caseName, "context-a", errCtx)
		note(caseName, "context-b", errCtx)
		for _, pair := range [][2]string{
			{"dup-index", infraNWTableINV561ErrDupIndex},
			{"unknown-column", infraNWTableINV561ErrUnknownCol},
		} {
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "compile-error", Statement: pair[0],
				Value: pair[1],
			})
		}
		note(caseName, "dup-column", infraNWTableINV561ErrDupCol)
		for _, pair := range [][2]string{
			{"unknown-infra", infraNWTableINV561ErrUnknownInf},
			{"bad-kind", infraNWTableINV561ErrBadKind},
		} {
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case: caseName, Operation: "compile-error", Statement: pair[0],
				Value: pair[1],
			})
		}
		note(caseName, "gugu", infraNWTableINV561ErrGugu)
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "compile-error", Statement: "unique-btree",
			Value: infraNWTableINV561ErrUniqueBT,
		})
		note(caseName, "null-typed", infraNWTableINV561ErrNullTyped)
		for _, label := range []string{"create-two", "insert", "index-unique"} {
			deploy(caseName, label)
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "send-error", Statement: "unique-violation",
			Value: errSend,
		})
		if table {
			deploy(caseName, "create-nokey")
			note(caseName, "no-pk-index", infraNWTableINV561ErrNoPK)
		}
	}
	mcmiCase("mcmi-window")
	mcmiCase("mcmi-table")
	onrCase("onr-window", true)
	onrCase("onr-table", false)
	invalidCase("invalid-window", false)
	invalidCase("invalid-table", true)
	if len(trace.Records) != 92 {
		t.Fatalf("fixture records = %d, want 92", len(trace.Records))
	}
	mutate(&trace)
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "java-trace.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// assertInfraNWTableIndexOps561Trace pins the 92-record index-ops trace:
// the six cases in execution order with their deployed markers, snapshot
// rows, listener delivery, index counts, compile-error prefixes,
// unrepresentable notes and send-error texts.
func assertInfraNWTableIndexOps561Trace(t *testing.T, trace compat.Trace) {
	t.Helper()
	if len(trace.Records) != 92 {
		t.Fatalf("trace records = %d, want 92", len(trace.Records))
	}
	wantRow := compat.ResultRecord{Kind: "row", Fields: map[string]any{
		"f1": "E1", "f2": json.Number("-2"), "f3": ">E1<", "f4": "?E1?",
	}}
	byCase := map[string][]compat.TraceRecord{}
	for _, rec := range trace.Records {
		byCase[rec.Case] = append(byCase[rec.Case], rec)
	}
	for _, caseName := range infraNWTableIndexOps561Cases {
		if len(byCase[caseName]) == 0 {
			t.Fatalf("case %q has no records", caseName)
		}
	}
	for _, caseName := range []string{"mcmi-window", "mcmi-table"} {
		recs := byCase[caseName]
		if len(recs) != 11 {
			t.Fatalf("%s records = %d, want 11", caseName, len(recs))
		}
		for index, label := range []string{"create", "insert", "index-one", "index-two", "index-three"} {
			rec := recs[index]
			if rec.Operation != "deployed" || rec.Statement != label || rec.Sequence != 1 {
				t.Fatalf("%s record %d = %#v", caseName, index, rec)
			}
		}
		for index, label := range []string{"select-f3", "select-f3-f2", "select-full",
			"select-f2", "select-f1", "select-all"} {
			rec := recs[5+index]
			if rec.Operation != "snapshot" || rec.Statement != label ||
				len(rec.New) != 1 || !reflect.DeepEqual(rec.New[0], wantRow) {
				t.Fatalf("%s snapshot %d = %#v", caseName, index, rec)
			}
		}
	}
	for _, pair := range []struct {
		caseName    string
		namedWindow bool
	}{
		{"onr-window", true}, {"onr-table", false},
	} {
		recs := byCase[pair.caseName]
		if len(recs) != 15 {
			t.Fatalf("%s records = %d, want 15", pair.caseName, len(recs))
		}
		listeners := 0
		counts := 0
		notes := 0
		deployed := 0
		for _, rec := range recs {
			switch rec.Operation {
			case "deployed":
				deployed++
			case "listener":
				listeners++
				if rec.Statement != "s0" || len(rec.New) != 1 ||
					rec.New[0].Fields["f1"] != "E1" ||
					rec.New[0].Fields["f2"] != json.Number("1") {
					t.Fatalf("%s listener = %#v", pair.caseName, rec)
				}
			case "index-count":
				counts++
				if rec.Count == nil {
					t.Fatalf("%s index-count missing count", pair.caseName)
				}
				if pair.namedWindow && *rec.Count != 1 {
					t.Fatalf("%s index-count = %d, want 1", pair.caseName, *rec.Count)
				}
				if !pair.namedWindow && *rec.Count != 2 {
					t.Fatalf("%s index-count = %d, want 2", pair.caseName, *rec.Count)
				}
			case "unrepresentable":
				notes++
				if rec.Value == nil || rec.Value == "" {
					t.Fatalf("%s unrepresentable missing note", pair.caseName)
				}
			}
		}
		if deployed != 9 || listeners != 1 {
			t.Fatalf("%s deployed/listener = %d/%d, want 9/1", pair.caseName, deployed, listeners)
		}
		if pair.namedWindow && (counts != 1 || notes != 4) {
			t.Fatalf("%s index-count/notes = %d/%d, want 1/4", pair.caseName, counts, notes)
		}
		if !pair.namedWindow && (counts != 4 || notes != 1) {
			t.Fatalf("%s index-count/notes = %d/%d, want 4/1", pair.caseName, counts, notes)
		}
	}
	for _, pair := range []struct {
		caseName string
		table    bool
	}{
		{"invalid-window", false}, {"invalid-table", true},
	} {
		recs := byCase[pair.caseName]
		want := 19
		if pair.table {
			want = 21
		}
		if len(recs) != want {
			t.Fatalf("%s records = %d, want %d", pair.caseName, len(recs), want)
		}
		compile := 0
		notes := 0
		for _, rec := range recs {
			switch rec.Operation {
			case "compile-error":
				compile++
				if rec.Value == nil || rec.Value == "" {
					t.Fatalf("%s compile-error missing prefix", pair.caseName)
				}
			case "send-error":
				wantText := infraNWTableINV561ErrSendNW
				if pair.table {
					wantText = infraNWTableINV561ErrSendTBL
				}
				if rec.Value != wantText {
					t.Fatalf("%s send-error = %#v, want %q", pair.caseName, rec.Value, wantText)
				}
			case "unrepresentable":
				notes++
			}
		}
		wantNotes := 5
		if pair.table {
			wantNotes = 6
		}
		if compile != 5 || notes != wantNotes {
			t.Fatalf("%s compile-error/notes = %d/%d, want 5/%d",
				pair.caseName, compile, notes, wantNotes)
		}
	}
}

func TestRunInfraNWTableIndexOps561DirectReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableIndexOps561ID,
		"-scenario", filepath.Join(root, infraNWTableIndexOps561ID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	trace, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	assertInfraNWTableIndexOps561Trace(t, trace)
}

// TestRunInfraNWTableIndexOps561PinnedArtifacts pins the byte-exact EPL
// constants transcribed from InfraNWTableCreateIndex.java lines 284-316
// (MCMI), 168-201 (OnSelectReUse) and 78-149 (Invalid), plus the per-case
// step sequence lengths, so a transcription slip fails the unit even when
// the checked-in scenario is regenerated.
func TestRunInfraNWTableIndexOps561PinnedArtifacts(t *testing.T) {
	wants := map[string]string{
		"mcmi-create-window": "@public create window MyInfraMCMI#keepall as (f1 string, f2 int, f3 string, f4 string)",
		"mcmi-create-table":  "@public create table MyInfraMCMI as (f1 string primary key, f2 int, f3 string, f4 string)",
		"mcmi-insert":        "insert into MyInfraMCMI(f1, f2, f3, f4) select theString, intPrimitive, '>'||theString||'<', '?'||theString||'?' from SupportBean",
		"mcmi-index1":        "create index MyInfraMCMIIndex1 on MyInfraMCMI(f2, f3, f1)",
		"mcmi-index2":        "create index MyInfraMCMIIndex2 on MyInfraMCMI(f2, f3)",
		"mcmi-index3":        "create index MyInfraMCMIIndex3 on MyInfraMCMI(f2)",
		"mcmi-select-f3":     "select * from MyInfraMCMI where f3='>E1<'",
		"mcmi-select-f3f2":   "select * from MyInfraMCMI where f3='>E1<' and f2=-2",
		"mcmi-select-full":   "select * from MyInfraMCMI where f3='>E1<' and f2=-2 and f1='E1'",
		"mcmi-select-f2":     "select * from MyInfraMCMI where f2=-2",
		"mcmi-select-f1":     "select * from MyInfraMCMI where f1='E1'",
		"mcmi-select-all":    "select * from MyInfraMCMI where f3='>E1<' and f2=-2 and f1='E1' and f4='?E1?'",
		"onr-create-window":  "@name('create') @public create window MyInfraONR#keepall as (f1 string, f2 int)",
		"onr-create-table":   "@name('create') @public create table MyInfraONR as (f1 string primary key, f2 int primary key)",
		"onr-insert":         "insert into MyInfraONR(f1, f2) select theString, intPrimitive from SupportBean",
		"onr-index":          "@name('indexOne') create index MyInfraONRIndex1 on MyInfraONR(f2)",
		"onr-select-s0":      "@name('s0') on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id",
		"onr-select-two":     "@name('stmtTwo') on SupportBean_S0 s0 select nw.f1 as f1, nw.f2 as f2 from MyInfraONR nw where nw.f2 = s0.id",
		"onr-cw":             "@name('cw') @public create window MyInfraFour#keepall as SupportBean",
		"onr-cw-index":       "create index idx1 on MyInfraFour (theString, intPrimitive)",
		"onr-on-select-a":    "on SupportBean sb select * from MyInfraFour w where w.theString = sb.theString and w.intPrimitive = sb.intPrimitive",
		"onr-on-select-b":    "on SupportBean sb select * from MyInfraFour w where w.intPrimitive = sb.intPrimitive and w.theString = sb.theString",
		"inv-create-window":  "@public create window MyInfraOne#keepall as (f1 string, f2 int)",
		"inv-create-table":   "@public create table MyInfraOne as (f1 string primary key, f2 int primary key)",
		"inv-index":          "create index MyInfraIndex on MyInfraOne(f1)",
		"inv-ctx-one":        "@public create context ContextOne initiated by SupportBean terminated after 5 sec",
		"inv-ctx-two":        "@public create context ContextTwo initiated by SupportBean terminated after 5 sec",
		"inv-create-ctx-nw":  "@public context ContextOne create window MyInfraCtx#keepall as (f1 string, f2 int)",
		"inv-create-ctx-tbl": "@public context ContextOne create table MyInfraCtx as (f1 string primary key, f2 int primary key)",
		"inv-create-two-nw":  "@Name('create') @public create window MyInfraTwo#keepall as SupportBean",
		"inv-create-two-tbl": "@Name('create') @public create table MyInfraTwo(theString string primary key, intPrimitive int primary key)",
		"inv-insert-two":     "@Name('insert') insert into MyInfraTwo select theString, intPrimitive from SupportBean",
		"inv-unique-index":   "create unique index I1 on MyInfraTwo(theString)",
		"inv-create-nokey":   "@public create table MyTable (p0 string, sumint sum(int))",
		"inv-probe-ctx-a":    "create unique index IndexTwo on MyInfraCtx(f1)",
		"inv-probe-ctx-b":    "context ContextTwo create unique index IndexTwo on MyInfraCtx(f1)",
		"inv-probe-dup-idx":  "create index MyInfraIndex on MyInfraOne(f1)",
		"inv-probe-unknown":  "create index IndexTwo on MyInfraOne(fx)",
		"inv-probe-dup-col":  "create index IndexTwo on MyInfraOne(f1, f1)",
		"inv-probe-infra":    "create index IndexTwo on MyWindowX(f1, f1)",
		"inv-probe-bad-kind": "create index IndexTwo on MyInfraOne(f1 bubu, f2)",
		"inv-probe-gugu":     "create gugu index IndexTwo on MyInfraOne(f2)",
		"inv-probe-uniq-bt":  "create unique index IndexTwo on MyInfraOne(f2 btree)",
		"inv-probe-no-pk":    "create index MyIndex on MyTable(p0)",
	}
	got := map[string]string{
		"mcmi-create-window": infraNWTableMCMI561CreateWindow,
		"mcmi-create-table":  infraNWTableMCMI561CreateTable,
		"mcmi-insert":        infraNWTableMCMI561Insert,
		"mcmi-index1":        infraNWTableMCMI561Index1,
		"mcmi-index2":        infraNWTableMCMI561Index2,
		"mcmi-index3":        infraNWTableMCMI561Index3,
		"mcmi-select-f3":     infraNWTableMCMI561SelectF3,
		"mcmi-select-f3f2":   infraNWTableMCMI561SelectF3F2,
		"mcmi-select-full":   infraNWTableMCMI561SelectFull,
		"mcmi-select-f2":     infraNWTableMCMI561SelectF2,
		"mcmi-select-f1":     infraNWTableMCMI561SelectF1,
		"mcmi-select-all":    infraNWTableMCMI561SelectAll,
		"onr-create-window":  infraNWTableONR561CreateWindow,
		"onr-create-table":   infraNWTableONR561CreateTable,
		"onr-insert":         infraNWTableONR561Insert,
		"onr-index":          infraNWTableONR561Index,
		"onr-select-s0":      infraNWTableONR561SelectS0,
		"onr-select-two":     infraNWTableONR561SelectTwo,
		"onr-cw":             infraNWTableONR561CreateFour,
		"onr-cw-index":       infraNWTableONR561IndexFour,
		"onr-on-select-a":    infraNWTableONR561OnSelectA,
		"onr-on-select-b":    infraNWTableONR561OnSelectB,
		"inv-create-window":  infraNWTableINV561CreateWindow,
		"inv-create-table":   infraNWTableINV561CreateTable,
		"inv-index":          infraNWTableINV561Index,
		"inv-ctx-one":        infraNWTableINV561ContextOne,
		"inv-ctx-two":        infraNWTableINV561ContextTwo,
		"inv-create-ctx-nw":  infraNWTableINV561CreateCtxNW,
		"inv-create-ctx-tbl": infraNWTableINV561CreateCtxTBL,
		"inv-create-two-nw":  infraNWTableINV561CreateTwoNW,
		"inv-create-two-tbl": infraNWTableINV561CreateTwoTBL,
		"inv-insert-two":     infraNWTableINV561InsertTwo,
		"inv-unique-index":   infraNWTableINV561UniqueIndex,
		"inv-create-nokey":   infraNWTableINV561CreateNoKey,
		"inv-probe-ctx-a":    infraNWTableINV561ProbeCtxA,
		"inv-probe-ctx-b":    infraNWTableINV561ProbeCtxB,
		"inv-probe-dup-idx":  infraNWTableINV561ProbeDupIndex,
		"inv-probe-unknown":  infraNWTableINV561ProbeUnknownCol,
		"inv-probe-dup-col":  infraNWTableINV561ProbeDupCol,
		"inv-probe-infra":    infraNWTableINV561ProbeUnknownInf,
		"inv-probe-bad-kind": infraNWTableINV561ProbeBadKind,
		"inv-probe-gugu":     infraNWTableINV561ProbeGugu,
		"inv-probe-uniq-bt":  infraNWTableINV561ProbeUniqueBT,
		"inv-probe-no-pk":    infraNWTableINV561ProbeNoPK,
	}
	for name, want := range wants {
		if got[name] != want {
			t.Fatalf("EPL %s = %q, want %q", name, got[name], want)
		}
	}
	wantSteps := map[string]int{
		"mcmi-window":    20,
		"mcmi-table":     20,
		"onr-window":     29,
		"onr-table":      29,
		"invalid-window": 29,
		"invalid-table":  32,
	}
	for _, caseName := range infraNWTableIndexOps561Cases {
		if len(infraNWTableIndexOps561CaseSteps[caseName]) != wantSteps[caseName] {
			t.Fatalf("case %q pinned steps = %d, want %d",
				caseName, len(infraNWTableIndexOps561CaseSteps[caseName]), wantSteps[caseName])
		}
	}
	if infraNWTableINV561ProbeNullTyped !=
		"create schema MyMap(somefield null);\ncreate window MyWindow#keepall as MyMap;\ncreate unique index MyIndex on MyWindow(somefield)" {
		t.Fatalf("null-typed probe = %q", infraNWTableINV561ProbeNullTyped)
	}
}

// TestRunInfraNWTableIndexOps561ScenarioLoader pins the strict loader: the
// checked-in scenario parses, and runtime-id drift plus step-count drift
// are both rejected.
func TestRunInfraNWTableIndexOps561ScenarioLoader(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	scenarioPath := filepath.Join(root, infraNWTableIndexOps561ID+".json")
	scenarioFile, err := os.Open(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	_, loadErr := loadInfraNWTableIndexOps561Scenario(scenarioFile)
	closeErr := scenarioFile.Close()
	if loadErr != nil {
		t.Fatalf("checked-in scenario rejected: %v", loadErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	data, err := os.ReadFile(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	var mutated map[string]any
	if err := json.Unmarshal(data, &mutated); err != nil {
		t.Fatal(err)
	}
	runtimes := mutated["javaRuntimes"].([]any)
	mutated["javaRuntimes"] = []any{"java-runtime-mutated", runtimes[1], runtimes[2],
		runtimes[3], runtimes[4], runtimes[5]}
	raw, err := json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableIndexOps561Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("runtime-id drift accepted")
	}
	mutated["javaRuntimes"] = runtimes
	steps := mutated["steps"].([]any)
	mutated["steps"] = steps[:len(steps)-1]
	raw, err = json.Marshal(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadInfraNWTableIndexOps561Scenario(strings.NewReader(string(raw))); err == nil {
		t.Fatal("step-count drift accepted")
	}
}

func TestRunInfraNWTableIndexOps561DiffWritesPassingEvidence(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := infraNWTableIndexOps561JavaTraceFixture(t, func(*compat.Trace) {})
	evidencePath := filepath.Join(t.TempDir(), infraNWTableIndexOps561ID+".evidence.json")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableIndexOps561ID + "-diff",
		"-scenario", filepath.Join(root, infraNWTableIndexOps561ID+".json"),
		"-java-trace", javaTracePath,
		"-evidence", evidencePath,
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("diff exit code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("passing diff wrote stdout = %q", stdout.String())
	}
	evidence, err := loadDifferentialEvidenceFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("evidence = %#v", evidence)
	}
	if evidence.JavaCommit != infraNWTableIndexOps561JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableIndexOps561JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableIndexOps561JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableIndexOps561JavaExecutions) {
		t.Fatalf("Java metadata = %#v", evidence)
	}
	assertInfraNWTableIndexOps561Trace(t, evidence.JavaTrace)
	assertInfraNWTableIndexOps561Trace(t, evidence.GoTrace)
}

func TestRunInfraNWTableIndexOps561DiffRejectsTraceMutations(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	tests := []struct {
		name   string
		mutate func(*compat.Trace)
	}{
		{
			// The f1 probe pins the concat projection; a drifted f3 value
			// means the insert-into feed lost the '>'||'<' decoration.
			name: "concat-value-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "mcmi-window" && rec.Statement == "select-f1" && len(rec.New) > 0 {
						rec.New[0].Fields["f3"] = "E1"
						return
					}
				}
				panic("no mcmi-window select-f1 snapshot record")
			},
		},
		{
			// The on-select listener pins {E1,1}; a drifted f2 means the
			// f2 = s0.id join picked the wrong row.
			name: "onr-listener-row-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "onr-window" && rec.Operation == "listener" && len(rec.New) > 0 {
						rec.New[0].Fields["f2"] = json.Number("2")
						return
					}
				}
				panic("no onr-window listener record")
			},
		},
		{
			// A pinned index-count value drift means the declared/implicit
			// index bookkeeping no longer matches the oracle.
			name: "index-count-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "onr-table" && rec.Operation == "index-count" && rec.Count != nil {
						*rec.Count = 3
						return
					}
				}
				panic("no onr-table index-count record")
			},
		},
		{
			// The unique-violation send pins the Java statement-attributed
			// text; a drifted message means the violation surfacing changed.
			name: "send-error-text-drift",
			mutate: func(trace *compat.Trace) {
				for index := range trace.Records {
					rec := &trace.Records[index]
					if rec.Case == "invalid-table" && rec.Operation == "send-error" {
						rec.Value = "different"
						return
					}
				}
				panic("no invalid-table send-error record")
			},
		},
		{
			name: "deployed-order",
			mutate: func(trace *compat.Trace) {
				trace.Records[0], trace.Records[1] = trace.Records[1], trace.Records[0]
			},
		},
		{
			name: "record-count-short",
			mutate: func(trace *compat.Trace) {
				trace.Records = trace.Records[:len(trace.Records)-1]
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			javaTracePath := infraNWTableIndexOps561JavaTraceFixture(t, test.mutate)
			evidencePath := filepath.Join(t.TempDir(), infraNWTableIndexOps561ID+".evidence.json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"-mode", infraNWTableIndexOps561ID + "-diff",
				"-scenario", filepath.Join(root, infraNWTableIndexOps561ID+".json"),
				"-java-trace", javaTracePath,
				"-evidence", evidencePath,
			}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("mutation %q unexpectedly passed; stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
			evidence, err := loadDifferentialEvidenceFile(evidencePath)
			if err != nil {
				t.Fatal(err)
			}
			if evidence.Status != "different" || len(evidence.Differences) == 0 {
				t.Fatalf("mutation %q evidence = %#v", test.name, evidence)
			}
		})
	}
}

// TestRunInfraNWTableIndexOps561CheckedInEvidenceMatchesTraceAndReplay
// verifies the landed Java trace, Go trace and evidence against the
// runner's pinned constants; the test is inert until the unit's
// differential artifacts are checked in next to the scenario.
func TestRunInfraNWTableIndexOps561CheckedInEvidenceMatchesTraceAndReplay(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	javaTracePath := filepath.Join(root, infraNWTableIndexOps561ID+".trace.json")
	goTracePath := filepath.Join(root, infraNWTableIndexOps561ID+".go.trace.json")
	evidencePath := filepath.Join(root, infraNWTableIndexOps561ID+".evidence.json")
	for _, path := range []string{javaTracePath, goTracePath, evidencePath} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("differential artifacts not checked in yet: %v", err)
		}
	}
	javaTrace, err := loadTraceFile(javaTracePath)
	if err != nil {
		t.Fatal(err)
	}
	goTrace, err := loadTraceFile(goTracePath)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := loadDifferentialEvidenceFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "passing" || len(evidence.Differences) != 0 {
		t.Fatalf("checked-in evidence = %#v", evidence)
	}
	if differences := compat.DiffTraces(javaTrace, evidence.JavaTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Java trace differs from checked-in trace: %#v", differences)
	}
	if differences := compat.DiffTraces(goTrace, evidence.GoTrace); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from checked-in Go trace: %#v", differences)
	}
	if evidence.JavaCommit != infraNWTableIndexOps561JavaCommit ||
		!reflect.DeepEqual(evidence.JavaRuntimeIDs, infraNWTableIndexOps561JavaRuntimeIDs) ||
		!reflect.DeepEqual(evidence.JavaSourceFiles, infraNWTableIndexOps561JavaSources) ||
		!reflect.DeepEqual(evidence.JavaExecutions, infraNWTableIndexOps561JavaExecutions) {
		t.Fatalf("checked-in Java metadata = %#v", evidence)
	}
	assertInfraNWTableIndexOps561Trace(t, javaTrace)
	assertInfraNWTableIndexOps561Trace(t, goTrace)

	var stdout, stderr bytes.Buffer
	if code := Run([]string{
		"-mode", infraNWTableIndexOps561ID,
		"-scenario", filepath.Join(root, infraNWTableIndexOps561ID+".json"),
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("replay exit code = %d, stderr = %q", code, stderr.String())
	}
	replayed, err := compat.LoadTrace(strings.NewReader(stdout.String()))
	if err != nil {
		t.Fatal(err)
	}
	if differences := compat.DiffTraces(evidence.GoTrace, replayed); len(differences) != 0 {
		t.Fatalf("checked-in evidence Go trace differs from current replay: %#v", differences)
	}
	assertInfraNWTableIndexOps561Trace(t, replayed)
}

func TestRunInfraNWTableIndexOps561RejectsMalformedRawScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableIndexOps561ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "top-level-extra", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"steps": [`), []byte(`"extra": 0, "steps": [`), 1)
		}},
		{name: "step-extra", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`{"op": "deploy", "case": "mcmi-window", "statement": "create"`),
				[]byte(`{"op": "deploy", "case": "mcmi-window", "statement": "create", "extra": 0`), 1)
		}},
		{name: "case-runtime-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"runtimeId": "java-runtime-b3fc4383cc57caee42ff"`),
				[]byte(`"runtimeId": "java-runtime-wrong"`), 1)
		}},
		{name: "epl-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`select * from MyInfraMCMI where f2=-2`),
				[]byte(`select * from MyInfraMCMI where f2=-3`), 1)
		}},
		{name: "payload-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"intPrimitive":-2`),
				[]byte(`"intPrimitive":-5`), 1)
		}},
		{name: "snapshot-mode-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"statement": "select-f2", "mode": "any"`),
				[]byte(`"statement": "select-f2", "mode": "ordered"`), 1)
		}},
		{name: "index-count-drift", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"op": "index-count", "case": "onr-table", "statement": "MyInfraONR", "create": "create", "of": "indexes", "count": 2`),
				[]byte(`"op": "index-count", "case": "onr-table", "statement": "MyInfraONR", "create": "create", "of": "indexes", "count": 3`), 1)
		}},
		{name: "trailing-json", mutate: func(data []byte) []byte {
			return append(append([]byte(nil), data...), []byte("\n{}\n")...)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := test.mutate(data)
			if bytes.Equal(mutated, data) {
				t.Fatalf("raw mutation %q did not change scenario", test.name)
			}
			scenarioPath := filepath.Join(t.TempDir(), "scenario.json")
			if err := os.WriteFile(scenarioPath, mutated, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := Run([]string{
				"-mode", infraNWTableIndexOps561ID,
				"-scenario", scenarioPath,
			}, &stdout, &stderr); code == 0 {
				t.Fatalf("malformed scenario %q unexpectedly replayed: stdout=%q stderr=%q", test.name, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunInfraNWTableIndexOps561RuntimeIDMappingMatchesScenario(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "parity")
	data, err := os.ReadFile(filepath.Join(root, infraNWTableIndexOps561ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		JavaRuntimes  []string `json:"javaRuntimes"`
		JavaNames     []string `json:"javaNames"`
		JavaStaticIDs []string `json:"javaStaticIds"`
		JavaFlags     []string `json:"javaFlags"`
		Cases         []struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw.JavaRuntimes, infraNWTableIndexOps561JavaRuntimeIDs) ||
		!reflect.DeepEqual(raw.JavaNames, infraNWTableIndexOps561JavaExecutions) ||
		!reflect.DeepEqual(raw.JavaStaticIDs, infraNWTableIndexOps561JavaStaticIDs) ||
		!reflect.DeepEqual(raw.JavaFlags, infraNWTableIndexOps561JavaFlags) {
		t.Fatalf("scenario Java identity = %#v", raw)
	}
	for index, definition := range raw.Cases {
		if definition.Case != infraNWTableIndexOps561Cases[index] ||
			definition.Ordinal != infraNWTableIndexOps561Ordinals[index] ||
			definition.RuntimeID != infraNWTableIndexOps561JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableIndexOps561JavaExecutions[index] {
			t.Fatalf("case %d mapping = %#v", index, definition)
		}
	}
}
