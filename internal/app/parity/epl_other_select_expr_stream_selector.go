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
	eplOtherSelectExprStreamSelectorID          = "epl-other-select-expr-stream-selector"
	eplOtherSelectExprStreamSelectorDescription = "EPLOtherSelectExprStreamSelector alias-with-properties executions: stream-dot notation with alias (theString.* as s0/s1) beside plain property aliases (intPrimitive as a/b) over a length window, and mixed join select with stream-as-object columns (s0stream/s1stream), plain properties (intPrimitive, theString), and aliased columns (symbol as sym) over a length-keepall inner join (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java)."
	eplOtherSelectExprStreamSelectorJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplOtherSelectExprStreamSelectorSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java"

	eplOtherSelectExprStreamSelectorNoJoinEPL = "@name('s0') select theString.* as s0, intPrimitive as a, theString.* as s1, intPrimitive as b from SupportBean#length(3) as theString"
	eplOtherSelectExprStreamSelectorJoinEPL   = "@name('s0') select intPrimitive, s1.* as s1stream, theString, symbol as sym, s0.* as s0stream from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1"
)

var (
	eplOtherSelectExprStreamSelectorJavaSources = []string{
		eplOtherSelectExprStreamSelectorSource,
	}
	eplOtherSelectExprStreamSelectorJavaRuntimeIDs = []string{
		"java-runtime-89123cf55af0a7f5987e",
		"java-runtime-b53494cb36a6b54c2c6f",
	}
	eplOtherSelectExprStreamSelectorJavaExecutions = []string{
		"EPLOtherNoJoinWithAliasWithProperties",
		"EPLOtherJoinWithAliasWithProperties",
	}
	eplOtherSelectExprStreamSelectorJavaStaticIDs = []string{
		"java-b5352434faea014585a8",
		"java-9682b92ad00f7295162d",
	}
	eplOtherSelectExprStreamSelectorCases = []string{
		"no-join-alias-props",
		"join-alias-props",
	}
	eplOtherSelectExprStreamSelectorOrdinals = []int{8, 9}
)

// Harness-local mirrors of the Java event types.
type eplOtherSelectExprStreamSelectorBean struct {
	TheString    *string `esper:"theString"`
	IntPrimitive int     `esper:"intPrimitive"`
}

type eplOtherSelectExprStreamSelectorMD struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

func loadEplOtherSelectExprStreamSelectorScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherSelectExprStreamSelectorID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherSelectExprStreamSelectorID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherSelectExprStreamSelectorID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherSelectExprStreamSelectorID, err)
	}
	if err := requireEplOtherSelectExprStreamSelectorFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherSelectExprStreamSelectorID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherSelectExprStreamSelectorID ||
		metadata.Description != eplOtherSelectExprStreamSelectorDescription ||
		metadata.JavaCommit != eplOtherSelectExprStreamSelectorJavaCommit ||
		metadata.JavaSource != eplOtherSelectExprStreamSelectorSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplOtherSelectExprStreamSelectorID)
	}
	if err := validateEplOtherSelectExprStreamSelectorStringArray(root["javaRuntimes"], eplOtherSelectExprStreamSelectorJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherSelectExprStreamSelectorStringArray(root["javaNames"], eplOtherSelectExprStreamSelectorJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherSelectExprStreamSelectorStringArray(root["javaStaticIds"], eplOtherSelectExprStreamSelectorJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherSelectExprStreamSelectorStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplOtherSelectExprStreamSelectorCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly two cases", eplOtherSelectExprStreamSelectorID)
	}
	observedEPL := map[string]string{
		"no-join-alias-props": eplOtherSelectExprStreamSelectorNoJoinEPL,
		"join-alias-props":    eplOtherSelectExprStreamSelectorJoinEPL,
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplOtherSelectExprStreamSelectorFields(object,
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
		if definition.Case != eplOtherSelectExprStreamSelectorCases[index] ||
			definition.Ordinal != eplOtherSelectExprStreamSelectorOrdinals[index] ||
			definition.RuntimeID != eplOtherSelectExprStreamSelectorJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplOtherSelectExprStreamSelectorJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != observedEPL[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplOtherSelectExprStreamSelectorID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplOtherSelectExprStreamSelectorID)
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
			if err := requireEplOtherSelectExprStreamSelectorFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplOtherSelectExprStreamSelectorFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplOtherSelectExprStreamSelectorFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplOtherSelectExprStreamSelectorPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplOtherSelectExprStreamSelectorFields(object, "op", "case"); err != nil {
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
	if err := validateEplOtherSelectExprStreamSelectorScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateEplOtherSelectExprStreamSelectorScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplOtherSelectExprStreamSelectorID {
		return fmt.Errorf("%s scenario shape is not pinned", eplOtherSelectExprStreamSelectorID)
	}
	beanE1 := compat.Step{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "E1", "intPrimitive": 12}`)}
	beanE1p13 := compat.Step{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "E1", "intPrimitive": 13}`)}
	mdE2 := compat.Step{EventType: "SupportMarketDataBean", Payload: json.RawMessage(`{"symbol": "E2", "price": 0, "volume": 0, "feed": ""}`)}
	// Per-case explicit step tables (deploy EPL; sends in order).
	type want struct {
		op        string
		statement string
		epl       string
		send      compat.Step
	}
	tables := map[string][]want{
		"no-join-alias-props": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherSelectExprStreamSelectorNoJoinEPL},
			{op: "send", send: beanE1},
			{op: "undeploy-all"},
		},
		"join-alias-props": {
			{op: "case"},
			{op: "deploy", statement: "s0", epl: eplOtherSelectExprStreamSelectorJoinEPL},
			// The bean send alone leaves the inner join incomplete and must
			// not produce output; the market data send completes it.
			{op: "send", send: beanE1p13},
			{op: "send", send: mdE2},
			{op: "undeploy-all"},
		},
	}
	offset := 0
	for _, caseName := range eplOtherSelectExprStreamSelectorCases {
		expected := tables[caseName]
		steps := scenario.Steps[offset : offset+len(expected)]
		offset += len(expected)
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", eplOtherSelectExprStreamSelectorID, caseName)
		}
		for index, want := range expected {
			step := steps[index]
			if step.Op != want.op || step.Case != caseName {
				return fmt.Errorf("%s case %q step %d must be op %q in order", eplOtherSelectExprStreamSelectorID, caseName, index, want.op)
			}
			switch want.op {
			case "deploy":
				if step.Statement != want.statement || step.Epl != want.epl {
					return fmt.Errorf("%s case %q step %d deploy %q is not pinned", eplOtherSelectExprStreamSelectorID, caseName, index, want.statement)
				}
			case "send":
				if step.EventType != want.send.EventType {
					return fmt.Errorf("%s case %q step %d must send %q", eplOtherSelectExprStreamSelectorID, caseName, index, want.send.EventType)
				}
				payload, err := decodeEplOtherSelectExprStreamSelectorPayload(step)
				if err != nil {
					return fmt.Errorf("%s case %q step %d: %w", eplOtherSelectExprStreamSelectorID, caseName, index, err)
				}
				if err := eplOtherSelectExprStreamSelectorPinPayload(index, payload, want.send); err != nil {
					return err
				}
			case "undeploy-all":
			}
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplOtherSelectExprStreamSelectorID)
	}
	return nil
}

func eplOtherSelectExprStreamSelectorPinPayload(index int, payload any, want compat.Step) error {
	var wantPayload struct {
		TheString    string `json:"theString"`
		IntPrimitive int    `json:"intPrimitive"`
		Symbol       string `json:"symbol"`
		Price        int64  `json:"price"`
		Volume       int64  `json:"volume"`
		Feed         string `json:"feed"`
	}
	if err := json.Unmarshal(want.Payload, &wantPayload); err != nil {
		return err
	}
	switch typed := payload.(type) {
	case eplOtherSelectExprStreamSelectorBean:
		if typed.IntPrimitive != wantPayload.IntPrimitive {
			return fmt.Errorf("%s step %d SupportBean payload is not pinned", eplOtherSelectExprStreamSelectorID, index)
		}
		if typed.TheString == nil || *typed.TheString != wantPayload.TheString {
			return fmt.Errorf("%s step %d SupportBean payload is not pinned", eplOtherSelectExprStreamSelectorID, index)
		}
	case eplOtherSelectExprStreamSelectorMD:
		if typed.Symbol != wantPayload.Symbol || typed.Price != float64(wantPayload.Price) ||
			typed.Volume != wantPayload.Volume {
			return fmt.Errorf("%s step %d SupportMarketDataBean payload is not pinned", eplOtherSelectExprStreamSelectorID, index)
		}
		// Feed is a pointer so null stays distinct from empty, exactly like
		// the Java String field; every pinned send in this unit is non-null.
		if typed.Feed == nil || *typed.Feed != wantPayload.Feed {
			return fmt.Errorf("%s step %d SupportMarketDataBean payload is not pinned", eplOtherSelectExprStreamSelectorID, index)
		}
	}
	return nil
}

func runEplOtherSelectExprStreamSelectorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplOtherSelectExprStreamSelectorScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	spans := map[string]int{
		"no-join-alias-props": 4,
		"join-alias-props":    5,
	}
	for caseIndex, caseName := range eplOtherSelectExprStreamSelectorCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseName]]
		offset += spans[caseName]
		caseTrace, err := runEplOtherSelectExprStreamSelectorCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplOtherSelectExprStreamSelectorID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplOtherSelectExprStreamSelectorCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherSelectExprStreamSelectorBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherSelectExprStreamSelectorMD](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplOtherSelectExprStreamSelectorJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: eplOtherSelectExprStreamSelectorID}
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
		switch caseName {
		case "no-join-alias-props":
			// Stream-dot alias select: theString.* projects the whole stream
			// event under the s0/s1 aliases beside plain property aliases.
			plan, err = env.Build(esper.Select(
				esper.From[eplOtherSelectExprStreamSelectorBean](env, "SupportBean").
					Window(esper.LengthWindow(3)),
				esper.Alias("s0", esper.EventValue[esper.Event]()),
				esper.Alias("a", esper.Field[eplOtherSelectExprStreamSelectorBean, int]("intPrimitive")),
				esper.Alias("s1", esper.EventValue[esper.Event]()),
				esper.Alias("b", esper.Field[eplOtherSelectExprStreamSelectorBean, int]("intPrimitive")),
			).Query(esper.StatementName("s0")))
		case "join-alias-props":
			// Mixed join select: plain and aliased properties beside
			// stream-as-object columns from both join sources.
			plan, err = env.Build(esper.JoinMany(
				esper.JoinSource(esper.From[eplOtherSelectExprStreamSelectorBean](env, "SupportBean").
					Window(esper.LengthWindow(3))),
				esper.JoinSource(esper.From[eplOtherSelectExprStreamSelectorMD](env, "SupportMarketDataBean").
					Window(esper.KeepAll())),
			).Select(
				esper.SelectFrom(0, "intPrimitive", esper.Field[eplOtherSelectExprStreamSelectorBean, int]("intPrimitive")),
				esper.SelectSourceEvent(1, "s1stream"),
				esper.SelectFrom(0, "theString", esper.Field[eplOtherSelectExprStreamSelectorBean, string]("theString")),
				esper.SelectFrom(1, "sym", esper.Field[eplOtherSelectExprStreamSelectorMD, string]("symbol")),
				esper.SelectSourceEvent(0, "s0stream"),
			).Query(esper.StatementName("s0")))
		default:
			return fmt.Errorf("unexpected case %q", caseName)
		}
		if err != nil {
			return fmt.Errorf("build %q: %w", label, err)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("deploy %q: %w", label, err)
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

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			if err := buildAndDeploy(step.Statement, step.Epl); err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
		case "send":
			payload, err := decodeEplOtherSelectExprStreamSelectorPayload(step)
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

func decodeEplOtherSelectExprStreamSelectorPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireEplOtherSelectExprStreamSelectorFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean eplOtherSelectExprStreamSelectorBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireEplOtherSelectExprStreamSelectorFields(fields, "symbol", "price", "volume", "feed"); err != nil {
			return nil, err
		}
		var md eplOtherSelectExprStreamSelectorMD
		if err := json.Unmarshal(step.Payload, &md); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return md, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplOtherSelectExprStreamSelectorID, step.EventType)
	}
}

func requireEplOtherSelectExprStreamSelectorFields(object map[string]json.RawMessage, names ...string) error {
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

func validateEplOtherSelectExprStreamSelectorStringArray(raw json.RawMessage, expected []string, name string) error {
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
