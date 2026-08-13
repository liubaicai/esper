package esper

import (
	"context"
	"reflect"
	"testing"
)

type contextAdminListenStart struct {
	ID int `esper:"id"`
}

type contextAdminListenEnd struct {
	ID int `esper:"id"`
}

type contextAdminListenQuery struct {
	ID int `esper:"id"`
}

// TestContextAdminListenInitTermParity covers ContextAdminListenInitTerm.
// Lifecycle events arrive on dedicated event types while the context statement
// consumes SupportBean, matching Esper's start/end controller boundary.
func TestContextAdminListenInitTermParity(t *testing.T) {
	env := NewEnvironment()
	for name, register := range map[string]func() error{
		"SupportBean_S0": func() error {
			_, err := RegisterStruct[contextAdminListenStart](env, "SupportBean_S0")
			return err
		},
		"SupportBean_S1": func() error {
			_, err := RegisterStruct[contextAdminListenEnd](env, "SupportBean_S1")
			return err
		},
		"SupportBean": func() error {
			_, err := RegisterStruct[contextAdminListenQuery](env, "SupportBean")
			return err
		},
	} {
		if err := register(); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	start := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0"))
	end := Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S1"))
	if _, err := CreateInitiatedTerminatedContext(env, "MyContext", Literal("global"), start, end); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	listener := &contextStateTestListener{}
	if err := engine.AddContextStateListener(listener); err != nil {
		t.Fatal(err)
	}
	if err := engine.AddContextPartitionStateListener("MyContext", listener); err != nil {
		t.Fatal(err)
	}
	if got, want := listener.sequence, []string{"created"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("context replay sequence = %#v, want %#v", got, want)
	}

	plan, err := env.Build(From[contextAdminListenQuery](env, "SupportBean").Query(
		StatementName("s0"),
		WithContext("MyContext"),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if got, want := listener.sequence, []string{"created", "statement-added", "activated"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("activation sequence = %#v, want %#v", got, want)
	}
	if len(listener.added) != 1 || listener.added[0].StatementName != "s0" || listener.added[0].StatementDeploymentID == "" {
		t.Fatalf("statement-added event = %#v", listener.added)
	}

	if err := engine.SendEvent(context.Background(), contextAdminListenStart{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(listener.allocated) != 1 || !listener.allocated[0].Allocated {
		t.Fatalf("allocation events = %#v", listener.allocated)
	}
	allocated := listener.allocated[0]
	if allocated.ContextName != "MyContext" || allocated.Key == "" || allocated.PartitionID != allocated.Descriptor.ID {
		t.Fatalf("allocation identity = %#v", allocated)
	}

	if err := engine.SendEvent(context.Background(), contextAdminListenEnd{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if len(listener.deallocated) != 1 || listener.deallocated[0].Allocated {
		t.Fatalf("deallocation events = %#v", listener.deallocated)
	}
	if listener.deallocated[0].PartitionID != allocated.PartitionID || listener.deallocated[0].Key != allocated.Key {
		t.Fatalf("allocation/deallocation identity mismatch: allocated=%#v deallocated=%#v", allocated, listener.deallocated[0])
	}
	if got, want := listener.sequence, []string{"created", "statement-added", "activated", "partition-allocated", "partition-deallocated"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partition lifecycle sequence = %#v, want %#v", got, want)
	}
	if got := statement.ContextPartitionCount(); got != 0 {
		t.Fatalf("active partitions after end = %d, want 0", got)
	}

	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := listener.sequence, []string{"created", "statement-added", "activated", "partition-allocated", "partition-deallocated", "statement-removed", "deactivated"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("statement teardown sequence = %#v, want %#v", got, want)
	}
	if len(listener.removed) != 1 || listener.removed[0].StatementName != "s0" || listener.removed[0].StatementDeploymentID == "" {
		t.Fatalf("statement-removed event = %#v", listener.removed)
	}

	if err := engine.DestroyContext(context.Background(), "MyContext"); err != nil {
		t.Fatal(err)
	}
	if got, want := listener.sequence, []string{"created", "statement-added", "activated", "partition-allocated", "partition-deallocated", "statement-removed", "deactivated", "destroyed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("context teardown sequence = %#v, want %#v", got, want)
	}
	if len(listener.destroyed) != 1 || listener.destroyed[0].ContextName != "MyContext" {
		t.Fatalf("context-destroyed event = %#v", listener.destroyed)
	}
}
