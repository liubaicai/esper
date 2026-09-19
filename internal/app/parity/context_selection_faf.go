package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// context_selection_faf.go replays ContextSelectionAndFireAndForget ords 0-2
// against the pinned Java oracle: two invalid fire-and-forget compiles pin
// the context-partition join rejections over context-bound named windows
// (ord 0); a keyed length(5) aggregate statement exercises the
// iterator-selector matrix — default/all/by-id {0,1,2}/{1}/empty/null sets,
// a nil selector, and a non-context statement — with ordered rows and
// partition descriptors (ord 1); a context-bound keepall named window fed
// by insert-into answers fire-and-forget queries without a context clause,
// with a context clause, and under all/segmented/by-id selectors, plus a
// category-selector invalid-selector rejection (ord 2).
//
// Approved differences (observably identical to the Java EPL):
//   - `create context`, `create window` and `insert into` map to env-level
//     registrations (CreateKeyContext, CreateNamedWindow) and a deployed
//     routing statement; Go has no module path, so the deploy steps only
//     steer the oracle's compiler path accumulation.
//   - The Java execution's prepared-query, parameterized, SODA and
//     no-selector executeQuery forms collapse to the single Go
//     ExecuteFireAndForgetWithSelector boundary; the "all" steps also run
//     ExecuteFireAndForget and assert the same rows like runQueryAll.
//   - The nil-selector and non-context iterator probes pass a nil selector
//     through the "filtered" step encoding (filterProperty "nil" and
//     "non-context"); the empty/null by-id sets encode as filterProperty
//     "ids" with a payload array or null.

const (
	contextSelectionFAFID         = "context-selection-faf"
	contextSelectionFAFJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextSelectionFAFSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextSelectionAndFireAndForget.java"
)

const contextSelectionFAFDescription = "ContextSelectionAndFireAndForget ords 0-2: two invalid fire-and-forget compiles pin the context-partition join rejections over context-bound named windows (ord 0); a keyed length(5) aggregate statement exercises the iterator-selector matrix — default/all/by-id {0,1,2}/{1}/empty/null sets, a nil selector, and a non-context statement — with ordered rows and partition descriptors (ord 1); a context-bound keepall named window fed by insert-into answers fire-and-forget queries without a context clause, with a context clause, and under all/segmented/by-id selectors, plus a category-selector invalid-selector rejection (ord 2)."

var (
	contextSelectionFAFJavaRuntimeIDs = []string{
		"java-runtime-c8c49c4c40e41d383d25",
		"java-runtime-6dd5b9086935002cc50d",
		"java-runtime-bf4cefd62580e2abd2c6",
	}
	contextSelectionFAFJavaExecutions = []string{
		"ContextSelectionAndFireAndForgetInvalid",
		"ContextSelectionIterateStatement",
		"ContextSelectionAndFireAndForgetNamedWindowQuery",
	}
	contextSelectionFAFJavaStaticIDs = []string{
		"java-1c493e94eff9beb161b3",
		"java-1c493e94eff9beb161b3",
		"java-1c493e94eff9beb161b3",
	}
	contextSelectionFAFJavaFlags = []string{"FIREANDFORGET", "INVALIDITY"}
	contextSelectionFAFCases     = []string{
		"invalid",
		"iterate-statement",
		"named-window-query",
	}
	contextSelectionFAFOrdinals = []int{0, 1, 2}
	contextSelectionFAFSources  = []string{contextSelectionFAFSource}
)

var contextSelectionFAFCaseObservations = []string{
	"compile-error; five fixture deploys register two segmented contexts, two context-bound keepall windows and one context-free window; the first probe joins two context-bound windows under a context clause, the second joins two context-bound windows without one; G1/G2 events materialize one partition per key before the second probe",
	"listener+snapshot+snapshot-selector+selector-error; one module deploys the keyed context and s0 (context.key1 plus per-partition sum over length(5)); E1/E2/E2 deliver three listener rows; the default iterator and the all/by-id {0,1,2} selectors yield {E1,10},{E2,41} in order, by-id {1} yields {E2,41}, empty and null id sets yield no rows, a nil selector fails with 'No selector provided', and the context-free s2 fails with 'Iterator with context selector is only supported for statements under context'",
	"faf+selector-error; a keyed context, a context-bound keepall window and an insert-into feed materialize partitions E1={10} and E2={20,21}; context-free sums pin 51 and 41, segmented {E2} and by-id {1} pin 41, context-clause queries pin all three rows and the filtered pair, and a category selector over the keyed context is rejected as an invalid context partition selector",
}

