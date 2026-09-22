package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for RowRecogMultikeyWArray (ords 0-1): match_recognize
// partition-by over multi-component keys — an int[] array key with deep
// content equality (null and empty arrays are distinct partitions) and a
// plain two-scalar key tuple. Approved differences: Go has no EPL text;
// each pinned deploy EPL maps to the fluent MatchRecognize builder.
var rowRecogMultikeyWArrayJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogMultikeyWArray.java",
}

var rowRecogMultikeyWArrayJavaRuntimeIDs = []string{
	"java-runtime-7a2e1b6edbc6c814817e", // RowRecogPartitionMultikeyWArray
	"java-runtime-11baac617fb4b4f194c6", // RowRecogPartitionMultikeyPlain
}

var rowRecogMultikeyWArrayJavaExecutions = []string{
	"RowRecogPartitionMultikeyWArray",
	"RowRecogPartitionMultikeyPlain",
}

const rowRecogMultikeyWArrayID = "rowrecog-multikey-warray"

// rowRecogMultikeyArrayBean mirrors SupportEventWithIntArray{id, array
// int[], value}.
type rowRecogMultikeyArrayBean struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

// rowRecogMultikeySupportBean mirrors SupportBean{theString,
// intPrimitive, longPrimitive, doublePrimitive}.
type rowRecogMultikeySupportBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

func rowRecogMultikeyWArrayRuntimeURI(caseName string) string {
	return rowRecogMultikeyWArrayID + "-" + caseName
}

func runRowRecogMultikeyWArrayScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	var traces []compat.Trace
	for _, caseName := range []string{
		"partition-multikey-warray",
		"partition-multikey-plain",
	} {
		caseTrace, err := runRowRecogMultikeyWArrayCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rowrecog-multikey-warray case %q: %w", caseName, err)
		}
		traces = append(traces, caseTrace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRowRecogMultikeyWArrayCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rowRecogMultikeyArrayBean](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[rowRecogMultikeySupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(rowRecogMultikeyWArrayRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: rowRecogMultikeyWArrayID}
	var sequence uint64
	var deployment *esper.Deployment
	deploy := func(epl string) error {
		query, err := rowRecogMultikeyWArrayQueryForEPL(env, caseName, epl)
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
					return trace, fmt.Errorf("%s undeploy %q: %w", rowRecogMultikeyWArrayID, caseName, err)
				}
				deployment = nil
			}
		case "send":
			if err := rowRecogMultikeyWArraySend(ctx, engine, step); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("%s unsupported step op %q", rowRecogMultikeyWArrayID, step.Op)
		}
	}
	return trace, nil
}

func rowRecogMultikeyWArraySend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case "SupportEventWithIntArray":
		var bean rowRecogMultikeyArrayBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return fmt.Errorf("%s decode SupportEventWithIntArray: %w", rowRecogMultikeyWArrayID, err)
		}
		return engine.Send(ctx, step.EventType, bean)
	case "SupportBean":
		var bean rowRecogMultikeySupportBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return fmt.Errorf("%s decode SupportBean: %w", rowRecogMultikeyWArrayID, err)
		}
		return engine.Send(ctx, step.EventType, bean)
	default:
		return fmt.Errorf("%s unsupported event type %q", rowRecogMultikeyWArrayID, step.EventType)
	}
}

// rowRecogMultikeyWArrayQueryForEPL maps each pinned deploy EPL to the
// fluent MatchRecognize builder. The EPL text is pinned verbatim from the
// Java source; the builder mirrors the same semantics.
func rowRecogMultikeyWArrayQueryForEPL(env *esper.Environment, caseName, epl string) (esper.Query, error) {
	A := esper.RowVar("A")
	B := esper.RowVar("B")

	switch caseName {
	case "partition-multikey-warray":
		// partition by array (int[] deep-equals key) measures A.id/B.id
		// pattern (A B) define A.value=1, B.value=2.
		if epl == "@name('s0') select * from SupportEventWithIntArray match_recognize ( partition by array measures A.id as a, B.id as b pattern (A B) define A as A.value = 1, B as B.value = 2)" {
			array := esper.Field[rowRecogMultikeyArrayBean, []int]("array")
			value := esper.Field[rowRecogMultikeyArrayBean, int]("value")
			return esper.From[rowRecogMultikeyArrayBean](env, "SupportEventWithIntArray").
				MatchRecognize(esper.RowSequence(A, B)).
				Define("A", esper.Equal[int](value, esper.Literal(1))).
				Define("B", esper.Equal[int](value, esper.Literal(2))).
				FirstMatch().
				PartitionBy(array).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "id")),
					esper.Alias("b", esper.TagField[string]("B", "id")),
				).
				Query(esper.StatementName("s0")), nil
		}
	case "partition-multikey-plain":
		// partition by intPrimitive,longPrimitive measures
		// A.theString/B.theString pattern (A B) define
		// A.doublePrimitive=1, B.doublePrimitive=2.
		if epl == "@name('s0') select * from SupportBean match_recognize ( partition by intPrimitive, longPrimitive measures A.theString as a, B.theString as b pattern (A B) define A as A.doublePrimitive = 1, B as B.doublePrimitive = 2)" {
			intPrimitive := esper.Field[rowRecogMultikeySupportBean, int]("intPrimitive")
			longPrimitive := esper.Field[rowRecogMultikeySupportBean, int64]("longPrimitive")
			doublePrimitive := esper.Field[rowRecogMultikeySupportBean, float64]("doublePrimitive")
			return esper.From[rowRecogMultikeySupportBean](env, "SupportBean").
				MatchRecognize(esper.RowSequence(A, B)).
				Define("A", esper.Equal[float64](doublePrimitive, esper.Literal(1.0))).
				Define("B", esper.Equal[float64](doublePrimitive, esper.Literal(2.0))).
				FirstMatch().
				PartitionBy(intPrimitive, longPrimitive).
				Measures(
					esper.Alias("a", esper.TagField[string]("A", "theString")),
					esper.Alias("b", esper.TagField[string]("B", "theString")),
				).
				Query(esper.StatementName("s0")), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s case %q: unpinned deploy epl %q", rowRecogMultikeyWArrayID, caseName, epl)
}
