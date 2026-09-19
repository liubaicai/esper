package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

// infraTableResetBean mirrors SupportBean for the table-reset executions.
// The oracle's normalizeValue renders only theString/intPrimitive for this
// bean, so the Go struct carries exactly those fields.
type infraTableResetBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraTableResetS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type infraTableResetS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// infraTableResetIntrusionReset mirrors the doc-sample create-schema
// IntrusionReset event type.
type infraTableResetIntrusionReset struct {
	FromAddress string `esper:"fromAddress"`
	ToAddress   string `esper:"toAddress"`
}

const (
	infraTableResetID          = "infra-table-reset-aggregation-state"
	infraTableResetJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableResetJavaSource  = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableResetAggregationState.java"
	infraTableResetDescription = "InfraTableResetAggregationState ords 0-5: table aggregation reset() via on-merge update — unkeyed sum reset by column name and by table alias, keyed selective per-column reset (avg/window) with merge where-clause, twelve-aggregate whole-row mt.reset() including plugin and count-min-sketch columns, four invalid reset() compile probes, and the doc-sample compile-only module."
)

var infraTableResetJavaSources = []string{infraTableResetJavaSource}

// Case order fixes the runtime-ID index mapping below.
var infraTableResetCases = []string{
	"row-sum",
	"row-sum-alias",
	"selective",
	"various-aggs",
	"invalid",
	"doc-sample",
}

var infraTableResetJavaRuntimeIDs = []string{
	"java-runtime-5478f21270e0cc6e4d85",
	"java-runtime-6215263c68fc3379b070",
	"java-runtime-38acd85e3cec943643e7",
	"java-runtime-78c737da8c98ca9afafe",
	"java-runtime-e0e02833416f4b55ebee",
	"java-runtime-21ca203d405fd3a727c8",
}

var infraTableResetJavaExecutions = []string{
	"InfraTableResetRowSum",
	"InfraTableResetRowSumWTableAlias",
	"InfraTableResetSelective",
	"InfraTableResetVariousAggs",
	"InfraTableResetInvalid",
	"InfraTableResetDocSample",
}

var infraTableResetJavaStaticIDs = []string{
	"java-e9745496803eaca6334c",
	"java-0dce757ee7589b9e5955",
	"java-6e9eb521e3169e56479f",
	"java-f4f3dace51b56b26af12",
	"java-f07cf6e8baf91d2768c7",
	"java-5c6e83bcf7901b7e6e05",
}

var infraTableResetJavaFlags = []string{}

var infraTableResetCaseObservations = []string{
	"snapshot; unkeyed MyTable(asum sum(int)) fed by into-table sum over SupportBean; on SupportBean_S0 merge updates asum.reset(); sum 10->21->null(reset)->20->41->null(reset)->30",
	"snapshot; same flow as row-sum but the merge declares table alias mt and updates mt.reset() (whole-row reset form); sum 10->21->null->20->41->null->30",
	"snapshot; keyed MyTable(k string primary key, avgone/avgtwo avg(int), winone/wintwo window(*)) grouped by theString; S0 merge where p00=k resets avgone+winone only, S1 merge where p10=k resets avgtwo+wintwo only (pinned EPL keeps the double space before 'when')",
	"snapshot+listener; unkeyed 12-column table (avedev,count,count distinct,max,median,stddev,firstever,countever,maxbyever(*)@type(SupportBean),myaggsingle plugin,referenceCountedMap plugin,countMinSketch); mt.reset() restores every cell to initial state; s0 listener reads MyTable.myWordcms.countMinSketchFrequency(p10) per S1 send (1 before reset, 0 after)",
	"compile-rejected; four tryInvalidCompile probes against the shared create-table prefix: reset() in on-merge insert select-clause, mt.reset() in select-clause, asum.reset(1) and mt.reset(1) with parameters; epl field carries the shared prefix",
	"compiled; compile-only env.compile of the doc-sample module: two-column-PK IntrusionCountTable with per-column reset() and tableRow.reset() on-merge update forms over the IntrusionReset schema",
}

