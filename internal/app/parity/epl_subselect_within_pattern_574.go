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

// Parity coverage for EPLSubselectWithinPattern ords 1-4: subqueries inside
// pattern filters and stream filters over the SupportBean_S0/S1/S2 alphabet.
// Nine spellings replayed across the four Java executions.
//
// Covered executions (all variant collection:executions(), flags []):
//   - ord 1 EPLSubselectCorrelated java-runtime-3d9d3a714bd642e784bd
//     (3 spellings; tryAssertionCorrelated interleave for the two exists
//     spellings, plus the S2-preload scene-two for the followed-by scalar
//     subquery gate)
//   - ord 2 EPLSubselectAggregation java-runtime-9a15ec8213060ac0185c
//     (rolling sum(id) over S1#length(2) gate sequence)
//   - ord 3 EPLSubselectSubqueryAgainstNamedWindowInUDFInPattern
//     java-runtime-3c1d7cc144c168f6a0d6 (UDF arg = subquery over named
//     window inside a pattern predicate)
//   - ord 4 EPLSubselectFilterPatternNamedWindowNoAlias
//     java-runtime-0db35509697665c0960e (4 spellings sharing tryAssertion)
//
// Known representational notes:
//   - Java milestone() savepoints restore identical state for these
//     non-contextual executions and are omitted (573 precedent).
//   - Esper auto-names subqueries (stream1/stream2/subselect_N); the fluent
//     Go API needs no inner alias, and the outer "as stream0"/"as s0"
//     stream aliases carry no observable — dropped.
//   - Java's untagged pattern atom in ord 3 maps to PatternFrom with an
//     empty tag; select * over the untagged atom yields the same empty {}
//     row Java produces.
//
// Correlation scope note: ord 1 spelling 3 correlates the subquery's
// where clause to an EARLIER pattern tag (sp0.p00); the Go subquery eval
// context inside a pattern-filter limb resolves the enclosing pattern's
// tag bindings, so TagField("sp0", "p00") addresses the bound sp0 event
// exactly as the EPL reference does.

const eplSubselectWithinPattern574ID = "epl-subselect-within-pattern-574"

const eplSubselectWithinPattern574Description = "EPLSubselectWithinPattern ords 1-4: subqueries within pattern and stream filters over SupportBean_S0/S1/S2 — correlated exists over keepall (pattern and filter spellings), a followed-by scalar-subquery gate, rolling sum aggregation over length(2), a named-window subquery as UDF argument in a pattern, and the lastevent/named-window in-subquery quartet (including the followed-by scalar-subquery gate correlated to the earlier sp0 tag)."

const eplSubselectWithinPattern574JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const eplSubselectWithinPattern574JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectWithinPattern.java"

// Byte-exact EPL pins (EPLSubselectWithinPattern.java; concatenated string
// segments joined exactly — no whitespace at the join points).
const (
	eplSubselectWithinPattern574CorrelatedPatternEPL = "@name('s0') select sp1.id as myid from pattern[every sp1=SupportBean_S0(exists (select * from SupportBean_S1#keepall as stream1 where stream1.p10 = sp1.p00))]"

	eplSubselectWithinPattern574CorrelatedFilterEPL = "@name('s0') select id as myid from SupportBean_S0(exists (select stream1.id from SupportBean_S1#keepall as stream1 where stream1.p10 = stream0.p00)) as stream0"

	eplSubselectWithinPattern574CorrelatedFollowedByEPL = "@name('s0') select sp0.p00||'+'||sp1.p10 as myid from pattern[every sp0=SupportBean_S0 -> sp1=SupportBean_S1(p11 = (select stream2.p21 from SupportBean_S2#keepall as stream2 where stream2.p20 = sp0.p00))]"

	eplSubselectWithinPattern574AggregationEPL = "@name('s0') select * from SupportBean_S0(id = (select sum(id) from SupportBean_S1#length(2)))"

	eplSubselectWithinPattern574NamedWindowUDFEPL = "create window MyWindowSNW#unique(p00)#keepall as SupportBean_S0;\n" +
		"@name('s0') select * from pattern[SupportBean_S1(supportSingleRowFunction((select * from MyWindowSNW)))];\n"

	eplSubselectWithinPattern574PatternLastEventEPL = "@name('s0') select s.id as myid from pattern [every s=SupportBean_S0(p00 in (select p10 from SupportBean_S1#lastevent))]"

	eplSubselectWithinPattern574FilterLastEventEPL = "@name('s0') select id as myid from SupportBean_S0(p00 in (select p10 from SupportBean_S1#lastevent))"

	eplSubselectWithinPattern574FilterNamedWindowEPL = "create window MyS1Window#lastevent as select * from SupportBean_S1;\n" +
		"insert into MyS1Window select * from SupportBean_S1;\n" +
		"@name('s0') select id as myid from SupportBean_S0(p00 in (select p10 from MyS1Window))"

	eplSubselectWithinPattern574PatternNamedWindowEPL = "create window MyS1Window#lastevent as select * from SupportBean_S1;\n" +
		"insert into MyS1Window select * from SupportBean_S1;\n" +
		"@name('s0') select s.id as myid from pattern [every s=SupportBean_S0(p00 in (select p10 from MyS1Window))];\n"
)

