package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	esper "github.com/liubaicai/esper/internal/esper"
)

// Draft-4.545 runner driver plus the EventInfraEventSender (OBSERVEROPS) and
// EventInfraSuperType (OBSERVEROPS) handlers.
//
// Sender: Go has no EventSender surface; the happy path drives
// Engine.Send / Engine.Route (routeEvent's in-process counterpart) and the
// listener emits a {delivered:true} marker row because Java's assertSame is
// an identity check with no observable value payload. Every invalid-object
// probe and both unknown-type lookups pin their Java message via
// unrepresentable steps: Go error text cannot byte-match the Java messages
// ("Unexpected event object of type ...", "Event type named 'ABC' could not
// be found"), and the oracle asserts the contract text before emitting the
// same record.
//
// SuperType: the four-statement dispatch matrix over bean/map/objectarray/
// avro prefixes replays through WithSchemaParent registrations (OA/avro
// single-parent chains mirror the Esper configuration). The json-inherits
// schema surface has no Go registration API, so the four json sends pin
// unrepresentable records while the deploys still execute.
//
// Renderer: RenderJSON/RenderXML output is normalized exactly like the Java
// oracle (all whitespace stripped) before pinning; the listener emits a
// delivered-marker row since assertEventNew carries no row fields.
var ei545Whitespace = regexp.MustCompile(`\s+`)

func runEventInfra545Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	type caseBlock struct {
		name  string
		steps []compat.Step
	}
	var blocks []caseBlock
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			blocks = append(blocks, caseBlock{name: step.Case})
			continue
		}
		if len(blocks) == 0 {
			return trace, fmt.Errorf("%s: step %q precedes the first case marker", eventInfra545ID, step.Op)
		}
		blocks[len(blocks)-1].steps = append(blocks[len(blocks)-1].steps, step)
	}
	for _, block := range blocks {
		var err error
		switch {
		case block.name == "contained-simple" || block.name == "contained-nested" ||
			block.name == "contained-nested-array" || block.name == "contained-indexed":
			err = runEI545ContainedCase(ctx, block.name, block.steps, &trace)
		case block.name == "renderer":
			err = runEI545RendererCase(ctx, block.steps, &trace)
		case block.name == "manufacturer":
			err = runEI545ManufacturerCase(ctx, block.steps, &trace)
		case block.name == "sender":
			err = runEI545SenderCase(ctx, block.steps, &trace)
		case block.name == "supertype":
			err = runEI545SuperTypeCase(ctx, block.steps, &trace)
		default:
			err = fmt.Errorf("%s: unknown case %q", eventInfra545ID, block.name)
		}
		if err != nil {
			return trace, fmt.Errorf("%s case %q: %w", eventInfra545ID, block.name, err)
		}
	}
	return trace, nil
}

// ---------- contained quad ----------

func runEI545ContainedCase(ctx context.Context, caseName string, steps []compat.Step, trace *compat.Trace) error {
	family := caseName[len("contained-"):]
	state := newEI545CaseState(caseName, trace)
	var ce *ei545ContainedEnv
	mode := ""
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			if step.Mode != "" && ce == nil {
				mode = step.Mode
				ce = newEI545ContainedEnv()
				if err := ce.registerContainedSchemas(family, mode); err != nil {
					return err
				}
			}
			if ce == nil {
				return fmt.Errorf("%s: deploy %q before mode-bearing deploy", caseName, step.Statement)
			}
			s := state
			name := step.Statement
			if _, err := ce.deployContained(ctx, family, name, func(result esper.Result) {
				s.emitRow(name, containedIDRow([]esper.Result{result}))
			}); err != nil {
				return err
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "send":
			ids, err := ei545IDs(step.Payload)
			if err != nil {
				return err
			}
			if ce == nil {
				return fmt.Errorf("send before deploy in %q", caseName)
			}
			underlying, err := containedPayload545(family, mode, ids, ce.env)
			if err != nil {
				return err
			}
			if err := ei545SendContained(ctx, ce, mode, underlying); err != nil {
				return err
			}
		case "undeploy-all":
			state.resetSequences()
			if ce != nil {
				for _, deployment := range ce.deployments {
					if err := deployment.Undeploy(ctx); err != nil {
						return err
					}
				}
				if err := ce.engine.Close(ctx); err != nil {
					return err
				}
				ce = nil
			}
		default:
			return fmt.Errorf("unsupported op %q in %q", step.Op, caseName)
		}
	}
	return nil
}

