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

// expr_class_type_use.go replays the four ExprClassTypeUse executions
// (ordinals 0-3) against the pinned Java oracle as one replayable
// differential chain: enum binds an inlined-class enum constant and calls
// its accessor; const binds a public static final field; inner-class binds
// a static field on a nested class through the $ binary name; new-keyword
// instantiates an inlined class per event and reads it through its getter.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle compiles the pinned EPL verbatim, including the
//     inlined_class triple-quote class text. Go has no inlined_class
//     directive, so the EPL text is never replayed: each class member is
//     bound as a typed Go expression (Literal/Func0/Construct) and the
//     fluent query is named s0. The manifest records this surface
//     difference.
//   - Java's MyLevel.MEDIUM enum constant is the Go Literal ectuLevel{2}
//     and getLevelCode() is the exported GetLevelCode method; Java's
//     MyConstants.VALUE and MyConstants$MyInnerClass.VALUE static fields
//     are zero-argument Go functions returning the pinned values; Java's
//     new MyResult(theString) is Construct with a factory decoding the
//     theString argument, and getId() is the exported GetId method.
//   - Java asserts the new-keyword result reflectively (getId() == "E1");
//     the oracle trace renders the MyResult instance through getId() so
//     both traces carry c0="E1".

const exprClassTypeUseID = "expr-class-type-use"
const exprClassTypeUseJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprClassTypeUseJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassTypeUse.java",
}

var (
	// Inventory order is authoritative: ExprClassTypeUseEnum is ordinal 0,
	// ExprClassTypeConst ordinal 1, ExprClassTypeInnerClass ordinal 2 and
	// ExprClassTypeNewKeyword ordinal 3 in ExprClassTypeUse.executions().
	exprClassTypeUseJavaRuntimeIDs = []string{
		"java-runtime-c92db51ca175df9b4a5c", // ExprClassTypeUseEnum
		"java-runtime-8015ac40d3460b710377", // ExprClassTypeConst
		"java-runtime-6daa4310cb2a1762a0dd", // ExprClassTypeInnerClass
		"java-runtime-d6258e053355ac85022b", // ExprClassTypeNewKeyword
	}
	exprClassTypeUseJavaExecutions = []string{
		"ExprClassTypeUseEnum",
		"ExprClassTypeConst",
		"ExprClassTypeInnerClass",
		"ExprClassTypeNewKeyword",
	}
	exprClassTypeUseJavaStaticIDs = []string{
		"java-5d5ce4aa6c78b6bf048d",
		"java-f9f7cebbd7aac634448e",
		"java-a6911da1ff68fc3f399c",
		"java-13f14291ee7d14670c89",
	}
)

const (
	exprClassTypeUseEnumCase       = "enum"
	exprClassTypeUseConstCase      = "const"
	exprClassTypeUseInnerClassCase = "inner-class"
	exprClassTypeUseNewKeywordCase = "new-keyword"
)

var exprClassTypeUseCaseOrder = []string{
	exprClassTypeUseEnumCase,
	exprClassTypeUseConstCase,
	exprClassTypeUseInnerClassCase,
	exprClassTypeUseNewKeywordCase,
}

var exprClassTypeUseCaseOrdinals = []int{0, 1, 2, 3}

// exprClassTypeUseEPLs pins the contract EPL text verbatim: the
// inlined_class triple-quote class text (including the trailing space and
// newline after the closing quotes produced by escapeClass) and the
// @name('s0') select are byte-exact.
var exprClassTypeUseEPLs = []string{
	"inlined_class \"\"\"\n" +
		"public enum MyLevel {\n" +
		"  HIGH(3), MEDIUM(2), LOW(1);\n" +
		"  final int levelCode;\n" +
		"  MyLevel(int levelCode) {this.levelCode = levelCode;}\n" +
		"  public int getLevelCode() {return levelCode;}\n" +
		"}\"\"\" \n" +
		"@name('s0') select MyLevel.MEDIUM.getLevelCode() as c0 from SupportBean",
	"inlined_class \"\"\"\n" +
		"public class MyConstants {\n" +
		"  public final static String VALUE = \"test\";\n" +
		"}\"\"\" \n" +
		"@name('s0') select MyConstants.VALUE as c0 from SupportBean",
	"inlined_class \"\"\"\n" +
		"public class MyConstants {\n" +
		"  public static class MyInnerClass {" +
		"    public final static String VALUE = \"abc\";\n" +
		"  }" +
		"}\"\"\" \n" +
		"@name('s0') select MyConstants$MyInnerClass.VALUE as c0 from SupportBean",
	"inlined_class \"\"\"\n" +
		"public class MyResult {\n" +
		"  private final String id;\n" +
		"  public MyResult(String id) {this.id = id;}\n" +
		"  public String getId() {return id;}\n" +
		"}\"\"\" \n" +
		"@name('s0') select new MyResult(theString) as c0 from SupportBean",
}

