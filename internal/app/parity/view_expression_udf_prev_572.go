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
// UDF+Prev quartet: batch ords 3/5 and window ords 4/6 of the pinned
// executions() collections — four statement-level executions whose
// expiry/keep expression is a plug-in single-row function or whose
// select clause projects prev(1, theString) over the expression view.
//
// Covered executions:
//   - ViewExpressionBatch ord 3 ViewExpressionBatchUDFBuiltin
//     java-runtime-5804edaff708d2d83a5e (case udf-batch)
//   - ViewExpressionBatch ord 5 ViewExpressionBatchPrev
//     java-runtime-3bec407608d81eebcc12 (case prev-batch)
//   - ViewExpressionWindow ord 4 ViewExpressionWindowUDFBuiltin
//     java-runtime-2ea3302a92f786153790 (case udf-window)
//   - ViewExpressionWindow ord 6 ViewExpressionWindowPrev
//     java-runtime-9171e672cfde4fa06cb5 (case prev-window)
//
// udf-batch pins the TestSuiteView `udf` plug-in single-row function
// (ViewExpressionWindow.LocalUDF.evaluateExpiryUDF(String key,
// Object viewref, Integer expiryCount)) inside #expr_batch: with
// result=true E1 flushes the single-row batch (new {E1}) and E2
// flushes its own batch; plain `select *` statements are istream
// only, so the prior batch's {E1} rstream row never reaches the
// listener (new {E2} alone). With result=false E3 stays silent. The
// pinned UDF observations carry key/expiryCount/viewref:
// expired_count is ALWAYS 0 because expr_batch evaluates the arriving
// event only — batch expiry is a whole-batch flush, never per-row.
//
// udf-window pins the same UDF inside #expr: E1 and E2 are retained
// (new-only deliveries), then result=false at E3 delivers new {E3}
// alone — the mass-expiry {E1,E2,E3} flows on the remove stream the
// istream statement never surfaces. The keep predicate fails for both
// retained rows (expired_count reaches 2) and then for the arriving
// row itself, so the pinned observation reads key=E3 expiryCount=2
// viewref non-null. Java asserts the LocalUDF statics only after E1
// and E3, so the scenario pins udf-mode snapshots at those positions
// only; the Java
// execution performs no observation between E2's send and the
// setResult(false) flip.
//
// prev-batch pins current_count > 2 as the flush trigger with
// select-side prev: E1 and E2 accumulate silently and E3 flushes the
// whole 3-row batch in ONE delivery with val0 {null, E1, E2} — prev
// is evaluated per output row inside the flushed batch.
//
// prev-window pins expr(true) (a keep predicate that never expires)
// with the same select-side prev: deliveries are per-row, val0 null
// at E1 and E1 at E2.
//
// The Java regression runs compileDeployAddListenerMileZero /
// milestone() calls as regression-harness savepoints only; they
// restore identical state for these non-contextual executions, so
// the scenario omits them (they pin no observable). The Java
// executions assert the UDF statics via env.assertThat and never
// assert listener shapes, but the listener deliveries are
// deterministic on both engines so they are recorded symmetrically.
const viewExprUDFPrev572ID = "view-expression-udf-prev-572"

const viewExprUDFPrev572Description = "ViewExpressionBatch ords 3/5 + ViewExpressionWindow ords 4/6 — the UDF+Prev quartet. udf-batch (ViewExpressionBatchUDFBuiltin, ord 3) replays `@name('s0') select * from SupportBean#expr_batch(udf(theString, view_reference, expired_count))` with the TestSuiteView udf plug-in single-row function (ViewExpressionWindow.LocalUDF.evaluateExpiryUDF(String key, Object viewref, Integer expiryCount) -> bool, registered via addPlugInSingleRowFunction with threadPoolCompilerNumThreads=0): result=true flushes E1 as new {E1} and E2 as new {E2} — plain-select istream statements surface NO remove stream, so the prior batch's {E1} rstream row never reaches the listener — and result=false keeps E3 silent; pinned UDF observations read key=E1 then key=E3, expiryCount=0 at both (expr_batch evaluates the arriving event only — batch expiry is a whole-batch flush, so expired_count never counts retained rows), viewref non-null. udf-window (ViewExpressionWindowUDFBuiltin, ord 4) replays `select * from SupportBean#expr(udf(theString, view_reference, expired_count))`: E1 and E2 are retained as new-only deliveries, then result=false at E3 delivers new {E3} alone — the mass-expiry {E1,E2,E3} flows on the remove stream the istream statement never surfaces — while the pinned observation reads key=E3 expiryCount=2 viewref non-null (the keep predicate fails for both retained rows before the arriving row's own evaluation). prev-batch (ViewExpressionBatchPrev, ord 5) replays `select prev(1, theString) as val0 from SupportBean#expr_batch(current_count > 2)`: E1 and E2 accumulate silently and E3 flushes the whole batch as one 3-row delivery with val0 {null,E1,E2} — prev is evaluated per output row inside the flushed batch. prev-window (ViewExpressionWindowPrev, ord 6) replays `select prev(1, theString) as val0 from SupportBean#expr(true)`: per-row deliveries carry val0 null at E1 then val0 E1 at E2; expr(true) never expires so no old data flows. Java milestone()/mileZero calls are regression-harness savepoints with identical restored state for these executions, so the scenario omits them; the Java executions assert LocalUDF statics only after E1 and E3, so udf-mode snapshots pin those two positions per UDF case."

