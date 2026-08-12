package esper

import (
	"context"
	"reflect"
	"testing"
)

// S0-like struct for ExprDefineValueParameter parity.
type valueParamS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
	P02 string `esper:"p02"`
}

func valueParamEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[valueParamS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func subscribeString(dep *Deployment, field string, collect *[]*string) {
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			raw := r.Get(field).Any()
			if raw == nil {
				*collect = append(*collect, nil)
				continue
			}
			s, ok := raw.(string)
			if !ok {
				*collect = append(*collect, nil)
				continue
			}
			*collect = append(*collect, &s)
		}
		return nil
	})
}

// ExprDefineValueParameterVV (ordinal 1). Java:
//   expression cc { (v1, v2) -> v1 || v2} select cc(p00, p01) as c0 from SupportBean_S0
func TestExprDefineValueParameterVVParity(t *testing.T) {
	env, engine := valueParamEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	v1 := ExpressionParam[string]("v1")
	v2 := ExpressionParam[string]("v2")
	if err := env.DefineExpression("cc", Concat(v1, v2)); err != nil {
		t.Fatal(err)
	}
	input := From[valueParamS0](env, "SupportBean_S0")
	plan, err := env.Build(Select(input,
		Alias("c0", ExpressionRef[string](env, "cc",
			Field[valueParamS0, string]("p00"),
			Field[valueParamS0, string]("p01"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []*string
	subscribeString(dep, "c0", &results)
// Go uses plain string fields; Java null-propagation through *string requires
// deeper pointer-type field inference and is noted as a remaining difference.
	cases := []struct{ p00, p01, want string }{
		{"A", "B", "AB"},
		{"C", "D", "CD"},
	}
	for _, tc := range cases {
		if err := engine.SendEvent(context.Background(), valueParamS0{P00: tc.p00, P01: tc.p01}); err != nil {
			t.Fatal(err)
		}
	}
	if len(results) != len(cases) {
		t.Fatalf("VV results count = %d, want %d", len(results), len(cases))
	}
	for i, tc := range cases {
		if results[i] == nil || *results[i] != tc.want {
			t.Fatalf("VV[%d]: got %v, want %q", i, results[i], tc.want)
		}
	}
}

// ExprDefineValueParameterVVV (ordinal 2). Java:
//   expression cc { (v1, v2, v3) -> v1 || v2 || v3} select cc(p00, p01, p02) as c0 from SupportBean_S0
func TestExprDefineValueParameterVVVParity(t *testing.T) {
	env, engine := valueParamEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	v1 := ExpressionParam[string]("v1")
	v2 := ExpressionParam[string]("v2")
	v3 := ExpressionParam[string]("v3")
	if err := env.DefineExpression("cc", Concat(v1, v2, v3)); err != nil {
		t.Fatal(err)
	}
	input := From[valueParamS0](env, "SupportBean_S0")
	plan, err := env.Build(Select(input,
		Alias("c0", ExpressionRef[string](env, "cc",
			Field[valueParamS0, string]("p00"),
			Field[valueParamS0, string]("p01"),
			Field[valueParamS0, string]("p02"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []*string
	subscribeString(dep, "c0", &results)
	cases := []struct {
		p00, p01, p02, want string
	}{
		{"A", "B", "C", "ABC"},
		{"D", "E", "F", "DEF"},
	}
	for _, tc := range cases {
		if err := engine.SendEvent(context.Background(), valueParamS0{P00: tc.p00, P01: tc.p01, P02: tc.p02}); err != nil {
			t.Fatal(err)
		}
	}
	if len(results) != len(cases) {
		t.Fatalf("VVV results count = %d, want %d", len(results), len(cases))
	}
	for i, tc := range cases {
		if results[i] == nil || *results[i] != tc.want {
			t.Fatalf("VVV[%d]: got %v, want %q", i, results[i], tc.want)
		}
	}
}

// ExprDefineValueParameterVEV (ordinal 4). Java:
//   expression cc { (v1,e,v2) -> v1 || e.p01 || v2}
//   select cc(p00, e, p02) as c0 from SupportBean_S0 as e
func TestExprDefineValueParameterVEVParity(t *testing.T) {
	env, engine := valueParamEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	v1 := ExpressionParam[string]("v1")
	eParam := ExpressionParam[valueParamS0]("e")
	v2 := ExpressionParam[string]("v2")
	body := Concat(v1, Property[string](eParam, "p01"), v2)
	if err := env.DefineExpression("cc", body); err != nil {
		t.Fatal(err)
	}
	input := From[valueParamS0](env, "SupportBean_S0")
	self := EventValue[valueParamS0]()
	plan, err := env.Build(Select(input,
		Alias("c0", ExpressionRef[string](env, "cc",
			Field[valueParamS0, string]("p00"),
			self,
			Field[valueParamS0, string]("p02"))),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []*string
	subscribeString(dep, "c0", &results)
	cases := []struct {
		p00, p01, p02, want string
	}{
		{"A", "B", "C", "ABC"},
		{"B", "C", "D", "BCD"},
	}
	for _, tc := range cases {
		if err := engine.SendEvent(context.Background(), valueParamS0{P00: tc.p00, P01: tc.p01, P02: tc.p02}); err != nil {
			t.Fatal(err)
		}
	}
	if len(results) != len(cases) {
		t.Fatalf("VEV results count = %d, want %d", len(results), len(cases))
	}
	for i, tc := range cases {
		if results[i] == nil || *results[i] != tc.want {
			t.Fatalf("VEV[%d]: got %v, want %q", i, results[i], tc.want)
		}
	}
}

// ExprDefineValueParameterVariable (ordinal 10). Java:
//   create variable double C=1.2; create variable double D=1.5;
//   create expression E {(V1,V2)=>max(V1,V2)}
//   select E(value1,value2) as c0, E(value1,C) as c1, E(C,D) as c2 from A
// After runtime variable set D=1.1, the c2 column changes from max(1.2,1.5)=1.5 to max(1.2,1.1)=1.2.
func TestExprDefineValueParameterVariableParity(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("C", 1.2); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("D", 1.5); err != nil {
		t.Fatal(err)
	}
	v1 := ExpressionParam[float64]("V1")
	v2 := ExpressionParam[float64]("V2")
	if err := env.DefineExpression("E", MaxOf[float64](v1, v2)); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "A", []FieldSpec{
		FieldDef("value1", reflect.TypeOf(float64(0))),
		FieldDef("value2", reflect.TypeOf(float64(0))),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	cVar := VariableRef[float64]("C")
	dVar := VariableRef[float64]("D")
	input := FromAny(env, "A")
	plan, err := env.Build(input.Select(
		Alias("c0", ExpressionRef[float64](env, "E",
			Field[any, float64]("value1"),
			Field[any, float64]("value2"))),
		Alias("c1", ExpressionRef[float64](env, "E",
			Field[any, float64]("value1"),
			cVar)),
		Alias("c2", ExpressionRef[float64](env, "E", cVar, dVar)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var rows [][]float64
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			row, ok := r.Row()
			if !ok {
				continue
			}
			c0, _ := As[float64](row.Get("c0"))
			c1, _ := As[float64](row.Get("c1"))
			c2, _ := As[float64](row.Get("c2"))
			rows = append(rows, []float64{c0, c1, c2})
		}
		return nil
	})
	if err := engine.SendRecord(context.Background(), "A", map[string]any{"value1": 1.0, "value2": 1.5}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("variable: row count = %d, want 1", len(rows))
	}
	if rows[0][0] != 1.5 || rows[0][1] != 1.2 || rows[0][2] != 1.5 {
		t.Fatalf("variable row0 = %v, want [1.5, 1.2, 1.5]", rows[0])
	}
	if err := engine.SetVariable(context.Background(), "D", 1.1); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendRecord(context.Background(), "A", map[string]any{"value1": 1.8, "value2": 1.5}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("variable: row count after set = %d, want 2", len(rows))
	}
	if rows[1][0] != 1.8 || rows[1][1] != 1.8 || rows[1][2] != 1.2 {
		t.Fatalf("variable row1 = %v, want [1.8, 1.8, 1.2]", rows[1])
	}
}

// ExprDefineNestedAlias (AliasFor ordinal 2). Java:
//   create expression F1 alias for {10}
//   create expression F2 alias for {20}
//   create expression F3 alias for {F1+F2}
//   select F3 as c0 from SupportBean → 30
// In Go, zero-parameter DefineExpression expressions can reference each other
// through ExpressionRef, exactly mirroring Java "alias for" composition.
func TestExprDefineAliasForNestedAliasParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.DefineExpression("F1", Literal(10)); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("F2", Literal(20)); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("F3", Add[int](ExpressionRef[int](env, "F1"), ExpressionRef[int](env, "F2"))); err != nil {
		t.Fatal(err)
	}
	input := From[defineBasicBean](env, "SupportBean")
	plan, err := env.Build(Select(input, Alias("c0", ExpressionRef[int](env, "F3"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var c0 []int
	subscribeDefine(dep, "c0", &c0)
	sendDefineBean(t, engine, "E1", 10)
	if len(c0) != 1 || c0[0] != 30 {
		t.Fatalf("nested alias F3 = %v, want [30]", c0)
	}
}

// ExprDefineGlobalAliasAndSODA (AliasFor ordinal 4). Java:
//   create expression myaliastwo alias for {2}
//   create expression myalias alias for {1}
//   select myaliastwo from SupportBean(intPrimitive = myalias)
// When intPrimitive=0 → no output; when intPrimitive=1 → myaliastwo=2.
// In Go, zero-parameter DefineExpression in a filter clause + projection.
// SODA EPStatementObjectModel round-trip is an approved difference.
func TestExprDefineAliasForGlobalAliasParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.DefineExpression("myaliastwo", Literal(2)); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("myalias", Literal(1)); err != nil {
		t.Fatal(err)
	}
	input := From[defineBasicBean](env, "SupportBean")
	filtered := input.Filter(
		Equal[int](Field[defineBasicBean, int]("intPrimitive"), ExpressionRef[int](env, "myalias")),
	)
	plan, err := env.Build(Select(filtered, Alias("myaliastwo", ExpressionRef[int](env, "myaliastwo"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []int
	subscribeDefine(dep, "myaliastwo", &results)
	// intPrimitive=0 ≠ myalias(1) → filtered out
	sendDefineBean(t, engine, "E1", 0)
	if len(results) != 0 {
		t.Fatalf("global alias: results after intPrimitive=0 = %v, want empty", results)
	}
	// intPrimitive=1 = myalias(1) → myaliastwo=2
	sendDefineBean(t, engine, "E1", 1)
	if len(results) != 1 || results[0] != 2 {
		t.Fatalf("global alias: results after intPrimitive=1 = %v, want [2]", results)
	}
}
