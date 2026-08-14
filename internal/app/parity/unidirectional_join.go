package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type unidirectionalMarketBean struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

type unidirectionalSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

const unidirectionalJoinCaseGrouped = "grouped"

const unidirectionalJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var unidirectionalJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/join/EPLJoinUnidirectionalStream.java",
}

var (
	unidirectionalJoinJavaRuntimeIDs = []string{
		"java-runtime-b89b3e49bf5d655c3bb6",
	}
	unidirectionalJoinJavaExecutions = []string{
		"EPLJoin2TableJoinGrouped",
	}
)

// runUnidirectionalJoinScenario replays the transient market-data driver
// against retained SupportBean rows and captures grouped irstream output.
func runUnidirectionalJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if !scenarioHasCase(scenario, unidirectionalJoinCaseGrouped) {
		return compat.Trace{}, fmt.Errorf("unidirectional join scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, unidirectionalJoinCaseGrouped)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[unidirectionalMarketBean](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[unidirectionalSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	query := esper.Join(
		esper.From[unidirectionalMarketBean](env, "SupportMarketDataBean"),
		esper.From[unidirectionalSupportBean](env, "SupportBean").Window(esper.KeepAll()),
		esper.OnEqual(
			esper.Field[unidirectionalMarketBean, string]("symbol"),
			esper.Field[unidirectionalSupportBean, string]("theString"),
		),
	).Unidirectional(esper.JoinLeft).GroupBy(
		esper.JoinField[string](0, "symbol"),
		esper.JoinField[string](1, "theString"),
	).Select(
		esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
		esper.Alias("cnt", esper.CountAll()),
	).Query(
		esper.StatementName("s0"),
		esper.WithOldStream(),
	)
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeUnidirectionalJoinPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown unidirectional join statement %q", name)
		}
		return statement, nil
	})
}

func decodeUnidirectionalJoinPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportMarketDataBean":
		var value unidirectionalMarketBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value unidirectionalSupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported unidirectional join event type %q", step.EventType)
	}
}
