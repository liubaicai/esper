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
	infraNWTableOnMergePatternNoWhereID          = "infra-nwtable-on-merge-pattern-nowhere"
	infraNWTableOnMergePatternNoWhereDescription = "InfraNWTableOnMerge ordinals 26-31: InfraPatternMultimatch merges every completed every-A-then-B pattern match into the MyInfraPM keepall named window or composite-primary-key table under a composite-key where-clause so re-matches are no-ops; InfraNoWhereClause merges without a where-clause over the MyInfraNWC keepall named window or unkeyed table so every existing row is matched and the not-matched insert branches fire only while the target is empty; InfraMultipleInsert evaluates four ordered when-not-matched insert clauses over the MyInfraMI keepall named window or primary-key table and delivers each inserted row to the 'Merge' listener; each execution runs over a named window and a table (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergePatternNoWhereJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergePatternNoWhereSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	// Verbatim transcriptions of InfraNWTableOnMerge lines 1002-1011
	// (InfraPatternMultimatch), 756-771 (InfraNoWhereClause) and 695-710
	// (InfraMultipleInsert). The pattern merge ends with a trailing space
	// and no semicolon; the module deploys end with ";\n".
	infraNWTableOnMergePatternNoWhereCreateNW  = "@name('create') @public create window MyInfraPM#keepall as (c1 string, c2 string)"
	infraNWTableOnMergePatternNoWhereCreateTbl = "@name('create') @public create table MyInfraPM as (c1 string primary key, c2 string primary key)"
	infraNWTableOnMergePatternNoWhereMergePM   = "@name('Merge') on pattern[every a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%', intPrimitive = a.intPrimitive)] me merge MyInfraPM mw where me.a.theString = mw.c1 and me.b.theString = mw.c2 when not matched then insert select me.a.theString as c1, me.b.theString as c2 "

	infraNWTableOnMergePatternNoWhereSchemaHead = "@public @buseventtype create schema MyEvent as (in1 string, in2 int);\n" +
		"create schema MySchema as (col1 string, col2 int);\n"
	infraNWTableOnMergePatternNoWhereNWCNW   = "@name('create') @public create window MyInfraNWC#keepall as MySchema;\n"
	infraNWTableOnMergePatternNoWhereNWCTbl  = "@name('create') @public create table MyInfraNWC (col1 string, col2 int);\n"
	infraNWTableOnMergePatternNoWhereNWCTail = "on SupportBean_A delete from MyInfraNWC;\n" +
		"on MyEvent me merge MyInfraNWC mw " +
		"when not matched and me.in1 like \"A%\" then insert(col1, col2) select me.in1, me.in2 " +
		"when not matched and me.in1 like \"B%\" then insert select me.in1 as col1, me.in2 as col2 " +
		"when matched and me.in1 like \"C%\" then update set col1='Z', col2=-1 " +
		"when not matched then insert select \"x\" || me.in1 || \"x\" as col1, me.in2 * -1 as col2;\n"

	infraNWTableOnMergePatternNoWhereMINW   = "@public create window MyInfraMI#keepall as MySchema;\n"
	infraNWTableOnMergePatternNoWhereMITbl  = "@public create table MyInfraMI (col1 string primary key, col2 int);\n"
	infraNWTableOnMergePatternNoWhereMITail = "@name('Merge') on MyEvent merge MyInfraMI " +
		"where col1=in1 " +
		"when not matched and in1 like \"A%\" then insert(col1, col2) select in1, in2 " +
		"when not matched and in1 like \"B%\" then insert select in1 as col1, in2 as col2 " +
		"when not matched and in1 like \"C%\" then insert select \"Z\" as col1, -1 as col2 " +
		"when not matched and in1 like \"D%\" then insert select \"x\"||in1||\"x\" as col1, in2*-1 as col2;\n"
)

