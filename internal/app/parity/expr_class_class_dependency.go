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

// expr_class_class_dependency.go replays the three ExprClassClassDependency
// executions (ordinals 0-2) against the pinned Java oracle as one replayable
// differential chain covering inlined_class class-to-class dependencies:
//
//   - all-local (ord 0, ExprClassClassDependencyAllLocal): one statement
//     carries two inlined_class blocks — MyUtil.someFunction wraps |p| and
//     MyClass.doIt calls MyUtil.someFunction(">"+p+"<"). Same-module inlined
//     classes compile in a single Janino batch, so MyClass sees MyUtil
//     regardless of textual order; one SupportBean("E1",1) yields a single
//     listener invocation with c0="|>E1<|" (assertEqualsNew).
//   - invalid (ord 1, ExprClassClassDependencyInvalid): compile-only. The
//     @public create inlined_class MyUtil compiles onto the path, then two
//     tryInvalidCompile probes pin the Java message prefixes — a
//     statement-local inlined_class and a create inlined_class depending on
//     the path-provided class both fail because inlined-class Janino cannot
//     see path-provided classes. The prefixes differ only by trailing space
//     ("Failed to compile an inlined-class:" vs "...: ").
//   - classpath (ord 2, ExprClassClassDependencyClasspath): two independent
//     deploy/undeploy cycles where inlined MyUtil.doIt calls
//     ExprClassClassDependency.supportQuoteString — first by fully-qualified
//     name, then by import (the import line has no trailing newline, so the
//     ';' and "public class MyUtil {" share one source line). Each cycle's
//     SupportBean("E1",1) yields c0="'E1'".
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle compiles the pinned EPL verbatim, including the
//     inlined_class triple-quote class text, the missing trailing space on
//     the invalid create-class EPL and the import-line quirk. Go has no
//     inlined_class directive, so the EPL text is never replayed:
//     MyClass.doIt binds as a Func1 whose closure calls the MyUtil
//     someFunction closure (all-local), and MyUtil.doIt binds as a Func1
//     closing over the supportQuoteString helper (classpath). The FQN-vs-
//     import distinction is EPL-text only; both classpath cycles bind the
//     same Go expression.
//   - The invalid case's create-class step is compile-only in Java
//     (env.compile + path.add); Go has no inlined_class directive so the
//     step is a no-op after the pinned-EPL check.
//   - Both invalid probes are unrepresentable on the typed Go surface: Go
//     has no inlined_class compilation boundary, so no Go rejection exists.
//     The compile-error records pin the Java prefixes only, preserving the
//     trailing-space difference.

const exprClassClassDependencyID = "expr-class-class-dependency"
const exprClassClassDependencyJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprClassClassDependencyJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassClassDependency.java",
}

var (
	// Inventory order is authoritative: ExprClassClassDependencyAllLocal is
	// ordinal 0, ExprClassClassDependencyInvalid ordinal 1 and
	// ExprClassClassDependencyClasspath ordinal 2 in
	// ExprClassClassDependency.executions(). The static ids come from
	// static-manifest.json per execution class.
	exprClassClassDependencyJavaRuntimeIDs = []string{
		"java-runtime-f53aa2ad87c7997d7c36", // ExprClassClassDependencyAllLocal
		"java-runtime-247c1169dd5c24bd21f0", // ExprClassClassDependencyInvalid
		"java-runtime-bc85bc8d79bc09bb8500", // ExprClassClassDependencyClasspath
	}
	exprClassClassDependencyJavaExecutions = []string{
		"ExprClassClassDependencyAllLocal",
		"ExprClassClassDependencyInvalid",
		"ExprClassClassDependencyClasspath",
	}
	exprClassClassDependencyJavaStaticIDs = []string{
		"java-9027f7526d0d0a784faa",
		"java-c83ffad3efc5ef5bef30",
		"java-a8a6b01c7c18cdfb950a",
	}
)

const (
	exprClassClassDependencyAllLocalCase  = "all-local"
	exprClassClassDependencyInvalidCase   = "invalid"
	exprClassClassDependencyClasspathCase = "classpath"
)

