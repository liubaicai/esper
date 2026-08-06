package esper

import (
	"context"
	"testing"
)

type joinInPassiveEvent struct {
	Value string `esper:"value"`
}

type joinInDriverEvent struct {
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

type joinInS0Event struct {
	P00 string `esper:"p00"`
}

type joinInS1Event struct {
	P10 string `esper:"p10"`
}

type joinInS2Event struct {
	P20 string `esper:"p20"`
}

func TestJoinInKeywordPredicateMatchesEsper(t *testing.T) {
	t.Run("single-index-driver-on-right", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinInPassiveEvent](env, "JoinInPassiveRight"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinInDriverEvent](env, "JoinInDriverRight"); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(JoinMany(
			JoinSource(From[joinInPassiveEvent](env, "JoinInPassiveRight").Window(KeepAll())),
			JoinSource(From[joinInDriverEvent](env, "JoinInDriverRight")).Unidirectional(),
		).Select(
			SelectFrom(0, "value", JoinField[string](0, "value")),
		).Where(
			In[string](
				JoinField[string](0, "value"),
				JoinField[string](1, "p00"),
				JoinField[string](1, "p01"),
			),
		).Query(StatementName("join-in-single-index-right")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		var values []string
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if ok {
					values = append(values, row.Get("value").Any().(string))
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		send := func(source string, event any) {
			t.Helper()
			if err := engine.Send(context.Background(), source, event); err != nil {
				t.Fatal(err)
			}
		}
		send("JoinInPassiveRight", joinInPassiveEvent{Value: "A"})
		send("JoinInPassiveRight", joinInPassiveEvent{Value: "B"})
		send("JoinInDriverRight", joinInDriverEvent{P00: "X", P01: "A"})
		if len(values) != 1 || values[0] != "A" {
			t.Fatalf("right-driver in rows = %#v", values)
		}
		send("JoinInDriverRight", joinInDriverEvent{P00: "B", P01: "Y"})
		if len(values) != 2 || values[1] != "B" {
			t.Fatalf("right-driver second in rows = %#v", values)
		}
	})

	t.Run("single-index-driver-on-left", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinInPassiveEvent](env, "JoinInPassiveLeft"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinInDriverEvent](env, "JoinInDriverLeft"); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(JoinMany(
			JoinSource(From[joinInDriverEvent](env, "JoinInDriverLeft")).Unidirectional(),
			JoinSource(From[joinInPassiveEvent](env, "JoinInPassiveLeft").Window(KeepAll())),
		).Select(
			SelectFrom(1, "value", JoinField[string](1, "value")),
		).Where(
			In[string](
				JoinField[string](1, "value"),
				JoinField[string](0, "p00"),
				JoinField[string](0, "p01"),
			),
		).Query(StatementName("join-in-single-index-left")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		defer deployment.Undeploy(context.Background())
		var values []string
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if ok {
					values = append(values, row.Get("value").Any().(string))
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		send := func(source string, event any) {
			t.Helper()
			if err := engine.Send(context.Background(), source, event); err != nil {
				t.Fatal(err)
			}
		}
		send("JoinInPassiveLeft", joinInPassiveEvent{Value: "A"})
		send("JoinInPassiveLeft", joinInPassiveEvent{Value: "B"})
		send("JoinInDriverLeft", joinInDriverEvent{P00: "X", P01: "B"})
		if len(values) != 1 || values[0] != "B" {
			t.Fatalf("left-driver in rows = %#v", values)
		}
		send("JoinInDriverLeft", joinInDriverEvent{P00: "A", P01: "Y"})
		if len(values) != 2 || values[1] != "A" {
			t.Fatalf("left-driver second in rows = %#v", values)
		}
	})

	t.Run("three-stream", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[joinInS0Event](env, "JoinInS0"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinInS1Event](env, "JoinInS1"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[joinInS2Event](env, "JoinInS2"); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(JoinMany(
			JoinSource(From[joinInS0Event](env, "JoinInS0").Window(KeepAll())),
			JoinSource(From[joinInS1Event](env, "JoinInS1").Window(KeepAll())),
			JoinSource(From[joinInS2Event](env, "JoinInS2").Window(KeepAll())),
		).Select(
			SelectFrom(0, "p00", JoinField[string](0, "p00")),
			SelectFrom(1, "p10", JoinField[string](1, "p10")),
			SelectFrom(2, "p20", JoinField[string](2, "p20")),
		).Where(
			In[string](
				JoinField[string](0, "p00"),
				JoinField[string](1, "p10"),
				JoinField[string](2, "p20"),
			),
		).Query(StatementName("join-in-three-stream")))
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
		send := func(source string, event any) {
			t.Helper()
			if err := engine.Send(context.Background(), source, event); err != nil {
				t.Fatal(err)
			}
		}
		send("JoinInS0", joinInS0Event{P00: "A"})
		send("JoinInS1", joinInS1Event{P10: "B"})
		if len(rows) != 0 {
			t.Fatalf("three-stream incomplete in rows = %#v", rows)
		}
		send("JoinInS2", joinInS2Event{P20: "A"})
		if len(rows) != 1 || rows[0].Get("p00").Any() != "A" || rows[0].Get("p10").Any() != "B" || rows[0].Get("p20").Any() != "A" {
			t.Fatalf("three-stream in rows = %#v", rows)
		}
		send("JoinInS2", joinInS2Event{P20: "C"})
		if len(rows) != 1 {
			t.Fatalf("three-stream false in rows = %#v", rows)
		}
		send("JoinInS0", joinInS0Event{P00: "B"})
		if len(rows) != 3 || rows[1].Get("p00").Any() != "B" || rows[2].Get("p00").Any() != "B" {
			t.Fatalf("three-stream second in rows = %#v", rows)
		}
		if rows[1].Get("p20").Any() != "A" || rows[2].Get("p20").Any() != "C" {
			t.Fatalf("three-stream second in candidates = %#v", rows)
		}
	})
}
