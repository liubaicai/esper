package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLOtherSelectWildcardWAdditional.
// Approved differences: Java Pair underlying vs Go flat rows; SODA OM has no
// Go counterpart; InvalidRepeatedProperties message text unasserted.
var eplOtherWildcardAdditionalJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectWildcardWAdditional.java",
}

type wcwSimple struct {
	MyString string `esper:"myString"`
	MyInt    int32  `esper:"myInt"`
}

type wcwMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type wcwBeanA struct {
	ID string `esper:"id"`
}

type wcwBeanB struct {
	ID string `esper:"id"`
}

type wcwMapEvent struct {
	TheString string `esper:"theString"`
	Int       int32  `esper:"int"`
}

var eplOtherWildcardAdditionalJavaRuntimeIDs = []string{
	"java-runtime-0d7fa958a52fab1f4f47",
	"java-runtime-e9aa163116193bbd7106",
	"java-runtime-33e95a630f502ba99b94",
}

var eplOtherWildcardAdditionalJavaExecutions = []string{
	"EPLOtherSingle",
	"EPLOtherWildcardMapEvent",
	"EPLOtherInvalidRepeatedProperties",
}

func runEplOtherWildcardAdditionalScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range []string{"single", "wildcard-map", "invalid-repeated"} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runEplOtherWildcardAdditionalCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runEplOtherWildcardAdditionalCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[wcwSimple](env, "SupportBeanSimple"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[wcwMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if caseName == "wildcard-map" {
		if _, err := esper.RegisterStruct[wcwMapEvent](env, "MyMapEventIntString"); err != nil {
			return compat.Trace{}, err
		}
	}

	var plans []esper.Plan
	build := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	}

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	switch caseName {
	case "single":
		ms := esper.Field[wcwSimple, string]("myString")
		mi := esper.Field[wcwSimple, int32]("myInt")
		err = build(esper.Select(
			esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5)),
			esper.Alias("myString", ms),
			esper.Alias("myInt", mi),
			esper.Alias("concat", esper.Concat(ms, ms)),
		).Query(esper.StatementName("s0")))
	case "wildcard-map":
		mapStr := esper.Field[wcwMapEvent, string]("theString")
		mapInt := esper.Field[wcwMapEvent, int32]("int")
		err = build(esper.Select(
			esper.From[wcwMapEvent](env, "MyMapEventIntString").Window(esper.LengthWindow(5)),
			esper.Alias("theString", mapStr),
			esper.Alias("int", mapInt),
			esper.Alias("concat", esper.Concat(mapStr, mapStr)),
		).Query(esper.StatementName("s0")))
	case "invalid-repeated":
		ms := esper.Field[wcwSimple, string]("myString")
		mi := esper.Field[wcwSimple, int32]("myInt")
		_, buildErr := env.Build(esper.Select(
			esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5)),
			esper.Alias("myString", ms),
			esper.Alias("myInt", mi),
			esper.Alias("myString", esper.Concat(ms, ms)),
		).Query(esper.StatementName("s0")))
		if buildErr == nil {
			return trace, fmt.Errorf("duplicate column unexpectedly built")
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "deployed",
			Statement: "invalid",
		})
		return trace, nil
	default:
		return compat.Trace{}, fmt.Errorf("unsupported case %q", caseName)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	seq := uint64(0)
	for _, plan := range plans {
		deployment, deployErr := engine.Deploy(ctx, plan)
		if deployErr != nil {
			return trace, deployErr
		}
		for _, st := range deployment.Statements() {
			st := st
			if st.Name() != "s0" {
				continue
			}
			if _, subErr := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				seq++
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: st.Name(),
					Sequence:  seq,
					Time:      "1970-01-01T00:00:00Z",
				}
				record.New = compat.NormalizeResults(batch.New)
				record.Old = compat.NormalizeResults(batch.Old)
				trace.Records = append(trace.Records, record)
				return nil
			}); subErr != nil {
				return trace, subErr
			}
		}
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, err
			}
			if sendErr := engine.SendRecord(ctx, step.EventType, payload); sendErr != nil {
				return trace, sendErr
			}
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}
