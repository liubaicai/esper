package esper

import (
	"context"
	"reflect"
	"testing"
	"time"
)

type clientRuntimePortScanAlert struct {
	typ string
	cnt int64
}

type clientRuntimePortScanTimerAnchor struct{}

func TestClientRuntimePortScanPrimarySuccessParity(t *testing.T) {
	origin := clientRuntimePortScanOrigin()
	engine, alerts := deployClientRuntimePortScan(t, origin)
	sendClientRuntimePortScanEvents(t, engine, 20, "A", "B")
	assertClientRuntimePortScanAlerts(t, alerts, clientRuntimePortScanAlert{"DETECTED", 20})
}

func TestClientRuntimePortScanKeepAlertingParity(t *testing.T) {
	origin := clientRuntimePortScanOrigin()
	engine, alerts := deployClientRuntimePortScan(t, origin)
	sendClientRuntimePortScanEvents(t, engine, 20, "A", "B")
	assertClientRuntimePortScanAlerts(t, alerts, clientRuntimePortScanAlert{"DETECTED", 20})
	advanceClientRuntimePortScan(t, engine, origin.Add(29*time.Second))
	sendClientRuntimePortScanEvents(t, engine, 20, "A", "B")
	advanceClientRuntimePortScan(t, engine, origin.Add(59*time.Second))
	sendClientRuntimePortScanEvents(t, engine, 20, "A", "B")
	assertClientRuntimePortScanAlerts(t, alerts)
	advanceClientRuntimePortScan(t, engine, origin.Add(time.Minute))
	assertClientRuntimePortScanAlerts(t, alerts, clientRuntimePortScanAlert{"UPDATE", 20})
}

func TestClientRuntimePortScanFallsUnderThresholdParity(t *testing.T) {
	origin := clientRuntimePortScanOrigin()
	engine, alerts := deployClientRuntimePortScan(t, origin)
	sendClientRuntimePortScanEvents(t, engine, 20, "A", "B")
	assertClientRuntimePortScanAlerts(t, alerts, clientRuntimePortScanAlert{"DETECTED", 20})
	advanceClientRuntimePortScan(t, engine, origin.Add(time.Minute))
	assertClientRuntimePortScanAlerts(t, alerts,
		clientRuntimePortScanAlert{"UPDATE", 0},
		clientRuntimePortScanAlert{"DONE", 0},
	)
}

