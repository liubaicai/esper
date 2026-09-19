package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_insert_into_wrapper.go replays the three EPLInsertIntoWrapper
// executions (ords 0-2) against the pinned Java oracle:
//
//   - wrapper-bean (ord 0 EPLInsertIntoWrapperBean): two producers into one
//     shared wrapper type. i1 'insert into WrappedBean select *,
//     intPrimitive as p0 from SupportBean' -> {E1,1,p0:1}; i2 'insert into
//     WrappedBean select sb from SupportEventContainsSupportBean sb' ->
//     {E2,2,p0:null} ('sb' resolves to the nested-bean PROPERTY, not the
//     stream alias; the unprovided p0 column projects null).
//   - three-stream-wrapper (ord 1 EPLInsertInto3StreamWrapper): three
//     chained 'insert into select irstream *' producers over #length(2)
//     with || concat columns; listener on s2 sees e3's new row and e1's
//     fully-wrapped old row in the same invocation (the rstream cascade
//     removes e1's row through all three windows).
//   - split-fork-join (ord 2 EPLInsertIntoOnSplitForkJoin): a single module
//     of on-trigger multi-clause splits — transpose(UDF(event)) -> MyEvent,
//     where-filtered fork branches, and 'output all' dual inserts.
//     S0(1,T,T,F) -> final id=1; S0(1,T,T,T) -> final NOT invoked.
//
// Approved differences (observably identical to the Java EPL):
//   - Esper auto-creates wrapper event types; Go pre-registers the targets
//     (WrappedBean/StreamA/StreamB/StreamC as map schemas, the split-fork
//     streams as the MyEvent struct schema).
//   - 'select *, intPrimitive as p0' and 'select sb' use the Transpose +
//     companion-Alias form: a companion column unlocks struct-payload
//     flattening into a Map target, and Alias("p0", NullLiteral[int]())
//   - 'select irstream *' is WithOldStream() on the insert-into route:
//     Esper's insert-into route selector is istream-only regardless of
//     the select-clause keyword, so the route posts new rows and each
//     downstream #length(2) window expires the matching row itself.
//   - 'on X insert into Y select * where cond' is SplitFirst/
//     SplitIntoWhen; 'output all' is SplitAll; the transpose(UDF) producer
//     is a continuous Select+Transpose+InsertInto (TriggerStream has no
//     plain stream insert-into — observably identical for stream targets).
//   - milestone ops are no-ops (no observable records).

const eplInsertIntoWrapperJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoWrapperJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoWrapper.java",
}

var eplInsertIntoWrapperJavaRuntimeIDs = []string{
	"java-runtime-79356b0865c8de3ace17",
	"java-runtime-32434556dcbd672d7cf7",
	"java-runtime-581a1f109ff2588c6cde",
}

var eplInsertIntoWrapperJavaExecutions = []string{
	"EPLInsertIntoWrapperBean",
	"EPLInsertInto3StreamWrapper",
	"EPLInsertIntoOnSplitForkJoin",
}

var eplInsertIntoWrapperCases = []string{
	"wrapper-bean",
	"three-stream-wrapper",
	"split-fork-join",
}

var eplInsertIntoWrapperCaseRuntimeIDs = map[string]string{
	"wrapper-bean":         "java-runtime-79356b0865c8de3ace17",
	"three-stream-wrapper": "java-runtime-32434556dcbd672d7cf7",
	"split-fork-join":      "java-runtime-581a1f109ff2588c6cde",
}

// wrapperBean mirrors SupportBean's asserted fields.
type wrapperBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// wrapperContainer mirrors SupportEventContainsSupportBean: a bean-typed
// 'sb' property carrying an inner SupportBean.
type wrapperContainer struct {
	Sb wrapperBean `esper:"sb"`
}

