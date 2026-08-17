package esper

import (
	"context"
	"reflect"
	"testing"
)

// Parity coverage for the istream/rstream keyword executions of
// EPLOtherIStreamRStreamKeywords. OM/Compile variants
// (EPLOtherRStreamOnlyOM/EPLOtherRStreamOnlyCompile) are compile-API
// duplicates of EPLOtherRStreamOnly and are not separately sliced;
// EPLOtherRStreamOutputSnapshot is a compile-deploy-undeploy smoke test
// requiring time(30 minutes) output snapshot, excluded.
//
// - EPLOtherRStreamOnly: select rstream * over length(3)
// - EPLOtherRStreamInsertInto: insert into strips rstream selector
// - EPLOtherRStreamInsertIntoRStream: insert rstream into forwards removals
// - EPLOtherRStreamJoin: rstream join fires on window expiry
// - EPLOtherIStreamOnly: select istream * over length(1)
// - EPLOtherIStreamInsertIntoRStream: insert rstream into over istream source
// - EPLOtherIStreamJoin: istream join fires on match, silent on expiry

type irStreamBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newIRStreamEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[irStreamBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env
}

type irCapture struct {
	newRows []Result
	oldRows []Result
}

func irSubscribe(t *testing.T, deployment *Deployment, index int) *irCapture {
	t.Helper()
	capture := &irCapture{}
	_, err := deployment.Statements()[index].Subscribe(func(_ context.Context, batch ResultBatch) error {
		capture.newRows = append(capture.newRows, batch.New...)
		capture.oldRows = append(capture.oldRows, batch.Old...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return capture
}

func irSend(t *testing.T, engine *Engine, theString string, intPrimitive int) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), irStreamBean{TheString: theString, IntPrimitive: intPrimitive}); err != nil {
		t.Fatal(err)
	}
}

func irRegisterNextStream(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterMap(env, "NextStream", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
	}); err != nil {
		t.Fatal(err)
	}
}

// TestEPLOtherRStreamOnlyParity covers EPLOtherRStreamOnly:
// select rstream * from SupportBean#length(3). No output until the 4th event
// expires the first ('a'); that batch delivers the expired event as new data
// with no old data. Java runtime: java-runtime-d268f6c45cd4375e9c1c.
func TestEPLOtherRStreamOnlyParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	plan, err := env.Build(
		From[irStreamBean](env, "SupportBean").Window(LengthWindow(3)).
			Query(StatementName("s0"), WithRemoveStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	got := irSubscribe(t, deployment, 0)

	irSend(t, engine, "a", 2)
	if len(got.newRows) != 0 {
		t.Fatalf("rstream fired on insert: %#v", got.newRows)
	}
	irSend(t, engine, "a", 2)
	irSend(t, engine, "b", 2)
	if len(got.newRows) != 0 {
		t.Fatalf("rstream fired before expiry: %#v", got.newRows)
	}
	irSend(t, engine, "d", 2)
	if len(got.newRows) != 1 {
		t.Fatalf("expected 1 rstream row, got %d", len(got.newRows))
	}
	if got.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("rstream row theString = %#v", got.newRows[0].Get("theString").Any())
	}
	if len(got.oldRows) != 0 {
		t.Fatalf("expected no old rows, got %#v", got.oldRows)
	}
}

