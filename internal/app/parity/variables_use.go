package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type variablesUseBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// supportEnum mirrors Java SupportEnum: the equality-only named constants
// ENUM_VALUE_1..3 carried by SupportBean.enumValue and by enum variables.
type supportEnum int32

const (
	supportEnumValue1 supportEnum = iota + 1
	supportEnumValue2
	supportEnumValue3
)

func (s supportEnum) String() string {
	switch s {
	case supportEnumValue1:
		return "ENUM_VALUE_1"
	case supportEnumValue2:
		return "ENUM_VALUE_2"
	case supportEnumValue3:
		return "ENUM_VALUE_3"
	default:
		return fmt.Sprintf("SUPPORT_ENUM_%d", int32(s))
	}
}

// UnmarshalJSON accepts the scenario spelling "ENUM_VALUE_N".
func (s *supportEnum) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("variables-use enum: %w", err)
	}
	switch name {
	case "ENUM_VALUE_1":
		*s = supportEnumValue1
	case "ENUM_VALUE_2":
		*s = supportEnumValue2
	case "ENUM_VALUE_3":
		*s = supportEnumValue3
	default:
		return fmt.Errorf("variables-use enum: unknown value %q", name)
	}
	return nil
}

// variablesUseDateMarker stands in for the nondeterministic START_TIME Date
// constant of ESPER-653: reads render the fixed "<date>" marker on both sides,
// never wall-clock values.
type variablesUseDateMarker struct{}

// variablesUseAPIBean models SupportBean for the ep-runtime case; theString
// stays nullable so the on-set statement can assign variable null.
type variablesUseAPIBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int32   `esper:"intPrimitive"`
}

// variablesUseFullBean mirrors every SupportBean property so select * rows
// carry exactly the alphabetical 20-name field set the Java oracle records.
type variablesUseFullBean struct {
	TheString       *string      `esper:"theString"`
	BoolPrimitive   bool         `esper:"boolPrimitive"`
	IntPrimitive    int32        `esper:"intPrimitive"`
	LongPrimitive   int64        `esper:"longPrimitive"`
	CharPrimitive   int32        `esper:"charPrimitive"`
	ShortPrimitive  int16        `esper:"shortPrimitive"`
	BytePrimitive   int8         `esper:"bytePrimitive"`
	FloatPrimitive  float32      `esper:"floatPrimitive"`
	DoublePrimitive float64      `esper:"doublePrimitive"`
	BoolBoxed       *bool        `esper:"boolBoxed"`
	IntBoxed        *int32       `esper:"intBoxed"`
	LongBoxed       *int64       `esper:"longBoxed"`
	CharBoxed       *int32       `esper:"charBoxed"`
	ShortBoxed      *int16       `esper:"shortBoxed"`
	ByteBoxed       *int8        `esper:"byteBoxed"`
	FloatBoxed      *float32     `esper:"floatBoxed"`
	DoubleBoxed     *float64     `esper:"doubleBoxed"`
	EnumValue       *supportEnum `esper:"enumValue"`
	BigDecimal      *big.Rat     `esper:"bigDecimal"`
	BigInteger      *big.Int     `esper:"bigInteger"`
}

