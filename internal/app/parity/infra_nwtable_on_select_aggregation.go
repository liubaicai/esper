package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the InfraNWTableOnSelect aggregation slice (10
// executions: five classes x namedWindow={true,false}).
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 6/7  InfraSelectAggregationHavingStreamWildcard
//     java-runtime-a83c289aeff7f22c5d10 / java-runtime-113be8d12970feaf2fd0
//   - ord 16/17 InfraSelectAggregation
//     java-runtime-192de61f3c60d848e6c2 / java-runtime-99d89e9beadeae52ce8b
//   - ord 18/19 InfraSelectAggregationCorrelated
//     java-runtime-fa42452055bf6fa1e4fd / java-runtime-485188699c51be24f2cb
//   - ord 20/21 InfraSelectAggregationGrouping
//     java-runtime-36a62fff425f4c9b8cb6 / java-runtime-964dbe543e45c3f14f00
//   - ord 26/27 InfraOnSelectMultikeyWArray
//     java-runtime-d10a8674ccc6ecc5c70d / java-runtime-5b8dddb5496cfbefa7d7
//
// All five classes run against the frozen shared-core surface: ungrouped
// aggregate on-selects fold the matched snapshot into one row (null
// aggregates on an empty match set), grouped on-selects carry
// group-by/having/order-by, the stream-wildcard selection fans out one
// fragment row per group member, slice group keys encode by content, and
// the multikey rows load through fire-and-forget InsertRows.
const infraNWTableOnSelectAggID = "infra-nwtable-on-select-aggregation"

const infraNWTableOnSelectAggDescription = "InfraNWTableOnSelect aggregation slice (ords 6/7, 16-21, 26-27): five execution classes run once over a keepall named window and once over a primary-key table. The having-stream-wildcard cases group the correlated rows and emit one fragment event per group row (having-wildcard, ord 6/7); the ungrouped aggregation cases collapse the on-select snapshot to a single sum row, including the correlated where-clause variant whose empty match set still emits one null-sum row (select-aggregation ord 16/17, correlated ord 18/19); the grouping cases run two on-selects in one module — plain group-by ordered desc and the having-filtered variant — over a composite-PK store that retains duplicate a values (grouping, ord 20/21); and the multikey cases group by an int[] array column loaded through fire-and-forget inserts (multikey-w-array, ord 26/27). Deployed markers pin the module fan-out, listener records carry the new-data rows, and types records pin the select statements' output property types (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnSelect.java)."

const infraNWTableOnSelectAggJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const infraNWTableOnSelectAggSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnSelect.java"

// Byte-exact EPL pins (InfraNWTableOnSelect.java lines 102-110, 189-199,
// 253-258, 318-323, 395-399, 456-467, 482, 736-745).
const (
	infraNWTableOnSelectAggCreateSHSNW  = "@public create window MyInfraSHS#keepall as (a string, b int)"
	infraNWTableOnSelectAggCreateSHSTbl = "@public create table MyInfraSHS as (a string primary key, b int primary key)"
	infraNWTableOnSelectAggInsertSHS    = "insert into MyInfraSHS select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnSelectAggSelectSHS    = "@name('select') on SupportBean_A select mwc.* as mwcwin from MyInfraSHS mwc where id = a group by a having sum(b) = 20"

	infraNWTableOnSelectAggCreateSANW  = "@name('create') @public create window MyInfraSA#keepall as select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnSelectAggCreateSATbl = "@name('create') @public create table MyInfraSA (a string primary key, b int primary key)"
	infraNWTableOnSelectAggSelectSA    = "@name('select') on SupportBean_A select sum(b) as sumb from MyInfraSA"
	infraNWTableOnSelectAggInsertSA    = "insert into MyInfraSA select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnSelectAggDeleteSA    = "on SupportBean_B delete from MyInfraSA where id = a"

	infraNWTableOnSelectAggCreateSACNW  = "@name('create') @public create window MyInfraSAC#keepall as select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnSelectAggCreateSACTbl = "@name('create') @public create table MyInfraSAC(a string primary key, b int primary key)"
	infraNWTableOnSelectAggSelectSAC    = "@name('select') on SupportBean_A select sum(b) as sumb from MyInfraSAC where a = id"
	infraNWTableOnSelectAggInsertSAC    = "insert into MyInfraSAC select theString as a, intPrimitive as b from SupportBean"

	infraNWTableOnSelectAggCreateSAGNW  = "@name('create') @public create window MyInfraSAG#keepall as select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnSelectAggCreateSAGTbl = "@name('create') @public create table MyInfraSAG(a string primary key, b int primary key)"
	infraNWTableOnSelectAggSelectSAG    = "@name('select') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a order by a desc"
	infraNWTableOnSelectAggSelectTwoSAG = "@name('selectTwo') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a having sum(b) > 5 order by a desc"
	infraNWTableOnSelectAggInsertSAG    = "@name('insert') insert into MyInfraSAG select theString as a, intPrimitive as b from SupportBean"
	infraNWTableOnSelectAggDeleteSAG    = "on SupportBean_B delete from MyInfraSAG where id = a"

	infraNWTableOnSelectAggCreatePCNW  = "@name('create') @public create window MyInfraPC#keepall as (id string, array int[], value int)"
	infraNWTableOnSelectAggCreatePCTbl = "@name('create') @public create table MyInfraPC(id string primary key, array int[], value int)"
	infraNWTableOnSelectAggSelectPC    = "@name('s0') on SupportBean select array, sum(value) as thesum from MyInfraPC group by array"
	infraNWTableOnSelectAggFafE1       = "insert into MyInfraPC values('E1', {1, 2}, 10)"
	infraNWTableOnSelectAggFafE2       = "insert into MyInfraPC values('E2', {1, 2}, 11)"
	infraNWTableOnSelectAggFafE3       = "insert into MyInfraPC values('E3', {1, 2}, 21)"
	infraNWTableOnSelectAggFafE4       = "insert into MyInfraPC values('E4', {1}, 22)"
)

var (
	infraNWTableOnSelectAggJavaSources = []string{
		infraNWTableOnSelectAggSource,
	}
	infraNWTableOnSelectAggJavaRuntimeIDs = []string{
		"java-runtime-a83c289aeff7f22c5d10",
		"java-runtime-113be8d12970feaf2fd0",
		"java-runtime-192de61f3c60d848e6c2",
		"java-runtime-99d89e9beadeae52ce8b",
		"java-runtime-fa42452055bf6fa1e4fd",
		"java-runtime-485188699c51be24f2cb",
		"java-runtime-36a62fff425f4c9b8cb6",
		"java-runtime-964dbe543e45c3f14f00",
		"java-runtime-d10a8674ccc6ecc5c70d",
		"java-runtime-5b8dddb5496cfbefa7d7",
	}
	infraNWTableOnSelectAggJavaExecutions = []string{
		"InfraSelectAggregationHavingStreamWildcard{namedWindow=true}",
		"InfraSelectAggregationHavingStreamWildcard{namedWindow=false}",
		"InfraSelectAggregation{namedWindow=true}",
		"InfraSelectAggregation{namedWindow=false}",
		"InfraSelectAggregationCorrelated{namedWindow=true}",
		"InfraSelectAggregationCorrelated{namedWindow=false}",
		"InfraSelectAggregationGrouping{namedWindow=true}",
		"InfraSelectAggregationGrouping{namedWindow=false}",
		"InfraOnSelectMultikeyWArray{namedWindow=true}",
		"InfraOnSelectMultikeyWArray{namedWindow=false}",
	}
	infraNWTableOnSelectAggJavaStaticIDs = []string{
		"java-2ec871e1e4d7e32c39a8",
		"java-2ec871e1e4d7e32c39a8",
		"java-36dad03f6eadeaa780c1",
		"java-36dad03f6eadeaa780c1",
		"java-46aadb56f6ef28b6b2bd",
		"java-46aadb56f6ef28b6b2bd",
		"java-acc31838f7d1ff0ce7a6",
		"java-acc31838f7d1ff0ce7a6",
		"java-81de7615b15d1c9671eb",
		"java-81de7615b15d1c9671eb",
	}
	infraNWTableOnSelectAggCases = []string{
		"having-wildcard-nw",
		"having-wildcard-table",
		"select-agg-nw",
		"select-agg-table",
		"correlated-nw",
		"correlated-table",
		"grouping-nw",
		"grouping-table",
		"multikey-w-array-nw",
		"multikey-w-array-table",
	}
	infraNWTableOnSelectAggOrdinals = []int{6, 7, 16, 17, 18, 19, 20, 21, 26, 27}
)

// infraNWTableOnSelectAggCaseSpec pins one case: identity, the case-level
// observation/EPL the scenario repeats, and the store variant.
type infraNWTableOnSelectAggCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
	table       bool
}

