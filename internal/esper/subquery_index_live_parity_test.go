package esper

import (
	"context"
	"testing"
)

// TestInfraNWTableSubqCorrelIndexLiveListenerParity covers the live listener
// part of InfraNWTableSubqCorrelIndexAssertion.  Java sends SupportBean_S0
// after the inner Named Window/Table has been populated, then undeploys and
// redeploys the consumer to verify the late-start path.  The Go rule keeps the
// same shape with a map-backed event source and a typed fluent subquery; the
// physical lookup counter is an implementation-neutral observable for the
// hash-index-sharing choices.
func TestInfraNWTableSubqCorrelIndexLiveListenerParity(t *testing.T) {
	type testCase struct {
		name           string
		namedWindow    bool
		share          bool
		declaredIndex  bool
		disableShare   bool
		noIndex        bool
		wantIndexProbe bool
	}
	cases := []testCase{
		{name: "named-window-no-share", namedWindow: true},
		{name: "named-window-no-share-noindex", namedWindow: true, noIndex: true},
		{name: "named-window-no-share-explicit", namedWindow: true, declaredIndex: true, wantIndexProbe: true},
		{name: "named-window-share", namedWindow: true, share: true, wantIndexProbe: true},
		{name: "named-window-share-explicit", namedWindow: true, share: true, declaredIndex: true, wantIndexProbe: true},
		{name: "named-window-share-explicit-noindex", namedWindow: true, share: true, declaredIndex: true, noIndex: true},
		{name: "named-window-share-disable", namedWindow: true, share: true, disableShare: true},
		{name: "named-window-share-disable-explicit", namedWindow: true, share: true, declaredIndex: true, disableShare: true, wantIndexProbe: true},
		{name: "table-no-index", wantIndexProbe: false},
		{name: "table-explicit", declaredIndex: true, wantIndexProbe: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := NewEnvironment()
			schema := newFAFSubqueryIndexSchema(t, env, "SubqLiveSchema")
			outerType := "SubqLiveOuter"
			outer := FromAny(env, outerType)
			if _, err := RegisterMap(env, outerType, []FieldSpec{
				FieldDef("id", typeOf[int64]()),
				FieldDef("key", typeOf[string]()),
			}); err != nil {
				t.Fatal(err)
			}

			const innerName = "SubqLiveInner"
			var inner RecordStream
			if tc.namedWindow {
				options := []NamedWindowOption{NamedWindowRetention(Unique(Field[any, string]("lookup")))}
				if tc.share {
					options = append(options, NamedWindowSubqueryIndexSharing())
				}
				if tc.declaredIndex {
					options = append(options, NamedWindowIndex("by-lookup", "lookup"))
				}
				if _, err := CreateNamedWindow(env, innerName, schema, options...); err != nil {
					t.Fatal(err)
				}
				inner = FromNamedWindow(env, innerName)
			} else {
				options := make([]TableOption, 0, 1)
				if tc.declaredIndex {
					options = append(options, SecondaryIndex("by-lookup", "lookup"))
				}
				if _, err := CreateTable(env, innerName, []TableColumn{
					PrimaryKeyColumn[int64]("id"),
					TableColumnOf[string]("lookup"),
					TableColumnOf[string]("value"),
				}, options...); err != nil {
					t.Fatal(err)
				}
				inner = FromTable(env, innerName)
			}

			engine := NewEngine(env)
			ctx := context.Background()
			innerRows := []map[string]any{
				{"id": int64(101), "lookup": "E1", "value": "E1-hit"},
				{"id": int64(102), "lookup": "E2", "value": "E2-hit"},
				{"id": int64(103), "lookup": "unused", "value": "not-selected"},
			}
			for _, row := range innerRows {
				if tc.namedWindow {
					if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table(innerName)
				if !ok {
					t.Fatal("live subquery table is missing")
				}
				if _, err := table.Insert(ctx, row); err != nil {
					t.Fatal(err)
				}
			}

			options := []SubqueryOption{
				SubqueryWhere(Equal[string](Field[any, string]("lookup"), OuterField[string]("key"))),
			}
			if tc.disableShare {
				options = append(options, SubqueryDisableIndexSharing())
			}
			if tc.noIndex {
				options = append(options, SubqueryNoIndex())
			}
			value := SubqueryValueWithOptions[string](inner, Field[any, string]("value"), options...)
			plan, err := env.Build(outer.Select(
				Alias("id", Field[any, int64]("id")),
				Alias("value", value),
			).Query(StatementName("subquery-live-consumer")))
			if err != nil {
				t.Fatal(err)
			}

			lookupCount := func() uint64 {
				if tc.namedWindow {
					window, ok := engine.NamedWindow(innerName)
					if !ok {
						t.Fatal("live subquery named window is missing")
					}
					return window.state.indexLookups.Load()
				}
				table, ok := engine.Table(innerName)
				if !ok {
					t.Fatal("live subquery table is missing")
				}
				return table.state.indexLookups.Load()
			}

			type outerCase struct {
				id   int64
				key  string
				want string
			}
			sendAndAssert := func(events ...outerCase) {
				t.Helper()
				var batches []ResultBatch
				deployment, deployErr := engine.Deploy(ctx, plan)
				if deployErr != nil {
					t.Fatal(deployErr)
				}
				statement := deployment.Statements()[0]
				if _, subscribeErr := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
					batches = append(batches, batch)
					return nil
				}); subscribeErr != nil {
					t.Fatal(subscribeErr)
				}
				for _, event := range events {
					if err := engine.SendRecord(ctx, outerType, map[string]any{"id": event.id, "key": event.key}); err != nil {
						t.Fatal(err)
					}
				}
				if err := deployment.Undeploy(ctx); err != nil {
					t.Fatal(err)
				}
				if len(batches) != len(events) {
					t.Fatalf("live subquery listener batches = %#v, want %d batches", batches, len(events))
				}
				for index, event := range events {
					if len(batches[index].New) != 1 || len(batches[index].Old) != 0 {
						t.Fatalf("live subquery listener batch %d = %#v, want one new row", index, batches[index])
					}
					row := batches[index].New[0]
					if row.Get("id").Any() != event.id || row.Get("value").Any() != event.want {
						t.Fatalf("live subquery row %d = %#v, want id=%d value=%q", index, row, event.id, event.want)
					}
				}
			}

			before := lookupCount()
			sendAndAssert(outerCase{id: 1, key: "E1", want: "E1-hit"}, outerCase{id: 2, key: "E2", want: "E2-hit"})
			// Match Java's late-start check: the consumer is deployed after the
			// inner rows already exist, then redeployed and fed the same probes.
			sendAndAssert(outerCase{id: 1, key: "E1", want: "E1-hit"}, outerCase{id: 2, key: "E2", want: "E2-hit"})
			after := lookupCount()
			if tc.wantIndexProbe {
				if after != before+4 {
					t.Fatalf("live subquery expected one index probe per outer event and late-start replay: before=%d after=%d", before, after)
				}
			} else if after != before {
				t.Fatalf("live subquery unexpectedly used an index: before=%d after=%d", before, after)
			}
		})
	}
}

