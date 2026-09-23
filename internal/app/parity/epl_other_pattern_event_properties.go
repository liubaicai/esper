package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_other_pattern_event_properties.go replays EPLOtherPatternEventProperties
// ords 0-3 against the pinned Java oracle: event-pattern tag projections.
// Four executions on four runtimes, each deploying one @name('s0') statement
// before its sends and ending with undeployAll.
//
// wildcard-simple-pattern (ord 0, EPLOtherWildcardSimplePattern) deploys
// select * from pattern [a=SupportBean] and sends one default SupportBean:
// the row carries the tagged event under column a.
//
// wildcard-or-pattern (ord 1, EPLOtherWildcardOrPattern) deploys select *
// from pattern [every(a=SupportBean or b=SupportBeanComplexProps)]: a
// default SupportBean send yields {a=event,b=null}, then a default
// SupportBeanComplexProps send yields {b=event,a=null}.
//
// properties-simple-pattern (ord 2, EPLOtherPropertiesSimplePattern)
// deploys select a, a as myEvent, a.intPrimitive as myInt, a.theString and
// sends SupportBean{intPrimitive=1,theString="test"}: the event projects
// under both a and myEvent while the property columns read through the tag.
//
// properties-or-pattern (ord 3, EPLOtherPropertiesOrPattern) deploys the
// nine-column projection over the same every-or pattern: the
// SupportBeanComplexProps send delivers simple/indexed/nestedVal with the
// a-side columns null, then SupportBean{intPrimitive=2,theString="test2"}
// delivers myInt/a.theString with the b-side columns null.
//
// Approved differences (observably identical to the Java EPL):
//   - Go has no select-* chain form over pattern tags, so the wildcard
//     projections enumerate the pinned explicit Alias columns the Java
//     assertions read (a for ord 0; a,b for ord 1).
//   - Java asserts event identity with assertSame; identity is not
//     trace-observable, so both traces render the tagged event's field
//     values instead (SupportBean -> {theString,intPrimitive},
//     SupportBeanComplexProps -> the six-field schema projection).
//   - TagField validates plain property names only; the indexed[0] and
//     nested.nestedValue paths project through NestedField over the tagged
//     PatternEvent, which resolves full property paths at evaluation time.
//   - The default SupportBean's null theString is observable in the
//     wildcard rows, so the Go bean carries *string to preserve the
//     null/{state:null} rendering (a Go string would render "").

const eplOtherPatternEventPropertiesID = "epl-other-pattern-event-properties"
const eplOtherPatternEventPropertiesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const eplOtherPatternEventPropertiesDescription = "EPLOtherPatternEventProperties ords 0-3: event-pattern tag projections. Ord 0 selects * (the tagged event) from pattern [a=SupportBean]. Ord 1 selects * from pattern [every(a=SupportBean or b=SupportBeanComplexProps)] with the absent OR-branch tag projecting null. Ord 2 selects the tagged event twice plus its intPrimitive/theString properties. Ord 3 selects both tagged events, their aliases, and the simple/indexed/nested property paths across the every-or pattern (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherPatternEventProperties.java)."

const eplOtherPatternEventPropertiesSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherPatternEventProperties.java"

// Verbatim transcriptions of EPLOtherPatternEventProperties: setupSimplePattern
// (line 125) wraps select criteria "*" (ord 0, line 35) and
// "a, a as myEvent, a.intPrimitive as myInt, a.theString" (ord 2, line 70);
// setupOrPattern (lines 130-131) wraps "*" (ord 1, line 48) and the
// nine-column criteria (ord 3, lines 90-91).
const (
	pepEPLWildcardSimple = "@name('s0') select * from pattern [a=SupportBean]"
	pepEPLOrWildcard     = "@name('s0') select * from pattern [every(a=SupportBean or b=SupportBeanComplexProps)]"
	pepEPLSimpleProps    = "@name('s0') select a, a as myEvent, a.intPrimitive as myInt, a.theString from pattern [a=SupportBean]"
	pepEPLOrProps        = "@name('s0') select a, a as myAEvent, b, b as myBEvent, a.intPrimitive as myInt, a.theString, b.simpleProperty as simple, b.indexed[0] as indexed, b.nested.nestedValue as nestedVal from pattern [every(a=SupportBean or b=SupportBeanComplexProps)]"
)

