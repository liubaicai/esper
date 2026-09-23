package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// contextKeySegmentedInfraPrioritizedBean mirrors the real SupportBean
// surface: the create-index case's `select * from MyInfra` fire-and-forget
// projects the full 20-property bean, so the Go trace renders the same row.
// Boxed, decimal, and enum columns are pointers so a plain send decodes to
// Java's nulls, and charPrimitive mirrors the Java char default via decode.
type contextKeySegmentedInfraPrioritizedBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
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

type contextKeySegmentedInfraPrioritizedS0 struct {
	ID  int     `esper:"id"`
	P00 string  `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

type contextKeySegmentedInfraPrioritizedS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

const contextKeySegmentedInfraPrioritizedID = "context-key-segmented-infra-prioritized"

const contextKeySegmentedInfraPrioritizedJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedInfraPrioritizedJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmentedInfra.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmentedPrioritized.java",
}

var contextKeySegmentedInfraPrioritizedJavaRuntimeIDs = []string{
	"java-runtime-f297e13be96337235ae0", // ContextKeySegmentedInfraAggregatedSubquery
	"java-runtime-f49875a427fb00323404", // ContextKeySegmentedInfraCreateIndex
	"java-runtime-3b57cf3ad453555fb0b5", // ContextKeySegmentedPrioritized
}

var contextKeySegmentedInfraPrioritizedJavaExecutions = []string{
	"ContextKeySegmentedInfraAggregatedSubquery",
	"ContextKeySegmentedInfraCreateIndex",
	"ContextKeySegmentedPrioritized",
}

// runContextKeySegmentedInfraPrioritizedScenario replays three keyed-context
// executions: the partition-scoped max subquery over a context-bound named
// window and table (AggregatedSubquery, two variants), the overlapping
// initiated/terminated context with a live create-index and a by-id FAF
// probe (CreateIndex, two variants), and the @Drop @Priority(1) preemption
// of a lower-priority consumer (ContextKeySegmentedPrioritized).
func runContextKeySegmentedInfraPrioritizedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedInfraPrioritizedCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("context-key-segmented-infra-prioritized case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedInfraPrioritizedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextKeySegmentedInfraPrioritizedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedInfraPrioritizedS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[contextKeySegmentedInfraPrioritizedS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	beanSource := esper.From[contextKeySegmentedInfraPrioritizedBean](env, "SupportBean")
	s0Source := esper.From[contextKeySegmentedInfraPrioritizedS0](env, "SupportBean_S0")
	theString := esper.Field[contextKeySegmentedInfraPrioritizedBean, string]("theString")
	intPrimitive := esper.Field[contextKeySegmentedInfraPrioritizedBean, int]("intPrimitive")

	switch caseName {
	case "infra-aggregated-subquery-nw", "infra-aggregated-subquery-table":
		// create context SegmentedByString partition by theString from
		// SupportBean, p00 from SupportBean_S0
		if _, err := esper.CreateKeyContextByStreams(env, "SegmentedByString",
			esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}},
			esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{esper.Field[contextKeySegmentedInfraPrioritizedS0, string]("p00")}},
		); err != nil {
			return compat.Trace{}, err
		}
		// @public context SegmentedByString create window MyInfra#keepall as
		// SupportBean / create table MyInfra (theString string primary key,
		// intPrimitive int)
		var infra esper.RecordStream
		if caseName == "infra-aggregated-subquery-nw" {
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return compat.Trace{}, fmt.Errorf("%s: SupportBean schema is not registered", contextKeySegmentedInfraPrioritizedID)
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfra", schema,
				esper.NamedWindowContext("SegmentedByString"), esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return compat.Trace{}, err
			}
			infra = esper.FromNamedWindow(env, "MyInfra")
		} else {
			if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
				{Name: "theString", Type: reflect.TypeOf(""), PrimaryKey: true},
				{Name: "intPrimitive", Type: reflect.TypeOf(int(0))},
			}, esper.TableContext("SegmentedByString")); err != nil {
				return compat.Trace{}, err
			}
			infra = esper.FromTable(env, "MyInfra")
		}
		var insertPlan esper.Plan
		if caseName == "infra-aggregated-subquery-nw" {
			insertPlan, err = env.Build(esper.Select(beanSource,
				esper.Alias("theString", theString),
				esper.Alias("intPrimitive", intPrimitive),
			).InsertInto("MyInfra", esper.StatementName("insert"), esper.WithContext("SegmentedByString")))
		} else {
			insertPlan, err = env.Build(esper.OnRecord(beanSource.AsRecord()).InsertIntoTable("MyInfra",
				esper.SetColumn("theString", esper.Field[any, string]("theString")),
				esper.SetColumn("intPrimitive", esper.Field[any, int]("intPrimitive")),
			).Query(esper.StatementName("insert"), esper.WithContext("SegmentedByString")))
		}
		if err != nil {
			return compat.Trace{}, err
		}
		s0Plan, err := env.Build(esper.Select(s0Source,
			esper.Alias("id", esper.Field[contextKeySegmentedInfraPrioritizedS0, int]("id")),
			esper.Alias("p00", esper.Field[contextKeySegmentedInfraPrioritizedS0, string]("p00")),
			esper.Alias("p01", esper.Field[contextKeySegmentedInfraPrioritizedS0, *string]("p01")),
			esper.Alias("p02", esper.Field[contextKeySegmentedInfraPrioritizedS0, *string]("p02")),
			esper.Alias("p03", esper.Field[contextKeySegmentedInfraPrioritizedS0, *string]("p03")),
			esper.Alias("mymax", esper.SubqueryValue[int](infra, esper.Max[int](esper.Field[any, int]("intPrimitive")))),
		).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString")))
		if err != nil {
			return compat.Trace{}, err
		}
		engine := esper.NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		if _, err := engine.Deploy(ctx, insertPlan); err != nil {
			return compat.Trace{}, err
		}
		deployment, err := engine.Deploy(ctx, s0Plan)
		if err != nil {
			return compat.Trace{}, err
		}
		if len(deployment.Statements()) != 1 {
			return compat.Trace{}, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
		}
		statement := deployment.Statements()[0]
		return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, caseScenario, decodeContextKeySegmentedInfraPrioritizedPayload, func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown context-key-segmented-infra-prioritized statement %q", name)
			}
			return statement, nil
		}, contextKeySegmentedInfraPrioritizedNoopHandlers())
	case "infra-create-index-nw", "infra-create-index-table":
		return runContextKeySegmentedInfraCreateIndexCase(ctx, env, beanSource, caseScenario, caseName)
	case "keyed-prioritized":
		// @public create context SegmentedByMessage partition by theString
		// from SupportBean
		if _, err := esper.CreateKeyContext(env, "SegmentedByMessage", theString); err != nil {
			return compat.Trace{}, err
		}
		// @name('s0') @Drop @Priority(1) context SegmentedByMessage select
		// 'test1' from SupportBean — the unaliased constant auto-names to
		// the quoted literal "test1".
		s0Plan, err := env.Build(esper.Select(beanSource,
			esper.Alias("\"test1\"", esper.Literal("test1")),
		).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByMessage"),
			esper.StatementPriority(1), esper.StatementDrop()))
		if err != nil {
			return compat.Trace{}, err
		}
		// @name('s1') @Priority(0) context SegmentedByMessage select 'test2'
		// from SupportBean
		s1Plan, err := env.Build(esper.Select(beanSource,
			esper.Alias("\"test2\"", esper.Literal("test2")),
		).Query(esper.StatementName("s1"), esper.WithContext("SegmentedByMessage"),
			esper.StatementPriority(0)))
		if err != nil {
			return compat.Trace{}, err
		}
		engine := esper.NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()
		s0Deployment, err := engine.Deploy(ctx, s0Plan)
		if err != nil {
			return compat.Trace{}, err
		}
		s1Deployment, err := engine.Deploy(ctx, s1Plan)
		if err != nil {
			return compat.Trace{}, err
		}
		statements := map[string]*esper.Statement{}
		for _, st := range s0Deployment.Statements() {
			statements[st.Name()] = st
		}
		for _, st := range s1Deployment.Statements() {
			statements[st.Name()] = st
		}
		return compat.ReplayWithStatementsAndHandlers(ctx, engine, statements["s0"], caseScenario, decodeContextKeySegmentedInfraPrioritizedPayload, func(name string) (*esper.Statement, error) {
			statement, ok := statements[name]
			if !ok {
				return nil, fmt.Errorf("unknown context-key-segmented-infra-prioritized statement %q", name)
			}
			return statement, nil
		}, contextKeySegmentedInfraPrioritizedNoopHandlers(), statements["s1"])
	default:
		return compat.Trace{}, fmt.Errorf("unsupported context-key-segmented-infra-prioritized case %q", caseName)
	}
}

// contextKeySegmentedInfraPrioritizedNoopHandlers swallows the pinned
// deploy/undeploy-all steps: the runner pre-deploys every statement before
// replay, so the oracle's deploy steps are byte-exact EPL evidence only.
func contextKeySegmentedInfraPrioritizedNoopHandlers() map[string]compat.StepHandler {
	noop := func(compat.Step, func(*esper.Statement) error) ([]compat.TraceRecord, error) { return nil, nil }
	return map[string]compat.StepHandler{"deploy": noop, "undeploy-all": noop}
}

// runContextKeySegmentedInfraCreateIndexCase replays the overlapping
// initiated/terminated SegmentedByCustomer context: two S0 sends open
// partitions A and B, one SupportBean inserts into the context-bound infra
// in both partitions, a by-id FAF probe reads partition 1, and the
// correlated S1 terminates partition A.
func runContextKeySegmentedInfraCreateIndexCase(ctx context.Context, env *esper.Environment, beanSource esper.Stream[contextKeySegmentedInfraPrioritizedBean], caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	isS0 := esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S0"))
	// terminated by SupportBean_S1(p00 = p10): the initiating event's p00
	// equals the terminating event's p10.
	end := esper.And(
		esper.Equal[string](esper.TypeName(esper.EventValue[esper.Event]()), esper.Literal("SupportBean_S1")),
		esper.Equal[string](
			esper.Property[string](esper.ContextInitiatingEvent(), "p00"),
			esper.Field[contextKeySegmentedInfraPrioritizedS1, string]("p10")))
	if _, err := esper.CreateOverlappingInitiatedTerminatedContext(env, "SegmentedByCustomer", esper.Literal("global"), isS0, end); err != nil {
		return trace, err
	}
	var infra esper.RecordStream
	if caseName == "infra-create-index-nw" {
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return trace, fmt.Errorf("%s: SupportBean schema is not registered", contextKeySegmentedInfraPrioritizedID)
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfra", schema,
			esper.NamedWindowContext("SegmentedByCustomer"), esper.NamedWindowRetention(esper.KeepAll()),
			esper.NamedWindowIndex("MyIndex", "intPrimitive")); err != nil {
			return trace, err
		}
		infra = esper.FromNamedWindow(env, "MyInfra")
	} else {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			{Name: "theString", Type: reflect.TypeOf(""), PrimaryKey: true},
			{Name: "intPrimitive", Type: reflect.TypeOf(int(0))},
		}, esper.TableContext("SegmentedByCustomer")); err != nil {
			return trace, err
		}
		infra = esper.FromTable(env, "MyInfra")
	}
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	if caseName == "infra-create-index-table" {
		// create index MyIndex on MyInfra(intPrimitive) — a live index on
		// the deployed table.
		table, ok := engine.Table("MyInfra")
		if !ok {
			return trace, fmt.Errorf("%s: MyInfra table is not deployed", contextKeySegmentedInfraPrioritizedID)
		}
		if err := table.CreateIndex("MyIndex", []string{"intPrimitive"}, esper.IndexHash, false); err != nil {
			return trace, err
		}
	}
	// insert into MyInfra select theString, intPrimitive from SupportBean —
	// the named-window variant carries no context clause (the window's
	// context routes); the table variant carries context SegmentedByCustomer.
	// Esper fills unprojected window columns with the event bean's defaults,
	// so the Go chain routes the whole decoded event: the bean already
	// carries Java's defaults (charPrimitive NUL, boxed nulls) and the
	// two-column projection would re-zero them.
	var insertPlan esper.Plan
	var err error
	if caseName == "infra-create-index-nw" {
		insertPlan, err = env.Build(beanSource.InsertInto("MyInfra", esper.StatementName("insert-into-window")))
	} else {
		insertPlan, err = env.Build(esper.OnRecord(beanSource.AsRecord()).InsertIntoTable("MyInfra",
			esper.SetColumn("theString", esper.Field[any, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[any, int]("intPrimitive")),
		).Query(esper.StatementName("insert-into-table"), esper.WithContext("SegmentedByCustomer")))
	}
	if err != nil {
		return trace, err
	}
	if _, err := engine.Deploy(ctx, insertPlan); err != nil {
		return trace, err
	}
	for _, step := range caseScenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case", "deploy":
			// The context/infra/insert deploy up front; the pinned deploy
			// steps carry the byte-exact EPL the oracle compiles.
		case "send":
			event, err := decodeContextKeySegmentedInfraPrioritizedPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return trace, err
			}
		case "faf":
			// select * from MyInfra where intPrimitive = 1 with
			// ContextPartitionSelectorById{1}
			plan, err := env.Build(infra.Filter(
				esper.Equal[int](esper.Field[any, int]("intPrimitive"), esper.Literal(1)),
			).Query(esper.WithContext("SegmentedByCustomer")))
			if err != nil {
				return trace, err
			}
			result, err := engine.ExecuteFireAndForgetWithSelector(ctx, plan, esper.SelectContextPartitionIDs(step.IDs...))
			if err != nil {
				return trace, err
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "faf",
				Statement: step.Statement,
				Time:      compat.FormatTraceTime(engine.Now()),
				New:       compat.NormalizeResults(result.Results()),
			})
		case "undeploy-all":
			// deferred engine close covers it
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", contextKeySegmentedInfraPrioritizedID, step.Op)
		}
	}
	return trace, nil
}
func decodeContextKeySegmentedInfraPrioritizedPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedInfraPrioritizedBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		// charPrimitive mirrors Java's char default: an absent payload
		// field decodes to the NUL character the Java bean carries.
		if value.CharPrimitive == "" {
			value.CharPrimitive = "\x00"
		}
		return value, nil
	case "SupportBean_S0":
		var value contextKeySegmentedInfraPrioritizedS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value contextKeySegmentedInfraPrioritizedS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported context-key-segmented-infra-prioritized event type %q", step.EventType)
	}
}

// loadContextKeySegmentedInfraPrioritizedScenario decodes the scenario with
// the strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadContextKeySegmentedInfraPrioritizedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read context-key-segmented-infra-prioritized scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-infra-prioritized scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-infra-prioritized scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-infra-prioritized cases: %w", err)
	}
	wantCases := []struct {
		name      string
		ordinal   int
		runtimeID string
		execution string
	}{
		{"infra-aggregated-subquery-nw", 0, "java-runtime-f297e13be96337235ae0", "ContextKeySegmentedInfraAggregatedSubquery"},
		{"infra-aggregated-subquery-table", 0, "java-runtime-f297e13be96337235ae0", "ContextKeySegmentedInfraAggregatedSubquery"},
		{"infra-create-index-nw", 2, "java-runtime-f49875a427fb00323404", "ContextKeySegmentedInfraCreateIndex"},
		{"infra-create-index-table", 2, "java-runtime-f49875a427fb00323404", "ContextKeySegmentedInfraCreateIndex"},
		{"keyed-prioritized", 0, "java-runtime-3b57cf3ad453555fb0b5", "ContextKeySegmentedPrioritized"},
	}
	if len(cases) != len(wantCases) {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized scenario must contain exactly %d cases", len(wantCases))
	}
	for index, entry := range cases {
		want := wantCases[index]
		if entry.Case != want.name || entry.Ordinal != want.ordinal ||
			entry.RuntimeID != want.runtimeID || entry.ExecutionName != want.execution {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-infra-prioritized steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"faf":          {"op", "case", "statement", "epl", "selector", "ids"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 38 {
		return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized scenario must contain exactly 38 steps, found %d", len(rawSteps))
	}
	knownCases := map[string]bool{}
	for _, want := range wantCases {
		knownCases[want.name] = true
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d has unsupported op %q", index, op)
		}
		for field := range object {
			found := false
			for _, name := range allowed {
				if field == name {
					found = true
					break
				}
			}
			if !found {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d case: %w", index, err)
		}
		if !knownCases[stepCase] {
			return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d has unknown case %q", index, stepCase)
		}
		if op == "send" {
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d eventType: %w", index, err)
			}
			payloadFields := map[string][]string{
				"SupportBean":    {"theString", "intPrimitive"},
				"SupportBean_S0": {"id", "p00"},
				"SupportBean_S1": {"id", "p10"},
			}
			allowedPayload, ok := payloadFields[eventType]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d payload: %w", index, err)
			}
			for field := range payload {
				found := false
				for _, name := range allowedPayload {
					if field == name {
						found = true
						break
					}
				}
				if !found {
					return compat.Scenario{}, fmt.Errorf("context-key-segmented-infra-prioritized step %d payload has unexpected field %q", index, field)
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode context-key-segmented-infra-prioritized scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
