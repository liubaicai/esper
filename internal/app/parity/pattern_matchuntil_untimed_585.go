package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for PatternOperatorMatchUntil ords 0/2/3/5: the
// untimed until-array quad — unbounded (and [1:]) repeated-tag collect
// followed by projection, correlation and count over SupportBean and the
// SupportBean_A/B/C letter beans. Four Java executions replayed as four
// cases with the direct compileDeploy -> sendEventBean -> assertListener
// harness: NO clock calls (milestone() savepoints restore identical
// state for these non-contextual executions and are documented no-ops,
// so they carry no steps).
//
// Covered executions (all variant collection:executions(), flags [],
// file java-2169accb9d43755e87e4):
//   - ord 0 PatternMatchUntilSimple java-runtime-93ea4ae0a2ca85d18a0d
//     (case simple): a=SupportBean(intPrimitive=0) until
//     b=SupportBean(intPrimitive=1) — A1(0)/A2(0) collect silently,
//     B1(1) fires ONE row {A1,A2,B1}, and the completed until stays
//     permanently false (the trailing A1(0)/B1(1) pair does NOT refire;
//     single-shot semantics are load-bearing). The EPL pins @Name with
//     a CAPITAL N.
//   - ord 2 PatternSelectArray java-runtime-bd06e4f21e0083fb261d
//     (case select-array): TWO legs over the same
//     a=SupportBean_A until b=SupportBean_B pattern — leg 1 the explicit
//     select a, b, a[0]..a[2]+id projection (a[2] and a[2].id render
//     Null, not an error) and leg 2 the select * wildcard (fragments:
//     a as the collected bean array, b the terminator bean). Java pins
//     assertEqualsExactOrder on the a array and assertSame identity;
//     undeployModuleContaining("s0") separates the legs (undeploy-all
//     equivalent). ONE fire per leg.
//   - ord 3 PatternUseFilter java-runtime-4b8c99341f4a3af06ea9
//     (case use-filter): FIVE select-* legs of
//     a until b -> c(filter correlating against a[i]) — concat
//     ('C' || a[0].id || a[1].id || b.id), equals (theString = a[1].id),
//     in (theString in(a[2].id) with NO space after `in`), triple !=
//     not-in, and between (intPrimitive between a[0].intPrimitive and
//     a[1].intPrimitive, inclusive, over like-'A%'/'B%' filtered
//     SupportBeans). Each leg asserts non-matching probes stay silent
//     then a correlating event fires ONCE.
//   - ord 5 PatternArrayFunctionRepeat java-runtime-602d438d756709deb50e
//     (case array-function-repeat): [1:] a=SupportBean_A until the
//     UNTAGGED SupportBean_B terminator — A1/A2/A3 collect (milestone(0)
//     between A2 and A3 is a no-op), B("A2") fires ONE row
//     {length:3, l2:3} via SupportStaticMethodLib.arrayLength(a) and
//     java.lang.reflect.Array.getLength(a).
//
// Byte-exact EPL pins keep the Java source verbatim: ord 0's @Name is
// capitalized while ords 2/3/5 use @name; the concat leg keeps its
// inner spaces `(id = ('C' || a[0].id || a[1].id || b.id))`; the in-leg
// keeps `in(a[2].id)` with no space; the not-in leg keeps `!=` and the
// between leg keeps `theString like 'A%'`/`'B%'` literal quoting.
//
// Approved mappings (observably identical): Java's a column materializes
// the collected beans as Object[] (select a / select *) which the fluent
// API projects via TagEvents; a[i] beans project via
// ArrayAt(TagEvents) (out-of-range -> Null); arrayLength/getLength map
// to TagCount (the registered Go-style mapping for the static-method
// array functions — no JVM static-method surface exists).
//
// Siblings excluded from this slice (manifest dispositions are
// main-agent owned): ord 1 PatternOp (52-case W-harness over the timed
// mixed event set), ord 4 PatternRepeatUseTags (every-repeated
// correlated sequences plus timer:interval chains needing a clock
// harness), ords 6/7 PatternExpressionBounds/PatternBoundRepeatWithNot
// (variable bounds + timer/not semantics tracked under pattern-basic),
// and ord 8 PatternInvalid (compile-only probes).
const patternMatchUntilUntimed585ID = "pattern-matchuntil-untimed-585"

const patternMatchUntilUntimed585Description = "PatternOperatorMatchUntil ords 0/2/3/5 untimed until-array quad — ord 0 PatternMatchUntilSimple `@Name('s0') select a[0].theString as c0, a[1].theString as c1, b.theString as c2 from pattern [a=SupportBean(intPrimitive=0) until b=SupportBean(intPrimitive=1)]` (capital @Name; A1/A2 silent, B1 fires {A1,A2,B1}, trailing pair permanently silent); ord 2 PatternSelectArray two legs (explicit a/b/a[0..2]+id projection with null a[2], then select *) over a=SupportBean_A until b=SupportBean_B, ONE fire each with exact-order a array; ord 3 PatternUseFilter five select-* legs correlating a follow-on c against a[i] (concat/equals/in/not-in/between); ord 5 PatternArrayFunctionRepeat `[1:] a=SupportBean_A until SupportBean_B` firing {length:3,l2:3} via TagCount (registered mapping for arrayLength/Array.getLength). No clock ops; milestone() savepoints carry no steps."

