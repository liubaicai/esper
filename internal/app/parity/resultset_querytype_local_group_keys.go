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

const (
	resultSetQueryTypeLocalGroupKeysID          = "resultset-querytype-local-group-keys"
	resultSetQueryTypeLocalGroupKeysJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeLocalGroupKeysSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
	resultSetQueryTypeLocalGroupKeysDescription = "ResultSetQueryTypeLocalGroupBy ordinals 18/19/24/26/27: local-group key representation and readback - repeated scalar keys on object-array events, local groups shared across outer groups, the empty group_by:() statement-wide level, array-typed (int[]/long[]/double[]) keys compared by deep content, and the window(*)/first(*) accessor methods resolved per local group."

	resultSetQueryTypeLocalGroupKeysSameKeyRuntimeID      = "java-runtime-890001d4334d5de6c50a"
	resultSetQueryTypeLocalGroupKeysGroupedKeysID         = "java-runtime-a2b77511196632040e51"
	resultSetQueryTypeLocalGroupKeysEnumMethodsID         = "java-runtime-b6a938fde543383eb73c"
	resultSetQueryTypeLocalGroupKeysMultikeyArrayID       = "java-runtime-81855e4095ee0ca7cadd"
	resultSetQueryTypeLocalGroupKeysOnlyWGroupByID        = "java-runtime-e1253cd2c17a180c243a"
	resultSetQueryTypeLocalGroupKeysSameKeyStaticID       = "java-2c2e1d80b0046f88b67e"
	resultSetQueryTypeLocalGroupKeysGroupedKeysStaticID   = "java-b0aa55f10cfa580ccd4f"
	resultSetQueryTypeLocalGroupKeysEnumMethodsStaticID   = "java-98ac70ee0f434579c8c8"
	resultSetQueryTypeLocalGroupKeysMultikeyArrayStaticID = "java-6f7f7c3ba440787d3117"
	resultSetQueryTypeLocalGroupKeysOnlyWGroupByStaticID  = "java-e3b7ff9f0f3bf5872d7b"
	resultSetQueryTypeLocalGroupKeysObjectArrayOneType    = "MyEventOne"
	resultSetQueryTypeLocalGroupKeysObjectArrayTwoType    = "MyEventTwo"
	resultSetQueryTypeLocalGroupKeysThreeArrayType        = "SupportThreeArrayEvent"
	resultSetQueryTypeLocalGroupKeysSupportBeanType       = "SupportBean"
)

// The pinned EPL texts are the Java source statements verbatim: the ord-18/19
// object-array schema statements keep their newline (both statements compile as
// one deployment in Java), and the double spaces produced by the Java line
// continuations are preserved exactly.
const (
	resultSetQueryTypeLocalGroupKeysSameKeyEPL       = "@public @buseventtype create objectarray schema MyEventOne (d1 String, d2 String, val int);\n@name('s0') select sum(val, group_by: d1) as c0, sum(val, group_by: d2) as c1 from MyEventOne"
	resultSetQueryTypeLocalGroupKeysGroupedKeyEPL    = "@public @buseventtype create objectarray schema MyEventTwo (g1 String, d1 String, d2 String, val int);\n@name('s0') select sum(val) as c0, sum(val, group_by: d1) as c1, sum(val, group_by: d2) as c2 from MyEventTwo group by g1"
	resultSetQueryTypeLocalGroupKeysEnumMethodEPL    = "@name('s0') select window(*, group_by:()).firstOf() as c0, window(*, group_by:theString).firstOf() as c1, window(intPrimitive, group_by:()).firstOf() as c2, window(intPrimitive, group_by:theString).firstOf() as c3, first(*, group_by:()).intPrimitive as c4, first(*, group_by:theString).intPrimitive as c5  from SupportBean#keepall group by theString, intPrimitive"
	resultSetQueryTypeLocalGroupKeysMultikeyArrayEPL = "@Name('s0') select sum(value, group_by:(intArray)) as c0, sum(value, group_by:(longArray)) as c1, sum(value, group_by:(doubleArray)) as c2, sum(value, group_by:(intArray, longArray, doubleArray)) as c3, sum(value) as c4 from SupportThreeArrayEvent"
	resultSetQueryTypeLocalGroupKeysOnlyWGroupByEPL  = "@name('s0') select first(*, group_by:()).intPrimitive as c0  from SupportBean#keepall group by theString, intPrimitive"
)

