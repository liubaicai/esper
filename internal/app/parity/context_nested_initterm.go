package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type contextNestedInitTermBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type contextNestedInitTermS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type contextNestedInitTermS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

const contextNestedInitTermJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextNestedInitTermJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextNested.java",
}

var contextNestedInitTermJavaRuntimeIDs = []string{
	"java-runtime-8598d1eb6dbd61614f4f",
	"java-runtime-9539162a80f17616650f",
	"java-runtime-c91629ba5a771e1725db",
	"java-runtime-0c5d14d415a1162de7e4",
	"java-runtime-754484318bed39b8eea2",
}

var contextNestedInitTermJavaExecutions = []string{
	"ContextNestedPartitionedWithFilterOverlap",
	"ContextNestedPartitionedWithFilterNonOverlap",
	"ContextNestedPartitionWithMultiPropsAndTerm",
	"ContextNestedCategoryOverInitTermDistinct",
	"ContextNestedKeySegmentedWInitTermEndEvent",
}

var contextNestedInitTermCaseRuntimeIDs = map[string]string{
	"filter-overlap":               "java-runtime-8598d1eb6dbd61614f4f",
	"filter-nonoverlap-broadcast":  "java-runtime-9539162a80f17616650f",
	"multikey-correlated-term":     "java-runtime-c91629ba5a771e1725db",
	"category-initterm-distinct":   "java-runtime-0c5d14d415a1162de7e4",
	"initterm-endevent-projection": "java-runtime-754484318bed39b8eea2",
}

// runContextNestedInitTermScenario replays the five ContextNested
// initiated-terminated child executions: a segmented parent plus a
// start/end or initiated child whose lifecycle is scoped to the routed
// parent partitions.
func runContextNestedInitTermScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextNestedInitTermCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-nested-initterm case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextNestedInitTermCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextNestedInitTermBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextNestedInitTermS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextNestedInitTermS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	beanSource := esper.From[contextNestedInitTermBean](env, "SupportBean")
	s0Source := esper.From[contextNestedInitTermS0](env, "SupportBean_S0")
	theString := esper.Field[contextNestedInitTermBean, string]("theString")
	intPrimitive := esper.Field[contextNestedInitTermBean, int]("intPrimitive")
	longPrimitive := esper.Field[contextNestedInitTermBean, int64]("longPrimitive")
	s0ID := esper.Field[contextNestedInitTermS0, int]("id")
	s0P00 := esper.Field[contextNestedInitTermS0, string]("p00")
	isBean := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean"))
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	isS1 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1"))

	var plan esper.Plan
	subscriber := false
	switch caseName {
	case "filter-overlap":
		// context CtxSession partition by id from SupportBean_S0,
		// context CtxStartEnd start SupportBean_S0 as te end SupportBean_S1(id=te.id)
		parent, err := esper.CreateKeyContextByStreams(env, "CtxSession",
			esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{s0ID}})
		if err != nil {
			return compat.Trace{}, err
		}
		child, err := esper.NewInitiatedTerminatedContext("CtxStartEnd",
			esper.Literal("global"), isS0,
			esper.And(isS1,
				esper.Equal[int](esper.Field[contextNestedInitTermS1, int]("id"),
					esper.Property[int](esper.ContextInitiatingEvent(), "id"))))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNestedContext(env, "TheContext", parent.Name(), child); err != nil {
			return compat.Trace{}, err
		}
		query := esper.Join(
			s0Source.Window(esper.FirstEvent()).As("firstEvent"),
			s0Source.Window(esper.LastEvent()).As("lastEvent"),
		).Select(
			esper.SelectFrom(0, "firstEvent", esper.JoinEventValue[esper.Event](0)),
		).Query(esper.StatementName("s0"), esper.WithContext("TheContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
		subscriber = true
	case "filter-nonoverlap-broadcast":
		// context SegByString as partition by theString from SupportBean(intPrimitive > 0),
		// context InitCtx initiated by SupportBean_S0 as s0 terminated after 60 seconds
		parent, err := esper.CreateKeyContextByStreams(env, "SegByString",
			esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString},
				Filter: esper.Greater[int](intPrimitive, esper.Literal(0))})
		if err != nil {
			return compat.Trace{}, err
		}
		child, err := esper.NewOverlappingPatternTerminatedContext("InitCtx",
			esper.Literal("global"), isS0,
			esper.TimerInterval(s0Source, 60*time.Second))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNestedContext(env, "NestedContext", parent.Name(), child); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.GroupBy(theString).Select(
			esper.Alias("c0", esper.Property[string](esper.ContextInitiatingEvent(), "p00")),
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("NestedContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "multikey-correlated-term":
		// context PartitionedByKeys partition by theString, intPrimitive from SupportBean,
		// context InitiateAndTerm start SupportBean as e1
		// end SupportBean_S0(id=e1.intPrimitive and p00=e1.theString)
		parent, err := esper.CreateKeyContextByStreams(env, "PartitionedByKeys",
			esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString, intPrimitive}})
		if err != nil {
			return compat.Trace{}, err
		}
		child, err := esper.NewInitiatedTerminatedContext("InitiateAndTerm",
			esper.Literal("global"), isBean,
			esper.And(isS0,
				esper.And(
					esper.Equal[int](s0ID, esper.Property[int](esper.ContextInitiatingEvent(), "intPrimitive")),
					esper.Equal[string](s0P00, esper.Property[string](esper.ContextInitiatingEvent(), "theString")))))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNestedContext(env, "NestedContext", parent.Name(), child); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Aggregate(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", esper.Count[int64](longPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("NestedContext"),
			esper.WithOutput(esper.OutputWhenTerminated(esper.OutputLast())))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "category-initterm-distinct":
		// context ACtx group by intPrimitive < 0 as grp1, group by intPrimitive = 0 as grp2,
		// group by intPrimitive > 0 as grp3 from SupportBean,
		// context BCtx initiated by distinct(a.intPrimitive) SupportBean(theString='A') as a
		// terminated by SupportBean(theString='B')
		grp1, err := esper.NewContextCategory("grp1", esper.Less[int](intPrimitive, esper.Literal(0)))
		if err != nil {
			return compat.Trace{}, err
		}
		grp2, err := esper.NewContextCategory("grp2", esper.Equal[int](intPrimitive, esper.Literal(0)))
		if err != nil {
			return compat.Trace{}, err
		}
		grp3, err := esper.NewContextCategory("grp3", esper.Greater[int](intPrimitive, esper.Literal(0)))
		if err != nil {
			return compat.Trace{}, err
		}
		parent, err := esper.CreateCategoryContext(env, "ACtx", grp1, grp2, grp3)
		if err != nil {
			return compat.Trace{}, err
		}
		child, err := esper.NewDistinctInitiatedTerminatedContext("BCtx",
			intPrimitive,
			esper.And(isBean, esper.Equal[string](theString, esper.Literal("A"))),
			esper.And(isBean, esper.Equal[string](theString, esper.Literal("B"))))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNestedContext(env, "NestedContext", parent.Name(), child); err != nil {
			return compat.Trace{}, err
		}
		query := beanSource.Filter(
			esper.And(
				esper.Equal[int](intPrimitive, esper.Property[int](esper.ContextInitiatingEvent(), "intPrimitive")),
				esper.NotEqual[string](theString, esper.Literal("B"))),
		).Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext("NestedContext"))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	case "initterm-endevent-projection":
		// context OuterContext partition by theString from SupportBean,
		// context InnerContext start SupportBean(intPrimitive = 1) as startevent
		// end SupportBean(intPrimitive = 0) as endevent
		parent, err := esper.CreateKeyContextByStreams(env, "OuterContext",
			esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}})
		if err != nil {
			return compat.Trace{}, err
		}
		child, err := esper.NewInitiatedTerminatedContext("InnerContext",
			esper.Literal("global"),
			esper.And(isBean, esper.Equal[int](intPrimitive, esper.Literal(1))),
			esper.And(isBean, esper.Equal[int](intPrimitive, esper.Literal(0))))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNestedContext(env, "MyContext", parent.Name(), child); err != nil {
			return compat.Trace{}, err
		}
		query := esper.Select(beanSource.Filter(
			esper.Greater[int](intPrimitive, esper.Literal(0)),
		),
			esper.Alias("id", esper.ContextID()),
			esper.Alias("c0", esper.ContextInitiatingEvent()),
			esper.Alias("c1", esper.ContextTerminatingEvent()),
		).Query(esper.StatementName("s0"), esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputWhenTerminated(esper.OutputAll())))
		plan, err = env.Build(query)
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-nested-initterm case %q", caseName)
	}

	engine := esper.NewEngine(env)
	initial, ok := initialAdvanceTime(scenario, caseName)
	if ok {
		initialTime, err := time.Parse(time.RFC3339Nano, initial)
		if err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
		if err := engine.AdvanceTime(ctx, initialTime); err != nil {
			_ = engine.Close(context.Background())
			return compat.Trace{}, err
		}
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	if len(deployment.Statements()) != 1 {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	defer func() { _ = engine.Close(context.Background()) }()
	if subscriber {
		return replayContextNestedInitTermSubscriber(ctx, engine, statement, caseScenario)
	}
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeContextNestedInitTermPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown context-nested-initterm statement %q", name)
		}
		return statement, nil
	})
}

