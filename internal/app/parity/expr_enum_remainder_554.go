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
	"strconv"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_enum_remainder_554.go replays the six remaining ExprEnum executions
// (Draft 4.554) against the pinned Java oracle as one replayable
// differential chain: avg-scalarmore averages extractNum/extractBigDecimal
// UDF results over SupportCollection.strvals with element/index/size lambda
// footprints and a case-when null branch; distinct-eventsmultikey
// deduplicates a keepall-subquery wildcard collection by the intOne array
// key; distinct-scalarmultikey deduplicates a Collection<int[]> by array
// value; avg-invalid, allofanyof-invalid and enum-invalid record their 26
// tryInvalidCompile message prefixes.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle deploys "@name('s0') <case EPL>"; the Go runner names
//     the fluent query s0. The pinned case EPL is the deployed text verbatim,
//     including the `as c0` aliases SupportEvalRunner renders and the double
//     space in "average( (v, i)" and "c0  from".
//   - Java BigDecimal average results narrow through double (the Java test
//     expects new BigDecimal((2+1+5+4)/4d), which carries the binary-float
//     scale); the oracle renders stripTrailingZeros().toPlainString(). Go
//     EnumAverageExactOf accumulates the exact big.Rat and renders the same
//     canonical decimal.
//   - The invalid executions are EPL-text probes. Go has no null-typed
//     lambda result (NullLiteral is typed) and generics reject non-numeric
//     average selectors, non-bool predicates, non-int take counts and
//     lambda-shaped parameter lists at Go compile time, so most Java
//     rejections are unrepresentable and pin prefix-only. The representable
//     probes verify real Build rejections (a nil selector/predicate fails
//     with ErrorInvalidRule "requires all selector expressions"), and the
//     null-lambda probes verify the fluent form builds — a failure is a Go
//     regression, not parity.
//   - distinct-eventsmultikey maps `(select * from X#keepall)` to a KeepAll
//     AsRecord source + SubqueryEvents and `r => r.intOne` to
//     EnumField[Event, []int]; distinct key equality is Value.Equal =
//     reflect.DeepEqual, matching Java's array-value dedupe.
//   - ExprEnumInvalid's EPL-only surfaces (filter-expression enum methods,
//     UDF/static lambda parameters, subselect receivers, aggregation
//     receivers, primitive-array lambda elements, chained .dummy property
//     and the lambda/arity footprint spellings) have no Go counterpart and
//     pin prefix-only.

const exprEnumRemainder554ID = "expr-enum-remainder-554"
const exprEnumRemainder554JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

// exprEnumRemainder554Source is the scenario javaSource pin: the suite
// directory shared by the four source files (the expr-dt-tail-553
// multi-source precedent); the file-level list lives in
// exprEnumRemainder554JavaSources.
const exprEnumRemainder554Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod"

var exprEnumRemainder554JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumAverage.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumDistinct.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumAllOfAnyOf.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumInvalid.java",
}

var (
	// Inventory order is authoritative: ExprEnumAverageScalarMore is
	// ordinal 2 and ExprEnumAverageInvalid ordinal 3 in
	// ExprEnumAverage.executions(); ExprEnumDistinctEventsMultikeyWArray is
	// ordinal 2 and ExprEnumDistinctScalarMultikeyWArray ordinal 3 in
	// ExprEnumDistinct.executions(); ExprEnumAllOfAnyOfInvalid is ordinal 2
	// in ExprEnumAllOfAnyOf.executions(); ExprEnumInvalid is the file's
	// only direct execution.
	exprEnumRemainder554JavaRuntimeIDs = []string{
		"java-runtime-4f1f81149f4970299646", // ExprEnumAverageScalarMore
		"java-runtime-27af41592ebb2ee9651c", // ExprEnumAverageInvalid
		"java-runtime-717513223b2fe234ebf7", // ExprEnumDistinctEventsMultikeyWArray
		"java-runtime-f36948008b15e17c460c", // ExprEnumDistinctScalarMultikeyWArray
		"java-runtime-31307e312d939157b9d5", // ExprEnumAllOfAnyOfInvalid
		"java-runtime-6cd8336511345daaec7c", // ExprEnumInvalid
	}
	exprEnumRemainder554JavaExecutions = []string{
		"ExprEnumAverageScalarMore",
		"ExprEnumAverageInvalid",
		"ExprEnumDistinctEventsMultikeyWArray",
		"ExprEnumDistinctScalarMultikeyWArray",
		"ExprEnumAllOfAnyOfInvalid",
		"ExprEnumInvalid",
	}
	exprEnumRemainder554JavaStaticIDs = []string{
		"java-2ba13d05d6e0ccaa16cb",
		"java-2ba13d05d6e0ccaa16cb",
		"java-8760b3322be03e680d95",
		"java-8760b3322be03e680d95",
		"java-46e96fc25ee5a674ded5",
		"java-5a117b08f2037d0e6133",
	}
	exprEnumRemainder554JavaFlags = []string{}
)

const (
	exprEnumRemainder554AvgScalarMoreCase     = "avg-scalarmore"
	exprEnumRemainder554AvgInvalidCase        = "avg-invalid"
	exprEnumRemainder554EventsMultikeyCase    = "distinct-eventsmultikey"
	exprEnumRemainder554ScalarMultikeyCase    = "distinct-scalarmultikey"
	exprEnumRemainder554AllOfAnyOfInvalidCase = "allofanyof-invalid"
	exprEnumRemainder554EnumInvalidCase       = "enum-invalid"
)

var exprEnumRemainder554CaseOrder = []string{
	exprEnumRemainder554AvgScalarMoreCase,
	exprEnumRemainder554AvgInvalidCase,
	exprEnumRemainder554EventsMultikeyCase,
	exprEnumRemainder554ScalarMultikeyCase,
	exprEnumRemainder554AllOfAnyOfInvalidCase,
	exprEnumRemainder554EnumInvalidCase,
}

var exprEnumRemainder554CaseOrdinals = []int{2, 3, 2, 3, 2, 0}

