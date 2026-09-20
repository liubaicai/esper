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

// infra_nwtable_on_merge_insertonly_deletethenupdate.go replays
// InfraNWTableOnMerge ordinals 46-53 against the pinned Java oracle: the six
// remaining InfraInsertOnly executions (ord 46 = namedWindow=true
// soda+useColumnNames; ords 47-51 = all five namedWindow=false table
// variants) and InfraDeleteThenUpdate{namedWindow} (ords 52-53).
//
// Insert-only cases deploy the InsertOnlyInfra unique-key named window or
// primary-key table plus one 'on' merge variant, attach the listener to
// 'on', send SupportBean("E1",1) and SupportBean("E2",2) with an iterator
// snapshot of 'Window' after each, and end with undeployAll.
//
// Delete-then-update cases deploy the MyInfra keepall window or primary-key
// table plus the 'merge' statement — one matched clause carrying ordered
// delete then update actions — attach the listener to 'merge' (mirroring
// addListener("merge"); the Java execution never asserts it), seed {A,1}
// through a fire-and-forget insert, snapshot 'create', send
// SupportBean("A",10), snapshot again and undeploy. The named-window
// iterator reads {A,10} (update wins); the table iterator is empty (delete
// wins).
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table` map to env-level registrations; the
//     'Window'/'create' deploy steps attach the consumer query the Java
//     iterator reads.
//   - The soda variants (ords 46, 50, 51) pin the same EPL as their
//     non-soda twins; Java's compileDeploy(soda=true) only asserts the
//     eplToModel round-trip, so the Go replay is identical.
//   - Java's insert(p0, p1) select column list is sugar for the same
//     SetColumn assignments.
//   - Table merges have no where-predicate surface: `where 1=2` (ord 47)
//     maps to a constant never-matching key expression (contract-freeze
//     decision), and the no-where plain/colnames forms key on the trigger
//     theString like the simple-table precedent.

const (
	infraNWTableOnMergeIDTUID          = "infra-nwtable-on-merge-insertonly-deletethenupdate"
	infraNWTableOnMergeIDTUDescription = "InfraNWTableOnMerge ordinals 46-53: InfraInsertOnly replays the six remaining insert-only variants — ord 46 the named-window soda+useColumnNames form, ords 47-51 the five namedWindow=false table variants (useEquivalent where 1=2, plain, useColumnNames, soda, soda+useColumnNames) — deploying the InsertOnlyInfra unique-key window or primary-key table plus one 'on' merge and observing the 'on' listener rows and Window iterator across two SupportBean sends; InfraDeleteThenUpdate deploys the MyInfra keepall window or primary-key table plus the 'merge' delete-then-update statement, seeds {A,1} via fire-and-forget insert, sends SupportBean(A,10) and observes the divergent outcome — update-wins {A,10} for the named window, delete-wins empty for the table — with the 'merge' listener attached (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeIDTUJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeIDTUSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	// Verbatim transcriptions of InfraNWTableOnMerge lines 480-494
	// (InfraInsertOnly) and 319-330 (InfraDeleteThenUpdate).
	iduInsertOnlyCreateNW      = "@Name('Window') @public create window InsertOnlyInfra#unique(p0) as (p0 string, p1 int)"
	iduInsertOnlyCreateTable   = "@Name('Window') @public create table InsertOnlyInfra (p0 string primary key, p1 int)"
	iduInsertOnlyEquivalentEPL = "@name('on') on SupportBean merge InsertOnlyInfra where 1=2 when not matched then insert select theString as p0, intPrimitive as p1"
	iduInsertOnlyPlainEPL      = "@name('on') on SupportBean merge InsertOnlyInfra insert select theString as p0, intPrimitive as p1"
	iduInsertOnlyColnamesEPL   = "@name('on') on SupportBean as provider merge InsertOnlyInfra insert(p0, p1) select provider.theString, intPrimitive"

	iduDeleteThenUpdateCreateNW    = "@name('create') @public create window MyInfra#keepall() as (p0 string, p1 int)"
	iduDeleteThenUpdateCreateTable = "@name('create') @public create table MyInfra(p0 string primary key, p1 int)"
	iduDeleteThenUpdateMergeEPL    = "@name('merge') on SupportBean sb merge MyInfra where theString = p0 when matched then delete then update set p1 = intPrimitive"
	iduDeleteThenUpdateFafInsert   = "insert into MyInfra select 'A' as p0, 1 as p1"
)

var (
	infraNWTableOnMergeIDTUJavaSources = []string{
		infraNWTableOnMergeIDTUSource,
	}
	infraNWTableOnMergeIDTUJavaRuntimeIDs = []string{
		"java-runtime-5cdc46289e4fac78a0c5",
		"java-runtime-8e9616eb8385c473d45a",
		"java-runtime-af614186a63cbeb33ae5",
		"java-runtime-eb7754c9e46c8c465c14",
		"java-runtime-f21a6fc889f14608828f",
		"java-runtime-f7a73c74e857ffbdfd15",
		"java-runtime-5816ec0ef519ec8a48e1",
		"java-runtime-3cca4ced23a6097b5023",
	}
	infraNWTableOnMergeIDTUJavaExecutions = []string{
		"InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=true, useColumnNames=true}",
		"InfraInsertOnly{namedWindow=false, useEquivalent=true, soda=false, useColumnNames=false}",
		"InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=false, useColumnNames=false}",
		"InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=false, useColumnNames=true}",
		"InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=true, useColumnNames=false}",
		"InfraInsertOnly{namedWindow=false, useEquivalent=false, soda=true, useColumnNames=true}",
		"InfraDeleteThenUpdate{namedWindow=true}",
		"InfraDeleteThenUpdate{namedWindow=false}",
	}
	infraNWTableOnMergeIDTUJavaStaticIDs = []string{
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0acf62afc326a95d7216",
		"java-0acf62afc326a95d7216",
	}
	infraNWTableOnMergeIDTUCases = []string{
		"insertonly-nw-soda-colnames",
		"insertonly-table-equivalent",
		"insertonly-table",
		"insertonly-table-colnames",
		"insertonly-table-soda",
		"insertonly-table-soda-colnames",
		"deletethenupdate-nw",
		"deletethenupdate-table",
	}
	infraNWTableOnMergeIDTUOrdinals = []int{46, 47, 48, 49, 50, 51, 52, 53}
)

func infraNWTableOnMergeIDTUIsInsertOnly(caseName string) bool {
	return strings.HasPrefix(caseName, "insertonly-")
}

func infraNWTableOnMergeIDTUIsTable(caseName string) bool {
	return strings.Contains(caseName, "-table")
}

// iduInsertOnlyMergeEPL pins the 'on' merge EPL per insert-only case (lines
// 485-492); the soda variants share their non-soda twin's EPL.
func iduInsertOnlyMergeEPL(caseName string) string {
	switch caseName {
	case "insertonly-table-equivalent":
		return iduInsertOnlyEquivalentEPL
	case "insertonly-nw-soda-colnames", "insertonly-table-colnames",
		"insertonly-table-soda-colnames":
		return iduInsertOnlyColnamesEPL
	default:
		// insertonly-table and insertonly-table-soda share the plain EPL;
		// soda only changes the Java compile path.
		return iduInsertOnlyPlainEPL
	}
}

// runInfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenario replays the eight
// InfraNWTableOnMerge executions: insert-only cases record deployed markers,
// the 'on' listener batches and Window snapshots; delete-then-update cases
// record deployed markers, the 'merge' listener batch and the ordered
// 'create' snapshots, all in Java's observable order.
func runInfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeIDTUCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeInsertOnlyDeleteThenUpdateCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeIDTUID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeIDTUID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeInsertOnlyDeleteThenUpdateCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnMergeIDTUIsTable(caseName)
	if err := infraNWTableOnMergeIDTUCreateInfra(env, caseName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeIDTUJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeIDTUID}
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
	pinned := infraNWTableOnMergeIDTUCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if step.Statement == "FafInsert" {
				// Java executes the seed insert via compileExecuteFAFNoResult:
				// a fire-and-forget insert-into with no result rows.
				plan, err := infraNWTableOnMergeIDTUBuildFAFInsert(env, isTable)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
				}
				if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
					return compat.Trace{}, fmt.Errorf("faf insert %q: %w", step.Statement, err)
				}
				continue
			}
			plan, err := infraNWTableOnMergeIDTUBuild(env, caseName, step.Statement, step.Epl, isTable)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			deployments = append(deployments, deployment)
			deploymentStatements := deployment.Statements()
			for _, statement := range deploymentStatements {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
				}
			}
			if _, ok := statements[step.Statement]; !ok && len(deploymentStatements) == 1 {
				statements[step.Statement] = deploymentStatements[0]
			}
			for _, statement := range deploymentStatements {
				// Java attaches the listener to 'on' for the insert-only
				// cases (env.addListener("on")) and to 'merge' for the
				// delete-then-update cases (compileDeploy(...).addListener
				// ("merge")); the 'merge' listener is attached but never
				// asserted by the Java execution.
				if statement.Name() == "on" || statement.Name() == "merge" {
					name := statement.Name()
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
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
			payload, err := decodeInfraNWTableOnMergePayload(step)
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
			rows = projectInfraNWTableOnMergeRows(rows, infraNWTableOnMergeSnapshotFields(pinned, stepIndex))
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

// infraNWTableOnMergeIDTUCreateInfra registers the environment-level
// artifacts the Java @public create statements establish: the
// InsertOnlyInfra unique-key named window or primary-key table for the
// insert-only cases, the MyInfra keepall window or primary-key table for
// the delete-then-update cases.
func infraNWTableOnMergeIDTUCreateInfra(env *esper.Environment, caseName string, isTable bool) error {
	if infraNWTableOnMergeIDTUIsInsertOnly(caseName) {
		if isTable {
			_, err := esper.CreateTable(env, "InsertOnlyInfra", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("p0"),
				esper.TableColumnOf[int64]("p1"),
			})
			return err
		}
		schema, err := esper.RegisterMap(env, "InsertOnlyInfra", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		_, err = esper.CreateNamedWindow(env, "InsertOnlyInfra", schema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("p0"))))
		return err
	}
	if isTable {
		_, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("p0"),
			esper.TableColumnOf[int64]("p1"),
		})
		return err
	}
	schema, err := esper.RegisterMap(env, "MyInfra", []esper.FieldSpec{
		esper.FieldDef("p0", reflect.TypeOf("")),
		esper.FieldDef("p1", reflect.TypeOf(int64(0))),
	})
	if err != nil {
		return err
	}
	return createKeepAllNamedWindow(env, "MyInfra", schema)
}

// infraNWTableOnMergeIDTUBuild mirrors the Java deploys: the 'Window' and
// 'create' labels deploy the consumer query the Java iterator reads; 'on'
// deploys the insert-only merge — MergeIntoNamedWindowWhen with a nil match
// (ord 46) or MergeIntoTableWhen keyed on the trigger theString (ords
// 48-51) or a constant never-matching key for the `where 1=2` equivalent
// form (ord 47) — and 'merge' deploys the delete-then-update action chain.
func infraNWTableOnMergeIDTUBuild(env *esper.Environment, caseName string, label string, epl string, isTable bool) (esper.Plan, error) {
	sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
	triggerString := esper.Field[infraNWTableOnMergeBean, string]("theString")
	triggerInt := esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")
	switch label {
	case "Window":
		want := iduInsertOnlyCreateNW
		if isTable {
			want = iduInsertOnlyCreateTable
		}
		if epl != want {
			return esper.Plan{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeIDTUID, label, epl)
		}
		if isTable {
			return env.Build(esper.FromTable(env, "InsertOnlyInfra").Query(
				esper.StatementName("Window"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, "InsertOnlyInfra").Query(
			esper.StatementName("Window"), esper.WithOldStream()))
	case "create":
		want := iduDeleteThenUpdateCreateNW
		if isTable {
			want = iduDeleteThenUpdateCreateTable
		}
		if epl != want {
			return esper.Plan{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeIDTUID, label, epl)
		}
		if isTable {
			return env.Build(esper.FromTable(env, "MyInfra").Query(
				esper.StatementName("create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, "MyInfra").Query(
			esper.StatementName("create"), esper.WithOldStream()))
	case "on":
		if want := iduInsertOnlyMergeEPL(caseName); epl != want {
			return esper.Plan{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeIDTUID, label, epl)
		}
		assignments := []esper.TableAssignment{
			esper.SetColumn("p0", triggerString),
			esper.SetColumn("p1", triggerInt),
		}
		if isTable {
			// Table merges match on primary-key expressions. The plain and
			// column-names forms key on the trigger theString (the
			// simple-table precedent); `where 1=2` has no where-predicate
			// surface, so a constant never-matching key routes every
			// trigger to the not-matched insert (contract-freeze
			// decision).
			keys := []esper.Expr{triggerString}
			if caseName == "insertonly-table-equivalent" {
				keys = []esper.Expr{esper.Literal("")}
			}
			return env.Build(esper.OnEvent(sourceBean).MergeIntoTableWhen("InsertOnlyInfra",
				keys, esper.WhenNotMatchedAny(assignments...)).Query(
				esper.StatementName("on"), esper.WithOldStream()))
		}
		return env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen("InsertOnlyInfra", nil,
			esper.WhenNotMatchedAny(assignments...)).Query(
			esper.StatementName("on"), esper.WithOldStream()))
	case "merge":
		if epl != iduDeleteThenUpdateMergeEPL {
			return esper.Plan{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeIDTUID, label, epl)
		}
		// Java: on SupportBean sb merge MyInfra where theString = p0 when
		// matched then delete then update set p1 = intPrimitive — one
		// matched clause with ordered delete then update actions.
		actions := esper.WhenMatchedActions(
			esper.ThenDelete(esper.Literal(true)),
			esper.ThenUpdate(esper.Literal(true), esper.SetColumn("p1", triggerInt)),
		)
		if isTable {
			return env.Build(esper.OnEvent(sourceBean).MergeIntoTableWhen("MyInfra",
				[]esper.Expr{triggerString}, actions).Query(
				esper.StatementName("merge"), esper.WithOldStream()))
		}
		return env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen("MyInfra",
			esper.Equal[string](esper.NamedWindowField[string]("p0"), triggerString),
			actions).Query(esper.StatementName("merge"), esper.WithOldStream()))
	}
	return esper.Plan{}, fmt.Errorf("%s: unexpected deploy statement %q for case %q",
		infraNWTableOnMergeIDTUID, label, caseName)
}

// infraNWTableOnMergeIDTUBuildFAFInsert builds the fire-and-forget seed
// insert `insert into MyInfra select 'A' as p0, 1 as p1` as an on-demand
// insert (compileExecuteFAFNoResult semantics).
func infraNWTableOnMergeIDTUBuildFAFInsert(env *esper.Environment, isTable bool) (esper.Plan, error) {
	row := esper.InsertValues(esper.Literal("A"), esper.Literal(int64(1)))
	if isTable {
		return env.Build(esper.FromTable(env, "MyInfra").OnDemand().InsertRows(row))
	}
	return env.Build(esper.FromNamedWindow(env, "MyInfra").OnDemand().InsertRows(row))
}

// loadInfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenario enforces the
// strict scenario contract shared by the differential runners: no duplicate
// or unknown JSON fields, pinned metadata, pinned per-case
// runtime/execution/EPL, and a per-op step field whitelist followed by a
// full step-shape pin.
func loadInfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeIDTUID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeIDTUID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeIDTUID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeIDTUID, err)
	}
	if err := requireInfraNWTableOnMergeFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeIDTUID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeIDTUID ||
		metadata.Description != infraNWTableOnMergeIDTUDescription ||
		metadata.JavaCommit != infraNWTableOnMergeIDTUJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeIDTUSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeIDTUID)
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaRuntimes"], infraNWTableOnMergeIDTUJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaNames"], infraNWTableOnMergeIDTUJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaStaticIds"], infraNWTableOnMergeIDTUJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeIDTUCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeIDTUID, len(infraNWTableOnMergeIDTUCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnMergeFields(object,
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
		if definition.Case != infraNWTableOnMergeIDTUCases[index] ||
			definition.Ordinal != infraNWTableOnMergeIDTUOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeIDTUJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeIDTUJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeIDTUCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeIDTUCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeIDTUID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeIDTUID)
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
			if err := requireInfraNWTableOnMergeFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnMergeFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnMergeFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnMergeFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnMergePayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnMergeFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnMergeFields(object, "op", "case"); err != nil {
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
		for _, name := range infraNWTableOnMergeIDTUCases {
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
	if err := validateInfraNWTableOnMergeIDTURawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableOnMergeIDTUCaseEPLs pins the first deploy EPL of each case,
// the value carried by the scenario cases[] metadata.
var infraNWTableOnMergeIDTUCaseEPLs = []string{
	iduInsertOnlyCreateNW,
	iduInsertOnlyCreateTable,
	iduInsertOnlyCreateTable,
	iduInsertOnlyCreateTable,
	iduInsertOnlyCreateTable,
	iduInsertOnlyCreateTable,
	iduDeleteThenUpdateCreateNW,
	iduDeleteThenUpdateCreateTable,
}

var infraNWTableOnMergeIDTUCaseObservations = []string{
	"listener+iterator; the insert(p0, p1) column-names insert-only on-merge compiled through the soda object-model round-trip over the InsertOnlyInfra unique-key named window",
	"listener+iterator; on-merge with the equivalent where 1=2 / when not matched then insert form over the InsertOnlyInfra primary-key table",
	"listener+iterator; plain insert-only on-merge (bare merge ... insert select) over the InsertOnlyInfra primary-key table",
	"listener+iterator; insert-only on-merge with an explicit insert(p0, p1) column list over the InsertOnlyInfra primary-key table",
	"listener+iterator; the plain insert-only on-merge compiled through the soda object-model round-trip over the InsertOnlyInfra primary-key table (observably identical EPL to insertonly-table)",
	"listener+iterator; the insert(p0, p1) column-names insert-only on-merge compiled through the soda object-model round-trip over the InsertOnlyInfra primary-key table",
	"listener+iterator; delete-then-update multi-action merge over the MyInfra keepall named window seeded {A,1} by fire-and-forget insert: the update wins and the iterator reads {A,10}",
	"listener+iterator; delete-then-update multi-action merge over the MyInfra primary-key table seeded {A,1} by fire-and-forget insert: the delete wins and the iterator is empty",
}

// validateInfraNWTableOnMergeIDTURawSteps pins the complete step sequence
// per case against the raw JSON objects: deploy statements with byte-exact
// EPL, deployed markers, send event types with canonical payloads, snapshot
// reads, and undeploy-all terminators.
func validateInfraNWTableOnMergeIDTURawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeIDTUCases {
		want, ok := infraNWTableOnMergeIDTUCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeIDTUID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeIDTUID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeIDTUID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeIDTUID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeIDTUStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeIDTUID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeIDTUID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeIDTUID)
	}
	return nil
}

// infraNWTableOnMergeIDTUStepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableOnMergeIDTUStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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

// iduInsertOnlyCaseSteps renders the pinned step sequence of one
// InfraInsertOnly case (lines 477-517): the Window and 'on' deploys with
// deployed markers, the E1 send and snapshot, the E2 send and snapshot
// across the milestone(0) no-op, and undeploy-all.
func iduInsertOnlyCaseSteps(caseName string) []string {
	create := iduInsertOnlyCreateNW
	if infraNWTableOnMergeIDTUIsTable(caseName) {
		create = iduInsertOnlyCreateTable
	}
	return []string{
		"deploy:Window:" + create,
		"deployed:Window",
		"deploy:on:" + iduInsertOnlyMergeEPL(caseName),
		"deployed:on",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		"snapshot:Window:any:p0,p1",
		`send:SupportBean:{"intPrimitive":2,"theString":"E2"}`,
		"snapshot:Window:any:p0,p1",
		"undeploy-all",
	}
}

// iduDeleteThenUpdateCaseSteps renders the pinned step sequence of one
// InfraDeleteThenUpdate case (lines 311-342): the 'create' and 'merge'
// deploys with deployed markers, the fire-and-forget seed insert (no
// marker), the ordered {A,1} snapshot, the SupportBean("A",10) send, the
// divergent second snapshot and undeploy-all.
func iduDeleteThenUpdateCaseSteps(caseName string) []string {
	create := iduDeleteThenUpdateCreateNW
	if infraNWTableOnMergeIDTUIsTable(caseName) {
		create = iduDeleteThenUpdateCreateTable
	}
	return []string{
		"deploy:create:" + create,
		"deployed:create",
		"deploy:merge:" + iduDeleteThenUpdateMergeEPL,
		"deployed:merge",
		"deploy:FafInsert:" + iduDeleteThenUpdateFafInsert,
		"snapshot:create:ordered:p0,p1",
		`send:SupportBean:{"intPrimitive":10,"theString":"A"}`,
		"snapshot:create:ordered:p0,p1",
		"undeploy-all",
	}
}

// infraNWTableOnMergeIDTUCaseSteps pins the exact op sequence per case:
// deploy statements with byte-exact EPL, deployed markers, send event types
// with canonical payloads, snapshot reads, and undeploy-all terminators.
var infraNWTableOnMergeIDTUCaseSteps = map[string][]string{
	"insertonly-nw-soda-colnames":    iduInsertOnlyCaseSteps("insertonly-nw-soda-colnames"),
	"insertonly-table-equivalent":    iduInsertOnlyCaseSteps("insertonly-table-equivalent"),
	"insertonly-table":               iduInsertOnlyCaseSteps("insertonly-table"),
	"insertonly-table-colnames":      iduInsertOnlyCaseSteps("insertonly-table-colnames"),
	"insertonly-table-soda":          iduInsertOnlyCaseSteps("insertonly-table-soda"),
	"insertonly-table-soda-colnames": iduInsertOnlyCaseSteps("insertonly-table-soda-colnames"),
	"deletethenupdate-nw":            iduDeleteThenUpdateCaseSteps("deletethenupdate-nw"),
	"deletethenupdate-table":         iduDeleteThenUpdateCaseSteps("deletethenupdate-table"),
}
