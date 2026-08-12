package esper

import (
	"context"
	"reflect"
	"testing"
)

type insertPatternS0 struct {
	ID int `esper:"id"`
}

type insertPatternS1 struct {
	ID int `esper:"id"`
}

// TestInsertIntoFromPatternMatchesEsper covers EPLInsertIntoPropsWildcard:
// insert pattern match results into a new stream, then consume from it.
func TestInsertIntoFromPatternMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[insertPatternS0](env, "S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[insertPatternS1](env, "S1"); err != nil {
		t.Fatal(err)
	}

	// Register the target event type for insert-into
	fields := []FieldSpec{
		FieldDef("es0id", reflect.TypeOf(0)),
		FieldDef("es1id", reflect.TypeOf(0)),
	}
	if _, err := RegisterMap(env, "MyStream", fields); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	// Pattern: every (es0=S0 or es1=S1), insert es0id/es1id into MyStream
	s0Source := From[insertPatternS0](env, "S0")
	s1Source := From[insertPatternS1](env, "S1")
	patternS0 := PatternFrom(s0Source, "es0", Literal(true))
	patternS1 := PatternFrom(s1Source, "es1", Literal(true))
	orPattern := patternS0.Or(patternS1).Every()

	// Select from pattern and insert into MyStream
	insertQuery := orPattern.Select(
		Alias("es0id", TagField[int]("es0", "id")),
		Alias("es1id", TagField[int]("es1", "id")),
	).InsertInto("MyStream", StatementName("i0"))

	insertPlan, err := env.Build(insertQuery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), insertPlan); err != nil {
		t.Fatal(err)
	}

	// Consumer: select * from MyStream
	consumerQuery := FromAny(env, "MyStream").Select(
		Alias("es0id", Field[Event, int]("es0id")),
		Alias("es1id", Field[Event, int]("es1id")),
	).Query(StatementName("s0"))

	consumerPlan, err := env.Build(consumerQuery)
	if err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}

	var batches []ResultBatch
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Send S0 event - pattern matches with es0 set, es1 null
	if err := engine.SendEvent(context.Background(), insertPatternS0{ID: 10}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("after S0: batches=%d, want 1", len(batches))
	}
	row := batches[0].New[0]
	if got := row.Get("es0id").Any(); got != 10 {
		t.Fatalf("es0id=%v, want 10", got)
	}
	// es1id should be null/absent for S0-only match
	if got := row.Get("es1id"); got.IsPresent() && got.Any() != nil && got.Any() != 0 {
		t.Fatalf("es1id should be nil/zero for S0-only match, got %v", got.Any())
	}

	// Send S1 event - pattern matches with es1 set, es0 null
	if err := engine.SendEvent(context.Background(), insertPatternS1{ID: 20}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("after S1: batches=%d, want 2", len(batches))
	}
	row = batches[1].New[0]
	if got := row.Get("es1id").Any(); got != 20 {
		t.Fatalf("es1id=%v, want 20", got)
	}
}
