package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for InfraNWTableEventType (3 executions, 4 cases):
// named-window/table event-type surfaces — two tryInvalidCompile probes
// pinning the name-collision messages when a window or table is declared
// under an existing event-type name, the two define-fields cycles asserting
// the create statement's positional column types (boxed int[] versus
// primitive int[primitive]) for the window and table variants, and the
// protected-window module whose private insert-into routes Fubar map events
// into the keepall Snafu window.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 0 InfraNWTableEventTypeInvalid            java-runtime-d3c3a24f11288969df7e
//   - ord 1 InfraNWTableEventTypeDefineFields       java-runtime-8dc1233d90336dcdb929
//   - ord 2 InfraNWTableEventTypeInsertIntoProtected java-runtime-14412293edec92196bd9
//
// Invalid pins the two collision prefixes: Java's create-window check
// rejects a window name that collides with an existing event type or
// schema, and the create-table check rejects a table name colliding with
// an event type; the Go probes register the schema env-level and verify
// the same rejection surfaces from CreateNamedWindow/CreateTable before
// recording the pinned prefix. Define-fields runs both cycles of the one
// Java execution as sibling cases sharing the runtime id: Java's int[]
// column is the boxed Integer[] type ([]*int32 in Go) while
// int[primitive] is the primitive int[] type ([]int32); the types record
// renders the Java class names positionally. Insert-into-protected pins
// the module's statement fan-out (the doubled @public literal on the
// create-schema statement is verbatim Java) and the keepall window's
// iterator order after the two Fubar sends.
//
// The Java suite deploys insert-into-protected as one module; the Go
// runner registers the Fubar schema env-level (the create-schema
// statement's only observable surface is the deployed marker) and deploys
// one plan per remaining deploy step in the same environment — module
// boundaries are a Java packaging detail with no observable effect.
const infraNWTableEventTypeID = "infra-nwtable-event-type"

const infraNWTableEventTypeDescription = "InfraNWTableEventType named-window/table event-type slice (ords 0-2): two tryInvalidCompile probes pinning the window/table name-collision messages against an existing event type (invalid, ord 0); the two define-fields cycles asserting the s0 event type's positional column types c0=Integer[] (int[]) and c1=int[] (int[primitive]) for the keepall window and unkeyed table variants (define-fields-window/-table, ord 1, one shared runtime); and the protected-window module replaying two Fubar map events through the private insert-into into the keepall Snafu window whose iterator yields foo/bar a:1, b:2 in order (insert-into-protected, ord 2). Compile-error records carry the pinned Java message prefixes; types records carry positional {name,type} entries with Java class names; the snapshot record mirrors assertPropsPerRowIterator (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableEventType.java)."

const infraNWTableEventTypeJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWTableEventTypeSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableEventType.java"

// Byte-exact EPL pins (InfraNWTableEventType.java lines 61-62/66-67 for
// ordinal 0, 75/76 for ordinal 1, and 34-37 for ordinal 2). The ord-0
// probe texts keep the source's per-statement trailing newlines; the ord-2
// case EPL keeps the module declaration and the doubled @public literal
// the source writes.
const (
	infraNWTableEventTypeInvalidWindowEPL = "create schema SchemaOne as (p0 string);\n" +
		"create window SchemaOne#keepall as SchemaOne;\n"
	infraNWTableEventTypeInvalidTableEPL = "create schema SchemaTwo as (p0 string);\n" +
		"create table SchemaTwo(c0 int);\n"

	infraNWTableEventTypeDefineWindowEPL = "@name('s0') @public create window MyInfra#keepall as (c0 int[], c1 int[primitive])"
	infraNWTableEventTypeDefineTableEPL  = "@name('s0') @public create table MyInfra (c0 int[], c1 int[primitive])"

	infraNWTableEventTypeProtectedEventEPL  = "@name('event') @public @buseventtype @public create map schema Fubar as (foo string, bar double)"
	infraNWTableEventTypeProtectedWindowEPL = "@name('window') @protected create window Snafu#keepall as Fubar"
	infraNWTableEventTypeProtectedInsertEPL = "@name('insert') @private insert into Snafu select * from Fubar"
)

// The pinned Java message prefixes the compile-error records carry.
const (
	infraNWTableEventTypeWindowCollisionPrefix = "Error starting statement: An event type or schema by name 'SchemaOne' already exists"
	infraNWTableEventTypeTableCollisionPrefix  = "An event type by name 'SchemaTwo' has already been declared"
)

