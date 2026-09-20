package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNWTableOnMergeMultiactionID          = "infra-nwtable-on-merge-multiaction"
	infraNWTableOnMergeMultiactionDescription = "InfraNWTableOnMerge ordinals 20-25: InfraMultiactionDeleteUpdate runs six ordered matched actions over WinMDU where later action where-clauses observe earlier action results (E5/E6 survive their trailing delete clauses); InfraUpdateOrderOfFields evaluates update-set assignments left-to-right so intBoxed reads the already-updated intPrimitive while initial.intPrimitive reads the pre-update value; InfraSubqueryNotMatched evaluates a correlated subquery over InfraTwo in the not-matched insert assignment; each execution runs over a named window and a primary-key table (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeMultiactionJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeMultiactionSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	// Verbatim transcriptions of InfraNWTableOnMerge lines 1096-1109
	// (InfraMultiactionDeleteUpdate), 1210-1216 (InfraUpdateOrderOfFields)
	// and 1165-1179 (InfraSubqueryNotMatched).
	infraNWTableOnMergeMultiactionCreateNW  = "@name('Create') @public create window WinMDU#keepall as SupportBean"
	infraNWTableOnMergeMultiactionCreateTbl = "@name('Create') @public create table WinMDU (theString string primary key, intPrimitive int)"
	infraNWTableOnMergeMultiactionInsert    = "insert into WinMDU select theString, intPrimitive from SupportBean"
	infraNWTableOnMergeMultiactionMerge     = "@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where st0.key0=win.theString when matched then delete where intPrimitive<0 then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 then update set intPrimitive=999 where intPrimitive=1000 then delete where intPrimitive=1000 then update set intPrimitive=1999 where intPrimitive=2000 then delete where intPrimitive=2000"

	infraNWTableOnMergeMultiactionUOFTail = "insert into MyInfraUOF select theString, intPrimitive, intBoxed, doublePrimitive from SupportBean;\n" +
		"@name('Merge') on SupportBean_S0 as sb merge MyInfraUOF as mywin where mywin.theString = sb.p00 when matched then update set intPrimitive=id, intBoxed=mywin.intPrimitive, doublePrimitive=initial.intPrimitive;\n"

	infraNWTableOnMergeMultiactionCreateOneNW  = "@name('Create') @public create window InfraOne#unique(string) (string string, intPrimitive int)"
	infraNWTableOnMergeMultiactionCreateOneTbl = "@name('Create') @public create table InfraOne (string string primary key, intPrimitive int)"
	infraNWTableOnMergeMultiactionCreateTwoNW  = "@public create window InfraTwo#unique(val0) (val0 string, val1 int)"
	infraNWTableOnMergeMultiactionCreateTwoTbl = "@public create table InfraTwo (val0 string primary key, val1 int primary key)"
	infraNWTableOnMergeMultiactionInsertTwo    = "insert into InfraTwo select 'W2' as val0, id as val1 from SupportBean_S0"
	infraNWTableOnMergeMultiactionMergeSubq    = "on SupportBean sb merge InfraOne w1 where sb.theString = w1.string when not matched then insert select 'Y' as string, (select val1 from InfraTwo as w2 where w2.val0 = sb.theString) as intPrimitive"
)

var (
	infraNWTableOnMergeMultiactionJavaSources = []string{
		infraNWTableOnMergeMultiactionSource,
	}
	infraNWTableOnMergeMultiactionJavaRuntimeIDs = []string{
		"java-runtime-dfa83c0593d19172be7d",
		"java-runtime-ffbda0563d50878dafd8",
		"java-runtime-3034e5da517c1235da22",
		"java-runtime-cb5eefe84a486b090e53",
		"java-runtime-905164d98662721e5509",
		"java-runtime-d3e1f3aa50c4375f657f",
	}
	infraNWTableOnMergeMultiactionJavaExecutions = []string{
		"InfraMultiactionDeleteUpdate{namedWindow=true}",
		"InfraMultiactionDeleteUpdate{namedWindow=false}",
		"InfraUpdateOrderOfFields{namedWindow=true}",
		"InfraUpdateOrderOfFields{namedWindow=false}",
		"InfraSubqueryNotMatched{namedWindow=true}",
		"InfraSubqueryNotMatched{namedWindow=false}",
	}
	infraNWTableOnMergeMultiactionJavaStaticIDs = []string{
		"java-d718ed89dce189e3cb3b",
		"java-d718ed89dce189e3cb3b",
		"java-71292c39e6d9067bb77d",
		"java-71292c39e6d9067bb77d",
		"java-1b30fec400708094c9a1",
		"java-1b30fec400708094c9a1",
	}
	infraNWTableOnMergeMultiactionCases = []string{
		"multiaction-nw",
		"multiaction-table",
		"orderoffields-nw",
		"orderoffields-table",
		"subquery-nw",
		"subquery-table",
	}
	infraNWTableOnMergeMultiactionOrdinals = []int{20, 21, 22, 23, 24, 25}
)

