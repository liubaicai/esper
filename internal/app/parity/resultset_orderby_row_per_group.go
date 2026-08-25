package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetOrderbyRowPerGroupMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

type resultsetOrderbyRowPerGroupString struct {
	TheString string `esper:"theString"`
}

type resultsetOrderbyRowPerGroupBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const resultsetOrderbyRowPerGroupJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOrderbyRowPerGroupJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerGroup.java",
}

// Case order fixes the runtime-ID index mapping below. The nine executions
// share one Java class, one scenario, one oracle and one runner surface, so
// the unit ports them together (the ECSM multi-runtime precedent).
var resultsetOrderbyRowPerGroupJavaRuntimeIDs = []string{
	"java-runtime-0378ad5f02530ec9a035",
	"java-runtime-76c087fbdf95d5f7880b",
	"java-runtime-c2f6f59e3e46c835a29e",
	"java-runtime-e07133b5b6253e493ded",
	"java-runtime-252daca2010790c61878",
	"java-runtime-9d22340c3f0b36d2a3f7",
	"java-runtime-3124b316394d884cf8f3",
	"java-runtime-fbb5b1ed4cd702566ff1",
	"java-runtime-be39f23e1ad826e99352",
}

var resultsetOrderbyRowPerGroupJavaExecutions = []string{
	"ResultSetNoHavingNoJoin",
	"ResultSetHavingNoJoin",
	"ResultSetNoHavingJoin",
	"ResultSetHavingJoin",
	"ResultSetHavingJoinAlias",
	"ResultSetLast",
	"ResultSetLastJoin",
	"ResultSetIteratorRowPerGroup",
	"ResultSetOrderByLast",
}

var resultsetOrderbyRowPerGroupCases = []string{
	"no-having-no-join",
	"having-no-join",
	"no-having-join",
	"having-join",
	"having-join-alias",
	"last",
	"last-join",
	"iterator-row-per-group",
	"order-by-last",
}

