package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	eplVariablesEventTypedID         = "epl-variables-event-typed"
	eplVariablesEventTypedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplVariablesEventTypedSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesEventTyped.java"
)

const eplVariablesEventTypedDescription = "EPLVariablesEventTyped event-typed and bean-typed variable semantics: Object/bean/event-type variables with null reads, single and bulk runtime set/get, and filtered on-set whole-event assignment (ord 0), a five-variable module with property navigation, on-set property mutation and filtered whole-event replacement (ord 1), preconfigured global variable reads plus a deployment-scoped create (ord 2), set-prop assignments that emit varbean.theString/varbean.intPrimitive columns, no-op on null receivers, copy-on-write, sequential self-evaluation and int-to-long widening (ord 3), type-mismatch probes asserted by error category (ord 4), and create-schema plus event-typed variable assignment with orderId reads (ord 5). listener records mirror assertPropsNew rows; variable records mirror getVariableValue; snapshot records mirror assertIterator; error records carry the category token since declared-type rendering differs (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesEventTyped.java)."

var (
	eplVariablesEventTypedJavaRuntimeIDs = []string{
		"java-runtime-0ccc8cc9831c8681b7b1",
		"java-runtime-683d12f7c34c448319da",
		"java-runtime-cc00e385af1428a74014",
		"java-runtime-067180db58b5432f57dc",
		"java-runtime-400881e0dc41c6d3e4e4",
		"java-runtime-0794ac05da19ce34dbaf",
	}
	eplVariablesEventTypedJavaExecutions = []string{
		"EPLVariableEventTypedSceneOne",
		"EPLVariableEventTypedSceneTwo",
		"EPLVariableConfig",
		"EPLVariableEventTypedSetProp",
		"EPLVariableInvalid",
		"EPLVariableEventTypedCreateSchema",
	}
	eplVariablesEventTypedJavaStaticIDs = []string{
		"java-0d1b65aff5dcb7751e5b",
		"java-0d1b65aff5dcb7751e5b",
		"java-0d1b65aff5dcb7751e5b",
		"java-0d1b65aff5dcb7751e5b",
		"java-0d1b65aff5dcb7751e5b",
		"java-0d1b65aff5dcb7751e5b",
	}
	eplVariablesEventTypedJavaFlags = []string{"SERDEREQUIRED", "RUNTIMEOPS", "INVALIDITY"}
	eplVariablesEventTypedCases     = []string{
		"event-typed-scene-one",
		"event-typed-scene-two",
		"event-typed-config",
		"event-typed-set-prop",
		"event-typed-invalid",
		"event-typed-create-schema",
	}
	eplVariablesEventTypedOrdinals = []int{0, 1, 2, 3, 4, 5}
	eplVariablesEventTypedSources  = []string{eplVariablesEventTypedSource}
)

var eplVariablesEventTypedCaseObservations = []string{
	"listener+variable+snapshot; Object/bean/event-type variables read null until API-set, an on-SupportBean_S0(p00='X') set assigns 1 to the Object var, the arrival event to the event-typed var and null to the bean var (listener and iterator both observe {1,S0(2,'X'),null}), a bulk set nulls all three, a second bulk set restores {10L,A('A1'),S0(2,'X')}, and an on-SupportBean_A(id='Y') set assigns the arrival bean",
	"listener; a five-variable module (two bean-typed, one event-typed, two boxed longs) feeds a seven-column select that reads null until API-set, an on-SupportBean_B set mutates the held bean's theString/intPrimitive to {'EX',-999}, and an on-SupportBean(intPrimitive=0) set replaces the variable's whole event so the select reads {'E2',0,...}",
	"variable; preconfigured global variables read back their seeded values (SupportBean_S0 id 10, SupportBean_S1 id 20, constant 123, NonSerializable 'abc', then SupportBean_S2 id 30, SupportBean_S3 id 40, constant 'ABC') and a deployment-scoped create variable object varsobj3=222 reads 222",
	"listener+variable+snapshot; set-prop on a null bean variable is a silent no-op, after an API-set bean the on-set writes emit varbean.theString/varbean.intPrimitive columns {'A',1} observed by listener, iterator and dependent select, the write is copy-on-write (the variable holds a new bean), sequential assignments self-evaluate to '>E3<', and an int literal widens into longPrimitive",
	"set-variable-error+compile-error; a runtime SupportBean_S1 write into event-typed vars0_A and compile probes assigning arrival/int to mismatched variables all fail with the type-mismatch category (declared-type rendering differs between event-type-name and class-name declarations, so the category token is pinned)",
	"listener; a create-schema OrderEvent deployment feeds an event-typed variable whose on-OrderEvent assignment is read back as orderEvent.orderId c0='O1' then 'O2' on SupportBean sends (the variable-declaration EVENTTYPE dependency edge is unmodelable in Go and asserted only inside the oracle)",
}

var eplVariablesEventTypedCaseEPLs = []string{
	"@name('s0') select varobject, varbean, varbean.id, vartype, vartype.id from SupportBean",
	"@Name('Select') select varbean.theString as c0,varbean.intPrimitive as c1,vars0.id as c2,vars0.p00 as c3,varobj as c4,varbeannull.theString as c5, varobjnull as c6 from SupportBean_A",
	"@name('create') create variable object varsobj3=222",
	"@name('set') on SupportBean_A set varbean.theString = 'A', varbean.intPrimitive = 1",
	"on SupportBean_S0 arrival set vars1_A = arrival",
	"on OrderEvent as oe set orderEvent = oe;\n@name('s0') select orderEvent.orderId as c0 from SupportBean;\n",
}

// eplVariablesEventTypedCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath keys.
// Deploy steps carry the byte-exact EPL text the Java oracle compiles; the Go
// runner replays them as registration fixtures plus fluent plans. Milestones
// are no-ops in this harness and carry no steps. expectError="type-mismatch"
// pins the error category only: Java's declared-type rendering differs between
// event-type-name and class-name declarations, so the oracle asserts the exact
// message internally while both sides emit the category token.
var eplVariablesEventTypedCaseSteps = map[string][]string{
	"event-typed-scene-one": {
		"deploy|v0|||@name('v0') @public create variable Object varobject = null|||",
		"deployed|v0||||||",
		"deploy|v1|||@name('v1') @public create variable com.espertech.esper.regressionlib.support.bean.SupportBean_A varbean = null|||",
		"deployed|v1||||||",
		"deploy|v2|||@name('v2') @public create variable SupportBean_S0 vartype = null|||",
		"deployed|v2||||||",
		"deploy|s0|||@name('s0') select varobject, varbean, varbean.id, vartype, vartype.id from SupportBean|||",
		"deployed|s0||||||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"set-variable|v0|varobject|||\"abc\"||",
		"set-variable|v1|varbean|||{\"type\":\"SupportBean_A\",\"value\":{\"id\":\"A1\"}}||",
		"set-variable|v2|vartype|||{\"type\":\"SupportBean_S0\",\"value\":{\"id\":1}}||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"deploy|set|||@name('set') on SupportBean_S0(p00='X') arrival set varobject=1, vartype=arrival, varbean=null|||",
		"deployed|set||||||",
		"send|||SupportBean_S0||{\"id\":2,\"p00\":\"X\"}||",
		"read-variable|v0|varobject|||||",
		"read-variable|v2|vartype|||||",
		"read-variable|v2|vartype|||||",
		"snapshot|set||||||",
		"set-variable||bulk|||[{\"deployment\":\"v0\",\"name\":\"varobject\",\"value\":null},{\"deployment\":\"v2\",\"name\":\"vartype\",\"value\":null},{\"deployment\":\"v1\",\"name\":\"varbean\",\"value\":null}]||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"set-variable||bulk|||[{\"deployment\":\"v0\",\"name\":\"varobject\",\"value\":{\"type\":\"long\",\"value\":10}},{\"deployment\":\"v2\",\"name\":\"vartype\",\"value\":{\"type\":\"SupportBean_S0\",\"value\":{\"id\":2,\"p00\":\"X\"}}},{\"deployment\":\"v1\",\"name\":\"varbean\",\"value\":{\"type\":\"SupportBean_A\",\"value\":{\"id\":\"A1\"}}}]||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"deploy|set-two|||@name('set-two') on SupportBean_A(id='Y') arrival set varobject=null, vartype=null, varbean=arrival|||",
		"deployed|set-two||||||",
		"send|||SupportBean_A||{\"id\":\"Y\"}||",
		"read-variable|v0|varobject|||||",
		"read-variable|v2|vartype|||||",
		"read-variable|v1|varbean|||||",
		"snapshot|set-two||||||",
		"undeploy-all|||||||",
	},
	"event-typed-scene-two": {
		"deploy|vars|||@name('vars') @public create variable com.espertech.esper.common.internal.support.SupportBean varbeannull;\n@public create variable com.espertech.esper.common.internal.support.SupportBean varbean;\n@public create variable SupportBean_S0 vars0;\n@public create variable long varobj;\n@public create variable long varobjnull;\n|||",
		"deployed|vars||||||",
		"deploy|Select|||@Name('Select') select varbean.theString as c0,varbean.intPrimitive as c1,vars0.id as c2,vars0.p00 as c3,varobj as c4,varbeannull.theString as c5, varobjnull as c6 from SupportBean_A|||",
		"deployed|Select||||||",
		"send|||SupportBean_A||{\"id\":\"A1\"}||",
		"set-variable|vars|varobj|||{\"type\":\"long\",\"value\":101}||",
		"set-variable|vars|vars0|||{\"type\":\"SupportBean_S0\",\"value\":{\"id\":1,\"p00\":\"S01\"}}||",
		"set-variable|vars|varbean|||{\"type\":\"SupportBean\",\"value\":{\"theString\":\"E1\",\"intPrimitive\":-1}}||",
		"send|||SupportBean_A||{\"id\":\"A2\"}||",
		"deploy|Update|||@Name('Update') on SupportBean_B set varbean.theString = 'EX', varbean.intPrimitive = -999|||",
		"deployed|Update||||||",
		"send|||SupportBean_B||{\"id\":\"B1\"}||",
		"send|||SupportBean_A||{\"id\":\"A3\"}||",
		"deploy|Update2|||@Name('Update2') on SupportBean(intPrimitive = 0) as sb set varbean = sb|||",
		"deployed|Update2||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":0}||",
		"send|||SupportBean_A||{\"id\":\"A4\"}||",
		"undeploy-all|||||||",
	},
	"event-typed-config": {
		"read-variable||vars0_A|||||",
		"read-variable||vars1_A|||||",
		"read-variable||varsobj1|||||",
		"read-variable||myNonSerializable|||||",
		"read-variable||vars2|||||",
		"read-variable||vars3|||||",
		"read-variable||varsobj2|||||",
		"deploy|create|||@name('create') create variable object varsobj3=222|||",
		"deployed|create||||||",
		"read-variable|create|varsobj3|||||",
		"undeploy-all|||||||",
	},
	"event-typed-set-prop": {
		"deploy|create|||@name('create') @public create variable SupportBean varbean|||",
		"deployed|create||||||",
		"deploy|s0|||@name('s0') select varbean.theString,varbean.intPrimitive,varbean.getTheString() from SupportBean_S0|||",
		"deployed|s0||||||",
		"send|||SupportBean_S0||{\"id\":1}||",
		"deploy|set|||@name('set') on SupportBean_A set varbean.theString = 'A', varbean.intPrimitive = 1|||",
		"deployed|set||||||",
		"send|||SupportBean_A||{\"id\":\"E1\"}||",
		"send|||SupportBean_S0||{\"id\":2}||",
		"set-variable|create|varbean|||{\"type\":\"SupportBean\",\"value\":{}}||",
		"send|||SupportBean_A||{\"id\":\"E2\"}||",
		"snapshot|s0||||||",
		"send|||SupportBean_S0||{\"id\":3}||",
		"read-variable|create|varbean|||||",
		"undeploy|set||||||",
		"deploy|set|||@name('set') on SupportBean_A set varbean.theString = SupportBean_A.id, varbean.theString = '>'||varbean.theString||'<'|||",
		"deployed|set||||||",
		"send|||SupportBean_A||{\"id\":\"E3\"}||",
		"read-variable|create|varbean|||||",
		"undeploy|set||||||",
		"deploy|set|||@name('set') on SupportBean_A set varbean.longPrimitive = 1|||",
		"deployed|set||||||",
		"send|||SupportBean_A||{\"id\":\"E4\"}||",
		"read-variable|create|varbean|||||",
		"undeploy-all|||||||",
	},
	"event-typed-invalid": {
		"set-variable||vars0_A|||{\"type\":\"SupportBean_S1\",\"value\":{\"id\":1}}|type-mismatch|",
		"build-error|set-vars1_A|||on SupportBean_S0 arrival set vars1_A = arrival||type-mismatch|",
		"build-error|set-vars0_A|||on SupportBean_S0 arrival set vars0_A = 1||type-mismatch|",
	},
	"event-typed-create-schema": {
		"deploy|schema|||@buseventtype @public @name('schema') create schema OrderEvent(orderId string);|||",
		"deployed|schema||||||",
		"deploy|variable|||@public @name('variable') create variable OrderEvent orderEvent;|||",
		"deployed|variable||||||",
		"deploy|onset|||on OrderEvent as oe set orderEvent = oe;\n@name('s0') select orderEvent.orderId as c0 from SupportBean;\n|||",
		"deployed|onset||||||",
		"send|||OrderEvent||{\"orderId\":\"O1\"}||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"send|||OrderEvent||{\"orderId\":\"O2\"}||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"undeploy-all|||||||",
	},
}

// eplVariablesEventTypedBean mirrors SupportBean for the event-typed cases:
// theString stays nullable so bare sends and unwritten properties carry the
// same null as Java, and GetTheString backs the varbean.getTheString()
// method-invocation field of ord 3.
type eplVariablesEventTypedBean struct {
	TheString     *string `esper:"theString"`
	IntPrimitive  int32   `esper:"intPrimitive"`
	LongPrimitive int64   `esper:"longPrimitive"`
}

// GetTheString mirrors SupportBean.getTheString for the ord-3 method field.
func (b eplVariablesEventTypedBean) GetTheString() *string { return b.TheString }

// eplVariablesEventTypedS0 mirrors SupportBean_S0: id plus the p00..p03
// properties its value-equals covers.
type eplVariablesEventTypedS0 struct {
	ID  int32   `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

// eplVariablesEventTypedS1 mirrors SupportBean_S1 (id-only surface).
type eplVariablesEventTypedS1 struct {
	ID int32 `esper:"id"`
}

// eplVariablesEventTypedS2 mirrors SupportBean_S2 (id-only surface).
type eplVariablesEventTypedS2 struct {
	ID int32 `esper:"id"`
}

// eplVariablesEventTypedS3 mirrors regression-lib SupportBean_S3 (id-only
// surface; Java compares it by identity).
type eplVariablesEventTypedS3 struct {
	ID int32 `esper:"id"`
}

// eplVariablesEventTypedA mirrors regression-lib SupportBean_A (id equals).
type eplVariablesEventTypedA struct {
	ID string `esper:"id"`
}

// eplVariablesEventTypedB mirrors regression-lib SupportBean_B (id equals).
type eplVariablesEventTypedB struct {
	ID string `esper:"id"`
}

// eplVariablesEventTypedNonSerializable mirrors the suite's NonSerializable
// configuration value (equals on myString).
type eplVariablesEventTypedNonSerializable struct {
	MyString string `esper:"myString"`
}

// eplVariablesEventTypedCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, and the statement
// registry used by snapshot steps.
type eplVariablesEventTypedCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
	deployOrder []string
	statements  map[string]*esper.Statement
	sequences   map[string]uint64
	trace       *compat.Trace
	caseName    string
}

func runEplVariablesEventTypedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplVariablesEventTypedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeEplVariablesEventTyped(ctx, scenario, &trace)
}

// executeEplVariablesEventTyped replays the scenario: each case runs on a
// fresh environment/engine pair (one runtime per Java execution) and every
// step dispatches to the matching runtime action.
func executeEplVariablesEventTyped(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *eplVariablesEventTypedCaseState
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
			state, err = startEplVariablesEventTypedCase(step.Case, trace)
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
			if err := state.send(ctx, step); err != nil {
				return *trace, err
			}
		case "read-variable":
			if err := state.readVariable(step); err != nil {
				return *trace, err
			}
		case "set-variable":
			if err := state.setVariable(ctx, step); err != nil {
				return *trace, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
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
			return *trace, fmt.Errorf("%s: unsupported step op %q", eplVariablesEventTypedID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startEplVariablesEventTypedCase builds the fresh per-case environment: the
// suite's event types plus the seven preconfigured global variables of
// TestSuiteEPLVariable (vars0_A/vars1_A/varsobj1/vars2/vars3/varsobj2/
// myNonSerializable), seeded through env.RegisterVariable at null deployment
// scope exactly like the Java configuration.
func startEplVariablesEventTypedCase(caseName string, trace *compat.Trace) (*eplVariablesEventTypedCaseState, error) {
	env := esper.NewEnvironment()
	registrations := []struct {
		name   string
		schema func() (esper.Schema, error)
	}{
		{"SupportBean", func() (esper.Schema, error) {
			return esper.RegisterStruct[eplVariablesEventTypedBean](env, "SupportBean")
		}},
		{"SupportBean_S0", func() (esper.Schema, error) {
			return esper.RegisterStruct[eplVariablesEventTypedS0](env, "SupportBean_S0")
		}},
		{"SupportBean_S1", func() (esper.Schema, error) {
			return esper.RegisterStruct[eplVariablesEventTypedS1](env, "SupportBean_S1")
		}},
		{"SupportBean_S2", func() (esper.Schema, error) {
			return esper.RegisterStruct[eplVariablesEventTypedS2](env, "SupportBean_S2")
		}},
		{"SupportBean_A", func() (esper.Schema, error) {
			return esper.RegisterStruct[eplVariablesEventTypedA](env, "SupportBean_A")
		}},
		{"SupportBean_B", func() (esper.Schema, error) {
			return esper.RegisterStruct[eplVariablesEventTypedB](env, "SupportBean_B")
		}},
	}
	for _, registration := range registrations {
		if _, err := registration.schema(); err != nil {
			return nil, err
		}
	}
	globals := []struct {
		name    string
		initial any
		options []esper.VariableOption
	}{
		{"vars0_A", eplVariablesEventTypedS0{ID: 10}, nil},
		{"vars1_A", eplVariablesEventTypedS1{ID: 20}, nil},
		{"varsobj1", int32(123), []esper.VariableOption{esper.ConstantVariable()}},
		{"vars2", eplVariablesEventTypedS2{ID: 30}, nil},
		{"vars3", eplVariablesEventTypedS3{ID: 40}, nil},
		{"varsobj2", "ABC", []esper.VariableOption{esper.ConstantVariable()}},
		{"myNonSerializable", eplVariablesEventTypedNonSerializable{MyString: "abc"}, nil},
	}
	for _, global := range globals {
		if err := env.RegisterVariable(global.name, global.initial, global.options...); err != nil {
			return nil, err
		}
	}
	state := &eplVariablesEventTypedCaseState{
		env:         env,
		deployments: map[string]*esper.Deployment{},
		statements:  map[string]*esper.Statement{},
		sequences:   map[string]uint64{},
		trace:       trace,
		caseName:    caseName,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(eplVariablesEventTypedJavaRuntimeIDs[eplVariablesEventTypedCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func eplVariablesEventTypedCaseOrdinal(caseName string) int {
	for index, name := range eplVariablesEventTypedCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// silentVariableStatement builds the never-emitting statement that stands in
// for a Java create-variable/create-schema statement: a constant-false filter
// keeps the deployment quiet while the registration fixture performs the
// observable work. The projection names the deployment label so the statement
// shape mirrors the declared object.
func (s *eplVariablesEventTypedCaseState) silentVariableStatement(statement string, selections ...esper.Selection) (esper.Plan, error) {
	return s.env.Build(esper.Select(
		esper.From[eplVariablesEventTypedBean](s.env, "SupportBean").Filter(esper.Literal(false)),
		selections...,
	).Query(esper.StatementName(statement), esper.WithIterableUnbound()))
}

// deploy executes one deploy step: registration fixtures model create-variable
// and create-schema EPL at their scenario position, and statement fixtures
// deploy labeled plans. Listeners attach to every deployment of a listened
// statement name, mirroring the Java execution's addListener calls (ord 3
// redeploys 'set' and re-adds its listener each time).
func (s *eplVariablesEventTypedCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	switch s.caseName {
	case "event-typed-scene-one":
		switch label {
		case "v0":
			if err := s.env.RegisterVariable("varobject", nil); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(label, esper.Alias("varobject", esper.VariableRef[any]("varobject")))
			return s.deployPlan(label, plan, err)
		case "v1":
			if err := s.env.RegisterVariable("varbean", nil, esper.VariableType(reflect.TypeOf(eplVariablesEventTypedA{}))); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(label, esper.Alias("varbean", esper.VariableRef[eplVariablesEventTypedA]("varbean")))
			return s.deployPlan(label, plan, err)
		case "v2":
			if err := s.env.RegisterVariable("vartype", nil, esper.VariableType(reflect.TypeOf(eplVariablesEventTypedS0{}))); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(label, esper.Alias("vartype", esper.VariableRef[eplVariablesEventTypedS0]("vartype")))
			return s.deployPlan(label, plan, err)
		case "s0":
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesEventTypedBean](s.env, "SupportBean"),
				esper.Alias("varobject", esper.VariableRef[any]("varobject")),
				esper.Alias("varbean", esper.VariableRef[eplVariablesEventTypedA]("varbean")),
				esper.Alias("varbean.id", esper.Property[string](esper.VariableRef[eplVariablesEventTypedA]("varbean"), "id")),
				esper.Alias("vartype", esper.VariableRef[eplVariablesEventTypedS0]("vartype")),
				esper.Alias("vartype.id", esper.Property[int32](esper.VariableRef[eplVariablesEventTypedS0]("vartype"), "id")),
			).Query(esper.StatementName("s0"), esper.WithIterableUnbound()))
			return s.deployPlan(label, plan, err)
		case "set":
			// on SupportBean_S0(p00='X') arrival set varobject=1,
			// vartype=arrival, varbean=null
			plan, err := s.env.Build(esper.OnEvent(
				esper.From[eplVariablesEventTypedS0](s.env, "SupportBean_S0").Filter(
					esper.Equal[string](esper.Field[eplVariablesEventTypedS0, string]("p00"), esper.Literal("X"))),
			).SetVariables(
				esper.SetVariableExpr("varobject", esper.Literal(int32(1))),
				esper.SetVariableExpr("vartype", esper.EventValue[eplVariablesEventTypedS0]()),
				esper.SetVariableExpr("varbean", esper.NullLiteral[eplVariablesEventTypedA]()),
			).Query(esper.StatementName("set")))
			return s.deployPlan(label, plan, err)
		case "set-two":
			// on SupportBean_A(id='Y') arrival set varobject=null,
			// vartype=null, varbean=arrival
			plan, err := s.env.Build(esper.OnEvent(
				esper.From[eplVariablesEventTypedA](s.env, "SupportBean_A").Filter(
					esper.Equal[string](esper.Field[eplVariablesEventTypedA, string]("id"), esper.Literal("Y"))),
			).SetVariables(
				esper.SetVariableExpr("varobject", esper.NullLiteral[any]()),
				esper.SetVariableExpr("vartype", esper.NullLiteral[eplVariablesEventTypedS0]()),
				esper.SetVariableExpr("varbean", esper.EventValue[eplVariablesEventTypedA]()),
			).Query(esper.StatementName("set-two")))
			return s.deployPlan(label, plan, err)
		}
	case "event-typed-scene-two":
		switch label {
		case "vars":
			declarations := []struct {
				name string
				typ  reflect.Type
			}{
				{"varbeannull", reflect.TypeOf(eplVariablesEventTypedBean{})},
				{"varbean", reflect.TypeOf(eplVariablesEventTypedBean{})},
				{"vars0", reflect.TypeOf(eplVariablesEventTypedS0{})},
				{"varobj", reflect.TypeOf(int64(0))},
				{"varobjnull", reflect.TypeOf(int64(0))},
			}
			for _, declaration := range declarations {
				if err := s.env.RegisterVariable(declaration.name, nil, esper.VariableType(declaration.typ)); err != nil {
					return err
				}
			}
			plan, err := s.silentVariableStatement(label,
				esper.Alias("varbeannull", esper.VariableRef[eplVariablesEventTypedBean]("varbeannull")),
				esper.Alias("varbean", esper.VariableRef[eplVariablesEventTypedBean]("varbean")),
				esper.Alias("vars0", esper.VariableRef[eplVariablesEventTypedS0]("vars0")),
				esper.Alias("varobj", esper.VariableRef[int64]("varobj")),
				esper.Alias("varobjnull", esper.VariableRef[int64]("varobjnull")))
			return s.deployPlan(label, plan, err)
		case "Select":
			varbean := esper.VariableRef[eplVariablesEventTypedBean]("varbean")
			vars0 := esper.VariableRef[eplVariablesEventTypedS0]("vars0")
			varbeannull := esper.VariableRef[eplVariablesEventTypedBean]("varbeannull")
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesEventTypedA](s.env, "SupportBean_A"),
				esper.Alias("c0", esper.Property[string](varbean, "theString")),
				esper.Alias("c1", esper.Property[int32](varbean, "intPrimitive")),
				esper.Alias("c2", esper.Property[int32](vars0, "id")),
				esper.Alias("c3", esper.Property[string](vars0, "p00")),
				esper.Alias("c4", esper.VariableRef[int64]("varobj")),
				esper.Alias("c5", esper.Property[string](varbeannull, "theString")),
				esper.Alias("c6", esper.VariableRef[int64]("varobjnull")),
			).Query(esper.StatementName("Select"), esper.WithIterableUnbound()))
			return s.deployPlan(label, plan, err)
		case "Update":
			// on SupportBean_B set varbean.theString = 'EX',
			// varbean.intPrimitive = -999
			plan, err := s.env.Build(esper.OnEvent(
				esper.From[eplVariablesEventTypedB](s.env, "SupportBean_B"),
			).SetVariables(
				esper.SetVariablePropExpr("varbean", "theString", esper.Literal("EX")),
				esper.SetVariablePropExpr("varbean", "intPrimitive", esper.Literal(int32(-999))),
			).Query(esper.StatementName("Update")))
			return s.deployPlan(label, plan, err)
		case "Update2":
			// on SupportBean(intPrimitive = 0) as sb set varbean = sb
			plan, err := s.env.Build(esper.OnEvent(
				esper.From[eplVariablesEventTypedBean](s.env, "SupportBean").Filter(
					esper.Equal[int32](esper.Field[eplVariablesEventTypedBean, int32]("intPrimitive"), esper.Literal(int32(0)))),
			).SetVariables(
				esper.SetVariableExpr("varbean", esper.EventValue[eplVariablesEventTypedBean]()),
			).Query(esper.StatementName("Update2")))
			return s.deployPlan(label, plan, err)
		}
	case "event-typed-config":
		if label == "create" {
			if err := s.env.RegisterVariable("varsobj3", int32(222)); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(label, esper.Alias("varsobj3", esper.VariableRef[int32]("varsobj3")))
			return s.deployPlan(label, plan, err)
		}
	case "event-typed-set-prop":
		switch label {
		case "create":
			if err := s.env.RegisterVariable("varbean", nil, esper.VariableType(reflect.TypeOf(eplVariablesEventTypedBean{}))); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(label, esper.Alias("varbean", esper.VariableRef[eplVariablesEventTypedBean]("varbean")))
			return s.deployPlan(label, plan, err)
		case "s0":
			varbean := esper.VariableRef[eplVariablesEventTypedBean]("varbean")
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesEventTypedS0](s.env, "SupportBean_S0"),
				esper.Alias("varbean.theString", esper.Property[string](varbean, "theString")),
				esper.Alias("varbean.intPrimitive", esper.Property[int32](varbean, "intPrimitive")),
				esper.Alias("varbean.getTheString()", esper.Method[*string](varbean, "GetTheString")),
			).Query(esper.StatementName("s0"), esper.WithIterableUnbound()))
			return s.deployPlan(label, plan, err)
		case "set":
			plan, err := s.eventTypedSetPropPlan(step.Epl)
			return s.deployPlan(label, plan, err)
		}
	case "event-typed-create-schema":
		switch label {
		case "schema":
			if _, err := esper.RegisterMap(s.env, "OrderEvent", []esper.FieldSpec{
				esper.FieldDef("orderId", reflect.TypeOf("")),
			}); err != nil {
				return err
			}
			plan, err := s.env.Build(esper.FromAny(s.env, "OrderEvent").Filter(esper.Literal(false)).
				Select(esper.Alias("schema", esper.Literal("OrderEvent"))).
				Query(esper.StatementName("schema")))
			return s.deployPlan(label, plan, err)
		case "variable":
			if err := s.env.RegisterVariable("orderEvent", nil, esper.VariableType(reflect.TypeOf(esper.Event{}))); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(label, esper.Alias("orderEvent", esper.VariableRef[esper.Event]("orderEvent")))
			return s.deployPlan(label, plan, err)
		case "onset":
			// on OrderEvent as oe set orderEvent = oe;
			// @name('s0') select orderEvent.orderId as c0 from SupportBean;
			onsetPlan, err := s.env.Build(esper.OnRecord(
				esper.FromAny(s.env, "OrderEvent"),
			).SetVariables(
				esper.SetVariableExpr("orderEvent", esper.EventValue[esper.Event]()),
			).Query())
			if err != nil {
				return err
			}
			selectPlan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesEventTypedBean](s.env, "SupportBean"),
				esper.Alias("c0", esper.NestedField[string](esper.VariableRef[esper.Event]("orderEvent"), "orderId")),
			).Query(esper.StatementName("s0"), esper.WithIterableUnbound()))
			if err != nil {
				return err
			}
			return s.deployPlans(label, onsetPlan, selectPlan)
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", eplVariablesEventTypedID, s.caseName, label)
}

// eventTypedSetPropPlan builds the ord-3 'set' statement for the pinned EPL:
// the two-assignment form, the sequential self-evaluating form, and the
// int-to-long widening form.
func (s *eplVariablesEventTypedCaseState) eventTypedSetPropPlan(epl string) (esper.Plan, error) {
	trigger := esper.OnEvent(esper.From[eplVariablesEventTypedA](s.env, "SupportBean_A"))
	var query esper.Query
	switch epl {
	case "@name('set') on SupportBean_A set varbean.theString = 'A', varbean.intPrimitive = 1":
		query = trigger.SetVariables(
			esper.SetVariablePropExpr("varbean", "theString", esper.Literal("A")),
			esper.SetVariablePropExpr("varbean", "intPrimitive", esper.Literal(int32(1))),
		).Query(esper.StatementName("set"))
	case "@name('set') on SupportBean_A set varbean.theString = SupportBean_A.id, varbean.theString = '>'||varbean.theString||'<'":
		varbean := esper.VariableRef[eplVariablesEventTypedBean]("varbean")
		query = trigger.SetVariables(
			esper.SetVariablePropExpr("varbean", "theString", esper.Field[eplVariablesEventTypedA, string]("id")),
			esper.SetVariablePropExpr("varbean", "theString", esper.Concat(
				esper.Literal(">"),
				esper.Property[string](varbean, "theString"),
				esper.Literal("<"),
			)),
		).Query(esper.StatementName("set"))
	case "@name('set') on SupportBean_A set varbean.longPrimitive = 1":
		query = trigger.SetVariables(
			esper.SetVariablePropExpr("varbean", "longPrimitive", esper.Literal(int32(1))),
		).Query(esper.StatementName("set"))
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown set-prop EPL %q", eplVariablesEventTypedID, epl)
	}
	return s.env.Build(query)
}

// deployPlan deploys one built plan under the step label, records the
// deployment for targeted undeploy, and attaches listeners to every statement
// whose name is listened in this suite (s0, set, set-two, Select).
func (s *eplVariablesEventTypedCaseState) deployPlan(label string, plan esper.Plan, planErr error) error {
	if planErr != nil {
		return planErr
	}
	return s.deployPlans(label, plan)
}

func (s *eplVariablesEventTypedCaseState) deployPlans(label string, plans ...esper.Plan) error {
	deployment, err := s.engine.DeployPlans(context.Background(), plans)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eplVariablesEventTypedID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
		switch statement.Name() {
		case "s0", "set", "set-two", "Select":
			if err := s.subscribeSelect(statement); err != nil {
				return err
			}
		}
	}
	return nil
}

// subscribeSelect mirrors the oracle's listener: one listener record per
// delivered batch with normalized row fields.
func (s *eplVariablesEventTypedCaseState) subscribeSelect(statement *esper.Statement) error {
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
		}
		for _, result := range batch.New {
			record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: s.renderRow(result)})
		}
		for _, result := range batch.Old {
			record.Old = append(record.Old, compat.ResultRecord{Kind: "row", Fields: s.renderRow(result)})
		}
		s.trace.Records = append(s.trace.Records, record)
		return nil
	})
	return err
}

// renderRow normalizes one result row: null and missing states become the
// tagged {"state":"null"} object, matching the oracle's normalize.
func (s *eplVariablesEventTypedCaseState) renderRow(result esper.Result) map[string]any {
	fields := map[string]any{}
	collect := func(name string, value esper.Value) {
		fields[name] = eplVariablesEventTypedField(value)
	}
	if event, ok := result.Event(); ok {
		for _, field := range event.Schema().Fields() {
			collect(field.Name, event.Get(field.Name))
		}
		return fields
	}
	if row, ok := result.Row(); ok {
		for _, field := range row.Schema().Fields() {
			collect(field.Name, row.Get(field.Name))
		}
	}
	return fields
}

// send dispatches one event send by event type: bean payloads decode into the
// registered struct types and OrderEvent payloads send as map records,
// mirroring sendEventBean/sendEventMap.
func (s *eplVariablesEventTypedCaseState) send(ctx context.Context, step compat.Step) error {
	switch step.EventType {
	case "SupportBean":
		var event eplVariablesEventTypedBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return fmt.Errorf("%s SupportBean: %w", eplVariablesEventTypedID, err)
		}
		return s.engine.Send(ctx, step.EventType, event)
	case "SupportBean_S0":
		var event eplVariablesEventTypedS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return fmt.Errorf("%s SupportBean_S0: %w", eplVariablesEventTypedID, err)
		}
		return s.engine.Send(ctx, step.EventType, event)
	case "SupportBean_A":
		var event eplVariablesEventTypedA
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return fmt.Errorf("%s SupportBean_A: %w", eplVariablesEventTypedID, err)
		}
		return s.engine.Send(ctx, step.EventType, event)
	case "SupportBean_B":
		var event eplVariablesEventTypedB
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return fmt.Errorf("%s SupportBean_B: %w", eplVariablesEventTypedID, err)
		}
		return s.engine.Send(ctx, step.EventType, event)
	case "OrderEvent":
		var record map[string]any
		if err := json.Unmarshal(step.Payload, &record); err != nil {
			return fmt.Errorf("%s OrderEvent: %w", eplVariablesEventTypedID, err)
		}
		return s.engine.SendRecord(ctx, step.EventType, record)
	default:
		return fmt.Errorf("%s: unsupported event type %q", eplVariablesEventTypedID, step.EventType)
	}
}

// readVariable emits the {"operation":"variable","name","value"} record. The
// step's statement selects the owning deployment on the Java side; on the Go
// side variables are environment-scoped so the name resolves directly.
func (s *eplVariablesEventTypedCaseState) readVariable(step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "variable",
		Name:      step.Name,
	}
	value, ok := s.engine.GetVariable(step.Name)
	if !ok {
		return fmt.Errorf("%s: variable %q not found", eplVariablesEventTypedID, step.Name)
	}
	if value.IsNull() || value.IsMissing() {
		record.Value = map[string]any{"state": "null"}
	} else {
		record.Value = eplVariablesEventTypedCanon(value.Any())
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// setVariable executes one runtime variable write. A payload array is the bulk
// form (ordered deployment/name/value entries) mapping to Engine.SetVariables,
// mirroring Java's setVariableValue(Map<DeploymentIdNamePair,Object>). Steps
// carrying expectError assert the write is rejected with the pinned category
// and emit the set-variable-error record; plain writes stay silent exactly
// like the Java execution.
func (s *eplVariablesEventTypedCaseState) setVariable(ctx context.Context, step compat.Step) error {
	expected := step.ExpectError
	isBulk := len(step.Payload) > 0 && step.Payload[0] == '['
	var setErr error
	if isBulk {
		assignments, err := decodeEplVariablesEventTypedAssignments(step.Payload)
		if err != nil {
			return err
		}
		setErr = s.engine.SetVariables(ctx, assignments...)
	} else {
		value, err := decodeEplVariablesEventTypedAssignedValue(step.Payload)
		if err != nil {
			return err
		}
		setErr = s.engine.SetVariable(ctx, step.Name, value)
	}
	if expected == "" {
		if setErr != nil {
			return fmt.Errorf("%s: set-variable %q failed: %w", eplVariablesEventTypedID, step.Name, setErr)
		}
		return nil
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "set-variable-error",
	}
	switch {
	case setErr == nil:
		record.Value = "<no-error>"
	case isEplVariablesEventTypedTypeMismatch(setErr):
		record.Value = expected
	default:
		record.Value = eplVariablesEventTypedBareMessage(setErr)
	}
	if actual, ok := record.Value.(string); !ok || actual != expected {
		return fmt.Errorf("%s: set-variable error drift for %q: expected %q got %v",
			eplVariablesEventTypedID, step.Name, expected, record.Value)
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// isEplVariablesEventTypedTypeMismatch verifies the rejection is the
// Java-parity type-mismatch before the category token is recorded.
func isEplVariablesEventTypedTypeMismatch(err error) bool {
	// The build wraps the validation failure in an InvalidRule envelope; the
	// category follows the deepest esper.Error code, matching how the bare
	// message helper walks the chain.
	var code esper.ErrorCode
	found := false
	for current := err; current != nil; current = errors.Unwrap(current) {
		var espErr *esper.Error
		if errors.As(current, &espErr) {
			code = espErr.Code
			found = true
		}
	}
	return found && code == esper.ErrorTypeMismatch
}

// snapshot emits the {"operation":"snapshot"} record mirroring assertIterator:
// the statement's current iterator rows render through the same normalization
// as listener rows.
func (s *eplVariablesEventTypedCaseState) snapshot(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: unknown snapshot statement %q", eplVariablesEventTypedID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("%s: snapshot %q: %w", eplVariablesEventTypedID, step.Statement, err)
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Time:      compat.FormatTraceTime(result.Batch.Time),
	}
	for _, row := range result.Batch.New {
		record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: s.renderRow(row)})
	}
	for _, row := range result.Batch.Old {
		record.Old = append(record.Old, compat.ResultRecord{Kind: "row", Fields: s.renderRow(row)})
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// buildError runs one expected-invalid compile probe. expectError pins the
// error category token ("type-mismatch"): the Go build rejection is verified
// to carry the matching esper.Error code before the token is recorded.
func (s *eplVariablesEventTypedCaseState) buildError(step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	var query esper.Query
	switch step.Statement {
	case "set-vars1_A":
		// on SupportBean_S0 arrival set vars1_A = arrival: the S0 event is
		// not assignable to the SupportBean_S1-typed variable.
		query = esper.OnEvent(
			esper.From[eplVariablesEventTypedS0](s.env, "SupportBean_S0"),
		).SetVariables(
			esper.SetVariableExpr("vars1_A", esper.EventValue[eplVariablesEventTypedS0]()),
		).Query()
	case "set-vars0_A":
		// on SupportBean_S0 arrival set vars0_A = 1: an int literal is not
		// assignable to the SupportBean_S0-typed variable.
		query = esper.OnEvent(
			esper.From[eplVariablesEventTypedS0](s.env, "SupportBean_S0"),
		).SetVariables(
			esper.SetVariableExpr("vars0_A", esper.Literal(int32(1))),
		).Query()
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", eplVariablesEventTypedID, step.Statement)
	}
	_, buildErr := s.env.Build(query)
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", eplVariablesEventTypedID, step.Statement)
	}
	if step.ExpectError != "" {
		caught := "<no-error>"
		if isEplVariablesEventTypedTypeMismatch(buildErr) {
			caught = "type-mismatch"
		} else {
			caught = eplVariablesEventTypedBareMessage(buildErr)
		}
		if caught != step.ExpectError {
			return fmt.Errorf("%s: compile-error category drift for %q: expected %q got %q",
				eplVariablesEventTypedID, step.Statement, step.ExpectError, caught)
		}
		record.Value = caught
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// undeploy removes the deployment registered under the label, mirroring
// undeployModuleContaining.
func (s *eplVariablesEventTypedCaseState) undeploy(ctx context.Context, label string) error {
	deployment, ok := s.deployments[label]
	if !ok {
		return fmt.Errorf("%s: unknown undeploy label %q", eplVariablesEventTypedID, label)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", eplVariablesEventTypedID, label, err)
	}
	delete(s.deployments, label)
	return nil
}

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the modules they reference, mirroring undeployAll.
func (s *eplVariablesEventTypedCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eplVariablesEventTypedID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// eplVariablesEventTypedAssignmentEntry is one bulk set-variable entry: the
// deployment label selects the owning deployment on the Java side (Go
// variables are environment-scoped so it is informational here).
type eplVariablesEventTypedAssignmentEntry struct {
	Deployment string          `json:"deployment"`
	Name       string          `json:"name"`
	Value      json.RawMessage `json:"value"`
}

func decodeEplVariablesEventTypedAssignments(payload json.RawMessage) ([]esper.VariableAssignment, error) {
	var entries []eplVariablesEventTypedAssignmentEntry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return nil, fmt.Errorf("%s assignments: %w", eplVariablesEventTypedID, err)
	}
	assignments := make([]esper.VariableAssignment, 0, len(entries))
	for _, entry := range entries {
		value, err := decodeEplVariablesEventTypedAssignedValue(entry.Value)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, esper.VariableAssignment{Name: entry.Name, Value: value})
	}
	return assignments, nil
}

// decodeEplVariablesEventTypedAssignedValue converts an assignment payload
// into the typed Go value. Tagged objects pin the bean shape or boxed width so
// Go assignability mirrors Java's: "long" is Java Long, and the SupportBean*
// tags decode the bean the Java execution constructs. Bare scalars follow Java
// autoboxing (number→Integer/int32).
func decodeEplVariablesEventTypedAssignedValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '{' {
		var tagged struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(raw, &tagged); err != nil {
			return nil, fmt.Errorf("%s: decode assignment: %w", eplVariablesEventTypedID, err)
		}
		switch tagged.Type {
		case "long":
			var value int64
			if err := json.Unmarshal(tagged.Value, &value); err != nil {
				return nil, err
			}
			return value, nil
		case "SupportBean":
			var bean eplVariablesEventTypedBean
			if len(tagged.Value) > 0 {
				if err := json.Unmarshal(tagged.Value, &bean); err != nil {
					return nil, err
				}
			}
			return bean, nil
		case "SupportBean_S0":
			var bean eplVariablesEventTypedS0
			if err := json.Unmarshal(tagged.Value, &bean); err != nil {
				return nil, err
			}
			return bean, nil
		case "SupportBean_S1":
			var bean eplVariablesEventTypedS1
			if err := json.Unmarshal(tagged.Value, &bean); err != nil {
				return nil, err
			}
			return bean, nil
		case "SupportBean_A":
			var bean eplVariablesEventTypedA
			if err := json.Unmarshal(tagged.Value, &bean); err != nil {
				return nil, err
			}
			return bean, nil
		default:
			return nil, fmt.Errorf("%s: unknown assignment type tag %q", eplVariablesEventTypedID, tagged.Type)
		}
	}
	var scalar any
	if err := json.Unmarshal(raw, &scalar); err != nil {
		return nil, err
	}
	switch value := scalar.(type) {
	case float64:
		return int32(value), nil
	case string, bool:
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported assignment payload %s", eplVariablesEventTypedID, string(raw))
	}
}

// eplVariablesEventTypedField renders one listener/row field: null and missing
// states become the tagged {"state":"null"} object, matching the oracle's
// normalize; other values pass through canonical rendering.
func eplVariablesEventTypedField(value esper.Value) any {
	if value.IsNull() || value.IsMissing() {
		return map[string]any{"state": "null"}
	}
	return eplVariablesEventTypedCanon(value.Any())
}

// eplVariablesEventTypedCanon mirrors the oracle's canonical value rendering:
// numbers render as their long-truncated JSON number, strings/booleans pass
// through, beans render as sorted property objects over the Java-observed
// property surface, and pointer values render through their element.
func eplVariablesEventTypedCanon(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case bool, string:
		return typed
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float32:
		return float64(typed)
	case float64:
		return typed
	case *string:
		if typed == nil {
			return nil
		}
		return *typed
	case eplVariablesEventTypedBean:
		return map[string]any{
			"intPrimitive":  int64(typed.IntPrimitive),
			"longPrimitive": typed.LongPrimitive,
			"theString":     eplVariablesEventTypedCanon(typed.TheString),
		}
	case eplVariablesEventTypedS0:
		return map[string]any{
			"id":  int64(typed.ID),
			"p00": eplVariablesEventTypedCanon(typed.P00),
			"p01": eplVariablesEventTypedCanon(typed.P01),
			"p02": eplVariablesEventTypedCanon(typed.P02),
			"p03": eplVariablesEventTypedCanon(typed.P03),
		}
	case eplVariablesEventTypedS1:
		return map[string]any{"id": int64(typed.ID)}
	case eplVariablesEventTypedS2:
		return map[string]any{"id": int64(typed.ID)}
	case eplVariablesEventTypedS3:
		return map[string]any{"id": int64(typed.ID)}
	case eplVariablesEventTypedA:
		return map[string]any{"id": typed.ID}
	case eplVariablesEventTypedB:
		return map[string]any{"id": typed.ID}
	case eplVariablesEventTypedNonSerializable:
		return map[string]any{"myString": typed.MyString}
	case esper.Event:
		return eplVariablesEventTypedCanon(typed.Underlying())
	default:
		return fmt.Sprintf("%v", value)
	}
}

// eplVariablesEventTypedBareMessage mirrors the oracle's rootCauseMessage: the
// deepest esper.Error message carries the bare validation sentence.
func eplVariablesEventTypedBareMessage(err error) string {
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

func loadEplVariablesEventTypedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplVariablesEventTypedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplVariablesEventTypedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariablesEventTypedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariablesEventTypedID, err)
	}
	if err := requireEplVariablesEventTypedFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplVariablesEventTypedID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplVariablesEventTypedID ||
		metadata.Description != eplVariablesEventTypedDescription ||
		metadata.JavaCommit != eplVariablesEventTypedJavaCommit ||
		metadata.JavaSource != eplVariablesEventTypedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplVariablesEventTypedID)
	}
	if err := validateEplVariablesEventTypedStringArray(root["javaRuntimes"], eplVariablesEventTypedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariablesEventTypedStringArray(root["javaNames"], eplVariablesEventTypedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariablesEventTypedStringArray(root["javaStaticIds"], eplVariablesEventTypedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariablesEventTypedStringArray(root["javaFlags"], eplVariablesEventTypedJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplVariablesEventTypedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplVariablesEventTypedID, len(eplVariablesEventTypedCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplVariablesEventTypedFields(object,
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
		if definition.Case != eplVariablesEventTypedCases[index] ||
			definition.Ordinal != eplVariablesEventTypedOrdinals[index] ||
			definition.RuntimeID != eplVariablesEventTypedJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplVariablesEventTypedJavaExecutions[index] ||
			definition.Observation != eplVariablesEventTypedCaseObservations[index] ||
			definition.EPL != eplVariablesEventTypedCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplVariablesEventTypedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eplVariablesEventTypedID, err)
	}
	offset := 0
	for _, caseName := range eplVariablesEventTypedCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eplVariablesEventTypedID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplVariablesEventTypedID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eplVariablesEventTypedID, offset, caseName)
		}
		offset++
		want, ok := eplVariablesEventTypedCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eplVariablesEventTypedID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eplVariablesEventTypedID, caseName)
		}
		for _, pinned := range want {
			key, err := eplVariablesEventTypedStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplVariablesEventTypedID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eplVariablesEventTypedID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eplVariablesEventTypedID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariablesEventTypedID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplVariablesEventTypedStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath with
// the payload compacted. Unknown fields on the step object are rejected.
func eplVariablesEventTypedStepKey(raw json.RawMessage) (string, error) {
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
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true,
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
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp, nil
}

// validateEplVariablesEventTypedScenario re-checks a decoded scenario (used
// when the runner receives a scenario decoded by the generic loader path).
func validateEplVariablesEventTypedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplVariablesEventTypedID {
		return fmt.Errorf("%s scenario id %q is not pinned", eplVariablesEventTypedID, scenario.ID)
	}
	return nil
}

func requireEplVariablesEventTypedFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eplVariablesEventTypedID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eplVariablesEventTypedID, name)
		}
	}
	return nil
}

func validateEplVariablesEventTypedStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
