package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	contextVariablesID         = "context-variables"
	contextVariablesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextVariablesSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextVariables.java"
)

const contextVariablesDescription = "ContextVariables context-partitioned variable semantics: segmented key context where an uncorrelated on-set fires only for the partition the event keys into, allocating it if absent (ord 0); overlapping initiated/terminated context with correlated and uncorrelated (all-partition) on-set writes and reset-to-initial on re-initiation, plus a deploy/undeploy module tail using the integer keyword and a distinct initiator (ord 1); create-variable listener IR pairs and per-partition iterators on the on-set and variable statements (ord 2); partition-id-addressed variable get/set with global-variable rejection (ord 3); and compile-failure probes for unknown context, wrong-context variable, and out-of-context variable use in select, expr-window, limit, offset and output-every positions (ord 4). listener records on 'var' mirror the create-variable statement's IR pairs; snapshot records mirror statement iterators; variable records mirror getVariableValue reads (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextVariables.java)."

var (
	contextVariablesJavaRuntimeIDs = []string{
		"java-runtime-700973f399356686a11e",
		"java-runtime-821c37bbc3256af33468",
		"java-runtime-ea85235554e99c41e863",
		"java-runtime-4d98fe6487ed39e7d1de",
		"java-runtime-c7b860b3ff30ed78f3f5",
	}
	contextVariablesJavaExecutions = []string{
		"ContextVariablesSegmentedByKey",
		"ContextVariablesOverlapping",
		"ContextVariablesIterateAndListen",
		"ContextVariablesGetSetAPI",
		"ContextVariablesInvalid",
	}
	contextVariablesJavaStaticIDs = []string{
		"java-2443c804eb31da7902e7",
		"java-2443c804eb31da7902e7",
		"java-2443c804eb31da7902e7",
		"java-2443c804eb31da7902e7",
		"java-2443c804eb31da7902e7",
	}
	contextVariablesJavaFlags = []string{"RUNTIMEOPS"}
	contextVariablesCases     = []string{
		"segmented-by-key",
		"overlapping",
		"iterate-and-listen",
		"get-set-api",
		"invalid",
	}
	contextVariablesOrdinals = []int{0, 1, 2, 3, 4}
	contextVariablesSources  = []string{contextVariablesSource}
)

var contextVariablesCaseObservations = []string{
	"listener; an uncorrelated on-set fires only for the partition the event keys into, allocating it if absent: SupportBean(\"P1\",0) allocates without setting, SupportBean(\"P2\",11) allocates and sets in one event, and S0(5,\"P3\") allocates P3 which reads back the initial 0",
	"listener; correlated on-set writes the initiating partition while the uncorrelated intPrimitive<0 on-set writes every live partition (P1 reads -1 after the P2-targeted write); terminated partitions reset to the initial 5 on re-initiation; the module tail deploys and undeploys a distinct-initiator context with an integer-typed variable",
	"listener+snapshot; the create-variable statement listener fires one IR pair per set (new=assigned, old=previous including the initial 5) only for the updated partition and stays silent on partition creation; iterators on 'upd' and 'var' yield one row per live partition, 'upd' ordered by partition id and 'var' any-order",
	"variable+set-variable-error+variable-error; partition-id-addressed variable get/set (partition 0 reads 5 then 10, partition 1 reads 5 then 11) and VariableNotFoundException for a global variable via the partition-scoped APIs",
	"compile-error; probes pin context-not-found, wrong-context variable, and out-of-context variable use in select, expr-window, limit, offset and output-every positions; the reclaim_group_aged hint probe is asserted inside the oracle only (Go hints are string-typed with no expression surface)",
}

var contextVariablesCaseEPLs = []string{
	"@name('s0') context MyCtx select mycontextvar from SupportBean_S0",
	"@name('s0') context MyCtx select mycontextvar from SupportBean_S2(p20 = context.s0.p00)",
	"@name('upd') context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive",
	"context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive",
	"context MyCtx create variable int mycontext_invalid1 = 0",
}

// contextVariablesCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids
// keys. Deploy steps carry the byte-exact EPL text the Java oracle compiles;
// compileWithoutPath marks the ord-1 module tail whose Java execution compiles
// without the accumulated RegressionPath. Java's milestone calls are
// documented no-ops and carry no steps.
var contextVariablesCaseSteps = map[string][]string{
	"segmented-by-key": {
		"deploy|ctx|||@public create context MyCtx as partition by theString from SupportBean, p00 from SupportBean_S0|||||",
		"deployed|ctx||||||||",
		"deploy|var|||@public context MyCtx create variable int mycontextvar = 0|||||",
		"deployed|var||||||||",
		"deploy|upd|||context MyCtx on SupportBean(intPrimitive > 0) set mycontextvar = intPrimitive|||||",
		"deployed|upd||||||||",
		"deploy|s0|||@name('s0') context MyCtx select mycontextvar from SupportBean_S0|||||",
		"deployed|s0||||||||",
		"send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":0}||||",
		"send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":10}||||",
		"send|||SupportBean_S0||{\"id\":1,\"p00\":\"P1\"}||||",
		"send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":11}||||",
		"send|||SupportBean_S0||{\"id\":2,\"p00\":\"P2\"}||||",
		"send|||SupportBean_S0||{\"id\":3,\"p00\":\"P1\"}||||",
		"send|||SupportBean_S0||{\"id\":4,\"p00\":\"P2\"}||||",
		"send|||SupportBean_S0||{\"id\":5,\"p00\":\"P3\"}||||",
		"send|||SupportBean||{\"theString\":\"P3\",\"intPrimitive\":12}||||",
		"send|||SupportBean_S0||{\"id\":6,\"p00\":\"P3\"}||||",
		"undeploy-all|||||||||",
	},
	"overlapping": {
		"deploy|ctx|||@public create context MyCtx as initiated by SupportBean_S0 s0 terminated by SupportBean_S1(p10 = s0.p00)|||||",
		"deployed|ctx||||||||",
		"deploy|var|||@public context MyCtx create variable int mycontextvar = 5|||||",
		"deployed|var||||||||",
		"deploy|upd|||context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive|||||",
		"deployed|upd||||||||",
		"deploy|upd-all|||context MyCtx on SupportBean(intPrimitive < 0) set mycontextvar = intPrimitive|||||",
		"deployed|upd-all||||||||",
		"deploy|s0|||@name('s0') context MyCtx select mycontextvar from SupportBean_S2(p20 = context.s0.p00)|||||",
		"deployed|s0||||||||",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
		"send|||SupportBean_S2||{\"id\":1,\"p20\":\"P1\"}||||",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P2\"}||||",
		"send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":10}||||",
		"send|||SupportBean_S2||{\"id\":2,\"p20\":\"P2\"}||||",
		"send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":-1}||||",
		"send|||SupportBean_S2||{\"id\":2,\"p20\":\"P2\"}||||",
		"send|||SupportBean_S2||{\"id\":2,\"p20\":\"P1\"}||||",
		"send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":20}||||",
		"send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":21}||||",
		"send|||SupportBean_S2||{\"id\":2,\"p20\":\"P2\"}||||",
		"send|||SupportBean_S2||{\"id\":2,\"p20\":\"P1\"}||||",
		"send|||SupportBean_S1||{\"id\":0,\"p10\":\"P1\"}||||",
		"send|||SupportBean_S1||{\"id\":0,\"p10\":\"P2\"}||||",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
		"send|||SupportBean_S2||{\"id\":1,\"p20\":\"P1\"}||||",
		"undeploy-all|||||||||",
		"deploy|module|||@Name(\"context\")\ncreate context MyContext\ninitiated by distinct(theString) SupportBean as input\nterminated by SupportBean(theString = input.theString);\n\n@Name(\"ctx variable counter\")\ncontext MyContext create variable integer counter = 0;\n|||1||",
		"deployed|module||||||||",
		"undeploy-all|||||||||",
	},
	"iterate-and-listen": {
		"deploy|ctx|||@name('ctx') @public create context MyCtx as initiated by SupportBean_S0 s0 terminated after 24 hours|||||",
		"deployed|ctx||||||||",
		"deploy|var|||@name('var') @public context MyCtx create variable int mycontextvar = 5|||||",
		"deployed|var||||||||",
		"deploy|upd|||@name('upd') context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive|||||",
		"deployed|upd||||||||",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
		"send|||SupportBean||{\"theString\":\"P1\",\"intPrimitive\":100}||||",
		"snapshot|upd||||||||",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P2\"}||||",
		"send|||SupportBean||{\"theString\":\"P2\",\"intPrimitive\":101}||||",
		"snapshot|upd||||||||",
		"snapshot|var|||||||any|",
		"undeploy-all|||||||||",
	},
	"get-set-api": {
		"deploy|ctx|||@public create context MyCtx as initiated by SupportBean_S0 s0 terminated after 24 hours|||||",
		"deployed|ctx||||||||",
		"deploy|var|||@name('var') @public context MyCtx create variable int mycontextvar = 5|||||",
		"deployed|var||||||||",
		"deploy|upd|||context MyCtx on SupportBean(theString = context.s0.p00) set mycontextvar = intPrimitive|||||",
		"deployed|upd||||||||",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P1\"}||||",
		"read-variable|var|mycontextvar|||||||[0]",
		"set-variable|var|mycontextvar|||10||||[0]",
		"read-variable|var|mycontextvar|||||||[0]",
		"send|||SupportBean_S0||{\"id\":0,\"p00\":\"P2\"}||||",
		"read-variable|var|mycontextvar|||||||[1]",
		"set-variable|var|mycontextvar|||11||||[1]",
		"read-variable|var|mycontextvar|||||||[1]",
		"deploy|globalvar|||@name('globalvar') create variable int myglobarvar = 0|||||",
		"deployed|globalvar||||||||",
		"set-variable|globalvar|myglobarvar|||11|Variable by name 'myglobarvar' is a global variable and not context-partitioned|||[0]",
		"read-variable|globalvar|myglobarvar||||Variable by name 'myglobarvar' is a global variable and not context-partitioned|||[1]",
		"undeploy-all|||||||||",
	},
	"invalid": {
		"deploy|ctx-one|||@public create context MyCtxOne as partition by theString from SupportBean|||||",
		"deployed|ctx-one||||||||",
		"deploy|ctx-two|||@public create context MyCtxTwo as partition by p00 from SupportBean_S0|||||",
		"deployed|ctx-two||||||||",
		"deploy|var|||@public context MyCtxOne create variable int myctxone_int = 0|||||",
		"deployed|var||||||||",
		"build-error|invalid-context|||context MyCtx create variable int mycontext_invalid1 = 0||Context by name 'MyCtx' could not be found|||",
		"build-error|wrong-context|||context MyCtxTwo select myctxone_int from SupportBean_S0||Variable 'myctxone_int' defined for use with context 'MyCtxOne' is not available for use with context 'MyCtxTwo'|||",
		"build-error|outside-context-select|||select myctxone_int from SupportBean_S0||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
		"build-error|outside-context-expr-window|||select * from SupportBean_S0#expr(myctxone_int > 5)||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
		"build-error|outside-context-limit|||select * from SupportBean_S0#keepall limit myctxone_int||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
		"build-error|outside-context-offset|||select * from SupportBean_S0#keepall limit 10 offset myctxone_int||Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
		"build-error|outside-context-output-every|||select * from SupportBean_S0#keepall output every myctxone_int events||Failed to validate the output rate limiting clause: Variable 'myctxone_int' defined for use with context 'MyCtxOne' can only be accessed within that context|||",
		"undeploy-all|||||||||",
	},
}