// infraNWTableOnMergeMultiactionS0 mirrors the regression SupportBean_S0
// properties the executions read: id feeds the InfraTwo insert and the
// order-of-fields update assignment, p00 carries the merge match key.
type infraNWTableOnMergeMultiactionS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func infraNWTableOnMergeMultiactionIsTable(caseName string) bool {
	return strings.HasSuffix(caseName, "-table")
}

// infraNWTableOnMergeMultiactionUOFModuleEPL renders the verbatim
// three-statement module of InfraUpdateOrderOfFields (lines 1210-1216).
func infraNWTableOnMergeMultiactionUOFModuleEPL(isTable bool) string {
	if isTable {
		return "@public create table MyInfraUOF(theString string primary key, intPrimitive int, intBoxed int, doublePrimitive double);\n" +
			infraNWTableOnMergeMultiactionUOFTail
	}
	return "@public create window MyInfraUOF#keepall as SupportBean;\n" +
		infraNWTableOnMergeMultiactionUOFTail
}

// runInfraNWTableOnMergeMultiactionScenario replays the six
// InfraNWTableOnMerge executions: each case deploys the pinned statements,
// sends the pinned events, and records deployed markers, 'Merge' listener
// batches and 'Create' iterator snapshots in Java's observable order.
func runInfraNWTableOnMergeMultiactionScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeMultiactionCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeMultiactionCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeMultiactionID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeMultiactionID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeMultiactionCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean",
		// The Java bean leaves charPrimitive at the zero char default;
		// partial insert-into projections materialize the remaining
		// properties from the schema defaults.
		esper.WithJSONDefaults(map[string]any{"charPrimitive": "\u0000"})); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnMergeNestedST0](env, "SupportBean_ST0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnMergeMultiactionS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnMergeMultiactionIsTable(caseName)
	if err := infraNWTableOnMergeMultiactionCreateInfra(env, caseName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeMultiactionJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeMultiactionID}
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

	statements := map[string]*esper.Statement{}
	labelDeployments := map[string]*esper.Deployment{}
	pinned := infraNWTableOnMergeMultiactionCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plans, err := infraNWTableOnMergeMultiactionBuild(env, caseName, step.Statement, isTable)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			for _, bound := range plans {
				deployment, err := engine.Deploy(ctx, bound.plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", bound.label, err)
				}
				labelDeployments[bound.label] = deployment
				deploymentStatements := deployment.Statements()
				for _, statement := range deploymentStatements {
					if statement.Name() == bound.label {
						statements[bound.label] = statement
					}
				}
				if _, ok := statements[bound.label]; !ok && len(deploymentStatements) == 1 {
					// The create consumers, insert feeders and the subquery
					// merge are unnamed in Java; bind the deployment's
					// single statement to the step label.
					statements[bound.label] = deploymentStatements[0]
				}
				for _, statement := range deploymentStatements {
					if statement.Name() == "Merge" {
						if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
							record("Merge", batch)
							return nil
						}); err != nil {
							return compat.Trace{}, err
						}
					}
				}
			}
		case "deployed":
			if _, ok := statements[step.Statement]; !ok {
				return compat.Trace{}, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
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
			payload, err := decodeInfraNWTableOnMergeMultiactionPayload(step)
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
			rows = projectInfraNWTableOnMergeMultiactionRows(rows, infraNWTableOnMergeMultiactionSnapshotFields(pinned, stepIndex))
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  0,
				Time:      "1970-01-01T00:00:00Z",
				New:       rows,
			})
		case "undeploy":
			// Mirrors env.undeployModuleContaining: the label resolves the
			// deployment that owns the statement.
			deployment, ok := labelDeployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("no deployment for statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, err
			}
			delete(labelDeployments, step.Statement)
			delete(statements, step.Statement)
		case "undeploy-all":
		default:
			return compat.Trace{}, fmt.Errorf("unexpected op %q", step.Op)
		}
	}
	for label, deployment := range labelDeployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, fmt.Errorf("undeploy %q: %w", label, err)
		}
	}
	return trace, nil
}