var exprClassClassDependencyCaseOrder = []string{
	exprClassClassDependencyAllLocalCase,
	exprClassClassDependencyInvalidCase,
	exprClassClassDependencyClasspathCase,
}

var exprClassClassDependencyCaseOrdinals = []int{0, 1, 2}

// Byte-exact EPL transcriptions of ExprClassClassDependency.java, including
// the escapeClass trailing space+newline, the missing trailing space on the
// invalid create-class EPL and the import-line quirk (no newline after the
// ';' so "public class MyUtil {" shares the import line).
const (
	exprClassClassDependencyAllLocalEPL = "@name('s0') " +
		"inlined_class \"\"\"\n" +
		"    public class MyUtil {\n" +
		"        public static String someFunction(String parameter) {\n" +
		"            return \"|\" + parameter + \"|\";\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\" \n" +
		"inlined_class \"\"\"\n" +
		"    public class MyClass {\n" +
		"        public static String doIt(String parameter) {\n" +
		"            return MyUtil.someFunction(\">\" + parameter + \"<\");\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\" \n" +
		"select MyClass.doIt(theString) as c0 from SupportBean\n"
	exprClassClassDependencyCreateClassEPL = "@public create inlined_class \"\"\"\n" +
		"    public class MyUtil {\n" +
		"        public static String someFunction(String parameter) {\n" +
		"            return \"|\" + parameter + \"|\";\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\""
	exprClassClassDependencyClasspathFQNEPL = "@name('s0') " +
		"inlined_class \"\"\"\n" +
		"    public class MyUtil {\n" +
		"        public static String doIt(String parameter) {\n" +
		"            return com.espertech.esper.regressionlib.suite.expr.clazz.ExprClassClassDependency.supportQuoteString(parameter);\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\" \n" +
		"select MyUtil.doIt(theString) as c0 from SupportBean\n"
	exprClassClassDependencyClasspathImportEPL = "@name('s0') " +
		"inlined_class \"\"\"\n" +
		"    import com.espertech.esper.regressionlib.suite.expr.clazz.ExprClassClassDependency;" +
		"    public class MyUtil {\n" +
		"        public static String doIt(String parameter) {\n" +
		"            return ExprClassClassDependency.supportQuoteString(parameter);\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\" \n" +
		"select MyUtil.doIt(theString) as c0 from SupportBean\n"
)

// exprClassClassDependencyProbeEPLs pins the byte-exact EPL each invalid-case
// build-error step carries (the statement-local probe keeps the escapeClass
// trailing space+newline; the create-class probe ends at the closing
// triple-quote exactly like the Java concatenation).
var exprClassClassDependencyProbeEPLs = map[string]string{
	"local-on-path-class": "inlined_class \"\"\"\n" +
		"    public class MyClass {\n" +
		"        public static String doIt(String parameter) {\n" +
		"            return MyUtil.someFunction(\">\" + parameter + \"<\");\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\" \n" +
		"select MyClass.doIt(theString) as c0 from SupportBean\n",
	"create-on-path-class": "create inlined_class \"\"\"\n" +
		"    public class MyClass {\n" +
		"        public static String doIt(String parameter) {\n" +
		"            return MyUtil.someFunction(\">\" + parameter + \"<\");\n" +
		"        }\n" +
		"    }\n" +
		"\"\"\"",
}

// exprClassClassDependencyProbeErrors pins the Java message prefix each
// invalid probe asserts (assertMessage startsWith semantics). The prefixes
// differ only by trailing space: the statement-local probe pins "Failed to
// compile an inlined-class:" while the create-class probe pins "Failed to
// compile an inlined-class: ".
var exprClassClassDependencyProbeErrors = map[string]string{
	"local-on-path-class":  "Failed to compile an inlined-class:",
	"create-on-path-class": "Failed to compile an inlined-class: ",
}

