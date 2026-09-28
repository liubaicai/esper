package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the ViewExpressionBatch/ViewExpressionWindow
// variable quartet: ords 11/12 of each pinned executions() collection —
// four statement-level executions whose variable text is deployed inside
// a create-variable module and mutated through runtimeSetVariable under
// the virtual clock.
//
// Covered executions:
//   - ViewExpressionBatch ord 11 ViewExpressionBatchDynamicTimeBatch
//     java-runtime-ace804f86e16f62ae8f5 (case dynamic-time-batch)
//   - ViewExpressionBatch ord 12 ViewExpressionBatchVariableBatch
//     java-runtime-afb34438de3f25c964cb (case variable-batch)
//   - ViewExpressionWindow ord 11 ViewExpressionWindowVariable
//     java-runtime-8f167cedfd9b5d56fe67 (case variable-window)
//   - ViewExpressionWindow ord 12 ViewExpressionWindowDynamicTimeWindow
//     java-runtime-5ccd88d79fe264701059 (case dynamic-time-window)
//
// dynamic-time-batch pins the strict `newest_timestamp - oldest_timestamp
// > SIZE` trigger: E1@1000/E2@1900 accumulate silently, shrinking SIZE
// to 500 followed by the 1901 advance flushes new {E1,E2} WITHOUT a new
// send, E3@1901/E4@2300 accumulate (the 2500 advance stays silent — a
// clock advance alone with a satisfied-looking span under the OLD value
// re-evaluates and still does not fire), E5@2500 flushes new {E3,E4,E5}/
// old {E1,E2}, E6@3100/E7@3700 accumulate under the raised SIZE=999 and
// E8@4100 flushes new {E6,E7,E8}/old {E3,E4,E5}.
//
// variable-batch pins the boolean POST toggle: POST=false keeps E1@1000
// silent, flipping POST=true plus the 1001 advance flushes new {E1}
// without a send, E2 and E3 each flush the pending batch (new {E2}/old
// {E1}, new {E3}/old {E2}), flipping POST=false silences E4/E5 and the
// 2000 advance, flipping POST=true plus the 2001 advance flushes
// new {E4,E5}/old {E3} and E6 flushes new {E6}/old {E4,E5}.
//
// variable-window pins the KEEP toggle over #expr(KEEP): E1@1000 is
// retained, KEEP=false alone does NOT evict (the iterator still reads
// {E1}), the 1001 advance delivers the old-only {E1} expel, E2@1001
// self-expires as an {E2}/{E2} pair and after KEEP=true is restored E3
// is retained.
//
// dynamic-time-window pins the strict
// `newest_timestamp - oldest_timestamp < SIZE` keep: E1@1000 accumulates,
// E2@2000 expires E1 (the span 1000 is not < 1000), SIZE=10000 keeps
// {E2,E3} at E3@5000 and shrinking to SIZE=2000 lets the 6000 advance
// lazily evict E2 so E4@6000 leaves {E3,E4}.
//
// The Java regression runs milestone() calls as regression-harness
// savepoints only; they restore identical state for these non-contextual
// executions, so the scenario omits them (they pin no observable). The
// batch cases pin listener deliveries only (assertPropsPerRowIRPair /
// assertListenerNotInvoked); the window cases pin in-order iterator
// snapshots at every assertPropsPerRowIterator plus the listener
// deliveries, including W11's old-only expel record and W12's lazy
// eviction record on the 6000 advance.
const viewExprVar571ID = "view-expression-variable-571"

