package esper

import (
	"strings"
	"testing"
)

type aggInvalidS0 struct {
	ID    string
	P00   string
	P01   string
	Value int
}

type aggInvalidS1 struct {
	ID    string
	P10   string
	P11   string
	Value int
}

func TestAggregateRejectsSortedNoWindow(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[aggInvalidS0](env, "S0"); err != nil {
		t.Fatal(err)
	}
	_, err := env.Build(From[aggInvalidS0](env, "S0").Aggregate(
		Alias("c0", SortedValues[string](Field[aggInvalidS0, string]("P00"), false)),
	).Query())
	if err == nil || !strings.Contains(err.Error(), "requires that a data window is declared") {
		t.Fatalf("err = %v", err)
	}
}

func TestAggregateRejectsMaxByCrossStream(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[aggInvalidS0](env, "S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[aggInvalidS1](env, "S1"); err != nil {
		t.Fatal(err)
	}
	_, err := env.Build(JoinMany(
		JoinSource(From[aggInvalidS0](env, "S0").Window(LastEvent())),
		JoinSource(From[aggInvalidS1](env, "S1").Window(LastEvent())),
	).On(OnSourcesEqual(0, JoinField[string](0, "ID"), 1, JoinField[string](1, "ID"))).Aggregate(
		Alias("c0", MaxBy[Event, string](
			EventValue[Event](),
			Concat(JoinField[string](0, "P00"), JoinField[string](1, "P10")),
		)),
	).Query())
	if err == nil || !strings.Contains(err.Error(), "same stream") {
		t.Fatalf("err = %v", err)
	}
}
