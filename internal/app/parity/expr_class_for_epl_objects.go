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

// expr_class_for_epl_objects.go replays the four ExprClassForEPLObjects
// executions (ordinals 0-3) against the pinned Java oracle as one
// replayable differential chain covering the visibility boundaries of
// inlined_class classes:
//
//   - from-clause-method (ord 0, ExprClassResolutionFromClauseMethod): a
//     path-deployed @public create inlined_class MyFromClauseMethod whose
//     static getBeans() feeds the method: from-clause join; one
//     SupportBean("E1",10) yields a single listener invocation carrying
//     the ordered rows c0=1 then c0=2 (assertPropsPerRowLastNew).
//   - output-col-type (ord 1, ExprClassResolutionOutputColType): the
//     statement-local inlined MyBean's static getBean(intPrimitive) is
//     selected as c0; the Java execution asserts the c0 property type is
//     the inlined class itself and reads the instance reflectively
//     through getId(), so both traces render c0=10.
//   - invalid (ord 2, ExprClassResolutionInvalid): five tryInvalidCompile
//     probes pin the Java message prefixes — inlined classes are
//     invisible to annotation classes, create-schema bean types, nestable
//     schema/window property types and table column types.
//   - script (ord 3, ExprClassResolutionScript): a js: expression whose
//     Java.type("MyScriptResult") lookup compiles and deploys but fails
//     at runtime because the Nashorn classloader cannot see inlined
//     classes; Java's sendEventBean raises EPException "java.lang.
//     RuntimeException: Unexpected exception in statement 's0'".
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle compiles the pinned EPL verbatim, including the
//     inlined_class triple-quote class text, the duplicated
//     @name('s0')@name('s0') annotation and the single-line concatenation
//     quirks. Go has no inlined_class directive, so the EPL text is never
//     replayed: getBeans() binds as a MethodProviderFunc returning the
//     pinned rows, MyBean.getBean(int) binds as Construct with a factory
//     decoding the intPrimitive argument, and the js: script binds as a
//     registered Go provider.
//   - Java's EPCompiled.getClasses() exclusion for path-provided
//     create-class bytes has no Go artifact counterpart; the oracle
//     asserts it inside the deploy step.
//   - Java asserts the output-col-type property type is the inlined
//     MyBean class; Go projects the constructed instance through GetId so
//     both traces carry c0=10 (the type-use new-keyword precedent).
//   - All five invalid probes are unrepresentable on the typed Go
//     surface: Go has no annotation surface, no class-by-name schema or
//     property-type resolution, so no Go rejection boundary exists. The
//     compile-error records pin the Java prefixes only.
//   - The script case's runtime failure is unrepresentable: Go ScriptCall
//     folds a provider error to Null (verified by the runner on send)
//     while Java raises EPException out of sendEventBean. Both traces pin
//     the same unrepresentable record documenting the contract.

const exprClassForEPLObjectsID = "expr-class-for-epl-objects"
const exprClassForEPLObjectsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprClassForEPLObjectsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassForEPLObjects.java",
}

var (
	// Inventory order is authoritative: ExprClassResolutionFromClauseMethod
	// is ordinal 0, ExprClassResolutionOutputColType ordinal 1,
	// ExprClassResolutionInvalid ordinal 2 and ExprClassResolutionScript
	// ordinal 3 in ExprClassForEPLObjects.executions(). All four share the
	// inventory static id java-3fd69b8c66fd163574ec.
	exprClassForEPLObjectsJavaRuntimeIDs = []string{
		"java-runtime-cb993aa0d17d61b0d45f", // ExprClassResolutionFromClauseMethod
		"java-runtime-203a97dc1cba0371469c", // ExprClassResolutionOutputColType
		"java-runtime-1335a465f707da5fce60", // ExprClassResolutionInvalid
		"java-runtime-785742544d6ada82189d", // ExprClassResolutionScript
	}
	exprClassForEPLObjectsJavaExecutions = []string{
		"ExprClassResolutionFromClauseMethod",
		"ExprClassResolutionOutputColType",
		"ExprClassResolutionInvalid",
		"ExprClassResolutionScript",
	}
	exprClassForEPLObjectsJavaStaticIDs = []string{
		"java-3fd69b8c66fd163574ec",
		"java-3fd69b8c66fd163574ec",
		"java-3fd69b8c66fd163574ec",
		"java-3fd69b8c66fd163574ec",
	}
)

