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
	infraNWTableOnMergeID          = "infra-nwtable-on-merge"
	infraNWTableOnMergeDescription = "InfraNWTableOnMerge on-trigger merge semantics over a keepall named window and over a primary-key table: insert-only merge delivering each trigger row as new data, and ordered matched-delete/not-matched-insert/matched-update branches with insert-remove IR-pair delivery to the merge listener and the named-window create consumer (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"

	infraNWTableOnMergeCreateSimpleNW  = "@name('create') @public create window MyInfra#keepall() as (p0 string, p1 int)"
	infraNWTableOnMergeCreateSimpleTbl = "@name('create') @public create table MyInfra(p0 string primary key, p1 int)"
	infraNWTableOnMergeMergeSimple     = "@name('merge') on SupportBean sb merge MyInfra insert select theString as p0, intPrimitive as p1"

	infraNWTableOnMergeCreateMatchNW  = "@name('create') @public create window MyInfra.win:keepall() as SupportBean"
	infraNWTableOnMergeCreateMatchTbl = "@name('create') @public create table MyInfra(theString string primary key, intPrimitive int)"
	// The named-window merge text is verbatim: Java concatenates
	// "insert select *" + "when matched" without a separating space.
	infraNWTableOnMergeMergeMatchNW = "@name('merge') on SupportBean sb merge MyInfra mw where sb.theString = mw.theString when matched and sb.intPrimitive < 0 then delete when not matched and intPrimitive > 0 then insert select *when matched and sb.intPrimitive > 0 then update set intPrimitive = sb.intPrimitive + mw.intPrimitive"
	// The table merge text is verbatim: Java concatenates the explicit
	// column list with a trailing space before "when matched".
	infraNWTableOnMergeMergeMatchTbl = "@name('merge') on SupportBean sb merge MyInfra mw where sb.theString = mw.theString when matched and sb.intPrimitive < 0 then delete when not matched and intPrimitive > 0 then insert select theString, intPrimitive when matched and sb.intPrimitive > 0 then update set intPrimitive = sb.intPrimitive + mw.intPrimitive"
)

var (
	infraNWTableOnMergeJavaSources = []string{
		infraNWTableOnMergeSource,
	}
	infraNWTableOnMergeJavaRuntimeIDs = []string{
		"java-runtime-dbf13fb6d1ca3a37275a",
		"java-runtime-df3d7a21bdd769f1acce",
		"java-runtime-240cb61eabb22eb2a215",
		"java-runtime-aaa45c95e95a919bd0c0",
	}
	infraNWTableOnMergeJavaExecutions = []string{
		"InfraOnMergeSimpleInsert{namedWindow=true}",
		"InfraOnMergeSimpleInsert{namedWindow=false}",
		"InfraOnMergeMatchNoMatch{namedWindow=true}",
		"InfraOnMergeMatchNoMatch{namedWindow=false}",
	}
	infraNWTableOnMergeJavaStaticIDs = []string{
		"java-0fdb4e6490ae6d9e9103",
		"java-0fdb4e6490ae6d9e9103",
		"java-5f5d122bcbf66d59da6c",
		"java-5f5d122bcbf66d59da6c",
	}
	infraNWTableOnMergeCases = []string{
		"simple-nw",
		"simple-table",
		"matchnomatch-nw",
		"matchnomatch-table",
	}
	infraNWTableOnMergeOrdinals = []int{0, 1, 2, 3}
)

