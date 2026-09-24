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

// Parity coverage for InfraNamedWindowSubquery (3 executions): named-window
// subqueries — an on-window set trigger whose scalar subquery observes the
// just-inserted #length(1) row, a late-deployed select-* consumer filtered by
// an uncorrelated count(*) subquery, and an exists-subquery with a parens
// named-window filter re-evaluated per trigger event.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 0 InfraSubqueryTwoConsumerWindow       java-runtime-901e88676d86ad580af1
//   - ord 1 InfraSubqueryLateConsumerAggregation java-runtime-4261803b5e9dcef8a867
//   - ord 2 InfraSubqueryWithFilterInParens      java-runtime-95adfcdb21f327e1cc4e
//
// Two-consumer-window pins consumer ordering: the window root view holds the
// inserted row before the on-set trigger dispatches, so the scalar mycount
// subquery sees the just-inserted row and myvar reads 1 (a pre-insert
// evaluation would leave it null). The Java variable is deployment-private;
// Go registers it env-level before the assign plan builds, which is the only
// observable surface (read-variable). Late-consumer-aggregation pins the
// preload boundary: E1/E2 insert between the insert and s0 deploys so the s0
// preload output is produced before the listener attaches and stays
// unobserved; the post-attach E3 insert fires the listener once.
// Filter-in-parens pins the parens named-window filter inside an uncorrelated
// exists-subquery, re-evaluated against live window contents per S0 trigger.
//
// The Java suite deploys two-consumer-window and filter-in-parens as single
// multi-statement modules and late-consumer-aggregation as three
// single-statement modules sharing one path; the Go runner deploys one plan
// per deploy step in the same environment (create variable maps to
// env.RegisterVariable) — module boundaries are a Java packaging detail with
// no observable effect beyond the deployed markers the scenario pins.
const infraNWSubqueryID = "infra-named-window-subquery"

const infraNWSubqueryDescription = "InfraNamedWindowSubquery named-window subquery slice (ords 0-2): an on-window set trigger whose scalar subquery observes the just-inserted #length(1) row and assigns the deployment variable myvar (two-consumer-window, ord 0), a late-deployed select-* consumer whose uncorrelated count(*) subquery filter passes for the post-attach E3 insert while the pre-attach E1/E2 preload stays unobserved (late-consumer-aggregation, ord 1), and an exists-subquery with a parens named-window filter re-evaluated live per SupportBean_S0 trigger (filter-in-parens, ord 2). Deployed markers pin the module fan-out; the ord-0 variable record carries the myvar value the Java assertRuntime pins (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowSubquery.java)."

const infraNWSubqueryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWSubquerySource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowSubquery.java"

// Byte-exact EPL pins (InfraNamedWindowSubquery.java lines 63-66 for
// ordinal 0, 79/80/85 for ordinal 1, and 38-40 for ordinal 2). The ord-0
// source module text carries a leading newline before the first statement;
// per-statement deploy EPLs exclude the source's inter-statement whitespace
// exactly like the other per-statement scenario slices.
const (
	infraNWSubqueryTwoCreate      = "create window MyWindowTwo#length(1) as (mycount long)"
	infraNWSubqueryTwoInsertCount = "@Name('insert-count') insert into MyWindowTwo select 1L as mycount from SupportBean"
	infraNWSubqueryTwoVariable    = "create variable long myvar = 0"
	infraNWSubqueryTwoAssign      = "@Name('assign') on MyWindowTwo set myvar = (select mycount from MyWindowTwo)"

	infraNWSubqueryLateCreate = "@public create window MyWindow#keepall as SupportBean"
	infraNWSubqueryLateInsert = "insert into MyWindow select * from SupportBean"
	infraNWSubqueryLateS0     = "@name('s0') select * from MyWindow where (select count(*) from MyWindow) > 0"

	infraNWSubqueryParensCreate = "create window MyWindow#keepall as SupportBean"
	infraNWSubqueryParensInsert = "@name('insert') insert into MyWindow select * from SupportBean"
	infraNWSubqueryParensS0     = "@name('s0') select exists (select * from MyWindow(theString='E1')) as c0 from SupportBean_S0"
)

