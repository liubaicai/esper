package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultsetAggregateFilteredAllBean is the typed SupportBean shape used by
// ResultSetAggregateAllAggFunctions.  The boxed integer remains a pointer so
// a missing/null intBoxed value is distinct from the primitive zero value.
type resultsetAggregateFilteredAllBean struct {
	IntBoxed        *int    `esper:"intBoxed"`
	BoolPrimitive   bool    `esper:"boolPrimitive"`
	FloatPrimitive  float32 `esper:"floatPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	ShortPrimitive  int16   `esper:"shortPrimitive"`
}

// resultsetAggregateFilteredAllNumeric mirrors SupportBeanNumeric while
// retaining BigInteger/BigDecimal precision all the way through evaluation.
type resultsetAggregateFilteredAllNumeric struct {
	BigInt big.Int `esper:"bigint"`
	BigDec big.Rat `esper:"bigdec"`
}

const resultsetAggregateFilteredAllJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFilteredAllJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFiltered.java",
}

var resultsetAggregateFilteredAllJavaRuntimeIDs = []string{
	"java-runtime-2287285f8221e7fc14d4",
}

var resultsetAggregateFilteredAllJavaExecutions = []string{
	"ResultSetAggregateAllAggFunctions",
}

const resultsetAggregateFilteredAllRuntimeURI = "java-runtime-2287285f8221e7fc14d4"

var resultsetAggregateFilteredAllCases = []string{
	"filtered-length3-all",
	"filtered-sums-length2",
	"filtered-stateless",
	"filtered-exact-big",
	"filtered-distinct-epl",
	"filtered-distinct-soda",
}

// runResultSetAggregateFilteredAllScenario replays the one fixed Java
// execution as six isolated subcases.  The Java execution creates a fresh
// statement between each phase; the scenario therefore has only case/send
// operations and each case gets a fresh environment and runtime here.
func runResultSetAggregateFilteredAllScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetAggregateFilteredAllCases))
	for _, caseName := range resultsetAggregateFilteredAllCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateFilteredAllCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-aggregate-filtered-all case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("resultset-aggregate-filtered-all scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateFilteredAllCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFilteredAllBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateFilteredAllNumeric](env, "SupportBeanNumeric"); err != nil {
		return compat.Trace{}, err
	}

	var query esper.Query
	switch caseName {
	case "filtered-length3-all":
		stream := esper.From[resultsetAggregateFilteredAllBean](env, "SupportBean").Window(esper.LengthWindow(3))
		intBoxed := esper.Cast[*int, int](esper.Field[resultsetAggregateFilteredAllBean, *int]("intBoxed"))
		predicate := esper.Field[resultsetAggregateFilteredAllBean, bool]("boolPrimitive")
		query = stream.Aggregate(
			esper.Alias("cavedev", esper.FilterAggregate[float64](esper.Avedev[int](intBoxed), predicate)),
			esper.Alias("cavg", esper.FilterAggregate[float64](esper.Avg[int](intBoxed), predicate)),
			esper.Alias("cmax", esper.FilterAggregate[int](esper.Max[int](intBoxed), predicate)),
			esper.Alias("cmedian", esper.FilterAggregate[float64](esper.Median[int](intBoxed), predicate)),
			esper.Alias("cmin", esper.FilterAggregate[int](esper.Min[int](intBoxed), predicate)),
			esper.Alias("cstddev", esper.FilterAggregate[float64](esper.StdDev[int](intBoxed), predicate)),
			esper.Alias("csum", esper.FilterAggregate[int](esper.Sum[int](intBoxed), predicate)),
			esper.Alias("cfmaxever", esper.FilterAggregate[int](esper.MaxEver[int](intBoxed), predicate)),
			esper.Alias("cfminever", esper.FilterAggregate[int](esper.MinEver[int](intBoxed), predicate)),
		).Query(esper.StatementName("s0"))
	case "filtered-sums-length2":
		stream := esper.From[resultsetAggregateFilteredAllBean](env, "SupportBean").Window(esper.LengthWindow(2))
		predicate := esper.Field[resultsetAggregateFilteredAllBean, bool]("boolPrimitive")
		query = stream.Aggregate(
			esper.Alias("c1", esper.FilterAggregate[float32](esper.Sum[float32](esper.Field[resultsetAggregateFilteredAllBean, float32]("floatPrimitive")), predicate)),
			esper.Alias("c2", esper.FilterAggregate[float64](esper.Sum[float64](esper.Field[resultsetAggregateFilteredAllBean, float64]("doublePrimitive")), predicate)),
			esper.Alias("c3", esper.FilterAggregate[int64](esper.Sum[int64](esper.Field[resultsetAggregateFilteredAllBean, int64]("longPrimitive")), predicate)),
			esper.Alias("c4", esper.FilterAggregate[int16](esper.Sum[int16](esper.Field[resultsetAggregateFilteredAllBean, int16]("shortPrimitive")), predicate)),
		).Query(esper.StatementName("s0"))
	case "filtered-stateless":
		stream := esper.From[resultsetAggregateFilteredAllBean](env, "SupportBean")
		intBoxed := esper.Cast[*int, int](esper.Field[resultsetAggregateFilteredAllBean, *int]("intBoxed"))
		predicate := esper.Field[resultsetAggregateFilteredAllBean, bool]("boolPrimitive")
		query = stream.Aggregate(
			esper.Alias("c1", esper.FilterAggregate[int](esper.Max[int](intBoxed), predicate)),
			esper.Alias("c2", esper.FilterAggregate[int](esper.Min[int](intBoxed), predicate)),
		).Query(esper.StatementName("s0"))
	case "filtered-exact-big":
		stream := esper.From[resultsetAggregateFilteredAllNumeric](env, "SupportBeanNumeric").Window(esper.LengthWindow(2))
		bigInt := esper.Field[resultsetAggregateFilteredAllNumeric, big.Int]("bigint")
		bigDec := esper.Field[resultsetAggregateFilteredAllNumeric, big.Rat]("bigdec")
		var hundred big.Int
		hundred.SetInt64(100)
		predicate := esper.LessExact[big.Int](bigInt, esper.Literal(hundred))
		query = stream.Aggregate(
			esper.Alias("c1", esper.FilterAggregate[big.Rat](esper.AvgExact[big.Rat](bigDec), predicate)),
			esper.Alias("c2", esper.FilterAggregate[big.Rat](esper.SumExact[big.Rat](bigDec), predicate)),
			esper.Alias("c3", esper.FilterAggregate[big.Int](esper.SumExact[big.Int](bigInt), predicate)),
		).Query(esper.StatementName("s0"))
	case "filtered-distinct-epl", "filtered-distinct-soda":
		stream := esper.From[resultsetAggregateFilteredAllBean](env, "SupportBean").Window(esper.LengthWindow(3))
		intBoxed := esper.Cast[*int, int](esper.Field[resultsetAggregateFilteredAllBean, *int]("intBoxed"))
		predicate := esper.Field[resultsetAggregateFilteredAllBean, bool]("boolPrimitive")
		query = stream.Aggregate(
			esper.Alias("cavedev", resultsetAggregateFilteredAllDistinctFloat(esper.Avedev[int](intBoxed), intBoxed, predicate)),
			esper.Alias("cavg", resultsetAggregateFilteredAllDistinctFloat(esper.Avg[int](intBoxed), intBoxed, predicate)),
			esper.Alias("cmax", resultsetAggregateFilteredAllDistinctInt(esper.Max[int](intBoxed), intBoxed, predicate)),
			esper.Alias("cmedian", resultsetAggregateFilteredAllDistinctFloat(esper.Median[int](intBoxed), intBoxed, predicate)),
			esper.Alias("cmin", resultsetAggregateFilteredAllDistinctInt(esper.Min[int](intBoxed), intBoxed, predicate)),
			esper.Alias("cstddev", resultsetAggregateFilteredAllDistinctFloat(esper.StdDev[int](intBoxed), intBoxed, predicate)),
			esper.Alias("csum", resultsetAggregateFilteredAllDistinctInt(esper.Sum[int](intBoxed), intBoxed, predicate)),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-aggregate-filtered-all case %q", caseName)
	}

	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFilteredAllRuntimeURI),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one resultset aggregate statement, got %d", len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFilteredAllPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-aggregate-filtered-all statement %q", name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	if caseName == "filtered-exact-big" {
		for index := range trace.Records {
			trace.Records[index].New = normalizeResultSetAggregateFilteredAllBigNumbers(trace.Records[index].New)
			trace.Records[index].Old = normalizeResultSetAggregateFilteredAllBigNumbers(trace.Records[index].Old)
		}
	}
	return trace, nil
}

func resultsetAggregateFilteredAllDistinctFloat(
	aggregate esper.AggregateExpression[float64], input esper.Expr, predicate esper.Expression[bool],
) esper.AggregateExpression[float64] {
	return esper.FilterAggregate[float64](esper.DistinctAggregate[float64](aggregate, input), predicate)
}

func resultsetAggregateFilteredAllDistinctInt(
	aggregate esper.AggregateExpression[int], input esper.Expr, predicate esper.Expression[bool],
) esper.AggregateExpression[int] {
	return esper.FilterAggregate[int](esper.DistinctAggregate[int](aggregate, input), predicate)
}

func decodeResultSetAggregateFilteredAllPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value resultsetAggregateFilteredAllBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBeanNumeric":
		var raw struct {
			BigInt json.RawMessage `json:"bigint"`
			BigDec json.RawMessage `json:"bigdec"`
		}
		if err := json.Unmarshal(step.Payload, &raw); err != nil {
			return nil, fmt.Errorf("decode SupportBeanNumeric: %w", err)
		}
		bigIntText, err := resultsetAggregateFilteredAllExactText(raw.BigInt)
		if err != nil {
			return nil, fmt.Errorf("decode bigint: %w", err)
		}
		bigDecText, err := resultsetAggregateFilteredAllExactText(raw.BigDec)
		if err != nil {
			return nil, fmt.Errorf("decode bigdec: %w", err)
		}
		value := resultsetAggregateFilteredAllNumeric{}
		if _, ok := value.BigInt.SetString(bigIntText, 10); !ok {
			return nil, fmt.Errorf("invalid bigint literal %q", bigIntText)
		}
		if _, ok := value.BigDec.SetString(bigDecText); !ok {
			return nil, fmt.Errorf("invalid bigdec literal %q", bigDecText)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset-aggregate-filtered-all event type %q", step.EventType)
	}
}

func resultsetAggregateFilteredAllExactText(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", fmt.Errorf("exact numeric value is required")
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("exact numeric value is required")
		}
		return text, nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return "", err
	}
	return number.String(), nil
}

func normalizeResultSetAggregateFilteredAllBigNumbers(rows []compat.ResultRecord) []compat.ResultRecord {
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
				rows[rowIndex].Fields[field] = resultsetAggregateFilteredAllBigRatText(&typed)
			case *big.Rat:
				if typed == nil {
					rows[rowIndex].Fields[field] = nil
				} else {
					rows[rowIndex].Fields[field] = resultsetAggregateFilteredAllBigRatText(typed)
				}
			}
		}
	}
	return rows
}

func resultsetAggregateFilteredAllBigRatText(value *big.Rat) string {
	if value.IsInt() {
		return value.Num().String()
	}
	return value.String()
}