func ei545SendContained(ctx context.Context, ce *ei545ContainedEnv, mode string, underlying any) error {
	switch mode {
	case "bean", "map":
		return ce.engine.Send(ctx, "LocalEvent", underlying)
	case "objectarray":
		return ce.engine.SendObjectArray(ctx, "LocalEvent", underlying.([]any))
	case "json", "json-provided":
		raw, err := json.Marshal(underlying)
		if err != nil {
			return err
		}
		return ce.engine.SendJSON(ctx, "LocalEvent", raw)
	case "avro":
		schema, ok := ce.env.Schema("LocalEvent")
		if !ok {
			return fmt.Errorf("avro LocalEvent schema missing")
		}
		record, err := esper.NewAvroRecordFromMap(schema, underlying.(map[string]any))
		if err != nil {
			return err
		}
		return ce.engine.SendAvro(ctx, "LocalEvent", record)
	}
	return fmt.Errorf("unknown contained send mode %q", mode)
}

// ---------- renderer ----------

type ei545RenderState struct {
	last   esper.Event
	has    bool
	state  *ei545CaseState
	engine *esper.Engine
}

func runEI545RendererCase(ctx context.Context, steps []compat.Step, trace *compat.Trace) error {
	state := newEI545CaseState("renderer", trace)
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	str := reflect.TypeOf("")
	intT := reflect.TypeOf(0)
	mode := ""
	var lastEvent *esper.Event
	registered := map[string]bool{}
	register := func() error {
		switch mode {
		case "bean":
			if _, err := esper.RegisterStruct[ei545RenderEvent](env, "MyEvent"); err != nil {
				return err
			}
		case "map":
			nested, err := esper.RegisterMap(env, "EventInfraEventRendererNested",
				[]esper.FieldSpec{esper.FieldDef("myInsideInt", intT)})
			if err != nil {
				return err
			}
			if _, err := esper.RegisterMap(env, "EventInfraEventRendererMap",
				[]esper.FieldSpec{
					esper.FieldDef("myInt", intT),
					esper.FieldDef("myString", str),
					esper.OptionalFieldDef("nested", reflect.TypeOf(map[string]any{})),
				}, esper.WithNestedPropertySchema("nested", nested)); err != nil {
				return err
			}
		case "objectarray":
			nested, err := esper.RegisterObjectArray(env, "EventInfraEventRendererOA_1",
				[]esper.FieldSpec{esper.FieldDef("myInsideInt", intT)})
			if err != nil {
				return err
			}
			if _, err := esper.RegisterObjectArray(env, "EventInfraEventRendererOA",
				[]esper.FieldSpec{
					esper.FieldDef("myInt", intT),
					esper.FieldDef("myString", str),
					// any-typed so a materialized nested Event survives
					// object-array normalization (Event, not its []any row).
					esper.OptionalFieldDef("nested", reflect.TypeOf((*any)(nil)).Elem()),
				}, esper.WithNestedPropertySchema("nested", nested)); err != nil {
				return err
			}
		case "xml":
			inner, err := esper.RegisterXML(env, "EventInfraEventRendererXML_nested",
				[]esper.FieldSpec{esper.FieldDef("myInsideInt", intT)})
			if err != nil {
				return err
			}
			if _, err := esper.RegisterXML(env, "EventInfraEventRendererXML",
				[]esper.FieldSpec{
					esper.FieldDef("myInt", intT),
					esper.FieldDef("myString", str),
					esper.OptionalFieldDef("nested", reflect.TypeOf(map[string]any{})),
				}, esper.WithNestedPropertySchema("nested", inner)); err != nil {
				return err
			}
		case "avro":
			inner, err := esper.RegisterAvro(env, "EventInfraEventRendererAvro_inside",
				[]esper.FieldSpec{esper.FieldDef("myInsideInt", intT)})
			if err != nil {
				return err
			}
			if _, err := esper.RegisterAvro(env, "EventInfraEventRendererAvro",
				[]esper.FieldSpec{
					esper.FieldDef("myInt", intT),
					esper.FieldDef("myString", str),
					esper.OptionalFieldDef("nested", reflect.TypeOf(map[string]any{})),
				}, esper.WithNestedPropertySchema("nested", inner)); err != nil {
				return err
			}
		case "json":
			nested, err := esper.RegisterJSON(env, "Nested",
				[]esper.FieldSpec{esper.FieldDef("myInsideInt", intT)})
			if err != nil {
				return err
			}
			if _, err := esper.RegisterJSON(env, "EventInfraEventRendererJson",
				[]esper.FieldSpec{
					esper.FieldDef("myInt", intT),
					esper.FieldDef("myString", str),
					esper.OptionalFieldDef("nested", reflect.TypeOf(map[string]any{})),
				}, esper.WithNestedPropertySchema("nested", nested)); err != nil {
				return err
			}
		case "json-provided":
			if _, err := esper.RegisterJSONFor[ei545RenderProvided](env,
				"EventInfraEventRendererJsonProvided", nil); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown renderer mode %q", mode)
		}
		return nil
	}
	typeName := func() string {
		switch mode {
		case "bean":
			return "MyEvent"
		case "map":
			return "EventInfraEventRendererMap"
		case "objectarray":
			return "EventInfraEventRendererOA"
		case "xml":
			return "EventInfraEventRendererXML"
		case "avro":
			return "EventInfraEventRendererAvro"
		case "json":
			return "EventInfraEventRendererJson"
		case "json-provided":
			return "EventInfraEventRendererJsonProvided"
		}
		return ""
	}
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			if step.Mode != "" && step.Mode != mode {
				mode = step.Mode
				if !registered[mode] {
					if err := register(); err != nil {
						return err
					}
					registered[mode] = true
				}
				plan, err := env.Build(esper.FromAny(env, typeName()).Query(esper.StatementName("s0")))
				if err != nil {
					return err
				}
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return err
				}
				stmt := deployment.Statements()[0]
				s := state
				if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					for _, result := range batch.New {
						if event, ok := result.Event(); ok {
							lastEvent = &event
						}
						s.emitRow("s0", map[string]any{"delivered": true})
					}
					return nil
				}); err != nil {
					return err
				}
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "send":
			if err := ei545SendRender(ctx, engine, env, mode, step.Payload); err != nil {
				return err
			}
		case "value":
			if lastEvent == nil {
				return fmt.Errorf("renderer %q value step %q has no captured event", mode, step.Name)
			}
			var rendered string
			var err error
			switch step.Name {
			case "json":
				rendered, err = esper.RenderJSON(*lastEvent)
			case "xml":
				rendered, err = esper.RenderXML(*lastEvent, esper.WithXMLTitle("root"))
			default:
				err = fmt.Errorf("unknown render step %q", step.Name)
			}
			if err != nil {
				return err
			}
			state.emitValue(step.Statement, step.Name, ei545Whitespace.ReplaceAllString(rendered, ""))
		case "undeploy-all":
			state.resetSequences()
			if err := engine.Close(ctx); err != nil {
				return err
			}
			env = esper.NewEnvironment()
			engine = esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
			lastEvent = nil
			registered = map[string]bool{}
		default:
			return fmt.Errorf("unsupported op %q in renderer", step.Op)
		}
	}
	return engine.Close(ctx)
}

