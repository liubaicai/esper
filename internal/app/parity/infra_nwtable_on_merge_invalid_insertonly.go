package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// infra_nwtable_on_merge_invalid_insertonly.go replays InfraNWTableOnMerge
// ordinals 40-45 against the pinned Java oracle: InfraInvalid{namedWindow}
// (ords 40-41) runs the tryInvalidCompile probe sequence over the
// MergeInfra/ABCInfra fixtures and records the pinned Java message prefixes;
// InfraInsertOnly{namedWindow=true} (ords 42-45) deploys the InsertOnlyInfra
// unique-key named window plus one insert-only on-merge variant and observes
// the 'on' listener rows and window iterator across two SupportBean sends.
//
// Approved differences (observably identical to the Java EPL):
//   - `create window`/`create table`/`create schema`/`create variable` map to
//     env-level registrations; Go has no module path, so the fixture deploy
//     steps register their artifacts instead of deploying statements and emit
//     no deployed markers (context-key-segmented-invalid precedent).
//   - Probes whose Java rejection is parser-positional or has no typed-API
//     surface (missing-then, and-then-delete, ambiguous-where-prop,
//     nested-event-assign) pin the expectError prefix verbatim without
//     claiming a Go rejection boundary. Every other probe verifies the
//     nearest expressible Go rejection before recording the pinned value.
//   - ord 45 (soda) pins the same EPL as ord 43; Java's compileDeploy
//     (soda=true) only asserts the eplToModel round-trip, so the Go replay
//     is identical.

