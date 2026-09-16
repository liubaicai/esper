package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultsetOutputLimitRowPerGroupMultikeyBean mirrors SupportBean for the
// (theString,longPrimitive) multi-key group-bys; intPrimitive feeds the sum.
type resultsetOutputLimitRowPerGroupMultikeyBean struct {
	TheString     string `esper:"theString"`
	LongPrimitive int64  `esper:"longPrimitive"`
	IntPrimitive  int32  `esper:"intPrimitive"`
}

// resultsetOutputLimitRowPerGroupMultikeyIntArray mirrors
// SupportEventWithIntArray; the []int array is the group-by key, matched by
// content equality through encodeKey.
type resultsetOutputLimitRowPerGroupMultikeyIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

const resultsetOutputLimitRowPerGroupMultikeyJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var resultsetOutputLimitRowPerGroupMultikeyJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowPerGroup.java",
}

var (
	// Inventory order is authoritative: OutputFirstMultikeyWArray is ordinal
	// 39, OutputAllMultikeyWArray 40, OutputLastMultikeyWArray 41 and
	// OutputSnapshotMultikeyWArray 42 in
	// ResultSetOutputLimitRowPerGroup.executions().
	resultsetOutputLimitRowPerGroupMultikeyJavaRuntimeIDs = []string{
		"java-runtime-4b69198e8a2cc665a379", // ResultSetOutputFirstMultikeyWArray
		"java-runtime-49a1acbfbe84da2b0479", // ResultSetOutputAllMultikeyWArray
		"java-runtime-80584d4ff67f59c3a260", // ResultSetOutputLastMultikeyWArray
		"java-runtime-dbe1c30970ff2232fc35", // ResultSetOutputSnapshotMultikeyWArray
	}
	resultsetOutputLimitRowPerGroupMultikeyJavaExecutions = []string{
		"ResultSetOutputFirstMultikeyWArray",
		"ResultSetOutputAllMultikeyWArray",
		"ResultSetOutputLastMultikeyWArray",
		"ResultSetOutputSnapshotMultikeyWArray",
	}
)

const (
	resultsetOutputLimitRowPerGroupMultikeyFirst    = "first-multikey-warray"
	resultsetOutputLimitRowPerGroupMultikeyAll      = "all-multikey-warray"
	resultsetOutputLimitRowPerGroupMultikeyLast     = "last-multikey-warray"
	resultsetOutputLimitRowPerGroupMultikeySnapshot = "snapshot-multikey-warray"
	resultsetOutputLimitRowPerGroupMultikeyID       = "resultset-output-limit-row-per-group-multikey"
)

var resultsetOutputLimitRowPerGroupMultikeyCaseOrder = []string{
	resultsetOutputLimitRowPerGroupMultikeyFirst,
	resultsetOutputLimitRowPerGroupMultikeyAll,
	resultsetOutputLimitRowPerGroupMultikeyLast,
	resultsetOutputLimitRowPerGroupMultikeySnapshot,
}

// runResultSetOutputLimitRowPerGroupMultikeyScenario replays the four
// time-based multi-key output-limiting executions from
// ResultSetOutputLimitRowPerGroup. Each case receives a fresh environment and
// runtime, matching the isolated Java execution lifecycle.
func runResultSetOutputLimitRowPerGroupMultikeyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(resultsetOutputLimitRowPerGroupMultikeyCaseOrder))
	for _, caseName := range resultsetOutputLimitRowPerGroupMultikeyCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runResultSetOutputLimitRowPerGroupMultikeyCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitRowPerGroupMultikeyID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitRowPerGroupMultikeyID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetOutputLimitRowPerGroupMultikeyCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowPerGroupMultikeyBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitRowPerGroupMultikeyIntArray](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}

	query, err := resultsetOutputLimitRowPerGroupMultikeyQuery(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("build %q: %w", caseName, err)
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetOutputLimitRowPerGroupMultikeyRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("deploy %q: %w", caseName, err)
	}
	if len(deployment.Statements()) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one parity statement, got %d", len(deployment.Statements()))
	}
	statement := deployment.Statements()[0]
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario,
		decodeResultSetOutputLimitRowPerGroupMultikeyPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOutputLimitRowPerGroupMultikeyID, name)
			}
			return statement, nil
		})
}

