package esper

import (
	"context"
	"testing"
)

type joinUniqueIndexOne struct {
	S string  `esper:"s1"`
	I int     `esper:"i1"`
	D float64 `esper:"d1"`
	L int64   `esper:"l1"`
}

type joinUniqueIndexTwo struct {
	S string  `esper:"s2"`
	I int     `esper:"i2"`
	D float64 `esper:"d2"`
	L int64   `esper:"l2"`
}

type joinUniqueIndexThird struct {
	Name string `esper:"name"`
}

func TestJoinUniqueIndexRetainsLatestCompositeKeyAcrossDriverVariants(t *testing.T) {
	cases := []struct {
		name           string
		unidirectional bool
		threeStream    bool
		reverseKeys    bool
		includeLong    bool
	}{
		{name: "last-event-two-stream"},
		{name: "unidirectional-two-stream", unidirectional: true},
		{name: "last-event-three-stream", threeStream: true},
		{name: "unidirectional-three-stream", unidirectional: true, threeStream: true},
		{name: "reverse-key-condition", reverseKeys: true},
		{name: "additional-equality-condition", includeLong: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[joinUniqueIndexOne](env, "JoinUniqueIndexOne"); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[joinUniqueIndexTwo](env, "JoinUniqueIndexTwo"); err != nil {
				t.Fatal(err)
			}
			if testCase.threeStream {
				if _, err := RegisterStruct[joinUniqueIndexThird](env, "JoinUniqueIndexThird"); err != nil {
					t.Fatal(err)
				}
			}

			leftStream := From[joinUniqueIndexOne](env, "JoinUniqueIndexOne")
			leftInput := JoinSource(leftStream)
			if testCase.unidirectional {
				leftInput = leftInput.Unidirectional()
			} else {
				leftInput = JoinSource(leftStream.Window(LastEvent()))
			}
			rightInput := JoinSource(From[joinUniqueIndexTwo](env, "JoinUniqueIndexTwo").Window(
				UniqueBy(
					Field[joinUniqueIndexTwo, float64]("d2"),
					Field[joinUniqueIndexTwo, int]("i2"),
				),
			))
			inputs := []JoinInput{leftInput, rightInput}
			if testCase.threeStream {
				inputs = append(inputs, JoinSource(From[joinUniqueIndexThird](env, "JoinUniqueIndexThird").Window(LastEvent())))
			}

			dCondition := OnSourcesEqual(
				0, Field[joinUniqueIndexOne, float64]("d1"),
				1, Field[joinUniqueIndexTwo, float64]("d2"),
			)
			iCondition := OnSourcesEqual(
				0, Field[joinUniqueIndexOne, int]("i1"),
				1, Field[joinUniqueIndexTwo, int]("i2"),
			)
			conditions := []JoinCondition{dCondition, iCondition}
			if testCase.reverseKeys {
				conditions = []JoinCondition{iCondition, dCondition}
			}
			if testCase.includeLong {
				conditions = append(conditions, OnSourcesEqual(
					0, Field[joinUniqueIndexOne, int64]("l1"),
					1, Field[joinUniqueIndexTwo, int64]("l2"),
				))
			}
			selections := []JoinSelection{
				SelectFrom(0, "left", Field[joinUniqueIndexOne, string]("s1")),
				SelectFrom(1, "right", Field[joinUniqueIndexTwo, string]("s2")),
			}
			if testCase.threeStream {
				selections = append(selections, SelectFrom(2, "third", Field[joinUniqueIndexThird, string]("name")))
			}
			query := JoinMany(inputs...).On(conditions...).Select(selections...).Query(
				StatementName("join-unique-index-" + testCase.name),
			)
			plan, err := env.Build(query)
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
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

			if testCase.threeStream {
				if err := engine.SendEvent(context.Background(), joinUniqueIndexThird{Name: "JOINEVENT"}); err != nil {
					t.Fatal(err)
				}
			}
			sendRight := func(event joinUniqueIndexTwo) {
				t.Helper()
				if err := engine.SendEvent(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			sendRight(joinUniqueIndexTwo{S: "E1", I: 1, D: 3, L: 10})
			sendRight(joinUniqueIndexTwo{S: "E2", I: 1, D: 2, L: 0})
			sendRight(joinUniqueIndexTwo{S: "E3", I: 1, D: 3, L: 9})
			if len(rows) != 0 {
				t.Fatalf("unique join emitted before driver = %#v", rows)
			}
			if err := engine.SendEvent(context.Background(), joinUniqueIndexOne{S: "EX", I: 1, D: 3, L: 9}); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 {
				t.Fatalf("unique join rows = %#v", rows)
			}
			if rows[0].Get("left").Any() != "EX" || rows[0].Get("right").Any() != "E3" {
				t.Fatalf("unique join row = %#v", rows[0].AsMap())
			}
			if testCase.threeStream && rows[0].Get("third").Any() != "JOINEVENT" {
				t.Fatalf("unique three-stream row = %#v", rows[0].AsMap())
			}
		})
	}
}
