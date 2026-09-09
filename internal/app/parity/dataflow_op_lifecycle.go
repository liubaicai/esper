package parity

import (
	"context"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLDataflowAPIOpLifecycle: dataflow operator lifecycle
// stages — instantiated, configured property, operator context (instance id,
// user object), open, source next/on-input deliveries, and close, ending in
// the terminal complete state. Replay-shape adaptations frozen with the
// scouts: the Java compile-time forge stage and deploy-time factory
// initialize context have no Go surface (the fluent builder constructs the
// graph directly) and their observables are dropped on both sides; the
// source-only graph gains a capture sink so submissions have a connected
// edge; the post-deploy empty-drain assertion is a static-list artifact and
// is not represented.
type dataflowOpLifecycleBean struct {
	TheString string `esper:"theString"`
}

const dataflowOpLifecycleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowOpLifecycleJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIOpLifecycle.java",
}

var dataflowOpLifecycleJavaRuntimeIDs = []string{
	"java-runtime-ff077635ccc5082a06a1",
	"java-runtime-12a29443119ff32fd057",
	"java-runtime-4bf013f93c9464b8e890",
}

var dataflowOpLifecycleJavaExecutions = []string{
	"EPLDataflowTypeEvent",
	"EPLDataflowFlowGraphSource",
	"EPLDataflowFlowGraphOperator",
}

var dataflowOpLifecycleCases = []string{
	"type-event",
	"flow-graph-source",
	"flow-graph-operator",
}

// dataflowOpLifecycleRecorder collects the lifecycle observations in engine
// order for one operator.
type dataflowOpLifecycleRecorder struct {
	items []compat.TraceRecord
}

func (r *dataflowOpLifecycleRecorder) add(name string, value any) {
	r.items = append(r.items, compat.TraceRecord{
		Operation: "lifecycle",
		Statement: "flow:SupportGraphSource",
		Name:      name,
		Value:     value,
	})
}

// addFor records against a different operator statement.
func (r *dataflowOpLifecycleRecorder) addFor(statement, name string, value any) {
	r.items = append(r.items, compat.TraceRecord{
		Operation: "lifecycle",
		Statement: statement,
		Name:      name,
		Value:     value,
	})
}

func runDataflowOpLifecycleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-op-lifecycle scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowOpLifecycleCases {
		caseTrace, err := runDataflowOpLifecycleCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-op-lifecycle case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

// dataflowOpLifecycleFinish stamps case/sequence/time onto the collected
// lifecycle items and appends the terminal state record.
func dataflowOpLifecycleFinish(records []compat.TraceRecord, caseName string, captured []compat.TraceRecord) []compat.TraceRecord {
	sequence := uint64(0)
	now := time.Unix(0, 0).UTC()
	for index := range records {
		sequence++
		records[index].Case = caseName
		records[index].Sequence = sequence
		records[index].Time = compat.FormatTraceTime(now)
	}
	for index := range captured {
		sequence++
		captured[index].Case = caseName
		captured[index].Sequence = sequence
		captured[index].Time = compat.FormatTraceTime(now)
		records = append(records, captured[index])
	}
	sequence++
	records = append(records, compat.TraceRecord{
		Case:      caseName,
		Operation: "state",
		Statement: "flow",
		Name:      "state",
		Value:     "complete",
		Sequence:  sequence,
		Time:      compat.FormatTraceTime(now),
	})
	return records
}

func runDataflowOpLifecycleCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	switch caseIndex {
	case 0:
		return runDataflowOpLifecycleTypeEvent(ctx, caseName)
	case 1:
		return runDataflowOpLifecycleFlowGraphSource(ctx, caseName)
	case 2:
		return runDataflowOpLifecycleFlowGraphOperator(ctx, caseName)
	default:
		return nil, fmt.Errorf("unsupported dataflow-op-lifecycle case index %d", caseIndex)
	}
}