// wrapperS0 mirrors SupportBean_S0 (id + p00..p03 string properties).
type wrapperS0 struct {
	ID  string `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P02 string `esper:"p02"`
	P03 string `esper:"p03"`
}

// wrapperMyEvent mirrors the Java MyEvent bean produced by
// EPLInsertIntoWrapper.transpose(SupportBean_S0).
type wrapperMyEvent struct {
	ID        int  `esper:"id"`
	PropOne   bool `esper:"propOne"`
	PropTwo   bool `esper:"propTwo"`
	PropThree bool `esper:"propThree"`
}

// runEplInsertIntoWrapperScenario replays the three EPLInsertIntoWrapper
// executions: wrapper-type producers (wildcard + extra column, nested-bean
// fragment select), a three-hop irstream wrapper chain, and an on-trigger
// split/fork/join module.
func runEplInsertIntoWrapperScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplInsertIntoWrapperCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplInsertIntoWrapperCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl insert-into wrapper case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl insert-into wrapper scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplInsertIntoWrapperCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plans, err := buildEplInsertIntoWrapperCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplInsertIntoWrapperCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequences := make(map[string]uint64)
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return trace, err
		}
		for _, statement := range deployment.Statements() {
			name := statement.Name()
			if name != "i1" && name != "i2" && name != "s2" && name != "final" {
				continue
			}
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequences["listener:"+stmt.Name()]++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  sequences["listener:"+stmt.Name()],
					Time:      engine.Now().UTC().Format(time.RFC3339Nano),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); err != nil {
				return trace, err
			}
		}
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeEplInsertIntoWrapperPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported epl insert-into wrapper step op %q", step.Op)
		}
	}
	return trace, nil
}

// buildEplInsertIntoWrapperCase registers the case's schemas and targets
// and returns the deployment plans in pinned order (producers before
// consumers, matching the Java deployment order).
func buildEplInsertIntoWrapperCase(env *esper.Environment, caseName string) ([]esper.Plan, error) {
	switch caseName {
	case "wrapper-bean":
		if _, err := esper.RegisterStruct[wrapperBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[wrapperContainer](env, "SupportEventContainsSupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterMap(env, "WrappedBean", []esper.FieldSpec{
			esper.FieldDef("theString", reflect.TypeOf("")),
			esper.FieldDef("intPrimitive", reflect.TypeOf(0)),
			esper.FieldDef("p0", reflect.TypeOf(0)),
		}); err != nil {
			return nil, err
		}
		// @name('i1') @public insert into WrappedBean select *,
		// intPrimitive as p0 from SupportBean — Transpose + companion
		// unlocks struct-payload flattening into the Map target.
		i1Plan, err := env.Build(
			esper.From[wrapperBean](env, "SupportBean").AsRecord().Select(
				esper.Selection{Expr: esper.Transpose[wrapperBean](esper.EventValue[wrapperBean]())},
				esper.Alias("p0", esper.Field[wrapperBean, int]("intPrimitive")),
			).InsertInto("WrappedBean", esper.StatementName("i1")))
		if err != nil {
			return nil, err
		}
		// @name('i2') @public insert into WrappedBean select sb from
		// SupportEventContainsSupportBean sb — 'sb' resolves to the
		// nested-bean property; Transpose flattens it and the NullLiteral
		// companion pins the unprovided p0 column to null.
		i2Plan, err := env.Build(
			esper.From[wrapperContainer](env, "SupportEventContainsSupportBean").AsRecord().Select(
				esper.Selection{Expr: esper.Transpose[wrapperBean](esper.Field[wrapperContainer, wrapperBean]("sb"))},
				esper.Alias("p0", esper.NullLiteral[int]()),
			).InsertInto("WrappedBean", esper.StatementName("i2")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{i1Plan, i2Plan}, nil

	case "three-stream-wrapper":
		if _, err := esper.RegisterStruct[wcwSimple](env, "SupportBeanSimple"); err != nil {
			return nil, err
		}
		for _, target := range []struct {
			name   string
			fields []esper.FieldSpec
		}{
			{"StreamA", []esper.FieldSpec{
				esper.FieldDef("myString", reflect.TypeOf("")),
				esper.FieldDef("myInt", reflect.TypeOf(int32(0))),
			}},
			{"StreamB", []esper.FieldSpec{
				esper.FieldDef("myString", reflect.TypeOf("")),
				esper.FieldDef("myInt", reflect.TypeOf(int32(0))),
				esper.FieldDef("propA", reflect.TypeOf("")),
			}},
			{"StreamC", []esper.FieldSpec{
				esper.FieldDef("myString", reflect.TypeOf("")),
				esper.FieldDef("myInt", reflect.TypeOf(int32(0))),
				esper.FieldDef("propA", reflect.TypeOf("")),
				esper.FieldDef("propB", reflect.TypeOf("")),
			}},
		} {
			if _, err := esper.RegisterMap(env, target.name, target.fields); err != nil {
				return nil, err
			}
		}
		// @name('s0') @public insert into StreamA select irstream * from
		// SupportBeanSimple#length(2) — no selections preserves identity;
		// WithOldStream selects the irstream output while the route posts
		// istream-only (Esper's plain insert-into route selector).
		s0Plan, err := env.Build(
			esper.From[wcwSimple](env, "SupportBeanSimple").
				Window(esper.LengthWindow(2)).
				InsertInto("StreamA", esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		// @name('s1') @public insert into StreamB select irstream *,
		// myString||'A' as propA from StreamA#length(2)
		s1Plan, err := env.Build(
			esper.FromAny(env, "StreamA").
				Window(esper.LengthWindow(2)).
				Select(
					esper.Selection{Expr: esper.Transpose[map[string]any](esper.EventValue[map[string]any]())},
					esper.Alias("propA", esper.Concat(
						esper.Field[map[string]any, string]("myString"),
						esper.Literal("A"),
					)),
				).InsertInto("StreamB", esper.StatementName("s1"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		// @name('s2') @public insert into StreamC select irstream *,
		// propA||'B' as propB from StreamB#length(2)
		s2Plan, err := env.Build(
			esper.FromAny(env, "StreamB").
				Window(esper.LengthWindow(2)).
				Select(
					esper.Selection{Expr: esper.Transpose[map[string]any](esper.EventValue[map[string]any]())},
					esper.Alias("propB", esper.Concat(
						esper.Field[map[string]any, string]("propA"),
						esper.Literal("B"),
					)),
				).InsertInto("StreamC", esper.StatementName("s2"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{s0Plan, s1Plan, s2Plan}, nil

	case "split-fork-join":
		if _, err := esper.RegisterStruct[wrapperS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		for _, name := range []string{
			"AStream", "BStream", "DStreamOne", "DStreamTwo",
			"FStreamOne", "FStreamTwo", "FinalStream", "otherstream",
		} {
			if _, err := esper.RegisterStruct[wrapperMyEvent](env, name); err != nil {
				return nil, err
			}
		}
		// @Name('A') on SupportBean_S0 event insert into AStream select
		// transpose(EPLInsertIntoWrapper.transpose(event)) — the UDF maps
		// (id,p00,p01,p02) -> MyEvent; the transpose builtin wraps the
		// returned object as a bean event.
		transpose := func(s0 wrapperS0) wrapperMyEvent {
			return wrapperMyEvent{
				ID:        atoiWrapper(s0.ID),
				PropOne:   s0.P00 == "true",
				PropTwo:   s0.P01 == "true",
				PropThree: s0.P02 == "true",
			}
		}
		aPlan, err := env.Build(
			esper.From[wrapperS0](env, "SupportBean_S0").AsRecord().Select(
				esper.Selection{Expr: esper.Transpose[wrapperMyEvent](
					esper.Func1[wrapperS0, wrapperMyEvent]("transpose", transpose, esper.EventValue[wrapperS0]()),
				)},
			).InsertInto("AStream", esper.StatementName("A")))
		if err != nil {
			return nil, err
		}
		// @Name('B') on AStream insert into BStream select * where propOne
		bPlan, err := env.Build(
			esper.OnEvent(esper.From[wrapperMyEvent](env, "AStream")).SplitFirst(
				esper.SplitIntoWhen(esper.Field[wrapperMyEvent, bool]("propOne"), "BStream"),
			).Query(esper.StatementName("B")))
		if err != nil {
			return nil, err
		}
		// @Name('C') select * from AStream — listenerless consumer.
		cPlan, err := env.Build(
			esper.From[wrapperMyEvent](env, "AStream").Query(esper.StatementName("C")))
		if err != nil {
			return nil, err
		}
		// @Name('D') on BStream insert into DStreamOne select * where
		// propTwo insert into DStreamTwo select * where not propTwo
		dPlan, err := env.Build(
			esper.OnEvent(esper.From[wrapperMyEvent](env, "BStream")).SplitFirst(
				esper.SplitIntoWhen(esper.Field[wrapperMyEvent, bool]("propTwo"), "DStreamOne"),
				esper.SplitIntoWhen(esper.Not(esper.Field[wrapperMyEvent, bool]("propTwo")), "DStreamTwo"),
			).Query(esper.StatementName("D")))
		if err != nil {
			return nil, err
		}
		// @Name('E') on DStreamTwo insert into FinalStream select *
		// insert into otherstream select * output all
		ePlan, err := env.Build(
			esper.OnEvent(esper.From[wrapperMyEvent](env, "DStreamTwo")).SplitAll(
				esper.SplitInto("FinalStream"),
				esper.SplitInto("otherstream"),
			).Query(esper.StatementName("E")))
		if err != nil {
			return nil, err
		}
		// @Name('F') on DStreamOne insert into FStreamOne select * where
		// propThree insert into FStreamTwo select * where not propThree
		fPlan, err := env.Build(
			esper.OnEvent(esper.From[wrapperMyEvent](env, "DStreamOne")).SplitFirst(
				esper.SplitIntoWhen(esper.Field[wrapperMyEvent, bool]("propThree"), "FStreamOne"),
				esper.SplitIntoWhen(esper.Not(esper.Field[wrapperMyEvent, bool]("propThree")), "FStreamTwo"),
			).Query(esper.StatementName("F")))
		if err != nil {
			return nil, err
		}
		// @Name('G') on FStreamTwo insert into FinalStream select *
		// insert into otherstream select * output all
		gPlan, err := env.Build(
			esper.OnEvent(esper.From[wrapperMyEvent](env, "FStreamTwo")).SplitAll(
				esper.SplitInto("FinalStream"),
				esper.SplitInto("otherstream"),
			).Query(esper.StatementName("G")))
		if err != nil {
			return nil, err
		}
		// @name('final') select * from FinalStream
		finalPlan, err := env.Build(
			esper.From[wrapperMyEvent](env, "FinalStream").Query(esper.StatementName("final")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{aPlan, bPlan, cPlan, dPlan, ePlan, fPlan, gPlan, finalPlan}, nil
	}
	return nil, fmt.Errorf("unknown epl insert-into wrapper case %q", caseName)
}

func atoiWrapper(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func decodeEplInsertIntoWrapperPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    string `json:"theString"`
			IntPrimitive int    `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		return wrapperBean{TheString: payload.TheString, IntPrimitive: payload.IntPrimitive}, nil
	case "SupportEventContainsSupportBean":
		var payload struct {
			Sb struct {
				TheString    string `json:"theString"`
				IntPrimitive int    `json:"intPrimitive"`
			} `json:"sb"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		return wrapperContainer{Sb: wrapperBean{
			TheString:    payload.Sb.TheString,
			IntPrimitive: payload.Sb.IntPrimitive,
		}}, nil
	case "SupportBeanSimple":
		var payload struct {
			MyString string `json:"myString"`
			MyInt    int32  `json:"myInt"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		return wcwSimple{MyString: payload.MyString, MyInt: payload.MyInt}, nil
	case "SupportBean_S0":
		var payload struct {
			ID  string `json:"id"`
			P00 string `json:"p00"`
			P01 string `json:"p01"`
			P02 string `json:"p02"`
			P03 string `json:"p03"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		return wrapperS0{ID: payload.ID, P00: payload.P00, P01: payload.P01, P02: payload.P02, P03: payload.P03}, nil
	}
	return nil, fmt.Errorf("unsupported epl insert-into wrapper event type %q", step.EventType)
}
