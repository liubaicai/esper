package esper

import (
	"context"
	"testing"
	"time"
)

// The dispatch pruning guard must be observably invisible: statements that
// cannot accept an event are skipped, every other statement (including all
// guard-protected shapes) keeps its exact behavior.

type acceptIndexEventA struct {
	Value string `esper:"value"`
}

type acceptIndexEventB struct {
	Value string `esper:"value"`
}

type acceptIndexSub struct {
	Key string `esper:"key"`
}

type acceptIndexParent struct {
	Base string `esper:"base"`
}

type acceptIndexChild struct {
	acceptIndexParent
	Extra string `esper:"extra"`
}

func acceptIndexEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	var parentSchema Schema
	for _, registration := range []struct {
		name string
		fn   func() error
	}{
		{"AcceptIndexA", func() error { _, err := RegisterStruct[acceptIndexEventA](env, "AcceptIndexA"); return err }},
		{"AcceptIndexB", func() error { _, err := RegisterStruct[acceptIndexEventB](env, "AcceptIndexB"); return err }},
		{"AcceptIndexSub", func() error { _, err := RegisterStruct[acceptIndexSub](env, "AcceptIndexSub"); return err }},
		{"AcceptIndexParent", func() error {
			var err error
			parentSchema, err = RegisterStruct[acceptIndexParent](env, "AcceptIndexParent")
			return err
		}},
		{"AcceptIndexChild", func() error {
			_, err := RegisterStruct[acceptIndexChild](env, "AcceptIndexChild", WithSchemaParent(parentSchema))
			return err
		}},
	} {
		if err := registration.fn(); err != nil {
			t.Fatalf("register %s: %v", registration.name, err)
		}
	}
	return env
}