var contextSelectionFAFCaseEPLs = []string{
	"context SegmentedSB select * from WinSB, WinS0",
	"@Name('s0') context PartitionedByString select context.key1 as c0, sum(intPrimitive) as c1 from SupportBean#length(5)",
	"context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow",
}

// contextSelectionFAFDeployEPLs pins the byte-exact EPL each deploy step
// carries; the runner verifies the step EPL before mapping the label to its
// Go fixture.
var contextSelectionFAFDeployEPLs = map[string]map[string]string{
	"invalid": {
		"ctx-sb":     "@public create context SegmentedSB as partition by theString from SupportBean",
		"ctx-s0":     "@public create context SegmentedS0 as partition by p00 from SupportBean_S0",
		"win-sb":     "@public context SegmentedSB create window WinSB#keepall as SupportBean",
		"win-s0":     "@public context SegmentedS0 create window WinS0#keepall as SupportBean_S0",
		"win-s1":     "@public create window WinS1#keepall as SupportBean_S1",
		"ctx-string": "@public create context PartitionedByString partition by theString from SupportBean",
		"win-one":    "@public context PartitionedByString create window MyWindowOne#keepall as SupportBean",
		"ctx-p00":    "@public create context PartitionedByP00 partition by p00 from SupportBean_S0",
		"win-two":    "@public context PartitionedByP00 create window MyWindowTwo#keepall as SupportBean_S0",
	},
	"iterate-statement": {
		"module": "create context PartitionedByString partition by theString from SupportBean;\n@Name('s0') context PartitionedByString select context.key1 as c0, sum(intPrimitive) as c1 from SupportBean#length(5);\n",
		"s2":     "@name('s2') select * from SupportBean",
	},
	"named-window-query": {
		"ctx":    "@public create context PartitionedByString partition by theString from SupportBean",
		"win":    "@public context PartitionedByString create window MyWindow#keepall as SupportBean",
		"insert": "insert into MyWindow select * from SupportBean",
	},
}

// contextSelectionFAFFafEPLs pins the byte-exact EPL each faf step carries.
var contextSelectionFAFFafEPLs = map[string]string{
	"sum-all":           "select sum(intPrimitive) as c1 from MyWindow",
	"sum-filtered":      "select sum(intPrimitive) as c1 from MyWindow where intPrimitive > 15",
	"sum-segmented":     "select sum(intPrimitive) as c1 from MyWindow",
	"sum-byid":          "select sum(intPrimitive) as c1 from MyWindow",
	"context-all":       "context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow",
	"context-filtered":  "context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow where intPrimitive > 15",
	"context-segmented": "context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow where intPrimitive > 15",
	"invalid-selector":  "context PartitionedByString select * from MyWindow",
}

// contextSelectionFAFProbeEPLs pins the byte-exact EPL each build-error step
// carries.
var contextSelectionFAFProbeEPLs = map[string]string{
	"join-context-clause": "context SegmentedSB select * from WinSB, WinS0",
	"join-no-context":     "select mw1.intPrimitive as c1, mw2.id as c2 from MyWindowOne mw1, MyWindowTwo mw2 where mw1.theString = mw2.p00",
}

// contextSelectionFAFCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|
// ids|selector|filterProperty|filterValue keys. Deploy steps carry the
// byte-exact EPL text the Java oracle compiles; "filtered" snapshot-selector
// steps encode the by-id empty/null sets (filterProperty "ids" with a
// payload array or null) and the nil-selector/non-context probes
// (filterProperty "nil" and "non-context"); faf selector "category" marks
// the invalid-selector probe. Java's milestone calls are documented no-ops
// and carry no steps.
var contextSelectionFAFCaseSteps = map[string][]string{
	"invalid": {
		"deploy|ctx-sb|||@public create context SegmentedSB as partition by theString from SupportBean||||||||",
		"deployed|ctx-sb|||||||||||",
		"deploy|ctx-s0|||@public create context SegmentedS0 as partition by p00 from SupportBean_S0||||||||",
		"deployed|ctx-s0|||||||||||",
		"deploy|win-sb|||@public context SegmentedSB create window WinSB#keepall as SupportBean||||||||",
		"deployed|win-sb|||||||||||",
		"deploy|win-s0|||@public context SegmentedS0 create window WinS0#keepall as SupportBean_S0||||||||",
		"deployed|win-s0|||||||||||",
		"deploy|win-s1|||@public create window WinS1#keepall as SupportBean_S1||||||||",
		"deployed|win-s1|||||||||||",
		"build-error|join-context-clause|||context SegmentedSB select * from WinSB, WinS0||Joins in runtime queries for context partitions are not supported||||||",
		"deploy|ctx-string|||@public create context PartitionedByString partition by theString from SupportBean||||||||",
		"deployed|ctx-string|||||||||||",
		"deploy|win-one|||@public context PartitionedByString create window MyWindowOne#keepall as SupportBean||||||||",
		"deployed|win-one|||||||||||",
		"deploy|ctx-p00|||@public create context PartitionedByP00 partition by p00 from SupportBean_S0||||||||",
		"deployed|ctx-p00|||||||||||",
		"deploy|win-two|||@public context PartitionedByP00 create window MyWindowTwo#keepall as SupportBean_S0||||||||",
		"deployed|win-two|||||||||||",
		"send|||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":10}|||||||",
		"send|||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":11}|||||||",
		"send|||SupportBean_S0||{\"id\":1,\"p00\":\"G2\"}|||||||",
		"send|||SupportBean_S0||{\"id\":2,\"p00\":\"G1\"}|||||||",
		"build-error|join-no-context|||select mw1.intPrimitive as c1, mw2.id as c2 from MyWindowOne mw1, MyWindowTwo mw2 where mw1.theString = mw2.p00||Joins against named windows that are under context are not supported||||||",
		"undeploy-all||||||||||||",
	},
	"iterate-statement": {
		"deploy|module|||create context PartitionedByString partition by theString from SupportBean;\n@Name('s0') context PartitionedByString select context.key1 as c0, sum(intPrimitive) as c1 from SupportBean#length(5);\n||||||||",
		"deployed|module|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":21}|||||||",
		"snapshot|s0|||||||||||",
		"snapshot-selector|s0|||||||||all||",
		"snapshot-selector|s0||||||||[0,1,2]|ids||",
		"snapshot-selector|s0||||||||[1]|ids||",
		"snapshot-selector|s0||||[]|||||filtered|ids|",
		"snapshot-selector|s0||||null|||||filtered|ids|",
		"snapshot-selector|s0|||||No selector provided||||filtered|nil|",
		"deploy|s2|||@name('s2') select * from SupportBean||||||||",
		"deployed|s2|||||||||||",
		"snapshot-selector|s2|||||Iterator with context selector is only supported for statements under context||||filtered|non-context|",
		"undeploy-all||||||||||||",
	},
	"named-window-query": {
		"deploy|ctx|||@public create context PartitionedByString partition by theString from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"deploy|win|||@public context PartitionedByString create window MyWindow#keepall as SupportBean||||||||",
		"deployed|win|||||||||||",
		"deploy|insert|||insert into MyWindow select * from SupportBean||||||||",
		"deployed|insert|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":21}|||||||",
		"faf|sum-all|||select sum(intPrimitive) as c1 from MyWindow||||||all||",
		"faf|sum-filtered|||select sum(intPrimitive) as c1 from MyWindow where intPrimitive > 15||||||all||",
		"faf|sum-segmented|||select sum(intPrimitive) as c1 from MyWindow|[[\"E2\"]]|||||segmented||",
		"faf|sum-byid|||select sum(intPrimitive) as c1 from MyWindow|||||[1]|ids||",
		"faf|context-all|||context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow||||||all||",
		"faf|context-filtered|||context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow where intPrimitive > 15||||||all||",
		"faf|context-segmented|||context PartitionedByString select context.key1 as c0, intPrimitive as c1 from MyWindow where intPrimitive > 15|[[\"E2\"]]|||||segmented||",
		"faf|invalid-selector|||context PartitionedByString select * from MyWindow||Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorFiltered, ContextPartitionSelectorById, ContextPartitionSelectorSegmented] interfaces but received com||||category||",
		"undeploy-all||||||||||||",
	},
}

// contextSelectionFAFBean mirrors SupportBean for the selection cases.
type contextSelectionFAFBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// contextSelectionFAFS0 mirrors SupportBean_S0 for the selection cases.
type contextSelectionFAFS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// contextSelectionFAFS1 mirrors SupportBean_S1 for the selection cases; the
// type is registered but never sent.
type contextSelectionFAFS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// contextSelectionFAFCaseState carries the per-case replay state: the
// environment/engine pair, label->deployment bookkeeping, the deployed
// statements, the per-statement listener sequence counters and the trace.
type contextSelectionFAFCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
	statements  map[string]*esper.Statement
	deployOrder []string
	sequences   map[string]uint64
	trace       *compat.Trace
	caseName    string
}

func runContextSelectionFAFScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextSelectionFAFScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextSelectionFAF(ctx, scenario, &trace)
}

