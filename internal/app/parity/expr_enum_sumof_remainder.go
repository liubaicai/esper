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

// expr_enum_sumof_remainder.go replays the four remaining ExprEnumSumOf
// executions (ords 1, 3, 4, 5) against the pinned Java oracle as one
// replayable differential chain: sum-events-plus sums intBoxed over a
// collection of events with element/index/size lambda footprints and a
// case-when null branch; sum-scalar-string sums extractNum/extractBigDecimal
// UDF results over a scalar string collection; sum-invalid records the two
// tryInvalidCompile message prefixes; sum-array sums constant collections of
// Double, BigInteger and nullable Long literals.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle deploys "@name('s0') <case EPL>"; the Go runner names
//     the fluent query s0. The pinned case EPL is the contract text verbatim,
//     including the implicit-alias select items and the double space in
//     "sumOf( (x, i)".
//   - Java BigDecimal/BigInteger results normalize to exact decimal strings
//     (the oracle renders toPlainString()/toString()); Go big.Rat/big.Int
//     match through the same rendering.
//   - Probe sumof-no-param: Java rejects a 0-parameter sumof() over a
//     collection of events (input shape). Go generics cannot express a
//     numeric sum over event elements, so the nearest expressible boundary —
//     EnumSumOf with a nil selector — must fail Build before the pinned
//     prefix is recorded.
//   - Probe sumof-null-lambda: Java rejects a null-typed lambda result at
//     compile time; Go has no null-typed expression (NullLiteral is typed),
//     so the Java rejection is unrepresentable. The fluent form must build;
//     a Build failure is a Go regression. The pinned prefix is then recorded.

const exprEnumSumOfRemainderID = "expr-enum-sumof-remainder"
const exprEnumSumOfRemainderJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprEnumSumOfRemainderJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumSumOf.java",
}

var (
	// Inventory order is authoritative: ExprEnumSumEventsPlus is ordinal 1,
	// ExprEnumSumScalarStringValue ordinal 3, ExprEnumSumInvalid ordinal 4
	// and ExprEnumSumArray ordinal 5 in ExprEnumSumOf.executions().
	exprEnumSumOfRemainderJavaRuntimeIDs = []string{
		"java-runtime-8497175e9fc13501285c", // ExprEnumSumEventsPlus
		"java-runtime-93ec466957ff19bc38a6", // ExprEnumSumScalarStringValue
		"java-runtime-bbe116cdf8ad7e13172f", // ExprEnumSumInvalid
		"java-runtime-b0455a5d1e4b447f34b1", // ExprEnumSumArray
	}
	exprEnumSumOfRemainderJavaExecutions = []string{
		"ExprEnumSumEventsPlus",
		"ExprEnumSumScalarStringValue",
		"ExprEnumSumInvalid",
		"ExprEnumSumArray",
	}
	exprEnumSumOfRemainderJavaStaticIDs = []string{
		"java-7443fe2db668140640df",
		"java-ce9a9b8124ed9b9cda09",
		"java-3bdebdfafa650da60077",
		"java-2c99e8eabfc5cdb0b018",
	}
)

const (
	exprEnumSumOfRemainderSumEventsPlusCase   = "sum-events-plus"
	exprEnumSumOfRemainderSumScalarStringCase = "sum-scalar-string"
	exprEnumSumOfRemainderSumInvalidCase      = "sum-invalid"
	exprEnumSumOfRemainderSumArrayCase        = "sum-array"
)

var exprEnumSumOfRemainderCaseOrder = []string{
	exprEnumSumOfRemainderSumEventsPlusCase,
	exprEnumSumOfRemainderSumScalarStringCase,
	exprEnumSumOfRemainderSumInvalidCase,
	exprEnumSumOfRemainderSumArrayCase,
}

var exprEnumSumOfRemainderCaseOrdinals = []int{1, 3, 4, 5}

