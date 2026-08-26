package esper

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Parity coverage for EPLInsertIntoPopulateUndStreamSelect (4 executions).
//
// Java source:
// regression-lib/src/main/java/com/espertech/esper/regressionlib/
// suite/epl/insertinto/EPLInsertIntoPopulateUndStreamSelect.java
//
// Runtime IDs pinned by the frozen work-unit contract:
//   - exec0 NamedWindowInheritsMap java-runtime-854310513054197751bb
//   - exec1 NamedWindowRep         java-runtime-9231859e3d46ac11c84e
//   - exec2 StreamInsertWWidenOA   java-runtime-b6c30a857600ff444a40
//   - exec3 Invalid                java-runtime-9c96c9930655f732930a
//
// Executions 0-2 are covered below as parity tests. Execution 3 is an
// INVALIDITY execution whose Java protocol has no trace step; following the
// epl_insert_into_transpose_stream_invalid_test.go precedent its Go
// equivalents are Go-unit build-error tests at the bottom of this file,
// registered as implemented (not differential-verified).
//
// Representation notes (frozen contract):
//   - `select mysrc.*` routes natively for every representation: a bare
//     insert-into route preserves the source properties and projects them
//     onto the declared target fields.
//   - `mysrc.*` combined with additional named columns uses Transpose plus
//     companion aliases natively for map/default targets only. plan.go
//     freezes the additional-properties Build gate to untyped Map targets,
//     so objectarray/avro/json representations express those projections as
//     explicit-Alias equivalents (approved representation note; the manifest
//     records the same split).

// ipusRepresentation is one event-underlying representation of exec1/exec2.
// The default representation maps to the annotation-less map registration,
// exactly like Java's EventRepresentationChoice.DEFAULT.
type ipusRepresentation struct {
	name     string
	kind     SchemaKind
	provided bool // JSON class-provided variant (MyLocalJsonProvided* classes)
}

var (
	ipusObjectArray  = ipusRepresentation{name: "objectarray", kind: SchemaObjectArray}
	ipusMap          = ipusRepresentation{name: "map", kind: SchemaMap}
	ipusAvro         = ipusRepresentation{name: "avro", kind: SchemaAvro}
	ipusJSON         = ipusRepresentation{name: "json", kind: SchemaJSON}
	ipusDefault      = ipusRepresentation{name: "default", kind: SchemaMap}
	ipusJSONProvided = ipusRepresentation{name: "json-provided", kind: SchemaJSON, provided: true}
)

func ipusRegister(env *Environment, rep ipusRepresentation, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	switch rep.kind {
	case SchemaObjectArray:
		return RegisterObjectArray(env, name, fields, opts...)
	case SchemaAvro:
		return RegisterAvro(env, name, fields, opts...)
	case SchemaJSON:
		return RegisterJSON(env, name, fields, opts...)
	default:
		return RegisterMap(env, name, fields, opts...)
	}
}

func ipusSend(ctx context.Context, t *testing.T, engine *Engine, rep ipusRepresentation, schema Schema, typeName string, myint int, mystr string) {
	t.Helper()
	var err error
	switch {
	case rep.kind == SchemaJSON:
		err = engine.SendJSON(ctx, typeName, []byte(`{"myint":`+strconv.Itoa(myint)+`,"mystr":"`+mystr+`"}`))
	case rep.kind == SchemaObjectArray:
		err = engine.SendObjectArray(ctx, typeName, []any{myint, mystr})
	case rep.kind == SchemaAvro:
		record, recordErr := NewAvroRecordFromMap(schema, map[string]any{"myint": myint, "mystr": mystr})
		if recordErr != nil {
			t.Fatal(recordErr)
		}
		err = engine.SendAvro(ctx, typeName, record)
	default:
		err = engine.SendRecord(ctx, typeName, map[string]any{"myint": myint, "mystr": mystr})
	}
	if err != nil {
		t.Fatal(err)
	}
}

