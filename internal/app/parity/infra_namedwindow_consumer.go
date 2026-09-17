package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNWConsumerID         = "infra-namedwindow-consumer"
	infraNWConsumerJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWConsumerSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowConsumer.java"
)

const infraNWConsumerDescription = "InfraNamedWindowConsumer named-window consumer semantics: keepall irstream select, length-window aggregate consumer, and an expr_batch insert-into chain (ords 0-2). listener records capture the irstream select's per-insert new rows and the aggregate consumer's per-update new/old pairs (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowConsumer.java)."

var infraNWConsumerCaseObservations = []string{
	"listener; a keepall named window feeds an irstream select consumer that observes each insert as a new-only row carrying the full SupportBean shape",
	"listener; a length(2) named window feeds an aggregate consumer that re-evaluates per window update, binding theString to the triggering event and sum(intPrimitive) over the retained rows, delivered as one new/old pair per update",
	"deployed; an expr_batch(current_count >= 10000) view buffers 10000 IncomingEvent sends then releases them through insert-into into a keepall named window; the execution attaches no listener so only deployment markers are observable",
}

var infraNWConsumerCaseEPLs = []string{
	"@Name('create') create window MyWindow.win:keepall() as SupportBean;\n@Name('insert') insert into MyWindow select * from SupportBean;\n@Name('select') select irstream * from MyWindow;\n",
	"create window MyWindow#length(2) as SupportBean;\ninsert into MyWindow select * from SupportBean;\n@name('s0') select theString as c0, sum(intPrimitive) as c1 from MyWindow;\n",
	"@buseventtype @public create schema IncomingEvent(id int);\ncreate schema RetainedEvent(id int);\ninsert into RetainedEvent select * from IncomingEvent#expr_batch(current_count >= 10000);\ncreate window RetainedEventWindow#keepall as RetainedEvent;\ninsert into RetainedEventWindow select * from RetainedEvent;\n",
}

var (
	infraNWConsumerJavaRuntimeIDs = []string{
		"java-runtime-c2d6b88fc4d77c643aab",
		"java-runtime-a707367e42b2736c2fcb",
		"java-runtime-229ba7f65962eb4594a1",
	}
	infraNWConsumerJavaExecutions = []string{
		"InfraNamedWindowConsumerKeepAll",
		"InfraNamedWindowConsumerLengthWin",
		"InfraNamedWindowConsumerWBatch",
	}
	infraNWConsumerJavaStaticIDs = []string{
		"java-04f7e86e470bf275affa",
		"java-04f7e86e470bf275affa",
		"java-04f7e86e470bf275affa",
	}
	infraNWConsumerJavaFlags = []string{"EXCLUDEWHENINSTRUMENTED"}
	infraNWConsumerCases     = []string{"keepall", "lengthwin", "wbatch"}
	infraNWConsumerOrdinals  = []int{0, 1, 2}
	infraNWConsumerSources   = []string{infraNWConsumerSource}
)

// infraNWConsumerCaseSteps pins the complete step sequence per case: deploy
// steps carry their byte-exact statement EPL, send steps carry the compacted
// payload, and wbatch's 10000 identical sends collapse into one send-batch
// step.
var infraNWConsumerCaseSteps = map[string][]string{
	"keepall": {
		"deploy:create:@Name('create') create window MyWindow.win:keepall() as SupportBean",
		"deployed:create",
		"deploy:insert:@Name('insert') insert into MyWindow select * from SupportBean",
		"deployed:insert",
		"deploy:select:@Name('select') select irstream * from MyWindow",
		"deployed:select",
		"send:SupportBean:{\"theString\":\"E1\",\"intPrimitive\":10}",
		"send:SupportBean:{\"theString\":\"E2\",\"intPrimitive\":10}",
		"undeploy-all:",
	},
	"lengthwin": {
		"deploy:create:create window MyWindow#length(2) as SupportBean",
		"deployed:create",
		"deploy:insert:insert into MyWindow select * from SupportBean",
		"deployed:insert",
		"deploy:s0:@name('s0') select theString as c0, sum(intPrimitive) as c1 from MyWindow",
		"deployed:s0",
		"send:SupportBean:{\"theString\":\"E1\",\"intPrimitive\":10}",
		"send:SupportBean:{\"theString\":\"E2\",\"intPrimitive\":20}",
		"send:SupportBean:{\"theString\":\"E3\",\"intPrimitive\":25}",
		"send:SupportBean:{\"theString\":\"E4\",\"intPrimitive\":26}",
		"undeploy-all:",
	},
	"wbatch": {
		"deploy:schema-incoming:@buseventtype @public create schema IncomingEvent(id int)",
		"deployed:schema-incoming",
		"deploy:schema-retained:create schema RetainedEvent(id int)",
		"deployed:schema-retained",
		"deploy:insert-retained:insert into RetainedEvent select * from IncomingEvent#expr_batch(current_count >= 10000)",
		"deployed:insert-retained",
		"deploy:create-window:create window RetainedEventWindow#keepall as RetainedEvent",
		"deployed:create-window",
		"deploy:insert-window:insert into RetainedEventWindow select * from RetainedEvent",
		"deployed:insert-window",
		"send-batch:IncomingEvent:{\"id\":1}:10000",
		"undeploy-all:",
	},
}

