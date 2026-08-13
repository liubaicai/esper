package esper

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type contextCategorySupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type contextCategorySupportBeanS0 struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func newContextCategoryParityEnvironment(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[contextCategorySupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func collectContextCategoryRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("context category result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows
}

func contextCategoryRowValues(t *testing.T, row Row, fields []string) []any {
	t.Helper()
	values := make([]any, len(fields))
	for index, field := range fields {
		values[index] = row.Get(field).Any()
	}
	return values
}

func assertContextCategoryRows(t *testing.T, rows []Row, fields []string, want ...[]any) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("context category rows = %#v, want %d rows", rows, len(want))
	}
	for index, expected := range want {
		if got := contextCategoryRowValues(t, rows[index], fields); !reflect.DeepEqual(got, expected) {
			t.Fatalf("context category row %d = %#v, want %#v", index, got, expected)
		}
	}
}

func assertContextCategoryRowsAnyOrder(t *testing.T, rows []Row, fields []string, want ...[]any) {
	t.Helper()
	remaining := append([][]any(nil), want...)
	for _, row := range rows {
		got := contextCategoryRowValues(t, row, fields)
		found := -1
		for index, expected := range remaining {
			if reflect.DeepEqual(got, expected) {
				found = index
				break
			}
		}
		if found < 0 {
			t.Fatalf("unexpected context category row %#v; remaining %#v", got, remaining)
		}
		remaining = append(remaining[:found], remaining[found+1:]...)
	}
	if len(remaining) != 0 {
		t.Fatalf("missing context category rows %#v", remaining)
	}
}

func contextCategorySnapshot(t *testing.T, statement *Statement, selector ContextPartitionSelector) []Row {
	t.Helper()
	result, err := statement.SnapshotWithSelector(context.Background(), selector)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]Row, 0, len(result.Results()))
	for _, result := range result.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("context category snapshot result is not a row: %#v", result)
		}
		rows = append(rows, row)
	}
	return rows
}

// TestContextCategorySceneOneParity covers ContextCategorySceneOne's
// predefined category partitions, context administration and per-category
// count/label delivery.
func TestContextCategorySceneOneParity(t *testing.T) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	theString := Field[contextCategorySupportBean, string]("theString")
	if _, err := CreateCategoryContext(env, "CategoryContext",
		Category("cat1", Equal[string](theString, Literal("A"))),
		Category("cat2", Equal[string](theString, Literal("B"))),
	); err != nil {
		t.Fatal(err)
	}

	plan, err := env.Build(From[contextCategorySupportBean](env, "SupportBean").Aggregate(
		Alias("c0", CountAll()),
		Alias("c1", ContextLabel()),
	).Query(StatementName("s0"), WithContext("CategoryContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	statement := deployment.Statements()[0]

	names, err := engine.ContextStatementNames("CategoryContext")
	if err != nil || !reflect.DeepEqual(names, []string{"s0"}) {
		t.Fatalf("context statement names = %#v, err=%v", names, err)
	}
	level, err := engine.ContextNestingLevel("CategoryContext")
	if err != nil || level != 1 {
		t.Fatalf("context nesting level = %d, err=%v", level, err)
	}
	descriptors, err := engine.ContextPartitionDescriptors("CategoryContext", ContextPartitionSelectorAll{})
	if err != nil || len(descriptors) != 2 {
		t.Fatalf("category partitions = %#v, err=%v", descriptors, err)
	}
	labelsByID := map[int]string{}
	for _, descriptor := range descriptors {
		label, ok := descriptor.Property("label")
		if !ok || label.Any() == nil {
			t.Fatalf("category descriptor has no label: %#v", descriptor)
		}
		labelsByID[descriptor.ID] = label.Any().(string)
	}
	if labelsByID[0] != "cat1" || labelsByID[1] != "cat2" {
		t.Fatalf("category descriptor IDs/labels = %#v", labelsByID)
	}

	fields := []string{"c0", "c1"}
	rows := collectContextCategoryRows(t, deployment)
	send := func(theStringValue string, intValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: theStringValue, IntPrimitive: intValue}); err != nil {
			t.Fatal(err)
		}
	}
	send("A", 1)
	assertContextCategoryRows(t, (*rows)[:1], fields, []any{int64(1), "cat1"})
	send("C", 2)
	if len(*rows) != 1 {
		t.Fatalf("unmatched category produced output: %#v", *rows)
	}
	send("B", 3)
	assertContextCategoryRows(t, *rows, fields, []any{int64(1), "cat1"}, []any{int64(1), "cat2"})
	send("A", 4)
	assertContextCategoryRows(t, *rows, fields, []any{int64(1), "cat1"}, []any{int64(1), "cat2"}, []any{int64(2), "cat1"})
	send("A", 6)
	send("B", 5)
	send("C", 7)
	assertContextCategoryRows(t, *rows, fields,
		[]any{int64(1), "cat1"}, []any{int64(1), "cat2"}, []any{int64(2), "cat1"},
		[]any{int64(3), "cat1"}, []any{int64(2), "cat2"},
	)

	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("statement category partitions = %d, want 2", statement.ContextPartitionCount())
	}
}

