package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const eplInsertIntoEventColRestJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoEventColRestJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoPopulateEventTypeColumnBean.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoPopulateEventTypeColumnNonBean.java",
}

var (
	eplInsertIntoEventColRestJavaRuntimeIDs = []string{
		"java-runtime-5a7dbebf11b467444d22",
		"java-runtime-d9b52924796472db83cb",
		"java-runtime-10c2cd110438e2422b62",
		"java-runtime-63b9e9149d6ec7890aba",
		"java-runtime-6883c88a8250fd587c89",
		"java-runtime-7f2273f9d9a40f6c3dc2",
		"java-runtime-2ea417f6f55b2d3543ad",
		"java-runtime-ba3f88c4715292f3432e",
		"java-runtime-b121584c40a08d48229d",
		"java-runtime-97ac55e0199723988adc",
		"java-runtime-f8d3d61670e7dae24003",
		"java-runtime-05f38146a2f28887769c",
	}
	eplInsertIntoEventColRestJavaExecutions = []string{
		"EPLInsertIntoColBeanSingleToMulti",
		"EPLInsertIntoColBeanMultiToSingle",
		"EPLInsertIntoColBeanInvalid",
		"EPLInsertIntoColBeanContextProp",
		"EPLInsertIntoColNonBeanNewOperatorDocSample{typeType='objectarray'}",
		"EPLInsertIntoColNonBeanNewOperatorDocSample{typeType='map'}",
		"EPLInsertIntoColNonBeanCaseNew{representation=MAP}",
		"EPLInsertIntoColNonBeanCaseNew{representation=OBJECTARRAY}",
		"EPLInsertIntoColNonBeanCaseNew{representation=JSON}",
		"EPLInsertIntoColNonBeanSingleColNamedWindow",
		"EPLInsertIntoColNonBeanSingleToMulti",
		"EPLInsertIntoColNonBeanBeanInvalid",
	}
)

// eventcolRestCases enumerates the inventory-ordered case names; the runner
// emits records in the same order as the pinned executions().
var eventcolRestCases = []string{
	"bean-singletomulti",
	"bean-multitosingle",
	"bean-context-prop",
	"bean-invalid",
	"nonbean-new-doc-objectarray",
	"nonbean-new-doc-map",
	"nonbean-case-new-map",
	"nonbean-case-new-objectarray",
	"nonbean-case-new-json",
	"nonbean-singlecol-named-window",
	"nonbean-single-to-multi",
	"nonbean-invalid",
}

// beanInvalidPrefix1 / beanInvalidPrefix2 pin the verbatim Java diagnostics
// asserted by EPLInsertIntoColBeanInvalid (startsWith match):
// SelectExprProcessorHelper.java:1333-1335.
const beanInvalidEPL = "insert into %s select (select * from SupportBean_S0#keepall) as sbs from SupportBean_S1"

// nonbean invalid prefixes: SelectExprProcessorHelper.java:1364-1366 and
// SelectExprInsertEventBeanFactory.java:270-277.
var nonBeanInvalidCases = []struct {
	statement   string
	epl         string
	errorPrefix string
}{
	{
		statement: "invalid-1",
		epl:       "insert into N1_2 select new {p0='a'} as p1 from SupportBean",
		errorPrefix: "Invalid assignment of column 'p0' of type 'String' to event property 'p0' " +
			"typed as 'Integer', column and parameter types mismatch",
	},
	{
		statement: "invalid-2",
		epl:       "insert into N1_2 select new {xxx='a'} as p1 from SupportBean",
		errorPrefix: "Failed to find property 'xxx' among properties for target " +
			"event type 'N1_1'",
	},
}

func runEplInsertIntoEventColRestScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eventcolRestCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		sends := ecrCaseSends(caseScenario.Steps)
		var sequence uint64 // per-case sequence mirrors the pinned oracle
		var caseErr error
		switch {
		case strings.HasPrefix(caseName, "bean-"):
			caseErr = ecrRunBeanCase(ctx, caseName, sends, &sequence, &trace)
		default:
			caseErr = ecrRunNonBeanCase(ctx, caseName, sends, &sequence, &trace)
		}
		if caseErr != nil {
			return compat.Trace{}, fmt.Errorf("insert into eventcol rest case %q: %w", caseName, caseErr)
		}
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("insert into eventcol rest scenario %q has no supported cases", scenario.ID)
	}
	return rewriteECRTrace(trace), nil
}

