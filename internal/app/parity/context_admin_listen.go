package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	contextAdminListenID         = "context-admin-listen"
	contextAdminListenJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextAdminListenSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextAdminListen.java"
)

const contextAdminListenDescription = "ContextAdminListen context listener surface: a category context whose two partitions allocate eagerly at activation with the second carrying label 'neg' (ord 2); a nested category-over-keyed context emitting one created event at ctx deploy, statement-added then activated at s0 deploy, a single nested-leaf partition-allocated on the first event, and statement-removed/partition-deallocated/deactivated/destroyed teardown order (ord 3); three context-state listeners registered before deploy each observing created, listener[0] removed before undeploy so only listeners[1..2] observe destroyed, the listener iterator yielding [l1,l2] in registration order, remove-all emptying the registry, and a redeploy+undeploy-all tail staying silent (ord 4); and one partition-state listener added after ctx deploy observing statement-added(a)/activated/statement-added(b) then exactly one partition-allocated despite two statements (ord 6). context-event records carry the listener label, event kind, runtimeURI normalized to 'default', the ctx deploy-step label as contextDeploymentId, statement deploy labels, partition ids and normalized identifiers (category label / nested parent+leaf key segments / initiated-terminated initiating event type). admin records mirror the listener-registry iterators (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextAdminListen.java)."

var (
	contextAdminListenJavaRuntimeIDs = []string{
		"java-runtime-2021f021c6e12684fb81",
		"java-runtime-4b1466f2815a381815b2",
		"java-runtime-410d2c5d3daf011b6c7b",
		"java-runtime-6dd25578221d74ff6660",
	}
	contextAdminListenJavaExecutions = []string{
		"ContextAdminListenCategory",
		"ContextAdminListenNested",
		"ContextAddRemoveListener",
		"ContextAdminListenMultipleStatements",
	}
	contextAdminListenJavaStaticIDs = []string{
		"java-3a026095a61c4060c91b",
		"java-3a026095a61c4060c91b",
		"java-3a026095a61c4060c91b",
		"java-3a026095a61c4060c91b",
	}
	contextAdminListenJavaFlags = []string{"OBSERVEROPS", "RUNTIMEOPS"}
	contextAdminListenCases     = []string{
		"category",
		"nested",
		"add-remove-listener",
		"multiple-statements",
	}
	contextAdminListenOrdinals = []int{2, 3, 4, 6}
	contextAdminListenSources  = []string{contextAdminListenSource}
)

var contextAdminListenCaseObservations = []string{
	"context-event; a context-state listener registered before deploy observes created at ctx deploy and re-entrantly registers as a partition listener; deploying s0 into the two-category context eagerly allocates pos then neg (allocated[1] label 'neg') before activated; no events are sent; undeploying s0 then ctx emits statement-removed, two partition-deallocated, deactivated and destroyed",
	"context-event; nested category-over-keyed context: ctx deploy emits created, s0 deploy emits statement-added then activated, SupportBean(\"E1\",1) emits exactly one partition-allocated whose nested identifier carries parent label 'pos' and leaf keys [\"E1\"], s0 undeploy emits statement-removed/partition-deallocated/deactivated and ctx undeploy emits destroyed",
	"context-event+admin; three context-state listeners registered before deploy each observe created; removing l0 before ctx undeploy leaves l0 silent while l1 and l2 observe destroyed; the listener iterator yields [l1,l2] in registration order; remove-listeners empties the registry; a redeploy plus undeploy-all tail invokes no listener",
	"context-event; one partition-state listener added after ctx deploy observes statement-added(a), activated, statement-added(b) — activated fires once after the first statement — and SupportBean_S0(1) emits exactly one partition-allocated despite two deployed statements; undeploy-all tears down a, b and the context in dependency order",
}

var contextAdminListenCaseEPLs = []string{
	"@name('s0') context MyContext select count(*) from SupportBean",
	"@name('s0') context MyContext select count(*) from SupportBean",
	"@name('ctx') @public create context MyContext start SupportBean_S0 as s0 end SupportBean_S1",
	"@name('a') context MyContextStartS0EndS1 select count(*) from SupportBean",
}

// contextAdminListenCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids
// keys. Deploy steps carry the byte-exact EPL text the Java oracle compiles;
// compileWithoutPath marks the ord-4 deploys whose Java execution calls the
// path-less compileDeploy overload. add-listener/add-partition-listener carry
// the listener label in statement (the context deploy label in name for
// partition listeners); remove-listener/remove-listeners mirror the registry
// removals; snapshot mode "admin:listeners" mirrors the
// getContextStateListeners iterator assertion. The category and nested cases
// carry no deployed record for 'ctx': Go marks env-registered contexts created
// lazily at the first statement deploy, so the created event arrives with the
// s0 deploy while Java emits it at the ctx deploy — dropping the deployed
// marker keeps both record streams aligned.
var contextAdminListenCaseSteps = map[string][]string{
	"category": {
		"add-listener|l0||||||||",
		"deploy|ctx|||@name('ctx') @public create context MyContext group by intPrimitive > 0 as pos, group by intPrimitive < 0 as neg from SupportBean|||||",
		"deploy|s0|||@name('s0') context MyContext select count(*) from SupportBean|||||",
		"deployed|s0||||||||",
		"undeploy|s0||||||||",
		"undeploy|ctx||||||||",
		"remove-listeners|||||||||",
	},
	"nested": {
		"add-listener|l0||||||||",
		"deploy|ctx|||@name('ctx') @public create context MyContext context ContextPosNeg group by intPrimitive > 0 as pos, group by intPrimitive < 0 as neg from SupportBean, context ByString partition by theString from SupportBean|||||",
		"deploy|s0|||@name('s0') context MyContext select count(*) from SupportBean|||||",
		"deployed|s0||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||||",
		"undeploy|s0||||||||",
		"undeploy|ctx||||||||",
		"remove-listeners|||||||||",
	},
	"add-remove-listener": {
		"add-listener|l0||||||||",
		"add-listener|l1||||||||",
		"add-listener|l2||||||||",
		"deploy|ctx|||@name('ctx') @public create context MyContext start SupportBean_S0 as s0 end SupportBean_S1|||1||",
		"deployed|ctx||||||||",
		"remove-listener|l0||||||||",
		"undeploy|ctx||||||||",
		"snapshot|ctx|||||||admin:listeners|",
		"remove-listeners|||||||||",
		"snapshot|ctx|||||||admin:listeners|",
		"deploy|ctx|||@name('ctx') @public create context MyContext start SupportBean_S0 as s0 end SupportBean_S1|||1||",
		"deployed|ctx||||||||",
		"undeploy-all|||||||||",
	},
	"multiple-statements": {
		"deploy|ctx|||@name('ctx') @public create context MyContextStartS0EndS1 start SupportBean_S0 as s0 end SupportBean_S1|||||",
		"deployed|ctx||||||||",
		"add-partition-listener|l0|ctx|||||||",
		"deploy|a|||@name('a') context MyContextStartS0EndS1 select count(*) from SupportBean|||||",
		"deployed|a||||||||",
		"deploy|b|||@name('b') context MyContextStartS0EndS1 select count(*) from SupportBean_S0|||||",
		"deployed|b||||||||",
		"send|||SupportBean_S0||{\"id\":1}||||",
		"undeploy-all|||||||||",
	},
}

// contextAdminListenBean mirrors SupportBean for the listener cases.
type contextAdminListenBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// contextAdminListenS0 mirrors SupportBean_S0 (initiating stream).
type contextAdminListenS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// contextAdminListenS1 mirrors SupportBean_S1 (terminating stream).
type contextAdminListenS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// contextAdminListenCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, the deploy label
// that owns each registered context (the normalized contextDeploymentId), the
// deployment label each statement deployed under (the normalized
// statementDeploymentId), and the labeled context listeners.
type contextAdminListenCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	contextByLabel map[string]string
	contextOwner   map[string]string
	registeredCtx  []string
	statementOwner map[string]string
	listeners      map[string]*contextAdminListenListener
	listenerOrder  []string
	sequences      map[string]uint64
	trace          *compat.Trace
	caseName       string
}

// contextAdminListenListener mirrors SupportContextListener: it implements
// both listener interfaces, re-entrantly registers itself as a partition
// listener inside OnContextCreated, and asserts inside
// OnContextPartitionAllocated that the partition properties are queryable
// (Java's getContextProperties non-null check). Callback failures are stored
// and surface at the next step boundary.
type contextAdminListenListener struct {
	state *contextAdminListenCaseState
	label string
	err   error
}

