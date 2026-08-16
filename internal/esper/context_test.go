package esper

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type contextLifecycleEvent struct {
	ID    string `esper:"id"`
	Kind  string `esper:"kind"`
	Value int    `esper:"value"`
}

type contextNullableEvent struct {
	Group string `esper:"group"`
	Key   *int   `esper:"key"`
	Value int    `esper:"value"`
}

type contextMultiLifecycleEvent struct {
	ID    string `esper:"id"`
	Group string `esper:"group"`
	Kind  string `esper:"kind"`
}

type contextArrayEvent struct {
	ID    string `esper:"id"`
	Keys  []int  `esper:"keys"`
	Value int    `esper:"value"`
}

type contextDistinctStartEvent struct {
	ID   string `esper:"id"`
	Kind string `esper:"kind"`
}

type contextDistinctEndEvent struct {
	Reason string `esper:"reason"`
}

type contextDistinctQueryEvent struct {
	Value int `esper:"value"`
}

type contextVariableEvent struct {
	Symbol string `esper:"symbol"`
	Price  int    `esper:"price"`
	Read   bool   `esper:"read"`
}

type contextPartitionTestListener struct {
	allocated   []ContextPartitionStateEvent
	deallocated []ContextPartitionStateEvent
}

func (l *contextPartitionTestListener) OnContextPartitionAllocated(event ContextPartitionStateEvent) {
	l.allocated = append(l.allocated, event)
}

func (l *contextPartitionTestListener) OnContextPartitionDeallocated(event ContextPartitionStateEvent) {
	l.deallocated = append(l.deallocated, event)
}

type contextStateTestListener struct {
	created     []ContextStateEvent
	destroyed   []ContextStateEvent
	added       []ContextStateEvent
	activated   []ContextStateEvent
	removed     []ContextStateEvent
	deactivated []ContextStateEvent
	allocated   []ContextPartitionStateEvent
	deallocated []ContextPartitionStateEvent
	sequence    []string
}

func (l *contextStateTestListener) OnContextCreated(event ContextStateEvent) {
	l.created = append(l.created, event)
	l.sequence = append(l.sequence, "created")
}

func (l *contextStateTestListener) OnContextDestroyed(event ContextStateEvent) {
	l.destroyed = append(l.destroyed, event)
	l.sequence = append(l.sequence, "destroyed")
}

func (l *contextStateTestListener) OnContextStatementAdded(event ContextStateEvent) {
	l.added = append(l.added, event)
	l.sequence = append(l.sequence, "statement-added")
}

func (l *contextStateTestListener) OnContextActivated(event ContextStateEvent) {
	l.activated = append(l.activated, event)
	l.sequence = append(l.sequence, "activated")
}

func (l *contextStateTestListener) OnContextStatementRemoved(event ContextStateEvent) {
	l.removed = append(l.removed, event)
	l.sequence = append(l.sequence, "statement-removed")
}

func (l *contextStateTestListener) OnContextDeactivated(event ContextStateEvent) {
	l.deactivated = append(l.deactivated, event)
	l.sequence = append(l.sequence, "deactivated")
}

func (l *contextStateTestListener) OnContextPartitionAllocated(event ContextPartitionStateEvent) {
	l.allocated = append(l.allocated, event)
	l.sequence = append(l.sequence, "partition-allocated")
}

func (l *contextStateTestListener) OnContextPartitionDeallocated(event ContextPartitionStateEvent) {
	l.deallocated = append(l.deallocated, event)
	l.sequence = append(l.sequence, "partition-deallocated")
}

func TestContextStateAndPartitionLifecycleListeners(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "lifecycle-context", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	listener := &contextStateTestListener{}
	if err := engine.AddContextStateListener(listener); err != nil {
		t.Fatal(err)
	}
	if len(listener.created) != 1 || listener.created[0].ContextName != "lifecycle-context" {
		t.Fatalf("created context replay = %#v", listener.created)
	}
	if err := engine.AddContextPartitionStateListener("lifecycle-context", listener); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("lifecycle-statement"),
		WithContext("lifecycle-context"),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(listener.added) != 1 || len(listener.activated) != 1 {
		t.Fatalf("statement lifecycle after deploy: added=%#v activated=%#v", listener.added, listener.activated)
	}
	if listener.added[0].StatementName != "lifecycle-statement" || listener.added[0].StatementDeploymentID == "" {
		t.Fatalf("statement event identity = %#v", listener.added[0])
	}
	if got, want := listener.sequence, []string{"created", "statement-added", "activated"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("deploy lifecycle order = %#v, want %#v", got, want)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(listener.allocated) != 1 {
		t.Fatalf("partition allocation events = %#v", listener.allocated)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(listener.removed) != 1 || len(listener.deallocated) != 1 || len(listener.deactivated) != 1 {
		t.Fatalf("statement teardown lifecycle: removed=%#v deallocated=%#v deactivated=%#v", listener.removed, listener.deallocated, listener.deactivated)
	}
	if len(listener.sequence) < 6 || listener.sequence[len(listener.sequence)-3] != "statement-removed" || listener.sequence[len(listener.sequence)-2] != "partition-deallocated" || listener.sequence[len(listener.sequence)-1] != "deactivated" {
		t.Fatalf("teardown lifecycle order = %#v", listener.sequence)
	}
	if err := engine.DestroyContext(context.Background(), "lifecycle-context"); err != nil {
		t.Fatal(err)
	}
	if len(listener.destroyed) != 1 || listener.destroyed[0].ContextName != "lifecycle-context" {
		t.Fatalf("destroyed context event = %#v", listener.destroyed)
	}
	if _, ok := env.Context("lifecycle-context"); ok {
		t.Fatal("context definition remains after DestroyContext")
	}
}

func TestContextPartitionStateListenerUsesSharedLifecycle(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	buildPlan := func(name string) Plan {
		plan, err := env.Build(Select(
			From[runtimeTestTrade](env, "Trade"),
			Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		).Query(StatementName(name), WithContext("by-symbol")))
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	first, err := engine.Deploy(context.Background(), buildPlan("context-listener-first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Deploy(context.Background(), buildPlan("context-listener-second"))
	if err != nil {
		t.Fatal(err)
	}
	listener := &contextPartitionTestListener{}
	if err := engine.AddContextPartitionStateListener("by-symbol", listener); err != nil {
		t.Fatal(err)
	}
	if listeners := engine.ContextPartitionStateListeners("by-symbol"); len(listeners) != 1 {
		t.Fatalf("registered context listeners = %#v", listeners)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(listener.allocated) != 2 {
		t.Fatalf("expected one allocation per shared partition, got %#v", listener.allocated)
	}
	if listener.allocated[0].PartitionID != 0 || listener.allocated[1].PartitionID != 1 {
		t.Fatalf("unexpected shared partition IDs: %#v", listener.allocated)
	}
	if listener.allocated[0].Descriptor.Key != listener.allocated[0].Key || listener.allocated[0].Descriptor.ContextName != "by-symbol" {
		t.Fatalf("allocation descriptor identity = %#v", listener.allocated[0])
	}
	count, err := engine.ContextPartitionCount("by-symbol")
	if err != nil || count != 2 {
		t.Fatalf("engine context partition count = %d, err=%v", count, err)
	}
	ids, err := engine.ContextPartitionIDs("by-symbol", SelectContextPartitions(listener.allocated[1].Key))
	if err != nil || len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("engine context partition IDs = %#v, err=%v", ids, err)
	}
	descriptor, found, err := engine.ContextPartition("by-symbol", 1)
	if err != nil || !found || descriptor.Key != listener.allocated[1].Key {
		t.Fatalf("engine context partition descriptor = %#v, found=%v, err=%v", descriptor, found, err)
	}
	properties, found, err := engine.ContextPartitionProperties("by-symbol", 1)
	if err != nil || !found || properties["key1"] != "B" {
		t.Fatalf("engine context partition properties = %#v, found=%v, err=%v", properties, found, err)
	}
	names, err := engine.ContextStatementNames("by-symbol")
	if err != nil || len(names) != 2 || names[0] != "context-listener-first" || names[1] != "context-listener-second" {
		t.Fatalf("engine context statement names = %#v, err=%v", names, err)
	}
	level, err := engine.ContextNestingLevel("by-symbol")
	if err != nil || level != 1 {
		t.Fatalf("engine context nesting level = %d, err=%v", level, err)
	}
	if err := first.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(listener.deallocated) != 0 {
		t.Fatalf("partition was deallocated while second statement remained: %#v", listener.deallocated)
	}
	if err := second.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(listener.deallocated) != 2 {
		t.Fatalf("expected final shared deallocations, got %#v", listener.deallocated)
	}
	if listener.deallocated[0].Key != listener.allocated[0].Key || listener.deallocated[1].Key != listener.allocated[1].Key {
		t.Fatalf("deallocation order/keys = %#v", listener.deallocated)
	}
	engine.RemoveContextPartitionStateListener("by-symbol", listener)
	if listeners := engine.ContextPartitionStateListeners("by-symbol"); len(listeners) != 0 {
		t.Fatalf("listener removal left registrations = %#v", listeners)
	}
}

func TestKeyContextPartitionsWindowState(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(1)).Query(
		StatementName("context-window"),
		WithContext("by-symbol"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}, {Symbol: "A", Price: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 3 || len(batches[0].Old) != 0 || len(batches[1].Old) != 0 || len(batches[2].Old) != 1 {
		t.Fatalf("context batches = %#v", batches)
	}
	old, ok := batches[2].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Price != 1 {
		t.Fatalf("context partition old stream = %#v", batches[2].Old)
	}
}

func TestContextFieldsExposeStableKeyNameAndID(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("contextName", ContextName()),
		Alias("contextID", ContextID()),
		Alias("contextKey", ContextKeyValue[string](0)),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("context-fields"), WithContext("by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "context field result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}, {Symbol: "A", Price: 3}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("context field rows = %#v", rows)
	}
	if rows[0].Get("contextName").Any() != "by-symbol" || rows[0].Get("contextID").Any() != 0 || rows[0].Get("contextKey").Any() != "A" {
		t.Fatalf("first context fields = %#v", rows[0].AsMap())
	}
	if rows[1].Get("contextName").Any() != "by-symbol" || rows[1].Get("contextID").Any() != 1 || rows[1].Get("contextKey").Any() != "B" {
		t.Fatalf("second context fields = %#v", rows[1].AsMap())
	}
	if rows[2].Get("contextID").Any() != rows[0].Get("contextID").Any() || rows[2].Get("contextKey").Any() != "A" {
		t.Fatalf("reused context fields = %#v", rows[2].AsMap())
	}
}

func TestTimePeriodContextCyclesWithVirtualClock(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := NewTimePeriodContext("invalid-negative", -time.Second, time.Second); err == nil {
		t.Fatal("negative temporal start delay was accepted")
	}
	if _, err := NewTimePeriodContext("invalid-active", time.Second, 0); err == nil {
		t.Fatal("zero temporal active duration was accepted")
	}
	definition, err := CreateTimePeriodContext(env, "periodic", 5*time.Second, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Kind() != ContextTimePeriod {
		t.Fatalf("temporal context kind = %v", definition.Kind())
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).Aggregate(
		Alias("start", ContextStartTime()),
		Alias("end", ContextEndTime()),
		Alias("total", Sum[float64](Field[runtimeTestTrade, float64]("price"))),
	).Query(StatementName("periodic-context"), WithContext("periodic")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	listener := &contextPartitionTestListener{}
	if err := engine.AddContextPartitionStateListener("periodic", listener); err != nil {
		t.Fatal(err)
	}

	assertCount := func(want int) {
		t.Helper()
		got, countErr := engine.ContextPartitionCount("periodic")
		if countErr != nil || got != want {
			t.Fatalf("temporal context partition count = %d, err=%v, want %d", got, countErr, want)
		}
	}
	assertDescriptorWindow := func(descriptor ContextPartitionDescriptor, start, end time.Time) {
		t.Helper()
		startValue, startOK := descriptor.Property("startTime")
		endValue, endOK := descriptor.Property("endTime")
		gotStart, startTypeOK := startValue.Any().(time.Time)
		gotEnd, endTypeOK := endValue.Any().(time.Time)
		if !startOK || !endOK || !startTypeOK || !endTypeOK || !gotStart.Equal(start) || !gotEnd.Equal(end) {
			t.Fatalf("temporal descriptor window = start(%#v,%v) end(%#v,%v), want %s..%s", startValue.Any(), startOK, endValue.Any(), endOK, start, end)
		}
	}
	assertRowWindow := func(row Row, start, end time.Time) {
		t.Helper()
		gotStart, startOK := row.Get("start").Any().(time.Time)
		gotEnd, endOK := row.Get("end").Any().(time.Time)
		if !startOK || !endOK || !gotStart.Equal(start) || !gotEnd.Equal(end) {
			t.Fatalf("temporal result window = %#v, want %s..%s", row.AsMap(), start, end)
		}
	}

	assertCount(0)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "before", Price: 1}); err != nil {
		t.Fatal(err)
	}
	before, err := statement.Snapshot(context.Background())
	if err != nil || len(before.Results()) != 0 {
		t.Fatalf("event before temporal start snapshot = %#v, err=%v", before.Results(), err)
	}

	startOne := time.Unix(5, 0).UTC()
	endOne := time.Unix(15, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), startOne); err != nil {
		t.Fatal(err)
	}
	assertCount(1)
	if len(listener.allocated) != 1 {
		t.Fatalf("temporal allocation events = %#v", listener.allocated)
	}
	descriptors, err := engine.ContextPartitionDescriptors("periodic", nil)
	if err != nil || len(descriptors) != 1 {
		t.Fatalf("temporal descriptors = %#v, err=%v", descriptors, err)
	}
	assertDescriptorWindow(descriptors[0], startOne, endOne)
	firstID := descriptors[0].ID

	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	active, err := statement.Snapshot(context.Background())
	if err != nil || len(active.Results()) != 1 {
		t.Fatalf("event inside first temporal window snapshot = %#v, err=%v", active.Results(), err)
	}
	row, ok := active.Results()[0].Row()
	if !ok || row.Get("total").Any() != float64(2) {
		t.Fatalf("first temporal result = %#v, row=%#v", active.Results()[0], row.AsMap())
	}
	assertRowWindow(row, startOne, endOne)
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 3}); err != nil {
		t.Fatal(err)
	}
	active, err = statement.Snapshot(context.Background())
	if err != nil || len(active.Results()) != 1 {
		t.Fatalf("updated first temporal window snapshot = %#v, err=%v", active.Results(), err)
	}
	row, ok = active.Results()[0].Row()
	if !ok || row.Get("total").Any() != float64(5) {
		t.Fatalf("updated first temporal result = %#v", active.Results()[0])
	}
	assertRowWindow(row, startOne, endOne)

	if err := engine.AdvanceTime(context.Background(), endOne); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if len(listener.deallocated) != 1 {
		t.Fatalf("temporal deallocation at end boundary = %#v", listener.deallocated)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "gap", Price: 3}); err != nil {
		t.Fatal(err)
	}
	gap, err := statement.Snapshot(context.Background())
	if err != nil || len(gap.Results()) != 0 {
		t.Fatalf("event in temporal gap snapshot = %#v, err=%v", gap.Results(), err)
	}

	startTwo := time.Unix(20, 0).UTC()
	endTwo := time.Unix(30, 0).UTC()
	if err := engine.AdvanceTime(context.Background(), startTwo); err != nil {
		t.Fatal(err)
	}
	assertCount(1)
	if len(listener.allocated) != 2 {
		t.Fatalf("second temporal allocation events = %#v", listener.allocated)
	}
	descriptors, err = engine.ContextPartitionDescriptors("periodic", nil)
	if err != nil || len(descriptors) != 1 {
		t.Fatalf("second temporal descriptors = %#v, err=%v", descriptors, err)
	}
	assertDescriptorWindow(descriptors[0], startTwo, endTwo)
	if descriptors[0].ID == firstID {
		t.Fatalf("temporal cycle reused active partition ID %d", firstID)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C", Price: 4}); err != nil {
		t.Fatal(err)
	}
	active, err = statement.Snapshot(context.Background())
	if err != nil || len(active.Results()) != 1 {
		t.Fatalf("event inside second temporal window snapshot = %#v, err=%v", active.Results(), err)
	}
	row, ok = active.Results()[0].Row()
	if !ok || row.Get("total").Any() != float64(4) {
		t.Fatalf("second temporal result = %#v", active.Results()[0])
	}
	assertRowWindow(row, startTwo, endTwo)

	if err := engine.AdvanceTime(context.Background(), endTwo); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if len(listener.deallocated) != 2 {
		t.Fatalf("second temporal deallocation = %#v", listener.deallocated)
	}
}

func TestTemporalContextTerminationSnapshotAtWindowBoundary(t *testing.T) {
	type temporalCase struct {
		name     string
		context  string
		origin   time.Time
		start    time.Time
		end      time.Time
		register func(*Environment) error
	}
	location := time.FixedZone("temporal-termination", 8*60*60)
	startSchedule := NewCronScheduleWithSeconds(
		CronValues(0), CronValues(0), CronValues(8),
		CronWildcard(), CronWildcard(), CronWildcard(),
	)
	endSchedule := NewCronScheduleWithSeconds(
		CronValues(0), CronValues(0), CronValues(9),
		CronWildcard(), CronWildcard(), CronWildcard(),
	)
	cases := []temporalCase{
		{
			name:    "periodic",
			context: "temporal-termination-periodic",
			origin:  time.Unix(0, 0).UTC(),
			start:   time.Unix(1, 0).UTC(),
			end:     time.Unix(5, 0).UTC(),
			register: func(env *Environment) error {
				_, err := CreateTimePeriodContext(env, "temporal-termination-periodic", time.Second, 4*time.Second)
				return err
			},
		},
		{
			name:    "daily",
			context: "temporal-termination-daily",
			origin:  time.Date(2024, time.May, 1, 8, 0, 0, 0, location),
			start:   time.Date(2024, time.May, 1, 9, 0, 0, 0, location),
			end:     time.Date(2024, time.May, 1, 10, 0, 0, 0, location),
			register: func(env *Environment) error {
				start, err := NewTimeOfDay(9, 0, 0)
				if err != nil {
					return err
				}
				end, err := NewTimeOfDay(10, 0, 0)
				if err != nil {
					return err
				}
				_, err = CreateDailyTimeContext(env, "temporal-termination-daily", start, end)
				return err
			},
		},
		{
			name:    "cron",
			context: "temporal-termination-cron",
			origin:  time.Date(2024, time.May, 1, 7, 30, 0, 0, location),
			start:   time.Date(2024, time.May, 1, 8, 0, 0, 0, location),
			end:     time.Date(2024, time.May, 1, 9, 0, 0, 0, location),
			register: func(env *Environment) error {
				_, err := CreateCronTimeContext(env, "temporal-termination-cron", startSchedule, endSchedule)
				return err
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
				t.Fatal(err)
			}
			if err := testCase.register(env); err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env, WithStartTime(testCase.origin))
			plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(KeepAll()).Aggregate(
				Alias("total", Sum[float64](Field[runtimeTestTrade, float64]("price"))),
				Alias("start", ContextStartTime()),
				Alias("end", ContextEndTime()),
			).Query(
				StatementName("termination-"+testCase.name),
				WithContext(testCase.context),
				WithOutput(OutputSnapshotWhenTerminated()),
			))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var batches []ResultBatch
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := engine.AdvanceTime(context.Background(), testCase.start); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
				t.Fatal(err)
			}
			if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 3}); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 0 {
				t.Fatalf("temporal termination emitted before boundary: %#v", batches)
			}
			if err := engine.AdvanceTime(context.Background(), testCase.end); err != nil {
				t.Fatal(err)
			}
			if len(batches) != 1 || len(batches[0].New) != 1 {
				t.Fatalf("temporal termination batches = %#v", batches)
			}
			row, ok := batches[0].New[0].Row()
			if !ok || row.Get("total").Any() != float64(5) {
				t.Fatalf("temporal termination row = %#v", batches[0].New[0])
			}
			gotStart, startOK := row.Get("start").Any().(time.Time)
			gotEnd, endOK := row.Get("end").Any().(time.Time)
			if !startOK || !endOK || !gotStart.Equal(testCase.start) || !gotEnd.Equal(testCase.end) {
				t.Fatalf("temporal termination window = %#v", row.AsMap())
			}
			if count := deployment.Statements()[0].ContextPartitionCount(); count != 0 {
				t.Fatalf("temporal partition remained after termination = %d", count)
			}
		})
	}
}

