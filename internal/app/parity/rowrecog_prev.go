package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for RowRecogPrev (ords 0-4) and RowRecogDataSet
// RowRecogExampleWithPREV: prev() inside match_recognize DEFINE over keepall
// and time-window sources, partitioned and unpartitioned, plain and indexed
// offsets, multi-index prev on string and numeric fields, prev(x,0) under
// Math.abs, IN over prev values, prev history surviving time-window
// eviction, and the 20-measure-column financial pattern with
// skip-to-current-row.
// Approved differences: Go has no EPL text; each pinned deploy EPL maps to
// the fluent MatchRecognize builder; Java sendTimer maps to advance-time
// steps; assertPropsPerRowIterator maps to statement.Snapshot; the
// unpartitioned-keepall second deployment maps to undeploy-all + deploy.
var rowRecogPrevJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogPrev.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogDataSet.java",
}

var rowRecogPrevJavaRuntimeIDs = []string{
	"java-runtime-c096e55f89e15fcc54b7", // RowRecogTimeWindowPartitionedSimple
	"java-runtime-9c42dd39d70d409a64d7", // RowRecogPartitionBy2FieldsKeepall
	"java-runtime-1200bc1d6899155bac4a", // RowRecogUnpartitionedKeepAll
	"java-runtime-241336e6d46c14fbcf06", // RowRecogTimeWindowUnpartitioned
	"java-runtime-59f7afa17be17f57fbd5", // RowRecogTimeWindowPartitioned
	"java-runtime-41c5b0a59540aec34895", // RowRecogExampleWithPREV
}

var rowRecogPrevJavaExecutions = []string{
	"RowRecogTimeWindowPartitionedSimple",
	"RowRecogPartitionBy2FieldsKeepall",
	"RowRecogUnpartitionedKeepAll",
	"RowRecogTimeWindowUnpartitioned",
	"RowRecogTimeWindowPartitioned",
	"RowRecogExampleWithPREV",
}

const rowRecogPrevID = "rowrecog-prev"

type rowRecogPrevBean struct {
	TheString string  `esper:"theString"`
	Value     int     `esper:"value"`
	Cat       *string `esper:"cat"`
}

func rowRecogPrevRuntimeURI(caseName string) string {
	return rowRecogPrevID + "-" + caseName
}

func runRowRecogPrevScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"timewindow-partitioned-simple",
		"partition-by-2-fields-keepall",
		"unpartitioned-keepall",
		"timewindow-unpartitioned",
		"timewindow-partitioned",
		"example-with-prev",
	} {
		caseTrace, err := runRowRecogPrevCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog-prev case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogPrevCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rowRecogPrevBean](env, "SupportRecogBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(rowRecogPrevRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: rowRecogPrevID}
	var sequence uint64
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, err := rowRecogPrevQueryForEPL(env, caseName, epl)
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
					return trace, fmt.Errorf("%s undeploy %q: %w", rowRecogPrevID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			var bean rowRecogPrevBean
			if err := json.Unmarshal(step.Payload, &bean); err != nil {
				return trace, fmt.Errorf("%s decode SupportRecogBean: %w", rowRecogPrevID, err)
			}
			if err := engine.Send(ctx, step.EventType, bean); err != nil {
				return trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "snapshot":
			if deployment == nil {
				return trace, fmt.Errorf("%s snapshot without deployment", rowRecogPrevID)
			}
			result, err := deployment.Statements()[0].Snapshot(ctx)
			if err != nil {
				return trace, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: "s0",
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
			}
			// Java omits 'new' for an empty iterator; mirror that shape.
			if rows != nil {
				record.New = rows
			}
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", rowRecogPrevID, step.Op)
		}
	}
	return trace, nil
}