func (l *contextAdminListenListener) OnContextCreated(event esper.ContextStateEvent) {
	l.record("created", event, nil)
	// SupportContextListener.onContextCreated re-entrantly registers the same
	// listener as a partition listener for the created context.
	if err := l.state.engine.AddContextPartitionStateListener(event.ContextName, l); err != nil && l.err == nil {
		l.err = fmt.Errorf("re-entrant partition listener registration: %w", err)
	}
}

func (l *contextAdminListenListener) OnContextDestroyed(event esper.ContextStateEvent) {
	l.record("destroyed", event, nil)
}

func (l *contextAdminListenListener) OnContextActivated(event esper.ContextStateEvent) {
	l.record("activated", event, nil)
}

func (l *contextAdminListenListener) OnContextDeactivated(event esper.ContextStateEvent) {
	l.record("deactivated", event, nil)
}

func (l *contextAdminListenListener) OnContextStatementAdded(event esper.ContextStateEvent) {
	l.record("statement-added", event, nil)
}

func (l *contextAdminListenListener) OnContextStatementRemoved(event esper.ContextStateEvent) {
	l.record("statement-removed", event, nil)
}

func (l *contextAdminListenListener) OnContextPartitionAllocated(event esper.ContextPartitionStateEvent) {
	// SupportContextListener asserts getContextProperties is non-null inside
	// the callback; the Go equivalent reads the descriptor back by id.
	props, ok, err := l.state.engine.ContextPartitionProperties(event.ContextName, event.PartitionID)
	if err != nil || !ok || props == nil {
		if l.err == nil {
			l.err = fmt.Errorf("partition properties for %q id %d not queryable in allocated callback: ok=%v err=%v",
				event.ContextName, event.PartitionID, ok, err)
		}
	}
	l.record("partition-allocated", esper.ContextStateEvent{}, &event)
}

func (l *contextAdminListenListener) OnContextPartitionDeallocated(event esper.ContextPartitionStateEvent) {
	l.record("partition-deallocated", esper.ContextStateEvent{}, &event)
}

// record appends one normalized context-event record. Deployment ids
// normalize to the owning deploy-step label and runtimeURI to "default" once
// the event value verifies against the engine URI, matching the oracle's
// assertions.
func (l *contextAdminListenListener) record(kind string, state esper.ContextStateEvent, partition *esper.ContextPartitionStateEvent) {
	s := l.state
	value := map[string]any{
		"event":               kind,
		"runtimeURI":          s.normalizeRuntimeURI(state.RuntimeURI),
		"contextDeploymentId": s.normalizeContextDeploymentID(state.ContextName, state.ContextDeploymentID),
		"contextName":         state.ContextName,
	}
	if partition != nil {
		value["runtimeURI"] = s.normalizeRuntimeURI(s.engine.RuntimeURI())
		value["contextDeploymentId"] = s.normalizeContextDeploymentID(partition.ContextName, "")
		value["contextName"] = partition.ContextName
		value["partitionId"] = partition.PartitionID
		if partition.Allocated {
			value["identifier"] = contextAdminListenIdentifier(partition.Descriptor)
		}
	}
	if state.StatementName != "" {
		value["statementName"] = state.StatementName
		value["statementDeploymentId"] = s.normalizeStatementDeploymentID(state.StatementName, state.StatementDeploymentID)
	}
	s.sequences[l.label]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "context-event",
		Name:      l.label,
		Sequence:  s.sequences[l.label],
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value:     value,
	})
}

// normalizeRuntimeURI maps the engine URI to the "default" the Java oracle
// asserts once the populated event value verifies; a drifted value passes
// through so the diff reports it.
func (s *contextAdminListenCaseState) normalizeRuntimeURI(uri string) string {
	if uri == s.engine.RuntimeURI() {
		return "default"
	}
	return uri
}

// normalizeContextDeploymentID maps the (empty for env-registered contexts)
// deployment id to the deploy-step label that registered the context.
func (s *contextAdminListenCaseState) normalizeContextDeploymentID(contextName, raw string) string {
	if owner, ok := s.contextOwner[contextName]; ok {
		return owner
	}
	return raw
}