var (
	infraNWTableOnMergePatternNoWhereJavaSources = []string{
		infraNWTableOnMergePatternNoWhereSource,
	}
	infraNWTableOnMergePatternNoWhereJavaRuntimeIDs = []string{
		"java-runtime-d28146beedb9cfdf9dbb",
		"java-runtime-ac7258306c8be9b66f8f",
		"java-runtime-0dc6b8eb05a1d3989b23",
		"java-runtime-c6f6bda40f34d29acb8c",
		"java-runtime-2c4851ea026cc835d39b",
		"java-runtime-f075aa28d64b1ae78d2e",
	}
	infraNWTableOnMergePatternNoWhereJavaExecutions = []string{
		"InfraPatternMultimatch{namedWindow=true}",
		"InfraPatternMultimatch{namedWindow=false}",
		"InfraNoWhereClause{namedWindow=true}",
		"InfraNoWhereClause{namedWindow=false}",
		"InfraMultipleInsert{namedWindow=true}",
		"InfraMultipleInsert{namedWindow=false}",
	}
	infraNWTableOnMergePatternNoWhereJavaStaticIDs = []string{
		"java-74cbcb4f6ad520a0f3fd",
		"java-74cbcb4f6ad520a0f3fd",
		"java-2b5b9c17389359bc78f7",
		"java-2b5b9c17389359bc78f7",
		"java-227e4a044ce4b9c1de81",
		"java-227e4a044ce4b9c1de81",
	}
	infraNWTableOnMergePatternNoWhereCases = []string{
		"patternmultimatch-nw",
		"patternmultimatch-table",
		"nowhere-nw",
		"nowhere-table",
		"multipleinsert-nw",
		"multipleinsert-table",
	}
	infraNWTableOnMergePatternNoWhereOrdinals = []int{26, 27, 28, 29, 30, 31}
)

func infraNWTableOnMergePatternNoWhereIsTable(caseName string) bool {
	return strings.HasSuffix(caseName, "-table")
}

func infraNWTableOnMergePatternNoWhereIsPattern(caseName string) bool {
	return strings.HasPrefix(caseName, "patternmultimatch-")
}

func infraNWTableOnMergePatternNoWhereIsNoWhere(caseName string) bool {
	return strings.HasPrefix(caseName, "nowhere-")
}

func infraNWTableOnMergePatternNoWhereIsMultipleInsert(caseName string) bool {
	return strings.HasPrefix(caseName, "multipleinsert-")
}

// infraNWTableOnMergePatternNoWhereNWCModuleEPL renders the verbatim
// five-statement module of InfraNoWhereClause (lines 756-771).
func infraNWTableOnMergePatternNoWhereNWCModuleEPL(isTable bool) string {
	if isTable {
		return infraNWTableOnMergePatternNoWhereSchemaHead +
			infraNWTableOnMergePatternNoWhereNWCTbl +
			infraNWTableOnMergePatternNoWhereNWCTail
	}
	return infraNWTableOnMergePatternNoWhereSchemaHead +
		infraNWTableOnMergePatternNoWhereNWCNW +
		infraNWTableOnMergePatternNoWhereNWCTail
}

// infraNWTableOnMergePatternNoWhereMIModuleEPL renders the verbatim
// four-statement module of InfraMultipleInsert (lines 695-710).
func infraNWTableOnMergePatternNoWhereMIModuleEPL(isTable bool) string {
	if isTable {
		return infraNWTableOnMergePatternNoWhereSchemaHead +
			infraNWTableOnMergePatternNoWhereMITbl +
			infraNWTableOnMergePatternNoWhereMITail
	}
	return infraNWTableOnMergePatternNoWhereSchemaHead +
		infraNWTableOnMergePatternNoWhereMINW +
		infraNWTableOnMergePatternNoWhereMITail
}

// infraNWTableOnMergePatternNoWhereLabels lists the statement labels that
// receive deployed markers per case, in EPL order: the pattern cases deploy
// create and merge separately; the module cases bind every module statement
// positionally (schema and infra labels are environment-level on the Go
// side and have no deployed statement).
func infraNWTableOnMergePatternNoWhereLabels(caseName string) []string {
	switch {
	case infraNWTableOnMergePatternNoWhereIsPattern(caseName):
		return []string{"create", "merge"}
	case infraNWTableOnMergePatternNoWhereIsNoWhere(caseName):
		return []string{"schema-myevent", "schema-myschema", "create", "delete", "merge"}
	case infraNWTableOnMergePatternNoWhereIsMultipleInsert(caseName):
		return []string{"schema-myevent", "schema-myschema", "infra", "merge"}
	}
	return nil
}

// runInfraNWTableOnMergePatternNoWhereScenario replays the six
// InfraNWTableOnMerge executions: each case deploys the pinned statements,
// sends the pinned events, and records deployed markers, 'Merge' listener
// batches and 'create' iterator snapshots in Java's observable order.
func runInfraNWTableOnMergePatternNoWhereScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergePatternNoWhereCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergePatternNoWhereCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergePatternNoWhereID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergePatternNoWhereID)
	}
	return trace, nil
}