// infraNWTableOnMergeMultiactionCreateInfra registers the environment-level
// artifacts the Java @public create statements establish: the WinMDU keepall
// window or primary-key table, the MyInfraUOF keepall window or four-column
// table, or the InfraOne/InfraTwo window or table pair.
func infraNWTableOnMergeMultiactionCreateInfra(env *esper.Environment, caseName string, isTable bool) error {
	switch caseName {
	case "multiaction-nw", "multiaction-table":
		if isTable {
			_, err := esper.CreateTable(env, "WinMDU", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("theString"),
				esper.TableColumnOf[int64]("intPrimitive"),
			})
			return err
		}
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("SupportBean schema is missing")
		}
		return createKeepAllNamedWindow(env, "WinMDU", schema)
	case "orderoffields-nw", "orderoffields-table":
		if isTable {
			// intBoxed is a nullable Java int column: Optional marks the
			// column nullable while the stored value stays int64.
			_, err := esper.CreateTable(env, "MyInfraUOF", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("theString"),
				esper.TableColumnOf[int64]("intPrimitive"),
				esper.OptionalTableColumnOf[int64]("intBoxed"),
				esper.TableColumnOf[float64]("doublePrimitive"),
			})
			return err
		}
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("SupportBean schema is missing")
		}
		return createKeepAllNamedWindow(env, "MyInfraUOF", schema)
	case "subquery-nw", "subquery-table":
		if isTable {
			if _, err := esper.CreateTable(env, "InfraOne", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("string"),
				esper.TableColumnOf[int64]("intPrimitive"),
			}); err != nil {
				return err
			}
			_, err := esper.CreateTable(env, "InfraTwo", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("val0"),
				esper.PrimaryKeyColumn[int64]("val1"),
			})
			return err
		}
		oneSchema, err := esper.RegisterMap(env, "InfraOneSchema", []esper.FieldSpec{
			esper.FieldDef("string", reflect.TypeOf("")),
			esper.FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		if _, err := esper.CreateNamedWindow(env, "InfraOne", oneSchema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("string")))); err != nil {
			return err
		}
		twoSchema, err := esper.RegisterMap(env, "InfraTwoSchema", []esper.FieldSpec{
			esper.FieldDef("val0", reflect.TypeOf("")),
			esper.FieldDef("val1", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		_, err = esper.CreateNamedWindow(env, "InfraTwo", twoSchema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("val0"))))
		return err
	}
	return fmt.Errorf("unexpected case %q", caseName)
}

// infraNWTableOnMergeMultiactionBoundPlan pairs one statement label with
// the Go plan that produces the deployed statement.
type infraNWTableOnMergeMultiactionBoundPlan struct {
	label string
	plan  esper.Plan
}