const viewExprVar571Description = "ViewExpressionBatch ords 11/12 + ViewExpressionWindow ords 11/12 — the variable quartet. dynamic-time-batch (ViewExpressionBatchDynamicTimeBatch, ord 11) replays `create variable long SIZE = 1000; @name('s0') select irstream * from SupportBean#expr_batch(newest_timestamp - oldest_timestamp > SIZE)` (strict `>`, trailing newline): E1@1000 and E2@1900 accumulate silently, runtimeSetVariable(s0,SIZE,500) plus the 1901 advance flushes new {E1,E2} WITHOUT a new send, E3/E4 accumulate through the silent 2500 advance, E5@2500 flushes new {E3,E4,E5}/old {E1,E2}, E6@3100 and E7@3700 accumulate under SIZE=999 and E8@4100 flushes new {E6,E7,E8}/old {E3,E4,E5}. variable-batch (ViewExpressionBatchVariableBatch, ord 12) replays `create variable boolean POST = false; @name('s0') select irstream * from SupportBean#expr_batch(POST)` (trailing newline): POST=false keeps E1@1000 silent, POST=true plus the 1001 advance flushes new {E1} without a send, E2 and E3 each flush the pending batch, POST=false silences E4/E5 and the 2000 advance, POST=true plus the 2001 advance flushes new {E4,E5}/old {E3} and E6 flushes new {E6}/old {E4,E5}. variable-window (ViewExpressionWindowVariable, ord 11) replays `create variable boolean KEEP = true; @name('s0') select irstream * from SupportBean#expr(KEEP)` (trailing newline): E1@1000 is retained, KEEP=false alone does not evict (iterator still {E1}), the 1001 advance delivers the old-only {E1} expel, E2@1001 self-expires as an {E2}/{E2} pair and after KEEP=true is restored E3@1001 is retained. dynamic-time-window (ViewExpressionWindowDynamicTimeWindow, ord 12) replays `create variable long SIZE = 1000; @name('s0') select irstream * from SupportBean#expr(newest_timestamp - oldest_timestamp < SIZE)` (strict `<`, NO trailing semicolon/newline — byte-exact asymmetry pinned): E1@1000 accumulates, E2@2000 expires E1 (span 1000 is not < 1000), SIZE=10000 keeps {E2,E3} at E3@5000 and shrinking to SIZE=2000 lets the 6000 advance lazily evict E2 so E4@6000 leaves {E3,E4}. Java milestone() calls are regression-harness savepoints with identical restored state for these executions, so the scenario omits them."

const viewExprVar571JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprVar571JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view"

// Byte-exact EPL pins (ViewExpressionBatch.java lines 242-243 for
// dynamic-time-batch and 202-203 for variable-batch; ViewExpressionWindow.java
// lines 238-239 for variable-window and 275-276 for dynamic-time-window; the
// trailing `;\n` on the batch EPLs and variable-window versus the missing
// `;\n` on dynamic-time-window is pinned verbatim).
const (
	viewExprVar571EPLDynamicTimeBatch  = "create variable long SIZE = 1000;\n@name('s0') select irstream * from SupportBean#expr_batch(newest_timestamp - oldest_timestamp > SIZE);\n"
	viewExprVar571EPLVariableBatch     = "create variable boolean POST = false;\n@name('s0') select irstream * from SupportBean#expr_batch(POST);\n"
	viewExprVar571EPLVariableWindow    = "create variable boolean KEEP = true;\n@name('s0') select irstream * from SupportBean#expr(KEEP);\n"
	viewExprVar571EPLDynamicTimeWindow = "create variable long SIZE = 1000;\n@name('s0') select irstream * from SupportBean#expr(newest_timestamp - oldest_timestamp < SIZE)"

	viewExprVar571TimeEpoch        = "1970-01-01T00:00:00.000Z"
	viewExprVar571Time1000         = "1970-01-01T00:00:01.000Z"
	viewExprVar571Time1001         = "1970-01-01T00:00:01.001Z"
	viewExprVar571Time1900         = "1970-01-01T00:00:01.900Z"
	viewExprVar571Time1901         = "1970-01-01T00:00:01.901Z"
	viewExprVar571Time2000         = "1970-01-01T00:00:02.000Z"
	viewExprVar571Time2001         = "1970-01-01T00:00:02.001Z"
	viewExprVar571Time2300         = "1970-01-01T00:00:02.300Z"
	viewExprVar571Time2500         = "1970-01-01T00:00:02.500Z"
	viewExprVar571Time3100         = "1970-01-01T00:00:03.100Z"
	viewExprVar571Time3700         = "1970-01-01T00:00:03.700Z"
	viewExprVar571Time4100         = "1970-01-01T00:00:04.100Z"
	viewExprVar571Time5000         = "1970-01-01T00:00:05.000Z"
	viewExprVar571Time6000         = "1970-01-01T00:00:06.000Z"
	viewExprVar571SupportBeanEvent = "SupportBean"
)

