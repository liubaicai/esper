package esper

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

// TestFAFSubstitutionParametersMatchesEsper exercises the Java
// InfraNWTableFAFSubstitutionParams matrix through the typed Go fluent API.
// The same plan shape is run against a Named Window and a Table so the
// prepared binding contract is independent of the target representation.
func TestFAFSubstitutionParametersMatchesEsper(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env, engine, target := setupSubstitutionParameterInfra(t, namedWindow)
			ctx := context.Background()

			oneParameter, err := env.Build(target.Filter(
				Equal[int](Field[any, int]("intPrimitive"), ParameterAt[int](1)),
			).Query(StatementName("faf-positional-one")))
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := engine.PrepareFireAndForget(oneParameter)
			if err != nil {
				t.Fatal(err)
			}
			for index := 0; index < 10; index++ {
				result, executeErr := prepared.ExecuteWithPositionalParameters(ctx, index)
				if executeErr != nil || len(result.Results()) != 1 {
					t.Fatalf("positional parameter %d result = %#v, err=%v", index, result.Results(), executeErr)
				}
				if got := result.Results()[0].Get("theString").Any(); got != "E"+strconv.Itoa(index) {
					t.Fatalf("positional parameter %d event = %#v", index, got)
				}
			}
			if result, executeErr := prepared.ExecuteWithPositionalParameters(ctx, -1); executeErr != nil || len(result.Results()) != 0 {
				t.Fatalf("positional parameter miss = %#v, err=%v", result.Results(), executeErr)
			}

			twoParameters, err := env.Build(target.Filter(And(
				Equal[int](Field[any, int]("intPrimitive"), ParameterAt[int](1)),
				Equal[int64](Field[any, int64]("longPrimitive"), ParameterAt[int64](2)),
			)).Query(StatementName("faf-positional-two")))
			if err != nil {
				t.Fatal(err)
			}
			for index := 0; index < 10; index++ {
				result, executeErr := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, twoParameters, index, int64(index*1000))
				if executeErr != nil || len(result.Results()) != 1 {
					t.Fatalf("two positional parameters %d result = %#v, err=%v", index, result.Results(), executeErr)
				}
			}
			if result, executeErr := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, twoParameters, 3, int64(4000)); executeErr != nil || len(result.Results()) != 0 {
				t.Fatalf("two positional parameter mismatch = %#v, err=%v", result.Results(), executeErr)
			}

			inPlan, err := env.Build(target.Filter(
				InSlice[string](Field[any, string]("theString"), ParameterAt[[]string](1)),
			).Query(StatementName("faf-positional-in")))
			if err != nil {
				t.Fatal(err)
			}
			inResult, err := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, inPlan, []string{"E3", "E6", "E8"})
			if err != nil || len(inResult.Results()) != 3 {
				t.Fatalf("positional IN result = %#v, err=%v", inResult.Results(), err)
			}

			namedPlan, err := env.Build(target.Filter(
				Equal[int](Field[any, int]("intPrimitive"), Parameter[int]("p0")),
			).Query(StatementName("faf-named-one")))
			if err != nil {
				t.Fatal(err)
			}
			namedResult, err := engine.ExecuteFireAndForgetWithParameters(ctx, namedPlan, ParameterValues{"p0": 5})
			if err != nil || len(namedResult.Results()) != 1 || namedResult.Results()[0].Get("theString").Any() != "E5" {
				t.Fatalf("named parameter result = %#v, err=%v", namedResult.Results(), err)
			}

			namedTwice, err := env.Build(target.Filter(Or(
				Equal[int](Field[any, int]("intPrimitive"), Parameter[int]("p0")),
				Equal[int](Field[any, int]("intBoxed"), Parameter[int]("p0")),
			)).Query(StatementName("faf-named-twice")))
			if err != nil {
				t.Fatal(err)
			}
			twiceResult, err := engine.ExecuteFireAndForgetWithParameters(ctx, namedTwice, ParameterValues{"p0": 12})
			if err != nil || len(twiceResult.Results()) != 1 || twiceResult.Results()[0].Get("theString").Any() != "E2" {
				t.Fatalf("named repeated parameter result = %#v, err=%v", twiceResult.Results(), err)
			}
		})
	}
}

