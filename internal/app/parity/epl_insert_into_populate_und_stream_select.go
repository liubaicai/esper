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

// Local mirrors of the suite's json-provided underlyings; field names, types
// and declaration order match the pinned MyLocalJsonProvided* classes so the
// Go RegisterJSONFor registrations resolve identically to the Java
// @JsonSchema(className='...MyLocalJsonProvided*') annotations.
type iupsLocalJsonProvidedSrc struct {
	Myint int    `esper:"myint"`
	Mystr string `esper:"mystr"`
}

type iupsLocalJsonProvidedD1 struct {
	Myint   int    `esper:"myint"`
	Mystr   string `esper:"mystr"`
	Addprop int64  `esper:"addprop"`
}

type iupsLocalJsonProvidedD2 struct {
	Mystr   string  `esper:"mystr"`
	Myint   int     `esper:"myint"`
	Addprop float64 `esper:"addprop"`
}

type iupsLocalJsonProvidedD3 struct {
	Mystr   string `esper:"mystr"`
	Addprop int    `esper:"addprop"`
}

type iupsLocalJsonProvidedD4 struct {
	Myint int    `esper:"myint"`
	Mystr string `esper:"mystr"`
}

const eplInsertIntoPopulateUndStreamSelectJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoPopulateUndStreamSelectJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoPopulateUndStreamSelect.java",
}

var (
	eplInsertIntoPopulateUndStreamSelectJavaRuntimeIDs = []string{
		"java-runtime-854310513054197751bb",
		"java-runtime-9231859e3d46ac11c84e",
		"java-runtime-b6c30a857600ff444a40",
	}
	eplInsertIntoPopulateUndStreamSelectJavaExecutions = []string{
		"EPLInsertIntoNamedWindowInheritsMap",
		"EPLInsertIntoNamedWindowRep",
		"EPLInsertIntoStreamInsertWWidenOA",
	}
)

// Representation note (frozen contract; recorded alongside the approved
// transpose+additional-columns Build-gate freeze): map and default
// representations express `select mya.*` through the native transpose forms,
// while objectarray/avro/json/json-provided representations use the
// explicit-Alias equivalents below. Both spellings project identical rows.
func iupsNativeTranspose(kind string) bool {
	return kind == "map" || kind == "default"
}

func runEplInsertIntoPopulateUndStreamSelectScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var sequence uint64
	found := false
	for _, caseName := range []string{"named-window-inherits-map", "named-window-rep", "stream-insert-w-widen"} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		var caseTrace compat.Trace
		switch caseName {
		case "named-window-inherits-map":
			caseTrace, err = runIUPSNamedWindowInheritsMap(ctx, caseScenario, &sequence)
		case "named-window-rep":
			caseTrace, err = runIUPSNamedWindowRep(ctx, caseScenario, &sequence)
		case "stream-insert-w-widen":
			caseTrace, err = runIUPSStreamInsertWWiden(ctx, caseScenario, &sequence)
		}
		if err != nil {
			return compat.Trace{}, fmt.Errorf("insert into populate und stream select case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("insert into populate und stream select scenario %q has no supported cases", scenario.ID)
	}
	return rewriteIUPSTrace(trace), nil
}

