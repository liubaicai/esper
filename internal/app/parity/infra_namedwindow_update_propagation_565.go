package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the named-window update-propagation bundle: four
// executions that share one semantic — a named window's row lifecycle
// (insert-into feed, update-istream preprocessing, on-update trigger,
// grouped output-snapshot consumer and rstream chain) and how each mutation
// propagates to the window's downstream consumers.
//
// Covered executions (Java ordinals of their suite's executions()):
//   - EPLOtherUpdateIStream ord 6 EPLOtherUpdateNamedWindow      java-runtime-7c91cbd63d65e3078e25
//   - InfraNamedWindowOnUpdateWMultiDispatch ord 0               java-runtime-fcbbceeaf04849dc64bc
//   - InfraNamedWindowOutputrate ord 0                          java-runtime-da3a1e5e9ab73d4a067e
//   - InfraNamedWindowRemoveStream ord 0                        java-runtime-4971ea67797956c303ee
//
// update-namedwindow replays update-istream copy-on-write over a keepall
// window: the insert statement's listener observes the pre-update row while
// the create-window statement and the select consumer observe the updated
// row; on-select/on-insert triggers read the window post-update and the
// second update-istream preprocesses the on-insert's routed target. The
// Java milestones (env.milestone is a serde checkpoint in the harness and a
// no-op in the base runner) are modelled as redeploy barriers: an
// undeploy-all step destroys every deployment — including the named
// window's data, exactly like Esper's deployment teardown — followed by
// explicit redeploy steps and explicit replayed sends, so the Go runner
// rebuilds a fresh environment/engine at each barrier while the Java
// oracle undeploys and redeploys the same statements.
//
// onupdate-multidispatch replays the uncorrelated on-update trigger over
// the #time(25 hour)#firstunique(company) intersect window. Esper's
// on-trigger named-window action is preemptive: it runs before the same
// event's continuous insert, so the runner deploys the update trigger
// internally ahead of the insert (mirroring the deployment-order dispatch
// that Esper's preemptive hook observes). The Java contract leaves the
// final s0 multi-dispatch batch shape undefined ([1,2] or [2]); the count
// is pinned deterministically through iterator snapshots of s0 and the
// update trigger's old/new batches through its listener.
//
// outputrate-snapshot replays the grouped irstream count with output
// snapshot every 1 second over virtual time, including the t=3000 tick that
// re-emits identical group rows.
//
// removestream-chain replays the rstream cascade: insert rstream into W2
// select rstream * from W1 feeds W1's length(2) evictions into W2, whose
// own evictions feed W3, pinned by any-order iterator snapshots.
const infraNWUP565Id = "infra-namedwindow-update-propagation-565"

const infraNWUP565Description = "EPLOtherUpdateNamedWindow (EPLOtherUpdateIStream ord 6), InfraNamedWindowOnUpdateWMultiDispatch ord 0, InfraNamedWindowOutputrate ord 0 and InfraNamedWindowRemoveStream ord 0: one named-window row-lifecycle propagation bundle. update-namedwindow replays update-istream copy-on-write over a keepall AWindow — the insert listener sees the pre-update {E1,oldvalue} copy while the window/select/onselect/oninsert consumers see {E1,newvalue}, and the second update-istream rewrites MyOtherStream routed rows to {a,b}; the three Java milestones are redeploy barriers (undeploy-all plus explicit redeploy and send replay steps). onupdate-multidispatch replays the uncorrelated on S2 update of the #time(25 hour)#firstunique(company) intersect window — the preemptive trigger sees the window pre-insert, so totals run 0, 6+3=9, 9+5=8, 8+4=7 while the rejected firstunique duplicates and the admitted BComp row carry value 3/4/5/4 through select count snapshots (the Java-defined-ambiguous final s0 batch shape [1,2] or [2] is pinned deterministically through the s0 iterator instead of a listener batch). outputrate-snapshot replays select irstream theString, count(*) group-by-theString output snapshot every 1 second over virtual time, including the identical-repeat emission at t=3000. removestream-chain replays the insert-rstream cascade W1->W2->W3 over length(2) windows pinned by any-order iterator snapshots."

const infraNWUP565JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWUP565Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite"

