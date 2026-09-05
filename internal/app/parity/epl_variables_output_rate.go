package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	eplVariableOutputRateID          = "epl-variable-output-rate"
	eplVariableOutputRateDescription = "EPLVariablesOutputRate variable-driven output rates: event-count 'output last every var_output_limit events' across plain, SODA-model, and epl-to-model compile-deploy forms plus time-based 'output snapshot every var_output_limit seconds' with on-set variable reassignment and the null-rate evaluation failure."
	eplVariableOutputRateJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplVariableOutputRateSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesOutputRate.java"

	eplVariableOutputRateOnSetEPL = "on SupportMarketDataBean set var_output_limit = volume"
)

var (
	eplVariableOutputRateJavaSources = []string{
		eplVariableOutputRateSource,
	}
	eplVariableOutputRateJavaRuntimeIDs = []string{
		"java-runtime-0b028b6fe8bddbbb6480",
		"java-runtime-0539523182c81174c7f6",
		"java-runtime-303a9b5ee74d52e62125",
		"java-runtime-4909a80e9612b815310e",
	}
	eplVariableOutputRateJavaExecutions = []string{
		"EPLVariableOutputRateEventsAll",
		"EPLVariableOutputRateEventsAllOM",
		"EPLVariableOutputRateEventsAllCompile",
		"EPLVariableOutputRateTimeAll",
	}
	eplVariableOutputRateJavaStaticIDs = []string{
		"java-b8a2d4466ca4d002fa69",
		"java-b08aeecf9371278c24f6",
		"java-9ee028b9ce2729673e8e",
		"java-b3199a3fee2738477788",
	}
	eplVariableOutputRateCases = []string{
		"events",
		"events-om",
		"events-compile",
		"time-snapshot",
	}
	eplVariableOutputRateOrdinals = []int{0, 1, 2, 3}
	eplVariableOutputRateEPLs     = []string{
		"@name('s0') select count(*) as cnt from SupportBean output last every var_output_limit events",
		"@name('s0') select count(*) as cnt from SupportBean output last every var_output_limit events",
		"@name('s0') select count(*) as cnt from SupportBean output last every var_output_limit events",
		"@name('s0') select count(*) as cnt from SupportBean output snapshot every var_output_limit seconds",
	}
)

type eplVariableOutputRateBean struct {
	TheString string `esper:"theString"`
}

type eplVariableOutputRateMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   string  `esper:"feed"`
}

func loadEplVariableOutputRateScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplVariableOutputRateID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplVariableOutputRateID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariableOutputRateID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplVariableOutputRateID, err)
	}
	if err := requireEplVariableOutputRateFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplVariableOutputRateID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplVariableOutputRateID ||
		metadata.Description != eplVariableOutputRateDescription ||
		metadata.JavaCommit != eplVariableOutputRateJavaCommit ||
		metadata.JavaSource != eplVariableOutputRateSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplVariableOutputRateID)
	}
	if err := validateEplVariableOutputRateStringArray(root["javaRuntimes"], eplVariableOutputRateJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariableOutputRateStringArray(root["javaNames"], eplVariableOutputRateJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariableOutputRateStringArray(root["javaStaticIds"], eplVariableOutputRateJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplVariableOutputRateStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplVariableOutputRateCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", eplVariableOutputRateID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplVariableOutputRateFields(object,
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
		if definition.Case != eplVariableOutputRateCases[index] ||
			definition.Ordinal != eplVariableOutputRateOrdinals[index] ||
			definition.RuntimeID != eplVariableOutputRateJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplVariableOutputRateJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != eplVariableOutputRateEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplVariableOutputRateID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplVariableOutputRateID)
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
			if err := requireEplVariableOutputRateFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "set-variable":
			if err := requireEplVariableOutputRateFields(object, "op", "case", "name", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplVariableOutputRateVariableValue(object["payload"]); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplVariableOutputRateFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != "s0" && step.Statement != "s1" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q", index, step.Statement)
			}
		case "send":
			if err := requireEplVariableOutputRateFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplVariableOutputRatePayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time":
			if err := requireEplVariableOutputRateFields(object, "op", "case", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "advance-time-error":
			if err := requireEplVariableOutputRateFields(object, "op", "case", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplVariableOutputRateFields(object, "op", "case"); err != nil {
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
	if err := validateEplVariableOutputRateScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplVariableOutputRateStepKind describes one pinned scenario step in the
// frozen timelines.
type eplVariableOutputRateStepKind struct {
	op        string
	statement string
	event     string
	bean      string // SupportBean theString; empty for market steps
	volume    *int64 // market payload volume; nil value with marketVolumeSet
	market    bool
	at        string
}

func eplVariableOutputRateLong(v int64) *int64 { return &v }

// eplVariableOutputRateEventsSteps pins the shared events timeline: rate 3
// fires {cnt:3} on E3; the on-set statement moves the rate to 5 (E8 fires
// {cnt:8}), 2 (E10 fires {cnt:10}), and 1 (E11/E12 fire); null keeps rate 1
// so E13 fires {cnt:13}.
func eplVariableOutputRateEventsSteps() []eplVariableOutputRateStepKind {
	return []eplVariableOutputRateStepKind{
		{op: "set-variable"},
		{op: "deploy", statement: "s0"},
		{op: "send", event: "SupportBean", bean: "E1"},
		{op: "send", event: "SupportBean", bean: "E2"},
		{op: "send", event: "SupportBean", bean: "E3"},
		{op: "deploy", statement: "s1"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(5)},
		{op: "send", event: "SupportBean", bean: "E4"},
		{op: "send", event: "SupportBean", bean: "E5"},
		{op: "send", event: "SupportBean", bean: "E6"},
		{op: "send", event: "SupportBean", bean: "E7"},
		{op: "send", event: "SupportBean", bean: "E8"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(2)},
		{op: "send", event: "SupportBean", bean: "E9"},
		{op: "send", event: "SupportBean", bean: "E10"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(1)},
		{op: "send", event: "SupportBean", bean: "E11"},
		{op: "send", event: "SupportBean", bean: "E12"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: nil},
		{op: "send", event: "SupportBean", bean: "E13"},
		{op: "undeploy-all"},
	}
}

// eplVariableOutputRateTimeSteps pins the snapshot timeline: outputs at 3s
// {cnt:2}, 4s {cnt:4}, 5s {cnt:4}, 8s {cnt:4}, 12s {cnt:6}; the null set at
// 13999 makes the 14000 schedule advance fail before any dispatch.
func eplVariableOutputRateTimeSteps() []eplVariableOutputRateStepKind {
	return []eplVariableOutputRateStepKind{
		{op: "set-variable"},
		{op: "advance-time", at: "1970-01-01T00:00:00Z"},
		{op: "deploy", statement: "s0"},
		{op: "send", event: "SupportBean", bean: "E1"},
		{op: "send", event: "SupportBean", bean: "E2"},
		{op: "advance-time", at: "1970-01-01T00:00:02.999Z"},
		{op: "advance-time", at: "1970-01-01T00:00:03Z"},
		{op: "deploy", statement: "s1"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(5)},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(1)},
		{op: "advance-time", at: "1970-01-01T00:00:03.200Z"},
		{op: "send", event: "SupportBean", bean: "E3"},
		{op: "send", event: "SupportBean", bean: "E4"},
		{op: "advance-time", at: "1970-01-01T00:00:03.999Z"},
		{op: "advance-time", at: "1970-01-01T00:00:04Z"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(4)},
		{op: "advance-time", at: "1970-01-01T00:00:04.999Z"},
		{op: "advance-time", at: "1970-01-01T00:00:05Z"},
		{op: "advance-time", at: "1970-01-01T00:00:07.999Z"},
		{op: "advance-time", at: "1970-01-01T00:00:08Z"},
		{op: "send", event: "SupportBean", bean: "E5"},
		{op: "send", event: "SupportBean", bean: "E6"},
		{op: "advance-time", at: "1970-01-01T00:00:11.999Z"},
		{op: "advance-time", at: "1970-01-01T00:00:12Z"},
		{op: "advance-time", at: "1970-01-01T00:00:13Z"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: eplVariableOutputRateLong(2)},
		{op: "send", event: "SupportBean", bean: "E7"},
		{op: "send", event: "SupportBean", bean: "E8"},
		{op: "advance-time", at: "1970-01-01T00:00:13.999Z"},
		{op: "send", event: "SupportMarketDataBean", market: true, volume: nil},
		{op: "advance-time-error", at: "1970-01-01T00:00:14Z"},
		{op: "undeploy-all"},
	}
}

func validateEplVariableOutputRateScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplVariableOutputRateID {
		return fmt.Errorf("%s scenario shape is not pinned", eplVariableOutputRateID)
	}
	offset := 0
	for _, caseName := range eplVariableOutputRateCases {
		steps := scenario.Steps[offset:]
		expected := eplVariableOutputRateEventsSteps()
		label := "events"
		if caseName == "time-snapshot" {
			expected = eplVariableOutputRateTimeSteps()
			label = "time-snapshot"
		}
		if len(steps) < len(expected)+1 {
			return fmt.Errorf("%s case %q steps are truncated", eplVariableOutputRateID, caseName)
		}
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", eplVariableOutputRateID, caseName)
		}
		if err := validateEplVariableOutputRateCaseSteps(steps[1:len(expected)+1], expected, caseName, label); err != nil {
			return err
		}
		offset += len(expected) + 1
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplVariableOutputRateID)
	}
	return nil
}

func validateEplVariableOutputRateCaseSteps(steps []compat.Step, expected []eplVariableOutputRateStepKind, caseName, label string) error {
	for index, want := range expected {
		step := steps[index]
		if step.Op != want.op || step.Case != caseName {
			return fmt.Errorf("%s case %q step %d must be op %q", label, caseName, index, want.op)
		}
		switch want.op {
		case "set-variable":
			if step.Name != "var_output_limit" {
				return fmt.Errorf("%s case %q step %d must set var_output_limit", label, caseName, index)
			}
			value, err := decodeEplVariableOutputRateVariableValue(step.Payload)
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", label, caseName, index, err)
			}
			if value != 3 {
				return fmt.Errorf("%s case %q step %d must reset the variable to 3", label, caseName, index)
			}
		case "deploy":
			if want.statement != step.Statement {
				return fmt.Errorf("%s case %q step %d must deploy %s", label, caseName, index, want.statement)
			}
			if want.statement == "s0" && step.Epl != eplVariableOutputRateEPLs[eplVariableOutputRateCaseIndex(caseName)] {
				return fmt.Errorf("%s case %q step %d must deploy the pinned select EPL", label, caseName, index)
			}
			if want.statement == "s1" && step.Epl != eplVariableOutputRateOnSetEPL {
				return fmt.Errorf("%s case %q step %d must deploy the on-set statement", label, caseName, index)
			}
		case "send":
			if step.EventType != want.event {
				return fmt.Errorf("%s case %q step %d must send %s", label, caseName, index, want.event)
			}
			var payload compat.Step
			raw, err := json.Marshal(step)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &payload); err != nil {
				return err
			}
			decoded, err := decodeEplVariableOutputRatePayload(payload)
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", label, caseName, index, err)
			}
			if !want.market {
				bean, ok := decoded.(eplVariableOutputRateBean)
				if !ok || bean.TheString != want.bean {
					return fmt.Errorf("%s case %q step %d must send %s", label, caseName, index, want.bean)
				}
			} else {
				market, ok := decoded.(eplVariableOutputRateMarket)
				if !ok {
					return fmt.Errorf("%s case %q step %d must send SupportMarketDataBean", label, caseName, index)
				}
				if want.volume == nil {
					if market.Volume != nil {
						return fmt.Errorf("%s case %q step %d must send a null volume", label, caseName, index)
					}
				} else if market.Volume == nil || *market.Volume != *want.volume {
					return fmt.Errorf("%s case %q step %d must send volume %d", label, caseName, index, *want.volume)
				}
			}
		case "advance-time", "advance-time-error":
			if step.At != want.at {
				return fmt.Errorf("%s case %q step %d must target %s", label, caseName, index, want.at)
			}
		case "undeploy-all":
		}
	}
	return nil
}

