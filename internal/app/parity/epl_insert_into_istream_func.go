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

// epl_insert_into_istream_func.go replays the EPLInsertIntoIRStreamFunc
// execution (ord 0) against the pinned Java oracle:
//
//   - lastevent-irstream: '@name('i0') @public insert irstream into
//     MyStream select irstream theString as c0, istream() as c1 from
//     SupportBean#lastevent' + '@name('s0') select * from MyStream'.
//     'insert irstream into' routes the remove stream as INSERTS, so the
//     MyStream consumer sees both routed rows flattened as newData while
//     i0's own listener sees proper IR pairs. istream() returns true on
//     insert rows and false on routed-remove rows.
//   - join-irstream: '@name('s0') select irstream theString as c0, id as
//     c1, istream() as c2 from SupportBean#lastevent,
//     SupportBean_S0#lastevent' — a plain join select whose listener sees
//     new{E2,10,true} old{E1,10,false} when the SupportBean side replaces.
//
// Approved differences (observably identical to the Java EPL):
//   - Esper auto-creates MyStream; Go pre-registers it as a map schema.
//   - The SODA leg 'select istream() from SupportBean' asserts only the
//     compile-time property type (Boolean) — no listener data, so the
//     trace carries no record for it.
//   - undeployAll between legs is modelled as separate cases with fresh
//     runtimes.

const eplInsertIntoIStreamFuncJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoIStreamFuncJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoIRStreamFunc.java",
}

var eplInsertIntoIStreamFuncJavaRuntimeIDs = []string{
	"java-runtime-033d9d0dabb864fd8179",
}

var eplInsertIntoIStreamFuncJavaExecutions = []string{
	"EPLInsertIntoIRStreamFunc",
}

var eplInsertIntoIStreamFuncCases = []string{
	"lastevent-irstream",
	"join-irstream",
}

var eplInsertIntoIStreamFuncCaseRuntimeIDs = map[string]string{
	"lastevent-irstream": "java-runtime-033d9d0dabb864fd8179",
	"join-irstream":      "java-runtime-033d9d0dabb864fd8179",
}

// istreamFuncBean mirrors SupportBean's asserted fields.
type istreamFuncBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// istreamFuncS0 mirrors SupportBean_S0's asserted fields.
type istreamFuncS0 struct {
	ID int `esper:"id"`
}

// runEplInsertIntoIStreamFuncScenario replays the EPLInsertIntoIRStreamFunc
// execution: 'insert irstream into' removes-as-inserts routing plus the
// istream() function, over a #lastevent producer and a two-stream join.
func runEplInsertIntoIStreamFuncScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplInsertIntoIStreamFuncCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplInsertIntoIStreamFuncCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl insert-into istream-func case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl insert-into istream-func scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplInsertIntoIStreamFuncCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plans, err := buildEplInsertIntoIStreamFuncCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplInsertIntoIStreamFuncCaseRuntimeIDs[caseName]),
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
			if name != "i0" && name != "s0" {
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
			payload, err := decodeEplInsertIntoIStreamFuncPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported epl insert-into istream-func step op %q", step.Op)
		}
	}
	return trace, nil
}

// buildEplInsertIntoIStreamFuncCase registers the case's schemas and
// targets and returns the deployment plans in pinned order (producer
// insert-into before consumer, matching the Java deployment order).
func buildEplInsertIntoIStreamFuncCase(env *esper.Environment, caseName string) ([]esper.Plan, error) {
	switch caseName {
	case "lastevent-irstream":
		if _, err := esper.RegisterStruct[istreamFuncBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterMap(env, "MyStream", []esper.FieldSpec{
			esper.FieldDef("c0", reflect.TypeOf("")),
			esper.FieldDef("c1", reflect.TypeOf(false)),
		}); err != nil {
			return nil, err
		}
		// @name('i0') @public insert irstream into MyStream select irstream
		// theString as c0, istream() as c1 from SupportBean#lastevent —
		// 'insert irstream into' routes the remove stream as inserts
		// (WithOldStream selects irstream output, WithIRStreamRoute selects
		// the irstream route).
		i0Plan, err := env.Build(
			esper.From[istreamFuncBean](env, "SupportBean").
				Window(esper.LastEvent()).
				AsRecord().Select(
				esper.Alias("c0", esper.Field[istreamFuncBean, string]("theString")),
				esper.Alias("c1", esper.IStream()),
			).InsertInto("MyStream", esper.StatementName("i0"), esper.WithOldStream(), esper.WithIRStreamRoute()))
		if err != nil {
			return nil, err
		}
		// @name('s0') select * from MyStream — the consumer sees both
		// routed rows flattened as newData.
		s0Plan, err := env.Build(
			esper.FromAny(env, "MyStream").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{i0Plan, s0Plan}, nil

	case "join-irstream":
		if _, err := esper.RegisterStruct[istreamFuncBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[istreamFuncS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		// @name('s0') select irstream theString as c0, id as c1, istream()
		// as c2 from SupportBean#lastevent, SupportBean_S0#lastevent — a
		// plain join select whose listener sees the IR pair.
		s0Plan, err := env.Build(
			esper.Join(
				esper.From[istreamFuncBean](env, "SupportBean").Window(esper.LastEvent()),
				esper.From[istreamFuncS0](env, "SupportBean_S0").Window(esper.LastEvent()),
			).Select(
				esper.SelectFrom(0, "c0", esper.JoinField[string](0, "theString")),
				esper.SelectFrom(1, "c1", esper.JoinField[int](1, "id")),
				esper.SelectFrom(0, "c2", esper.IStream()),
			).Query(esper.StatementName("s0"), esper.WithOldStream()))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{s0Plan}, nil
	}
	return nil, fmt.Errorf("unknown epl insert-into istream-func case %q", caseName)
}

func decodeEplInsertIntoIStreamFuncPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value istreamFuncBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value istreamFuncS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported epl insert-into istream-func event type %q", step.EventType)
	}
}
