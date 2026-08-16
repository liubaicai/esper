package esper

import (
	"context"
	"testing"
)

type keySegTermEventSelectBean struct {
	UserID *string `esper:"userId"`
	Alert  *string `esper:"alert"`
}

func stringPtr(value string) *string { return &value }

// TestKeyContextTermEventSelectProjectionParity locks the term-event
// projection of a keyed initiated-terminated context verified against
// ContextKeySegmentedTermEventSelect: `partition by userId initiated by
// UserEvent(alert='A') terminated by UserEvent(alert='B') as termEvent`
// with `select *, context.termEvent as term from UserEvent#firstevent
// output snapshot when terminated`. The snapshot observes the state before
// the terminating event (the firstevent window still holds the initiating
// event) while the term column carries the terminating event. Nullable
// partition keys must partition by value: pointer-typed fields share a
// partition across events.
func TestKeyContextTermEventSelectProjectionParity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[keySegTermEventSelectBean](env, "UserEvent"); err != nil {
		t.Fatal(err)
	}
	source := From[keySegTermEventSelectBean](env, "UserEvent")
	userID := Field[keySegTermEventSelectBean, *string]("userId")
	alert := Field[keySegTermEventSelectBean, *string]("alert")
	if _, err := CreateInitiatedTerminatedContext(env, "UserSessionContext", userID,
		Equal[*string](alert, Literal("A")),
		Equal[*string](alert, Literal("B"))); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(Select(source.Window(FirstEvent()),
		Alias("userId", userID),
		Alias("alert", alert),
		Alias("term", ContextTerminatingEvent()),
	).Query(StatementName("s0"), WithContext("UserSessionContext"), WithOutput(OutputSnapshotWhenTerminated())))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
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
	send := func(user, alert string) {
		t.Helper()
		var alertValue *string
		if alert != "" {
			alertValue = stringPtr(alert)
		}
		if err := engine.SendEvent(context.Background(), keySegTermEventSelectBean{UserID: stringPtr(user), Alert: alertValue}); err != nil {
			t.Fatal(err)
		}
	}
	termAlert := func(row Row) string {
		event, ok := row.Get("term").Any().(Event)
		if !ok {
			t.Fatalf("term not an event: %#v", row.Get("term").Any())
		}
		value := event.Get("alert").Any()
		if value == nil {
			return ""
		}
		return *value.(*string)
	}

	send("U1", "A")
	send("U1", "") // mid null-alert events: no output
	send("U1", "")
	if len(rows) != 0 {
		t.Fatalf("before termination rows = %d, want 0", len(rows))
	}
	send("U1", "B")
	if len(rows) != 1 {
		t.Fatalf("after U1 termination rows = %d, want 1", len(rows))
	}
	if alert, ok := rows[0].Get("alert").Any().(*string); !ok || *alert != "A" {
		t.Fatalf("snapshot alert = %#v, want A (window before terminating event)", rows[0].Get("alert").Any())
	}
	if termAlert(rows[0]) != "B" {
		t.Fatalf("term alert = %q, want B", termAlert(rows[0]))
	}
	send("U2", "A")
	send("U2", "B")
	if len(rows) != 2 {
		t.Fatalf("after U2 termination rows = %d, want 2", len(rows))
	}
	if termAlert(rows[1]) != "B" {
		t.Fatalf("U2 term alert = %q, want B", termAlert(rows[1]))
	}
	if user, ok := rows[1].Get("userId").Any().(*string); !ok || *user != "U2" {
		t.Fatalf("U2 snapshot userId = %#v, want U2", rows[1].Get("userId").Any())
	}
}
