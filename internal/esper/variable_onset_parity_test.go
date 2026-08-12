package esper

import (
	"context"
	"testing"
)

// onSetVarBean mirrors the Esper regression SupportBean with a plain (always
// present) intBoxed column, used by the EPLVariablesOnSet parity tests that do
// not exercise null boxed values.
type onSetVarBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     int    `esper:"intBoxed"`
}

type onSetVarS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

// onSetVarBeanBox mirrors SupportBean with a nullable (pointer) intBoxed
// column for tests that need to distinguish null from zero.
type onSetVarBeanBox struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	IntBoxed     *int   `esper:"intBoxed"`
}

type onSetVarA struct {
	ID string `esper:"id"`
}

func registerOnSetVarTypes(t *testing.T, env *Environment) {
	t.Helper()
	if _, err := RegisterStruct[onSetVarBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[onSetVarS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[onSetVarA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
}

func subscribeOnSetVarCapture(t *testing.T, stmt *Statement) *[]ResultBatch {
	t.Helper()
	batches := make([]ResultBatch, 0, 8)
	if _, err := stmt.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return &batches
}

func lastOnSetVarRow(t *testing.T, batches []ResultBatch) Row {
	t.Helper()
	if len(batches) == 0 {
		t.Fatal("on-set variable trigger produced no result batch")
	}
	last := batches[len(batches)-1]
	if len(last.New) != 1 {
		t.Fatalf("on-set variable trigger expected one new result, got %#v", last)
	}
	row, ok := last.New[0].Row()
	if !ok {
		t.Fatalf("on-set variable trigger result is not a row: %#v", last.New[0])
	}
	return row
}

func mustDeployOnSetVar(t *testing.T, engine *Engine, plan Plan) *Deployment {
	t.Helper()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	return deployment
}

func assertOnSetVarInt(t *testing.T, engine *Engine, name string, want int) {
	t.Helper()
	v, ok := engine.GetVariable(name)
	if !ok || v.Any() != want {
		t.Fatalf("GetVariable %s = %v (ok=%v), want %d", name, v, ok, want)
	}
}

// TestVariableOnSetSimpleParity mirrors EPLVariableOnSetSimple: a boolean
// variable is flipped to false by an on SupportBean_S0 trigger and a dependent
// select observing SupportBean sees the change.
func TestVariableOnSetSimpleParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("var_simple_set", true); err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(Select(From[onSetVarBean](env, "SupportBean"),
		Alias("c0", VariableRef[bool]("var_simple_set"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarS0](env, "SupportBean_S0")).
		SetVariable("var_simple_set", Literal(false)).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	selectDeployment := mustDeployOnSetVar(t, engine, selectPlan)
	triggerDeployment := mustDeployOnSetVar(t, engine, triggerPlan)
	selectBatches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])
	triggerBatches := subscribeOnSetVarCapture(t, triggerDeployment.Statements()[0])
	ctx := context.Background()
	if err := engine.SendEvent(ctx, onSetVarBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	row := lastOnSetVarRow(t, *selectBatches)
	if row.Get("c0").Any() != true {
		t.Fatalf("E1 c0 = %v, want true", row.Get("c0"))
	}
	if err := engine.SendEvent(ctx, onSetVarS0{ID: 0}); err != nil {
		t.Fatal(err)
	}
	trow := lastOnSetVarRow(t, *triggerBatches)
	if trow.Get("var_simple_set").Any() != false {
		t.Fatalf("trigger var_simple_set = %v, want false", trow.Get("var_simple_set"))
	}
	if v, _ := engine.GetVariable("var_simple_set"); v.Any() != false {
		t.Fatalf("GetVariable var_simple_set = %v, want false", v)
	}
	if err := engine.SendEvent(ctx, onSetVarBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	row2 := lastOnSetVarRow(t, *selectBatches)
	if row2.Get("c0").Any() != false {
		t.Fatalf("E2 c0 = %v, want false", row2.Get("c0"))
	}
}

// TestVariableOnSetAssignmentOrderNoDupParity mirrors
// EPLVariableOnSetAssignmentOrderNoDup: later assignments observe earlier
// assignments within the same on-set batch.
func TestVariableOnSetAssignmentOrderNoDupParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("var1OND", 12); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2OND", 2); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var3OND", nil); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarBean](env, "SupportBean")).SetVariables(
		SetVariableExpr("var1OND", Field[onSetVarBean, int]("intPrimitive")),
		SetVariableExpr("var2OND", Add[int](VariableRef[int]("var1OND"), Literal(1))),
		SetVariableExpr("var3OND", Add[int](VariableRef[int]("var1OND"), VariableRef[int]("var2OND"))),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment := mustDeployOnSetVar(t, engine, triggerPlan)
	batches := subscribeOnSetVarCapture(t, deployment.Statements()[0])
	assertOnSetVarInt(t, engine, "var1OND", 12)
	assertOnSetVarInt(t, engine, "var2OND", 2)
	if v, _ := engine.GetVariable("var3OND"); v.State() != ValueNull {
		t.Fatalf("initial var3OND = %v, want null", v)
	}
	ctx := context.Background()
	send := func(intPrimitive, want1, want2, want3 int) {
		t.Helper()
		*batches = (*batches)[:0]
		if err := engine.SendEvent(ctx, onSetVarBean{TheString: "S1", IntPrimitive: intPrimitive}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *batches)
		if row.Get("var1OND").Any() != want1 || row.Get("var2OND").Any() != want2 || row.Get("var3OND").Any() != want3 {
			t.Fatalf("intPrimitive=%d got {%v,%v,%v}, want {%d,%d,%d}",
				intPrimitive, row.Get("var1OND"), row.Get("var2OND"), row.Get("var3OND"), want1, want2, want3)
		}
		assertOnSetVarInt(t, engine, "var1OND", want1)
		assertOnSetVarInt(t, engine, "var2OND", want2)
		assertOnSetVarInt(t, engine, "var3OND", want3)
	}
	send(3, 3, 4, 7)
	send(-1, -1, 0, -1)
	send(90, 90, 91, 181)
}

// TestVariableOnSetAssignmentOrderDupParity mirrors
// EPLVariableOnSetAssignmentOrderDup: a variable assigned twice in the same
// batch keeps the last value, and self/incremental references resolve in order.
func TestVariableOnSetAssignmentOrderDupParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	for name, init := range map[string]int{"var1OD": 0, "var2OD": 1, "var3OD": 2} {
		if err := env.RegisterVariable(name, init); err != nil {
			t.Fatal(err)
		}
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarBean](env, "SupportBean")).SetVariables(
		SetVariableExpr("var1OD", Field[onSetVarBean, int]("intPrimitive")),
		SetVariableExpr("var2OD", VariableRef[int]("var2OD")),
		SetVariableExpr("var1OD", Field[onSetVarBean, int]("intBoxed")),
		SetVariableExpr("var3OD", Add[int](VariableRef[int]("var3OD"), Literal(1))),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment := mustDeployOnSetVar(t, engine, triggerPlan)
	batches := subscribeOnSetVarCapture(t, deployment.Statements()[0])
	assertOnSetVarInt(t, engine, "var1OD", 0)
	assertOnSetVarInt(t, engine, "var2OD", 1)
	assertOnSetVarInt(t, engine, "var3OD", 2)
	ctx := context.Background()
	send := func(boxed, want1, want2, want3 int) {
		t.Helper()
		*batches = (*batches)[:0]
		if err := engine.SendEvent(ctx, onSetVarBean{TheString: "S1", IntPrimitive: -1, IntBoxed: boxed}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *batches)
		if row.Get("var1OD").Any() != want1 || row.Get("var2OD").Any() != want2 || row.Get("var3OD").Any() != want3 {
			t.Fatalf("boxed=%d got {%v,%v,%v}, want {%d,%d,%d}",
				boxed, row.Get("var1OD"), row.Get("var2OD"), row.Get("var3OD"), want1, want2, want3)
		}
		assertOnSetVarInt(t, engine, "var1OD", want1)
		assertOnSetVarInt(t, engine, "var2OD", want2)
		assertOnSetVarInt(t, engine, "var3OD", want3)
	}
	send(10, 10, 1, 3)
	send(20, 20, 1, 4)
	send(30, 30, 1, 5)
	send(40, 40, 1, 6)
}

