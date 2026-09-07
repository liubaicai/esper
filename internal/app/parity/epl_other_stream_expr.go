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

const (
	eplOtherStreamExprID          = "epl-other-stream-expr"
	eplOtherStreamExprDescription = "EPLOtherStreamExpr stream method expressions: static-method where filters with stream-name, wildcard, and EventBean arguments, aliased and verbatim no-alias expression-text output names with Long/Double/String value rendering, stream-as-object join columns, and a followed-by pattern with a static UDF filter referencing the prior tag (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherStreamExpr.java; chained parameterized, outer-join instance-method, static-method instance, and invalid-select executions are deferred with rationale in the capability manifest)."
	eplOtherStreamExprJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplOtherStreamExprSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherStreamExpr.java"

	eplOtherStreamExprChainedEPL   = `@name('s0') select top.getChildOne("abc",10).getChildTwo("append") from SupportChainTop as top`
	eplOtherStreamExprFunctionA    = `@name('s0') select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZero(s0)`
	eplOtherStreamExprFunctionB    = `@name('s0') select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZero(*)`
	eplOtherStreamExprFunctionC    = `@name('s0') select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZeroEventBean(s0)`
	eplOtherStreamExprFunctionD    = `@name('s0') select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZeroEventBean(*)`
	eplOtherStreamExprOuterJoin    = "@name('s0') select symbol, s1.getTheString() as theString from SupportMarketDataBean#keepall as s0 left outer join SupportBean#keepall as s1 on s0.symbol=s1.theString"
	eplOtherStreamExprStaticJoin   = "@name('s0') select symbol, s1.getSimpleProperty() as simpleprop, s1.makeDefaultBean() as def from SupportMarketDataBean#keepall as s0 left outer join SupportBeanComplexProps#keepall as s1 on s0.symbol=s1.simpleProperty"
	eplOtherStreamExprAliasedEPL   = "@name('s0') select s0.getVolume() as volume, s0.getSymbol() as symbol, s0.getPriceTimesVolume(2) as pvf from SupportMarketDataBean as s0 "
	eplOtherStreamExprNoAliasEPL   = "@name('s0') select s0.getVolume(), s0.getPriceTimesVolume(3) from SupportMarketDataBean as s0 "
	eplOtherStreamExprMyTestSchema = "@public @buseventtype create schema MyTestEvent as com.espertech.esper.regressionlib.suite.epl.other.EPLOtherStreamExpr$MyTestEvent"
	eplOtherStreamExprMyTestEPL    = "@name('s0') select s0.getValueAsInt(s0, 'id') as c0,s0.getValueAsInt(*, 'id') as c1 from MyTestEvent as s0"
	eplOtherStreamExprJoinAliased  = "@name('s0') select s0 as s0stream, s1 as s1stream from SupportMarketDataBean#keepall as s0, SupportBean#keepall as s1"
	eplOtherStreamExprJoinPlain    = "@name('s0') select s0, s1 from SupportMarketDataBean#keepall as s0, SupportBean#keepall as s1"
	eplOtherStreamExprPatternEPL   = "@name('s0') select * from pattern [every e1=SupportMarketDataBean -> e2=SupportBean(com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.compareEvents(e1, e2))]"
)

var (
	eplOtherStreamExprJavaSources = []string{
		eplOtherStreamExprSource,
	}
	eplOtherStreamExprJavaRuntimeIDs = []string{
		"java-runtime-67e9ea0d239585623711",
		"java-runtime-cc45d135a75bb01736f0",
		"java-runtime-469a37a746e59d25a115",
		"java-runtime-a59b12bbe5788257c37b",
		"java-runtime-827ea8daeea9baec40cf",
	}
	eplOtherStreamExprJavaExecutions = []string{
		"EPLOtherStreamFunction",
		"EPLOtherStreamInstanceMethodAliased",
		"EPLOtherStreamInstanceMethodNoAlias",
		"EPLOtherJoinStreamSelectNoWildcard",
		"EPLOtherPatternStreamSelectNoWildcard",
	}
	eplOtherStreamExprJavaStaticIDs = []string{
		"java-e571ee83c24576b8aba7",
		"java-b448cd11a74aaf56dbab",
		"java-572869619d74ba3c95be",
		"java-534aeb16b8d33707670a",
		"java-dfb3493bb3eb08a778db",
	}
	eplOtherStreamExprCases = []string{
		"stream-function",
		"stream-instance-method-aliased",
		"stream-instance-method-no-alias",
		"join-stream-select",
		"pattern-stream-select",
	}
	eplOtherStreamExprOrdinals = []int{1, 4, 5, 6, 7}
)