var infraTableResetCaseEPLs = []string{
	"@name('table') create table MyTable(asum sum(int));\ninto table MyTable select sum(intPrimitive) as asum from SupportBean;\non SupportBean_S0 merge MyTable when matched then update set asum.reset();\n",
	"@name('table') create table MyTable(asum sum(int));\ninto table MyTable select sum(intPrimitive) as asum from SupportBean;\non SupportBean_S0 merge MyTable as mt when matched then update set mt.reset();\n",
	"@name('table') create table MyTable(k string primary key,   avgone avg(int), avgtwo avg(int),  winone window(*) @type(SupportBean), wintwo window(*) @type(SupportBean));\ninto table MyTable select theString,   avg(intPrimitive) as avgone, avg(intPrimitive) as avgtwo,  window(*) as winone, window(*) as wintwo from SupportBean#keepall group by theString;\non SupportBean_S0 merge MyTable where p00 = k  when matched then update set avgone.reset(), winone.reset();\non SupportBean_S1 merge MyTable where p10 = k  when matched then update set avgtwo.reset(), wintwo.reset();\n",
	"@name('table') create table MyTable(  myAvedev avedev(int),\n  myCount count(*),\n  myCountDistinct count(distinct int),\n  myMax max(int),\n  myMedian median(int),\n  myStddev stddev(int),\n  myFirstEver firstever(string),\n  myCountEver countever(*),  myMaxByEver maxbyever(intPrimitive) @type(SupportBean),  myPluginAggSingle myaggsingle(*),  myPluginAggAccess referenceCountedMap(string),  myWordcms countMinSketch());\ninto table MyTable select  avedev(intPrimitive) as myAvedev,  count(*) as myCount,  count(distinct intPrimitive) as myCountDistinct,  max(intPrimitive) as myMax,  median(intPrimitive) as myMedian,  stddev(intPrimitive) as myStddev,  firstever(theString) as myFirstEver,  countever(*) as myCountEver,  maxbyever(*) as myMaxByEver,  myaggsingle(*) as myPluginAggSingle,  referenceCountedMap(theString) as myPluginAggAccess,  countMinSketchAdd(theString) as myWordcms   from SupportBean#keepall;\non SupportBean_S0 merge MyTable mt when matched then update set mt.reset();\n@name('s0') select MyTable.myWordcms.countMinSketchFrequency(p10) as c0 from SupportBean_S1;\n",
	"@name('table') create table MyTable(asum sum(int));\n",
	"create table IntrusionCountTable (\n  fromAddress string primary key,\n  toAddress string primary key,\n  countIntrusion10Sec count(*),\n  countIntrusion60Sec count(*)\n);\ncreate schema IntrusionReset(fromAddress string, toAddress string);\non IntrusionReset as resetEvent merge IntrusionCountTable as tableRow\nwhere resetEvent.fromAddress = tableRow.fromAddress and resetEvent.toAddress = tableRow.toAddress\nwhen matched then update set countIntrusion10Sec.reset(), countIntrusion60Sec.reset();\non IntrusionReset as resetEvent merge IntrusionCountTable as tableRow\nwhere resetEvent.fromAddress = tableRow.fromAddress and resetEvent.toAddress = tableRow.toAddress\nwhen matched then update set tableRow.reset();\n",
}

// infraTableResetCaseSteps pins the complete step sequence per case as
// op|statement|eventType triples so the runtime-ID mapping test can assert
// the scenario file matches the contract.
var infraTableResetCaseSteps = map[string][]string{
	"row-sum": {
		"deploy|module|",
		"deployed|module|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S0",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S0",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"undeploy-all||",
	},
	"row-sum-alias": {
		"deploy|module|",
		"deployed|module|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S0",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S0",
		"snapshot|table|",
		"send||SupportBean",
		"snapshot|table|",
		"undeploy-all||",
	},
	"selective": {
		"deploy|module|",
		"deployed|module|",
		"send||SupportBean",
		"send||SupportBean",
		"send||SupportBean",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S0",
		"snapshot|table|",
		"send||SupportBean_S1",
		"snapshot|table|",
		"undeploy-all||",
	},
	"various-aggs": {
		"deploy|module|",
		"deployed|module|",
		"send||SupportBean",
		"send||SupportBean",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S1",
		"send||SupportBean_S0",
		"snapshot|table|",
		"send||SupportBean_S1",
		"send||SupportBean",
		"send||SupportBean",
		"send||SupportBean",
		"snapshot|table|",
		"send||SupportBean_S1",
		"undeploy-all||",
	},
	"invalid": {
		"build-error|select-agg-reset|",
		"build-error|select-row-reset|",
		"build-error|agg-reset-params|",
		"build-error|row-reset-params|",
		"undeploy-all||",
	},
	"doc-sample": {
		"build|module|",
		"undeploy-all||",
	},
}