var (
	eplSubselectWithinPattern574JavaSources = []string{
		eplSubselectWithinPattern574JavaSource,
	}
	eplSubselectWithinPattern574JavaRuntimeIDs = []string{
		"java-runtime-3d9d3a714bd642e784bd",
		"java-runtime-9a15ec8213060ac0185c",
		"java-runtime-3c1d7cc144c168f6a0d6",
		"java-runtime-0db35509697665c0960e",
	}
	eplSubselectWithinPattern574JavaExecutions = []string{
		"EPLSubselectCorrelated",
		"EPLSubselectAggregation",
		"EPLSubselectSubqueryAgainstNamedWindowInUDFInPattern",
		"EPLSubselectFilterPatternNamedWindowNoAlias",
	}
	eplSubselectWithinPattern574JavaStaticIDs = []string{
		"java-495107e31d1fe86086ab",
		"java-495107e31d1fe86086ab",
		"java-495107e31d1fe86086ab",
		"java-495107e31d1fe86086ab",
	}
	eplSubselectWithinPattern574CaseNames = []string{
		"correlated-pattern-exists",
		"correlated-filter-exists",
		"correlated-followed-by-scalar",
		"aggregation",
		"named-window-udf",
		"noalias-pattern-lastevent",
		"noalias-filter-lastevent",
		"noalias-filter-named-window",
		"noalias-pattern-named-window",
	}
)

// eplSubselectWithinPattern574CaseSpec pins one case: Java execution
// identity and the byte-exact EPL script the deploy step carries.
type eplSubselectWithinPattern574CaseSpec struct {
	name           string
	ordinal        int
	runtimeIndex   int
	epl            string
	sendsS2Preload bool
}

var eplSubselectWithinPattern574CaseSpecs = []eplSubselectWithinPattern574CaseSpec{
	{name: "correlated-pattern-exists", ordinal: 1, runtimeIndex: 0, epl: eplSubselectWithinPattern574CorrelatedPatternEPL},
	{name: "correlated-filter-exists", ordinal: 1, runtimeIndex: 0, epl: eplSubselectWithinPattern574CorrelatedFilterEPL},
	{name: "correlated-followed-by-scalar", ordinal: 1, runtimeIndex: 0, epl: eplSubselectWithinPattern574CorrelatedFollowedByEPL, sendsS2Preload: true},
	{name: "aggregation", ordinal: 2, runtimeIndex: 1, epl: eplSubselectWithinPattern574AggregationEPL},
	{name: "named-window-udf", ordinal: 3, runtimeIndex: 2, epl: eplSubselectWithinPattern574NamedWindowUDFEPL},
	{name: "noalias-pattern-lastevent", ordinal: 4, runtimeIndex: 3, epl: eplSubselectWithinPattern574PatternLastEventEPL},
	{name: "noalias-filter-lastevent", ordinal: 4, runtimeIndex: 3, epl: eplSubselectWithinPattern574FilterLastEventEPL},
	{name: "noalias-filter-named-window", ordinal: 4, runtimeIndex: 3, epl: eplSubselectWithinPattern574FilterNamedWindowEPL},
	{name: "noalias-pattern-named-window", ordinal: 4, runtimeIndex: 3, epl: eplSubselectWithinPattern574PatternNamedWindowEPL},
}

func eplSubselectWithinPattern574CaseSpecFor(name string) (eplSubselectWithinPattern574CaseSpec, bool) {
	for _, spec := range eplSubselectWithinPattern574CaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return eplSubselectWithinPattern574CaseSpec{}, false
}