// TestContextCategorySceneTwoParity covers the boolean predicate categories,
// preallocated partition metadata and cumulative ungrouped sum used by
// ContextCategorySceneTwo.
func TestContextCategorySceneTwoParity(t *testing.T) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	theString := Field[contextCategorySupportBean, string]("theString")
	intPrimitive := Field[contextCategorySupportBean, int]("intPrimitive")
	if _, err := CreateCategoryContext(env, "CtxCategory",
		Category("cat1", Greater[int](intPrimitive, Literal(0))),
		Category("cat2", Less[int](intPrimitive, Literal(0))),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextCategorySupportBean](env, "SupportBean").Aggregate(
		Alias("c1", theString),
		Alias("c2", Sum[int](intPrimitive)),
		Alias("c3", ContextLabel()),
		Alias("c4", ContextName()),
		Alias("c5", ContextID()),
	).Query(StatementName("s0"), WithContext("CtxCategory")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	statement := deployment.Statements()[0]

	assertCategoryPartitionInfo := func() {
		t.Helper()
		if got := statement.ContextPartitionCount(); got != 2 {
			t.Fatalf("category partition count = %d, want 2", got)
		}
		descriptors := statement.ContextPartitions()
		labels := []string{contextCategoryDescriptorLabel(descriptors[0]), contextCategoryDescriptorLabel(descriptors[1])}
		sort.Strings(labels)
		if !reflect.DeepEqual(labels, []string{"cat1", "cat2"}) {
			t.Fatalf("category labels = %#v", labels)
		}
		byID, ok, err := engine.ContextPartition("CtxCategory", 0)
		if err != nil || !ok {
			t.Fatalf("category id 0 = %#v, ok=%v, err=%v", byID, ok, err)
		}
		if contextCategoryDescriptorLabel(byID) != "cat1" {
			t.Fatalf("category id 0 label = %q", contextCategoryDescriptorLabel(byID))
		}
		props, ok, err := engine.ContextPartitionProperties("CtxCategory", 1)
		if err != nil || !ok || props["label"] != "cat2" {
			t.Fatalf("category id 1 properties = %#v, ok=%v, err=%v", props, ok, err)
		}
	}
	assertCategoryPartitionInfo()

	fields := []string{"c1", "c2", "c3", "c4", "c5"}
	rows := collectContextCategoryRows(t, deployment)
	send := func(theStringValue string, intValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: theStringValue, IntPrimitive: intValue}); err != nil {
			t.Fatal(err)
		}
	}
	send("G1", 1)
	assertContextCategoryRows(t, (*rows)[:1], fields, []any{"G1", 1, "cat1", "CtxCategory", 0})
	assertCategoryPartitionInfo()
	send("G2", -2)
	assertContextCategoryRows(t, (*rows)[1:], fields, []any{"G2", -2, "cat2", "CtxCategory", 1})
	send("G3", 3)
	assertContextCategoryRows(t, (*rows)[2:], fields, []any{"G3", 4, "cat1", "CtxCategory", 0})
	send("G4", -4)
	assertContextCategoryRows(t, (*rows)[3:], fields, []any{"G4", -6, "cat2", "CtxCategory", 1})
	send("G5", 5)
	assertContextCategoryRows(t, (*rows)[4:], fields, []any{"G5", 9, "cat1", "CtxCategory", 0})
}

