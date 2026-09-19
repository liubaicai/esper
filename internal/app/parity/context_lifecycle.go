package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// context_lifecycle.go replays the five ContextLifecycle executions (ords
// 0-4) against the pinned Java oracle:
//
//   - split-stream (ord 0, ContextLifecycleSplitStream): two sub-cases on one
//     runtime. Part A deploys the CtxSegmentedByTarget keyed context plus an
//     on-trigger that routes SupportBean(intPrimitive=100) into the
//     NewSupportBean insert-into stream; s0 (select * from NewSupportBean)
//     stays silent for E1/1 and emits one row for E1/100. Part B adds the
//     context-bound NewEvent#unique(theString) named window and an output-all
//     on-trigger whose second branch projects a max(intPrimitive) subquery
//     over NewEvent into NewEventTwo; the subquery observes the pre-trigger
//     window state, so E1/1 and E1/100 emit {mymax:null} and E1/0 emits
//     {mymax:100}.
//   - vdw-unrepresentable (ord 1, ContextLifecycleVirtualDataWindow): the
//     Java SPI contract (one virtual data window per context partition,
//     destroyed on undeploy) has no Go boundary — the Go plugin window
//     provider is never instantiated per partition — so the case emits one
//     pinned "unrepresentable" record after the oracle replays the deploy,
//     two sends and undeploy-all.
//   - nw-other-context-onexpr (ord 2, ContextLifecycleNWOtherContextOnExpr):
//     the NineToFive/TenToFive cron contexts plus the NineToFive-bound
//     MyWindow#keepall named window pin three on-trigger merge rejections —
//     no-context trigger, different-context trigger and (after undeploying
//     createwindow and deploying contextless MyWindowTwo) context trigger
//     against a contextless window. Each probe verifies the corresponding
//     validateNamedWindowTriggerContext rejection before recording the
//     pinned Java prefix.
//   - invalid (ord 3, ContextLifecycleInvalid, INVALIDITY): six probes pin
//     the duplicate-context registration boundary, the provider-undeploy
//     precondition while a statement references the context, the unknown
//     context name, update-istream under a context, the unrepresentable
//     statement-form create-context-in-context, and the context-field
//     reference without a statement context.
//   - simple (ord 4, ContextLifecycleSimple, STATICHOOK): a deploy/undeploy
//     sequence asserting context-count (len(env.Contexts())) and
//     schedule-count-overall (Engine.ScheduleCountOverall) — one schedule
//     per temporal context with refs>0, shared across statements.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context`/`create window`/`insert into` declarations map to
//     env-level registrations (CreateKeyContextByStreams,
//     CreateCronTimeContext, CreateTimePeriodContext, CreateNamedWindow,
//     RegisterStruct/RegisterMap); Go has no module path, so
//     compileWithoutPath only steers the oracle's compiler args.
//   - Java's undeployModuleContaining("createwindow") has no Go named-window
//     removal counterpart; the undeploy step is a no-op because no later
//     probe references MyWindow.
//   - The ord-3 create-context deploy maps to a module registration plus a
//     provider deployment so the duplicate registration and the
//     referenced-by undeploy precondition keep their Go boundaries.
//   - Ord 1's virtual data window has no Go plugin-window binding, so its
//     deploy/send steps are no-ops and the pinned record documents the Java
//     SPI contract.

const (
	contextLifecycleID         = "context-lifecycle"
	contextLifecycleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextLifecycleSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextLifecycle.java"
)

const contextLifecycleDescription = "ContextLifecycle lifecycle surface (ords 0-4): split-stream replays a keyed-context on-trigger routing intPrimitive=100 into an insert-into stream (part A) and an output-all split whose second branch projects a pre-trigger max(intPrimitive) subquery over a context-bound named window into NewEventTwo (part B, mymax null,null,100); vdw-unrepresentable pins the Java virtual-data-window SPI contract (one window per context partition, destroyed on undeploy) with no Go boundary; nw-other-context-onexpr pins three on-trigger merge rejections across the NineToFive/TenToFive cron contexts and the MyWindow/MyWindowTwo named windows; invalid pins six rejection probes (duplicate context, referenced-context undeploy precondition, unknown context, update-istream under a context, unrepresentable statement-form create-context, context field without a statement context); simple pins the context-count/schedule-count-overall deploy-undeploy sequence with one shared schedule per referenced temporal context. compile-error/undeploy-error records carry the pinned Java message prefixes; unrepresentable carries the pinned SPI note; context-count and schedule-count-overall carry observed counts (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextLifecycle.java)."

var (
	contextLifecycleJavaRuntimeIDs = []string{
		"java-runtime-d59fd16257279a2118a7",
		"java-runtime-38ddfd09bd97e862e438",
		"java-runtime-a80d326a65c440aa438f",
		"java-runtime-f04b99d603d61f3808fb",
		"java-runtime-ba7774dedca1bd391c8c",
	}
	contextLifecycleJavaExecutions = []string{
		"ContextLifecycleSplitStream",
		"ContextLifecycleVirtualDataWindow",
		"ContextLifecycleNWOtherContextOnExpr",
		"ContextLifecycleInvalid",
		"ContextLifecycleSimple",
	}
	contextLifecycleJavaStaticIDs = []string{
		"java-1d9805ba018e9b1bed1d",
		"java-724063e9b5a1a2b3d83a",
		"java-15148b122e0ca21c3681",
		"java-1a96001402ca3a4c074e",
		"java-407112a1151ec755e45b",
	}
	contextLifecycleJavaFlags = []string{"INVALIDITY", "STATICHOOK"}
	contextLifecycleCases     = []string{
		"split-stream",
		"vdw-unrepresentable",
		"nw-other-context-onexpr",
		"invalid",
		"simple",
	}
	contextLifecycleOrdinals = []int{0, 1, 2, 3, 4}
	contextLifecycleSources  = []string{contextLifecycleSource}
)