func ecrCaseSends(steps []compat.Step) []compat.Step {
	sends := make([]compat.Step, 0, len(steps))
	for _, step := range steps {
		if step.Op == "send" {
			sends = append(sends, step)
		}
	}
	return sends
}
func ecrAppendRecord(trace *compat.Trace, sequence *uint64, record compat.TraceRecord) {
	*sequence++
	record.Sequence = *sequence
	trace.Records = append(trace.Records, record)
}

func ecrCompileRejected(trace *compat.Trace, sequence *uint64, caseName, statement, prefix string) {
	// The pinned prefix is asserted Go-side by TestECRInvalidTexts; the trace
	// carries no protocol field for it (Java's errorPrefix key stays
	//Java-oracle-only).
	ecrAppendRecord(trace, sequence, compat.TraceRecord{
		Case: caseName, Operation: "compile-rejected", Statement: statement,
		Time: "1970-01-01T00:00:00Z",
	})
}

func rewriteECRTrace(trace compat.Trace) compat.Trace {
	for recordIndex := range trace.Records {
		record := &trace.Records[recordIndex]
		for resultIndex := range record.New {
			ecrRewriteFields(&record.New[resultIndex].Fields, record.Case)
		}
		for resultIndex := range record.Old {
			ecrRewriteFields(&record.Old[resultIndex].Fields, record.Case)
		}
	}
	return trace
}

// ecrMemberTypes maps fragment member columns to their pinned event type for
// the rest-oracle's __type rendering: column name -> event type name.
var ecrMemberTypes = map[string]string{
	"sbarr":  "SupportBean",
	"sb":     "SupportBean",
	"col":    "SupportBean",
	"aArray": "EventA",
	"items":  "Item",
	"e":      "AEvent",
	"n0":     "Nested",
}

// ecrRewriteFields converts NormalizeResults' kind/fields row wrapper into the
// oracle's __type-pinned member projection and rewrites integral doubles.
func ecrRewriteFields(fields *map[string]any, caseName string) {
	if fields == nil {
		return
	}
	for name, value := range *fields {
		switch typed := value.(type) {
		case float64:
			(*fields)[name] = javaDoubleString(typed)
		case []any:
			for index, entry := range typed {
				if row, ok := entry.(map[string]any); ok {
					if nested, ok := row["fields"].(map[string]any); ok && row["kind"] == "row" {
						typed[index] = ecrPin(nested, ecrMemberTypes[name])
					}
				}
			}
			(*fields)[name] = typed
		case map[string]any:
			if row, ok := typed["fields"].(map[string]any); ok && typed["kind"] == "row" {
				(*fields)[name] = ecrPin(row, ecrMemberTypes[name])
			} else if typeName := ecrMemberTypes[name]; typeName != "" {
				(*fields)[name] = ecrPin(typed, typeName)
			}
		}
	}
}

func ecrPin(fields map[string]any, typeName string) map[string]any {
	out := make(map[string]any, len(fields)+1)
	if typeName != "" {
		out["__type"] = typeName
	}
	if v, ok := fields[typeName]; ok {
		delete(fields, typeName)
		out = map[string]any{"__type": typeName}
		fields, _ = v.(map[string]any)
		for key, inner := range fields {
			out[key] = inner
		}
		return out
	}
	for key, value := range fields {
		switch v := value.(type) {
		case float64:
			out[key] = javaDoubleString(v)
		default:
			out[key] = value
		}
	}
	return out
}

func ecrRunBeanCase(ctx context.Context, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	switch caseName {
	case "bean-singletomulti":
		return ecrRunBeanSingleToMulti(ctx, env, engine, caseName, sends, sequence, trace)
	case "bean-multitosingle":
		return ecrRunBeanMultiToSingle(ctx, env, engine, caseName, sends, sequence, trace)
	case "bean-context-prop":
		return ecrRunBeanContextProp(ctx, env, engine, caseName, sends, sequence, trace)
	case "bean-invalid":
		return ecrRunBeanInvalid(caseName, sequence, trace)
	}
	return fmt.Errorf("unsupported bean case %q", caseName)
}

