package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the second EPLDatabaseJoin slice — five later
// executions of the same suite (deterministic historical joins over the
// shared mytesttable fixture):
//   - EPLDatabase2HistoricalStarInner (java-runtime-8961178ee540998a99f3):
//     two exclusion-keyed historical sides (<>) joined inner on theString;
//     only ("B",3) survives the double exclusion.
//   - EPLDatabaseJoinIndexNullType (java-runtime-a8180e298e71e8135b38): a
//     null-typed indexed key matches nothing in the mybigint unique index.
//   - EPLDatabaseWithPattern (java-runtime-3ddf3346fbe67a639057): a constant
//     historical stream joined to timer:interval(5 sec); the 9999 ms advance
//     stays silent, the 10 s advance fires again.
//   - EPLDatabaseVariables (java-runtime-0a048d0e0005df5f6279): the
//     historical key reads the queryvar variable; queryvar 5/6 poll the
//     mybigint-5/6 rows. Java's two deployments with reversed stream order
//     are builder text — one Go statement, two variable values.
//   - EPLDatabase3Stream (java-runtime-fc8c20664fc4787fc987): two
//     length-window streams plus an unrestricted historical re-polled per
//     trigger cycle.
//
// The Go side reuses the identical canonical 10-row mytesttable fixture of
// the first slice through eplDatabaseJoinSeedRowMaps() and a function-fed
// HistoricalProvider (no database driver); the Java oracle runs against the
// esper-mysql mysql:8.0 Docker fixture and records the identical canonical
// forms.
const eplDatabaseJoin2JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplDatabaseJoin2JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/database/EPLDatabaseJoin.java",
}

var eplDatabaseJoin2JavaRuntimeIDs = []string{
	"java-runtime-8961178ee540998a99f3", // 2historical-star-inner
	"java-runtime-a8180e298e71e8135b38", // join-index-null-type
	"java-runtime-3ddf3346fbe67a639057", // with-pattern
	"java-runtime-0a048d0e0005df5f6279", // variables
	"java-runtime-fc8c20664fc4787fc987", // 3stream
}

var eplDatabaseJoin2JavaExecutions = []string{
	"EPLDatabase2HistoricalStarInner", "EPLDatabaseJoinIndexNullType",
	"EPLDatabaseWithPattern", "EPLDatabaseVariables", "EPLDatabase3Stream",
}

var eplDatabaseJoin2Cases = []string{
	"2historical-star-inner", "join-index-null-type", "with-pattern",
	"variables", "3stream",
}

// eplDatabaseJoin2TriggerBean mirrors the SupportBean join trigger.
type eplDatabaseJoin2TriggerBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// eplDatabaseJoin2BeanTwo mirrors the SupportBeanTwo stream event.
type eplDatabaseJoin2BeanTwo struct {
	StringTwo       string `esper:"stringTwo"`
	IntPrimitiveTwo int    `esper:"intPrimitiveTwo"`
}

// eplDatabaseJoin2BeanA mirrors the SupportBean_A historical trigger.
type eplDatabaseJoin2BeanA struct {
	ID string `esper:"id"`
}

// eplDatabaseJoin2S0 is the pattern-root trigger carrier for the timer
// interval (the Java pattern never matches on its supertype event).
type eplDatabaseJoin2S0 struct {
	ID int `esper:"id"`
}

// eplDatabaseJoin2Provider is a function-fed historical row source over the
// canonical fixture rows with a per-row match predicate.
type eplDatabaseJoin2Provider struct {
	schema esper.Schema
	rows   []map[string]any
	match  func(map[string]any, esper.HistoricalRequest) bool
}

