package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// resultset_aggregate_invalid_closure.go replays the four invalid-form
// ResultSetAggregate executions pinned by Draft 4.496, closing the suite's
// compile-time rejection surface across four Java files (10 probes):
//
//   - ext-invalid (ResultSetAggregateExtInvalid, single-execution file):
//     one path-less tryInvalidCompile probe pins `select rate(10) from
//     SupportBean` rejected as an unknown function because the suite runs
//     with extendedAggregation=false. The typed Go API has no
//     extended-aggregation toggle and Rate compiles, so the probe pins as
//     an unrepresentable record.
//   - aggregate-invalid (ResultSetAggregateSortedMinMaxBy ord 7,
//     ResultSetAggregateInvalid): two path-less tryInvalidCompile probes
//     pin the sorted/minmax-by declaration rejections — maxBy over a
//     criteria expression spanning two streams, and sorted over a stream
//     with no data window.
//   - window-invalid (ResultSetAggregationMethodWindow ord 4,
//     ResultSetAggregateWindowInvalid): deploys the @public table
//     MyTable(windowcol window(*) @type('SupportBean')) onto the path,
//     then runs two path-carrying tryInvalidCompile probes —
//     windowcol.first(id) whose argument validates against the column's
//     contained type (SupportBean has no id) and
//     windowcol.listReference(intPrimitive) whose arity check rejects any
//     parameter. Both probes pin as unrepresentable records: the typed Go
//     API has no first(property)/listReference(property) forms and the
//     Method escape hatch bypasses the boundary, so no Go rejection is
//     claimed. The case ends with undeploy-all.
//   - filter-named-param-invalid (ResultSetAggregateFilterNamedParameter
//     ord 20, ResultSetAggregateFilterNamedParamInvalid): five path-less
//     tryInvalidCompile probes — a multi-value filter tuple, multiple
//     filter expressions and a non-boolean filter (all unrepresentable in
//     the typed API: FilterAggregate takes exactly one Expression[bool]),
//     a create-table column declaring filter:true (Go rejects
//     TableAggDecl.Filter in CreateTable), and a correlated subquery whose
//     aggregate filter reads an outer-stream property (rejected by the
//     existing subselect-correlation check).
//
// Approved differences (observably identical to the Java EPL):
//   - The rate(10) rejection depends on the suite's
//     setExtendedAggregation(false) session configuration; Go has no such
//     toggle and Rate compiles, so the step records the pinned Java
//     message verbatim without claiming a Go rejection boundary (the
//     context_lifecycle.go unrepresentable precedent).
//   - The multi-value filter tuple, the positional-plus-named filter
//     pair, and the non-boolean filter have no typed-API form
//     (FilterAggregate accepts exactly one Expression[bool] predicate),
//     so those steps pin the Java prefixes verbatim.
//   - windowcol.first(id) and windowcol.listReference(intPrimitive) have
//     no typed-API form either — WindowAccessExpression.First takes no
//     argument and ListReference is zero-arg, while the generic Method
//     escape hatch bypasses the validation boundary — so both steps pin
//     the Java prefixes verbatim.
//   - Java error text embeds the normalized expression (lowercased
//     function names, no spaces, truncated at 35 chars + '...(N chars)')
//     and the compiler appends ' [<original EPL>]'; Go diagnostics differ,
//     so build-error steps assert a contains-guard on the Go rejection
//     before recording the pinned Java prefix.
//   - env.compileDeploy(epl, path) for the MyTable module maps to the
//     env-level CreateTable registration; Go has no module path, so the
//     deploy step replays the registration and undeploy-all is a
//     documented no-op (the per-case environment is discarded with the
//     engine).

const (
	resultsetAggregateInvalidClosureID         = "resultset-aggregate-invalid-closure"
	resultsetAggregateInvalidClosureJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateInvalidClosureSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateExtInvalid.java"
	resultsetAggregateInvalidClosureSource2    = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java"
	resultsetAggregateInvalidClosureSource3    = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodWindow.java"
	resultsetAggregateInvalidClosureSource4    = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"
)

