package esper

import (
	"context"
	"testing"
	"time"
)

type probeInclusiveBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// Phase-1 shape: every-distinct (a.theString, 10 sec) @Inclusive + output last when terminated
// TestInclusivePatternContextEveryDistinctRoutesMatchEventsParity locks the
// @Inclusive pattern-start semantics verified against
// ContextInitTermPatternInclusion: the every-distinct 10-second start pattern
// routes its match event into the new partition (output last when terminated
// retains the last matching row), keys expire at first-seen + 10s so E1@10100
// starts a second partition, and E1@8000 is analyzed without starting a new
// partition (key still fresh).
func TestInclusivePatternContextEveryDistinctRoutesMatchEventsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[probeInclusiveBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[probeInclusiveBean](env, "SupportBean")
	theString := Field[probeInclusiveBean, string]("theString")
	start := PatternFrom(source, "a", Literal(true)).EveryDistinctFor(10*time.Second, Field[probeInclusiveBean, string]("theString"))
	end := TimerInterval(source, 10*time.Second)
	if _, err := CreateOverlappingPatternInitiatedTerminatedContextInclusive(env, "CtxPerId", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source.Filter(Equal[string](theString, ContextPatternField[string]("a", "theString"))),
		Alias("c0", theString),
		Alias("c1", Field[probeInclusiveBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithContext("CtxPerId"), WithOutput(OutputWhenTerminated(OutputLast()))))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	now := time.Unix(0, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []string
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row.Get("c0").Any().(string)+"/"+probeItoa(row.Get("c1").Any().(int)))
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), probeInclusiveBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	adv := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), now.Add(time.Duration(ms)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
	}
	rows = nil
	send("E1", 1)
	adv(1000)
	send("E2", 2)
	adv(8000)
	send("E1", 3)
	adv(9999)
	adv(10000)
	adv(10100)
	send("E2", 4)
	send("E1", 5)
	adv(11000)
	adv(16100)
	send("E2", 6)
	adv(20099)
	adv(20100)
	adv(26099)
	adv(26100)
	want := []string{"E1/3", "E2/4", "E1/5", "E2/6"}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("rows = %v, want %v", rows, want)
		}
	}
}

func probeItoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

type probeInclusiveS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 string  `esper:"p01"`
}

type probeInclusiveS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 string  `esper:"p11"`
}

// Phase-2 shape: every a=S0 -> b=S1 @Inclusive + select * from pattern [...]
// TestInclusivePatternContextMultiEventRoutesStartEventsParity locks the
// multi-event @Inclusive start verified against the second half of
// ContextInitTermPatternInclusion: `every a=SupportBean_S0 -> b=SupportBean_S1`
// routes S0 then S1 (tag order) into the new partition, completing the inner
// statement pattern exactly once.
func TestInclusivePatternContextMultiEventRoutesStartEventsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[probeInclusiveS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[probeInclusiveS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	s0Base := From[probeInclusiveS0](env, "SupportBean_S0")
	s1Base := From[probeInclusiveS1](env, "SupportBean_S1")
	start := PatternFrom(s0Base, "a", Literal(true)).Then(PatternFrom(s1Base, "b", Literal(true))).Every()
	end := TimerInterval(From[probeInclusiveS0](env, "SupportBean_S0"), 10*time.Second)
	if _, err := CreateOverlappingPatternInitiatedTerminatedContextInclusive(env, "CtxPerId", start, end); err != nil {
		t.Fatal(err)
	}
	statementPattern := PatternFrom(s0Base, "a", Literal(true)).Then(PatternFrom(s1Base, "b", Literal(true))).Every()
	plan, err := env.Build(statementPattern.Select(
		Alias("a.id", TagField[int]("a", "id")),
		Alias("a.p00", TagField[*string]("a", "p00")),
		Alias("b.id", TagField[int]("b", "id")),
		Alias("b.p10", TagField[*string]("b", "p10")),
	).Query(StatementName("s0"), WithContext("CtxPerId")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	now := time.Unix(0, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p00 := "S0_1"
	if err := engine.SendEvent(context.Background(), probeInclusiveS0{ID: 10, P00: &p00}); err != nil {
		t.Fatal(err)
	}
	p10 := "S1_1"
	if err := engine.SendEvent(context.Background(), probeInclusiveS1{ID: 20, P10: &p10}); err != nil {
		t.Fatal(err)
	}
	if batches != 1 {
		t.Fatalf("batches = %d, want 1", batches)
	}
}