const patternMatchUntilUntimed585JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const patternMatchUntilUntimed585JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java"

// Byte-exact EPL pins (PatternOperatorMatchUntil.java verbatim): ord 0
// uses @Name (capital N) while ords 2/3/5 use @name; the concat leg
// keeps the spaces inside `(id = ('C' || a[0].id || a[1].id || b.id))`,
// the in leg keeps `in(a[2].id)` with NO space, the not-in leg keeps
// `theString!=a[i].id` without spaces, and ord 5's until terminator is
// UNTAGGED with the `[1:]` lower-bound range.
const (
	patternMatchUntilUntimed585SimpleEPL = "@Name('s0') select a[0].theString as c0, a[1].theString as c1, b.theString as c2 from pattern [a=SupportBean(intPrimitive=0) until b=SupportBean(intPrimitive=1)]"

	patternMatchUntilUntimed585SelectArrayEPL = "@name('s0') select a, b, a[0] as a0, a[0].id as a0Id, a[1] as a1, a[1].id as a1Id, a[2] as a2, a[2].id as a2Id from pattern [a=SupportBean_A until b=SupportBean_B]"

	patternMatchUntilUntimed585SelectWildcardEPL = "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B]"

	patternMatchUntilUntimed585UseFilterConcatEPL = "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> c=SupportBean_C(id = ('C' || a[0].id || a[1].id || b.id))]"

	patternMatchUntilUntimed585UseFilterEqualsEPL = "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> c=SupportBean(theString = a[1].id)]"

	patternMatchUntilUntimed585UseFilterInEPL = "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> c=SupportBean(theString in(a[2].id))]"

	patternMatchUntilUntimed585UseFilterNotInEPL = "@name('s0') select * from pattern [a=SupportBean_A until b=SupportBean_B -> c=SupportBean(theString!=a[0].id and theString!=a[1].id and theString!=a[2].id)]"

	patternMatchUntilUntimed585UseFilterBetweenEPL = "@name('s0') select * from pattern [a=SupportBean(theString like 'A%') until b=SupportBean(theString like 'B%') -> c=SupportBean(intPrimitive between a[0].intPrimitive and a[1].intPrimitive)]"

	patternMatchUntilUntimed585ArrayFunctionRepeatEPL = "@name('s0') select SupportStaticMethodLib.arrayLength(a) as length, java.lang.reflect.Array.getLength(a) as l2 from pattern [[1:] a=SupportBean_A until SupportBean_B]"

	patternMatchUntilUntimed585BeanType = "SupportBean"
	patternMatchUntilUntimed585AType    = "SupportBean_A"
	patternMatchUntilUntimed585BType    = "SupportBean_B"
	patternMatchUntilUntimed585CType    = "SupportBean_C"
)

// Per-case Java identity: each case owns its ordinal's runtimeId; the
// static ids are the per-execution discovery static-candidate rows (ord 0
// carries the file-level id java-2169accb9d43755e87e4 shared with the
// deduplicated inventory row).
var (
	patternMatchUntilUntimed585JavaRuntimeIDs = []string{
		"java-runtime-93ea4ae0a2ca85d18a0d",
		"java-runtime-bd06e4f21e0083fb261d",
		"java-runtime-4b8c99341f4a3af06ea9",
		"java-runtime-602d438d756709deb50e",
	}
	patternMatchUntilUntimed585JavaSources = []string{
		patternMatchUntilUntimed585JavaSource,
		"common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_A.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_C.java",
	}
	patternMatchUntilUntimed585JavaExecutions = []string{
		"PatternMatchUntilSimple",
		"PatternSelectArray",
		"PatternUseFilter",
		"PatternArrayFunctionRepeat",
	}
	patternMatchUntilUntimed585JavaStaticIDs = []string{
		"java-2169accb9d43755e87e4",
		"java-c393b406de406e3ffd1f",
		"java-bf3211b9b2822d0cbbf4",
		"java-7a5adcaeaa460c4641cb",
	}
	patternMatchUntilUntimed585JavaFlags = []string{}
)

// patternMatchUntilUntimed585Bean mirrors SupportBean's asserted
// properties: theString and intPrimitive (the only fields the Java
// assertions and select-* fragments pin).
type patternMatchUntilUntimed585Bean struct {
	TheString    string `json:"theString" esper:"theString"`
	IntPrimitive int    `json:"intPrimitive" esper:"intPrimitive"`
}

// patternMatchUntilUntimed585A/B/C mirror the SupportBean_A/B/C letter
// beans: a single id property each.
type patternMatchUntilUntimed585A struct {
	ID string `json:"id" esper:"id"`
}

