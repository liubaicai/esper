package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the eleventh differential slice of InfraNamedWindowViews:
// the named-window consumers (filtered, prior access, univariate statistics)
// and the late-started consumers including a left outer join.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 43 InfraFilteringConsumer                   java-runtime-a7827f22ee0c135e84d2
//   - ord 45 InfraFilteringConsumerLateStart          java-runtime-502dd5b0e84f28fb2c68
//   - ord 49 InfraPriorStats                          java-runtime-5ccc9535c9241efda4cc
//   - ord 50 InfraLateConsumer                        java-runtime-5118f72a4d8684d593d0
//   - ord 51 InfraLateConsumerJoin                    java-runtime-c49a6a43a1efd3fa0729
//
// A from-clause filter on a named window (`MyWindow(value > 0, value < 10)`)
// lives inside the consumer view and filters both streams, so a unique-window
// replacement whose incoming row fails the filter still delivers the replaced
// row as old data. `#uni(value)` is the univariate-statistics derived view (not
// a dedupe view): it adds datapoints/total/average to every event and iterates
// its single derived row, while a plain consumer over it sees one aggregate row
// per update. The late-started consumers preload the retained window as one
// filtered update, which fills the aggregation state and the per-statement
// previous-event history before the first iterator read. The late join replays
// the retained window rows null-padded through the left-outer join and
// recomputes the whole join result on every iterator read.
//
// Listeners the suite never asserts (ord 43/45 `delete`, ord 49 `create`) are
// excluded from the trace.
const infraNWCViewId = "infra-named-window-consumer-views"

const infraNWCViewDescription = "InfraNamedWindowViews consumer slice: the unique-key window with its filtered consumer and the keepall window with a late filtered aggregate, plus the prior-value, late univariate-statistics and late left-outer-join consumers whose preload and iterator shapes differ from their listener deltas (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)."

const infraNWCViewJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const infraNWCViewSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java"

// Byte-exact EPL pins (Java lines 2951-2954 for ord 43, 3125-3126/3135/3151 for
// ord 45, 3226-3229 for ord 49, 3269-3270/3277-3278/3287-3288/3299-3300 for
// ord 50 and 3319-3320/3327-3328/3338-3340 for ord 51). Each pin is the
// statement text without the trailing ";" and newline of the deployment that
// joins the statements.
const (
	infraNWCViewCreateFilteringConsumer = "@name('create') create window MyWindowFC#unique(key) as select theString as key, intPrimitive as value from SupportBean"
	infraNWCViewInsertFilteringConsumer = "insert into MyWindowFC select theString as key, intPrimitive as value from SupportBean"
	infraNWCViewSelectFilteringConsumer = "@name('s0') select irstream key, value as value from MyWindowFC(value > 0, value < 10)"
	infraNWCViewDeleteFilteringConsumer = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFC as s1 where s0.symbol = s1.key"

	infraNWCViewCreateFilteringLateStart = "@name('create') @public create window MyWindowFCLS#keepall as select theString as key, intPrimitive as value from SupportBean"
	infraNWCViewInsertFilteringLateStart = "insert into MyWindowFCLS select theString as key, intPrimitive as value from SupportBean"
	infraNWCViewSelectFilteringLateStart = "@name('s0') select irstream sum(value) as sumvalue from MyWindowFCLS(value > 0, value < 10)"
	infraNWCViewDeleteFilteringLateStart = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindowFCLS as s1 where s0.symbol = s1.key"

	infraNWCViewCreatePriorStats    = "@name('create') create window MyWindowPS#keepall as MySimpleKeyValueMap"
	infraNWCViewInsertPriorStats    = "insert into MyWindowPS select theString as key, longBoxed as value from SupportBean"
	infraNWCViewSelectPriorStats    = "@name('s0') select prior(1, key) as priorKeyOne, prior(2, key) as priorKeyTwo from MyWindowPS"
	infraNWCViewSelectTwoPriorStats = "@name('s3') select average from MyWindowPS#uni(value)"

	infraNWCViewCreateLateConsumer    = "@name('create') @public create window MyWindowLCL#keepall as MySimpleKeyValueMap"
	infraNWCViewInsertLateConsumer    = "insert into MyWindowLCL select theString as key, longBoxed as value from SupportBean"
	infraNWCViewSelectLateConsumer    = "@name('s0') select irstream average from MyWindowLCL#uni(value)"
	infraNWCViewSelectTwoLateConsumer = "@name('s2') select count(*) as cnt from MyWindowLCL"

	infraNWCViewCreateLateConsumerJoin    = "@name('create') @public create window MyWindowLCJ#keepall as MySimpleKeyValueMap"
	infraNWCViewInsertLateConsumerJoin    = "insert into MyWindowLCJ select theString as key, longBoxed as value from SupportBean"
	infraNWCViewSelectTwoLateConsumerJoin = "@name('s2') select key, value, symbol from MyWindowLCJ as s0 left outer join SupportMarketDataBean#keepall as s1 on s0.value = s1.volume"
)

// infraNWCViewShape selects the named window declaration of a case: the
// projection schema and the retention declared by the create statement.
type infraNWCViewShape int

