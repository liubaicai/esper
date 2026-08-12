package esper

import (
	"context"
	"testing"
)

type joinNoTableMarketData struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

type joinNoTableSupportBean struct {
	TheString string `esper:"theString"`
	LongBoxed int64  `esper:"longBoxed"`
}

func TestJoinUnqualifiedPropertyMappingMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinNoTableMarketData](env, "JoinNoTableMarketData"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinNoTableSupportBean](env, "JoinNoTableSupportBean"); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(Join(
		From[joinNoTableMarketData](env, "JoinNoTableMarketData").Window(LengthWindow(3)),
		From[joinNoTableSupportBean](env, "JoinNoTableSupportBean").Window(LengthWindow(3)),
		OnEqual(
			Field[joinNoTableMarketData, string]("symbol"),
			Field[joinNoTableSupportBean, string]("theString"),
		),
		OnEqual(
			Field[joinNoTableMarketData, int64]("volume"),
			Field[joinNoTableSupportBean, int64]("longBoxed"),
		),
	).Select(
		SelectFrom(0, "symbol", Field[joinNoTableMarketData, string]("symbol")),
		SelectFrom(0, "volume", Field[joinNoTableMarketData, int64]("volume")),
		SelectFrom(1, "theString", Field[joinNoTableSupportBean, string]("theString")),
		SelectFrom(1, "longBoxed", Field[joinNoTableSupportBean, int64]("longBoxed")),
	).Query(StatementName("join-unqualified-property-mapping")))
	if err != nil {
		t.Fatal(err)
	}

	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())

	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), joinNoTableMarketData{Symbol: "IBM", Volume: 7}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), joinNoTableSupportBean{TheString: "IBM", LongBoxed: 8}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("non-matching unqualified join emitted %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), joinNoTableSupportBean{TheString: "IBM", LongBoxed: 7}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("matching unqualified join rows = %#v, want one row", rows)
	}
	row := rows[0]
	if row.Get("symbol").Any() != "IBM" || row.Get("volume").Any() != int64(7) ||
		row.Get("theString").Any() != "IBM" || row.Get("longBoxed").Any() != int64(7) {
		t.Fatalf("unqualified join projection = %#v", row.AsMap())
	}
}