// TestVariableOnSetWithFilterParity mirrors EPLVariableOnSetWithFilter: the
// on-set trigger only fires for events matching the trigger-stream filter.
func TestVariableOnSetWithFilterParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("papi_1", "begin"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("papi_2", true); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("papi_3", "value"); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarBean](env, "SupportBean").Filter(
		StartsWith(Field[onSetVarBean, string]("theString"), Literal("S")),
	)).SetVariables(
		SetVariableExpr("papi_1", Literal("end")),
		SetVariableExpr("papi_2", Literal(false)),
		SetVariableExpr("papi_3", NullLiteral[string]()),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment := mustDeployOnSetVar(t, engine, triggerPlan)
	batches := subscribeOnSetVarCapture(t, deployment.Statements()[0])
	ctx := context.Background()
	if err := engine.SendEvent(ctx, onSetVarBean{TheString: "E1", IntPrimitive: 3}); err != nil {
		t.Fatal(err)
	}
	if len(*batches) != 0 {
		t.Fatalf("non-matching event fired the trigger: %#v", *batches)
	}
	if v, _ := engine.GetVariable("papi_1"); v.Any() != "begin" {
		t.Fatalf("papi_1 changed by filtered-out event: %v", v)
	}
	send := func(theString string) {
		t.Helper()
		*batches = (*batches)[:0]
		if err := engine.SendEvent(ctx, onSetVarBean{TheString: theString, IntPrimitive: 3}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *batches)
		if row.Get("papi_1").Any() != "end" || row.Get("papi_2").Any() != false || row.Get("papi_3").State() != ValueNull {
			t.Fatalf("%s got {%v,%v,%v}, want {end,false,null}",
				theString, row.Get("papi_1"), row.Get("papi_2"), row.Get("papi_3"))
		}
		if v, _ := engine.GetVariable("papi_3"); v.State() != ValueNull {
			t.Fatalf("papi_3 = %v, want null", v)
		}
	}
	send("S1")
	send("S2")
}

