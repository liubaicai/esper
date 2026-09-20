package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

// infra_table_select_enum_multikey.go replays InfraTableSelect ordinals 1-4
// against the pinned Java oracle: the "select from table" surface covering
// the enum firstOf() read and the array/composite multikey table joins.
//
// enum-firstof (ord 1, InfraTableSelectEnum) deploys one module — an unkeyed
// MyTable(p string) plus the 's0' select of t.firstOf() as c0 — attaches NO
// listener, seeds {'a'} through a fire-and-forget insert
// (compileExecuteFAFNoResult semantics), and asserts the 's0' iterator:
// next().get("c0") is the first table row's UNDERLYING Object[]{'a'}, not an
// EventBean.
//
// multikey-warray-single (ord 2) deploys one module — a MyTable keyed by the
// primitive int array column k, an insert-into fed by
// SupportEventWithIntArray.array, and the 's0' join of
// SupportEventWithManyArray.intOne to k — attaches the listener to 's0',
// sends E1/E2/E3 inserts, crosses the milestone(0) serde checkpoint (a
// harness no-op carrying no step), and probes [2], [1,3] and [1,2] for
// c0 = 30/20/10. Array primary-key equality is by content.
//
// multikey-warray-two (ord 3) deploys one module — a MyTable keyed by the
// two primitive int array columns k1/k2, an insert-into fed by
// SupportEventWithManyArray(id='I'), and the 's0' join of
// SupportEventWithManyArray(id='Q') on k1=intOne and k2=intTwo — attaches
// the listener to 's0', sends three 'I' inserts (including the empty-array
// key component []), crosses milestone(0), and probes ([2],[]),
// ([1,2],[3,4]) and ([1,3],[1]) for c0 = 30/10/20.
//
// multikey-warray-composite (ord 4) deploys one module — a MyTable keyed by
// three string columns k0/k1/k2 with a planning-only btree secondary index
// on (k0,k1,v), an insert-into fed by SupportBean_S0, and the 's0' join of
// SupportBean_S1 on k0=p10 and k1=p11 with the lexicographic range predicate
// v>p12 — attaches the listener to 's0', sends four S0 rows, crosses
// milestone(0), and probes ('A','CC',''), ('C','CC',''), ('A','BB','X3') and
// ('A','BB','Z'); the last leaves the listener uninvoked
// (assertListenerNotInvoked = absence of a listener record).
//
// Approved differences (observably identical to the Java EPL):
//   - `create table`/`create index` map to env-level registrations; the
//     'module' deploy label carries the insert-into and 's0' plans while the
//     deployed markers pin the deployment label and the named 's0'
//     statement (the inner-join-on precedent).
//   - Java's milestone(0) checkpoints are harness no-ops and carry no steps.
//   - Java's `insert into MyTable select ...` continuous inserts map to
//     OnEvent(...).InsertIntoTable with the same SetColumn assignments.
//   - ord 1's t.firstOf() maps to Func1("firstOf", Event.Underlying,
//     FirstEventValue()): the first table row's underlying. Go table rows
//     carry a map underlying where Java carries Object[]; the snapshot
//     normalizer renders the map positionally in declared column order so
//     both traces pin ["a"].

// infraTableSelectIntArray mirrors SupportEventWithIntArray for the ord-2
// insert stream; the bean property is literally named 'array'.
type infraTableSelectIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

// infraTableSelectManyArray mirrors SupportEventWithManyArray for the ord-2
// probe stream and the ord-3 'I'/'Q' streams.
type infraTableSelectManyArray struct {
	ID     string `esper:"id"`
	Value  int    `esper:"value"`
	IntOne []int  `esper:"intOne"`
	IntTwo []int  `esper:"intTwo"`
}

// infraTableSelectS0 mirrors SupportBean_S0 for the ord-4 insert stream.
type infraTableSelectS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P02 string `esper:"p02"`
	P03 string `esper:"p03"`
}

// infraTableSelectS1 mirrors SupportBean_S1 for the ord-4 probe stream.
type infraTableSelectS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
	P11 string `esper:"p11"`
	P12 string `esper:"p12"`
}

