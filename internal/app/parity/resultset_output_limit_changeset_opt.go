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
	resultsetOutputLimitChangesetID          = "resultset-output-limit-changeset-opt"
	resultsetOutputLimitChangesetDescription = "ResultSetOutputLimitChangeSetOpt: white-box changeset counter across output-limit hint, select, group-by, having, and kind variants."
	resultsetOutputLimitChangesetJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitChangesetSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitChangeSetOpt.java"

	resultsetOutputLimitChangesetCaseName = "changeset"
	resultsetOutputLimitChangesetRounds   = 36
	resultsetOutputLimitChangesetSends    = 5
)

var (
	resultsetOutputLimitChangesetJavaSources = []string{
		resultsetOutputLimitChangesetSource,
	}
	resultsetOutputLimitChangesetJavaRuntimeIDs = []string{
		"java-runtime-9b0f27f829d9820e3792",
	}
	resultsetOutputLimitChangesetJavaExecutions = []string{
		"ResultSetOutputLimitChangeSetOpt",
	}
	resultsetOutputLimitChangesetJavaStaticIDs = []string{
		"java-9f9a094322f3c6687f00",
	}
	resultsetOutputLimitChangesetCases = []string{
		resultsetOutputLimitChangesetCaseName,
	}
	resultsetOutputLimitChangesetOrdinals = []int{0}
	resultsetOutputLimitChangesetEPLs     = []string{
		"@name('s0') select irstream intPrimitive from SupportBean#length(2) output last every 1 seconds",
	}
)

type resultsetOutputLimitChangesetBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func loadResultsetOutputLimitChangesetScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitChangesetID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitChangesetID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitChangesetID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitChangesetID, err)
	}
	if err := requireResultsetOutputLimitChangesetFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitChangesetID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitChangesetID ||
		metadata.Description != resultsetOutputLimitChangesetDescription ||
		metadata.JavaCommit != resultsetOutputLimitChangesetJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitChangesetSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitChangesetID)
	}
	if err := validateResultsetOutputLimitChangesetStringArray(root["javaRuntimes"], resultsetOutputLimitChangesetJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitChangesetStringArray(root["javaNames"], resultsetOutputLimitChangesetJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitChangesetStringArray(root["javaStaticIds"], resultsetOutputLimitChangesetJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetOutputLimitChangesetStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != 1 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly one case", resultsetOutputLimitChangesetID)
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(rawCases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case 0: %w", err)
	}
	if err := requireResultsetOutputLimitChangesetFields(caseObject,
		"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case 0: %w", err)
	}
	var caseDefinition struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	if err := json.Unmarshal(rawCases[0], &caseDefinition); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case 0: %w", err)
	}
	if caseDefinition.Case != resultsetOutputLimitChangesetCaseName ||
		caseDefinition.Ordinal != 0 ||
		caseDefinition.RuntimeID != resultsetOutputLimitChangesetJavaRuntimeIDs[0] ||
		caseDefinition.ExecutionName != resultsetOutputLimitChangesetJavaExecutions[0] ||
		caseDefinition.Observation != "listener" || caseDefinition.IteratorSnapshots != 0 ||
		caseDefinition.EPL != resultsetOutputLimitChangesetEPLs[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario case 0 metadata is not pinned", resultsetOutputLimitChangesetID)
	}

	expectedSteps := 1 + resultsetOutputLimitChangesetRounds*(resultsetOutputLimitChangesetSends+1)
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != expectedSteps {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultsetOutputLimitChangesetID, expectedSteps)
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
			if err := requireResultsetOutputLimitChangesetFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetOutputLimitChangesetFields(object, "op", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetOutputLimitChangesetPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "advance-time":
			if err := requireResultsetOutputLimitChangesetFields(object, "op", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateResultsetOutputLimitChangesetScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetOutputLimitChangesetScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetOutputLimitChangesetID {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetOutputLimitChangesetID)
	}
	steps := scenario.Steps
	if len(steps) != 1+resultsetOutputLimitChangesetRounds*(resultsetOutputLimitChangesetSends+1) {
		return fmt.Errorf("%s scenario must contain exactly %d steps", resultsetOutputLimitChangesetID, len(steps))
	}
	if steps[0].Op != "case" || steps[0].Case != resultsetOutputLimitChangesetCaseName {
		return fmt.Errorf("%s scenario must start with the %q case marker", resultsetOutputLimitChangesetID, resultsetOutputLimitChangesetCaseName)
	}
	offset := 1
	for round := 0; round < resultsetOutputLimitChangesetRounds; round++ {
		for send := 0; send < resultsetOutputLimitChangesetSends; send++ {
			step := steps[offset]
			offset++
			if step.Op != "send" || step.EventType != "SupportBean" {
				return fmt.Errorf("round %d step %d must send SupportBean", round, send)
			}
			bean, err := decodeResultsetOutputLimitChangesetPayload(step)
			if err != nil {
				return fmt.Errorf("round %d step %d: %w", round, send, err)
			}
			expected := resultsetOutputLimitChangesetBean{TheString: fmt.Sprintf("E%d", send), IntPrimitive: send}
			if bean != any(expected) {
				return fmt.Errorf("round %d step %d payload is not pinned", round, send)
			}
		}
		step := steps[offset]
		offset++
		expectedAt := time.Unix(int64(round+1), 0).UTC().Format(time.RFC3339)
		if step.Op != "advance-time" || step.At != expectedAt {
			return fmt.Errorf("round %d must advance time to %q", round, expectedAt)
		}
	}
	if offset != len(steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetOutputLimitChangesetID)
	}
	return nil
}