// Byte-exact EPL pins (EPLOtherUpdateIStream.java lines 517-542 for the
// update-namedwindow case; InfraNamedWindowOnUpdateWMultiDispatch.java
// lines 37-45 for onupdate-multidispatch; InfraNamedWindowOutputrate.java
// lines 24-30 for outputrate-snapshot; InfraNamedWindowRemoveStream.java
// lines 26-32 for removestream-chain).
const (
	infraNWUP565E1Window    = "@name('window') @public create window AWindow#keepall select * from MyMapTypeNW"
	infraNWUP565E1Insert    = "@name('insert') insert into AWindow select * from MyMapTypeNW"
	infraNWUP565E1Select    = "@name('select') select * from AWindow"
	infraNWUP565E1Update    = "update istream AWindow set p1='newvalue'"
	infraNWUP565E1OnSelect  = "@name('onselect') on SupportBean(theString='A') select win.* from AWindow as win"
	infraNWUP565E1OnInsert  = "@name('oninsert') @public on SupportBean(theString='B') insert into MyOtherStream select win.* from AWindow as win"
	infraNWUP565E1UpdateOth = "update istream MyOtherStream set p0='a', p1='b'"
	infraNWUP565E1S0        = "@name('s0') select * from MyOtherStream"

	infraNWUP565E2Schema = "@public @buseventtype create schema S2 ( company string, value double, total double)"
	infraNWUP565E2Create = "@name('create') @public create window S2Win#time(25 hour)#firstunique(company) as S2"
	infraNWUP565E2Insert = "insert into S2Win select * from S2#firstunique(company)"
	infraNWUP565E2Update = "on S2 as a update S2Win as b set total = b.value + a.value"
	infraNWUP565E2S0     = "@name('s0') select count(*) as cnt from S2Win"

	infraNWUP565E3Create = "@public create window MyWindowOne#keepall as (theString string, intv int)"
	infraNWUP565E3Insert = "insert into MyWindowOne select theString, intPrimitive as intv from SupportBean"
	infraNWUP565E3S0     = "@name('s0') select irstream theString, count(*) as c from MyWindowOne group by theString output snapshot every 1 second"

	infraNWUP565E4CreateW1 = "@name('c1') @public create window W1#length(2) as select * from SupportBean"
	infraNWUP565E4CreateW2 = "@name('c2') @public create window W2#length(2) as select * from SupportBean"
	infraNWUP565E4CreateW3 = "@name('c3') @public create window W3#length(2) as select * from SupportBean"
	infraNWUP565E4Insert   = "insert into W1 select * from SupportBean"
	infraNWUP565E4RouteW2  = "insert rstream into W2 select rstream * from W1"
	infraNWUP565E4RouteW3  = "insert rstream into W3 select rstream * from W2"
)

var (
	infraNWUP565JavaRuntimeIDs = []string{
		"java-runtime-7c91cbd63d65e3078e25",
		"java-runtime-fcbbceeaf04849dc64bc",
		"java-runtime-da3a1e5e9ab73d4a067e",
		"java-runtime-4971ea67797956c303ee",
	}
	infraNWUP565JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherUpdateIStream.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdateWMultiDispatch.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOutputrate.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowRemoveStream.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean.java",
	}
	infraNWUP565JavaExecutions = []string{
		"EPLOtherUpdateNamedWindow",
		"InfraNamedWindowOnUpdateWMultiDispatch",
		"InfraNamedWindowOutputrate",
		"InfraNamedWindowRemoveStream",
	}
	infraNWUP565JavaStaticIDs = []string{
		"java-082395e7cb9dbac98bea",
		"java-608c6908a0d53bf60fb5",
		"java-3ffa81dff07fbd6b6a84",
		"java-5d1931fb11b4c00a1ecc",
	}
	infraNWUP565JavaFlags = []string{"EXCLUDEWHENINSTRUMENTED"}
)

// infraNWUP565CaseSpec pins one Java execution: case identity, the EPL
// deploy labels in Java statement order and the listener/snapshot
// projections the trace records carry.
type infraNWUP565CaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	description string
	deploys     []string            // deploy labels in Java step order
	epl         map[string]string   // label -> byte-exact EPL
	listened    []string            // labels whose deliveries emit listener records
	newOnly     map[string]bool     // labels whose old rows are unasserted
	rowFields   map[string][]string // label -> projected fields
}

