package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetOutputLimitParameterizedContextID          = "resultset-output-limit-parameterized-context"
	resultsetOutputLimitParameterizedContextDescription = "ResultSetOutputLimitParameterizedByContext: a context started by SupportScheduleSimpleEvent parameterizes the output-last crontab from the start event; one S0 event joins the partition and exactly one new-only single-row count callback fires at the scheduled 10:15 instant."
	resultsetOutputLimitParameterizedContextJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitParameterizedContextSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitParameterizedByContext.java"
)

var (
	resultsetOutputLimitParameterizedContextJavaSources = []string{
		resultsetOutputLimitParameterizedContextSource,
	}
	resultsetOutputLimitParameterizedContextJavaRuntimeIDs = []string{
		"java-runtime-3fc747eca88c34b6ab3a",
	}
	resultsetOutputLimitParameterizedContextJavaExecutions = []string{
		"ResultSetOutputLimitParameterizedByContext",
	}
	resultsetOutputLimitParameterizedContextJavaStaticIDs = []string{
		"java-3bf9eee607100a7666ab",
	}
	resultsetOutputLimitParameterizedContextCases = []string{
		"context-cron",
	}
	resultsetOutputLimitParameterizedContextOrdinals = []int{0}
	resultsetOutputLimitParameterizedContextEPLs     = []string{
		"@name('ctx') create context MyCtx start SupportScheduleSimpleEvent as sse;\n@name('s0') context MyCtx\nselect count(*) as c \nfrom SupportBean_S0\noutput last at(context.sse.atminute, context.sse.athour, *, *, *, *) and when terminated\n",
	}
)

type resultsetOutputLimitParameterizedScheduleEvent struct {
	AtHour   int `esper:"athour"`
	AtMinute int `esper:"atminute"`
}

type resultsetOutputLimitParameterizedS0 struct {
	ID int `esper:"id"`
}

func loadResultsetOutputLimitParameterizedContextScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitParameterizedContextID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitParameterizedContextID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitParameterizedContextID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitParameterizedContextID, err)
	}
	if err := requireResultsetOutputLimitParameterizedContextFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitParameterizedContextID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitParameterizedContextID ||
		metadata.Description != resultsetOutputLimitParameterizedContextDescription ||
		metadata.JavaCommit != resultsetOutputLimitParameterizedContextJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitParameterizedContextSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitParameterizedContextID)
	}
	if err := validateResultsetOutputLimitParameterizedContextStringArray(root["javaRuntimes"], resultsetOutputLimitParameterizedContextJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitParameterizedContextStringArray(root["javaNames"], resultsetOutputLimitParameterizedContextJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitParameterizedContextStringArray(root["javaStaticIds"], resultsetOutputLimitParameterizedContextJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitParameterizedContextStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitParameterizedContextCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetOutputLimitParameterizedContextID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetOutputLimitParameterizedContextFields(object,
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
		if definition.Case != resultsetOutputLimitParameterizedContextCases[index] ||
			definition.Ordinal != resultsetOutputLimitParameterizedContextOrdinals[index] ||
			definition.RuntimeID != resultsetOutputLimitParameterizedContextJavaRuntimeIDs[0] ||
			definition.ExecutionName != resultsetOutputLimitParameterizedContextJavaExecutions[0] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != resultsetOutputLimitParameterizedContextEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitParameterizedContextID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 6 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly six steps", resultsetOutputLimitParameterizedContextID)
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
			if err := requireResultsetOutputLimitParameterizedContextFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOutputLimitParameterizedContextFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOutputLimitParameterizedContextPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "advance-time":
			if err := requireResultsetOutputLimitParameterizedContextFields(object, "op", "at"); err != nil {
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
	if err := validateResultsetOutputLimitParameterizedContextScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitParameterizedContextScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitParameterizedContextID || len(scenario.Steps) != 6 {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitParameterizedContextID)
	}
	index := 0
	if err := validateResultsetOutputLimitParameterizedContextCaseMarker(scenario.Steps[index], resultsetOutputLimitParameterizedContextCases[0]); err != nil {
		return fmt.Errorf("context-cron marker: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitParameterizedContextAdvanceStep(scenario.Steps[index], "2002-05-01T09:00:00.000Z"); err != nil {
		return fmt.Errorf("context-cron start advance: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitParameterizedContextScheduleStep(scenario.Steps[index], 10, 15); err != nil {
		return fmt.Errorf("context-cron schedule event: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitParameterizedContextS0Step(scenario.Steps[index]); err != nil {
		return fmt.Errorf("context-cron s0 event: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitParameterizedContextAdvanceStep(scenario.Steps[index], "2002-05-01T10:14:59.000Z"); err != nil {
		return fmt.Errorf("context-cron pre-fire advance: %w", err)
	}
	index++
	if err := validateResultsetOutputLimitParameterizedContextAdvanceStep(scenario.Steps[index], "2002-05-01T10:15:00.000Z"); err != nil {
		return fmt.Errorf("context-cron fire advance: %w", err)
	}
	index++
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitParameterizedContextID)
	}
	return nil
}

func validateResultsetOutputLimitParameterizedContextCaseMarker(step compat.Step, expected string) error {
	if step.Op != "case" || step.Case != expected {
		return fmt.Errorf("must start case %q", expected)
	}
	return nil
}

func validateResultsetOutputLimitParameterizedContextAdvanceStep(step compat.Step, expected string) error {
	if step.Op != "advance-time" || step.Case != "" || step.At != expected {
		return fmt.Errorf("must advance time to %q", expected)
	}
	return nil
}

func validateResultsetOutputLimitParameterizedContextScheduleStep(step compat.Step, atHour, atMinute int) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportScheduleSimpleEvent" {
		return fmt.Errorf("must send SupportScheduleSimpleEvent")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	if err := requireResultsetOutputLimitParameterizedContextFields(fields, "athour", "atminute"); err != nil {
		return err
	}
	var event resultsetOutputLimitParameterizedScheduleEvent
	if err := json.Unmarshal(step.Payload, &event); err != nil {
		return fmt.Errorf("decode SupportScheduleSimpleEvent: %w", err)
	}
	if event.AtHour != atHour || event.AtMinute != atMinute {
		return fmt.Errorf("schedule payload is not pinned")
	}
	return nil
}

func validateResultsetOutputLimitParameterizedContextS0Step(step compat.Step) error {
	if step.Op != "send" || step.Case != "" || step.EventType != "SupportBean_S0" {
		return fmt.Errorf("must send SupportBean_S0")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	if err := requireResultsetOutputLimitParameterizedContextFields(fields, "id"); err != nil {
		return err
	}
	var event resultsetOutputLimitParameterizedS0
	if err := json.Unmarshal(step.Payload, &event); err != nil {
		return fmt.Errorf("decode SupportBean_S0: %w", err)
	}
	if event.ID != 0 {
		return fmt.Errorf("s0 payload is not pinned")
	}
	return nil
}

func runResultsetOutputLimitParameterizedContextScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitParameterizedContextScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	caseName := resultsetOutputLimitParameterizedContextCases[0]
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", resultsetOutputLimitParameterizedContextID, scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	caseTrace, err := runResultsetOutputLimitParameterizedContextCase(ctx, caseScenario, caseName)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetOutputLimitParameterizedContextID, caseName, err)
	}
	trace.Records = append(trace.Records, caseTrace.Records...)
	return trace, nil
}

func runResultsetOutputLimitParameterizedContextCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitParameterizedScheduleEvent](env, "SupportScheduleSimpleEvent"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetOutputLimitParameterizedS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	startStream := esper.From[resultsetOutputLimitParameterizedScheduleEvent](env, "SupportScheduleSimpleEvent")
	contextStart := esper.PatternFrom(startStream, "sse", esper.Literal(true))
	contextDef, err := esper.CreatePatternInitiatedContext(env, "MyCtx", contextStart)
	if err != nil {
		return compat.Trace{}, err
	}
	_ = contextDef

	base := esper.From[resultsetOutputLimitParameterizedS0](env, "SupportBean_S0")
	cron := esper.CronSchedule{
		Minute:     esper.CronValuesExpr(esper.ContextPatternField[int]("sse", "atminute")),
		Hour:       esper.CronValuesExpr(esper.ContextPatternField[int]("sse", "athour")),
		DayOfMonth: esper.CronWildcard(),
		Month:      esper.CronWildcard(),
		Weekday:    esper.CronWildcard(),
	}
	query := base.Aggregate(esper.Alias("c", esper.CountAll())).Query(
		esper.StatementName("s0"),
		esper.WithContext("MyCtx"),
		esper.WithOutput(esper.OutputAndWhenTerminated(esper.OutputAt(cron, esper.OutputLast()))),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetOutputLimitParameterizedContextJavaRuntimeIDs[0]),
		esper.WithStartTime(time.Date(2002, 5, 1, 9, 0, 0, 0, time.UTC)),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	statements := deployment.Statements()
	if len(statements) != 1 {
		return compat.Trace{}, fmt.Errorf("%s case %q deployed %d statements", resultsetOutputLimitParameterizedContextID, caseName, len(statements))
	}
	return compat.ReplayWithStatements(ctx, engine, statements[0], scenario,
		decodeResultsetOutputLimitParameterizedContextPayload,
		func(name string) (*esper.Statement, error) {
			if name != statements[0].Name() {
				return nil, fmt.Errorf("unknown %s statement %q", resultsetOutputLimitParameterizedContextID, name)
			}
			return statements[0], nil
		})
}

func decodeResultsetOutputLimitParameterizedContextPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportScheduleSimpleEvent":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultsetOutputLimitParameterizedContextFields(fields, "athour", "atminute"); err != nil {
			return nil, err
		}
		var event resultsetOutputLimitParameterizedScheduleEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportScheduleSimpleEvent: %w", err)
		}
		return event, nil
	case "SupportBean_S0":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireResultsetOutputLimitParameterizedContextFields(fields, "id"); err != nil {
			return nil, err
		}
		var event resultsetOutputLimitParameterizedS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitParameterizedContextID, step.EventType)
	}
}

func requireResultsetOutputLimitParameterizedContextFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetOutputLimitParameterizedContextStringArray(raw json.RawMessage, expected []string, name string) error {
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