func eplVariableOutputRateCaseIndex(caseName string) int {
	for index, candidate := range eplVariableOutputRateCases {
		if candidate == caseName {
			return index
		}
	}
	return 0
}

func runEplVariableOutputRateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplVariableOutputRateScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplVariableOutputRateBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplVariableOutputRateMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if err := env.RegisterVariable("var_output_limit", int64(3), esper.VariableType(reflect.TypeOf(int64(0)))); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplVariableOutputRateJavaRuntimeIDs[0]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	// selectCountQuery pins the count(*) select shared by all four cases;
	// the rate expression is the only difference between the families.
	selectCountQuery := func(caseIndex int) esper.Query {
		aggregated := esper.From[eplVariableOutputRateBean](env, "SupportBean").Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		)
		options := []esper.QueryOption{esper.StatementName("s0")}
		if eplVariableOutputRateCases[caseIndex] == "time-snapshot" {
			options = append(options, esper.WithOutput(esper.OutputSnapshotEveryExpr(esper.VariableRef[int64]("var_output_limit"))))
		} else {
			options = append(options, esper.WithOutput(esper.OutputLastEveryEventsExpr(esper.VariableRef[int64]("var_output_limit"))))
		}
		return aggregated.Query(options...)
	}
	onSetQuery := func() esper.Query {
		volume := esper.Cast[*int64, int64](esper.Field[eplVariableOutputRateMarket, *int64]("volume"))
		return esper.OnEvent(esper.From[eplVariableOutputRateMarket](env, "SupportMarketDataBean")).SetVariables(
			esper.SetVariableExpr("var_output_limit", volume),
		).Query(esper.StatementName("s1"))
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for _, caseName := range eplVariableOutputRateCases {
		caseIndex := eplVariableOutputRateCaseIndex(caseName)
		selectPlan, err := env.Build(selectCountQuery(caseIndex))
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplVariableOutputRateID, caseName, err)
		}
		onSetPlan, err := env.Build(onSetQuery())
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplVariableOutputRateID, caseName, err)
		}
		sequence := uint64(0)
		var deployments []*esper.Deployment
		var s0 *esper.Statement
		recordListener := func(caseName string, batch esper.ResultBatch) {
			sequence++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: "s0",
				Sequence:  sequence,
				Time:      compat.FormatTraceTime(batch.Time),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
		}
		stepErr := func(step int, err error) error {
			return fmt.Errorf("%s case %q step %d: %w", eplVariableOutputRateID, caseName, step, err)
		}
		steps := 0
		for {
			step := scenario.Steps[offset]
			if step.Op == "case" {
				offset++
				continue
			}
			if step.Op == "undeploy-all" {
				offset++
				break
			}
			offset++
			steps++
			switch step.Op {
			case "set-variable":
				value, err := decodeEplVariableOutputRateVariableValue(step.Payload)
				if err != nil {
					return compat.Trace{}, stepErr(steps, err)
				}
				if err := engine.SetVariable(ctx, step.Name, value); err != nil {
					return compat.Trace{}, stepErr(steps, err)
				}
			case "deploy":
				if step.Statement == "s0" {
					deployment, err := engine.Deploy(ctx, selectPlan)
					if err != nil {
						return compat.Trace{}, stepErr(steps, err)
					}
					deployed := deployment.Statements()
					if len(deployed) != 1 || deployed[0].Name() != "s0" {
						return compat.Trace{}, fmt.Errorf("%s case %q deployed unexpected select statements", eplVariableOutputRateID, caseName)
					}
					s0 = deployed[0]
					if _, err := s0.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						recordListener(caseName, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, stepErr(steps, err)
					}
					deployments = append(deployments, deployment)
				} else {
					deployment, err := engine.Deploy(ctx, onSetPlan)
					if err != nil {
						return compat.Trace{}, stepErr(steps, err)
					}
					deployments = append(deployments, deployment)
				}
			case "send":
				payload, err := decodeEplVariableOutputRatePayload(step)
				if err != nil {
					return compat.Trace{}, stepErr(steps, err)
				}
				if err := engine.Send(ctx, step.EventType, payload); err != nil {
					return compat.Trace{}, stepErr(steps, err)
				}
			case "advance-time":
				at, err := time.Parse(time.RFC3339Nano, step.At)
				if err != nil {
					return compat.Trace{}, stepErr(steps, fmt.Errorf("advance-time: %w", err))
				}
				if err := engine.AdvanceTime(ctx, at); err != nil {
					return compat.Trace{}, stepErr(steps, err)
				}
			case "advance-time-error":
				at, err := time.Parse(time.RFC3339Nano, step.At)
				if err != nil {
					return compat.Trace{}, stepErr(steps, fmt.Errorf("advance-time-error: %w", err))
				}
				advanceErr := engine.AdvanceTime(ctx, at)
				if advanceErr == nil {
					return compat.Trace{}, stepErr(steps, fmt.Errorf("expected the null-rate schedule advance to fail"))
				}
				sequence++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "advance-time-error",
					Statement: "s0",
					Sequence:  sequence,
					Time:      compat.FormatTraceTime(at),
					Value:     parityErrorMessage(advanceErr),
				})
			default:
				return compat.Trace{}, fmt.Errorf("%s case %q has unsupported op %q", eplVariableOutputRateID, caseName, step.Op)
			}
		}
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, fmt.Errorf("%s case %q undeploy: %w", eplVariableOutputRateID, caseName, err)
			}
		}
		s0 = nil
	}
	return trace, nil
}

