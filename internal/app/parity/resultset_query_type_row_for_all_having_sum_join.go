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
	resultSetQueryTypeRowForAllHavingSumJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRowForAllHavingSumJoinCase        = "sum-join"
	resultSetQueryTypeRowForAllHavingSumJoinID          = "resultset-querytype-row-for-all-having-sum-join"
	resultSetQueryTypeRowForAllHavingSumJoinDescription = "ResultSetQueryTypeRowForAllHaving ordinal 1: time-window join sum with having threshold and listener expiry."
	resultSetQueryTypeRowForAllHavingSumJoinEPL         = "@name('s0') select irstream sum(longBoxed) as mySum from SupportBeanString#time(10 seconds) as one, SupportBean#time(10 seconds) as two where one.theString = two.theString having sum(longBoxed) > 10"
	resultSetQueryTypeRowForAllHavingSumJoinStaticID    = "java-bf6637bf00f9e0e78f89"
)

var (
	resultSetQueryTypeRowForAllHavingSumJoinJavaSources    = []string{"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAllHaving.java"}
	resultSetQueryTypeRowForAllHavingSumJoinJavaRuntimeIDs = []string{"java-runtime-dfb97881f562ac33e873"}
	resultSetQueryTypeRowForAllHavingSumJoinJavaExecutions = []string{"ResultSetQueryTypeRowForAllWHavingSumJoin"}
)

type resultSetQueryTypeRowForAllHavingSumJoinString struct {
	TheString string `esper:"theString"`
}

type resultSetQueryTypeRowForAllHavingSumJoinBean struct {
	TheString  string `esper:"theString"`
	IntBoxed   *int32 `esper:"intBoxed"`
	ShortBoxed *int16 `esper:"shortBoxed"`
	LongBoxed  *int64 `esper:"longBoxed"`
}

func loadResultSetQueryTypeRowForAllHavingSumJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum-join scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-row-for-all-having-sum-join scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, err
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("scenario is missing field %q", name)
		}
	}
	var version, id, description, javaCommit, javaSource string
	for name, target := range map[string]*string{
		"version": &version, "id": &id, "description": &description,
		"javaCommit": &javaCommit, "javaSource": &javaSource,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", name)
		}
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRowForAllHavingSumJoinID ||
		description != resultSetQueryTypeRowForAllHavingSumJoinDescription ||
		javaCommit != resultSetQueryTypeRowForAllHavingSumJoinJavaCommit ||
		javaSource != resultSetQueryTypeRowForAllHavingSumJoinJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum-join metadata is not pinned")
	}
	var runtimes, names, staticIDs, flags []string
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRowForAllHavingSumJoinJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRowForAllHavingSumJoinJavaExecutions,
		"javaStaticIds": {resultSetQueryTypeRowForAllHavingSumJoinStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}
	if err := json.Unmarshal(root["javaRuntimes"], &runtimes); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaNames"], &names); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaStaticIds"], &staticIDs); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaFlags"], &flags); err != nil {
		return compat.Scenario{}, err
	}
	if !equalStrings(runtimes, resultSetQueryTypeRowForAllHavingSumJoinJavaRuntimeIDs) ||
		!equalStrings(names, resultSetQueryTypeRowForAllHavingSumJoinJavaExecutions) ||
		!equalStrings(staticIDs, []string{resultSetQueryTypeRowForAllHavingSumJoinStaticID}) || len(flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum-join Java references are not pinned")
	}

	var cases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &cases); err != nil || len(cases) != 1 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly one case")
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(cases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	caseFields := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
	if len(caseObject) != len(caseFields) {
		return compat.Scenario{}, fmt.Errorf("scenario case contains unexpected or missing fields")
	}
	for _, name := range caseFields {
		if _, ok := caseObject[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("scenario case is missing field %q", name)
		}
	}
	var caseMeta struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	ordinal, ordinalErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(caseObject["ordinal"], "ordinal")
	iteratorSnapshots, iteratorErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(caseObject["iteratorSnapshots"], "iteratorSnapshots")
	if ordinalErr != nil || iteratorErr != nil || ordinal != 1 || iteratorSnapshots != 0 ||
		json.Unmarshal(cases[0], &caseMeta) != nil || caseMeta.Case != resultSetQueryTypeRowForAllHavingSumJoinCase ||
		caseMeta.RuntimeID != resultSetQueryTypeRowForAllHavingSumJoinJavaRuntimeIDs[0] ||
		caseMeta.ExecutionName != resultSetQueryTypeRowForAllHavingSumJoinJavaExecutions[0] ||
		caseMeta.Observation != "listener" || caseMeta.EPL != resultSetQueryTypeRowForAllHavingSumJoinEPL {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum-join case metadata is not pinned")
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 9 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly nine steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		expectedFields := 2
		if index == 2 || index == 3 || index == 5 || index == 7 {
			expectedFields = 3
		}
		if len(object) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("scenario step %d fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeRowForAllHavingSumJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeRowForAllHavingSumJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRowForAllHavingSumJoinID || len(scenario.Steps) != 9 {
		return fmt.Errorf("resultset-querytype-row-for-all-having-sum-join scenario steps are not pinned")
	}
	if marker := scenario.Steps[0]; marker.Op != "case" || marker.Case != resultSetQueryTypeRowForAllHavingSumJoinCase {
		return fmt.Errorf("scenario must start with case %q", resultSetQueryTypeRowForAllHavingSumJoinCase)
	}
	for index, seconds := range map[int]int64{1: 0, 4: 5, 6: 8, 8: 10} {
		step := scenario.Steps[index]
		if step.Op != "advance-time" {
			return fmt.Errorf("scenario step %d must advance time", index)
		}
		at, err := time.Parse(time.RFC3339Nano, step.At)
		if err != nil || !at.Equal(time.Unix(seconds, 0).UTC()) {
			return fmt.Errorf("scenario step %d time is not pinned", index)
		}
	}
	stringStep := scenario.Steps[2]
	if stringStep.Op != "send" || stringStep.EventType != "SupportBeanString" {
		return fmt.Errorf("scenario step 2 must send SupportBeanString")
	}
	stringEvent, err := decodeResultSetQueryTypeRowForAllHavingSumJoinStringPayload(stringStep)
	if err != nil || stringEvent.TheString != "KEY" {
		return fmt.Errorf("scenario step 2 has unexpected SupportBeanString payload")
	}
	for index, expected := range map[int]int64{3: 10, 5: 15, 7: -5} {
		step := scenario.Steps[index]
		if step.Op != "send" || step.EventType != "SupportBean" {
			return fmt.Errorf("scenario step %d must send SupportBean", index)
		}
		bean, err := decodeResultSetQueryTypeRowForAllHavingSumJoinBeanPayload(step)
		if err != nil || bean.TheString != "KEY" || bean.LongBoxed == nil || *bean.LongBoxed != expected ||
			bean.IntBoxed == nil || *bean.IntBoxed != 0 || bean.ShortBoxed == nil || *bean.ShortBoxed != 0 {
			return fmt.Errorf("scenario step %d has unexpected SupportBean payload", index)
		}
	}
	return nil
}

func runResultSetQueryTypeRowForAllHavingSumJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRowForAllHavingSumJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllHavingSumJoinString](env, "SupportBeanString"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllHavingSumJoinBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	left := esper.From[resultSetQueryTypeRowForAllHavingSumJoinString](env, "SupportBeanString").Window(esper.TimeWindow(10 * time.Second))
	right := esper.From[resultSetQueryTypeRowForAllHavingSumJoinBean](env, "SupportBean").Window(esper.TimeWindow(10 * time.Second))
	joined := esper.Join(left, right, esper.OnEqual(
		esper.Field[resultSetQueryTypeRowForAllHavingSumJoinString, string]("theString"),
		esper.Field[resultSetQueryTypeRowForAllHavingSumJoinBean, string]("theString"),
	))
	longBoxed := esper.JoinField[*int64](1, "longBoxed")
	sum := esper.Sum[int64](esper.Cast[*int64, int64](longBoxed))
	query := joined.Aggregate(esper.Alias("mySum", sum)).Having(
		esper.Greater[int64](sum, esper.Literal(int64(10))),
	).Query(esper.StatementName("s0"), esper.WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRowForAllHavingSumJoinJavaRuntimeIDs[0])
	if err != nil {
		return compat.Trace{}, err
	}
	defer engine.Close(context.Background())
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetQueryTypeRowForAllHavingSumJoinPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-querytype-row-for-all-having-sum-join statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetQueryTypeRowForAllHavingSumJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBeanString":
		return decodeResultSetQueryTypeRowForAllHavingSumJoinStringPayload(step)
	case "SupportBean":
		return decodeResultSetQueryTypeRowForAllHavingSumJoinBeanPayload(step)
	default:
		return nil, fmt.Errorf("unknown event type %q", step.EventType)
	}
}