func TestDailyTimeContextUsesLocalCalendarBoundaries(t *testing.T) {
	start, err := NewTimeOfDay(9, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	end, err := NewTimeOfDay(17, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTimeOfDay(24, 0, 0); err == nil {
		t.Fatal("invalid daily start time was accepted")
	}
	if _, err := NewDailyTimeContext("equal-daily", start, start); err == nil {
		t.Fatal("equal daily start/end times were accepted")
	}
	overnightStart, err := NewTimeOfDay(22, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	overnightEnd, err := NewTimeOfDay(6, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	overnight, err := NewDailyTimeContext("overnight", overnightStart, overnightEnd)
	if err != nil {
		t.Fatal(err)
	}
	overnightWindowStart := time.Date(2024, time.May, 1, 20, 0, 0, 0, time.FixedZone("daily", 8*60*60))
	windowStart, windowEnd, active := overnight.temporalWindow(overnightWindowStart, time.Date(2024, time.May, 1, 23, 0, 0, 0, overnightWindowStart.Location()))
	if !active || !windowStart.Equal(time.Date(2024, time.May, 1, 22, 0, 0, 0, overnightWindowStart.Location())) || !windowEnd.Equal(time.Date(2024, time.May, 2, 6, 0, 0, 0, overnightWindowStart.Location())) {
		t.Fatalf("overnight daily window = %s..%s active=%v", windowStart, windowEnd, active)
	}
	_, _, active = overnight.temporalWindow(overnightWindowStart, time.Date(2024, time.May, 2, 6, 0, 0, 0, overnightWindowStart.Location()))
	if active {
		t.Fatal("overnight daily window remained active at its exclusive end")
	}

	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	definition, err := CreateDailyTimeContext(env, "business-hours", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Kind() != ContextDailyTime {
		t.Fatalf("daily context kind = %v", definition.Kind())
	}
	origin := time.Date(2024, time.May, 1, 8, 0, 0, 0, time.FixedZone("daily", 8*60*60))
	engine := NewEngine(env, WithStartTime(origin))
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade").Window(KeepAll()),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("start", ContextStartTime()),
		Alias("end", ContextEndTime()),
	).Query(StatementName("daily-context"), WithContext("business-hours")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if err := engine.AdvanceTime(context.Background(), time.Date(2024, time.May, 1, 8, 59, 59, 0, origin.Location())); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("daily context opened before start = %v", statement.ContextPartitions())
	}
	firstStart := time.Date(2024, time.May, 1, 9, 0, 0, 0, origin.Location())
	firstEnd := time.Date(2024, time.May, 1, 17, 0, 0, 0, origin.Location())
	if err := engine.AdvanceTime(context.Background(), firstStart); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("daily context did not open at start = %v", statement.ContextPartitions())
	}
	descriptors := statement.ContextPartitions()
	startValue, startOK := descriptors[0].Property("startTime")
	endValue, endOK := descriptors[0].Property("endTime")
	if !startOK || !endOK || !startValue.Any().(time.Time).Equal(firstStart) || !endValue.Any().(time.Time).Equal(firstEnd) {
		t.Fatalf("daily descriptor window = %#v", descriptors[0].Properties())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	result, err := statement.Snapshot(context.Background())
	if err != nil || len(result.Results()) != 1 {
		t.Fatalf("daily active snapshot = %#v, err=%v", result.Results(), err)
	}
	row, ok := result.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" {
		t.Fatalf("daily active row = %#v", result.Results()[0])
	}
	if got, ok := row.Get("start").Any().(time.Time); !ok || !got.Equal(firstStart) {
		t.Fatalf("daily row start = %#v", row.AsMap())
	}
	if err := engine.AdvanceTime(context.Background(), firstEnd); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("daily context remained active at end = %v", statement.ContextPartitions())
	}
	secondStart := time.Date(2024, time.May, 2, 9, 0, 0, 0, origin.Location())
	if err := engine.AdvanceTime(context.Background(), secondStart); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("daily context did not reopen next day = %v", statement.ContextPartitions())
	}
}

func TestCronTimeContextUsesNextCalendarEnd(t *testing.T) {
	startSchedule := NewCronScheduleWithSeconds(
		CronValues(0), CronValues(0), CronValues(8),
		CronWildcard(), CronWildcard(), CronWildcard(),
	)
	endSchedule := NewCronScheduleWithSeconds(
		CronValues(0), CronValues(0), CronValues(9),
		CronWildcard(), CronWildcard(), CronWildcard(),
	)
	if _, err := NewCronTimeContext("dynamic-cron", CronSchedule{Minute: CronEveryExpr(Literal(1))}, endSchedule); err == nil {
		t.Fatal("dynamic cron temporal context was accepted")
	}
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	definition, err := CreateCronTimeContext(env, "cron-hours", startSchedule, endSchedule)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Kind() != ContextCronTime {
		t.Fatalf("cron temporal context kind = %v", definition.Kind())
	}
	location := time.FixedZone("cron", 8*60*60)
	origin := time.Date(2024, time.May, 30, 7, 30, 0, 0, location)
	engine := NewEngine(env, WithStartTime(origin))
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade").Window(KeepAll()),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("start", ContextStartTime()),
		Alias("end", ContextEndTime()),
	).Query(StatementName("cron-context"), WithContext("cron-hours")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if err := engine.AdvanceTime(context.Background(), time.Date(2024, time.May, 30, 7, 59, 59, 0, location)); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("cron context opened before start = %v", statement.ContextPartitions())
	}
	firstStart := time.Date(2024, time.May, 30, 8, 0, 0, 0, location)
	firstEnd := time.Date(2024, time.May, 30, 9, 0, 0, 0, location)
	if err := engine.AdvanceTime(context.Background(), firstStart); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("cron context did not open at start = %v", statement.ContextPartitions())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	active, err := statement.Snapshot(context.Background())
	if err != nil || len(active.Results()) != 1 {
		t.Fatalf("cron active snapshot = %#v, err=%v", active.Results(), err)
	}
	row, ok := active.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" {
		t.Fatalf("cron active row = %#v", active.Results()[0])
	}
	if got, ok := row.Get("end").Any().(time.Time); !ok || !got.Equal(firstEnd) {
		t.Fatalf("cron end property = %#v", row.AsMap())
	}
	if err := engine.AdvanceTime(context.Background(), firstEnd); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("cron context remained active at end = %v", statement.ContextPartitions())
	}
	secondStart := time.Date(2024, time.May, 31, 8, 0, 0, 0, location)
	if err := engine.AdvanceTime(context.Background(), secondStart); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("cron context did not reopen next day = %v", statement.ContextPartitions())
	}
}

