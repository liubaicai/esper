package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the Infra3StreamInnerJoin representation matrix of
// InfraNWTableFAF: three-stream inner joins over named windows or tables
// executed as fire-and-forget queries across every event representation.
// InfraInvalid/InfraInvalidInsert stay compile-only approved differences
// (Java asserts EPL message prefixes; Go classifies diagnostics via ErrorCode).
var infraNwTableFafJoinJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableFAF.java",
}

type fafJoinCaseSpec struct {
	name        string
	rep         string
	runtimeID   string
	namedWindow bool
}

var fafJoinCases = []fafJoinCaseSpec{
	{"objectarray-namedwindow", "objectarray", "java-runtime-d1ad3031ef878ae3b9c3", true},
	{"objectarray-table", "objectarray", "java-runtime-3d9fd0b9f7448d2ab7eb", false},
	{"map-namedwindow", "map", "java-runtime-e884a4dfcb1160430000", true},
	{"map-table", "map", "java-runtime-a3fa439ad02f345f954f", false},
	{"avro-namedwindow", "avro", "java-runtime-e706579958dbaeb9550a", true},
	{"avro-table", "avro", "java-runtime-707436cb235b08f67d64", false},
	{"json-namedwindow", "json", "java-runtime-fe3c9605d34ad30b7cd9", true},
	{"json-table", "json", "java-runtime-71b37328d592aa52faaa", false},
	{"jsonclassprovided-namedwindow", "jsonprovided", "java-runtime-117b2202ead4651d4267", true},
	{"jsonclassprovided-table", "jsonprovided", "java-runtime-d5783637873ef63c8c8c", false},
	{"default-namedwindow", "default", "java-runtime-411b91f5c4bdf53bfcde", true},
	{"default-table", "default", "java-runtime-99aeb672638674bd9b02", false},
}

var (
	infraNwTableFafJoinJavaRuntimeIDs = func() []string {
		ids := make([]string, 0, len(fafJoinCases))
		for _, c := range fafJoinCases {
			ids = append(ids, c.runtimeID)
		}
		return ids
	}()
	infraNwTableFafJoinJavaExecutions = func() []string {
		names := make([]string, 0, len(fafJoinCases))
		for _, c := range fafJoinCases {
			names = append(names, fmt.Sprintf("Infra3StreamInnerJoin{rep=%s,namedWindow=%t}", c.rep, c.namedWindow))
		}
		return names
	}()
)

func fafJoinFields() []esper.FieldSpec {
	str := reflect.TypeOf("")
	return []esper.FieldSpec{
		esper.FieldDef("productId", str),
		esper.FieldDef("categoryId", str),
		esper.FieldDef("owner", str),
	}
}

// registerFafJoinSchemas registers Product/Category/ProductOwnerDetails with
// the case's representation and returns their schemas.
func registerFafJoinSchemas(env *esper.Environment, rep string) (product, category, details esper.Schema, err error) {
	fields := fafJoinFields()
	productFields := fields[:2]
	categoryFields := []esper.FieldSpec{fields[1], fields[2]}
	detailsFields := []esper.FieldSpec{fields[0], fields[2]}
	switch rep {
	case "objectarray":
		if product, err = esper.NewObjectArraySchema("Product", productFields); err != nil {
			return product, category, details, err
		}
		if category, err = esper.NewObjectArraySchema("Category", categoryFields); err != nil {
			return product, category, details, err
		}
		if details, err = esper.NewObjectArraySchema("ProductOwnerDetails", detailsFields); err != nil {
			return product, category, details, err
		}
		if _, err = esper.RegisterObjectArray(env, "Product", productFields); err != nil {
			return product, category, details, err
		}
		if _, err = esper.RegisterObjectArray(env, "Category", categoryFields); err != nil {
			return product, category, details, err
		}
		_, err = esper.RegisterObjectArray(env, "ProductOwnerDetails", detailsFields)
	case "json", "jsonprovided":
		if product, err = esper.NewJSONSchema("Product", productFields); err != nil {
			return product, category, details, err
		}
		if category, err = esper.NewJSONSchema("Category", categoryFields); err != nil {
			return product, category, details, err
		}
		if details, err = esper.NewJSONSchema("ProductOwnerDetails", detailsFields); err != nil {
			return product, category, details, err
		}
		if _, err = esper.RegisterJSON(env, "Product", productFields); err != nil {
			return product, category, details, err
		}
		if _, err = esper.RegisterJSON(env, "Category", categoryFields); err != nil {
			return product, category, details, err
		}
		_, err = esper.RegisterJSON(env, "ProductOwnerDetails", detailsFields)
	case "avro":
		product, err = esper.RegisterAvro(env, "Product", productFields)
		if err == nil {
			category, err = esper.RegisterAvro(env, "Category", categoryFields)
		}
		if err == nil {
			details, err = esper.RegisterAvro(env, "ProductOwnerDetails", detailsFields)
		}
	default: // map / default representations share the map underlying
		if product, err = esper.NewMapSchema("Product", productFields); err != nil {
			return product, category, details, err
		}
		if category, err = esper.NewMapSchema("Category", categoryFields); err != nil {
			return product, category, details, err
		}
		if details, err = esper.NewMapSchema("ProductOwnerDetails", detailsFields); err != nil {
			return product, category, details, err
		}
		if _, err = esper.RegisterMap(env, "Product", productFields); err != nil {
			return product, category, details, err
		}
		if _, err = esper.RegisterMap(env, "Category", categoryFields); err != nil {
			return product, category, details, err
		}
		_, err = esper.RegisterMap(env, "ProductOwnerDetails", detailsFields)
	}
	return product, category, details, err
}

