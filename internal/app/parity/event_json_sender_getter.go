package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the JSON event representation — two executions pinning
// the parse/send flow and the nested-Map getter surface:
//   - EventJsonEventSenderParseAndSend (java-runtime-8d0258718d883f9c5497):
//     a JSONSender parses the exact payload bytes {"p1": "abc"} and sends the
//     underlying; one listener delivery {p1:"abc"}.
//   - EventJsonGetterMapType (java-runtime-b36d999f7edc275dacd2): a nested
//     Map property (prop java.util.Map) projected as a kind/row wrapper
//     {"x":"y"}; the getter-surface divergence (prop.somefield? not
//     advertised) is in-process only.
//
// Record protocol: listener records, epoch time, case-local sequences.
const eventJsonSenderGetterJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eventJsonSenderGetterJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/json/EventJsonEventSender.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/json/EventJsonGetter.java",
}

var eventJsonSenderGetterJavaRuntimeIDs = []string{
	"java-runtime-8d0258718d883f9c5497",
	"java-runtime-b36d999f7edc275dacd2",
}

var eventJsonSenderGetterJavaExecutions = []string{
	"EventJsonEventSenderParseAndSend",
	"EventJsonGetterMapType",
}

var eventJsonSenderGetterCases = []string{
	"json-sender-parse-and-send",
	"json-getter-map-type",
}

func runDataflowEventJsonSenderGetterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("event-json-sender-getter scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eventJsonSenderGetterCases {
		caseTrace, err := runDataflowEventJsonSenderGetterCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("event-json-sender-getter case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowEventJsonSenderGetterCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emitListener := func(batch esper.ResultBatch) {
		sequence++
		normalized := compat.NormalizeResults(batch.New)
		for _, rec := range normalized {
			for name, val := range rec.Fields {
				if m, ok := val.(map[string]any); ok {
					// Plain nested maps project as kind/row wrappers to
					// match the Java oracle's EventBean convention.
					inner := make(map[string]any, len(m))
					for k, v := range m {
						inner[k] = v
					}
					rec.Fields[name] = compat.ResultRecord{Kind: "row", Fields: inner}
				}
			}
		}
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       normalized,
		})
	}

	switch caseIndex {
	case 0: // json-sender-parse-and-send
		env := esper.NewEnvironment()
		_, err := esper.RegisterJSON(env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("p1", reflect.TypeOf("")),
		})
		if err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eventJsonSenderGetterJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		plan, err := env.Build(esper.FromAny(env, "MyEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		statement := deployment.Statements()[0]
		deliveries := 0
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			deliveries++
			emitListener(batch)
			return nil
		}); err != nil {
			return nil, err
		}
		sender, err := engine.JSONSender("MyEvent")
		if err != nil {
			return nil, err
		}
		underlying, err := sender.Parse([]byte(`{"p1": "abc"}`))
		if err != nil {
			return nil, err
		}
		if err := sender.SendEvent(ctx, underlying); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries = %d, want 1", deliveries)
		}
	case 1: // json-getter-map-type — nested Map property
		env := esper.NewEnvironment()
		_, err := esper.RegisterJSON(env, "JsonEvent", []esper.FieldSpec{
			esper.FieldDef("prop", reflect.TypeOf(map[string]any{})),
		})
		if err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eventJsonSenderGetterJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		plan, err := env.Build(esper.FromAny(env, "JsonEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return nil, err
		}
		statement := deployment.Statements()[0]
		deliveries := 0
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			deliveries++
			emitListener(batch)
			return nil
		}); err != nil {
			return nil, err
		}
		sender, err := engine.JSONSender("JsonEvent")
		if err != nil {
			return nil, err
		}
		underlying, err := sender.Parse([]byte(`{"prop":{"x":"y"}}`))
		if err != nil {
			return nil, err
		}
		if err := sender.SendEvent(ctx, underlying); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries = %d, want 1", deliveries)
		}
	default:
		return nil, fmt.Errorf("unsupported event-json-sender-getter case index %d", caseIndex)
	}
	return records, nil
}
