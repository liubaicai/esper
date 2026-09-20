package parity

import (
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
	infraNWTableOnMergeInsertStreamID          = "infra-nwtable-on-merge-insertstream"
	infraNWTableOnMergeInsertStreamDescription = "InfraNWTableOnMerge ordinals 8-19: InfraInsertOtherStream event-representation matrix — MyEvent merges into a #unique(name) named window or a composite-primary-key table over OBJECTARRAY, MAP, AVRO, JSON, JSONCLASSPROVIDED and DEFAULT payloads; each trigger routes event_name plus the matched target value (0d when not matched) into OtherStreamOne observed by the s0 listener, so the named window's first event is not matched (0d, then previous-value matches 10d/11d) while the table's first event is matched (10d) (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java)."
	infraNWTableOnMergeInsertStreamJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableOnMergeInsertStreamSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java"
)

var (
	infraNWTableOnMergeInsertStreamJavaSources = []string{
		infraNWTableOnMergeInsertStreamSource,
	}
	infraNWTableOnMergeInsertStreamJavaRuntimeIDs = []string{
		"java-runtime-c91b3df37512cfac454f",
		"java-runtime-b54b14998ff59e65f2fc",
		"java-runtime-945ba0790a1cfa2b1033",
		"java-runtime-fef4b3a46f9b7da65a72",
		"java-runtime-d8cb24fc9e29421236e2",
		"java-runtime-e22957dc02c3ac4b8bb7",
		"java-runtime-284497e8a9729bb76ac2",
		"java-runtime-f6718d2a96a3e3668239",
		"java-runtime-51773f11823a58156447",
		"java-runtime-b891f1f54423ccb8a628",
		"java-runtime-dd95faed8e31c95bec9f",
		"java-runtime-6e68e972f23cef8dde85",
	}
	infraNWTableOnMergeInsertStreamJavaExecutions = []string{
		"InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=OBJECTARRAY}",
		"InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=OBJECTARRAY}",
		"InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=MAP}",
		"InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=MAP}",
		"InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=AVRO}",
		"InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=AVRO}",
		"InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=JSON}",
		"InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=JSON}",
		"InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=JSONCLASSPROVIDED}",
		"InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=JSONCLASSPROVIDED}",
		"InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=DEFAULT}",
		"InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=DEFAULT}",
	}
	infraNWTableOnMergeInsertStreamJavaStaticIDs = []string{
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
		"java-8304a4459a4ea865bf1f",
	}
	infraNWTableOnMergeInsertStreamCases = []string{
		"insertstream-nw-objectarray",
		"insertstream-table-objectarray",
		"insertstream-nw-map",
		"insertstream-table-map",
		"insertstream-nw-avro",
		"insertstream-table-avro",
		"insertstream-nw-json",
		"insertstream-table-json",
		"insertstream-nw-jsonclassprovided",
		"insertstream-table-jsonclassprovided",
		"insertstream-nw-default",
		"insertstream-table-default",
	}
	infraNWTableOnMergeInsertStreamOrdinals = []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
)

// infraNWTableOnMergeInsertStreamJSONProvided* stand in for Java's
// InfraNWTableOnMerge.MyLocalJsonProvided* classes used by the
// JSONCLASSPROVIDED variant's @JsonSchema annotations: one provided class
// per declared event type.
type infraNWTableOnMergeInsertStreamJSONProvidedMyEvent struct {
	Name  string  `esper:"name" json:"name"`
	Value float64 `esper:"value" json:"value"`
}

type infraNWTableOnMergeInsertStreamJSONProvidedInputEvent struct {
	Col1 string  `esper:"col1" json:"col1"`
	Col2 float64 `esper:"col2" json:"col2"`
}

// infraNWTableOnMergeInsertStreamDefault* back the DEFAULT variant's
// create-schema declarations, which carry no @EventRepresentation
// annotation.
type infraNWTableOnMergeInsertStreamDefaultMyEvent struct {
	Name  string  `esper:"name" json:"name"`
	Value float64 `esper:"value" json:"value"`
}

type infraNWTableOnMergeInsertStreamDefaultInputEvent struct {
	Col1 string  `esper:"col1" json:"col1"`
	Col2 float64 `esper:"col2" json:"col2"`
}

