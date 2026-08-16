package esper

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type joinPatternS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type joinPatternS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

type joinPatternS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

type joinPatternS3 struct {
	ID  int    `esper:"id"`
	P30 string `esper:"p30"`
}

func TestPatternJoinRejectsUnmaterializablePattern(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); err != nil {
		t.Fatal(err)
	}
	s0 := From[joinPatternS0](env, "PatternS0")
	query := JoinMany(
		JoinPatternSource(PatternFrom(s0, "", nil)),
		JoinSource(s0),
	).Select(
		SelectFrom(1, "id", Field[joinPatternS0, int]("id")),
	).Query(StatementName("invalid-pattern-join"))
	if _, err := env.Build(query); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("empty pattern join build error = %v, want %v", err, ErrorInvalidRule)
	}
}

func TestPatternUnidirectionalTimerJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); err != nil {
		t.Fatal(err)
	}
	s0 := From[joinPatternS0](env, "PatternS0")
	query := JoinMany(
		JoinPatternSource(TimerInterval(s0, time.Second).Every()).Unidirectional(),
		JoinSource(s0).Window(KeepAll()),
	).LeftOuter().Aggregate(
		Alias("sum", Sum[int](JoinField[int](1, "id"))),
		Alias("count", CountAll()),
	).Query(StatementName("pattern-unidirectional-timer-join"))
	plan, err := env.Build(query)
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
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 0).Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("timer-only pattern first trigger = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("sum").IsPresent() || row.Get("count").Any() != int64(1) {
		t.Fatalf("timer-only pattern unmatched row = %#v", row.AsMap())
	}
	if err := engine.Send(context.Background(), "PatternS0", joinPatternS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("passive pattern-join event emitted = %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 0).Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("timer-only pattern second trigger = %#v", batches)
	}
	newRow, newOK := batches[1].New[0].Row()
	sum, sumOK := numericValue(newRow.Get("sum"))
	if !newOK || !sumOK || sum != 10 || newRow.Get("count").Any() != int64(1) {
		t.Fatalf("timer-only pattern matched row = %#v", newRow.AsMap())
	}
}

func TestPatternUnidirectionalRejectsPatternResultWindow(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); err != nil {
		t.Fatal(err)
	}
	s0 := From[joinPatternS0](env, "PatternS0")
	pattern := PatternFrom(s0, "a", Literal[bool](true)).Every()
	query := JoinMany(
		JoinPatternSource(pattern).Window(LengthWindow(2)).Unidirectional(),
		JoinSource(s0).Window(KeepAll()),
	).Select(
		SelectFrom(0, "id", JoinPatternField[int](0, "a", "id")),
	).Query(StatementName("invalid-pattern-unidirectional-window"))
	if _, err := env.Build(query); err == nil || !strings.Contains(err.Error(), "pattern result window") {
		t.Fatalf("pattern unidirectional window error = %v", err)
	}
}

