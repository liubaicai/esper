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
	infraNWTableOnMergeNestedID          = "infra-nwtable-on-merge-nested"
	infraNWTableOnMergeNestedDescription = "InfraNWTableOnMerge ordinals 4-7: nested-event merge assignment of map and objectarray Composite payloads over a lastevent named window and an unkeyed table asserted by fire-and-forget select, and insert-stream merge whose single not-matched branch runs five ordered insert actions into three side streams, a where-filtered side stream, and the merge target (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeNestedJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeNestedSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	infraNWTableOnMergeNestedFAF = "select cflat.c0 as cf0, carr[0].c0 as ca0, carr[1].c0 as ca1 from AInfra"
)

var (
	infraNWTableOnMergeNestedJavaSources = []string{
		infraNWTableOnMergeNestedSource,
	}
	infraNWTableOnMergeNestedJavaRuntimeIDs = []string{
		"java-runtime-065003de88aca37795b8",
		"java-runtime-065003de88aca37795b8",
		"java-runtime-2e8d691b5e2c927038d7",
		"java-runtime-2e8d691b5e2c927038d7",
		"java-runtime-2c687a68317caea3c148",
		"java-runtime-081455731ccafbf6847c",
	}
	infraNWTableOnMergeNestedJavaExecutions = []string{
		"InfraUpdateNestedEvent{namedWindow=true}",
		"InfraUpdateNestedEvent{namedWindow=true}",
		"InfraUpdateNestedEvent{namedWindow=false}",
		"InfraUpdateNestedEvent{namedWindow=false}",
		"InfraOnMergeInsertStream{namedWindow=true}",
		"InfraOnMergeInsertStream{namedWindow=false}",
	}
	infraNWTableOnMergeNestedJavaStaticIDs = []string{
		"java-712b26dbe20bbda50f37",
		"java-712b26dbe20bbda50f37",
		"java-712b26dbe20bbda50f37",
		"java-712b26dbe20bbda50f37",
		"java-9bcebf3cac321ec7aee2",
		"java-9bcebf3cac321ec7aee2",
	}
	infraNWTableOnMergeNestedCases = []string{
		"nested-nw-map",
		"nested-nw-oa",
		"nested-table-map",
		"nested-table-oa",
		"insertstream-nw",
		"insertstream-table",
	}
	infraNWTableOnMergeNestedOrdinals = []int{4, 4, 5, 5, 6, 7}
)

// infraNWTableOnMergeNestedST0 mirrors the regression SupportBean_ST0:
// id/key0/p00 plus the nullable p01Long and pcommon properties so the
// StreamOne wildcard row carries the full five-property shape the Java
// listener renders.
type infraNWTableOnMergeNestedST0 struct {
	ID      string  `esper:"id"`
	Key0    string  `esper:"key0"`
	P00     int     `esper:"p00"`
	P01Long *int64  `esper:"p01Long"`
	Pcommon *string `esper:"pcommon"`
}

func infraNWTableOnMergeNestedIsTable(caseName string) bool {
	return caseName == "nested-table-map" || caseName == "nested-table-oa" ||
		caseName == "insertstream-table"
}

func infraNWTableOnMergeNestedIsObjectArray(caseName string) bool {
	return caseName == "nested-nw-oa" || caseName == "nested-table-oa"
}

func infraNWTableOnMergeNestedIsInsertStream(caseName string) bool {
	return caseName == "insertstream-nw" || caseName == "insertstream-table"
}

