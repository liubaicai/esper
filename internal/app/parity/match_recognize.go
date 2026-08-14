package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type matchRecognizeBean struct {
	TheString string `esper:"theString"`
	Value     int    `esper:"value"`
}

const matchRecognizeJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var matchRecognizeJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogOps.java",
}

var (
	matchRecognizeJavaRuntimeIDs = []string{
		"java-runtime-ce4e140777e93d5226b5",
	}
	matchRecognizeJavaExecutions = []string{
		"RowRecogConcatenation",
	}
)

func runMatchRecognizeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseName := "match"
	if !scenarioHasCase(scenario, caseName) {
		return compat.Trace{}, fmt.Errorf("match recognize scenario %q has no supported cases", scenario.ID)
	}
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[matchRecognizeBean](env, "SupportRecogBean"); err != nil {
		return compat.Trace{}, err
	}
	stream := esper.From[matchRecognizeBean](env, "SupportRecogBean").Window(esper.KeepAll())
	query := stream.MatchRecognize(esper.RowSequence(esper.RowVar("A"), esper.RowVar("B"))).
		Define("B", esper.Greater[int](esper.TagField[int]("B", "value"), esper.TagField[int]("A", "value"))).
		Measures(
			esper.Alias("a_string", esper.TagField[string]("A", "theString")),
			esper.Alias("b_string", esper.TagField[string]("B", "theString")),
		).Query(
		esper.StatementName("s0"),
		esper.OrderBy(
			esper.Ascending(esper.ResultField[string]("a_string")),
			esper.Ascending(esper.ResultField[string]("b_string")),
		),
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

	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeMatchRecognizePayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown match recognize statement %q", name)
		}
		return statement, nil
	})
}

func decodeMatchRecognizePayload(step compat.Step) (any, error) {
	var value matchRecognizeBean
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return nil, fmt.Errorf("decode SupportRecogBean: %w", err)
	}
	return value, nil
}