// TestEPLOtherRStreamInsertIntoParity covers EPLOtherRStreamInsertInto:
// insert into NextStream select rstream ... — the insert-into without the
// rstream keyword strips the selector; the downstream consumer sees all
// inserts (a, b, c, d), while the s0 listener only fires on expiry.
// Java runtime: java-runtime-0e4714c5a90f9e774235.
func TestEPLOtherRStreamInsertIntoParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	irRegisterNextStream(t, env)
	// Java models one statement whose select-rstream feeds the s0 listener
	// while the plain insert-into (no rstream keyword) routes all inserts to
	// the target. Go's InsertInto shares a single selector between routing
	// and dispatch, so the equivalent Go plan decomposes into two statements
	// over the same windowed source: s0 (remove-stream-only listener) and
	// s0-route (default istream insert-into routing all inserts).
	s0Plan, err := env.Build(
		Select(
			From[irStreamBean](env, "SupportBean").Window(LengthWindow(3)),
			Alias("theString", Field[irStreamBean, string]("theString")),
		).Query(StatementName("s0"), WithRemoveStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(
		Select(
			From[irStreamBean](env, "SupportBean").Window(LengthWindow(3)),
			Alias("theString", Field[irStreamBean, string]("theString")),
		).InsertInto("NextStream", StatementName("s0-route")),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "NextStream").Query(StatementName("ii")))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	s0Got := irSubscribe(t, s0Deployment, 0)
	consumerGot := irSubscribe(t, consumerDeployment, 0)

	// First event: s0 silent (rstream), consumer sees insert "a"
	irSend(t, engine, "a", 2)
	if len(s0Got.newRows) != 0 {
		t.Fatalf("s0 fired on insert: %#v", s0Got.newRows)
	}
	if len(consumerGot.newRows) != 1 || consumerGot.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("consumer = %#v", consumerGot.newRows)
	}

	// Events b, c: s0 silent, consumer sees both inserts
	irSend(t, engine, "b", 2)
	irSend(t, engine, "c", 2)
	if len(s0Got.newRows) != 0 {
		t.Fatalf("s0 fired before expiry: %#v", s0Got.newRows)
	}
	if len(consumerGot.newRows) != 3 {
		t.Fatalf("expected 3 consumer rows, got %d", len(consumerGot.newRows))
	}

	// Event d: window expires 'a' — s0 fires with 'a'; consumer sees insert 'd'
	irSend(t, engine, "d", 2)
	if len(s0Got.newRows) != 1 || s0Got.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("s0 rows = %#v", s0Got.newRows)
	}
	if len(s0Got.oldRows) != 0 {
		t.Fatalf("s0 old rows = %#v", s0Got.oldRows)
	}
	if len(consumerGot.newRows) != 4 || consumerGot.newRows[3].Get("theString").Any() != "d" {
		t.Fatalf("consumer rows = %#v", consumerGot.newRows)
	}
	if len(consumerGot.oldRows) != 0 {
		t.Fatalf("consumer old rows = %#v", consumerGot.oldRows)
	}
}

// TestEPLOtherRStreamInsertIntoRStreamParity covers
// EPLOtherRStreamInsertIntoRStream: insert rstream into NextStream select
// rstream ... — only removals are forwarded to the downstream consumer.
// Java runtime: java-runtime-26cbb822fec0dd23e976.
func TestEPLOtherRStreamInsertIntoRStreamParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	irRegisterNextStream(t, env)
	s0Plan, err := env.Build(
		Select(
			From[irStreamBean](env, "SupportBean").Window(LengthWindow(3)),
			Alias("theString", Field[irStreamBean, string]("theString")),
		).InsertInto("NextStream", StatementName("s0"), WithRemoveStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "NextStream").Query(StatementName("ii")))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	s0Got := irSubscribe(t, s0Deployment, 0)
	consumerGot := irSubscribe(t, consumerDeployment, 0)

	irSend(t, engine, "a", 2)
	if len(s0Got.newRows) != 0 || len(consumerGot.newRows) != 0 {
		t.Fatalf("s0=%#v consumer=%#v", s0Got.newRows, consumerGot.newRows)
	}
	irSend(t, engine, "b", 2)
	irSend(t, engine, "c", 2)
	if len(s0Got.newRows) != 0 || len(consumerGot.newRows) != 0 {
		t.Fatalf("s0=%#v consumer=%#v", s0Got.newRows, consumerGot.newRows)
	}
	irSend(t, engine, "d", 2)
	if len(s0Got.newRows) != 1 || s0Got.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("s0 rows = %#v", s0Got.newRows)
	}
	if len(consumerGot.newRows) != 1 || consumerGot.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("consumer rows = %#v", consumerGot.newRows)
	}
}

