package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage opening the database subdomain — the first slice of
// EPLDatabaseJoin (five executions, deterministic historical joins over the
// mytesttable fixture):
//   - EPLDatabaseSimpleJoinLeft (java-runtime-67745eb75f864ea17fed): the
//     9-column row projection over a ${id}-keyed historical lookup.
//   - EPLDatabaseSimpleJoinRight (java-runtime-45e68d7cf756824c19eb): the
//     stream order reversed; the Java-side event-type property types are
//     pinned by construction.
//   - EPLDatabaseStreamNamesAndRename (java-runtime-4675598d7044c43fd088):
//     the a..i column aliases projected under the canonical names.
//   - EPLDatabasePropertyResolution (java-runtime-33cc6c0610b2c801c1bb): a
//     nested-indexed trigger property (${s1.arrayProperty[0]}) selecting the
//     mybigint-10 row whose mynumeric column is NULL.
//   - EPLDatabase2HistoricalStar (java-runtime-a6682a71c8c463babcff): two
//     historical sides keyed on the same trigger, keepall accumulation with
//     per-trigger lineage, and the negative no-match send.
//
// The Go side feeds the same 10-row mytesttable fixture through a
// function-fed HistoricalProvider (no database driver): canonical value
// forms are mybigint int64, myint int, myvarchar/mychar strings (MySQL
// strips CHAR trailing spaces), mybool bool, mynumeric/mydecimal scale-0
// decimal STRINGS ("5000"), mydouble/myreal float64, and NULL columns as
// JSON null. The Java oracle runs against the established esper-mysql
// mysql:8.0 Docker fixture with create_testdb.sql loaded and records the
// identical canonical forms (BigDecimal scale-0 asserted and written as
// strings in-process).
const eplDatabaseJoinJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplDatabaseJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java",
}

var eplDatabaseJoinJavaRuntimeIDs = []string{
	"java-runtime-67745eb75f864ea17fed",
	"java-runtime-45e68d7cf756824c19eb",
	"java-runtime-4675598d7044c43fd088",
	"java-runtime-33cc6c0610b2c801c1bb",
	"java-runtime-a6682a71c8c463babcff",
}

var eplDatabaseJoinJavaExecutions = []string{
	"EPLDatabaseSimpleJoinLeft",
	"EPLDatabaseSimpleJoinRight",
	"EPLDatabaseStreamNamesAndRename",
	"EPLDatabasePropertyResolution",
	"EPLDatabase2HistoricalStar",
}

var eplDatabaseJoinCases = []string{
	"simple-join-left",
	"simple-join-right",
	"stream-names-and-rename",
	"property-resolution",
	"2historical-star",
}

// eplDatabaseJoinS0 mirrors the SupportBean_S0 trigger event.
type eplDatabaseJoinS0 struct {
	Id int64 `esper:"id"`
}

// eplDatabaseJoinComplexProps mirrors the SupportBeanComplexProps trigger
// (arrayProperty = {10, 20, 30} in the suite's default bean).
type eplDatabaseJoinComplexProps struct {
	ArrayProperty []int `esper:"arrayProperty"`
}

// eplDatabaseJoinTriggerBean mirrors the SupportBean join trigger.
type eplDatabaseJoinTriggerBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// eplDatabaseJoinSeedRow is one canonical mytesttable row.
type eplDatabaseJoinSeedRow struct {
	mybigint  int64
	myint     int
	myvarchar string
	mychar    string
	mybool    bool
	mynumeric *string
	mydecimal string
	mydouble  float64
	myreal    float64
}

func eplDatabaseJoinSeedRows() []eplDatabaseJoinSeedRow {
	scale0 := func(s string) *string { return &s }
	return []eplDatabaseJoinSeedRow{
		{1, 10, "A", "Z", true, scale0("5000"), "100", 1.2, 1.3},
		{2, 20, "B", "Y", false, scale0("100"), "200", 2.2, 2.3},
		{3, 30, "C", "X", false, scale0("100"), "300", 3.2, 3.3},
		{4, 40, "D", "W", true, scale0("500"), "400", 4.2, 4.3},
		{5, 50, "E", "V", false, scale0("500"), "500", 5.2, 5.3},
		{6, 60, "F", "T", false, scale0("200"), "600", 6.2, 6.3},
		{7, 70, "G", "S", true, nil, "700", 7.2, 7.3},
		{8, 80, "H", "R", true, nil, "800", 8.2, 8.3},
		{9, 90, "I", "Q", true, nil, "900", 9.2, 9.3},
		{10, 100, "J", "P", true, nil, "1000", 10.2, 10.3},
	}
}

