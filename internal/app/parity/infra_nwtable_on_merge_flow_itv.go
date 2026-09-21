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
	infraNWTableOnMergeFlowITVID          = "infra-nwtable-on-merge-flow-itv"
	infraNWTableOnMergeFlowITVDescription = "InfraNWTableOnMerge ordinals 32-39: InfraFlow wires a filtered SupportBean insert-into feeder, an unconditional SupportBean_A delete-all trigger and a four-branch on-merge (matched delete on intPrimitive<0, matched reset on intPrimitive=0, matched fallback update accumulating intBoxed, not-matched insert) over the MyMergeInfra unique-key named window or primary-key table, runs the assertion flow twice across an undeploy/redeploy of the merge module, then exercises a wildcard merge tail and an ambiguous-columns module; InfraInnerTypeAndVariable merges MyEventSchema events into the MyInfraITV keepall named window or primary-key table under the tri-state myvar variable selecting among three not-matched insert branches with a nested-fragment c2 column and a matched-delete, over OBJECTARRAY, MAP and DEFAULT representations (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeFlowITVJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeFlowITVSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	// Verbatim transcriptions of InfraNWTableOnMerge lines 538-586
	// (InfraFlow) and 906-925 (InfraInnerTypeAndVariable). The merge EPLs
	// carry no trailing semicolon; the ambiguous-columns module ends with
	// ";\n"-free newline-terminated statements exactly as the Java string
	// concatenation produces them.
	infraNWTableOnMergeFlowITVCreateNW  = "@Name('Window') @public create window MyMergeInfra#unique(theString) as SupportBean"
	infraNWTableOnMergeFlowITVCreateTbl = "@Name('Window') @public create table MyMergeInfra (theString string primary key, intPrimitive int, intBoxed int)"
	infraNWTableOnMergeFlowITVInsert    = "@Name('Insert') insert into MyMergeInfra select theString, intPrimitive, intBoxed from SupportBean(boolPrimitive)"
	infraNWTableOnMergeFlowITVDelete    = "@Name('Delete') on SupportBean_A delete from MyMergeInfra"

	infraNWTableOnMergeFlowITVMergeHead = "@Name('Merge') on SupportBean(boolPrimitive=false) as up " +
		"merge MyMergeInfra as mv " +
		"where mv.theString=up.theString " +
		"when matched and up.intPrimitive<0 then " +
		"delete " +
		"when matched and up.intPrimitive=0 then " +
		"update set intPrimitive=0, intBoxed=0 " +
		"when matched then " +
		"update set intPrimitive=up.intPrimitive, intBoxed=up.intBoxed+mv.intBoxed " +
		"when not matched then " +
		"insert select "
	infraNWTableOnMergeFlowITVMergeNW    = infraNWTableOnMergeFlowITVMergeHead + "*"
	infraNWTableOnMergeFlowITVMergeTable = infraNWTableOnMergeFlowITVMergeHead + "theString, intPrimitive, intBoxed"

	infraNWTableOnMergeFlowITVWildHead = "@name('Merge') on SupportBean(boolPrimitive = false) as up " +
		"merge MyMergeInfra as mv " +
		"where mv.theString = up.theString " +
		"when not matched then " +
		"insert select "
	infraNWTableOnMergeFlowITVWildNW    = infraNWTableOnMergeFlowITVWildHead + "up.*"
	infraNWTableOnMergeFlowITVWildTable = infraNWTableOnMergeFlowITVWildHead + "theString, intPrimitive, intBoxed"

	infraNWTableOnMergeFlowITVModuleHead = "create schema TypeOne (id long, mylong long, mystring long);\n"
	infraNWTableOnMergeFlowITVModuleNW   = "@public create window MyInfraTwo#unique(id) as select * from TypeOne;\n"
	infraNWTableOnMergeFlowITVModuleTbl  = "@public create table MyInfraTwo (id long, mylong long, mystring long);\n"
	infraNWTableOnMergeFlowITVModuleTail = "on TypeOne as t1 merge MyInfraTwo nm where nm.id = t1.id\n" +
		"  when not matched and mystring = 0 then insert select *\n" +
		"  when not matched then insert (id, mylong, mystring) select 0L, 0L, 0L\n"

	infraNWTableOnMergeFlowITVCreateVar = "@name('createvar') @public create variable boolean myvar"
	infraNWTableOnMergeFlowITVMerge     = "@name('Merge') on MyEventSchema me " +
		"merge MyInfraITV mw " +
		"where me.col1 = mw.c1 " +
		" when not matched and myvar then " +
		"  insert select col1 as c1, col2 as c2 " +
		" when not matched and myvar = false then " +
		"  insert select 'A' as c1, null as c2 " +
		" when not matched and myvar is null then " +
		"  insert select 'B' as c1, me.col2 as c2 " +
		" when matched then " +
		"  delete"
)

var (
	infraNWTableOnMergeFlowITVJavaSources = []string{
		infraNWTableOnMergeFlowITVSource,
	}
	infraNWTableOnMergeFlowITVJavaRuntimeIDs = []string{
		"java-runtime-403bba8c6b29e32b1a8f",
		"java-runtime-ae05c015767242106de7",
		"java-runtime-3303d0922bd2d722fa3c",
		"java-runtime-f20972a347aabfcc0260",
		"java-runtime-484a8e636d3734b87673",
		"java-runtime-76d16e1334c83e6d4002",
		"java-runtime-462d190f20742a0c266d",
		"java-runtime-8495f57749c2105b15a8",
	}
	infraNWTableOnMergeFlowITVJavaExecutions = []string{
		"InfraFlow{namedWindow=true}",
		"InfraFlow{namedWindow=false}",
		"InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=OBJECTARRAY}",
		"InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=OBJECTARRAY}",
		"InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=MAP}",
		"InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=MAP}",
		"InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=DEFAULT}",
		"InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=DEFAULT}",
	}
	infraNWTableOnMergeFlowITVJavaStaticIDs = []string{
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
		"java-097f9b8edb46959e0763",
	}
	infraNWTableOnMergeFlowITVJavaFlags = []string{"OBSERVEROPS"}
	infraNWTableOnMergeFlowITVCases     = []string{
		"flow-nw",
		"flow-table",
		"itv-nw-objectarray",
		"itv-table-objectarray",
		"itv-nw-map",
		"itv-table-map",
		"itv-nw-default",
		"itv-table-default",
	}
	infraNWTableOnMergeFlowITVOrdinals = []int{32, 33, 34, 35, 36, 37, 38, 39}
)

