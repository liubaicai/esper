package esper

import (
	"context"
	"strings"
	"testing"
)

type rowRecogVariantS0 struct {
	ID int `esper:"id"`
}

type rowRecogVariantS1 struct {
	ID int `esper:"id"`
}

type rowRecogVariantS2 struct {
	ID int `esper:"id"`
}

func TestRowRecogVariantStreamPreservesMemberType(t *testing.T) {
	env := NewEnvironment()
	s0, err := RegisterStruct[rowRecogVariantS0](env, "SupportBean_S0")
	if err != nil {
		t.Fatal(err)
	}
	s1, err := RegisterStruct[rowRecogVariantS1](env, "SupportBean_S1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "MyVariantType", s0, s1); err != nil {
		t.Fatal(err)
	}

	memberType := func(name string) Expression[bool] {
		actual := Func1[Event, string]("event-type", func(event Event) string {
			return event.TypeName()
		}, EventValue[Event]())
		return Equal[string](actual, Literal(name))
	}
	variantStream := FromAny(env, "MyVariantType").Window(KeepAll())
	query := variantStream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("A", memberType("SupportBean_S0")).
		Define("B", memberType("SupportBean_S1")).
		Measures(
			Alias("a", TagField[int]("A", "id")),
			Alias("b", TagField[int]("B", "id")),
		).
		Query(StatementName("rowrecog-variant-stream"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	s0Insert, err := env.Build(From[rowRecogVariantS0](env, "SupportBean_S0").InsertInto("MyVariantType", StatementName("variant-s0")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), s0Insert); err != nil {
		t.Fatal(err)
	}
	s1Insert, err := env.Build(From[rowRecogVariantS1](env, "SupportBean_S1").InsertInto("MyVariantType", StatementName("variant-s1")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), s1Insert); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), rowRecogVariantS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), rowRecogVariantS1{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Get("a").Any() != 1 || (*rows)[0].Get("b").Any() != 2 {
		t.Fatalf("variant row-recognize listener rows = %#v", *rows)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results()) != 1 {
		t.Fatalf("variant row-recognize snapshot = %#v", snapshot.Results())
	}
	row, ok := snapshot.Results()[0].Row()
	if !ok || row.Get("a").Any() != 1 || row.Get("b").Any() != 2 {
		t.Fatalf("variant row-recognize snapshot row = %#v", snapshot.Results())
	}
}

func TestRowRecogVariantStreamRejectsInvalidMemberRoute(t *testing.T) {
	env := NewEnvironment()
	member, err := RegisterStruct[rowRecogVariantS0](env, "SupportBean_S0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterVariant(env, "MyVariantType", member); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[rowRecogVariantS2](env, "SupportBean_S2"); err != nil {
		t.Fatal(err)
	}
	invalidInsert := From[rowRecogVariantS2](env, "SupportBean_S2").InsertInto("MyVariantType", StatementName("variant-invalid-member"))
	if _, err := env.Build(invalidInsert); err == nil {
		t.Fatal("non-member variant route should fail during Build")
	} else if !strings.Contains(err.Error(), "not a member") {
		t.Fatalf("invalid member route error = %v", err)
	}
	unknown := FromAny(env, "UnknownType").MatchRecognize(RowVar("A")).
		Measures(Alias("a", TagField[int]("A", "id"))).
		Query(StatementName("variant-unknown-source"))
	if _, err := env.Build(unknown); err == nil {
		t.Fatal("unknown variant source should fail during Build")
	} else if !strings.Contains(err.Error(), "UnknownType") {
		t.Fatalf("unknown variant source error = %v", err)
	}

	engine := NewEngine(env)
	if err := engine.Send(context.Background(), "MyVariantType", rowRecogVariantS0{ID: 1}); err == nil {
		t.Fatal("direct struct payload must not be accepted by predefined variant")
	} else if !strings.Contains(err.Error(), "requires a routed member Event") {
		t.Fatalf("invalid variant route error = %v", err)
	}
}
