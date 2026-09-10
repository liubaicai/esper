package parity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the dataflow exception-propagation chain:
//   - EPLDataflowAPIExceptions (java-runtime-c1d106a7e1cc70655006, the whole
//     class is one direct execution): two synchronous-throw flows —
//     a failing custom source (Java wraps as 'Support-graph-source generated
//     exception: ...'; the handler sees one context and the instance
//     completes) and a failing operator process (Java's handler receives the
//     raw unwrapped throwable and swallows it, so submit returns normally and
//     exactly one context exists). Both flows end COMPLETE — never CANCELLED.
//
// Record protocol: count/value/state records only (no data rows), six per
// flow: handler-contexts count, error-class token, operator name/number,
// pretty-print, and the COMPLETE state. Exception message text is asserted
// in-process by the oracle and pinned Go-side by unit tests — never recorded
// (invalidity policy). The operator pretty-print is recorded in the
// normalized bare-port form: Java asserts its full '<SupportBean>' string
// in-process while Go renders typed ports package-qualified.
// Error-policy semantics note: Go's Fail policy surfaces the error from
// Run/Join while Java's start-and-swallow completes silently — the state
// (COMPLETE) and handler invocation are the shared observables.
const dataflowExceptionsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var dataflowExceptionsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIExceptions.java",
}

var dataflowExceptionsJavaRuntimeIDs = []string{
	"java-runtime-c1d106a7e1cc70655006",
}

var dataflowExceptionsJavaExecutions = []string{
	"EPLDataflowAPIExceptions",
}

var dataflowExceptionsCases = []string{
	"exceptions",
}

// dataflowExceptionsBean mirrors the SupportBean event type.
type dataflowExceptionsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// dataflowExceptionsHandler records every exception context.
type dataflowExceptionsHandler struct {
	mu   sync.Mutex
	errs []esper.DataflowError
}

func (h *dataflowExceptionsHandler) handle(_ context.Context, err esper.DataflowError) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errs = append(h.errs, err)
	return nil
}

func (h *dataflowExceptionsHandler) snapshot() []esper.DataflowError {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]esper.DataflowError(nil), h.errs...)
}

// normalizeDataflowPrettyPrint strips the type label from a pretty-printed
// port ('outstream<pkg.Type>' -> 'outstream') and Go's implicit default
// output suffix (' -> out') so both sides record the bare-port form.
func normalizeDataflowPrettyPrint(prettyPrint string) string {
	if strings.HasSuffix(prettyPrint, " -> out") {
		prettyPrint = strings.TrimSuffix(prettyPrint, " -> out")
	}
	open := strings.Index(prettyPrint, "<")
	if open < 0 {
		return prettyPrint
	}
	close := strings.LastIndex(prettyPrint, ">")
	if close < 0 || close < open {
		return prettyPrint
	}
	return prettyPrint[:open] + prettyPrint[close+1:]
}

func runDataflowExceptionsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("dataflow-exceptions scenario has no steps")
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for caseIndex, caseName := range dataflowExceptionsCases {
		caseTrace, err := runDataflowExceptionsCase(ctx, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("dataflow-exceptions case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runDataflowExceptionsCase(ctx context.Context, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := uint64(0)
	var records []compat.TraceRecord
	record := func(operation, statement, name string, value any) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      name,
			Value:     value,
		})
	}
	countRecord := func(operation, statement, name string, count int64) {
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      caseName,
			Operation: operation,
			Statement: statement,
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(now),
			Name:      name,
			Count:     &count,
		})
	}
	// emitFlow pins the six-record handler/state shape for one flow.
	emitFlow := func(handler *dataflowExceptionsHandler, instance *esper.DataflowInstance,
		errorClass, operatorName string, operatorNumber int) error {
		contexts := handler.snapshot()
		if len(contexts) != 1 {
			return fmt.Errorf("handler contexts = %d, want 1", len(contexts))
		}
		ctx := contexts[0]
		if ctx.OperatorName != operatorName || ctx.OperatorNum != operatorNumber {
			return fmt.Errorf("handler context operator = %q#%d, want %q#%d", ctx.OperatorName, ctx.OperatorNum, operatorName, operatorNumber)
		}
		if instance.State() != esper.DataflowComplete {
			return fmt.Errorf("instance state = %v, want complete", instance.State())
		}
		countRecord("count", "flow", "handler-contexts", int64(len(contexts)))
		record("lifecycle", "flow", "handler.error-class", errorClass)
		record("lifecycle", "flow", "handler.operator-name", ctx.OperatorName)
		record("lifecycle", "flow", "handler.operator-number", ctx.OperatorNum)
		record("lifecycle", "flow", "handler.operator-pretty-print", normalizeDataflowPrettyPrint(ctx.OperatorPrettyPrint))
		record("state", "flow", "instance.state", "COMPLETE")
		return nil
	}

	// Flow A — the source itself throws.
	subEnv := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowExceptionsBean](subEnv, "SupportBean"); err != nil {
		return nil, err
	}
	handler := &dataflowExceptionsHandler{}
	sourceFactory := func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
		return dataflowExceptionsFailingSource{}, nil
	}
	definition, buildErr := esper.DefineDataflow(subEnv, "MyDataFlow").
		CustomTypedSource("DefaultSupportSourceOp", sourceFactory,
			[]esper.DataflowPort{esper.DataflowPortOf[dataflowExceptionsBean]("outstream")}).
		Build()
	if buildErr != nil {
		return nil, buildErr
	}
	subEngine := esper.NewEngine(subEnv,
		esper.WithRuntimeURI(dataflowExceptionsJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	instance, err := subEngine.InstantiateDataflowWithOptions(ctx, definition, esper.DataflowOptions{
		ExceptionHandler: handler.handle,
	})
	if err != nil {
		_ = subEngine.Close(context.Background())
		return nil, err
	}
	// Java's start-and-swallow completes the instance; Go surfaces the same
	// failure from Join under the Fail policy while the state and the single
	// handler context remain the shared observables.
	runErr := instance.Run(ctx)
	var dataflowErr esper.DataflowError
	if runErr == nil || !errors.As(runErr, &dataflowErr) {
		_ = subEngine.Close(context.Background())
		return nil, fmt.Errorf("flow A run error = %v, want a dataflow error", runErr)
	}
	if err := emitFlow(handler, instance, "source-error", "DefaultSupportSourceOp", 0); err != nil {
		_ = subEngine.Close(context.Background())
		return nil, err
	}
	if err := subEngine.Close(context.Background()); err != nil {
		return nil, err
	}

	// Flow B — the operator throws after one event passes through.
	subEnv = esper.NewEnvironment()
	if _, err := esper.RegisterStruct[dataflowExceptionsBean](subEnv, "SupportBean"); err != nil {
		return nil, err
	}
	handler = &dataflowExceptionsHandler{}
	sourceFactory = func(esper.DataflowOperatorContext) (esper.DataflowSourceRuntime, error) {
		return dataflowExceptionsOneBeanSource{}, nil
	}
	failFactory := func(esper.DataflowOperatorContext) (esper.DataflowOperatorRuntime, error) {
		return dataflowExceptionsFailingOperator{}, nil
	}
	definition, buildErr = esper.DefineDataflow(subEnv, "MyDataFlow").
		CustomTypedSource("DefaultSupportSourceOp", sourceFactory,
			[]esper.DataflowPort{esper.DataflowPortOf[dataflowExceptionsBean]("outstream")}).
		CustomPorts("MyExceptionOp", failFactory, []string{"outstream"}, nil).
		ConnectPorts("DefaultSupportSourceOp", "outstream", "MyExceptionOp", "outstream").
		Build()
	if buildErr != nil {
		return nil, buildErr
	}
	subEngine = esper.NewEngine(subEnv,
		esper.WithRuntimeURI(dataflowExceptionsJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(now),
	)
	instance, err = subEngine.InstantiateDataflowWithOptions(ctx, definition, esper.DataflowOptions{
		ExceptionHandler: handler.handle,
	})
	if err != nil {
		_ = subEngine.Close(context.Background())
		return nil, err
	}
	runErr = instance.Run(ctx)
	if runErr == nil || !errors.As(runErr, &dataflowErr) {
		_ = subEngine.Close(context.Background())
		return nil, fmt.Errorf("flow B run error = %v, want a dataflow error", runErr)
	}
	if err := emitFlow(handler, instance, "operator-throw", "MyExceptionOp", 1); err != nil {
		_ = subEngine.Close(context.Background())
		return nil, err
	}
	if err := subEngine.Close(context.Background()); err != nil {
		return nil, err
	}
	return records, nil
}

// dataflowExceptionsFailingSource mirrors the throwing DefaultSupportSourceOp
// instruction (the message text is asserted in-process, never recorded).
type dataflowExceptionsFailingSource struct{}

func (s dataflowExceptionsFailingSource) Run(_ context.Context, _ *esper.DataflowEmitter) error {
	return errors.New("My-Exception-Is-Here")
}

// dataflowExceptionsOneBeanSource emits a single event, then finishes.
type dataflowExceptionsOneBeanSource struct{}

func (s dataflowExceptionsOneBeanSource) Run(ctx context.Context, emitter *esper.DataflowEmitter) error {
	return emitter.SubmitPort(ctx, "outstream", dataflowExceptionsBean{TheString: "E1", IntPrimitive: 1})
}

// dataflowExceptionsFailingOperator mirrors the throwing MyExceptionOp
// onInput (raw 'Operator-thrown-exception').
type dataflowExceptionsFailingOperator struct{}

func (o dataflowExceptionsFailingOperator) Process(_ context.Context, _ esper.DataflowInput) ([]esper.DataflowEmission, error) {
	return nil, errors.New("Operator-thrown-exception")
}