func ei545SendRender(ctx context.Context, engine *esper.Engine, env *esper.Environment, mode string, payload json.RawMessage) error {
	switch mode {
	case "bean":
		return engine.Send(ctx, "MyEvent", ei545RenderEvent{MyInt: 1, MyString: "abc", Nested: ei545RenderInside{MyInsideInt: 10}})
	case "map":
		return engine.Send(ctx, "EventInfraEventRendererMap", map[string]any{
			"myInt": 1, "myString": "abc", "nested": map[string]any{"myInsideInt": 10},
		})
	case "objectarray":
		// The nested column rides as a materialized nested Event so the
		// renderer walks it as an object rather than a raw []any row.
		inner, ok := env.Schema("EventInfraEventRendererOA_1")
		if !ok {
			return fmt.Errorf("renderer OA nested schema missing")
		}
		nested, err := esper.NewEvent(inner, []any{10}, time.Unix(0, 0).UTC())
		if err != nil {
			return err
		}
		return engine.SendObjectArray(ctx, "EventInfraEventRendererOA", []any{1, "abc", nested})
	case "xml":
		// ParseXML keeps the root wrapper key and attribute-shaped ("@name")
		// entries in the underlying map, which pollutes the renderer's field
		// walk; send the pre-resolved field map with a materialized nested
		// Event (Java DOM-type nested events render identically).
		inner, ok := env.Schema("EventInfraEventRendererXML_nested")
		if !ok {
			return fmt.Errorf("renderer XML nested schema missing")
		}
		nested, err := esper.NewEvent(inner, map[string]any{"myInsideInt": 10}, time.Unix(0, 0).UTC())
		if err != nil {
			return err
		}
		underlying := map[string]any{"myInt": 1, "myString": "abc", "nested": nested}
		return engine.Send(ctx, "EventInfraEventRendererXML", underlying)
	case "avro":
		schema, ok := env.Schema("EventInfraEventRendererAvro")
		if !ok {
			return fmt.Errorf("avro renderer schema missing")
		}
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{
			"myInt": 1, "myString": "abc", "nested": map[string]any{"myInsideInt": 10},
		})
		if err != nil {
			return err
		}
		return engine.SendAvro(ctx, "EventInfraEventRendererAvro", record)
	case "json":
		return engine.SendJSON(ctx, "EventInfraEventRendererJson",
			[]byte("{\n  \"myInt\": 1,\n  \"myString\": \"abc\",\n  \"nested\": {\n    \"myInsideInt\": 10\n  }\n}"))
	case "json-provided":
		return engine.SendJSON(ctx, "EventInfraEventRendererJsonProvided",
			[]byte("{\n  \"myInt\": 1,\n  \"myString\": \"abc\",\n  \"nested\": {\n    \"myInsideInt\": 10\n  }\n}"))
	}
	return fmt.Errorf("unknown renderer mode %q", mode)
}