var contextLifecycleCaseObservations = []string{
	"listener; part A routes SupportBean(intPrimitive=100) into NewSupportBean under the keyed context so s0 emits one row for E1/100 and stays silent for E1/1; part B's output-all split projects the pre-trigger max(intPrimitive) over NewEvent into NewEventTwo so s0 emits mymax null, null, then 100 for E1/1, E1/100, E1/0",
	"unrepresentable; the Java virtual-data-window SPI instantiates one window per context partition (two after E1/E2) and destroys every window plus the factory on undeploy-all; no Go plugin-window boundary exists, so the pinned record documents the contract",
	"compile-error; three on-trigger merge probes pin the named-window context rules: a contextless trigger against NineToFive-bound MyWindow, a TenToFive trigger against MyWindow, and a TenToFive trigger against contextless MyWindowTwo after createwindow is undeployed",
	"compile-error+undeploy-error; six probes pin the duplicate NineToFive registration, the provider undeploy precondition while s0 references the context, the unknown EightToSix context, update-istream under SegmentedByAString, the unrepresentable statement-form create-context under ABC, and the context.sb.theString reference without a statement context",
	"context-count+schedule-count-overall; deploy/undeploy steps pin len(env.Contexts()) and Engine.ScheduleCountOverall: the cron context counts once registered, one shared schedule exists while any statement references it, and undeploy-all returns both counters to zero",
}

var contextLifecycleCaseEPLs = []string{
	"@Name('out') @public context CtxSegmentedByTarget on SupportBean insert into NewEvent select * where intPrimitive = 100 insert into NewEventTwo select (select max(intPrimitive) from NewEvent) as mymax  output all",
	"@public context CtxSegmented create window TestVDWWindow.test:vdw() as SupportBean",
	"context TenToFive on SupportBean_S0 s0 merge MyWindow mw when matched then update set intPrimitive = 1",
	"context EightToSix select * from SupportBean",
	"@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)",
}

// The byte-exact EPLs the deploy steps pin, keyed by case then label. The
// split-stream "out" and "s0" labels each carry two EPLs (parts A and B).
var contextLifecycleDeployEPLs = map[string]map[string][]string{
	"split-stream": {
		"ctx": {
			"@public create context CtxSegmentedByTarget partition by theString from SupportBean",
		},
		"out": {
			"@Name('out') @public context CtxSegmentedByTarget on SupportBean insert into NewSupportBean select * where intPrimitive = 100",
			"@Name('out') @public context CtxSegmentedByTarget on SupportBean insert into NewEvent select * where intPrimitive = 100 insert into NewEventTwo select (select max(intPrimitive) from NewEvent) as mymax  output all",
		},
		"window": {
			"@public context CtxSegmentedByTarget create window NewEvent#unique(theString) as SupportBean",
		},
		"s0": {
			"@name('s0') select * from NewSupportBean",
			"@name('s0') select * from NewEventTwo",
		},
	},
	"vdw-unrepresentable": {
		"ctx":    {"@public create context CtxSegmented as partition by theString from SupportBean"},
		"window": {"@public context CtxSegmented create window TestVDWWindow.test:vdw() as SupportBean"},
		"s0":     {"select * from TestVDWWindow"},
	},
	"nw-other-context-onexpr": {
		"ctx-nine":     {"@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)"},
		"ctx-ten":      {"@public create context TenToFive as start (0, 10, *, *, *) end (0, 17, *, *, *)"},
		"createwindow": {"@name('createwindow') @public context NineToFive create window MyWindow#keepall as SupportBean"},
		"window-two":   {"@public create window MyWindowTwo#keepall as SupportBean"},
	},
	"invalid": {
		"ctx":           {"@name('ctx') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)"},
		"s0":            {"context NineToFive select * from SupportBean"},
		"abc-stream":    {"@public insert into ABCStream select * from SupportBean"},
		"ctx-segmented": {"@Name('context') @public create context SegmentedByAString partition by theString from ABCStream"},
		"ctx-abc":       {"@public create context ABC start @now end after 5 seconds"},
	},
	"simple": {
		"ctx": {"@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)"},
		"s0":  {"@Name('s0') context NineToFive select * from SupportBean"},
		"c":   {"@Name('C') context NineToFive select * from SupportBean"},
		"d":   {"@Name('D') context NineToFive select * from SupportBean"},
	},
}

// The byte-exact EPLs and pinned Java message prefixes the build-error and
// undeploy-error probes pin.
var contextLifecycleProbeEPLs = map[string]string{
	"trigger-no-context":         "on SupportBean_S0 s0 merge MyWindow mw when matched then update set intPrimitive = 1",
	"trigger-other-context":      "context TenToFive on SupportBean_S0 s0 merge MyWindow mw when matched then update set intPrimitive = 1",
	"trigger-contextless-window": "context TenToFive on SupportBean_S0 s0 merge MyWindowTwo mw when matched then update set intPrimitive = 1",
	"duplicate-context":          "@name('ctx') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)",
	"context-not-found":          "context EightToSix select * from SupportBean",
	"update-istream-context":     "context SegmentedByAString update istream ABCStream set intPrimitive = (select id from SupportBean_S0#lastevent) where intPrimitive < 0",
	"create-context-in-context":  "context ABC create context DEF start @now end after 5 seconds",
	"context-field-no-context":   "select context.sb.theString from SupportBean as sb",
}

