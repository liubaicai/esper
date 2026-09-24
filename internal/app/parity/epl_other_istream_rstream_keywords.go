package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_other_istream_rstream_keywords.go replays the EPLOtherIStreamRStreamKeywords
// executions assigned to this unit (ords 0, 1 and 9) against the pinned Java
// oracle:
//
//   - rstream-only-om (ord 0, EPLOtherRStreamOnlyOM): 'select rstream * from
//     SupportBean#length(3)' built through the SODA object model
//     (SelectClause.createWildcard(StreamSelector.RSTREAM_ONLY) over a
//     FilterStream length(3) view, named s0 via annotation). Sends a/a/b/d:
//     the first three inserts fire nothing and the fourth expires the first
//     'a' bean, delivered as the single newData row with oldData null.
//   - rstream-only-compile (ord 1, EPLOtherRStreamOnlyCompile): the identical
//     statement reached through eplToModel — zero observable difference, so
//     the same plan and the same single delivery.
//   - rstream-output-snapshot (ord 9, EPLOtherRStreamOutputSnapshot):
//     'select rstream * from SupportBean#time(30 minutes) output snapshot'
//     compileDeploy + undeployAll smoke — no listener, no sends, no records.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java OM/compile legs assert model.toEPL() round-trips the statement
//     text; that compile-time surface has no Go counterpart, so the scenario
//     pins the byte-exact EPL in case metadata instead.
//   - undeployAll between executions is modelled as separate cases with fresh
//     runtimes; the snapshot case's deploy/undeploy-all steps drive the Go
//     deployment lifecycle explicitly.

const (
	eplOtherIStreamRStreamKeywordsID         = "epl-other-istream-rstream-keywords"
	eplOtherIStreamRStreamKeywordsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplOtherIStreamRStreamKeywordsSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherIStreamRStreamKeywords.java"
)

const eplOtherIStreamRStreamKeywordsDescription = "EPLOtherIStreamRStreamKeywords ord 0/1/9 replay (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): rstream-only-om (ord 0, EPLOtherRStreamOnlyOM) builds 'select rstream * from SupportBean#length(3)' via the SODA object model (SelectClause wildcard RSTREAM_ONLY over a FilterStream length(3) view, named s0 via annotation); rstream-only-compile (ord 1, EPLOtherRStreamOnlyCompile) reaches the identical statement through eplToModel — zero observable difference; rstream-output-snapshot (ord 9, EPLOtherRStreamOutputSnapshot) compileDeploys 'select rstream * from SupportBean#time(30 minutes) output snapshot' then undeployAll with no listener and no sends. Each rstream-only case sends a/a/b/d: the first three inserts fire nothing and the fourth expires the first 'a' bean, delivered as the single newData row with oldData null. The Java OM toEPL round-trip assertion has no Go counterpart (approved difference); undeployAll between executions is modelled as fresh runtimes per case."

// Byte-exact EPL transcriptions of EPLOtherIStreamRStreamKeywords.java:
// stmtText lines 51/80 (ords 0/1 — the OM case's toEPL round-trip text) and
// the snapshot EPL line 44 (ord 9).
const (
	eplOtherIStreamRStreamKeywordsEPLRStreamOnly = "select rstream * from SupportBean#length(3)"
	eplOtherIStreamRStreamKeywordsEPLSnapshot    = "select rstream * from SupportBean#time(30 minutes) output snapshot"
)

var eplOtherIStreamRStreamKeywordsCaseObservations = []string{
	"listener; 'select rstream * from SupportBean#length(3)' deployed via the SODA object model: sends a/a/b fire nothing (insert stream suppressed), send d expires the first 'a' bean which arrives as the single newData row {theString:a,intPrimitive:2} with oldData null",
	"listener; 'select rstream * from SupportBean#length(3)' deployed via eplToModel: sends a/a/b fire nothing (insert stream suppressed), send d expires the first 'a' bean which arrives as the single newData row {theString:a,intPrimitive:2} with oldData null",
	"none; 'select rstream * from SupportBean#time(30 minutes) output snapshot' compileDeploy + undeployAll smoke — no listener, no sends, no records",
}