var infraNWUP565CaseSpecs = []infraNWUP565CaseSpec{
	{
		name:        "update-namedwindow",
		ordinal:     6,
		runtimeID:   "java-runtime-7c91cbd63d65e3078e25",
		execution:   "EPLOtherUpdateNamedWindow",
		description: "update-istream copy-on-write over a keepall window with insert/window/select consumers, on-select and on-insert triggers and a second update-istream over the routed target; three milestone redeploy barriers",
		deploys: []string{
			"window", "insert", "select", "update", "onselect", "oninsert",
			"update-other", "s0",
		},
		epl: map[string]string{
			"window":       infraNWUP565E1Window,
			"insert":       infraNWUP565E1Insert,
			"select":       infraNWUP565E1Select,
			"update":       infraNWUP565E1Update,
			"onselect":     infraNWUP565E1OnSelect,
			"oninsert":     infraNWUP565E1OnInsert,
			"update-other": infraNWUP565E1UpdateOth,
			"s0":           infraNWUP565E1S0,
		},
		listened: []string{"window", "insert", "select", "onselect", "oninsert", "s0"},
		newOnly:  map[string]bool{},
		rowFields: map[string][]string{
			"window":   {"p0", "p1"},
			"insert":   {"p0", "p1"},
			"select":   {"p0", "p1"},
			"onselect": {"p0", "p1"},
			"oninsert": {"p0", "p1"},
			"s0":       {"p0", "p1"},
		},
	},
	{
		name:        "onupdate-multidispatch",
		ordinal:     0,
		runtimeID:   "java-runtime-fcbbceeaf04849dc64bc",
		execution:   "InfraNamedWindowOnUpdateWMultiDispatch",
		description: "uncorrelated on-update trigger over a time+firstunique intersect window: the preemptive trigger sees the pre-insert window so totals accumulate over retained value 3 (0->9->8->7) while BComp is admitted untouched; s0 count pinned by iterator snapshots (final batch shape [1,2] or [2] is defined-ambiguous in Java)",
		deploys:     []string{"schema", "create", "insert", "upd", "s0"},
		epl: map[string]string{
			"schema": infraNWUP565E2Schema,
			"create": infraNWUP565E2Create,
			"insert": infraNWUP565E2Insert,
			"upd":    infraNWUP565E2Update,
			"s0":     infraNWUP565E2S0,
		},
		listened: []string{"upd"},
		newOnly:  map[string]bool{},
		rowFields: map[string][]string{
			"upd":    {"company", "value", "total"},
			"create": {"company", "value", "total"},
			"s0":     {"cnt"},
		},
	},
	{
		name:        "outputrate-snapshot",
		ordinal:     0,
		runtimeID:   "java-runtime-da3a1e5e9ab73d4a067e",
		execution:   "InfraNamedWindowOutputrate",
		description: "grouped irstream count over a keepall window with output snapshot every 1 second: each virtual-time tick emits the full group set as new rows, including the identical-repeat emission at t=3000",
		deploys:     []string{"create", "insert", "s0"},
		epl: map[string]string{
			"create": infraNWUP565E3Create,
			"insert": infraNWUP565E3Insert,
			"s0":     infraNWUP565E3S0,
		},
		listened: []string{"s0"},
		newOnly:  map[string]bool{"s0": true},
		rowFields: map[string][]string{
			"s0": {"theString", "c"},
		},
	},
	{
		name:        "removestream-chain",
		ordinal:     0,
		runtimeID:   "java-runtime-4971ea67797956c303ee",
		execution:   "InfraNamedWindowRemoveStream",
		description: "insert rstream chain over length(2) windows: W1 evictions insert into W2, W2 evictions insert into W3; any-order iterator snapshots pin the cascade after each send wave",
		deploys:     []string{"c1", "c2", "c3", "insert", "route-w2", "route-w3"},
		epl: map[string]string{
			"c1":       infraNWUP565E4CreateW1,
			"c2":       infraNWUP565E4CreateW2,
			"c3":       infraNWUP565E4CreateW3,
			"insert":   infraNWUP565E4Insert,
			"route-w2": infraNWUP565E4RouteW2,
			"route-w3": infraNWUP565E4RouteW3,
		},
		listened: []string{},
		newOnly:  map[string]bool{},
		rowFields: map[string][]string{
			"c1": {"theString"},
			"c2": {"theString"},
			"c3": {"theString"},
		},
	},
}

func infraNWUP565CaseSpecFor(name string) (infraNWUP565CaseSpec, bool) {
	for _, spec := range infraNWUP565CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWUP565CaseSpec{}, false
}

// infraNWUP565Bean mirrors the SupportBean columns the executions read:
// theString and intPrimitive (sendEventBean new SupportBean(text, int)).
type infraNWUP565Bean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int64  `esper:"intPrimitive"`
}

// infraNWUP565StepPin pins one scenario step's shape: the op plus the fields
// that step kind carries (deploy statement/epl, send eventType/payload,
// snapshot statement/mode, advance-time at).
type infraNWUP565StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
	at        string
	mode      string
}

func infraNWUP565DeployPin(statement, epl string) infraNWUP565StepPin {
	return infraNWUP565StepPin{op: "deploy", statement: statement, epl: epl}
}

func infraNWUP565SendPin(eventType string, payload map[string]any) infraNWUP565StepPin {
	return infraNWUP565StepPin{op: "send", eventType: eventType, payload: payload}
}

func infraNWUP565SendMapPin(p0, p1 string) infraNWUP565StepPin {
	return infraNWUP565SendPin("MyMapTypeNW", map[string]any{"p0": p0, "p1": p1})
}

func infraNWUP565SendBeanPin(theString string, intPrimitive int64) infraNWUP565StepPin {
	return infraNWUP565SendPin("SupportBean", map[string]any{"theString": theString, "intPrimitive": float64(intPrimitive)})
}

func infraNWUP565SendS2Pin(company string, value float64) infraNWUP565StepPin {
	return infraNWUP565SendPin("S2", map[string]any{"company": company, "value": value, "total": float64(0)})
}

func infraNWUP565SnapshotPin(statement, mode string) infraNWUP565StepPin {
	return infraNWUP565StepPin{op: "snapshot", statement: statement, mode: mode}
}

func infraNWUP565AdvancePin(at string) infraNWUP565StepPin {
	return infraNWUP565StepPin{op: "advance-time", at: at}
}

