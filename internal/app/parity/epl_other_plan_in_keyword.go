package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const eplOtherPlanInKeywordID = "epl-other-plan-in-keyword"
const eplOtherPlanInKeywordJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherPlanInKeywordJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherPlanInKeywordQuery.java",
}

var eplOtherPlanInKeywordJavaRuntimeIDs = []string{
	"java-runtime-cf29d6b68db89aa0b8a8",
	"java-runtime-c982eaac78b6fd1b45ae",
	"java-runtime-c5daa1f63378ab985abd",
	"java-runtime-dd7da2447d43068347ff",
	"java-runtime-d1bd81d5833b3bb70b20",
	"java-runtime-b537100e9c81481cfe31",
	"java-runtime-8607ff3a12954cbb6373",
	"java-runtime-fb1e28fea28321f53bd1",
	"java-runtime-36887b719198c76fca9d",
}

var eplOtherPlanInKeywordJavaExecutions = []string{
	"EPLOtherNotIn",
	"EPLOtherMultiIdxMultipleInAndMultirow",
	"EPLOtherMultiIdxSubquery",
	"EPLOtherSingleIdxMultipleInAndMultirow",
	"EPLOtherSingleIdxSubquery",
	"EPLOtherSingleIdxConstants",
	"EPLOtherMultiIdxConstants",
	"EPLOtherQueryPlan3Stream",
	"EPLOtherQueryPlan2Stream",
}

const eplOtherPlanInKeywordSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherPlanInKeywordQuery.java"

const eplOtherPlanInKeywordDescription = "EPLOtherPlanInKeywordQuery ordinals 0-8: in-keyword and not-in query-plan executions over SupportBean_S0/SupportBean_S1/SupportBean_S2. not-in (ord 0) is deploy-only. multi-idx (ord 1) and single-idx (ord 3) replay the same send script against three surfaces each (unidirectional join, keepall named window via on-trigger, primary-key table via on-trigger) asserting joined s0.id/s1.id rows. multi-idx-subquery (ord 2) and single-idx-subquery (ord 4) select c0 plus a selectFrom collection c1 (null on no match) and finish with a coercion-absence deploy-only probe. single-idx-constants (ord 5) and multi-idx-constants (ord 6) join on constant in-expressions. plan-3stream (ord 7) and plan-2stream (ord 8) are deploy-only plan-shape cycles. Every observed statement carries the byte-exact INDEX_CALLBACK_HOOK @Hook annotation; the hook class is a compile-time no-op mirror because regression-lib is not on the oracle classpath."

var eplOtherPlanInKeywordJavaStaticIDs = []string{
	"java-61ef48fd70c1eb9fb712",
	"java-150700e404fe914eb1e9",
	"java-6f47e17a2b64ac117a22",
	"java-6124a7e0a85c0c5cad08",
	"java-a848701a26a2f89a8e81",
	"java-9a9e2f1ca46c53feffb0",
	"java-07fef47260a50e65bca4",
	"java-3f44a9a497b71647f01d",
	"java-2587b5e921c4bc0c5be0",
}

// The thirteen scenario cases in order; the multi-idx and single-idx
// executions each own the join, named-window and table variants of the same
// assertion, so several cases share one runtime ID.
var eplOtherPlanInKeywordCases = []string{
	"not-in",
	"multi-idx-join",
	"multi-idx-window",
	"multi-idx-table",
	"multi-idx-subquery",
	"single-idx-join",
	"single-idx-window",
	"single-idx-table",
	"single-idx-subquery",
	"single-idx-constants",
	"multi-idx-constants",
	"plan-3stream",
	"plan-2stream",
}

var eplOtherPlanInKeywordOrdinals = []int{0, 1, 1, 1, 2, 3, 3, 3, 4, 5, 6, 7, 8}

var eplOtherPlanInKeywordCaseRuntimeIDs = []string{
	"java-runtime-cf29d6b68db89aa0b8a8",
	"java-runtime-c982eaac78b6fd1b45ae",
	"java-runtime-c982eaac78b6fd1b45ae",
	"java-runtime-c982eaac78b6fd1b45ae",
	"java-runtime-c5daa1f63378ab985abd",
	"java-runtime-dd7da2447d43068347ff",
	"java-runtime-dd7da2447d43068347ff",
	"java-runtime-dd7da2447d43068347ff",
	"java-runtime-d1bd81d5833b3bb70b20",
	"java-runtime-b537100e9c81481cfe31",
	"java-runtime-8607ff3a12954cbb6373",
	"java-runtime-fb1e28fea28321f53bd1",
	"java-runtime-36887b719198c76fca9d",
}

var eplOtherPlanInKeywordCaseExecutions = []string{
	"EPLOtherNotIn",
	"EPLOtherMultiIdxMultipleInAndMultirow",
	"EPLOtherMultiIdxMultipleInAndMultirow",
	"EPLOtherMultiIdxMultipleInAndMultirow",
	"EPLOtherMultiIdxSubquery",
	"EPLOtherSingleIdxMultipleInAndMultirow",
	"EPLOtherSingleIdxMultipleInAndMultirow",
	"EPLOtherSingleIdxMultipleInAndMultirow",
	"EPLOtherSingleIdxSubquery",
	"EPLOtherSingleIdxConstants",
	"EPLOtherMultiIdxConstants",
	"EPLOtherQueryPlan3Stream",
	"EPLOtherQueryPlan2Stream",
}

const eplOtherPlanInKeywordHook = "@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')"