// TestInfraNWTableSubqCorrelIndexChoiceLiveListenerParity keeps the live
// event-send assertion used by both Java index-choice executions.  The
// structured selection itself is checked by TestInfraNWTableSubqCorrelIndexChoiceParity;
// this test proves that the chosen candidate still reaches a live listener
// and that the final predicate filters the candidate correctly.
func TestInfraNWTableSubqCorrelIndexChoiceLiveListenerParity(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(indexStoreName(namedWindow), func(t *testing.T) {
			env := NewEnvironment()
			name := indexStoreName(namedWindow)
			schema, err := RegisterStruct[infraIndexChoiceEvent](env, "SubqChoiceLiveSchema"+name)
			if err != nil {
				t.Fatal(err)
			}
			innerName := "SubqChoiceLiveInner" + name
			if namedWindow {
				if _, err := CreateNamedWindow(env, innerName, schema,
					NamedWindowRetention(KeepAll()),
					NamedWindowIndex("by-s1", "s1"),
					NamedWindowIndex("by-s1-d1", "s1", "d1"),
					NamedWindowBTreeIndex("by-s1-d1-range", "s1", "d1")); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, innerName, []TableColumn{
				PrimaryKeyColumn[string]("s1"),
				TableColumnOf[int64]("i1"),
				TableColumnOf[float64]("d1"),
			}, SecondaryIndex("by-s1", "s1"), SecondaryIndex("by-s1-d1", "s1", "d1"),
				SecondaryBTreeIndex("by-s1-d1-range", "s1", "d1")); err != nil {
				t.Fatal(err)
			}

			engine := NewEngine(env)
			ctx := context.Background()
			innerRows := []infraIndexChoiceEvent{
				{S1: "A", I1: 10, D1: 11},
				{S1: "B", I1: 20, D1: 21},
			}
			for _, row := range innerRows {
				if namedWindow {
					if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
						t.Fatal(err)
					}
					continue
				}
				table, ok := engine.Table(innerName)
				if !ok {
					t.Fatal("live index-choice table is missing")
				}
				if _, err := table.Insert(ctx, map[string]any{"s1": row.S1, "i1": row.I1, "d1": row.D1}); err != nil {
					t.Fatal(err)
				}
			}

			inner := FromNamedWindow(env, innerName)
			if !namedWindow {
				inner = FromTable(env, innerName)
			}
			outer := From[infraIndexChoiceEvent](env, schema.Name())
			assertLive := func(t *testing.T, statementName string, predicate Expression[bool], option SubqueryOption) {
				t.Helper()
				subqueryOptions := []SubqueryOption{SubqueryWhere(predicate)}
				if option != nil {
					subqueryOptions = append(subqueryOptions, option)
				}
				value := SubqueryValueWithOptions[int64](inner, Field[any, int64]("i1"), subqueryOptions...)
				plan, err := env.Build(Select(outer,
					Alias("s1", Field[any, string]("s1")),
					Alias("value", value),
				).Query(StatementName(statementName)))
				if err != nil {
					t.Fatal(err)
				}
				before := lookupCountForSubqueryIndexTest(t, engine, innerName, namedWindow)
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					t.Fatal(err)
				}
				var batches []ResultBatch
				if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
					batches = append(batches, batch)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				for _, row := range innerRows {
					if err := engine.Send(ctx, schema.Name(), row); err != nil {
						t.Fatal(err)
					}
				}
				if err := deployment.Undeploy(ctx); err != nil {
					t.Fatal(err)
				}
				if len(batches) != 2 {
					t.Fatalf("live index-choice listener batches = %#v, want 2", batches)
				}
				for index, row := range batches {
					if len(row.New) != 1 || row.New[0].Get("s1").Any() != innerRows[index].S1 || row.New[0].Get("value").Any() != innerRows[index].I1 {
						t.Fatalf("live index-choice batch %d = %#v, want %s/%d", index, row, innerRows[index].S1, innerRows[index].I1)
					}
				}
				after := lookupCountForSubqueryIndexTest(t, engine, innerName, namedWindow)
				if after != before+2 {
					t.Fatalf("live index-choice expected one lookup per outer event: before=%d after=%d", before, after)
				}
			}

			assertLive(t, "choice-composite", And(
				EqualOf(Field[any, string]("s1"), OuterField[string]("s1")),
				EqualOf(Field[any, float64]("d1"), OuterField[float64]("d1")),
			), nil)
			assertLive(t, "choice-range", And(
				EqualOf(Field[any, string]("s1"), OuterField[string]("s1")),
				GreaterOrEqual[float64](Field[any, float64]("d1"), OuterField[float64]("d1")),
			), nil)
			assertLive(t, "choice-explicit-override", And(
				EqualOf(Field[any, string]("s1"), OuterField[string]("s1")),
				EqualOf(Field[any, float64]("d1"), OuterField[float64]("d1")),
			), SubqueryUseIndex("by-s1"))
		})
	}
}