// TestContextCategoryBooleanExpressionFilterParity covers
// ContextCategoryBooleanExprFilter's string-like predicates and ordered
// category counts.
func TestContextCategoryBooleanExpressionFilterParity(t *testing.T) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	theString := Field[contextCategorySupportBean, string]("theString")
	if _, err := CreateCategoryContext(env, "Ctx600a",
		Category("agroup", Like(theString, Literal("A%"))),
		Category("bgroup", Like(theString, Literal("B%"))),
		Category("cgroup", Like(theString, Literal("C%"))),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextCategorySupportBean](env, "SupportBean").Aggregate(
		Alias("c0", ContextLabel()),
		Alias("c1", CountAll()),
	).Query(StatementName("s0"), WithContext("Ctx600a")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectContextCategoryRows(t, deployment)
	fields := []string{"c0", "c1"}
	for _, test := range []struct {
		theString string
		label     string
		count     int64
	}{
		{"B1", "bgroup", 1},
		{"A1", "agroup", 1},
		{"B171771", "bgroup", 2},
		{"A  x", "agroup", 2},
	} {
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: test.theString, IntPrimitive: 1}); err != nil {
			t.Fatal(err)
		}
		if got := (*rows)[len(*rows)-1:]; !reflect.DeepEqual(contextCategoryRowValues(t, got[0], fields), []any{test.label, test.count}) {
			t.Fatalf("boolean filter row = %#v, want %v/%d", got[0].AsMap(), test.label, test.count)
		}
	}
	if len(*rows) != 4 {
		t.Fatalf("boolean filter rows = %#v", *rows)
	}
}

// TestContextCategoryContextPropertiesParity covers
// ContextCategoryWContextProps' preallocated empty aggregate rows, label/name
// built-ins and undeploy cleanup.
func TestContextCategoryContextPropertiesParity(t *testing.T) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	intPrimitive := Field[contextCategorySupportBean, int]("intPrimitive")
	if _, err := CreateCategoryContext(env, "CategorizedContext",
		Category("cat1", Less[int](intPrimitive, Literal(10))),
		Category("cat2", Between[int](intPrimitive, Literal(10), Literal(20))),
		Category("cat3", Greater[int](intPrimitive, Literal(20))),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextCategorySupportBean](env, "SupportBean").Aggregate(
		Alias("c0", ContextName()),
		Alias("c1", ContextLabel()),
		Alias("c2", Sum[int](intPrimitive)),
	).Query(StatementName("s0"), WithContext("CategorizedContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if statement.ContextPartitionCount() != 3 {
		t.Fatalf("category context preallocation = %d, want 3", statement.ContextPartitionCount())
	}
	fields := []string{"c0", "c1", "c2"}
	rows := collectContextCategoryRows(t, deployment)
	send := func(theStringValue string, intValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: theStringValue, IntPrimitive: intValue}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 5)
	assertContextCategoryRows(t, (*rows)[:1], fields, []any{"CategorizedContext", "cat1", 5})
	assertContextCategoryRowsAnyOrder(t, contextCategorySnapshot(t, statement, nil), fields,
		[]any{"CategorizedContext", "cat1", 5},
		[]any{"CategorizedContext", "cat2", nil},
		[]any{"CategorizedContext", "cat3", nil},
	)
	send("E2", 4)
	send("E3", 11)
	assertContextCategoryRows(t, (*rows)[1:], fields,
		[]any{"CategorizedContext", "cat1", 9},
		[]any{"CategorizedContext", "cat2", 11},
	)
	send("E4", 25)
	send("E5", 25)
	send("E6", 3)
	assertContextCategoryRows(t, (*rows)[3:], fields,
		[]any{"CategorizedContext", "cat3", 25},
		[]any{"CategorizedContext", "cat3", 50},
		[]any{"CategorizedContext", "cat1", 12},
	)
	assertContextCategoryRowsAnyOrder(t, contextCategorySnapshot(t, statement, nil), fields,
		[]any{"CategorizedContext", "cat1", 12},
		[]any{"CategorizedContext", "cat2", 11},
		[]any{"CategorizedContext", "cat3", 50},
	)
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	count, err := engine.ContextPartitionCount("CategorizedContext")
	if err != nil || count != 0 {
		t.Fatalf("category partitions after undeploy = %d, err=%v", count, err)
	}
}