// TestEPLOtherRStreamJoinParity covers EPLOtherRStreamJoin: rstream join of
// SupportBean('a')#length(2) with SupportBean('b')#keepall on intPrimitive.
// Silent until the 3rd 'a' event expires the first 'a' — then (aID=1,bID=1).
// Java runtime: java-runtime-2fd326ba46db2a7e03c4.
func TestEPLOtherRStreamJoinParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	s1 := From[irStreamBean](env, "SupportBean").Filter(
		Equal[string](Field[irStreamBean, string]("theString"), Literal("a")),
	).Window(LengthWindow(2))
	s2 := From[irStreamBean](env, "SupportBean").Filter(
		Equal[string](Field[irStreamBean, string]("theString"), Literal("b")),
	).Window(KeepAll())
	plan, err := env.Build(
		Join(s1, s2, OnEqual(
			Field[irStreamBean, int]("intPrimitive"),
			Field[irStreamBean, int]("intPrimitive"),
		)).Select(
			SelectFrom(0, "aID", Field[irStreamBean, int]("intPrimitive")),
			SelectFrom(1, "bID", Field[irStreamBean, int]("intPrimitive")),
		).Query(StatementName("s0"), WithRemoveStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	got := irSubscribe(t, deployment, 0)

	irSend(t, engine, "a", 1)
	irSend(t, engine, "b", 1)
	if len(got.newRows) != 0 {
		t.Fatalf("rstream join fired on match: %#v", got.newRows)
	}
	irSend(t, engine, "a", 2)
	if len(got.newRows) != 0 {
		t.Fatalf("rstream join fired early: %#v", got.newRows)
	}
	irSend(t, engine, "a", 3)
	if len(got.newRows) != 1 {
		t.Fatalf("expected 1 rstream join row, got %d", len(got.newRows))
	}
	if got.newRows[0].Get("aID").Any() != 1 || got.newRows[0].Get("bID").Any() != 1 {
		t.Fatalf("rstream join row aID=%#v bID=%#v", got.newRows[0].Get("aID").Any(), got.newRows[0].Get("bID").Any())
	}
	if len(got.oldRows) != 0 {
		t.Fatalf("expected no old rows, got %#v", got.oldRows)
	}
}

// TestEPLOtherIStreamOnlyParity covers EPLOtherIStreamOnly:
// select istream * from SupportBean#length(1). Each insert fires as new data;
// the length(1) expiry is suppressed. Java runtime: java-runtime-490347196f7e74a39fd0.
func TestEPLOtherIStreamOnlyParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	plan, err := env.Build(
		From[irStreamBean](env, "SupportBean").Window(LengthWindow(1)).
			Query(StatementName("s0"), WithNewStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	got := irSubscribe(t, deployment, 0)

	irSend(t, engine, "a", 2)
	if len(got.newRows) != 1 || got.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("istream rows = %#v", got.newRows)
	}
	irSend(t, engine, "b", 2)
	if len(got.newRows) != 2 || got.newRows[1].Get("theString").Any() != "b" {
		t.Fatalf("istream rows = %#v", got.newRows)
	}
	if len(got.oldRows) != 0 {
		t.Fatalf("expected no old rows, got %#v", got.oldRows)
	}
}

// TestEPLOtherIStreamInsertIntoRStreamParity covers
// EPLOtherIStreamInsertIntoRStream: insert rstream into NextStream select
// istream ... — the source is istream-only over length(1); the insert
// rstream into forwards only the expiries generated by the window, so the
// consumer sees the expired 'a' after the second event arrives.
// Java runtime: java-runtime-bdda0ee0cc72ee5d18a1.
func TestEPLOtherIStreamInsertIntoRStreamParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	irRegisterNextStream(t, env)
	// Java models one statement whose select-istream feeds the s0 listener
	// while insert rstream into routes only removals to the target. Go's
	// InsertInto shares a single selector between routing and dispatch, so
	// the equivalent Go plan decomposes into two statements over the same
	// windowed source: s0 (new-stream-only listener) and s0-route
	// (remove-stream-only insert-into routing expiries).
	s0Plan, err := env.Build(
		Select(
			From[irStreamBean](env, "SupportBean").Window(LengthWindow(1)),
			Alias("theString", Field[irStreamBean, string]("theString")),
		).Query(StatementName("s0"), WithNewStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(
		Select(
			From[irStreamBean](env, "SupportBean").Window(LengthWindow(1)),
			Alias("theString", Field[irStreamBean, string]("theString")),
		).InsertInto("NextStream", StatementName("s0-route"), WithRemoveStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(FromAny(env, "NextStream").Query(StatementName("ii")))
	if err != nil {
		t.Fatal(err)
	}
	s0Deployment, err := engine.Deploy(context.Background(), s0Plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	s0Got := irSubscribe(t, s0Deployment, 0)
	consumerGot := irSubscribe(t, consumerDeployment, 0)

	// Event a: s0 istream fires "a"; insert rstream into does NOT forward insert
	irSend(t, engine, "a", 2)
	if len(s0Got.newRows) != 1 || s0Got.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("s0 rows = %#v", s0Got.newRows)
	}
	if len(consumerGot.newRows) != 0 {
		t.Fatalf("consumer fired on insert: %#v", consumerGot.newRows)
	}

	// Event b: s0 istream fires "b" (old suppressed); the expiry of 'a' is
	// forwarded to the consumer via insert rstream into.
	irSend(t, engine, "b", 2)
	if len(s0Got.newRows) != 2 || s0Got.newRows[1].Get("theString").Any() != "b" {
		t.Fatalf("s0 rows = %#v", s0Got.newRows)
	}
	if len(s0Got.oldRows) != 0 {
		t.Fatalf("s0 old rows = %#v", s0Got.oldRows)
	}
	if len(consumerGot.newRows) != 1 || consumerGot.newRows[0].Get("theString").Any() != "a" {
		t.Fatalf("consumer rows = %#v", consumerGot.newRows)
	}
}

// TestEPLOtherIStreamJoinParity covers EPLOtherIStreamJoin: istream join of
// SupportBean('a')#length(2) with SupportBean('b')#keepall on intPrimitive.
// Fires once when 'b' matches the retained 'a'; later 'a' arrivals and the
// length(2) expiry stay silent. Java runtime: java-runtime-ccc0bd023a72659c1a43.
func TestEPLOtherIStreamJoinParity(t *testing.T) {
	env := newIRStreamEnv(t)
	engine := NewEngine(env)
	s1 := From[irStreamBean](env, "SupportBean").Filter(
		Equal[string](Field[irStreamBean, string]("theString"), Literal("a")),
	).Window(LengthWindow(2))
	s2 := From[irStreamBean](env, "SupportBean").Filter(
		Equal[string](Field[irStreamBean, string]("theString"), Literal("b")),
	).Window(KeepAll())
	plan, err := env.Build(
		Join(s1, s2, OnEqual(
			Field[irStreamBean, int]("intPrimitive"),
			Field[irStreamBean, int]("intPrimitive"),
		)).Select(
			SelectFrom(0, "aID", Field[irStreamBean, int]("intPrimitive")),
			SelectFrom(1, "bID", Field[irStreamBean, int]("intPrimitive")),
		).Query(StatementName("s0"), WithNewStreamOnly()),
	)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(context.Background())
	got := irSubscribe(t, deployment, 0)

	irSend(t, engine, "a", 1)
	irSend(t, engine, "b", 1)
	if len(got.newRows) != 1 {
		t.Fatalf("expected 1 istream join row, got %d", len(got.newRows))
	}
	if got.newRows[0].Get("aID").Any() != 1 || got.newRows[0].Get("bID").Any() != 1 {
		t.Fatalf("istream join row aID=%#v bID=%#v", got.newRows[0].Get("aID").Any(), got.newRows[0].Get("bID").Any())
	}
	if len(got.oldRows) != 0 {
		t.Fatalf("expected no old rows, got %#v", got.oldRows)
	}
	irSend(t, engine, "a", 2)
	if len(got.newRows) != 1 {
		t.Fatalf("istream join fired again: %#v", got.newRows)
	}
	irSend(t, engine, "a", 3)
	if len(got.newRows) != 1 {
		t.Fatalf("istream join fired on expiry: %#v", got.newRows)
	}
}