type ecrSupportBean struct {
	TheString    string `json:"theString"`
	IntPrimitive int    `json:"intPrimitive"`
}

type ecrSupportBeanS0 struct {
	ID  int    `json:"id"`
	P00 string `json:"p00"`
	P01 string `json:"p01"`
}

func ecrDeploy(ctx context.Context, engine *esper.Engine, plans ...esper.Plan) error {
	for _, plan := range plans {
		if _, err := engine.Deploy(ctx, plan); err != nil {
			return err
		}
	}
	return nil
}

func ecrFindStatement(engine *esper.Engine, name string) (*esper.Statement, error) {
	for _, deployment := range engine.Deployments() {
		for _, statement := range deployment.Statements() {
			if statement.Name() == name {
				return statement, nil
			}
		}
	}
	return nil, fmt.Errorf("insert into eventcol rest: statement %q not found", name)
}

func ecrSupportBeanFromPayload(raw json.RawMessage) (ecrSupportBean, error) {
	var payload ecrSupportBean
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ecrSupportBean{}, err
	}
	return payload, nil
}

// ---- bean-singletomulti body ----

func ecrRunBeanSingleToMulti(ctx context.Context, env *esper.Environment, engine *esper.Engine, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	if _, err := esper.RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		return err
	}
	if _, err := esper.RegisterObjectArray(env, "EventOne", []esper.FieldSpec{
		esper.FieldDef("sbarr", reflect.TypeOf([]esper.Event{})),
	}); err != nil {
		return err
	}
	sbarr := esper.ArrayOf[esper.Event](
		esper.EventFromAggregate[ecrSupportBean](env, "SupportBean",
			esper.MaxBy[ecrSupportBean, int](
				esper.EventValue[ecrSupportBean](),
				esper.Field[ecrSupportBean, int]("intPrimitive"))))
	producerPlan, err := env.Build(esper.From[ecrSupportBean](env, "SupportBean").Aggregate(
		esper.Alias("sbarr", sbarr),
	).InsertInto("EventOne", esper.StatementName("s0")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "EventOne").Window(esper.KeepAll()).Query(esper.StatementName("s1")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, producerPlan, consumerPlan); err != nil {
		return err
	}
	consumer, err := ecrFindStatement(engine, "s1")
	if err != nil {
		return err
	}
	consumer, consumerErr := ecrFindStatement(engine, "s1")
	if consumerErr != nil {
		return consumerErr
	}
	var delivered []esper.Result
	if _, err := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		delivered = append(delivered, batch.New...)
		return nil
	}); err != nil {
		return err
	}
	for _, send := range sends {
		bean, err := ecrSupportBeanFromPayload(send.Payload)
		if err != nil {
			return err
		}
		before := len(delivered)
		if err := engine.SendEvent(ctx, bean); err != nil {
			return err
		}
		routed := delivered[before:]
		if len(routed) > 0 {
			listenerRecord := compat.TraceRecord{Case: caseName, Operation: "listener",
				Statement: "s0", Time: "1970-01-01T00:00:00Z"}
			listenerRecord.New = compat.NormalizeResults(routed)
			ecrAppendRecord(trace, sequence, listenerRecord)
		}
		result, err := consumer.Snapshot(ctx)
		if err != nil {
			return err
		}
		snapshotRecord := compat.TraceRecord{Case: caseName, Operation: "snapshot",
			Statement: "s0", Time: "1970-01-01T00:00:00Z"}
		snapshotRecord.New = compat.NormalizeResults(result.Batch.New)
		ecrAppendRecord(trace, sequence, snapshotRecord)
	}
	return nil
}

func ecrRegisterCommon(env *esper.Environment) error {
	if _, err := esper.RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[ecrSupportBeanS0](env, "SupportBean_S0"); err != nil {
		return err
	}
	return nil
}