const resultsetAggregateInvalidClosureDescription = "ResultSetAggregate invalid-closure surface (4 executions across 4 files, 10 compile-time probes): ext-invalid replays ResultSetAggregateExtInvalid — `select rate(10) from SupportBean` under extendedAggregation=false fails name resolution (unrepresentable: Go has no extended-aggregation toggle and Rate compiles); aggregate-invalid replays ResultSetAggregateSortedMinMaxBy ord 7 ResultSetAggregateInvalid — `maxBy(p00||p10)` spanning SupportBean_S0/SupportBean_S1 fails the same-stream criteria check and `sorted(p00)` over a windowless stream fails the data-window requirement; window-invalid replays ResultSetAggregationMethodWindow ord 4 ResultSetAggregateWindowInvalid — the @public MyTable(windowcol window(*) @type('SupportBean')) deploy precedes two path-carrying probes, `windowcol.first(id)` (argument validated against the contained type, SupportBean has no id) and `windowcol.listReference(intPrimitive)` (zero-parameter arity check), both unrepresentable in the typed API; filter-named-param-invalid replays ResultSetAggregateFilterNamedParameter ord 20 ResultSetAggregateFilterNamedParamInvalid — multi-value, multiple and non-boolean filter named parameters are unrepresentable (FilterAggregate takes one Expression[bool]), `create table MyTable(totals sum(int, filter:true))` is rejected by CreateTable, and the correlated `filter:s0.p00='a'` subquery aggregate is rejected by the existing subselect check. compile-error records carry the pinned Java message prefixes; unrepresentable records pin the probe EPL and prefix; the deployed record marks the MyTable module deploy (Java sources regression-lib/.../ResultSetAggregateExtInvalid.java, ResultSetAggregateSortedMinMaxBy.java, ResultSetAggregationMethodWindow.java, ResultSetAggregateFilterNamedParameter.java)."

var (
	resultsetAggregateInvalidClosureJavaRuntimeIDs = []string{
		"java-runtime-98f0ac5299780e2d6656",
		"java-runtime-09530eec55a704b01c96",
		"java-runtime-fbbdc48ea2da6d4bc426",
		"java-runtime-50935b7efcc1fdced314",
	}
	resultsetAggregateInvalidClosureJavaExecutions = []string{
		"ResultSetAggregateExtInvalid",
		"ResultSetAggregateInvalid",
		"ResultSetAggregateWindowInvalid",
		"ResultSetAggregateFilterNamedParamInvalid",
	}
	resultsetAggregateInvalidClosureJavaStaticIDs = []string{
		"java-9c4ed42f2a9b55c3fd1c",
		"java-553516b9d01c12a13172",
		"java-313287657d14b5c686f8",
		"java-0c29efb6d43971aba5c4",
	}
	resultsetAggregateInvalidClosureJavaFlags = []string{}
	resultsetAggregateInvalidClosureCases     = []string{
		"ext-invalid",
		"aggregate-invalid",
		"window-invalid",
		"filter-named-param-invalid",
	}
	resultsetAggregateInvalidClosureOrdinals = []int{0, 7, 4, 20}
	resultsetAggregateInvalidClosureSources  = []string{
		resultsetAggregateInvalidClosureSource,
		resultsetAggregateInvalidClosureSource2,
		resultsetAggregateInvalidClosureSource3,
		resultsetAggregateInvalidClosureSource4,
	}
)
var resultsetAggregateInvalidClosureCaseRuntimeIDs = map[string]string{
	"ext-invalid":                "java-runtime-98f0ac5299780e2d6656",
	"aggregate-invalid":          "java-runtime-09530eec55a704b01c96",
	"window-invalid":             "java-runtime-fbbdc48ea2da6d4bc426",
	"filter-named-param-invalid": "java-runtime-50935b7efcc1fdced314",
}

