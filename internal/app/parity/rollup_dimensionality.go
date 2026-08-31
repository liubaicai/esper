package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type rollupDimensionalityBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	ShortPrimitive  int16   `esper:"shortPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	IntBoxed        *int    `esper:"intBoxed"`
}

type rollupDimensionalityS0 struct {
	ID int `esper:"id"`
}

var rollupDimensionalityJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRollupDimensionality.java",
}

var (
	rollupDimensionalityJavaRuntimeIDs = []string{
		"java-runtime-30499c2e4ff9aece48b2",
		"java-runtime-b059890b735f776a9e03",
		"java-runtime-e60ea25dc87dcfbdcc08",
		"java-runtime-f5da6be14e939f2b26cc",
		"java-runtime-b06640d26b3b63075791",
		"java-runtime-14c4aecdca8b446299f1",
		"java-runtime-3b6467afa76b0966c475",
		"java-runtime-274b66386ba625a8b24c",
		"java-runtime-c3795d43550db4779a8c",
		"java-runtime-19844b30add2415c2bcc",
		"java-runtime-effa54ebac66e75bdcb6",
		"java-runtime-3f406a35c51cd03ab734",
		"java-runtime-e17c22ce9356d4acfad9",
		"java-runtime-58abe8e5ebfa57510a04",
		"java-runtime-8178e315c9b40968a218",
		"java-runtime-153edf6a4d78354167f1",
		"java-runtime-23faa930e8e9cbc671c2",
		"java-runtime-a4a78ec3230ca521cf03",
		"java-runtime-0a3b5f198b8a6467078a",
		"java-runtime-3f45c2d30f96ffe3c19f"}
	rollupDimensionalityJavaExecutions = []string{
		"ResultSetQueryTypeUnboundRollup2Dim",
		"ResultSetQueryTypeUnboundRollup1Dim",
		"ResultSetQueryTypeUnboundRollupUnenclosed",
		"ResultSetQueryTypeUnboundRollup3Dim",
		"ResultSetQueryTypeUnboundCubeUnenclosed",
		"ResultSetQueryTypeUnboundCube4Dim",
		"ResultSetQueryTypeBoundRollup2Dim",
		"ResultSetQueryTypeUnboundRollup2DimBatchWindow",
		"ResultSetQueryTypeRollupMultikeyWArray{join=false, unbound=true}",
		"ResultSetQueryTypeRollupMultikeyWArray{join=false, unbound=false}",
		"ResultSetQueryTypeRollupMultikeyWArray{join=true, unbound=false}",
		"ResultSetQueryTypeRollupMultikeyWArrayGroupingSet",
		"ResultSetQueryTypeNamedWindowCube2Dim",
		"ResultSetQueryTypeOnSelect",
		"ResultSetQueryTypeOutputWhenTerminated",
		"ResultSetQueryTypeBoundGroupingSet2LevelNoTopNoDetail",
		"ResultSetQueryTypeBoundGroupingSet2LevelTopAndDetail",
		"ResultSetQueryTypeMixedAccessAggregation",
		"ResultSetQueryTypeNonBoxedTypeWithRollup",
		"ResultSetQueryTypeGroupByWithComputation"}
)

const (
	rollupDimensionalityDedicatedID          = "rollup-dimensionality-dedicated"
	rollupDimensionalityDedicatedDescription = "Dedicated ResultSetQueryTypeRollupDimensionality ordinals 10, 11, and 17: unbound grouping-set, bounded cube, and context-partition rollup semantics."
)

var rollupDimensionalityDedicatedJavaRuntimeIDs = []string{
	"java-runtime-20c08346e2644a7201e5",
	"java-runtime-994120aef6b9ff1c0e75",
	"java-runtime-71fb38d67af471287089",
}

var rollupDimensionalityDedicatedJavaExecutions = []string{
	"ResultSetQueryTypeUnboundGroupingSet2LevelUnenclosed",
	"ResultSetQueryTypeBoundCube3Dim",
	"ResultSetQueryTypeContextPartitionAlsoRollup",
}

var rollupDimensionalityDedicatedJavaStaticIDs = []string{
	"java-04090eecaf8dbe129c67",
	"java-2bb53dfa0bac679a3c29",
	"java-e684c2e0c509d0592a0d",
}

var rollupDimensionalityDedicatedCases = []string{
	"unbound-grouping-set-2level-unenclosed-a",
	"unbound-grouping-set-2level-unenclosed-b",
	"bound-cube-3dim-cube",
	"bound-cube-3dim-gs",
	"context-partition-also-rollup",
}

var rollupDimensionalityDedicatedCaseRuntimes = []string{
	"java-runtime-20c08346e2644a7201e5",
	"java-runtime-20c08346e2644a7201e5",
	"java-runtime-994120aef6b9ff1c0e75",
	"java-runtime-994120aef6b9ff1c0e75",
	"java-runtime-71fb38d67af471287089",
}

var rollupDimensionalityDedicatedCaseOrdinals = []int{10, 10, 11, 11, 17}

var rollupDimensionalityDedicatedCaseEPL = []string{
	"@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by theString, grouping sets(intPrimitive, longPrimitive)",
	"@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, sum(doublePrimitive) as c3 from SupportBean group by grouping sets((theString, intPrimitive), (theString, longPrimitive))",
	"@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4,grouping(theString) as c5,grouping(intPrimitive) as c6,grouping(longPrimitive) as c7,grouping_id(theString, intPrimitive, longPrimitive) as c8 from SupportBean#length(4) group by cube(theString, intPrimitive, longPrimitive)",
	"@Name('s0')select theString as c0, intPrimitive as c1, longPrimitive as c2, count(*) as c3, sum(doublePrimitive) as c4,grouping(theString) as c5,grouping(intPrimitive) as c6,grouping(longPrimitive) as c7,grouping_id(theString, intPrimitive, longPrimitive) as c8 from SupportBean#length(4) group by grouping sets((theString, intPrimitive, longPrimitive),(theString, intPrimitive),(theString, longPrimitive),(theString),(intPrimitive, longPrimitive),(intPrimitive),(longPrimitive),())",
	"create context SegmentedByString partition by theString from SupportBean;\n@name('s0') context SegmentedByString select theString as c0, intPrimitive as c1, sum(longPrimitive) as c2 from SupportBean group by rollup(theString, intPrimitive)",
}

// runRollupDimensionalityScenario replays the unbound rollup, cube and
// bound/batch family of ResultSetQueryTypeRollupDimensionality (8 executions
// across 16 scenario cases; the 1-dim rollup/cube pair, the three unenclosed
func loadRollupDimensionalityScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("rollup-dimensionality scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read rollup-dimensionality scenario: %w", err)
	}
	var envelope struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-dimensionality scenario: %w", err)
	}
	if envelope.ID == rollupDimensionalityDedicatedID {
		return loadRollupDimensionalityDedicatedScenario(bytes.NewReader(raw))
	}
	return compat.LoadScenario(bytes.NewReader(raw))
}

func loadRollupDimensionalityDedicatedScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read rollup-dimensionality dedicated scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-dimensionality dedicated scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode rollup-dimensionality dedicated scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario is missing field %q", name)
		}
	}
	var version, id, description, javaCommit, javaSource string
	for name, target := range map[string]*string{
		"version": &version, "id": &id, "description": &description,
		"javaCommit": &javaCommit, "javaSource": &javaSource,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario %s must be a string", name)
		}
	}
	if version != compat.ScenarioVersion || id != rollupDimensionalityDedicatedID ||
		description != rollupDimensionalityDedicatedDescription ||
		javaCommit != "9e1b9f1cc9117fea4bf33ab043762c045d73839c" ||
		javaSource != rollupDimensionalityJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario metadata is not pinned")
	}
	for name, expected := range map[string][]string{
		"javaRuntimes":  rollupDimensionalityDedicatedJavaRuntimeIDs,
		"javaNames":     rollupDimensionalityDedicatedJavaExecutions,
		"javaStaticIds": rollupDimensionalityDedicatedJavaStaticIDs,
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[name], expected, name); err != nil {
			return compat.Scenario{}, err
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(rollupDimensionalityDedicatedCases) {
		return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario must contain exactly five cases")
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated case %d: %w", index, err)
		}
		requiredCase := []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"}
		if len(object) != len(requiredCase) {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated case %d contains unexpected or missing fields", index)
		}
		for _, name := range requiredCase {
			if _, ok := object[name]; !ok {
				return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated case %d is missing field %q", index, name)
			}
		}
		var caseName, runtimeID, executionName, observation, epl string
		var ordinal, iteratorSnapshots int
		if err := json.Unmarshal(object["case"], &caseName); err != nil ||
			json.Unmarshal(object["runtimeId"], &runtimeID) != nil ||
			json.Unmarshal(object["executionName"], &executionName) != nil ||
			json.Unmarshal(object["observation"], &observation) != nil ||
			json.Unmarshal(object["epl"], &epl) != nil ||
			json.Unmarshal(object["ordinal"], &ordinal) != nil ||
			json.Unmarshal(object["iteratorSnapshots"], &iteratorSnapshots) != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated case %d metadata has invalid types", index)
		}
		if caseName != rollupDimensionalityDedicatedCases[index] ||
			ordinal != rollupDimensionalityDedicatedCaseOrdinals[index] ||
			runtimeID != rollupDimensionalityDedicatedCaseRuntimes[index] ||
			executionName != rollupDimensionalityDedicatedJavaExecutions[index/2] ||
			observation != "listener" || iteratorSnapshots != 0 ||
			epl != rollupDimensionalityDedicatedCaseEPL[index] {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated case %d metadata is not pinned", index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != 26 {
		return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated scenario must contain exactly 26 steps")
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d: %w", index, err)
		}
		if index == 0 || index == 5 || index == 10 || index == 16 || index == 22 {
			expectedCase := rollupDimensionalityDedicatedCases[index/5]
			if len(object) != 2 || string(bytes.TrimSpace(object["op"])) != `"case"` || string(bytes.TrimSpace(object["case"])) != fmt.Sprintf(`%q`, expectedCase) {
				return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d marker fields are not pinned", index)
			}
		} else if len(object) != 3 {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d send fields are not pinned", index)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d: %w", index, err)
		}
		if steps[index].Op == "send" {
			var payload map[string]json.RawMessage
			if err := strictObject(steps[index].Payload, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d payload: %w", index, err)
			}
			expectedPayloadFields := 4
			if index >= 22 {
				expectedPayloadFields = 3
			}
			if len(payload) != expectedPayloadFields {
				return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d payload fields are not pinned", index)
			}
			for _, field := range []string{"theString", "intPrimitive", "longPrimitive"} {
				if _, ok := payload[field]; !ok {
					return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d payload is missing %q", index, field)
				}
			}
			if expectedPayloadFields == 4 {
				if _, ok := payload["doublePrimitive"]; !ok {
					return compat.Scenario{}, fmt.Errorf("rollup-dimensionality dedicated step %d payload is missing doublePrimitive", index)
				}
			}
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateRollupDimensionalityDedicatedScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// syntax variants, the two 3-dim pairs, the cube-unenclosed trio, the 4-dim
// cube, and the bound/batch pair each replay one shared sequence):
// hierarchical detail-to-overall rows, null-padded aggregated key columns,
// monotonic accumulation on unbounded and windowed streams, rollup/cube/
// grouping-sets syntax equivalence, cube bitmask row ordering, and batch
// flush new/old IR pairs.
// runRollupDimensionalityScenario replays the unbound rollup, cube and
// bound/batch family of ResultSetQueryTypeRollupDimensionality. The dedicated
// scenario is kept separate so its exact three-execution metadata and trace
// contract cannot be confused with the broad umbrella scenario.
func runRollupDimensionalityScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if scenario.ID == rollupDimensionalityDedicatedID {
		return runRollupDimensionalityDedicatedScenario(ctx, scenario)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"unbound-rollup-2dim",
		"unbound-rollup-1dim-rollup", "unbound-rollup-1dim-cube",
		"unbound-rollup-unenclosed-a", "unbound-rollup-unenclosed-b", "unbound-rollup-unenclosed-c",
		"unbound-rollup-3dim-rollup", "unbound-rollup-3dim-gs",
		"unbound-rollup-3dim-rollup-join", "unbound-rollup-3dim-gs-join",
		"unbound-cube-unenclosed-a", "unbound-cube-unenclosed-b", "unbound-cube-unenclosed-c",
		"unbound-cube-4dim",
		"bound-rollup", "unbound-rollup-2dim-batch",
		// Draft 4.239 completion: the 12 remaining executions. nw-cube-gs
		// replays the named-window cube's grouping-sets spelling (byte-equal
		// to the cube form); the five out-when-term variants share one
		// runtime ID, one per output-limit/hint combination.
		"warray-unbound", "warray-bound", "warray-join", "warray-gs",
		"nw-cube", "nw-cube-gs",
		"onselect-rollup",
		"out-when-term-last", "out-when-term-last-opt", "out-when-term-last-optdis",
		"out-when-term-all", "out-when-term-snapshot",
		"bound-gs-no-top", "bound-gs-top-detail",
		"mixed-access", "non-boxed-types", "groupby-computation",
	}
	if !scenarioHasCase(scenario, caseOrder[0]) {
		return compat.Trace{}, fmt.Errorf("rollup-dimensionality scenario %q has no supported cases", scenario.ID)
	}
	traces := make([]compat.Trace, 0, len(caseOrder))
	for _, caseName := range caseOrder {
		var trace compat.Trace
		var err error
		if rollupDimensionalityExtendedCases[caseName] {
			trace, err = runRollupDimensionalityExtendedCase(ctx, scenario, caseName)
		} else {
			trace, err = runRollupDimensionalityCase(ctx, scenario, caseName)
		}
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rollup-dimensionality case %q: %w", caseName, err)
		}
		traces = append(traces, trace)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseTrace := range traces {
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runRollupDimensionalityDedicatedScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateRollupDimensionalityDedicatedScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range rollupDimensionalityDedicatedCases {
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runRollupDimensionalityDedicatedCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("rollup-dimensionality dedicated case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func validateRollupDimensionalityDedicatedScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != rollupDimensionalityDedicatedID || len(scenario.Steps) != 26 {
		return fmt.Errorf("rollup-dimensionality dedicated scenario steps are not pinned")
	}
	expectedSymbols := [][]string{
		{"E1", "E1", "E1", "E1"},
		{"E1", "E1", "E1", "E1"},
		{"E1", "E2", "E1", "E2", "E2"},
		{"E1", "E2", "E1", "E2", "E2"},
		{"E1", "E1", "E2"},
	}
	expectedInts := [][]int{
		{10, 20, 10, 20},
		{10, 20, 10, 20},
		{1, 1, 2, 2, 1},
		{1, 1, 2, 2, 1},
		{1, 2, 1},
	}
	expectedLongs := [][]int64{
		{100, 200, 200, 100},
		{100, 200, 200, 100},
		{10, 20, 10, 20, 10},
		{10, 20, 10, 20, 10},
		{10, 20, 25},
	}
	expectedDoubles := [][]float64{
		{1000, 2000, 3000, 4000},
		{1000, 2000, 3000, 4000},
		{100, 200, 300, 400, 500},
		{100, 200, 300, 400, 500},
		nil,
	}
	expectedPayloadFields := [][]string{
		{"theString", "intPrimitive", "longPrimitive", "doublePrimitive"},
		{"theString", "intPrimitive", "longPrimitive", "doublePrimitive"},
		{"theString", "intPrimitive", "longPrimitive", "doublePrimitive"},
		{"theString", "intPrimitive", "longPrimitive", "doublePrimitive"},
		{"theString", "intPrimitive", "longPrimitive"},
	}
	caseIndex := -1
	sendIndex := 0
	seenCases := make([]bool, len(rollupDimensionalityDedicatedCases))
	for stepIndex, step := range scenario.Steps {
		if step.Op == "case" {
			caseIndex++
			if caseIndex >= len(rollupDimensionalityDedicatedCases) || seenCases[caseIndex] ||
				step.Case != rollupDimensionalityDedicatedCases[caseIndex] || len(step.Payload) != 0 {
				return fmt.Errorf("rollup-dimensionality dedicated case marker %d is not pinned", caseIndex)
			}
			seenCases[caseIndex] = true
			sendIndex = 0
			continue
		}
		if step.Op != "send" || step.EventType != "SupportBean" || step.Case != "" {
			return fmt.Errorf("rollup-dimensionality dedicated step %d must be an unscoped SupportBean send", stepIndex)
		}
		if caseIndex < 0 || sendIndex >= len(expectedSymbols[caseIndex]) {
			return fmt.Errorf("rollup-dimensionality dedicated send sequence is not pinned")
		}
		var payload map[string]json.RawMessage
		if err := strictObject(step.Payload, &payload); err != nil {
			return fmt.Errorf("rollup-dimensionality dedicated step %d payload: %w", stepIndex, err)
		}
		if len(payload) != len(expectedPayloadFields[caseIndex]) {
			return fmt.Errorf("rollup-dimensionality dedicated step %d payload fields are not pinned", stepIndex)
		}
		for _, field := range expectedPayloadFields[caseIndex] {
			if _, ok := payload[field]; !ok {
				return fmt.Errorf("rollup-dimensionality dedicated step %d payload is missing %q", stepIndex, field)
			}
		}
		var values struct {
			TheString       string  `json:"theString"`
			IntPrimitive    int     `json:"intPrimitive"`
			LongPrimitive   int64   `json:"longPrimitive"`
			DoublePrimitive float64 `json:"doublePrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &values); err != nil {
			return fmt.Errorf("rollup-dimensionality dedicated step %d payload: %w", stepIndex, err)
		}
		if values.TheString != expectedSymbols[caseIndex][sendIndex] ||
			values.IntPrimitive != expectedInts[caseIndex][sendIndex] ||
			values.LongPrimitive != expectedLongs[caseIndex][sendIndex] ||
			(caseIndex < 4 && values.DoublePrimitive != expectedDoubles[caseIndex][sendIndex]) {
			return fmt.Errorf("rollup-dimensionality dedicated payload %d is not pinned", sendIndex)
		}
		sendIndex++
		if sendIndex == len(expectedSymbols[caseIndex]) {
			continue
		}
	}
	if caseIndex != len(rollupDimensionalityDedicatedCases)-1 || sendIndex != len(expectedSymbols[caseIndex]) {
		return fmt.Errorf("rollup-dimensionality dedicated case/send sequence is incomplete")
	}
	return nil
}

func runRollupDimensionalityDedicatedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rollupDimensionalityBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	theString := esper.Field[rollupDimensionalityBean, string]("theString")
	intPrimitive := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
	longPrimitive := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
	doublePrimitive := esper.Field[rollupDimensionalityBean, float64]("doublePrimitive")
	var query esper.Query
	switch caseName {
	case rollupDimensionalityDedicatedCases[0]:
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").GroupByGroupingSets(
			esper.GroupingSet(theString, intPrimitive), esper.GroupingSet(theString, longPrimitive),
		).Select(esper.Alias("c0", theString), esper.Alias("c1", intPrimitive), esper.Alias("c2", longPrimitive), esper.Alias("c3", esper.Sum[float64](doublePrimitive))).Query(esper.StatementName("s0"))
	case rollupDimensionalityDedicatedCases[1]:
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").GroupByGroupingSets(
			esper.GroupingSet(theString, intPrimitive), esper.GroupingSet(theString, longPrimitive),
		).Select(esper.Alias("c0", theString), esper.Alias("c1", intPrimitive), esper.Alias("c2", longPrimitive), esper.Alias("c3", esper.Sum[float64](doublePrimitive))).Query(esper.StatementName("s0"))
	case rollupDimensionalityDedicatedCases[2], rollupDimensionalityDedicatedCases[3]:
		stream := esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.LengthWindow(4))
		var aggregate esper.AggregateStream
		if caseName == rollupDimensionalityDedicatedCases[2] {
			aggregate = stream.GroupByCube(theString, intPrimitive, longPrimitive)
		} else {
			aggregate = stream.GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive), esper.GroupingSet(theString, longPrimitive), esper.GroupingSet(theString),
				esper.GroupingSet(intPrimitive, longPrimitive), esper.GroupingSet(intPrimitive), esper.GroupingSet(longPrimitive), esper.GroupingSet(),
			)
		}
		query = aggregate.Select(
			esper.Alias("c0", theString), esper.Alias("c1", intPrimitive), esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.CountAll()), esper.Alias("c4", esper.Sum[float64](doublePrimitive)),
			esper.Alias("c5", esper.Grouping(theString)), esper.Alias("c6", esper.Grouping(intPrimitive)), esper.Alias("c7", esper.Grouping(longPrimitive)), esper.Alias("c8", esper.GroupingID(theString, intPrimitive, longPrimitive)),
		).Query(esper.StatementName("s0"))
	case rollupDimensionalityDedicatedCases[4]:
		if _, err := esper.CreateKeyContext(env, "SegmentedByString", theString); err != nil {
			return compat.Trace{}, err
		}
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").GroupByRollup(theString, intPrimitive).Select(
			esper.Alias("c0", theString), esper.Alias("c1", intPrimitive), esper.Alias("c2", esper.Sum[int64](longPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("SegmentedByString"))
	default:
		return compat.Trace{}, fmt.Errorf("unsupported rollup-dimensionality dedicated case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, rollupDimensionalityDedicatedCaseRuntimes[indexOfString(rollupDimensionalityDedicatedCases, caseName)])
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, scenario, decodeRollupDimensionalityPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown rollup-dimensionality dedicated statement %q", name)
		}
		return statement, nil
	})
}