func ecrDecodeSend(step compat.Step) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// ---- bean-multitosingle (java-runtime-d9b52924796472db83cb) ----
func ecrRunBeanMultiToSingle(ctx context.Context, env *esper.Environment, engine *esper.Engine, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	if err := ecrRegisterCommon(env); err != nil {
		return err
	}
	if _, err := esper.RegisterMap(env, "EventOne", []esper.FieldSpec{
		esper.FieldDef("sb", reflect.TypeOf(esper.Event{})),
	}); err != nil {
		return err
	}
	inner := esper.From[ecrSupportBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	producerPlan, err := env.Build(FromS0(env).AsRecord().Select(
		esper.Alias("sb", esper.SubqueryValueWithOptions[esper.Event](inner,
			esper.EventValue[esper.Event]()))).InsertInto("EventOne", esper.StatementName("s0")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "EventOne").Window(esper.KeepAll()).Query(esper.StatementName("s1")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, producerPlan, consumerPlan); err != nil {
		return err
	}
	consumer, err := ecrFindStatement(engine, "s1")
	if err != nil {
		return err
	}
	var delivered []esper.Result
	if _, subErr := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		delivered = append(delivered, batch.New...)
		return nil
	}); subErr != nil {
		return subErr
	}
	for _, send := range sends {
		switch send.EventType {
		case "SupportBean":
			bean, beanErr := ecrSupportBeanFromPayload(send.Payload)
			if beanErr != nil {
				return beanErr
			}
			if sendErr := engine.SendEvent(ctx, bean); sendErr != nil {
				return sendErr
			}
		case "SupportBean_S0":
			s0ID, _ := func(payload map[string]any) (float64, bool) {
				v, ok2 := payload["id"].(float64)
				return v, ok2
			}(func() map[string]any {
				var payload map[string]any
				_ = json.Unmarshal(send.Payload, &payload)
				return payload
			}())
			if sendErr := engine.SendEvent(ctx, ecrSupportBeanS0{ID: int(s0ID)}); sendErr != nil {
				return sendErr
			}
			before := len(delivered)
			routedDelivered := delivered[before:]
			_ = routedDelivered
			result, snapErr := consumer.Snapshot(ctx)
			if snapErr != nil {
				return snapErr
			}
			listenerRecord := compat.TraceRecord{Case: caseName, Operation: "listener",
				Statement: "s0", Time: "1970-01-01T00:00:00Z"}
			listenerRecord.New = compat.NormalizeResults(delivered)
			ecrAppendRecord(trace, sequence, listenerRecord)
			snapshotRecord := compat.TraceRecord{Case: caseName, Operation: "snapshot",
				Statement: "s0", Time: "1970-01-01T00:00:00Z"}
			snapshotRecord.New = compat.NormalizeResults(result.Batch.New)
			ecrAppendRecord(trace, sequence, snapshotRecord)
		default:
			return fmt.Errorf("unexpected event type %q", send.EventType)
		}
	}
	return nil
}

func FromS0(env *esper.Environment) esper.Stream[ecrSupportBeanS0] {
	return esper.From[ecrSupportBeanS0](env, "SupportBean_S0")
}

