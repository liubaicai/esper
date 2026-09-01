package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type resultsetAggregateSortedTableAccessBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateSortedTableAccessTrigger struct {
	ID int `esper:"id"`
}

type resultsetAggregateSortedTableAccessSubmapEvent struct {
	FromKey       int  `esper:"fromKey"`
	FromInclusive bool `esper:"fromInclusive"`
	ToKey         int  `esper:"toKey"`
	ToInclusive   bool `esper:"toInclusive"`
}

const (
	resultsetAggregateSortedTableAccessID          = "resultset-aggregate-sorted-table-access"
	resultsetAggregateSortedTableAccessJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateSortedTableAccessDescription = "ResultSetAggregationMethodSorted ordinals 7-9: table-backed sorted get/contains/counts, inclusive submap/eventsBetween ranges, and detached navigable-map projections over duplicate-key buckets."
	resultsetAggregateSortedTableAccessSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java"
)

var resultsetAggregateSortedTableAccessJavaRuntimeIDs = []string{
	"java-runtime-d7d60fec056d6eb16659",
	"java-runtime-0c0e45c751e3cafeb454",
	"java-runtime-2117ba8232651a61a058",
}

var resultsetAggregateSortedTableAccessJavaStaticIDs = []string{
	"java-ad219f9a27aebc7885fc",
	"java-4b989f8ba297c8e8d426",
	"java-db7b9e752ef9e4000cc5",
}

var resultsetAggregateSortedTableAccessJavaExecutions = []string{
	"ResultSetAggregateSortedGetContainsCounts",
	"ResultSetAggregateSortedSubmapEventsBetween",
	"ResultSetAggregateSortedNavigableMapReference",
}

var resultsetAggregateSortedTableAccessCases = []string{
	"get-contains-counts",
	"submap-events-between",
	"navigable-map-reference",
}

var resultsetAggregateSortedTableAccessOrdinals = []int{7, 8, 9}

var resultsetAggregateSortedTableAccessEPLs = []string{
	"create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\ninto table MyTable select sorted(*) as sortcol from SupportBean;\n@name('s0') select MyTable.sortcol.getEvent(id) as ge,MyTable.sortcol.getEvents(id) as ges,MyTable.sortcol.containsKey(id) as ck,MyTable.sortcol.countEvents() as cnte,MyTable.sortcol.countKeys() as cntk,MyTable.sortcol.getEvent(id).theString as geid,MyTable.sortcol.getEvent(id).firstOf() as gefo,MyTable.sortcol.getEvents(id).lastOf() as geslo from SupportBean_S0",
	"@public @buseventtype create schema MySubmapEvent as ResultSetAggregationMethodSortedTableAccessScenarioOracle$MySubmapEvent;\ncreate table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\ninto table MyTable select sorted(*) as sortcol from SupportBean;\n@name('s0') select MyTable.sortcol.eventsBetween(fromKey, fromInclusive, toKey, toInclusive) as eb,MyTable.sortcol.eventsBetween(fromKey, fromInclusive, toKey, toInclusive).lastOf() as eblastof,MyTable.sortcol.subMap(fromKey, fromInclusive, toKey, toInclusive) as sm from MySubmapEvent",
	"create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\ninto table MyTable select sorted(*) as sortcol from SupportBean;\n@name('s0') select MyTable.sortcol.navigableMapReference() as nmr from SupportBean_S0",
}

var resultsetAggregateSortedTableAccessSeeds = []resultsetAggregateSortedTableAccessSeed{
	{name: "E1a", key: 1},
	{name: "E1b", key: 1},
	{name: "E4b", key: 4},
	{name: "E6a", key: 6},
	{name: "E6b", key: 6},
	{name: "E8", key: 8},
	{name: "E9", key: 9},
}

type resultsetAggregateSortedTableAccessSeed struct {
	name string
	key  int
}

type resultsetAggregateSortedTableAccessExpectedEvent = resultsetAggregateSortedTableAccessSeed

func loadResultSetAggregateSortedTableAccessScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateSortedTableAccessID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateSortedTableAccessID, err)
	}
	if err := rejectResultSetAggregateSortedTableAccessDuplicateKeys(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedTableAccessID, err)
	}
	root, err := decodeResultSetAggregateSortedTableAccessJSONObject(raw)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateSortedTableAccessID, err)
	}
	if err := requireResultSetAggregateSortedTableAccessFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	metadata := make(map[string]string, 5)
	for _, name := range []string{"version", "id", "description", "javaCommit", "javaSource"} {
		value, err := decodeResultSetAggregateSortedTableAccessString(root[name], name)
		if err != nil {
			return compat.Scenario{}, err
		}
		metadata[name] = value
	}
	if metadata["version"] != compat.ScenarioVersion || metadata["id"] != resultsetAggregateSortedTableAccessID ||
		metadata["description"] != resultsetAggregateSortedTableAccessDescription || metadata["javaCommit"] != resultsetAggregateSortedTableAccessJavaCommit ||
		metadata["javaSource"] != resultsetAggregateSortedTableAccessSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateSortedTableAccessID)
	}
	if err := validateResultSetAggregateSortedTableAccessStringArray(root["javaRuntimes"], resultsetAggregateSortedTableAccessJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedTableAccessStringArray(root["javaNames"], resultsetAggregateSortedTableAccessJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedTableAccessStringArray(root["javaStaticIds"], resultsetAggregateSortedTableAccessJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetAggregateSortedTableAccessStringArray(root["javaFlags"], nil, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	rawCases, err := decodeResultSetAggregateSortedTableAccessJSONArray(root["cases"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario cases: %w", err)
	}
	if len(rawCases) != len(resultsetAggregateSortedTableAccessCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetAggregateSortedTableAccessID)
	}
	for index, rawCase := range rawCases {
		object, err := decodeResultSetAggregateSortedTableAccessJSONObject(rawCase)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetAggregateSortedTableAccessFields(object, "case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		caseName, err := decodeResultSetAggregateSortedTableAccessString(object["case"], "case")
		if err != nil {
			return compat.Scenario{}, err
		}
		ordinal, err := decodeResultSetAggregateSortedTableAccessInt(object["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		runtimeID, err := decodeResultSetAggregateSortedTableAccessString(object["runtimeId"], "runtimeId")
		if err != nil {
			return compat.Scenario{}, err
		}
		executionName, err := decodeResultSetAggregateSortedTableAccessString(object["executionName"], "executionName")
		if err != nil {
			return compat.Scenario{}, err
		}
		observation, err := decodeResultSetAggregateSortedTableAccessString(object["observation"], "observation")
		if err != nil {
			return compat.Scenario{}, err
		}
		iteratorSnapshots, err := decodeResultSetAggregateSortedTableAccessInt(object["iteratorSnapshots"], "iteratorSnapshots")
		if err != nil {
			return compat.Scenario{}, err
		}
		epl, err := decodeResultSetAggregateSortedTableAccessString(object["epl"], "epl")
		if err != nil {
			return compat.Scenario{}, err
		}
		if caseName != resultsetAggregateSortedTableAccessCases[index] || ordinal != resultsetAggregateSortedTableAccessOrdinals[index] ||
			runtimeID != resultsetAggregateSortedTableAccessJavaRuntimeIDs[index] || executionName != resultsetAggregateSortedTableAccessJavaExecutions[index] ||
			observation != "listener" || iteratorSnapshots != 0 || epl != resultsetAggregateSortedTableAccessEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateSortedTableAccessID, index)
		}
	}

	rawSteps, err := decodeResultSetAggregateSortedTableAccessJSONArray(root["steps"])
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario steps: %w", err)
	}
	steps := make([]compat.Step, 0, len(rawSteps))
	for index, rawStep := range rawSteps {
		object, err := decodeResultSetAggregateSortedTableAccessJSONObject(rawStep)
		if err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		op, err := decodeResultSetAggregateSortedTableAccessString(object["op"], fmt.Sprintf("scenario step %d op", index))
		if err != nil {
			return compat.Scenario{}, err
		}
		switch op {
		case "case":
			if err := requireResultSetAggregateSortedTableAccessFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			caseName, err := decodeResultSetAggregateSortedTableAccessString(object["case"], fmt.Sprintf("scenario step %d case", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			steps = append(steps, compat.Step{Op: op, Case: caseName})
		case "send":
			if err := requireResultSetAggregateSortedTableAccessFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			eventType, err := decodeResultSetAggregateSortedTableAccessString(object["eventType"], fmt.Sprintf("scenario step %d eventType", index))
			if err != nil {
				return compat.Scenario{}, err
			}
			if _, err := decodeResultSetAggregateSortedTableAccessJSONObject(object["payload"]); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			steps = append(steps, compat.Step{Op: op, EventType: eventType, Payload: append(json.RawMessage(nil), object["payload"]...)})
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, op)
		}
	}
	scenario := compat.Scenario{Version: metadata["version"], ID: metadata["id"], Steps: steps}
	if err := validateResultSetAggregateSortedTableAccessScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func runResultSetAggregateSortedTableAccessScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateSortedTableAccessScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultsetAggregateSortedTableAccessCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetAggregateSortedTableAccessCase(ctx, caseScenario, index, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateSortedTableAccessID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultSetAggregateSortedTableAccessCase(ctx context.Context, scenario compat.Scenario, index int, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateSortedTableAccessBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if index == 1 {
		if _, err := esper.RegisterStruct[resultsetAggregateSortedTableAccessSubmapEvent](env, "MySubmapEvent"); err != nil {
			return compat.Trace{}, err
		}
	} else if _, err := esper.RegisterStruct[resultsetAggregateSortedTableAccessTrigger](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
		esper.TableColumnOf[esper.SortedAccessValue[int, esper.Event]]("sortcol"),
	}); err != nil {
		return compat.Trace{}, err
	}

	intPrimitive := esper.Field[resultsetAggregateSortedTableAccessBean, int]("intPrimitive")
	sorted := esper.SortedAccessBy[esper.Event, int](esper.EventValue[esper.Event](), intPrimitive)
	aggregatePlan, err := env.Build(esper.From[resultsetAggregateSortedTableAccessBean](env, "SupportBean").
		Window(esper.KeepAll()).
		Aggregate(esper.Alias("sortcol", sorted)).
		IntoTable("MyTable", esper.StatementName("aggregate")))
	if err != nil {
		return compat.Trace{}, err
	}

	sortcol := esper.TableField[esper.SortedAccessValue[int, esper.Event]]("sortcol")
	var triggerPlan esper.Plan
	switch caseName {
	case "get-contains-counts", "navigable-map-reference":
		trigger := esper.From[resultsetAggregateSortedTableAccessTrigger](env, "SupportBean_S0")
		if caseName == "get-contains-counts" {
			id := esper.Field[resultsetAggregateSortedTableAccessTrigger, int]("id")
			getEvent := esper.Method[esper.Event](sortcol, "GetEvent", id)
			getEvents := esper.Method[[]esper.Event](sortcol, "GetEvents", id)
			triggerPlan, err = env.Build(esper.OnEvent(trigger).SelectFromTableWhere("MyTable", esper.Literal(true),
				esper.Alias("ge", getEvent),
				esper.Alias("ges", getEvents),
				esper.Alias("ck", esper.Method[bool](sortcol, "ContainsKey", id)),
				esper.Alias("cnte", esper.Method[int64](sortcol, "CountEvents")),
				esper.Alias("cntk", esper.Method[int64](sortcol, "CountKeys")),
				esper.Alias("geid", esper.Property[string](getEvent, "theString")),
				esper.Alias("gefo", esper.EnumFirstOf[esper.Event](getEvents)),
				esper.Alias("geslo", esper.EnumLastOf[esper.Event](getEvents)),
			).Query(esper.StatementName("s0")))
		} else {
			triggerPlan, err = env.Build(esper.OnEvent(trigger).SelectFromTableWhere("MyTable", esper.Literal(true),
				esper.Alias("nmr", esper.Method[esper.SortedAccessValue[int, esper.Event]](sortcol, "NavigableMapReference")),
			).Query(esper.StatementName("s0")))
		}
	case "submap-events-between":
		trigger := esper.From[resultsetAggregateSortedTableAccessSubmapEvent](env, "MySubmapEvent")
		fromKey := esper.Field[resultsetAggregateSortedTableAccessSubmapEvent, int]("fromKey")
		fromInclusive := esper.Field[resultsetAggregateSortedTableAccessSubmapEvent, bool]("fromInclusive")
		toKey := esper.Field[resultsetAggregateSortedTableAccessSubmapEvent, int]("toKey")
		toInclusive := esper.Field[resultsetAggregateSortedTableAccessSubmapEvent, bool]("toInclusive")
		eventsBetween := esper.Method[[]esper.Event](sortcol, "EventsBetween", fromKey, fromInclusive, toKey, toInclusive)
		submap := esper.Method[esper.SortedAccessValue[int, esper.Event]](sortcol, "SubMap", fromKey, fromInclusive, toKey, toInclusive)
		triggerPlan, err = env.Build(esper.OnEvent(trigger).SelectFromTableWhere("MyTable", esper.Literal(true),
			esper.Alias("eb", eventsBetween),
			esper.Alias("eblastof", esper.EnumLastOf[esper.Event](eventsBetween)),
			esper.Alias("sm", submap),
		).Query(esper.StatementName("s0")))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported case %q", caseName)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(resultsetAggregateSortedTableAccessJavaRuntimeIDs[index]),
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
	trace, err := compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetAggregateSortedTableAccessPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown %s statement %q", resultsetAggregateSortedTableAccessID, name)
		}
		return statement, nil
	})
	if err != nil {
		return compat.Trace{}, err
	}
	trace = normalizeResultSetAggregateSortedTableAccessTrace(trace)
	if err := validateResultSetAggregateSortedTableAccessTrace(trace, index, caseName); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func decodeResultSetAggregateSortedTableAccessPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		value, err := decodeResultSetAggregateSortedTableAccessBeanPayload(step.Payload)
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedTableAccessBean{TheString: value.name, IntPrimitive: value.key}, nil
	case "SupportBean_S0":
		payload, err := decodeResultSetAggregateSortedTableAccessJSONObject(step.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if err := requireResultSetAggregateSortedTableAccessFields(payload, "id"); err != nil {
			return nil, err
		}
		id, err := decodeResultSetAggregateSortedTableAccessInt(payload["id"], "id")
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedTableAccessTrigger{ID: id}, nil
	case "MySubmapEvent":
		payload, err := decodeResultSetAggregateSortedTableAccessJSONObject(step.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode MySubmapEvent: %w", err)
		}
		if err := requireResultSetAggregateSortedTableAccessFields(payload, "fromKey", "fromInclusive", "toKey", "toInclusive"); err != nil {
			return nil, err
		}
		fromKey, err := decodeResultSetAggregateSortedTableAccessInt(payload["fromKey"], "fromKey")
		if err != nil {
			return nil, err
		}
		fromInclusive, err := decodeResultSetAggregateSortedTableAccessBool(payload["fromInclusive"], "fromInclusive")
		if err != nil {
			return nil, err
		}
		toKey, err := decodeResultSetAggregateSortedTableAccessInt(payload["toKey"], "toKey")
		if err != nil {
			return nil, err
		}
		toInclusive, err := decodeResultSetAggregateSortedTableAccessBool(payload["toInclusive"], "toInclusive")
		if err != nil {
			return nil, err
		}
		return resultsetAggregateSortedTableAccessSubmapEvent{FromKey: fromKey, FromInclusive: fromInclusive, ToKey: toKey, ToInclusive: toInclusive}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateSortedTableAccessID, step.EventType)
	}
}

func decodeResultSetAggregateSortedTableAccessBeanPayload(raw json.RawMessage) (resultsetAggregateSortedTableAccessExpectedEvent, error) {
	payload, err := decodeResultSetAggregateSortedTableAccessJSONObject(raw)
	if err != nil {
		return resultsetAggregateSortedTableAccessExpectedEvent{}, fmt.Errorf("payload must be an object: %w", err)
	}
	if err := requireResultSetAggregateSortedTableAccessFields(payload, "theString", "intPrimitive"); err != nil {
		return resultsetAggregateSortedTableAccessExpectedEvent{}, err
	}
	name, err := decodeResultSetAggregateSortedTableAccessString(payload["theString"], "theString")
	if err != nil {
		return resultsetAggregateSortedTableAccessExpectedEvent{}, err
	}
	key, err := decodeResultSetAggregateSortedTableAccessInt(payload["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultsetAggregateSortedTableAccessExpectedEvent{}, err
	}
	return resultsetAggregateSortedTableAccessExpectedEvent{name: name, key: key}, nil
}

func validateResultSetAggregateSortedTableAccessScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateSortedTableAccessID {
		return fmt.Errorf("%s scenario has unsupported id %q", resultsetAggregateSortedTableAccessID, scenario.ID)
	}
	stepIndex := 0
	for caseIndex, caseName := range resultsetAggregateSortedTableAccessCases {
		if stepIndex >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario is missing case %q", resultsetAggregateSortedTableAccessID, caseName)
		}
		marker := scenario.Steps[stepIndex]
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("%s scenario case marker %d is not pinned to %q", resultsetAggregateSortedTableAccessID, caseIndex, caseName)
		}
		stepIndex++
		for seedIndex, expected := range resultsetAggregateSortedTableAccessSeeds {
			if stepIndex >= len(scenario.Steps) {
				return fmt.Errorf("%s case %q is missing seed %d", resultsetAggregateSortedTableAccessID, caseName, seedIndex)
			}
			step := scenario.Steps[stepIndex]
			if step.Op != "send" || step.EventType != "SupportBean" || step.Case != "" {
				return fmt.Errorf("%s case %q seed %d is not a SupportBean send", resultsetAggregateSortedTableAccessID, caseName, seedIndex)
			}
			actual, err := decodeResultSetAggregateSortedTableAccessBeanPayload(step.Payload)
			if err != nil {
				return fmt.Errorf("%s case %q seed %d: %w", resultsetAggregateSortedTableAccessID, caseName, seedIndex, err)
			}
			if actual != expected {
				return fmt.Errorf("%s case %q seed %d = %#v, want %#v", resultsetAggregateSortedTableAccessID, caseName, seedIndex, actual, expected)
			}
			stepIndex++
		}
		switch caseIndex {
		case 0:
			for id := 0; id < 12; id++ {
				if err := validateResultSetAggregateSortedTableAccessTriggerStep(scenario.Steps, &stepIndex, id); err != nil {
					return fmt.Errorf("%s case %q: %w", resultsetAggregateSortedTableAccessID, caseName, err)
				}
			}
		case 1:
			for fromKey := 0; fromKey < 12; fromKey++ {
				for toKey := fromKey; toKey < 12; toKey++ {
					for _, fromInclusive := range []bool{false, true} {
						for _, toInclusive := range []bool{false, true} {
							if err := validateResultSetAggregateSortedTableAccessSubmapStep(scenario.Steps, &stepIndex, fromKey, fromInclusive, toKey, toInclusive); err != nil {
								return fmt.Errorf("%s case %q: %w", resultsetAggregateSortedTableAccessID, caseName, err)
							}
						}
					}
				}
			}
		case 2:
			if err := validateResultSetAggregateSortedTableAccessTriggerStep(scenario.Steps, &stepIndex, -1); err != nil {
				return fmt.Errorf("%s case %q: %w", resultsetAggregateSortedTableAccessID, caseName, err)
			}
		}
	}
	if stepIndex != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetAggregateSortedTableAccessID)
	}
	return nil
}

