package esper

import (
	"context"
	"testing"
)

type joinCoercionBean struct {
	ID           string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type joinCoercionRange struct {
	ID         string `esper:"id"`
	Key        string `esper:"key"`
	RangeStart int64  `esper:"rangeStartLong"`
	RangeEnd   int64  `esper:"rangeEndLong"`
}

func TestJoinRangeAndEqualityCoercionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinCoercionBean](env, "JoinCoercionBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinCoercionRange](env, "JoinCoercionRange"); err != nil {
		t.Fatal(err)
	}
	bean := From[joinCoercionBean](env, "JoinCoercionBean").Window(LengthWindow(10))
	ranges := From[joinCoercionRange](env, "JoinCoercionRange").Window(LengthWindow(10))
	plan, err := env.Build(Join(bean, ranges,
		OnGreaterOrEqual(
			Field[joinCoercionBean, int]("intPrimitive"),
			Field[joinCoercionRange, int64]("rangeStartLong"),
		),
		OnLessOrEqual(
			Field[joinCoercionBean, int]("intPrimitive"),
			Field[joinCoercionRange, int64]("rangeEndLong"),
		),
	).Select(
		SelectLeft("bean", Field[joinCoercionBean, string]("theString")),
		SelectRight("range", Field[joinCoercionRange, string]("id")),
	).Query(StatementName("join-coercion-range")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
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
	sendRange := func(id string, start, end int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), joinCoercionRange{ID: id, Key: "G", RangeStart: start, RangeEnd: end}); err != nil {
			t.Fatal(err)
		}
	}
	sendBean := func(id string, value int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), joinCoercionBean{ID: id, IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	sendRange("R1", 100, 200)
	sendBean("E1", 10)
	if len(rows) != 0 {
		t.Fatalf("range coercion low value rows = %#v", rows)
	}
	sendBean("E2", 100)
	sendRange("R2", 90, 100)
	sendRange("R3", 1, 99)
	if len(rows) != 3 {
		t.Fatalf("range coercion rows = %#v", rows)
	}
	want := [][2]any{{"E2", "R1"}, {"E2", "R2"}, {"E1", "R3"}}
	for index, expected := range want {
		if rows[index].Get("bean").Any() != expected[0] || rows[index].Get("range").Any() != expected[1] {
			t.Fatalf("range coercion row %d = %#v", index, rows[index].AsMap())
		}
	}
	sendRange("R4", 2000, 3000)
	sendBean("E1", 1000)
	if len(rows) != 3 {
		t.Fatalf("range coercion unmatched tail rows = %#v", rows)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	keyPlan, err := env.Build(Join(bean, ranges,
		OnEqual(
			Field[joinCoercionBean, string]("theString"),
			Field[joinCoercionRange, string]("key"),
		),
		OnGreaterOrEqual(
			Field[joinCoercionBean, int]("intPrimitive"),
			Field[joinCoercionRange, int64]("rangeStartLong"),
		),
		OnLessOrEqual(
			Field[joinCoercionBean, int]("intPrimitive"),
			Field[joinCoercionRange, int64]("rangeEndLong"),
		),
	).Select(
		SelectLeft("bean", Field[joinCoercionBean, string]("theString")),
		SelectRight("range", Field[joinCoercionRange, string]("id")),
	).Query(StatementName("join-coercion-key-range")))
	if err != nil {
		t.Fatal(err)
	}
	keyDeployment, err := engine.Deploy(context.Background(), keyPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer keyDeployment.Undeploy(context.Background())
	rows = nil
	if _, err := keyDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendRange("R1", 100, 200)
	sendBean("G", 10)
	if len(rows) != 0 {
		t.Fatalf("key range coercion low value rows = %#v", rows)
	}
	sendBean("G", 101)
	sendRange("R2", 90, 102)
	sendRange("R3", 1, 99)
	sendRange("R4", 2000, 3000)
	sendBean("G", 1000)
	if len(rows) != 3 {
		t.Fatalf("key range coercion rows = %#v", rows)
	}
	wantKey := [][2]any{{"G", "R1"}, {"G", "R2"}, {"G", "R3"}}
	for index, expected := range wantKey {
		if rows[index].Get("bean").Any() != expected[0] || rows[index].Get("range").Any() != expected[1] {
			t.Fatalf("key range coercion row %d = %#v", index, rows[index].AsMap())
		}
	}
}

type joinCoercionMarket struct {
	Volume int64 `esper:"volume"`
}

func TestJoinIntegralEqualityCoercionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinCoercionMarket](env, "JoinCoercionMarket"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinCoercionBean](env, "JoinCoercionIntegralBean"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Join(
		From[joinCoercionMarket](env, "JoinCoercionMarket").Window(LengthWindow(3)),
		From[joinCoercionBean](env, "JoinCoercionIntegralBean").Window(LengthWindow(3)),
		OnEqual(
			Field[joinCoercionMarket, int64]("volume"),
			Field[joinCoercionBean, int]("intPrimitive"),
		),
	).Select(
		SelectLeft("volume", Field[joinCoercionMarket, int64]("volume")),
	).Query(StatementName("join-coercion-integral")))
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
	if err := engine.Send(context.Background(), "JoinCoercionIntegralBean", joinCoercionBean{ID: "", IntPrimitive: 100}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "JoinCoercionMarket", joinCoercionMarket{Volume: 100}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Get("volume").Any() != int64(100) {
		t.Fatalf("integral coercion rows = %#v", rows)
	}
}
