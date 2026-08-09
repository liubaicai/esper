package esper

import (
	"context"
	"reflect"
	"testing"
)

type clientRuntimeUnmatchedBean struct {
	TheString string `esper:"theString"`
}

func newClientRuntimeUnmatchedEnvironment(t *testing.T) (*Environment, Stream[clientRuntimeUnmatchedBean], Plan) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientRuntimeUnmatchedBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	source := From[clientRuntimeUnmatchedBean](env, "SupportBean")
	plan, err := env.Build(source.Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	return env, source, plan
}

func TestClientRuntimeUnmatchedSendEventParity(t *testing.T) {
	env, _, plan := newClientRuntimeUnmatchedEnvironment(t)
	engine := NewEngine(env)
	var received []Event
	listener := func(_ context.Context, event Event) error {
		received = append(received, event)
		return nil
	}
	engine.SetUnmatchedListener(listener)

	first := &clientRuntimeUnmatchedBean{TheString: "E1"}
	if err := engine.SendEvent(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].Underlying() != first {
		t.Fatalf("unmatched initial event = %#v, want identity %p", received, first)
	}
	received = nil

	engine.SetUnmatchedListener(nil)
	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "silent"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("disabled unmatched listener received %#v", received)
	}

	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	engine.SetUnmatchedListener(listener)
	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "matched"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("active statement reported matched event as unmatched: %#v", received)
	}

	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopped := &clientRuntimeUnmatchedBean{TheString: "E2"}
	if err := engine.SendEvent(context.Background(), stopped); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].Underlying() != stopped {
		t.Fatalf("unmatched after undeploy = %#v, want identity %p", received, stopped)
	}
	received = nil

	deployment, err = engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "matched-again"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("redeployed statement reported matched event as unmatched: %#v", received)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	destroyed := &clientRuntimeUnmatchedBean{TheString: "E3"}
	if err := engine.SendEvent(context.Background(), destroyed); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].Underlying() != destroyed {
		t.Fatalf("unmatched after final undeploy = %#v, want identity %p", received, destroyed)
	}

	t.Run("input-filter-vs-having", func(t *testing.T) {
		filterEnv, filterSource, _ := newClientRuntimeUnmatchedEnvironment(t)
		filterPlan, err := filterEnv.Build(filterSource.Filter(Equal[string](
			Field[clientRuntimeUnmatchedBean, string]("theString"), Literal("MATCH"),
		)).Query(StatementName("filtered")))
		if err != nil {
			t.Fatal(err)
		}
		filterEngine := NewEngine(filterEnv)
		var unmatched []string
		filterEngine.SetUnmatchedListener(func(_ context.Context, event Event) error {
			unmatched = append(unmatched, event.Get("theString").Any().(string))
			return nil
		})
		if _, err := filterEngine.Deploy(context.Background(), filterPlan); err != nil {
			t.Fatal(err)
		}
		if err := filterEngine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "MISS"}); err != nil {
			t.Fatal(err)
		}
		if err := filterEngine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "MATCH"}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(unmatched, []string{"MISS"}) {
			t.Fatalf("input-filter unmatched values = %#v, want [MISS]", unmatched)
		}

		havingEnv, havingSource, _ := newClientRuntimeUnmatchedEnvironment(t)
		havingPlan, err := havingEnv.Build(havingSource.Aggregate(
			Alias("count", CountAll()),
		).Having(Literal(false)).Query(StatementName("having")))
		if err != nil {
			t.Fatal(err)
		}
		havingEngine := NewEngine(havingEnv)
		var havingUnmatched []Event
		havingEngine.SetUnmatchedListener(func(_ context.Context, event Event) error {
			havingUnmatched = append(havingUnmatched, event)
			return nil
		})
		if _, err := havingEngine.Deploy(context.Background(), havingPlan); err != nil {
			t.Fatal(err)
		}
		if err := havingEngine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "NO-OUTPUT"}); err != nil {
			t.Fatal(err)
		}
		if len(havingUnmatched) != 0 {
			t.Fatalf("post-having suppression reported unmatched: %#v", havingUnmatched)
		}
	})
}

func TestClientRuntimeUnmatchedCreateStatementParity(t *testing.T) {
	env, _, plan := newClientRuntimeUnmatchedEnvironment(t)
	engine := NewEngine(env)
	var received []string
	engine.SetUnmatchedListener(func(_ context.Context, event Event) error {
		received = append(received, event.Get("theString").Any().(string))
		_, err := engine.Deploy(context.Background(), plan)
		return err
	})

	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(received, []string{"E1"}) {
		t.Fatalf("unmatched create-statement callback = %#v", received)
	}
	received = nil
	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Fatalf("statement deployed by unmatched callback did not match next event: %#v", received)
	}
}

func TestClientRuntimeUnmatchedInsertIntoParity(t *testing.T) {
	env, source, _ := newClientRuntimeUnmatchedEnvironment(t)
	if _, err := RegisterMap(env, "MyEvent", []FieldSpec{FieldDef("theString", reflect.TypeOf(""))}); err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(Select(source,
		Alias("theString", Field[clientRuntimeUnmatchedBean, string]("theString")),
	).InsertInto("MyEvent", StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	var received []Event
	engine.SetUnmatchedListener(func(_ context.Context, event Event) error {
		received = append(received, event)
		return nil
	})

	deployment, err := engine.Deploy(context.Background(), routePlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].TypeName() != "MyEvent" || received[0].Get("theString").Any() != "E1" {
		t.Fatalf("unmatched insert-into route = %#v", received)
	}
	received = nil

	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	original := &clientRuntimeUnmatchedBean{TheString: "E2"}
	if err := engine.SendEvent(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].Underlying() != original {
		t.Fatalf("unmatched source after insert undeploy = %#v, want identity %p", received, original)
	}
	received = nil

	// The Java execution labels this step "start insert-into"; Go makes the
	// lifecycle explicit by redeploying the immutable plan.
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), &clientRuntimeUnmatchedBean{TheString: "E3"}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || received[0].TypeName() != "MyEvent" || received[0].Get("theString").Any() != "E3" {
		t.Fatalf("unmatched route after redeploy = %#v", received)
	}
}
