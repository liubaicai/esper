package esper

import (
	"context"
	"testing"
)

// Parity coverage for the EXISTS/NOT EXISTS subquery executions of
// EPLSubselectExists:
// - EPLSubselectExistsInSelect: select exists (subquery) as value from S0
// - EPLSubselectExistsSceneOne: select id from S0 where exists (subquery)
// - EPLSubselectExistsFiltered: exists with correlated filter (s1.id=s0.id)
// - EPLSubselectTwoExistsFiltered: two exists with correlated filters
// - EPLSubselectNotExists: select id from S0 where not exists (subquery)
//
// The OM and Compile variants (EPLSubselectExistsInSelectOM,
// EPLSubselectExistsInSelectCompile, EPLSubselectNotExistsOM,
// EPLSubselectNotExistsCompile) are SODA/EPL text compilation, which the Go
// implementation does not support (by design); they are intentionally-different.
//
// Java semantics pinned by this slice:
//   - exists (subquery) returns true when the subquery retains at least one row,
//     false when empty (length(1000) window retains all rows until 1000).
//   - not exists (subquery) returns true when the subquery is empty, false when
//     at least one row is retained.
//   - correlated exists (s1.id=s0.id) filters the subquery by the outer event's
//     id; only matching rows count toward existence.

type subselectExistsS0 struct {
	ID int `esper:"id"`
}

type subselectExistsS1 struct {
	ID int `esper:"id"`
}

type subselectExistsS2 struct {
	ID int `esper:"id"`
}

// TestEPLSubselectExistsInSelectParity covers EPLSubselectExistsInSelect:
// select exists (select * from SupportBean_S1#length(1000)) as value from SupportBean_S0.
// The exists expression returns false when the S1 window is empty, true when
// at least one S1 row is retained.
func TestEPLSubselectExistsInSelectParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subselectExistsS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subselectExistsS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}

	s1 := From[subselectExistsS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	exists := SubqueryExists(s1, Literal(true))

	plan, err := env.Build(
		Select(From[subselectExistsS0](env, "SupportBean_S0"),
			Alias("value", exists),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	var received []bool
	for _, stmt := range deployment.Statements() {
		if stmt.Name() == "s0" {
			stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					received = append(received, r.Get("value").Any().(bool))
				}
				return nil
			})
		}
	}

	// Send S0(2) with empty S1 window: exists = false
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0] != false {
		t.Fatalf("expected [false], got %v", received)
	}

	// Send S1(-1), then S0(2): exists = true
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[1] != true {
		t.Fatalf("expected [false true], got %v", received)
	}
}

// TestEPLSubselectExistsSceneOneParity covers EPLSubselectExistsSceneOne:
// select id from SupportBean_S0 where exists (select * from SupportBean_S1#length(1000)).
// The where clause filters out S0 events when the S1 window is empty.
func TestEPLSubselectExistsSceneOneParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subselectExistsS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subselectExistsS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}

	s1 := From[subselectExistsS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	exists := SubqueryExists(s1, Literal(true))

	plan, err := env.Build(
		Select(From[subselectExistsS0](env, "SupportBean_S0").Filter(exists),
			Alias("id", Field[subselectExistsS0, int]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	var received []int
	for _, stmt := range deployment.Statements() {
		if stmt.Name() == "s0" {
			stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					received = append(received, r.Get("id").Any().(int))
				}
				return nil
			})
		}
	}

	// Send S0(2) with empty S1 window: filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("expected [], got %v", received)
	}

	// Send S1(-1), then S0(2): passes
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0] != 2 {
		t.Fatalf("expected [2], got %v", received)
	}

	// Send S1(-2), then S0(3): passes
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[1] != 3 {
		t.Fatalf("expected [2 3], got %v", received)
	}
}