// TestContextCategoryContextPartitionSelectionParity covers the ID/category
// iterator selectors, filtered descriptor selectors and invalid selector
// rejection used by ContextCategoryContextPartitionSelection.
func TestContextCategoryContextPartitionSelectionParity(t *testing.T) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	theString := Field[contextCategorySupportBean, string]("theString")
	intPrimitive := Field[contextCategorySupportBean, int]("intPrimitive")
	if _, err := CreateCategoryContext(env, "MyCtx",
		Category("grp1", Less[int](intPrimitive, Literal(-5))),
		Category("grp2", Between[int](intPrimitive, Literal(-5), Literal(5))),
		Category("grp3", Greater[int](intPrimitive, Literal(5))),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextCategorySupportBean](env, "SupportBean").GroupBy(theString).Select(
		Alias("c0", ContextID()),
		Alias("c1", ContextLabel()),
		Alias("c2", theString),
		Alias("c3", Sum[int](intPrimitive)),
	).Query(StatementName("s0"), WithContext("MyCtx")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	statement := deployment.Statements()[0]
	fields := []string{"c0", "c1", "c2", "c3"}
	send := func(theStringValue string, intValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: theStringValue, IntPrimitive: intValue}); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range []contextCategorySupportBean{
		{TheString: "E1", IntPrimitive: 1},
		{TheString: "E2", IntPrimitive: -5},
		{TheString: "E1", IntPrimitive: 2},
		{TheString: "E3", IntPrimitive: -100},
		{TheString: "E3", IntPrimitive: -8},
		{TheString: "E1", IntPrimitive: 60},
	} {
		send(event.TheString, event.IntPrimitive)
	}
	assertContextCategoryRowsAnyOrder(t, contextCategorySnapshot(t, statement, nil), fields,
		[]any{0, "grp1", "E3", -108},
		[]any{1, "grp2", "E1", 3},
		[]any{1, "grp2", "E2", -5},
		[]any{2, "grp3", "E1", 60},
	)
	assertContextCategoryRowsAnyOrder(t, contextCategorySnapshot(t, statement, SelectContextPartitionIDs(1)), fields,
		[]any{1, "grp2", "E1", 3},
		[]any{1, "grp2", "E2", -5},
	)
	assertContextCategoryRowsAnyOrder(t, contextCategorySnapshot(t, statement, SelectContextPartitionCategories("grp1", "grp3")), fields,
		[]any{0, "grp1", "E3", -108},
		[]any{2, "grp3", "E1", 60},
	)
	assertContextCategoryRowsAnyOrder(t, contextCategorySnapshot(t, statement, ContextPartitionSelectorDescriptorFunc(func(descriptor ContextPartitionDescriptor) bool {
		return contextCategoryDescriptorLabel(descriptor) == "grp1"
	})), fields, []any{0, "grp1", "E3", -108})

	collector := &contextCategoryCollectingSelector{}
	if got := contextCategorySnapshot(t, statement, collector); len(got) != 0 {
		t.Fatalf("always-false category selector rows = %#v", got)
	}
	if !reflect.DeepEqual(collector.labels, []string{"grp1", "grp2", "grp3"}) {
		t.Fatalf("filtered selector observed labels = %#v", collector.labels)
	}
	if _, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitions("category:grp1")); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("segmented key selector on category context error = %v, want InvalidRule", err)
	}
}

type contextCategoryCollectingSelector struct {
	labels []string
}

func (s *contextCategoryCollectingSelector) SelectContextPartition(string) bool { return true }

func (s *contextCategoryCollectingSelector) SelectContextPartitionDescriptor(descriptor ContextPartitionDescriptor) bool {
	s.labels = append(s.labels, contextCategoryDescriptorLabel(descriptor))
	return false
}