const (
	exprClassForEPLObjectsFromClauseMethodCase = "from-clause-method"
	exprClassForEPLObjectsOutputColTypeCase    = "output-col-type"
	exprClassForEPLObjectsInvalidCase          = "invalid"
	exprClassForEPLObjectsScriptCase           = "script"
)

var exprClassForEPLObjectsCaseOrder = []string{
	exprClassForEPLObjectsFromClauseMethodCase,
	exprClassForEPLObjectsOutputColTypeCase,
	exprClassForEPLObjectsInvalidCase,
	exprClassForEPLObjectsScriptCase,
}

var exprClassForEPLObjectsCaseOrdinals = []int{0, 1, 2, 3}

// Byte-exact EPL transcriptions of ExprClassForEPLObjects.java, including
// the escapeClass trailing space+newline, the single-line concatenation
// quirks ("private final int id;" followed by the constructor on the same
// line) and the duplicated @name('s0')@name('s0') annotation.
const (
	exprClassForEPLObjectsCreateClassEPL = "@public create inlined_class \"\"\"\n" +
		"  public class MyFromClauseMethod {\n" +
		"    public static MyBean[] getBeans() {\n" +
		"       return new MyBean[] {new MyBean(1), new MyBean(2)};\n" +
		"    }\n" +
		"    public static class MyBean {\n" +
		"      private final int id;" +
		"      public MyBean(int id) {this.id = id;}\n" +
		"      public int getId() {return id;}\n" +
		"    }\n" +
		"  }\n" +
		"\"\"\" \n"
	exprClassForEPLObjectsFromClauseSelectEPL = "@name('s0')" +
		"@name('s0') select s.id as c0 from SupportBean as e,\n" +
		"method:MyFromClauseMethod.getBeans() as s"
	exprClassForEPLObjectsOutputColTypeEPL = "inlined_class \"\"\"\n" +
		"  public class MyBean {\n" +
		"    private final int id;" +
		"    public MyBean(int id) {this.id = id;}\n" +
		"    public int getId() {return id;}\n" +
		"    public static MyBean getBean(int id) {return new MyBean(id);}\n" +
		"  }\n" +
		"\"\"\" \n" +
		"@name('s0') select MyBean.getBean(intPrimitive) as c0 from SupportBean"
	exprClassForEPLObjectsScriptEPL = "inlined_class \"\"\"\n" +
		"public class MyScriptResult {}" +
		"\"\"\" \n" +
		"expression Object[] js:myItemProducerScript() [\n" +
		"myItemProducerScript();" +
		"function myItemProducerScript() {" +
		"  var arrayType = Java.type(\"MyScriptResult\");\n" +
		"  var rows = new arrayType(2);\n" +
		"  return rows;\n" +
		"}]" +
		"@name('s0') select myItemProducerScript() from SupportBean"
)

// exprClassForEPLObjectsProbeEPLs pins the byte-exact EPL each invalid-case
// build-error step carries (escapeClass text plus the trailing newline the
// Java string concatenation produces).
var exprClassForEPLObjectsProbeEPLs = map[string]string{
	"annotation-class":     "inlined_class \"\"\"\npublic @interface MyAnnotation{}\"\"\" \n@MyAnnotation @name('s0') select * from SupportBean\n",
	"schema-bean-type":     "inlined_class \"\"\"\npublic class MyEventBean {}\"\"\" \ncreate schema MyEvent as MyEventBean\n",
	"schema-property-type": "inlined_class \"\"\"\npublic class MyEventBean {}\"\"\" \ncreate schema MyEvent as (field1 MyEventBean)\n",
	"window-property-type": "inlined_class \"\"\"\npublic class MyType {}\"\"\" \ncreate window MyWindow(myfield MyType)\n",
	"table-column-type":    "inlined_class \"\"\"\npublic class MyType {}\"\"\" \ncreate table MyTable(myfield MyType)\n",
}