// infraNWSubqueryCaseSpec pins one Java execution: identity, the listened
// statements and the case-level observation/EPL the scenario repeats.
type infraNWSubqueryCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	listened    map[string]bool
	observation string
	epl         string
}

var infraNWSubqueryCaseSpecs = []infraNWSubqueryCaseSpec{
	{
		name:      "two-consumer-window",
		ordinal:   0,
		runtimeID: "java-runtime-901e88676d86ad580af1",
		execution: "InfraSubqueryTwoConsumerWindow",
		listened:  map[string]bool{},
		observation: "deployed+variable; one four-statement module (length-1 window over" +
			" (mycount long), constant-1L insert-count, create variable long myvar = 0," +
			" on-window assign setting myvar to the scalar mycount subquery): the E1" +
			" insert fires the on-set trigger after the window root view holds the row," +
			" so the subquery-consumer sees the just-inserted row and myvar reads 1" +
			" (a pre-insert evaluation would leave it null)",
		epl: "\n create window MyWindowTwo#length(1) as (mycount long);\n" +
			" @Name('insert-count') insert into MyWindowTwo select 1L as mycount from SupportBean;\n" +
			" create variable long myvar = 0;\n" +
			" @Name('assign') on MyWindowTwo set myvar = (select mycount from MyWindowTwo);",
	},
	{
		name:      "late-consumer-aggregation",
		ordinal:   1,
		runtimeID: "java-runtime-4261803b5e9dcef8a867",
		execution: "InfraSubqueryLateConsumerAggregation",
		listened:  map[string]bool{"s0": true},
		observation: "listener+deployed; three separately deployed statements sharing one path" +
			" (@public keepall window, wildcard insert, s0 select-* whose where clause is an" +
			" uncorrelated count(*) subquery over the same window): E1/E2 insert between the" +
			" insert and s0 deploys so the s0 preload output stays unobserved (the listener" +
			" attaches after deploy), and the post-attach E3 insert fires the listener once" +
			" with the full SupportBean row",
		epl: "@public create window MyWindow#keepall as SupportBean;\n" +
			"insert into MyWindow select * from SupportBean;\n" +
			"@name('s0') select * from MyWindow where (select count(*) from MyWindow) > 0;\n",
	},
	{
		name:      "filter-in-parens",
		ordinal:   2,
		runtimeID: "java-runtime-95adfcdb21f327e1cc4e",
		execution: "InfraSubqueryWithFilterInParens",
		listened:  map[string]bool{"s0": true},
		observation: "listener+deployed; one three-statement module (keepall window, wildcard" +
			" insert, s0 selecting exists(select * from MyWindow(theString='E1')) as c0 from" +
			" SupportBean_S0): each S0(0) trigger re-evaluates the parens-filtered" +
			" exists-subquery against live window contents, yielding c0 false on the empty" +
			" window, false after the E2 insert and true after the E1 insert, each as" +
			" exactly one new event with no old data",
		epl: "create window MyWindow#keepall as SupportBean;\n" +
			"@name('insert') insert into MyWindow select * from SupportBean;\n" +
			"@name('s0') select exists (select * from MyWindow(theString='E1')) as c0 from SupportBean_S0;\n",
	},
}

var (
	infraNWSubqueryJavaSources = []string{
		infraNWSubquerySource,
	}
	infraNWSubqueryJavaRuntimeIDs = []string{
		"java-runtime-901e88676d86ad580af1",
		"java-runtime-4261803b5e9dcef8a867",
		"java-runtime-95adfcdb21f327e1cc4e",
	}
	infraNWSubqueryJavaExecutions = []string{
		"InfraSubqueryTwoConsumerWindow",
		"InfraSubqueryLateConsumerAggregation",
		"InfraSubqueryWithFilterInParens",
	}
	infraNWSubqueryJavaStaticIDs = []string{
		"java-f8f4ec7164371451341d",
		"java-ff6a2dd6de98a3f1bb81",
		"java-557b2408e6489254dd8e",
	}
	infraNWSubqueryCases = []string{
		"two-consumer-window",
		"late-consumer-aggregation",
		"filter-in-parens",
	}
)

