package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultsetAggregateFirstLastWindowStarBean mirrors all properties exposed by
// the Java SupportBean. The replay payload supplies only the fields each
// execution sends; the remaining primitive fields retain Java's zero defaults
// and boxed fields remain null. CharPrimitive is a string so Java's default
// '\u0000' renders as a character rather than the numeric Go zero.
type resultsetAggregateFirstLastWindowStarBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int32   `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// resultsetAggregateFirstLastWindowStarPayload carries the fields each send
// supplies; doublePrimitive is present only for the unbounded-stream case.
type resultsetAggregateFirstLastWindowStarPayload struct {
	TheString       string  `json:"theString"`
	IntPrimitive    int32   `json:"intPrimitive"`
	DoublePrimitive float64 `json:"doublePrimitive"`
}

const resultsetAggregateFirstLastWindowStarJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetAggregateFirstLastWindowStarJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFirstLastWindow.java",
}

var (
	// Inventory order is authoritative: Star is ordinal 0, UnboundedStream is
	// ordinal 2 and LastMaxMixedOnSelect is ordinal 20 in
	// ResultSetAggregateFirstLastWindow.executions().
	resultsetAggregateFirstLastWindowStarJavaRuntimeIDs = []string{
		"java-runtime-00be68da7736fcafb968", // ResultSetAggregateStar
		"java-runtime-da477a833deb82cf0225", // ResultSetAggregateUnboundedStream
		"java-runtime-edb70b7eb3a9cd4217e8", // ResultSetAggregateLastMaxMixedOnSelect
	}
	resultsetAggregateFirstLastWindowStarJavaExecutions = []string{
		"ResultSetAggregateStar",
		"ResultSetAggregateUnboundedStream",
		"ResultSetAggregateLastMaxMixedOnSelect",
	}
)

const (
	resultsetAggregateFirstLastWindowStarCase         = "star"
	resultsetAggregateFirstLastWindowUnboundedCase    = "unbounded-stream"
	resultsetAggregateFirstLastWindowOnSelectCase     = "last-max-on-select"
	resultsetAggregateFirstLastWindowStarScenarioName = "resultset-aggregate-firstlastwindow-star"
)

// runResultSetAggregateFirstLastWindowStarScenario replays the three star,
// unbounded-stream and on-select executions from
// ResultSetAggregateFirstLastWindow. Each case receives a fresh environment
// and runtime, matching the isolated Java execution lifecycle.
func runResultSetAggregateFirstLastWindowStarScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		resultsetAggregateFirstLastWindowStarCase,
		resultsetAggregateFirstLastWindowUnboundedCase,
		resultsetAggregateFirstLastWindowOnSelectCase,
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetAggregateFirstLastWindowStarCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateFirstLastWindowStarScenarioName, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetAggregateFirstLastWindowStarScenarioName, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateFirstLastWindowStarCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFirstLastWindowStarBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	var plans []esper.Plan
	switch caseName {
	case resultsetAggregateFirstLastWindowStarCase:
		// Ordinal 0: star-form first/last/window/firstever/lastever over a
		// length-2 window. The Java execution runs compileDeploy then
		// eplToModelCompileDeploy under one runtime id; the compile path is
		// not observable, so a single pass is replayed.
		plans = append(plans, mustBuildFirstLastWindowStarPlan(env,
			esper.From[resultsetAggregateFirstLastWindowStarBean](env, "SupportBean").
				Window(esper.LengthWindow(2)).
				Aggregate(
					esper.Alias("firststar", esper.FirstEventValue()),
					esper.Alias("firststarsb", esper.FirstEventValue()),
					esper.Alias("laststar", esper.LastEventValue()),
					esper.Alias("laststarsb", esper.LastEventValue()),
					esper.Alias("windowstar", esper.WindowEvents()),
					esper.Alias("windowstarsb", esper.WindowEvents()),
					esper.Alias("firsteverstar", esper.FirstEver[esper.Event](esper.EventValue[esper.Event]())),
					esper.Alias("lasteverstar", esper.LastEver[esper.Event](esper.EventValue[esper.Event]())),
				).
				Query(esper.StatementName("s0"))))
	case resultsetAggregateFirstLastWindowUnboundedCase:
		// Ordinal 2: with no data window first/last behave as ever
		// aggregates, so the Go form pins FirstEver/LastEver.
		theString := esper.Field[resultsetAggregateFirstLastWindowStarBean, string]("theString")
		plans = append(plans, mustBuildFirstLastWindowStarPlan(env,
			esper.From[resultsetAggregateFirstLastWindowStarBean](env, "SupportBean").
				Aggregate(
					esper.Alias("f1", esper.FirstEver[string](theString)),
					esper.Alias("f2", esper.FirstEver[esper.Event](esper.EventValue[esper.Event]())),
					esper.Alias("f3", esper.FirstEver[esper.Event](esper.EventValue[esper.Event]())),
					esper.Alias("l1", esper.LastEver[string](theString)),
					esper.Alias("l2", esper.LastEver[esper.Event](esper.EventValue[esper.Event]())),
					esper.Alias("l3", esper.LastEver[esper.Event](esper.EventValue[esper.Event]())),
				).
				Query(esper.StatementName("s0"))))
	case resultsetAggregateFirstLastWindowOnSelectCase:
		// Ordinal 20: a keep-all named window fed by an A%-filtered
		// insert-into, observed by a constant-key grouped on-select that
		// emits one aggregate row per B% trigger.
		windowSchema, err := esper.StructSchema[resultsetAggregateFirstLastWindowStarBean]("MyWindowOne")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowOne", windowSchema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[resultsetAggregateFirstLastWindowStarBean, string]("theString")
		intPrimitive := esper.Field[resultsetAggregateFirstLastWindowStarBean, int32]("intPrimitive")
		insert := esper.OnEvent(esper.From[resultsetAggregateFirstLastWindowStarBean](env, "SupportBean").
			Filter(esper.Like(theString, esper.Literal("A%")))).
			InsertIntoNamedWindow("MyWindowOne", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert"))
		onSelect := esper.OnEvent(esper.From[resultsetAggregateFirstLastWindowStarBean](env, "SupportBean").
			Filter(esper.Like(theString, esper.Literal("B%")))).
			SelectFromNamedWindowGroupBy("MyWindowOne", nil,
				[]esper.Expr{esper.Literal(1)},
				esper.Alias("li", esper.Last[int32](intPrimitive)),
				esper.Alias("mi", esper.Max[int32](intPrimitive)),
			).Query(esper.StatementName("s0"))
		for _, built := range []esper.Query{insert, onSelect} {
			plans = append(plans, mustBuildFirstLastWindowStarPlan(env, built))
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetAggregateFirstLastWindowStarScenarioName, caseName)
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateFirstLastWindowStarRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	var statement *esper.Statement
	for index, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		if index == len(plans)-1 {
			statements := deployment.Statements()
			if len(statements) != 1 || statements[0].Name() != "s0" {
				return compat.Trace{}, fmt.Errorf("%s deployment did not expose statement s0", resultsetAggregateFirstLastWindowStarScenarioName)
			}
			statement = statements[0]
		}
	}
	if statement == nil {
		return compat.Trace{}, fmt.Errorf("%s statement s0 was not deployed", resultsetAggregateFirstLastWindowStarScenarioName)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeResultSetAggregateFirstLastWindowStarPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateFirstLastWindowStarScenarioName, name)
		}
		return statement, nil
	})
}

func mustBuildFirstLastWindowStarPlan(env *esper.Environment, query esper.Query) esper.Plan {
	plan, err := env.Build(query)
	if err != nil {
		panic(err)
	}
	return plan
}

func resultsetAggregateFirstLastWindowStarRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetAggregateFirstLastWindowStarCase:
		return resultsetAggregateFirstLastWindowStarJavaRuntimeIDs[0]
	case resultsetAggregateFirstLastWindowUnboundedCase:
		return resultsetAggregateFirstLastWindowStarJavaRuntimeIDs[1]
	case resultsetAggregateFirstLastWindowOnSelectCase:
		return resultsetAggregateFirstLastWindowStarJavaRuntimeIDs[2]
	default:
		return "parity-" + resultsetAggregateFirstLastWindowStarScenarioName + "-" + caseName
	}
}

func decodeResultSetAggregateFirstLastWindowStarPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateFirstLastWindowStarScenarioName, step.EventType)
	}
	var payload resultsetAggregateFirstLastWindowStarPayload
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return resultsetAggregateFirstLastWindowStarBean{
		TheString:       payload.TheString,
		IntPrimitive:    payload.IntPrimitive,
		DoublePrimitive: payload.DoublePrimitive,
		CharPrimitive:   "\u0000",
	}, nil
}
