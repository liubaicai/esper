package esper

import (
	"context"
	"strings"
	"testing"
)

type contextHashStreamsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextHashStreamsS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// TestHashContextByStreamsStatementTypeValidation mirrors Esper's segmented
// statement rule for hash contexts that list event types explicitly
// (`coalesce hash_code(intPrimitive) from SupportBean granularity 10`): a
// statement binding the context over an unlisted type must fail to build,
// while the listed type builds. Prior single-type hash constructors declared
// no stream set and skipped this validation entirely.
func TestHashContextByStreamsStatementTypeValidation(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextHashStreamsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[contextHashStreamsS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateHashContextByStreams(env, "ACtx", HashAlgorithmJavaHashCode, 10,
		KeyContextStream{Type: "SupportBean", Keys: []Expr{Field[contextHashStreamsBean, int]("intPrimitive")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.Build(From[contextHashStreamsBean](env, "SupportBean").Query(
		StatementName("s0"), WithContext("ACtx"))); err != nil {
		t.Fatalf("listed-type statement should build: %v", err)
	}
	_, err := env.Build(From[contextHashStreamsS0](env, "SupportBean_S0").Query(
		StatementName("s1"), WithContext("ACtx")))
	if err == nil {
		t.Fatal("unlisted-type statement unexpectedly built")
	}
	if !strings.Contains(err.Error(), "requires that any of the event types that are listed in the segmented context") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHashContextByStreamsPartitioning verifies events of listed types
// partition by that stream's key expressions while the hash context stays
// lazy: identical keys share a partition, distinct keys may land apart, and
// the aggregate sees per-key rows. This pins that streamKeys routing reaches
// the same observable bucket assignment as the single-type form.
func TestHashContextByStreamsPartitioning(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextHashStreamsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateHashContextByStreams(env, "ACtx", HashAlgorithmCRC32, 8,
		KeyContextStream{Type: "SupportBean", Keys: []Expr{Field[contextHashStreamsBean, string]("theString")}}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextHashStreamsBean](env, "SupportBean").
		GroupBy(Field[contextHashStreamsBean, string]("theString")).Select(
		Alias("c0", Field[contextHashStreamsBean, string]("theString")),
		Alias("c1", Sum[int](Field[contextHashStreamsBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("ACtx")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var last []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		last = nil
		for _, item := range batch.New {
			row, _ := item.Row()
			last = append(last, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []contextHashStreamsBean{{"E1", 1}, {"E2", 2}, {"E1", 3}} {
		if err := engine.Send(context.Background(), "SupportBean", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(last) != 1 || last[0].Get("c0").Any() != "E1" || last[0].Get("c1").Any() != 4 {
		t.Fatalf("grouped rows = %#v, want single E1 row summing to 4", last)
	}
}

// TestHashContextByStreamsStreamFilter mirrors Esper's
// `coalesce <func>(k) from T(filter)`: events failing the declared stream
// filter neither allocate a partition nor reach statements.
func TestHashContextByStreamsStreamFilter(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextHashStreamsBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateHashContextByStreams(env, "ACtx", HashAlgorithmCRC32, 8,
		KeyContextStream{
			Type:   "SupportBean",
			Keys:   []Expr{Field[contextHashStreamsBean, string]("theString")},
			Filter: Greater[int](Field[contextHashStreamsBean, int]("intPrimitive"), Literal(10)),
		}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextHashStreamsBean](env, "SupportBean").
		GroupBy(Field[contextHashStreamsBean, string]("theString")).Select(
		Alias("c0", Field[contextHashStreamsBean, string]("theString")),
		Alias("c1", Field[contextHashStreamsBean, int]("intPrimitive")),
	).Query(StatementName("s0"), WithContext("ACtx")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, item := range batch.New {
			row, _ := item.Row()
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []contextHashStreamsBean{{"E1", 1}, {"E2", 11}, {"E3", 5}} {
		if err := engine.Send(context.Background(), "SupportBean", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 1 || rows[0].Get("c0").Any() != "E2" {
		t.Fatalf("filtered rows = %#v, want only the E2 row", rows)
	}
}

// TestHashContextByStreamsNamedWindowRejected mirrors Esper's "Partition
// criteria may not include named windows" for the new stream-listing
// constructor: validation reuses validateKeyContextStreams.
func TestHashContextByStreamsNamedWindowRejected(t *testing.T) {
	env := NewEnvironment()
	beanSchema, err := RegisterStruct[contextHashStreamsBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MyWindow", beanSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	_, err = CreateHashContextByStreams(env, "SegmentedByWhat", HashAlgorithmFNV1a, 10,
		KeyContextStream{Type: "MyWindow", Keys: []Expr{Field[contextHashStreamsBean, string]("theString")}})
	if err == nil {
		t.Fatal("named-window partition type unexpectedly accepted")
	}
	if !strings.Contains(err.Error(), "partition criteria may not include named windows") {
		t.Fatalf("unexpected error: %v", err)
	}
}
