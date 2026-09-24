package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_enum_select_from.go replays the three remaining ExprEnumSelectFrom
// executions (ords 1, 3 and 4) against the pinned Java oracle as one
// replayable differential chain: events-windex-wsize projects contained
// events into anonymous new{v0,v1} map rows with element/index/size lambda
// footprints; scalar-plain projects strvals through the extractNum plug-in
// single-row function; scalar-windex-wsize projects strvals into index- and
// size-suffixed strings. Ords 0 (ExprEnumSelectFromEventsPlain) and 2
// (ExprEnumSelectFromEventsWithNew) are already covered by
// case.expr-enum-select and case.expr-enum-aggregate.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle deploys "@name('s0') <case EPL>"; the Go runner names
//     the fluent query s0. The pinned case EPL is the contract text verbatim,
//     including the double space in "selectFrom( (v, i)" and the no-space
//     assignments in "new {v0=v.id,v1=i}".
//   - Both event types register as Go map types (SendRecord payloads):
//     struct-backed typed-nil slices normalize to present-empty collections
//     under Go's value model, while Java selectFrom distinguishes a null
//     collection (null result) from an empty collection (present empty
//     result). Map-backed nil fields preserve the Java null state, matching
//     the expr-enum-select map-source precedent.
//   - Java's new{v0,v1} anonymous rows surface as Collection<Map>; the Go
//     EnumSelectMap rows are []map[string]any and both sides render as JSON
//     objects inside a JSON array.
//   - extractNum is the ExprEnumMinMax.MyService single-row function
//     (Integer.parseInt(arg.substring(1))); the Go Func1 mirror panics on a
//     parse failure like the Java NumberFormatException.

const exprEnumSelectFromID = "expr-enum-select-from"
const exprEnumSelectFromJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var exprEnumSelectFromJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumSelectFrom.java",
}

var (
	// Inventory order is authoritative: ExprEnumSelectFromEventsWIndexWSize
	// is ordinal 1, ExprEnumSelectFromScalarPlain ordinal 3 and
	// ExprEnumSelectFromScalarWIndexWSize ordinal 4 in
	// ExprEnumSelectFrom.executions().
	exprEnumSelectFromJavaRuntimeIDs = []string{
		"java-runtime-54fc91f276056a1d66be", // ExprEnumSelectFromEventsWIndexWSize
		"java-runtime-f8cd483364f4584c1bba", // ExprEnumSelectFromScalarPlain
		"java-runtime-754f50455eb82b9e64a7", // ExprEnumSelectFromScalarWIndexWSize
	}
	exprEnumSelectFromJavaExecutions = []string{
		"ExprEnumSelectFromEventsWIndexWSize",
		"ExprEnumSelectFromScalarPlain",
		"ExprEnumSelectFromScalarWIndexWSize",
	}
	exprEnumSelectFromJavaStaticIDs = []string{
		"java-952dc3d4e9027831151a",
		"java-4b6f2e5fc3f78b140fdb",
		"java-0a95beaf68eb949bdc86",
	}
)

const (
	exprEnumSelectFromEventsWIndexWSizeCase = "events-windex-wsize"
	exprEnumSelectFromScalarPlainCase       = "scalar-plain"
	exprEnumSelectFromScalarWIndexWSizeCase = "scalar-windex-wsize"
)

var exprEnumSelectFromCaseOrder = []string{
	exprEnumSelectFromEventsWIndexWSizeCase,
	exprEnumSelectFromScalarPlainCase,
	exprEnumSelectFromScalarWIndexWSizeCase,
}

var exprEnumSelectFromCaseOrdinals = []int{1, 3, 4}

// exprEnumSelectFromEPLs pins the contract EPL text verbatim: the explicit
// "as cN" aliases SupportEvalRunner appends, the double space in
// "selectFrom( (v, i)" and the no-space assignments in "new {v0=v.id,v1=i}"
// are byte-exact.
var exprEnumSelectFromEPLs = []string{
	"select contained.selectFrom( (v, i) => new {v0=v.id,v1=i}) as c0, contained.selectFrom( (v, i, s) => new {v0=v.id,v1=i + 100*s}) as c1 from SupportBean_ST0_Container",
	"select strvals.selectFrom(v => extractNum(v)) as c0 from SupportCollection",
	"select strvals.selectFrom( (v, i) => v || '_' || Integer.toString(i)) as c0, strvals.selectFrom( (v, i, s) => v || '_' || Integer.toString(i) || '_' || Integer.toString(s)) as c1 from SupportCollection",
}