// runIUPSNamedWindowInheritsMap mirrors EPLInsertIntoNamedWindowInheritsMap:
// object-array schemas Event(), ChildEvent(id,action) inherits Event and
// Incident(name,event Event); a merge rule whose where clause chains
// OptionalProperty+Cast over the supertype-typed fragment (Java's
// cast(w.event.id? as string)); not-matched insert gated on action='INSERT',
// matched update storing the subtype instance, then matched delete. The
// window iteration renders as one snapshot record whose nested event
// fragment is a row-of-fields.
func runIUPSNamedWindowInheritsMap(ctx context.Context, scenario compat.Scenario, sequence *uint64) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	eventSchema, err := esper.RegisterObjectArray(env, "Event", nil)
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterObjectArray(env, "ChildEvent", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf("")),
		esper.FieldDef("action", reflect.TypeOf("")),
	}, esper.WithSchemaParent(eventSchema)); err != nil {
		return compat.Trace{}, err
	}
	incidentSchema, err := esper.RegisterObjectArray(env, "Incident", []esper.FieldSpec{
		esper.FieldDef("name", reflect.TypeOf("")),
		esper.FieldDef("event", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("event", eventSchema))
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "IncidentWindow", incidentSchema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return compat.Trace{}, err
	}

	insertAction := esper.Equal[string](esper.Field[any, string]("action"), esper.Literal("INSERT"))
	match := esper.Equal[string](
		esper.Field[any, string]("id"),
		esper.Cast[any, string](esper.OptionalProperty[string](esper.NamedWindowField[esper.Event]("event"), "id")),
	)
	mergePlan, err := env.Build(esper.OnRecord(esper.FromAny(env, "ChildEvent")).MergeIntoNamedWindowWhen("IncidentWindow", match,
		esper.WhenNotMatched(insertAction,
			esper.SetColumn("name", esper.Literal("ChildIncident")),
			esper.SetColumn("event", esper.EventValue[esper.Event]()),
		),
		esper.WhenMatched(insertAction,
			esper.SetColumn("event", esper.EventValue[esper.Event]())),
		esper.WhenMatchedDelete(insertAction),
	).Query(esper.StatementName("on-merge")))
	if err != nil {
		return compat.Trace{}, err
	}
	windowPlan, err := env.Build(esper.FromNamedWindow(env, "IncidentWindow").Query(esper.StatementName("window")))
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env, esper.WithRuntimeURI(eplInsertIntoPopulateUndStreamSelectJavaRuntimeIDs[0]))
	defer func() { _ = engine.Close(context.Background()) }()
	for _, plan := range []esper.Plan{mergePlan, windowPlan} {
		if _, err := engine.Deploy(ctx, plan); err != nil {
			return compat.Trace{}, err
		}
	}
	windowStatement, err := iupsFindStatement(engine, "window")
	if err != nil {
		return compat.Trace{}, err
	}

	const caseName = "named-window-inherits-map"
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, step := range scenario.Steps {
		if err := contextErrForIUPS(ctx); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload struct {
				ID     string `json:"id"`
				Action string `json:"action"`
			}
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("decode ChildEvent payload: %w", err)
			}
			if step.EventType != "ChildEvent" {
				return trace, fmt.Errorf("unexpected named-window-inherits-map event type %q", step.EventType)
			}
			if err := engine.SendObjectArray(ctx, step.EventType, []any{payload.ID, payload.Action}); err != nil {
				return trace, err
			}
		case "snapshot":
			result, err := windowStatement.Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			*sequence++
			record := compat.TraceRecord{Case: caseName, Operation: "snapshot", Statement: step.Statement, Sequence: *sequence, Time: currentTimeString(engine)}
			record.New = compat.NormalizeResults(result.Results())
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported named-window-inherits-map step %q", step.Op)
		}
	}
	return trace, nil
}

