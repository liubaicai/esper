package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultSetQueryTypeRollupOrderByUnidirectionalJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRollupOrderByUnidirectionalID          = "resultset-querytype-rollup-orderby-unidirectional"
	resultSetQueryTypeRollupOrderByUnidirectionalDescription = "ResultSetQueryTypeRollupHavingAndOrderBy ordinals 4-7: time-batch rollup ordering and unidirectional cube listener semantics."
	resultSetQueryTypeRollupOrderByUnidirectionalSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupHavingAndOrderBy.java"
)

var resultSetQueryTypeRollupOrderByUnidirectionalJavaStaticIDs = []string{
	"java-715eae421ccf37a5062a",
	"java-24e47ca92d533e352474",
	"java-ba719758f9ea057c8d79",
}

var (
	resultSetQueryTypeRollupOrderByUnidirectionalJavaRuntimeIDs = []string{
		"java-runtime-b09e67f84d7426e94834",
		"java-runtime-7859d904aebfc874ac11",
		"java-runtime-7f3911bcedf70526d5df",
		"java-runtime-2ff9039bfd91c9054507",
	}
	resultSetQueryTypeRollupOrderByUnidirectionalJavaExecutions = []string{
		"ResultSetQueryTypeOrderByTwoCriteriaAsc{join=false}",
		"ResultSetQueryTypeOrderByTwoCriteriaAsc{join=true}",
		"ResultSetQueryTypeUnidirectional",
		"ResultSetQueryTypeOrderByOneCriteriaDesc",
	}
	resultSetQueryTypeRollupOrderByUnidirectionalCases = []string{
		"order-by-two-criteria-no-join",
		"order-by-two-criteria-join",
		"unidirectional-cube",
		"order-by-one-criteria-desc",
	}
	resultSetQueryTypeRollupOrderByUnidirectionalOrdinals = []int{4, 5, 6, 7}
)

var resultSetQueryTypeRollupOrderByUnidirectionalJavaSources = []string{
	resultSetQueryTypeRollupOrderByUnidirectionalSource,
}

var resultSetQueryTypeRollupOrderByUnidirectionalCaseEPL = []string{
	"@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#time_batch(1 sec) group by rollup(theString, intPrimitive) order by theString, intPrimitive",
	"@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#time_batch(1 sec) , SupportBean_S0#lastevent group by rollup(theString, intPrimitive) order by theString, intPrimitive",
	"@Name('s0')select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean_S0 unidirectional, SupportBean#keepall group by cube(theString, intPrimitive)",
	"@Name('s0')select irstream theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean#time_batch(1 sec) group by rollup(theString, intPrimitive) order by theString desc;",
}

type resultSetQueryTypeRollupOrderByBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type resultSetQueryTypeRollupOrderByS0 struct {
	ID int `esper:"id"`
}