// contextVariablesBean mirrors SupportBean for the context-variable cases.
type contextVariablesBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type contextVariablesS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type contextVariablesS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

type contextVariablesS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

// contextVariablesCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, and the deployed
// statements the snapshot and listener fixtures resolve by label.
type contextVariablesCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string]*esper.Deployment
	statements     map[string]*esper.Statement
	deployOrder    []string
	listenedLabels map[string]bool
	sequences      map[string]uint64
	trace          *compat.Trace
	caseName       string
}

// contextVariablesChangeListener mirrors the create-variable statement
// listener: each committed write to the variable arrives as one IR pair whose
// single new row carries the current value and whose single old row carries
// the previous value.
type contextVariablesChangeListener struct {
	state     *contextVariablesCaseState
	statement string
}

func (l contextVariablesChangeListener) OnVariableChanged(event esper.VariableChangeEvent) {
	state := l.state
	state.sequences[l.statement]++
	record := compat.TraceRecord{
		Case:      state.caseName,
		Operation: "listener",
		Statement: l.statement,
		Sequence:  state.sequences[l.statement],
		Time:      compat.FormatTraceTime(state.engine.Now()),
		New: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
			event.Name: contextVariablesField(event.New),
		}}},
		Old: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
			event.Name: contextVariablesField(event.Old),
		}}},
	}
	state.trace.Records = append(state.trace.Records, record)
}

func runContextVariablesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextVariablesScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextVariables(ctx, scenario, &trace)
}

// executeContextVariables replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeContextVariables(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextVariablesCaseState
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
			state, err = startContextVariablesCase(step.Case, trace)
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
			event, err := decodeContextVariablesPayload(step)
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
		case "read-variable":
			if err := state.readVariable(ctx, step); err != nil {
				return *trace, err
			}
		case "set-variable":
			if err := state.setVariable(ctx, step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
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
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextVariablesID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startContextVariablesCase builds the fresh per-case environment: the four
// event types the suite registers plus the engine pinned to the case's Java
// runtime id at the epoch start time.
func startContextVariablesCase(caseName string, trace *compat.Trace) (*contextVariablesCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextVariablesBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextVariablesS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextVariablesS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextVariablesS2](env, "SupportBean_S2"); err != nil {
		return nil, err
	}
	state := &contextVariablesCaseState{
		env:            env,
		deployments:    map[string]*esper.Deployment{},
		statements:     map[string]*esper.Statement{},
		listenedLabels: map[string]bool{},
		sequences:      map[string]uint64{},
		trace:          trace,
		caseName:       caseName,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextVariablesJavaRuntimeIDs[contextVariablesCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextVariablesCaseOrdinal(caseName string) int {
	for index, name := range contextVariablesCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy executes one deploy step: registration fixtures model create-context
// and create-variable EPL at their scenario positions (the established
// approved difference for the missing deployable statement types), and
// statement fixtures deploy labeled plans. The 's0' listener attaches on
// first deploy, mirroring addListener.
func (s *contextVariablesCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	beanSource := esper.From[contextVariablesBean](s.env, "SupportBean")
	s0Base := esper.From[contextVariablesS0](s.env, "SupportBean_S0")
	theString := esper.Field[contextVariablesBean, string]("theString")
	intPrimitive := esper.Field[contextVariablesBean, int32]("intPrimitive")
	initiatingP00 := esper.Property[string](esper.ContextInitiatingEvent(), "p00")
	correlated := esper.Equal[string](theString, initiatingP00)
	setCorrelated := func(name string) (esper.Plan, error) {
		return s.env.Build(esper.OnEvent(beanSource.Filter(correlated)).
			SetVariable("mycontextvar", intPrimitive).
			Query(esper.StatementName(name), esper.WithContext("MyCtx"), esper.WithIterableUnbound()))
	}
	switch s.caseName {
	case "segmented-by-key":
		switch label {
		case "ctx":
			_, err := esper.CreateKeyContextByStreams(s.env, "MyCtx",
				esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}},
				esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{esper.Field[contextVariablesS0, string]("p00")}})
			return err
		case "var":
			return s.env.RegisterContextVariable("MyCtx", "mycontextvar", int32(0))
		case "upd":
			plan, err := s.env.Build(esper.OnEvent(beanSource.
				Filter(esper.Greater[int32](intPrimitive, esper.Literal(int32(0))))).
				SetVariable("mycontextvar", intPrimitive).
				Query(esper.StatementName("upd"), esper.WithContext("MyCtx"), esper.WithIterableUnbound()))
			_, err = s.deployPlan(label, plan, err)
			return err
		case "s0":
			plan, err := s.env.Build(esper.Select(
				esper.From[contextVariablesS0](s.env, "SupportBean_S0"),
				esper.Alias("mycontextvar", esper.VariableRef[int32]("mycontextvar")),
			).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"), esper.WithIterableUnbound()))
			_, err = s.deployPlan(label, plan, err)
			return err
		}
	case "overlapping":
		switch label {
		case "ctx":
			end := esper.And(
				esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1")),
				esper.Equal[string](esper.Field[contextVariablesS1, string]("p10"), initiatingP00))
			_, err := esper.CreateOverlappingInitiatedTerminatedContext(s.env, "MyCtx", esper.Literal("global"),
				esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0")), end)
			return err
		case "var":
			return s.env.RegisterContextVariable("MyCtx", "mycontextvar", int32(5))
		case "upd":
			plan, err := setCorrelated("upd")
			_, err = s.deployPlan(label, plan, err)
			return err
		case "upd-all":
			plan, err := s.env.Build(esper.OnEvent(beanSource.
				Filter(esper.Less[int32](intPrimitive, esper.Literal(int32(0))))).
				SetVariable("mycontextvar", intPrimitive).
				Query(esper.StatementName("upd-all"), esper.WithContext("MyCtx"), esper.WithIterableUnbound()))
			_, err = s.deployPlan(label, plan, err)
			return err
		case "s0":
			plan, err := s.env.Build(esper.Select(
				esper.From[contextVariablesS2](s.env, "SupportBean_S2").
					Filter(esper.Equal[string](esper.Field[contextVariablesS2, string]("p20"), initiatingP00)),
				esper.Alias("mycontextvar", esper.VariableRef[int32]("mycontextvar")),
			).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"), esper.WithIterableUnbound()))
			_, err = s.deployPlan(label, plan, err)
			return err
		case "module":
			// The Java module tail deploys a distinct-initiator context with
			// an integer-typed variable and undeploys it without events. The
			// Go fixture registers the same context shape and variable, then
			// deploys a silent statement so undeploy-all exercises the
			// deployment lifecycle; env-scoped registrations persist past it
			// (documented deployment-lifecycle difference).
			end := esper.Equal[string](theString,
				esper.Property[string](esper.ContextInitiatingEvent(), "theString"))
			if _, err := esper.CreateDistinctInitiatedTerminatedContext(s.env, "MyContext", theString,
				esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean")), end); err != nil {
				return err
			}
			if err := s.env.RegisterContextVariable("MyContext", "counter", int32(0)); err != nil {
				return err
			}
			plan, err := s.env.Build(esper.Select(
				beanSource.Filter(esper.Literal(false)),
				esper.Alias("counter", esper.VariableRef[int32]("counter")),
			).Query(esper.StatementName("ctx variable counter"), esper.WithContext("MyContext")))
			_, err = s.deployPlan(label, plan, err)
			return err
		}
	case "iterate-and-listen":
		switch label {
		case "ctx":
			_, err := esper.CreateOverlappingPatternTerminatedContext(s.env, "MyCtx", esper.Literal("global"),
				esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0")),
				esper.TimerInterval(s0Base, 24*time.Hour))
			return err
		case "var":
			return s.env.RegisterContextVariable("MyCtx", "mycontextvar", int32(5))
		case "upd":
			plan, err := setCorrelated("upd")
			if err != nil {
				return err
			}
			deployment, err := s.deployPlan(label, plan, nil)
			if err != nil {
				return err
			}
			// env.addListener("var").addListener("upd"): the variable-change
			// listener mirrors the create-variable statement's IR pairs, then
			// the 'upd' subscriber mirrors the on-set statement listener.
			if err := s.engine.AddVariableChangeListener("mycontextvar",
				contextVariablesChangeListener{state: s, statement: "var"}); err != nil {
				return err
			}
			return s.subscribeStatement(deployment.Statements()[0])
		}
	case "get-set-api":
		switch label {
		case "ctx":
			_, err := esper.CreateOverlappingPatternTerminatedContext(s.env, "MyCtx", esper.Literal("global"),
				esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0")),
				esper.TimerInterval(s0Base, 24*time.Hour))
			return err
		case "var":
			return s.env.RegisterContextVariable("MyCtx", "mycontextvar", int32(5))
		case "upd":
			plan, err := setCorrelated("upd")
			_, err = s.deployPlan(label, plan, err)
			return err
		case "globalvar":
			return s.env.RegisterVariable("myglobarvar", int32(0))
		}
	case "invalid":
		switch label {
		case "ctx-one":
			_, err := esper.CreateKeyContext(s.env, "MyCtxOne", theString)
			return err
		case "ctx-two":
			_, err := esper.CreateKeyContext(s.env, "MyCtxTwo", esper.Field[contextVariablesS0, string]("p00"))
			return err
		case "var":
			return s.env.RegisterContextVariable("MyCtxOne", "myctxone_int", int32(0))
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextVariablesID, s.caseName, label)
}

// deployPlan deploys one built plan under the step label, records the
// deployment for targeted undeploy, and attaches the s0 listener.
func (s *contextVariablesCaseState) deployPlan(label string, plan esper.Plan, planErr error) (*esper.Deployment, error) {
	if planErr != nil {
		return nil, planErr
	}
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return nil, fmt.Errorf("%s: deploy %q: %w", contextVariablesID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		s.statements[label] = statement
		if statement.Name() == "s0" && !s.listenedLabels["s0"] {
			s.listenedLabels["s0"] = true
			if err := s.subscribeStatement(statement); err != nil {
				return nil, err
			}
		}
	}
	return deployment, nil
}

// subscribeStatement mirrors the oracle's listener: one listener record per
// delivered batch with normalized row fields.
func (s *contextVariablesCaseState) subscribeStatement(statement *esper.Statement) error {
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequences[statement.Name()]++
		record := compat.TraceRecord{
			Case:      s.caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  s.sequences[statement.Name()],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		}
		s.trace.Records = append(s.trace.Records, record)
		return nil
	})
	return err
}

// snapshot emits one {"operation":"snapshot"} record mirroring the Java
// statement iterator. 'upd' iterates the on-set statement (one row of current
// variable values per partition, ordered by partition id); 'var' reads the
// per-partition variable states, the Go analog of the create-variable
// statement's any-order iterator (the step carries mode "any").
func (s *contextVariablesCaseState) snapshot(ctx context.Context, step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Time:      compat.FormatTraceTime(s.engine.Now()),
	}
	switch step.Statement {
	case "upd":
		statement, ok := s.statements["upd"]
		if !ok {
			return fmt.Errorf("%s: snapshot statement %q was not deployed", contextVariablesID, step.Statement)
		}
		result, err := statement.Snapshot(ctx)
		if err != nil {
			return err
		}
		record.New = compat.NormalizeResults(result.Results())
	case "var":
		states, err := s.engine.ContextVariableStates(ctx, "MyCtx", esper.ContextPartitionSelectorAll{}, "mycontextvar")
		if err != nil {
			return err
		}
		for _, state := range states {
			fields := map[string]any{}
			for name, value := range state.Values {
				fields[name] = contextVariablesField(value)
			}
			record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: fields})
		}
	default:
		return fmt.Errorf("%s: unknown snapshot statement %q", contextVariablesID, step.Statement)
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// readVariable emits the {"operation":"variable"} record mirroring
// getVariableValue(pair, SupportSelectorById): the value is the single
// partition state addressed by the step's ids. Steps carrying expectError
// assert the read is rejected as an unknown-name error (Java's
// VariableNotFoundException) and record the canonical Java text.
func (s *contextVariablesCaseState) readVariable(ctx context.Context, step compat.Step) error {
	if len(step.IDs) != 1 {
		return fmt.Errorf("%s: read-variable %q requires exactly one partition id", contextVariablesID, step.Name)
	}
	states, err := s.engine.ContextVariableStates(ctx, "MyCtx",
		esper.SelectContextPartitionIDs(step.IDs[0]), step.Name)
	if step.ExpectError != "" {
		record := compat.TraceRecord{
			Case:      s.caseName,
			Operation: "variable-error",
			Statement: step.Statement,
			Name:      step.Name,
		}
		switch {
		case err == nil:
			record.Value = "<no-error>"
		case isContextVariablesNotFound(err):
			record.Value = step.ExpectError
		default:
			record.Value = contextVariablesBareMessage(err)
		}
		if actual, ok := record.Value.(string); !ok || actual != step.ExpectError {
			return fmt.Errorf("%s: read-variable message drift for %q: expected %q got %v",
				contextVariablesID, step.Name, step.ExpectError, record.Value)
		}
		s.trace.Records = append(s.trace.Records, record)
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: read-variable %q failed: %w", contextVariablesID, step.Name, err)
	}
	if len(states) != 1 {
		return fmt.Errorf("%s: read-variable %q returned %d states, want 1", contextVariablesID, step.Name, len(states))
	}
	value, ok := states[0].Values[step.Name]
	if !ok {
		return fmt.Errorf("%s: read-variable %q missing in partition state", contextVariablesID, step.Name)
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "variable",
		Statement: step.Statement,
		Name:      step.Name,
	}
	record.Value = contextVariablesField(value)
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// setVariable executes one partition-id-addressed write, mirroring
// setVariableValue(map, agentInstanceId). Steps carrying expectError assert
// the write is rejected as an unknown-name error (Java's
// VariableNotFoundException) and record the canonical Java text; plain writes
// stay silent exactly like the Java execution.
func (s *contextVariablesCaseState) setVariable(ctx context.Context, step compat.Step) error {
	if len(step.IDs) != 1 {
		return fmt.Errorf("%s: set-variable %q requires exactly one partition id", contextVariablesID, step.Name)
	}
	var value int32
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return fmt.Errorf("%s set-variable payload: %w", contextVariablesID, err)
	}
	setErr := s.engine.SetContextVariablesByID(ctx, "MyCtx", step.IDs[0],
		esper.VariableAssignment{Name: step.Name, Value: value})
	if step.ExpectError == "" {
		if setErr != nil {
			return fmt.Errorf("%s: set-variable %q failed: %w", contextVariablesID, step.Name, setErr)
		}
		return nil
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "set-variable-error",
		Statement: step.Statement,
		Name:      step.Name,
	}
	switch {
	case setErr == nil:
		record.Value = "<no-error>"
	case isContextVariablesNotFound(setErr):
		record.Value = step.ExpectError
	default:
		record.Value = contextVariablesBareMessage(setErr)
	}
	if actual, ok := record.Value.(string); !ok || actual != step.ExpectError {
		return fmt.Errorf("%s: set-variable message drift for %q: expected %q got %v",
			contextVariablesID, step.Name, step.ExpectError, record.Value)
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// isContextVariablesNotFound verifies the rejection is the Java-parity
// unknown-name error (VariableNotFoundException) before the canonical message
// is recorded.
func isContextVariablesNotFound(err error) bool {
	var espErr *esper.Error
	return errors.As(err, &espErr) && espErr.Code == esper.ErrorUnknownName
}

// buildError runs one expected-invalid compile probe against the fluent
// equivalent of the pinned EPL. The record carries the pinned Java message
// prefix once the Go rejection verifies, matching the oracle's prefix
// assertion; the eighth Java probe (reclaim_group_aged hint) has no Go
// expression surface and is asserted inside the oracle only.
func (s *contextVariablesCaseState) buildError(step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	s0 := esper.From[contextVariablesS0](s.env, "SupportBean_S0")
	variable := esper.VariableRef[int32]("myctxone_int")
	var buildErr error
	switch step.Statement {
	case "invalid-context":
		// Java's `context MyCtx create variable` fails to compile because the
		// context does not exist; the Go registration fixture rejects the same
		// unknown context name.
		buildErr = s.env.RegisterContextVariable("MyCtx", "mycontext_invalid1", int32(0))
	case "wrong-context":
		_, buildErr = s.env.Build(esper.Select(s0,
			esper.Alias("myctxone_int", variable),
		).Query(esper.WithContext("MyCtxTwo")))
	case "outside-context-select":
		_, buildErr = s.env.Build(esper.Select(s0,
			esper.Alias("myctxone_int", variable),
		).Query())
	case "outside-context-expr-window":
		_, buildErr = s.env.Build(s0.Window(esper.ExpressionWindow(
			esper.Greater[int32](variable, esper.Literal(int32(5))))).Query())
	case "outside-context-limit":
		_, buildErr = s.env.Build(s0.Window(esper.KeepAll()).Query(
			esper.LimitExpression(variable)))
	case "outside-context-offset":
		_, buildErr = s.env.Build(s0.Window(esper.KeepAll()).Query(
			esper.Limit(10), esper.OffsetExpression(variable)))
	case "outside-context-output-every":
		_, buildErr = s.env.Build(s0.Window(esper.KeepAll()).Query(
			esper.WithOutput(esper.OutputLastEveryEventsExpr(variable))))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", contextVariablesID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", contextVariablesID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix, mirroring the
	// isContextVariablesNotFound gate on the runtime read/set paths.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"invalid-context":              {esper.ErrorUnknownName, `context "MyCtx" is not registered`},
		"wrong-context":                {esper.ErrorInvalidRule, `belongs to context "MyCtxOne", not "MyCtxTwo"`},
		"outside-context-select":       {esper.ErrorInvalidRule, `can only be accessed within context "MyCtxOne"`},
		"outside-context-expr-window":  {esper.ErrorInvalidRule, `can only be accessed within context "MyCtxOne"`},
		"outside-context-limit":        {esper.ErrorInvalidRule, `can only be accessed within context "MyCtxOne"`},
		"outside-context-offset":       {esper.ErrorInvalidRule, `can only be accessed within context "MyCtxOne"`},
		"outside-context-output-every": {esper.ErrorInvalidRule, `can only be accessed within context "MyCtxOne"`},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", contextVariablesID, step.Statement, buildErr)
		}
	}
	if step.ExpectError != "" {
		record.Value = step.ExpectError
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// undeploy removes the deployment registered under the label.
func (s *contextVariablesCaseState) undeploy(ctx context.Context, label string) error {
	deployment, ok := s.deployments[label]
	if !ok {
		return fmt.Errorf("%s: unknown undeploy label %q", contextVariablesID, label)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", contextVariablesID, label, err)
	}
	delete(s.deployments, label)
	delete(s.statements, label)
	return nil
}

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the modules they reference, mirroring undeployAll.
func (s *contextVariablesCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextVariablesID, label, err)
		}
		delete(s.deployments, label)
		delete(s.statements, label)
	}
	s.deployOrder = nil
	return nil
}

// decodeContextVariablesPayload converts a send payload into the typed bean
// for the step's event type.
func decodeContextVariablesPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextVariablesBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean: %w", contextVariablesID, err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextVariablesS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S0: %w", contextVariablesID, err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextVariablesS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S1: %w", contextVariablesID, err)
		}
		return value, nil
	case "SupportBean_S2":
		var value contextVariablesS2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S2: %w", contextVariablesID, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", contextVariablesID, step.EventType)
	}
}