const (
	// infraNWCViewUniqueIntRows is the ord-43 window:
	// #unique(key) over the (key String, value Integer) projection.
	infraNWCViewUniqueIntRows infraNWCViewShape = iota
	// infraNWCViewKeepAllIntRows is the ord-45 window: #keepall over the same
	// (key, value Integer) projection.
	infraNWCViewKeepAllIntRows
	// infraNWCViewKeepAllMapRows is the ords 49/50/51 window: #keepall over
	// the MySimpleKeyValueMap {key String, value long} schema.
	infraNWCViewKeepAllMapRows
)

// infraNWCViewConsumer selects how one consumer statement of a case is built.
type infraNWCViewConsumer int

const (
	// infraNWCViewConsumerFilter projects the row through the from-clause
	// filter with the remove stream (ord 43 s0).
	infraNWCViewConsumerFilter infraNWCViewConsumer = iota
	// infraNWCViewConsumerFilterSum is the filtered sum aggregate with the
	// remove stream (ord 45 s0).
	infraNWCViewConsumerFilterSum
	// infraNWCViewConsumerPrior projects prior(1)/prior(2) without a remove
	// stream (ord 49 s0).
	infraNWCViewConsumerPrior
	// infraNWCViewConsumerUniAverage is the #uni(value) average without a
	// remove stream (ord 49 s3).
	infraNWCViewConsumerUniAverage
	// infraNWCViewConsumerUniAverageIstream is the #uni(value) average with
	// the remove stream (ord 50 s0).
	infraNWCViewConsumerUniAverageIstream
	// infraNWCViewConsumerCount is count(*) without a remove stream (ord 50 s2).
	infraNWCViewConsumerCount
	// infraNWCViewConsumerJoin is the left outer join against the keep-all
	// market stream without a remove stream (ord 51 s2).
	infraNWCViewConsumerJoin
)

type infraNWCViewCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	windowName  string
	description string
	createEPL   string
	insertEPL   string
	s0EPL       string
	s2EPL       string
	s3EPL       string
	consumeEPL  string
	deleteEPL   string
	varEPL      string
	onSetEPL    string
	deploys     []string
	listened    map[string]bool
	sends       int
	advances    int
	// caseMode is the optional mode of the case step ("any" sorts that case's
	// listener rows); empty means the step carries no mode at all.
	caseMode string
	// undeployStatements is the pinned ordered list of module-scoped undeploy
	// targets (empty when the case tears down with undeploy-all).
	undeployStatements []string
	snapshots          int
	// snapModes, snapStatements and snapFields are the per-snapshot pins in
	// scenario order: the comparison mode, the statement iterated and the
	// exact field set the Java assertion reads (empty for a count-only
	// iterator assertion).
	snapModes      []string
	snapStatements []string
	snapFields     [][]string
	shape          infraNWCViewShape
	consumers      map[string]infraNWCViewConsumer
	rowFields      map[string][]string
	sendFields     []string
	marketFields   []string
}

