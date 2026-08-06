package esper

import (
	"context"
	"reflect"
	"testing"
)

type dotParityObjectEvent struct {
	ID    string `esper:"id"`
	Score int    `esper:"score"`
}

func (event dotParityObjectEvent) Equals(other dotParityObjectEvent) bool {
	return event.ID == other.ID
}

type dotParityEnumNested struct {
	Value   int      `esper:"value"`
	Strings []string `esper:"strings"`
}

func (nested dotParityEnumNested) GetValue() int { return nested.Value }

func (nested dotParityEnumNested) GetMyStringsNestedAsList() []string {
	return append([]string(nil), nested.Strings...)
}

type dotParityEnum struct {
	Associated int                 `esper:"associated"`
	Nested     dotParityEnumNested `esper:"nested"`
	Strings    []string            `esper:"strings"`
}

func (value dotParityEnum) GetAssociatedValue() int { return value.Associated }

func (value dotParityEnum) CheckAssociatedValue(candidate int) bool {
	return value.Associated == candidate
}

func (value dotParityEnum) GetNested() dotParityEnumNested { return value.Nested }

func (value dotParityEnum) CheckEventBeanPropInt(event dotParityEnumEvent, name string) bool {
	if name != "intPrimitive" {
		return false
	}
	return value.Associated == event.IntPrimitive
}

func (value dotParityEnum) GetMyStringsAsList() []string {
	return append([]string(nil), value.Strings...)
}

type dotParityEnumEvent struct {
	IntPrimitive int `esper:"intPrimitive"`
}

type dotParityInner struct {
	IDs []int `esper:"ids"`
}

func (inner dotParityInner) GetIDs(index int) int {
	if index < 0 || index >= len(inner.IDs) {
		return 999999
	}
	return inner.IDs[index]
}

func (inner dotParityInner) GetIDsWithSuffix(index int, suffix string) int {
	if suffix != "xyz" {
		return 999999
	}
	return inner.GetIDs(index)
}

func (inner dotParityInner) GetIDsWithEvent(_ dotParityErasureEvent, suffix string) int {
	if suffix != "xyz" {
		return 999999
	}
	return 999999
}

type dotParityErasureEvent struct {
	Key             string                    `esper:"key"`
	Subkey          int                       `esper:"subkey"`
	InnerTypes      map[string]dotParityInner `esper:"innerTypes"`
	InnerTypesArray []dotParityInner          `esper:"innerTypesArray"`
}

type dotParityChainTop struct {
	Seed string `esper:"seed"`
}

type dotParityChainOne struct {
	Prefix string
	Number int
}

func (top dotParityChainTop) GetChildOne(prefix string, number int) dotParityChainOne {
	return dotParityChainOne{Prefix: prefix, Number: number}
}

type dotParityChainTwo struct {
	Text string
}

func (child dotParityChainOne) GetChildTwo(suffix string) dotParityChainTwo {
	return dotParityChainTwo{Text: child.Prefix + suffix}
}

func (child dotParityChainTwo) GetText() string { return child.Text }

type dotParityNestedThree struct {
	Value string `esper:"value"`
}

func (nested dotParityNestedThree) GetCustomLevelThree(value int) string {
	return nested.Value + ":" + formatDotInt(value)
}

type dotParityNestedTwo struct {
	LevelThree dotParityNestedThree `esper:"levelThree"`
}

func (nested dotParityNestedTwo) GetCustomLevelTwo(value int) string {
	return nested.LevelThree.Value + ":" + formatDotInt(value)
}

type dotParityNestedOne struct {
	LevelTwo dotParityNestedTwo `esper:"levelTwo"`
	Value    string             `esper:"value"`
}

func (nested dotParityNestedOne) GetCustomLevelOne(value int) string {
	return nested.Value + ":" + formatDotInt(value)
}

func (nested dotParityNestedOne) GetNestLevOneVal() string { return nested.Value }