// runInfraNWTableOnMergeNestedScenario replays the four InfraNWTableOnMerge
// executions as six cases: each nested case deploys the pinned six-statement
// module, sends the SupportBean seed and the MyEvent trigger, and records the
// FAF select row; each insert-stream case deploys the pinned seven-statement
// module, sends the two SupportBean_ST0 triggers, and records the s1-s4
// listener batches and the ordered 'Create' snapshot in Java's observable
// order.
func runInfraNWTableOnMergeNestedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeNestedCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeNestedCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeNestedID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeNestedID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeNestedCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnMergeNestedST0](env, "SupportBean_ST0"); err != nil {
		return compat.Trace{}, err
	}
	isTable := infraNWTableOnMergeNestedIsTable(caseName)
	if err := infraNWTableOnMergeNestedCreateInfra(env, caseName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeNestedJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeNestedID}
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
	// The Java module deploy binds every statement positionally; schema and
	// infra labels are environment-level on the Go side and have no deployed
	// statement, so they are tracked as known labels only.
	knownLabels := map[string]bool{}
	for _, label := range infraNWTableOnMergeNestedModuleLabels(caseName) {
		knownLabels[label] = true
	}
	pinned := infraNWTableOnMergeNestedCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plans, err := infraNWTableOnMergeNestedBuild(env, caseName, isTable)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			for _, bound := range plans {
				deployment, err := engine.Deploy(ctx, bound.plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", bound.label, err)
				}
				deployments = append(deployments, deployment)
				deploymentStatements := deployment.Statements()
				for _, statement := range deploymentStatements {
					if statement.Name() == bound.label {
						statements[bound.label] = statement
					}
				}
				if _, ok := statements[bound.label]; !ok && len(deploymentStatements) == 1 {
					// The merge statement is unnamed in Java; bind the
					// deployment's single anonymous statement.
					statements[bound.label] = deploymentStatements[0]
				}
				for _, statement := range deploymentStatements {
					if infraNWTableOnMergeNestedListened(statement.Name()) {
						name := statement.Name()
						if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
							record(name, batch)
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
			if err := infraNWTableOnMergeNestedSend(ctx, engine, caseName, step); err != nil {
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
			rows = projectInfraNWTableOnMergeNestedRows(rows, infraNWTableOnMergeNestedStepFields(pinned, stepIndex))
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
		case "faf":
			plan, err := infraNWTableOnMergeNestedBuildFAF(env, isTable, infraNWTableOnMergeNestedIsObjectArray(caseName))
			if err != nil {
				return compat.Trace{}, err
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				return compat.Trace{}, err
			}
			rows := compat.NormalizeResults(result.Batch.New)
			rows = projectInfraNWTableOnMergeNestedRows(rows, infraNWTableOnMergeNestedStepFields(pinned, stepIndex))
			sequence["faf"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "faf",
				Statement: step.Statement,
				Sequence:  sequence["faf"],
				Time:      "1970-01-01T00:00:00Z",
				New:       rows,
			})
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

// infraNWTableOnMergeNestedModuleLabels lists the module statements in EPL
// order; the Java oracle binds deployment.getStatements() positionally to
// these labels.
func infraNWTableOnMergeNestedModuleLabels(caseName string) []string {
	if infraNWTableOnMergeNestedIsInsertStream(caseName) {
		return []string{"schema", "create", "merge", "s1", "s2", "s3", "s4"}
	}
	return []string{"schema-composite", "schema-ainfra", "infra", "insert", "schema-myevent", "merge"}
}

// infraNWTableOnMergeNestedListened reports whether the Java execution
// attaches a listener to the named statement: only the insert-stream cases
// listen, on s1-s4.
func infraNWTableOnMergeNestedListened(statementName string) bool {
	switch statementName {
	case "s1", "s2", "s3", "s4":
		return true
	}
	return false
}

// infraNWTableOnMergeNestedCreateInfra registers the environment-level
// artifacts the Java module's @public statements establish: the Composite,
// AInfraType and MyEvent schemas plus the AInfra window/table for the nested
// cases, and the WinOMISSchema, StreamOne-Four types plus the WinOMIS
// window/table for the insert-stream cases.
func infraNWTableOnMergeNestedCreateInfra(env *esper.Environment, caseName string, isTable bool) error {
	if infraNWTableOnMergeNestedIsInsertStream(caseName) {
		return infraNWTableOnMergeNestedCreateInsertStreamInfra(env, isTable)
	}
	objectArray := infraNWTableOnMergeNestedIsObjectArray(caseName)
	register := esper.RegisterMap
	if objectArray {
		register = esper.RegisterObjectArray
	}
	composite, err := register(env, "Composite", []esper.FieldSpec{
		esper.FieldDef("c0", reflect.TypeOf(0)),
	})
	if err != nil {
		return err
	}
	var ainfraType esper.Schema
	if objectArray {
		ainfraType, err = register(env, "AInfraType", []esper.FieldSpec{
			esper.FieldDef("k", reflect.TypeOf("")),
			esper.FieldDef("cflat", reflect.TypeOf([]any{})),
			esper.FieldDef("carr", reflect.TypeOf([][]any{})),
		}, esper.WithNestedPropertySchema("cflat", composite), esper.WithNestedPropertySchema("carr", composite))
	} else {
		ainfraType, err = register(env, "AInfraType", []esper.FieldSpec{
			esper.FieldDef("k", reflect.TypeOf("")),
			esper.FieldDef("cflat", reflect.TypeOf(map[string]any{})),
			esper.FieldDef("carr", reflect.TypeOf([]map[string]any{})),
		}, esper.WithNestedPropertySchema("cflat", composite), esper.WithNestedPropertySchema("carr", composite))
	}
	if err != nil {
		return err
	}
	if isTable {
		var cflatColumn, carrColumn esper.TableColumn
		if objectArray {
			cflatColumn = esper.OptionalTableColumnOf[[]any]("cflat", esper.WithTableColumnNestedSchema(composite))
			carrColumn = esper.OptionalTableColumnOf[[][]any]("carr", esper.WithTableColumnNestedSchema(composite))
		} else {
			cflatColumn = esper.OptionalTableColumnOf[map[string]any]("cflat", esper.WithTableColumnNestedSchema(composite))
			carrColumn = esper.OptionalTableColumnOf[[]map[string]any]("carr", esper.WithTableColumnNestedSchema(composite))
		}
		if _, err := esper.CreateTable(env, "AInfra", []esper.TableColumn{
			esper.TableColumnOf[string]("k"),
			cflatColumn,
			carrColumn,
		}); err != nil {
			return err
		}
	} else {
		if _, err := esper.CreateNamedWindow(env, "AInfra", ainfraType,
			esper.NamedWindowRetention(esper.LastEvent())); err != nil {
			return err
		}
	}
	if objectArray {
		_, err = register(env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("cf", reflect.TypeOf([]any{})),
			esper.FieldDef("ca", reflect.TypeOf([][]any{})),
		})
	} else {
		_, err = register(env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("cf", reflect.TypeOf(map[string]any{})),
			esper.FieldDef("ca", reflect.TypeOf([]map[string]any{})),
		})
	}
	return err
}

// infraNWTableOnMergeNestedCreateInsertStreamInfra registers the
// insert-stream module's environment-level artifacts: the WinOMISSchema map
// type, the four side-stream types (StreamOne carries the full
// SupportBean_ST0 shape for the wildcard insert), and the WinOMIS keepall
// window or primary-key table.
func infraNWTableOnMergeNestedCreateInsertStreamInfra(env *esper.Environment, isTable bool) error {
	winSchema, err := esper.RegisterMap(env, "WinOMISSchema", []esper.FieldSpec{
		esper.FieldDef("v1", reflect.TypeOf("")),
		esper.FieldDef("v2", reflect.TypeOf(0)),
	})
	if err != nil {
		return err
	}
	if _, err := esper.RegisterMap(env, "StreamOne", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf("")),
		esper.FieldDef("key0", reflect.TypeOf("")),
		esper.FieldDef("p00", reflect.TypeOf(0)),
		esper.FieldDef("p01Long", reflect.TypeOf((*int64)(nil))),
		esper.FieldDef("pcommon", reflect.TypeOf((*string)(nil))),
	}); err != nil {
		return err
	}
	for _, name := range []string{"StreamTwo", "StreamThree", "StreamFour"} {
		if _, err := esper.RegisterMap(env, name, []esper.FieldSpec{
			esper.FieldDef("id", reflect.TypeOf("")),
			esper.FieldDef("key0", reflect.TypeOf("")),
		}); err != nil {
			return err
		}
	}
	if isTable {
		_, err := esper.CreateTable(env, "WinOMIS", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("v1"),
			esper.TableColumnOf[int]("v2"),
		})
		return err
	}
	_, err = esper.CreateNamedWindow(env, "WinOMIS", winSchema,
		esper.NamedWindowRetention(esper.KeepAll()))
	return err
}

