package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type rowRecogRegexCase struct {
	input    string
	expected []string
}

type rowRecogRegexScenario struct {
	name     string
	measures []string
	pattern  RowPattern
	cases    []rowRecogRegexCase
}

func TestRowRecogRegexMatrix(t *testing.T) {
	scenarios := []rowRecogRegexScenario{
		{
			name:     "optionalPrefixAndGroups",
			measures: []string{"a", "b", "c", "d"},
			pattern: RowSequence(
				RowVar("A").Optional(),
				RowVar("B").Optional(),
				RowSequence(RowVar("C"), RowVar("D")).Optional(),
			),
			cases: []rowRecogRegexCase{
				{input: "a", expected: []string{"a,null,null,null"}},
				{input: "b", expected: []string{"null,b,null,null"}},
				{input: "x"},
				{input: "d"},
				{input: "c"},
				{input: "d,c"},
				{input: "c,d", expected: []string{"null,null,c,d"}},
				{input: "a,c,d", expected: []string{"a,null,null,null", "a,null,c,d", "null,null,c,d"}},
				{input: "b,c,d", expected: []string{"null,b,null,null", "null,null,c,d", "null,b,c,d"}},
				{input: "a,b,c,d", expected: []string{"a,b,null,null", "a,null,null,null", "null,b,null,null", "null,b,c,d", "a,b,c,d", "null,null,c,d"}},
			},
		},
		{
			name:     "alternatingVariables",
			measures: []string{"a", "b", "c", "d"},
			pattern: RowSequence(
				RowAlternation(RowVar("A"), RowVar("B")),
				RowAlternation(RowVar("C"), RowVar("D")),
			),
			cases: []rowRecogRegexCase{
				{input: "a"},
				{input: "c"},
				{input: "d,c"},
				{input: "a,b"},
				{input: "a,d", expected: []string{"a,null,null,d"}},
				{input: "a,d,c", expected: []string{"a,null,null,d"}},
				{input: "b,c", expected: []string{"null,b,c,null"}},
				{input: "b,d", expected: []string{"null,b,null,d"}},
				{input: "b,a,d,c", expected: []string{"a,null,null,d"}},
				{input: "x,a,x,b,x,b,c,x", expected: []string{"null,b,c,null"}},
			},
		},
		{
			name:     "optionalAlternativeTail",
			measures: []string{"a", "b", "c", "d", "e"},
			pattern: RowSequence(
				RowVar("A"),
				RowAlternation(
					RowSequence(RowVar("B"), RowVar("C")).Optional(),
					RowSequence(RowVar("D"), RowVar("E")).Optional(),
				),
			),
			cases: []rowRecogRegexCase{
				{input: "a", expected: []string{"a,null,null,null,null"}},
				{input: "a,b,c", expected: []string{"a,null,null,null,null", "a,b,c,null,null"}},
				{input: "a,d,e", expected: []string{"a,null,null,null,null", "a,null,null,d,e"}},
				{input: "b,c"},
				{input: "x,d,e"},
			},
		},
		{
			name:     "optionalPrefixAlternation",
			measures: []string{"a", "b", "c"},
			pattern: RowAlternation(
				RowSequence(RowVar("A").Optional(), RowVar("B")),
				RowSequence(RowVar("A").Optional(), RowVar("C")),
			),
			cases: []rowRecogRegexCase{
				{input: "a"},
				{input: "a,b", expected: []string{"a,b,null", "null,b,null"}},
				{input: "a,c", expected: []string{"a,null,c", "null,null,c"}},
				{input: "b", expected: []string{"null,b,null"}},
				{input: "c", expected: []string{"null,null,c"}},
				{input: "a,x,b", expected: []string{"null,b,null"}},
			},
		},
		{
			name:     "optionalWholeSequence",
			measures: []string{"a", "b", "c"},
			pattern:  RowSequence(RowVar("A"), RowVar("B").Optional(), RowVar("C")).Optional(),
			cases: []rowRecogRegexCase{
				{input: "x"},
				{input: "a"},
				{input: "a,c", expected: []string{"a,null,c"}},
				{input: "a,b,c", expected: []string{"a,b,c"}},
			},
		},
		{
			name:     "optionalPrefixAndSuffix",
			measures: []string{"a", "b", "c"},
			pattern:  RowSequence(RowVar("A").Optional(), RowVar("B"), RowVar("C").Optional()),
			cases: []rowRecogRegexCase{
				{input: "x"},
				{input: "a"},
				{input: "a,c"},
				{input: "b", expected: []string{"null,b,null"}},
				{input: "a,b,c", expected: []string{"a,b,null", "null,b,null", "a,b,c", "null,b,c"}},
			},
		},
		{
			name:     "repeatedPrefixBeforeTail",
			measures: []string{"a[0]", "b[0]", "a[1]", "b[1]", "c", "d"},
			pattern: RowSequence(
				RowSequence(RowVar("A"), RowVar("B")).ZeroOrMore(),
				RowVar("C"),
				RowVar("D"),
			),
			cases: []rowRecogRegexCase{
				{input: "c,d", expected: []string{"null,null,null,null,c,d"}},
				{input: "a1,b1,c,d", expected: []string{"a1,b1,null,null,c,d", "null,null,null,null,c,d"}},
				{input: "a2,b2,x,c,d", expected: []string{"null,null,null,null,c,d"}},
				{input: "a1,b1,a2,b2,c,d", expected: []string{"null,null,null,null,c,d", "a2,b2,null,null,c,d", "a1,b1,a2,b2,c,d"}},
			},
		},
		{
			name:     "nestedRepeatedGroups",
			measures: []string{"a[0]", "b[0]", "c[0]", "a[1]", "b[1]", "c[1]", "d[0]", "e[0]", "d[1]", "e[1]"},
			pattern: RowSequence(
				RowSequence(RowVar("A"), RowSequence(RowVar("B"), RowVar("C")).ZeroOrMore()).ZeroOrMore(),
				RowSequence(RowVar("D"), RowVar("E")).OneOrMore(),
			),
			cases: []rowRecogRegexCase{
				{input: "a,b,c"},
				{input: "d,e", expected: []string{"null,null,null,null,null,null,d,e,null,null"}},
				{input: "a,b,c,d,e", expected: []string{"a,b,c,null,null,null,d,e,null,null", "null,null,null,null,null,null,d,e,null,null"}},
				{input: "a1,b1,c1,a2,b2,c2,d,e", expected: []string{"a1,b1,c1,a2,b2,c2,d,e,null,null", "a2,b2,c2,null,null,null,d,e,null,null", "null,null,null,null,null,null,d,e,null,null"}},
				{input: "d1,e1,d2,e2", expected: []string{"null,null,null,null,null,null,d1,e1,null,null", "null,null,null,null,null,null,d2,e2,null,null", "null,null,null,null,null,null,d1,e1,d2,e2"}},
			},
		},
		{
			name:     "repeatedPrefixAndRepeatedTail",
			measures: []string{"a[0]", "a[1]", "d[0]", "e[0]", "d[1]", "e[1]"},
			pattern: RowSequence(
				RowVar("A").OneOrMore(),
				RowSequence(RowVar("D"), RowVar("E")).OneOrMore(),
			),
			cases: []rowRecogRegexCase{
				{input: "a,e,a,d,d,e,a,e,e,a,d,d,e,d,e"},
				{input: "a,d,e", expected: []string{"a,null,d,e,null,null"}},
				{input: "a1,a2,d,e", expected: []string{"a1,a2,d,e,null,null", "a2,null,d,e,null,null"}},
				{input: "a1,d1,e1,d2,e2", expected: []string{"a1,null,d1,e1,null,null", "a1,null,d1,e1,d2,e2"}},
			},
		},
		{
			name:     "nestedAlternatives",
			measures: []string{"a", "b", "c", "d", "e", "f"},
			pattern: RowAlternation(
				RowSequence(RowVar("A"), RowAlternation(RowVar("B"), RowVar("C"))),
				RowSequence(RowVar("D"), RowAlternation(RowVar("E"), RowVar("F"))),
			),
			cases: []rowRecogRegexCase{
				{input: "a,e,d,b,a,f,f,d,c,a"},
				{input: "a,f,c,b,a,d,f", expected: []string{"null,null,null,d,null,f"}},
				{input: "c,b,d,a,b,x,y", expected: []string{"a,b,null,null,null,null"}},
				{input: "a,d,c,f,d,e,x,a,c", expected: []string{"null,null,null,d,e,null", "a,null,c,null,null,null"}},
			},
		},
		{
			name:     "optionalNestedAlternatives",
			measures: []string{"a", "b", "c", "d", "e", "f"},
			pattern: RowAlternation(
				RowSequence(RowVar("A"), RowAlternation(RowVar("B"), RowVar("C")).Optional()),
				RowSequence(RowVar("D").Optional(), RowAlternation(RowVar("E"), RowVar("F"))),
			),
			cases: []rowRecogRegexCase{
				{input: "a1,f1,c,b,a2,d,f2", expected: []string{"a1,null,null,null,null,null", "a2,null,null,null,null,null", "null,null,null,null,null,f1", "null,null,null,null,null,f2", "null,null,null,d,null,f2"}},
				{input: "d,f", expected: []string{"null,null,null,d,null,f", "null,null,null,null,null,f"}},
			},
		},
		{
			name:     "fixedAndRepeatedAlternative",
			measures: []string{"a[0]", "a[1]", "b", "c", "d"},
			pattern: RowAlternation(
				RowSequence(RowVar("A"), RowVar("B"), RowVar("C")),
				RowSequence(RowVar("A").OneOrMore(), RowVar("B"), RowVar("D")),
			),
			cases: []rowRecogRegexCase{
				{input: "a1,c,a2,b,d", expected: []string{"a2,null,b,null,d"}},
				{input: "a1,b1,x,a2,b2,c1", expected: []string{"a2,null,b2,c1,null"}},
			},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			for _, testCase := range scenario.cases {
				testCase := testCase
				caseName := strings.ReplaceAll(testCase.input, ",", "_")
				t.Run(caseName, func(t *testing.T) {
					runRowRecogRegexCase(t, scenario, testCase)
				})
			}
		})
	}
}