func executeInfraNWConsumer(ctx context.Context, steps []compat.Step) (compat.Trace, error) {
	caseName := ""
	caseIndex := -1
	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWConsumerID}
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
	startCase := func() error {
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[infraNWProcessingOrderBean](env, "SupportBean"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithRuntimeURI(infraNWConsumerJavaRuntimeIDs[caseIndex]),
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
			deployments = map[string]*esper.Deployment{}
			statements = map[string]*esper.Statement{}
			if err := startCase(); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			plan, err := infraNWConsumerBuild(env, caseName, step.Statement)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			// Schema statements register env-level and produce no plan; Java
			// deploys them as statements but they carry no runtime behavior.
			if plan.Hash() == "" {
				continue
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			for _, statement := range deployment.Statements() {
				statements[step.Statement] = statement
				if infraNWConsumerListened(caseName, step.Statement) {
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
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			fields, err := decodeInfraNWConsumerPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := infraNWConsumerSend(ctx, engine, step.EventType, fields); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "send-batch":
			fields, count, err := decodeInfraNWConsumerBatchPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			for i := range count {
				if err := infraNWConsumerSend(ctx, engine, step.EventType, fields); err != nil {
					return compat.Trace{}, fmt.Errorf("send-batch %s #%d: %w", step.EventType, i, err)
				}
			}
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
			statements = map[string]*esper.Statement{}
		default:
			return compat.Trace{}, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

func infraNWConsumerListened(caseName, statement string) bool {
	switch caseName {
	case "keepall":
		return statement == "select"
	case "lengthwin":
		return statement == "s0"
	}
	return false
}

func infraNWConsumerBuild(env *esper.Environment, caseName, statement string) (esper.Plan, error) {
	mapSchema := func(name string, fields ...esper.FieldSpec) (esper.Schema, error) {
		return esper.RegisterMap(env, name, fields)
	}
	switch caseName {
	case "keepall":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema not registered")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.From[infraNWProcessingOrderBean](env, "SupportBean").
				InsertInto("MyWindow", esper.StatementName("insert")))
		case "select":
			// select irstream * projects the full SupportBean shape.
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				Select(infraNWProcessingOrderBeanSelections()...).
				Query(esper.StatementName("select"), esper.WithOldStream()))
		}
	case "lengthwin":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema not registered")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.LengthWindow(2))); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.From[infraNWProcessingOrderBean](env, "SupportBean").
				InsertInto("MyWindow", esper.StatementName("insert")))
		case "s0":
			// select theString as c0, sum(intPrimitive) as c1: theString binds
			// to the triggering event, the sum re-evaluates over the retained
			// window rows on every update. No irstream keyword: Java gates
			// old-row generation on isSelectRStream so deliveries are
			// new-only even when a window row expires.
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				Aggregate(
					esper.Alias("c0", esper.Field[any, string]("theString")),
					esper.Alias("c1", esper.Sum[int64](esper.Field[any, int64]("intPrimitive"))),
				).
				Query(esper.StatementName("s0")))
		}
	case "wbatch":
		switch statement {
		case "schema-incoming":
			_, err := mapSchema("IncomingEvent", esper.FieldDef("id", reflect.TypeOf(int64(0))))
			return esper.Plan{}, err
		case "schema-retained":
			_, err := mapSchema("RetainedEvent", esper.FieldDef("id", reflect.TypeOf(int64(0))))
			return esper.Plan{}, err
		case "insert-retained":
			return env.Build(esper.FromAny(env, "IncomingEvent").
				Window(esper.ExpressionBatch(
					esper.GreaterOrEqual[int64](esper.WindowCurrentCount(), esper.Literal(int64(10000))))).
				InsertInto("RetainedEvent", esper.StatementName("insert-retained")))
		case "create-window":
			schema, ok := env.Schema("RetainedEvent")
			if !ok {
				return esper.Plan{}, fmt.Errorf("RetainedEvent schema not registered")
			}
			if _, err := esper.CreateNamedWindow(env, "RetainedEventWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "RetainedEventWindow").
				CreateNamedWindowQuery(esper.StatementName("create-window"), esper.WithOldStream()))
		case "insert-window":
			return env.Build(esper.FromAny(env, "RetainedEvent").
				InsertInto("RetainedEventWindow", esper.StatementName("insert-window")))
		}
	}
	return esper.Plan{}, fmt.Errorf("%s case %q statement %q has no plan", infraNWConsumerID, caseName, statement)
}