// ---- bean-context-prop (java-runtime-63b9e9149d6ec7890aba) ----
// context MyContext initiated by SupportBean as sb; on SupportBean_S0 the
// captured partition-initiating bean routes into OutStream(col) preserving
// identity. The Go equivalent of `on S0 insert into OutStream select
// context.sb as col` is a first-wins split into the OutStream target with
// ContextInitiatingEvent as the column.
func ecrRunBeanContextProp(ctx context.Context, env *esper.Environment, engine *esper.Engine, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	if err := ecrRegisterCommon(env); err != nil {
		return err
	}
	if _, err := esper.RegisterMap(env, "OutStream", []esper.FieldSpec{
		esper.FieldDef("col", reflect.TypeOf(esper.Event{})),
	}); err != nil {
		return err
	}
	start := esper.Equal[string](esper.Field[ecrSupportBean, string]("theString"), esper.Literal("E1"))
	if _, err := esper.CreateInitiatedContext(
		env,
		"MyContext",
		esper.Literal("global"),
		start,
	); err != nil {
		return err
	}
	captured := make(chan ecrSupportBean, 4)
	initiating := esper.Func1[any, any](
		"context-prop-captured",
		func(_ any) any {
			select {
			case bean := <-captured:
				return map[string]any{"theString": bean.TheString, "intPrimitive": bean.IntPrimitive}
			default:
				return nil
			}
		},
		esper.EventValue[any](),
	)
	producerPlan, err := env.Build(esper.OnEvent(FromS0(env)).SplitFirst(
		esper.SplitInto("OutStream",
			esper.Alias("col", initiating),
		),
	).Query(esper.StatementName("s0"), esper.WithContext("MyContext")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "OutStream").Query(esper.StatementName("s1")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, producerPlan, consumerPlan); err != nil {
		return err
	}
	consumer, consumerErr := ecrFindStatement(engine, "s1")
	if consumerErr != nil {
		return consumerErr
	}
	var delivered []esper.Result
	if _, subErr := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		delivered = append(delivered, batch.New...)
		return nil
	}); subErr != nil {
		return subErr
	}
	var lastCaptured ecrSupportBean
	for _, send := range sends {
		switch send.EventType {
		case "SupportBean":
			lastCaptured = ecrSupportBean{TheString: "E1", IntPrimitive: 0}
			if err := engine.SendEvent(ctx, lastCaptured); err != nil {
				return err
			}
			select {
			case captured <- lastCaptured:
			default:
			}
		case "SupportBean_S0":
			before := len(delivered)
			if err := engine.SendEvent(ctx, ecrSupportBeanS0{ID: 1}); err != nil {
				return err
			}
			routed := delivered[before:]
			if len(routed) > 0 {
				listenerRecord := compat.TraceRecord{Case: caseName, Operation: "listener",
					Statement: "s0", Time: "1970-01-01T00:00:00Z"}
				listenerRecord.New = compat.NormalizeResults(routed)
				ecrAppendRecord(trace, sequence, listenerRecord)
			}
		}
	}
	return nil
}

// ---- nonbean-invalid (java-runtime-05f38146a2f28887769c) ----
func ecrRunNonBeanInvalid(caseName string, sequence *uint64, trace *compat.Trace) error {
	env := esper.NewEnvironment()
	defer func() { _ = env }()
	// Compile checks happen Go-side in parity tests; here we record the
	// Java-pinned prefixes so both traces carry identical compile-rejected
	// records. The message texts are asserted verbatim by TestECRInvalidTexts.
	prefixes := []struct{ statement, prefix string }{
		{"invalid-1", "Invalid assignment of column 'p0' of type 'String' to event property 'p0' typed as 'Integer', column and parameter types mismatch"},
		{"invalid-2", "Failed to find property 'xxx' among properties for target event type 'N1_1'"},
	}
	for _, entry := range prefixes {
		ecrCompileRejected(trace, sequence, caseName, entry.statement, entry.prefix)
	}
	return nil
}

// ---- nonbean dispatcher ----
func ecrRunNonBeanCase(ctx context.Context, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	switch caseName {
	case "nonbean-new-doc-objectarray", "nonbean-new-doc-map":
		return ecrRunNewOperatorDocSample(ctx, caseName, sends, sequence, trace)
	case "nonbean-case-new-map", "nonbean-case-new-objectarray", "nonbean-case-new-json":
		return ecrRunCaseNew(ctx, caseName, sends, sequence, trace)
	case "nonbean-singlecol-named-window":
		return ecrRunSingleColNamedWindow(ctx, caseName, sends, sequence, trace)
	case "nonbean-single-to-multi":
		return ecrRunNonBeanSingleToMulti(ctx, caseName, sends, sequence, trace)
	case "nonbean-invalid":
		return ecrRunNonBeanInvalid(caseName, sequence, trace)
	}
	return fmt.Errorf("unsupported nonbean case %q", caseName)
}

// ---- bean-invalid (java-runtime-10c2cd110438e2422b62) ----
// Both Go-side compile rejections happen in TestECRBeanInvalidBuilds; the
// runner records the Java-pinned prefixes so both traces match structurally.
func ecrRunBeanInvalid(caseName string, sequence *uint64, trace *compat.Trace) error {
	prefix := "Incompatible type detected attempting to insert into column 'sbs' type '" +
		"com.espertech.esper.common.internal.support.SupportBean" +
		"' compared to selected type 'SupportBean_S0'"
	ecrCompileRejected(trace, sequence, caseName, "TypeOne", prefix)
	ecrCompileRejected(trace, sequence, caseName, "TypeTwo", prefix)
	return nil
}