// Harness-local mirrors of the Java event types, with the Go methods the
// stream method expressions invoke.
type eplOtherStreamExprChainTop struct{}

type eplOtherStreamExprChainChild struct {
	Text string `esper:"text"`
}

type eplOtherStreamExprChainChildTwo struct {
	Text string `esper:"text"`
}

func (eplOtherStreamExprChainTop) GetChildOne(text string, value int) eplOtherStreamExprChainChild {
	return eplOtherStreamExprChainChild{Text: text}
}

func (c eplOtherStreamExprChainChild) GetChildTwo(text string) eplOtherStreamExprChainChildTwo {
	return eplOtherStreamExprChainChildTwo{Text: c.Text + text}
}

func (c eplOtherStreamExprChainChildTwo) GetText() string { return c.Text }

type eplOtherStreamExprMD struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

func (m eplOtherStreamExprMD) GetVolume() int64  { return m.Volume }
func (m eplOtherStreamExprMD) GetSymbol() string { return m.Symbol }

func (m eplOtherStreamExprMD) GetPriceTimesVolume(factor float64) float64 {
	return m.Price * float64(m.Volume) * factor
}

func eplOtherStreamExprVolumeGreaterZero(m eplOtherStreamExprMD) bool {
	return m.Volume > 0
}

func eplOtherStreamExprVolumeGreaterZeroEvent(event esper.Event) bool {
	if value := event.Get("volume"); value.IsPresent() && !value.IsNull() {
		if volume, ok := value.Any().(int64); ok {
			return volume > 0
		}
	}
	return false
}

type eplOtherStreamExprBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
}

type eplOtherStreamExprStaticInnerTwo struct{}

func (eplOtherStreamExprStaticInnerTwo) GetMyOtherString() string { return "hello2" }

type eplOtherStreamExprStaticInner struct {
	InsideTwo eplOtherStreamExprStaticInnerTwo `esper:"insideTwo"`
}

func (eplOtherStreamExprStaticInner) GetMyString() string { return "hello" }

type eplOtherStreamExprStaticOuter struct {
	Inside eplOtherStreamExprStaticInner `esper:"inside"`
}

type eplOtherStreamExprComplexProps struct {
	SimpleProperty string `esper:"simpleProperty"`
}

func (c eplOtherStreamExprComplexProps) GetSimpleProperty() string { return c.SimpleProperty }

func (c eplOtherStreamExprComplexProps) MakeDefaultBean() eplOtherStreamExprComplexProps {
	return eplOtherStreamExprComplexProps{SimpleProperty: "simple"}
}

type eplOtherStreamExprMyTestEvent struct {
	ID int `esper:"id"`
}

func (m eplOtherStreamExprMyTestEvent) GetID() int { return m.ID }

func (m eplOtherStreamExprMyTestEvent) GetValueAsInt(event esper.Event, name string) int {
	if value := event.Get(name); value.IsPresent() && !value.IsNull() {
		if id, ok := value.Any().(int); ok {
			return id
		}
	}
	return 0
}

func loadEplOtherStreamExprScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherStreamExprID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherStreamExprID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherStreamExprID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherStreamExprID, err)
	}
	if err := requireEplOtherStreamExprFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherStreamExprID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherStreamExprID ||
		metadata.Description != eplOtherStreamExprDescription ||
		metadata.JavaCommit != eplOtherStreamExprJavaCommit ||
		metadata.JavaSource != eplOtherStreamExprSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplOtherStreamExprID)
	}
	if err := validateEplOtherStreamExprStringArray(root["javaRuntimes"], eplOtherStreamExprJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherStreamExprStringArray(root["javaNames"], eplOtherStreamExprJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherStreamExprStringArray(root["javaStaticIds"], eplOtherStreamExprJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherStreamExprStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplOtherStreamExprCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly eight cases", eplOtherStreamExprID)
	}
	observedEPL := map[string]string{
		"chained-parameterized":           eplOtherStreamExprChainedEPL,
		"stream-function":                 eplOtherStreamExprFunctionA,
		"instance-method-outer-join":      eplOtherStreamExprOuterJoin,
		"instance-method-static":          eplOtherStreamExprStaticJoin,
		"stream-instance-method-aliased":  eplOtherStreamExprAliasedEPL,
		"stream-instance-method-no-alias": eplOtherStreamExprNoAliasEPL,
		"join-stream-select":              eplOtherStreamExprJoinAliased,
		"pattern-stream-select":           eplOtherStreamExprPatternEPL,
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplOtherStreamExprFields(object,
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
		if definition.Case != eplOtherStreamExprCases[index] ||
			definition.Ordinal != eplOtherStreamExprOrdinals[index] ||
			definition.RuntimeID != eplOtherStreamExprJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplOtherStreamExprJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != observedEPL[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplOtherStreamExprID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplOtherStreamExprID)
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
			if err := requireEplOtherStreamExprFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplOtherStreamExprFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplOtherStreamExprFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplOtherStreamExprPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplOtherStreamExprFields(object, "op", "case"); err != nil {
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
	if err := validateEplOtherStreamExprScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplOtherStreamExprDeployEPLs pins the exact deploy EPLs per case, in order.
func eplOtherStreamExprDeployEPLs(caseName string) []string {
	switch caseName {
	case "chained-parameterized":
		return []string{eplOtherStreamExprChainedEPL}
	case "stream-function":
		return []string{eplOtherStreamExprFunctionA, eplOtherStreamExprFunctionB, eplOtherStreamExprFunctionC, eplOtherStreamExprFunctionD}
	case "instance-method-outer-join":
		return []string{eplOtherStreamExprOuterJoin}
	case "instance-method-static":
		return []string{eplOtherStreamExprStaticJoin}
	case "stream-instance-method-aliased":
		return []string{eplOtherStreamExprAliasedEPL}
	case "stream-instance-method-no-alias":
		return []string{eplOtherStreamExprNoAliasEPL, eplOtherStreamExprMyTestSchema, eplOtherStreamExprMyTestEPL}
	case "join-stream-select":
		return []string{eplOtherStreamExprJoinAliased, eplOtherStreamExprJoinPlain}
	default:
		return []string{eplOtherStreamExprPatternEPL}
	}
}

func validateEplOtherStreamExprScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplOtherStreamExprID {
		return fmt.Errorf("%s scenario shape is not pinned", eplOtherStreamExprID)
	}
	mdACME0 := compat.Step{EventType: "SupportMarketDataBean", Payload: json.RawMessage(`{"symbol": "ACME", "price": 0, "volume": 0, "feed": null}`)}
	mdACME100 := compat.Step{EventType: "SupportMarketDataBean", Payload: json.RawMessage(`{"symbol": "ACME", "price": 0, "volume": 100, "feed": null}`)}
	mdACME99 := compat.Step{EventType: "SupportMarketDataBean", Payload: json.RawMessage(`{"symbol": "ACME", "price": 4, "volume": 99, "feed": null}`)}
	mdACME4vol2 := compat.Step{EventType: "SupportMarketDataBean", Payload: json.RawMessage(`{"symbol": "ACME", "price": 4, "volume": 2, "feed": null}`)}
	beanACME := compat.Step{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "ACME", "intPrimitive": 1}`)}
	beanNull := compat.Step{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": null, "intPrimitive": 0}`)}
	chainTop := compat.Step{EventType: "SupportChainTop", Payload: json.RawMessage(`{}`)}
	complexACME := compat.Step{EventType: "SupportBeanComplexProps", Payload: json.RawMessage(`{"simpleProperty": "ACME"}`)}
	myTest10 := compat.Step{EventType: "MyTestEvent", Payload: json.RawMessage(`{"id": 10}`)}
	s1Kickoff := compat.Step{EventType: "SupportBean_S1", Payload: json.RawMessage(`{"id": 0}`)}
	_ = s1Kickoff
	// Per-case explicit step tables (deploy label -> EPL; sends in order).
	type want struct {
		op        string
		statement string
		epl       string
		send      compat.Step
	}
	mk := func(caseName string, deploys []string, sends []compat.Step) []want {
		var out []want
		out = append(out, want{op: "case"})
		for i, epl := range deploys {
			out = append(out, want{op: "deploy", statement: epl2Label(deploys, i), epl: epl})
		}
		for _, send := range sends {
			out = append(out, want{op: "send", send: send})
		}
		out = append(out, want{op: "undeploy-all"})
		return out
	}
	_ = mk
	_ = s1Kickoff
	tables := map[string][]want{
		"chained-parameterized": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherStreamExprChainedEPL},
			{op: "send", send: chainTop},
			{op: "undeploy-all"},
		},
		"stream-function": {
			{op: "case"},
			{op: "deploy", statement: "s0a", epl: eplOtherStreamExprFunctionA},
			{op: "deploy", statement: "s0b", epl: eplOtherStreamExprFunctionB},
			{op: "deploy", statement: "s0c", epl: eplOtherStreamExprFunctionC},
			{op: "deploy", statement: "s0d", epl: eplOtherStreamExprFunctionD},
			{op: "send", send: mdACME0},
			{op: "send", send: mdACME100},
			{op: "undeploy-all"},
		},
		"instance-method-outer-join": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherStreamExprOuterJoin},
			{op: "send", send: mdACME0},
			{op: "undeploy-all"},
		},
		"instance-method-static": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherStreamExprStaticJoin},
			{op: "send", send: mdACME0},
			{op: "send", send: complexACME},
			{op: "undeploy-all"},
		},
		"stream-instance-method-aliased": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherStreamExprAliasedEPL},
			{op: "send", send: mdACME99},
			{op: "undeploy-all"},
		},
		"stream-instance-method-no-alias": {
			{op: "case"},
			{op: "deploy", statement: "s0a", epl: eplOtherStreamExprNoAliasEPL},
			{op: "send", send: mdACME4vol2},
			{op: "deploy", statement: "s0b", epl: eplOtherStreamExprMyTestSchema},
			{op: "deploy", statement: "s0c", epl: eplOtherStreamExprMyTestEPL},
			{op: "send", send: myTest10},
			{op: "undeploy-all"},
		},
		"join-stream-select": {
			{op: "case"},
			{op: "deploy", statement: "s0a", epl: eplOtherStreamExprJoinAliased},
			{op: "deploy", statement: "s0b", epl: eplOtherStreamExprJoinPlain},
			{op: "send", send: mdACME0},
			{op: "send", send: beanNull},
			{op: "undeploy-all"},
		},
		"pattern-stream-select": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherStreamExprPatternEPL},
			{op: "send", send: mdACME0},
			{op: "send", send: beanACME},
			{op: "undeploy-all"},
		},
	}
	offset := 0
	for _, caseName := range eplOtherStreamExprCases {
		expected := tables[caseName]
		steps := scenario.Steps[offset : offset+len(expected)]
		offset += len(expected)
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", eplOtherStreamExprID, caseName)
		}
		for index, want := range expected {
			step := steps[index]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q in order", eplOtherStreamExprID, caseName, index, want.op)
			}
			switch want.op {
			case "deploy":
				if step.Statement != want.statement || step.Epl != want.epl {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", eplOtherStreamExprID, caseName, index, want.statement)
				}
			case "send":
				if step.EventType != want.send.EventType {
					return fmt.Errorf("%s case %q step %d must send %q", eplOtherStreamExprID, caseName, index, want.send.EventType)
				}
				payload, err := decodeEplOtherStreamExprPayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q step %d: %w", eplOtherStreamExprID, caseName, index, err)
				}
				if err := eplOtherStreamExprPinPayload(caseName, index, step, payload, want.send); err != nil {
					return err
				}
			case "undeploy-all":
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplOtherStreamExprID)
	}
	return nil
}