// Byte-exact EPL texts carried by the scenario deploy steps. The @Hook
// annotation is a Java-only query-plan probe; the Go runner maps the same
// statement to the equivalent fluent API.
const (
	eplOtherPlanInKeywordNotIn = eplOtherPlanInKeywordHook +
		"select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where p00 not in (p10, p11)"
	eplOtherPlanInKeywordMultiIdxJoin = "@name('s0') " + eplOtherPlanInKeywordHook +
		"select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where p00 in (p10, p11) and p01 in (p12, p13)"
	eplOtherPlanInKeywordCreateS1Window = "@public create window S1Window#keepall as SupportBean_S1"
	eplOtherPlanInKeywordInsertS1Window = "insert into S1Window select * from SupportBean_S1"
	eplOtherPlanInKeywordMultiIdxWindow = "@name('s0') " + eplOtherPlanInKeywordHook +
		"on SupportBean_S0 as s0 select * from S1Window as s1 where p00 in (p10, p11) and p01 in (p12, p13)"
	eplOtherPlanInKeywordCreateS1Table = "@public create table S1Table(id int primary key, p10 string primary key, p11 string primary key, p12 string primary key, p13 string primary key)"
	eplOtherPlanInKeywordInsertS1Table = "insert into S1Table select * from SupportBean_S1"
	eplOtherPlanInKeywordMultiIdxTable = "@name('s0') " + eplOtherPlanInKeywordHook +
		"on SupportBean_S0 as s0 select * from S1Table as s1 where p00 in (p10, p11) and p01 in (p12, p13)"
	eplOtherPlanInKeywordMultiIdxSubquery = "@name('s0') " + eplOtherPlanInKeywordHook +
		"select s0.id as c0,(select * from SupportBean_S1#keepall as s1   where s0.p00 in (s1.p10, SupportBean_S1.p11) and s0.p01 in (s1.p12, SupportBean_S1.p13)).selectFrom(a=>SupportBean_S1.id) as c1 from SupportBean_S0 as s0"
	eplOtherPlanInKeywordMultiIdxCoercion = eplOtherPlanInKeywordHook +
		"select *,(select * from SupportBean_S0#keepall as s0 where sb.longPrimitive in (id)) from SupportBean as sb"
	eplOtherPlanInKeywordSingleIdxJoin = "@name('s0') " + eplOtherPlanInKeywordHook +
		"select * from SupportBean_S0#keepall as s0, SupportBean_S1 as s1 unidirectional where p00 in (p10, p11) and p01 in (p12, p13)"
	eplOtherPlanInKeywordCreateS0Window  = "@public create window S0Window#keepall as SupportBean_S0"
	eplOtherPlanInKeywordInsertS0Window  = "insert into S0Window select * from SupportBean_S0"
	eplOtherPlanInKeywordSingleIdxWindow = "@name('s0') " + eplOtherPlanInKeywordHook +
		"on SupportBean_S1 as s1 select * from S0Window as s0 where p00 in (p10, p11) and p01 in (p12, p13)"
	eplOtherPlanInKeywordCreateS0Table  = "@public create table S0Table(id int primary key, p00 string primary key, p01 string primary key, p02 string primary key, p03 string primary key)"
	eplOtherPlanInKeywordInsertS0Table  = "insert into S0Table select * from SupportBean_S0"
	eplOtherPlanInKeywordSingleIdxTable = "@name('s0') " + eplOtherPlanInKeywordHook +
		"on SupportBean_S1 as s1 select * from S0Table as s0 where p00 in (p10, p11) and p01 in (p12, p13)"
	eplOtherPlanInKeywordSingleIdxSubquery = "@name('s0') " + eplOtherPlanInKeywordHook +
		"select s1.id as c0,(select * from SupportBean_S0#keepall as s0   where s0.p00 in (s1.p10, SupportBean_S1.p11) and s0.p01 in (s1.p12, SupportBean_S1.p13)).selectFrom(a=>SupportBean_S0.id) as c1  from SupportBean_S1 as s1"
	eplOtherPlanInKeywordSingleIdxCoercion = eplOtherPlanInKeywordHook +
		"select *,(select * from SupportBean#keepall as sb where sb.longPrimitive in (s0.id)) from SupportBean_S0 as s0"
	eplOtherPlanInKeywordSingleIdxConstants = "@name('s0') " + eplOtherPlanInKeywordHook +
		"select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where p10 in ('a', 'b')"
	eplOtherPlanInKeywordMultiIdxConstants = "@name('s0') " + eplOtherPlanInKeywordHook +
		"select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where 'a' in (p10, p11)"
)

// eplOtherPlanInKeyword3StreamEPLs are the four deploy-only 3-stream plan
// probes; each is deployed and undeployed without events.
var eplOtherPlanInKeyword3StreamEPLs = []string{
	eplOtherPlanInKeywordHook + "@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p00 in (p10, p11)",
	eplOtherPlanInKeywordHook + "@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p00 in (p10, p20)",
	eplOtherPlanInKeywordHook + "@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p10 in (p00, p01)",
	eplOtherPlanInKeywordHook + "@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p10 in (p00, p20)",
}

// eplOtherPlanInKeyword2StreamEPLs are the thirteen deploy-only 2-stream plan
// probes in scenario order.
var eplOtherPlanInKeyword2StreamEPLs = []string{
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall ",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 = p10",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 = p10 and p00 in (p11, p12, p13)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 in (p11, p12)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 = p11 or p00 = p12",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional full outer join SupportBean_S1#keepall where p00 in (p11, p12)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' in (p11, p12)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' = p11 or 'A' = p12",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' in ('B', p12)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' in ('B', 'C')",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p10 in (p00, p01)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p10 in ('A', p01)",
	eplOtherPlanInKeywordHook + "select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p10 in ('A', 'B')",
}

type eplOtherPlanInKeywordS0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

type eplOtherPlanInKeywordS1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
	P12 *string `esper:"p12"`
	P13 *string `esper:"p13"`
}

type eplOtherPlanInKeywordS2 struct {
	ID  int     `esper:"id"`
	P20 *string `esper:"p20"`
	P21 *string `esper:"p21"`
	P22 *string `esper:"p22"`
	P23 *string `esper:"p23"`
}

type eplOtherPlanInKeywordBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// eplOtherPlanInKeywordTableColumns renders a table row event as the
// positional array the Java oracle emits for Object[]-backed rows.
var eplOtherPlanInKeywordTableColumns = map[string][]string{
	"S1Table": {"id", "p10", "p11", "p12", "p13"},
	"S0Table": {"id", "p00", "p01", "p02", "p03"},
}

func runEplOtherPlanInKeywordScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherPlanInKeywordS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherPlanInKeywordS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherPlanInKeywordS2](env, "SupportBean_S2"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherPlanInKeywordBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: scenario.ID}
	currentCase := ""
	sequence := uint64(0)
	var deployments []*esper.Deployment
	record := func(operation, statement string, batch esper.ResultBatch) {
		sequence++
		entry := compat.TraceRecord{Case: currentCase, Operation: operation,
			Statement: statement, Sequence: sequence, Time: compat.FormatTraceTime(batch.Time)}
		entry.New = compat.NormalizeResults(batch.New)
		entry.Old = compat.NormalizeResults(batch.Old)
		eplOtherPlanInKeywordNormalize(currentCase, entry.New)
		eplOtherPlanInKeywordNormalize(currentCase, entry.Old)
		trace.Records = append(trace.Records, entry)
	}
	listener := func(_ context.Context, batch esper.ResultBatch) error {
		record("listener", "s0", batch)
		return nil
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			currentCase = step.Case
			sequence = 0
		case "deploy":
			if strings.HasPrefix(step.Epl, "create index ") {
				if err := eplOtherPlanInKeywordCreateIndex(engine, step.Epl); err != nil {
					return trace, err
				}
				continue
			}
			query, err := eplOtherPlanInKeywordQuery(env, engine, currentCase, step.Epl, step.Statement)
			if err != nil {
				return trace, err
			}
			plan, err := env.Build(query)
			if err != nil {
				return trace, fmt.Errorf("%s deploy %q: %w", eplOtherPlanInKeywordID, step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return trace, fmt.Errorf("%s deploy %q: %w", eplOtherPlanInKeywordID, step.Statement, err)
			}
			deployments = append(deployments, deployment)
			if step.Statement == "s0" && currentCase != "plan-3stream" {
				statements := deployment.Statements()
				if len(statements) != 1 {
					return trace, fmt.Errorf("%s deploy %q: expected one statement, got %d", eplOtherPlanInKeywordID, step.Statement, len(statements))
				}
				statements[0].Subscribe(listener)
			}
		case "deployed":
			sequence++
			trace.Records = append(trace.Records, compat.TraceRecord{Case: currentCase,
				Operation: "deployed", Statement: step.Statement, Sequence: sequence,
				Time: "1970-01-01T00:00:00Z"})
		case "send":
			if err := eplOtherPlanInKeywordSend(ctx, engine, step); err != nil {
				return trace, err
			}
		case "undeploy-all":
			for _, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, err
				}
			}
			deployments = nil
		}
	}
	return trace, nil
}