func infraNWTableOnMergeInsertStreamIsTable(caseName string) bool {
	return len(caseName) > len("insertstream-table-") &&
		caseName[:len("insertstream-table-")] == "insertstream-table-"
}

func infraNWTableOnMergeInsertStreamRepr(caseName string) string {
	if infraNWTableOnMergeInsertStreamIsTable(caseName) {
		return caseName[len("insertstream-table-"):]
	}
	return caseName[len("insertstream-nw-"):]
}

// runInfraNWTableOnMergeInsertStreamScenario replays the twelve
// InfraInsertOtherStream executions: each case deploys the pinned
// six-statement module, sends the MyEvent triggers in the case's event
// representation, and records the s0 listener batches in Java's observable
// order.
func runInfraNWTableOnMergeInsertStreamScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableOnMergeInsertStreamCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableOnMergeInsertStreamCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableOnMergeInsertStreamID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeInsertStreamID)
	}
	return trace, nil
}

func runInfraNWTableOnMergeInsertStreamCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	isTable := infraNWTableOnMergeInsertStreamIsTable(caseName)
	if err := infraNWTableOnMergeInsertStreamCreateInfra(env, caseName, isTable); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableOnMergeInsertStreamJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableOnMergeInsertStreamID}
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
	// The Java module deploy binds every statement positionally; schema and
	// infra labels are environment-level on the Go side and have no deployed
	// statement, so they are tracked as known labels only.
	knownLabels := map[string]bool{}
	for _, label := range infraNWTableOnMergeInsertStreamModuleLabels() {
		knownLabels[label] = true
	}
	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plans, err := infraNWTableOnMergeInsertStreamBuild(env, isTable)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			for _, bound := range plans {
				deployment, err := engine.Deploy(ctx, bound.plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", bound.label, err)
				}
				deployments = append(deployments, deployment)
				for _, statement := range deployment.Statements() {
					if statement.Name() == "s0" {
						if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
							record("s0", batch)
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
			if err := infraNWTableOnMergeInsertStreamSend(ctx, env, engine, step); err != nil {
				return compat.Trace{}, err
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

// infraNWTableOnMergeInsertStreamModuleLabels lists the module statements
// in EPL order; the Java oracle binds deployment.getStatements()
// positionally to these labels.
func infraNWTableOnMergeInsertStreamModuleLabels() []string {
	return []string{"schema-myevent", "infra", "insert", "schema-input", "merge", "s0"}
}

// infraNWTableOnMergeInsertStreamCreateInfra registers the
// environment-level artifacts the Java module's @public statements
// establish: the MyEvent and dead InputEvent schemas in the case's event
// representation, the MyInfraIOS unique-key named window or
// composite-primary-key table, and the OtherStreamOne side-stream type the
// merge's insert actions route into.
func infraNWTableOnMergeInsertStreamCreateInfra(env *esper.Environment, caseName string, isTable bool) error {
	repr := infraNWTableOnMergeInsertStreamRepr(caseName)
	myEventFields := []esper.FieldSpec{
		esper.FieldDef("name", reflect.TypeOf("")),
		esper.FieldDef("value", reflect.TypeOf(float64(0))),
	}
	inputEventFields := []esper.FieldSpec{
		esper.FieldDef("col1", reflect.TypeOf("")),
		esper.FieldDef("col2", reflect.TypeOf(float64(0))),
	}
	var myEventSchema esper.Schema
	var err error
	switch repr {
	case "objectarray":
		if myEventSchema, err = esper.RegisterObjectArray(env, "MyEvent", myEventFields, esper.BusEventType()); err != nil {
			return err
		}
		if _, err = esper.RegisterObjectArray(env, "InputEvent", inputEventFields); err != nil {
			return err
		}
	case "map":
		if myEventSchema, err = esper.RegisterMap(env, "MyEvent", myEventFields, esper.BusEventType()); err != nil {
			return err
		}
		if _, err = esper.RegisterMap(env, "InputEvent", inputEventFields); err != nil {
			return err
		}
	case "avro":
		if myEventSchema, err = esper.RegisterAvro(env, "MyEvent", myEventFields, esper.BusEventType()); err != nil {
			return err
		}
		if _, err = esper.RegisterAvro(env, "InputEvent", inputEventFields); err != nil {
			return err
		}
	case "json":
		if myEventSchema, err = esper.RegisterJSON(env, "MyEvent", myEventFields, esper.BusEventType()); err != nil {
			return err
		}
		if _, err = esper.RegisterJSON(env, "InputEvent", inputEventFields); err != nil {
			return err
		}
	case "jsonclassprovided":
		if myEventSchema, err = esper.RegisterJSONFor[infraNWTableOnMergeInsertStreamJSONProvidedMyEvent](env, "MyEvent", nil, esper.BusEventType()); err != nil {
			return err
		}
		if _, err = esper.RegisterJSONFor[infraNWTableOnMergeInsertStreamJSONProvidedInputEvent](env, "InputEvent", nil); err != nil {
			return err
		}
	case "default":
		if myEventSchema, err = esper.RegisterStruct[infraNWTableOnMergeInsertStreamDefaultMyEvent](env, "MyEvent", esper.BusEventType()); err != nil {
			return err
		}
		if _, err = esper.RegisterStruct[infraNWTableOnMergeInsertStreamDefaultInputEvent](env, "InputEvent"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown representation %q", repr)
	}
	if isTable {
		// create table MyInfraIOS (name string primary key, value double
		// primary key): both columns form the composite primary key.
		if _, err := esper.CreateTable(env, "MyInfraIOS", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("name"),
			esper.PrimaryKeyColumn[float64]("value"),
		}); err != nil {
			return err
		}
	} else {
		if _, err := esper.CreateNamedWindow(env, "MyInfraIOS", myEventSchema,
			esper.NamedWindowRetention(esper.Unique(esper.Field[any, string]("name")))); err != nil {
			return err
		}
	}
	// insert into OtherStreamOne creates the side-stream type implicitly in
	// Java; Go requires the route target registered with the projected
	// field set.
	_, err = esper.RegisterMap(env, "OtherStreamOne", []esper.FieldSpec{
		esper.FieldDef("event_name", reflect.TypeOf("")),
		esper.FieldDef("status", reflect.TypeOf(float64(0))),
	})
	return err
}

// infraNWTableOnMergeInsertStreamBoundPlan pairs one module statement
// label with the Go plan that produces the deployed statement.
type infraNWTableOnMergeInsertStreamBoundPlan struct {
	label string
	plan  esper.Plan
}

// infraNWTableOnMergeInsertStreamBuild mirrors the Java module: schema and
// infra statements are environment-level on the Go side, so only the
// insert feeder, the merge and the s0 consumer deploy. The module boundary
// is a Java packaging detail with no observable effect.
//
// Deploy order reproduces Java's observable ordering: for named windows
// the on-merge trigger fires before the insert-into route (merge deploys
// first), while for tables insert-into routes the row before the trigger
// evaluates (insert deploys first).
func infraNWTableOnMergeInsertStreamBuild(env *esper.Environment, isTable bool) ([]infraNWTableOnMergeInsertStreamBoundPlan, error) {
	source := esper.FromAny(env, "MyEvent")
	name := esper.Field[any, string]("name")
	value := esper.Field[any, float64]("value")

	var insertPlan esper.Plan
	var err error
	if isTable {
		insertPlan, err = env.Build(esper.OnRecord(source).InsertIntoTable("MyInfraIOS",
			esper.SetColumn("name", name),
			esper.SetColumn("value", value)).Query())
	} else {
		insertPlan, err = env.Build(esper.OnRecord(source).InsertIntoNamedWindow("MyInfraIOS",
			esper.SetColumn("name", name),
			esper.SetColumn("value", value)).Query())
	}
	if err != nil {
		return nil, err
	}

	// when matched then insert into OtherStreamOne select eme.name as
	// event_name, MyInfraIOS.value as status — the matched branch reads the
	// target row's value through the named-window or table field accessor.
	var targetValue esper.Expr
	if isTable {
		targetValue = esper.TableField[float64]("value")
	} else {
		targetValue = esper.NamedWindowField[float64]("value")
	}
	clauses := []esper.TableMergeClause{
		esper.WhenMatchedActions(
			esper.ThenInsertInto("OtherStreamOne",
				esper.Alias("event_name", name),
				esper.Alias("status", targetValue))),
		// when not matched then insert into OtherStreamOne select eme.name
		// as event_name, 0d as status
		esper.WhenNotMatchedActions(
			esper.ThenInsertInto("OtherStreamOne",
				esper.Alias("event_name", name),
				esper.Alias("status", esper.Literal(0.0)))),
	}
	var mergePlan esper.Plan
	if isTable {
		mergePlan, err = env.Build(esper.OnRecord(source).MergeIntoTableWhen("MyInfraIOS",
			[]esper.Expr{name, value}, clauses...).Query())
	} else {
		mergePlan, err = env.Build(esper.OnRecord(source).MergeIntoNamedWindowWhen("MyInfraIOS",
			esper.Equal[string](esper.NamedWindowField[string]("name"), name), clauses...).Query())
	}
	if err != nil {
		return nil, err
	}

	consumerPlan, err := env.Build(esper.FromAny(env, "OtherStreamOne").Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}

	if isTable {
		return []infraNWTableOnMergeInsertStreamBoundPlan{
			{label: "insert", plan: insertPlan},
			{label: "merge", plan: mergePlan},
			{label: "s0", plan: consumerPlan},
		}, nil
	}
	return []infraNWTableOnMergeInsertStreamBoundPlan{
		{label: "merge", plan: mergePlan},
		{label: "insert", plan: insertPlan},
		{label: "s0", plan: consumerPlan},
	}, nil
}

// infraNWTableOnMergeInsertStreamSend decodes and sends one pinned MyEvent
// trigger in the case's event representation, mirroring
// makeSendNameValueEvent: positional object array for OBJECTARRAY,
// name/value map for MAP and DEFAULT, a schema-bound Avro record for AVRO,
// and a JSON object for JSON and JSONCLASSPROVIDED.
func infraNWTableOnMergeInsertStreamSend(ctx context.Context, env *esper.Environment, engine *esper.Engine, step compat.Step) error {
	if step.EventType != "MyEvent" {
		return fmt.Errorf("unexpected event type %q", step.EventType)
	}
	spec, err := decodeInfraNWTableOnMergeInsertStreamPayload(step)
	if err != nil {
		return err
	}
	switch spec.repr {
	case "objectarray":
		return engine.SendObjectArray(ctx, step.EventType, []any{spec.name, spec.value})
	case "map", "default":
		return engine.SendRecord(ctx, step.EventType, map[string]any{"name": spec.name, "value": spec.value})
	case "avro":
		schema, ok := env.Schema(step.EventType)
		if !ok {
			return fmt.Errorf("%s schema not registered", step.EventType)
		}
		record, err := esper.NewAvroRecordFromMap(schema, map[string]any{"name": spec.name, "value": spec.value})
		if err != nil {
			return err
		}
		return engine.SendAvro(ctx, step.EventType, record)
	case "json", "jsonclassprovided":
		data, err := json.Marshal(map[string]any{"name": spec.name, "value": spec.value})
		if err != nil {
			return err
		}
		return engine.SendJSON(ctx, step.EventType, data)
	default:
		return fmt.Errorf("unknown representation %q", spec.repr)
	}
}

type infraNWTableOnMergeInsertStreamSendSpec struct {
	repr  string
	name  string
	value float64
}

// decodeInfraNWTableOnMergeInsertStreamPayload validates one send payload
// and extracts the pinned name/value pair.
func decodeInfraNWTableOnMergeInsertStreamPayload(step compat.Step) (infraNWTableOnMergeInsertStreamSendSpec, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return infraNWTableOnMergeInsertStreamSendSpec{}, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	if err := requireInfraNWTableOnMergeInsertStreamFields(fields, "repr", "fields"); err != nil {
		return infraNWTableOnMergeInsertStreamSendSpec{}, err
	}
	var inner map[string]json.RawMessage
	if err := strictObject(fields["fields"], &inner); err != nil {
		return infraNWTableOnMergeInsertStreamSendSpec{}, fmt.Errorf("decode %s fields: %w", step.EventType, err)
	}
	if err := requireInfraNWTableOnMergeInsertStreamFields(inner, "name", "value"); err != nil {
		return infraNWTableOnMergeInsertStreamSendSpec{}, err
	}
	var payload struct {
		Repr   string `json:"repr"`
		Fields struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return infraNWTableOnMergeInsertStreamSendSpec{}, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	return infraNWTableOnMergeInsertStreamSendSpec{repr: payload.Repr, name: payload.Fields.Name, value: payload.Fields.Value}, nil
}

func requireInfraNWTableOnMergeInsertStreamFields(object map[string]json.RawMessage, names ...string) error {
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

// loadInfraNWTableOnMergeInsertStreamScenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableOnMergeInsertStreamScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableOnMergeInsertStreamID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableOnMergeInsertStreamID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeInsertStreamID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableOnMergeInsertStreamID, err)
	}
	if err := requireInfraNWTableOnMergeInsertStreamFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableOnMergeInsertStreamID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableOnMergeInsertStreamID ||
		metadata.Description != infraNWTableOnMergeInsertStreamDescription ||
		metadata.JavaCommit != infraNWTableOnMergeInsertStreamJavaCommit ||
		metadata.JavaSource != infraNWTableOnMergeInsertStreamSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableOnMergeInsertStreamID)
	}
	if err := validateInfraNWTableOnMergeInsertStreamStringArray(root["javaRuntimes"], infraNWTableOnMergeInsertStreamJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeInsertStreamStringArray(root["javaNames"], infraNWTableOnMergeInsertStreamJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeInsertStreamStringArray(root["javaStaticIds"], infraNWTableOnMergeInsertStreamJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableOnMergeInsertStreamStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableOnMergeInsertStreamCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableOnMergeInsertStreamID, len(infraNWTableOnMergeInsertStreamCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableOnMergeInsertStreamFields(object,
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
		if definition.Case != infraNWTableOnMergeInsertStreamCases[index] ||
			definition.Ordinal != infraNWTableOnMergeInsertStreamOrdinals[index] ||
			definition.RuntimeID != infraNWTableOnMergeInsertStreamJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableOnMergeInsertStreamJavaExecutions[index] ||
			definition.Observation != infraNWTableOnMergeInsertStreamCaseObservations[index] ||
			definition.EPL != infraNWTableOnMergeInsertStreamCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableOnMergeInsertStreamID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableOnMergeInsertStreamID)
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
			if err := requireInfraNWTableOnMergeInsertStreamFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableOnMergeInsertStreamFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableOnMergeInsertStreamFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableOnMergeInsertStreamFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableOnMergeInsertStreamPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableOnMergeInsertStreamFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWTableOnMergeInsertStreamRawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWTableOnMergeInsertStreamStringArray(raw json.RawMessage, expected []string, name string) error {
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

// infraNWTableOnMergeInsertStreamAnnotation renders the per-statement
// annotation text mirroring
// EventRepresentationChoice.getAnnotationTextWJsonProvided: the Java
// source concatenates annotationText + " " before each statement, so
// DEFAULT's empty annotation leaves a leading space.
func infraNWTableOnMergeInsertStreamAnnotation(repr, provided string) string {
	switch repr {
	case "objectarray":
		return "@EventRepresentation('objectarray') "
	case "map":
		return "@EventRepresentation('map') "
	case "avro":
		return "@EventRepresentation('avro') "
	case "json":
		return "@EventRepresentation('json') "
	case "jsonclassprovided":
		return "@JsonSchema(className='com.espertech.esper.regressionlib.suite.infra.nwtable.InfraNWTableOnMerge$MyLocalJsonProvided" + provided + "') @EventRepresentation('json') "
	default:
		return " "
	}
}

// infraNWTableOnMergeInsertStreamModuleEPL renders the pinned module text
// for one case: the annotated MyEvent schema, the annotated window or
// unannotated composite-primary-key table, the insert-into feeder, the
// dead InputEvent schema, the blank line, the multi-line on-merge, and the
// s0 consumer.
func infraNWTableOnMergeInsertStreamModuleEPL(caseName string) string {
	repr := infraNWTableOnMergeInsertStreamRepr(caseName)
	isTable := infraNWTableOnMergeInsertStreamIsTable(caseName)
	infra := "@public create table MyInfraIOS (name string primary key, value double primary key);\n"
	if !isTable {
		infra = infraNWTableOnMergeInsertStreamAnnotation(repr, "MyEvent") +
			"@public create window MyInfraIOS#unique(name) as MyEvent;\n"
	}
	return infraNWTableOnMergeInsertStreamAnnotation(repr, "MyEvent") +
		"@public @buseventtype @public create schema MyEvent as (name string, value double);\n" +
		infra +
		"insert into MyInfraIOS select * from MyEvent;\n" +
		infraNWTableOnMergeInsertStreamAnnotation(repr, "InputEvent") +
		"create schema InputEvent as (col1 string, col2 double);\n" +
		"\n" +
		"on MyEvent as eme\n" +
		"  merge MyInfraIOS as MyInfraIOS where MyInfraIOS.name = eme.name\n" +
		"   when matched then\n" +
		"      insert into OtherStreamOne select eme.name as event_name, MyInfraIOS.value as status\n" +
		"   when not matched then\n" +
		"      insert into OtherStreamOne select eme.name as event_name, 0d as status;\n" +
		"@name('s0') select * from OtherStreamOne;\n"
}

// infraNWTableOnMergeInsertStreamCaseEPLs pins the module EPL of each
// case, the value carried by the scenario cases[] metadata.
var infraNWTableOnMergeInsertStreamCaseEPLs = []string{
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-objectarray"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-objectarray"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-map"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-map"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-avro"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-avro"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-json"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-json"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-jsonclassprovided"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-jsonclassprovided"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-default"),
	infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-default"),
}

var infraNWTableOnMergeInsertStreamCaseObservations = []string{
	"listener; objectarray positional MyEvent payload merges into the #unique(name) named window: the on-merge trigger fires before the insert-into route, so the first event is not matched (status=0d) and each later same-key event matches the previous value (10d then 11d); merge never mutates the target",
	"listener; objectarray positional MyEvent payload merges into the composite-primary-key table: insert-into routes the row before the on-merge trigger evaluates, so the first event is matched (status=10d); merge never mutates the target",
	"listener; map MyEvent payload merges into the #unique(name) named window: the on-merge trigger fires before the insert-into route, so the first event is not matched (status=0d) and each later same-key event matches the previous value (10d then 11d); merge never mutates the target",
	"listener; map MyEvent payload merges into the composite-primary-key table: insert-into routes the row before the on-merge trigger evaluates, so the first event is matched (status=10d); merge never mutates the target",
	"listener; Avro GenericData.Record MyEvent payload over the preconfigured schema merges into the #unique(name) named window: the on-merge trigger fires before the insert-into route, so the first event is not matched (status=0d) and each later same-key event matches the previous value (10d then 11d); merge never mutates the target",
	"listener; Avro GenericData.Record MyEvent payload over the preconfigured schema merges into the composite-primary-key table: insert-into routes the row before the on-merge trigger evaluates, so the first event is matched (status=10d); merge never mutates the target",
	"listener; JSON object MyEvent payload merges into the #unique(name) named window: the on-merge trigger fires before the insert-into route, so the first event is not matched (status=0d) and each later same-key event matches the previous value (10d then 11d); merge never mutates the target",
	"listener; JSON object MyEvent payload merges into the composite-primary-key table: insert-into routes the row before the on-merge trigger evaluates, so the first event is matched (status=10d); merge never mutates the target",
	"listener; JSON object MyEvent payload over the @JsonSchema provided class merges into the #unique(name) named window: the on-merge trigger fires before the insert-into route, so the first event is not matched (status=0d) and each later same-key event matches the previous value (10d then 11d); merge never mutates the target",
	"listener; JSON object MyEvent payload over the @JsonSchema provided class merges into the composite-primary-key table: insert-into routes the row before the on-merge trigger evaluates, so the first event is matched (status=10d); merge never mutates the target",
	"listener; default-representation map MyEvent payload merges into the #unique(name) named window: the on-merge trigger fires before the insert-into route, so the first event is not matched (status=0d) and each later same-key event matches the previous value (10d then 11d); merge never mutates the target",
	"listener; default-representation map MyEvent payload merges into the composite-primary-key table: insert-into routes the row before the on-merge trigger evaluates, so the first event is matched (status=10d); merge never mutates the target",
}

// validateInfraNWTableOnMergeInsertStreamRawSteps pins the complete step
// sequence per case against the raw JSON objects: one module deploy with
// byte-exact EPL, positional deployed markers, send event types with
// canonical payloads, and undeploy-all terminators.
func validateInfraNWTableOnMergeInsertStreamRawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableOnMergeInsertStreamCases {
		want, ok := infraNWTableOnMergeInsertStreamCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableOnMergeInsertStreamID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableOnMergeInsertStreamID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeInsertStreamID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableOnMergeInsertStreamID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableOnMergeInsertStreamStepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableOnMergeInsertStreamID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned", infraNWTableOnMergeInsertStreamID, caseName, index+1)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableOnMergeInsertStreamID)
	}
	return nil
}

// infraNWTableOnMergeInsertStreamStepKey renders a raw step object into
// its pinned string form.
func infraNWTableOnMergeInsertStreamStepKey(object map[string]json.RawMessage, operation string) (string, error) {
	stringField := func(name string) (string, error) {
		var value string
		if err := json.Unmarshal(object[name], &value); err != nil {
			return "", fmt.Errorf("step field %q must be a string", name)
		}
		return value, nil
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
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// infraNWTableOnMergeInsertStreamCaseSteps pins the exact op sequence per
// case: one module deploy with byte-exact EPL, positional deployed
// markers, the MyEvent sends with representation-tagged canonical
// payloads, and undeploy-all terminators. Named-window cases carry three
// sends; table cases carry one.
var infraNWTableOnMergeInsertStreamCaseSteps = map[string][]string{
	"insertstream-nw-objectarray": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-objectarray"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"objectarray\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":11},\"repr\":\"objectarray\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":12},\"repr\":\"objectarray\"}",
		"undeploy-all",
	},
	"insertstream-table-objectarray": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-objectarray"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"objectarray\"}",
		"undeploy-all",
	},
	"insertstream-nw-map": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-map"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"map\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":11},\"repr\":\"map\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":12},\"repr\":\"map\"}",
		"undeploy-all",
	},
	"insertstream-table-map": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-map"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"map\"}",
		"undeploy-all",
	},
	"insertstream-nw-avro": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-avro"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"avro\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":11},\"repr\":\"avro\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":12},\"repr\":\"avro\"}",
		"undeploy-all",
	},
	"insertstream-table-avro": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-avro"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"avro\"}",
		"undeploy-all",
	},
	"insertstream-nw-json": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-json"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"json\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":11},\"repr\":\"json\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":12},\"repr\":\"json\"}",
		"undeploy-all",
	},
	"insertstream-table-json": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-json"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"json\"}",
		"undeploy-all",
	},
	"insertstream-nw-jsonclassprovided": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-jsonclassprovided"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"jsonclassprovided\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":11},\"repr\":\"jsonclassprovided\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":12},\"repr\":\"jsonclassprovided\"}",
		"undeploy-all",
	},
	"insertstream-table-jsonclassprovided": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-jsonclassprovided"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"jsonclassprovided\"}",
		"undeploy-all",
	},
	"insertstream-nw-default": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-nw-default"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"default\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":11},\"repr\":\"default\"}",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":12},\"repr\":\"default\"}",
		"undeploy-all",
	},
	"insertstream-table-default": {
		"deploy:module:" + infraNWTableOnMergeInsertStreamModuleEPL("insertstream-table-default"),
		"deployed:schema-myevent",
		"deployed:infra",
		"deployed:insert",
		"deployed:schema-input",
		"deployed:merge",
		"deployed:s0",
		"send:MyEvent:{\"fields\":{\"name\":\"name1\",\"value\":10},\"repr\":\"default\"}",
		"undeploy-all",
	},
}