// infraNWSubqueryLongType is the (mycount long) window column type.
var infraNWSubqueryLongType = reflect.TypeOf(int64(0))

func infraNWSubqueryCaseSpecFor(name string) (infraNWSubqueryCaseSpec, bool) {
	for _, spec := range infraNWSubqueryCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWSubqueryCaseSpec{}, false
}

func loadInfraNWSubqueryScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWSubqueryID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWSubqueryID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWSubqueryID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWSubqueryID, err)
	}
	if err := requireInfraNWSubqueryFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaStaticID []string `json:"javaStaticIds"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWSubqueryID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWSubqueryID ||
		metadata.Description != infraNWSubqueryDescription ||
		metadata.JavaCommit != infraNWSubqueryJavaCommit || metadata.JavaSource != infraNWSubquerySource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWSubqueryID)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWSubqueryID)
	}
	if err := infraNWSubqueryRequireEqual(metadata.JavaRuntimes, infraNWSubqueryJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWSubqueryRequireEqual(metadata.JavaNames, infraNWSubqueryJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWSubqueryRequireEqual(metadata.JavaStaticID, infraNWSubqueryJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWSubqueryID, err)
	}
	if len(rawCases) != len(infraNWSubqueryCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWSubqueryID, len(rawCases), len(infraNWSubqueryCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWSubqueryFields(object,
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
		spec := infraNWSubqueryCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Observation != spec.observation {
			return compat.Scenario{}, fmt.Errorf("scenario case %q observation does not match the pinned slice description", spec.name)
		}
		if definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWSubqueryID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWSubqueryID)
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
			if err := requireInfraNWSubqueryFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWSubqueryFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWSubqueryCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWSubqueryEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
		case "deployed":
			if err := requireInfraNWSubqueryFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWSubqueryFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := infraNWSubqueryDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "read-variable":
			if err := requireInfraNWSubqueryFields(object, "op", "case", "name"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "two-consumer-window" || step.Name != "myvar" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d reads unexpected variable %q for case %q", index, step.Name, step.Case)
			}
		case "undeploy-all":
			if err := requireInfraNWSubqueryFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	if err := validateInfraNWSubqueryRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWSubqueryEPLForStep returns the byte-exact EPL of one deploy step.
func infraNWSubqueryEPLForStep(spec infraNWSubqueryCaseSpec, statement string) (string, bool) {
	switch spec.name {
	case "two-consumer-window":
		switch statement {
		case "create":
			return infraNWSubqueryTwoCreate, true
		case "insert-count":
			return infraNWSubqueryTwoInsertCount, true
		case "variable":
			return infraNWSubqueryTwoVariable, true
		case "assign":
			return infraNWSubqueryTwoAssign, true
		}
	case "late-consumer-aggregation":
		switch statement {
		case "create":
			return infraNWSubqueryLateCreate, true
		case "insert":
			return infraNWSubqueryLateInsert, true
		case "s0":
			return infraNWSubqueryLateS0, true
		}
	case "filter-in-parens":
		switch statement {
		case "create":
			return infraNWSubqueryParensCreate, true
		case "insert":
			return infraNWSubqueryParensInsert, true
		case "s0":
			return infraNWSubqueryParensS0, true
		}
	}
	return "", false
}

// infraNWSubqueryCaseSteps pins the complete step sequence per case: case
// marker, deploy/deployed pairs in module order, sends, the ord-0
// read-variable and undeploy-all.
var infraNWSubqueryCaseSteps = map[string][]string{
	"two-consumer-window": {
		"deploy:create:" + infraNWSubqueryTwoCreate,
		"deploy:insert-count:" + infraNWSubqueryTwoInsertCount,
		"deploy:variable:" + infraNWSubqueryTwoVariable,
		"deploy:assign:" + infraNWSubqueryTwoAssign,
		"deployed:create",
		"deployed:insert-count",
		"deployed:variable",
		"deployed:assign",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"read-variable:myvar",
		"undeploy-all",
	},
	"late-consumer-aggregation": {
		"deploy:create:" + infraNWSubqueryLateCreate,
		"deployed:create",
		"deploy:insert:" + infraNWSubqueryLateInsert,
		"deployed:insert",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E2\"}",
		"deploy:s0:" + infraNWSubqueryLateS0,
		"deployed:s0",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E3\"}",
		"undeploy-all",
	},
	"filter-in-parens": {
		"deploy:create:" + infraNWSubqueryParensCreate,
		"deploy:insert:" + infraNWSubqueryParensInsert,
		"deploy:s0:" + infraNWSubqueryParensS0,
		"deployed:create",
		"deployed:insert",
		"deployed:s0",
		"send:SupportBean_S0:{\"id\":0}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E2\"}",
		"send:SupportBean_S0:{\"id\":0}",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean_S0:{\"id\":0}",
		"undeploy-all",
	},
}

func validateInfraNWSubqueryRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWSubqueryCases {
		want, ok := infraNWSubqueryCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWSubqueryID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWSubqueryID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWSubqueryID, caseName)
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
				Name      string          `json:"name"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWSubqueryID, offset, err)
			}
			if step.Case != caseName {
				return fmt.Errorf("%s step %d is not pinned for case %q", infraNWSubqueryID, offset, caseName)
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
					return fmt.Errorf("%s step %d send payload: %w", infraNWSubqueryID, offset, err)
				}
				canonical, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWSubqueryID, offset, err)
				}
				key = "send:" + step.EventType + ":" + string(canonical)
			case "read-variable":
				key = "read-variable:" + step.Name
			case "undeploy-all":
				key = "undeploy-all"
			default:
				return fmt.Errorf("%s step %d has unsupported op %q", infraNWSubqueryID, offset, step.Op)
			}
			if key != pinned {
				return fmt.Errorf("%s step %d is not pinned: got %q want %q", infraNWSubqueryID, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWSubqueryID)
	}
	return nil
}

