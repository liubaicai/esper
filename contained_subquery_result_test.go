package esper

import (
	"context"
	"reflect"
	"testing"
)

type containedSubqueryRoom struct {
	RoomID string `esper:"roomId"`
}

type containedSubqueryPerson struct {
	PersonID string `esper:"personId"`
}

type containedSubqueryPersonRooms struct {
	PersonID string                  `esper:"personId"`
	Rooms    []containedSubqueryRoom `esper:"rooms"`
}

func TestContainedSubqueryResultArrayMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSubqueryRoom](env, "ContainedRoom"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSubqueryPerson](env, "ContainedPerson"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSubqueryPersonRooms](env, "ContainedPersonAndRooms"); err != nil {
		t.Fatal(err)
	}
	roomSchema, ok := env.Schema("ContainedRoom")
	if !ok {
		t.Fatal("contained room schema is missing")
	}
	if _, err := CreateNamedWindow(env, "ContainedRoomWindow", roomSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, roomID := range []string{"r1", "r2"} {
		if err := engine.InsertNamedWindow(context.Background(), "ContainedRoomWindow", containedSubqueryRoom{RoomID: roomID}); err != nil {
			t.Fatal(err)
		}
	}

	roomValues := SubqueryValues[containedSubqueryRoom](
		FromNamedWindow(env, "ContainedRoomWindow"),
		EventValue[containedSubqueryRoom](),
	)
	producer := Select(From[containedSubqueryPerson](env, "ContainedPerson"),
		Alias("personId", Field[containedSubqueryPerson, string]("personId")),
		Alias("rooms", roomValues),
	).InsertInto("ContainedPersonAndRooms", StatementName("contained-subquery-producer"))
	producerPlan, err := env.Build(producer)
	if err != nil {
		t.Fatal(err)
	}

	personRooms := From[containedSubqueryPersonRooms](env, "ContainedPersonAndRooms")
	rooms := Unnest[containedSubqueryPersonRooms, containedSubqueryRoom](
		personRooms,
		Property[[]containedSubqueryRoom](EventValue[containedSubqueryPersonRooms](), "rooms"),
	)
	consumerPlan, err := env.Build(Select(rooms,
		Alias("personId", ContainedParentField[string]("personId")),
		Alias("roomId", Field[containedSubqueryRoom, string]("roomId")),
	).Query(StatementName("contained-subquery-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), producerPlan); err != nil {
		t.Fatal(err)
	}

	var rows []Row
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained subquery result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedSubqueryPerson{PersonID: "va"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("contained subquery rows = %#v", rows)
	}
	got := make([][]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, []string{row.Get("personId").Any().(string), row.Get("roomId").Any().(string)})
	}
	want := [][]string{{"va", "r1"}, {"va", "r2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("contained subquery rows = %#v, want %#v", got, want)
	}
}
