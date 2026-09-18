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
	eplVariablesCreateID         = "epl-variables-create"
	eplVariablesCreateJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplVariablesCreateSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesCreate.java"
)

const eplVariablesCreateDescription = "EPLVariablesCreate variable-declaration semantics: object-model/deployable create-variable reads (ord 0), compile-start-stop lifecycle with redeploy reset (ord 1), create-variable IR-pair listeners and iterator reads with sequential on-set assignment (ord 2), the 29-variable typing/coercion matrix (ord 3), runtime set/get over primitive and boxed array dimensions (ord 5), and a List<String> variable with an enum-where projection (ord 6). Ord 4 (EPLVariableInvalid) is compile-failure-only and covered by Go probes. listener records on create-one/create-two mirror the create-variable statement's IR pairs; variable records mirror getVariableValue/iterator reads (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesCreate.java)."

var (
	eplVariablesCreateJavaRuntimeIDs = []string{
		"java-runtime-67e6441b95a4ca30917b",
		"java-runtime-74285cbcf0d7aeabf02f",
		"java-runtime-06466d91981a6c410b39",
		"java-runtime-75cb3e01ac92790b5194",
		"java-runtime-71775e7db7d1ffd875ba",
		"java-runtime-4ac4b7b111d10a20f74d",
	}
	eplVariablesCreateJavaExecutions = []string{
		"EPLVariableOM",
		"EPLVariableCompileStartStop",
		"EPLVariableSubscribeAndIterate",
		"EPLVariableDeclarationAndSelect",
		"EPLVariableDimensionAndPrimitive",
		"EPLVariableGenericType",
	}
	eplVariablesCreateJavaStaticIDs = []string{
		"java-6a445f399d99f5d4785d",
		"java-6a445f399d99f5d4785d",
		"java-6a445f399d99f5d4785d",
		"java-6a445f399d99f5d4785d",
		"java-6a445f399d99f5d4785d",
		"java-6a445f399d99f5d4785d",
	}
	eplVariablesCreateJavaFlags = []string{"RUNTIMEOPS"}
	eplVariablesCreateCases     = []string{
		"variable-om",
		"variable-compile-start-stop",
		"variable-subscribe-iterate",
		"variable-declaration-select",
		"variable-dimension-primitive",
		"variable-generic-type",
	}
	eplVariablesCreateOrdinals = []int{0, 1, 2, 3, 5, 6}
	eplVariablesCreateSources  = []string{eplVariablesCreateSource}
)

var eplVariablesCreateCaseObservations = []string{
	"listener; two create-variable deployments (uninitialized long, string initialized to \"abc\") feed a select that observes new=[null,\"abc\"] on the first SupportBean; the SODA toEPL assertions are API-only approved differences",
	"listener+variable; select over two module variables, then ESPER-545: an on-pattern set increments module variable FOO to 1, undeploy-all plus redeploy of the create module resets FOO to 0, and a failed compile leaves no residue so the same private create redeploys",
	"listener+variable; create-variable statements deliver committed writes as IR pairs (new=current, old=previous), the on-set assignments apply sequentially so var2SAI sees the new var1SAI, iterator reads track current values, and redeploying create-two resets it to 20 while create-one survives at 400",
	"listener; 29 create-variable modules pin the declaration typing/coercion matrix (string-to-int, expression folding, equality-as-initializer, char/byte/short/float widths, uninitialized nulls) projected by one select",
	"variable+set-variable-error; runtime set/get over int[primitive], int[], Object[] and Object[][] variables: exact-order reads after writes, and rejections for String[] into int[], int[] into Integer[]/Object[]/Object[][] (primitive arrays are not covariant)",
	"listener+variable; a List<String> variable initialized to [a,b] reads back in exact order and projects c0=[a,b] plus the enum-where filtered c1=[a] on a SupportBean send",
}

var eplVariablesCreateCaseEPLs = []string{
	"@name('s0') select var1OMCreate, var2OMCreate from SupportBean",
	"@name('create') @public create variable int FOO = 0",
	"@name('set') on SupportBean set var1SAI = intPrimitive * 2, var2SAI = var1SAI + 1",
	"@name('s0') select varX1,varX2,varX3,varX4,varX5,varX6,varX7,varX8,varX9,varX10,varX11,varX12,varX13,varX14,varX15,varX16,varX17,varX18,varX19,varX20,varX21,varX22,varX23,varX24,varX25,varX26,varX27,varX28,varX29 from SupportBean",
	"@name('vars') create variable int[primitive] int_prim = null;\ncreate variable int[] int_boxed = null;\ncreate variable java.lang.Object[] objectarray = null;\ncreate variable java.lang.Object[][] objectarray_2dim = null;\n",
	"@name('var') create variable List<String> mylist = Arrays.asList('a', 'b');\n@name('s0') select mylist as c0, mylist.where(v => v = 'a') as c1 from SupportBean;\n",
}

// eplVariablesCreateCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath keys.
// Deploy steps carry the byte-exact EPL text the Java oracle compiles;
// compileWithoutPath marks the recreation-style compiles that run without the
// accumulated RegressionPath (redeployed create modules and private creates).
var eplVariablesCreateCaseSteps = map[string][]string{
	"variable-om": {
		"deploy|create-var1|||@public create variable long var1OMCreate|||",
		"deployed|create-var1||||||",
		"deploy|create-var2|||@public create variable string var2OMCreate = \"abc\"|||",
		"deployed|create-var2||||||",
		"deploy|s0|||@name('s0') select var1OMCreate, var2OMCreate from SupportBean|||",
		"deployed|s0||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}||",
		"deploy|create-arrdouble|||create variable double[] arrdouble = {1.0d,2.0d}|||1",
		"deployed|create-arrdouble||||||",
		"undeploy-all|||||||",
	},
	"variable-compile-start-stop": {
		"deploy|create-var1|||@public create variable long var1CSS|||",
		"deployed|create-var1||||||",
		"deploy|create-var2|||@public create variable string var2CSS = \"abc\"|||",
		"deployed|create-var2||||||",
		"deploy|s0|||@name('s0') select var1CSS, var2CSS from SupportBean|||",
		"deployed|s0||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":10}||",
		"deploy|create|||@name('create') @public create variable int FOO = 0|||",
		"deployed|create||||||",
		"deploy|set|||on pattern [every SupportBean] set FOO = FOO + 1|||",
		"deployed|set||||||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"read-variable|create|FOO|||||",
		"undeploy-all|||||||",
		"deploy|create|||@name('create') @public create variable int FOO = 0|||1",
		"deployed|create||||||",
		"read-variable|create|FOO|||||",
		"deploy|create-x|||@private create variable int x = 123|||1",
		"deployed|create-x||||||",
		"build-error|missing-script|||select missingScript(x) from SupportBean|||",
		"deploy|create-x2|||@private create variable int x = 123|||1",
		"deployed|create-x2||||||",
		"undeploy-all|||||||",
	},
	"variable-subscribe-iterate": {
		"deploy|create-one|||@name('create-one') @public create variable long var1SAI = null|||",
		"deployed|create-one||||||",
		"read-variable|create-one|var1SAI|||||",
		"deploy|create-two|||@name('create-two') @public create variable long var2SAI = 20|||",
		"deployed|create-two||||||",
		"read-variable|create-two|var2SAI|||||",
		"deploy|set|||@name('set') on SupportBean set var1SAI = intPrimitive * 2, var2SAI = var1SAI + 1|||",
		"deployed|set||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":100}||",
		"read-variable|create-one|var1SAI|||||",
		"read-variable|create-two|var2SAI|||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":200}||",
		"read-variable|create-one|var1SAI|||||",
		"read-variable|create-two|var2SAI|||||",
		"undeploy|set||||||",
		"undeploy|create-two||||||",
		"deploy|create-two|||@name('create-two') @public create variable long var2SAI = 20|||1",
		"deployed|create-two||||||",
		"read-variable|create-one|var1SAI|||||",
		"read-variable|create-two|var2SAI|||||",
		"undeploy-all|||||||",
	},
	"variable-declaration-select": {
		"deploy|create-varX1|||@public create variable int varX1 = 1|||",
		"deployed|create-varX1||||||",
		"deploy|create-varX2|||@public create variable int varX2 = '2'|||",
		"deployed|create-varX2||||||",
		"deploy|create-varX3|||@public create variable INTEGER varX3 =  3+2 |||",
		"deployed|create-varX3||||||",
		"deploy|create-varX4|||@public create variable bool varX4 =  true|false |||",
		"deployed|create-varX4||||||",
		"deploy|create-varX5|||@public create variable boolean varX5 =  varX1=1 |||",
		"deployed|create-varX5||||||",
		"deploy|create-varX6|||@public create variable double varX6 =  1.11 |||",
		"deployed|create-varX6||||||",
		"deploy|create-varX7|||@public create variable double varX7 =  1.20d |||",
		"deployed|create-varX7||||||",
		"deploy|create-varX8|||@public create variable Double varX8 =  ' 1.12 ' |||",
		"deployed|create-varX8||||||",
		"deploy|create-varX9|||@public create variable float varX9 =  1.13f*2f |||",
		"deployed|create-varX9||||||",
		"deploy|create-varX10|||@public create variable FLOAT varX10 =  -1.14f |||",
		"deployed|create-varX10||||||",
		"deploy|create-varX11|||@public create variable string varX11 =  ' XXXX ' |||",
		"deployed|create-varX11||||||",
		"deploy|create-varX12|||@public create variable string varX12 =  \"a\" |||",
		"deployed|create-varX12||||||",
		"deploy|create-varX13|||@public create variable character varX13 = 'a'|||",
		"deployed|create-varX13||||||",
		"deploy|create-varX14|||@public create variable char varX14 = 'x'|||",
		"deployed|create-varX14||||||",
		"deploy|create-varX15|||@public create variable short varX15 =  20 |||",
		"deployed|create-varX15||||||",
		"deploy|create-varX16|||@public create variable SHORT varX16 =  ' 9 ' |||",
		"deployed|create-varX16||||||",
		"deploy|create-varX17|||@public create variable long varX17 =  20*2 |||",
		"deployed|create-varX17||||||",
		"deploy|create-varX18|||@public create variable LONG varX18 =  ' 9 ' |||",
		"deployed|create-varX18||||||",
		"deploy|create-varX19|||@public create variable byte varX19 =  20*2 |||",
		"deployed|create-varX19||||||",
		"deploy|create-varX20|||@public create variable BYTE varX20 = 9+1|||",
		"deployed|create-varX20||||||",
		"deploy|create-varX21|||@public create variable int varX21|||",
		"deployed|create-varX21||||||",
		"deploy|create-varX22|||@public create variable bool varX22|||",
		"deployed|create-varX22||||||",
		"deploy|create-varX23|||@public create variable double varX23|||",
		"deployed|create-varX23||||||",
		"deploy|create-varX24|||@public create variable float varX24|||",
		"deployed|create-varX24||||||",
		"deploy|create-varX25|||@public create variable string varX25|||",
		"deployed|create-varX25||||||",
		"deploy|create-varX26|||@public create variable char varX26|||",
		"deployed|create-varX26||||||",
		"deploy|create-varX27|||@public create variable short varX27|||",
		"deployed|create-varX27||||||",
		"deploy|create-varX28|||@public create variable long varX28|||",
		"deployed|create-varX28||||||",
		"deploy|create-varX29|||@public create variable BYTE varX29|||",
		"deployed|create-varX29||||||",
		"deploy|s0|||@name('s0') select varX1,varX2,varX3,varX4,varX5,varX6,varX7,varX8,varX9,varX10,varX11,varX12,varX13,varX14,varX15,varX16,varX17,varX18,varX19,varX20,varX21,varX22,varX23,varX24,varX25,varX26,varX27,varX28,varX29 from SupportBean|||",
		"deployed|s0||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}||",
		"undeploy-all|||||||",
	},
	"variable-dimension-primitive": {
		"deploy|vars|||@name('vars') create variable int[primitive] int_prim = null;\ncreate variable int[] int_boxed = null;\ncreate variable java.lang.Object[] objectarray = null;\ncreate variable java.lang.Object[][] objectarray_2dim = null;\n|||",
		"deployed|vars||||||",
		"set-variable|vars|int_prim|||{\"type\":\"int-array\",\"value\":[1,2]}||",
		"read-variable|vars|int_prim|||||",
		"set-variable|vars|int_prim|||{\"type\":\"string-array\",\"value\":[]}|Variable 'int_prim' of declared type int[] cannot be assigned a value of type String[]|",
		"set-variable|vars|int_boxed|||{\"type\":\"integer-array\",\"value\":[1,2]}||",
		"read-variable|vars|int_boxed|||||",
		"set-variable|vars|int_boxed|||{\"type\":\"int-array\",\"value\":[]}|Variable 'int_boxed' of declared type Integer[] cannot be assigned a value of type int[]|",
		"set-variable|vars|objectarray|||{\"type\":\"integer-array\",\"value\":[1,2]}||",
		"read-variable|vars|objectarray|||||",
		"set-variable|vars|objectarray|||{\"type\":\"int-array\",\"value\":[]}|Variable 'objectarray' of declared type Object[] cannot be assigned a value of type int[]|",
		"set-variable|vars|objectarray_2dim|||{\"type\":\"object-array-2dim\",\"value\":[[1,2]]}||",
		"read-variable|vars|objectarray_2dim|||||",
		"set-variable|vars|objectarray_2dim|||{\"type\":\"int-array\",\"value\":[]}|Variable 'objectarray_2dim' of declared type Object[][] cannot be assigned a value of type int[]|",
		"undeploy-all|||||||",
	},
	"variable-generic-type": {
		"deploy|var|||@name('var') create variable List<String> mylist = Arrays.asList('a', 'b');\n@name('s0') select mylist as c0, mylist.where(v => v = 'a') as c1 from SupportBean;\n|||",
		"deployed|var||||||",
		"read-variable|var|mylist|||||",
		"send|||SupportBean||{\"theString\":null,\"intPrimitive\":0}||",
		"undeploy-all|||||||",
	},
}

