package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// infra_table_subquery.go replays InfraTableSubquery ordinals 0-3 against the
// pinned Java oracle: correlated scalar subqueries and subquery-in-filter
// against tables.
//
// subquery-keyed (ord 0, InfraTableSubqueryAgainstKeyed) deploys keyed
// varagg(key string primary key, total sum(int)) plus a grouped into-table
// sum feed and a correlated scalar subquery `(select total from varagg where
// key = s0.p00)` over SupportBean_S0; the listener pins value=null for the
// missing G1 row and 200 for G2, then after milestone(0) and
// SupportBean("G1",100) pins 100/200.
//
// subquery-unkeyed (ord 1, InfraTableSubqueryAgainstUnkeyed) deploys the
// unkeyed InfraOne(string string, intPrimitive int), the correlated subquery
// statement BEFORE the insert-into feed, then sends SupportBean("E1",10),
// milestone(0) and S0(0,"E1") pinning c0=10 through a full-scan subquery.
//
// subquery-secondary-index (ord 2, InfraTableSubquerySecondaryIndex) deploys
// composite-key MyTable(k0,k1 primary keys, p2, value), a secondary index on
// p2 before any rows, an on-SupportBean_S0 merge that inserts/updates p2 and
// value, and a correlated subquery `where sb.theString = tbl.p2`; the merge
// update of the indexed column moves the row from P2_1 to P2_2 across
// milestones.
//
// subquery-in-filter (ord 3, InfraTableSubqueryInFilter — defined first in
// the Java file but ordinal 3 in executions()) deploys one three-statement
// module: module-private MyTable(tablecol primary key), an insert-into feed
// from SupportBean_S0.p00, and `select * from SupportBean(theString=(select
// tablecol from MyTable).orderBy().firstOf())` — an uncorrelated enum
// orderBy().firstOf() subquery inside the stream filter. Filtered-out sends
// produce no listener record; matching sends render the full SupportBean row.
//
// Approved differences (observably identical to the Java EPL):
//   - `create table` and `create index` map to env-level catalog calls; the
//     deploy labels carry the pinned EPL while deployed markers pin the step
//     labels (the infra-table-faf-execute-query precedent).
//   - Ord 3's single Java module deploys as one step labelled "module"; the
//     module-private create table maps to an env-level registration and the
//     insert/select deploy as plans under the same label so the deployed
//     marker and undeploy-all bookkeeping match.
//   - Java milestone checkpoints are harness no-ops and carry no steps.
//   - Java's assertListenerInvokedFlag(false) for filtered-out sends is the
//     absent listener record; assertPropsNew/assertEventNew are the listener
//     records themselves.

// infraTableSubqueryBean mirrors the full SupportBean schema: ord 3's
// `select *` listener rows render every property. Boxed, decimal, and enum
// columns are pointers so a plain send decodes to Java's nulls, and
// charPrimitive mirrors the Java char default via the decode.
type infraTableSubqueryBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// infraTableSubqueryS0 mirrors SupportBean_S0's asserted fields (id, p00,
// p01, p02 for the ord-2 merge keys and insert columns).
type infraTableSubqueryS0 struct {
	ID    int    `esper:"id"`
	P00   string `esper:"p00"`
	P01   string `esper:"p01"`
	P02   string `esper:"p02"`
	P03   string `esper:"p03"`
	Value int    `esper:"value"`
}

