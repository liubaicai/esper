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

// context_selection_faf_nested.go replays ContextSelectionAndFireAndForget
// ord 3 (ContextSelectionFAFNestedNamedWindowQuery,
// java-runtime-f51a1493ad61c1f0d0d1) against the pinned Java oracle: a nested
// context whose ACtx parent is an overlapping initiated-terminated context
// (SupportBean_S0 starts a partition, SupportBean_S1(id=s0.id) ends it) over
// a three-category BCtx child (grp1 intPrimitive < 0, grp2 = 0, grp3 > 0) on
// SupportBean. Every S0 activation eagerly instantiates all three category
// leaves in declaration order with globally allocated leaf ids, so S0(1)
// materializes leaves 0-2 and S0(2) materializes leaves 3-5. SupportBean
// events broadcast to every live parent partition and land in one leaf per
// parent by category; the triggering S0 never enters MyWindow. The
// context-bound keepall window fed by insert-into answers the group-by
// merge fire-and-forget query across all leaves ({E1,5},{E2,-2},{E3,10}),
// the same query under a by-id {2} selector ({E1,3},{E3,5}), and the
// per-partition context-clause query projecting context.ACtx.s0.p00 and
// context.BCtx.label under by-id {2} ({S0_1,grp3,E1,3},{S0_1,grp3,E3,5}).
// A context-level snapshot-selector step pins the six leaf descriptors
// (id, composite key, startTime/endTime, initiating S0 p00, category
// label), and a segmented selector over the nested context is rejected as
// an invalid context partition selector.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context` maps to env-level registrations: the ACtx level is an
//     overlapping initiated-terminated context (CreateOverlappingInitiated
//     TerminatedContext with the S1 end predicate correlated through
//     ContextInitiatingEvent), the BCtx level is a three-category context
//     (NewCategoryContext), and CreateNestedContext composes them; `create
//     window` and `insert into` map to CreateNamedWindow with
//     NamedWindowContext plus a deployed routing statement. Go has no module
//     path, so the deploy steps only steer the oracle's compiler path
//     accumulation.
//   - context.ACtx.s0.p00 reads the parent level's initiating event through
//     ContextField[esper.Event]("parent.initiating_event") plus Property,
//     and context.BCtx.label reads ContextField[string]("label").
//   - The Java execution's prepared-query, parameterized, SODA and
//     no-selector executeQuery forms collapse to the single Go
//     ExecuteFireAndForgetWithSelector boundary; the "all" steps also run
//     ExecuteFireAndForget and assert the same rows like runQueryAll.
//   - The context-level snapshot-selector step (name set) reads
//     engine.ContextPartitionDescriptors over the nested context's leaf
//     partitions, mirroring EPContextPartitionService.getContextPartitions;
//     statement-scoped steps keep the template's SnapshotWithSelector path.

const (
	contextSelectionFAFNestedID         = "context-selection-faf-nested"
	contextSelectionFAFNestedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextSelectionFAFNestedSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextSelectionAndFireAndForget.java"
)

const contextSelectionFAFNestedDescription = "ContextSelectionAndFireAndForget ord 3 (ContextSelectionFAFNestedNamedWindowQuery): a nested context whose ACtx parent is an overlapping initiated-terminated context (SupportBean_S0 starts a partition, SupportBean_S1(id=s0.id) ends it) over a three-category BCtx child (grp1 intPrimitive < 0, grp2 = 0, grp3 > 0) on SupportBean. Each S0 activation eagerly instantiates all three category leaves in declaration order with globally allocated leaf ids — S0(1) materializes leaves 0-2, S0(2) materializes leaves 3-5. SupportBean events broadcast to every live parent partition and land in one leaf per parent by category; the triggering S0 never enters MyWindow. The context-bound keepall window fed by insert-into answers the group-by merge fire-and-forget query across all leaves ({E1,5},{E2,-2},{E3,10}), the same query under a by-id {2} selector ({E1,3},{E3,5}), and the per-partition context-clause query projecting context.ACtx.s0.p00 and context.BCtx.label under by-id {2} ({S0_1,grp3,E1,3},{S0_1,grp3,E3,5}). A context-level snapshot-selector step pins the six leaf descriptors (id, composite key, startTime/endTime, initiating S0 p00, category label), and a segmented selector over the nested context is rejected as an invalid context partition selector."