// exprEnumSumOfRemainderEPLs pins the contract EPL text verbatim: the
// implicit-alias select items and the double space in "sumOf( (x, i)" are
// byte-exact. The sum-invalid entry pins the first probe EPL, mirroring the
// invalid-case convention of carrying the first probed statement.
var exprEnumSumOfRemainderEPLs = []string{
	"select beans.sumOf(x => intBoxed) c0, beans.sumOf( (x, i) => intBoxed + i*10) c1, beans.sumOf( (x, i, s) => intBoxed + i*10 + s*100) c2, beans.sumOf( (x, i) => case when i = 1 then null else 1 end) c3 from SupportBean_Container",
	"select strvals.sumOf(v => extractNum(v)) c0, strvals.sumOf(v => extractBigDecimal(v)) c1, strvals.sumOf( (v, i) => extractNum(v) + i*10) c2, strvals.sumOf( (v, i, s) => extractNum(v) + i*10 + s*100) c3 from SupportCollection",
	"select beans.sumof() from SupportBean_Container",
	"select {1d, 2d}.sumOf() c0, {BigInteger.valueOf(1), BigInteger.valueOf(2)}.sumOf() c1, {1L, 2L}.sumOf() c2, {1L, 2L, null}.sumOf() c3 from SupportBean",
}

var exprEnumSumOfRemainderObservations = []string{
	"listener",
	"listener",
	"compile-error; two tryInvalidCompile probes pin the Java message prefixes: beans.sumof() rejects a collection of events as the 0-parameter input and strvals.sumOf(v => null) rejects a null-typed lambda result; both compile without the runtime path",
	"listener",
}

// exprEnumSumOfRemainderBean mirrors the SupportBean fields the scenario
// sends: theString (unobserved but pinned for Java makeSB fidelity) and the
// nullable intBoxed the lambdas sum.
type exprEnumSumOfRemainderBean struct {
	TheString string `json:"theString" esper:"theString"`
	IntBoxed  *int64 `json:"intBoxed" esper:"intBoxed"`
}

// exprEnumSumOfRemainderContainer mirrors SupportBean_Container: a beans
// collection of SupportBean events.
type exprEnumSumOfRemainderContainer struct {
	Beans []exprEnumSumOfRemainderBean `json:"beans" esper:"beans"`
}

// exprEnumSumOfRemainderCollection mirrors SupportCollection's strvals
// string collection.
type exprEnumSumOfRemainderCollection struct {
	Strvals []string `json:"strvals" esper:"strvals"`
}

// exprEnumSumOfRemainderProbeEPLs pins the byte-exact tryInvalidCompile EPLs.
var exprEnumSumOfRemainderProbeEPLs = map[string]string{
	"sumof-no-param":    "select beans.sumof() from SupportBean_Container",
	"sumof-null-lambda": "select strvals.sumOf(v => null) from SupportCollection",
}

// exprEnumSumOfRemainderProbeErrors pins the Java message prefixes verbatim
// from ExprEnumSumOf lines 166 and 169 (prefixes only; the JVM FQN suffix of
// the first message diverges by design).
var exprEnumSumOfRemainderProbeErrors = map[string]string{
	"sumof-no-param":    "Failed to validate select-clause expression 'beans.sumof()': Invalid input for built-in enumeration method 'sumof' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '",
	"sumof-null-lambda": "Failed to validate select-clause expression 'strvals.sumOf()': Failed to validate enumeration method 'sumOf', expected a non-null result for expression parameter 0 but received a null-typed expression",
}

// runExprEnumSumOfRemainderScenario replays the four remaining ExprEnumSumOf
// executions (ords 1, 3, 4, 5) as one differential chain. Mirroring
// SupportEvalRunner, each listener case deploys s0 once, sends every
// assertion event, then undeploys; the invalid case runs its two
// tryInvalidCompile probes without deploying.
func runExprEnumSumOfRemainderScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateExprEnumSumOfRemainderScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprEnumSumOfRemainderCaseOrder))
	for _, caseName := range exprEnumSumOfRemainderCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprEnumSumOfRemainderCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprEnumSumOfRemainderID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprEnumSumOfRemainderID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateExprEnumSumOfRemainderScenario pins the scenario metadata against
// the Java contract: version, id, and per-case order.
func validateExprEnumSumOfRemainderScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != exprEnumSumOfRemainderID {
		return fmt.Errorf("%s scenario metadata is not pinned", exprEnumSumOfRemainderID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprEnumSumOfRemainderID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(exprEnumSumOfRemainderCaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", exprEnumSumOfRemainderID, step.Case)
		}
		if step.Case != exprEnumSumOfRemainderCaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", exprEnumSumOfRemainderID, caseIndex, step.Case, exprEnumSumOfRemainderCaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(exprEnumSumOfRemainderCaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", exprEnumSumOfRemainderID, caseIndex, len(exprEnumSumOfRemainderCaseOrder))
	}
	return nil
}

func runExprEnumSumOfRemainderCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprEnumSumOfRemainderBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumSumOfRemainderContainer](env, "SupportBean_Container"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[exprEnumSumOfRemainderCollection](env, "SupportCollection"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprEnumSumOfRemainderRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: exprEnumSumOfRemainderID}
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
			New:       normalizeExprEnumSumOfRemainderRows(compat.NormalizeResults(batch.New)),
			Old:       normalizeExprEnumSumOfRemainderRows(compat.NormalizeResults(batch.Old)),
		})
	}

	var deployment *esper.Deployment
	deploy := func() error {
		query, err := exprEnumSumOfRemainderQuery(env, caseName)
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
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s advance-time: %w", exprEnumSumOfRemainderID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodeExprEnumSumOfRemainderPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "build-error":
			if err := exprEnumSumOfRemainderBuildError(env, &trace, caseName, step); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", exprEnumSumOfRemainderID, step.Op)
		}
	}
	return trace, nil
}

// exprEnumSumOfRemainderQuery builds the fluent equivalent of the pinned
// case EPL for the three listener cases.
func exprEnumSumOfRemainderQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	switch caseName {
	case exprEnumSumOfRemainderSumEventsPlusCase:
		// select beans.sumOf(x => intBoxed) c0,
		//        beans.sumOf( (x, i) => intBoxed + i*10) c1,
		//        beans.sumOf( (x, i, s) => intBoxed + i*10 + s*100) c2,
		//        beans.sumOf( (x, i) => case when i = 1 then null else 1 end) c3
		// from SupportBean_Container
		beans := esper.Field[exprEnumSumOfRemainderContainer, []exprEnumSumOfRemainderBean]("beans")
		intBoxed := func() esper.Expression[int64] {
			return esper.EnumField[exprEnumSumOfRemainderBean, int64]("intBoxed")
		}
		index := esper.EnumIndex()
		size := esper.EnumSize()
		ten := esper.Literal(int64(10))
		hundred := esper.Literal(int64(100))
		return esper.Select(
			esper.From[exprEnumSumOfRemainderContainer](env, "SupportBean_Container"),
			esper.Alias("c0", esper.EnumSumOf[exprEnumSumOfRemainderBean, int64](beans, intBoxed())),
			esper.Alias("c1", esper.EnumSumOf[exprEnumSumOfRemainderBean, int64](beans,
				esper.AddOf[int64](intBoxed(), esper.MultiplyOf[int64](index, ten)))),
			esper.Alias("c2", esper.EnumSumOf[exprEnumSumOfRemainderBean, int64](beans,
				esper.AddOf[int64](
					esper.AddOf[int64](intBoxed(), esper.MultiplyOf[int64](index, ten)),
					esper.MultiplyOf[int64](size, hundred)))),
			esper.Alias("c3", esper.EnumSumOf[exprEnumSumOfRemainderBean, int64](beans,
				esper.CaseWhen[int64](esper.EqualOf(index, esper.Literal(int64(1))), esper.NullLiteral[int64]()).
					Else(esper.Literal(int64(1))))),
		).Query(esper.StatementName("s0")), nil
	case exprEnumSumOfRemainderSumScalarStringCase:
		// select strvals.sumOf(v => extractNum(v)) c0,
		//        strvals.sumOf(v => extractBigDecimal(v)) c1,
		//        strvals.sumOf( (v, i) => extractNum(v) + i*10) c2,
		//        strvals.sumOf( (v, i, s) => extractNum(v) + i*10 + s*100) c3
		// from SupportCollection
		strvals := esper.Field[exprEnumSumOfRemainderCollection, []string]("strvals")
		extractNum := func() esper.Expression[int64] {
			return esper.Func1[string, int64]("extractNum", exprEnumSumOfRemainderExtractNum, esper.EnumElement[string]())
		}
		extractBigDecimal := func() esper.Expression[big.Rat] {
			return esper.Func1[string, big.Rat]("extractBigDecimal", exprEnumSumOfRemainderExtractBigDecimal, esper.EnumElement[string]())
		}
		index := esper.EnumIndex()
		size := esper.EnumSize()
		ten := esper.Literal(int64(10))
		hundred := esper.Literal(int64(100))
		return esper.Select(
			esper.From[exprEnumSumOfRemainderCollection](env, "SupportCollection"),
			esper.Alias("c0", esper.EnumSumOf[string, int64](strvals, extractNum())),
			esper.Alias("c1", esper.EnumSumOf[string, big.Rat](strvals, extractBigDecimal())),
			esper.Alias("c2", esper.EnumSumOf[string, int64](strvals,
				esper.AddOf[int64](extractNum(), esper.MultiplyOf[int64](index, ten)))),
			esper.Alias("c3", esper.EnumSumOf[string, int64](strvals,
				esper.AddOf[int64](
					esper.AddOf[int64](extractNum(), esper.MultiplyOf[int64](index, ten)),
					esper.MultiplyOf[int64](size, hundred)))),
		).Query(esper.StatementName("s0")), nil
	case exprEnumSumOfRemainderSumArrayCase:
		// select {1d, 2d}.sumOf() c0,
		//        {BigInteger.valueOf(1), BigInteger.valueOf(2)}.sumOf() c1,
		//        {1L, 2L}.sumOf() c2,
		//        {1L, 2L, null}.sumOf() c3
		// from SupportBean — the constant collections are Go literals; the
		// nullable Long collection sums through a Cast that turns nil
		// elements into the skipped null lambda result.
		return esper.Select(
			esper.From[exprEnumSumOfRemainderBean](env, "SupportBean"),
			esper.Alias("c0", esper.EnumSum[float64](esper.Literal([]float64{1, 2}))),
			esper.Alias("c1", esper.EnumSum[big.Int](esper.Literal([]big.Int{*big.NewInt(1), *big.NewInt(2)}))),
			esper.Alias("c2", esper.EnumSum[int64](esper.Literal([]int64{1, 2}))),
			esper.Alias("c3", esper.EnumSumOf[*int64, int64](
				esper.Literal([]*int64{exprEnumSumOfRemainderInt64(1), exprEnumSumOfRemainderInt64(2), nil}),
				esper.Cast[*int64, int64](esper.EnumElement[*int64]()))),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s has no query for case %q", exprEnumSumOfRemainderID, caseName)
	}
}