const viewExprUDFPrev572JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const viewExprUDFPrev572JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view"

// Byte-exact EPL pins (ViewExpressionBatchUDFBuiltin at
// ViewExpressionBatch.java:284-285, ViewExpressionBatchPrev at :350-351,
// ViewExpressionWindowUDFBuiltin at ViewExpressionWindow.java:313-314 and
// ViewExpressionWindowPrev at :374-375; none carries a trailing `;\n`).
const (
	viewExprUDFPrev572EPLUDFBatch   = "@name('s0') select * from SupportBean#expr_batch(udf(theString, view_reference, expired_count))"
	viewExprUDFPrev572EPLPrevBatch  = "@name('s0') select prev(1, theString) as val0 from SupportBean#expr_batch(current_count > 2)"
	viewExprUDFPrev572EPLUDFWindow  = "@name('s0') select * from SupportBean#expr(udf(theString, view_reference, expired_count))"
	viewExprUDFPrev572EPLPrevWindow = "@name('s0') select prev(1, theString) as val0 from SupportBean#expr(true)"
	viewExprUDFPrev572SupportBean   = "SupportBean"
	viewExprUDFPrev572UDFResultName = "udf-result"
	viewExprUDFPrev572SnapshotUDF   = "udf"
)

var (
	viewExprUDFPrev572JavaRuntimeIDs = []string{
		"java-runtime-5804edaff708d2d83a5e",
		"java-runtime-3bec407608d81eebcc12",
		"java-runtime-2ea3302a92f786153790",
		"java-runtime-9171e672cfde4fa06cb5",
	}
	viewExprUDFPrev572JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionBatch.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java",
		"regression-run/src/test/java/com/espertech/esper/regressionrun/suite/view/TestSuiteView.java",
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
	}
	viewExprUDFPrev572JavaExecutions = []string{
		"ViewExpressionBatchUDFBuiltin",
		"ViewExpressionBatchPrev",
		"ViewExpressionWindowUDFBuiltin",
		"ViewExpressionWindowPrev",
	}
	// Deduplicated inventory ids: the two batch runtime rows share static
	// id java-20551a17cb2af08c67fc and the two window runtime rows share
	// java-06e6b1f6c905b8f12b82, pinned once per runtimeId row.
	viewExprUDFPrev572JavaStaticIDs = []string{
		"java-20551a17cb2af08c67fc",
		"java-20551a17cb2af08c67fc",
		"java-06e6b1f6c905b8f12b82",
		"java-06e6b1f6c905b8f12b82",
	}
	viewExprUDFPrev572JavaFlags = []string{}
)

// viewExprUDFPrev572Bean mirrors the SupportBean properties the
// executions use: theString (the projected/UDF-key field) and
// intPrimitive (carried by the pinned sends but not projected).
type viewExprUDFPrev572Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// viewExprUDFPrev572DeploySpec binds one scenario deploy label to its
// byte-exact statement text. Every case deploys a single s0.
type viewExprUDFPrev572DeploySpec struct {
	label string
	epl   string
}

// viewExprUDFPrev572CaseSpec pins one Java execution: case identity,
// observation text, the case-level EPL pin, the deploy-label sequence,
// whether the case drives the `udf` plug-in (set-udf-result toggles plus
// udf-mode snapshot observations) and the pinned listener field
// projection.
type viewExprUDFPrev572CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	deploys     []viewExprUDFPrev572DeploySpec
	udf         bool
	fields      []string
}