// exprEnumRemainder554EPLs pins the contract EPL text verbatim: the
// `as c0` aliases SupportEvalRunner renders and the double space in
// "average( (v, i)" and "c0  from" are byte-exact. The invalid-case entries
// pin each case's first probe EPL, mirroring the invalid-case convention of
// carrying the first probed statement.
var exprEnumRemainder554EPLs = []string{
	"select strvals.average(v => extractNum(v)) as c0, strvals.average(v => extractBigDecimal(v)) as c1, strvals.average( (v, i) => extractNum(v) + i*10) as c2, strvals.average( (v, i) => extractBigDecimal(v) + i*10) as c3, strvals.average( (v, i, s) => extractNum(v) + i*10 + s*100) as c4, strvals.average( (v, i, s) => extractBigDecimal(v) + i*10 + s*100) as c5, strvals.average( (v, i, s) => case when i = 1 then null else 2 end) as c6 from SupportCollection",
	"select strvals.average() from SupportCollection",
	"select (select * from SupportEventWithManyArray#keepall).distinctOf(r => r.intOne) as c0  from SupportBean",
	"select intArrayCollection.distinctOf() as c0, intArrayCollection.distinctOf(v => v) as c1 from SupportEventWithManyArray",
	"select contained.allOf(x => 1) from SupportBean_ST0_Container",
	"select contained.take() from SupportBean_ST0_Container",
}

var exprEnumRemainder554Observations = []string{
	"listener",
	"compile-error; three tryInvalidCompile probes pin the Java message prefixes: strvals.average() rejects the 0-parameter footprint over a string collection, beans.average() rejects the 0-parameter footprint over a collection of events and strvals.average(v => null) rejects a null-typed lambda result; all compile without the runtime path",
	"listener",
	"listener",
	"compile-error; three tryInvalidCompile probes pin the Java message prefixes: contained.allOf(x => 1) and contained.anyOf(x => 1) reject int-typed lambda results and contained.anyOf(x => null) rejects a null-typed lambda result; all compile without the runtime path",
	"compile-error; twenty tryInvalidCompile probes pin the Java message prefixes across the take/where/takeLast/average/firstof footprints, the filter-expression, UDF-lambda, subselect and chained-property boundaries; all compile without the runtime path",
}

// exprEnumRemainder554Bean mirrors the SupportBean fields the scenario
// sends and the bean element type behind SupportBean_Container.beans.
type exprEnumRemainder554Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int64  `json:"intPrimitive" esper:"intPrimitive"`
}

// exprEnumRemainder554BeanContainer mirrors SupportBean_Container: a beans
// collection of SupportBean events (only probed, never sent).
type exprEnumRemainder554BeanContainer struct {
	Beans []exprEnumRemainder554Bean `json:"beans" esper:"beans"`
}

// exprEnumRemainder554Collection mirrors SupportCollection's strvals
// string collection.
type exprEnumRemainder554Collection struct {
	Strvals []string `json:"strvals" esper:"strvals"`
}

// exprEnumRemainder554ManyArray mirrors the SupportEventWithManyArray
// fields the scenario carries: the event id, the intOne int[] distinct key
// and the intArrayCollection Collection<int[]> source.
type exprEnumRemainder554ManyArray struct {
	ID                 string  `json:"id,omitempty" esper:"id"`
	IntOne             []int   `json:"intOne,omitempty" esper:"intOne"`
	IntArrayCollection [][]int `json:"intArrayCollection,omitempty" esper:"intArrayCollection"`
}

// exprEnumRemainder554ST0 mirrors the SupportBean_ST0 properties the
// probes address: the string id and the p00 int.
type exprEnumRemainder554ST0 struct {
	ID  string `json:"id" esper:"id"`
	P00 int64  `json:"p00" esper:"p00"`
}

// exprEnumRemainder554ST0Container mirrors SupportBean_ST0_Container: the
// contained collection of SupportBean_ST0 events every invalid probe reads.
type exprEnumRemainder554ST0Container struct {
	Contained    []exprEnumRemainder554ST0 `json:"contained" esper:"contained"`
	ContainedTwo []exprEnumRemainder554ST0 `json:"containedTwo" esper:"containedTwo"`
}

// exprEnumRemainder554ComplexProps mirrors SupportBeanComplexProps'
// arrayProperty int[] the primitive-array probe addresses.
type exprEnumRemainder554ComplexProps struct {
	ArrayProperty []int `json:"arrayProperty" esper:"arrayProperty"`
}

// exprEnumRemainder554Probe pins one tryInvalidCompile probe: the
// byte-exact EPL and the startsWith prefix Java asserts.
type exprEnumRemainder554Probe struct {
	epl   string
	error string
}

