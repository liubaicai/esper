package esper

import (
	"context"
	"testing"
)

type joinDerivedViewSupportBean struct {
	ID            string `esper:"id"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

func TestJoinDerivedLinearRegressionBatchSourceMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinDerivedViewSupportBean](env, "JoinDerivedViewSupportBean"); err != nil {
		t.Fatal(err)
	}
	x := Field[joinDerivedViewSupportBean, int]("intPrimitive")
	y := Field[joinDerivedViewSupportBean, int64]("longPrimitive")
	leftRegression := Linest[int, int64](x, y)
	rightRegression := Linest[int, int64](x, y)

	left := From[joinDerivedViewSupportBean](env, "JoinDerivedViewSupportBean").Window(LengthBatch(3)).Aggregate(
		Alias("slope", leftRegression.Slope()),
	)
	right := From[joinDerivedViewSupportBean](env, "JoinDerivedViewSupportBean").Window(LengthBatch(2)).Aggregate(
		Alias("slope", rightRegression.Slope()),
	)
	plan, err := env.Build(JoinMany(
		left.AsJoinSource(),
		right.AsJoinSource(),
	).Select(
		SelectFrom(0, "s1", Signum(JoinField[float64](0, "slope"))),
		SelectFrom(1, "s2", Signum(JoinField[float64](1, "slope"))),
	).Query(StatementName("join-derived-linear-regression")))
	if err != nil {
		t.Fatal(err)
	}

	engine := env.NewEngine()
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

	send := func(id string, integer int, long int64) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), joinDerivedViewSupportBean{ID: id, IntPrimitive: integer, LongPrimitive: long}); err != nil {
			t.Fatal(err)
		}
	}
	send("E3", 1, 100)
	send("E4", 2, 200)
	if len(rows) != 0 {
		t.Fatalf("derived join emitted before both batches were ready: %#v", rows)
	}
	send("E5", 3, 300)
	if len(rows) != 1 {
		t.Fatalf("derived join rows after the first left batch = %#v, want one row", rows)
	}
	send("E6", 4, 400)
	if len(rows) != 2 {
		t.Fatalf("derived join rows after the second right batch = %#v, want two rows", rows)
	}
	for index, row := range rows {
		if row.Get("s1").Any() != float64(1) || row.Get("s2").Any() != float64(1) {
			t.Fatalf("derived join signum projection at %d = %#v", index, row.AsMap())
		}
	}
}
