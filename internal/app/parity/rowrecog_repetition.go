package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for RowRecogRepetition (ords 0-5): quantifier expansion and
// repeated-variable measures — single bounds (A{2}), ranges (A{2,3}), open
// bounds (A{,2}, A{2,}), nested group repeats ((A B){2}), alternation repeats
// ((B{2}|C{2})), prev() in DEFINE over repeated captures, the nine invalid
// quantifier probes, and the 62 RowRecogPatternExpandUtil expansion pairs.
// Approved differences: Go has no EPL text; each pinned deploy EPL maps to the
// fluent MatchRecognize builder; ord0/ord1 share one runtime trace because
// soda only switches the compileDeploy path; ord3's expression-form
// quantifiers (A{}, A{null}, A{myvariable}, A{prev(A)}) are unrepresentable
// and pin the Java message without a Go gate; ord5's expansion text is
// rendered by a runner-local expander that mirrors
// RowRecogPatternExpandUtil.expand over the Go pattern model.
var rowRecogRepetitionJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogRepetition.java",
}

var rowRecogRepetitionJavaRuntimeIDs = []string{
	"java-runtime-d6d3a12949ef3e346d80", // RowRecogRepetitionRepeats{soda=false}
	"java-runtime-cdf672905648f281ad59", // RowRecogRepetitionRepeats{soda=true}
	"java-runtime-ad8cb0aeafdfcdced0f2", // RowRecogRepetitionPrev
	"java-runtime-941bbeda7e12013505b3", // RowRecogRepetitionInvalid
	"java-runtime-aa81841e62a62098b578", // RowRecogRepetitionDocSamples
	"java-runtime-a47314c7aeffaf6d632c", // RowRecogRepetitionEquivalent
}

var rowRecogRepetitionJavaExecutions = []string{
	"RowRecogRepetitionRepeats{soda=false}",
	"RowRecogRepetitionRepeats{soda=true}",
	"RowRecogRepetitionPrev",
	"RowRecogRepetitionInvalid",
	"RowRecogRepetitionDocSamples",
	"RowRecogRepetitionEquivalent",
}

const rowRecogRepetitionID = "rowrecog-repetition"

type rowRecogRepetitionBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func rowRecogRepetitionRuntimeURI(caseName string) string {
	return rowRecogRepetitionID + "-" + caseName
}

func runRowRecogRepetitionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"repeats",
		"repeats-soda",
		"prev",
		"invalid",
		"doc-samples",
		"equivalent",
	} {
		caseTrace, err := runRowRecogRepetitionCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog-repetition case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogRepetitionCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if caseName == "doc-samples" {
		if _, err := esper.RegisterObjectArray(env, "TemperatureSensorEvent", []esper.FieldSpec{
			esper.FieldDef("id", reflect.TypeOf("")),
			esper.FieldDef("device", reflect.TypeOf(0)),
			esper.FieldDef("temp", reflect.TypeOf(0.0)),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else if caseName != "equivalent" {
		if _, err := esper.RegisterStruct[rowRecogRepetitionBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(rowRecogRepetitionRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: rowRecogRepetitionID}
	var sequence uint64
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, skip, err := rowRecogRepetitionQueryForEPL(env, caseName, epl)
		if err != nil {
			return err
		}
		if skip {
			// 'create variable int myvariable = 0' has no Go runtime-variable
			// surface; the invalid probes pin the Java messages directly.
			return nil
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
					return trace, fmt.Errorf("%s undeploy %q: %w", rowRecogRepetitionID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			if err := rowRecogRepetitionSend(ctx, engine, caseName, step); err != nil {
				return trace, err
			}
		case "build-error":
			if err := rowRecogRepetitionBuildError(env, &trace, caseName, step); err != nil {
				return trace, err
			}
		case "compile-text":
			if err := rowRecogRepetitionCompileText(&trace, caseName, step); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", rowRecogRepetitionID, step.Op)
		}
	}
	return trace, nil
}

func rowRecogRepetitionSend(ctx context.Context, engine *esper.Engine, caseName string, step compat.Step) error {
	if caseName == "doc-samples" {
		var payload map[string]any
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s decode TemperatureSensorEvent: %w", rowRecogRepetitionID, err)
		}
		// Object-array columns are positional: id, device, temp.
		return engine.SendObjectArray(ctx, step.EventType, []any{payload["id"], payload["device"], payload["temp"]})
	}
	var bean rowRecogRepetitionBean
	if err := json.Unmarshal(step.Payload, &bean); err != nil {
		return fmt.Errorf("%s decode SupportBean: %w", rowRecogRepetitionID, err)
	}
	return engine.Send(ctx, step.EventType, bean)
}

// rowRecogRepetitionQueryForEPL maps each pinned deploy EPL to the fluent
// MatchRecognize builder. The EPL text is pinned verbatim from the Java
// source; the builder mirrors the same semantics. A variable is a scalar
// measure (PatternEvent) when it can bind at most one event per match and an
// array measure (TagEvents) when it is repeated or appears more than once.
func rowRecogRepetitionQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, bool, error) {
	if caseName == "invalid" && epl == "create variable int myvariable = 0" {
		return esper.Query{}, true, nil
	}
	if caseName == "doc-samples" {
		q, err := rowRecogRepetitionDocSampleQuery(env, epl)
		return q, false, err
	}
	if caseName != "repeats" && caseName != "prev" {
		return esper.Query{}, false, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogRepetitionID, caseName, epl)
	}

	stream := esper.From[rowRecogRepetitionBean](env, "SupportBean")
	theString := esper.Field[rowRecogRepetitionBean, string]("theString")
	intPrimitive := esper.Field[rowRecogRepetitionBean, int]("intPrimitive")
	defineA := esper.Like(theString, esper.Literal("A%"))
	defineB := esper.Like(theString, esper.Literal("B%"))
	defineC := esper.Like(theString, esper.Literal("C%"))

	// measure helpers: scalar (single-bound) vs array (repeated) tags
	scalar := func(alias, tag string) esper.Selection {
		return esper.Alias(alias, esper.PatternEvent(tag))
	}
	array := func(alias, tag string) esper.Selection {
		return esper.Alias(alias, esper.TagEvents(tag))
	}
	query := func(pattern esper.RowPattern, defines []struct {
		name string
		expr esper.Expr
	}, measures []esper.Selection) esper.Query {
		q := stream.MatchRecognize(pattern).FirstMatch().SkipPastLastRow()
		for _, d := range defines {
			q = q.Define(d.name, d.expr)
		}
		return q.PartitionBy(intPrimitive).Measures(measures...).Query(esper.StatementName("s0"))
	}
	defs := func(pairs ...struct {
		name string
		expr esper.Expr
	}) []struct {
		name string
		expr esper.Expr
	} {
		return pairs
	}
	dA := struct {
		name string
		expr esper.Expr
	}{"A", defineA}
	dB := struct {
		name string
		expr esper.Expr
	}{"B", defineB}
	dC := struct {
		name string
		expr esper.Expr
	}{"C", defineC}
	A := esper.RowVar("A")
	B := esper.RowVar("B")
	C := esper.RowVar("C")

	if caseName == "prev" {
		// measures A as a pattern (A{3})
		// define A as A.intPrimitive > prev(A.intPrimitive)
		if epl == "@name('s0') select * from SupportBean match_recognize (  measures A as a  pattern (A{3})   define     A as A.intPrimitive > prev(A.intPrimitive))" {
			return stream.MatchRecognize(A.Repeat(3, 3)).
				Define("A", esper.Greater[int](
					esper.TagField[int]("A", "intPrimitive"),
					esper.PrevTag[int](1, "A", "intPrimitive"),
				)).
				FirstMatch().
				SkipPastLastRow().
				Measures(array("a", "A")).
				Query(esper.StatementName("s0")), false, nil
		}
		return esper.Query{}, false, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogRepetitionID, caseName, epl)
	}

	// repeats: 28 pinned EPLs, all partitioned by intPrimitive.
	type entry struct {
		epl      string
		pattern  esper.RowPattern
		measures []esper.Selection
		defines  []struct {
			name string
			expr esper.Expr
		}
	}
	prefix := "@name('s0') select * from SupportBean match_recognize ( partition by intPrimitive measures "
	suffix := ")"
	entries := []entry{
		{"A as a pattern (A A) define A as A.theString like \"A%\"",
			esper.RowSequence(A, A), []esper.Selection{array("a", "A")}, defs(dA)},
		{"A as a pattern (A{2}) define A as A.theString like \"A%\"",
			A.Repeat(2, 2), []esper.Selection{array("a", "A")}, defs(dA)},
		{"A as a pattern ((A{2})) define A as A.theString like \"A%\"",
			esper.RowSequence(A.Repeat(2, 2)), []esper.Selection{array("a", "A")}, defs(dA)},
		{"A as a, B as b, C as c pattern (A B B C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, B, B, C), []esper.Selection{scalar("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern (A B{2} C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, B.Repeat(2, 2), C), []esper.Selection{scalar("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern (A (B B) C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, esper.RowSequence(B, B), C), []esper.Selection{scalar("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern (A (B{2}) C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, esper.RowSequence(B.Repeat(2, 2)), C), []esper.Selection{scalar("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern (A (B B|C C)) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, esper.RowAlternation(esper.RowSequence(B, B), esper.RowSequence(C, C))), []esper.Selection{scalar("a", "A"), array("b", "B"), array("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern (A (B{2}|C{2})) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, esper.RowAlternation(B.Repeat(2, 2), C.Repeat(2, 2))), []esper.Selection{scalar("a", "A"), array("b", "B"), array("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b pattern (A A B B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A, A, B, B), []esper.Selection{array("a", "A"), array("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A{2} B{2}) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A.Repeat(2, 2), B.Repeat(2, 2)), []esper.Selection{array("a", "A"), array("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A A A? B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A, A, A.Optional(), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A{2,3} B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A.Repeat(2, 3), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A? A? B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A.Optional(), A.Optional(), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A{,2} B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A.Repeat(0, 2), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A A A* B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A, A, A.ZeroOrMore(), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A{2,} B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A.Repeat(2, 0), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern (A{2,4} B) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A.Repeat(2, 4), B), []esper.Selection{array("a", "A"), scalar("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern ((A B) (A B)) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(esper.RowSequence(A, B), esper.RowSequence(A, B)), []esper.Selection{array("a", "A"), array("b", "B")}, defs(dA, dB)},
		{"A as a, B as b pattern ((A B){2}) define A as A.theString like \"A%\", B as B.theString like \"B%\"",
			esper.RowSequence(A, B).Repeat(2, 2), []esper.Selection{array("a", "A"), array("b", "B")}, defs(dA, dB)},
		{"A as a, B as b, C as c pattern (A (B C) (B C)) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, esper.RowSequence(B, C), esper.RowSequence(B, C)), []esper.Selection{scalar("a", "A"), array("b", "B"), array("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern (A (B C){2}) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(A, esper.RowSequence(B, C).Repeat(2, 2)), []esper.Selection{scalar("a", "A"), array("b", "B"), array("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern ((A B) (A B)? C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(esper.RowSequence(A, B), esper.RowSequence(A, B).Optional(), C), []esper.Selection{array("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern ((A B){1,2} C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(esper.RowSequence(A, B).Repeat(1, 2), C), []esper.Selection{array("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern ((A B)? (A B)? C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(esper.RowSequence(A, B).Optional(), esper.RowSequence(A, B).Optional(), C), []esper.Selection{array("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern ((A B){,2} C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(esper.RowSequence(A, B).Repeat(0, 2), C), []esper.Selection{array("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern ((A B) (A B) (A B)* C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(esper.RowSequence(A, B), esper.RowSequence(A, B), esper.RowSequence(A, B).ZeroOrMore(), C), []esper.Selection{array("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
		{"A as a, B as b, C as c pattern ((A B){2,} C) define A as A.theString like \"A%\", B as B.theString like \"B%\", C as C.theString like \"C%\"",
			esper.RowSequence(esper.RowSequence(A, B).Repeat(2, 0), C), []esper.Selection{array("a", "A"), array("b", "B"), scalar("c", "C")}, defs(dA, dB, dC)},
	}
	for _, e := range entries {
		if epl == prefix+e.epl+suffix {
			return query(e.pattern, e.defines, e.measures), false, nil
		}
	}
	return esper.Query{}, false, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogRepetitionID, caseName, epl)
}

// rowRecogRepetitionDocSampleQuery maps the four pinned doc-sample EPLs over
// the object-array TemperatureSensorEvent partitioned by device.
func rowRecogRepetitionDocSampleQuery(env *esper.Environment, epl string) (esper.Query, error) {
	stream := esper.FromAny(env, "TemperatureSensorEvent")
	temp := esper.Field[any, float64]("temp")
	device := esper.Field[any, int]("device")
	a0 := esper.Alias("a0_id", esper.TagFieldAt[string]("A", 0, "id"))
	a1 := esper.Alias("a1_id", esper.TagFieldAt[string]("A", 1, "id"))
	a2 := esper.Alias("a2_id", esper.TagFieldAt[string]("A", 2, "id"))
	bID := esper.Alias("b_id", esper.TagField[string]("B", "id"))
	A := esper.RowVar("A")
	B := esper.RowVar("B")
	query := func(pattern esper.RowPattern, hasB bool, measures ...esper.Selection) esper.Query {
		q := stream.MatchRecognize(pattern).
			Define("A", esper.GreaterOrEqual[float64](temp, esper.Literal(100.0)))
		if hasB {
			q = q.Define("B", esper.GreaterOrEqual[float64](temp, esper.Literal(102.0)))
		}
		return q.
			FirstMatch().
			SkipPastLastRow().
			PartitionBy(device).
			Measures(measures...).
			Query(esper.StatementName("s0"))
	}
	switch epl {
	case "@name('s0') select * from TemperatureSensorEvent\nmatch_recognize (\n  partition by device\n  measures A[0].id as a0_id, A[1].id as a1_id\n  pattern (A{2})\n  define \n\tA as A.temp >= 100)":
		return query(A.Repeat(2, 2), false, a0, a1), nil
	case "@name('s0') select * from TemperatureSensorEvent\nmatch_recognize (\n  partition by device\n  measures A[0].id as a0_id, A[1].id as a1_id, A[2].id as a2_id, B.id as b_id\n  pattern (A{2,} B)\n  define \n\tA as A.temp >= 100,\n\tB as B.temp >= 102)":
		return query(esper.RowSequence(A.Repeat(2, 0), B), true, a0, a1, a2, bID), nil
	case "@name('s0') select * from TemperatureSensorEvent\nmatch_recognize (\n  partition by device\n  measures A[0].id as a0_id, A[1].id as a1_id, A[2].id as a2_id, B.id as b_id\n  pattern (A{2,3} B)\n  define \n\tA as A.temp >= 100,\n\tB as B.temp >= 102)":
		return query(esper.RowSequence(A.Repeat(2, 3), B), true, a0, a1, a2, bID), nil
	case "@name('s0') select * from TemperatureSensorEvent\nmatch_recognize (\n  partition by device\n  measures A[0].id as a0_id, A[1].id as a1_id, B.id as b_id\n  pattern (A{,2} B)\n  define \n\tA as A.temp >= 100,\n\tB as B.temp >= 102)":
		return query(esper.RowSequence(A.Repeat(0, 2), B), true, a0, a1, bID), nil
	}
	return esper.Query{}, fmt.Errorf("%s doc-samples: unpinned deploy epl %q", rowRecogRepetitionID, epl)
}

// rowRecogRepetitionBuildError replays the nine invalid-quantifier probes.
// Expression-form quantifiers (empty, null, variable, prev) have no Go
// surface and pin the Java message without a gate; numeric-bound probes are
// verified against the Go builder's own rejection before the pinned prefix
// is recorded.
func rowRecogRepetitionBuildError(env *esper.Environment, trace *compat.Trace, caseName string, step compat.Step) error {
	pinned := map[string]string{
		"empty-quantifier":     "select * from SupportBean match_recognize (  measures A as a  pattern (A{}) )",
		"null-quantifier":      "select * from SupportBean match_recognize (  measures A as a  pattern (A{null}) )",
		"variable-quantifier":  "select * from SupportBean match_recognize (  measures A as a  pattern (A{myvariable}) )",
		"prev-quantifier":      "select * from SupportBean match_recognize (  measures A as a  pattern (A{prev(A)}) )",
		"negative-exact":       "select * from SupportBean match_recognize (  measures A as a  pattern (A{-1}) )",
		"negative-upper":       "select * from SupportBean match_recognize (  measures A as a  pattern (A{,-1}) )",
		"negative-lower-range": "select * from SupportBean match_recognize (  measures A as a  pattern (A{-1,10}) )",
		"negative-lower-open":  "select * from SupportBean match_recognize (  measures A as a  pattern (A{-1,}) )",
		"inverted-range":       "select * from SupportBean match_recognize (  measures A as a  pattern (A{5,3}) )",
	}
	want, ok := pinned[step.Statement]
	if !ok {
		return fmt.Errorf("%s: unknown build-error probe %q", rowRecogRepetitionID, step.Statement)
	}
	if step.Epl != want {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", rowRecogRepetitionID, step.Statement, step.Epl)
	}
	// Numeric-bound probes have a Go boundary: the fluent Repeat bounds are
	// validated at Build. Expression-form probes are unrepresentable.
	bounds := map[string][2]int{
		"negative-exact":       {-1, -1},
		"negative-upper":       {0, -1},
		"negative-lower-range": {-1, 10},
		"negative-lower-open":  {-1, 0},
		"inverted-range":       {5, 3},
	}
	if bound, gated := bounds[step.Statement]; gated {
		stream := esper.From[rowRecogRepetitionBean](env, "SupportBean")
		q := stream.MatchRecognize(esper.RowVar("A").Repeat(bound[0], bound[1])).
			Define("A", esper.Like(esper.Field[rowRecogRepetitionBean, string]("theString"), esper.Literal("A%"))).
			Measures(esper.Alias("a", esper.TagEvents("A"))).
			Query(esper.StatementName("s0"))
		_, buildErr := env.Build(q)
		if buildErr == nil {
			return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", rowRecogRepetitionID, step.Statement)
		}
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", rowRecogRepetitionID, step.Statement, buildErr)
		}
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// rowRecogRepetitionCompileText verifies the Go-side expansion of the pinned
// pattern against the Java RowRecogPatternExpandUtil expansion. The Go
// pattern model is parsed from the pinned EPL into a runner-local AST that
// mirrors Java's RowRecogExprNode (atom type + optional repeat), expanded
// with the same expandRepeat rules, and rendered; the record carries the
// pinned Java expansion text.
func rowRecogRepetitionCompileText(trace *compat.Trace, caseName string, step compat.Step) error {
	const prefix = "@name('s0') select * from SupportBean#keepall match_recognize ( measures A as a pattern ("
	const suffix = ") define A as A.theString like \"A%\")"
	if !strings.HasPrefix(step.Epl, prefix) || !strings.HasSuffix(step.Epl, suffix) {
		return fmt.Errorf("%s: compile-text step %q carries an unpinned EPL %q", rowRecogRepetitionID, step.Statement, step.Epl)
	}
	patternText := step.Epl[len(prefix) : len(step.Epl)-len(suffix)]
	node, err := rowRecogRepetitionParsePattern(patternText)
	if err != nil {
		return fmt.Errorf("%s: compile-text step %q pattern %q: %w", rowRecogRepetitionID, step.Statement, patternText, err)
	}
	expanded := rowRecogRepetitionExpand(node)
	rendered := rowRecogRepetitionRender(expanded)
	if rendered != step.ExpectExpansion {
		return fmt.Errorf("%s: compile-text step %q expansion drift: got %q want %q", rowRecogRepetitionID, step.Statement, rendered, step.ExpectExpansion)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "compile-text",
		Statement: step.Statement,
		Value:     step.ExpectExpansion,
	})
	return nil
}

// rrExpKind mirrors Java's RowRecogExprNode kinds needed by the 62 pinned
// expansion pairs.
type rrExpKind int

const (
	rrExpAtom rrExpKind = iota
	rrExpConcat
	rrExpNested
	rrExpAlteration
)

// rrExpNode mirrors RowRecogExprNode: an atom carries a tag and an NFA type
// suffix (?, *, +, ??, *?, +?); a repeat carries the {n}/{l,u} bounds. The Go
// fluent RowPattern merges type and repeat into one quantifier, so the
// compile-text surface needs this runner-local AST to model Java's
// type+repeat combinations (A+{2}, (A B)?{1,3}).
type rrExpNode struct {
	kind     rrExpKind
	tag      string
	nfaType  string // "", "?", "*", "+", "??", "*?", "+?"
	repeat   *rrExpRepeat
	children []*rrExpNode
}

type rrExpRepeat struct {
	single *int
	lower  *int
	upper  *int
}

// rowRecogRepetitionParsePattern parses the restricted pattern grammar used
// by the 62 pinned EPLs: atoms, parenthesized groups, alternations, type
// suffixes and {n}/{l,u}/{l,}/{,u} repeats.
func rowRecogRepetitionParsePattern(text string) (*rrExpNode, error) {
	p := &rrExpParser{text: text}
	node, err := p.parseConcat()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.text) {
		return nil, fmt.Errorf("trailing input at %d", p.pos)
	}
	return node, nil
}

type rrExpParser struct {
	text string
	pos  int
}

func (p *rrExpParser) skipSpace() {
	for p.pos < len(p.text) && (p.text[p.pos] == ' ' || p.text[p.pos] == '\t' || p.text[p.pos] == '\n') {
		p.pos++
	}
}

func (p *rrExpParser) parseConcat() (*rrExpNode, error) {
	var terms []*rrExpNode
	for {
		p.skipSpace()
		if p.pos >= len(p.text) || p.text[p.pos] == ')' || p.text[p.pos] == '|' {
			break
		}
		term, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("empty concatenation at %d", p.pos)
	}
	if len(terms) == 1 {
		return terms[0], nil
	}
	return &rrExpNode{kind: rrExpConcat, children: terms}, nil
}

func (p *rrExpParser) parseTerm() (*rrExpNode, error) {
	p.skipSpace()
	var node *rrExpNode
	if p.text[p.pos] == '(' {
		p.pos++
		// group content: concat possibly with alternation
		first, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		var content *rrExpNode
		if p.pos < len(p.text) && p.text[p.pos] == '|' {
			alts := []*rrExpNode{first}
			for p.pos < len(p.text) && p.text[p.pos] == '|' {
				p.pos++
				alt, err := p.parseConcat()
				if err != nil {
					return nil, err
				}
				alts = append(alts, alt)
			}
			content = &rrExpNode{kind: rrExpAlteration, children: alts}
		} else {
			content = first
		}
		p.skipSpace()
		if p.pos >= len(p.text) || p.text[p.pos] != ')' {
			return nil, fmt.Errorf("missing ')' at %d", p.pos)
		}
		p.pos++
		// Every parenthesized group is a nested node; parens are a render
		// concern, not a parse one. The renderer drops them when the child
		// renders identically at GROUPING precedence (atoms, nested groups,
		// alternations) and keeps them for concatenations.
		node = &rrExpNode{kind: rrExpNested, children: []*rrExpNode{content}}
	} else {
		start := p.pos
		for p.pos < len(p.text) && p.text[p.pos] >= 'A' && p.text[p.pos] <= 'Z' {
			p.pos++
		}
		if p.pos == start {
			return nil, fmt.Errorf("expected atom at %d", p.pos)
		}
		node = &rrExpNode{kind: rrExpAtom, tag: p.text[start:p.pos]}
	}
	// type suffix: ??, *?, +?, ?, *, +
	for _, suffix := range []string{"??", "*?", "+?", "?", "*", "+"} {
		if strings.HasPrefix(p.text[p.pos:], suffix) {
			node.nfaType = suffix
			p.pos += len(suffix)
			break
		}
	}
	// repeat bounds
	if p.pos < len(p.text) && p.text[p.pos] == '{' {
		p.pos++
		repeat := &rrExpRepeat{}
		lower := p.parseInt()
		p.skipSpace()
		if p.pos < len(p.text) && p.text[p.pos] == ',' {
			p.pos++
			p.skipSpace()
			upper := p.parseInt()
			repeat.lower = lower
			repeat.upper = upper
		} else {
			repeat.single = lower
		}
		p.skipSpace()
		if p.pos >= len(p.text) || p.text[p.pos] != '}' {
			return nil, fmt.Errorf("missing '}' at %d", p.pos)
		}
		p.pos++
		node.repeat = repeat
	}
	return node, nil
}

func (p *rrExpParser) parseInt() *int {
	p.skipSpace()
	start := p.pos
	if p.pos < len(p.text) && p.text[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.text) && p.text[p.pos] >= '0' && p.text[p.pos] <= '9' {
		p.pos++
	}
	if p.pos == start {
		return nil
	}
	value := 0
	negative := false
	for _, ch := range p.text[start:p.pos] {
		if ch == '-' {
			negative = true
			continue
		}
		value = value*10 + int(ch-'0')
	}
	if negative {
		value = -value
	}
	return &value
}

// rowRecogRepetitionExpand mirrors RowRecogPatternExpandUtil.expand: children
// expand first (atoms before nested, deepest first — equivalent to recursive
// post-order), then the node's own repeat expands into a concatenation of
// copies whose NFA type is adjusted per the range form.
func rowRecogRepetitionExpand(node *rrExpNode) *rrExpNode {
	if node == nil {
		return nil
	}
	// expand children first; concat children splice their expanded copies
	// inline while nested/alteration parents keep the concat wrapper
	if node.kind == rrExpConcat {
		spliced := make([]*rrExpNode, 0, len(node.children))
		for _, child := range node.children {
			expanded := rowRecogRepetitionExpand(child)
			if expanded.kind == rrExpConcat {
				spliced = append(spliced, expanded.children...)
			} else {
				spliced = append(spliced, expanded)
			}
		}
		node.children = spliced
	} else {
		for index, child := range node.children {
			node.children[index] = rowRecogRepetitionExpand(child)
		}
	}
	if node.repeat == nil {
		return node
	}
	copies := rowRecogRepetitionExpandRepeat(node)
	return &rrExpNode{kind: rrExpConcat, children: copies}
}

// rowRecogRepetitionExpandRepeat mirrors expandRepeat: single bounds copy the
// node verbatim; {lower,upper} copies lower verbatim then (upper-lower)
// optional-adjusted copies; {lower,} copies lower verbatim then one
// zero-to-many-adjusted copy; {,upper} copies upper optional-adjusted copies.
func rowRecogRepetitionExpandRepeat(node *rrExpNode) []*rrExpNode {
	repeat := node.repeat
	copyWithType := func(nfaType string) *rrExpNode {
		clone := &rrExpNode{kind: node.kind, tag: node.tag, nfaType: nfaType}
		clone.children = append([]*rrExpNode(nil), node.children...)
		return clone
	}
	var repeated []*rrExpNode
	if repeat.single != nil {
		for i := 0; i < *repeat.single; i++ {
			repeated = append(repeated, copyWithType(node.nfaType))
		}
		return repeated
	}
	lower, upper := repeat.lower, repeat.upper
	if lower != nil && upper != nil {
		for i := 0; i < *lower; i++ {
			repeated = append(repeated, copyWithType(node.nfaType))
		}
		for i := *lower; i < *upper; i++ {
			newType := node.nfaType
			switch newType {
			case "":
				newType = "?"
			case "+":
				newType = "*"
			case "+?":
				newType = "*?"
			}
			repeated = append(repeated, copyWithType(newType))
		}
		return repeated
	}
	if upper == nil {
		for i := 0; i < *lower; i++ {
			repeated = append(repeated, copyWithType(node.nfaType))
		}
		newType := node.nfaType
		switch newType {
		case "", "?", "+":
			newType = "*"
		case "??", "+?":
			newType = "*?"
		}
		repeated = append(repeated, copyWithType(newType))
		return repeated
	}
	for i := 0; i < *upper; i++ {
		newType := node.nfaType
		switch newType {
		case "":
			newType = "?"
		case "+":
			newType = "*"
		case "+?":
			newType = "*?"
		}
		repeated = append(repeated, copyWithType(newType))
	}
	return repeated
}

// rowRecogRepetitionRender renders the expanded AST the way Esper's toEPL
// renders the expanded RowRecogExprNode tree: concatenations join with
// spaces, nested nodes keep parens only around concatenations (GROUPING
// precedence), alternations wrap in parens, atoms render tag+type-suffix.
func rowRecogRepetitionRender(node *rrExpNode) string {
	if node == nil {
		return ""
	}
	switch node.kind {
	case rrExpAtom:
		return node.tag + node.nfaType
	case rrExpConcat:
		parts := make([]string, 0, len(node.children))
		for _, child := range node.children {
			parts = append(parts, rowRecogRepetitionRender(child))
		}
		return strings.Join(parts, " ")
	case rrExpNested:
		parts := make([]string, 0, len(node.children))
		for _, child := range node.children {
			parts = append(parts, rowRecogRepetitionRender(child))
		}
		// Esper's RowRecogExprNodeNested.toEPL adds parens only when the
		// child's precedence is below GROUPING: a concatenation keeps them
		// ((A{2}) renders (A A)) while atoms, nested groups and alternations
		// render bare ((A?){2} renders A? A?, ((A B)){2} renders (A B) (A B),
		// (A|B) renders (A|B) via the alternation's own parens).
		if len(node.children) == 1 && node.children[0].kind == rrExpConcat {
			return "(" + strings.Join(parts, " ") + ")" + node.nfaType
		}
		return strings.Join(parts, " ") + node.nfaType
	case rrExpAlteration:
		parts := make([]string, 0, len(node.children))
		for _, child := range node.children {
			parts = append(parts, rowRecogRepetitionRender(child))
		}
		return "(" + strings.Join(parts, "|") + ")" + node.nfaType
	}
	return ""
}