func TestContextFieldsRequireAStatementContext(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("name", ContextName()),
	).Query(StatementName("context-field-without-context"))); err == nil {
		t.Fatal("context field outside a context statement was accepted")
	}
}

func TestCategoryContextFieldsExposeLabel(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateCategoryContext(env, "price-bands",
		Category("high", GreaterOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
		Category("low", Less[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("name", ContextName()),
		Alias("label", ContextLabel()),
		Alias("id", ContextID()),
	).Query(StatementName("category-context-fields"), WithContext("price-bands")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var labels []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "category context result is not a row")
			}
			label, ok := row.Get("label").Any().(string)
			if !ok {
				return NewError(ErrorTypeMismatch, "category label is not a string")
			}
			labels = append(labels, label)
			if row.Get("name").Any() != "price-bands" || row.Get("id").Any() == nil {
				return NewError(ErrorState, "category context built-ins are incorrect")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 11}, {Symbol: "B", Price: 1}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(labels) != 2 || labels[0] != "high" || labels[1] != "low" {
		t.Fatalf("category context labels = %v", labels)
	}
}

func TestNestedContextFieldsExposeParentKey(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	child, err := NewKeyContext("by-price", Field[runtimeTestTrade, float64]("price"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "symbol-price", "by-symbol", child); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("name", ContextName()),
		Alias("childKey", ContextKeyValue[float64](0)),
		Alias("parentKey", ContextField[string]("parent.key1")),
	).Query(StatementName("nested-context-fields"), WithContext("symbol-price")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var row Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		if len(batch.New) != 0 {
			var ok bool
			row, ok = batch.New[0].Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "nested context result is not a row")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 12}); err != nil {
		t.Fatal(err)
	}
	if row.Get("name").Any() != "symbol-price" || row.Get("childKey").Any() != float64(12) || row.Get("parentKey").Any() != "A" {
		t.Fatalf("nested context fields = %#v", row.AsMap())
	}
}

func TestContextPartitionDescriptorsExposeSnapshotProperties(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("value", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("descriptor"), WithContext("by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 2}); err != nil {
		t.Fatal(err)
	}
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 2 {
		t.Fatalf("expected two descriptors, got %#v", descriptors)
	}
	if descriptors[0].ContextName != "by-symbol" || descriptors[0].Key == "" {
		t.Fatalf("unexpected descriptor identity: %#v", descriptors[0])
	}
	if descriptors[0].ID != 0 || descriptors[1].ID != 1 {
		t.Fatalf("expected monotonic partition ids, got %d and %d", descriptors[0].ID, descriptors[1].ID)
	}
	value, ok := descriptors[0].Property("key1")
	if !ok || value.State() != ValuePresent || value.Any() != "A" {
		t.Fatalf("expected key1=A, got %#v (present=%v)", value, ok)
	}
	properties := descriptors[0].Properties()
	if properties["name"] != "by-symbol" || properties["id"] != 0 || properties["key1"] != "A" {
		t.Fatalf("unexpected descriptor properties: %#v", properties)
	}
	properties["key1"] = "mutated"
	value, _ = descriptors[0].Property("key1")
	if value.Any() != "A" {
		t.Fatalf("descriptor properties leaked mutation: %#v", value.Any())
	}
	selected := statement.ContextPartitionsWith(SelectContextPartitions(descriptors[1].Key))
	if len(selected) != 1 || selected[0].ID != descriptors[1].ID {
		t.Fatalf("selector did not return requested descriptor: %#v", selected)
	}
	selectedByID := statement.ContextPartitionsWith(SelectContextPartitionIDs(descriptors[0].ID))
	if len(selectedByID) != 1 || selectedByID[0].ID != descriptors[0].ID || selectedByID[0].Key != descriptors[0].Key {
		t.Fatalf("id selector did not return requested descriptor: %#v", selectedByID)
	}
	keysByID := statement.ContextPartitionKeysWith(SelectContextPartitionIDs(descriptors[1].ID))
	if len(keysByID) != 1 || keysByID[0] != descriptors[1].Key {
		t.Fatalf("id selector did not return requested key: %#v", keysByID)
	}
}

func TestCategoryContextPartitionDescriptorExposesLabel(t *testing.T) {
	env, engine := newRuntimeTest(t)
	_, err := CreateCategoryContext(env, "risk",
		Category("high", GreaterOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
		Category("low", Less[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade"),
		Alias("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("category-descriptor"), WithContext("risk")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	for _, price := range []float64{100, 1} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "event", Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 2 {
		t.Fatalf("expected two category descriptors, got %#v", descriptors)
	}
	labels := map[string]bool{}
	for _, descriptor := range descriptors {
		value, ok := descriptor.Property("label")
		if !ok || value.State() != ValuePresent {
			t.Fatalf("missing category label in %#v", descriptor)
		}
		labels[value.Any().(string)] = true
	}
	if !labels["high"] || !labels["low"] {
		t.Fatalf("unexpected category labels: %#v", labels)
	}
	selected := statement.ContextPartitionsWith(SelectContextPartitionCategories("high"))
	if len(selected) != 1 {
		t.Fatalf("category selector returned %#v", selected)
	}
	label, ok := selected[0].Property("label")
	if !ok || label.Any() != "high" {
		t.Fatalf("category selector returned non-high partition: %#v", selected[0])
	}
}

func TestInitiatedTerminatedContextPartitionDescriptorFollowsLifecycle(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "Lifecycle"); err != nil {
		t.Fatal(err)
	}
	_, err := CreateInitiatedTerminatedContext(env, "session", Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[contextLifecycleEvent](env, "Lifecycle"),
		Alias("kind", Field[contextLifecycleEvent, string]("kind")),
	).Query(StatementName("lifecycle-descriptor"), WithContext("session")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	listener := &contextPartitionTestListener{}
	if err := engine.AddContextPartitionStateListener("session", listener); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextLifecycleEvent{ID: "A", Kind: "start"}); err != nil {
		t.Fatal(err)
	}
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 1 {
		t.Fatalf("expected active initiated partition, got %#v", descriptors)
	}
	if value, ok := descriptors[0].Property("key1"); !ok || value.Any() != "A" {
		t.Fatalf("unexpected initiated descriptor key: %#v", descriptors[0])
	}
	if err := engine.SendEvent(context.Background(), contextLifecycleEvent{ID: "A", Kind: "end"}); err != nil {
		t.Fatal(err)
	}
	if len(listener.allocated) != 1 || len(listener.deallocated) != 1 {
		t.Fatalf("initiated context lifecycle events = allocated %#v, deallocated %#v", listener.allocated, listener.deallocated)
	}
	if value, ok := listener.deallocated[0].Descriptor.Property("terminating_event"); !ok || value.IsNull() {
		t.Fatalf("deallocation descriptor lost terminating event: %#v", listener.deallocated[0].Descriptor)
	}
	if got := statement.ContextPartitions(); len(got) != 0 {
		t.Fatalf("expected terminated partition to disappear, got %#v", got)
	}
}

func TestInitiatedTerminatedContextFieldsExposeBoundaryEvents(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "Lifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(env, "session", Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end"))); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[contextLifecycleEvent](env, "Lifecycle"),
		Alias("startID", Property[string](ContextInitiatingEvent(), "id")),
		Alias("endID", Property[string](ContextTerminatingEvent(), "id")),
		Alias("kind", Field[contextLifecycleEvent, string]("kind")),
	).Query(StatementName("context-boundary-events"), WithContext("session")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "context boundary result is not a row")
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []contextLifecycleEvent{
		{ID: "A", Kind: "start"},
		{ID: "A", Kind: "end"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("boundary event rows = %#v", rows)
	}
	if rows[0].Get("startID").Any() != "A" || !rows[0].Get("endID").IsMissing() {
		t.Fatalf("start boundary row = %#v", rows[0].AsMap())
	}
	if rows[1].Get("startID").Any() != "A" || rows[1].Get("endID").Any() != "A" {
		t.Fatalf("end boundary row = %#v", rows[1].AsMap())
	}
}

func TestContextVariablesArePartitionScopedAndSharedAcrossStatements(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextVariableEvent](env, "ContextVariableEvent"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-symbol", Field[contextVariableEvent, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("by-symbol", "counter", 0); err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	input := From[contextVariableEvent](env, "ContextVariableEvent")
	setter, err := env.Build(OnEvent(input.Filter(Equal[bool](Field[contextVariableEvent, bool]("read"), Literal(false)))).SetVariable(
		"counter", Field[contextVariableEvent, int]("price"),
	).Query(StatementName("context-variable-set"), WithContext("by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := env.Build(Select(
		input.Filter(Equal[bool](Field[contextVariableEvent, bool]("read"), Literal(true))),
		Alias("counter", VariableRef[int]("counter")),
	).Query(StatementName("context-variable-read"), WithContext("by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	setterDeployment, err := engine.Deploy(context.Background(), setter)
	if err != nil {
		t.Fatal(err)
	}
	readerDeployment, err := engine.Deploy(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	var counters []int
	if _, err := readerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "context variable result is not a row")
			}
			value, ok := row.Get("counter").Any().(int)
			if !ok {
				return NewError(ErrorTypeMismatch, "context variable result is not an int")
			}
			counters = append(counters, value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []contextVariableEvent{
		{Symbol: "A", Price: 0, Read: false},
		{Symbol: "A", Price: 10, Read: false},
		{Symbol: "A", Read: true},
		{Symbol: "B", Read: true},
		{Symbol: "B", Price: 11, Read: false},
		{Symbol: "B", Read: true},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(counters) != 3 || counters[0] != 10 || counters[1] != 0 || counters[2] != 11 {
		t.Fatalf("partition-scoped counters = %#v", counters)
	}
	var keyA string
	for _, descriptor := range readerDeployment.Statements()[0].ContextPartitions() {
		keyValue, _ := descriptor.Property("key1")
		if keyValue.Any() == "A" {
			keyA = descriptor.Key
		}
	}
	if keyA == "" {
		t.Fatal("did not find A context partition")
	}
	if err := engine.SetContextVariable(context.Background(), "by-symbol", keyA, "counter", 20); err != nil {
		t.Fatal(err)
	}
	value, ok := engine.GetContextVariable(context.Background(), "by-symbol", keyA, "counter")
	if !ok || value.Any() != 20 {
		t.Fatalf("context variable API value = %#v, %v", value, ok)
	}
	snapshot := engine.ContextVariableValues(context.Background(), "by-symbol", keyA)
	snapshot["counter"] = Present(99)
	value, _ = engine.GetContextVariable(context.Background(), "by-symbol", keyA, "counter")
	if value.Any() != 20 {
		t.Fatalf("context variable snapshot leaked mutation: %#v", value)
	}
	if err := engine.SendEvent(context.Background(), contextVariableEvent{Symbol: "A", Read: true}); err != nil {
		t.Fatal(err)
	}
	if len(counters) != 4 || counters[3] != 20 {
		t.Fatalf("direct context variable update not observed: %#v", counters)
	}
	if setterDeployment.Statements()[0].ContextPartitionCount() != 2 {
		t.Fatalf("setter context partition count = %d", setterDeployment.Statements()[0].ContextPartitionCount())
	}
}

func TestContextVariableManagementSnapshotsAndAtomicBatch(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("by-symbol", "counter", 0); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("by-symbol", "limit", 100); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("global", 1); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("context-variable-management"),
		WithContext("by-symbol"),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	statement := deployment.Statements()[0]
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 2 {
		t.Fatalf("management context descriptors = %#v", descriptors)
	}
	states, err := engine.ContextVariableStates(context.Background(), "by-symbol", ContextPartitionSelectorAll{}, "counter", "limit")
	if err != nil || len(states) != 2 {
		t.Fatalf("context variable states = %#v, err=%v", states, err)
	}
	if states[0].Values["counter"].Any() != 0 || states[0].Values["limit"].Any() != 100 {
		t.Fatalf("initial context variable state = %#v", states[0])
	}
	keyA := descriptors[0].Key
	if keyValue, ok := descriptors[0].Property("key1"); !ok || keyValue.Any() != "A" {
		keyA = descriptors[1].Key
	}
	if err := engine.SetContextVariables(context.Background(), "by-symbol", keyA,
		VariableAssignment{Name: "counter", Value: 10},
		VariableAssignment{Name: "limit", Value: "wrong"},
	); err == nil || !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("context variable batch type error = %v", err)
	}
	unchanged, ok := engine.ContextVariableValues(context.Background(), "by-symbol", keyA)["counter"]
	if !ok || unchanged.Any() != 0 {
		t.Fatalf("failed context batch partially changed state = %#v", unchanged)
	}
	if err := engine.SetContextVariables(context.Background(), "by-symbol", keyA,
		VariableAssignment{Name: "counter", Value: 10},
		VariableAssignment{Name: "limit", Value: 20},
	); err != nil {
		t.Fatal(err)
	}
	selectedStates, err := engine.ContextVariableStates(context.Background(), "by-symbol", SelectContextPartitions(keyA), "counter", "limit")
	if err != nil || len(selectedStates) != 1 {
		t.Fatalf("selected context variable states = %#v, err=%v", selectedStates, err)
	}
	if selectedStates[0].Values["counter"].Any() != 10 || selectedStates[0].Values["limit"].Any() != 20 {
		t.Fatalf("updated context variable state = %#v", selectedStates[0])
	}
	selectedStates[0].Values["counter"] = Present(999)
	selectedAgain, err := engine.ContextVariableStates(context.Background(), "by-symbol", SelectContextPartitions(keyA), "counter")
	if err != nil || len(selectedAgain) != 1 || selectedAgain[0].Values["counter"].Any() != 10 {
		t.Fatalf("context variable state leaked mutation = %#v, err=%v", selectedAgain, err)
	}
	global, err := engine.VariableValues(context.Background(), "global")
	if err != nil || global["global"].Any() != 1 {
		t.Fatalf("global variable snapshot = %#v, err=%v", global, err)
	}
	if _, err := engine.VariableValues(context.Background(), "counter"); err == nil {
		t.Fatal("context variable was returned as a global variable")
	}
}

func TestContextVariableScopeValidation(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "other-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("by-symbol", "counter", 0); err != nil {
		t.Fatal(err)
	}
	query := Select(From[runtimeTestTrade](env, "Trade"), Alias("counter", VariableRef[int]("counter")))
	if _, err := env.Build(query.Query(StatementName("context-variable-outside"))); err == nil {
		t.Fatal("context variable was usable outside its context")
	}
	if _, err := env.Build(query.Query(StatementName("context-variable-wrong-context"), WithContext("other-symbol"))); err == nil {
		t.Fatal("context variable was usable from a different context")
	}
}

func TestInitiatedTerminatedContextVariablesResetAfterLastPartitionEnds(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "Lifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(env, "session", Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end"))); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("session", "counter", 5); err != nil {
		t.Fatal(err)
	}
	input := From[contextLifecycleEvent](env, "Lifecycle")
	setter, err := env.Build(OnEvent(input.Filter(Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("update")))).SetVariable(
		"counter", Field[contextLifecycleEvent, int]("value"),
	).Query(StatementName("initiated-context-variable-set"), WithContext("session")))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := env.Build(Select(
		input.Filter(Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("read"))),
		Alias("counter", VariableRef[int]("counter")),
	).Query(StatementName("initiated-context-variable-read"), WithContext("session")))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	if _, err := engine.Deploy(context.Background(), setter); err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), reader)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if !ok {
				return NewError(ErrorTypeMismatch, "initiated context variable result is not a row")
			}
			values = append(values, row.Get("counter").Any().(int))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []contextLifecycleEvent{
		{ID: "A", Kind: "start"},
		{ID: "A", Kind: "update", Value: 10},
		{ID: "A", Kind: "read"},
		{ID: "A", Kind: "end"},
		{ID: "A", Kind: "start"},
		{ID: "A", Kind: "read"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(values) != 2 || values[0] != 10 || values[1] != 5 {
		t.Fatalf("initiated context variable lifecycle = %#v", values)
	}
}

func TestContextFieldsAreAvailableInFireAndForgetPartitions(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "context-fields-live", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-symbol", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.InsertNamedWindow(context.Background(), "context-fields-live", event); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := env.Build(FromNamedWindow(env, "context-fields-live").Select(
		Alias("key", ContextKeyValue[string](0)),
		Alias("id", ContextID()),
		Alias("symbol", Field[any, string]("symbol")),
	).Query(StatementName("context-fields-faf"), WithContext("by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results()) != 2 {
		t.Fatalf("context field FAF result count = %#v", result.Results())
	}
	for index, expected := range []string{"A", "B"} {
		row, ok := result.Results()[index].Row()
		if !ok || row.Get("key").Any() != expected || row.Get("symbol").Any() != expected || row.Get("id").Any() != index {
			t.Fatalf("context field FAF row %d = %#v", index, result.Results()[index])
		}
	}
}

func TestMultiKeyContextPartitionsByCompleteTuple(t *testing.T) {
	env, _ := newRuntimeTest(t)
	definition, err := CreateKeyContext(
		env,
		"by-symbol-price",
		Field[runtimeTestTrade, string]("symbol"),
		Field[runtimeTestTrade, float64]("price"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Keys()) != 2 || definition.Key() == nil {
		t.Fatalf("multi-key context definition = %#v", definition.Keys())
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(1)).Query(
		StatementName("multi-key-context-window"),
		WithContext("by-symbol-price"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("multi-key partitions = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
	selected := statement.ContextPartitionsWith(SelectContextPartitionSegments([]any{"A", float64(2)}))
	if len(selected) != 1 {
		t.Fatalf("segmented selector returned %#v", selected)
	}
	if value, ok := selected[0].Property("key2"); !ok || value.Any() != float64(2) {
		t.Fatalf("segmented selector returned wrong tuple: %#v", selected[0])
	}
	if len(batches) != 3 || len(batches[2].Old) != 1 {
		t.Fatalf("multi-key context batches = %#v", batches)
	}
	old, ok := batches[2].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Price != 1 {
		t.Fatalf("multi-key tuple old event = %#v", batches[2].Old)
	}
}

func TestMultiKeyHashContextUsesCompleteTuple(t *testing.T) {
	env, engine := newRuntimeTest(t)
	definition, err := CreateHashContextBy(
		env,
		"hash-symbol-price",
		3,
		Field[runtimeTestTrade, string]("symbol"),
		Field[runtimeTestTrade, float64]("price"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Keys()) != 2 || definition.Partitions() != 3 {
		t.Fatalf("multi-key hash definition = %#v", definition)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("multi-key-hash-context"),
		WithContext("hash-symbol-price"),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if statement.ContextPartitionCount() == 0 || statement.ContextPartitionCount() > 3 {
		t.Fatalf("multi-key hash partitions = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
	for _, key := range statement.ContextPartitionKeys() {
		if len(key) < len("hash:") || key[:len("hash:")] != "hash:" {
			t.Fatalf("multi-key hash partition key = %q", key)
		}
	}
}

func TestMultiKeyContextKeepsNullTupleComponentStable(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextNullableEvent](env, "Nullable"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(
		env,
		"by-group-key",
		Field[contextNullableEvent, string]("group"),
		Field[contextNullableEvent, *int]("key"),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextNullableEvent](env, "Nullable").Window(LengthWindow(1)).Query(
		StatementName("multi-key-null-context"),
		WithContext("by-group-key"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	if err := engine.SendEvent(context.Background(), contextNullableEvent{Group: "A", Value: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextNullableEvent{Group: "A", Value: 2}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("null tuple partition count after repeat = %d", statement.ContextPartitionCount())
	}
	key := 10
	if err := engine.SendEvent(context.Background(), contextNullableEvent{Group: "A", Key: &key, Value: 3}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("null tuple partition count after present key = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
}

func TestMultiKeyContextUsesArrayComponentContent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextArrayEvent](env, "ArrayContext"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(
		env,
		"by-id-array",
		Field[contextArrayEvent, string]("id"),
		Field[contextArrayEvent, []int]("keys"),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextArrayEvent](env, "ArrayContext").Query(
		StatementName("array-context"),
		WithContext("by-id-array"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	for _, event := range []contextArrayEvent{
		{ID: "A", Keys: []int{1, 2}, Value: 1},
		{ID: "A", Keys: []int{1, 2}, Value: 2},
		{ID: "A", Keys: []int{2, 1}, Value: 3},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("array tuple partitions = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
}

func TestMultiKeyContextConstructorsRejectInvalidKeys(t *testing.T) {
	if _, err := NewKeyContext("empty"); err == nil {
		t.Fatal("empty multi-key context was accepted")
	}
	if _, err := NewKeyContext("nil-key", Literal("A"), nil); err == nil {
		t.Fatal("nil key in multi-key context was accepted")
	}
	if _, err := NewHashContextBy("empty-hash", 2); err == nil {
		t.Fatal("empty multi-key hash context was accepted")
	}
	if _, err := NewHashContextBy("bad-partitions", 0, Literal("A"), Literal("B")); err == nil {
		t.Fatal("zero-partition multi-key hash context was accepted")
	}
}

func TestContextPartitionedAggregateKeepsRollupStateIsolated(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	price := Field[runtimeTestTrade, float64]("price")
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(2)).GroupByRollup(price).Select(
		Alias("price", price),
		Alias("groupingId", GroupingID(price)),
		Alias("count", CountAll()),
	).Query(StatementName("context-rollup-aggregate"), WithContext("by-symbol"), WithOldStream()))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var last ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		last = batch
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}, {Symbol: "B", Price: 9}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if len(last.New) != 2 {
		t.Fatalf("context rollup last new rows = %#v", last.New)
	}
	var detail, subtotal Row
	for _, result := range last.New {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("context rollup result is not a row: %#v", result)
		}
		if row.Get("price").IsNull() {
			subtotal = row
		} else {
			detail = row
		}
	}
	if detail.Get("price").Any() != float64(9) || detail.Get("groupingId").Any() != int64(0) || detail.Get("count").Any() != int64(1) {
		t.Fatalf("context rollup detail = %#v", detail.AsMap())
	}
	if !subtotal.Get("price").IsNull() || subtotal.Get("groupingId").Any() != int64(1) || subtotal.Get("count").Any() != int64(1) {
		t.Fatalf("context rollup subtotal = %#v", subtotal.AsMap())
	}
}

func TestContextFireAndForgetAggregateHonorsPartitionSelector(t *testing.T) {
	env, _ := newRuntimeTest(t)
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "context-trades", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-symbol", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "A", Price: 2}, {Symbol: "B", Price: 9}} {
		if err := engine.InsertNamedWindow(context.Background(), "context-trades", event); err != nil {
			t.Fatal(err)
		}
	}
	price := Field[any, float64]("price")
	plan, err := env.Build(FromNamedWindow(env, "context-trades").GroupByRollup(price).Select(
		Alias("price", price),
		Alias("groupingId", GroupingID(price)),
		Alias("count", CountAll()),
	).Query(StatementName("context-faf-rollup"), WithContext("by-symbol")))
	if err != nil {
		t.Fatal(err)
	}
	keyA := encodeKey([]any{ValuePresent, "A"})
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyA))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Results()) != 3 {
		t.Fatalf("context FAF aggregate A rows = %#v", selected.Results())
	}
	selectedByID, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitionIDs(0))
	if err != nil {
		t.Fatal(err)
	}
	if len(selectedByID.Results()) != 3 {
		t.Fatalf("context FAF aggregate partition id 0 rows = %#v", selectedByID.Results())
	}
	all, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Results()) != 5 {
		t.Fatalf("context FAF aggregate all rows = %#v", all.Results())
	}
}

func TestHashAndCategoryContextsMaterializeExpectedPartitions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateHashContext(env, "hash-symbol", Field[runtimeTestTrade, string]("symbol"), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCategoryContext(env, "price-bands",
		Category("high", GreaterOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
		Category("low", Less[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
	); err != nil {
		t.Fatal(err)
	}

	hashPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("hash-context"),
		WithContext("hash-symbol"),
	))
	if err != nil {
		t.Fatal(err)
	}
	hashDeployment, err := engine.Deploy(context.Background(), hashPlan)
	if err != nil {
		t.Fatal(err)
	}
	hashStatement := hashDeployment.Statements()[0]
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if hashStatement.ContextPartitionCount() == 0 || hashStatement.ContextPartitionCount() > 2 {
		t.Fatalf("hash partitions = %d (%v)", hashStatement.ContextPartitionCount(), hashStatement.ContextPartitionKeys())
	}
	for _, key := range hashStatement.ContextPartitionKeys() {
		if len(key) < len("hash:") || key[:len("hash:")] != "hash:" {
			t.Fatalf("hash partition key = %q", key)
		}
	}
	hashDescriptors := hashStatement.ContextPartitions()
	if len(hashDescriptors) == 0 {
		t.Fatal("hash descriptors are empty")
	}
	hashValue, ok := hashDescriptors[0].Property("hash")
	if !ok || hashValue.State() != ValuePresent {
		t.Fatalf("hash descriptor property = %#v", hashDescriptors[0])
	}
	selectedHash := hashStatement.ContextPartitionsWith(SelectContextPartitionHashes(hashValue.Any().(int64)))
	if len(selectedHash) != 1 || selectedHash[0].ID != hashDescriptors[0].ID {
		t.Fatalf("hash selector returned %#v", selectedHash)
	}

	categoryPlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("category-context"),
		WithContext("price-bands"),
	))
	if err != nil {
		t.Fatal(err)
	}
	categoryDeployment, err := engine.Deploy(context.Background(), categoryPlan)
	if err != nil {
		t.Fatal(err)
	}
	categoryStatement := categoryDeployment.Statements()[0]
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C", Price: 11}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "D", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if categoryStatement.ContextPartitionCount() != 2 {
		t.Fatalf("category partitions = %d (%v)", categoryStatement.ContextPartitionCount(), categoryStatement.ContextPartitionKeys())
	}
	keys := categoryStatement.ContextPartitionKeys()
	if keys[0] != "category:high" || keys[1] != "category:low" {
		t.Fatalf("category keys = %v", keys)
	}
}

func TestCategoryContextRejectsNoCategories(t *testing.T) {
	env := NewEnvironment()
	if _, err := CreateCategoryContext(env, "empty"); err == nil {
		t.Fatal("empty category context was accepted")
	}
}

func TestInitiatedTerminatedContextCreatesAndRemovesPartition(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "Lifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(env, "lifecycle",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextLifecycleEvent](env, "Lifecycle").Query(
		StatementName("initiated-terminated"),
		WithContext("lifecycle"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches int
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextLifecycleEvent{ID: "A", Kind: "noise", Value: 0})
	send(contextLifecycleEvent{ID: "A", Kind: "start", Value: 1})
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("partition after start = %d", statement.ContextPartitionCount())
	}
	send(contextLifecycleEvent{ID: "A", Kind: "middle", Value: 2})
	send(contextLifecycleEvent{ID: "A", Kind: "end", Value: 3})
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("partition after end = %d", statement.ContextPartitionCount())
	}
	send(contextLifecycleEvent{ID: "A", Kind: "middle", Value: 4})
	if batches != 3 {
		t.Fatalf("initiated-terminated batches = %d, want 3", batches)
	}
}

func TestInitiatedTerminatedContextSnapshotExcludesTerminatingEvent(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "SnapshotLifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(
		env,
		"snapshot-lifecycle",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextLifecycleEvent](env, "SnapshotLifecycle").Window(KeepAll()).Aggregate(
		Alias("total", Sum[int](Field[contextLifecycleEvent, int]("value"))),
		Alias("terminatingValue", Property[int](ContextTerminatingEvent(), "value")),
	).Query(
		StatementName("snapshot-when-terminated"),
		WithContext("snapshot-lifecycle"),
		WithOutput(OutputSnapshotWhenTerminated()),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextLifecycleEvent{ID: "A", Kind: "start", Value: 1})
	send(contextLifecycleEvent{ID: "A", Kind: "middle", Value: 2})
	send(contextLifecycleEvent{ID: "A", Kind: "end", Value: 100})
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("termination snapshot batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("total").Any() != 3 || row.Get("terminatingValue").Any() != 100 {
		t.Fatalf("termination snapshot row = %#v", row)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("termination snapshot partition count = %d", statement.ContextPartitionCount())
	}
}

func TestInitiatedTerminatedContextCanCombineEveryOutputWithTerminationSnapshot(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "EveryLifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(
		env,
		"every-lifecycle",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextLifecycleEvent](env, "EveryLifecycle").Window(KeepAll()).Query(
		StatementName("every-and-terminated"),
		WithContext("every-lifecycle"),
		WithOutput(OutputAndWhenTerminated(OutputEvery(2))),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextLifecycleEvent{ID: "A", Kind: "start", Value: 1})
	send(contextLifecycleEvent{ID: "A", Kind: "middle", Value: 2})
	send(contextLifecycleEvent{ID: "A", Kind: "middle", Value: 3})
	send(contextLifecycleEvent{ID: "A", Kind: "end", Value: 100})
	if len(batches) != 2 || len(batches[0].New) != 2 || len(batches[1].New) != 3 {
		t.Fatalf("every plus termination batches = %#v", batches)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("every plus termination partition count = %d", statement.ContextPartitionCount())
	}
}

func TestInitiatedTerminatedContextCanFlushPendingOutputAtTermination(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "PendingLifecycle"); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterVariable("pending-termination-fired", false); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(
		env,
		"pending-lifecycle",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	policy := OutputWhenTerminatedIf(
		GreaterOrEqual[int64](OutputCountInsert(), Literal(int64(2))),
		SetOutputVariable("pending-termination-fired", Literal(true)),
	)
	plan, err := env.Build(From[contextLifecycleEvent](env, "PendingLifecycle").Window(KeepAll()).Query(
		StatementName("pending-termination"),
		WithContext("pending-lifecycle"),
		WithOutput(policy),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextLifecycleEvent{ID: "A", Kind: "start", Value: 1})
	send(contextLifecycleEvent{ID: "A", Kind: "middle", Value: 2})
	send(contextLifecycleEvent{ID: "A", Kind: "end", Value: 100})
	if len(batches) != 1 || len(batches[0].New) != 2 {
		t.Fatalf("pending termination batches = %#v", batches)
	}
	value, ok := engine.GetVariable("pending-termination-fired")
	if !ok || value.Any() != true {
		t.Fatalf("pending termination variable = %#v, ok=%v", value, ok)
	}
}

func TestMultiKeyInitiatedTerminatedContextUsesTupleIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextMultiLifecycleEvent](env, "MultiLifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContextBy(
		env,
		"multi-lifecycle",
		[]Expr{
			Field[contextMultiLifecycleEvent, string]("id"),
			Field[contextMultiLifecycleEvent, string]("group"),
		},
		Equal[string](Field[contextMultiLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextMultiLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextMultiLifecycleEvent](env, "MultiLifecycle").Query(
		StatementName("multi-lifecycle-context"),
		WithContext("multi-lifecycle"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	send := func(event contextMultiLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextMultiLifecycleEvent{ID: "A", Group: "X", Kind: "start"})
	send(contextMultiLifecycleEvent{ID: "A", Group: "Y", Kind: "start"})
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("multi-key lifecycle partitions after start = %d", statement.ContextPartitionCount())
	}
	send(contextMultiLifecycleEvent{ID: "A", Group: "X", Kind: "end"})
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("multi-key lifecycle partitions after first end = %d", statement.ContextPartitionCount())
	}
	send(contextMultiLifecycleEvent{ID: "A", Group: "X", Kind: "middle"})
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("terminated multi-key lifecycle was recreated = %d", statement.ContextPartitionCount())
	}
	send(contextMultiLifecycleEvent{ID: "A", Group: "Y", Kind: "end"})
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("multi-key lifecycle partitions after all end = %d", statement.ContextPartitionCount())
	}
}

