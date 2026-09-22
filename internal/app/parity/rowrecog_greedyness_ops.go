package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for RowRecogGreedyness (ords 0-2) and RowRecogOps (ords
// 5/7/9): reluctant quantifiers (A?? B?, A*? B? C, A+? B? C) that bind the
// minimum needed, partition state growth past the initial collection size
// (1000 partitions), alternation inside concatenation ((A|B)(C|D)) under all
// matches, and the JVM-only String.matches sanity check pinned as
// unrepresentable.
// Approved differences: Go has no EPL text; each pinned deploy EPL maps to
// the fluent MatchRecognize builder; the Java assertPropsPerRowIterator maps
// to statement.Snapshot; ord9 exercises no Esper runtime surface and pins a
// JVM-only note via the unrepresentable op.
var rowRecogGreedynessOpsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogGreedyness.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogOps.java",
}

var rowRecogGreedynessOpsJavaRuntimeIDs = []string{
	"java-runtime-6db0d9ba9099e88cb87c", // RowRecogReluctantZeroToOne
	"java-runtime-20afb4135dd06ce5fb8c", // RowRecogReluctantZeroToMany
	"java-runtime-470384aabd98754fe4f4", // RowRecogReluctantOneToMany
	"java-runtime-ca4a83fa8b88dd4d2b32", // RowRecogUnlimitedPartition
	"java-runtime-2e6e0566fe07043c23c7", // RowRecogAlterWithinConcat
	"java-runtime-c489386411c57214eb75", // RowRecogRegex
}

var rowRecogGreedynessOpsJavaExecutions = []string{
	"RowRecogReluctantZeroToOne",
	"RowRecogReluctantZeroToMany",
	"RowRecogReluctantOneToMany",
	"RowRecogUnlimitedPartition",
	"RowRecogAlterWithinConcat",
	"RowRecogRegex",
}

const rowRecogGreedynessOpsID = "rowrecog-greedyness-ops"

type rowRecogGreedynessOpsBean struct {
	TheString string `esper:"theString"`
	Value     int    `esper:"value"`
}

func rowRecogGreedynessOpsRuntimeURI(caseName string) string {
	return rowRecogGreedynessOpsID + "-" + caseName
}

func runRowRecogGreedynessOpsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"reluctant-zero-to-one",
		"reluctant-zero-to-many",
		"reluctant-one-to-many",
		"unlimited-partition",
		"alter-within-concat",
		"regex",
	} {
		caseTrace, err := runRowRecogGreedynessOpsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog-greedyness-ops case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogGreedynessOpsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rowRecogGreedynessOpsBean](env, "SupportRecogBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(rowRecogGreedynessOpsRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: rowRecogGreedynessOpsID}
	var sequence uint64
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, err := rowRecogGreedynessOpsQueryForEPL(env, caseName, epl)
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
					return trace, fmt.Errorf("%s undeploy %q: %w", rowRecogGreedynessOpsID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			var bean rowRecogGreedynessOpsBean
			if err := json.Unmarshal(step.Payload, &bean); err != nil {
				return trace, fmt.Errorf("%s decode SupportRecogBean: %w", rowRecogGreedynessOpsID, err)
			}
			if err := engine.Send(ctx, step.EventType, bean); err != nil {
				return trace, err
			}
		case "snapshot":
			if deployment == nil {
				return trace, fmt.Errorf("%s snapshot without deployment", rowRecogGreedynessOpsID)
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
		case "unrepresentable":
			// ord9 RowRecogRegex asserts JVM String.matches semantics only;
			// no Esper runtime surface exists to replay, so the pinned note
			// records the coverage boundary.
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "unrepresentable",
				Statement: step.Statement,
				Value:     step.ExpectError,
			})
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", rowRecogGreedynessOpsID, step.Op)
		}
	}
	return trace, nil
}

