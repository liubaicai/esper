package esper

import (
	"context"
	"iter"
	"testing"
)

type containedSplitSequenceInput struct {
	Rows   iter.Seq[map[string]any] `esper:"rows"`
	Events iter.Seq[Event]          `esper:"events"`
}

type containedSplitSequenceWord struct {
	Word string `esper:"word"`
}

func TestContainedSplitSequenceMaterializesGoIterSeqInOrder(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitSequenceInput](env, "ContainedSplitSequenceInput"); err != nil {
		t.Fatal(err)
	}
	wordSchema, err := RegisterStruct[containedSplitSequenceWord](env, "ContainedSplitSequenceWord")
	if err != nil {
		t.Fatal(err)
	}
	input := From[containedSplitSequenceInput](env, "ContainedSplitSequenceInput")
	rows := UnnestSeqAs[containedSplitSequenceInput, map[string]any](
		input,
		Property[iter.Seq[map[string]any]](EventValue[containedSplitSequenceInput](), "rows"),
		"ContainedSplitSequenceWord",
	)
	events := UnnestSeqEvents(
		input,
		Property[iter.Seq[Event]](EventValue[containedSplitSequenceInput](), "events"),
		"ContainedSplitSequenceWord",
	)
	rowsPlan, err := env.Build(Select(rows,
		Alias("word", Field[Event, string]("word")),
		Alias("parentRows", ContainedParentField[iter.Seq[map[string]any]]("rows")),
	).Query(StatementName("contained-split-sequence-rows")))
	if err != nil {
		t.Fatal(err)
	}
	eventsPlan, err := env.Build(Select(events,
		Alias("word", Field[Event, string]("word")),
	).Query(StatementName("contained-split-sequence-events")))
	if err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env)
	rowResults := subscribeRows(t, engine, rowsPlan)
	eventResults := subscribeRows(t, engine, eventsPlan)
	rowSequence := iter.Seq[map[string]any](func(yield func(map[string]any) bool) {
		for _, value := range []string{"seq-one", "seq-two", "seq-three"} {
			if !yield(map[string]any{"word": value}) {
				return
			}
		}
	})
	eventOne, err := NewEvent(wordSchema, containedSplitSequenceWord{Word: "event-one"}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	eventTwo, err := NewEvent(wordSchema, containedSplitSequenceWord{Word: "event-two"}, engine.Now())
	if err != nil {
		t.Fatal(err)
	}
	eventSequence := iter.Seq[Event](func(yield func(Event) bool) {
		if !yield(eventOne) {
			return
		}
		yield(eventTwo)
	})
	if err := engine.SendEvent(context.Background(), containedSplitSequenceInput{Rows: rowSequence, Events: eventSequence}); err != nil {
		t.Fatal(err)
	}

	assertSequenceRows(t, rowResults(), []string{"seq-one", "seq-two", "seq-three"})
	assertSequenceRows(t, eventResults(), []string{"event-one", "event-two"})
}

func assertSequenceRows(t *testing.T, rows []Row, want []string) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("contained sequence rows = %#v, want %d", rows, len(want))
	}
	for index, expected := range want {
		if got := rows[index].Get("word").Any(); got != expected {
			t.Fatalf("contained sequence row %d word = %#v, want %q", index, got, expected)
		}
	}
}
