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

type aggGroupedBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	CharPrimitive   int32    `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float64  `esper:"floatPrimitive"`
	IntBoxed        *int32   `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	CharBoxed       *int32   `esper:"charBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	EnumValue       *string  `esper:"enumValue"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
}

type aggGroupedMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

type aggGroupedBeanString struct {
	TheString string `esper:"theString"`
}

// aggGroupedIntArray mirrors SupportEventWithIntArray: int[] group keys
// compare by content (multikey-with-array semantics).
type aggGroupedIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

type aggGroupedStockAverages struct {
	Symbol   string  `esper:"symbol"`
	Average  float64 `esper:"average"`
	Sumation int64   `esper:"sumation"`
}

var resultSetQueryTypeAggregateGroupedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeAggregateGrouped.java",
}

// Inventory ordinal order: executions() registers CriteriaByDotMethod,
// IterateUnbound, UnaggregatedHaving, Wildcard, AggregationOverGroupedProps,
// SumOneView, SumJoin, InsertInto, MultikeyWArray.
var resultSetQueryTypeAggregateGroupedJavaRuntimeIDs = []string{
	"java-runtime-de8f7d7c8aef4f94257f",
	"java-runtime-f158e09462cf81dcaed6",
	"java-runtime-53a0852371cfa557bb19",
	"java-runtime-40a398cbeadf315e6404",
	"java-runtime-2b8ffb9e25212f96d12c",
	"java-runtime-0dd2d8188c6705b7e352",
	"java-runtime-0b86c8778cda88c48804",
	"java-runtime-f5ae7195e04ce1676ec4",
	"java-runtime-91048f4568185e6225f9",
}

var resultSetQueryTypeAggregateGroupedJavaExecutions = []string{
	"ResultSetCriteriaByDotMethod",
	"ResultSetIterateUnbound",
	"ResultSetUnaggregatedHaving",
	"ResultSetWildcard",
	"ResultSetAggregationOverGroupedProps",
	"ResultSetSumOneView",
	"ResultSetSumJoin",
	"ResultSetInsertInto",
	"ResultSetMultikeyWArray",
}

var resultSetQueryTypeAggregateGroupedCases = []string{
	"criteria-by-dot-method",
	"iterate-unbound",
	"unaggregated-having",
	"wildcard-min",
	"aggregation-over-grouped-props",
	"sum-one-view",
	"sum-join",
	"insert-into",
	"multikey-w-array",
}

func runResultSetQueryTypeAggregateGroupedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultSetQueryTypeAggregateGroupedCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetQueryTypeAggregateGroupedCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runResultSetQueryTypeAggregateGroupedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	seq := uint64(0)

	for _, registration := range []struct {
		typeName string
		fn       func() error
	}{
		{"SupportBean", func() error { _, err := esper.RegisterStruct[aggGroupedBean](env, "SupportBean"); return err }},
		{"SupportMarketDataBean", func() error {
			_, err := esper.RegisterStruct[aggGroupedMarketData](env, "SupportMarketDataBean")
			return err
		}},
		{"SupportBeanString", func() error {
			_, err := esper.RegisterStruct[aggGroupedBeanString](env, "SupportBeanString")
			return err
		}},
		{"SupportEventWithIntArray", func() error {
			_, err := esper.RegisterStruct[aggGroupedIntArray](env, "SupportEventWithIntArray")
			return err
		}},
		{"StockAverages", func() error {
			_, err := esper.RegisterStruct[aggGroupedStockAverages](env, "StockAverages")
			return err
		}},
	} {
		if err := registration.fn(); err != nil {
			return compat.Trace{}, fmt.Errorf("register %s: %w", registration.typeName, err)
		}
	}

	// planBuilder defers statement construction until deployment so the
	// insert-into case can deploy s1/s2 mid-sequence like the Java suite.
	type planBuilder struct {
		name   string
		build  func() (esper.Query, error)
		record bool
	}
	builders := map[string]planBuilder{}
	order := []string{}
	addBuilder := func(name string, record bool, build func() (esper.Query, error)) {
		builders[name] = planBuilder{name: name, build: build, record: record}
		order = append(order, name)
	}

	deployed := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
	deploy := func(name string) error {
		builder, ok := builders[name]
		if !ok {
			return fmt.Errorf("unknown plan %q", name)
		}
		if deployed[name] != nil {
			return nil
		}
		query, err := builder.build()
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		deployed[name] = deployment
		for _, statement := range deployment.Statements() {
			statements[statement.Name()] = statement
		}
		if !builder.record {
			return nil
		}
		for _, statement := range deployment.Statements() {
			captured := statement
			if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				newRows := compat.NormalizeResults(batch.New)
				oldRows := compat.NormalizeResults(batch.Old)
				aggGroupedRenderCharPrimitive(newRows)
				aggGroupedRenderCharPrimitive(oldRows)
				if len(newRows) == 0 && len(oldRows) == 0 {
					return nil
				}
				seq++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: captured.Name(),
					Sequence:  seq,
					Time:      batch.Time.UTC().Format(time.RFC3339),
					New:       newRows,
					Old:       oldRows,
				})
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	theString := esper.Field[aggGroupedBean, string]("theString")
	intPrimitive := esper.Field[aggGroupedBean, int]("intPrimitive")
	longPrimitive := esper.Field[aggGroupedBean, int64]("longPrimitive")
	symbol := esper.Field[aggGroupedMarketData, string]("symbol")
	price := esper.Field[aggGroupedMarketData, float64]("price")
	volume := esper.Field[aggGroupedMarketData, int64]("volume")

	switch caseName {
	case "criteria-by-dot-method":
		addBuilder("s0", true, func() (esper.Query, error) {
			// sb.getTheString() dot-method group key is the property read in
			// Go's fluent surface; the longPrimitive projection and the
			// length_batch flush order carry the observable contract.
			return esper.From[aggGroupedBean](env, "SupportBean").
				Window(esper.LengthBatch(2)).
				GroupBy(theString).
				Select(
					esper.Alias("c0", longPrimitive),
					esper.Alias("c1", esper.Sum[int](intPrimitive)),
				).Query(esper.StatementName("s0")), nil
		})
	case "iterate-unbound":
		addBuilder("s0", false, func() (esper.Query, error) {
			return esper.From[aggGroupedBean](env, "SupportBean").
				GroupBy(theString).
				Select(
					esper.Alias("c0", theString),
					esper.Alias("c1", esper.Sum[int](intPrimitive)),
				).Query(esper.StatementName("s0")), nil
		})
	case "unaggregated-having":
		addBuilder("s0", true, func() (esper.Query, error) {
			return esper.From[aggGroupedBean](env, "SupportBean").
				GroupBy(theString).
				Select(esper.Alias("theString", theString)).
				Having(esper.Greater[int](intPrimitive, esper.Literal(5))).
				Query(esper.StatementName("s0")), nil
		})
	case "wildcard-min":
		addBuilder("s0", true, func() (esper.Query, error) {
			// Java `select *` flattens the full SupportBean surface plus
			// minval; the Go surface spells every column explicitly so the
			// normalized row carries the identical field set.
			return esper.From[aggGroupedBean](env, "SupportBean").
				Window(esper.LengthWindow(2)).
				GroupBy(theString).
				Select(
					esper.Alias("theString", theString),
					esper.Alias("boolPrimitive", esper.Field[aggGroupedBean, bool]("boolPrimitive")),
					esper.Alias("intPrimitive", intPrimitive),
					esper.Alias("longPrimitive", longPrimitive),
					esper.Alias("doublePrimitive", esper.Field[aggGroupedBean, float64]("doublePrimitive")),
					esper.Alias("charPrimitive", esper.Field[aggGroupedBean, int32]("charPrimitive")),
					esper.Alias("shortPrimitive", esper.Field[aggGroupedBean, int16]("shortPrimitive")),
					esper.Alias("bytePrimitive", esper.Field[aggGroupedBean, int8]("bytePrimitive")),
					esper.Alias("floatPrimitive", esper.Field[aggGroupedBean, float64]("floatPrimitive")),
					esper.Alias("intBoxed", esper.Field[aggGroupedBean, *int32]("intBoxed")),
					esper.Alias("longBoxed", esper.Field[aggGroupedBean, *int64]("longBoxed")),
					esper.Alias("shortBoxed", esper.Field[aggGroupedBean, *int16]("shortBoxed")),
					esper.Alias("byteBoxed", esper.Field[aggGroupedBean, *int8]("byteBoxed")),
					esper.Alias("charBoxed", esper.Field[aggGroupedBean, *int32]("charBoxed")),
					esper.Alias("floatBoxed", esper.Field[aggGroupedBean, *float32]("floatBoxed")),
					esper.Alias("doubleBoxed", esper.Field[aggGroupedBean, *float64]("doubleBoxed")),
					esper.Alias("boolBoxed", esper.Field[aggGroupedBean, *bool]("boolBoxed")),
					esper.Alias("enumValue", esper.Field[aggGroupedBean, *string]("enumValue")),
					esper.Alias("bigDecimal", esper.Field[aggGroupedBean, *big.Rat]("bigDecimal")),
					esper.Alias("bigInteger", esper.Field[aggGroupedBean, *big.Int]("bigInteger")),
					esper.Alias("minval", esper.Min[int](intPrimitive)),
				).Query(esper.StatementName("s0")), nil
		})
	case "aggregation-over-grouped-props":
		addBuilder("s0", true, func() (esper.Query, error) {
			return esper.From[aggGroupedMarketData](env, "SupportMarketDataBean").
				Window(esper.LengthWindow(5)).
				GroupBy(symbol, price).
				Select(
					esper.Alias("volume", volume),
					esper.Alias("symbol", symbol),
					esper.Alias("price", price),
					esper.Alias("mycount", esper.Count[float64](price)),
				).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
		})
	case "sum-one-view", "sum-join":
		buildSum := func() (esper.Query, error) {
			if caseName == "sum-one-view" {
				return esper.From[aggGroupedMarketData](env, "SupportMarketDataBean").
					Window(esper.LengthWindow(3)).
					Filter(esper.Or(
						esper.Equal[string](symbol, esper.Literal("DELL")),
						esper.Or(
							esper.Equal[string](symbol, esper.Literal("IBM")),
							esper.Equal[string](symbol, esper.Literal("GE")),
						),
					)).
					GroupBy(symbol).
					Select(
						esper.Alias("symbol", symbol),
						esper.Alias("volume", volume),
						esper.Alias("mySum", esper.Sum[float64](price)),
					).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
			}
			oneSymbol := esper.JoinField[string](1, "symbol")
			oneVolume := esper.JoinField[int64](1, "volume")
			onePrice := esper.JoinField[float64](1, "price")
			joined := esper.Join(
				esper.From[aggGroupedBeanString](env, "SupportBeanString").Window(esper.LengthWindow(100)),
				esper.From[aggGroupedMarketData](env, "SupportMarketDataBean").Window(esper.LengthWindow(3)),
				esper.OnEqual(
					esper.Field[aggGroupedBeanString, string]("theString"),
					oneSymbol,
				),
			)
			return joined.GroupBy(oneSymbol).Select(
				esper.Alias("symbol", oneSymbol),
				esper.Alias("volume", oneVolume),
				esper.Alias("mySum", esper.Sum[float64](onePrice)),
			).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
		}
		addBuilder("s0", true, buildSum)
	case "insert-into":
		addBuilder("s0", true, func() (esper.Query, error) {
			return esper.From[aggGroupedMarketData](env, "SupportMarketDataBean").
				Window(esper.LengthWindow(3000)).
				Aggregate(
					esper.Alias("symbol", symbol),
					esper.Alias("average", esper.Avg[float64](price)),
					esper.Alias("sumation", esper.Sum[int64](volume)),
				).Query(esper.StatementName("s0")), nil
		})
		addBuilder("s1", false, func() (esper.Query, error) {
			return esper.From[aggGroupedMarketData](env, "SupportMarketDataBean").
				Window(esper.LengthWindow(3000)).
				Aggregate(
					esper.Alias("symbol", symbol),
					esper.Alias("average", esper.Avg[float64](price)),
					esper.Alias("sumation", esper.Sum[int64](volume)),
				).InsertInto("StockAverages", esper.StatementName("s1")), nil
		})
		addBuilder("s2", true, func() (esper.Query, error) {
			return esper.From[aggGroupedStockAverages](env, "StockAverages").
				Query(esper.StatementName("s2")), nil
		})
	case "multikey-w-array":
		addBuilder("s0", true, func() (esper.Query, error) {
			return esper.From[aggGroupedIntArray](env, "SupportEventWithIntArray").
				GroupBy(esper.Field[aggGroupedIntArray, []int]("array")).
				Select(
					esper.Alias("id", esper.Field[aggGroupedIntArray, string]("id")),
					esper.Alias("thesum", esper.Sum[int](esper.Field[aggGroupedIntArray, int]("value"))),
				).Query(esper.StatementName("s0")), nil
		})
	}

	// Deploy the case's initial statements (everything except lazily
	// deployed insert-into secondaries) before replaying sends.
	for _, name := range order {
		if caseName == "insert-into" && name != "s0" {
			continue
		}
		if err := deploy(name); err != nil {
			return trace, err
		}
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "deploy":
			for _, name := range strings.Split(step.Statement, "+") {
				if err := deploy(strings.TrimSpace(name)); err != nil {
					return trace, fmt.Errorf("deploy %s: %w", name, err)
				}
			}
		case "send":
			switch step.EventType {
			case "SupportBean":
				var payload struct {
					TheString     string `json:"theString"`
					IntPrimitive  int32  `json:"intPrimitive"`
					LongPrimitive int64  `json:"longPrimitive"`
				}
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return trace, fmt.Errorf("decode SupportBean: %w", err)
				}
				event := aggGroupedBean{
					TheString:     payload.TheString,
					IntPrimitive:  payload.IntPrimitive,
					LongPrimitive: payload.LongPrimitive,
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return trace, err
				}
			case "SupportMarketDataBean":
				var payload struct {
					Symbol string  `json:"symbol"`
					Price  float64 `json:"price"`
					Volume int64   `json:"volume"`
				}
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return trace, fmt.Errorf("decode SupportMarketDataBean: %w", err)
				}
				event := aggGroupedMarketData{
					Symbol: payload.Symbol,
					Price:  payload.Price,
					Volume: payload.Volume,
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return trace, err
				}
			case "SupportBeanString":
				var payload struct {
					TheString string `json:"theString"`
				}
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return trace, fmt.Errorf("decode SupportBeanString: %w", err)
				}
				if err := engine.Send(ctx, step.EventType, aggGroupedBeanString{TheString: payload.TheString}); err != nil {
					return trace, err
				}
			case "SupportEventWithIntArray":
				var payload struct {
					ID    string `json:"id"`
					Array []int  `json:"array"`
					Value int    `json:"value"`
				}
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return trace, fmt.Errorf("decode SupportEventWithIntArray: %w", err)
				}
				event := aggGroupedIntArray{ID: payload.ID, Array: payload.Array, Value: payload.Value}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return trace, err
				}
			default:
				return trace, fmt.Errorf("unsupported event type %q", step.EventType)
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown snapshot statement %q", step.Statement)
			}
			snapshot, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(snapshot.Results())
			aggGroupedRenderCharPrimitive(rows)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			seq++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Time:      "1970-01-01T00:00:00Z",
				Sequence:  seq,
				New:       rows,
			})
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

// aggGroupedRenderCharPrimitive renders the unset charPrimitive the way the
// Java oracle does: a single NUL character string instead of the numeric 0.
func aggGroupedRenderCharPrimitive(rows []compat.ResultRecord) {
	for _, row := range rows {
		if v, ok := row.Fields["charPrimitive"]; ok {
			if n, ok := v.(int32); ok && n == 0 {
				row.Fields["charPrimitive"] = "\u0000"
			}
		}
	}
}