// infraNWUP565CaseSteps pins the complete step sequence per case in Java
// source order. update-namedwindow interleaves the three milestone barriers
// as undeploy-all steps followed by the redeploy and replayed-send steps
// the barrier implies (milestone(N) redeploys every statement deployed so
// far and replays every send so far, in order).
var infraNWUP565CaseSteps = map[string][]infraNWUP565StepPin{
	"update-namedwindow": func() []infraNWUP565StepPin {
		e1Deploy := func() []infraNWUP565StepPin {
			return []infraNWUP565StepPin{
				infraNWUP565DeployPin("window", infraNWUP565E1Window),
				infraNWUP565DeployPin("insert", infraNWUP565E1Insert),
				infraNWUP565DeployPin("select", infraNWUP565E1Select),
				infraNWUP565DeployPin("update", infraNWUP565E1Update),
			}
		}
		steps := append([]infraNWUP565StepPin{}, e1Deploy()...)
		// milestone(0): no sends yet, so the barrier redeploys the four
		// statements and replays nothing.
		steps = append(steps, infraNWUP565StepPin{op: "undeploy-all"})
		steps = append(steps, e1Deploy()...)
		steps = append(steps,
			infraNWUP565SendMapPin("E1", "oldvalue"),
			infraNWUP565DeployPin("onselect", infraNWUP565E1OnSelect),
			infraNWUP565SendBeanPin("A", 0),
		)
		// milestone(1): redeploy five statements, replay map send + bean A.
		steps = append(steps, infraNWUP565StepPin{op: "undeploy-all"})
		steps = append(steps, e1Deploy()...)
		steps = append(steps,
			infraNWUP565DeployPin("onselect", infraNWUP565E1OnSelect),
			infraNWUP565SendMapPin("E1", "oldvalue"),
			infraNWUP565SendBeanPin("A", 0),
			infraNWUP565DeployPin("oninsert", infraNWUP565E1OnInsert),
			infraNWUP565SendBeanPin("B", 1),
		)
		// milestone(2): redeploy six statements, replay map send + beans
		// A and B, then the second update-istream and the s0 consumer.
		steps = append(steps, infraNWUP565StepPin{op: "undeploy-all"})
		steps = append(steps, e1Deploy()...)
		steps = append(steps,
			infraNWUP565DeployPin("onselect", infraNWUP565E1OnSelect),
			infraNWUP565DeployPin("oninsert", infraNWUP565E1OnInsert),
			infraNWUP565SendMapPin("E1", "oldvalue"),
			infraNWUP565SendBeanPin("A", 0),
			infraNWUP565SendBeanPin("B", 1),
			infraNWUP565DeployPin("update-other", infraNWUP565E1UpdateOth),
			infraNWUP565DeployPin("s0", infraNWUP565E1S0),
			infraNWUP565SendBeanPin("B", 1),
			infraNWUP565StepPin{op: "undeploy-all"},
		)
		return steps
	}(),
	"onupdate-multidispatch": {
		infraNWUP565DeployPin("schema", infraNWUP565E2Schema),
		infraNWUP565DeployPin("create", infraNWUP565E2Create),
		infraNWUP565DeployPin("insert", infraNWUP565E2Insert),
		infraNWUP565DeployPin("upd", infraNWUP565E2Update),
		infraNWUP565DeployPin("s0", infraNWUP565E2S0),
		infraNWUP565SendS2Pin("AComp", 3),
		infraNWUP565SnapshotPin("s0", "ordered"),
		infraNWUP565SnapshotPin("create", "ordered"),
		infraNWUP565SendS2Pin("AComp", 6),
		infraNWUP565SnapshotPin("s0", "ordered"),
		infraNWUP565SnapshotPin("create", "ordered"),
		infraNWUP565SendS2Pin("AComp", 5),
		infraNWUP565SnapshotPin("s0", "ordered"),
		infraNWUP565SnapshotPin("create", "ordered"),
		infraNWUP565SendS2Pin("BComp", 4),
		infraNWUP565SnapshotPin("s0", "ordered"),
		infraNWUP565SnapshotPin("create", "ordered"),
		infraNWUP565StepPin{op: "undeploy-all"},
	},
	"outputrate-snapshot": {
		infraNWUP565DeployPin("create", infraNWUP565E3Create),
		infraNWUP565DeployPin("insert", infraNWUP565E3Insert),
		infraNWUP565AdvancePin("1970-01-01T00:00:00Z"),
		infraNWUP565DeployPin("s0", infraNWUP565E3S0),
		infraNWUP565SendBeanPin("A", 1),
		infraNWUP565SendBeanPin("A", 2),
		infraNWUP565SendBeanPin("B", 4),
		infraNWUP565AdvancePin("1970-01-01T00:00:01Z"),
		infraNWUP565SendBeanPin("B", 5),
		infraNWUP565AdvancePin("1970-01-01T00:00:02Z"),
		infraNWUP565AdvancePin("1970-01-01T00:00:03Z"),
		infraNWUP565SendBeanPin("A", 5),
		infraNWUP565SendBeanPin("C", 1),
		infraNWUP565AdvancePin("1970-01-01T00:00:04Z"),
		infraNWUP565StepPin{op: "undeploy-all"},
	},
	"removestream-chain": {
		infraNWUP565DeployPin("c1", infraNWUP565E4CreateW1),
		infraNWUP565DeployPin("c2", infraNWUP565E4CreateW2),
		infraNWUP565DeployPin("c3", infraNWUP565E4CreateW3),
		infraNWUP565DeployPin("insert", infraNWUP565E4Insert),
		infraNWUP565DeployPin("route-w2", infraNWUP565E4RouteW2),
		infraNWUP565DeployPin("route-w3", infraNWUP565E4RouteW3),
		infraNWUP565SendBeanPin("E1", 1),
		infraNWUP565SendBeanPin("E2", 1),
		infraNWUP565SnapshotPin("c1", "any"),
		infraNWUP565SendBeanPin("E3", 1),
		infraNWUP565SnapshotPin("c1", "any"),
		infraNWUP565SnapshotPin("c2", "any"),
		infraNWUP565SendBeanPin("E4", 1),
		infraNWUP565SendBeanPin("E5", 1),
		infraNWUP565SnapshotPin("c1", "any"),
		infraNWUP565SnapshotPin("c2", "any"),
		infraNWUP565SnapshotPin("c3", "any"),
		infraNWUP565StepPin{op: "undeploy-all"},
	},
}