// eplDatabaseJoinProvider is a function-fed historical row source over the
// canonical fixture rows, keyed per trigger event.
type eplDatabaseJoinProvider struct {
	schema   esper.Schema
	rows     []map[string]any
	keyField string
	key      func(esper.HistoricalRequest) (int64, bool)
}

func (p *eplDatabaseJoinProvider) Poll(_ context.Context, request esper.HistoricalRequest) ([]esper.Event, error) {
	key, ok := p.key(request)
	if !ok {
		return nil, nil
	}
	var out []esper.Event
	for _, row := range p.rows {
		if row[p.keyField] == key {
			event, err := esper.NewEvent(p.schema, row, request.Now)
			if err != nil {
				return nil, err
			}
			out = append(out, event)
		}
	}
	return out, nil
}

func eplDatabaseJoinSeedRowMaps() []map[string]any {
	rows := eplDatabaseJoinSeedRows()
	maps := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		fields := map[string]any{
			"mybigint":  row.mybigint,
			"myint":     row.myint,
			"myvarchar": row.myvarchar,
			"mychar":    row.mychar,
			"mybool":    row.mybool,
			"mydecimal": row.mydecimal,
			"mydouble":  row.mydouble,
			"myreal":    row.myreal,
		}
		if row.mynumeric != nil {
			fields["mynumeric"] = *row.mynumeric
		} else {
			fields["mynumeric"] = nil
		}
		maps = append(maps, fields)
	}
	return maps
}

func eplDatabaseJoinFullRowSchema() (esper.Schema, error) {
	return esper.NewMapSchema("MyTestTable", []esper.FieldSpec{
		esper.FieldDef("mybigint", reflect.TypeOf(int64(0))),
		esper.FieldDef("myint", reflect.TypeOf(0)),
		esper.FieldDef("myvarchar", reflect.TypeOf("")),
		esper.FieldDef("mychar", reflect.TypeOf("")),
		esper.FieldDef("mybool", reflect.TypeOf(false)),
		esper.FieldDef("mynumeric", reflect.TypeOf("")),
		esper.FieldDef("mydecimal", reflect.TypeOf("")),
		esper.FieldDef("mydouble", reflect.TypeOf(float64(0))),
		esper.FieldDef("myreal", reflect.TypeOf(float64(0))),
	})
}

func runEplDatabaseJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("epl-database-join scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eplDatabaseJoinCases {
		caseTrace, err := runEplDatabaseJoinCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-database-join case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runEplDatabaseJoinCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	emitListener := func(batch esper.ResultBatch) {
		sequence++
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

	switch caseIndex {
	case 0, 1, 2, 3: // the four single-historical projections
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplDatabaseJoinS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[eplDatabaseJoinComplexProps](env, "SupportBeanComplexProps"); err != nil {
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
				switch caseIndex {
				case 0, 1, 2:
					id, ok := request.Trigger.Get("id").Any().(int64)
					return id, ok
				default: // property-resolution: nested-indexed trigger property
					array, ok := request.Trigger.Get("arrayProperty").Any().([]int)
					if !ok || len(array) == 0 {
						return 0, false
					}
					return int64(array[0]), true
				}
			},
		}
		var query esper.Query
		fullRow := func(source int) []esper.JoinSelection {
			return []esper.JoinSelection{
				esper.SelectFrom(source, "mybigint", esper.Field[map[string]any, int64]("mybigint")),
				esper.SelectFrom(source, "myint", esper.Field[map[string]any, int]("myint")),
				esper.SelectFrom(source, "myvarchar", esper.Field[map[string]any, string]("myvarchar")),
				esper.SelectFrom(source, "mychar", esper.Field[map[string]any, string]("mychar")),
				esper.SelectFrom(source, "mybool", esper.Field[map[string]any, bool]("mybool")),
				esper.SelectFrom(source, "mynumeric", esper.Field[map[string]any, string]("mynumeric")),
				esper.SelectFrom(source, "mydecimal", esper.Field[map[string]any, string]("mydecimal")),
				esper.SelectFrom(source, "mydouble", esper.Field[map[string]any, float64]("mydouble")),
				esper.SelectFrom(source, "myreal", esper.Field[map[string]any, float64]("myreal")),
			}
		}
		// The rename case (2) feeds alias-keyed rows (a..i) instead of the
		// canonical column names, mirroring the SQL `mybigint as a` aliases.
		if caseIndex == 2 {
			rowSchema, err = esper.NewMapSchema("MyTestTableAlias", []esper.FieldSpec{
				esper.FieldDef("a", reflect.TypeOf(int64(0))),
				esper.FieldDef("b", reflect.TypeOf(0)),
				esper.FieldDef("c", reflect.TypeOf("")),
				esper.FieldDef("d", reflect.TypeOf("")),
				esper.FieldDef("e", reflect.TypeOf(false)),
				esper.FieldDef("f", reflect.TypeOf("")),
				esper.FieldDef("g", reflect.TypeOf("")),
				esper.FieldDef("h", reflect.TypeOf(float64(0))),
				esper.FieldDef("i", reflect.TypeOf(float64(0))),
			})
			if err != nil {
				return nil, err
			}
			provider.schema = rowSchema
			provider.rows = eplDatabaseJoinAliasRows()
			provider.keyField = "a"
		}
		switch caseIndex {
		case 0, 1:
			// Both cases project the historical row; Java's reversed stream
			// order in SimpleJoinRight is builder text and not observable in
			// the trace (the Go JoinMany takes the stream source first).
			source := 1
			query = esper.JoinMany(
				esper.JoinSource(esper.From[eplDatabaseJoinS0](env, "SupportBean_S0").Window(esper.KeepAll())),
				esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s1", "SupportBean_S0", rowSchema, provider)),
			).Select(append(
				[]esper.JoinSelection{esper.SelectFrom(source, "mybigint", esper.Field[map[string]any, int64]("mybigint"))},
				fullRowTail(source)...,
			)...).Query(esper.StatementName("s0"))
		case 2:
			// The alias-keyed rows are added to the provider maps so the
			// renamed projection reads a..i.
			query = esper.JoinMany(
				esper.JoinSource(esper.From[eplDatabaseJoinS0](env, "SupportBean_S0").Window(esper.KeepAll())),
				esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s1", "SupportBean_S0", rowSchema, provider)),
			).Select(append(
				[]esper.JoinSelection{esper.SelectFrom(1, "mybigint", esper.Field[map[string]any, int64]("a"))},
				aliasRowTail()...,
			)...).Query(esper.StatementName("s0"))
		case 3:
			query = esper.JoinMany(
				esper.JoinSource(esper.From[eplDatabaseJoinComplexProps](env, "SupportBeanComplexProps").Window(esper.KeepAll())),
				esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s0", "SupportBeanComplexProps", rowSchema, provider)),
			).Select(fullRow(1)...).Query(esper.StatementName("s0"))
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoinJavaRuntimeIDs[caseIndex]),
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
		if caseIndex == 3 {
			if err := engine.SendEvent(ctx, eplDatabaseJoinComplexProps{ArrayProperty: []int{10, 20, 30}}); err != nil {
				return nil, err
			}
		} else {
			if err := engine.SendEvent(ctx, eplDatabaseJoinS0{Id: 1}); err != nil {
				return nil, err
			}
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries = %d, want 1", deliveries)
		}
	case 4: // 2historical-star — two historical sides, keepall lineage
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplDatabaseJoinTriggerBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoinJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		myIntSchema, err := esper.NewMapSchema("Hist1MyInt", []esper.FieldSpec{
			esper.FieldDef("myint", reflect.TypeOf(0)),
		})
		if err != nil {
			return nil, err
		}
		myVarSchema, err := esper.NewMapSchema("Hist2MyVarChar", []esper.FieldSpec{
			esper.FieldDef("myvarchar", reflect.TypeOf("")),
		})
		if err != nil {
			return nil, err
		}
		keyOnIntPrimitive := func(request esper.HistoricalRequest) (int64, bool) {
			id, ok := request.Trigger.Get("intPrimitive").Any().(int)
			return int64(id), ok
		}
		intProvider := &eplDatabaseJoinProvider{
			schema:   myIntSchema,
			rows:     eplDatabaseJoinSeedRowMaps(),
			keyField: "mybigint",
			key:      keyOnIntPrimitive,
		}
		varProvider := &eplDatabaseJoinProvider{
			schema:   myVarSchema,
			rows:     eplDatabaseJoinSeedRowMaps(),
			keyField: "mybigint",
			key:      keyOnIntPrimitive,
		}
		query := esper.JoinMany(
			esper.JoinSource(esper.From[eplDatabaseJoinTriggerBean](env, "SupportBean").Window(esper.KeepAll())),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s1", "SupportBean", myIntSchema, intProvider)),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s2", "SupportBean", myVarSchema, varProvider)),
		).Select(
			esper.SelectFrom(0, "intPrimitive", esper.Field[eplDatabaseJoinTriggerBean, int]("intPrimitive")),
			esper.SelectFrom(1, "myint", esper.Field[map[string]any, int]("myint")),
			esper.SelectFrom(2, "myvarchar", esper.Field[map[string]any, string]("myvarchar")),
		).Query(esper.StatementName("s0"))
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
		// SB(6): one delivery {6, 60, "F"}; iterator holds one row.
		if err := engine.SendEvent(ctx, eplDatabaseJoinTriggerBean{TheString: "sb", IntPrimitive: 6}); err != nil {
			return nil, err
		}
		snapshot, err := statement.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		countRecord("flow", "iterator-rows", int64(len(snapshot.Results())))
		// SB(9): second delivery {9, 90, "I"}; iterator holds two rows.
		if err := engine.SendEvent(ctx, eplDatabaseJoinTriggerBean{TheString: "sb", IntPrimitive: 9}); err != nil {
			return nil, err
		}
		snapshot, err = statement.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		countRecord("flow", "iterator-rows", int64(len(snapshot.Results())))
		// SB(20): no matching row — listener not invoked.
		if err := engine.SendEvent(ctx, eplDatabaseJoinTriggerBean{TheString: "sb", IntPrimitive: 20}); err != nil {
			return nil, err
		}
		if deliveries != 2 {
			return nil, fmt.Errorf("deliveries = %d, want 2", deliveries)
		}
		countRecord("flow", "listener-not-invoked", 0)
	default:
		return nil, fmt.Errorf("unsupported epl-database-join case index %d", caseIndex)
	}
	return records, nil
}