// exprClassForEPLObjectsProbeErrors pins the Java message prefix each
// invalid probe asserts (assertMessage startsWith semantics). The
// table-column-type probe is "skip"-pinned in Java: the compile must fail
// but the message is unvalidated, so the prefix is empty.
var exprClassForEPLObjectsProbeErrors = map[string]string{
	"annotation-class":     "Failed to process statement annotations: Failed to resolve @-annotation class: Could not load annotation class by name 'MyAnnotation', please check imports",
	"schema-bean-type":     "Could not load class by name 'MyEventBean', please check imports",
	"schema-property-type": "Nestable type configuration encountered an unexpected property type name",
	"window-property-type": "Nestable type configuration encountered an unexpected property type name",
	"table-column-type":    "",
}

// exprClassForEPLObjectsScriptSendError pins the EPException message prefix
// the Java execution asserts on the script-case send.
const exprClassForEPLObjectsScriptSendError = "java.lang.RuntimeException: Unexpected exception in statement 's0'"

// exprClassForEPLObjectsScriptNote pins the unrepresentable record for the
// script case: Java's runtime EPException has no Go boundary because
// ScriptCall folds provider errors to Null.
const exprClassForEPLObjectsScriptNote = "runtime script class lookup: Java.type(\"MyScriptResult\") fails under Nashorn because the script engine classloader cannot see inlined classes; Java sendEventBean raises EPException 'java.lang.RuntimeException: Unexpected exception in statement 's0'' while Go ScriptCall folds the provider error to Null - no Go runtime-error boundary"

// exprClassForEPLObjectsDeployEPLs pins the byte-exact EPL each deploy step
// carries, keyed by case + "/" + statement label.
var exprClassForEPLObjectsDeployEPLs = map[string]string{
	exprClassForEPLObjectsFromClauseMethodCase + "/create-class": exprClassForEPLObjectsCreateClassEPL,
	exprClassForEPLObjectsFromClauseMethodCase + "/s0":           exprClassForEPLObjectsFromClauseSelectEPL,
	exprClassForEPLObjectsOutputColTypeCase + "/s0":              exprClassForEPLObjectsOutputColTypeEPL,
	exprClassForEPLObjectsScriptCase + "/s0":                     exprClassForEPLObjectsScriptEPL,
}

// exprClassForEPLObjectsCaseEPLs pins the display EPL each case entry
// carries: the case's deploy/probe EPLs concatenated in replay order.
var exprClassForEPLObjectsCaseEPLs = []string{
	exprClassForEPLObjectsCreateClassEPL + exprClassForEPLObjectsFromClauseSelectEPL,
	exprClassForEPLObjectsOutputColTypeEPL,
	exprClassForEPLObjectsProbeEPLs["annotation-class"] +
		exprClassForEPLObjectsProbeEPLs["schema-bean-type"] +
		exprClassForEPLObjectsProbeEPLs["schema-property-type"] +
		exprClassForEPLObjectsProbeEPLs["window-property-type"] +
		exprClassForEPLObjectsProbeEPLs["table-column-type"],
	exprClassForEPLObjectsScriptEPL,
}

var exprClassForEPLObjectsObservations = []string{
	"listener; the path-deployed @public create inlined_class MyFromClauseMethod.getBeans() feeds the method: from-clause join: SupportBean(\"E1\",10) yields one listener invocation with ordered c0=1 then c0=2; the dependent EPCompiled carries no MyFromClauseMethod classes (asserted inside the oracle)",
	"listener; MyBean.getBean(intPrimitive) returns the inlined-class instance whose c0 property type is MyBean (asserted inside the oracle): SupportBean(\"E1\",10) yields c0 rendered through getId() as 10",
	"compile-error; five tryInvalidCompile probes pin the Java message prefixes: inlined classes are invisible to annotation classes, create-schema bean types, nestable schema/window property types and table column types; the table probe pins failure only (Java 'skip')",
	"unrepresentable; the js: script's Java.type(\"MyScriptResult\") fails at runtime because Nashorn cannot see inlined classes - Java sendEventBean raises EPException 'java.lang.RuntimeException: Unexpected exception in statement 's0'' while Go ScriptCall folds the provider error to Null",
}

// exprClassForEPLObjectsBean mirrors the SupportBean fields the scenario
// sends.
type exprClassForEPLObjectsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// exprClassForEPLObjectsMyBean mirrors the inlined MyBean class
// instantiated by MyBean.getBean(int) in the output-col-type case.
type exprClassForEPLObjectsMyBean struct {
	id int32
}