type patternMatchUntilUntimed585B struct {
	ID string `json:"id" esper:"id"`
}

type patternMatchUntilUntimed585C struct {
	ID string `json:"id" esper:"id"`
}

// patternMatchUntilUntimed585CaseSpec pins one case: Java execution
// identity (ordinal plus runtimeIndex), the observation text and the
// per-leg byte-exact EPL scripts in deploy order (case.epl in the
// scenario pins the first leg as the representative).
type patternMatchUntilUntimed585CaseSpec struct {
	name         string
	ordinal      int
	runtimeIndex int
	observation  string
	epls         []string
}

var patternMatchUntilUntimed585CaseSpecs = []patternMatchUntilUntimed585CaseSpec{
	{
		name:         "simple",
		ordinal:      0,
		runtimeIndex: 0,
		observation: "listener; no clock ops (milestone 0-3 savepoints carry no steps); " +
			"a=SupportBean(intPrimitive=0) until b=SupportBean(intPrimitive=1): A1(0)/A2(0) " +
			"collect silently, B1(1) fires ONE row {c0:A1,c1:A2,c2:B1}, then the trailing " +
			"A1(0)/B1(1) pair stays silent — the completed until is permanently false " +
			"(single-shot); EPL pins capital @Name('s0')",
		epls: []string{patternMatchUntilUntimed585SimpleEPL},
	},
	{
		name:         "select-array",
		ordinal:      2,
		runtimeIndex: 1,
		observation: "listener x2; no clock ops (milestone 0-2 carry no steps); " +
			"a=SupportBean_A until b=SupportBean_B over A1/A2 then B1 — leg 1 explicit " +
			"select a,b,a[0..2]+id fires {a:[A1,A2] exact order, a0/a1 beans, a2+a2Id null, " +
			"b:B1}; undeployModuleContaining (undeploy-all), then leg 2 select * fires " +
			"{a:[A1,A2],b:B1} fragments (Java pins assertSame bean identity + exact order)",
		epls: []string{patternMatchUntilUntimed585SelectArrayEPL, patternMatchUntilUntimed585SelectWildcardEPL},
	},
	{
		name:         "use-filter",
		ordinal:      3,
		runtimeIndex: 2,
		observation: "listener x5; no clock ops (milestone 0-7 carry no steps); five " +
			"select-* legs of a until b -> c correlating against a[i]: concat " +
			"(C('C'||a0||a1||b.id) silent then CA1A2B1 fires), equals (A3/20 silent, A2/10 " +
			"fires c.intPrimitive=10), in (a[2]=A3: A2/20 silent, A3/5 fires), not-in " +
			"(A2/20 + A1/20 silent, A6/5 fires), between over like-'A%'/'B%' SupportBeans " +
			"(E1/20 + E2/3 silent, E3/5 fires inclusively); undeploy-all between legs",
		epls: []string{
			patternMatchUntilUntimed585UseFilterConcatEPL,
			patternMatchUntilUntimed585UseFilterEqualsEPL,
			patternMatchUntilUntimed585UseFilterInEPL,
			patternMatchUntilUntimed585UseFilterNotInEPL,
			patternMatchUntilUntimed585UseFilterBetweenEPL,
		},
	},
	{
		name:         "array-function-repeat",
		ordinal:      5,
		runtimeIndex: 3,
		observation: "listener; no clock ops (milestone 0 carries no step); " +
			"[1:] a=SupportBean_A until UNTAGGED SupportBean_B: A1/A2/A3 collect, B(A2) " +
			"fires ONE row {length:3,l2:3} — SupportStaticMethodLib.arrayLength(a) and " +
			"java.lang.reflect.Array.getLength(a) both map to TagCount",
		epls: []string{patternMatchUntilUntimed585ArrayFunctionRepeatEPL},
	},
}

// patternMatchUntilUntimed585StepPin pins one scenario step's shape:
// deploys carry the byte-exact EPL and sends pin the full payload.
// There are NO advance-time steps and NO milestone steps — the Java
// executions perform no clock calls and milestone() savepoints restore
// identical state for these non-contextual statements.
type patternMatchUntilUntimed585StepPin struct {
	op        string
	statement string
	epl       string
	eventType string
	payload   map[string]any
}

func patternMatchUntilUntimed585DeployPin(statement, epl string) patternMatchUntilUntimed585StepPin {
	return patternMatchUntilUntimed585StepPin{op: "deploy", statement: statement, epl: epl}
}

func patternMatchUntilUntimed585SendPin(eventType string, payload map[string]any) patternMatchUntilUntimed585StepPin {
	return patternMatchUntilUntimed585StepPin{op: "send", eventType: eventType, payload: payload}
}

// patternMatchUntilUntimed585BeanSendPin pins one SupportBean send:
// theString plus intPrimitive.
func patternMatchUntilUntimed585BeanSendPin(theString string, intPrimitive int64) patternMatchUntilUntimed585StepPin {
	return patternMatchUntilUntimed585SendPin(patternMatchUntilUntimed585BeanType, map[string]any{
		"theString":    theString,
		"intPrimitive": float64(intPrimitive),
	})
}

