package esper

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type clientCompileSubstBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type clientCompileSubstMarket struct {
	Symbol string `esper:"symbol"`
}

type clientCompileSubstS0 struct {
	ID int `esper:"id"`
}

type clientCompileSubstS1 struct {
	P10 string `esper:"p10"`
}

type clientCompileSubstKey interface{ clientCompileSubstKeyMarker() }

type clientCompileSubstInterfaceKey struct{}

func (*clientCompileSubstInterfaceKey) clientCompileSubstKeyMarker() {}

type clientCompileSubstConcreteKey struct{}

func (*clientCompileSubstConcreteKey) clientCompileSubstKeyMarker() {}

type clientCompileSubstEventOne struct {
	Key clientCompileSubstKey `esper:"key"`
}

type clientCompileSubstEventTwo struct {
	Key *clientCompileSubstConcreteKey `esper:"key"`
}

func newClientCompileSubstEnvironment(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	for _, registration := range []func() error{
		func() error { _, err := RegisterStruct[clientCompileSubstBean](env, "SupportBean"); return err },
		func() error {
			_, err := RegisterStruct[clientCompileSubstMarket](env, "SupportMarketDataBean")
			return err
		},
		func() error { _, err := RegisterStruct[clientCompileSubstS0](env, "SupportBean_S0"); return err },
		func() error { _, err := RegisterStruct[clientCompileSubstS1](env, "SupportBean_S1"); return err },
		func() error { _, err := RegisterStruct[clientCompileSubstEventOne](env, "MyEventOne"); return err },
		func() error { _, err := RegisterStruct[clientCompileSubstEventTwo](env, "MyEventTwo"); return err },
	} {
		if err := registration(); err != nil {
			t.Fatal(err)
		}
	}
	return env
}