func loadResultSetQueryTypeRollupOrderByUnidirectionalScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read resultset-querytype-rollup-orderby-unidirectional scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-rollup-orderby-unidirectional scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-rollup-orderby-unidirectional scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario is missing field %q", name)
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
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRollupOrderByUnidirectionalID ||
		description != resultSetQueryTypeRollupOrderByUnidirectionalDescription ||
		javaCommit != resultSetQueryTypeRollupOrderByUnidirectionalJavaCommit ||
		javaSource != resultSetQueryTypeRollupOrderByUnidirectionalSource {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario metadata is not pinned")
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRollupOrderByUnidirectionalJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRollupOrderByUnidirectionalJavaExecutions,
		"javaStaticIds": resultSetQueryTypeRollupOrderByUnidirectionalJavaStaticIDs,
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional %w", err)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeRollupOrderByUnidirectionalCases) {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario must contain exactly four cases")
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		requiredCase := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
		if len(object) != len(requiredCase) {
			return compat.Scenario{}, fmt.Errorf("scenario case %d contains unexpected or missing fields", index)
		}
		for _, name := range requiredCase {
			if _, ok := object[name]; !ok {
				return compat.Scenario{}, fmt.Errorf("scenario case %d is missing field %q", index, name)
			}
		}
		var metadata struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		ordinal, ordinalErr := decodeResultSetQueryTypeRollupOrderByInteger(object["ordinal"], "ordinal")
		iteratorSnapshots, iteratorErr := decodeResultSetQueryTypeRollupOrderByInteger(object["iteratorSnapshots"], "iteratorSnapshots")
		if ordinalErr != nil || iteratorErr != nil || json.Unmarshal(rawCase, &metadata) != nil ||
			metadata.Case != resultSetQueryTypeRollupOrderByUnidirectionalCases[index] ||
			ordinal != resultSetQueryTypeRollupOrderByUnidirectionalOrdinals[index] ||
			metadata.RuntimeID != resultSetQueryTypeRollupOrderByUnidirectionalJavaRuntimeIDs[index] ||
			metadata.ExecutionName != resultSetQueryTypeRollupOrderByUnidirectionalJavaExecutions[index] ||
			metadata.Observation != "listener" || iteratorSnapshots != 0 ||
			metadata.EPL != resultSetQueryTypeRollupOrderByUnidirectionalCaseEPL[index] {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 42 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario must contain exactly 42 steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		expectedFields := 3
		if resultSetQueryTypeRollupOrderByUnidirectionalStepIsMarker(index) || resultSetQueryTypeRollupOrderByUnidirectionalStepIsAdvance(index) {
			expectedFields = 2
		}
		if len(object) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("scenario step %d fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeRollupOrderByUnidirectionalScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func resultSetQueryTypeRollupOrderByUnidirectionalStepIsMarker(index int) bool {
	switch index {
	case 0, 11, 23, 31:
		return true
	default:
		return false
	}
}

func resultSetQueryTypeRollupOrderByUnidirectionalStepIsAdvance(index int) bool {
	switch index {
	case 1, 6, 10, 12, 18, 22, 32, 37, 41:
		return true
	default:
		return false
	}
}

func validateResultSetQueryTypeRollupOrderByUnidirectionalScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeRollupOrderByUnidirectionalID || len(scenario.Steps) != 42 {
		return fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional scenario steps are not pinned")
	}
	markers := map[int]string{
		0:  resultSetQueryTypeRollupOrderByUnidirectionalCases[0],
		11: resultSetQueryTypeRollupOrderByUnidirectionalCases[1],
		23: resultSetQueryTypeRollupOrderByUnidirectionalCases[2],
		31: resultSetQueryTypeRollupOrderByUnidirectionalCases[3],
	}
	for index, expected := range markers {
		step := scenario.Steps[index]
		if step.Op != "case" || step.Case != expected {
			return fmt.Errorf("scenario step %d must start case %q", index, expected)
		}
	}
	advances := map[int]string{
		1:  "1970-01-01T00:00:00Z",
		6:  "1970-01-01T00:00:01Z",
		10: "1970-01-01T00:00:02Z",
		12: "1970-01-01T00:00:00Z",
		18: "1970-01-01T00:00:01Z",
		22: "1970-01-01T00:00:02Z",
		32: "1970-01-01T00:00:00Z",
		37: "1970-01-01T00:00:01Z",
		41: "1970-01-01T00:00:02Z",
	}
	for index, expected := range advances {
		step := scenario.Steps[index]
		if step.Op != "advance-time" || step.At != expected || step.Case != "" {
			return fmt.Errorf("scenario step %d must advance time to %s", index, expected)
		}
	}
	for index, step := range scenario.Steps {
		if resultSetQueryTypeRollupOrderByUnidirectionalStepIsMarker(index) || resultSetQueryTypeRollupOrderByUnidirectionalStepIsAdvance(index) {
			continue
		}
		if step.Op != "send" || step.Case != "" {
			return fmt.Errorf("scenario step %d must be an unlabelled send", index)
		}
	}

	beans := []resultSetQueryTypeRollupOrderByBean{
		{TheString: "E2", IntPrimitive: 10, LongPrimitive: 100},
		{TheString: "E1", IntPrimitive: 11, LongPrimitive: 200},
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 300},
		{TheString: "E1", IntPrimitive: 11, LongPrimitive: 400},
	}
	secondBatch := []resultSetQueryTypeRollupOrderByBean{
		{TheString: "E1", IntPrimitive: 11, LongPrimitive: 500},
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 600},
		{TheString: "E1", IntPrimitive: 12, LongPrimitive: 700},
	}
	validateBean := func(index int, expected resultSetQueryTypeRollupOrderByBean) error {
		step := scenario.Steps[index]
		if step.Op != "send" || step.EventType != "SupportBean" {
			return fmt.Errorf("scenario step %d must send SupportBean", index)
		}
		value, err := decodeResultSetQueryTypeRollupOrderByBeanPayload(step)
		if err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if value != expected {
			return fmt.Errorf("scenario step %d SupportBean payload is not pinned", index)
		}
		return nil
	}
	for _, index := range []int{2, 3, 4, 5} {
		if err := validateBean(index, beans[index-2]); err != nil {
			return err
		}
	}
	for _, index := range []int{7, 8, 9} {
		if err := validateBean(index, secondBatch[index-7]); err != nil {
			return err
		}
	}
	if err := validateResultSetQueryTypeRollupOrderByS0Step(scenario.Steps[13], 1); err != nil {
		return err
	}
	for _, index := range []int{14, 15, 16, 17} {
		if err := validateBean(index, beans[index-14]); err != nil {
			return err
		}
	}
	for _, index := range []int{19, 20, 21} {
		if err := validateBean(index, secondBatch[index-19]); err != nil {
			return err
		}
	}
	for index, expected := range []resultSetQueryTypeRollupOrderByBean{
		{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100},
		{TheString: "E2", IntPrimitive: 20, LongPrimitive: 200},
		{TheString: "E1", IntPrimitive: 11, LongPrimitive: 300},
		{TheString: "E2", IntPrimitive: 20, LongPrimitive: 400},
	} {
		if err := validateBean(24+index, expected); err != nil {
			return err
		}
	}
	if err := validateResultSetQueryTypeRollupOrderByS0Step(scenario.Steps[28], 1); err != nil {
		return err
	}
	if err := validateBean(29, resultSetQueryTypeRollupOrderByBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 1}); err != nil {
		return err
	}
	if err := validateResultSetQueryTypeRollupOrderByS0Step(scenario.Steps[30], 2); err != nil {
		return err
	}
	for _, index := range []int{33, 34, 35, 36} {
		if err := validateBean(index, beans[index-33]); err != nil {
			return err
		}
	}
	for _, index := range []int{38, 39, 40} {
		if err := validateBean(index, secondBatch[index-38]); err != nil {
			return err
		}
	}
	return nil
}

