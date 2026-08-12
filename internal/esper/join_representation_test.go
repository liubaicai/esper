package esper

import (
	"context"
	"reflect"
	"strconv"
	"testing"
)

type joinRepresentationJSONProvided struct {
	ID  string `json:"id"`
	P00 int    `json:"p00"`
}

func TestJoinEventRepresentationsProjectFieldsMatchesEsper(t *testing.T) {
	fields := []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("p00", reflect.TypeOf(int(0))),
	}
	cases := []struct {
		name     string
		register func(*Environment, string) error
		send     func(*Engine, string, string, int) error
	}{
		{
			name: "map",
			register: func(env *Environment, name string) error {
				_, err := RegisterMap(env, name, fields)
				return err
			},
			send: func(engine *Engine, name, id string, p00 int) error {
				return engine.SendRecord(context.Background(), name, map[string]any{"id": id, "p00": p00})
			},
		},
		{
			name: "object-array",
			register: func(env *Environment, name string) error {
				_, err := RegisterObjectArray(env, name, fields)
				return err
			},
			send: func(engine *Engine, name, id string, p00 int) error {
				return engine.SendObjectArray(context.Background(), name, []any{id, p00})
			},
		},
		{
			name: "json",
			register: func(env *Environment, name string) error {
				_, err := RegisterJSON(env, name, fields)
				return err
			},
			send: func(engine *Engine, name, id string, p00 int) error {
				return engine.SendJSON(context.Background(), name, []byte(`{"id":"`+id+`","p00":`+strconv.Itoa(p00)+`}`))
			},
		},
		{
			name: "json-provided",
			register: func(env *Environment, name string) error {
				_, err := RegisterJSONFor[joinRepresentationJSONProvided](env, name, nil)
				return err
			},
			send: func(engine *Engine, name, id string, p00 int) error {
				return engine.SendJSON(context.Background(), name, []byte(`{"id":"`+id+`","p00":`+strconv.Itoa(p00)+`}`))
			},
		},
		{
			name: "xml",
			register: func(env *Environment, name string) error {
				_, err := RegisterXML(env, name, fields)
				return err
			},
			send: func(engine *Engine, name, id string, p00 int) error {
				return engine.SendXML(context.Background(), name, []byte(`<event><id>`+id+`</id><p00>`+strconv.Itoa(p00)+`</p00></event>`))
			},
		},
		{
			name: "avro",
			register: func(env *Environment, name string) error {
				_, err := RegisterAvro(env, name, fields)
				return err
			},
			send: func(engine *Engine, name, id string, p00 int) error {
				return engine.SendAvroJSON(context.Background(), name, []byte(`{"id":"`+id+`","p00":`+strconv.Itoa(p00)+`}`))
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			leftName := "JoinRepresentationLeft_" + testCase.name
			rightName := "JoinRepresentationRight_" + testCase.name
			if err := testCase.register(env, leftName); err != nil {
				t.Fatal(err)
			}
			if err := testCase.register(env, rightName); err != nil {
				t.Fatal(err)
			}
			query := JoinMany(
				JoinRecordSource(FromAny(env, leftName)).Window(KeepAll()),
				JoinRecordSource(FromAny(env, rightName)).Window(KeepAll()),
			).On(OnSourcesEqual(
				0, Field[Event, string]("id"),
				1, Field[Event, string]("id"),
			)).Select(
				SelectFrom(0, "s0id", Field[Event, string]("id")),
				SelectFrom(1, "s1id", Field[Event, string]("id")),
				SelectFrom(0, "s0p00", Field[Event, int]("p00")),
				SelectFrom(1, "s1p00", Field[Event, int]("p00")),
			).Query(StatementName("join-representation-" + testCase.name))
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			var rows []Row
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					if row, ok := result.Row(); ok {
						rows = append(rows, row)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := testCase.send(engine, leftName, "a", 1); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 0 {
				t.Fatalf("representation left-only rows = %#v", rows)
			}
			if err := testCase.send(engine, rightName, "a", 2); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("representation joined rows = %#v", rows)
			}
			row := rows[0]
			if row.Get("s0id").Any() != "a" || row.Get("s1id").Any() != "a" || row.Get("s0p00").Any() != 1 || row.Get("s1p00").Any() != 2 {
				t.Fatalf("representation joined row = %#v", row.AsMap())
			}
		})
	}
}