// TestVariableOnSetCoercionParity mirrors EPLVariableOnSetCoercion: int values
// are coerced to the declared variable types (float32, float64, int64).
func TestVariableOnSetCoercionParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("var1COE", float32(0)); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2COE", float64(0)); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var3COE", int64(0)); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarBean](env, "SupportBean")).SetVariables(
		SetVariableExpr("var1COE", Field[onSetVarBean, int]("intPrimitive")),
		SetVariableExpr("var2COE", Field[onSetVarBean, int]("intPrimitive")),
		SetVariableExpr("var3COE", Field[onSetVarBean, int]("intBoxed")),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment := mustDeployOnSetVar(t, engine, triggerPlan)
	batches := subscribeOnSetVarCapture(t, deployment.Statements()[0])
	ctx := context.Background()
	send := func(primitive, boxed int) {
		t.Helper()
		*batches = (*batches)[:0]
		if err := engine.SendEvent(ctx, onSetVarBean{TheString: "S1", IntPrimitive: primitive, IntBoxed: boxed}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *batches)
		if v1, ok := row.Get("var1COE").Any().(float32); !ok || v1 != float32(primitive) {
			t.Fatalf("var1COE = %#v, want float32(%d)", row.Get("var1COE"), primitive)
		}
		if v2, ok := row.Get("var2COE").Any().(float64); !ok || v2 != float64(primitive) {
			t.Fatalf("var2COE = %#v, want float64(%d)", row.Get("var2COE"), primitive)
		}
		if v3, ok := row.Get("var3COE").Any().(int64); !ok || v3 != int64(boxed) {
			t.Fatalf("var3COE = %#v, want int64(%d)", row.Get("var3COE"), boxed)
		}
	}
	send(1, 2)
	send(10, 20)
}