// executeContextSelectionFAF replays the scenario: each case runs on a
// fresh environment/engine pair (one runtime per Java execution) and every
// step dispatches to the matching runtime action.
func executeContextSelectionFAF(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextSelectionFAFCaseState
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
			state, err = startContextSelectionFAFCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "deployed":
			state.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      state.caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequences[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(state.engine.Now()),
			})
		case "send":
			event, err := decodeContextSelectionFAFPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return *trace, err
			}
		case "snapshot-selector":
			if err := state.snapshotSelector(ctx, step); err != nil {
				return *trace, err
			}
		case "faf":
			if err := state.faf(ctx, step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextSelectionFAFID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startContextSelectionFAFCase builds the fresh per-case environment: the
// three event types the suite registers plus the engine pinned to the
// case's Java runtime id at the epoch start time.
func startContextSelectionFAFCase(caseName string, trace *compat.Trace) (*contextSelectionFAFCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextSelectionFAFBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextSelectionFAFS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextSelectionFAFS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	state := &contextSelectionFAFCaseState{
		env:         env,
		deployments: map[string]*esper.Deployment{},
		statements:  map[string]*esper.Statement{},
		sequences:   map[string]uint64{},
		trace:       trace,
		caseName:    caseName,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextSelectionFAFJavaRuntimeIDs[contextSelectionFAFCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextSelectionFAFCaseOrdinal(caseName string) int {
	for index, name := range contextSelectionFAFCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy executes one deploy step: create-context and create-window EPL map
// to env-level registrations (the established approved difference for the
// missing deployable statement types), the ord-1 module registers the
// context and deploys s0 with its listener, and the ord-2 insert deploys
// the routing statement. The step EPL is verified against the pinned text
// before the label maps to its fixture.
func (s *contextSelectionFAFCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	pinned, ok := contextSelectionFAFDeployEPLs[s.caseName][label]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", contextSelectionFAFID, label, step.Epl)
	}
	beanSource := esper.From[contextSelectionFAFBean](s.env, "SupportBean")
	theString := esper.Field[contextSelectionFAFBean, string]("theString")
	intPrimitive := esper.Field[contextSelectionFAFBean, int]("intPrimitive")
	registerKeyContext := func(name, eventType string, key esper.Expr) error {
		// Go contexts are environment-scoped registrations; the deploy label
		// owns the registration like the category runner's contextByLabel.
		if _, exists := s.env.Context(name); exists {
			return fmt.Errorf("%s: context %q is already registered", contextSelectionFAFID, name)
		}
		_, err := esper.CreateKeyContextByStreams(s.env, name, esper.KeyContextStream{
			Type: eventType,
			Keys: []esper.Expr{key},
		})
		return err
	}
	createWindow := func(name, eventType, contextName string) error {
		schema, ok := s.env.Schema(eventType)
		if !ok {
			return fmt.Errorf("%s: %s schema is not registered", contextSelectionFAFID, eventType)
		}
		options := []esper.NamedWindowOption{esper.NamedWindowRetention(esper.KeepAll())}
		if contextName != "" {
			options = append([]esper.NamedWindowOption{esper.NamedWindowContext(contextName)}, options...)
		}
		_, err := esper.CreateNamedWindow(s.env, name, schema, options...)
		return err
	}
	switch s.caseName {
	case "invalid":
		switch label {
		case "ctx-sb":
			return registerKeyContext("SegmentedSB", "SupportBean", theString)
		case "ctx-s0":
			return registerKeyContext("SegmentedS0", "SupportBean_S0",
				esper.Field[contextSelectionFAFS0, string]("p00"))
		case "win-sb":
			return createWindow("WinSB", "SupportBean", "SegmentedSB")
		case "win-s0":
			return createWindow("WinS0", "SupportBean_S0", "SegmentedS0")
		case "win-s1":
			return createWindow("WinS1", "SupportBean_S1", "")
		case "ctx-string":
			return registerKeyContext("PartitionedByString", "SupportBean", theString)
		case "win-one":
			return createWindow("MyWindowOne", "SupportBean", "PartitionedByString")
		case "ctx-p00":
			return registerKeyContext("PartitionedByP00", "SupportBean_S0",
				esper.Field[contextSelectionFAFS0, string]("p00"))
		case "win-two":
			return createWindow("MyWindowTwo", "SupportBean_S0", "PartitionedByP00")
		}
	case "iterate-statement":
		switch label {
		case "module":
			if err := registerKeyContext("PartitionedByString", "SupportBean", theString); err != nil {
				return err
			}
			plan, err := s.env.Build(beanSource.Window(esper.LengthWindow(5)).Aggregate(
				esper.Alias("c0", esper.ContextField[string]("key1")),
				esper.Alias("c1", esper.Sum[int](intPrimitive)),
			).Query(esper.StatementName("s0"), esper.WithContext("PartitionedByString")))
			if err != nil {
				return err
			}
			deployment, err := s.deployPlan(label, plan)
			if err != nil {
				return err
			}
			// The Java execution calls addListener("s0") on this deployment.
			for _, statement := range deployment.Statements() {
				if statement.Name() == "s0" {
					return s.subscribeStatement(statement)
				}
			}
			return fmt.Errorf("%s: module deploy produced no s0 statement", contextSelectionFAFID)
		case "s2":
			plan, err := s.env.Build(beanSource.Query(esper.StatementName("s2")))
			if err != nil {
				return err
			}
			_, err = s.deployPlan(label, plan)
			return err
		}
	case "named-window-query":
		switch label {
		case "ctx":
			return registerKeyContext("PartitionedByString", "SupportBean", theString)
		case "win":
			return createWindow("MyWindow", "SupportBean", "PartitionedByString")
		case "insert":
			// `insert into MyWindow select * from SupportBean` — the routing
			// statement carries no context clause; the window's context
			// routes each event to its partition like the Java insert-into.
			plan, err := s.env.Build(beanSource.InsertInto("MyWindow"))
			if err != nil {
				return err
			}
			_, err = s.deployPlan(label, plan)
			return err
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextSelectionFAFID, s.caseName, label)
}

// deployPlan deploys one built plan under the step label and records the
// deployment for undeploy-all.
func (s *contextSelectionFAFCaseState) deployPlan(label string, plan esper.Plan) (*esper.Deployment, error) {
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return nil, fmt.Errorf("%s: deploy %q: %w", contextSelectionFAFID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
	}
	return deployment, nil
}

// subscribeStatement mirrors the oracle's listener: one listener record per
// delivered batch with normalized row fields.
func (s *contextSelectionFAFCaseState) subscribeStatement(statement *esper.Statement) error {
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequences[statement.Name()]++
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  s.sequences[statement.Name()],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	})
	return err
}

// snapshot emits one snapshot record mirroring the Java execution's
// assertPropsPerRowIterator("s0", ...) call: the statement's default
// iterator rows in partition-allocation order plus the statement's
// partition descriptors.
func (s *contextSelectionFAFCaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed", contextSelectionFAFID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return err
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:       s.caseName,
		Operation:  "snapshot",
		Statement:  step.Statement,
		Time:       compat.FormatTraceTime(s.engine.Now()),
		New:        compat.NormalizeResults(result.Results()),
		Partitions: contextSelectionFAFPartitions(statement.ContextPartitions()),
	})
	return nil
}

// snapshotSelector replays one selector-targeted iterator step, mirroring
// the Java execution's statement.iterator(selector) calls: "all" maps to
// ContextPartitionSelectorAll, "ids" maps to SelectContextPartitionIDs over
// the step's id array, "filtered" with filterProperty "ids" maps to
// SelectContextPartitionIDs over the payload id set (an empty array and
// null both select nothing), and "filtered" with filterProperty "nil" or
// "non-context" passes a nil selector to pin the 'No selector provided' and
// non-context rejections.
func (s *contextSelectionFAFCaseState) snapshotSelector(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot-selector statement %q was not deployed", contextSelectionFAFID, step.Statement)
	}
	var selector esper.ContextPartitionSelector
	switch step.Selector {
	case "all":
		selector = esper.ContextPartitionSelectorAll{}
	case "ids":
		selector = esper.SelectContextPartitionIDs(step.IDs...)
	case "filtered":
		switch step.FilterProperty {
		case "ids":
			var ids []int
			if len(step.Payload) > 0 && string(step.Payload) != "null" {
				if err := json.Unmarshal(step.Payload, &ids); err != nil {
					return fmt.Errorf("%s: selector ids payload: %w", contextSelectionFAFID, err)
				}
			}
			selector = esper.SelectContextPartitionIDs(ids...)
		case "nil", "non-context":
			// The nil-selector and non-context probes pass a nil selector;
			// the statement decides which rejection fires.
			selector = nil
		default:
			return fmt.Errorf("%s: filtered selector property %q is not pinned", contextSelectionFAFID, step.FilterProperty)
		}
	default:
		return fmt.Errorf("%s: unsupported selector %q", contextSelectionFAFID, step.Selector)
	}
	result, err := statement.SnapshotWithSelector(ctx, selector)
	if err != nil {
		if step.ExpectError == "" {
			return err
		}
		// Verify the Go rejection is the InvalidRule boundary before
		// recording the pinned Java message; the nil-selector and
		// non-context probes pin the exact wording like Java's assertEquals.
		var espErr *esper.Error
		if !errors.As(err, &espErr) || espErr.Code != esper.ErrorInvalidRule {
			return fmt.Errorf("%s: selector-error drift for %q: got %v", contextSelectionFAFID, step.Selector, err)
		}
		if (step.FilterProperty == "nil" || step.FilterProperty == "non-context") && espErr.Message != step.ExpectError {
			return fmt.Errorf("%s: selector-error wording drift for %q: got %v", contextSelectionFAFID, step.FilterProperty, err)
		}
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "selector-error",
			Statement: step.Statement,
			Time:      compat.FormatTraceTime(s.engine.Now()),
			Value:     step.ExpectError,
		})
		return nil
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:       s.caseName,
		Operation:  "snapshot-selector",
		Statement:  step.Statement,
		Time:       compat.FormatTraceTime(s.engine.Now()),
		New:        compat.NormalizeResults(result.Results()),
		Partitions: contextSelectionFAFPartitions(statement.ContextPartitionsWith(selector)),
	})
	return nil
}

