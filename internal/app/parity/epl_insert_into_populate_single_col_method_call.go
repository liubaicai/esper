package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_insert_into_populate_single_col_method_call.go replays the
// EPLInsertIntoPopulateSingleColByMethodCall execution (ord 0) against the
// pinned Java oracle: nine rounds of single-column method-call insert-into
// population in one Java runtime, modelled as nine cases with fresh
// runtimes (the Java undeploys between rounds; per-case runtimes are the
// established observably-equivalent convention).
//
// Implicit rounds deploy 'insert into <Prefix>_Stream select * from
// <origin>' (s1, listener attached but silent — the sent event is the
// sibling type) plus 'insert into <Prefix>_Stream select
// SupportStaticMethodLib.<fn>(s0) from <eventType> as s0' (s2). Configured
// rounds deploy 'insert into <target> select <fn>(s0) from <origin> as s0'
// (insert, no listener) plus 'select * from <target>' (s0). Each UDF passes
// field 'one' through and wraps field 'two' in "|…|"; the bean UDF maps
// SupportMarketDataBean{symbol,volume} to SupportBean{theString,
// intPrimitive}.
//
// Approved differences (observably identical to the Java EPL):
//   - Esper auto-creates the <Prefix>_Stream insert-into targets; Go
//     pre-registers them with the origin's representation and fields.
//   - Java's statement/delivered-event class assertions (BeanEventType,
//     WrapperEventType, underlyingType subclass checks) have no Go
//     counterpart; they are pinned as "value" records carrying the schema
//     kind name (Struct/Map/ObjectArray/Avro/JSON), the typed-columns
//     precedent. The configured-json round's Object.class assertions are
//     tautological in Java; the kind record still pins the observable JSON
//     representation.
//   - The implicit-json s1 wildcard uses explicit one/two column aliases:
//     Transpose to a JSON target requires a string payload, so the
//     explicit-Alias equivalent (iupsNativeTranspose precedent) is used.
//   - Bean rows render the projected asserted fields {theString,
//     intPrimitive}; Java asserts theString only and intPrimitive=0 is
//     deterministic from the source event.

const eplInsertIntoSingleColMethodCallJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoSingleColMethodCallJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoPopulateSingleColByMethodCall.java",
}

var eplInsertIntoSingleColMethodCallJavaRuntimeIDs = []string{
	"java-runtime-abe5e5cbda9667e7e112",
}

var eplInsertIntoSingleColMethodCallJavaExecutions = []string{
	"EPLInsertIntoPopulateSingleColByMethodCall",
}

var eplInsertIntoSingleColMethodCallCases = []string{
	"implicit-bean",
	"implicit-map",
	"configured-map",
	"implicit-oa",
	"configured-oa",
	"implicit-avro",
	"configured-avro",
	"implicit-json",
	"configured-json",
}

var eplInsertIntoSingleColMethodCallCaseRuntimeIDs = map[string]string{
	"implicit-bean":   "java-runtime-abe5e5cbda9667e7e112",
	"implicit-map":    "java-runtime-abe5e5cbda9667e7e112",
	"configured-map":  "java-runtime-abe5e5cbda9667e7e112",
	"implicit-oa":     "java-runtime-abe5e5cbda9667e7e112",
	"configured-oa":   "java-runtime-abe5e5cbda9667e7e112",
	"implicit-avro":   "java-runtime-abe5e5cbda9667e7e112",
	"configured-avro": "java-runtime-abe5e5cbda9667e7e112",
	"implicit-json":   "java-runtime-abe5e5cbda9667e7e112",
	"configured-json": "java-runtime-abe5e5cbda9667e7e112",
}

// scmBean mirrors SupportBean's asserted fields: the bean UDF returns
// SupportBean(symbol, volume.intValue()) and the suite asserts theString
// (intPrimitive=0 is deterministic from the source event).
type scmBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// scmMarketData mirrors SupportMarketDataBean's send payload fields.
type scmMarketData struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   string  `esper:"feed"`
}