// infraNWTableEventTypeCaseSpec pins one case: identity, the case-level
// observation/EPL the scenario repeats, and whether the case deploys a
// named window or a table for the define-fields cycles.
type infraNWTableEventTypeCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
}

var infraNWTableEventTypeCaseSpecs = []infraNWTableEventTypeCaseSpec{
	{
		name:      "invalid",
		ordinal:   0,
		runtimeID: "java-runtime-d3c3a24f11288969df7e",
		execution: "InfraNWTableEventTypeInvalid",
		observation: "compile-error; two tryInvalidCompile probes: create window SchemaOne#keepall" +
			" colliding with the SchemaOne event type (prefix 'Error starting statement: An event" +
			" type or schema by name 'SchemaOne' already exists') and create table SchemaTwo" +
			" colliding with the SchemaTwo event type (prefix 'An event type by name 'SchemaTwo'" +
			" has already been declared')",
		epl: infraNWTableEventTypeInvalidWindowEPL + infraNWTableEventTypeInvalidTableEPL,
	},
	{
		name:      "define-fields-window",
		ordinal:   1,
		runtimeID: "java-runtime-8dc1233d90336dcdb929",
		execution: "InfraNWTableEventTypeDefineFields",
		observation: "deployed+types; @name('s0') @public create window MyInfra#keepall as" +
			" (c0 int[], c1 int[primitive]): the s0 event type's positional descriptors pin" +
			" c0=Integer[] and c1=int[]",
		epl: infraNWTableEventTypeDefineWindowEPL,
	},
	{
		name:      "define-fields-table",
		ordinal:   1,
		runtimeID: "java-runtime-8dc1233d90336dcdb929",
		execution: "InfraNWTableEventTypeDefineFields",
		observation: "deployed+types; @name('s0') @public create table MyInfra (c0 int[]," +
			" c1 int[primitive]): the same positional descriptors pin c0=Integer[] and c1=int[]" +
			" for the table variant (second cycle of the shared execution)",
		epl: infraNWTableEventTypeDefineTableEPL,
	},
	{
		name:      "insert-into-protected",
		ordinal:   2,
		runtimeID: "java-runtime-14412293edec92196bd9",
		execution: "InfraNWTableEventTypeInsertIntoProtected",
		observation: "deployed+snapshot; one three-statement module (public bus map schema" +
			" Fubar(foo string, bar double), protected keepall window Snafu as Fubar, private" +
			" insert into Snafu select *): two Fubar map sends route into the window whose" +
			" iterator yields {foo:a,bar:1},{foo:b,bar:2} in order",
		epl: "module test;\n" +
			infraNWTableEventTypeProtectedEventEPL + ";\n" +
			infraNWTableEventTypeProtectedWindowEPL + ";\n" +
			infraNWTableEventTypeProtectedInsertEPL + ";\n",
	},
}

var (
	infraNWTableEventTypeJavaSources = []string{
		infraNWTableEventTypeSource,
	}
	infraNWTableEventTypeJavaRuntimeIDs = []string{
		"java-runtime-d3c3a24f11288969df7e",
		"java-runtime-8dc1233d90336dcdb929",
		"java-runtime-14412293edec92196bd9",
	}
	infraNWTableEventTypeJavaExecutions = []string{
		"InfraNWTableEventTypeInvalid",
		"InfraNWTableEventTypeDefineFields",
		"InfraNWTableEventTypeInsertIntoProtected",
	}
	infraNWTableEventTypeJavaStaticIDs = []string{
		"java-08b4a7da65c76bf5abdc",
		"java-50c0d9ea894da18ddb54",
		"java-c007ca393634b73ec8b5",
	}
	infraNWTableEventTypeCases = []string{
		"invalid",
		"define-fields-window",
		"define-fields-table",
		"insert-into-protected",
	}
)

// infraNWTableEventTypeBoxedInts is the Java int[] column type (boxed
// Integer[]); infraNWTableEventTypePrimitiveInts is int[primitive].
var (
	infraNWTableEventTypeBoxedInts     = reflect.TypeOf((*[]*int32)(nil)).Elem()
	infraNWTableEventTypePrimitiveInts = reflect.TypeOf((*[]int32)(nil)).Elem()
)

func infraNWTableEventTypeCaseSpecFor(name string) (infraNWTableEventTypeCaseSpec, bool) {
	for _, spec := range infraNWTableEventTypeCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWTableEventTypeCaseSpec{}, false
}

func loadInfraNWTableEventTypeScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableEventTypeID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableEventTypeID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableEventTypeID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableEventTypeID, err)
	}
	if err := requireInfraNWTableEventTypeFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableEventTypeID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableEventTypeID ||
		metadata.Description != infraNWTableEventTypeDescription ||
		metadata.JavaCommit != infraNWTableEventTypeJavaCommit || metadata.JavaSource != infraNWTableEventTypeSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWTableEventTypeID)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWTableEventTypeID)
	}
	if err := infraNWTableEventTypeRequireEqual(metadata.JavaRuntimes, infraNWTableEventTypeJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWTableEventTypeRequireEqual(metadata.JavaNames, infraNWTableEventTypeJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWTableEventTypeRequireEqual(metadata.JavaStaticID, infraNWTableEventTypeJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWTableEventTypeID, err)
	}
	if len(rawCases) != len(infraNWTableEventTypeCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWTableEventTypeID, len(rawCases), len(infraNWTableEventTypeCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableEventTypeFields(object,
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
		spec := infraNWTableEventTypeCaseSpecs[index]
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
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWTableEventTypeID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWTableEventTypeID)
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
			if err := requireInfraNWTableEventTypeFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case", "statement", "epl"); err != nil {
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
			spec, ok := infraNWTableEventTypeCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			want, ok := infraNWTableEventTypeEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
		case "deployed":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "build-error":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case        string `json:"case"`
				Statement   string `json:"statement"`
				EPL         string `json:"epl"`
				ExpectError string `json:"expectError"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "invalid" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d build-error outside the invalid case", index)
			}
			wantEPL, wantError, ok := infraNWTableEventTypeProbeFor(step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d has unknown build-error probe %q", index, step.Statement)
			}
			if step.EPL != wantEPL || step.ExpectError != wantError {
				return compat.Scenario{}, fmt.Errorf("scenario step %d build-error probe %q is not pinned", index, step.Statement)
			}
		case "send":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string          `json:"case"`
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "insert-into-protected" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d sends outside insert-into-protected", index)
			}
			if _, err := infraNWTableEventTypeDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "types":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if (step.Case != "define-fields-window" && step.Case != "define-fields-table") || step.Statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d types probe %q/%q is not pinned", index, step.Case, step.Statement)
			}
		case "snapshot":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "insert-into-protected" || step.Statement != "window" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot %q/%q is not pinned", index, step.Case, step.Statement)
			}
		case "undeploy-all":
			if err := requireInfraNWTableEventTypeFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	if err := validateInfraNWTableEventTypeRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWTableEventTypeEPLForStep returns the byte-exact EPL of one deploy
// step.
func infraNWTableEventTypeEPLForStep(spec infraNWTableEventTypeCaseSpec, statement string) (string, bool) {
	switch spec.name {
	case "define-fields-window":
		if statement == "s0" {
			return infraNWTableEventTypeDefineWindowEPL, true
		}
	case "define-fields-table":
		if statement == "s0" {
			return infraNWTableEventTypeDefineTableEPL, true
		}
	case "insert-into-protected":
		switch statement {
		case "event":
			return infraNWTableEventTypeProtectedEventEPL, true
		case "window":
			return infraNWTableEventTypeProtectedWindowEPL, true
		case "insert":
			return infraNWTableEventTypeProtectedInsertEPL, true
		}
	}
	return "", false
}

// infraNWTableEventTypeProbeFor returns the pinned EPL text and Java
// message prefix of one invalid-case build-error probe.
func infraNWTableEventTypeProbeFor(statement string) (string, string, bool) {
	switch statement {
	case "window-name-collision":
		return infraNWTableEventTypeInvalidWindowEPL, infraNWTableEventTypeWindowCollisionPrefix, true
	case "table-name-collision":
		return infraNWTableEventTypeInvalidTableEPL, infraNWTableEventTypeTableCollisionPrefix, true
	}
	return "", "", false
}

// infraNWTableEventTypeCaseSteps pins the complete step sequence per case:
// case marker, the ord-0 build-error probes, deploy/deployed pairs in
// module order, the types probes, sends, the window snapshot and
// undeploy-all.
var infraNWTableEventTypeCaseSteps = map[string][]string{
	"invalid": {
		"build-error:window-name-collision:" + infraNWTableEventTypeInvalidWindowEPL + ":" + infraNWTableEventTypeWindowCollisionPrefix,
		"build-error:table-name-collision:" + infraNWTableEventTypeInvalidTableEPL + ":" + infraNWTableEventTypeTableCollisionPrefix,
	},
	"define-fields-window": {
		"deploy:s0:" + infraNWTableEventTypeDefineWindowEPL,
		"deployed:s0",
		"types:s0",
		"undeploy-all",
	},
	"define-fields-table": {
		"deploy:s0:" + infraNWTableEventTypeDefineTableEPL,
		"deployed:s0",
		"types:s0",
		"undeploy-all",
	},
	"insert-into-protected": {
		"deploy:event:" + infraNWTableEventTypeProtectedEventEPL,
		"deploy:window:" + infraNWTableEventTypeProtectedWindowEPL,
		"deploy:insert:" + infraNWTableEventTypeProtectedInsertEPL,
		"deployed:event",
		"deployed:window",
		"deployed:insert",
		"send:Fubar:{\"bar\":1,\"foo\":\"a\"}",
		"send:Fubar:{\"bar\":2,\"foo\":\"b\"}",
		"snapshot:window",
		"undeploy-all",
	},
}

func validateInfraNWTableEventTypeRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableEventTypeCases {
		want, ok := infraNWTableEventTypeCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableEventTypeID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableEventTypeID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWTableEventTypeID, caseName)
		}
		offset++
		for _, pinned := range want {
			var step struct {
				Op          string          `json:"op"`
				Case        string          `json:"case"`
				Statement   string          `json:"statement"`
				EPL         string          `json:"epl"`
				ExpectError string          `json:"expectError"`
				EventType   string          `json:"eventType"`
				Payload     json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWTableEventTypeID, offset, err)
			}
			if step.Case != caseName {
				return fmt.Errorf("%s step %d is not pinned for case %q", infraNWTableEventTypeID, offset, caseName)
			}
			var key string
			switch step.Op {
			case "deploy":
				key = "deploy:" + step.Statement + ":" + step.EPL
			case "deployed":
				key = "deployed:" + step.Statement
			case "build-error":
				key = "build-error:" + step.Statement + ":" + step.EPL + ":" + step.ExpectError
			case "send":
				var payload map[string]any
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWTableEventTypeID, offset, err)
				}
				canonical, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWTableEventTypeID, offset, err)
				}
				key = "send:" + step.EventType + ":" + string(canonical)
			case "types":
				key = "types:" + step.Statement
			case "snapshot":
				key = "snapshot:" + step.Statement
			case "undeploy-all":
				key = "undeploy-all"
			default:
				return fmt.Errorf("%s step %d has unsupported op %q", infraNWTableEventTypeID, offset, step.Op)
			}
			if key != pinned {
				return fmt.Errorf("%s step %d is not pinned: got %q want %q", infraNWTableEventTypeID, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWTableEventTypeID)
	}
	return nil
}

func infraNWTableEventTypeRequireEqual(got, want []string, label string) error {
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

func requireInfraNWTableEventTypeFields(object map[string]json.RawMessage, names ...string) error {
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

// runInfraNWTableEventTypeScenario replays the pinned executions, one
// fresh environment and engine per case (the two define-fields cycles
// share the Java runtime id but run on fresh environments exactly like
// the Java execution's undeployAll between cycles).
func runInfraNWTableEventTypeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWTableEventTypeID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWTableEventTypeCaseSpecs {
		caseTrace, err := runInfraNWTableEventTypeCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableEventTypeID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWTableEventTypeCase(ctx context.Context, scenario compat.Scenario, spec infraNWTableEventTypeCaseSpec) ([]compat.TraceRecord, error) {
	now := time.Unix(0, 0).UTC()
	sequence := map[string]uint64{}
	var records []compat.TraceRecord

	env := esper.NewEnvironment()
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
			if spec.name == "insert-into-protected" && step.Statement == "event" {
				// `@name('event') @public @buseventtype @public create map
				// schema Fubar as (foo string, bar double)` is a deployed
				// statement in Java; Go registers the schema env-level (the
				// only observable surface is the deployed marker).
				if _, err := esper.RegisterMap(env, "Fubar", []esper.FieldSpec{
					esper.FieldDef("foo", reflect.TypeOf("")),
					esper.FieldDef("bar", reflect.TypeOf(float64(0))),
				}, esper.BusEventType()); err != nil {
					return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
				}
				deployedLabels[step.Statement] = true
				continue
			}
			plan, err := infraNWTableEventTypeBuildPlan(env, spec, step.Statement)
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
		case "build-error":
			record, err := infraNWTableEventTypeBuildError(env, spec, step)
			if err != nil {
				return nil, err
			}
			records = append(records, record)
		case "send":
			payload, err := infraNWTableEventTypeDecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "types":
			// Java reads the deployed statement's event type via
			// `epService.EPRuntime.EventTypeService.GetEventType(...)`;
			// in Go the window/table schema is the event type, so the
			// runner reads it from the environment registry.
			var schema esper.Schema
			switch spec.name {
			case "define-fields-window":
				window, found := env.NamedWindow("MyInfra")
				if !found {
					return nil, fmt.Errorf("types statement %q: named window MyInfra is missing", step.Statement)
				}
				schema = window.Schema()
			case "define-fields-table":
				table, found := env.Table("MyInfra")
				if !found {
					return nil, fmt.Errorf("types statement %q: table MyInfra is missing", step.Statement)
				}
				schema = table.Schema()
			default:
				return nil, fmt.Errorf("types statement %q is not pinned for case %q", step.Statement, spec.name)
			}
			record := compat.TraceRecord{
				Case:      spec.name,
				Operation: "types",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
			}
			var entries []map[string]any
			for _, name := range schema.PropertyNames() {
				typ, _ := schema.PropertyType(name)
				entries = append(entries, map[string]any{"name": name, "type": infraNWTableEventTypeToken(typ)})
			}
			record.Value = entries
			records = append(records, record)
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return nil, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(now),
				New:       compat.NormalizeResults(result.Batch.New),
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

// infraNWTableEventTypeBuildError runs one expected-invalid compile probe
// against the fluent equivalent of the pinned EPL. The record carries the
// pinned Java message prefix once the Go rejection verifies, matching the
// oracle's prefix assertion (convention: context_variables.go buildError).
func infraNWTableEventTypeBuildError(env *esper.Environment, spec infraNWTableEventTypeCaseSpec, step compat.Step) (compat.TraceRecord, error) {
	record := compat.TraceRecord{
		Case:      spec.name,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	var buildErr error
	switch step.Statement {
	case "window-name-collision":
		// `create schema SchemaOne as (p0 string); create window
		// SchemaOne#keepall as SchemaOne;` — Java's create-window check
		// rejects the window name because the event type already exists.
		schema, err := esper.RegisterMap(env, "SchemaOne", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
		})
		if err != nil {
			return record, fmt.Errorf("%s: build-error probe %q schema registration failed: %w", infraNWTableEventTypeID, step.Statement, err)
		}
		_, buildErr = esper.CreateNamedWindow(env, "SchemaOne", schema,
			esper.NamedWindowRetention(esper.KeepAll()))
	case "table-name-collision":
		// `create schema SchemaTwo as (p0 string); create table
		// SchemaTwo(c0 int);` — Java's create-table check rejects the
		// table name because the event type was already declared.
		if _, err := esper.RegisterMap(env, "SchemaTwo", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
		}); err != nil {
			return record, fmt.Errorf("%s: build-error probe %q schema registration failed: %w", infraNWTableEventTypeID, step.Statement, err)
		}
		_, buildErr = esper.CreateTable(env, "SchemaTwo", []esper.TableColumn{
			esper.TableColumnOf[int32]("c0"),
		})
	default:
		return record, fmt.Errorf("%s: unknown build-error probe %q", infraNWTableEventTypeID, step.Statement)
	}
	if buildErr == nil {
		return record, fmt.Errorf("%s: build-error probe %q unexpectedly compiled", infraNWTableEventTypeID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"window-name-collision": {esper.ErrorInvalidRule, "An event type or schema by name 'SchemaOne' already exists"},
		"table-name-collision":  {esper.ErrorInvalidRule, "An event type by name 'SchemaTwo' has already been declared"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return record, fmt.Errorf("%s: build-error probe %q drift: got %v", infraNWTableEventTypeID, step.Statement, buildErr)
		}
	}
	if step.ExpectError != "" {
		record.Value = step.ExpectError
	}
	return record, nil
}

// infraNWTableEventTypeBuildPlan maps one scenario deploy statement onto
// the typed chain. The create statements register the named window or
// table env-level and deploy a direct-child query to keep the statement
// slot; the insert statement routes Fubar records into the Snafu window.
func infraNWTableEventTypeBuildPlan(env *esper.Environment, spec infraNWTableEventTypeCaseSpec, statement string) (esper.Plan, error) {
	switch spec.name {
	case "define-fields-window":
		if statement == "s0" {
			// `create window MyInfra#keepall as (c0 int[], c1
			// int[primitive])` — the inline column list is an anonymous
			// type, so the Go schema is built unregistered under the
			// window name (registering it would trip the same
			// event-type-name collision the invalid case pins).
			schema, err := esper.NewMapSchema("MyInfra", []esper.FieldSpec{
				esper.FieldDef("c0", infraNWTableEventTypeBoxedInts),
				esper.FieldDef("c1", infraNWTableEventTypePrimitiveInts),
			})
			if err != nil {
				return esper.Plan{}, err
			}
			if _, err := esper.CreateNamedWindow(env, "MyInfra", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "MyInfra").
				CreateNamedWindowQuery(esper.StatementName("s0")))
		}
	case "define-fields-table":
		if statement == "s0" {
			// `create table MyInfra (c0 int[], c1 int[primitive])` — an
			// unkeyed table; int[] maps to the boxed []*int32 column and
			// int[primitive] to the primitive []int32 column.
			if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
				esper.TableColumnOf[[]*int32]("c0"),
				esper.TableColumnOf[[]int32]("c1"),
			}); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromTable(env, "MyInfra").Query(esper.StatementName("s0")))
		}
	case "insert-into-protected":
		switch statement {
		case "window":
			// `@name('window') @protected create window Snafu#keepall as
			// Fubar` — the window rows carry the registered Fubar schema.
			schema, ok := env.Schema("Fubar")
			if !ok {
				return esper.Plan{}, fmt.Errorf("Fubar schema is missing")
			}
			if _, err := esper.CreateNamedWindow(env, "Snafu", schema,
				esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return esper.Plan{}, err
			}
			return env.Build(esper.FromNamedWindow(env, "Snafu").
				CreateNamedWindowQuery(esper.StatementName("window")))
		case "insert":
			// `@name('insert') @private insert into Snafu select * from
			// Fubar` — the wildcard route copies the matching fields.
			return env.Build(esper.OnRecord(esper.FromAny(env, "Fubar")).
				InsertIntoNamedWindow("Snafu", esper.CopyMatchingFields()).
				Query(esper.StatementName("insert")))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
}

func infraNWTableEventTypeDecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "Fubar":
		if err := requireInfraNWTableEventTypeFields(fields, "foo", "bar"); err != nil {
			return nil, fmt.Errorf("Fubar payload: %w", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode Fubar: %w", err)
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// infraNWTableEventTypeToken renders one column type the way the Java
// oracle prints it: Class.getSimpleName on the event-type property type.
// Java's int[] column is the boxed Integer[] type ([]*int32 in Go) while
// int[primitive] is the primitive int[] type ([]int32); other slices
// render their element token plus "[]" and scalars render the boxed Java
// name (convention: epl_other_select_expr.go eplOtherSelectExprTypeToken,
// extended locally for array types).
func infraNWTableEventTypeToken(typ reflect.Type) string {
	if typ == nil {
		return "Object"
	}
	if typ.Kind() == reflect.Slice {
		elem := typ.Elem()
		if elem.Kind() == reflect.Pointer {
			return infraNWTableEventTypeScalarToken(elem.Elem()) + "[]"
		}
		return infraNWTableEventTypePrimitiveToken(elem) + "[]"
	}
	return infraNWTableEventTypeScalarToken(typ)
}

// infraNWTableEventTypePrimitiveToken renders the Java primitive class
// name for array element types (int[] not Integer[]).
func infraNWTableEventTypePrimitiveToken(typ reflect.Type) string {
	switch typ.Kind() {
	case reflect.String:
		return "String"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "int"
	case reflect.Int64:
		return "long"
	case reflect.Float32:
		return "float"
	case reflect.Float64:
		return "double"
	case reflect.Bool:
		return "boolean"
	}
	return infraNWTableEventTypeScalarToken(typ)
}

// infraNWTableEventTypeScalarToken renders the boxed Java class name for
// scalar property types, mirroring eplOtherSelectExprTypeToken.
func infraNWTableEventTypeScalarToken(typ reflect.Type) string {
	if typ == nil {
		return "Object"
	}
	switch typ.Kind() {
	case reflect.String:
		return "String"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "Integer"
	case reflect.Int64:
		return "Long"
	case reflect.Float32:
		return "Float"
	case reflect.Float64:
		return "Double"
	case reflect.Bool:
		return "Boolean"
	case reflect.Pointer:
		return infraNWTableEventTypeScalarToken(typ.Elem())
	}
	return "Object"
}