// faf replays one fire-and-forget step, mirroring the Java execution's
// runQuery/runQueryAll helpers: the pinned plan executes through
// ExecuteFireAndForgetWithSelector for the mapped selector; "all" steps
// additionally run the no-selector ExecuteFireAndForget like runQueryAll
// and assert the same rows. Selector "category" pins the invalid-selector
// rejection of a category selector over the keyed context. Rows sort by
// their compact field rendering because the Java assertions are any-order.
func (s *contextSelectionFAFCaseState) faf(ctx context.Context, step compat.Step) error {
	label := step.Statement
	pinned, ok := contextSelectionFAFFafEPLs[label]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: faf %q carries an unpinned EPL %q", contextSelectionFAFID, label, step.Epl)
	}
	var selector esper.ContextPartitionSelector
	switch step.Selector {
	case "all":
		selector = esper.ContextPartitionSelectorAll{}
	case "ids":
		selector = esper.SelectContextPartitionIDs(step.IDs...)
	case "segmented":
		var keys [][]any
		if len(step.Payload) > 0 && string(step.Payload) != "null" {
			if err := json.Unmarshal(step.Payload, &keys); err != nil {
				return fmt.Errorf("%s: faf segmented payload: %w", contextSelectionFAFID, err)
			}
		}
		selector = esper.SelectContextPartitionSegments(keys...)
	case "category":
		// The Java probe passes a category selector with null labels; the
		// empty Go label set is the same no-match selector, and the kind
		// mismatch against the keyed context fires first anyway.
		selector = esper.SelectContextPartitionCategories()
	default:
		return fmt.Errorf("%s: unsupported faf selector %q", contextSelectionFAFID, step.Selector)
	}
	plan, err := s.buildFafPlan(label)
	if err != nil {
		return err
	}
	if step.Selector == "category" {
		_, err := s.engine.ExecuteFireAndForgetWithSelector(ctx, plan, selector)
		if err == nil {
			return fmt.Errorf("%s: faf %q unexpectedly accepted a category selector", contextSelectionFAFID, label)
		}
		var espErr *esper.Error
		if !errors.As(err, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
			!strings.Contains(espErr.Message, "Invalid context partition selector") {
			return fmt.Errorf("%s: faf %q selector-error drift: got %v", contextSelectionFAFID, label, err)
		}
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "selector-error",
			Statement: label,
			Time:      compat.FormatTraceTime(s.engine.Now()),
			Value:     step.ExpectError,
		})
		return nil
	}
	result, err := s.engine.ExecuteFireAndForgetWithSelector(ctx, plan, selector)
	if err != nil {
		return fmt.Errorf("%s: faf %q: %w", contextSelectionFAFID, label, err)
	}
	rows := contextSelectionFAFSortedRows(compat.NormalizeResults(result.Results()))
	if step.Selector == "all" {
		// runQueryAll also runs the same query without a selector; the Go
		// no-selector form must return the same rows.
		plain, err := s.engine.ExecuteFireAndForget(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: faf %q no-selector: %w", contextSelectionFAFID, label, err)
		}
		plainRows := contextSelectionFAFSortedRows(compat.NormalizeResults(plain.Results()))
		if !reflect.DeepEqual(rows, plainRows) {
			return fmt.Errorf("%s: faf %q no-selector rows drift: got %v want %v", contextSelectionFAFID, label, plainRows, rows)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "faf",
		Statement: label,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		New:       rows,
	})
	return nil
}