var eplOtherIStreamRStreamKeywordsCaseEPLs = []string{
	eplOtherIStreamRStreamKeywordsEPLRStreamOnly,
	eplOtherIStreamRStreamKeywordsEPLRStreamOnly,
	eplOtherIStreamRStreamKeywordsEPLSnapshot,
}

var (
	eplOtherIStreamRStreamKeywordsJavaRuntimeIDs = []string{
		"java-runtime-162eb033cafbb4532e4f",
		"java-runtime-9cff0da41992171acf55",
		"java-runtime-8b199ae3084d181b5b02",
	}
	eplOtherIStreamRStreamKeywordsJavaExecutions = []string{
		"EPLOtherRStreamOnlyOM",
		"EPLOtherRStreamOnlyCompile",
		"EPLOtherRStreamOutputSnapshot",
	}
	eplOtherIStreamRStreamKeywordsJavaStaticIDs = []string{
		"java-e56acdb23677f35e3414",
		"java-04de14f530434ce4c3a0",
		"java-418f46e08d6693df74be",
	}
	eplOtherIStreamRStreamKeywordsJavaFlags = []string{}
	eplOtherIStreamRStreamKeywordsCases     = []string{
		"rstream-only-om",
		"rstream-only-compile",
		"rstream-output-snapshot",
	}
	eplOtherIStreamRStreamKeywordsOrdinals = []int{0, 1, 9}
	eplOtherIStreamRStreamKeywordsSources  = []string{eplOtherIStreamRStreamKeywordsSource}
)

var eplOtherIStreamRStreamKeywordsCaseRuntimeIDs = map[string]string{
	"rstream-only-om":         "java-runtime-162eb033cafbb4532e4f",
	"rstream-only-compile":    "java-runtime-9cff0da41992171acf55",
	"rstream-output-snapshot": "java-runtime-8b199ae3084d181b5b02",
}

// eplOtherIStreamRStreamKeywordsCaseSteps pins the complete step sequence per
// case as op|case|statement|eventType|epl|payload keys so the loader asserts
// the scenario file matches the contract. The rstream-only cases deploy s0
// before the sends (mirroring compileDeploy+addListener); the snapshot case
// carries explicit deploy/undeploy-all steps for its compileDeploy+undeployAll
// smoke.
var eplOtherIStreamRStreamKeywordsCaseSteps = map[string][]string{
	"rstream-only-om": {
		`send|rstream-only-om||SupportBean||{"theString":"a","intPrimitive":2}`,
		`send|rstream-only-om||SupportBean||{"theString":"a","intPrimitive":2}`,
		`send|rstream-only-om||SupportBean||{"theString":"b","intPrimitive":2}`,
		`send|rstream-only-om||SupportBean||{"theString":"d","intPrimitive":2}`,
	},
	"rstream-only-compile": {
		`send|rstream-only-compile||SupportBean||{"theString":"a","intPrimitive":2}`,
		`send|rstream-only-compile||SupportBean||{"theString":"a","intPrimitive":2}`,
		`send|rstream-only-compile||SupportBean||{"theString":"b","intPrimitive":2}`,
		`send|rstream-only-compile||SupportBean||{"theString":"d","intPrimitive":2}`,
	},
	"rstream-output-snapshot": {
		"deploy|rstream-output-snapshot|s0||select rstream * from SupportBean#time(30 minutes) output snapshot|",
		"undeploy-all|rstream-output-snapshot||||",
	},
}

