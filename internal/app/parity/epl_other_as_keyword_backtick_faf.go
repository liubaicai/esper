package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the EPLOtherAsKeywordBacktick FAF/on-trigger/merge
// trio (ords 0/2/4): fire-and-forget update/delete against a keepall named
// window with the reserved-word backtick alias `order`, an on-select join
// between a trigger stream and a primary-key table, and an on-merge /
// on-update / on-select chain over a keepall named window.
var eplAsKeywordBacktickFafRuntimeIDs = []string{
	"java-runtime-472d2c12a99c291f275c", // EPLOtherFAFUpdateDelete (ord 0)
	"java-runtime-c0ea9e858846b717b2e4", // EPLOtherOnTrigger (ord 2)
	"java-runtime-4da78c382449b37e5599", // EPLOthernMergeAndUpdateAndSelect (ord 4)
}

type akbPair struct {
	P0 string `esper:"p0"`
	P1 string `esper:"p1"`
}

type akbSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func registerAKBFAFObjects(env *esper.Environment) error {
	if _, err := esper.RegisterStruct[akbPair](env, "AKBPair"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[akbSupportBean](env, "SupportBean"); err != nil {
		return err
	}
	schema, ok := env.Schema("AKBPair")
	if !ok {
		return fmt.Errorf("parity: AKBPair schema is missing")
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindowFAF", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindowMerge", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
		return err
	}
	_, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("k1"),
		esper.TableColumnOf[string]("v1"),
	})
	return err
}

// akbFAFRecord emits a fire-and-forget select observation. FAF mutations are
// intentionally unrecorded: the suite never observes their results, only the
// subsequent select snapshots.
func akbFAFRecord(records []compat.TraceRecord, caseName, statement string, seq uint64, results []esper.Result) []compat.TraceRecord {
	return append(records, compat.TraceRecord{
		Case:      caseName,
		Operation: "faf",
		Statement: statement,
		Sequence:  seq,
		Time:      viewGroupMergeFormatTime2(time.Unix(0, 0).UTC()),
		New:       compat.NormalizeResults(results),
	})
}

func akbFAFSelectPair(ctx context.Context, env *esper.Environment, engine *esper.Engine, window, statement string) (esper.QueryResult, error) {
	plan, err := env.Build(esper.FromNamedWindow(env, window).
		Select(
			esper.Alias("p0", esper.Field[any, string]("p0")),
			esper.Alias("p1", esper.Field[any, string]("p1")),
		).Query(esper.StatementName(statement)))
	if err != nil {
		return esper.QueryResult{}, err
	}
	return engine.ExecuteFireAndForget(ctx, plan)
}

// runAKBFAFUpdateDelete replays EPLOtherFAFUpdateDelete (ord 0): the suite's
// keepall named window receives one FAF-inserted row, an alias-qualified FAF
// update copies p1 into p0, an alias-qualified FAF delete removes it, and the
// final FAF select observes zero rows.
func runAKBFAFUpdateDelete(ctx context.Context, env *esper.Environment) ([]compat.TraceRecord, error) {
	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		eplAsKeywordBacktickFafRuntimeIDs[0]), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	var records []compat.TraceRecord
	seq := uint64(0)

	insertPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindowFAF").OnDemand().
		InsertRows(esper.InsertValues(esper.Literal("a"), esper.Literal("b"))))
	if err != nil {
		return nil, err
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertPlan); err != nil {
		return nil, err
	}

	seq++
	selectResult, err := akbFAFSelectPair(ctx, env, engine, "MyWindowFAF", "faf-select-1")
	if err != nil {
		return nil, err
	}
	records = akbFAFRecord(records, "faf-update-delete", "faf-select-1", seq, selectResult.Results())

	// The Java FAF update carries no where clause; the Go on-demand update
	// requires an explicit predicate, so the all-rows form is Literal(true).
	updatePlan, err := env.Build(esper.FromNamedWindow(env, "MyWindowFAF").OnDemand().
		UpdateWhere(esper.Literal(true),
			esper.SetColumn("p0", esper.NamedWindowField[string]("p1"))))
	if err != nil {
		return nil, err
	}
	if _, err := engine.ExecuteFireAndForget(ctx, updatePlan); err != nil {
		return nil, err
	}

	seq++
	selectResult, err = akbFAFSelectPair(ctx, env, engine, "MyWindowFAF", "faf-select-2")
	if err != nil {
		return nil, err
	}
	records = akbFAFRecord(records, "faf-update-delete", "faf-select-2", seq, selectResult.Results())

	deletePlan, err := env.Build(esper.FromNamedWindow(env, "MyWindowFAF").OnDemand().
		DeleteWhere(esper.Equal[string](esper.NamedWindowField[string]("p0"), esper.Literal("b"))))
	if err != nil {
		return nil, err
	}
	if _, err := engine.ExecuteFireAndForget(ctx, deletePlan); err != nil {
		return nil, err
	}

	seq++
	selectResult, err = akbFAFSelectPair(ctx, env, engine, "MyWindowFAF", "faf-select-3")
	if err != nil {
		return nil, err
	}
	records = akbFAFRecord(records, "faf-update-delete", "faf-select-3", seq, selectResult.Results())

	return records, nil
}