// buildFafPlan maps one faf step label to the pinned Go plan: the
// context-free sums read the named window directly, the context-clause
// queries carry WithContext("PartitionedByString"), and the invalid-selector
// probe is the wildcard context-clause read.
func (s *contextSelectionFAFCaseState) buildFafPlan(label string) (esper.Plan, error) {
	window := esper.FromNamedWindow(s.env, "MyWindow")
	intPrimitive := esper.Field[any, int]("intPrimitive")
	var query esper.Query
	switch label {
	case "sum-all", "sum-segmented", "sum-byid":
		query = window.Aggregate(
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query()
	case "sum-filtered":
		query = window.Filter(esper.Greater[int](intPrimitive, esper.Literal(15))).Aggregate(
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query()
	case "context-all":
		query = window.Select(
			esper.Alias("c0", esper.ContextField[string]("key1")),
			esper.Alias("c1", intPrimitive),
		).Query(esper.WithContext("PartitionedByString"))
	case "context-filtered", "context-segmented":
		query = window.Filter(esper.Greater[int](intPrimitive, esper.Literal(15))).Select(
			esper.Alias("c0", esper.ContextField[string]("key1")),
			esper.Alias("c1", intPrimitive),
		).Query(esper.WithContext("PartitionedByString"))
	case "invalid-selector":
		query = window.Query(esper.WithContext("PartitionedByString"))
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf statement %q", contextSelectionFAFID, label)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return esper.Plan{}, fmt.Errorf("%s: faf %q build: %w", contextSelectionFAFID, label, err)
	}
	return plan, nil
}

// buildError runs one expected-invalid fire-and-forget compile probe
// against the fluent equivalent of the pinned EPL. The Go plan builds
// first; when the plan builds, the FAF execute boundary carries the
// rejection (the shared-core FAF join checks run at execution time). The
// record carries the pinned Java message prefix once the Go rejection
// verifies, matching the oracle's prefix assertion (the approved wording
// difference).
func (s *contextSelectionFAFCaseState) buildError(step compat.Step) error {
	pinned, ok := contextSelectionFAFProbeEPLs[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", contextSelectionFAFID, step.Statement, step.Epl)
	}
	var probeErr error
	switch step.Statement {
	case "join-context-clause":
		// `context SegmentedSB select * from WinSB, WinS0` — a join of two
		// context-bound windows under a context clause. The plan must build
		// (deployed context joins are legal; only compileFAF rejects them),
		// so the probe carries a projection and the rejection surfaces at
		// the FAF execute boundary.
		plan, err := s.env.Build(esper.Join(
			esper.FromNamedWindowAs[contextSelectionFAFBean](s.env, "WinSB"),
			esper.FromNamedWindowAs[contextSelectionFAFS0](s.env, "WinS0"),
		).Select(
			esper.SelectFrom(0, "c1", esper.JoinField[int](0, "intPrimitive")),
			esper.SelectFrom(1, "c2", esper.JoinField[int](1, "id")),
		).Query(esper.WithContext("SegmentedSB")))
		if err != nil {
			probeErr = err
			break
		}
		_, probeErr = s.engine.ExecuteFireAndForget(context.Background(), plan)
	case "join-no-context":
		// `select mw1.intPrimitive as c1, mw2.id as c2 from MyWindowOne mw1,
		// MyWindowTwo mw2 where mw1.theString = mw2.p00` — a join of two
		// context-bound windows without a context clause.
		plan, err := s.env.Build(esper.Join(
			esper.FromNamedWindowAs[contextSelectionFAFBean](s.env, "MyWindowOne"),
			esper.FromNamedWindowAs[contextSelectionFAFS0](s.env, "MyWindowTwo"),
		).Select(
			esper.SelectFrom(0, "c1", esper.JoinField[int](0, "intPrimitive")),
			esper.SelectFrom(1, "c2", esper.JoinField[int](1, "id")),
		).Where(esper.Equal[string](
			esper.JoinField[string](0, "theString"),
			esper.JoinField[string](1, "p00"),
		)).Query())
		if err != nil {
			probeErr = err
			break
		}
		_, probeErr = s.engine.ExecuteFireAndForget(context.Background(), plan)
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", contextSelectionFAFID, step.Statement)
	}
	if probeErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", contextSelectionFAFID, step.Statement)
	}
	// Verify the Go rejection carries the expected wording before recording
	// the pinned Java prefix, mirroring the category runner's gate.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"join-context-clause": {esper.ErrorInvalidRule, "Joins in runtime queries for context partitions are not supported"},
		"join-no-context":     {esper.ErrorInvalidRule, "Joins against named windows that are under context are not supported"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(probeErr, &espErr) || espErr.Code != want.code || !strings.Contains(probeErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", contextSelectionFAFID, step.Statement, probeErr)
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

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the modules they reference, mirroring undeployAll. The
// env-scoped context and window registrations retire with the engine.
func (s *contextSelectionFAFCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextSelectionFAFID, label, err)
		}
		delete(s.deployments, label)
		for _, statement := range deployment.Statements() {
			delete(s.statements, statement.Name())
		}
	}
	s.deployOrder = nil
	return nil
}

// decodeContextSelectionFAFPayload converts a send payload into the typed
// bean for the step's event type.
func decodeContextSelectionFAFPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextSelectionFAFBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean: %w", contextSelectionFAFID, err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextSelectionFAFS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S0: %w", contextSelectionFAFID, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", contextSelectionFAFID, step.EventType)
	}
}