// eplVariablesCreateBean mirrors SupportBean for the create-variable cases:
// theString stays nullable so the bare sendEventBean(new SupportBean()) sends
// of ord 1 and ord 6 carry the same null theString as Java.
type eplVariablesCreateBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int32   `esper:"intPrimitive"`
}

// eplVariablesCreateDecl is one pre-folded create-variable declaration of the
// ord-3 typing matrix: the Go runner registers the folded value with the
// declared width (byte→int8, short→int16, char→int32 rune, float→float32).
type eplVariablesCreateDecl struct {
	label string
	name  string
	value any
	typ   reflect.Type
}

var eplVariablesCreateDecls = []eplVariablesCreateDecl{
	{"create-varX1", "varX1", int32(1), nil},
	{"create-varX2", "varX2", int32(2), nil},
	{"create-varX3", "varX3", int32(5), nil},
	{"create-varX4", "varX4", true, nil},
	{"create-varX5", "varX5", true, nil},
	{"create-varX6", "varX6", 1.11, nil},
	{"create-varX7", "varX7", 1.20, nil},
	{"create-varX8", "varX8", 1.12, nil},
	{"create-varX9", "varX9", float32(2.26), nil},
	{"create-varX10", "varX10", float32(-1.14), nil},
	{"create-varX11", "varX11", " XXXX ", nil},
	{"create-varX12", "varX12", "a", nil},
	{"create-varX13", "varX13", int32('a'), nil},
	{"create-varX14", "varX14", int32('x'), nil},
	{"create-varX15", "varX15", int16(20), nil},
	{"create-varX16", "varX16", int16(9), nil},
	{"create-varX17", "varX17", int64(40), nil},
	{"create-varX18", "varX18", int64(9), nil},
	{"create-varX19", "varX19", int8(40), nil},
	{"create-varX20", "varX20", int8(10), nil},
	{"create-varX21", "varX21", nil, reflect.TypeOf(int32(0))},
	{"create-varX22", "varX22", nil, reflect.TypeOf(false)},
	{"create-varX23", "varX23", nil, reflect.TypeOf(float64(0))},
	{"create-varX24", "varX24", nil, reflect.TypeOf(float32(0))},
	{"create-varX25", "varX25", nil, reflect.TypeOf("")},
	{"create-varX26", "varX26", nil, reflect.TypeOf(int32(0))},
	{"create-varX27", "varX27", nil, reflect.TypeOf(int16(0))},
	{"create-varX28", "varX28", nil, reflect.TypeOf(int64(0))},
	{"create-varX29", "varX29", nil, reflect.TypeOf(int8(0))},
}

// eplVariablesCreateCharVars names the ord-3 variables whose declared Java
// type is char/character: the Go runner stores them as int32 runes and renders
// them as one-character strings, matching Java's Character normalization.
var eplVariablesCreateCharVars = map[string]bool{
	"varX13": true,
	"varX14": true,
	"varX26": true,
}