// TestVariableOnSetRuntimeOrderMultipleParity mirrors
// EPLVariableOnSetRuntimeOrderMultiple: an OR-filtered trigger updates nullable
// variables and a later select observes the latest values. The nullable boxed
// column uses *int so null propagation matches the Java Integer semantics.
func TestVariableOnSetRuntimeOrderMultipleParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[onSetVarBeanBox](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var1ROM", nil); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var2ROM", nil); err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarBeanBox](env, "SupportBean").Filter(
		Or(StartsWith(Field[onSetVarBeanBox, string]("theString"), Literal("S")),
			StartsWith(Field[onSetVarBeanBox, string]("theString"), Literal("B"))),
	)).SetVariables(
		SetVariableExpr("var1ROM", Field[onSetVarBeanBox, int]("intPrimitive")),
		SetVariableExpr("var2ROM", Field[onSetVarBeanBox, int]("intBoxed")),
	).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.SetVariable(context.Background(), "var2ROM", 1); err != nil {
		t.Fatal(err)
	}
	deployment := mustDeployOnSetVar(t, engine, triggerPlan)
	batches := subscribeOnSetVarCapture(t, deployment.Statements()[0])
	if v, _ := engine.GetVariable("var1ROM"); v.State() != ValueNull {
		t.Fatalf("initial var1ROM = %v, want null", v)
	}
	if v, _ := engine.GetVariable("var2ROM"); v.Any() != 1 {
		t.Fatalf("initial var2ROM = %v, want 1", v)
	}
	ctx := context.Background()
	sendTrigger := func(theString string, primitive int, boxed *int, want1, want2 any) {
		t.Helper()
		*batches = (*batches)[:0]
		if err := engine.SendEvent(ctx, onSetVarBeanBox{TheString: theString, IntPrimitive: primitive, IntBoxed: boxed}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *batches)
		if !onSetVarEquals(row.Get("var1ROM"), want1) || !onSetVarEquals(row.Get("var2ROM"), want2) {
			t.Fatalf("%s got {%v,%v}, want {%v,%v}", theString, row.Get("var1ROM"), row.Get("var2ROM"), want1, want2)
		}
	}
	sendTrigger("S1", 3, nil, 3, nil)
	sendTrigger("S1", -1, onSetVarIntPtr(-2), -1, -2)

	selectPlan, err := env.Build(Select(From[onSetVarBeanBox](env, "SupportBean").Filter(
		Or(StartsWith(Field[onSetVarBeanBox, string]("theString"), Literal("E")),
			StartsWith(Field[onSetVarBeanBox, string]("theString"), Literal("B"))),
	),
		Alias("v1", VariableRef[int]("var1ROM")),
		Alias("v2", VariableRef[int]("var2ROM")),
		Alias("s", Field[onSetVarBeanBox, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	selectDeployment := mustDeployOnSetVar(t, engine, selectPlan)
	selectBatches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])
	sendSelect := func(theString string, want1, want2 int) {
		t.Helper()
		if err := engine.SendEvent(ctx, onSetVarBeanBox{TheString: theString, IntPrimitive: 1}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *selectBatches)
		if row.Get("v1").Any() != want1 || row.Get("v2").Any() != want2 || row.Get("s").Any() != theString {
			t.Fatalf("%s got {%v,%v,%v}, want {%d,%d,%s}",
				theString, row.Get("v1"), row.Get("v2"), row.Get("s"), want1, want2, theString)
		}
	}
	sendSelect("E1", -1, -2)
	sendTrigger("S1", 11, onSetVarIntPtr(12), 11, 12)
	sendSelect("E2", 11, 12)
}

func onSetVarEquals(v Value, want any) bool {
	if want == nil {
		return v.State() == ValueNull
	}
	return v.Any() == want
}

func onSetVarIntPtr(v int) *int { return &v }

// TestVariableOnSetWDeployParity mirrors EPLVariableOnSetWDeploy: a select
// referencing a variable is deployed first, then an on-set trigger updates it.
func TestVariableOnSetWDeployParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("var1RTC", 10); err != nil {
		t.Fatal(err)
	}
	selectPlan, err := env.Build(Select(From[onSetVarBean](env, "SupportBean").Filter(
		StartsWith(Field[onSetVarBean, string]("theString"), Literal("E")),
	),
		Alias("v", VariableRef[int]("var1RTC")),
		Alias("s", Field[onSetVarBean, string]("theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	triggerPlan, err := env.Build(OnEvent(From[onSetVarBean](env, "SupportBean").Filter(
		StartsWith(Field[onSetVarBean, string]("theString"), Literal("S")),
	)).SetVariable("var1RTC", Field[onSetVarBean, int]("intPrimitive")).Query(StatementName("set")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	selectDeployment := mustDeployOnSetVar(t, engine, selectPlan)
	triggerDeployment := mustDeployOnSetVar(t, engine, triggerPlan)
	selectBatches := subscribeOnSetVarCapture(t, selectDeployment.Statements()[0])
	_ = subscribeOnSetVarCapture(t, triggerDeployment.Statements()[0])
	ctx := context.Background()
	sendSelect := func(theString string, want int) {
		t.Helper()
		if err := engine.SendEvent(ctx, onSetVarBean{TheString: theString}); err != nil {
			t.Fatal(err)
		}
		row := lastOnSetVarRow(t, *selectBatches)
		if row.Get("v").Any() != want || row.Get("s").Any() != theString {
			t.Fatalf("%s got {%v,%v}, want {%d,%s}", theString, row.Get("v"), row.Get("s"), want, theString)
		}
	}
	sendTrigger := func(theString string, primitive int) {
		t.Helper()
		if err := engine.SendEvent(ctx, onSetVarBean{TheString: theString, IntPrimitive: primitive}); err != nil {
			t.Fatal(err)
		}
	}
	sendSelect("E1", 10)
	sendSelect("E2", 10)
	sendTrigger("S1", 3)
	sendSelect("E3", 3)
	sendTrigger("S2", -1)
	sendSelect("E4", -1)
}

// TestVariableOnSetInvalidParity mirrors the build-time diagnostics from
// EPLVariableOnSetInvalid: unknown variables and type mismatches are rejected.
func TestVariableOnSetInvalidParity(t *testing.T) {
	env := NewEnvironment()
	registerOnSetVarTypes(t, env)
	if err := env.RegisterVariable("var1IS", ""); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("var3IS", 1); err != nil {
		t.Fatal(err)
	}
	stream := From[onSetVarBean](env, "SupportBean")
	if _, err := env.Build(OnEvent(stream).SetVariable("dummy", Literal(100)).Query()); err == nil {
		t.Fatal("unknown variable 'dummy' should fail at build time")
	}
	if _, err := env.Build(OnEvent(stream).SetVariable("var1IS", Literal(1)).Query()); err == nil {
		t.Fatal("assigning int to String variable should fail at build time")
	}
	if _, err := env.Build(OnEvent(stream).SetVariable("var3IS", Literal("abc")).Query()); err == nil {
		t.Fatal("assigning string to int variable should fail at build time")
	}
}