// TestAcceptIndexPrunesOnlyNonAcceptingStatements deploys three plain filter
// statements over two event types in one engine and checks that events reach
// exactly the statements of their own type while the unmatched listener
// accounting is preserved.
func TestAcceptIndexPrunesOnlyNonAcceptingStatements(t *testing.T) {
	env := acceptIndexEnv(t)
	ctx := context.Background()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()

	build := func(name string, typ string) *Statement {
		plan, err := env.Build(From[acceptIndexEventA](env, typ).
			Filter(Equal[string](Field[acceptIndexEventA, string]("value"), Literal("hit"))).
			Query(StatementName(name)))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			t.Fatal(err)
		}
		return deployment.Statements()[0]
	}
	statementA1 := build("a1", "AcceptIndexA")
	statementA2 := build("a2", "AcceptIndexA")
	bPlan, err := env.Build(From[acceptIndexEventB](env, "AcceptIndexB").
		Filter(Equal[string](Field[acceptIndexEventB, string]("value"), Literal("hit"))).
		Query(StatementName("b")))
	if err != nil {
		t.Fatal(err)
	}
	bDeployment, err := engine.Deploy(ctx, bPlan)
	if err != nil {
		t.Fatal(err)
	}
	statementB := bDeployment.Statements()[0]

	counts := map[string]int{}
	subscribe := func(statement *Statement) {
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			if len(batch.New) > 0 {
				counts[statement.name]++
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	subscribe(statementA1)
	subscribe(statementA2)
	subscribe(statementB)

	unmatched := 0
	engine.SetUnmatchedListener(func(_ context.Context, _ Event) error {
		unmatched++
		return nil
	})

	if err := engine.Send(ctx, "AcceptIndexA", acceptIndexEventA{Value: "hit"}); err != nil {
		t.Fatal(err)
	}
	if counts["a1"] != 1 || counts["a2"] != 1 || counts["b"] != 0 {
		t.Fatalf("after A hit: counts=%v", counts)
	}
	if unmatched != 0 {
		t.Fatalf("matched event recorded unmatched=%d", unmatched)
	}

	if err := engine.Send(ctx, "AcceptIndexB", acceptIndexEventB{Value: "hit"}); err != nil {
		t.Fatal(err)
	}
	if counts["a1"] != 1 || counts["a2"] != 1 || counts["b"] != 1 {
		t.Fatalf("after B hit: counts=%v", counts)
	}

	if err := engine.Send(ctx, "AcceptIndexB", acceptIndexEventB{Value: "miss"}); err != nil {
		t.Fatal(err)
	}
	if counts["b"] != 1 {
		t.Fatalf("miss changed counts: %v", counts)
	}
	if unmatched != 1 {
		t.Fatalf("unmatched=%d, want 1", unmatched)
	}
}

// TestAcceptIndexKeepsSubqueryFeeds pins the subquery guard: a statement on
// AcceptIndexA carrying a subquery over AcceptIndexSub must keep observing
// AcceptIndexSub events even though its own source can never accept them.
func TestAcceptIndexKeepsSubqueryFeeds(t *testing.T) {
	env := acceptIndexEnv(t)
	ctx := context.Background()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()

	source := From[acceptIndexEventA](env, "AcceptIndexA")
	plan, err := env.Build(Select(
		source,
		Alias("c0", SubquerySum[int64](
			From[acceptIndexSub](env, "AcceptIndexSub").Window(LengthWindow(10)).AsRecord(),
			Literal[int64](1),
			Equal[string](Field[acceptIndexSub, string]("key"), Literal("k")),
		)),
	).Query(StatementName("subquery-guard")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if statement.acceptIndex.prunable {
		t.Fatal("statement with subquery must not be prunable")
	}
	received := []float64{}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			value, _ := numericValue(row.Get("c0"))
			received = append(received, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Feed the subquery source; the statement's own source can never accept
	// these, but the subquery window must still accumulate.
	for range 3 {
		if err := engine.Send(ctx, "AcceptIndexSub", acceptIndexSub{Key: "k"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Send(ctx, "AcceptIndexA", acceptIndexEventA{Value: "x"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0] != 3 {
		t.Fatalf("subquery sum = %v, want single row 3", received)
	}
}

// TestAcceptIndexSupertypeAndDescriptor pins the event-side name set: a
// subtype event (AcceptIndexChild declaring AcceptIndexParent) must reach a
// statement sourcing the parent type, and the child statement's descriptor
// must be prunable with the child name.
func TestAcceptIndexSupertypeAndDescriptor(t *testing.T) {
	env := acceptIndexEnv(t)
	ctx := context.Background()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()

	parentPlan, err := env.Build(From[acceptIndexParent](env, "AcceptIndexParent").Query(StatementName("parent")))
	if err != nil {
		t.Fatal(err)
	}
	parentDeployment, err := engine.Deploy(ctx, parentPlan)
	if err != nil {
		t.Fatal(err)
	}
	parentStatement := parentDeployment.Statements()[0]
	if !parentStatement.acceptIndex.prunable {
		t.Fatalf("plain parent statement descriptor: %#v", parentStatement.acceptIndex)
	}
	parentRows := 0
	if _, err := parentStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		parentRows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	childPlan, err := env.Build(From[acceptIndexChild](env, "AcceptIndexChild").Query(StatementName("child")))
	if err != nil {
		t.Fatal(err)
	}
	childDeployment, err := engine.Deploy(ctx, childPlan)
	if err != nil {
		t.Fatal(err)
	}
	childStatement := childDeployment.Statements()[0]
	if !childStatement.acceptIndex.prunable {
		t.Fatalf("plain child statement descriptor: %#v", childStatement.acceptIndex)
	}
	childRows := 0
	if _, err := childStatement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		childRows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The child event's accepted set includes the parent name, so the parent
	// statement must not be pruned for it.
	if err := engine.Send(ctx, "AcceptIndexChild", acceptIndexChild{Extra: "e"}); err != nil {
		t.Fatal(err)
	}
	if childRows != 1 {
		t.Fatalf("child rows = %d, want 1", childRows)
	}
	if parentRows != 1 {
		t.Fatalf("parent rows = %d, want 1 (supertype acceptance must survive pruning)", parentRows)
	}

	// The parent event does not reach the child statement; with pruning that
	// skip happens before any evaluation, with identical output.
	if err := engine.Send(ctx, "AcceptIndexParent", acceptIndexParent{Base: "b"}); err != nil {
		t.Fatal(err)
	}
	if parentRows != 2 || childRows != 1 {
		t.Fatalf("parent rows=%d child rows=%d, want 2/1", parentRows, childRows)
	}
}

// TestAcceptIndexVariantNotPrunable pins that variant-sourced statements stay
// on the full path: member events must keep resolving through variant
// membership rather than a name set.
func TestAcceptIndexVariantNotPrunable(t *testing.T) {
	env := acceptIndexEnv(t)
	memberSchema, schemaOK := env.Schema("AcceptIndexA")
	if !schemaOK {
		t.Fatal("member schema missing")
	}
	if _, err := RegisterVariant(env, "AcceptIndexVariant", memberSchema); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()

	plan, err := env.Build(From[acceptIndexEventA](env, "AcceptIndexVariant").Query(StatementName("variant")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if statement.acceptIndex.prunable {
		t.Fatal("variant-sourced statement must not be prunable")
	}
	// Variant sources are never pruned: their acceptance is membership
	// semantics (StreamType equality for routed events), not a name set. Direct
	// sends of the member type are not accepted by a variant source in either
	// path, so delivery itself is pinned by the variant suite; here the guard
	// only asserts the descriptor keeps the statement on the full path.
	event, err := newEvent(memberSchema, acceptIndexEventA{Value: "v"}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	if sourceNodeAcceptsEvent(env, statement.plan.query.input, event) {
		t.Fatal("direct member-type send must not be accepted by a variant source")
	}
	if !statement.matchesEventFilter(event, engine.Now(), nil) == false {
		t.Fatal("matchesEventFilter must agree with the acceptance walk")
	}
}

// TestAcceptIndexRoutedEventNames pins the routed-event acceptance set: a
// routed event is accepted by exact source-name equality for ordinary
// sources, while named-window (and contained) sources also accept by their
// exact TypeName. The set must therefore carry both names, or a
// named-window-sourced statement would be pruned even though the generic
// acceptance walk accepts it.
func TestAcceptIndexRoutedEventNames(t *testing.T) {
	env := acceptIndexEnv(t)
	schema := mustSchemaOf(env, "AcceptIndexA")
	event, err := newEvent(schema, acceptIndexEventA{Value: "v"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	event.streamType = "RoutedTarget"
	names := eventAcceptedTypeNames(env, event)
	if _, ok := names["RoutedTarget"]; !ok {
		t.Fatalf("routed set must contain the stream type: %v", names)
	}
	if _, ok := names["AcceptIndexA"]; !ok {
		t.Fatalf("routed set must keep the type name for named-window acceptance: %v", names)
	}
	namedWindowIndex := statementAcceptIndex{prunable: true, names: []string{"AcceptIndexA"}}
	if !namedWindowIndex.mayAccept(names) {
		t.Fatal("named-window TypeName acceptance must survive pruning")
	}
	otherIndex := statementAcceptIndex{prunable: true, names: []string{"AcceptIndexB"}}
	if otherIndex.mayAccept(names) {
		t.Fatal("unrelated source name must not match the routed set")
	}
}

// TestAcceptIndexRoutedCacheKeyIncludesTypeName pins the routed memoization
// key: two routed events sharing a stream type but naming different member
// types must each get their own accepted-name set. Keying by stream type
// alone would serve the first set to the second event and prune a statement
// the generic acceptance walk accepts.
func TestAcceptIndexRoutedCacheKeyIncludesTypeName(t *testing.T) {
	env := acceptIndexEnv(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	schemaA := mustSchemaOf(env, "AcceptIndexA")
	schemaB := mustSchemaOf(env, "AcceptIndexB")
	now := time.Now()

	first, err := newEvent(schemaA, acceptIndexEventA{Value: "a"}, now)
	if err != nil {
		t.Fatal(err)
	}
	first.streamType = "AcceptIndexVariant"
	second, err := newEvent(schemaB, acceptIndexEventB{Value: "b"}, now)
	if err != nil {
		t.Fatal(err)
	}
	second.streamType = "AcceptIndexVariant"

	firstNames := engine.eventAcceptedTypeNamesCached(first)
	secondNames := engine.eventAcceptedTypeNamesCached(second)
	if _, ok := firstNames["AcceptIndexA"]; !ok {
		t.Fatalf("first routed set lost its type name: %v", firstNames)
	}
	if _, ok := secondNames["AcceptIndexB"]; !ok {
		t.Fatalf("second routed set must be recomputed for its own type name, got %v", secondNames)
	}
	if _, leaked := secondNames["AcceptIndexA"]; leaked {
		t.Fatalf("second routed set leaked the first type name: %v", secondNames)
	}
}

// TestAcceptIndexGuardsKeepContextAndContainedStatements pins the descriptor
// guards that protect per-event side effects: a context statement and a
// contained/unnest source must both stay off the pruning path.
func TestAcceptIndexGuardsKeepContextAndContainedStatements(t *testing.T) {
	env := acceptIndexEnv(t)
	if _, err := RegisterStruct[unnestOrder](env, "UnnestOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[unnestBook](env, "UnnestBook"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()

	if _, err := CreateInitiatedTerminatedContext(env, "accept-index-context", Literal("global"),
		Equal[string](Field[acceptIndexEventA, string]("value"), Literal("start")),
		Equal[string](Field[acceptIndexEventA, string]("value"), Literal("end"))); err != nil {
		t.Fatal(err)
	}
	contextPlan, err := env.Build(From[acceptIndexEventA](env, "AcceptIndexA").
		Filter(Equal[string](Field[acceptIndexEventA, string]("value"), Literal("hit"))).
		Query(StatementName("context-statement"), WithContext("accept-index-context")))
	if err != nil {
		t.Fatal(err)
	}
	contextDeployment, err := engine.Deploy(ctx, contextPlan)
	if err != nil {
		t.Fatal(err)
	}
	if contextDeployment.Statements()[0].acceptIndex.prunable {
		t.Fatal("context statement must not be prunable")
	}

	unnestPlan, err := env.Build(Unnest[unnestOrder, unnestBook](
		From[unnestOrder](env, "UnnestOrder"),
		Property[[]unnestBook](EventValue[unnestOrder](), "books"),
	).Query(StatementName("contained-statement")))
	if err != nil {
		t.Fatal(err)
	}
	unnestDeployment, err := engine.Deploy(ctx, unnestPlan)
	if err != nil {
		t.Fatal(err)
	}
	if unnestDeployment.Statements()[0].acceptIndex.prunable {
		t.Fatal("contained/unnest source must not be prunable")
	}
}

// TestAcceptIndexCompiledStatelessEndToEnd pins the compiled-predicate wiring
// through the public send path: a statement whose plan compiled must deliver
// exactly the rows the generic pipeline produced (accepted rows only).
func TestAcceptIndexCompiledStatelessEndToEnd(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[statelessFilterEvent](env, "StatelessFilterEvent"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[statelessFilterEvent](env, "StatelessFilterEvent").
		Filter(Equal[string](Field[statelessFilterEvent, string]("category"), Literal("keep"))).
		Query(StatementName("compiled-e2e")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	defer func() { _ = engine.Close(ctx) }()
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	rows := 0
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "drop", Value: 1}); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("rejected event delivered %d rows", rows)
	}
	if err := engine.SendEvent(ctx, statelessFilterEvent{Category: "keep", Value: 2}); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("accepted event delivered %d rows, want 1", rows)
	}
	if plan := statement.statelessPlan; plan == nil || plan.compiled == nil {
		t.Fatal("stateless plan must compile for this predicate shape")
	}
}

// BenchmarkAcceptIndexMultiStatement measures the dispatch loop over many
// plain statements of several event types in one engine, the application
// shape the type-level pruning serves: one event must reach its own type's
// statements while all other statements are skipped before evaluation.
func BenchmarkAcceptIndexMultiStatement(b *testing.B) {
	env := NewEnvironment()
	if _, err := RegisterStruct[acceptIndexEventA](env, "AcceptIndexA"); err != nil {
		b.Fatal(err)
	}
	if _, err := RegisterStruct[acceptIndexEventB](env, "AcceptIndexB"); err != nil {
		b.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	defer func() { _ = engine.Close(ctx) }()
	for index := range 64 {
		name := StatementName("s" + string(rune('a'+index%26)) + string(rune('a'+index/26)))
		var plan Plan
		var err error
		if index%4 == 3 {
			plan, err = env.Build(From[acceptIndexEventB](env, "AcceptIndexB").
				Filter(Equal[string](Field[acceptIndexEventB, string]("value"), Literal("hit"))).
				Query(name))
		} else {
			plan, err = env.Build(From[acceptIndexEventA](env, "AcceptIndexA").
				Filter(Equal[string](Field[acceptIndexEventA, string]("value"), Literal("hit"))).
				Query(name))
		}
		if err != nil {
			b.Fatal(err)
		}
		deployment, deployErr := engine.Deploy(ctx, plan)
		if deployErr != nil {
			b.Fatal(deployErr)
		}
		if _, subErr := deployment.Statements()[0].Subscribe(func(_ context.Context, _ ResultBatch) error { return nil }); subErr != nil {
			b.Fatal(subErr)
		}
	}
	event := acceptIndexEventA{Value: "miss"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := engine.SendEvent(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}