// ipusWildcardTranspose builds the native `mysrc.*` payload projection: a
// transpose whose payload carries every source property by name.
func ipusWildcardTranspose() Selection {
	return Selection{Name: "", Expr: Transpose[map[string]any](
		Func2[int, string, map[string]any]("wildcard", func(myint int, mystr string) map[string]any {
			return map[string]any{"myint": myint, "mystr": mystr}
		}, Field[Event, int]("myint"), Field[Event, string]("mystr")),
	)}
}

func ipusSubscribe(t *testing.T, deployment *Deployment) *[]Result {
	t.Helper()
	var results []Result
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &results
}

// ipusFieldValue reads one observable field from a statement result. Route
// producers emit either the routed target Event or the projected Row; both
// answer property reads identically. Absent and null values normalize to a
// Go nil, mirroring the oracle's null rendering.
func ipusFieldValue(result Result, field string) any {
	if event, ok := result.Event(); ok {
		value := event.Get(field)
		if value.IsPresent() {
			return value.Any()
		}
		return nil
	}
	if row, ok := result.Row(); ok {
		value := row.Get(field)
		if value.IsPresent() {
			return value.Any()
		}
		return nil
	}
	return nil
}

func ipusAssertRow(t *testing.T, label string, result Result, want map[string]any) {
	t.Helper()
	for field, expected := range want {
		got := ipusFieldValue(result, field)
		if got != expected {
			t.Fatalf("%s: field %s = %#v (%T), want %#v (%T)", label, field, got, got, expected, expected)
		}
	}
}

