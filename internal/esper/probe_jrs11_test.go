package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type probeWebEventB struct {
	PageName  string `esper:"pageName"`
	SessionID string `esper:"sessionId"`
}

func TestProbeJoinIRStreamDebug(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[probeWebEventB](env, "SupportWebEvent"); err != nil {
		t.Fatal(err)
	}
	source := From[probeWebEventB](env, "SupportWebEvent")
	pageName := Field[probeWebEventB, string]("pageName")
	start := source.Filter(Equal[string](pageName, Literal("Start"))).Window(TimeWindow(30 * time.Second))
	end := source.Filter(Equal[string](pageName, Literal("End"))).Window(TimeWindow(30 * time.Second))
	chain := JoinChain(JoinSource(start)).FullOuterJoin(JoinSource(end), OnSourcesEqual(
		0, Field[probeWebEventB, string]("sessionId"),
		1, Field[probeWebEventB, string]("sessionId"),
	))
	query := chain.Select(
		SelectFrom(0, "pageNameA", JoinField[string](0, "pageName")),
		SelectFrom(1, "pageNameC", JoinField[string](1, "pageName")),
	).Query(StatementName("s0"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []string
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				var vals []any
				for _, f := range row.Schema().Fields() {
					vals = append(vals, row.Get(f.Name).Any())
				}
				rows = append(rows, "NEW:"+fmt.Sprint(vals))
			}
		}
		for _, result := range batch.Old {
			if row, ok := result.Row(); ok {
				var vals []any
				for _, f := range row.Schema().Fields() {
					vals = append(vals, row.Get(f.Name).Any())
				}
				rows = append(rows, "OLD:"+fmt.Sprint(vals))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(page, session string) {
		if err := engine.SendEvent(context.Background(), probeWebEventB{PageName: page, SessionID: session}); err != nil {
			t.Fatal(err)
		}
	}
	send("Start", "3")
	fmt.Println("after Start3:", rows)
	send("End", "3")
	fmt.Println("after End3:", rows)
}
