package esper

import (
	"context"
	"fmt"
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

type contextAdminListenHashBean struct {
	TheString string `esper:"theString"`
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

// TestContextAdminListenHashParity covers the preallocated hash bucket
// lifecycle used by ContextAdminListenHash.
func TestContextAdminListenHashParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextAdminListenHashBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	definition, err := CreatePreallocatedHashContext(
		env,
		"MyContext",
		Field[contextAdminListenHashBean, string]("theString"),
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !definition.Preallocate() {
		t.Fatal("hash context is not marked for preallocation")
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

	plan, err := env.Build(From[contextAdminListenHashBean](env, "SupportBean").Query(
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
	if got, want := listener.sequence, []string{"created", "statement-added", "activated", "partition-allocated", "partition-allocated"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hash activation sequence = %#v, want %#v", got, want)
	}
	if got, want := statement.ContextPartitionKeys(), []string{"hash:0", "hash:1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("preallocated statement partitions = %#v, want %#v", got, want)
	}
	if len(listener.allocated) != 2 {
		t.Fatalf("hash allocation events = %#v", listener.allocated)
	}
	for bucket, event := range listener.allocated {
		wantKey := fmt.Sprintf("hash:%d", bucket)
		if event.ContextName != "MyContext" || event.Key != wantKey || event.PartitionID != bucket || event.Descriptor.ID != bucket || event.Descriptor.Key != wantKey {
			t.Fatalf("hash allocation identity[%d] = %#v", bucket, event)
		}
		hash, ok := event.Descriptor.Property("hash")
		if !ok || hash.State() != ValuePresent || hash.Any() != int64(bucket) {
			t.Fatalf("hash allocation descriptor[%d] hash = %#v, present=%v", bucket, hash, ok)
		}
		name, ok := event.Descriptor.Property("name")
		if !ok || name.Any() != "MyContext" {
			t.Fatalf("hash allocation descriptor[%d] name = %#v, present=%v", bucket, name, ok)
		}
		id, ok := event.Descriptor.Property("id")
		if !ok || id.Any() != bucket {
			t.Fatalf("hash allocation descriptor[%d] id = %#v, present=%v", bucket, id, ok)
		}
	}
	descriptors, err := engine.ContextPartitionDescriptors("MyContext", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != 2 || descriptors[0].ID != 0 || descriptors[1].ID != 1 {
		t.Fatalf("engine hash descriptors = %#v", descriptors)
	}

	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := listener.sequence, []string{"created", "statement-added", "activated", "partition-allocated", "partition-allocated", "statement-removed", "partition-deallocated", "partition-deallocated", "deactivated"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hash teardown sequence = %#v, want %#v", got, want)
	}
	if len(listener.deallocated) != 2 {
		t.Fatalf("hash deallocation events = %#v", listener.deallocated)
	}
	for bucket, event := range listener.deallocated {
		wantKey := fmt.Sprintf("hash:%d", bucket)
		if event.Key != wantKey || event.PartitionID != bucket || event.Descriptor.ID != bucket {
			t.Fatalf("hash deallocation identity[%d] = %#v", bucket, event)
		}
	}

	if err := engine.DestroyContext(context.Background(), "MyContext"); err != nil {
		t.Fatal(err)
	}
	if got, want := listener.sequence, []string{"created", "statement-added", "activated", "partition-allocated", "partition-allocated", "statement-removed", "partition-deallocated", "partition-deallocated", "deactivated", "destroyed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hash context teardown sequence = %#v, want %#v", got, want)
	}
	if len(listener.destroyed) != 1 || listener.destroyed[0].ContextName != "MyContext" {
		t.Fatalf("hash context-destroyed event = %#v", listener.destroyed)
	}
}