var exprClassTypeUseObservations = []string{
	"listener; MyLevel.MEDIUM.getLevelCode() reads the inlined enum constant's accessor: SupportBean(\"E1\",0) yields c0=2",
	"listener; MyConstants.VALUE reads the inlined class's public static final field: SupportBean(\"E1\",0) yields c0=\"test\"",
	"listener; MyConstants$MyInnerClass.VALUE reads the nested class's static field through the $ binary name: SupportBean(\"E1\",0) yields c0=\"abc\"",
	"listener; new MyResult(theString) instantiates the inlined class per event and the Java execution asserts getId() reflectively: SupportBean(\"E1\",0) yields c0 rendered as \"E1\"",
}

// exprClassTypeUseBean mirrors the SupportBean fields the scenario sends.
type exprClassTypeUseBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// ectuLevel mirrors the MyLevel enum constant bound by the enum case;
// MEDIUM carries levelCode 2.
type ectuLevel struct {
	code int32
}

// GetLevelCode mirrors MyLevel.getLevelCode().
func (l ectuLevel) GetLevelCode() int32 { return l.code }

// ectuResult mirrors the MyResult inlined class instantiated by the
// new-keyword case.
type ectuResult struct {
	id string
}

// GetId mirrors MyResult.getId().
func (r ectuResult) GetId() string { return r.id }

// runExprClassTypeUseScenario replays the four ExprClassTypeUse executions
// (ords 0-3) as one differential chain. Mirroring the Java harness, each
// case compiles and deploys s0 once, sends one SupportBean, then
// undeploys.
func runExprClassTypeUseScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateExprClassTypeUseScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprClassTypeUseCaseOrder))
	for _, caseName := range exprClassTypeUseCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprClassTypeUseCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprClassTypeUseID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprClassTypeUseID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateExprClassTypeUseScenario pins the scenario metadata against the
// Java contract: version, id, and per-case order.
func validateExprClassTypeUseScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != exprClassTypeUseID {
		return fmt.Errorf("%s scenario metadata is not pinned", exprClassTypeUseID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprClassTypeUseID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(exprClassTypeUseCaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", exprClassTypeUseID, step.Case)
		}
		if step.Case != exprClassTypeUseCaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", exprClassTypeUseID, caseIndex, step.Case, exprClassTypeUseCaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(exprClassTypeUseCaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", exprClassTypeUseID, caseIndex, len(exprClassTypeUseCaseOrder))
	}
	return nil
}

// runExprClassTypeUseCase replays one execution on a fresh
// environment/engine pair. SupportBean registers as a Go struct type and
// the single send delivers the pinned SupportBean("E1", 0) payload.
func runExprClassTypeUseCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprClassTypeUseBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprClassTypeUseRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: exprClassTypeUseID}
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
	deploy := func() error {
		query, err := exprClassTypeUseQuery(env, caseName)
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
			if statement.Name() == "s0" {
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(batch)
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := deploy(); err != nil {
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
			payload, err := decodeExprClassTypeUsePayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.SendEvent(ctx, payload); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", exprClassTypeUseID, step.Op)
		}
	}
	return trace, nil
}