func indexOfString(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}

func runRollupDimensionalityCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[rollupDimensionalityBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[rollupDimensionalityS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	theString := esper.Field[any, string]("theString")
	intPrimitive := esper.Field[any, int]("intPrimitive")
	longPrimitive := esper.Field[any, int64]("longPrimitive")
	doublePrimitive := esper.Field[any, float64]("doublePrimitive")

	var query esper.Query
	switch caseName {
	case "unbound-rollup-2dim":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0"))
	case "unbound-rollup-1dim-rollup", "unbound-rollup-1dim-cube":
		stream := esper.From[rollupDimensionalityBean](env, "SupportBean")
		var agg esper.AggregateStream
		if caseName == "unbound-rollup-1dim-rollup" {
			agg = stream.GroupByRollup(theString)
		} else {
			agg = stream.GroupByCube(theString)
		}
		query = agg.Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-rollup-unenclosed-a", "unbound-rollup-unenclosed-b", "unbound-rollup-unenclosed-c":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive),
				esper.GroupingSet(theString),
			).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.Sum[float64](doublePrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-rollup-3dim-rollup", "unbound-rollup-3dim-gs":
		stream := esper.From[rollupDimensionalityBean](env, "SupportBean")
		var agg esper.AggregateStream
		if caseName == "unbound-rollup-3dim-gs" {
			agg = stream.GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive),
				esper.GroupingSet(theString),
				esper.GroupingSet(),
			)
		} else {
			agg = stream.GroupByRollup(theString, intPrimitive, longPrimitive)
		}
		query = agg.Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.CountAll()),
			esper.Alias("c4", esper.Sum[float64](doublePrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-rollup-3dim-rollup-join", "unbound-rollup-3dim-gs-join":
		jTheString := esper.JoinField[any](0, "theString")
		jIntPrimitive := esper.JoinField[any](0, "intPrimitive")
		jLongPrimitive := esper.JoinField[any](0, "longPrimitive")
		var agg esper.AggregateStream
		joinAgg := esper.Join(
			esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.KeepAll()),
			esper.From[rollupDimensionalityS0](env, "SupportBean_S0").Window(esper.LastEvent()),
		).GroupBy(jTheString, jIntPrimitive, jLongPrimitive)
		if caseName == "unbound-rollup-3dim-gs-join" {
			agg = joinAgg.GroupingSets(
				esper.GroupingSet(jTheString, jIntPrimitive, jLongPrimitive),
				esper.GroupingSet(jTheString, jIntPrimitive),
				esper.GroupingSet(jTheString),
				esper.GroupingSet(),
			)
		} else {
			agg = joinAgg.Rollup(jTheString, jIntPrimitive, jLongPrimitive)
		}
		query = agg.Select(
			esper.Alias("c0", jTheString),
			esper.Alias("c1", jIntPrimitive),
			esper.Alias("c2", jLongPrimitive),
			esper.Alias("c3", esper.CountAll()),
			esper.Alias("c4", esper.Sum[float64](esper.JoinField[float64](0, "doublePrimitive"))),
		).Query(esper.StatementName("s0"))
	case "unbound-cube-unenclosed-a", "unbound-cube-unenclosed-b", "unbound-cube-unenclosed-c":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByGroupingSets(
				esper.GroupingSet(theString, intPrimitive, longPrimitive),
				esper.GroupingSet(theString, intPrimitive),
				esper.GroupingSet(theString, longPrimitive),
				esper.GroupingSet(theString),
			).Select(
			esper.Alias("c0", theString),
			esper.Alias("c1", intPrimitive),
			esper.Alias("c2", longPrimitive),
			esper.Alias("c3", esper.Sum[float64](doublePrimitive)),
		).Query(esper.StatementName("s0"))
	case "unbound-cube-4dim":
		intBoxed := esper.Field[any, *int]("intBoxed")
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByCube(theString, intPrimitive, longPrimitive, doublePrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", longPrimitive),
				esper.Alias("c3", doublePrimitive),
				esper.Alias("c4", esper.Sum[int](intBoxed)),
			).Query(esper.StatementName("s0"))
	case "bound-rollup":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			Window(esper.LengthWindow(3)).
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0"))
	case "unbound-rollup-2dim-batch":
		query = esper.From[rollupDimensionalityBean](env, "SupportBean").
			Window(esper.LengthBatch(4)).
			GroupByRollup(theString, intPrimitive).
			Select(
				esper.Alias("c0", theString),
				esper.Alias("c1", intPrimitive),
				esper.Alias("c2", esper.Sum[int64](longPrimitive)),
			).Query(esper.StatementName("s0"), esper.WithOldStream())
	default:
		return compat.Trace{}, fmt.Errorf("unsupported rollup-dimensionality case %q", caseName)
	}
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statement, err := deployParityStatement(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statement, caseScenario, decodeRollupDimensionalityPayload, func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown rollup-dimensionality statement %q", name)
		}
		return statement, nil
	})
}

