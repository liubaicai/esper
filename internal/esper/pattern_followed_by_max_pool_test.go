package esper

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// Parity coverage for the runtime-wide pattern subexpression pool suites
// (PatternOperatorFollowedByMax2Noprevent / 2Prevent / 4Prevent, configured via
// ConfigurationRuntimePatterns.maxSubexpressions + preventStart) and the two
// remaining PatternOperatorFollowedByMax executions
// (PatternSinglePermFalseAndQuit, PatternOperatorFollowedByMaxInvalid).

type poolLimitRecorder struct {
	events []PatternRuntimeSubexpressionLimitEvent
}

func (r *poolLimitRecorder) listener() PatternRuntimeSubexpressionLimitListener {
	return PatternRuntimeSubexpressionLimitListenerFunc(func(event PatternRuntimeSubexpressionLimitEvent) {
		r.events = append(r.events, event)
	})
}

func (r *poolLimitRecorder) requireSingle(t *testing.T, statement string, maximum int64, counts map[string]int64) {
	t.Helper()
	if len(r.events) != 1 {
		t.Fatalf("pool limit events = %v, want exactly one", r.events)
	}
	event := r.events[0]
	if event.StatementName != statement || event.Maximum != maximum {
		t.Fatalf("pool limit event = %+v, want statement %q max %d", event, statement, maximum)
	}
	if len(event.Counts) != len(counts) {
		t.Fatalf("pool limit counts = %v, want %v", event.Counts, counts)
	}
	for name, want := range counts {
		if event.Counts[name] != want {
			t.Fatalf("pool limit counts = %v, want %v", event.Counts, counts)
		}
	}
	r.events = nil
}

func (r *poolLimitRecorder) requireEmpty(t *testing.T) {
	t.Helper()
	if len(r.events) != 0 {
		t.Fatalf("pool limit events = %v, want none", r.events)
	}
}

type edgeLimitRecorder struct {
	events []PatternSubexpressionLimitEvent
}

func (r *edgeLimitRecorder) listener() PatternSubexpressionLimitListener {
	return PatternSubexpressionLimitListenerFunc(func(event PatternSubexpressionLimitEvent) {
		r.events = append(r.events, event)
	})
}

func (r *edgeLimitRecorder) requireSingle(t *testing.T, statement string, maximum int) {
	t.Helper()
	if len(r.events) != 1 {
		t.Fatalf("edge limit events = %v, want exactly one", r.events)
	}
	event := r.events[0]
	if event.StatementName != statement || event.Maximum != maximum {
		t.Fatalf("edge limit event = %+v, want statement %q max %d", event, statement, maximum)
	}
	r.events = nil
}

func (r *edgeLimitRecorder) requireEmpty(t *testing.T) {
	t.Helper()
	if len(r.events) != 0 {
		t.Fatalf("edge limit events = %v, want none", r.events)
	}
}

func newPoolParityEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotB](env, "SupportBean_B"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotC](env, "SupportBean_C"); err != nil {
		t.Fatal(err)
	}
	return env
}

func deployPatternAB(t *testing.T, env *Environment, engine *Engine, name string) *Deployment {
	t.Helper()
	aStream := From[patternNotA](env, "SupportBean_A")
	bStream := From[patternNotB](env, "SupportBean_B")
	pattern := PatternFrom(aStream, "a", Literal(true)).Every().
		Then(PatternFrom(bStream, "b", Literal(true)))
	plan, err := env.Build(pattern.Select(
		Alias("a", TagField[string]("a", "id")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName(name)))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func sendPoolA(t *testing.T, engine *Engine, id string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), patternNotA{ID: id}); err != nil {
		t.Fatal(err)
	}
}

func sendPoolB(t *testing.T, engine *Engine, id string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), patternNotB{ID: id}); err != nil {
		t.Fatal(err)
	}
}

func sendPoolC(t *testing.T, engine *Engine, id string) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), patternNotC{ID: id}); err != nil {
		t.Fatal(err)
	}
}

// rowCollector collects sorted "a=X b=Y" rows per result batch.
type rowCollector struct {
	batches [][]string
}

func (c *rowCollector) subscriber() func(context.Context, ResultBatch) error {
	return func(_ context.Context, batch ResultBatch) error {
		var rows []string
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return fmt.Errorf("no row")
			}
			parts := []string{}
			for _, name := range []string{"a", "b"} {
				if value := row.Get(name); value.State() == ValuePresent {
					parts = append(parts, fmt.Sprintf("%s=%v", name, value.Any()))
				}
			}
			rows = append(rows, strings.Join(parts, " "))
		}
		sort.Strings(rows)
		c.batches = append(c.batches, rows)
		return nil
	}
}

