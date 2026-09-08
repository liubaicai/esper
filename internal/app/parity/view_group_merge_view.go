package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for ViewGroup merge-view semantics: the groupwin parent is
// a merge/union point whose downstream sees the union of all groups' subview
// contents, so an ordinary select-clause aggregate (sum(p2) with no group-by)
// stays one ungrouped aggregate over that union (Java ord 0: 10/21/33/36
// with in-group eviction subtracted), while grouped-view retention delivers
// per-group irstream pairs with in-group eviction (Java ord 14).
var viewGroupMergeViewJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewGroup.java",
}

type viewGroupMergeBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

var (
	viewGroupMergeViewJavaRuntimeIDs = []string{
		"java-runtime-a3b6bef89e22a122cc7a", // ViewGroupObjectArrayEvent
		"java-runtime-639bc9b69621f3b6a417", // ViewGroupLengthWin
	}
	viewGroupMergeViewJavaExecutions = []string{
		"ViewGroupObjectArrayEvent",
		"ViewGroupLengthWin",
	}
	viewGroupMergeViewCases = []string{
		"merge-view-union-aggregate",
		"length-win-groups",
	}
)

func runViewGroupMergeViewScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	spans := map[string]int{
		"merge-view-union-aggregate": 5,
		"length-win-groups":          9,
	}
	for _, caseName := range viewGroupMergeViewCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseName]]
		offset += spans[caseName]
		caseTrace, err := runViewGroupMergeViewCase(ctx, caseScenarioFor(scenario, caseName, caseSteps), caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("viewgroup-merge-view case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("viewgroup-merge-view scenario %q has trailing steps", scenario.ID)
	}
	return trace, nil
}

func caseScenarioFor(scenario compat.Scenario, caseName string, steps []compat.Step) compat.Scenario {
	return compat.Scenario{Version: scenario.Version, ID: scenario.ID, Steps: steps}
}

func runViewGroupMergeViewCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()

	var plans []struct {
		name     string
		plan     esper.Plan
		listener bool
	}
	build := func(name string, listener bool, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, struct {
			name     string
			plan     esper.Plan
			listener bool
		}{name, plan, listener})
		return nil
	}

	switch caseName {
	case "merge-view-union-aggregate":
		// Java ord 0 registers OAEventStringInt as an object-array event type
		// {p1: String, p2: int}.
		if _, err := esper.RegisterObjectArray(env, "OAEventStringInt", []esper.FieldSpec{
			esper.FieldDef("p1", reflect.TypeOf("")),
			esper.FieldDef("p2", reflect.TypeOf(0)),
		}); err != nil {
			return compat.Trace{}, err
		}
		p1 := esper.Field[map[string]any, string]("p1")
		p2 := esper.Field[map[string]any, int]("p2")
		err := build("s0", true, esper.FromAny(env, "OAEventStringInt").Window(
			esper.GroupWindow(p1, esper.LengthWindow(2)),
		).Aggregate(
			esper.Alias("p1", p1),
			esper.Alias("sp2", esper.Sum[int](p2)),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return compat.Trace{}, err
		}
	case "length-win-groups":
		if _, err := esper.RegisterStruct[viewGroupMergeBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		theString := esper.Field[viewGroupMergeBean, string]("theString")
		intPrimitive := esper.Field[viewGroupMergeBean, int]("intPrimitive")
		err := build("s0", true, esper.Select(
			esper.From[viewGroupMergeBean](env, "SupportBean").Window(
				esper.GroupWindow(theString, esper.LengthWindow(3))),
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported viewgroup-merge-view case %q", caseName)
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		viewGroupMergeViewJavaRuntimeIDs[indexOfCase(viewGroupMergeViewCases, caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	seq := uint64(0)
	for _, item := range plans {
		deployment, err := engine.Deploy(ctx, item.plan)
		if err != nil {
			return trace, err
		}
		if !item.listener {
			continue
		}
		st := deployment.Statements()[0]
		if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			seq++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: st.Name(),
				Time:      batch.Time.UTC().Format(time.RFC3339),
				Sequence:  seq,
			}
			record.New = compat.NormalizeResults(batch.New)
			record.Old = compat.NormalizeResults(batch.Old)
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return trace, err
		}
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("viewgroup-merge-view decode %s: %w", step.EventType, err)
			}
			var underlying any = payload
			if step.EventType == "OAEventStringInt" {
				// Object-array events send positional underlyings.
				underlying = []any{
					payload["p1"],
					int(payload["p2"].(float64)),
				}
			}
			if err := engine.Send(ctx, step.EventType, underlying); err != nil {
				return trace, err
			}
		case "snapshot":
			return trace, fmt.Errorf("snapshot op is not supported by this runner (ord 6 deferred)")
		default:
			return trace, fmt.Errorf("unsupported viewgroup-merge-view step op %q", step.Op)
		}
	}
	return trace, nil
}

func indexOfCase(cases []string, wanted string) int {
	for index, name := range cases {
		if name == wanted {
			return index
		}
	}
	return -1
}