func loadInfraNWUP565Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWUP565Id)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWUP565Id, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWUP565Id, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWUP565Id, err)
	}
	if err := requireInfraNWUP565Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWUP565Id, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWUP565Id ||
		metadata.Description != infraNWUP565Description ||
		metadata.JavaCommit != infraNWUP565JavaCommit || metadata.JavaSource != infraNWUP565Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWUP565Id)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, infraNWUP565JavaSources},
		{metadata.JavaRuntimes, infraNWUP565JavaRuntimeIDs},
		{metadata.JavaNames, infraNWUP565JavaExecutions},
		{metadata.JavaStaticIDs, infraNWUP565JavaStaticIDs},
		{metadata.JavaFlags, infraNWUP565JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", infraNWUP565Id)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWUP565CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWUP565Id, len(infraNWUP565CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWUP565Fields(object, "case", "ordinal", "runtimeId", "executionName", "observation", "epl", "deploys"); err != nil {
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
		spec := infraNWUP565CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal || definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution || definition.Observation != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
		// epl pins the newline-joined EPL of the case's deploy steps in step
		// order; deploys pins the label sequence.
		joined := make([]string, 0, len(definition.Deploys))
		for _, label := range definition.Deploys {
			joined = append(joined, spec.epl[label])
		}
		if definition.EPL != strings.Join(joined, "\n") || !reflect.DeepEqual(definition.Deploys, spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d deploy metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWUP565Id)
	}
	if err := validateInfraNWUP565RawSteps(rawSteps); err != nil {
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

// validateInfraNWUP565RawSteps pins the complete step sequence per case
// against the raw JSON objects: op field whitelists are enforced per step
// kind and every step is compared positionally to the pinned sequence.
func validateInfraNWUP565RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(infraNWUP565CaseSpecs))
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
			Mode      string          `json:"mode"`
		}
		if operation == "case" {
			if err := requireInfraNWUP565Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWUP565CaseSpecFor(marker.Case); !ok {
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
			return fmt.Errorf("scenario step %d declares case %q inside the %q block", index, step.Case, currentCase)
		}
		pins := infraNWUP565CaseSteps[currentCase]
		position := positions[currentCase]
		if position >= len(pins) {
			return fmt.Errorf("scenario step %d exceeds the pinned %s step sequence", index, currentCase)
		}
		pin := pins[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q for case %q position %d", index, operation, pin.op, currentCase, position)
		}
		switch operation {
		case "deploy":
			if err := requireInfraNWUP565Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requireInfraNWUP565Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "snapshot":
			if err := requireInfraNWUP565Fields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.Mode != pin.mode {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q", index, currentCase)
			}
		case "advance-time":
			if err := requireInfraNWUP565Fields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.At != pin.at {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requireInfraNWUP565Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(infraNWUP565CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases", caseOrder, len(infraNWUP565CaseSpecs))
	}
	for index, spec := range infraNWUP565CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(infraNWUP565CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d", spec.name, positions[spec.name], len(infraNWUP565CaseSteps[spec.name]))
		}
	}
	return nil
}

func requireInfraNWUP565Fields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

// infraNWUP565CaseState carries the per-case replay state: environment and
// engine (rebuilt at each redeploy barrier so undeploy-all drops the named
// windows like Esper's deployment teardown), deployed statements and
// per-statement listener sequences that persist across barriers.
type infraNWUP565CaseState struct {
	spec       infraNWUP565CaseSpec
	env        *esper.Environment
	engine     *esper.Engine
	now        time.Time
	statements map[string]*esper.Statement
	sequence   map[string]uint64
	records    []compat.TraceRecord
}

// infraNWUP565StartEnvironment registers the case's event types and named
// windows; redeploy barriers call it again so the rebuilt engine sees an
// empty window just like Esper's undeploy destroys window contents.
func infraNWUP565StartEnvironment(spec infraNWUP565CaseSpec) (*esper.Environment, error) {
	env := esper.NewEnvironment()
	stringType := reflect.TypeOf("")
	intType := reflect.TypeOf(int64(0))
	doubleType := reflect.TypeOf(float64(0))
	switch spec.name {
	case "update-namedwindow":
		schema, err := esper.RegisterMap(env, "MyMapTypeNW", []esper.FieldSpec{
			esper.FieldDef("p0", stringType),
			esper.FieldDef("p1", stringType),
		})
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "AWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
		// Java creates MyOtherStream implicitly through the on-insert's
		// insert-into; Go registers the map schema explicitly, matching the
		// established explicit-schema convention.
		if _, err := esper.RegisterMap(env, "MyOtherStream", []esper.FieldSpec{
			esper.FieldDef("p0", stringType),
			esper.FieldDef("p1", stringType),
		}); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWUP565Bean](env, "SupportBean"); err != nil {
			return nil, err
		}
	case "onupdate-multidispatch":
		schema, err := esper.RegisterMap(env, "S2", []esper.FieldSpec{
			esper.FieldDef("company", stringType),
			esper.FieldDef("value", doubleType),
			esper.FieldDef("total", doubleType),
		})
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "S2Win", schema,
			esper.NamedWindowRetention(esper.IntersectWindows(
				esper.TimeWindow(25*time.Hour),
				esper.FirstUnique(esper.Field[any, string]("company")),
			))); err != nil {
			return nil, err
		}
	case "outputrate-snapshot":
		if _, err := esper.RegisterStruct[infraNWUP565Bean](env, "SupportBean"); err != nil {
			return nil, err
		}
		schema, err := esper.NewMapSchema("MyWindowOneRow", []esper.FieldSpec{
			esper.FieldDef("theString", stringType),
			esper.FieldDef("intv", intType),
		})
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowOne", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
	case "removestream-chain":
		schema, err := esper.RegisterStruct[infraNWUP565Bean](env, "SupportBean")
		if err != nil {
			return nil, err
		}
		for _, name := range []string{"W1", "W2", "W3"} {
			if _, err := esper.CreateNamedWindow(env, name, schema, esper.NamedWindowRetention(esper.LengthWindow(2))); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unknown case %q", spec.name)
	}
	return env, nil
}