var eplOtherPatternEventPropertiesJavaSources = []string{
	eplOtherPatternEventPropertiesSource,
}

var eplOtherPatternEventPropertiesJavaRuntimeIDs = []string{
	"java-runtime-97cfec67b539a40837da", // EPLOtherWildcardSimplePattern
	"java-runtime-157bfa584c8111e1ec07", // EPLOtherWildcardOrPattern
	"java-runtime-ec5a7e8cfd338e68a315", // EPLOtherPropertiesSimplePattern
	"java-runtime-e469a171adadf0bcd221", // EPLOtherPropertiesOrPattern
}

var eplOtherPatternEventPropertiesJavaExecutions = []string{
	"EPLOtherWildcardSimplePattern",
	"EPLOtherWildcardOrPattern",
	"EPLOtherPropertiesSimplePattern",
	"EPLOtherPropertiesOrPattern",
}

// One static-manifest id per execution, aligned with javaRuntimes order.
var eplOtherPatternEventPropertiesJavaStaticIDs = []string{
	"java-526cb327bfe0c4b23edb",
	"java-e66c56ee60a585967eac",
	"java-6f9f30d76350678d1ff0",
	"java-e3432d4a451aede65cac",
}

// The suite declares no execution flags for this file.
var eplOtherPatternEventPropertiesJavaFlags = []string{}

var eplOtherPatternEventPropertiesCases = []string{
	"wildcard-simple-pattern",
	"wildcard-or-pattern",
	"properties-simple-pattern",
	"properties-or-pattern",
}

var eplOtherPatternEventPropertiesOrdinals = []int{0, 1, 2, 3}

var eplOtherPatternEventPropertiesCaseObservations = []string{
	"listener; one deploy selects * from pattern [a=SupportBean]; one default SupportBean send delivers {a=event}",
	"listener; one deploy selects * from pattern [every(a=SupportBean or b=SupportBeanComplexProps)]; a default SupportBean send delivers {a=event,b=null} then a default SupportBeanComplexProps send delivers {b=event,a=null}",
	"listener; one deploy selects a, a as myEvent, a.intPrimitive as myInt, a.theString from pattern [a=SupportBean]; one SupportBean{intPrimitive=1,theString=test} send delivers {a=event,myEvent=event,myInt=1,a.theString=test}",
	"listener; one deploy selects the nine-column projection from pattern [every(a=SupportBean or b=SupportBeanComplexProps)]; a default SupportBeanComplexProps send delivers {b=event,simple=simple,indexed=1,nestedVal=nestedValue} with the a-side columns null, then SupportBean{intPrimitive=2,theString=test2} delivers {myInt=2,a.theString=test2} with the b-side columns null",
}

// eplOtherPatternEventPropertiesCaseEPLs pins the EPL of the case's single
// deploy — the value carried by the scenario cases[] metadata.
var eplOtherPatternEventPropertiesCaseEPLs = []string{
	pepEPLWildcardSimple,
	pepEPLOrWildcard,
	pepEPLSimpleProps,
	pepEPLOrProps,
}

// eplOtherPatternEventPropertiesCaseSteps pins the complete step sequence
// per case as op|case|statement|eventType|epl|payload|fields|listen keys so
// the loader asserts the scenario file matches the contract. Each Java
// execution deploys its statement before sending, so the runner deploys the
// pinned plan at case start and the steps carry sends only; undeployAll is
// mirrored by the per-case engine teardown.
var eplOtherPatternEventPropertiesCaseSteps = map[string][]string{
	"wildcard-simple-pattern": {
		`send|wildcard-simple-pattern||SupportBean||{"theString":null,"intPrimitive":0}||`,
	},
	"wildcard-or-pattern": {
		`send|wildcard-or-pattern||SupportBean||{"theString":null,"intPrimitive":0}||`,
		`send|wildcard-or-pattern||SupportBeanComplexProps||{"simpleProperty":"simple","mapped":{"keyOne":"valueOne","keyTwo":"valueTwo"},"indexed":[1,2],"mapProperty":{"xOne":"yOne","xTwo":"yTwo"},"arrayProperty":[10,20,30],"nested":{"nestedValue":"nestedValue","nestedNested":{"nestedNestedValue":"nestedNestedValue"}}}||`,
	},
	"properties-simple-pattern": {
		`send|properties-simple-pattern||SupportBean||{"theString":"test","intPrimitive":1}||`,
	},
	"properties-or-pattern": {
		`send|properties-or-pattern||SupportBeanComplexProps||{"simpleProperty":"simple","mapped":{"keyOne":"valueOne","keyTwo":"valueTwo"},"indexed":[1,2],"mapProperty":{"xOne":"yOne","xTwo":"yTwo"},"arrayProperty":[10,20,30],"nested":{"nestedValue":"nestedValue","nestedNested":{"nestedNestedValue":"nestedNestedValue"}}}||`,
		`send|properties-or-pattern||SupportBean||{"theString":"test2","intPrimitive":2}||`,
	},
}