var contextLifecycleProbeErrors = map[string]string{
	"trigger-no-context":         "Cannot create on-trigger expression: Named window 'MyWindow' was declared with context 'NineToFive', please declare the same context name",
	"trigger-other-context":      "Cannot create on-trigger expression: Named window 'MyWindow' was declared with context 'NineToFive', please use the same context instead",
	"trigger-contextless-window": "Cannot create on-trigger expression: Named window 'MyWindowTwo' was declared without a context",
	"duplicate-context":          "Context by name 'NineToFive' already exists",
	"context-in-use":             "A precondition is not satisfied: Context 'NineToFive' cannot be un-deployed as it is referenced by deployment",
	"context-not-found":          "Context by name 'EightToSix' could not be found",
	"update-istream-context":     "Update IStream is not supported in conjunction with a context",
	"create-context-in-context":  "A create-context statement cannot itself be associated to a context, please declare a nested context instead",
	"context-field-no-context":   "Failed to validate select-clause expression 'context.sb.theString': Failed to resolve property 'context.sb.theString' to a stream or nested property in a stream",
}

// contextLifecycleVDWNote pins the ord-1 Java SPI contract the
// unrepresentable record documents.
const contextLifecycleVDWNote = "virtual data window SPI: one VirtualDataWindow instance per context partition (two partitions observed for E1/E2), every window and the factory destroyed on undeploy; no Go plugin-window boundary"

// contextLifecycleCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count
// keys. Deploy steps carry the byte-exact EPL text the Java oracle compiles;
// compileWithoutPath marks the ord-4 first deploy whose Java execution calls
// the path-less compileDeploy overload. build-error/undeploy-error steps pin
// the Java message prefix in expectError; the unrepresentable step pins the
// SPI note in expectError; context-count/schedule-count-overall steps pin
// the asserted count.
var contextLifecycleCaseSteps = map[string][]string{
	"split-stream": {
		"deploy|ctx|||@public create context CtxSegmentedByTarget partition by theString from SupportBean||||||",
		"deploy|out|||@Name('out') @public context CtxSegmentedByTarget on SupportBean insert into NewSupportBean select * where intPrimitive = 100||||||",
		"deploy|s0|||@name('s0') select * from NewSupportBean||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":100}|||||",
		"undeploy-all||||||||||",
		"deploy|ctx|||@public create context CtxSegmentedByTarget partition by theString from SupportBean||||||",
		"deploy|window|||@public context CtxSegmentedByTarget create window NewEvent#unique(theString) as SupportBean||||||",
		"deploy|out|||@Name('out') @public context CtxSegmentedByTarget on SupportBean insert into NewEvent select * where intPrimitive = 100 insert into NewEventTwo select (select max(intPrimitive) from NewEvent) as mymax  output all||||||",
		"deploy|s0|||@name('s0') select * from NewEventTwo||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":100}|||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":0}|||||",
		"undeploy-all||||||||||",
	},
	"vdw-unrepresentable": {
		"deploy|ctx|||@public create context CtxSegmented as partition by theString from SupportBean||||||",
		"deploy|window|||@public context CtxSegmented create window TestVDWWindow.test:vdw() as SupportBean||||||",
		"deploy|s0|||select * from TestVDWWindow||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":2}|||||",
		"undeploy-all||||||||||",
		"unrepresentable|virtual-data-window|||||" + contextLifecycleVDWNote + "||||",
	},
	"nw-other-context-onexpr": {
		"deploy|ctx-nine|||@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)||||||",
		"deploy|ctx-ten|||@public create context TenToFive as start (0, 10, *, *, *) end (0, 17, *, *, *)||||||",
		"deploy|createwindow|||@name('createwindow') @public context NineToFive create window MyWindow#keepall as SupportBean||||||",
		"build-error|trigger-no-context|||on SupportBean_S0 s0 merge MyWindow mw when matched then update set intPrimitive = 1||Cannot create on-trigger expression: Named window 'MyWindow' was declared with context 'NineToFive', please declare the same context name||||",
		"build-error|trigger-other-context|||context TenToFive on SupportBean_S0 s0 merge MyWindow mw when matched then update set intPrimitive = 1||Cannot create on-trigger expression: Named window 'MyWindow' was declared with context 'NineToFive', please use the same context instead||||",
		"undeploy|createwindow|||||||||",
		"deploy|window-two|||@public create window MyWindowTwo#keepall as SupportBean||||||",
		"build-error|trigger-contextless-window|||context TenToFive on SupportBean_S0 s0 merge MyWindowTwo mw when matched then update set intPrimitive = 1||Cannot create on-trigger expression: Named window 'MyWindowTwo' was declared without a context||||",
		"undeploy-all||||||||||",
	},
	"invalid": {
		"deploy|ctx|||@name('ctx') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)||||||",
		"build-error|duplicate-context|||@name('ctx') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)||Context by name 'NineToFive' already exists||||",
		"deploy|s0|||context NineToFive select * from SupportBean||||||",
		"undeploy-error|ctx|||||A precondition is not satisfied: Context 'NineToFive' cannot be un-deployed as it is referenced by deployment||||",
		"build-error|context-not-found|||context EightToSix select * from SupportBean||Context by name 'EightToSix' could not be found||||",
		"deploy|abc-stream|||@public insert into ABCStream select * from SupportBean||||||",
		"deploy|ctx-segmented|||@Name('context') @public create context SegmentedByAString partition by theString from ABCStream||||||",
		"build-error|update-istream-context|||context SegmentedByAString update istream ABCStream set intPrimitive = (select id from SupportBean_S0#lastevent) where intPrimitive < 0||Update IStream is not supported in conjunction with a context||||",
		"deploy|ctx-abc|||@public create context ABC start @now end after 5 seconds||||||",
		"build-error|create-context-in-context|||context ABC create context DEF start @now end after 5 seconds||A create-context statement cannot itself be associated to a context, please declare a nested context instead||||",
		"build-error|context-field-no-context|||select context.sb.theString from SupportBean as sb||Failed to validate select-clause expression 'context.sb.theString': Failed to resolve property 'context.sb.theString' to a stream or nested property in a stream|1|||",
		"undeploy-all||||||||||",
	},
	"simple": {
		"context-count||||||||||0",
		"schedule-count-overall||||||||||0",
		"deploy|ctx|||@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)|||1|||",
		"context-count||||||||||1",
		"schedule-count-overall||||||||||0",
		"undeploy|ctx|||||||||",
		"context-count||||||||||0",
		"deploy|ctx|||@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)||||||",
		"context-count||||||||||1",
		"deploy|s0|||@Name('s0') context NineToFive select * from SupportBean||||||",
		"schedule-count-overall||||||||||1",
		"undeploy|s0|||||||||",
		"schedule-count-overall||||||||||0",
		"undeploy|ctx|||||||||",
		"context-count||||||||||0",
		"deploy|ctx|||@Name('context') @public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)||||||",
		"deploy|c|||@Name('C') context NineToFive select * from SupportBean||||||",
		"deploy|d|||@Name('D') context NineToFive select * from SupportBean||||||",
		"schedule-count-overall||||||||||1",
		"undeploy-all||||||||||",
		"context-count||||||||||0",
		"schedule-count-overall||||||||||0",
		"undeploy-all||||||||||",
	},
}