// infraNWTableOnMergeNestedBoundPlan pairs one module statement label with
// the Go plan that produces the deployed statement.
type infraNWTableOnMergeNestedBoundPlan struct {
	label string
	plan  esper.Plan
}

// infraNWTableOnMergeNestedBuild mirrors the Java module: schema and infra
// statements are environment-level on the Go side, so only the insert and
// merge plans (nested cases) or the create-consumer, merge and s1-s4 plans
// (insert-stream cases) deploy. The module boundary is a Java packaging
// detail with no observable effect.
func infraNWTableOnMergeNestedBuild(env *esper.Environment, caseName string, isTable bool) ([]infraNWTableOnMergeNestedBoundPlan, error) {
	if infraNWTableOnMergeNestedIsInsertStream(caseName) {
		return infraNWTableOnMergeNestedBuildInsertStream(env, isTable)
	}
	return infraNWTableOnMergeNestedBuildNested(env, caseName, isTable)
}

// infraNWTableOnMergeNestedBuildNested builds the insert feed and the
// matched merge of the nested module: insert into AInfra select theString
// as k, null as cflat, null as carr from SupportBean, then on MyEvent e
// merge AInfra when matched then update set cflat = e.cf, carr = e.ca.
func infraNWTableOnMergeNestedBuildNested(env *esper.Environment, caseName string, isTable bool) ([]infraNWTableOnMergeNestedBoundPlan, error) {
	sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
	theString := esper.Field[infraNWTableOnMergeBean, string]("theString")
	var cflat, carr esper.Expr
	if infraNWTableOnMergeNestedIsObjectArray(caseName) {
		cflat = esper.NullLiteral[[]any]()
		carr = esper.NullLiteral[[][]any]()
	} else {
		cflat = esper.NullLiteral[map[string]any]()
		carr = esper.NullLiteral[[]map[string]any]()
	}
	var insertPlan esper.Plan
	var err error
	if isTable {
		insertPlan, err = env.Build(esper.OnEvent(sourceBean).InsertIntoTable("AInfra",
			esper.SetColumn("k", theString),
			esper.SetColumn("cflat", cflat),
			esper.SetColumn("carr", carr)).Query())
	} else {
		insertPlan, err = env.Build(esper.OnEvent(sourceBean).InsertIntoNamedWindow("AInfra",
			esper.SetColumn("k", theString),
			esper.SetColumn("cflat", cflat),
			esper.SetColumn("carr", carr)).Query())
	}
	if err != nil {
		return nil, err
	}

	sourceEvent := esper.FromAny(env, "MyEvent")
	var cf, ca esper.Expr
	if infraNWTableOnMergeNestedIsObjectArray(caseName) {
		cf = esper.Field[any, []any]("cf")
		ca = esper.Field[any, [][]any]("ca")
	} else {
		cf = esper.Field[any, map[string]any]("cf")
		ca = esper.Field[any, []map[string]any]("ca")
	}
	var mergePlan esper.Plan
	if isTable {
		// The unkeyed table holds a single row; the merge carries no where
		// clause, so the key slice is empty and the matched branch is
		// unconditional.
		mergePlan, err = env.Build(esper.OnRecord(sourceEvent).MergeIntoTableWhen("AInfra", nil,
			esper.WhenMatchedAny(
				esper.SetColumn("cflat", cf),
				esper.SetColumn("carr", ca))).Query())
	} else {
		mergePlan, err = env.Build(esper.OnRecord(sourceEvent).MergeIntoNamedWindowWhen("AInfra", nil,
			esper.WhenMatchedAny(
				esper.SetColumn("cflat", cf),
				esper.SetColumn("carr", ca))).Query())
	}
	if err != nil {
		return nil, err
	}
	return []infraNWTableOnMergeNestedBoundPlan{
		{label: "insert", plan: insertPlan},
		{label: "merge", plan: mergePlan},
	}, nil
}