func (c *rowCollector) requireLast(t *testing.T, want ...string) {
	t.Helper()
	if len(c.batches) == 0 {
		t.Fatalf("no result batches, want %v", want)
	}
	got := c.batches[len(c.batches)-1]
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", sorted) {
		t.Fatalf("last batch = %v, want %v", got, sorted)
	}
}

// TestPatternFollowedByMaxPool2NopreventMatchesEsper mirrors
// PatternOperatorFollowedByMax2Noprevent: pool max 2 with preventStart=false —
// overflows report ConditionPatternRuntimeSubexpressionMax but still admit.
func TestPatternFollowedByMaxPool2NopreventMatchesEsper(t *testing.T) {
	env := newPoolParityEnv(t)
	engine := NewEngine(env, WithPatternSubexpressionMax(2), WithPatternSubexpressionPreventStart(false))
	defer func() { _ = engine.Close(context.Background()) }()
	recorder := &poolLimitRecorder{}
	if err := engine.AddPatternRuntimeSubexpressionLimitListener(recorder.listener()); err != nil {
		t.Fatal(err)
	}
	deployment := deployPatternAB(t, env, engine, "A")
	rows := &rowCollector{}
	if _, err := deployment.Statements()[0].Subscribe(rows.subscriber()); err != nil {
		t.Fatal(err)
	}

	sendPoolA(t, engine, "A1")
	sendPoolA(t, engine, "A2")
	recorder.requireEmpty(t)

	sendPoolA(t, engine, "A3")
	recorder.requireSingle(t, "A", 2, map[string]int64{"A": 2})

	sendPoolA(t, engine, "A4")
	recorder.requireSingle(t, "A", 2, map[string]int64{"A": 3})

	sendPoolB(t, engine, "B1")
	rows.requireLast(t, "a=A1 b=B1", "a=A2 b=B1", "a=A3 b=B1", "a=A4 b=B1")
	recorder.requireEmpty(t)
}

// TestPatternFollowedByMaxPool2PreventMatchesEsper mirrors
// PatternOperatorFollowedByMax2Prevent: pool max 2 with preventStart=true —
// overflowing subexpressions never start.
func TestPatternFollowedByMaxPool2PreventMatchesEsper(t *testing.T) {
	env := newPoolParityEnv(t)
	engine := NewEngine(env, WithPatternSubexpressionMax(2), WithPatternSubexpressionPreventStart(true))
	defer func() { _ = engine.Close(context.Background()) }()
	recorder := &poolLimitRecorder{}
	if err := engine.AddPatternRuntimeSubexpressionLimitListener(recorder.listener()); err != nil {
		t.Fatal(err)
	}
	deployment := deployPatternAB(t, env, engine, "A")
	rows := &rowCollector{}
	if _, err := deployment.Statements()[0].Subscribe(rows.subscriber()); err != nil {
		t.Fatal(err)
	}

	sendPoolA(t, engine, "A1")
	sendPoolA(t, engine, "A2")
	sendPoolA(t, engine, "A3")
	recorder.requireSingle(t, "A", 2, map[string]int64{"A": 2})

	sendPoolB(t, engine, "B1")
	rows.requireLast(t, "a=A1 b=B1", "a=A2 b=B1")
	recorder.requireEmpty(t)

	sendPoolA(t, engine, "A4")
	sendPoolB(t, engine, "B2")
	rows.requireLast(t, "a=A4 b=B2")
	recorder.requireEmpty(t)

	for i := 5; i < 9; i++ {
		sendPoolA(t, engine, fmt.Sprintf("A%d", i))
		if i >= 7 {
			recorder.requireSingle(t, "A", 2, map[string]int64{"A": 2})
		}
	}
	recorder.requireEmpty(t)

	sendPoolB(t, engine, "B3")
	rows.requireLast(t, "a=A5 b=B3", "a=A6 b=B3")

	batchesBefore := len(rows.batches)
	sendPoolB(t, engine, "B4")
	if len(rows.batches) != batchesBefore {
		t.Fatalf("B4 produced rows %v, want no fire", rows.batches[len(rows.batches)-1])
	}

	sendPoolA(t, engine, "A20")
	sendPoolA(t, engine, "A21")
	sendPoolB(t, engine, "B5")
	rows.requireLast(t, "a=A20 b=B5", "a=A21 b=B5")
	recorder.requireEmpty(t)
}