var infraNWCViewCaseSpecs = []infraNWCViewCaseSpec{
	{
		name:           "filtering-consumer",
		ordinal:        43,
		runtimeID:      "java-runtime-a7827f22ee0c135e84d2",
		execution:      "InfraFilteringConsumer",
		windowName:     "MyWindowFC",
		description:    "unique(key) window over the theString/intPrimitive projection: a same-key arrival replaces the stored row in one callback carrying the new and the replaced row, and the filtered consumer evaluates its filter on both streams so the replaced row still passes while the filtered-out new row is dropped",
		createEPL:      infraNWCViewCreateFilteringConsumer,
		insertEPL:      infraNWCViewInsertFilteringConsumer,
		s0EPL:          infraNWCViewSelectFilteringConsumer,
		deleteEPL:      infraNWCViewDeleteFilteringConsumer,
		deploys:        []string{"create", "insert", "s0", "delete"},
		listened:       map[string]bool{"create": true, "s0": true},
		sends:          8,
		snapshots:      5,
		snapModes:      []string{"any", "ordered", "any", "ordered", "any"},
		snapStatements: []string{"create", "s0", "create", "s0", "s0"},
		snapFields: [][]string{
			{"key", "value"},
			{"key", "value"},
			{"key", "value"},
			{"key", "value"},
			{"key", "value"},
		},
		shape:        infraNWCViewUniqueIntRows,
		consumers:    map[string]infraNWCViewConsumer{"s0": infraNWCViewConsumerFilter},
		rowFields:    map[string][]string{"create": {"key", "value"}, "s0": {"key", "value"}},
		sendFields:   []string{"theString", "intPrimitive"},
		marketFields: []string{"symbol"},
	},
	{
		name:               "filtering-consumer-late-start",
		ordinal:            45,
		runtimeID:          "java-runtime-502dd5b0e84f28fb2c68",
		execution:          "InfraFilteringConsumerLateStart",
		windowName:         "MyWindowFCLS",
		description:        "keepall window over the theString/intPrimitive projection with a late filtered aggregate consumer: the preload pushes the two matching window rows as one update so the sum starts at 7, and a filtered-out arrival never enters the aggregation, not even for its later delete",
		createEPL:          infraNWCViewCreateFilteringLateStart,
		insertEPL:          infraNWCViewInsertFilteringLateStart,
		s0EPL:              infraNWCViewSelectFilteringLateStart,
		deleteEPL:          infraNWCViewDeleteFilteringLateStart,
		deploys:            []string{"create", "insert", "s0", "delete"},
		listened:           map[string]bool{"s0": true},
		sends:              8,
		undeployStatements: []string{"s0", "delete", "create"},
		snapshots:          6,
		snapModes:          []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatements:     []string{"s0", "s0", "s0", "s0", "s0", "s0"},
		snapFields: [][]string{
			{"sumvalue"},
			{"sumvalue"},
			{"sumvalue"},
			{"sumvalue"},
			{"sumvalue"},
			{"sumvalue"},
		},
		shape:        infraNWCViewKeepAllIntRows,
		consumers:    map[string]infraNWCViewConsumer{"s0": infraNWCViewConsumerFilterSum},
		rowFields:    map[string][]string{"s0": {"sumvalue"}},
		sendFields:   []string{"theString", "intPrimitive"},
		marketFields: []string{"symbol"},
	},
	{
		name:           "prior-stats",
		ordinal:        49,
		runtimeID:      "java-runtime-5ccc9535c9241efda4cc",
		execution:      "InfraPriorStats",
		windowName:     "MyWindowPS",
		description:    "keepall map window with a prior-values consumer and a univariate-statistics consumer: prior(1|2, key) reads the statement's own arrival history while the uni(value) view exposes the running average as a single-row iterator on every update",
		createEPL:      infraNWCViewCreatePriorStats,
		insertEPL:      infraNWCViewInsertPriorStats,
		s0EPL:          infraNWCViewSelectPriorStats,
		s3EPL:          infraNWCViewSelectTwoPriorStats,
		deploys:        []string{"create", "insert", "s0", "s3"},
		listened:       map[string]bool{"s0": true, "s3": true},
		sends:          4,
		snapshots:      4,
		snapModes:      []string{"ordered", "ordered", "ordered", "ordered"},
		snapStatements: []string{"s3", "s3", "s3", "s3"},
		snapFields: [][]string{
			{"average"},
			{"average"},
			{"average"},
			{"average"},
		},
		shape: infraNWCViewKeepAllMapRows,
		consumers: map[string]infraNWCViewConsumer{
			"s0": infraNWCViewConsumerPrior,
			"s3": infraNWCViewConsumerUniAverage,
		},
		rowFields:  map[string][]string{"s0": {"priorKeyOne", "priorKeyTwo"}, "s3": {"average"}},
		sendFields: []string{"theString", "longBoxed"},
	},
	{
		name:           "late-consumer",
		ordinal:        50,
		runtimeID:      "java-runtime-5118f72a4d8684d593d0",
		execution:      "InfraLateConsumer",
		windowName:     "MyWindowLCL",
		description:    "keepall map window whose univariate-statistics consumer and count(*) consumer are both deployed late: the two independent preloads start the average at 1.5 and the count at 4, and the statistics consumer is irstream so the previous average is delivered as old data",
		createEPL:      infraNWCViewCreateLateConsumer,
		insertEPL:      infraNWCViewInsertLateConsumer,
		s0EPL:          infraNWCViewSelectLateConsumer,
		s2EPL:          infraNWCViewSelectTwoLateConsumer,
		deploys:        []string{"create", "insert", "s0", "s2"},
		listened:       map[string]bool{"create": true, "s0": true},
		sends:          5,
		snapshots:      7,
		snapModes:      []string{"ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered"},
		snapStatements: []string{"s0", "s0", "s0", "s2", "s0", "s0", "s2"},
		snapFields: [][]string{
			{"average"},
			{"average"},
			{"average"},
			{"cnt"},
			{"average"},
			{"average"},
			{"cnt"},
		},
		shape: infraNWCViewKeepAllMapRows,
		consumers: map[string]infraNWCViewConsumer{
			"s0": infraNWCViewConsumerUniAverageIstream,
			"s2": infraNWCViewConsumerCount,
		},
		rowFields:  map[string][]string{"create": {"key", "value"}, "s0": {"average"}},
		sendFields: []string{"theString", "longBoxed"},
	},
	{
		name:           "late-consumer-join",
		ordinal:        51,
		runtimeID:      "java-runtime-c49a6a43a1efd3fa0729",
		execution:      "InfraLateConsumerJoin",
		windowName:     "MyWindowLCJ",
		description:    "keepall map window whose left-outer join consumer against the market-data window is deployed late: the replayed window arrives in the join before the first iterator read, the market send matches both rows in either order, and the join iterator recomputes the full result so an unmatched row vanishes without a remove-stream record",
		createEPL:      infraNWCViewCreateLateConsumerJoin,
		insertEPL:      infraNWCViewInsertLateConsumerJoin,
		s2EPL:          infraNWCViewSelectTwoLateConsumerJoin,
		deploys:        []string{"create", "insert", "s2"},
		listened:       map[string]bool{"create": true, "s2": true},
		sends:          5,
		caseMode:       "any",
		snapshots:      4,
		snapModes:      []string{"ordered", "any", "any", "any"},
		snapStatements: []string{"s2", "s2", "s2", "s2"},
		snapFields: [][]string{
			{"key", "value", "symbol"},
			{"key", "value", "symbol"},
			{"key", "value", "symbol"},
			{"key", "value", "symbol"},
		},
		shape:        infraNWCViewKeepAllMapRows,
		consumers:    map[string]infraNWCViewConsumer{"s2": infraNWCViewConsumerJoin},
		rowFields:    map[string][]string{"create": {"key", "value"}, "s2": {"key", "value", "symbol"}},
		sendFields:   []string{"theString", "longBoxed"},
		marketFields: []string{"symbol", "volume"},
	},
}