// Host objects: the SupportBean_S0/S1/S2 property sets the executions use.
// S0 carries a `value` int property in Java but nothing in this file reads
// or writes it and Java's `select *` row for ord 2 does not project it, so
// the Go struct omits it.
type eplSubselectWithinPattern574S0 struct {
	ID  int     `esper:"id"`
	P00 *string `esper:"p00"`
	P01 *string `esper:"p01"`
	P02 *string `esper:"p02"`
	P03 *string `esper:"p03"`
}

type eplSubselectWithinPattern574S1 struct {
	ID  int     `esper:"id"`
	P10 *string `esper:"p10"`
	P11 *string `esper:"p11"`
	P12 *string `esper:"p12"`
	P13 *string `esper:"p13"`
}

type eplSubselectWithinPattern574S2 struct {
	ID  int     `esper:"id"`
	P20 *string `esper:"p20"`
	P21 *string `esper:"p21"`
	P22 *string `esper:"p22"`
	P23 *string `esper:"p23"`
}

// eplSubselectWithinPattern574SendPin pins one send step's event type and
// exact payload field values. Fields are pointers so "absent" and "set" are
// distinct payload shapes (mirroring the Java constructor overloads).
type eplSubselectWithinPattern574SendPin struct {
	event string
	id    int
	a     *string // first optional string arg (p00/p10/p20)
	b     *string // second optional string arg (p11/p21)
}

func eplSubselectWithinPattern574S0Send(id int, p00 *string) eplSubselectWithinPattern574SendPin {
	return eplSubselectWithinPattern574SendPin{event: "SupportBean_S0", id: id, a: p00}
}

func eplSubselectWithinPattern574S1Send(id int, p10, p11 *string) eplSubselectWithinPattern574SendPin {
	return eplSubselectWithinPattern574SendPin{event: "SupportBean_S1", id: id, a: p10, b: p11}
}

func eplSubselectWithinPattern574S2Send(id int, p20, p21 *string) eplSubselectWithinPattern574SendPin {
	return eplSubselectWithinPattern574SendPin{event: "SupportBean_S2", id: id, a: p20, b: p21}
}

func strp(s string) *string { return &s }