// TestPatternFollowedByMaxPool4PreventMatchesEsper mirrors
// PatternOperatorFollowedByMax4Prevent: pool max 4 shared by two statements,
// including the statement-level -[2]> edge limit taking precedence on S1 and
// pool slots released by undeploy.
func TestPatternFollowedByMaxPool4PreventMatchesEsper(t *testing.T) {
	env := newPoolParityEnv(t)
	engine := NewEngine(env, WithPatternSubexpressionMax(4), WithPatternSubexpressionPreventStart(true))
	defer func() { _ = engine.Close(context.Background()) }()
	poolRecorder := &poolLimitRecorder{}
	if err := engine.AddPatternRuntimeSubexpressionLimitListener(poolRecorder.listener()); err != nil {
		t.Fatal(err)
	}
	edgeRecorder := &edgeLimitRecorder{}
	if err := engine.AddPatternSubexpressionLimitHandler(edgeRecorder.listener()); err != nil {
		t.Fatal(err)
	}

	sbStream := From[patternOpBean](env, "SupportBean")
	aStream := From[patternNotA](env, "SupportBean_A")
	bStream := From[patternNotB](env, "SupportBean_B")
	theString := Field[patternOpBean, string]("theString")
	s1Pattern := PatternFrom(sbStream, "a", LikeOf(theString, Literal("A%"))).Every().
		ThenMax(2, PatternFrom(aStream, "b", Equal[string](Field[patternNotA, string]("id"), TagField[string]("a", "theString"))))
	s1Plan, err := env.Build(s1Pattern.Select(
		Alias("a", TagField[string]("a", "theString")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName("S1")))
	if err != nil {
		t.Fatal(err)
	}
	s2Pattern := PatternFrom(sbStream, "a", LikeOf(theString, Literal("B%"))).Every().
		Then(PatternFrom(bStream, "b", Equal[string](Field[patternNotB, string]("id"), TagField[string]("a", "theString"))))
	s2Plan, err := env.Build(s2Pattern.Select(
		Alias("a", TagField[string]("a", "theString")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName("S2")))
	if err != nil {
		t.Fatal(err)
	}

	// Scenario runAssertionFollowedWithMax: S1 carries the statement-level
	// -[2]> edge limit on top of the runtime pool.
	s1Deployment, err := engine.Deploy(context.Background(), s1Plan)
	if err != nil {
		t.Fatal(err)
	}
	s1Rows := &rowCollector{}
	if _, err := s1Deployment.Statements()[0].Subscribe(s1Rows.subscriber()); err != nil {
		t.Fatal(err)
	}
	s2Deployment, err := engine.Deploy(context.Background(), s2Plan)
	if err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "A1", 0)
	sendPatternOpBean(t, engine, "A2", 0)
	sendPatternOpBean(t, engine, "B1", 0)
	poolRecorder.requireEmpty(t)
	edgeRecorder.requireEmpty(t)

	// S1 hits its statement-level edge limit first; the runtime pool is not
	// consulted for the rejected spawn, matching Java's trackWithMax-first order.
	sendPatternOpBean(t, engine, "A3", 0)
	edgeRecorder.requireSingle(t, "S1", 2)
	poolRecorder.requireEmpty(t)

	sendPatternOpBean(t, engine, "B2", 0)
	poolRecorder.requireEmpty(t)

	sendPatternOpBean(t, engine, "B3", 0)
	poolRecorder.requireSingle(t, "S2", 4, map[string]int64{"S1": 2, "S2": 2})

	sendPoolA(t, engine, "A2")
	s1Rows.requireLast(t, "a=A2 b=A2")
	sendPatternOpBean(t, engine, "B4", 0)
	poolRecorder.requireEmpty(t)

	sendPatternOpBean(t, engine, "A3", 0)
	poolRecorder.requireSingle(t, "S1", 4, map[string]int64{"S1": 1, "S2": 3})
	edgeRecorder.requireEmpty(t)

	if err := s1Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "B4", 0)
	poolRecorder.requireEmpty(t)
	sendPatternOpBean(t, engine, "B5", 0)
	poolRecorder.requireSingle(t, "S2", 4, map[string]int64{"S2": 4})

	if err := s2Deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Scenario runAssertionTwoStatementsAndStopDestroy: plain followed-by on
	// both statements.
	s1Plain := PatternFrom(sbStream, "a", LikeOf(theString, Literal("A%"))).Every().
		Then(PatternFrom(aStream, "b", Equal[string](Field[patternNotA, string]("id"), TagField[string]("a", "theString"))))
	s1PlainPlan, err := env.Build(s1Plain.Select(
		Alias("a", TagField[string]("a", "theString")),
		Alias("b", TagField[string]("b", "id")),
	).Query(StatementName("S1")))
	if err != nil {
		t.Fatal(err)
	}
	s1DeploymentTwo, err := engine.Deploy(context.Background(), s1PlainPlan)
	if err != nil {
		t.Fatal(err)
	}
	s2DeploymentTwo, err := engine.Deploy(context.Background(), s2Plan)
	if err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "A1", 0)
	sendPatternOpBean(t, engine, "A2", 0)
	sendPatternOpBean(t, engine, "A3", 0)
	sendPatternOpBean(t, engine, "B1", 0)
	poolRecorder.requireEmpty(t)

	sendPatternOpBean(t, engine, "B2", 0)
	poolRecorder.requireSingle(t, "S2", 4, map[string]int64{"S1": 3, "S2": 1})

	sendPatternOpBean(t, engine, "A4", 0)
	poolRecorder.requireSingle(t, "S1", 4, map[string]int64{"S1": 3, "S2": 1})

	if err := s1DeploymentTwo.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	sendPatternOpBean(t, engine, "B3", 0)
	sendPatternOpBean(t, engine, "B4", 0)
	sendPatternOpBean(t, engine, "B5", 0)
	poolRecorder.requireEmpty(t)

	sendPatternOpBean(t, engine, "B6", 0)
	poolRecorder.requireSingle(t, "S2", 4, map[string]int64{"S2": 4})
	sendPatternOpBean(t, engine, "B7", 0)
	poolRecorder.requireSingle(t, "S2", 4, map[string]int64{"S2": 4})

	if err := s2DeploymentTwo.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestPatternSinglePermFalseAndQuitMatchesEsper mirrors