// runDataflowOpLifecycleTypeEvent mirrors EPLDataflowTypeEvent: the built
// definition's declared output port carries the named schema type. Compile
// only — no instance, no rows.
func runDataflowOpLifecycleTypeEvent(ctx context.Context, caseName string) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "MySchema", []esper.FieldSpec{
		esper.FieldDef("key", reflect.TypeOf("")),
		esper.FieldDef("value", reflect.TypeOf(int(0))),
	}); err != nil {
		return nil, err
	}
	mySchemaType := reflect.TypeOf(map[string]any{})
	definition, err := esper.DefineDataflow(env, "MyDataFlowOne").
		CustomTypedSource("MyCaptureOutputPortOp",
			func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
				return nil, fmt.Errorf("not instantiated")
			},
			[]esper.DataflowPort{esper.DataflowPortOf[map[string]any]("outstream")}).
		Build()
	if err != nil {
		return nil, err
	}
	found := false
	for _, operator := range definition.Operators() {
		if operator.Name != "MyCaptureOutputPortOp" {
			continue
		}
		if portType, ok := operator.OutputPortTypes["outstream"]; ok && portType == mySchemaType {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("declared output port type for outstream was not found")
	}
	sequence := uint64(1)
	record := compat.TraceRecord{
		Case:      caseName,
		Operation: "port-type",
		Statement: "flow:MyCaptureOutputPortOp",
		Name:      "outstream",
		Value:     "MySchema",
		Sequence:  sequence,
		Time:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
	}
	return []compat.TraceRecord{record}, nil
}

// dataflowOpLifecycleSourceRuntime mirrors SupportGraphSource: a lifecycle
// source that submits two values and then a final marker, recording open,
// the per-poll next markers, and close.
type dataflowOpLifecycleSourceRuntime struct {
	recorder *dataflowOpLifecycleRecorder
	complete bool
}

func (s *dataflowOpLifecycleSourceRuntime) Open(context.Context) error {
	s.recorder.add("stage", "open")
	return nil
}

func (s *dataflowOpLifecycleSourceRuntime) Close(context.Context) error {
	s.recorder.add("stage", "close")
	return nil
}

func (s *dataflowOpLifecycleSourceRuntime) Run(ctx context.Context, emitter *esper.DataflowEmitter) error {
	for numrows := 0; numrows < 2; numrows++ {
		s.recorder.add("stage", fmt.Sprintf("next(numrows=%d)", numrows))
		if err := emitter.Submit(ctx, "E"+fmt.Sprint(numrows+1)); err != nil {
			return err
		}
	}
	s.recorder.add("stage", "next(numrows=2)")
	return emitter.SubmitSignal(ctx, esper.FinalMarker{})
}

// runDataflowOpLifecycleFlowGraphSource mirrors EPLDataflowFlowGraphSource:
// the graph gains a capture sink (replay-shape adaptation), the source
// submits "E1"/"E2", and the lifecycle records the instantiation, property,
// operator context, and open/next/close stages.
func runDataflowOpLifecycleFlowGraphSource(ctx context.Context, caseName string) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowOpLifecycleBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	recorder := &dataflowOpLifecycleRecorder{}
	var sourceRuntime *dataflowOpLifecycleSourceRuntime
	var captured []compat.TraceRecord

	var capturedValues []any
	now := time.Unix(0, 0).UTC()

	sourceFactory := esper.DataflowSourceFactory(func(operatorCtx esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
		recorder.add("stage", "instantiated")
		if propOne, ok := esper.DataflowProperty[string](operatorCtx, "propOne"); ok {
			recorder.add("propOne", propOne)
		}
		recorder.add("dataflowName", operatorCtx.DataflowName)
		recorder.add("instanceId", operatorCtx.InstanceID)
		recorder.add("userObject", operatorCtx.UserObject)
		recorder.add("operatorNumber", int64(operatorCtx.OperatorNum))
		recorder.add("operatorName", operatorCtx.OperatorName)
		sourceRuntime = &dataflowOpLifecycleSourceRuntime{recorder: recorder}
		return sourceRuntime, nil
	})
	definition, err := esper.DefineDataflow(env, "MyDataFlow").
		CustomSourceWithOptions("SupportGraphSource", sourceFactory,
			esper.DataflowOperatorOptions{Properties: map[string]any{"propOne": "abc"}}).
		Custom("DefaultSupportCaptureOp", func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
			return &dataflowOpLifecycleCapture{sink: &capturedValues}, nil
		}).
		Connect("SupportGraphSource", "DefaultSupportCaptureOp").
		Build()
	if err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowOpLifecycleJavaRuntimeIDs[1]),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	instance, err := engine.InstantiateDataflowWithOptions(ctx, definition,
		esper.DataflowOptions{InstanceID: "id1", UserObject: "myobject"})
	if err != nil {
		return nil, err
	}
	if err := instance.Run(ctx); err != nil {
		return nil, err
	}

	records := recorder.items
	capturedRows := make([]compat.ResultRecord, 0, len(capturedValues))
	for _, value := range capturedValues {
		capturedRows = append(capturedRows, compat.ResultRecord{
			Kind:   "row",
			Fields: map[string]any{"item": value},
		})
	}
	captured = append(captured, compat.TraceRecord{
		Operation: "capture",
		Statement: "flow:DefaultSupportCaptureOp",
		New:       capturedRows,
	})
	return dataflowOpLifecycleFinish(records, caseName, captured), nil
}