var (
	contextSelectionFAFNestedJavaRuntimeIDs = []string{
		"java-runtime-f51a1493ad61c1f0d0d1",
	}
	contextSelectionFAFNestedJavaExecutions = []string{
		"ContextSelectionFAFNestedNamedWindowQuery",
	}
	contextSelectionFAFNestedJavaStaticIDs = []string{
		"java-1c493e94eff9beb161b3",
	}
	contextSelectionFAFNestedJavaFlags = []string{"FIREANDFORGET"}
	contextSelectionFAFNestedCases     = []string{
		"nested-named-window-query",
	}
	contextSelectionFAFNestedOrdinals = []int{3}
	contextSelectionFAFNestedSources  = []string{contextSelectionFAFNestedSource}
)

var contextSelectionFAFNestedCaseObservations = []string{
	"deploy+send+snapshot-selector+faf+selector-error; the nested context deploys an overlapping initiated ACtx parent (S0 starts, S1(id=s0.id) ends) over a three-category BCtx child; S0(1) and S0(2) eagerly materialize leaves 0-2 and 3-5; SupportBean events broadcast to all live parents and route by category so leaf2={E1(1),E3(5),E1(2)} and leaf5={E3(5),E1(2)}; the all-selector group-by merge pins {E1,5},{E2,-2},{E3,10}, by-id {2} pins {E1,3},{E3,5}, the context-clause per-partition query pins {S0_1,grp3,E1,3},{S0_1,grp3,E3,5}, and a segmented selector is rejected as an invalid context partition selector",
}

var contextSelectionFAFNestedCaseEPLs = []string{
	"context NestedContext select context.ACtx.s0.p00 as c1, context.BCtx.label as c2, theString as c3, sum(intPrimitive) as c4 from MyWindow group by theString",
}

// contextSelectionFAFNestedDeployEPLs pins the byte-exact EPL each deploy
// step carries; the runner verifies the step EPL before mapping the label
// to its Go fixture.
var contextSelectionFAFNestedDeployEPLs = map[string]map[string]string{
	"nested-named-window-query": {
		"ctx":    "@public create context NestedContext context ACtx initiated by SupportBean_S0 as s0 terminated by SupportBean_S1(id=s0.id), context BCtx group by intPrimitive < 0 as grp1, group by intPrimitive = 0 as grp2, group by intPrimitive > 0 as grp3 from SupportBean",
		"win":    "@public context NestedContext create window MyWindow#keepall as SupportBean",
		"insert": "insert into MyWindow select * from SupportBean",
	},
}

// contextSelectionFAFNestedFafEPLs pins the byte-exact EPL each faf step
// carries.
var contextSelectionFAFNestedFafEPLs = map[string]string{
	"sum-all":          "select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString",
	"sum-byid":         "select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString",
	"context-byid":     "context NestedContext select context.ACtx.s0.p00 as c1, context.BCtx.label as c2, theString as c3, sum(intPrimitive) as c4 from MyWindow group by theString",
	"invalid-selector": "context NestedContext select * from MyWindow",
}

// contextSelectionFAFNestedProbeEPLs pins the byte-exact EPL each
// compile-error/build-error step carries; the nested case has no compile
// probes so the map is empty and any probe step fails the pinned check.
var contextSelectionFAFNestedProbeEPLs = map[string]string{}