// patternMatchUntilUntimed585LetterSendPin pins one letter-bean send:
// the id-only payload for SupportBean_A/B/C.
func patternMatchUntilUntimed585LetterSendPin(eventType, id string) patternMatchUntilUntimed585StepPin {
	return patternMatchUntilUntimed585SendPin(eventType, map[string]any{"id": id})
}

func patternMatchUntilUntimed585UndeployAllPin() patternMatchUntilUntimed585StepPin {
	return patternMatchUntilUntimed585StepPin{op: "undeploy-all"}
}

// patternMatchUntilUntimed585CaseSteps pins the complete step sequence
// per case in Java source order. undeployModuleContaining("s0") between
// the ord 2 legs is the undeploy-all op (no other deployment exists).
var patternMatchUntilUntimed585CaseSteps = func() map[string][]patternMatchUntilUntimed585StepPin {
	deploy := func(epl string) patternMatchUntilUntimed585StepPin {
		return patternMatchUntilUntimed585DeployPin("s0", epl)
	}
	sendA := func(id string) patternMatchUntilUntimed585StepPin {
		return patternMatchUntilUntimed585LetterSendPin(patternMatchUntilUntimed585AType, id)
	}
	sendB := func(id string) patternMatchUntilUntimed585StepPin {
		return patternMatchUntilUntimed585LetterSendPin(patternMatchUntilUntimed585BType, id)
	}
	sendC := func(id string) patternMatchUntilUntimed585StepPin {
		return patternMatchUntilUntimed585LetterSendPin(patternMatchUntilUntimed585CType, id)
	}
	return map[string][]patternMatchUntilUntimed585StepPin{
		"simple": {
			deploy(patternMatchUntilUntimed585SimpleEPL),
			patternMatchUntilUntimed585BeanSendPin("A1", 0),
			patternMatchUntilUntimed585BeanSendPin("A2", 0),
			patternMatchUntilUntimed585BeanSendPin("B1", 1),
			patternMatchUntilUntimed585BeanSendPin("A1", 0),
			patternMatchUntilUntimed585BeanSendPin("B1", 1),
			patternMatchUntilUntimed585UndeployAllPin(),
		},
		"select-array": {
			deploy(patternMatchUntilUntimed585SelectArrayEPL),
			sendA("A1"),
			sendA("A2"),
			sendB("B1"),
			patternMatchUntilUntimed585UndeployAllPin(),
			deploy(patternMatchUntilUntimed585SelectWildcardEPL),
			sendA("A1"),
			sendA("A2"),
			sendB("B1"),
			patternMatchUntilUntimed585UndeployAllPin(),
		},
		"use-filter": {
			deploy(patternMatchUntilUntimed585UseFilterConcatEPL),
			sendA("A1"),
			sendA("A2"),
			sendB("B1"),
			sendC("C1"),
			sendC("CA1A2B1"),
			patternMatchUntilUntimed585UndeployAllPin(),
			deploy(patternMatchUntilUntimed585UseFilterEqualsEPL),
			sendA("A1"),
			sendA("A2"),
			sendB("B1"),
			patternMatchUntilUntimed585BeanSendPin("A3", 20),
			patternMatchUntilUntimed585BeanSendPin("A2", 10),
			patternMatchUntilUntimed585UndeployAllPin(),
			deploy(patternMatchUntilUntimed585UseFilterInEPL),
			sendA("A1"),
			sendA("A2"),
			sendA("A3"),
			sendB("B1"),
			patternMatchUntilUntimed585BeanSendPin("A2", 20),
			patternMatchUntilUntimed585BeanSendPin("A3", 5),
			patternMatchUntilUntimed585UndeployAllPin(),
			deploy(patternMatchUntilUntimed585UseFilterNotInEPL),
			sendA("A1"),
			sendA("A2"),
			sendA("A3"),
			sendB("B1"),
			patternMatchUntilUntimed585BeanSendPin("A2", 20),
			patternMatchUntilUntimed585BeanSendPin("A1", 20),
			patternMatchUntilUntimed585BeanSendPin("A6", 5),
			patternMatchUntilUntimed585UndeployAllPin(),
			deploy(patternMatchUntilUntimed585UseFilterBetweenEPL),
			patternMatchUntilUntimed585BeanSendPin("A1", 5),
			patternMatchUntilUntimed585BeanSendPin("A2", 8),
			patternMatchUntilUntimed585BeanSendPin("B1", -1),
			patternMatchUntilUntimed585BeanSendPin("E1", 20),
			patternMatchUntilUntimed585BeanSendPin("E2", 3),
			patternMatchUntilUntimed585BeanSendPin("E3", 5),
			patternMatchUntilUntimed585UndeployAllPin(),
		},
		"array-function-repeat": {
			deploy(patternMatchUntilUntimed585ArrayFunctionRepeatEPL),
			sendA("A1"),
			sendA("A2"),
			sendA("A3"),
			sendB("A2"),
			patternMatchUntilUntimed585UndeployAllPin(),
		},
	}
}()