// eplSubselectWithinPattern574SendSequences pins the send legs per case in
// Java source order. The tryAssertionCorrelated legs (S0/S1 interleave
// hitting myid 5/6/9) drive both exists spellings; tryAssertion (myid 5,
// then 10) drives all four NoAlias spellings.
func eplSubselectWithinPattern574SendSequences(caseName string) []eplSubselectWithinPattern574SendPin {
	switch caseName {
	case "correlated-pattern-exists", "correlated-filter-exists":
		// tryAssertionCorrelated
		return []eplSubselectWithinPattern574SendPin{
			eplSubselectWithinPattern574S0Send(1, strp("A")),
			eplSubselectWithinPattern574S1Send(2, strp("A"), nil),
			eplSubselectWithinPattern574S0Send(3, strp("B")),
			eplSubselectWithinPattern574S1Send(4, strp("C"), nil),
			eplSubselectWithinPattern574S0Send(5, strp("C")), // hit: myid 5
			eplSubselectWithinPattern574S0Send(6, strp("A")), // hit: myid 6
			eplSubselectWithinPattern574S0Send(7, strp("D")),
			eplSubselectWithinPattern574S1Send(8, strp("E"), nil),
			eplSubselectWithinPattern574S0Send(9, strp("C")), // hit: myid 9
		}
	case "correlated-followed-by-scalar":
		// scene-two: S2 X/A,Y/B,Z/C preload; S0 A/Y/C silent;
		// S1(4,B,B) -> "Y+B"; then C/B, X/A, A/C, A/C silent legs.
		return []eplSubselectWithinPattern574SendPin{
			eplSubselectWithinPattern574S2Send(21, strp("X"), strp("A")),
			eplSubselectWithinPattern574S2Send(22, strp("Y"), strp("B")),
			eplSubselectWithinPattern574S2Send(23, strp("Z"), strp("C")),
			eplSubselectWithinPattern574S0Send(1, strp("A")),
			eplSubselectWithinPattern574S0Send(2, strp("Y")),
			eplSubselectWithinPattern574S0Send(3, strp("C")),
			eplSubselectWithinPattern574S1Send(4, strp("B"), strp("B")), // hit: myid "Y+B"
			eplSubselectWithinPattern574S1Send(4, strp("B"), strp("C")),
			eplSubselectWithinPattern574S1Send(5, strp("C"), strp("B")),
			eplSubselectWithinPattern574S1Send(6, strp("X"), strp("A")),
			eplSubselectWithinPattern574S1Send(7, strp("A"), strp("C")),
		}
	case "aggregation":
		// rolling-sum gate: sum(id) over S1#length(2) equals candidate id.
		return []eplSubselectWithinPattern574SendPin{
			eplSubselectWithinPattern574S0Send(1, nil),
			eplSubselectWithinPattern574S1Send(1, nil, nil),
			eplSubselectWithinPattern574S0Send(1, nil), // hit
			eplSubselectWithinPattern574S1Send(3, nil, nil),
			eplSubselectWithinPattern574S0Send(3, nil),
			eplSubselectWithinPattern574S0Send(5, nil),
			eplSubselectWithinPattern574S0Send(4, nil), // hit
			eplSubselectWithinPattern574S1Send(10, nil, nil),
			eplSubselectWithinPattern574S0Send(10, nil),
			eplSubselectWithinPattern574S0Send(3, nil),
			eplSubselectWithinPattern574S0Send(13, nil), // hit
		}
	case "named-window-udf":
		return []eplSubselectWithinPattern574SendPin{
			eplSubselectWithinPattern574S1Send(1, nil, nil), // hit
		}
	default:
		// tryAssertion, shared by the four NoAlias spellings.
		return []eplSubselectWithinPattern574SendPin{
			eplSubselectWithinPattern574S0Send(1, strp("A")),
			eplSubselectWithinPattern574S1Send(2, strp("A"), nil),
			eplSubselectWithinPattern574S0Send(3, strp("B")),
			eplSubselectWithinPattern574S1Send(4, strp("C"), nil),
			eplSubselectWithinPattern574S0Send(5, strp("C")), // hit: myid 5
			eplSubselectWithinPattern574S0Send(6, strp("A")),
			eplSubselectWithinPattern574S0Send(7, strp("D")),
			eplSubselectWithinPattern574S1Send(8, strp("E"), nil),
			eplSubselectWithinPattern574S0Send(9, strp("C")),
			eplSubselectWithinPattern574S0Send(10, strp("E")), // hit: myid 10
		}
	}
}