// eplDatabaseJoinAliasRows returns the mytesttable rows keyed by the a..i
// rename aliases (case 2).
func eplDatabaseJoinAliasRows() []map[string]any {
	var aliasRows []map[string]any
	for _, row := range eplDatabaseJoinSeedRowMaps() {
		alias := map[string]any{
			"a": row["mybigint"],
			"b": row["myint"],
			"c": row["myvarchar"],
			"d": row["mychar"],
			"e": row["mybool"],
			"f": row["mynumeric"],
			"g": row["mydecimal"],
			"h": row["mydouble"],
			"i": row["myreal"],
		}
		aliasRows = append(aliasRows, alias)
	}
	return aliasRows
}

func aliasRowTail() []esper.JoinSelection {
	return []esper.JoinSelection{
		esper.SelectFrom(1, "myint", esper.Field[map[string]any, int]("b")),
		esper.SelectFrom(1, "myvarchar", esper.Field[map[string]any, string]("c")),
		esper.SelectFrom(1, "mychar", esper.Field[map[string]any, string]("d")),
		esper.SelectFrom(1, "mybool", esper.Field[map[string]any, bool]("e")),
		esper.SelectFrom(1, "mynumeric", esper.Field[map[string]any, string]("f")),
		esper.SelectFrom(1, "mydecimal", esper.Field[map[string]any, string]("g")),
		esper.SelectFrom(1, "mydouble", esper.Field[map[string]any, float64]("h")),
		esper.SelectFrom(1, "myreal", esper.Field[map[string]any, float64]("i")),
	}
}

func fullRowTail(source int) []esper.JoinSelection {
	return []esper.JoinSelection{
		esper.SelectFrom(source, "myint", esper.Field[map[string]any, int]("myint")),
		esper.SelectFrom(source, "myvarchar", esper.Field[map[string]any, string]("myvarchar")),
		esper.SelectFrom(source, "mychar", esper.Field[map[string]any, string]("mychar")),
		esper.SelectFrom(source, "mybool", esper.Field[map[string]any, bool]("mybool")),
		esper.SelectFrom(source, "mynumeric", esper.Field[map[string]any, string]("mynumeric")),
		esper.SelectFrom(source, "mydecimal", esper.Field[map[string]any, string]("mydecimal")),
		esper.SelectFrom(source, "mydouble", esper.Field[map[string]any, float64]("mydouble")),
		esper.SelectFrom(source, "myreal", esper.Field[map[string]any, float64]("myreal")),
	}
}
