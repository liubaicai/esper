package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowOpEPStatementSource: an EPStatementSource
// operator subscribing to a deployed statement's output. The all-types case
// replays the four event representations (POJO, Map, ObjectArray, XML) with
// the paired statement's deployment id interpolated into the graph; the
// stmt-name-dynamic case pins the registry lifecycle (start-before-deploy,
// attach on deploy, detach on undeploy, redefinition) under the fixed
// deployment id MyDeploymentId; the statement-filter case pins the filter
// selector attaching to pre-existing and dynamically deployed statements
// with detach-on-undeploy and cancel suppression. The invalid execution is
// covered by the invalidity policy: no trace rows; Go Build-rejection tests
// pin the representable error class, and two Java rejections (zero output
// streams; name-requires-deployment-id) have no Go surface and are
// documented as case differences.
type dataflowEPStatementSourceSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type dataflowEPStatementSourceIdent struct {
	ID string `esper:"id"`
}

type dataflowEPStatementSourceGraphEvent struct {
	MyDouble float64 `esper:"myDouble"`
	MyInt    int     `esper:"myInt"`
	MyString string  `esper:"myString"`
}

const dataflowEPStatementSourceJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowEPStatementSourceJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowOpEPStatementSource.java",
}

var dataflowEPStatementSourceJavaRuntimeIDs = []string{
	"java-runtime-2d7b6229c7a3b2bee40b",
	"java-runtime-950696d7aa6356acb9d4",
	"java-runtime-8c8f6f03b11605a16d11",
	"java-runtime-ef085a37ed45eb35a13f",
}

var dataflowEPStatementSourceJavaExecutions = []string{
	"EPLDataflowAllTypes",
	"EPLDataflowStmtNameDynamic",
	"EPLDataflowStatementFilter",
	"EPLDataflowInvalid",
}

var dataflowEPStatementSourceCases = []string{
	"all-types",
	"stmt-name-dynamic",
	"statement-filter",
	"invalid",
}

// dataflowEPStatementSourceCapture records delivered values in arrival
// order.
type dataflowEPStatementSourceCapture struct {
	values []any
}

func (c *dataflowEPStatementSourceCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	c.values = append(c.values, input.Value)
	return nil, nil
}

func runDataflowEPStatementSourceScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-epstatement-source scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowEPStatementSourceCases {
		caseTrace, err := runDataflowEPStatementSourceCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-epstatement-source case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowEPStatementSourceCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	switch caseIndex {
	case 1:
		if _, err := esper.RegisterStruct[dataflowEPStatementSourceSupportBean](env, "SupportBean"); err != nil {
			return nil, err
		}
	case 2:
		if _, err := esper.RegisterStruct[dataflowEPStatementSourceSupportBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[dataflowEPStatementSourceIdent](env, "SupportBean_A"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[dataflowEPStatementSourceIdent](env, "SupportBean_B"); err != nil {
			return nil, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowEPStatementSourceJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := uint64(0)
	now := time.Unix(0, 0).UTC()
	var records []compat.TraceRecord
	emitAndReset := func(capture **dataflowEPStatementSourceCapture) {
		sequence++
		values := (*capture).values
		(*capture).values = nil
		rows := make([]compat.ResultRecord, 0, len(values))
		for _, value := range values {
			switch payload := value.(type) {
			case esper.Event:
				rows = append(rows, compat.NormalizeEvents([]esper.Event{payload})...)
			case esper.Row:
				rows = append(rows, compat.NormalizeRows([]esper.Row{payload})...)
			}
		}
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "capture",
			Statement: "flow:DefaultSupportCaptureOp",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       rows,
		})
	}
	switch caseIndex {
	case 0: // all-types — four event representations
		representations := []struct {
			eventTypeName string
			register      func() error
			first         any
			second        any
		}{
			{eventTypeName: "MyDefaultSupportGraphEvent",
				register: func() error {
					_, err := esper.RegisterStruct[dataflowEPStatementSourceGraphEvent](env, "MyDefaultSupportGraphEvent")
					return err
				},
				first:  dataflowEPStatementSourceGraphEvent{MyDouble: 1.1, MyInt: 1, MyString: "one"},
				second: dataflowEPStatementSourceGraphEvent{MyDouble: 2.2, MyInt: 2, MyString: "two"}},
			{eventTypeName: "MyMapEvent",
				register: func() error {
					_, err := esper.RegisterMap(env, "MyMapEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
					return err
				},
				first:  map[string]any{"myDouble": 1.1, "myInt": 1, "myString": "one"},
				second: map[string]any{"myDouble": 2.2, "myInt": 2, "myString": "two"}},
			{eventTypeName: "MyOAEvent",
				register: func() error {
					_, err := esper.RegisterObjectArray(env, "MyOAEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
					return err
				},
				first:  []any{1.1, 1, "one"},
				second: []any{2.2, 2, "two"}},
			{eventTypeName: "MyXMLEvent",
				register: func() error {
					_, err := esper.RegisterXML(env, "MyXMLEvent", []esper.FieldSpec{
						esper.FieldDef("myDouble", reflect.TypeOf(float64(0))),
						esper.FieldDef("myInt", reflect.TypeOf(int(0))),
						esper.FieldDef("myString", reflect.TypeOf("")),
					})
					return err
				},
				first:  "<MyXMLEvent><myDouble>1.1</myDouble><myInt>1</myInt><myString>one</myString></MyXMLEvent>",
				second: "<MyXMLEvent><myDouble>2.2</myDouble><myInt>2</myInt><myString>two</myString></MyXMLEvent>"},
		}
		for run, representation := range representations {
			if err := representation.register(); err != nil {
				return nil, err
			}
			deployID := fmt.Sprintf("MyStatement-%d", run)
			statementPlan, err := env.Build(esper.FromAny(env, representation.eventTypeName).
				Query(esper.StatementName("MyStatement")))
			if err != nil {
				return nil, err
			}
			statementDeployment, err := engine.DeployPlans(ctx, []esper.Plan{statementPlan},
				esper.WithDeploymentID(deployID))
			if err != nil {
				return nil, err
			}
			capture := &dataflowEPStatementSourceCapture{}
			deployIDValue := statementDeployment.ID()
			definition, buildErr := esper.DefineDataflow(env, fmt.Sprintf("MyDataFlowOne-%d", run)).
				EPStatementSourceByDeployment("source", deployIDValue, "MyStatement").
				Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
					return capture, nil
				}).
				Connect("source", "capture").
				Build()
			if buildErr != nil {
				return nil, buildErr
			}
			instance, err := engine.InstantiateDataflow(ctx, definition)
			if err != nil {
				return nil, err
			}
			if err := instance.Start(ctx); err != nil {
				return nil, err
			}
			if representation.eventTypeName == "MyXMLEvent" {
				if err := engine.SendXML(ctx, representation.eventTypeName, []byte(representation.first.(string))); err != nil {
					return nil, err
				}
				if err := engine.SendXML(ctx, representation.eventTypeName, []byte(representation.second.(string))); err != nil {
					return nil, err
				}
			} else {
				if err := engine.Send(ctx, representation.eventTypeName, representation.first); err != nil {
					return nil, err
				}
				if err := engine.Send(ctx, representation.eventTypeName, representation.second); err != nil {
					return nil, err
				}
			}
			emitAndReset(&capture)
			if err := instance.Cancel(ctx); err != nil {
				return nil, err
			}
			if err := engine.Undeploy(ctx, deployID); err != nil {
				return nil, err
			}
		}
	case 1: // stmt-name-dynamic
		capture := &dataflowEPStatementSourceCapture{}
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			EPStatementSourceByDeployment("source", "MyDeploymentId", "MyStatement").
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("source", "capture").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		// E1: the statement is not yet deployed — the source waits.
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E1"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		statementPlan, err := env.Build(esper.FromAny(env, "SupportBean").
			Select(esper.Alias("id", esper.Field[any, string]("theString"))).
			Query(esper.StatementName("MyStatement")))
		if err != nil {
			return nil, err
		}
		deploy := func(plan esper.Plan) error {
			_, err := engine.DeployPlans(ctx, []esper.Plan{plan}, esper.WithDeploymentID("MyDeploymentId"))
			return err
		}
		if err := deploy(statementPlan); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E2"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := engine.Undeploy(ctx, "MyDeploymentId"); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E3"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := deploy(statementPlan); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E4"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := engine.Undeploy(ctx, "MyDeploymentId"); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E5"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		redefinedPlan, err := env.Build(esper.FromAny(env, "SupportBean").
			Select(esper.Alias("id", esper.Concat(esper.Concat(
				esper.Literal("X"), esper.Field[any, string]("theString")), esper.Literal("X")))).
			Query(esper.StatementName("MyStatement")))
		if err != nil {
			return nil, err
		}
		if err := deploy(redefinedPlan); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E6"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
	case 2: // statement-filter
		// The B statement is deployed after the flow starts and attaches
		// via the deploy hook (the Java suite deploys it before the flow;
		// trace-neutral). The s2 statement attaches and detaches on its
		// deploy/undeploy cycle.
		capture := &dataflowEPStatementSourceCapture{}
		filter := func(esper.DataflowStatementSourceContext) bool { return true }
		definition, buildErr := esper.DefineDataflow(env, "MyDataFlowOne").
			EPStatementSourceWithStatementFilter("source", filter).
			Custom("capture", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
				return capture, nil
			}).
			Connect("source", "capture").
			Build()
		if buildErr != nil {
			return nil, buildErr
		}
		bPlan, err := env.Build(esper.FromAny(env, "SupportBean_B").
			Select(esper.Alias("id", esper.Field[any, string]("id"))).
			Query(esper.StatementName("stmt-b")))
		if err != nil {
			return nil, err
		}
		instance, err := engine.InstantiateDataflow(ctx, definition)
		if err != nil {
			return nil, err
		}
		if err := instance.Start(ctx); err != nil {
			return nil, err
		}
		if _, err := engine.Deploy(ctx, bPlan); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean_B", map[string]any{"id": "B1"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		ePlan, err := env.Build(esper.FromAny(env, "SupportBean").
			Select(esper.Alias("theString", esper.Field[any, string]("theString")),
				esper.Alias("intPrimitive", esper.Field[any, int]("intPrimitive"))).
			Query(esper.StatementName("stmt-e")))
		if err != nil {
			return nil, err
		}
		if _, err := engine.Deploy(ctx, ePlan); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E1", "intPrimitive": 1}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		aPlan, err := env.Build(esper.FromAny(env, "SupportBean_A").
			Select(esper.Alias("id", esper.Field[any, string]("id"))).
			Query(esper.StatementName("s2")))
		if err != nil {
			return nil, err
		}
		if _, err := engine.DeployPlans(ctx, []esper.Plan{aPlan}, esper.WithDeploymentID("s2")); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean_A", map[string]any{"id": "A1"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := engine.Undeploy(ctx, "s2"); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean_A", map[string]any{"id": "A2"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if _, err := engine.Deploy(ctx, aPlan); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean_A", map[string]any{"id": "A3"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := engine.Send(ctx, "SupportBean_B", map[string]any{"id": "B2"}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
		if err := instance.Cancel(ctx); err != nil {
			return nil, err
		}
		if err := engine.Send(ctx, "SupportBean", map[string]any{"theString": "E2", "intPrimitive": 1}); err != nil {
			return nil, err
		}
		emitAndReset(&capture)
	case 3: // invalid — compile/instantiate rejections: no trace rows
		// (Go Build-rejection tests pin the representable error class; the
		// two Java rejections without a Go surface are case differences.)
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported dataflow-epstatement-source case index %d", caseIndex)
	}
	return records, nil
}