// infraNWTableOnMergeNestedBuildInsertStream builds the create-consumer,
// merge and s1-s4 plans of the insert-stream module: the merge's single
// not-matched branch runs five ordered insert actions — StreamOne wildcard,
// StreamTwo/Three id+key0 projections, the key0="K2"-filtered StreamFour,
// and the WinOMIS target insert — and s1-s4 are the stream consumers the
// Java execution listens on.
func infraNWTableOnMergeNestedBuildInsertStream(env *esper.Environment, isTable bool) ([]infraNWTableOnMergeNestedBoundPlan, error) {
	source := esper.From[infraNWTableOnMergeNestedST0](env, "SupportBean_ST0")
	id := esper.Field[infraNWTableOnMergeNestedST0, string]("id")
	key0 := esper.Field[infraNWTableOnMergeNestedST0, string]("key0")
	p00 := esper.Field[infraNWTableOnMergeNestedST0, int]("p00")
	p01Long := esper.Field[infraNWTableOnMergeNestedST0, *int64]("p01Long")
	pcommon := esper.Field[infraNWTableOnMergeNestedST0, *string]("pcommon")
	actions := esper.WhenNotMatchedActions(
		// insert into StreamOne select * — the wildcard projection carries
		// every SupportBean_ST0 property, including the null p01Long and
		// pcommon.
		esper.ThenInsertInto("StreamOne",
			esper.Alias("id", id), esper.Alias("key0", key0), esper.Alias("p00", p00),
			esper.Alias("p01Long", p01Long), esper.Alias("pcommon", pcommon)),
		esper.ThenInsertInto("StreamTwo",
			esper.Alias("id", id), esper.Alias("key0", key0)),
		esper.ThenInsertInto("StreamThree",
			esper.Alias("id", id), esper.Alias("key0", key0)),
		esper.ThenInsertIntoWhen(
			esper.Equal[string](key0, esper.Literal("K2")), "StreamFour",
			esper.Alias("id", id), esper.Alias("key0", key0)),
		esper.ThenInsertIntoTarget(
			esper.SetColumn("v1", key0), esper.SetColumn("v2", p00)),
	)
	var createPlan, mergePlan esper.Plan
	var err error
	if isTable {
		createPlan, err = env.Build(esper.FromTable(env, "WinOMIS").Query(esper.StatementName("Create")))
		if err != nil {
			return nil, err
		}
		mergePlan, err = env.Build(esper.OnEvent(source).MergeIntoTableWhen("WinOMIS",
			[]esper.Expr{key0}, actions).Query())
	} else {
		createPlan, err = env.Build(esper.FromNamedWindow(env, "WinOMIS").Query(esper.StatementName("Create")))
		if err != nil {
			return nil, err
		}
		mergePlan, err = env.Build(esper.OnEvent(source).MergeIntoNamedWindowWhen("WinOMIS",
			esper.Equal[string](esper.NamedWindowField[string]("v1"), key0), actions).Query())
	}
	if err != nil {
		return nil, err
	}
	plans := []infraNWTableOnMergeNestedBoundPlan{
		{label: "create", plan: createPlan},
		{label: "merge", plan: mergePlan},
	}
	for _, label := range []string{"s1", "s2", "s3", "s4"} {
		stream := map[string]string{"s1": "StreamOne", "s2": "StreamTwo", "s3": "StreamThree", "s4": "StreamFour"}[label]
		consumerPlan, buildErr := env.Build(esper.FromAny(env, stream).Query(esper.StatementName(label)))
		if buildErr != nil {
			return nil, buildErr
		}
		plans = append(plans, infraNWTableOnMergeNestedBoundPlan{label: label, plan: consumerPlan})
	}
	return plans, nil
}