func validateResultSetAggregateSortedTableAccessTriggerStep(steps []compat.Step, index *int, id int) error {
	if index == nil || *index >= len(steps) {
		return fmt.Errorf("missing SupportBean_S0 trigger")
	}
	step := steps[*index]
	if step.Op != "send" || step.EventType != "SupportBean_S0" || step.Case != "" {
		return fmt.Errorf("step %d is not a SupportBean_S0 trigger", *index)
	}
	payload, err := decodeResultSetAggregateSortedTableAccessJSONObject(step.Payload)
	if err != nil {
		return fmt.Errorf("trigger payload: %w", err)
	}
	if err := requireResultSetAggregateSortedTableAccessFields(payload, "id"); err != nil {
		return err
	}
	actual, err := decodeResultSetAggregateSortedTableAccessInt(payload["id"], "id")
	if err != nil {
		return err
	}
	if actual != id {
		return fmt.Errorf("trigger id = %d, want %d", actual, id)
	}
	*index++
	return nil
}

func validateResultSetAggregateSortedTableAccessSubmapStep(steps []compat.Step, index *int, fromKey int, fromInclusive bool, toKey int, toInclusive bool) error {
	if index == nil || *index >= len(steps) {
		return fmt.Errorf("missing MySubmapEvent trigger")
	}
	step := steps[*index]
	if step.Op != "send" || step.EventType != "MySubmapEvent" || step.Case != "" {
		return fmt.Errorf("step %d is not a MySubmapEvent trigger", *index)
	}
	payload, err := decodeResultSetAggregateSortedTableAccessJSONObject(step.Payload)
	if err != nil {
		return fmt.Errorf("submap payload: %w", err)
	}
	if err := requireResultSetAggregateSortedTableAccessFields(payload, "fromKey", "fromInclusive", "toKey", "toInclusive"); err != nil {
		return err
	}
	actualFrom, err := decodeResultSetAggregateSortedTableAccessInt(payload["fromKey"], "fromKey")
	if err != nil {
		return err
	}
	actualFromInclusive, err := decodeResultSetAggregateSortedTableAccessBool(payload["fromInclusive"], "fromInclusive")
	if err != nil {
		return err
	}
	actualTo, err := decodeResultSetAggregateSortedTableAccessInt(payload["toKey"], "toKey")
	if err != nil {
		return err
	}
	actualToInclusive, err := decodeResultSetAggregateSortedTableAccessBool(payload["toInclusive"], "toInclusive")
	if err != nil {
		return err
	}
	if actualFrom != fromKey || actualFromInclusive != fromInclusive || actualTo != toKey || actualToInclusive != toInclusive {
		return fmt.Errorf("submap trigger = (%d,%t,%d,%t), want (%d,%t,%d,%t)", actualFrom, actualFromInclusive, actualTo, actualToInclusive, fromKey, fromInclusive, toKey, toInclusive)
	}
	*index++
	return nil
}