// replayContextNestedInitTermSubscriber mirrors the Java SupportSubscriber
// binding for the filter-overlap case: the single-column `select firstEvent`
// projection delivers the selected event's underlying value per row, which
// the trace renders under the firstEvent column name.
func replayContextNestedInitTermSubscriber(ctx context.Context, engine *esper.Engine, statement *esper.Statement, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	caseName := ""
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			caseName = step.Case
			break
		}
	}
	var sequence uint64
	if err := statement.SetSubscriber(func(_ context.Context, update esper.SubscriberUpdate) error {
		if len(update.NewRows()) == 0 && len(update.OldRows()) == 0 {
			return nil
		}
		sequence++
		newRows := make([]compat.ResultRecord, 0, len(update.NewRows()))
		for _, row := range update.NewRows() {
			newRows = append(newRows, compat.NormalizeResults([]esper.Result{row.Result()})...)
		}
		oldRows := make([]compat.ResultRecord, 0, len(update.OldRows()))
		for _, row := range update.OldRows() {
			oldRows = append(oldRows, compat.NormalizeResults([]esper.Result{row.Result()})...)
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "subscriber",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      engine.Now().UTC().Format(time.RFC3339Nano),
			New:       newRows,
			Old:       oldRows,
		})
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}
	for _, step := range scenario.Steps {
		switch step.Op {
		case "send":
			payload, err := decodeContextNestedInitTermPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.SendEvent(ctx, payload); err != nil {
				return compat.Trace{}, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return compat.Trace{}, err
			}
		case "case":
			// The single-case scenario carries one case marker; replay skips it.
		default:
			return compat.Trace{}, fmt.Errorf("unsupported context-nested-initterm op %q", step.Op)
		}
	}
	return trace, nil
}

func decodeContextNestedInitTermPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextNestedInitTermBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextNestedInitTermS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextNestedInitTermS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-nested-initterm event type %q", step.EventType)
	}
}