// runInfraNWUP565Scenario replays all four executions, one fresh runtime
// per case like the Java oracle.
func runInfraNWUP565Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWUP565Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWUP565CaseSpecs {
		records, err := runInfraNWUP565Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWUP565Id, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validateInfraNWUP565Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWUP565Id {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, infraNWUP565Id)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateInfraNWUP565RawSteps(rawSteps)
}

// runInfraNWUP565Case replays one Java execution on a fresh environment and
// engine. The oracle runs with the internal timer disabled at the epoch, so
// record times track the pinned advance-time steps.
func runInfraNWUP565Case(ctx context.Context, scenario compat.Scenario, spec infraNWUP565CaseSpec) ([]compat.TraceRecord, error) {
	env, err := infraNWUP565StartEnvironment(spec)
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	state := &infraNWUP565CaseState{
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
			if err := state.deploy(ctx, step.Statement); err != nil {
				return nil, err
			}
		case "send":
			payload, err := infraNWUP565DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return nil, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("advance-time %q: %w", step.At, err)
			}
			state.now = at.UTC()
			if err := state.engine.AdvanceTime(ctx, state.now); err != nil {
				return nil, err
			}
		case "undeploy-all":
			// Milestone redeploy barrier: Esper's undeploy destroys every
			// deployment including named-window contents, so the runner
			// rebuilds a fresh environment and engine at the current virtual
			// time; the following deploy/send steps redeploy and replay.
			if err := state.rebuild(ctx); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	return state.records, nil
}

// rebuild replaces the case's environment and engine after undeploy-all so
// the next deploy steps observe empty named windows.
func (s *infraNWUP565CaseState) rebuild(ctx context.Context) error {
	if err := s.engine.Close(ctx); err != nil {
		return err
	}
	env, err := infraNWUP565StartEnvironment(s.spec)
	if err != nil {
		return err
	}
	s.env = env
	s.engine = esper.NewEngine(env, esper.WithRuntimeURI(s.spec.runtimeID), esper.WithStartTime(s.now))
	s.statements = make(map[string]*esper.Statement)
	return nil
}

