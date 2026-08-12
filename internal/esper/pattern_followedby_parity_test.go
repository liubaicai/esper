package esper

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// patternFollowedByCall mirrors SupportCallEvent: a call with a source and
// destination number plus start/end timestamps in milliseconds.
type patternFollowedByCall struct {
	CallID    int64  `esper:"callId"`
	Source    string `esper:"source"`
	Dest      string `esper:"dest"`
	StartTime int64  `esper:"startTime"`
	EndTime   int64  `esper:"endTime"`
}

// patternFollowedByRFID mirrors SupportRFIDEvent: a tag location report with
// a mac address and a zone identifier.
type patternFollowedByRFID struct {
	Mac    string `esper:"mac"`
	ZoneID string `esper:"zoneID"`
}

// TestPatternOperatorFollowedByWHarnessMatchesEsper covers PatternOpWHarness:
// the shared sixteen-case followed-by list over the mixed event set. Every
// case asserts the exact match sequence Esper produces, including vacant
// not-branches, bounded -[N]> edges, and nested-every multiplicity.
func TestPatternOperatorFollowedByWHarnessMatchesEsper(t *testing.T) {
	trueExpr := Literal[bool](true)

	cases := []struct {
		name  string
		tags  []string
		build func(env *Environment) PatternStream
		want  [][]string // each fire as flattened tag=id pairs in order
	}{
		{
			name: "b -> (d or not d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr).Or(PatternFrom(d, "d", trueExpr).Not()))
			},
			want: [][]string{{"b", "B1"}, {"b", "B1", "d", "D1"}},
		},
		{
			name: "b -[1000]> (d or not d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).ThenMax(1000, PatternFrom(d, "d", trueExpr).Or(PatternFrom(d, "d", trueExpr).Not()))
			},
			want: [][]string{{"b", "B1"}, {"b", "B1", "d", "D1"}},
		},
		{
			name: "b -> every d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr).Every())
			},
			want: [][]string{{"b", "B1", "d", "D1"}, {"b", "B1", "d", "D2"}, {"b", "B1", "d", "D3"}},
		},
		{
			name: "b -> d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr))
			},
			want: [][]string{{"b", "B1", "d", "D1"}},
		},
		{
			name: "b -> not d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr).Not())
			},
			want: [][]string{{"b", "B1"}},
		},
		{
			name: "b -[1000]> not d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).ThenMax(1000, PatternFrom(d, "d", trueExpr).Not())
			},
			want: [][]string{{"b", "B1"}},
		},
		{
			name: "every b -> every d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Every().Then(PatternFrom(d, "d", trueExpr).Every())
			},
			want: [][]string{
				{"b", "B1", "d", "D1"}, {"b", "B2", "d", "D1"},
				{"b", "B1", "d", "D2"}, {"b", "B2", "d", "D2"},
				{"b", "B1", "d", "D3"}, {"b", "B2", "d", "D3"}, {"b", "B3", "d", "D3"},
			},
		},
		{
			name: "every b -> d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Every().Then(PatternFrom(d, "d", trueExpr))
			},
			want: [][]string{{"b", "B1", "d", "D1"}, {"b", "B2", "d", "D1"}, {"b", "B3", "d", "D3"}},
		},
		{
			name: "every b -[10]> d",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Every().ThenMax(10, PatternFrom(d, "d", trueExpr))
			},
			want: [][]string{{"b", "B1", "d", "D1"}, {"b", "B2", "d", "D1"}, {"b", "B3", "d", "D3"}},
		},
		{
			name: "every (b -> every d)",
			tags: []string{"b", "d"},
			build: func(env *Environment) PatternStream {
				b := From[patternNotB](env, "SupportBean_B")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(b, "b", trueExpr).Then(PatternFrom(d, "d", trueExpr).Every()).Every()
			},
			want: [][]string{
				{"b", "B1", "d", "D1"},
				{"b", "B1", "d", "D2"},
				{"b", "B1", "d", "D3"}, {"b", "B3", "d", "D3"}, {"b", "B3", "d", "D3"},
			},
		},
		{
			name: "every (a_1 -> b -> a_2)",
			tags: []string{"a_1", "b", "a_2"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(a, "a_1", trueExpr).Then(PatternFrom(b, "b", trueExpr)).Then(PatternFrom(a, "a_2", trueExpr)).Every()
			},
			want: [][]string{{"a_1", "A1", "b", "B1", "a_2", "A2"}},
		},
		{
			name: "c -> d -> a",
			tags: []string{"c", "d", "a"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				c := From[patternNotC](env, "SupportBean_C")
				d := From[patternNotD](env, "SupportBean_D")
				return PatternFrom(c, "c", trueExpr).Then(PatternFrom(d, "d", trueExpr)).Then(PatternFrom(a, "a", trueExpr))
			},
			want: nil,
		},
		{
			name: "every (a_1 -> b -> a_2) paren",
			tags: []string{"a_1", "b", "a_2"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(a, "a_1", trueExpr).Then(PatternFrom(b, "b", trueExpr)).Then(PatternFrom(a, "a_2", trueExpr)).Every()
			},
			want: [][]string{{"a_1", "A1", "b", "B1", "a_2", "A2"}},
		},
		{
			name: "every (a_1 -[10]> b -[10]> a_2)",
			tags: []string{"a_1", "b", "a_2"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(a, "a_1", trueExpr).ThenMax(10, PatternFrom(b, "b", trueExpr)).ThenMax(10, PatternFrom(a, "a_2", trueExpr)).Every()
			},
			want: [][]string{{"a_1", "A1", "b", "B1", "a_2", "A2"}},
		},
		{
			name: "every (every a -> every b)",
			tags: []string{"a", "b"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(a, "a", trueExpr).Every().Then(PatternFrom(b, "b", trueExpr).Every()).Every()
			},
			want: [][]string{
				{"a", "A1", "b", "B1"},
				{"a", "A1", "b", "B2"},
				{"a", "A1", "b", "B3"}, {"a", "A2", "b", "B3"}, {"a", "A2", "b", "B3"}, {"a", "A2", "b", "B3"},
			},
		},
		{
			name: "every (a -> every b)",
			tags: []string{"a", "b"},
			build: func(env *Environment) PatternStream {
				a := From[patternNotA](env, "SupportBean_A")
				b := From[patternNotB](env, "SupportBean_B")
				return PatternFrom(a, "a", trueExpr).Then(PatternFrom(b, "b", trueExpr).Every()).Every()
			},
			want: [][]string{
				{"a", "A1", "b", "B1"},
				{"a", "A1", "b", "B2"},
				{"a", "A1", "b", "B3"}, {"a", "A2", "b", "B3"}, {"a", "A2", "b", "B3"},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env, engine := newPatternNotEnv(t)
			defer func() { _ = engine.Close(context.Background()) }()
			selections := make([]Selection, 0, len(testCase.tags))
			for _, tag := range testCase.tags {
				selections = append(selections, Alias(tag, TagField[string](tag, "id")))
			}
			plan, err := env.Build(testCase.build(env).Select(selections...).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			var got [][]string
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				for _, result := range batch.New {
					row, ok := result.Row()
					if !ok {
						return fmt.Errorf("pattern result is not a row: %#v", result)
					}
					fire := []string{}
					for _, tag := range testCase.tags {
						if value := row.Get(tag); value.State() == ValuePresent {
							fire = append(fire, tag, value.Any().(string))
						}
					}
					got = append(got, fire)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			sendPatternNotSet(t, engine)
			if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", testCase.want) {
				t.Fatalf("%s: fires = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

// TestPatternFollowedByWithNotMatchesEsper covers PatternFollowedByWithNot:
// every a=SupportBean_A -> (timer:interval(10 seconds) and not
// (SupportBean_B(id=a.id) or SupportBean_C(id=a.id))). A correlated B or C
// event cancels only its own branch; branches without a cancel fire when
// their ten-second timer expires.
func TestPatternFollowedByWithNotMatchesEsper(t *testing.T) {
	env, engine := newPatternNotEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(0).UTC()); err != nil {
		t.Fatal(err)
	}

	a := From[patternNotA](env, "SupportBean_A")
	b := From[patternNotB](env, "SupportBean_B")
	c := From[patternNotC](env, "SupportBean_C")
	pattern := PatternFrom(a, "a", Literal[bool](true)).Every().Then(
		TimerInterval(a, 10*time.Second).And(
			PatternFrom(b, "b", Equal[string](Field[patternNotB, string]("id"), TagField[string]("a", "id"))).
				Or(PatternFrom(c, "c", Equal[string](Field[patternNotC, string]("id"), TagField[string]("a", "id")))).Not()),
	)
	plan, err := env.Build(pattern.Select(
		Alias("id", TagField[string]("a", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, result.Get("id").Any().(string))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	send := func(event any) {
		t.Helper()
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	// No Completed or Cancel event arrives: A1 fires when its timer expires.
	send(patternNotA{ID: "A1"})
	advance(9999)
	advance(10000)

	// A Completed event arrives within the time set: A2 never fires.
	advance(20000)
	send(patternNotA{ID: "A2"})
	advance(29999)
	send(patternNotB{ID: "A2"})
	advance(30000)

	// A Cancelled event arrives within the time set: A3 never fires.
	advance(30000)
	send(patternNotA{ID: "A3"})
	advance(30000)
	send(patternNotC{ID: "A3"})
	advance(40000)

	// No matching Completed or Cancel event arrives: A4 fires at t=50000.
	send(patternNotA{ID: "A4"})
	send(patternNotB{ID: "B4"})
	send(patternNotC{ID: "A5"})
	advance(50000)

	if fmt.Sprintf("%v", got) != "[A1 A4]" {
		t.Fatalf("fires = %v, want [A1 A4]", got)
	}
}

// TestPatternFollowedByTimerMatchesEsper covers PatternFollowedByTimer: every
// A=SupportCallEvent -> every B=SupportCallEvent(dest=A.dest, startTime in
// [A.startTime:A.endTime]) where timer:within (7200000), plus the
// statement-level where B.source != A.source filter. Each later call pairs
// with every earlier call whose window covers its start time.
func TestPatternFollowedByTimerMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternFollowedByCall](env, "SupportCallEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	call := From[patternFollowedByCall](env, "SupportCallEvent")
	pattern := PatternFrom(call, "A", Literal[bool](true)).Every().Then(
		PatternFrom(call, "B", And(
			Equal[string](Field[patternFollowedByCall, string]("dest"), TagField[string]("A", "dest")),
			Between[int64](Field[patternFollowedByCall, int64]("startTime"), TagField[int64]("A", "startTime"), TagField[int64]("A", "endTime")),
		)).Every().Within(7200000 * time.Millisecond),
	)
	plan, err := env.Build(pattern.Select(
		Alias("aId", TagField[int64]("A", "callId")),
		Alias("bId", TagField[int64]("B", "callId")),
	).Where(NotEqual[string](TagField[string]("B", "source"), TagField[string]("A", "source"))).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, fmt.Sprintf("%v,%v", result.Get("aId").Any(), result.Get("bId").Any()))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	send := func(callID int64, source string, startTime, endTime int64) {
		t.Helper()
		event := patternFollowedByCall{CallID: callID, Source: source, Dest: "123456789014795", StartTime: startTime, EndTime: endTime}
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	send(2000002601, "18", 0, 41200)
	send(2000002607, "20", 24100, 65400)
	send(2000002610, "22", 38100, 78900)

	want := []string{"2000002601,2000002607", "2000002601,2000002610", "2000002607,2000002610"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("fires = %v, want %v", got, want)
	}
}

// TestPatternMemoryRFIDEventMatchesEsper covers PatternMemoryRFIDEvent: every
// tagMayBeBroken=SupportRFIDEvent -> (timer:interval(10 sec) and not
// SupportRFIDEvent(mac=tagMayBeBroken.mac)). Each repeat location report
// cancels the pending branch for the same mac, so nothing ever fires.
func TestPatternMemoryRFIDEventMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternFollowedByRFID](env, "SupportRFIDEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	rfid := From[patternFollowedByRFID](env, "SupportRFIDEvent")
	pattern := PatternFrom(rfid, "tagMayBeBroken", Literal[bool](true)).Every().Then(
		TimerInterval(rfid, 10*time.Second).And(
			PatternFrom(rfid, "n", Equal[string](Field[patternFollowedByRFID, string]("mac"), TagField[string]("tagMayBeBroken", "mac"))).Not()),
	)
	plan, err := env.Build(pattern.Select(
		Alias("mac", TagField[string]("tagMayBeBroken", "mac")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fires := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		fires += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := engine.SendEvent(context.Background(), patternFollowedByRFID{Mac: "a", ZoneID: "111"}); err != nil {
			t.Fatal(err)
		}
		if err := engine.SendEvent(context.Background(), patternFollowedByRFID{Mac: "a", ZoneID: "111"}); err != nil {
			t.Fatal(err)
		}
	}
	if fires != 0 {
		t.Fatalf("fires = %d, want 0", fires)
	}
}

// TestPatternRFIDZoneExitMatchesEsper covers PatternRFIDZoneExit: every
// a=SupportRFIDEvent(zoneID='1') -> (b=SupportRFIDEvent(mac=a.mac,
// zoneID!='1') and not SupportRFIDEvent(mac=a.mac, zoneID='1')). A zone-1
// report arms the exit watch; a different-zone report completes it unless a
// same-mac zone-1 report cancelled the branch first.
func TestPatternRFIDZoneExitMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternFollowedByRFID](env, "SupportRFIDEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	rfid := From[patternFollowedByRFID](env, "SupportRFIDEvent")
	pattern := PatternFrom(rfid, "a", Equal[string](Field[patternFollowedByRFID, string]("zoneID"), Literal("1"))).Every().Then(
		PatternFrom(rfid, "b", And(
			Equal[string](Field[patternFollowedByRFID, string]("mac"), TagField[string]("a", "mac")),
			NotEqual[string](Field[patternFollowedByRFID, string]("zoneID"), Literal("1")),
		)).And(
			PatternFrom(rfid, "n", And(
				Equal[string](Field[patternFollowedByRFID, string]("mac"), TagField[string]("a", "mac")),
				Equal[string](Field[patternFollowedByRFID, string]("zoneID"), Literal("1")),
			)).Not()),
	)
	plan, err := env.Build(pattern.Select(
		Alias("mac", TagField[string]("b", "mac")),
		Alias("zone", TagField[string]("b", "zoneID")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, fmt.Sprintf("%v,%v", result.Get("mac").Any(), result.Get("zone").Any()))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []patternFollowedByRFID{
		{Mac: "a", ZoneID: "1"},
		{Mac: "a", ZoneID: "2"},
		{Mac: "b", ZoneID: "1"},
		{Mac: "b", ZoneID: "1"},
		{Mac: "b", ZoneID: "2"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"a,2", "b,2"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("fires = %v, want %v", got, want)
	}
}

// TestPatternRFIDZoneEnterMatchesEsper covers PatternRFIDZoneEnter: every
// a=SupportRFIDEvent(zoneID!='1') -> (b=SupportRFIDEvent(mac=a.mac,
// zoneID='1') and not SupportRFIDEvent(mac=a.mac, zoneID=a.zoneID)). An
// out-of-zone report arms the entry watch; the zone-1 report completes it
// unless a repeat same-zone report cancelled the branch first.
func TestPatternRFIDZoneEnterMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternFollowedByRFID](env, "SupportRFIDEvent"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	rfid := From[patternFollowedByRFID](env, "SupportRFIDEvent")
	pattern := PatternFrom(rfid, "a", NotEqual[string](Field[patternFollowedByRFID, string]("zoneID"), Literal("1"))).Every().Then(
		PatternFrom(rfid, "b", And(
			Equal[string](Field[patternFollowedByRFID, string]("mac"), TagField[string]("a", "mac")),
			Equal[string](Field[patternFollowedByRFID, string]("zoneID"), Literal("1")),
		)).And(
			PatternFrom(rfid, "n", And(
				Equal[string](Field[patternFollowedByRFID, string]("mac"), TagField[string]("a", "mac")),
				Equal[string](Field[patternFollowedByRFID, string]("zoneID"), TagField[string]("a", "zoneID")),
			)).Not()),
	)
	plan, err := env.Build(pattern.Select(
		Alias("mac", TagField[string]("b", "mac")),
		Alias("zone", TagField[string]("b", "zoneID")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, fmt.Sprintf("%v,%v", result.Get("mac").Any(), result.Get("zone").Any()))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []patternFollowedByRFID{
		{Mac: "a", ZoneID: "2"},
		{Mac: "a", ZoneID: "1"},
		{Mac: "b", ZoneID: "2"},
		{Mac: "b", ZoneID: "2"},
		{Mac: "b", ZoneID: "1"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"a,1", "b,1"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("fires = %v, want %v", got, want)
	}
}

// TestPatternFollowedNotEveryMatchesEsper covers PatternFollowedNotEvery:
// every A=SupportBean -> (timer:interval(1 seconds) and not SupportBean_A).
// Both armed branches survive (no SupportBean_A ever arrives) and fire
// together when the one-second timer expires.
func TestPatternFollowedNotEveryMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternNotA](env, "SupportBean_A"); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(0).UTC()); err != nil {
		t.Fatal(err)
	}

	sb := From[patternOpBean](env, "SupportBean")
	a := From[patternNotA](env, "SupportBean_A")
	pattern := PatternFrom(sb, "A", Literal[bool](true)).Every().Then(
		TimerInterval(sb, time.Second).And(PatternFrom(a, "na", Literal[bool](true)).Not()),
	)
	plan, err := env.Build(pattern.Select(
		Alias("theString", TagField[string]("A", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fires := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		fires += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E1"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), patternOpBean{TheString: "E2"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(1000).UTC()); err != nil {
		t.Fatal(err)
	}
	if fires != 2 {
		t.Fatalf("fires = %d, want 2", fires)
	}
}

// TestPatternFollowedEveryMultipleMatchesEsper covers
// PatternFollowedEveryMultiple: every a=SupportBean_A -> b=SupportBean_B ->
// c=SupportBean_C -> d=SupportBean_D. Both A-branches pair with the shared
// B1/C1/D1 tail, firing A1's branch first.
func TestPatternFollowedEveryMultipleMatchesEsper(t *testing.T) {
	env, engine := newPatternNotEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()

	a := From[patternNotA](env, "SupportBean_A")
	b := From[patternNotB](env, "SupportBean_B")
	c := From[patternNotC](env, "SupportBean_C")
	d := From[patternNotD](env, "SupportBean_D")
	pattern := PatternFrom(a, "a", Literal[bool](true)).Every().
		Then(PatternFrom(b, "b", Literal[bool](true))).
		Then(PatternFrom(c, "c", Literal[bool](true))).
		Then(PatternFrom(d, "d", Literal[bool](true)))
	plan, err := env.Build(pattern.Select(
		Alias("ida", TagField[string]("a", "id")),
		Alias("idb", TagField[string]("b", "id")),
		Alias("idc", TagField[string]("c", "id")),
		Alias("idd", TagField[string]("d", "id")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			got = append(got, fmt.Sprintf("%v,%v,%v,%v",
				result.Get("ida").Any(), result.Get("idb").Any(), result.Get("idc").Any(), result.Get("idd").Any()))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, event := range []any{
		patternNotA{ID: "A1"},
		patternNotA{ID: "A2"},
		patternNotB{ID: "B1"},
		patternNotC{ID: "C1"},
		patternNotD{ID: "D1"},
	} {
		if err := engine.SendEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"A1,B1,C1,D1", "A2,B1,C1,D1"}
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("fires = %v, want %v", got, want)
	}
}

// TestPatternFilterGreaterThenMatchesEsper covers PatternFilterGreaterThen
// (ESPER-411): the correlated predicate in the second leg must evaluate
// against the captured first-leg event in either operand order, so neither
// variant fires for the E1(10), E2(11) pair.
func TestPatternFilterGreaterThenMatchesEsper(t *testing.T) {
	cases := []struct {
		name      string
		predicate func() Expression[bool]
	}{
		{
			name: "b.intPrimitive <= a.intPrimitive",
			predicate: func() Expression[bool] {
				return LessOrEqual[int](Field[patternOpBean, int]("intPrimitive"), TagField[int]("a", "intPrimitive"))
			},
		},
		{
			name: "a.intPrimitive >= b.intPrimitive",
			predicate: func() Expression[bool] {
				return GreaterOrEqual[int](TagField[int]("a", "intPrimitive"), Field[patternOpBean, int]("intPrimitive"))
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := newPatternOpEnv(t)
			engine := NewEngine(env)
			defer func() { _ = engine.Close(context.Background()) }()

			sb := From[patternOpBean](env, "SupportBean")
			pattern := PatternFrom(sb, "a", Literal[bool](true)).Every().Then(PatternFrom(sb, "b", testCase.predicate()))
			plan, err := env.Build(pattern.Select(
				Alias("theString", TagField[string]("b", "theString")),
			).Query(StatementName("s0")))
			if err != nil {
				t.Fatal(err)
			}
			deployment, err := engine.Deploy(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			fires := 0
			if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
				fires += len(batch.New)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			sendPatternOpBean(t, engine, "E1", 10)
			sendPatternOpBean(t, engine, "E2", 11)
			if fires != 0 {
				t.Fatalf("fires = %d, want 0", fires)
			}
		})
	}
}

// TestPatternFollowedOrPermFalseMatchesEsper covers PatternFollowedOrPermFalse:
// every s=SupportBean(theString='E') -> (timer:interval(10) and not
// SupportBean(theString='C1')) or (SupportBean(theString='C2') and not
// timer:interval(10)). The right alternative's not-timer dies permanently at
// t=10000; the left branch fires when its ten-second timer expires.
func TestPatternFollowedOrPermFalseMatchesEsper(t *testing.T) {
	env := newPatternOpEnv(t)
	engine := NewEngine(env, WithStartTime(time.UnixMilli(0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()
	if err := engine.AdvanceTime(context.Background(), time.UnixMilli(0).UTC()); err != nil {
		t.Fatal(err)
	}

	sb := From[patternOpBean](env, "SupportBean")
	left := PatternFrom(sb, "s", Equal[string](Field[patternOpBean, string]("theString"), Literal("E"))).Every().Then(
		TimerInterval(sb, 10*time.Second).And(
			PatternFrom(sb, "c1", Equal[string](Field[patternOpBean, string]("theString"), Literal("C1"))).Not()))
	right := PatternFrom(sb, "c2", Equal[string](Field[patternOpBean, string]("theString"), Literal("C2"))).
		And(TimerInterval(sb, 10*time.Second).Not())
	pattern := left.Or(right)
	plan, err := env.Build(pattern.Select(
		Alias("theString", TagField[string]("s", "theString")),
	).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	fires := 0
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		fires += len(batch.New)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	advance := func(ms int64) {
		t.Helper()
		if err := engine.AdvanceTime(context.Background(), time.UnixMilli(ms).UTC()); err != nil {
			t.Fatal(err)
		}
	}
	advance(1000)
	sendPatternOpBean(t, engine, "E", 0)
	advance(10999)
	if fires != 0 {
		t.Fatalf("fires before timer expiry = %d, want 0", fires)
	}
	advance(11000)
	if fires != 1 {
		t.Fatalf("fires = %d, want 1", fires)
	}
}