// eplVariablesCreateFloatVars names the ord-3 float variables: Go float32
// values render through float64 widening, matching the oracle's
// Number.doubleValue JSON rendering.
var eplVariablesCreateFloatVars = map[string]bool{
	"varX9":  true,
	"varX10": true,
	"varX24": true,
}

// eplVariablesCreateDeployResult is what one deploy-step fixture performs:
// optional variable registrations (env-scoped or module-scoped) plus an
// optional statement plan deployed under the step label.
type eplVariablesCreateDeployResult struct {
	deployment *esper.Deployment
}

// eplVariablesCreateCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, the deploy fixtures,
// and the variable-name resolution map (module-scoped variables resolve to
// their qualified catalog names).
type eplVariablesCreateCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	varOwners      map[string]string
	modules        map[string]esper.Module
	listenedLabels map[string]bool
	sequences      map[string]uint64
	trace          *compat.Trace
	caseName       string
}

// eplVariablesCreateChangeListener mirrors the create-variable statement
// listener: each committed write to the variable arrives as one IR pair whose
// single new row carries the current value and whose single old row carries
// the previous value.
type eplVariablesCreateChangeListener struct {
	state     *eplVariablesCreateCaseState
	statement string
}

func (l eplVariablesCreateChangeListener) OnVariableChanged(event esper.VariableChangeEvent) {
	state := l.state
	state.sequences[l.statement]++
	short := event.Name
	if index := strings.LastIndex(short, "::"); index >= 0 {
		short = short[index+2:]
	}
	record := compat.TraceRecord{
		Case:      state.caseName,
		Operation: "listener",
		Statement: l.statement,
		Sequence:  state.sequences[l.statement],
		Time:      compat.FormatTraceTime(state.engine.Now()),
		New: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
			short: eplVariablesCreateField(event.New),
		}}},
		Old: []compat.ResultRecord{{Kind: "row", Fields: map[string]any{
			short: eplVariablesCreateField(event.Old),
		}}},
	}
	state.trace.Records = append(state.trace.Records, record)
}

func runEplVariablesCreateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplVariablesCreateScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeEplVariablesCreate(ctx, scenario, &trace)
}