type resultSetQueryTypeLocalGroupKeysCaseSpec struct {
	name              string
	ordinal           int
	runtimeID         string
	execution         string
	observation       string
	iteratorSnapshots int
	records           int
	epl               string
}

var resultSetQueryTypeLocalGroupKeysCaseSpecs = []resultSetQueryTypeLocalGroupKeysCaseSpec{
	{
		name:        "ungrouped-same-key",
		records:     5,
		ordinal:     18,
		runtimeID:   resultSetQueryTypeLocalGroupKeysSameKeyRuntimeID,
		execution:   "ResultSetLocalUngroupedSameKey",
		observation: "listener",
		epl:         resultSetQueryTypeLocalGroupKeysSameKeyEPL,
	},
	{
		name:        "grouped-same-key",
		records:     5,
		ordinal:     19,
		runtimeID:   resultSetQueryTypeLocalGroupKeysGroupedKeysID,
		execution:   "ResultSetLocalGroupedSameKey",
		observation: "listener",
		epl:         resultSetQueryTypeLocalGroupKeysGroupedKeyEPL,
	},
	{
		name:        "enum-methods-grouped",
		records:     1,
		ordinal:     24,
		runtimeID:   resultSetQueryTypeLocalGroupKeysEnumMethodsID,
		execution:   "ResultSetLocalEnumMethods",
		observation: "listener",
		epl:         resultSetQueryTypeLocalGroupKeysEnumMethodEPL,
	},
	{
		name:        "multikey-w-array",
		records:     7,
		ordinal:     26,
		runtimeID:   resultSetQueryTypeLocalGroupKeysMultikeyArrayID,
		execution:   "ResultSetLocalMultikeyWArray",
		observation: "listener",
		epl:         resultSetQueryTypeLocalGroupKeysMultikeyArrayEPL,
	},
	{
		name:        "ungrouped-only-w-group-by",
		records:     2,
		ordinal:     27,
		runtimeID:   resultSetQueryTypeLocalGroupKeysOnlyWGroupByID,
		execution:   "ResultSetLocalUngroupedOnlyWGroupBy",
		observation: "listener",
		epl:         resultSetQueryTypeLocalGroupKeysOnlyWGroupByEPL,
	},
}

var resultSetQueryTypeLocalGroupKeysRuntimes = []string{
	resultSetQueryTypeLocalGroupKeysSameKeyRuntimeID,
	resultSetQueryTypeLocalGroupKeysGroupedKeysID,
	resultSetQueryTypeLocalGroupKeysEnumMethodsID,
	resultSetQueryTypeLocalGroupKeysMultikeyArrayID,
	resultSetQueryTypeLocalGroupKeysOnlyWGroupByID,
}

var resultSetQueryTypeLocalGroupKeysExecutions = []string{
	"ResultSetLocalUngroupedSameKey",
	"ResultSetLocalGroupedSameKey",
	"ResultSetLocalEnumMethods",
	"ResultSetLocalMultikeyWArray",
	"ResultSetLocalUngroupedOnlyWGroupBy",
}

var (
	resultSetQueryTypeLocalGroupKeysJavaSources = []string{resultSetQueryTypeLocalGroupKeysSource}
	resultSetQueryTypeLocalGroupKeysStaticIDs   = []string{
		resultSetQueryTypeLocalGroupKeysSameKeyStaticID,
		resultSetQueryTypeLocalGroupKeysGroupedKeysStaticID,
		resultSetQueryTypeLocalGroupKeysEnumMethodsStaticID,
		resultSetQueryTypeLocalGroupKeysMultikeyArrayStaticID,
		resultSetQueryTypeLocalGroupKeysOnlyWGroupByStaticID,
	}
)