// The byte-exact EPLs the deploy and probes pin. The deploy EPL is the
// verbatim compileDeploy input including the trailing newline; the probe
// EPLs are the verbatim tryInvalidCompile inputs.
const resultsetAggregateInvalidClosureTableEPL = "@public create table MyTable(windowcol window(*) @type('SupportBean'));\n"

var resultsetAggregateInvalidClosureProbeEPLs = map[string]string{
	"rate-unknown-function":         "select rate(10) from SupportBean",
	"maxby-cross-stream":            "select maxBy(p00||p10) from SupportBean_S0#lastevent, SupportBean_S1#lastevent",
	"sorted-no-window":              "select sorted(p00) from SupportBean_S0",
	"windowcol-first-id":            "select MyTable.windowcol.first(id) from SupportBean_S0",
	"windowcol-listreference-arity": "select MyTable.windowcol.listReference(intPrimitive) from SupportBean_S0",
	"filter-multi-value":            "select sum(intPrimitive, filter:(intPrimitive, doublePrimitive)) from SupportBean",
	"filter-multiple":               "select sum(intPrimitive, intPrimitive > 0, filter:intPrimitive < 0) from SupportBean",
	"filter-non-bool":               "select sum(intPrimitive, filter:intPrimitive) from SupportBean",
	"create-table-filter":           "create table MyTable(totals sum(int, filter:true))",
	"filter-correlated-subquery":    "select (select sum(intPrimitive, filter:s0.p00='a') from SupportBean) from SupportBean_S0 as s0",
}

// The pinned Java message prefixes (SupportMessageAssertUtil.assertMessage
// startsWith semantics). The rate-unknown-function entry is the complete
// message: the suite pins the ' [<original EPL>]' suffix literally.
var resultsetAggregateInvalidClosureProbeErrors = map[string]string{
	"rate-unknown-function":         "Failed to validate select-clause expression 'rate(10)': Unknown single-row function, aggregation function or mapped or indexed property named 'rate' could not be resolved [select rate(10) from SupportBean]",
	"maxby-cross-stream":            "Failed to validate select-clause expression 'maxby(p00||p10)': The 'maxby' aggregation function requires that any parameter expressions evaluate properties of the same stream",
	"sorted-no-window":              "Failed to validate select-clause expression 'sorted(p00)': The 'sorted' aggregation function requires that a data window is declared for the stream",
	"windowcol-first-id":            "Failed to validate select-clause expression 'MyTable.windowcol.first(id)': Failed to validate aggregation function parameter expression 'id': Property named 'id' is not valid in any stream",
	"windowcol-listreference-arity": "Failed to validate select-clause expression 'MyTable.windowcol.listReference(int...(45 chars)': Invalid number of parameters",
	"filter-multi-value":            "Failed to validate select-clause expression 'sum(intPrimitive,filter:(intPrimiti...(55 chars)': Filter named parameter requires a single expression returning a boolean-typed value",
	"filter-multiple":               "Failed to validate select-clause expression 'sum(intPrimitive,intPrimitive>0,fil...(54 chars)': Only a single filter expression can be provided",
	"filter-non-bool":               "Failed to validate select-clause expression 'sum(intPrimitive,filter:intPrimitive)': Filter named parameter requires a single expression returning a boolean-typed value",
	"create-table-filter":           "Failed to validate table-column expression 'sum(int,filter:true)': The 'group_by' and 'filter' parameter is not allowed in create-table statements",
	"filter-correlated-subquery":    "Failed to plan subquery number 1 querying SupportBean: Subselect aggregation functions cannot aggregate across correlated properties",
}