func infraNWSubqueryRequireEqual(got, want []string, label string) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	return nil
}

func requireInfraNWSubqueryFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

// runInfraNWSubqueryScenario replays the three pinned executions, one fresh
// environment, window and engine per case.
func runInfraNWSubqueryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWSubqueryID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWSubqueryCaseSpecs {
		caseTrace, err := runInfraNWSubqueryCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWSubqueryID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWSubqueryCase(ctx context.Context, scenario compat.Scenario, spec infraNWSubqueryCaseSpec) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := map[string]uint64{}
	var records []compat.TraceRecord
	recordListener := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWOSBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWOSS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	statements := map[string]*esper.Statement{}
	deployedLabels := map[string]bool{}
	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		switch step.Op {
		case "deploy":
			if step.Statement == "variable" {
				// `create variable long myvar = 0` is a deployed statement in
				// Java; Go registers the variable env-level before the assign
				// plan builds (the only observable surface is read-variable).
				if err := env.RegisterVariable("myvar", int64(0)); err != nil {
					return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
				}
				deployedLabels[step.Statement] = true
				continue
			}
			plan, err := infraNWSubqueryBuildPlan(env, spec, step.Statement)
			if err != nil {
				return nil, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
				}
			}
			deployedLabels[step.Statement] = true
			if spec.listened[step.Statement] {
				statement, ok := statements[step.Statement]
				if !ok {
					return nil, fmt.Errorf("deploy %q did not register the statement", step.Statement)
				}
				name := step.Statement
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					recordListener(name, batch)
					return nil
				}); err != nil {
					return nil, err
				}
			}
		case "deployed":
			if !deployedLabels[step.Statement] {
				return nil, fmt.Errorf("deployed marker for unknown statement %q", step.Statement)
			}
			sequence[step.Statement+":deployed"]++
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			payload, err := infraNWSubqueryDecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "read-variable":
			value, ok := engine.GetVariable(step.Name)
			if !ok {
				return nil, fmt.Errorf("variable %q not found", step.Name)
			}
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "variable",
				Name:      step.Name,
				Value:     infraNWSubqueryVariableValue(value),
			})
		case "undeploy-all":
			// The Java suite tears the module(s) down at this point; teardown
			// emits no trace records, and the next case uses a fresh
			// environment, so nothing further is replayed here.
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	return records, nil
}