func TestDistinctInitiatedTerminatedContextBroadcastsAndSuppressesDuplicate(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "DistinctLifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDistinctInitiatedTerminatedContext(
		env,
		"distinct-lifecycle",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(From[contextLifecycleEvent](env, "DistinctLifecycle"),
		Alias("contextKey", ContextKeyValue[string](0)),
		Alias("startID", Property[string](ContextInitiatingEvent(), "id")),
		Alias("endID", Property[string](ContextTerminatingEvent(), "id")),
	).Query(
		StatementName("distinct-lifecycle-statement"),
		WithContext("distinct-lifecycle"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	send(contextLifecycleEvent{ID: "noise", Kind: "noise"})
	send(contextLifecycleEvent{ID: "A", Kind: "start"})
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("distinct partition count after A = %d", statement.ContextPartitionCount())
	}
	send(contextLifecycleEvent{ID: "A", Kind: "start"})
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("duplicate distinct initiation created a partition: %v", statement.ContextPartitionKeys())
	}
	send(contextLifecycleEvent{ID: "B", Kind: "start"})
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("distinct partition count after B = %d", statement.ContextPartitionCount())
	}
	if len(batches) != 3 || len(batches[2].New) != 2 {
		t.Fatalf("distinct broadcast batches = %#v", batches)
	}
	for _, result := range batches[2].New {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("distinct broadcast result is not a row: %#v", result)
		}
		if key := row.Get("contextKey").Any(); key != "A" && key != "B" {
			t.Fatalf("distinct broadcast context key = %#v", key)
		}
		if row.Get("startID").Any() == nil {
			t.Fatalf("distinct broadcast lost initiating event: %#v", row.AsMap())
		}
	}

	send(contextLifecycleEvent{ID: "close", Kind: "end"})
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("distinct partitions after all-termination = %d", statement.ContextPartitionCount())
	}
	if len(batches) != 4 || len(batches[3].New) != 2 {
		t.Fatalf("distinct terminating broadcast batches = %#v", batches)
	}
	for _, result := range batches[3].New {
		row, ok := result.Row()
		if !ok || row.Get("endID").Any() != "close" {
			t.Fatalf("distinct terminating event property = %#v", result)
		}
	}
}

