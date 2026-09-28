package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for PatternComplexPropertyAccess ords 0-2: complex
// property paths (mapped/indexed/array/nested/combined) inside pattern
// filter predicates over SupportBeanComplexProps/SupportBeanCombinedProps.
// Eighteen spellings replayed across the three Java executions.
//
// Covered executions (all variant collection:executions(), flags []):
//   - ord 0 PatternComplexProperties java-runtime-5fd2cb676f155061c987
//     (16 EventExpressionCase spellings: each deploys the Java
//     PatternTestHarness USE_EPL text
//     "select * from pattern [<EventExpressionCase atom>]" with the atom
//     verbatim; dataset e1 = SupportBeanComplexProps.makeDefaultBean,
//     e2 = SupportBeanCombinedProps.makeDefaultBean)
//   - ord 1 PatternIndexedFilterProp java-runtime-5d046d758dced3b2d879
//     (every-atom indexed filter capturing tag a; {3,4}->{a},{6}->silent,
//     {3}->{a})
//   - ord 2 PatternIndexedValueProp java-runtime-fd23ca72c8ba2aaf67ad
//     (every a -> b(indexed[0]=a.indexed[0]) tag-correlated followed-by;
//     {3} silent, {6} silent, {3} fires {a=eventOne,b=eventTwo})
//
// Excluded (same runtime expectations, JVM front-end precedent): ord 3
// PatternIndexedValuePropOM java-runtime-e0804e3e156cf0ee3656 (SODA object
// model + SerializableObjectCopier) and ord 4
// PatternIndexedValuePropCompile java-runtime-4364d60de0353a761ba1
// (eplToModelCompileDeploy). Both replay through the same fluent form in
// Go, mirroring the existing subtest loop in
// internal/esper/pattern_complexprops_parity_test.go; no separate scenario.
//
// Known representational notes:
//   - Java's assertSame event-identity assertions (ord 1 captured tag a,
//     ord 2 tags a/b) have no Go counterpart: Go events are values. The
//     trace pins the captured bean's payload projection instead, and the
//     ord 2 sends carry a simpleProperty marker (eventOne/eventTwo) to
//     distinguish the two {3} events exactly like the existing Go surface
//     test. The marker is payload data the pattern ignores.
//   - The Java oracle normalizes captured beans from their recorded send
//     payload (EPLOtherPatternEventProperties precedent): absent nullable
//     fields render {"state":"null"} and absent collection fields render
//     null, matching the Go schema rendering where *T/pointer fields map
//     to Null and nil maps/slices marshal as null.
//   - SupportBeanCombinedProps.getArray() aliases getIndexed(): both
//     payload fields carry the same four-element NestedLevOne array; the
//     trailing element is null on purpose and renders null (Go nil
//     pointer element).
//   - Java milestone() savepoints restore identical state for these
//     non-contextual executions and are omitted (573/574 precedent). The
//     PatternTestHarness USE_EPL style wraps each atom with
//     @Audit('pattern')/@Audit('pattern-instances'); audits are logging
//     only and carry no trace observable.

const patternComplexPropertyAccess575ID = "pattern-complex-property-access-575"

const patternComplexPropertyAccess575Description = "PatternComplexPropertyAccess ords 0-2: complex property access in pattern filters over SupportBeanComplexProps/SupportBeanCombinedProps — 16 EventExpressionCase spellings (mapped key, indexed, arrayProperty with an in-range check, nested and nested-nested navigation, and indexed[i].mapped(k).value combined chains including the wrong-value, missing-key, out-of-range-index and unknown-key no-fires), an every indexed[0]=3 filter capturing tag a, and the every-a->b(indexed[0]=a.indexed[0]) correlated followed-by with a single {eventOne,eventTwo} delivery."

const patternComplexPropertyAccess575JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternComplexPropertyAccess575JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternComplexPropertyAccess.java"