// infraNWTableOnMergeMultiactionBuild mirrors the Java deploys: the @public
// create statements are environment-level on the Go side, so each create
// label deploys the consumer query the Java iterator reads, and the insert
// feeders and merge statements deploy as their own plans. The
// orderoffields "module" label deploys all three plans in module order,
// mirroring the single compileDeploy.
func infraNWTableOnMergeMultiactionBuild(env *esper.Environment, caseName string, label string, isTable bool) ([]infraNWTableOnMergeMultiactionBoundPlan, error) {
	sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
	theString := esper.Field[infraNWTableOnMergeBean, string]("theString")
	intPrimitive := esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")
	// intBoxed is a nullable *int64 bean property; the schema dereferences
	// pointer fields at runtime, so the expression type is the yielded int64.
	intBoxed := esper.Field[infraNWTableOnMergeBean, int64]("intBoxed")
	doublePrimitive := esper.Field[infraNWTableOnMergeBean, float64]("doublePrimitive")
	sourceST0 := esper.From[infraNWTableOnMergeNestedST0](env, "SupportBean_ST0")
	key0 := esper.Field[infraNWTableOnMergeNestedST0, string]("key0")
	p00 := esper.Field[infraNWTableOnMergeNestedST0, int]("p00")
	sourceS0 := esper.From[infraNWTableOnMergeMultiactionS0](env, "SupportBean_S0")
	s0ID := esper.Field[infraNWTableOnMergeMultiactionS0, int]("id")
	s0P00 := esper.Field[infraNWTableOnMergeMultiactionS0, string]("p00")

	// createConsumer deploys the query standing in for the Java create
	// statement: its iterator reads the window or table rows.
	createConsumer := func(name string, named bool) (esper.Plan, error) {
		if named {
			return env.Build(esper.FromNamedWindow(env, name).Query(
				esper.StatementName("Create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromTable(env, name).Query(
			esper.StatementName("Create"), esper.WithOldStream()))
	}

	switch caseName {
	case "multiaction-nw", "multiaction-table":
		switch label {
		case "create":
			plan, err := createConsumer("WinMDU", !isTable)
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "create", plan: plan}}, nil
		case "insert":
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceBean).InsertIntoTable("WinMDU",
					esper.SetColumn("theString", theString),
					esper.SetColumn("intPrimitive", intPrimitive)).Query())
			} else {
				plan, err = env.Build(esper.OnEvent(sourceBean).InsertIntoNamedWindow("WinMDU",
					esper.SetColumn("theString", theString),
					esper.SetColumn("intPrimitive", intPrimitive)).Query())
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "insert", plan: plan}}, nil
		case "merge":
			// Six ordered matched actions: unqualified intPrimitive in the
			// action where-clauses reads the merge-target row state left by
			// earlier actions, unqualified p00 reads the trigger event.
			var targetInt esper.Expression[int64]
			if isTable {
				targetInt = esper.TableField[int64]("intPrimitive")
			} else {
				targetInt = esper.NamedWindowField[int64]("intPrimitive")
			}
			actions := esper.WhenMatchedActions(
				esper.ThenDelete(esper.Less[int64](targetInt, esper.Literal(int64(0)))),
				esper.ThenUpdate(
					esper.Or(
						esper.Equal[int64](targetInt, esper.Literal(int64(3000))),
						esper.Equal[int](p00, esper.Literal(3000)),
					),
					esper.SetColumn("intPrimitive", p00),
				),
				esper.ThenUpdate(esper.Equal[int64](targetInt, esper.Literal(int64(1000))),
					esper.SetColumn("intPrimitive", esper.Literal(int64(999)))),
				esper.ThenDelete(esper.Equal[int64](targetInt, esper.Literal(int64(1000)))),
				esper.ThenUpdate(esper.Equal[int64](targetInt, esper.Literal(int64(2000))),
					esper.SetColumn("intPrimitive", esper.Literal(int64(1999)))),
				esper.ThenDelete(esper.Equal[int64](targetInt, esper.Literal(int64(2000)))),
			)
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceST0).MergeIntoTableWhen("WinMDU",
					[]esper.Expr{key0}, actions).Query(esper.StatementName("merge"), esper.WithOldStream()))
			} else {
				plan, err = env.Build(esper.OnEvent(sourceST0).MergeIntoNamedWindowWhen("WinMDU",
					esper.Equal[string](esper.NamedWindowField[string]("theString"), key0),
					actions).Query(esper.StatementName("merge"), esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "merge", plan: plan}}, nil
		}
	case "orderoffields-nw", "orderoffields-table":
		switch label {
		case "module", "create":
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.FromTable(env, "MyInfraUOF").Query())
			} else {
				plan, err = env.Build(esper.FromNamedWindow(env, "MyInfraUOF").Query())
			}
			if err != nil {
				return nil, err
			}
			createPlan := []infraNWTableOnMergeMultiactionBoundPlan{{label: "create", plan: plan}}
			if label == "create" {
				return createPlan, nil
			}
			insertPlans, err := infraNWTableOnMergeMultiactionBuild(env, caseName, "insert", isTable)
			if err != nil {
				return nil, err
			}
			mergePlans, err := infraNWTableOnMergeMultiactionBuild(env, caseName, "merge", isTable)
			if err != nil {
				return nil, err
			}
			return append(append(createPlan, insertPlans...), mergePlans...), nil
		case "insert":
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceBean).InsertIntoTable("MyInfraUOF",
					esper.SetColumn("theString", theString),
					esper.SetColumn("intPrimitive", intPrimitive),
					esper.SetColumn("intBoxed", intBoxed),
					esper.SetColumn("doublePrimitive", doublePrimitive)).Query())
			} else {
				plan, err = env.Build(esper.OnEvent(sourceBean).InsertIntoNamedWindow("MyInfraUOF",
					esper.SetColumn("theString", theString),
					esper.SetColumn("intPrimitive", intPrimitive),
					esper.SetColumn("intBoxed", intBoxed),
					esper.SetColumn("doublePrimitive", doublePrimitive)).Query())
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "insert", plan: plan}}, nil
		case "merge":
			// update set intPrimitive=id, intBoxed=mywin.intPrimitive,
			// doublePrimitive=initial.intPrimitive — assignments evaluate
			// left-to-right: intBoxed reads the already-updated
			// intPrimitive while initial.* reads the pre-update row.
			var targetInt, initialInt esper.Expression[int64]
			if isTable {
				targetInt = esper.TableField[int64]("intPrimitive")
				initialInt = esper.InitialTableField[int64]("intPrimitive")
			} else {
				targetInt = esper.NamedWindowField[int64]("intPrimitive")
				initialInt = esper.InitialNamedWindowField[int64]("intPrimitive")
			}
			assignments := []esper.TableAssignment{
				esper.SetColumn("intPrimitive", s0ID),
				esper.SetColumn("intBoxed", targetInt),
				esper.SetColumn("doublePrimitive", esper.Cast[int64, float64](initialInt)),
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceS0).MergeIntoTableWhen("MyInfraUOF",
					[]esper.Expr{s0P00},
					esper.WhenMatchedAny(assignments...)).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			} else {
				plan, err = env.Build(esper.OnEvent(sourceS0).MergeIntoNamedWindowWhen("MyInfraUOF",
					esper.Equal[string](esper.NamedWindowField[string]("theString"), s0P00),
					esper.WhenMatchedAny(assignments...)).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "merge", plan: plan}}, nil
		}
	case "subquery-nw", "subquery-table":
		switch label {
		case "create":
			plan, err := createConsumer("InfraOne", !isTable)
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "create", plan: plan}}, nil
		case "create-two":
			plan, err := createConsumer("InfraTwo", !isTable)
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "create-two", plan: plan}}, nil
		case "insert":
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceS0).InsertIntoTable("InfraTwo",
					esper.SetColumn("val0", esper.Literal("W2")),
					esper.SetColumn("val1", s0ID)).Query())
			} else {
				plan, err = env.Build(esper.OnEvent(sourceS0).InsertIntoNamedWindow("InfraTwo",
					esper.SetColumn("val0", esper.Literal("W2")),
					esper.SetColumn("val1", s0ID)).Query())
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "insert", plan: plan}}, nil
		case "merge":
			// when not matched then insert select 'Y' as string, (select
			// val1 from InfraTwo as w2 where w2.val0 = sb.theString) as
			// intPrimitive — the subquery correlates to the trigger event.
			var lookupStream esper.RecordStream
			if isTable {
				lookupStream = esper.FromTable(env, "InfraTwo")
			} else {
				lookupStream = esper.FromNamedWindow(env, "InfraTwo")
			}
			lookupValue := esper.SubqueryValue[int64](lookupStream,
				esper.Field[any, int64]("val1"),
				esper.Equal[string](esper.Field[any, string]("val0"),
					esper.OuterField[string]("theString")))
			assignments := []esper.TableAssignment{
				esper.SetColumn("string", esper.Literal("Y")),
				esper.SetColumn("intPrimitive", lookupValue),
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceBean).MergeIntoTableWhen("InfraOne",
					[]esper.Expr{theString},
					esper.WhenNotMatchedAny(assignments...)).Query(esper.WithOldStream()))
			} else {
				plan, err = env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen("InfraOne",
					esper.Equal[string](esper.NamedWindowField[string]("string"), theString),
					esper.WhenNotMatchedAny(assignments...)).Query(esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeMultiactionBoundPlan{{label: "merge", plan: plan}}, nil
		}
	}
	return nil, fmt.Errorf("unexpected deploy statement %q for case %q", label, caseName)
}