const (
	infraTableSubqueryID          = "infra-table-subquery"
	infraTableSubqueryJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableSubqueryJavaSource  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSubquery.java"
	infraTableSubqueryDescription = "InfraTableSubquery ordinals 0-3: InfraTableSubqueryAgainstKeyed deploys keyed varagg fed by a grouped into-table sum and pins the correlated scalar subquery value null/200 then 100/200 across milestone(0); InfraTableSubqueryAgainstUnkeyed deploys the unkeyed InfraOne with the subquery statement before the insert-into feed and pins c0=10; InfraTableSubquerySecondaryIndex deploys composite-key MyTable with a secondary index on p2 and an on-merge that moves the indexed column from P2_1 to P2_2, pinning c0=10 then null/11; InfraTableSubqueryInFilter deploys one module whose select * filters SupportBean on theString=(select tablecol from MyTable).orderBy().firstOf(), emitting listener records only for matching sends (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSubquery.java)."

	// Verbatim transcriptions of InfraTableSubquery lines 78-83 (ord 0);
	// 102-104 (ord 1); 121-133 (ord 2, the merge EPL keeps its trailing
	// space after `id `); and 41-43 (ord 3, a single three-statement module
	// whose create table is module-private and therefore cannot deploy
	// separately).
	tsqKeyedCreate = "@public create table varagg as (key string primary key, total sum(int))"
	tsqKeyedInto   = "into table varagg select sum(intPrimitive) as total from SupportBean group by theString"
	tsqKeyedS0     = "@name('s0') select (select total from varagg where key = s0.p00) as value from SupportBean_S0 as s0"

	tsqUnkeyedCreate = "@public create table InfraOne (string string, intPrimitive int)"
	tsqUnkeyedS0     = "@name('s0') select (select intPrimitive from InfraOne where string = s0.p00) as c0 from SupportBean_S0 as s0"
	tsqUnkeyedInsert = "insert into InfraOne select theString as string, intPrimitive from SupportBean"

	tsqSecIdxCreate = "@public create table MyTable(k0 string primary key, k1 string primary key, p2 string, value int)"
	tsqSecIdxIndex  = "create index MyIndex on MyTable(p2)"
	tsqSecIdxMerge  = "on SupportBean_S0 merge MyTable where p00 = k0 and p01 = k1 when not matched then insert select p00 as k0, p01 as k1, p02 as p2, id as value when matched then update set p2 = p02, value = id "
	tsqSecIdxS0     = "@Name('s0') select (select value from MyTable as tbl where sb.theString = tbl.p2) as c0 from SupportBean as sb"

	tsqFilterModule = "create table MyTable(tablecol string primary key);\n" +
		"insert into MyTable select p00 as tablecol from SupportBean_S0;\n" +
		"@name('s0') select * from SupportBean(theString=(select tablecol from MyTable).orderBy().firstOf())"
)

var (
	infraTableSubqueryJavaSources = []string{
		infraTableSubqueryJavaSource,
	}
	infraTableSubqueryJavaRuntimeIDs = []string{
		"java-runtime-7b449dd45dd6961c5d61",
		"java-runtime-9cde668ef5b4781b068a",
		"java-runtime-8ece65643b6ec15616f2",
		"java-runtime-a839574d871f88c2fdbf",
	}
	infraTableSubqueryJavaExecutions = []string{
		"InfraTableSubqueryAgainstKeyed",
		"InfraTableSubqueryAgainstUnkeyed",
		"InfraTableSubquerySecondaryIndex",
		"InfraTableSubqueryInFilter",
	}
	infraTableSubqueryJavaStaticIDs = []string{
		"java-84c3e4b24f1621c4e20f",
		"java-8a837e4f238a838a719c",
		"java-22c9a3e16812af54b583",
		"java-05c24f1601cc75b1683e",
	}
	infraTableSubqueryCases = []string{
		"subquery-keyed",
		"subquery-unkeyed",
		"subquery-secondary-index",
		"subquery-in-filter",
	}
	infraTableSubqueryOrdinals = []int{0, 1, 2, 3}
)

// infraTableSubqueryCaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata.
var infraTableSubqueryCaseEPLs = []string{
	strings.Join([]string{tsqKeyedCreate, tsqKeyedInto, tsqKeyedS0}, "\n"),
	strings.Join([]string{tsqUnkeyedCreate, tsqUnkeyedS0, tsqUnkeyedInsert}, "\n"),
	strings.Join([]string{tsqSecIdxCreate, tsqSecIdxIndex, tsqSecIdxMerge, tsqSecIdxS0}, "\n"),
	tsqFilterModule,
}

// infraTableSubqueryCaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all, the deployed-label set for marker checks, the per-statement
// listener sequence counters, and the trace the listener records into.
type infraTableSubqueryCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	sequences      map[string]uint64
	trace          *compat.Trace
	caseName       string
}