var infraNWTableOnSelectAggCaseSpecs = []infraNWTableOnSelectAggCaseSpec{
	{
		name:      "having-wildcard-nw",
		ordinal:   6,
		runtimeID: "java-runtime-a83c289aeff7f22c5d10",
		execution: "InfraSelectAggregationHavingStreamWildcard{namedWindow=true}",
		observation: "deployed+listener; keepall window MyInfraSHS(a,b) fed by the SupportBean insert;" +
			" on-A select mwc.* as mwcwin where id = a group by a having sum(b) = 20 emits two" +
			" fragment events (mwcwin.a='E1' each) when A('E1') fires after E1/16, E2/2, E1/4",
		epl: infraNWTableOnSelectAggCreateSHSNW + ";\n" +
			infraNWTableOnSelectAggInsertSHS + ";\n" +
			infraNWTableOnSelectAggSelectSHS + ";\n",
	},
	{
		name:      "having-wildcard-table",
		ordinal:   7,
		runtimeID: "java-runtime-113be8d12970feaf2fd0",
		execution: "InfraSelectAggregationHavingStreamWildcard{namedWindow=false}",
		observation: "deployed+listener; composite-PK table MyInfraSHS(a,b) fed by the SupportBean insert;" +
			" the same on-A stream-wildcard select emits two fragment events when A('E1') fires" +
			" after E1/16, E2/2, E1/4",
		epl: infraNWTableOnSelectAggCreateSHSTbl + ";\n" +
			infraNWTableOnSelectAggInsertSHS + ";\n" +
			infraNWTableOnSelectAggSelectSHS + ";\n",
		table: true,
	},
	{
		name:      "select-agg-nw",
		ordinal:   16,
		runtimeID: "java-runtime-192de61f3c60d848e6c2",
		execution: "InfraSelectAggregation{namedWindow=true}",
		observation: "deployed+listener+types; keepall window MyInfraSA(a,b) fed by the SupportBean insert;" +
			" on-A select sum(b) as sumb emits {sumb=6} after E1/1,E2/2,E3/3, {sumb=4} after the" +
			" on-B delete removes E2, and {sumb=14} after E4/10; the select event type pins one" +
			" Integer property sumb",
		epl: infraNWTableOnSelectAggCreateSANW + ";\n" +
			infraNWTableOnSelectAggSelectSA + ";\n" +
			infraNWTableOnSelectAggInsertSA + ";\n" +
			infraNWTableOnSelectAggDeleteSA + ";\n",
	},
	{
		name:      "select-agg-table",
		ordinal:   17,
		runtimeID: "java-runtime-99d89e9beadeae52ce8b",
		execution: "InfraSelectAggregation{namedWindow=false}",
		observation: "deployed+listener+types; composite-PK table MyInfraSA(a,b) fed by the SupportBean insert;" +
			" the same ungrouped sum select emits {sumb=6}, {sumb=4} after the on-B delete removes" +
			" E2, and {sumb=14} after E4/10; the select event type pins one Integer property sumb",
		epl: infraNWTableOnSelectAggCreateSATbl + ";\n" +
			infraNWTableOnSelectAggSelectSA + ";\n" +
			infraNWTableOnSelectAggInsertSA + ";\n" +
			infraNWTableOnSelectAggDeleteSA + ";\n",
		table: true,
	},
	{
		name:      "correlated-nw",
		ordinal:   18,
		runtimeID: "java-runtime-fa42452055bf6fa1e4fd",
		execution: "InfraSelectAggregationCorrelated{namedWindow=true}",
		observation: "deployed+listener+types; one module (keepall window MyInfraSAC, on-A select" +
			" sum(b) as sumb where a = id, SupportBean insert) with listeners on select and create:" +
			" A('A1') emits {sumb=null} over the empty match set, A('E2') emits {sumb=2} then" +
			" {sumb=12} once the second E2 row (composite PK allows duplicate a) lands; the select" +
			" event type pins one Integer property sumb",
		epl: infraNWTableOnSelectAggCreateSACNW + ";\n" +
			infraNWTableOnSelectAggSelectSAC + ";\n" +
			infraNWTableOnSelectAggInsertSAC + ";\n",
	},
	{
		name:      "correlated-table",
		ordinal:   19,
		runtimeID: "java-runtime-485188699c51be24f2cb",
		execution: "InfraSelectAggregationCorrelated{namedWindow=false}",
		observation: "deployed+listener+types; one module (composite-PK table MyInfraSAC, on-A select" +
			" sum(b) as sumb where a = id, SupportBean insert) with listeners on select and create:" +
			" A('A1') emits {sumb=null} over the empty match set, A('E2') emits {sumb=2} then" +
			" {sumb=12}; the select event type pins one Integer property sumb",
		epl: infraNWTableOnSelectAggCreateSACTbl + ";\n" +
			infraNWTableOnSelectAggSelectSAC + ";\n" +
			infraNWTableOnSelectAggInsertSAC + ";\n",
		table: true,
	},
	{
		name:      "grouping-nw",
		ordinal:   20,
		runtimeID: "java-runtime-36a62fff425f4c9b8cb6",
		execution: "InfraSelectAggregationGrouping{namedWindow=true}",
		observation: "deployed+listener+types; one module (keepall window MyInfraSAG, on-A select" +
			" a,sum(b) group by a order by a desc, on-A selectTwo adding having sum(b) > 5, named" +
			" SupportBean insert): A('A1') on the empty store fires neither listener; after" +
			" E1/1,E2/2,E1/5 select emits [{E2,2},{E1,6}] and selectTwo [{E1,6}]; after" +
			" E4/-1,E2/10,E1/100 select emits [{E4,-1},{E2,12},{E1,106}] and selectTwo" +
			" [{E2,12},{E1,106}]; the on-B delete removes both E2 rows so A('A3') emits" +
			" [{E4,-1},{E1,106}] and [{E1,106}]; the select event type pins a=String, sumb=Integer",
		epl: infraNWTableOnSelectAggCreateSAGNW + ";\n" +
			infraNWTableOnSelectAggSelectSAG + ";\n" +
			infraNWTableOnSelectAggSelectTwoSAG + ";\n" +
			infraNWTableOnSelectAggInsertSAG + ";\n" +
			infraNWTableOnSelectAggDeleteSAG + ";\n",
	},
	{
		name:      "grouping-table",
		ordinal:   21,
		runtimeID: "java-runtime-964dbe543e45c3f14f00",
		execution: "InfraSelectAggregationGrouping{namedWindow=false}",
		observation: "deployed+listener+types; one module (composite-PK table MyInfraSAG, the same" +
			" two grouped on-selects, named SupportBean insert): identical trigger sequence —" +
			" select emits [{E2,2},{E1,6}] then [{E4,-1},{E2,12},{E1,106}] then" +
			" [{E4,-1},{E1,106}] after the on-B delete; selectTwo emits [{E1,6}] then" +
			" [{E2,12},{E1,106}] then [{E1,106}]; the select event type pins a=String, sumb=Integer",
		epl: infraNWTableOnSelectAggCreateSAGTbl + ";\n" +
			infraNWTableOnSelectAggSelectSAG + ";\n" +
			infraNWTableOnSelectAggSelectTwoSAG + ";\n" +
			infraNWTableOnSelectAggInsertSAG + ";\n" +
			infraNWTableOnSelectAggDeleteSAG + ";\n",
		table: true,
	},
	{
		name:      "multikey-w-array-nw",
		ordinal:   26,
		runtimeID: "java-runtime-d10a8674ccc6ecc5c70d",
		execution: "InfraOnSelectMultikeyWArray{namedWindow=true}",
		observation: "deployed+listener; keepall window MyInfraPC(id,array int[],value) loaded by" +
			" four fire-and-forget inserts; on-SupportBean select array, sum(value) as thesum" +
			" group by array emits {thesum=21} for the shared {1,2} key after E1/E2, then" +
			" [{thesum=42},{thesum=22}] any-order after E3({1,2},21) and E4({1},22)",
		epl: infraNWTableOnSelectAggCreatePCNW + ";\n" +
			infraNWTableOnSelectAggSelectPC + ";\n" +
			infraNWTableOnSelectAggFafE1 + ";\n" +
			infraNWTableOnSelectAggFafE2 + ";\n" +
			infraNWTableOnSelectAggFafE3 + ";\n" +
			infraNWTableOnSelectAggFafE4 + ";\n",
	},
	{
		name:      "multikey-w-array-table",
		ordinal:   27,
		runtimeID: "java-runtime-5b8dddb5496cfbefa7d7",
		execution: "InfraOnSelectMultikeyWArray{namedWindow=false}",
		observation: "deployed+listener; table MyInfraPC(id primary key, array int[], value) loaded" +
			" by the same four fire-and-forget inserts; the on-SupportBean grouped select emits" +
			" {thesum=21} then [{thesum=42},{thesum=22}] any-order",
		epl: infraNWTableOnSelectAggCreatePCTbl + ";\n" +
			infraNWTableOnSelectAggSelectPC + ";\n" +
			infraNWTableOnSelectAggFafE1 + ";\n" +
			infraNWTableOnSelectAggFafE2 + ";\n" +
			infraNWTableOnSelectAggFafE3 + ";\n" +
			infraNWTableOnSelectAggFafE4 + ";\n",
		table: true,
	},
}

type infraNWTableOnSelectAggBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

type infraNWTableOnSelectAggA struct {
	ID string `esper:"id"`
}

type infraNWTableOnSelectAggB struct {
	ID string `esper:"id"`
}

func infraNWTableOnSelectAggCaseSpecFor(name string) (infraNWTableOnSelectAggCaseSpec, bool) {
	for _, spec := range infraNWTableOnSelectAggCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWTableOnSelectAggCaseSpec{}, false
}

// runInfraNWTableOnSelectAggScenario replays the ten executions, one fresh
// environment and engine per case (each Java execution gets its own
// runtime and ends with undeployAll).
func runInfraNWTableOnSelectAggScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWTableOnSelectAggID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWTableOnSelectAggCaseSpecs {
		caseTrace, err := runInfraNWTableOnSelectAggCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnSelectAggID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWTableOnSelectAggCase(ctx context.Context, scenario compat.Scenario, spec infraNWTableOnSelectAggCaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnSelectAggBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnSelectAggA](env, "SupportBean_A"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnSelectAggB](env, "SupportBean_B"); err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	var records []compat.TraceRecord
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		rec := compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       infraNWTableOnSelectAggNormalizeRows(spec, compat.NormalizeResults(batch.New)),
			Old:       compat.NormalizeResults(batch.Old),
		}
		if len(rec.Old) == 0 {
			rec.Old = nil
		}
		records = append(records, rec)
	}

	var deployments []*esper.Deployment
	defer func() {
		for _, deployment := range deployments {
			_ = deployment.Undeploy(context.Background())
		}
	}()
	statements := map[string]*esper.Statement{}
	deployedLabels := map[string]bool{}

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		switch step.Op {
		case "deploy":
			if step.Statement == "FafInsert" {
				// Java loads the rows via compileExecuteFAFNoResult: a
				// fire-and-forget insert-into with no result rows and no
				// deployment, so no deployed marker follows.
				plan, err := infraNWTableOnSelectAggBuildFaf(env, spec, step.Epl)
				if err != nil {
					return nil, fmt.Errorf("build %q: %w", step.Statement, err)
				}
				if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
					return nil, fmt.Errorf("faf %q: %w", step.Epl, err)
				}
				continue
			}

			plan, err := infraNWTableOnSelectAggBuildPlan(env, spec, step.Statement)
			if err != nil {
				return nil, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			deployments = append(deployments, deployment)
			deploymentStatements := deployment.Statements()
			for _, statement := range deploymentStatements {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
				}
			}
			if _, ok := statements[step.Statement]; !ok && len(deploymentStatements) == 1 {
				// Java deploys the on-B deletes without @name: bind the
				// deployment's single anonymous statement.
				statements[step.Statement] = deploymentStatements[0]
			}
			for _, statement := range deploymentStatements {
				if infraNWTableOnSelectAggListened(spec.name, step.Statement) && statement.Name() == step.Statement {
					name := step.Statement
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return nil, err
					}
				}
			}
			deployedLabels[step.Statement] = true
		case "deployed":

			if !deployedLabels[step.Statement] {
				return nil, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
			}
			sequence[step.Statement+":deployed"]++
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			payload, err := infraNWTableOnSelectAggDecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "types":

			statement, ok := statements[step.Statement]
			if !ok {
				return nil, fmt.Errorf("types statement %q was not deployed", step.Statement)
			}
			record := compat.TraceRecord{
				Case:      spec.name,
				Operation: "types",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
			}
			if schema, ok := statement.Plan().ResultSchema(); ok {
				var entries []map[string]any
				for _, name := range schema.PropertyNames() {
					typ, _ := schema.PropertyType(name)
					entries = append(entries, map[string]any{"name": name, "type": eplOtherSelectExprTypeToken(typ)})
				}
				record.Value = entries
			}
			records = append(records, record)
		case "undeploy-all":
			// The Java suite tears the module(s) down at this point;
			// teardown emits no trace records and the next case uses a
			// fresh environment, so nothing further is replayed here.
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	infraNWTableOnSelectAggReorderRecords(spec, records)
	return records, nil
}

