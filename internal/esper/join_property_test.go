package esper

import (
	"context"
	"testing"
)

type joinPropertyNested struct {
	Nested string            `esper:"nested"`
	Value  string            `esper:"value"`
	Mapped map[string]string `esper:"mapped"`
}

type joinPropertyComplex struct {
	Mapped  map[string]string    `esper:"mapped"`
	Indexed []joinPropertyNested `esper:"indexed"`
	Nested  joinPropertyNested   `esper:"nested"`
}

type joinPropertyCombined struct {
	Indexed []joinPropertyNested `esper:"indexed"`
}

func TestJoinPropertyAccessMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPropertyComplex](env, "JoinPropertyComplex"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPropertyCombined](env, "JoinPropertyCombined"); err != nil {
		t.Fatal(err)
	}
	complex := From[joinPropertyComplex](env, "JoinPropertyComplex").Window(LengthWindow(3))
	combined := From[joinPropertyCombined](env, "JoinPropertyCombined").Window(LengthWindow(3))
	complexIndexed := Field[joinPropertyComplex, []joinPropertyNested]("indexed")
	combinedIndexed := Field[joinPropertyCombined, []joinPropertyNested]("indexed")
	complexFirst := ArrayAt[joinPropertyNested](complexIndexed, Literal[int64](0))
	combinedFirst := ArrayAt[joinPropertyNested](combinedIndexed, Literal[int64](0))
	combinedThird := ArrayAt[joinPropertyNested](combinedIndexed, Literal[int64](2))
	plan, err := env.Build(Join(
		complex,
		combined,
		OnEqual(
			MapAt[string](Field[joinPropertyComplex, map[string]string]("mapped"), Literal("keyOne")),
			MapAt[string](Property[map[string]string](combinedThird, "mapped"), Literal("2ma")),
		),
		OnEqual(
			MapAt[string](Property[map[string]string](complexFirst, "mapped"), Literal("0ma")),
			Literal("0ma0"),
		),
	).Select(
		SelectLeft("nested", Property[string](Field[joinPropertyComplex, joinPropertyNested]("nested"), "nested")),
		SelectRight("indexed", combinedFirst),
		SelectLeft("left", JoinEventValue[Event](0)),
		SelectRight("right", JoinEventValue[Event](1)),
	).Query(StatementName("join-property-access")))
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
	combinedEvent := joinPropertyCombined{
		Indexed: []joinPropertyNested{
			{Mapped: map[string]string{"0ma": "combined-0"}},
			{Mapped: map[string]string{"1ma": "combined-1"}},
			{Mapped: map[string]string{"2ma": "join-key"}},
		},
	}
	complexEvent := joinPropertyComplex{
		Mapped: map[string]string{"keyOne": "join-key"},
		Indexed: []joinPropertyNested{
			{Mapped: map[string]string{"0ma": "0ma0"}},
			{Mapped: map[string]string{"1ma": "complex-1"}},
		},
		Nested: joinPropertyNested{Nested: "nested-value"},
	}
	if err := engine.SendEvent(context.Background(), combinedEvent); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), complexEvent); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("nested property join rows = %#v", rows)
	}
	row := rows[0]
	if row.Get("nested").Any() != "nested-value" {
		t.Fatalf("nested property projection = %#v", row.AsMap())
	}
	indexed, ok := row.Get("indexed").Any().(joinPropertyNested)
	if !ok || indexed.Mapped["0ma"] != "combined-0" {
		t.Fatalf("indexed property projection = %#v", row.AsMap())
	}
	left, ok := row.Get("left").Any().(Event)
	if !ok || left.Underlying().(joinPropertyComplex).Nested.Nested != "nested-value" {
		t.Fatalf("left event projection = %#v", row.AsMap())
	}
	right, ok := row.Get("right").Any().(Event)
	if !ok || right.Underlying().(joinPropertyCombined).Indexed[2].Mapped["2ma"] != "join-key" {
		t.Fatalf("right event projection = %#v", row.AsMap())
	}
}

func TestJoinOuterPropertyAccessPreservesNullSide(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPropertyComplex](env, "JoinOuterPropertyComplex"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPropertyCombined](env, "JoinOuterPropertyCombined"); err != nil {
		t.Fatal(err)
	}
	left := From[joinPropertyComplex](env, "JoinOuterPropertyComplex").Window(KeepAll())
	right := From[joinPropertyCombined](env, "JoinOuterPropertyCombined").Window(KeepAll())
	leftKey := MapAt[string](Field[joinPropertyComplex, map[string]string]("mapped"), Literal("keyOne"))
	rightThird := ArrayAt[joinPropertyNested](Field[joinPropertyCombined, []joinPropertyNested]("indexed"), Literal[int64](2))
	rightKey := MapAt[string](Property[map[string]string](rightThird, "mapped"), Literal("2ma"))
	plan, err := env.Build(Join(left, right, OnEqual(leftKey, rightKey)).LeftOuter().Select(
		SelectLeft("left", JoinEventValue[Event](0)),
		SelectRight("right", JoinEventValue[Event](1)),
		SelectRight("rightKey", rightKey),
	).Query(StatementName("join-outer-property-access")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	unmatched := joinPropertyComplex{Mapped: map[string]string{"keyOne": "missing"}}
	if err := engine.SendEvent(context.Background(), unmatched); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("outer property unmatched batch = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || !row.Get("left").IsPresent() || !row.Get("right").IsNull() || !row.Get("rightKey").IsNull() {
		t.Fatalf("outer property null side = %#v", row.AsMap())
	}
}