// GetId mirrors MyBean.getId().
func (b exprClassForEPLObjectsMyBean) GetId() int32 { return b.id }

// runExprClassForEPLObjectsScenario replays the four ExprClassForEPLObjects
// executions (ords 0-3) as one differential chain. Mirroring the Java
// harness, each case compiles and deploys on a fresh environment/engine
// pair, sends one SupportBean, then undeploys.
func runExprClassForEPLObjectsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateExprClassForEPLObjectsScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprClassForEPLObjectsCaseOrder))
	for _, caseName := range exprClassForEPLObjectsCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprClassForEPLObjectsCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprClassForEPLObjectsID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprClassForEPLObjectsID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateExprClassForEPLObjectsScenario pins the scenario metadata against
// the Java contract: version, id, and per-case order.
func validateExprClassForEPLObjectsScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != exprClassForEPLObjectsID {
		return fmt.Errorf("%s scenario metadata is not pinned", exprClassForEPLObjectsID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprClassForEPLObjectsID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(exprClassForEPLObjectsCaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", exprClassForEPLObjectsID, step.Case)
		}
		if step.Case != exprClassForEPLObjectsCaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", exprClassForEPLObjectsID, caseIndex, step.Case, exprClassForEPLObjectsCaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(exprClassForEPLObjectsCaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", exprClassForEPLObjectsID, caseIndex, len(exprClassForEPLObjectsCaseOrder))
	}
	return nil
}

// runExprClassForEPLObjectsCase replays one execution on a fresh
// environment/engine pair. SupportBean registers as a Go struct type and
// each send delivers the pinned payload.
func runExprClassForEPLObjectsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprClassForEPLObjectsBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprClassForEPLObjectsRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: exprClassForEPLObjectsID}
	var sequence uint64
	record := func(batch esper.ResultBatch) {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			// Force-dispatched empty pairs are not recorded (Java oracle
			// convention: payload-carrying callbacks only).
			return
		}
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	// scriptBatches captures the script case's listener deliveries without
	// recording them: the Java runtime failure is pinned as an
	// unrepresentable record, so the Go-side Null fold is verified but not
	// traced.
	var scriptBatches []esper.ResultBatch

	var deployment *esper.Deployment
	deploy := func(step compat.Step) error {
		if step.Statement == "create-class" {
			// Go has no inlined_class directive: the create-class deploy is
			// a no-op after the pinned-EPL check; getBeans() binds as the
			// MethodProviderFunc inside the s0 query.
			return nil
		}
		query, err := exprClassForEPLObjectsQuery(env, caseName)
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return fmt.Errorf("build %q: %w", caseName, err)
		}
		deployment, err = engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("deploy %q: %w", caseName, err)
		}
		for _, statement := range deployment.Statements() {
			if statement.Name() != "s0" {
				continue
			}
			if caseName == exprClassForEPLObjectsScriptCase {
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					scriptBatches = append(scriptBatches, batch)
					return nil
				}); err != nil {
					return err
				}
				continue
			}
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				record(batch)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if pinned, ok := exprClassForEPLObjectsDeployEPLs[caseName+"/"+step.Statement]; !ok || step.Epl != pinned {
				return compat.Trace{}, fmt.Errorf("%s: deploy step %q carries an unpinned EPL %q", exprClassForEPLObjectsID, step.Statement, step.Epl)
			}
			if err := deploy(step); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy %q: %w", caseName, err)
				}
				deployment = nil
			}
		case "send":
			payload, err := decodeExprClassForEPLObjectsPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.SendEvent(ctx, payload); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
			if caseName == exprClassForEPLObjectsScriptCase {
				if err := verifyExprClassForEPLObjectsScriptFold(scriptBatches); err != nil {
					return compat.Trace{}, err
				}
			}
		case "build-error":
			if err := exprClassForEPLObjectsBuildError(&trace, caseName, step); err != nil {
				return compat.Trace{}, err
			}
		case "unrepresentable":
			if err := exprClassForEPLObjectsUnrepresentable(&trace, caseName, step); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", exprClassForEPLObjectsID, step.Op)
		}
	}
	return trace, nil
}

