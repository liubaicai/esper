package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLOtherAsKeywordBacktick behavioral trio (ords 3/5/1):
// update-istream alias rewrite, lastevent subselect alias, and two-stream
// lastevent join with the reserved-word backtick aliases `order` and `select`.
var eplAsKeywordBacktickJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherAsKeywordBacktick.java",
}

var eplAsKeywordBacktickJavaRuntimeIDs = []string{
	"java-runtime-5c48441abdc543566dd1", // EPLOtherUpdateIStream (ord 3)
	"java-runtime-7ca72ccdfaf2f99f4ca6", // EPLOtherSubselect (ord 5)
	"java-runtime-a9b9ecfe0dc6693d0e31", // EPLOtherFromClause (ord 1)
	"java-runtime-472d2c12a99c291f275c", // EPLOtherFAFUpdateDelete (ord 0)
	"java-runtime-c0ea9e858846b717b2e4", // EPLOtherOnTrigger (ord 2)
	"java-runtime-4da78c382449b37e5599", // EPLOthernMergeAndUpdateAndSelect (ord 4)
}

var eplAsKeywordBacktickJavaExecutions = []string{
	"EPLOtherUpdateIStream",
	"EPLOtherSubselect",
	"EPLOtherFromClause",
	"EPLOtherFAFUpdateDelete",
	"EPLOtherOnTrigger",
	"EPLOthernMergeAndUpdateAndSelect",
}

type akbS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

type akbS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
	P12 *string `esper:"p12"`
	P13 *string `esper:"p13"`
}

func akbStrPtr(s string) *string { return &s }