// contextLifecycleBean mirrors the real SupportBean surface: the Java
// execution's `select * from NewSupportBean` projects the full 20-property
// bean, so the Go trace renders the same row. Boxed, decimal, and enum
// columns are pointers so a plain send decodes to Java's nulls, and
// charPrimitive mirrors the Java char default via the decode.
type contextLifecycleBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// contextLifecycleS0 mirrors SupportBean_S0's asserted fields (the merge
// trigger type in the nw-other-context-onexpr and invalid cases).
type contextLifecycleS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// contextLifecycleCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, the deploy label
// that owns each registered context, the registered context names in
// registration order, the invalid case's provider module, and the per-s0
// listener sequence.
type contextLifecycleCaseState struct {
	caseName       string
	env            *esper.Environment
	engine         *esper.Engine
	trace          *compat.Trace
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	contextByLabel map[string]string
	registeredCtx  []string
	ctxModule      esper.Module
	listenerSeq    uint64
}

func runContextLifecycleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextLifecycleScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextLifecycle(ctx, scenario, &trace)
}

// executeContextLifecycle replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeContextLifecycle(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextLifecycleCaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startContextLifecycleCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "send":
			event, err := decodeContextLifecyclePayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "undeploy-error":
			if err := state.undeployError(ctx, step); err != nil {
				return *trace, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return *trace, err
			}
		case "undeploy":
			if err := state.undeploy(ctx, step.Statement); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		case "context-count", "schedule-count-overall":
			if err := state.countRecord(ctx, step); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextLifecycleID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startContextLifecycleCase builds the fresh per-case environment: the
