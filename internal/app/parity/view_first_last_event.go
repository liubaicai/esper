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

const viewFirstLastEventJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var viewFirstLastEventJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewFirstEvent.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewFirstLength.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewLastEvent.java",
}

var viewFirstLastEventJavaRuntimeIDs = []string{
	"java-runtime-607d915be914a5dce34f", // ViewFirstEventSceneOne (ord 0)
	"java-runtime-5c32ea97d29ecbe957cb", // ViewFirstEventMarketData (ord 1)
	"java-runtime-d99cd0eba0b2e2ca64ba", // ViewFirstLengthSceneOne (ord 0)
	"java-runtime-2389705da83584b445a1", // ViewFirstLengthMarketData (ord 1)
	"java-runtime-260c9f5af6a4a10d5c80", // ViewLastEventSceneOne (ord 0)
	"java-runtime-af419392d33ae4b948d8", // ViewLastEventMarketData (ord 1)
}

var viewFirstLastEventJavaExecutions = []string{
	"ViewFirstEventSceneOne",
	"ViewFirstEventMarketData",
	"ViewFirstLengthSceneOne",
	"ViewFirstLengthMarketData",
	"ViewLastEventSceneOne",
	"ViewLastEventMarketData",
}

const (
	viewFirstLastEventID = "view-first-last-event"

	viewFirstLastEventFirstEventSceneOneCase    = "firstevent-scene-one"
	viewFirstLastEventFirstEventMarketDataCase  = "firstevent-marketdata"
	viewFirstLastEventFirstLengthSceneOneCase   = "firstlength-scene-one"
	viewFirstLastEventFirstLengthMarketDataCase = "firstlength-marketdata"
	viewFirstLastEventLastEventSceneOneCase     = "lastevent-scene-one"
	viewFirstLastEventLastEventMarketDataCase   = "lastevent-marketdata"
)

var viewFirstLastEventCaseOrder = []string{
	viewFirstLastEventFirstEventSceneOneCase,
	viewFirstLastEventFirstEventMarketDataCase,
	viewFirstLastEventFirstLengthSceneOneCase,
	viewFirstLastEventFirstLengthMarketDataCase,
	viewFirstLastEventLastEventSceneOneCase,
	viewFirstLastEventLastEventMarketDataCase,
}

// viewFirstLastEventBean is the SupportBean carrier for the three scene-one
// cases: theString/intPrimitive serve the c0/c1 (or c0-only) projections.
type viewFirstLastEventBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// runViewFirstLastEventScenario replays the six executions of the frozen
// Draft 4.442 unit: ViewFirstEvent ordinals 0/1, ViewFirstLength ordinals
// 0/1 and ViewLastEvent ordinals 0/1.  Each Java execution is one scenario
// case inside its own runtime; every statement selects irstream so the
// listener observes both the insert and the remove stream.
//
// The trace pins the two behaviors under test.  First, firstevent and
// firstlength admit only the first (or first n) inserts and silently drop
// every later insert: the listener is never invoked for a dropped event, so
// the trace carries no record for those sends.  Second, lastevent keeps only
// the newest event, so every insert delivers an insert/remove pair whose old
// row is the previously retained event (null for the first insert).
// Snapshot steps iterate the statement and emit the window contents the
// suite's assertPropsPerRowIterator[/AnyOrder] assertions read; "any"-mode
// steps sort rows canonically because the AnyOrder assertions do not pin
// iteration order.
//
// SupportMarketDataBean is a map event type carrying the four members the
// pinned makeMarketDataEvent helper supplies (symbol, price, volume, feed;
// id stays unset), matching the Java oracle's map registration.
func runViewFirstLastEventScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(viewFirstLastEventCaseOrder))
	for _, caseName := range viewFirstLastEventCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runViewFirstLastEventCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", viewFirstLastEventID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", viewFirstLastEventID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runViewFirstLastEventCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: viewFirstLastEventID}
	var sequence uint64

	now := time.Unix(0, 0).UTC()
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewFirstLastEventBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	str := reflect.TypeOf("")
	f64 := reflect.TypeOf(float64(0))
	i64 := reflect.TypeOf(int64(0))
	if _, err := esper.RegisterMap(env, "SupportMarketDataBean", []esper.FieldSpec{
		esper.FieldDef("symbol", str),
		esper.FieldDef("price", f64),
		esper.FieldDef("volume", i64),
		esper.FieldDef("feed", str),
	}); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(now),
		esper.WithRuntimeURI(viewFirstLastEventRuntimeURI(caseName)),
	)
	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
	deployCount := 0

	record := func(batch esper.ResultBatch) {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return
		}
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

	deploy := func(statement string) error {
		if statement != "s0" {
			return fmt.Errorf("unexpected deploy statement %q", statement)
		}
		if deployCount != 0 {
			return fmt.Errorf("unexpected deploy index %d", deployCount)
		}
		deployCount++
		query, err := viewFirstLastEventQuery(env, caseName)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return fmt.Errorf("build %q: %w", statement, err)
		}
		deployment, err := engine.DeployPlans(ctx, []esper.Plan{plan})
		if err != nil {
			return fmt.Errorf("deploy %q: %w", statement, err)
		}
		deployments[statement] = deployment
		for _, stmt := range deployment.Statements() {
			if stmt.Name() == "s0" {
				statements[stmt.Name()] = stmt
				if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deploy(step.Statement); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
			statements = map[string]*esper.Statement{}
		case "send":
			payload, err := decodeViewFirstLastEventPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			if step.Mode != "ordered" && step.Mode != "any" {
				return compat.Trace{}, fmt.Errorf("unsupported snapshot mode %q", step.Mode)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return compat.Trace{}, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Time:      compat.FormatTraceTime(now),
				New:       rows,
			})
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", viewFirstLastEventID, step.Op)
		}
	}
	_ = engine.Close(context.Background())
	return trace, nil
}