// runResultsetOrderbyRowPerGroupScenario replays the nine grouped
// row-per-group executions: irstream symbol/sum(price) over length(20)
// [joined with SupportBeanString#length(100)] with having and output-last
// variants, the continuous join observed through iterator snapshots, and
// the length_batch order-by-last bean case. Order keys are expressed as
// ResultField projections because the output-every boundary re-sort
// evaluates keys against the projected rows, reproducing Java's global
// buffered-batch sort (a direct aggregate key is inert in that context);
// alias vs expression spelling is observationally identical in the pinned
// expectations.
func runResultsetOrderbyRowPerGroupScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbyRowPerGroupCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbyRowPerGroupCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset orderby-row-per-group case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("resultset orderby-row-per-group scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbyRowPerGroupCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbyRowPerGroupMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	join := caseName == "no-having-join" || caseName == "having-join" || caseName == "having-join-alias" || caseName == "last-join" || caseName == "iterator-row-per-group"
	if join {
		if _, err := esper.RegisterStruct[resultsetOrderbyRowPerGroupString](env, "SupportBeanString"); err != nil {
			return compat.Trace{}, err
		}
	}
	if caseName == "order-by-last" {
		if _, err := esper.RegisterStruct[resultsetOrderbyRowPerGroupBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	having := caseName == "having-no-join" || caseName == "having-join" || caseName == "having-join-alias"

	market := esper.From[resultsetOrderbyRowPerGroupMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(20))
	symbol := esper.Field[resultsetOrderbyRowPerGroupMarket, string]("symbol")
	price := esper.Field[resultsetOrderbyRowPerGroupMarket, float64]("price")
	sum := esper.Sum[float64](price)

	// Order keys are ResultField projections: the output-every boundary
	// re-sort evaluates keys against the projected rows (Java re-sorts the
	// whole buffered batch globally; a direct aggregate key would be inert
	// in that context).
	orderedOptions := func(order ...esper.SortKey) []esper.QueryOption {
		return []esper.QueryOption{
			esper.StatementName("s0"),
			esper.WithOldStream(),
			esper.OrderBy(order...),
		}
	}

	var query esper.Query
	switch caseName {
	case "no-having-no-join", "having-no-join":
		aggregate := market.GroupBy(symbol).
			Select(
				esper.Alias("symbol", symbol),
				esper.Alias("mysum", sum),
			)
		if having {
			aggregate = aggregate.Having(esper.Greater[float64](sum, esper.Literal(0.0)))
		}
		query = aggregate.Query(append(orderedOptions(
			esper.Ascending(esper.ResultField[float64]("mysum")),
			esper.Ascending(esper.ResultField[string]("symbol")),
		), esper.WithOutput(esper.OutputEvery(6)))...)
	case "no-having-join", "having-join", "having-join-alias":
		seed := esper.From[resultsetOrderbyRowPerGroupString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		seedString := esper.Field[resultsetOrderbyRowPerGroupString, string]("theString")
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		joinStream := esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
			GroupBy(esper.JoinField[string](0, "symbol")).
			Select(
				esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
				esper.Alias("mysum", joinSum),
			)
		if having {
			joinStream = joinStream.Having(esper.Greater[float64](joinSum, esper.Literal(0.0)))
		}
		query = joinStream.Query(append(orderedOptions(
			esper.Ascending(esper.ResultField[float64]("mysum")),
			esper.Ascending(esper.ResultField[string]("symbol")),
		), esper.WithOutput(esper.OutputEvery(6)))...)
	case "last", "last-join":
		outputLast := []esper.QueryOption{esper.WithOutput(esper.OutputLastEveryEvents(6))}
		if caseName == "last" {
			aggregate := market.GroupBy(symbol).
				Select(
					esper.Alias("symbol", symbol),
					esper.Alias("mysum", sum),
				)
			query = aggregate.Query(append(orderedOptions(
				esper.Ascending(esper.ResultField[float64]("mysum")),
				esper.Ascending(esper.ResultField[string]("symbol")),
			), outputLast...)...)
		} else {
			seed := esper.From[resultsetOrderbyRowPerGroupString](env, "SupportBeanString").Window(esper.LengthWindow(100))
			seedString := esper.Field[resultsetOrderbyRowPerGroupString, string]("theString")
			joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
			query = esper.Join(market, seed, esper.OnEqual(symbol, seedString)).
				GroupBy(esper.JoinField[string](0, "symbol")).
				Select(
					esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
					esper.Alias("mysum", joinSum),
				).Query(append(orderedOptions(
				esper.Ascending(esper.ResultField[float64]("mysum")),
				esper.Ascending(esper.ResultField[string]("symbol")),
			), outputLast...)...)
		}
	case "iterator-row-per-group":
		seed := esper.From[resultsetOrderbyRowPerGroupString](env, "SupportBeanString").Window(esper.LengthWindow(100))
		seedString := esper.Field[resultsetOrderbyRowPerGroupString, string]("theString")
		joinSum := esper.Sum[float64](esper.JoinField[float64](0, "price"))
		query = esper.Join(esper.From[resultsetOrderbyRowPerGroupMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(10)), seed, esper.OnEqual(symbol, seedString)).
			GroupBy(esper.JoinField[string](0, "symbol")).
			Select(
				esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
				esper.Alias("sumPrice", joinSum),
			).Query(esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(esper.ResultField[string]("symbol"))),
		)
	case "order-by-last":
		bean := esper.From[resultsetOrderbyRowPerGroupBean](env, "SupportBean").Window(esper.LengthBatch(5))
		theString := esper.Field[resultsetOrderbyRowPerGroupBean, string]("theString")
		intPrimitive := esper.Field[resultsetOrderbyRowPerGroupBean, int]("intPrimitive")
		query = bean.GroupBy(theString).
			Select(
				esper.Alias("c0", esper.Last[int](intPrimitive)),
				esper.Alias("c1", theString),
			).Query(esper.StatementName("s0"),
			// The pinned EPL carries no irstream keyword: the length_batch
			// flush is insert-stream only (assertPropsPerRowNewOnly).
			esper.OrderBy(esper.Descending(esper.ResultField[int]("c0"))),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset orderby-row-per-group case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbyRowPerGroupRuntimeID(caseName)))
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("resultset orderby-row-per-group case %q deployed %d statements", caseName, len(statements))
	}
	statement := statements[0]
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		func(step compat.Step) (any, error) {
			return decodeResultsetOrderbyRowPerGroupPayload(step)
		}, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown resultset orderby-row-per-group statement %q", name)
			}
			return statement, nil
		})
}

func decodeResultsetOrderbyRowPerGroupPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value resultsetOrderbyRowPerGroupMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBeanString":
		var value resultsetOrderbyRowPerGroupString
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanString: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value resultsetOrderbyRowPerGroupBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported resultset orderby-row-per-group event type %q", step.EventType)
	}
}

func resultsetOrderbyRowPerGroupRuntimeID(caseName string) string {
	for index, name := range resultsetOrderbyRowPerGroupCases {
		if name == caseName {
			return resultsetOrderbyRowPerGroupJavaRuntimeIDs[index]
		}
	}
	return "resultset-orderby-row-per-group-unknown"
}