func validateResultSetAggregateSortedTableAccessTrace(trace compat.Trace, index int, caseName string) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultsetAggregateSortedTableAccessID {
		return fmt.Errorf("%s trace identity is not pinned", resultsetAggregateSortedTableAccessID)
	}
	for recordIndex, record := range trace.Records {
		if record.Case != caseName || record.Operation != "listener" || record.Statement != "s0" || record.Sequence != uint64(recordIndex+1) ||
			record.Time != "1970-01-01T00:00:00Z" || len(record.New) != 1 || len(record.Old) != 0 {
			return fmt.Errorf("%s case %q trace record %d metadata is not pinned", resultsetAggregateSortedTableAccessID, caseName, recordIndex)
		}
	}
	var expected []map[string]any
	switch index {
	case 0:
		expected = resultsetAggregateSortedTableAccessExpectedGetRows()
	case 1:
		expected = resultsetAggregateSortedTableAccessExpectedSubmapRows()
	case 2:
		expected = []map[string]any{{"nmr": resultsetAggregateSortedTableAccessExpectedMap(resultsetAggregateSortedTableAccessSeeds)}}
	default:
		return fmt.Errorf("unsupported sorted table access case index %d", index)
	}
	if len(trace.Records) != len(expected) {
		return fmt.Errorf("%s case %q trace has %d records, want %d", resultsetAggregateSortedTableAccessID, caseName, len(trace.Records), len(expected))
	}
	for recordIndex, want := range expected {
		fields := trace.Records[recordIndex].New[0].Fields
		if len(fields) != len(want) {
			return fmt.Errorf("%s case %q record %d has %d fields, want %d", resultsetAggregateSortedTableAccessID, caseName, recordIndex, len(fields), len(want))
		}
		for name, expectedValue := range want {
			if actual := fields[name]; !reflect.DeepEqual(actual, expectedValue) {
				return fmt.Errorf("%s case %q record %d field %q = %#v, want %#v", resultsetAggregateSortedTableAccessID, caseName, recordIndex, name, actual, expectedValue)
			}
		}
	}
	return nil
}