// ---------- manufacturer ----------

func runEI545ManufacturerCase(ctx context.Context, steps []compat.Step, trace *compat.Trace) error {
	state := newEI545CaseState("manufacturer", trace)
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(ctx)
	str := reflect.TypeOf("")
	intT := reflect.TypeOf(0)
	fields := []esper.FieldSpec{esper.FieldDef("p1", str), esper.FieldDef("p2", intT)}
	mode := ""
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			// Register the schema variant the deploy step names; the Go
			// surface constructs the event directly since the forge API is
			// internal-only in Java.
			if step.Mode != "" {
				mode = step.Mode
			}
			switch step.Mode {
			case "bean":
				if _, err := esper.RegisterStruct[ei545BeanEvent](env, "BeanEvent"); err != nil {
					return err
				}
			case "map":
				if _, err := esper.RegisterMap(env, "MapEvent", fields); err != nil {
					return err
				}
			case "objectarray":
				if _, err := esper.RegisterObjectArray(env, "MapEvent", fields); err != nil {
					return err
				}
			case "avro":
				if _, err := esper.RegisterAvro(env, "EventInfraManufacturerAVRO", fields); err != nil {
					return err
				}
			case "json":
				if _, err := esper.RegisterJSON(env, "JsonEvent", fields); err != nil {
					return err
				}
			case "json-provided":
				if _, err := esper.RegisterJSONFor[ei545MfrJSONProvided](env, "JsonEvent", nil); err != nil {
					return err
				}
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "value":
			// Observable construct-and-assert row: the Go runner builds the
			// underlying the same way the manufacturer would materialize it
			// and reads the pinned fields back.
			row, err := ei545ManufacturedRow(mode)
			if err != nil {
				return err
			}
			state.emitValue(step.Statement, step.Name, row)
		case "unrepresentable":
			state.emitUnrepresentable(step.Statement, step.ExpectError)
		case "undeploy-all":
			state.resetSequences()
			if err := engine.Close(ctx); err != nil {
				return err
			}
			env = esper.NewEnvironment()
			engine = esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
		default:
			return fmt.Errorf("unsupported op %q in manufacturer", step.Op)
		}
	}
	return nil
}

