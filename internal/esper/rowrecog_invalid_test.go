package esper

import (
	"errors"
	"testing"
)

func TestRowRecogInvalidJavaMatrix(t *testing.T) {
	env, _ := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	symbol := Field[rowRecogTestEvent, string]("symbol")

	makeQuery := func(name string, pattern RowPattern, defines func(RowRecogQuery) RowRecogQuery, measures ...Selection) Query {
		query := stream.MatchRecognize(pattern)
		if defines != nil {
			query = defines(query)
		}
		return query.Measures(measures...).Query(StatementName(name))
	}

	invalid := []struct {
		name  string
		query Query
	}{
		{
			name: "duplicate-define",
			query: makeQuery("rowrecog-invalid-duplicate-define", RowSequence(RowVar("A"), RowVar("B")), func(query RowRecogQuery) RowRecogQuery {
				return query.Define("A", Literal(true)).Define("A", Literal(false))
			}, Alias("a", TagField[string]("A", "symbol"))),
		},
		{
			name: "future-define-tag",
			query: makeQuery("rowrecog-invalid-future-tag", RowSequence(RowVar("A"), RowVar("B")), func(query RowRecogQuery) RowRecogQuery {
				return query.Define("A", Equal[string](TagField[string]("B", "symbol"), symbol))
			}, Alias("a", TagField[string]("A", "symbol"))),
		},
		{
			name: "aggregate-multiple-groups",
			query: makeQuery("rowrecog-invalid-aggregate-multiple", RowSequence(RowVar("A").ZeroOrMore(), RowVar("B").ZeroOrMore()), func(query RowRecogQuery) RowRecogQuery {
				return query
			}, Alias("total", Sum[float64](Add[float64](TagField[float64]("A", "price"), TagField[float64]("B", "price"))))),
		},
		{
			name: "aggregate-no-group",
			query: makeQuery("rowrecog-invalid-aggregate-no-group", RowSequence(RowVar("A"), RowVar("B").ZeroOrMore()), func(query RowRecogQuery) RowRecogQuery {
				return query
			}, Alias("total", Sum[float64](TagField[float64]("A", "price")))),
		},
		{
			name: "aggregate-in-define",
			query: makeQuery("rowrecog-invalid-aggregate-define", RowSequence(RowVar("A"), RowVar("B")), func(query RowRecogQuery) RowRecogQuery {
				return query.Define("A", Greater[float64](Sum[float64](Add[float64](TagField[float64]("A", "price"), TagField[float64]("A", "price"))), Literal(3000.0)))
			}, Alias("a", TagField[string]("A", "symbol"))),
		},
	}

	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := env.Build(testCase.query); err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("invalid row-recognize query error = %v", err)
			}
		})
	}
}