// runIUPSNamedWindowRep mirrors EPLInsertIntoNamedWindowRep over the five
// non-json-provided representations in inventory order (the pinned assertion
// skips json-provided because it relies on type inheritance). Each
// representation runs phase a (`select mya.*`) then undeploys the insert
// module while the time(5 days) window persists, then phase b
// (`select mya.*, 1 as addprop`); the s0 listener on the window observes
// both routed rows per representation.
func runIUPSNamedWindowRep(ctx context.Context, scenario compat.Scenario, sequence *uint64) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	sends := iupsCaseSends(scenario.Steps)
	if len(sends) != 10 {
		return compat.Trace{}, fmt.Errorf("named-window-rep needs ten sends, got %d", len(sends))
	}
	const caseName = "named-window-rep"
	caseTraceLocal := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	// Two passes over the representations mirror the Java oracle's
	// JVM-per-phase invocation order (all phase-a rows precede all
	// phase-b rows). Each pass uses a fresh environment per representation.
	for _, phase := range []int{0, 1} {
		for kindIndex, kind := range []string{"objectarray", "map", "avro", "json", "default"} {
			env := esper.NewEnvironment()
			aSchema, cSchema, err := iupsRegisterNamedWindowPair(env, kind)
			if err != nil {
				return compat.Trace{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", cSchema,
				esper.NamedWindowRetention(esper.TimeWindow(5*24*time.Hour))); err != nil {
				return compat.Trace{}, err
			}
			s0Plan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").Query(esper.StatementName("s0")))
			if err != nil {
				return compat.Trace{}, err
			}
			var selections []esper.Selection
			if phase == 1 {
				selections = iupsWildcardSelection(kind,
					[]esper.Selection{esper.Alias("addprop", esper.Literal(1))})
			} else {
				selections = iupsWildcardSelection(kind, nil)
			}
			producerPlan, err := env.Build(esper.FromAny(env, "A").
				Select(selections...).
				InsertInto("MyWindow", esper.StatementName("insert")))
			if err != nil {
				return compat.Trace{}, err
			}

			engine := esper.NewEngine(env, esper.WithRuntimeURI(eplInsertIntoPopulateUndStreamSelectJavaRuntimeIDs[1]))
			s0Deployment, err := engine.Deploy(ctx, s0Plan)
			if err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			s0Statements := s0Deployment.Statements()
			if len(s0Statements) != 1 {
				_ = engine.Close(context.Background())
				return compat.Trace{}, fmt.Errorf("named-window-rep s0 deployment has %d statements", len(s0Statements))
			}
			if err := iupsSubscribe(s0Statements[0], caseName, sequence, &caseTraceLocal, engine); err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			if _, err := engine.Deploy(ctx, producerPlan); err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			send := sends[kindIndex*2+phase]
			if err := iupsSendSourceEvent(ctx, engine, kind, aSchema, send); err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			if err := engine.Close(context.Background()); err != nil {
				return compat.Trace{}, err
			}
		}
	}
	return caseTraceLocal, nil
}

// iupsWidenStage pins one inserting-statement observation of
// tryAssertionStreamInsertWWidenMap: the routed target plus the projected
// source-field columns for the explicit-Alias representations (native
// transpose representations derive them from the wildcard), and any
// wildcard-extra literal columns such as `1 as addprop` or the eplFive/eplSix
// override pair `999 as myint, 'xxx' as mystr`.
type iupsWidenStage struct {
	target       string
	projected    []string
	extraColumns []iupsColumn
}

// iupsColumn carries one named literal projection; the concrete Go types pin
// the numeric literal typings the suite asserts (long 1 renders integer 1,
// double 1d renders "1.0" through javaDoubleString).
type iupsColumn struct {
	name  string
	value any
}

func iupsWidenStages() []iupsWidenStage {
	return []iupsWidenStage{
		{target: "D1", projected: []string{"myint", "mystr"},
			extraColumns: []iupsColumn{{name: "addprop", value: int64(1)}}},
		{target: "D2", projected: []string{"myint", "mystr"},
			extraColumns: []iupsColumn{{name: "addprop", value: float64(1)}}},
		{target: "D3", projected: []string{"mystr"},
			extraColumns: []iupsColumn{{name: "addprop", value: 1}}},
		{target: "D4", projected: []string{"myint", "mystr"}},
		{target: "D4", extraColumns: []iupsColumn{{name: "myint", value: 999}, {name: "mystr", value: "xxx"}}},
		{target: "D4", extraColumns: []iupsColumn{{name: "myint", value: 999}, {name: "mystr", value: "xxx"}}},
	}
}

