package parity

import (
	"context"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the time-batch historical release — EPLDatabaseTimeBatch
// (java-runtime-fb0cea6fe1e469ee8237), the first of the three time-batch
// variants that share runtestTimeBatch (the OM and Compile siblings replay the
// identical runner over a SODA object model and an eplToModel round-trip and
// stay implemented-not-differential pending the Go compile surface).
//
// The statement joins the mytesttable historical source (driving side) with
// SupportBean#time_batch(10 sec): each arriving bean drives the historical
// lookup and the joined row is iterator-visible immediately while the bean
// itself stays buffered in the batch window; the scheduled boundary releases
// the accumulated rows to the listener as one new-data batch and clears the
// iterator, after which the next sends re-buffer silently.
//
// The Go side feeds the same canonical 10-row mytesttable fixture through a
// function-fed HistoricalProvider keyed on request.Trigger.Get("intPrimitive")
// (no database driver), reusing the epl-database-join seed rows and schema.
// The projected row shape is the statement's byte-exact select list — the
// nine canonical mytesttable columns mybigint/myint/myvarchar/mychar/mybool/
// mynumeric/mydecimal/mydouble/myreal — with NULL columns rendered by the
// differential protocol null marker (seed rows 7-10 have a NULL mynumeric, so
// the flushed mybigint-10 row carries exactly one null marker).
const eplDatabaseTimeBatchJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplDatabaseTimeBatchJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java",
}

var eplDatabaseTimeBatchJavaRuntimeIDs = []string{
	"java-runtime-fb0cea6fe1e469ee8237",
}

var eplDatabaseTimeBatchJavaExecutions = []string{
	"EPLDatabaseTimeBatch",
}

var eplDatabaseTimeBatchCases = []string{
	"timebatch",
}