// rollupDimensionalityIntArray mirrors the pinned SupportEventWithIntArray
// regression bean: int[] group keys compare by content.
type rollupDimensionalityIntArray struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

// rollupDimensionalityThreeArray mirrors SupportThreeArrayEvent with three
// differently typed array properties.
type rollupDimensionalityThreeArray struct {
	ID          string    `esper:"id"`
	Value       int       `esper:"value"`
	IntArray    []int     `esper:"intArray"`
	LongArray   []int64   `esper:"longArray"`
	DoubleArray []float64 `esper:"doubleArray"`
}

func decodeRollupDimensionalityPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var bean rollupDimensionalityBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		var event rollupDimensionalityS0
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportBean_S0: %w", err)
		}
		return event, nil
	case "SupportEventWithIntArray":
		var event rollupDimensionalityIntArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportEventWithIntArray: %w", err)
		}
		return event, nil
	case "SupportThreeArrayEvent":
		var event rollupDimensionalityThreeArray
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("rollup-dimensionality SupportThreeArrayEvent: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("rollup-dimensionality: unsupported event type %q", step.EventType)
	}
}

// rollupDimensionalityExtendedCases routes the Draft 4.239 completion cases
// through the multi-statement/handler-capable runner.
var rollupDimensionalityExtendedCases = map[string]bool{
	"warray-unbound": true, "warray-bound": true, "warray-join": true, "warray-gs": true,
	"nw-cube": true, "nw-cube-gs": true,
	"onselect-rollup":    true,
	"out-when-term-last": true, "out-when-term-last-opt": true, "out-when-term-last-optdis": true,
	"out-when-term-all": true, "out-when-term-snapshot": true,
	"bound-gs-no-top": true, "bound-gs-top-detail": true,
	"mixed-access": true, "non-boxed-types": true, "groupby-computation": true,
}