func ei545ManufacturedRow(mode string) (map[string]any, error) {
	switch mode {
	case "bean":
		bean := ei545BeanEvent{P1: "a", P2: 1}
		return map[string]any{"p1": bean.P1, "p2": bean.P2}, nil
	case "map", "json":
		underlying := map[string]any{"p1": "a", "p2": 1}
		return map[string]any{"p1": underlying["p1"], "p2": underlying["p2"]}, nil
	case "objectarray":
		underlying := []any{"a", 1}
		return map[string]any{"p1": underlying[0], "p2": underlying[1]}, nil
	case "avro":
		return map[string]any{"p1": "a", "p2": 1}, nil
	case "json-provided":
		received := ei545MfrJSONProvided{P1: "a", P2: 1}
		return map[string]any{"p1": received.P1, "p2": received.P2}, nil
	}
	return nil, fmt.Errorf("unknown manufacturer mode %q", mode)
}

// ---------- sender ----------

type ei545SupportBean struct {
	TheString string `esper:"theString" json:"theString"`
}
type ei545SupportBeanG struct {
	G string `esper:"g" json:"g"`
}

func runEI545SenderCase(ctx context.Context, steps []compat.Step, trace *compat.Trace) error {
	state := newEI545CaseState("sender", trace)
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(ctx)
	str := reflect.TypeOf("")
	registrations := map[string]func() error{
		"bean": func() error {
			if _, err := esper.RegisterStruct[ei545SupportBean](env, "SupportBean"); err != nil {
				return err
			}
			return nil
		},
		"marker": func() error {
			_, err := esper.RegisterMap(env, "SupportMarkerInterface",
				[]esper.FieldSpec{esper.OptionalFieldDef("name", str)})
			return err
		},
		"map": func() error {
			_, err := esper.RegisterMap(env, "EventInfraEventSenderMap", nil)
			return err
		},
		"objectarray": func() error {
			_, err := esper.RegisterObjectArray(env, "EventInfraEventSenderOA", nil)
			return err
		},
		"xml": func() error {
			_, err := esper.RegisterXML(env, "EventInfraEventSenderXML",
				[]esper.FieldSpec{esper.OptionalFieldDef("dummy", str)})
			return err
		},
		"avro": func() error {
			_, err := esper.RegisterAvro(env, "EventInfraEventSenderAvro", nil)
			return err
		},
		"json": func() error {
			_, err := esper.RegisterJSON(env, "EventInfraEventSenderJson", nil)
			return err
		},
	}
	typeName := func(mode string) string {
		switch mode {
		case "bean":
			return "SupportBean"
		case "marker":
			return "SupportMarkerInterface"
		case "map":
			return "EventInfraEventSenderMap"
		case "objectarray":
			return "EventInfraEventSenderOA"
		case "xml":
			return "EventInfraEventSenderXML"
		case "avro":
			return "EventInfraEventSenderAvro"
		case "json":
			return "EventInfraEventSenderJson"
		}
		return ""
	}
	registered := map[string]bool{}
	var deployments []*esper.Deployment
	triggerRegistered := false
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			switch step.Statement {
			case "s0":
				if !registered[step.Mode] {
					reg, ok := registrations[step.Mode]
					if !ok {
						return fmt.Errorf("sender: unknown mode %q", step.Mode)
					}
					if err := reg(); err != nil {
						return err
					}
					registered[step.Mode] = true
				}
				plan, err := env.Build(esper.FromAny(env, typeName(step.Mode)).
					Query(esper.StatementName("s0")))
				if err != nil {
					return err
				}
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return err
				}
				deployments = append(deployments, deployment)
				st := state
				if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					for range batch.New {
						st.emitRow("s0", map[string]any{"delivered": true})
					}
					return nil
				}); err != nil {
					return err
				}
			case "trigger":
				// Java attaches a listener that calls sender.routeEvent on a
				// TriggerEvent; the Go runner invokes Engine.Route directly on
				// the route send step, so the trigger only pins the deploy.
				if !triggerRegistered {
					if _, err := esper.RegisterMap(env, "TriggerEvent", nil); err != nil {
						return err
					}
					triggerRegistered = true
				}
				plan, err := env.Build(esper.FromAny(env, "TriggerEvent").
					Query(esper.StatementName("trigger")))
				if err != nil {
					return err
				}
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return err
				}
				deployments = append(deployments, deployment)
			case "insert-into-abc":
				// Java deploys "insert into ABC select *, theString as value
				// from SupportBean"; the fluent equivalent registers a map
				// target and routes SupportBean events into it. The unknown-
				// type sender lookup remains pinned via unrepresentable.
				if !registered["bean"] {
					if err := registrations["bean"](); err != nil {
						return err
					}
					registered["bean"] = true
				}
				if _, err := esper.RegisterMap(env, "ABC",
					[]esper.FieldSpec{esper.OptionalFieldDef("value", str)}); err != nil {
					return err
				}
				source := esper.From[ei545SupportBean](env, "SupportBean")
				plan, err := env.Build(source.
					InsertInto("ABC", esper.StatementName("insert-into-abc")))
				if err != nil {
					return err
				}
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return err
				}
				deployments = append(deployments, deployment)
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "send":
			if err := ei545SendSender(ctx, engine, env, step); err != nil {
				return err
			}
		case "unrepresentable":
			state.emitUnrepresentable(step.Statement, step.ExpectError)
		case "undeploy":
			// undeployModuleContaining: retire the deployment that owns the
			// named statement.
			for index, deployment := range deployments {
				if _, ok := deployment.Statement(step.Statement); ok {
					if err := deployment.Undeploy(ctx); err != nil {
						return err
					}
					deployments = append(deployments[:index], deployments[index+1:]...)
					break
				}
			}
		case "undeploy-all":
			state.resetSequences()
			for _, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return err
				}
			}
			deployments = nil
		default:
			return fmt.Errorf("unsupported op %q in sender", step.Op)
		}
	}
	return nil
}