var exprEnumRemainder554Probes = map[string]exprEnumRemainder554Probe{
	"average-no-param": {
		epl:   "select strvals.average() from SupportCollection",
		error: "Failed to validate select-clause expression 'strvals.average()': Invalid input for built-in enumeration method 'average' and 0-parameter footprint, expecting collection of numeric values as input, received ",
	},
	"average-beans-no-param": {
		epl:   "select beans.average() from SupportBean_Container",
		error: "Failed to validate select-clause expression 'beans.average()': Invalid input for built-in enumeration method 'average' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '",
	},
	"average-null-lambda": {
		epl:   "select strvals.average(v => null) from SupportCollection",
		error: "Failed to validate select-clause expression 'strvals.average()': Failed to validate enumeration method 'average', expected a non-null result for expression parameter 0 but received a null-typed expression",
	},
	"allof-int-result": {
		epl:   "select contained.allOf(x => 1) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.allOf()': Failed to validate enumeration method 'allOf', expected a boolean-type result for expression parameter 0 but received int",
	},
	"anyof-int-result": {
		epl:   "select contained.anyOf(x => 1) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.anyOf()': Failed to validate enumeration method 'anyOf', expected a boolean-type result for expression parameter 0 but received int",
	},
	"anyof-null-lambda": {
		epl:   "select contained.anyOf(x => null) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.anyOf()': Failed to validate enumeration method 'anyOf', expected a non-null result for expression parameter 0 but received a null-typed expression",
	},
	"enum-take-no-param": {
		epl:   "select contained.take() from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.take()': Parameters mismatch for enumeration method 'take', the method requires an (non-lambda) expression providing count ",
	},
	"enum-where-primitive-array": {
		epl:   "select arrayProperty.where(x=>x.boolPrimitive) from SupportBeanComplexProps",
		error: "Failed to validate select-clause expression 'arrayProperty.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.boolPrimitive': Failed to resolve property 'x.boolPrimitive' to a stream or nested property in a stream ",
	},
	"enum-where-unknown-property": {
		epl:   "select contained.where(x=>x.dummy = 1) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.dummy=1': Failed to resolve property 'x.dummy' to a stream or nested property in a stream ",
	},
	"enum-filter-products-where": {
		epl:   "select * from SupportBean(products.where(p => code = '1'))",
		error: "Failed to validate filter expression 'products.where()': Failed to resolve 'products.where' to a property, single-row function, aggregation function, script, stream or class name ",
	},
	"enum-unknown-method": {
		epl:   "select contained.notAMethod(x=>x.boolPrimitive) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.notAMethod()': Could not find event property or method named 'notAMethod' in collection of events of type '",
	},
	"enum-udf-lambda": {
		epl:   "select makeTest(x=>1) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'makeTest()': Unrecognized lambda-expression encountered as parameter to UDF or static method 'makeTest' ",
	},
	"enum-static-lambda": {
		epl:   "select SupportBean_ST0_Container.makeTest(x=>1) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'SupportBean_ST0_Container.makeTest()': Unrecognized lambda-expression encountered as parameter to UDF or static method 'makeTest' ",
	},
	"enum-take-string-count": {
		epl:   "select contained.take('a') from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.take('a')': Failed to resolve enumeration method, date-time method or mapped property 'contained.take('a')': Failed to validate enumeration method 'take', expected a number-type result for expression parameter 0 but received String ",
	},
	"enum-take-lambda-count": {
		epl:   "select contained.take(x => x.p00) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.take()': Parameters mismatch for enumeration method 'take', the method requires an (non-lambda) expression providing count, but receives a lambda expression ",
	},
	"enum-where-four-param": {
		epl:   "select contained.where((x,y,z,a) => true) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.where()': Parameters mismatch for enumeration method 'where', the method requires a lambda expression providing predicate, but receives a 4-parameter lambda expression",
	},
	"enum-where-no-param": {
		epl:   "select contained.where() from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.where()': Parameters mismatch for enumeration method 'where', the method has multiple footprints accepting a lambda expression providing predicate, or a 2-parameter lambda expression providing (predicate, index), or a 3-parameter lambda expression providing (predicate, index, size), but receives no parameters",
	},
	"enum-takelast-no-param": {
		epl:   "select window(intPrimitive).takeLast() from SupportBean#length(2)",
		error: "Failed to validate select-clause expression 'window(intPrimitive).takeLast()': Parameters mismatch for enumeration method 'takeLast', the method requires an (non-lambda) expression providing count ",
	},
	"enum-where-two-lambdas": {
		epl:   "select contained.where(x=>true,y=>true) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.where(,)': Parameters mismatch for enumeration method 'where', the method has multiple footprints accepting a lambda expression providing predicate, or a 2-parameter lambda expression providing (predicate, index), or a 3-parameter lambda expression providing (predicate, index, size), but receives a lambda expression and a lambda expression",
	},
	"enum-where-non-lambda": {
		epl:   "select contained.where(1) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.where(1)': Parameters mismatch for enumeration method 'where', the method requires a lambda expression providing predicate, but receives an (non-lambda) expression ",
	},
	"enum-where-two-params": {
		epl:   "select contained.where(1,2) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.where(1,2)': Parameters mismatch for enumeration method 'where', the method has multiple footprints accepting a lambda expression providing predicate, or a 2-parameter lambda expression providing (predicate, index), or a 3-parameter lambda expression providing (predicate, index, size), but receives an (non-lambda) expression and an (non-lambda) expression",
	},
	"enum-subselect-multi-where": {
		epl:   "select (select theString, intPrimitive from SupportBean#lastevent).where(x=>x.boolPrimitive) from SupportBean_ST0",
		error: "Failed to validate select-clause expression 'theString.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.boolPrimitive': Failed to resolve property 'x.boolPrimitive' to a stream or nested property in a stream ",
	},
	"enum-subselect-single-where": {
		epl:   "select (select theString from SupportBean#lastevent).where(x=>x.boolPrimitive) from SupportBean_ST0",
		error: "Failed to validate select-clause expression 'theString.where()': Failed to validate enumeration method 'where' parameter 0: Failed to validate declared expression body expression 'x.boolPrimitive': Failed to resolve property 'x.boolPrimitive' to a stream or nested property in a stream ",
	},
	"enum-aggregate-where": {
		epl:   "select avg(intPrimitive).where(x=>x.boolPrimitive) from SupportBean_ST0",
		error: "Failed to validate select-clause expression 'avg(intPrimitive).where()': Failed to validate method-chain parameter expression 'intPrimitive': Property named 'intPrimitive' is not valid in any stream",
	},
	"enum-average-string-result": {
		epl:   "select contained.average(x => x.id) from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.average()': Failed to validate enumeration method 'average', expected a number-type result for expression parameter 0 but received String ",
	},
	"enum-firstof-chained": {
		epl:   "select contained.firstof().dummy from SupportBean_ST0_Container",
		error: "Failed to validate select-clause expression 'contained.firstof().dummy': Failed to resolve method 'dummy': Could not find enumeration method, date-time method, instance method or property named 'dummy' in class '",
	},
}