// runIUPSStreamInsertWWiden mirrors EPLInsertIntoStreamInsertWWidenOA: all
// six representations times six inserting-statement observations (D1..D4,
// eplFive, eplSix), each statement named s0, subscribed before exactly one
// Src send, then undeployed - the listener sits on the inserting statements
// themselves, matching runStreamInsertAssertion.
func runIUPSStreamInsertWWiden(ctx context.Context, scenario compat.Scenario, sequence *uint64) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	sends := iupsCaseSends(scenario.Steps)
	if len(sends) != 36 {
		return compat.Trace{}, fmt.Errorf("stream-insert-w-widen needs thirty-six sends, got %d", len(sends))
	}
	stages := iupsWidenStages()
	const caseName = "stream-insert-w-widen"
	widenTrace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	cursor := 0
	for _, kind := range []string{"objectarray", "map", "avro", "json", "jsonprovided", "default"} {
		env := esper.NewEnvironment()
		srcSchema, err := iupsRegisterWidenSchemas(env, kind)
		if err != nil {
			return compat.Trace{}, err
		}
		engine := esper.NewEngine(env, esper.WithRuntimeURI(eplInsertIntoPopulateUndStreamSelectJavaRuntimeIDs[2]))
		for _, stage := range stages {
			plan, err := env.Build(esper.FromAny(env, "Src").
				Select(iupsWidenSelection(kind, stage)...).
				InsertInto(stage.target, esper.StatementName("s0")))
			if err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			statements := deployment.Statements()
			if len(statements) != 1 {
				_ = engine.Close(context.Background())
				return compat.Trace{}, fmt.Errorf("stream-insert-w-widen s0 deployment has %d statements", len(statements))
			}
			if err := iupsSubscribe(statements[0], caseName, sequence, &widenTrace, engine); err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			send := sends[cursor]
			cursor++
			if err := iupsSendSourceEvent(ctx, engine, kind, srcSchema, send); err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
			if err := engine.Undeploy(ctx, deployment.ID()); err != nil {
				_ = engine.Close(context.Background())
				return compat.Trace{}, err
			}
		}
		if err := engine.Close(context.Background()); err != nil {
			return compat.Trace{}, err
		}
	}
	return widenTrace, nil
}

// iupsRegisterNamedWindowPair registers A(myint int,mystr string) and
// C(addprop int) inherits A under the representation's underlying kind,
// returning both schemas (A backs the source events, C backs MyWindow).
func iupsRegisterNamedWindowPair(env *esper.Environment, kind string) (esper.Schema, esper.Schema, error) {
	baseFields := []esper.FieldSpec{
		esper.FieldDef("myint", reflect.TypeOf(0)),
		esper.FieldDef("mystr", reflect.TypeOf("")),
	}
	addprop := []esper.FieldSpec{esper.FieldDef("addprop", reflect.TypeOf(0))}
	switch kind {
	case "objectarray":
		aSchema, err := esper.RegisterObjectArray(env, "A", baseFields)
		if err != nil {
			return esper.Schema{}, esper.Schema{}, err
		}
		cSchema, err := esper.RegisterObjectArray(env, "C", addprop, esper.WithSchemaParent(aSchema))
		return aSchema, cSchema, err
	case "map", "default":
		aSchema, err := esper.RegisterMap(env, "A", baseFields)
		if err != nil {
			return esper.Schema{}, esper.Schema{}, err
		}
		cSchema, err := esper.RegisterMap(env, "C", addprop, esper.WithSchemaParent(aSchema))
		return aSchema, cSchema, err
	case "avro":
		aSchema, err := esper.RegisterAvro(env, "A", baseFields)
		if err != nil {
			return esper.Schema{}, esper.Schema{}, err
		}
		cSchema, err := esper.RegisterAvro(env, "C", addprop, esper.WithSchemaParent(aSchema))
		return aSchema, cSchema, err
	case "json":
		aSchema, err := esper.RegisterJSON(env, "A", baseFields)
		if err != nil {
			return esper.Schema{}, esper.Schema{}, err
		}
		cSchema, err := esper.RegisterJSON(env, "C", addprop, esper.WithSchemaParent(aSchema))
		return aSchema, cSchema, err
	default:
		return esper.Schema{}, esper.Schema{}, fmt.Errorf("unsupported named-window-rep representation %q", kind)
	}
}

