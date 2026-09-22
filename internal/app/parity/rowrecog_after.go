package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for RowRecogAfter (ords 0-5): AFTER MATCH SKIP semantics —
// skip-to-current-row extends the in-flight match, skip-to-next-row suppresses
// the listener callback on extension while the iterator still reflects it,
// all-matches chained rows, variable-twice patterns, partitioned matches, and
// skip-past-last-row non-overlap.
// Approved differences: Go has no EPL text; the deploy step's verbatim EPL is
// pinned per case and mapped to the fluent MatchRecognize builder; the Java
// assertPropsPerRowIterator maps to statement.Snapshot; ord0's second
// eplToModelCompileDeploy pass maps to undeploy+redeploy.
var rowRecogAfterJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogAfter.java",
}

var rowRecogAfterJavaRuntimeIDs = []string{
	"java-runtime-040bbf4e7c7c45f3cd76",
	"java-runtime-ad336d2b8e1195e4abf6",
	"java-runtime-ebf5f4119f980f7d9258",
	"java-runtime-3ff12edd4f2bb4ecec9b",
	"java-runtime-21259f97372ba54fe074",
	"java-runtime-a960711a30045575a880",
}

var rowRecogAfterJavaExecutions = []string{
	"RowRecogAfterCurrentRow",
	"RowRecogAfterNextRow",
	"RowRecogSkipToNextRow",
	"RowRecogVariableMoreThenOnce",
	"RowRecogSkipToNextRowPartitioned",
	"RowRecogAfterSkipPastLast",
}

const rowRecogAfterID = "rowrecog-after"

type rowRecogAfterBean struct {
	TheString string `esper:"theString" json:"theString"`
	Value     int    `esper:"value" json:"value"`
}

func rowRecogAfterRuntimeURI(caseName string) string {
	return rowRecogAfterID + "-" + caseName
}

func runRowRecogAfterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"after-current-row",
		"after-next-row",
		"skip-to-next-row",
		"variable-more-then-once",
		"skip-to-next-row-partitioned",
		"skip-past-last",
	} {
		caseTrace, err := runRowRecogAfterCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog-after case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("rowrecog-after scenario %q has no supported cases", scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogAfterCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rowRecogAfterBean](env, "SupportRecogBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(rowRecogAfterRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: rowRecogAfterID}
	var sequence uint64
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, err := rowRecogAfterQueryForEPL(env, caseName, epl)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err = engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		// Java attaches a fresh listener per deployment; the sequence resets.
		sequence = 0
		for _, st := range deployment.Statements() {
			st := st
			if _, subErr := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequence++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: st.Name(),
					Sequence:  sequence,
					Time:      compat.FormatTraceTime(batch.Time),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); subErr != nil {
				return subErr
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deploy(step.Epl); err != nil {
				return trace, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, fmt.Errorf("%s undeploy %q: %w", rowRecogAfterID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			var bean rowRecogAfterBean
			if err := json.Unmarshal(step.Payload, &bean); err != nil {
				return trace, fmt.Errorf("%s decode SupportRecogBean: %w", rowRecogAfterID, err)
			}
			if err := engine.Send(ctx, step.EventType, bean); err != nil {
				return trace, err
			}
		case "snapshot":
			if deployment == nil {
				return trace, fmt.Errorf("%s snapshot without deployment", rowRecogAfterID)
			}
			result, err := deployment.Statements()[0].Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: "s0",
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
				New:       rows,
			})
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", rowRecogAfterID, step.Op)
		}
	}
	return trace, nil
}