// rowRecogGreedynessOpsQueryForEPL maps each pinned deploy EPL to the fluent
// MatchRecognize builder. The EPL text is pinned verbatim from the Java
// source; the builder mirrors the same semantics. Non-all-matches EPLs need
// FirstMatch because the Go builder defaults to all matches.
func rowRecogGreedynessOpsQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	stream := esper.From[rowRecogGreedynessOpsBean](env, "SupportRecogBean").Window(esper.KeepAll())
	theString := esper.Field[rowRecogGreedynessOpsBean, string]("theString")
	value := esper.Field[rowRecogGreedynessOpsBean, int]("value")
	A := esper.RowVar("A")
	B := esper.RowVar("B")
	C := esper.RowVar("C")
	D := esper.RowVar("D")

	switch caseName {
	case "reluctant-zero-to-one":
		// measures A.theString as a_string, B.theString as b_string
		// pattern (A?? B?) define A as A.value = 1, B as B.value = 1
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string   pattern (A?? B?)   define    A as A.value = 1,   B as B.value = 1)" {
			return stream.MatchRecognize(esper.RowSequence(
				A.Optional().Reluctant(),
				B.Optional(),
			)).
				Define("A", esper.Equal[int](value, esper.Literal(1))).
				Define("B", esper.Equal[int](value, esper.Literal(1))).
				FirstMatch().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "reluctant-zero-to-many":
		// measures A[0..2].theString as a0/a1/a2, B.theString as b, C.theString as c
		// pattern (A*? B? C) define A v=1, B v in (1,2), C v=3
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A[0].theString as a0, A[1].theString as a1, A[2].theString as a2, B.theString as b, C.theString as c  pattern (A*? B? C)   define    A as A.value = 1,   B as B.value in (1, 2),   C as C.value = 3)" {
			return stream.MatchRecognize(esper.RowSequence(
				A.ZeroOrMore().Reluctant(),
				B.Optional(),
				C,
			)).
				Define("A", esper.Equal[int](value, esper.Literal(1))).
				Define("B", esper.In[int](value, esper.Literal(1), esper.Literal(2))).
				Define("C", esper.Equal[int](value, esper.Literal(3))).
				FirstMatch().
				Measures(
					esper.Alias("a0", esper.TagFieldAt[string]("A", 0, "theString")),
					esper.Alias("a1", esper.TagFieldAt[string]("A", 1, "theString")),
					esper.Alias("a2", esper.TagFieldAt[string]("A", 2, "theString")),
					esper.Alias("b", esper.TagField[string]("B", "theString")),
					esper.Alias("c", esper.TagField[string]("C", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "reluctant-one-to-many":
		// same measures, pattern (A+? B? C)
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A[0].theString as a0, A[1].theString as a1, A[2].theString as a2, B.theString as b, C.theString as c  pattern (A+? B? C)   define    A as A.value = 1,   B as B.value in (1, 2),   C as C.value = 3)" {
			return stream.MatchRecognize(esper.RowSequence(
				A.OneOrMore().Reluctant(),
				B.Optional(),
				C,
			)).
				Define("A", esper.Equal[int](value, esper.Literal(1))).
				Define("B", esper.In[int](value, esper.Literal(1), esper.Literal(2))).
				Define("C", esper.Equal[int](value, esper.Literal(3))).
				FirstMatch().
				Measures(
					esper.Alias("a0", esper.TagFieldAt[string]("A", 0, "theString")),
					esper.Alias("a1", esper.TagFieldAt[string]("A", 1, "theString")),
					esper.Alias("a2", esper.TagFieldAt[string]("A", 2, "theString")),
					esper.Alias("b", esper.TagField[string]("B", "theString")),
					esper.Alias("c", esper.TagField[string]("C", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "unlimited-partition":
		// partition by value measures A.theString as a_string pattern (A B)
		// define A as (A.theString = 'A'), B as (B.theString = 'B')
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  partition by value  measures A.theString as a_string   pattern (A B)   define     A as (A.theString = 'A'),    B as (B.theString = 'B'))" {
			return stream.MatchRecognize(esper.RowSequence(A, B)).
				Define("A", esper.Equal[string](theString, esper.Literal("A"))).
				Define("B", esper.Equal[string](theString, esper.Literal("B"))).
				FirstMatch().
				PartitionBy(value).
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "alter-within-concat":
		// measures A..D theString all matches pattern ( (A | B) (C | D) )
		// define A v=1, B v=2, C v=3, D v=4
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string, C.theString as c_string, D.theString as d_string   all matches pattern ( (A | B) (C | D) )   define     A as (A.value = 1),    B as (B.value = 2),    C as (C.value = 3),    D as (D.value = 4))" {
			return stream.MatchRecognize(esper.RowSequence(
				esper.RowAlternation(A, B),
				esper.RowAlternation(C, D),
			)).
				Define("A", esper.Equal[int](value, esper.Literal(1))).
				Define("B", esper.Equal[int](value, esper.Literal(2))).
				Define("C", esper.Equal[int](value, esper.Literal(3))).
				Define("D", esper.Equal[int](value, esper.Literal(4))).
				AllMatches().
				Measures(
					esper.Alias("a_string", esper.TagField[string]("A", "theString")),
					esper.Alias("b_string", esper.TagField[string]("B", "theString")),
					esper.Alias("c_string", esper.TagField[string]("C", "theString")),
					esper.Alias("d_string", esper.TagField[string]("D", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogGreedynessOpsID, caseName, epl)
}