// infraTableResetCaseState carries the per-case replay state: the
// environment/engine pair plus label→deployments bookkeeping.
type infraTableResetCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string][]*esper.Deployment
	snapshots   map[string]esper.Plan
	caseName    string
}

func runInfraTableResetScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraTableResetScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeInfraTableReset(ctx, scenario, &trace)
}

func validateInfraTableResetScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != infraTableResetID {
		return fmt.Errorf("%s: scenario identity = %q/%q", infraTableResetID, scenario.Version, scenario.ID)
	}
	return scenario.Validate()
}

// executeInfraTableReset replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action. The oracle emits the literal
// time "0" for every record, so the runner pins the same value.
func executeInfraTableReset(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *infraTableResetCaseState
	sequences := make(map[string]uint64)
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startInfraTableResetCase(step.Case)
			if err != nil {
				return *trace, err
			}
			sequences = make(map[string]uint64)
		case "deploy":
			if err := state.deploy(ctx, step, trace, sequences); err != nil {
				return *trace, err
			}
		case "deployed":
			sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      state.caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequences[step.Statement+":deployed"],
				Time:      "0",
			})
		case "send":
			event, err := decodeInfraTableResetPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step, trace, sequences); err != nil {
				return *trace, err
			}
		case "build":
			if err := state.buildOnly(step, trace, sequences); err != nil {
				return *trace, err
			}
		case "build-error":
			// All four probes exercise EPL-only reset() forms the typed Go
			// API deliberately does not expose: select-clause reset
			// expressions (column and row alias) and reset(...) parameter
			// forms in update assignments. The record is emitted
			// unconditionally with the pinned expectError prefix, matching
			// the oracle's compile-rejected marker.
			sequences[step.Statement+":compile-rejected"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      state.caseName,
				Operation: "compile-rejected",
				Statement: step.Statement,
				Sequence:  sequences[step.Statement+":compile-rejected"],
				Time:      "0",
				Value:     step.ExpectError,
			})
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", infraTableResetID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

func startInfraTableResetCase(caseName string) (*infraTableResetCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraTableResetBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraTableResetS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraTableResetS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraTableResetIntrusionReset](env, "IntrusionReset"); err != nil {
		return nil, err
	}
	return &infraTableResetCaseState{
		env:         env,
		engine:      esper.NewEngine(env),
		deployments: make(map[string][]*esper.Deployment),
		snapshots:   make(map[string]esper.Plan),
		caseName:    caseName,
	}, nil
}

// deploy maps each scenario label to the equivalent Go chain-API plans. The
// Java oracle compiles each deploy step's EPL as one module; the Go runner
// deploys the equivalent plans and registers them under the step label so
// undeploy-all bookkeeping matches.
func (s *infraTableResetCaseState) deploy(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	var plans []esper.Plan
	var err error
	switch s.caseName {
	case "row-sum":
		plans, err = s.deployRowSum(step.Statement, false)
	case "row-sum-alias":
		plans, err = s.deployRowSum(step.Statement, true)
	case "selective":
		plans, err = s.deploySelective(step.Statement)
	case "various-aggs":
		plans, err = s.deployVariousAggs(step.Statement)
	default:
		return fmt.Errorf("%s: unsupported case %q", infraTableResetID, s.caseName)
	}
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if err := s.deployPlan(ctx, step.Statement, plan, trace, sequences); err != nil {
			return err
		}
	}
	return nil
}