// SupportBean/SupportBean_S0 event types plus the case's insert-into and
// named-window schemas, and the engine pinned to the case's Java runtime id
// at the epoch start time.
func startContextLifecycleCase(caseName string, trace *compat.Trace) (*contextLifecycleCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextLifecycleBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextLifecycleS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	switch caseName {
	case "split-stream":
		// Part A's insert-into target carries the SupportBean shape; part
		// B's NewEventTwo carries the nullable mymax projection.
		if _, err := esper.RegisterStruct[contextLifecycleBean](env, "NewSupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterMap(env, "NewEventTwo", []esper.FieldSpec{
			esper.FieldDef("mymax", reflect.TypeOf((*any)(nil)).Elem()),
		}); err != nil {
			return nil, err
		}
	case "invalid":
		// ABCStream is the insert-into target the SegmentedByAString
		// context partitions and the update-istream probe targets.
		if _, err := esper.RegisterMap(env, "ABCStream", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf("")),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int(0))),
		}); err != nil {
			return nil, err
		}
	}
	state := &contextLifecycleCaseState{
		caseName:       caseName,
		env:            env,
		deployments:    map[string]*esper.Deployment{},
		contextByLabel: map[string]string{},
		trace:          trace,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextLifecycleJavaRuntimeIDs[contextLifecycleCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextLifecycleCaseOrdinal(caseName string) int {
	for index, name := range contextLifecycleCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy executes one deploy step: registration fixtures model the
// create-context/create-window EPL at their scenario positions (the
// established approved difference for the missing deployable statement
// types) and statement fixtures deploy labeled plans. The vdw case's window
// and select deploys are no-ops — the plugin window is unrepresentable.
func (s *contextLifecycleCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if pinned, ok := contextLifecycleDeployEPLs[s.caseName][label]; !ok || !contextLifecycleContains(pinned, step.Epl) {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", contextLifecycleID, s.caseName, label, step.Epl)
	}
	beanSource := esper.From[contextLifecycleBean](s.env, "SupportBean")
	theString := esper.Field[contextLifecycleBean, string]("theString")
	intPrimitive := esper.Field[contextLifecycleBean, int]("intPrimitive")
	switch s.caseName {
	case "split-stream":
		switch step.Epl {
		case contextLifecycleDeployEPLs["split-stream"]["ctx"][0]:
			return s.registerKeyContext(label, "CtxSegmentedByTarget", "SupportBean", theString)
		case contextLifecycleDeployEPLs["split-stream"]["out"][0]:
			// on SupportBean insert into NewSupportBean select * where
			// intPrimitive = 100 — a single conditional branch.
			plan, err := s.env.Build(esper.OnEvent(beanSource).SplitAll(
				esper.SplitIntoWhen(esper.Equal[int](intPrimitive, esper.Literal(100)), "NewSupportBean"),
			).Query(esper.StatementName("out"), esper.WithContext("CtxSegmentedByTarget")))
			_, err = s.deployPlan(label, plan, err)
			return err
		case contextLifecycleDeployEPLs["split-stream"]["s0"][0]:
			plan, err := s.env.Build(esper.From[contextLifecycleBean](s.env, "NewSupportBean").Query(esper.StatementName("s0")))
			return s.deployListened(label, plan, err)
		case contextLifecycleDeployEPLs["split-stream"]["window"][0]:
			schema, ok := s.env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("%s: SupportBean schema is not registered", contextLifecycleID)
			}
			_, err := esper.CreateNamedWindow(s.env, "NewEvent", schema,
				esper.NamedWindowContext("CtxSegmentedByTarget"),
				esper.NamedWindowRetention(esper.Unique(theString)))
			return err
		case contextLifecycleDeployEPLs["split-stream"]["out"][1]:
			// on SupportBean insert into NewEvent select * where
			// intPrimitive = 100 insert into NewEventTwo select (select
			// max(intPrimitive) from NewEvent) as mymax output all — the
			// unconditional branch's subquery observes the pre-trigger
			// window state.
			subquery := esper.SubqueryValue[int](
				esper.FromNamedWindow(s.env, "NewEvent"),
				esper.Max[int](esper.Field[any, int]("intPrimitive")))
			plan, err := s.env.Build(esper.OnEvent(beanSource).SplitAll(
				esper.SplitIntoWhen(esper.Equal[int](intPrimitive, esper.Literal(100)), "NewEvent"),
				esper.SplitInto("NewEventTwo", esper.Alias("mymax", subquery)),
			).Query(esper.StatementName("out"), esper.WithContext("CtxSegmentedByTarget")))
			_, err = s.deployPlan(label, plan, err)
			return err
		case contextLifecycleDeployEPLs["split-stream"]["s0"][1]:
			plan, err := s.env.Build(esper.FromAny(s.env, "NewEventTwo").Query(esper.StatementName("s0")))
			return s.deployListened(label, plan, err)
		}
	case "vdw-unrepresentable":
		switch label {
		case "ctx":
			return s.registerKeyContext(label, "CtxSegmented", "SupportBean", theString)
		case "window", "s0":
			// The test:vdw() plugin window and its consumer have no Go
			// boundary; the pinned unrepresentable record carries the SPI
			// contract.
			return nil
		}
	case "nw-other-context-onexpr":
		switch label {
		case "ctx-nine":
			return s.registerCronContext(label, "NineToFive", 9)
		case "ctx-ten":
			return s.registerCronContext(label, "TenToFive", 10)
		case "createwindow":
			schema, ok := s.env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("%s: SupportBean schema is not registered", contextLifecycleID)
			}
			_, err := esper.CreateNamedWindow(s.env, "MyWindow", schema,
				esper.NamedWindowContext("NineToFive"),
				esper.NamedWindowRetention(esper.KeepAll()))
			return err
		case "window-two":
			schema, ok := s.env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("%s: SupportBean schema is not registered", contextLifecycleID)
			}
			_, err := esper.CreateNamedWindow(s.env, "MyWindowTwo", schema,
				esper.NamedWindowRetention(esper.KeepAll()))
			return err
		}
	case "invalid":
		switch label {
		case "ctx":
			// The create-context deploy maps to a module registration plus
			// a provider deployment so the duplicate registration and the
			// referenced-by undeploy precondition keep their boundaries.
			module, err := s.env.RegisterModule("ctx", esper.PublicModule())
			if err != nil {
				return err
			}
			s.ctxModule = module
			definition, err := esper.NewCronTimeContext("NineToFive", contextLifecycleNineToFive(), contextLifecycleFivePM())
			if err != nil {
				return err
			}
			if _, err := module.RegisterContextDefinition("NineToFive", definition); err != nil {
				return err
			}
			plan, err := module.Build(esper.Select(beanSource,
				esper.Alias("theString", theString)).Query(esper.StatementName("ctx")))
			if err != nil {
				return err
			}
			deployment, err := s.engine.Deploy(ctx, plan, esper.WithDeploymentID("ctx"))
			if err != nil {
				return fmt.Errorf("%s: deploy %q: %w", contextLifecycleID, label, err)
			}
			s.deployments[label] = deployment
			s.deployOrder = append(s.deployOrder, label)
			return nil
		case "s0":
			// context NineToFive select * from SupportBean — the consumer
			// that blocks the provider undeploy.
			if s.ctxModule.Name() == "" {
				return fmt.Errorf("%s: invalid case deploy s0 requires the ctx provider module", contextLifecycleID)
			}
			plan, err := s.env.Build(beanSource.Query(
				esper.StatementName("s0"), esper.WithContext(s.ctxModule.Context("NineToFive"))))
			if err != nil {
				return err
			}
			deployment, err := s.engine.Deploy(ctx, plan, esper.WithDeploymentID("s0"))
			if err != nil {
				return fmt.Errorf("%s: deploy %q: %w", contextLifecycleID, label, err)
			}
			s.deployments[label] = deployment
			s.deployOrder = append(s.deployOrder, label)
			return nil
		case "abc-stream":
			plan, err := s.env.Build(beanSource.InsertInto("ABCStream", esper.StatementName("abc-stream")))
			_, err = s.deployPlan(label, plan, err)
			return err
		case "ctx-segmented":
			return s.registerKeyContext(label, "SegmentedByAString", "ABCStream",
				esper.Field[map[string]any, string]("theString"))
		case "ctx-abc":
			// create context ABC start @now end after 5 seconds — a
			// time-period context starting immediately.
			if _, ok := s.env.Context("ABC"); ok {
				s.contextByLabel[label] = "ABC"
				return nil
			}
			if _, err := esper.CreateTimePeriodContext(s.env, "ABC", 0, 5*time.Second); err != nil {
				return err
			}
			s.contextByLabel[label] = "ABC"
			s.registeredCtx = append(s.registeredCtx, "ABC")
			return nil
		}
	case "simple":
		switch label {
		case "ctx":
			return s.registerCronContext(label, "NineToFive", 9)
		case "s0", "c", "d":
			name := map[string]string{"s0": "s0", "c": "C", "d": "D"}[label]
			plan, err := s.env.Build(beanSource.Query(
				esper.StatementName(name), esper.WithContext("NineToFive")))
			_, err = s.deployPlan(label, plan, err)
			return err
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextLifecycleID, s.caseName, label)
}

// registerKeyContext registers a `partition by <key> from <type>` context
// under the deploy label, reusing an already-registered name (the simple
// case redeploys after DestroyContext removed the definition).
func (s *contextLifecycleCaseState) registerKeyContext(label, name, eventType string, key esper.Expr) error {
	if _, ok := s.env.Context(name); ok {
		s.contextByLabel[label] = name
		return nil
	}
	_, err := esper.CreateKeyContextByStreams(s.env, name,
		esper.KeyContextStream{Type: eventType, Keys: []esper.Expr{key}})
	if err != nil {
		return err
	}
	s.contextByLabel[label] = name
	s.registeredCtx = append(s.registeredCtx, name)
	return nil
}

// registerCronContext registers a `start (0, hour, *, *, *) end (0, 17, *,
// *, *)` cron context under the deploy label.
func (s *contextLifecycleCaseState) registerCronContext(label, name string, startHour int) error {
	if _, ok := s.env.Context(name); ok {
		s.contextByLabel[label] = name
		return nil
	}
	_, err := esper.CreateCronTimeContext(s.env, name,
		esper.NewCronSchedule(esper.CronValues(0), esper.CronValues(startHour),
			esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard()),
		contextLifecycleFivePM())
	if err != nil {
		return err
	}
	s.contextByLabel[label] = name
	s.registeredCtx = append(s.registeredCtx, name)
	return nil
}

func contextLifecycleNineToFive() esper.CronSchedule {
	return esper.NewCronSchedule(esper.CronValues(0), esper.CronValues(9),
		esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard())
}

func contextLifecycleFivePM() esper.CronSchedule {
	return esper.NewCronSchedule(esper.CronValues(0), esper.CronValues(17),
		esper.CronWildcard(), esper.CronWildcard(), esper.CronWildcard())
}

// deployPlan deploys one built plan under the step label and records the
// deployment for targeted undeploy.
func (s *contextLifecycleCaseState) deployPlan(label string, plan esper.Plan, planErr error) (*esper.Deployment, error) {
	if planErr != nil {
		return nil, planErr
	}
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return nil, fmt.Errorf("%s: deploy %q: %w", contextLifecycleID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	return deployment, nil
}

// deployListened deploys s0 and subscribes the trace listener, mirroring
// the Java execution's addListener("s0"). The sequence restarts per deploy,
// matching the per-listener sequence in the Java oracle.
func (s *contextLifecycleCaseState) deployListened(label string, plan esper.Plan, planErr error) error {
	deployment, err := s.deployPlan(label, plan, planErr)
	if err != nil {
		return err
	}
	if len(deployment.Statements()) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements", contextLifecycleID, label, len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	s.listenerSeq = 0
	_, err = statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.listenerSeq++
		record := compat.TraceRecord{
			Case:      s.caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  s.listenerSeq,
			Time:      compat.FormatTraceTime(batch.Time),
		}
		record.New = compat.NormalizeResults(batch.New)
		record.Old = compat.NormalizeResults(batch.Old)
		s.trace.Records = append(s.trace.Records, record)
		return nil
	})
	return err
}