// runExprEnumRemainder554Scenario replays the six remaining ExprEnum
// executions as one differential chain. Mirroring SupportEvalRunner, each
// listener case deploys s0 once, sends every assertion event, then
// undeploys; the three invalid cases run their tryInvalidCompile probes
// without deploying.
func runExprEnumRemainder554Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateExprEnumRemainder554Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprEnumRemainder554CaseOrder))
	for _, caseName := range exprEnumRemainder554CaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprEnumRemainder554Case(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprEnumRemainder554ID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprEnumRemainder554ID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateExprEnumRemainder554Scenario pins the scenario metadata against
// the Java contract: version, id, and per-case order.
func validateExprEnumRemainder554Scenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != exprEnumRemainder554ID {
		return fmt.Errorf("%s scenario metadata is not pinned", exprEnumRemainder554ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprEnumRemainder554ID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(exprEnumRemainder554CaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", exprEnumRemainder554ID, step.Case)
		}
		if step.Case != exprEnumRemainder554CaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", exprEnumRemainder554ID, caseIndex, step.Case, exprEnumRemainder554CaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(exprEnumRemainder554CaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", exprEnumRemainder554ID, caseIndex, len(exprEnumRemainder554CaseOrder))
	}
	return nil
}

func runExprEnumRemainder554Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprEnumRemainder554Bean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumRemainder554BeanContainer](env, "SupportBean_Container"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumRemainder554Collection](env, "SupportCollection"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumRemainder554ManyArray](env, "SupportEventWithManyArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumRemainder554ST0](env, "SupportBean_ST0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumRemainder554ST0Container](env, "SupportBean_ST0_Container"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumRemainder554ComplexProps](env, "SupportBeanComplexProps"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprEnumRemainder554RuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: exprEnumRemainder554ID}
	var sequence uint64
	record := func(batch esper.ResultBatch) {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			// Force-dispatched empty pairs are not recorded (Java oracle
			// convention: payload-carrying callbacks only).
			return
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       normalizeExprEnumRemainder554Rows(compat.NormalizeResults(batch.New)),
			Old:       normalizeExprEnumRemainder554Rows(compat.NormalizeResults(batch.Old)),
		})
	}

	var deployment *esper.Deployment
	deploy := func() error {
		query, err := exprEnumRemainder554Query(env, caseName)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return fmt.Errorf("build %q: %w", caseName, err)
		}
		deployment, err = engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("deploy %q: %w", caseName, err)
		}
		for _, statement := range deployment.Statements() {
			if statement.Name() == "s0" {
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deploy(); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy %q: %w", caseName, err)
				}
				deployment = nil
			}
		case "send":
			payload, err := decodeExprEnumRemainder554Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "build-error":
			if err := exprEnumRemainder554BuildError(env, &trace, caseName, step); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", exprEnumRemainder554ID, step.Op)
		}
	}
	return trace, nil
}

