package esper

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

type rowRecogFinancialWEvent struct {
	symbol   string
	price    float64
	expected []string
}

func TestRowRecogFinancialWPatternDataSet(t *testing.T) {
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent")
	price := Field[rowRecogTestEvent, float64]("price")
	previous := func() Expression[float64] { return Prev[float64](1, price) }
	query := stream.MatchRecognize(RowSequence(
		RowVar("A"),
		RowVar("W").OneOrMore(),
		RowVar("X").OneOrMore(),
		RowVar("Y").OneOrMore(),
		RowVar("Z").OneOrMore(),
	)).
		Define("W", Less[float64](price, previous())).
		Define("X", Greater[float64](price, previous())).
		Define("Y", Less[float64](price, previous())).
		Define("Z", Greater[float64](price, previous())).
		AllMatches().
		SkipToCurrentRow().
		Measures(
			Alias("beginA", TagField[string]("A", "symbol")),
			Alias("lastZ", TagField[string]("Z", "symbol")),
		).
		Query(StatementName("rowrecog-financial-w"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)

	data := []rowRecogFinancialWEvent{
		{symbol: "E1", price: 8},
		{symbol: "E2", price: 8},
		{symbol: "E3", price: 8},
		{symbol: "E4", price: 6},
		{symbol: "E5", price: 3},
		{symbol: "E6", price: 7},
		{symbol: "E7", price: 6},
		{symbol: "E8", price: 2},
		{symbol: "E9", price: 6, expected: []string{"E3/E9", "E4/E9"}},
		{symbol: "E10", price: 2},
		{symbol: "E11", price: 9, expected: []string{"E6/E11", "E7/E11"}},
		{symbol: "E12", price: 9},
		{symbol: "E13", price: 8},
		{symbol: "E14", price: 5},
		{symbol: "E15", price: 0},
		{symbol: "E16", price: 9},
		{symbol: "E17", price: 2},
		{symbol: "E18", price: 0},
		{symbol: "E19", price: 2, expected: []string{"E12/E19", "E13/E19", "E14/E19"}},
		{symbol: "E20", price: 3, expected: []string{"E12/E20", "E13/E20", "E14/E20"}},
		{symbol: "E21", price: 8, expected: []string{"E12/E21", "E13/E21", "E14/E21"}},
		{symbol: "E22", price: 5},
		{symbol: "E23", price: 9, expected: []string{"E16/E23", "E17/E23"}},
		{symbol: "E24", price: 9},
		{symbol: "E25", price: 4},
		{symbol: "E26", price: 7},
		{symbol: "E27", price: 2},
		{symbol: "E28", price: 8, expected: []string{"E24/E28"}},
		{symbol: "E29", price: 0},
		{symbol: "E30", price: 4, expected: []string{"E26/E30"}},
		{symbol: "E31", price: 4},
		{symbol: "E32", price: 7},
		{symbol: "E33", price: 8},
		{symbol: "E34", price: 6},
		{symbol: "E35", price: 4},
		{symbol: "E36", price: 5},
		{symbol: "E37", price: 1},
		{symbol: "E38", price: 7, expected: []string{"E33/E38", "E34/E38"}},
		{symbol: "E39", price: 5},
		{symbol: "E40", price: 8, expected: []string{"E36/E40"}},
		{symbol: "E41", price: 6},
		{symbol: "E42", price: 6},
		{symbol: "E43", price: 0},
		{symbol: "E44", price: 6},
		{symbol: "E45", price: 8},
		{symbol: "E46", price: 4},
		{symbol: "E47", price: 3},
		{symbol: "E48", price: 8, expected: []string{"E42/E48"}},
		{symbol: "E49", price: 2},
		{symbol: "E50", price: 5, expected: []string{"E45/E50", "E46/E50"}},
		{symbol: "E51", price: 3},
		{symbol: "E52", price: 3},
		{symbol: "E53", price: 9},
		{symbol: "E54", price: 8},
		{symbol: "E55", price: 5},
		{symbol: "E56", price: 5},
		{symbol: "E57", price: 9},
		{symbol: "E58", price: 7},
		{symbol: "E59", price: 3},
		{symbol: "E60", price: 3},
	}

	total := 0
	for _, event := range data {
		before := len(*rows)
		if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: event.symbol, Price: event.price}); err != nil {
			t.Fatal(err)
		}
		got := rowRecogFinancialWKeys((*rows)[before:])
		want := sortedStrings(event.expected)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("financial W event %s listener batch = %#v, want %#v", event.symbol, got, want)
		}
		total += len(event.expected)
		if len(*rows) != total {
			t.Fatalf("financial W event %s cumulative listener rows = %d, want %d", event.symbol, len(*rows), total)
		}
	}
}

func rowRecogFinancialWKeys(rows []Row) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		begin := row.Get("beginA")
		last := row.Get("lastZ")
		beginValue, lastValue := "", ""
		if begin.IsPresent() {
			beginValue = fmt.Sprint(begin.Any())
		}
		if last.IsPresent() {
			lastValue = fmt.Sprint(last.Any())
		}
		keys = append(keys, beginValue+"/"+lastValue)
	}
	sort.Strings(keys)
	return keys
}