func runRowRecogRegexCase(t *testing.T, scenario rowRecogRegexScenario, testCase rowRecogRegexCase) {
	t.Helper()
	env, engine := newRowRecogTest(t)
	stream := From[rowRecogTestEvent](env, "RowRecogEvent").Window(KeepAll())
	query := stream.MatchRecognize(scenario.pattern)
	for _, measure := range scenario.measures {
		tag := strings.ToUpper(measure)
		if index := strings.IndexByte(tag, '['); index >= 0 {
			tag = tag[:index]
		}
		query = query.Define(tag, StartsWith(Field[rowRecogTestEvent, string]("symbol"), Literal(strings.ToLower(tag))))
	}
	selections := make([]Selection, 0, len(scenario.measures))
	for _, measure := range scenario.measures {
		selections = append(selections, rowRecogRegexMeasure(measure))
	}
	builtQuery := query.AllMatches().SkipToCurrentRow().Measures(selections...).Query(StatementName("rowrecog-regex-" + scenario.name))
	plan, err := env.Build(builtQuery)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	rows := collectRowRecogRows(t, deployment)
	for index, symbol := range strings.Split(testCase.input, ",") {
		if err := engine.SendEvent(context.Background(), rowRecogTestEvent{Symbol: symbol, Price: float64(index)}); err != nil {
			t.Fatal(err)
		}
	}
	want := rowRecogRegexExpectedKeys(testCase.expected)
	got := rowRecogRegexRowKeys(*rows, scenario.measures)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("listener rows for %q = %#v, want %#v", testCase.input, got, want)
	}
	snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got = rowRecogRegexResultKeys(t, snapshot.Results(), scenario.measures)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("iterator rows for %q = %#v, want %#v", testCase.input, got, want)
	}
}