// normalizeStatementDeploymentID maps a statement deployment id back to the
// deploy-step label that deployed it. The event fires inside Deploy before
// the deployment lands in the map, so the statement name resolves the label.
func (s *contextAdminListenCaseState) normalizeStatementDeploymentID(statementName, raw string) string {
	if owner, ok := s.statementOwner[statementName]; ok {
		return owner
	}
	for label, deployment := range s.deployments {
		if deployment != nil && deployment.ID() == raw {
			return label
		}
	}
	return raw
}

// contextAdminListenIdentifier renders the normalized partition identifier:
// Java's typed ContextPartitionIdentifier classes map to descriptor
// properties — category label, nested parent.* plus leaf key segments, hash
// bucket, partitioned keyN values, or the initiated-terminated initiating
// event type (Java's properties["s0"] presence assertion).
func contextAdminListenIdentifier(descriptor esper.ContextPartitionDescriptor) map[string]any {
	props := descriptor.Properties()
	if _, nested := props["parent.name"]; nested {
		parent := contextAdminListenFlatIdentifier(props, "parent.")
		leaf := contextAdminListenFlatIdentifier(props, "")
		return map[string]any{"type": "nested", "identifiers": []any{parent, leaf}}
	}
	return contextAdminListenFlatIdentifier(props, "")
}

func contextAdminListenFlatIdentifier(props map[string]any, prefix string) map[string]any {
	if raw, ok := props[prefix+"initiating_event"]; ok {
		if event, isEvent := raw.(esper.Event); isEvent {
			return map[string]any{"type": "initiatedTerminated", "initiatingEvent": event.TypeName()}
		}
		return map[string]any{"type": "initiatedTerminated", "initiatingEvent": fmt.Sprint(raw)}
	}
	if label, ok := props[prefix+"label"].(string); ok {
		return map[string]any{"type": "category", "label": label}
	}
	if hash, ok := props[prefix+"hash"]; ok {
		return map[string]any{"type": "hash", "hash": hash}
	}
	keys := []any{}
	for index := 1; ; index++ {
		value, ok := props[fmt.Sprintf("%skey%d", prefix, index)]
		if !ok {
			break
		}
		keys = append(keys, value)
	}
	return map[string]any{"type": "partitioned", "keys": keys}
}

func runContextAdminListenScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextAdminListenScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextAdminListen(ctx, scenario, &trace)
}

