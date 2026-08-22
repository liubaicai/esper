package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type streamSelectorBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

var streamSelectorJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherIStreamRStreamConfigSelectorIRStream.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherIStreamRStreamConfigSelectorRStream.java",
}

var (
	streamSelectorJavaRuntimeIDs = []string{
		"java-runtime-6dbc99a56abddbf3d38a",
		"java-runtime-6556dd46777be4afa86f",
		"java-runtime-70753b738456d049f005",
		"java-runtime-bc1c24102a3db1950ec8",
		"java-runtime-04453c1c3c73e4a9ba36",
		"java-runtime-dccd7d03c70875f69975",
		"java-runtime-bb3332c825b24eb5046a",
		"java-runtime-4e3a14053213c9ffd08d",
		"java-runtime-4a8e3f2dc067ae2b7602",
		"java-runtime-f7193a89174e1e9c9ca0",
		"java-runtime-043dced54921cbf3b571",
		"java-runtime-1e4ca2cb69bc79b30c91",
	}
	streamSelectorJavaExecutions = []string{
		"EPLOtherNoJoinWildcardNoAlias",
		"EPLOtherJoinWildcardNoAlias",
		"EPLOtherNoJoinWildcardWithAlias",
		"EPLOtherJoinWildcardWithAlias",
		"EPLOtherNoJoinNoAliasWithProperties",
		"EPLOtherJoinNoAliasWithProperties",
		"EPLOtherAloneNoJoinNoAlias",
		"EPLOtherAloneNoJoinAlias",
		"EPLOtherAloneJoinAlias",
		"EPLOtherAloneJoinNoAlias",
		"EPLOtherIStreamRStreamConfigSelectorIRStream",
		"EPLOtherIStreamRStreamConfigSelectorRStream",
	}
)

// runStreamSelectorScenario replays the stream-selector family: wildcard,
// stream-alias and named-property projections over single streams and joins,
// reverse-stream second generations, and the engine-default istream/rstream
// config selectors (mirrored per statement, observationally identical for a
// single-statement deployment).
func runStreamSelectorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	caseOrder := []string{
		"nojoin-wildcard-noalias", "join-wildcard-noalias",
		"nojoin-wildcard-alias", "join-wildcard-alias",
		"nojoin-noalias-withproperties", "join-noalias-withproperties",
		"alone-nojoin-noalias", "alone-nojoin-alias",
		"alone-join-alias", "alone-join-alias-reverse",
		"alone-join-noalias", "alone-join-noalias-reverse",
		"config-selector-istream", "config-selector-rstream",
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range caseOrder {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runStreamSelectorCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("stream-selector case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("stream-selector scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runStreamSelectorCase(ctx context.Context, caseScenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[streamSelectorBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "SupportMarketDataBean", []esper.FieldSpec{
		esper.FieldDef("symbol", reflect.TypeOf("")),
		esper.FieldDef("price", reflect.TypeOf(float64(0))),
		esper.FieldDef("volume", reflect.TypeOf(int64(0))),
	}); err != nil {
		return compat.Trace{}, err
	}

	str := esper.Field[streamSelectorBean, string]("theString")
	num := esper.Field[streamSelectorBean, int32]("intPrimitive")
	market := esper.FromAny(env, "SupportMarketDataBean")
	symbol := esper.Field[any, string]("symbol")
	volume := esper.Field[any, int64]("volume")

	base := esper.From[streamSelectorBean](env, "SupportBean")
	var query esper.Query
	switch caseName {
	case "nojoin-wildcard-noalias":
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(esper.StatementName("s0"))
	case "nojoin-wildcard-alias":
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
			esper.Alias("s0", esper.EventValue[esper.Event]()),
		).Query(esper.StatementName("s0"))
	case "alone-nojoin-noalias":
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("theString", str),
		).Query(esper.StatementName("s0"))
	case "alone-nojoin-alias":
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("s0", esper.EventValue[esper.Event]()),
		).Query(esper.StatementName("s0"))
	case "join-wildcard-noalias", "alone-join-noalias", "alone-join-noalias-reverse":
		left := base.Window(esper.LengthWindow(3))
		right := market.Window(esper.KeepAll())
		var selections []esper.JoinSelection
		switch caseName {
		case "join-wildcard-noalias":
			selections = []esper.JoinSelection{
				esper.SelectSourceEvent(0, "s0"),
				esper.SelectSourceEvent(1, "s1"),
				esper.SelectFrom(1, "symbol", symbol),
				esper.SelectFrom(1, "volume", volume),
			}
		case "alone-join-noalias":
			// Unaliased s1.* flattens the market columns.
			selections = []esper.JoinSelection{
				esper.SelectFrom(1, "symbol", symbol),
			}
		default: // alone-join-noalias-reverse
			// Unaliased s0.* flattens the bean columns.
			selections = []esper.JoinSelection{
				esper.SelectFrom(0, "theString", str),
			}
		}
		// The Java statements carry no where clause: an unconditional join.
		query = esper.JoinMany(
			esper.JoinSource(left),
			esper.JoinRecordSource(right),
		).Select(selections...).Query(esper.StatementName("s0"))
	case "join-wildcard-alias", "alone-join-alias", "alone-join-alias-reverse":
		left := base.Window(esper.LengthWindow(3))
		right := market.Window(esper.KeepAll())
		var selections []esper.JoinSelection
		switch caseName {
		case "join-wildcard-alias":
			selections = []esper.JoinSelection{
				esper.SelectSourceEvent(0, "s0"),
				esper.SelectSourceEvent(1, "s1"),
				esper.SelectSourceEvent(0, "s0stream"),
				esper.SelectSourceEvent(1, "s1stream"),
			}
		case "alone-join-alias":
			selections = []esper.JoinSelection{
				esper.SelectSourceEvent(1, "s1"),
			}
		default: // alone-join-alias-reverse
			selections = []esper.JoinSelection{
				esper.SelectSourceEvent(0, "szero"),
			}
		}
		// The Java statements carry no where clause: an unconditional join.
		query = esper.JoinMany(
			esper.JoinSource(left),
			esper.JoinRecordSource(right),
		).Select(selections...).Query(esper.StatementName("s0"))
	case "nojoin-noalias-withproperties":
		// Unaliased string.* flattens every bean column; the assertion
		// surface includes intPrimitive alongside the named a/b aliases.
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("a", num),
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
			esper.Alias("b", num),
		).Query(esper.StatementName("s0"))
	case "join-noalias-withproperties":
		// The Java statement carries no where clause: an unconditional join.
		query = esper.JoinMany(
			esper.JoinSource(base.Window(esper.LengthWindow(3))),
			esper.JoinRecordSource(market.Window(esper.KeepAll())),
		).Select(
			esper.SelectFrom(0, "intPrimitive", num),
			esper.SelectFrom(1, "symbol", symbol),
			esper.SelectFrom(1, "sym", symbol),
		).Query(esper.StatementName("s0"))
	case "config-selector-istream":
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			// Engine-default RSTREAM_ISTREAM_BOTH mirrored per statement:
			// both insert- and remove-stream rows are delivered.
			esper.WithOldStream(),
		)
	case "config-selector-rstream":
		query = esper.Select(base.Window(esper.LengthWindow(3)),
			esper.Alias("theString", str),
			esper.Alias("intPrimitive", num),
		).Query(
			esper.StatementName("s0"),
			// Engine-default RSTREAM_ONLY mirrored per statement: removed
			// rows are delivered on the insert channel.
			esper.WithRemoveStreamOnly(),
		)
	default:
		return compat.Trace{}, fmt.Errorf("unsupported stream-selector case %q", caseName)
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

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seq := uint64(0)
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		seq++
		record := compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  seq,
			Time:      currentTimeString(engine),
		}
		for _, row := range batch.New {
			fields, err := streamSelectorSurfaceRow(row)
			if err != nil {
				return err
			}
			record.New = append(record.New, compat.ResultRecord{Kind: "row", Fields: fields})
		}
		for _, row := range batch.Old {
			fields, err := streamSelectorSurfaceRow(row)
			if err != nil {
				return err
			}
			record.Old = append(record.Old, compat.ResultRecord{Kind: "row", Fields: fields})
		}
		trace.Records = append(trace.Records, record)
		return nil
	}); err != nil {
		return trace, err
	}
	for _, step := range caseScenario.Steps {
		if step.Op == "case" {
			continue
		}
		if step.Op != "send" {
			return trace, fmt.Errorf("unsupported stream-selector step op %q", step.Op)
		}
		payload, err := decodeStreamSelectorPayload(step)
		if err != nil {
			return trace, err
		}
		if err := engine.Send(ctx, step.EventType, payload); err != nil {
			return trace, err
		}
	}
	return trace, nil
}