var (
	viewExprVar571JavaRuntimeIDs = []string{
		"java-runtime-ace804f86e16f62ae8f5",
		"java-runtime-afb34438de3f25c964cb",
		"java-runtime-8f167cedfd9b5d56fe67",
		"java-runtime-5ccd88d79fe264701059",
	}
	viewExprVar571JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	viewExprVar571JavaExecutions = []string{
		"ViewExpressionBatchDynamicTimeBatch",
		"ViewExpressionBatchVariableBatch",
		"ViewExpressionWindowVariable",
		"ViewExpressionWindowDynamicTimeWindow",
	}
	// Deduplicated inventory ids: the two batch runtime rows share static id
	// java-20551a17cb2af08c67fc and the two window runtime rows share
	// java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
	viewExprVar571JavaStaticIDs = []string{
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
	}
	viewExprVar571JavaFlags = []string{}
)

// viewExprVar571Bean mirrors the SupportBean properties the executions use:
// theString (the listener/snapshot field) and intPrimitive (carried by the
// pinned sends but not projected; the executions assert theString only).
type viewExprVar571Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// viewExprVar571DeploySpec binds one scenario deploy label to its byte-exact
// statement text (the full create-variable module). Every case deploys once.
type viewExprVar571DeploySpec struct {
	label string
	epl   string
}

// viewExprVar571VariableSpec pins the declared variable each case registers
// before deploy and mutates through set-variable: the Java name, the declared
// kind (bool or long) and the initial value (POST=false, KEEP=true,
// SIZE=1000).
type viewExprVar571VariableSpec struct {
	name    string
	initial any
}

// viewExprVar571CaseSpec pins one Java execution: case identity, observation
// text, the case-level EPL pin, the deploy-label sequence, the declared
// variable and the pinned listener/snapshot field projection.
type viewExprVar571CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []viewExprVar571DeploySpec
	variable    viewExprVar571VariableSpec
	fields      []string
}

var viewExprVar571CaseSpecs = []viewExprVar571CaseSpec{
	{
		name:      "dynamic-time-batch",
		ordinal:   11,
		runtimeID: "java-runtime-ace804f86e16f62ae8f5",
		execution: "ViewExpressionBatchDynamicTimeBatch",
		observation: "deployed+listener; create variable long SIZE = 1000 with " +
			"#expr_batch(newest_timestamp - oldest_timestamp > SIZE): E1@1000 and " +
			"E2@1900 accumulate silently, runtimeSetVariable(s0,SIZE,500) plus the " +
			"1901 advance flushes new {E1,E2} WITHOUT a new send, E3/E4 accumulate " +
			"through the silent 2500 advance, E5@2500 flushes new {E3,E4,E5}/old " +
			"{E1,E2}, E6@3100 and E7@3700 accumulate under SIZE=999 and E8@4100 " +
			"flushes new {E6,E7,E8}/old {E3,E4,E5}; the Java execution pins " +
			"flattened IR pairs only, so no snapshots are recorded",
		epl: viewExprVar571EPLDynamicTimeBatch,
		deploys: []viewExprVar571DeploySpec{
			{label: "s0", epl: viewExprVar571EPLDynamicTimeBatch},
		},
		variable: viewExprVar571VariableSpec{name: "SIZE", initial: int64(1000)},
		fields:   []string{"theString"},
	},
	{
		name:      "variable-batch",
		ordinal:   12,
		runtimeID: "java-runtime-afb34438de3f25c964cb",
		execution: "ViewExpressionBatchVariableBatch",
		observation: "deployed+listener; create variable boolean POST = false with " +
			"#expr_batch(POST): POST=false keeps E1@1000 silent, POST=true plus the " +
			"1001 advance flushes new {E1} without a send, E2 and E3 each flush the " +
			"pending batch (new {E2}/old {E1}, new {E3}/old {E2}), POST=false " +
			"silences E4/E5 and the 2000 advance, POST=true plus the 2001 advance " +
			"flushes new {E4,E5}/old {E3} and E6 flushes new {E6}/old {E4,E5}; the " +
			"Java execution pins flattened IR pairs only, so no snapshots are " +
			"recorded",
		epl: viewExprVar571EPLVariableBatch,
		deploys: []viewExprVar571DeploySpec{
			{label: "s0", epl: viewExprVar571EPLVariableBatch},
		},
		variable: viewExprVar571VariableSpec{name: "POST", initial: false},
		fields:   []string{"theString"},
	},
	{
		name:      "variable-window",
		ordinal:   11,
		runtimeID: "java-runtime-8f167cedfd9b5d56fe67",
		execution: "ViewExpressionWindowVariable",
		observation: "deployed+listener+snapshot; create variable boolean KEEP = " +
			"true with #expr(KEEP): E1@1000 is retained, KEEP=false alone does not " +
			"evict (the iterator still reads {E1}), the 1001 advance delivers the " +
			"old-only {E1} expel and an empty iterator, E2@1001 self-expires as an " +
			"{E2}/{E2} pair with an empty iterator, and after KEEP=true is " +
			"restored E3@1001 is retained (new-only delivery, iterator {E3}); " +
			"in-order iterator pins project theString at every " +
			"assertPropsPerRowIterator",
		epl: viewExprVar571EPLVariableWindow,
		deploys: []viewExprVar571DeploySpec{
			{label: "s0", epl: viewExprVar571EPLVariableWindow},
		},
		variable: viewExprVar571VariableSpec{name: "KEEP", initial: true},
		fields:   []string{"theString"},
	},
	{
		name:      "dynamic-time-window",
		ordinal:   12,
		runtimeID: "java-runtime-5ccd88d79fe264701059",
		execution: "ViewExpressionWindowDynamicTimeWindow",
		observation: "deployed+listener+snapshot; create variable long SIZE = " +
			"1000 with #expr(newest_timestamp - oldest_timestamp < SIZE) (strict " +
			"`<`, no trailing semicolon): E1@1000 accumulates, E2@2000 expires E1 " +
			"(span 1000 is not < 1000), SIZE=10000 keeps {E2,E3} at E3@5000 and " +
			"shrinking to SIZE=2000 lets the 6000 advance lazily evict E2 so " +
			"E4@6000 leaves {E3,E4}; in-order iterator pins project theString at " +
			"every assertPropsPerRowIterator",
		epl: viewExprVar571EPLDynamicTimeWindow,
		deploys: []viewExprVar571DeploySpec{
			{label: "s0", epl: viewExprVar571EPLDynamicTimeWindow},
		},
		variable: viewExprVar571VariableSpec{name: "SIZE", initial: int64(1000)},
		fields:   []string{"theString"},
	},
}