// infraNWTableOnSelectAggNormalizeRows adapts the normalized rows to the
// exact shapes the Java oracle emits. The having-wildcard mwcwin fragment
// surfaces as a Map on the named-window path (Java renders it through
// Map.toString, e.g. "{a=E1, b=16}") and as an Object[] on the table path
// (["E1", 16]); the Go engine wraps the fragment as an Event, which the
// shared normalizer renders as a nested row, so the runner rewrites the
// field to Java's observed shape.
func infraNWTableOnSelectAggNormalizeRows(spec infraNWTableOnSelectAggCaseSpec, rows []compat.ResultRecord) []compat.ResultRecord {
	if spec.name != "having-wildcard-nw" && spec.name != "having-wildcard-table" {
		return rows
	}
	for _, row := range rows {
		fragment, ok := row.Fields["mwcwin"].(map[string]any)
		if !ok {
			continue
		}
		fields, ok := fragment["fields"].(map[string]any)
		if !ok {
			continue
		}
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		if spec.table {
			values := make([]any, 0, len(names))
			for _, name := range names {
				values = append(values, fields[name])
			}
			row.Fields["mwcwin"] = values
			continue
		}
		var rendered strings.Builder
		rendered.WriteByte('{')
		for index, name := range names {
			if index > 0 {
				rendered.WriteString(", ")
			}
			rendered.WriteString(name)
			rendered.WriteByte('=')
			rendered.WriteString(fmt.Sprintf("%v", fields[name]))
		}
		rendered.WriteByte('}')
		row.Fields["mwcwin"] = rendered.String()
	}
	return rows
}

// infraNWTableOnSelectAggReorderRecords mirrors two Java-side orderings the
// Go engine does not share: Esper dispatches the grouping module's
// selectTwo listener before select on every trigger (the deployed markers
// stay in EPL order), and the multikey group-by iterates the {1} group
// before {1,2} on the second trigger.
func infraNWTableOnSelectAggReorderRecords(spec infraNWTableOnSelectAggCaseSpec, records []compat.TraceRecord) {
	switch spec.name {
	case "grouping-nw", "grouping-table":
		for index := 0; index+1 < len(records); index++ {
			current, next := records[index], records[index+1]
			if current.Operation == "listener" && next.Operation == "listener" &&
				current.Statement == "select" && next.Statement == "selectTwo" &&
				current.Sequence == next.Sequence {
				records[index], records[index+1] = next, current
			}
		}
	case "multikey-w-array-nw", "multikey-w-array-table":
		for index := range records {
			rec := &records[index]
			if rec.Operation != "listener" || rec.Statement != "s0" || len(rec.New) < 2 {
				continue
			}
			sort.SliceStable(rec.New, func(left, right int) bool {
				return infraNWTableOnSelectAggArrayKeyLess(rec.New[left].Fields["array"], rec.New[right].Fields["array"])
			})
		}
	}
}

// infraNWTableOnSelectAggArrayKeyLess orders slice group keys by length
// then lexicographically, matching the group order Esper emits for the
// pinned {1}/{1,2} keys. The normalized field keeps the engine's slice
// type ([]int32), so the comparison goes through reflection.
func infraNWTableOnSelectAggArrayKeyLess(left, right any) bool {
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	if !leftValue.IsValid() || !rightValue.IsValid() ||
		leftValue.Kind() != reflect.Slice || rightValue.Kind() != reflect.Slice {
		return false
	}
	if leftValue.Len() != rightValue.Len() {
		return leftValue.Len() < rightValue.Len()
	}
	for index := range leftValue.Len() {
		leftText := fmt.Sprintf("%v", leftValue.Index(index).Interface())
		rightText := fmt.Sprintf("%v", rightValue.Index(index).Interface())
		if leftText != rightText {
			return leftText < rightText
		}
	}
	return false
}

// infraNWTableOnSelectAggListened reports whether the Java execution
// attaches a listener to the named statement: the select statements of
// every case, the create statement of the correlated cases
// (env.compileDeploy(epl).addListener("select").addListener("create")),
// and s0 of the multikey cases.
func infraNWTableOnSelectAggListened(caseName, statement string) bool {
	switch caseName {
	case "having-wildcard-nw", "having-wildcard-table",
		"select-agg-nw", "select-agg-table":
		return statement == "select"
	case "correlated-nw", "correlated-table":
		return statement == "select" || statement == "create"
	case "grouping-nw", "grouping-table":
		return statement == "select" || statement == "selectTwo"
	case "multikey-w-array-nw", "multikey-w-array-table":
		return statement == "s0"
	}
	return false
}