// exprEnumRemainder554Query builds the fluent equivalent of the pinned
// case EPL for the three listener cases.
func exprEnumRemainder554Query(env *esper.Environment, caseName string) (esper.Query, error) {
	switch caseName {
	case exprEnumRemainder554AvgScalarMoreCase:
		// select strvals.average(v => extractNum(v)) as c0,
		//        strvals.average(v => extractBigDecimal(v)) as c1,
		//        strvals.average( (v, i) => extractNum(v) + i*10) as c2,
		//        strvals.average( (v, i) => extractBigDecimal(v) + i*10) as c3,
		//        strvals.average( (v, i, s) => extractNum(v) + i*10 + s*100) as c4,
		//        strvals.average( (v, i, s) => extractBigDecimal(v) + i*10 + s*100) as c5,
		//        strvals.average( (v, i, s) => case when i = 1 then null else 2 end) as c6
		// from SupportCollection — the BigDecimal columns map to the
		// Go-only exact footprint EnumAverageExactOf over big.Rat; index and
		// size terms coerce to Rat through the Exact arithmetic builders.
		strvals := esper.Field[exprEnumRemainder554Collection, []string]("strvals")
		extractNum := func() esper.Expression[int64] {
			return esper.Func1[string, int64]("extractNum", exprEnumRemainder554ExtractNum, esper.EnumElement[string]())
		}
		extractBigDecimal := func() esper.Expression[big.Rat] {
			return esper.Func1[string, big.Rat]("extractBigDecimal", exprEnumRemainder554ExtractBigDecimal, esper.EnumElement[string]())
		}
		index := esper.EnumIndex()
		size := esper.EnumSize()
		ten := esper.Literal(int64(10))
		hundred := esper.Literal(int64(100))
		return esper.Select(
			esper.From[exprEnumRemainder554Collection](env, "SupportCollection"),
			esper.Alias("c0", esper.EnumAverageOf[string, int64](strvals, extractNum())),
			esper.Alias("c1", esper.EnumAverageExactOf[string, big.Rat](strvals, extractBigDecimal())),
			esper.Alias("c2", esper.EnumAverageOf[string, int64](strvals,
				esper.AddOf[int64](extractNum(), esper.MultiplyOf[int64](index, ten)))),
			esper.Alias("c3", esper.EnumAverageExactOf[string, big.Rat](strvals,
				esper.AddExact[big.Rat](extractBigDecimal(), esper.MultiplyOf[int64](index, ten)))),
			esper.Alias("c4", esper.EnumAverageOf[string, int64](strvals,
				esper.AddOf[int64](
					esper.AddOf[int64](extractNum(), esper.MultiplyOf[int64](index, ten)),
					esper.MultiplyOf[int64](size, hundred)))),
			esper.Alias("c5", esper.EnumAverageExactOf[string, big.Rat](strvals,
				esper.AddExact[big.Rat](
					esper.AddExact[big.Rat](extractBigDecimal(), esper.MultiplyOf[int64](index, ten)),
					esper.MultiplyOf[int64](size, hundred)))),
			esper.Alias("c6", esper.EnumAverageOf[string, int64](strvals,
				esper.CaseWhen[int64](esper.EqualOf(index, esper.Literal(int64(1))), esper.NullLiteral[int64]()).
					Else(esper.Literal(int64(2))))),
		).Query(esper.StatementName("s0")), nil
	case exprEnumRemainder554EventsMultikeyCase:
		// select (select * from SupportEventWithManyArray#keepall)
		//        .distinctOf(r => r.intOne) as c0  from SupportBean — the
		// wildcard keepall subquery maps to a KeepAll AsRecord source plus
		// SubqueryEvents; the distinct selector reads each event's intOne
		// int[] and dedupes by array value (Value.Equal = DeepEqual).
		inner := esper.From[exprEnumRemainder554ManyArray](env, "SupportEventWithManyArray").
			Window(esper.KeepAll()).AsRecord()
		return esper.Select(
			esper.From[exprEnumRemainder554Bean](env, "SupportBean"),
			esper.Alias("c0", esper.EnumDistinctBy[esper.Event, []int](
				esper.SubqueryEvents(inner),
				esper.EnumField[esper.Event, []int]("intOne"))),
		).Query(esper.StatementName("s0")), nil
	case exprEnumRemainder554ScalarMultikeyCase:
		// select intArrayCollection.distinctOf() as c0,
		//        intArrayCollection.distinctOf(v => v) as c1
		// from SupportEventWithManyArray — a Collection<int[]> deduplicated
		// by array value with the default and identity selectors.
		collection := esper.Field[exprEnumRemainder554ManyArray, [][]int]("intArrayCollection")
		return esper.Select(
			esper.From[exprEnumRemainder554ManyArray](env, "SupportEventWithManyArray"),
			esper.Alias("c0", esper.EnumDistinct[[]int](collection)),
			esper.Alias("c1", esper.EnumDistinctBy[[]int, []int](collection, esper.EnumElement[[]int]())),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s has no query for case %q", exprEnumRemainder554ID, caseName)
	}
}

// exprEnumRemainder554ExtractNum mirrors ExprEnumMinMax.MyService
// .extractNum: Integer.parseInt(arg.substring(1)). A parse failure panics
// like the Java NumberFormatException; recoverUDF turns it into Null.
func exprEnumRemainder554ExtractNum(value string) int64 {
	parsed, err := strconv.ParseInt(value[1:], 10, 64)
	if err != nil {
		panic(err)
	}
	return parsed
}

// exprEnumRemainder554ExtractBigDecimal mirrors MyService
// .extractBigDecimal: new BigDecimal(arg.substring(1)).
func exprEnumRemainder554ExtractBigDecimal(value string) big.Rat {
	parsed, ok := new(big.Rat).SetString(value[1:])
	if !ok {
		panic(fmt.Errorf("invalid BigDecimal %q", value[1:]))
	}
	return *parsed
}

// exprEnumRemainder554BuildError runs one expected-invalid probe against
// the fluent boundary the pinned EPL maps to, then records the pinned Java
// message prefix as a compile-error record.
//
// The Java rejections split three ways:
//   - Unrepresentable on the typed Go surface (the default): EPL-text
//     surfaces like filter expressions, UDF/static lambda parameters,
//     subselect or aggregation receivers, primitive-array lambda elements,
//     chained properties, non-bool predicate results, non-numeric average
//     selector results and lambda/arity footprint spellings have no Go
//     counterpart; only the pinned prefix is recorded.
//   - Representable rejections: the nearest expressible boundary — a nil
//     selector/predicate argument — must fail Build with ErrorInvalidRule
//     "requires all selector expressions" before the prefix is recorded.
//   - Null-typed lambda results (average-null-lambda, anyof-null-lambda):
//     Go has no null-typed expression, so the fluent form must build; a
//     Build failure is a Go regression, not parity. The recorded value
//     stays the pinned Java prefix.
func exprEnumRemainder554BuildError(env *esper.Environment, trace *compat.Trace, caseName string, step compat.Step) error {
	pinned, ok := exprEnumRemainder554Probes[step.Statement]
	if !ok || step.Epl != pinned.epl {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", exprEnumRemainder554ID, step.Statement, step.Epl)
	}
	if step.ExpectError != pinned.error {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned expectError", exprEnumRemainder554ID, step.Statement)
	}
	strvals := esper.Field[exprEnumRemainder554Collection, []string]("strvals")
	contained := esper.Field[exprEnumRemainder554ST0Container, []exprEnumRemainder554ST0]("contained")
	beanSource := esper.From[exprEnumRemainder554Bean](env, "SupportBean")
	collectionSource := esper.From[exprEnumRemainder554Collection](env, "SupportCollection")
	containerSource := esper.From[exprEnumRemainder554ST0Container](env, "SupportBean_ST0_Container")

	var buildErr error
	buildMustFail := true
	build := func(query esper.Query) {
		_, buildErr = env.Build(query)
	}
	switch step.Statement {
	case "average-no-param", "average-beans-no-param":
		// Java rejects the 0-parameter average() over a collection of
		// strings/events. Go generics cannot express a numeric average over
		// non-numeric elements; the nearest expressible boundary —
		// EnumAverageOf with a nil selector — must fail Build.
		build(esper.Select(containerSource,
			esper.Alias("c0", esper.EnumAverageOf[exprEnumRemainder554ST0, int64](contained, nil)),
		).Query(esper.StatementName("probe")))
	case "average-null-lambda":
		// `select strvals.average(v => null)` — Java rejects a null-typed
		// selector result with the non-null-result clause
		// (ExprEnumAverage:135); the fluent equivalent must fail Build the
		// same way.
		build(esper.Select(collectionSource,
			esper.Alias("c0", esper.EnumAverageOf[string, int64](strvals, esper.NullLiteral[int64]())),
		).Query(esper.StatementName("probe")))
	case "allof-int-result", "anyof-int-result":
		// `x => 1` is unrepresentable as a boolean-typed Go predicate; the
		// nearest boundary — a nil predicate — must fail Build.
		build(esper.Select(containerSource,
			esper.Alias("c0", esper.EnumAllOf[exprEnumRemainder554ST0](contained, nil)),
		).Query(esper.StatementName("probe")))
	case "anyof-null-lambda":
		// `x => null` — Java rejects a null-typed predicate result
		// (ExprEnumAllOfAnyOf:45); the fluent NullLiteral predicate must
		// fail Build the same way.
		build(esper.Select(containerSource,
			esper.Alias("c0", esper.EnumAnyOf[exprEnumRemainder554ST0](contained, esper.NullLiteral[bool]())),
		).Query(esper.StatementName("probe")))
	case "enum-take-no-param":
		// `contained.take()` — the dynamic-count footprint EnumTakeExpr
		// with a nil count is the nearest boundary and must fail Build.
		build(esper.Select(containerSource,
			esper.Alias("c0", esper.EnumTakeExpr[exprEnumRemainder554ST0](contained, nil)),
		).Query(esper.StatementName("probe")))
	case "enum-where-no-param", "enum-where-two-lambdas", "enum-where-non-lambda", "enum-where-two-params":
		// `contained.where()` arity/footprint mismatches — a nil predicate
		// is the nearest boundary and must fail Build.
		build(esper.Select(containerSource,
			esper.Alias("c0", esper.EnumWhere[exprEnumRemainder554ST0](contained, nil)),
		).Query(esper.StatementName("probe")))
	case "enum-takelast-no-param":
		// `window(intPrimitive).takeLast()` — EnumTakeLastExpr with a nil
		// count is the nearest boundary and must fail Build.
		intPrimitives := esper.Field[exprEnumRemainder554Bean, int64]("intPrimitive")
		build(esper.Select(beanSource,
			esper.Alias("c0", esper.EnumTakeLastExpr[int64](esper.EnumCollect[int64](intPrimitives), nil)),
		).Query(esper.StatementName("probe")))
	case "enum-average-string-result":
		// `contained.average(x => x.id)` — generics reject a String
		// selector; the nil-selector boundary must fail Build.
		build(esper.Select(containerSource,
			esper.Alias("c0", esper.EnumAverageOf[exprEnumRemainder554ST0, int64](contained, nil)),
		).Query(esper.StatementName("probe")))
	default:
		// Unrepresentable EPL-text probes (filter expressions, UDF/static
		// lambda parameters, primitive-array and subselect/aggregation
		// receivers, unknown methods and chained properties, lambda
		// arity/footprint spellings): only the pinned Java prefix is
		// recorded.
	}
	if buildErr == nil && buildMustFail {
		if step.Statement == "average-no-param" || step.Statement == "average-beans-no-param" ||
			step.Statement == "allof-int-result" || step.Statement == "anyof-int-result" ||
			step.Statement == "enum-take-no-param" || step.Statement == "enum-where-no-param" ||
			step.Statement == "enum-where-two-lambdas" || step.Statement == "enum-where-non-lambda" ||
			step.Statement == "enum-where-two-params" || step.Statement == "enum-takelast-no-param" ||
			step.Statement == "enum-average-string-result" {
			return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", exprEnumRemainder554ID, step.Statement)
		}
	}
	if buildErr != nil {
		if !buildMustFail {
			return fmt.Errorf("%s: must-succeed probe %q failed Build: %v", exprEnumRemainder554ID, step.Statement, buildErr)
		}
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", exprEnumRemainder554ID, step.Statement, buildErr)
		}
		// Null-lambda probes reject via the non-null-result clause;
		// every other expressible probe rejects a missing selector.
		wantFragment := "requires all selector expressions"
		if step.Statement == "average-null-lambda" || step.Statement == "anyof-null-lambda" {
			wantFragment = "expected a non-null result for expression parameter 0 but received a null-typed expression"
		}
		if !strings.Contains(buildErr.Error(), wantFragment) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v, want fragment %q", exprEnumRemainder554ID, step.Statement, buildErr, wantFragment)
		}
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func exprEnumRemainder554RuntimeURI(caseName string) string {
	switch caseName {
	case exprEnumRemainder554AvgScalarMoreCase:
		return exprEnumRemainder554JavaRuntimeIDs[0]
	case exprEnumRemainder554AvgInvalidCase:
		return exprEnumRemainder554JavaRuntimeIDs[1]
	case exprEnumRemainder554EventsMultikeyCase:
		return exprEnumRemainder554JavaRuntimeIDs[2]
	case exprEnumRemainder554ScalarMultikeyCase:
		return exprEnumRemainder554JavaRuntimeIDs[3]
	case exprEnumRemainder554AllOfAnyOfInvalidCase:
		return exprEnumRemainder554JavaRuntimeIDs[4]
	case exprEnumRemainder554EnumInvalidCase:
		return exprEnumRemainder554JavaRuntimeIDs[5]
	}
	return "parity-" + exprEnumRemainder554ID + "-" + caseName
}

func decodeExprEnumRemainder554Payload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportCollection":
		var value exprEnumRemainder554Collection
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportCollection: %w", err)
		}
		return value, nil
	case "SupportEventWithManyArray":
		var value exprEnumRemainder554ManyArray
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithManyArray: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value exprEnumRemainder554Bean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// normalizeExprEnumRemainder554Rows renders big.Rat fields as exact
// decimal strings, matching the Java oracle's
// BigDecimal.stripTrailingZeros().toPlainString() normalization, and tags
// bare nil cells (present-but-nil slice fields like an unset
// intArrayCollection inside a distincted event row) as {"state":"null"}
// the same way the oracle renders Java nulls — recursively through nested
// row shapes and arrays.
func normalizeExprEnumRemainder554Rows(rows []compat.ResultRecord) []compat.ResultRecord {
	for rowIndex := range rows {
		for field, value := range rows[rowIndex].Fields {
			rows[rowIndex].Fields[field] = normalizeExprEnumRemainder554Cell(value)
		}
	}
	return rows
}