func runEplAsKeywordBacktickScenario(ctx context.Context, _ compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: "esper-parity/v1", ID: "epl-as-keyword-backtick"}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[akbS0](env, "SupportBean_S0"); err != nil {
		return trace, err
	}
	if _, err := esper.RegisterStruct[akbS1](env, "SupportBean_S1"); err != nil {
		return trace, err
	}
	if err := registerAKBFAFObjects(env); err != nil {
		return trace, err
	}

	// Case 1: update-istream (ord 3)
	caseTrace, err := runAKBUpdateIStream(ctx, env)
	if err != nil {
		return trace, fmt.Errorf("case update-istream: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace...)

	// Case 2: subselect-groups (ord 5)
	caseTrace, err = runAKBSubselect(ctx, env)
	if err != nil {
		return trace, fmt.Errorf("case subselect-groups: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace...)

	// Case 3: from-clause-join (ord 1)
	caseTrace, err = runAKBFromClauseJoin(ctx, env)
	if err != nil {
		return trace, fmt.Errorf("case from-clause-join: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace...)

	// Case 4: faf-update-delete (ord 0)
	caseTrace, err = runAKBFAFUpdateDelete(ctx, env)
	if err != nil {
		return trace, fmt.Errorf("case faf-update-delete: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace...)

	// Case 5: on-trigger-table-select (ord 2)
	caseTrace, err = runAKBOnTriggerTableSelect(ctx, env)
	if err != nil {
		return trace, fmt.Errorf("case on-trigger-table-select: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace...)

	// Case 6: merge-update-select (ord 4)
	caseTrace, err = runAKBMergeUpdateSelect(ctx, env)
	if err != nil {
		return trace, fmt.Errorf("case merge-update-select: %w", err)
	}
	trace.Records = append(trace.Records, caseTrace...)

	return trace, nil
}

func runAKBUpdateIStream(ctx context.Context, env *esper.Environment) ([]compat.TraceRecord, error) {
	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		eplAsKeywordBacktickJavaRuntimeIDs[0]), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	// Module 1: update istream
	updateStream := esper.From[akbS0](env, "SupportBean_S0").
		UpdateStream(esper.SetColumn("p00", esper.Field[akbS0, *string]("p01")))
	updatePlan, err := env.Build(updateStream.Query())
	if err != nil {
		return nil, err
	}
	if _, err := engine.Deploy(ctx, updatePlan); err != nil {
		return nil, err
	}

	// Module 2: select * consumer
	consumer, err := env.Build(esper.From[akbS0](env, "SupportBean_S0").
		Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}
	consumerDeploy, err := engine.Deploy(ctx, consumer)
	if err != nil {
		return nil, err
	}

	var records []compat.TraceRecord
	seq := uint64(0)
	st := consumerDeploy.Statements()[0]
	if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		records = append(records, compat.TraceRecord{
			Case:      "update-istream",
			Operation: "listener",
			Statement: "s0",
			Sequence:  seq,
			Time:      viewGroupMergeFormatTime2(time.Unix(0, 0).UTC()),
			New:       compat.NormalizeResults(batch.New),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	if err := engine.Send(ctx, "SupportBean_S0", akbS0{ID: 1, P00: akbStrPtr("a"), P01: akbStrPtr("x")}); err != nil {
		return nil, err
	}
	return records, nil
}

func runAKBSubselect(ctx context.Context, env *esper.Environment) ([]compat.TraceRecord, error) {
	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		eplAsKeywordBacktickJavaRuntimeIDs[1]), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	inner := esper.From[akbS0](env, "SupportBean_S0").AsRecord().Window(esper.LastEvent())
	subQ := esper.SubqueryValue[string](inner, esper.Field[akbS0, string]("p00"))
	consumer, err := env.Build(esper.From[akbS1](env, "SupportBean_S1").AsRecord().
		Select(esper.Alias("c0", subQ)).
		Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}
	consumerDeploy, err := engine.Deploy(ctx, consumer)
	if err != nil {
		return nil, err
	}

	var records []compat.TraceRecord
	seq := uint64(0)
	st := consumerDeploy.Statements()[0]
	if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		records = append(records, compat.TraceRecord{
			Case:      "subselect-groups",
			Operation: "listener",
			Statement: "s0",
			Sequence:  seq,
			Time:      viewGroupMergeFormatTime2(time.Unix(0, 0).UTC()),
			New:       compat.NormalizeResults(batch.New),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	if err := engine.Send(ctx, "SupportBean_S0", akbS0{ID: 1, P00: akbStrPtr("A")}); err != nil {
		return nil, err
	}
	if err := engine.Send(ctx, "SupportBean_S1", akbS1{ID: 2}); err != nil {
		return nil, err
	}
	return records, nil
}

func runAKBFromClauseJoin(ctx context.Context, env *esper.Environment) ([]compat.TraceRecord, error) {
	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		eplAsKeywordBacktickJavaRuntimeIDs[2]), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	consumer, err := env.Build(esper.JoinMany(
		esper.JoinSource(esper.From[akbS0](env, "SupportBean_S0").Window(esper.LastEvent())),
		esper.JoinSource(esper.From[akbS1](env, "SupportBean_S1").Window(esper.LastEvent())),
	).Select(
		esper.SelectSourceEvent(0, "order"),
		esper.SelectLeft("order.p00", esper.Field[akbS0, string]("p00")),
		esper.SelectSourceEvent(1, "select"),
		esper.SelectRight("select.p10", esper.Field[akbS1, string]("p10")),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}
	consumerDeploy, err := engine.Deploy(ctx, consumer)
	if err != nil {
		return nil, err
	}

	var records []compat.TraceRecord
	seq := uint64(0)
	st := consumerDeploy.Statements()[0]
	if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		records = append(records, compat.TraceRecord{
			Case:      "from-clause-join",
			Operation: "listener",
			Statement: "s0",
			Sequence:  seq,
			Time:      viewGroupMergeFormatTime2(time.Unix(0, 0).UTC()),
			New:       compat.NormalizeResults(batch.New),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	if err := engine.Send(ctx, "SupportBean_S0", akbS0{ID: 1, P00: akbStrPtr("S0_1")}); err != nil {
		return nil, err
	}
	if err := engine.Send(ctx, "SupportBean_S1", akbS1{ID: 10, P10: akbStrPtr("S1_1")}); err != nil {
		return nil, err
	}

	// Unwrap NormalizeResults' {kind,row,fields} wrapper for nested
	// event-bean columns to match the Java oracle's plain-object rendering.
	for i := range records {
		for _, row := range records[i].New {
			unwrapAKBNested(row.Fields)
		}
	}
	return records, nil
}

func unwrapAKBNested(fields map[string]any) {
	for key, value := range fields {
		if obj, ok := value.(map[string]any); ok {
			if kind, hasKind := obj["kind"]; hasKind && kind == "row" {
				if inner, hasFields := obj["fields"].(map[string]any); hasFields {
					fields[key] = inner
					unwrapAKBNested(inner)
				}
			}
		}
	}
}

func viewGroupMergeFormatTime2(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z07:00")
}