// eplOtherPlanInKeywordNormalize mirrors two Java trace-boundary renderings
// the Go protocol does not share: table rows render as positional arrays in
// declared column order (Object[] underlying), and a selectFrom subquery over
// zero rows renders null while non-empty collections render sorted by value.
func eplOtherPlanInKeywordNormalize(caseName string, records []compat.ResultRecord) {
	// Only the table-side alias renders positionally; the other alias is
	// the trigger bean event, which stays a nested row.
	var tableFieldName string
	var columns []string
	switch caseName {
	case "multi-idx-table":
		tableFieldName, columns = "s1", eplOtherPlanInKeywordTableColumns["S1Table"]
	case "single-idx-table":
		tableFieldName, columns = "s0", eplOtherPlanInKeywordTableColumns["S0Table"]
	}
	for index := range records {
		fields := records[index].Fields
		if columns != nil {
			if row, ok := fields[tableFieldName].(map[string]any); ok {
				if row["kind"] == "row" {
					if rowFields, ok := row["fields"].(map[string]any); ok {
						fields[tableFieldName] = eplOtherPlanInKeywordPositionalRow(rowFields, columns)
					}
				}
			}
			continue
		}
		if caseName == "multi-idx-subquery" || caseName == "single-idx-subquery" {
			if c1, ok := fields["c1"]; ok {
				fields["c1"] = eplOtherPlanInKeywordSelectFrom(c1)
			}
		}
	}
}

func eplOtherPlanInKeywordPositionalRow(fields map[string]any, columns []string) []any {
	row := make([]any, 0, len(columns))
	for _, column := range columns {
		value := fields[column]
		if value == nil {
			// A nil *string column marshals as bare null; the Java oracle
			// renders absent Object[] slots with the null marker.
			value = map[string]any{"state": "null"}
		}
		row = append(row, value)
	}
	return row
}

func eplOtherPlanInKeywordSelectFrom(value any) any {
	switch values := value.(type) {
	case []int:
		if len(values) == 0 {
			return map[string]any{"state": "null"}
		}
		sorted := append([]int(nil), values...)
		sort.Ints(sorted)
		return sorted
	case []any:
		if len(values) == 0 {
			return map[string]any{"state": "null"}
		}
		sorted := append([]any(nil), values...)
		sort.SliceStable(sorted, func(i, j int) bool {
			return fmt.Sprint(sorted[i]) < fmt.Sprint(sorted[j])
		})
		return sorted
	}
	return value
}

func eplOtherPlanInKeywordCreateIndex(engine *esper.Engine, epl string) error {
	// create index S1Idx1 on S1Table(p10): a late catalog operation on the
	// live table, matching the infra_nwtable_subq runners.
	rest := strings.TrimPrefix(epl, "create index ")
	open := strings.Index(rest, " on ")
	closeParen := strings.Index(rest, "(")
	if open < 0 || closeParen < 0 || !strings.HasSuffix(rest, ")") {
		return fmt.Errorf("%s unsupported create index %q", eplOtherPlanInKeywordID, epl)
	}
	indexName := strings.TrimSpace(rest[:open])
	target := strings.TrimSpace(rest[open+4 : closeParen])
	columns := strings.Split(strings.TrimSuffix(rest[closeParen+1:], ")"), ",")
	for index := range columns {
		columns[index] = strings.TrimSpace(columns[index])
	}
	table, ok := engine.Table(target)
	if !ok {
		return fmt.Errorf("%s table %q is missing", eplOtherPlanInKeywordID, target)
	}
	if err := table.CreateIndex(indexName, columns, esper.IndexHash, false); err != nil {
		return fmt.Errorf("%s create index %q: %w", eplOtherPlanInKeywordID, indexName, err)
	}
	return nil
}

func eplOtherPlanInKeywordSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var payload map[string]any
	if len(step.Payload) > 0 {
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s send payload: %w", eplOtherPlanInKeywordID, err)
		}
	}
	stringField := func(name string) *string {
		if v, ok := payload[name].(string); ok {
			return &v
		}
		return nil
	}
	intField := func(name string) int {
		if v, ok := payload[name].(float64); ok {
			return int(v)
		}
		return 0
	}
	switch step.EventType {
	case "SupportBean_S0":
		return engine.SendEvent(ctx, eplOtherPlanInKeywordS0{
			ID: intField("id"), P00: stringField("p00"), P01: stringField("p01"),
			P02: stringField("p02"), P03: stringField("p03"),
		})
	case "SupportBean_S1":
		return engine.SendEvent(ctx, eplOtherPlanInKeywordS1{
			ID: intField("id"), P10: stringField("p10"), P11: stringField("p11"),
			P12: stringField("p12"), P13: stringField("p13"),
		})
	case "SupportBean_S2":
		return engine.SendEvent(ctx, eplOtherPlanInKeywordS2{
			ID: intField("id"), P20: stringField("p20"), P21: stringField("p21"),
			P22: stringField("p22"), P23: stringField("p23"),
		})
	case "SupportBean":
		event := eplOtherPlanInKeywordBean{IntPrimitive: intField("intPrimitive")}
		if v, ok := payload["theString"].(string); ok {
			event.TheString = v
		}
		if v, ok := payload["longPrimitive"].(float64); ok {
			event.LongPrimitive = int64(v)
		}
		return engine.SendEvent(ctx, event)
	}
	return fmt.Errorf("%s unsupported event type %q", eplOtherPlanInKeywordID, step.EventType)
}