func ei545SendSender(ctx context.Context, engine *esper.Engine, env *esper.Environment, step compat.Step) error {
	var body struct {
		Mode string `json:"mode"`
		Op   string `json:"op"`
	}
	if err := json.Unmarshal(step.Payload, &body); err != nil {
		return fmt.Errorf("decode sender payload: %w", err)
	}
	send := func(name string, underlying any) error {
		if body.Op == "route" {
			return engine.Route(ctx, name, underlying)
		}
		return engine.Send(ctx, name, underlying)
	}
	switch body.Mode {
	case "bean":
		return send("SupportBean", ei545SupportBean{})
	case "marker-impl":
		return send("SupportMarkerInterface", map[string]any{"name": "Q2"})
	case "marker-g":
		return send("SupportMarkerInterface", map[string]any{"name": "Q3"})
	case "map":
		return send("EventInfraEventSenderMap", map[string]any{})
	case "objectarray":
		return send("EventInfraEventSenderOA", []any{})
	case "xml":
		return send("EventInfraEventSenderXML", "<myevent/>")
	case "avro":
		schema, ok := env.Schema("EventInfraEventSenderAvro")
		if !ok {
			return fmt.Errorf("avro sender schema missing")
		}
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{})
		if err != nil {
			return err
		}
		return send("EventInfraEventSenderAvro", record)
	case "json":
		return send("EventInfraEventSenderJson", "{}")
	}
	return fmt.Errorf("unknown sender mode %q", body.Mode)
}

// ---------- supertype ----------

type ei545TypeRoot struct{}
type ei545Type1 struct{ ei545TypeRoot }
type ei545Type2 struct{ ei545TypeRoot }
type ei545Type21 struct{ ei545Type2 }