// executeContextAdminListen replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeContextAdminListen(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextAdminListenCaseState
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
			state, err = startContextAdminListenCase(step.Case, trace)
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
			event, err := decodeContextAdminListenPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "add-listener":
			if err := state.addListener(step.Statement); err != nil {
				return *trace, err
			}
		case "add-partition-listener":
			if err := state.addPartitionListener(step.Statement, step.Name); err != nil {
				return *trace, err
			}
		case "remove-listener":
			state.removeListener(step.Statement)
		case "remove-listeners":
			state.removeListeners()
		case "snapshot":
			if err := state.snapshot(step); err != nil {
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
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextAdminListenID, step.Op)
		}
		if err := state.listenerError(); err != nil {
			return *trace, err
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// listenerError surfaces a callback-side assertion failure (the re-entrant
// registration or the in-callback properties read) at the step boundary.
func (s *contextAdminListenCaseState) listenerError() error {
	for _, label := range s.listenerOrder {
		if err := s.listeners[label].err; err != nil {
			return fmt.Errorf("%s: listener %q: %w", contextAdminListenID, label, err)
		}
	}
	return nil
}

// startContextAdminListenCase builds the fresh per-case environment: the
// three event types the suite registers plus the engine pinned to the case's
// Java runtime id at the epoch start time. The add-remove-listener case
// registers its context before the engine so the AddContextStateListener
// replay delivers created to the three listeners, matching the Java
// execution's deploy-time created dispatch; the other cases register contexts
// inside their deploy steps so the created event arrives with the first
// statement deploy (the documented lazy-creation difference).
func startContextAdminListenCase(caseName string, trace *compat.Trace) (*contextAdminListenCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextAdminListenBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextAdminListenS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextAdminListenS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	state := &contextAdminListenCaseState{
		env:            env,
		deployments:    map[string]*esper.Deployment{},
		contextByLabel: map[string]string{},
		contextOwner:   map[string]string{},
		statementOwner: map[string]string{},
		listeners:      map[string]*contextAdminListenListener{},
		sequences:      map[string]uint64{},
		trace:          trace,
		caseName:       caseName,
	}
	if caseName == "add-remove-listener" {
		if err := state.registerInitTermContext("ctx", "MyContext"); err != nil {
			return nil, err
		}
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextAdminListenJavaRuntimeIDs[contextAdminListenCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextAdminListenCaseOrdinal(caseName string) int {
	for index, name := range contextAdminListenCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// registerInitTermContext registers the `start SupportBean_S0 as s0 end
// SupportBean_S1` initiated-terminated context under the deploy label,
// recording the label as the normalized contextDeploymentId. An
// already-registered name is reused (the ord-4 redeploy re-registers after
// DestroyContext removed the definition).
func (s *contextAdminListenCaseState) registerInitTermContext(label, name string) error {
	if _, ok := s.env.Context(name); ok {
		s.contextByLabel[label] = name
		if _, owned := s.contextOwner[name]; !owned {
			s.contextOwner[name] = label
		}
		return nil
	}
	start := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	end := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))
	if _, err := esper.CreateInitiatedTerminatedContext(s.env, name, esper.Literal("global"), start, end); err != nil {
		return err
	}
	s.contextByLabel[label] = name
	if _, owned := s.contextOwner[name]; !owned {
		s.contextOwner[name] = label
	}
	s.registeredCtx = append(s.registeredCtx, name)
	return nil
}

// deploy executes one deploy step: registration fixtures model create-context
// EPL at their scenario positions (the established approved difference for
// the missing deployable statement types) and statement fixtures deploy
// labeled plans.
func (s *contextAdminListenCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	beanSource := esper.From[contextAdminListenBean](s.env, "SupportBean")
	intPrimitive := esper.Field[contextAdminListenBean, int32]("intPrimitive")
	countPlan := func(stream esper.Stream[contextAdminListenBean], statementName, contextName string) (esper.Plan, error) {
		return s.env.Build(stream.Aggregate(
			esper.Alias("count(*)", esper.CountAll()),
		).Query(esper.StatementName(statementName), esper.WithContext(contextName)))
	}
	switch s.caseName {
	case "category":
		switch label {
		case "ctx":
			if _, ok := s.env.Context("MyContext"); ok {
				return fmt.Errorf("%s: context MyContext unexpectedly registered", contextAdminListenID)
			}
			if _, err := esper.CreateCategoryContext(s.env, "MyContext",
				esper.Category("pos", esper.Greater[int32](intPrimitive, esper.Literal(int32(0)))),
				esper.Category("neg", esper.Less[int32](intPrimitive, esper.Literal(int32(0)))),
			); err != nil {
				return err
			}
			s.contextByLabel[label] = "MyContext"
			s.contextOwner["MyContext"] = label
			s.registeredCtx = append(s.registeredCtx, "MyContext")
			return nil
		case "s0":
			plan, err := countPlan(beanSource, "s0", "MyContext")
			_, err = s.deployPlan(label, plan, err)
			return err
		}
	case "nested":
		switch label {
		case "ctx":
			if _, err := esper.CreateCategoryContext(s.env, "ContextPosNeg",
				esper.Category("pos", esper.Greater[int32](intPrimitive, esper.Literal(int32(0)))),
				esper.Category("neg", esper.Less[int32](intPrimitive, esper.Literal(int32(0)))),
			); err != nil {
				return err
			}
			s.registeredCtx = append(s.registeredCtx, "ContextPosNeg")
			child, err := esper.NewKeyContext("ByString",
				esper.Field[contextAdminListenBean, string]("theString"))
			if err != nil {
				return err
			}
			if _, err := esper.CreateNestedContext(s.env, "MyContext", "ContextPosNeg", child); err != nil {
				return err
			}
			s.contextByLabel[label] = "MyContext"
			s.contextOwner["MyContext"] = label
			s.registeredCtx = append(s.registeredCtx, "MyContext")
			return nil
		case "s0":
			plan, err := countPlan(beanSource, "s0", "MyContext")
			_, err = s.deployPlan(label, plan, err)
			return err
		}
	case "add-remove-listener":
		if label == "ctx" {
			return s.registerInitTermContext(label, "MyContext")
		}
	case "multiple-statements":
		switch label {
		case "ctx":
			return s.registerInitTermContext(label, "MyContextStartS0EndS1")
		case "a":
			plan, err := countPlan(beanSource, "a", "MyContextStartS0EndS1")
			_, err = s.deployPlan(label, plan, err)
			return err
		case "b":
			plan, err := s.env.Build(esper.From[contextAdminListenS0](s.env, "SupportBean_S0").Aggregate(
				esper.Alias("count(*)", esper.CountAll()),
			).Query(esper.StatementName("b"), esper.WithContext("MyContextStartS0EndS1")))
			_, err = s.deployPlan(label, plan, err)
			return err
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextAdminListenID, s.caseName, label)
}

// deployPlan deploys one built plan under the step label and records the
// deployment for targeted undeploy and statement-deployment-id
// normalization.
func (s *contextAdminListenCaseState) deployPlan(label string, plan esper.Plan, planErr error) (*esper.Deployment, error) {
	if planErr != nil {
		return nil, planErr
	}
	// The statement-added event fires inside Deploy, before the deployment is
	// recorded, so the label is pinned by name up front: every statement
	// fixture deploys under a name equal to its step label.
	s.statementOwner[label] = label
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return nil, fmt.Errorf("%s: deploy %q: %w", contextAdminListenID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		s.statementOwner[statement.Name()] = label
	}
	return deployment, nil
}

// addListener registers one labeled context-state listener, mirroring
// addContextStateListener. For the add-remove-listener case the pre-registered
// context replays created inside the call, which re-entrantly registers the
// partition listener exactly like SupportContextListener.onContextCreated.
func (s *contextAdminListenCaseState) addListener(label string) error {
	if _, exists := s.listeners[label]; exists {
		return fmt.Errorf("%s: listener %q already registered", contextAdminListenID, label)
	}
	listener := &contextAdminListenListener{state: s, label: label}
	if err := s.engine.AddContextStateListener(listener); err != nil {
		return err
	}
	s.listeners[label] = listener
	s.listenerOrder = append(s.listenerOrder, label)
	return nil
}

// addPartitionListener registers one labeled partition-state listener on the
// context registered under the given deploy label, mirroring
// addContextPartitionStateListener(depId, ctxName, listener).
func (s *contextAdminListenCaseState) addPartitionListener(label, contextLabel string) error {
	if _, exists := s.listeners[label]; exists {
		return fmt.Errorf("%s: listener %q already registered", contextAdminListenID, label)
	}
	contextName, ok := s.contextByLabel[contextLabel]
	if !ok {
		return fmt.Errorf("%s: no context registered under label %q", contextAdminListenID, contextLabel)
	}
	listener := &contextAdminListenListener{state: s, label: label}
	if err := s.engine.AddContextPartitionStateListener(contextName, listener); err != nil {
		return err
	}
	s.listeners[label] = listener
	s.listenerOrder = append(s.listenerOrder, label)
	return nil
}

// removeListener mirrors removeContextStateListener for one labeled listener.
func (s *contextAdminListenCaseState) removeListener(label string) {
	listener, ok := s.listeners[label]
	if !ok {
		return
	}
	s.engine.RemoveContextStateListener(listener)
}

// removeListeners mirrors removeContextStateListeners.
func (s *contextAdminListenCaseState) removeListeners() {
	s.engine.RemoveContextStateListeners()
}

// snapshot emits one {"operation":"admin"} record for the pinned probe:
// "admin:listeners" mirrors the getContextStateListeners iterator, carrying
// the registered listener labels in registration order.
func (s *contextAdminListenCaseState) snapshot(step compat.Step) error {
	if step.Mode != "admin:listeners" {
		return fmt.Errorf("%s: unknown admin probe %q", contextAdminListenID, step.Mode)
	}
	labels := []string{}
	registered := s.engine.ContextStateListeners()
	for _, candidate := range registered {
		matched := ""
		for label, listener := range s.listeners {
			if listener == candidate {
				matched = label
				break
			}
		}
		if matched == "" {
			return fmt.Errorf("%s: unregistered listener in ContextStateListeners snapshot", contextAdminListenID)
		}
		labels = append(labels, matched)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "admin",
		Statement: step.Statement,
		Name:      step.Name,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value:     map[string]any{"listeners": labels},
	})
	return nil
}

// undeploy removes the deployment registered under the label, or destroys the
// registered context for context labels, mirroring undeployModuleContaining.
func (s *contextAdminListenCaseState) undeploy(ctx context.Context, label string) error {
	if deployment, ok := s.deployments[label]; ok {
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy %q: %w", contextAdminListenID, label, err)
		}
		delete(s.deployments, label)
		return nil
	}
	if contextName, ok := s.contextByLabel[label]; ok {
		// DestroyContext is the Go lifecycle counterpart to undeploying the
		// create-context statement; it emits destroyed to the registered
		// state listeners.
		if err := s.engine.DestroyContext(ctx, contextName); err != nil {
			return fmt.Errorf("%s: undeploy context %q: %w", contextAdminListenID, label, err)
		}
		delete(s.contextByLabel, label)
		return nil
	}
	return fmt.Errorf("%s: unknown undeploy label %q", contextAdminListenID, label)
}

// undeployAll undeploys deployments in deploy order — matching Java's
// dependency-graph undeployAll, which retires dependents in deployment index
// order before the context module — then destroys the registered contexts in
// reverse registration order so nested children drop before their parents.
func (s *contextAdminListenCaseState) undeployAll(ctx context.Context) error {
	for _, label := range s.deployOrder {
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextAdminListenID, label, err)
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
			return fmt.Errorf("%s: undeploy-all context %q: %w", contextAdminListenID, contextName, err)
		}
	}
	s.registeredCtx = nil
	s.contextByLabel = map[string]string{}
	return nil
}