// contextSelectionFAFNestedCaseSteps pins the complete step sequence per
// case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
// mode|ids|selector|filterProperty|filterValue|at keys. Deploy steps carry
// the byte-exact EPL text the Java oracle compiles; the snapshot-selector
// step carries the context name in "name" and reads context-level leaf
// descriptors; faf selector "segmented" marks the invalid-selector probe.
// Java's milestone calls are documented no-ops and carry no steps.
var contextSelectionFAFNestedCaseSteps = map[string][]string{
	"nested-named-window-query": {
		"deploy|ctx|||@public create context NestedContext context ACtx initiated by SupportBean_S0 as s0 terminated by SupportBean_S1(id=s0.id), context BCtx group by intPrimitive < 0 as grp1, group by intPrimitive = 0 as grp2, group by intPrimitive > 0 as grp3 from SupportBean|||||||||",
		"deployed|ctx||||||||||||",
		"deploy|win|||@public context NestedContext create window MyWindow#keepall as SupportBean|||||||||",
		"deployed|win||||||||||||",
		"deploy|insert|||insert into MyWindow select * from SupportBean|||||||||",
		"deployed|insert||||||||||||",
		"send|||SupportBean_S0||{\"id\":1,\"p00\":\"S0_1\"}||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||||||||",
		"send|||SupportBean_S0||{\"id\":2,\"p00\":\"S0_2\"}||||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":-1}||||||||",
		"send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":5}||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":2}||||||||",
		"snapshot-selector|ctx|NestedContext||||||||all|||",
		"faf|sum-all|||select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString||||||all|||",
		"faf|sum-byid|||select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString|||||[2]|ids|||",
		"faf|context-byid|||context NestedContext select context.ACtx.s0.p00 as c1, context.BCtx.label as c2, theString as c3, sum(intPrimitive) as c4 from MyWindow group by theString|||||[2]|ids|||",
		"faf|invalid-selector|||context NestedContext select * from MyWindow||Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorById, ContextPartitionSelectorNested] interfaces but received com||||segmented|||",
		"undeploy-all|||||||||||||",
	},
}

// contextSelectionFAFNestedBean mirrors SupportBean for the nested case.
type contextSelectionFAFNestedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// contextSelectionFAFNestedS0 mirrors SupportBean_S0, the ACtx initiating
// event.
type contextSelectionFAFNestedS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// contextSelectionFAFNestedS1 mirrors SupportBean_S1, the ACtx terminating
// event; the type is registered but never sent in the pinned sequence.
type contextSelectionFAFNestedS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// contextSelectionFAFNestedCaseState carries the per-case replay state: the
// environment/engine pair, label->deployment bookkeeping, the deployed
// statements, the per-statement listener sequence counters and the trace.
type contextSelectionFAFNestedCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
	statements  map[string]*esper.Statement
	deployOrder []string
	sequences   map[string]uint64
	trace       *compat.Trace
	caseName    string
}

func runContextSelectionFAFNestedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextSelectionFAFNestedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextSelectionFAFNested(ctx, scenario, &trace)
}