// infraNWTableOnMergeNestedBuildFAF builds the fire-and-forget select that
// reads the nested Composite properties off the AInfra row, mirroring
// compileExecuteFAF("select cflat.c0 as cf0, carr[0].c0 as ca0, carr[1].c0
// as ca1 from AInfra", path). Map rows resolve the Composite property by
// name through Property; objectarray rows resolve it positionally through
// ArrayAt because the schema-less nested read has no property-name table.
func infraNWTableOnMergeNestedBuildFAF(env *esper.Environment, isTable bool, objectArray bool) (esper.Plan, error) {
	var inner esper.RecordStream
	if isTable {
		inner = esper.FromTable(env, "AInfra")
	} else {
		inner = esper.FromNamedWindow(env, "AInfra")
	}
	cflat := esper.Field[any, any]("cflat")
	carr := esper.Field[any, any]("carr")
	var cf0, ca0, ca1 esper.Expr
	if objectArray {
		cf0 = esper.ArrayAt[int](cflat, esper.Literal(0))
		ca0 = esper.ArrayAt[int](esper.ArrayAt[any](carr, esper.Literal(0)), esper.Literal(0))
		ca1 = esper.ArrayAt[int](esper.ArrayAt[any](carr, esper.Literal(1)), esper.Literal(0))
	} else {
		cf0 = esper.Property[int](cflat, "c0")
		ca0 = esper.Property[int](esper.ArrayAt[any](carr, esper.Literal(0)), "c0")
		ca1 = esper.Property[int](esper.ArrayAt[any](carr, esper.Literal(1)), "c0")
	}
	return env.Build(inner.Select(
		esper.Alias("cf0", cf0),
		esper.Alias("ca0", ca0),
		esper.Alias("ca1", ca1),
	).Query())
}