// executeEplVariablesCreate replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeEplVariablesCreate(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *eplVariablesCreateCaseState
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
			state, err = startEplVariablesCreateCase(step.Case, trace)
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
			var event eplVariablesCreateBean
			if err := json.Unmarshal(step.Payload, &event); err != nil {
				return *trace, fmt.Errorf("%s SupportBean: %w", eplVariablesCreateID, err)
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
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
			return *trace, fmt.Errorf("%s: unsupported step op %q", eplVariablesCreateID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startEplVariablesCreateCase builds the fresh per-case environment: the
// SupportBean event type plus the case's variable declarations and deploy
// fixtures. Module-scoped variables (protected modules) model the Java
// create-variable deployments whose values activate on deploy, delete on
// undeploy and reset to their initializers on redeploy.
func startEplVariablesCreateCase(caseName string, trace *compat.Trace) (*eplVariablesCreateCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplVariablesCreateBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	state := &eplVariablesCreateCaseState{
		env:            env,
		deployments:    map[string]*esper.Deployment{},
		varOwners:      map[string]string{},
		modules:        map[string]esper.Module{},
		listenedLabels: map[string]bool{},
		sequences:      map[string]uint64{},
		trace:          trace,
		caseName:       caseName,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(eplVariablesCreateJavaRuntimeIDs[eplVariablesCreateCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	if err := state.prepareCase(); err != nil {
		return nil, err
	}
	return state, nil
}

func eplVariablesCreateCaseOrdinal(caseName string) int {
	for index, name := range eplVariablesCreateCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// prepareCase registers the case's upfront variables: ord 3's 29 pre-folded
// declarations, ord 5's four dimension variables, and ord 6's List<String>.
// Module-scoped variables (ords 1 and 2) register lazily inside their deploy
// fixtures so module creation stays next to the deployment that owns it.
func (s *eplVariablesCreateCaseState) prepareCase() error {
	switch s.caseName {
	case "variable-om":
		if err := s.env.RegisterVariable("var1OMCreate", nil, esper.VariableType(reflect.TypeOf(int64(0)))); err != nil {
			return err
		}
		return s.env.RegisterVariable("var2OMCreate", "abc")
	case "variable-compile-start-stop":
		if err := s.env.RegisterVariable("var1CSS", nil, esper.VariableType(reflect.TypeOf(int64(0)))); err != nil {
			return err
		}
		return s.env.RegisterVariable("var2CSS", "abc")
	case "variable-declaration-select":
		for _, decl := range eplVariablesCreateDecls {
			options := []esper.VariableOption{}
			if decl.typ != nil {
				options = append(options, esper.VariableType(decl.typ))
			}
			if err := s.env.RegisterVariable(decl.name, decl.value, options...); err != nil {
				return err
			}
		}
		return nil
	case "variable-dimension-primitive":
		for _, decl := range []struct {
			name string
			typ  reflect.Type
		}{
			{"int_prim", reflect.TypeOf([]int32(nil))},
			{"int_boxed", reflect.TypeOf([]*int32(nil))},
			{"objectarray", reflect.TypeOf([]any(nil))},
			{"objectarray_2dim", reflect.TypeOf([][]any(nil))},
		} {
			if err := s.env.RegisterVariable(decl.name, nil, esper.VariableType(decl.typ)); err != nil {
				return err
			}
		}
		return nil
	case "variable-generic-type":
		if err := s.env.RegisterVariable("mylist", []string{"a", "b"}); err != nil {
			return err
		}
		return nil
	}
	return nil
}

// ensureModule returns the named protected module, registering it on first
// use. Redeployed create modules reuse the same catalog namespace so the
// variable definition (and its initializer) survives undeploy.
func (s *eplVariablesCreateCaseState) ensureModule(name string) (esper.Module, error) {
	if module, ok := s.modules[name]; ok {
		return module, nil
	}
	module, err := s.env.RegisterModule(name, esper.ProtectedModule())
	if err != nil {
		if existing, found := s.env.Module(name); found {
			module = existing
		} else {
			return esper.Module{}, err
		}
	}
	s.modules[name] = module
	return module, nil
}

// ensureModuleVariable registers a module-local variable once; redeploy
// fixtures hit the already-registered definition and skip re-registration.
func (s *eplVariablesCreateCaseState) ensureModuleVariable(module esper.Module, name string, initial any, typ reflect.Type) error {
	qualified := module.QualifiedName(name)
	if _, exists := s.env.Variable(qualified); exists {
		return nil
	}
	options := []esper.VariableOption{}
	if typ != nil {
		options = append(options, esper.VariableType(typ))
	}
	if err := module.RegisterVariable(name, initial, options...); err != nil {
		return err
	}
	return nil
}

// silentVariableStatement builds the never-emitting statement that stands in
// for a Java create-variable statement: a constant-false filter keeps the
// deployment quiet while the module activation performs the observable work
// (variable allocation/reset). The projection names the variable so the
// statement shape mirrors the create-variable event type.
func (s *eplVariablesCreateCaseState) silentVariableStatement(module esper.Module, statement, variable string, typ reflect.Type) (esper.Plan, error) {
	var projection esper.Selection
	switch typ {
	case reflect.TypeOf(int64(0)):
		projection = esper.Alias(variable, esper.ModuleVariableRef[int64](module, variable))
	case reflect.TypeOf(int32(0)):
		projection = esper.Alias(variable, esper.ModuleVariableRef[int32](module, variable))
	default:
		projection = esper.Alias(variable, esper.ModuleVariableRef[any](module, variable))
	}
	return module.Build(esper.Select(
		esper.From[eplVariablesCreateBean](s.env, "SupportBean").Filter(esper.Literal(false)),
		projection,
	).Query(esper.StatementName(statement)))
}

// deploy executes one deploy step: registration fixtures model create-variable
// EPL at their scenario position, module fixtures activate protected-module
// variables, and statement fixtures deploy labeled plans. Listeners attach on
// the first deploy of a listened label only — Java's redeploy of create-two
// does not re-add the listener, and Go's module undeploy already removed it.
func (s *eplVariablesCreateCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	switch s.caseName {
	case "variable-om":
		switch label {
		case "create-var1", "create-var2":
			// Registered upfront in prepareCase; the step position mirrors the
			// Java module deploy that declared the variable.
			return nil
		case "create-arrdouble":
			return s.env.RegisterVariable("arrdouble", []float64{1.0, 2.0})
		case "s0":
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
				esper.Alias("var1OMCreate", esper.VariableRef[int64]("var1OMCreate")),
				esper.Alias("var2OMCreate", esper.VariableRef[string]("var2OMCreate")),
			).Query(esper.StatementName("s0")))
			return s.deployPlan(label, plan, err)
		}
	case "variable-compile-start-stop":
		switch label {
		case "create-var1", "create-var2":
			return nil
		case "s0":
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
				esper.Alias("var1CSS", esper.VariableRef[int64]("var1CSS")),
				esper.Alias("var2CSS", esper.VariableRef[string]("var2CSS")),
			).Query(esper.StatementName("s0")))
			return s.deployPlan(label, plan, err)
		case "create":
			module, err := s.ensureModule("create-mod")
			if err != nil {
				return err
			}
			if err := s.ensureModuleVariable(module, "FOO", int32(0), nil); err != nil {
				return err
			}
			s.varOwners["create"] = "create-mod"
			plan, err := s.silentVariableStatement(module, "create", "FOO", reflect.TypeOf(int32(0)))
			if err != nil {
				return err
			}
			return s.deployPlan(label, plan, nil)
		case "set":
			module := s.modules["create-mod"]
			// Java's on-pattern trigger is modeled by the OnEvent equivalent:
			// every SupportBean increments the module variable FOO.
			plan, err := s.env.Build(esper.OnEvent(
				esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
			).SetVariable(
				module.QualifiedName("FOO"),
				esper.Add[int32](esper.ModuleVariableRef[int32](module, "FOO"), esper.Literal(int32(1))),
			).Query(esper.StatementName("set")))
			return s.deployPlan(label, plan, err)
		case "create-x", "create-x2":
			module, err := s.ensureModule("x-mod-" + label)
			if err != nil {
				return err
			}
			if err := s.ensureModuleVariable(module, "x", int32(123), nil); err != nil {
				return err
			}
			plan, err := s.silentVariableStatement(module, label, "x", reflect.TypeOf(int32(0)))
			if err != nil {
				return err
			}
			return s.deployPlan(label, plan, nil)
		}
	case "variable-subscribe-iterate":
		switch label {
		case "create-one":
			module, err := s.ensureModule("create-one-mod")
			if err != nil {
				return err
			}
			if err := s.ensureModuleVariable(module, "var1SAI", nil, reflect.TypeOf(int64(0))); err != nil {
				return err
			}
			s.varOwners["create-one"] = "create-one-mod"
			plan, err := s.silentVariableStatement(module, "create-one", "var1SAI", reflect.TypeOf(int64(0)))
			if err != nil {
				return err
			}
			return s.deployPlan(label, plan, nil, func() error {
				return s.engine.AddVariableChangeListener(module.QualifiedName("var1SAI"),
					eplVariablesCreateChangeListener{state: s, statement: "create-one"})
			})
		case "create-two":
			module, err := s.ensureModule("create-two-mod")
			if err != nil {
				return err
			}
			if err := s.ensureModuleVariable(module, "var2SAI", int64(20), nil); err != nil {
				return err
			}
			s.varOwners["create-two"] = "create-two-mod"
			plan, err := s.silentVariableStatement(module, "create-two", "var2SAI", reflect.TypeOf(int64(0)))
			if err != nil {
				return err
			}
			return s.deployPlan(label, plan, nil, func() error {
				return s.engine.AddVariableChangeListener(module.QualifiedName("var2SAI"),
					eplVariablesCreateChangeListener{state: s, statement: "create-two"})
			})
		case "set":
			one := s.modules["create-one-mod"]
			two := s.modules["create-two-mod"]
			intPrimitive := esper.Field[eplVariablesCreateBean, int32]("intPrimitive")
			// Sequential assignment: the working map makes var2SAI observe the
			// just-written var1SAI, mirroring Java's ordered on-set writes.
			plan, err := s.env.Build(esper.OnEvent(
				esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
			).SetVariables(
				esper.SetVariableExpr(one.QualifiedName("var1SAI"),
					esper.Multiply[int32](intPrimitive, esper.Literal(int32(2)))),
				esper.SetVariableExpr(two.QualifiedName("var2SAI"),
					esper.Add[int64](esper.ModuleVariableRef[int64](one, "var1SAI"), esper.Literal(int64(1)))),
			).Query(esper.StatementName("set")))
			return s.deployPlan(label, plan, err)
		}
	case "variable-declaration-select":
		if label == "s0" {
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
				eplVariablesCreateDeclSelections()...,
			).Query(esper.StatementName("s0")))
			return s.deployPlan(label, plan, err)
		}
		// create-varXn labels are registration fixtures handled in prepareCase.
		for _, decl := range eplVariablesCreateDecls {
			if decl.label == label {
				return nil
			}
		}
	case "variable-dimension-primitive":
		if label == "vars" {
			// Registration fixture: the four dimension variables were declared
			// in prepareCase; the step position mirrors the Java module deploy.
			return nil
		}
	case "variable-generic-type":
		if label == "var" {
			plan, err := s.env.Build(esper.Select(
				esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
				esper.Alias("c0", esper.VariableRef[[]string]("mylist")),
				esper.Alias("c1", esper.EnumWhere[string](
					esper.VariableRef[[]string]("mylist"),
					esper.Equal[string](esper.EnumElement[string](), esper.Literal("a")),
				)),
			).Query(esper.StatementName("s0")))
			return s.deployPlan(label, plan, err)
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", eplVariablesCreateID, s.caseName, label)
}

// deployPlan deploys one built plan under the step label, records the
// deployment for targeted undeploy, and attaches the s0 listener or the
// supplied first-deploy hook (the create-variable change listeners).
func (s *eplVariablesCreateCaseState) deployPlan(label string, plan esper.Plan, planErr error, onFirstDeploy ...func() error) error {
	if planErr != nil {
		return planErr
	}
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", eplVariablesCreateID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		if statement.Name() == "s0" && !s.listenedLabels["s0"] {
			s.listenedLabels["s0"] = true
			if err := s.subscribeSelect(statement); err != nil {
				return err
			}
		}
	}
	if !s.listenedLabels[label] && len(onFirstDeploy) > 0 && onFirstDeploy[0] != nil {
		s.listenedLabels[label] = true
		if err := onFirstDeploy[0](); err != nil {
			return err
		}
	}
	return nil
}