// exprEnumSumOfRemainderExtractNum mirrors ExprEnumMinMax.MyService
// .extractNum: Integer.parseInt(arg.substring(1)). A parse failure panics
// like the Java NumberFormatException; recoverUDF turns it into Null.
func exprEnumSumOfRemainderExtractNum(value string) int64 {
	parsed, err := strconv.ParseInt(value[1:], 10, 64)
	if err != nil {
		panic(err)
	}
	return parsed
}

// exprEnumSumOfRemainderExtractBigDecimal mirrors MyService
// .extractBigDecimal: new BigDecimal(arg.substring(1)).
func exprEnumSumOfRemainderExtractBigDecimal(value string) big.Rat {
	parsed, ok := new(big.Rat).SetString(value[1:])
	if !ok {
		panic(fmt.Errorf("invalid BigDecimal %q", value[1:]))
	}
	return *parsed
}

func exprEnumSumOfRemainderInt64(value int64) *int64 {
	return &value
}

// exprEnumSumOfRemainderBuildError runs one expected-invalid probe against
// the fluent equivalent of the pinned EPL, then records the pinned Java
// message prefix as a compile-error record.
func exprEnumSumOfRemainderBuildError(env *esper.Environment, trace *compat.Trace, caseName string, step compat.Step) error {
	if pinned, ok := exprEnumSumOfRemainderProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", exprEnumSumOfRemainderID, step.Statement, step.Epl)
	}
	if pinned, ok := exprEnumSumOfRemainderProbeErrors[step.Statement]; !ok || step.ExpectError != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned expectError", exprEnumSumOfRemainderID, step.Statement)
	}
	var buildErr error
	switch step.Statement {
	case "sumof-no-param":
		// `select beans.sumof() from SupportBean_Container` — Java rejects
		// the 0-parameter footprint over a collection of events. Go generics
		// cannot express a numeric sum over event elements; the nearest
		// expressible boundary is EnumSumOf with a nil selector, which must
		// fail Build.
		_, buildErr = env.Build(esper.Select(
			esper.From[exprEnumSumOfRemainderContainer](env, "SupportBean_Container"),
			esper.Alias("c0", esper.EnumSumOf[exprEnumSumOfRemainderBean, int64](
				esper.Field[exprEnumSumOfRemainderContainer, []exprEnumSumOfRemainderBean]("beans"), nil)),
		).Query(esper.StatementName("s0")))
	case "sumof-null-lambda":
		// `select strvals.sumOf(v => null) from SupportCollection` — Java
		// rejects the null-typed selector result with the non-null-result
		// clause (ExprEnumSumOf:169); the fluent equivalent must fail
		// Build with the same category and message family.
		_, err := env.Build(esper.Select(
			esper.From[exprEnumSumOfRemainderCollection](env, "SupportCollection"),
			esper.Alias("c0", esper.EnumSumOf[string, int64](
				esper.Field[exprEnumSumOfRemainderCollection, []string]("strvals"), esper.NullLiteral[int64]())),
		).Query(esper.StatementName("s0")))
		var espErr *esper.Error
		if !errors.As(err, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
			!strings.Contains(err.Error(), `enumeration method "sumOf" expected a non-null result for expression parameter 0 but received a null-typed expression`) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", exprEnumSumOfRemainderID, step.Statement, err)
		}
		buildErr = err
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", exprEnumSumOfRemainderID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", exprEnumSumOfRemainderID, step.Statement)
	}
	if step.Statement == "sumof-no-param" {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
			!strings.Contains(buildErr.Error(), "requires all selector expressions") {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", exprEnumSumOfRemainderID, step.Statement, buildErr)
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

func exprEnumSumOfRemainderRuntimeURI(caseName string) string {
	switch caseName {
	case exprEnumSumOfRemainderSumEventsPlusCase:
		return exprEnumSumOfRemainderJavaRuntimeIDs[0]
	case exprEnumSumOfRemainderSumScalarStringCase:
		return exprEnumSumOfRemainderJavaRuntimeIDs[1]
	case exprEnumSumOfRemainderSumInvalidCase:
		return exprEnumSumOfRemainderJavaRuntimeIDs[2]
	case exprEnumSumOfRemainderSumArrayCase:
		return exprEnumSumOfRemainderJavaRuntimeIDs[3]
	}
	return "parity-" + exprEnumSumOfRemainderID + "-" + caseName
}

func decodeExprEnumSumOfRemainderPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_Container":
		var value exprEnumSumOfRemainderContainer
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_Container: %w", err)
		}
		return value, nil
	case "SupportCollection":
		var value exprEnumSumOfRemainderCollection
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportCollection: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value exprEnumSumOfRemainderBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// normalizeExprEnumSumOfRemainderRows renders big.Rat/big.Int fields as
// exact decimal strings, matching the Java oracle's
// BigDecimal.toPlainString()/BigInteger.toString() normalization.
func normalizeExprEnumSumOfRemainderRows(rows []compat.ResultRecord) []compat.ResultRecord {
	for rowIndex := range rows {
		for field, value := range rows[rowIndex].Fields {
			switch typed := value.(type) {
			case big.Int:
				rows[rowIndex].Fields[field] = typed.String()
			case *big.Int:
				if typed == nil {
					rows[rowIndex].Fields[field] = nil
				} else {
					rows[rowIndex].Fields[field] = typed.String()
				}
			case big.Rat:
				rows[rowIndex].Fields[field] = bigRatExactDecimal(typed)
			case *big.Rat:
				if typed == nil {
					rows[rowIndex].Fields[field] = nil
				} else {
					rows[rowIndex].Fields[field] = bigRatExactDecimal(*typed)
				}
			}
		}
	}
	return rows
}

const exprEnumSumOfRemainderDescription = "ExprEnumSumOf ordinals 1, 3, 4 and 5 (the remaining executions): sum-events-plus sums intBoxed over SupportBean_Container.beans with element/index/size lambda footprints and a case-when null branch; sum-scalar-string sums extractNum/extractBigDecimal UDF results over SupportCollection.strvals; sum-invalid records the two tryInvalidCompile message prefixes; sum-array sums constant Double, BigInteger and nullable Long collections. Each listener case deploys s0 once, sends every assertion event, then undeploys."

// exprEnumSumOfRemainderExpectedStep is one pinned schedule entry: a deploy,
// a send with an exact decoded payload, a build-error probe or undeploy-all.
type exprEnumSumOfRemainderExpectedStep struct {
	op          string
	statement   string
	eventType   string
	payload     any
	epl         string
	expectError string
}

// exprEnumSumOfRemainderSchedules pins each case's step sequence after its
// case marker: listener cases deploy s0, send every assertion event and
// undeploy; the invalid case runs its two path-less probes then undeploys.
var exprEnumSumOfRemainderSchedules = map[string][]exprEnumSumOfRemainderExpectedStep{
	exprEnumSumOfRemainderSumEventsPlusCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean_Container", payload: exprEnumSumOfRemainderContainer{}},
		{op: "send", eventType: "SupportBean_Container", payload: exprEnumSumOfRemainderContainer{Beans: []exprEnumSumOfRemainderBean{}}},
		{op: "send", eventType: "SupportBean_Container", payload: exprEnumSumOfRemainderContainer{Beans: []exprEnumSumOfRemainderBean{
			{TheString: "E1", IntBoxed: exprEnumSumOfRemainderInt64(10)},
		}}},
		{op: "send", eventType: "SupportBean_Container", payload: exprEnumSumOfRemainderContainer{Beans: []exprEnumSumOfRemainderBean{
			{TheString: "E1", IntBoxed: exprEnumSumOfRemainderInt64(10)},
			{TheString: "E2", IntBoxed: exprEnumSumOfRemainderInt64(11)},
		}}},
		{op: "undeploy-all"},
	},
	exprEnumSumOfRemainderSumScalarStringCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportCollection", payload: exprEnumSumOfRemainderCollection{Strvals: []string{"E2", "E1", "E5", "E4"}}},
		{op: "send", eventType: "SupportCollection", payload: exprEnumSumOfRemainderCollection{Strvals: []string{"E1"}}},
		{op: "send", eventType: "SupportCollection", payload: exprEnumSumOfRemainderCollection{}},
		{op: "send", eventType: "SupportCollection", payload: exprEnumSumOfRemainderCollection{Strvals: []string{}}},
		{op: "undeploy-all"},
	},
	exprEnumSumOfRemainderSumInvalidCase: {
		{op: "build-error", statement: "sumof-no-param",
			epl:         exprEnumSumOfRemainderProbeEPLs["sumof-no-param"],
			expectError: exprEnumSumOfRemainderProbeErrors["sumof-no-param"]},
		{op: "build-error", statement: "sumof-null-lambda",
			epl:         exprEnumSumOfRemainderProbeEPLs["sumof-null-lambda"],
			expectError: exprEnumSumOfRemainderProbeErrors["sumof-null-lambda"]},
		{op: "undeploy-all"},
	},
	exprEnumSumOfRemainderSumArrayCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean", payload: exprEnumSumOfRemainderBean{}},
		{op: "undeploy-all"},
	},
}