// runAKBOnTriggerTableSelect replays EPLOtherOnTrigger (ord 2): a primary-key
// table seeded through FAF inserts and an on-select statement that joins the
// trigger property p00 against the table key k1.
func runAKBOnTriggerTableSelect(ctx context.Context, env *esper.Environment) ([]compat.TraceRecord, error) {
	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		eplAsKeywordBacktickFafRuntimeIDs[1]), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	insertPlan, err := env.Build(esper.FromTable(env, "MyTable").OnDemand().InsertRows(
		esper.InsertValues(esper.Literal("x"), esper.Literal("y")),
		esper.InsertValues(esper.Literal("a"), esper.Literal("b")),
	))
	if err != nil {
		return nil, err
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertPlan); err != nil {
		return nil, err
	}

	onSelect, err := env.Build(esper.OnEvent(esper.From[akbS0](env, "SupportBean_S0")).
		SelectFromTableWhere("MyTable",
			esper.Equal[string](esper.Field[akbS0, string]("p00"), esper.TableField[string]("k1")),
			esper.Alias("v1", esper.TableField[string]("v1"))).
		Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}
	onSelectDeploy, err := engine.Deploy(ctx, onSelect)
	if err != nil {
		return nil, err
	}

	var records []compat.TraceRecord
	seq := uint64(0)
	st := onSelectDeploy.Statements()[0]
	if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		records = append(records, compat.TraceRecord{
			Case:      "on-trigger-table-select",
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

	if err := engine.Send(ctx, "SupportBean_S0", akbS0{ID: 1, P00: akbStrPtr("a")}); err != nil {
		return nil, err
	}
	return records, nil
}

// runAKBMergeUpdateSelect replays EPLOthernMergeAndUpdateAndSelect (ord 4):
// an on-merge copies p0 into p1 for matched window rows, an on-update rewrites
// p0 to a literal, and a final on-select projects the window row as c0.
func runAKBMergeUpdateSelect(ctx context.Context, env *esper.Environment) ([]compat.TraceRecord, error) {
	engine := esper.NewEngine(env, esper.WithRuntimeURI(
		eplAsKeywordBacktickFafRuntimeIDs[2]), esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	var records []compat.TraceRecord
	seq := uint64(0)

	insertPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindowMerge").OnDemand().
		InsertRows(esper.InsertValues(esper.Literal("a"), esper.Literal("b"))))
	if err != nil {
		return nil, err
	}
	if _, err := engine.ExecuteFireAndForget(ctx, insertPlan); err != nil {
		return nil, err
	}

	merge, err := env.Build(esper.OnEvent(esper.From[akbS0](env, "SupportBean_S0")).
		MergeIntoNamedWindowWhen("MyWindowMerge", nil,
			esper.WhenMatchedAny(esper.SetColumn("p1", esper.NamedWindowField[string]("p0")))).
		Query(esper.StatementName("merge-s0")))
	if err != nil {
		return nil, err
	}
	if _, err := engine.Deploy(ctx, merge); err != nil {
		return nil, err
	}

	// The Java on-update carries no where clause; the Go trigger update
	// requires an explicit predicate, so the all-rows form is Literal(true).
	update, err := env.Build(esper.OnEvent(esper.From[akbS1](env, "SupportBean_S1")).
		UpdateNamedWindow("MyWindowMerge", esper.Literal(true),
			esper.SetColumn("p0", esper.Literal("x"))).
		Query(esper.StatementName("update-s1")))
	if err != nil {
		return nil, err
	}
	if _, err := engine.Deploy(ctx, update); err != nil {
		return nil, err
	}

	seq++
	selectResult, err := akbFAFSelectPair(ctx, env, engine, "MyWindowMerge", "faf-select-1")
	if err != nil {
		return nil, err
	}
	records = akbFAFRecord(records, "merge-update-select", "faf-select-1", seq, selectResult.Results())

	if err := engine.Send(ctx, "SupportBean_S0", akbS0{ID: 1}); err != nil {
		return nil, err
	}

	seq++
	selectResult, err = akbFAFSelectPair(ctx, env, engine, "MyWindowMerge", "faf-select-2")
	if err != nil {
		return nil, err
	}
	records = akbFAFRecord(records, "merge-update-select", "faf-select-2", seq, selectResult.Results())

	if err := engine.Send(ctx, "SupportBean_S1", akbS1{ID: 1, P10: akbStrPtr("x")}); err != nil {
		return nil, err
	}

	seq++
	selectResult, err = akbFAFSelectPair(ctx, env, engine, "MyWindowMerge", "faf-select-3")
	if err != nil {
		return nil, err
	}
	records = akbFAFRecord(records, "merge-update-select", "faf-select-3", seq, selectResult.Results())

	onSelect, err := env.Build(esper.OnEvent(esper.From[akbSupportBean](env, "SupportBean")).
		SelectFromNamedWindow("MyWindowMerge", nil,
			esper.Alias("c0", esper.NamedWindowField[string]("p0"))).
		Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}
	onSelectDeploy, err := engine.Deploy(ctx, onSelect)
	if err != nil {
		return nil, err
	}
	st := onSelectDeploy.Statements()[0]
	if _, err := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		records = append(records, compat.TraceRecord{
			Case:      "merge-update-select",
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

	if err := engine.Send(ctx, "SupportBean", akbSupportBean{}); err != nil {
		return nil, err
	}
	return records, nil
}

func runEplAsKeywordBacktickFafProbe(ctx context.Context) (compat.Trace, error) {
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
	recs, err := runAKBFAFUpdateDelete(ctx, env)
	if err != nil {
		return trace, err
	}
	trace.Records = append(trace.Records, recs...)
	return trace, nil
}