// infraNWTableOnMergeBean mirrors the full SupportBean schema: the
// matchnomatch named window keeps every property, so listener and iterator
// rows carry all twenty fields with Java's default values.
type infraNWTableOnMergeBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int64   `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

func infraNWTableOnMergeIsTable(caseName string) bool {
	return caseName == "simple-table" || caseName == "matchnomatch-table"
}

// runInfraNWTableOnMergeScenario replays the four InfraNWTableOnMerge
// executions: each case deploys the pinned statements, sends the pinned
// events, and records listener batches, iterator snapshots and undeploy
// markers in Java's observable order.
func runInfraNWTableOnMergeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableOnMergeBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableOnMergeIsTable(caseName)
	const infraName = "MyInfra"
	if err := infraNWTableOnMergeCreateInfra(env, caseName, infraName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeID}
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
	pinned := infraNWTableOnMergeCaseSteps[caseName]
	for stepIndex, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plan, err := infraNWTableOnMergeBuild(env, caseName, step.Statement, infraName, isTable)
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
				if infraNWTableOnMergeListened(caseName, step.Statement) && statement.Name() == step.Statement {
					name := step.Statement
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

// infraNWTableOnMergeCreateInfra pre-creates the named window or table that
// the Java @public create statement establishes, so the 'create' deploy step
// can attach the consumer query the Java listener observes.
func infraNWTableOnMergeCreateInfra(env *esper.Environment, caseName string, infraName string, isTable bool) error {
	switch caseName {
	case "simple-nw":
		projection, err := esper.NewMapSchema("MyInfraMergeSchema", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			return err
		}
		if err := env.RegisterSchema(projection); err != nil {
			return err
		}
		return createKeepAllNamedWindow(env, infraName, projection)
	case "matchnomatch-nw":
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("SupportBean schema is missing")
		}
		return createKeepAllNamedWindow(env, infraName, schema)
	case "simple-table":
		_, err := esper.CreateTable(env, infraName, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("p0"),
			esper.TableColumnOf[int64]("p1"),
		})
		return err
	case "matchnomatch-table":
		_, err := esper.CreateTable(env, infraName, []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("theString"),
			esper.TableColumnOf[int64]("intPrimitive"),
		})
		return err
	}
	return fmt.Errorf("unexpected case %q", caseName)
}

// infraNWTableOnMergeListened reports whether the Java execution attaches a
// listener to the named statement: every case listens on 'merge'; the
// MatchNoMatch cases also listen on 'create'. Table 'create' listeners never
// fire in Java (tables do not stream to listeners), so Go attaches them too
// for fidelity where the API allows.
func infraNWTableOnMergeListened(caseName string, statement string) bool {
	switch caseName {
	case "simple-nw", "simple-table":
		return statement == "merge"
	case "matchnomatch-nw", "matchnomatch-table":
		return statement == "create" || statement == "merge"
	}
	return false
}

func infraNWTableOnMergeBuild(env *esper.Environment, caseName string, statement string, infraName string, isTable bool) (esper.Plan, error) {
	sourceBean := esper.From[infraNWTableOnMergeBean](env, "SupportBean")
	switch statement {
	case "create":
		if isTable {
			return env.Build(esper.FromTable(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
		}
		return env.Build(esper.FromNamedWindow(env, infraName).Query(esper.StatementName("create"), esper.WithOldStream()))
	case "merge":
		switch caseName {
		case "simple-nw", "simple-table":
			// Java: on SupportBean sb merge MyInfra insert select
			// theString as p0, intPrimitive as p1 — an insert-only merge
			// with no where clause; every trigger takes the not-matched
			// branch. The table form supplies the primary-key expression
			// the Go builder requires.
			assignments := []esper.TableAssignment{
				esper.SetColumn("p0", esper.Field[infraNWTableOnMergeBean, string]("theString")),
				esper.SetColumn("p1", esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")),
			}
			if isTable {
				return env.Build(esper.OnEvent(sourceBean).MergeIntoTableWhen(infraName,
					[]esper.Expr{esper.Field[infraNWTableOnMergeBean, string]("theString")},
					esper.WhenNotMatchedAny(assignments...)).Query(esper.StatementName("merge"), esper.WithOldStream()))
			}
			return env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen(infraName, nil,
				esper.WhenNotMatchedAny(assignments...)).Query(esper.StatementName("merge"), esper.WithOldStream()))
		case "matchnomatch-nw", "matchnomatch-table":
			// Java: merge MyInfra mw where sb.theString = mw.theString
			//   when matched and sb.intPrimitive < 0 then delete
			//   when not matched and intPrimitive > 0 then insert select *
			//     (table: insert select theString, intPrimitive)
			//   when matched and sb.intPrimitive > 0 then update set
			//     intPrimitive = sb.intPrimitive + mw.intPrimitive
			// Unqualified names in branch conditions read the trigger
			// event; mw.* reads the matched target row.
			triggerInt := esper.Field[infraNWTableOnMergeBean, int64]("intPrimitive")
			triggerString := esper.Field[infraNWTableOnMergeBean, string]("theString")
			deleteWhen := esper.Less[int64](triggerInt, esper.Literal(int64(0)))
			insertWhen := esper.Greater[int64](triggerInt, esper.Literal(int64(0)))
			updateWhen := esper.Greater[int64](triggerInt, esper.Literal(int64(0)))
			if isTable {
				return env.Build(esper.OnEvent(sourceBean).MergeIntoTableWhen(infraName,
					[]esper.Expr{triggerString},
					esper.WhenMatchedDelete(deleteWhen),
					esper.WhenNotMatched(insertWhen,
						esper.SetColumn("theString", triggerString),
						esper.SetColumn("intPrimitive", triggerInt)),
					esper.WhenMatched(updateWhen,
						esper.SetColumn("intPrimitive", esper.Add[int64](triggerInt, esper.TableField[int64]("intPrimitive")))),
				).Query(esper.StatementName("merge"), esper.WithOldStream()))
			}
			return env.Build(esper.OnEvent(sourceBean).MergeIntoNamedWindowWhen(infraName,
				esper.Equal[string](esper.NamedWindowField[string]("theString"), triggerString),
				esper.WhenMatchedDelete(deleteWhen),
				esper.WhenNotMatched(insertWhen, esper.CopyMatchingFields()),
				esper.WhenMatched(updateWhen,
					esper.SetColumn("intPrimitive", esper.Add[int64](triggerInt, esper.NamedWindowField[int64]("intPrimitive")))),
			).Query(esper.StatementName("merge"), esper.WithOldStream()))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q", statement)
}

func decodeInfraNWTableOnMergePayload(step compat.Step) (any, error) {
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
	}
	return nil, fmt.Errorf("unexpected event type %q", step.EventType)
}

func requireInfraNWTableOnMergeFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnMergeScenario enforces the strict scenario contract
// shared by the differential runners: no duplicate or unknown JSON fields,
// pinned metadata, pinned per-case runtime/execution/EPL, and a per-op step
// field whitelist followed by a full step-shape pin.
func loadInfraNWTableOnMergeScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeID, err)
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeID ||
		metadata.Description != infraNWTableOnMergeDescription ||
		metadata.JavaCommit != infraNWTableOnMergeJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeID)
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaRuntimes"], infraNWTableOnMergeJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaNames"], infraNWTableOnMergeJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaStaticIds"], infraNWTableOnMergeJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeID, len(infraNWTableOnMergeCases))
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
		if definition.Case != infraNWTableOnMergeCases[index] ||
			definition.Ordinal != infraNWTableOnMergeOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeID)
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
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnMergeStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnMergeCaseEPLs pins the first deploy EPL of each case, the
// value carried by the scenario cases[] metadata.
var infraNWTableOnMergeCaseEPLs = []string{
	infraNWTableOnMergeCreateSimpleNW,
	infraNWTableOnMergeCreateSimpleTbl,
	infraNWTableOnMergeCreateMatchNW,
	infraNWTableOnMergeCreateMatchTbl,
}

var infraNWTableOnMergeCaseObservations = []string{
	"listener+iterator; insert-only merge delivering each trigger row as new data over the keepall named window",
	"listener+iterator; same insert-only merge over the primary-key table",
	"listener+iterator; ordered matched-delete/not-matched-insert/matched-update branches over the keepall named window with create-statement IR pairs",
	"listener+iterator; same ordered merge branches over the primary-key table with no create-statement deliveries",
}

// validateInfraNWTableOnMergeRawSteps pins the complete step sequence per
// case against the raw JSON objects: deploy statements with byte-exact EPL,
// deployed markers, send event types with canonical payloads, snapshot reads,
// and undeploy-all terminators.
func validateInfraNWTableOnMergeRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeCases {
		want, ok := infraNWTableOnMergeCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeID)
	}
	return nil
}

// infraNWTableOnMergeStepKey renders a raw step object into its pinned
// string form. Fields are read from the raw JSON because compat.Step does
// not carry the fields array.
func infraNWTableOnMergeStepKey(object map[string]json.RawMessage, operation string) (string, error) {
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

// infraNWTableOnMergeCaseSteps pins the exact op sequence per case:
// deploy statements with byte-exact EPL, deployed markers, send event types
// with canonical payloads, snapshot reads, and undeploy-all terminators.
var infraNWTableOnMergeCaseSteps = map[string][]string{
	"simple-nw": {
		"deploy:create:@name('create') @public create window MyInfra#keepall() as (p0 string, p1 int)",
		"deployed:create",
		"deploy:merge:@name('merge') on SupportBean sb merge MyInfra insert select theString as p0, intPrimitive as p1",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:create:ordered:p0,p1",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"snapshot:create:any:p0,p1",
		"undeploy-all",
	},
	"simple-table": {
		"deploy:create:@name('create') @public create table MyInfra(p0 string primary key, p1 int)",
		"deployed:create",
		"deploy:merge:@name('merge') on SupportBean sb merge MyInfra insert select theString as p0, intPrimitive as p1",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"snapshot:create:any:p0,p1",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"snapshot:create:any:p0,p1",
		"undeploy-all",
	},
	"matchnomatch-nw": {
		"deploy:create:@name('create') @public create window MyInfra.win:keepall() as SupportBean",
		"deployed:create",
		"deploy:merge:@name('merge') on SupportBean sb merge MyInfra mw where sb.theString = mw.theString when matched and sb.intPrimitive < 0 then delete when not matched and intPrimitive > 0 then insert select *when matched and sb.intPrimitive > 0 then update set intPrimitive = sb.intPrimitive + mw.intPrimitive",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E2\"}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E2\"}",
		"snapshot:create:ordered:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E3\"}",
		"snapshot:create:ordered:theString,intPrimitive",
		"undeploy-all",
	},
	"matchnomatch-table": {
		"deploy:create:@name('create') @public create table MyInfra(theString string primary key, intPrimitive int)",
		"deployed:create",
		"deploy:merge:@name('merge') on SupportBean sb merge MyInfra mw where sb.theString = mw.theString when matched and sb.intPrimitive < 0 then delete when not matched and intPrimitive > 0 then insert select theString, intPrimitive when matched and sb.intPrimitive > 0 then update set intPrimitive = sb.intPrimitive + mw.intPrimitive",
		"deployed:merge",
		"send:SupportBean:{\"intPrimitive\":0,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":10,\"theString\":\"E2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E2\"}",
		"snapshot:create:any:theString,intPrimitive",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E3\"}",
		"snapshot:create:any:theString,intPrimitive",
		"undeploy-all",
	},
}

// infraNWTableOnMergeSnapshotFields extracts the pinned projection list
// for the snapshot at steps[stepIndex] from the pinned step key
// ("snapshot:<statement>:<mode>:<f1,f2,...>"). The case marker occupies
// steps[0], so the pinned index is stepIndex-1.
func infraNWTableOnMergeSnapshotFields(pinned []string, stepIndex int) []string {
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

// projectInfraNWTableOnMergeRows reduces each row to the pinned assertion
// fields: the Java iterator assertions read only these properties even when
// the infra row carries the full SupportBean schema.
func projectInfraNWTableOnMergeRows(rows []compat.ResultRecord, fields []string) []compat.ResultRecord {
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