func validateResultSetQueryTypeRollupOrderByS0Step(step compat.Step, expected int) error {
	if step.Op != "send" || step.EventType != "SupportBean_S0" {
		return fmt.Errorf("scenario SupportBean_S0 step must send SupportBean_S0")
	}
	value, err := decodeResultSetQueryTypeRollupOrderByS0Payload(step)
	if err != nil {
		return err
	}
	if value.ID != expected {
		return fmt.Errorf("SupportBean_S0 id is not pinned: got %d want %d", value.ID, expected)
	}
	return nil
}

func runResultSetQueryTypeRollupOrderByUnidirectionalScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRollupOrderByUnidirectionalScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for index, caseName := range resultSetQueryTypeRollupOrderByUnidirectionalCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runResultSetQueryTypeRollupOrderByUnidirectionalCase(ctx, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) != 8 {
		return compat.Trace{}, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional expected eight listener records, got %d", len(trace.Records))
	}
	return trace, nil
}

func runResultSetQueryTypeRollupOrderByUnidirectionalCase(ctx context.Context, scenario compat.Scenario, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeRollupOrderByBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeRollupOrderByS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	beanStream := esper.From[resultSetQueryTypeRollupOrderByBean](env, "SupportBean")
	var query esper.Query
	switch caseIndex {
	case 0, 1, 3:
		batch := beanStream.Window(esper.TimeBatch(time.Second))
		var aggregate esper.AggregateStream
		if caseIndex == 0 || caseIndex == 3 {
			theString := esper.Field[resultSetQueryTypeRollupOrderByBean, string]("theString")
			intPrimitive := esper.Field[resultSetQueryTypeRollupOrderByBean, int]("intPrimitive")
			aggregate = batch.GroupByRollup(theString, intPrimitive).Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](esper.Field[resultSetQueryTypeRollupOrderByBean, int64]("longPrimitive"))),
			)
		} else {
			s0 := esper.From[resultSetQueryTypeRollupOrderByS0](env, "SupportBean_S0").Window(esper.LastEvent())
			joined := esper.Join(batch, s0)
			theString := esper.JoinField[string](0, "theString")
			intPrimitive := esper.JoinField[int](0, "intPrimitive")
			aggregate = joined.Aggregate().Rollup(theString, intPrimitive).Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](esper.JoinField[int64](0, "longPrimitive"))),
			)
		}
		options := []esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()}
		if caseIndex == 3 {
			options = append(options, esper.OrderBy(esper.Descending(esper.ResultField[string]("c0"))))
		} else {
			options = append(options, esper.OrderBy(
				esper.Ascending(esper.ResultField[string]("c0")),
				esper.Ascending(esper.ResultField[int]("c1")),
			))
		}
		query = aggregate.Query(options...)
	case 2:
		passive := esper.From[resultSetQueryTypeRollupOrderByBean](env, "SupportBean").Window(esper.KeepAll())
		driver := esper.From[resultSetQueryTypeRollupOrderByS0](env, "SupportBean_S0")
		joined := esper.JoinMany(
			esper.JoinSource(driver).Unidirectional(),
			esper.JoinSource(passive),
		)
		theString := esper.JoinField[string](1, "theString")
		intPrimitive := esper.JoinField[int](1, "intPrimitive")
		query = joined.Aggregate().Cube(theString, intPrimitive).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", esper.Sum[int64](esper.JoinField[int64](1, "longPrimitive"))),
		).Query(esper.StatementName("s0"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported resultset-querytype-rollup-orderby-unidirectional case index %d", caseIndex)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRollupOrderByUnidirectionalJavaRuntimeIDs[caseIndex])
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	caseName := resultSetQueryTypeRollupOrderByUnidirectionalCases[caseIndex]
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		newRows := compat.NormalizeResults(batch.New)
		oldRows := compat.NormalizeResults(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			return nil
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case: caseName, Operation: "listener", Statement: statement.Name(), Sequence: sequence,
			Time: batch.Time.UTC().Format(time.RFC3339Nano), New: newRows, Old: oldRows,
		})
		return nil
	}); err != nil {
		return compat.Trace{}, err
	}
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			if step.Case != caseName {
				return compat.Trace{}, fmt.Errorf("case marker %q is not pinned", step.Case)
			}
		case "send":
			payload, err := decodeResultSetQueryTypeRollupOrderByPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, fmt.Errorf("decode advance-time: %w", err)
			}
			if err := engine.AdvanceTime(ctx, at.UTC()); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported resultset-querytype-rollup-orderby-unidirectional step op %q", step.Op)
		}
	}
	if len(trace.Records) != 2 {
		return trace, fmt.Errorf("resultset-querytype-rollup-orderby-unidirectional case %q expected two listener records, got %d", caseName, len(trace.Records))
	}
	return trace, nil
}