// runInfraTableSubqueryScenario replays the four InfraTableSubquery
// executions: each case runs on a fresh environment/engine pair (one runtime
// per Java execution) and every step dispatches to the matching runtime
// action. The oracle emits the epoch time for every record, so the runner
// pins the same value.
func runInfraTableSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraTableSubqueryScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableSubqueryCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraTableSubqueryCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraTableSubqueryID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraTableSubqueryID)
	}
	return trace, nil
}

func runInfraTableSubqueryCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableSubqueryBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableSubqueryS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableSubqueryJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraTableSubqueryID}
	state := &infraTableSubqueryCaseState{
		env:            env,
		engine:         engine,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		sequences:      make(map[string]uint64),
		trace:          &trace,
		caseName:       caseName,
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return compat.Trace{}, err
		}
		switch step.Op {
		case "case":
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "deployed":
			if !state.deployedLabels[step.Statement] {
				return compat.Trace{}, fmt.Errorf("%s: deployed marker for unknown statement %q",
					infraTableSubqueryID, step.Statement)
			}
			state.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequences[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			event, err := decodeInfraTableSubqueryPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraTableSubqueryID, step.Op)
		}
	}
	return trace, nil
}

// record emits one listener record per invocation with a per-statement
// sequence counter, mirroring the Java oracle's UpdateListener: only a new
// array renders and only when non-empty.
func (s *infraTableSubqueryCaseState) record(statement string, batch esper.ResultBatch) {
	s.sequences[statement]++
	rec := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "listener",
		Statement: statement,
		Sequence:  s.sequences[statement],
		Time:      compat.FormatTraceTime(batch.Time),
		New:       compat.NormalizeResults(batch.New),
		Old:       compat.NormalizeResults(batch.Old),
	}
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.trace.Records = append(s.trace.Records, rec)
}

// deploy maps each scenario label to the equivalent Go chain-API plans or
// catalog calls. The Java oracle compiles each deploy step's EPL as one
// module; the Go runner deploys the equivalent plans and registers them
// under the step label so undeploy-all bookkeeping matches.
func (s *infraTableSubqueryCaseState) deploy(ctx context.Context, step compat.Step) error {
	switch s.caseName {
	case "subquery-keyed":
		return s.deployKeyed(ctx, step)
	case "subquery-unkeyed":
		return s.deployUnkeyed(ctx, step)
	case "subquery-secondary-index":
		return s.deploySecondaryIndex(ctx, step)
	case "subquery-in-filter":
		return s.deployInFilter(ctx, step)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraTableSubqueryID, s.caseName)
	}
}