// Byte-exact EPL pins (PatternComplexPropertyAccess.java). Ord 0 deploys
// the Java PatternTestHarness USE_EPL text "select * from pattern [<atom>]"
// with each EventExpressionCase atom verbatim (spacing inside the atom is
// as written in the Java source); the name annotation is standardized to
// s0 like the ord 1/2 statements. Ords 1-2 are the verbatim
// env.compileDeploy texts.
const (
	patternComplexPropertyAccess575MappedKeyEPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(mapped('keyOne') = 'valueOne')]"

	patternComplexPropertyAccess575Indexed1Eq2EPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(indexed[1] = 2)]"

	patternComplexPropertyAccess575Indexed0Eq2EPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(indexed[0] = 2)]"

	patternComplexPropertyAccess575Array1Eq20EPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(arrayProperty[1] = 20)]"

	patternComplexPropertyAccess575Array1RangeEPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(arrayProperty[1] in (10:30))]"

	patternComplexPropertyAccess575Array2Eq20EPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(arrayProperty[2] = 20)]"

	patternComplexPropertyAccess575NestedValueEPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(nested.nestedValue = 'nestedValue')]"

	patternComplexPropertyAccess575NestedDummyEPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(nested.nestedValue = 'dummy')]"

	patternComplexPropertyAccess575NestedNestedEPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(nested.nestedNested.nestedNestedValue = 'nestedNestedValue')]"

	patternComplexPropertyAccess575NestedNestedXEPL = "@name('s0') select * from pattern [s=SupportBeanComplexProps(nested.nestedNested.nestedNestedValue = 'x')]"

	patternComplexPropertyAccess575CombinedIndexed1EPL = "@name('s0') select * from pattern [s=SupportBeanCombinedProps(indexed[1].mapped('1mb').value = '1ma1')]"

	patternComplexPropertyAccess575CombinedIndexed0EPL = "@name('s0') select * from pattern [s=SupportBeanCombinedProps(indexed[0].mapped('1ma').value = 'x')]"

	patternComplexPropertyAccess575CombinedArray0EPL = "@name('s0') select * from pattern [s=SupportBeanCombinedProps(array[0].mapped('0ma').value = '0ma0')]"

	patternComplexPropertyAccess575CombinedArray2EPL = "@name('s0') select * from pattern [s=SupportBeanCombinedProps(array[2].mapped('x').value = 'x')]"

	patternComplexPropertyAccess575CombinedArray879787EPL = "@name('s0') select * from pattern [s=SupportBeanCombinedProps(array[879787].mapped('x').value = 'x')]"

	patternComplexPropertyAccess575CombinedArrayXxxEPL = "@name('s0') select * from pattern [s=SupportBeanCombinedProps(array[0].mapped('xxx').value = 'x')]"

	patternComplexPropertyAccess575IndexedFilterPropEPL = "@name('s0') select * from pattern[every a=SupportBeanComplexProps(indexed[0]=3)]"

	patternComplexPropertyAccess575IndexedValuePropEPL = "@name('s0') select * from pattern[every a=SupportBeanComplexProps -> b=SupportBeanComplexProps(indexed[0] = a.indexed[0])]"
)

var (
	patternComplexPropertyAccess575JavaSources = []string{
		patternComplexPropertyAccess575JavaSource,
	}
	patternComplexPropertyAccess575JavaRuntimeIDs = []string{
		"java-runtime-5fd2cb676f155061c987",
		"java-runtime-5d046d758dced3b2d879",
		"java-runtime-fd23ca72c8ba2aaf67ad",
	}
	patternComplexPropertyAccess575JavaExecutions = []string{
		"PatternComplexProperties",
		"PatternIndexedFilterProp",
		"PatternIndexedValueProp",
	}
	patternComplexPropertyAccess575JavaStaticIDs = []string{
		"java-be2858d5e4c76bbf7f85",
		"java-ec97467eeb93b4ff0e67",
		"java-9504034cfc1c929363d8",
	}
)

// patternComplexPropertyAccess575CaseSpec pins one case: Java execution
// identity and the byte-exact EPL script the deploy step carries.
type patternComplexPropertyAccess575CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	epl          string
}