func rowRecogRegexMeasure(measure string) Selection {
	alias := strings.ReplaceAll(strings.ReplaceAll(measure, "[", ""), "]", "")
	tag := strings.ToUpper(measure)
	index := -1
	if open := strings.IndexByte(tag, '['); open >= 0 {
		close := strings.IndexByte(tag[open+1:], ']')
		if close >= 0 {
			close += open + 1
			fmt.Sscanf(tag[open+1:close], "%d", &index)
			tag = tag[:open]
		}
	}
	if index >= 0 {
		return Alias(alias, TagFieldAt[string](tag, index, "symbol"))
	}
	return Alias(alias, TagField[string](tag, "symbol"))
}

func rowRecogRegexExpectedKeys(values []string) []string {
	keys := make([]string, 0, len(values))
	for _, value := range values {
		fields := strings.Split(value, ",")
		for index, field := range fields {
			if field == "null" {
				fields[index] = ""
			}
		}
		keys = append(keys, strings.Join(fields, "/"))
	}
	return sortedStrings(keys)
}

func rowRecogRegexRowKeys(rows []Row, measures []string) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		fields := make([]string, 0, len(measures))
		for _, measure := range measures {
			alias := strings.ReplaceAll(strings.ReplaceAll(measure, "[", ""), "]", "")
			value := row.Get(alias)
			if !value.IsPresent() {
				fields = append(fields, "")
				continue
			}
			fields = append(fields, fmt.Sprint(value.Any()))
		}
		keys = append(keys, strings.Join(fields, "/"))
	}
	return sortedStrings(keys)
}

func rowRecogRegexResultKeys(t *testing.T, results []Result, measures []string) []string {
	t.Helper()
	rows := make([]Row, 0, len(results))
	for _, result := range results {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("row-recognize iterator result is not a row: %#v", result)
		}
		rows = append(rows, row)
	}
	return rowRecogRegexRowKeys(rows, measures)
}