var exprEnumSelectFromObservations = []string{
	"listener; contained.selectFrom projects SupportBean_ST0 elements into new{v0,v1} map rows: c0 carries the element index, c1 carries i + 100*s; a null contained yields null columns and an empty contained yields present empty collections",
	"listener; strvals.selectFrom applies the extractNum plug-in single-row function (Integer.parseInt of the value after its leading marker); a null strvals yields null and an empty strvals yields a present empty collection",
	"listener; strvals.selectFrom concatenates the element with its index (c0) and with index and input size (c1) through Integer.toString; a null strvals yields null columns and an empty strvals yields present empty collections",
}

// exprEnumSelectFromST0 mirrors the SupportBean_ST0 fields the scenario
// sends: id (projected as v0), key0 and p00 (unobserved but pinned for
// Java make3Value fidelity).
type exprEnumSelectFromST0 struct {
	ID   string `json:"id" esper:"id"`
	Key0 string `json:"key0" esper:"key0"`
	P00  int64  `json:"p00" esper:"p00"`
}

// runExprEnumSelectFromScenario replays the three remaining
// ExprEnumSelectFrom executions (ords 1, 3 and 4) as one differential
// chain. Mirroring SupportEvalRunner, each case deploys s0 once, sends
// every assertion event, then undeploys.
func runExprEnumSelectFromScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if err := validateExprEnumSelectFromScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	traces := make([]compat.Trace, 0, len(exprEnumSelectFromCaseOrder))
	for _, caseName := range exprEnumSelectFromCaseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		trace, err := runExprEnumSelectFromCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprEnumSelectFromID, caseName, err)
		}
		traces = append(traces, trace)
	}
	if len(traces) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", exprEnumSelectFromID, scenario.ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

// validateExprEnumSelectFromScenario pins the scenario metadata against the
// Java contract: version, id, and per-case order.
func validateExprEnumSelectFromScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != exprEnumSelectFromID {
		return fmt.Errorf("%s scenario metadata is not pinned", exprEnumSelectFromID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", exprEnumSelectFromID)
	}
	caseIndex := 0
	for _, step := range scenario.Steps {
		if step.Op != "case" {
			continue
		}
		if caseIndex >= len(exprEnumSelectFromCaseOrder) {
			return fmt.Errorf("%s scenario has unexpected case %q", exprEnumSelectFromID, step.Case)
		}
		if step.Case != exprEnumSelectFromCaseOrder[caseIndex] {
			return fmt.Errorf("%s scenario case %d = %q, want %q", exprEnumSelectFromID, caseIndex, step.Case, exprEnumSelectFromCaseOrder[caseIndex])
		}
		caseIndex++
	}
	if caseIndex != len(exprEnumSelectFromCaseOrder) {
		return fmt.Errorf("%s scenario has %d cases, want %d", exprEnumSelectFromID, caseIndex, len(exprEnumSelectFromCaseOrder))
	}
	return nil
}

// runExprEnumSelectFromCase replays one execution on a fresh
// environment/engine pair. Both event types register as map types so a nil
// field value keeps Java's null collection state (struct-backed typed-nil
// slices normalize to present-empty under Go's value model); sends deliver
// SendRecord payloads decoded from the pinned JSON.
func runExprEnumSelectFromCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterMap(env, "SupportBean_ST0_Container", []esper.FieldSpec{
		{Name: "contained", Type: reflect.TypeOf([]exprEnumSelectFromST0{})},
	}); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "SupportCollection", []esper.FieldSpec{
		{Name: "strvals", Type: reflect.TypeOf([]string{})},
	}); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprEnumSelectFromRuntimeURI(caseName)),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: compat.ScenarioVersion, ID: exprEnumSelectFromID}
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
		query, err := exprEnumSelectFromQuery(env, caseName)
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
			payload, err := decodeExprEnumSelectFromPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.SendRecord(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s unsupported step op %q", exprEnumSelectFromID, step.Op)
		}
	}
	return trace, nil
}