// resultsetOutputLimitRowPerGroupMultikeyQuery builds the single s0 statement
// for a case, mirroring the Java EPL byte-for-byte in the typed surface.
func resultsetOutputLimitRowPerGroupMultikeyQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	theString := esper.Field[resultsetOutputLimitRowPerGroupMultikeyBean, string]("theString")
	longPrimitive := esper.Field[resultsetOutputLimitRowPerGroupMultikeyBean, int64]("longPrimitive")
	intPrimitive := esper.Field[resultsetOutputLimitRowPerGroupMultikeyBean, int32]("intPrimitive")
	switch caseName {
	case resultsetOutputLimitRowPerGroupMultikeyFirst:
		array := esper.Field[resultsetOutputLimitRowPerGroupMultikeyIntArray, []int]("array")
		value := esper.Field[resultsetOutputLimitRowPerGroupMultikeyIntArray, int]("value")
		return esper.From[resultsetOutputLimitRowPerGroupMultikeyIntArray](env, "SupportEventWithIntArray").
			GroupBy(array).
			Select(esper.Alias("thesum", esper.Sum[int](value))).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputFirstEveryTime(10*time.Second))), nil
	case resultsetOutputLimitRowPerGroupMultikeyAll:
		return esper.From[resultsetOutputLimitRowPerGroupMultikeyBean](env, "SupportBean").
			Window(esper.KeepAll()).
			GroupBy(theString, longPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("longPrimitive", longPrimitive),
				esper.Alias("thesum", esper.Sum[int32](intPrimitive)),
			).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryTime(time.Second))), nil
	case resultsetOutputLimitRowPerGroupMultikeyLast:
		return esper.From[resultsetOutputLimitRowPerGroupMultikeyBean](env, "SupportBean").
			Window(esper.KeepAll()).
			GroupBy(theString, longPrimitive).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("longPrimitive", longPrimitive),
				esper.Alias("thesum", esper.Sum[int32](intPrimitive)),
			).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputLastEveryTime(time.Second))), nil
	case resultsetOutputLimitRowPerGroupMultikeySnapshot:
		return esper.From[resultsetOutputLimitRowPerGroupMultikeyBean](env, "SupportBean").
			GroupBy(theString, longPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", longPrimitive),
				esper.Alias("c2", esper.Sum[int32](intPrimitive)),
			).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputSnapshotEvery(10*time.Second))), nil
	default:
		return esper.Query{}, fmt.Errorf("unsupported %s case %q", resultsetOutputLimitRowPerGroupMultikeyID, caseName)
	}
}

func resultsetOutputLimitRowPerGroupMultikeyRuntimeURI(caseName string) string {
	switch caseName {
	case resultsetOutputLimitRowPerGroupMultikeyFirst:
		return resultsetOutputLimitRowPerGroupMultikeyJavaRuntimeIDs[0]
	case resultsetOutputLimitRowPerGroupMultikeyAll:
		return resultsetOutputLimitRowPerGroupMultikeyJavaRuntimeIDs[1]
	case resultsetOutputLimitRowPerGroupMultikeyLast:
		return resultsetOutputLimitRowPerGroupMultikeyJavaRuntimeIDs[2]
	case resultsetOutputLimitRowPerGroupMultikeySnapshot:
		return resultsetOutputLimitRowPerGroupMultikeyJavaRuntimeIDs[3]
	default:
		return "parity-" + resultsetOutputLimitRowPerGroupMultikeyID + "-" + caseName
	}
}

func decodeResultSetOutputLimitRowPerGroupMultikeyPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value resultsetOutputLimitRowPerGroupMultikeyBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportEventWithIntArray":
		var value resultsetOutputLimitRowPerGroupMultikeyIntArray
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithIntArray: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitRowPerGroupMultikeyID, step.EventType)
	}
}