// verifyExprClassForEPLObjectsScriptFold asserts the script case's Go
// boundary: the provider error folds to Null and the listener observes a
// single c0=null row instead of Java's EPException.
func verifyExprClassForEPLObjectsScriptFold(batches []esper.ResultBatch) error {
	if len(batches) != 1 || len(batches[0].New) != 1 || len(batches[0].Old) != 0 {
		return fmt.Errorf("%s: script case listener deliveries = %#v, want one new-only batch", exprClassForEPLObjectsID, batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok {
		return fmt.Errorf("%s: script case delivery is not a row: %#v", exprClassForEPLObjectsID, batches[0].New[0])
	}
	if !row.Get("c0").IsNull() {
		return fmt.Errorf("%s: script case c0 = %v, want Null fold", exprClassForEPLObjectsID, row.Get("c0"))
	}
	return nil
}

// exprClassForEPLObjectsBuildError emits the pinned compile-error record
// for one invalid-case probe. All five probes are unrepresentable on the
// typed Go surface — Go has no annotation surface and no class-by-name
// schema, window or table type resolution — so no Go rejection boundary is
// claimed and the record carries the pinned Java prefix (empty for the
// "skip"-pinned table probe).
func exprClassForEPLObjectsBuildError(trace *compat.Trace, caseName string, step compat.Step) error {
	pinnedEPL, ok := exprClassForEPLObjectsProbeEPLs[step.Statement]
	if !ok || step.Epl != pinnedEPL {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", exprClassForEPLObjectsID, step.Statement, step.Epl)
	}
	pinnedError, ok := exprClassForEPLObjectsProbeErrors[step.Statement]
	if !ok || step.ExpectError != pinnedError {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned expectError", exprClassForEPLObjectsID, step.Statement)
	}
	record := compat.TraceRecord{
		Case:      caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	// Java omits the value field for "skip"-pinned probes; an empty string
	// would pin a message the oracle never asserted.
	if step.ExpectError != "" {
		record.Value = step.ExpectError
	}
	trace.Records = append(trace.Records, record)
	return nil
}

// exprClassForEPLObjectsUnrepresentable emits the pinned unrepresentable
// record for the script case's runtime failure after the send verified the
// Go-side Null fold.
func exprClassForEPLObjectsUnrepresentable(trace *compat.Trace, caseName string, step compat.Step) error {
	if step.Statement != "script-java-type" || step.ExpectError != exprClassForEPLObjectsScriptNote {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned", exprClassForEPLObjectsID, step.Statement)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// exprClassForEPLObjectsQuery builds the fluent equivalent of the pinned
// case EPL. Go has no inlined_class directive, so each class member binds
// as a typed Go expression carrying the pinned value.
func exprClassForEPLObjectsQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	stream := esper.From[exprClassForEPLObjectsBean](env, "SupportBean")
	switch caseName {
	case exprClassForEPLObjectsFromClauseMethodCase:
		// select s.id as c0 from SupportBean as e,
		// method:MyFromClauseMethod.getBeans() as s — getBeans() binds as a
		// MethodProviderFunc returning the pinned MyBean rows {id:1},{id:2}
		// and the Cartesian join projects the method stream's id.
		schema, err := esper.NewMapSchema("MyFromClauseMethodMyBean", []esper.FieldSpec{
			esper.FieldDef("id", reflect.TypeOf(int32(0))),
		})
		if err != nil {
			return esper.Query{}, err
		}
		provider := esper.MethodProviderFunc(func(_ context.Context, request esper.MethodRequest) ([]esper.Event, error) {
			rows := make([]esper.Event, 0, 2)
			for _, id := range []int32{1, 2} {
				event, err := esper.NewEvent(schema, map[string]any{"id": id}, request.Now)
				if err != nil {
					return nil, err
				}
				rows = append(rows, event)
			}
			return rows, nil
		})
		method := esper.FromMethod[map[string]any](env, "s", schema, provider)
		return esper.Join(stream, method).
			Select(esper.SelectRight("c0", esper.Field[map[string]any, int32]("id"))).
			Query(esper.StatementName("s0")), nil
	case exprClassForEPLObjectsOutputColTypeCase:
		// select MyBean.getBean(intPrimitive) as c0 — the static factory
		// binds as Construct with a factory decoding the intPrimitive
		// argument; the Java execution asserts getId() reflectively, so the
		// projection reads GetId and both traces carry c0=10.
		bean := esper.Construct[exprClassForEPLObjectsMyBean]("MyBean",
			func(args []esper.Value) (exprClassForEPLObjectsMyBean, error) {
				id, err := esper.As[int32](args[0])
				if err != nil {
					return exprClassForEPLObjectsMyBean{}, err
				}
				return exprClassForEPLObjectsMyBean{id: id}, nil
			},
			esper.Field[exprClassForEPLObjectsBean, int32]("intPrimitive"))
		return esper.Select(stream,
			esper.Alias("c0", esper.Method[int32](bean, "GetId")),
		).Query(esper.StatementName("s0")), nil
	case exprClassForEPLObjectsScriptCase:
		// select myItemProducerScript() — the js: script binds as a
		// registered Go provider whose Java.type("MyScriptResult") failure
		// is the provider error; ScriptCall folds it to Null where Java
		// raises EPException.
		if err := esper.RegisterScript[[]any](env, "myItemProducerScript", "js",
			func(esper.ScriptContext) ([]any, error) {
				return nil, fmt.Errorf("Java.type(\"MyScriptResult\") failed: inlined classes are not visible to the script engine")
			}); err != nil {
			return esper.Query{}, err
		}
		return esper.Select(stream,
			esper.Alias("c0", esper.ScriptCall[[]any](env, "myItemProducerScript")),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s has no query for case %q", exprClassForEPLObjectsID, caseName)
	}
}

func exprClassForEPLObjectsRuntimeURI(caseName string) string {
	switch caseName {
	case exprClassForEPLObjectsFromClauseMethodCase:
		return exprClassForEPLObjectsJavaRuntimeIDs[0]
	case exprClassForEPLObjectsOutputColTypeCase:
		return exprClassForEPLObjectsJavaRuntimeIDs[1]
	case exprClassForEPLObjectsInvalidCase:
		return exprClassForEPLObjectsJavaRuntimeIDs[2]
	case exprClassForEPLObjectsScriptCase:
		return exprClassForEPLObjectsJavaRuntimeIDs[3]
	}
	return "parity-" + exprClassForEPLObjectsID + "-" + caseName
}

// decodeExprClassForEPLObjectsPayload rebuilds the pinned send payload as a
// SupportBean struct.
func decodeExprClassForEPLObjectsPayload(step compat.Step) (exprClassForEPLObjectsBean, error) {
	if step.EventType != "SupportBean" {
		return exprClassForEPLObjectsBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return exprClassForEPLObjectsBean{}, err
	}
	return exprClassForEPLObjectsBean{
		TheString:    jsonString(payload["theString"]),
		IntPrimitive: jsonInt32(payload["intPrimitive"]),
	}, nil
}

const exprClassForEPLObjectsDescription = "ExprClassForEPLObjects ordinals 0-3 (all executions): from-clause-method deploys @public create inlined_class MyFromClauseMethod on the path then selects s.id as c0 from SupportBean joined with method:MyFromClauseMethod.getBeans() (duplicated @name('s0') pinned verbatim), yielding one listener invocation with ordered c0=1 then c0=2 for SupportBean(\"E1\",10); output-col-type selects MyBean.getBean(intPrimitive) as c0 typed as the inlined MyBean class, rendering c0 through getId() as 10; invalid runs five tryInvalidCompile probes (annotation class, create-schema bean type, schema/window property types, table column type) that fail because inlined classes are invisible to those resolution sites; script deploys a js: expression whose Java.type(\"MyScriptResult\") lookup fails at runtime with EPException 'java.lang.RuntimeException: Unexpected exception in statement 's0''. Go has no inlined_class directive, so class members bind as typed Go expressions (MethodProviderFunc rows, Construct+GetId, RegisterScript+ScriptCall); the script case's runtime failure folds to Null in Go and is pinned as an unrepresentable record."

// exprClassForEPLObjectsExpectedStep is one pinned schedule entry: a
// deploy, a send with an exact decoded payload, a build-error probe, an
// unrepresentable record, or undeploy-all.
type exprClassForEPLObjectsExpectedStep struct {
	op          string
	statement   string
	eventType   string
	epl         string
	expectError string
	payload     exprClassForEPLObjectsBean
	hasPayload  bool
}

// exprClassForEPLObjectsSchedules pins each case's step sequence after its
// case marker. The invalid case has no undeploy-all, mirroring the Java
// execution which never deploys.
var exprClassForEPLObjectsSchedules = map[string][]exprClassForEPLObjectsExpectedStep{
	exprClassForEPLObjectsFromClauseMethodCase: {
		{op: "deploy", statement: "create-class", epl: exprClassForEPLObjectsCreateClassEPL},
		{op: "deploy", statement: "s0", epl: exprClassForEPLObjectsFromClauseSelectEPL},
		{op: "send", eventType: "SupportBean", payload: exprClassForEPLObjectsBean{TheString: "E1", IntPrimitive: 10}, hasPayload: true},
		{op: "undeploy-all"},
	},
	exprClassForEPLObjectsOutputColTypeCase: {
		{op: "deploy", statement: "s0", epl: exprClassForEPLObjectsOutputColTypeEPL},
		{op: "send", eventType: "SupportBean", payload: exprClassForEPLObjectsBean{TheString: "E1", IntPrimitive: 10}, hasPayload: true},
		{op: "undeploy-all"},
	},
	exprClassForEPLObjectsInvalidCase: {
		{op: "build-error", statement: "annotation-class", epl: exprClassForEPLObjectsProbeEPLs["annotation-class"], expectError: exprClassForEPLObjectsProbeErrors["annotation-class"]},
		{op: "build-error", statement: "schema-bean-type", epl: exprClassForEPLObjectsProbeEPLs["schema-bean-type"], expectError: exprClassForEPLObjectsProbeErrors["schema-bean-type"]},
		{op: "build-error", statement: "schema-property-type", epl: exprClassForEPLObjectsProbeEPLs["schema-property-type"], expectError: exprClassForEPLObjectsProbeErrors["schema-property-type"]},
		{op: "build-error", statement: "window-property-type", epl: exprClassForEPLObjectsProbeEPLs["window-property-type"], expectError: exprClassForEPLObjectsProbeErrors["window-property-type"]},
		{op: "build-error", statement: "table-column-type", epl: exprClassForEPLObjectsProbeEPLs["table-column-type"], expectError: exprClassForEPLObjectsProbeErrors["table-column-type"]},
	},
	exprClassForEPLObjectsScriptCase: {
		{op: "deploy", statement: "s0", epl: exprClassForEPLObjectsScriptEPL},
		{op: "send", eventType: "SupportBean", payload: exprClassForEPLObjectsBean{TheString: "E1", IntPrimitive: 1}, expectError: exprClassForEPLObjectsScriptSendError, hasPayload: true},
		{op: "unrepresentable", statement: "script-java-type", expectError: exprClassForEPLObjectsScriptNote},
		{op: "undeploy-all"},
	},
}

// loadExprClassForEPLObjectsScenario decodes the checked-in scenario with
// strict raw-bytes validation: duplicate keys, unknown fields, metadata
// drift, and payload drift are all rejected before replay.
func loadExprClassForEPLObjectsScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprClassForEPLObjectsID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprClassForEPLObjectsID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprClassForEPLObjectsID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprClassForEPLObjectsID, err)
	}
	if err := requireExprClassForEPLObjectsFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprClassForEPLObjectsID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprClassForEPLObjectsID ||
		metadata.Description != exprClassForEPLObjectsDescription ||
		metadata.JavaCommit != exprClassForEPLObjectsJavaCommit ||
		metadata.JavaSource != exprClassForEPLObjectsJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprClassForEPLObjectsID)
	}
	if err := validateExprClassForEPLObjectsStringArray(root["javaRuntimes"], exprClassForEPLObjectsJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassForEPLObjectsStringArray(root["javaNames"], exprClassForEPLObjectsJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassForEPLObjectsStringArray(root["javaStaticIds"], exprClassForEPLObjectsJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassForEPLObjectsStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprClassForEPLObjectsCaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", exprClassForEPLObjectsID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprClassForEPLObjectsFields(object,
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
		if definition.Case != exprClassForEPLObjectsCaseOrder[index] ||
			definition.Ordinal != exprClassForEPLObjectsCaseOrdinals[index] ||
			definition.RuntimeID != exprClassForEPLObjectsJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprClassForEPLObjectsJavaExecutions[index] ||
			definition.Observation != exprClassForEPLObjectsObservations[index] ||
			definition.EPL != exprClassForEPLObjectsCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprClassForEPLObjectsID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 20 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 20 steps", exprClassForEPLObjectsID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireExprClassForEPLObjectsFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprClassForEPLObjectsFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireExprClassForEPLObjectsFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			// The script-case send additionally pins expectError (the Java
			// EPException prefix the oracle verifies); every other send
			// carries exactly op/case/eventType/payload.
			if err := requireExprClassForEPLObjectsFields(object, "op", "case", "eventType", "payload"); err != nil {
				if err := requireExprClassForEPLObjectsFields(object, "op", "case", "eventType", "payload", "expectError"); err != nil {
					return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
				}
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeExprClassForEPLObjectsStrictPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "build-error":
			if err := requireExprClassForEPLObjectsFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireExprClassForEPLObjectsFields(object, "op", "case", "statement", "expectError"); err != nil {
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
	if err := validateExprClassForEPLObjectsScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassForEPLObjectsSchedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateExprClassForEPLObjectsSchedule walks the decoded steps and pins
// the full per-case schedule.
func validateExprClassForEPLObjectsSchedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range exprClassForEPLObjectsCaseOrder {
		if index >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario ends before case %q", exprClassForEPLObjectsID, caseName)
		}
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", exprClassForEPLObjectsID, index, caseName)
		}
		index++
		for _, expected := range exprClassForEPLObjectsSchedules[caseName] {
			if index >= len(scenario.Steps) {
				return fmt.Errorf("%s scenario case %d schedule is truncated", exprClassForEPLObjectsID, caseIndex)
			}
			if err := validateExprClassForEPLObjectsScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
				return fmt.Errorf("%s scenario case %d step %d: %w", exprClassForEPLObjectsID, caseIndex, index, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", exprClassForEPLObjectsID)
	}
	return nil
}

func validateExprClassForEPLObjectsScheduleStep(step compat.Step, caseName string, expected exprClassForEPLObjectsExpectedStep) error {
	if step.Op != expected.op || step.Case != caseName {
		return fmt.Errorf("step %q is not pinned", expected.op)
	}
	if step.Statement != expected.statement || step.Epl != expected.epl || step.ExpectError != expected.expectError {
		return fmt.Errorf("step %q fields are not pinned", expected.op)
	}
	if expected.op == "send" {
		if step.EventType != expected.eventType {
			return fmt.Errorf("send event type %q is not pinned", step.EventType)
		}
		payload, err := decodeExprClassForEPLObjectsStrictPayload(step)
		if err != nil {
			return err
		}
		if expected.hasPayload && payload != expected.payload {
			return fmt.Errorf("send payload for %q is not pinned", step.EventType)
		}
	}
	return nil
}

// decodeExprClassForEPLObjectsStrictPayload decodes a send payload with a
// strict field set so extra keys are rejected, then returns the same
// SupportBean struct decodeExprClassForEPLObjectsPayload produces.
func decodeExprClassForEPLObjectsStrictPayload(step compat.Step) (exprClassForEPLObjectsBean, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return exprClassForEPLObjectsBean{}, err
	}
	if step.EventType != "SupportBean" {
		return exprClassForEPLObjectsBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	if err := requireExprClassForEPLObjectsFields(fields, "theString", "intPrimitive"); err != nil {
		return exprClassForEPLObjectsBean{}, err
	}
	var bean struct {
		TheString    string `json:"theString"`
		IntPrimitive int32  `json:"intPrimitive"`
	}
	if err := json.Unmarshal(step.Payload, &bean); err != nil {
		return exprClassForEPLObjectsBean{}, fmt.Errorf("decode SupportBean payload: %w", err)
	}
	return exprClassForEPLObjectsBean{TheString: bean.TheString, IntPrimitive: bean.IntPrimitive}, nil
}

func requireExprClassForEPLObjectsFields(object map[string]json.RawMessage, names ...string) error {
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

func validateExprClassForEPLObjectsStringArray(raw json.RawMessage, expected []string, name string) error {
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

// normalizeExprClassForEPLObjectsTrace is the identity normalizer: every
// deploy attaches a single s0 statement and listener delivery is
// synchronous on the sending thread, so the dispatch order is already the
// canonical record order on both traces.
func normalizeExprClassForEPLObjectsTrace(trace compat.Trace) compat.Trace {
	return trace
}