func resultsetAggregateSortedTableAccessExpectedGetRows() []map[string]any {
	rows := make([]map[string]any, 0, 12)
	seeds := append([]resultsetAggregateSortedTableAccessSeed(nil), resultsetAggregateSortedTableAccessSeeds...)
	for id := 0; id < 12; id++ {
		bucket := resultsetAggregateSortedTableAccessBucket(seeds, id)
		row := map[string]any{
			"ge":    resultsetAggregateSortedTableAccessNull(),
			"ges":   resultsetAggregateSortedTableAccessNull(),
			"ck":    false,
			"cnte":  int64(len(seeds)),
			"cntk":  int64(resultsetAggregateSortedTableAccessKeyCount(seeds)),
			"geid":  resultsetAggregateSortedTableAccessNull(),
			"gefo":  resultsetAggregateSortedTableAccessNull(),
			"geslo": resultsetAggregateSortedTableAccessNull(),
		}
		if len(bucket) != 0 {
			row["ge"] = resultsetAggregateSortedTableAccessEventRow(bucket[0])
			row["ges"] = resultsetAggregateSortedTableAccessEventRows(bucket)
			row["ck"] = true
			row["geid"] = bucket[0].name
			row["gefo"] = resultsetAggregateSortedTableAccessEventRow(bucket[0])
			row["geslo"] = resultsetAggregateSortedTableAccessEventRow(bucket[len(bucket)-1])
		}
		rows = append(rows, row)
	}
	return rows
}