func loadEplSubselectWithinPattern574Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplSubselectWithinPattern574ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplSubselectWithinPattern574ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplSubselectWithinPattern574ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplSubselectWithinPattern574ID, err)
	}
	if err := requireEplSubselectWithinPattern574Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplSubselectWithinPattern574ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplSubselectWithinPattern574ID ||
		metadata.Description != eplSubselectWithinPattern574Description ||
		metadata.JavaCommit != eplSubselectWithinPattern574JavaCommit ||
		metadata.JavaSource != eplSubselectWithinPattern574JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplSubselectWithinPattern574ID)
	}
	if err := validateEplSubselectWithinPattern574StringArray(root["javaRuntimes"], eplSubselectWithinPattern574JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectWithinPattern574StringArray(root["javaNames"], eplSubselectWithinPattern574JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectWithinPattern574StringArray(root["javaStaticIds"], eplSubselectWithinPattern574JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectWithinPattern574StringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplSubselectWithinPattern574CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly nine cases", eplSubselectWithinPattern574ID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplSubselectWithinPattern574Fields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := eplSubselectWithinPattern574CaseSpecs[index]
		if definition.Case != spec.name ||
			definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != eplSubselectWithinPattern574JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != eplSubselectWithinPattern574JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplSubselectWithinPattern574ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplSubselectWithinPattern574ID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireEplSubselectWithinPattern574Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplSubselectWithinPattern574Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplSubselectWithinPattern574Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplSubselectWithinPattern574Payload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplSubselectWithinPattern574Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateEplSubselectWithinPattern574Scenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateEplSubselectWithinPattern574Scenario positionally compares every
// step against the pinned per-case sequences: case marker, one deploy of
// statement s0 with the byte-exact EPL script, the pinned sends, undeploy.
func validateEplSubselectWithinPattern574Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplSubselectWithinPattern574ID {
		return fmt.Errorf("%s scenario shape is not pinned", eplSubselectWithinPattern574ID)
	}
	offset := 0
	for _, spec := range eplSubselectWithinPattern574CaseSpecs {
		sends := eplSubselectWithinPattern574SendSequences(spec.name)
		expected := 1 + len(sends) + 1 // deploy + sends + undeploy-all
		steps := scenario.Steps[offset:]
		if len(steps) < expected+1 {
			return fmt.Errorf("%s case %q steps are truncated", eplSubselectWithinPattern574ID, spec.name)
		}
		if steps[0].Op != "case" || steps[0].Case != spec.name {
			return fmt.Errorf("%s case %q must start with its case marker", eplSubselectWithinPattern574ID, spec.name)
		}
		deploy := steps[1]
		if deploy.Op != "deploy" || deploy.Case != spec.name || deploy.Statement != "s0" || deploy.Epl != spec.epl {
			return fmt.Errorf("%s case %q must deploy s0 with the pinned EPL", eplSubselectWithinPattern574ID, spec.name)
		}
		for index, send := range sends {
			step := steps[index+2]
			if step.Op != "send" || step.Case != spec.name || step.EventType != send.event {
				return fmt.Errorf("%s case %q step %d must send %s", eplSubselectWithinPattern574ID, spec.name, index, send.event)
			}
			if err := eplSubselectWithinPattern574ValidatePayload(step, send); err != nil {
				return fmt.Errorf("%s case %q step %d: %w", eplSubselectWithinPattern574ID, spec.name, index, err)
			}
		}
		undeploy := steps[len(sends)+2]
		if undeploy.Op != "undeploy-all" || undeploy.Case != spec.name {
			return fmt.Errorf("%s case %q must end with undeploy-all", eplSubselectWithinPattern574ID, spec.name)
		}
		offset += expected + 1
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplSubselectWithinPattern574ID)
	}
	return nil
}

// eplSubselectWithinPattern574ValidatePayload pins the exact send payload:
// field set, id, and the optional string args in constructor order.
func eplSubselectWithinPattern574ValidatePayload(step compat.Step, send eplSubselectWithinPattern574SendPin) error {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return err
	}
	var names []string
	var first, second string
	switch send.event {
	case "SupportBean_S0":
		names = append(names, "id")
		if send.a != nil {
			names = append(names, "p00")
			first = "p00"
		}
	case "SupportBean_S1":
		names = append(names, "id")
		if send.a != nil {
			names = append(names, "p10")
			first = "p10"
		}
		if send.b != nil {
			names = append(names, "p11")
			second = "p11"
		}
	case "SupportBean_S2":
		names = append(names, "id", "p20", "p21")
		first = "p20"
		second = "p21"
	default:
		return fmt.Errorf("unsupported event type %q", send.event)
	}
	if err := requireEplSubselectWithinPattern574Fields(fields, names...); err != nil {
		return err
	}
	var id int
	if err := json.Unmarshal(fields["id"], &id); err != nil || id != send.id {
		return fmt.Errorf("payload id is not pinned")
	}
	check := func(field string, want *string) error {
		var got string
		if err := json.Unmarshal(fields[field], &got); err != nil || want == nil || got != *want {
			return fmt.Errorf("payload %s is not pinned", field)
		}
		return nil
	}
	if first != "" {
		if err := check(first, send.a); err != nil {
			return err
		}
	}
	if second != "" {
		if err := check(second, send.b); err != nil {
			return err
		}
	}
	return nil
}

