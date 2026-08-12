package esper

import (
	"context"
	"testing"
)

// ExprDefineExpressionSimpleTwoModule (ordinal 2). The expression is declared
// at environment (public) scope and consumed by a separate deployment, mirroring
// Java's "@public create expression returnsOne {1}" followed by a different
// module that selects returnsOne. Env-level definitions are visible to every
// deployment, which is the Go-native form of @public expression visibility.
func TestExprDefineSimpleTwoModuleParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := env.DefineExpression("returnsOne", Literal(1)); err != nil {
		t.Fatal(err)
	}
	input := From[defineBasicBean](env, "SupportBean")
	plan, err := env.Build(Select(input, Alias("c0", ExpressionRef[int](env, "returnsOne"))).Query(StatementName("s0")))
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
	sendDefineBean(t, engine, "E1", 1)
	if len(c0) != 1 || c0[0] != 1 {
		t.Fatalf("cross-module returnsOne = %v, want [1]", c0)
	}
}

// ExprDefineWildcardAndPattern (ordinal 6), non-join form. Java:
//   expression abc { x => intPrimitive }
//   expression def { (x, y) => x.intPrimitive * y.intPrimitive }
//   select abc(*) as c0, def(*, *) as c1 from SupportBean
// The "*" wildcard passes the current event as every positional argument; Go
// uses EventValue for the same effect. intPrimitive=2 yields c0=2, c1=4.
func TestExprDefineWildcardNonJoinParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	x := ExpressionParam[defineBasicBean]("x")
	y := ExpressionParam[defineBasicBean]("y")
	if err := env.DefineExpression("abc", Property[int](x, "intPrimitive")); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("def", Multiply[int](Property[int](x, "intPrimitive"), Property[int](y, "intPrimitive"))); err != nil {
		t.Fatal(err)
	}
	input := From[defineBasicBean](env, "SupportBean")
	self := EventValue[defineBasicBean]()
	plan, err := env.Build(Select(input,
		Alias("c0", ExpressionRef[int](env, "abc", self)),
		Alias("c1", ExpressionRef[int](env, "def", self, self)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var c0, c1 []int
	subscribeDefine(dep, "c0", &c0)
	subscribeDefine(dep, "c1", &c1)
	sendDefineBean(t, engine, "E1", 2)
	if len(c0) != 1 || c0[0] != 2 || len(c1) != 1 || c1[0] != 4 {
		t.Fatalf("wildcard: c0=%v c1=%v, want [2] [4]", c0, c1)
	}
}

// ExprDefineWhereClauseExpression (ordinal 12), no-alias form. Java:
//   expression one {x=>x.boolPrimitive} select * from SupportBean as sb where one(sb)
// A parameterized declared expression is evaluated in the where/filter clause
// against the current event. boolPrimitive=false suppresses output; true emits.
// The "alias for" variant is an approved difference (Go has no textual alias).
func TestExprDefineWhereClauseExpressionParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	x := ExpressionParam[defineBasicBean]("x")
	if err := env.DefineExpression("one", Property[bool](x, "boolPrimitive")); err != nil {
		t.Fatal(err)
	}
	input := From[defineBasicBean](env, "SupportBean")
	self := EventValue[defineBasicBean]()
	plan, err := env.Build(input.Filter(ExpressionRef[bool](env, "one", self)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var hits int
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		hits += len(batch.New)
		return nil
	})
	sendDefineBean(t, engine, "E1", 1)
	if hits != 0 {
		t.Fatalf("where clause: hits after false = %d, want 0", hits)
	}
	if err := engine.SendEvent(context.Background(), defineBasicBean{TheString: "E2", IntPrimitive: 2, BoolPrimitive: true}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("where clause: hits after true = %d, want 1", hits)
	}
}

// ExprDefineEventTypeAndSODA (ordinal 23), declared-expression form. Java:
//   expression fZero {10}
//   expression fOne {x => x.intPrimitive}
//   expression fTwo {(x,y) => x.intPrimitive+y.intPrimitive}
//   expression fThree {(x,y) => x.intPrimitive+100}
//   select fZero(), fOne(t), fTwo(t,t), fThree(t,t) from SupportBean as t
// intPrimitive=11 yields 10, 11, 22, 111. The SODA EPStatementObjectModel
// round-trip and the "alias for" variant are approved differences (Go uses an
// immutable fluent Plan rather than a text/model parser). fThree declares a
// second parameter (y) that the body never reads; Go infers arity from body
// usage, so the redundant declared parameter is not represented, while the
// observable scalar result (111) is identical.
func TestExprDefineEventTypeAndSODAParity(t *testing.T) {
	env, engine := defineBasicEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	x := ExpressionParam[defineBasicBean]("x")
	y := ExpressionParam[defineBasicBean]("y")
	if err := env.DefineExpression("fZero", Literal(10)); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("fOne", Property[int](x, "intPrimitive")); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("fTwo", Add[int](Property[int](x, "intPrimitive"), Property[int](y, "intPrimitive"))); err != nil {
		t.Fatal(err)
	}
	if err := env.DefineExpression("fThree", Add[int](Property[int](x, "intPrimitive"), Literal(100))); err != nil {
		t.Fatal(err)
	}
	input := From[defineBasicBean](env, "SupportBean")
	self := EventValue[defineBasicBean]()
	plan, err := env.Build(Select(input,
		Alias("fZero", ExpressionRef[int](env, "fZero")),
		Alias("fOne", ExpressionRef[int](env, "fOne", self)),
		Alias("fTwo", ExpressionRef[int](env, "fTwo", self, self)),
		Alias("fThree", ExpressionRef[int](env, "fThree", self)),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var fZero, fOne, fTwo, fThree []int
	subscribeDefine(dep, "fZero", &fZero)
	subscribeDefine(dep, "fOne", &fOne)
	subscribeDefine(dep, "fTwo", &fTwo)
	subscribeDefine(dep, "fThree", &fThree)
	sendDefineBean(t, engine, "E1", 11)
	if len(fZero) != 1 || fZero[0] != 10 {
		t.Fatalf("fZero = %v, want [10]", fZero)
	}
	if len(fOne) != 1 || fOne[0] != 11 {
		t.Fatalf("fOne = %v, want [11]", fOne)
	}
	if len(fTwo) != 1 || fTwo[0] != 22 {
		t.Fatalf("fTwo = %v, want [22]", fTwo)
	}
	if len(fThree) != 1 || fThree[0] != 111 {
		t.Fatalf("fThree = %v, want [111]", fThree)
	}
}