func loadPatternMatchUntilUntimed585Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", patternMatchUntilUntimed585ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", patternMatchUntilUntimed585ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilUntimed585ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", patternMatchUntilUntimed585ID, err)
	}
	if err := requirePatternMatchUntilUntimed585Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version        string   `json:"version"`
		ID             string   `json:"id"`
		Description    string   `json:"description"`
		JavaCommit     string   `json:"javaCommit"`
		JavaSource     string   `json:"javaSource"`
		JavaSourceFile []string `json:"javaSourceFiles"`
		JavaRuntimes   []string `json:"javaRuntimes"`
		JavaNames      []string `json:"javaNames"`
		JavaStaticIDs  []string `json:"javaStaticIds"`
		JavaFlags      []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", patternMatchUntilUntimed585ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != patternMatchUntilUntimed585ID ||
		metadata.Description != patternMatchUntilUntimed585Description ||
		metadata.JavaCommit != patternMatchUntilUntimed585JavaCommit ||
		metadata.JavaSource != patternMatchUntilUntimed585JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", patternMatchUntilUntimed585ID)
	}
	for _, pair := range [][2][]string{
		{metadata.JavaSourceFile, patternMatchUntilUntimed585JavaSources},
		{metadata.JavaRuntimes, patternMatchUntilUntimed585JavaRuntimeIDs},
		{metadata.JavaNames, patternMatchUntilUntimed585JavaExecutions},
		{metadata.JavaStaticIDs, patternMatchUntilUntimed585JavaStaticIDs},
		{metadata.JavaFlags, patternMatchUntilUntimed585JavaFlags},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			return compat.Scenario{}, fmt.Errorf("%s scenario metadata arrays are not pinned", patternMatchUntilUntimed585ID)
		}
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(patternMatchUntilUntimed585CaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases",
			patternMatchUntilUntimed585ID, len(patternMatchUntilUntimed585CaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requirePatternMatchUntilUntimed585Fields(object, "case", "ordinal", "runtimeId",
			"executionName", "observation", "epl"); err != nil {
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
		spec := patternMatchUntilUntimed585CaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != patternMatchUntilUntimed585JavaRuntimeIDs[spec.runtimeIndex] ||
			definition.ExecutionName != patternMatchUntilUntimed585JavaExecutions[spec.runtimeIndex] ||
			definition.Observation != spec.observation || definition.EPL != spec.epls[0] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", patternMatchUntilUntimed585ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || rawSteps == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", patternMatchUntilUntimed585ID)
	}
	if err := validatePatternMatchUntilUntimed585RawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validatePatternMatchUntilUntimed585RawSteps pins the complete step
// sequence: op field whitelists per step kind plus positional comparison
// against the pinned per-case sequences — per leg one s0 deploy with the
// verbatim EPL, the pinned sends and undeploy-all. There are no
// advance-time or milestone steps because the Java executions perform
// no clock calls.
func validatePatternMatchUntilUntimed585RawSteps(rawSteps []json.RawMessage) error {
	currentCase := ""
	caseOrder := make([]string, 0, len(patternMatchUntilUntimed585CaseSpecs))
	positions := make(map[string]int)
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return fmt.Errorf("scenario step %d op must be a string: %w", index, err)
		}
		var step struct {
			Case      string          `json:"case"`
			Statement string          `json:"statement"`
			EPL       string          `json:"epl"`
			EventType string          `json:"eventType"`
			Payload   json.RawMessage `json:"payload"`
		}
		if operation == "case" {
			if err := requirePatternMatchUntilUntimed585Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var marker struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &marker); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := patternMatchUntilUntimed585CaseSteps[marker.Case]; !ok {
				return fmt.Errorf("scenario step %d selects unknown case %q", index, marker.Case)
			}
			currentCase = marker.Case
			caseOrder = append(caseOrder, marker.Case)
			continue
		}
		if currentCase == "" {
			return fmt.Errorf("scenario step %d is outside any case block", index)
		}
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		if step.Case != currentCase {
			return fmt.Errorf("scenario step %d declares case %q inside the %q block",
				index, step.Case, currentCase)
		}
		pins := patternMatchUntilUntimed585CaseSteps[currentCase]
		position := positions[currentCase]
		if position >= len(pins) {
			return fmt.Errorf("scenario step %d exceeds the pinned %s step sequence",
				index, currentCase)
		}
		pin := pins[position]
		if operation != pin.op {
			return fmt.Errorf("scenario step %d op %q is not the pinned %q for case %q position %d",
				index, operation, pin.op, currentCase, position)
		}
		switch operation {
		case "deploy":
			if err := requirePatternMatchUntilUntimed585Fields(object, "op", "case", "statement", "epl"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Statement != pin.statement || step.EPL != pin.epl {
				return fmt.Errorf("scenario step %d deploy is not pinned for case %q", index, currentCase)
			}
		case "send":
			if err := requirePatternMatchUntilUntimed585Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return fmt.Errorf("scenario step %d payload: %w", index, err)
			}
			if step.EventType != pin.eventType || !reflect.DeepEqual(payload, pin.payload) {
				return fmt.Errorf("scenario step %d send is not pinned for case %q", index, currentCase)
			}
		case "undeploy-all":
			if err := requirePatternMatchUntilUntimed585Fields(object, "op", "case"); err != nil {
				return fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		positions[currentCase]++
	}
	if len(caseOrder) != len(patternMatchUntilUntimed585CaseSpecs) {
		return fmt.Errorf("scenario case order = %v, want %d cases",
			caseOrder, len(patternMatchUntilUntimed585CaseSpecs))
	}
	for index, spec := range patternMatchUntilUntimed585CaseSpecs {
		if caseOrder[index] != spec.name {
			return fmt.Errorf("scenario case order = %v, want pinned order", caseOrder)
		}
		if positions[spec.name] != len(patternMatchUntilUntimed585CaseSteps[spec.name]) {
			return fmt.Errorf("scenario case %q has %d steps, want %d",
				spec.name, positions[spec.name], len(patternMatchUntilUntimed585CaseSteps[spec.name]))
		}
	}
	return nil
}