// exprEnumSelectFromQuery builds the fluent equivalent of the pinned case
// EPL. Both event types are map-backed, so the streams come from FromAny
// and the collection fields read through Field[any, []T].
func exprEnumSelectFromQuery(env *esper.Environment, caseName string) (esper.Query, error) {
	switch caseName {
	case exprEnumSelectFromEventsWIndexWSizeCase:
		// select contained.selectFrom( (v, i) => new {v0=v.id,v1=i}) as c0,
		//        contained.selectFrom( (v, i, s) => new {v0=v.id,v1=i + 100*s}) as c1
		// from SupportBean_ST0_Container
		contained := esper.Field[any, []exprEnumSelectFromST0]("contained")
		id := func() esper.Expression[string] {
			return esper.EnumField[exprEnumSelectFromST0, string]("id")
		}
		index := esper.EnumIndex()
		size := esper.EnumSize()
		hundred := esper.Literal(int64(100))
		return esper.FromAny(env, "SupportBean_ST0_Container").Select(
			esper.Alias("c0", esper.EnumSelectMap[exprEnumSelectFromST0](contained,
				esper.Alias("v0", id()),
				esper.Alias("v1", index))),
			esper.Alias("c1", esper.EnumSelectMap[exprEnumSelectFromST0](contained,
				esper.Alias("v0", id()),
				esper.Alias("v1", esper.AddOf[int64](index, esper.MultiplyOf[int64](hundred, size))))),
		).Query(esper.StatementName("s0")), nil
	case exprEnumSelectFromScalarPlainCase:
		// select strvals.selectFrom(v => extractNum(v)) as c0
		// from SupportCollection
		strvals := esper.Field[any, []string]("strvals")
		return esper.FromAny(env, "SupportCollection").Select(
			esper.Alias("c0", esper.EnumSelect[string, int64](strvals,
				esper.Func1[string, int64]("extractNum", exprEnumSelectFromExtractNum, esper.EnumElement[string]()))),
		).Query(esper.StatementName("s0")), nil
	case exprEnumSelectFromScalarWIndexWSizeCase:
		// select strvals.selectFrom( (v, i) => v || '_' || Integer.toString(i)) as c0,
		//        strvals.selectFrom( (v, i, s) => v || '_' || Integer.toString(i) || '_' || Integer.toString(s)) as c1
		// from SupportCollection — ConcatOf renders the int64 index/size
		// operands as decimal text, matching Integer.toString.
		strvals := esper.Field[any, []string]("strvals")
		underscore := esper.Literal("_")
		return esper.FromAny(env, "SupportCollection").Select(
			esper.Alias("c0", esper.EnumSelect[string, string](strvals,
				esper.ConcatOf(esper.EnumElement[string](), underscore, esper.EnumIndex()))),
			esper.Alias("c1", esper.EnumSelect[string, string](strvals,
				esper.ConcatOf(esper.EnumElement[string](), underscore, esper.EnumIndex(), underscore, esper.EnumSize()))),
		).Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("%s has no query for case %q", exprEnumSelectFromID, caseName)
	}
}

// exprEnumSelectFromExtractNum mirrors ExprEnumMinMax.MyService.extractNum:
// Integer.parseInt(arg.substring(1)). A parse failure panics like the Java
// NumberFormatException; recoverUDF turns it into Null.
func exprEnumSelectFromExtractNum(value string) int64 {
	parsed, err := strconv.ParseInt(value[1:], 10, 64)
	if err != nil {
		panic(err)
	}
	return parsed
}

func exprEnumSelectFromRuntimeURI(caseName string) string {
	switch caseName {
	case exprEnumSelectFromEventsWIndexWSizeCase:
		return exprEnumSelectFromJavaRuntimeIDs[0]
	case exprEnumSelectFromScalarPlainCase:
		return exprEnumSelectFromJavaRuntimeIDs[1]
	case exprEnumSelectFromScalarWIndexWSizeCase:
		return exprEnumSelectFromJavaRuntimeIDs[2]
	}
	return "parity-" + exprEnumSelectFromID + "-" + caseName
}

