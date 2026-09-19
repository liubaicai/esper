package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type eplInsertIntoFromPatternS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type eplInsertIntoFromPatternS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

type eplInsertIntoFromPatternBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const eplInsertIntoFromPatternJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoFromPatternJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoFromPattern.java",
}

var eplInsertIntoFromPatternJavaRuntimeIDs = []string{
	"java-runtime-4dc2b394114fdaf84056",
	"java-runtime-98eba92039ca0e871ad6",
	"java-runtime-1dd189e49dda82f29078",
	"java-runtime-9092d240993543259d6e",
}

var eplInsertIntoFromPatternJavaExecutions = []string{
	"EPLInsertIntoPropsWildcard",
	"EPLInsertIntoProps",
	"EPLInsertIntoNoProps",
	"EPLInsertIntoFromPatternNamedWindow",
}

var eplInsertIntoFromPatternCases = []string{
	"pattern-wildcard-props",
	"pattern-explicit-props",
	"pattern-no-props",
	"pattern-named-window",
}

var eplInsertIntoFromPatternCaseRuntimeIDs = map[string]string{
	"pattern-wildcard-props": "java-runtime-4dc2b394114fdaf84056",
	"pattern-explicit-props": "java-runtime-98eba92039ca0e871ad6",
	"pattern-no-props":       "java-runtime-1dd189e49dda82f29078",
	"pattern-named-window":   "java-runtime-9092d240993543259d6e",
}

// runEplInsertIntoFromPatternScenario replays the four
// EPLInsertIntoFromPattern executions: pattern matches routed into derived
// streams (wildcard tag properties, bean-typed tag columns, default
// tag-named columns) and into a derived stream fed by a named-window
// pattern source.
func runEplInsertIntoFromPatternScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplInsertIntoFromPatternCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplInsertIntoFromPatternCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl insert-into from-pattern case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl insert-into from-pattern scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplInsertIntoFromPatternCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plans, err := buildEplInsertIntoFromPatternCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplInsertIntoFromPatternCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequences := make(map[string]uint64)
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return trace, err
		}
		for _, statement := range deployment.Statements() {
			name := statement.Name()
			if name != "s0" && name != "s1" {
				continue
			}
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequences["listener:"+stmt.Name()]++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  sequences["listener:"+stmt.Name()],
					Time:      engine.Now().UTC().Format(time.RFC3339Nano),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); err != nil {
				return trace, err
			}
		}
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeEplInsertIntoFromPatternPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported epl insert-into from-pattern step op %q", step.Op)
		}
	}
	return trace, nil
}

