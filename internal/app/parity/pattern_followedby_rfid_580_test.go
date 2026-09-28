package parity

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/liubaicai/esper/internal/compat"
)

// Siblings excluded from this slice (no scenario steps, no trace):
//   - ord 0 PatternOpWHarness java-runtime-* — the shared sixteen-case
//     followed-by list over the mixed event set (separate harness slice).
//   - ords 1/2 (timer/guard variants) and ords 6-9 (followed-by chains,
//     quiesce) — future slices; manifest dispositions are main-agent
//     owned.

// TestRunPatternFollowedByRFID580ScenarioReplay replays the checked-in
// scenario through the Go runner and pins the record surface the oracle
// produces: 4 listener deliveries across the three cases — ZERO for
// memory-rfid (each ("a","111") repeat cancels the pending same-mac
// branch so the 10-second timer branch never completes), 2 for
// zone-exit ((a,2) and (b,2) exit reports; the duplicate (b,1) cancels
// and re-arms only its own mac branch), and 2 for zone-enter ((a,1) and
// (b,1) entry reports; the repeat (b,2) cancels the armed watch via the
// correlated zoneID=a.zoneID not-branch). Every delivered row carries
// both bound tags `a` and `b` as SupportRFIDEvent fragments with
// locationReportId rendered as the null marker (the Java two-argument
// constructor leaves it null).
func TestRunPatternFollowedByRFID580ScenarioReplay(t *testing.T) {
	scenario := loadPatternFollowedByRFID580ScenarioForTest(t)
	trace, err := runPatternFollowedByRFID580Scenario(t.Context(), scenario)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if trace.Version != compat.ScenarioVersion {
		t.Fatalf("trace version = %q", trace.Version)
	}
	if trace.ID != patternFollowedByRFID580ID {
		t.Fatalf("trace id = %q", trace.ID)
	}
	if len(trace.Records) != 4 {
		t.Fatalf("records = %d, want 4 (memory-rfid contributes zero)", len(trace.Records))
	}
	byCase := map[string][]compat.TraceRecord{}
	for _, record := range trace.Records {
		if record.Operation != "listener" {
			t.Fatalf("unexpected operation %q", record.Operation)
		}
		if record.Statement != "s0" {
			t.Fatalf("unexpected statement %q", record.Statement)
		}
		if len(record.Old) != 0 {
			t.Fatalf("delivery should be new-only: %v", record.Old)
		}
		byCase[record.Case] = append(byCase[record.Case], record)
	}
	// rfidRow reads one bound tag's delivered RFID bean row fields.
	rfidRow := func(record compat.TraceRecord, tag string) map[string]any {
		row, ok := record.New[0].Fields[tag].(map[string]any)
		if !ok {
			return nil
		}
		fields, ok := row["fields"].(map[string]any)
		if !ok {
			return nil
		}
		return fields
	}
	assertRFID := func(name string, wants [][2][2]string) {
		t.Helper()
		deliveries := byCase[name]
		if len(deliveries) != len(wants) {
			t.Fatalf("%s deliveries = %d, want %d", name, len(deliveries), len(wants))
		}
		for index, want := range wants {
			if len(deliveries[index].New) != 1 {
				t.Fatalf("%s delivery %d = %v, want one row", name, index, deliveries[index].New)
			}
			aFields := rfidRow(deliveries[index], "a")
			bFields := rfidRow(deliveries[index], "b")
			if aFields == nil || bFields == nil {
				t.Fatalf("%s delivery %d = %v, want a and b RFID fragments",
					name, index, deliveries[index].New)
			}
			if aFields["mac"] != want[0][0] || aFields["zoneID"] != want[0][1] {
				t.Fatalf("%s delivery %d a = %v, want {%s,%s}",
					name, index, aFields, want[0][0], want[0][1])
			}
			if bFields["mac"] != want[1][0] || bFields["zoneID"] != want[1][1] {
				t.Fatalf("%s delivery %d b = %v, want {%s,%s}",
					name, index, bFields, want[1][0], want[1][1])
			}
			for tag, fields := range map[string]map[string]any{"a": aFields, "b": bFields} {
				nullMarker, ok := fields["locationReportId"].(map[string]any)
				if !ok || nullMarker["state"] != "null" {
					t.Fatalf("%s delivery %d %s.locationReportId = %v, want null marker",
						name, index, tag, fields["locationReportId"])
				}
			}
		}
	}
	if len(byCase["memory-rfid"]) != 0 {
		t.Fatalf("memory-rfid deliveries = %d, want 0", len(byCase["memory-rfid"]))
	}
	assertRFID("zone-exit", [][2][2]string{{{"a", "1"}, {"a", "2"}}, {{"b", "1"}, {"b", "2"}}})
	assertRFID("zone-enter", [][2][2]string{{{"a", "2"}, {"a", "1"}}, {{"b", "2"}, {"b", "1"}}})
}