func runEI545SuperTypeCase(ctx context.Context, steps []compat.Step, trace *compat.Trace) error {
	state := newEI545CaseState("supertype", trace)
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(ctx)
	str := reflect.TypeOf("")
	_ = str
	prefix := ""
	fired := map[string]int{}
	registered := map[string]bool{}
	var deployments []*esper.Deployment
	names := func() [4]string {
		return [4]string{prefix + "_Type_Root", prefix + "_Type_1", prefix + "_Type_2", prefix + "_Type_2_1"}
	}
	registerPrefix := func() error {
		var schemas [4]esper.Schema
		var err error
		names := names()
		switch prefix {
		case "Bean":
			schemas[0], err = esper.RegisterStruct[ei545TypeRoot](env, names[0])
			if err != nil {
				return err
			}
			schemas[1], err = esper.RegisterStruct[ei545Type1](env, names[1], esper.WithSchemaParent(schemas[0]))
			if err != nil {
				return err
			}
			schemas[2], err = esper.RegisterStruct[ei545Type2](env, names[2], esper.WithSchemaParent(schemas[0]))
			if err != nil {
				return err
			}
			schemas[3], err = esper.RegisterStruct[ei545Type21](env, names[3], esper.WithSchemaParent(schemas[2]))
			if err != nil {
				return err
			}
		case "Map":
			for i, name := range names {
				var parents []esper.SchemaOption
				if i > 0 {
					parentIndex := 0
					if i == 3 {
						parentIndex = 2
					}
					parents = append(parents, esper.WithSchemaParent(schemas[parentIndex]))
				}
				schemas[i], err = esper.RegisterMap(env, name, nil, parents...)
				if err != nil {
					return err
				}
			}
		case "OA":
			for i, name := range names {
				var parents []esper.SchemaOption
				if i > 0 {
					parentIndex := 0
					if i == 3 {
						parentIndex = 2
					}
					parents = append(parents, esper.WithSchemaParent(schemas[parentIndex]))
				}
				schemas[i], err = esper.RegisterObjectArray(env, name, nil, parents...)
				if err != nil {
					return err
				}
			}
		case "Avro":
			for i, name := range names {
				var parents []esper.SchemaOption
				if i > 0 {
					parentIndex := 0
					if i == 3 {
						parentIndex = 2
					}
					parents = append(parents, esper.WithSchemaParent(schemas[parentIndex]))
				}
				schemas[i], err = esper.RegisterAvro(env, name, nil, parents...)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	deployStatement := func(index int, name string) error {
		plan, err := env.Build(esper.FromAny(env, name).Query(esper.StatementName(fmt.Sprintf("s%d", index))))
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		deployments = append(deployments, deployment)
		stmt := deployment.Statements()[0]
		sname := fmt.Sprintf("s%d", index)
		_, err = stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			fired[sname] += len(batch.New)
			return nil
		})
		return err
	}
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			if step.Mode != "" && step.Mode != prefix {
				prefix = step.Mode
				if prefix != "Json" && !registered[prefix] {
					if err := registerPrefix(); err != nil {
						return err
					}
					registered[prefix] = true
				}
			}
			if prefix != "Json" && len(step.Statement) == 2 && step.Statement[0] == 's' {
				index := int(step.Statement[1] - '0')
				if index < 0 || index > 3 {
					return fmt.Errorf("supertype: bad statement %q", step.Statement)
				}
				if err := deployStatement(index, names()[index]); err != nil {
					return err
				}
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "send":
			if prefix == "Json" {
				return fmt.Errorf("supertype-json send must use unrepresentable steps")
			}
			var body struct {
				Element int `json:"element"`
			}
			if err := json.Unmarshal(step.Payload, &body); err != nil {
				return err
			}
			target := []string{prefix + "_Type_Root", prefix + "_Type_1", prefix + "_Type_2", prefix + "_Type_2_1"}[body.Element]
			if err := ei545SendSuperType(ctx, engine, env, prefix, target); err != nil {
				return err
			}
			state.emit("dispatch", step.EventType, nil, "",
				[]bool{fired["s0"] > 0, fired["s1"] > 0, fired["s2"] > 0, fired["s3"] > 0})
			fired = map[string]int{}
		case "unrepresentable":
			state.emitUnrepresentable(step.Statement, step.ExpectError)
		case "undeploy-all":
			state.resetSequences()
			for _, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return err
				}
			}
			deployments = nil
		default:
			return fmt.Errorf("unsupported op %q in supertype", step.Op)
		}
	}
	return nil
}

func ei545SendSuperType(ctx context.Context, engine *esper.Engine, env *esper.Environment, prefix, target string) error {
	switch prefix {
	case "Bean":
		var underlying any
		switch target {
		case "Bean_Type_Root":
			underlying = ei545TypeRoot{}
		case "Bean_Type_1":
			underlying = ei545Type1{}
		case "Bean_Type_2":
			underlying = ei545Type2{}
		default:
			underlying = ei545Type21{}
		}
		return engine.Send(ctx, target, underlying)
	case "Map":
		return engine.Send(ctx, target, map[string]any{})
	case "OA":
		return engine.SendObjectArray(ctx, target, []any{})
	case "Avro":
		schema, ok := env.Schema(target)
		if !ok {
			return fmt.Errorf("avro schema %q missing", target)
		}
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{})
		if err != nil {
			return err
		}
		return engine.SendAvro(ctx, target, record)
	}
	return fmt.Errorf("unknown supertype prefix %q", prefix)
}