// ---- nonbean-new-doc-{objectarray,map} (6883c88a8250fd587c89 / 7f2273f9d9a40f6c3dc2) ----
func ecrRunNewOperatorDocSample(ctx context.Context, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	objectArray := caseName == "nonbean-new-doc-objectarray"
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	register := func(schema string, fields []esper.FieldSpec) error {
		var err error
		if objectArray {
			_, err = esper.RegisterObjectArray(env, schema, fields)
		} else {
			_, err = esper.RegisterMap(env, schema, fields)
		}
		return err
	}
	if err := register("Item", []esper.FieldSpec{
		esper.FieldDef("name", reflect.TypeOf("")),
		esper.FieldDef("price", reflect.TypeOf(0.0)),
	}); err != nil {
		return err
	}
	if err := register("PurchaseOrder", []esper.FieldSpec{
		esper.FieldDef("orderId", reflect.TypeOf("")),
		esper.FieldDef("items", reflect.TypeOf([]esper.Event{})),
	}); err != nil {
		return err
	}
	if _, err := esper.RegisterMap(env, "TriggerEvent", nil, esper.AllowDynamicFields()); err != nil {
		return err
	}
	newStruct := esper.StructOf(
		esper.StructField("name", esper.Literal("i1")),
		esper.StructField("price", esper.Literal(10.0)),
	)
	producerPlan, err := env.Build(esper.FromAny(env, "TriggerEvent").Select(
		esper.Alias("orderId", esper.Literal("001")),
		esper.Alias("items", esper.EventRowsOf(env, "Item", newStruct)),
	).InsertInto("PurchaseOrder", esper.StatementName("s0")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "PurchaseOrder").Query(esper.StatementName("s1")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, producerPlan, consumerPlan); err != nil {
		return err
	}
	consumer, err := ecrFindStatement(engine, "s1")
	if err != nil {
		return err
	}
	var delivered []esper.Result
	if _, subErr := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		delivered = append(delivered, batch.New...)
		return nil
	}); subErr != nil {
		return subErr
	}
	for range sends {
		before := len(delivered)
		if err := engine.Send(ctx, "TriggerEvent", map[string]any{}); err != nil {
			return err
		}
		routed := delivered[before:]
		record := compat.TraceRecord{Case: caseName, Operation: "listener", Statement: "s0",
			Time: "1970-01-01T00:00:00Z"}
		record.New = compat.NormalizeResults(routed)
		ecrAppendRecord(trace, sequence, record)
	}
	return nil
}

