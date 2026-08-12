package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	clientInstrumentAuditDocRuntimeID = "java-runtime-664276ef5b76026debd7"
	clientInstrumentAuditRuntimeID    = "java-runtime-b836e76e45c52c47b969"
)

type clientInstrumentAuditBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func (b clientInstrumentAuditBean) String() string {
	return fmt.Sprintf("SupportBean(%s, %d)", b.TheString, b.IntPrimitive)
}

type clientInstrumentAuditContextEvent struct {
	ID   string `esper:"id"`
	Kind string `esper:"kind"`
}

func newClientInstrumentAuditEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[clientInstrumentAuditBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientInstrumentAuditContextEvent](env, "AuditContextEvent"); err != nil {
		t.Fatal(err)
	}
	return env
}

func collectAudit(t *testing.T, engine *Engine) (*[]AuditRecord, *AuditSubscription) {
	t.Helper()
	records := make([]AuditRecord, 0)
	subscription, err := engine.SubscribeAudit(func(_ context.Context, record AuditRecord) error {
		// Re-entering Engine APIs proves callbacks are delivered outside the
		// transaction lock instead of through Java's process-global AuditPath.
		_ = engine.Now()
		_ = engine.RuntimeURI()
		records = append(records, record)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return &records, subscription
}

func TestClientInstrumentAuditDocSampleParity(t *testing.T) {
	env := newClientInstrumentAuditEnv(t)
	orderSchema, err := NewMapSchema("OrderEvent", []FieldSpec{FieldDef("price", reflect.TypeOf(float64(0)))})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(orderSchema); err != nil {
		t.Fatal(err)
	}
	query := Select(
		From[map[string]any](env, "OrderEvent"),
		Alias("price", Field[map[string]any, float64]("price")),
	).Query(StatementName("All-Order-Events"), StatementAudit(AuditStream, AuditProperty))
	plan, err := env.Build(query)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithRuntimeURI(clientInstrumentAuditDocRuntimeID))
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	records, subscription := collectAudit(t, engine)
	defer func() { _ = subscription.Close() }()
	if err := engine.Send(context.Background(), "OrderEvent", map[string]any{"price": 100.0}); err != nil {
		t.Fatal(err)
	}
	if len(*records) != 2 || (*records)[0].Category() != AuditStream || (*records)[1].Category() != AuditProperty {
		t.Fatalf("doc-sample audit records = %#v", *records)
	}
	if (*records)[1].Message() != "price value 100" {
		t.Fatalf("doc-sample property message = %q", (*records)[1].Message())
	}
}

func TestClientInstrumentAuditCallbackParity(t *testing.T) {
	origin := time.Unix(0, int64(time.Millisecond)).UTC()
	env := newClientInstrumentAuditEnv(t)
	theString := Field[clientInstrumentAuditBean, string]("theString")
	plan, err := env.Build(From[clientInstrumentAuditBean](env, "SupportBean").
		Filter(Equal[string](theString, Literal("E1"))).
		Query(StatementName("ABC"), StatementAudit(AuditStream)))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(origin), WithRuntimeURI("default"))
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	records, subscription := collectAudit(t, engine)
	defer func() { _ = subscription.Close() }()
	if err := engine.SendEvent(context.Background(), clientInstrumentAuditBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if len(*records) != 1 {
		t.Fatalf("stream callback count = %d, records=%#v", len(*records), *records)
	}
	record := (*records)[0]
	if record.Message() != "SupportBean(theString=...) inserted SupportBean[SupportBean(E1, 1)]" ||
		record.DeploymentID() != deployment.ID() || record.StatementName() != "ABC" ||
		record.RuntimeURI() != "default" || record.Category() != AuditStream || record.RuntimeTime() != 1 || record.AgentInstanceID() != -1 {
		t.Fatalf("stream callback = %#v, formatted=%q", record, record.Format())
	}
	formatter := NewAuditFormatter("[%u] [%d] [%s] [%i] [%c] [%tutc] [%tzone] %m", time.UTC)
	formatted := formatter.Format(record)
	for _, fragment := range []string{"[default]", "[" + deployment.ID() + "]", "[ABC]", "[-1]", "[STREAM]", "1970-01-01T00:00:00.001Z", record.Message()} {
		if !strings.Contains(formatted, fragment) {
			t.Fatalf("formatted audit %q lacks %q", formatted, fragment)
		}
	}
}

func TestClientInstrumentAuditAllCategoriesParity(t *testing.T) {
	origin := time.Unix(0, 0).UTC()
	env := newClientInstrumentAuditEnv(t)
	intField := Field[clientInstrumentAuditBean, int]("intPrimitive")
	if err := DefineExpression[int](env, "audit-double", Multiply[int](intField, Literal(2))); err != nil {
		t.Fatal(err)
	}
	outSchema, err := NewMapSchema("AuditOut", []FieldSpec{FieldDef("value", reflect.TypeOf(0))})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.RegisterSchema(outSchema); err != nil {
		t.Fatal(err)
	}
	expression := Add[int](ExpressionRef[int](env, "audit-double"), Literal(1))
	mainPlan, err := env.Build(Select(
		From[clientInstrumentAuditBean](env, "SupportBean").Window(TimeWindow(time.Second)),
		Alias("value", expression),
	).InsertInto("AuditOut", StatementName("audit-main"), StatementAudit()))
	if err != nil {
		t.Fatal(err)
	}

	pattern := PatternFrom(From[clientInstrumentAuditBean](env, "SupportBean"), "a", Equal[string](Field[clientInstrumentAuditBean, string]("theString"), Literal("P1"))).
		FollowedBy("b", Equal[string](Field[clientInstrumentAuditBean, string]("theString"), Literal("P2")))
	patternPlan, err := env.Build(pattern.Select(Alias("value", TagField[int]("a", "intPrimitive"))).
		Query(StatementName("audit-pattern"), StatementAudit(AuditPattern, AuditPatternInstances)))
	if err != nil {
		t.Fatal(err)
	}

	id := Field[clientInstrumentAuditContextEvent, string]("id")
	kind := Field[clientInstrumentAuditContextEvent, string]("kind")
	if _, err := CreateInitiatedTerminatedContext(env, "WhenEventArrives", id,
		Equal[string](kind, Literal("start")), Equal[string](kind, Literal("end"))); err != nil {
		t.Fatal(err)
	}
	contextPlan, err := env.Build(From[clientInstrumentAuditContextEvent](env, "AuditContextEvent").Query(
		StatementName("audit-context"), WithContext("WhenEventArrives"), StatementAudit(AuditContextPartition)))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(origin), WithRuntimeURI(clientInstrumentAuditRuntimeID))
	if _, err := engine.DeployPlans(context.Background(), []Plan{mainPlan, patternPlan, contextPlan}); err != nil {
		t.Fatal(err)
	}
	records, subscription := collectAudit(t, engine)
	defer func() { _ = subscription.Close() }()

	if err := engine.SendEvent(context.Background(), clientInstrumentAuditBean{TheString: "E1", IntPrimitive: 50}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), origin.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, event := range []clientInstrumentAuditBean{{TheString: "P1", IntPrimitive: 1}, {TheString: "P2", IntPrimitive: 2}} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentAuditContextEvent{ID: "A", Kind: "start"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentAuditContextEvent{ID: "A", Kind: "end"}); err != nil {
		t.Fatal(err)
	}

	definition, err := DefineDataflow(env, "MyFlow").
		Audit().
		BeaconSource("source", "I1").
		LogSink("sink", func(context.Context, any) error { return nil }).
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	seen := make(map[AuditCategory]bool)
	for _, record := range *records {
		seen[record.Category()] = true
	}
	for _, category := range allAuditCategories {
		if !seen[category] {
			t.Fatalf("missing audit category %s; seen=%v records=%#v", category, seen, *records)
		}
	}
	if metadata := mainPlan.Query().Metadata(); len(metadata.AuditCategories) != len(allAuditCategories) {
		t.Fatalf("statement audit metadata = %#v", metadata.AuditCategories)
	}
	if len(definition.AuditCategories()) != len(allAuditCategories) {
		t.Fatalf("dataflow audit metadata = %#v", definition.AuditCategories())
	}
}