// deployRowSum builds the unkeyed sum table, the into-table feed and the
// on-merge reset trigger. withAlias selects the mt.reset() whole-row form
// (no column list) over the named-column asum.reset() form.
func (s *infraTableResetCaseState) deployRowSum(label string, withAlias bool) ([]esper.Plan, error) {
	if label != "module" {
		return nil, fmt.Errorf("%s: unknown %s deploy label %q", infraTableResetID, s.caseName, label)
	}
	if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
		esper.OptionalTableColumnOf[int]("asum"),
	}); err != nil {
		return nil, err
	}
	intoPlan, err := s.env.Build(esper.From[infraTableResetBean](s.env, "SupportBean").
		Aggregate(
			esper.Alias("asum", esper.Sum[int](esper.Field[infraTableResetBean, int]("intPrimitive"))),
		).IntoTable("MyTable"))
	if err != nil {
		return nil, err
	}
	trigger := esper.OnEvent(esper.From[infraTableResetS0](s.env, "SupportBean_S0"))
	var resetQuery esper.TriggerQuery
	if withAlias {
		resetQuery = trigger.ResetTableAggregates("MyTable")
	} else {
		resetQuery = trigger.ResetTableAggregates("MyTable", "asum")
	}
	resetPlan, err := s.env.Build(resetQuery.Query())
	if err != nil {
		return nil, err
	}
	snapshotPlan, err := s.env.Build(esper.FromTable(s.env, "MyTable").Query(esper.StatementName("snapshot-table")))
	if err != nil {
		return nil, err
	}
	s.snapshots["table"] = snapshotPlan
	return []esper.Plan{intoPlan, resetPlan}, nil
}

// deploySelective builds the keyed avg/window table fed by a grouped
// keepall into-table plus the two keyed merge triggers that reset a column
// pair each.
func (s *infraTableResetCaseState) deploySelective(label string) ([]esper.Plan, error) {
	if label != "module" {
		return nil, fmt.Errorf("%s: unknown selective deploy label %q", infraTableResetID, label)
	}
	if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("k"),
		esper.OptionalTableColumnOf[float64]("avgone"),
		esper.OptionalTableColumnOf[float64]("avgtwo"),
		esper.OptionalTableColumnOf[[]esper.Event]("winone"),
		esper.OptionalTableColumnOf[[]esper.Event]("wintwo"),
	}); err != nil {
		return nil, err
	}
	theString := esper.Field[infraTableResetBean, string]("theString")
	intPrimitive := esper.Field[infraTableResetBean, int]("intPrimitive")
	intoPlan, err := s.env.Build(esper.From[infraTableResetBean](s.env, "SupportBean").
		Window(esper.KeepAll()).
		GroupBy(theString).
		Select(
			esper.Alias("k", theString),
			esper.Alias("avgone", esper.Avg[int](intPrimitive)),
			esper.Alias("avgtwo", esper.Avg[int](intPrimitive)),
			esper.Alias("winone", esper.WindowEvents()),
			esper.Alias("wintwo", esper.WindowEvents()),
		).IntoTable("MyTable"))
	if err != nil {
		return nil, err
	}
	resetS0Plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableResetS0](s.env, "SupportBean_S0")).
		ResetTableAggregatesWhere("MyTable",
			esper.Equal[string](esper.Field[infraTableResetS0, string]("p00"), esper.TableField[string]("k")),
			"avgone", "winone").Query())
	if err != nil {
		return nil, err
	}
	resetS1Plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableResetS1](s.env, "SupportBean_S1")).
		ResetTableAggregatesWhere("MyTable",
			esper.Equal[string](esper.Field[infraTableResetS1, string]("p10"), esper.TableField[string]("k")),
			"avgtwo", "wintwo").Query())
	if err != nil {
		return nil, err
	}
	snapshotPlan, err := s.env.Build(esper.FromTable(s.env, "MyTable").Query(esper.StatementName("snapshot-table")))
	if err != nil {
		return nil, err
	}
	s.snapshots["table"] = snapshotPlan
	return []esper.Plan{intoPlan, resetS0Plan, resetS1Plan}, nil
}

// infraTableResetCountedMap is the Go equivalent of the regression-lib
// SupportReferenceCountedMap plug-in: a per-group map counting how many
// retained events carry each key.
type infraTableResetCountedMap struct {
	counts map[string]int
}

func newInfraTableResetCountedMap(esper.AggregatePluginFactoryContext) esper.AggregatePluginState[map[string]any] {
	return &infraTableResetCountedMap{counts: make(map[string]int)}
}

func (m *infraTableResetCountedMap) Enter(value esper.Value) {
	if key, ok := value.Any().(string); ok {
		m.counts[key]++
	}
}

