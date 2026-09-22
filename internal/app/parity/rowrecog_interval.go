package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for RowRecogInterval (ords 0-3) and
// RowRecogIntervalOrTerminated (ord 0): match_recognize interval semantics —
// begin-anchored inclusive deadlines, scheduled-vs-iterator visibility,
// partition-shared schedule keys, silent strand death on misfit, calendar
// month intervals, and the or-terminated family (termination claims,
// dead-end immediate emission, alternation branch survival, mandatory
// quantifier silent death, all-matches batching, object-array doc sample).
// Approved differences: Go has no EPL text; each pinned deploy EPL maps to
// the fluent MatchRecognize builder; Java sendTimer maps to advance-time
// steps; assertPropsPerRowIterator maps to statement.Snapshot; each
// or-terminated sub-assertion maps to its own case (fresh engine + clock).
var rowRecogIntervalJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogInterval.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogIntervalOrTerminated.java",
}

var rowRecogIntervalJavaRuntimeIDs = []string{
	"java-runtime-02e57a7f151323edc4be", // RowRecogIntervalSimple
	"java-runtime-b9a2b0afe61a2bfabcd0", // RowRecogPartitioned
	"java-runtime-1361f7530aa059720913", // RowRecogMultiCompleted
	"java-runtime-373cb011a639d99148f0", // RowRecogMonthScoped
	"java-runtime-bd18c2cfd8ff34a0b5a3", // RowRecogIntervalOrTerminated
}

var rowRecogIntervalJavaExecutions = []string{
	"RowRecogIntervalSimple",
	"RowRecogPartitioned",
	"RowRecogMultiCompleted",
	"RowRecogMonthScoped",
	"RowRecogIntervalOrTerminated",
}

const rowRecogIntervalID = "rowrecog-interval"

// rowRecogIntervalSupportBean mirrors SupportBean{theString, intPrimitive}
// for the month-scoped case.
type rowRecogIntervalSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func rowRecogIntervalRuntimeURI(caseName string) string {
	return rowRecogIntervalID + "-" + caseName
}

func runRowRecogIntervalScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"interval-simple",
		"interval-partitioned",
		"interval-multicompleted",
		"interval-monthscoped",
		"orterminated-doc-sample",
		"orterminated-a-b",
		"orterminated-a-bstar",
		"orterminated-a-bstar-allmatches",
		"orterminated-a-bstar-or-c",
		"orterminated-a-bstar-or-cstar",
		"orterminated-a-b-cstar",
		"orterminated-a-bplus",
		"orterminated-astar",
		"orterminated-a-parens-bstar",
	} {
		caseTrace, err := runRowRecogIntervalCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog-interval case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogIntervalCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rowRecogPrevBean](env, "SupportRecogBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[rowRecogIntervalSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterObjectArray(env, "TemperatureSensorEvent", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf("")),
		esper.FieldDef("device", reflect.TypeOf(0)),
		esper.FieldDef("temp", reflect.TypeOf(float64(0))),
	}); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(rowRecogIntervalRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: rowRecogIntervalID}
	var sequence uint64
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, err := rowRecogIntervalQueryForEPL(env, caseName, epl)
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
					return trace, fmt.Errorf("%s undeploy %q: %w", rowRecogIntervalID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			if err := rowRecogIntervalSend(ctx, engine, step); err != nil {
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
				return trace, fmt.Errorf("%s snapshot without deployment", rowRecogIntervalID)
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
			return trace, fmt.Errorf("%s unsupported step op %q", rowRecogIntervalID, step.Op)
		}
	}
	return trace, nil
}

func rowRecogIntervalSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportRecogBean":
		var bean rowRecogPrevBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return fmt.Errorf("%s decode SupportRecogBean: %w", rowRecogIntervalID, err)
		}
		return engine.Send(ctx, step.EventType, bean)
	case "SupportBean":
		var bean rowRecogIntervalSupportBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return fmt.Errorf("%s decode SupportBean: %w", rowRecogIntervalID, err)
		}
		return engine.Send(ctx, step.EventType, bean)
	case "TemperatureSensorEvent":
		var raw []any
		if err := json.Unmarshal(step.Payload, &raw); err != nil {
			return fmt.Errorf("%s decode TemperatureSensorEvent: %w", rowRecogIntervalID, err)
		}
		if len(raw) != 3 {
			return fmt.Errorf("%s TemperatureSensorEvent payload must have 3 fields", rowRecogIntervalID)
		}
		id, ok := raw[0].(string)
		if !ok {
			return fmt.Errorf("%s TemperatureSensorEvent id must be a string", rowRecogIntervalID)
		}
		device, ok := raw[1].(float64)
		if !ok {
			return fmt.Errorf("%s TemperatureSensorEvent device must be a number", rowRecogIntervalID)
		}
		temp, ok := raw[2].(float64)
		if !ok {
			return fmt.Errorf("%s TemperatureSensorEvent temp must be a number", rowRecogIntervalID)
		}
		return engine.SendObjectArray(ctx, step.EventType, []any{id, int(device), temp})
	default:
		return fmt.Errorf("%s unsupported event type %q", rowRecogIntervalID, step.EventType)
	}
}