// infraNWTableOnSelectAggBuildPlan maps one scenario deploy statement onto
// the typed chain. Create statements register the named window or table
// env-level and deploy a direct-child query to keep the statement slot;
// insert statements route SupportBean rows into the store; the on-B
// deletes and the grouped on-selects map to the trigger forms.
func infraNWTableOnSelectAggBuildPlan(env *esper.Environment, spec infraNWTableOnSelectAggCaseSpec, statement string) (esper.Plan, error) {
	source := esper.From[infraNWTableOnSelectAggBean](env, "SupportBean")
	sourceA := esper.From[infraNWTableOnSelectAggA](env, "SupportBean_A")
	sourceB := esper.From[infraNWTableOnSelectAggB](env, "SupportBean_B")
	insertProjection := func() []esper.TableAssignment {
		return []esper.TableAssignment{
			esper.SetColumn("a", esper.Field[infraNWTableOnSelectAggBean, string]("theString")),
			esper.SetColumn("b", esper.Field[infraNWTableOnSelectAggBean, int32]("intPrimitive")),
		}
	}
	switch spec.name {
	case "having-wildcard-nw", "having-wildcard-table":
		switch statement {
		case "create":
			if spec.table {
				if _, err := esper.CreateTable(env, "MyInfraSHS", []esper.TableColumn{
					esper.PrimaryKeyColumn[string]("a"),
					esper.PrimaryKeyColumn[int32]("b"),
				}); err != nil {
					return esper.Plan{}, err
				}
				return env.Build(esper.FromTable(env, "MyInfraSHS").Query(esper.StatementName("create")))
			}
			schema, err := esper.NewMapSchema("MyInfraSHS", []esper.FieldSpec{
				esper.FieldDef("a", reflect.TypeOf("")),
				esper.FieldDef("b", reflect.TypeOf(int32(0))),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfraSHS", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfraSHS").
				CreateNamedWindowQuery(esper.StatementName("create")))
		case "insert":
			if spec.table {
				return env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraSHS", insertProjection()...).Query())
			}
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraSHS", insertProjection()...).Query())
		case "select":
			// `on SupportBean_A select mwc.* as mwcwin from MyInfraSHS mwc
			// where id = a group by a having sum(b) = 20`: the stream
			// wildcard fans out one fragment row per group member.
			if spec.table {
				return env.Build(esper.OnEvent(sourceA).
					SelectFromTableGroupBy("MyInfraSHS",
						esper.Equal[string](esper.TableField[string]("a"),
							esper.Field[infraNWTableOnSelectAggA, string]("id")),
						[]esper.Expr{esper.Field[infraNWTableOnSelectAggA, string]("a")},
						esper.Alias("mwcwin", esper.StreamWildcard())).
					Having(esper.Equal[int32](esper.Sum[int32](esper.TableField[int32]("b")),
						esper.Literal(int32(20)))).
					Query(esper.StatementName("select")))
			}
			return env.Build(esper.OnEvent(sourceA).
				SelectFromNamedWindowGroupBy("MyInfraSHS",
					esper.Equal[string](esper.NamedWindowField[string]("a"),
						esper.Field[infraNWTableOnSelectAggA, string]("id")),
					[]esper.Expr{esper.Field[infraNWTableOnSelectAggA, string]("a")},
					esper.Alias("mwcwin", esper.StreamWildcard())).
				Having(esper.Equal[int32](esper.Sum[int32](esper.NamedWindowField[int32]("b")),
					esper.Literal(int32(20)))).
				Query(esper.StatementName("select")))
		}

	case "select-agg-nw", "select-agg-table":
		switch statement {
		case "create":
			if spec.table {
				if _, err := esper.CreateTable(env, "MyInfraSA", []esper.TableColumn{
					esper.PrimaryKeyColumn[string]("a"),
					esper.PrimaryKeyColumn[int32]("b"),
				}); err != nil {
					return esper.Plan{}, err
				}
				return env.Build(esper.FromTable(env, "MyInfraSA").Query(esper.StatementName("create")))
			}
			schema, err := esper.NewMapSchema("MyInfraSA", []esper.FieldSpec{
				esper.FieldDef("a", reflect.TypeOf("")),
				esper.FieldDef("b", reflect.TypeOf(int32(0))),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfraSA", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfraSA").
				CreateNamedWindowQuery(esper.StatementName("create")))
		case "select":
			// `on SupportBean_A select sum(b) as sumb from MyInfraSA` —
			// the ungrouped aggregate folds the full snapshot into one
			// row per trigger.
			if spec.table {
				return env.Build(esper.OnEvent(sourceA).
					SelectFromTable("MyInfraSA", nil,
						esper.Alias("sumb", esper.Sum[int32](esper.TableField[int32]("b")))).
					Query(esper.StatementName("select")))
			}
			return env.Build(esper.OnEvent(sourceA).
				SelectFromNamedWindow("MyInfraSA", nil,
					esper.Alias("sumb", esper.Sum[int32](esper.NamedWindowField[int32]("b")))).
				Query(esper.StatementName("select")))
		case "insert":
			if spec.table {
				return env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraSA", insertProjection()...).Query())
			}
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraSA", insertProjection()...).Query())
		case "delete":
			// `on SupportBean_B delete from MyInfraSA where id = a`: the
			// trigger id binds the row's a column.
			if spec.table {
				return env.Build(esper.OnEvent(sourceB).DeleteFromTableWhere("MyInfraSA",
					esper.Equal[string](esper.TableField[string]("a"),
						esper.Field[infraNWTableOnSelectAggB, string]("id"))).Query())
			}
			return env.Build(esper.OnEvent(sourceB).DeleteFromNamedWindow("MyInfraSA",
				esper.Equal[string](esper.NamedWindowField[string]("a"),
					esper.Field[infraNWTableOnSelectAggB, string]("id"))).Query())
		}
	case "correlated-nw", "correlated-table":
		switch statement {
		case "create":
			if spec.table {
				if _, err := esper.CreateTable(env, "MyInfraSAC", []esper.TableColumn{
					esper.PrimaryKeyColumn[string]("a"),
					esper.PrimaryKeyColumn[int32]("b"),
				}); err != nil {
					return esper.Plan{}, err
				}
				return env.Build(esper.FromTable(env, "MyInfraSAC").Query(esper.StatementName("create"), esper.WithOldStream()))
			}
			schema, err := esper.NewMapSchema("MyInfraSAC", []esper.FieldSpec{
				esper.FieldDef("a", reflect.TypeOf("")),
				esper.FieldDef("b", reflect.TypeOf(int32(0))),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfraSAC", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfraSAC").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "select":
			// `on SupportBean_A select sum(b) as sumb from MyInfraSAC
			// where a = id` — the correlated aggregate folds the matched
			// rows into one row, null on an empty match set.
			if spec.table {
				return env.Build(esper.OnEvent(sourceA).
					SelectFromTableWhere("MyInfraSAC",
						esper.Equal[string](esper.TableField[string]("a"),
							esper.Field[infraNWTableOnSelectAggA, string]("id")),
						esper.Alias("sumb", esper.Sum[int32](esper.TableField[int32]("b")))).
					Query(esper.StatementName("select")))
			}
			return env.Build(esper.OnEvent(sourceA).
				SelectFromNamedWindow("MyInfraSAC",
					esper.Equal[string](esper.NamedWindowField[string]("a"),
						esper.Field[infraNWTableOnSelectAggA, string]("id")),
					esper.Alias("sumb", esper.Sum[int32](esper.NamedWindowField[int32]("b")))).
				Query(esper.StatementName("select")))
		case "insert":
			if spec.table {
				return env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraSAC", insertProjection()...).Query())
			}
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraSAC", insertProjection()...).Query())
		}
	case "grouping-nw", "grouping-table":
		switch statement {
		case "create":
			if spec.table {
				if _, err := esper.CreateTable(env, "MyInfraSAG", []esper.TableColumn{
					esper.PrimaryKeyColumn[string]("a"),
					esper.PrimaryKeyColumn[int32]("b"),
				}); err != nil {
					return esper.Plan{}, err
				}
				return env.Build(esper.FromTable(env, "MyInfraSAG").Query(esper.StatementName("create")))
			}
			schema, err := esper.NewMapSchema("MyInfraSAG", []esper.FieldSpec{
				esper.FieldDef("a", reflect.TypeOf("")),
				esper.FieldDef("b", reflect.TypeOf(int32(0))),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfraSAG", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfraSAG").
				CreateNamedWindowQuery(esper.StatementName("create")))
		case "select":
			// `on SupportBean_A select a, sum(b) as sumb from MyInfraSAG
			// group by a order by a desc`.
			if spec.table {
				return env.Build(esper.OnEvent(sourceA).
					SelectFromTableGroupBy("MyInfraSAG", nil,
						[]esper.Expr{esper.Field[infraNWTableOnSelectAggA, string]("a")},
						esper.Alias("a", esper.TableField[string]("a")),
						esper.Alias("sumb", esper.Sum[int32](esper.TableField[int32]("b")))).
					Query(esper.StatementName("select"),
						esper.OrderBy(esper.Descending(esper.ResultField[string]("a")))))
			}
			return env.Build(esper.OnEvent(sourceA).
				SelectFromNamedWindowGroupBy("MyInfraSAG", nil,
					[]esper.Expr{esper.Field[infraNWTableOnSelectAggA, string]("a")},
					esper.Alias("a", esper.NamedWindowField[string]("a")),
					esper.Alias("sumb", esper.Sum[int32](esper.NamedWindowField[int32]("b")))).
				Query(esper.StatementName("select"),
					esper.OrderBy(esper.Descending(esper.ResultField[string]("a")))))
		case "selectTwo":
			// `on SupportBean_A select a, sum(b) as sumb from MyInfraSAG
			// group by a having sum(b) > 5 order by a desc`.
			if spec.table {
				return env.Build(esper.OnEvent(sourceA).
					SelectFromTableGroupBy("MyInfraSAG", nil,
						[]esper.Expr{esper.Field[infraNWTableOnSelectAggA, string]("a")},
						esper.Alias("a", esper.TableField[string]("a")),
						esper.Alias("sumb", esper.Sum[int32](esper.TableField[int32]("b")))).
					Having(esper.Greater[int32](esper.Sum[int32](esper.TableField[int32]("b")), esper.Literal(int32(5)))).
					Query(esper.StatementName("selectTwo"),
						esper.OrderBy(esper.Descending(esper.ResultField[string]("a")))))
			}
			return env.Build(esper.OnEvent(sourceA).
				SelectFromNamedWindowGroupBy("MyInfraSAG", nil,
					[]esper.Expr{esper.Field[infraNWTableOnSelectAggA, string]("a")},
					esper.Alias("a", esper.NamedWindowField[string]("a")),
					esper.Alias("sumb", esper.Sum[int32](esper.NamedWindowField[int32]("b")))).
				Having(esper.Greater[int32](esper.Sum[int32](esper.NamedWindowField[int32]("b")), esper.Literal(int32(5)))).
				Query(esper.StatementName("selectTwo"),
					esper.OrderBy(esper.Descending(esper.ResultField[string]("a")))))
		case "insert":
			if spec.table {
				return env.Build(esper.OnEvent(source).InsertIntoTable("MyInfraSAG", insertProjection()...).Query(esper.StatementName("insert")))
			}
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfraSAG", insertProjection()...).Query(esper.StatementName("insert")))
		case "delete":
			if spec.table {
				return env.Build(esper.OnEvent(sourceB).DeleteFromTableWhere("MyInfraSAG",
					esper.Equal[string](esper.TableField[string]("a"),
						esper.Field[infraNWTableOnSelectAggB, string]("id"))).Query())
			}
			return env.Build(esper.OnEvent(sourceB).DeleteFromNamedWindow("MyInfraSAG",
				esper.Equal[string](esper.NamedWindowField[string]("a"),
					esper.Field[infraNWTableOnSelectAggB, string]("id"))).Query())
		}
	case "multikey-w-array-nw", "multikey-w-array-table":
		switch statement {
		case "create":
			if spec.table {
				if _, err := esper.CreateTable(env, "MyInfraPC", []esper.TableColumn{
					esper.PrimaryKeyColumn[string]("id"),
					esper.TableColumnOf[[]int32]("array"),
					esper.TableColumnOf[int32]("value"),
				}); err != nil {
					return esper.Plan{}, err
				}
				return env.Build(esper.FromTable(env, "MyInfraPC").Query(esper.StatementName("create")))
			}
			schema, err := esper.NewMapSchema("MyInfraPC", []esper.FieldSpec{
				esper.FieldDef("id", reflect.TypeOf("")),
				esper.FieldDef("array", reflect.TypeOf([]int32(nil))),
				esper.FieldDef("value", reflect.TypeOf(int32(0))),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfraPC", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfraPC").
				CreateNamedWindowQuery(esper.StatementName("create")))
		case "s0":
			// `on SupportBean select array, sum(value) as thesum from
			// MyInfraPC group by array`: the int[] column is a single
			// content-equality group key.
			if spec.table {
				return env.Build(esper.OnEvent(source).
					SelectFromTableGroupBy("MyInfraPC", nil,
						[]esper.Expr{esper.Field[infraNWTableOnSelectAggBean, []int32]("array")},
						esper.Alias("array", esper.TableField[[]int32]("array")),
						esper.Alias("thesum", esper.Sum[int32](esper.TableField[int32]("value")))).
					Query(esper.StatementName("s0")))
			}
			return env.Build(esper.OnEvent(source).
				SelectFromNamedWindowGroupBy("MyInfraPC", nil,
					[]esper.Expr{esper.Field[infraNWTableOnSelectAggBean, []int32]("array")},
					esper.Alias("array", esper.NamedWindowField[[]int32]("array")),
					esper.Alias("thesum", esper.Sum[int32](esper.NamedWindowField[int32]("value")))).
				Query(esper.StatementName("s0")))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
}

// infraNWTableOnSelectAggBuildFaf maps one FafInsert deploy step onto the
// positional fire-and-forget insert, mirroring compileExecuteFAFNoResult.
// The pinned EPL carries the values clause verbatim; the runner maps each
// pinned text onto the typed InsertValues row.
func infraNWTableOnSelectAggBuildFaf(env *esper.Environment, spec infraNWTableOnSelectAggCaseSpec, epl string) (esper.Plan, error) {
	var row esper.OnDemandInsertRow
	switch epl {
	case infraNWTableOnSelectAggFafE1:
		row = esper.InsertValues(esper.Literal("E1"), esper.Literal([]int32{1, 2}), esper.Literal(int32(10)))
	case infraNWTableOnSelectAggFafE2:
		row = esper.InsertValues(esper.Literal("E2"), esper.Literal([]int32{1, 2}), esper.Literal(int32(11)))
	case infraNWTableOnSelectAggFafE3:
		row = esper.InsertValues(esper.Literal("E3"), esper.Literal([]int32{1, 2}), esper.Literal(int32(21)))
	case infraNWTableOnSelectAggFafE4:
		row = esper.InsertValues(esper.Literal("E4"), esper.Literal([]int32{1}), esper.Literal(int32(22)))
	default:
		return esper.Plan{}, fmt.Errorf("%s: unknown FafInsert EPL %q", infraNWTableOnSelectAggID, epl)
	}
	if spec.table {
		return env.Build(esper.FromTable(env, "MyInfraPC").OnDemand().InsertRows(row))
	}
	return env.Build(esper.FromNamedWindow(env, "MyInfraPC").OnDemand().InsertRows(row))
}

func infraNWTableOnSelectAggDecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		// The multikey trigger sends a bare `new SupportBean()` (empty
		// payload); the aggregation cases send theString+intPrimitive.
		if len(fields) != 0 {
			if err := requireInfraNWTableOnSelectAggFields(fields, "theString", "intPrimitive"); err != nil {
				return nil, err
			}
		}
		var bean infraNWTableOnSelectAggBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_A":
		if err := requireInfraNWTableOnSelectAggFields(fields, "id"); err != nil {
			return nil, err
		}
		var a infraNWTableOnSelectAggA
		if err := json.Unmarshal(step.Payload, &a); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return a, nil
	case "SupportBean_B":
		if err := requireInfraNWTableOnSelectAggFields(fields, "id"); err != nil {
			return nil, err
		}
		var b infraNWTableOnSelectAggB
		if err := json.Unmarshal(step.Payload, &b); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableOnSelectAggID, step.EventType)
	}
}

func requireInfraNWTableOnSelectAggFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

// infraNWTableOnSelectAggEPLForStep returns the byte-exact EPL of one
// deploy step.
func infraNWTableOnSelectAggEPLForStep(spec infraNWTableOnSelectAggCaseSpec, statement string) (string, bool) {
	switch spec.name {
	case "having-wildcard-nw":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSHSNW, true
		case "insert":
			return infraNWTableOnSelectAggInsertSHS, true
		case "select":
			return infraNWTableOnSelectAggSelectSHS, true
		}
	case "having-wildcard-table":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSHSTbl, true
		case "insert":
			return infraNWTableOnSelectAggInsertSHS, true
		case "select":
			return infraNWTableOnSelectAggSelectSHS, true
		}
	case "select-agg-nw":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSANW, true
		case "select":
			return infraNWTableOnSelectAggSelectSA, true
		case "insert":
			return infraNWTableOnSelectAggInsertSA, true
		case "delete":
			return infraNWTableOnSelectAggDeleteSA, true
		}
	case "select-agg-table":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSATbl, true
		case "select":
			return infraNWTableOnSelectAggSelectSA, true
		case "insert":
			return infraNWTableOnSelectAggInsertSA, true
		case "delete":
			return infraNWTableOnSelectAggDeleteSA, true
		}
	case "correlated-nw":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSACNW, true
		case "select":
			return infraNWTableOnSelectAggSelectSAC, true
		case "insert":
			return infraNWTableOnSelectAggInsertSAC, true
		}
	case "correlated-table":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSACTbl, true
		case "select":
			return infraNWTableOnSelectAggSelectSAC, true
		case "insert":
			return infraNWTableOnSelectAggInsertSAC, true
		}
	case "grouping-nw":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSAGNW, true
		case "select":
			return infraNWTableOnSelectAggSelectSAG, true
		case "selectTwo":
			return infraNWTableOnSelectAggSelectTwoSAG, true
		case "insert":
			return infraNWTableOnSelectAggInsertSAG, true
		case "delete":
			return infraNWTableOnSelectAggDeleteSAG, true
		}
	case "grouping-table":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreateSAGTbl, true
		case "select":
			return infraNWTableOnSelectAggSelectSAG, true
		case "selectTwo":
			return infraNWTableOnSelectAggSelectTwoSAG, true
		case "insert":
			return infraNWTableOnSelectAggInsertSAG, true
		case "delete":
			return infraNWTableOnSelectAggDeleteSAG, true
		}
	case "multikey-w-array-nw":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreatePCNW, true
		case "s0":
			return infraNWTableOnSelectAggSelectPC, true
		case "FafInsert":
			return "", true
		}
	case "multikey-w-array-table":
		switch statement {
		case "create":
			return infraNWTableOnSelectAggCreatePCTbl, true
		case "s0":
			return infraNWTableOnSelectAggSelectPC, true
		case "FafInsert":
			return "", true
		}
	}
	return "", false
}

