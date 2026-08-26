package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

var eventMapCoreJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/map/EventMapCore.java",
}

// Inventory ordinal order minus the InvalidStatement execution: the Go
// type-safe chained API cannot express Java's three compile-time rejection
// shapes (unresolved property, String arithmetic, static misuse), so that
// execution stays unrepresented instead of registered.
var eventMapCoreJavaRuntimeIDs = []string{
	"java-runtime-b9ad55d94a0fae4f6aef",
	"java-runtime-cec11239155e518fa819",
	"java-runtime-69cbc6c78f1b84546092",
	"java-runtime-baefe25be71d54724d17",
}

var eventMapCoreJavaExecutions = []string{
	"EventMapCoreMapNestedEventType",
	"EventMapCoreMetadata",
	"EventMapCoreNestedObjects",
	"EventMapCoreQueryFields",
}

var eventMapCoreCases = []string{
	"map-nested-event-type",
	"metadata",
	"nested-objects",
	"query-fields",
}

func runEventMapCoreScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventMapCoreCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEventMapCoreCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runEventMapCoreCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	seq := uint64(0)

	anyT := reflect.TypeOf(any(nil))
	stringSliceT := reflect.TypeOf([]string{})

	// MyMap: three-level map-of-maps whose leaf carries a SupportBean
	// payload (Java configures sb:'SupportBean'; the Go surface passes the
	// nested map through the any-typed leaf).
	if _, err := esper.RegisterMap(env, "MyMapLev1", []esper.FieldSpec{{Name: "sb", Type: anyT}}); err != nil {
		return trace, fmt.Errorf("register MyMapLev1: %w", err)
	}
	lev1, _ := env.Schema("MyMapLev1")
	if _, err := esper.RegisterMap(env, "MyMapLev0", []esper.FieldSpec{{Name: "lev1name", Type: anyT}},
		esper.WithNestedPropertySchema("lev1name", lev1)); err != nil {
		return trace, fmt.Errorf("register MyMapLev0: %w", err)
	}
	lev0, _ := env.Schema("MyMapLev0")
	if _, err := esper.RegisterMap(env, "MyMap", []esper.FieldSpec{{Name: "lev0name", Type: anyT}},
		esper.WithNestedPropertySchema("lev0name", lev0)); err != nil {
		return trace, fmt.Errorf("register MyMap: %w", err)
	}

	// myMapEvent: {myInt:int, myString:string, beanA:SupportBeanComplexProps,
	// myStringArray:string[]}; beanA rides as the any-typed readable map.
	if _, err := esper.RegisterMap(env, "myMapEvent", []esper.FieldSpec{
		{Name: "myInt", Type: anyT},
		{Name: "myString", Type: anyT},
		{Name: "beanA", Type: anyT},
		{Name: "myStringArray", Type: stringSliceT},
	}); err != nil {
		return trace, fmt.Errorf("register myMapEvent: %w", err)
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	statements := map[string]*esper.Statement{}
	deploy := func(name string, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		for _, statement := range deployment.Statements() {
			statements[statement.Name()] = statement
			captured := statement
			if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				newRows := compat.NormalizeResults(batch.New)
				oldRows := compat.NormalizeResults(batch.Old)
				if len(newRows) == 0 && len(oldRows) == 0 {
					return nil
				}
				seq++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: captured.Name(),
					Sequence:  seq,
					Time:      batch.Time.UTC().Format(time.RFC3339),
					New:       newRows,
					Old:       oldRows,
				})
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	eventValue := esper.EventValue[esper.Event]()
	switch caseName {
	case "map-nested-event-type":
		// Pre-execution pinned check: MyMap must resolve.
		if _, ok := env.Schema("MyMap"); !ok {
			return trace, fmt.Errorf("preconfigured type MyMap not found")
		}
		query := esper.FromAny(env, "MyMap").Select(
			esper.Alias("val", esper.Property[any](eventValue, "lev0name.lev1name.sb.theString")),
		).Query(esper.StatementName("s0"))
		if err := deploy("s0", query); err != nil {
			return trace, err
		}
	case "metadata":
		// Pure introspection pinned by the oracle's assertions: four
		// descriptors in declaration order with the expected shapes. The
		// case emits only the deployed marker, exactly like the oracle.
		schema, ok := env.Schema("myMapEvent")
		if !ok {
			return trace, fmt.Errorf("preconfigured type myMapEvent not found")
		}
		if schema.Kind() != esper.SchemaMap {
			return trace, fmt.Errorf("expected MAP application type, got %v", schema.Kind())
		}
		specs := schema.Fields()
		want := []struct {
			name string
			kind reflect.Kind
		}{
			{"myInt", reflect.Interface},
			{"myString", reflect.Interface},
			{"beanA", reflect.Interface},
			{"myStringArray", reflect.Slice},
		}
		if len(specs) != len(want) {
			return trace, fmt.Errorf("expected four descriptors, got %d", len(specs))
		}
		for index, expected := range want {
			if specs[index].Name != expected.name {
				return trace, fmt.Errorf("descriptor %d = %q, want %q", index, specs[index].Name, expected.name)
			}
			if specs[index].Type.Kind() != expected.kind {
				return trace, fmt.Errorf("descriptor %s kind %s, want %s", expected.name, specs[index].Type.Kind(), expected.kind)
			}
		}
	case "nested-objects":
		query := esper.FromAny(env, "myMapEvent").Window(esper.LengthWindow(5)).Select(
			esper.Alias("simple", esper.Property[any](eventValue, "beanA.simpleProperty")),
			esper.Alias("nested", esper.Property[any](eventValue, "beanA.nested.nestedValue")),
			esper.Alias("indexed", esper.Property[any](eventValue, "beanA.indexedProps[1]")),
			esper.Alias("nestednested", esper.Property[any](eventValue, "beanA.nested.nestedNested.nestedNestedValue")),
		).Query(esper.StatementName("s0"))
		if err := deploy("s0", query); err != nil {
			return trace, err
		}
	case "query-fields":
		query := esper.FromAny(env, "myMapEvent").Window(esper.LengthWindow(5)).Select(
			esper.Alias("intVal", esper.Property[any](eventValue, "myInt")),
			esper.Alias("stringVal", esper.Property[any](eventValue, "myString")),
		).Query(esper.StatementName("s0"))
		if err := deploy("s0", query); err != nil {
			return trace, err
		}
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "deploy":
			continue // plans deploy up-front per case
		case "deployed":
			seq = 0
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  0,
			})
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("decode %s: %w", step.EventType, err)
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "send-error":
			if step.EventType == "MyMap" {
				// Java pins the sender-kind rejection text verbatim; the Go
				// engine now mirrors EventSenderObjectArray's message.
				err := engine.SendObjectArray(ctx, step.EventType, []any{})
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "send-error",
					Statement: step.EventType,
					Sequence:  0,
					Value:     "<no-error>",
				}
				if err != nil {
					// Mirror the oracle's rootCauseMessage: the bare
					// exception text without the Go error-class prefix.
					message := err.Error()
					var espErr *esper.Error
					if errors.As(err, &espErr) && espErr.Message != "" {
						message = espErr.Message
					}
					record.Value = message
				}
				trace.Records = append(trace.Records, record)
				continue
			}
			return trace, fmt.Errorf("unsupported send-error step in case %q", caseName)
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}