// resultsetOutputLimitChangesetRound pins one execution of
// ResultSetOutputLimitChangeSetOpt.run: the typed rule shape plus the hint
// that selects the buffering changeset view. The label mirrors the round's
// EPL for diagnostics only; behavior is defined by the typed query.
type resultsetOutputLimitChangesetRound struct {
	label string
	build func(env *esper.Environment) (esper.Query, error)
}

// resultsetOutputLimitChangesetRoundBuilds pins the 36-round execution order
// of ResultSetOutputLimitChangeSetOpt: unaggregated ungrouped, fully
// aggregated ungrouped (with and without having), aggregated ungrouped, and
// the two grouped families, each across default/enable/disable hints and the
// last/all/first output kinds.
func resultsetOutputLimitChangesetRoundBuilds() ([]resultsetOutputLimitChangesetRound, error) {
	intPrim := esper.Field[resultsetOutputLimitChangesetBean, int]("intPrimitive")
	theString := esper.Field[resultsetOutputLimitChangesetBean, string]("theString")
	count := esper.CountAll()
	window := func(env *esper.Environment) esper.Stream[resultsetOutputLimitChangesetBean] {
		return esper.From[resultsetOutputLimitChangesetBean](env, "SupportBean").Window(esper.LengthWindow(2))
	}
	// options pins the s0 statement name and the irstream selector shared by
	// every round, prepending the round's hint and appending its output
	// policy and any order-by clause.
	options := func(kind esper.StatementHintKind, rest ...esper.QueryOption) ([]esper.QueryOption, error) {
		opts := []esper.QueryOption{esper.StatementName("s0"), esper.WithOldStream()}
		if kind != "" {
			hint, err := esper.NewStatementHint(kind)
			if err != nil {
				return nil, err
			}
			opts = append(opts, esper.WithStatementHints(hint))
		}
		return append(opts, rest...), nil
	}
	plain := func(kind esper.StatementHintKind, rest ...esper.QueryOption) func(*esper.Environment) (esper.Query, error) {
		return func(env *esper.Environment) (esper.Query, error) {
			opts, err := options(kind, rest...)
			if err != nil {
				return esper.Query{}, err
			}
			return window(env).AsRecord().Select(esper.Alias("intPrimitive", intPrim)).Query(opts...), nil
		}
	}
	countOnly := func(kind esper.StatementHintKind, having bool, rest ...esper.QueryOption) func(*esper.Environment) (esper.Query, error) {
		return func(env *esper.Environment) (esper.Query, error) {
			opts, err := options(kind, rest...)
			if err != nil {
				return esper.Query{}, err
			}
			aggregated := window(env).Aggregate(esper.Alias("count(*)", count))
			if having {
				aggregated = aggregated.Having(esper.Greater[int64](count, esper.Literal(0)))
			}
			return aggregated.Query(opts...), nil
		}
	}
	stringCount := func(kind esper.StatementHintKind, having bool, rest ...esper.QueryOption) func(*esper.Environment) (esper.Query, error) {
		return func(env *esper.Environment) (esper.Query, error) {
			opts, err := options(kind, rest...)
			if err != nil {
				return esper.Query{}, err
			}
			aggregated := window(env).Aggregate(
				esper.Alias("theString", theString),
				esper.Alias("count(*)", count),
			)
			if having {
				aggregated = aggregated.Having(esper.Greater[int64](count, esper.Literal(0)))
			}
			return aggregated.Query(opts...), nil
		}
	}
	groupedCount := func(kind esper.StatementHintKind, withIntPrimitive bool, rest ...esper.QueryOption) func(*esper.Environment) (esper.Query, error) {
		return func(env *esper.Environment) (esper.Query, error) {
			opts, err := options(kind, rest...)
			if err != nil {
				return esper.Query{}, err
			}
			grouped := window(env).GroupBy(theString)
			if withIntPrimitive {
				grouped = grouped.Select(
					esper.Alias("theString", theString),
					esper.Alias("intPrimitive", intPrim),
					esper.Alias("count(*)", count),
				)
			} else {
				grouped = grouped.Select(
					esper.Alias("theString", theString),
					esper.Alias("count(*)", count),
				)
			}
			return grouped.Query(opts...), nil
		}
	}
	const (
		none    esper.StatementHintKind = ""
		enable                          = esper.HintEnableOutputLimitOptimization
		disable                         = esper.HintDisableOutputLimitOptimization
	)
	return []resultsetOutputLimitChangesetRound{
		{label: "plain-last", build: plain(none, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "plain-last-orderby", build: plain(none,
			esper.WithOutput(esper.OutputLastEveryTime(time.Second)),
			esper.OrderBy(esper.Ascending(intPrim)))},
		{label: "plain-all", build: plain(none, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "plain-all-enable", build: plain(enable, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "plain-all-disable", build: plain(disable, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "plain-first", build: plain(none, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},

		{label: "count-last", build: countOnly(none, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "count-last-enable", build: countOnly(enable, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "count-last-disable", build: countOnly(disable, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "count-all", build: countOnly(none, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "count-all-enable", build: countOnly(enable, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "count-all-disable", build: countOnly(disable, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "count-first", build: countOnly(none, false, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},
		{label: "count-first-having", build: countOnly(none, true, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},

		{label: "string-count-last", build: stringCount(none, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "string-count-last-enable", build: stringCount(enable, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "string-count-last-disable", build: stringCount(disable, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "string-count-all", build: stringCount(none, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "string-count-all-enable", build: stringCount(enable, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "string-count-all-disable", build: stringCount(disable, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "string-count-first", build: stringCount(none, false, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},
		{label: "string-count-first-having", build: stringCount(none, true, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},

		{label: "grouped-count-last", build: groupedCount(none, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "grouped-count-last-enable", build: groupedCount(enable, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "grouped-count-last-disable", build: groupedCount(disable, false, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "grouped-count-all", build: groupedCount(none, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "grouped-count-all-enable", build: groupedCount(enable, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "grouped-count-all-disable", build: groupedCount(disable, false, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "grouped-count-first", build: groupedCount(none, false, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},

		{label: "grouped-key-count-last", build: groupedCount(none, true, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "grouped-key-count-last-enable", build: groupedCount(enable, true, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "grouped-key-count-last-disable", build: groupedCount(disable, true, esper.WithOutput(esper.OutputLastEveryTime(time.Second)))},
		{label: "grouped-key-count-all", build: groupedCount(none, true, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "grouped-key-count-all-enable", build: groupedCount(enable, true, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "grouped-key-count-all-disable", build: groupedCount(disable, true, esper.WithOutput(esper.OutputAllEveryTime(time.Second)))},
		{label: "grouped-key-count-first", build: groupedCount(none, true, esper.WithOutput(esper.OutputFirstEveryTime(time.Second)))},
	}, nil
}

func runResultsetOutputLimitChangesetScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetOutputLimitChangesetScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	if err := assertResultsetOutputLimitChangesetCompileRejection(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitChangesetBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetOutputLimitChangesetJavaRuntimeIDs[0]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	builds, err := resultsetOutputLimitChangesetRoundBuilds()
	if err != nil {
		return compat.Trace{}, err
	}
	if len(builds) != resultsetOutputLimitChangesetRounds {
		return compat.Trace{}, fmt.Errorf("%s has %d pinned rounds, want %d", resultsetOutputLimitChangesetID, len(builds), resultsetOutputLimitChangesetRounds)
	}
	var sequence uint64
	offset := 1
	for round, definition := range builds {
		query, err := definition.build(env)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
		}
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
		}
		statements := deployment.Statements()
		if len(statements) != 1 || statements[0].Name() != "s0" {
			return compat.Trace{}, fmt.Errorf("%s round %d deployed unexpected statements", resultsetOutputLimitChangesetID, round)
		}
		statement := statements[0]
		for send := 0; send < resultsetOutputLimitChangesetSends; send++ {
			step := scenario.Steps[offset]
			offset++
			payload, err := decodeResultsetOutputLimitChangesetPayload(step)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
			}
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      resultsetOutputLimitChangesetCaseName,
			Operation: "changeset",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(engine.Now()),
			Value:     statement.NumChangesetRows(),
		})
		step := scenario.Steps[offset]
		offset++
		at, err := time.Parse(time.RFC3339Nano, step.At)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s round %d advance-time: %w", resultsetOutputLimitChangesetID, round, err)
		}
		if err := engine.AdvanceTime(ctx, at); err != nil {
			return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      resultsetOutputLimitChangesetCaseName,
			Operation: "changeset",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(engine.Now()),
			Value:     statement.NumChangesetRows(),
		})
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, fmt.Errorf("%s round %d: %w", resultsetOutputLimitChangesetID, round, err)
		}
	}
	return trace, nil
}

// assertResultsetOutputLimitChangesetCompileRejection mirrors the Java
// oracle's in-code assertion that the ENABLE_OUTPUTLIMIT_OPT hint cannot be
// combined with an order-by clause. It emits no trace record.
func assertResultsetOutputLimitChangesetCompileRejection() error {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetOutputLimitChangesetBean](env, "SupportBean"); err != nil {
		return err
	}
	hint, err := esper.NewStatementHint(esper.HintEnableOutputLimitOptimization)
	if err != nil {
		return err
	}
	query := esper.From[resultsetOutputLimitChangesetBean](env, "SupportBean").Query(
		esper.StatementName("s0"),
		esper.WithStatementHints(hint),
		esper.OrderBy(esper.Ascending(esper.Field[resultsetOutputLimitChangesetBean, string]("theString"))),
	)
	_, err = env.Build(query)
	if err == nil {
		return fmt.Errorf("%s: ENABLE_OUTPUTLIMIT_OPT with order-by was not rejected", resultsetOutputLimitChangesetID)
	}
	if !containsParityErrorText(err, "The ENABLE_OUTPUTLIMIT_OPT hint is not supported with order-by") {
		return fmt.Errorf("%s: order-by rejection error = %v", resultsetOutputLimitChangesetID, err)
	}
	return nil
}

func containsParityErrorText(err error, text string) bool {
	for current := err; current != nil; current = unwrapParityError(current) {
		if esperError, ok := current.(*esper.Error); ok && esperError.Message == text {
			return true
		}
	}
	return false
}

func unwrapParityError(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

func decodeResultsetOutputLimitChangesetPayload(step compat.Step) (any, error) {
	if step.EventType != "SupportBean" {
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetOutputLimitChangesetID, step.EventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return esper.Query{}, err
	}
	if err := requireResultsetOutputLimitChangesetFields(fields, "theString", "intPrimitive"); err != nil {
		return esper.Query{}, err
	}
	var value resultsetOutputLimitChangesetBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportBean: %w", err)
	}
	return value, nil
}

func requireResultsetOutputLimitChangesetFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetOutputLimitChangesetStringArray(raw json.RawMessage, expected []string, name string) error {
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