// eplOtherPlanInKeywordQuery maps each scenario deploy EPL to the equivalent
// Go fluent query. The Java @Hook query-plan probe has no Go counterpart; the
// observable contract is deployment success plus the listener rows.
func eplOtherPlanInKeywordQuery(env *esper.Environment, engine *esper.Engine, caseName, epl, statement string) (esper.Query, error) {
	s0 := esper.From[eplOtherPlanInKeywordS0](env, "SupportBean_S0")
	s1 := esper.From[eplOtherPlanInKeywordS1](env, "SupportBean_S1")
	s2 := esper.From[eplOtherPlanInKeywordS2](env, "SupportBean_S2")

	// in-predicate helpers: JoinField reads the joined tuple, Field the
	// trigger event, NamedWindowField/TableField the candidate row.
	s0Field := func(name string) esper.Expression[any] { return esper.Field[eplOtherPlanInKeywordS0, any](name) }
	s1Field := func(name string) esper.Expression[any] { return esper.Field[eplOtherPlanInKeywordS1, any](name) }
	joinField := func(source int, name string) esper.Expression[any] { return esper.JoinField[any](source, name) }
	windowField := func(name string) esper.Expression[any] { return esper.NamedWindowField[any](name) }
	tableField := func(name string) esper.Expression[any] { return esper.TableField[any](name) }
	literal := func(value string) esper.Expression[any] { return esper.Literal[any](value) }

	// multiIdxJoinPredicate is p00 in (p10, p11) and p01 in (p12, p13) over
	// the joined tuple (s0 fields at source 0, s1 fields at source 1).
	multiIdxJoinPredicate := esper.And(
		esper.In[any](joinField(0, "p00"), joinField(1, "p10"), joinField(1, "p11")),
		esper.In[any](joinField(0, "p01"), joinField(1, "p12"), joinField(1, "p13")),
	)
	multiIdxJoinSelect := []esper.JoinSelection{
		esper.SelectSourceEvent(0, "s0"),
		esper.SelectSourceEvent(1, "s1"),
	}
	// multiIdxTriggerPredicate is the same in-pair evaluated against the
	// trigger event (Field) and the on-select candidate row (target field).
	multiIdxTriggerPredicate := func(target func(string) esper.Expression[any]) esper.Expression[bool] {
		return esper.And(
			esper.In[any](s0Field("p00"), target("p10"), target("p11")),
			esper.In[any](s0Field("p01"), target("p12"), target("p13")),
		)
	}
	// singleIdxTriggerPredicate is p00 in (p10, p11) and p01 in (p12, p13):
	// the candidate row's p0x fields are tested against the S1 trigger's
	// p1x fields (the same direction as the join form, where p00/p01 are
	// the S0 source fields).
	singleIdxTriggerPredicate := func(target func(string) esper.Expression[any]) esper.Expression[bool] {
		return esper.And(
			esper.In[any](target("p00"), s1Field("p10"), s1Field("p11")),
			esper.In[any](target("p01"), s1Field("p12"), s1Field("p13")),
		)
	}
	// selectFromSubquery renders (select * ...).selectFrom(a=>id): the
	// WindowValues aggregate projects every matching id; SubqueryValue keeps
	// Esper's null-on-empty cardinality.
	selectFromSubquery := func(inner esper.RecordStream, predicate esper.Expression[bool]) esper.Expression[[]int] {
		return esper.SubqueryValue[[]int](inner,
			esper.WindowValues[int](esper.Field[any, int]("id")),
			predicate)
	}

	switch {
	case caseName == "not-in" && epl == eplOtherPlanInKeywordNotIn:
		return esper.JoinMany(
			esper.JoinSource(s0).Unidirectional(),
			esper.JoinSource(s1.Window(esper.KeepAll())),
		).Select(multiIdxJoinSelect...).Where(
			esper.Not(esper.In[any](joinField(0, "p00"), joinField(1, "p10"), joinField(1, "p11"))),
		).Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-join" && epl == eplOtherPlanInKeywordMultiIdxJoin:
		return esper.JoinMany(
			esper.JoinSource(s0).Unidirectional(),
			esper.JoinSource(s1.Window(esper.KeepAll())),
		).Select(multiIdxJoinSelect...).Where(multiIdxJoinPredicate).Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-window" && epl == eplOtherPlanInKeywordCreateS1Window:
		schema, ok := env.Schema("SupportBean_S1")
		if !ok {
			return esper.Query{}, fmt.Errorf("%s SupportBean_S1 schema is missing", eplOtherPlanInKeywordID)
		}
		if _, err := esper.CreateNamedWindow(env, "S1Window", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return esper.Query{}, err
		}
		if _, ok := engine.NamedWindow("S1Window"); !ok {
			return esper.Query{}, fmt.Errorf("%s S1Window was not materialized", eplOtherPlanInKeywordID)
		}
		return esper.FromNamedWindow(env, "S1Window").CreateNamedWindowQuery(esper.StatementName(statement)), nil

	case caseName == "multi-idx-window" && epl == eplOtherPlanInKeywordInsertS1Window:
		return esper.OnEvent(s1).InsertIntoNamedWindow("S1Window", esper.CopyMatchingFields()).
			Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-window" && epl == eplOtherPlanInKeywordMultiIdxWindow:
		return esper.OnEvent(s0).SelectFromNamedWindow("S1Window", multiIdxTriggerPredicate(windowField),
			esper.Alias("s0", esper.EventValue[esper.Event]()),
			esper.Alias("s1", esper.StreamWildcard()),
		).Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-table" && epl == eplOtherPlanInKeywordCreateS1Table:
		if _, err := esper.CreateTable(env, "S1Table", []esper.TableColumn{
			// Java declares all five columns as the primary key; Go
			// rejects null PK components, so only id is keyed and the
			// string columns stay plain nullable columns. The scenario
			// never inserts duplicate ids, so the narrower key is
			// unobservable.
			esper.PrimaryKeyColumn[int]("id"),
			esper.TableColumnOf[*string]("p10"),
			esper.TableColumnOf[*string]("p11"),
			esper.TableColumnOf[*string]("p12"),
			esper.TableColumnOf[*string]("p13"),
		}); err != nil {
			return esper.Query{}, err
		}
		if _, ok := engine.Table("S1Table"); !ok {
			return esper.Query{}, fmt.Errorf("%s S1Table was not materialized", eplOtherPlanInKeywordID)
		}
		return esper.FromTable(env, "S1Table").Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-table" && epl == eplOtherPlanInKeywordInsertS1Table:
		return esper.OnEvent(s1).InsertIntoTable("S1Table", esper.CopyMatchingFields()).
			Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-table" && epl == eplOtherPlanInKeywordMultiIdxTable:
		return esper.OnEvent(s0).SelectFromTableWhere("S1Table", multiIdxTriggerPredicate(tableField),
			esper.Alias("s0", esper.EventValue[esper.Event]()),
			esper.Alias("s1", esper.StreamWildcard()),
		).Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-subquery" && epl == eplOtherPlanInKeywordMultiIdxSubquery:
		// s0 is the outer stream, s1 the subquery source: s0.p00 in
		// (s1.p10, s1.p11) reads the outer value against inner candidates.
		inner := s1.Window(esper.KeepAll()).AsRecord()
		predicate := esper.And(
			esper.In[any](esper.OuterField[any]("p00"), esper.Field[any, any]("p10"), esper.Field[any, any]("p11")),
			esper.In[any](esper.OuterField[any]("p01"), esper.Field[any, any]("p12"), esper.Field[any, any]("p13")),
		)
		return esper.Select(s0,
			esper.Alias("c0", s0Field("id")),
			esper.Alias("c1", selectFromSubquery(inner, predicate)),
		).Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-subquery" && epl == eplOtherPlanInKeywordMultiIdxCoercion:
		// Deploy-only probe: Java asserts the coercion subquery plans a full
		// table scan. The Go projection keeps the subquery column; the
		// wildcard prefix is not observable because no events are sent.
		inner := s0.Window(esper.KeepAll()).AsRecord()
		return esper.Select(esper.From[eplOtherPlanInKeywordBean](env, "SupportBean"),
			esper.Alias("c1", esper.SubqueryValue[esper.Event](inner, esper.EventValue[esper.Event](),
				esper.In[any](esper.OuterField[any]("longPrimitive"), esper.Field[any, any]("id")))),
		).Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-join" && epl == eplOtherPlanInKeywordSingleIdxJoin:
		return esper.JoinMany(
			esper.JoinSource(s0.Window(esper.KeepAll())),
			esper.JoinSource(s1).Unidirectional(),
		).Select(multiIdxJoinSelect...).Where(multiIdxJoinPredicate).Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-window" && epl == eplOtherPlanInKeywordCreateS0Window:
		schema, ok := env.Schema("SupportBean_S0")
		if !ok {
			return esper.Query{}, fmt.Errorf("%s SupportBean_S0 schema is missing", eplOtherPlanInKeywordID)
		}
		if _, err := esper.CreateNamedWindow(env, "S0Window", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return esper.Query{}, err
		}
		if _, ok := engine.NamedWindow("S0Window"); !ok {
			return esper.Query{}, fmt.Errorf("%s S0Window was not materialized", eplOtherPlanInKeywordID)
		}
		return esper.FromNamedWindow(env, "S0Window").CreateNamedWindowQuery(esper.StatementName(statement)), nil

	case caseName == "single-idx-window" && epl == eplOtherPlanInKeywordInsertS0Window:
		return esper.OnEvent(s0).InsertIntoNamedWindow("S0Window", esper.CopyMatchingFields()).
			Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-window" && epl == eplOtherPlanInKeywordSingleIdxWindow:
		return esper.OnEvent(s1).SelectFromNamedWindow("S0Window", singleIdxTriggerPredicate(windowField),
			esper.Alias("s0", esper.StreamWildcard()),
			esper.Alias("s1", esper.EventValue[esper.Event]()),
		).Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-table" && epl == eplOtherPlanInKeywordCreateS0Table:
		if _, err := esper.CreateTable(env, "S0Table", []esper.TableColumn{
			esper.PrimaryKeyColumn[int]("id"),
			esper.TableColumnOf[*string]("p00"),
			esper.TableColumnOf[*string]("p01"),
			esper.TableColumnOf[*string]("p02"),
			esper.TableColumnOf[*string]("p03"),
		}); err != nil {
			return esper.Query{}, err
		}
		if _, ok := engine.Table("S0Table"); !ok {
			return esper.Query{}, fmt.Errorf("%s S0Table was not materialized", eplOtherPlanInKeywordID)
		}
		return esper.FromTable(env, "S0Table").Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-table" && epl == eplOtherPlanInKeywordInsertS0Table:
		return esper.OnEvent(s0).InsertIntoTable("S0Table", esper.CopyMatchingFields()).
			Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-table" && epl == eplOtherPlanInKeywordSingleIdxTable:
		return esper.OnEvent(s1).SelectFromTableWhere("S0Table", singleIdxTriggerPredicate(tableField),
			esper.Alias("s0", esper.StreamWildcard()),
			esper.Alias("s1", esper.EventValue[esper.Event]()),
		).Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-subquery" && epl == eplOtherPlanInKeywordSingleIdxSubquery:
		// s0 is the subquery source, s1 the outer stream: s0.p00 in
		// (s1.p10, s1.p11) reads the inner value against outer candidates.
		inner := s0.Window(esper.KeepAll()).AsRecord()
		predicate := esper.And(
			esper.In[any](esper.Field[any, any]("p00"), esper.OuterField[any]("p10"), esper.OuterField[any]("p11")),
			esper.In[any](esper.Field[any, any]("p01"), esper.OuterField[any]("p12"), esper.OuterField[any]("p13")),
		)
		return esper.Select(s1,
			esper.Alias("c0", s1Field("id")),
			esper.Alias("c1", selectFromSubquery(inner, predicate)),
		).Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-subquery" && epl == eplOtherPlanInKeywordSingleIdxCoercion:
		inner := esper.From[eplOtherPlanInKeywordBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
		return esper.Select(s0,
			esper.Alias("c1", esper.SubqueryValue[esper.Event](inner, esper.EventValue[esper.Event](),
				esper.In[any](esper.Field[any, any]("longPrimitive"), esper.OuterField[any]("id")))),
		).Query(esper.StatementName(statement)), nil

	case caseName == "single-idx-constants" && epl == eplOtherPlanInKeywordSingleIdxConstants:
		return esper.JoinMany(
			esper.JoinSource(s0).Unidirectional(),
			esper.JoinSource(s1.Window(esper.KeepAll())),
		).Select(multiIdxJoinSelect...).Where(
			esper.In[any](joinField(1, "p10"), literal("a"), literal("b")),
		).Query(esper.StatementName(statement)), nil

	case caseName == "multi-idx-constants" && epl == eplOtherPlanInKeywordMultiIdxConstants:
		return esper.JoinMany(
			esper.JoinSource(s0).Unidirectional(),
			esper.JoinSource(s1.Window(esper.KeepAll())),
		).Select(multiIdxJoinSelect...).Where(
			esper.In[any](literal("a"), joinField(1, "p10"), joinField(1, "p11")),
		).Query(esper.StatementName(statement)), nil

	case caseName == "plan-3stream":
		for index, candidate := range eplOtherPlanInKeyword3StreamEPLs {
			if epl != candidate {
				continue
			}
			var predicate esper.Expression[bool]
			switch index {
			case 0:
				predicate = esper.In[any](joinField(0, "p00"), joinField(1, "p10"), joinField(1, "p11"))
			case 1:
				predicate = esper.In[any](joinField(0, "p00"), joinField(1, "p10"), joinField(2, "p20"))
			case 2:
				predicate = esper.In[any](joinField(1, "p10"), joinField(0, "p00"), joinField(0, "p01"))
			default:
				predicate = esper.In[any](joinField(1, "p10"), joinField(0, "p00"), joinField(2, "p20"))
			}
			return esper.JoinMany(
				esper.JoinSource(s0).Unidirectional(),
				esper.JoinSource(s1.Window(esper.KeepAll())),
				esper.JoinSource(s2.Window(esper.KeepAll())),
			).Select(
				esper.SelectSourceEvent(0, "s0"),
				esper.SelectSourceEvent(1, "s1"),
				esper.SelectSourceEvent(2, "s2"),
			).Where(predicate).Query(esper.StatementName(statement)), nil
		}

	case caseName == "plan-2stream":
		for index, candidate := range eplOtherPlanInKeyword2StreamEPLs {
			if epl != candidate {
				continue
			}
			join := esper.JoinMany(
				esper.JoinSource(s0).Unidirectional(),
				esper.JoinSource(s1.Window(esper.KeepAll())),
			)
			if index == 5 {
				join = join.FullOuter()
			}
			selected := join.Select(multiIdxJoinSelect...)
			var predicate esper.Expression[bool]
			switch index {
			case 0:
				// no where clause
			case 1:
				predicate = esper.Equal[any](joinField(0, "p00"), joinField(1, "p10"))
			case 2:
				predicate = esper.And(
					esper.Equal[any](joinField(0, "p00"), joinField(1, "p10")),
					esper.In[any](joinField(0, "p00"), joinField(1, "p11"), joinField(1, "p12"), joinField(1, "p13")))
			case 3:
				predicate = esper.In[any](joinField(0, "p00"), joinField(1, "p11"), joinField(1, "p12"))
			case 4:
				predicate = esper.Or(
					esper.Equal[any](joinField(0, "p00"), joinField(1, "p11")),
					esper.Equal[any](joinField(0, "p00"), joinField(1, "p12")))
			case 5:
				predicate = esper.In[any](joinField(0, "p00"), joinField(1, "p11"), joinField(1, "p12"))
			case 6:
				predicate = esper.In[any](literal("A"), joinField(1, "p11"), joinField(1, "p12"))
			case 7:
				predicate = esper.Or(
					esper.Equal[any](literal("A"), joinField(1, "p11")),
					esper.Equal[any](literal("A"), joinField(1, "p12")))
			case 8:
				predicate = esper.In[any](literal("A"), literal("B"), joinField(1, "p12"))
			case 9:
				predicate = esper.In[any](literal("A"), literal("B"), literal("C"))
			case 10:
				predicate = esper.In[any](joinField(1, "p10"), joinField(0, "p00"), joinField(0, "p01"))
			case 11:
				predicate = esper.In[any](joinField(1, "p10"), literal("A"), joinField(0, "p01"))
			default:
				predicate = esper.In[any](joinField(1, "p10"), literal("A"), literal("B"))
			}
			if predicate != nil {
				selected = selected.Where(predicate)
			}
			return selected.Query(esper.StatementName(statement)), nil
		}
	}
	return esper.Query{}, fmt.Errorf("%s unsupported deploy %q in case %q", eplOtherPlanInKeywordID, epl, caseName)
}

// eplOtherPlanInKeywordCaseEPLs pins the first deploy EPL of each case, the
// value carried by the scenario cases[] metadata.
var eplOtherPlanInKeywordCaseEPLs = []string{
	eplOtherPlanInKeywordNotIn,
	eplOtherPlanInKeywordMultiIdxJoin,
	eplOtherPlanInKeywordMultiIdxWindow,
	eplOtherPlanInKeywordMultiIdxTable,
	eplOtherPlanInKeywordMultiIdxSubquery,
	eplOtherPlanInKeywordSingleIdxJoin,
	eplOtherPlanInKeywordSingleIdxWindow,
	eplOtherPlanInKeywordSingleIdxTable,
	eplOtherPlanInKeywordSingleIdxSubquery,
	eplOtherPlanInKeywordSingleIdxConstants,
	eplOtherPlanInKeywordMultiIdxConstants,
	eplOtherPlanInKeyword3StreamEPLs[0],
	eplOtherPlanInKeyword2StreamEPLs[0],
}

var eplOtherPlanInKeywordCaseObservations = []string{
	"deployed; p00 not in (p10, p11) compiles and deploys with no events sent",
	"listener; unidirectional join surface of tryAssertionMultiIdx emitting joined s0.id/s1.id rows",
	"listener; keepall named window S1Window surface of tryAssertionMultiIdx via on SupportBean_S0 trigger",
	"listener; primary-key table S1Table surface of tryAssertionMultiIdx via on SupportBean_S0 trigger",
	"listener; c0 carries s0.id and c1 carries the selectFrom collection of matching s1.id or null, then a coercion-absence deploy-only probe",
	"listener; unidirectional join surface of tryAssertionSingleIdx emitting joined s0.id/s1.id rows",
	"listener; keepall named window S0Window surface of tryAssertionSingleIdx via on SupportBean_S1 trigger",
	"listener; primary-key table S0Table surface of tryAssertionSingleIdx via on SupportBean_S1 trigger",
	"listener; c0 carries s1.id and c1 carries the selectFrom collection of matching s0.id or null, then a coercion-absence deploy-only probe",
	"listener; unidirectional join on p10 in ('a', 'b') emitting joined s0.id/s1.id rows",
	"listener; unidirectional join on 'a' in (p10, p11) emitting joined s0.id/s1.id rows",
	"deployed; four compile/deploy cycles of the three-stream in-keyword join with no events sent",
	"deployed; thirteen compile/deploy cycles of the two-stream in-keyword join variants with no events sent",
}

// loadEplOtherPlanInKeywordScenario decodes the scenario with the same strict
// contract as the other differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist.
func loadEplOtherPlanInKeywordScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherPlanInKeywordID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherPlanInKeywordID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherPlanInKeywordID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherPlanInKeywordID, err)
	}
	if err := requireEplOtherPlanInKeywordFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherPlanInKeywordID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherPlanInKeywordID ||
		metadata.Description != eplOtherPlanInKeywordDescription ||
		metadata.JavaCommit != eplOtherPlanInKeywordJavaCommit ||
		metadata.JavaSource != eplOtherPlanInKeywordSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplOtherPlanInKeywordID)
	}
	if err := validateEplOtherPlanInKeywordStringArray(root["javaRuntimes"], eplOtherPlanInKeywordJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherPlanInKeywordStringArray(root["javaNames"], eplOtherPlanInKeywordJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherPlanInKeywordStringArray(root["javaStaticIds"], eplOtherPlanInKeywordJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherPlanInKeywordStringArray(root["javaFlags"], []string{"INVALIDITY"}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplOtherPlanInKeywordCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplOtherPlanInKeywordID, len(eplOtherPlanInKeywordCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplOtherPlanInKeywordFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != eplOtherPlanInKeywordCases[index] ||
			definition.Ordinal != eplOtherPlanInKeywordOrdinals[index] ||
			definition.RuntimeID != eplOtherPlanInKeywordCaseRuntimeIDs[index] ||
			definition.ExecutionName != eplOtherPlanInKeywordCaseExecutions[index] ||
			definition.Observation != eplOtherPlanInKeywordCaseObservations[index] ||
			definition.EPL != eplOtherPlanInKeywordCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplOtherPlanInKeywordID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplOtherPlanInKeywordID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			// mode:"any" marks order-insensitive cases; absent otherwise.
			if _, hasMode := object["mode"]; hasMode {
				if err := requireEplOtherPlanInKeywordFields(object, "op", "case", "mode"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			} else if err := requireEplOtherPlanInKeywordFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplOtherPlanInKeywordFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireEplOtherPlanInKeywordFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplOtherPlanInKeywordFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if err := eplOtherPlanInKeywordValidatePayload(payload.EventType, payload.Payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplOtherPlanInKeywordFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherPlanInKeywordScenarioShape(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplOtherPlanInKeywordValidatePayload rejects unknown payload fields per
// event type so scenario drift cannot silently change the sent events.
func eplOtherPlanInKeywordValidatePayload(eventType string, raw json.RawMessage) error {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return err
	}
	var fields []string
	switch eventType {
	case "SupportBean_S0":
		fields = []string{"id", "p00", "p01", "p02", "p03"}
	case "SupportBean_S1":
		fields = []string{"id", "p10", "p11", "p12", "p13"}
	case "SupportBean_S2":
		fields = []string{"id", "p20", "p21", "p22", "p23"}
	case "SupportBean":
		fields = []string{"theString", "intPrimitive", "longPrimitive"}
	default:
		return fmt.Errorf("unsupported event type %q", eventType)
	}
	allowed := make(map[string]struct{}, len(fields))
	for _, name := range fields {
		allowed[name] = struct{}{}
	}
	for name := range object {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("payload for %s has unexpected field %q", eventType, name)
		}
	}
	return nil
}

func requireEplOtherPlanInKeywordFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateEplOtherPlanInKeywordStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

// eplOtherPlanInKeywordCaseModes pins the case-op mode: "any" marks
// order-insensitive listener cases, empty means exact order.
var eplOtherPlanInKeywordCaseModes = map[string]string{
	"not-in":               "",
	"multi-idx-join":       "any",
	"multi-idx-window":     "any",
	"multi-idx-table":      "any",
	"multi-idx-subquery":   "",
	"single-idx-join":      "any",
	"single-idx-window":    "any",
	"single-idx-table":     "any",
	"single-idx-subquery":  "",
	"single-idx-constants": "any",
	"multi-idx-constants":  "any",
	"plan-3stream":         "",
	"plan-2stream":         "",
}

// eplOtherPlanInKeywordCaseSteps pins the exact op sequence per case:
// deploy statements, send event types with canonical payloads, and the
// trailing undeploy-all. The strict loader rejects any drift.
var eplOtherPlanInKeywordCaseSteps = map[string][]string{
	"not-in": {
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where p00 not in (p10, p11)",
		"deployed:plan",
		"undeploy-all",
	},
	"multi-idx-join": {
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where p00 in (p10, p11) and p01 in (p12, p13)",
		"send:SupportBean_S1:{\"id\":101,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":2,\"p00\":\"b\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":3,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":102,\"p10\":\"a1\",\"p11\":\"a\",\"p12\":\"d1\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":10,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":11,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":12,\"p00\":\"a1\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":13,\"p00\":\"a\",\"p01\":\"d1\"}",
		"send:SupportBean_S1:{\"id\":103,\"p10\":\"a\",\"p11\":\"a2\",\"p12\":\"d\",\"p13\":\"d2\"}",
		"send:SupportBean_S0:{\"id\":20,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":21,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":22,\"p00\":\"a2\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":23,\"p00\":\"a\",\"p01\":\"d2\"}",
		"undeploy-all",
	},
	"multi-idx-window": {
		"deploy:create:@public create window S1Window#keepall as SupportBean_S1",
		"deploy:insert:insert into S1Window select * from SupportBean_S1",
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')on SupportBean_S0 as s0 select * from S1Window as s1 where p00 in (p10, p11) and p01 in (p12, p13)",
		"send:SupportBean_S1:{\"id\":101,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":2,\"p00\":\"b\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":3,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":102,\"p10\":\"a1\",\"p11\":\"a\",\"p12\":\"d1\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":10,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":11,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":12,\"p00\":\"a1\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":13,\"p00\":\"a\",\"p01\":\"d1\"}",
		"send:SupportBean_S1:{\"id\":103,\"p10\":\"a\",\"p11\":\"a2\",\"p12\":\"d\",\"p13\":\"d2\"}",
		"send:SupportBean_S0:{\"id\":20,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":21,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":22,\"p00\":\"a2\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":23,\"p00\":\"a\",\"p01\":\"d2\"}",
		"undeploy-all",
	},
	"multi-idx-table": {
		"deploy:create:@public create table S1Table(id int primary key, p10 string primary key, p11 string primary key, p12 string primary key, p13 string primary key)",
		"deploy:insert:insert into S1Table select * from SupportBean_S1",
		"deploy:idx1:create index S1Idx1 on S1Table(p10)",
		"deploy:idx2:create index S1Idx2 on S1Table(p11)",
		"deploy:idx3:create index S1Idx3 on S1Table(p12)",
		"deploy:idx4:create index S1Idx4 on S1Table(p13)",
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')on SupportBean_S0 as s0 select * from S1Table as s1 where p00 in (p10, p11) and p01 in (p12, p13)",
		"send:SupportBean_S1:{\"id\":101,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":2,\"p00\":\"b\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":3,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":102,\"p10\":\"a1\",\"p11\":\"a\",\"p12\":\"d1\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":10,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":11,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":12,\"p00\":\"a1\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":13,\"p00\":\"a\",\"p01\":\"d1\"}",
		"send:SupportBean_S1:{\"id\":103,\"p10\":\"a\",\"p11\":\"a2\",\"p12\":\"d\",\"p13\":\"d2\"}",
		"send:SupportBean_S0:{\"id\":20,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":21,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":22,\"p00\":\"a2\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":23,\"p00\":\"a\",\"p01\":\"d2\"}",
		"undeploy-all",
	},
	"multi-idx-subquery": {
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select s0.id as c0,(select * from SupportBean_S1#keepall as s1   where s0.p00 in (s1.p10, SupportBean_S1.p11) and s0.p01 in (s1.p12, SupportBean_S1.p13)).selectFrom(a=>SupportBean_S1.id) as c1 from SupportBean_S0 as s0",
		"send:SupportBean_S1:{\"id\":101,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":2,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":3,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":4,\"p00\":\"b\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":5,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":102,\"p10\":\"a1\",\"p11\":\"a\",\"p12\":\"d1\",\"p13\":\"d\"}",
		"send:SupportBean_S0:{\"id\":10,\"p00\":\"a\",\"p01\":\"x\"}",
		"send:SupportBean_S0:{\"id\":11,\"p00\":\"x\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":12,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":13,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":14,\"p00\":\"a1\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":15,\"p00\":\"a\",\"p01\":\"d1\"}",
		"send:SupportBean_S1:{\"id\":103,\"p10\":\"a\",\"p11\":\"a2\",\"p12\":\"d\",\"p13\":\"d2\"}",
		"send:SupportBean_S0:{\"id\":20,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S0:{\"id\":21,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":22,\"p00\":\"a2\",\"p01\":\"d\"}",
		"send:SupportBean_S0:{\"id\":23,\"p00\":\"a\",\"p01\":\"d2\"}",
		"undeploy-all",
		"deploy:coercion:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select *,(select * from SupportBean_S0#keepall as s0 where sb.longPrimitive in (id)) from SupportBean as sb",
		"deployed:coercion",
		"undeploy-all",
	},
	"single-idx-join": {
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0#keepall as s0, SupportBean_S1 as s1 unidirectional where p00 in (p10, p11) and p01 in (p12, p13)",
		"send:SupportBean_S0:{\"id\":100,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":1,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":2,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S0:{\"id\":101,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":10,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":11,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":12,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"x\"}",
		"send:SupportBean_S0:{\"id\":102,\"p00\":\"b\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c1\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":20,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":21,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":22,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":23,\"p10\":\"b\",\"p11\":\"x\",\"p12\":\"x\",\"p13\":\"c\"}",
		"undeploy-all",
	},
	"single-idx-window": {
		"deploy:create:@public create window S0Window#keepall as SupportBean_S0",
		"deploy:insert:insert into S0Window select * from SupportBean_S0",
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')on SupportBean_S1 as s1 select * from S0Window as s0 where p00 in (p10, p11) and p01 in (p12, p13)",
		"send:SupportBean_S0:{\"id\":100,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":1,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":2,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S0:{\"id\":101,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":10,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":11,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":12,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"x\"}",
		"send:SupportBean_S0:{\"id\":102,\"p00\":\"b\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c1\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":20,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":21,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":22,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":23,\"p10\":\"b\",\"p11\":\"x\",\"p12\":\"x\",\"p13\":\"c\"}",
		"undeploy-all",
	},
	"single-idx-table": {
		"deploy:create:@public create table S0Table(id int primary key, p00 string primary key, p01 string primary key, p02 string primary key, p03 string primary key)",
		"deploy:insert:insert into S0Table select * from SupportBean_S0",
		"deploy:idx1:create index S0Idx1 on S0Table(p00)",
		"deploy:idx2:create index S0Idx2 on S0Table(p01)",
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')on SupportBean_S1 as s1 select * from S0Table as s0 where p00 in (p10, p11) and p01 in (p12, p13)",
		"send:SupportBean_S0:{\"id\":100,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":1,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":2,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S0:{\"id\":101,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":10,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":11,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":12,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"x\"}",
		"send:SupportBean_S0:{\"id\":102,\"p00\":\"b\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c1\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":0,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":20,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":21,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":22,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":23,\"p10\":\"b\",\"p11\":\"x\",\"p12\":\"x\",\"p13\":\"c\"}",
		"undeploy-all",
	},
	"single-idx-subquery": {
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select s1.id as c0,(select * from SupportBean_S0#keepall as s0   where s0.p00 in (s1.p10, SupportBean_S1.p11) and s0.p01 in (s1.p12, SupportBean_S1.p13)).selectFrom(a=>SupportBean_S0.id) as c1  from SupportBean_S1 as s1",
		"send:SupportBean_S0:{\"id\":100,\"p00\":\"a\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":1,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":2,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":3,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":4,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S0:{\"id\":101,\"p00\":\"a\",\"p01\":\"d\"}",
		"send:SupportBean_S1:{\"id\":10,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":11,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":12,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":13,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":14,\"p10\":\"x\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"x\"}",
		"send:SupportBean_S0:{\"id\":102,\"p00\":\"b\",\"p01\":\"c\"}",
		"send:SupportBean_S1:{\"id\":20,\"p10\":\"a1\",\"p11\":\"b\",\"p12\":\"c1\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":21,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"x\",\"p13\":\"c1\"}",
		"send:SupportBean_S1:{\"id\":22,\"p10\":\"a\",\"p11\":\"b\",\"p12\":\"c\",\"p13\":\"d\"}",
		"send:SupportBean_S1:{\"id\":23,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"x\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":24,\"p10\":\"b\",\"p11\":\"a\",\"p12\":\"d\",\"p13\":\"c\"}",
		"send:SupportBean_S1:{\"id\":25,\"p10\":\"b\",\"p11\":\"x\",\"p12\":\"x\",\"p13\":\"c\"}",
		"undeploy-all",
		"deploy:coercion:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select *,(select * from SupportBean#keepall as sb where sb.longPrimitive in (s0.id)) from SupportBean_S0 as s0",
		"deployed:coercion",
		"undeploy-all",
	},
	"single-idx-constants": {
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where p10 in ('a', 'b')",
		"send:SupportBean_S1:{\"id\":100,\"p10\":\"x\"}",
		"send:SupportBean_S1:{\"id\":101,\"p10\":\"a\"}",
		"send:SupportBean_S0:{\"id\":1}",
		"send:SupportBean_S1:{\"id\":102,\"p10\":\"b\"}",
		"send:SupportBean_S0:{\"id\":2}",
		"undeploy-all",
	},
	"multi-idx-constants": {
		"deploy:s0:@name('s0') @Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall as s1 where 'a' in (p10, p11)",
		"send:SupportBean_S1:{\"id\":100,\"p10\":\"x\",\"p11\":\"y\"}",
		"send:SupportBean_S1:{\"id\":101,\"p10\":\"x\",\"p11\":\"a\"}",
		"send:SupportBean_S0:{\"id\":1}",
		"send:SupportBean_S1:{\"id\":102,\"p10\":\"b\",\"p11\":\"a\"}",
		"send:SupportBean_S0:{\"id\":2}",
		"undeploy-all",
	},
	"plan-3stream": {
		"deploy:s0:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p00 in (p10, p11)",
		"deployed:s0",
		"undeploy-all",
		"deploy:s0:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p00 in (p10, p20)",
		"deployed:s0",
		"undeploy-all",
		"deploy:s0:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p10 in (p00, p01)",
		"deployed:s0",
		"undeploy-all",
		"deploy:s0:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')@name('s0') select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall, SupportBean_S2#keepall  where p10 in (p00, p20)",
		"deployed:s0",
		"undeploy-all",
	},
	"plan-2stream": {
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall ",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 = p10",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 = p10 and p00 in (p11, p12, p13)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 in (p11, p12)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p00 = p11 or p00 = p12",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional full outer join SupportBean_S1#keepall where p00 in (p11, p12)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' in (p11, p12)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' = p11 or 'A' = p12",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' in ('B', p12)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where 'A' in ('B', 'C')",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p10 in (p00, p01)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p10 in ('A', p01)",
		"deployed:plan",
		"undeploy-all",
		"deploy:plan:@Hook(type=com.espertech.esper.common.client.annotation.HookType.INTERNAL_QUERY_PLAN,hook='com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook')select * from SupportBean_S0 as s0 unidirectional, SupportBean_S1#keepall where p10 in ('A', 'B')",
		"deployed:plan",
		"undeploy-all",
	},
}

// validateEplOtherPlanInKeywordScenarioShape pins the complete step sequence
// per case: deploy statements with byte-exact EPL, deployed markers, send
// event types with canonical payloads, and undeploy-all terminators.
func validateEplOtherPlanInKeywordScenarioShape(scenario compat.Scenario) error {
	offset := 0
	for _, caseName := range eplOtherPlanInKeywordCases {
		want, ok := eplOtherPlanInKeywordCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", eplOtherPlanInKeywordID, caseName)
		}
		if offset+1+len(want) > len(scenario.Steps) {
			return fmt.Errorf("%s case %q is truncated", eplOtherPlanInKeywordID, caseName)
		}
		head := scenario.Steps[offset]
		if head.Op != "case" || head.Case != caseName {
			return fmt.Errorf("%s step %d must open case %q", eplOtherPlanInKeywordID, offset, caseName)
		}
		if head.Mode != eplOtherPlanInKeywordCaseModes[caseName] {
			return fmt.Errorf("%s case %q mode is not pinned", eplOtherPlanInKeywordID, caseName)
		}
		offset++
		for index, pinned := range want {
			step := scenario.Steps[offset+index]
			actual, err := eplOtherPlanInKeywordStepKey(step)
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", eplOtherPlanInKeywordID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", eplOtherPlanInKeywordID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplOtherPlanInKeywordID)
	}
	return nil
}

func eplOtherPlanInKeywordStepKey(step compat.Step) (string, error) {
	switch step.Op {
	case "deploy":
		return "deploy:" + step.Statement + ":" + step.Epl, nil
	case "deployed":
		return "deployed:" + step.Statement, nil
	case "send":
		var payload map[string]any
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		return "send:" + step.EventType + ":" + string(canonical), nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", step.Op)
}