func infraNWTableOnMergeFlowITVIsFlow(caseName string) bool {
	return strings.HasPrefix(caseName, "flow-")
}

func infraNWTableOnMergeFlowITVIsTable(caseName string) bool {
	return caseName == "flow-table" || strings.Contains(caseName, "-table-")
}

// infraNWTableOnMergeFlowITVRep extracts the event representation suffix of
// an itv case name (objectarray, map or default). Java DEFAULT resolves to
// the map underlying (EventUnderlyingType.getDefault() == MAP and
// EventRepresentationChoice.isMapEvent() covers DEFAULT), so the Go runner
// registers map schemas and sends map records for both map and default.
func infraNWTableOnMergeFlowITVRep(caseName string) string {
	switch {
	case strings.HasSuffix(caseName, "-objectarray"):
		return "objectarray"
	case strings.HasSuffix(caseName, "-map"):
		return "map"
	case strings.HasSuffix(caseName, "-default"):
		return "default"
	}
	return ""
}

// infraNWTableOnMergeFlowITVAnnotation renders the @EventRepresentation
// annotation prefix the Java eventRepresentationEnum.getAnnotationText
// produces: a trailing space for OBJECTARRAY/MAP and a single leading space
// for DEFAULT's empty annotation text.
func infraNWTableOnMergeFlowITVAnnotation(rep string) string {
	switch rep {
	case "objectarray":
		return "@EventRepresentation('objectarray')"
	case "map":
		return "@EventRepresentation('map')"
	}
	return ""
}

// infraNWTableOnMergeFlowITVSchemaEPL renders the verbatim two-statement
// schema module of InfraInnerTypeAndVariable (lines 907-909).
func infraNWTableOnMergeFlowITVSchemaEPL(rep string) string {
	annotation := infraNWTableOnMergeFlowITVAnnotation(rep)
	prefix := annotation
	if prefix != "" {
		prefix += " "
	} else {
		prefix = " "
	}
	return prefix + "@public create schema MyInnerSchema(in1 string, in2 int);\n" +
		prefix + "@public @buseventtype @public create schema MyEventSchema(col1 string, col2 MyInnerSchema)"
}

// infraNWTableOnMergeFlowITVInfraEPL renders the verbatim create statement
// for the MyInfraITV keepall named window or primary-key table (lines
// 911-914).
func infraNWTableOnMergeFlowITVInfraEPL(caseName string) string {
	if infraNWTableOnMergeFlowITVIsTable(caseName) {
		return "@public create table MyInfraITV as (c1 string primary key, c2 MyInnerSchema)"
	}
	annotation := infraNWTableOnMergeFlowITVAnnotation(infraNWTableOnMergeFlowITVRep(caseName))
	prefix := annotation
	if prefix != "" {
		prefix += " "
	} else {
		prefix = " "
	}
	return prefix + "@public create window MyInfraITV#keepall as (c1 string, c2 MyInnerSchema)"
}

// infraNWTableOnMergeFlowITVModuleEPL renders the verbatim three-statement
// ambiguous-columns module of InfraFlow (lines 578-584).
func infraNWTableOnMergeFlowITVModuleEPL(isTable bool) string {
	if isTable {
		return infraNWTableOnMergeFlowITVModuleHead +
			infraNWTableOnMergeFlowITVModuleTbl +
			infraNWTableOnMergeFlowITVModuleTail
	}
	return infraNWTableOnMergeFlowITVModuleHead +
		infraNWTableOnMergeFlowITVModuleNW +
		infraNWTableOnMergeFlowITVModuleTail
}

// infraNWTableOnMergeFlowITVLabels lists the statement labels that receive
// deployed markers per case, in EPL order: flow cases deploy Window, Insert,
// Delete and Merge separately plus the ambiguous module's three statements;
// itv cases bind the schema module's two statements positionally, then the
// infra consumer, the createvar variable and the Merge statement (schema,
// infra and createvar artifacts are environment-level on the Go side and
// have no deployed statement).
func infraNWTableOnMergeFlowITVLabels(caseName string) []string {
	if infraNWTableOnMergeFlowITVIsFlow(caseName) {
		return []string{"Window", "Insert", "Delete", "Merge", "schema-typeone", "infra-two", "merge-two"}
	}
	return []string{"schema-inner", "schema-event", "infra", "createvar", "Merge"}
}