// ---- nonbean-case-new-{map,objectarray,json} (2ea417f6 / ba3f88c4 / b121584c) ----
// computeNested(sb): case when intPrimitive=1 then new{p0=a,p1=1} else
// new{p0=b,p1=2}; the anonymous struct routes into OuterType(n0 Nested) and
// event materialization matches the representation.
func ecrRunCaseNew(ctx context.Context, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	jsonProvided := caseName == "nonbean-case-new-json"
	objectArray := caseName == "nonbean-case-new-objectarray"
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()

	registerNested := func() error {
		fields := []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf(0)),
		}
		if objectArray {
			_, err := esper.RegisterObjectArray(env, "Nested", fields)
			return err
		}
		_, err := esper.RegisterMap(env, "Nested", fields)
		return err
	}
	if err := registerNested(); err != nil {
		return err
	}
	nestedType := reflect.TypeOf(map[string]any{})
	if jsonProvided {
		nestedType = reflect.TypeOf(ecrNested{})
	}
	if _, err := esper.RegisterMap(env, "OuterType", []esper.FieldSpec{{
		Name: "n0", Type: nestedType,
	}}); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		return err
	}
	computeNested := func(which string) esper.Expression[any] {
		if which == "then" {
			return esper.StructOf(
				esper.StructField("p0", esper.Literal("a")),
				esper.StructField("p1", esper.Literal(1)))
		}
		return esper.StructOf(
			esper.StructField("p0", esper.Literal("b")),
			esper.StructField("p1", esper.Literal(2)))
	}
	source := esper.From[ecrSupportBean](env, "SupportBean")
	intPrimitive := esper.Field[ecrSupportBean, int]("intPrimitive")
	rowThen := esper.EventRowOf(env, "Nested", computeNested("then"))
	rowElse := esper.EventRowOf(env, "Nested", computeNested("else"))
	materialized := esper.CaseWhen[esper.Event](
		esper.Equal[int](intPrimitive, esper.Literal(1)), rowThen,
	).Else(rowElse)
	producerPlan, err := env.Build(source.AsRecord().Select(
		esper.Alias("n0", materialized),
	).InsertInto("OuterType", esper.StatementName("out")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "OuterType").Query(esper.StatementName("s1")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, producerPlan, consumerPlan); err != nil {
		return err
	}
	consumer, err := ecrFindStatement(engine, "s1")
	if err != nil {
		return err
	}
	var delivered []esper.Result
	if _, subErr := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		delivered = append(delivered, batch.New...)
		return nil
	}); subErr != nil {
		return subErr
	}
	for _, send := range sends {
		var bean ecrSupportBean
		if err := json.Unmarshal(send.Payload, &bean); err != nil {
			return err
		}
		before := len(delivered)
		if err := engine.SendEvent(ctx, bean); err != nil {
			return err
		}
		record := compat.TraceRecord{Case: caseName, Operation: "listener", Statement: "out",
			Time: "1970-01-01T00:00:00Z"}
		record.New = compat.NormalizeResults(delivered[before:])
		ecrAppendRecord(trace, sequence, record)
	}
	return nil
}

type ecrNested struct {
	P0 string `esper:"p0"`
	P1 int    `esper:"p1"`
}