// pepSupportBean mirrors SupportBean's asserted fields (theString,
// intPrimitive). theString is a *string so the default bean's null renders
// as {state:null} exactly like the Java bean.
type pepSupportBean struct {
	TheString    *string `esper:"theString" json:"theString"`
	IntPrimitive int     `esper:"intPrimitive" json:"intPrimitive"`
}

// pepComplexNestedNested mirrors SupportBeanSpecialGetterNestedNested.
type pepComplexNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue" json:"nestedNestedValue"`
}

// pepComplexNested mirrors SupportBeanSpecialGetterNested.
type pepComplexNested struct {
	NestedValue  string                 `esper:"nestedValue" json:"nestedValue"`
	NestedNested pepComplexNestedNested `esper:"nestedNested" json:"nestedNested"`
}

// pepComplexProps mirrors SupportBeanComplexProps.makeDefaultBean(): the six
// payload-pinned readable properties in PROPERTIES order (objectArray is
// omitted: makeDefaultBean leaves it null and the oracle renders event
// columns from the send payload). The json tags keep the nested struct's
// trace rendering on the Java property names.
type pepComplexProps struct {
	SimpleProperty string            `esper:"simpleProperty" json:"simpleProperty"`
	Mapped         map[string]string `esper:"mapped" json:"mapped"`
	Indexed        []int             `esper:"indexed" json:"indexed"`
	MapProperty    map[string]string `esper:"mapProperty" json:"mapProperty"`
	ArrayProperty  []int             `esper:"arrayProperty" json:"arrayProperty"`
	Nested         pepComplexNested  `esper:"nested" json:"nested"`
}

func runEPLOtherPatternEventPropertiesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplOtherPatternEventPropertiesCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEPLOtherPatternEventPropertiesCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplOtherPatternEventPropertiesID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eplOtherPatternEventPropertiesID, scenario.ID)
	}
	return trace, nil
}

// runEPLOtherPatternEventPropertiesCase replays one execution on a fresh
// environment/engine pair, mirroring the Java execution's fresh runtime:
// both bean types register up front, the pinned plan deploys with the s0
// listener attached (compileDeploy(epl).addListener("s0")), and the sends
// replay in order. undeployAll is mirrored by the engine teardown.
func runEPLOtherPatternEventPropertiesCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[pepSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[pepComplexProps](env, "SupportBeanComplexProps"); err != nil {
		return compat.Trace{}, err
	}
	plan, err := buildEPLOtherPatternEventPropertiesCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplOtherPatternEventPropertiesJavaRuntimeIDs[eplOtherPatternEventPropertiesOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	deployment, err := engine.Deploy(ctx, plan)
	if err != nil {
		return trace, err
	}
	sequences := make(map[string]uint64)
	for _, statement := range deployment.Statements() {
		if statement.Name() != "s0" {
			continue
		}
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			sequences["listener:"+stmt.Name()]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  sequences["listener:"+stmt.Name()],
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			return nil
		}); err != nil {
			return trace, err
		}
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeEPLOtherPatternEventPropertiesPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("%s: unsupported step op %q", eplOtherPatternEventPropertiesID, step.Op)
		}
	}
	return trace, nil
}