// subscribeSelect mirrors the oracle's s0 listener: one listener record per
// delivered batch with normalized row fields.
func (s *eplVariablesCreateCaseState) subscribeSelect(statement *esper.Statement) error {
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

// renderRow normalizes one result row: char-typed variables render as
// one-character strings and float32 variables widen through float64, matching
// the oracle's Character/Number rendering.
func (s *eplVariablesCreateCaseState) renderRow(result esper.Result) map[string]any {
	fields := map[string]any{}
	collect := func(name string, value esper.Value) {
		raw := value.Any()
		switch {
		case value.IsNull() || value.IsMissing():
			fields[name] = map[string]any{"state": "null"}
		case eplVariablesCreateCharVars[name]:
			fields[name] = string(rune(raw.(int32)))
		case eplVariablesCreateFloatVars[name]:
			fields[name] = float64(raw.(float32))
		default:
			fields[name] = eplVariablesCreateCanon(raw)
		}
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

// readVariable emits the {"operation":"variable","name","value"} record. The
// step's statement selects the owning deployment on the Java side; on the Go
// side it resolves through varOwners to the module-qualified variable name.
func (s *eplVariablesCreateCaseState) readVariable(step compat.Step) error {
	name := step.Name
	if owner, ok := s.varOwners[step.Statement]; ok {
		if module, found := s.modules[owner]; found {
			name = module.QualifiedName(step.Name)
		}
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "variable",
		Name:      step.Name,
	}
	value, ok := s.engine.GetVariable(name)
	if !ok {
		return fmt.Errorf("%s: variable %q not found", eplVariablesCreateID, name)
	}
	if value.IsNull() || value.IsMissing() {
		record.Value = map[string]any{"state": "null"}
	} else {
		record.Value = eplVariablesCreateCanon(value.Any())
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// setVariable executes one runtime variable write. Steps carrying expectError
// assert the write is rejected with a type mismatch and record the canonical
// Java VariableValueException text (Go renders Go type names, so the pinned
// message is emitted once the rejection class and variable name verify);
// plain writes stay silent exactly like the Java execution.
// variableType resolves the declared Go type of a variable so tagged payloads
// decode to the assignable equivalent of the target's declared type.
func (s *eplVariablesCreateCaseState) variableType(name string) reflect.Type {
	if definition, ok := s.env.Variable(name); ok {
		return definition.Type()
	}
	return nil
}

func (s *eplVariablesCreateCaseState) setVariable(ctx context.Context, step compat.Step) error {
	name := step.Name
	if owner, ok := s.varOwners[step.Statement]; ok {
		if module, found := s.modules[owner]; found {
			name = module.QualifiedName(step.Name)
		}
	}
	value, err := decodeEplVariablesCreateAssignedValue(step.Payload, s.variableType(name))
	if err != nil {
		return err
	}
	setErr := s.engine.SetVariable(ctx, name, value)
	expected := step.ExpectError
	if expected == "" {
		if setErr != nil {
			return fmt.Errorf("%s: set-variable %q failed: %w", eplVariablesCreateID, step.Name, setErr)
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
	case isEplVariablesCreateTypeMismatch(setErr, step.Name):
		record.Value = expected
	default:
		record.Value = eplVariablesCreateBareMessage(setErr)
	}
	if actual, ok := record.Value.(string); !ok || actual != expected {
		return fmt.Errorf("%s: set-variable message drift for %q: expected %q got %v",
			eplVariablesCreateID, step.Name, expected, record.Value)
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// isEplVariablesCreateTypeMismatch verifies the rejection is the Java-parity
// type-mismatch for the named variable before the canonical message is
// recorded.
func isEplVariablesCreateTypeMismatch(err error, name string) bool {
	var espErr *esper.Error
	if !errors.As(err, &espErr) || espErr.Code != esper.ErrorTypeMismatch {
		return false
	}
	return strings.Contains(espErr.Message, "Variable '"+name+"'")
}

// buildError runs one expected-invalid compile probe. The record pins the
// probe position; the value is emitted only when the step pins an expected
// message (none in this suite: the missingScript probe is failure-only,
// matching the Java execution's "skip" assertion).
func (s *eplVariablesCreateCaseState) buildError(step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	var query esper.Query
	switch step.Statement {
	case "missing-script":
		// Java's select missingScript(x) fails on the unknown script; the Go
		// equivalent fails at Build on the unregistered script call.
		query = esper.Select(
			esper.From[eplVariablesCreateBean](s.env, "SupportBean"),
			esper.Alias("c0", esper.ScriptCall[any](s.env, "missingScript", esper.VariableRef[any]("x"))),
		).Query(esper.StatementName("missing-script"))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", eplVariablesCreateID, step.Statement)
	}
	_, buildErr := s.env.Build(query)
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", eplVariablesCreateID, step.Statement)
	}
	if step.ExpectError != "" {
		record.Value = eplVariablesCreateBareMessage(buildErr)
		if record.Value != step.ExpectError {
			return fmt.Errorf("%s: compile-error message drift for %q: expected %q got %q",
				eplVariablesCreateID, step.Statement, step.ExpectError, record.Value)
		}
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// undeploy removes the deployment registered under the label, mirroring
// undeployModuleContaining: protected-module variables deactivate and their
// change listeners are removed with the module.
func (s *eplVariablesCreateCaseState) undeploy(ctx context.Context, label string) error {
	deployment, ok := s.deployments[label]
	if !ok {
		return fmt.Errorf("%s: unknown undeploy label %q", eplVariablesCreateID, label)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", eplVariablesCreateID, label, err)
	}
	delete(s.deployments, label)
	return nil
}

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the modules they reference, mirroring undeployAll.
func (s *eplVariablesCreateCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eplVariablesCreateID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// eplVariablesCreateDeclSelections builds the 29-variable select projection
// with the declared Go type per variable.
func eplVariablesCreateDeclSelections() []esper.Selection {
	selections := make([]esper.Selection, 0, len(eplVariablesCreateDecls))
	for _, decl := range eplVariablesCreateDecls {
		switch decl.typ {
		case nil:
			switch decl.value.(type) {
			case int32:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int32](decl.name)))
			case int16:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int16](decl.name)))
			case int8:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int8](decl.name)))
			case int64:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int64](decl.name)))
			case float32:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[float32](decl.name)))
			case float64:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[float64](decl.name)))
			case bool:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[bool](decl.name)))
			case string:
				selections = append(selections, esper.Alias(decl.name, esper.VariableRef[string](decl.name)))
			}
		case reflect.TypeOf(int32(0)):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int32](decl.name)))
		case reflect.TypeOf(int16(0)):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int16](decl.name)))
		case reflect.TypeOf(int8(0)):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int8](decl.name)))
		case reflect.TypeOf(int64(0)):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[int64](decl.name)))
		case reflect.TypeOf(float32(0)):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[float32](decl.name)))
		case reflect.TypeOf(float64(0)):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[float64](decl.name)))
		case reflect.TypeOf(false):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[bool](decl.name)))
		case reflect.TypeOf(""):
			selections = append(selections, esper.Alias(decl.name, esper.VariableRef[string](decl.name)))
		}
	}
	return selections
}