var resultsetAggregateInvalidClosureCaseObservations = []string{
	"unrepresentable; the single path-less tryInvalidCompile probe pins the rate(10) unknown-function rejection the suite produces under extendedAggregation=false (Go has no toggle and Rate compiles, so the record carries the pinned message verbatim)",
	"compile-error; two path-less tryInvalidCompile probes pin the sorted/minmax-by rejections: maxBy over a criteria expression spanning SupportBean_S0 and SupportBean_S1 (same-stream check), and sorted over a stream with no declared data window",
	"deployed+unrepresentable; the @public MyTable(windowcol window(*) @type('SupportBean')) deploy precedes two path-carrying probes — windowcol.first(id) validates the argument against the contained type (SupportBean has no id) and windowcol.listReference(intPrimitive) fails the zero-parameter arity check; both pin verbatim (no typed-API form)",
	"compile-error+unrepresentable; five path-less tryInvalidCompile probes pin the filter named-parameter rejections: multi-value filter tuple, multiple filter expressions and non-boolean filter (pinned-only; FilterAggregate takes one Expression[bool]), create-table filter:true rejected by CreateTable, and the correlated filter:s0.p00='a' subquery aggregate rejected by the subselect-correlation check",
}

var resultsetAggregateInvalidClosureCaseEPLs = []string{
	resultsetAggregateInvalidClosureProbeEPLs["rate-unknown-function"],
	resultsetAggregateInvalidClosureProbeEPLs["maxby-cross-stream"],
	resultsetAggregateInvalidClosureTableEPL,
	resultsetAggregateInvalidClosureProbeEPLs["filter-multi-value"],
}

// resultsetAggregateInvalidClosureCaseSteps pins the complete step sequence
// per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
// keys. The deploy step carries the byte-exact table EPL the Java oracle
// compiles; build-error and unrepresentable steps carry the byte-exact
// probe EPL and the pinned expectError prefix. The window-invalid probes
// compile against the accumulated path (no compileWithoutPath marker),
// mirroring env.tryInvalidCompile(path, epl, ...); every other probe is
// path-less (compileWithoutPath=1), mirroring env.tryInvalidCompile(epl,
// ...).
var resultsetAggregateInvalidClosureCaseSteps = map[string][]string{
	"ext-invalid": {
		"unrepresentable|rate-unknown-function|||" + resultsetAggregateInvalidClosureProbeEPLs["rate-unknown-function"] + "||" + resultsetAggregateInvalidClosureProbeErrors["rate-unknown-function"] + "|1|",
	},
	"aggregate-invalid": {
		"build-error|maxby-cross-stream|||" + resultsetAggregateInvalidClosureProbeEPLs["maxby-cross-stream"] + "||" + resultsetAggregateInvalidClosureProbeErrors["maxby-cross-stream"] + "|1|",
		"build-error|sorted-no-window|||" + resultsetAggregateInvalidClosureProbeEPLs["sorted-no-window"] + "||" + resultsetAggregateInvalidClosureProbeErrors["sorted-no-window"] + "|1|",
	},
	"window-invalid": {
		"deploy|create-table|||" + resultsetAggregateInvalidClosureTableEPL + "||||",
		"deployed|create-table|||||||",
		"unrepresentable|windowcol-first-id|||" + resultsetAggregateInvalidClosureProbeEPLs["windowcol-first-id"] + "||" + resultsetAggregateInvalidClosureProbeErrors["windowcol-first-id"] + "||",
		"unrepresentable|windowcol-listreference-arity|||" + resultsetAggregateInvalidClosureProbeEPLs["windowcol-listreference-arity"] + "||" + resultsetAggregateInvalidClosureProbeErrors["windowcol-listreference-arity"] + "||",
		"undeploy-all||||||||",
	},
	"filter-named-param-invalid": {
		"unrepresentable|filter-multi-value|||" + resultsetAggregateInvalidClosureProbeEPLs["filter-multi-value"] + "||" + resultsetAggregateInvalidClosureProbeErrors["filter-multi-value"] + "|1|",
		"unrepresentable|filter-multiple|||" + resultsetAggregateInvalidClosureProbeEPLs["filter-multiple"] + "||" + resultsetAggregateInvalidClosureProbeErrors["filter-multiple"] + "|1|",
		"unrepresentable|filter-non-bool|||" + resultsetAggregateInvalidClosureProbeEPLs["filter-non-bool"] + "||" + resultsetAggregateInvalidClosureProbeErrors["filter-non-bool"] + "|1|",
		"unrepresentable|create-table-filter|||" + resultsetAggregateInvalidClosureProbeEPLs["create-table-filter"] + "||" + resultsetAggregateInvalidClosureProbeErrors["create-table-filter"] + "|1|",
		"build-error|filter-correlated-subquery|||" + resultsetAggregateInvalidClosureProbeEPLs["filter-correlated-subquery"] + "||" + resultsetAggregateInvalidClosureProbeErrors["filter-correlated-subquery"] + "|1|",
	},
}