// buildEplInsertIntoFromPatternCase registers the case's schemas and
// targets and returns the deployment plans in pinned order (producer
// insert-into before consumer, matching the Java deployment order).
func buildEplInsertIntoFromPatternCase(env *esper.Environment, caseName string) ([]esper.Plan, error) {
	switch caseName {
	case "pattern-wildcard-props":
		s0Schema, s1Schema, err := registerEplInsertIntoFromPatternBeans(env)
		if err != nil {
			return nil, err
		}
		_ = s0Schema
		_ = s1Schema
		if _, err := esper.RegisterMap(env, "MyThirdStream", []esper.FieldSpec{
			esper.FieldDef("es0id", reflect.TypeOf(0)),
			esper.FieldDef("es1id", reflect.TypeOf(0)),
		}); err != nil {
			return nil, err
		}
		// @public insert into MyThirdStream(es0id, es1id) select es0.id,
		// es1.id from pattern[every (es0=SupportBean_S0 or
		// es1=SupportBean_S1)] — the column list maps to alias names and an
		// absent OR-branch tag projects null (TagField null-safe).
		s0Source := esper.From[eplInsertIntoFromPatternS0](env, "SupportBean_S0")
		s1Source := esper.From[eplInsertIntoFromPatternS1](env, "SupportBean_S1")
		insertPlan, err := env.Build(
			esper.PatternFrom(s0Source, "es0", esper.Literal(true)).
				Or(esper.PatternFrom(s1Source, "es1", esper.Literal(true))).
				Every().
				Select(
					esper.Alias("es0id", esper.TagField[int]("es0", "id")),
					esper.Alias("es1id", esper.TagField[int]("es1", "id")),
				).InsertInto("MyThirdStream"))
		if err != nil {
			return nil, err
		}
		s0Plan, err := env.Build(esper.FromAny(env, "MyThirdStream").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil
	case "pattern-explicit-props":
		s0Schema, s1Schema, err := registerEplInsertIntoFromPatternBeans(env)
		if err != nil {
			return nil, err
		}
		// @public insert into MySecondStream(s0, s1) select es0, es1: the
		// columns hold the tagged event objects, bean-typed.
		if _, err := esper.RegisterMap(env, "MySecondStream", []esper.FieldSpec{
			esper.FieldDef("s0", reflect.TypeOf(esper.Event{})),
			esper.FieldDef("s1", reflect.TypeOf(esper.Event{})),
		}, esper.WithNestedPropertySchema("s0", s0Schema),
			esper.WithNestedPropertySchema("s1", s1Schema)); err != nil {
			return nil, err
		}
		s0Source := esper.From[eplInsertIntoFromPatternS0](env, "SupportBean_S0")
		s1Source := esper.From[eplInsertIntoFromPatternS1](env, "SupportBean_S1")
		insertPlan, err := env.Build(
			esper.PatternFrom(s0Source, "es0", esper.Literal(true)).
				Or(esper.PatternFrom(s1Source, "es1", esper.Literal(true))).
				Every().
				Select(
					esper.Alias("s0", esper.PatternEvent("es0")),
					esper.Alias("s1", esper.PatternEvent("es1")),
				).InsertInto("MySecondStream"))
		if err != nil {
			return nil, err
		}
		// @name('s0') select s0.id as es0id, s1.id as es1id from
		// MySecondStream — NestedField on a null fragment yields null,
		// matching Java's null-safe property access.
		s0Plan, err := env.Build(esper.FromAny(env, "MySecondStream").Select(
			esper.Alias("es0id", esper.NestedField[int](esper.Field[any, esper.Event]("s0"), "id")),
			esper.Alias("es1id", esper.NestedField[int](esper.Field[any, esper.Event]("s1"), "id")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil
	case "pattern-no-props":
		s0Schema, s1Schema, err := registerEplInsertIntoFromPatternBeans(env)
		if err != nil {
			return nil, err
		}
		// @public insert into MyStream select es0, es1: no column list, so
		// the default column names are the tag names es0/es1.
		if _, err := esper.RegisterMap(env, "MyStream", []esper.FieldSpec{
			esper.FieldDef("es0", reflect.TypeOf(esper.Event{})),
			esper.FieldDef("es1", reflect.TypeOf(esper.Event{})),
		}, esper.WithNestedPropertySchema("es0", s0Schema),
			esper.WithNestedPropertySchema("es1", s1Schema)); err != nil {
			return nil, err
		}
		s0Source := esper.From[eplInsertIntoFromPatternS0](env, "SupportBean_S0")
		s1Source := esper.From[eplInsertIntoFromPatternS1](env, "SupportBean_S1")
		insertPlan, err := env.Build(
			esper.PatternFrom(s0Source, "es0", esper.Literal(true)).
				Or(esper.PatternFrom(s1Source, "es1", esper.Literal(true))).
				Every().
				Select(
					esper.Alias("es0", esper.PatternEvent("es0")),
					esper.Alias("es1", esper.PatternEvent("es1")),
				).InsertInto("MyStream"))
		if err != nil {
			return nil, err
		}
		// @name('s0') select es0.id as es0id, es1.id as es1id from
		// MyStream#length(10).
		s0Plan, err := env.Build(esper.FromAny(env, "MyStream").
			Window(esper.LengthWindow(10)).
			Select(
				esper.Alias("es0id", esper.NestedField[int](esper.Field[any, esper.Event]("es0"), "id")),
				esper.Alias("es1id", esper.NestedField[int](esper.Field[any, esper.Event]("es1"), "id")),
			).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil
	case "pattern-named-window":
		beanSchema, err := esper.RegisterStruct[eplInsertIntoFromPatternBean](env, "SupportBean")
		if err != nil {
			return nil, err
		}
		// @public create window PositionW.win:time(1
		// hour).std:unique(intPrimitive) as select * from SupportBean
		if _, err := esper.CreateNamedWindow(env, "PositionW", beanSchema,
			esper.NamedWindowRetention(esper.IntersectWindows(
				esper.TimeWindow(time.Hour),
				esper.Unique(esper.Field[eplInsertIntoFromPatternBean, int]("intPrimitive")),
			))); err != nil {
			return nil, err
		}
		// insert into PositionW select * from SupportBean
		feedPlan, err := env.Build(esper.From[eplInsertIntoFromPatternBean](env, "SupportBean").
			InsertInto("PositionW"))
		if err != nil {
			return nil, err
		}
		// @name('s1') insert into Foo select * from pattern[every a =
		// PositionW -> every b = PositionW]: the listener attaches to the
		// insert-into statement itself and sees the routed rows. select *
		// expands to per-tag event projections (zero-selection tagged
		// patterns are Build-rejected — approved difference); Foo is
		// pre-registered with Event columns carrying the bean nested
		// schema (Java auto-creates the type — approved difference).
		if _, err := esper.RegisterMap(env, "Foo", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf(esper.Event{})),
			esper.FieldDef("b", reflect.TypeOf(esper.Event{})),
		}, esper.WithNestedPropertySchema("a", beanSchema),
			esper.WithNestedPropertySchema("b", beanSchema)); err != nil {
			return nil, err
		}
		windowSource := esper.FromNamedWindow(env, "PositionW")
		s1Plan, err := env.Build(
			esper.PatternFromRecord(windowSource, "a", esper.Literal(true)).
				Every().
				Then(esper.PatternFromRecord(windowSource, "b", esper.Literal(true)).Every()).
				Select(
					esper.Alias("a", esper.PatternEvent("a")),
					esper.Alias("b", esper.PatternEvent("b")),
				).InsertInto("Foo", esper.StatementName("s1")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{feedPlan, s1Plan}, nil
	default:
		return nil, fmt.Errorf("unsupported epl insert-into from-pattern case %q", caseName)
	}
}

func registerEplInsertIntoFromPatternBeans(env *esper.Environment) (esper.Schema, esper.Schema, error) {
	s0Schema, err := esper.RegisterStruct[eplInsertIntoFromPatternS0](env, "SupportBean_S0")
	if err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	s1Schema, err := esper.RegisterStruct[eplInsertIntoFromPatternS1](env, "SupportBean_S1")
	if err != nil {
		return esper.Schema{}, esper.Schema{}, err
	}
	return s0Schema, s1Schema, nil
}

func decodeEplInsertIntoFromPatternPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value eplInsertIntoFromPatternBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value eplInsertIntoFromPatternS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value eplInsertIntoFromPatternS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported epl insert-into from-pattern event type %q", step.EventType)
	}
}
