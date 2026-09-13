package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the final differential slice of InfraNamedWindowViews:
// the pattern consumer and the time-to-live window with on-delete triggers.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 52 InfraPattern                        java-runtime-476271957d6ffdb3a878
//   - ord 57 InfraNamedWindowTimeToLiveDelete    java-runtime-3c2f3a2696127c04b2a6
//
// Ord 52 deploys one keep-all window, a pattern select over the window's
// insert stream (every a=PAT(key='S1') or a=PAT(key='S2')) and one insert
// trigger; five SupportBean sends yield three ordered new rows on s0 while
// the first and the last send stay silent (the S2 completion quits the whole
// or-expression, including the every branch).
//
// Ord 57 deploys a whole-bean timetolive window (deadline
// current_timestamp()+longPrimitive evaluated per row at insert), a merge
// insert trigger and an on-delete trigger matched on theString=p00; the
// observable contract is five any-order iterator snapshots over absolute
// virtual time 0/500/1000/2000. Milestones are persistence round-trips with
// no observable effect and stay out of the trace.
//
// The ord 46/47/48 invalid executions are compile/deploy-only diagnostics
// with zero output rows and are disposed as intentionally-different in the
// manifest (case.infra-namedwindow-views-invalid); their typed-builder
// counterparts stay asserted in-process by TestInfraNWViewsInvalidParity.
const infraNWFVId = "infra-named-window-final-views"

const infraNWFVDescription = "InfraNamedWindowViews final slice: the pattern consumer over the named-window insert stream with or-quit silence, and the time-to-live window with merge insert and on-delete triggers observed through any-order iterator snapshots under absolute virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWFVJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWFVSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 3375-3377 for ord 52 and 111-113 for
// ord 57).
const (
	infraNWFVCreatePattern = "@name('create') create window MyWindowPAT#keepall as MySimpleKeyValueMap"
	infraNWFVS0Pattern     = "@name('s0') select a.key as key, a.value as value from pattern [every a=MyWindowPAT(key='S1') or a=MyWindowPAT(key='S2')]"
	infraNWFVInsertPattern = "insert into MyWindowPAT select theString as key, longBoxed as value from SupportBean"

	infraNWFVCreateTTL = "@name('win') create window MyWindow#timetolive(current_timestamp() + longPrimitive) as SupportBean"
	infraNWFVMergeTTL  = "on SupportBean merge MyWindow insert select *"
	infraNWFVDeleteTTL = "on SupportBean_S0 delete from MyWindow where theString = p00"
)

var (
	infraNWFVJavaRuntimeIDs = []string{
		"java-runtime-476271957d6ffdb3a878",
		"java-runtime-3c2f3a2696127c04b2a6",
	}
	infraNWFVJavaSources = []string{
		infraNWFVSource,
	}
	infraNWFVJavaExecutions = []string{
		"InfraPattern",
		"InfraNamedWindowTimeToLiveDelete",
	}
	infraNWFVJavaStaticIDs = []string{
		"java-030c8e6d456d680e8745",
	}
)

type infraNWFVExpectedSend struct {
	eventType string
	fields    []string
	stringVal string
	longVal   int64
	p00       string
}

type infraNWFVCaseSpec struct {
	name          string
	ordinal       int
	runtimeID     string
	execution     string
	windowName    string
	description   string
	epl           string
	createEPL     string
	insertEPL     string
	s0EPL         string
	mergeEPL      string
	deleteEPL     string
	deploys       []string
	listened      []string
	snapshots     int
	snapModes     []string
	snapStatement string
	snapFields    []string
	advances      []string
	sends         []infraNWFVExpectedSend
	rowFields     map[string][]string
}