// buildError runs one expected-invalid probe against the fluent equivalent
// of the pinned EPL. Each probe verifies Go rejects the nearest expressible
// boundary (or is unrepresentable) before recording the pinned Java prefix.
func (s *contextLifecycleCaseState) buildError(step compat.Step) error {
	if pinned, ok := contextLifecycleProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", contextLifecycleID, step.Statement, step.Epl)
	}
	s0Source := esper.From[contextLifecycleS0](s.env, "SupportBean_S0")
	var buildErr error
	switch step.Statement {
	case "trigger-no-context":
		// on SupportBean_S0 merge MyWindow — the trigger declares no
		// context while MyWindow is bound to NineToFive.
		_, buildErr = s.env.Build(esper.OnEvent(s0Source).MergeIntoNamedWindowWhen("MyWindow", nil,
			esper.WhenMatchedAny(esper.SetColumn("intPrimitive", esper.Literal(1))),
		).Query(esper.StatementName("probe")))
	case "trigger-other-context":
		// context TenToFive on SupportBean_S0 merge MyWindow — the trigger
		// context differs from the window's NineToFive.
		_, buildErr = s.env.Build(esper.OnEvent(s0Source).MergeIntoNamedWindowWhen("MyWindow", nil,
			esper.WhenMatchedAny(esper.SetColumn("intPrimitive", esper.Literal(1))),
		).Query(esper.StatementName("probe"), esper.WithContext("TenToFive")))
	case "trigger-contextless-window":
		// context TenToFive on SupportBean_S0 merge MyWindowTwo — the
		// window was declared without a context.
		_, buildErr = s.env.Build(esper.OnEvent(s0Source).MergeIntoNamedWindowWhen("MyWindowTwo", nil,
			esper.WhenMatchedAny(esper.SetColumn("intPrimitive", esper.Literal(1))),
		).Query(esper.StatementName("probe"), esper.WithContext("TenToFive")))
	case "duplicate-context":
		// The second create-context deploy maps to a duplicate module
		// registration (DuplicateModuleObjectError).
		if s.ctxModule.Name() == "" {
			return fmt.Errorf("%s: build-error probe %q requires the ctx provider module", contextLifecycleID, step.Statement)
		}
		definition, err := esper.NewCronTimeContext("NineToFive", contextLifecycleNineToFive(), contextLifecycleFivePM())
		if err != nil {
			return err
		}
		_, buildErr = s.ctxModule.RegisterContextDefinition("NineToFive", definition)
	case "context-not-found":
		_, buildErr = s.env.Build(esper.From[contextLifecycleBean](s.env, "SupportBean").Query(
			esper.StatementName("probe"), esper.WithContext("EightToSix")))
	case "update-istream-context":
		// context SegmentedByAString update istream ABCStream set
		// intPrimitive = (select id from SupportBean_S0#lastevent) where
		// intPrimitive < 0 — the context check fires before the subquery
		// is evaluated.
		subquery := esper.SubqueryValue[int](
			s0Source.Window(esper.LastEvent()).AsRecord(),
			esper.Field[contextLifecycleS0, int]("id"))
		_, buildErr = s.env.Build(esper.FromAny(s.env, "ABCStream").UpdateStream(
			esper.SetColumn("intPrimitive", subquery),
		).Where(esper.Less[int](esper.Field[any, int]("intPrimitive"), esper.Literal(0))).
			Query(esper.StatementName("probe"), esper.WithContext("SegmentedByAString")))
	case "create-context-in-context":
		// Go has no statement-form create-context, so the Java form is
		// unrepresentable. Pin the prefix without claiming a boundary.
		buildErr = fmt.Errorf("statement-form create-context is unrepresentable")
	case "context-field-no-context":
		// select context.sb.theString from SupportBean as sb — context
		// fields require a statement context.
		_, buildErr = s.env.Build(esper.Select(
			esper.From[contextLifecycleBean](s.env, "SupportBean").As("sb"),
			esper.Alias("c0", esper.ContextPatternField[string]("sb", "theString")),
		).Query(esper.StatementName("probe")))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", contextLifecycleID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", contextLifecycleID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix; the unrepresentable probe
	// skips the gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"trigger-no-context":         {esper.ErrorInvalidRule, "was declared with context"},
		"trigger-other-context":      {esper.ErrorInvalidRule, "was declared with context"},
		"trigger-contextless-window": {esper.ErrorInvalidRule, "was declared without a context"},
		"context-not-found":          {esper.ErrorUnknownName, "EightToSix"},
		"update-istream-context":     {esper.ErrorInvalidRule, "update with a statement context is not yet supported"},
		"context-field-no-context":   {esper.ErrorInvalidRule, "context fields require a statement context"},
	}
	if step.Statement == "duplicate-context" {
		var dup *esper.DuplicateModuleObjectError
		if !errors.As(buildErr, &dup) || dup.Kind != esper.DeploymentResourceContext {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", contextLifecycleID, step.Statement, buildErr)
		}
	} else if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", contextLifecycleID, step.Statement, buildErr)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployError mirrors the Java execution's undeployModuleContaining("ctx")