func resultSetQueryTypeLocalGroupKeysJavaRuntimeIDs() []string {
	return append([]string(nil), resultSetQueryTypeLocalGroupKeysRuntimes...)
}

func resultSetQueryTypeLocalGroupKeysJavaExecutions() []string {
	return append([]string(nil), resultSetQueryTypeLocalGroupKeysExecutions...)
}

// resultSetQueryTypeLocalGroupKeysThreeArrayEvent mirrors the Java support bean
// SupportThreeArrayEvent (id, value, int[], long[], double[]). The array
// properties are the local-group keys of ordinal 26.
type resultSetQueryTypeLocalGroupKeysThreeArrayEvent struct {
	ID          string    `esper:"id"`
	Value       int32     `esper:"value"`
	IntArray    []int32   `esper:"intArray"`
	LongArray   []int64   `esper:"longArray"`
	DoubleArray []float64 `esper:"doubleArray"`
}

// resultSetQueryTypeLocalGroupKeysStepSpec pins every scenario step in order;
// the loader rejects any drift before the runtime is touched. The payload pin
// is the whitespace-free JSON spelling, which keeps array and bean payloads on
// one comparable footing.
type resultSetQueryTypeLocalGroupKeysStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   string
}

func resultSetQueryTypeLocalGroupKeysObjectArrayOne(d1, d2 string, val int32) string {
	return fmt.Sprintf(`["%s","%s",%d]`, d1, d2, val)
}

func resultSetQueryTypeLocalGroupKeysObjectArrayTwo(g1, d1, d2 string, val int32) string {
	return fmt.Sprintf(`["%s","%s","%s",%d]`, g1, d1, d2, val)
}

func resultSetQueryTypeLocalGroupKeysBeanSend(theString string, intPrimitive int32) string {
	return fmt.Sprintf(`{"theString":"%s","intPrimitive":%d,"longPrimitive":0}`, theString, intPrimitive)
}

func resultSetQueryTypeLocalGroupKeysArraySend(id string, value int32, intArray []int32, longArray []int64, doubleArray []float64) string {
	ints, _ := json.Marshal(intArray)
	longs, _ := json.Marshal(longArray)
	doubles, _ := json.Marshal(doubleArray)
	return fmt.Sprintf(`{"id":"%s","value":%d,"intArray":%s,"longArray":%s,"doubleArray":%s}`,
		id, value, ints, longs, doubles)
}