var viewExprUDFPrev572CaseSpecs = []viewExprUDFPrev572CaseSpec{
	{
		name:      "udf-batch",
		ordinal:   3,
		runtimeID: "java-runtime-5804edaff708d2d83a5e",
		execution: "ViewExpressionBatchUDFBuiltin",
		observation: "deployed+listener+udf; #expr_batch(udf(theString, " +
			"view_reference, expired_count)) with the TestSuiteView udf " +
			"plug-in: result=true flushes E1 as new {E1} and E2 as new " +
			"{E2} (plain-select istream statements surface no remove " +
			"stream, so the prior batch's {E1} rstream row never " +
			"reaches the listener), result=false keeps E3 silent; the " +
			"pinned observations read key=E1/key=E3 with expiryCount=0 " +
			"at both (expr_batch evaluates the arriving event only, so " +
			"expired_count never counts retained rows) and viewref " +
			"non-null; the Java execution asserts listener shape never, " +
			"so no iterator snapshots are recorded",
		epl: viewExprUDFPrev572EPLUDFBatch,
		deploys: []viewExprUDFPrev572DeploySpec{
			{label: "s0", epl: viewExprUDFPrev572EPLUDFBatch},
		},
		udf:    true,
		fields: []string{"theString"},
	},
	{
		name:      "prev-batch",
		ordinal:   5,
		runtimeID: "java-runtime-3bec407608d81eebcc12",
		execution: "ViewExpressionBatchPrev",
		observation: "deployed+listener; select prev(1, theString) as val0 " +
			"over #expr_batch(current_count > 2): E1 and E2 accumulate " +
			"silently and E3 flushes the whole batch as ONE 3-row " +
			"delivery with val0 {null,E1,E2} — prev is evaluated per " +
			"output row inside the flushed batch; the Java execution " +
			"pins the flattened IR pair only, so no snapshots are " +
			"recorded",
		epl: viewExprUDFPrev572EPLPrevBatch,
		deploys: []viewExprUDFPrev572DeploySpec{
			{label: "s0", epl: viewExprUDFPrev572EPLPrevBatch},
		},
		fields: []string{"val0"},
	},
	{
		name:      "udf-window",
		ordinal:   4,
		runtimeID: "java-runtime-2ea3302a92f786153790",
		execution: "ViewExpressionWindowUDFBuiltin",
		observation: "deployed+listener+udf; #expr(udf(theString, " +
			"view_reference, expired_count)) with the TestSuiteView udf " +
			"plug-in: E1 and E2 are retained as new-only deliveries, " +
			"result=false at E3 delivers new {E3} alone — the " +
			"mass-expiry {E1,E2,E3} flows on the remove stream the " +
			"istream statement never surfaces; the pinned observations " +
			"read key=E1 then key=E3 expiryCount=2 viewref non-null — " +
			"the keep predicate fails for both retained rows before " +
			"the arriving row's own evaluation; the Java execution " +
			"asserts the statics only after E1 and E3, so no iterator " +
			"snapshots are recorded",
		epl: viewExprUDFPrev572EPLUDFWindow,
		deploys: []viewExprUDFPrev572DeploySpec{
			{label: "s0", epl: viewExprUDFPrev572EPLUDFWindow},
		},
		udf:    true,
		fields: []string{"theString"},
	},
	{
		name:      "prev-window",
		ordinal:   6,
		runtimeID: "java-runtime-9171e672cfde4fa06cb5",
		execution: "ViewExpressionWindowPrev",
		observation: "deployed+listener; select prev(1, theString) as " +
			"val0 over #expr(true): deliveries are per-row, val0 null " +
			"at E1 and val0 E1 at E2; expr(true) never expires so no " +
			"old data flows; the Java execution pins assertPropsNew " +
			"only, so no snapshots are recorded",
		epl: viewExprUDFPrev572EPLPrevWindow,
		deploys: []viewExprUDFPrev572DeploySpec{
			{label: "s0", epl: viewExprUDFPrev572EPLPrevWindow},
		},
		fields: []string{"val0"},
	},
}

func viewExprUDFPrev572CaseSpecFor(name string) (viewExprUDFPrev572CaseSpec, bool) {
	for _, spec := range viewExprUDFPrev572CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewExprUDFPrev572CaseSpec{}, false
}

