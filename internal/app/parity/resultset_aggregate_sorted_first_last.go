package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateSortedFirstLastBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateSortedFirstLastTrigger struct {
	ID int `esper:"id"`
}

const (
	resultsetAggregateSortedFirstLastID          = "resultset-aggregate-sorted-first-last"
	resultsetAggregateSortedFirstLastJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateSortedFirstLastDescription = "ResultSetAggregationMethodSorted ordinals 5-6: table-backed sorted first/last event, bucket, and key access with enumeration projections."
	resultsetAggregateSortedFirstLastSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java"
	resultsetAggregateSortedFirstLastInventoryID = "java-0ded2b32c81677a9d36f"
)

var (
	resultsetAggregateSortedFirstLastJavaRuntimeIDs = []string{
		"java-runtime-2ec322fbd681b83590f1",
		"java-runtime-b7e36a11fb9b9c982249",
	}
	resultsetAggregateSortedFirstLastJavaStaticIDs = []string{
		resultsetAggregateSortedFirstLastInventoryID,
		"java-5eb997e432653280b46b",
	}
	resultsetAggregateSortedFirstLastJavaExecutions = []string{
		"ResultSetAggregateSortedFirstLast",
		"ResultSetAggregateSortedFirstLastEnumerationAndDot",
	}
	resultsetAggregateSortedFirstLastCases = []string{
		"first-last",
		"first-last-dot",
	}
	resultsetAggregateSortedFirstLastOrdinals = []int{5, 6}
	resultsetAggregateSortedFirstLastEPLs     = []string{
		"create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\ninto table MyTable select sorted(*) as sortcol from SupportBean;\n@name('s0') select MyTable.sortcol.firstEvent() as fe,MyTable.sortcol.minBy() as minb,MyTable.sortcol.firstEvents() as fes,MyTable.sortcol.firstKey() as fk,MyTable.sortcol.lastEvent() as le,MyTable.sortcol.maxBy() as maxb,MyTable.sortcol.lastEvents() as les,MyTable.sortcol.lastKey() as lk from SupportBean_S0",
		"create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\ninto table MyTable select sorted(*) as sortcol from SupportBean;\n@name('s0') select MyTable.sortcol.firstEvent().theString as feid,MyTable.sortcol.firstEvent().firstOf() as fefo,MyTable.sortcol.firstEvents().lastOf() as feslo,MyTable.sortcol.lastEvent().theString() as leid,MyTable.sortcol.lastEvent().firstOf() as lefo,MyTable.sortcol.lastEvents().lastOf as leslo from SupportBean_S0",
	}
)

type resultsetAggregateSortedFirstLastSeed struct {
	name string
	key  int
}

var resultsetAggregateSortedFirstLastSeeds = []resultsetAggregateSortedFirstLastSeed{
	{name: "E1a", key: 1},
	{name: "E1b", key: 1},
	{name: "E4b", key: 4},
	{name: "E6a", key: 6},
	{name: "E6b", key: 6},
	{name: "E8", key: 8},
	{name: "E9", key: 9},
}

func loadResultSetAggregateSortedFirstLastScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateSortedFirstLastID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateSortedFirstLastID, err)
	}
	if err := rejectResultSetAggregateSortedFirstLastDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedFirstLastID, err)
	}
	root, err := decodeResultSetAggregateSortedFirstLastJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedFirstLastID, err)
	}
	if err := requireResultSetAggregateSortedFirstLastFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		value, err := decodeResultSetAggregateSortedFirstLastString(root[name], name)
		if err != nil {
			return compat.Scenario{}, err
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetAggregateSortedFirstLastID ||
		metadata["description"] != resultsetAggregateSortedFirstLastDescription ||
		metadata["javaCommit"] != resultsetAggregateSortedFirstLastJavaCommit ||
		metadata["javaSource"] != resultsetAggregateSortedFirstLastSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateSortedFirstLastID)
	}
	if err := validateResultSetAggregateSortedFirstLastStringArray(root["javaRuntimes"], resultsetAggregateSortedFirstLastJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedFirstLastStringArray(root["javaNames"], resultsetAggregateSortedFirstLastJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedFirstLastStringArray(root["javaStaticIds"], resultsetAggregateSortedFirstLastJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedFirstLastStringArray(root["javaFlags"], nil, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	rawCases, err := decodeResultSetAggregateSortedFirstLastJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != len(resultsetAggregateSortedFirstLastCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", resultsetAggregateSortedFirstLastID)
	}
	for index, rawCase := range rawCases {
		object, err := decodeResultSetAggregateSortedFirstLastJSONObject(rawCase)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetAggregateSortedFirstLastFields(object, "case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseName, err := decodeResultSetAggregateSortedFirstLastString(object["case"], "case")
		if err != nil {
			return compat.Scenario{}, err
		}
		ordinal, err := decodeResultSetAggregateSortedFirstLastInt(object["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		runtimeID, err := decodeResultSetAggregateSortedFirstLastString(object["runtimeId"], "runtimeId")
		if err != nil {
			return compat.Scenario{}, err
		}
		executionName, err := decodeResultSetAggregateSortedFirstLastString(object["executionName"], "executionName")
		if err != nil {
			return compat.Scenario{}, err
		}
		observation, err := decodeResultSetAggregateSortedFirstLastString(object["observation"], "observation")
		if err != nil {
			return compat.Scenario{}, err
		}
		iteratorSnapshots, err := decodeResultSetAggregateSortedFirstLastInt(object["iteratorSnapshots"], "iteratorSnapshots")
		if err != nil {
			return compat.Scenario{}, err
		}
		epl, err := decodeResultSetAggregateSortedFirstLastString(object["epl"], "epl")
		if err != nil {
			return compat.Scenario{}, err
		}
		if caseName != resultsetAggregateSortedFirstLastCases[index] || ordinal != resultsetAggregateSortedFirstLastOrdinals[index] ||
			runtimeID != resultsetAggregateSortedFirstLastJavaRuntimeIDs[index] || executionName != resultsetAggregateSortedFirstLastJavaExecutions[index] ||
			observation != "listener" || iteratorSnapshots != 0 || epl != resultsetAggregateSortedFirstLastEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateSortedFirstLastID, index)
		}
	}

	rawSteps, err := decodeResultSetAggregateSortedFirstLastJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	steps := make([]compat.Step, 0, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateSortedFirstLastJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultSetAggregateSortedFirstLastString(object["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		switch op {
		case "case":
			if err := requireResultSetAggregateSortedFirstLastFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultSetAggregateSortedFirstLastString(object["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps = append(steps, compat.Step{Op: op, Case: caseName})
		case "send":
			if err := requireResultSetAggregateSortedFirstLastFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			eventType, err := decodeResultSetAggregateSortedFirstLastString(object["eventType"], fmt.Sprintf("scenario step %d eventType", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			if _, err := decodeResultSetAggregateSortedFirstLastJSONObject(object["payload"]); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			steps = append(steps, compat.Step{Op: op, EventType: eventType, Payload: append(json.RawMessage(nil), object["payload"]...)})
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, op)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateSortedFirstLastScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func runResultSetAggregateSortedFirstLastScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedFirstLastScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateSortedFirstLastCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateSortedFirstLastCase(ctx, caseScenario, index, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateSortedFirstLastID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateSortedFirstLastCase(ctx context.Context, scenario compat.Scenario, index int, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedFirstLastBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateSortedFirstLastTrigger](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
		esper.TableColumnOf[esper.SortedAccessValue[int, esper.Event]]("sortcol"),
	}); err != nil {
		return compat.Trace{}, err
	}

	intPrimitive := esper.Field[resultsetAggregateSortedFirstLastBean, int]("intPrimitive")
	sorted := esper.SortedAccessBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	aggregatePlan, err := env.Build(esper.From[resultsetAggregateSortedFirstLastBean](env, "SupportBean").
		Window(esper.KeepAll()).
		Aggregate(esper.Alias("sortcol", sorted)).
		IntoTable("MyTable", esper.StatementName("aggregate")))
	if err != nil {
		return compat.Trace{}, err
	}

	sortcol := esper.TableField[esper.SortedAccessValue[int, esper.Event]]("sortcol")
	firstEvent := esper.Method[esper.Event](sortcol, "FirstEvent")
	lastEvent := esper.Method[esper.Event](sortcol, "LastEvent")
	firstEvents := esper.Method[[]esper.Event](sortcol, "FirstEvents")
	lastEvents := esper.Method[[]esper.Event](sortcol, "LastEvents")
	var triggerPlan esper.Plan
	switch index {
	case 0:
		triggerPlan, err = env.Build(esper.OnEvent(esper.From[resultsetAggregateSortedFirstLastTrigger](env, "SupportBean_S0")).
			SelectFromTable("MyTable", nil,
				esper.Alias("fe", firstEvent),
				esper.Alias("minb", esper.Method[esper.Event](sortcol, "MinBy")),
				esper.Alias("fes", firstEvents),
				esper.Alias("fk", esper.Method[int](sortcol, "FirstKey")),
				esper.Alias("le", lastEvent),
				esper.Alias("maxb", esper.Method[esper.Event](sortcol, "MaxBy")),
				esper.Alias("les", lastEvents),
				esper.Alias("lk", esper.Method[int](sortcol, "LastKey")),
			).Query(esper.StatementName("s0")))
	case 1:
		triggerPlan, err = env.Build(esper.OnEvent(esper.From[resultsetAggregateSortedFirstLastTrigger](env, "SupportBean_S0")).
			SelectFromTable("MyTable", nil,
				esper.Alias("feid", esper.Property[string](firstEvent, "theString")),
				esper.Alias("fefo", esper.EnumFirstOf[esper.Event](firstEvents)),
				esper.Alias("feslo", esper.EnumLastOf[esper.Event](firstEvents)),
				esper.Alias("leid", esper.Property[string](lastEvent, "theString")),
				esper.Alias("lefo", esper.EnumFirstOf[esper.Event](lastEvents)),
				esper.Alias("leslo", esper.EnumLastOf[esper.Event](lastEvents)),
			).Query(esper.StatementName("s0")))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported case index %d", index)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedFirstLastJavaRuntimeIDs[index]),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(ctx, aggregatePlan); err != nil {
		return compat.Trace{}, err
	}
	deployment, err := engine.Deploy(ctx, triggerPlan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("expected one trigger statement, got %d", len(statements))
	}
	statement := statements[0]
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateSortedFirstLastPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateSortedFirstLastID, name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	trace = normalizeResultSetAggregateSortedFirstLastTrace(trace)
	if err := validateResultSetAggregateSortedFirstLastTrace(trace, index, caseName); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func decodeResultSetAggregateSortedFirstLastPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		payload, err := decodeResultSetAggregateSortedFirstLastJSONObject(step.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		if err := requireResultSetAggregateSortedFirstLastFields(payload, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		name, err := decodeResultSetAggregateSortedFirstLastString(payload["theString"], "theString")
		if err != nil {
			return nil, err
		}
		key, err := decodeResultSetAggregateSortedFirstLastInt(payload["intPrimitive"], "intPrimitive")
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedFirstLastBean{TheString: name, IntPrimitive: key}, nil
	case "SupportBean_S0":
		payload, err := decodeResultSetAggregateSortedFirstLastJSONObject(step.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if err := requireResultSetAggregateSortedFirstLastFields(payload, "id"); err != nil {
			return nil, err
		}
		id, err := decodeResultSetAggregateSortedFirstLastInt(payload["id"], "id")
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedFirstLastTrigger{ID: id}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateSortedFirstLastID, step.EventType)
	}
}

func validateResultSetAggregateSortedFirstLastScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateSortedFirstLastID {
		return fmt.Errorf("%s scenario has unsupported id %q", resultsetAggregateSortedFirstLastID, scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range resultsetAggregateSortedFirstLastCases {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario is missing case %q", resultsetAggregateSortedFirstLastID, caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("%s scenario case marker %d is not pinned to %q", resultsetAggregateSortedFirstLastID, caseIndex, caseName)
		}
		stepIndex++
		for seedIndex, expected := range resultsetAggregateSortedFirstLastSeeds {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("%s case %q is missing seed %d", resultsetAggregateSortedFirstLastID, caseName, seedIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportBean" || step.Case != "" {
				return fmt.Errorf("%s case %q seed %d is not a SupportBean send", resultsetAggregateSortedFirstLastID, caseName, seedIndex)
			}
			payload, err := decodeResultSetAggregateSortedFirstLastJSONObject(step.Payload)
			if err != nil {
				return fmt.Errorf("%s case %q seed %d: %w", resultsetAggregateSortedFirstLastID, caseName, seedIndex, err)
			}
			if err := requireResultSetAggregateSortedFirstLastFields(payload, "theString", "intPrimitive"); err != nil {
				return fmt.Errorf("%s case %q seed %d: %w", resultsetAggregateSortedFirstLastID, caseName, seedIndex, err)
			}
			name, err := decodeResultSetAggregateSortedFirstLastString(payload["theString"], "theString")
			if err != nil {
				return fmt.Errorf("%s case %q seed %d: %w", resultsetAggregateSortedFirstLastID, caseName, seedIndex, err)
			}
			key, err := decodeResultSetAggregateSortedFirstLastInt(payload["intPrimitive"], "intPrimitive")
			if err != nil {
				return fmt.Errorf("%s case %q seed %d: %w", resultsetAggregateSortedFirstLastID, caseName, seedIndex, err)
			}
			if name != expected.name || key != expected.key {
				return fmt.Errorf("%s case %q seed %d = (%s,%d), want (%s,%d)", resultsetAggregateSortedFirstLastID, caseName, seedIndex, name, key, expected.name, expected.key)
			}
			stepIndex++
		}
		if err := validateResultSetAggregateSortedFirstLastTriggerStep(scenario.Steps, &stepIndex); err != nil {
			return fmt.Errorf("%s case %q: %w", resultsetAggregateSortedFirstLastID, caseName, err)
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetAggregateSortedFirstLastID)
	}
	return nil
}

func validateResultSetAggregateSortedFirstLastTriggerStep(steps []compat.Step, index *int) error {
	if index == nil || *index >= len(steps) {
		return fmt.Errorf("missing SupportBean_S0 trigger")
	}
	step := steps[*index]
	if step.Op != "send" || step.EventType != "SupportBean_S0" || step.Case != "" {
		return fmt.Errorf("step %d is not a SupportBean_S0 trigger", *index)
	}
	payload, err := decodeResultSetAggregateSortedFirstLastJSONObject(step.Payload)
	if err != nil {
		return fmt.Errorf("trigger payload: %w", err)
	}
	if err := requireResultSetAggregateSortedFirstLastFields(payload, "id"); err != nil {
		return err
	}
	actual, err := decodeResultSetAggregateSortedFirstLastInt(payload["id"], "id")
	if err != nil {
		return err
	}
	if actual != -1 {
		return fmt.Errorf("trigger id = %d, want -1", actual)
	}
	*index++
	return nil
}

func validateResultSetAggregateSortedFirstLastTrace(trace compat.Trace, index int, caseName string) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultsetAggregateSortedFirstLastID {
		return fmt.Errorf("%s trace identity is not pinned", resultsetAggregateSortedFirstLastID)
	}
	expected := resultsetAggregateSortedFirstLastExpectedRows(index)
	if len(trace.Records) != len(expected) {
		return fmt.Errorf("%s case %q trace has %d records, want %d", resultsetAggregateSortedFirstLastID, caseName, len(trace.Records), len(expected))
	}
	for recordIndex, want := range expected {
		record := trace.Records[recordIndex]
		if record.Case != caseName || record.Operation != "listener" || record.Statement != "s0" ||
			record.Sequence != uint64(recordIndex+1) || record.Time != "1970-01-01T00:00:00Z" || len(record.New) != 1 || len(record.Old) != 0 {
			return fmt.Errorf("%s case %q trace record %d metadata is not pinned", resultsetAggregateSortedFirstLastID, caseName, recordIndex)
		}
		row := record.New[0]
		if row.Kind != "row" || len(row.Fields) != len(want) {
			return fmt.Errorf("%s case %q trace record %d row shape is not pinned", resultsetAggregateSortedFirstLastID, caseName, recordIndex)
		}
		for name, expectedValue := range want {
			if actual := row.Fields[name]; !reflect.DeepEqual(actual, expectedValue) {
				return fmt.Errorf("%s case %q record %d field %q = %#v, want %#v", resultsetAggregateSortedFirstLastID, caseName, recordIndex, name, actual, expectedValue)
			}
		}
	}
	return nil
}

func resultsetAggregateSortedFirstLastExpectedRows(index int) []map[string]any {
	seed := resultsetAggregateSortedFirstLastSeeds
	firstBucket := []resultsetAggregateSortedFirstLastSeed{seed[0], seed[1]}
	lastBucket := []resultsetAggregateSortedFirstLastSeed{seed[6]}
	eventRow := resultsetAggregateSortedFirstLastEventRow
	eventRows := resultsetAggregateSortedFirstLastEventRows
	null := resultsetAggregateSortedFirstLastNull
	switch index {
	case 0:
		return []map[string]any{{
			"fe":   eventRow(seed[0]),
			"minb": eventRow(seed[0]),
			"fes":  eventRows(firstBucket),
			"fk":   seed[0].key,
			"le":   eventRow(seed[6]),
			"maxb": eventRow(seed[6]),
			"les":  eventRows(lastBucket),
			"lk":   seed[6].key,
		}}
	case 1:
		return []map[string]any{{
			"feid":  seed[0].name,
			"fefo":  eventRow(seed[0]),
			"feslo": eventRow(seed[1]),
			"leid":  seed[6].name,
			"lefo":  eventRow(seed[6]),
			"leslo": eventRow(seed[6]),
		}}
	default:
		_ = null
		return nil
	}
}

func resultsetAggregateSortedFirstLastEventRow(seed resultsetAggregateSortedFirstLastSeed) map[string]any {
	return map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": seed.key, "theString": seed.name}}
}

func resultsetAggregateSortedFirstLastEventRows(seeds []resultsetAggregateSortedFirstLastSeed) []any {
	rows := make([]any, 0, len(seeds))
	for _, seed := range seeds {
		rows = append(rows, resultsetAggregateSortedFirstLastEventRow(seed))
	}
	return rows
}

func resultsetAggregateSortedFirstLastNull() map[string]any {
	return map[string]any{"state": "null"}
}

func normalizeResultSetAggregateSortedFirstLastTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for _, rows := range [][]compat.ResultRecord{record.New, record.Old} {
			for rowIndex := range rows {
				for name, value := range rows[rowIndex].Fields {
					rows[rowIndex].Fields[name] = normalizeResultSetAggregateSortedFirstLastValue(value)
				}
			}
		}
	}
	return trace
}

func normalizeResultSetAggregateSortedFirstLastValue(value any) any {
	switch typed := value.(type) {
	case []esper.Event:
		rows := make([]any, 0, len(typed))
		for _, event := range typed {
			rows = append(rows, normalizeResultSetAggregateSortedFirstLastEvent(event))
		}
		return rows
	case esper.Event:
		return normalizeResultSetAggregateSortedFirstLastEvent(typed)
	case []any:
		for index := range typed {
			typed[index] = normalizeResultSetAggregateSortedFirstLastValue(typed[index])
		}
		return typed
	case map[string]any:
		if typed["kind"] == "row" || typed["state"] != nil {
			return typed
		}
		for name, nested := range typed {
			typed[name] = normalizeResultSetAggregateSortedFirstLastValue(nested)
		}
		return typed
	default:
		return value
	}
}

func normalizeResultSetAggregateSortedFirstLastEvent(event esper.Event) map[string]any {
	fields := make(map[string]any)
	for _, field := range event.Schema().Fields() {
		value := event.Get(field.Name)
		if value.IsMissing() {
			fields[field.Name] = map[string]any{"state": "missing"}
		} else if value.IsNull() {
			fields[field.Name] = map[string]any{"state": "null"}
		} else {
			fields[field.Name] = value.Any()
		}
	}
	return map[string]any{"kind": "row", "fields": fields}
}

func decodeResultSetAggregateSortedFirstLastJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	object := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("JSON object key is not a string")
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("JSON object contains duplicate field %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		object[key] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, fmt.Errorf("JSON object is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON value contains trailing data")
		}
		return nil, err
	}
	return object, nil
}

func decodeResultSetAggregateSortedFirstLastJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, fmt.Errorf("JSON value must be an array")
	}
	values := make([]json.RawMessage, 0)
	for decoder.More() {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim(']') {
		return nil, fmt.Errorf("JSON array is not closed")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON value contains trailing data")
		}
		return nil, err
	}
	return values, nil
}

func requireResultSetAggregateSortedFirstLastFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func decodeResultSetAggregateSortedFirstLastString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateSortedFirstLastInt(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok || !resultsetAggregateSortedFirstLastIntegerSyntax(string(number)) {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func resultsetAggregateSortedFirstLastIntegerSyntax(text string) bool {
	if text == "0" {
		return true
	}
	if text == "" {
		return false
	}
	if text[0] == '-' {
		text = text[1:]
	}
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validateResultSetAggregateSortedFirstLastStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateSortedFirstLastJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateSortedFirstLastString(rawValue, name)
		if err != nil || value != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func rejectResultSetAggregateSortedFirstLastDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateSortedFirstLastJSON(decoder); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON value contains trailing data")
		}
		return err
	}
	return nil
}

func walkResultSetAggregateSortedFirstLastJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("JSON object contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := walkResultSetAggregateSortedFirstLastJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case json.Delim('['):
		for decoder.More() {
			if err := walkResultSetAggregateSortedFirstLastJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("JSON array is not closed")
		}
	case json.Delim(']'), json.Delim('}'):
		return fmt.Errorf("unexpected JSON delimiter %q", token)
	}
	return nil
}
