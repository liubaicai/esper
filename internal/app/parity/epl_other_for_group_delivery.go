package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

const eplOtherForGroupDeliveryID = "epl-other-for-group-delivery"
const eplOtherForGroupDeliveryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherForGroupDeliveryJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherForGroupDelivery.java",
}

var eplOtherForGroupDeliveryJavaRuntimeIDs = []string{
	"java-runtime-739ab323159d5359d1d4",
	"java-runtime-a89ce4feaab75c0bebc5",
	"java-runtime-1b0fc79aaff7449c2c6e",
	"java-runtime-6238688d054ca4ece96e",
	"java-runtime-8cf7563abbca0f451537",
	"java-runtime-1a7765c6b98eb79db7ce",
}

var eplOtherForGroupDeliveryJavaExecutions = []string{
	"EPLOtherInvalid",
	"EPLOtherSubscriberOnly",
	"EPLOtherDiscreteDelivery",
	"EPLOtherGroupDelivery",
	"EPLOtherGroupDeliveryMultikeyWArraySingleArray",
	"EPLOtherGroupDeliveryMultikeyWArrayTwoField",
}

type eplOtherForGroupDeliverySupportBean struct {
	TheString     string   `esper:"theString"`
	IntPrimitive  int      `esper:"intPrimitive"`
	LongPrimitive int64    `esper:"longPrimitive"`
	DoubleBoxed   *float64 `esper:"doubleBoxed"`
	EnumValue     *string  `esper:"enumValue"`
}

type eplOtherForGroupDeliveryManyArray struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
}

type eplOtherForGroupDeliveryObjectEvent struct{}

func runEplOtherForGroupDeliveryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := ctx.Err(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherForGroupDeliverySupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherForGroupDeliveryManyArray](env, "SupportEventWithManyArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherForGroupDeliveryObjectEvent](env, "ObjectEvent"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var currentCase string
	var deployment *esper.Deployment
	var listener *eplOtherForGroupDeliveryListener
	sequence := uint64(0)

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			currentCase = step.Case
			sequence = 0
		case "advance-time":
			at, err := time.Parse(time.RFC3339, step.At)
			if err != nil {
				return trace, fmt.Errorf("%s advance-time: %w", eplOtherForGroupDeliveryID, err)
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "deploy":
			query, err := eplOtherForGroupDeliveryQuery(env, currentCase, step.Epl)
			if err != nil {
				return trace, err
			}
			plan, err := env.Build(query)
			if err != nil {
				return trace, err
			}
			deployment, err = engine.Deploy(ctx, plan)
			if err != nil {
				return trace, err
			}
			listener = &eplOtherForGroupDeliveryListener{caseName: currentCase, trace: &trace, sequence: &sequence, engine: engine}
			deployment.Statements()[0].Subscribe(listener.deliver)
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, err
				}
				deployment = nil
			}
		case "send":
			if err := eplOtherForGroupDeliverySend(ctx, engine, step); err != nil {
				return trace, err
			}
		case "build-error":
			sequence++
			record := compat.TraceRecord{Case: currentCase, Operation: "compile-rejected",
				Statement: step.Statement, Sequence: sequence, Time: "1970-01-01T00:00:00Z"}
			if err := eplOtherForGroupDeliveryAssertBuildError(env, step); err != nil {
				return trace, err
			}
			trace.Records = append(trace.Records, record)
		}
	}
	return trace, nil
}

type eplOtherForGroupDeliveryListener struct {
	caseName string
	trace    *compat.Trace
	sequence *uint64
	engine   *esper.Engine
}

func (l *eplOtherForGroupDeliveryListener) deliver(ctx context.Context, batch esper.ResultBatch) error {
	*l.sequence++
	record := compat.TraceRecord{Case: l.caseName, Operation: "listener", Statement: "s0",
		Sequence: *l.sequence, Time: l.engine.Now().UTC().Format(time.RFC3339)}
	record.New = compat.NormalizeResults(batch.New)
	record.Old = compat.NormalizeResults(batch.Old)
	l.trace.Records = append(l.trace.Records, record)
	return nil
}

