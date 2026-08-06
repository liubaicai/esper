package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type joinUnidirectionalTripleEvent struct {
	ID  int    `esper:"id"`
	Key string `esper:"key"`
}

func TestUnidirectionalRightDriverJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniRightOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniRightPayment"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Join(
		From[joinOrder](env, "UniRightOrder").Window(KeepAll()),
		From[joinPayment](env, "UniRightPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Unidirectional(JoinRight).Select(
		SelectLeft("order", Field[joinOrder, string]("orderID")),
		SelectRight("amount", Field[joinPayment, float64]("amount")),
	).Query(StatementName("unidirectional-right-driver-parity")))
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
	if err := engine.Send(context.Background(), "UniRightOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("right-driver passive event emitted = %#v", batches)
	}
	if err := engine.Send(context.Background(), "UniRightPayment", joinPayment{OrderID: "O1", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("right-driver result = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("order").Any() != "O1" || row.Get("amount").Any() != float64(7) {
		t.Fatalf("right-driver row = %#v", batches[0].New)
	}
	if err := engine.Send(context.Background(), "UniRightPayment", joinPayment{OrderID: "O1", Amount: 9}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("right-driver second trigger = %#v", batches)
	}
	if err := engine.Send(context.Background(), "UniRightOrder", joinOrder{OrderID: "O1"}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("right-driver second passive event emitted = %#v", batches)
	}
}

func TestUnidirectionalThreeStreamInnerOrderVariantsMatchEsper(t *testing.T) {
	variants := []struct {
		name  string
		order []int
	}{
		{name: "driver-first", order: []int{0, 1, 2}},
		{name: "driver-middle", order: []int{1, 0, 2}},
		{name: "driver-middle-reverse", order: []int{2, 0, 1}},
		{name: "driver-last", order: []int{1, 2, 0}},
	}
	for _, current := range variants {
		current := current
		t.Run(current.name, func(t *testing.T) {
			env := NewEnvironment()
			streams := make([]Stream[joinUnidirectionalTripleEvent], 3)
			inputs := make([]JoinInput, 3)
			position := make(map[int]int, 3)
			for logical := 0; logical < 3; logical++ {
				name := fmt.Sprintf("UniTripleInnerS%d", logical)
				if _, err := RegisterStruct[joinUnidirectionalTripleEvent](env, name); err != nil {
					t.Fatal(err)
				}
				streams[logical] = From[joinUnidirectionalTripleEvent](env, name)
			}
			for index, logical := range current.order {
				position[logical] = index
				input := JoinSource(streams[logical])
				if logical == 0 {
					input = input.Unidirectional()
				} else {
					input = input.Window(KeepAll())
				}
				inputs[index] = input
			}
			field := Field[joinUnidirectionalTripleEvent, string]("key")
			plan, err := env.Build(JoinMany(inputs...).On(
				OnSourcesEqual(position[0], field, position[1], field),
				OnSourcesEqual(position[1], field, position[2], field),
			).Select(
				SelectFrom(position[0], "s0", JoinField[string](position[0], "key")),
				SelectFrom(position[1], "s1", JoinField[string](position[1], "key")),
				SelectFrom(position[2], "s2", JoinField[string](position[2], "key")),
			).Query(StatementName("unidirectional-three-inner-" + current.name)))
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
			for _, logical := range []int{1, 2} {
				if err := engine.Send(context.Background(), fmt.Sprintf("UniTripleInnerS%d", logical), joinUnidirectionalTripleEvent{ID: logical, Key: "A"}); err != nil {
					t.Fatal(err)
				}
			}
			if len(batches) != 0 {
				t.Fatalf("inner passive events emitted = %#v", batches)
			}
			if err := engine.Send(context.Background(), "UniTripleInnerS0", joinUnidirectionalTripleEvent{ID: 0, Key: "A"}); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 1 || len(batches[0].New) != 1 {
				t.Fatalf("inner variant %s batches = %#v", current.name, batches)
			}
			row, ok := batches[0].New[0].Row()
			if !ok || row.Get("s0").Any() != "A" || row.Get("s1").Any() != "A" || row.Get("s2").Any() != "A" {
				t.Fatalf("inner variant %s row = %#v", current.name, batches[0].New)
			}
		})
	}
}

func TestUnidirectionalThreeStreamOuterVariantsMatchEsper(t *testing.T) {
	variants := []struct {
		name string
		kind JoinKind
	}{
		{name: "full-outer-chain", kind: JoinFullOuter},
		{name: "left-outer-chain", kind: JoinLeftOuter},
	}
	for _, current := range variants {
		current := current
		t.Run(current.name, func(t *testing.T) {
			env := NewEnvironment()
			streams := make([]Stream[joinUnidirectionalTripleEvent], 3)
			for logical := 0; logical < 3; logical++ {
				name := fmt.Sprintf("UniTripleOuterS%d", logical)
				if _, err := RegisterStruct[joinUnidirectionalTripleEvent](env, name); err != nil {
					t.Fatal(err)
				}
				streams[logical] = From[joinUnidirectionalTripleEvent](env, name)
			}
			field := Field[joinUnidirectionalTripleEvent, string]("key")
			chain := JoinChain(JoinSource(streams[0]).Unidirectional())
			chain = chain.FullOuterJoin(JoinSource(streams[1].Window(KeepAll())), OnSourcesEqual(0, field, 1, field))
			switch current.kind {
			case JoinFullOuter:
				chain = chain.FullOuterJoin(JoinSource(streams[2].Window(KeepAll())), OnSourcesEqual(1, field, 2, field))
			case JoinLeftOuter:
				chain = chain.LeftOuterJoin(JoinSource(streams[2].Window(KeepAll())), OnSourcesEqual(1, field, 2, field))
			default:
				t.Fatalf("unexpected outer kind %d", current.kind)
			}
			plan, err := env.Build(chain.Select(
				SelectFrom(0, "s0", JoinField[string](0, "key")),
				SelectFrom(1, "s1", JoinField[string](1, "key")),
				SelectFrom(2, "s2", JoinField[string](2, "key")),
			).Query(StatementName("unidirectional-three-outer-" + current.name)))
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
			if err := engine.Send(context.Background(), "UniTripleOuterS1", joinUnidirectionalTripleEvent{ID: 1, Key: "A"}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), "UniTripleOuterS2", joinUnidirectionalTripleEvent{ID: 2, Key: "A"}); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 0 {
				t.Fatalf("outer passive events emitted = %#v", batches)
			}
			if err := engine.Send(context.Background(), "UniTripleOuterS0", joinUnidirectionalTripleEvent{ID: 0, Key: "A"}); err != nil {
				t.Fatal(err)
			}
			assertUnidirectionalOuterRow(t, batches, 0, "A", "A", "A")
			if err := engine.Send(context.Background(), "UniTripleOuterS0", joinUnidirectionalTripleEvent{ID: 0, Key: "B"}); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 2 || len(batches[1].New) != 1 {
				t.Fatalf("outer unmatched trigger = %#v", batches)
			}
			row, ok := batches[1].New[0].Row()
			if !ok || row.Get("s0").Any() != "B" || !row.Get("s1").IsNull() || !row.Get("s2").IsNull() {
				t.Fatalf("outer unmatched row = %#v", batches[1].New)
			}
		})
	}
}