// precondition probe: the provider deployment cannot undeploy while s0
// references its context. The Go boundary is engine.Undeploy's
// UndeployPreconditionError carrying the context resource.
func (s *contextLifecycleCaseState) undeployError(ctx context.Context, step compat.Step) error {
	err := s.engine.Undeploy(ctx, step.Statement)
	if err == nil {
		return fmt.Errorf("%s: undeploy-error probe %q unexpectedly succeeded", contextLifecycleID, step.Statement)
	}
	var precondition *esper.UndeployPreconditionError
	if !errors.As(err, &precondition) || precondition.Resource == nil ||
		precondition.Resource.Kind != esper.DeploymentResourceContext ||
		precondition.Resource.Name != "NineToFive" || precondition.ReferencedBy != "s0" {
		return fmt.Errorf("%s: undeploy-error probe %q drift: got %v", contextLifecycleID, step.Statement, err)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "undeploy-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// unrepresentable emits the pinned record for a Java surface with no Go
// boundary. The oracle verifies the SPI contract before emitting the same
// record; the Go side only pins the note.
func (s *contextLifecycleCaseState) unrepresentable(step compat.Step) error {
	if step.ExpectError != contextLifecycleVDWNote {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned note %q", contextLifecycleID, step.Statement, step.ExpectError)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeploy removes the deployment registered under the label, or destroys
// the registered context for context labels, mirroring
// undeployModuleContaining. The nw-other-context "createwindow" label is a
// no-op: Go has no named-window removal counterpart and no later probe
// references MyWindow.
func (s *contextLifecycleCaseState) undeploy(ctx context.Context, label string) error {
	if deployment, ok := s.deployments[label]; ok {
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy %q: %w", contextLifecycleID, label, err)
		}
		delete(s.deployments, label)
		return nil
	}
	if contextName, ok := s.contextByLabel[label]; ok {
		if err := s.engine.DestroyContext(ctx, contextName); err != nil {
			return fmt.Errorf("%s: undeploy context %q: %w", contextLifecycleID, label, err)
		}
		delete(s.contextByLabel, label)
		return nil
	}
	if s.caseName == "nw-other-context-onexpr" && label == "createwindow" {
		return nil
	}
	return fmt.Errorf("%s: unknown undeploy label %q", contextLifecycleID, label)
}

// undeployAll undeploys deployments in reverse deploy order — dependents
// retire before the provider deployments whose resources they reference —
// then destroys the registered contexts in reverse registration order.
func (s *contextLifecycleCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextLifecycleID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	for index := len(s.registeredCtx) - 1; index >= 0; index-- {
		contextName := s.registeredCtx[index]
		if _, ok := s.env.Context(contextName); !ok {
			continue
		}
		if err := s.engine.DestroyContext(ctx, contextName); err != nil {
			return fmt.Errorf("%s: undeploy-all context %q: %w", contextLifecycleID, contextName, err)
		}
	}
	s.registeredCtx = nil
	s.contextByLabel = map[string]string{}
	return nil
}

// countRecord emits one count record for the pinned introspection probes:
// "context-count" mirrors SupportContextMgmtHelper.getContextCount (the
// registered context definitions) and "schedule-count-overall" mirrors
// SupportScheduleHelper.scheduleCountOverall (pending schedule callbacks
// across the runtime). The step's declared count is the Java-asserted value;
// the record carries the observed count.
func (s *contextLifecycleCaseState) countRecord(ctx context.Context, step compat.Step) error {
	var count int64
	switch step.Op {
	case "context-count":
		count = int64(len(s.env.Contexts()))
	case "schedule-count-overall":
		n, err := s.engine.ScheduleCountOverall(ctx)
		if err != nil {
			return err
		}
		count = int64(n)
	default:
		return fmt.Errorf("%s: unknown count op %q", contextLifecycleID, step.Op)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: step.Op,
		Count:     &count,
	})
	return nil
}

// decodeContextLifecyclePayload converts a send payload into the typed bean
// for the step's event type.
func decodeContextLifecyclePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextLifecycleBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean: %w", contextLifecycleID, err)
		}
		// charPrimitive mirrors Java's char default: an absent payload
		// field decodes to the NUL character the Java bean carries.
		if value.CharPrimitive == "" {
			value.CharPrimitive = "\x00"
		}
		return value, nil
	case "SupportBean_S0":
		var value contextLifecycleS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S0: %w", contextLifecycleID, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", contextLifecycleID, step.EventType)
	}
}