var patternComplexPropertyAccess575CaseSpecs = []patternComplexPropertyAccess575CaseSpec{
	{name: "mapped-key", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575MappedKeyEPL},
	{name: "indexed-1-eq-2", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575Indexed1Eq2EPL},
	{name: "indexed-0-eq-2-no-fire", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575Indexed0Eq2EPL},
	{name: "array-1-eq-20", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575Array1Eq20EPL},
	{name: "array-1-in-range", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575Array1RangeEPL},
	{name: "array-2-eq-20-no-fire", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575Array2Eq20EPL},
	{name: "nested-value", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575NestedValueEPL},
	{name: "nested-value-no-fire", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575NestedDummyEPL},
	{name: "nested-nested-value", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575NestedNestedEPL},
	{name: "nested-nested-value-no-fire", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575NestedNestedXEPL},
	{name: "combined-indexed-mapped", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575CombinedIndexed1EPL},
	{name: "combined-indexed-mapped-no-fire", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575CombinedIndexed0EPL},
	{name: "combined-array-mapped", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575CombinedArray0EPL},
	{name: "combined-array-mapped-missing-key", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575CombinedArray2EPL},
	{name: "combined-array-out-of-range", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575CombinedArray879787EPL},
	{name: "combined-array-unknown-key", ordinal: 0, runtimeIndex: 0, epl: patternComplexPropertyAccess575CombinedArrayXxxEPL},
	{name: "indexed-filter-prop", ordinal: 1, runtimeIndex: 1, epl: patternComplexPropertyAccess575IndexedFilterPropEPL},
	{name: "indexed-value-prop", ordinal: 2, runtimeIndex: 2, epl: patternComplexPropertyAccess575IndexedValuePropEPL},
}

// Host objects: SupportBeanComplexProps/SupportBeanCombinedProps mirror
// the Java beans' payload-pinned properties (the pepComplexProps shape).
// simpleProperty and nested are pointers so absent fields render
// {"state":"null"} like Java nulls; nil maps/slices render null, matching
// the oracle's payload projection for the accessor properties that have
// no enumerable getters.
type patternComplexPropertyAccess575NestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue" json:"nestedNestedValue"`
}

type patternComplexPropertyAccess575Nested struct {
	NestedValue  string                                      `esper:"nestedValue" json:"nestedValue"`
	NestedNested patternComplexPropertyAccess575NestedNested `esper:"nestedNested" json:"nestedNested"`
}

type patternComplexPropertyAccess575ComplexProps struct {
	SimpleProperty *string                                `esper:"simpleProperty" json:"simpleProperty"`
	Mapped         map[string]string                      `esper:"mapped" json:"mapped"`
	Indexed        []int                                  `esper:"indexed" json:"indexed"`
	MapProperty    map[string]string                      `esper:"mapProperty" json:"mapProperty"`
	ArrayProperty  []int                                  `esper:"arrayProperty" json:"arrayProperty"`
	Nested         *patternComplexPropertyAccess575Nested `esper:"nested" json:"nested"`
}

// patternComplexPropertyAccess575CombinedNestedTwo mirrors NestedLevTwo
// and ...CombinedNestedOne mirrors NestedLevOne; elements are pointers so
// the intentionally null trailing element renders null like Java.
type patternComplexPropertyAccess575CombinedNestedTwo struct {
	Value string `esper:"value" json:"value"`
}

type patternComplexPropertyAccess575CombinedNestedOne struct {
	Mapped map[string]patternComplexPropertyAccess575CombinedNestedTwo `esper:"mapped" json:"mapped"`
}

type patternComplexPropertyAccess575CombinedProps struct {
	Indexed []*patternComplexPropertyAccess575CombinedNestedOne `esper:"indexed" json:"indexed"`
	Array   []*patternComplexPropertyAccess575CombinedNestedOne `esper:"array" json:"array"`
}

// patternComplexPropertyAccess575SendPin pins one send step's event type
// and exact payload: the compacted JSON bytes as authored in the
// scenario.
type patternComplexPropertyAccess575SendPin struct {
	event   string
	payload string
}

// Send payload pins. e1/e2 are the makeDefaultBean shapes used by every
// ord 0 case; ord 1/2 send indexed-only probes (ord 2 carries the
// simpleProperty marker replacing Java's assertSame identity).
const (
	patternComplexPropertyAccess575ComplexPayload = `{"simpleProperty":"simple","mapped":{"keyOne":"valueOne","keyTwo":"valueTwo"},"indexed":[1,2],"mapProperty":{"xOne":"yOne","xTwo":"yTwo"},"arrayProperty":[10,20,30],"nested":{"nestedValue":"nestedValue","nestedNested":{"nestedNestedValue":"nestedNestedValue"}}}`

	patternComplexPropertyAccess575CombinedPayload = `{"indexed":[{"mapped":{"0ma":{"value":"0ma0"},"0mb":{"value":"0ma1"}}},{"mapped":{"1ma":{"value":"1ma0"},"1mb":{"value":"1ma1"}}},{"mapped":{"2ma":{"value":"valueOne"},"2mb":{"value":"2ma1"}}},null],"array":[{"mapped":{"0ma":{"value":"0ma0"},"0mb":{"value":"0ma1"}}},{"mapped":{"1ma":{"value":"1ma0"},"1mb":{"value":"1ma1"}}},{"mapped":{"2ma":{"value":"valueOne"},"2mb":{"value":"2ma1"}}},null]}`
)

