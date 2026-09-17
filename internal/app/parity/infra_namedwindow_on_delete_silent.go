package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNWOnDeleteSilentID          = "infra-namedwindow-on-delete-silent"
	infraNWOnDeleteSilentDescription = "InfraNamedWindowOnDelete silent-delete semantics: @hint('silent_delete') strips the deleted rows from the named window's own-statement delivery while the on-delete output (deleted rows as new data) and tail-view consumers still observe the delta, over a length(2) window and a groupwin(theString)#length(2) window (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java)."
	infraNWOnDeleteSilentJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWOnDeleteSilentSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java"

	infraNWOnDeleteSilentModule = "@name('create') create window MyWindow#length(2) as SupportBean;\n" +
		"insert into MyWindow select * from SupportBean;\n" +
		"@name('delete') @hint('silent_delete') on SupportBean_S0 delete from MyWindow where p00 = theString;\n" +
		"@name('count') select count(*) as cnt from MyWindow;"
	infraNWOnDeleteSilentModuleMany = "@name('create') create window MyWindow#groupwin(theString)#length(2) as SupportBean;\n" +
		"insert into MyWindow select * from SupportBean;\n" +
		"@name('delete') @hint('silent_delete') on SupportBean_S0 delete from MyWindow;\n" +
		"@name('count') select count(*) as cnt from MyWindow;"
)

var (
	infraNWOnDeleteSilentJavaSources = []string{
		infraNWOnDeleteSilentSource,
	}
	infraNWOnDeleteSilentJavaRuntimeIDs = []string{
		"java-runtime-38dd6f716920e46075c7",
		"java-runtime-6bce45980de6b3f7aa9f",
	}
	infraNWOnDeleteSilentJavaExecutions = []string{
		"InfraNamedWindowSilentDeleteOnDelete",
		"InfraNamedWindowSilentDeleteOnDeleteMany",
	}
	infraNWOnDeleteSilentJavaStaticIDs = []string{
		"java-06bf0eb71230b3119293",
		"java-06bf0eb71230b3119293",
	}
	infraNWOnDeleteSilentCases = []string{
		"silent-delete",
		"silent-delete-many",
	}
	infraNWOnDeleteSilentOrdinals = []int{5, 6}
	infraNWOnDeleteSilentEPLs     = []string{
		infraNWOnDeleteSilentModule,
		infraNWOnDeleteSilentModuleMany,
	}
)

// infraNWOnDeleteSilentBean mirrors the full SupportBean schema: the Java
// listener asserts read only theString/intPrimitive but the trace records the
// complete row, so every declared property is carried with Java defaults.
type infraNWOnDeleteSilentBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntBoxed        *int64   `esper:"intBoxed"`
	CharPrimitive   string   `esper:"charPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *string  `esper:"bigDecimal"`
	BigInteger      *string  `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

type infraNWOnDeleteSilentS0 struct {
	ID  int64   `esper:"id"`
	P00 *string `esper:"p00"`
}

func runInfraNWOnDeleteSilentScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	return executeInfraNWOnDeleteSilent(ctx, scenario.Steps)
}

func loadInfraNWOnDeleteSilentScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWOnDeleteSilentID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWOnDeleteSilentID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOnDeleteSilentID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOnDeleteSilentID, err)
	}
	if err := requireInfraNWOnDeleteSilentFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWOnDeleteSilentID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWOnDeleteSilentID ||
		metadata.Description != infraNWOnDeleteSilentDescription ||
		metadata.JavaCommit != infraNWOnDeleteSilentJavaCommit ||
		metadata.JavaSource != infraNWOnDeleteSilentSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWOnDeleteSilentID)
	}
	if err := validateInfraNWOnDeleteSilentStringArray(root["javaRuntimes"], infraNWOnDeleteSilentJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOnDeleteSilentStringArray(root["javaNames"], infraNWOnDeleteSilentJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOnDeleteSilentStringArray(root["javaStaticIds"], infraNWOnDeleteSilentJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOnDeleteSilentStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWOnDeleteSilentCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWOnDeleteSilentID, len(infraNWOnDeleteSilentCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWOnDeleteSilentFields(object,
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
		if definition.Case != infraNWOnDeleteSilentCases[index] ||
			definition.Ordinal != infraNWOnDeleteSilentOrdinals[index] ||
			definition.RuntimeID != infraNWOnDeleteSilentJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWOnDeleteSilentJavaExecutions[index] ||
			definition.Observation != infraNWOnDeleteSilentCaseObservations[index] ||
			definition.EPL != infraNWOnDeleteSilentEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWOnDeleteSilentID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWOnDeleteSilentID)
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
			if err := requireInfraNWOnDeleteSilentFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWOnDeleteSilentFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWOnDeleteSilentFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWOnDeleteSilentFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWOnDeleteSilentPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWOnDeleteSilentFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWOnDeleteSilentRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func requireInfraNWOnDeleteSilentFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWOnDeleteSilentStringArray(raw json.RawMessage, expected []string, name string) error {
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

var infraNWOnDeleteSilentCaseObservations = []string{
	"listener; silent delete over the length(2) window: create sees inserts and the length-2 expiry IR pair but never the silent deletes, delete publishes deleted rows as new data, count still observes every delta",
	"listener; silent delete-all over the groupwin(theString)#length(2) window: delete publishes all four rows as new data, count drops to zero, create stays silent",
}

// infraNWOnDeleteSilentCaseSteps pins the complete step sequence per case:
// case marker, one module deploy, four deployed markers in EPL order, the
// send sequence, undeploy-all.
var infraNWOnDeleteSilentCaseSteps = map[string][]string{
	"silent-delete": {
		"deploy:module:" + infraNWOnDeleteSilentEPLs[0],
		"deployed:create",
		"deployed:insert",
		"deployed:delete",
		"deployed:count",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"E4\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"E4\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"E3\"}",
		"send:SupportBean_S0:{\"id\":0,\"p00\":\"EX\"}",
		"undeploy-all",
	},
	"silent-delete-many": {
		"deploy:module:" + infraNWOnDeleteSilentEPLs[1],
		"deployed:create",
		"deployed:insert",
		"deployed:delete",
		"deployed:count",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"A\"}",
		"send:SupportBean:{\"intPrimitive\":2,\"theString\":\"A\"}",
		"send:SupportBean:{\"intPrimitive\":3,\"theString\":\"B\"}",
		"send:SupportBean:{\"intPrimitive\":4,\"theString\":\"B\"}",
		"send:SupportBean_S0:{\"id\":0}",
		"undeploy-all",
	},
}

func validateInfraNWOnDeleteSilentRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWOnDeleteSilentCases {
		want, ok := infraNWOnDeleteSilentCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWOnDeleteSilentID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWOnDeleteSilentID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWOnDeleteSilentID, caseName)
		}
		offset++
		for _, pinned := range want {
			var step struct {
				Op        string          `json:"op"`
				Case      string          `json:"case"`
				Statement string          `json:"statement"`
				EPL       string          `json:"epl"`
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWOnDeleteSilentID, offset, err)
			}
			if step.Case != caseName {
				return fmt.Errorf("%s step %d is not pinned for case %q", infraNWOnDeleteSilentID, offset, caseName)
			}
			var key string
			switch step.Op {
			case "deploy":
				key = "deploy:" + step.Statement + ":" + step.EPL
			case "deployed":
				key = "deployed:" + step.Statement
			case "send":
				var payload map[string]any
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOnDeleteSilentID, offset, err)
				}
				canonical, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOnDeleteSilentID, offset, err)
				}
				key = "send:" + step.EventType + ":" + string(canonical)
			case "undeploy-all":
				key = "undeploy-all"
			default:
				return fmt.Errorf("%s step %d has unsupported op %q", infraNWOnDeleteSilentID, offset, step.Op)
			}
			if key != pinned {
				return fmt.Errorf("%s step %d is not pinned: got %q want %q", infraNWOnDeleteSilentID, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWOnDeleteSilentID)
	}
	return nil
}

