package esper

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestJoinChainMixedOuterPreservesIntermediateRowsMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "ChainOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "ChainPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "ChainShipment"); err != nil {
		t.Fatal(err)
	}
	orders := From[joinOrder](env, "ChainOrder")
	payments := From[joinPayment](env, "ChainPayment")
	shipments := From[joinShipment](env, "ChainShipment")
	chain := JoinChain(JoinSource(orders)).
		LeftOuterJoin(JoinSource(payments), OnSourcesEqual(
			0, Field[joinOrder, string]("orderID"),
			1, Field[joinPayment, string]("orderID"),
		)).
		FullOuterJoin(JoinSource(shipments), OnSourcesEqual(
			0, Field[joinOrder, string]("orderID"),
			2, Field[joinShipment, string]("orderID"),
		))
	plan, err := env.Build(chain.Select(
		SelectFrom(0, "order", Field[joinOrder, string]("orderID")),
		SelectFrom(1, "amount", Field[joinPayment, float64]("amount")),
		SelectFrom(2, "shipment", Field[joinShipment, string]("orderID")),
	).Query(StatementName("mixed-outer-chain")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	if _, err := deployment.Statements()[0].Subscribe(func(context.Context, ResultBatch) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ChainOrder", joinOrder{OrderID: "O1", Symbol: "one"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ChainShipment", joinShipment{OrderID: "O2", Carrier: "two"}); err != nil {
		t.Fatal(err)
	}
	assertJoinChainRows(t, deployment.Statements()[0], map[string]bool{
		"O1/null/null": true,
		"null/null/O2": true,
	})

	if err := engine.Send(context.Background(), "ChainPayment", joinPayment{OrderID: "O1", Amount: 7}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Send(context.Background(), "ChainShipment", joinShipment{OrderID: "O1", Carrier: "one"}); err != nil {
		t.Fatal(err)
	}
	assertJoinChainRows(t, deployment.Statements()[0], map[string]bool{
		"O1/7/O1":      true,
		"null/null/O2": true,
	})
}

func assertJoinChainRows(t *testing.T, statement *Statement, expected map[string]bool) {
	t.Helper()
	snapshot, err := statement.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, len(snapshot.Results()))
	for _, result := range snapshot.Results() {
		row, ok := result.Row()
		if !ok {
			t.Fatalf("join chain result = %#v", result)
		}
		parts := make([]string, 3)
		for index, name := range []string{"order", "amount", "shipment"} {
			value := row.Get(name)
			if value.IsNull() {
				parts[index] = "null"
			} else {
				parts[index] = value.String()
			}
		}
		seen[strings.Join(parts, "/")] = true
	}
	if len(seen) != len(expected) {
		t.Fatalf("join chain rows = %#v, want %#v", seen, expected)
	}
	for key := range expected {
		if !seen[key] {
			t.Fatalf("join chain rows = %#v, missing %q", seen, key)
		}
	}
}

func TestJoinChainValidationAndPlanIdentity(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[joinOrder](env, "ChainPlanOrder"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinPayment](env, "ChainPlanPayment"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[joinShipment](env, "ChainPlanShipment"); err != nil {
		t.Fatal(err)
	}
	order := JoinSource(From[joinOrder](env, "ChainPlanOrder"))
	payment := JoinSource(From[joinPayment](env, "ChainPlanPayment"))
	shipment := JoinSource(From[joinShipment](env, "ChainPlanShipment"))
	firstCondition := OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID"))
	secondCondition := OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 2, Field[joinShipment, string]("orderID"))
	selectRows := func(chain ChainedJoinStream) Query {
		return chain.Select(
			SelectFrom(0, "order", Field[joinOrder, string]("orderID")),
		).Query(StatementName("join-chain-plan"))
	}
	leftFull, err := env.Build(selectRows(JoinChain(order).
		LeftOuterJoin(payment, firstCondition).
		FullOuterJoin(shipment, secondCondition)))
	if err != nil {
		t.Fatal(err)
	}
	leftRight, err := env.Build(selectRows(JoinChain(order).
		LeftOuterJoin(payment, firstCondition).
		RightOuterJoin(shipment, secondCondition)))
	if err != nil {
		t.Fatal(err)
	}
	if leftFull.Hash() == leftRight.Hash() {
		t.Fatal("mixed join edge kinds did not enter plan identity")
	}
	if _, err := env.Build(selectRows(JoinChain(order).LeftOuterJoin(payment))); err == nil || !strings.Contains(err.Error(), "requires at least one condition") {
		t.Fatalf("conditionless join edge error = %v", err)
	}
	wrongEdge := JoinChain(order).
		LeftOuterJoin(payment, firstCondition).
		LeftOuterJoin(shipment, firstCondition)
	if _, err := env.Build(selectRows(wrongEdge)); err == nil || !strings.Contains(err.Error(), "does not reference introduced source 2") {
		t.Fatalf("wrong-source join edge error = %v", err)
	}
	futureCondition := AllJoin(
		firstCondition,
		OnSourcesEqual(1, Field[joinPayment, string]("orderID"), 2, Field[joinShipment, string]("orderID")),
	)
	if _, err := env.Build(selectRows(JoinChain(order).LeftOuterJoin(payment, futureCondition))); err == nil || !strings.Contains(err.Error(), "outside 2 sources") {
		t.Fatalf("future-source join edge error = %v", err)
	}
}

type joinChainTail struct {
	OrderID string `esper:"orderID"`
	Code    string `esper:"code"`
}

func TestJoinChainFourStreamOuterRootsMatchEsper(t *testing.T) {
	for _, root := range []int{0, 1, 2, 3} {
		root := root
		t.Run(fmt.Sprintf("root-s%d", root), func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[joinOrder](env, fmt.Sprintf("ChainRootS%d", root)); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[joinPayment](env, fmt.Sprintf("ChainRootS%dPayment", root)); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[joinShipment](env, fmt.Sprintf("ChainRootS%dShipment", root)); err != nil {
				t.Fatal(err)
			}
			if _, err := RegisterStruct[joinChainTail](env, fmt.Sprintf("ChainRootS%dTail", root)); err != nil {
				t.Fatal(err)
			}

			s0 := JoinSource(From[joinOrder](env, fmt.Sprintf("ChainRootS%d", root)))
			s1 := JoinSource(From[joinPayment](env, fmt.Sprintf("ChainRootS%dPayment", root)))
			s2 := JoinSource(From[joinShipment](env, fmt.Sprintf("ChainRootS%dShipment", root)))
			s3 := JoinSource(From[joinChainTail](env, fmt.Sprintf("ChainRootS%dTail", root)))
			c01 := OnSourcesEqual(0, Field[joinOrder, string]("orderID"), 1, Field[joinPayment, string]("orderID"))
			c12 := OnSourcesEqual(1, Field[joinPayment, string]("orderID"), 2, Field[joinShipment, string]("orderID"))
			c23 := OnSourcesEqual(2, Field[joinShipment, string]("orderID"), 3, Field[joinChainTail, string]("orderID"))
			var chain ChainedJoinStream
			switch root {
			case 0:
				chain = JoinChain(s0).LeftOuterJoin(s1, c01).LeftOuterJoin(s2, c12).LeftOuterJoin(s3, c23)
			case 1:
				chain = JoinChain(s1).RightOuterJoin(s0, OnSourcesEqual(0, Field[joinPayment, string]("orderID"), 1, Field[joinOrder, string]("orderID"))).LeftOuterJoin(s2, c12).LeftOuterJoin(s3, c23)
			case 2:
				chain = JoinChain(s2).RightOuterJoin(s1, OnSourcesEqual(0, Field[joinShipment, string]("orderID"), 1, Field[joinPayment, string]("orderID"))).RightOuterJoin(s0, OnSourcesEqual(1, Field[joinPayment, string]("orderID"), 2, Field[joinOrder, string]("orderID"))).LeftOuterJoin(s3, OnSourcesEqual(0, Field[joinShipment, string]("orderID"), 3, Field[joinChainTail, string]("orderID")))
			case 3:
				chain = JoinChain(s3).RightOuterJoin(s2, OnSourcesEqual(0, Field[joinChainTail, string]("orderID"), 1, Field[joinShipment, string]("orderID"))).RightOuterJoin(s1, OnSourcesEqual(1, Field[joinShipment, string]("orderID"), 2, Field[joinPayment, string]("orderID"))).RightOuterJoin(s0, OnSourcesEqual(2, Field[joinPayment, string]("orderID"), 3, Field[joinOrder, string]("orderID")))
			default:
				t.Fatalf("unexpected root %d", root)
			}
			// JoinChain indexes follow declaration order. Map them back to the
			// logical Java aliases so reverse-root variants assert the same tuple.
			logicalIndexes := [4]int{0, 1, 2, 3}
			switch root {
			case 1:
				logicalIndexes = [4]int{1, 0, 2, 3}
			case 2:
				logicalIndexes = [4]int{2, 1, 0, 3}
			case 3:
				logicalIndexes = [4]int{3, 2, 1, 0}
			}
			plan, err := env.Build(chain.Select(
				SelectFrom(logicalIndexes[0], "s0", Field[joinOrder, string]("orderID")),
				SelectFrom(logicalIndexes[1], "s1", Field[joinPayment, string]("orderID")),
				SelectFrom(logicalIndexes[2], "s2", Field[joinShipment, string]("orderID")),
				SelectFrom(logicalIndexes[3], "s3", Field[joinChainTail, string]("orderID")),
			).Query(StatementName(fmt.Sprintf("four-stream-outer-root-%d", root))))
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			defer deployment.Undeploy(context.Background())

			// The Java chain suite sends the upstream events first and the root
			// event last. Each root direction must produce the same full tuple.
			if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%dPayment", root), joinPayment{OrderID: "A", Amount: 7}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%dShipment", root), joinShipment{OrderID: "A", Carrier: "carrier"}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%dTail", root), joinChainTail{OrderID: "A", Code: "tail"}); err != nil {
				t.Fatal(err)
			}
			if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%d", root), joinOrder{OrderID: "A", Symbol: "order"}); err != nil {
				t.Fatal(err)
			}

			// A second key stops before the final tail to retain the expected
			// outer row with a null tail, regardless of the chosen root.
			if root == 3 {
				if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%d", root), joinOrder{OrderID: "B", Symbol: "order"}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%dPayment", root), joinPayment{OrderID: "B", Amount: 8}); err != nil {
					t.Fatal(err)
				}
				if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%dShipment", root), joinShipment{OrderID: "B", Carrier: "carrier"}); err != nil {
					t.Fatal(err)
				}
				if root == 0 {
					if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%d", root), joinOrder{OrderID: "B", Symbol: "order"}); err != nil {
						t.Fatal(err)
					}
				} else if root == 1 {
					if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%d", root), joinOrder{OrderID: "B", Symbol: "order"}); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := engine.Send(context.Background(), fmt.Sprintf("ChainRootS%d", root), joinOrder{OrderID: "B", Symbol: "order"}); err != nil {
						t.Fatal(err)
					}
				}
			}

			snapshot, err := deployment.Statements()[0].Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			seen := make(map[string]bool)
			for _, result := range snapshot.Results() {
				row, ok := result.Row()
				if !ok {
					t.Fatalf("four-stream outer root result = %#v", result)
				}
				values := make([]string, 4)
				for index, name := range []string{"s0", "s1", "s2", "s3"} {
					value := row.Get(name)
					if value.IsNull() {
						values[index] = "null"
					} else {
						values[index] = value.String()
					}
				}
				seen[strings.Join(values, "/")] = true
			}
			expectedPartial := "B/B/B/null"
			if root == 3 {
				expectedPartial = "B/null/null/null"
			}
			if !seen["A/A/A/A"] || !seen[expectedPartial] {
				t.Fatalf("four-stream outer root %d rows = %#v", root, seen)
			}
		})
	}
}