func assertUnidirectionalOuterRow(t *testing.T, batches []ResultBatch, index int, s0, s1, s2 string) {
	t.Helper()
	if len(batches) <= index || len(batches[index].New) != 1 {
		t.Fatalf("unidirectional outer batch %d = %#v", index, batches)
	}
	row, ok := batches[index].New[0].Row()
	if !ok || row.Get("s0").Any() != s0 || row.Get("s1").Any() != s1 || row.Get("s2").Any() != s2 {
		t.Fatalf("unidirectional outer row %d = %#v", index, batches[index].New)
	}
}

func TestUnidirectionalJoinValidationMatchesEsperBoundary(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "UniBoundaryOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "UniBoundaryPayment"); err != nil {
		t.Fatal(err)
	}
	windowed := Join(
		From[joinOrder](env, "UniBoundaryOrder").Window(KeepAll()),
		From[joinPayment](env, "UniBoundaryPayment"),
		OnEqual(Field[joinOrder, string]("orderID"), Field[joinPayment, string]("orderID")),
	).Unidirectional(JoinLeft).Select(
		SelectLeft("order", Field[joinOrder, string]("orderID")),
	).Query(StatementName("unidirectional-boundary-window"))
	if _, err := env.Build(windowed); err == nil || !strings.Contains(err.Error(), "cannot declare a window view") {
		t.Fatalf("windowed unidirectional boundary error = %v", err)
	}
}