// viewFirstLastEventQuery builds the s0 select for one case: the fluent
// equivalent of the pinned irstream EPL.
func viewFirstLastEventQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	theString := esper.Field[viewFirstLastEventBean, string]("theString")
	intPrimitive := esper.Field[viewFirstLastEventBean, int]("intPrimitive")
	switch caseName {
	case viewFirstLastEventFirstEventSceneOneCase:
		return esper.Select(
			esper.From[viewFirstLastEventBean](env, "SupportBean").Window(esper.FirstEvent()),
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewFirstLastEventFirstEventMarketDataCase:
		return esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.FirstEvent()).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewFirstLastEventFirstLengthSceneOneCase:
		return esper.Select(
			esper.From[viewFirstLastEventBean](env, "SupportBean").Window(esper.FirstLength(2)),
			esper.Alias("c0", theString),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewFirstLastEventFirstLengthMarketDataCase:
		return esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.FirstLength(3)).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewFirstLastEventLastEventSceneOneCase:
		return esper.Select(
			esper.From[viewFirstLastEventBean](env, "SupportBean").Window(esper.LastEvent()),
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	case viewFirstLastEventLastEventMarketDataCase:
		return esper.FromAny(env, "SupportMarketDataBean").
			Window(esper.LastEvent()).
			Query(esper.StatementName("s0"), esper.WithOldStream()), nil
	}
	return esper.Query{}, fmt.Errorf("%s has no case %q", viewFirstLastEventID, caseName)
}

func viewFirstLastEventRuntimeURI(caseName string) string {
	for index, name := range viewFirstLastEventCaseOrder {
		if name == caseName {
			return viewFirstLastEventJavaRuntimeIDs[index]
		}
	}
	return "parity-" + viewFirstLastEventID + "-" + caseName
}

// decodeViewFirstLastEventPayload decodes a send payload to the record map
// the engine materializes against the registered schema.  Absent members
// take the Java helper defaults: SupportBean intPrimitive 0 and market
// events price 0.0, volume 0 and feed null, mirroring the pinned
// SupportBean(string,int) and makeMarketDataEvent(symbol,0,0L,null)
// constructors.
func decodeViewFirstLastEventPayload(step compat.Step) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", step.EventType, err)
	}
	switch step.EventType {
	case "SupportBean":
		if _, ok := payload["intPrimitive"]; !ok {
			payload["intPrimitive"] = 0
		}
	case "SupportMarketDataBean":
		for _, field := range []struct {
			name   string
			defVal any
		}{{"price", float64(0)}, {"volume", int64(0)}, {"feed", nil}} {
			if _, ok := payload[field.name]; !ok {
				payload[field.name] = field.defVal
			}
		}
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	return payload, nil
}