// rowRecogIntervalQueryForEPL maps each pinned deploy EPL to the fluent
// MatchRecognize builder. The EPL text is pinned verbatim from the Java
// source; the builder mirrors the same semantics.
func rowRecogIntervalQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	theString := esper.Field[rowRecogPrevBean, string]("theString")
	cat := esper.Field[rowRecogPrevBean, *string]("cat")
	A := esper.RowVar("A")
	B := esper.RowVar("B")
	C := esper.RowVar("C")

	likeA := func(field esper.Expression[string]) esper.Expr {
		return esper.Like(field, esper.Literal("A%"))
	}
	likeB := func(field esper.Expression[string]) esper.Expr {
		return esper.Like(field, esper.Literal("B%"))
	}
	likeC := func(field esper.Expression[string]) esper.Expr {
		return esper.Like(field, esper.Literal("C%"))
	}

	// measuresABStarLast emits the shared a/b0/b1/lastb measure set for the
	// (A B*) interval cases.
	measuresABStarLast := func() []esper.Selection {
		return []esper.Selection{
			esper.Alias("a", esper.TagField[string]("A", "theString")),
			esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
			esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
			esper.Alias("lastb", esper.TagLast[string]("B", theString)),
		}
	}
	orderAB := esper.OrderBy(
		esper.Ascending(esper.ResultField[string]("a")),
		esper.Ascending(esper.ResultField[string]("b0")),
		esper.Ascending(esper.ResultField[string]("b1")),
		esper.Ascending(esper.ResultField[string]("lastb")),
	)

	switch caseName {
	case "interval-simple":
		// keepall measures a/b0/b1/lastb pattern (A B*) interval 10 seconds
		// order by a,b0,b1,lastb; deployed twice (compileDeploy +
		// eplToModelCompileDeploy paths).
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, last(B.theString) as lastb pattern (A B*) interval 10 seconds define A as A.theString like \"A%\", B as B.theString like \"B%\") order by a, b0, b1, lastb" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				Interval(10*time.Second).
				Measures(measuresABStarLast()...).
				Query(esper.StatementName("s0"), orderAB), nil
		}
	case "interval-partitioned":
		// same shape plus partition by cat.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  partition by cat   measures A.theString as a, B[0].theString as b0, B[1].theString as b1, last(B.theString) as lastb  pattern (A B*)   INTERVAL 10 seconds   define     A as A.theString like 'A%',    B as B.theString like 'B%') order by a, b0, b1, lastb" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				PartitionBy(cat).
				Interval(10*time.Second).
				Measures(measuresABStarLast()...).
				Query(esper.StatementName("s0"), orderAB), nil
		}
	case "interval-multicompleted":
		// same shape without partition; overlapping matches exercise the
		// silent strand-death rule.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a, B[0].theString as b0, B[1].theString as b1, last(B.theString) as lastb  pattern (A B*)   interval 10 seconds   define     A as A.theString like 'A%',    B as B.theString like 'B%') order by a, b0, b1, lastb" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				Interval(10*time.Second).
				Measures(measuresABStarLast()...).
				Query(esper.StatementName("s0"), orderAB), nil
		}
	case "interval-monthscoped":
		// SupportBean, no window, interval 1 month (calendar-aware).
		if epl == "@name('s0') select * from SupportBean match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1  pattern (A B*) interval 1 month define A as A.theString like \"A%\", B as B.theString like \"B%\")" {
			supportString := esper.Field[rowRecogIntervalSupportBean, string]("theString")
			return esper.From[rowRecogIntervalSupportBean](env, "SupportBean").
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", likeA(supportString)).
				Define("B", likeB(supportString)).
				FirstMatch().
				IntervalCalendar(0, 1, 0).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-doc-sample":
		// TemperatureSensorEvent object-array, partition by device,
		// interval 5 seconds or terminated, count/first/last measures.
		if epl == "@name('s0') select * from TemperatureSensorEvent\nmatch_recognize (\n  partition by device\n  measures A.id as a_id, count(B.id) as count_b, first(B.id) as first_b, last(B.id) as last_b\n  pattern (A B*)\n  interval 5 seconds or terminated\n  define\n    A as A.temp > 100,\n    B as B.temp > 100)" {
			id := esper.Field[any, string]("id")
			device := esper.Field[any, int]("device")
			temp := esper.Field[any, float64]("temp")
			return esper.FromAny(env, "TemperatureSensorEvent").
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", esper.Greater[float64](temp, esper.Literal(100.0))).
				Define("B", esper.Greater[float64](temp, esper.Literal(100.0))).
				FirstMatch().
				PartitionBy(device).
				IntervalOrTerminated(5*time.Second).
				Measures(
					esper.Alias("a_id", esper.TagField[string]("A", "id")),
					esper.Alias("count_b", esper.TagCount("B")),
					esper.Alias("first_b", esper.TagFirst[string]("B", id)),
					esper.Alias("last_b", esper.TagLast[string]("B", id)),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-b":
		// pattern (A B): dead-end end states emit immediately, the interval
		// is not effective.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B.theString as b pattern (A B) interval 10 seconds or terminated define A as A.theString like 'A%', B as B.theString like 'B%')" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B)).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b", esper.TagField[string]("B", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-bstar":
		// pattern (A B*) first-match.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2 pattern (A B*) interval 10 seconds or terminated define A as A.theString like \"A%\", B as B.theString like \"B%\")" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
					esper.Alias("b2", esper.TagFieldAt[string]("B", 2, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-bstar-allmatches":
		// same EPL plus ' all matches'; Java asserts any-order, so the
		// scenario case step carries mode:"any".
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2 all matches pattern (A B*) interval 10 seconds or terminated define A as A.theString like \"A%\", B as B.theString like \"B%\")" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B.ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				AllMatches().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
					esper.Alias("b2", esper.TagFieldAt[string]("B", 2, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-bstar-or-c":
		// pattern (A (B* | C)): the C branch is a dead end and emits
		// immediately; the B* branch dies on C1 without a termination row.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2, C.theString as c  pattern (A (B* | C)) interval 10 seconds or terminated define A as A.theString like 'A%', B as B.theString like 'B%', C as C.theString like 'C%')" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, esper.RowAlternation(B.ZeroOrMore(), C))).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				Define("C", likeC(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
					esper.Alias("b2", esper.TagFieldAt[string]("B", 2, "theString")),
					esper.Alias("c", esper.TagField[string]("C", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-bstar-or-cstar":
		// pattern (A (B* | C*)): a misfit on one branch emits the prefix
		// while the sibling branch keeps running.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, C[0].theString as c0, C[1].theString as c1  pattern (A (B* | C*)) interval 10 seconds or terminated define A as A.theString like 'A%', B as B.theString like 'B%', C as C.theString like 'C%')" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, esper.RowAlternation(B.ZeroOrMore(), C.ZeroOrMore()))).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				Define("C", likeC(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
					esper.Alias("c0", esper.TagFieldAt[string]("C", 0, "theString")),
					esper.Alias("c1", esper.TagFieldAt[string]("C", 1, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-b-cstar":
		// pattern (A B C*): mandatory B before the repeated tail.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B.theString as b, C[0].theString as c0, C[1].theString as c1, C[2].theString as c2  pattern (A B C*) interval 10 seconds or terminated define A as A.theString like 'A%', B as B.theString like 'B%', C as C.theString like 'C%')" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B, C.ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				Define("C", likeC(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b", esper.TagField[string]("B", "theString")),
					esper.Alias("c0", esper.TagFieldAt[string]("C", 0, "theString")),
					esper.Alias("c1", esper.TagFieldAt[string]("C", 1, "theString")),
					esper.Alias("c2", esper.TagFieldAt[string]("C", 2, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-bplus":
		// pattern (A B+): the mandatory first B has no EndEval successor,
		// so a misfit kills the strand silently.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2 pattern (A B+) interval 10 seconds or terminated define A as A.theString like 'A%', B as B.theString like 'B%')" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, B.OneOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
					esper.Alias("b2", esper.TagFieldAt[string]("B", 2, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-astar":
		// pattern (A*) with an unqualified define (theString like 'A%').
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A[0].theString as a0, A[1].theString as a1, A[2].theString as a2, A[3].theString as a3, A[4].theString as a4 pattern (A*) interval 10 seconds or terminated define A as theString like 'A%')" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(A.ZeroOrMore()).
				Define("A", likeA(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a0", esper.TagFieldAt[string]("A", 0, "theString")),
					esper.Alias("a1", esper.TagFieldAt[string]("A", 1, "theString")),
					esper.Alias("a2", esper.TagFieldAt[string]("A", 2, "theString")),
					esper.Alias("a3", esper.TagFieldAt[string]("A", 3, "theString")),
					esper.Alias("a4", esper.TagFieldAt[string]("A", 4, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "orterminated-a-parens-bstar":
		// pattern (A (B)*): parenthesized repeated group.
		if epl == "@name('s0') select * from SupportRecogBean#keepall match_recognize ( measures A.theString as a, B[0].theString as b0, B[1].theString as b1, B[2].theString as b2 pattern (A (B)*) interval 10 seconds or terminated define A as A.theString like \"A%\", B as B.theString like \"B%\")" {
			return esper.From[rowRecogPrevBean](env, "SupportRecogBean").
				Window(esper.KeepAll()).
				MatchRecognize(esper.RowSequence(A, esper.RowSequence(B).ZeroOrMore())).
				Define("A", likeA(theString)).
				Define("B", likeB(theString)).
				FirstMatch().
				IntervalOrTerminated(10*time.Second).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b0", esper.TagFieldAt[string]("B", 0, "theString")),
					esper.Alias("b1", esper.TagFieldAt[string]("B", 1, "theString")),
					esper.Alias("b2", esper.TagFieldAt[string]("B", 2, "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogIntervalID, caseName, epl)
}