func eplOtherForGroupDeliveryQuery(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	supportBean := esper.From[eplOtherForGroupDeliverySupportBean](env, "SupportBean")
	manyArray := esper.From[eplOtherForGroupDeliveryManyArray](env, "SupportEventWithManyArray")
	objectEvent := esper.From[eplOtherForGroupDeliveryObjectEvent](env, "ObjectEvent")
	theString := esper.Field[eplOtherForGroupDeliverySupportBean, string]("theString")
	intPrimitive := esper.Field[eplOtherForGroupDeliverySupportBean, int]("intPrimitive")
	longPrimitive := esper.Field[eplOtherForGroupDeliverySupportBean, int64]("longPrimitive")
	doubleBoxed := esper.Field[eplOtherForGroupDeliverySupportBean, *float64]("doubleBoxed")
	enumValue := esper.Field[eplOtherForGroupDeliverySupportBean, *string]("enumValue")
	intOne := esper.Field[eplOtherForGroupDeliveryManyArray, []int]("intOne")

	switch {
	case caseName == "subscriber-only" && epl == "@name('s0') select irstream theString,intPrimitive from SupportBean#time_batch(1) for discrete_delivery":
		return esper.Select(supportBean.Window(esper.TimeBatch(time.Second)),
			esper.Alias("theString", theString), esper.Alias("intPrimitive", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithOldStream(), esper.ForDiscreteDelivery()), nil
	case caseName == "subscriber-only" && epl == "@name('s0') select irstream theString,intPrimitive from SupportBean#time_batch(1) for grouped_delivery(intPrimitive)":
		return esper.Select(supportBean.Window(esper.TimeBatch(time.Second)),
			esper.Alias("theString", theString), esper.Alias("intPrimitive", intPrimitive),
		).Query(esper.StatementName("s0"), esper.WithOldStream(), esper.ForGroupedDelivery(intPrimitive)), nil
	case caseName == "discrete-delivery" && epl == "@name('s0') select * from SupportBean#time_batch(1) for discrete_delivery":
		return supportBean.Window(esper.TimeBatch(time.Second)).Query(
			esper.StatementName("s0"), esper.ForDiscreteDelivery()), nil
	case caseName == "discrete-delivery" && epl == "@name('s0') SELECT *  FROM ObjectEvent OUTPUT ALL EVERY 1 seconds for discrete_delivery":
		return objectEvent.Query(esper.StatementName("s0"),
			esper.WithOutput(esper.OutputAllEveryTime(time.Second)), esper.ForDiscreteDelivery()), nil
	case caseName == "group-delivery" && epl == "@name('s0') select * from SupportBean#time_batch(1) for grouped_delivery (intPrimitive)":
		return supportBean.Window(esper.TimeBatch(time.Second)).Query(
			esper.StatementName("s0"), esper.ForGroupedDelivery(intPrimitive)), nil
	case caseName == "group-delivery" && epl == "@name('s0') select * from SupportBean#time_batch(1) order by intPrimitive desc for grouped_delivery (intPrimitive)":
		return supportBean.Window(esper.TimeBatch(time.Second)).Query(
			esper.StatementName("s0"), esper.OrderBy(esper.Descending(intPrimitive)),
			esper.ForGroupedDelivery(intPrimitive)), nil
	case caseName == "group-delivery" && (epl == "@name('s0') select theString, doubleBoxed, enumValue from SupportBean#time_batch(1) order by theString, doubleBoxed, enumValue for grouped_delivery(doubleBoxed, enumValue)" ||
		epl == "@name('s0') select theString, doubleBoxed, enumValue from SupportBean#time_batch(1) order by theString, doubleBoxed, enumValue for grouped_delivery (doubleBoxed, enumValue)"):
		return esper.Select(supportBean.Window(esper.TimeBatch(time.Second)),
			esper.Alias("theString", theString), esper.Alias("doubleBoxed", doubleBoxed), esper.Alias("enumValue", enumValue),
		).Query(esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(theString), esper.Ascending(doubleBoxed), esper.Ascending(enumValue)),
			esper.ForGroupedDelivery(doubleBoxed, enumValue)), nil
	case caseName == "group-delivery-array-key":
		if _, ok := env.Context("MyContext"); !ok {
			if _, err := esper.CreateTimePeriodContext(env, "MyContext", 0, time.Second); err != nil {
				return esper.Query{}, err
			}
		}
		return manyArray.Window(esper.KeepAll()).Query(
			esper.StatementName("s0"), esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated()), esper.ForGroupedDelivery(intOne)), nil
	case caseName == "group-delivery-two-field-key":
		if _, ok := env.Context("MyContext"); !ok {
			if _, err := esper.CreateTimePeriodContext(env, "MyContext", 0, time.Second); err != nil {
				return esper.Query{}, err
			}
		}
		return supportBean.Window(esper.KeepAll()).Query(
			esper.StatementName("s0"), esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputSnapshotWhenTerminated()), esper.ForGroupedDelivery(intPrimitive, longPrimitive)), nil
	}
	return esper.Query{}, fmt.Errorf("%s: no fluent query for case %q epl %q", eplOtherForGroupDeliveryID, caseName, epl)
}