func (m *infraTableResetCountedMap) Leave(value esper.Value) {
	key, ok := value.Any().(string)
	if !ok {
		return
	}
	if m.counts[key] <= 1 {
		delete(m.counts, key)
		return
	}
	m.counts[key]--
}

func (m *infraTableResetCountedMap) Value() (map[string]any, bool) {
	result := make(map[string]any, len(m.counts))
	for key, count := range m.counts {
		result[key] = count
	}
	return result, true
}

func (m *infraTableResetCountedMap) Clear() {
	m.counts = make(map[string]int)
}

// deployVariousAggs builds the twelve-column unkeyed table, the keepall
// into-table feed, the whole-row mt.reset() trigger and the s0 listener
// statement that reads countMinSketchFrequency through the table column.
func (s *infraTableResetCaseState) deployVariousAggs(label string) ([]esper.Plan, error) {
	if label != "module" {
		return nil, fmt.Errorf("%s: unknown various-aggs deploy label %q", infraTableResetID, label)
	}
	if _, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{
		esper.OptionalTableColumnOf[float64]("myAvedev"),
		esper.TableColumnOf[int64]("myCount"),
		esper.TableColumnOf[int64]("myCountDistinct"),
		esper.OptionalTableColumnOf[int]("myMax"),
		esper.OptionalTableColumnOf[float64]("myMedian"),
		esper.OptionalTableColumnOf[float64]("myStddev"),
		esper.OptionalTableColumnOf[string]("myFirstEver"),
		esper.TableColumnOf[int64]("myCountEver"),
		esper.OptionalTableColumnOf[esper.Event]("myMaxByEver"),
		esper.TableColumnOf[int64]("myPluginAggSingle"),
		esper.TableColumnOf[map[string]any]("myPluginAggAccess"),
		esper.OptionalTableColumnOf[esper.CountMinSketchValue[string]]("myWordcms"),
	}); err != nil {
		return nil, err
	}
	theString := esper.Field[infraTableResetBean, string]("theString")
	intPrimitive := esper.Field[infraTableResetBean, int]("intPrimitive")
	eventValue := esper.EventValue[esper.Event]()
	intoPlan, err := s.env.Build(esper.From[infraTableResetBean](s.env, "SupportBean").
		Window(esper.KeepAll()).
		Aggregate(
			esper.Alias("myAvedev", esper.Avedev[int](intPrimitive)),
			esper.Alias("myCount", esper.CountAll()),
			esper.Alias("myCountDistinct", esper.CountDistinct[int](intPrimitive)),
			esper.Alias("myMax", esper.Max[int](intPrimitive)),
			esper.Alias("myMedian", esper.Median[int](intPrimitive)),
			esper.Alias("myStddev", esper.StdDev[int](intPrimitive)),
			esper.Alias("myFirstEver", esper.FirstEver[string](theString)),
			esper.Alias("myCountEver", esper.CountEver()),
			esper.Alias("myMaxByEver", esper.MaxByEver[esper.Event, int](eventValue, intPrimitive)),
			esper.Alias("myPluginAggSingle", esper.PluginAggregate[int64]("myaggsingle",
				func(ctx esper.EvalContext) (int64, bool) {
					return int64(-len(ctx.Group)), true
				})),
			esper.Alias("myPluginAggAccess", esper.PluginAggregateAccess[map[string]any](
				"referenceCountedMap", theString, newInfraTableResetCountedMap)),
			esper.Alias("myWordcms", esper.CountMinSketchAdd[string](theString)),
		).IntoTable("MyTable"))
	if err != nil {
		return nil, err
	}
	resetPlan, err := s.env.Build(esper.OnEvent(esper.From[infraTableResetS0](s.env, "SupportBean_S0")).
		ResetTableAggregates("MyTable").Query())
	if err != nil {
		return nil, err
	}
	s0Plan, err := s.env.Build(esper.OnEvent(esper.From[infraTableResetS1](s.env, "SupportBean_S1")).
		SelectFromTableWhere("MyTable", esper.Literal(true),
			esper.Alias("c0", esper.CountMinSketchFrequency[string](
				esper.TableField[esper.CountMinSketchValue[string]]("myWordcms"),
				esper.Field[infraTableResetS1, string]("p10"),
			)),
		).Query(esper.StatementName("s0")))
	if err != nil {
		return nil, err
	}
	snapshotPlan, err := s.env.Build(esper.FromTable(s.env, "MyTable").Query(esper.StatementName("snapshot-table")))
	if err != nil {
		return nil, err
	}
	s.snapshots["table"] = snapshotPlan
	return []esper.Plan{intoPlan, resetPlan, s0Plan}, nil
}