// executeContextSelectionFAFNested replays the scenario: each case runs on
// a fresh environment/engine pair (one runtime per Java execution) and
// every step dispatches to the matching runtime action.
func executeContextSelectionFAFNested(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextSelectionFAFNestedCaseState
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
			state, err = startContextSelectionFAFNestedCase(step.Case, trace)
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
			event, err := decodeContextSelectionFAFNestedPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return *trace, fmt.Errorf("%s advance-time: %w", contextSelectionFAFNestedID, err)
			}
			if err := state.engine.AdvanceTime(ctx, at); err != nil {
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
		case "compile-error", "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextSelectionFAFNestedID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startContextSelectionFAFNestedCase builds the fresh per-case environment:
// the three event types the suite registers plus the engine pinned to the
// case's Java runtime id at the epoch start time.
func startContextSelectionFAFNestedCase(caseName string, trace *compat.Trace) (*contextSelectionFAFNestedCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextSelectionFAFNestedBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextSelectionFAFNestedS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextSelectionFAFNestedS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	state := &contextSelectionFAFNestedCaseState{
		env:         env,
		deployments: map[string]*esper.Deployment{},
		statements:  map[string]*esper.Statement{},
		sequences:   map[string]uint64{},
		trace:       trace,
		caseName:    caseName,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextSelectionFAFNestedJavaRuntimeIDs[contextSelectionFAFNestedCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextSelectionFAFNestedCaseOrdinal(caseName string) int {
	for index, name := range contextSelectionFAFNestedCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy executes one deploy step: the create-context EPL registers the
// overlapping initiated ACtx parent and the three-category BCtx child
// composed as NestedContext, the create-window EPL registers the
// context-bound keepall named window, and the insert EPL deploys the
// routing statement. The step EPL is verified against the pinned text
// before the label maps to its fixture.
func (s *contextSelectionFAFNestedCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	pinned, ok := contextSelectionFAFNestedDeployEPLs[s.caseName][label]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", contextSelectionFAFNestedID, label, step.Epl)
	}
	beanSource := esper.From[contextSelectionFAFNestedBean](s.env, "SupportBean")
	intPrimitive := esper.Field[contextSelectionFAFNestedBean, int]("intPrimitive")
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isS1 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
	createWindow := func(name, eventType, contextName string) error {
		schema, ok := s.env.Schema(eventType)
		if !ok {
			return fmt.Errorf("%s: %s schema is not registered", contextSelectionFAFNestedID, eventType)
		}
		options := []esper.NamedWindowOption{esper.NamedWindowRetention(esper.KeepAll())}
		if contextName != "" {
			options = append([]esper.NamedWindowOption{esper.NamedWindowContext(contextName)}, options...)
		}
		_, err := esper.CreateNamedWindow(s.env, name, schema, options...)
		return err
	}
	switch s.caseName {
	case "nested-named-window-query":
		switch label {
		case "ctx":
			// `context ACtx initiated by SupportBean_S0 as s0 terminated by
			// SupportBean_S1(id=s0.id)` — an overlapping initiated-terminated
			// parent: every S0 opens a fresh concurrent partition and the
			// correlated S1 ends it.
			parent, err := esper.CreateOverlappingInitiatedTerminatedContext(s.env, "ACtx",
				esper.Literal("global"),
				isS0,
				esper.And(isS1,
					esper.Equal[int](esper.Field[contextSelectionFAFNestedS1, int]("id"),
						esper.Property[int](esper.ContextInitiatingEvent(), "id"))))
			if err != nil {
				return err
			}
			// `context BCtx group by intPrimitive < 0 as grp1, group by
			// intPrimitive = 0 as grp2, group by intPrimitive > 0 as grp3
			// from SupportBean` — the category child level.
			child, err := esper.NewCategoryContext("BCtx",
				esper.Category("grp1", esper.Less[int](intPrimitive, esper.Literal(0))),
				esper.Category("grp2", esper.Equal[int](intPrimitive, esper.Literal(0))),
				esper.Category("grp3", esper.Greater[int](intPrimitive, esper.Literal(0))),
			)
			if err != nil {
				return err
			}
			_, err = esper.CreateNestedContext(s.env, "NestedContext", parent.Name(), child)
			return err
		case "win":
			return createWindow("MyWindow", "SupportBean", "NestedContext")
		case "insert":
			// `insert into MyWindow select * from SupportBean` — the routing
			// statement carries no context clause; the window's context
			// routes each event to its leaf partition like the Java
			// insert-into.
			plan, err := s.env.Build(beanSource.InsertInto("MyWindow"))
			if err != nil {
				return err
			}
			_, err = s.deployPlan(label, plan)
			return err
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextSelectionFAFNestedID, s.caseName, label)
}

// deployPlan deploys one built plan under the step label and records the
// deployment for undeploy-all.
func (s *contextSelectionFAFNestedCaseState) deployPlan(label string, plan esper.Plan) (*esper.Deployment, error) {
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return nil, fmt.Errorf("%s: deploy %q: %w", contextSelectionFAFNestedID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
	}
	return deployment, nil
}

// snapshot emits one snapshot record mirroring the Java execution's
// assertPropsPerRowIterator call: the statement's default iterator rows in
// partition-allocation order plus the statement's partition descriptors.
func (s *contextSelectionFAFNestedCaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed", contextSelectionFAFNestedID, step.Statement)
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
		Partitions: contextSelectionFAFNestedPartitions(statement.ContextPartitions()),
	})
	return nil
}

// snapshotSelector replays one selector-targeted step. When the step
// carries a context name in "name" the read is context-level, mirroring
// EPContextPartitionService.getContextPartitions over the nested context's
// leaf partitions; otherwise the step mirrors the Java execution's
// statement.iterator(selector) calls. Selector "all" maps to
// ContextPartitionSelectorAll, "ids" to SelectContextPartitionIDs over the
// step's id array, "nested" to SelectNestedContextPartitions over the
// pinned per-level all selectors, "segmented" to
// SelectContextPartitionSegments over the payload key tuples, "hashes" to
// SelectContextPartitionHashes over the id array, and "filtered" with
// filterProperty "ids" to SelectContextPartitionIDs over the payload id
// set. The segmented/hashes/filtered kinds are invalid over nested
// contexts and drive the selector-error records.
func (s *contextSelectionFAFNestedCaseState) snapshotSelector(ctx context.Context, step compat.Step) error {
	selector, err := contextSelectionFAFNestedSelector(step)
	if err != nil {
		return err
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot-selector",
		Statement: step.Statement,
		Name:      step.Name,
		Time:      compat.FormatTraceTime(s.engine.Now()),
	}
	if step.Name != "" {
		// Context-level read: the leaf descriptors of the nested context in
		// partition-id order.
		descriptors, err := s.engine.ContextPartitionDescriptors(step.Name, selector)
		if err != nil {
			return s.selectorError(step, err)
		}
		record.Partitions = contextSelectionFAFNestedPartitions(descriptors)
		s.trace.Records = append(s.trace.Records, record)
		return nil
	}
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot-selector statement %q was not deployed", contextSelectionFAFNestedID, step.Statement)
	}
	result, err := statement.SnapshotWithSelector(ctx, selector)
	if err != nil {
		return s.selectorError(step, err)
	}
	record.New = compat.NormalizeResults(result.Results())
	record.Partitions = contextSelectionFAFNestedPartitions(statement.ContextPartitionsWith(selector))
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// selectorError verifies the Go rejection is the InvalidRule boundary and
// records the pinned Java message prefix, mirroring the oracle's
// InvalidContextPartitionSelector catch.
func (s *contextSelectionFAFNestedCaseState) selectorError(step compat.Step, err error) error {
	if step.ExpectError == "" {
		return err
	}
	var espErr *esper.Error
	if !errors.As(err, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
		!strings.Contains(espErr.Message, "Invalid context partition selector") {
		return fmt.Errorf("%s: selector-error drift for %q: got %v", contextSelectionFAFNestedID, step.Selector, err)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "selector-error",
		Statement: step.Statement,
		Name:      step.Name,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value:     step.ExpectError,
	})
	return nil
}

// contextSelectionFAFNestedSelector maps one step's selector encoding to
// the Go selector instance, mirroring the oracle's selectorFor helper.
func contextSelectionFAFNestedSelector(step compat.Step) (esper.ContextPartitionSelector, error) {
	switch step.Selector {
	case "all":
		return esper.ContextPartitionSelectorAll{}, nil
	case "ids":
		return esper.SelectContextPartitionIDs(step.IDs...), nil
	case "nested":
		return esper.SelectNestedContextPartitions(
			esper.ContextPartitionSelectorAll{},
			esper.ContextPartitionSelectorAll{},
		), nil
	case "segmented":
		var keys [][]any
		if len(step.Payload) > 0 && string(step.Payload) != "null" {
			if err := json.Unmarshal(step.Payload, &keys); err != nil {
				return nil, fmt.Errorf("%s: segmented selector payload: %w", contextSelectionFAFNestedID, err)
			}
		}
		return esper.SelectContextPartitionSegments(keys...), nil
	case "hashes":
		hashes := step.Hashes
		if len(hashes) == 0 {
			for _, id := range step.IDs {
				hashes = append(hashes, int64(id))
			}
		}
		return esper.SelectContextPartitionHashes(hashes...), nil
	case "filtered":
		switch step.FilterProperty {
		case "ids":
			var ids []int
			if len(step.Payload) > 0 && string(step.Payload) != "null" {
				if err := json.Unmarshal(step.Payload, &ids); err != nil {
					return nil, fmt.Errorf("%s: selector ids payload: %w", contextSelectionFAFNestedID, err)
				}
			}
			return esper.SelectContextPartitionIDs(ids...), nil
		case "nil", "non-context":
			// The nil-selector and non-context probes pass a nil selector;
			// the statement decides which rejection fires.
			return nil, nil
		default:
			return nil, fmt.Errorf("%s: filtered selector property %q is not pinned", contextSelectionFAFNestedID, step.FilterProperty)
		}
	default:
		return nil, fmt.Errorf("%s: unsupported selector %q", contextSelectionFAFNestedID, step.Selector)
	}
}

// faf replays one fire-and-forget step, mirroring the Java execution's
// runQuery/runQueryAll helpers: the pinned plan executes through
// ExecuteFireAndForgetWithSelector for the mapped selector; "all" steps
// additionally run the no-selector ExecuteFireAndForget like runQueryAll
// and assert the same rows. Any selector kind rejected with the
// InvalidRule boundary records the pinned Java message prefix as a
// selector-error. Rows sort by their compact field rendering because the
// Java assertions are any-order.
func (s *contextSelectionFAFNestedCaseState) faf(ctx context.Context, step compat.Step) error {
	label := step.Statement
	pinned, ok := contextSelectionFAFNestedFafEPLs[label]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: faf %q carries an unpinned EPL %q", contextSelectionFAFNestedID, label, step.Epl)
	}
	selector, err := contextSelectionFAFNestedSelector(step)
	if err != nil {
		return err
	}
	plan, err := s.buildFafPlan(label)
	if err != nil {
		return err
	}
	result, err := s.engine.ExecuteFireAndForgetWithSelector(ctx, plan, selector)
	if err != nil {
		if step.ExpectError == "" {
			return fmt.Errorf("%s: faf %q: %w", contextSelectionFAFNestedID, label, err)
		}
		var espErr *esper.Error
		if !errors.As(err, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
			!strings.Contains(espErr.Message, "Invalid context partition selector") {
			return fmt.Errorf("%s: faf %q selector-error drift: got %v", contextSelectionFAFNestedID, label, err)
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
	rows := contextSelectionFAFNestedSortedRows(compat.NormalizeResults(result.Results()))
	if step.Selector == "all" {
		// runQueryAll also runs the same query without a selector; the Go
		// no-selector form must return the same rows.
		plain, err := s.engine.ExecuteFireAndForget(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: faf %q no-selector: %w", contextSelectionFAFNestedID, label, err)
		}
		plainRows := contextSelectionFAFNestedSortedRows(compat.NormalizeResults(plain.Results()))
		if !reflect.DeepEqual(rows, plainRows) {
			return fmt.Errorf("%s: faf %q no-selector rows drift: got %v want %v", contextSelectionFAFNestedID, label, plainRows, rows)
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

// buildFafPlan maps one faf step label to the pinned Go plan: the group-by
// merge queries read the named window directly, the context-clause query
// carries WithContext("NestedContext") and projects the parent level's
// initiating-event p00 plus the leaf category label, and the
// invalid-selector probe is the wildcard context-clause read.
func (s *contextSelectionFAFNestedCaseState) buildFafPlan(label string) (esper.Plan, error) {
	window := esper.FromNamedWindow(s.env, "MyWindow")
	theString := esper.Field[any, string]("theString")
	intPrimitive := esper.Field[any, int]("intPrimitive")
	var query esper.Query
	switch label {
	case "sum-all", "sum-byid":
		query = window.GroupBy(theString).Select(
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query()
	case "context-byid":
		// context.ACtx.s0.p00 reads the parent level's initiating S0 event;
		// context.BCtx.label reads the leaf's category label.
		query = window.GroupBy(theString).Select(
			esper.Alias("c1", esper.Property[string](
				esper.ContextField[esper.Event]("parent.initiating_event"), "p00")),
			esper.Alias("c2", esper.ContextField[string]("label")),
			esper.Alias("c3", theString),
			esper.Alias("c4", esper.Sum[int](intPrimitive)),
		).Query(esper.WithContext("NestedContext"))
	case "invalid-selector":
		query = window.Query(esper.WithContext("NestedContext"))
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown faf statement %q", contextSelectionFAFNestedID, label)
	}
	plan, err := s.env.Build(query)
	if err != nil {
		return esper.Plan{}, fmt.Errorf("%s: faf %q build: %w", contextSelectionFAFNestedID, label, err)
	}
	return plan, nil
}

// buildError runs one expected-invalid compile probe against the fluent
// equivalent of the pinned EPL. The nested case pins no compile probes, so
// any compile-error/build-error step fails the pinned-EPL check.
func (s *contextSelectionFAFNestedCaseState) buildError(step compat.Step) error {
	pinned, ok := contextSelectionFAFNestedProbeEPLs[step.Statement]
	if !ok || step.Epl != pinned {
		return fmt.Errorf("%s: compile-error probe %q carries an unpinned EPL %q", contextSelectionFAFNestedID, step.Statement, step.Epl)
	}
	return fmt.Errorf("%s: unknown compile-error probe %q", contextSelectionFAFNestedID, step.Statement)
}

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the modules they reference, mirroring undeployAll. The
// env-scoped context and window registrations retire with the engine.
func (s *contextSelectionFAFNestedCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextSelectionFAFNestedID, label, err)
		}
		delete(s.deployments, label)
		for _, statement := range deployment.Statements() {
			delete(s.statements, statement.Name())
		}
	}
	s.deployOrder = nil
	return nil
}

// decodeContextSelectionFAFNestedPayload converts a send payload into the
// typed bean for the step's event type.
func decodeContextSelectionFAFNestedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextSelectionFAFNestedBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean: %w", contextSelectionFAFNestedID, err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextSelectionFAFNestedS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S0: %w", contextSelectionFAFNestedID, err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextSelectionFAFNestedS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S1: %w", contextSelectionFAFNestedID, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", contextSelectionFAFNestedID, step.EventType)
	}
}

// contextSelectionFAFNestedPartitions renders the leaf partition
// descriptors the Java oracle emits for nested contexts: one record per
// leaf carrying its id, a composite "nested:<parent>/<child>" key, and flat
// properties (startTime/endTime and initiating.p00 from the initiated
// parent level, label from the category child level, hash from a hash
// child level, key:<value> from a partitioned level). Non-nested
// descriptors render their own level like the shared normalizer.
func contextSelectionFAFNestedPartitions(descriptors []esper.ContextPartitionDescriptor) []compat.PartitionRecord {
	if len(descriptors) == 0 {
		return []compat.PartitionRecord{}
	}
	result := make([]compat.PartitionRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		properties := make(map[string]any)
		parentPart := ""
		childPart := ""
		if start, ok := descriptor.Property("parent.startTime"); ok && start.IsPresent() {
			if instant, ok := start.Any().(time.Time); ok {
				millis := instant.UnixMilli()
				properties["startTime"] = millis
				parentPart = fmt.Sprintf("start:%d", millis)
			}
		}
		if parentPart != "" {
			// Active initiated parents expose endTime as null, matching the
			// oracle's ContextPartitionIdentifierInitiatedTerminated shape.
			properties["endTime"] = nil
			if end, ok := descriptor.Property("parent.endTime"); ok && end.IsPresent() {
				if instant, ok := end.Any().(time.Time); ok {
					properties["endTime"] = instant.UnixMilli()
				}
			}
		}
		if initiating, ok := descriptor.Property("parent.initiating_event"); ok && initiating.IsPresent() {
			if event, ok := initiating.Any().(esper.Event); ok {
				properties["initiating.p00"] = contextSelectionFAFNestedScalar(event.Get("p00"))
			}
		}
		if label, ok := descriptor.Property("label"); ok && label.IsPresent() {
			if text, ok := label.Any().(string); ok {
				properties["label"] = text
				childPart = "category:" + text
			}
		}
		if childPart == "" {
			if hash, ok := descriptor.Property("hash"); ok && hash.IsPresent() {
				properties["hash"] = contextSelectionFAFNestedScalar(hash)
				childPart = fmt.Sprintf("hash:%v", hash.Any())
			}
		}
		if childPart == "" {
			if key, ok := descriptor.Property("key1"); ok && key.IsPresent() {
				childPart = fmt.Sprintf("key:%v", key.Any())
			}
		}
		key := descriptor.Key
		if parentPart != "" || childPart != "" {
			key = "nested:" + parentPart + "/" + childPart
		}
		result = append(result, compat.PartitionRecord{ID: descriptor.ID, Key: key, Properties: properties})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ID != result[j].ID {
			return result[i].ID < result[j].ID
		}
		return result[i].Key < result[j].Key
	})
	return result
}

// contextSelectionFAFNestedScalar renders one descriptor property value
// with the same tagged-null convention the shared normalizer uses for row
// fields.
func contextSelectionFAFNestedScalar(value esper.Value) any {
	if value.IsMissing() {
		return map[string]any{"state": "missing"}
	}
	if value.IsNull() {
		return map[string]any{"state": "null"}
	}
	return value.Any()
}

// contextSelectionFAFNestedSortedRows canonicalizes row order for the Java
// assertPropsPerRowAnyOrder assertions: rows sort by their compact JSON
// field rendering so both traces pin the same order.
func contextSelectionFAFNestedSortedRows(rows []compat.ResultRecord) []compat.ResultRecord {
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

// loadContextSelectionFAFNestedScenario decodes the scenario with the
// strict pinned checks: no duplicate JSON keys, the exact top-level field
// set, pinned case metadata, and the complete pinned step sequence per
// case.
func loadContextSelectionFAFNestedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextSelectionFAFNestedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextSelectionFAFNestedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextSelectionFAFNestedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextSelectionFAFNestedID, err)
	}
	if err := requireContextSelectionFAFNestedFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextSelectionFAFNestedID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextSelectionFAFNestedID ||
		metadata.Description != contextSelectionFAFNestedDescription ||
		metadata.JavaCommit != contextSelectionFAFNestedJavaCommit ||
		metadata.JavaSource != contextSelectionFAFNestedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextSelectionFAFNestedID)
	}
	if err := validateContextSelectionFAFNestedStringArray(root["javaRuntimes"], contextSelectionFAFNestedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextSelectionFAFNestedStringArray(root["javaNames"], contextSelectionFAFNestedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextSelectionFAFNestedStringArray(root["javaStaticIds"], contextSelectionFAFNestedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextSelectionFAFNestedStringArray(root["javaFlags"], contextSelectionFAFNestedJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextSelectionFAFNestedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextSelectionFAFNestedID, len(contextSelectionFAFNestedCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextSelectionFAFNestedFields(object,
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
		if definition.Case != contextSelectionFAFNestedCases[index] ||
			definition.Ordinal != contextSelectionFAFNestedOrdinals[index] ||
			definition.RuntimeID != contextSelectionFAFNestedJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextSelectionFAFNestedJavaExecutions[index] ||
			definition.Observation != contextSelectionFAFNestedCaseObservations[index] ||
			definition.EPL != contextSelectionFAFNestedCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextSelectionFAFNestedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextSelectionFAFNestedID, err)
	}
	offset := 0
	for _, caseName := range contextSelectionFAFNestedCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextSelectionFAFNestedID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextSelectionFAFNestedID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextSelectionFAFNestedID, offset, caseName)
		}
		offset++
		want, ok := contextSelectionFAFNestedCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextSelectionFAFNestedID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextSelectionFAFNestedID, caseName)
		}
		for _, pinned := range want {
			key, err := contextSelectionFAFNestedStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextSelectionFAFNestedID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextSelectionFAFNestedID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextSelectionFAFNestedID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextSelectionFAFNestedID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// contextSelectionFAFNestedStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|
// mode|ids|selector|filterProperty|filterValue|at with the payload and ids
// compacted. Unknown fields on the step object are rejected.
func contextSelectionFAFNestedStepKey(raw json.RawMessage) (string, error) {
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
		At                 string          `json:"at"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "mode": true, "ids": true,
		"selector": true, "filterProperty": true, "filterValue": true, "at": true,
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
		"|" + step.FilterValue + "|" + step.At, nil
}

// validateContextSelectionFAFNestedScenario re-checks a decoded scenario
// (used when the runner receives a scenario decoded by the generic loader
// path).
func validateContextSelectionFAFNestedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != contextSelectionFAFNestedID {
		return fmt.Errorf("%s scenario id %q is not pinned", contextSelectionFAFNestedID, scenario.ID)
	}
	return nil
}

func requireContextSelectionFAFNestedFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextSelectionFAFNestedID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextSelectionFAFNestedID, name)
		}
	}
	return nil
}

func validateContextSelectionFAFNestedStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