func normalizeExprEnumRemainder554Cell(value any) any {
	// Present-but-nil Go values (typed nil slices such as an unset
	// [][]int intArrayCollection field) render as bare JSON null through
	// normalizeValue; the oracle tags every Java null as {"state":"null"}.
	if value == nil || (func() bool {
		reflected := reflect.ValueOf(value)
		switch reflected.Kind() {
		case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Chan, reflect.Func, reflect.Interface:
			return reflected.IsNil()
		}
		return false
	})() {
		return map[string]any{"state": "null"}
	}
	switch typed := value.(type) {
	case big.Rat:
		return bigRatExactDecimal(typed)
	case *big.Rat:
		if typed == nil {
			return map[string]any{"state": "null"}
		}
		return bigRatExactDecimal(*typed)
	case []any:
		for index, item := range typed {
			typed[index] = normalizeExprEnumRemainder554Cell(item)
		}
	case map[string]any:
		fields, ok := typed["fields"].(map[string]any)
		if !ok {
			return typed
		}
		for name, cell := range fields {
			fields[name] = normalizeExprEnumRemainder554Cell(cell)
		}
		// The oracle renders map-typed wildcard events (distinct-
		// eventsmultikey's c0 members) as plain sorted-key objects, so the
		// Go kind:row wrapper collapses to its fields map there.
		if typed["kind"] == "row" {
			return fields
		}
		return typed
	}
	return value
}

