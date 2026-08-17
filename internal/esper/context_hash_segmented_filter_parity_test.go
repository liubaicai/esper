package esper

import (
	"context"
	"testing"
)

// TestContextHashSegmentedMultiParity locks the multi-stream hash context
// join verified against ContextHashSegmentedMulti. The Java test uses
// `coalesce consistent_hash_crc32(theString) from SupportBean,
// consistent_hash_crc32(p00) from SupportBean_S0 granularity 4
// preallocate` with a keepall join. The Go API does not support
// per-stream key association in hash contexts, so this test verifies
// the core hash context + join behavior with a single-stream key.
func TestContextHashSegmentedMultiParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[hashMultiBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[hashMultiS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	sbKey := Field[hashMultiBean, string]("theString")
	if _, err := CreatePreallocatedHashContextWithAlgorithm(
		env, "HashSegmentedContext", HashAlgorithmCRC32, 4, sbKey,
	); err != nil {
		t.Fatal(err)
	}
	sb := From[hashMultiBean](env, "SupportBean").Window(KeepAll())
	s0 := From[hashMultiS0](env, "SupportBean_S0").Window(KeepAll())
	plan, err := env.Build(Join(
		sb, s0,
		OnEqual(
			Field[hashMultiBean, string]("theString"),
			Field[hashMultiS0, string]("p00"),
		),
	).Select(
		SelectFrom(0, "c0", ContextName()),
		SelectFrom(0, "c1", Field[hashMultiBean, int]("intPrimitive")),
		SelectFrom(1, "c2", Field[hashMultiS0, int]("id")),
	).Query(StatementName("s0"), WithContext("HashSegmentedContext")))
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
		c0 string
		c1 int
		c2 int
	}
	var rows []row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if rowValue, ok := result.Row(); ok {
				rows = append(rows, row{
					c0: rowValue.Get("c0").Any().(string),
					c1: rowValue.Get("c1").Any().(int),
					c2: rowValue.Get("c2").Any().(int),
				})
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendSB := func(s string, i int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), hashMultiBean{TheString: s, IntPrimitive: i}); err != nil {
			t.Fatal(err)
		}
	}
	sendS0 := func(id int, p00 string) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), hashMultiS0{ID: id, P00: p00}); err != nil {
			t.Fatal(err)
		}
	}
	sendSB("E1", 10)
	sendS0(1, "E2")
	sendSB("E3", 11)
	sendS0(2, "E4")
	// No join yet (E1≠E2, E3≠E4)
	if len(rows) != 0 {
		t.Fatalf("after initial events: rows = %d, want 0: %#v", len(rows), rows)
	}
	sendS0(3, "E1") // joins with E1 (theString=E1, p00=E1)
	if len(rows) != 1 || rows[0].c1 != 10 || rows[0].c2 != 3 {
		t.Fatalf("after S0(3,E1): rows = %#v, want [{HashSegmentedContext 10 3}]", rows)
	}
	sendS0(4, "E4")
	sendS0(5, "E5")
	if len(rows) != 1 {
		t.Fatalf("after S0(4,E4)+S0(5,E5): rows = %d, want 1", len(rows))
	}
	sendSB("E2", 12) // joins with S0(1,"E2")
	if len(rows) != 2 {
		t.Fatalf("after E2/12: rows = %d, want 2: %#v", len(rows), rows)
	}
	if rows[1].c1 != 12 || rows[1].c2 != 1 {
		t.Fatalf("rows[1] = %#v, want {HashSegmentedContext 12 1}", rows[1])
	}
}

type hashMultiBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type hashMultiS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}