// scmKindNames renders the schema kind names the value records pin; the
// Java oracle maps the asserted underlying classes to the same vocabulary
// (SupportBean→Struct, Map→Map, Object[]→ObjectArray, GenericData.Record→
// Avro, JsonEventObject→JSON).
var scmKindNames = map[esper.SchemaKind]string{
	esper.SchemaStruct:      "Struct",
	esper.SchemaMap:         "Map",
	esper.SchemaObjectArray: "ObjectArray",
	esper.SchemaAvro:        "Avro",
	esper.SchemaJSON:        "JSON",
}

// scmConvertEvent mirrors SupportStaticMethodLib.convertEvent:
// SupportMarketDataBean → SupportBean(symbol, volume.intValue()).
func scmConvertEvent(bean scmMarketData) scmBean {
	return scmBean{TheString: bean.Symbol, IntPrimitive: int(bean.Volume)}
}

// scmConvertEventMap mirrors convertEventMap: one passes through, two is
// wrapped in "|…|".
func scmConvertEventMap(values map[string]any) map[string]any {
	return map[string]any{"one": values["one"], "two": "|" + fmt.Sprint(values["two"]) + "|"}
}

// scmConvertEventObjectArray mirrors convertEventObjectArray.
func scmConvertEventObjectArray(values []any) []any {
	return []any{values[0], "|" + fmt.Sprint(values[1]) + "|"}
}

// scmConvertEventAvro mirrors convertEventAvro: a new record over the same
// schema with two wrapped; the Go equivalent returns the field map that the
// Avro route target materializes.
func scmConvertEventAvro(row *esper.AvroRecord) map[string]any {
	return map[string]any{"one": fmt.Sprint(row.Get("one")), "two": "|" + fmt.Sprint(row.Get("two")) + "|"}
}

// scmConvertEventJSON mirrors convertEventJson: the UDF returns the JSON
// text that the JSON route target re-parses.
func scmConvertEventJSON(row map[string]any) string {
	// Marshaling a string-valued map cannot fail.
	text, _ := json.Marshal(map[string]any{
		"one": fmt.Sprint(row["one"]),
		"two": "|" + fmt.Sprint(row["two"]) + "|",
	})
	return string(text)
}

