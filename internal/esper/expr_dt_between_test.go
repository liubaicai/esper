package esper

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type dateTimeBetweenTestEvent struct {
	Epoch int64     `esper:"epoch"`
	Date  time.Time `esper:"date"`
}

func TestDateTimeBetweenExpressionsMatchEsperEndpointAndRepresentationSemantics(t *testing.T) {
	origin := time.UnixMilli(1000).UTC()
	if got := DateTimeBetween(Literal(origin), Literal(int64(900)), Literal(int64(1100))).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("inclusive time.Time/epoch result = %v, want true", got)
	}
	if got := DateTimeBetween(Literal(int64(1000)), Literal(origin.Add(time.Second)), Literal(origin)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("reversed inclusive result = %v, want true", got)
	}
	if got := DateTimeBetweenRangeOf(Literal(int64(1000)), Literal(origin.Add(time.Second)), Literal(origin), false, true).eval(EvalContext{}); !got.Equal(Present(false)) {
		t.Fatalf("reversed endpoint result = %v, want false", got)
	}
	for _, endpoints := range []struct {
		name           string
		lowerInclusive bool
		upperInclusive bool
	}{
		{name: "both-inclusive", lowerInclusive: true, upperInclusive: true},
		{name: "lower-inclusive", lowerInclusive: true, upperInclusive: false},
		{name: "upper-inclusive", lowerInclusive: false, upperInclusive: true},
		{name: "both-exclusive", lowerInclusive: false, upperInclusive: false},
	} {
		if got := DateTimeBetweenRangeOf(Literal(int64(1500)), Literal(int64(2000)), Literal(int64(1000)), endpoints.lowerInclusive, endpoints.upperInclusive).eval(EvalContext{}); !got.Equal(Present(true)) {
			t.Errorf("reversed static %s result = %v, want true", endpoints.name, got)
		}
	}
	if got := DateTimeAfter(Literal(origin.Add(time.Millisecond)), Literal(origin)).eval(EvalContext{}); !got.Equal(Present(true)) {
		t.Fatalf("after result = %v, want true", got)
	}
}

func TestDateTimeBetweenExpressionsEvaluateRuntimeEndpointVariablesAndNulls(t *testing.T) {
	expression := DateTimeBetweenWithEndpoints(
		Literal(int64(1000)),
		Literal(int64(1000)),
		Literal(int64(1100)),
		VariableRef[bool]("include-low"),
		VariableRef[bool]("include-high"),
	)
	ctx := EvalContext{Variables: map[string]Value{
		"include-low":  Present(false),
		"include-high": Present(true),
	}}
	if got := expression.eval(ctx); !got.Equal(Present(false)) {
		t.Fatalf("exclusive lower result = %v, want false", got)
	}
	ctx.Variables["include-low"] = Present(true)
	if got := expression.eval(ctx); !got.Equal(Present(true)) {
		t.Fatalf("inclusive lower result = %v, want true", got)
	}
	ctx.Variables["include-low"] = Null()
	if got := expression.eval(ctx); !got.Equal(Null()) {
		t.Fatalf("null endpoint flag result = %v, want null", got)
	}
	if got := DateTimeBetween(NullLiteral[time.Time](), Literal(int64(0)), Literal(int64(1))).eval(EvalContext{}); !got.Equal(Null()) {
		t.Fatalf("null date-time value result = %v, want null", got)
	}
}

func TestDateTimeBetweenExpressionsBuildLiveProjectionAndPreservePlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dateTimeBetweenTestEvent](env, "DateTimeBetweenEvent"); err != nil {
		t.Fatal(err)
	}
	input := From[dateTimeBetweenTestEvent](env, "DateTimeBetweenEvent")
	expression := DateTimeBetween(
		Field[dateTimeBetweenTestEvent, time.Time]("date"),
		Literal(time.UnixMilli(900).UTC()),
		Field[dateTimeBetweenTestEvent, int64]("epoch"),
	)
	query := Select(input, Alias("matched", expression)).Query(StatementName("date-time-between"))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hash() != repeat.Hash() || !reflect.DeepEqual(plan.Canonical(), repeat.Canonical()) {
		t.Fatalf("equivalent date-time plans differ: %s != %s", plan.Hash(), repeat.Hash())
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = deployment.Undeploy(context.Background()) }()
	var values []Value
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, row := range batch.New {
			values = append(values, row.Get("matched"))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), dateTimeBetweenTestEvent{
		Epoch: 1100,
		Date:  time.UnixMilli(1000).UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || !values[0].Equal(Present(true)) {
		t.Fatalf("live date-time projection = %v, want one true row", values)
	}
}

func TestDateTimeBetweenExpressionsRejectInvalidBuilders(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[dateTimeBetweenTestEvent](env, "DateTimeBetweenInvalid"); err != nil {
		t.Fatal(err)
	}
	input := From[dateTimeBetweenTestEvent](env, "DateTimeBetweenInvalid")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{name: "bool-date-time", expr: DateTimeBetween(Literal(true), Literal(int64(0)), Literal(int64(1))), want: "epoch milliseconds or time.Time"},
		{name: "missing-endpoint", expr: DateTimeBetweenWithEndpoints(Literal(int64(0)), Literal(int64(0)), Literal(int64(1)), nil, Literal(true)), want: "lower endpoint is required"},
		{name: "non-bool-endpoint", expr: dateTimeBetweenExpression("date-time-between-endpoints", Literal(int64(0)), Literal(int64(0)), Literal(int64(1)), Literal(1), Literal(true)), want: "must be boolean"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := env.Build(Select(input, Alias("matched", testCase.expr)).Query(StatementName("invalid-" + testCase.name)))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("invalid date-time between error = %v, want %q", err, testCase.want)
			}
		})
	}
}
