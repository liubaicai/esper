package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for EPLOtherSelectWildcardWAdditional.
// Approved differences: Java Pair underlying vs Go flat rows; SODA OM has no
// Go counterpart (single-om replays the same listener contract as single);
// InvalidRepeatedProperties message text unasserted; Java's two sequential
// deploy cycles for join-no-common/join-common flatten into one module with
// s0 (no where) + s1 (where) deployed together; insert-into targets are
// pre-registered map types (Go requires the target type to exist).

const eplOtherWildcardAdditionalJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherWildcardAdditionalJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectWildcardWAdditional.java",
}

type wcwSimple struct {
	MyString string `esper:"myString"`
	MyInt    int32  `esper:"myInt"`
}

type wcwMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   *string `esper:"feed"`
}

type wcwBeanA struct {
	ID string `esper:"id"`
}

type wcwBeanB struct {
	ID string `esper:"id"`
}

type wcwMapEvent struct {
	TheString string `esper:"theString"`
	Int       int32  `esper:"int"`
}

// wcwNestedTwo mirrors SupportBeanCombinedProps.NestedLevTwo.
type wcwNestedTwo struct {
	Value string `esper:"value"`
}

// wcwNestedOne mirrors SupportBeanCombinedProps.NestedLevOne: the Java
// getMapped(key) method is expressed as the mapprop map field, and
// getNestLevOneVal() is a constant "abc" supplied by the runner.
type wcwNestedOne struct {
	Mapprop       map[string]wcwNestedTwo `esper:"mapprop"`
	NestLevOneVal string                  `esper:"nestLevOneVal"`
}

// wcwCombined mirrors SupportBeanCombinedProps. getIndexed(int) and
// getArray() both expose the same slice; Java's wildcard row carries both
// columns but plain get("indexed") is unreadable (indexed-only property).
type wcwCombined struct {
	Indexed []*wcwNestedOne `esper:"indexed"`
	Array   []*wcwNestedOne `esper:"array"`
}

var eplOtherWildcardAdditionalJavaRuntimeIDs = []string{
	"java-runtime-bcacb282274dc1ea256b",
	"java-runtime-0d7fa958a52fab1f4f47",
	"java-runtime-0fe48a706db1e3cde7f0",
	"java-runtime-d85b62bf7cff58f9ed71",
	"java-runtime-948405f7925b1af087bd",
	"java-runtime-98f8ad1516096842f30d",
	"java-runtime-e9caf805d44d7a1f0ad1",
	"java-runtime-e9aa163116193bbd7106",
	"java-runtime-33e95a630f502ba99b94",
}

var eplOtherWildcardAdditionalJavaExecutions = []string{
	"EPLOtherSingleOM",
	"EPLOtherSingle",
	"EPLOtherSingleInsertInto",
	"EPLOtherJoinInsertInto",
	"EPLOtherJoinNoCommonProperties",
	"EPLOtherJoinCommonProperties",
	"EPLOtherCombinedProperties",
	"EPLOtherWildcardMapEvent",
	"EPLOtherInvalidRepeatedProperties",
}

func runEplOtherWildcardAdditionalScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, caseName := range []string{
		"single-om", "single", "single-insert-into", "join-insert-into",
		"join-no-common", "join-common", "combined-props", "wildcard-map",
		"invalid-repeated",
	} {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		caseTrace, err := runEplOtherWildcardAdditionalCase(ctx, scenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runEplOtherWildcardAdditionalCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[wcwSimple](env, "SupportBeanSimple"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[wcwMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[wcwBeanA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[wcwBeanB](env, "SupportBean_B"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[wcwCombined](env, "SupportBeanCombinedProps"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[wcwMapEvent](env, "MyMapEventIntString"); err != nil {
		return compat.Trace{}, err
	}
	eventType := reflect.TypeOf(esper.Event{})
	if _, err := esper.RegisterMap(env, "SomeEvent", []esper.FieldSpec{
		esper.FieldDef("myString", reflect.TypeOf("")),
		esper.FieldDef("myInt", reflect.TypeOf(int32(0))),
		esper.FieldDef("concat", reflect.TypeOf("")),
	}); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "SomeJoinEvent", []esper.FieldSpec{
		esper.FieldDef("eventOne", eventType),
		esper.FieldDef("eventTwo", eventType),
		esper.FieldDef("concat", reflect.TypeOf("")),
	}); err != nil {
		return compat.Trace{}, err
	}

	var plans []esper.Plan
	build := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	}

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	simpleWildcard := func() esper.Query {
		ms := esper.Field[wcwSimple, string]("myString")
		mi := esper.Field[wcwSimple, int32]("myInt")
		return esper.Select(
			esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5)),
			esper.Alias("myString", ms),
			esper.Alias("myInt", mi),
			esper.Alias("concat", esper.Concat(ms, ms)),
		).Query(esper.StatementName("s0"))
	}
	switch caseName {
	case "single-om":
		// Go has no SODA object model; the Java oracle emits only listener
		// records for this case, so the EPL twin replays the same contract.
		err = build(simpleWildcard())
	case "single":
		err = build(simpleWildcard())
	case "single-insert-into":
		ms := esper.Field[wcwSimple, string]("myString")
		mi := esper.Field[wcwSimple, int32]("myInt")
		if err = build(esper.Select(
			esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5)),
			esper.Alias("myString", ms),
			esper.Alias("myInt", mi),
			esper.Alias("concat", esper.Concat(ms, ms)),
		).InsertInto("SomeEvent", esper.StatementName("insert"))); err != nil {
			break
		}
		err = build(esper.FromAny(env, "SomeEvent").Window(esper.LengthWindow(5)).
			Select(
				esper.Alias("myString", esper.Field[any, string]("myString")),
				esper.Alias("myInt", esper.Field[any, int32]("myInt")),
				esper.Alias("concat", esper.Field[any, string]("concat")),
			).Query(esper.StatementName("s0")))
	case "join-insert-into":
		ms := esper.JoinField[string](0, "myString")
		if err = build(esper.JoinMany(
			esper.JoinSource(esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5))),
			esper.JoinSource(esper.From[wcwMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))),
		).Select(
			esper.SelectSourceEvent(0, "eventOne"),
			esper.SelectSourceEvent(1, "eventTwo"),
			esper.SelectFrom(0, "concat", esper.Concat(ms, ms)),
		).InsertInto("SomeJoinEvent", esper.StatementName("insert"))); err != nil {
			break
		}
		err = build(esper.FromAny(env, "SomeJoinEvent").Window(esper.LengthWindow(5)).
			Select(
				esper.Alias("eventOne", esper.Field[any, esper.Event]("eventOne")),
				esper.Alias("eventTwo", esper.Field[any, esper.Event]("eventTwo")),
				esper.Alias("concat", esper.Field[any, string]("concat")),
			).Query(esper.StatementName("s0")))
	case "join-no-common":
		ms := esper.JoinField[string](0, "myString")
		join := func() esper.MultiJoinStream {
			return esper.JoinMany(
				esper.JoinSource(esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5))),
				esper.JoinSource(esper.From[wcwMarket](env, "SupportMarketDataBean").Window(esper.LengthWindow(5))),
			)
		}
		selections := func() []esper.JoinSelection {
			return []esper.JoinSelection{
				esper.SelectSourceEvent(0, "eventOne"),
				esper.SelectSourceEvent(1, "eventTwo"),
				esper.SelectFrom(0, "concat", esper.Concat(ms, ms)),
			}
		}
		if err = build(join().Select(selections()...).Query(esper.StatementName("s0"))); err != nil {
			break
		}
		err = build(join().Select(selections()...).
			Where(esper.Equal[string](
				esper.JoinField[string](0, "myString"),
				esper.JoinField[string](1, "symbol"))).
			Query(esper.StatementName("s1")))
	case "join-common":
		join := func() esper.MultiJoinStream {
			return esper.JoinMany(
				esper.JoinSource(esper.From[wcwBeanA](env, "SupportBean_A").Window(esper.LengthWindow(5))),
				esper.JoinSource(esper.From[wcwBeanB](env, "SupportBean_B").Window(esper.LengthWindow(5))),
			)
		}
		selections := func() []esper.JoinSelection {
			return []esper.JoinSelection{
				esper.SelectSourceEvent(0, "eventOne"),
				esper.SelectSourceEvent(1, "eventTwo"),
				esper.SelectFrom(0, "concat", esper.Concat(
					esper.JoinField[string](0, "id"),
					esper.JoinField[string](1, "id"))),
			}
		}
		if err = build(join().Select(selections()...).Query(esper.StatementName("s0"))); err != nil {
			break
		}
		err = build(join().Select(selections()...).
			Where(esper.Equal[string](
				esper.JoinField[string](0, "id"),
				esper.JoinField[string](1, "id"))).
			Query(esper.StatementName("s1")))
	case "combined-props":
		indexed := esper.Field[wcwCombined, []*wcwNestedOne]("indexed")
		array := esper.Field[wcwCombined, []*wcwNestedOne]("array")
		err = build(esper.Select(
			esper.From[wcwCombined](env, "SupportBeanCombinedProps").Window(esper.LengthWindow(5)),
			esper.Alias("indexed", indexed),
			esper.Alias("array", array),
			esper.Alias("concat", esper.Concat(
				wcwMappedValue(indexed, 0, "0ma"),
				wcwMappedValue(indexed, 0, "0mb"))),
		).Query(esper.StatementName("s0")))
	case "wildcard-map":
		mapStr := esper.Field[wcwMapEvent, string]("theString")
		mapInt := esper.Field[wcwMapEvent, int32]("int")
		err = build(esper.Select(
			esper.From[wcwMapEvent](env, "MyMapEventIntString").Window(esper.LengthWindow(5)),
			esper.Alias("theString", mapStr),
			esper.Alias("int", mapInt),
			esper.Alias("concat", esper.Concat(mapStr, mapStr)),
		).Query(esper.StatementName("s0")))
	case "invalid-repeated":
		ms := esper.Field[wcwSimple, string]("myString")
		mi := esper.Field[wcwSimple, int32]("myInt")
		_, buildErr := env.Build(esper.Select(
			esper.From[wcwSimple](env, "SupportBeanSimple").Window(esper.LengthWindow(5)),
			esper.Alias("myString", ms),
			esper.Alias("myInt", mi),
			esper.Alias("myString", esper.Concat(ms, ms)),
		).Query(esper.StatementName("s0")))
		if buildErr == nil {
			return trace, fmt.Errorf("duplicate column unexpectedly built")
		}
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "deployed",
			Statement: "invalid",
		})
		return trace, nil
	default:
		return compat.Trace{}, fmt.Errorf("unsupported case %q", caseName)
	}
	if err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	seq := uint64(0)
	for _, plan := range plans {
		deployment, deployErr := engine.Deploy(ctx, plan)
		if deployErr != nil {
			return trace, deployErr
		}
		for _, st := range deployment.Statements() {
			st := st
			if _, subErr := st.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				seq++
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: st.Name(),
					Sequence:  seq,
					Time:      "1970-01-01T00:00:00Z",
				}
				if caseName == "combined-props" {
					var renderErr error
					record.New, renderErr = wcwCombinedRows(batch.New)
					if renderErr != nil {
						return renderErr
					}
					record.Old, renderErr = wcwCombinedRows(batch.Old)
					if renderErr != nil {
						return renderErr
					}
				} else {
					record.New = compat.NormalizeResults(batch.New)
					record.Old = compat.NormalizeResults(batch.Old)
				}
				trace.Records = append(trace.Records, record)
				return nil
			}); subErr != nil {
				return trace, subErr
			}
		}
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if caseName == "combined-props" {
				if sendErr := wcwSendCombined(ctx, engine, step); sendErr != nil {
					return trace, sendErr
				}
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, err
			}
			if sendErr := engine.SendRecord(ctx, step.EventType, payload); sendErr != nil {
				return trace, sendErr
			}
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