// infraNWTableOnMergeNestedSend decodes and sends one pinned event: the
// SupportBean seed, the MyEvent trigger (map object or positional
// objectarray payload), or a SupportBean_ST0 trigger.
func infraNWTableOnMergeNestedSend(ctx context.Context, engine *esper.Engine, caseName string, step compat.Step) error {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    *string `json:"theString"`
			IntPrimitive int64   `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode SupportBean payload: %w", err)
		}
		event := infraNWTableOnMergeBean{
			IntPrimitive:  payload.IntPrimitive,
			CharPrimitive: "\u0000",
		}
		if payload.TheString != nil {
			event.TheString = *payload.TheString
		}
		return engine.Send(ctx, step.EventType, event)
	case "SupportBean_ST0":
		var payload struct {
			ID   string `json:"id"`
			Key0 string `json:"key0"`
			P00  int    `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode SupportBean_ST0 payload: %w", err)
		}
		return engine.Send(ctx, step.EventType, infraNWTableOnMergeNestedST0{
			ID: payload.ID, Key0: payload.Key0, P00: payload.P00,
		})
	case "MyEvent":
		if infraNWTableOnMergeNestedIsObjectArray(caseName) {
			// Positional payload [[1],[[1],[2]]]: cf is one Composite row
			// and ca a [][]any pair, matching makeNestedOAEvent.
			var positional []json.RawMessage
			if err := json.Unmarshal(step.Payload, &positional); err != nil || len(positional) != 2 {
				return fmt.Errorf("decode MyEvent objectarray payload: %w", err)
			}
			var cfInts []int
			if err := json.Unmarshal(positional[0], &cfInts); err != nil {
				return fmt.Errorf("decode MyEvent cf: %w", err)
			}
			var caInts [][]int
			if err := json.Unmarshal(positional[1], &caInts); err != nil {
				return fmt.Errorf("decode MyEvent ca: %w", err)
			}
			cf := make([]any, len(cfInts))
			for index, item := range cfInts {
				cf[index] = item
			}
			ca := make([][]any, len(caInts))
			for index, row := range caInts {
				element := make([]any, len(row))
				for column, item := range row {
					element[column] = item
				}
				ca[index] = element
			}
			return engine.SendObjectArray(ctx, step.EventType, []any{cf, ca})
		}
		// Map payload {"cf":{"c0":1},"ca":[{"c0":1},{"c0":2}]}, matching
		// makeNestedMapEvent: cf is one Composite map and ca a
		// []map[string]any pair.
		var payload struct {
			CF map[string]int   `json:"cf"`
			CA []map[string]int `json:"ca"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode MyEvent map payload: %w", err)
		}
		cf := make(map[string]any, len(payload.CF))
		for name, item := range payload.CF {
			cf[name] = item
		}
		ca := make([]map[string]any, len(payload.CA))
		for index, row := range payload.CA {
			element := make(map[string]any, len(row))
			for name, item := range row {
				element[name] = item
			}
			ca[index] = element
		}
		return engine.Send(ctx, step.EventType, map[string]any{"cf": cf, "ca": ca})
	}
	return fmt.Errorf("unexpected event type %q", step.EventType)
}

// decodeInfraNWTableOnMergeNestedPayload validates one send payload during
// scenario load without sending it.
func decodeInfraNWTableOnMergeNestedPayload(step compat.Step, caseName string) error {
	var fields map[string]json.RawMessage
	switch step.EventType {
	case "SupportBean":
		if err := strictObject(step.Payload, &fields); err != nil {
			return err
		}
		return requireInfraNWTableOnMergeNestedFields(fields, "theString", "intPrimitive")
	case "SupportBean_ST0":
		if err := strictObject(step.Payload, &fields); err != nil {
			return err
		}
		return requireInfraNWTableOnMergeNestedFields(fields, "id", "key0", "p00")
	case "MyEvent":
		if infraNWTableOnMergeNestedIsObjectArray(caseName) {
			var payload []any
			if err := json.Unmarshal(step.Payload, &payload); err != nil || payload == nil {
				return fmt.Errorf("MyEvent payload must be a JSON array")
			}
			return nil
		}
		if err := strictObject(step.Payload, &fields); err != nil {
			return err
		}
		return requireInfraNWTableOnMergeNestedFields(fields, "cf", "ca")
	}
	return fmt.Errorf("unexpected event type %q", step.EventType)
}

func requireInfraNWTableOnMergeNestedFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnMergeNestedScenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableOnMergeNestedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeNestedID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeNestedID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeNestedID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeNestedID, err)
	}
	if err := requireInfraNWTableOnMergeNestedFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeNestedID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeNestedID ||
		metadata.Description != infraNWTableOnMergeNestedDescription ||
		metadata.JavaCommit != infraNWTableOnMergeNestedJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeNestedSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeNestedID)
	}
	if err := validateInfraNWTableOnMergeNestedStringArray(root["javaRuntimes"], infraNWTableOnMergeNestedJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeNestedStringArray(root["javaNames"], infraNWTableOnMergeNestedJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeNestedStringArray(root["javaStaticIds"], infraNWTableOnMergeNestedJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeNestedStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeNestedCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeNestedID, len(infraNWTableOnMergeNestedCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnMergeNestedFields(object,
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
		if definition.Case != infraNWTableOnMergeNestedCases[index] ||
			definition.Ordinal != infraNWTableOnMergeNestedOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeNestedJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeNestedJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeNestedCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeNestedCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeNestedID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeNestedID)
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
		var stepCase string
		if rawCase, ok := object["case"]; ok {
			if err := json.Unmarshal(rawCase, &stepCase); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d case must be a string", index)
			}
		}
		switch operation {
		case "case":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if err := decodeInfraNWTableOnMergeNestedPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}, stepCase); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "faf":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case", "statement", "epl", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnMergeNestedFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnMergeNestedRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnMergeNestedStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnMergeNestedModuleEPL renders the pinned module text for one
// case: the nested module substitutes the metaType and the window/table
// create; the insert-stream module substitutes the WinOMIS create.
func infraNWTableOnMergeNestedModuleEPL(caseName string) string {
	if infraNWTableOnMergeNestedIsInsertStream(caseName) {
		create := "@name('Create') create table WinOMIS as (v1 string primary key, v2 int);\n"
		if !infraNWTableOnMergeNestedIsTable(caseName) {
			create = "@name('Create') create window WinOMIS#keepall as WinOMISSchema;\n"
		}
		return "create schema WinOMISSchema as (v1 string, v2 int);\n" +
			create +
			"on SupportBean_ST0 as st0 merge WinOMIS as win where win.v1=st0.key0 " +
			"when not matched " +
			"then insert into StreamOne select * " +
			"then insert into StreamTwo select st0.id as id, st0.key0 as key0 " +
			"then insert into StreamThree(id, key0) select st0.id, st0.key0 " +
			"then insert into StreamFour select id, key0 where key0=\"K2\" " +
			"then insert into WinOMIS select key0 as v1, p00 as v2;\n" +
			"@name('s1') select * from StreamOne;\n" +
			"@name('s2') select * from StreamTwo;\n" +
			"@name('s3') select * from StreamThree;\n" +
			"@name('s4') select * from StreamFour;\n"
	}
	metaType := "map"
	if infraNWTableOnMergeNestedIsObjectArray(caseName) {
		metaType = "objectarray"
	}
	infra := "@public create table AInfra (k string, cflat Composite, carr Composite[]);\n"
	if !infraNWTableOnMergeNestedIsTable(caseName) {
		infra = "@public create window AInfra#lastevent as AInfraType;\n"
	}
	return "@public create " + metaType + " schema Composite as (c0 int);\n" +
		"@buseventtype @public create " + metaType + " schema AInfraType as (k string, cflat Composite, carr Composite[]);\n" +
		infra +
		"insert into AInfra select theString as k, null as cflat, null as carr from SupportBean;\n" +
		"@public @buseventtype create " + metaType + " schema MyEvent as (cf Composite, ca Composite[]);\n" +
		"on MyEvent e merge AInfra when matched then update set cflat = e.cf, carr = e.ca"
}

// infraNWTableOnMergeNestedCaseEPLs pins the module EPL of each case, the
// value carried by the scenario cases[] metadata.
var infraNWTableOnMergeNestedCaseEPLs = []string{
	infraNWTableOnMergeNestedModuleEPL("nested-nw-map"),
	infraNWTableOnMergeNestedModuleEPL("nested-nw-oa"),
	infraNWTableOnMergeNestedModuleEPL("nested-table-map"),
	infraNWTableOnMergeNestedModuleEPL("nested-table-oa"),
	infraNWTableOnMergeNestedModuleEPL("insertstream-nw"),
	infraNWTableOnMergeNestedModuleEPL("insertstream-table"),
}

var infraNWTableOnMergeNestedCaseObservations = []string{
	"faf; map sub-run of the nested-event merge: a six-statement module declares Composite/AInfraType/MyEvent map schemas and a lastevent window, the SupportBean insert seeds k=E1 with null composites, the MyEvent merge assigns cf/ca wholesale, and the FAF select reads cflat.c0=1, carr[0].c0=1, carr[1].c0=2",
	"faf; objectarray sub-run of the nested-event merge over the lastevent window: same module shape with objectarray schemas, positional MyEvent payload [[1],[[1],[2]]], identical FAF assertion",
	"faf; map sub-run of the nested-event merge over the unkeyed table (no primary key): same module shape, single-row table updated by the matched merge, identical FAF assertion",
	"faf; objectarray sub-run of the nested-event merge over the unkeyed table: same module shape with objectarray schemas, positional MyEvent payload, identical FAF assertion",
	"listener+iterator; not-matched merge over the keepall window runs five ordered insert actions: StreamOne gets the wildcard row, StreamTwo/Three get id+key0, StreamFour filters key0=K2, and the target insert lands last; s1-s3 fire on both sends, s4 only on K2, Create iterates {K1,1},{K2,2}",
	"listener+iterator; same five-action not-matched merge over the primary-key table: s1-s3 fire on both sends, s4 only on K2, Create iterates {K1,1},{K2,2}",
}

// validateInfraNWTableOnMergeNestedRawSteps pins the complete step sequence
// per case against the raw JSON objects: one module deploy with byte-exact
// EPL, positional deployed markers, send event types with canonical
// payloads, the faf/snapshot reads, and undeploy-all terminators.
func validateInfraNWTableOnMergeNestedRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeNestedCases {
		want, ok := infraNWTableOnMergeNestedCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeNestedID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeNestedID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeNestedID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeNestedID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeNestedStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeNestedID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeNestedID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeNestedID)
	}
	return nil
}

// infraNWTableOnMergeNestedStepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because compat.Step
// does not carry the fields array.
func infraNWTableOnMergeNestedStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
		var payload any
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
	case "faf":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		fields, err := fieldsList()
		if err != nil {
			return "", err
		}
		return "faf:" + statement + ":" + epl + ":" + fields, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableOnMergeNestedCaseSteps pins the exact op sequence per case:
// one module deploy with byte-exact EPL, positional deployed markers, send
// event types with canonical payloads, the faf/snapshot reads, and
// undeploy-all terminators.
var infraNWTableOnMergeNestedCaseSteps = map[string][]string{
	"nested-nw-map": {
		"deploy:module:" + infraNWTableOnMergeNestedModuleEPL("nested-nw-map"),
		"deployed:schema-composite",
		"deployed:schema-ainfra",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-myevent",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:MyEvent:{\"ca\":[{\"c0\":1},{\"c0\":2}],\"cf\":{\"c0\":1}}",
		"faf:faf:" + infraNWTableOnMergeNestedFAF + ":cf0,ca0,ca1",
		"undeploy-all",
	},
	"nested-nw-oa": {
		"deploy:module:" + infraNWTableOnMergeNestedModuleEPL("nested-nw-oa"),
		"deployed:schema-composite",
		"deployed:schema-ainfra",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-myevent",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:MyEvent:[[1],[[1],[2]]]",
		"faf:faf:" + infraNWTableOnMergeNestedFAF + ":cf0,ca0,ca1",
		"undeploy-all",
	},
	"nested-table-map": {
		"deploy:module:" + infraNWTableOnMergeNestedModuleEPL("nested-table-map"),
		"deployed:schema-composite",
		"deployed:schema-ainfra",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-myevent",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:MyEvent:{\"ca\":[{\"c0\":1},{\"c0\":2}],\"cf\":{\"c0\":1}}",
		"faf:faf:" + infraNWTableOnMergeNestedFAF + ":cf0,ca0,ca1",
		"undeploy-all",
	},
	"nested-table-oa": {
		"deploy:module:" + infraNWTableOnMergeNestedModuleEPL("nested-table-oa"),
		"deployed:schema-composite",
		"deployed:schema-ainfra",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-myevent",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:MyEvent:[[1],[[1],[2]]]",
		"faf:faf:" + infraNWTableOnMergeNestedFAF + ":cf0,ca0,ca1",
		"undeploy-all",
	},
	"insertstream-nw": {
		"deploy:module:" + infraNWTableOnMergeNestedModuleEPL("insertstream-nw"),
		"deployed:schema",
		"deployed:create",
		"deployed:merge",
		"deployed:s1",
		"deployed:s2",
		"deployed:s3",
		"deployed:s4",
		"send:SupportBean_ST0:{\"id\":\"ID1\",\"key0\":\"K1\",\"p00\":1}",
		"send:SupportBean_ST0:{\"id\":\"ID1\",\"key0\":\"K2\",\"p00\":2}",
		"snapshot:create:ordered:v1,v2",
		"undeploy-all",
	},
	"insertstream-table": {
		"deploy:module:" + infraNWTableOnMergeNestedModuleEPL("insertstream-table"),
		"deployed:schema",
		"deployed:create",
		"deployed:merge",
		"deployed:s1",
		"deployed:s2",
		"deployed:s3",
		"deployed:s4",
		"send:SupportBean_ST0:{\"id\":\"ID1\",\"key0\":\"K1\",\"p00\":1}",
		"send:SupportBean_ST0:{\"id\":\"ID1\",\"key0\":\"K2\",\"p00\":2}",
		"snapshot:create:ordered:v1,v2",
		"undeploy-all",
	},
}

// infraNWTableOnMergeNestedStepFields extracts the pinned projection list
// for the snapshot/faf at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>" or
// "faf:<statement>:<epl>:<f1,f2,...>"). The case marker occupies steps[0],
// so the pinned index is stepIndex-1.
func infraNWTableOnMergeNestedStepFields(pinned []string, stepIndex int) []string {
	if stepIndex < 1 || stepIndex-1 >= len(pinned) {
		return nil
	}
	key := pinned[stepIndex-1]
	if !strings.HasPrefix(key, "snapshot:") && !strings.HasPrefix(key, "faf:") {
		return nil
	}
	parts := strings.SplitN(key, ":", 4)
	if len(parts) != 4 || parts[3] == "" {
		return nil
	}
	return strings.Split(parts[3], ",")
}

// projectInfraNWTableOnMergeNestedRows reduces each row to the pinned
// assertion fields: the Java FAF/iterator assertions read only these
// properties even when the row carries a wider schema.
func projectInfraNWTableOnMergeNestedRows(rows []compat.ResultRecord, fields []string) []compat.ResultRecord {
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