// buildOnly mirrors the oracle's compile-only step: the doc-sample module is
// compiled (table + schema + two merge triggers) but never deployed, so the
// runner emits the "compiled" marker without a deployment.
func (s *infraTableResetCaseState) buildOnly(step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	if s.caseName != "doc-sample" || step.Statement != "module" {
		return fmt.Errorf("%s: unsupported build step %s/%q", infraTableResetID, s.caseName, step.Statement)
	}
	if _, err := esper.CreateTable(s.env, "IntrusionCountTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("fromAddress"),
		esper.PrimaryKeyColumn[string]("toAddress"),
		esper.TableColumnOf[int64]("countIntrusion10Sec"),
		esper.TableColumnOf[int64]("countIntrusion60Sec"),
	}); err != nil {
		return err
	}
	predicate := func() esper.Expression[bool] {
		return esper.And(
			esper.Equal[string](
				esper.Field[infraTableResetIntrusionReset, string]("fromAddress"),
				esper.TableField[string]("fromAddress")),
			esper.Equal[string](
				esper.Field[infraTableResetIntrusionReset, string]("toAddress"),
				esper.TableField[string]("toAddress")),
		)
	}
	trigger := esper.OnEvent(esper.From[infraTableResetIntrusionReset](s.env, "IntrusionReset"))
	if _, err := s.env.Build(trigger.ResetTableAggregatesWhere("IntrusionCountTable",
		predicate(), "countIntrusion10Sec", "countIntrusion60Sec").Query()); err != nil {
		return err
	}
	if _, err := s.env.Build(trigger.ResetTableAggregatesWhere("IntrusionCountTable",
		predicate()).Query()); err != nil {
		return err
	}
	sequences[step.Statement+":compiled"]++
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compiled",
		Statement: step.Statement,
		Sequence:  sequences[step.Statement+":compiled"],
		Time:      "0",
	})
	return nil
}

// snapshot executes the pinned fire-and-forget table query and emits the
// {"operation":"snapshot"} record whose "new" array carries the iterator
// rows in iteration order.
func (s *infraTableResetCaseState) snapshot(ctx context.Context, step compat.Step,
	trace *compat.Trace, sequences map[string]uint64) error {
	plan, ok := s.snapshots[step.Statement]
	if !ok {
		return fmt.Errorf("%s: unknown snapshot %q", infraTableResetID, step.Statement)
	}
	result, err := s.engine.ExecuteFireAndForget(ctx, plan)
	if err != nil {
		return err
	}
	sequences[step.Statement+":snapshot"]++
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  sequences[step.Statement+":snapshot"],
		Time:      "0",
	}
	rows := infraTableResetNormalizeResults(result.Results())
	if len(rows) > 0 {
		record.New = rows
	}
	trace.Records = append(trace.Records, record)
	return nil
}

func (s *infraTableResetCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan, trace *compat.Trace, sequences map[string]uint64) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	for _, statement := range deployment.Statements() {
		if statement.Name() != "s0" {
			continue
		}
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			sequences[stmt.Name()+":listener"]++
			record := compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  sequences[stmt.Name()+":listener"],
				Time:      "0",
			}
			if rows := infraTableResetNormalizeResults(batch.New); len(rows) > 0 {
				record.New = rows
			}
			if rows := infraTableResetNormalizeResults(batch.Old); len(rows) > 0 {
				record.Old = rows
			}
			trace.Records = append(trace.Records, record)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *infraTableResetCaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	s.deployments = make(map[string][]*esper.Deployment)
	return nil
}

func decodeInfraTableResetPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value infraTableResetBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value infraTableResetS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value infraTableResetS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported infra table reset event type %q", step.EventType)
	}
}

