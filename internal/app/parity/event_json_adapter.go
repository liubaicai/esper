package parity

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the JSON adapter surface — the three observable
// executions of EventJsonAdapter.java (the EventJsonAdapterInvalid compile
// diagnostics are disposed as intentionally-different in the manifest):
//   - EventJsonAdapterInsertInto: a POJO send routed through insert-into into
//     the adapter-declared JSON type; s0 carries the adapter-written strings
//     and s1 the byte-exact rendered JSON object.
//   - EventJsonAdapterCreateSchemaWStringTransform: an explicit json schema
//     with point/date string adapters, one filled and one null delivery.
//   - EventJsonAdapterDocSample: the single-field myDate json schema with a
//     dd-MM-yyyy adapter, the exact spaced payload bytes, and the root-named
//     renderer output.
//
// Both sides canonicalize row values as adapter-written strings; the Go
// runner never re-derives Java object toString forms. Java's create-json-schema
// statements are modeled by up-front RegisterJSON calls (the accepted typed
// substitution). All timestamps are pinned: TZ=UTC on the Java side, UTC
// layouts here.
const eventJsonAdapterJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eventJsonAdapterJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/json/EventJsonAdapter.java",
}

var eventJsonAdapterJavaRuntimeIDs = []string{
	"java-runtime-706d2a2f6bdd3f522457",
	"java-runtime-30ad022c4b1542e493e3",
	"java-runtime-39d9f1a890a50fb92d53",
}

var eventJsonAdapterJavaExecutions = []string{
	"EventJsonAdapterInsertInto",
	"EventJsonAdapterCreateSchemaWStringTransform",
	"EventJsonAdapterDocSample",
}

var eventJsonAdapterCases = []string{
	"adapter-insert-into",
	"adapter-create-schema-w-string-transform",
	"adapter-doc-sample",
}

type jsonAdapterPoint struct {
	X int
	Y int
}

type jsonAdapterLocalEvent struct {
	Point  jsonAdapterPoint `esper:"point"`
	Mydate time.Time        `esper:"mydate"`
}

func jsonAdapterPointAdapter() esper.JSONFieldAdapter {
	return esper.NewJSONFieldAdapter(func(text string) (jsonAdapterPoint, error) {
		parts := strings.Split(text, ",")
		if len(parts) != 2 {
			return jsonAdapterPoint{}, fmt.Errorf("invalid point %q", text)
		}
		x, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
		y, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errX != nil || errY != nil {
			return jsonAdapterPoint{}, fmt.Errorf("invalid point %q", text)
		}
		return jsonAdapterPoint{X: x, Y: y}, nil
	}, func(point jsonAdapterPoint) (string, error) {
		return strconv.Itoa(point.X) + "," + strconv.Itoa(point.Y), nil
	})
}

func jsonAdapterStdDateAdapter() esper.JSONFieldAdapter {
	return esper.NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("2006-01-02T15:04:05.000", text)
	}, func(value time.Time) (string, error) {
		return value.UTC().Format("2006-01-02T15:04:05.000"), nil
	})
}

func jsonAdapterDocDateAdapter() esper.JSONFieldAdapter {
	return esper.NewJSONFieldAdapter(func(text string) (time.Time, error) {
		return time.Parse("02-01-2006", text)
	}, func(value time.Time) (string, error) {
		return value.UTC().Format("02-01-2006"), nil
	})
}

func runEventJsonAdapterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("event-json-adapter scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eventJsonAdapterCases {
		caseTrace, err := runEventJsonAdapterCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("event-json-adapter case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runEventJsonAdapterCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emit := func(statement, operation string, fields map[string]any) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New: []compat.ResultRecord{{
				Kind:   "row",
				Fields: fields,
			}},
		})
	}
	// subscribeRow maps a delivered row through the record functions below:
	// projected statements record adapter-written strings, select-star
	// statements record the rendered JSON document.
	subscribe := func(statement *esper.Statement, record func(esper.Result) (map[string]any, error)) error {
		_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			for _, row := range batch.New {
				fields, err := record(row)
				if err != nil {
					return err
				}
				emit(statement.Name(), "listener", fields)
			}
			return nil
		})
		return err
	}
	renderedJSON := func(row esper.Result) (map[string]any, error) {
		event, ok := row.Event()
		if !ok {
			return nil, fmt.Errorf("json adapter select-star row carries no event")
		}
		rendered, err := esper.RenderJSON(event)
		if err != nil {
			return nil, err
		}
		return map[string]any{"json": rendered}, nil
	}
	switch caseIndex {
	case 0: // adapter-insert-into
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[jsonAdapterLocalEvent](env, "LocalEvent"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterJSON(env, "JsonEvent", []esper.FieldSpec{
			esper.FieldDef("point", reflect.TypeOf(jsonAdapterPoint{})),
			esper.FieldDef("mydate", reflect.TypeOf(time.Time{})),
		}, esper.WithJSONFieldAdapter("point", jsonAdapterPointAdapter()), esper.WithJSONFieldAdapter("mydate", jsonAdapterStdDateAdapter())); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eventJsonAdapterJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()

		producerPlan, err := env.Build(esper.Select(
			esper.From[jsonAdapterLocalEvent](env, "LocalEvent"),
			esper.Alias("point", esper.Field[jsonAdapterLocalEvent, jsonAdapterPoint]("point")),
			esper.Alias("mydate", esper.Field[jsonAdapterLocalEvent, time.Time]("mydate")),
		).InsertInto("JsonEvent", esper.StatementName("producer")))
		if err != nil {
			return nil, err
		}
		if _, err := engine.Deploy(ctx, producerPlan); err != nil {
			return nil, err
		}
		s0Plan, err := env.Build(esper.FromAny(env, "JsonEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		s0Deployment, err := engine.Deploy(ctx, s0Plan)
		if err != nil {
			return nil, err
		}
		s1Plan, err := env.Build(esper.FromAny(env, "JsonEvent").Query(esper.StatementName("s1")))
		if err != nil {
			return nil, err
		}
		s1Deployment, err := engine.Deploy(ctx, s1Plan)
		if err != nil {
			return nil, err
		}
		pointWrite := func(value any) (string, error) { return jsonAdapterPointAdapter().Write(value) }
		if err := subscribe(s0Deployment.Statements()[0], func(row esper.Result) (map[string]any, error) {
			return map[string]any{
				"point":  mustAdapterString(pointWrite, row, "point"),
				"mydate": mustAdapterString(func(value any) (string, error) { return jsonAdapterStdDateAdapter().Write(value) }, row, "mydate"),
			}, nil
		}); err != nil {
			return nil, err
		}
		if err := subscribe(s1Deployment.Statements()[0], renderedJSON); err != nil {
			return nil, err
		}
		mydate, err := time.Parse("2006-01-02T15:04:05.000", "2002-05-01T08:00:01.999")
		if err != nil {
			return nil, err
		}
		if err := engine.SendEvent(ctx, jsonAdapterLocalEvent{
			Point:  jsonAdapterPoint{X: 7, Y: 14},
			Mydate: mydate,
		}); err != nil {
			return nil, err
		}
	case 1: // adapter-create-schema-w-string-transform
		env := esper.NewEnvironment()
		if _, err := esper.RegisterJSON(env, "JsonEvent", []esper.FieldSpec{
			esper.FieldDef("point", reflect.TypeOf(jsonAdapterPoint{})),
			esper.FieldDef("mydate", reflect.TypeOf(time.Time{})),
		}, esper.WithJSONFieldAdapter("point", jsonAdapterPointAdapter()), esper.WithJSONFieldAdapter("mydate", jsonAdapterStdDateAdapter())); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eventJsonAdapterJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		s0Plan, err := env.Build(esper.FromAny(env, "JsonEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		s0Deployment, err := engine.Deploy(ctx, s0Plan)
		if err != nil {
			return nil, err
		}
		s1Plan, err := env.Build(esper.FromAny(env, "JsonEvent").Query(esper.StatementName("s1")))
		if err != nil {
			return nil, err
		}
		s1Deployment, err := engine.Deploy(ctx, s1Plan)
		if err != nil {
			return nil, err
		}
		pointWrite := func(value any) (string, error) { return jsonAdapterPointAdapter().Write(value) }
		dateWrite := func(value any) (string, error) { return jsonAdapterStdDateAdapter().Write(value) }
		if err := subscribe(s0Deployment.Statements()[0], func(row esper.Result) (map[string]any, error) {
			return map[string]any{
				"point":  mustAdapterString(pointWrite, row, "point"),
				"mydate": mustAdapterString(dateWrite, row, "mydate"),
			}, nil
		}); err != nil {
			return nil, err
		}
		if err := subscribe(s1Deployment.Statements()[0], renderedJSON); err != nil {
			return nil, err
		}
		sender, err := engine.JSONSender("JsonEvent")
		if err != nil {
			return nil, err
		}
		filled, err := sender.Parse([]byte(`{"point":"7,14","mydate":"2002-05-01T08:00:01.999"}`))
		if err != nil {
			return nil, err
		}
		if err := sender.SendEvent(ctx, filled); err != nil {
			return nil, err
		}
		nulled, err := sender.Parse([]byte(`{"point":null,"mydate":null}`))
		if err != nil {
			return nil, err
		}
		if err := sender.SendEvent(ctx, nulled); err != nil {
			return nil, err
		}
	case 2: // adapter-doc-sample
		env := esper.NewEnvironment()
		if _, err := esper.RegisterJSON(env, "JsonEvent", []esper.FieldSpec{
			esper.FieldDef("myDate", reflect.TypeOf(time.Time{})),
		}, esper.WithJSONFieldAdapter("myDate", jsonAdapterDocDateAdapter())); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eventJsonAdapterJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		s0Plan, err := env.Build(esper.FromAny(env, "JsonEvent").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		s0Deployment, err := engine.Deploy(ctx, s0Plan)
		if err != nil {
			return nil, err
		}
		docWrite := func(value any) (string, error) { return jsonAdapterDocDateAdapter().Write(value) }
		if err := subscribe(s0Deployment.Statements()[0], func(row esper.Result) (map[string]any, error) {
			return map[string]any{
				"myDate": mustAdapterString(docWrite, row, "myDate"),
			}, nil
		}); err != nil {
			return nil, err
		}
		sender, err := engine.JSONSender("JsonEvent")
		if err != nil {
			return nil, err
		}
		underlying, err := sender.Parse([]byte(`{"myDate" : "22-09-2018"}`))
		if err != nil {
			return nil, err
		}
		if err := sender.SendEvent(ctx, underlying); err != nil {
			return nil, err
		}
		rendered, err := esper.RenderJSON(underlying, esper.WithJSONTitle("hello"))
		if err != nil {
			return nil, err
		}
		emit("s0", "render", map[string]any{"json": rendered})
	default:
		return nil, fmt.Errorf("unsupported event-json-adapter case index %d", caseIndex)
	}
	return records, nil
}

// mustAdapterString renders one adapted row field; null and missing values
// become the differential protocol null marker.
func mustAdapterString(write func(any) (string, error), row esper.Result, name string) any {
	value := row.Get(name)
	if !value.IsPresent() || value.IsNull() {
		return map[string]any{"state": "null"}
	}
	written, err := write(value.Any())
	if err != nil {
		return map[string]any{"state": "null"}
	}
	return written
}