// rowRecogAfterQueryForEPL maps each pinned deploy EPL to the fluent
// MatchRecognize builder. The EPL text is pinned verbatim from the Java
// source; the builder mirrors the same semantics.
func rowRecogAfterQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	stream := esper.From[rowRecogAfterBean](env, "SupportRecogBean").Window(esper.KeepAll())
	value := esper.Field[rowRecogAfterBean, int]("value")
	theString := esper.Field[rowRecogAfterBean, string]("theString")

	switch caseName {
	case "after-current-row":
		// measures A.theString as a, B[0].theString as b0, B[1].theString as b1
		// after match skip to current row pattern (A B*)
		// define A as A.theString like "A%", B as B.theString like "B%"
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1 after match skip to current row pattern (A B*) define A as A.theString like \"A%\", B as B.theString like \"B%\")" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowVar("A"),
				esper.RowVar("B").ZeroOrMore(),
			)).
				Define("A", esper.Like(theString, esper.Literal("A%"))).
				Define("B", esper.Like(theString, esper.Literal("B%"))).
				FirstMatch().
				SkipToCurrentRow().
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "after-next-row":
		// same measures/pattern but AFTER MATCH SKIP TO NEXT ROW
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a, B[0].theString as b0, B[1].theString as b1  AFTER MATCH SKIP TO NEXT ROW   pattern (A B*)   define     A as A.theString like 'A%',    B as B.theString like 'B%')" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowVar("A"),
				esper.RowVar("B").ZeroOrMore(),
			)).
				Define("A", esper.Like(theString, esper.Literal("A%"))).
				Define("B", esper.Like(theString, esper.Literal("B%"))).
				FirstMatch().
				SkipToNextRow().
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "skip-to-next-row":
		// measures A.theString as a_string, B.theString as b_string all matches
		// after match skip to next row pattern (A B) define B as B.value > A.value
		// order by a_string, b_string
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string   all matches   after match skip to next row   pattern (A B)   define B as B.value > A.value) order by a_string, b_string" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowVar("A"),
				esper.RowVar("B"),
			)).
				Define("B", esper.Greater[int](value, esper.TagField[int]("A", "value"))).
				AllMatches().
				SkipToNextRow().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
				).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(
						esper.Ascending(esper.ResultField[string]("a_string")),
						esper.Ascending(esper.ResultField[string]("b_string")),
					),
				), nil
		}
	case "variable-more-then-once":
		// measures A[0].theString as a0, B.theString as b, A[1].theString as a1
		// all matches after match skip to next row pattern ( A B A )
		// define A as (A.value = 1), B as (B.value = 2)
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A[0].theString as a0, B.theString as b, A[1].theString as a1   all matches   after match skip to next row   pattern ( A B A )   define     A as (A.value = 1),    B as (B.value = 2))" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowVar("A"),
				esper.RowVar("B"),
				esper.RowVar("A"),
			)).
				Define("A", esper.Equal[int](value, esper.Literal(1))).
				Define("B", esper.Equal[int](value, esper.Literal(2))).
				AllMatches().
				SkipToNextRow().
				Measures(
					esper.Alias("a0", esper.TagFieldAt[string]("A", 0, "theString")),
					esper.Alias("b", esper.TagField[string]("B", "theString")),
					esper.Alias("a1", esper.TagFieldAt[string]("A", 1, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "skip-to-next-row-partitioned":
		// partition by theString measures A.theString as a_string, A.value as
		// a_value, B.value as b_value all matches after match skip to next row
		// pattern (A B) define B as (B.value > A.value) order by a_string
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  partition by theString  measures A.theString as a_string, A.value as a_value, B.value as b_value   all matches   after match skip to next row   pattern (A B)   define B as (B.value > A.value)) order by a_string" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowVar("A"),
				esper.RowVar("B"),
			)).
				PartitionBy(theString).
				Define("B", esper.Greater[int](value, esper.TagField[int]("A", "value"))).
				AllMatches().
				SkipToNextRow().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("a_value", esper.TagField[int]("A", "value")),
					esper.Alias("b_value", esper.TagField[int]("B", "value")),
				).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(esper.Ascending(esper.ResultField[string]("a_string"))),
				), nil
		}
	case "skip-past-last":
		// measures A.theString as a_string, B.theString as b_string all matches
		// after match skip past last row pattern (A B) define B as B.value > A.value
		// order by a_string, b_string
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string   all matches   after match skip past last row  pattern (A B)   define B as B.value > A.value) order by a_string, b_string" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowVar("A"),
				esper.RowVar("B"),
			)).
				Define("B", esper.Greater[int](value, esper.TagField[int]("A", "value"))).
				AllMatches().
				SkipPastLastRow().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
				).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(
						esper.Ascending(esper.ResultField[string]("a_string")),
						esper.Ascending(esper.ResultField[string]("b_string")),
					),
				), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogAfterID, caseName, epl)
}
