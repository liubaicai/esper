package esper

import (
	"context"
	"testing"
)

// TestKeyContextPatternSceneTwoCrossStreamParity locks the two-stream
// segmented-context pattern verified against ContextKeySegmentedPatternSceneTwo:
// `partition by theString from SupportBean, p00 from SupportBean_S0` with
// `every a=SupportBean -> b=SupportBean_S0(id=a.intPrimitive)`. Partner
// events route to the partition whose p00 equals the SB key, so each
// partition's a-wait completes only with its own matching S0 event; a
// consumed a never re-fires.
func TestKeyContextPatternSceneTwoCrossStreamParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegJoinBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[keySegJoinS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	theString := Field[keySegJoinBean, string]("theString")
	p00 := Field[keySegJoinS0, string]("p00")
	if _, err := CreateKeyContextByStreams(env, "SegmentedByString",
		KeyContextStream{Type: "SupportBean", Keys: []Expr{theString}},
		KeyContextStream{Type: "SupportBean_S0", Keys: []Expr{p00}},
	); err != nil {
		t.Fatal(err)
	}
	pattern := PatternFrom(From[keySegJoinBean](env, "SupportBean"), "a", Literal(true)).
		Every().
		Then(PatternFrom(From[keySegJoinS0](env, "SupportBean_S0"), "b",
			Equal[int](
				TagField[int]("b", "id"),
				TagField[int]("a", "intPrimitive"),
			)))
	query := pattern.Select(
		Alias("c0", TagField[string]("a", "theString")),
		Alias("c1", TagField[int]("a", "intPrimitive")),
		Alias("c2", TagField[int]("b", "id")),
		Alias("c3", TagField[string]("b", "p00")),
	).Query(StatementName("S1"), WithContext("SegmentedByString"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	type row struct {
		c0, c3 string
		c1, c2 int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					c0: rowValue.Get("c0").Any().(string),
					c1: rowValue.Get("c1").Any().(int),
					c2: rowValue.Get("c2").Any().(int),
					c3: rowValue.Get("c3").Any().(string),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendSB := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(p00 string, id int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), keySegJoinS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendSB("G1", 10)
	sendSB("G2", 20)
	sendS0("G1", 0)
	sendS0("G2", 10)
	if len(rows) != 0 {
		t.Fatalf("before any match rows = %#v, want none", rows)
	}
	sendS0("G2", 20)
	if len(rows) != 1 || rows[0] != (row{c0: "G2", c1: 20, c2: 20, c3: "G2"}) {
		t.Fatalf("after G2 match rows = %#v, want [{G2 20 20 G2}]", rows)
	}
	sendS0("G2", 20)
	sendS0("G1", 0)
	if len(rows) != 1 {
		t.Fatalf("after consumed-a rows = %#v, want unchanged", rows)
	}
	sendS0("G1", 10)
	if len(rows) != 2 || rows[1] != (row{c0: "G1", c1: 10, c2: 10, c3: "G1"}) {
		t.Fatalf("after G1 match rows = %#v, want second {G1 10 10 G1}", rows)
	}
}