func TestPatternUnidirectionalTimerJoinOutputRateMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); err != nil {
		t.Fatal(err)
	}
	base := From[joinPatternS0](env, "PatternS0")
	a := base.Filter(Equal[int](Field[joinPatternS0, int]("id"), Literal(1))).Window(Unique(Field[joinPatternS0, string]("p00")))
	b := base.Filter(Equal[int](Field[joinPatternS0, int]("id"), Literal(2))).Window(Unique(Field[joinPatternS0, string]("p00")))
	query := JoinMany(
		JoinPatternSource(TimerInterval(base, time.Minute).Every()).Unidirectional(),
		JoinSource(a),
		JoinSource(b),
	).On(OnSourcesEqual(
		1, Field[joinPatternS0, string]("p00"),
		2, Field[joinPatternS0, string]("p00"),
	)).Aggregate(
		Alias("num", CountAll()),
	).Query(
		StatementName("pattern-unidirectional-timer-output-rate"),
		WithOutput(OutputEveryTime(2*time.Minute)),
	)
	plan, err := env.Build(query)
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
	for _, event := range []joinPatternS0{
		{ID: 1, P00: "A"},
		{ID: 1, P00: "B"},
		{ID: 2, P00: "A"},
		{ID: 2, P00: "B"},
	} {
		if err := engine.Send(context.Background(), "PatternS0", event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 0).Add(70*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("output-rate timer flushed too early = %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(0, 0).Add(140*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("output-rate timer batch = %#v", batches)
	}
	for _, result := range batches[0].New {
		row, ok := result.Row()
		if !ok || row.Get("num").Any() != int64(2) {
			t.Fatalf("output-rate timer row = %#v", result)
		}
	}
}

func TestPatternFilterJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	for _, register := range []func(*Environment) error{
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); return err },
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS1](env, "PatternS1"); return err },
	} {
		if err := register(env); err != nil {
			t.Fatal(err)
		}
	}
	s0 := From[joinPatternS0](env, "PatternS0")
	s1 := From[joinPatternS1](env, "PatternS1")
	pattern := PatternFrom(s0, "a", Equal[string](Field[joinPatternS0, string]("p00"), Literal("a"))).
		Or(PatternFrom(s0, "b", Equal[string](Field[joinPatternS0, string]("p00"), Literal("b")))).
		Every()

	query := JoinMany(
		JoinPatternSource(pattern).Window(LengthWindow(5)),
		JoinSource(s1).Window(LengthWindow(5)),
	).On(AnyJoin(
		OnSourcesEqual(0, JoinPatternField[int](0, "a", "id"), 1, Field[joinPatternS1, int]("id")),
		OnSourcesEqual(0, JoinPatternField[int](0, "b", "id"), 1, Field[joinPatternS1, int]("id")),
	)).Select(
		SelectFrom(0, "aID", JoinPatternField[int](0, "a", "id")),
		SelectFrom(0, "aP00", JoinPatternField[string](0, "a", "p00")),
		SelectFrom(0, "bID", JoinPatternField[int](0, "b", "id")),
		SelectFrom(0, "bP00", JoinPatternField[string](0, "b", "p00")),
		SelectFrom(1, "s1ID", Field[joinPatternS1, int]("id")),
		SelectFrom(1, "s1P10", Field[joinPatternS1, string]("p10")),
	).Query(StatementName("pattern-filter-join"), WithOldStream())
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send0 := func(event joinPatternS0) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send1 := func(event joinPatternS1) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	send1(joinPatternS1{ID: 1, P10: "s1A"})
	send0(joinPatternS0{ID: 2, P00: "a"})
	if len(batches) != 0 {
		t.Fatalf("unmatched pattern event emitted: %#v", batches)
	}
	send0(joinPatternS0{ID: 1, P00: "b"})
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("pattern filter join first match = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("aID").IsPresent() || row.Get("bID").Any() != 1 || row.Get("bP00").Any() != "b" || row.Get("s1P10").Any() != "s1A" {
		t.Fatalf("pattern filter join b row = %#v", row.AsMap())
	}

	send1(joinPatternS1{ID: 2, P10: "s2A"})
	if len(batches) != 2 || len(batches[1].New) != 1 {
		t.Fatalf("pattern filter join second match = %#v", batches)
	}
	row, ok = batches[1].New[0].Row()
	if !ok || row.Get("aID").Any() != 2 || row.Get("aP00").Any() != "a" || row.Get("bID").IsPresent() {
		t.Fatalf("pattern filter join a row = %#v", row.AsMap())
	}

	// Five additional pattern matches push the first `a(2)` result out of
	// the pattern-side length window. Its existing join tuple is removed from
	// the result set, matching Esper's old-stream behavior.
	for _, event := range []joinPatternS0{
		{ID: 20, P00: "a"},
		{ID: 20, P00: "b"},
		{ID: 40, P00: "a"},
		{ID: 50, P00: "b"},
	} {
		send0(event)
	}
	if len(batches) != 3 || len(batches[2].Old) != 1 {
		t.Fatalf("pattern filter join eviction = %#v", batches)
	}
	old, ok := batches[2].Old[0].Row()
	if !ok || old.Get("aID").Any() != 2 || old.Get("s1ID").Any() != 2 {
		t.Fatalf("pattern filter join evicted row = %#v", old.AsMap())
	}
}

func TestTwoPatternJoinProjectsTagEventsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	registrations := []func(*Environment) error{
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); return err },
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS1](env, "PatternS1"); return err },
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS2](env, "PatternS2"); return err },
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS3](env, "PatternS3"); return err },
	}
	for _, register := range registrations {
		if err := register(env); err != nil {
			t.Fatal(err)
		}
	}
	s0 := From[joinPatternS0](env, "PatternS0")
	s1 := From[joinPatternS1](env, "PatternS1")
	s2 := From[joinPatternS2](env, "PatternS2")
	s3 := From[joinPatternS3](env, "PatternS3")
	p0 := PatternFrom(s0, "es0", Literal[bool](true)).
		And(PatternFrom(s1, "es1", Literal[bool](true))).Every()
	p1 := PatternFrom(s2, "es2", Literal[bool](true)).
		And(PatternFrom(s3, "es3", Literal[bool](true))).Every()

	query := JoinMany(
		JoinPatternSource(p0).Window(LengthWindow(3)),
		JoinPatternSource(p1).Window(LengthWindow(3)),
	).On(OnSourcesEqual(
		0, JoinPatternField[int](0, "es0", "id"),
		1, JoinPatternField[int](1, "es2", "id"),
	)).Select(
		SelectFrom(0, "s0es0Id", JoinPatternField[int](0, "es0", "id")),
		SelectFrom(0, "s0es1Id", JoinPatternField[int](0, "es1", "id")),
		SelectFrom(1, "s1es2Id", JoinPatternField[int](1, "es2", "id")),
		SelectFrom(1, "s1es3Id", JoinPatternField[int](1, "es3", "id")),
		SelectFrom(0, "s0", JoinEventValue[Event](0)),
		SelectFrom(1, "s1", JoinEventValue[Event](1)),
	).Query(StatementName("two-pattern-join"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
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
	for _, event := range []any{
		joinPatternS3{ID: 2, P30: "d"},
		joinPatternS0{ID: 3, P00: "a"},
		joinPatternS2{ID: 3, P20: "c"},
		joinPatternS1{ID: 1, P10: "b"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("two-pattern join rows = %#v", rows)
	}
	row := rows[0]
	for name, want := range map[string]any{
		"s0es0Id": 3,
		"s0es1Id": 1,
		"s1es2Id": 3,
		"s1es3Id": 2,
	} {
		if row.Get(name).Any() != want {
			t.Fatalf("two-pattern join %s = %#v, want %v", name, row.Get(name).Any(), want)
		}
	}
	left, ok := row.Get("s0").Any().(Event)
	if !ok {
		t.Fatalf("two-pattern wildcard left = %#v", row.Get("s0").Any())
	}
	leftEvent, ok := left.Get("es0").Any().(Event)
	if !ok || leftEvent.Underlying().(joinPatternS0).P00 != "a" {
		t.Fatalf("two-pattern wildcard left tag = %#v", left.Get("es0").Any())
	}
	right, ok := row.Get("s1").Any().(Event)
	if !ok {
		t.Fatalf("two-pattern wildcard right = %#v", row.Get("s1").Any())
	}
	rightEvent, ok := right.Get("es3").Any().(Event)
	if !ok || rightEvent.Underlying().(joinPatternS3).P30 != "d" {
		t.Fatalf("two-pattern wildcard right tag = %#v", right.Get("es3").Any())
	}
}

func TestPatternFilterJoinRedeployClearsStateMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	for _, register := range []func(*Environment) error{
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS0](env, "PatternS0"); return err },
		func(env *Environment) error { _, err := RegisterStruct[joinPatternS1](env, "PatternS1"); return err },
	} {
		if err := register(env); err != nil {
			t.Fatal(err)
		}
	}
	s0 := From[joinPatternS0](env, "PatternS0")
	s1 := From[joinPatternS1](env, "PatternS1")
	pattern := PatternFrom(s0, "es0a", Equal[string](Field[joinPatternS0, string]("p00"), Literal("a"))).
		Or(PatternFrom(s0, "es0b", Equal[string](Field[joinPatternS0, string]("p00"), Literal("b")))).
		Every()
	query := JoinMany(
		JoinPatternSource(pattern).Window(LengthWindow(5)),
		JoinSource(s1).Window(LengthWindow(5)),
	).On(AnyJoin(
		OnSourcesEqual(0, JoinPatternField[int](0, "es0a", "id"), 1, Field[joinPatternS1, int]("id")),
		OnSourcesEqual(0, JoinPatternField[int](0, "es0b", "id"), 1, Field[joinPatternS1, int]("id")),
	)).Select(
		SelectFrom(0, "es0aId", JoinPatternField[int](0, "es0a", "id")),
		SelectFrom(0, "es0bId", JoinPatternField[int](0, "es0b", "id")),
		SelectFrom(1, "s1Id", Field[joinPatternS1, int]("id")),
	).Query(StatementName("pattern-filter-join-redeploy"), WithOldStream())

	engine := env.NewEngine()
	var batches []ResultBatch
	deploy := func() *Deployment {
		t.Helper()
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return deployment
	}
	send0 := func(event joinPatternS0) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send1 := func(event joinPatternS1) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	deployment := deploy()
	send1(joinPatternS1{ID: 1, P10: "s1A"})
	send0(joinPatternS0{ID: 2, P00: "a"})
	send0(joinPatternS0{ID: 1, P00: "b"})
	send1(joinPatternS1{ID: 2, P10: "s2A"})
	send1(joinPatternS1{ID: 20, P10: "s20A"})
	send1(joinPatternS1{ID: 30, P10: "s30A"})
	send0(joinPatternS0{ID: 20, P00: "a"})
	send0(joinPatternS0{ID: 20, P00: "b"})
	send0(joinPatternS0{ID: 30, P00: "c"})
	send0(joinPatternS0{ID: 40, P00: "a"})
	send0(joinPatternS0{ID: 50, P00: "b"})
	if len(batches) != 5 || len(batches[4].Old) != 1 {
		t.Fatalf("pattern filter join initial lifecycle = %#v", batches)
	}
	old, ok := batches[4].Old[0].Row()
	if !ok || old.Get("es0aId").Any() != 2 || old.Get("s1Id").Any() != 2 {
		t.Fatalf("pattern filter join initial eviction = %#v", old.AsMap())
	}

	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	batches = nil
	send1(joinPatternS1{ID: 60, P10: "s20"})
	send0(joinPatternS0{ID: 70, P00: "a"})
	send0(joinPatternS0{ID: 71, P00: "b"})
	if len(batches) != 0 {
		t.Fatalf("events reached undeployed pattern join: %#v", batches)
	}

	deployment = deploy()
	send1(joinPatternS1{ID: 70, P10: "s1-70"})
	send0(joinPatternS0{ID: 60, P00: "a"})
	send1(joinPatternS1{ID: 20, P10: "s1"})
	if len(batches) != 0 {
		t.Fatalf("re-deployed pattern join emitted premature row: %#v", batches)
	}
	send0(joinPatternS0{ID: 70, P00: "b"})
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("re-deployed pattern join result = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("es0bId").Any() != 70 || row.Get("s1Id").Any() != 70 || row.Get("es0aId").IsPresent() {
		t.Fatalf("re-deployed pattern join row = %#v", row.AsMap())
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}