func requirePatternMatchUntilUntimed585Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has %d fields, want %d", len(object), len(names))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

// patternMatchUntilUntimed585CaseState carries per-case replay state:
// the deployed statement, the leg counter and the delivery records.
type patternMatchUntilUntimed585CaseState struct {
	spec        patternMatchUntilUntimed585CaseSpec
	env         *esper.Environment
	engine      *esper.Engine
	deployment  *esper.Deployment
	deployIndex int
	records     []compat.TraceRecord
}

// runPatternMatchUntilUntimed585Scenario replays the executions against
// a fresh engine per case like the Java oracle's fresh per-execution
// runtime. The engine clock stays at epoch — the scenario carries no
// advance-time steps and milestone() savepoints are documented no-ops.
func runPatternMatchUntilUntimed585Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validatePatternMatchUntilUntimed585Scenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range patternMatchUntilUntimed585CaseSpecs {
		records, err := runPatternMatchUntilUntimed585Case(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", patternMatchUntilUntimed585ID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	return trace, nil
}

func validatePatternMatchUntilUntimed585Scenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != patternMatchUntilUntimed585ID {
		return fmt.Errorf("scenario id %q is not %q", scenario.ID, patternMatchUntilUntimed585ID)
	}
	rawSteps := make([]json.RawMessage, 0, len(scenario.Steps))
	for _, step := range scenario.Steps {
		raw, err := json.Marshal(step)
		if err != nil {
			return err
		}
		rawSteps = append(rawSteps, raw)
	}
	return validatePatternMatchUntilUntimed585RawSteps(rawSteps)
}

func runPatternMatchUntilUntimed585Case(ctx context.Context, scenario compat.Scenario, spec patternMatchUntilUntimed585CaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[patternMatchUntilUntimed585Bean](env, patternMatchUntilUntimed585BeanType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilUntimed585A](env, patternMatchUntilUntimed585AType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilUntimed585B](env, patternMatchUntilUntimed585BType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[patternMatchUntilUntimed585C](env, patternMatchUntilUntimed585CType); err != nil {
		return nil, err
	}
	state := &patternMatchUntilUntimed585CaseState{
		spec:   spec,
		env:    env,
		engine: esper.NewEngine(env, esper.WithRuntimeURI(patternMatchUntilUntimed585JavaRuntimeIDs[spec.runtimeIndex]), esper.WithStartTime(time.Unix(0, 0).UTC())),
	}
	defer func() { _ = state.engine.Close(context.Background()) }()

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
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "send":
			payload, err := patternMatchUntilUntimed585DecodePayload(step)
			if err != nil {
				return nil, err
			}
			if err := state.engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, err
			}
		case "undeploy-all":
			if state.deployment != nil {
				if err := state.deployment.Undeploy(ctx); err != nil {
					return nil, err
				}
				state.deployment = nil
			}
		default:
			return nil, fmt.Errorf("%s: unsupported step op %q", patternMatchUntilUntimed585ID, step.Op)
		}
	}
	return state.records, nil
}