// TestEPLSubselectExistsFilteredParity covers EPLSubselectExistsFiltered:
// select id from SupportBean_S0 as s0 where exists (select * from SupportBean_S1#length(1000) as s1 where s1.id=s0.id).
// The correlated exists filters the subquery by the outer event's id.
func TestEPLSubselectExistsFilteredParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subselectExistsS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subselectExistsS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}

	s1 := From[subselectExistsS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	exists := SubqueryExists(s1,
		Equal[int](Field[any, int]("id"), OuterField[int]("id")),
	)

	plan, err := env.Build(
		Select(From[subselectExistsS0](env, "SupportBean_S0").Filter(exists),
			Alias("id", Field[subselectExistsS0, int]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	var received []int
	for _, stmt := range deployment.Statements() {
		if stmt.Name() == "s0" {
			stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					received = append(received, r.Get("id").Any().(int))
				}
				return nil
			})
		}
	}

	// Send S0(2) with empty S1 window: filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("expected [], got %v", received)
	}

	// Send S1(-1), then S0(2): no match (id -1 != 2), filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("expected [], got %v", received)
	}

	// Send S1(-2), then S0(-2): match, passes
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: -2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0] != -2 {
		t.Fatalf("expected [-2], got %v", received)
	}

	// Send S1(1), S1(2), S1(3), then S0(3): match (id 3), passes
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[1] != 3 {
		t.Fatalf("expected [-2 3], got %v", received)
	}
}

// TestEPLSubselectTwoExistsFilteredParity covers EPLSubselectTwoExistsFiltered:
// select id from SupportBean_S0 as s0 where exists (S1 filtered) and exists (S2 filtered).
// Both exists conditions must be satisfied for the S0 event to pass.
func TestEPLSubselectTwoExistsFilteredParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subselectExistsS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subselectExistsS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subselectExistsS2](env, "SupportBean_S2"); err != nil {
		t.Fatal(err)
	}

	s1 := From[subselectExistsS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	exists1 := SubqueryExists(s1,
		Equal[int](Field[any, int]("id"), OuterField[int]("id")),
	)

	s2 := From[subselectExistsS2](env, "SupportBean_S2").Window(LengthWindow(1000)).AsRecord()
	exists2 := SubqueryExists(s2,
		Equal[int](Field[any, int]("id"), OuterField[int]("id")),
	)

	plan, err := env.Build(
		Select(From[subselectExistsS0](env, "SupportBean_S0").Filter(And(exists1, exists2)),
			Alias("id", Field[subselectExistsS0, int]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	var received []int
	for _, stmt := range deployment.Statements() {
		if stmt.Name() == "s0" {
			stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					received = append(received, r.Get("id").Any().(int))
				}
				return nil
			})
		}
	}

	// Send S0(2) with empty S1/S2 windows: filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("expected [], got %v", received)
	}

	// Send S2(3), then S0(3): S1 empty, filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS2{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("expected [], got %v", received)
	}

	// Send S1(3), then S0(3): both match, passes
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0] != 3 {
		t.Fatalf("expected [3], got %v", received)
	}

	// Send S1(1), S1(2), S2(1), then S0(1): both match (S1 has 1, S2 has 1), passes
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS2{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[1] != 1 {
		t.Fatalf("expected [3 1], got %v", received)
	}

	// Send S0(2), S0(0): S1 has 2 but S2 doesn't have 2 or 0, filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 {
		t.Fatalf("expected [3 1], got %v", received)
	}
}

// TestEPLSubselectNotExistsParity covers EPLSubselectNotExists:
// select id from SupportBean_S0 where not exists (select * from SupportBean_S1#length(1000)).
// The not exists expression returns true when the S1 window is empty, false when
// at least one S1 row is retained.
func TestEPLSubselectNotExistsParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[subselectExistsS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[subselectExistsS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}

	s1 := From[subselectExistsS1](env, "SupportBean_S1").Window(LengthWindow(1000)).AsRecord()
	notExists := Not(SubqueryExists(s1, Literal(true)))

	plan, err := env.Build(
		Select(From[subselectExistsS0](env, "SupportBean_S0").Filter(notExists),
			Alias("id", Field[subselectExistsS0, int]("id")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())

	var received []int
	for _, stmt := range deployment.Statements() {
		if stmt.Name() == "s0" {
			stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, r := range batch.New {
					received = append(received, r.Get("id").Any().(int))
				}
				return nil
			})
		}
	}

	// Send S0(2) with empty S1 window: not exists = true, passes
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 2}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0] != 2 {
		t.Fatalf("expected [2], got %v", received)
	}

	// Send S1(-1), then S0(1): not exists = false, filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("expected [2], got %v", received)
	}

	// Send S1(-2), then S0(3): not exists = false, filtered out
	if err := engine.SendEvent(context.Background(), subselectExistsS1{ID: -2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), subselectExistsS0{ID: 3}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("expected [2], got %v", received)
	}
}
