package esper

import (
	"context"
	"reflect"
	"testing"
)

type resultsetRowLimitBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetRowLimitContextStart struct {
	ID int `esper:"id"`
}

type resultsetRowLimitContextEnd struct {
	ID int `esper:"id"`
}

type resultsetRowLimitGroupedBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// TestResultSetOutputLimitRowLimitContextAndBatchParity covers
// ResultSetLimitOneWithOrderOptimization. The batch statements retain the
// complete input batch until its boundary, then sort before applying limit 1.
// The context statements use keepall state and emit only at S1 termination;
// each new S0/S1 cycle must start from an empty partition.
func TestResultSetOutputLimitRowLimitContextAndBatchParity(t *testing.T) {
	t.Run("batch-single-key", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetRowLimitBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()

		theString := Field[resultsetRowLimitBean, string]("theString")
		plan, err := env.Build(Select(
			From[resultsetRowLimitBean](env, "SupportBean").Window(LengthBatch(10)),
			Alias("theString", theString),
		).Query(
			StatementName("s0"),
			OrderBy(Ascending(ResultField[string]("theString"))),
			Limit(1),
		))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		var batches []ResultBatch
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		send := func(value string) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitBean{TheString: value}); err != nil {
				t.Fatal(err)
			}
		}
		sequences := []struct {
			want     string
			boundary int
			values   []string
		}{
			{"A", 10, []string{"F", "Q", "R", "T", "M", "T", "A", "I", "P", "B"}},
			{"B", 10, []string{"P", "Q", "P", "T", "P", "T", "P", "P", "P", "B"}},
			{"C", 10, []string{"C", "P", "Q", "P", "T", "P", "T", "P", "P", "P", "X"}},
		}
		for _, sequence := range sequences {
			before := len(batches)
			for index, value := range sequence.values {
				send(value)
				wantBatches := before
				if index >= sequence.boundary-1 {
					wantBatches++
				}
				if len(batches) != wantBatches {
					t.Fatalf("single-key batch count after input %d = %d, want %d", index+1, len(batches), wantBatches)
				}
			}
			if len(batches) != before+1 {
				t.Fatalf("single-key batch count = %d, want %d", len(batches), before+1)
			}
			batch := batches[before]
			if len(batch.New) != 1 || len(batch.Old) != 0 {
				t.Fatalf("single-key batch = %#v, want one new row", batch)
			}
			row, ok := batch.New[0].Row()
			value := row.Get("theString")
			if !ok || value.Any() != sequence.want {
				t.Fatalf("single-key batch row = %#v, want %q (schema=%q fields=%v values=%#v state=%d any=%#v type=%T)", batch.New[0], sequence.want, row.Schema().Name(), row.Schema().PropertyNames(), row.Values(), value.State(), value.Any(), value.Any())
			}
		}
	})

	t.Run("batch-multi-key", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetRowLimitBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()

		theString := Field[resultsetRowLimitBean, string]("theString")
		intPrimitive := Field[resultsetRowLimitBean, int]("intPrimitive")
		plan, err := env.Build(Select(
			From[resultsetRowLimitBean](env, "SupportBean").Window(LengthBatch(5)),
			Alias("theString", theString),
			Alias("intPrimitive", intPrimitive),
		).Query(
			StatementName("s0"),
			OrderBy(
				Ascending(ResultField[string]("theString")),
				Descending(ResultField[int]("intPrimitive")),
			),
			Limit(1),
		))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		var batches []ResultBatch
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		send := func(value string, number int) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitBean{TheString: value, IntPrimitive: number}); err != nil {
				t.Fatal(err)
			}
		}
		sequences := []struct {
			wantString string
			wantNumber int
			values     []resultsetRowLimitBean
		}{
			{"F", 10, []resultsetRowLimitBean{{TheString: "F", IntPrimitive: 10}, {TheString: "X", IntPrimitive: 8}, {TheString: "F", IntPrimitive: 8}, {TheString: "G", IntPrimitive: 10}, {TheString: "X", IntPrimitive: 1}}},
			{"G", 12, []resultsetRowLimitBean{{TheString: "X", IntPrimitive: 10}, {TheString: "G", IntPrimitive: 12}, {TheString: "H", IntPrimitive: 100}, {TheString: "G", IntPrimitive: 10}, {TheString: "X", IntPrimitive: 1}}},
			{"G", 11, []resultsetRowLimitBean{{TheString: "G", IntPrimitive: 10}, {TheString: "G", IntPrimitive: 8}, {TheString: "G", IntPrimitive: 8}, {TheString: "G", IntPrimitive: 10}, {TheString: "G", IntPrimitive: 11}}},
		}
		for _, sequence := range sequences {
			before := len(batches)
			for index, value := range sequence.values {
				send(value.TheString, value.IntPrimitive)
				wantBatches := before
				if index == len(sequence.values)-1 {
					wantBatches++
				}
				if len(batches) != wantBatches {
					t.Fatalf("multi-key batch count after input %d = %d, want %d", index+1, len(batches), wantBatches)
				}
			}
			if len(batches) != before+1 {
				t.Fatalf("multi-key batch count = %d, want %d", len(batches), before+1)
			}
			batch := batches[before]
			if len(batch.New) != 1 || len(batch.Old) != 0 {
				t.Fatalf("multi-key batch = %#v, want one new row", batch)
			}
			row, ok := batch.New[0].Row()
			stringValue := row.Get("theString")
			numberValue := row.Get("intPrimitive")
			if !ok || stringValue.Any() != sequence.wantString || numberValue.Any() != sequence.wantNumber {
				t.Fatalf("multi-key batch row = %#v, want {%q, %d} (schema=%q fields=%v values=%#v string=%#v/%T number=%#v/%T)", batch.New[0], sequence.wantString, sequence.wantNumber, row.Schema().Name(), row.Schema().PropertyNames(), row.Values(), stringValue.Any(), stringValue.Any(), numberValue.Any(), numberValue.Any())
			}
		}
	})

	t.Run("context-single-key", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetRowLimitBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[resultsetRowLimitContextStart](env, "SupportBean_S0"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[resultsetRowLimitContextEnd](env, "SupportBean_S1"); err != nil {
			t.Fatal(err)
		}
		start := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0"))
		end := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S1"))
		if _, err := CreateInitiatedTerminatedContext(env, "StartS0EndS1", Literal("global"), start, end); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()

		theString := Field[resultsetRowLimitBean, string]("theString")
		plan, err := env.Build(Select(
			From[resultsetRowLimitBean](env, "SupportBean").Window(KeepAll()),
			Alias("theString", theString),
		).Query(
			StatementName("s0"),
			WithContext("StartS0EndS1"),
			WithOutput(OutputSnapshotWhenTerminated()),
			OrderBy(Ascending(ResultField[string]("theString"))),
			Limit(1),
		))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		var batches []ResultBatch
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		sendStart := func() {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitContextStart{ID: 0}); err != nil {
				t.Fatal(err)
			}
		}
		sendEnd := func() {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitContextEnd{ID: 0}); err != nil {
				t.Fatal(err)
			}
		}
		send := func(value string) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitBean{TheString: value}); err != nil {
				t.Fatal(err)
			}
		}
		sequences := []struct {
			want   string
			values []string
		}{
			{"A", []string{"F", "Q", "R", "T", "M", "T", "A", "I", "P", "B"}},
			{"B", []string{"P", "Q", "P", "T", "P", "T", "P", "P", "P", "B"}},
			{"C", []string{"C", "P", "Q", "P", "T", "P", "T", "P", "P", "P", "X"}},
		}
		for _, sequence := range sequences {
			before := len(batches)
			sendStart()
			if got := statement.ContextPartitionCount(); got != 1 {
				t.Fatalf("single-key context partitions after start = %d, want 1", got)
			}
			for _, value := range sequence.values {
				send(value)
				if len(batches) != before {
					t.Fatalf("single-key context emitted before S1: %#v", batches)
				}
			}
			sendEnd()
			if len(batches) != before+1 {
				t.Fatalf("single-key context batch count = %d, want %d", len(batches), before+1)
			}
			batch := batches[before]
			if len(batch.New) != 1 || len(batch.Old) != 0 {
				t.Fatalf("single-key context batch = %#v, want one new row", batch)
			}
			row, ok := batch.New[0].Row()
			value := row.Get("theString")
			if !ok || value.Any() != sequence.want {
				t.Fatalf("single-key context row = %#v, want %q (schema=%q fields=%v values=%#v state=%d any=%#v type=%T)", batch.New[0], sequence.want, row.Schema().Name(), row.Schema().PropertyNames(), row.Values(), value.State(), value.Any(), value.Any())
			}
			if got := statement.ContextPartitionCount(); got != 0 {
				t.Fatalf("single-key context partitions after termination = %d, want 0", got)
			}
			snapshot, err := statement.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Results()) != 0 {
				t.Fatalf("single-key context state after termination = %#v, want empty", snapshot.Results())
			}
		}
	})

	t.Run("context-multi-key", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[resultsetRowLimitBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[resultsetRowLimitContextStart](env, "SupportBean_S0"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[resultsetRowLimitContextEnd](env, "SupportBean_S1"); err != nil {
			t.Fatal(err)
		}
		start := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0"))
		end := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S1"))
		if _, err := CreateInitiatedTerminatedContext(env, "StartS0EndS1", Literal("global"), start, end); err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		defer func() { _ = engine.Close(context.Background()) }()

		theString := Field[resultsetRowLimitBean, string]("theString")
		intPrimitive := Field[resultsetRowLimitBean, int]("intPrimitive")
		plan, err := env.Build(Select(
			From[resultsetRowLimitBean](env, "SupportBean").Window(KeepAll()),
			Alias("theString", theString),
			Alias("intPrimitive", intPrimitive),
		).Query(
			StatementName("s0"),
			WithContext("StartS0EndS1"),
			WithOutput(OutputSnapshotWhenTerminated()),
			OrderBy(
				Ascending(ResultField[string]("theString")),
				Descending(ResultField[int]("intPrimitive")),
			),
			Limit(1),
		))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		var batches []ResultBatch
		if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
			batches = append(batches, batch)
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		sendStart := func() {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitContextStart{ID: 0}); err != nil {
				t.Fatal(err)
			}
		}
		sendEnd := func() {
			t.Helper()
			if err := engine.SendEvent(context.Background(), resultsetRowLimitContextEnd{ID: 0}); err != nil {
				t.Fatal(err)
			}
		}
		send := func(event resultsetRowLimitBean) {
			t.Helper()
			if err := engine.SendEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
		}
		sequences := []struct {
			wantString string
			wantNumber int
			values     []resultsetRowLimitBean
		}{
			{"F", 10, []resultsetRowLimitBean{{TheString: "F", IntPrimitive: 10}, {TheString: "X", IntPrimitive: 8}, {TheString: "F", IntPrimitive: 8}, {TheString: "G", IntPrimitive: 10}, {TheString: "X", IntPrimitive: 1}}},
			{"G", 12, []resultsetRowLimitBean{{TheString: "X", IntPrimitive: 10}, {TheString: "G", IntPrimitive: 12}, {TheString: "H", IntPrimitive: 100}, {TheString: "G", IntPrimitive: 10}, {TheString: "X", IntPrimitive: 1}}},
			{"G", 11, []resultsetRowLimitBean{{TheString: "G", IntPrimitive: 10}, {TheString: "G", IntPrimitive: 8}, {TheString: "G", IntPrimitive: 8}, {TheString: "G", IntPrimitive: 10}, {TheString: "G", IntPrimitive: 11}}},
		}
		for _, sequence := range sequences {
			before := len(batches)
			sendStart()
			if got := statement.ContextPartitionCount(); got != 1 {
				t.Fatalf("multi-key context partitions after start = %d, want 1", got)
			}
			for _, event := range sequence.values {
				send(event)
				if len(batches) != before {
					t.Fatalf("multi-key context emitted before S1: %#v", batches)
				}
			}
			sendEnd()
			if len(batches) != before+1 {
				t.Fatalf("multi-key context batch count = %d, want %d", len(batches), before+1)
			}
			batch := batches[before]
			if len(batch.New) != 1 || len(batch.Old) != 0 {
				t.Fatalf("multi-key context batch = %#v, want one new row", batch)
			}
			row, ok := batch.New[0].Row()
			stringValue := row.Get("theString")
			numberValue := row.Get("intPrimitive")
			if !ok || stringValue.Any() != sequence.wantString || numberValue.Any() != sequence.wantNumber {
				t.Fatalf("multi-key context row = %#v, want {%q, %d} (schema=%q fields=%v values=%#v string=%#v/%T number=%#v/%T)", batch.New[0], sequence.wantString, sequence.wantNumber, row.Schema().Name(), row.Schema().PropertyNames(), row.Values(), stringValue.Any(), stringValue.Any(), numberValue.Any(), numberValue.Any())
			}
			if got := statement.ContextPartitionCount(); got != 0 {
				t.Fatalf("multi-key context partitions after termination = %d, want 0", got)
			}
			snapshot, err := statement.Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.Results()) != 0 {
				t.Fatalf("multi-key context state after termination = %#v, want empty", snapshot.Results())
			}
		}
	})
}