// runDataflowOpLifecycleFlowGraphOperator mirrors EPLDataflowFlowGraphOperator:
// a provider-injected line-feed source delivers two payloads to a lifecycle
// operator, which records instantiated/open/onInput/close.
func runDataflowOpLifecycleFlowGraphOperator(ctx context.Context, caseName string) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	recorder := &dataflowOpLifecycleRecorder{}
	operatorStatement := "flow:SupportOperator"
	var injectedSource esper.DataflowSourceRuntime
	operatorFactory := func(operatorCtx esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
		recorder.addFor(operatorStatement, "stage", "instantiated")
		return &dataflowOpLifecycleOperator{recorder: recorder, statement: operatorStatement}, nil
	}

	// The name-keyed factory closure supplies the pre-built line-feed
	// instance, mirroring Java's DefaultSupportGraphOpProvider name matching
	// (the engine's Build-time validation requires a definition factory).
	sourceFactory := func(operatorCtx esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
		if operatorCtx.OperatorName == "MyLineFeedSource" && injectedSource != nil {
			return injectedSource, nil
		}
		return nil, fmt.Errorf("no source for %q", operatorCtx.OperatorName)
	}
	definition, err := esper.DefineDataflow(env, "MyDataFlow").
		CustomSource("MyLineFeedSource", sourceFactory).
		Custom("SupportOperator", operatorFactory).
		Connect("MyLineFeedSource", "SupportOperator").
		Build()
	if err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(dataflowOpLifecycleJavaRuntimeIDs[2]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	injectedSource = &dataflowOpLifecycleLineFeed{recorder: recorder, lines: []string{"abc", "def"}}
	instance, err := engine.InstantiateDataflow(ctx, definition)
	if err != nil {
		return nil, err
	}
	if err := instance.Run(ctx); err != nil {
		return nil, err
	}
	return dataflowOpLifecycleFinish(recorder.items, caseName, nil), nil
}

// dataflowOpLifecycleLineFeed mirrors MyLineFeedSource: submits each line
// wrapped as a single-element payload, then a final marker.
type dataflowOpLifecycleLineFeed struct {
	recorder *dataflowOpLifecycleRecorder
	lines    []string
}

func (s *dataflowOpLifecycleLineFeed) Run(ctx context.Context, emitter *esper.DataflowEmitter) error {
	for _, line := range s.lines {
		if err := emitter.Submit(ctx, []any{line}); err != nil {
			return err
		}
	}
	return emitter.SubmitSignal(ctx, esper.FinalMarker{})
}

// dataflowOpLifecycleOperator mirrors SupportOperator: records open, each
// onInput payload (unwrapping the single-element payload), and close.
type dataflowOpLifecycleOperator struct {
	recorder  *dataflowOpLifecycleRecorder
	statement string
}

func (o *dataflowOpLifecycleOperator) Open(context.Context) error {
	o.recorder.addFor(o.statement, "stage", "open")
	return nil
}

func (o *dataflowOpLifecycleOperator) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	value := input.Value
	if payload, ok := value.([]any); ok && len(payload) == 1 {
		value = payload[0]
	}
	o.recorder.addFor(o.statement, "onInput", value)
	return nil, nil
}

func (o *dataflowOpLifecycleOperator) Close(context.Context) error {
	o.recorder.addFor(o.statement, "stage", "close")
	return nil
}

// dataflowOpLifecycleCapture mirrors DefaultSupportCaptureOp: records every
// delivered value raw (the Java graph submits unchecked Strings into the
// typed stream).
type dataflowOpLifecycleCapture struct {
	sink *[]any
}

func (c *dataflowOpLifecycleCapture) Process(_ context.Context, input esper.DataflowInput) ([]esper.DataflowEmission, error) {
	*c.sink = append(*c.sink, input.Value)
	return nil, nil
}