// loadExprEnumSumOfRemainderScenario decodes the checked-in scenario with
// strict raw-bytes validation: duplicate keys, unknown fields, metadata
// drift, and payload drift are all rejected before replay.
func loadExprEnumSumOfRemainderScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprEnumSumOfRemainderID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprEnumSumOfRemainderID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumSumOfRemainderID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumSumOfRemainderID, err)
	}
	if err := requireExprEnumSumOfRemainderFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprEnumSumOfRemainderID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprEnumSumOfRemainderID ||
		metadata.Description != exprEnumSumOfRemainderDescription ||
		metadata.JavaCommit != exprEnumSumOfRemainderJavaCommit ||
		metadata.JavaSource != exprEnumSumOfRemainderJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprEnumSumOfRemainderID)
	}
	if err := validateExprEnumSumOfRemainderStringArray(root["javaRuntimes"], exprEnumSumOfRemainderJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSumOfRemainderStringArray(root["javaNames"], exprEnumSumOfRemainderJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSumOfRemainderStringArray(root["javaStaticIds"], exprEnumSumOfRemainderJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSumOfRemainderStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprEnumSumOfRemainderCaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", exprEnumSumOfRemainderID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprEnumSumOfRemainderFields(object,
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
		if definition.Case != exprEnumSumOfRemainderCaseOrder[index] ||
			definition.Ordinal != exprEnumSumOfRemainderCaseOrdinals[index] ||
			definition.RuntimeID != exprEnumSumOfRemainderJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprEnumSumOfRemainderJavaExecutions[index] ||
			definition.Observation != exprEnumSumOfRemainderObservations[index] ||
			definition.EPL != exprEnumSumOfRemainderEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprEnumSumOfRemainderID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 22 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 22 steps", exprEnumSumOfRemainderID)
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
			if err := requireExprEnumSumOfRemainderFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprEnumSumOfRemainderFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireExprEnumSumOfRemainderFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireExprEnumSumOfRemainderFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeExprEnumSumOfRemainderStrictPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "build-error":
			if err := requireExprEnumSumOfRemainderFields(object, "op", "case", "statement", "epl", "expectError", "compileWithoutPath"); err != nil {
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
	if err := validateExprEnumSumOfRemainderScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSumOfRemainderSchedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateExprEnumSumOfRemainderSchedule walks the decoded steps and pins
// the full per-case schedule: case marker, then the pinned step sequence
// (deploy/sends/undeploy-all for listener cases, the two probes and
// undeploy-all for the invalid case).
func validateExprEnumSumOfRemainderSchedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range exprEnumSumOfRemainderCaseOrder {
		if index >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario ends before case %q", exprEnumSumOfRemainderID, caseName)
		}
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", exprEnumSumOfRemainderID, index, caseName)
		}
		index++
		for _, expected := range exprEnumSumOfRemainderSchedules[caseName] {
			if index >= len(scenario.Steps) {
				return fmt.Errorf("%s scenario case %d schedule is truncated", exprEnumSumOfRemainderID, caseIndex)
			}
			if err := validateExprEnumSumOfRemainderScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
				return fmt.Errorf("%s scenario case %d step %d: %w", exprEnumSumOfRemainderID, caseIndex, index, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", exprEnumSumOfRemainderID)
	}
	return nil
}

func validateExprEnumSumOfRemainderScheduleStep(step compat.Step, caseName string, expected exprEnumSumOfRemainderExpectedStep) error {
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
		payload, err := decodeExprEnumSumOfRemainderStrictPayload(step)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(payload, expected.payload) {
			return fmt.Errorf("send payload for %q is not pinned", step.EventType)
		}
	}
	return nil
}

// decodeExprEnumSumOfRemainderStrictPayload decodes a send payload with a
// strict field set so extra keys are rejected.
func decodeExprEnumSumOfRemainderStrictPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean_Container":
		if err := requireExprEnumSumOfRemainderFields(fields, "beans"); err != nil {
			return nil, err
		}
		if !exprEnumSumOfRemainderJSONNull(fields["beans"]) {
			var rawBeans []json.RawMessage
			if err := json.Unmarshal(fields["beans"], &rawBeans); err != nil {
				return nil, fmt.Errorf("decode SupportBean_Container beans: %w", err)
			}
			for beanIndex, rawBean := range rawBeans {
				var beanFields map[string]json.RawMessage
				if err := strictObject(rawBean, &beanFields); err != nil {
					return nil, fmt.Errorf("decode SupportBean_Container bean %d: %w", beanIndex, err)
				}
				if err := requireExprEnumSumOfRemainderFields(beanFields, "theString", "intBoxed"); err != nil {
					return nil, fmt.Errorf("decode SupportBean_Container bean %d: %w", beanIndex, err)
				}
			}
		}
		var value exprEnumSumOfRemainderContainer
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_Container: %w", err)
		}
		return value, nil
	case "SupportCollection":
		if err := requireExprEnumSumOfRemainderFields(fields, "strvals"); err != nil {
			return nil, err
		}
		if !exprEnumSumOfRemainderJSONNull(fields["strvals"]) {
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
		var value exprEnumSumOfRemainderCollection
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportCollection: %w", err)
		}
		return value, nil
	case "SupportBean":
		if err := requireExprEnumSumOfRemainderFields(fields); err != nil {
			return nil, err
		}
		var value exprEnumSumOfRemainderBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func exprEnumSumOfRemainderJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func requireExprEnumSumOfRemainderFields(object map[string]json.RawMessage, names ...string) error {
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

func validateExprEnumSumOfRemainderStringArray(raw json.RawMessage, expected []string, name string) error {
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
