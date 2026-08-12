package esper

import (
	"context"
	"reflect"
	"testing"
)

type patternCompositeA struct {
	ID string `esper:"id"`
}

type patternCompositeB struct {
	ID string `esper:"id"`
}

func newPatternCompositeEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternCompositeA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternCompositeB](env, "SupportBean_B"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	return env, engine
}

// TestPatternCompositeFollowedByFilterMatchesEsper covers
// PatternCompositeSelect PatternFollowedByFilter:
// insert into StreamOne select * from pattern [a=SupportBean_A -> b=SupportBean_B],
// then select *, 1 as code from StreamOne. The wildcard pattern insert carries
// the tagged match events as fragment properties; Go projects them explicitly
// as typed []Event values (TagEvents), which preserve the per-tag event type
// and property access of Esper fragments.
func TestPatternCompositeFollowedByFilterMatchesEsper(t *testing.T) {
	env, engine := newPatternCompositeEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	eventSlice := reflect.TypeOf([]Event(nil))
	if _, err := RegisterMap(env, "StreamOne", []FieldSpec{
		FieldDef("a", eventSlice),
		FieldDef("b", eventSlice),
	}); err != nil {
		t.Fatal(err)
	}

	a := From[patternCompositeA](env, "SupportBean_A")
	b := From[patternCompositeB](env, "SupportBean_B")
	pattern := PatternFrom(a, "a", Literal(true)).Then(PatternFrom(b, "b", Literal(true)))
	insertPlan, err := env.Build(pattern.Select(
		Alias("a", TagEvents("a")),
		Alias("b", TagEvents("b")),
	).InsertInto("StreamOne", StatementName("insert")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}

	consumerPlan, err := env.Build(FromAny(env, "StreamOne").Select(
		Alias("a", Field[Event, []Event]("a")),
		Alias("b", Field[Event, []Event]("b")),
		Alias("code", Literal(1)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), patternCompositeA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternCompositeB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("consumer rows = %#v", rows)
	}
	row := rows[0]
	if got := row.Get("code").Any(); got != 1 {
		t.Fatalf("code = %v, want 1", got)
	}
	aEvents, ok := row.Get("a").Any().([]Event)
	if !ok || len(aEvents) != 1 {
		t.Fatalf("a = %#v, want one fragment event", row.Get("a").Any())
	}
	if aEvents[0].TypeName() != "SupportBean_A" || aEvents[0].Get("id").Any() != "A1" {
		t.Fatalf("a fragment = %s/%v, want SupportBean_A/A1", aEvents[0].TypeName(), aEvents[0].Get("id").Any())
	}
	bEvents, ok := row.Get("b").Any().([]Event)
	if !ok || len(bEvents) != 1 {
		t.Fatalf("b = %#v, want one fragment event", row.Get("b").Any())
	}
	if bEvents[0].TypeName() != "SupportBean_B" || bEvents[0].Get("id").Any() != "B1" {
		t.Fatalf("b fragment = %s/%v, want SupportBean_B/B1", bEvents[0].TypeName(), bEvents[0].Get("id").Any())
	}
}

// TestPatternCompositeFragmentMatchesEsper covers PatternCompositeSelect
// PatternFragment: select * from pattern [[2] a=SupportBean_A -> b=SupportBean_B].
// The repeated tag a collects an indexed fragment (A1, A2) and the single tag
// b a single fragment; Go exposes both as typed []Event projections with
// per-element type and property access (a[0].id == A1, a[1].id == A2).
func TestPatternCompositeFragmentMatchesEsper(t *testing.T) {
	env, engine := newPatternCompositeEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	a := From[patternCompositeA](env, "SupportBean_A")
	b := From[patternCompositeB](env, "SupportBean_B")
	repeated := PatternFrom(a, "a", Literal(true)).MatchUntil(2, 2)
	pattern := repeated.Then(PatternFrom(b, "b", Literal(true)))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagEvents("a")),
		Alias("b", TagEvents("b")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
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

	if err := engine.SendEvent(context.Background(), patternCompositeA{ID: "A1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternCompositeA{ID: "A2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternCompositeB{ID: "B1"}); err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	row := rows[0]
	aEvents, ok := row.Get("a").Any().([]Event)
	if !ok || len(aEvents) != 2 {
		t.Fatalf("a = %#v, want two indexed fragment events", row.Get("a").Any())
	}
	if aEvents[0].TypeName() != "SupportBean_A" || aEvents[0].Get("id").Any() != "A1" {
		t.Fatalf("a[0] = %s/%v, want SupportBean_A/A1", aEvents[0].TypeName(), aEvents[0].Get("id").Any())
	}
	if aEvents[1].TypeName() != "SupportBean_A" || aEvents[1].Get("id").Any() != "A2" {
		t.Fatalf("a[1] = %s/%v, want SupportBean_A/A2", aEvents[1].TypeName(), aEvents[1].Get("id").Any())
	}
	bEvents, ok := row.Get("b").Any().([]Event)
	if !ok || len(bEvents) != 1 {
		t.Fatalf("b = %#v, want one fragment event", row.Get("b").Any())
	}
	if bEvents[0].TypeName() != "SupportBean_B" || bEvents[0].Get("id").Any() != "B1" {
		t.Fatalf("b[0] = %s/%v, want SupportBean_B/B1", bEvents[0].TypeName(), bEvents[0].Get("id").Any())
	}
}