func patternComplexPropertyAccess575Send(eventType, payload string) patternComplexPropertyAccess575SendPin {
	return patternComplexPropertyAccess575SendPin{event: eventType, payload: payload}
}

// patternComplexPropertyAccess575SendSequences pins the send legs per
// case in Java source order: ord 0 sends e1 (complex default) then e2
// (combined default) per PatternTestHarness; ord 1 sends the {3,4}/{6}/{3}
// probes; ord 2 sends eventOne{3}/other{6}/eventTwo{3}.
func patternComplexPropertyAccess575SendSequences(caseName string) []patternComplexPropertyAccess575SendPin {
	switch caseName {
	case "indexed-filter-prop":
		return []patternComplexPropertyAccess575SendPin{
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", `{"indexed":[3,4]}`),
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", `{"indexed":[6]}`),
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", `{"indexed":[3]}`),
		}
	case "indexed-value-prop":
		return []patternComplexPropertyAccess575SendPin{
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", `{"simpleProperty":"eventOne","indexed":[3]}`),
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", `{"indexed":[6]}`),
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", `{"simpleProperty":"eventTwo","indexed":[3]}`),
		}
	default:
		// getSetSixComplexProperties: e1 = complex default, e2 = combined default.
		return []patternComplexPropertyAccess575SendPin{
			patternComplexPropertyAccess575Send("SupportBeanComplexProps", patternComplexPropertyAccess575ComplexPayload),
			patternComplexPropertyAccess575Send("SupportBeanCombinedProps", patternComplexPropertyAccess575CombinedPayload),
		}
	}
}

