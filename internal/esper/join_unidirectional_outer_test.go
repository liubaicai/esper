package esper

import (
	"context"
	"fmt"
	"testing"
)

func TestUnidirectionalFullOuterAllDriversEmitTransientRowsMatchesEsper(t *testing.T) {
	for _, sourceCount := range []int{2, 3} {
		sourceCount := sourceCount
		t.Run(fmt.Sprintf("%d-stream", sourceCount), func(t *testing.T) {
			env := NewEnvironment()
			sourceNames := make([]string, sourceCount)
			inputs := make([]JoinInput, sourceCount)
			selections := make([]JoinSelection, 0, sourceCount)
			for source := 0; source < sourceCount; source++ {
				sourceNames[source] = fmt.Sprintf("UniAllDriverS%d", source)
				if _, err := RegisterStruct[joinUnidirectionalWhereRow](env, sourceNames[source]); err != nil {
					t.Fatal(err)
				}
				inputs[source] = JoinSource(From[joinUnidirectionalWhereRow](env, sourceNames[source])).Unidirectional()
				selections = append(selections, SelectFrom(
					source,
					fmt.Sprintf("s%d", source),
					JoinField[string](source, "id"),
				))
			}
			plan, err := env.Build(JoinMany(inputs...).FullOuter().Select(selections...).Query(
				StatementName(fmt.Sprintf("unidirectional-full-all-%d", sourceCount)),
			))
			if err != nil {
				t.Fatal(err)
			}
			engine := env.NewEngine()
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())
			var batches []ResultBatch
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				batches = append(batches, batch)
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			assertCurrent := func(source int, id string) {
				t.Helper()
				if len(batches) == 0 {
					t.Fatalf("all-driver full outer emitted no batch for source %d/%s", source, id)
				}
				batch := batches[len(batches)-1]
				if len(batch.Old) != 0 || len(batch.New) != 1 {
					t.Fatalf("all-driver full outer batch = %#v", batch)
				}
				row, ok := batch.New[0].Row()
				if !ok {
					t.Fatalf("all-driver full outer result is not a row: %#v", batch.New[0])
				}
				for candidate := 0; candidate < sourceCount; candidate++ {
					value := row.Get(fmt.Sprintf("s%d", candidate))
					if candidate == source {
						if value.IsNull() || value.Any() != id {
							t.Fatalf("all-driver current source %d row = %#v", source, row.AsMap())
						}
					} else if !value.IsNull() {
						t.Fatalf("all-driver source %d retained passive value in row = %#v", candidate, row.AsMap())
					}
				}
			}
			send := func(source int, id string) {
				t.Helper()
				if err := engine.Send(context.Background(), sourceNames[source], joinUnidirectionalWhereRow{ID: id}); err != nil {
					t.Fatal(err)
				}
				assertCurrent(source, id)
			}

			send(0, "A1")
			if sourceCount == 2 {
				send(1, "B1")
				send(1, "B2")
				send(0, "A2")
				return
			}
			send(2, "C1")
			send(2, "C2")
			send(0, "A2")
			send(1, "B1")
			send(1, "B2")
		})
	}
}