// TestRunPatternFollowedByRFID580PinnedArtifacts pins the scenario
// file's Java identity plus the step sequence the loader accepts.
func TestRunPatternFollowedByRFID580PinnedArtifacts(t *testing.T) {
	scenario := loadPatternFollowedByRFID580ScenarioForTest(t)
	if scenario.ID != patternFollowedByRFID580ID {
		t.Fatalf("id = %q", scenario.ID)
	}
	if len(scenario.Steps) != 39 {
		t.Fatalf("steps = %d, want 39", len(scenario.Steps))
	}
	if scenario.Steps[0].Op != "case" || scenario.Steps[0].Case != "memory-rfid" {
		t.Fatalf("first step = %+v", scenario.Steps[0])
	}
	var cases, deploys, sends, undeploys int
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			cases++
		case "deploy":
			deploys++
		case "send":
			sends++
		case "undeploy-all":
			undeploys++
		}
	}
	if cases != 3 || deploys != 3 || undeploys != 3 || sends != 30 {
		t.Fatalf("cases/deploys/sends/undeploys = %d/%d/%d/%d",
			cases, deploys, sends, undeploys)
	}
}

// TestRunPatternFollowedByRFID580RejectsMalformedRawScenario verifies
// the loader pins the step sequence: a mutated EPL or send payload is
// rejected. The ord 5 mutation constantizes the correlated
// zoneID=a.zoneID not-conjunct — the discriminant that makes the repeat
// (b,2) cancel the armed entry watch.
func TestRunPatternFollowedByRFID580RejectsMalformedRawScenario(t *testing.T) {
	raw, err := os.ReadFile(patternFollowedByRFID580ScenarioPath(t))
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	for _, mutate := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"timer length", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("timer:interval(10 sec)"),
				[]byte("timer:interval(10 seconds)"), 1)
		}},
		{"exit not-branch zone", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("not SupportRFIDEvent(mac=a.mac,zoneID='1'))]"),
				[]byte("not SupportRFIDEvent(mac=a.mac,zoneID='2'))]"), 1)
		}},
		{"enter not-branch correlation", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte("not SupportRFIDEvent(mac=a.mac,zoneID=a.zoneID))]"),
				[]byte("not SupportRFIDEvent(mac=a.mac,zoneID='1'))]"), 1)
		}},
		{"zoneID case", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"mac": "a", "zoneID": "2"}`),
				[]byte(`"payload": {"mac": "a", "zoneid": "2"}`), 1)
		}},
		{"send payload", func(data []byte) []byte {
			return bytes.Replace(data,
				[]byte(`"payload": {"mac": "a", "zoneID": "1"}`),
				[]byte(`"payload": {"mac": "b", "zoneID": "1"}`), 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if _, err := loadPatternFollowedByRFID580Scenario(bytes.NewReader(mutate.mutate(raw))); err == nil {
				t.Fatalf("mutated %s scenario was accepted", mutate.name)
			}
		})
	}
}

// TestRunPatternFollowedByRFID580RuntimeIDMappingMatchesScenario pins
// the case->runtime/execution/ordinal mapping the -diff path reports:
// each of the three cases owns its execution's runtimeId and ordinal.
func TestRunPatternFollowedByRFID580RuntimeIDMappingMatchesScenario(t *testing.T) {
	wantOrdinals := []int{3, 4, 5}
	wantRuntimeIndex := []int{0, 1, 2}
	if len(patternFollowedByRFID580CaseSpecs) != len(wantOrdinals) {
		t.Fatalf("case spec count = %d", len(patternFollowedByRFID580CaseSpecs))
	}
	for index, spec := range patternFollowedByRFID580CaseSpecs {
		if spec.ordinal != wantOrdinals[index] || spec.runtimeIndex != wantRuntimeIndex[index] {
			t.Fatalf("case %d mapping = ordinal %d runtimeIndex %d",
				index, spec.ordinal, spec.runtimeIndex)
		}
	}
	for index, id := range patternFollowedByRFID580JavaStaticIDs {
		if id != "java-089b2086c9945dff918f" {
			t.Fatalf("static id %d = %q", index, id)
		}
	}
}

func patternFollowedByRFID580ScenarioPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "parity",
		"pattern-followedby-rfid-580.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scenario path: %v", err)
	}
	return path
}

func loadPatternFollowedByRFID580ScenarioForTest(t *testing.T) compat.Scenario {
	t.Helper()
	file, err := os.Open(patternFollowedByRFID580ScenarioPath(t))
	if err != nil {
		t.Fatalf("open scenario: %v", err)
	}
	defer file.Close()
	scenario, err := loadPatternFollowedByRFID580Scenario(file)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	return scenario
}