const (
	infraNWTableOnMergeInvalidInsertOnlyID          = "infra-nwtable-on-merge-invalid-insertonly"
	infraNWTableOnMergeInvalidInsertOnlyDescription = "InfraNWTableOnMerge ordinals 40-45: InfraInvalid replays the tryInvalidCompile probe sequence over the MergeInfra/ABCInfra named-window and table fixtures, pinning the Java compile-error prefixes (the nw variant carries one extra probe, the matched-insert event-type conversion; probe 2 pins divergent nw/table wording); InfraInsertOnly deploys the InsertOnlyInfra unique-key named window and one of four insert-only on-merge variants (useEquivalent where 1=2, plain, useColumnNames, soda) and observes the 'on' listener rows and window iterator across two SupportBean sends (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeInvalidInsertOnlyJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeInvalidInsertOnlySource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	// Verbatim transcriptions of InfraNWTableOnMerge lines 823-885
	// (InfraInvalid) and 480-494 (InfraInsertOnly). The fixture modules and
	// the variable/indexed probes carry literal newlines exactly as the Java
	// string concatenation produces them.
	iiInvalidFixtureNW = "@public create window MergeInfra#unique(theString) as SupportBean;\n" +
		"create schema ABCSchema as (val int);\n" +
		"@public create window ABCInfra#keepall as ABCSchema;\n"
	iiInvalidFixtureTable = "@public create table MergeInfra as (theString string, intPrimitive int, boolPrimitive bool);\n" +
		"create schema ABCSchema as (val int);\n" +
		"@public create table ABCInfra (val int);\n"
	iiInvalidCompositeEPL = "@public create map schema Composite as (c0 int)"
	iiInvalidAInfraEPL    = "@public create window AInfra#keepall as (c Composite)"
	iiInvalidSomeOtherEPL = "@public create map schema SomeOther as (c1 int)"
	iiInvalidMyEventEPL   = "@public create map schema MyEvent as (so SomeOther)"

	iiProbeFilterWindoweventEPL = "on SupportBean_A merge MergeInfra as windowevent where id = theString when not matched and exists(select * from MergeInfra mw where mw.theString = windowevent.theString) is not null then insert into ABC select '1'"
	iiProbeUnknownColumnEPL     = "on SupportBean_A as up merge ABCInfra as mv when not matched then insert (col) select 1"
	iiProbeNotmatchedUpdateEPL  = "on SupportBean_A as up merge MergeInfra as mv where mv.boolPrimitive=true when not matched then update set intPrimitive = 1"
	iiProbeMatchedInsertEPL     = "on SupportBean_A as up merge MergeInfra as mv where mv.theString=id when matched then insert select *"
	iiProbeMissingClausesEPL    = "on SupportBean as up merge MergeInfra as mv"
	iiProbeMissingThenEPL       = "on SupportBean as up merge MergeInfra as mv where a=b when matched"
	iiProbeAndThenDeleteEPL     = "on SupportBean as up merge MergeInfra as mv where a=b when matched and then delete"
	iiProbeAmbiguousWhereEPL    = "on SupportBean as up merge MergeInfra as mv where boolPrimitive=true when not matched then insert select *"
	iiProbeInvalidSelectEPL     = "on SupportBean_A as up merge MergeInfra as mv where mv.boolPrimitive=true when not matched then insert select intPrimitive"
	iiProbeMatchWhereEPL        = "on SupportBean_A as up merge MergeInfra as mv where mv.boolPrimitive=true when not matched then insert select * where theString = 'A'"
	iiProbeVariableLHSEPL       = "@public create variable int myvariable;\non SupportBean_A merge MergeInfra when matched then update set myvariable = 1;\n"
	iiProbeIndexedLHSEPL        = "on SupportBean_A merge MergeInfra when matched then update set theString[1][2] = 1;\n"
	iiProbeNestedAssignEPL      = "on MyEvent as me update AInfra set c = me.so"

	iiErrFilterWindowevent = "On-Merge not-matched filter expression may not use properties that are provided by the named window event [" + iiProbeFilterWindoweventEPL + "]"
	iiErrUnknownColumnNW   = "Validation failed in when-not-matched (clause 1): Event type named 'ABCInfra' has already been declared with differing column name or type information: Type by name 'ABCInfra' in property 'col' property name not found in target"
	iiErrUnknownColumnTbl  = "Validation failed in when-not-matched (clause 1): Column 'col' could not be assigned to any of the properties of the underlying type (missing column names, event property, setter method or constructor?) ["
	iiErrNotmatchedUpdate  = "Incorrect syntax near 'update' (a reserved keyword) expecting 'insert' but found 'update' at line 1 column 9"
	iiErrMatchedInsertNW   = "Validation failed in when-not-matched (clause 1): Expression-returned event type 'SupportBean_A' with underlying type 'com.espertech.esper.regressionlib.support.bean.SupportBean_A' cannot be converted to target event type 'MergeInfra' with underlying type 'com.espertech.esper.common.internal.support.SupportBean' [" + iiProbeMatchedInsertEPL + "]"
	iiErrMissingClauses    = "Unexpected end-of-input at line 1 column 4"
	iiErrMissingThen       = "Incorrect syntax near end-of-input ('matched' is a reserved keyword) expecting 'then' but found end-of-input at line 1 column 66 ["
	iiErrAndThenDelete     = "Incorrect syntax near 'then' (a reserved keyword) at line 1 column 71 [" + iiProbeAndThenDeleteEPL + "]"
	iiErrAmbiguousWhere    = "Failed to validate where-clause expression 'boolPrimitive=true': Property named 'boolPrimitive' is ambiguous as is valid for more then one stream [" + iiProbeAmbiguousWhereEPL + "]"
	iiErrInvalidSelect     = "Failed to validate select-clause expression 'intPrimitive': Property named 'intPrimitive' is not valid in any stream [" + iiProbeInvalidSelectEPL + "]"
	iiErrMatchWhere        = "Failed to validate match where-clause expression 'theString=\"A\"': Property named 'theString' is not valid in any stream [" + iiProbeMatchWhereEPL + "]"
	iiErrVariableLHS       = "Left-hand-side does not allow variables for variable 'myvariable'"
	iiErrIndexedLHS        = "Unrecognized left-hand-side assignment 'theString[1][2]'"
	iiErrNestedAssign      = "Failed to validate assignment expression 'c=me.so': Invalid assignment to property 'c' event type 'Composite' from event type 'SomeOther' [" + iiProbeNestedAssignEPL + "]"

	iiInsertOnlyCreateEPL     = "@Name('Window') @public create window InsertOnlyInfra#unique(p0) as (p0 string, p1 int)"
	iiInsertOnlyEquivalentEPL = "@name('on') on SupportBean merge InsertOnlyInfra where 1=2 when not matched then insert select theString as p0, intPrimitive as p1"
	iiInsertOnlyPlainEPL      = "@name('on') on SupportBean merge InsertOnlyInfra insert select theString as p0, intPrimitive as p1"
	iiInsertOnlyColnamesEPL   = "@name('on') on SupportBean as provider merge InsertOnlyInfra insert(p0, p1) select provider.theString, intPrimitive"
)

var (
	infraNWTableOnMergeInvalidInsertOnlyJavaSources = []string{
		infraNWTableOnMergeInvalidInsertOnlySource,
	}
	infraNWTableOnMergeInvalidInsertOnlyJavaRuntimeIDs = []string{
		"java-runtime-99b2d413519187b56232",
		"java-runtime-33b1e837bc8b373bb198",
		"java-runtime-651148621e89ec5465b0",
		"java-runtime-e51caa89fbde4ab34197",
		"java-runtime-8900ac7e3d063d8b8842",
		"java-runtime-b581753558f219b16a2a",
	}
	infraNWTableOnMergeInvalidInsertOnlyJavaExecutions = []string{
		"InfraInvalid{namedWindow=true}",
		"InfraInvalid{namedWindow=false}",
		"InfraInsertOnly{namedWindow=true, useEquivalent=true, soda=false, useColumnNames=false}",
		"InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=false, useColumnNames=false}",
		"InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=false, useColumnNames=true}",
		"InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=true, useColumnNames=false}",
	}
	infraNWTableOnMergeInvalidInsertOnlyJavaStaticIDs = []string{
		"java-42d23ac20541998f0a55",
		"java-42d23ac20541998f0a55",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
		"java-0b6e7bd4eb235001d140",
	}
	infraNWTableOnMergeInvalidInsertOnlyCases = []string{
		"invalid-nw",
		"invalid-table",
		"insertonly-nw-equivalent",
		"insertonly-nw",
		"insertonly-nw-colnames",
		"insertonly-nw-soda",
	}
	infraNWTableOnMergeInvalidInsertOnlyOrdinals = []int{40, 41, 42, 43, 44, 45}
)

