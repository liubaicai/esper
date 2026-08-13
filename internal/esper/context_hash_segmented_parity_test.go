package esper

import (
	"context"
	"reflect"
	"testing"
)

type contextHashSegmentedParityBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type contextHashSegmentedParityS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func contextHashSegmentedParityRows(t *testing.T, result QueryResult) []Row {
	t.Helper()
	rows := make([]Row, 0, len(result.Results()))
	for _, item := range result.Results() {
		row, ok := item.Row()
		if !ok {
			t.Fatalf("context hash snapshot result is not a row: %#v", item)
		}
		rows = append(rows, row)
	}
	return rows
}

func assertContextHashSegmentedRows(t *testing.T, rows []Row, want [][]any) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("context hash rows = %#v, want %#v", rows, want)
	}
	remaining := append([][]any(nil), want...)
	for _, row := range rows {
		got := []any{row.Get("c0").Any(), row.Get("c1").Any(), row.Get("c2").Any()}
		found := -1
		for index, expected := range remaining {
			if reflect.DeepEqual(got, expected) {
				found = index
				break
			}
		}
		if found < 0 {
			t.Fatalf("unexpected context hash row %#v; remaining %#v", got, remaining)
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	if len(remaining) != 0 {
		t.Fatalf("missing context hash rows %#v", remaining)
	}
}

func TestContextHashNoPreallocateParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextHashSegmentedParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	definition, err := CreateHashContextWithAlgorithm(
		env,
		"CtxHash",
		HashAlgorithmCRC32,
		16,
		Field[contextHashSegmentedParityBean, string]("theString"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Preallocate() || definition.HashAlgorithm() != HashAlgorithmCRC32 {
		t.Fatalf("lazy CRC32 hash definition = %#v", definition)
	}

	key := Field[contextHashSegmentedParityBean, string]("theString")
	plan, err := env.Build(From[contextHashSegmentedParityBean](env, "SupportBean").GroupBy(key).Select(
		Alias("c0", ContextID()),
		Alias("c1", key),
		Alias("c2", Sum[int](Field[contextHashSegmentedParityBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("CtxHash")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	statement := deployment.Statements()[0]
	var rows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, item := range batch.New {
			row, ok := item.Row()
			if !ok {
				t.Fatalf("context hash listener result is not a row: %#v", item)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []contextHashSegmentedParityBean{
		{TheString: "E1", IntPrimitive: 10},
		{TheString: "E2", IntPrimitive: 11},
		{TheString: "E2", IntPrimitive: 12},
		{TheString: "E1", IntPrimitive: 14},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	assertContextHashSegmentedRows(t, rows, [][]any{
		{0, "E1", 10},
		{1, "E2", 11},
		{1, "E2", 23},
		{0, "E1", 24},
	})
	if got := statement.ContextPartitionCount(); got != 2 {
		t.Fatalf("lazy hash partition count = %d, want 2", got)
	}
	for _, descriptor := range statement.ContextPartitions() {
		hash, ok := descriptor.Property("hash")
		if !ok || hash.State() != ValuePresent {
			t.Fatalf("lazy hash descriptor = %#v", descriptor.Properties())
		}
	}
}

func TestContextHashPartitionSelectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextHashSegmentedParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	definition, err := CreatePreallocatedHashContextWithAlgorithm(
		env,
		"MyCtx",
		HashAlgorithmCRC32,
		16,
		Field[contextHashSegmentedParityBean, string]("theString"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !definition.Preallocate() {
		t.Fatal("hash partition selection context is not preallocated")
	}
	key := Field[contextHashSegmentedParityBean, string]("theString")
	plan, err := env.Build(From[contextHashSegmentedParityBean](env, "SupportBean").Window(KeepAll()).GroupBy(key).Select(
		Alias("c0", ContextID()),
		Alias("c1", key),
		Alias("c2", Sum[int](Field[contextHashSegmentedParityBean, int]("intPrimitive"))),
	).Query(StatementName("s0"), WithContext("MyCtx")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	statement := deployment.Statements()[0]
	for _, event := range []contextHashSegmentedParityBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: 10},
		{TheString: "E1", IntPrimitive: 2},
		{TheString: "E3", IntPrimitive: 100},
		{TheString: "E3", IntPrimitive: 101},
		{TheString: "E1", IntPrimitive: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	all, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertContextHashSegmentedRows(t, contextHashSegmentedParityRows(t, all), [][]any{
		{5, "E1", 6},
		{15, "E2", 10},
		{9, "E3", 201},
	})
	if got := statement.ContextPartitionCount(); got != 16 {
		t.Fatalf("preallocated hash partition count = %d, want 16", got)
	}

	selected, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitionHashes(15))
	if err != nil {
		t.Fatal(err)
	}
	assertContextHashSegmentedRows(t, contextHashSegmentedParityRows(t, selected), [][]any{{15, "E2", 10}})
	selected, err = statement.SnapshotWithSelector(context.Background(), SelectContextPartitionHashes(1, 9, 5))
	if err != nil {
		t.Fatal(err)
	}
	assertContextHashSegmentedRows(t, contextHashSegmentedParityRows(t, selected), [][]any{
		{5, "E1", 6},
		{9, "E3", 201},
	})
	filtered, err := statement.SnapshotWithSelector(context.Background(), ContextPartitionSelectorDescriptorFunc(func(descriptor ContextPartitionDescriptor) bool {
		hash, ok := descriptor.Property("hash")
		return ok && hash.Any() == int64(15)
	}))
	if err != nil {
		t.Fatal(err)
	}
	assertContextHashSegmentedRows(t, contextHashSegmentedParityRows(t, filtered), [][]any{{15, "E2", 10}})

	for name, selector := range map[string]ContextPartitionSelector{
		"empty":   SelectContextPartitionHashes(),
		"unknown": SelectContextPartitionHashes(99),
	} {
		result, selectorErr := statement.SnapshotWithSelector(context.Background(), selector)
		if selectorErr != nil {
			t.Fatalf("%s selector: %v", name, selectorErr)
		}
		if rows := contextHashSegmentedParityRows(t, result); len(rows) != 0 {
			t.Fatalf("%s selector rows = %#v, want empty", name, rows)
		}
	}
}

func TestContextHashSegmentedManyArgParity(t *testing.T) {
	for _, test := range []struct {
		name      string
		algorithm HashAlgorithm
		wantHash  int64
	}{
		{name: "consistent-hash-crc32", algorithm: HashAlgorithmCRC32, wantHash: 550184},
		{name: "hash-code", algorithm: HashAlgorithmJavaHashCode, wantHash: 67928},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[contextHashSegmentedParityBean](env, "SupportBean"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[contextHashSegmentedParityS0](env, "SupportBean_S0"); err != nil {
				t.Fatal(err)
			}
			if _, err := CreateHashContextWithAlgorithm(
				env,
				"Ctx1",
				test.algorithm,
				1000000,
				Field[contextHashSegmentedParityBean, string]("theString"),
				Field[contextHashSegmentedParityBean, int]("intPrimitive"),
			); err != nil {
				t.Fatal(err)
			}
			inner := From[contextHashSegmentedParityS0](env, "SupportBean_S0").Window(LengthWindow(2)).AsRecord()
			longValue := Field[contextHashSegmentedParityBean, int64]("longPrimitive")
			plan, err := env.Build(From[contextHashSegmentedParityBean](env, "SupportBean").Window(LengthWindow(3)).Aggregate(
				Alias("c1", Field[contextHashSegmentedParityBean, int]("intPrimitive")),
				Alias("c2", Sum[int64](longValue)),
				Alias("c3", Prev[int64](1, longValue)),
				Alias("c4", Prior[int64](0, longValue)),
				Alias("c5", SubqueryValue[string](inner, Field[contextHashSegmentedParityS0, string]("p00"))),
			).Query(StatementName("s0"), WithContext("Ctx1")))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = deployment.Undeploy(context.Background()) }()
			statement := deployment.Statements()[0]
			var rows []Row
			if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, item := range batch.New {
					row, ok := item.Row()
					if !ok {
						t.Fatalf("many-arg result is not a row: %#v", item)
					}
					rows = append(rows, row)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			send := func(event contextHashSegmentedParityBean) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			send(contextHashSegmentedParityBean{TheString: "E1", IntPrimitive: 100, LongPrimitive: 20})
			if len(rows) != 1 || rows[0].Get("c1").Any() != 100 || rows[0].Get("c2").Any() != int64(20) || !rows[0].Get("c3").IsNull() || !rows[0].Get("c4").IsNull() || !rows[0].Get("c5").IsNull() {
				t.Fatalf("many-arg first row = %#v", rows[0].AsMap())
			}
			descriptors := statement.ContextPartitions()
			if len(descriptors) != 1 {
				t.Fatalf("many-arg descriptors = %#v", descriptors)
			}
			hash, ok := descriptors[0].Property("hash")
			if !ok || hash.Any() != test.wantHash {
				t.Fatalf("many-arg hash = %#v, want %d", hash, test.wantHash)
			}
			send(contextHashSegmentedParityBean{TheString: "E1", IntPrimitive: 100, LongPrimitive: 21})
			if len(rows) != 2 || rows[1].Get("c2").Any() != int64(41) || rows[1].Get("c3").Any() != int64(20) || rows[1].Get("c4").Any() != int64(20) || !rows[1].Get("c5").IsNull() {
				t.Fatalf("many-arg second row = %#v", rows[1].AsMap())
			}
			if err := engine.SendEvent(context.Background(), contextHashSegmentedParityS0{ID: 1000, P00: "S0"}); err != nil {
				t.Fatal(err)
			}
			send(contextHashSegmentedParityBean{TheString: "E1", IntPrimitive: 100, LongPrimitive: 22})
			if len(rows) != 3 || rows[2].Get("c2").Any() != int64(63) || rows[2].Get("c3").Any() != int64(21) || rows[2].Get("c4").Any() != int64(21) || rows[2].Get("c5").Any() != "S0" {
				t.Fatalf("many-arg third row = %#v", rows[2].AsMap())
			}
			if got := statement.ContextPartitionCount(); got != 1 {
				t.Fatalf("many-arg context partition count = %d, want 1", got)
			}
		})
	}
}