// exprClassClassDependencyDeployEPLs pins the byte-exact EPL each deploy step
// carries in replay order, keyed by case. The classpath case deploys s0 twice
// (FQN cycle then import cycle), so the runner consumes entries positionally.
var exprClassClassDependencyDeployEPLs = map[string][]string{
	exprClassClassDependencyAllLocalCase: {
		exprClassClassDependencyAllLocalEPL,
	},
	exprClassClassDependencyInvalidCase: {
		exprClassClassDependencyCreateClassEPL,
	},
	exprClassClassDependencyClasspathCase: {
		exprClassClassDependencyClasspathFQNEPL,
		exprClassClassDependencyClasspathImportEPL,
	},
}

// exprClassClassDependencyCaseEPLs pins the display EPL each case entry
// carries: the case's deploy/probe EPLs concatenated in replay order.
var exprClassClassDependencyCaseEPLs = []string{
	exprClassClassDependencyAllLocalEPL,
	exprClassClassDependencyCreateClassEPL +
		exprClassClassDependencyProbeEPLs["local-on-path-class"] +
		exprClassClassDependencyProbeEPLs["create-on-path-class"],
	exprClassClassDependencyClasspathFQNEPL + exprClassClassDependencyClasspathImportEPL,
}

var exprClassClassDependencyObservations = []string{
	"listener; the statement-local inlined_class pair compiles in one Janino batch so MyClass.doIt sees MyUtil.someFunction: SupportBean(\"E1\",1) yields c0=\"|>E1<|\" (MyUtil wraps |p| around MyClass's >p<)",
	"compile-error; two tryInvalidCompile probes pin the Java message prefixes: a statement-local inlined_class and a create inlined_class depending on the path-provided MyUtil both fail because inlined-class Janino cannot see path-provided classes (the prefixes differ only by trailing space)",
	"listener; two deploy/undeploy cycles: inlined MyUtil.doIt calls ExprClassClassDependency.supportQuoteString by fully-qualified name then by import (the import line has no trailing newline): each SupportBean(\"E1\",1) yields c0=\"'E1'\"",
}

// exprClassClassDependencyBean mirrors the SupportBean fields the scenario
// sends.
type exprClassClassDependencyBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// exprClassClassDependencySupportQuoteString mirrors the Java helper
// ExprClassClassDependency.supportQuoteString: "'" + s + "'".
func exprClassClassDependencySupportQuoteString(s string) string {
	return "'" + s + "'"
}

// runExprClassClassDependencyScenario replays the three
// ExprClassClassDependency executions (ords 0-2) as one differential chain.
// Mirroring the Java harness, each case compiles and deploys on a fresh
// environment/engine pair, sends the pinned SupportBean payloads, then
// undeploys; the invalid case never deploys a statement.
func runExprClassClassDependencyScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateExprClassClassDependencyScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprClassClassDependencyCaseOrder))
	for _, caseName := range exprClassClassDependencyCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprClassClassDependencyCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprClassClassDependencyID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprClassClassDependencyID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateExprClassClassDependencyScenario pins the scenario metadata against
// the Java contract: version, id, and per-case order.
func validateExprClassClassDependencyScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != exprClassClassDependencyID {
		return fmt.Errorf("%s scenario metadata is not pinned", exprClassClassDependencyID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprClassClassDependencyID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(exprClassClassDependencyCaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", exprClassClassDependencyID, step.Case)
		}
		if step.Case != exprClassClassDependencyCaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", exprClassClassDependencyID, caseIndex, step.Case, exprClassClassDependencyCaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(exprClassClassDependencyCaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", exprClassClassDependencyID, caseIndex, len(exprClassClassDependencyCaseOrder))
	}
	return nil
}

// runExprClassClassDependencyCase replays one execution on a fresh
// environment/engine pair. SupportBean registers as a Go struct type and
// each send delivers the pinned payload.
func runExprClassClassDependencyCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprClassClassDependencyBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprClassClassDependencyRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: exprClassClassDependencyID}
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

	var deployment *esper.Deployment
	deploy := func(step compat.Step) error {
		if step.Statement == "create-class" {
			// Go has no inlined_class directive: the create-class step is a
			// no-op after the pinned-EPL check (Java compiles it onto the
			// path without deploying; the invalid probes are pinned
			// compile-error records).
			return nil
		}
		query, err := exprClassClassDependencyQuery(env, caseName)
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
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				record(batch)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	deployIndex := 0
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			pinned := exprClassClassDependencyDeployEPLs[caseName]
			if deployIndex >= len(pinned) || step.Epl != pinned[deployIndex] {
				return compat.Trace{}, fmt.Errorf("%s: deploy step %q carries an unpinned EPL %q", exprClassClassDependencyID, step.Statement, step.Epl)
			}
			deployIndex++
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
			payload, err := decodeExprClassClassDependencyPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.SendEvent(ctx, payload); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "build-error":
			if err := exprClassClassDependencyBuildError(&trace, caseName, step); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", exprClassClassDependencyID, step.Op)
		}
	}
	return trace, nil
}