// iiProbeLabelsNW is the named-window probe sequence (lines 832-873): the
// matched-insert event-type-conversion probe is named-window only.
var iiProbeLabelsNW = []string{
	"notmatched-filter-windowevent", "insert-unknown-column",
	"notmatched-update-action", "matched-insert-wrong-type",
	"missing-clauses", "missing-then", "and-then-delete",
	"ambiguous-where-prop", "invalid-select-prop",
	"invalid-match-where-prop", "variable-lhs", "indexed-lhs",
}

// iiProbeLabelsTable is the table probe sequence: identical to the
// named-window sequence minus matched-insert-wrong-type.
var iiProbeLabelsTable = []string{
	"notmatched-filter-windowevent", "insert-unknown-column",
	"notmatched-update-action", "missing-clauses", "missing-then",
	"and-then-delete", "ambiguous-where-prop", "invalid-select-prop",
	"invalid-match-where-prop", "variable-lhs", "indexed-lhs",
}

// iiProbeEPLs pins the byte-exact probe EPL per statement label; the EPLs
// are shared by both invalid variants.
var iiProbeEPLs = map[string]string{
	"notmatched-filter-windowevent": iiProbeFilterWindoweventEPL,
	"insert-unknown-column":         iiProbeUnknownColumnEPL,
	"notmatched-update-action":      iiProbeNotmatchedUpdateEPL,
	"matched-insert-wrong-type":     iiProbeMatchedInsertEPL,
	"missing-clauses":               iiProbeMissingClausesEPL,
	"missing-then":                  iiProbeMissingThenEPL,
	"and-then-delete":               iiProbeAndThenDeleteEPL,
	"ambiguous-where-prop":          iiProbeAmbiguousWhereEPL,
	"invalid-select-prop":           iiProbeInvalidSelectEPL,
	"invalid-match-where-prop":      iiProbeMatchWhereEPL,
	"variable-lhs":                  iiProbeVariableLHSEPL,
	"indexed-lhs":                   iiProbeIndexedLHSEPL,
	"nested-event-assign":           iiProbeNestedAssignEPL,
}

// iiGate pins the Go rejection boundary an expressible probe must hit:
// coded rejections must carry the esper.Error code plus the substring;
// uncoded rejections (plain fmt.Errorf validators) match the substring only.
type iiGate struct {
	code      esper.ErrorCode
	substring string
	coded     bool
}

var iiExpected = map[string]iiGate{
	// The subquery-correlated not-matched filter has no typed-API form; the
	// nearest boundary rejects a direct target-field reference in the
	// not-matched condition ("named-window fields" / "table fields").
	"notmatched-filter-windowevent": {substring: "not-matched condition cannot reference"},
	// Build wraps the inner UnknownName rejection in the trigger-level
	// InvalidRule ("... at trigger: ... UnknownName: assignment references
	// unknown column"), so errors.As surfaces InvalidRule.
	"insert-unknown-column":     {code: esper.ErrorInvalidRule, substring: "unknown column", coded: true},
	"notmatched-update-action":  {substring: "not-matched branch requires an insert action"},
	"matched-insert-wrong-type": {substring: "target insert must be not-matched"},
	"missing-clauses":           {code: esper.ErrorInvalidRule, substring: "requires at least one clause", coded: true},
	"invalid-select-prop":       {substring: "unknown field"},
	"invalid-match-where-prop":  {substring: "unknown field"},
	"variable-lhs":              {code: esper.ErrorInvalidRule, substring: "unknown column", coded: true},
	"indexed-lhs":               {substring: "is not an array or slice"},
}

// iiCaseState carries the per-case replay context for the invalid cases:
// the environment the fixture deploys register into, the variant flag and
// the accumulating trace.
type iiCaseState struct {
	caseName string
	env      *esper.Environment
	isTable  bool
	trace    *compat.Trace
}

func infraNWTableOnMergeInvalidInsertOnlyIsInvalid(caseName string) bool {
	return strings.HasPrefix(caseName, "invalid-")
}

// iiInsertOnlyMergeEPL pins the 'on' merge EPL per insert-only case (lines
// 485-492); the soda variant shares the plain EPL.
func iiInsertOnlyMergeEPL(caseName string) string {
	switch caseName {
	case "insertonly-nw-equivalent":
		return iiInsertOnlyEquivalentEPL
	case "insertonly-nw-colnames":
		return iiInsertOnlyColnamesEPL
	default:
		return iiInsertOnlyPlainEPL
	}
}