// viewExprUDFPrev572StepPin pins one scenario step's shape.
type viewExprUDFPrev572StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	name      string
	value     any
	mode      string
}

func viewExprUDFPrev572DeployPin(statement, epl string) viewExprUDFPrev572StepPin {
	return viewExprUDFPrev572StepPin{op: "deploy", statement: statement, epl: epl}
}

func viewExprUDFPrev572DeployedPin(statement string) viewExprUDFPrev572StepPin {
	return viewExprUDFPrev572StepPin{op: "deployed", statement: statement}
}

func viewExprUDFPrev572SendPin(theString string, intPrimitive int) viewExprUDFPrev572StepPin {
	return viewExprUDFPrev572StepPin{
		op:        "send",
		eventType: viewExprUDFPrev572SupportBean,
		payload:   map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)},
	}
}

func viewExprUDFPrev572SetResultPin(value bool) viewExprUDFPrev572StepPin {
	return viewExprUDFPrev572StepPin{op: "set-variable", name: viewExprUDFPrev572UDFResultName, value: value}
}

func viewExprUDFPrev572UDFSnapshotPin(statement string) viewExprUDFPrev572StepPin {
	return viewExprUDFPrev572StepPin{op: "snapshot", statement: statement, mode: viewExprUDFPrev572SnapshotUDF}
}

func viewExprUDFPrev572UndeployAllPin() viewExprUDFPrev572StepPin {
	return viewExprUDFPrev572StepPin{op: "undeploy-all"}
}

// viewExprUDFPrev572CaseSteps pins the complete step sequence per case
// in Java source order. Milestones are omitted (regression-harness
// savepoints with identical restored state); set-variable steps named
// "udf-result" mirror ViewExpressionWindow.LocalUDF.setResult calls —
// the scenario op vocabulary has no UDF-specific op, so the result
// toggle rides the generic set-variable carrier pinned by name; and
// udf-mode snapshot steps sit exactly where the Java execution's
// env.assertThat reads the LocalUDF statics (after E1 and after E3 —
// never after E2, which the Java executions do not observe).
var viewExprUDFPrev572CaseSteps = map[string][]viewExprUDFPrev572StepPin{
	"udf-batch": {
		viewExprUDFPrev572DeployPin("s0", viewExprUDFPrev572EPLUDFBatch),
		viewExprUDFPrev572DeployedPin("s0"),
		viewExprUDFPrev572SetResultPin(true),
		viewExprUDFPrev572SendPin("E1", 0),
		viewExprUDFPrev572UDFSnapshotPin("s0"),
		viewExprUDFPrev572SendPin("E2", 0),
		viewExprUDFPrev572SetResultPin(false),
		viewExprUDFPrev572SendPin("E3", 0),
		viewExprUDFPrev572UDFSnapshotPin("s0"),
		viewExprUDFPrev572UndeployAllPin(),
	},
	"prev-batch": {
		viewExprUDFPrev572DeployPin("s0", viewExprUDFPrev572EPLPrevBatch),
		viewExprUDFPrev572DeployedPin("s0"),
		viewExprUDFPrev572SendPin("E1", 1),
		viewExprUDFPrev572SendPin("E2", 2),
		viewExprUDFPrev572SendPin("E3", 3),
		viewExprUDFPrev572UndeployAllPin(),
	},
	"udf-window": {
		viewExprUDFPrev572DeployPin("s0", viewExprUDFPrev572EPLUDFWindow),
		viewExprUDFPrev572DeployedPin("s0"),
		viewExprUDFPrev572SetResultPin(true),
		viewExprUDFPrev572SendPin("E1", 0),
		viewExprUDFPrev572UDFSnapshotPin("s0"),
		viewExprUDFPrev572SendPin("E2", 0),
		viewExprUDFPrev572SetResultPin(false),
		viewExprUDFPrev572SendPin("E3", 0),
		viewExprUDFPrev572UDFSnapshotPin("s0"),
		viewExprUDFPrev572UndeployAllPin(),
	},
	"prev-window": {
		viewExprUDFPrev572DeployPin("s0", viewExprUDFPrev572EPLPrevWindow),
		viewExprUDFPrev572DeployedPin("s0"),
		viewExprUDFPrev572SendPin("E1", 1),
		viewExprUDFPrev572SendPin("E2", 2),
		viewExprUDFPrev572UndeployAllPin(),
	},
}