func clientCompileSubstSubscribeRows(t *testing.T, deployment *Deployment, statementName string) *[]Row {
	t.Helper()
	statement, ok := deployment.Statement(statementName)
	if !ok {
		t.Fatalf("statement %q is missing", statementName)
	}
	rows := &[]Row{}
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				*rows = append(*rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

func clientCompileSubstSubscribeCount(t *testing.T, deployment *Deployment, statementName string) *int {
	t.Helper()
	statement, ok := deployment.Statement(statementName)
	if !ok {
		t.Fatalf("statement %q is missing", statementName)
	}
	count := new(int)
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		*count += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestClientCompileSubstitutionNamedPositionalMethodAndCastMatchEsper(t *testing.T) {
	env := newClientCompileSubstEnvironment(t)
	source := From[clientCompileSubstBean](env, "SupportBean")
	namedQuery := Select(source.Filter(And(
		Equal[string](Field[clientCompileSubstBean, string]("theString"), Parameter[string]("pstring")),
		And(
			Equal[int](Field[clientCompileSubstBean, int]("intPrimitive"), Parameter[int]("pint")),
			Equal[int64](Field[clientCompileSubstBean, int64]("longPrimitive"), Parameter[int64]("plong")),
		),
	)), Alias("c0", Parameter[int]("pint"))).Query(StatementName("named"))
	namedPlan, err := env.Build(namedQuery)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	namedDeployment, err := engine.DeployWithParameters(context.Background(), namedPlan, ParameterValues{
		"pstring": "E1", "pint": 10, "plong": int64(100),
	})
	if err != nil {
		t.Fatal(err)
	}
	namedRows := clientCompileSubstSubscribeRows(t, namedDeployment, "named")
	for range 2 {
		if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "E1", IntPrimitive: 10, LongPrimitive: 100}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*namedRows) != 2 || (*namedRows)[0].Get("c0").Any() != 10 || (*namedRows)[1].Get("c0").Any() != 10 {
		t.Fatalf("named rows = %#v", *namedRows)
	}

	methodQuery := source.Filter(Equal[string](
		Field[clientCompileSubstBean, string]("theString"),
		Property[string](Parameter[*clientCompileSubstBean]("psb"), "theString"),
	)).Query(StatementName("method"))
	methodPlan, err := env.Build(methodQuery)
	if err != nil {
		t.Fatal(err)
	}
	methodDeployment, err := engine.DeployWithParameters(context.Background(), methodPlan, ParameterValues{
		"psb": &clientCompileSubstBean{TheString: "E2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	methodCount := clientCompileSubstSubscribeCount(t, methodDeployment, "method")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	if *methodCount != 1 {
		t.Fatalf("method parameter matches = %d", *methodCount)
	}

	positionalObject := source.Filter(Equal[string](
		Field[clientCompileSubstBean, string]("theString"),
		Property[string](ParameterAt[*clientCompileSubstBean](1), "theString"),
	)).Query(StatementName("positional-object"))
	positionalObjectPlan, err := env.Build(positionalObject)
	if err != nil {
		t.Fatal(err)
	}
	positionalObjectDeployment, err := engine.DeployWithPositionalParameters(context.Background(), positionalObjectPlan,
		&clientCompileSubstBean{TheString: "E3"})
	if err != nil {
		t.Fatal(err)
	}
	positionalObjectCount := clientCompileSubstSubscribeCount(t, positionalObjectDeployment, "positional-object")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "E3"}); err != nil {
		t.Fatal(err)
	}
	if *positionalObjectCount != 1 {
		t.Fatalf("positional object matches = %d", *positionalObjectCount)
	}

	castQuery := source.Filter(Equal[string](
		Field[clientCompileSubstBean, string]("theString"),
		Cast[any, string](ParameterAt[any](1)),
	)).Query(StatementName("cast"))
	castPlan, err := env.Build(castQuery)
	if err != nil {
		t.Fatal(err)
	}
	castOne, err := engine.DeployWithPositionalParameters(context.Background(), castPlan, "e1")
	if err != nil {
		t.Fatal(err)
	}
	castTwo, err := engine.DeployWithPositionalParameters(context.Background(), castPlan, "e2")
	if err != nil {
		t.Fatal(err)
	}
	castOneCount := clientCompileSubstSubscribeCount(t, castOne, "cast")
	castTwoCount := clientCompileSubstSubscribeCount(t, castTwo, "cast")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "e2"}); err != nil {
		t.Fatal(err)
	}
	if *castOneCount != 0 || *castTwoCount != 1 {
		t.Fatalf("cast counts = %d/%d", *castOneCount, *castTwoCount)
	}
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "e1"}); err != nil {
		t.Fatal(err)
	}
	if *castOneCount != 1 || *castTwoCount != 1 {
		t.Fatalf("cast counts after e1 = %d/%d", *castOneCount, *castTwoCount)
	}

	boxedPlan, err := env.Build(Select(source,
		Alias("c0", Parameter[int]("p0")),
		Alias("c1", Parameter[int]("p1")),
	).Query(StatementName("boxed")))
	if err != nil {
		t.Fatal(err)
	}
	boxedDeployment, err := engine.DeployWithParameters(context.Background(), boxedPlan, ParameterValues{"p0": 10, "p1": 11})
	if err != nil {
		t.Fatal(err)
	}
	boxedRows := clientCompileSubstSubscribeRows(t, boxedDeployment, "boxed")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{}); err != nil {
		t.Fatal(err)
	}
	if len(*boxedRows) != 1 || (*boxedRows)[0].Get("c0").Any() != 10 || (*boxedRows)[0].Get("c1").Any() != 11 {
		t.Fatalf("primitive/boxed row = %#v", *boxedRows)
	}
}