// wcwMappedValue mirrors the Java nested path indexed[i].mapped('k').value:
// the bean's getMapped(key) method is expressed as the mapprop map field.
func wcwMappedValue(indexed esper.Expression[[]*wcwNestedOne], index int, key string) esper.Expression[string] {
	return esper.Property[string](
		esper.MapValue[wcwNestedTwo](
			esper.Property[map[string]wcwNestedTwo](
				esper.ArrayAt[wcwNestedOne](indexed, esper.Literal(index)),
				"mapprop"),
			esper.Literal(key)),
		"value")
}

// wcwSendCombined decodes the pinned combined-props payload into the typed
// mirror bean. The trailing null slot stays nil, mirroring
// SupportBeanCombinedProps.makeDefaultBean's empty [3] slot.
func wcwSendCombined(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	if step.EventType != "SupportBeanCombinedProps" {
		return fmt.Errorf("combined-props: unexpected eventType %q", step.EventType)
	}
	var payload struct {
		Indexed []map[string]string `json:"indexed"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return err
	}
	indexed := make([]*wcwNestedOne, len(payload.Indexed))
	for i, entry := range payload.Indexed {
		if entry == nil {
			continue
		}
		nested := &wcwNestedOne{
			Mapprop:       make(map[string]wcwNestedTwo, len(entry)),
			NestLevOneVal: "abc",
		}
		for key, value := range entry {
			nested.Mapprop[key] = wcwNestedTwo{Value: value}
		}
		indexed[i] = nested
	}
	return engine.SendEvent(ctx, wcwCombined{Indexed: indexed, Array: indexed})
}

// wcwCombinedRows renders combined-props rows to the Java trace shape:
// array is a nested row array (mapprop -> key -> {value}, nestLevOneVal),
// indexed is the pinned "<unreadable>" marker (Java's indexed-only property
// rejects plain get("indexed")), concat is the string value.
func wcwCombinedRows(results []esper.Result) ([]compat.ResultRecord, error) {
	if len(results) == 0 {
		return nil, nil
	}
	records := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		arrayValue, err := esper.As[[]*wcwNestedOne](result.Get("array"))
		if err != nil {
			return nil, fmt.Errorf("combined-props: array column: %w", err)
		}
		concat, err := esper.As[string](result.Get("concat"))
		if err != nil {
			return nil, fmt.Errorf("combined-props: concat column: %w", err)
		}
		records = append(records, compat.ResultRecord{
			Kind: "row",
			Fields: map[string]any{
				"array":   wcwNestedArrayRows(arrayValue),
				"indexed": "<unreadable>",
				"concat":  concat,
			},
		})
	}
	return records, nil
}

func wcwNestedArrayRows(indexed []*wcwNestedOne) []any {
	rows := make([]any, 0, len(indexed))
	for _, nested := range indexed {
		if nested == nil {
			rows = append(rows, map[string]any{"state": "null"})
			continue
		}
		mapprop := make(map[string]any, len(nested.Mapprop))
		for key, value := range nested.Mapprop {
			mapprop[key] = map[string]any{
				"kind":   "row",
				"fields": map[string]any{"value": value.Value},
			}
		}
		rows = append(rows, map[string]any{
			"kind": "row",
			"fields": map[string]any{
				"mapprop":       map[string]any{"kind": "row", "fields": mapprop},
				"nestLevOneVal": nested.NestLevOneVal,
			},
		})
	}
	return rows
}