// decodeInfraNWTableOnMergeMultiactionPayload validates one send payload
// and builds the event: SupportBean carries theString/intPrimitive plus
// doublePrimitive when the payload sets it (mirroring makeSupportBean),
// SupportBean_ST0 carries id/key0/p00, and SupportBean_S0 carries id plus
// p00 when present.
func decodeInfraNWTableOnMergeMultiactionPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString       *string  `json:"theString"`
			IntPrimitive    int64    `json:"intPrimitive"`
			DoublePrimitive *float64 `json:"doublePrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean payload: %w", err)
		}
		event := infraNWTableOnMergeBean{
			IntPrimitive:  payload.IntPrimitive,
			CharPrimitive: "\u0000",
		}
		if payload.TheString != nil {
			event.TheString = *payload.TheString
		}
		if payload.DoublePrimitive != nil {
			event.DoublePrimitive = *payload.DoublePrimitive
		}
		return event, nil
	case "SupportBean_ST0":
		var payload struct {
			ID   string `json:"id"`
			Key0 string `json:"key0"`
			P00  int    `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_ST0 payload: %w", err)
		}
		return infraNWTableOnMergeNestedST0{
			ID: payload.ID, Key0: payload.Key0, P00: payload.P00,
		}, nil
	case "SupportBean_S0":
		var payload struct {
			ID  int     `json:"id"`
			P00 *string `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0 payload: %w", err)
		}
		event := infraNWTableOnMergeMultiactionS0{ID: payload.ID}
		if payload.P00 != nil {
			event.P00 = *payload.P00
		}
		return event, nil
	}
	return nil, fmt.Errorf("unexpected event type %q", step.EventType)
}

func requireInfraNWTableOnMergeMultiactionFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnMergeMultiactionScenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableOnMergeMultiactionScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeMultiactionID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeMultiactionID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeMultiactionID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeMultiactionID, err)
	}
	if err := requireInfraNWTableOnMergeMultiactionFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeMultiactionID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeMultiactionID ||
		metadata.Description != infraNWTableOnMergeMultiactionDescription ||
		metadata.JavaCommit != infraNWTableOnMergeMultiactionJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeMultiactionSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeMultiactionID)
	}
	if err := validateInfraNWTableOnMergeMultiactionStringArray(root["javaRuntimes"], infraNWTableOnMergeMultiactionJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeMultiactionStringArray(root["javaNames"], infraNWTableOnMergeMultiactionJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeMultiactionStringArray(root["javaStaticIds"], infraNWTableOnMergeMultiactionJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeMultiactionStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeMultiactionCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeMultiactionID, len(infraNWTableOnMergeMultiactionCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnMergeMultiactionFields(object,
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
		if definition.Case != infraNWTableOnMergeMultiactionCases[index] ||
			definition.Ordinal != infraNWTableOnMergeMultiactionOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeMultiactionJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeMultiactionJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeMultiactionCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeMultiactionCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeMultiactionID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeMultiactionID)
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
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnMergeMultiactionPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnMergeMultiactionFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnMergeMultiactionRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnMergeMultiactionStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnMergeMultiactionCaseEPLs pins the first deploy EPL of each
// case, the value carried by the scenario cases[] metadata.
var infraNWTableOnMergeMultiactionCaseEPLs = []string{
	infraNWTableOnMergeMultiactionCreateNW,
	infraNWTableOnMergeMultiactionCreateTbl,
	infraNWTableOnMergeMultiactionUOFModuleEPL(false),
	infraNWTableOnMergeMultiactionUOFModuleEPL(true),
	infraNWTableOnMergeMultiactionCreateOneNW,
	infraNWTableOnMergeMultiactionCreateOneTbl,
}

var infraNWTableOnMergeMultiactionCaseObservations = []string{
	"iterator; six ordered matched actions over the WinMDU keepall named window where later action where-clauses observe earlier action results, ending with an undeployModuleContaining('merge') and an EPL-to-model redeploy",
	"iterator; same six ordered matched actions over the WinMDU primary-key table",
	"listener; left-to-right update-set assignments over the MyInfraUOF keepall named window: intBoxed reads the already-updated intPrimitive while doublePrimitive reads initial.intPrimitive",
	"listener; same left-to-right update-set assignments over the MyInfraUOF primary-key table",
	"iterator; correlated subquery over the InfraTwo unique-key named window in the not-matched insert assignment into the InfraOne unique-key named window",
	"iterator; same correlated subquery over the InfraTwo composite-primary-key table into the InfraOne primary-key table",
}

// validateInfraNWTableOnMergeMultiactionRawSteps pins the complete step
// sequence per case against the raw JSON objects: deploy statements with
// byte-exact EPL, deployed markers, send event types with canonical
// payloads, snapshot reads, the merge undeploy and undeploy-all
// terminators.
func validateInfraNWTableOnMergeMultiactionRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeMultiactionCases {
		want, ok := infraNWTableOnMergeMultiactionCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeMultiactionID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeMultiactionID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeMultiactionID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeMultiactionID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeMultiactionStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeMultiactionID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeMultiactionID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeMultiactionID)
	}
	return nil
}

// infraNWTableOnMergeMultiactionStepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because compat.Step
// does not carry the fields array.
func infraNWTableOnMergeMultiactionStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "undeploy":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		return "undeploy:" + statement, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableOnMergeMultiactionCaseSteps pins the exact op sequence per
// case: deploy statements with byte-exact EPL, deployed markers, send event
// types with canonical payloads, snapshot reads, the merge undeploy and
// undeploy-all terminators.
var infraNWTableOnMergeMultiactionCaseSteps = map[string][]string{
	"multiaction-nw": {
		"deploy:create:@name('Create') @public create window WinMDU#keepall as SupportBean",
		"deployed:create",
		"deploy:insert:insert into WinMDU select theString, intPrimitive from SupportBean",
		"deployed:insert",
		"deploy:merge:@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where st0.key0=win.theString when matched then delete where intPrimitive<0 then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 then update set intPrimitive=999 where intPrimitive=1000 then delete where intPrimitive=1000 then update set intPrimitive=1999 where intPrimitive=2000 then delete where intPrimitive=2000",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E1\",\"p00\":0}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E2\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E2\",\"p00\":0}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":3000,\"theString\":\"E3\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E3\",\"p00\":3}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E4\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E4\",\"p00\":3000}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":1000,\"theString\":\"E5\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E5\",\"p00\":0}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":2000,\"theString\":\"E6\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E6\",\"p00\":0}",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy:merge",
		"deploy:merge:@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where st0.key0=win.theString when matched then delete where intPrimitive<0 then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 then update set intPrimitive=999 where intPrimitive=1000 then delete where intPrimitive=1000 then update set intPrimitive=1999 where intPrimitive=2000 then delete where intPrimitive=2000",
		"deployed:merge",
		"undeploy-all",
	},
	"multiaction-table": {
		"deploy:create:@name('Create') @public create table WinMDU (theString string primary key, intPrimitive int)",
		"deployed:create",
		"deploy:insert:insert into WinMDU select theString, intPrimitive from SupportBean",
		"deployed:insert",
		"deploy:merge:@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where st0.key0=win.theString when matched then delete where intPrimitive<0 then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 then update set intPrimitive=999 where intPrimitive=1000 then delete where intPrimitive=1000 then update set intPrimitive=1999 where intPrimitive=2000 then delete where intPrimitive=2000",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E1\",\"p00\":0}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E2\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E2\",\"p00\":0}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":3000,\"theString\":\"E3\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E3\",\"p00\":3}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E4\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E4\",\"p00\":3000}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":1000,\"theString\":\"E5\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E5\",\"p00\":0}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":2000,\"theString\":\"E6\"}",
		"send:SupportBean_ST0:{\"id\":\"ST0\",\"key0\":\"E6\",\"p00\":0}",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy:merge",
		"deploy:merge:@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where st0.key0=win.theString when matched then delete where intPrimitive<0 then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 then update set intPrimitive=999 where intPrimitive=1000 then delete where intPrimitive=1000 then update set intPrimitive=1999 where intPrimitive=2000 then delete where intPrimitive=2000",
		"deployed:merge",
		"undeploy-all",
	},
	"orderoffields-nw": {
		"deploy:module:@public create window MyInfraUOF#keepall as SupportBean;\ninsert into MyInfraUOF select theString, intPrimitive, intBoxed, doublePrimitive from SupportBean;\n@name('Merge') on SupportBean_S0 as sb merge MyInfraUOF as mywin where mywin.theString = sb.p00 when matched then update set intPrimitive=id, intBoxed=mywin.intPrimitive, doublePrimitive=initial.intPrimitive;\n",
		"deployed:create",
		"deployed:insert",
		"deployed:merge",
		"send:SupportBean:{\"doublePrimitive\":2,\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_S0:{\"id\":5,\"p00\":\"E1\"}",
		"send:SupportBean:{\"doublePrimitive\":20,\"intPrimitive\":10,\"theString\":\"E2\"}",
		"send:SupportBean_S0:{\"id\":6,\"p00\":\"E2\"}",
		"send:SupportBean_S0:{\"id\":7,\"p00\":\"E1\"}",
		"undeploy-all",
	},
	"orderoffields-table": {
		"deploy:module:@public create table MyInfraUOF(theString string primary key, intPrimitive int, intBoxed int, doublePrimitive double);\ninsert into MyInfraUOF select theString, intPrimitive, intBoxed, doublePrimitive from SupportBean;\n@name('Merge') on SupportBean_S0 as sb merge MyInfraUOF as mywin where mywin.theString = sb.p00 when matched then update set intPrimitive=id, intBoxed=mywin.intPrimitive, doublePrimitive=initial.intPrimitive;\n",
		"deployed:create",
		"deployed:insert",
		"deployed:merge",
		"send:SupportBean:{\"doublePrimitive\":2,\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_S0:{\"id\":5,\"p00\":\"E1\"}",
		"send:SupportBean:{\"doublePrimitive\":20,\"intPrimitive\":10,\"theString\":\"E2\"}",
		"send:SupportBean_S0:{\"id\":6,\"p00\":\"E2\"}",
		"send:SupportBean_S0:{\"id\":7,\"p00\":\"E1\"}",
		"undeploy-all",
	},
	"subquery-nw": {
		"deploy:create:@name('Create') @public create window InfraOne#unique(string) (string string, intPrimitive int)",
		"deployed:create",
		"deploy:create-two:@public create window InfraTwo#unique(val0) (val0 string, val1 int)",
		"deployed:create-two",
		"deploy:insert:insert into InfraTwo select 'W2' as val0, id as val1 from SupportBean_S0",
		"deployed:insert",
		"deploy:merge:on SupportBean sb merge InfraOne w1 where sb.theString = w1.string when not matched then insert select 'Y' as string, (select val1 from InfraTwo as w2 where w2.val0 = sb.theString) as intPrimitive",
		"deployed:merge",
		"send:SupportBean_S0:{\"id\":50}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"W2\"}",
		"snapshot:create:ordered:string,intPrimitive",
		"send:SupportBean_S0:{\"id\":51}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"W2\"}",
		"snapshot:create:ordered:string,intPrimitive",
		"undeploy-all",
	},
	"subquery-table": {
		"deploy:create:@name('Create') @public create table InfraOne (string string primary key, intPrimitive int)",
		"deployed:create",
		"deploy:create-two:@public create table InfraTwo (val0 string primary key, val1 int primary key)",
		"deployed:create-two",
		"deploy:insert:insert into InfraTwo select 'W2' as val0, id as val1 from SupportBean_S0",
		"deployed:insert",
		"deploy:merge:on SupportBean sb merge InfraOne w1 where sb.theString = w1.string when not matched then insert select 'Y' as string, (select val1 from InfraTwo as w2 where w2.val0 = sb.theString) as intPrimitive",
		"deployed:merge",
		"send:SupportBean_S0:{\"id\":50}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"W2\"}",
		"snapshot:create:ordered:string,intPrimitive",
		"undeploy-all",
	},
}

// infraNWTableOnMergeMultiactionSnapshotFields extracts the pinned
// projection list for the snapshot at steps[stepIndex] from the pinned step
// key ("snapshot:<statement>:<mode>:<f1,f2,...>"). The case marker occupies
// steps[0], so the pinned index is stepIndex-1.
func infraNWTableOnMergeMultiactionSnapshotFields(pinned []string, stepIndex int) []string {
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

// projectInfraNWTableOnMergeMultiactionRows reduces each row to the pinned
// assertion fields: the Java iterator assertions read only these properties
// even when the infra row carries the full SupportBean schema.
func projectInfraNWTableOnMergeMultiactionRows(rows []compat.ResultRecord, fields []string) []compat.ResultRecord {
	if len(fields) == 0 {
		return rows
	}
	projected := make([]compat.ResultRecord, len(rows))
	for index, row := range rows {
		narrowed := row
		if row.Fields != nil {
			narrowed.Fields = make(map[string]any, len(fields))
			for _, name := range fields {
				narrowed.Fields[name] = row.Fields[name]
			}
		}
		projected[index] = narrowed
	}
	return projected
}
