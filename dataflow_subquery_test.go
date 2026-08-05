package esper

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func TestDataflowArrayKeySubqueryWindowMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[namedWindowArrayTrade](env, "DataflowArrayKeyTrade"); err != nil {
		t.Fatal(err)
	}
	inner := Select(From[namedWindowArrayTrade](env, "DataflowArrayKeyTrade")).Window(
		Unique(Field[namedWindowArrayTrade, []int64]("coll")),
	)
	var captured []Row
	definition, err := DefineDataflow(env, "dataflow-array-key-subquery").
		Emitter("source").
		Select("select", Alias("ids", SubqueryValue[[]string](inner, WindowValues[string](Field[any, string]("id"))))).
		LogSink("sink", func(_ context.Context, value any) error {
			row, ok := value.(Row)
			if !ok {
				t.Fatalf("dataflow captive output = %#v, want Row", value)
			}
			captured = append(captured, row)
			return nil
		}).
		Connect("source", "select").
		Connect("select", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	emitter, err := instance.CaptiveEmitter("source")
	if err != nil {
		t.Fatal(err)
	}
	send := func(event namedWindowArrayTrade, want []string) {
		t.Helper()
		if err := emitter.Submit(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if len(captured) == 0 {
			t.Fatal("dataflow did not emit a select row")
		}
		row := captured[len(captured)-1]
		got, ok := row.Get("ids").Any().([]string)
		if !ok {
			t.Fatalf("dataflow ids = %#v, want []string", row.Get("ids").Any())
		}
		got = append([]string(nil), got...)
		want = append([]string(nil), want...)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("dataflow ids = %#v, want %#v", got, want)
		}
	}

	send(namedWindowArrayTrade{ID: "E1", Coll: []int64{1, 2}}, []string{"E1"})
	send(namedWindowArrayTrade{ID: "E2", Coll: []int64{1, 2}}, []string{"E2"})
	send(namedWindowArrayTrade{ID: "E3", Coll: []int64{1}}, []string{"E2", "E3"})
	send(namedWindowArrayTrade{ID: "E4", Coll: []int64{1}}, []string{"E2", "E4"})
	send(namedWindowArrayTrade{ID: "E5", Coll: []int64{1, 2}}, []string{"E4", "E5"})

	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
}
