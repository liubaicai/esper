package esper

import (
	"context"
	"testing"
	"time"
)

type keySegJoinRemoveStreamBean struct {
	PageName  string `esper:"pageName"`
	SessionID string `esper:"sessionId"`
}

// TestKeyContextJoinRemoveStreamParity locks the rstream outer-join
// removal of a keyed context verified against
// ContextKeySegmentedJoinRemoveStream: `partition by sessionId` joining
// three per-partition time(30) windows (Start, Middle, End) with full
// outer semantics and a where clause admitting only incomplete sessions.
// Inserts produce no remove-stream rows (intermediate null-padded
// replacements are not removals in Esper's outer join), and exactly one
// removal fires when session 3's Start expires: {Start, 3, null, End}.
func TestKeyContextJoinRemoveStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegJoinRemoveStreamBean](env, "SupportWebEvent"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegJoinRemoveStreamBean](env, "SupportWebEvent")
	sessionID := Field[keySegJoinRemoveStreamBean, string]("sessionId")
	pageName := Field[keySegJoinRemoveStreamBean, string]("pageName")
	if _, err := CreateKeyContext(env, "SegmentedBySession", sessionID); err != nil {
		t.Fatal(err)
	}
	start := source.Filter(Equal[string](pageName, Literal("Start"))).Window(TimeWindow(30 * time.Second))
	middle := source.Filter(Equal[string](pageName, Literal("Middle"))).Window(TimeWindow(30 * time.Second))
	end := source.Filter(Equal[string](pageName, Literal("End"))).Window(TimeWindow(30 * time.Second))
	chain := JoinChain(JoinSource(start)).
		FullOuterJoin(JoinSource(middle), OnSourcesEqual(
			0, Field[keySegJoinRemoveStreamBean, string]("sessionId"),
			1, Field[keySegJoinRemoveStreamBean, string]("sessionId"),
		)).
		FullOuterJoin(JoinSource(end), OnSourcesEqual(
			0, Field[keySegJoinRemoveStreamBean, string]("sessionId"),
			2, Field[keySegJoinRemoveStreamBean, string]("sessionId"),
		))
	query := chain.Select(
		SelectFrom(0, "pageNameA", JoinField[string](0, "pageName")),
		SelectFrom(0, "sessionIdA", JoinField[string](0, "sessionId")),
		SelectFrom(1, "pageNameB", JoinField[string](1, "pageName")),
		SelectFrom(2, "pageNameC", JoinField[string](2, "pageName")),
	).Where(And(
		Not(IsNull[string](JoinField[string](0, "pageName"))),
		Or(
			IsNull[string](JoinField[string](1, "pageName")),
			IsNull[string](JoinField[string](2, "pageName")),
		),
	)).Query(StatementName("s0"), WithContext("SegmentedBySession"), WithRemoveStreamOnly())
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
	type row struct {
		a, b, c, session string
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				b := rowValue.Get("pageNameB").Any()
				c := rowValue.Get("pageNameC").Any()
				rows = append(rows, row{
					a:       rowValue.Get("pageNameA").Any().(string),
					b:       stringOrEmpty(b),
					c:       stringOrEmpty(c),
					session: rowValue.Get("sessionIdA").Any().(string),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(page, session string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinRemoveStreamBean{PageName: page, SessionID: session}); err != nil {
			t.Fatal(err)
		}
	}
	complete := func(id string) {
		send("Start", id)
		send("Middle", id)
		send("End", id)
	}
	complete("0")
	engine.AdvanceTime(context.Background(), time.UnixMilli(20000).UTC())
	complete("1")
	engine.AdvanceTime(context.Background(), time.UnixMilli(40000).UTC())
	complete("2")
	engine.AdvanceTime(context.Background(), time.UnixMilli(60000).UTC())
	send("Start", "3")
	send("End", "3")
	if len(rows) != 0 {
		t.Fatalf("insert-phase rows = %#v, want none", rows)
	}
	engine.AdvanceTime(context.Background(), time.UnixMilli(80000).UTC())
	if len(rows) != 0 {
		t.Fatalf("pre-expiry rows = %#v, want none", rows)
	}
	engine.AdvanceTime(context.Background(), time.UnixMilli(100000).UTC())
	want := []row{{a: "Start", b: "", c: "End", session: "3"}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %#v, want %#v", rows, want)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Fatalf("rows[%d] = %#v, want %#v", index, rows[index], want[index])
		}
	}
}

func stringOrEmpty(value any) string {
	if value == nil {
		return ""
	}
	return value.(string)
}