// decodeExprEnumSelectFromPayload rebuilds one pinned send payload as a
// SendRecord map. A JSON null collection decodes to an untyped nil so the
// map-backed field reads Null (Java's null collection); a JSON array
// decodes to the typed slice (empty stays a present empty collection).
func decodeExprEnumSelectFromPayload(step compat.Step) (map[string]any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	switch step.EventType {
	case "SupportBean_ST0_Container":
		if len(fields) != 1 || fields["contained"] == nil {
			return nil, fmt.Errorf("SupportBean_ST0_Container payload must carry only contained")
		}
		if exprEnumSelectFromJSONNull(fields["contained"]) {
			return map[string]any{"contained": nil}, nil
		}
		var items []exprEnumSelectFromST0
		if err := json.Unmarshal(fields["contained"], &items); err != nil {
			return nil, fmt.Errorf("decode SupportBean_ST0_Container contained: %w", err)
		}
		return map[string]any{"contained": items}, nil
	case "SupportCollection":
		if len(fields) != 1 || fields["strvals"] == nil {
			return nil, fmt.Errorf("SupportCollection payload must carry only strvals")
		}
		if exprEnumSelectFromJSONNull(fields["strvals"]) {
			return map[string]any{"strvals": nil}, nil
		}
		var items []string
		if err := json.Unmarshal(fields["strvals"], &items); err != nil {
			return nil, fmt.Errorf("decode SupportCollection strvals: %w", err)
		}
		return map[string]any{"strvals": items}, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

const exprEnumSelectFromDescription = "ExprEnumSelectFrom ordinals 1, 3 and 4 (the remaining executions): events-windex-wsize projects SupportBean_ST0_Container.contained into new{v0,v1} map rows with element/index/size lambda footprints; scalar-plain projects SupportCollection.strvals through the extractNum plug-in single-row function; scalar-windex-wsize projects strvals into index- and size-suffixed strings through Integer.toString concatenation. Null collections yield null columns and empty collections yield present empty results. Each case deploys s0 once, sends every assertion event, then undeploys."

// exprEnumSelectFromExpectedStep is one pinned schedule entry: a deploy, a
// send with an exact decoded payload, or undeploy-all.
type exprEnumSelectFromExpectedStep struct {
	op        string
	statement string
	eventType string
	payload   map[string]any
}

// exprEnumSelectFromSchedules pins each case's step sequence after its case
// marker: deploy s0, send every assertion event and undeploy.
var exprEnumSelectFromSchedules = map[string][]exprEnumSelectFromExpectedStep{
	exprEnumSelectFromEventsWIndexWSizeCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportBean_ST0_Container", payload: map[string]any{"contained": []exprEnumSelectFromST0{
			{ID: "E1", Key0: "12", P00: 0},
			{ID: "E2", Key0: "11", P00: 0},
			{ID: "E3", Key0: "2", P00: 0},
		}}},
		{op: "send", eventType: "SupportBean_ST0_Container", payload: map[string]any{"contained": []exprEnumSelectFromST0{
			{ID: "E4", Key0: "0", P00: 1},
		}}},
		{op: "send", eventType: "SupportBean_ST0_Container", payload: map[string]any{"contained": nil}},
		{op: "send", eventType: "SupportBean_ST0_Container", payload: map[string]any{"contained": []exprEnumSelectFromST0{}}},
		{op: "undeploy-all"},
	},
	exprEnumSelectFromScalarPlainCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": []string{"E2", "E1", "E5", "E4"}}},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": []string{"E1"}}},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": nil}},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": []string{}}},
		{op: "undeploy-all"},
	},
	exprEnumSelectFromScalarWIndexWSizeCase: {
		{op: "deploy", statement: "s0"},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": []string{"E1", "E2", "E3"}}},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": []string{}}},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": []string{"E1"}}},
		{op: "send", eventType: "SupportCollection", payload: map[string]any{"strvals": nil}},
		{op: "undeploy-all"},
	},
}