var infraNWFVCaseSpecs = []infraNWFVCaseSpec{
	{
		name:        "pattern",
		ordinal:     52,
		runtimeID:   "java-runtime-476271957d6ffdb3a878",
		execution:   "InfraPattern",
		windowName:  "MyWindowPAT",
		description: "pattern consumer over the named-window insert stream: every S1 match re-arms while the single S2 match quits the whole or-expression",
		epl:         infraNWFVCreatePattern,
		createEPL:   infraNWFVCreatePattern,
		insertEPL:   infraNWFVInsertPattern,
		s0EPL:       infraNWFVS0Pattern,
		deploys:     []string{"create", "s0", "insert"},
		listened:    []string{"s0"},
		rowFields: map[string][]string{
			"s0": {"key", "value"},
		},
		sends: []infraNWFVExpectedSend{
			{eventType: "SupportBean", fields: []string{"theString", "longBoxed"}, stringVal: "E1", longVal: 1},
			{eventType: "SupportBean", fields: []string{"theString", "longBoxed"}, stringVal: "S1", longVal: 2},
			{eventType: "SupportBean", fields: []string{"theString", "longBoxed"}, stringVal: "S1", longVal: 3},
			{eventType: "SupportBean", fields: []string{"theString", "longBoxed"}, stringVal: "S2", longVal: 4},
			{eventType: "SupportBean", fields: []string{"theString", "longBoxed"}, stringVal: "S1", longVal: 1},
		},
	},
	{
		name:          "ttl-delete",
		ordinal:       57,
		runtimeID:     "java-runtime-3c2f3a2696127c04b2a6",
		execution:     "InfraNamedWindowTimeToLiveDelete",
		windowName:    "MyWindow",
		description:   "whole-bean timetolive window with merge insert and p00 deletes observed through any-order iterator snapshots over absolute virtual time",
		epl:           infraNWFVCreateTTL,
		createEPL:     infraNWFVCreateTTL,
		mergeEPL:      infraNWFVMergeTTL,
		deleteEPL:     infraNWFVDeleteTTL,
		deploys:       []string{"win", "merge", "delete"},
		listened:      []string{},
		snapshots:     5,
		snapModes:     []string{"any", "any", "any", "any", "any"},
		snapStatement: "win",
		snapFields:    []string{"theString"},
		advances: []string{
			"1970-01-01T00:00:00Z",
			"1970-01-01T00:00:00.500Z",
			"1970-01-01T00:00:01Z",
			"1970-01-01T00:00:02Z",
		},
		sends: []infraNWFVExpectedSend{
			{eventType: "SupportBean", fields: []string{"theString", "longPrimitive"}, stringVal: "E1", longVal: 2000},
			{eventType: "SupportBean", fields: []string{"theString", "longPrimitive"}, stringVal: "E2", longVal: 3000},
			{eventType: "SupportBean", fields: []string{"theString", "longPrimitive"}, stringVal: "E3", longVal: 1000},
			{eventType: "SupportBean", fields: []string{"theString", "longPrimitive"}, stringVal: "E4", longVal: 2000},
			{eventType: "SupportBean_S0", fields: []string{"p00"}, p00: "E2"},
			{eventType: "SupportBean_S0", fields: []string{"p00"}, p00: "E1"},
		},
	},
}

func infraNWFVCaseSpecFor(name string) (infraNWFVCaseSpec, bool) {
	for _, spec := range infraNWFVCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWFVCaseSpec{}, false
}

func loadInfraNWFVScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWFVId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWFVId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWFVId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWFVId, err)
	}
	if err := requireInfraNWFVFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version       string   `json:"version"`
		ID            string   `json:"id"`
		Description   string   `json:"description"`
		JavaCommit    string   `json:"javaCommit"`
		JavaSource    string   `json:"javaSource"`
		JavaRuntimes  []string `json:"javaRuntimes"`
		JavaNames     []string `json:"javaNames"`
		JavaStaticIDs []string `json:"javaStaticIds"`
		JavaFlags     []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWFVId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWFVId ||
		metadata.Description != infraNWFVDescription ||
		metadata.JavaCommit != infraNWFVJavaCommit || metadata.JavaSource != infraNWFVSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWFVId)
	}
	if err := infraNWFVRequireEqual(metadata.JavaRuntimes, infraNWFVJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWFVRequireEqual(metadata.JavaNames, infraNWFVJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWFVRequireEqual(metadata.JavaStaticIDs, infraNWFVJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWFVRequireEqual(metadata.JavaFlags, []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWFVCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWFVId, len(infraNWFVCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWFVFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "observation", "iteratorSnapshots", "epl",
			"createEpl", "insertEpl", "s0Epl", "mergeEpl", "deleteEpl", "deploys", "listened"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string   `json:"case"`
			Ordinal           int      `json:"ordinal"`
			RuntimeID         string   `json:"runtimeId"`
			ExecutionName     string   `json:"executionName"`
			Description       string   `json:"description"`
			Observation       string   `json:"observation"`
			IteratorSnapshots int      `json:"iteratorSnapshots"`
			EPL               string   `json:"epl"`
			CreateEPL         string   `json:"createEpl"`
			InsertEPL         string   `json:"insertEpl"`
			S0EPL             string   `json:"s0Epl"`
			MergeEPL          string   `json:"mergeEpl"`
			DeleteEPL         string   `json:"deleteEpl"`
			Deploys           []string `json:"deploys"`
			Listened          []string `json:"listened"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWFVCaseSpecs[index]
		observation := "listener"
		if spec.snapshots > 0 && len(spec.listened) == 0 {
			observation = "iterator"
		}
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal || definition.RuntimeID != spec.runtimeID ||
			definition.ExecutionName != spec.execution || definition.Description != spec.description ||
			definition.Observation != observation || definition.IteratorSnapshots != spec.snapshots || definition.EPL != spec.epl ||
			definition.CreateEPL != spec.createEPL || definition.InsertEPL != spec.insertEPL || definition.S0EPL != spec.s0EPL ||
			definition.MergeEPL != spec.mergeEPL || definition.DeleteEPL != spec.deleteEPL {
			return compat.Scenario{}, fmt.Errorf("scenario case %d metadata is not pinned", index)
		}
		if err := infraNWFVRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWFVRequireEqual(definition.Listened, spec.listened, spec.name+" listened"); err != nil {
			return compat.Scenario{}, err
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWFVId)
	}
	if err := validateInfraNWFVRawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraNWFVScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateInfraNWFVRawSteps(rawSteps []json.RawMessage) error {
	deployCounts := make(map[string]int)
	sendCounts := make(map[string]int)
	snapshotCounts := make(map[string]int)
	advanceCounts := make(map[string]int)
	undeployCounts := make(map[string]int)
	caseOrder := make([]string, 0, len(infraNWFVCaseSpecs))
	currentCase := ""
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string: %w", index, err)
		}
		switch operation {
		case "case":
			if err := requireInfraNWFVFields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWFVCaseSpecFor(step.Case); !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, step.Case)
			}
			currentCase = step.Case
			caseOrder = append(caseOrder, step.Case)
		case "deploy":
			if err := requireInfraNWFVFields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWFVCaseSpecFor(step.Case)
			if !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d deploy is outside its case block", index)
			}
			position := deployCounts[step.Case]
			if position >= len(spec.deploys) || step.Statement != spec.deploys[position] {
				return fmt.Errorf("scenario step %d deploy statement %q is not pinned for case %q", index, step.Statement, step.Case)
			}
			if want := infraNWFVDeployEPL(spec, step.Statement); step.EPL != want {
				return fmt.Errorf("scenario step %d deploy EPL is not pinned for statement %q", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWFVFields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step compat.Step
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWFVCaseSpecFor(step.Case)
			if !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d send is outside its case block", index)
			}
			position := sendCounts[step.Case]
			if position >= len(spec.sends) {
				return fmt.Errorf("scenario case %q has too many sends", step.Case)
			}
			if err := infraNWFVValidateSend(spec, position, step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			sendCounts[step.Case]++
		case "snapshot":
			if err := requireInfraNWFVFields(object, "op", "case", "statement", "mode"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				Mode      string `json:"mode"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWFVCaseSpecFor(step.Case)
			if !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d snapshot is outside its case block", index)
			}
			position := snapshotCounts[step.Case]
			if position >= len(spec.snapModes) || step.Statement != spec.snapStatement || step.Mode != spec.snapModes[position] {
				return fmt.Errorf("scenario step %d snapshot is not pinned for case %q", index, step.Case)
			}
			snapshotCounts[step.Case]++
		case "advance-time":
			if err := requireInfraNWFVFields(object, "op", "case", "at"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
				At   string `json:"at"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWFVCaseSpecFor(step.Case)
			if !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d advance-time is outside its case block", index)
			}
			position := advanceCounts[step.Case]
			if position >= len(spec.advances) || step.At != spec.advances[position] {
				return fmt.Errorf("scenario step %d advance-time is not pinned for case %q", index, step.Case)
			}
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return fmt.Errorf("scenario step %d advance-time at %q: %w", index, step.At, err)
			}
			advanceCounts[step.Case]++
		case "undeploy-all":
			if err := requireInfraNWFVFields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWFVCaseSpecFor(step.Case); !ok || step.Case != currentCase {
				return fmt.Errorf("scenario step %d undeploy-all is outside its case block", index)
			}
			undeployCounts[step.Case]++
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	if len(caseOrder) != len(infraNWFVCaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want two cases", caseOrder)
	}
	for index, spec := range infraNWFVCaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if deployCounts[spec.name] != len(spec.deploys) || sendCounts[spec.name] != len(spec.sends) ||
			snapshotCounts[spec.name] != spec.snapshots || advanceCounts[spec.name] != len(spec.advances) ||
			undeployCounts[spec.name] != 1 {
			return fmt.Errorf("scenario case %q has unpinned step counts", spec.name)
		}
	}
	return nil
}

func validateInfraNWFVScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWFVId {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, infraNWFVId)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validateInfraNWFVRawSteps(rawSteps)
}

func infraNWFVDeployEPL(spec infraNWFVCaseSpec, statement string) string {
	switch statement {
	case "create", "win":
		return spec.createEPL
	case "insert":
		return spec.insertEPL
	case "s0":
		return spec.s0EPL
	case "merge":
		return spec.mergeEPL
	case "delete":
		return spec.deleteEPL
	default:
		return ""
	}
}

func infraNWFVValidateSend(spec infraNWFVCaseSpec, index int, step compat.Step) error {
	want := spec.sends[index]
	if step.EventType != want.eventType {
		return fmt.Errorf("case %q send %d event type %q, want %q", spec.name, index, step.EventType, want.eventType)
	}
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	if err := requireInfraNWFVFields(fields, want.fields...); err != nil {
		return err
	}
	switch want.eventType {
	case "SupportBean":
		var payload struct {
			TheString     string `json:"theString"`
			LongBoxed     int64  `json:"longBoxed"`
			LongPrimitive int64  `json:"longPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode SupportBean: %w", err)
		}
		if payload.TheString != want.stringVal {
			return fmt.Errorf("case %q SupportBean payload %d theString is not pinned", spec.name, index)
		}
		for _, field := range want.fields {
			switch field {
			case "longBoxed":
				if payload.LongBoxed != want.longVal {
					return fmt.Errorf("case %q SupportBean payload %d longBoxed is not pinned", spec.name, index)
				}
			case "longPrimitive":
				if payload.LongPrimitive != want.longVal {
					return fmt.Errorf("case %q SupportBean payload %d longPrimitive is not pinned", spec.name, index)
				}
			}
		}
	case "SupportBean_S0":
		var payload struct {
			P00 string `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if payload.P00 != want.p00 {
			return fmt.Errorf("case %q SupportBean_S0 payload %d is not pinned", spec.name, index)
		}
	default:
		return fmt.Errorf("unsupported event type %q", want.eventType)
	}
	return nil
}

func infraNWFVRequireEqual(got, want []string, label string) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	return nil
}

func requireInfraNWFVFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

type infraNWFVSupportBean struct {
	TheString     string `esper:"theString"`
	LongBoxed     int64  `esper:"longBoxed"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type infraNWFVKeyValueLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

type infraNWFVBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func runInfraNWFVScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWFVScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWFVCaseSpecs {
		records, err := runInfraNWFVCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWFVId, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func infraNWFVStartEnvironment(spec infraNWFVCaseSpec) (*esper.Environment, error) {
	env := esper.NewEnvironment()
	switch spec.name {
	case "pattern":
		if _, err := esper.RegisterStruct[infraNWFVSupportBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		windowSchema, err := esper.RegisterStruct[infraNWFVKeyValueLong](env, "MySimpleKeyValueMap")
		if err != nil {
			return nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindowPAT", windowSchema, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, err
		}
	case "ttl-delete":
		schema, err := esper.RegisterStruct[infraNWFVSupportBean](env, "SupportBean")
		if err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraNWFVBeanS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		ttl := esper.Func2("ttlDeadline", func(now time.Time, delta int64) int64 {
			return now.UnixMilli() + delta
		}, esper.CurrentTime(), esper.Field[infraNWFVSupportBean, int64]("longPrimitive"))
		if _, err := esper.CreateNamedWindow(env, "MyWindow", schema, esper.NamedWindowRetention(esper.TimeToLiveAt(ttl))); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown case %q", spec.name)
	}
	return env, nil
}

func runInfraNWFVCase(ctx context.Context, scenario compat.Scenario, spec infraNWFVCaseSpec) ([]compat.TraceRecord, error) {
	env, err := infraNWFVStartEnvironment(spec)
	if err != nil {
		return nil, err
	}
	now := time.Unix(0, 0).UTC()
	engine := esper.NewEngine(env, esper.WithRuntimeURI(spec.runtimeID), esper.WithStartTime(now))
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := make(map[string]uint64)
	statements := make(map[string]*esper.Statement)
	var records []compat.TraceRecord
	recordListener := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(now),
			New:       projectRecords(compat.NormalizeResults(batch.New), spec.rowFields[statement]),
			Old:       projectRecords(compat.NormalizeResults(batch.Old), spec.rowFields[statement]),
		})
	}

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		switch step.Op {
		case "deploy":
			plan, err := infraNWFVBuildPlan(env, spec, step.Statement)
			if err != nil {
				return nil, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
				}
			}
			if infraNWFVContains(spec.listened, step.Statement) {
				statement, ok := statements[step.Statement]
				if !ok {
					return nil, fmt.Errorf("deploy %q did not register its statement", step.Statement)
				}
				name := step.Statement
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					recordListener(name, batch)
					return nil
				}); err != nil {
					return nil, err
				}
			}
		case "send":
			payload, err := infraNWFVDecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return nil, fmt.Errorf("snapshot statement %q was not deployed", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			rows := projectRecords(compat.NormalizeResults(result.Batch.New), spec.snapFields)
			if step.Mode == "any" {
				sortRowsCanonical(rows)
			}
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(now),
				New:       rows,
			})
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("advance-time %q: %w", step.At, err)
			}
			now = at.UTC()
			if err := engine.AdvanceTime(ctx, now); err != nil {
				return nil, err
			}
		case "undeploy-all":
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	return records, nil
}