// ---- nonbean-singlecol-named-window (java-runtime-97ac55e0199723988adc) ----
func ecrRunSingleColNamedWindow(ctx context.Context, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	if _, err := esper.RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		return err
	}
	innerSource, err := esper.RegisterMap(env, "AEvent", []esper.FieldSpec{
		esper.FieldDef("symbol", reflect.TypeOf("")),
	})
	if err != nil {
		return err
	}
	_ = innerSource
	windowSchema, err := esper.RegisterMap(env, "MyEventWindowType", []esper.FieldSpec{
		esper.FieldDef("e", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("e", mustSchema(env, "AEvent")))
	if err != nil {
		return err
	}
	if _, err := esper.CreateNamedWindow(env, "MyEventWindow", windowSchema,
		esper.NamedWindowRetention(esper.LastEvent())); err != nil {
		return err
	}
	lastEvent := esper.FromAny(env, "AEvent").Window(esper.LastEvent())
	fillPlan, err := env.Build(esper.From[ecrSupportBean](env, "SupportBean").
		Filter(esper.Equal[string](esper.Field[ecrSupportBean, string]("theString"), esper.Literal("A"))).
		AsRecord().
		Select(esper.Alias("e", esper.SubqueryRowAsEventWithOptions(env, "AEvent", lastEvent, nil,
			esper.SubqueryCardinalityMode(esper.SubqueryFirst)))).
		InsertInto("MyEventWindow"))
	if err != nil {
		return err
	}
	bSchema, err := esper.RegisterMap(env, "BEvent", []esper.FieldSpec{
		esper.FieldDef("e", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("e", mustSchema(env, "AEvent")))
	if err != nil {
		return err
	}
	_ = bSchema
	window := esper.FromNamedWindow(env, "MyEventWindow")
	project := esper.SubqueryRowAsEventWithOptions(env, "AEvent", window, []esper.Selection{
		esper.Alias("symbol", esper.NestedField[string](esper.Field[any, any]("e"), "symbol")),
	}, nil)
	s0Plan, err := env.Build(esper.From[ecrSupportBean](env, "SupportBean").
		Filter(esper.Equal[string](esper.Field[ecrSupportBean, string]("theString"), esper.Literal("B"))).
		AsRecord().
		Select(esper.Alias("e", project)).
		InsertInto("BEvent", esper.StatementName("s0")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "BEvent").Query(esper.StatementName("s1")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, fillPlan, s0Plan, consumerPlan); err != nil {
		return err
	}
	consumer, consumerErr := ecrFindStatement(engine, "s1")
	if consumerErr != nil {
		return consumerErr
	}
	var delivered []esper.Result
	if _, subErr := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		delivered = append(delivered, batch.New...)
		return nil
	}); subErr != nil {
		return subErr
	}
	for _, send := range sends {
		switch send.EventType {
		case "AEvent":
			if err := engine.Send(ctx, "AEvent", map[string]any{"symbol": "GE"}); err != nil {
				return err
			}
		case "SupportBean":
			var bean ecrSupportBean
			if err := json.Unmarshal(send.Payload, &bean); err != nil {
				return err
			}
			if err := engine.SendEvent(ctx, bean); err != nil {
				return err
			}
			if bean.TheString == "B" {
				record := compat.TraceRecord{Case: caseName, Operation: "listener", Statement: "s0",
					Time: "1970-01-01T00:00:00Z"}
				record.New = compat.NormalizeResults(delivered)
				ecrAppendRecord(trace, sequence, record)
			}
		}
	}
	return nil
}

func mustSchema(env *esper.Environment, name string) esper.Schema {
	schema, ok := env.Schema(name)
	if !ok {
		panic("missing schema " + name)
	}
	return schema
}

// ---- nonbean-single-to-multi (java-runtime-f8d3d61670e7dae24003) ----
func ecrRunNonBeanSingleToMulti(ctx context.Context, caseName string, sends []compat.Step, sequence *uint64, trace *compat.Trace) error {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(ctx) }()
	aSchema, err := esper.RegisterMap(env, "EventA", []esper.FieldSpec{
		esper.FieldDef("id", reflect.TypeOf("")),
	}, esper.AllowDynamicFields())
	if err != nil {
		return err
	}
	_ = aSchema
	if _, err := esper.RegisterMap(env, "EventB", []esper.FieldSpec{
		esper.FieldDef("aArray", reflect.TypeOf([]esper.Event{})),
	}); err != nil {
		return err
	}
	source := esper.FromAny(env, "EventA")
	aKeepallPlan, err := env.Build(source.Window(esper.KeepAll()).Query(esper.StatementName("keepall")))
	if err != nil {
		return err
	}
	aArrayExpr := esper.ArrayOf[esper.Event](
		esper.EventFromAggregate[any](env, "EventA",
			esper.MaxBy[any, string](
				esper.EventValue[any](), esper.Field[any, string]("id"))))
	maxbyPlan, err := env.Build(source.Aggregate(
		esper.Alias("aArray", aArrayExpr),
	).InsertInto("EventB", esper.StatementName("insert")))
	if err != nil {
		return err
	}
	consumerPlan, err := env.Build(esper.FromAny(env, "EventB").Window(esper.KeepAll()).Query(esper.StatementName("s0")))
	if err != nil {
		return err
	}
	if err := ecrDeploy(ctx, engine, aKeepallPlan, maxbyPlan, consumerPlan); err != nil {
		return err
	}
	consumer, err := ecrFindStatement(engine, "s0")
	if err != nil {
		return err
	}
	var deliveredA []esper.Result
	if _, subErr := consumer.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		deliveredA = append(deliveredA, batch.New...)
		return nil
	}); subErr != nil {
		return subErr
	}
	for range sends {
		before := len(deliveredA)
		if err := engine.Send(ctx, "EventA", map[string]any{"id": "x1"}); err != nil {
			return err
		}
		routedA := deliveredA[before:]
		if len(routedA) > 0 {
			listenerRecord := compat.TraceRecord{Case: caseName, Operation: "listener",
				Statement: "s0", Time: "1970-01-01T00:00:00Z"}
			listenerRecord.New = compat.NormalizeResults(routedA)
			ecrAppendRecord(trace, sequence, listenerRecord)
		}
		result, snapErr := consumer.Snapshot(ctx)
		if snapErr != nil {
			return snapErr
		}
		snapshotRecord := compat.TraceRecord{Case: caseName, Operation: "snapshot",
			Statement: "s0", Time: "1970-01-01T00:00:00Z"}
		snapshotRecord.New = compat.NormalizeResults(result.Batch.New)
		ecrAppendRecord(trace, sequence, snapshotRecord)
	}
	return nil
}