// exprClassClassDependencyBuildError emits the pinned compile-error record
// for one invalid-case probe. Both probes are unrepresentable on the typed Go
// surface — Go has no inlined_class compilation boundary — so no Go rejection
// is claimed and the record carries the pinned Java prefix (the two prefixes
// differ only by trailing space).
func exprClassClassDependencyBuildError(trace *compat.Trace, caseName string, step compat.Step) error {
	pinnedEPL, ok := exprClassClassDependencyProbeEPLs[step.Statement]
	if !ok || step.Epl != pinnedEPL {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", exprClassClassDependencyID, step.Statement, step.Epl)
	}
	pinnedError, ok := exprClassClassDependencyProbeErrors[step.Statement]
	if !ok || step.ExpectError != pinnedError {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned expectError", exprClassClassDependencyID, step.Statement)
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// exprClassClassDependencyQuery builds the fluent equivalent of the pinned
// case EPL. Go has no inlined_class directive, so each class member binds as
// a typed Go expression carrying the pinned value.
func exprClassClassDependencyQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	stream := esper.From[exprClassClassDependencyBean](env, "SupportBean")
	theString := esper.Field[exprClassClassDependencyBean, string]("theString")
	switch caseName {
	case exprClassClassDependencyAllLocalCase:
		// select MyClass.doIt(theString) as c0 — MyClass.doIt calls
		// MyUtil.someFunction(">"+p+"<") and MyUtil wraps |p|; the
		// class-to-class call binds as Go closure composition inside the
		// Func1.
		someFunction := func(s string) string { return "|" + s + "|" }
		doIt := esper.Func1[string, string]("MyClass.doIt",
			func(s string) string { return someFunction(">" + s + "<") }, theString)
		return esper.Select(stream,
			esper.Alias("c0", doIt),
		).Query(esper.StatementName("s0")), nil
	case exprClassClassDependencyClasspathCase:
		// select MyUtil.doIt(theString) as c0 — MyUtil.doIt calls the
		// classpath helper ExprClassClassDependency.supportQuoteString; the
		// FQN-vs-import distinction is EPL-text only, so both cycles bind
		// the same Func1 closure over the Go helper.
		doIt := esper.Func1[string, string]("MyUtil.doIt",
			exprClassClassDependencySupportQuoteString, theString)
		return esper.Select(stream,
			esper.Alias("c0", doIt),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s has no query for case %q", exprClassClassDependencyID, caseName)
	}
}

func exprClassClassDependencyRuntimeURI(caseName string) string {
	switch caseName {
	case exprClassClassDependencyAllLocalCase:
		return exprClassClassDependencyJavaRuntimeIDs[0]
	case exprClassClassDependencyInvalidCase:
		return exprClassClassDependencyJavaRuntimeIDs[1]
	case exprClassClassDependencyClasspathCase:
		return exprClassClassDependencyJavaRuntimeIDs[2]
	}
	return "parity-" + exprClassClassDependencyID + "-" + caseName
}

// decodeExprClassClassDependencyPayload rebuilds the pinned send payload as a
// SupportBean struct.
func decodeExprClassClassDependencyPayload(step compat.Step) (exprClassClassDependencyBean, error) {
	if step.EventType != "SupportBean" {
		return exprClassClassDependencyBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return exprClassClassDependencyBean{}, err
	}
	return exprClassClassDependencyBean{
		TheString:    jsonString(payload["theString"]),
		IntPrimitive: jsonInt32(payload["intPrimitive"]),
	}, nil
}

const exprClassClassDependencyDescription = "ExprClassClassDependency ordinals 0-2 (all executions): all-local deploys one statement carrying two inlined_class blocks (MyUtil.someFunction wraps |p|, MyClass.doIt calls MyUtil.someFunction(\">\"+p+\"<\")) and selects MyClass.doIt(theString) as c0, yielding c0=\"|>E1<|\" for SupportBean(\"E1\",1); invalid compiles @public create inlined_class MyUtil onto the path then runs two tryInvalidCompile probes (a statement-local inlined_class and a create inlined_class depending on the path-provided class) that fail because inlined-class Janino cannot see path-provided classes, pinning the prefixes \"Failed to compile an inlined-class:\" and \"Failed to compile an inlined-class: \" (trailing space); classpath runs two deploy/undeploy cycles where inlined MyUtil.doIt calls ExprClassClassDependency.supportQuoteString by fully-qualified name then by import (the import line has no trailing newline), each SupportBean(\"E1\",1) yielding c0=\"'E1'\". Go has no inlined_class directive, so class members bind as typed Go expressions (Func1 composition for all-local, a Func1 closure over the supportQuoteString helper for classpath); the invalid probes are unrepresentable on the typed Go surface and pin the Java prefixes only."

// exprClassClassDependencyExpectedStep is one pinned schedule entry: a
// deploy, a send with an exact decoded payload, a build-error probe, or
// undeploy-all.
type exprClassClassDependencyExpectedStep struct {
	op          string
	statement   string
	eventType   string
	epl         string
	expectError string
	payload     exprClassClassDependencyBean
	hasPayload  bool
}

// exprClassClassDependencySchedules pins each case's step sequence after its
// case marker. The invalid case has no undeploy-all, mirroring the Java
// execution which never deploys.
var exprClassClassDependencySchedules = map[string][]exprClassClassDependencyExpectedStep{
	exprClassClassDependencyAllLocalCase: {
		{op: "deploy", statement: "s0", epl: exprClassClassDependencyAllLocalEPL},
		{op: "send", eventType: "SupportBean", payload: exprClassClassDependencyBean{TheString: "E1", IntPrimitive: 1}, hasPayload: true},
		{op: "undeploy-all"},
	},
	exprClassClassDependencyInvalidCase: {
		{op: "deploy", statement: "create-class", epl: exprClassClassDependencyCreateClassEPL},
		{op: "build-error", statement: "local-on-path-class", epl: exprClassClassDependencyProbeEPLs["local-on-path-class"], expectError: exprClassClassDependencyProbeErrors["local-on-path-class"]},
		{op: "build-error", statement: "create-on-path-class", epl: exprClassClassDependencyProbeEPLs["create-on-path-class"], expectError: exprClassClassDependencyProbeErrors["create-on-path-class"]},
	},
	exprClassClassDependencyClasspathCase: {
		{op: "deploy", statement: "s0", epl: exprClassClassDependencyClasspathFQNEPL},
		{op: "send", eventType: "SupportBean", payload: exprClassClassDependencyBean{TheString: "E1", IntPrimitive: 1}, hasPayload: true},
		{op: "undeploy-all"},
		{op: "deploy", statement: "s0", epl: exprClassClassDependencyClasspathImportEPL},
		{op: "send", eventType: "SupportBean", payload: exprClassClassDependencyBean{TheString: "E1", IntPrimitive: 1}, hasPayload: true},
		{op: "undeploy-all"},
	},
}

// loadExprClassClassDependencyScenario decodes the checked-in scenario with
// strict raw-bytes validation: duplicate keys, unknown fields, metadata
// drift, and payload drift are all rejected before replay.
func loadExprClassClassDependencyScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprClassClassDependencyID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprClassClassDependencyID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprClassClassDependencyID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprClassClassDependencyID, err)
	}
	if err := requireExprClassClassDependencyFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprClassClassDependencyID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprClassClassDependencyID ||
		metadata.Description != exprClassClassDependencyDescription ||
		metadata.JavaCommit != exprClassClassDependencyJavaCommit ||
		metadata.JavaSource != exprClassClassDependencyJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprClassClassDependencyID)
	}
	if err := validateExprClassClassDependencyStringArray(root["javaRuntimes"], exprClassClassDependencyJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassClassDependencyStringArray(root["javaNames"], exprClassClassDependencyJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassClassDependencyStringArray(root["javaStaticIds"], exprClassClassDependencyJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassClassDependencyStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprClassClassDependencyCaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", exprClassClassDependencyID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprClassClassDependencyFields(object,
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
		if definition.Case != exprClassClassDependencyCaseOrder[index] ||
			definition.Ordinal != exprClassClassDependencyCaseOrdinals[index] ||
			definition.RuntimeID != exprClassClassDependencyJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprClassClassDependencyJavaExecutions[index] ||
			definition.Observation != exprClassClassDependencyObservations[index] ||
			definition.EPL != exprClassClassDependencyCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprClassClassDependencyID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 15 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 15 steps", exprClassClassDependencyID)
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
			if err := requireExprClassClassDependencyFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprClassClassDependencyFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireExprClassClassDependencyFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireExprClassClassDependencyFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeExprClassClassDependencyStrictPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		case "build-error":
			if err := requireExprClassClassDependencyFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
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
	if err := validateExprClassClassDependencyScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassClassDependencySchedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateExprClassClassDependencySchedule walks the decoded steps and pins
// the full per-case schedule.
func validateExprClassClassDependencySchedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range exprClassClassDependencyCaseOrder {
		if index >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario ends before case %q", exprClassClassDependencyID, caseName)
		}
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", exprClassClassDependencyID, index, caseName)
		}
		index++
		for _, expected := range exprClassClassDependencySchedules[caseName] {
			if index >= len(scenario.Steps) {
				return fmt.Errorf("%s scenario case %d schedule is truncated", exprClassClassDependencyID, caseIndex)
			}
			if err := validateExprClassClassDependencyScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
				return fmt.Errorf("%s scenario case %d step %d: %w", exprClassClassDependencyID, caseIndex, index, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", exprClassClassDependencyID)
	}
	return nil
}

func validateExprClassClassDependencyScheduleStep(step compat.Step, caseName string, expected exprClassClassDependencyExpectedStep) error {
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
		payload, err := decodeExprClassClassDependencyStrictPayload(step)
		if err != nil {
			return err
		}
		if expected.hasPayload && payload != expected.payload {
			return fmt.Errorf("send payload for %q is not pinned", step.EventType)
		}
	}
	return nil
}

// decodeExprClassClassDependencyStrictPayload decodes a send payload with a
// strict field set so extra keys are rejected, then returns the same
// SupportBean struct decodeExprClassClassDependencyPayload produces.
func decodeExprClassClassDependencyStrictPayload(step compat.Step) (exprClassClassDependencyBean, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return exprClassClassDependencyBean{}, err
	}
	if step.EventType != "SupportBean" {
		return exprClassClassDependencyBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	if err := requireExprClassClassDependencyFields(fields, "theString", "intPrimitive"); err != nil {
		return exprClassClassDependencyBean{}, err
	}
	var bean struct {
		TheString    string `json:"theString"`
		IntPrimitive int32  `json:"intPrimitive"`
	}
	if err := json.Unmarshal(step.Payload, &bean); err != nil {
		return exprClassClassDependencyBean{}, fmt.Errorf("decode SupportBean payload: %w", err)
	}
	return exprClassClassDependencyBean{TheString: bean.TheString, IntPrimitive: bean.IntPrimitive}, nil
}

func requireExprClassClassDependencyFields(object map[string]json.RawMessage, names ...string) error {
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

func validateExprClassClassDependencyStringArray(raw json.RawMessage, expected []string, name string) error {
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

// normalizeExprClassClassDependencyTrace is the identity normalizer: every
// deploy attaches a single s0 statement and listener delivery is synchronous
// on the sending thread, so the dispatch order is already the canonical
// record order on both traces.
func normalizeExprClassClassDependencyTrace(trace compat.Trace) compat.Trace {
	return trace
}