func TestClientCompileSubstitutionTwoParameterFilterWhereAndNoParameterMatchEsper(t *testing.T) {
	env := newClientCompileSubstEnvironment(t)
	source := From[clientCompileSubstBean](env, "SupportBean")
	predicate := And(
		Equal[string](Field[clientCompileSubstBean, string]("theString"), ParameterAt[string](1)),
		Equal[int](Field[clientCompileSubstBean, int]("intPrimitive"), ParameterAt[int](2)),
	)
	filterPlan, err := env.Build(source.Filter(predicate).Query(StatementName("filter")))
	if err != nil {
		t.Fatal(err)
	}
	wherePlan, err := env.Build(source.Window(KeepAll()).Filter(predicate).Query(StatementName("where")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	filterA, err := engine.DeployWithPositionalParameters(context.Background(), filterPlan, "e1", 1)
	if err != nil {
		t.Fatal(err)
	}
	filterB, err := engine.DeployWithPositionalParameters(context.Background(), filterPlan, "e2", 2)
	if err != nil {
		t.Fatal(err)
	}
	where, err := engine.DeployWithPositionalParameters(context.Background(), wherePlan, "e1", 1)
	if err != nil {
		t.Fatal(err)
	}
	filterACount := clientCompileSubstSubscribeCount(t, filterA, "filter")
	filterBCount := clientCompileSubstSubscribeCount(t, filterB, "filter")
	whereCount := clientCompileSubstSubscribeCount(t, where, "where")
	for _, event := range []clientCompileSubstBean{
		{TheString: "e2", IntPrimitive: 2},
		{TheString: "e1", IntPrimitive: 1},
		{TheString: "e1", IntPrimitive: 2},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if *filterACount != 1 || *filterBCount != 1 || *whereCount != 1 {
		t.Fatalf("two-parameter counts = %d/%d/%d", *filterACount, *filterBCount, *whereCount)
	}

	noParameterPlan, err := env.Build(source.Filter(Equal[string](
		Field[clientCompileSubstBean, string]("theString"), Literal("literal"),
	)).Query(StatementName("no-parameter")))
	if err != nil {
		t.Fatal(err)
	}
	noParameterDeployment, err := engine.DeployPlans(context.Background(), []Plan{noParameterPlan},
		WithDeploymentParameterResolver(func(StatementParameterContext) (StatementParameterBindings, error) {
			return StatementParameterBindings{}, nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	noParameterCount := clientCompileSubstSubscribeCount(t, noParameterDeployment, "no-parameter")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "other"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "literal"}); err != nil {
		t.Fatal(err)
	}
	if *noParameterCount != 1 {
		t.Fatalf("no-parameter matches = %d", *noParameterCount)
	}
}

func TestClientCompileSubstitutionPatternAndInheritanceMatchEsper(t *testing.T) {
	env := newClientCompileSubstEnvironment(t)
	beanSource := From[clientCompileSubstBean](env, "SupportBean")
	patternPlan, err := env.Build(PatternFrom(beanSource, "a", Equal[string](
		Field[clientCompileSubstBean, string]("theString"), ParameterAt[string](1),
	)).Select(Alias("value", TagField[string]("a", "theString"))).Query(StatementName("pattern")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	patternOne, err := engine.DeployWithPositionalParameters(context.Background(), patternPlan, "e1")
	if err != nil {
		t.Fatal(err)
	}
	patternTwo, err := engine.DeployWithPositionalParameters(context.Background(), patternPlan, "e2")
	if err != nil {
		t.Fatal(err)
	}
	patternOneCount := clientCompileSubstSubscribeCount(t, patternOne, "pattern")
	patternTwoCount := clientCompileSubstSubscribeCount(t, patternTwo, "pattern")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "e2"}); err != nil {
		t.Fatal(err)
	}
	if *patternOneCount != 0 || *patternTwoCount != 1 {
		t.Fatalf("pattern counts after e2 = %d/%d", *patternOneCount, *patternTwoCount)
	}
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{TheString: "e1"}); err != nil {
		t.Fatal(err)
	}
	if *patternOneCount != 1 || *patternTwoCount != 1 {
		t.Fatalf("pattern counts after e1 = %d/%d", *patternOneCount, *patternTwoCount)
	}

	interfacePlan, err := env.Build(From[clientCompileSubstEventOne](env, "MyEventOne").Filter(Equal[clientCompileSubstKey](
		Field[clientCompileSubstEventOne, clientCompileSubstKey]("key"), ParameterAt[clientCompileSubstKey](1),
	)).Query(StatementName("interface")))
	if err != nil {
		t.Fatal(err)
	}
	interfaceKey := &clientCompileSubstInterfaceKey{}
	interfaceDeployment, err := engine.DeployWithPositionalParameters(context.Background(), interfacePlan, interfaceKey)
	if err != nil {
		t.Fatal(err)
	}
	interfaceCount := clientCompileSubstSubscribeCount(t, interfaceDeployment, "interface")
	if err := engine.SendEvent(context.Background(), clientCompileSubstEventOne{Key: interfaceKey}); err != nil {
		t.Fatal(err)
	}
	if *interfaceCount != 1 {
		t.Fatalf("interface-key matches = %d", *interfaceCount)
	}

	concretePlan, err := env.Build(From[clientCompileSubstEventTwo](env, "MyEventTwo").Filter(Equal[*clientCompileSubstConcreteKey](
		Field[clientCompileSubstEventTwo, *clientCompileSubstConcreteKey]("key"), ParameterAt[*clientCompileSubstConcreteKey](1),
	)).Query(StatementName("concrete")))
	if err != nil {
		t.Fatal(err)
	}
	concreteKey := &clientCompileSubstConcreteKey{}
	concreteDeployment, err := engine.DeployWithPositionalParameters(context.Background(), concretePlan, concreteKey)
	if err != nil {
		t.Fatal(err)
	}
	concreteCount := clientCompileSubstSubscribeCount(t, concreteDeployment, "concrete")
	if err := engine.SendEvent(context.Background(), clientCompileSubstEventTwo{Key: concreteKey}); err != nil {
		t.Fatal(err)
	}
	if *concreteCount != 1 {
		t.Fatalf("concrete-key matches = %d", *concreteCount)
	}
}

func TestClientCompileSubstitutionSubqueryDeploymentIsolationMatchEsper(t *testing.T) {
	env := newClientCompileSubstEnvironment(t)
	inner := From[clientCompileSubstMarket](env, "SupportMarketDataBean").Filter(Equal[string](
		Field[clientCompileSubstMarket, string]("symbol"), ParameterAt[string](1),
	)).Window(LastEvent()).AsRecord()
	query := Select(From[clientCompileSubstBean](env, "SupportBean"), Alias("mysymbol", SubqueryValue[string](
		inner, Field[any, string]("symbol"),
	))).Query(StatementName("subquery"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	first, err := engine.DeployWithPositionalParameters(context.Background(), plan, "S1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.DeployWithPositionalParameters(context.Background(), plan, "S2")
	if err != nil {
		t.Fatal(err)
	}
	firstRows := clientCompileSubstSubscribeRows(t, first, "subquery")
	secondRows := clientCompileSubstSubscribeRows(t, second, "subquery")
	sendOuter := func(wantFirst, wantSecond any) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), clientCompileSubstBean{}); err != nil {
			t.Fatal(err)
		}
		firstValue := (*firstRows)[len(*firstRows)-1].Get("mysymbol").Any()
		secondValue := (*secondRows)[len(*secondRows)-1].Get("mysymbol").Any()
		if firstValue != wantFirst || secondValue != wantSecond {
			t.Fatalf("subquery values = %#v/%#v, want %#v/%#v", firstValue, secondValue, wantFirst, wantSecond)
		}
	}
	sendOuter(nil, nil)
	if err := engine.SendEvent(context.Background(), clientCompileSubstMarket{Symbol: "XX"}); err != nil {
		t.Fatal(err)
	}
	sendOuter(nil, nil)
	if err := engine.SendEvent(context.Background(), clientCompileSubstMarket{Symbol: "S2"}); err != nil {
		t.Fatal(err)
	}
	sendOuter(nil, "S2")
	if err := engine.SendEvent(context.Background(), clientCompileSubstMarket{Symbol: "S1"}); err != nil {
		t.Fatal(err)
	}
	sendOuter("S1", "S2")
}

func TestClientCompileSubstitutionArrayAndGenericTypeMetadataMatchEsper(t *testing.T) {
	env := newClientCompileSubstEnvironment(t)
	plan, err := env.Build(Select(From[clientCompileSubstBean](env, "SupportBean"),
		Alias("boxed", Parameter[[]*int]("boxed")),
		Alias("primitive", Parameter[[]int]("primitive")),
		Alias("objects", Parameter[[]any]("objects")),
		Alias("strings2d", Parameter[[][]string]("strings2d")),
		Alias("objects2d", Parameter[[][]any]("objects2d")),
		Alias("list", Parameter[[]string]("list")),
		Alias("mapping", Parameter[map[string]int]("mapping")),
	).Query(StatementName("arrays")))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, ok := plan.ResultSchema()
	if !ok {
		t.Fatal("parameter result schema is missing")
	}
	expectedTypes := map[string]reflect.Type{
		"boxed": reflect.TypeOf([]*int{}), "primitive": reflect.TypeOf([]int{}),
		"objects": reflect.TypeOf([]any{}), "strings2d": reflect.TypeOf([][]string{}),
		"objects2d": reflect.TypeOf([][]any{}), "list": reflect.TypeOf([]string{}),
		"mapping": reflect.TypeOf(map[string]int{}),
	}
	for name, want := range expectedTypes {
		field, ok := resultSchema.Field(name)
		if !ok || field.Type != want {
			t.Fatalf("result type %s = %#v, want %s", name, field, want)
		}
	}
	one, two := 1, 2
	values := ParameterValues{
		"boxed": []*int{&one, &two}, "primitive": []int{3, 4},
		"objects": []any{"a", "b"}, "strings2d": [][]string{{"A"}},
		"objects2d": [][]any{{5, 6}}, "list": []string{"a"},
		"mapping": map[string]int{"k1": 10},
	}
	engine := NewEngine(env)
	deployment, err := engine.DeployWithParameters(context.Background(), plan, values)
	if err != nil {
		t.Fatal(err)
	}
	rows := clientCompileSubstSubscribeRows(t, deployment, "arrays")
	if err := engine.SendEvent(context.Background(), clientCompileSubstBean{}); err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 {
		t.Fatalf("array rows = %#v", *rows)
	}
	for name, want := range values {
		if got := (*rows)[0].Get(name).Any(); !reflect.DeepEqual(got, want) {
			t.Fatalf("array/generic %s = %#v, want %#v", name, got, want)
		}
	}
}

func TestClientCompileSubstitutionInvalidBindingsMatchEsperBoundary(t *testing.T) {
	env := newClientCompileSubstEnvironment(t)
	if _, err := env.Build(SelectOnce(env,
		Alias("named", Parameter[int]("p0")),
		Alias("positional", ParameterAt[int](1)),
	)); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("mixed parameter modes error = %v", err)
	}
	if _, err := env.Build(SelectOnce(env,
		Alias("first", Parameter[int]("same")),
		Alias("second", Parameter[int64]("same")),
	)); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("incompatible repeated type error = %v", err)
	}
	// Go has no EPL parser keyword namespace: a typed parameter called
	// "select" is an ordinary identifier and remains valid.
	if _, err := env.Build(SelectOnce(env, Alias("keyword", Parameter[int]("select")))); err != nil {
		t.Fatalf("Go-native parameter identifier was rejected: %v", err)
	}

	source := From[clientCompileSubstBean](env, "SupportBean")
	positionalPlan, err := env.Build(source.Filter(Equal[string](
		Field[clientCompileSubstBean, string]("theString"), ParameterAt[string](1),
	)).Query(StatementName("positional")))
	if err != nil {
		t.Fatal(err)
	}
	namedPlan, err := env.Build(source.Filter(Equal[string](
		Field[clientCompileSubstBean, string]("theString"), Parameter[string]("p0"),
	)).Query(StatementName("named")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for name, deploy := range map[string]func() error{
		"no callback": func() error { _, err := engine.Deploy(context.Background(), positionalPlan); return err },
		"missing positional": func() error {
			_, err := engine.DeployWithPositionalParameters(context.Background(), positionalPlan)
			return err
		},
		"wrong positional type": func() error {
			_, err := engine.DeployWithPositionalParameters(context.Background(), positionalPlan, 10)
			return err
		},
		"missing named": func() error {
			_, err := engine.DeployWithParameters(context.Background(), namedPlan, ParameterValues{})
			return err
		},
		"wrong named type": func() error {
			_, err := engine.DeployWithParameters(context.Background(), namedPlan, ParameterValues{"p0": 10})
			return err
		},
	} {
		if err := deploy(); err == nil {
			t.Fatalf("%s unexpectedly succeeded", name)
		}
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{positionalPlan},
		WithDeploymentParameterResolver(func(StatementParameterContext) (StatementParameterBindings, error) {
			return BindNamedParameters(ParameterValues{"p0": "x"}), nil
		})); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("named resolver on positional plan error = %v", err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{namedPlan},
		WithDeploymentParameterResolver(func(StatementParameterContext) (StatementParameterBindings, error) {
			return BindPositionalParameters("x"), nil
		})); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("positional resolver on named plan error = %v", err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{namedPlan},
		WithDeploymentParameterResolver(func(StatementParameterContext) (StatementParameterBindings, error) {
			return StatementParameterBindings{Named: ParameterValues{"p0": "x"}, Positional: []any{"x"}}, nil
		})); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("mixed resolver bindings error = %v", err)
	}
	noParameterPlan, err := env.Build(source.Query(StatementName("none")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{noParameterPlan},
		WithDeploymentParameterResolver(func(StatementParameterContext) (StatementParameterBindings, error) {
			return BindNamedParameters(ParameterValues{"extra": 1}), nil
		})); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("binding on no-parameter statement error = %v", err)
	}
	if _, err := engine.DeployPlans(context.Background(), []Plan{noParameterPlan}, WithDeploymentID(" ")); err == nil || !errors.Is(err, ErrorDeployment) {
		t.Fatalf("blank deployment id error = %v", err)
	}
}

func TestClientCompileSubstitutionResolverContextAndMultiStatementMatchEsper(t *testing.T) {
	t.Run("resolver-context", func(t *testing.T) {
		env := newClientCompileSubstEnvironment(t)
		plan, err := env.Build(Select(From[clientCompileSubstBean](env, "SupportBean"),
			Alias("c0", Parameter[int]("p0")),
		).Query(StatementName("s0"), StatementDescription("resolver metadata")))
		if err != nil {
			t.Fatal(err)
		}
		var captured StatementParameterContext
		_, err = NewEngine(env).DeployPlans(context.Background(), []Plan{plan},
			WithDeploymentID("abc"),
			WithDeploymentParameterResolver(func(parameterContext StatementParameterContext) (StatementParameterBindings, error) {
				captured = parameterContext
				return StatementParameterBindings{}, nil
			}),
		)
		if err == nil || !errors.Is(err, ErrorInvalidRule) {
			t.Fatalf("missing resolver value error = %v", err)
		}
		if captured.DeploymentID != "abc" || captured.StatementID != "abc:s0" || captured.StatementName != "s0" || captured.Index != 0 {
			t.Fatalf("resolver identity context = %#v", captured)
		}
		if len(captured.Plan.Canonical()) == 0 || !captured.Metadata.HasDescription || captured.Metadata.Description != "resolver metadata" {
			t.Fatalf("resolver plan/metadata context = %#v", captured)
		}
		if captured.Positional || len(captured.ParameterTypes) != 1 || captured.ParameterTypes[0] != reflect.TypeOf(int(0)) || captured.ParameterNames["p0"] != 1 {
			t.Fatalf("resolver parameter metadata = %#v", captured)
		}
	})

	t.Run("multi-statement", func(t *testing.T) {
		env := newClientCompileSubstEnvironment(t)
		planS0, err := env.Build(From[clientCompileSubstS0](env, "SupportBean_S0").Filter(Equal[int](
			Field[clientCompileSubstS0, int]("id"), Parameter[int]("subs_1"),
		)).Query(StatementName("s0")))
		if err != nil {
			t.Fatal(err)
		}
		planS1, err := env.Build(From[clientCompileSubstS1](env, "SupportBean_S1").Filter(Equal[string](
			Field[clientCompileSubstS1, string]("p10"), Parameter[string]("subs_2"),
		)).Query(StatementName("s1")))
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env)
		seen := make([]string, 0, 2)
		deployment, err := engine.DeployPlans(context.Background(), []Plan{planS0, planS1},
			WithDeploymentID("multi"),
			WithDeploymentParameterResolver(func(parameterContext StatementParameterContext) (StatementParameterBindings, error) {
				seen = append(seen, parameterContext.StatementName)
				if parameterContext.StatementName == "s1" {
					return BindNamedParameters(ParameterValues{"subs_2": "abc"}), nil
				}
				return BindNamedParameters(ParameterValues{"subs_1": 100}), nil
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
		if deployment.ID() != "multi" || !reflect.DeepEqual(seen, []string{"s0", "s1"}) {
			t.Fatalf("multi-statement resolver trace = %q/%#v", deployment.ID(), seen)
		}
		s0Count := clientCompileSubstSubscribeCount(t, deployment, "s0")
		s1Count := clientCompileSubstSubscribeCount(t, deployment, "s1")
		if err := engine.SendEvent(context.Background(), clientCompileSubstS1{P10: "abc"}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), clientCompileSubstS0{ID: 100}); err != nil {
			t.Fatal(err)
		}
		if *s0Count != 1 || *s1Count != 1 {
			t.Fatalf("multi-statement counts = %d/%d", *s0Count, *s1Count)
		}
		if _, err := engine.DeployPlans(context.Background(), []Plan{planS0},
			WithDeploymentID("multi"),
			WithDeploymentParameterResolver(func(StatementParameterContext) (StatementParameterBindings, error) {
				return BindNamedParameters(ParameterValues{"subs_1": 100}), nil
			})); err == nil || !errors.Is(err, ErrorDeployment) {
			t.Fatalf("duplicate deployment id error = %v", err)
		}
	})
}

func TestClientCompileSubstitutionGoNativeLiteralApprovedDifference(t *testing.T) {
	// Java SODA rejects an arbitrary Object constant because it cannot encode
	// it into compiler bytecode and recommends a substitution parameter. Go's
	// immutable Plan can canonically describe native struct literals directly,
	// so both a literal and a parameter are valid typed forms.
	type nativeConstant struct{ Value int }
	env := newClientCompileSubstEnvironment(t)
	literalPlan, err := env.Build(SelectOnce(env, Alias("value", Literal(nativeConstant{Value: 10}))))
	if err != nil {
		t.Fatal(err)
	}
	parameterPlan, err := env.Build(SelectOnce(env, Alias("value", Parameter[nativeConstant]("value"))))
	if err != nil {
		t.Fatal(err)
	}
	if literalPlan.Hash() == parameterPlan.Hash() || !strings.Contains(string(literalPlan.Canonical()), "nativeConstant") {
		t.Fatalf("native literal/parameter plan identity is not distinct")
	}
}