// streamSelectorSurfaceRow projects one output row onto the execution's
// assertion surface, rendering event and map columns deterministically.
func streamSelectorSurfaceRow(result esper.Result) (map[string]any, error) {
	fields := make(map[string]any)
	collect := func(name string, value esper.Value) {
		fields[name] = streamSelectorRender(value.Any())
	}
	if event, ok := result.Event(); ok {
		for _, field := range event.Schema().Fields() {
			collect(field.Name, event.Get(field.Name))
		}
		return fields, nil
	}
	row, ok := result.Row()
	if !ok {
		return nil, fmt.Errorf("stream-selector result is neither event nor row")
	}
	for _, field := range row.Schema().Fields() {
		collect(field.Name, row.Get(field.Name))
	}
	return fields, nil
}

// streamSelectorRender mirrors the Java oracle's deterministic rendering:
// SupportBean columns as [k=v,...], map columns as sorted {k=v;...} with
// doubles carrying at least one decimal place.
func streamSelectorRender(value any) any {
	switch typed := value.(type) {
	case nil:
		return "null"
	case esper.Event:
		return streamSelectorRender(typed.Underlying())
	case *streamSelectorBean:
		return fmt.Sprintf("SupportBean[theString=%s,intPrimitive=%d]", typed.TheString, typed.IntPrimitive)
	case streamSelectorBean:
		return fmt.Sprintf("SupportBean[theString=%s,intPrimitive=%d]", typed.TheString, typed.IntPrimitive)
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			rendered := streamSelectorRender(typed[key])
			text, ok := rendered.(string)
			if !ok {
				text = fmt.Sprintf("%v", rendered)
			}
			parts = append(parts, key+"="+text)
		}
		return "{" + strings.Join(parts, ";") + "}"
	case float64:
		// Mirror Java Double.toString: shortest round-trip form, always with
		// a fractional part for integral values, upper-case exponent.
		rendered := strconv.FormatFloat(typed, 'g', -1, 64)
		if !strings.ContainsAny(rendered, ".eE") {
			rendered += ".0"
		}
		return strings.Replace(rendered, "e", "E", 1)
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func decodeStreamSelectorPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var event streamSelectorBean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("stream-selector SupportBean: %w", err)
		}
		return event, nil
	case "SupportMarketDataBean":
		var payload struct {
			Symbol string  `json:"symbol"`
			Price  float64 `json:"price"`
			Volume int64   `json:"volume"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("stream-selector SupportMarketDataBean: %w", err)
		}
		return map[string]any{"symbol": payload.Symbol, "price": payload.Price, "volume": payload.Volume}, nil
	default:
		return nil, fmt.Errorf("stream-selector: unsupported event type %q", step.EventType)
	}
}