func loadPatternComplexPropertyAccess575Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternComplexPropertyAccess575ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternComplexPropertyAccess575ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternComplexPropertyAccess575ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternComplexPropertyAccess575ID, err)
	}
	if err := requirePatternComplexPropertyAccess575Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternComplexPropertyAccess575ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternComplexPropertyAccess575ID ||
		metadata.Description != patternComplexPropertyAccess575Description ||
		metadata.JavaCommit != patternComplexPropertyAccess575JavaCommit ||
		metadata.JavaSource != patternComplexPropertyAccess575JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", patternComplexPropertyAccess575ID)
	}
	if err := validatePatternComplexPropertyAccess575StringArray(root["javaRuntimes"], patternComplexPropertyAccess575JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validatePatternComplexPropertyAccess575StringArray(root["javaNames"], patternComplexPropertyAccess575JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validatePatternComplexPropertyAccess575StringArray(root["javaStaticIds"], patternComplexPropertyAccess575JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validatePatternComplexPropertyAccess575StringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternComplexPropertyAccess575CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", patternComplexPropertyAccess575ID, len(patternComplexPropertyAccess575CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternComplexPropertyAccess575Fields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := patternComplexPropertyAccess575CaseSpecs[index]
		if definition.Case != spec.name ||
			definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternComplexPropertyAccess575JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternComplexPropertyAccess575JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternComplexPropertyAccess575ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternComplexPropertyAccess575ID)
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
			if err := requirePatternComplexPropertyAccess575Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requirePatternComplexPropertyAccess575Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requirePatternComplexPropertyAccess575Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodePatternComplexPropertyAccess575Payload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requirePatternComplexPropertyAccess575Fields(object, "op", "case"); err != nil {
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
	if err := validatePatternComplexPropertyAccess575Scenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validatePatternComplexPropertyAccess575Scenario positionally compares
// every step against the pinned per-case sequences: case marker, one
// deploy of statement s0 with the byte-exact EPL script, the pinned sends,
// undeploy.
func validatePatternComplexPropertyAccess575Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternComplexPropertyAccess575ID {
		return fmt.Errorf("%s scenario shape is not pinned", patternComplexPropertyAccess575ID)
	}
	offset := 0
	for _, spec := range patternComplexPropertyAccess575CaseSpecs {
		sends := patternComplexPropertyAccess575SendSequences(spec.name)
		expected := 1 + len(sends) + 1 // deploy + sends + undeploy-all
		steps := scenario.Steps[offset:]
		if len(steps) < expected+1 {
			return fmt.Errorf("%s case %q steps are truncated", patternComplexPropertyAccess575ID, spec.name)
		}
		if steps[0].Op != "case" || steps[0].Case != spec.name {
			return fmt.Errorf("%s case %q must start with its case marker", patternComplexPropertyAccess575ID, spec.name)
		}
		deploy := steps[1]
		if deploy.Op != "deploy" || deploy.Case != spec.name || deploy.Statement != "s0" || deploy.Epl != spec.epl {
			return fmt.Errorf("%s case %q must deploy s0 with the pinned EPL", patternComplexPropertyAccess575ID, spec.name)
		}
		for index, send := range sends {
			step := steps[index+2]
			if step.Op != "send" || step.Case != spec.name || step.EventType != send.event {
				return fmt.Errorf("%s case %q step %d must send %s", patternComplexPropertyAccess575ID, spec.name, index, send.event)
			}
			if err := patternComplexPropertyAccess575ValidatePayload(step, send); err != nil {
				return fmt.Errorf("%s case %q step %d: %w", patternComplexPropertyAccess575ID, spec.name, index, err)
			}
		}
		undeploy := steps[len(sends)+2]
		if undeploy.Op != "undeploy-all" || undeploy.Case != spec.name {
			return fmt.Errorf("%s case %q must end with undeploy-all", patternComplexPropertyAccess575ID, spec.name)
		}
		offset += expected + 1
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", patternComplexPropertyAccess575ID)
	}
	return nil
}

// patternComplexPropertyAccess575ValidatePayload pins the exact send
// payload bytes (compacted, in authored member order).
func patternComplexPropertyAccess575ValidatePayload(step compat.Step, send patternComplexPropertyAccess575SendPin) error {
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, step.Payload); err != nil {
		return fmt.Errorf("send payload: %w", err)
	}
	if compacted.String() != send.payload {
		return fmt.Errorf("send payload is not pinned")
	}
	return nil
}

func runPatternComplexPropertyAccess575Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternComplexPropertyAccess575Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for _, spec := range patternComplexPropertyAccess575CaseSpecs {
		length := 1 + 1 + len(patternComplexPropertyAccess575SendSequences(spec.name)) + 1
		caseSteps := scenario.Steps[offset : offset+length]
		offset += length
		caseTrace, err := runPatternComplexPropertyAccess575Case(ctx, caseSteps, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternComplexPropertyAccess575ID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runPatternComplexPropertyAccess575Case(ctx context.Context, steps []compat.Step, spec patternComplexPropertyAccess575CaseSpec) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternComplexPropertyAccess575ComplexProps](env, "SupportBeanComplexProps"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[patternComplexPropertyAccess575CombinedProps](env, "SupportBeanCombinedProps"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(patternComplexPropertyAccess575JavaRuntimeIDs[spec.runtimeIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := uint64(0)
	trace := compat.Trace{Version: "esper-parity/v1", ID: patternComplexPropertyAccess575ID}
	record := func(batch esper.ResultBatch) {
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plan, err := patternComplexPropertyAccess575Plan(env, spec)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			statements := deployment.Statements()
			if len(statements) != 1 || statements[0].Name() != "s0" {
				return compat.Trace{}, fmt.Errorf("expected one s0 statement")
			}
			if _, err := statements[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				record(batch)
				return nil
			}); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			payload, err := decodePatternComplexPropertyAccess575Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
		}
	}
	return trace, nil
}

// patternComplexPropertyAccess575Plan builds the Go plan equivalent of
// the case's pinned EPL. select * over the pattern expands to the pinned
// per-tag Alias(PatternEvent) columns; each ord 0 atom maps to the
// composed MapValue/ArrayAt/Property/BetweenOf predicate of the existing
// Go surface (internal/esper/pattern_complexprops_parity_test.go).
func patternComplexPropertyAccess575Plan(env *esper.Environment, spec patternComplexPropertyAccess575CaseSpec) (esper.Plan, error) {
	complex := esper.From[patternComplexPropertyAccess575ComplexProps](env, "SupportBeanComplexProps")
	combined := esper.From[patternComplexPropertyAccess575CombinedProps](env, "SupportBeanCombinedProps")

	mappedValue := func(key string) esper.Expression[string] {
		return esper.MapValue[string](esper.Field[patternComplexPropertyAccess575ComplexProps, map[string]string]("mapped"), esper.Literal(key))
	}
	indexedValue := func(index int) esper.Expression[int] {
		return esper.ArrayAt[int](esper.Field[patternComplexPropertyAccess575ComplexProps, []int]("indexed"), esper.Literal(index))
	}
	arrayPropertyValue := func(index int) esper.Expression[int] {
		return esper.ArrayAt[int](esper.Field[patternComplexPropertyAccess575ComplexProps, []int]("arrayProperty"), esper.Literal(index))
	}
	nestedValue := func() esper.Expression[string] {
		return esper.Property[string](esper.Field[patternComplexPropertyAccess575ComplexProps, *patternComplexPropertyAccess575Nested]("nested"), "nestedValue")
	}
	nestedNestedValue := func() esper.Expression[string] {
		return esper.Property[string](
			esper.Property[*patternComplexPropertyAccess575NestedNested](
				esper.Field[patternComplexPropertyAccess575ComplexProps, *patternComplexPropertyAccess575Nested]("nested"),
				"nestedNested"),
			"nestedNestedValue")
	}
	// cmbMappedProperty renders indexed[i].mapped('k').value /
	// array[i].mapped('k').value: ArrayAt -> Property(mapped) ->
	// MapValue(k) -> Property(value). Out-of-range indexes, missing keys
	// and null elements all evaluate to Null so the equality does not
	// fire, matching Esper's safe getters.
	cmbMappedProperty := func(field string, index int, key string) esper.Expression[string] {
		return esper.Property[string](
			esper.MapValue[patternComplexPropertyAccess575CombinedNestedTwo](
				esper.Property[map[string]patternComplexPropertyAccess575CombinedNestedTwo](
					esper.ArrayAt[*patternComplexPropertyAccess575CombinedNestedOne](
						esper.Field[patternComplexPropertyAccess575CombinedProps, []*patternComplexPropertyAccess575CombinedNestedOne](field),
						esper.Literal(index)),
					"mapped"),
				esper.Literal(key)),
			"value")
	}
	selectS := func(stream esper.PatternStream) esper.PatternQuery {
		return stream.Select(esper.Alias("s", esper.PatternEvent("s")))
	}

	var query esper.Query
	switch spec.name {
	case "mapped-key":
		// s=SupportBeanComplexProps(mapped('keyOne') = 'valueOne')
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[string](mappedValue("keyOne"), esper.Literal("valueOne")))).
			Query(esper.StatementName("s0"))
	case "indexed-1-eq-2":
		// s=SupportBeanComplexProps(indexed[1] = 2)
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[int](indexedValue(1), esper.Literal(2)))).
			Query(esper.StatementName("s0"))
	case "indexed-0-eq-2-no-fire":
		// s=SupportBeanComplexProps(indexed[0] = 2) — indexed[0] is 1.
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[int](indexedValue(0), esper.Literal(2)))).
			Query(esper.StatementName("s0"))
	case "array-1-eq-20":
		// s=SupportBeanComplexProps(arrayProperty[1] = 20)
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[int](arrayPropertyValue(1), esper.Literal(20)))).
			Query(esper.StatementName("s0"))
	case "array-1-in-range":
		// s=SupportBeanComplexProps(arrayProperty[1] in (10:30))
		query = selectS(esper.PatternFrom(complex, "s",
			esper.BetweenOf(arrayPropertyValue(1), esper.Literal(10), esper.Literal(30)))).
			Query(esper.StatementName("s0"))
	case "array-2-eq-20-no-fire":
		// s=SupportBeanComplexProps(arrayProperty[2] = 20) — value is 30.
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[int](arrayPropertyValue(2), esper.Literal(20)))).
			Query(esper.StatementName("s0"))
	case "nested-value":
		// s=SupportBeanComplexProps(nested.nestedValue = 'nestedValue')
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[string](nestedValue(), esper.Literal("nestedValue")))).
			Query(esper.StatementName("s0"))
	case "nested-value-no-fire":
		// s=SupportBeanComplexProps(nested.nestedValue = 'dummy')
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[string](nestedValue(), esper.Literal("dummy")))).
			Query(esper.StatementName("s0"))
	case "nested-nested-value":
		// s=SupportBeanComplexProps(nested.nestedNested.nestedNestedValue =
		// 'nestedNestedValue')
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[string](nestedNestedValue(), esper.Literal("nestedNestedValue")))).
			Query(esper.StatementName("s0"))
	case "nested-nested-value-no-fire":
		// s=SupportBeanComplexProps(nested.nestedNested.nestedNestedValue =
		// 'x')
		query = selectS(esper.PatternFrom(complex, "s",
			esper.Equal[string](nestedNestedValue(), esper.Literal("x")))).
			Query(esper.StatementName("s0"))
	case "combined-indexed-mapped":
		// s=SupportBeanCombinedProps(indexed[1].mapped('1mb').value = '1ma1')
		query = selectS(esper.PatternFrom(combined, "s",
			esper.Equal[string](cmbMappedProperty("indexed", 1, "1mb"), esper.Literal("1ma1")))).
			Query(esper.StatementName("s0"))
	case "combined-indexed-mapped-no-fire":
		// s=SupportBeanCombinedProps(indexed[0].mapped('1ma').value = 'x') —
		// the value is 0ma0.
		query = selectS(esper.PatternFrom(combined, "s",
			esper.Equal[string](cmbMappedProperty("indexed", 0, "1ma"), esper.Literal("x")))).
			Query(esper.StatementName("s0"))
	case "combined-array-mapped":
		// s=SupportBeanCombinedProps(array[0].mapped('0ma').value = '0ma0')
		query = selectS(esper.PatternFrom(combined, "s",
			esper.Equal[string](cmbMappedProperty("array", 0, "0ma"), esper.Literal("0ma0")))).
			Query(esper.StatementName("s0"))
	case "combined-array-mapped-missing-key":
		// s=SupportBeanCombinedProps(array[2].mapped('x').value = 'x') —
		// key x is absent from element 2's map.
		query = selectS(esper.PatternFrom(combined, "s",
			esper.Equal[string](cmbMappedProperty("array", 2, "x"), esper.Literal("x")))).
			Query(esper.StatementName("s0"))
	case "combined-array-out-of-range":
		// s=SupportBeanCombinedProps(array[879787].mapped('x').value = 'x') —
		// the index is out of range.
		query = selectS(esper.PatternFrom(combined, "s",
			esper.Equal[string](cmbMappedProperty("array", 879787, "x"), esper.Literal("x")))).
			Query(esper.StatementName("s0"))
	case "combined-array-unknown-key":
		// s=SupportBeanCombinedProps(array[0].mapped('xxx').value = 'x') —
		// key xxx is absent from element 0's map.
		query = selectS(esper.PatternFrom(combined, "s",
			esper.Equal[string](cmbMappedProperty("array", 0, "xxx"), esper.Literal("x")))).
			Query(esper.StatementName("s0"))
	case "indexed-filter-prop":
		// @name('s0') select * from pattern[every
		// a=SupportBeanComplexProps(indexed[0]=3)]
		query = esper.PatternFrom(complex, "a",
			esper.Equal[int](indexedValue(0), esper.Literal(3))).
			Every().
			Select(esper.Alias("a", esper.PatternEvent("a"))).
			Query(esper.StatementName("s0"))
	case "indexed-value-prop":
		// @name('s0') select * from pattern[every a=SupportBeanComplexProps
		// -> b=SupportBeanComplexProps(indexed[0] = a.indexed[0])]:
		// TagField("a","indexed") resolves the bound earlier limb's indexed
		// property at b-evaluation time.
		query = esper.PatternFrom(complex, "a", esper.Literal(true)).Every().
			Then(esper.PatternFrom(complex, "b",
				esper.Equal[int](
					indexedValue(0),
					esper.ArrayAt[int](esper.TagField[[]int]("a", "indexed"), esper.Literal(0))))).
			Select(
				esper.Alias("a", esper.PatternEvent("a")),
				esper.Alias("b", esper.PatternEvent("b")),
			).
			Query(esper.StatementName("s0"))
	default:
		return esper.Plan{}, fmt.Errorf("unknown %s case %q", patternComplexPropertyAccess575ID, spec.name)
	}
	return env.Build(query)
}

func decodePatternComplexPropertyAccess575Payload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBeanComplexProps":
		var value patternComplexPropertyAccess575ComplexProps
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanComplexProps: %w", err)
		}
		return value, nil
	case "SupportBeanCombinedProps":
		var value patternComplexPropertyAccess575CombinedProps
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBeanCombinedProps: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternComplexPropertyAccess575ID, step.EventType)
	}
}

func requirePatternComplexPropertyAccess575Fields(object map[string]json.RawMessage, names ...string) error {
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

func validatePatternComplexPropertyAccess575StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