type variablesUseS0 struct {
	ID  int32  `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

var variablesUseJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesUse.java",
}

var (
	variablesUseJavaRuntimeIDs = []string{
		"java-runtime-5a38cfa84dadd7dd6f61",
		"java-runtime-849ebec4996c28823d57",
		"java-runtime-5a91cdbc149502c6fb7a",
		"java-runtime-eb01093e6db83f057d77",
		"java-runtime-8457cbd989256d935b22",
		"java-runtime-4373a1c6d8c903357942",
		"java-runtime-80cb4763680bc91e38ce",
		"java-runtime-826b551e883c9398df67",
		"java-runtime-d273a38f6415e6c3ee62",
	}
	variablesUseJavaExecutions = []string{
		"EPLVariableUseVariableInFilter",
		"EPLVariableUseVariableInFilterBoolean",
		"EPLVariableUseSimpleSameModule",
		"EPLVariableUseSimplePreconfigured",
		"EPLVariableUseSimpleTwoModules",
		"EPLVariableUseInvokeMethod",
		"EPLVariableUseFilterConstantCustomTypePreconfigured",
		"EPLVariableUseEPRuntime",
		"EPLVariableUseConstantVariable",
	}
)

// runVariablesUseScenario replays variable-use scenarios: variables assigned
// by on-set statements and consumed inside equality / or filters.
func runVariablesUseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"variable-in-filter", "variable-in-filter-boolean", "simple-same-module", "simple-preconfigured", "simple-two-modules", "invoke-method", "filter-constant-custom-type", "ep-runtime", "constant-variable"}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runVariablesUseCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("variables-use case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("variables-use scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runVariablesUseCase(ctx context.Context, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	// Cases with a dedicated SupportBean shape register it in their own
	// branch; the shared default covers the legacy filter scenarios.
	if caseName != "ep-runtime" && caseName != "constant-variable" {
		if _, err := esper.RegisterStruct[variablesUseBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	if _, err := esper.RegisterStruct[variablesUseS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	switch caseName {
	case "variable-in-filter":
		// Java registers var1IF as String with null initial; the Go runner
		// uses an unreachable sentinel so equality never matches before the
		// first assignment (established precedent).
		if err := env.RegisterVariable("var1IF", "\x00UNSET"); err != nil {
			return compat.Trace{}, err
		}
		p00 := esper.Field[variablesUseS0, string]("p00")
		setQuery := esper.OnEvent(esper.From[variablesUseS0](env, "SupportBean_S0")).SetVariable(
			"var1IF", p00,
		).Query(esper.StatementName("set"))
		theString := esper.Field[variablesUseBean, string]("theString")
		num := esper.Field[variablesUseBean, int32]("intPrimitive")
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean").Filter(
				esper.Equal[string](theString, esper.VariableRef[string]("var1IF")),
			),
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", num),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{setQuery, query}, caseScenario, caseName)
	case "variable-in-filter-boolean":
		if err := env.RegisterVariable("var1IFB", "\x00UNSET"); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2IFB", "\x00UNSET"); err != nil {
			return compat.Trace{}, err
		}
		p00 := esper.Field[variablesUseS0, string]("p00")
		p01 := esper.Field[variablesUseS0, string]("p01")
		setQuery := esper.OnEvent(esper.From[variablesUseS0](env, "SupportBean_S0")).SetVariables(
			esper.SetVariableExpr("var1IFB", p00),
			esper.SetVariableExpr("var2IFB", p01),
		).Query(esper.StatementName("set"))
		theString := esper.Field[variablesUseBean, string]("theString")
		num := esper.Field[variablesUseBean, int32]("intPrimitive")
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean").Filter(
				esper.Or(
					esper.Equal[string](theString, esper.VariableRef[string]("var1IFB")),
					esper.Equal[string](theString, esper.VariableRef[string]("var2IFB")),
				),
			),
			esper.Alias("theString", theString),
			esper.Alias("intPrimitive", num),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{setQuery, query}, caseScenario, caseName)
	case "simple-same-module":
		if err := env.RegisterVariable("var_simple_module_const", true); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", esper.VariableRef[bool]("var_simple_module_const")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "simple-preconfigured":
		if err := env.RegisterVariable("var_simple_preconfig_const", true, esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", esper.VariableRef[bool]("var_simple_preconfig_const")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "simple-two-modules":
		// Module A's @public create-variable is modeled as the environment
		// registration (the established representation of `create variable`),
		// which module B's deployment then reads; Java pins @public + path.
		if err := env.RegisterVariable("var_simple_twomodule_const", true); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", esper.VariableRef[bool]("var_simple_twomodule_const")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "invoke-method":
		// Constant variable initialized from the factory plus the
		// preconfigured instance variable; dot-invocation is represented by
		// function expressions over the variable-hosted values.
		if err := env.RegisterVariable("myService", variablesUseServiceFactory().makeService(), esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("myInitService", variablesUseServiceFactory().makeService()); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseBean](env, "SupportBean"),
			esper.Alias("c0", variablesUseDoSomething("myService")),
			esper.Alias("c1", variablesUseDoSomething("myInitService")),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "filter-constant-custom-type":
		if _, err := esper.RegisterStruct[variablesUseCustomEvent](env, "MyVariableCustomEvent"); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("my_variable_custom_typed", variablesUseCustomType{Value: "abc"}, esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		query = esper.Select(
			esper.From[variablesUseCustomEvent](env, "MyVariableCustomEvent").Filter(
				esper.Equal[variablesUseCustomType](
					esper.Field[variablesUseCustomEvent, variablesUseCustomType]("name"),
					esper.VariableRef[variablesUseCustomType]("my_variable_custom_typed"),
				),
			),
		).Query(esper.StatementName("s0"))
		return replayVariablesUse(ctx, env, []esper.Query{query}, caseScenario, caseName)
	case "ep-runtime":
		if _, err := esper.RegisterStruct[variablesUseAPIBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		// Preconfigured suite variables (TestSuiteEPLVariable.configure):
		// var1 int init -1, var2 String init "abc".
		if err := env.RegisterVariable("var1", int32(-1)); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("var2", "abc"); err != nil {
			return compat.Trace{}, err
		}
		intPrimitive := esper.Field[variablesUseAPIBean, int32]("intPrimitive")
		theString := esper.Field[variablesUseAPIBean, string]("theString")
		setQuery := esper.OnEvent(esper.From[variablesUseAPIBean](env, "SupportBean")).SetVariables(
			esper.SetVariableExpr("var1", intPrimitive),
			esper.SetVariableExpr("var2", theString),
		).Query(esper.StatementName("set"))
		fixtures := variablesUseFixtures{
			registrations: map[string]func(*esper.Environment) error{
				// Mid-case representation of the create-on-the-fly module
				// `@name('create') create variable int dummy = 20 + 20`:
				// the folded initializer registers at its scenario position,
				// which keeps the earlier unknown-name failures honest.
				"create-dummy": func(env *esper.Environment) error {
					return env.RegisterVariable("dummy", int32(40))
				},
			},
		}
		return replayVariablesUseWithOptions(ctx, env, []esper.Query{setQuery}, caseScenario, caseName, variablesUseReplayOptions{
			decode:      decodeVariablesUseAPIPayload,
			renderField: func(_ string, value any) any { return variablesUseRender(value) },
			fixtures:    fixtures,
		})
	case "constant-variable":
		if _, err := esper.RegisterStruct[variablesUseFullBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		// Preconfigured constants (TestSuiteEPLVariable.configure):
		// MYCONST_TWO String null constant, MYCONST_THREE boolean true.
		if err := env.RegisterVariable("MYCONST_TWO", nil, esper.VariableType(reflect.TypeOf("")), esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		if err := env.RegisterVariable("MYCONST_THREE", true, esper.ConstantVariable()); err != nil {
			return compat.Trace{}, err
		}
		myconst := esper.VariableRef[int32]("MYCONST")
		intBoxed := esper.Field[variablesUseFullBean, int32]("intBoxed")
		intPrimitive := esper.Field[variablesUseFullBean, int32]("intPrimitive")
		shortBoxed := esper.Field[variablesUseFullBean, int16]("shortBoxed")
		enumValueField := esper.Field[variablesUseFullBean, supportEnum]("enumValue")
		theStringField := esper.Field[variablesUseFullBean, string]("theString")
		shortAsInt := esper.Cast[int16, int32](shortBoxed)
		operator := func(filter esper.Expression[bool]) func(*esper.Environment) (esper.Query, error) {
			return func(*esper.Environment) (esper.Query, error) {
				return esper.Select(
					esper.From[variablesUseFullBean](env, "SupportBean").Filter(filter),
					esper.Alias("c0", theStringField),
					esper.Alias("c1", intPrimitive),
				).Query(esper.StatementName("s0")), nil
			}
		}
		builders := map[string]func(*esper.Environment) (esper.Query, error){
			"op-const-eq-int":         operator(esper.Equal[int32](myconst, intBoxed)),
			"op-const-gt-int":         operator(esper.Greater[int32](myconst, intBoxed)),
			"op-const-ge-int":         operator(esper.GreaterOrEqual[int32](myconst, intBoxed)),
			"op-const-lt-int":         operator(esper.Less[int32](myconst, intBoxed)),
			"op-const-le-int":         operator(esper.LessOrEqual[int32](myconst, intBoxed)),
			"op-int-lt-const":         operator(esper.Less[int32](intBoxed, myconst)),
			"op-int-le-const":         operator(esper.LessOrEqual[int32](intBoxed, myconst)),
			"op-int-gt-const":         operator(esper.Greater[int32](intBoxed, myconst)),
			"op-int-ge-const":         operator(esper.GreaterOrEqual[int32](intBoxed, myconst)),
			"op-int-in-const":         operator(esper.In[int32](intBoxed, myconst)),
			"op-int-between-const":    operator(esper.Between[int32](intBoxed, myconst, myconst)),
			"op-const-ne-int":         operator(esper.NotEqual[int32](myconst, intBoxed)),
			"op-int-ne-const":         operator(esper.NotEqual[int32](intBoxed, myconst)),
			"op-int-notin-const":      operator(esper.Not(esper.In[int32](intBoxed, myconst))),
			"op-int-notbetween-const": operator(esper.Not(esper.Between[int32](intBoxed, myconst, myconst))),
			"op-const-is-int":         operator(esper.Is(myconst, intBoxed)),
			"op-int-is-const":         operator(esper.Is(intBoxed, myconst)),
			"op-const-isnot-int":      operator(esper.IsNot(myconst, intBoxed)),
			"op-int-isnot-const":      operator(esper.IsNot(intBoxed, myconst)),
			"op-const-eq-short":       operator(esper.Equal[int32](myconst, shortAsInt)),
			"op-short-eq-const":       operator(esper.Equal[int32](shortAsInt, myconst)),
			"op-const-gt-short":       operator(esper.Greater[int32](myconst, shortAsInt)),
			"op-short-lt-const":       operator(esper.Less[int32](shortAsInt, myconst)),
			"op-short-in-const":       operator(esper.In[int32](shortAsInt, myconst)),
			"op-int-in-literals":      operator(esper.In[int32](intBoxed, esper.Literal[int32](10), esper.Literal[int32](8))),
			"select-var-strings": func(*esper.Environment) (esper.Query, error) {
				return esper.Select(
					esper.From[variablesUseFullBean](env, "SupportBean"),
					esper.Alias("var_strings", esper.VariableRef[[]string]("var_strings")),
				).Query(esper.StatementName("s0")), nil
			},
			"strings-in-arrayvar": func(*esper.Environment) (esper.Query, error) {
				return esper.Select(
					esper.From[variablesUseFullBean](env, "SupportBean").Filter(
						esper.InSlice[string](theStringField, esper.VariableRef[[]string]("var_strings")),
					),
				).Query(esper.StatementName("s0")), nil
			},
			"op-int-in-varints": operator(esper.InSlice[int32](intBoxed, esper.VariableRef[[]int32]("var_ints"))),
			"op-int-in-varints-two": operator(esper.InOf(intBoxed,
				esper.VariableRef[[]int32]("var_ints"),
				esper.VariableRef[[]int32]("var_intstwo"))),
			"op-enum-const-eq-field": operator(esper.Equal[supportEnum](
				esper.VariableRef[supportEnum]("var_enumone"), enumValueField)),
			// Java pins `enumValue in (var_enumarr, var_enumone)`: one
			// result row per matching candidate slot (ENUM_VALUE_2 matches
			// the array element and the scalar, delivering twice).
			"op-enum-in-arrayconst": operator(esper.InOf(enumValueField,
				esper.VariableRef[[]supportEnum]("var_enumarr"),
				esper.VariableRef[supportEnum]("var_enumone"))),
			"on-set-enumtwo": func(*esper.Environment) (esper.Query, error) {
				return esper.OnEvent(esper.From[variablesUseFullBean](env, "SupportBean")).SetVariable(
					"var_enumtwo", enumValueField,
				).Query(esper.StatementName("on-set-enumtwo")), nil
			},
		}
		registrations := map[string]func(*esper.Environment) error{
			"create-myconst": func(env *esper.Environment) error {
				return env.RegisterVariable("MYCONST", int32(10), esper.ConstantVariable())
			},
			// Java re-creates MYCONST after undeployAll (SODA step); Go
			// variables are environment-scoped so the re-create is a no-op.
			"recreate-myconst": func(*esper.Environment) error { return nil },
			"start-time-create": func(env *esper.Environment) error {
				// ESPER-653: the Date constant's value is nondeterministic;
				// the marker type renders the fixed "<date>" read marker.
				return env.RegisterVariable("START_TIME", variablesUseDateMarker{}, esper.ConstantVariable())
			},
			"create-var-strings": func(env *esper.Environment) error {
				return env.RegisterVariable("var_strings", []string{"E1", "E2"}, esper.ConstantVariable())
			},
			"create-var-ints": func(env *esper.Environment) error {
				return env.RegisterVariable("var_ints", []int32{8, 10}, esper.ConstantVariable())
			},
			"create-var-intstwo": func(env *esper.Environment) error {
				return env.RegisterVariable("var_intstwo", []int32{9}, esper.ConstantVariable())
			},
			"create-var-enumone": func(env *esper.Environment) error {
				return env.RegisterVariable("var_enumone", supportEnumValue2, esper.ConstantVariable())
			},
			"create-var-enumarr": func(env *esper.Environment) error {
				return env.RegisterVariable("var_enumarr", []supportEnum{supportEnumValue2, supportEnumValue1}, esper.ConstantVariable())
			},
			"create-var-enumtwo": func(env *esper.Environment) error {
				return env.RegisterVariable("var_enumtwo", supportEnumValue2)
			},
		}
		probes := map[string]func(*esper.Environment) (esper.Query, error){
			"on-set-const": func(*esper.Environment) (esper.Query, error) {
				return esper.OnEvent(esper.From[variablesUseFullBean](env, "SupportBean")).SetVariable(
					"MYCONST", esper.Literal[int32](10),
				).Query(esper.StatementName("probe-on-set-const")), nil
			},
			"output-rate-const": func(*esper.Environment) (esper.Query, error) {
				return esper.Select(
					esper.From[variablesUseFullBean](env, "SupportBean"),
				).Query(
					esper.StatementName("probe-output-rate"),
					esper.WithOutput(esper.OutputWhen(
						esper.Literal(true),
						esper.SetOutputVariable("MYCONST", esper.Literal[int32](1)),
					)),
				), nil
			},
		}
		return replayVariablesUseWithOptions(ctx, env, nil, caseScenario, caseName, variablesUseReplayOptions{
			decode:      decodeVariablesUseConstantPayload,
			renderField: variablesUseConstantRowField,
			fixtures: variablesUseFixtures{
				builders:      builders,
				probes:        probes,
				registrations: registrations,
			},
		})
	default:
		return compat.Trace{}, fmt.Errorf("unsupported variables-use case %q", caseName)
	}
}

// variablesUseService mirrors MySimpleVariableService: the dot-call surface
// hosts a hello responder on a variable.
type variablesUseService struct{}

func (variablesUseService) doSomething() string { return "hello" }

type variablesUseServiceFactoryType struct{}

func (variablesUseServiceFactoryType) makeService() variablesUseService {
	return variablesUseService{}
}

func variablesUseServiceFactory() variablesUseServiceFactoryType {
	return variablesUseServiceFactoryType{}
}

// variablesUseDoSomething reads the variable-hosted service inside the
// evaluation context and invokes it, mirroring the Java dot-call. A missing
// or mistyped variable degrades to the empty string here (Java fails at
// deploy); registration drift surfaces as a trace mismatch.
func variablesUseDoSomething(variableName string) esper.Expression[string] {
	return esper.Func1Ctx[esper.Event, string]("variable-dot-"+variableName,
		func(event esper.Event, ctx esper.EvalContext) string {
			if ctx.Variables == nil {
				return ""
			}
			value, ok := ctx.Variables[variableName]
			if !ok {
				return ""
			}
			if service, ok := value.Any().(variablesUseService); ok {
				return service.doSomething()
			}
			return ""
		}, esper.EventValue[esper.Event]())
}

// variablesUseCustomType mirrors MyVariableCustomType's value equality.
type variablesUseCustomType struct {
	Value string `esper:"value"`
}

type variablesUseCustomEvent struct {
	Name variablesUseCustomType `esper:"name"`
}

// variablesUseFixtures carries the label-keyed mid-case deployments of the two
// runtime-API cases: registration fixtures model create-variable EPL at their
// scenario position, builders deploy labeled statements, and probes assert
// compile failures for build-error steps.
type variablesUseFixtures struct {
	builders      map[string]func(*esper.Environment) (esper.Query, error)
	probes        map[string]func(*esper.Environment) (esper.Query, error)
	registrations map[string]func(*esper.Environment) error
}

func (f variablesUseFixtures) empty() bool {
	return len(f.builders) == 0 && len(f.probes) == 0 && len(f.registrations) == 0
}

type variablesUseReplayOptions struct {
	decode      func(compat.Step) (any, error)
	renderField func(name string, value any) any
	fixtures    variablesUseFixtures
}

func replayVariablesUse(ctx context.Context, env *esper.Environment, queries []esper.Query, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	return replayVariablesUseWithOptions(ctx, env, queries, caseScenario, caseName, variablesUseReplayOptions{
		decode:      decodeVariablesUsePayload,
		renderField: func(_ string, value any) any { return variablesUseRender(value) },
	})
}

func replayVariablesUseWithOptions(ctx context.Context, env *esper.Environment, queries []esper.Query, caseScenario compat.Scenario, caseName string, options variablesUseReplayOptions) (compat.Trace, error) {
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	var selectStatement *esper.Statement
	deploymentByLabel := make(map[string]string, 4)
	subscribeSelect := func(statement *esper.Statement) error {
		_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			for _, row := range batch.New {
				seq++
				fields := map[string]any{}
				if event, ok := row.Event(); ok {
					for _, field := range event.Schema().Fields() {
						fields[field.Name] = options.renderField(field.Name, event.Get(field.Name).Any())
					}
				} else if r, ok := row.Row(); ok {
					for _, field := range r.Schema().Fields() {
						fields[field.Name] = options.renderField(field.Name, r.Get(field.Name).Any())
					}
				}
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: statement.Name(),
					Sequence:  seq,
					Time:      currentTimeString(engine),
					New:       []compat.ResultRecord{{Kind: "row", Fields: fields}},
				})
			}
			return nil
		})
		return err
	}
	deployQuery := func(label string, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		deploymentByLabel[label] = deployment.ID()
		for _, statement := range deployment.Statements() {
			if statement.Name() == "s0" {
				selectStatement = statement
				if err := subscribeSelect(statement); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, query := range queries {
		if err := deployQuery("s0", query); err != nil {
			return trace, err
		}
	}
	if selectStatement == nil && options.fixtures.empty() {
		return trace, fmt.Errorf("variables-use: statement s0 not found")
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := options.decode(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "read-variable":
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "variable",
				Name:      step.Name,
			}
			value, ok := engine.GetVariable(step.Name)
			if !ok {
				return trace, fmt.Errorf("variables-use: variable %q not found", step.Name)
			}
			if value.IsNull() || value.IsMissing() {
				record.Value = map[string]any{"state": "null"}
			} else {
				record.Value = canonicalVariablesUseValue(value.Any())
			}
			trace.Records = append(trace.Records, record)
		case "set-variable":
			if err := replayVariablesUseSetStep(ctx, engine, step, caseName, &trace); err != nil {
				return trace, err
			}
		case "deploy":
			label := step.Statement
			if register := options.fixtures.registrations[label]; register != nil {
				if err := register(env); err != nil {
					return trace, err
				}
				continue
			}
			builder := options.fixtures.builders[label]
			if builder == nil {
				return trace, fmt.Errorf("variables-use: unknown deploy fixture %q", label)
			}
			query, err := builder(env)
			if err != nil {
				return trace, err
			}
			if err := deployQuery(label, query); err != nil {
				return trace, err
			}
		case "build-error":
			probe := options.fixtures.probes[step.Statement]
			if probe == nil {
				return trace, fmt.Errorf("variables-use: unknown build-error probe %q", step.Statement)
			}
			query, err := probe(env)
			if err != nil {
				return trace, err
			}
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "compile-error",
				Statement: step.Statement,
			}
			_, buildErr := env.Build(query)
			switch {
			case buildErr == nil:
				record.Value = "<no-error>"
			default:
				record.Value = variablesUseBareMessage(buildErr)
			}
			if err := variablesUseAssertExpectedMessage(step.ExpectError, record.Value.(string)); err != nil {
				return trace, err
			}
			trace.Records = append(trace.Records, record)
		case "undeploy-all":
			for _, id := range deploymentByLabel {
				if err := engine.Undeploy(ctx, id); err != nil {
					return trace, err
				}
			}
			deploymentByLabel = make(map[string]string, 4)
			selectStatement = nil
		case "undeploy":
			// Targeted module removal mirroring Java's
			// undeployModuleContaining between operator redeploys.
			id, ok := deploymentByLabel[step.Statement]
			if !ok {
				if options.fixtures.registrations[step.Statement] != nil {
					// Registration-only fixture: nothing was deployed.
					selectStatement = nil
					continue
				}
				return trace, fmt.Errorf("variables-use: unknown undeploy label %q", step.Statement)
			}
			if err := engine.Undeploy(ctx, id); err != nil {
				return trace, err
			}
			delete(deploymentByLabel, step.Statement)
			selectStatement = nil
		default:
			return trace, fmt.Errorf("unsupported variables-use step op %q", step.Op)
		}
	}
	return trace, nil
}

// replayVariablesUseSetStep executes one runtime variable-write step. Steps
// carrying expectError attempt the write, catch the failure, and emit the
// set-variable-error record with the bare aligned message; plain writes stay
// silent exactly like the Java execution. The bulk form (ordered assignment
// array) maps to Engine.SetVariables, which validates every assignment before
// applying anything, so failed batches roll back like Java's runtime-set.
func replayVariablesUseSetStep(ctx context.Context, engine *esper.Engine, step compat.Step, caseName string, trace *compat.Trace) error {
	expected := strings.TrimSpace(step.ExpectError)
	isBulk := len(step.Payload) > 0 && step.Payload[0] == '['
	if expected == "" {
		if isBulk {
			assignments, err := decodeVariablesUseAssignments(step.Payload)
			if err != nil {
				return err
			}
			return engine.SetVariables(ctx, assignments...)
		}
		value, err := decodeVariablesUseAssignedValue(step.Payload)
		if err != nil {
			return err
		}
		return engine.SetVariable(ctx, step.Name, value)
	}
	record := compat.TraceRecord{
		Case:      caseName,
		Operation: "set-variable-error",
	}
	caught := "<no-error>"
	if isBulk {
		assignments, err := decodeVariablesUseAssignments(step.Payload)
		if err != nil {
			return err
		}
		if setErr := engine.SetVariables(ctx, assignments...); setErr != nil {
			caught = variablesUseBareMessage(setErr)
		}
	} else {
		value, err := decodeVariablesUseAssignedValue(step.Payload)
		if err != nil {
			return err
		}
		if setErr := engine.SetVariable(ctx, step.Name, value); setErr != nil {
			caught = variablesUseBareMessage(setErr)
		}
	}
	if err := variablesUseAssertExpectedMessage(expected, caught); err != nil {
		return err
	}
	record.Value = caught
	trace.Records = append(trace.Records, record)
	return nil
}

func decodeVariablesUsePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event variablesUseBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var event variablesUseS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean_S0: %w", err)
		}
		return event, nil
	case "MyVariableCustomEvent":
		// The payload carries the raw value; MyVariableCustomType.of wraps it.
		var shim struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(step.Payload, &shim); err != nil {
			return nil, fmt.Errorf("variables-use MyVariableCustomEvent: %w", err)
		}
		return variablesUseCustomEvent{Name: variablesUseCustomType{Value: shim.Name}}, nil
	default:
		return nil, fmt.Errorf("variables-use: unsupported event type %q", step.EventType)
	}
}

func decodeVariablesUseAPIPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event variablesUseAPIBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var event variablesUseS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean_S0: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("variables-use: unsupported event type %q", step.EventType)
	}
}

func decodeVariablesUseConstantPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event variablesUseFullBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var event variablesUseS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("variables-use SupportBean_S0: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("variables-use: unsupported event type %q", step.EventType)
	}
}

type variablesUseAssignmentEntry struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

func decodeVariablesUseAssignments(payload json.RawMessage) ([]esper.VariableAssignment, error) {
	var entries []variablesUseAssignmentEntry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return nil, fmt.Errorf("variables-use assignments: %w", err)
	}
	assignments := make([]esper.VariableAssignment, 0, len(entries))
	for _, entry := range entries {
		value, err := decodeVariablesUseAssignedValue(entry.Value)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, esper.VariableAssignment{Name: entry.Name, Value: value})
	}
	return assignments, nil
}

// decodeVariablesUseAssignedValue converts an assignment payload scalar into
// the typed value handed to the engine. Tagged objects pin the boxed width so
// runtime type-mismatch messages name the same Java type on both sides; bare
// numbers follow Java autoboxing semantics (Integer).
func decodeVariablesUseAssignedValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var tag struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &tag); err == nil && tag.Type != "" {
		switch tag.Type {
		case "integer":
			var value int32
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "long":
			var value int64
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "short":
			var value int16
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "byte":
			var value int8
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "float":
			var value float32
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "double":
			var value float64
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "boolean":
			var value bool
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		case "string":
			var value string
			err := json.Unmarshal(tag.Value, &value)
			return value, err
		default:
			return nil, fmt.Errorf("variables-use: unknown assignment type tag %q", tag.Type)
		}
	}
	switch raw[0] {
	case '"':
		var value string
		err := json.Unmarshal(raw, &value)
		return value, err
	case 't', 'f':
		var value bool
		err := json.Unmarshal(raw, &value)
		return value, err
	case '[', '{':
		return nil, fmt.Errorf("variables-use: nested assignment payload %s", raw)
	default:
		var value int32
		if err := json.Unmarshal(raw, &value); err == nil {
			return value, nil
		}
		var wide float64
		if err := json.Unmarshal(raw, &wide); err == nil {
			return wide, nil
		}
		return nil, fmt.Errorf("variables-use: unsupported assignment payload %s", raw)
	}
}

// canonicalVariablesUseValue mirrors the shared record protocol for variable
// reads: ints numeric (long-truncated), strings bare, enums as ENUM_VALUE_N
// strings, arrays elementwise, and the nondeterministic Date constant as its
// fixed "<date>" marker.
func canonicalVariablesUseValue(value any) any {
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
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	case []string:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = element
		}
		return rendered
	case []int32:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = int64(element)
		}
		return rendered
	case []int64:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = element
		}
		return rendered
	case supportEnum:
		return typed.String()
	case []supportEnum:
		rendered := make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = element.String()
		}
		return rendered
	case variablesUseDateMarker:
		return "<date>"
	default:
		return fmt.Sprintf("%v", value)
	}
}

// variablesUseConstantRowField mirrors the oracle listener projection for the
// constant-variable rows: JSON null for unset properties, Java toString for
// scalars, NUL-character strings for char properties, enum constants by name.
func variablesUseConstantRowField(name string, value any) any {
	if value == nil {
		return nil
	}
	// Nullable boxed struct fields surface as Go pointers; dereference so
	// set values render like their Java boxed toString forms and unset
	// values reach the nil branch above.
	switch typed := value.(type) {
	case *string:
		if typed == nil {
			return nil
		}
		value = *typed
	case *bool:
		if typed == nil {
			return nil
		}
		value = *typed
	case *int32:
		if typed == nil {
			return nil
		}
		value = *typed
	case *int64:
		if typed == nil {
			return nil
		}
		value = *typed
	case *int16:
		if typed == nil {
			return nil
		}
		value = *typed
	case *int8:
		if typed == nil {
			return nil
		}
		value = *typed
	case *float32:
		if typed == nil {
			return nil
		}
		value = *typed
	case *float64:
		if typed == nil {
			return nil
		}
		value = *typed
	case *supportEnum:
		if typed == nil {
			return nil
		}
		value = *typed
	}
	if name == "charPrimitive" || name == "charBoxed" {
		if typed, ok := value.(int32); ok {
			return string(rune(typed))
		}
	}
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float32:
		return variablesUseFloatString(typed)
	case float64:
		return javaDoubleString(typed)
	case supportEnum:
		return typed.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func variablesUseFloatString(value float32) string {
	rendered := strconv.FormatFloat(float64(value), 'g', -1, 32)
	if !strings.ContainsAny(rendered, ".eE") {
		rendered += ".0"
	}
	return strings.Replace(rendered, "e", "E", 1)
}

// variablesUseBareMessage mirrors the oracle's rootCauseMessage: it descends
// the wrapped-error chain and reports the deepest esper.Error message, which
// carries the bare validation sentence rather than an outer wrapper rendering.
func variablesUseBareMessage(err error) string {
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

func variablesUseAssertExpectedMessage(expected, caught string) error {
	if expected != "" && expected != caught {
		return fmt.Errorf("variables-use: message drift: expected %q got %q", expected, caught)
	}
	return nil
}

// variablesUseRender mirrors the oracle's value conventions: the custom
// variable-hosted type renders as the canonical sorted-public-field object.
func variablesUseRender(value any) any {
	if custom, ok := value.(variablesUseCustomType); ok {
		return map[string]any{"name": custom.Value}
	}
	return streamSelectorRender(value)
}