// iupsRegisterWidenSchemas registers Src(myint int,mystr string) plus targets
// D1(myint,mystr,addprop long), D2(mystr,myint,addprop double),
// D3(mystr,addprop int) and D4(myint,mystr) under the representation's
// underlying kind; the json-provided kind registers the local
// MyLocalJsonProvided* mirror structs instead, returning the Src schema.
func iupsRegisterWidenSchemas(env *esper.Environment, kind string) (esper.Schema, error) {
	baseFields := []esper.FieldSpec{
		esper.FieldDef("myint", reflect.TypeOf(0)),
		esper.FieldDef("mystr", reflect.TypeOf("")),
	}
	d1 := []esper.FieldSpec{
		esper.FieldDef("myint", reflect.TypeOf(0)),
		esper.FieldDef("mystr", reflect.TypeOf("")),
		esper.FieldDef("addprop", reflect.TypeOf(int64(0))),
	}
	d2 := []esper.FieldSpec{
		esper.FieldDef("mystr", reflect.TypeOf("")),
		esper.FieldDef("myint", reflect.TypeOf(0)),
		esper.FieldDef("addprop", reflect.TypeOf(float64(0))),
	}
	d3 := []esper.FieldSpec{
		esper.FieldDef("mystr", reflect.TypeOf("")),
		esper.FieldDef("addprop", reflect.TypeOf(0)),
	}
	d4 := []esper.FieldSpec{
		esper.FieldDef("myint", reflect.TypeOf(0)),
		esper.FieldDef("mystr", reflect.TypeOf("")),
	}
	register := func(name string, fields []esper.FieldSpec) (esper.Schema, error) {
		switch kind {
		case "objectarray":
			return esper.RegisterObjectArray(env, name, fields)
		case "map", "default":
			return esper.RegisterMap(env, name, fields)
		case "avro":
			return esper.RegisterAvro(env, name, fields)
		case "json":
			return esper.RegisterJSON(env, name, fields)
		default:
			return esper.Schema{}, fmt.Errorf("unsupported stream-insert-w-widen representation %q", kind)
		}
	}
	switch kind {
	case "jsonprovided":
		if _, err := esper.RegisterJSONFor[iupsLocalJsonProvidedSrc](env, "Src", baseFields); err != nil {
			return esper.Schema{}, err
		}
		if _, err := esper.RegisterJSONFor[iupsLocalJsonProvidedD1](env, "D1", d1); err != nil {
			return esper.Schema{}, err
		}
		if _, err := esper.RegisterJSONFor[iupsLocalJsonProvidedD2](env, "D2", d2); err != nil {
			return esper.Schema{}, err
		}
		if _, err := esper.RegisterJSONFor[iupsLocalJsonProvidedD3](env, "D3", d3); err != nil {
			return esper.Schema{}, err
		}
		return esper.RegisterJSONFor[iupsLocalJsonProvidedD4](env, "D4", d4)
	default:
		srcSchema, err := register("Src", baseFields)
		if err != nil {
			return esper.Schema{}, err
		}
		for _, pair := range []struct {
			name   string
			fields []esper.FieldSpec
		}{{"D1", d1}, {"D2", d2}, {"D3", d3}, {"D4", d4}} {
			if _, err := register(pair.name, pair.fields); err != nil {
				return esper.Schema{}, err
			}
		}
		return srcSchema, nil
	}
}

// iupsWildcardSelection builds the phase-a/phase-b projections of
// named-window-rep: the native transpose form for map/default
// representations, the explicit-Alias equivalent otherwise; extras appends
// phase-b's `1 as addprop`.
func iupsWildcardSelection(kind string, extras []esper.Selection) []esper.Selection {
	selections := make([]esper.Selection, 0, 3+len(extras))
	if iupsNativeTranspose(kind) {
		selections = append(selections, esper.Selection{
			Expr: esper.Transpose[map[string]any](esper.EventValue[map[string]any]()),
		})
	} else {
		selections = append(selections,
			iupsIntAlias("myint"),
			iupsStringAlias("mystr"))
	}
	return append(selections, extras...)
}

// iupsWidenSelection builds one stream-insert-w-widen statement projection.
// Native transpose representations append extraColumns after the transposed
// wildcard (override semantics included); other representations project the
// target-driven explicit-Alias equivalent.
func iupsWidenSelection(kind string, stage iupsWidenStage) []esper.Selection {
	selections := make([]esper.Selection, 0, len(stage.projected)+len(stage.extraColumns)+1)
	if iupsNativeTranspose(kind) {
		selections = append(selections, esper.Selection{
			Expr: esper.Transpose[map[string]any](esper.EventValue[map[string]any]()),
		})
	} else {
		for _, source := range stage.projected {
			if source == "myint" {
				selections = append(selections, iupsIntAlias(source))
			} else {
				selections = append(selections, iupsStringAlias(source))
			}
		}
	}
	for _, extra := range stage.extraColumns {
		selections = append(selections, esper.Alias(extra.name, iupsLiteral(extra.value)))
	}
	return selections
}

