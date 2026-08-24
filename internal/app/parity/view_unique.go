package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

var viewUniqueJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewUnique.java",
}

type vuMarket struct {
	Symbol string  `esper:"symbol"`
	Feed   *string `esper:"feed"`
	Price  float64 `esper:"price"`
}

type vuBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
	IntBoxed     *int32 `esper:"intBoxed"`
}

var viewUniqueJavaRuntimeIDs = []string{
	"java-runtime-0f8822c71264f4aaf788",
	"java-runtime-2461852109912927f5ac",
	"java-runtime-f354e2a6054730d80312",
	"java-runtime-ba0287f83e55f5fd2454",
	"java-runtime-5956f7d158bda2623d70",
}

var viewUniqueJavaExecutions = []string{
	"ViewLastUniqueSceneOne",
	"ViewLastUniqueSceneTwo",
	"ViewLastUniqueWithAnnotationPrefix",
	"ViewUniqueExpressionParameter",
	"ViewUniqueTwoWindows",
}

func runViewUniqueScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, cn := range []string{
		"scene-one", "scene-two", "annotation-prefix",
		"expression-parameter", "two-windows",
	} {
		if !scenarioHasCase(scenario, cn) {
			continue
		}
		ct, err := runViewUniqueCase(ctx, scenario, cn)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", cn, err)
		}
		trace.Records = append(trace.Records, ct.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("no supported cases")
	}
	return trace, nil
}

// vuQuery is one built statement within a case: the query plus whether its
// deployment is deferred until a scenario deploy step (TwoWindows s1).
type vuQuery struct {
	name     string
	query    esper.Query
	deferred bool
}

func runViewUniqueCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[vuMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[vuBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	al := func(e esper.Expr, s string) esper.Selection { return esper.Alias(s, e) }

	var queries []vuQuery
	switch caseName {
	case "scene-one":
		sym := esper.Field[vuMarket, string]("symbol")
		prc := esper.Field[vuMarket, float64]("price")
		queries = []vuQuery{{
			name: "s0",
			query: esper.Select(
				esper.From[vuMarket](env, "SupportMarketDataBean").Window(esper.Unique(sym)),
				al(sym, "symbol"),
				al(prc, "price"),
			).Query(esper.StatementName("s0"), esper.WithOldStream()),
		}}

	case "scene-two":
		sym := esper.Field[vuMarket, string]("symbol")
		feed := esper.Field[vuMarket, string]("feed")
		prc := esper.Field[vuMarket, float64]("price")
		queries = []vuQuery{{
			name: "s0",
			query: esper.Select(
				esper.From[vuMarket](env, "SupportMarketDataBean").Window(esper.UniqueBy(sym, feed)),
				al(sym, "symbol"),
				al(feed, "feed"),
				al(prc, "price"),
			).Query(esper.StatementName("s0"), esper.WithOldStream()),
		}}

	case "annotation-prefix":
		ts := esper.Field[vuBean, string]("theString")
		ip := esper.Field[vuBean, int32]("intPrimitive")
		queries = []vuQuery{{
			name: "s0",
			query: esper.Select(
				esper.From[vuBean](env, "SupportBean").Window(esper.Unique(ts)),
				al(ts, "c0"),
				al(ip, "c1"),
			).Query(esper.StatementName("s0"), esper.WithOldStream()),
		}}

	case "expression-parameter":
		ip := esper.Field[vuBean, int32]("intPrimitive")
		absExpr := esper.Func1[int32, int32]("absInt",
			func(v int32) int32 {
				if v < 0 {
					return -v
				}
				return v
			}, ip)
		queries = []vuQuery{{
			name: "s0",
			query: esper.Select(
				esper.From[vuBean](env, "SupportBean").Window(esper.Unique(absExpr)),
				al(esper.Field[vuBean, string]("theString"), "theString"),
				al(ip, "intPrimitive"),
				al(esper.Field[vuBean, *int32]("intBoxed"), "intBoxed"),
			).Query(esper.StatementName("s0")),
		}}

	case "two-windows":
		ib := esper.Field[vuBean, *int32]("intBoxed")
		ts := esper.Field[vuBean, string]("theString")
		queries = []vuQuery{
			{
				name: "s0",
				query: esper.Select(
					esper.From[vuBean](env, "SupportBean").Window(esper.Unique(ib)),
					al(ts, "theString"),
					al(ib, "intBoxed"),
					al(esper.Field[vuBean, int32]("intPrimitive"), "intPrimitive"),
				).Query(esper.StatementName("s0"), esper.WithOldStream()),
			},
			{
				name:     "s1",
				deferred: true,
				query: esper.Select(
					esper.From[vuBean](env, "SupportBean").Window(esper.Unique(ib)),
					al(ts, "theString"),
					al(ib, "intBoxed"),
					al(esper.Field[vuBean, int32]("intPrimitive"), "intPrimitive"),
				).Query(esper.StatementName("s1"), esper.WithOldStream()),
			},
		}

	default:
		return compat.Trace{}, fmt.Errorf("unsupported view-unique case %q", caseName)
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := &compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seqByStmt := make(map[string]uint64)
	statements := make(map[string]*esper.Statement)

	deploy := func(name string) error {
		var q *vuQuery
		for i := range queries {
			if queries[i].name == name {
				q = &queries[i]
				break
			}
		}
		if q == nil {
			return fmt.Errorf("no plan named %q", name)
		}
		plan, err := env.Build(q.query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		for _, st := range deployment.Statements() {
			stmt := st
			statements[stmt.Name()] = stmt
			if stmt.Name() != "s0" && stmt.Name() != "s1" {
				continue
			}
			if _, subErr := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				hasNew := len(batch.New) > 0
				hasOld := len(batch.Old) > 0
				if !hasNew && !hasOld {
					return nil
				}
				name := stmt.Name()
				stmtSeq := seqByStmt[name]
				stmtSeq++
				seqByStmt[name] = stmtSeq
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: name,
					Time:      batch.Time.UTC().Format(time.RFC3339),
					Sequence:  stmtSeq,
				}
				record.New = compat.NormalizeResults(batch.New)
				record.Old = compat.NormalizeResults(batch.Old)
				trace.Records = append(trace.Records, record)
				return nil
			}); subErr != nil {
				return subErr
			}
		}
		return nil
	}

	for _, q := range queries {
		if q.deferred {
			continue
		}
		if err := deploy(q.name); err != nil {
			return *trace, err
		}
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return *trace, err
			}
			if sendErr := engine.SendRecord(ctx, step.EventType, payload); sendErr != nil {
				return *trace, sendErr
			}
		case "snapshot":
			st, ok := statements[step.Statement]
			if !ok {
				continue
			}
			result, snapErr := st.Snapshot(ctx)
			if snapErr != nil {
				return *trace, snapErr
			}
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
			}
			record.New = compat.NormalizeResults(result.Batch.New)
			if record.New == nil {
				record.New = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, record)
		case "deploy":
			if err := deploy(step.Statement); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return *trace, nil
}