func TestJoinMapRepresentationSupportsNonUniqueKeys(t *testing.T) {
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("id", reflect.TypeOf("")),
		FieldDef("p00", reflect.TypeOf(int(0))),
	}
	if _, err := RegisterMap(env, "JoinMapNotUniqueS0", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "JoinMapNotUniqueS1", fields); err != nil {
		t.Fatal(err)
	}
	query := JoinMany(
		JoinRecordSource(FromAny(env, "JoinMapNotUniqueS0")).Window(KeepAll()),
		JoinRecordSource(FromAny(env, "JoinMapNotUniqueS1")).Window(KeepAll()),
	).On(OnSourcesEqual(
		0, Field[Event, string]("id"),
		1, Field[Event, string]("id"),
	)).Select(
		SelectFrom(0, "s0p00", Field[Event, int]("p00")),
		SelectFrom(1, "s1p00", Field[Event, int]("p00")),
	).Query(StatementName("join-map-not-unique"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	joined := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		joined += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		stream := "JoinMapNotUniqueS0"
		if i%2 == 0 {
			stream = "JoinMapNotUniqueS1"
		}
		if err := engine.SendRecord(context.Background(), stream, map[string]any{"id": "a", "p00": i}); err != nil {
			t.Fatal(err)
		}
	}
	if joined != 2500 {
		t.Fatalf("map non-unique join rows = %d, want 2500", joined)
	}
}

type joinWrapperSource struct {
	IntBoxed int `esper:"intBoxed"`
}

func TestJoinInsertedWrapperRepresentationSupportsNonUniqueKeys(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinWrapperSource](env, "JoinWrapperSource"); err != nil {
		t.Fatal(err)
	}
	wrapperFields := []FieldSpec{
		FieldDef("streamone", reflect.TypeOf("")),
		FieldDef("intBoxed", reflect.TypeOf(int(0))),
	}
	for _, name := range []string{"JoinWrapperS0", "JoinWrapperS1"} {
		if _, err := RegisterMap(env, name, wrapperFields); err != nil {
			t.Fatal(err)
		}
	}
	source := From[joinWrapperSource](env, "JoinWrapperSource")
	leftRoute := Select(
		source,
		Alias("streamone", Literal("s0")),
		Alias("intBoxed", Field[joinWrapperSource, int]("intBoxed")),
	).InsertInto("JoinWrapperS0", StatementName("join-wrapper-s0"))
	rightRoute := Select(
		source,
		Alias("streamone", Literal("s1")),
		Alias("intBoxed", Field[joinWrapperSource, int]("intBoxed")),
	).InsertInto("JoinWrapperS1", StatementName("join-wrapper-s1"))
	joinQuery := JoinMany(
		JoinRecordSource(FromAny(env, "JoinWrapperS0")).Window(KeepAll()),
		JoinRecordSource(FromAny(env, "JoinWrapperS1")).Window(KeepAll()),
	).On(OnSourcesEqual(
		0, Field[Event, int]("intBoxed"),
		1, Field[Event, int]("intBoxed"),
	)).Select(
		SelectSourceEvent(0, "s0"),
		SelectSourceEvent(1, "s1"),
	).Query(StatementName("join-wrapper-not-unique"))
	plans := make([]Plan, 0, 3)
	for _, query := range []Query{leftRoute, rightRoute, joinQuery} {
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, plan)
	}
	engine := NewEngine(env)
	var joinDeployment *Deployment
	for index, plan := range plans {
		deployed, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if index == len(plans)-1 {
			joinDeployment = deployed
		}
	}
	defer joinDeployment.Undeploy(context.Background())
	joined := 0
	if _, err := joinDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		joined += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := engine.SendEvent(context.Background(), joinWrapperSource{IntBoxed: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if joined == 0 {
		t.Fatal("inserted wrapper join produced no rows")
	}
}