// TestResultSetOutputLimitRowLimitFullyGroupedIteratorParity covers
// ResultSetFullyGroupedOrdered. The aggregate iterator is ordered by each
// current group sum before applying limit 2; updating a group replaces its
// aggregate row, and the length window's retained state drives later eviction.
func TestResultSetOutputLimitRowLimitFullyGroupedIteratorParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[resultsetRowLimitGroupedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	theString := Field[resultsetRowLimitGroupedBean, string]("theString")
	intPrimitive := Field[resultsetRowLimitGroupedBean, int]("intPrimitive")
	plan, err := env.Build(From[resultsetRowLimitGroupedBean](env, "SupportBean").Window(LengthWindow(5)).GroupBy(theString).Select(
		Alias("theString", theString),
		Alias("mysum", Sum[int](intPrimitive)),
	).Query(
		StatementName("s0"),
		OrderBy(Ascending(ResultField[int]("mysum"))),
		Limit(2),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]

	assertSnapshot := func(want [][]any, label string) {
		t.Helper()
		snapshot, err := statement.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got := make([][]any, 0, len(snapshot.Results()))
		for _, result := range snapshot.Results() {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("%s result is not a row: %#v", label, result)
			}
			got = append(got, []any{row.Get("theString").Any(), row.Get("mysum").Any()})
		}
		if len(got) == 0 && len(want) == 0 {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s iterator rows = %v, want %v", label, got, want)
		}
	}

	send := func(value string, number int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), resultsetRowLimitGroupedBean{TheString: value, IntPrimitive: number}); err != nil {
			t.Fatal(err)
		}
	}

	assertSnapshot(nil, "initial")
	send("E1", 90)
	assertSnapshot([][]any{{"E1", 90}}, "E1")
	send("E2", 5)
	assertSnapshot([][]any{{"E2", 5}, {"E1", 90}}, "E2")
	send("E3", 60)
	assertSnapshot([][]any{{"E2", 5}, {"E3", 60}}, "E3")
	send("E3", 40)
	assertSnapshot([][]any{{"E2", 5}, {"E1", 90}}, "E3 replacement")
	send("E2", 1000)
	assertSnapshot([][]any{{"E1", 90}, {"E3", 100}}, "E2 replacement and limit eviction")
}