// TestContextCategorySingleCategoryPriorParity covers
// ContextCategorySingleCategorySODAPrior's keep-all prior navigation in one
// predefined category. The Go fluent Plan replaces the Java EPL/SODA textual
// pair with one immutable plan.
func TestContextCategorySingleCategoryPriorParity(t *testing.T) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	intPrimitive := Field[contextCategorySupportBean, int]("intPrimitive")
	if _, err := CreateCategoryContext(env, "CategorizedContext",
		Category("cat1", Less[int](intPrimitive, Literal(10))),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[contextCategorySupportBean](env, "SupportBean").Window(KeepAll()),
		Alias("c0", ContextName()),
		Alias("c1", ContextLabel()),
		Alias("c2", Prior[int](0, intPrimitive)),
	).Query(StatementName("s0"), WithContext("CategorizedContext")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectContextCategoryRows(t, deployment)
	fields := []string{"c0", "c1", "c2"}
	send := func(theStringValue string, intValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: theStringValue, IntPrimitive: intValue}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", 5)
	assertContextCategoryRows(t, (*rows)[:1], fields, []any{"CategorizedContext", "cat1", nil})
	send("E2", 20)
	if len(*rows) != 1 {
		t.Fatalf("out-of-category event produced output: %#v", *rows)
	}
	send("E1", 4)
	assertContextCategoryRows(t, (*rows)[1:], fields, []any{"CategorizedContext", "cat1", 5})
	if count, err := engine.ContextPartitionCount("CategorizedContext"); err != nil || count != 1 {
		t.Fatalf("category partition count = %d, err=%v", count, err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("CategorizedContext"); err != nil || count != 0 {
		t.Fatalf("category partitions after undeploy = %d, err=%v", count, err)
	}
}

// TestContextCategoryInvalidParity covers the invalid typed boundaries:
// unknown category filter fields, mismatched flat stream event type and an
// invalid category predicate. A non-boolean predicate is rejected by Go's
// generic Expression[bool] signature and therefore has no runtime invalid
// execution path.
func TestContextCategoryInvalidParity(t *testing.T) {
	t.Run("unknown filter field", func(t *testing.T) {
		env, engine := newContextCategoryParityEnvironment(t)
		defer func() { _ = engine.Close(context.Background()) }()
		if _, err := CreateCategoryContext(env, "ACtx",
			Category("cat1", Equal[int](Field[contextCategorySupportBean, int]("dummy"), Literal(1))),
		); err != nil {
			t.Fatal(err)
		}
		_, err := env.Build(From[contextCategorySupportBean](env, "SupportBean").Query(StatementName("s0"), WithContext("ACtx")))
		if err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), `unknown field "dummy"`) {
			t.Fatalf("unknown category field error = %v, want invalid unknown-field diagnostic", err)
		}
	})

	t.Run("mismatched statement event type", func(t *testing.T) {
		env := NewEnvironment()
		if _, err := RegisterStruct[contextCategorySupportBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[contextCategorySupportBeanS0](env, "SupportBean_S0"); err != nil {
			t.Fatal(err)
		}
		if _, err := CreateCategoryContext(env, "ACtx",
			Category("cat1", Less[int](Field[contextCategorySupportBean, int]("intPrimitive"), Literal(10))),
		); err != nil {
			t.Fatal(err)
		}
		_, err := env.Build(From[contextCategorySupportBeanS0](env, "SupportBean_S0").Query(StatementName("s0"), WithContext("ACtx")))
		if err == nil || !errors.Is(err, ErrorInvalidRule) || !strings.Contains(err.Error(), "requires that any of the event types") {
			t.Fatalf("mismatched category event type error = %v, want category event-type diagnostic", err)
		}
	})

	t.Run("invalid category predicate", func(t *testing.T) {
		if _, err := NewContextCategory("cat1", nil); err == nil || !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("nil category predicate error = %v, want invalid rule", err)
		}
	})
}

// contextCategoryDeclaredExpressionParity runs the shared Java
// ContextCategoryDeclaredExpr scenario. isAlias is retained in the test name
// for Java correspondence; the Go fluent API has no textual alias syntax and
// uses the same typed declaration for both variants.
func contextCategoryDeclaredExpressionParity(t *testing.T, isAlias bool) {
	env, engine := newContextCategoryParityEnvironment(t)
	defer func() { _ = engine.Close(context.Background()) }()
	intPrimitive := Field[contextCategorySupportBean, int]("intPrimitive")
	if _, err := CreateCategoryContext(env, "MyCtx",
		Category("n", Less[int](intPrimitive, Literal(0))),
		Category("p", Greater[int](intPrimitive, Literal(0))),
	); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("getLabelOne", ContextLabel()); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("getLabelTwo", Concat(Literal("x"), ContextLabel(), Literal("x"))); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("getLabelThree", ContextLabel()); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[contextCategorySupportBean](env, "SupportBean"),
		Alias("c0", ExpressionRef[string](env, "getLabelOne")),
		Alias("c1", ExpressionRef[string](env, "getLabelTwo")),
		Alias("c2", ExpressionRef[string](env, "getLabelThree")),
	).Query(StatementName("s0"), WithContext("MyCtx")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	rows := collectContextCategoryRows(t, deployment)
	fields := []string{"c0", "c1", "c2"}
	send := func(theStringValue string, intValue int) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), contextCategorySupportBean{TheString: theStringValue, IntPrimitive: intValue}); err != nil {
			t.Fatal(err)
		}
	}
	send("E1", -2)
	assertContextCategoryRows(t, (*rows)[:1], fields, []any{"n", "xnx", "n"})
	send("E2", 1)
	assertContextCategoryRows(t, (*rows)[1:], fields, []any{"p", "xpx", "p"})
}

func TestContextCategoryDeclaredExpressionCallMatchesEsper(t *testing.T) {
	contextCategoryDeclaredExpressionParity(t, false)
}

func TestContextCategoryDeclaredExpressionAliasMatchesEsper(t *testing.T) {
	contextCategoryDeclaredExpressionParity(t, true)
}

func contextCategoryDescriptorLabel(d ContextPartitionDescriptor) string {
	value, ok := d.Property("label")
	if !ok || value.Any() == nil {
		return ""
	}
	text, _ := value.Any().(string)
	return text
}