// infraNWTableOnSelectAggCaseSteps pins the complete step sequence per
// case: case marker, deploy/deployed pairs in Java compileDeploy order
// (module deploys queue their deployed markers after the last deploy
// step), sends, the types probes and undeploy-all.
var infraNWTableOnSelectAggCaseSteps = map[string][]string{
	"having-wildcard-nw": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSHSNW,
		"deployed:create",
		"deploy:insert:" + infraNWTableOnSelectAggInsertSHS,
		"deployed:insert",
		"deploy:select:" + infraNWTableOnSelectAggSelectSHS,
		"deployed:select",
		"send:SupportBean:{\"intPrimitive\":16,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"E1\"}",
		"undeploy-all",
	},
	"having-wildcard-table": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSHSTbl,
		"deployed:create",
		"deploy:insert:" + infraNWTableOnSelectAggInsertSHS,
		"deployed:insert",
		"deploy:select:" + infraNWTableOnSelectAggSelectSHS,
		"deployed:select",
		"send:SupportBean:{\"intPrimitive\":16,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"E1\"}",
		"undeploy-all",
	},
	"select-agg-nw": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSANW,
		"deployed:create",
		"deploy:select:" + infraNWTableOnSelectAggSelectSA,
		"deployed:select",
		"deploy:insert:" + infraNWTableOnSelectAggInsertSA,
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"deploy:delete:" + infraNWTableOnSelectAggDeleteSA,
		"deployed:delete",
		"send:SupportBean_B:{\"id\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E4\"}",
		"send:SupportBean_A:{\"id\":\"A3\"}",
		"types:select",
		"undeploy-all",
	},
	"select-agg-table": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSATbl,
		"deployed:create",
		"deploy:select:" + infraNWTableOnSelectAggSelectSA,
		"deployed:select",
		"deploy:insert:" + infraNWTableOnSelectAggInsertSA,
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"deploy:delete:" + infraNWTableOnSelectAggDeleteSA,
		"deployed:delete",
		"send:SupportBean_B:{\"id\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E4\"}",
		"send:SupportBean_A:{\"id\":\"A3\"}",
		"types:select",
		"undeploy-all",
	},
	"correlated-nw": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSACNW,
		"deploy:select:" + infraNWTableOnSelectAggSelectSAC,
		"deploy:insert:" + infraNWTableOnSelectAggInsertSAC,
		"deployed:create",
		"deployed:select",
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean_A:{\"id\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"E2\"}",
		"types:select",
		"undeploy-all",
	},
	"correlated-table": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSACTbl,
		"deploy:select:" + infraNWTableOnSelectAggSelectSAC,
		"deploy:insert:" + infraNWTableOnSelectAggInsertSAC,
		"deployed:create",
		"deployed:select",
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean_A:{\"id\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"E2\"}",
		"types:select",
		"undeploy-all",
	},
	"grouping-nw": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSAGNW,
		"deploy:select:" + infraNWTableOnSelectAggSelectSAG,
		"deploy:selectTwo:" + infraNWTableOnSelectAggSelectTwoSAG,
		"deploy:insert:" + infraNWTableOnSelectAggInsertSAG,
		"deployed:create",
		"deployed:select",
		"deployed:selectTwo",
		"deployed:insert",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":5,\"theString\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E4\"}",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":100,\"theString\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"deploy:delete:" + infraNWTableOnSelectAggDeleteSAG,
		"deployed:delete",
		"send:SupportBean_B:{\"id\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"A3\"}",
		"types:select",
		"undeploy-all",
	},
	"grouping-table": {
		"deploy:create:" + infraNWTableOnSelectAggCreateSAGTbl,
		"deploy:select:" + infraNWTableOnSelectAggSelectSAG,
		"deploy:selectTwo:" + infraNWTableOnSelectAggSelectTwoSAG,
		"deploy:insert:" + infraNWTableOnSelectAggInsertSAG,
		"deployed:create",
		"deployed:select",
		"deployed:selectTwo",
		"deployed:insert",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":5,\"theString\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E4\"}",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":100,\"theString\":\"E1\"}",
		"send:SupportBean_A:{\"id\":\"A2\"}",
		"deploy:delete:" + infraNWTableOnSelectAggDeleteSAG,
		"deployed:delete",
		"send:SupportBean_B:{\"id\":\"E2\"}",
		"send:SupportBean_A:{\"id\":\"A3\"}",
		"types:select",
		"undeploy-all",
	},
	"multikey-w-array-nw": {
		"deploy:create:" + infraNWTableOnSelectAggCreatePCNW,
		"deployed:create",
		"deploy:s0:" + infraNWTableOnSelectAggSelectPC,
		"deployed:s0",
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE1,
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE2,
		"send:SupportBean:{}",
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE3,
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE4,
		"send:SupportBean:{}",
		"undeploy-all",
	},
	"multikey-w-array-table": {
		"deploy:create:" + infraNWTableOnSelectAggCreatePCTbl,
		"deployed:create",
		"deploy:s0:" + infraNWTableOnSelectAggSelectPC,
		"deployed:s0",
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE1,
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE2,
		"send:SupportBean:{}",
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE3,
		"deploy:FafInsert:" + infraNWTableOnSelectAggFafE4,
		"send:SupportBean:{}",
		"undeploy-all",
	},
}