// contextVariablesField renders one listener/row/state field: null and
// missing states become the tagged {"state":"null"} object, matching the
// oracle's normalize; other values pass through as their JSON scalar.
func contextVariablesField(value esper.Value) any {
	if value.IsNull() || value.IsMissing() {
		return map[string]any{"state": "null"}
	}
	return value.Any()
}

// contextVariablesBareMessage mirrors the oracle's rootCauseMessage: the
// deepest esper.Error message carries the bare validation sentence.
func contextVariablesBareMessage(err error) string {
	bare := ""
	for current := err; current != nil; current = errors.Unwrap(current) {
		var espErr *esper.Error
		if errors.As(current, &espErr) && espErr.Message != "" {
			bare = espErr.Message
		}
	}
	if bare == "" && err != nil {
		return err.Error()
	}
	return bare
}

func loadContextVariablesScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextVariablesID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextVariablesID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextVariablesID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextVariablesID, err)
	}
	if err := requireContextVariablesFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextVariablesID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextVariablesID ||
		metadata.Description != contextVariablesDescription ||
		metadata.JavaCommit != contextVariablesJavaCommit ||
		metadata.JavaSource != contextVariablesSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextVariablesID)
	}
	if err := validateContextVariablesStringArray(root["javaRuntimes"], contextVariablesJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextVariablesStringArray(root["javaNames"], contextVariablesJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextVariablesStringArray(root["javaStaticIds"], contextVariablesJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextVariablesStringArray(root["javaFlags"], contextVariablesJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextVariablesCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextVariablesID, len(contextVariablesCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextVariablesFields(object,
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
		if definition.Case != contextVariablesCases[index] ||
			definition.Ordinal != contextVariablesOrdinals[index] ||
			definition.RuntimeID != contextVariablesJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextVariablesJavaExecutions[index] ||
			definition.Observation != contextVariablesCaseObservations[index] ||
			definition.EPL != contextVariablesCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextVariablesID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextVariablesID, err)
	}
	offset := 0
	for _, caseName := range contextVariablesCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextVariablesID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextVariablesID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextVariablesID, offset, caseName)
		}
		offset++
		want, ok := contextVariablesCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextVariablesID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextVariablesID, caseName)
		}
		for _, pinned := range want {
			key, err := contextVariablesStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextVariablesID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextVariablesID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextVariablesID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextVariablesID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// contextVariablesStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids
// with the payload compacted and ids rendered as a compact JSON array.
// Unknown fields on the step object are rejected.
func contextVariablesStepKey(raw json.RawMessage) (string, error) {
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

// validateContextVariablesScenario re-checks a decoded scenario (used when
// the runner receives a scenario decoded by the generic loader path).
func validateContextVariablesScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != contextVariablesID {
		return fmt.Errorf("%s scenario id %q is not pinned", contextVariablesID, scenario.ID)
	}
	return nil
}

func requireContextVariablesFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextVariablesID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextVariablesID, name)
		}
	}
	return nil
}

func validateContextVariablesStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