func eplOtherPatternEventPropertiesOrdinal(caseName string) int {
	for index, name := range eplOtherPatternEventPropertiesCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// buildEPLOtherPatternEventPropertiesCase returns the Go plan equivalent of
// the case's pinned EPL. select * over pattern tags expands to the pinned
// per-tag Alias columns; the property paths project through TagField for
// plain names and NestedField over the tagged PatternEvent for the
// indexed[0]/nested.nestedValue paths.
func buildEPLOtherPatternEventPropertiesCase(env *esper.Environment, caseName string) (esper.Plan, error) {
	bean := esper.From[pepSupportBean](env, "SupportBean")
	complex := esper.From[pepComplexProps](env, "SupportBeanComplexProps")
	switch caseName {
	case "wildcard-simple-pattern":
		// @name('s0') select * from pattern [a=SupportBean]
		return env.Build(esper.PatternFrom(bean, "a", esper.Literal(true)).
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
			).Query(esper.StatementName("s0")))
	case "wildcard-or-pattern":
		// @name('s0') select * from pattern [every(a=SupportBean or
		// b=SupportBeanComplexProps)] — the absent OR-branch tag projects
		// null (PatternEvent null-safe).
		return env.Build(esper.PatternFrom(bean, "a", esper.Literal(true)).
			Or(esper.PatternFrom(complex, "b", esper.Literal(true))).
			Every().
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
				esper.Alias("b", esper.PatternEvent("b")),
			).Query(esper.StatementName("s0")))
	case "properties-simple-pattern":
		// @name('s0') select a, a as myEvent, a.intPrimitive as myInt,
		// a.theString from pattern [a=SupportBean]
		return env.Build(esper.PatternFrom(bean, "a", esper.Literal(true)).
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
				esper.Alias("myEvent", esper.PatternEvent("a")),
				esper.Alias("myInt", esper.TagField[int]("a", "intPrimitive")),
				esper.Alias("a.theString", esper.TagField[*string]("a", "theString")),
			).Query(esper.StatementName("s0")))
	case "properties-or-pattern":
		// @name('s0') select a, a as myAEvent, b, b as myBEvent,
		// a.intPrimitive as myInt, a.theString, b.simpleProperty as simple,
		// b.indexed[0] as indexed, b.nested.nestedValue as nestedVal from
		// pattern [every(a=SupportBean or b=SupportBeanComplexProps)]
		return env.Build(esper.PatternFrom(bean, "a", esper.Literal(true)).
			Or(esper.PatternFrom(complex, "b", esper.Literal(true))).
			Every().
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
				esper.Alias("myAEvent", esper.PatternEvent("a")),
				esper.Alias("b", esper.PatternEvent("b")),
				esper.Alias("myBEvent", esper.PatternEvent("b")),
				esper.Alias("myInt", esper.TagField[int]("a", "intPrimitive")),
				esper.Alias("a.theString", esper.TagField[*string]("a", "theString")),
				esper.Alias("simple", esper.TagField[string]("b", "simpleProperty")),
				esper.Alias("indexed", esper.NestedField[int](esper.PatternEvent("b"), "indexed[0]")),
				esper.Alias("nestedVal", esper.NestedField[string](esper.PatternEvent("b"), "nested.nestedValue")),
			).Query(esper.StatementName("s0")))
	default:
		return esper.Plan{}, fmt.Errorf("unsupported %s case %q", eplOtherPatternEventPropertiesID, caseName)
	}
}

func decodeEPLOtherPatternEventPropertiesPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value pepSupportBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBeanComplexProps":
		var value pepComplexProps
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanComplexProps: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplOtherPatternEventPropertiesID, step.EventType)
	}
}

// loadEPLOtherPatternEventPropertiesScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadEPLOtherPatternEventPropertiesScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherPatternEventPropertiesID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherPatternEventPropertiesID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherPatternEventPropertiesID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherPatternEventPropertiesID, err)
	}
	if err := requireEPLOtherPatternEventPropertiesFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherPatternEventPropertiesID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherPatternEventPropertiesID ||
		metadata.Description != eplOtherPatternEventPropertiesDescription ||
		metadata.JavaCommit != eplOtherPatternEventPropertiesJavaCommit ||
		metadata.JavaSource != eplOtherPatternEventPropertiesSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplOtherPatternEventPropertiesID)
	}
	if err := validateEPLOtherPatternEventPropertiesStringArray(root["javaRuntimes"], eplOtherPatternEventPropertiesJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEPLOtherPatternEventPropertiesStringArray(root["javaNames"], eplOtherPatternEventPropertiesJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEPLOtherPatternEventPropertiesStringArray(root["javaStaticIds"], eplOtherPatternEventPropertiesJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEPLOtherPatternEventPropertiesStringArray(root["javaFlags"], eplOtherPatternEventPropertiesJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplOtherPatternEventPropertiesCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplOtherPatternEventPropertiesID, len(eplOtherPatternEventPropertiesCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEPLOtherPatternEventPropertiesFields(object,
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
		if definition.Case != eplOtherPatternEventPropertiesCases[index] ||
			definition.Ordinal != eplOtherPatternEventPropertiesOrdinals[index] ||
			definition.RuntimeID != eplOtherPatternEventPropertiesJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplOtherPatternEventPropertiesJavaExecutions[index] ||
			definition.Observation != eplOtherPatternEventPropertiesCaseObservations[index] ||
			definition.EPL != eplOtherPatternEventPropertiesCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplOtherPatternEventPropertiesID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eplOtherPatternEventPropertiesID, err)
	}
	offset := 0
	for _, caseName := range eplOtherPatternEventPropertiesCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eplOtherPatternEventPropertiesID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherPatternEventPropertiesID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eplOtherPatternEventPropertiesID, offset, caseName)
		}
		if _, err := eplOtherPatternEventPropertiesStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherPatternEventPropertiesID, offset, err)
		}
		offset++
		want, ok := eplOtherPatternEventPropertiesCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eplOtherPatternEventPropertiesID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eplOtherPatternEventPropertiesID, caseName)
		}
		for _, pinned := range want {
			key, err := eplOtherPatternEventPropertiesStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherPatternEventPropertiesID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eplOtherPatternEventPropertiesID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eplOtherPatternEventPropertiesID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherPatternEventPropertiesID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplOtherPatternEventPropertiesStepKey renders one raw step as its pinned
// key: op|case|statement|eventType|epl|payload|fields|listen with the
// payload compacted. Unknown fields on the step object are rejected; send
// payloads are restricted to the registered event type's asserted fields.
func eplOtherPatternEventPropertiesStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op        string          `json:"op"`
		Case      string          `json:"case"`
		Statement string          `json:"statement"`
		EventType string          `json:"eventType"`
		Epl       string          `json:"epl"`
		Listen    string          `json:"listen"`
		Fields    []string        `json:"fields"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case": {"op", "case"},
		"send": {"op", "case", "eventType", "payload"},
	}
	fields, ok := allowed[step.Op]
	if !ok {
		return "", fmt.Errorf("step has unsupported op %q", step.Op)
	}
	for field := range object {
		found := false
		for _, name := range fields {
			if field == name {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payloadText := ""
	if step.Op == "send" {
		payloadFields := map[string][]string{
			"SupportBean":             {"theString", "intPrimitive"},
			"SupportBeanComplexProps": {"simpleProperty", "mapped", "indexed", "mapProperty", "arrayProperty", "nested"},
		}
		allowedPayload, ok := payloadFields[step.EventType]
		if !ok {
			return "", fmt.Errorf("step has unknown event type %q", step.EventType)
		}
		var payload map[string]json.RawMessage
		if err := strictObject(step.Payload, &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		for field := range payload {
			found := false
			for _, name := range allowedPayload {
				if field == name {
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("send payload has unexpected field %q", field)
			}
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		payloadText = compacted.String()
	}
	return step.Op + "|" + step.Case + "|" + step.Statement + "|" + step.EventType +
		"|" + step.Epl + "|" + payloadText + "|" + strings.Join(step.Fields, ",") +
		"|" + step.Listen, nil
}

func requireEPLOtherPatternEventPropertiesFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eplOtherPatternEventPropertiesID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eplOtherPatternEventPropertiesID, name)
		}
	}
	return nil
}

func validateEPLOtherPatternEventPropertiesStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// normalizeEPLOtherPatternEventPropertiesTrace is the identity normalizer:
// every case deploys a single s0 statement and listener delivery is
// synchronous on the sending thread, so the dispatch order is already the
// canonical record order on both traces.
func normalizeEPLOtherPatternEventPropertiesTrace(trace compat.Trace) compat.Trace {
	return trace
}