// istreamRStreamKeywordsBean mirrors SupportBean's asserted fields.
type istreamRStreamKeywordsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// runEplOtherIStreamRStreamKeywordsScenario replays the assigned
// EPLOtherIStreamRStreamKeywords executions: the two rstream-only variants
// (SODA object model and eplToModel — observably identical) plus the
// output-snapshot compile/deploy/undeploy smoke.
func runEplOtherIStreamRStreamKeywordsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplOtherIStreamRStreamKeywordsCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplOtherIStreamRStreamKeywordsCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl-other-istream-rstream-keywords case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl-other-istream-rstream-keywords scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplOtherIStreamRStreamKeywordsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plan, err := buildEplOtherIStreamRStreamKeywordsCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplOtherIStreamRStreamKeywordsCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var deployment *esper.Deployment
	var sequence uint64
	deploy := func() error {
		if deployment != nil {
			return fmt.Errorf("epl-other-istream-rstream-keywords case %q deploys twice", caseName)
		}
		deployed, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		deployment = deployed
		return nil
	}
	subscribe := func() error {
		for _, statement := range deployment.Statements() {
			if statement.Name() != "s0" {
				continue
			}
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequence++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  sequence,
					Time:      engine.Now().UTC().Format(time.RFC3339Nano),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}

	// The rstream-only cases deploy s0 up front, mirroring the Java
	// compileDeploy+addListener before the sends; the snapshot case deploys
	// on its explicit deploy step and attaches no listener.
	if caseName != "rstream-output-snapshot" {
		if err := deploy(); err != nil {
			return trace, err
		}
		if err := subscribe(); err != nil {
			return trace, err
		}
	}

	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			payload, err := decodeEplOtherIStreamRStreamKeywordsPayload(step)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "deploy":
			if err := deploy(); err != nil {
				return trace, err
			}
		case "undeploy-all":
			if deployment != nil {
				if err := deployment.Undeploy(ctx); err != nil {
					return trace, err
				}
				deployment = nil
			}
		default:
			return trace, fmt.Errorf("unsupported epl-other-istream-rstream-keywords step op %q", step.Op)
		}
	}
	return trace, nil
}

// buildEplOtherIStreamRStreamKeywordsCase registers SupportBean and builds
// the case's single-statement plan: 'select rstream *' over length(3) for the
// rstream-only variants (WithRemoveStreamOnly delivers expired events in the
// new-data slot, matching Java's rstream selector) and the time(30 minutes)
// output-snapshot statement for the smoke case.
func buildEplOtherIStreamRStreamKeywordsCase(env *esper.Environment, caseName string) (esper.Plan, error) {
	if _, err := esper.RegisterStruct[istreamRStreamKeywordsBean](env, "SupportBean"); err != nil {
		return esper.Plan{}, err
	}
	source := esper.From[istreamRStreamKeywordsBean](env, "SupportBean")
	switch caseName {
	case "rstream-only-om", "rstream-only-compile":
		// select rstream * from SupportBean#length(3) — the OM and
		// eplToModel legs compile the identical statement, so one plan
		// covers both cases.
		return env.Build(source.Window(esper.LengthWindow(3)).
			Query(esper.StatementName("s0"), esper.WithRemoveStreamOnly()))
	case "rstream-output-snapshot":
		// select rstream * from SupportBean#time(30 minutes) output
		// snapshot — compile/deploy/undeploy smoke with no listener.
		return env.Build(source.Window(esper.TimeWindow(30*time.Minute)).
			Query(esper.StatementName("s0"), esper.WithRemoveStreamOnly(), esper.WithOutput(esper.OutputSnapshot())))
	}
	return esper.Plan{}, fmt.Errorf("unknown epl-other-istream-rstream-keywords case %q", caseName)
}

func decodeEplOtherIStreamRStreamKeywordsPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value istreamRStreamKeywordsBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported epl-other-istream-rstream-keywords event type %q", step.EventType)
	}
}

// loadEplOtherIStreamRStreamKeywordsScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys, the
// exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadEplOtherIStreamRStreamKeywordsScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplOtherIStreamRStreamKeywordsID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplOtherIStreamRStreamKeywordsID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherIStreamRStreamKeywordsID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherIStreamRStreamKeywordsID, err)
	}
	if err := requireEplOtherIStreamRStreamKeywordsFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplOtherIStreamRStreamKeywordsID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplOtherIStreamRStreamKeywordsID ||
		metadata.Description != eplOtherIStreamRStreamKeywordsDescription ||
		metadata.JavaCommit != eplOtherIStreamRStreamKeywordsJavaCommit ||
		metadata.JavaSource != eplOtherIStreamRStreamKeywordsSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplOtherIStreamRStreamKeywordsID)
	}
	if err := validateEplOtherIStreamRStreamKeywordsStringArray(root["javaRuntimes"], eplOtherIStreamRStreamKeywordsJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherIStreamRStreamKeywordsStringArray(root["javaNames"], eplOtherIStreamRStreamKeywordsJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherIStreamRStreamKeywordsStringArray(root["javaStaticIds"], eplOtherIStreamRStreamKeywordsJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplOtherIStreamRStreamKeywordsStringArray(root["javaFlags"], eplOtherIStreamRStreamKeywordsJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplOtherIStreamRStreamKeywordsCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplOtherIStreamRStreamKeywordsID, len(eplOtherIStreamRStreamKeywordsCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplOtherIStreamRStreamKeywordsFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != eplOtherIStreamRStreamKeywordsCases[index] ||
			definition.Ordinal != eplOtherIStreamRStreamKeywordsOrdinals[index] ||
			definition.RuntimeID != eplOtherIStreamRStreamKeywordsJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplOtherIStreamRStreamKeywordsJavaExecutions[index] ||
			definition.Observation != eplOtherIStreamRStreamKeywordsCaseObservations[index] ||
			definition.EPL != eplOtherIStreamRStreamKeywordsCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplOtherIStreamRStreamKeywordsID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eplOtherIStreamRStreamKeywordsID, err)
	}
	offset := 0
	for _, caseName := range eplOtherIStreamRStreamKeywordsCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eplOtherIStreamRStreamKeywordsID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherIStreamRStreamKeywordsID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eplOtherIStreamRStreamKeywordsID, offset, caseName)
		}
		if _, err := eplOtherIStreamRStreamKeywordsStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherIStreamRStreamKeywordsID, offset, err)
		}
		offset++
		want, ok := eplOtherIStreamRStreamKeywordsCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eplOtherIStreamRStreamKeywordsID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eplOtherIStreamRStreamKeywordsID, caseName)
		}
		for _, pinned := range want {
			key, err := eplOtherIStreamRStreamKeywordsStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplOtherIStreamRStreamKeywordsID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eplOtherIStreamRStreamKeywordsID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eplOtherIStreamRStreamKeywordsID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplOtherIStreamRStreamKeywordsID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplOtherIStreamRStreamKeywordsStepKey renders one raw step as its pinned
// key: op|case|statement|eventType|epl|payload with the payload compacted.
// Unknown fields on the step object are rejected per op.
func eplOtherIStreamRStreamKeywordsStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op        string          `json:"op"`
		Case      string          `json:"case"`
		Statement string          `json:"statement"`
		EventType string          `json:"eventType"`
		Epl       string          `json:"epl"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"send":         {"op", "case", "eventType", "payload"},
		"undeploy-all": {"op", "case"},
	}
	fields, ok := allowed[step.Op]
	if !ok {
		return "", fmt.Errorf("step has unsupported op %q", step.Op)
	}
	for field := range object {
		found := false
		for _, name := range fields {
			if field == name {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payloadText := ""
	if step.Op == "send" {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		payloadText = compacted.String()
	}
	return step.Op + "|" + step.Case + "|" + step.Statement + "|" + step.EventType +
		"|" + step.Epl + "|" + payloadText, nil
}

func requireEplOtherIStreamRStreamKeywordsFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eplOtherIStreamRStreamKeywordsID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eplOtherIStreamRStreamKeywordsID, name)
		}
	}
	return nil
}

func validateEplOtherIStreamRStreamKeywordsStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// normalizeEplOtherIStreamRStreamKeywordsTrace is the identity normalizer:
// each case attaches a single s0 statement and listener delivery is
// synchronous on the sending thread, so the dispatch order is already the
// canonical record order on both traces.
func normalizeEplOtherIStreamRStreamKeywordsTrace(trace compat.Trace) compat.Trace {
	return trace
}