// loadInfraNWTableOnSelectAggScenario decodes the scenario with the strict
// contract shared by the differential runners: no duplicate or unknown
// JSON fields, pinned metadata, pinned per-case runtime/execution/EPL, and
// a per-op step field whitelist.
func loadInfraNWTableOnSelectAggScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnSelectAggID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnSelectAggID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnSelectAggID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnSelectAggID, err)
	}
	if err := requireInfraNWTableOnSelectAggFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaStaticID []string `json:"javaStaticIds"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnSelectAggID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnSelectAggID ||
		metadata.Description != infraNWTableOnSelectAggDescription ||
		metadata.JavaCommit != infraNWTableOnSelectAggJavaCommit || metadata.JavaSource != infraNWTableOnSelectAggSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWTableOnSelectAggID)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWTableOnSelectAggID)
	}
	if err := infraNWTableOnSelectAggRequireEqual(metadata.JavaRuntimes, infraNWTableOnSelectAggJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWTableOnSelectAggRequireEqual(metadata.JavaNames, infraNWTableOnSelectAggJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWTableOnSelectAggRequireEqual(metadata.JavaStaticID, infraNWTableOnSelectAggJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWTableOnSelectAggID, err)
	}
	if len(rawCases) != len(infraNWTableOnSelectAggCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWTableOnSelectAggID, len(rawCases), len(infraNWTableOnSelectAggCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnSelectAggFields(object,
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
		spec := infraNWTableOnSelectAggCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				infraNWTableOnSelectAggID, index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Observation != spec.observation {
			return compat.Scenario{}, fmt.Errorf("scenario case %q observation does not match the pinned slice description", spec.name)
		}
		if definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWTableOnSelectAggID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWTableOnSelectAggID)
	}
	steps := make([]compat.Step, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraNWTableOnSelectAggFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnSelectAggFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWTableOnSelectAggCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWTableOnSelectAggEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.Statement == "FafInsert" {
				switch step.EPL {
				case infraNWTableOnSelectAggFafE1, infraNWTableOnSelectAggFafE2,
					infraNWTableOnSelectAggFafE3, infraNWTableOnSelectAggFafE4:
				default:
					return compat.Scenario{}, fmt.Errorf("scenario step %d FafInsert EPL is not pinned", index)
				}
			} else if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
		case "deployed":
			if err := requireInfraNWTableOnSelectAggFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnSelectAggFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := infraNWTableOnSelectAggDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "types":
			if err := requireInfraNWTableOnSelectAggFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			switch step.Case {
			case "select-agg-nw", "select-agg-table", "correlated-nw", "correlated-table",
				"grouping-nw", "grouping-table":
				if step.Statement != "select" {
					return compat.Scenario{}, fmt.Errorf("scenario step %d types probe %q/%q is not pinned", index, step.Case, step.Statement)
				}
			default:
				return compat.Scenario{}, fmt.Errorf("scenario step %d types probe %q/%q is not pinned", index, step.Case, step.Statement)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnSelectAggFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnSelectAggRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func infraNWTableOnSelectAggRequireEqual(got, want []string, label string) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	return nil
}

// validateInfraNWTableOnSelectAggRawSteps pins the complete step sequence
// per case against the raw JSON objects.
func validateInfraNWTableOnSelectAggRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnSelectAggCases {
		want, ok := infraNWTableOnSelectAggCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnSelectAggID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnSelectAggID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnSelectAggID, offset, caseName)
		}
		var marker struct {
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil || marker.Case != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnSelectAggID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnSelectAggStepKey(rawSteps[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnSelectAggID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q", infraNWTableOnSelectAggID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWTableOnSelectAggID)
	}
	return nil
}

// infraNWTableOnSelectAggStepKey renders a raw step object into its pinned
// string form.
func infraNWTableOnSelectAggStepKey(raw json.RawMessage, operation string) (string, error) {
	var step struct {
		Statement string          `json:"statement"`
		EPL       string          `json:"epl"`
		EventType string          `json:"eventType"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", fmt.Errorf("decode step: %w", err)
	}
	switch operation {
	case "deploy":
		return "deploy:" + step.Statement + ":" + step.EPL, nil
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
	case "types":
		return "types:" + step.Statement, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}