var (
	infraNWCViewJavaSources = []string{
		infraNWCViewSource,
	}
	infraNWCViewJavaRuntimeIDs = []string{
		"java-runtime-a7827f22ee0c135e84d2",
		"java-runtime-502dd5b0e84f28fb2c68",
		"java-runtime-5ccc9535c9241efda4cc",
		"java-runtime-5118f72a4d8684d593d0",
		"java-runtime-c49a6a43a1efd3fa0729",
	}
	infraNWCViewJavaExecutions = []string{
		"InfraFilteringConsumer",
		"InfraFilteringConsumerLateStart",
		"InfraPriorStats",
		"InfraLateConsumer",
		"InfraLateConsumerJoin",
	}
)

// infraNWCViewBean mirrors SupportBean; each case validates the exact send
// fields it pins, so the unset fields stay at their Go zero value.
type infraNWCViewBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
	LongBoxed    int64  `esper:"longBoxed"`
}

type infraNWCViewMarket struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
}

// infraNWCViewKVInt mirrors the (key String, value Integer) window projection
// of ords 43/45.
type infraNWCViewKVInt struct {
	Key   string `esper:"key"`
	Value int    `esper:"value"`
}

// infraNWCViewKVLong mirrors the MySimpleKeyValueMap window schema of
// ords 49/50/51.
type infraNWCViewKVLong struct {
	Key   string `esper:"key"`
	Value int64  `esper:"value"`
}

func infraNWCViewCaseSpecFor(name string) (infraNWCViewCaseSpec, bool) {
	for _, spec := range infraNWCViewCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return infraNWCViewCaseSpec{}, false
}

// infraNWCViewCaseNames returns the pinned case order used by the loader's
// ordering check.
func infraNWCViewCaseNames() []string {
	names := make([]string, 0, len(infraNWCViewCaseSpecs))
	for _, spec := range infraNWCViewCaseSpecs {
		names = append(names, spec.name)
	}
	return names
}

func loadInfraNWCViewScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWCViewId)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWCViewId, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWCViewId, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWCViewId, err)
	}
	if err := requireInfraNWCViewFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWCViewId, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWCViewId ||
		metadata.Description != infraNWCViewDescription ||
		metadata.JavaCommit != infraNWCViewJavaCommit || metadata.JavaSource != infraNWCViewSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", infraNWCViewId)
	}
	if len(metadata.JavaFlags) != 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario must carry no Java flags", infraNWCViewId)
	}
	if err := infraNWCViewRequireEqual(metadata.JavaRuntimes, infraNWCViewJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := infraNWCViewRequireEqual(metadata.JavaNames, infraNWCViewJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", infraNWCViewId, err)
	}
	if len(rawCases) != len(infraNWCViewCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", infraNWCViewId, len(rawCases), len(infraNWCViewCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWCViewFields(object,
			"case", "ordinal", "runtimeId", "executionName", "description", "createEpl",
			"insertEpl", "s0Epl", "s2Epl", "s3Epl", "consumeEpl", "deleteEpl", "varEpl", "onSetEpl",
			"deploys", "listened", "iteratorSnapshots"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string   `json:"case"`
			Ordinal           int      `json:"ordinal"`
			RuntimeID         string   `json:"runtimeId"`
			ExecutionName     string   `json:"executionName"`
			Description       string   `json:"description"`
			CreateEPL         string   `json:"createEpl"`
			InsertEPL         string   `json:"insertEpl"`
			S0EPL             string   `json:"s0Epl"`
			S2EPL             string   `json:"s2Epl"`
			S3EPL             string   `json:"s3Epl"`
			ConsumeEPL        string   `json:"consumeEpl"`
			DeleteEPL         string   `json:"deleteEpl"`
			VarEPL            string   `json:"varEpl"`
			OnSetEPL          string   `json:"onSetEpl"`
			Deploys           []string `json:"deploys"`
			Listened          []string `json:"listened"`
			IteratorSnapshots int      `json:"iteratorSnapshots"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		spec := infraNWCViewCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Description != spec.description {
			return compat.Scenario{}, fmt.Errorf("scenario case %q description does not match the pinned slice description", spec.name)
		}
		if drifted := infraNWCViewEPLDrift(spec, definition.CreateEPL, definition.InsertEPL, definition.S0EPL,
			definition.S2EPL, definition.S3EPL, definition.ConsumeEPL, definition.DeleteEPL,
			definition.VarEPL, definition.OnSetEPL); len(drifted) != 0 {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source (slots %s)",
				spec.name, strings.Join(drifted, ", "))
		}
		if err := infraNWCViewRequireEqual(definition.Deploys, spec.deploys, spec.name+" deploys"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWCViewRequireListened(definition.Listened, spec); err != nil {
			return compat.Scenario{}, err
		}
		if definition.IteratorSnapshots != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q iteratorSnapshots = %d, want %d",
				spec.name, definition.IteratorSnapshots, spec.snapshots)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", infraNWCViewId, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraNWCViewId)
	}
	deployCounts := map[string]int{}
	sendCounts := map[string]int{}
	advanceCounts := map[string]int{}
	undeployStatements := map[string][]string{}
	caseOrder := make([]string, 0, len(infraNWCViewCaseSpecs))
	currentCase := ""
	snapshotCounts := map[string]int{}
	snapshotModes := map[string][]string{}
	snapshotStatements := map[string][]string{}
	snapshotFields := map[string][][]string{}
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
			var step struct {
				Case string `json:"case"`
				Mode string `json:"mode"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWCViewCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d selects unknown case %q", index, step.Case)
			}
			allowed := []string{"op", "case"}
			if spec.caseMode != "" {
				allowed = append(allowed, "mode")
			}
			if err := requireInfraNWCViewFields(object, allowed...); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Mode != spec.caseMode {
				return compat.Scenario{}, fmt.Errorf("scenario step %d case %q mode %q must be %q",
					index, step.Case, step.Mode, spec.caseMode)
			}
			currentCase = step.Case
			caseOrder = append(caseOrder, step.Case)
		case "deploy":
			if err := requireInfraNWCViewFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWCViewCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unknown case %q", index, step.Case)
			}
			if err := infraNWCViewRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			want, ok := infraNWCViewEPLForStep(spec, step.Statement)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploys unexpected statement %q for case %q", index, step.Statement, step.Case)
			}
			if step.EPL != want {
				return compat.Scenario{}, fmt.Errorf("scenario step %d statement %q EPL does not match the Java source", index, step.Statement)
			}
			deployCounts[step.Case]++
		case "send":
			if err := requireInfraNWCViewFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWCViewCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d sends for unknown case %q", index, step.Case)
			}
			if err := infraNWCViewRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			sendCounts[step.Case]++
		case "snapshot":
			if err := requireInfraNWCViewFields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string   `json:"case"`
				Statement string   `json:"statement"`
				Mode      string   `json:"mode"`
				Fields    []string `json:"fields"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWCViewCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshots unknown case %q", index, step.Case)
			}
			if err := infraNWCViewRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			if snapshotCounts[step.Case] >= len(spec.snapStatements) {
				return compat.Scenario{}, fmt.Errorf("scenario case %q has more snapshot steps than the pinned sites", spec.name)
			}
			if step.Statement != spec.snapStatements[snapshotCounts[step.Case]] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot statement %q must be %q for case %q",
					index, step.Statement, spec.snapStatements[snapshotCounts[step.Case]], step.Case)
			}
			if step.Mode != spec.snapModes[snapshotCounts[step.Case]] {
				return compat.Scenario{}, fmt.Errorf("scenario step %d snapshot mode %q must be %q for case %q",
					index, step.Mode, spec.snapModes[snapshotCounts[step.Case]], step.Case)
			}
			if err := infraNWCViewRequireEqual(step.Fields, spec.snapFields[snapshotCounts[step.Case]], step.Case+" snapshot fields"); err != nil {
				return compat.Scenario{}, err
			}
			snapshotModes[step.Case] = append(snapshotModes[step.Case], step.Mode)
			snapshotStatements[step.Case] = append(snapshotStatements[step.Case], step.Statement)
			snapshotFields[step.Case] = append(snapshotFields[step.Case], step.Fields)
			snapshotCounts[step.Case]++
		case "undeploy":
			if err := requireInfraNWCViewFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			spec, ok := infraNWCViewCaseSpecFor(step.Case)
			if !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys unknown case %q", index, step.Case)
			}
			if err := infraNWCViewRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			if !infraNWCViewContains(spec.deploys, step.Statement) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys statement %q which case %q never deploys",
					index, step.Statement, step.Case)
			}
			undeployStatements[step.Case] = append(undeployStatements[step.Case], step.Statement)
		case "advance-time":
			if err := requireInfraNWCViewFields(object, "op", "case", "at"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
				At   string `json:"at"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := time.Parse(time.RFC3339Nano, step.At); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advance-time at %q: %w", index, step.At, err)
			}
			// Java parses Instants (ISO_INSTANT), which rejects numeric offsets;
			// require the UTC form so both sides accept the same spellings.
			if !strings.HasSuffix(step.At, "Z") {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advance-time at %q must be in UTC (Z) form", index, step.At)
			}
			if _, ok := infraNWCViewCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d advances unknown case %q", index, step.Case)
			}
			if err := infraNWCViewRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
			advanceCounts[step.Case]++
		case "undeploy-all":
			if err := requireInfraNWCViewFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case string `json:"case"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, ok := infraNWCViewCaseSpecFor(step.Case); !ok {
				return compat.Scenario{}, fmt.Errorf("scenario step %d undeploys all for unknown case %q", index, step.Case)
			}
			if err := infraNWCViewRequireInBlock(index, step.Case, currentCase); err != nil {
				return compat.Scenario{}, err
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
	}
	for index, spec := range infraNWCViewCaseSpecs {
		if index >= len(caseOrder) || caseOrder[index] != spec.name {
			return compat.Scenario{}, fmt.Errorf("scenario case order = %v, want %v", caseOrder, infraNWCViewCaseNames())
		}
		if sendCounts[spec.name] != spec.sends {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d send steps, want %d",
				spec.name, sendCounts[spec.name], spec.sends)
		}
		if advanceCounts[spec.name] != spec.advances {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d advance-time steps, want %d",
				spec.name, advanceCounts[spec.name], spec.advances)
		}
		if deployCounts[spec.name] != len(spec.deploys) {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d deploy steps, want %d",
				spec.name, deployCounts[spec.name], len(spec.deploys))
		}
		if snapshotCounts[spec.name] != spec.snapshots {
			return compat.Scenario{}, fmt.Errorf("scenario case %q has %d snapshot steps, want %d",
				spec.name, snapshotCounts[spec.name], spec.snapshots)
		}
		if err := infraNWCViewRequireEqual(undeployStatements[spec.name], spec.undeployStatements, spec.name+" undeploy statements"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWCViewRequireEqual(snapshotModes[spec.name], spec.snapModes, spec.name+" snapshot modes"); err != nil {
			return compat.Scenario{}, err
		}
		if err := infraNWCViewRequireEqual(snapshotStatements[spec.name], spec.snapStatements, spec.name+" snapshot statements"); err != nil {
			return compat.Scenario{}, err
		}
		for index := range spec.snapFields {
			if err := infraNWCViewRequireEqual(snapshotFields[spec.name][index], spec.snapFields[index],
				fmt.Sprintf("%s snapshot %d fields", spec.name, index)); err != nil {
				return compat.Scenario{}, err
			}
		}
	}
	var steps []compat.Step
	if err := json.Unmarshal(root["steps"], &steps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps: %w", infraNWCViewId, err)
	}
	return compat.Scenario{
		Version: metadata.Version,
		ID:      metadata.ID,
		Steps:   steps,
	}, nil
}

// infraNWCViewEPLForStep returns the byte-exact EPL of one deploy step; a case
// that does not deploy the statement has an empty slot and rejects it.
func infraNWCViewEPLForStep(spec infraNWCViewCaseSpec, statement string) (string, bool) {
	switch statement {
	case "create":
		return spec.createEPL, spec.createEPL != ""
	case "insert":
		return spec.insertEPL, spec.insertEPL != ""
	case "s0":
		return spec.s0EPL, spec.s0EPL != ""
	case "s2":
		return spec.s2EPL, spec.s2EPL != ""
	case "s3":
		return spec.s3EPL, spec.s3EPL != ""
	case "delete":
		return spec.deleteEPL, spec.deleteEPL != ""
	default:
		return "", false
	}
}

// infraNWCViewEPLDrift names the case-definition EPL slots that differ from the
// pinned Java statement text.
func infraNWCViewEPLDrift(spec infraNWCViewCaseSpec, create, insert, s0, s2, s3, consume, deleteEPL, variable, onSet string) []string {
	var drifted []string
	slots := []struct {
		name string
		got  string
		want string
	}{
		{"createEpl", create, spec.createEPL},
		{"insertEpl", insert, spec.insertEPL},
		{"s0Epl", s0, spec.s0EPL},
		{"s2Epl", s2, spec.s2EPL},
		{"s3Epl", s3, spec.s3EPL},
		{"consumeEpl", consume, spec.consumeEPL},
		{"deleteEpl", deleteEPL, spec.deleteEPL},
		{"varEpl", variable, spec.varEPL},
		{"onSetEpl", onSet, spec.onSetEPL},
	}
	for _, slot := range slots {
		if slot.got != slot.want {
			drifted = append(drifted, slot.name)
		}
	}
	return drifted
}

func infraNWCViewRequireEqual(got, want []string, label string) error {
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

func infraNWCViewRequireListened(got []string, spec infraNWCViewCaseSpec) error {
	if len(got) != len(spec.listened) {
		return fmt.Errorf("scenario case %q listened = %v, want %d statements", spec.name, got, len(spec.listened))
	}
	seen := make(map[string]bool, len(got))
	for _, name := range got {
		if !spec.listened[name] {
			return fmt.Errorf("scenario case %q listens to %q which is not in the pinned listener set", spec.name, name)
		}
		if seen[name] {
			return fmt.Errorf("scenario case %q listens to %q twice", spec.name, name)
		}
		seen[name] = true
	}
	return nil
}

// infraNWCViewRequireInBlock rejects a case-scoped step that appears outside
// its own case block.
func infraNWCViewRequireInBlock(index int, stepCase, currentCase string) error {
	if stepCase != currentCase {
		return fmt.Errorf("scenario step %d targets case %q outside its block (current case %q)",
			index, stepCase, currentCase)
	}
	return nil
}

func infraNWCViewContains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func requireInfraNWCViewFields(object map[string]json.RawMessage, names ...string) error {
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

func runInfraNWCViewScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWCViewId)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range infraNWCViewCaseSpecs {
		caseTrace, err := runInfraNWCViewCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWCViewId, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

func runInfraNWCViewCase(ctx context.Context, scenario compat.Scenario, spec infraNWCViewCaseSpec) ([]compat.TraceRecord, error) {
	// Virtual clock: starts at the epoch and moves only on advance-time steps,
	// so every record carries the clock value at delivery.
	now := time.Unix(0, 0).UTC()
	sequence := map[string]uint64{}
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

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWCViewBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if len(spec.marketFields) != 0 {
		if _, err := esper.RegisterStruct[infraNWCViewMarket](env, "SupportMarketDataBean"); err != nil {
			return nil, err
		}
	}
	var windowSchema esper.Schema
	var err error
	switch spec.shape {
	case infraNWCViewUniqueIntRows, infraNWCViewKeepAllIntRows:
		windowSchema, err = esper.RegisterStruct[infraNWCViewKVInt](env, spec.windowName)
	case infraNWCViewKeepAllMapRows:
		windowSchema, err = esper.RegisterStruct[infraNWCViewKVLong](env, spec.windowName)
	default:
		return nil, fmt.Errorf("unexpected window shape %d for case %q", spec.shape, spec.name)
	}
	if err != nil {
		return nil, err
	}
	retention, err := infraNWCViewRetentionSpec(spec)
	if err != nil {
		return nil, err
	}
	if _, err := esper.CreateNamedWindow(env, spec.windowName, windowSchema,
		esper.NamedWindowRetention(retention)); err != nil {
		return nil, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(now),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
	snapshotCount := 0
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
			plan, err := infraNWCViewBuildPlan(env, spec, step.Statement)
			if err != nil {
				return nil, err
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return nil, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == step.Statement {
					statements[step.Statement] = statement
					deployments[step.Statement] = deployment
				}
			}
			if spec.listened[step.Statement] {
				statement, ok := statements[step.Statement]
				if !ok {
					return nil, fmt.Errorf("deploy %q did not register the statement", step.Statement)
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
			payload, err := infraNWCViewDecodePayload(spec, step)
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
			if snapshotCount >= len(spec.snapFields) {
				return nil, fmt.Errorf("case %q has more snapshot steps than the pinned field sets", spec.name)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			rows := projectRecords(compat.NormalizeResults(result.Batch.New), spec.snapFields[snapshotCount])
			snapshotCount++
			// A count-only iterator assertion pins no fields; both sides emit
			// empty rows and mark the step "any" so the comparison is a count
			// comparison.
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
		case "undeploy":
			deployment, ok := deployments[step.Statement]
			if !ok {
				return nil, fmt.Errorf("no deployment for statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return nil, err
			}
			delete(deployments, step.Statement)
			delete(statements, step.Statement)
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return nil, fmt.Errorf("advance-time %q: %w", step.At, err)
			}
			// The clock moves first: expiry deliveries produced by the advance
			// carry the boundary time, matching the Java trace.
			now = at.UTC()
			if err := engine.AdvanceTime(ctx, now); err != nil {
				return nil, err
			}
		case "undeploy-all":
			// The Java suite tears the deployment down at this point; teardown
			// emits no trace records and the next case uses a fresh
			// environment, so nothing further is replayed here.
		default:
			return nil, fmt.Errorf("unsupported step op %q", step.Op)
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	return records, nil
}

// infraNWCViewRetentionSpec maps a case onto its pinned named window retention.
func infraNWCViewRetentionSpec(spec infraNWCViewCaseSpec) (esper.WindowSpec, error) {
	switch spec.shape {
	case infraNWCViewUniqueIntRows:
		return esper.Unique(esper.Field[infraNWCViewKVInt, string]("key")), nil
	case infraNWCViewKeepAllIntRows, infraNWCViewKeepAllMapRows:
		return esper.KeepAll(), nil
	default:
		return nil, fmt.Errorf("case %q: unexpected window shape %d", spec.name, spec.shape)
	}
}

// infraNWCViewBuildPlan maps one scenario deploy statement onto the typed chain.
func infraNWCViewBuildPlan(env *esper.Environment, spec infraNWCViewCaseSpec, statement string) (esper.Plan, error) {
	switch statement {
	case "create":
		return env.Build(esper.FromNamedWindow(env, spec.windowName).
			CreateNamedWindowQuery(esper.StatementName("create"), esper.WithOldStream()))
	case "insert":
		source := esper.From[infraNWCViewBean](env, "SupportBean")
		switch spec.shape {
		case infraNWCViewUniqueIntRows, infraNWCViewKeepAllIntRows:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWCViewBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWCViewBean, int]("intPrimitive")),
			).Query(esper.StatementName("insert")))
		case infraNWCViewKeepAllMapRows:
			return env.Build(esper.OnEvent(source).InsertIntoNamedWindow(spec.windowName,
				esper.SetColumn("key", esper.Field[infraNWCViewBean, string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWCViewBean, int64]("longBoxed")),
			).Query(esper.StatementName("insert")))
		default:
			return esper.Plan{}, fmt.Errorf("case %q: unexpected insert for window shape %d", spec.name, spec.shape)
		}
	case "s0", "s2", "s3":
		return infraNWCViewBuildConsumer(env, spec, statement)
	case "delete":
		return env.Build(esper.OnEvent(esper.From[infraNWCViewMarket](env, "SupportMarketDataBean")).
			DeleteFromNamedWindow(spec.windowName,
				esper.Equal[string](esper.NamedWindowField[string]("key"),
					esper.Field[infraNWCViewMarket, string]("symbol"))).
			Query(esper.StatementName("delete")))
	default:
		return esper.Plan{}, fmt.Errorf("unexpected deploy statement %q for case %q", statement, spec.name)
	}
}

// infraNWCViewBuildConsumer maps one consumer statement onto the typed chain.
func infraNWCViewBuildConsumer(env *esper.Environment, spec infraNWCViewCaseSpec, statement string) (esper.Plan, error) {
	kind, ok := spec.consumers[statement]
	if !ok {
		return esper.Plan{}, fmt.Errorf("case %q has no pinned consumer for statement %q", spec.name, statement)
	}
	window := esper.FromNamedWindow(env, spec.windowName)
	// The from-clause filter of ords 43/45 lives inside the consumer view and
	// filters both the new and the old array.
	filter := esper.And(
		esper.Greater[int](esper.Field[any, int]("value"), esper.Literal(0)),
		esper.Less[int](esper.Field[any, int]("value"), esper.Literal(10)),
	)
	switch kind {
	case infraNWCViewConsumerFilter:
		return env.Build(window.Filter(filter).
			Query(esper.StatementName(statement), esper.WithOldStream()))
	case infraNWCViewConsumerFilterSum:
		return env.Build(window.Filter(filter).
			Aggregate(esper.Alias("sumvalue", esper.Sum[int](esper.Field[any, int]("value")))).
			Query(esper.StatementName(statement), esper.WithOldStream()))
	case infraNWCViewConsumerPrior:
		// Esper prior(N, x) maps to Go Prior(N-1, x).
		return env.Build(window.Select(
			esper.Alias("priorKeyOne", esper.Prior[string](0, esper.Field[any, string]("key"))),
			esper.Alias("priorKeyTwo", esper.Prior[string](1, esper.Field[any, string]("key"))),
		).Query(esper.StatementName(statement)))
	case infraNWCViewConsumerUniAverage, infraNWCViewConsumerUniAverageIstream:
		// #uni(value) is the univariate-statistics derived view: the consumer
		// aggregates its running average.
		query := window.Aggregate(
			esper.Alias("average", esper.UnivariateStatistics[int64](esper.Field[any, int64]("value")).Average()),
		)
		if kind == infraNWCViewConsumerUniAverageIstream {
			return env.Build(query.Query(esper.StatementName(statement), esper.WithOldStream()))
		}
		return env.Build(query.Query(esper.StatementName(statement)))
	case infraNWCViewConsumerCount:
		return env.Build(window.Aggregate(
			esper.Alias("cnt", esper.CountAll()),
		).Query(esper.StatementName(statement)))
	case infraNWCViewConsumerJoin:
		return env.Build(esper.JoinMany(
			esper.JoinRecordSource(esper.FromNamedWindow(env, spec.windowName)),
			esper.JoinRecordSource(esper.From[infraNWCViewMarket](env, "SupportMarketDataBean").
				Window(esper.KeepAll()).AsRecord()),
		).On(
			esper.OnSourcesEqual(0, esper.Field[any, int64]("value"), 1, esper.Field[any, int64]("volume")),
		).LeftOuter().Select(
			esper.SelectFrom(0, "key", esper.Field[any, string]("key")),
			esper.SelectFrom(0, "value", esper.Field[any, int64]("value")),
			esper.SelectFrom(1, "symbol", esper.Field[any, string]("symbol")),
		).Query(esper.StatementName(statement)))
	default:
		return esper.Plan{}, fmt.Errorf("case %q statement %q: unexpected consumer %d", spec.name, statement, kind)
	}
}

func infraNWCViewDecodePayload(spec infraNWCViewCaseSpec, step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWCViewFields(fields, spec.sendFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportBean payload: %w", spec.name, err)
		}
		var bean infraNWCViewBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWCViewFields(fields, spec.marketFields...); err != nil {
			return nil, fmt.Errorf("case %q SupportMarketDataBean payload: %w", spec.name, err)
		}
		var market infraNWCViewMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}