// rowRecogPrevQueryForEPL maps each pinned deploy EPL to the fluent
// MatchRecognize builder. The EPL text is pinned verbatim from the Java
// source; the builder mirrors the same semantics.
func rowRecogPrevQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	theString := esper.Field[rowRecogPrevBean, string]("theString")
	value := esper.Field[rowRecogPrevBean, int]("value")
	cat := esper.Field[rowRecogPrevBean, *string]("cat")
	A := esper.RowVar("A")
	B := esper.RowVar("B")
	C := esper.RowVar("C")
	D := esper.RowVar("D")
	E := esper.RowVar("E")
	F := esper.RowVar("F")

	switch caseName {
	case "timewindow-partitioned-simple":
		// #time(5 sec) partition by cat measures A.cat as cat, A.theString as
		// a_string all matches pattern (A) define A as PREV(A.value) =
		// (A.value - 1) order by a_string
		if epl == "@name('s0') select * from SupportRecogBean#time(5 sec) match_recognize (  partition by cat   measures A.cat as cat, A.theString as a_string  all matches pattern (A)   define     A as PREV(A.value) = (A.value - 1)) order by a_string" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.TimeWindow(5*time.Second)).
				MatchRecognize(A).
				Define("A", esper.Equal[int](
					esper.PrevTag[int](1, "A", "value"),
					esper.Subtract[int](value, esper.Literal(1)),
				)).
				AllMatches().
				PartitionBy(cat).
				Measures(
					esper.Alias("cat", esper.TagField[*string]("A", "cat")),
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
				).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(esper.Ascending(esper.ResultField[string]("a_string"))),
				), nil
		}
	case "partition-by-2-fields-keepall":
		// keepall partition by theString, cat measures a_string/a_cat/
		// a_value/b_value all matches pattern (A B) define A as
		// (A.value > PREV(A.value)), B as (B.value > PREV(B.value))
		// order by a_string, a_cat
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  partition by theString, cat  measures A.theString as a_string, A.cat as a_cat, A.value as a_value, B.value as b_value   all matches pattern (A B)   define     A as (A.value > PREV(A.value)),    B as (B.value > PREV(B.value))) order by a_string, a_cat" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B)).
				Define("A", esper.Greater[int](value, esper.PrevTag[int](1, "A", "value"))).
				Define("B", esper.Greater[int](value, esper.PrevTag[int](1, "B", "value"))).
				AllMatches().
				PartitionBy(theString, cat).
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("a_cat", esper.TagField[*string]("A", "cat")),
					esper.Alias("a_value", esper.TagField[int]("A", "value")),
					esper.Alias("b_value", esper.TagField[int]("B", "value")),
				).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(
						esper.Ascending(esper.ResultField[string]("a_string")),
						esper.Ascending(esper.ResultField[*string]("a_cat")),
					),
				), nil
		}
	case "unpartitioned-keepall":
		// keepall measures a_string all matches pattern (A) order by
		// a_string; two sequential deployments: plain prev then indexed
		// prev(A.value, 2) = 5.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string  all matches pattern (A)   define A as (A.value > PREV(A.value))) order by a_string" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(A).
				Define("A", esper.Greater[int](value, esper.PrevTag[int](1, "A", "value"))).
				AllMatches().
				Measures(esper.Alias("a_string", esper.TagField[string]("A", "theString"))).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(esper.Ascending(esper.ResultField[string]("a_string"))),
				), nil
		}
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string  all matches pattern (A)   define A as (PREV(A.value, 2) = 5)) order by a_string" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(A).
				Define("A", esper.Equal[int](esper.PrevTag[int](2, "A", "value"), esper.Literal(5))).
				AllMatches().
				Measures(esper.Alias("a_string", esper.TagField[string]("A", "theString"))).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(esper.Ascending(esper.ResultField[string]("a_string"))),
				), nil
		}
	case "timewindow-unpartitioned":
		// #time(5) measures a_string/b_string all matches pattern (A B)
		// define A as PREV(A.theString,3)='P3' and PREV(A.theString,2)='P2'
		// and PREV(A.theString,4)='P4' and Math.abs(prev(A.value,0))>=0,
		// B as B.value in (PREV(B.value,4), PREV(B.value,2))
		if epl == "@name('s0') select * from SupportRecogBean#time(5) match_recognize (  measures A.theString as a_string, B.theString as b_string  all matches pattern (A B)   define     A as PREV(A.theString, 3) = 'P3' and PREV(A.theString, 2) = 'P2' and PREV(A.theString, 4) = 'P4' and Math.abs(prev(A.value, 0)) >= 0,    B as B.value in (PREV(B.value, 4), PREV(B.value, 2)))" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.TimeWindow(5*time.Second)).
				MatchRecognize(esper.RowSequence(A, B)).
				Define("A", esper.And(
					esper.And(
						esper.Equal[string](esper.PrevTag[string](3, "A", "theString"), esper.Literal("P3")),
						esper.Equal[string](esper.PrevTag[string](2, "A", "theString"), esper.Literal("P2")),
					),
					esper.And(
						esper.Equal[string](esper.PrevTag[string](4, "A", "theString"), esper.Literal("P4")),
						esper.GreaterOrEqual[int](esper.Abs[int](esper.PrevTag[int](0, "A", "value")), esper.Literal(0)),
					),
				)).
				Define("B", esper.In[int](value,
					esper.PrevTag[int](4, "B", "value"),
					esper.PrevTag[int](2, "B", "value"),
				)).
				AllMatches().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "timewindow-partitioned":
		// same shape plus partition by cat, measures add A.cat, order by cat
		if epl == "@name('s0') select * from SupportRecogBean#time(5) match_recognize (  partition by cat  measures A.cat as cat, A.theString as a_string, B.theString as b_string  all matches pattern (A B)   define     A as PREV(A.theString, 3) = 'P3' and PREV(A.theString, 2) = 'P2' and PREV(A.theString, 4) = 'P4',    B as B.value in (PREV(B.value, 4), PREV(B.value, 2))) order by cat" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.TimeWindow(5*time.Second)).
				MatchRecognize(esper.RowSequence(A, B)).
				Define("A", esper.And(
					esper.Equal[string](esper.PrevTag[string](3, "A", "theString"), esper.Literal("P3")),
					esper.And(
						esper.Equal[string](esper.PrevTag[string](2, "A", "theString"), esper.Literal("P2")),
						esper.Equal[string](esper.PrevTag[string](4, "A", "theString"), esper.Literal("P4")),
					),
				)).
				Define("B", esper.In[int](value,
					esper.PrevTag[int](4, "B", "value"),
					esper.PrevTag[int](2, "B", "value"),
				)).
				AllMatches().
				PartitionBy(cat).
				Measures(
					esper.Alias("cat", esper.TagField[*string]("A", "cat")),
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
				).
				Query(
					esper.StatementName("s0"),
					esper.OrderBy(esper.Ascending(esper.ResultField[*string]("cat"))),
				), nil
		}
	case "example-with-prev":
		// 20 measure columns, ALL MATCHES, after match skip to current row,
		// pattern (A B C* D E* F+), A undefined (matches any), defines
		// B/C/D/E/F with prev() comparisons.
		if epl == "@name('s0') SELECT * FROM SupportRecogBean#keepall   MATCH_RECOGNIZE (       MEASURES A.theString AS a_string,         A.value AS a_value,         B.theString AS b_string,         B.value AS b_value,         C[0].theString AS c0_string,         C[0].value AS c0_value,         C[1].theString AS c1_string,         C[1].value AS c1_value,         C[2].theString AS c2_string,         C[2].value AS c2_value,         D.theString AS d_string,         D.value AS d_value,         E[0].theString AS e0_string,         E[0].value AS e0_value,         E[1].theString AS e1_string,         E[1].value AS e1_value,         F[0].theString AS f0_string,         F[0].value AS f0_value,         F[1].theString AS f1_string,         F[1].value AS f1_value       ALL MATCHES       after match skip to current row       PATTERN ( A B C* D E* F+ )       DEFINE /* A is unspecified, defaults to TRUE, matches any row */            B AS (B.value < PREV (B.value)),            C AS (C.value <= PREV (C.value)),            D AS (D.value < PREV (D.value)),            E AS (E.value >= PREV (E.value)),            F AS (F.value >= PREV (F.value) and F.value > A.value))" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(
					A, B, C.ZeroOrMore(), D, E.ZeroOrMore(), F.OneOrMore(),
				)).
				Define("B", esper.Less[int](value, esper.PrevTag[int](1, "B", "value"))).
				Define("C", esper.LessOrEqual[int](value, esper.PrevTag[int](1, "C", "value"))).
				Define("D", esper.Less[int](value, esper.PrevTag[int](1, "D", "value"))).
				Define("E", esper.GreaterOrEqual[int](value, esper.PrevTag[int](1, "E", "value"))).
				Define("F", esper.And(
					esper.GreaterOrEqual[int](value, esper.PrevTag[int](1, "F", "value")),
					esper.Greater[int](value, esper.TagField[int]("A", "value")),
				)).
				AllMatches().
				SkipToCurrentRow().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("a_value", esper.TagField[int]("A", "value")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
					esper.Alias("b_value", esper.TagField[int]("B", "value")),
					esper.Alias("c0_string", esper.TagFieldAt[string]("C", 0, "theString")),
					esper.Alias("c0_value", esper.TagFieldAt[int]("C", 0, "value")),
					esper.Alias("c1_string", esper.TagFieldAt[string]("C", 1, "theString")),
					esper.Alias("c1_value", esper.TagFieldAt[int]("C", 1, "value")),
					esper.Alias("c2_string", esper.TagFieldAt[string]("C", 2, "theString")),
					esper.Alias("c2_value", esper.TagFieldAt[int]("C", 2, "value")),
					esper.Alias("d_string", esper.TagField[string]("D", "theString")),
					esper.Alias("d_value", esper.TagField[int]("D", "value")),
					esper.Alias("e0_string", esper.TagFieldAt[string]("E", 0, "theString")),
					esper.Alias("e0_value", esper.TagFieldAt[int]("E", 0, "value")),
					esper.Alias("e1_string", esper.TagFieldAt[string]("E", 1, "theString")),
					esper.Alias("e1_value", esper.TagFieldAt[int]("E", 1, "value")),
					esper.Alias("f0_string", esper.TagFieldAt[string]("F", 0, "theString")),
					esper.Alias("f0_value", esper.TagFieldAt[int]("F", 0, "value")),
					esper.Alias("f1_string", esper.TagFieldAt[string]("F", 1, "theString")),
					esper.Alias("f1_value", esper.TagFieldAt[int]("F", 1, "value")),
				).
				Query(esper.StatementName("s0")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogPrevID, caseName, epl)
}