// runInfraNWTableOnMergeFlowITVScenario replays the eight InfraNWTableOnMerge
// executions: each case deploys the pinned statements, sends the pinned
// events, and records deployed markers, 'Window'/'Merge' listener batches
// (flow cases only), iterator snapshots and set-variable markers in Java's
// observable order.
func runInfraNWTableOnMergeFlowITVScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeFlowITVCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeFlowITVCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeFlowITVID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeFlowITVID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeFlowITVCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnDeleteA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnMergeFlowITVIsTable(caseName)
	if err := infraNWTableOnMergeFlowITVCreateInfra(env, caseName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeFlowITVJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	if !infraNWTableOnMergeFlowITVIsFlow(caseName) {
		// Java's `create variable boolean myvar` initializes the variable
		// to null; the Go registration seeds false, so the runner sets the
		// initial null explicitly before the merge deploys (the
		// trigger_inner_type_variable_test.go mirror does the same).
		if err := engine.SetVariable(ctx, "myvar", nil); err != nil {
			return compat.Trace{}, err
		}
	}

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeFlowITVID}
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
	// Schema, infra and createvar labels are environment-level on the Go
	// side; every label that receives a deployed marker is known up front.
	knownLabels := map[string]bool{}
	for _, label := range infraNWTableOnMergeFlowITVLabels(caseName) {
		knownLabels[label] = true
	}
	pinned := infraNWTableOnMergeFlowITVCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plans, err := infraNWTableOnMergeFlowITVBuild(env, caseName, step.Statement, step.Epl, isTable)
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
					// The ambiguous module's merge is unnamed in Java;
					// bind the deployment's single statement to the
					// step label.
					statements[bound.label] = deploymentStatements[0]
				}
				for _, statement := range deploymentStatements {
					// Java attaches the listener only for the flow
					// cases' 'Window' and 'Merge' statements
					// (compileDeploy(epl).addListener(...)); the itv
					// cases are iterator-only per the scenario
					// contract.
					if infraNWTableOnMergeFlowITVIsFlow(caseName) &&
						(statement.Name() == "Window" || statement.Name() == "Merge") {
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
			payload, err := decodeInfraNWTableOnMergeFlowITVPayload(step, caseName)
			if err != nil {
				return compat.Trace{}, err
			}
			switch typed := payload.(type) {
			case []any:
				// OBJECTARRAY MyEventSchema events send positionally,
				// mirroring sendEventObjectArray.
				err = engine.SendObjectArray(ctx, step.EventType, typed)
			case map[string]any:
				// MAP and DEFAULT MyEventSchema events send as
				// name/value records, mirroring sendEventMap.
				err = engine.SendRecord(ctx, step.EventType, typed)
			default:
				err = engine.Send(ctx, step.EventType, payload)
			}
			if err != nil {
				return compat.Trace{}, err
			}
		case "set-variable":
			// Mirrors env.runtime().getVariableService()
			// .setVariableValue(env.deploymentId("createvar"), "myvar",
			// value): the Go variable is environment-scoped, so the
			// statement label only keys the emitted record.
			var assigned *bool
			if len(step.Payload) > 0 && string(step.Payload) != "null" {
				var value bool
				if err := json.Unmarshal(step.Payload, &value); err != nil {
					return compat.Trace{}, fmt.Errorf("set-variable payload: %w", err)
				}
				assigned = &value
			}
			var value any
			if assigned != nil {
				value = *assigned
			}
			if err := engine.SetVariable(ctx, step.Name, value); err != nil {
				return compat.Trace{}, fmt.Errorf("set-variable %q: %w", step.Name, err)
			}
			sequence[step.Statement+":set-variable"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "set-variable",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":set-variable"],
				Time:      "1970-01-01T00:00:00Z",
				Name:      step.Name,
				Value:     value,
			})
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return compat.Trace{}, err
			}
			rows := projectInfraNWTableOnMergeFlowITVRows(result.Batch.New, infraNWTableOnMergeFlowITVSnapshotFields(pinned, stepIndex))
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
			// Mirrors env.undeployModuleContaining: the label resolves
			// the deployment that owns the statement.
			deployment, ok := labelDeployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("no deployment for statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, err
			}
			delete(labelDeployments, step.Statement)
			delete(statements, step.Statement)
			// Drop the undeployed deployment from the cleanup list so the
			// final undeployAll-equivalent loop does not re-undeploy it.
			for index, candidate := range deployments {
				if candidate == deployment {
					deployments = append(deployments[:index], deployments[index+1:]...)
					break
				}
			}
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

// infraNWTableOnMergeFlowITVCreateInfra registers the environment-level
// artifacts the Java @public create statements establish: the MyMergeInfra
// unique-key window or primary-key table plus the TypeOne schema and
// MyInfraTwo infra for the ambiguous module, or the MyInnerSchema/
// MyEventSchema schemas, the MyInfraITV keepall window or primary-key table
// and the myvar variable. Java creates MyInfraTwo inside the module deploy;
// registering it up front is unobservable because the module never receives
// TypeOne events.
func infraNWTableOnMergeFlowITVCreateInfra(env *esper.Environment, caseName string, isTable bool) error {
	if infraNWTableOnMergeFlowITVIsFlow(caseName) {
		if isTable {
			if _, err := esper.CreateTable(env, "MyMergeInfra", []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("theString"),
				esper.TableColumnOf[int64]("intPrimitive"),
				esper.TableColumnOf[int64]("intBoxed"),
			}); err != nil {
				return err
			}
		} else {
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return fmt.Errorf("SupportBean schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyMergeInfra", schema,
				esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("theString")))); err != nil {
				return err
			}
		}
		if _, err := esper.RegisterMap(env, "TypeOne", []esper.FieldSpec{
			esper.FieldDef("id", reflect.TypeOf(int64(0))),
			esper.FieldDef("mylong", reflect.TypeOf(int64(0))),
			esper.FieldDef("mystring", reflect.TypeOf(int64(0))),
		}); err != nil {
			return err
		}
		if isTable {
			_, err := esper.CreateTable(env, "MyInfraTwo", []esper.TableColumn{
				esper.TableColumnOf[int64]("id"),
				esper.TableColumnOf[int64]("mylong"),
				esper.TableColumnOf[int64]("mystring"),
			})
			return err
		}
		typeOneSchema, ok := env.Schema("TypeOne")
		if !ok {
			return fmt.Errorf("TypeOne schema is missing")
		}
		_, err := esper.CreateNamedWindow(env, "MyInfraTwo", typeOneSchema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("id"))))
		return err
	}

	rep := infraNWTableOnMergeFlowITVRep(caseName)
	var innerSchema esper.Schema
	var eventSchema esper.Schema
	var infraSchema esper.Schema
	var err error
	if rep == "objectarray" {
		innerSchema, err = esper.RegisterObjectArray(env, "MyInnerSchema", []esper.FieldSpec{
			esper.FieldDef("in1", reflect.TypeOf("")),
			esper.FieldDef("in2", reflect.TypeOf(int(0))),
		})
		if err != nil {
			return err
		}
		eventSchema, err = esper.RegisterObjectArray(env, "MyEventSchema", []esper.FieldSpec{
			esper.FieldDef("col1", reflect.TypeOf("")),
			esper.FieldDef("col2", reflect.TypeOf([]any{})),
		}, esper.WithNestedPropertySchema("col2", innerSchema))
		if err != nil {
			return err
		}
		if !isTable {
			// Java registers the MyInfraITV event type only for the named
			// window variant; the table variant declares a table of the
			// same name and create-table rejects the schema collision.
			infraSchema, err = esper.RegisterObjectArray(env, "MyInfraITV", []esper.FieldSpec{
				esper.FieldDef("c1", reflect.TypeOf("")),
				esper.FieldDef("c2", reflect.TypeOf([]any{})),
			}, esper.WithNestedPropertySchema("c2", innerSchema))
			if err != nil {
				return err
			}
		}
	} else {
		// MAP and DEFAULT both resolve to the map underlying.
		innerSchema, err = esper.RegisterMap(env, "MyInnerSchema", []esper.FieldSpec{
			esper.FieldDef("in1", reflect.TypeOf("")),
			esper.FieldDef("in2", reflect.TypeOf(int(0))),
		})
		if err != nil {
			return err
		}
		eventSchema, err = esper.RegisterMap(env, "MyEventSchema", []esper.FieldSpec{
			esper.FieldDef("col1", reflect.TypeOf("")),
			esper.FieldDef("col2", reflect.TypeOf(map[string]any{})),
		}, esper.WithNestedPropertySchema("col2", innerSchema))
		if err != nil {
			return err
		}
		if !isTable {
			infraSchema, err = esper.RegisterMap(env, "MyInfraITV", []esper.FieldSpec{
				esper.FieldDef("c1", reflect.TypeOf("")),
				esper.FieldDef("c2", reflect.TypeOf(map[string]any{})),
			}, esper.WithNestedPropertySchema("c2", innerSchema))
			if err != nil {
				return err
			}
		}
	}
	_ = eventSchema
	if isTable {
		c2Type := reflect.TypeOf(map[string]any{})
		if rep == "objectarray" {
			c2Type = reflect.TypeOf([]any{})
		}
		if _, err := esper.CreateTable(env, "MyInfraITV", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("c1"),
			{Name: "c2", Type: c2Type, Nested: innerSchema},
		}); err != nil {
			return err
		}
	} else {
		if err := createKeepAllNamedWindow(env, "MyInfraITV", infraSchema); err != nil {
			return err
		}
	}
	// @name('createvar') @public create variable boolean myvar — the Go
	// registration seeds the boolean zero value; runCase sets the initial
	// null before the merge deploys, matching Java's uninitialized Boolean.
	return env.RegisterVariable("myvar", false)
}