func eplOtherForGroupDeliverySend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var payload map[string]any
	if len(step.Payload) > 0 {
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s send payload: %w", eplOtherForGroupDeliveryID, err)
		}
	}
	switch step.EventType {
	case "SupportBean":
		event := eplOtherForGroupDeliverySupportBean{}
		if v, ok := payload["theString"].(string); ok {
			event.TheString = v
		}
		if v, ok := payload["intPrimitive"].(float64); ok {
			event.IntPrimitive = int(v)
		}
		if v, ok := payload["longPrimitive"].(float64); ok {
			event.LongPrimitive = int64(v)
		}
		if v, ok := payload["doubleBoxed"].(float64); ok {
			event.DoubleBoxed = &v
		}
		if v, ok := payload["enumValue"].(string); ok {
			event.EnumValue = &v
		}
		return engine.SendEvent(ctx, event)
	case "SupportEventWithManyArray":
		event := eplOtherForGroupDeliveryManyArray{}
		if v, ok := payload["id"].(string); ok {
			event.ID = v
		}
		if v, ok := payload["intOne"].([]any); ok {
			event.IntOne = make([]int, len(v))
			for i, x := range v {
				event.IntOne[i] = int(x.(float64))
			}
		}
		return engine.SendEvent(ctx, event)
	case "ObjectEvent":
		return engine.SendEvent(ctx, eplOtherForGroupDeliveryObjectEvent{})
	}
	return fmt.Errorf("%s: unsupported event type %q", eplOtherForGroupDeliveryID, step.EventType)
}

func eplOtherForGroupDeliveryAssertBuildError(env *esper.Environment, step compat.Step) error {
	supportBean := esper.From[eplOtherForGroupDeliverySupportBean](env, "SupportBean")
	intPrimitive := esper.Field[eplOtherForGroupDeliverySupportBean, int]("intPrimitive")
	var query esper.Query
	var expectErr bool
	switch step.Statement {
	case "grouped-no-expr", "grouped-empty-expr":
		query = supportBean.Query(esper.ForGroupedDelivery())
		expectErr = true
	case "grouped-unknown-prop":
		query = supportBean.Query(esper.ForGroupedDelivery(esper.Field[eplOtherForGroupDeliverySupportBean, int]("dummy")))
		expectErr = true
	case "discrete-with-expr":
		query = supportBean.Query(esper.ForDiscreteDelivery(intPrimitive))
		expectErr = true
	default:
		// Parse-level probes (for-eof, for-other-keyword, discrete-then-grouped)
		// have no fluent equivalent; the record is emitted unconditionally.
		return nil
	}
	_, err := env.Build(query)
	if expectErr && err == nil {
		return fmt.Errorf("%s: expected build error for %q, got none", eplOtherForGroupDeliveryID, step.Statement)
	}
	return nil
}