// PatternOperatorFollowedByMax.PatternSinglePermFalseAndQuit: branches that
// turn permanently false (not-operator falsified) or expire (timer:within
// guard) release their statement-level -[2]> slots, while retained every
// branches keep theirs.
func TestPatternSinglePermFalseAndQuitMatchesEsper(t *testing.T) {
	t.Run("not-operator", func(t *testing.T) {
		env := newPoolParityEnv(t)
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		recorder := &edgeLimitRecorder{}
		if err := engine.AddPatternSubexpressionLimitHandler(recorder.listener()); err != nil {
			t.Fatal(err)
		}
		aStream := From[patternNotA](env, "SupportBean_A")
		bStream := From[patternNotB](env, "SupportBean_B")
		cStream := From[patternNotC](env, "SupportBean_C")
		pattern := PatternFrom(aStream, "a", Literal(true)).Every().
			ThenMax(2, PatternFrom(bStream, "b", Literal(true)).And(PatternFrom(cStream, "c", Literal(true)).Not()))
		plan, err := env.Build(pattern.Select(
			Alias("a", TagField[string]("a", "id")),
			Alias("b", TagField[string]("b", "id")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := &rowCollector{}
		if _, err := deployment.Statements()[0].Subscribe(rows.subscriber()); err != nil {
			t.Fatal(err)
		}

		sendPoolA(t, engine, "A1")
		sendPoolA(t, engine, "A2")
		sendPoolC(t, engine, "C1")
		sendPoolA(t, engine, "A3")
		sendPoolA(t, engine, "A4")
		sendPoolB(t, engine, "B1")
		rows.requireLast(t, "a=A3 b=B1", "a=A4 b=B1")
		recorder.requireEmpty(t)

		sendPoolA(t, engine, "A5")
		sendPoolA(t, engine, "A6")
		sendPoolA(t, engine, "A7")
		recorder.requireSingle(t, "s0", 2)
	})

	t.Run("guard", func(t *testing.T) {
		env := newPoolParityEnv(t)
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		recorder := &edgeLimitRecorder{}
		if err := engine.AddPatternSubexpressionLimitHandler(recorder.listener()); err != nil {
			t.Fatal(err)
		}
		aStream := From[patternNotA](env, "SupportBean_A")
		bStream := From[patternNotB](env, "SupportBean_B")
		pattern := PatternFrom(aStream, "a", Literal(true)).Every().
			ThenMax(2, PatternFrom(bStream, "b", Literal(true)).Within(time.Second))
		plan, err := env.Build(pattern.Select(
			Alias("a", TagField[string]("a", "id")),
			Alias("b", TagField[string]("b", "id")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := &rowCollector{}
		if _, err := deployment.Statements()[0].Subscribe(rows.subscriber()); err != nil {
			t.Fatal(err)
		}

		sendPoolA(t, engine, "A1")
		sendPoolA(t, engine, "A2")
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(2000).UTC()); err != nil {
			t.Fatal(err)
		}
		recorder.requireEmpty(t)

		sendPoolA(t, engine, "A3")
		sendPoolA(t, engine, "A4")
		sendPoolB(t, engine, "B1")
		rows.requireLast(t, "a=A3 b=B1", "a=A4 b=B1")
		recorder.requireEmpty(t)

		sendPoolA(t, engine, "A5")
		sendPoolA(t, engine, "A6")
		sendPoolA(t, engine, "A7")
		recorder.requireSingle(t, "s0", 2)
	})

	t.Run("every-operator", func(t *testing.T) {
		env := newPoolParityEnv(t)
		engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
		defer func() { _ = engine.Close(context.Background()) }()
		recorder := &edgeLimitRecorder{}
		if err := engine.AddPatternSubexpressionLimitHandler(recorder.listener()); err != nil {
			t.Fatal(err)
		}
		aStream := From[patternNotA](env, "SupportBean_A")
		bStream := From[patternNotB](env, "SupportBean_B")
		cStream := From[patternNotC](env, "SupportBean_C")
		correlated := PatternFrom(bStream, "b", Equal[string](Field[patternNotB, string]("id"), TagField[string]("a", "id"))).Every().
			And(PatternFrom(cStream, "c", Equal[string](Field[patternNotC, string]("id"), TagField[string]("a", "id"))).Not())
		pattern := PatternFrom(aStream, "a", Literal(true)).Every().ThenMax(2, correlated)
		plan, err := env.Build(pattern.Select(
			Alias("a", TagField[string]("a", "id")),
			Alias("b", TagField[string]("b", "id")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := &rowCollector{}
		if _, err := deployment.Statements()[0].Subscribe(rows.subscriber()); err != nil {
			t.Fatal(err)
		}

		sendPoolA(t, engine, "1")
		sendPoolA(t, engine, "2")
		sendPoolB(t, engine, "1")
		rows.requireLast(t, "a=1 b=1")
		sendPoolB(t, engine, "2")
		rows.requireLast(t, "a=2 b=2")
		sendPoolC(t, engine, "1")
		sendPoolA(t, engine, "3")
		sendPoolB(t, engine, "3")
		rows.requireLast(t, "a=3 b=3")
		recorder.requireEmpty(t)
	})
}

// TestPatternFollowedByMaxInvalidCasesMatchEsper mirrors
// PatternOperatorFollowedByMax.PatternOperatorFollowedByMaxInvalid: the
// followed-by maximum expression rejects event-property references and
// non-integer results at Build time.
func TestPatternFollowedByMaxInvalidCasesMatchEsper(t *testing.T) {
	env := newPoolParityEnv(t)
	aStream := From[patternNotA](env, "SupportBean_A")
	sbStream := From[patternOpBean](env, "SupportBean")

	// Java: a=SupportBean_A -[a.intPrimitive]> SupportBean_B is rejected
	// because event properties are not allowed in the maximum expression.
	fieldMax := PatternFrom(sbStream, "a", Literal(true)).FollowedByMaxExpr(
		TagField[int]("a", "intPrimitive"), "b", Literal(true))
	if _, err := env.Build(fieldMax.Select(Alias("a", TagField[string]("a", "id"))).Query()); err == nil {
		t.Fatal("followed-by maximum referencing an event property was accepted")
	} else if !strings.Contains(err.Error(), "cannot reference") {
		t.Fatalf("field maximum error = %v, want reference rejection", err)
	}

	// Java: a=SupportBean_A -[false]> SupportBean_B is rejected because the
	// maximum expression must return an integer value.
	boolMax := PatternFrom(aStream, "a", Literal(true)).FollowedByMaxExpr(
		Literal(false), "b", Literal(true))
	if _, err := env.Build(boolMax.Select(Alias("a", TagField[string]("a", "id"))).Query()); err == nil {
		t.Fatal("non-integer followed-by maximum was accepted")
	} else if !strings.Contains(err.Error(), "must return int") {
		t.Fatalf("boolean maximum error = %v, want integer-type rejection", err)
	}
}