func infraNWFVBuildPlan(env *esper.Environment, spec infraNWFVCaseSpec, statement string) (esper.Plan, error) {
	switch spec.name {
	case "pattern":
		switch statement {
		case "create":
			return env.Build(esper.FromNamedWindow(env, "MyWindowPAT").CreateNamedWindowQuery(esper.StatementName("create")))
		case "s0":
			key := esper.Field[any, string]("key")
			s1 := esper.PatternFromRecord(esper.FromNamedWindow(env, "MyWindowPAT"), "a",
				esper.Equal[string](key, esper.Literal[string]("S1")))
			s2 := esper.PatternFromRecord(esper.FromNamedWindow(env, "MyWindowPAT"), "a",
				esper.Equal[string](key, esper.Literal[string]("S2")))
			return env.Build(s1.Every().Or(s2).Select(
				esper.Alias("key", esper.TagField[string]("a", "key")),
				esper.Alias("value", esper.TagField[int64]("a", "value")),
			).Query(esper.StatementName("s0")))
		case "insert":
			return env.Build(esper.OnEvent(esper.From[infraNWFVSupportBean](env, "SupportBean")).InsertIntoNamedWindow(
				"MyWindowPAT",
				esper.SetColumn("key", esper.Field[infraNWFVSupportBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWFVSupportBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		}
	case "ttl-delete":
		switch statement {
		case "win":
			return env.Build(esper.FromNamedWindow(env, "MyWindow").CreateNamedWindowQuery(esper.StatementName("win")))
		case "merge":
			return env.Build(esper.OnEvent(esper.From[infraNWFVSupportBean](env, "SupportBean")).MergeInsertIntoNamedWindow(
				"MyWindow", esper.Literal(false), esper.CopyMatchingFields(),
			).Query(esper.StatementName("merge")))
		case "delete":
			return env.Build(esper.OnEvent(esper.From[infraNWFVBeanS0](env, "SupportBean_S0")).DeleteFromNamedWindow(
				"MyWindow",
				esper.Equal[string](esper.NamedWindowField[string]("theString"), esper.Field[infraNWFVBeanS0, string]("p00")),
			).Query(esper.StatementName("delete")))
		}
	}
	return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
}

func infraNWFVDecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		var payload infraNWFVSupportBean
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return payload, nil
	case "SupportBean_S0":
		var payload infraNWFVBeanS0
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWFVId, step.EventType)
	}
}

func infraNWFVContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
