package esper

import (
	"context"
	"testing"
)

type matchRecognizeBean struct {
	TheString string `esper:"theString"`
	Value     int    `esper:"value"`
}

// TestMatchRecognizeSimpleParity mirrors the shared match-recognize-simple
// parity scenario (Java RowRecogConcatenation): pattern (A B) with
// define B as B.value > A.value produces exactly E2/E3 and E4/E5.
func TestMatchRecognizeSimpleParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[matchRecognizeBean](env, "SupportRecogBean"); err != nil {
		t.Fatal(err)
	}
	stream := From[matchRecognizeBean](env, "SupportRecogBean").Window(KeepAll())
	query := stream.MatchRecognize(RowSequence(RowVar("A"), RowVar("B"))).
		Define("B", Greater[int](TagField[int]("B", "value"), TagField[int]("A", "value"))).
		Measures(
			Alias("a_string", TagField[string]("A", "theString")),
			Alias("b_string", TagField[string]("B", "theString")),
		).Query(
		StatementName("s0"),
		OrderBy(
			Ascending(ResultField[string]("a_string")),
			Ascending(ResultField[string]("b_string")),
		),
	)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("match result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []matchRecognizeBean{
		{TheString: "E1", Value: 5},
		{TheString: "E2", Value: 3},
		{TheString: "E3", Value: 6},
		{TheString: "E4", Value: 4},
		{TheString: "E5", Value: 6},
		{TheString: "E6", Value: 10},
		{TheString: "E7", Value: 9},
		{TheString: "E8", Value: 4},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 || rows[0].Get("a_string").Any() != "E2" || rows[0].Get("b_string").Any() != "E3" ||
		rows[1].Get("a_string").Any() != "E4" || rows[1].Get("b_string").Any() != "E5" {
		t.Fatalf("match rows = %#v", rows)
	}
}