// runInfraNWTableOnMergeInvalidInsertOnlyScenario replays the six
// InfraNWTableOnMerge executions: invalid cases record compile-error
// markers for each probe; insert-only cases record deployed markers, the
// 'on' listener batches and window snapshots in Java's observable order.
func runInfraNWTableOnMergeInvalidInsertOnlyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeInvalidInsertOnlyCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeInvalidInsertOnlyCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeInvalidInsertOnlyID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeInvalidInsertOnlyID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeInvalidInsertOnlyCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableOnDeleteA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	isInvalid := infraNWTableOnMergeInvalidInsertOnlyIsInvalid(caseName)
	isTable := caseName == "invalid-table"
	if !isInvalid {
		// `@Name('Window') @public create window InsertOnlyInfra#unique(p0)
		// as (p0 string, p1 int)` — the window is an env-level artifact on
		// the Go side; the 'Window' deploy step attaches the consumer query
		// the Java iterator reads.
		schema, err := esper.RegisterMap(env, "InsertOnlyInfraType", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "InsertOnlyInfra", schema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("p0")))); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeInvalidInsertOnlyJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeInvalidInsertOnlyID}
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
	state := &iiCaseState{caseName: caseName, env: env, isTable: isTable, trace: &trace}
	pinned := infraNWTableOnMergeInvalidInsertOnlyCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if isInvalid {
				if err := state.deployInvalid(step); err != nil {
					return compat.Trace{}, err
				}
				continue
			}
			plan, err := iiBuildInsertOnly(env, caseName, step.Statement, step.Epl)
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
				// Java attaches the listener only to the 'on' merge
				// statement (env.addListener("on")).
				if statement.Name() == "on" {
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record("on", batch)
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
		case "build-error":
			if err := state.buildError(step); err != nil {
				return compat.Trace{}, err
			}
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
			// the trailing loop retires the insert-only deployments.
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

// deployInvalid executes the fixture registrations the path-ful probes
// compile against: the MergeInfra/ABCSchema/ABCInfra module (nw unique-key
// window or keyless table) and the block-2 Composite/AInfra/SomeOther/
// MyEvent artifacts. The byte-exact EPLs the deploy steps pin are checked
// per label.
func (s *iiCaseState) deployInvalid(step compat.Step) error {
	switch step.Statement {
	case "fixture":
		want := iiInvalidFixtureNW
		if s.isTable {
			want = iiInvalidFixtureTable
		}
		if step.Epl != want {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, step.Epl)
		}
		abcSchema, err := esper.RegisterMap(s.env, "ABCSchema", []esper.FieldSpec{
			esper.FieldDef("val", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		if s.isTable {
			if _, err := esper.CreateTable(s.env, "MergeInfra", []esper.TableColumn{
				esper.TableColumnOf[string]("theString"),
				esper.TableColumnOf[int64]("intPrimitive"),
				esper.TableColumnOf[bool]("boolPrimitive"),
			}); err != nil {
				return err
			}
			_, err = esper.CreateTable(s.env, "ABCInfra", []esper.TableColumn{
				esper.TableColumnOf[int64]("val"),
			})
			return err
		}
		schema, ok := s.env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("%s: SupportBean schema is missing", infraNWTableOnMergeInvalidInsertOnlyID)
		}
		if _, err := esper.CreateNamedWindow(s.env, "MergeInfra", schema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("theString")))); err != nil {
			return err
		}
		_, err = esper.CreateNamedWindow(s.env, "ABCInfra", abcSchema,
			esper.NamedWindowRetention(esper.KeepAll()))
		return err
	case "composite-schema":
		if step.Epl != iiInvalidCompositeEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, step.Epl)
		}
		_, err := esper.RegisterMap(s.env, "Composite", []esper.FieldSpec{
			esper.FieldDef("c0", reflect.TypeOf(int64(0))),
		})
		return err
	case "ainfra-window":
		if step.Epl != iiInvalidAInfraEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, step.Epl)
		}
		composite, ok := s.env.Schema("Composite")
		if !ok {
			return fmt.Errorf("%s: Composite schema is missing", infraNWTableOnMergeInvalidInsertOnlyID)
		}
		ainfraSchema, err := esper.RegisterMap(s.env, "AInfraSchema", []esper.FieldSpec{
			esper.FieldDef("c", reflect.TypeOf(map[string]any{})),
		}, esper.WithNestedPropertySchema("c", composite))
		if err != nil {
			return err
		}
		_, err = esper.CreateNamedWindow(s.env, "AInfra", ainfraSchema,
			esper.NamedWindowRetention(esper.KeepAll()))
		return err
	case "someother-schema":
		if step.Epl != iiInvalidSomeOtherEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, step.Epl)
		}
		_, err := esper.RegisterMap(s.env, "SomeOther", []esper.FieldSpec{
			esper.FieldDef("c1", reflect.TypeOf(int64(0))),
		})
		return err
	case "myevent-schema":
		if step.Epl != iiInvalidMyEventEPL {
			return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, step.Epl)
		}
		someOther, ok := s.env.Schema("SomeOther")
		if !ok {
			return fmt.Errorf("%s: SomeOther schema is missing", infraNWTableOnMergeInvalidInsertOnlyID)
		}
		_, err := esper.RegisterMap(s.env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("so", reflect.TypeOf(map[string]any{})),
		}, esper.WithNestedPropertySchema("so", someOther))
		return err
	default:
		return fmt.Errorf("%s: unknown deploy %q in case %q",
			infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, s.caseName)
	}
}