// deploy maps the pinned EPL onto the fluent Go equivalent, mirroring
// internal/esper/pattern_matchuntil_parity_test.go's ord 0/2/3/5 tests:
//
//   - simple: a=SupportBean(intPrimitive=0) until b=(intPrimitive=1) ->
//     PatternFrom(bean,"a",Equal(intPrimitive,0)).Until(PatternFrom(bean,
//     "b",Equal(intPrimitive,1))) projecting a[0]/a[1]/b .theString via
//     TagFieldAt/TagField.
//   - select-array leg 1: a until b projecting the collected array via
//     TagEvents("a"), the element beans via ArrayAt(TagEvents, i) (index
//     2 -> Null) plus TagFieldAt ids; leg 2 select * -> {a:TagEvents,
//     b:PatternEvent} fragments.
//   - use-filter legs: (a until b).Then(c=...) correlating the follow-on
//     filter against TagFieldAt(a,i)/TagField(b) via Concat/Equal/InOf/
//     NotEqual x3/BetweenOf; select * -> {a:TagEvents,b:PatternEvent,
//     c:PatternEvent}.
//   - array-function-repeat: MatchUntil(1,0) ([1:] bound) .Until(...) +
//     TagCount("a") for both arrayLength(a) and Array.getLength(a).
//
// Each deploy gets a fresh listener sequence counter, mirroring the Java
// oracle's per-deployment TraceWriter (the counter restarts at each
// redeploy leg).
func (s *patternMatchUntilUntimed585CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.deployIndex >= len(s.spec.epls) || step.Statement != "s0" || step.Epl != s.spec.epls[s.deployIndex] {
		return fmt.Errorf("%s: case %q deploy %d is not pinned", patternMatchUntilUntimed585ID, s.spec.name, s.deployIndex)
	}
	query, err := s.query()
	if err != nil {
		return err
	}
	s.deployIndex++
	plan, err := s.env.Build(query)
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	statements := deployment.Statements()
	if len(statements) != 1 || statements[0].Name() != "s0" {
		return fmt.Errorf("%s: expected one s0 statement", patternMatchUntilUntimed585ID)
	}
	statement := statements[0]
	s.deployment = deployment
	var sequence uint64
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		sequence++
		s.records = append(s.records, compat.TraceRecord{
			Case:      s.spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// query builds the fluent plan for the case's current leg (deployIndex
// selects the leg's EPL). select * expands to the per-tag fragment
// columns — a as TagEvents (the collected bean array) and b/c as
// PatternEvent — matching the Java wildcard fragments; the explicit ord
// 2 leg projects TagEvents + ArrayAt(TagEvents, i) element beans +
// TagFieldAt ids (a[2]/a[2].id -> Null).
func (s *patternMatchUntilUntimed585CaseState) query() (esper.Query, error) {
	bean := esper.From[patternMatchUntilUntimed585Bean](s.env, patternMatchUntilUntimed585BeanType)
	streamA := esper.From[patternMatchUntilUntimed585A](s.env, patternMatchUntilUntimed585AType)
	streamB := esper.From[patternMatchUntilUntimed585B](s.env, patternMatchUntilUntimed585BType)
	streamC := esper.From[patternMatchUntilUntimed585C](s.env, patternMatchUntilUntimed585CType)
	trueExpr := esper.Literal(true)
	selectABC := func(stream esper.PatternStream) esper.Query {
		return stream.Select(
			esper.Alias("a", esper.TagEvents("a")),
			esper.Alias("b", esper.PatternEvent("b")),
			esper.Alias("c", esper.PatternEvent("c")),
		).Query(esper.StatementName("s0"))
	}
	switch s.spec.name {
	case "simple":
		pattern := esper.PatternFrom(bean, "a",
			esper.Equal[int](
				esper.Field[patternMatchUntilUntimed585Bean, int]("intPrimitive"),
				esper.Literal(0))).
			Until(esper.PatternFrom(bean, "b",
				esper.Equal[int](
					esper.Field[patternMatchUntilUntimed585Bean, int]("intPrimitive"),
					esper.Literal(1))))
		return pattern.Select(
			esper.Alias("c0", esper.TagFieldAt[string]("a", 0, "theString")),
			esper.Alias("c1", esper.TagFieldAt[string]("a", 1, "theString")),
			esper.Alias("c2", esper.TagField[string]("b", "theString")),
		).Query(esper.StatementName("s0")), nil
	case "select-array":
		pattern := esper.PatternFrom(streamA, "a", trueExpr).
			Until(esper.PatternFrom(streamB, "b", trueExpr))
		if s.deployIndex == 0 {
			return pattern.Select(
				esper.Alias("a", esper.TagEvents("a")),
				esper.Alias("b", esper.PatternEvent("b")),
				esper.Alias("a0", esper.ArrayAt[esper.Event](esper.TagEvents("a"), esper.Literal(0))),
				esper.Alias("a0Id", esper.TagFieldAt[string]("a", 0, "id")),
				esper.Alias("a1", esper.ArrayAt[esper.Event](esper.TagEvents("a"), esper.Literal(1))),
				esper.Alias("a1Id", esper.TagFieldAt[string]("a", 1, "id")),
				esper.Alias("a2", esper.ArrayAt[esper.Event](esper.TagEvents("a"), esper.Literal(2))),
				esper.Alias("a2Id", esper.TagFieldAt[string]("a", 2, "id")),
			).Query(esper.StatementName("s0")), nil
		}
		return pattern.Select(
			esper.Alias("a", esper.TagEvents("a")),
			esper.Alias("b", esper.PatternEvent("b")),
		).Query(esper.StatementName("s0")), nil
	case "use-filter":
		switch s.deployIndex {
		case 0:
			// c=SupportBean_C(id = ('C' || a[0].id || a[1].id || b.id))
			repeated := esper.PatternFrom(streamA, "a", trueExpr).
				Until(esper.PatternFrom(streamB, "b", trueExpr))
			correlated := esper.PatternFrom(streamC, "c", esper.Equal[string](
				esper.Field[patternMatchUntilUntimed585C, string]("id"),
				esper.Concat(
					esper.Literal("C"),
					esper.TagFieldAt[string]("a", 0, "id"),
					esper.TagFieldAt[string]("a", 1, "id"),
					esper.TagField[string]("b", "id"),
				)))
			return selectABC(repeated.Then(correlated)), nil
		case 1:
			// c=SupportBean(theString = a[1].id)
			repeated := esper.PatternFrom(streamA, "a", trueExpr).
				Until(esper.PatternFrom(streamB, "b", trueExpr))
			correlated := esper.PatternFrom(bean, "c", esper.Equal[string](
				esper.Field[patternMatchUntilUntimed585Bean, string]("theString"),
				esper.TagFieldAt[string]("a", 1, "id")))
			return selectABC(repeated.Then(correlated)), nil
		case 2:
			// c=SupportBean(theString in(a[2].id))
			repeated := esper.PatternFrom(streamA, "a", trueExpr).
				Until(esper.PatternFrom(streamB, "b", trueExpr))
			correlated := esper.PatternFrom(bean, "c", esper.InOf(
				esper.Field[patternMatchUntilUntimed585Bean, string]("theString"),
				esper.TagFieldAt[string]("a", 2, "id")))
			return selectABC(repeated.Then(correlated)), nil
		case 3:
			// c=SupportBean(theString!=a[0].id and theString!=a[1].id and
			// theString!=a[2].id)
			repeated := esper.PatternFrom(streamA, "a", trueExpr).
				Until(esper.PatternFrom(streamB, "b", trueExpr))
			theString := esper.Field[patternMatchUntilUntimed585Bean, string]("theString")
			correlated := esper.PatternFrom(bean, "c", esper.And(
				esper.And(
					esper.NotEqual[string](theString, esper.TagFieldAt[string]("a", 0, "id")),
					esper.NotEqual[string](theString, esper.TagFieldAt[string]("a", 1, "id")),
				),
				esper.NotEqual[string](theString, esper.TagFieldAt[string]("a", 2, "id")),
			))
			return selectABC(repeated.Then(correlated)), nil
		case 4:
			// a=SupportBean(theString like 'A%') until
			// b=SupportBean(theString like 'B%') ->
			// c=SupportBean(intPrimitive between a[0].intPrimitive and
			// a[1].intPrimitive)
			theString := esper.Field[patternMatchUntilUntimed585Bean, string]("theString")
			repeated := esper.PatternFrom(bean, "a",
				esper.LikeOf(theString, esper.Literal("A%"))).
				Until(esper.PatternFrom(bean, "b",
					esper.LikeOf(theString, esper.Literal("B%"))))
			correlated := esper.PatternFrom(bean, "c", esper.BetweenOf(
				esper.Field[patternMatchUntilUntimed585Bean, int]("intPrimitive"),
				esper.TagFieldAt[int]("a", 0, "intPrimitive"),
				esper.TagFieldAt[int]("a", 1, "intPrimitive")))
			return selectABC(repeated.Then(correlated)), nil
		}
	case "array-function-repeat":
		pattern := esper.PatternFrom(streamA, "a", trueExpr).MatchUntil(1, 0).
			Until(esper.PatternFrom(streamB, "b", trueExpr))
		return pattern.Select(
			esper.Alias("length", esper.TagCount("a")),
			esper.Alias("l2", esper.TagCount("a")),
		).Query(esper.StatementName("s0")), nil
	}
	return esper.Query{}, fmt.Errorf("%s: case %q has no leg %d", patternMatchUntilUntimed585ID, s.spec.name, s.deployIndex)
}

// patternMatchUntilUntimed585DecodePayload converts a scenario send
// payload into the typed host object for the event type.
func patternMatchUntilUntimed585DecodePayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case patternMatchUntilUntimed585BeanType:
		if err := requirePatternMatchUntilUntimed585Fields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var event patternMatchUntilUntimed585Bean
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return event, nil
	case patternMatchUntilUntimed585AType:
		if err := requirePatternMatchUntilUntimed585Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternMatchUntilUntimed585A
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return event, nil
	case patternMatchUntilUntimed585BType:
		if err := requirePatternMatchUntilUntimed585Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternMatchUntilUntimed585B
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	case patternMatchUntilUntimed585CType:
		if err := requirePatternMatchUntilUntimed585Fields(fields, "id"); err != nil {
			return nil, err
		}
		var event patternMatchUntilUntimed585C
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_C: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", patternMatchUntilUntimed585ID, step.EventType)
	}
}