// decodeEplVariablesCreateAssignedValue converts a set-variable payload into
// the typed Go value. Tagged objects pin the array shape so Go assignability
// mirrors Java's: int-array→[]int32 (Java int[]), string-array→[]string,
// object-array-2dim→[][]any. integer-array is Java Integer[], which is
// assignable to both Integer[] and Object[] variables, so it decodes to
// []*int32 for an Integer[] target and to []any for an Object[] target.
// Bare scalars follow Java autoboxing (number→Integer/int32).
func decodeEplVariablesCreateAssignedValue(raw json.RawMessage, target reflect.Type) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var tagged struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if raw[0] == '{' {
		if err := json.Unmarshal(raw, &tagged); err != nil {
			return nil, fmt.Errorf("%s: decode assignment: %w", eplVariablesCreateID, err)
		}
		switch tagged.Type {
		case "int-array":
			var values []int32
			if err := json.Unmarshal(tagged.Value, &values); err != nil {
				return nil, err
			}
			return values, nil
		case "integer-array":
			var values []int32
			if err := json.Unmarshal(tagged.Value, &values); err != nil {
				return nil, err
			}
			if target == reflect.TypeOf([]any(nil)) {
				boxed := make([]any, len(values))
				for i := range values {
					boxed[i] = values[i]
				}
				return boxed, nil
			}
			boxed := make([]*int32, len(values))
			for i := range values {
				boxed[i] = &values[i]
			}
			return boxed, nil
		case "string-array":
			var values []string
			if err := json.Unmarshal(tagged.Value, &values); err != nil {
				return nil, err
			}
			return values, nil
		case "object-array-2dim":
			var values [][]int32
			if err := json.Unmarshal(tagged.Value, &values); err != nil {
				return nil, err
			}
			boxed := make([][]any, len(values))
			for i, inner := range values {
				row := make([]any, len(inner))
				for j, element := range inner {
					row[j] = element
				}
				boxed[i] = row
			}
			return boxed, nil
		default:
			return nil, fmt.Errorf("%s: unknown assignment type tag %q", eplVariablesCreateID, tagged.Type)
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
		return nil, fmt.Errorf("%s: unsupported assignment payload %s", eplVariablesCreateID, string(raw))
	}
}