func contextLifecycleContains(pinned []string, epl string) bool {
	for _, candidate := range pinned {
		if candidate == epl {
			return true
		}
	}
	return false
}

// loadContextLifecycleScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned case metadata, and the pinned per-case step
// keys so unknown or mutated steps fail the replay.
func loadContextLifecycleScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextLifecycleID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextLifecycleID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextLifecycleID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextLifecycleID, err)
	}
	if err := requireContextLifecycleFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextLifecycleID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextLifecycleID ||
		metadata.Description != contextLifecycleDescription ||
		metadata.JavaCommit != contextLifecycleJavaCommit ||
		metadata.JavaSource != contextLifecycleSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextLifecycleID)
	}
	if err := validateContextLifecycleStringArray(root["javaRuntimes"], contextLifecycleJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextLifecycleStringArray(root["javaNames"], contextLifecycleJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextLifecycleStringArray(root["javaStaticIds"], contextLifecycleJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextLifecycleStringArray(root["javaFlags"], contextLifecycleJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextLifecycleCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextLifecycleID, len(contextLifecycleCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextLifecycleFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != contextLifecycleCases[index] ||
			definition.Ordinal != contextLifecycleOrdinals[index] ||
			definition.RuntimeID != contextLifecycleJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextLifecycleJavaExecutions[index] ||
			definition.Observation != contextLifecycleCaseObservations[index] ||
			definition.EPL != contextLifecycleCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextLifecycleID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextLifecycleID, err)
	}
	offset := 0
	for _, caseName := range contextLifecycleCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextLifecycleID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextLifecycleID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextLifecycleID, offset, caseName)
		}
		offset++
		want, ok := contextLifecycleCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextLifecycleID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextLifecycleID, caseName)
		}
		for _, pinned := range want {
			key, err := contextLifecycleStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextLifecycleID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextLifecycleID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextLifecycleID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextLifecycleID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", contextLifecycleID)
	}
	return scenario, nil
}

// contextLifecycleStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count
// with the payload, ids and count compacted. Unknown fields on the step
// object are rejected.
func contextLifecycleStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op                 string          `json:"op"`
		Case               string          `json:"case"`
		Statement          string          `json:"statement"`
		Name               string          `json:"name"`
		EventType          string          `json:"eventType"`
		Epl                string          `json:"epl"`
		Payload            json.RawMessage `json:"payload"`
		ExpectError        string          `json:"expectError"`
		CompileWithoutPath bool            `json:"compileWithoutPath"`
		Mode               string          `json:"mode"`
		IDs                json.RawMessage `json:"ids"`
		Count              json.RawMessage `json:"count"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "mode": true, "ids": true, "count": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	compact := func(value json.RawMessage) (string, error) {
		if len(value) == 0 {
			return "", nil
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, value); err != nil {
			return "", err
		}
		return compacted.String(), nil
	}
	payload, err := compact(step.Payload)
	if err != nil {
		return "", fmt.Errorf("payload: %w", err)
	}
	ids, err := compact(step.IDs)
	if err != nil {
		return "", fmt.Errorf("ids: %w", err)
	}
	count, err := compact(step.Count)
	if err != nil {
		return "", fmt.Errorf("count: %w", err)
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.Mode + "|" + ids + "|" + count, nil
}

// validateContextLifecycleScenario re-checks a decoded scenario (used when
// the runner receives a scenario decoded by the generic loader path). The
// generic compat.Step op whitelist does not cover the probe and count ops
// this scenario pins, so validation is the id and step-count invariants.
func validateContextLifecycleScenario(scenario compat.Scenario) error {
	if scenario.ID != contextLifecycleID {
		return fmt.Errorf("%s scenario id %q is not pinned", contextLifecycleID, scenario.ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", contextLifecycleID)
	}
	return nil
}

func requireContextLifecycleFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextLifecycleID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextLifecycleID, name)
		}
	}
	return nil
}

func validateContextLifecycleStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