func decodeResultSetQueryTypeRollupOrderByPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		return decodeResultSetQueryTypeRollupOrderByBeanPayload(step)
	case "SupportBean_S0":
		return decodeResultSetQueryTypeRollupOrderByS0Payload(step)
	default:
		return nil, fmt.Errorf("unknown resultset-querytype-rollup-orderby-unidirectional event type %q", step.EventType)
	}
}

func decodeResultSetQueryTypeRollupOrderByBeanPayload(step compat.Step) (resultSetQueryTypeRollupOrderByBean, error) {
	if step.EventType != "SupportBean" {
		return resultSetQueryTypeRollupOrderByBean{}, fmt.Errorf("step must send SupportBean")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeRollupOrderByBean{}, err
	}
	if len(fields) != 3 {
		return resultSetQueryTypeRollupOrderByBean{}, fmt.Errorf("SupportBean payload must contain exactly theString, intPrimitive, and longPrimitive")
	}
	if string(bytes.TrimSpace(fields["theString"])) == "null" {
		return resultSetQueryTypeRollupOrderByBean{}, fmt.Errorf("theString must be a string")
	}
	var result resultSetQueryTypeRollupOrderByBean
	if err := json.Unmarshal(fields["theString"], &result.TheString); err != nil {
		return resultSetQueryTypeRollupOrderByBean{}, fmt.Errorf("theString must be a string")
	}
	intPrimitive, err := decodeResultSetQueryTypeRollupOrderByInteger(fields["intPrimitive"], "intPrimitive")
	if err != nil {
		return resultSetQueryTypeRollupOrderByBean{}, err
	}
	longPrimitive, err := decodeResultSetQueryTypeRollupOrderByInt64(fields["longPrimitive"], "longPrimitive")
	if err != nil {
		return resultSetQueryTypeRollupOrderByBean{}, err
	}
	result.IntPrimitive = intPrimitive
	result.LongPrimitive = longPrimitive
	return result, nil
}

func decodeResultSetQueryTypeRollupOrderByS0Payload(step compat.Step) (resultSetQueryTypeRollupOrderByS0, error) {
	if step.EventType != "SupportBean_S0" {
		return resultSetQueryTypeRollupOrderByS0{}, fmt.Errorf("step must send SupportBean_S0")
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return resultSetQueryTypeRollupOrderByS0{}, err
	}
	if len(fields) != 1 {
		return resultSetQueryTypeRollupOrderByS0{}, fmt.Errorf("SupportBean_S0 payload must contain exactly id")
	}
	id, err := decodeResultSetQueryTypeRollupOrderByInteger(fields["id"], "id")
	if err != nil {
		return resultSetQueryTypeRollupOrderByS0{}, err
	}
	return resultSetQueryTypeRollupOrderByS0{ID: id}, nil
}

func decodeResultSetQueryTypeRollupOrderByInteger(raw json.RawMessage, name string) (int, error) {
	value, err := decodeResultSetQueryTypeRollupOrderByInt64(raw, name)
	if err != nil || int64(int(value)) != value {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func decodeResultSetQueryTypeRollupOrderByInt64(raw json.RawMessage, name string) (int64, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number == "" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return value, nil
}
