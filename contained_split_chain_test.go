package esper

import (
	"context"
	"strings"
	"testing"
)

type containedSplitChainInput struct {
	Sentence string `esper:"sentence"`
}

type containedSplitChainWord struct {
	Word string `esper:"word"`
}

type containedSplitChainCharacter struct {
	Character string `esper:"character"`
}

func TestContainedSplitChainPreservesNestedParentScopeMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[containedSplitChainInput](env, "ContainedSplitChainInput"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitChainWord](env, "ContainedSplitChainWord"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[containedSplitChainCharacter](env, "ContainedSplitChainCharacter"); err != nil {
		t.Fatal(err)
	}

	input := From[containedSplitChainInput](env, "ContainedSplitChainInput")
	splitSentence := Func1[string, []map[string]any]("split-sentence", func(sentence string) []map[string]any {
		words := strings.Fields(sentence)
		result := make([]map[string]any, 0, len(words))
		for _, word := range words {
			result = append(result, map[string]any{"word": word})
		}
		return result
	}, Field[containedSplitChainInput, string]("sentence"))
	words := UnnestAs[containedSplitChainInput, map[string]any](input, splitSentence, "ContainedSplitChainWord")
	splitWord := Func1[string, []map[string]any]("split-word", func(word string) []map[string]any {
		characters := []rune(word)
		result := make([]map[string]any, 0, len(characters))
		for _, character := range characters {
			result = append(result, map[string]any{"character": string(character)})
		}
		return result
	}, Field[Event, string]("word"))
	characters := UnnestAs[Event, map[string]any](words, splitWord, "ContainedSplitChainCharacter")

	plan, err := env.Build(Select(characters,
		Alias("character", Field[Event, string]("character")),
		Alias("word", ContainedParentField[string]("word")),
		Alias("sentence", ContainedAncestorField[string](2, "sentence")),
		Alias("parentEvent", ContainedParentEvent()),
		Alias("ancestorEvent", ContainedAncestorEvent(2)),
	).Query(StatementName("contained-split-chain")))
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
			row, ok := result.Row()
			if !ok {
				t.Fatalf("contained split chain result is not a row: %#v", result)
			}
			rows = append(rows, row)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), containedSplitChainInput{Sentence: "Go 汉字"}); err != nil {
		t.Fatal(err)
	}

	if len(rows) != 4 {
		t.Fatalf("contained split chain rows = %#v, want four characters", rows)
	}
	want := []struct {
		character string
		word      string
	}{
		{"G", "Go"},
		{"o", "Go"},
		{"汉", "汉字"},
		{"字", "汉字"},
	}
	for index, expected := range want {
		if rows[index].Get("character").Any() != expected.character || rows[index].Get("word").Any() != expected.word || rows[index].Get("sentence").Any() != "Go 汉字" {
			t.Fatalf("contained split chain row %d = %#v, want character=%q word=%q sentence=%q", index, rows[index], expected.character, expected.word, "Go 汉字")
		}
		parent, ok := rows[index].Get("parentEvent").Any().(Event)
		if !ok || parent.TypeName() != "ContainedSplitChainWord" || parent.Get("word").Any() != expected.word {
			t.Fatalf("contained split chain parent %d = %#v, want word=%q", index, rows[index].Get("parentEvent"), expected.word)
		}
		ancestor, ok := rows[index].Get("ancestorEvent").Any().(Event)
		if !ok || ancestor.TypeName() != "ContainedSplitChainInput" || ancestor.Get("sentence").Any() != "Go 汉字" {
			t.Fatalf("contained split chain ancestor %d = %#v", index, rows[index].Get("ancestorEvent"))
		}
	}
}