func deployClientRuntimePortScan(t *testing.T, origin time.Time) (*Engine, *[]clientRuntimePortScanAlert) {
	t.Helper()
	env := NewEnvironment()
	fields := []FieldSpec{
		FieldDef("src", reflect.TypeOf("")),
		FieldDef("dst", reflect.TypeOf("")),
		FieldDef("port", reflect.TypeOf(int(0))),
		FieldDef("marker", reflect.TypeOf("")),
	}
	if _, err := RegisterObjectArray(env, "PortScanEvent", fields); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[clientRuntimePortScanTimerAnchor](env, "PortScanTimerAnchor"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "OutputAlerts", []FieldSpec{
		FieldDef("type", reflect.TypeOf("")),
		FieldDef("cnt", reflect.TypeOf(int64(0))),
		FieldDef("contributors", reflect.TypeOf([]Event{})),
	}); err != nil {
		t.Fatal(err)
	}
	situationSchema, err := RegisterMap(env, "PortScanSituation", []FieldSpec{
		FieldDef("src", reflect.TypeOf("")),
		FieldDef("dst", reflect.TypeOf("")),
		FieldDef("detectionTime", reflect.TypeOf(time.Time{})),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateNamedWindow(env, "SituationsWindow", situationSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTable(env, "ScanCountTable", []TableColumn{
		PrimaryKeyColumn[string]("src"),
		PrimaryKeyColumn[string]("dst"),
		TableColumnOf[int64]("cnt"),
		TableColumnOf[WindowAccessValue[Event]]("win"),
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(env, WithStartTime(origin))
	alerts := make([]clientRuntimePortScanAlert, 0, 4)
	outputPlan, err := env.Build(FromAny(env, "OutputAlerts").Query(StatementName("output")))
	if err != nil {
		t.Fatal(err)
	}
	outputDeployment, err := engine.Deploy(context.Background(), outputPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outputDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			alerts = append(alerts, clientRuntimePortScanAlert{
				typ: result.Get("type").Any().(string),
				cnt: result.Get("cnt").Any().(int64),
			})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	src := Field[any, string]("src")
	dst := Field[any, string]("dst")
	port := Field[any, int]("port")
	window := WindowAccessBy[Event](EventValue[Event]())
	countPlan, err := env.Build(FromAny(env, "PortScanEvent").
		Window(UniqueBy(src, dst, port)).
		Window(TimeWindow(30*time.Second)).
		GroupBy(src, dst).
		Select(
			Alias("src", src),
			Alias("dst", dst),
			Alias("cnt", CountAll()),
			Alias("win", window),
		).
		IntoTable("ScanCountTable", StatementName("count-stream")))
	if err != nil {
		t.Fatal(err)
	}
	countDeployment, err := engine.Deploy(context.Background(), countPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := countDeployment.Statements()[0].Subscribe(func(ctx context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if result.Get("cnt").Any().(int64) < 20 {
				continue
			}
			source := result.Get("src").Any().(string)
			destination := result.Get("dst").Any().(string)
			window, _ := engine.NamedWindow("SituationsWindow")
			existing, err := window.Snapshot(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, situation := range existing {
				if situation.Get("src").Any() == source && situation.Get("dst").Any() == destination {
					found = true
					break
				}
			}
			if found {
				continue
			}
			if err := engine.InsertNamedWindow(ctx, "SituationsWindow", map[string]any{
				"src": source, "dst": destination, "detectionTime": batch.Time,
			}); err != nil {
				return err
			}
			contributors := result.Get("win").Any().(WindowAccessValue[Event]).Values()
			if err := engine.Send(ctx, "OutputAlerts", map[string]any{
				"type": "DETECTED", "cnt": int64(len(contributors)), "contributors": contributors,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	timerPlan, err := env.Build(TimerCron(From[clientRuntimePortScanTimerAnchor](env, "PortScanTimerAnchor"), CronSchedule{
		Minute: CronWildcard(), Hour: CronWildcard(), DayOfMonth: CronWildcard(), Month: CronWildcard(), Weekday: CronWildcard(),
	}).Select(Alias("tick", CurrentTime())).Query(StatementName("minute-maintenance")))
	if err != nil {
		t.Fatal(err)
	}
	timerDeployment, err := engine.Deploy(context.Background(), timerPlan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := timerDeployment.Statements()[0].Subscribe(func(ctx context.Context, batch ResultBatch) error {
		window, _ := engine.NamedWindow("SituationsWindow")
		situations, err := window.Snapshot(ctx)
		if err != nil {
			return err
		}
		table, _ := engine.Table("ScanCountTable")
		for _, situation := range situations {
			source := situation.Get("src").Any().(string)
			destination := situation.Get("dst").Any().(string)
			row, ok, err := table.Get(ctx, source, destination)
			if err != nil {
				return err
			}
			count := int64(0)
			var contributors []Event
			if ok {
				count = row.Get("cnt").Any().(int64)
				contributors = row.Get("win").Any().(WindowAccessValue[Event]).Values()
			}
			if err := engine.Send(ctx, "OutputAlerts", map[string]any{"type": "UPDATE", "cnt": count, "contributors": contributors}); err != nil {
				return err
			}
			detected := situation.Get("detectionTime").Any().(time.Time)
			typeName := ""
			alertCount := count
			if count < 10 {
				typeName = "DONE"
			} else if !batch.Time.Before(detected.Add(16 * time.Hour)) {
				typeName = "EXPIRED"
				alertCount = -1
			}
			if typeName == "" {
				continue
			}
			if _, err := window.DeleteWhere(ctx, func(candidate Event) bool {
				return candidate.Get("src").Any() == source && candidate.Get("dst").Any() == destination
			}); err != nil {
				return err
			}
			if err := engine.Send(ctx, "OutputAlerts", map[string]any{"type": typeName, "cnt": alertCount, "contributors": nil}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return engine, &alerts
}

func clientRuntimePortScanOrigin() time.Time {
	return time.Date(2002, time.May, 30, 8, 0, 0, 0, time.UTC)
}

func sendClientRuntimePortScanEvents(t *testing.T, engine *Engine, count int, src, dst string) {
	t.Helper()
	for index := 0; index < count; index++ {
		if err := engine.Send(context.Background(), "PortScanEvent", []any{src, dst, 16 + index, "m20"}); err != nil {
			t.Fatal(err)
		}
	}
}

func advanceClientRuntimePortScan(t *testing.T, engine *Engine, at time.Time) {
	t.Helper()
	if err := engine.AdvanceTimeSpan(context.Background(), at); err != nil {
		t.Fatal(err)
	}
}

func assertClientRuntimePortScanAlerts(t *testing.T, alerts *[]clientRuntimePortScanAlert, want ...clientRuntimePortScanAlert) {
	t.Helper()
	if !reflect.DeepEqual(*alerts, want) {
		t.Fatalf("port-scan alerts = %#v, want %#v", *alerts, want)
	}
	*alerts = nil
}