// deploy maps one scenario label onto the typed Go chain equivalent of the
// Java compileDeploy statement and subscribes the labels the Java execution
// listens on. For onupdate-multidispatch the Java execution's on-update
// trigger is preemptive (it observes the window before the continuous
// insert of the same event), so the runner deploys upd ahead of insert when
// the insert step arrives and treats the following upd step as already
// deployed; the deploy statement order inside the step list stays the Java
// order.
func (s *infraNWUP565CaseState) deploy(ctx context.Context, label string) error {
	// schema is a config-level declaration in both languages: the typed Go
	// surface registers the map schema at environment build time.
	if label == "schema" {
		return nil
	}
	order := []string{label}
	if s.spec.name == "onupdate-multidispatch" && label == "insert" {
		order = []string{"upd", "insert"}
	}
	if len(order) == 1 {
		if _, exists := s.statements[label]; exists {
			// For onupdate-multidispatch the upd trigger deploys ahead of
			// insert (preemptive trigger ordering); its own deploy step is
			// then a no-op.
			if s.spec.name == "onupdate-multidispatch" && label == "upd" {
				return nil
			}
			return fmt.Errorf("%s: label %q is already deployed", s.spec.name, label)
		}
	}
	for _, name := range order {
		if _, exists := s.statements[name]; exists {
			continue
		}
		plan, err := s.buildPlan(name)
		if err != nil {
			return fmt.Errorf("%s: build %q: %w", s.spec.name, name, err)
		}
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: deploy %q: %w", s.spec.name, name, err)
		}
		statements := deployment.Statements()
		if len(statements) != 1 {
			return fmt.Errorf("%s: deploy %q produced %d statements", s.spec.name, name, len(statements))
		}
		statement := statements[0]
		s.statements[name] = statement
		if infraNWUP565Listened(s.spec, name) {
			label := name
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				s.recordListener(label, batch)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func infraNWUP565Listened(spec infraNWUP565CaseSpec, name string) bool {
	for _, label := range spec.listened {
		if label == name {
			return true
		}
	}
	return false
}

// recordListener appends the normalized new/old rows of one listener
// delivery under the scenario label. Rows are projected to the pinned field
// set; labels whose Java assertions only pin the new stream drop the old
// rows (the grouped-irstream snapshot's old pairing is undefined).
func (s *infraNWUP565CaseState) recordListener(label string, batch esper.ResultBatch) {
	newRows := projectRecords(compat.NormalizeResults(batch.New), s.spec.rowFields[label])
	oldRows := projectRecords(compat.NormalizeResults(batch.Old), s.spec.rowFields[label])
	if len(newRows) == 0 && len(oldRows) == 0 {
		// Java's listeners skip force-dispatched empty pairs; keep the same
		// convention so redeploy barriers do not invent records.
		return
	}
	s.sequence[label]++
	record := compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "listener",
		Statement: label,
		Sequence:  s.sequence[label],
		Time:      compat.FormatTraceTime(s.now),
		New:       newRows,
	}
	if !s.spec.newOnly[label] {
		record.Old = oldRows
	}
	s.records = append(s.records, record)
}

// snapshot emits one iterator snapshot record for the named statement,
// projecting the pinned fields and sorting canonically when the Java
// assertion is any-order.
func (s *infraNWUP565CaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed", s.spec.name, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	rows := projectRecords(compat.NormalizeResults(result.Batch.New), s.spec.rowFields[step.Statement])
	if step.Mode == "any" {
		sortRowsCanonical(rows)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.spec.name,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.now),
		New:       rows,
	})
	return nil
}