// runEplInsertIntoSingleColMethodCallScenario replays the nine
// single-column method-call insert-into rounds.
func runEplInsertIntoSingleColMethodCallScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplInsertIntoSingleColMethodCallCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplInsertIntoSingleColMethodCallCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl insert-into single-col-method-call case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl insert-into single-col-method-call scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplInsertIntoSingleColMethodCallCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plans, err := buildEplInsertIntoSingleColMethodCallCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplInsertIntoSingleColMethodCallCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	sequences := make(map[string]uint64)
	listened := map[string]bool{"s1": true, "s2": true, "s0": true}
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return trace, err
		}
		for _, statement := range deployment.Statements() {
			name := statement.Name()
			// Implicit rounds pin the s1/s2 statement type assertions as
			// value records at deploy time (the Java asserts them right
			// after each compileDeploy, before the send).
			if scmImplicitCase(caseName) && (name == "s1" || name == "s2") {
				trace.Records = append(trace.Records, scmValueRecord(caseName, name, env))
			}
			if !listened[name] {
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
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := scmSendSourceEvent(ctx, engine, env, step); err != nil {
				return trace, err
			}
			// Configured rounds pin the delivered event's underlying class
			// (assertEventNew) as a value record after the listener fires.
			if !scmImplicitCase(caseName) {
				trace.Records = append(trace.Records, scmValueRecord(caseName, "s0", env))
			}
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

func scmImplicitCase(caseName string) bool {
	return len(caseName) >= len("implicit") && caseName[:len("implicit")] == "implicit"
}

// scmValueRecord pins the statement's output schema kind, the Go
// observable for the Java eventType/underlying class assertions. The
// output schema is the insert-into target's registered schema for the
// implicit s1/s2 legs and the configured s0's source type.
func scmValueRecord(caseName, statement string, env *esper.Environment) compat.TraceRecord {
	kind := "unknown"
	if schema, ok := env.Schema(scmStatementOutputType(caseName)); ok {
		kind = scmKindNames[schema.Kind()]
		if kind == "" {
			kind = fmt.Sprintf("SchemaKind(%d)", schema.Kind())
		}
	}
	return compat.TraceRecord{
		Case:      caseName,
		Operation: "value",
		Statement: statement,
		Value:     kind,
	}
}

// scmStatementOutputType maps a case's asserted statement to the event
// type whose schema carries the asserted representation: implicit s1/s2
// output the <Prefix>_Stream type, configured s0 reads the target type.
func scmStatementOutputType(caseName string) string {
	switch caseName {
	case "implicit-bean":
		return "Bean_Stream"
	case "implicit-map":
		return "Map_Stream"
	case "configured-map":
		return "MapOne"
	case "implicit-oa":
		return "OA_Stream"
	case "configured-oa":
		return "OAOne"
	case "implicit-avro":
		return "Avro_Stream"
	case "configured-avro":
		return "AvroOne"
	case "implicit-json":
		return "Json_Stream"
	case "configured-json":
		return "JsonOne"
	}
	return ""
}

// scmSendSourceEvent delivers the round's single source event under the
// representation's send semantics: bean payload struct, map record,
// positional object array ordered by schema declaration, Avro record over
// the deployed schema, or raw JSON text.
func scmSendSourceEvent(ctx context.Context, engine *esper.Engine, env *esper.Environment, step compat.Step) error {
	eventType := step.EventType
	switch eventType {
	case "SupportMarketDataBean":
		var values struct {
			Symbol string  `json:"symbol"`
			Price  float64 `json:"price"`
			Volume int64   `json:"volume"`
			Feed   *string `json:"feed"`
		}
		if err := json.Unmarshal(step.Payload, &values); err != nil {
			return fmt.Errorf("decode %s payload: %w", eventType, err)
		}
		event := scmMarketData{Symbol: values.Symbol, Price: values.Price, Volume: values.Volume}
		if values.Feed != nil {
			event.Feed = *values.Feed
		}
		return engine.Send(ctx, eventType, event)
	case "MapTwo":
		var values map[string]any
		if err := json.Unmarshal(step.Payload, &values); err != nil {
			return fmt.Errorf("decode %s payload: %w", eventType, err)
		}
		return engine.SendRecord(ctx, eventType, values)
	case "OATwo":
		var values map[string]any
		if err := json.Unmarshal(step.Payload, &values); err != nil {
			return fmt.Errorf("decode %s payload: %w", eventType, err)
		}
		schema, ok := env.Schema(eventType)
		if !ok {
			return fmt.Errorf("schema %q is not registered", eventType)
		}
		ordered := make([]any, 0, len(schema.Fields()))
		for _, field := range schema.Fields() {
			ordered = append(ordered, values[field.Name])
		}
		return engine.SendObjectArray(ctx, eventType, ordered)
	case "AvroTwo":
		var values map[string]any
		if err := json.Unmarshal(step.Payload, &values); err != nil {
			return fmt.Errorf("decode %s payload: %w", eventType, err)
		}
		schema, ok := env.Schema(eventType)
		if !ok {
			return fmt.Errorf("schema %q is not registered", eventType)
		}
		record, err := esper.NewAvroRecordFromMap(schema, values)
		if err != nil {
			return err
		}
		return engine.SendAvro(ctx, eventType, record)
	case "JsonTwo":
		return engine.SendJSON(ctx, eventType, step.Payload)
	default:
		return fmt.Errorf("unknown eventType %q", eventType)
	}
}

// buildEplInsertIntoSingleColMethodCallCase registers the round's schemas
// and builds its statements. Implicit rounds build s1 (wildcard route into
// the pre-registered <Prefix>_Stream) and s2 (single-column UDF route);
// configured rounds build insert (UDF route into the preconfigured target)
// and s0 (plain select over the target).
func buildEplInsertIntoSingleColMethodCallCase(env *esper.Environment, caseName string) ([]esper.Plan, error) {
	stringFields := []esper.FieldSpec{
		esper.FieldDef("one", reflect.TypeOf("")),
		esper.FieldDef("two", reflect.TypeOf("")),
	}
	switch caseName {
	case "implicit-bean":
		if _, err := esper.RegisterStruct[scmBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[scmMarketData](env, "SupportMarketDataBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[scmBean](env, "Bean_Stream"); err != nil {
			return nil, err
		}
		s1, err := env.Build(esper.FromAny(env, "SupportBean").Select(
			esper.Selection{Expr: esper.Transpose[scmBean](esper.EventValue[scmBean]())},
		).InsertInto("Bean_Stream", esper.StatementName("s1")))
		if err != nil {
			return nil, err
		}
		s2, err := env.Build(esper.FromAny(env, "SupportMarketDataBean").Select(
			esper.Selection{Expr: esper.Transpose[scmBean](esper.Func1("convertEvent", scmConvertEvent, esper.EventValue[scmMarketData]()))},
		).InsertInto("Bean_Stream", esper.StatementName("s2")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{s1, s2}, nil

	case "implicit-map", "configured-map":
		for _, name := range []string{"MapOne", "MapTwo"} {
			if _, err := esper.RegisterMap(env, name, stringFields); err != nil {
				return nil, err
			}
		}
		if caseName == "implicit-map" {
			if _, err := esper.RegisterMap(env, "Map_Stream", stringFields); err != nil {
				return nil, err
			}
			s1, err := env.Build(esper.FromAny(env, "MapOne").Select(
				esper.Selection{Expr: esper.Transpose[map[string]any](esper.EventValue[map[string]any]())},
			).InsertInto("Map_Stream", esper.StatementName("s1")))
			if err != nil {
				return nil, err
			}
			s2, err := env.Build(esper.FromAny(env, "MapTwo").Select(
				esper.Selection{Expr: esper.Transpose[map[string]any](esper.Func1("convertEventMap", scmConvertEventMap, esper.EventValue[map[string]any]()))},
			).InsertInto("Map_Stream", esper.StatementName("s2")))
			if err != nil {
				return nil, err
			}
			return []esper.Plan{s1, s2}, nil
		}
		insert, err := env.Build(esper.FromAny(env, "MapTwo").Select(
			esper.Selection{Expr: esper.Transpose[map[string]any](esper.Func1("convertEventMap", scmConvertEventMap, esper.EventValue[map[string]any]()))},
		).InsertInto("MapOne", esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		s0, err := env.Build(esper.FromAny(env, "MapOne").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insert, s0}, nil

	case "implicit-oa", "configured-oa":
		for _, name := range []string{"OAOne", "OATwo"} {
			if _, err := esper.RegisterObjectArray(env, name, stringFields); err != nil {
				return nil, err
			}
		}
		if caseName == "implicit-oa" {
			if _, err := esper.RegisterObjectArray(env, "OA_Stream", stringFields); err != nil {
				return nil, err
			}
			s1, err := env.Build(esper.FromAny(env, "OAOne").Select(
				esper.Selection{Expr: esper.Transpose[[]any](esper.EventValue[[]any]())},
			).InsertInto("OA_Stream", esper.StatementName("s1")))
			if err != nil {
				return nil, err
			}
			s2, err := env.Build(esper.FromAny(env, "OATwo").Select(
				esper.Selection{Expr: esper.Transpose[[]any](esper.Func1("convertEventObjectArray", scmConvertEventObjectArray, esper.EventValue[[]any]()))},
			).InsertInto("OA_Stream", esper.StatementName("s2")))
			if err != nil {
				return nil, err
			}
			return []esper.Plan{s1, s2}, nil
		}
		insert, err := env.Build(esper.FromAny(env, "OATwo").Select(
			esper.Selection{Expr: esper.Transpose[[]any](esper.Func1("convertEventObjectArray", scmConvertEventObjectArray, esper.EventValue[[]any]()))},
		).InsertInto("OAOne", esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		s0, err := env.Build(esper.FromAny(env, "OAOne").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insert, s0}, nil

	case "implicit-avro", "configured-avro":
		for _, name := range []string{"AvroOne", "AvroTwo"} {
			if _, err := esper.RegisterAvro(env, name, stringFields); err != nil {
				return nil, err
			}
		}
		if caseName == "implicit-avro" {
			if _, err := esper.RegisterAvro(env, "Avro_Stream", stringFields); err != nil {
				return nil, err
			}
			s1, err := env.Build(esper.FromAny(env, "AvroOne").Select(
				esper.Selection{Expr: esper.Transpose[*esper.AvroRecord](esper.EventValue[*esper.AvroRecord]())},
			).InsertInto("Avro_Stream", esper.StatementName("s1")))
			if err != nil {
				return nil, err
			}
			s2, err := env.Build(esper.FromAny(env, "AvroTwo").Select(
				esper.Selection{Expr: esper.Transpose[map[string]any](esper.Func1("convertEventAvro", scmConvertEventAvro, esper.EventValue[*esper.AvroRecord]()))},
			).InsertInto("Avro_Stream", esper.StatementName("s2")))
			if err != nil {
				return nil, err
			}
			return []esper.Plan{s1, s2}, nil
		}
		insert, err := env.Build(esper.FromAny(env, "AvroTwo").Select(
			esper.Selection{Expr: esper.Transpose[map[string]any](esper.Func1("convertEventAvro", scmConvertEventAvro, esper.EventValue[*esper.AvroRecord]()))},
		).InsertInto("AvroOne", esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		s0, err := env.Build(esper.FromAny(env, "AvroOne").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insert, s0}, nil

	case "implicit-json", "configured-json":
		for _, name := range []string{"JsonOne", "JsonTwo"} {
			if _, err := esper.RegisterJSON(env, name, stringFields); err != nil {
				return nil, err
			}
		}
		if caseName == "implicit-json" {
			if _, err := esper.RegisterJSON(env, "Json_Stream", stringFields); err != nil {
				return nil, err
			}
			// Transpose to a JSON target requires a string payload, so the
			// wildcard leg uses the explicit-Alias equivalent.
			s1, err := env.Build(esper.FromAny(env, "JsonOne").Select(
				esper.Alias("one", esper.Field[map[string]any, string]("one")),
				esper.Alias("two", esper.Field[map[string]any, string]("two")),
			).InsertInto("Json_Stream", esper.StatementName("s1")))
			if err != nil {
				return nil, err
			}
			s2, err := env.Build(esper.FromAny(env, "JsonTwo").Select(
				esper.Selection{Expr: esper.Transpose[string](esper.Func1("convertEventJson", scmConvertEventJSON, esper.EventValue[map[string]any]()))},
			).InsertInto("Json_Stream", esper.StatementName("s2")))
			if err != nil {
				return nil, err
			}
			return []esper.Plan{s1, s2}, nil
		}
		insert, err := env.Build(esper.FromAny(env, "JsonTwo").Select(
			esper.Selection{Expr: esper.Transpose[string](esper.Func1("convertEventJson", scmConvertEventJSON, esper.EventValue[map[string]any]()))},
		).InsertInto("JsonOne", esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		s0, err := env.Build(esper.FromAny(env, "JsonOne").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, err
		}
		return []esper.Plan{insert, s0}, nil
	}
	return nil, fmt.Errorf("unknown epl insert-into single-col-method-call case %q", caseName)
}