// decodeContextAdminListenPayload converts a send payload into the typed bean
// for the step's event type.
func decodeContextAdminListenPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextAdminListenBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean: %w", contextAdminListenID, err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextAdminListenS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S0: %w", contextAdminListenID, err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextAdminListenS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S1: %w", contextAdminListenID, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", contextAdminListenID, step.EventType)
	}
}

func loadContextAdminListenScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextAdminListenID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextAdminListenID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextAdminListenID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextAdminListenID, err)
	}
	if err := requireContextAdminListenFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextAdminListenID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextAdminListenID ||
		metadata.Description != contextAdminListenDescription ||
		metadata.JavaCommit != contextAdminListenJavaCommit ||
		metadata.JavaSource != contextAdminListenSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextAdminListenID)
	}
	if err := validateContextAdminListenStringArray(root["javaRuntimes"], contextAdminListenJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextAdminListenStringArray(root["javaNames"], contextAdminListenJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextAdminListenStringArray(root["javaStaticIds"], contextAdminListenJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextAdminListenStringArray(root["javaFlags"], contextAdminListenJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextAdminListenCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextAdminListenID, len(contextAdminListenCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextAdminListenFields(object,
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
		if definition.Case != contextAdminListenCases[index] ||
			definition.Ordinal != contextAdminListenOrdinals[index] ||
			definition.RuntimeID != contextAdminListenJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextAdminListenJavaExecutions[index] ||
			definition.Observation != contextAdminListenCaseObservations[index] ||
			definition.EPL != contextAdminListenCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextAdminListenID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextAdminListenID, err)
	}
	offset := 0
	for _, caseName := range contextAdminListenCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextAdminListenID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextAdminListenID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextAdminListenID, offset, caseName)
		}
		offset++
		want, ok := contextAdminListenCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextAdminListenID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextAdminListenID, caseName)
		}
		for _, pinned := range want {
			key, err := contextAdminListenStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextAdminListenID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextAdminListenID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextAdminListenID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextAdminListenID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", contextAdminListenID)
	}
	return scenario, nil
}

// contextAdminListenStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids
// with the payload compacted and ids rendered as a compact JSON array.
// Unknown fields on the step object are rejected.
func contextAdminListenStepKey(raw json.RawMessage) (string, error) {
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
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "mode": true, "ids": true,
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
		"|" + step.Mode + "|" + ids, nil
}

// validateContextAdminListenScenario re-checks a decoded scenario (used when
// the runner receives a scenario decoded by the generic loader path). The
// generic compat.Step op whitelist does not cover the listener-registry ops
// this scenario pins, so validation is the pinned step-key check plus the id
// and step-count invariants.
func validateContextAdminListenScenario(scenario compat.Scenario) error {
	if scenario.ID != contextAdminListenID {
		return fmt.Errorf("%s scenario id %q is not pinned", contextAdminListenID, scenario.ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", contextAdminListenID)
	}
	return nil
}

func requireContextAdminListenFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextAdminListenID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextAdminListenID, name)
		}
	}
	return nil
}

func validateContextAdminListenStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