func eplOtherStreamExprPinPayload(caseName string, index int, step compat.Step, payload any, want compat.Step) error {
	var wantPayload struct {
		TheString      string  `json:"theString"`
		IntPrimitive   int     `json:"intPrimitive"`
		Symbol         string  `json:"symbol"`
		Price          float64 `json:"price"`
		Volume         int64   `json:"volume"`
		SimpleProperty string  `json:"simpleProperty"`
		ID             int     `json:"id"`
	}
	if err := json.Unmarshal(want.Payload, &wantPayload); err != nil {
		return err
	}
	switch typed := payload.(type) {
	case eplOtherStreamExprMD:
		if typed.Symbol != wantPayload.Symbol {
			return fmt.Errorf("%s step %d SupportMarketDataBean payload is not pinned", eplOtherStreamExprID, index)
		}
		if typed.Price != wantPayload.Price || typed.Volume != wantPayload.Volume {
			return fmt.Errorf("%s step %d SupportMarketDataBean payload is not pinned", eplOtherStreamExprID, index)
		}
	case eplOtherStreamExprBean:
		if typed.IntPrimitive != wantPayload.IntPrimitive {
			return fmt.Errorf("%s step %d SupportBean payload is not pinned", eplOtherStreamExprID, index)
		}
		// TheString may be null in the scenario and defaults to nil; only
		// non-null pins are compared.
		if wantPayload.TheString != "" {
			if typed.TheString == nil || *typed.TheString != wantPayload.TheString {
				return fmt.Errorf("%s step %d SupportBean payload is not pinned", eplOtherStreamExprID, index)
			}
		} else if typed.TheString != nil {
			return fmt.Errorf("%s step %d SupportBean payload is not pinned", eplOtherStreamExprID, index)
		}
	case eplOtherStreamExprComplexProps:
		if typed.SimpleProperty != wantPayload.SimpleProperty {
			return fmt.Errorf("%s step %d SupportBeanComplexProps payload is not pinned", eplOtherStreamExprID, index)
		}
	case eplOtherStreamExprMyTestEvent:
		if typed.ID != wantPayload.ID {
			return fmt.Errorf("%s step %d MyTestEvent payload is not pinned", eplOtherStreamExprID, index)
		}
	case eplOtherStreamExprChainTop:
	}
	return nil
}