const (
	infraTableSelectEnumMultikeyID          = "infra-table-select-enum-multikey"
	infraTableSelectEnumMultikeyDescription = "InfraTableSelect ordinals 1-4: InfraTableSelectEnum deploys an unkeyed MyTable(p string) plus a 's0' select of t.firstOf() as c0, seeds {'a'} via fire-and-forget insert and observes the iterator row whose c0 is the first table row's Object[] underlying; InfraTableSelectMultikeyWArraySingleArray joins SupportEventWithManyArray.intOne to the int[primitive] primary key of a table filled from SupportEventWithIntArray; InfraTableSelectMultikeyWArrayTwoArray joins intOne/intTwo to a two-array-key table filled and probed through the same event type filtered on id 'I'/'Q'; InfraTableSelectMultikeyWArrayComposite joins SupportBean_S1 to a three-string-key table with a btree secondary index and a lexicographic v > p12 range predicate. Java milestone(0) checkpoints are harness no-ops and carry no steps (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSelect.java)."
	infraTableSelectEnumMultikeyJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableSelectEnumMultikeySource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSelect.java"

	// Verbatim transcriptions of InfraTableSelect lines 150-151 (ord 1),
	// 119-121 (ord 2), 88-90 (ord 3) and 50-53 (ord 4); each case deploys
	// its module in a single compileDeploy call.
	tsemEnumModule = "@public create table MyTable(p string);\n" +
		"@name('s0') select t.firstOf() as c0 from MyTable as t;\n"
	tsemEnumFafInsert = "insert into MyTable select 'a' as p"
	tsemSingleModule  = "@public create table MyTable(k int[primitive] primary key, value int);\n" +
		"insert into MyTable select array as k, value from SupportEventWithIntArray;\n" +
		"@name('s0') select t.value as c0 from SupportEventWithManyArray, MyTable as t where k = intOne;\n"
	tsemTwoModule = "@public create table MyTable(k1 int[primitive] primary key, k2 int[primitive] primary key, value int);\n" +
		"insert into MyTable select intOne as k1, intTwo as k2, value from SupportEventWithManyArray(id = 'I');\n" +
		"@name('s0') select t.value as c0 from SupportEventWithManyArray(id='Q'), MyTable as t where k1 = intOne and k2 = intTwo;\n"
	tsemCompositeModule = "@public create table MyTable(k0 string primary key, k1 string primary key, k2 string primary key, v string);\n" +
		"create index MyIndex on MyTable(k0, k1, v btree);\n" +
		"insert into MyTable select p00 as k0, p01 as k1, p02 as k2, p03 as v from SupportBean_S0;\n" +
		"@name('s0') select t.v as v from SupportBean_S1, MyTable as t where k0 = p10 and k1 = p11 and v > p12;\n"
)

var (
	infraTableSelectEnumMultikeyJavaSources = []string{
		infraTableSelectEnumMultikeySource,
	}
	infraTableSelectEnumMultikeyJavaRuntimeIDs = []string{
		"java-runtime-27e7ce929b90e4009b48",
		"java-runtime-d52d06b4618e30451543",
		"java-runtime-83451626aeee59b34ac2",
		"java-runtime-b8b3e8c1f04c0c918ea2",
	}
	infraTableSelectEnumMultikeyJavaExecutions = []string{
		"InfraTableSelectEnum",
		"InfraTableSelectMultikeyWArraySingleArray",
		"InfraTableSelectMultikeyWArrayTwoArray",
		"InfraTableSelectMultikeyWArrayComposite",
	}
	infraTableSelectEnumMultikeyJavaStaticIDs = []string{
		"java-ece967984c50b6e02d47",
		"java-2ae2dfa34214fa74ffc8",
		"java-79057cf89b8898584f05",
		"java-2a4767401dba7efd8f97",
	}
	infraTableSelectEnumMultikeyCases = []string{
		"enum-firstof",
		"multikey-warray-single",
		"multikey-warray-two",
		"multikey-warray-composite",
	}
	infraTableSelectEnumMultikeyOrdinals = []int{1, 2, 3, 4}
)