// iupsLiteral pins the suite's numeric literal typings: int64 for long
// targets, float64 for double targets, plain int elsewhere.
func iupsLiteral(value any) esper.Expr {
	switch typed := value.(type) {
	case int64:
		return esper.Literal(typed)
	case float64:
		return esper.Literal(typed)
	case int:
		return esper.Literal(typed)
	case string:
		return esper.Literal(typed)
	default:
		return esper.Literal(value)
	}
}

func iupsIntAlias(name string) esper.Selection {
	return esper.Alias(name, esper.Field[any, int](name))
}

func iupsStringAlias(name string) esper.Selection {
	return esper.Alias(name, esper.Field[any, string](name))
}

func iupsCaseSends(steps []compat.Step) []compat.Step {
	sends := make([]compat.Step, 0, len(steps))
	for _, step := range steps {
		if step.Op == "send" {
			sends = append(sends, step)
		}
	}
	return sends
}

// iupsSendSourceEvent delivers one two-property source event (A/Src with
// {myint:int, mystr:string}) under the representation's send semantics:
// positional object arrays ordered by schema declaration, Avro records over
// the deployed schema, JSON text, or map underlyings.
func iupsSendSourceEvent(ctx context.Context, engine *esper.Engine, kind string, schema esper.Schema, step compat.Step) error {
	var values struct {
		Myint int    `json:"myint"`
		Mystr string `json:"mystr"`
	}
	eventType := step.EventType
	if err := json.Unmarshal(step.Payload, &values); err != nil {
		return fmt.Errorf("decode %s payload: %w", eventType, err)
	}
	switch kind {
	case "objectarray":
		byName := map[string]any{"myint": values.Myint, "mystr": values.Mystr}
		ordered := make([]any, 0, len(byName))
		for _, field := range schema.Fields() {
			ordered = append(ordered, byName[field.Name])
		}
		return engine.SendObjectArray(ctx, eventType, ordered)
	case "avro":
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{"myint": values.Myint, "mystr": values.Mystr})
		if err != nil {
			return err
		}
		return engine.Send(ctx, eventType, record)
	case "json", "jsonprovided":
		return engine.SendJSON(ctx, eventType, step.Payload)
	default:
		return engine.SendRecord(ctx, eventType, map[string]any{"myint": values.Myint, "mystr": values.Mystr})
	}
}

func iupsSubscribe(statement *esper.Statement, caseName string, sequence *uint64, trace *compat.Trace, engine *esper.Engine) error {
	current := statement
	_, err := current.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		*sequence++
		record := compat.TraceRecord{Case: caseName, Operation: "listener", Statement: current.Name(), Sequence: *sequence, Time: currentTimeString(engine)}
		record.New = compat.NormalizeResults(batch.New)
		record.Old = compat.NormalizeResults(batch.Old)
		trace.Records = append(trace.Records, record)
		return nil
	})
	return err
}

func iupsFindStatement(engine *esper.Engine, name string) (*esper.Statement, error) {
	if engine == nil {
		return nil, fmt.Errorf("insert into populate und stream select engine is nil")
	}
	for _, deployment := range engine.Deployments() {
		for _, statement := range deployment.Statements() {
			if statement.Name() == name {
				return statement, nil
			}
		}
	}
	return nil, fmt.Errorf("insert into populate und stream select statement %q is missing", name)
}

func contextErrForIUPS(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// rewriteIUPSTrace applies the record protocol's numeric rendering to both
// result streams: doubles render through javaDoubleString (the pinned 1d
// becomes "1.0"), matching the Java oracle byte for byte.
func rewriteIUPSTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for resultIndex := range record.New {
			iupsRewriteFields(&record.New[resultIndex].Fields)
		}
		for resultIndex := range record.Old {
			iupsRewriteFields(&record.Old[resultIndex].Fields)
		}
	}
	return trace
}

func iupsRewriteFields(fields *map[string]any) {
	if fields == nil {
		return
	}
	for name, value := range *fields {
		if typed, ok := value.(float64); ok {
			(*fields)[name] = javaDoubleString(typed)
		}
	}
}