func resultsetAggregateSortedTableAccessExpectedSubmapRows() []map[string]any {
	rows := make([]map[string]any, 0, 312)
	seeds := append([]resultsetAggregateSortedTableAccessSeed(nil), resultsetAggregateSortedTableAccessSeeds...)
	for fromKey := 0; fromKey < 12; fromKey++ {
		for toKey := fromKey; toKey < 12; toKey++ {
			for _, fromInclusive := range []bool{false, true} {
				for _, toInclusive := range []bool{false, true} {
					selected := resultsetAggregateSortedTableAccessRange(seeds, fromKey, fromInclusive, toKey, toInclusive)
					row := map[string]any{
						"eb":       resultsetAggregateSortedTableAccessEventRows(selected),
						"eblastof": resultsetAggregateSortedTableAccessNull(),
						"sm":       resultsetAggregateSortedTableAccessExpectedMap(selected),
					}
					if len(selected) != 0 {
						row["eblastof"] = resultsetAggregateSortedTableAccessEventRow(selected[len(selected)-1])
					}
					rows = append(rows, row)
				}
			}
		}
	}
	return rows
}

func resultsetAggregateSortedTableAccessBucket(seeds []resultsetAggregateSortedTableAccessSeed, key int) []resultsetAggregateSortedTableAccessSeed {
	bucket := make([]resultsetAggregateSortedTableAccessSeed, 0, len(seeds))
	for _, seed := range seeds {
		if seed.key == key {
			bucket = append(bucket, seed)
		}
	}
	return bucket
}

