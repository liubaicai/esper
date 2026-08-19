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

type transposePatternThisBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func (event transposePatternThisBean) GetThis() transposePatternThisBean { return event }

type transposePatternBeanA struct {
	ID string `esper:"id"`
}

type transposePatternBeanB struct {
	ID string `esper:"id"`
}

const transposePatternJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var transposePatternJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoTransposePattern.java",
}

var (
	transposePatternJavaRuntimeIDs = []string{
		"java-runtime-b449bbd35d5ff2ee128d",
		"java-runtime-42af77eea911eeaed74f",
		"java-runtime-491c5cf7a48b7d584c16",
	}
	transposePatternJavaExecutions = []string{
		"EPLInsertIntoThisAsColumn",
		"EPLInsertIntoTransposePOJOEventPattern",
		"EPLInsertIntoTransposeMapEventPattern",
	}
)

func runTransposePatternScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range []string{"this-as-column", "pattern-pojo", "pattern-map"} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		var caseTrace compat.Trace
		switch caseName {
		case "this-as-column":
			caseTrace, err = runTransposePatternThisAsColumn(ctx, caseScenario)
		case "pattern-pojo":
			caseTrace, err = runTransposePatternPOJO(ctx, caseScenario)
		case "pattern-map":
			caseTrace, err = runTransposePatternMap(ctx, caseScenario)
		}
		if err != nil {
			return compat.Trace{}, fmt.Errorf("transpose pattern case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("transpose pattern scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runTransposePatternThisAsColumn(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	env := esper.NewEnvironment()
	sourceSchema, err := esper.RegisterStruct[transposePatternThisBean](env, "SupportBeanWithThis", esper.WithAccessorStyle(esper.AccessorJavaBean))
	if err != nil {
		return compat.Trace{}, err
	}
	oneWindowSchema, err := esper.RegisterMap(env, "OneWindow", []esper.FieldSpec{
		esper.FieldDef("alertId", reflect.TypeOf("")),
		esper.FieldDef("this", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("this", sourceSchema))
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "OneWindow", oneWindowSchema, esper.NamedWindowRetention(esper.TimeWindow(24*time.Hour))); err != nil {
		return compat.Trace{}, err
	}
	twoWindowSchema, err := esper.RegisterMap(env, "TwoWindow", []esper.FieldSpec{
		esper.FieldDef("alertId", reflect.TypeOf("")),
		esper.FieldDef("theString", reflect.TypeOf("")),
		esper.FieldDef("intPrimitive", reflect.TypeOf(0)),
		esper.FieldDef("this", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("this", sourceSchema))
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "TwoWindow", twoWindowSchema, esper.NamedWindowRetention(esper.TimeWindow(24*time.Hour))); err != nil {
		return compat.Trace{}, err
	}

	source := esper.From[transposePatternThisBean](env, "SupportBeanWithThis")
	pattern := func(value string) esper.PatternStream {
		return esper.PatternFrom(source, "quote", esper.Equal[string](
			esper.Field[transposePatternThisBean, string]("theString"), esper.Literal(value),
		)).Every()
	}
	producerA, err := env.Build(pattern("A").Select(
		esper.Alias("alertId", esper.Literal("1")),
		esper.Alias("this", esper.PatternEvent("quote")),
	).InsertInto("OneWindow", esper.StatementName("producer-A")))
	if err != nil {
		return compat.Trace{}, err
	}
	producerB, err := env.Build(pattern("B").Select(
		esper.Alias("alertId", esper.Literal("2")),
		esper.Alias("this", esper.PatternEvent("quote")),
	).InsertInto("OneWindow", esper.StatementName("producer-B")))
	if err != nil {
		return compat.Trace{}, err
	}
	producerC, err := env.Build(pattern("C").Select(
		esper.Alias("alertId", esper.Literal("3")),
		esper.Alias("theString", esper.TagField[string]("quote", "theString")),
		esper.Alias("intPrimitive", esper.TagField[int]("quote", "intPrimitive")),
		esper.Alias("this", esper.PatternEvent("quote")),
	).InsertInto("TwoWindow", esper.StatementName("producer-C")))
	if err != nil {
		return compat.Trace{}, err
	}
	windowView, err := env.Build(esper.FromNamedWindow(env, "OneWindow").Query(esper.StatementName("window-view")))
	if err != nil {
		return compat.Trace{}, err
	}
	windowTwoView, err := env.Build(esper.FromNamedWindow(env, "TwoWindow").Query(esper.StatementName("window-2-view")))
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(transposePatternJavaRuntimeIDs[0]))
	defer func() { _ = engine.Close(context.Background()) }()
	for _, plan := range []esper.Plan{producerA, producerB, producerC, windowView, windowTwoView} {
		if _, err := engine.Deploy(ctx, plan); err != nil {
			return compat.Trace{}, err
		}
	}
	statements := map[string]*esper.Statement{}
	for _, name := range []string{"window-view", "window-2-view"} {
		statement, err := findTransposePatternStatementResult(engine, name)
		if err != nil {
			return compat.Trace{}, err
		}
		statements[name] = statement
	}
	return replayTransposePatternSnapshots(ctx, engine, scenario, statements, decodeTransposePatternPayload)
}

func runTransposePatternPOJO(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[transposePatternBeanA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[transposePatternBeanB](env, "SupportBean_B"); err != nil {
		return compat.Trace{}, err
	}
	aSchema, _ := env.Schema("SupportBean_A")
	bSchema, _ := env.Schema("SupportBean_B")
	if _, err := esper.RegisterMap(env, "MyStreamABBean", []esper.FieldSpec{
		esper.FieldDef("a", reflect.TypeOf(esper.Event{})),
		esper.FieldDef("b", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("a", aSchema), esper.WithNestedPropertySchema("b", bSchema)); err != nil {
		return compat.Trace{}, err
	}
	pattern := esper.PatternFrom(esper.From[transposePatternBeanA](env, "SupportBean_A"), "a", esper.Literal(true)).Then(
		esper.PatternFrom(esper.From[transposePatternBeanB](env, "SupportBean_B"), "b", esper.Literal(true)),
	)
	producer, err := env.Build(pattern.Select(
		esper.Alias("a", esper.PatternEvent("a")),
		esper.Alias("b", esper.PatternEvent("b")),
	).InsertInto("MyStreamABBean", esper.StatementName("producer")))
	if err != nil {
		return compat.Trace{}, err
	}
	consumer, err := env.Build(esper.FromAny(env, "MyStreamABBean").Select(
		esper.Alias("a.id", esper.NestedField[string](esper.Field[esper.Event, esper.Event]("a"), "id")),
		esper.Alias("b.id", esper.NestedField[string](esper.Field[esper.Event, esper.Event]("b"), "id")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(transposePatternJavaRuntimeIDs[1]))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, producer); err != nil {
		return compat.Trace{}, err
	}
	consumerDeployment, err := engine.Deploy(ctx, consumer)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := consumerDeployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one POJO consumer statement, got %d", len(statements))
	}
	return replayTransposePatternListeners(ctx, engine, scenario, statements, decodeTransposePatternPayload, false)
}

func runTransposePatternMap(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	env := esper.NewEnvironment()
	aSchema, err := esper.RegisterMap(env, "AEventMap", []esper.FieldSpec{esper.FieldDef("id", reflect.TypeOf(""))})
	if err != nil {
		return compat.Trace{}, err
	}
	bSchema, err := esper.RegisterMap(env, "BEventMap", []esper.FieldSpec{esper.FieldDef("id", reflect.TypeOf(""))})
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "MyStreamABMap", []esper.FieldSpec{
		esper.FieldDef("a", reflect.TypeOf(esper.Event{})),
		esper.FieldDef("b", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("a", aSchema), esper.WithNestedPropertySchema("b", bSchema)); err != nil {
		return compat.Trace{}, err
	}
	pattern := esper.PatternFromRecord(esper.FromAny(env, "AEventMap"), "a", esper.Literal(true)).Then(
		esper.PatternFromRecord(esper.FromAny(env, "BEventMap"), "b", esper.Literal(true)),
	)
	producer, err := env.Build(pattern.Select(
		esper.Alias("a", esper.PatternEvent("a")),
		esper.Alias("b", esper.PatternEvent("b")),
	).InsertInto("MyStreamABMap", esper.StatementName("i1")))
	if err != nil {
		return compat.Trace{}, err
	}
	consumer, err := env.Build(esper.FromAny(env, "MyStreamABMap").Select(
		esper.Alias("a.id", esper.NestedField[string](esper.Field[esper.Event, esper.Event]("a"), "id")),
		esper.Alias("b.id", esper.NestedField[string](esper.Field[esper.Event, esper.Event]("b"), "id")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(transposePatternJavaRuntimeIDs[2]))
	defer func() { _ = engine.Close(context.Background()) }()
	producerDeployment, err := engine.Deploy(ctx, producer)
	if err != nil {
		return compat.Trace{}, err
	}
	consumerDeployment, err := engine.Deploy(ctx, consumer)
	if err != nil {
		return compat.Trace{}, err
	}
	producerStatements := producerDeployment.Statements()
	consumerStatements := consumerDeployment.Statements()
	if len(producerStatements) != 1 || len(consumerStatements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one Map producer and consumer statement, got %d/%d", len(producerStatements), len(consumerStatements))
	}
	return replayTransposePatternListeners(ctx, engine, scenario, []*esper.Statement{producerStatements[0], consumerStatements[0]}, decodeTransposePatternPayload, true)
}

func decodeTransposePatternPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBeanWithThis":
		var value transposePatternThisBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanWithThis: %w", err)
		}
		return value, nil
	case "SupportBean_A":
		var value transposePatternBeanA
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return value, nil
	case "SupportBean_B":
		var value transposePatternBeanB
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return value, nil
	case "AEventMap", "BEventMap":
		var value map[string]any
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode %s: %w", step.EventType, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported transpose pattern event type %q", step.EventType)
	}
}

func findTransposePatternStatementResult(engine *esper.Engine, name string) (*esper.Statement, error) {
	if engine == nil {
		return nil, fmt.Errorf("transpose pattern engine is nil")
	}
	for _, deployment := range engine.Deployments() {
		for _, statement := range deployment.Statements() {
			if statement.Name() == name {
				return statement, nil
			}
		}
	}
	return nil, fmt.Errorf("transpose pattern statement %q is missing", name)
}

func replayTransposePatternSnapshots(ctx context.Context, engine *esper.Engine, scenario compat.Scenario, statements map[string]*esper.Statement, decode compat.DecodePayload) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	caseName := ""
	for _, step := range scenario.Steps {
		if err := contextErrForTransposePattern(ctx); err != nil {
			return trace, err
		}
		if step.Op == "case" {
			caseName = step.Case
			continue
		}
		if step.Case != "" && step.Case != caseName {
			continue
		}
		switch step.Op {
		case "send":
			payload, err := decode(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
			if caseName != "this-as-column" {
				continue
			}
			statementName := "window-view"
			if value, ok := payload.(transposePatternThisBean); ok && value.TheString == "C" {
				statementName = "window-2-view"
			}
			statement := statements[statementName]
			if statement == nil {
				return trace, fmt.Errorf("unknown transpose pattern implicit snapshot statement %q", statementName)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			record := compat.TraceRecord{Case: caseName, Operation: "snapshot", Statement: statement.Name(), Time: currentTimeString(engine)}
			record.New = compat.NormalizeResults(result.Results())
			trace.Records = append(trace.Records, record)
		case "snapshot":
			statement := statements[step.Statement]
			if statement == nil {
				return trace, fmt.Errorf("unknown transpose pattern snapshot statement %q", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			record := compat.TraceRecord{Case: caseName, Operation: "snapshot", Statement: statement.Name(), Time: currentTimeString(engine)}
			record.New = compat.NormalizeResults(result.Results())
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported transpose pattern snapshot step %q", step.Op)
		}
	}
	return rewriteTransposePatternTrace(trace), nil
}

func replayTransposePatternListeners(ctx context.Context, engine *esper.Engine, scenario compat.Scenario, statements []*esper.Statement, decode compat.DecodePayload, rewrite bool) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	caseName := ""
	var sequence uint64
	for _, statement := range statements {
		if statement == nil {
			return compat.Trace{}, fmt.Errorf("transpose pattern listener statement is nil")
		}
		current := statement
		if _, err := current.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			sequence++
			record := compat.TraceRecord{Case: caseName, Operation: "listener", Statement: current.Name(), Sequence: sequence, Time: currentTimeString(engine)}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return compat.Trace{}, err
		}
	}
	for _, step := range scenario.Steps {
		if err := contextErrForTransposePattern(ctx); err != nil {
			return trace, err
		}
		if step.Op == "case" {
			caseName = step.Case
			continue
		}
		if step.Case != "" && step.Case != caseName {
			continue
		}
		if step.Op != "send" {
			return trace, fmt.Errorf("unsupported transpose pattern listener step %q", step.Op)
		}
		payload, err := decode(step)
		if err != nil {
			return trace, err
		}
		if err := engine.Send(ctx, step.EventType, payload); err != nil {
			return trace, err
		}
	}
	if rewrite {
		return rewriteTransposePatternTrace(trace), nil
	}
	return trace, nil
}

func currentTimeString(engine *esper.Engine) string {
	return engine.Now().UTC().Format(time.RFC3339Nano)
}

func contextErrForTransposePattern(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func rewriteTransposePatternTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for resultIndex := range record.New {
			rewriteTransposePatternFields(record.Case, record.Statement, &record.New[resultIndex].Fields)
		}
		for resultIndex := range record.Old {
			rewriteTransposePatternFields(record.Case, record.Statement, &record.Old[resultIndex].Fields)
		}
	}
	return trace
}

func rewriteTransposePatternFields(caseName, statement string, fields *map[string]any) {
	if fields == nil {
		return
	}
	for name, value := range *fields {
		if caseName == "this-as-column" && name == "this" {
			if row, ok := value.(map[string]any); ok {
				if nested, ok := row["fields"].(map[string]any); ok && row["kind"] == "row" {
					copy := make(map[string]any, len(nested)+1)
					copy["__type"] = "SupportBeanWithThis"
					for key, nestedValue := range nested {
						if key != "this" {
							copy[key] = nestedValue
						}
					}
					(*fields)[name] = copy
				}
			}
		}
		if caseName == "pattern-map" && statement == "i1" {
			if row, ok := value.(map[string]any); ok {
				if nested, ok := row["fields"].(map[string]any); ok && row["kind"] == "row" {
					(*fields)[name] = nested
				}
			}
		}
	}
}
