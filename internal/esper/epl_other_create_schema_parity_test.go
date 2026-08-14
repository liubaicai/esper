package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// eplCreateSchemaSupportBean mirrors SupportBean for EPLOtherCreateSchema.
type eplCreateSchemaSupportBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

// eplCreateSchemaST0 mirrors SupportBean_ST0 (id, p00).
type eplCreateSchemaST0 struct {
	ID  string `esper:"id"`
	P00 int    `esper:"p00"`
}

// eplCreateSchemaGeneric mirrors the generic-typed fields used by
// EPLOtherCreateSchemaTypeParameterized.
type eplCreateSchemaGeneric struct {
	ListOfString          []string       `esper:"listOfString"`
	ListOfOptionalInteger []*int         `esper:"listOfOptionalInteger"`
	MapOfStringAndInteger map[string]int `esper:"mapOfStringAndInteger"`
	ListArrayOfString     [][]string     `esper:"listArrayOfString"`
	ListArray2DimOfString [][][]string   `esper:"listArray2DimOfString"`
}

func newEPLOtherCreateSchemaEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[eplCreateSchemaSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[eplCreateSchemaST0](env, "SupportBean_ST0"); err != nil {
		t.Fatal(err)
	}
	return env
}

func eplCreateSchemaRows(t *testing.T, stmt *Statement) func() []Row {
	t.Helper()
	var rows []Row
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Row {
		return append([]Row(nil), rows...)
	}
}

func eplCreateSchemaResults(t *testing.T, stmt *Statement) func() []Result {
	t.Helper()
	var results []Result
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		results = append(results, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return func() []Result {
		return append([]Result(nil), results...)
	}
}

func eplCreateSchemaAssertRows(t *testing.T, rows []Row, field string, want []any, label string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("%s: got %d rows, want %d: %#v", label, len(rows), len(want), rows)
	}
	for index, expected := range want {
		got := rows[index].Get(field).Any()
		if got != expected {
			t.Fatalf("%s row %d: got %#v, want %#v", label, index, got, expected)
		}
	}
}