var resultSetQueryTypeLocalGroupKeysStepSpecs = func() []resultSetQueryTypeLocalGroupKeysStepSpec {
	specs := make([]resultSetQueryTypeLocalGroupKeysStepSpec, 0, 25)
	mark := func(name string) {
		specs = append(specs, resultSetQueryTypeLocalGroupKeysStepSpec{op: "case", caseName: name})
	}
	send := func(caseName, eventType, payload string) {
		specs = append(specs, resultSetQueryTypeLocalGroupKeysStepSpec{
			op: "send", caseName: caseName, eventType: eventType, payload: payload,
		})
	}

	mark("ungrouped-same-key")
	send("ungrouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayOneType, resultSetQueryTypeLocalGroupKeysObjectArrayOne("E1", "E1", 10))
	send("ungrouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayOneType, resultSetQueryTypeLocalGroupKeysObjectArrayOne("E1", "E2", 11))
	send("ungrouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayOneType, resultSetQueryTypeLocalGroupKeysObjectArrayOne("E2", "E1", 12))
	send("ungrouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayOneType, resultSetQueryTypeLocalGroupKeysObjectArrayOne("E3", "E1", 13))
	send("ungrouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayOneType, resultSetQueryTypeLocalGroupKeysObjectArrayOne("E3", "E3", 14))

	mark("grouped-same-key")
	send("grouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayTwoType, resultSetQueryTypeLocalGroupKeysObjectArrayTwo("E1", "E1", "E1", 10))
	send("grouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayTwoType, resultSetQueryTypeLocalGroupKeysObjectArrayTwo("E1", "E1", "E2", 11))
	send("grouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayTwoType, resultSetQueryTypeLocalGroupKeysObjectArrayTwo("E1", "E2", "E1", 12))
	send("grouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayTwoType, resultSetQueryTypeLocalGroupKeysObjectArrayTwo("X", "E1", "E1", 13))
	send("grouped-same-key", resultSetQueryTypeLocalGroupKeysObjectArrayTwoType, resultSetQueryTypeLocalGroupKeysObjectArrayTwo("E1", "E2", "E3", 14))

	mark("enum-methods-grouped")
	send("enum-methods-grouped", resultSetQueryTypeLocalGroupKeysSupportBeanType, resultSetQueryTypeLocalGroupKeysBeanSend("E1", 10))

	mark("multikey-w-array")
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E1", 10, []int32{1}, []int64{10}, []float64{100}))
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E2", 11, []int32{2}, []int64{20}, []float64{200}))
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E3", 12, []int32{3}, []int64{10}, []float64{300}))
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E4", 13, []int32{1}, []int64{20}, []float64{200}))
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E5", 14, []int32{1}, []int64{10}, []float64{100}))
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E6", 15, []int32{3}, []int64{20}, []float64{300}))
	send("multikey-w-array", resultSetQueryTypeLocalGroupKeysThreeArrayType, resultSetQueryTypeLocalGroupKeysArraySend("E7", 16, []int32{2}, []int64{20}, []float64{200}))

	mark("ungrouped-only-w-group-by")
	send("ungrouped-only-w-group-by", resultSetQueryTypeLocalGroupKeysSupportBeanType, resultSetQueryTypeLocalGroupKeysBeanSend("E1", 1))
	send("ungrouped-only-w-group-by", resultSetQueryTypeLocalGroupKeysSupportBeanType, resultSetQueryTypeLocalGroupKeysBeanSend("E2", 2))

	return specs
}()

func loadResultSetQueryTypeLocalGroupKeysScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultSetQueryTypeLocalGroupKeysID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultSetQueryTypeLocalGroupKeysID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupKeysID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultSetQueryTypeLocalGroupKeysID, err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	version, err := decodeResultSetQueryTypeLocalGroupByString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetQueryTypeLocalGroupByString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	description, err := decodeResultSetQueryTypeLocalGroupByString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaCommit, err := decodeResultSetQueryTypeLocalGroupByString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaSource, err := decodeResultSetQueryTypeLocalGroupByString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeLocalGroupKeysID ||
		description != resultSetQueryTypeLocalGroupKeysDescription ||
		javaCommit != resultSetQueryTypeLocalGroupKeysJavaCommit ||
		javaSource != resultSetQueryTypeLocalGroupKeysSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultSetQueryTypeLocalGroupKeysID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], resultSetQueryTypeLocalGroupKeysRuntimes, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], resultSetQueryTypeLocalGroupKeysExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], resultSetQueryTypeLocalGroupKeysStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultSetQueryTypeLocalGroupKeysCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultSetQueryTypeLocalGroupKeysID, len(resultSetQueryTypeLocalGroupKeysCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var caseObject map[string]json.RawMessage
		if err := strictObject(rawCase, &caseObject); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(caseObject,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		ordinal, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["ordinal"], "ordinal")
		if err != nil {
			return compat.Scenario{}, err
		}
		iteratorSnapshots, err := decodeResultSetQueryTypeLocalGroupByInteger(caseObject["iteratorSnapshots"], "iteratorSnapshots")
		if err != nil {
			return compat.Scenario{}, err
		}
		var caseMeta struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		spec := resultSetQueryTypeLocalGroupKeysCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || ordinal != spec.ordinal || caseMeta.Ordinal != ordinal ||
			caseMeta.RuntimeID != spec.runtimeID || caseMeta.ExecutionName != spec.execution ||
			caseMeta.Observation != spec.observation || iteratorSnapshots != spec.iteratorSnapshots ||
			caseMeta.IteratorSnapshots != iteratorSnapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultSetQueryTypeLocalGroupKeysID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(resultSetQueryTypeLocalGroupKeysStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", resultSetQueryTypeLocalGroupKeysID, len(resultSetQueryTypeLocalGroupKeysStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := resultSetQueryTypeLocalGroupKeysStepSpecs[index]
		var expected []string
		if spec.op == "case" {
			expected = []string{"op", "case"}
		} else {
			expected = []string{"op", "eventType", "payload"}
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateResultSetQueryTypeLocalGroupKeysScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultSetQueryTypeLocalGroupKeysScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultSetQueryTypeLocalGroupKeysID ||
		len(scenario.Steps) != len(resultSetQueryTypeLocalGroupKeysStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", resultSetQueryTypeLocalGroupKeysID)
	}
	for index, spec := range resultSetQueryTypeLocalGroupKeysStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", resultSetQueryTypeLocalGroupKeysID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", resultSetQueryTypeLocalGroupKeysID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.eventType || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", resultSetQueryTypeLocalGroupKeysID, index, spec.eventType)
		}
		compact, err := resultSetQueryTypeLocalGroupKeysCompactJSON(step.Payload)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", resultSetQueryTypeLocalGroupKeysID, index, err)
		}
		if compact != spec.payload {
			return fmt.Errorf("%s scenario step %d %s payload is not pinned", resultSetQueryTypeLocalGroupKeysID, index, spec.eventType)
		}
	}
	return nil
}

func resultSetQueryTypeLocalGroupKeysCompactJSON(raw json.RawMessage) (string, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		return "", fmt.Errorf("payload is not valid JSON: %w", err)
	}
	return buffer.String(), nil
}

func runResultSetQueryTypeLocalGroupKeysScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeLocalGroupKeysScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: resultSetQueryTypeLocalGroupKeysID}
	for index, spec := range resultSetQueryTypeLocalGroupKeysCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runResultSetQueryTypeLocalGroupKeysCase(ctx, spec, caseScenario, index)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultSetQueryTypeLocalGroupKeysID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateResultSetQueryTypeLocalGroupKeysTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func resultSetQueryTypeLocalGroupKeysQuery(env *esper.Environment, caseIndex int) (esper.Query, error) {
	switch caseIndex {
	case 0:
		// Java's `group_by: d1` is a single-key local group; the statement is
		// ungrouped, so every event delivers its own row.
		d1 := esper.Field[any, string]("d1")
		d2 := esper.Field[any, string]("d2")
		val := esper.Field[any, int32]("val")
		return esper.FromAny(env, resultSetQueryTypeLocalGroupKeysObjectArrayOneType).
			Aggregate(
				esper.Alias("c0", esper.LocalGroupBy[int32](esper.Sum[int32](val), d1)),
				esper.Alias("c1", esper.LocalGroupBy[int32](esper.Sum[int32](val), d2)),
			).
			Query(esper.StatementName("s0")), nil
	case 1:
		// `sum(val)` stays scoped to the outer g1 group while the two local
		// groups accumulate across every outer group, which is what the "X"
		// row of the Java execution pins.
		g1 := esper.Field[any, string]("g1")
		d1 := esper.Field[any, string]("d1")
		d2 := esper.Field[any, string]("d2")
		val := esper.Field[any, int32]("val")
		return esper.FromAny(env, resultSetQueryTypeLocalGroupKeysObjectArrayTwoType).
			GroupBy(g1).
			Select(
				esper.Alias("c0", esper.Sum[int32](val)),
				esper.Alias("c1", esper.LocalGroupBy[int32](esper.Sum[int32](val), d1)),
				esper.Alias("c2", esper.LocalGroupBy[int32](esper.Sum[int32](val), d2)),
			).
			Query(esper.StatementName("s0")), nil
	case 2:
		// window(*)/window(intPrimitive) with an empty key list read the
		// statement-wide level; the keyed variants read the group selected by
		// the current event's theString. first(*) keeps the event identity, so
		// c0/c1 project the earliest SupportBean of each level.
		theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
		intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
		windowsGlobal := esper.LocalGroupBy[[]esper.Event](esper.WindowEvents())
		windowsByKey := esper.LocalGroupBy[[]esper.Event](esper.WindowEvents(), theString)
		scalarsGlobal := esper.LocalGroupBy[[]int32](esper.WindowValues[int32](intPrimitive))
		scalarsByKey := esper.LocalGroupBy[[]int32](esper.WindowValues[int32](intPrimitive), theString)
		firstGlobal := esper.NestedField[int32](esper.LocalGroupBy[esper.Event](esper.FirstEventValue()), "intPrimitive")
		firstByKey := esper.NestedField[int32](esper.LocalGroupBy[esper.Event](esper.FirstEventValue(), theString), "intPrimitive")
		return esper.From[resultSetQueryTypeLocalGroupByBean](env, resultSetQueryTypeLocalGroupKeysSupportBeanType).
			Window(esper.KeepAll()).
			GroupBy(theString, intPrimitive).
			Select(
				esper.Alias("c0", esper.EnumFirstOf[esper.Event](windowsGlobal)),
				esper.Alias("c1", esper.EnumFirstOf[esper.Event](windowsByKey)),
				esper.Alias("c2", esper.EnumFirstOf[int32](scalarsGlobal)),
				esper.Alias("c3", esper.EnumFirstOf[int32](scalarsByKey)),
				esper.Alias("c4", firstGlobal),
				esper.Alias("c5", firstByKey),
			).
			Query(esper.StatementName("s0")), nil
	case 3:
		// Array-typed local-group keys compare by deep content, and a distinct
		// array instance with equal content shares the group (Java wraps the
		// keys in Arrays.equals-based MultiKeyArray instances).
		value := esper.Field[resultSetQueryTypeLocalGroupKeysThreeArrayEvent, int32]("value")
		intArray := esper.Field[resultSetQueryTypeLocalGroupKeysThreeArrayEvent, []int32]("intArray")
		longArray := esper.Field[resultSetQueryTypeLocalGroupKeysThreeArrayEvent, []int64]("longArray")
		doubleArray := esper.Field[resultSetQueryTypeLocalGroupKeysThreeArrayEvent, []float64]("doubleArray")
		sum := func(keys ...esper.Expr) esper.AggregateExpression[int32] {
			return esper.LocalGroupBy[int32](esper.Sum[int32](value), keys...)
		}
		return esper.From[resultSetQueryTypeLocalGroupKeysThreeArrayEvent](env, resultSetQueryTypeLocalGroupKeysThreeArrayType).
			Aggregate(
				esper.Alias("c0", sum(intArray)),
				esper.Alias("c1", sum(longArray)),
				esper.Alias("c2", sum(doubleArray)),
				esper.Alias("c3", sum(intArray, longArray, doubleArray)),
				esper.Alias("c4", esper.Sum[int32](value)),
			).
			Query(esper.StatementName("s0")), nil
	case 4:
		// `first(*, group_by:())` reads the statement-wide level, so the value
		// stays on the first event ever even when the outer group changes.
		theString := esper.Field[resultSetQueryTypeLocalGroupByBean, string]("theString")
		intPrimitive := esper.Field[resultSetQueryTypeLocalGroupByBean, int32]("intPrimitive")
		first := esper.NestedField[int32](esper.LocalGroupBy[esper.Event](esper.FirstEventValue()), "intPrimitive")
		return esper.From[resultSetQueryTypeLocalGroupByBean](env, resultSetQueryTypeLocalGroupKeysSupportBeanType).
			Window(esper.KeepAll()).
			GroupBy(theString, intPrimitive).
			Select(esper.Alias("c0", first)).
			Query(esper.StatementName("s0")), nil
	default:
		return esper.Query{}, fmt.Errorf("unknown %s case index %d", resultSetQueryTypeLocalGroupKeysID, caseIndex)
	}
}

func runResultSetQueryTypeLocalGroupKeysCase(ctx context.Context, spec resultSetQueryTypeLocalGroupKeysCaseSpec, caseScenario compat.Scenario, caseIndex int) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupByBean](env, resultSetQueryTypeLocalGroupKeysSupportBeanType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[resultSetQueryTypeLocalGroupKeysThreeArrayEvent](env, resultSetQueryTypeLocalGroupKeysThreeArrayType); err != nil {
		return nil, err
	}
	switch caseIndex {
	case 0:
		if _, err := esper.RegisterObjectArray(env, resultSetQueryTypeLocalGroupKeysObjectArrayOneType, []esper.FieldSpec{
			esper.FieldDef("d1", reflect.TypeOf("")),
			esper.FieldDef("d2", reflect.TypeOf("")),
			esper.FieldDef("val", reflect.TypeOf(int32(0))),
		}); err != nil {
			return nil, err
		}
	case 1:
		if _, err := esper.RegisterObjectArray(env, resultSetQueryTypeLocalGroupKeysObjectArrayTwoType, []esper.FieldSpec{
			esper.FieldDef("g1", reflect.TypeOf("")),
			esper.FieldDef("d1", reflect.TypeOf("")),
			esper.FieldDef("d2", reflect.TypeOf("")),
			esper.FieldDef("val", reflect.TypeOf(int32(0))),
		}); err != nil {
			return nil, err
		}
	}
	query, err := resultSetQueryTypeLocalGroupKeysQuery(env, caseIndex)
	if err != nil {
		return nil, err
	}
	plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, spec.runtimeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	records := make([]compat.TraceRecord, 0, spec.records)
	invocations := 0
	emptyCallbacks := 0
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		invocations++
		newRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.New)
		oldRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			emptyCallbacks++
			return nil
		}
		sequence++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       newRows,
			Old:       oldRows,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := resultSetQueryTypeLocalGroupKeysSend(ctx, engine, step); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", resultSetQueryTypeLocalGroupKeysID, step.Op)
		}
	}
	// Every Java execution of this batch delivers exactly one non-empty callback
	// per send and never an empty one (assertPropsNew/assertEqualsNew assert a
	// single new row), so a callback-count drift or an empty delivery is a
	// parity failure rather than something to filter away.
	if emptyCallbacks != 0 {
		return nil, fmt.Errorf("%s case %q delivered %d empty listener callbacks", resultSetQueryTypeLocalGroupKeysID, spec.name, emptyCallbacks)
	}
	if invocations != spec.records || len(records) != spec.records {
		return nil, fmt.Errorf("%s case %q delivered %d listener callbacks (%d records), want %d",
			resultSetQueryTypeLocalGroupKeysID, spec.name, invocations, len(records), spec.records)
	}
	return records, nil
}

func resultSetQueryTypeLocalGroupKeysSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	switch step.EventType {
	case resultSetQueryTypeLocalGroupKeysObjectArrayOneType:
		var fields []json.RawMessage
		if err := json.Unmarshal(step.Payload, &fields); err != nil || len(fields) != 3 {
			return fmt.Errorf("%s payload must be a three-element array", resultSetQueryTypeLocalGroupKeysObjectArrayOneType)
		}
		values, err := resultSetQueryTypeLocalGroupKeysDecodeObjectArray(fields, []reflect.Type{
			reflect.TypeOf(""), reflect.TypeOf(""), reflect.TypeOf(int32(0)),
		})
		if err != nil {
			return err
		}
		return engine.SendObjectArray(ctx, step.EventType, values)
	case resultSetQueryTypeLocalGroupKeysObjectArrayTwoType:
		var fields []json.RawMessage
		if err := json.Unmarshal(step.Payload, &fields); err != nil || len(fields) != 4 {
			return fmt.Errorf("%s payload must be a four-element array", resultSetQueryTypeLocalGroupKeysObjectArrayTwoType)
		}
		values, err := resultSetQueryTypeLocalGroupKeysDecodeObjectArray(fields, []reflect.Type{
			reflect.TypeOf(""), reflect.TypeOf(""), reflect.TypeOf(""), reflect.TypeOf(int32(0)),
		})
		if err != nil {
			return err
		}
		return engine.SendObjectArray(ctx, step.EventType, values)
	case resultSetQueryTypeLocalGroupKeysThreeArrayType:
		var fields map[string]json.RawMessage
		if err := strictObject(step.Payload, &fields); err != nil {
			return err
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(fields, "id", "value", "intArray", "longArray", "doubleArray"); err != nil {
			return fmt.Errorf("%s payload: %w", resultSetQueryTypeLocalGroupKeysThreeArrayType, err)
		}
		var event resultSetQueryTypeLocalGroupKeysThreeArrayEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return fmt.Errorf("%s payload: %w", resultSetQueryTypeLocalGroupKeysThreeArrayType, err)
		}
		return engine.Send(ctx, step.EventType, event)
	case resultSetQueryTypeLocalGroupKeysSupportBeanType:
		payload, err := decodeResultSetQueryTypeLocalGroupByPayload(step)
		if err != nil {
			return err
		}
		bean := resultSetQueryTypeLocalGroupByBean{
			TheString: payload.TheString, IntPrimitive: payload.IntPrimitive,
			LongPrimitive: payload.LongPrimitive, CharPrimitive: "\u0000",
		}
		return engine.Send(ctx, step.EventType, bean)
	default:
		return fmt.Errorf("unknown %s event type %q", resultSetQueryTypeLocalGroupKeysID, step.EventType)
	}
}