// infraTableSelectEnumMultikeyCaseEPLs pins the module EPL of each case,
// the value carried by the scenario cases[] metadata.
var infraTableSelectEnumMultikeyCaseEPLs = []string{
	tsemEnumModule,
	tsemSingleModule,
	tsemTwoModule,
	tsemCompositeModule,
}

var infraTableSelectEnumMultikeyCaseObservations = []string{
	"iterator; unkeyed MyTable(p string) seeded {'a'} by fire-and-forget insert; the s0 iterator row carries c0 = Object[]{'a'} — firstOf() yields the first table row's underlying, not an EventBean; no listener is attached",
	"listener; int[primitive] primary key k filled from SupportEventWithIntArray.array by content equality; SupportEventWithManyArray.intOne probes [2]/[1,3]/[1,2] yield c0 30/20/10",
	"listener; two int[primitive] primary keys k1/k2 filled from SupportEventWithManyArray(id='I') and probed by id='Q' sends; the empty array [] is a valid key component; probes ([2],[])/([1,2],[3,4])/([1,3],[1]) yield c0 30/10/20",
	"listener; three string primary keys k0/k1/k2 plus a planning-only btree index on (k0,k1,v); SupportBean_S1 probes join on k0=p10 and k1=p11 with lexicographic v>p12 — ('A','CC','') yields X3, ('C','CC','') yields X4, ('A','BB','X3') yields X4 because 'X1' is not > 'X3', and ('A','BB','Z') leaves the listener uninvoked",
}

// runInfraTableSelectEnumMultikeyScenario replays the four InfraTableSelect
// executions: the enum case records deployed markers and the ordered 's0'
// snapshot; the multikey cases record deployed markers and the 's0'
// listener batches, all in Java's observable order.
func runInfraTableSelectEnumMultikeyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableSelectEnumMultikeyCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraTableSelectEnumMultikeyCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraTableSelectEnumMultikeyID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraTableSelectEnumMultikeyID)
	}
	return trace, nil
}

func runInfraTableSelectEnumMultikeyCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableSelectIntArray](env, "SupportEventWithIntArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableSelectManyArray](env, "SupportEventWithManyArray"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableSelectS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraTableSelectS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableSelectEnumMultikeyJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraTableSelectEnumMultikeyID}
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		rec := compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		}
		if len(rec.Old) == 0 {
			rec.Old = nil
		}
		trace.Records = append(trace.Records, rec)
	}

	var deployments []*esper.Deployment
	deployedLabels := map[string]bool{}
	statements := map[string]*esper.Statement{}
	pinned := infraTableSelectEnumMultikeyCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if step.Statement == "FafInsert" {
				// Java executes the seed insert via compileExecuteFAFNoResult:
				// a fire-and-forget insert-into with no result rows.
				plan, err := env.Build(esper.FromTable(env, "MyTable").OnDemand().InsertRows(
					esper.InsertValues(esper.Literal("a")),
				))
				if err != nil {
					return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
				}
				if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
					return compat.Trace{}, fmt.Errorf("faf insert %q: %w", step.Statement, err)
				}
				continue
			}
			plans, err := infraTableSelectEnumMultikeyBuild(env, caseName, step.Statement, step.Epl)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployedLabels[step.Statement] = true
			for _, plan := range plans {
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
				}
				deployments = append(deployments, deployment)
				for _, statement := range deployment.Statements() {
					statements[statement.Name()] = statement
					// The multikey executions attach the listener to 's0'
					// (compileDeploy(...).addListener("s0")); the enum
					// execution attaches none.
					if statement.Name() == "s0" && caseName != "enum-firstof" {
						name := statement.Name()
						if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
							if len(batch.New) == 0 && len(batch.Old) == 0 {
								return nil
							}
							record(name, batch)
							return nil
						}); err != nil {
							return compat.Trace{}, err
						}
					}
				}
			}
		case "deployed":
			if !deployedLabels[step.Statement] {
				if _, ok := statements[step.Statement]; !ok {
					return compat.Trace{}, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
				}
			}
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      "1970-01-01T00:00:00Z",
			})
		case "send":
			payload, err := decodeInfraTableSelectEnumMultikeyPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return compat.Trace{}, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			rows = projectInfraNWTableOnMergeRows(rows, infraTableSelectEnumMultikeySnapshotFields(pinned, stepIndex))
			// ord 1's c0 is the first table row's underlying: Java carries
			// Object[] while Go carries a map, so the enum case renders the
			// map positionally in declared column order.
			rows = infraTableSelectEnumMultikeyNormalizeSnapshotRows(caseName, rows)
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      "1970-01-01T00:00:00Z",
				New:       rows,
			})
		case "undeploy-all":
			// The Java execution ends with undeployAll; the Go fixture's
			// registrations are env-scoped and retire with the engine, and
			// the trailing loop retires the deployments.
		default:
			return compat.Trace{}, fmt.Errorf("unexpected op %q", step.Op)
		}
	}
	for _, deployment := range deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, err
		}
	}
	return trace, nil
}