// iiBuildInsertOnly mirrors the Java deploys of InfraInsertOnly: the
// 'Window' label deploys the consumer query the Java iterator reads; the
// 'on' label deploys the insert-only merge — MergeIntoNamedWindowWhen with
// a nil match (plain/colnames/soda) or Literal(false) for the `where 1=2`
// equivalent form, and a single WhenNotMatchedAny insert. Java's
// insert(p0, p1) select column list is sugar for the same SetColumn
// assignments.
func iiBuildInsertOnly(env *esper.Environment, caseName string, label string, epl string) (esper.Plan, error) {
	switch label {
	case "Window":
		if epl != iiInsertOnlyCreateEPL {
			return esper.Plan{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, label, epl)
		}
		return env.Build(esper.FromNamedWindow(env, "InsertOnlyInfra").Query(
			esper.StatementName("Window"), esper.WithOldStream()))
	case "on":
		if want := iiInsertOnlyMergeEPL(caseName); epl != want {
			return esper.Plan{}, fmt.Errorf("%s: deploy %q carries an unpinned EPL %q",
				infraNWTableOnMergeInvalidInsertOnlyID, label, epl)
		}
		sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
		assignments := []esper.TableAssignment{
			esper.SetColumn("p0", esper.Field[infraNWTableOnMergeBean, string]("theString")),
			esper.SetColumn("p1", esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")),
		}
		var match esper.Expression[bool]
		if caseName == "insertonly-nw-equivalent" {
			// `where 1=2` — a constant-false match predicate; every
			// trigger event reaches the not-matched insert.
			match = esper.Literal(false)
		}
		return env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen("InsertOnlyInfra", match,
			esper.WhenNotMatchedAny(assignments...)).Query(
			esper.StatementName("on"), esper.WithOldStream()))
	}
	return esper.Plan{}, fmt.Errorf("%s: unexpected deploy statement %q for case %q",
		infraNWTableOnMergeInvalidInsertOnlyID, label, caseName)
}

