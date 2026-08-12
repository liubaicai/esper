package esper

import (
	"context"
	"strings"
	"testing"
)

type variantSingleColumnSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type variantSingleColumnOtherMember struct {
	TheString string `esper:"theString"`
}

// TestVariantSingleColumnConversionMatchesEsper mirrors
// EventVariantSingleColumnConversion: a concrete SupportBean first enters a
// predefined Variant, a static Go function returns another concrete member,
// and the single event value is inserted into a Variant-typed Named Window.
func TestVariantSingleColumnConversionMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	supportBeanSchema, err := RegisterStruct[variantSingleColumnSupportBean](env, "SupportBean")
	if err != nil {
		t.Fatal(err)
	}
	otherSchema, err := RegisterStruct[variantSingleColumnOtherMember](env, "SupportBeanVariantStream")
	if err != nil {
		t.Fatal(err)
	}
	variantSchema, err := RegisterVariant(env, "MyVariantTwoTypedSBVariant", otherSchema, supportBeanSchema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "MainEventWindow", variantSchema, NamedWindowRetention(LengthWindow(10000))); err != nil {
		t.Fatal(err)
	}

	var converted []Event
	preProcessEvent := Func1[Event, variantSingleColumnSupportBean](
		"preProcessEvent",
		func(event Event) variantSingleColumnSupportBean {
			converted = append(converted, event)
			return variantSingleColumnSupportBean{TheString: "E2", IntPrimitive: 0}
		},
		EventValue[Event](),
	)

	routePlan, err := env.Build(
		From[variantSingleColumnSupportBean](env, "SupportBean").InsertInto(
			"MyVariantTwoTypedSBVariant",
			StatementName("variant-single-column-source"),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	conversionPlan, err := env.Build(
		OnRecord(FromAny(env, "MyVariantTwoTypedSBVariant")).InsertEventIntoNamedWindow(
			"MainEventWindow",
			preProcessEvent,
		).Query(StatementName("variant-single-column-conversion")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if canonical := string(conversionPlan.Canonical()); !strings.Contains(canonical, "insert-event(preProcessEvent(event()))") {
		t.Fatalf("single-column event conversion missing from plan canonical: %s", canonical)
	}
	consumerPlan, err := env.Build(
		FromNamedWindow(env, "MainEventWindow").Filter(
			Equal[string](Field[any, string]("theString"), Literal("E")),
		).Query(StatementName("s0")),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), conversionPlan); err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	var consumerBatches []ResultBatch
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		consumerBatches = append(consumerBatches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := engine.SendEvent(context.Background(), variantSingleColumnSupportBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(converted) != 1 || converted[0].TypeName() != "SupportBean" || converted[0].StreamType() != "MyVariantTwoTypedSBVariant" || converted[0].Get("theString").Any() != "E1" {
		t.Fatalf("static conversion input = %#v", converted)
	}
	if len(consumerBatches) != 0 {
		t.Fatalf("filtered MainEventWindow listener invoked = %#v", consumerBatches)
	}

	window, ok := engine.NamedWindow("MainEventWindow")
	if !ok {
		t.Fatal("MainEventWindow is missing")
	}
	snapshot, err := window.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("MainEventWindow snapshot = %#v", snapshot)
	}
	stored := snapshot[0]
	if stored.TypeName() != "MainEventWindow" || stored.Schema().Name() != "SupportBean" || stored.Get("theString").Any() != "E2" || stored.Get("intPrimitive").Any() != 0 {
		t.Fatalf("converted member in MainEventWindow = %#v", stored)
	}
}