// rollupDimensionalityFullBean mirrors every Java SupportBean property for
// window(*) recursive rendering (MixedAccessAggregation). Nullable boxes stay
// nil when absent; the char primitive defaults to Java's '\u0000'.
type rollupDimensionalityFullBean struct {
	TheString       *string  `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *float64 `esper:"bigDecimal"`
	BigInteger      *int64   `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

// runRollupDimensionalityExtendedCase replays the Draft 4.239 completion
// cases: multi-statement modules (named windows, contexts, on-select), the
// array-keyed rollup family, output-when-terminated variants, and the
// deploy-only types record.
func runRollupDimensionalityExtendedCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	// mixed-access registers a full-fidelity SupportBean mirror in its own
	// branch and must not collide with the short-form registration here.
	if caseName != "mixed-access" {
		if _, err := esper.RegisterStruct[rollupDimensionalityBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	}
	if _, err := esper.RegisterStruct[rollupDimensionalityS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}

	var plans []esper.Plan
	s0Name := "s0"
	build := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	}

	intPrimitive := func() esper.Expr { return esper.Field[rollupDimensionalityBean, int]("intPrimitive") }
	doublePrimitive := func() esper.Expr { return esper.Field[rollupDimensionalityBean, float64]("doublePrimitive") }

	switch caseName {
	case "warray-unbound", "warray-bound":
		if _, err := esper.RegisterStruct[rollupDimensionalityIntArray](env, "SupportEventWithIntArray"); err != nil {
			return compat.Trace{}, err
		}
		arr := esper.Field[rollupDimensionalityIntArray, []int]("array")
		val := esper.Field[rollupDimensionalityIntArray, int]("value")
		stream := esper.From[rollupDimensionalityIntArray](env, "SupportEventWithIntArray")
		if caseName == "warray-bound" {
			stream = stream.Window(esper.KeepAll())
		}
		err := build(stream.GroupByRollup(arr, val).Select(
			esper.Alias("array", arr),
			esper.Alias("value", val),
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "warray-join":
		if _, err := esper.RegisterStruct[rollupDimensionalityIntArray](env, "SupportEventWithIntArray"); err != nil {
			return compat.Trace{}, err
		}
		jArr := esper.JoinField[any](0, "array")
		jVal := esper.JoinField[any](0, "value")
		joinAgg := esper.Join(
			esper.From[rollupDimensionalityIntArray](env, "SupportEventWithIntArray").Window(esper.KeepAll()),
			esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.KeepAll()),
		).GroupBy().Rollup(jArr, jVal)
		err := build(joinAgg.Select(
			esper.Alias("array", jArr),
			esper.Alias("value", jVal),
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "warray-gs":
		if _, err := esper.RegisterStruct[rollupDimensionalityThreeArray](env, "SupportThreeArrayEvent"); err != nil {
			return compat.Trace{}, err
		}
		intArray := esper.Field[rollupDimensionalityThreeArray, []int]("intArray")
		longArray := esper.Field[rollupDimensionalityThreeArray, []int64]("longArray")
		doubleArray := esper.Field[rollupDimensionalityThreeArray, []float64]("doubleArray")
		value := esper.Field[rollupDimensionalityThreeArray, int]("value")
		err := build(esper.From[rollupDimensionalityThreeArray](env, "SupportThreeArrayEvent").
			GroupByGroupingSets(
				esper.GroupingSet(intArray),
				esper.GroupingSet(longArray),
				esper.GroupingSet(doubleArray),
			).Select(
			esper.Alias("thesum", esper.Sum[int](value)),
		).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "nw-cube", "nw-cube-gs":
		schema, err := esper.StructSchema[rollupDimensionalityBean]("SupportBean")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		intBoxed := esper.Field[rollupDimensionalityBean, *int]("intBoxed")
		insertPlan, err := env.Build(esper.OnEvent(esper.From[rollupDimensionalityBean](env, "SupportBean").Filter(
			esper.EqualOf(intBoxed, esper.Literal(0)),
		)).InsertIntoNamedWindow("MyWindow",
			esper.CopyMatchingFields(),
		).Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = append(plans, insertPlan)
		deletePlan, err := env.Build(esper.OnEvent(esper.From[rollupDimensionalityBean](env, "SupportBean").Filter(
			esper.EqualOf(intBoxed, esper.Literal(3)),
		)).DeleteFromNamedWindow("MyWindow", nil).
			Query(esper.StatementName("delete")))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = append(plans, deletePlan)
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		lp := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		var agg esper.AggregateStream
		if caseName == "nw-cube" {
			agg = esper.FromNamedWindow(env, "MyWindow").GroupByCube(ts, ip)
		} else {
			agg = esper.FromNamedWindow(env, "MyWindow").GroupByGroupingSets(
				esper.GroupingSet(ts, ip),
				esper.GroupingSet(ts),
				esper.GroupingSet(ip),
				esper.GroupingSet(),
			)
		}
		err = build(agg.Select(
			esper.Alias("c0", ts),
			esper.Alias("c1", ip),
			esper.Alias("c2", esper.Sum[int64](lp)),
		).Query(esper.StatementName(s0Name), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "onselect-rollup":
		schema, err := esper.StructSchema[rollupDimensionalityBean]("SupportBean")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		insertPlan, err := env.Build(esper.OnEvent(esper.From[rollupDimensionalityBean](env, "SupportBean")).
			InsertIntoNamedWindow("MyWindow", esper.CopyMatchingFields()).
			Query(esper.StatementName("insert")))
		if err != nil {
			return compat.Trace{}, err
		}
		plans = append(plans, insertPlan)
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		err = build(esper.OnEvent(esper.From[rollupDimensionalityS0](env, "SupportBean_S0")).
			SelectFromNamedWindowRollup("MyWindow", nil,
				[]esper.Expr{ts},
				esper.Alias("c0", ts),
				esper.Alias("c1", esper.Sum[int](ip)),
				esper.Alias("c2", esper.CountAll()),
			).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	case "out-when-term-last", "out-when-term-last-opt", "out-when-term-last-optdis", "out-when-term-all", "out-when-term-snapshot":
		idField := esper.Field[rollupDimensionalityS0, int]("id")
		isStart := esper.Equal[int](idField, esper.Literal(1))
		isEnd := esper.Equal[int](idField, esper.Literal(0))
		if _, err := esper.CreateInitiatedTerminatedContext(env, "MyContext", esper.Literal("global"), isStart, isEnd); err != nil {
			return compat.Trace{}, err
		}
		var base esper.OutputPolicy
		switch caseName {
		case "out-when-term-all":
			base = esper.OutputAll()
		case "out-when-term-snapshot":
			base = esper.OutputSnapshot()
		default:
			base = esper.OutputLast()
		}
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		err := build(esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(ts).
			Select(
				esper.Alias("c0", ts),
				esper.Alias("c1", esper.Sum[int](ip)),
			).Query(
			esper.StatementName(s0Name),
			esper.WithContext("MyContext"),
			esper.WithOutput(esper.OutputWhenTerminated(base)),
		))
		if err != nil {
			return compat.Trace{}, err
		}
	case "bound-gs-no-top", "bound-gs-top-detail":
		ts := esper.Field[rollupDimensionalityBean, string]("theString")
		ip := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		lp := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		var agg esper.AggregateStream
		if caseName == "bound-gs-no-top" {
			agg = esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.LengthWindow(4)).
				GroupByGroupingSets(esper.GroupingSet(ts), esper.GroupingSet(ip))
		} else {
			agg = esper.From[rollupDimensionalityBean](env, "SupportBean").Window(esper.LengthWindow(4)).
				GroupByGroupingSets(esper.GroupingSet(), esper.GroupingSet(ts, ip))
		}
		err := build(agg.Select(
			esper.Alias("c0", ts),
			esper.Alias("c1", ip),
			esper.Alias("c2", esper.Sum[int64](lp)),
		).Query(esper.StatementName(s0Name), esper.WithOldStream()))
		if err != nil {
			return compat.Trace{}, err
		}
	case "mixed-access":
		if _, err := esper.RegisterStruct[rollupDimensionalityFullBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
		fullString := esper.Field[rollupDimensionalityFullBean, *string]("theString")
		fullInt := esper.Field[rollupDimensionalityFullBean, int]("intPrimitive")
		err := build(esper.From[rollupDimensionalityFullBean](env, "SupportBean").
			Window(esper.LengthWindow(2)).
			GroupByRollup(fullString).
			Select(
				esper.Alias("c0", esper.Sum[int](fullInt)),
				esper.Alias("c1", fullString),
				esper.Alias("c2", esper.WindowEvents()),
			).Query(
			esper.StatementName(s0Name),
			esper.OrderBy(esper.Ascending(fullString)),
		))
		if err != nil {
			return compat.Trace{}, err
		}
	case "non-boxed-types":
		shortPrimitive := esper.Field[rollupDimensionalityBean, int16]("shortPrimitive")
		iF := intPrimitive()
		dF := doublePrimitive()
		lF := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		selectColumns := []esper.Selection{
			esper.Alias("c0", iF),
			esper.Alias("c1", dF),
			esper.Alias("c2", lF),
			esper.Alias("c3", esper.Sum[int16](shortPrimitive)),
		}
		newStream := func() esper.Stream[rollupDimensionalityBean] {
			return esper.From[rollupDimensionalityBean](env, "SupportBean")
		}
		spellings := []struct {
			name string
			agg  esper.AggregateStream
		}{
			// Java s0: group by intPrimitive, rollup(doublePrimitive, longPrimitive)
			{"s0", newStream().GroupByGroupingSets(
				esper.GroupingSet(iF, dF, lF),
				esper.GroupingSet(iF, dF),
				esper.GroupingSet(iF),
			)},
			{"s1", newStream().GroupByGroupingSets(esper.GroupingSet(iF, dF, lF))},
			{"s2", newStream().GroupByGroupingSets(
				esper.GroupingSet(iF, dF, lF),
				esper.GroupingSet(iF, dF),
			)},
			{"s3", newStream().GroupByGroupingSets(
				esper.GroupingSet(dF, iF),
				esper.GroupingSet(lF, iF),
			)},
		}
		for _, spelling := range spellings {
			query := spelling.agg.Select(selectColumns...).
				Query(esper.StatementName(spelling.name))
			if err := build(query); err != nil {
				return compat.Trace{}, err
			}
		}
	case "groupby-computation":
		longF := esper.Field[rollupDimensionalityBean, int64]("longPrimitive")
		computedKey := esper.CaseWhen[int64](
			esper.Greater[int64](longF, esper.Literal[int64](0)),
			esper.Literal[int64](1),
		).Else(esper.Literal[int64](0))
		intF := esper.Field[rollupDimensionalityBean, int]("intPrimitive")
		err := build(esper.From[rollupDimensionalityBean](env, "SupportBean").
			GroupByRollup(computedKey).
			Select(
				esper.Alias("c0", longF),
				esper.Alias("c1", esper.Sum[int](intF)),
			).Query(esper.StatementName(s0Name)))
		if err != nil {
			return compat.Trace{}, err
		}
	default:
		return compat.Trace{}, fmt.Errorf("unsupported extended rollup-dimensionality case %q", caseName)
	}

	engine := esper.NewEngine(env)
	cleanup := true
	defer func() {
		if cleanup {
			_ = engine.Close(context.Background())
		}
	}()
	type deployedStatement struct {
		statement *esper.Statement
		schema    esper.Schema
		hasSchema bool
	}
	statements := make(map[string]*esper.Statement)
	var order []deployedStatement
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		schema, schemaOK := plan.ResultSchema()
		for _, statement := range deployment.Statements() {
			statements[statement.Name()] = statement
			order = append(order, deployedStatement{statement: statement, schema: schema, hasSchema: schemaOK})
		}
	}
	primary := statements[s0Name]
	if primary == nil {
		return compat.Trace{}, fmt.Errorf("extended case %q has no %q statement", caseName, s0Name)
	}
	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	// Listener records come from the compat replay's own subscription, which
	// mirrors the oracle's per-statement sequence numbering.
	_ = primary

	handlers := map[string]compat.StepHandler{}
	if caseName == "non-boxed-types" {
		handlers["types"] = func(step compat.Step, _ func(*esper.Statement) error) ([]compat.TraceRecord, error) {
			records := make([]compat.TraceRecord, 0, len(order))
			for _, deployed := range order {
				value := map[string]any{}
				if !deployed.hasSchema {
					continue
				}
				for _, name := range []string{"c0", "c1", "c2"} {
					field, ok := deployed.schema.Field(name)
					if !ok {
						continue
					}
					if token := javaTypeName(field.Type); token != "" {
						value[name] = token
					}
				}
				records = append(records, compat.TraceRecord{
					Case:      caseName,
					Operation: "types",
					Statement: deployed.statement.Name(),
					Value:     value,
				})
			}
			return records, nil
		}
	}

	decoder := decodeRollupDimensionalityPayload
	if caseName == "mixed-access" {
		decoder = func(step compat.Step) (any, error) {
			if step.EventType == "SupportBean" {
				var bean rollupDimensionalityFullBean
				if err := json.Unmarshal(step.Payload, &bean); err != nil {
					return nil, fmt.Errorf("rollup-dimensionality SupportBean: %w", err)
				}
				// Java char primitive defaults to '\u0000'; sends never
				// override it in this scenario.
				if bean.CharPrimitive == "" {
					bean.CharPrimitive = "\u0000"
				}
				return bean, nil
			}
			return decodeRollupDimensionalityPayload(step)
		}
	}
	result, err := compat.ReplayWithStatementsAndHandlers(ctx, engine, primary, caseScenario,
		decoder,
		func(name string) (*esper.Statement, error) {
			statement, ok := statements[name]
			if !ok {
				return nil, fmt.Errorf("unknown rollup-dimensionality statement %q", name)
			}
			return statement, nil
		}, handlers)
	cleanup = false
	if err != nil {
		return trace, err
	}
	trace.Records = append(trace.Records, result.Records...)
	return trace, nil
}

// javaTypeName maps a Go output field type to the Java boxed-class simple
// name token used by the types record.
func javaTypeName(t reflect.Type) string {
	if t == nil {
		return ""
	}
	switch t.Kind() {
	case reflect.Bool:
		return "Boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "Integer"
	case reflect.Int64, reflect.Uint64:
		return "Long"
	case reflect.Float32:
		return "Float"
	case reflect.Float64:
		return "Double"
	case reflect.String:
		return "String"
	default:
		return ""
	}
}