// deployKeyed builds the ord-0 fixture: keyed varagg, the grouped into-table
// sum feed and the correlated scalar subquery statement s0.
func (s *infraTableSubqueryCaseState) deployKeyed(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		if _, err := esper.CreateTable(s.env, "varagg", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key"),
			esper.TableColumnOf[int]("total"),
		}); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "into":
		// `into table varagg select sum(intPrimitive) as total from
		// SupportBean group by theString` — the group key maps positionally
		// to primary-key column key.
		theString := esper.Field[infraTableSubqueryBean, string]("theString")
		intPrimitive := esper.Field[infraTableSubqueryBean, int]("intPrimitive")
		plan, err := s.env.Build(esper.From[infraTableSubqueryBean](s.env, "SupportBean").
			GroupBy(theString).
			Select(
				esper.Alias("key", theString),
				esper.Alias("total", esper.Sum[int](intPrimitive)),
			).IntoTable("varagg"))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	case "s0":
		// `select (select total from varagg where key = s0.p00) as value from
		// SupportBean_S0 as s0` — a correlated scalar subquery on the primary
		// key; null when no row matches.
		plan, err := s.env.Build(esper.Select(
			esper.From[infraTableSubqueryS0](s.env, "SupportBean_S0"),
			esper.Alias("value", esper.SubqueryValueWithOptions[int](
				esper.FromTable(s.env, "varagg"),
				esper.Field[any, int]("total"),
				esper.SubqueryWhere(esper.Equal[string](
					esper.Field[any, string]("key"), esper.OuterField[string]("p00"))),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return err
		}
		return s.deployListened(ctx, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unknown subquery-keyed deploy label %q", infraTableSubqueryID, step.Statement)
	}
}

// deployUnkeyed builds the ord-1 fixture: the unkeyed InfraOne, the
// correlated subquery statement s0 deployed BEFORE the insert-into feed.
func (s *infraTableSubqueryCaseState) deployUnkeyed(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		if _, err := esper.CreateTable(s.env, "InfraOne", []esper.TableColumn{
			esper.OptionalTableColumnOf[string]("string"),
			esper.OptionalTableColumnOf[int]("intPrimitive"),
		}); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "s0":
		// `select (select intPrimitive from InfraOne where string = s0.p00)
		// as c0 from SupportBean_S0 as s0` — a correlated scalar subquery
		// over the unkeyed table (full scan).
		plan, err := s.env.Build(esper.Select(
			esper.From[infraTableSubqueryS0](s.env, "SupportBean_S0"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](
				esper.FromTable(s.env, "InfraOne"),
				esper.Field[any, int]("intPrimitive"),
				esper.SubqueryWhere(esper.Equal[string](
					esper.Field[any, string]("string"), esper.OuterField[string]("p00"))),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return err
		}
		return s.deployListened(ctx, step.Statement, plan)
	case "insert":
		// `insert into InfraOne select theString as string, intPrimitive
		// from SupportBean`.
		source := esper.From[infraTableSubqueryBean](s.env, "SupportBean")
		plan, err := s.env.Build(esper.OnEvent(source).InsertIntoTable("InfraOne",
			esper.SetColumn("string", esper.Field[infraTableSubqueryBean, string]("theString")),
			esper.SetColumn("intPrimitive", esper.Field[infraTableSubqueryBean, int]("intPrimitive")),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unknown subquery-unkeyed deploy label %q", infraTableSubqueryID, step.Statement)
	}
}

// deploySecondaryIndex builds the ord-2 fixture: composite-key MyTable, the
// secondary index on p2 created before any rows, the on-SupportBean_S0 merge
// and the correlated subquery statement s0 keyed on the indexed column.
func (s *infraTableSubqueryCaseState) deploySecondaryIndex(ctx context.Context, step compat.Step) error {
	switch step.Statement {
	case "create":
		if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("k0"),
			esper.PrimaryKeyColumn[string]("k1"),
			esper.OptionalTableColumnOf[string]("p2"),
			esper.OptionalTableColumnOf[int]("value"),
		}); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "index":
		// `create index MyIndex on MyTable(p2)` — a late catalog operation on
		// the live table before any rows arrive.
		table, ok := s.engine.Table("MyTable")
		if !ok {
			return fmt.Errorf("%s: MyTable is missing", infraTableSubqueryID)
		}
		if err := table.CreateIndex("MyIndex", []string{"p2"}, esper.IndexHash, false); err != nil {
			return fmt.Errorf("%s: create index: %w", infraTableSubqueryID, err)
		}
		s.deployedLabels[step.Statement] = true
		return nil
	case "merge":
		// `on SupportBean_S0 merge MyTable where p00 = k0 and p01 = k1 when
		// not matched then insert select p00 as k0, p01 as k1, p02 as p2, id
		// as value when matched then update set p2 = p02, value = id ` —
		// the positional keys map p00/p01 to primary-key columns k0/k1.
		p00 := esper.Field[infraTableSubqueryS0, string]("p00")
		p01 := esper.Field[infraTableSubqueryS0, string]("p01")
		p02 := esper.Field[infraTableSubqueryS0, string]("p02")
		id := esper.Field[infraTableSubqueryS0, int]("id")
		plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableSubqueryS0](s.env, "SupportBean_S0")).
			MergeIntoTableWhen("MyTable",
				[]esper.Expr{p00, p01},
				esper.WhenNotMatchedAny(
					esper.SetColumn("k0", p00),
					esper.SetColumn("k1", p01),
					esper.SetColumn("p2", p02),
					esper.SetColumn("value", id)),
				esper.WhenMatchedAny(
					esper.SetColumn("p2", p02),
					esper.SetColumn("value", id)),
			).Query(esper.StatementName("merge")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, step.Statement, plan)
	case "s0":
		// `select (select value from MyTable as tbl where sb.theString =
		// tbl.p2) as c0 from SupportBean as sb` — a correlated scalar
		// subquery resolved through the p2 secondary index.
		plan, err := s.env.Build(esper.Select(
			esper.From[infraTableSubqueryBean](s.env, "SupportBean"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[int](
				esper.FromTable(s.env, "MyTable"),
				esper.Field[any, int]("value"),
				esper.SubqueryWhere(esper.Equal[string](
					esper.Field[any, string]("p2"), esper.OuterField[string]("theString"))),
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return err
		}
		return s.deployListened(ctx, step.Statement, plan)
	default:
		return fmt.Errorf("%s: unknown subquery-secondary-index deploy label %q", infraTableSubqueryID, step.Statement)
	}
}

// deployInFilter builds the ord-3 fixture behind the single "module" deploy
// step: the module-private MyTable maps to an env-level registration, the
// insert-into feed and the filtered select * deploy as plans under the same
// label. The select's stream filter carries the uncorrelated enum
// orderBy().firstOf() subquery `theString=(select tablecol from
// MyTable).orderBy().firstOf()`.
func (s *infraTableSubqueryCaseState) deployInFilter(ctx context.Context, step compat.Step) error {
	if step.Statement != "module" {
		return fmt.Errorf("%s: unknown subquery-in-filter deploy label %q", infraTableSubqueryID, step.Statement)
	}
	if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("tablecol"),
	}); err != nil {
		return err
	}
	// `insert into MyTable select p00 as tablecol from SupportBean_S0`.
	insertPlan, err := s.env.Build(esper.OnEvent(esper.From[infraTableSubqueryS0](s.env, "SupportBean_S0")).
		InsertIntoTable("MyTable",
			esper.SetColumn("tablecol", esper.Field[infraTableSubqueryS0, string]("p00")),
		).Query(esper.StatementName("insert")))
	if err != nil {
		return err
	}
	if err := s.deployPlan(ctx, step.Statement, insertPlan); err != nil {
		return err
	}
	// `select * from SupportBean(theString=(select tablecol from
	// MyTable).orderBy().firstOf())` — the uncorrelated subquery inside the
	// stream filter returns every tablecol; orderBy().firstOf() picks the
	// natural-order minimum.
	firstOf := esper.EnumFirstOf[string](esper.EnumOrderByNatural[string](
		esper.SubqueryValues[string](
			esper.FromTable(s.env, "MyTable"),
			esper.Field[any, string]("tablecol")),
		false))
	selectPlan, err := s.env.Build(esper.Select(
		esper.From[infraTableSubqueryBean](s.env, "SupportBean").Filter(
			esper.Equal[string](esper.Field[infraTableSubqueryBean, string]("theString"), firstOf)),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return err
	}
	return s.deployListened(ctx, step.Statement, selectPlan)
}

func (s *infraTableSubqueryCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	return nil
}

// deployListened deploys a plan and subscribes the listener to its s0
// statement, mirroring the Java oracle's addListener("s0").
func (s *infraTableSubqueryCaseState) deployListened(ctx context.Context, label string,
	plan esper.Plan) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	for _, statement := range deployment.Statements() {
		name := statement.Name()
		if name != "s0" {
			continue
		}
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			s.record(name, batch)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *infraTableSubqueryCaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	s.deployments = make(map[string][]*esper.Deployment)
	s.deployedLabels = make(map[string]bool)
	return nil
}

func decodeInfraTableSubqueryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableSubqueryBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		// Java's `new SupportBean(theString, intPrimitive)` leaves
		// charPrimitive at '\u0000' and every boxed column null.
		value.CharPrimitive = "\u0000"
		return value, nil
	case "SupportBean_S0":
		var value infraTableSubqueryS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraTableSubqueryID, step.EventType)
	}
}

// loadInfraTableSubqueryScenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraTableSubqueryScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableSubqueryID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableSubqueryID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableSubqueryID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableSubqueryID, err)
	}
	if err := requireInfraTableSubqueryFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableSubqueryID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableSubqueryID ||
		metadata.Description != infraTableSubqueryDescription ||
		metadata.JavaCommit != infraTableSubqueryJavaCommit ||
		metadata.JavaSource != infraTableSubqueryJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableSubqueryID)
	}
	if err := validateInfraTableSubqueryStringArray(root["javaRuntimes"], infraTableSubqueryJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSubqueryStringArray(root["javaNames"], infraTableSubqueryJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSubqueryStringArray(root["javaStaticIds"], infraTableSubqueryJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSubqueryStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableSubqueryCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableSubqueryID, len(infraTableSubqueryCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableSubqueryFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != infraTableSubqueryCases[index] ||
			definition.Ordinal != infraTableSubqueryOrdinals[index] ||
			definition.RuntimeID != infraTableSubqueryJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableSubqueryJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != infraTableSubqueryCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableSubqueryID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraTableSubqueryID)
	}
	steps := make([]compat.Step, len(rawSteps))
	objects := make([]map[string]json.RawMessage, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		objects[index] = object
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraTableSubqueryFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraTableSubqueryFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraTableSubqueryFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraTableSubqueryFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraTableSubqueryPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraTableSubqueryFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d case: %w", index, err)
		}
		found := false
		for _, name := range infraTableSubqueryCases {
			if stepCase == name {
				found = true
				break
			}
		}
		if !found {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unknown case %q", index, stepCase)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraTableSubqueryRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraTableSubqueryScenario re-pins the loaded scenario shape
// before replay (the runner entry point validates independently of the
// loader).
func validateInfraTableSubqueryScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraTableSubqueryID {
		return fmt.Errorf("%s scenario shape is not pinned", infraTableSubqueryID)
	}
	return nil
}

// validateInfraTableSubqueryRawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, send event types with canonical payloads, and the
// undeploy-all terminators.
func validateInfraTableSubqueryRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraTableSubqueryCases {
		want, ok := infraTableSubqueryCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraTableSubqueryID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraTableSubqueryID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraTableSubqueryID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraTableSubqueryID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraTableSubqueryStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraTableSubqueryID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraTableSubqueryID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraTableSubqueryID)
	}
	return nil
}