// TestEPLInsertIntoNamedWindowInheritsMapParity covers exec0
// EPLInsertIntoNamedWindowInheritsMap
// (java-runtime-854310513054197751bb): an on-merge over ChildEvent merges
// into IncidentWindow#keepall with the match condition
// `e.id = cast(w.event.id? as string)` and a conditional not-matched INSERT
// populated with the subtype trigger event itself. The three engine
// confirmations required by the frozen contract are exercised here:
//
//  1. a not-matched INSERT action carrying its own WHERE condition
//     (ThenInsertIntoTargetWhen inside WhenNotMatchedActions);
//  2. a subtype event (ChildEvent) assigned into the supertype-typed column
//     (Event) of the merge insert;
//  3. an OptionalProperty + Cast chain inside the merge where-clause.
func TestEPLInsertIntoNamedWindowInheritsMapParity(t *testing.T) {
	env := NewEnvironment()
	eventSchema, err := RegisterObjectArray(env, "Event", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "ChildEvent", []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("action", reflect.TypeOf("")),
	}, WithSchemaParent(eventSchema)); err != nil {
		t.Fatal(err)
	}
	incidentSchema, err := RegisterObjectArray(env, "Incident", []FieldSpec{
		FieldDef("name", reflect.TypeOf("")),
		FieldDef("event", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("event", eventSchema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "IncidentWindow", incidentSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}

	action := Field[Event, string]("action")
	match := Equal[string](
		Field[Event, string]("id"),
		Cast[any, string](OptionalProperty[any](NamedWindowField[Event]("event"), "id")),
	)
	plan, err := env.Build(OnRecord(FromAny(env, "ChildEvent")).MergeIntoNamedWindowWhen(
		"IncidentWindow",
		match,
		WhenNotMatchedActions(
			ThenInsertIntoTargetWhen(
				Equal[string](action, Literal("INSERT")),
				SetColumn("name", Literal("ChildIncident")),
				SetColumn("event", EventValue[Event]()),
			),
		),
		WhenMatchedActions(
			ThenUpdate(Equal[string](action, Literal("INSERT")), SetColumn("event", EventValue[Event]())),
			ThenDelete(Equal[string](action, Literal("CLEAR"))),
		),
	).Query(StatementName("merge")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithRuntimeURI("java-runtime-854310513054197751bb"))
	defer func() { _ = engine.Close(context.Background()) }()
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendObjectArray(context.Background(), "ChildEvent", []any{"ID1", "INSERT"}); err != nil {
		t.Fatal(err)
	}

	window, ok := engine.NamedWindow("IncidentWindow")
	if !ok {
		t.Fatal("IncidentWindow is missing")
	}
	snapshot, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("snapshot = %d rows, want exactly the inserted incident", len(snapshot))
	}
	row := snapshot[0]
	if row.Get("name").Any() != "ChildIncident" {
		t.Fatalf("name = %#v, want ChildIncident", row.Get("name").Any())
	}
	fragment, ok := row.Get("event").Any().(Event)
	if !ok {
		t.Fatalf("nested event = %#v, want a ChildEvent fragment", row.Get("event").Any())
	}
	if fragment.TypeName() != "ChildEvent" {
		t.Fatalf("fragment type = %q, want ChildEvent", fragment.TypeName())
	}
	if fragment.Get("id").Any() != "ID1" || fragment.Get("action").Any() != "INSERT" {
		t.Fatalf("fragment fields = %#v/%#v, want ID1/INSERT",
			fragment.Get("id").Any(), fragment.Get("action").Any())
	}
}

// TestEPLInsertIntoNamedWindowRepParity covers exec1 EPLInsertIntoNamedWindowRep
// (java-runtime-9231859e3d46ac11c84e) across objectarray/map/avro/json/default
// (the json-provided class representation is skipped, mirroring the Java skip
// because the assertion relies on schema inheritance). Phase a routes the
// plain `select mya.*` wildcard natively for every representation; phase b
// adds `, 1 as addprop`, expressed natively (transpose + companion alias) for
// map/default and as explicit-Alias equivalents for objectarray/avro/json.
func TestEPLInsertIntoNamedWindowRepParity(t *testing.T) {
	for _, rep := range []ipusRepresentation{ipusObjectArray, ipusMap, ipusAvro, ipusJSON, ipusDefault} {
		t.Run(rep.name, func(t *testing.T) {
			ctx := context.Background()
			env := NewEnvironment()
			aSchema, err := ipusRegister(env, rep, "A", []FieldSpec{
				FieldDef("myint", reflect.TypeOf(0)),
				FieldDef("mystr", reflect.TypeOf("")),
			})
			if err != nil {
				t.Fatal(err)
			}
			cSchema, err := ipusRegister(env, rep, "C", []FieldSpec{
				FieldDef("addprop", reflect.TypeOf(0)),
			}, WithSchemaParent(aSchema))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "MyWindow", cSchema, NamedWindowRetention(TimeWindow(5*24*time.Hour))); err != nil {
				t.Fatal(err)
			}

			engine := NewEngine(env, WithRuntimeURI("java-runtime-9231859e3d46ac11c84e"))
			defer func() { _ = engine.Close(ctx) }()
			s0Plan, err := env.Build(FromNamedWindow(env, "MyWindow").Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			s0Deployment, err := engine.Deploy(ctx, s0Plan)
			if err != nil {
				t.Fatal(err)
			}
			s0Results := ipusSubscribe(t, s0Deployment)

			// Phase a: select underlying (`select mya.* from A as mya`) as a
			// bare wildcard route, native for every representation. The
			// inherited addprop column arrives null.
			insertA, err := env.Build(FromAny(env, "A").InsertInto("MyWindow", StatementName("insert")))
			if err != nil {
				t.Fatal(err)
			}
			insertADeployment, err := engine.Deploy(ctx, insertA)
			if err != nil {
				t.Fatal(err)
			}
			ipusSend(ctx, t, engine, rep, aSchema, "A", 123, "abc")
			if len(*s0Results) != 1 {
				t.Fatalf("phase-a results = %d, want one", len(*s0Results))
			}
			ipusAssertRow(t, "phase-a", (*s0Results)[0], map[string]any{
				"myint": 123, "mystr": "abc", "addprop": nil,
			})
			// Undeploy only the insert module; the window persists.
			if err := insertADeployment.Undeploy(ctx); err != nil {
				t.Fatal(err)
			}

			// Phase b: `select mya.*, 1 as addprop`. Map/default targets take
			// the native transpose + companion-alias form; objectarray/avro/
			// json use explicit-Alias equivalents because the frozen Build
			// gate allows additional properties only beside an untyped Map
			// transpose target (representation note).
			var insertB Plan
			if rep.kind == SchemaMap {
				insertB, err = env.Build(FromAny(env, "A").Select(
					ipusWildcardTranspose(),
					Alias("addprop", Literal(1)),
				).InsertInto("MyWindow", StatementName("insert")))
			} else {
				insertB, err = env.Build(FromAny(env, "A").Select(
					Alias("myint", Field[Event, int]("myint")),
					Alias("mystr", Field[Event, string]("mystr")),
					Alias("addprop", Literal(1)),
				).InsertInto("MyWindow", StatementName("insert")))
			}
			if err != nil {
				t.Fatal(err)
			}
			insertBDeployment, err := engine.Deploy(ctx, insertB)
			if err != nil {
				t.Fatal(err)
			}
			ipusSend(ctx, t, engine, rep, aSchema, "A", 456, "def")
			if len(*s0Results) != 2 {
				t.Fatalf("phase-b results = %d, want two total", len(*s0Results))
			}
			ipusAssertRow(t, "phase-b", (*s0Results)[1], map[string]any{
				"myint": 456, "mystr": "def", "addprop": 1,
			})
			_ = insertBDeployment
		})
	}
}

// ipusJsonProvidedSrc mirrors MyLocalJsonProvidedSrc.
type ipusJsonProvidedSrc struct {
	MyInt int    `esper:"myint"`
	MyStr string `esper:"mystr"`
}

// ipusJsonProvidedD1 mirrors MyLocalJsonProvidedD1 (addprop long).
type ipusJsonProvidedD1 struct {
	MyInt   int    `esper:"myint"`
	MyStr   string `esper:"mystr"`
	Addprop int64  `esper:"addprop"`
}

// ipusJsonProvidedD2 mirrors MyLocalJsonProvidedD2 (addprop double).
type ipusJsonProvidedD2 struct {
	MyInt   int     `esper:"myint"`
	MyStr   string  `esper:"mystr"`
	Addprop float64 `esper:"addprop"`
}

// ipusJsonProvidedD3 mirrors MyLocalJsonProvidedD3 (surplus myint absent).
type ipusJsonProvidedD3 struct {
	MyStr   string `esper:"mystr"`
	Addprop int    `esper:"addprop"`
}

// ipusJsonProvidedD4 mirrors MyLocalJsonProvidedD4.
type ipusJsonProvidedD4 struct {
	MyInt int    `esper:"myint"`
	MyStr string `esper:"mystr"`
}

func ipusRegisterJSONProvidedTypes(env *Environment) error {
	registrations := []struct {
		name   string
		fields []FieldSpec
	}{
		{"Src", []FieldSpec{FieldDef("myint", reflect.TypeOf(0)), FieldDef("mystr", reflect.TypeOf(""))}},
		{"D1", []FieldSpec{FieldDef("myint", reflect.TypeOf(0)), FieldDef("mystr", reflect.TypeOf("")), FieldDef("addprop", reflect.TypeOf(int64(0)))}},
		{"D2", []FieldSpec{FieldDef("mystr", reflect.TypeOf("")), FieldDef("myint", reflect.TypeOf(0)), FieldDef("addprop", reflect.TypeOf(float64(0)))}},
		{"D3", []FieldSpec{FieldDef("mystr", reflect.TypeOf("")), FieldDef("addprop", reflect.TypeOf(0))}},
		{"D4", []FieldSpec{FieldDef("myint", reflect.TypeOf(0)), FieldDef("mystr", reflect.TypeOf(""))}},
	}
	for _, registration := range registrations {
		var err error
		switch registration.name {
		case "Src":
			_, err = RegisterJSONFor[ipusJsonProvidedSrc](env, registration.name, registration.fields)
		case "D1":
			_, err = RegisterJSONFor[ipusJsonProvidedD1](env, registration.name, registration.fields)
		case "D2":
			_, err = RegisterJSONFor[ipusJsonProvidedD2](env, registration.name, registration.fields)
		case "D3":
			_, err = RegisterJSONFor[ipusJsonProvidedD3](env, registration.name, registration.fields)
		default:
			_, err = RegisterJSONFor[ipusJsonProvidedD4](env, registration.name, registration.fields)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// TestEPLInsertIntoStreamInsertWWidenOAParity covers exec2
// EPLInsertIntoStreamInsertWWidenOA (java-runtime-b6c30a857600ff444a40)
// across all six representations including json-provided. Every inserting
// statement carries its own listener, and each observation is undeployed
// before the next, mirroring undeployModuleContaining("s0"):
//
//	D1 `select 1 as addprop, mysrc.*` -> {123,"abc",1L}
//	D2 same into (mystr,myint,addprop double) -> {123,"abc",1d}
//	D3 same into (mystr,addprop int) -> surplus myint dropped
//	D4 `select mysrc.*` passthrough
//	eplFive/eplSix override precedence in both orders -> {999,"xxx"}
//
// Numeric literal typing is pinned: Literal(int64(1)) fills the long addprop
// and Literal(float64(1)) the double addprop. Map/default targets take the
// native transpose + companion-alias form; objectarray/avro/json(-provided)
// use explicit-Alias equivalents where the frozen Build gate requires it.
func TestEPLInsertIntoStreamInsertWWidenOAParity(t *testing.T) {
	for _, rep := range []ipusRepresentation{ipusObjectArray, ipusMap, ipusAvro, ipusJSON, ipusDefault, ipusJSONProvided} {
		t.Run(rep.name, func(t *testing.T) {
			ctx := context.Background()
			env := NewEnvironment()
			var srcSchema Schema
			var err error
			if rep.provided {
				err = ipusRegisterJSONProvidedTypes(env)
			} else {
				srcSchema, err = ipusRegister(env, rep, "Src", []FieldSpec{
					FieldDef("myint", reflect.TypeOf(0)),
					FieldDef("mystr", reflect.TypeOf("")),
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			if !rep.provided {
				targets := []struct {
					name   string
					fields []FieldSpec
				}{
					{"D1", []FieldSpec{FieldDef("myint", reflect.TypeOf(0)), FieldDef("mystr", reflect.TypeOf("")), FieldDef("addprop", reflect.TypeOf(int64(0)))}},
					{"D2", []FieldSpec{FieldDef("mystr", reflect.TypeOf("")), FieldDef("myint", reflect.TypeOf(0)), FieldDef("addprop", reflect.TypeOf(float64(0)))}},
					{"D3", []FieldSpec{FieldDef("mystr", reflect.TypeOf("")), FieldDef("addprop", reflect.TypeOf(0))}},
					{"D4", []FieldSpec{FieldDef("myint", reflect.TypeOf(0)), FieldDef("mystr", reflect.TypeOf(""))}},
				}
				for _, target := range targets {
					if _, err := ipusRegister(env, rep, target.name, target.fields); err != nil {
						t.Fatal(err)
					}
				}
			}

			engine := NewEngine(env, WithRuntimeURI("java-runtime-b6c30a857600ff444a40"))
			defer func() { _ = engine.Close(ctx) }()

			native := rep.kind == SchemaMap
			runInsert := func(label string, query Query, want map[string]any) {
				t.Helper()
				plan, err := env.Build(query)
				if err != nil {
					t.Fatalf("%s: build: %v", label, err)
				}
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					t.Fatalf("%s: deploy: %v", label, err)
				}
				results := ipusSubscribe(t, deployment)
				ipusSend(ctx, t, engine, rep, srcSchema, "Src", 123, "abc")
				if len(*results) != 1 {
					t.Fatalf("%s: results = %d, want one", label, len(*results))
				}
				ipusAssertRow(t, label, (*results)[0], want)
				if err := deployment.Undeploy(ctx); err != nil {
					t.Fatalf("%s: undeploy: %v", label, err)
				}
			}

			// D1: `select 1 as addprop, mysrc.*` with addprop long.
			if native {
				runInsert("D1", FromAny(env, "Src").Select(
					ipusWildcardTranspose(),
					Alias("addprop", Literal(int64(1))),
				).InsertInto("D1", StatementName("s0")), map[string]any{
					"myint": 123, "mystr": "abc", "addprop": int64(1),
				})
			} else {
				runInsert("D1", FromAny(env, "Src").Select(
					Alias("addprop", Literal(int64(1))),
					Alias("myint", Field[Event, int]("myint")),
					Alias("mystr", Field[Event, string]("mystr")),
				).InsertInto("D1", StatementName("s0")), map[string]any{
					"myint": 123, "mystr": "abc", "addprop": int64(1),
				})
			}

			// D2: same projection into (mystr, myint, addprop double).
			if native {
				runInsert("D2", FromAny(env, "Src").Select(
					ipusWildcardTranspose(),
					Alias("addprop", Literal(float64(1))),
				).InsertInto("D2", StatementName("s0")), map[string]any{
					"myint": 123, "mystr": "abc", "addprop": float64(1),
				})
			} else {
				runInsert("D2", FromAny(env, "Src").Select(
					Alias("addprop", Literal(float64(1))),
					Alias("myint", Field[Event, int]("myint")),
					Alias("mystr", Field[Event, string]("mystr")),
				).InsertInto("D2", StatementName("s0")), map[string]any{
					"myint": 123, "mystr": "abc", "addprop": float64(1),
				})
			}

			// D3: `select 1 as addprop, mysrc.*` into (mystr, addprop int);
			// the surplus myint column is dropped.
			if native {
				runInsert("D3", FromAny(env, "Src").Select(
					ipusWildcardTranspose(),
					Alias("addprop", Literal(1)),
				).InsertInto("D3", StatementName("s0")), map[string]any{
					"mystr": "abc", "addprop": 1,
				})
			} else {
				runInsert("D3", FromAny(env, "Src").Select(
					Alias("addprop", Literal(1)),
					Alias("mystr", Field[Event, string]("mystr")),
				).InsertInto("D3", StatementName("s0")), map[string]any{
					"mystr": "abc", "addprop": 1,
				})
			}

			// D4 passthrough: `select mysrc.*` routes natively everywhere.
			runInsert("D4-passthrough", FromAny(env, "Src").InsertInto("D4", StatementName("s0")), map[string]any{
				"myint": 123, "mystr": "abc",
			})

			// eplFive: `select mysrc.*, 999 as myint, 'xxx' as mystr` - the
			// override follows the wildcard. Native map/default keeps both
			// terms (companion aliases overlay the transpose payload);
			// non-map reps collapse to the overriding literals because the
			// flat Go projection rejects duplicate aliases and Esper's
			// override rule makes the shadowed wildcard terms unobservable
			// (representation note).
			if native {
				runInsert("eplFive", FromAny(env, "Src").Select(
					ipusWildcardTranspose(),
					Alias("myint", Literal(999)),
					Alias("mystr", Literal("xxx")),
				).InsertInto("D4", StatementName("s0")), map[string]any{
					"myint": 999, "mystr": "xxx",
				})
			} else {
				runInsert("eplFive", FromAny(env, "Src").Select(
					Alias("myint", Literal(999)),
					Alias("mystr", Literal("xxx")),
				).InsertInto("D4", StatementName("s0")), map[string]any{
					"myint": 999, "mystr": "xxx",
				})
			}
			if native {
				runInsert("eplSix", FromAny(env, "Src").Select(
					Alias("myint", Literal(999)),
					Alias("mystr", Literal("xxx")),
					ipusWildcardTranspose(),
				).InsertInto("D4", StatementName("s0")), map[string]any{
					"myint": 999, "mystr": "xxx",
				})
			} else {
				runInsert("eplSix", FromAny(env, "Src").Select(
					Alias("mystr", Literal("xxx")),
					Alias("myint", Literal(999)),
				).InsertInto("D4", StatementName("s0")), map[string]any{
					"myint": 999, "mystr": "xxx",
				})
			}
		})
	}
}

// ipusInvalidE1 mirrors the typed long-typed target shape of the exec3
// mismatch-in-type invalid (Java runtime java-runtime-9c96c9930655f732930a).
type ipusInvalidE1 struct {
	MyInt int64 `esper:"myint"`
}

// ipusInvalidE2 mirrors the exec3 mismatch-in-column-name target.
type ipusInvalidE2 struct {
	SomeProp int64 `esper:"someprop"`
}

// ipusAssertBuildRejected asserts that Build rejects the query with the
// ErrorInvalidRule sentinel that wraps route validation and that the message
// contains every Go-produced fragment. Following the
// epl_insert_into_transpose_stream_invalid_test.go precedent these tests pin
// GO'S ACTUAL messages, not the verbatim Java diagnostics.
func ipusAssertBuildRejected(t *testing.T, name string, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("invalid %s was accepted at Build", name)
	}
	if !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("invalid %s error = %v, want ErrorInvalidRule", name, err)
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("invalid %s error = %v, missing fragment %q", name, err, fragment)
		}
	}
}

// TestEPLInsertIntoPopulateUndStreamSelectInvalidBuild covers exec3
// EPLInsertIntoInvalid (java-runtime-9c96c9930655f732930a) as Go-unit
// build-error tests, registered IMPLEMENTED-NOT-DIFFERENTIAL-VERIFIED per the
// transpose_stream_invalid precedent. Java flags the whole execution
// INVALIDITY per representation, so its protocol has no trace step and the
// scenario oracle preserves only the verbatim Java messages:
//
//   - non-avro reps: "Type by name 'E1' in property 'myint' expected Long but
//     receives Integer" / avro schema variant;
//   - "Failed to find column 'otherprop' in target type 'E2'".
//
// Go lacks static alias.* expansion against typed targets, so those exact
// shapes are unrepresentable; the tests below pin Go's actual Build behavior
// for the same scenarios instead. The manifest keeps the capability entry in
// "remaining" with the verbatim Java text.
func TestEPLInsertIntoPopulateUndStreamSelectInvalidBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterMap(env, "Src", []FieldSpec{
		FieldDef("myint", reflect.TypeOf(0)),
		FieldDef("mystr", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ipusInvalidE1](env, "E1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ipusInvalidE2](env, "E2"); err != nil {
		t.Fatal(err)
	}

	t.Run("mismatch-in-type-transpose-into-typed-target", func(t *testing.T) {
		// Java: insert into E1 select mysrc.* from Src as mysrc with
		// E1.myint long vs Src.myint int is rejected at compile time. The
		// typed Go equivalent routes a map-shaped wildcard payload into the
		// struct-backed E1 target, which Build rejects because the payload
		// cannot be converted to the target underlying.
		_, err := env.Build(FromAny(env, "Src").Select(
			ipusWildcardTranspose(),
		).InsertInto("E1", StatementName("s0")))
		ipusAssertBuildRejected(t, "type-mismatch", err,
			"cannot be converted to target event type \"E1\"")
	})

	t.Run("mismatch-in-type-numeric-widening-accepted", func(t *testing.T) {
		// Approved API-surface difference: an explicit int expression into
		// the int64 E1.myint column is accepted at Build (Go coerces numeric
		// projections; fieldExpressionTypesCompatible), whereas Java rejects
		// Integer-into-Long statically. Pinned here so a future tightening
		// of validateRoute updates this file deliberately.
		if _, err := env.Build(FromAny(env, "Src").Select(
			Alias("myint", Field[Event, int]("myint")),
		).InsertInto("E1", StatementName("s0"))); err != nil {
			t.Fatalf("numeric widening route = %v, want accepted", err)
		}
	})

	t.Run("mismatch-in-column-name", func(t *testing.T) {
		// Java: "Failed to find column 'otherprop' in target type 'E2'".
		// Go rejects the same surplus column at route validation.
		_, err := env.Build(FromAny(env, "Src").Select(
			Alias("otherprop", Literal(1)),
		).InsertInto("E2", StatementName("s0")))
		ipusAssertBuildRejected(t, "unknown-column", err,
			"route projection \"otherprop\" is not a field of target schema \"E2\"")
	})
}