func resultsetAggregateSortedTableAccessRange(seeds []resultsetAggregateSortedTableAccessSeed, fromKey int, fromInclusive bool, toKey int, toInclusive bool) []resultsetAggregateSortedTableAccessSeed {
	selected := make([]resultsetAggregateSortedTableAccessSeed, 0, len(seeds))
	for _, seed := range seeds {
		if seed.key < fromKey || (seed.key == fromKey && !fromInclusive) || seed.key > toKey || (seed.key == toKey && !toInclusive) {
			continue
		}
		selected = append(selected, seed)
	}
	return selected
}

func resultsetAggregateSortedTableAccessKeyCount(seeds []resultsetAggregateSortedTableAccessSeed) int {
	keys := make(map[int]struct{}, len(seeds))
	for _, seed := range seeds {
		keys[seed.key] = struct{}{}
	}
	return len(keys)
}

func resultsetAggregateSortedTableAccessEventRow(seed resultsetAggregateSortedTableAccessSeed) map[string]any {
	return map[string]any{"kind": "row", "fields": map[string]any{"intPrimitive": seed.key, "theString": seed.name}}
}

func resultsetAggregateSortedTableAccessEventRows(seeds []resultsetAggregateSortedTableAccessSeed) []any {
	rows := make([]any, 0, len(seeds))
	for _, seed := range seeds {
		rows = append(rows, resultsetAggregateSortedTableAccessEventRow(seed))
	}
	return rows
}

func resultsetAggregateSortedTableAccessExpectedMap(seeds []resultsetAggregateSortedTableAccessSeed) map[string]any {
	fields := make(map[string]any)
	keys := make([]int, 0, len(seeds))
	for _, seed := range seeds {
		found := false
		for _, key := range keys {
			if key == seed.key {
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, seed.key)
		}
	}
	sort.Ints(keys)
	for _, key := range keys {
		fields[strconv.Itoa(key)] = resultsetAggregateSortedTableAccessEventRows(resultsetAggregateSortedTableAccessBucket(seeds, key))
	}
	return map[string]any{"kind": "row", "fields": fields}
}

