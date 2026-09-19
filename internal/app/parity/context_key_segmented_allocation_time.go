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

// context_key_segmented_allocation_time.go replays three ContextKeySegmented
// executions against the pinned Java oracle:
//
//   - pattern-fire-when-allocated (ord 25
//     ContextKeySegmentedWPatternFireWhenAllocated): a keyed context
//     partitioned by theString where timer:interval(0) fires synchronously
//     at partition allocation. s0 delivers {key1:allocating-key} and the
//     on-pattern trigger sets the per-partition variable lastString =
//     context.key1 in the same allocation; later events for the same key
//     produce nothing and a new key fires again. read-variable steps replay
//     the execution's getVariableValue(pair, SupportSelectorPartitioned(key))
//     assertion through SelectContextPartitionSegments with the partition
//     key carried by the step's filterValue.
//   - regex-filter (ord 28 ContextKeySegmentedRegExFilter): a map-typed
//     MyEventWPartition partitioned by partitionId terminated after 15
//     minutes; the allocating event is evaluated against the statement's
//     like "%hello%" filter and outputs. The termination never fires
//     because the scenario carries no advance-time step.
//   - subtype (ord 6 ContextKeySegmentedSubtype): the bean hierarchy
//     ISupportBaseAB{baseAB} <- ISupportA{a} <- ISupportAImpl is modeled
//     with map schemas and WithSchemaParent; ISupportAImpl events allocate
//     partitions through the ISupportBaseAB-declared baseAB key and count
//     per partition under the ISupportA-typed statement.
//
// Java env.milestone calls are harness no-ops and carry no steps; the send
// order preserves the assertion order exactly.
//
// Approved differences (observably identical to the Java EPL):
//   - `create context` maps to the env-level CreateKeyContextByStreams /
//     CreateKeyContextByStreamsTerminatedAfter registration; the
//     single-stream form records streamKeys so subtype events resolve the
//     supertype-declared key.
//   - `create variable` maps to env.RegisterContextVariable and
//     `on pattern[...] set` maps to the OnPattern trigger source.
//   - The Java bean hierarchy maps to map schemas linked by
//     WithSchemaParent, consistent with the bean-representation conventions
//     of this suite.

const contextKeySegmentedAllocationTimeID = "context-key-segmented-allocation-time"
const contextKeySegmentedAllocationTimeJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var contextKeySegmentedAllocationTimeJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextKeySegmented.java",
}

var contextKeySegmentedAllocationTimeJavaRuntimeIDs = []string{
	"java-runtime-57199db349abfe7ad70e", // ContextKeySegmentedWPatternFireWhenAllocated
	"java-runtime-9356c517931472be6cad", // ContextKeySegmentedRegExFilter
	"java-runtime-820bb6f72b84ad070ce4", // ContextKeySegmentedSubtype
}

var contextKeySegmentedAllocationTimeJavaExecutions = []string{
	"ContextKeySegmentedWPatternFireWhenAllocated",
	"ContextKeySegmentedRegExFilter",
	"ContextKeySegmentedSubtype",
}

var contextKeySegmentedAllocationTimeCaseRuntimeIDs = map[string]string{
	"pattern-fire-when-allocated": "java-runtime-57199db349abfe7ad70e",
	"regex-filter":                "java-runtime-9356c517931472be6cad",
	"subtype":                     "java-runtime-820bb6f72b84ad070ce4",
}