func TestAuditConfigurationAndSubscriptionLifecycle(t *testing.T) {
	env := newClientInstrumentAuditEnv(t)
	invalid := AuditCategory("not-a-category")
	if _, err := env.Build(From[clientInstrumentAuditBean](env, "SupportBean").Query(StatementAudit(invalid))); err == nil || !strings.Contains(err.Error(), "audit category") {
		t.Fatalf("invalid statement audit category error = %v", err)
	}
	if _, err := DefineDataflow(env, "invalid-audit-flow").Audit(invalid).Emitter("sink").Build(); err == nil || !strings.Contains(err.Error(), "audit category") {
		t.Fatalf("invalid dataflow audit category error = %v", err)
	}

	plan, err := env.Build(From[clientInstrumentAuditBean](env, "SupportBean").Query(
		StatementName("subscription"), StatementAudit(AuditStream)))
	if err != nil {
		t.Fatal(err)
	}
	metadata := plan.Query().Metadata()
	metadata.AuditCategories[0] = AuditProperty
	if plan.Query().Metadata().AuditCategories[0] != AuditStream {
		t.Fatal("statement audit metadata aliases the immutable plan")
	}
	engine := NewEngine(env)
	if _, err := engine.SubscribeAudit(nil); err == nil {
		t.Fatal("nil audit listener was accepted")
	}
	if _, err := engine.Deploy(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	count := 0
	subscription, err := engine.SubscribeAudit(func(context.Context, AuditRecord) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), clientInstrumentAuditBean{TheString: "E1", IntPrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("closed audit subscription received %d records", count)
	}
}