func runEplSubselectWithinPattern574Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplSubselectWithinPattern574Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for _, spec := range eplSubselectWithinPattern574CaseSpecs {
		length := 1 + 1 + len(eplSubselectWithinPattern574SendSequences(spec.name)) + 1
		caseSteps := scenario.Steps[offset : offset+length]
		offset += length
		caseTrace, err := runEplSubselectWithinPattern574Case(ctx, caseSteps, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplSubselectWithinPattern574ID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplSubselectWithinPattern574Case(ctx context.Context, steps []compat.Step, spec eplSubselectWithinPattern574CaseSpec) (compat.Trace, error) {
	env := esper.NewEnvironment()
	s0Schema, err := esper.RegisterStruct[eplSubselectWithinPattern574S0](env, "SupportBean_S0")
	if err != nil {
		return compat.Trace{}, err
	}
	s1Schema, err := esper.RegisterStruct[eplSubselectWithinPattern574S1](env, "SupportBean_S1")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectWithinPattern574S2](env, "SupportBean_S2"); err != nil {
		return compat.Trace{}, err
	}
	// Named windows are created at the environment level before the engine
	// (the engine snapshots environment windows at construction), matching
	// the Java script's create window deployment.
	switch spec.name {
	case "named-window-udf":
		if _, err := esper.CreateNamedWindow(env, "MyWindowSNW", s0Schema,
			esper.NamedWindowRetention(esper.IntersectWindows(
				esper.Unique(esper.Field[any, string]("p00")), esper.KeepAll()))); err != nil {
			return compat.Trace{}, err
		}
	case "noalias-filter-named-window", "noalias-pattern-named-window":
		if _, err := esper.CreateNamedWindow(env, "MyS1Window", s1Schema,
			esper.NamedWindowRetention(esper.LastEvent())); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplSubselectWithinPattern574JavaRuntimeIDs[spec.runtimeIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := uint64(0)
	trace := compat.Trace{Version: "esper-parity/v1", ID: eplSubselectWithinPattern574ID}
	record := func(batch esper.ResultBatch) {
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			plans, err := eplSubselectWithinPattern574Plans(env, spec)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for index, plan := range plans {
				deployment, err := engine.Deploy(ctx, plan)
				if err != nil {
					return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
				}
				// s0 is the last plan in every case (feeder inserts deploy first).
				if index == len(plans)-1 {
					statements := deployment.Statements()
					if len(statements) != 1 || statements[0].Name() != "s0" {
						return compat.Trace{}, fmt.Errorf("expected one s0 statement")
					}
					if _, err := statements[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "send":
			payload, err := decodeEplSubselectWithinPattern574Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
		}
	}
	return trace, nil
}

// eplSubselectWithinPattern574Plans builds the deployment plans for the
// case: optional named-window feeder insert, then the s0 statement. The
// subquery source windows mirror the EPL (#keepall/#lastevent/#length(2)
// on the event stream, or the named window itself).
func eplSubselectWithinPattern574Plans(env *esper.Environment, spec eplSubselectWithinPattern574CaseSpec) ([]esper.Plan, error) {
	s1Select := func() esper.RecordStream {
		return esper.Select(esper.From[eplSubselectWithinPattern574S1](env, "SupportBean_S1"))
	}
	s1KeepAll := s1Select().Window(esper.KeepAll())
	s0From := esper.From[eplSubselectWithinPattern574S0](env, "SupportBean_S0")
	s1From := esper.From[eplSubselectWithinPattern574S1](env, "SupportBean_S1")

	var query esper.Query
	switch spec.name {
	case "correlated-pattern-exists":
		query = esper.PatternFrom(s0From, "sp1",
			esper.SubqueryExists(s1KeepAll, esper.Equal[*string](
				esper.Field[any, *string]("p10"), esper.OuterField[*string]("p00")))).
			Every().
			Select(esper.Alias("myid", esper.TagField[int]("sp1", "id"))).
			Query(esper.StatementName("s0"))
	case "correlated-filter-exists":
		query = esper.Select(
			s0From.Filter(esper.SubqueryExists(s1KeepAll, esper.Equal[*string](
				esper.Field[any, *string]("p10"), esper.OuterField[*string]("p00")))),
			esper.Alias("myid", esper.Field[eplSubselectWithinPattern574S0, int]("id")),
		).Query(esper.StatementName("s0"))
	case "correlated-followed-by-scalar":
		// TagField("sp0", "p00") inside the subquery predicate resolves the
		// bound earlier limb: subquery eval contexts propagate the
		// enclosing pattern's tag bindings, matching EPL's tag-correlated
		// subquery scope.
		s2KeepAll := esper.Select(esper.From[eplSubselectWithinPattern574S2](env, "SupportBean_S2")).Window(esper.KeepAll())
		query = esper.PatternFrom(s0From, "sp0", esper.Literal(true)).
			Every().
			Then(esper.PatternFrom(s1From, "sp1",
				esper.Equal[*string](
					esper.Field[eplSubselectWithinPattern574S1, *string]("p11"),
					esper.SubqueryValue[*string](s2KeepAll,
						esper.Field[any, *string]("p21"),
						esper.Equal[*string](esper.Field[any, *string]("p20"), esper.TagField[*string]("sp0", "p00")))))).
			Select(esper.Alias("myid", esper.ConcatOf(
				esper.TagField[*string]("sp0", "p00"),
				esper.Literal("+"),
				esper.TagField[*string]("sp1", "p10")))).
			Query(esper.StatementName("s0"))
	case "aggregation":
		s1Len2 := s1Select().Window(esper.LengthWindow(2))
		query = esper.Select(
			s0From.Filter(esper.Equal[int](
				esper.Field[eplSubselectWithinPattern574S0, int]("id"),
				esper.Cast[int64, int](esper.SubquerySum[int64](s1Len2, esper.Field[any, int64]("id"))),
			))).Query(esper.StatementName("s0"))
	case "named-window-udf":
		inner := esper.FromNamedWindow(env, "MyWindowSNW")
		udf := esper.Func1[[]esper.Event, bool]("supportSingleRowFunction", func(events []esper.Event) bool {
			return true
		}, esper.SubqueryEvents(inner))
		// Untagged atom (empty tag) + select *: yields the same empty {}
		// row Java produces for the untagged pattern atom.
		query = esper.PatternFrom(s1From, "", udf).Query(esper.StatementName("s0"))
	default:
		// The four NoAlias spellings share SubqueryIn(p00, inner, p10).
		var inner esper.RecordStream
		switch spec.name {
		case "noalias-filter-named-window", "noalias-pattern-named-window":
			inner = esper.FromNamedWindow(env, "MyS1Window")
		default:
			inner = s1Select().Window(esper.LastEvent())
		}
		predicate := esper.SubqueryIn[*string](
			esper.Field[eplSubselectWithinPattern574S0, *string]("p00"),
			inner, esper.Field[any, *string]("p10"))
		switch spec.name {
		case "noalias-pattern-lastevent", "noalias-pattern-named-window":
			query = esper.PatternFrom(s0From, "s", predicate).
				Every().
				Select(esper.Alias("myid", esper.TagField[int]("s", "id"))).
				Query(esper.StatementName("s0"))
		case "noalias-filter-lastevent", "noalias-filter-named-window":
			query = esper.Select(
				s0From.Filter(predicate),
				esper.Alias("myid", esper.Field[eplSubselectWithinPattern574S0, int]("id")),
			).Query(esper.StatementName("s0"))
		default:
			return nil, fmt.Errorf("unknown %s case %q", eplSubselectWithinPattern574ID, spec.name)
		}
	}

	plans := []esper.Plan{}
	switch spec.name {
	case "noalias-filter-named-window", "noalias-pattern-named-window":
		insertPlan, err := env.Build(esper.OnEvent(s1From).InsertIntoNamedWindow("MyS1Window",
			[]esper.TableAssignment{
				esper.SetColumn("id", esper.Field[eplSubselectWithinPattern574S1, int]("id")),
				esper.SetColumn("p10", esper.Field[eplSubselectWithinPattern574S1, *string]("p10")),
				esper.SetColumn("p11", esper.Field[eplSubselectWithinPattern574S1, *string]("p11")),
				esper.SetColumn("p12", esper.Field[eplSubselectWithinPattern574S1, *string]("p12")),
				esper.SetColumn("p13", esper.Field[eplSubselectWithinPattern574S1, *string]("p13")),
			}...).Query(esper.StatementName("insert")))
		if err != nil {
			return nil, err
		}
		plans = append(plans, insertPlan)
	}
	s0Plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	return append(plans, s0Plan), nil
}

func decodeEplSubselectWithinPattern574Payload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	optional := func(name string, target **string) error {
		raw, ok := fields[name]
		if !ok {
			return nil
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		*target = &value
		return nil
	}
	switch step.EventType {
	case "SupportBean_S0":
		var value eplSubselectWithinPattern574S0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if err := optional("p00", &value.P00); err != nil {
			return nil, err
		}
		return value, nil
	case "SupportBean_S1":
		var value eplSubselectWithinPattern574S1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		if err := optional("p10", &value.P10); err != nil {
			return nil, err
		}
		if err := optional("p11", &value.P11); err != nil {
			return nil, err
		}
		return value, nil
	case "SupportBean_S2":
		var value eplSubselectWithinPattern574S2
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S2: %w", err)
		}
		if err := optional("p20", &value.P20); err != nil {
			return nil, err
		}
		if err := optional("p21", &value.P21); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplSubselectWithinPattern574ID, step.EventType)
	}
}

func requireEplSubselectWithinPattern574Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateEplSubselectWithinPattern574StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}