// exprClassTypeUseQuery builds the fluent equivalent of the pinned case
// EPL. Go has no inlined_class directive, so each class member binds as a
// typed Go expression carrying the pinned value.
func exprClassTypeUseQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	stream := esper.From[exprClassTypeUseBean](env, "SupportBean")
	switch caseName {
	case exprClassTypeUseEnumCase:
		// select MyLevel.MEDIUM.getLevelCode() as c0 — the MEDIUM constant
		// binds as a typed literal and getLevelCode() as a method call.
		return esper.Select(stream,
			esper.Alias("c0", esper.Method[int32](esper.Literal(ectuLevel{code: 2}), "GetLevelCode")),
		).Query(esper.StatementName("s0")), nil
	case exprClassTypeUseConstCase:
		// select MyConstants.VALUE as c0 — the static final field binds as
		// a zero-argument function returning the pinned value.
		return esper.Select(stream,
			esper.Alias("c0", esper.Func0[string]("MyConstants.VALUE",
				func() string { return "test" })),
		).Query(esper.StatementName("s0")), nil
	case exprClassTypeUseInnerClassCase:
		// select MyConstants$MyInnerClass.VALUE as c0 — the nested class's
		// static field binds under its $ binary name.
		return esper.Select(stream,
			esper.Alias("c0", esper.Func0[string]("MyConstants$MyInnerClass.VALUE",
				func() string { return "abc" })),
		).Query(esper.StatementName("s0")), nil
	case exprClassTypeUseNewKeywordCase:
		// select new MyResult(theString) as c0 — the constructor binds as
		// Construct with a factory decoding the theString argument; the
		// Java execution asserts getId() reflectively, so the projection
		// reads GetId and both traces carry c0="E1".
		result := esper.Construct[ectuResult]("MyResult",
			func(args []esper.Value) (ectuResult, error) {
				id, err := esper.As[string](args[0])
				if err != nil {
					return ectuResult{}, err
				}
				return ectuResult{id: id}, nil
			},
			esper.Field[exprClassTypeUseBean, string]("theString"))
		return esper.Select(stream,
			esper.Alias("c0", esper.Method[string](result, "GetId")),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s has no query for case %q", exprClassTypeUseID, caseName)
	}
}

func exprClassTypeUseRuntimeURI(caseName string) string {
	switch caseName {
	case exprClassTypeUseEnumCase:
		return exprClassTypeUseJavaRuntimeIDs[0]
	case exprClassTypeUseConstCase:
		return exprClassTypeUseJavaRuntimeIDs[1]
	case exprClassTypeUseInnerClassCase:
		return exprClassTypeUseJavaRuntimeIDs[2]
	case exprClassTypeUseNewKeywordCase:
		return exprClassTypeUseJavaRuntimeIDs[3]
	}
	return "parity-" + exprClassTypeUseID + "-" + caseName
}

// decodeExprClassTypeUsePayload rebuilds the pinned send payload as a
// SupportBean struct.
func decodeExprClassTypeUsePayload(step compat.Step) (exprClassTypeUseBean, error) {
	if step.EventType != "SupportBean" {
		return exprClassTypeUseBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	var payload map[string]any
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return exprClassTypeUseBean{}, err
	}
	return exprClassTypeUseBean{
		TheString:    jsonString(payload["theString"]),
		IntPrimitive: jsonInt32(payload["intPrimitive"]),
	}, nil
}

const exprClassTypeUseDescription = "ExprClassTypeUse ordinals 0-3 (all executions): enum binds the inlined MyLevel.MEDIUM enum constant and calls getLevelCode(); const binds the MyConstants.VALUE public static final field; inner-class binds MyConstants$MyInnerClass.VALUE through the $ binary name; new-keyword instantiates new MyResult(theString) per event and asserts getId() reflectively. Each case deploys s0 once, sends one SupportBean(\"E1\", 0), then undeploys. Go has no inlined_class directive, so the EPL text is never replayed: class members bind as typed Go expressions (Literal/Func0/Construct) and the new-keyword result reads through GetId so both traces carry c0=\"E1\"."

// exprClassTypeUseExpectedStep is one pinned schedule entry: a deploy, a
// send with an exact decoded payload, or undeploy-all.
type exprClassTypeUseExpectedStep struct {
	op        string
	statement string
	eventType string
	payload   exprClassTypeUseBean
}

// exprClassTypeUseSchedules pins each case's step sequence after its case
// marker: deploy s0, send the single assertion event and undeploy-all.
var exprClassTypeUseSchedules = map[string][]exprClassTypeUseExpectedStep{
	exprClassTypeUseEnumCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean", payload: exprClassTypeUseBean{TheString: "E1", IntPrimitive: 0}},
		{op: "undeploy-all"},
	},
	exprClassTypeUseConstCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean", payload: exprClassTypeUseBean{TheString: "E1", IntPrimitive: 0}},
		{op: "undeploy-all"},
	},
	exprClassTypeUseInnerClassCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean", payload: exprClassTypeUseBean{TheString: "E1", IntPrimitive: 0}},
		{op: "undeploy-all"},
	},
	exprClassTypeUseNewKeywordCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean", payload: exprClassTypeUseBean{TheString: "E1", IntPrimitive: 0}},
		{op: "undeploy-all"},
	},
}