// contextKeySegmentedAllocationBean mirrors SupportBean's asserted fields.
type contextKeySegmentedAllocationBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func runContextKeySegmentedAllocationTimeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	for _, caseName := range scenarioCaseOrder(scenario) {
		caseTrace, err := runContextKeySegmentedAllocationTimeCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", contextKeySegmentedAllocationTimeID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runContextKeySegmentedAllocationTimeCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()

	var plans []esper.Plan
	contextName := ""
	switch caseName {
	case "pattern-fire-when-allocated":
		contextName = "MyContext"
		if _, err := esper.RegisterStruct[contextKeySegmentedAllocationBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		beanSource := esper.From[contextKeySegmentedAllocationBean](env, "SupportBean")
		theString := esper.Field[contextKeySegmentedAllocationBean, string]("theString")
		// `create context MyContext partition by theString from SupportBean`
		if _, err := esper.CreateKeyContextByStreams(env, contextName,
			esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{theString}},
		); err != nil {
			return compat.Trace{}, err
		}
		// `context MyContext create variable String lastString = null`
		if err := env.RegisterContextVariable(contextName, "lastString", nil,
			esper.VariableType(reflect.TypeOf(""))); err != nil {
			return compat.Trace{}, err
		}
		// `@name('s0') context MyContext select context.key1 as key1 from
		// pattern[timer:interval(0)]`
		selectPlan, err := env.Build(esper.TimerInterval(beanSource, 0).Select(
			esper.Alias("key1", esper.ContextKeyValue[string](0)),
		).Query(esper.StatementName("s0"), esper.WithContext(contextName)))
		if err != nil {
			return compat.Trace{}, err
		}
		// `context MyContext on pattern[timer:interval(0)] set lastString =
		// context.key1`
		triggerPlan, err := env.Build(esper.OnPattern(esper.TimerInterval(beanSource, 0)).
			SetVariable("lastString", esper.ContextKeyValue[string](0)).
			Query(esper.WithContext(contextName)))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = []esper.Plan{selectPlan, triggerPlan}
	case "regex-filter":
		contextName = "MyContext"
		if _, err := esper.RegisterMap(env, "MyEventWPartition", []esper.FieldSpec{
			esper.FieldDef("number", reflect.TypeOf(0)),
			esper.FieldDef("description", reflect.TypeOf("")),
			esper.FieldDef("partitionId", reflect.TypeOf("")),
		}); err != nil {
			return compat.Trace{}, err
		}
		// `create context MyContext partition by partitionId from
		// MyEventWPartition terminated after 15 minutes`
		if _, err := esper.CreateKeyContextByStreamsTerminatedAfter(env, contextName, 15*time.Minute,
			esper.KeyContextStream{Type: "MyEventWPartition", Keys: []esper.Expr{esper.Field[map[string]any, string]("partitionId")}},
		); err != nil {
			return compat.Trace{}, err
		}
		// `@name('s0') context MyContext select * from
		// MyEventWPartition(description like "%hello%")`
		plan, err := env.Build(esper.From[map[string]any](env, "MyEventWPartition").
			Filter(esper.Like(esper.Field[map[string]any, string]("description"), esper.Literal("%hello%"))).
			Query(esper.StatementName("s0"), esper.WithContext(contextName)))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = []esper.Plan{plan}
	case "subtype":
		contextName = "SegmentedByString"
		baseAB, err := esper.RegisterMap(env, "ISupportBaseAB", []esper.FieldSpec{
			esper.FieldDef("baseAB", reflect.TypeOf("")),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		supportA, err := esper.RegisterMap(env, "ISupportA", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
		}, esper.WithSchemaParent(baseAB))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.RegisterMap(env, "ISupportAImpl", nil, esper.WithSchemaParent(supportA)); err != nil {
			return compat.Trace{}, err
		}
		// `@Name('context') create context SegmentedByString partition by
		// baseAB from ISupportBaseAB`
		if _, err := esper.CreateKeyContextByStreams(env, contextName,
			esper.KeyContextStream{Type: "ISupportBaseAB", Keys: []esper.Expr{esper.Field[map[string]any, string]("baseAB")}},
		); err != nil {
			return compat.Trace{}, err
		}
		// `@name('s0') context SegmentedByString select count(*) as col1
		// from ISupportA`
		plan, err := env.Build(esper.From[map[string]any](env, "ISupportA").Aggregate(
			esper.Alias("col1", esper.CountAll()),
		).Query(esper.StatementName("s0"), esper.WithContext(contextName)))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = []esper.Plan{plan}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", contextKeySegmentedAllocationTimeID, caseName)
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(contextKeySegmentedAllocationTimeCaseRuntimeIDs[caseName]),
	)
	deployment, err := engine.DeployPlans(ctx, plans)
	if err != nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, err
	}
	var statement *esper.Statement
	for _, candidate := range deployment.Statements() {
		if candidate.Name() == "s0" {
			statement = candidate
			break
		}
	}
	if statement == nil {
		_ = engine.Close(context.Background())
		return compat.Trace{}, fmt.Errorf("%s case %q did not deploy statement s0", contextKeySegmentedAllocationTimeID, caseName)
	}
	defer func() { _ = engine.Close(context.Background()) }()

	handlers := map[string]compat.StepHandler{}
	if caseName == "pattern-fire-when-allocated" {
		name := contextName
		handlers["read-variable"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			// The step's filterValue carries the single partition key for
			// the segmented selector, mirroring the execution's
			// SupportSelectorPartitioned(theString) read.
			key := step.FilterValue
			if key == "" {
				return nil, fmt.Errorf("%s: read-variable %q requires a partition key in filterValue", contextKeySegmentedAllocationTimeID, step.Name)
			}
			states, err := engine.ContextVariableStates(ctx, name,
				esper.SelectContextPartitionSegments([]any{key}), step.Name)
			if err != nil {
				return nil, fmt.Errorf("%s: read-variable %q failed: %w", contextKeySegmentedAllocationTimeID, step.Name, err)
			}
			if len(states) != 1 {
				return nil, fmt.Errorf("%s: read-variable %q returned %d states, want 1", contextKeySegmentedAllocationTimeID, step.Name, len(states))
			}
			value, ok := states[0].Values[step.Name]
			if !ok {
				return nil, fmt.Errorf("%s: read-variable %q missing in partition state", contextKeySegmentedAllocationTimeID, step.Name)
			}
			return []compat.TraceRecord{{
				Case:      caseName,
				Operation: "variable",
				Statement: step.Statement,
				Name:      step.Name,
				Value:     contextVariablesField(value),
			}}, nil
		}
	}
	return compat.ReplayWithStatementsAndHandlers(ctx, engine, statement, caseScenario, decodeContextKeySegmentedAllocationTimePayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", contextKeySegmentedAllocationTimeID, name)
			}
			return statement, nil
		}, handlers)
}

func decodeContextKeySegmentedAllocationTimePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextKeySegmentedAllocationBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "MyEventWPartition":
		var fields struct {
			Number      int    `json:"number"`
			Description string `json:"description"`
			PartitionID string `json:"partitionId"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return nil, fmt.Errorf("decode MyEventWPartition: %w", err)
		}
		return map[string]any{
			"number":      fields.Number,
			"description": fields.Description,
			"partitionId": fields.PartitionID,
		}, nil
	case "ISupportAImpl":
		var fields struct {
			A      string `json:"a"`
			BaseAB string `json:"baseAB"`
		}
		if err := json.Unmarshal(step.Payload, &fields); err != nil {
			return nil, fmt.Errorf("decode ISupportAImpl: %w", err)
		}
		return map[string]any{
			"a":      fields.A,
			"baseAB": fields.BaseAB,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", contextKeySegmentedAllocationTimeID, step.EventType)
	}
}