// parityErrorMessage extracts the bare engine message, mirroring the Java
// trace's exception-message value.
func parityErrorMessage(err error) string {
	if esperErr, ok := err.(*esper.Error); ok {
		return esperErr.Message
	}
	return err.Error()
}

func decodeEplVariableOutputRateVariableValue(payload json.RawMessage) (int64, error) {
	var assignment struct {
		Type  string `json:"type"`
		Value int64  `json:"value"`
	}
	var fields map[string]json.RawMessage
	if err := strictObject(payload, &fields); err != nil {
		return 0, err
	}
	if err := requireEplVariableOutputRateFields(fields, "type", "value"); err != nil {
		return 0, err
	}
	if err := json.Unmarshal(payload, &assignment); err != nil {
		return 0, err
	}
	if assignment.Type != "long" {
		return 0, fmt.Errorf("variable payload type %q is not long", assignment.Type)
	}
	return assignment.Value, nil
}

func decodeEplVariableOutputRatePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireEplVariableOutputRateFields(fields, "theString"); err != nil {
			return nil, err
		}
		var value eplVariableOutputRateBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportMarketDataBean":
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return nil, err
		}
		if err := requireEplVariableOutputRateFields(fields, "symbol", "price", "volume", "feed"); err != nil {
			return nil, err
		}
		var value eplVariableOutputRateMarket
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplVariableOutputRateID, step.EventType)
	}
}

func requireEplVariableOutputRateFields(object map[string]json.RawMessage, names ...string) error {
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

func validateEplVariableOutputRateStringArray(raw json.RawMessage, expected []string, name string) error {
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