// infraNWTableOnMergeFlowITVBoundPlan pairs one statement label with the Go
// plan that produces the deployed statement.
type infraNWTableOnMergeFlowITVBoundPlan struct {
	label string
	plan  esper.Plan
}

// infraNWTableOnMergeFlowITVBuild mirrors the Java deploys: the @public
// create statements are environment-level on the Go side, so the Window and
// infra labels deploy the consumer query the Java iterator reads. The flow
// merge deploys the four-branch merge or the wildcard tail depending on the
// step's pinned EPL; the ambiguous module deploys only its merge plan (the
// schema and infra statements are environment-level). The itv schema and
// createvar deploys produce no plan — their artifacts are registered in
// createInfra — while the itv merge deploys the tri-state merge.
func infraNWTableOnMergeFlowITVBuild(env *esper.Environment, caseName string, label string, epl string, isTable bool) ([]infraNWTableOnMergeFlowITVBoundPlan, error) {
	sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
	theString := esper.Field[infraNWTableOnMergeBean, string]("theString")
	intPrimitive := esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")
	intBoxed := esper.Field[infraNWTableOnMergeBean, *int64]("intBoxed")
	// intBoxedColumn dereferences the boxed Integer for the int64 table
	// columns: the table writer rejects a *int64 value, while the named
	// window's SupportBean schema keeps the pointer.
	intBoxedColumn := esper.Cast[*int64, int64](intBoxed)
	boolPrimitive := esper.Field[infraNWTableOnMergeBean, bool]("boolPrimitive")
	trueSource := sourceBean.Filter(esper.Equal[bool](boolPrimitive, esper.Literal(true)))
	falseSource := sourceBean.Filter(esper.Equal[bool](boolPrimitive, esper.Literal(false)))
	sourceA := esper.From[infraNWTableOnDeleteA](env, "SupportBean_A")

	if infraNWTableOnMergeFlowITVIsFlow(caseName) {
		switch label {
		case "Window":
			// The consumer query stands in for the Java create
			// statement: its iterator reads the window or table rows
			// and its subscription mirrors the 'Window' listener.
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.FromTable(env, "MyMergeInfra").Query(
					esper.StatementName("Window"), esper.WithOldStream()))
			} else {
				plan, err = env.Build(esper.FromNamedWindow(env, "MyMergeInfra").Query(
					esper.StatementName("Window"), esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeFlowITVBoundPlan{{label: "Window", plan: plan}}, nil
		case "Insert":
			// insert into MyMergeInfra select theString, intPrimitive,
			// intBoxed from SupportBean(boolPrimitive) — a filtered
			// insert-into feeder.
			assignments := []esper.TableAssignment{
				esper.SetColumn("theString", theString),
				esper.SetColumn("intPrimitive", intPrimitive),
				esper.SetColumn("intBoxed", intBoxed),
			}
			if isTable {
				assignments[2] = esper.SetColumn("intBoxed", intBoxedColumn)
			} else {
				// The three-column select leaves the window's other
				// SupportBean fields at their schema defaults; Java's
				// char default is '\0' while Go's string zero is "",
				// so the feeder writes the Java default explicitly.
				assignments = append(assignments,
					esper.SetColumn("charPrimitive", esper.Literal("\u0000")))
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(trueSource).InsertIntoTable("MyMergeInfra",
					assignments...).Query(esper.StatementName("Insert")))
			} else {
				plan, err = env.Build(esper.OnEvent(trueSource).InsertIntoNamedWindow("MyMergeInfra",
					assignments...).Query(esper.StatementName("Insert")))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeFlowITVBoundPlan{{label: "Insert", plan: plan}}, nil
		case "Delete":
			// on SupportBean_A delete from MyMergeInfra — an
			// unconditional delete-all trigger.
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(sourceA).DeleteAllFromTable("MyMergeInfra").Query(
					esper.StatementName("Delete")))
			} else {
				plan, err = env.Build(esper.OnEvent(sourceA).DeleteAllFromNamedWindow("MyMergeInfra").Query(
					esper.StatementName("Delete")))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeFlowITVBoundPlan{{label: "Delete", plan: plan}}, nil
		case "Merge":
			var clauses []esper.TableMergeClause
			if epl == infraNWTableOnMergeFlowITVWildNW || epl == infraNWTableOnMergeFlowITVWildTable {
				// Wildcard tail: when not matched then insert select
				// up.* (named window) or the explicit three columns
				// (table).
				if isTable {
					clauses = []esper.TableMergeClause{
						esper.WhenNotMatchedAny(
							esper.SetColumn("theString", theString),
							esper.SetColumn("intPrimitive", intPrimitive),
							esper.SetColumn("intBoxed", intBoxedColumn),
						),
					}
				} else {
					clauses = []esper.TableMergeClause{
						esper.WhenNotMatchedAny(esper.CopyMatchingFields()),
					}
				}
			} else {
				// Four-branch merge: matched delete on
				// intPrimitive<0, matched reset on intPrimitive=0,
				// matched fallback update accumulating intBoxed,
				// not-matched insert.
				var targetIntBoxed esper.Expression[int64]
				if isTable {
					targetIntBoxed = esper.TableField[int64]("intBoxed")
				} else {
					targetIntBoxed = esper.NamedWindowField[int64]("intBoxed")
				}
				var notMatched esper.TableMergeClause
				if isTable {
					notMatched = esper.WhenNotMatchedAny(
						esper.SetColumn("theString", theString),
						esper.SetColumn("intPrimitive", intPrimitive),
						esper.SetColumn("intBoxed", intBoxedColumn),
					)
				} else {
					// insert select * copies every same-named
					// trigger field into the SupportBean-typed
					// window row.
					notMatched = esper.WhenNotMatchedAny(esper.CopyMatchingFields())
				}
				clauses = []esper.TableMergeClause{
					esper.WhenMatchedDelete(esper.Less[int64](intPrimitive, esper.Literal(int64(0)))),
					esper.WhenMatched(esper.Equal[int64](intPrimitive, esper.Literal(int64(0))),
						esper.SetColumn("intPrimitive", esper.Literal(int64(0))),
						esper.SetColumn("intBoxed", esper.Literal(int64(0))),
					),
					esper.WhenMatchedAny(
						esper.SetColumn("intPrimitive", intPrimitive),
						esper.SetColumn("intBoxed", esper.Add[int64](intBoxed, targetIntBoxed)),
					),
					notMatched,
				}
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnEvent(falseSource).MergeIntoTableWhen(
					"MyMergeInfra", []esper.Expr{theString}, clauses...,
				).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			} else {
				match := esper.Equal[string](esper.NamedWindowField[string]("theString"), theString)
				plan, err = env.Build(esper.OnEvent(falseSource).MergeIntoNamedWindowWhen(
					"MyMergeInfra", match, clauses...,
				).Query(esper.StatementName("Merge"), esper.WithOldStream()))
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeFlowITVBoundPlan{{label: "Merge", plan: plan}}, nil
		case "module":
			// on TypeOne as t1 merge MyInfraTwo nm where nm.id = t1.id
			// when not matched and mystring = 0 then insert select *
			// when not matched then insert (id, mylong, mystring)
			// select 0L, 0L, 0L — the ambiguous-columns merge. The
			// table variant is an unkeyed table with a where clause
			// that Go's key-only MergeIntoTableWhen cannot express;
			// the contract pins the nil-keys equivalent because Java
			// never sends TypeOne events.
			sourceTypeOne := esper.FromAny(env, "TypeOne")
			id := esper.Field[any, int64]("id")
			mystring := esper.Field[any, int64]("mystring")
			clauses := []esper.TableMergeClause{
				esper.WhenNotMatched(esper.Equal[int64](mystring, esper.Literal(int64(0))),
					esper.CopyMatchingFields()),
				esper.WhenNotMatchedAny(
					esper.SetColumn("id", esper.Literal(int64(0))),
					esper.SetColumn("mylong", esper.Literal(int64(0))),
					esper.SetColumn("mystring", esper.Literal(int64(0))),
				),
			}
			var plan esper.Plan
			var err error
			if isTable {
				plan, err = env.Build(esper.OnRecord(sourceTypeOne).MergeIntoTableWhen(
					"MyInfraTwo", nil, clauses...).Query())
			} else {
				match := esper.Equal[int64](esper.NamedWindowField[int64]("id"), id)
				plan, err = env.Build(esper.OnRecord(sourceTypeOne).MergeIntoNamedWindowWhen(
					"MyInfraTwo", match, clauses...).Query())
			}
			if err != nil {
				return nil, err
			}
			return []infraNWTableOnMergeFlowITVBoundPlan{{label: "merge-two", plan: plan}}, nil
		}
		return nil, fmt.Errorf("unexpected deploy statement %q for case %q", label, caseName)
	}

	// InfraInnerTypeAndVariable cases.
	sourceEvent := esper.FromAny(env, "MyEventSchema")
	col1 := esper.Field[any, string]("col1")
	myvar := esper.VariableRef[bool]("myvar")
	var col2 esper.Expr
	var nullInner esper.Expr
	switch infraNWTableOnMergeFlowITVRep(caseName) {
	case "objectarray":
		col2 = esper.Field[any, []any]("col2")
		nullInner = esper.NullLiteral[[]any]()
	default:
		col2 = esper.Field[any, map[string]any]("col2")
		nullInner = esper.NullLiteral[map[string]any]()
	}
	switch label {
	case "schema", "createvar":
		// The create-schema statements and the create-variable
		// statement are environment-level on the Go side; the deploy
		// step only emits the pinned deployed markers.
		return nil, nil
	case "infra":
		// The consumer query stands in for the Java create statement:
		// its iterator reads the MyInfraITV rows for the snapshot
		// assertions.
		var plan esper.Plan
		var err error
		if isTable {
			plan, err = env.Build(esper.FromTable(env, "MyInfraITV").Query(
				esper.StatementName("infra"), esper.WithOldStream()))
		} else {
			plan, err = env.Build(esper.FromNamedWindow(env, "MyInfraITV").Query(
				esper.StatementName("infra"), esper.WithOldStream()))
		}
		if err != nil {
			return nil, err
		}
		return []infraNWTableOnMergeFlowITVBoundPlan{{label: "infra", plan: plan}}, nil
	case "Merge":
		// @name('Merge') on MyEventSchema me merge MyInfraITV mw where
		// me.col1 = mw.c1 when not matched and myvar then insert
		// select col1 as c1, col2 as c2 when not matched and myvar =
		// false then insert select 'A' as c1, null as c2 when not
		// matched and myvar is null then insert select 'B' as c1,
		// me.col2 as c2 when matched then delete — the tri-state myvar
		// selects among the three not-matched branches.
		clauses := []esper.TableMergeClause{
			esper.WhenNotMatched(myvar,
				esper.SetColumn("c1", col1),
				esper.SetColumn("c2", col2),
			),
			esper.WhenNotMatched(esper.Equal[bool](myvar, esper.Literal(false)),
				esper.SetColumn("c1", esper.Literal("A")),
				esper.SetColumn("c2", nullInner),
			),
			esper.WhenNotMatched(esper.IsNull[bool](myvar),
				esper.SetColumn("c1", esper.Literal("B")),
				esper.SetColumn("c2", col2),
			),
			esper.WhenMatchedDeleteAny(),
		}
		var plan esper.Plan
		var err error
		if isTable {
			plan, err = env.Build(esper.OnRecord(sourceEvent).MergeIntoTableWhen(
				"MyInfraITV", []esper.Expr{col1}, clauses...,
			).Query(esper.StatementName("Merge"), esper.WithOldStream()))
		} else {
			match := esper.Equal[string](esper.NamedWindowField[string]("c1"), col1)
			plan, err = env.Build(esper.OnRecord(sourceEvent).MergeIntoNamedWindowWhen(
				"MyInfraITV", match, clauses...,
			).Query(esper.StatementName("Merge"), esper.WithOldStream()))
		}
		if err != nil {
			return nil, err
		}
		return []infraNWTableOnMergeFlowITVBoundPlan{{label: "Merge", plan: plan}}, nil
	}
	return nil, fmt.Errorf("unexpected deploy statement %q for case %q", label, caseName)
}

// decodeInfraNWTableOnMergeFlowITVPayload validates one send payload and
// builds the event: SupportBean carries theString/intPrimitive/intBoxed/
// boolPrimitive (sendSupportBeanEvent), SupportBean_A carries id, and
// MyEventSchema carries col1 plus the nested col2 fragment in the case's
// representation (sendMyInnerSchemaEvent: positional object array for
// OBJECTARRAY, name/value map for MAP and DEFAULT).
func decodeInfraNWTableOnMergeFlowITVPayload(step compat.Step, caseName string) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString     *string `json:"theString"`
			IntPrimitive  int64   `json:"intPrimitive"`
			IntBoxed      *int64  `json:"intBoxed"`
			BoolPrimitive bool    `json:"boolPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean payload: %w", err)
		}
		event := infraNWTableOnMergeBean{
			IntPrimitive:  payload.IntPrimitive,
			IntBoxed:      payload.IntBoxed,
			BoolPrimitive: payload.BoolPrimitive,
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
	case "MyEventSchema":
		var payload struct {
			Col1 string `json:"col1"`
			Col2 struct {
				In1 string `json:"in1"`
				In2 int    `json:"in2"`
			} `json:"col2"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode MyEventSchema payload: %w", err)
		}
		if infraNWTableOnMergeFlowITVRep(caseName) == "objectarray" {
			return []any{payload.Col1, []any{payload.Col2.In1, payload.Col2.In2}}, nil
		}
		return map[string]any{
			"col1": payload.Col1,
			"col2": map[string]any{"in1": payload.Col2.In1, "in2": payload.Col2.In2},
		}, nil
	}
	return nil, fmt.Errorf("unexpected event type %q", step.EventType)
}

func requireInfraNWTableOnMergeFlowITVFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnMergeFlowITVScenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableOnMergeFlowITVScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeFlowITVID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeFlowITVID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeFlowITVID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeFlowITVID, err)
	}
	if err := requireInfraNWTableOnMergeFlowITVFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeFlowITVID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeFlowITVID ||
		metadata.Description != infraNWTableOnMergeFlowITVDescription ||
		metadata.JavaCommit != infraNWTableOnMergeFlowITVJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeFlowITVSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeFlowITVID)
	}
	if err := validateInfraNWTableOnMergeFlowITVStringArray(root["javaRuntimes"], infraNWTableOnMergeFlowITVJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeFlowITVStringArray(root["javaNames"], infraNWTableOnMergeFlowITVJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeFlowITVStringArray(root["javaStaticIds"], infraNWTableOnMergeFlowITVJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeFlowITVStringArray(root["javaFlags"], infraNWTableOnMergeFlowITVJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeFlowITVCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeFlowITVID, len(infraNWTableOnMergeFlowITVCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnMergeFlowITVFields(object,
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
		if definition.Case != infraNWTableOnMergeFlowITVCases[index] ||
			definition.Ordinal != infraNWTableOnMergeFlowITVOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeFlowITVJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeFlowITVJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeFlowITVCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeFlowITVCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeFlowITVID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeFlowITVID)
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
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				Case      string          `json:"case"`
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnMergeFlowITVPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}, payload.Case); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "set-variable":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case", "statement", "name", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				Name    string          `json:"name"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var value bool
			if err := json.Unmarshal(payload.Payload, &value); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d set-variable payload must be a boolean: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnMergeFlowITVFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnMergeFlowITVRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnMergeFlowITVStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnMergeFlowITVCaseEPLs pins the first deploy EPL of each
// case, the value carried by the scenario cases[] metadata.
var infraNWTableOnMergeFlowITVCaseEPLs = []string{
	infraNWTableOnMergeFlowITVCreateNW,
	infraNWTableOnMergeFlowITVCreateTbl,
	infraNWTableOnMergeFlowITVSchemaEPL("objectarray"),
	infraNWTableOnMergeFlowITVSchemaEPL("objectarray"),
	infraNWTableOnMergeFlowITVSchemaEPL("map"),
	infraNWTableOnMergeFlowITVSchemaEPL("map"),
	infraNWTableOnMergeFlowITVSchemaEPL("default"),
	infraNWTableOnMergeFlowITVSchemaEPL("default"),
}

var infraNWTableOnMergeFlowITVCaseObservations = []string{
	"listener+iterator; filtered insert-into feeder, delete-all trigger and four-branch merge over the MyMergeInfra unique-key named window, run twice across merge-module undeploy/redeploy, then a wildcard merge tail and an ambiguous-columns module",
	"listener+iterator; same flow over the MyMergeInfra primary-key table (the 'Window' listener is attached but never fires for a table)",
	"iterator; tri-state myvar selects among three not-matched insert branches over the MyInfraITV keepall named window with a nested-fragment c2 column, matched deletes, OBJECTARRAY representation",
	"iterator; same tri-state merge over the MyInfraITV primary-key table, OBJECTARRAY representation",
	"iterator; same tri-state merge over the MyInfraITV keepall named window, MAP representation",
	"iterator; same tri-state merge over the MyInfraITV primary-key table, MAP representation",
	"iterator; same tri-state merge over the MyInfraITV keepall named window, DEFAULT representation",
	"iterator; same tri-state merge over the MyInfraITV primary-key table, DEFAULT representation",
}

// validateInfraNWTableOnMergeFlowITVRawSteps pins the complete step
// sequence per case against the raw JSON objects: deploy statements with
// byte-exact EPL, deployed markers, send event types with canonical
// payloads, snapshot reads, set-variable writes and the undeploy/undeploy-all
// terminators.
func validateInfraNWTableOnMergeFlowITVRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeFlowITVCases {
		want, ok := infraNWTableOnMergeFlowITVCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeFlowITVID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeFlowITVID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeFlowITVID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeFlowITVID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeFlowITVStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeFlowITVID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeFlowITVID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeFlowITVID)
	}
	return nil
}

// infraNWTableOnMergeFlowITVStepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because compat.Step
// does not carry the fields array.
func infraNWTableOnMergeFlowITVStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	canonicalPayload := func() (string, error) {
		var payload any
		if err := json.Unmarshal(object["payload"], &payload); err != nil {
			return "", fmt.Errorf("step payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("step payload: %w", err)
		}
		return string(canonical), nil
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
		canonical, err := canonicalPayload()
		if err != nil {
			return "", err
		}
		return "send:" + eventType + ":" + canonical, nil
	case "set-variable":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		name, err := stringField("name")
		if err != nil {
			return "", err
		}
		canonical, err := canonicalPayload()
		if err != nil {
			return "", err
		}
		return "set-variable:" + statement + ":" + name + ":" + canonical, nil
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

// infraNWTableOnMergeFlowITVFlowPass renders the nine send+snapshot pairs
// of runAssertionFlow (lines 592-665): insert-into E1, merge update E1,
// merge insert E2, two merge updates on E2, merge insert E3, the
// intPrimitive=0 reset on E3 and the two matched deletes.
func infraNWTableOnMergeFlowITVFlowPass() []string {
	return []string{
		`send:SupportBean:{"boolPrimitive":true,"intBoxed":200,"intPrimitive":10,"theString":"E1"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":201,"intPrimitive":11,"theString":"E1"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":300,"intPrimitive":13,"theString":"E2"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":301,"intPrimitive":14,"theString":"E2"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":302,"intPrimitive":15,"theString":"E2"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":400,"intPrimitive":40,"theString":"E3"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":1000,"intPrimitive":0,"theString":"E3"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":1000,"intPrimitive":-1,"theString":"E2"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":1000,"intPrimitive":-1,"theString":"E1"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
	}
}

// infraNWTableOnMergeFlowITVFlowSteps renders the pinned step sequence of
// one InfraFlow case: the four setup deploys, two assertion passes across a
// merge undeploy/redeploy with the SupportBean_A clear, the wildcard merge
// tail and the ambiguous-columns module.
func infraNWTableOnMergeFlowITVFlowSteps(isTable bool) []string {
	create := infraNWTableOnMergeFlowITVCreateNW
	merge := infraNWTableOnMergeFlowITVMergeNW
	wild := infraNWTableOnMergeFlowITVWildNW
	if isTable {
		create = infraNWTableOnMergeFlowITVCreateTbl
		merge = infraNWTableOnMergeFlowITVMergeTable
		wild = infraNWTableOnMergeFlowITVWildTable
	}
	steps := []string{
		"deploy:Window:" + create,
		"deployed:Window",
		"deploy:Insert:" + infraNWTableOnMergeFlowITVInsert,
		"deployed:Insert",
		"deploy:Delete:" + infraNWTableOnMergeFlowITVDelete,
		"deployed:Delete",
		"deploy:Merge:" + merge,
		"deployed:Merge",
	}
	steps = append(steps, infraNWTableOnMergeFlowITVFlowPass()...)
	steps = append(steps,
		"undeploy:Merge",
		`send:SupportBean_A:{"id":"A1"}`,
		"deploy:Merge:"+merge,
		"deployed:Merge",
	)
	steps = append(steps, infraNWTableOnMergeFlowITVFlowPass()...)
	steps = append(steps,
		`send:SupportBean_A:{"id":"A2"}`,
		"undeploy:Merge",
		"deploy:Merge:"+wild,
		"deployed:Merge",
		`send:SupportBean:{"boolPrimitive":false,"intBoxed":3,"intPrimitive":2,"theString":"E99"}`,
		"snapshot:Window:any:theString,intPrimitive,intBoxed",
		"deploy:module:"+infraNWTableOnMergeFlowITVModuleEPL(isTable),
		"deployed:schema-typeone",
		"deployed:infra-two",
		"deployed:merge-two",
		"undeploy-all",
	)
	return steps
}

// infraNWTableOnMergeFlowITVITVSteps renders the pinned step sequence of
// one InfraInnerTypeAndVariable case (lines 906-968): the schema module,
// infra and createvar deploys, the merge deploy, the tri-state send/set
// sequence with infra snapshots, and the undeploy/redeploy tail.
func infraNWTableOnMergeFlowITVITVSteps(caseName string) []string {
	return []string{
		"deploy:schema:" + infraNWTableOnMergeFlowITVSchemaEPL(infraNWTableOnMergeFlowITVRep(caseName)),
		"deployed:schema-inner",
		"deployed:schema-event",
		"deploy:infra:" + infraNWTableOnMergeFlowITVInfraEPL(caseName),
		"deployed:infra",
		"deploy:createvar:" + infraNWTableOnMergeFlowITVCreateVar,
		"deployed:createvar",
		"deploy:Merge:" + infraNWTableOnMergeFlowITVMerge,
		"deployed:Merge",
		`send:MyEventSchema:{"col1":"X1","col2":{"in1":"Y1","in2":10}}`,
		"snapshot:infra:any:c1,c2.in1,c2.in2",
		`send:MyEventSchema:{"col1":"B","col2":{"in1":"0","in2":0}}`,
		"snapshot:infra:any:c1,c2.in1,c2.in2",
		"set-variable:createvar:myvar:true",
		`send:MyEventSchema:{"col1":"X2","col2":{"in1":"Y2","in2":11}}`,
		"snapshot:infra:any:c1,c2.in1,c2.in2",
		"set-variable:createvar:myvar:false",
		`send:MyEventSchema:{"col1":"X3","col2":{"in1":"Y3","in2":12}}`,
		"snapshot:infra:any:c1,c2.in1,c2.in2",
		"undeploy:Merge",
		"deploy:Merge:" + infraNWTableOnMergeFlowITVMerge,
		"deployed:Merge",
		"set-variable:createvar:myvar:true",
		`send:MyEventSchema:{"col1":"X4","col2":{"in1":"Y4","in2":11}}`,
		"snapshot:infra:any:c1,c2.in1,c2.in2",
		"undeploy-all",
	}
}

// infraNWTableOnMergeFlowITVCaseSteps pins the exact op sequence per case:
// deploy statements with byte-exact EPL, deployed markers, send event types
// with canonical payloads, snapshot reads, set-variable writes and the
// undeploy/undeploy-all terminators.
var infraNWTableOnMergeFlowITVCaseSteps = map[string][]string{
	"flow-nw":               infraNWTableOnMergeFlowITVFlowSteps(false),
	"flow-table":            infraNWTableOnMergeFlowITVFlowSteps(true),
	"itv-nw-objectarray":    infraNWTableOnMergeFlowITVITVSteps("itv-nw-objectarray"),
	"itv-table-objectarray": infraNWTableOnMergeFlowITVITVSteps("itv-table-objectarray"),
	"itv-nw-map":            infraNWTableOnMergeFlowITVITVSteps("itv-nw-map"),
	"itv-table-map":         infraNWTableOnMergeFlowITVITVSteps("itv-table-map"),
	"itv-nw-default":        infraNWTableOnMergeFlowITVITVSteps("itv-nw-default"),
	"itv-table-default":     infraNWTableOnMergeFlowITVITVSteps("itv-table-default"),
}

// infraNWTableOnMergeFlowITVSnapshotFields extracts the pinned projection
// list for the snapshot at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>"). The case marker occupies
// steps[0], so the pinned index is stepIndex-1.
func infraNWTableOnMergeFlowITVSnapshotFields(pinned []string, stepIndex int) []string {
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

// projectInfraNWTableOnMergeFlowITVRows reduces each result row to the
// pinned assertion fields: the Java iterator assertions read only these
// properties, with dotted paths such as c2.in1 resolving through the
// nested-fragment schema.
func projectInfraNWTableOnMergeFlowITVRows(results []esper.Result, fields []string) []compat.ResultRecord {
	if len(fields) == 0 {
		return nil
	}
	sorted := append([]string(nil), fields...)
	for index := 1; index < len(sorted); index++ {
		for cursor := index; cursor > 0 && sorted[cursor-1] > sorted[cursor]; cursor-- {
			sorted[cursor-1], sorted[cursor] = sorted[cursor], sorted[cursor-1]
		}
	}
	projected := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		row := compat.ResultRecord{Kind: "row", Fields: make(map[string]any, len(sorted))}
		for _, name := range sorted {
			row.Fields[name] = infraNWTableOnMergeFlowITVNormalize(infraNWTableOnMergeFlowITVResultGet(result, name))
		}
		projected = append(projected, row)
	}
	return projected
}

// infraNWTableOnMergeFlowITVResultGet reads one pinned field from a result:
// events resolve dotted paths through the schema's nested-fragment lookup
// (Event.Get("c2.in1")); rows are flat, so a dotted path reads the head
// column and materializes the nested fragment for the tail, mirroring
// Java's event.get("c2.in1") which returns null when c2 itself is null.
func infraNWTableOnMergeFlowITVResultGet(result esper.Result, field string) esper.Value {
	if event, ok := result.Event(); ok {
		return event.Get(field)
	}
	row, ok := result.Row()
	if !ok {
		return esper.Missing()
	}
	head, tail, dotted := strings.Cut(field, ".")
	if !dotted {
		return row.Get(field)
	}
	value := row.Get(head)
	if !value.IsPresent() {
		// Missing propagates; a null head yields null for the nested
		// read exactly like Java's property access on a null fragment.
		return value
	}
	fragment, ok := row.GetFragment(head)
	if !ok {
		return esper.Missing()
	}
	return fragment.Get(tail)
}

// infraNWTableOnMergeFlowITVNormalize renders one value in the trace's
// canonical shape: missing and null (including typed-nil pointers such as a
// null Integer intBoxed) become the tagged state objects, nested events and
// maps recurse, and scalars pass through.
func infraNWTableOnMergeFlowITVNormalize(value esper.Value) any {
	if value.IsMissing() {
		return map[string]any{"state": "missing"}
	}
	if value.IsNull() {
		return map[string]any{"state": "null"}
	}
	raw := value.Any()
	if raw == nil {
		return map[string]any{"state": "null"}
	}
	reflected := reflect.ValueOf(raw)
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Interface:
		if reflected.IsNil() {
			return map[string]any{"state": "null"}
		}
		return reflected.Elem().Interface()
	case reflect.Map, reflect.Slice:
		if reflected.IsNil() {
			return map[string]any{"state": "null"}
		}
	}
	if event, ok := raw.(esper.Event); ok {
		fields := make(map[string]any)
		for _, schemaField := range event.Schema().Fields() {
			fields[schemaField.Name] = infraNWTableOnMergeFlowITVNormalize(event.Get(schemaField.Name))
		}
		return map[string]any{"kind": "row", "fields": fields}
	}
	if events, ok := raw.([]esper.Event); ok {
		rows := make([]any, 0, len(events))
		for _, event := range events {
			fields := make(map[string]any)
			for _, schemaField := range event.Schema().Fields() {
				fields[schemaField.Name] = infraNWTableOnMergeFlowITVNormalize(event.Get(schemaField.Name))
			}
			rows = append(rows, map[string]any{"kind": "row", "fields": fields})
		}
		return rows
	}
	if nested, ok := raw.(map[string]any); ok {
		fields := make(map[string]any, len(nested))
		for name, field := range nested {
			fields[name] = infraNWTableOnMergeFlowITVNormalize(esper.Present(field))
		}
		return fields
	}
	return raw
}