// eplDatabaseTimeBatchBean mirrors the SupportBean join trigger.
type eplDatabaseTimeBatchBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func runEplDatabaseTimeBatchScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("epl-database-timebatch scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eplDatabaseTimeBatchCases {
		caseTrace, err := runEplDatabaseTimeBatchCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-database-timebatch case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runEplDatabaseTimeBatchCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	var listenerInts []int
	emitListener := func(batch esper.ResultBatch) {
		sequence++
		listenerInts = eplDatabaseTimeBatchResultInts(batch.New)
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			New:       compat.NormalizeResults(batch.New),
		})
	}
	countRecord := func(statement, name string, count int64) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: "count",
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      name,
			Count:     &count,
		})
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplDatabaseTimeBatchBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	rowSchema, err := eplDatabaseJoinFullRowSchema()
	if err != nil {
		return nil, err
	}
	provider := &eplDatabaseJoinProvider{
		schema:   rowSchema,
		rows:     eplDatabaseJoinSeedRowMaps(),
		keyField: "mybigint",
		key: func(request esper.HistoricalRequest) (int64, bool) {
			id, ok := request.Trigger.Get("intPrimitive").Any().(int)
			return int64(id), ok
		},
	}
	// The byte-exact select list: the nine canonical mytesttable columns read
	// from the historical side (s0), mirroring the Java statement's ALL_FIELDS
	// projection.
	query := esper.JoinMany(
		esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s0", "SupportBean", rowSchema, provider)),
		esper.JoinSource(esper.From[eplDatabaseTimeBatchBean](env, "SupportBean").Window(esper.TimeBatch(10*time.Second))),
	).Select(
		esper.SelectFrom(0, "mybigint", esper.Field[map[string]any, int64]("mybigint")),
		esper.SelectFrom(0, "myint", esper.Field[map[string]any, int]("myint")),
		esper.SelectFrom(0, "myvarchar", esper.Field[map[string]any, string]("myvarchar")),
		esper.SelectFrom(0, "mychar", esper.Field[map[string]any, string]("mychar")),
		esper.SelectFrom(0, "mybool", esper.Field[map[string]any, bool]("mybool")),
		esper.SelectFrom(0, "mynumeric", esper.Field[map[string]any, string]("mynumeric")),
		esper.SelectFrom(0, "mydecimal", esper.Field[map[string]any, string]("mydecimal")),
		esper.SelectFrom(0, "mydouble", esper.Field[map[string]any, float64]("mydouble")),
		esper.SelectFrom(0, "myreal", esper.Field[map[string]any, float64]("myreal")),
	).Query(esper.StatementName("s0"))
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplDatabaseTimeBatchJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return nil, err
	}
	statement := deployment.Statements()[0]
	deliveries := 0
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		deliveries++
		emitListener(batch)
		return nil
	}); err != nil {
		return nil, err
	}
	// The iterator view over the joined rows, ordered by composition order.
	iteratorInts := func() ([]int, error) {
		snapshot, err := statement.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		return eplDatabaseTimeBatchResultInts(snapshot.Results()), nil
	}
	// One count record per in-process ordered iterator compare, at the same
	// positions as the Java assertPropsPerRowIterator calls; record=false
	// probes the iterator without emitting (the epoch empty check).
	expectIterator := func(record bool, want ...int) error {
		got, err := iteratorInts()
		if err != nil {
			return err
		}
		if len(got) != len(want) {
			return fmt.Errorf("iterator myint = %v, want %v", got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				return fmt.Errorf("iterator myint = %v, want %v", got, want)
			}
		}
		if record {
			countRecord("flow", "iterator-rows", int64(len(want)))
		}
		return nil
	}
	sendBean := func(value int) error {
		return engine.SendEvent(ctx, eplDatabaseTimeBatchBean{IntPrimitive: value})
	}
	// env.advanceTime(0): the engine starts at the epoch and the iterator is
	// asserted empty in-process without a record.
	if err := expectIterator(false); err != nil {
		return nil, fmt.Errorf("iterator at epoch: %w", err)
	}
	// SB(10): the joined mybigint-10 row (myint 100) is iterator-visible
	// immediately while the trigger bean sits in the batch window.
	if err := sendBean(10); err != nil {
		return nil, err
	}
	if err := expectIterator(true, 100); err != nil {
		return nil, fmt.Errorf("iterator after SB(10): %w", err)
	}
	// SB(5): iterator [100,50] (milestone(0) is a documented no-op).
	if err := sendBean(5); err != nil {
		return nil, err
	}
	if err := expectIterator(true, 100, 50); err != nil {
		return nil, fmt.Errorf("iterator after SB(5): %w", err)
	}
	// SB(2): iterator [100,50,20].
	if err := sendBean(2); err != nil {
		return nil, err
	}
	if err := expectIterator(true, 100, 50, 20); err != nil {
		return nil, fmt.Errorf("iterator after SB(2): %w", err)
	}
	// The 10s boundary releases the batch of three trigger beans into the
	// join and the flush delivers exactly the three joined rows to the
	// listener as one new-data batch (the first release posts no old data).
	if err := engine.AdvanceTime(ctx, now.Add(10*time.Second)); err != nil {
		return nil, err
	}
	if deliveries != 1 {
		return nil, fmt.Errorf("deliveries at 10s = %d, want 1", deliveries)
	}
	if want := []int{100, 50, 20}; len(listenerInts) != len(want) {
		return nil, fmt.Errorf("flush myint = %v, want %v", listenerInts, want)
	} else {
		for index := range want {
			if listenerInts[index] != want[index] {
				return nil, fmt.Errorf("flush myint = %v, want %v", listenerInts, want)
			}
		}
	}
	// The released window cleared: the iterator is empty again; the empty
	// read is a recorded count (Java record: iterator-rows 0).
	if err := expectIterator(true); err != nil {
		return nil, fmt.Errorf("iterator after flush: %w", err)
	}
	// SB(9): the poll re-fires and the joined row is iterator-visible while
	// batched; the listener stays silent.
	if err := sendBean(9); err != nil {
		return nil, err
	}
	if err := expectIterator(true, 90); err != nil {
		return nil, fmt.Errorf("iterator after SB(9): %w", err)
	}
	// SB(8): iterator [90,80], still batched.
	if err := sendBean(8); err != nil {
		return nil, err
	}
	if err := expectIterator(true, 90, 80); err != nil {
		return nil, fmt.Errorf("iterator after SB(8): %w", err)
	}
	if deliveries != 1 {
		return nil, fmt.Errorf("deliveries = %d, want 1", deliveries)
	}
	return records, nil
}

// eplDatabaseTimeBatchResultInts extracts the ordered myint column from
// projected result rows for the in-process ordered compares.
func eplDatabaseTimeBatchResultInts(results []esper.Result) []int {
	values := make([]int, 0, len(results))
	for _, result := range results {
		row, ok := result.Row()
		if !ok {
			continue
		}
		switch myint := row.AsMap()["myint"].(type) {
		case int:
			values = append(values, myint)
		case int64:
			values = append(values, int(myint))
		}
	}
	return values
}
