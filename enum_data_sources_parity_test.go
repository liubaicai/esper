package esper

import (
	"context"
	"reflect"
	"testing"
)

type enumDataSourceDetail struct {
	ItemID string `esper:"itemId"`
}

type enumDataSourceOrder struct {
	Details []enumDataSourceDetail `esper:"details"`
}

// TestEnumDataSourcesPreviousWindowMatchesEsper covers ExprEnumDataSources'
// PrevWindow/PrevFuncs source family with a typed chainable enumeration
// predicate. The collection is evaluated against the retained window state,
// not against the current event only.
func TestEnumDataSourcesPreviousWindowMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	stream := From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2))
	window := PrevWindow[runtimeTestTrade](EventValue[runtimeTestTrade]())
	allBelowFive := EnumAllOf[runtimeTestTrade](window, Less[float64](
		EnumField[runtimeTestTrade, float64]("price"),
		Literal(5.0),
	))
	selected := EnumSelect[runtimeTestTrade, string](window, EnumField[runtimeTestTrade, string]("symbol"))
	plan, err := env.Build(Select(stream,
		Alias("allBelowFive", allBelowFive),
		Alias("symbols", selected),
	).Query(StatementName("enum-prev-window")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("previous enumeration result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 1},
		{Symbol: "E2", Price: 10},
		{Symbol: "E3", Price: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	if len(rows) != 3 {
		t.Fatalf("previous enumeration rows = %d, want 3", len(rows))
	}
	wantAll := []bool{true, false, false}
	wantSymbols := [][]string{{"E1"}, {"E2", "E1"}, {"E3", "E2"}}
	for index, row := range rows {
		if got := row.Get("allBelowFive").Any(); got != wantAll[index] {
			t.Errorf("row %d allBelowFive = %#v, want %v", index, got, wantAll[index])
		}
		got, err := As[[]string](row.Get("symbols"))
		if err != nil || !reflect.DeepEqual(got, wantSymbols[index]) {
			t.Errorf("row %d symbols = %#v, err=%v, want %#v", index, got, err, wantSymbols[index])
		}
	}
}

// TestEnumDataSourcesSortedPreviousWindowMatchesEsper covers the sorted
// previous-window collection used by ExprEnumPrevWindowSorted. The enumerable
// sees view order, including eviction of the worst retained event.
func TestEnumDataSourcesSortedPreviousWindowMatchesEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	price := Field[runtimeTestTrade, float64]("price")
	stream := From[runtimeTestTrade](env, "Trade").Window(SortWindow(3, Ascending(price)))
	window := PrevWindow[runtimeTestTrade](EventValue[runtimeTestTrade]())
	ids := EnumSelect[runtimeTestTrade, string](window, EnumField[runtimeTestTrade, string]("symbol"))
	plan, err := env.Build(Select(stream, Alias("ids", ids)).Query(StatementName("enum-sorted-prev-window")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("sorted previous result is not a row: %#v", result)
			}
			values, err := As[[]string](row.Get("ids"))
			if err != nil {
				return err
			}
			got = append(got, values)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1", Price: 5},
		{Symbol: "E2", Price: 6},
		{Symbol: "E3", Price: 4},
		{Symbol: "E5", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := [][]string{{"E1"}, {"E1", "E2"}, {"E3", "E1", "E2"}, {"E5", "E3", "E1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted previous enumeration = %#v, want %#v", got, want)
	}
}

// TestEnumDataSourcesNamedWindowAndSubqueryMatchesEsper covers a named-window
// collection source and its empty/non-empty ALL semantics at outer-event
// evaluation time.
func TestEnumDataSourcesNamedWindowAndSubqueryMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	schema, err := RegisterStruct[runtimeTestTrade](env, "EnumNamedSourceTrade")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "EnumNamedSourceWindow", schema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	triggerSchema, err := RegisterStruct[enumTableTrigger](env, "EnumNamedSourceTrigger")
	if err != nil {
		t.Fatal(err)
	}
	window := FromNamedWindow(env, "EnumNamedSourceWindow")
	prices := SubqueryValues[float64](window, Field[any, float64]("price"))
	allBelowFive := EnumAllOf[float64](prices, Less[float64](EnumElement[float64](), Literal(5.0)))
	plan, err := env.Build(Select(
		From[enumTableTrigger](env, triggerSchema.Name()),
		Alias("allBelowFive", allBelowFive),
	).Query(StatementName("enum-named-window-source")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var values []any
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("named-window enumeration result is not a row: %#v", result)
			}
			values = append(values, row.Get("allBelowFive").Any())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumTableTrigger{ID: "empty"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "EnumNamedSourceWindow", runtimeTestTrade{Symbol: "E1", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "EnumNamedSourceWindow", runtimeTestTrade{Symbol: "E2", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumTableTrigger{ID: "mixed"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.InsertNamedWindow(context.Background(), "EnumNamedSourceWindow", runtimeTestTrade{Symbol: "E3", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumTableTrigger{ID: "mixed-again"}); err != nil {
		t.Fatal(err)
	}
	want := []any{true, false, false}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("named-window enumeration values = %#v, want %#v", values, want)
	}
}

// TestEnumDataSourcesVariableAndSubstitutionMatchEsper keeps the collection
// source late-bound in both forms used by ExprEnumSubstitutionParameter and
// ExprEnumVariable.
func TestEnumDataSourcesVariableAndSubstitutionMatchEsper(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if err := env.RegisterVariable("enum-symbols", []string{"E1", "E3"}); err != nil {
		t.Fatal(err)
	}
	symbol := Field[runtimeTestTrade, string]("symbol")
	variableValues := VariableRef[[]string]("enum-symbols")
	variableMatch := EnumAnyOf[string](variableValues, Equal[string](EnumElement[string](), symbol))
	parameterValues := Parameter[[]string]("symbols")
	parameterMatch := EnumAnyOf[string](parameterValues, Equal[string](EnumElement[string](), symbol))
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("variableMatch", variableMatch),
		Alias("parameterMatch", parameterMatch),
	).Query(StatementName("enum-late-bound-source")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"symbols": []string{"E2", "E4"}})
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("late-bound enumeration result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "E1"},
		{Symbol: "E2"},
		{Symbol: "E3"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("late-bound enumeration rows = %d, want 3", len(rows))
	}
	want := [][2]bool{{true, false}, {false, true}, {true, false}}
	for index, row := range rows {
		got := [2]bool{row.Get("variableMatch").Any() == true, row.Get("parameterMatch").Any() == true}
		if got != want[index] {
			t.Errorf("row %d late-bound matches = %#v, want %#v", index, got, want[index])
		}
	}
}

// TestEnumDataSourcesNestedPropertySchemaMatchesEsper covers a schema-backed
// array property as an enumeration source, preserving child field metadata.
func TestEnumDataSourcesNestedPropertySchemaMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[enumDataSourceDetail](env, "EnumOrderDetail"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[enumDataSourceOrder](env, "EnumOrder"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	orders := From[enumDataSourceOrder](env, "EnumOrder")
	details := Property[[]enumDataSourceDetail](EventValue[enumDataSourceOrder](), "details")
	selected := EnumSelect[enumDataSourceDetail, string](
		EnumWhere[enumDataSourceDetail](details, Equal[string](EnumField[enumDataSourceDetail, string]("itemId"), Literal("001"))),
		EnumField[enumDataSourceDetail, string]("itemId"),
	)
	plan, err := env.Build(Select(orders, Alias("matching", selected)).Query(StatementName("enum-nested-property-source")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("nested property enumeration result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), enumDataSourceOrder{Details: []enumDataSourceDetail{{ItemID: "002"}, {ItemID: "001"}}}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("nested property enumeration rows = %d, want 1", len(rows))
	}
	matching, err := As[[]string](rows[0].Get("matching"))
	if err != nil || !reflect.DeepEqual(matching, []string{"001"}) {
		t.Fatalf("nested property enumeration = %#v, err=%v", matching, err)
	}
}