func infraNWConsumerSend(ctx context.Context, engine *esper.Engine, eventType string, fields map[string]json.RawMessage) error {
	if eventType == "SupportBean" {
		// Java's `new SupportBean()` leaves charPrimitive at the char default
		// '\u0000', which the trace serializer emits as "\u0000".
		bean := infraNWProcessingOrderBean{CharPrimitive: "\u0000"}
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "theString":
				bean.TheString = stringField(value)
			case "intPrimitive":
				bean.IntPrimitive = int64Field(value)
			}
		}
		return engine.Send(ctx, eventType, bean)
	}
	if eventType == "IncomingEvent" {
		row := map[string]any{}
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			row[key] = int64Field(value)
		}
		return engine.Send(ctx, eventType, row)
	}
	return fmt.Errorf("unknown event type %q", eventType)
}

func decodeInfraNWConsumerPayload(step compat.Step) (map[string]json.RawMessage, error) {
	var payload struct {
		Payload json.RawMessage `json:"payload"`
	}
	raw, err := json.Marshal(step)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := strictObject(payload.Payload, &fields); err != nil {
		return nil, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	return fields, nil
}

func decodeInfraNWConsumerBatchPayload(step compat.Step) (map[string]json.RawMessage, int, error) {
	var payload struct {
		Payload json.RawMessage `json:"payload"`
		Count   int             `json:"count"`
	}
	raw, err := json.Marshal(step)
	if err != nil {
		return nil, 0, err
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, 0, err
	}
	var fields map[string]json.RawMessage
	if err := strictObject(payload.Payload, &fields); err != nil {
		return nil, 0, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	return fields, payload.Count, nil
}

func runInfraNWConsumerScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	return executeInfraNWConsumer(ctx, scenario.Steps)
}

func loadInfraNWConsumerScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWConsumerID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWConsumerID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWConsumerID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWConsumerID, err)
	}
	if err := requireInfraNWConsumerFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWConsumerID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWConsumerID ||
		metadata.Description != infraNWConsumerDescription ||
		metadata.JavaCommit != infraNWConsumerJavaCommit ||
		metadata.JavaSource != infraNWConsumerSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWConsumerID)
	}
	if err := validateInfraNWConsumerStringArray(root["javaRuntimes"], infraNWConsumerJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWConsumerStringArray(root["javaNames"], infraNWConsumerJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWConsumerStringArray(root["javaStaticIds"], infraNWConsumerJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWConsumerStringArray(root["javaFlags"], infraNWConsumerJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWConsumerCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWConsumerID, len(infraNWConsumerCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWConsumerFields(object,
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
		if definition.Case != infraNWConsumerCases[index] ||
			definition.Ordinal != infraNWConsumerOrdinals[index] ||
			definition.RuntimeID != infraNWConsumerJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWConsumerJavaExecutions[index] ||
			definition.Observation != infraNWConsumerCaseObservations[index] ||
			definition.EPL != infraNWConsumerCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWConsumerID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", infraNWConsumerID, err)
	}
	offset := 0
	for _, caseName := range infraNWConsumerCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", infraNWConsumerID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraNWConsumerID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", infraNWConsumerID, offset, caseName)
		}
		offset++
		want, ok := infraNWConsumerCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", infraNWConsumerID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", infraNWConsumerID, caseName)
		}
		for _, pinned := range want {
			var step struct {
				Op        string          `json:"op"`
				Case      string          `json:"case"`
				Statement string          `json:"statement"`
				EventType string          `json:"eventType"`
				Epl       string          `json:"epl"`
				Payload   json.RawMessage `json:"payload"`
				Count     int             `json:"count"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraNWConsumerID, offset, err)
			}
			if step.Case != caseName {
				return compat.Scenario{}, fmt.Errorf("%s step %d case = %q, want %q", infraNWConsumerID, offset, step.Case, caseName)
			}
			key := step.Op + ":" + step.Statement + step.EventType
			if step.Op == "send" {
				var compacted bytes.Buffer
				if err := json.Compact(&compacted, step.Payload); err != nil {
					return compat.Scenario{}, fmt.Errorf("%s step %d payload: %w", infraNWConsumerID, offset, err)
				}
				key += ":" + compacted.String()
			}
			if step.Op == "send-batch" {
				var compacted bytes.Buffer
				if err := json.Compact(&compacted, step.Payload); err != nil {
					return compat.Scenario{}, fmt.Errorf("%s step %d payload: %w", infraNWConsumerID, offset, err)
				}
				key += ":" + compacted.String() + fmt.Sprintf(":%d", step.Count)
			}
			if step.Op == "deploy" {
				key += ":" + step.Epl
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", infraNWConsumerID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", infraNWConsumerID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWConsumerID, err)
	}
	return scenario, nil
}

func requireInfraNWConsumerFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", infraNWConsumerID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", infraNWConsumerID, name)
		}
	}
	return nil
}

func validateInfraNWConsumerStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