// infraTableSubqueryStepKey renders a raw step object into its pinned
// string form.
func infraTableSubqueryStepKey(object map[string]json.RawMessage, operation string) (string, error) {
	stringField := func(name string) (string, error) {
		var value string
		if err := json.Unmarshal(object[name], &value); err != nil {
			return "", fmt.Errorf("step field %q must be a string", name)
		}
		return value, nil
	}
	switch operation {
	case "deploy":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		return "deploy:" + statement + ":" + epl, nil
	case "deployed":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		return operation + ":" + statement, nil
	case "send":
		eventType, err := stringField("eventType")
		if err != nil {
			return "", err
		}
		var payload map[string]any
		if err := json.Unmarshal(object["payload"], &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		return "send:" + eventType + ":" + string(canonical), nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// tsqKeyedCaseSteps renders the pinned step sequence of
// InfraTableSubqueryAgainstKeyed.run (lines 75-93): the create-table,
// into-table and s0 deploys, SupportBean("G2",200), the assertValues G1/G2
// S0 sends, milestone(0) (no step), SupportBean("G1",100), the second G1/G2
// pair and undeployAll.
func tsqKeyedCaseSteps() []string {
	return []string{
		"deploy:create:" + tsqKeyedCreate,
		"deployed:create",
		"deploy:into:" + tsqKeyedInto,
		"deployed:into",
		"deploy:s0:" + tsqKeyedS0,
		"deployed:s0",
		`send:SupportBean:{"intPrimitive":200,"theString":"G2"}`,
		`send:SupportBean_S0:{"id":0,"p00":"G1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"G2"}`,
		`send:SupportBean:{"intPrimitive":100,"theString":"G1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"G1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"G2"}`,
		"undeploy-all",
	}
}

// tsqUnkeyedCaseSteps renders the pinned step sequence of
// InfraTableSubqueryAgainstUnkeyed.run (lines 99-113): the unkeyed
// create-table, the s0 subquery deploy BEFORE the insert-into feed,
// SupportBean("E1",10), milestone(0) (no step), S0(0,"E1") and undeployAll.
func tsqUnkeyedCaseSteps() []string {
	return []string{
		"deploy:create:" + tsqUnkeyedCreate,
		"deployed:create",
		"deploy:s0:" + tsqUnkeyedS0,
		"deployed:s0",
		"deploy:insert:" + tsqUnkeyedInsert,
		"deployed:insert",
		`send:SupportBean:{"intPrimitive":10,"theString":"E1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"E1"}`,
		"undeploy-all",
	}
}

// tsqSecondaryIndexCaseSteps renders the pinned step sequence of
// InfraTableSubquerySecondaryIndex.run (lines 118-148): the composite-key
// create-table, the secondary index on p2 before any rows, the on-merge
// deploy, the s0 subquery deploy, the S0(10,G1,SG1,P2_1) insert and the
// P2_1 assert, milestone(0), the S0(11,G1,SG1,P2_2) update, milestone(1),
// the P2_1/P2_2 asserts and undeployAll.
func tsqSecondaryIndexCaseSteps() []string {
	return []string{
		"deploy:create:" + tsqSecIdxCreate,
		"deployed:create",
		"deploy:index:" + tsqSecIdxIndex,
		"deployed:index",
		"deploy:merge:" + tsqSecIdxMerge,
		"deployed:merge",
		"deploy:s0:" + tsqSecIdxS0,
		"deployed:s0",
		`send:SupportBean_S0:{"id":10,"p00":"G1","p01":"SG1","p02":"P2_1"}`,
		`send:SupportBean:{"intPrimitive":-1,"theString":"P2_1"}`,
		`send:SupportBean_S0:{"id":11,"p00":"G1","p01":"SG1","p02":"P2_2"}`,
		`send:SupportBean:{"intPrimitive":-1,"theString":"P2_1"}`,
		`send:SupportBean:{"intPrimitive":-1,"theString":"P2_2"}`,
		"undeploy-all",
	}
}

// tsqInFilterCaseSteps renders the pinned step sequence of
// InfraTableSubqueryInFilter.run (lines 40-61): one module deploy carrying
// the module-private create table, the insert-into feed and the filtered
// select *; the sendAssert/sendS0 interleaving (filtered-out SupportBean
// sends emit no record), the milestone(0) no-op and undeployAll.
func tsqInFilterCaseSteps() []string {
	return []string{
		"deploy:module:" + tsqFilterModule,
		"deployed:module",
		`send:SupportBean:{"intPrimitive":0,"theString":"E"}`,
		`send:SupportBean_S0:{"id":0,"p00":"E"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"E"}`,
		`send:SupportBean_S0:{"id":0,"p00":"C"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"E"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"C"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"A"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"C"}`,
		`send:SupportBean_S0:{"id":0,"p00":"A"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"A"}`,
		`send:SupportBean:{"intPrimitive":0,"theString":"C"}`,
		"undeploy-all",
	}
}

// infraTableSubqueryCaseSteps pins the exact op sequence per case.
var infraTableSubqueryCaseSteps = map[string][]string{
	"subquery-keyed":           tsqKeyedCaseSteps(),
	"subquery-unkeyed":         tsqUnkeyedCaseSteps(),
	"subquery-secondary-index": tsqSecondaryIndexCaseSteps(),
	"subquery-in-filter":       tsqInFilterCaseSteps(),
}

func requireInfraTableSubqueryFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraTableSubqueryID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraTableSubqueryID, name)
		}
	}
	return nil
}

func validateInfraTableSubqueryStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