// lookupCountForSubqueryIndexTest keeps the live listener tests focused on
// behavior while sharing the same observable counter access for both stores.
func lookupCountForSubqueryIndexTest(t *testing.T, engine *Engine, name string, namedWindow bool) uint64 {
	t.Helper()
	if namedWindow {
		window, ok := engine.NamedWindow(name)
		if !ok {
			t.Fatalf("named window %q is missing", name)
		}
		return window.state.indexLookups.Load()
	}
	table, ok := engine.Table(name)
	if !ok {
		t.Fatalf("table %q is missing", name)
	}
	return table.state.indexLookups.Load()
}

// TestInfraNWTableSubqIndexShareMultikeyArrayLiveListenerParity covers the
// live SupportEventWithManyArray path of the Java single-array and two-array
// executions.  The existing FAF test checks all four values; this test adds
// listener batches and confirms that slice keys are probed as value keys.
func TestInfraNWTableSubqIndexShareMultikeyArrayLiveListenerParity(t *testing.T) {
	for _, mode := range []struct {
		name        string
		namedWindow bool
		explicit    bool
	}{
		{name: "named-window-explicit", namedWindow: true, explicit: true},
		{name: "named-window-auto", namedWindow: true},
		{name: "table"},
	} {
		for _, multiKey := range []bool{false, true} {
			name := mode.name
			if multiKey {
				name += "-two-array"
			} else {
				name += "-one-array"
			}
			t.Run(name, func(t *testing.T) {
				env := NewEnvironment()
				schema, err := RegisterStruct[infraIndexArrayEvent](env, "SubqArrayLiveSchema"+name)
				if err != nil {
					t.Fatal(err)
				}
				innerName := "SubqArrayLiveInner" + name
				if mode.namedWindow {
					options := []NamedWindowOption{NamedWindowRetention(KeepAll()), NamedWindowSubqueryIndexSharing()}
					if mode.explicit {
						columns := []string{"arrayOne"}
						if multiKey {
							columns = append(columns, "arrayTwo")
						}
						options = append(options, NamedWindowIndex("by-array", columns...))
					}
					if _, err := CreateNamedWindow(env, innerName, schema, options...); err != nil {
						t.Fatal(err)
					}
				} else {
					columns := []TableColumn{PrimaryKeyColumn[[]string]("arrayOne"), TableColumnOf[[]string]("arrayTwo"), TableColumnOf[int64]("value")}
					if multiKey {
						columns = []TableColumn{PrimaryKeyColumn[[]string]("arrayOne"), PrimaryKeyColumn[[]string]("arrayTwo"), TableColumnOf[int64]("value")}
					}
					if _, err := CreateTable(env, innerName, columns); err != nil {
						t.Fatal(err)
					}
				}

				engine := NewEngine(env)
				ctx := context.Background()
				innerRows := []infraIndexArrayEvent{
					{ID: "E1", ArrayOne: []string{"a", "b"}, ArrayTwo: []string{"c", "d"}, Value: 10},
					{ID: "E2", ArrayOne: []string{"a", "c"}, ArrayTwo: []string{"b"}, Value: 20},
					{ID: "E3", ArrayOne: []string{"a"}, ArrayTwo: []string{"c", "d"}, Value: 30},
				}
				for _, row := range innerRows {
					if mode.namedWindow {
						if err := engine.InsertNamedWindow(ctx, innerName, row); err != nil {
							t.Fatal(err)
						}
						continue
					}
					table, ok := engine.Table(innerName)
					if !ok {
						t.Fatal("live array subquery table is missing")
					}
					if _, err := table.Insert(ctx, map[string]any{"arrayOne": row.ArrayOne, "arrayTwo": row.ArrayTwo, "value": row.Value}); err != nil {
						t.Fatal(err)
					}
				}

				var predicate Expression[bool]
				if multiKey {
					predicate = And(
						EqualOf(Field[any, []string]("arrayOne"), OuterField[[]string]("arrayOne")),
						EqualOf(Field[any, []string]("arrayTwo"), OuterField[[]string]("arrayTwo")),
					)
				} else {
					predicate = EqualOf(Field[any, []string]("arrayOne"), OuterField[[]string]("arrayOne"))
				}
				options := []SubqueryOption{SubqueryWhere(predicate)}
				if mode.explicit {
					options = append(options, SubqueryUseIndex("by-array"))
				}
				value := SubqueryValueWithOptions[int64](func() RecordStream {
					if mode.namedWindow {
						return FromNamedWindow(env, innerName)
					}
					return FromTable(env, innerName)
				}(), Field[any, int64]("value"), options...)
				plan, err := env.Build(Select(From[infraIndexArrayEvent](env, schema.Name()),
					Alias("id", Field[any, string]("id")),
					Alias("value", value),
				).Query(StatementName("subquery-array-live-" + name)))
				if err != nil {
					t.Fatal(err)
				}

				outerRows := []infraIndexArrayEvent{
					{ID: "O1", ArrayOne: []string{"a", "c"}, ArrayTwo: []string{"b"}},
					{ID: "O2", ArrayOne: []string{"a", "b"}, ArrayTwo: []string{"c", "d"}},
					{ID: "O3", ArrayOne: []string{"a"}, ArrayTwo: []string{"c", "d"}},
					{ID: "O4", ArrayOne: []string{"a", "d"}, ArrayTwo: []string{"b"}},
				}
				before := lookupCountForSubqueryIndexTest(t, engine, innerName, mode.namedWindow)
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					t.Fatal(err)
				}
				var batches []ResultBatch
				if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
					batches = append(batches, batch)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				for _, row := range outerRows {
					if err := engine.Send(ctx, schema.Name(), row); err != nil {
						t.Fatal(err)
					}
				}
				if err := deployment.Undeploy(ctx); err != nil {
					t.Fatal(err)
				}
				if len(batches) != len(outerRows) {
					t.Fatalf("live array subquery listener batches = %#v, want %d", batches, len(outerRows))
				}
				want := []any{int64(20), int64(10), int64(30), nil}
				for index, batch := range batches {
					if len(batch.New) != 1 || batch.New[0].Get("id").Any() != outerRows[index].ID {
						t.Fatalf("live array subquery batch %d = %#v", index, batch)
					}
					if want[index] == nil {
						if !batch.New[0].Get("value").IsNull() {
							t.Fatalf("live array subquery miss batch %d = %#v, want Null", index, batch)
						}
						continue
					}
					if batch.New[0].Get("value").Any() != want[index] {
						t.Fatalf("live array subquery batch %d = %#v, want %#v", index, batch.New[0].Get("value"), want[index])
					}
				}
				after := lookupCountForSubqueryIndexTest(t, engine, innerName, mode.namedWindow)
				if after != before+uint64(len(outerRows)) {
					t.Fatalf("live array subquery expected one lookup per outer event: before=%d after=%d", before, after)
				}
			})
		}
	}
}