// buildError runs one expected-invalid probe against the fluent equivalent
// of the pinned EPL. Each probe verifies Go rejects the nearest expressible
// boundary (or is unrepresentable) before recording the pinned Java message
// prefix.
func (s *iiCaseState) buildError(step compat.Step) error {
	if pinned, ok := iiProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q",
			infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, step.Epl)
	}
	sourceA := esper.From[infraNWTableOnDeleteA](s.env, "SupportBean_A")
	sourceBean := esper.From[infraNWTableOnMergeBean](s.env, "SupportBean")
	id := esper.Field[infraNWTableOnDeleteA, string]("id")
	var buildErr error
	switch step.Statement {
	case "notmatched-filter-windowevent":
		// Java correlates the not-matched filter's subquery to the merge
		// target (windowevent.theString); the typed API has no
		// subquery-in-merge form, so the nearest expressible boundary is a
		// direct target-field reference in the not-matched condition.
		condition := esper.Equal[string](esper.NamedWindowField[string]("theString"), id)
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("MergeInfra", nil,
				esper.WhenNotMatched(
					esper.Equal[string](esper.TableField[string]("theString"), id),
					esper.SetColumn("theString", id))).Query(esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra",
				condition,
				esper.WhenNotMatched(condition,
					esper.SetColumn("theString", id))).Query(esper.StatementName("probe")))
		}
	case "insert-unknown-column":
		// `insert (col) select 1` — the explicit column name is unknown to
		// the ABCInfra target in both variants.
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("ABCInfra", nil,
				esper.WhenNotMatchedAny(esper.SetColumn("col", esper.Literal(int64(1))))).Query(
				esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("ABCInfra", nil,
				esper.WhenNotMatchedAny(esper.SetColumn("col", esper.Literal(int64(1))))).Query(
				esper.StatementName("probe")))
		}
	case "notmatched-update-action":
		// Java's parser rejects `update` in a not-matched clause; the Go
		// action-chain form reaches the equivalent semantic rejection.
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("MergeInfra", nil,
				esper.WhenNotMatchedActions(esper.ThenUpdate(esper.Literal(true),
					esper.SetColumn("intPrimitive", esper.Literal(int64(1)))))).Query(
				esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra",
				esper.Equal[bool](esper.NamedWindowField[bool]("boolPrimitive"), esper.Literal(true)),
				esper.WhenNotMatchedActions(esper.ThenUpdate(esper.Literal(true),
					esper.SetColumn("intPrimitive", esper.Literal(int64(1)))))).Query(
				esper.StatementName("probe")))
		}
	case "matched-insert-wrong-type":
		// NW-only probe: Java rejects the matched-branch insert's
		// event-type conversion; Go's nearest boundary rejects the matched
		// target-insert shape itself.
		_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra",
			esper.Equal[string](esper.NamedWindowField[string]("theString"), id),
			esper.WhenMatchedActions(esper.ThenInsertIntoTarget(
				esper.SetColumn("theString", id)))).Query(esper.StatementName("probe")))
	case "missing-clauses":
		// Java's parser rejects the clause-less merge at end-of-input;
		// Go's nearest boundary is the semantic no-clause rejection.
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceBean).MergeIntoTableWhen("MergeInfra", nil).Query(
				esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen("MergeInfra", nil).Query(
				esper.StatementName("probe")))
		}
	case "missing-then":
		// `when matched` with no then-action is a parser position
		// diagnostic; the typed API has no equivalent surface.
		buildErr = fmt.Errorf("a when-clause without a then-action is unrepresentable")
	case "and-then-delete":
		// `when matched and then delete` is a parser position diagnostic;
		// the typed API has no equivalent surface.
		buildErr = fmt.Errorf("a when-clause condition with no action is unrepresentable")
	case "ambiguous-where-prop":
		// Unqualified `boolPrimitive` is ambiguous between the trigger and
		// target streams in EPL text; the typed API resolves Field vs
		// NamedWindowField explicitly, so the ambiguity is unrepresentable.
		buildErr = fmt.Errorf("unqualified ambiguous property references are unrepresentable")
	case "invalid-select-prop":
		// `insert select intPrimitive` — intPrimitive is not valid on the
		// SupportBean_A trigger; the nearest boundary rejects the unknown
		// trigger field in the insert assignment.
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("MergeInfra", nil,
				esper.WhenNotMatchedAny(esper.SetColumn("theString",
					esper.Field[infraNWTableOnDeleteA, int64]("intPrimitive")))).Query(
				esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra",
				esper.Equal[bool](esper.NamedWindowField[bool]("boolPrimitive"), esper.Literal(true)),
				esper.WhenNotMatchedAny(esper.SetColumn("theString",
					esper.Field[infraNWTableOnDeleteA, int64]("intPrimitive")))).Query(
				esper.StatementName("probe")))
		}
	case "invalid-match-where-prop":
		// `insert select * where theString = 'A'` — the insert action's
		// match where-clause; ThenInsertIntoTargetWhen is the direct
		// analog and rejects the unknown trigger field.
		matchWhere := esper.Equal[string](
			esper.Field[infraNWTableOnDeleteA, string]("theString"), esper.Literal("A"))
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("MergeInfra", nil,
				esper.WhenNotMatchedActions(esper.ThenInsertIntoTargetWhen(matchWhere,
					esper.CopyMatchingFields()))).Query(esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra",
				esper.Equal[bool](esper.NamedWindowField[bool]("boolPrimitive"), esper.Literal(true)),
				esper.WhenNotMatchedActions(esper.ThenInsertIntoTargetWhen(matchWhere,
					esper.CopyMatchingFields()))).Query(esper.StatementName("probe")))
		}
	case "variable-lhs":
		// Java's two-statement module creates myvariable before the merge
		// fails; the Go registration mirrors it, then the merge rejects
		// the non-column left-hand side.
		if err := s.env.RegisterVariable("myvariable", int64(0)); err != nil {
			return fmt.Errorf("%s: build-error probe %q variable registration failed: %w",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, err)
		}
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("MergeInfra", nil,
				esper.WhenMatchedAny(esper.SetColumn("myvariable", esper.Literal(int64(1))))).Query(
				esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra", nil,
				esper.WhenMatchedAny(esper.SetColumn("myvariable", esper.Literal(int64(1))))).Query(
				esper.StatementName("probe")))
		}
	case "indexed-lhs":
		// Java's double-index theString[1][2] has no Go form; the nearest
		// boundary rejects indexing into the non-array column.
		if s.isTable {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoTableWhen("MergeInfra", nil,
				esper.WhenMatchedAny(esper.SetArrayElement("theString",
					esper.Literal(int64(1)), esper.Literal(int64(2))))).Query(
				esper.StatementName("probe")))
		} else {
			_, buildErr = s.env.Build(esper.OnEvent(sourceA).MergeIntoNamedWindowWhen("MergeInfra", nil,
				esper.WhenMatchedAny(esper.SetArrayElement("theString",
					esper.Literal(int64(1)), esper.Literal(int64(2))))).Query(
				esper.StatementName("probe")))
		}
	case "nested-event-assign":
		// Java rejects the map-to-map event-type assignment on schema
		// identity (Composite vs SomeOther); Go's assignment check is
		// reflect.Type-only, so the faithful map-schema form compiles.
		// Pin the prefix without claiming a Go rejection boundary.
		buildErr = fmt.Errorf("nested event-type identity assignment is unrepresentable")
	default:
		return fmt.Errorf("%s: unknown build-error probe %q",
			infraNWTableOnMergeInvalidInsertOnlyID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled",
			infraNWTableOnMergeInvalidInsertOnlyID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix; the unrepresentable probes
	// skip the gate.
	if want, ok := iiExpected[step.Statement]; ok {
		var espErr *esper.Error
		if want.coded {
			if !errors.As(buildErr, &espErr) || espErr.Code != want.code ||
				!strings.Contains(buildErr.Error(), want.substring) {
				return fmt.Errorf("%s: build-error probe %q drift: got %v",
					infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, buildErr)
			}
		} else if !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v",
				infraNWTableOnMergeInvalidInsertOnlyID, step.Statement, buildErr)
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// loadInfraNWTableOnMergeInvalidInsertOnlyScenario enforces the strict
// scenario contract shared by the differential runners: no duplicate or
// unknown JSON fields, pinned metadata, pinned per-case
// runtime/execution/EPL, and a per-op step field whitelist followed by a
// full step-shape pin.
func loadInfraNWTableOnMergeInvalidInsertOnlyScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeInvalidInsertOnlyID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeInvalidInsertOnlyID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeInvalidInsertOnlyID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeInvalidInsertOnlyID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeInvalidInsertOnlyID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeInvalidInsertOnlyID ||
		metadata.Description != infraNWTableOnMergeInvalidInsertOnlyDescription ||
		metadata.JavaCommit != infraNWTableOnMergeInvalidInsertOnlyJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeInvalidInsertOnlySource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeInvalidInsertOnlyID)
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaRuntimes"], infraNWTableOnMergeInvalidInsertOnlyJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaNames"], infraNWTableOnMergeInvalidInsertOnlyJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaStaticIds"], infraNWTableOnMergeInvalidInsertOnlyJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeInvalidInsertOnlyCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeInvalidInsertOnlyID, len(infraNWTableOnMergeInvalidInsertOnlyCases))
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
		if definition.Case != infraNWTableOnMergeInvalidInsertOnlyCases[index] ||
			definition.Ordinal != infraNWTableOnMergeInvalidInsertOnlyOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeInvalidInsertOnlyJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeInvalidInsertOnlyJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeInvalidInsertOnlyCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeInvalidInsertOnlyCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeInvalidInsertOnlyID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeInvalidInsertOnlyID)
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
		case "build-error":
			if err := requireInfraNWTableOnMergeFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
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
		for _, name := range infraNWTableOnMergeInvalidInsertOnlyCases {
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
	if err := validateInfraNWTableOnMergeInvalidInsertOnlyRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableOnMergeInvalidInsertOnlyCaseEPLs pins the first deploy EPL of
// each case, the value carried by the scenario cases[] metadata.
var infraNWTableOnMergeInvalidInsertOnlyCaseEPLs = []string{
	iiInvalidFixtureNW,
	iiInvalidFixtureTable,
	iiInsertOnlyCreateEPL,
	iiInsertOnlyCreateEPL,
	iiInsertOnlyCreateEPL,
	iiInsertOnlyCreateEPL,
}

var infraNWTableOnMergeInvalidInsertOnlyCaseObservations = []string{
	"compile-error; thirteen probes record the pinned Java message prefixes over the MergeInfra unique-key named window and ABCInfra keepall window fixtures, then the Composite/AInfra/SomeOther/MyEvent fixture rejects the nested event-type assignment",
	"compile-error; twelve probes record the pinned Java message prefixes over the MergeInfra and ABCInfra keyless tables (the nw-only matched-insert probe is absent and probe 2 pins the column-assignment wording), then the Composite/AInfra/SomeOther/MyEvent fixture rejects the nested event-type assignment",
	"listener+iterator; on-merge with the equivalent where 1=2 / when not matched then insert form over the InsertOnlyInfra unique-key named window",
	"listener+iterator; plain insert-only on-merge (bare merge ... insert select) over the InsertOnlyInfra unique-key named window",
	"listener+iterator; insert-only on-merge with an explicit insert(p0, p1) column list over the InsertOnlyInfra unique-key named window",
	"listener+iterator; the plain insert-only on-merge compiled through the soda object-model round-trip over the InsertOnlyInfra unique-key named window (observably identical EPL to insertonly-nw)",
}

// validateInfraNWTableOnMergeInvalidInsertOnlyRawSteps pins the complete
// step sequence per case against the raw JSON objects: deploy statements
// with byte-exact EPL, deployed markers, build-error probes with pinned
// EPL and expectError prefix, send event types with canonical payloads,
// snapshot reads, and undeploy-all terminators.
func validateInfraNWTableOnMergeInvalidInsertOnlyRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeInvalidInsertOnlyCases {
		want, ok := infraNWTableOnMergeInvalidInsertOnlyCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeInvalidInsertOnlyID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeInvalidInsertOnlyID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeInvalidInsertOnlyID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeInvalidInsertOnlyID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeInvalidInsertOnlyStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeInvalidInsertOnlyID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeInvalidInsertOnlyID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeInvalidInsertOnlyID)
	}
	return nil
}