func epl2Label(deploys []string, i int) string {
	if len(deploys) == 1 {
		return "s0"
	}
	return fmt.Sprintf("s0%c", 'a'+rune(i))
}

func runEplOtherStreamExprScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplOtherStreamExprScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	spans := map[string]int{
		"chained-parameterized": 4, "stream-function": 8, "instance-method-outer-join": 4,
		"instance-method-static": 5, "stream-instance-method-aliased": 4,
		"stream-instance-method-no-alias": 7, "join-stream-select": 6, "pattern-stream-select": 5,
	}
	for caseIndex, caseName := range eplOtherStreamExprCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseName]]
		offset += spans[caseName]
		caseTrace, err := runEplOtherStreamExprCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplOtherStreamExprID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplOtherStreamExprCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	register := func() error {
		switch caseName {
		case "chained-parameterized":
			_, err := esper.RegisterStruct[eplOtherStreamExprChainTop](env, "SupportChainTop")
			return err
		case "instance-method-outer-join", "join-stream-select", "pattern-stream-select":
			if _, err := esper.RegisterStruct[eplOtherStreamExprMD](env, "SupportMarketDataBean"); err != nil {
				return err
			}
			_, err := esper.RegisterStruct[eplOtherStreamExprBean](env, "SupportBean")
			return err
		case "instance-method-static":
			if _, err := esper.RegisterStruct[eplOtherStreamExprMD](env, "SupportMarketDataBean"); err != nil {
				return err
			}
			_, err := esper.RegisterStruct[eplOtherStreamExprComplexProps](env, "SupportBeanComplexProps")
			return err
		case "stream-instance-method-no-alias":
			if _, err := esper.RegisterStruct[eplOtherStreamExprMD](env, "SupportMarketDataBean"); err != nil {
				return err
			}
			_, err := esper.RegisterStruct[eplOtherStreamExprMyTestEvent](env, "MyTestEvent")
			return err
		default:
			_, err := esper.RegisterStruct[eplOtherStreamExprMD](env, "SupportMarketDataBean")
			return err
		}
	}
	if err := register(); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplOtherStreamExprJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: eplOtherStreamExprID}
	sequence := uint64(0)
	record := func(batch esper.ResultBatch) {
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

	buildAndDeploy := func(label string, epl string) error {
		var plan esper.Plan
		var err error
		switch caseName + "/" + label {
		case "chained-parameterized/s0":
			// Chained parameterized event methods; the no-alias output name
			// is the verbatim chain text.
			plan, err = env.Build(esper.Select(
				esper.From[eplOtherStreamExprChainTop](env, "SupportChainTop"),
				esper.Alias(`top.getChildOne("abc",10).getChildTwo("append")`,
					esper.Method[string](
						esper.Method[eplOtherStreamExprChainChild](
							esper.EventValue[eplOtherStreamExprChainTop](),
							"GetChildOne", esper.Literal("abc"), esper.Literal(10)),
						"GetChildTwo", esper.Literal("append"))),
			).Query(esper.StatementName("s0")))
		case "stream-function/s0a", "stream-function/s0b", "stream-function/s0c", "stream-function/s0d":
			// Static-method where filters; the four Java argument forms
			// (stream name, wildcard, EventBean-typed variants) share the
			// same single-event surface on one stream. Each variant deploys
			// as its own statement named s0 (matching the Java @name), and
			// each deployment subscribes separately.
			predicate := esper.Func1[esper.Event, bool](
				"volumeGreaterZero",
				func(event esper.Event) bool {
					return eplOtherStreamExprVolumeGreaterZeroEvent(event)
				},
				esper.EventValue[esper.Event]())
			plan, err = env.Build(esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean").
				Filter(predicate).Query(esper.StatementName("s0")))
		case "instance-method-outer-join/s0":
			plan, err = env.Build(esper.Join(
				esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean").Window(esper.KeepAll()),
				esper.From[eplOtherStreamExprBean](env, "SupportBean").Window(esper.KeepAll()),
				esper.OnEqual(
					esper.Field[eplOtherStreamExprMD, string]("symbol"),
					esper.Field[eplOtherStreamExprBean, string]("theString")),
			).LeftOuter().Select(
				esper.SelectFrom(0, "symbol", esper.Field[eplOtherStreamExprMD, string]("symbol")),
				esper.SelectFrom(1, "theString", esper.Method[string](
					esper.EventValue[eplOtherStreamExprBean](), "GetTheString")),
			).Query(esper.StatementName("s0")))
		case "instance-method-static/s0":
			plan, err = env.Build(esper.Join(
				esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean").Window(esper.KeepAll()),
				esper.From[eplOtherStreamExprComplexProps](env, "SupportBeanComplexProps").Window(esper.KeepAll()),
				esper.OnEqual(
					esper.Field[eplOtherStreamExprMD, string]("symbol"),
					esper.Field[eplOtherStreamExprComplexProps, string]("simpleProperty")),
			).LeftOuter().Select(
				esper.SelectFrom(0, "symbol", esper.Field[eplOtherStreamExprMD, string]("symbol")),
				esper.SelectFrom(1, "simpleprop", esper.Method[string](
					esper.EventValue[eplOtherStreamExprComplexProps](), "GetSimpleProperty")),
				esper.SelectFrom(1, "def", esper.Method[eplOtherStreamExprComplexProps](
					esper.EventValue[eplOtherStreamExprComplexProps](), "MakeDefaultBean")),
			).Query(esper.StatementName("s0")))
		case "stream-instance-method-aliased/s0":
			plan, err = env.Build(esper.Select(
				esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean"),
				esper.Alias("volume", esper.Method[int64](
					esper.EventValue[eplOtherStreamExprMD](), "GetVolume")),
				esper.Alias("symbol", esper.Method[string](
					esper.EventValue[eplOtherStreamExprMD](), "GetSymbol")),
				esper.Alias("pvf", esper.Method[float64](
					esper.EventValue[eplOtherStreamExprMD](),
					"GetPriceTimesVolume", esper.Literal(2.0))),
			).Query(esper.StatementName("s0")))
		case "stream-instance-method-no-alias/s0a":
			plan, err = env.Build(esper.Select(
				esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean"),
				esper.Alias("s0.getVolume()", esper.Method[int64](
					esper.EventValue[eplOtherStreamExprMD](), "GetVolume")),
				esper.Alias("s0.getPriceTimesVolume(3)", esper.Method[float64](
					esper.EventValue[eplOtherStreamExprMD](),
					"GetPriceTimesVolume", esper.Literal(3.0))),
			).Query(esper.StatementName("s0")))
		case "stream-instance-method-no-alias/s0b", "stream-instance-method-no-alias/s0c":
			// The create-schema step is a catalog operation (MyTestEvent is
			// registered at env level); only the consumer statement deploys.
			if label != "s0c" {
				return nil
			}
			plan, err = env.Build(esper.Select(
				esper.From[eplOtherStreamExprMyTestEvent](env, "MyTestEvent"),
				esper.Alias("c0", esper.Method[int](
					esper.EventValue[eplOtherStreamExprMyTestEvent](),
					"GetValueAsInt", esper.EventValue[esper.Event](), esper.Literal("id"))),
				esper.Alias("c1", esper.Method[int](
					esper.EventValue[eplOtherStreamExprMyTestEvent](),
					"GetValueAsInt", esper.EventValue[esper.Event](), esper.Literal("id"))),
			).Query(esper.StatementName("s0")))
		case "join-stream-select/s0a", "join-stream-select/s0b":
			if label == "s0a" {
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean").Window(esper.KeepAll())),
					esper.JoinSource(esper.From[eplOtherStreamExprBean](env, "SupportBean").Window(esper.KeepAll())),
				).Select(
					esper.SelectSourceEvent(0, "s0stream"),
					esper.SelectSourceEvent(1, "s1stream"),
				).Query(esper.StatementName("s0a")))
			} else {
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean").Window(esper.KeepAll())),
					esper.JoinSource(esper.From[eplOtherStreamExprBean](env, "SupportBean").Window(esper.KeepAll())),
				).Select(
					esper.SelectSourceEvent(0, "s0"),
					esper.SelectSourceEvent(1, "s1"),
				).Query(esper.StatementName("s0b")))
			}
		case "pattern-stream-select/s0":
			// The followed-by second leg carries a static-UDF filter that
			// compares the two captured tags via their properties.
			second := esper.PatternFrom(
				esper.From[eplOtherStreamExprBean](env, "SupportBean"), "e2",
				esper.Equal[string](
					esper.Property[string](esper.PatternEvent("e1"), "symbol"),
					esper.Property[string](esper.PatternEvent("e2"), "theString")))
			pattern := esper.PatternFrom(
				esper.From[eplOtherStreamExprMD](env, "SupportMarketDataBean"), "e1", esper.Literal(true)).Then(second)
			plan, err = env.Build(pattern.Select(
				esper.Alias("e1", esper.PatternEvent("e1")),
				esper.Alias("e2", esper.PatternEvent("e2")),
			).Query(esper.StatementName("s0")))
		default:
			return fmt.Errorf("unexpected deploy %q for case %q", label, caseName)
		}
		if err != nil {
			return fmt.Errorf("build %q: %w", label, err)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("deploy %q: %w", label, err)
		}
		for _, statement := range deployment.Statements() {
			// Subscribe to the s0 consumer statements (s0a/s0b in the
			// join-stream-select case where two statements share a case).
			if statement.Name() == "s0" || statement.Name() == "s0a" || statement.Name() == "s0b" {
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

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := buildAndDeploy(step.Statement, step.Epl); err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
		case "send":
			payload, err := decodeEplOtherStreamExprPayload(step)
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

func decodeEplOtherStreamExprPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportMarketDataBean":
		if err := requireEplOtherStreamExprFields(fields, "symbol", "price", "volume", "feed"); err != nil {
			return nil, err
		}
		var md eplOtherStreamExprMD
		if err := json.Unmarshal(step.Payload, &md); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return md, nil
	case "SupportBean":
		if err := requireEplOtherStreamExprFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean eplOtherStreamExprBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportChainTop":
		if err := requireEplOtherStreamExprFields(fields); err != nil {
			return nil, err
		}
		return eplOtherStreamExprChainTop{}, nil
	case "SupportBeanComplexProps":
		if err := requireEplOtherStreamExprFields(fields, "simpleProperty"); err != nil {
			return nil, err
		}
		var props eplOtherStreamExprComplexProps
		if err := json.Unmarshal(step.Payload, &props); err != nil {
			return nil, fmt.Errorf("decode SupportBeanComplexProps: %w", err)
		}
		return props, nil
	case "MyTestEvent":
		if err := requireEplOtherStreamExprFields(fields, "id"); err != nil {
			return nil, err
		}
		var event eplOtherStreamExprMyTestEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode MyTestEvent: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplOtherStreamExprID, step.EventType)
	}
}

func requireEplOtherStreamExprFields(object map[string]json.RawMessage, names ...string) error {
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

func validateEplOtherStreamExprStringArray(raw json.RawMessage, expected []string, name string) error {
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