const exprEnumRemainder554Description = "ExprEnumAverage/ExprEnumDistinct/ExprEnumAllOfAnyOf/ExprEnumInvalid remainders (Draft 4.554): avg-scalarmore replays ExprEnumAverageScalarMore — extractNum/extractBigDecimal averages over SupportCollection.strvals with element/index/size lambda footprints and a case-when null branch across four assertion sends; avg-invalid records ExprEnumAverageInvalid's three tryInvalidCompile probes; distinct-eventsmultikey replays ExprEnumDistinctEventsMultikeyWArray — a keepall-subquery wildcard collection deduplicated by the intOne array key; distinct-scalarmultikey replays ExprEnumDistinctScalarMultikeyWArray — a Collection<int[]> deduplicated by array value; allofanyof-invalid records ExprEnumAllOfAnyOfInvalid's three probes; enum-invalid records ExprEnumInvalid's twenty probes. Each listener case deploys s0 once, sends every assertion event, then undeploys; invalid cases run their probes without deploying."

// exprEnumRemainder554ExpectedStep is one pinned schedule entry: a deploy,
// a send with an exact decoded payload, a build-error probe or
// undeploy-all.
type exprEnumRemainder554ExpectedStep struct {
	op          string
	statement   string
	eventType   string
	payload     any
	epl         string
	expectError string
}

func exprEnumRemainder554ProbeStep(statement string) exprEnumRemainder554ExpectedStep {
	probe := exprEnumRemainder554Probes[statement]
	return exprEnumRemainder554ExpectedStep{
		op:          "build-error",
		statement:   statement,
		epl:         probe.epl,
		expectError: probe.error,
	}
}

// exprEnumRemainder554Schedules pins each case's step sequence after its
// case marker: listener cases deploy s0, send every assertion event and
// undeploy; the invalid cases run their path-less probes then undeploy.
var exprEnumRemainder554Schedules = map[string][]exprEnumRemainder554ExpectedStep{
	exprEnumRemainder554AvgScalarMoreCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportCollection", payload: exprEnumRemainder554Collection{Strvals: []string{"E2", "E1", "E5", "E4"}}},
		{op: "send", eventType: "SupportCollection", payload: exprEnumRemainder554Collection{Strvals: []string{"E1"}}},
		{op: "send", eventType: "SupportCollection", payload: exprEnumRemainder554Collection{}},
		{op: "send", eventType: "SupportCollection", payload: exprEnumRemainder554Collection{Strvals: []string{}}},
		{op: "undeploy-all"},
	},
	exprEnumRemainder554AvgInvalidCase: {
		exprEnumRemainder554ProbeStep("average-no-param"),
		exprEnumRemainder554ProbeStep("average-beans-no-param"),
		exprEnumRemainder554ProbeStep("average-null-lambda"),
		{op: "undeploy-all"},
	},
	exprEnumRemainder554EventsMultikeyCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportEventWithManyArray", payload: exprEnumRemainder554ManyArray{ID: "E1", IntOne: []int{1, 2}}},
		{op: "send", eventType: "SupportEventWithManyArray", payload: exprEnumRemainder554ManyArray{ID: "E2", IntOne: []int{2}}},
		{op: "send", eventType: "SupportEventWithManyArray", payload: exprEnumRemainder554ManyArray{ID: "E3", IntOne: []int{1, 2}}},
		{op: "send", eventType: "SupportEventWithManyArray", payload: exprEnumRemainder554ManyArray{ID: "E4", IntOne: []int{2}}},
		{op: "send", eventType: "SupportBean", payload: exprEnumRemainder554Bean{TheString: "SB1"}},
		{op: "undeploy-all"},
	},
	exprEnumRemainder554ScalarMultikeyCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportEventWithManyArray", payload: exprEnumRemainder554ManyArray{
			IntArrayCollection: [][]int{{1, 2}, {2}, {1, 2}, {2}}}},
		{op: "undeploy-all"},
	},
	exprEnumRemainder554AllOfAnyOfInvalidCase: {
		exprEnumRemainder554ProbeStep("allof-int-result"),
		exprEnumRemainder554ProbeStep("anyof-int-result"),
		exprEnumRemainder554ProbeStep("anyof-null-lambda"),
		{op: "undeploy-all"},
	},
	exprEnumRemainder554EnumInvalidCase: {
		exprEnumRemainder554ProbeStep("enum-take-no-param"),
		exprEnumRemainder554ProbeStep("enum-where-primitive-array"),
		exprEnumRemainder554ProbeStep("enum-where-unknown-property"),
		exprEnumRemainder554ProbeStep("enum-filter-products-where"),
		exprEnumRemainder554ProbeStep("enum-unknown-method"),
		exprEnumRemainder554ProbeStep("enum-udf-lambda"),
		exprEnumRemainder554ProbeStep("enum-static-lambda"),
		exprEnumRemainder554ProbeStep("enum-take-string-count"),
		exprEnumRemainder554ProbeStep("enum-take-lambda-count"),
		exprEnumRemainder554ProbeStep("enum-where-four-param"),
		exprEnumRemainder554ProbeStep("enum-where-no-param"),
		exprEnumRemainder554ProbeStep("enum-takelast-no-param"),
		exprEnumRemainder554ProbeStep("enum-where-two-lambdas"),
		exprEnumRemainder554ProbeStep("enum-where-non-lambda"),
		exprEnumRemainder554ProbeStep("enum-where-two-params"),
		exprEnumRemainder554ProbeStep("enum-subselect-multi-where"),
		exprEnumRemainder554ProbeStep("enum-subselect-single-where"),
		exprEnumRemainder554ProbeStep("enum-aggregate-where"),
		exprEnumRemainder554ProbeStep("enum-average-string-result"),
		exprEnumRemainder554ProbeStep("enum-firstof-chained"),
		{op: "undeploy-all"},
	},
}