func executeInfraNWOnDeleteSilent(ctx context.Context, steps []compat.Step) (compat.Trace, error) {
	caseName := ""
	caseIndex := -1
	var env *esper.Environment
	var engine *esper.Engine
	var deployments []*esper.Deployment
	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWOnDeleteSilentID}
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
	// Each case runs on a fresh environment+engine, mirroring the Java
	// execution's undeployAll boundary (the named window lives in the
	// environment, not in a deployment).
	startCase := func() error {
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[infraNWOnDeleteSilentBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNWOnDeleteSilentS0](env, "SupportBean_S0"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithRuntimeURI(infraNWOnDeleteSilentJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(time.Unix(0, 0).UTC()))
		return nil
	}
	defer func() {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
	}()

	for _, step := range steps {
		switch step.Op {
		case "case":
			caseName = step.Case
			caseIndex++
			sequence = map[string]uint64{}
			if err := startCase(); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			plans, err := infraNWOnDeleteSilentBuild(env, caseName)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", caseName, err)
			}
			for _, plan := range plans {
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", caseName, err)
				}
				deployments = append(deployments, deployment)
				for _, statement := range deployment.Statements() {
					name := statement.Name()
					if name != "create" && name != "delete" && name != "count" {
						continue
					}
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "deployed":
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			event, err := decodeInfraNWOnDeleteSilentPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "undeploy-all":
			for _, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, err
				}
			}
			deployments = nil
			if err := engine.Close(ctx); err != nil {
				return compat.Trace{}, err
			}
			engine = nil
		default:
			return compat.Trace{}, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

// infraNWOnDeleteSilentBuild mirrors the Java module: create window + wildcard
// insert + silent-delete trigger + count consumer. The window is not public,
// so the Go side deploys the four statements as separate plans in the same
// environment — the module boundary is a Java packaging detail with no
// observable effect.
func infraNWOnDeleteSilentBuild(env *esper.Environment, caseName string) ([]esper.Plan, error) {
	schema, ok := env.Schema("SupportBean")
	if !ok {
		return nil, fmt.Errorf("SupportBean schema is missing")
	}
	retention := esper.WindowSpec(esper.LengthWindow(2))
	if caseName == "silent-delete-many" {
		retention = esper.GroupWindow(esper.Field[infraNWOnDeleteSilentBean, string]("theString"), esper.LengthWindow(2))
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(retention)); err != nil {
		return nil, err
	}

	sourceBean := esper.From[infraNWOnDeleteSilentBean](env, "SupportBean")
	sourceS0 := esper.From[infraNWOnDeleteSilentS0](env, "SupportBean_S0")

	// The Java 'create' statement's own listener is the named window's direct
	// child: select * over the window with the remove stream (silent deletes
	// strip it via the hint; the length-2 expiry IR pair still arrives).
	createPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").
		CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	if err != nil {
		return nil, err
	}

	insertPlan, err := env.Build(esper.OnEvent(sourceBean).
		InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
		Query(esper.StatementName("insert")))
	if err != nil {
		return nil, err
	}

	silentHint, err := esper.NewStatementHint(esper.HintSilentDelete)
	if err != nil {
		return nil, err
	}
	var deleteQuery esper.TriggerQuery
	if caseName == "silent-delete-many" {
		deleteQuery = esper.OnEvent(sourceS0).DeleteAllFromNamedWindow("MyWindow")
	} else {
		deleteQuery = esper.OnEvent(sourceS0).DeleteFromNamedWindow("MyWindow",
			esper.Equal[string](
				esper.Field[infraNWOnDeleteSilentS0, string]("p00"),
				esper.NamedWindowField[string]("theString")))
	}
	deletePlan, err := env.Build(deleteQuery.Query(
		esper.StatementName("delete"),
		esper.WithStatementHints(silentHint)))
	if err != nil {
		return nil, err
	}

	countPlan, err := env.Build(esper.FromNamedWindow(env, "MyWindow").
		Aggregate(esper.Alias("cnt", esper.CountAll())).
		Query(esper.StatementName("count")))
	if err != nil {
		return nil, err
	}
	return []esper.Plan{createPlan, insertPlan, deletePlan, countPlan}, nil
}

func decodeInfraNWOnDeleteSilentPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		var bean infraNWOnDeleteSilentBean
		bean.CharPrimitive = "\u0000"
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, fmt.Errorf("decode SupportBean.%s: %w", key, err)
			}
			switch key {
			case "theString":
				bean.TheString = value.(string)
			case "intPrimitive":
				bean.IntPrimitive = int64(value.(float64))
			default:
				return nil, fmt.Errorf("SupportBean payload has unknown field %q", key)
			}
		}
		return bean, nil
	case "SupportBean_S0":
		var event infraNWOnDeleteSilentS0
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, fmt.Errorf("decode SupportBean_S0.%s: %w", key, err)
			}
			switch key {
			case "id":
				event.ID = int64(value.(float64))
			case "p00":
				text := value.(string)
				event.P00 = &text
			default:
				return nil, fmt.Errorf("SupportBean_S0 payload has unknown field %q", key)
			}
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", step.EventType)
	}
}
