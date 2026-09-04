package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOrderbyMultiDeliveryID          = "resultset-orderby-multi-delivery"
	resultsetOrderbyMultiDeliveryDescription = "ResultSetOrderByMultiDelivery: pattern and grouped-time delivery batching with order-by over each delivered batch."
	resultsetOrderbyMultiDeliveryJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOrderbyMultiDeliverySource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderBySimple.java"
)

var (
	resultsetOrderbyMultiDeliveryJavaSources = []string{
		resultsetOrderbyMultiDeliverySource,
	}
	resultsetOrderbyMultiDeliveryJavaRuntimeIDs = []string{
		"java-runtime-1c3d57ae9e8d4bca739f",
	}
	resultsetOrderbyMultiDeliveryJavaExecutions = []string{
		"ResultSetOrderByMultiDelivery",
	}
	resultsetOrderbyMultiDeliveryJavaStaticIDs = []string{
		"java-5151c7da40772952b49a",
	}
	resultsetOrderbyMultiDeliveryCases = []string{
		"multi-delivery-pattern",
		"multi-delivery-pattern-output-limit",
		"multi-delivery-groupwin-time",
	}
	resultsetOrderbyMultiDeliveryOrdinals = []int{0, 0, 0}
	resultsetOrderbyMultiDeliveryEPLs     = []string{
		"@name('s0') select a.theString from pattern [every a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%')] order by a.theString desc",
		"@name('s0') select a.theString from pattern [every a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%')] output every 3 events order by a.theString desc",
		"@name('s0') select rstream theString from SupportBean#groupwin(theString)#time(10) order by theString desc",
	}
)

type resultsetOrderbyMultiDeliveryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var resultsetOrderbyMultiDeliveryPatternOne = []resultsetOrderbyMultiDeliveryBean{
	{TheString: "A1", IntPrimitive: 1},
	{TheString: "A2", IntPrimitive: 2},
	{TheString: "B", IntPrimitive: 3},
}
var resultsetOrderbyMultiDeliveryPatternTwo = []resultsetOrderbyMultiDeliveryBean{
	{TheString: "A1", IntPrimitive: 1},
	{TheString: "A2", IntPrimitive: 2},
	{TheString: "A3", IntPrimitive: 3},
	{TheString: "B", IntPrimitive: 3},
}
var resultsetOrderbyMultiDeliveryGroupTime = []resultsetOrderbyMultiDeliveryBean{
	{TheString: "A1", IntPrimitive: 1},
	{TheString: "A2", IntPrimitive: 1},
}

func loadResultsetOrderbyMultiDeliveryScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOrderbyMultiDeliveryID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOrderbyMultiDeliveryID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbyMultiDeliveryID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOrderbyMultiDeliveryID, err)
	}
	if err := requireResultsetOrderbyMultiDeliveryFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOrderbyMultiDeliveryID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOrderbyMultiDeliveryID ||
		metadata.Description != resultsetOrderbyMultiDeliveryDescription ||
		metadata.JavaCommit != resultsetOrderbyMultiDeliveryJavaCommit ||
		metadata.JavaSource != resultsetOrderbyMultiDeliverySource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOrderbyMultiDeliveryID)
	}
	if err := validateResultsetOrderbyMultiDeliveryStringArray(root["javaRuntimes"], resultsetOrderbyMultiDeliveryJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyMultiDeliveryStringArray(root["javaNames"], resultsetOrderbyMultiDeliveryJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyMultiDeliveryStringArray(root["javaStaticIds"], resultsetOrderbyMultiDeliveryJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOrderbyMultiDeliveryStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOrderbyMultiDeliveryCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetOrderbyMultiDeliveryID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOrderbyMultiDeliveryFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != resultsetOrderbyMultiDeliveryCases[index] ||
			definition.Ordinal != resultsetOrderbyMultiDeliveryOrdinals[index] ||
			definition.RuntimeID != resultsetOrderbyMultiDeliveryJavaRuntimeIDs[0] ||
			definition.ExecutionName != resultsetOrderbyMultiDeliveryJavaExecutions[0] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOrderbyMultiDeliveryEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOrderbyMultiDeliveryID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 14 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly fourteen steps", resultsetOrderbyMultiDeliveryID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireResultsetOrderbyMultiDeliveryFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOrderbyMultiDeliveryFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOrderbyMultiDeliveryPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "advance-time":
			if err := requireResultsetOrderbyMultiDeliveryFields(object, "op", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advance-time is not pinned", index)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOrderbyMultiDeliveryScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOrderbyMultiDeliveryScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOrderbyMultiDeliveryID || len(scenario.Steps) != 14 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOrderbyMultiDeliveryID)
	}
	index := 0
	for caseIndex := 0; caseIndex < 2; caseIndex++ {
		if err := validateResultsetOrderbyMultiDeliveryCaseMarker(scenario.Steps[index], resultsetOrderbyMultiDeliveryCases[caseIndex]); err != nil {
			return fmt.Errorf("case %d marker: %w", caseIndex, err)
		}
		index++
		beanSteps := resultsetOrderbyMultiDeliveryPatternOne
		if caseIndex == 1 {
			beanSteps = resultsetOrderbyMultiDeliveryPatternTwo
		}
		for _, expected := range beanSteps {
			if err := validateResultsetOrderbyMultiDeliveryBeanStep(scenario.Steps[index], expected); err != nil {
				return fmt.Errorf("case %d bean step: %w", caseIndex, err)
			}
			index++
		}
	}
	if err := validateResultsetOrderbyMultiDeliveryCaseMarker(scenario.Steps[index], resultsetOrderbyMultiDeliveryCases[2]); err != nil {
		return fmt.Errorf("group-time marker: %w", err)
	}
	index++
	if err := validateResultsetOrderbyMultiDeliveryAdvanceStep(scenario.Steps[index], "1970-01-01T00:00:01Z"); err != nil {
		return fmt.Errorf("group-time first advance: %w", err)
	}
	index++
	for _, expected := range resultsetOrderbyMultiDeliveryGroupTime {
		if err := validateResultsetOrderbyMultiDeliveryBeanStep(scenario.Steps[index], expected); err != nil {
			return fmt.Errorf("group-time bean step: %w", err)
		}
		index++
	}
	if err := validateResultsetOrderbyMultiDeliveryAdvanceStep(scenario.Steps[index], "1970-01-01T00:00:11Z"); err != nil {
		return fmt.Errorf("group-time final advance: %w", err)
	}
	index++
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOrderbyMultiDeliveryID)
	}
	return nil
}

func validateResultsetOrderbyMultiDeliveryCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOrderbyMultiDeliveryBeanStep(step compat.Step, expected resultsetOrderbyMultiDeliveryBean) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean" {
		return fmt.Errorf("must send SupportBean")
	}
	bean, err := decodeResultsetOrderbyMultiDeliveryBean(step)
	if err != nil {
		return err
	}
	if bean.TheString != expected.TheString || bean.IntPrimitive != expected.IntPrimitive {
		return fmt.Errorf("bean payload is not pinned")
	}
	return nil
}

func validateResultsetOrderbyMultiDeliveryAdvanceStep(step compat.Step, expected string) error {
	if step.Op != "advance-time" || step.Case != "" || step.At != expected {
		return fmt.Errorf("must advance time to %q", expected)
	}
	return nil
}

func runResultsetOrderbyMultiDeliveryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOrderbyMultiDeliveryScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range resultsetOrderbyMultiDeliveryCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultsetOrderbyMultiDeliveryCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOrderbyMultiDeliveryID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOrderbyMultiDeliveryID, scenario.ID)
	}
	return trace, nil
}

func runResultsetOrderbyMultiDeliveryCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOrderbyMultiDeliveryBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[resultsetOrderbyMultiDeliveryBean, string]("theString")
	var query esper.Query
	switch caseName {
	case "multi-delivery-pattern":
		pattern := esper.PatternFrom(
			esper.From[resultsetOrderbyMultiDeliveryBean](env, "SupportBean"),
			"a",
			esper.LikeOf(theString, esper.Literal("A%")),
		).Every().FollowedBy(
			"b",
			esper.LikeOf(theString, esper.Literal("B%")),
		)
		query = pattern.Select(
			esper.Alias("a.theString", esper.TagField[string]("a", "theString")),
		).Query(
			esper.StatementName("s0"),
			esper.OrderBy(esper.Descending(esper.ResultField[string]("a.theString"))),
		)
	case "multi-delivery-pattern-output-limit":
		pattern := esper.PatternFrom(
			esper.From[resultsetOrderbyMultiDeliveryBean](env, "SupportBean"),
			"a",
			esper.LikeOf(theString, esper.Literal("A%")),
		).Every().FollowedBy(
			"b",
			esper.LikeOf(theString, esper.Literal("B%")),
		)
		query = pattern.Select(
			esper.Alias("a.theString", esper.TagField[string]("a", "theString")),
		).Query(
			esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryEvents(3)),
			esper.OrderBy(esper.Descending(esper.ResultField[string]("a.theString"))),
		)
	case "multi-delivery-groupwin-time":
		grouped := esper.From[resultsetOrderbyMultiDeliveryBean](env, "SupportBean").
			Window(esper.GroupWindow(theString, esper.TimeWindow(10*time.Second)))
		query = esper.Select(grouped,
			esper.Alias("theString", theString),
		).Query(
			esper.StatementName("s0"),
			esper.WithRemoveStreamOnly(),
			esper.OrderBy(esper.Descending(esper.ResultField[string]("theString"))),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported %s case %q", resultsetOrderbyMultiDeliveryID, caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithRuntimeURI(resultsetOrderbyMultiDeliveryRuntimeID(caseName)))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOrderbyMultiDeliveryID, caseName, len(statements))
	}
	statement := statements[0]
	return compat.ReplayWithStatements(ctx, engine, statement, scenario,
		decodeResultsetOrderbyMultiDeliveryPayload,
		func(name string) (*esper.Statement, error) {
			if name != statement.Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOrderbyMultiDeliveryID, name)
			}
			return statement, nil
		})
}

func decodeResultsetOrderbyMultiDeliveryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		return decodeResultsetOrderbyMultiDeliveryBean(step)
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOrderbyMultiDeliveryID, step.EventType)
	}
}

func decodeResultsetOrderbyMultiDeliveryBean(step compat.Step) (resultsetOrderbyMultiDeliveryBean, error) {
	if step.EventType != "SupportBean" {
		return resultsetOrderbyMultiDeliveryBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultsetOrderbyMultiDeliveryBean{}, err
	}
	if err := requireResultsetOrderbyMultiDeliveryFields(fields, "theString", "intPrimitive"); err != nil {
		return resultsetOrderbyMultiDeliveryBean{}, err
	}
	var theString string
	if err := json.Unmarshal(fields["theString"], &theString); err != nil {
		return resultsetOrderbyMultiDeliveryBean{}, fmt.Errorf("theString must be a string")
	}
	var intPrimitive int
	decoder := json.NewDecoder(bytes.NewReader(fields["intPrimitive"]))
	if err := decoder.Decode(&intPrimitive); err != nil {
		return resultsetOrderbyMultiDeliveryBean{}, fmt.Errorf("intPrimitive must be an integer JSON number")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return resultsetOrderbyMultiDeliveryBean{}, fmt.Errorf("intPrimitive must be an integer JSON number")
	}
	return resultsetOrderbyMultiDeliveryBean{TheString: theString, IntPrimitive: intPrimitive}, nil
}

func requireResultsetOrderbyMultiDeliveryFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetOrderbyMultiDeliveryStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

func resultsetOrderbyMultiDeliveryRuntimeID(caseName string) string {
	_ = caseName
	return resultsetOrderbyMultiDeliveryJavaRuntimeIDs[0]
}
