package esper

import (
	"context"
	"reflect"
	"testing"
)

type multiKeyJoinArrayLeft struct {
	ID    string `esper:"id"`
	Array []int  `esper:"array"`
	Value int    `esper:"value"`
}

type multiKeyJoinArrayRight struct {
	ID     string `esper:"id"`
	IntOne []int  `esper:"intOne"`
	Value  int    `esper:"value"`
}

type multiKeyJoinTwoProp struct {
	Name       string `esper:"name"`
	IntValue   int    `esper:"intValue"`
	BoxedValue int    `esper:"boxedValue"`
}

type multiKeyJoinRangeProbe struct {
	ID        int    `esper:"id"`
	Key       string `esper:"key"`
	IntBoxed  *int   `esper:"intBoxed"`
	TheString string `esper:"theString"`
}

type multiKeyJoinRangeBounds struct {
	ID            string `esper:"id"`
	Key           string `esper:"key"`
	RangeStart    *int   `esper:"rangeStart"`
	RangeEnd      *int   `esper:"rangeEnd"`
	RangeStartStr string `esper:"rangeStartStr"`
	RangeEndStr   string `esper:"rangeEndStr"`
}

func TestJoinArrayCompositePredicateMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[multiKeyJoinArrayLeft](env, "MultiKeyArrayLeft"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[multiKeyJoinArrayRight](env, "MultiKeyArrayRight"); err != nil {
		t.Fatal(err)
	}
	left := JoinSource(From[multiKeyJoinArrayLeft](env, "MultiKeyArrayLeft").Window(KeepAll()))
	right := JoinSource(From[multiKeyJoinArrayRight](env, "MultiKeyArrayRight").Window(KeepAll()))
	query := JoinMany(left, right).On(AllJoin(
		OnSourcesEqual(0, Field[multiKeyJoinArrayLeft, []int]("array"), 1, Field[multiKeyJoinArrayRight, []int]("intOne")),
		OnSourcesCompare(0, Field[multiKeyJoinArrayLeft, int]("value"), 1, Field[multiKeyJoinArrayRight, int]("value"), JoinGreater),
	)).Select(
		SelectFrom(0, "leftID", Field[multiKeyJoinArrayLeft, string]("id")),
		SelectFrom(1, "rightID", Field[multiKeyJoinArrayRight, string]("id")),
	).Query(StatementName("join-array-composite"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return nil
			}
			rows = append(rows, row.Get("leftID").String()+"/"+row.Get("rightID").String())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sendLeft := func(event multiKeyJoinArrayLeft) {
		t.Helper()
		if err := engine.Send(context.Background(), "MultiKeyArrayLeft", event); err != nil {
			t.Fatal(err)
		}
	}
	sendRight := func(event multiKeyJoinArrayRight) {
		t.Helper()
		if err := engine.Send(context.Background(), "MultiKeyArrayRight", event); err != nil {
			t.Fatal(err)
		}
	}
	assertRows := func(want ...string) {
		t.Helper()
		if !reflect.DeepEqual(rows, want) {
			t.Fatalf("array composite rows = %#v, want %#v", rows, want)
		}
		rows = nil
	}

	sendLeft(multiKeyJoinArrayLeft{ID: "I1", Array: []int{1, 2}, Value: 10})
	sendRight(multiKeyJoinArrayRight{ID: "M1", IntOne: []int{1, 2}, Value: 5})
	assertRows("I1/M1")
	sendLeft(multiKeyJoinArrayLeft{ID: "I2", Array: []int{1, 2}, Value: 20})
	assertRows("I2/M1")
	sendRight(multiKeyJoinArrayRight{ID: "M2", IntOne: []int{1, 2}, Value: 1})
	assertRows("I1/M2", "I2/M2")
	sendRight(multiKeyJoinArrayRight{ID: "M3", IntOne: []int{1}, Value: 1})
	assertRows()
	sendLeft(multiKeyJoinArrayLeft{ID: "I3", Array: []int{2}, Value: 30})
	assertRows()
	sendLeft(multiKeyJoinArrayLeft{ID: "I4", Array: []int{1}, Value: 40})
	assertRows("I4/M3")
	sendRight(multiKeyJoinArrayRight{ID: "M4", IntOne: []int{2}, Value: 2})
	assertRows("I3/M4")
}

func TestJoinTwoPropertyEqualityMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[multiKeyJoinTwoProp](env, "MultiKeyTwoProp"); err != nil {
		t.Fatal(err)
	}
	base := From[multiKeyJoinTwoProp](env, "MultiKeyTwoProp")
	left := JoinSource(base.Filter(Like(Field[multiKeyJoinTwoProp, string]("name"), Literal("A%"))).Window(LengthWindow(3)))
	right := JoinSource(base.Filter(Like(Field[multiKeyJoinTwoProp, string]("name"), Literal("B%"))).Window(LengthWindow(3)))
	query := JoinMany(left, right).On(AllJoin(
		OnSourcesEqual(0, Field[multiKeyJoinTwoProp, int]("intValue"), 1, Field[multiKeyJoinTwoProp, int]("intValue")),
		OnSourcesEqual(0, Field[multiKeyJoinTwoProp, int]("boxedValue"), 1, Field[multiKeyJoinTwoProp, int]("boxedValue")),
	)).Select(
		SelectFrom(0, "leftName", Field[multiKeyJoinTwoProp, string]("name")),
		SelectFrom(1, "rightName", Field[multiKeyJoinTwoProp, string]("name")),
	).Query(StatementName("join-two-property-equality"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row.Get("leftName").String()+"/"+row.Get("rightName").String())
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event multiKeyJoinTwoProp) {
		t.Helper()
		if err := engine.Send(context.Background(), "MultiKeyTwoProp", event); err != nil {
			t.Fatal(err)
		}
	}
	send(multiKeyJoinTwoProp{Name: "A0", IntValue: 1, BoxedValue: 100})
	send(multiKeyJoinTwoProp{Name: "B1", IntValue: 2, BoxedValue: 100})
	send(multiKeyJoinTwoProp{Name: "B2", IntValue: 1, BoxedValue: 200})
	send(multiKeyJoinTwoProp{Name: "B3", IntValue: 2, BoxedValue: 200})
	if len(rows) != 0 {
		t.Fatalf("two-property premature rows = %#v", rows)
	}
	send(multiKeyJoinTwoProp{Name: "AX", IntValue: 2, BoxedValue: 100})
	send(multiKeyJoinTwoProp{Name: "BX", IntValue: 1, BoxedValue: 100})
	if !reflect.DeepEqual(rows, []string{"AX/B1", "A0/BX"}) {
		t.Fatalf("two-property rows = %#v", rows)
	}
}