type dotParityCombined struct {
	Array []dotParityNestedOne `esper:"array"`
}

func (combined dotParityCombined) GetArray() []dotParityNestedOne {
	return append([]dotParityNestedOne(nil), combined.Array...)
}

type dotParityComplexEvent struct {
	ArrayProperty []int              `esper:"arrayProperty"`
	Abc           dotParityCombined  `esper:"abc"`
	LevelOne      dotParityNestedOne `esper:"levelOne"`
	MapProperty   map[string]string  `esper:"mapProperty"`
}

type dotParityNode struct {
	ID string `esper:"id"`
}

type dotParityNodeData struct {
	NodeID string `esper:"nodeId"`
	Value  string `esper:"value"`
}

func (node dotParityNode) Compute(data dotParityNodeData) string {
	return node.ID + ":" + data.Value
}

type dotParityNodeWithData struct {
	Node dotParityNode     `esper:"node"`
	Data dotParityNodeData `esper:"data"`
}

type dotParityCollectionEvent struct {
	P00     string            `esper:"p00"`
	P01     string            `esper:"p01"`
	Numbers []int             `esper:"numbers"`
	MapData map[string]string `esper:"mapData"`
	Amount  dotParityAmount   `esper:"amount"`
}

type dotParityAmount struct {
	Value int `esper:"value"`
}

func (amount dotParityAmount) Abs() dotParityAmount {
	if amount.Value < 0 {
		amount.Value = -amount.Value
	}
	return amount
}

type dotParityFactory struct{}

type dotParitySimpleBean struct {
	TheString string `esper:"theString"`
}

func (dotParityFactory) ReturnBean(value string) dotParitySimpleBean {
	return dotParitySimpleBean{TheString: value}
}