func loadViewExprUDFPrev572Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewExprUDFPrev572ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewExprUDFPrev572ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprUDFPrev572ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewExprUDFPrev572ID, err)
	}
	if err := requireViewExprUDFPrev572Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewExprUDFPrev572ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewExprUDFPrev572ID ||
		metadata.Description != viewExprUDFPrev572Description ||
		metadata.JavaCommit != viewExprUDFPrev572JavaCommit ||
		metadata.JavaSource != viewExprUDFPrev572JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", viewExprUDFPrev572ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, viewExprUDFPrev572JavaSources},
		{metadata.JavaRuntimes, viewExprUDFPrev572JavaRuntimeIDs},
		{metadata.JavaNames, viewExprUDFPrev572JavaExecutions},
		{metadata.JavaStaticIDs, viewExprUDFPrev572JavaStaticIDs},
		{metadata.JavaFlags, viewExprUDFPrev572JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", viewExprUDFPrev572ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewExprUDFPrev572CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			viewExprUDFPrev572ID, len(viewExprUDFPrev572CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewExprUDFPrev572Fields(object, "case", "ordinal", "runtimeId",
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
		spec := viewExprUDFPrev572CaseSpecs[index]
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", viewExprUDFPrev572ID)
	}
	if err := validateViewExprUDFPrev572RawSteps(rawSteps); err != nil {
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

// validateViewExprUDFPrev572RawSteps pins the complete step sequence per
// case: op field whitelists per step kind plus positional comparison
// against the pinned sequence.
func validateViewExprUDFPrev572RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(viewExprUDFPrev572CaseSpecs))
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
			Name      string          `json:"name"`
			Mode      string          `json:"mode"`
		}
		if operation == "case" {
			if err := requireViewExprUDFPrev572Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := viewExprUDFPrev572CaseSpecFor(marker.Case); !ok {
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
		spec, ok := viewExprUDFPrev572CaseSpecFor(currentCase)
		if !ok {
			return fmt.Errorf("scenario step %d selects unknown case %q", index, currentCase)
		}
		pins := viewExprUDFPrev572CaseSteps[currentCase]
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
			if err := requireViewExprUDFPrev572Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "deployed":
			if err := requireViewExprUDFPrev572Fields(object, "op", "case", "statement"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement {
				return fmt.Errorf("scenario step %d deployed marker is not pinned for case %q",
					index, currentCase)
			}
		case "send":
			if err := requireViewExprUDFPrev572Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "set-variable":
			if !spec.udf {
				return fmt.Errorf("scenario step %d set-variable is only pinned for udf cases", index)
			}
			if err := requireViewExprUDFPrev572Fields(object, "op", "case", "name", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var value bool
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return fmt.Errorf("scenario step %d set-variable payload: %w", index, err)
			}
			if step.Name != pin.name || value != pin.value {
				return fmt.Errorf("scenario step %d set-variable is not pinned for case %q",
					index, currentCase)
			}
		case "snapshot":
			if !spec.udf {
				return fmt.Errorf("scenario step %d snapshot is only pinned for udf cases", index)
			}
			if err := requireViewExprUDFPrev572Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q",
					index, currentCase)
			}
		case "undeploy-all":
			if err := requireViewExprUDFPrev572Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(viewExprUDFPrev572CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(viewExprUDFPrev572CaseSpecs))
	}
	for index, spec := range viewExprUDFPrev572CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(viewExprUDFPrev572CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(viewExprUDFPrev572CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireViewExprUDFPrev572Fields(object map[string]json.RawMessage, names ...string) error {
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

// viewExprUDFPrev572CaseState carries per-case replay state: the
// deployed statements (label -> statement), deployments for
// undeploy-all, the positional deploy counter, the listener/deployed/
// observation sequence counters, the `udf` plug-in's mirrored LocalUDF
// statics (result toggle plus the last invocation's key/expiryCount/
// viewref) and the delivery records.
type viewExprUDFPrev572CaseState struct {
	spec        viewExprUDFPrev572CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	now         time.Time
	statements  map[string]*esper.Statement
	deployments []*esper.Deployment
	deployCount int
	sequence    map[string]uint64
	udfResult   bool
	udfSeen     bool
	udfKey      string
	udfExpiry   int64
	udfViewref  bool
	records     []compat.TraceRecord
}

func viewExprUDFPrev572StartEnvironment() (*esper.Environment, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewExprUDFPrev572Bean](env, viewExprUDFPrev572SupportBean); err != nil {
		return nil, err
	}
	return env, nil
}

// runViewExprUDFPrev572Scenario replays all four executions, one fresh
// engine per case like the Java oracle's per-execution runtime.
func runViewExprUDFPrev572Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewExprUDFPrev572Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range viewExprUDFPrev572CaseSpecs {
		records, err := runViewExprUDFPrev572Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewExprUDFPrev572ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateViewExprUDFPrev572Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != viewExprUDFPrev572ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, viewExprUDFPrev572ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateViewExprUDFPrev572RawSteps(rawSteps)
}

func runViewExprUDFPrev572Case(ctx context.Context, scenario compat.Scenario, spec viewExprUDFPrev572CaseSpec) ([]compat.TraceRecord, error) {
	env, err := viewExprUDFPrev572StartEnvironment()
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &viewExprUDFPrev572CaseState{
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
					viewExprUDFPrev572ID, step.Statement)
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
			payload, err := viewExprUDFPrev572DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "set-variable":
			if err := state.setUDFResult(step); err != nil {
				return nil, err
			}
		case "snapshot":
			if err := state.udfObservation(step); err != nil {
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
			return nil, fmt.Errorf("%s: unsupported step op %q", viewExprUDFPrev572ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps each pinned deploy step onto the fluent Go equivalent of
// the Java statement. The byte-exact EPL text of the Java
// compileDeploy(AddListener) call is pinned by the loader; the
// Func3Ctx UDF inside the expression view mirrors the TestSuiteView
// `udf` plug-in registration.
func (s *viewExprUDFPrev572CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.deployCount >= len(s.spec.deploys) {
		return fmt.Errorf("%s: case %q deploy %q exceeds the pinned deployments",
			viewExprUDFPrev572ID, s.spec.name, step.Statement)
	}
	pin := s.spec.deploys[s.deployCount]
	s.deployCount++
	if pin.label != step.Statement || pin.epl != step.Epl {
		return fmt.Errorf("%s: case %q deploy %q is not pinned",
			viewExprUDFPrev572ID, s.spec.name, step.Statement)
	}
	if _, ok := s.statements[pin.label]; ok {
		return fmt.Errorf("%s: label %q is already deployed", viewExprUDFPrev572ID, pin.label)
	}
	query, err := s.deployQuery(step.Epl)
	if err != nil {
		return err
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", viewExprUDFPrev572ID, pin.label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", viewExprUDFPrev572ID, pin.label, err)
	}
	s.deployments = append(s.deployments, deployment)
	statements := deployment.Statements()
	if len(statements) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements",
			viewExprUDFPrev572ID, pin.label, len(statements))
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

// deployQuery builds the typed query matching one pinned EPL. The UDF
// cases wire the three-argument plug-in function as a Func3Ctx over
// Field(theString), WindowReference() and WindowExpiredCount() — the
// Java signature is udf(String key, Object viewref, Integer
// expiryCount): Go's WindowExpiredCount() yields int64, so C=int64.
// The Prev cases project prev(1, theString) as val0 in the select
// clause over the pinned expression view. Every pinned EPL is a plain
// `select` (istream only — no irstream/rstream keyword), so the query
// carries NO WithOldStream: remove-stream rows (the prior expr_batch
// flush, the expr mass-expiry) never reach the listener, matching the
// Java listener's new-only records. Every statement carries Java's
// addListener("s0").
func (s *viewExprUDFPrev572CaseState) deployQuery(epl string) (esper.Query, error) {
	from := esper.From[viewExprUDFPrev572Bean](s.env, viewExprUDFPrev572SupportBean)
	switch epl {
	case viewExprUDFPrev572EPLUDFBatch, viewExprUDFPrev572EPLUDFWindow:
		// `udf(theString, view_reference, expired_count)`: the plug-in
		// single-row function mirrors ViewExpressionWindow.LocalUDF.
		// evaluateExpiryUDF — it records the pinned observation fields
		// (key = theString, viewref presence, expired_count) and gates
		// on the result toggle set by the udf-result carrier steps.
		udf := esper.Func3Ctx[string, []esper.Event, int64, bool]("udf",
			func(key string, reference []esper.Event, expiryCount int64, _ esper.EvalContext) bool {
				s.udfSeen = true
				s.udfKey = key
				s.udfExpiry = expiryCount
				s.udfViewref = len(reference) > 0
				return s.udfResult
			},
			esper.Field[viewExprUDFPrev572Bean, string]("theString"),
			esper.WindowReference(),
			esper.WindowExpiredCount())
		if epl == viewExprUDFPrev572EPLUDFBatch {
			return from.Window(esper.ExpressionBatch(udf)).
				Query(esper.StatementName("s0")), nil
		}
		return from.Window(esper.ExpressionWindow(udf)).
			Query(esper.StatementName("s0")), nil
	case viewExprUDFPrev572EPLPrevBatch:
		// `expr_batch(current_count > 2)`: the strict `>` trigger flushes
		// the accumulated batch once three rows pend; prev(1, theString)
		// then evaluates per flushed row.
		source := from.Window(esper.ExpressionBatch(
			esper.Greater[int64](esper.WindowCurrentCount(), esper.Literal(int64(2)))))
		return esper.Select(source,
			esper.Alias("val0", esper.Prev[string](1, esper.Field[viewExprUDFPrev572Bean, string]("theString")))).
			Query(esper.StatementName("s0")), nil
	case viewExprUDFPrev572EPLPrevWindow:
		// `expr(true)`: the keep predicate never expires, so deliveries
		// are per arriving row and prev(1, theString) reads the prior
		// retained row.
		source := from.Window(esper.ExpressionWindow(esper.Literal(true)))
		return esper.Select(source,
			esper.Alias("val0", esper.Prev[string](1, esper.Field[viewExprUDFPrev572Bean, string]("theString")))).
			Query(esper.StatementName("s0")), nil
	}
	return esper.Query{}, fmt.Errorf("%s: case %q EPL is not pinned", viewExprUDFPrev572ID, s.spec.name)
}

// setUDFResult executes one udf-result carrier step, mirroring
// ViewExpressionWindow.LocalUDF.setResult: a static toggle on the
// plug-in function, addressed by name rather than statement and silent
// in the record stream (the flip's effect surfaces on the next send).
func (s *viewExprUDFPrev572CaseState) setUDFResult(step compat.Step) error {
	if !s.spec.udf || step.Name != viewExprUDFPrev572UDFResultName {
		return fmt.Errorf("%s: case %q set-variable %q is not pinned",
			viewExprUDFPrev572ID, s.spec.name, step.Name)
	}
	var value bool
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return fmt.Errorf("%s: decode udf-result payload: %w", viewExprUDFPrev572ID, err)
	}
	s.udfResult = value
	return nil
}

// udfObservation emits one "observation" record mirroring the Java
// execution's env.assertThat reads of the LocalUDF statics: key is the
// last-evaluated event's theString, expiryCount is the expired_count
// argument of that invocation and viewref pins the non-null
// view_reference assertion (Go's WindowReference slice is non-null by
// construction, so presence is pinned as len > 0).
func (s *viewExprUDFPrev572CaseState) udfObservation(step compat.Step) error {
	if !s.spec.udf || step.Mode != viewExprUDFPrev572SnapshotUDF || step.Statement != "s0" {
		return fmt.Errorf("%s: case %q snapshot mode %q is not pinned",
			viewExprUDFPrev572ID, s.spec.name, step.Mode)
	}
	if !s.udfSeen {
		return fmt.Errorf("%s: case %q udf observation before any udf invocation",
			viewExprUDFPrev572ID, s.spec.name)
	}
	s.sequence[step.Statement+":observation"]++
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "observation",
		Statement: step.Statement,
		Sequence:  s.sequence[step.Statement+":observation"],
		Name:      "udf",
		Time:      compat.FormatTraceTime(s.now),
		Value: map[string]any{
			"key":         s.udfKey,
			"expiryCount": s.udfExpiry,
			"viewref":     s.udfViewref,
		},
	})
	return nil
}

// viewExprUDFPrev572DecodePayload converts a scenario send payload into
// the typed host object: a SupportBean struct with the pinned theString
// and intPrimitive fields.
func viewExprUDFPrev572DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case viewExprUDFPrev572SupportBean:
		if err := requireViewExprUDFPrev572Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean viewExprUDFPrev572Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", viewExprUDFPrev572ID, step.EventType)
	}
}