// loadExprEnumSelectFromScenario decodes the checked-in scenario with
// strict raw-bytes validation: duplicate keys, unknown fields, metadata
// drift, and payload drift are all rejected before replay.
func loadExprEnumSelectFromScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprEnumSelectFromID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprEnumSelectFromID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumSelectFromID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprEnumSelectFromID, err)
	}
	if err := requireExprEnumSelectFromFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprEnumSelectFromID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprEnumSelectFromID ||
		metadata.Description != exprEnumSelectFromDescription ||
		metadata.JavaCommit != exprEnumSelectFromJavaCommit ||
		metadata.JavaSource != exprEnumSelectFromJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", exprEnumSelectFromID)
	}
	if err := validateExprEnumSelectFromStringArray(root["javaRuntimes"], exprEnumSelectFromJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSelectFromStringArray(root["javaNames"], exprEnumSelectFromJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSelectFromStringArray(root["javaStaticIds"], exprEnumSelectFromJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSelectFromStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(exprEnumSelectFromCaseOrder) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", exprEnumSelectFromID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprEnumSelectFromFields(object,
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
		if definition.Case != exprEnumSelectFromCaseOrder[index] ||
			definition.Ordinal != exprEnumSelectFromCaseOrdinals[index] ||
			definition.RuntimeID != exprEnumSelectFromJavaRuntimeIDs[index] ||
			definition.ExecutionName != exprEnumSelectFromJavaExecutions[index] ||
			definition.Observation != exprEnumSelectFromObservations[index] ||
			definition.EPL != exprEnumSelectFromEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", exprEnumSelectFromID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 21 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly 21 steps", exprEnumSelectFromID)
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
			if err := requireExprEnumSelectFromFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprEnumSelectFromFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireExprEnumSelectFromFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireExprEnumSelectFromFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeExprEnumSelectFromStrictPayload(payload); err != nil {
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
	if err := validateExprEnumSelectFromScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprEnumSelectFromSchedule(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateExprEnumSelectFromSchedule walks the decoded steps and pins the
// full per-case schedule: case marker, then deploy s0, the assertion sends
// and undeploy-all.
func validateExprEnumSelectFromSchedule(scenario compat.Scenario) error {
	index := 0
	for caseIndex, caseName := range exprEnumSelectFromCaseOrder {
		if index >= len(scenario.Steps) {
			return fmt.Errorf("%s scenario ends before case %q", exprEnumSelectFromID, caseName)
		}
		if step := scenario.Steps[index]; step.Op != "case" || step.Case != caseName {
			return fmt.Errorf("%s scenario step %d must start case %q", exprEnumSelectFromID, index, caseName)
		}
		index++
		for _, expected := range exprEnumSelectFromSchedules[caseName] {
			if index >= len(scenario.Steps) {
				return fmt.Errorf("%s scenario case %d schedule is truncated", exprEnumSelectFromID, caseIndex)
			}
			if err := validateExprEnumSelectFromScheduleStep(scenario.Steps[index], caseName, expected); err != nil {
				return fmt.Errorf("%s scenario case %d step %d: %w", exprEnumSelectFromID, caseIndex, index, err)
			}
			index++
		}
	}
	if index != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", exprEnumSelectFromID)
	}
	return nil
}

func validateExprEnumSelectFromScheduleStep(step compat.Step, caseName string, expected exprEnumSelectFromExpectedStep) error {
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
		payload, err := decodeExprEnumSelectFromStrictPayload(step)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(payload, expected.payload) {
			return fmt.Errorf("send payload for %q is not pinned", step.EventType)
		}
	}
	return nil
}

// decodeExprEnumSelectFromStrictPayload decodes a send payload with a
// strict field set so extra keys are rejected, then returns the same
// SendRecord map decodeExprEnumSelectFromPayload produces.
func decodeExprEnumSelectFromStrictPayload(step compat.Step) (map[string]any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean_ST0_Container":
		if err := requireExprEnumSelectFromFields(fields, "contained"); err != nil {
			return nil, err
		}
		if !exprEnumSelectFromJSONNull(fields["contained"]) {
			var rawItems []json.RawMessage
			if err := json.Unmarshal(fields["contained"], &rawItems); err != nil {
				return nil, fmt.Errorf("decode SupportBean_ST0_Container contained: %w", err)
			}
			for itemIndex, rawItem := range rawItems {
				var itemFields map[string]json.RawMessage
				if err := strictObject(rawItem, &itemFields); err != nil {
					return nil, fmt.Errorf("decode SupportBean_ST0_Container item %d: %w", itemIndex, err)
				}
				if err := requireExprEnumSelectFromFields(itemFields, "id", "key0", "p00"); err != nil {
					return nil, fmt.Errorf("decode SupportBean_ST0_Container item %d: %w", itemIndex, err)
				}
			}
		}
	case "SupportCollection":
		if err := requireExprEnumSelectFromFields(fields, "strvals"); err != nil {
			return nil, err
		}
		if !exprEnumSelectFromJSONNull(fields["strvals"]) {
			var rawItems []json.RawMessage
			if err := json.Unmarshal(fields["strvals"], &rawItems); err != nil {
				return nil, fmt.Errorf("decode SupportCollection strvals: %w", err)
			}
			for itemIndex, rawItem := range rawItems {
				var item string
				if err := json.Unmarshal(rawItem, &item); err != nil {
					return nil, fmt.Errorf("decode SupportCollection strvals %d: %w", itemIndex, err)
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
	return decodeExprEnumSelectFromPayload(step)
}

func exprEnumSelectFromJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func requireExprEnumSelectFromFields(object map[string]json.RawMessage, names ...string) error {
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

func validateExprEnumSelectFromStringArray(raw json.RawMessage, expected []string, name string) error {
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

// normalizeExprEnumSelectFromTrace is the identity normalizer: every deploy
// attaches a single s0 statement and listener delivery is synchronous on
// the sending thread, so the dispatch order is already the canonical record
// order on both traces.
func normalizeExprEnumSelectFromTrace(trace compat.Trace) compat.Trace {
	return trace
}