func formatDotInt(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

func collectDotRows(t *testing.T, deployment *Deployment) *[]Row {
	t.Helper()
	rows := make([]Row, 0)
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				t.Fatalf("dot expression result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &rows
}

func deployDotPlan(t *testing.T, env *Environment, engine *Engine, query Query) *Deployment {
	t.Helper()
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func TestDotExpressionsMatchJavaObjectEqualityAndEnumChains(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dotParityObjectEvent](env, "DotParityObjectEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dotParityEnumEvent](env, "DotParityEnumEvent"); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	object := From[dotParityObjectEvent](env, "DotParityObjectEvent").Window(KeepAll())
	current := EventValue[dotParityObjectEvent]()
	maximum := MaxBy[dotParityObjectEvent, int](current, Field[dotParityObjectEvent, int]("score"))
	objectDeployment := deployDotPlan(t, env, engine, object.Aggregate(
		Alias("same", Method[bool](current, "Equals", maximum)),
	).Query(StatementName("dot-object-equals")))
	objectRows := collectDotRows(t, objectDeployment)
	for _, event := range []dotParityObjectEvent{
		{ID: "e1", Score: 10},
		{ID: "e2", Score: 9},
		{ID: "e3", Score: 11},
		{ID: "e4", Score: 8},
		{ID: "e5", Score: 11},
		{ID: "e6", Score: 12},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(*objectRows) != 6 {
		t.Fatalf("object equals rows = %d", len(*objectRows))
	}
	for index, expected := range []bool{true, false, true, false, false, true} {
		if got := (*objectRows)[index].Get("same").Any(); got != expected {
			t.Fatalf("object equals row %d = %#v, want %v", index, got, expected)
		}
	}

	enumOne := dotParityEnum{Associated: 100, Nested: dotParityEnumNested{Value: 100}, Strings: []string{"1", "0", "0"}}
	enumTwo := dotParityEnum{Associated: 200, Nested: dotParityEnumNested{Value: 200, Strings: []string{"2", "0", "0"}}, Strings: []string{"2", "0", "0"}}
	enumThree := dotParityEnum{Associated: 300, Nested: dotParityEnumNested{Value: 300, Strings: []string{"3", "0", "0"}}}
	valueOne := Literal(enumOne)
	valueTwo := Literal(enumTwo)
	valueThree := Literal(enumThree)
	nestedTwo := Method[dotParityEnumNested](valueTwo, "GetNested")
	nestedThree := Method[dotParityEnumNested](valueThree, "GetNested")
	enumSource := From[dotParityEnumEvent](env, "DotParityEnumEvent")
	enumDeployment := deployDotPlan(t, env, engine, Select(enumSource,
		Alias("c0", Equal[int](Field[dotParityEnumEvent, int]("intPrimitive"), Method[int](valueOne, "GetAssociatedValue"))),
		Alias("c1", Method[bool](valueTwo, "CheckAssociatedValue", Field[dotParityEnumEvent, int]("intPrimitive"))),
		Alias("c2", Method[int](nestedThree, "GetValue")),
		Alias("c3", Method[bool](valueTwo, "CheckEventBeanPropInt", EventValue[dotParityEnumEvent](), Literal("intPrimitive"))),
		Alias("c4", Method[bool](valueTwo, "CheckEventBeanPropInt", EventValue[dotParityEnumEvent](), Literal("intPrimitive"))),
		Alias("c5", Method[[]string](valueTwo, "GetMyStringsAsList")),
		Alias("c6", Method[[]string](nestedTwo, "GetMyStringsNestedAsList")),
	).Query(StatementName("dot-enum-methods")))
	enumRows := collectDotRows(t, enumDeployment)
	for _, value := range []int{100, 200} {
		if err := engine.SendEvent(context.Background(), dotParityEnumEvent{IntPrimitive: value}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*enumRows) != 2 {
		t.Fatalf("enum rows = %d", len(*enumRows))
	}
	if (*enumRows)[0].Get("c0").Any() != true || (*enumRows)[0].Get("c1").Any() != false || (*enumRows)[0].Get("c2").Any() != 300 || (*enumRows)[0].Get("c3").Any() != false || (*enumRows)[0].Get("c4").Any() != false {
		t.Fatalf("enum first row = %#v", (*enumRows)[0].AsMap())
	}
	if (*enumRows)[1].Get("c0").Any() != false || (*enumRows)[1].Get("c1").Any() != true || (*enumRows)[1].Get("c2").Any() != 300 || (*enumRows)[1].Get("c3").Any() != true || (*enumRows)[1].Get("c4").Any() != true {
		t.Fatalf("enum second row = %#v", (*enumRows)[1].AsMap())
	}
	for _, row := range *enumRows {
		if !reflect.DeepEqual(row.Get("c5").Any(), []string{"2", "0", "0"}) || !reflect.DeepEqual(row.Get("c6").Any(), []string{"2", "0", "0"}) {
			t.Fatalf("enum list methods = %#v", row.AsMap())
		}
	}
}

func TestDotExpressionsMatchJavaRootedMapArrayAndParameterizedChains(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dotParityErasureEvent](env, "DotParityErasureEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dotParityChainTop](env, "DotParityChainTop"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	erasure := EventValue[dotParityErasureEvent]()
	innerTypes := Property[map[string]dotParityInner](erasure, "innerTypes")
	innerLiteral := MapAt[dotParityInner](innerTypes, Literal("key1"))
	innerByKey := MapAt[dotParityInner](innerTypes, Field[dotParityErasureEvent, string]("key"))
	innerArray := Property[[]dotParityInner](erasure, "innerTypesArray")
	innerArrayOne := ArrayAt[dotParityInner](innerArray, Literal[int64](1))
	innerArrayBySubkey := ArrayAt[dotParityInner](innerArray, Field[dotParityErasureEvent, int]("subkey"))
	erasureDeployment := deployDotPlan(t, env, engine, Select(From[dotParityErasureEvent](env, "DotParityErasureEvent"),
		Alias("c0", innerLiteral),
		Alias("c1", innerByKey),
		Alias("c2", ArrayAt[int](Property[[]int](innerLiteral, "ids"), Literal[int64](1))),
		Alias("c3", Method[int](innerByKey, "GetIDs", Field[dotParityErasureEvent, int]("subkey"))),
		Alias("c4", ArrayAt[int](Property[[]int](innerArrayOne, "ids"), Literal[int64](1))),
		Alias("c5", Method[int](innerArrayBySubkey, "GetIDs", Field[dotParityErasureEvent, int]("subkey"))),
		Alias("c6", Method[int](innerArrayBySubkey, "GetIDsWithSuffix", Field[dotParityErasureEvent, int]("subkey"), Literal("xyz"))),
		Alias("c7", Method[int](innerArrayBySubkey, "GetIDsWithEvent", EventValue[dotParityErasureEvent](), Literal("xyz"))),
	).Query(StatementName("dot-rooted-map-array")))
	erasureRows := collectDotRows(t, erasureDeployment)
	event := dotParityErasureEvent{
		Key:    "key1",
		Subkey: 2,
		InnerTypes: map[string]dotParityInner{
			"key1": {IDs: []int{20, 30, 40}},
		},
		InnerTypesArray: []dotParityInner{{IDs: []int{2, 3}}, {IDs: []int{4, 5}}, {IDs: []int{6, 7, 8}}},
	}
	if err := engine.SendEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if len(*erasureRows) != 1 {
		t.Fatalf("rooted map/array rows = %d", len(*erasureRows))
	}
	row := (*erasureRows)[0]
	if row.Get("c2").Any() != 30 || row.Get("c3").Any() != 40 || row.Get("c4").Any() != 5 || row.Get("c5").Any() != 8 || row.Get("c6").Any() != 8 || row.Get("c7").Any() != 999999 {
		t.Fatalf("rooted map/array row = %#v", row.AsMap())
	}
	if !reflect.DeepEqual(row.Get("c0").Any(), event.InnerTypes["key1"]) || !reflect.DeepEqual(row.Get("c1").Any(), event.InnerTypes["key1"]) {
		t.Fatalf("rooted map values = %#v", row.AsMap())
	}

	chain := EventValue[dotParityChainTop]()
	childOne := Method[dotParityChainOne](chain, "GetChildOne", Literal("abc"), Literal(10))
	childTwo := Method[dotParityChainTwo](childOne, "GetChildTwo", Literal("append"))
	chainDeployment := deployDotPlan(t, env, engine, Select(From[dotParityChainTop](env, "DotParityChainTop"),
		Alias("text", Method[string](childTwo, "GetText")),
	).Query(StatementName("dot-parameterized-chain")))
	chainRows := collectDotRows(t, chainDeployment)
	if err := engine.SendEvent(context.Background(), dotParityChainTop{Seed: "seed"}); err != nil {
		t.Fatal(err)
	}
	if len(*chainRows) != 1 || (*chainRows)[0].Get("text").Any() != "abcappend" {
		t.Fatalf("parameterized chain rows = %#v", *chainRows)
	}
}

func TestDotExpressionsMatchJavaArrayNestedWindowAndCollectionSemantics(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dotParityComplexEvent](env, "DotParityComplexEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dotParityNode](env, "DotParityNode"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dotParityNodeData](env, "DotParityNodeData"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dotParityNodeWithData](env, "DotParityNodeWithData"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[dotParityCollectionEvent](env, "DotParityCollectionEvent"); err != nil {
		t.Fatal(err)
	}
	windowSchema, err := StructSchema[dotParityNodeWithData]("DotParityNodeWithData")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "DotParityNodeWindow", windowSchema, NamedWindowRetention(LastEvent())); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	complex := EventValue[dotParityComplexEvent]()
	array := Property[[]int](complex, "arrayProperty")
	combined := Property[dotParityCombined](complex, "abc")
	arrayValues := Method[[]dotParityNestedOne](combined, "GetArray")
	arrayFirst := ArrayAt[dotParityNestedOne](arrayValues, Literal[int64](0))
	levelOne := Property[dotParityNestedOne](complex, "levelOne")
	levelTwo := Property[dotParityNestedTwo](levelOne, "levelTwo")
	levelThree := Property[dotParityNestedThree](levelTwo, "levelThree")
	complexDeployment := deployDotPlan(t, env, engine, Select(From[dotParityComplexEvent](env, "DotParityComplexEvent"),
		Alias("size", ArraySize(array)),
		Alias("get0", ArrayAt[int](array, Literal[int64](0))),
		Alias("get1", ArrayAt[int](array, Literal[int64](1))),
		Alias("get2", ArrayAt[int](array, Literal[int64](2))),
		Alias("get3", ArrayAt[int](array, Literal[int64](3))),
		Alias("chainGet0", Method[string](arrayFirst, "GetNestLevOneVal")),
		Alias("val0", Method[string](levelOne, "GetCustomLevelOne", Literal(10))),
		Alias("val1", Method[string](levelTwo, "GetCustomLevelTwo", Literal(20))),
		Alias("val2", Method[string](levelThree, "GetCustomLevelThree", Literal(30))),
	).Query(StatementName("dot-array-nested")))
	complexRows := collectDotRows(t, complexDeployment)
	complexEvent := dotParityComplexEvent{
		ArrayProperty: []int{1, 2},
		Abc:           dotParityCombined{Array: []dotParityNestedOne{{Value: "first"}}},
		LevelOne: dotParityNestedOne{
			Value: "level1",
			LevelTwo: dotParityNestedTwo{
				LevelThree: dotParityNestedThree{Value: "level3"},
			},
		},
	}
	if err := engine.SendEvent(context.Background(), complexEvent); err != nil {
		t.Fatal(err)
	}
	if len(*complexRows) != 1 {
		t.Fatalf("array/nested rows = %d", len(*complexRows))
	}
	row := (*complexRows)[0]
	if row.Get("size").Any() != int64(2) || row.Get("get0").Any() != 1 || row.Get("get1").Any() != 2 || row.Get("get2").Any() != nil || row.Get("get3").Any() != nil || row.Get("chainGet0").Any() != "first" || row.Get("val0").Any() != "level1:10" || row.Get("val1").Any() != "level3:20" || row.Get("val2").Any() != "level3:30" {
		t.Fatalf("array/nested row = %#v", row.AsMap())
	}

	named := FromNamedWindowAs[dotParityNodeWithData](env, "DotParityNodeWindow")
	namedNode := Property[dotParityNode](EventValue[dotParityNodeWithData](), "node")
	namedData := Property[dotParityNodeData](EventValue[dotParityNodeWithData](), "data")
	namedDeployment := deployDotPlan(t, env, engine, Select(named,
		Alias("nodeID", Property[string](namedNode, "id")),
		Alias("dataID", Property[string](namedData, "nodeId")),
		Alias("value", Property[string](namedData, "value")),
		Alias("computed", Method[string](namedNode, "Compute", namedData)),
	).Query(StatementName("dot-named-window")))
	namedRows := collectDotRows(t, namedDeployment)
	if err := engine.InsertNamedWindow(context.Background(), "DotParityNodeWindow", dotParityNodeWithData{Node: dotParityNode{ID: "1"}, Data: dotParityNodeData{NodeID: "1", Value: "xxx"}}); err != nil {
		t.Fatal(err)
	}
	if len(*namedRows) != 1 || (*namedRows)[0].Get("nodeID").Any() != "1" || (*namedRows)[0].Get("dataID").Any() != "1" || (*namedRows)[0].Get("value").Any() != "xxx" || (*namedRows)[0].Get("computed").Any() != "1:xxx" {
		t.Fatalf("named-window dot row = %#v", *namedRows)
	}

	collectionSource := From[dotParityCollectionEvent](env, "DotParityCollectionEvent").Window(KeepAll())
	values := Split(Field[dotParityCollectionEvent, string]("p01"), Literal(","))
	selected := EnumSelect[string, string](values, EnumElement[string]())
	toArray := ToArray[string](selected)
	collectionDeployment := deployDotPlan(t, env, engine, collectionSource.Aggregate(
		Alias("size", ArraySize(toArray)),
		Alias("third", ArrayAt[string](toArray, Literal[int64](2))),
		Alias("length", StringLength(MapAt[string](Property[map[string]string](EventValue[dotParityCollectionEvent](), "mapData"), Literal("key1")))),
		Alias("abs", Method[dotParityAmount](First[dotParityAmount](Field[dotParityCollectionEvent, dotParityAmount]("amount")), "Abs")),
		Alias("bean", Property[string](Method[dotParitySimpleBean](Literal(dotParityFactory{}), "ReturnBean", Field[dotParityCollectionEvent, string]("p00")), "theString")),
	).Query(StatementName("dot-collection-methods")))
	collectionRows := collectDotRows(t, collectionDeployment)
	if err := engine.SendEvent(context.Background(), dotParityCollectionEvent{P00: "A", P01: "A,B,C", Numbers: []int{1, 2}, MapData: map[string]string{"key1": "AB"}, Amount: dotParityAmount{Value: -1}}); err != nil {
		t.Fatal(err)
	}
	if len(*collectionRows) != 1 {
		t.Fatalf("collection dot rows = %d", len(*collectionRows))
	}
	collectionRow := (*collectionRows)[0]
	if collectionRow.Get("size").Any() != int64(3) || collectionRow.Get("third").Any() != "C" || collectionRow.Get("length").Any() != int64(2) || !reflect.DeepEqual(collectionRow.Get("abs").Any(), dotParityAmount{Value: 1}) || collectionRow.Get("bean").Any() != "A" {
		t.Fatalf("collection dot row = %#v", collectionRow.AsMap())
	}
}

func TestDotExpressionsRejectInvalidChainsAndPreservePlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dotParityChainTop](env, "DotParityInvalidChainTop"); err != nil {
		t.Fatal(err)
	}
	input := From[dotParityChainTop](env, "DotParityInvalidChainTop")
	valid := Select(input, Alias("value", Method[string](EventValue[dotParityChainTop](), "GetChildOne", Literal("a"), Literal(1)))).Query(StatementName("dot-plan"))
	first, err := env.Build(valid)
	if err != nil {
		t.Fatal(err)
	}
	same, err := env.Build(Select(input, Alias("value", Method[string](EventValue[dotParityChainTop](), "GetChildOne", Literal("a"), Literal(1)))).Query(StatementName("dot-plan")))
	if err != nil {
		t.Fatal(err)
	}
	different, err := env.Build(Select(input, Alias("value", Method[string](EventValue[dotParityChainTop](), "GetChildOne", Literal("b"), Literal(1)))).Query(StatementName("dot-plan")))
	if err != nil {
		t.Fatal(err)
	}
	if first.Hash() != same.Hash() || !reflect.DeepEqual(first.Canonical(), same.Canonical()) || first.Hash() == different.Hash() {
		t.Fatal("dot method arguments did not participate in Plan identity")
	}
	invalid := []Expr{
		Method[string](nil, "GetText"),
		Method[string](EventValue[dotParityChainTop](), " "),
		ArrayAt[int](Literal(1), Literal[int64](0)),
		ArrayAt[int](Property[[]int](EventValue[dotParityCollectionEvent](), "numbers"), Literal("bad")),
	}
	for index, expression := range invalid {
		t.Run(formatDotInt(index), func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("value", expression)).Query(StatementName("dot-invalid")))
			if err == nil {
				t.Fatal("invalid dot expression was accepted")
			}
		})
	}
}
