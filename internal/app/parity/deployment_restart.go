package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type deploymentRestartMarket struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

const deploymentRestartJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var deploymentRestartJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/join/EPLJoinStartStop.java",
}

var (
	deploymentRestartJavaRuntimeIDs = []string{
		"java-runtime-4ae08580480e15494fea",
	}
	deploymentRestartJavaExecutions = []string{
		"EPLJoinStartStopSceneOne",
	}
)

func runDeploymentRestartScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{"first", "second", "third"}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runDeploymentRestartCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("deployment restart case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("deployment restart scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runDeploymentRestartCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[deploymentRestartMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	ibm := esper.From[deploymentRestartMarket](env, "SupportMarketDataBean").Filter(
		esper.Equal[string](esper.Field[deploymentRestartMarket, string]("symbol"), esper.Literal("IBM")),
	).Window(esper.LengthWindow(3))
	csco := esper.From[deploymentRestartMarket](env, "SupportMarketDataBean").Filter(
		esper.Equal[string](esper.Field[deploymentRestartMarket, string]("symbol"), esper.Literal("CSCO")),
	).Window(esper.LengthWindow(3))
	query := esper.Join(
		ibm,
		csco,
		esper.OnEqual(
			esper.Field[deploymentRestartMarket, int64]("volume"),
			esper.Field[deploymentRestartMarket, int64]("volume"),
		),
	).Select(
		esper.SelectLeft("s0volume", esper.Field[deploymentRestartMarket, int64]("volume")),
		esper.SelectRight("s1volume", esper.Field[deploymentRestartMarket, int64]("volume")),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeDeploymentRestartPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown deployment restart statement %q", name)
		}
		return statement, nil
	})
}

func decodeDeploymentRestartPayload(step compat.Step) (any, error) {
	var value deploymentRestartMarket
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
	}
	return value, nil
}
