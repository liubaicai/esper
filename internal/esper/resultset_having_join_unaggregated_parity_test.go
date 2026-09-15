package esper

import (
	"context"
	"reflect"
	"testing"
)

// TestUnaggregatedJoinRowPerTuple pins the Java
// ResultSetProcessorFactoryFactory branch 1 parity for join statements whose
// select and having clauses contain no aggregate functions (pinned
// ResultSetQueryTypeHaving NoAggregationJoinHaving/Where twins): every new
// joined tuple projects one new row and every leaving tuple one old row,
// each gated per tuple by the having clause. The aggregate state machine's
// ungrouped contracts - null-prior old-row pairing on first emission and
// previous-row re-emission on removal - must not run for this shape.
func TestUnaggregatedJoinRowPerTuple(t *testing.T) {
	for _, mode := range []string{"having", "where"} {
		t.Run(mode, func(t *testing.T) {
			env := NewEnvironment()
			str := reflect.TypeOf("")
			f64 := reflect.TypeOf(float64(0))
			if _, err := RegisterMap(env, "SupportMarketDataBean", []FieldSpec{
				FieldDef("symbol", str),
				FieldDef("price", f64),
			}); err != nil {
				t.Fatal(err)
			}
			aPrice := JoinField[float64](0, "price")
			bPrice := JoinField[float64](1, "price")
			spread := Subtract[float64](MaxOf[float64](aPrice, bPrice), MinOf[float64](aPrice, bPrice))
			threshold := GreaterOrEqual[float64](spread, Literal(1.4))
			join := JoinMany(
				JoinSource(From[map[string]any](env, "SupportMarketDataBean").
					Filter(Equal[string](Field[map[string]any, string]("symbol"), Literal("SYM1"))).
					Window(LengthWindow(1))),
				JoinSource(From[map[string]any](env, "SupportMarketDataBean").
					Filter(Equal[string](Field[map[string]any, string]("symbol"), Literal("SYM2"))).
					Window(LengthWindow(1))),
			)
			query := join.Aggregate(
				Alias("aPrice", aPrice),
				Alias("bPrice", bPrice),
				Alias("spread", spread),
			)
			if mode == "where" {
				query = query.Where(threshold)
			} else {
				query = query.Having(threshold)
			}
			plan, err := env.Build(query.Query(StatementName("s0"), WithOldStream()))
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
			type delivery struct {
				newRows [][]any
				oldRows [][]any
			}
			var deliveries []delivery
			if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				entry := delivery{}
				collect := func(results []Result) [][]any {
					rows := [][]any{}
					for _, result := range results {
						row, ok := result.Row()
						if !ok {
							continue
						}
						rows = append(rows, []any{row.Get("aPrice").Any(), row.Get("bPrice").Any(), row.Get("spread").Any()})
					}
					return rows
				}
				entry.newRows = collect(batch.New)
				entry.oldRows = collect(batch.Old)
				deliveries = append(deliveries, entry)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			sends := []struct {
				symbol string
				price  float64
			}{
				{"SYM1", 20}, {"SYM2", 10}, {"SYM2", 20}, {"SYM2", 20}, {"SYM2", 20},
				{"SYM1", 20}, {"SYM1", 18.7}, {"SYM2", 20}, {"SYM1", 18.5}, {"SYM2", 16}, {"SYM1", 12},
			}
			for _, send := range sends {
				if err := engine.Send(context.Background(), "SupportMarketDataBean", map[string]any{
					"symbol": send.symbol,
					"price":  send.price,
				}); err != nil {
					t.Fatal(err)
				}
			}

			// Java listener contract: new(20,10,10); old(20,10,10); nothing;
			// nothing; new(18.5,20,1.5); old(18.5,20,1.5)+new(18.5,16,2.5);
			// old(18.5,16,2.5)+new(12,16,4).
			if len(deliveries) != 5 {
				t.Fatalf("mode %s deliveries = %d, want 5", mode, len(deliveries))
			}
			assertRows := func(label string, got [][]any, want [][]any) {
				t.Helper()
				if len(got) != len(want) {
					t.Fatalf("mode %s %s rows = %v, want %v", mode, label, got, want)
				}
				for i := range want {
					for j := range want[i] {
						gotValue, gotOK := got[i][j].(float64)
						wantValue := want[i][j].(float64)
						if !gotOK || gotValue != wantValue {
							t.Fatalf("mode %s %s rows = %v, want %v", mode, label, got, want)
						}
					}
				}
			}
			assertRows("1 new", deliveries[0].newRows, [][]any{{20.0, 10.0, 10.0}})
			if len(deliveries[0].oldRows) != 0 {
				t.Fatalf("mode %s delivery 1 old = %v, want none", mode, deliveries[0].oldRows)
			}
			if len(deliveries[1].newRows) != 0 {
				t.Fatalf("mode %s delivery 2 new = %v, want none", mode, deliveries[1].newRows)
			}
			assertRows("2 old", deliveries[1].oldRows, [][]any{{20.0, 10.0, 10.0}})
			assertRows("3 new", deliveries[2].newRows, [][]any{{18.5, 20.0, 1.5}})
			if len(deliveries[2].oldRows) != 0 {
				t.Fatalf("mode %s delivery 3 old = %v, want none", mode, deliveries[2].oldRows)
			}
			assertRows("4 old", deliveries[3].oldRows, [][]any{{18.5, 20.0, 1.5}})
			assertRows("4 new", deliveries[3].newRows, [][]any{{18.5, 16.0, 2.5}})
			assertRows("5 old", deliveries[4].oldRows, [][]any{{18.5, 16.0, 2.5}})
			assertRows("5 new", deliveries[4].newRows, [][]any{{12.0, 16.0, 4.0}})
		})
	}
}