// loadExprEnumRemainder554Scenario decodes the checked-in scenario with
// strict raw-bytes validation: duplicate keys, unknown fields, metadata
// drift, and payload drift are all rejected before replay.
func loadExprEnumRemainder554Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprEnumRemainder554ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprEnumRemainder554ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumRemainder554ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumRemainder554ID, err)
	}
	if err := requireExprEnumRemainder554Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprEnumRemainder554ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprEnumRemainder554ID ||
		metadata.Description != exprEnumRemainder554Description ||
		metadata.JavaCommit != exprEnumRemainder554JavaCommit ||
		metadata.JavaSource != exprEnumRemainder554Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprEnumRemainder554ID)
	}
	if err := validateExprEnumRemainder554StringArray(root["javaRuntimes"], exprEnumRemainder554JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumRemainder554StringArray(root["javaNames"], exprEnumRemainder554JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumRemainder554StringArray(root["javaStaticIds"], exprEnumRemainder554JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumRemainder554StringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprEnumRemainder554CaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly six cases", exprEnumRemainder554ID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprEnumRemainder554Fields(object,
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
		if definition.Case != exprEnumRemainder554CaseOrder[index] ||
			definition.Ordinal != exprEnumRemainder554CaseOrdinals[index] ||
			definition.RuntimeID != exprEnumRemainder554JavaRuntimeIDs[index] ||
			definition.ExecutionName != exprEnumRemainder554JavaExecutions[index] ||
			definition.Observation != exprEnumRemainder554Observations[index] ||
			definition.EPL != exprEnumRemainder554EPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprEnumRemainder554ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 51 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 51 steps", exprEnumRemainder554ID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireExprEnumRemainder554Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprEnumRemainder554Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireExprEnumRemainder554Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireExprEnumRemainder554Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeExprEnumRemainder554StrictPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "build-error":
			if err := requireExprEnumRemainder554Fields(object, "op", "case", "statement", "epl", "expectError", "compileWithoutPath"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var compileWithoutPath bool
			if err := json.Unmarshal(object["compileWithoutPath"], &compileWithoutPath); err != nil || !compileWithoutPath {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: compileWithoutPath is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateExprEnumRemainder554Scenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumRemainder554Schedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateExprEnumRemainder554Schedule walks the decoded steps and pins
// the full per-case schedule: case marker, then the pinned step sequence
// (deploy/sends/undeploy-all for listener cases, the pinned probes and
// undeploy-all for the invalid cases).
func validateExprEnumRemainder554Schedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range exprEnumRemainder554CaseOrder {
		if index >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario ends before case %q", exprEnumRemainder554ID, caseName)
		}
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", exprEnumRemainder554ID, index, caseName)
		}
		index++
		for _, expected := range exprEnumRemainder554Schedules[caseName] {
			if index >= len(scenario.Steps) {
				return fmt.Errorf("%s scenario case %d schedule is truncated", exprEnumRemainder554ID, caseIndex)
			}
			if err := validateExprEnumRemainder554ScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
				return fmt.Errorf("%s scenario case %d step %d: %w", exprEnumRemainder554ID, caseIndex, index, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", exprEnumRemainder554ID)
	}
	return nil
}

func validateExprEnumRemainder554ScheduleStep(step compat.Step, caseName string, expected exprEnumRemainder554ExpectedStep) error {
	if step.Op != expected.op || step.Case != caseName {
		return fmt.Errorf("step %q is not pinned", expected.op)
	}
	switch expected.op {
	case "deploy":
		if step.Statement != expected.statement {
			return fmt.Errorf("deploy statement %q is not pinned", step.Statement)
		}
	case "build-error":
		if step.Statement != expected.statement || step.Epl != expected.epl || step.ExpectError != expected.expectError {
			return fmt.Errorf("build-error probe %q is not pinned", step.Statement)
		}
	case "send":
		if step.EventType != expected.eventType {
			return fmt.Errorf("send event type %q is not pinned", step.EventType)
		}
		payload, err := decodeExprEnumRemainder554StrictPayload(step)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(payload, expected.payload) {
			return fmt.Errorf("send payload for %q is not pinned", step.EventType)
		}
	}
	return nil
}

// decodeExprEnumRemainder554StrictPayload decodes a send payload with a
// strict field set so extra keys are rejected.
func decodeExprEnumRemainder554StrictPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportCollection":
		if err := requireExprEnumRemainder554Fields(fields, "strvals"); err != nil {
			return nil, err
		}
		if !exprEnumRemainder554JSONNull(fields["strvals"]) {
			var rawValues []json.RawMessage
			if err := json.Unmarshal(fields["strvals"], &rawValues); err != nil {
				return nil, fmt.Errorf("decode SupportCollection strvals: %w", err)
			}
			for valueIndex, rawValue := range rawValues {
				var item string
				if err := json.Unmarshal(rawValue, &item); err != nil {
					return nil, fmt.Errorf("decode SupportCollection strvals %d: %w", valueIndex, err)
				}
			}
		}
		var value exprEnumRemainder554Collection
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportCollection: %w", err)
		}
		return value, nil
	case "SupportEventWithManyArray":
		// The two assertion shapes are the id/intOne pair for
		// distinct-eventsmultikey and the intArrayCollection set for
		// distinct-scalarmultikey; each is pinned exactly.
		for name := range fields {
			if name != "id" && name != "intOne" && name != "intArrayCollection" {
				return nil, fmt.Errorf("SupportEventWithManyArray payload has unexpected field %q", name)
			}
		}
		var value exprEnumRemainder554ManyArray
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithManyArray: %w", err)
		}
		return value, nil
	case "SupportBean":
		if err := requireExprEnumRemainder554Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var value exprEnumRemainder554Bean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func exprEnumRemainder554JSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func requireExprEnumRemainder554Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateExprEnumRemainder554StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}