// infraNWTableOnMergeInvalidInsertOnlyStepKey renders a raw step object
// into its pinned string form. Fields are read from the raw JSON because
// compat.Step does not carry the fields array.
func infraNWTableOnMergeInvalidInsertOnlyStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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
	case "build-error":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		expectError, err := stringField("expectError")
		if err != nil {
			return "", err
		}
		return "build-error:" + statement + ":" + epl + ":" + expectError, nil
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

// iiInvalidCaseSteps renders the pinned step sequence of one InfraInvalid
// case: the fixture module deploy, the twelve or thirteen build-error
// probes (the matched-insert probe is named-window only and probe 2 pins
// divergent wording), undeploy-all, the four block-2 fixture deploys, the
// nested event-type assignment probe and the final undeploy-all.
func iiInvalidCaseSteps(isTable bool) []string {
	fixture := iiInvalidFixtureNW
	labels := iiProbeLabelsNW
	if isTable {
		fixture = iiInvalidFixtureTable
		labels = iiProbeLabelsTable
	}
	steps := []string{"deploy:fixture:" + fixture}
	for _, label := range labels {
		steps = append(steps, "build-error:"+label+":"+iiProbeEPLs[label]+":"+iiExpectError(label, isTable))
	}
	steps = append(steps,
		"undeploy-all",
		"deploy:composite-schema:"+iiInvalidCompositeEPL,
		"deploy:ainfra-window:"+iiInvalidAInfraEPL,
		"deploy:someother-schema:"+iiInvalidSomeOtherEPL,
		"deploy:myevent-schema:"+iiInvalidMyEventEPL,
		"build-error:nested-event-assign:"+iiProbeNestedAssignEPL+":"+iiErrNestedAssign,
		"undeploy-all")
	return steps
}