func resultSetQueryTypeLocalGroupKeysDecodeObjectArray(fields []json.RawMessage, types []reflect.Type) ([]any, error) {
	values := make([]any, len(fields))
	for index, raw := range fields {
		target := reflect.New(types[index]).Interface()
		if err := json.Unmarshal(raw, target); err != nil {
			return nil, fmt.Errorf("object-array field %d: %w", index, err)
		}
		values[index] = reflect.ValueOf(target).Elem().Interface()
	}
	return values, nil
}

func validateResultSetQueryTypeLocalGroupKeysTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != resultSetQueryTypeLocalGroupKeysID {
		return fmt.Errorf("%s trace identity is not pinned", resultSetQueryTypeLocalGroupKeysID)
	}
	type expectation struct {
		caseName string
		sequence uint64
	}
	var want []expectation
	for _, spec := range resultSetQueryTypeLocalGroupKeysCaseSpecs {
		for index := 1; index <= spec.records; index++ {
			want = append(want, expectation{caseName: spec.name, sequence: uint64(index)})
		}
	}
	if len(trace.Records) != len(want) {
		return fmt.Errorf("%s trace must contain exactly %d records", resultSetQueryTypeLocalGroupKeysID, len(want))
	}
	stamp := compat.FormatTraceTime(time.Unix(0, 0).UTC())
	for index, record := range trace.Records {
		if record.Case != want[index].caseName || record.Sequence != want[index].sequence {
			return fmt.Errorf("%s trace record %d is not pinned", resultSetQueryTypeLocalGroupKeysID, index)
		}
		if record.Operation != "listener" || record.Statement != "s0" || record.Time != stamp {
			return fmt.Errorf("%s trace record %d is not a pinned listener callback", resultSetQueryTypeLocalGroupKeysID, index)
		}
		if len(record.Old) != 0 {
			return fmt.Errorf("%s trace record %d must not carry old rows", resultSetQueryTypeLocalGroupKeysID, index)
		}
	}
	return nil
}