// loadExprClassTypeUseScenario decodes the checked-in scenario with strict
// raw-bytes validation: duplicate keys, unknown fields, metadata drift,
// and payload drift are all rejected before replay.
func loadExprClassTypeUseScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprClassTypeUseID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprClassTypeUseID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprClassTypeUseID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprClassTypeUseID, err)
	}
	if err := requireExprClassTypeUseFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprClassTypeUseID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprClassTypeUseID ||
		metadata.Description != exprClassTypeUseDescription ||
		metadata.JavaCommit != exprClassTypeUseJavaCommit ||
		metadata.JavaSource != exprClassTypeUseJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprClassTypeUseID)
	}
	if err := validateExprClassTypeUseStringArray(root["javaRuntimes"], exprClassTypeUseJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassTypeUseStringArray(root["javaNames"], exprClassTypeUseJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassTypeUseStringArray(root["javaStaticIds"], exprClassTypeUseJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassTypeUseStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprClassTypeUseCaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", exprClassTypeUseID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprClassTypeUseFields(object,
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
		if definition.Case != exprClassTypeUseCaseOrder[index] ||
			definition.Ordinal != exprClassTypeUseCaseOrdinals[index] ||
			definition.RuntimeID != exprClassTypeUseJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprClassTypeUseJavaExecutions[index] ||
			definition.Observation != exprClassTypeUseObservations[index] ||
			definition.EPL != exprClassTypeUseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprClassTypeUseID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 16 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 16 steps", exprClassTypeUseID)
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
			if err := requireExprClassTypeUseFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprClassTypeUseFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireExprClassTypeUseFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireExprClassTypeUseFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeExprClassTypeUseStrictPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d payload: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateExprClassTypeUseScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprClassTypeUseSchedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateExprClassTypeUseSchedule walks the decoded steps and pins the
// full per-case schedule: case marker, then deploy s0, the assertion send
// and undeploy-all.
func validateExprClassTypeUseSchedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range exprClassTypeUseCaseOrder {
		if index >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario ends before case %q", exprClassTypeUseID, caseName)
		}
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", exprClassTypeUseID, index, caseName)
		}
		index++
		for _, expected := range exprClassTypeUseSchedules[caseName] {
			if index >= len(scenario.Steps) {
				return fmt.Errorf("%s scenario case %d schedule is truncated", exprClassTypeUseID, caseIndex)
			}
			if err := validateExprClassTypeUseScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
				return fmt.Errorf("%s scenario case %d step %d: %w", exprClassTypeUseID, caseIndex, index, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", exprClassTypeUseID)
	}
	return nil
}

func validateExprClassTypeUseScheduleStep(step compat.Step, caseName string, expected exprClassTypeUseExpectedStep) error {
	if step.Op != expected.op || step.Case != caseName {
		return fmt.Errorf("step %q is not pinned", expected.op)
	}
	switch expected.op {
	case "deploy":
		if step.Statement != expected.statement {
			return fmt.Errorf("deploy statement %q is not pinned", step.Statement)
		}
	case "send":
		if step.EventType != expected.eventType {
			return fmt.Errorf("send event type %q is not pinned", step.EventType)
		}
		payload, err := decodeExprClassTypeUseStrictPayload(step)
		if err != nil {
			return err
		}
		if payload != expected.payload {
			return fmt.Errorf("send payload for %q is not pinned", step.EventType)
		}
	}
	return nil
}

// decodeExprClassTypeUseStrictPayload decodes a send payload with a strict
// field set so extra keys are rejected, then returns the same SupportBean
// struct decodeExprClassTypeUsePayload produces.
func decodeExprClassTypeUseStrictPayload(step compat.Step) (exprClassTypeUseBean, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return exprClassTypeUseBean{}, err
	}
	if step.EventType != "SupportBean" {
		return exprClassTypeUseBean{}, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	if err := requireExprClassTypeUseFields(fields, "theString", "intPrimitive"); err != nil {
		return exprClassTypeUseBean{}, err
	}
	var bean struct {
		TheString    string `json:"theString"`
		IntPrimitive int32  `json:"intPrimitive"`
	}
	if err := json.Unmarshal(step.Payload, &bean); err != nil {
		return exprClassTypeUseBean{}, fmt.Errorf("decode SupportBean payload: %w", err)
	}
	return exprClassTypeUseBean{TheString: bean.TheString, IntPrimitive: bean.IntPrimitive}, nil
}

func requireExprClassTypeUseFields(object map[string]json.RawMessage, names ...string) error {
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

func validateExprClassTypeUseStringArray(raw json.RawMessage, expected []string, name string) error {
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

// normalizeExprClassTypeUseTrace is the identity normalizer: every deploy
// attaches a single s0 statement and listener delivery is synchronous on
// the sending thread, so the dispatch order is already the canonical
// record order on both traces.
func normalizeExprClassTypeUseTrace(trace compat.Trace) compat.Trace {
	return trace
}