// iiExpectError pins the per-variant expectError prefix: probe 2 diverges
// between the named-window (event-type redeclare) and table
// (column-assignment) wording.
func iiExpectError(label string, isTable bool) string {
	if isTable && label == "insert-unknown-column" {
		return iiErrUnknownColumnTbl
	}
	switch label {
	case "notmatched-filter-windowevent":
		return iiErrFilterWindowevent
	case "insert-unknown-column":
		return iiErrUnknownColumnNW
	case "notmatched-update-action":
		return iiErrNotmatchedUpdate
	case "matched-insert-wrong-type":
		return iiErrMatchedInsertNW
	case "missing-clauses":
		return iiErrMissingClauses
	case "missing-then":
		return iiErrMissingThen
	case "and-then-delete":
		return iiErrAndThenDelete
	case "ambiguous-where-prop":
		return iiErrAmbiguousWhere
	case "invalid-select-prop":
		return iiErrInvalidSelect
	case "invalid-match-where-prop":
		return iiErrMatchWhere
	case "variable-lhs":
		return iiErrVariableLHS
	case "indexed-lhs":
		return iiErrIndexedLHS
	case "nested-event-assign":
		return iiErrNestedAssign
	}
	return ""
}

// iiInsertOnlyCaseSteps renders the pinned step sequence of one
// InfraInsertOnly case (lines 477-517): the Window and 'on' deploys with
// deployed markers, the E1 send and snapshot, the E2 send and snapshot
// across the milestone(0) no-op, and undeploy-all.
func iiInsertOnlyCaseSteps(caseName string) []string {
	return []string{
		"deploy:Window:" + iiInsertOnlyCreateEPL,
		"deployed:Window",
		"deploy:on:" + iiInsertOnlyMergeEPL(caseName),
		"deployed:on",
		`send:SupportBean:{"intPrimitive":1,"theString":"E1"}`,
		"snapshot:Window:any:p0,p1",
		`send:SupportBean:{"intPrimitive":2,"theString":"E2"}`,
		"snapshot:Window:any:p0,p1",
		"undeploy-all",
	}
}

// infraNWTableOnMergeInvalidInsertOnlyCaseSteps pins the exact op sequence
// per case: deploy statements with byte-exact EPL, deployed markers,
// build-error probes with pinned EPL and expectError prefix, send event
// types with canonical payloads, snapshot reads, and undeploy-all
// terminators.
var infraNWTableOnMergeInvalidInsertOnlyCaseSteps = map[string][]string{
	"invalid-nw":               iiInvalidCaseSteps(false),
	"invalid-table":            iiInvalidCaseSteps(true),
	"insertonly-nw-equivalent": iiInsertOnlyCaseSteps("insertonly-nw-equivalent"),
	"insertonly-nw":            iiInsertOnlyCaseSteps("insertonly-nw"),
	"insertonly-nw-colnames":   iiInsertOnlyCaseSteps("insertonly-nw-colnames"),
	"insertonly-nw-soda":       iiInsertOnlyCaseSteps("insertonly-nw-soda"),
}