// infraTableSelectEnumMultikeyBuild mirrors the Java module deploys: the
// 'module' label registers the case's table (plus the ord-4 btree index) at
// env level and returns the insert-into and 's0' plans in declaration order.
func infraTableSelectEnumMultikeyBuild(env *esper.Environment, caseName string, label string, epl string) ([]esper.Plan, error) {
	if label != "module" {
		return nil, fmt.Errorf("%s: unexpected deploy statement %q for case %q",
			infraTableSelectEnumMultikeyID, label, caseName)
	}
	caseIndex := indexOfInfraTableSelectEnumMultikeyCase(caseName)
	if caseIndex < 0 {
		return nil, fmt.Errorf("%s: unexpected case %q", infraTableSelectEnumMultikeyID, caseName)
	}
	if epl != infraTableSelectEnumMultikeyCaseEPLs[caseIndex] {
		return nil, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
			infraTableSelectEnumMultikeyID, label, epl)
	}
	switch caseName {
	case "enum-firstof":
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			esper.TableColumnOf[string]("p"),
		}); err != nil {
			return nil, err
		}
		// select t.firstOf() as c0 from MyTable as t: firstOf() yields the
		// first table row's underlying (Java Object[]), not an EventBean.
		s0Plan, err := env.Build(esper.FromTable(env, "MyTable").Select(
			esper.Alias("c0", esper.Func1[esper.Event, any]("firstOf",
				esper.Event.Underlying, esper.FirstEventValue())),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{s0Plan}, nil
	case "multikey-warray-single":
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[[]int]("k"),
			esper.TableColumnOf[int]("value"),
		}); err != nil {
			return nil, err
		}
		intArray := esper.From[infraTableSelectIntArray](env, "SupportEventWithIntArray")
		insertPlan, err := env.Build(esper.OnEvent(intArray).InsertIntoTable("MyTable",
			esper.SetColumn("k", esper.Field[infraTableSelectIntArray, []int]("array")),
			esper.SetColumn("value", esper.Field[infraTableSelectIntArray, int]("value")),
		).Query())
		if err != nil {
			return nil, err
		}
		manyArray := esper.From[infraTableSelectManyArray](env, "SupportEventWithManyArray")
		s0Plan, err := env.Build(esper.JoinMany(
			esper.JoinSource(manyArray),
			esper.JoinRecordSource(esper.FromTable(env, "MyTable")),
		).On(
			esper.OnSourcesEqual(1, esper.JoinField[[]int](1, "k"),
				0, esper.JoinField[[]int](0, "intOne")),
		).Select(
			esper.SelectFrom(1, "c0", esper.JoinField[int](1, "value")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil
	case "multikey-warray-two":
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[[]int]("k1"),
			esper.PrimaryKeyColumn[[]int]("k2"),
			esper.TableColumnOf[int]("value"),
		}); err != nil {
			return nil, err
		}
		manyArrayInsert := esper.From[infraTableSelectManyArray](env, "SupportEventWithManyArray").
			Filter(esper.Equal[string](
				esper.Field[infraTableSelectManyArray, string]("id"), esper.Literal("I")))
		insertPlan, err := env.Build(esper.OnEvent(manyArrayInsert).InsertIntoTable("MyTable",
			esper.SetColumn("k1", esper.Field[infraTableSelectManyArray, []int]("intOne")),
			esper.SetColumn("k2", esper.Field[infraTableSelectManyArray, []int]("intTwo")),
			esper.SetColumn("value", esper.Field[infraTableSelectManyArray, int]("value")),
		).Query())
		if err != nil {
			return nil, err
		}
		manyArrayQuery := esper.From[infraTableSelectManyArray](env, "SupportEventWithManyArray").
			Filter(esper.Equal[string](
				esper.Field[infraTableSelectManyArray, string]("id"), esper.Literal("Q")))
		s0Plan, err := env.Build(esper.JoinMany(
			esper.JoinSource(manyArrayQuery),
			esper.JoinRecordSource(esper.FromTable(env, "MyTable")),
		).On(
			esper.OnSourcesEqual(1, esper.JoinField[[]int](1, "k1"),
				0, esper.JoinField[[]int](0, "intOne")),
			esper.OnSourcesEqual(1, esper.JoinField[[]int](1, "k2"),
				0, esper.JoinField[[]int](0, "intTwo")),
		).Select(
			esper.SelectFrom(1, "c0", esper.JoinField[int](1, "value")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil
	case "multikey-warray-composite":
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("k0"),
			esper.PrimaryKeyColumn[string]("k1"),
			esper.PrimaryKeyColumn[string]("k2"),
			esper.TableColumnOf[string]("v"),
		}, esper.SecondaryBTreeIndex("MyIndex", "k0", "k1", "v")); err != nil {
			return nil, err
		}
		s0Insert := esper.From[infraTableSelectS0](env, "SupportBean_S0")
		insertPlan, err := env.Build(esper.OnEvent(s0Insert).InsertIntoTable("MyTable",
			esper.SetColumn("k0", esper.Field[infraTableSelectS0, string]("p00")),
			esper.SetColumn("k1", esper.Field[infraTableSelectS0, string]("p01")),
			esper.SetColumn("k2", esper.Field[infraTableSelectS0, string]("p02")),
			esper.SetColumn("v", esper.Field[infraTableSelectS0, string]("p03")),
		).Query())
		if err != nil {
			return nil, err
		}
		s1 := esper.From[infraTableSelectS1](env, "SupportBean_S1")
		s0Plan, err := env.Build(esper.JoinMany(
			esper.JoinSource(s1),
			esper.JoinRecordSource(esper.FromTable(env, "MyTable")),
		).On(
			esper.OnSourcesEqual(1, esper.JoinField[string](1, "k0"),
				0, esper.JoinField[string](0, "p10")),
			esper.OnSourcesEqual(1, esper.JoinField[string](1, "k1"),
				0, esper.JoinField[string](0, "p11")),
			esper.OnSourcesCompare(1, esper.JoinField[string](1, "v"),
				0, esper.JoinField[string](0, "p12"), esper.JoinGreater),
		).Select(
			esper.SelectFrom(1, "v", esper.JoinField[string](1, "v")),
		).Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil
	}
	return nil, fmt.Errorf("%s: unexpected case %q", infraTableSelectEnumMultikeyID, caseName)
}

func indexOfInfraTableSelectEnumMultikeyCase(caseName string) int {
	for index, name := range infraTableSelectEnumMultikeyCases {
		if name == caseName {
			return index
		}
	}
	return -1
}

// infraTableSelectEnumMultikeyNormalizeSnapshotRows renders the enum case's
// c0 map (the Go table row underlying) as the positional array Java's
// Object[] underlying produces, in declared column order.
func infraTableSelectEnumMultikeyNormalizeSnapshotRows(caseName string, rows []compat.ResultRecord) []compat.ResultRecord {
	if caseName != "enum-firstof" {
		return rows
	}
	for index := range rows {
		value, ok := rows[index].Fields["c0"]
		if !ok {
			continue
		}
		if fields, isMap := value.(map[string]any); isMap {
			// MyTable declares the single column p; the positional Object[]
			// rendering reads the declared column order.
			rows[index].Fields["c0"] = []any{fields["p"]}
		}
	}
	return rows
}

// infraTableSelectEnumMultikeySnapshotFields extracts the pinned projection
// list for the snapshot at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>"). The case marker occupies
// steps[0], so the pinned index is stepIndex-1.
func infraTableSelectEnumMultikeySnapshotFields(pinned []string, stepIndex int) []string {
	if stepIndex < 1 || stepIndex-1 >= len(pinned) {
		return nil
	}
	key := pinned[stepIndex-1]
	if !strings.HasPrefix(key, "snapshot:") {
		return nil
	}
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 || parts[3] == "" {
		return nil
	}
	return strings.Split(parts[3], ",")
}

func decodeInfraTableSelectEnumMultikeyPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportEventWithIntArray":
		var payload struct {
			ID    string `json:"id"`
			Array []int  `json:"array"`
			Value int    `json:"value"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithIntArray payload: %w", err)
		}
		return infraTableSelectIntArray{ID: payload.ID, Array: payload.Array, Value: payload.Value}, nil
	case "SupportEventWithManyArray":
		var payload struct {
			ID     *string `json:"id"`
			Value  int     `json:"value"`
			IntOne []int   `json:"intOne"`
			IntTwo []int   `json:"intTwo"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportEventWithManyArray payload: %w", err)
		}
		event := infraTableSelectManyArray{Value: payload.Value, IntOne: payload.IntOne, IntTwo: payload.IntTwo}
		if payload.ID != nil {
			event.ID = *payload.ID
		}
		return event, nil
	case "SupportBean_S0":
		var payload struct {
			ID  int    `json:"id"`
			P00 string `json:"p00"`
			P01 string `json:"p01"`
			P02 string `json:"p02"`
			P03 string `json:"p03"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0 payload: %w", err)
		}
		return infraTableSelectS0{ID: payload.ID, P00: payload.P00, P01: payload.P01, P02: payload.P02, P03: payload.P03}, nil
	case "SupportBean_S1":
		var payload struct {
			ID  int    `json:"id"`
			P10 string `json:"p10"`
			P11 string `json:"p11"`
			P12 string `json:"p12"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1 payload: %w", err)
		}
		return infraTableSelectS1{ID: payload.ID, P10: payload.P10, P11: payload.P11, P12: payload.P12}, nil
	}
	return nil, fmt.Errorf("unexpected event type %q", step.EventType)
}

// loadInfraTableSelectEnumMultikeyScenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist followed by a full step-shape pin.
func loadInfraTableSelectEnumMultikeyScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableSelectEnumMultikeyID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableSelectEnumMultikeyID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableSelectEnumMultikeyID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableSelectEnumMultikeyID, err)
	}
	if err := requireInfraTableSelectEnumMultikeyFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableSelectEnumMultikeyID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableSelectEnumMultikeyID ||
		metadata.Description != infraTableSelectEnumMultikeyDescription ||
		metadata.JavaCommit != infraTableSelectEnumMultikeyJavaCommit ||
		metadata.JavaSource != infraTableSelectEnumMultikeySource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableSelectEnumMultikeyID)
	}
	if err := validateInfraTableSelectEnumMultikeyStringArray(root["javaRuntimes"], infraTableSelectEnumMultikeyJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSelectEnumMultikeyStringArray(root["javaNames"], infraTableSelectEnumMultikeyJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSelectEnumMultikeyStringArray(root["javaStaticIds"], infraTableSelectEnumMultikeyJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSelectEnumMultikeyStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableSelectEnumMultikeyCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableSelectEnumMultikeyID, len(infraTableSelectEnumMultikeyCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableSelectEnumMultikeyFields(object,
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
		if definition.Case != infraTableSelectEnumMultikeyCases[index] ||
			definition.Ordinal != infraTableSelectEnumMultikeyOrdinals[index] ||
			definition.RuntimeID != infraTableSelectEnumMultikeyJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableSelectEnumMultikeyJavaExecutions[index] ||
			definition.Observation != infraTableSelectEnumMultikeyCaseObservations[index] ||
			definition.EPL != infraTableSelectEnumMultikeyCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableSelectEnumMultikeyID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraTableSelectEnumMultikeyID)
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
			if err := requireInfraTableSelectEnumMultikeyFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraTableSelectEnumMultikeyFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraTableSelectEnumMultikeyFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraTableSelectEnumMultikeyFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraTableSelectEnumMultikeyPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraTableSelectEnumMultikeyFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraTableSelectEnumMultikeyFields(object, "op", "case"); err != nil {
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
		for _, name := range infraTableSelectEnumMultikeyCases {
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
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableSelectEnumMultikeyRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraTableSelectEnumMultikeyRawSteps pins the complete step
// sequence per case against the raw JSON objects: the module deploy with
// byte-exact EPL, deployed markers, send event types with canonical
// payloads, the ord-1 snapshot read, and undeploy-all terminators.
func validateInfraTableSelectEnumMultikeyRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraTableSelectEnumMultikeyCases {
		want, ok := infraTableSelectEnumMultikeyCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraTableSelectEnumMultikeyID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraTableSelectEnumMultikeyID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraTableSelectEnumMultikeyID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraTableSelectEnumMultikeyID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraTableSelectEnumMultikeyStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraTableSelectEnumMultikeyID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraTableSelectEnumMultikeyID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraTableSelectEnumMultikeyID)
	}
	return nil
}

// infraTableSelectEnumMultikeyStepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because compat.Step
// does not carry the fields array.
func infraTableSelectEnumMultikeyStepKey(object map[string]json.RawMessage, operation string) (string, error) {
	stringField := func(name string) (string, error) {
		var value string
		if err := json.Unmarshal(object[name], &value); err != nil {
			return "", fmt.Errorf("step field %q must be a string", name)
		}
		return value, nil
	}
	fieldsList := func() (string, error) {
		var values []string
		if err := json.Unmarshal(object["fields"], &values); err != nil {
			return "", fmt.Errorf("step fields must be a string array")
		}
		return joinStrings(values, ","), nil
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
		return "deployed:" + statement, nil
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
	case "snapshot":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		mode, err := stringField("mode")
		if err != nil {
			return "", err
		}
		fields, err := fieldsList()
		if err != nil {
			return "", err
		}
		return "snapshot:" + statement + ":" + mode + ":" + fields, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// tsemEnumCaseSteps renders the pinned step sequence of
// InfraTableSelectEnum.run (lines 148-162): the module deploy with deployed
// markers for the deployment label and the named 's0' statement, the
// fire-and-forget seed insert (no marker), the ordered c0 snapshot
// mirroring assertIterator, and undeploy-all.
func tsemEnumCaseSteps() []string {
	return []string{
		"deploy:module:" + tsemEnumModule,
		"deployed:module",
		"deployed:s0",
		"deploy:FafInsert:" + tsemEnumFafInsert,
		"snapshot:s0:ordered:c0",
		"undeploy-all",
	}
}

// tsemSingleCaseSteps renders the pinned step sequence of
// InfraTableSelectMultikeyWArraySingleArray.run (lines 117-135): the module
// deploy with deployed markers, the three SupportEventWithIntArray inserts,
// the milestone(0) no-op (no step), the three SupportEventWithManyArray
// probes and undeploy-all.
func tsemSingleCaseSteps() []string {
	return []string{
		"deploy:module:" + tsemSingleModule,
		"deployed:module",
		"deployed:s0",
		`send:SupportEventWithIntArray:{"array":[1,2],"id":"E1","value":10}`,
		`send:SupportEventWithIntArray:{"array":[1,3],"id":"E2","value":20}`,
		`send:SupportEventWithIntArray:{"array":[2],"id":"E3","value":30}`,
		`send:SupportEventWithManyArray:{"intOne":[2]}`,
		`send:SupportEventWithManyArray:{"intOne":[1,3]}`,
		`send:SupportEventWithManyArray:{"intOne":[1,2]}`,
		"undeploy-all",
	}
}

// tsemTwoCaseSteps renders the pinned step sequence of
// InfraTableSelectMultikeyWArrayTwoArray.run (lines 86-104): the module
// deploy with deployed markers, the three id='I' inserts, the milestone(0)
// no-op (no step), the three id='Q' probes sent with value -1 by
// sendManyArrayAssert, and undeploy-all.
func tsemTwoCaseSteps() []string {
	return []string{
		"deploy:module:" + tsemTwoModule,
		"deployed:module",
		"deployed:s0",
		`send:SupportEventWithManyArray:{"id":"I","intOne":[1,2],"intTwo":[3,4],"value":10}`,
		`send:SupportEventWithManyArray:{"id":"I","intOne":[1,3],"intTwo":[1],"value":20}`,
		`send:SupportEventWithManyArray:{"id":"I","intOne":[2],"intTwo":[],"value":30}`,
		`send:SupportEventWithManyArray:{"id":"Q","intOne":[2],"intTwo":[],"value":-1}`,
		`send:SupportEventWithManyArray:{"id":"Q","intOne":[1,2],"intTwo":[3,4],"value":-1}`,
		`send:SupportEventWithManyArray:{"id":"Q","intOne":[1,3],"intTwo":[1],"value":-1}`,
		"undeploy-all",
	}
}

// tsemCompositeCaseSteps renders the pinned step sequence of
// InfraTableSelectMultikeyWArrayComposite.run (lines 48-69): the module
// deploy with deployed markers, the four SupportBean_S0 inserts, the
// milestone(0) no-op (no step), the four SupportBean_S1 probes (the last is
// assertListenerNotInvoked), and undeploy-all.
func tsemCompositeCaseSteps() []string {
	return []string{
		"deploy:module:" + tsemCompositeModule,
		"deployed:module",
		"deployed:s0",
		`send:SupportBean_S0:{"id":0,"p00":"A","p01":"BB","p02":"CCC","p03":"X1"}`,
		`send:SupportBean_S0:{"id":0,"p00":"A","p01":"BB","p02":"DDDD","p03":"X4"}`,
		`send:SupportBean_S0:{"id":0,"p00":"A","p01":"CC","p02":"CCC","p03":"X3"}`,
		`send:SupportBean_S0:{"id":0,"p00":"C","p01":"CC","p02":"CCC","p03":"X4"}`,
		`send:SupportBean_S1:{"id":0,"p10":"A","p11":"CC","p12":""}`,
		`send:SupportBean_S1:{"id":0,"p10":"C","p11":"CC","p12":""}`,
		`send:SupportBean_S1:{"id":0,"p10":"A","p11":"BB","p12":"X3"}`,
		`send:SupportBean_S1:{"id":0,"p10":"A","p11":"BB","p12":"Z"}`,
		"undeploy-all",
	}
}

// infraTableSelectEnumMultikeyCaseSteps pins the exact op sequence per
// case: the module deploy with byte-exact EPL, deployed markers, send event
// types with canonical payloads, the ord-1 snapshot read, and undeploy-all
// terminators.
var infraTableSelectEnumMultikeyCaseSteps = map[string][]string{
	"enum-firstof":              tsemEnumCaseSteps(),
	"multikey-warray-single":    tsemSingleCaseSteps(),
	"multikey-warray-two":       tsemTwoCaseSteps(),
	"multikey-warray-composite": tsemCompositeCaseSteps(),
}

func requireInfraTableSelectEnumMultikeyFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraTableSelectEnumMultikeyID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraTableSelectEnumMultikeyID, name)
		}
	}
	return nil
}

func validateInfraTableSelectEnumMultikeyStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// infraTableSelectEnumMultikeyRuntimeID maps each scenario case to the
// inventory runtime ID of the Java execution it replays.
func infraTableSelectEnumMultikeyRuntimeID(caseName string) string {
	for index, name := range infraTableSelectEnumMultikeyCases {
		if name == caseName {
			return infraTableSelectEnumMultikeyJavaRuntimeIDs[index]
		}
	}
	return ""
}