func (p *eplDatabaseJoin2Provider) Poll(ctx context.Context, request esper.HistoricalRequest) ([]esper.Event, error) {
	var out []esper.Event
	for _, row := range p.rows {
		if !p.match(row, request) {
			continue
		}
		event, err := esper.NewEvent(p.schema, row, request.Now)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, nil
}

// eplDatabaseJoin2IntPrimitiveKey keys the lookup on the trigger's
// intPrimitive.
func eplDatabaseJoin2IntPrimitiveKey(request esper.HistoricalRequest) (int64, bool) {
	id, ok := request.Trigger.Get("intPrimitive").Any().(int)
	return int64(id), ok
}

// eplDatabaseJoin2NullTypeKey exposes the null-typed fieldTypeNull indexed
// key; the any-typed column never carries an int64, so no row matches.
func eplDatabaseJoin2NullTypeKey(request esper.HistoricalRequest) (int64, bool) {
	key, ok := request.Trigger.Get("fieldTypeNull").Any().(int64)
	return key, ok
}

// eplDatabaseJoin2VariableKey keys the lookup on a variable value.
func eplDatabaseJoin2VariableKey(name string) func(esper.HistoricalRequest) (int64, bool) {
	return func(request esper.HistoricalRequest) (int64, bool) {
		raw, ok := request.Variables[name]
		if !ok {
			return 0, false
		}
		switch value := raw.Any().(type) {
		case int64:
			return value, true
		case int:
			return int64(value), true
		default:
			return 0, false
		}
	}
}

// eplDatabaseJoin2ColumnMatch matches rows whose numeric column equals the
// keyed lookup value.
func eplDatabaseJoin2ColumnMatch(column string, key func(esper.HistoricalRequest) (int64, bool)) func(map[string]any, esper.HistoricalRequest) bool {
	return func(row map[string]any, request esper.HistoricalRequest) bool {
		want, ok := key(request)
		if !ok {
			return false
		}
		switch value := row[column].(type) {
		case int64:
			return value == want
		case int:
			return int64(value) == want
		default:
			return false
		}
	}
}

// eplDatabaseJoin2ColumnExclusionMatch mirrors the ${intPrimitive} <>
// mytesttable.<column> SQL: rows survive when the column differs from the
// keyed lookup value.
func eplDatabaseJoin2ColumnExclusionMatch(column string, key func(esper.HistoricalRequest) (int64, bool)) func(map[string]any, esper.HistoricalRequest) bool {
	return func(row map[string]any, request esper.HistoricalRequest) bool {
		want, ok := key(request)
		if !ok {
			return false
		}
		switch value := row[column].(type) {
		case int64:
			return value != want
		case int:
			return int64(value) != want
		default:
			return false
		}
	}
}

// eplDatabaseJoin2ConstantMatch feeds the unrestricted constant historical
// sides (no WHERE parameterization).
func eplDatabaseJoin2ConstantMatch(row map[string]any, _ esper.HistoricalRequest) bool {
	return row != nil
}

func runEplDatabaseJoin2Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("epl-database-join-2 scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range eplDatabaseJoin2Cases {
		caseTrace, err := runEplDatabaseJoin2Case(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-database-join-2 case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runEplDatabaseJoin2Case(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
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
	case 0: // 2historical-star-inner — two exclusion-keyed historical sides
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplDatabaseJoin2TriggerBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		myVarSchema, err := esper.NewMapSchema("Hist2InnerMyVarChar", []esper.FieldSpec{
			esper.FieldDef("myvarchar", reflect.TypeOf("")),
		})
		if err != nil {
			return nil, err
		}
		// s1 excludes rows where ${intPrimitive} <> mybigint fails, s2 the
		// same against myint; both sides project myvarchar.
		providerOne := &eplDatabaseJoin2Provider{
			schema: myVarSchema,
			rows:   eplDatabaseJoinSeedRowMaps(),
			match:  eplDatabaseJoin2ColumnExclusionMatch("mybigint", eplDatabaseJoin2IntPrimitiveKey),
		}
		providerTwo := &eplDatabaseJoin2Provider{
			schema: myVarSchema,
			rows:   eplDatabaseJoinSeedRowMaps(),
			match:  eplDatabaseJoin2ColumnExclusionMatch("myint", eplDatabaseJoin2IntPrimitiveKey),
		}
		theString := esper.Field[eplDatabaseJoin2TriggerBean, string]("theString")
		query := esper.JoinMany(
			esper.JoinSource(esper.From[eplDatabaseJoin2TriggerBean](env, "SupportBean").Window(esper.KeepAll())),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s1", "SupportBean", myVarSchema, providerOne)),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s2", "SupportBean", myVarSchema, providerTwo)),
		).On(
			esper.OnSourcesEqual(1, esper.Field[map[string]any, string]("myvarchar"), 0, theString),
			esper.OnSourcesEqual(2, esper.Field[map[string]any, string]("myvarchar"), 0, theString),
		).Select(
			esper.SelectFrom(0, "a", theString),
			esper.SelectFrom(0, "b", esper.Field[eplDatabaseJoin2TriggerBean, int]("intPrimitive")),
			esper.SelectFrom(1, "c", esper.Field[map[string]any, string]("myvarchar")),
			esper.SelectFrom(2, "d", esper.Field[map[string]any, string]("myvarchar")),
		).Query(esper.StatementName("s0"))
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoin2JavaRuntimeIDs[caseIndex]),
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
		send := func(theString string, intPrimitive int) error {
			return engine.SendEvent(ctx, eplDatabaseJoin2TriggerBean{TheString: theString, IntPrimitive: intPrimitive})
		}
		// ("E1",1)/("A",1)/("A",10): every theString match is excluded by one
		// of the <> conditions, so the inner join stays empty.
		for _, negative := range []struct {
			theString    string
			intPrimitive int
		}{{"E1", 1}, {"A", 1}, {"A", 10}} {
			if err := send(negative.theString, negative.intPrimitive); err != nil {
				return nil, err
			}
		}
		if deliveries != 0 {
			return nil, fmt.Errorf("deliveries after negatives = %d, want 0", deliveries)
		}
		countRecord("flow", "negative-sends", 3)
		// ("B",3): only the mybigint-3/myint-20 "B" row survives both sides.
		if err := send("B", 3); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries after B = %d, want 1", deliveries)
		}
		// ("D",4): intPrimitive=4 drops the mybigint-4 "D" row from s1.
		if err := send("D", 4); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries after D = %d, want 1", deliveries)
		}
		countRecord("flow", "negative-sends", 1)
	case 1: // join-index-null-type — null-typed key matches nothing
		env := esper.NewEnvironment()
		if _, err := esper.RegisterMap(env, "InputEvent", []esper.FieldSpec{
			esper.FieldDef("id", reflect.TypeOf("")),
			esper.FieldDef("fieldTypeNull", reflect.TypeOf((*any)(nil)).Elem()),
		}); err != nil {
			return nil, err
		}
		myBigIntSchema, err := esper.NewMapSchema("Hist2NullTypeMyBigInt", []esper.FieldSpec{
			esper.FieldDef("mybigint", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return nil, err
		}
		provider := &eplDatabaseJoin2Provider{
			schema: myBigIntSchema,
			rows:   eplDatabaseJoinSeedRowMaps(),
			match:  eplDatabaseJoin2ColumnMatch("mybigint", eplDatabaseJoin2NullTypeKey),
		}
		query := esper.JoinMany(
			esper.JoinRecordSource(esper.FromAny(env, "InputEvent").Window(esper.Unique(esper.Field[any, string]("id")))),
			esper.JoinSource(esper.FromHistoricalOn[map[string]any](env, "s1", "InputEvent", myBigIntSchema, provider)),
		).Select(
			esper.SelectFrom(1, "mybigint", esper.Field[map[string]any, int64]("mybigint")),
		).Query(esper.StatementName("s0"))
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoin2JavaRuntimeIDs[caseIndex]),
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
		// The null-typed fieldTypeNull key matches nothing in the mybigint
		// unique index, so the 's1.mybigint is null' filter never fires.
		if err := engine.Send(ctx, "InputEvent", map[string]any{}); err != nil {
			return nil, err
		}
		if deliveries != 0 {
			return nil, fmt.Errorf("deliveries = %d, want 0", deliveries)
		}
		countRecord("flow", "capture-empty", 0)
	case 2: // with-pattern — constant historical joined to timer:interval
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplDatabaseJoin2S0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		myCharSchema, err := esper.NewMapSchema("Hist2PatternMyChar", []esper.FieldSpec{
			esper.FieldDef("mychar", reflect.TypeOf("")),
		})
		if err != nil {
			return nil, err
		}
		// The constant SQL pins mybigint = 2 (the mychar 'Y' row).
		provider := &eplDatabaseJoin2Provider{
			schema: myCharSchema,
			rows:   eplDatabaseJoinSeedRowMaps(),
			match: eplDatabaseJoin2ColumnMatch("mybigint",
				func(esper.HistoricalRequest) (int64, bool) { return 2, true }),
		}
		hist := esper.FromHistorical[map[string]any](env, "MyDBWithRetain", myCharSchema, provider)
		pattern := esper.TimerInterval(esper.From[eplDatabaseJoin2S0](env, "SupportBean_S0"), 5*time.Second).Every()
		query := esper.JoinMany(
			esper.JoinSource(hist),
			esper.JoinPatternSource(pattern),
		).Select(
			esper.SelectFrom(0, "mychar", esper.Field[map[string]any, string]("mychar")),
		).Query(esper.StatementName("s0"))
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoin2JavaRuntimeIDs[caseIndex]),
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
		// 5 s: first delivery {mychar:"Y"}.
		if err := engine.AdvanceTime(ctx, now.Add(5*time.Second)); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries at 5s = %d, want 1", deliveries)
		}
		// 9999 ms: inside the second interval — silent.
		if err := engine.AdvanceTime(ctx, now.Add(9999*time.Millisecond)); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries at 9999ms = %d, want 1", deliveries)
		}
		countRecord("flow", "silent-advances", 1)
		// 10 s: second delivery {mychar:"Y"}.
		if err := engine.AdvanceTime(ctx, now.Add(10*time.Second)); err != nil {
			return nil, err
		}
		if deliveries != 2 {
			return nil, fmt.Errorf("deliveries at 10s = %d, want 2", deliveries)
		}
	case 3: // variables — the historical key reads the queryvar variable
		env := esper.NewEnvironment()
		if err := env.RegisterVariable("queryvar", 0); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[eplDatabaseJoin2BeanA](env, "SupportBean_A"); err != nil {
			return nil, err
		}
		myIntSchema, err := esper.NewMapSchema("Hist2VarMyInt", []esper.FieldSpec{
			esper.FieldDef("myint", reflect.TypeOf(0)),
		})
		if err != nil {
			return nil, err
		}
		provider := &eplDatabaseJoin2Provider{
			schema: myIntSchema,
			rows:   eplDatabaseJoinSeedRowMaps(),
			match:  eplDatabaseJoin2ColumnMatch("mybigint", eplDatabaseJoin2VariableKey("queryvar")),
		}
		query := esper.Select(
			esper.FromHistoricalOn[map[string]any](env, "s0", "SupportBean_A", myIntSchema, provider),
			esper.Alias("myint", esper.Field[map[string]any, int]("myint")),
		).Query(esper.StatementName("s0"))
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoin2JavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(now),
		)
		defer func() { _ = engine.Close(context.Background()) }()
		// Java's 'on SupportBean set queryvar=intPrimitive' path deployment
		// and its two deployments with reversed stream order are builder
		// text: the observable contract is one statement delivering one row
		// per variable value (queryvar 5 then 6).
		if err := engine.SetVariable(ctx, "queryvar", 5); err != nil {
			return nil, err
		}
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
		if err := engine.SendEvent(ctx, eplDatabaseJoin2BeanA{ID: "A1"}); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries at queryvar=5 = %d, want 1", deliveries)
		}
		if err := engine.SetVariable(ctx, "queryvar", 6); err != nil {
			return nil, err
		}
		if err := engine.SendEvent(ctx, eplDatabaseJoin2BeanA{ID: "A1"}); err != nil {
			return nil, err
		}
		if deliveries != 2 {
			return nil, fmt.Errorf("deliveries at queryvar=6 = %d, want 2", deliveries)
		}
	case 4: // 3stream — two length windows plus the unrestricted historical
		env := esper.NewEnvironment()
		if _, err := esper.RegisterStruct[eplDatabaseJoin2BeanTwo](env, "SupportBeanTwo"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[eplDatabaseJoin2TriggerBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		myIntSchema, err := esper.NewMapSchema("Hist2ThreeStreamMyInt", []esper.FieldSpec{
			esper.FieldDef("myint", reflect.TypeOf(0)),
		})
		if err != nil {
			return nil, err
		}
		// Unparameterized SQL over the whole table; the unrestricted
		// historical is re-polled on every trigger cycle and filtered
		// against the current window states.
		provider := &eplDatabaseJoin2Provider{
			schema: myIntSchema,
			rows:   eplDatabaseJoinSeedRowMaps(),
			match:  eplDatabaseJoin2ConstantMatch,
		}
		sbStream := esper.From[eplDatabaseJoin2TriggerBean](env, "SupportBean").Window(esper.LengthWindow(1))
		sbtStream := esper.From[eplDatabaseJoin2BeanTwo](env, "SupportBeanTwo").Window(esper.LengthWindow(1))
		hist := esper.FromHistorical[map[string]any](env, "MyDB2ThreeStream", myIntSchema, provider)
		query := esper.JoinMany(
			esper.JoinSource(sbStream),
			esper.JoinSource(sbtStream),
			esper.JoinSource(hist),
		).On(
			esper.OnSourcesEqual(0, esper.Field[eplDatabaseJoin2TriggerBean, string]("theString"),
				1, esper.Field[eplDatabaseJoin2BeanTwo, string]("stringTwo")),
			esper.OnSourcesEqual(2, esper.Field[map[string]any, int]("myint"),
				1, esper.Field[eplDatabaseJoin2BeanTwo, int]("intPrimitiveTwo")),
		).Select(
			esper.SelectFrom(0, "theString", esper.Field[eplDatabaseJoin2TriggerBean, string]("theString")),
			esper.SelectFrom(1, "stringTwo", esper.Field[eplDatabaseJoin2BeanTwo, string]("stringTwo")),
			esper.SelectFrom(2, "myint", esper.Field[map[string]any, int]("myint")),
		).Query(esper.StatementName("s0"))
		engine := esper.NewEngine(env,
			esper.WithRuntimeURI(eplDatabaseJoin2JavaRuntimeIDs[caseIndex]),
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
		// T1/2: no myint=2 row — no output.
		if err := engine.SendEvent(ctx, eplDatabaseJoin2BeanTwo{StringTwo: "T1", IntPrimitiveTwo: 2}); err != nil {
			return nil, err
		}
		if err := engine.SendEvent(ctx, eplDatabaseJoin2TriggerBean{TheString: "T1", IntPrimitive: -1}); err != nil {
			return nil, err
		}
		if deliveries != 0 {
			return nil, fmt.Errorf("deliveries after T1 = %d, want 0", deliveries)
		}
		// T2/30: the myint-30 row joins the T2 pair.
		if err := engine.SendEvent(ctx, eplDatabaseJoin2BeanTwo{StringTwo: "T2", IntPrimitiveTwo: 30}); err != nil {
			return nil, err
		}
		if err := engine.SendEvent(ctx, eplDatabaseJoin2TriggerBean{TheString: "T2", IntPrimitive: -1}); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries after T2 = %d, want 1", deliveries)
		}
		// T3: SupportBean first (sbt still T2, no delivery), then
		// SupportBeanTwo — pins the per-trigger-cycle re-poll semantics.
		if err := engine.SendEvent(ctx, eplDatabaseJoin2TriggerBean{TheString: "T3", IntPrimitive: -1}); err != nil {
			return nil, err
		}
		if deliveries != 1 {
			return nil, fmt.Errorf("deliveries after T3 SupportBean = %d, want 1", deliveries)
		}
		if err := engine.SendEvent(ctx, eplDatabaseJoin2BeanTwo{StringTwo: "T3", IntPrimitiveTwo: 40}); err != nil {
			return nil, err
		}
		if deliveries != 2 {
			return nil, fmt.Errorf("deliveries after T3 = %d, want 2", deliveries)
		}
	default:
		return nil, fmt.Errorf("unsupported epl-database-join-2 case index %d", caseIndex)
	}
	return records, nil
}