func viewExprVar571CaseSpecFor(name string) (viewExprVar571CaseSpec, bool) {
	for _, spec := range viewExprVar571CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprVar571CaseSpec{}, false
}

// viewExprVar571StepPin pins one scenario step's shape.
type viewExprVar571StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
	name      string
	value     any
	mode      string
}

func viewExprVar571DeployPin(statement, epl string) viewExprVar571StepPin {
	return viewExprVar571StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprVar571DeployedPin(statement string) viewExprVar571StepPin {
	return viewExprVar571StepPin{op: "deployed", statement: statement}
}

func viewExprVar571SendPin(theString string, intPrimitive int) viewExprVar571StepPin {
	return viewExprVar571StepPin{
		op:        "send",
		eventType: viewExprVar571SupportBeanEvent,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprVar571AdvancePin(at string) viewExprVar571StepPin {
	return viewExprVar571StepPin{op: "advance-time", at: at}
}

func viewExprVar571SetVariablePin(statement, name string, value any) viewExprVar571StepPin {
	return viewExprVar571StepPin{op: "set-variable", statement: statement, name: name, value: value}
}

func viewExprVar571SnapshotPin(statement, mode string) viewExprVar571StepPin {
	return viewExprVar571StepPin{op: "snapshot", statement: statement, mode: mode}
}

func viewExprVar571UndeployAllPin() viewExprVar571StepPin {
	return viewExprVar571StepPin{op: "undeploy-all"}
}

// viewExprVar571CaseSteps pins the complete step sequence per case in Java
// source order. Milestones are omitted (regression-harness savepoints with
// identical restored state); advance-time steps sit exactly where the Java
// execution calls env.advanceTime, set-variable steps mirror
// runtimeSetVariable(s0, name, value) and snapshot steps sit where Java
// asserts the iterator. The advances right after each variable change pin
// that a variable update re-evaluates expiry/flush on the next clock tick —
// the E1/E2 flush at 1901, the POST flushes at 1001/2001, the E1 expel at
// 1001 and the lazy E2 eviction at 6000 all fire WITHOUT an intervening
// send.
var viewExprVar571CaseSteps = map[string][]viewExprVar571StepPin{
	"dynamic-time-batch": {
		viewExprVar571AdvancePin(viewExprVar571TimeEpoch),
		viewExprVar571DeployPin("s0", viewExprVar571EPLDynamicTimeBatch),
		viewExprVar571DeployedPin("s0"),
		viewExprVar571AdvancePin(viewExprVar571Time1000),
		viewExprVar571SendPin("E1", 0),
		viewExprVar571AdvancePin(viewExprVar571Time1900),
		viewExprVar571SendPin("E2", 0),
		viewExprVar571SetVariablePin("s0", "SIZE", int64(500)),
		viewExprVar571AdvancePin(viewExprVar571Time1901),
		viewExprVar571SendPin("E3", 0),
		viewExprVar571AdvancePin(viewExprVar571Time2300),
		viewExprVar571SendPin("E4", 0),
		viewExprVar571AdvancePin(viewExprVar571Time2500),
		viewExprVar571SendPin("E5", 0),
		viewExprVar571AdvancePin(viewExprVar571Time3100),
		viewExprVar571SendPin("E6", 0),
		viewExprVar571SetVariablePin("s0", "SIZE", int64(999)),
		viewExprVar571AdvancePin(viewExprVar571Time3700),
		viewExprVar571SendPin("E7", 0),
		viewExprVar571AdvancePin(viewExprVar571Time4100),
		viewExprVar571SendPin("E8", 0),
		viewExprVar571UndeployAllPin(),
	},
	"variable-batch": {
		viewExprVar571AdvancePin(viewExprVar571TimeEpoch),
		viewExprVar571DeployPin("s0", viewExprVar571EPLVariableBatch),
		viewExprVar571DeployedPin("s0"),
		viewExprVar571AdvancePin(viewExprVar571Time1000),
		viewExprVar571SendPin("E1", 1),
		viewExprVar571SetVariablePin("s0", "POST", true),
		viewExprVar571AdvancePin(viewExprVar571Time1001),
		viewExprVar571SendPin("E2", 1),
		viewExprVar571SendPin("E3", 1),
		viewExprVar571SetVariablePin("s0", "POST", false),
		viewExprVar571SendPin("E4", 1),
		viewExprVar571SendPin("E5", 2),
		viewExprVar571AdvancePin(viewExprVar571Time2000),
		viewExprVar571SetVariablePin("s0", "POST", true),
		viewExprVar571AdvancePin(viewExprVar571Time2001),
		viewExprVar571SendPin("E6", 1),
		viewExprVar571UndeployAllPin(),
	},
	"variable-window": {
		viewExprVar571AdvancePin(viewExprVar571TimeEpoch),
		viewExprVar571DeployPin("s0", viewExprVar571EPLVariableWindow),
		viewExprVar571DeployedPin("s0"),
		viewExprVar571AdvancePin(viewExprVar571Time1000),
		viewExprVar571SendPin("E1", 1),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571SetVariablePin("s0", "KEEP", false),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571AdvancePin(viewExprVar571Time1001),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571SendPin("E2", 2),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571SetVariablePin("s0", "KEEP", true),
		viewExprVar571SendPin("E3", 3),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571UndeployAllPin(),
	},
	"dynamic-time-window": {
		viewExprVar571AdvancePin(viewExprVar571TimeEpoch),
		viewExprVar571DeployPin("s0", viewExprVar571EPLDynamicTimeWindow),
		viewExprVar571DeployedPin("s0"),
		viewExprVar571AdvancePin(viewExprVar571Time1000),
		viewExprVar571SendPin("E1", 0),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571AdvancePin(viewExprVar571Time2000),
		viewExprVar571SendPin("E2", 0),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571SetVariablePin("s0", "SIZE", int64(10000)),
		viewExprVar571AdvancePin(viewExprVar571Time5000),
		viewExprVar571SendPin("E3", 0),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571SetVariablePin("s0", "SIZE", int64(2000)),
		viewExprVar571AdvancePin(viewExprVar571Time6000),
		viewExprVar571SendPin("E4", 0),
		viewExprVar571SnapshotPin("s0", "ordered"),
		viewExprVar571UndeployAllPin(),
	},
}

func loadViewExprVar571Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprVar571ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprVar571ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprVar571ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprVar571ID, err)
	}
	if err := requireViewExprVar571Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version        string   `json:"version"`
		ID             string   `json:"id"`
		Description    string   `json:"description"`
		JavaCommit     string   `json:"javaCommit"`
		JavaSource     string   `json:"javaSource"`
		JavaSourceFile []string `json:"javaSourceFiles"`
		JavaRuntimes   []string `json:"javaRuntimes"`
		JavaNames      []string `json:"javaNames"`
		JavaStaticIDs  []string `json:"javaStaticIds"`
		JavaFlags      []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprVar571ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprVar571ID ||
		metadata.Description != viewExprVar571Description ||
		metadata.JavaCommit != viewExprVar571JavaCommit ||
		metadata.JavaSource != viewExprVar571JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprVar571ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprVar571JavaSources},
		{metadata.JavaRuntimes, viewExprVar571JavaRuntimeIDs},
		{metadata.JavaNames, viewExprVar571JavaExecutions},
		{metadata.JavaStaticIDs, viewExprVar571JavaStaticIDs},
		{metadata.JavaFlags, viewExprVar571JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprVar571ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprVar571CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprVar571ID, len(viewExprVar571CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprVar571Fields(object, "case", "ordinal", "runtimeId",
			"executionName", "observation", "epl", "deploys"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string   `json:"case"`
			Ordinal       int      `json:"ordinal"`
			RuntimeID     string   `json:"runtimeId"`
			ExecutionName string   `json:"executionName"`
			Observation   string   `json:"observation"`
			EPL           string   `json:"epl"`
			Deploys       []string `json:"deploys"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := viewExprVar571CaseSpecs[index]
		labels := make([]string, 0, len(spec.deploys))
		for _, pin := range spec.deploys {
			labels = append(labels, pin.label)
		}
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution ||
			definition.Observation != spec.observation || definition.EPL != spec.epl ||
			!reflect.DeepEqual(definition.Deploys, labels) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprVar571ID)
	}
	if err := validateViewExprVar571RawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateViewExprVar571RawSteps pins the complete step sequence per case:
// op field whitelists per step kind plus positional comparison against the
// pinned sequence.
func validateViewExprVar571RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprVar571CaseSpecs))
	positions := make(map[string]int)
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string: %w", index, err)
		}
		var step struct {
			Case      string          `json:"case"`
			Statement string          `json:"statement"`
			EPL       string          `json:"epl"`
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
			At        string          `json:"at"`
			Name      string          `json:"name"`
			Mode      string          `json:"mode"`
		}
		if operation == "case" {
			if err := requireViewExprVar571Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprVar571CaseSpecFor(marker.Case); !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, marker.Case)
			}
			currentCase = marker.Case
			caseOrder = append(caseOrder, marker.Case)
			continue
		}
		if currentCase == "" {
			return fmt.Errorf("scenario step %d is outside any case block", index)
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if step.Case != currentCase {
			return fmt.Errorf("scenario step %d declares case %q inside the %q block",
				index, step.Case, currentCase)
		}
		pins := viewExprVar571CaseSteps[currentCase]
		position := positions[currentCase]
		if position >= len(pins) {
			return fmt.Errorf("scenario step %d exceeds the pinned %s step sequence",
				index, currentCase)
		}
		pin := pins[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q for case %q position %d",
				index, operation, pin.op, currentCase, position)
		}
		switch operation {
		case "deploy":
			if err := requireViewExprVar571Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprVar571Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprVar571Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "advance-time":
			if err := requireViewExprVar571Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q",
					index, currentCase)
			}
		case "set-variable":
			if err := requireViewExprVar571Fields(object, "op", "case", "statement", "name", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var value any
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return fmt.Errorf("scenario step %d set-variable payload: %w", index, err)
			}
			if step.Statement != pin.statement || step.Name != pin.name ||
				!reflect.DeepEqual(normalizeViewExprVar571VariableValue(value), pin.value) {
				return fmt.Errorf("scenario step %d set-variable is not pinned for case %q",
					index, currentCase)
			}
		case "snapshot":
			if err := requireViewExprVar571Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprVar571Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprVar571CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprVar571CaseSpecs))
	}
	for index, spec := range viewExprVar571CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprVar571CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprVar571CaseSteps[spec.name]))
		}
	}
	return nil
}

// normalizeViewExprVar571VariableValue renders a decoded set-variable payload
// the way the pins store it: JSON numbers decode to float64 while the pinned
// long assignments are int64.
func normalizeViewExprVar571VariableValue(value any) any {
	if number, ok := value.(float64); ok {
		if number == float64(int64(number)) {
			return int64(number)
		}
	}
	return value
}

func requireViewExprVar571Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has %d fields, want %d", len(object), len(names))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

// viewExprVar571CaseState carries per-case replay state: the deployed
// statements (label -> statement), deployments for undeploy-all, the
// positional deploy counter, the listener sequence counters and the
// delivery records.
type viewExprVar571CaseState struct {
	spec        viewExprVar571CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	now         time.Time
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	deployCount int
	sequence    map[string]uint64
	records     []compat.TraceRecord
}

func viewExprVar571StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprVar571Bean](env, viewExprVar571SupportBeanEvent); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprVar571Scenario replays all four executions, one fresh engine per
// case like the Java oracle's per-execution runtime.
func runViewExprVar571Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprVar571Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprVar571CaseSpecs {
		records, err := runViewExprVar571Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprVar571ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprVar571Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprVar571ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprVar571ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprVar571RawSteps(rawSteps)
}

func runViewExprVar571Case(ctx context.Context, scenario compat.Scenario, spec viewExprVar571CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprVar571StartEnvironment()
	if err != nil {
		return nil, err
	}
	if err := env.RegisterVariable(spec.variable.name, spec.variable.initial); err != nil {
		return nil, fmt.Errorf("%s: register variable %q: %w", viewExprVar571ID, spec.variable.name, err)
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprVar571CaseState{
		spec:       spec,
		env:        env,
		engine:     esper.NewEngine(env, esper.WithRuntimeURI(spec.runtimeID), esper.WithStartTime(now)),
		now:        now,
		statements: make(map[string]*esper.Statement),
		sequence:   make(map[string]uint64),
	}
	defer func() { _ = state.engine.Close(context.Background()) }()

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		switch step.Op {
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "deployed":
			if _, ok := state.statements[step.Statement]; !ok {
				return nil, fmt.Errorf("%s: deployed marker for unknown statement %q",
					viewExprVar571ID, step.Statement)
			}
			state.sequence[step.Statement+":deployed"]++
			state.records = append(state.records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(state.now),
			})
		case "send":
			payload, err := viewExprVar571DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("%s: parse advance-time %q: %w", viewExprVar571ID, step.At, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
				return nil, err
			}
			state.now = at
		case "set-variable":
			if err := state.setVariable(ctx, step); err != nil {
				return nil, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return nil, err
			}
		case "undeploy-all":
			for _, deployment := range state.deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
			}
			state.deployments = nil
			state.statements = make(map[string]*esper.Statement)
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprVar571ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned deploy step onto the fluent Go equivalent of the
// Java statement. The byte-exact EPL text of the Java compileDeploy call —
// the full create-variable module — is pinned by the loader; the declared
// variable is registered on the environment at case start so the
// VariableRef inside the window predicate resolves.
func (s *viewExprVar571CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.deployCount >= len(s.spec.deploys) {
		return fmt.Errorf("%s: case %q deploy %q exceeds the pinned deployments",
			viewExprVar571ID, s.spec.name, step.Statement)
	}
	pin := s.spec.deploys[s.deployCount]
	s.deployCount++
	if pin.label != step.Statement || pin.epl != step.Epl {
		return fmt.Errorf("%s: case %q deploy %q is not pinned",
			viewExprVar571ID, s.spec.name, step.Statement)
	}
	if _, ok := s.statements[pin.label]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprVar571ID, pin.label)
	}
	query, err := s.deployQuery(step.Epl)
	if err != nil {
		return err
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", viewExprVar571ID, pin.label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", viewExprVar571ID, pin.label, err)
	}
	s.deployments = append(s.deployments, deployment)
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements",
			viewExprVar571ID, pin.label, len(statements))
	}
	statement := statements[0]
	s.statements[pin.label] = statement
	label := pin.label
	_, err = statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequence[label]++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: label,
			Sequence:  s.sequence[label],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       projectRecords(compat.NormalizeResults(batch.New), s.spec.fields),
			Old:       projectRecords(compat.NormalizeResults(batch.Old), s.spec.fields),
		})
		return nil
	})
	return err
}

// deployQuery builds the typed query matching one pinned EPL. All four are
// `select irstream` statements over a single SupportBean expression view;
// the declared variable surfaces as a VariableRef inside the trigger/keep
// predicate. WithOldStream surfaces both streams like Java's irstream and
// every statement carries Java's addListener("s0").
func (s *viewExprVar571CaseState) deployQuery(epl string) (esper.Query, error) {
	from := esper.From[viewExprVar571Bean](s.env, viewExprVar571SupportBeanEvent)
	switch epl {
	case viewExprVar571EPLDynamicTimeBatch:
		// `newest_timestamp - oldest_timestamp > SIZE`: the strict `>`
		// trigger over the pending batch's arrival-timestamp span. The
		// SIZE=500 shrink pins that a variable change plus a clock advance
		// flushes the pending batch WITHOUT a new send.
		trigger := esper.Greater[int64](
			esper.Subtract[int64](esper.WindowNewestTimestamp(), esper.WindowOldestTimestamp()),
			esper.VariableRef[int64]("SIZE"))
		return from.Window(esper.ExpressionBatch(trigger)).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprVar571EPLVariableBatch:
		// `expr_batch(POST)`: the boolean variable is itself the trigger —
		// while true every arrival closes the pending batch (the Java form
		// defaults to include-trigger-event).
		return from.Window(esper.ExpressionBatch(esper.VariableRef[bool]("POST"))).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprVar571EPLVariableWindow:
		// `expr(KEEP)`: the boolean variable is the keep predicate —
		// KEEP=false means arriving and retained rows expire, and the
		// eviction surfaces on the next clock tick as old-only data.
		return from.Window(esper.ExpressionWindow(esper.VariableRef[bool]("KEEP"))).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewExprVar571EPLDynamicTimeWindow:
		// `newest_timestamp - oldest_timestamp < SIZE`: the strict `<`
		// keep over arrival-timestamp spans; a SIZE shrink lazily evicts
		// out-of-window rows on the next tick, not at setVariable time.
		predicate := esper.Less[int64](
			esper.Subtract[int64](esper.WindowNewestTimestamp(), esper.WindowOldestTimestamp()),
			esper.VariableRef[int64]("SIZE"))
		return from.Window(esper.ExpressionWindow(predicate)).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	}
	return esper.Query{}, fmt.Errorf("%s: case %q EPL is not pinned", viewExprVar571ID, s.spec.name)
}

// setVariable executes one runtimeSetVariable, mirroring
// env.runtimeSetVariable("s0", name, value); the Go engine's variable
// surface is environment-scoped, so the pinned statement label is verified
// but not addressed. Like Java's setVariableValue the write itself stays
// silent — the re-evaluation delivery surfaces on the next advance-time
// step, so no record is emitted here.
func (s *viewExprVar571CaseState) setVariable(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" || step.Name != s.spec.variable.name {
		return fmt.Errorf("%s: case %q set-variable %s.%s is not pinned",
			viewExprVar571ID, s.spec.name, step.Statement, step.Name)
	}
	var value any
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return fmt.Errorf("%s: decode set-variable payload: %w", viewExprVar571ID, err)
	}
	return s.engine.SetVariable(ctx, step.Name, normalizeViewExprVar571VariableValue(value))
}

// snapshot emits one iterator record for the statement, projecting the
// pinned field set. Row order stays engine-native; all pinned snapshots are
// mode "ordered" (assertPropsPerRowIterator in-order).
func (s *viewExprVar571CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed",
			viewExprVar571ID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	rows := projectRecords(compat.NormalizeResults(result.Batch.New), s.spec.fields)
	record := compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.now),
	}
	if len(rows) > 0 {
		record.New = rows
	}
	s.records = append(s.records, record)
	return nil
}

// viewExprVar571DecodePayload converts a scenario send payload into the
// typed host object: a SupportBean struct with the pinned theString and
// intPrimitive fields.
func viewExprVar571DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprVar571SupportBeanEvent:
		if err := requireViewExprVar571Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean viewExprVar571Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprVar571ID, step.EventType)
	}
}