func resultsetAggregateSortedTableAccessNull() map[string]any {
	return map[string]any{"state": "null"}
}

func normalizeResultSetAggregateSortedTableAccessTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for rowIndex := range record.New {
			for name, value := range record.New[rowIndex].Fields {
				record.New[rowIndex].Fields[name] = normalizeResultSetAggregateSortedTableAccessValue(value)
			}
		}
		for rowIndex := range record.Old {
			for name, value := range record.Old[rowIndex].Fields {
				record.Old[rowIndex].Fields[name] = normalizeResultSetAggregateSortedTableAccessValue(value)
			}
		}
	}
	return trace
}

func normalizeResultSetAggregateSortedTableAccessValue(value any) any {
	switch typed := value.(type) {
	case esper.SortedAccessValue[int, esper.Event]:
		return normalizeResultSetAggregateSortedTableAccessMap(typed)
	case []esper.Event:
		rows := make([]any, 0, len(typed))
		for _, event := range typed {
			rows = append(rows, normalizeResultSetAggregateSortedTableAccessEvent(event))
		}
		return rows
	case esper.Event:
		return normalizeResultSetAggregateSortedTableAccessEvent(typed)
	case []any:
		for index := range typed {
			typed[index] = normalizeResultSetAggregateSortedTableAccessValue(typed[index])
		}
		return typed
	case map[string]any:
		if typed["kind"] == "row" || typed["state"] != nil {
			return typed
		}
		for name, nested := range typed {
			typed[name] = normalizeResultSetAggregateSortedTableAccessValue(nested)
		}
		return typed
	default:
		return value
	}
}

func normalizeResultSetAggregateSortedTableAccessEvent(event esper.Event) map[string]any {
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

func normalizeResultSetAggregateSortedTableAccessMap(access esper.SortedAccessValue[int, esper.Event]) map[string]any {
	fields := make(map[string]any)
	for _, entry := range access.Entries() {
		rows := make([]any, 0, len(entry.Values))
		for _, event := range entry.Values {
			rows = append(rows, normalizeResultSetAggregateSortedTableAccessEvent(event))
		}
		fields[strconv.Itoa(entry.Key)] = rows
	}
	return map[string]any{"kind": "row", "fields": fields}
}

func decodeResultSetAggregateSortedTableAccessJSONObject(raw []byte) (map[string]json.RawMessage, error) {
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

func decodeResultSetAggregateSortedTableAccessJSONArray(raw json.RawMessage) ([]json.RawMessage, error) {
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

func requireResultSetAggregateSortedTableAccessFields(object map[string]json.RawMessage, names ...string) error {
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

func decodeResultSetAggregateSortedTableAccessString(raw json.RawMessage, name string) (string, error) {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return "", fmt.Errorf("%s must be a string", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return value, nil
}

func decodeResultSetAggregateSortedTableAccessBool(raw json.RawMessage, name string) (bool, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return value, nil
}

func decodeResultSetAggregateSortedTableAccessInt(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	number, ok := token.(json.Number)
	if !ok {
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

func validateResultSetAggregateSortedTableAccessStringArray(raw json.RawMessage, expected []string, name string) error {
	values, err := decodeResultSetAggregateSortedTableAccessJSONArray(raw)
	if err != nil {
		return fmt.Errorf("%s must be an array: %w", name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index, rawValue := range values {
		value, err := decodeResultSetAggregateSortedTableAccessString(rawValue, name)
		if err != nil || value != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func rejectResultSetAggregateSortedTableAccessDuplicateKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkResultSetAggregateSortedTableAccessJSON(decoder); err != nil {
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

func walkResultSetAggregateSortedTableAccessJSON(decoder *json.Decoder) error {
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
			if err := walkResultSetAggregateSortedTableAccessJSON(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("JSON object is not closed")
		}
	case json.Delim('['):
		for decoder.More() {
			if err := walkResultSetAggregateSortedTableAccessJSON(decoder); err != nil {
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