func runInfraNWTableOnMergePatternNoWhereCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnDeleteA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnMergePatternNoWhereIsTable(caseName)
	if err := infraNWTableOnMergePatternNoWhereCreateInfra(env, caseName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergePatternNoWhereJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergePatternNoWhereID}
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
	statements := map[string]*esper.Statement{}
	labelDeployments := map[string]*esper.Deployment{}
	// Schema and infra labels are environment-level on the Go side; every
	// label that receives a deployed marker is known up front.
	knownLabels := map[string]bool{}
	for _, label := range infraNWTableOnMergePatternNoWhereLabels(caseName) {
		knownLabels[label] = true
	}
	pinned := infraNWTableOnMergePatternNoWhereCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plans, err := infraNWTableOnMergePatternNoWhereBuild(env, caseName, step.Statement, isTable)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			for _, bound := range plans {
				deployment, err := engine.Deploy(ctx, bound.plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", bound.label, err)
				}
				deployments = append(deployments, deployment)
				labelDeployments[bound.label] = deployment
				deploymentStatements := deployment.Statements()
				for _, statement := range deploymentStatements {
					if statement.Name() == bound.label {
						statements[bound.label] = statement
					}
				}
				if _, ok := statements[bound.label]; !ok && len(deploymentStatements) == 1 {
					// The delete trigger and the no-where merge are unnamed
					// in Java; bind the deployment's single statement to
					// the step label.
					statements[bound.label] = deploymentStatements[0]
				}
				for _, statement := range deploymentStatements {
					// Java attaches the listener only for
					// InfraMultipleInsert's addListener("Merge"); the
					// pattern merge statement is also named 'Merge' but
					// is never listened.
					if infraNWTableOnMergePatternNoWhereIsMultipleInsert(caseName) &&
						statement.Name() == "Merge" {
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
			if !knownLabels[step.Statement] {
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
			payload, err := decodeInfraNWTableOnMergePatternNoWherePayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if record, ok := payload.(map[string]any); ok {
				// MyEvent is a schema-declared map type sent via
				// sendEventMap in Java (sendMyEvent default
				// representation).
				err = engine.SendRecord(ctx, step.EventType, record)
			} else {
				err = engine.Send(ctx, step.EventType, payload)
			}
			if err != nil {
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
			rows = projectInfraNWTableOnMergePatternNoWhereRows(rows, infraNWTableOnMergePatternNoWhereSnapshotFields(pinned, stepIndex))
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
	for _, deployment := range deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, err
		}
	}
	return trace, nil
}

// infraNWTableOnMergePatternNoWhereCreateInfra registers the
// environment-level artifacts the Java @public create statements establish:
// the MyInfraPM keepall window or composite-primary-key table, or the
// MyEvent/MySchema schemas plus the MyInfraNWC keepall window or unkeyed
// table and the MyInfraMI keepall window or primary-key table.
func infraNWTableOnMergePatternNoWhereCreateInfra(env *esper.Environment, caseName string, isTable bool) error {
	switch {
	case infraNWTableOnMergePatternNoWhereIsPattern(caseName):
		if isTable {
			_, err := esper.CreateTable(env, "MyInfraPM", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("c1"),
				esper.PrimaryKeyColumn[string]("c2"),
			})
			return err
		}
		schema, err := esper.RegisterMap(env, "MyInfraPMSchema", []esper.FieldSpec{
			esper.FieldDef("c1", reflect.TypeOf("")),
			esper.FieldDef("c2", reflect.TypeOf("")),
		})
		if err != nil {
			return err
		}
		return createKeepAllNamedWindow(env, "MyInfraPM", schema)
	case infraNWTableOnMergePatternNoWhereIsNoWhere(caseName),
		infraNWTableOnMergePatternNoWhereIsMultipleInsert(caseName):
		// MyEvent is the merge trigger type the Java module declares with
		// @buseventtype; MySchema types the infra rows.
		if _, err := esper.RegisterMap(env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("in1", reflect.TypeOf("")),
			esper.FieldDef("in2", reflect.TypeOf(int64(0))),
		}, esper.BusEventType()); err != nil {
			return err
		}
		mySchema, err := esper.RegisterMap(env, "MySchema", []esper.FieldSpec{
			esper.FieldDef("col1", reflect.TypeOf("")),
			esper.FieldDef("col2", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		switch caseName {
		case "nowhere-nw":
			return createKeepAllNamedWindow(env, "MyInfraNWC", mySchema)
		case "nowhere-table":
			// The Java create table has no primary key: an unkeyed table.
			_, err := esper.CreateTable(env, "MyInfraNWC", []esper.TableColumn{
				esper.TableColumnOf[string]("col1"),
				esper.TableColumnOf[int64]("col2"),
			})
			return err
		case "multipleinsert-nw":
			return createKeepAllNamedWindow(env, "MyInfraMI", mySchema)
		case "multipleinsert-table":
			_, err := esper.CreateTable(env, "MyInfraMI", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("col1"),
				esper.TableColumnOf[int64]("col2"),
			})
			return err
		}
	}
	return fmt.Errorf("unexpected case %q", caseName)
}

// infraNWTableOnMergePatternNoWhereBoundPlan pairs one statement label with
// the Go plan that produces the deployed statement.
type infraNWTableOnMergePatternNoWhereBoundPlan struct {
	label string
	plan  esper.Plan
}

// infraNWTableOnMergePatternNoWhereBuild mirrors the Java deploys: the
// @public create statements are environment-level on the Go side, so each
// create label deploys the consumer query the Java iterator reads. The
// pattern merge deploys as two plans — a pattern select routed into a map
// stream, then an OnRecord merge trigger — the established workaround for
// the blocked direct OnPattern merge (trigger_pattern_multimatch_test.go
// precedent). The module labels deploy their consumer, delete and merge
// plans in module order, mirroring the single compileDeploy.
func infraNWTableOnMergePatternNoWhereBuild(env *esper.Environment, caseName string, label string, isTable bool) ([]infraNWTableOnMergePatternNoWhereBoundPlan, error) {
	sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
	theString := esper.Field[infraNWTableOnMergeBean, string]("theString")
	intPrimitive := esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")
	sourceA := esper.From[infraNWTableOnDeleteA](env, "SupportBean_A")
	sourceMyEvent := esper.FromAny(env, "MyEvent")
	in1 := esper.Field[any, string]("in1")
	in2 := esper.Field[any, int64]("in2")

	// createConsumer deploys the query standing in for the Java create
	// statement: its iterator reads the window or table rows.
	createConsumer := func(name string, named bool) (esper.Plan, error) {
		if named {
			return env.Build(esper.FromNamedWindow(env, name).Query(
				esper.StatementName("create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromTable(env, name).Query(
			esper.StatementName("create"), esper.WithOldStream()))
	}

	switch {
	case infraNWTableOnMergePatternNoWhereIsPattern(caseName):
		switch label {
		case "create":
			plan, err := createConsumer("MyInfraPM", !isTable)
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergePatternNoWhereBoundPlan{{label: "create", plan: plan}}, nil
		case "merge":
			// on pattern[every a=SupportBean(theString like 'A%') ->
			// b=SupportBean(theString like 'B%', intPrimitive =
			// a.intPrimitive)] me merge MyInfraPM mw where me.a.theString =
			// mw.c1 and me.b.theString = mw.c2 when not matched then insert
			// select me.a.theString as c1, me.b.theString as c2 — the
			// pattern select routes each completed match as a {c1,c2}
			// record and the OnRecord merge inserts it under the
			// composite-key match, preserving the one-merge-per-match
			// cardinality.
			if _, err := esper.RegisterMap(env, "MyInfraPMMergeTrigger", []esper.FieldSpec{
				esper.FieldDef("c1", reflect.TypeOf("")),
				esper.FieldDef("c2", reflect.TypeOf("")),
			}); err != nil {
				return nil, err
			}
			// Every() applies to the left leg so each A-event spawns its
			// own waiting branch (every a -> b); one B event completes
			// every pending branch with matching intPrimitive.
			pattern := esper.PatternFrom(
				sourceBean,
				"a",
				esper.LikeOf(theString, esper.Literal("A%")),
			).Every().FollowedBy(
				"b",
				esper.And(
					esper.LikeOf(theString, esper.Literal("B%")),
					esper.Equal[int64](intPrimitive, esper.TagField[int64]("a", "intPrimitive")),
				),
			)
			patternPlan, err := env.Build(pattern.Select(
				esper.Alias("c1", esper.TagField[string]("a", "theString")),
				esper.Alias("c2", esper.TagField[string]("b", "theString")),
			).InsertInto("MyInfraPMMergeTrigger"))
			if err != nil {
				return nil, err
			}
			route := esper.FromAny(env, "MyInfraPMMergeTrigger")
			c1 := esper.Field[any, string]("c1")
			c2 := esper.Field[any, string]("c2")
			assignments := []esper.TableAssignment{
				esper.SetColumn("c1", c1),
				esper.SetColumn("c2", c2),
			}
			var mergePlan esper.Plan
			if isTable {
				mergePlan, err = env.Build(esper.OnRecord(route).MergeIntoTableWhen(
					"MyInfraPM",
					[]esper.Expr{c1, c2},
					esper.WhenNotMatchedAny(assignments...),
				).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			} else {
				match := esper.And(
					esper.Equal[string](esper.NamedWindowField[string]("c1"), c1),
					esper.Equal[string](esper.NamedWindowField[string]("c2"), c2),
				)
				mergePlan, err = env.Build(esper.OnRecord(route).MergeIntoNamedWindowWhen(
					"MyInfraPM",
					match,
					esper.WhenNotMatchedAny(assignments...),
				).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergePatternNoWhereBoundPlan{
				{label: "merge", plan: patternPlan},
				{label: "merge", plan: mergePlan},
			}, nil
		}
	case infraNWTableOnMergePatternNoWhereIsNoWhere(caseName):
		switch label {
		case "module", "create":
			plan, err := createConsumer("MyInfraNWC", !isTable)
			if err != nil {
				return nil, err
			}
			createPlan := []infraNWTableOnMergePatternNoWhereBoundPlan{{label: "create", plan: plan}}
			if label == "create" {
				return createPlan, nil
			}
			deletePlans, err := infraNWTableOnMergePatternNoWhereBuild(env, caseName, "delete", isTable)
			if err != nil {
				return nil, err
			}
			mergePlans, err := infraNWTableOnMergePatternNoWhereBuild(env, caseName, "merge", isTable)
			if err != nil {
				return nil, err
			}
			return append(append(createPlan, deletePlans...), mergePlans...), nil
		case "delete":
			// on SupportBean_A delete from MyInfraNWC — an unnamed
			// delete-all trigger.
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceA).DeleteAllFromTable("MyInfraNWC").Query())
			} else {
				plan, err = env.Build(esper.OnEvent(sourceA).DeleteAllFromNamedWindow("MyInfraNWC").Query())
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergePatternNoWhereBoundPlan{{label: "delete", plan: plan}}, nil
		case "merge":
			// on MyEvent me merge MyInfraNWC mw when not matched and
			// me.in1 like "A%" then insert ... when matched and me.in1
			// like "C%" then update set col1='Z', col2=-1 when not matched
			// then insert ... — no where clause: a nil match expression
			// treats every existing target row as matched, so the
			// not-matched branches fire only while the target is empty.
			clauses := []esper.TableMergeClause{
				esper.WhenNotMatched(esper.LikeOf(in1, esper.Literal("A%")),
					esper.SetColumn("col1", in1),
					esper.SetColumn("col2", in2)),
				esper.WhenNotMatched(esper.LikeOf(in1, esper.Literal("B%")),
					esper.SetColumn("col1", in1),
					esper.SetColumn("col2", in2)),
				esper.WhenMatched(esper.LikeOf(in1, esper.Literal("C%")),
					esper.SetColumn("col1", esper.Literal("Z")),
					esper.SetColumn("col2", esper.Literal(int64(-1)))),
				esper.WhenNotMatchedAny(
					esper.SetColumn("col1", esper.ConcatOf(esper.Literal("x"), in1, esper.Literal("x"))),
					esper.SetColumn("col2", esper.Negate[int64](in2))),
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnRecord(sourceMyEvent).MergeIntoTableWhen(
					"MyInfraNWC", nil, clauses...).Query(esper.WithOldStream()))
			} else {
				plan, err = env.Build(esper.OnRecord(sourceMyEvent).MergeIntoNamedWindowWhen(
					"MyInfraNWC", nil, clauses...).Query(esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergePatternNoWhereBoundPlan{{label: "merge", plan: plan}}, nil
		}
	case infraNWTableOnMergePatternNoWhereIsMultipleInsert(caseName):
		switch label {
		case "module", "merge":
			// @name('Merge') on MyEvent merge MyInfraMI where col1=in1
			// when not matched and in1 like "A%" then insert ... — four
			// ordered not-matched clauses; the first matching condition
			// wins and the inserted row reaches the 'Merge' listener.
			clauses := []esper.TableMergeClause{
				esper.WhenNotMatched(esper.LikeOf(in1, esper.Literal("A%")),
					esper.SetColumn("col1", in1),
					esper.SetColumn("col2", in2)),
				esper.WhenNotMatched(esper.LikeOf(in1, esper.Literal("B%")),
					esper.SetColumn("col1", in1),
					esper.SetColumn("col2", in2)),
				esper.WhenNotMatched(esper.LikeOf(in1, esper.Literal("C%")),
					esper.SetColumn("col1", esper.Literal("Z")),
					esper.SetColumn("col2", esper.Literal(int64(-1)))),
				esper.WhenNotMatched(esper.LikeOf(in1, esper.Literal("D%")),
					esper.SetColumn("col1", esper.ConcatOf(esper.Literal("x"), in1, esper.Literal("x"))),
					esper.SetColumn("col2", esper.Negate[int64](in2))),
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnRecord(sourceMyEvent).MergeIntoTableWhen(
					"MyInfraMI", []esper.Expr{in1}, clauses...,
				).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			} else {
				plan, err = env.Build(esper.OnRecord(sourceMyEvent).MergeIntoNamedWindowWhen(
					"MyInfraMI",
					esper.Equal[string](esper.NamedWindowField[string]("col1"), in1),
					clauses...,
				).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergePatternNoWhereBoundPlan{{label: "merge", plan: plan}}, nil
		}
	}
	return nil, fmt.Errorf("unexpected deploy statement %q for case %q", label, caseName)
}

// decodeInfraNWTableOnMergePatternNoWherePayload validates one send payload
// and builds the event: SupportBean carries theString/intPrimitive,
// SupportBean_A carries id, and MyEvent carries the in1/in2 map the Java
// sendMyEvent helper sends in the default representation.
func decodeInfraNWTableOnMergePatternNoWherePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    *string `json:"theString"`
			IntPrimitive int64   `json:"intPrimitive"`
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
		return event, nil
	case "SupportBean_A":
		var payload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A payload: %w", err)
		}
		return infraNWTableOnDeleteA{ID: payload.ID}, nil
	case "MyEvent":
		var payload struct {
			In1 string `json:"in1"`
			In2 int64  `json:"in2"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode MyEvent payload: %w", err)
		}
		return map[string]any{"in1": payload.In1, "in2": payload.In2}, nil
	}
	return nil, fmt.Errorf("unexpected event type %q", step.EventType)
}

func requireInfraNWTableOnMergePatternNoWhereFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnMergePatternNoWhereScenario enforces the strict
// scenario contract shared by the differential runners: no duplicate or
// unknown JSON fields, pinned metadata, pinned per-case
// runtime/execution/EPL, and a per-op step field whitelist followed by a
// full step-shape pin.
func loadInfraNWTableOnMergePatternNoWhereScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergePatternNoWhereID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergePatternNoWhereID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergePatternNoWhereID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergePatternNoWhereID, err)
	}
	if err := requireInfraNWTableOnMergePatternNoWhereFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergePatternNoWhereID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergePatternNoWhereID ||
		metadata.Description != infraNWTableOnMergePatternNoWhereDescription ||
		metadata.JavaCommit != infraNWTableOnMergePatternNoWhereJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergePatternNoWhereSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergePatternNoWhereID)
	}
	if err := validateInfraNWTableOnMergePatternNoWhereStringArray(root["javaRuntimes"], infraNWTableOnMergePatternNoWhereJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergePatternNoWhereStringArray(root["javaNames"], infraNWTableOnMergePatternNoWhereJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergePatternNoWhereStringArray(root["javaStaticIds"], infraNWTableOnMergePatternNoWhereJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergePatternNoWhereStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergePatternNoWhereCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergePatternNoWhereID, len(infraNWTableOnMergePatternNoWhereCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnMergePatternNoWhereFields(object,
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
		if definition.Case != infraNWTableOnMergePatternNoWhereCases[index] ||
			definition.Ordinal != infraNWTableOnMergePatternNoWhereOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergePatternNoWhereJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergePatternNoWhereJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergePatternNoWhereCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergePatternNoWhereCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergePatternNoWhereID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergePatternNoWhereID)
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
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnMergePatternNoWherePayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnMergePatternNoWhereFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnMergePatternNoWhereRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnMergePatternNoWhereStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnMergePatternNoWhereCaseEPLs pins the first deploy EPL of
// each case, the value carried by the scenario cases[] metadata.
var infraNWTableOnMergePatternNoWhereCaseEPLs = []string{
	infraNWTableOnMergePatternNoWhereCreateNW,
	infraNWTableOnMergePatternNoWhereCreateTbl,
	infraNWTableOnMergePatternNoWhereNWCModuleEPL(false),
	infraNWTableOnMergePatternNoWhereNWCModuleEPL(true),
	infraNWTableOnMergePatternNoWhereMIModuleEPL(false),
	infraNWTableOnMergePatternNoWhereMIModuleEPL(true),
}

var infraNWTableOnMergePatternNoWhereCaseObservations = []string{
	"iterator; every-A-then-B pattern merge into the MyInfraPM keepall named window: one B event completes every pending A branch and the composite-key where-clause makes re-matches no-ops",
	"iterator; same every-A-then-B pattern merge into the MyInfraPM composite-primary-key table",
	"iterator; merge without a where-clause over the MyInfraNWC keepall named window: every existing row is matched, the not-matched insert branches fire only while the target is empty and the 'C%' matched update rewrites all rows",
	"iterator; same no-where merge over the MyInfraNWC unkeyed table",
	"listener; four ordered when-not-matched insert clauses over the MyInfraMI keepall named window deliver each inserted row to the 'Merge' listener as new data",
	"listener; same four ordered when-not-matched insert clauses over the MyInfraMI primary-key table",
}

// validateInfraNWTableOnMergePatternNoWhereRawSteps pins the complete step
// sequence per case against the raw JSON objects: deploy statements with
// byte-exact EPL, deployed markers, send event types with canonical
// payloads, snapshot reads and the undeploy-all terminators.
func validateInfraNWTableOnMergePatternNoWhereRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergePatternNoWhereCases {
		want, ok := infraNWTableOnMergePatternNoWhereCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergePatternNoWhereID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergePatternNoWhereID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergePatternNoWhereID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergePatternNoWhereID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergePatternNoWhereStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergePatternNoWhereID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergePatternNoWhereID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergePatternNoWhereID)
	}
	return nil
}

// infraNWTableOnMergePatternNoWhereStepKey renders a raw step object into
// its pinned string form. Fields are read from the raw JSON because
// compat.Step does not carry the fields array.
func infraNWTableOnMergePatternNoWhereStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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

// infraNWTableOnMergePatternNoWhereCaseSteps pins the exact op sequence per
// case: deploy statements with byte-exact EPL, deployed markers, send event
// types with canonical payloads, snapshot reads and the undeploy-all
// terminators.
var infraNWTableOnMergePatternNoWhereCaseSteps = map[string][]string{
	"patternmultimatch-nw": {
		"deploy:create:@name('create') @public create window MyInfraPM#keepall as (c1 string, c2 string)",
		"deployed:create",
		"deploy:merge:@name('Merge') on pattern[every a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%', intPrimitive = a.intPrimitive)] me merge MyInfraPM mw where me.a.theString = mw.c1 and me.b.theString = mw.c2 when not matched then insert select me.a.theString as c1, me.b.theString as c2 ",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A2\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"B1\"}",
		"snapshot:create:any:c1,c2",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"A3\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"A4\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"B2\"}",
		"snapshot:create:any:c1,c2",
		"undeploy-all",
	},
	"patternmultimatch-table": {
		"deploy:create:@name('create') @public create table MyInfraPM as (c1 string primary key, c2 string primary key)",
		"deployed:create",
		"deploy:merge:@name('Merge') on pattern[every a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%', intPrimitive = a.intPrimitive)] me merge MyInfraPM mw where me.a.theString = mw.c1 and me.b.theString = mw.c2 when not matched then insert select me.a.theString as c1, me.b.theString as c2 ",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A1\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A2\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"B1\"}",
		"snapshot:create:any:c1,c2",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"A3\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"A4\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"B2\"}",
		"snapshot:create:any:c1,c2",
		"undeploy-all",
	},
	"nowhere-nw": {
		"deploy:module:@public @buseventtype create schema MyEvent as (in1 string, in2 int);\ncreate schema MySchema as (col1 string, col2 int);\n@name('create') @public create window MyInfraNWC#keepall as MySchema;\non SupportBean_A delete from MyInfraNWC;\non MyEvent me merge MyInfraNWC mw when not matched and me.in1 like \"A%\" then insert(col1, col2) select me.in1, me.in2 when not matched and me.in1 like \"B%\" then insert select me.in1 as col1, me.in2 as col2 when matched and me.in1 like \"C%\" then update set col1='Z', col2=-1 when not matched then insert select \"x\" || me.in1 || \"x\" as col1, me.in2 * -1 as col2;\n",
		"deployed:schema-myevent",
		"deployed:schema-myschema",
		"deployed:create",
		"deployed:delete",
		"deployed:merge",
		"send:MyEvent:{\"in1\":\"E1\",\"in2\":2}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"A1\",\"in2\":3}",
		"snapshot:create:any:col1,col2",
		"send:SupportBean_A:{\"id\":\"Ax1\"}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"A1\",\"in2\":4}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":5}",
		"snapshot:create:any:col1,col2",
		"send:SupportBean_A:{\"id\":\"Ax1\"}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":5}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"C\",\"in2\":6}",
		"snapshot:create:any:col1,col2",
		"undeploy-all",
	},
	"nowhere-table": {
		"deploy:module:@public @buseventtype create schema MyEvent as (in1 string, in2 int);\ncreate schema MySchema as (col1 string, col2 int);\n@name('create') @public create table MyInfraNWC (col1 string, col2 int);\non SupportBean_A delete from MyInfraNWC;\non MyEvent me merge MyInfraNWC mw when not matched and me.in1 like \"A%\" then insert(col1, col2) select me.in1, me.in2 when not matched and me.in1 like \"B%\" then insert select me.in1 as col1, me.in2 as col2 when matched and me.in1 like \"C%\" then update set col1='Z', col2=-1 when not matched then insert select \"x\" || me.in1 || \"x\" as col1, me.in2 * -1 as col2;\n",
		"deployed:schema-myevent",
		"deployed:schema-myschema",
		"deployed:create",
		"deployed:delete",
		"deployed:merge",
		"send:MyEvent:{\"in1\":\"E1\",\"in2\":2}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"A1\",\"in2\":3}",
		"snapshot:create:any:col1,col2",
		"send:SupportBean_A:{\"id\":\"Ax1\"}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"A1\",\"in2\":4}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":5}",
		"snapshot:create:any:col1,col2",
		"send:SupportBean_A:{\"id\":\"Ax1\"}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":5}",
		"snapshot:create:any:col1,col2",
		"send:MyEvent:{\"in1\":\"C\",\"in2\":6}",
		"snapshot:create:any:col1,col2",
		"undeploy-all",
	},
	"multipleinsert-nw": {
		"deploy:module:@public @buseventtype create schema MyEvent as (in1 string, in2 int);\ncreate schema MySchema as (col1 string, col2 int);\n@public create window MyInfraMI#keepall as MySchema;\n@name('Merge') on MyEvent merge MyInfraMI where col1=in1 when not matched and in1 like \"A%\" then insert(col1, col2) select in1, in2 when not matched and in1 like \"B%\" then insert select in1 as col1, in2 as col2 when not matched and in1 like \"C%\" then insert select \"Z\" as col1, -1 as col2 when not matched and in1 like \"D%\" then insert select \"x\"||in1||\"x\" as col1, in2*-1 as col2;\n",
		"deployed:schema-myevent",
		"deployed:schema-myschema",
		"deployed:infra",
		"deployed:merge",
		"send:MyEvent:{\"in1\":\"E1\",\"in2\":0}",
		"send:MyEvent:{\"in1\":\"A1\",\"in2\":1}",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":2}",
		"send:MyEvent:{\"in1\":\"C1\",\"in2\":3}",
		"send:MyEvent:{\"in1\":\"D1\",\"in2\":4}",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":2}",
		"undeploy-all",
	},
	"multipleinsert-table": {
		"deploy:module:@public @buseventtype create schema MyEvent as (in1 string, in2 int);\ncreate schema MySchema as (col1 string, col2 int);\n@public create table MyInfraMI (col1 string primary key, col2 int);\n@name('Merge') on MyEvent merge MyInfraMI where col1=in1 when not matched and in1 like \"A%\" then insert(col1, col2) select in1, in2 when not matched and in1 like \"B%\" then insert select in1 as col1, in2 as col2 when not matched and in1 like \"C%\" then insert select \"Z\" as col1, -1 as col2 when not matched and in1 like \"D%\" then insert select \"x\"||in1||\"x\" as col1, in2*-1 as col2;\n",
		"deployed:schema-myevent",
		"deployed:schema-myschema",
		"deployed:infra",
		"deployed:merge",
		"send:MyEvent:{\"in1\":\"E1\",\"in2\":0}",
		"send:MyEvent:{\"in1\":\"A1\",\"in2\":1}",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":2}",
		"send:MyEvent:{\"in1\":\"C1\",\"in2\":3}",
		"send:MyEvent:{\"in1\":\"D1\",\"in2\":4}",
		"send:MyEvent:{\"in1\":\"B1\",\"in2\":2}",
		"undeploy-all",
	},
}

// infraNWTableOnMergePatternNoWhereSnapshotFields extracts the pinned
// projection list for the snapshot at steps[stepIndex] from the pinned step
// key ("snapshot:<statement>:<mode>:<f1,f2,...>"). The case marker occupies
// steps[0], so the pinned index is stepIndex-1.
func infraNWTableOnMergePatternNoWhereSnapshotFields(pinned []string, stepIndex int) []string {
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

// projectInfraNWTableOnMergePatternNoWhereRows reduces each row to the
// pinned assertion fields: the Java iterator assertions read only these
// properties.
func projectInfraNWTableOnMergePatternNoWhereRows(rows []compat.ResultRecord, fields []string) []compat.ResultRecord {
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