// buildPlan returns the typed Go plan for one deploy label.
func (s *infraNWUP565CaseState) buildPlan(label string) (esper.Plan, error) {
	env := s.env
	switch s.spec.name {
	case "update-namedwindow":
		switch label {
		case "window":
			return env.Build(esper.FromNamedWindow(env, "AWindow").CreateNamedWindowQuery(esper.StatementName("window")))
		case "insert":
			return env.Build(esper.OnRecord(esper.FromAny(env, "MyMapTypeNW")).InsertIntoNamedWindow("AWindow",
				esper.SetColumn("p0", esper.Field[any, any]("p0")),
				esper.SetColumn("p1", esper.Field[any, any]("p1")),
			).Query(esper.StatementName("insert")))
		case "select":
			return env.Build(esper.FromNamedWindow(env, "AWindow").Query(
				esper.StatementName("select"), esper.WithOldStream()))
		case "update":
			return env.Build(esper.FromNamedWindow(env, "AWindow").UpdateStream(
				esper.SetColumn("p1", esper.Literal("newvalue")),
			).Query(esper.StatementName("update")))
		case "onselect":
			return env.Build(esper.OnEvent(esper.From[infraNWUP565Bean](env, "SupportBean").
				Filter(esper.Equal[string](esper.Field[infraNWUP565Bean, string]("theString"), esper.Literal("A")))).
				SelectFromNamedWindow("AWindow", esper.Literal(true),
					esper.Alias("p0", esper.NamedWindowField[string]("p0")),
					esper.Alias("p1", esper.NamedWindowField[string]("p1")),
				).Query(esper.StatementName("onselect")))
		case "oninsert":
			return env.Build(esper.OnEvent(esper.From[infraNWUP565Bean](env, "SupportBean").
				Filter(esper.Equal[string](esper.Field[infraNWUP565Bean, string]("theString"), esper.Literal("B")))).
				SelectFromNamedWindow("AWindow", esper.Literal(true),
					esper.Alias("p0", esper.NamedWindowField[string]("p0")),
					esper.Alias("p1", esper.NamedWindowField[string]("p1")),
				).Query(esper.RouteTo("MyOtherStream"), esper.StatementName("oninsert")))
		case "update-other":
			return env.Build(esper.FromAny(env, "MyOtherStream").UpdateStream(
				esper.SetColumn("p0", esper.Literal("a")),
				esper.SetColumn("p1", esper.Literal("b")),
			).Query(esper.StatementName("update-other")))
		case "s0":
			return env.Build(esper.FromAny(env, "MyOtherStream").Query(esper.StatementName("s0")))
		}
	case "onupdate-multidispatch":
		switch label {
		case "create":
			return env.Build(esper.FromNamedWindow(env, "S2Win").CreateNamedWindowQuery(esper.StatementName("create")))
		case "insert":
			return env.Build(esper.OnRecord(esper.FromAny(env, "S2")).InsertIntoNamedWindow("S2Win",
				esper.CopyMatchingFields(),
			).Query(esper.StatementName("insert")))
		case "upd":
			return env.Build(esper.OnRecord(esper.FromAny(env, "S2")).UpdateNamedWindow("S2Win",
				esper.Literal(true),
				esper.SetColumn("total", esper.AddOf[float64](
					esper.NamedWindowField[float64]("value"),
					esper.Field[any, float64]("value"))),
			).Query(esper.StatementName("upd")))
		case "s0":
			return env.Build(esper.FromNamedWindow(env, "S2Win").Aggregate(
				esper.Alias("cnt", esper.CountAll()),
			).Query(esper.StatementName("s0"), esper.WithOldStream()))
		}
	case "outputrate-snapshot":
		switch label {
		case "create":
			return env.Build(esper.FromNamedWindow(env, "MyWindowOne").CreateNamedWindowQuery(esper.StatementName("create")))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWUP565Bean](env, "SupportBean")).InsertIntoNamedWindow("MyWindowOne",
				esper.SetColumn("theString", esper.Field[infraNWUP565Bean, string]("theString")),
				esper.SetColumn("intv", esper.Field[infraNWUP565Bean, int64]("intPrimitive")),
			).Query(esper.StatementName("insert")))
		case "s0":
			return env.Build(esper.FromNamedWindow(env, "MyWindowOne").
				GroupBy(esper.Field[any, string]("theString")).
				Select(
					esper.Alias("theString", esper.Field[any, string]("theString")),
					esper.Alias("c", esper.CountAll()),
				).Query(esper.StatementName("s0"), esper.WithOldStream(),
				esper.WithOutput(esper.OutputSnapshotEvery(time.Second))))
		}
	case "removestream-chain":
		switch label {
		case "c1", "c2", "c3":
			window := "W" + strings.TrimPrefix(label, "c")
			return env.Build(esper.FromNamedWindow(env, window).CreateNamedWindowQuery(esper.StatementName(label)))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWUP565Bean](env, "SupportBean")).InsertIntoNamedWindow("W1",
				esper.CopyMatchingFields(),
			).Query(esper.StatementName("insert")))
		case "route-w2":
			return env.Build(esper.FromNamedWindow(env, "W1").Query(
				esper.StatementName("route-w2"), esper.RouteTo("W2"),
				esper.WithRemoveStreamOnly(), esper.WithRStreamRoute()))
		case "route-w3":
			return env.Build(esper.FromNamedWindow(env, "W2").Query(
				esper.StatementName("route-w3"), esper.RouteTo("W3"),
				esper.WithRemoveStreamOnly(), esper.WithRStreamRoute()))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", label, s.spec.name)
}

// infraNWUP565DecodePayload converts a scenario send payload into the typed
// host object: map payloads for MyMapTypeNW and S2, the SupportBean struct
// for bean sends.
func infraNWUP565DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "MyMapTypeNW":
		if err := requireInfraNWUP565Fields(fields, "p0", "p1"); err != nil {
			return nil, err
		}
		var payload struct {
			P0 string `json:"p0"`
			P1 string `json:"p1"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode MyMapTypeNW: %w", err)
		}
		return map[string]any{"p0": payload.P0, "p1": payload.P1}, nil
	case "S2":
		if err := requireInfraNWUP565Fields(fields, "company", "value", "total"); err != nil {
			return nil, err
		}
		var payload struct {
			Company string  `json:"company"`
			Value   float64 `json:"value"`
			Total   float64 `json:"total"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode S2: %w", err)
		}
		return map[string]any{"company": payload.Company, "value": payload.Value, "total": payload.Total}, nil
	case "SupportBean":
		if err := requireInfraNWUP565Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean infraNWUP565Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWUP565Id, step.EventType)
	}
}