// infraNWSubqueryBuildPlan maps one scenario deploy statement onto the typed
// chain. The create statements register the named window env-level (ord 1's
// window is @public so the later s0 deployment resolves it) and deploy a
// direct-child select to keep the statement slot.
func infraNWSubqueryBuildPlan(env *esper.Environment, spec infraNWSubqueryCaseSpec, statement string) (esper.Plan, error) {
	switch spec.name {
	case "two-consumer-window":
		switch statement {
		case "create":
			schema, err := esper.NewMapSchema("MyWindowTwoType", []esper.FieldSpec{
				esper.FieldDef("mycount", infraNWSubqueryLongType),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if err := env.RegisterSchema(schema); err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindowTwo", schema,
				esper.NamedWindowRetention(esper.LengthWindow(1))); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindowTwo").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert-count":
			return env.Build(esper.OnEvent(esper.From[infraNWOSBean](env, "SupportBean")).
				InsertIntoNamedWindow("MyWindowTwo",
					esper.SetColumn("mycount", esper.Literal(int64(1)))).
				Query(esper.StatementName("insert-count")))
		case "assign":
			return env.Build(esper.OnRecord(esper.FromNamedWindow(env, "MyWindowTwo")).
				SetVariable("myvar", esper.SubqueryValue[int64](
					esper.FromNamedWindow(env, "MyWindowTwo"),
					esper.Field[any, int64]("mycount"))).
				Query(esper.StatementName("assign")))
		}
	case "late-consumer-aggregation":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWOSBean](env, "SupportBean")).
				InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		case "s0":
			// select * from MyWindow where (select count(*) from MyWindow) > 0:
			// the uncorrelated count subquery evaluates live per window row.
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				Filter(esper.Greater[int64](esper.SubqueryCount(esper.FromNamedWindow(env, "MyWindow")),
					esper.Literal(int64(0)))).
				Query(esper.StatementName("s0")))
		}
	case "filter-in-parens":
		switch statement {
		case "create":
			schema, ok := env.Schema("SupportBean")
			if !ok {
				return esper.Plan{}, fmt.Errorf("SupportBean schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "MyWindow", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyWindow").
				CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWOSBean](env, "SupportBean")).
				InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		case "s0":
			// exists (select * from MyWindow(theString='E1')): the parens
			// named-window filter maps to a Filter on the inner stream; the
			// exists-subquery itself carries no predicate.
			return env.Build(esper.Select(
				esper.From[infraNWOSS0](env, "SupportBean_S0"),
				esper.Alias("c0", esper.SubqueryExists(
					esper.FromNamedWindow(env, "MyWindow").Filter(
						esper.Equal[string](esper.Field[any, string]("theString"), esper.Literal("E1"))),
					nil)),
			).Query(esper.StatementName("s0")))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
}

func infraNWSubqueryDecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWSubqueryFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, fmt.Errorf("SupportBean payload: %w", err)
		}
		var bean infraNWOSBean
		bean.CharPrimitive = "\u0000"
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireInfraNWSubqueryFields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_S0 payload: %w", err)
		}
		var event infraNWOSS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// infraNWSubqueryVariableValue renders the myvar read the way the Java oracle
// prints it: a JSON integer. Go surfaces registered long variables as int64
// (or json.Number through the generic value path); anything else passes
// through so a wrong type still fails the comparison.
func infraNWSubqueryVariableValue(value esper.Value) any {
	raw := value.Any()
	switch number := raw.(type) {
	case json.Number:
		if parsed, err := number.Int64(); err == nil {
			return parsed
		}
	case int64:
		return number
	case int:
		return int64(number)
	}
	return raw
}