// aggInvalidClosureBean mirrors SupportBean's asserted fields.
type aggInvalidClosureBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

// aggInvalidClosureS0 mirrors SupportBean_S0's asserted fields.
type aggInvalidClosureS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// aggInvalidClosureS1 mirrors SupportBean_S1's asserted fields.
type aggInvalidClosureS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// aggInvalidClosureCaseState carries the per-case replay state: the
// environment/engine pair, the deployed-step label bookkeeping, and the
// per-statement sequence counters.
type aggInvalidClosureCaseState struct {
	caseName       string
	env            *esper.Environment
	engine         *esper.Engine
	trace          *compat.Trace
	deployedLabels map[string]bool
	sequences      map[string]uint64
}

func runResultSetAggregateInvalidClosureScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetAggregateInvalidClosureScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeResultSetAggregateInvalidClosure(ctx, scenario, &trace)
}

// executeResultSetAggregateInvalidClosure replays the scenario: each case
// runs on a fresh environment/engine pair (one runtime per Java execution)
// and every step dispatches to the matching runtime action.
func executeResultSetAggregateInvalidClosure(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *aggInvalidClosureCaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		if step.Op != "case" && state == nil {
			return *trace, fmt.Errorf("%s: step %q arrives before any case marker", resultsetAggregateInvalidClosureID, step.Op)
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startAggInvalidClosureCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "deployed":
			if err := state.deployed(step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", resultsetAggregateInvalidClosureID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startAggInvalidClosureCase builds the fresh per-case environment: the
// SupportBean event type every case declares plus the SupportBean_S0 and
// SupportBean_S1 types the aggregate-invalid, window-invalid and
// filter-named-param-invalid suites register (ext-invalid's suite
// configuration registers SupportBean only), and the engine pinned to the
// case's Java runtime id at the epoch start time.
func startAggInvalidClosureCase(caseName string, trace *compat.Trace) (*aggInvalidClosureCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[aggInvalidClosureBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if caseName != "ext-invalid" {
		if _, err := esper.RegisterStruct[aggInvalidClosureS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[aggInvalidClosureS1](env, "SupportBean_S1"); err != nil {
			return nil, err
		}
	}
	state := &aggInvalidClosureCaseState{
		caseName:       caseName,
		env:            env,
		deployedLabels: map[string]bool{},
		sequences:      map[string]uint64{},
		trace:          trace,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetAggregateInvalidClosureCaseRuntimeIDs[caseName]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

// deploy executes one deploy step. The window-invalid create-table deploy
// replays env.compileDeploy(epl, path) of the @public MyTable module as the
// env-level CreateTable registration declaring the window(*) @type
// ('SupportBean') column; Go has no module path, so the registration is the
// nearest boundary.
func (s *aggInvalidClosureCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	if s.caseName != "window-invalid" || label != "create-table" || step.Epl != resultsetAggregateInvalidClosureTableEPL {
		return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", resultsetAggregateInvalidClosureID, s.caseName, label, step.Epl)
	}
	windowDecl := esper.TableAggDecl{Name: "window", Description: "window(*)",
		EventType: "SupportBean", NthSize: -1, RateInterval: -1}
	if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
		esper.OptionalTableColumnOf[any]("windowcol", esper.WithTableAggDecl(windowDecl)),
	}); err != nil {
		return fmt.Errorf("%s: deploy %q: %w", resultsetAggregateInvalidClosureID, label, err)
	}
	s.deployedLabels[label] = true
	return nil
}

// deployed emits the deployed marker for a statement label the preceding
// deploy step registered, mirroring the oracle's per-statement marker.
func (s *aggInvalidClosureCaseState) deployed(step compat.Step) error {
	if !s.deployedLabels[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", resultsetAggregateInvalidClosureID, step.Statement)
	}
	s.sequences[step.Statement+":deployed"]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequences[step.Statement+":deployed"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

// buildError runs one expected-invalid probe against the fluent equivalent
// of the pinned EPL. Each probe verifies Go rejects the nearest expressible
// boundary before recording the pinned Java message prefix.
func (s *aggInvalidClosureCaseState) buildError(step compat.Step) error {
	if pinned, ok := resultsetAggregateInvalidClosureProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", resultsetAggregateInvalidClosureID, step.Statement, step.Epl)
	}
	if step.ExpectError != resultsetAggregateInvalidClosureProbeErrors[step.Statement] {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned prefix %q", resultsetAggregateInvalidClosureID, step.Statement, step.ExpectError)
	}
	s0 := esper.From[aggInvalidClosureS0](s.env, "SupportBean_S0")
	var buildErr error
	switch step.Statement {
	case "maxby-cross-stream":
		// `maxBy(p00||p10)` over S0#lastevent, S1#lastevent — the criteria
		// expression spans two streams and fails the same-stream check.
		if s.caseName != "aggregate-invalid" {
			return fmt.Errorf("%s: build-error probe %q is not pinned for case %q", resultsetAggregateInvalidClosureID, step.Statement, s.caseName)
		}
		s1 := esper.From[aggInvalidClosureS1](s.env, "SupportBean_S1")
		_, buildErr = s.env.Build(esper.JoinMany(
			esper.JoinSource(s0.Window(esper.LastEvent())),
			esper.JoinSource(s1.Window(esper.LastEvent()))).
			Aggregate(esper.Alias("c0", esper.MaxBy[esper.Event, string](
				esper.JoinEventValue[esper.Event](0),
				esper.Concat(esper.JoinField[string](0, "p00"), esper.JoinField[string](1, "p10"))))).
			Query())
	case "sorted-no-window":
		// `sorted(p00)` over a windowless S0 — the sorted aggregation
		// requires a declared data window.
		if s.caseName != "aggregate-invalid" {
			return fmt.Errorf("%s: build-error probe %q is not pinned for case %q", resultsetAggregateInvalidClosureID, step.Statement, s.caseName)
		}
		_, buildErr = s.env.Build(s0.Aggregate(
			esper.Alias("c0", esper.SortedValues[string](esper.Field[aggInvalidClosureS0, string]("p00"), false))).
			Query())
	case "filter-correlated-subquery":
		// `select (select sum(intPrimitive, filter:s0.p00='a') from
		// SupportBean) from SupportBean_S0 as s0` — the subselect
		// aggregate's filter correlates to the outer stream.
		if s.caseName != "filter-named-param-invalid" {
			return fmt.Errorf("%s: build-error probe %q is not pinned for case %q", resultsetAggregateInvalidClosureID, step.Statement, s.caseName)
		}
		inner := esper.From[aggInvalidClosureBean](s.env, "SupportBean").AsRecord()
		_, buildErr = s.env.Build(esper.Select(s0,
			esper.Alias("c0", esper.SubqueryValue[int](inner,
				esper.FilterAggregate[int](esper.Sum[int](esper.Field[any, int]("intPrimitive")),
					esper.Equal[string](esper.OuterField[string]("p00"), esper.Literal("a")))))).
			Query())
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", resultsetAggregateInvalidClosureID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", resultsetAggregateInvalidClosureID, step.Statement)
	}
	// Verify the Go rejection carries the expected wording before recording
	// the pinned Java prefix.
	expected := map[string]string{
		"maxby-cross-stream":         "requires that any parameter expressions evaluate properties of the same stream",
		"sorted-no-window":           "requires that a data window is declared for the stream",
		"filter-correlated-subquery": "subselect aggregation functions cannot aggregate across correlated properties",
	}
	if want, ok := expected[step.Statement]; ok && !strings.Contains(buildErr.Error(), want) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", resultsetAggregateInvalidClosureID, step.Statement, buildErr)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// unrepresentable emits the pinned record for the seven surfaces the typed
// Go API cannot express: the rate(10) unknown-function rejection (no
// extended-aggregation toggle; Rate compiles), the multi-value, multiple
// and non-boolean filter named parameters (FilterAggregate takes exactly
// one Expression[bool] predicate), the create-table filter named parameter
// (TableAggDecl.Filter is a declared filtered aggregation, not the EPL
// filter: parameter), and the windowcol.first(id)/
// windowcol.listReference(intPrimitive) probes (no first(property) or
// listReference(property) form; the Method escape hatch bypasses the
// boundary). Each step records the pinned value without claiming a Go
// rejection boundary, mirroring the context_lifecycle.go precedent.
func (s *aggInvalidClosureCaseState) unrepresentable(step compat.Step) error {
	pinnedCase := map[string]string{
		"rate-unknown-function":         "ext-invalid",
		"windowcol-first-id":            "window-invalid",
		"windowcol-listreference-arity": "window-invalid",
		"filter-multi-value":            "filter-named-param-invalid",
		"filter-multiple":               "filter-named-param-invalid",
		"filter-non-bool":               "filter-named-param-invalid",
		"create-table-filter":           "filter-named-param-invalid",
	}
	wantCase, ok := pinnedCase[step.Statement]
	if !ok {
		return fmt.Errorf("%s: unknown unrepresentable step %q", resultsetAggregateInvalidClosureID, step.Statement)
	}
	if s.caseName != wantCase {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned for case %q", resultsetAggregateInvalidClosureID, step.Statement, s.caseName)
	}
	if step.Epl != resultsetAggregateInvalidClosureProbeEPLs[step.Statement] {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned EPL %q", resultsetAggregateInvalidClosureID, step.Statement, step.Epl)
	}
	if step.ExpectError != resultsetAggregateInvalidClosureProbeErrors[step.Statement] {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned value %q", resultsetAggregateInvalidClosureID, step.Statement, step.ExpectError)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployAll mirrors env.undeployAll(). The window-invalid table is an
// env-level registration, not a deployment, so the step is a documented
// no-op; the per-case environment is discarded with the engine.
func (s *aggInvalidClosureCaseState) undeployAll(ctx context.Context) error {
	return nil
}

// loadResultSetAggregateInvalidClosureScenario decodes the scenario with
// the strict-shape checks the raw-mutation tests pin: no duplicate JSON
// keys, the exact top-level field set, pinned metadata and case metadata,
// and the pinned per-case step keys so unknown or mutated steps fail the
// replay.
func loadResultSetAggregateInvalidClosureScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateInvalidClosureID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateInvalidClosureID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateInvalidClosureID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateInvalidClosureID, err)
	}
	if err := requireAggInvalidClosureFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSource2",
		"javaSource3", "javaSource4", "javaRuntimes", "javaNames", "javaStaticIds",
		"javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version     string `json:"version"`
		ID          string `json:"id"`
		Description string `json:"description"`
		JavaCommit  string `json:"javaCommit"`
		JavaSource  string `json:"javaSource"`
		JavaSource2 string `json:"javaSource2"`
		JavaSource3 string `json:"javaSource3"`
		JavaSource4 string `json:"javaSource4"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetAggregateInvalidClosureID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetAggregateInvalidClosureID ||
		metadata.Description != resultsetAggregateInvalidClosureDescription ||
		metadata.JavaCommit != resultsetAggregateInvalidClosureJavaCommit ||
		metadata.JavaSource != resultsetAggregateInvalidClosureSource ||
		metadata.JavaSource2 != resultsetAggregateInvalidClosureSource2 ||
		metadata.JavaSource3 != resultsetAggregateInvalidClosureSource3 ||
		metadata.JavaSource4 != resultsetAggregateInvalidClosureSource4 {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateInvalidClosureID)
	}
	if err := validateAggInvalidClosureStringArray(root["javaRuntimes"], resultsetAggregateInvalidClosureJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateAggInvalidClosureStringArray(root["javaNames"], resultsetAggregateInvalidClosureJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateAggInvalidClosureStringArray(root["javaStaticIds"], resultsetAggregateInvalidClosureJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateAggInvalidClosureStringArray(root["javaFlags"], resultsetAggregateInvalidClosureJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetAggregateInvalidClosureCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultsetAggregateInvalidClosureID, len(resultsetAggregateInvalidClosureCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireAggInvalidClosureFields(object,
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
		if definition.Case != resultsetAggregateInvalidClosureCases[index] ||
			definition.Ordinal != resultsetAggregateInvalidClosureOrdinals[index] ||
			definition.RuntimeID != resultsetAggregateInvalidClosureJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetAggregateInvalidClosureJavaExecutions[index] ||
			definition.Observation != resultsetAggregateInvalidClosureCaseObservations[index] ||
			definition.EPL != resultsetAggregateInvalidClosureCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateInvalidClosureID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", resultsetAggregateInvalidClosureID, err)
	}
	offset := 0
	for _, caseName := range resultsetAggregateInvalidClosureCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", resultsetAggregateInvalidClosureID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", resultsetAggregateInvalidClosureID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", resultsetAggregateInvalidClosureID, offset, caseName)
		}
		offset++
		want, ok := resultsetAggregateInvalidClosureCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", resultsetAggregateInvalidClosureID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", resultsetAggregateInvalidClosureID, caseName)
		}
		for _, pinned := range want {
			key, err := aggInvalidClosureStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", resultsetAggregateInvalidClosureID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", resultsetAggregateInvalidClosureID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", resultsetAggregateInvalidClosureID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateInvalidClosureID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", resultsetAggregateInvalidClosureID)
	}
	return scenario, nil
}

// aggInvalidClosureStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
// with the payload compacted. Unknown fields on the step object are
// rejected.
func aggInvalidClosureStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op                 string          `json:"op"`
		Case               string          `json:"case"`
		Statement          string          `json:"statement"`
		Name               string          `json:"name"`
		EventType          string          `json:"eventType"`
		Epl                string          `json:"epl"`
		Payload            json.RawMessage `json:"payload"`
		ExpectError        string          `json:"expectError"`
		CompileWithoutPath bool            `json:"compileWithoutPath"`
		At                 string          `json:"at"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "at": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payload := ""
	if len(step.Payload) != 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("payload: %w", err)
		}
		payload = compacted.String()
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.At, nil
}

// validateResultSetAggregateInvalidClosureScenario re-checks a decoded
// scenario (used when the runner receives a scenario decoded by the generic
// loader path). The generic compat.Step op whitelist accepts the
// unrepresentable op this scenario pins, so validation is the id and
// step-count invariants.
func validateResultSetAggregateInvalidClosureScenario(scenario compat.Scenario) error {
	if scenario.ID != resultsetAggregateInvalidClosureID {
		return fmt.Errorf("%s scenario id %q is not pinned", resultsetAggregateInvalidClosureID, scenario.ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", resultsetAggregateInvalidClosureID)
	}
	return nil
}

func requireAggInvalidClosureFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", resultsetAggregateInvalidClosureID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", resultsetAggregateInvalidClosureID, name)
		}
	}
	return nil
}

func validateAggInvalidClosureStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