// TestEPLOtherCreateSchemaPathAndPublicParity mirrors
// EPLOtherCreateSchemaPathSimple, EPLOtherCreateSchemaPublicSimple and
// EPLOtherCreateSchemaConfiguredNotRemoved.
func TestEPLOtherCreateSchemaPathAndPublicParity(t *testing.T) {
	env := newEPLOtherCreateSchemaEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	t.Run("path-simple", func(t *testing.T) {
		if _, err := RegisterMap(env, "SimpleSchema", []FieldSpec{
			FieldDef("p0", reflect.TypeOf("")),
			FieldDef("p1", reflect.TypeOf(0)),
		}); err != nil {
			t.Fatal(err)
		}
		insertPlan, err := env.Build(Select(
			From[eplCreateSchemaSupportBean](env, "SupportBean"),
			Alias("p0", Field[eplCreateSchemaSupportBean, string]("theString")),
			Alias("p1", Field[eplCreateSchemaSupportBean, int]("intPrimitive")),
		).InsertInto("SimpleSchema", StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		selectPlan, err := env.Build(FromAny(env, "SimpleSchema").Select(
			Alias("p0", Field[any, string]("p0")),
			Alias("p1", Field[any, int]("p1")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.DeployPlans(context.Background(), []Plan{insertPlan, selectPlan})
		if err != nil {
			t.Fatal(err)
		}
		rows := eplCreateSchemaRows(t, eplCreateSchemaStatement(t, deployment, "s0"))
		if err := engine.SendEvent(context.Background(), eplCreateSchemaSupportBean{TheString: "a", IntPrimitive: 20}); err != nil {
			t.Fatal(err)
		}
		eplCreateSchemaAssertRows(t, rows(), "p0", []any{"a"}, "path-p0")
		eplCreateSchemaAssertRows(t, rows(), "p1", []any{20}, "path-p1")
	})

	t.Run("public-simple", func(t *testing.T) {
		if _, err := RegisterMap(env, "MySchema", []FieldSpec{
			FieldDef("p0", reflect.TypeOf("")),
			FieldDef("p1", reflect.TypeOf(0)),
		}); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(FromAny(env, "MySchema").Select(
			Alias("p0", Field[any, string]("p0")),
			Alias("p1", Field[any, int]("p1")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		rows := eplCreateSchemaRows(t, deployment.Statements()[0])
		if err := engine.Send(context.Background(), "MySchema", map[string]any{"p0": "a", "p1": 20}); err != nil {
			t.Fatal(err)
		}
		eplCreateSchemaAssertRows(t, rows(), "p0", []any{"a"}, "public-p0")
		eplCreateSchemaAssertRows(t, rows(), "p1", []any{20}, "public-p1")
	})

	t.Run("configured-not-removed", func(t *testing.T) {
		if _, err := RegisterMap(env, "ABCType", []FieldSpec{
			FieldDef("col1", reflect.TypeOf(0)),
			FieldDef("col2", reflect.TypeOf(0)),
		}); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(FromAny(env, "ABCType").Query(StatementName("abc")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := deployment.Undeploy(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, ok := env.Schema("ABCType"); !ok {
			t.Fatal("registered schema disappeared after undeploy")
		}
		if _, err := RegisterMap(env, "ABCType", []FieldSpec{FieldDef("col1", reflect.TypeOf(0))}); err == nil {
			t.Fatal("duplicate schema registration was accepted")
		}
	})
}

func eplCreateSchemaStatement(t *testing.T, deployment *Deployment, name string) *Statement {
	t.Helper()
	for _, statement := range deployment.Statements() {
		if statement.Name() == name {
			return statement
		}
	}
	t.Fatalf("statement %q not found", name)
	return nil
}

// TestEPLOtherCreateSchemaCopyAndInheritParity mirrors
// EPLOtherCreateSchemaCopyFromOrderObjectArray, EPLOtherCreateSchemaCopyProperties,
// EPLOtherCreateSchemaInherit and EPLOtherCreateSchemaCopyFromDeepWithValueObject.
func TestEPLOtherCreateSchemaCopyAndInheritParity(t *testing.T) {
	env := newEPLOtherCreateSchemaEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	t.Run("object-array-copyfrom", func(t *testing.T) {
		one, err := RegisterObjectArray(env, "MyEventOne", []FieldSpec{
			FieldDef("p0", reflect.TypeOf("")),
			FieldDef("p1", reflect.TypeOf(0.0)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterObjectArray(env, "MyEventTwo", []FieldSpec{
			FieldDef("p2", reflect.TypeOf("")),
		}, WithSchemaCopiedFrom(one)); err != nil {
			t.Fatal(err)
		}
		insertPlan, err := env.Build(FromAny(env, "MyEventOne").Select(
			Alias("p2", Literal("abc")),
			Alias("p0", Field[any, string]("p0")),
			Alias("p1", Field[any, float64]("p1")),
		).InsertInto("MyEventTwo", StatementName("insert")))
		if err != nil {
			t.Fatal(err)
		}
		selectPlan, err := env.Build(FromAny(env, "MyEventTwo").Select(
			Alias("p0", Field[any, string]("p0")),
			Alias("p1", Field[any, float64]("p1")),
			Alias("p2", Field[any, string]("p2")),
		).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.DeployPlans(context.Background(), []Plan{insertPlan, selectPlan})
		if err != nil {
			t.Fatal(err)
		}
		rows := eplCreateSchemaRows(t, eplCreateSchemaStatement(t, deployment, "s0"))
		if err := engine.SendObjectArray(context.Background(), "MyEventOne", []any{"E1", 10.0}); err != nil {
			t.Fatal(err)
		}
		got := rows()
		if len(got) != 1 {
			t.Fatalf("object-array copyfrom rows = %#v", got)
		}
		for field, want := range map[string]any{"p0": "E1", "p1": 10.0, "p2": "abc"} {
			if got[0].Get(field).Any() != want {
				t.Fatalf("object-array copyfrom %s = %#v, want %#v", field, got[0].Get(field).Any(), want)
			}
		}
	})

	t.Run("copy-properties", func(t *testing.T) {
		baseOne, err := RegisterMap(env, "BaseOne", []FieldSpec{
			FieldDef("prop1", reflect.TypeOf("")),
			FieldDef("prop2", reflect.TypeOf(0)),
		})
		if err != nil {
			t.Fatal(err)
		}
		baseTwo, err := RegisterMap(env, "BaseTwo", []FieldSpec{
			FieldDef("prop3", reflect.TypeOf(int64(0))),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "E1", []FieldSpec{}, WithSchemaCopiedFrom(baseOne)); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "E2", []FieldSpec{}, WithSchemaCopiedFrom(baseOne), WithSchemaCopiedFrom(baseTwo)); err != nil {
			t.Fatal(err)
		}
		e1Plan, err := env.Build(FromAny(env, "E1").Query(StatementName("e1")))
		if err != nil {
			t.Fatal(err)
		}
		e2Plan, err := env.Build(FromAny(env, "E2").Query(StatementName("e2")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.DeployPlans(context.Background(), []Plan{e1Plan, e2Plan})
		if err != nil {
			t.Fatal(err)
		}
		e1Results := eplCreateSchemaResults(t, eplCreateSchemaStatement(t, deployment, "e1"))
		if err := engine.Send(context.Background(), "E1", map[string]any{"prop1": "v1", "prop2": 2}); err != nil {
			t.Fatal(err)
		}
		gotE1 := e1Results()
		if len(gotE1) != 1 || gotE1[0].Get("prop1").Any() != "v1" || gotE1[0].Get("prop2").Any() != 2 {
			t.Fatalf("E1 rows = %#v", gotE1)
		}
		e2Results := eplCreateSchemaResults(t, eplCreateSchemaStatement(t, deployment, "e2"))
		if err := engine.Send(context.Background(), "E2", map[string]any{"prop1": "v1", "prop2": 2, "prop3": int64(3)}); err != nil {
			t.Fatal(err)
		}
		gotE2 := e2Results()
		if len(gotE2) != 1 || gotE2[0].Get("prop3").Any() != int64(3) {
			t.Fatalf("E2 rows = %#v", gotE2)
		}

		myType, err := RegisterMap(env, "MyType", []FieldSpec{
			FieldDef("a", reflect.TypeOf("")),
			FieldDef("b", reflect.TypeOf("")),
			FieldDef("c", reflect.TypeOf(map[string]any{})),
			FieldDef("d", reflect.TypeOf([]map[string]any{})),
		}, WithNestedPropertySchema("c", baseOne), WithNestedPropertySchema("d", baseTwo))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "E3", []FieldSpec{
			FieldDef("e", reflect.TypeOf(int64(0))),
			FieldDef("f", reflect.TypeOf(map[string]any{})),
		}, WithSchemaCopiedFrom(myType), WithNestedPropertySchema("f", baseOne)); err != nil {
			t.Fatal(err)
		}
		e3Plan, err := env.Build(FromAny(env, "E3").Query(StatementName("e3")))
		if err != nil {
			t.Fatal(err)
		}
		e3Deployment, err := engine.Deploy(context.Background(), e3Plan)
		if err != nil {
			t.Fatal(err)
		}
		e3Results := eplCreateSchemaResults(t, e3Deployment.Statements()[0])
		if err := engine.Send(context.Background(), "E3", map[string]any{
			"a": "va", "b": "vb",
			"c": map[string]any{"prop1": "c1", "prop2": 2},
			"d": []map[string]any{{"prop3": int64(3)}},
			"e": int64(4),
			"f": map[string]any{"prop1": "f1", "prop2": 5},
		}); err != nil {
			t.Fatal(err)
		}
		gotE3 := e3Results()
		if len(gotE3) != 1 {
			t.Fatalf("E3 rows = %#v", gotE3)
		}
		if gotE3[0].Get("c.prop1").Any() != "c1" || gotE3[0].Get("d[0].prop3").Any() != int64(3) || gotE3[0].Get("f.prop2").Any() != 5 {
			t.Fatalf("E3 nested rows = %#v", gotE3[0])
		}
	})

	t.Run("inherit", func(t *testing.T) {
		parent, err := RegisterMap(env, "MyParentType", []FieldSpec{
			FieldDef("col1", reflect.TypeOf(0)),
			FieldDef("col2", reflect.TypeOf("")),
		})
		if err != nil {
			t.Fatal(err)
		}
		childOne, err := RegisterMap(env, "MyChildTypeOne", []FieldSpec{
			FieldDef("col3", reflect.TypeOf(0)),
		}, WithSchemaParent(parent))
		if err != nil {
			t.Fatal(err)
		}
		childTwo, err := RegisterMap(env, "MyChildTypeTwo", []FieldSpec{
			FieldDef("col4", reflect.TypeOf(false)),
		})
		if err != nil {
			t.Fatal(err)
		}
		grandchild, err := RegisterMap(env, "MyChildChildType", []FieldSpec{
			FieldDef("col5", reflect.TypeOf(int16(0))),
			FieldDef("col6", reflect.TypeOf(int64(0))),
		}, WithSchemaParent(childOne), WithSchemaParent(childTwo))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := grandchild.PropertyNames(), []string{"col1", "col2", "col3", "col4", "col5", "col6"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("grandchild property names = %#v, want %#v", got, want)
		}
		plan, err := env.Build(FromAny(env, "MyChildChildType").Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		results := eplCreateSchemaResults(t, deployment.Statements()[0])
		if err := engine.Send(context.Background(), "MyChildChildType", map[string]any{
			"col1": 1, "col2": "a", "col3": 2, "col4": true, "col5": int16(3), "col6": int64(4),
		}); err != nil {
			t.Fatal(err)
		}
		got := results()
		if len(got) != 1 {
			t.Fatalf("inherit rows = %#v", got)
		}
		for field, want := range map[string]any{"col1": 1, "col3": 2, "col4": true, "col5": int16(3), "col6": int64(4)} {
			if got[0].Get(field).Any() != want {
				t.Fatalf("inherit %s = %#v, want %#v", field, got[0].Get(field).Any(), want)
			}
		}
	})

	t.Run("copyfrom-deep", func(t *testing.T) {
		a, err := RegisterMap(env, "SchemaA", []FieldSpec{
			FieldDef("account", reflect.TypeOf("")),
			FieldDef("foo", reflect.TypeOf((*any)(nil)).Elem()),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "SchemaB", []FieldSpec{
			FieldDef("symbol", reflect.TypeOf("")),
		}, WithSchemaCopiedFrom(a)); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "SchemaC", []FieldSpec{}, WithSchemaCopiedFrom(a)); err != nil {
			t.Fatal(err)
		}
		schemaC, _ := env.Schema("SchemaC")
		if got, want := schemaC.PropertyNames(), []string{"account", "foo"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("SchemaC property names = %#v, want %#v", got, want)
		}
	})
}

// TestEPLOtherCreateSchemaRepresentationParity mirrors
// EPLOtherCreateSchemaColDefPlain, EPLOtherCreateSchemaArrayPrimitiveType,
// EPLOtherCreateSchemaNestableMapArray, EPLOtherCreateSchemaModelPOJO and
// EPLOtherCreateSchemaAvroSchemaWAnnotation.
func TestEPLOtherCreateSchemaRepresentationParity(t *testing.T) {
	env := newEPLOtherCreateSchemaEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	t.Run("col-def-plain", func(t *testing.T) {
		schema, err := RegisterMap(env, "MyEventType", []FieldSpec{
			FieldDef("col1", reflect.TypeOf("")),
			FieldDef("col2", reflect.TypeOf(0)),
			FieldDef("col3col4", reflect.TypeOf(0)),
			FieldDef("f4", reflect.TypeOf((*any)(nil)).Elem()),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := schema.PropertyNames(); !reflect.DeepEqual(got, []string{"col1", "col2", "col3col4", "f4"}) {
			t.Fatalf("col-def property names = %#v", got)
		}
		plan, err := env.Build(FromAny(env, "MyEventType").Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		results := eplCreateSchemaResults(t, deployment.Statements()[0])
		if err := engine.Send(context.Background(), "MyEventType", map[string]any{
			"col1": "abc", "col2": 1, "col3col4": 2, "f4": nil,
		}); err != nil {
			t.Fatal(err)
		}
		got := results()
		if len(got) != 1 || got[0].Get("col1").Any() != "abc" || got[0].Get("col2").Any() != 1 || got[0].Get("col3col4").Any() != 2 || !got[0].Get("f4").IsNull() {
			t.Fatalf("col-def rows = %#v", got)
		}
	})

	t.Run("array-primitive-type", func(t *testing.T) {
		schema, err := RegisterMap(env, "MySchema", []FieldSpec{
			FieldDef("c0", reflect.TypeOf([]int{})),
			FieldDef("c1", reflect.TypeOf([]*int{})),
		})
		if err != nil {
			t.Fatal(err)
		}
		c0, ok0 := schema.Field("c0")
		c1, ok1 := schema.Field("c1")
		if !ok0 || !ok1 || c0.Type != reflect.TypeOf([]int{}) || c1.Type != reflect.TypeOf([]*int{}) {
			t.Fatalf("array primitive fields = %#v / %#v", c0, c1)
		}
	})

	t.Run("nestable-map-array", func(t *testing.T) {
		inner, err := RegisterMap(env, "MyInnerType", []FieldSpec{
			FieldDef("inn1", reflect.TypeOf([]string{})),
			FieldDef("inn2", reflect.TypeOf([]int{})),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterMap(env, "MyOuterType", []FieldSpec{
			FieldDef("col1", reflect.TypeOf(map[string]any{})),
			FieldDef("col2", reflect.TypeOf([]map[string]any{})),
		}, WithNestedPropertySchema("col1", inner), WithNestedPropertySchema("col2", inner)); err != nil {
			t.Fatal(err)
		}
		plan, err := env.Build(FromAny(env, "MyOuterType").Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatal(err)
		}
		results := eplCreateSchemaResults(t, deployment.Statements()[0])
		if err := engine.Send(context.Background(), "MyOuterType", map[string]any{
			"col1": map[string]any{"inn1": []string{"abc", "def"}, "inn2": []int{1, 2}},
			"col2": []map[string]any{
				{"inn1": []string{"abc", "def"}, "inn2": []int{1, 2}},
				{"inn1": []string{"abc", "def"}, "inn2": []int{1, 2}},
			},
		}); err != nil {
			t.Fatal(err)
		}
		got := results()
		if len(got) != 1 || got[0].Get("col1.inn1[1]").Any() != "def" || got[0].Get("col2[1].inn2[1]").Any() != 2 {
			t.Fatalf("nestable rows = %#v", got)
		}
	})

	t.Run("model-pojo", func(t *testing.T) {
		if _, err := RegisterStruct[eplCreateSchemaST0](env, "SupportBeanOne"); err != nil {
			t.Fatal(err)
		}
		if _, err := RegisterStruct[eplCreateSchemaST0](env, "SupportBeanTwo"); err != nil {
			t.Fatal(err)
		}
		onePlan, err := env.Build(FromAny(env, "SupportBeanOne").Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		twoPlan, err := env.Build(FromAny(env, "SupportBeanTwo").Query(StatementName("s1")))
		if err != nil {
			t.Fatal(err)
		}
		deployment, err := engine.DeployPlans(context.Background(), []Plan{onePlan, twoPlan})
		if err != nil {
			t.Fatal(err)
		}
		oneResults := eplCreateSchemaResults(t, eplCreateSchemaStatement(t, deployment, "s0"))
		twoResults := eplCreateSchemaResults(t, eplCreateSchemaStatement(t, deployment, "s1"))
		if err := engine.Send(context.Background(), "SupportBeanOne", eplCreateSchemaST0{ID: "E1", P00: 2}); err != nil {
			t.Fatal(err)
		}
		if len(oneResults()) != 1 || len(twoResults()) != 0 {
			t.Fatalf("SupportBeanOne rows = %#v, SupportBeanTwo rows = %#v", oneResults(), twoResults())
		}
		if err := engine.Send(context.Background(), "SupportBeanTwo", eplCreateSchemaST0{ID: "E2", P00: 3}); err != nil {
			t.Fatal(err)
		}
		if len(oneResults()) != 1 || len(twoResults()) != 1 {
			t.Fatalf("SupportBeanOne rows = %#v, SupportBeanTwo rows = %#v", oneResults(), twoResults())
		}
	})

	t.Run("avro-schema", func(t *testing.T) {
		schema, err := RegisterAvro(env, "MyAvroEvent", []FieldSpec{
			FieldDef("carId", reflect.TypeOf(0)),
			FieldDef("name", reflect.TypeOf("")),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := schema.PropertyNames(); !reflect.DeepEqual(got, []string{"carId", "name"}) {
			t.Fatalf("avro schema properties = %#v", got)
		}
	})
}

// TestEPLOtherCreateSchemaVariantParity mirrors EPLOtherCreateSchemaVariantType:
// predefined common-field variants, predefined-with-any, and any variants.
func TestEPLOtherCreateSchemaVariantParity(t *testing.T) {
	env := newEPLOtherCreateSchemaEnvironment(t)
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	zero, err := RegisterMap(env, "MyTypeZero", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col2", reflect.TypeOf("")),
	})
	if err != nil {
		t.Fatal(err)
	}
	one, err := RegisterMap(env, "MyTypeOne", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col3", reflect.TypeOf("")),
		FieldDef("col4", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "MyTypeTwo", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col4", reflect.TypeOf(false)),
		FieldDef("col5", reflect.TypeOf(int16(0))),
	}); err != nil {
		t.Fatal(err)
	}

	predef, err := RegisterVariant(env, "MyVariantPredef", zero, one)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := predef.PropertyNames(), []string{"col1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("predefined variant properties = %#v, want %#v", got, want)
	}
	for _, source := range []string{"MyTypeZero", "MyTypeOne"} {
		plan, err := env.Build(FromAny(env, source).InsertInto("MyVariantPredef", StatementName("insert-"+source)))
		if err != nil {
			t.Fatalf("insert %s into predefined variant: %v", source, err)
		}
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.Build(FromAny(env, "MyTypeTwo").InsertInto("MyVariantPredef", StatementName("invalid"))); err == nil {
		t.Fatal("non-member insert into predefined variant was accepted")
	} else if !strings.Contains(err.Error(), "member") {
		t.Fatalf("predefined variant member error = %v", err)
	}

	if _, err := RegisterVariantAny(env, "MyVariantAny"); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"MyTypeZero", "MyTypeOne", "MyTypeTwo"} {
		plan, err := env.Build(FromAny(env, source).InsertInto("MyVariantAny", StatementName("insert-any-"+source)))
		if err != nil {
			t.Fatalf("insert %s into any variant: %v", source, err)
		}
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
	}
}

// TestEPLOtherCreateSchemaInvalidParity mirrors EPLOtherCreateSchemaInvalid:
// duplicate columns/types, duplicate names, unknown supertypes and invalid
// object-array inheritance are rejected.
func TestEPLOtherCreateSchemaInvalidParity(t *testing.T) {
	env := newEPLOtherCreateSchemaEnvironment(t)
	if _, err := RegisterMap(env, "DuplicateType", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "DuplicateType", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
	}); err == nil {
		t.Fatal("duplicate schema registration was accepted")
	}
	if _, err := RegisterMap(env, "DuplicateColumns", []FieldSpec{
		FieldDef("col1", reflect.TypeOf(0)),
		FieldDef("col1", reflect.TypeOf("")),
	}); err == nil {
		t.Fatal("duplicate schema columns were accepted")
	}
	if _, err := RegisterMap(env, "UnknownNested", []FieldSpec{
		FieldDef("c", reflect.TypeOf(map[string]any{})),
	}, WithNestedPropertySchema("c", Schema{})); err == nil {
		t.Fatal("unknown nested schema was accepted")
	}

	one, err := RegisterObjectArray(env, "OAOne", []FieldSpec{FieldDef("a", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	two, err := RegisterObjectArray(env, "OATwo", []FieldSpec{FieldDef("b", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "OAInvalid", []FieldSpec{}, WithSchemaParent(one), WithSchemaParent(two)); err == nil {
		t.Fatal("multi-parent object-array schema was accepted")
	}
}

// TestEPLOtherCreateSchemaTypeParameterizedParity mirrors
// EPLOtherCreateSchemaTypeParameterized: typed generic collections/maps/arrays
// survive schema registration and projection.
func TestEPLOtherCreateSchemaTypeParameterizedParity(t *testing.T) {
	env := NewEnvironment()
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	schema, err := RegisterStruct[eplCreateSchemaGeneric](env, "MyEvent")
	if err != nil {
		t.Fatal(err)
	}
	wantFields := []string{"listOfString", "listOfOptionalInteger", "mapOfStringAndInteger", "listArrayOfString", "listArray2DimOfString"}
	if got := schema.PropertyNames(); !reflect.DeepEqual(got, wantFields) {
		t.Fatalf("generic schema properties = %#v, want %#v", got, wantFields)
	}
	plan, err := env.Build(From[eplCreateSchemaGeneric](env, "MyEvent").Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	results := eplCreateSchemaResults(t, deployment.Statements()[0])
	optional := 2
	event := eplCreateSchemaGeneric{
		ListOfString:          []string{"a", "b"},
		ListOfOptionalInteger: []*int{&optional},
		MapOfStringAndInteger: map[string]int{"k": 1},
		ListArrayOfString:     [][]string{{"x", "y"}},
		ListArray2DimOfString: [][][]string{{{"z"}}},
	}
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	got := results()
	if len(got) != 1 {
		t.Fatalf("generic rows = %#v", got)
	}
	if !reflect.DeepEqual(got[0].Get("listOfString").Any(), event.ListOfString) ||
		!reflect.DeepEqual(got[0].Get("mapOfStringAndInteger").Any(), event.MapOfStringAndInteger) ||
		!reflect.DeepEqual(got[0].Get("listArrayOfString").Any(), event.ListArrayOfString) {
		t.Fatalf("generic values = %#v", got[0])
	}
}

// eplCreateSchemaAssertNoRows is a tiny helper used by a few subtests where a
// statement must not fire.
func eplCreateSchemaAssertNoRows(t *testing.T, rows []Row, label string) {
	t.Helper()
	if len(rows) != 0 {
		t.Fatalf("%s: unexpected rows %#v", label, rows)
	}
}