// contextSelectionFAFPartitions renders the statement-scoped partition
// identifiers the Java oracle emits for snapshot records: one record per
// ContextPartitionIdentifierPartitioned carrying its id, key and empty
// properties object. The shared normalizer renders the raw key value as
// "key:<value>", matching the oracle's identifier projection.
func contextSelectionFAFPartitions(descriptors []esper.ContextPartitionDescriptor) []compat.PartitionRecord {
	return compat.NormalizePartitions(descriptors)
}

// contextSelectionFAFSortedRows canonicalizes row order for the Java
// assertPropsPerRowAnyOrder assertions: rows sort by their compact JSON
// field rendering so both traces pin the same order.
func contextSelectionFAFSortedRows(rows []compat.ResultRecord) []compat.ResultRecord {
	if len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := json.Marshal(rows[i].Fields)
		right, _ := json.Marshal(rows[j].Fields)
		return string(left) < string(right)
	})
	return rows
}

// loadContextSelectionFAFScenario decodes the scenario with the strict
// pinned checks the raw-mutation tests pin: no duplicate JSON keys, the
// exact top-level field set, pinned case metadata, and the complete pinned
// step sequence per case.
func loadContextSelectionFAFScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextSelectionFAFID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextSelectionFAFID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextSelectionFAFID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextSelectionFAFID, err)
	}
	if err := requireContextSelectionFAFFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextSelectionFAFID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextSelectionFAFID ||
		metadata.Description != contextSelectionFAFDescription ||
		metadata.JavaCommit != contextSelectionFAFJavaCommit ||
		metadata.JavaSource != contextSelectionFAFSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextSelectionFAFID)
	}
	if err := validateContextSelectionFAFStringArray(root["javaRuntimes"], contextSelectionFAFJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextSelectionFAFStringArray(root["javaNames"], contextSelectionFAFJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextSelectionFAFStringArray(root["javaStaticIds"], contextSelectionFAFJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextSelectionFAFStringArray(root["javaFlags"], contextSelectionFAFJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextSelectionFAFCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextSelectionFAFID, len(contextSelectionFAFCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextSelectionFAFFields(object,
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
		if definition.Case != contextSelectionFAFCases[index] ||
			definition.Ordinal != contextSelectionFAFOrdinals[index] ||
			definition.RuntimeID != contextSelectionFAFJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextSelectionFAFJavaExecutions[index] ||
			definition.Observation != contextSelectionFAFCaseObservations[index] ||
			definition.EPL != contextSelectionFAFCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextSelectionFAFID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextSelectionFAFID, err)
	}
	offset := 0
	for _, caseName := range contextSelectionFAFCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextSelectionFAFID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextSelectionFAFID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextSelectionFAFID, offset, caseName)
		}
		offset++
		want, ok := contextSelectionFAFCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextSelectionFAFID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextSelectionFAFID, caseName)
		}
		for _, pinned := range want {
			key, err := contextSelectionFAFStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextSelectionFAFID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextSelectionFAFID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextSelectionFAFID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextSelectionFAFID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// contextSelectionFAFStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|
// ids|selector|filterProperty|filterValue with the payload and ids compacted.
// Unknown fields on the step object are rejected.
func contextSelectionFAFStepKey(raw json.RawMessage) (string, error) {
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
		Selector           string          `json:"selector"`
		FilterProperty     string          `json:"filterProperty"`
		FilterValue        string          `json:"filterValue"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "mode": true, "ids": true,
		"selector": true, "filterProperty": true, "filterValue": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payload := ""
	if len(step.Payload) > 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("payload: %w", err)
		}
		payload = compacted.String()
	}
	ids := ""
	if len(step.IDs) > 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.IDs); err != nil {
			return "", fmt.Errorf("ids: %w", err)
		}
		ids = compacted.String()
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.Mode + "|" + ids + "|" + step.Selector + "|" + step.FilterProperty +
		"|" + step.FilterValue, nil
}

// validateContextSelectionFAFScenario re-checks a decoded scenario (used
// when the runner receives a scenario decoded by the generic loader path).
func validateContextSelectionFAFScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != contextSelectionFAFID {
		return fmt.Errorf("%s scenario id %q is not pinned", contextSelectionFAFID, scenario.ID)
	}
	return nil
}

func requireContextSelectionFAFFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextSelectionFAFID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextSelectionFAFID, name)
		}
	}
	return nil
}

func validateContextSelectionFAFStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