func TestDistinctInitiatedTerminatedContextSupportsArrayTupleIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextArrayEvent](env, "DistinctArray"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDistinctInitiatedTerminatedContextBy(
		env,
		"distinct-array",
		[]Expr{
			Field[contextArrayEvent, string]("id"),
			Field[contextArrayEvent, []int]("keys"),
		},
		Equal[int](Field[contextArrayEvent, int]("value"), Literal(1)),
		Equal[int](Field[contextArrayEvent, int]("value"), Literal(9)),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextArrayEvent](env, "DistinctArray").Query(
		StatementName("distinct-array-statement"),
		WithContext("distinct-array"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	send := func(event contextArrayEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextArrayEvent{ID: "A", Keys: []int{1, 2}, Value: 1})
	send(contextArrayEvent{ID: "A", Keys: []int{1, 2}, Value: 1})
	send(contextArrayEvent{ID: "A", Keys: []int{2, 1}, Value: 1})
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("distinct array tuple partitions = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
	send(contextArrayEvent{ID: "close", Value: 9})
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("distinct array partitions after termination = %d", statement.ContextPartitionCount())
	}
}

func TestDistinctInitiatedTerminatedContextKeepsNullTupleStable(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextNullableEvent](env, "DistinctNullable"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDistinctInitiatedTerminatedContextBy(
		env,
		"distinct-nullable",
		[]Expr{
			Field[contextNullableEvent, string]("group"),
			Field[contextNullableEvent, *int]("key"),
		},
		Equal[int](Field[contextNullableEvent, int]("value"), Literal(1)),
		Equal[int](Field[contextNullableEvent, int]("value"), Literal(9)),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextNullableEvent](env, "DistinctNullable").Query(
		StatementName("distinct-nullable-statement"),
		WithContext("distinct-nullable"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	key := 7
	for _, event := range []contextNullableEvent{
		{Group: "A", Value: 1},
		{Group: "A", Value: 2},
		{Group: "A", Key: &key, Value: 1},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("distinct null tuple partitions = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
}

func TestOverlappingInitiatedContextCreatesBroadcastInstances(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "OverlappingLifecycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateOverlappingInitiatedContext(
		env,
		"overlapping-lifecycle",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(From[contextLifecycleEvent](env, "OverlappingLifecycle"),
		Alias("contextID", ContextID()),
		Alias("startID", Property[string](ContextInitiatingEvent(), "id")),
	).Query(
		StatementName("overlapping-lifecycle-statement"),
		WithContext("overlapping-lifecycle"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextLifecycleEvent{ID: "A", Kind: "start"})
	send(contextLifecycleEvent{ID: "A", Kind: "start"})
	send(contextLifecycleEvent{ID: "A", Kind: "middle"})
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("overlapping instances = %d (%v)", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
	baseKey := "initiated:" + encodeKey([]any{ValuePresent, "A"})
	descriptors, err := engine.ContextPartitionDescriptors("overlapping-lifecycle", SelectContextPartitions(baseKey))
	if err != nil || len(descriptors) != 2 {
		t.Fatalf("base-key selector did not select all overlapping instances: %#v, err=%v", descriptors, err)
	}
	for _, descriptor := range descriptors {
		if descriptor.BaseKey != baseKey || descriptor.Key == descriptor.BaseKey {
			t.Fatalf("overlapping descriptor public/internal key mapping = %#v", descriptor)
		}
	}
	if len(batches) != 3 || len(batches[0].New) != 1 || len(batches[1].New) != 2 || len(batches[2].New) != 2 {
		t.Fatalf("overlapping broadcast batches = %#v", batches)
	}
}

func TestInitiatedContextWithoutTerminationConditionIsNonOverlapping(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "InitiatedNoEnd"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedContext(
		env,
		"initiated-no-end",
		Field[contextLifecycleEvent, string]("id"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[contextLifecycleEvent](env, "InitiatedNoEnd").Query(
		StatementName("initiated-no-end-statement"),
		WithContext("initiated-no-end"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	for _, event := range []contextLifecycleEvent{
		{ID: "A", Kind: "start"},
		{ID: "A", Kind: "start"},
		{ID: "A", Kind: "middle"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("non-overlapping no-end partition count = %d", statement.ContextPartitionCount())
	}
}

func TestOverlappingInitiatedTerminatedContextCorrelatesEachInstance(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[contextLifecycleEvent](env, "OverlappingTerminated"); err != nil {
		t.Fatal(err)
	}
	initiatingID := Property[string](ContextInitiatingEvent(), "id")
	if _, err := CreateOverlappingInitiatedTerminatedContext(
		env,
		"overlapping-terminated",
		Literal("global"),
		Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("start")),
		And(
			Equal[string](Field[contextLifecycleEvent, string]("kind"), Literal("end")),
			Equal[string](initiatingID, Field[contextLifecycleEvent, string]("id")),
		),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(From[contextLifecycleEvent](env, "OverlappingTerminated"),
		Alias("startID", Property[string](ContextInitiatingEvent(), "id")),
		Alias("endID", Property[string](ContextTerminatingEvent(), "id")),
	).Query(
		StatementName("overlapping-terminated-statement"),
		WithContext("overlapping-terminated"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event contextLifecycleEvent) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(contextLifecycleEvent{ID: "A", Kind: "start"})
	send(contextLifecycleEvent{ID: "B", Kind: "start"})
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("overlapping terminated instances = %d", statement.ContextPartitionCount())
	}
	send(contextLifecycleEvent{ID: "A", Kind: "end"})
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("one correlated instance should remain = %d", statement.ContextPartitionCount())
	}
	if len(batches) != 3 || len(batches[2].New) != 2 {
		t.Fatalf("correlated termination batch = %#v", batches)
	}
	send(contextLifecycleEvent{ID: "B", Kind: "end"})
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("all overlapping instances after correlated ends = %d", statement.ContextPartitionCount())
	}
}

func TestInitiatedTerminatedContextTerminatesFromDifferentEventType(t *testing.T) {
	env := NewEnvironment()
	for name, register := range map[string]func() error{
		"Start": func() error { _, err := RegisterStruct[contextDistinctStartEvent](env, "Start"); return err },
		"End":   func() error { _, err := RegisterStruct[contextDistinctEndEvent](env, "End"); return err },
		"Query": func() error { _, err := RegisterStruct[contextDistinctQueryEvent](env, "Query"); return err },
	} {
		if err := register(); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	if _, err := CreateDistinctInitiatedTerminatedContext(
		env,
		"cross-type-lifecycle",
		Field[contextDistinctStartEvent, string]("id"),
		Equal[string](Field[contextDistinctStartEvent, string]("kind"), Literal("start")),
		Equal[string](Field[contextDistinctEndEvent, string]("reason"), Literal("stop")),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(From[contextDistinctQueryEvent](env, "Query"),
		Alias("startID", Property[string](ContextInitiatingEvent(), "id")),
	).Query(
		StatementName("cross-type-lifecycle-statement"),
		WithContext("cross-type-lifecycle"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := env.NewEngine()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []Result
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		rows = append(rows, batch.New...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), contextDistinctStartEvent{ID: "A", Kind: "start"}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("cross-type partition after start = %d", statement.ContextPartitionCount())
	}
	if err := engine.SendEvent(context.Background(), contextDistinctQueryEvent{Value: 7}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("cross-type query rows = %#v", rows)
	}
	if err := engine.SendEvent(context.Background(), contextDistinctEndEvent{Reason: "stop"}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 0 {
		t.Fatalf("cross-type partition after termination = %d", statement.ContextPartitionCount())
	}
}

func TestNestedContextCombinesParentAndChildPartitions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "by-symbol", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	child, err := NewCategoryContext("price-band",
		Category("high", GreaterOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal[float64](10))),
		Category("low", Less[float64](Field[runtimeTestTrade, float64]("price"), Literal[float64](10))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "symbol-price", "by-symbol", child); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Window(LengthWindow(1)).Query(
		StatementName("nested-context"),
		WithContext("symbol-price"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(event runtimeTestTrade) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	send(runtimeTestTrade{Symbol: "A", Price: 1})
	send(runtimeTestTrade{Symbol: "A", Price: 10})
	send(runtimeTestTrade{Symbol: "B", Price: 1})
	if statement.ContextPartitionCount() != 3 {
		t.Fatalf("nested partition count = %d, keys=%v", statement.ContextPartitionCount(), statement.ContextPartitionKeys())
	}
	level, err := engine.ContextNestingLevel("symbol-price")
	if err != nil || level != 2 {
		t.Fatalf("nested context level = %d, err=%v", level, err)
	}
	selected := statement.ContextPartitionsWith(SelectNestedContextPartitions(
		SelectContextPartitionSegments([]any{"A"}),
		SelectContextPartitionCategories("high"),
	))
	if len(selected) != 1 {
		t.Fatalf("nested selector returned %#v", selected)
	}
	if value, ok := selected[0].Property("label"); !ok || value.Any() != "high" {
		t.Fatalf("nested selector returned wrong child: %#v", selected[0])
	}
	send(runtimeTestTrade{Symbol: "A", Price: 11})
	if len(batches) != 4 || len(batches[3].Old) != 1 {
		t.Fatalf("nested context batches = %#v", batches)
	}
	old, ok := batches[3].Old[0].Event()
	if !ok || old.Underlying().(runtimeTestTrade).Price != 10 {
		t.Fatalf("nested child old stream = %#v", batches[3].Old)
	}
}

func TestNestedInitiatedTerminatedChildUsesParentPartition(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "parent", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	child, err := NewInitiatedTerminatedContext(
		"child",
		Field[runtimeTestTrade, string]("symbol"),
		GreaterOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal[float64](10)),
		Less[float64](Field[runtimeTestTrade, float64]("price"), Literal[float64](0)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNestedContext(env, "nested-initiated", "parent", child); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade").Window(KeepAll()),
		Alias("parentKey", ContextField[string]("parent.key1")),
		Alias("childKey", ContextKeyValue[string](0)),
		Alias("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("nested-initiated"), WithContext("nested-initiated")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var rows []Row
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("nested initiated partition after child start = %d", statement.ContextPartitionCount())
	}
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 1 {
		t.Fatalf("nested initiated descriptors = %#v", descriptors)
	}
	if got, ok := descriptors[0].Property("parent.key1"); !ok || got.Any() != "A" {
		t.Fatalf("nested initiated parent property = %#v", descriptors[0].Properties())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 10}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("nested initiated partitions after second parent = %d", statement.ContextPartitionCount())
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: -1}); err != nil {
		t.Fatal(err)
	}
	if statement.ContextPartitionCount() != 1 {
		t.Fatalf("nested initiated partition after child termination = %d", statement.ContextPartitionCount())
	}
	if len(rows) != 3 || rows[0].Get("parentKey").Any() != "A" || rows[1].Get("parentKey").Any() != "B" || rows[2].Get("parentKey").Any() != "A" {
		t.Fatalf("nested initiated rows = %#v", rows)
	}
	temporal, err := NewTimePeriodContext("temporal", time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := NewKeyContext("parent-for-invalid", Literal("parent"))
	if err != nil {
		t.Fatal(err)
	}
	patternChild, err := NewPatternInitiatedContext("pattern-child", TimerAt(From[runtimeTestTrade](env, "Trade"), time.Unix(1, 0).UTC()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewNestedContext("nested-pattern-child", parent, patternChild); err == nil {
		t.Fatal("pattern initiated child was accepted in nested context")
	}
	if _, err := NewNestedContext("nested-temporal-child", parent, temporal); err == nil {
		t.Fatal("temporal child was accepted in nested context")
	}
	if _, err := NewNestedContext("nested-temporal-parent", temporal, parent); err == nil {
		t.Fatal("temporal parent was accepted in nested context")
	}
	initiatedParent, err := NewInitiatedTerminatedContext("initiated-parent", Literal("key"), Literal(true), Literal(false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewNestedContext("nested-initiated-parent", initiatedParent, parent); err == nil {
		t.Fatal("initiated parent was accepted in nested context")
	}
}

func TestContextSelectorFiltersFireAndForgetNamedWindowRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "live", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-symbol", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.InsertNamedWindow(context.Background(), "live", event); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := env.Build(FromNamedWindow(env, "live").Query(
		StatementName("context-faf"),
		WithContext("by-symbol"),
	))
	if err != nil {
		t.Fatal(err)
	}
	all, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil || len(all.Results()) != 2 {
		t.Fatalf("all context FAF results = %#v, err=%v", all.Results(), err)
	}
	keyA := encodeKey([]any{ValuePresent, "A"})
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitions(keyA))
	if err != nil || len(selected.Results()) != 1 {
		t.Fatalf("selected context FAF results = %#v, err=%v", selected.Results(), err)
	}
	event, ok := selected.Results()[0].Event()
	if !ok || event.Underlying().(runtimeTestTrade).Symbol != "A" {
		t.Fatalf("selected context FAF event = %#v", selected.Results()[0])
	}
	selectedByID, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitionIDs(0))
	if err != nil || len(selectedByID.Results()) != 1 {
		t.Fatalf("selected context FAF id results = %#v, err=%v", selectedByID.Results(), err)
	}
	event, ok = selectedByID.Results()[0].Event()
	if !ok || event.Underlying().(runtimeTestTrade).Symbol != "A" {
		t.Fatalf("selected context FAF id event = %#v", selectedByID.Results()[0])
	}
}

func TestCategoryContextSelectorFiltersFireAndForgetNamedWindowRows(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "category-live", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCategoryContext(env, "risk",
		Category("high", GreaterOrEqual[float64](Field[any, float64]("price"), Literal(10.0))),
		Category("low", Less[float64](Field[any, float64]("price"), Literal(10.0))),
	); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	for _, event := range []runtimeTestTrade{{Symbol: "high", Price: 20}, {Symbol: "low", Price: 2}} {
		if err := engine.InsertNamedWindow(context.Background(), "category-live", event); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := env.Build(FromNamedWindow(env, "category-live").Query(
		StatementName("category-context-faf"),
		WithContext("risk"),
	))
	if err != nil {
		t.Fatal(err)
	}
	all, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{})
	if err != nil || len(all.Results()) != 2 {
		t.Fatalf("all category context FAF results = %#v, err=%v", all.Results(), err)
	}
	selected, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitionCategories("high"))
	if err != nil || len(selected.Results()) != 1 {
		t.Fatalf("selected category context FAF results = %#v, err=%v", selected.Results(), err)
	}
	event, ok := selected.Results()[0].Event()
	if !ok || event.Underlying().(runtimeTestTrade).Price != 20 {
		t.Fatalf("selected category context FAF event = %#v", selected.Results()[0])
	}
}

func TestContextSelectorValidationRejectsMismatchedBuiltIns(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "selector-key", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateHashContext(env, "selector-hash", Field[runtimeTestTrade, string]("symbol"), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCategoryContext(env, "selector-category",
		Category("high", GreaterOrEqual[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
		Category("low", Less[float64](Field[runtimeTestTrade, float64]("price"), Literal(10.0))),
	); err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterContextVariable("selector-key", "counter", 0); err != nil {
		t.Fatal(err)
	}

	invalid := []struct {
		name     string
		context  string
		selector ContextPartitionSelector
	}{
		{name: "key-category", context: "selector-key", selector: SelectContextPartitionCategories("high")},
		{name: "key-nested", context: "selector-key", selector: SelectNestedContextPartitions(ContextPartitionSelectorAll{})},
		{name: "hash-category", context: "selector-hash", selector: SelectContextPartitionCategories("high")},
		{name: "category-hash", context: "selector-category", selector: SelectContextPartitionHashes(1)},
		{name: "category-key", context: "selector-category", selector: SelectContextPartitions("category:high")},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := engine.ContextPartitionDescriptors(test.context, test.selector); err == nil || !errors.Is(err, ErrorInvalidRule) {
				t.Fatalf("ContextPartitionDescriptors(%q, %T) error = %v, want InvalidRule", test.context, test.selector, err)
			}
		})
	}
	if _, err := engine.ContextVariableStates(context.Background(), "selector-key", SelectContextPartitionCategories("high"), "counter"); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("ContextVariableStates mismatched selector error = %v, want InvalidRule", err)
	}

	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "selector-live", schema); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromNamedWindow(env, "selector-live").Query(
		StatementName("selector-faf-invalid"),
		WithContext("selector-key"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, SelectContextPartitionCategories("high")); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("FAF mismatched selector error = %v, want InvalidRule", err)
	}

	livePlan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(
		StatementName("selector-live-context"),
		WithContext("selector-key"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), livePlan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 20}); err != nil {
		t.Fatal(err)
	}
	byKey, err := engine.ContextPartitionDescriptors("selector-key", SelectContextPartitions(encodeKey([]any{ValuePresent, "A"})))
	if err != nil || len(byKey) != 1 {
		t.Fatalf("valid key selector returned %#v, err=%v", byKey, err)
	}
}

func TestContextSnapshotWithSelectorFiltersIteratorState(t *testing.T) {
	env, engine := newRuntimeTest(t)
	if _, err := CreateKeyContext(env, "snapshot-selector", Field[runtimeTestTrade, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade").Window(KeepAll()),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
		Alias("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("snapshot-selector"), WithContext("snapshot-selector")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	descriptors := statement.ContextPartitions()
	if len(descriptors) != 2 {
		t.Fatalf("snapshot selector descriptors = %#v", descriptors)
	}
	var selectedDescriptor ContextPartitionDescriptor
	for _, descriptor := range descriptors {
		if value, ok := descriptor.Property("key1"); ok && value.Any() == "A" {
			selectedDescriptor = descriptor
			break
		}
	}
	if selectedDescriptor.Key == "" {
		t.Fatalf("did not find A descriptor in %#v", descriptors)
	}

	all, err := statement.Snapshot(context.Background())
	if err != nil || len(all.Results()) != 2 {
		t.Fatalf("all statement snapshot = %#v, err=%v", all.Results(), err)
	}
	selected, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitionIDs(selectedDescriptor.ID))
	if err != nil || len(selected.Results()) != 1 {
		t.Fatalf("ID-filtered statement snapshot = %#v, err=%v", selected.Results(), err)
	}
	row, ok := selected.Results()[0].Row()
	if !ok || row.Get("symbol").Any() != "A" {
		t.Fatalf("ID-filtered statement snapshot row = %#v", selected.Results()[0])
	}
	selected, err = statement.SnapshotWithSelector(context.Background(), SelectContextPartitions(selectedDescriptor.Key))
	if err != nil || len(selected.Results()) != 1 {
		t.Fatalf("key-filtered statement snapshot = %#v, err=%v", selected.Results(), err)
	}
	if _, err := statement.SnapshotWithSelector(context.Background(), SelectContextPartitionCategories("high")); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("mismatched statement snapshot selector error = %v, want InvalidRule", err)
	}

	nonContextPlan, err := env.Build(Select(
		From[runtimeTestTrade](env, "Trade").Window(KeepAll()),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("snapshot-selector-no-context")))
	if err != nil {
		t.Fatal(err)
	}
	nonContextDeployment, err := engine.Deploy(context.Background(), nonContextPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nonContextDeployment.Statements()[0].SnapshotWithSelector(context.Background(), SelectContextPartitionIDs(0)); err == nil || !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("non-context statement selector error = %v, want InvalidRule", err)
	}
}

func TestInitiatedTerminatedContextFireAndForgetIsRejected(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, ok := env.Schema("Trade")
	if !ok {
		t.Fatal("Trade schema is missing")
	}
	if _, err := CreateNamedWindow(env, "initiated-live", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateInitiatedTerminatedContext(
		env,
		"initiated-by-symbol",
		Field[any, string]("symbol"),
		Literal(true),
		Literal(false),
	); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromNamedWindow(env, "initiated-live").Query(
		StatementName("initiated-context-faf"),
		WithContext("initiated-by-symbol"),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.ExecuteFireAndForgetWithSelector(context.Background(), plan, ContextPartitionSelectorAll{}); err == nil {
		t.Fatal("initiated-terminated context FAF unexpectedly succeeded")
	}
}

func TestPatternInitiatedTerminatedContextCorrelatesCapturedTags(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	end := PatternFrom(base, "b", And(
		Equal[string](Field[runtimeTestTrade, string]("symbol"), TagField[string]("a", "symbol")),
		Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(100.0)),
	))
	if _, err := CreatePatternInitiatedTerminatedContext(env, "pattern-session", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("starter", ContextPatternField[string]("a", "symbol")),
		Alias("price", Field[runtimeTestTrade, float64]("price")),
	).Query(StatementName("pattern-session-statement"), WithContext("pattern-session")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			row, ok := result.Row()
			if ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, trade := range []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 101},
		{Symbol: "Z", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("pattern context rows = %d, want 3: %#v", len(rows), rows)
	}
	for index, row := range rows {
		if got := row.Get("starter").Any(); got != "A" {
			t.Fatalf("row %d starter = %#v, want A", index, got)
		}
	}
	descriptors, err := engine.ContextPartitionDescriptors("pattern-session", ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != 0 {
		t.Fatalf("terminated pattern context descriptors = %#v", descriptors)
	}
}

func TestPatternInitiatedContextEveryCreatesOverlappingPartitions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A"))).Every()
	if _, err := CreatePatternInitiatedContext(env, "pattern-overlap", start); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("starter", ContextPatternField[string]("a", "symbol")),
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-overlap-statement"), WithContext("pattern-overlap")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batchSizes []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batchSizes = append(batchSizes, len(batch.New))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "A", Price: 2},
		{Symbol: "Z", Price: 3},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := batchSizes, []int{1, 2, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pattern overlap batch sizes = %#v, want %#v", got, want)
	}
	descriptors, err := engine.ContextPartitionDescriptors("pattern-overlap", ContextPartitionSelectorAll{})
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != 2 || descriptors[0].BaseKey != descriptors[1].BaseKey || descriptors[0].Key == descriptors[1].Key {
		t.Fatalf("pattern overlap descriptors = %#v", descriptors)
	}
}

func TestPatternContextTerminationSnapshotCarriesEndTags(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	end := PatternFrom(base, "b", And(
		Equal[string](Field[runtimeTestTrade, string]("symbol"), TagField[string]("a", "symbol")),
		Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(100.0)),
	))
	if _, err := CreatePatternInitiatedTerminatedContext(env, "pattern-snapshot", start, end); err != nil {
		t.Fatal(err)
	}
	query := base.Aggregate(
		Alias("count", CountAll()),
		Alias("starter", ContextPatternField[string]("a", "symbol")),
		Alias("ender", ContextPatternField[string]("b", "symbol")),
		Alias("terminatingPrice", Property[float64](ContextTerminatingEvent(), "price")),
	).Query(
		StatementName("pattern-snapshot-statement"),
		WithContext("pattern-snapshot"),
		WithOutput(OutputSnapshotWhenTerminated()),
	)
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, trade := range []runtimeTestTrade{
		{Symbol: "A", Price: 1},
		{Symbol: "A", Price: 2},
		{Symbol: "A", Price: 101},
	} {
		if err := engine.SendEvent(context.Background(), trade); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("pattern termination snapshot batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok {
		t.Fatalf("pattern termination result is not a row: %#v", batches[0].New[0])
	}
	if got := row.Get("count").Any(); got != int64(2) {
		t.Fatalf("pattern termination snapshot count = %#v, want 2", got)
	}
	if got := row.Get("starter").Any(); got != "A" {
		t.Fatalf("pattern termination snapshot starter = %#v", got)
	}
	if got := row.Get("ender").Any(); got != "A" {
		t.Fatalf("pattern termination snapshot ender = %#v", got)
	}
	if got := row.Get("terminatingPrice").Any(); got != 101.0 {
		t.Fatalf("pattern termination snapshot terminatingPrice = %#v", got)
	}
}

func TestPatternTimerContextStartsAndTerminatesOnVirtualClock(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := TimerAt(base, time.Unix(1, 0).UTC())
	end := TimerInterval(base, time.Second)
	if _, err := CreatePatternInitiatedTerminatedContext(env, "pattern-timer", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(base.Aggregate(
		Alias("count", CountAll()),
	).Query(
		StatementName("pattern-timer-statement"),
		WithContext("pattern-timer"),
		WithOutput(OutputSnapshotWhenTerminated()),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-timer"); err != nil || count != 1 {
		t.Fatalf("timer context count = %d, err=%v", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("timer context emitted before termination: %#v", batches)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("timer context termination batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("count").Any() != int64(1) {
		t.Fatalf("timer context termination row = %#v", batches[0].New[0])
	}
	if count, err := engine.ContextPartitionCount("pattern-timer"); err != nil || count != 0 {
		t.Fatalf("timer context count after termination = %d, err=%v", count, err)
	}
}

func TestPatternTimerScheduleContextCreatesRecurringOverlappingPartitions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := TimerSchedule(base, time.Unix(1, 0).UTC(), time.Unix(2, 0).UTC())
	if _, err := CreatePatternInitiatedContext(env, "pattern-schedule", start); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-schedule-statement"), WithContext("pattern-schedule")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		sizes = append(sizes, len(batch.New))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-schedule"); err != nil || count != 2 {
		t.Fatalf("schedule context count = %d, err=%v", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "S", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sizes, []int{2}) {
		t.Fatalf("schedule context batch sizes = %#v, want [2]", sizes)
	}
}

func TestPatternTimerAtScheduleContextCreatesOnePartition(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	schedule := CronSchedule{
		Minute:     CronValues(20),
		Hour:       CronValues(17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}
	if _, err := CreatePatternInitiatedContext(env, "pattern-at-schedule-context", TimerAtSchedule(base, schedule)); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-at-schedule-context-statement"), WithContext("pattern-at-schedule-context")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), start.Add(10*time.Minute-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-at-schedule-context"); err != nil || count != 0 {
		t.Fatalf("one-shot schedule context started early: count=%d err=%v", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), start.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-at-schedule-context"); err != nil || count != 1 {
		t.Fatalf("one-shot schedule context count=%d err=%v, want 1", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), start.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-at-schedule-context"); err != nil || count != 1 {
		t.Fatalf("one-shot schedule context fired again: count=%d err=%v", count, err)
	}
	_ = deployment
}

func TestPatternTimerIntervalCalendarContextCreatesMonthlyPartitions(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	if _, err := CreateOverlappingPatternInitiatedContext(env, "pattern-calendar-timer-context", TimerIntervalCalendar(base, 0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-calendar-timer-context-statement"), WithContext("pattern-calendar-timer-context")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batchSizes []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batchSizes = append(batchSizes, len(batch.New))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	first := start.AddDate(0, 1, 0)
	second := first.AddDate(0, 1, 0)
	if err := engine.AdvanceTime(context.Background(), first.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-calendar-timer-context"); err != nil || count != 0 {
		t.Fatalf("calendar timer context created a partition early=%d err=%v", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-calendar-timer-context"); err != nil || count != 1 {
		t.Fatalf("calendar timer context first partition=%d err=%v, want 1", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), second.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-calendar-timer-context"); err != nil || count != 2 {
		t.Fatalf("calendar timer context partitions=%d err=%v, want 2", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "S"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batchSizes, []int{2}) {
		t.Fatalf("calendar timer context batch sizes=%#v, want [2]", batchSizes)
	}
}

func TestPatternTimerIntervalContextRequiresDeploymentParameters(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := TimerIntervalExpr(base, DurationSeconds[float64](Parameter[float64]("seconds")))
	if _, err := CreatePatternInitiatedContext(env, "pattern-parameter-timer-context", start); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-parameter-timer-context-statement"), WithContext("pattern-parameter-timer-context")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	if _, err := engine.Deploy(context.Background(), plan); err == nil {
		t.Fatal("context timer deployed without duration parameter")
	}
	deployment, err := engine.DeployWithParameters(context.Background(), plan, ParameterValues{"seconds": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	var batchSizes []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batchSizes = append(batchSizes, len(batch.New))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 999999999).UTC()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-parameter-timer-context"); err != nil || count != 0 {
		t.Fatalf("parameter timer context started early=%d err=%v", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-parameter-timer-context"); err != nil || count != 1 {
		t.Fatalf("parameter timer context partitions=%d err=%v, want 1", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "S"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batchSizes, []int{1}) {
		t.Fatalf("parameter timer context batch sizes=%#v, want [1]", batchSizes)
	}
}

func TestPatternTimerCronContextCreatesAllDueCalendarPartitions(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	schedule := CronSchedule{
		Minute:     CronEvery(15),
		Hour:       CronRange(8, 17),
		DayOfMonth: CronWildcard(),
		Month:      CronWildcard(),
		Weekday:    CronWildcard(),
	}
	start := time.Date(2008, time.February, 1, 17, 10, 0, 0, time.UTC)
	if _, err := CreatePatternInitiatedContext(env, "pattern-cron-context", TimerCron(base, schedule)); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-cron-context-statement"), WithContext("pattern-cron-context")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(start))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		sizes = append(sizes, len(batch.New))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Date(2008, time.February, 1, 17, 46, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-cron-context"); err != nil || count != 3 {
		t.Fatalf("cron context count = %d, err=%v", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sizes, []int{3}) {
		t.Fatalf("cron context batch sizes = %#v, want [3]", sizes)
	}
}

func TestPatternTimerContextComposesTimerAndEventLifecycle(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := TimerInterval(base, time.Second).FollowedBy("start", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A")))
	end := TimerInterval(base, time.Second).FollowedBy("end", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("E")))
	if _, err := CreatePatternInitiatedTerminatedContext(env, "pattern-composed-timer", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(base.Aggregate(
		Alias("count", CountAll()),
	).Query(
		StatementName("pattern-composed-timer-statement"),
		WithContext("pattern-composed-timer"),
		WithOutput(OutputSnapshotWhenTerminated()),
	))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var batches []ResultBatch
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-composed-timer"); err != nil || count != 0 {
		t.Fatalf("composed timer context started before event: count=%d err=%v", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-composed-timer"); err != nil || count != 1 {
		t.Fatalf("composed timer context count after start=%d err=%v", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "D", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 0 {
		t.Fatalf("composed timer context terminated before end event: %#v", batches)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].New) != 1 {
		t.Fatalf("composed timer context termination batches = %#v", batches)
	}
	row, ok := batches[0].New[0].Row()
	if !ok || row.Get("count").Any() != int64(2) {
		t.Fatalf("composed timer context termination row = %#v", batches[0].New[0])
	}
	if count, err := engine.ContextPartitionCount("pattern-composed-timer"); err != nil || count != 0 {
		t.Fatalf("composed timer context count after termination=%d err=%v", count, err)
	}
}

func TestPatternContextEveryDistinctExpiresOnVirtualClock(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Literal[bool](true)).EveryDistinctFor(
		time.Second,
		Field[runtimeTestTrade, string]("symbol"),
	)
	if _, err := CreateOverlappingPatternInitiatedContext(env, "pattern-distinct-expiry", start); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-distinct-expiry-statement"), WithContext("pattern-distinct-expiry")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-distinct-expiry"); err != nil || count != 1 {
		t.Fatalf("pattern distinct duplicate count=%d err=%v", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-distinct-expiry"); err != nil || count != 2 {
		t.Fatalf("pattern distinct expiry count=%d err=%v", count, err)
	}
}

func TestPatternContextEveryDistinctInsideWithinKeepsOnePartitionPerKey(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Literal[bool](true)).Within(10 * time.Second).EveryDistinct(
		Field[runtimeTestTrade, string]("symbol"),
	)
	if _, err := CreatePatternInitiatedContext(env, "pattern-inner-within-distinct", start); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-inner-within-distinct-statement"), WithContext("pattern-inner-within-distinct")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "A", "B", "A"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if count, err := engine.ContextPartitionCount("pattern-inner-within-distinct"); err != nil || count != 2 {
		t.Fatalf("inner within distinct context count=%d err=%v, want one partition per key", count, err)
	}
}

func TestPatternContextWithinOrMaxStopsOverlappingPartitions(t *testing.T) {
	env, engine := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Literal[bool](true)).Every().WithinOrMax(time.Second, 2)
	if _, err := CreateOverlappingPatternInitiatedContext(env, "pattern-within-or-max-context", start); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-within-or-max-context-statement"), WithContext("pattern-within-or-max-context")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"A", "B"} {
		if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: symbol}); err != nil {
			t.Fatal(err)
		}
	}
	if count, err := engine.ContextPartitionCount("pattern-within-or-max-context"); err != nil || count != 2 {
		t.Fatalf("within-or-max context count=%d err=%v, want 2", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "C"}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-within-or-max-context"); err != nil || count != 2 {
		t.Fatalf("within-or-max context restarted after guard=%d err=%v", count, err)
	}
}

func TestPatternContextWithinCalendarStopsNewPartitionsAtMonthBoundary(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	startPattern := PatternFrom(base, "a", Literal[bool](true)).Every().WithinCalendar(0, 1, 0)
	if _, err := CreateOverlappingPatternInitiatedContext(env, "pattern-calendar-context", startPattern); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(FromAny(env, "Trade").Select(
		Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
	).Query(StatementName("pattern-calendar-context-statement"), WithContext("pattern-calendar-context")))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2002, time.February, 1, 9, 0, 0, 0, time.UTC)
	engine := NewEngine(env, WithStartTime(start))
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1"}); err != nil {
		t.Fatal(err)
	}
	deadline := start.AddDate(0, 1, 0)
	if err := engine.AdvanceTime(context.Background(), deadline.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E2"}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-calendar-context"); err != nil || count != 2 {
		t.Fatalf("calendar context partitions before month boundary=%d err=%v", count, err)
	}
	if err := engine.AdvanceTime(context.Background(), deadline); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E3"}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("pattern-calendar-context"); err != nil || count != 2 {
		t.Fatalf("calendar context created a partition at month boundary=%d err=%v", count, err)
	}
}

func TestContextPartitionedNamedWindowConsumerRoutesByPartition(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	schema, _ := env.Schema("Trade")
	if _, err := CreateNamedWindow(env, "live", schema); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateKeyContext(env, "by-symbol", Field[any, string]("symbol")); err != nil {
		t.Fatal(err)
	}
	// Esper rejects data window views on named-window consumers, so the
	// consumer reads the keep-all window directly; partition routing is
	// observable through the per-symbol context partitions and new events.
	plan, err := env.Build(FromNamedWindow(env, "live").Query(
		StatementName("context-named-window"),
		WithContext("by-symbol"),
		WithOldStream(),
	))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	statement := deployment.Statements()[0]
	var batches []ResultBatch
	if _, err := statement.Subscribe(func(_ context.Context, batch ResultBatch) error {
		batches = append(batches, batch)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []runtimeTestTrade{{Symbol: "A", Price: 1}, {Symbol: "B", Price: 2}, {Symbol: "A", Price: 3}} {
		if err := engine.InsertNamedWindow(context.Background(), "live", event); err != nil {
			t.Fatal(err)
		}
	}
	if len(batches) != 3 || len(batches[2].New) != 1 {
		t.Fatalf("context named-window batches = %#v", batches)
	}
	current, ok := batches[2].New[0].Event()
	if !ok || current.Underlying().(runtimeTestTrade).Price != 3 {
		t.Fatalf("context named-window new event = %#v", batches[2].New)
	}
	if statement.ContextPartitionCount() != 2 {
		t.Fatalf("context named-window partitions = %d", statement.ContextPartitionCount())
	}
}

func TestTemporalContextOriginAnchorsAtDeployNotEpoch(t *testing.T) {
	// The virtual clock starts at the Unix epoch. Deploying a recurring
	// cron context after advancing the clock far ahead must anchor the
	// context at the deploy instant: anchoring at the epoch would force
	// cronWindow to step one cycle per second from 1970 to the deploy
	// time (a multi-billion-iteration hang for an every-second schedule).
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	everySecond := NewCronScheduleWithSeconds(CronWildcard(), CronWildcard(), CronWildcard(),
		CronWildcard(), CronWildcard(), CronWildcard())
	if _, err := CreateCronTimeContext(env, "every-second-anchor", everySecond, everySecond); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("s0"), WithContext("every-second-anchor")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	initial := time.Date(2002, time.May, 1, 8, 0, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := engine.Deploy(context.Background(), plan); err != nil {
			t.Errorf("deploy: %v", err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("every-second cron context deploy hung: origin was anchored at the epoch")
	}
	origin := engine.contextTemporalOrigins["every-second-anchor"]
	if !origin.Equal(initial) {
		t.Fatalf("temporal origin = %s, want deploy time %s", origin, initial)
	}
	count, err := engine.ContextPartitionCount("every-second-anchor")
	if err != nil || count != 1 {
		t.Fatalf("active every-second partition count = %d, err=%v", count, err)
	}
}

func TestDailyContextActivatesCurrentWindowWhenDeployedInsideIt(t *testing.T) {
	env, _ := newRuntimeTest(t)
	nine, _ := NewTimeOfDay(9, 0, 0)
	five, _ := NewTimeOfDay(17, 0, 0)
	if _, err := CreateDailyTimeContext(env, "business-window", nine, five); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[runtimeTestTrade](env, "Trade").Query(StatementName("s0"), WithContext("business-window")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	// Deploy at 09:15, inside the 09:00-17:00 window: Esper activates the
	// current cycle at deploy, so the partition must exist immediately.
	deployAt := time.Date(2024, time.May, 1, 9, 15, 0, 0, time.UTC)
	if err := engine.AdvanceTime(context.Background(), deployAt); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	count, err := engine.ContextPartitionCount("business-window")
	if err != nil || count != 1 {
		t.Fatalf("daily context partition count inside window = %d, err=%v, want 1", count, err)
	}
}

func TestPatternContextStartPatternRearmsAfterCompletion(t *testing.T) {
	// A non-every start pattern `a=A -> timer:interval(1 sec)` completes
	// when its timer fires and the context condition must re-arm: a later
	// A event starts a fresh instance, and a completion that overlaps an
	// active partition is discarded (Esper's repeatable condition).
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	isStart := Or(
		Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A1")),
		Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("A3")),
	)
	start := PatternFrom(base, "a", isStart).Then(TimerInterval(base, time.Second))
	end := PatternFrom(base, "b", Equal[string](Field[runtimeTestTrade, string]("symbol"), Literal("B"))).Then(TimerInterval(base, time.Second))
	if _, err := CreatePatternInitiatedTerminatedContext(env, "rearming-context", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(base.Window(KeepAll()).Aggregate(
		Alias("c1", ContextPatternField[string]("a", "symbol")),
		Alias("c2", Sum[int](Field[runtimeTestTrade, int]("price"))),
	).Query(StatementName("s0"), WithContext("rearming-context")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A1", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E1", Price: 3}); err != nil {
		t.Fatal(err)
	}
	// A2's start completes at t=2 while A1's partition is still active and
	// is discarded; B1's end completes at t=2 and terminates A1.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A2", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(2, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E2", Price: 4}); err != nil {
		t.Fatal(err)
	}
	// A3 after A1's termination re-arms the condition and starts a fresh
	// partition at t=3.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A3", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(3, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E3", Price: 5}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "E4", Price: 6}); err != nil {
		t.Fatal(err)
	}
	count, err := engine.ContextPartitionCount("rearming-context")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("re-armed pattern context partition count = %d, want 1", count)
	}
	// A1's partition aggregates every Trade event while active (E1=3,
	// A2=1, B=1); A2's overlapping start completion is discarded and never
	// opens a partition. A3 re-arms the condition after A1's termination
	// and its partition aggregates E3=5 and E4=6.
	want := []struct {
		symbol string
		sum    float64
	}{
		{"A1", 3}, {"A1", 4}, {"A1", 5}, {"A3", 5}, {"A3", 11},
	}
	if len(rows) != len(want) {
		t.Fatalf("re-armed pattern rows = %#v", rows)
	}
	for index, expected := range want {
		got, ok := rows[index].Get("c1").Any().(string)
		sum, _ := numericValue(rows[index].Get("c2"))
		if !ok || got != expected.symbol || sum != expected.sum {
			t.Fatalf("re-armed pattern row %d = %#v, want %s/%v", index, rows[index].AsMap(), expected.symbol, expected.sum)
		}
	}
}

func TestPatternContextDuplicateStartEndTagRejected(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Literal[bool](true))
	end := PatternFrom(base, "a", Literal[bool](true))
	if _, err := NewPatternInitiatedTerminatedContext("duplicate-tag", start, end); err == nil {
		t.Fatal("duplicate start/end pattern tag was accepted")
	} else if !errors.Is(err, ErrorInvalidRule) {
		t.Fatalf("duplicate tag error = %v, want %v", err, ErrorInvalidRule)
	}
	// Distinct tags remain valid.
	if _, err := NewPatternInitiatedTerminatedContext(
		"distinct-tags", PatternFrom(base, "a", Literal[bool](true)), PatternFrom(base, "b", Literal[bool](true))); err != nil {
		t.Fatalf("distinct start/end tags rejected: %v", err)
	}
}

func TestPatternStartFilterEndMixedContextTerminatesOnCorrelatedFilter(t *testing.T) {
	env, _ := newRuntimeTest(t)
	base := From[runtimeTestTrade](env, "Trade")
	start := PatternFrom(base, "a", Literal[bool](true))
	isTrade := Equal[string](TypeName(EventValue[Event]()), Literal("Trade"))
	// The end filter terminates only a same-symbol event whose price is
	// above the threshold, so the starting event never ends itself.
	end := And(
		And(isTrade,
			Equal[string](Field[runtimeTestTrade, string]("symbol"), Property[string](ContextInitiatingEvent(), "symbol"))),
		Greater[float64](Field[runtimeTestTrade, float64]("price"), Literal(1.0)))
	if _, err := CreatePatternInitiatedTerminatedByFilterContext(env, "mixed-filter-end", start, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(base.Query(StatementName("s0"), WithContext("mixed-filter-end")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	// A(1) starts the partition; A(2) terminates it via the correlated filter.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("mixed-filter-end"); err != nil || count != 1 {
		t.Fatalf("partition count after start = %d, err=%v", count, err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 2}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("mixed-filter-end"); err != nil || count != 0 {
		t.Fatalf("partition count after end = %d, err=%v", count, err)
	}
	// A different symbol never terminates; a same-symbol low-price event
	// does not terminate either.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 1}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.ContextPartitionCount("mixed-filter-end"); err != nil || count != 1 {
		t.Fatalf("partition count after non-matching end = %d, err=%v", count, err)
	}
}

type timerEndTrigger struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

func TestFilterStartPatternEndTimerTerminatesWithOutput(t *testing.T) {
	env, _ := newRuntimeTest(t)
	if _, err := RegisterStruct[timerEndTrigger](env, "Trigger"); err != nil {
		t.Fatal(err)
	}
	base := From[runtimeTestTrade](env, "Trade")
	triggerBase := From[timerEndTrigger](env, "Trigger")
	isTrade := Equal[string](TypeName(EventValue[Event]()), Literal("Trade"))
	// The end pattern consumes Trigger events only, so the Trade start
	// event never terminates its own partition; the timer branch is a
	// second termination path.
	end := PatternFrom(triggerBase, "b",
		Equal[string](Field[timerEndTrigger, string]("symbol"), Property[string](ContextInitiatingEvent(), "symbol")),
	).Or(TimerInterval(triggerBase, 10*time.Second))
	if _, err := CreatePatternTerminatedContext(env, "mixed-timer-end", Literal("global"), isTrade, end); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(base.Aggregate(
		Alias("c1", Property[string](ContextInitiatingEvent(), "symbol")),
		Alias("c2", ContextPatternField[string]("b", "symbol")),
	).Query(StatementName("s0"), WithContext("mixed-timer-end"),
		WithOutput(OutputWhenTerminated())))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var rows []Row
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if row, ok := result.Row(); ok {
				rows = append(rows, row)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "A", Price: 1}); err != nil {
		t.Fatal(err)
	}
	// Correlated filter end: a matching Trigger terminates immediately.
	if err := engine.SendEvent(context.Background(), timerEndTrigger{Symbol: "A"}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("termination rows after filter end = %d, want 1", len(rows))
	}
	if rows[0].Get("c1").Any() != "A" || rows[0].Get("c2").Any() != "A" {
		t.Fatalf("filter-end row = %#v", rows[0].AsMap())
	}
	// Timer end: the second partition terminates at the 10-second deadline
	// with no captured end tag.
	if err := engine.SendEvent(context.Background(), runtimeTestTrade{Symbol: "B", Price: 3}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("termination rows after timer end = %d, want 2", len(rows))
	}
	if rows[1].Get("c1").Any() != "B" || rows[1].Get("c2").IsPresent() {
		t.Fatalf("timer-end row = %#v", rows[1].AsMap())
	}
}