func TestFAFSubstitutionParametersRejectMixedAndInvalidBindings(t *testing.T) {
	env, engine, target := setupSubstitutionParameterInfra(t, true)
	ctx := context.Background()

	if _, err := env.Build(SelectOnce(env,
		Alias("positional", ParameterAt[int](1)),
		Alias("named", Parameter[int]("p0")),
	)); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("mixed named/positional parameters error = %v", err)
	}
	if _, err := env.Build(SelectOnce(env,
		Alias("first", Parameter[int]("same")),
		Alias("second", Parameter[string]("same")),
	)); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("incompatible repeated named parameter error = %v", err)
	}
	if _, err := env.Build(SelectOnce(env, Alias("jump", ParameterAt[int](2)))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("positional parameter gap error = %v", err)
	}
	if _, err := env.Build(SelectOnce(env, Alias("zero", ParameterAt[int](0)))); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("invalid positional parameter index error = %v", err)
	}

	positionalPlan, err := env.Build(target.Filter(
		Equal[int](Field[any, int]("intPrimitive"), ParameterAt[int](1)),
	).Query(StatementName("faf-positional-invalid")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForget(ctx, positionalPlan); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unbound positional direct execution error = %v", err)
	}
	prepared, err := engine.PrepareFireAndForget(positionalPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepared.Execute(ctx); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("unbound positional prepared execution error = %v", err)
	}
	if _, err := prepared.ExecuteWithPositionalParameters(ctx); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("insufficient positional values error = %v", err)
	}
	if _, err := prepared.ExecuteWithParameters(ctx, ParameterValues{"1": 5}); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("named binding on positional query error = %v", err)
	}

	twoParameters, err := env.Build(target.Filter(And(
		Equal[int](Field[any, int]("intPrimitive"), ParameterAt[int](1)),
		Equal[int64](Field[any, int64]("longPrimitive"), ParameterAt[int64](2)),
	)).Query(StatementName("faf-positional-insufficient")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, twoParameters, 1); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("missing second positional value error = %v", err)
	}
	if _, err := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, twoParameters, 1, int64(1000), "extra"); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("extra positional value error = %v", err)
	}

	typedNamed, err := env.Build(target.Filter(
		Equal[string](Field[any, string]("theString"), Parameter[string]("p0")),
	).Query(StatementName("faf-named-typed-invalid")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForgetWithParameters(ctx, typedNamed, ParameterValues{"p0": 10}); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("typed named parameter error = %v", err)
	}
	typedPositional, err := env.Build(target.Filter(
		Equal[string](Field[any, string]("theString"), ParameterAt[string](1)),
	).Query(StatementName("faf-positional-typed-invalid")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, typedPositional, 10); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("typed positional parameter error = %v", err)
	}

	noParameter, err := env.Build(target.Query(StatementName("faf-no-parameter")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForgetWithPositionalParameters(ctx, noParameter, 1); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("positional values on unparameterized query error = %v", err)
	}
}

func setupSubstitutionParameterInfra(t *testing.T, namedWindow bool) (*Environment, *Engine, RecordStream) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[substitutionParamEvent](env, "SubstitutionParamEvent"); err != nil {
		t.Fatal(err)
	}
	if namedWindow {
		schema, ok := env.Schema("SubstitutionParamEvent")
		if !ok {
			t.Fatal("substitution parameter schema is missing")
		}
		if _, err := CreateNamedWindow(env, "SubstitutionParamInfra", schema, NamedWindowRetention(KeepAll())); err != nil {
			t.Fatal(err)
		}
	} else if _, err := CreateTable(env, "SubstitutionParamInfra", []TableColumn{
		PrimaryKeyColumn[string]("theString"),
		PrimaryKeyColumn[int]("intPrimitive"),
		TableColumnOf[int64]("longPrimitive"),
		TableColumnOf[int]("intBoxed"),
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for index := 0; index < 10; index++ {
		event := substitutionParamEvent{
			TheString:     "E" + strconv.Itoa(index),
			IntPrimitive:  index,
			LongPrimitive: int64(index * 1000),
			IntBoxed:      10 + index,
		}
		if namedWindow {
			if err := engine.InsertNamedWindow(context.Background(), "SubstitutionParamInfra", event); err != nil {
				t.Fatal(err)
			}
		} else {
			table, ok := engine.Table("SubstitutionParamInfra")
			if !ok {
				t.Fatal("substitution parameter table is missing")
			}
			if _, err := table.Insert(context.Background(), map[string]any{
				"theString": event.TheString, "intPrimitive": event.IntPrimitive,
				"longPrimitive": event.LongPrimitive, "intBoxed": event.IntBoxed,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if namedWindow {
		return env, engine, FromNamedWindow(env, "SubstitutionParamInfra")
	}
	return env, engine, FromTable(env, "SubstitutionParamInfra")
}

type substitutionParamEvent struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
	IntBoxed      int    `esper:"intBoxed"`
}