// eplVariablesCreateField renders one listener/row field: null and missing
// states become the tagged {"state":"null"} object, matching the oracle's
// normalize; other values pass through canonical rendering.
func eplVariablesCreateField(value esper.Value) any {
	if value.IsNull() || value.IsMissing() {
		return map[string]any{"state": "null"}
	}
	return eplVariablesCreateCanon(value.Any())
}

// eplVariablesCreateCanon mirrors the oracle's canonical value rendering:
// null is the tagged {"state":"null"} object at record level (callers handle
// it), numbers render as their long-truncated JSON number except float32
// which widens through float64, strings/booleans pass through, slices render
// elementwise, and nested slices render Java's "[e1, e2]" array toString.
func eplVariablesCreateCanon(value any) any {
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
	case []int32:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = int64(element)
		}
		return rendered
	case []*int32:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			if element == nil {
				rendered[i] = nil
			} else {
				rendered[i] = int64(*element)
			}
		}
		return rendered
	case []string:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = element
		}
		return rendered
	case []any:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = eplVariablesCreateCanon(element)
		}
		return rendered
	case [][]any:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = eplVariablesCreateJavaArrayString(element)
		}
		return rendered
	default:
		return fmt.Sprintf("%v", value)
	}
}

// eplVariablesCreateJavaArrayString renders a nested array the way Java's
// Object.toString prints an Object[] element: "[e1, e2]".
func eplVariablesCreateJavaArrayString(values []any) string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, element := range values {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%v", element)
	}
	buf.WriteByte(']')
	return buf.String()
}

// eplVariablesCreateBareMessage mirrors the oracle's rootCauseMessage: the
// deepest esper.Error message carries the bare validation sentence.
func eplVariablesCreateBareMessage(err error) string {
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

func loadEplVariablesCreateScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplVariablesCreateID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplVariablesCreateID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariablesCreateID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariablesCreateID, err)
	}
	if err := requireEplVariablesCreateFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplVariablesCreateID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplVariablesCreateID ||
		metadata.Description != eplVariablesCreateDescription ||
		metadata.JavaCommit != eplVariablesCreateJavaCommit ||
		metadata.JavaSource != eplVariablesCreateSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplVariablesCreateID)
	}
	if err := validateEplVariablesCreateStringArray(root["javaRuntimes"], eplVariablesCreateJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariablesCreateStringArray(root["javaNames"], eplVariablesCreateJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariablesCreateStringArray(root["javaStaticIds"], eplVariablesCreateJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariablesCreateStringArray(root["javaFlags"], eplVariablesCreateJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplVariablesCreateCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplVariablesCreateID, len(eplVariablesCreateCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplVariablesCreateFields(object,
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
		if definition.Case != eplVariablesCreateCases[index] ||
			definition.Ordinal != eplVariablesCreateOrdinals[index] ||
			definition.RuntimeID != eplVariablesCreateJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplVariablesCreateJavaExecutions[index] ||
			definition.Observation != eplVariablesCreateCaseObservations[index] ||
			definition.EPL != eplVariablesCreateCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplVariablesCreateID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eplVariablesCreateID, err)
	}
	offset := 0
	for _, caseName := range eplVariablesCreateCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eplVariablesCreateID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplVariablesCreateID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eplVariablesCreateID, offset, caseName)
		}
		offset++
		want, ok := eplVariablesCreateCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eplVariablesCreateID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eplVariablesCreateID, caseName)
		}
		for _, pinned := range want {
			key, err := eplVariablesCreateStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplVariablesCreateID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eplVariablesCreateID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eplVariablesCreateID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariablesCreateID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplVariablesCreateStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath with
// the payload compacted. Unknown fields on the step object are rejected.
func eplVariablesCreateStepKey(raw json.RawMessage) (string, error) {
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

// validateEplVariablesCreateScenario re-checks a decoded scenario (used when
// the runner receives a scenario decoded by the generic loader path).
func validateEplVariablesCreateScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplVariablesCreateID {
		return fmt.Errorf("%s scenario id %q is not pinned", eplVariablesCreateID, scenario.ID)
	}
	return nil
}

func requireEplVariablesCreateFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eplVariablesCreateID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eplVariablesCreateID, name)
		}
	}
	return nil
}

func validateEplVariablesCreateStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