// sendFafJoinEvent delivers one payload under the case's representation.
func sendFafJoinEvent(ctx context.Context, engine *esper.Engine, rep string, schemas map[string]esper.Schema, eventType string, payload []byte) error {
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return fmt.Errorf("infra-nwtable-faf-join %s payload: %w", eventType, err)
	}
	switch rep {
	case "objectarray":
		var ordered []any
		for _, field := range schemas[eventType].Fields() {
			ordered = append(ordered, values[field.Name])
		}
		return engine.SendObjectArray(ctx, eventType, ordered)
	case "json", "jsonprovided":
		return engine.SendJSON(ctx, eventType, payload)
	case "avro":
		record, err := esper.NewAvroRecordFromMap(schemas[eventType], values)
		if err != nil {
			return err
		}
		return engine.Send(ctx, eventType, record)
	default:
		return engine.SendRecord(ctx, eventType, values)
	}
}

// runInfraNwTableFafJoinScenario replays the three-stream inner join matrix:
// identical five-event population and four fire-and-forget join queries
// across every representation and both infra kinds.
func runInfraNwTableFafJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range fafJoinCases {
		if !scenarioHasCase(scenario, spec.name) {
			continue
		}
		caseTrace, err := runInfraNwTableFafJoinCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra-nwtable-faf-join case %q: %w", spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if len(trace.Records) == 0 {
		return compat.Trace{}, fmt.Errorf("infra-nwtable-faf-join scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runInfraNwTableFafJoinCase(ctx context.Context, scenario compat.Scenario, spec fafJoinCaseSpec) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, spec.name)
	if err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	product, category, details, err := registerFafJoinSchemas(env, spec.rep)
	if err != nil {
		return compat.Trace{}, err
	}
	schemas := map[string]esper.Schema{
		"Product": product, "Category": category, "ProductOwnerDetails": details,
	}

	var infraPlans []esper.Plan
	build := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		infraPlans = append(infraPlans, plan)
		return nil
	}

	if spec.namedWindow {
		for _, pair := range []struct {
			window string
			schema esper.Schema
		}{
			{"WinProduct", product}, {"WinCategory", category}, {"WinProductOwnerDetails", details},
		} {
			if _, err := esper.CreateNamedWindow(env, pair.window, pair.schema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
				return compat.Trace{}, err
			}
		}
		streams := map[string]esper.RecordStream{
			"Product":             esper.FromAny(env, "Product"),
			"Category":            esper.FromAny(env, "Category"),
			"ProductOwnerDetails": esper.FromAny(env, "ProductOwnerDetails"),
		}
		for _, pair := range []struct {
			source string
			window string
		}{
			{"Product", "WinProduct"}, {"Category", "WinCategory"}, {"ProductOwnerDetails", "WinProductOwnerDetails"},
		} {
			if err := build(esper.OnRecord(streams[pair.source]).
				InsertIntoNamedWindow(pair.window, esper.CopyMatchingFields()).
				Query(esper.StatementName("insert-" + pair.window))); err != nil {
				return compat.Trace{}, err
			}
		}
	} else {
		str := reflect.TypeOf("")
		if _, err := esper.CreateTableInModule(env, "", "WinProduct", []esper.TableColumn{
			{Name: "productId", Type: str, PrimaryKey: true},
			{Name: "categoryId", Type: str, PrimaryKey: true},
		}); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateTableInModule(env, "", "WinCategory", []esper.TableColumn{
			{Name: "categoryId", Type: str, PrimaryKey: true},
			{Name: "owner", Type: str, PrimaryKey: true},
		}); err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateTableInModule(env, "", "WinProductOwnerDetails", []esper.TableColumn{
			{Name: "productId", Type: str, PrimaryKey: true},
			{Name: "owner", Type: str},
		}); err != nil {
			return compat.Trace{}, err
		}
		merges := []struct {
			event string
			table string
			keys  []esper.Expr
			cols  []string
		}{
			{"Product", "WinProduct",
				[]esper.Expr{fieldAnyExpr("productId"), fieldAnyExpr("categoryId")},
				[]string{"productId", "categoryId"}},
			{"Category", "WinCategory",
				[]esper.Expr{fieldAnyExpr("categoryId"), fieldAnyExpr("owner")},
				[]string{"categoryId", "owner"}},
			{"ProductOwnerDetails", "WinProductOwnerDetails",
				[]esper.Expr{fieldAnyExpr("productId")},
				[]string{"productId", "owner"}},
		}
		for _, m := range merges {
			assignments := make([]esper.TableAssignment, 0, len(m.cols))
			for _, col := range m.cols {
				assignments = append(assignments, esper.SetColumn(col, fieldAnyExpr(col)))
			}
			query := esper.OnRecord(esper.FromAny(env, m.event)).
				MergeIntoTableWhen(m.table, m.keys,
					esper.WhenNotMatchedAny(assignments...)).
				Query(esper.StatementName("merge-" + m.table))
			if err := build(query); err != nil {
				return compat.Trace{}, err
			}
		}
	}

	fafPlans := map[string]esper.Plan{}
	buildFaf := func(name string, query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		fafPlans[name] = plan
		return nil
	}
	jPID := func(src int) esper.Expr { return esper.JoinField[string](src, "productId") }
	jCat := func(src int) esper.Expr { return esper.JoinField[string](src, "categoryId") }
	jOwn := func(src int) esper.Expr { return esper.JoinField[string](src, "owner") }
	source := func(name string) esper.JoinInput {
		if spec.namedWindow {
			return esper.JoinRecordSource(esper.FromNamedWindow(env, name))
		}
		return esper.JoinRecordSource(esper.FromTable(env, name))
	}
	sources := []esper.JoinInput{
		source("WinProduct"), source("WinCategory"), source("WinProductOwnerDetails"),
	}
	onConditions := []esper.JoinCondition{
		esper.OnSourcesEqual(0, jCat(0), 1, jCat(1)),
		esper.OnSourcesEqual(0, jPID(0), 2, jPID(2)),
	}
	projection := []esper.JoinSelection{esper.SelectFrom(0, "WinProduct.productId", jPID(0))}

	q1 := esper.JoinMany(sources...).On(onConditions...).Select(projection...)
	if err := buildFaf("q1", q1.Query(esper.StatementName("q1"))); err != nil {
		return compat.Trace{}, err
	}
	q2 := esper.JoinMany(sources...).On(onConditions...).Select(projection...).
		Where(esper.EqualOf(jOwn(1), jOwn(2))).
		Query(esper.StatementName("q2"))
	if err := buildFaf("q2", q2); err != nil {
		return compat.Trace{}, err
	}
	// q3 mirrors Java's comma-style form: a filtered cross join.
	q3 := esper.JoinMany(sources...).Select(projection...).
		Where(esper.And(
			esper.EqualOf(jOwn(1), jOwn(2)),
			esper.And(
				esper.EqualOf(jCat(0), jCat(1)),
				esper.EqualOf(jPID(0), jPID(2)),
			),
		)).Query(esper.StatementName("q3"))
	if err := buildFaf("q3", q3); err != nil {
		return compat.Trace{}, err
	}
	// q4 moves the owner predicate into HAVING (no group-by).
	q4 := esper.JoinMany(sources...).On(onConditions...).Select(projection...).
		Having(esper.EqualOf(jOwn(1), jOwn(2))).
		Query(esper.StatementName("q4"))
	if err := buildFaf("q4", q4); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	for _, plan := range infraPlans {
		if _, err := engine.Deploy(ctx, plan); err != nil {
			return compat.Trace{}, err
		}
	}

	trace := compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			if err := sendFafJoinEvent(ctx, engine, spec.rep, schemas, step.EventType, step.Payload); err != nil {
				return trace, err
			}
		case "faf":
			plan, ok := fafPlans[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown faf query %q", step.Statement)
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				return trace, err
			}
			record := compat.TraceRecord{
				Case:      spec.name,
				Operation: "faf",
				Statement: step.Statement,
			}
			record.New = compat.NormalizeResults(result.Batch.New)
			record.Old = compat.NormalizeResults(result.Batch.Old)
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported infra-nwtable-faf-join step op %q", step.Op)
		}
	}
	return trace, nil
}

func fieldAnyExpr(name string) esper.Expr {
	return esper.Field[any, string](name)
}