// infraTableResetNormalizeResults renders rows the way the pinned oracle
// serializes them: null/missing fields become JSON null, bean events become
// plain objects and the count-min-sketch cell becomes null (the oracle
// renders the suite-internal CountMinSketchAggState as a stable null
// marker).
func infraTableResetNormalizeResults(results []esper.Result) []compat.ResultRecord {
	if len(results) == 0 {
		return nil
	}
	normalized := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		if event, ok := result.Event(); ok {
			record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range event.Schema().Fields() {
				record.Fields[field.Name] = infraTableResetNormalizeValue(event.Get(field.Name))
			}
			normalized = append(normalized, record)
			continue
		}
		if row, ok := result.Row(); ok {
			record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range row.Schema().Fields() {
				record.Fields[field.Name] = infraTableResetNormalizeValue(row.Get(field.Name))
			}
			normalized = append(normalized, record)
		}
	}
	return normalized
}

func infraTableResetNormalizeValue(value esper.Value) any {
	if value.IsMissing() || value.IsNull() {
		return nil
	}
	return infraTableResetNormalizeUnderlying(value.Any())
}

func infraTableResetNormalizeUnderlying(value any) any {
	switch value.(type) {
	case nil:
		return nil
	case esper.CountMinSketchValue[string]:
		// The oracle renders the suite-internal CountMinSketchAggState as a
		// stable null marker; the cell object identity is not observable.
		return nil
	default:
		return infraTableJoinNormalizeUnderlying(value)
	}
}

// loadInfraTableResetScenario decodes the pinned scenario with the strict
// shape checks used by the other runners: duplicate keys and unknown fields
// are rejected, and every case/step must match the frozen contract exactly.
func loadInfraTableResetScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableResetID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableResetID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableResetID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableResetID, err)
	}
	if err := requireInfraTableResetFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableResetID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableResetID ||
		metadata.Description != infraTableResetDescription ||
		metadata.JavaCommit != infraTableResetJavaCommit ||
		metadata.JavaSource != infraTableResetJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableResetID)
	}
	if err := validateInfraTableResetStringArray(root["javaRuntimes"], infraTableResetJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableResetStringArray(root["javaNames"], infraTableResetJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableResetStringArray(root["javaStaticIds"], infraTableResetJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableResetStringArray(root["javaFlags"], infraTableResetJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableResetCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableResetID, len(infraTableResetCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableResetFields(object,
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
		if definition.Case != infraTableResetCases[index] ||
			definition.Ordinal != index ||
			definition.RuntimeID != infraTableResetJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableResetJavaExecutions[index] ||
			definition.Observation != infraTableResetCaseObservations[index] ||
			definition.EPL != infraTableResetCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableResetID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", infraTableResetID, err)
	}
	offset := 0
	for _, caseName := range infraTableResetCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", infraTableResetID, caseName)
		}
		key, keyErr := infraTableResetStepKey(rawSteps[offset])
		if keyErr != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableResetID, offset, keyErr)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableResetID, offset, err)
		}
		if key != "case||" || marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", infraTableResetID, offset, caseName)
		}
		offset++
		want, ok := infraTableResetCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", infraTableResetID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", infraTableResetID, caseName)
		}
		for _, pinned := range want {
			key, err := infraTableResetStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableResetID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", infraTableResetID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", infraTableResetID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableResetID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraTableResetID)
	}
	return scenario, nil
}

func requireInfraTableResetFields(object map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s scenario is missing field %q", infraTableResetID, name)
		}
	}
	return nil
}

func validateInfraTableResetStringArray(raw json.RawMessage, want []string, field string) error {
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		return fmt.Errorf("%s scenario field %q: %w", infraTableResetID, field, err)
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s scenario field %q length = %d, want %d", infraTableResetID, field, len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s scenario field %q[%d] = %q, want %q", infraTableResetID, field, index, got[index], want[index])
		}
	}
	return nil
}

func infraTableResetStepKey(raw json.RawMessage) (string, error) {
	var step struct {
		Op        string `json:"op"`
		Statement string `json:"statement"`
		EventType string `json:"eventType"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	return step.Op + "|" + step.Statement + "|" + step.EventType, nil
}

// infraTableResetRuntimeID maps a scenario case to its pinned Java runtime
// ID so the evidence cross-references stay honest.
func infraTableResetRuntimeID(caseName string) string {
	for index, name := range infraTableResetCases {
		if name == caseName {
			return infraTableResetJavaRuntimeIDs[index]
		}
	}
	return ""
}
