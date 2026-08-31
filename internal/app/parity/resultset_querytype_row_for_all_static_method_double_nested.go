package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

import esper "github.com/liubaicai/esper"
import "github.com/liubaicai/esper/internal/compat"

const (
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedCase        = "static-method-double-nested"
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedID          = "resultset-querytype-row-for-all-static-method-double-nested"
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedDescription = "ResultSetQueryTypeRowForAll ordinal 12: nested static method composition with last aggregate."
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedEPL         = "import com.espertech.esper.regressionlib.suite.resultset.querytype.ResultSetQueryTypeRowForAll$MyHelper;\n@name('s0') select MyHelper.doOuter(MyHelper.doInner(last(theString))) as c0 from SupportBean;\n"
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedStaticID    = "java-421838b50fc1c8eff9f6"
)

var (
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAll.java",
	}
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaRuntimeIDs = []string{
		"java-runtime-f8487c524573a3a2f08c",
	}
	resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaExecutions = []string{
		"ResultSetQueryTypeRowForAllStaticMethodDoubleNested",
	}
)

type resultSetQueryTypeRowForAllStaticMethodDoubleNestedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

func loadResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-row-for-all-static-method-double-nested scenario: %w", err)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRowForAllStaticMethodDoubleNestedID ||
		description != resultSetQueryTypeRowForAllStaticMethodDoubleNestedDescription ||
		javaCommit != resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaCommit ||
		javaSource != resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested metadata is not pinned")
	}

	var runtimes, names, staticIDs, flags []string
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaExecutions,
		"javaStaticIds": {resultSetQueryTypeRowForAllStaticMethodDoubleNestedStaticID},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}
	if err := json.Unmarshal(root["javaRuntimes"], &runtimes); err != nil ||
		!equalStrings(runtimes, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested Java runtime references are not pinned")
	}
	if err := json.Unmarshal(root["javaNames"], &names); err != nil ||
		!equalStrings(names, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested Java execution references are not pinned")
	}
	if err := json.Unmarshal(root["javaStaticIds"], &staticIDs); err != nil ||
		!equalStrings(staticIDs, []string{resultSetQueryTypeRowForAllStaticMethodDoubleNestedStaticID}) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested Java static references are not pinned")
	}
	if err := json.Unmarshal(root["javaFlags"], &flags); err != nil || len(flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested Java flags are not pinned")
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly one case")
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
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
	if ordinalErr != nil || iteratorErr != nil || ordinal != 12 || iteratorSnapshots != 0 ||
		json.Unmarshal(rawCases[0], &caseMeta) != nil ||
		caseMeta.Case != resultSetQueryTypeRowForAllStaticMethodDoubleNestedCase ||
		caseMeta.RuntimeID != resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaRuntimeIDs[0] ||
		caseMeta.ExecutionName != resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaExecutions[0] ||
		caseMeta.Observation != "listener" || caseMeta.EPL != resultSetQueryTypeRowForAllStaticMethodDoubleNestedEPL {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested case metadata is not pinned")
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 2 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly two steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		expectedFields := 2
		if index == 1 {
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
	if err := validateResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRowForAllStaticMethodDoubleNestedID || len(scenario.Steps) != 2 {
		return fmt.Errorf("resultset-querytype-row-for-all-static-method-double-nested scenario steps are not pinned")
	}
	marker := scenario.Steps[0]
	if marker.Op != "case" || marker.Case != resultSetQueryTypeRowForAllStaticMethodDoubleNestedCase {
		return fmt.Errorf("scenario must start with case %q", resultSetQueryTypeRowForAllStaticMethodDoubleNestedCase)
	}
	step := scenario.Steps[1]
	if step.Op != "send" || step.EventType != "SupportBean" {
		return fmt.Errorf("scenario step 1 must send SupportBean")
	}
	payloadValue, err := decodeResultSetQueryTypeRowForAllStaticMethodDoubleNestedPayload(step)
	if err != nil {
		return fmt.Errorf("scenario step 1: %w", err)
	}
	payload, ok := payloadValue.(resultSetQueryTypeRowForAllStaticMethodDoubleNestedBean)
	if !ok || payload.TheString != "E1" || payload.IntPrimitive != 1 {
		return fmt.Errorf("scenario step 1 has unexpected SupportBean payload")
	}
	return nil
}

func runResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRowForAllStaticMethodDoubleNestedBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[resultSetQueryTypeRowForAllStaticMethodDoubleNestedBean, string]("theString")
	last := esper.Last[string](theString)
	inner := esper.Func1[string, string]("MyHelper.doInner", func(value string) string {
		return "i" + value + "i"
	}, last)
	outer := esper.Func1[string, string]("MyHelper.doOuter", func(value string) string {
		return "o" + value + "o"
	}, inner)
	query := esper.From[resultSetQueryTypeRowForAllStaticMethodDoubleNestedBean](env, "SupportBean").
		Aggregate(esper.Alias("c0", outer)).
		Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaRuntimeIDs[0])
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeResultSetQueryTypeRowForAllStaticMethodDoubleNestedPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown resultset-querytype-row-for-all-static-method-double-nested statement %q", name)
		}
		return statement, nil
	})
}

func decodeResultSetQueryTypeRowForAllStaticMethodDoubleNestedPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	if len(fields) != 2 {
		return nil, fmt.Errorf("SupportBean payload must contain exactly theString and intPrimitive")
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" {
		return nil, fmt.Errorf("theString must be a string")
	}
	if string(bytes.TrimSpace(fields["intPrimitive"])) == "null" {
		return nil, fmt.Errorf("intPrimitive must be an integer")
	}
	var result resultSetQueryTypeRowForAllStaticMethodDoubleNestedBean
	if err := json.Unmarshal(fields["theString"], &result.TheString); err != nil {
		return nil, fmt.Errorf("theString must be a string")
	}
	if err := json.Unmarshal(fields["intPrimitive"], &result.IntPrimitive); err != nil {
		return nil, fmt.Errorf("intPrimitive must be an integer")
	}
	return result, nil
}