func TestJoinRangeNullDuplicateAndStringBoundsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[multiKeyJoinRangeProbe](env, "MultiKeyRangeProbe"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[multiKeyJoinRangeBounds](env, "MultiKeyRangeBounds"); err != nil {
		t.Fatal(err)
	}
	probe := JoinSource(From[multiKeyJoinRangeProbe](env, "MultiKeyRangeProbe").Window(KeepAll()))
	bounds := JoinSource(From[multiKeyJoinRangeBounds](env, "MultiKeyRangeBounds").Window(LastEvent()))
	query := JoinMany(probe, bounds).On(AllJoin(
		OnSourcesCompare(0, Field[multiKeyJoinRangeProbe, *int]("intBoxed"), 1, Field[multiKeyJoinRangeBounds, *int]("rangeStart"), JoinGreaterOrEqual),
		OnSourcesCompare(0, Field[multiKeyJoinRangeProbe, *int]("intBoxed"), 1, Field[multiKeyJoinRangeBounds, *int]("rangeEnd"), JoinLessOrEqual),
	)).Select(
		SelectFrom(0, "probeID", Field[multiKeyJoinRangeProbe, int]("id")),
	).Query(StatementName("join-range-null-duplicate"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var rows []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				value, _ := row.Get("probeID").Any().(int)
				rows = append(rows, value)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	intPtr := func(value int) *int { return &value }
	sendProbe := func(event multiKeyJoinRangeProbe) {
		t.Helper()
		if err := engine.Send(context.Background(), "MultiKeyRangeProbe", event); err != nil {
			t.Fatal(err)
		}
	}
	sendBounds := func(event multiKeyJoinRangeBounds) {
		t.Helper()
		if err := engine.Send(context.Background(), "MultiKeyRangeBounds", event); err != nil {
			t.Fatal(err)
		}
	}
	sendBounds(multiKeyJoinRangeBounds{ID: "R1", Key: "G"})
	sendProbe(multiKeyJoinRangeProbe{ID: 1, Key: "G", IntBoxed: intPtr(5)})
	sendBounds(multiKeyJoinRangeBounds{ID: "R2", Key: "G", RangeEnd: intPtr(10)})
	sendProbe(multiKeyJoinRangeProbe{ID: 2, Key: "G", IntBoxed: intPtr(5)})
	sendBounds(multiKeyJoinRangeBounds{ID: "R3", Key: "G", RangeStart: intPtr(10)})
	sendProbe(multiKeyJoinRangeProbe{ID: 3, Key: "G", IntBoxed: intPtr(5)})
	sendBounds(multiKeyJoinRangeBounds{ID: "R4", Key: "G", RangeStart: intPtr(10), RangeEnd: intPtr(0)})
	sendProbe(multiKeyJoinRangeProbe{ID: 4, Key: "G", IntBoxed: intPtr(5)})
	sendProbe(multiKeyJoinRangeProbe{ID: 100, Key: "G", IntBoxed: intPtr(5)})
	sendProbe(multiKeyJoinRangeProbe{ID: 101, Key: "G", IntBoxed: intPtr(5)})
	sendBounds(multiKeyJoinRangeBounds{ID: "R5", Key: "G", RangeStart: intPtr(0), RangeEnd: intPtr(10)})
	if !reflect.DeepEqual(rows, []int{1, 2, 3, 4, 100, 101}) {
		t.Fatalf("range rows = %#v", rows)
	}

	stringQuery := JoinMany(
		JoinSource(From[multiKeyJoinRangeBounds](env, "MultiKeyRangeBounds").Window(KeepAll())),
		JoinSource(From[multiKeyJoinRangeProbe](env, "MultiKeyRangeProbe").Window(LastEvent())),
	).On(AllJoin(
		OnSourcesEqual(0, Field[multiKeyJoinRangeBounds, string]("key"), 1, Field[multiKeyJoinRangeProbe, string]("key")),
		OnSourcesCompare(0, Field[multiKeyJoinRangeBounds, string]("rangeStartStr"), 1, Field[multiKeyJoinRangeProbe, string]("theString"), JoinLessOrEqual),
		OnSourcesCompare(0, Field[multiKeyJoinRangeBounds, string]("rangeEndStr"), 1, Field[multiKeyJoinRangeProbe, string]("theString"), JoinGreaterOrEqual),
	)).Select(
		SelectFrom(1, "probeID", Field[multiKeyJoinRangeProbe, int]("id")),
	).Query(StatementName("join-range-string"))
	stringPlan, err := env.Build(stringQuery)
	if err != nil {
		t.Fatal(err)
	}
	stringDeployment, err := engine.Deploy(context.Background(), stringPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer stringDeployment.Undeploy(context.Background())
	var stringRows []int
	if _, err := stringDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				value, _ := row.Get("probeID").Any().(int)
				stringRows = append(stringRows, value)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MultiKeyRangeProbe", multiKeyJoinRangeProbe{ID: 200, TheString: "P"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "MultiKeyRangeBounds", multiKeyJoinRangeBounds{ID: "R6", RangeStartStr: "O", RangeEndStr: "Q"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stringRows, []int{200}) {
		t.Fatalf("string range rows = %#v", stringRows)
	}
}