func decodeResultSetQueryTypeRowForAllHavingSumJoinStringPayload(step compat.Step) (resultSetQueryTypeRowForAllHavingSumJoinString, error) {
	if step.EventType != "SupportBeanString" {
		return resultSetQueryTypeRowForAllHavingSumJoinString{}, fmt.Errorf("step must send SupportBeanString")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinString{}, err
	}
	if len(fields) != 1 {
		return resultSetQueryTypeRowForAllHavingSumJoinString{}, fmt.Errorf("SupportBeanString payload must contain exactly theString")
	}
	var result resultSetQueryTypeRowForAllHavingSumJoinString
	if err := json.Unmarshal(fields["theString"], &result.TheString); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinString{}, fmt.Errorf("theString must be a string")
	}
	return result, nil
}

func decodeResultSetQueryTypeRowForAllHavingSumJoinBeanPayload(step compat.Step) (resultSetQueryTypeRowForAllHavingSumJoinBean, error) {
	if step.EventType != "SupportBean" {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, fmt.Errorf("step must send SupportBean")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, err
	}
	if len(fields) != 4 {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, fmt.Errorf("SupportBean payload must contain exactly theString, intBoxed, shortBoxed, and longBoxed")
	}
	var result resultSetQueryTypeRowForAllHavingSumJoinBean
	if err := json.Unmarshal(fields["theString"], &result.TheString); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, fmt.Errorf("theString must be a string")
	}
	if err := decodeResultSetQueryTypeRowForAllHavingSumJoinInteger(fields["intBoxed"], &result.IntBoxed, "intBoxed"); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, err
	}
	if err := decodeResultSetQueryTypeRowForAllHavingSumJoinInteger(fields["shortBoxed"], &result.ShortBoxed, "shortBoxed"); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, err
	}
	if err := decodeResultSetQueryTypeRowForAllHavingSumJoinInteger(fields["longBoxed"], &result.LongBoxed, "longBoxed"); err != nil {
		return resultSetQueryTypeRowForAllHavingSumJoinBean{}, err
	}
	return result, nil
}

func decodeResultSetQueryTypeRowForAllHavingSumJoinInteger[T interface{ int32 | int16 | int64 }](raw json.RawMessage, target **T, name string) error {
	if string(bytes.TrimSpace(raw)) == "null" {
		return fmt.Errorf("%s cannot be null", name)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%s must be an integer", name)
	}
	*target = &value
	return nil
}
