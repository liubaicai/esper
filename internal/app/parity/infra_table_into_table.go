package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type infraTableIntoTableBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type infraTableIntoTableS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type infraTableIntoTableNumeric struct {
	BigInt big.Int `esper:"bigint"`
	BigDec big.Rat `esper:"bigdec"`
}

type infraTableIntoTableManyArray struct {
	ID     string `esper:"id"`
	Value  int    `esper:"value"`
	IntOne []int  `esper:"intOne"`
	IntTwo []int  `esper:"intTwo"`
}

const infraTableIntoTableJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var infraTableIntoTableJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableIntoTable.java",
}

// Case order fixes the runtime-ID index mapping below. The nine executions
// share one Java class, one scenario, one oracle and one runner surface (the
// ECSM multi-runtime precedent); the InfraBoundUnbound execution contributes
// three scenario cases because its sub-phases run against fresh tables and
// statements, and all three map to the single inventory runtime ID.
var infraTableIntoTableJavaRuntimeIDs = []string{
	"java-runtime-36615ff5ace54c29450b",
	"java-runtime-01df86362eaeba3bb58d",
	"java-runtime-338e401bfe934ccd41f0",
	"java-runtime-a71e110710b112d908b0",
	"java-runtime-d2ce221ae91707742548",
	"java-runtime-ad72afcd0fe33a7b5598",
	"java-runtime-fe0db4bb05cef63d525d",
	"java-runtime-8e288e9ab881caa8ffb4",
	"java-runtime-6bfb9dbe40659f113ce9",
}

var infraTableIntoTableJavaExecutions = []string{
	"InfraIntoTableUnkeyedSimpleSameModule",
	"InfraIntoTableUnkeyedSimpleTwoModule",
	"InfraBoundUnbound",
	"InfraIntoTableWindowSortedFromJoin",
	"InfraTableIntoTableNoKeys",
	"InfraTableIntoTableWithKeys",
	"InfraTableBigNumberAggregation",
	"InfraIntoTableMultikeyWArraySingleArrayKeyed",
	"InfraIntoTableMultikeyWArrayTwoKeyed",
}

var infraTableIntoTableCases = []string{
	"unkeyed-simple-same-module",
	"unkeyed-simple-two-module",
	"bound-unbound-minmax",
	"bound-unbound-lastfirst-window",
	"bound-unbound-sorted-minmaxby",
	"window-sorted-from-join",
	"no-keys",
	"with-keys",
	"big-number-avg-sum",
	"multikey-warray-single",
	"multikey-warray-two",
}

// infraTableIntoTableRuntimeID maps each scenario case to the inventory
// runtime ID declared for it; the three BoundUnbound sub-phase cases share
// that execution's single runtime ID.
func infraTableIntoTableRuntimeID(caseName string) string {
	switch caseName {
	case "unkeyed-simple-same-module":
		return "java-runtime-36615ff5ace54c29450b"
	case "unkeyed-simple-two-module":
		return "java-runtime-01df86362eaeba3bb58d"
	case "bound-unbound-minmax", "bound-unbound-lastfirst-window", "bound-unbound-sorted-minmaxby":
		return "java-runtime-338e401bfe934ccd41f0"
	case "window-sorted-from-join":
		return "java-runtime-a71e110710b112d908b0"
	case "no-keys":
		return "java-runtime-d2ce221ae91707742548"
	case "with-keys":
		return "java-runtime-ad72afcd0fe33a7b5598"
	case "big-number-avg-sum":
		return "java-runtime-fe0db4bb05cef63d525d"
	case "multikey-warray-single":
		return "java-runtime-8e288e9ab881caa8ffb4"
	case "multikey-warray-two":
		return "java-runtime-6bfb9dbe40659f113ce9"
	}
	return ""
}

// runInfraTableIntoTableScenario replays the into-table family: unkeyed
// count(*) accumulation within one and across two modules, the
// bound/unbound aggregation matrix with its exact compile-reject messages,
// window/sorted access columns fed from a join and read by FAF, keyed and
// unkeyed sums observed through correlated table subqueries, exact
// big-number averages and sums over a last-event stream, and int[]
// content-equality primary keys. Milestones are harness-only checkpoints
// and are dropped; Java's S0 sends trigger its iterate reader statement,
// whose whole-row observation equals a direct table snapshot, so the Go
// side skips those sends for the bound/unbound cases.
func runInfraTableIntoTableScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range infraTableIntoTableCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runInfraTableIntoTableCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("infra table into-table case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("infra table into-table scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

// infraTableIntoTableListener captures consumer deliveries so the runner can
// emit them as listener records right after the triggering send, mirroring
// the oracle's per-statement sequence counters.
type infraTableIntoTableListener struct {
	statement *esper.Statement
	batches   [][]esper.Result
}

type infraTableIntoTableFixture struct {
	env           *esper.Environment
	engine        *esper.Engine
	plans         []esper.Plan
	snapshots     map[string]esper.Plan
	listenerNames []string
	probes        map[string]func(env *esper.Environment) esper.Query
}

func runInfraTableIntoTableCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	fixture, err := buildInfraTableIntoTableCase(caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = fixture.engine.Close(context.Background()) }()
	var deployed *esper.Deployment
	for _, plan := range fixture.plans {
		current, err := fixture.engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, err
		}
		deployed = current
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var listeners []*infraTableIntoTableListener
	sendsSeen := false
	deferredRowLabels := deferredUnkeyedRowLabels(caseName)
	for _, name := range fixture.listenerNames {
		statement, ok := deployed.Statement(name)
		if !ok {
			return trace, fmt.Errorf("infra table into-table case %q missing consumer %q", caseName, name)
		}
		listener := &infraTableIntoTableListener{statement: statement}
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			listener.batches = append(listener.batches, batch.New)
			return nil
		}); err != nil {
			return trace, err
		}
		listeners = append(listeners, listener)
	}
	listenerSequences := make(map[string]uint64)
	emitSnapshot := func(statement string, rows []compat.ResultRecord) {
		if rows == nil {
			rows = []compat.ResultRecord{}
		}
		sort.SliceStable(rows, func(left, right int) bool {
			// AnyOrder row sets canonicalize by their sorted-key JSON form.
			leftJSON, _ := json.Marshal(rows[left].Fields)
			rightJSON, _ := json.Marshal(rows[right].Fields)
			return string(leftJSON) < string(rightJSON)
		})
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "snapshot",
			Statement: statement,
			Time:      "1970-01-01T00:00:00Z",
			New:       rows,
		})
	}
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "send":
			if step.EventType == "SupportBean_S0" && isBoundUnboundCase(caseName) {
				// Java's S0 send triggers its iterate reader statement; the
				// Go side observes the same table state directly.
				continue
			}
			payload, err := decodeInfraTableIntoTablePayload(step)
			if err != nil {
				return trace, err
			}
			sendsSeen = true
			if err := fixture.engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
			for _, listener := range listeners {
				for _, newResults := range listener.batches {
					listenerSequences[listener.statement.Name()]++
					record := compat.TraceRecord{
						Case:      caseName,
						Operation: "listener",
						Statement: listener.statement.Name(),
						Sequence:  listenerSequences[listener.statement.Name()],
						Time:      "1970-01-01T00:00:00Z",
					}
					record.New = compat.NormalizeResults(newResults)
					trace.Records = append(trace.Records, record)
				}
				listener.batches = nil
			}
		case "snapshot":
			snapshotPlan, ok := fixture.snapshots[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown infra table into-table snapshot %q", step.Statement)
			}
			result, err := fixture.engine.ExecuteFireAndForget(ctx, snapshotPlan)
			if err != nil {
				return trace, err
			}
			rows := normalizeInfraTableIntoTableRows(
				compat.NormalizeResults(materializedTableResults(result.Results())))
			// Representation difference (registered): Go materializes an
			// unkeyed into-table row at deployment while Java defers it to
			// the first contribution. Pre-contribution snapshots therefore
			// normalize to the Java-observable empty set.
			if sendsSeen == false && len(rows) == 1 && deferredRowLabels[step.Statement] {
				allDefaulted := true
				for _, field := range rows[0].Fields {
					if !isZeroDefault(field) {
						allDefaulted = false
						break
					}
				}
				if allDefaulted {
					rows = []compat.ResultRecord{}
				}
			}
			emitSnapshot(step.Statement, rows)
		case "build-error":
			probe, ok := fixture.probes[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown infra table into-table build probe %q", step.Statement)
			}
			_, buildErr := fixture.env.Build(probe(fixture.env))
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "build-error",
				Statement: step.Statement,
				Time:      "1970-01-01T00:00:00Z",
			}
			switch {
			case buildErr == nil:
				record.Value = "<no-error>"
			default:
				// Mirror the oracle's ex.getMessage(): diagnostic text plus
				// the bracketed statement source that Java appends.
				message := buildErr.Error()
				var espErr *esper.Error
				if errors.As(buildErr, &espErr) && espErr.Message != "" {
					message = espErr.Message
				}
				record.Value = message + " [" + step.Epl + "]"
			}
			trace.Records = append(trace.Records, record)
		default:
			return trace, fmt.Errorf("unsupported infra table into-table step op %q", step.Op)
		}
	}
	return trace, nil
}

// normalizeInfraTableIntoTableRows renders exact numeric aggregates the way
// the pinned oracle renders BigDecimal/BigInteger values: plain decimal
// text tokens.
func normalizeInfraTableIntoTableRows(rows []compat.ResultRecord) []compat.ResultRecord {
	for rowIndex := range rows {
		for field, value := range rows[rowIndex].Fields {
			if text, ok := bigNumberDecimalString(value); ok {
				rows[rowIndex].Fields[field] = text
			}
		}
	}
	return rows
}

func bigNumberDecimalString(value any) (string, bool) {
	switch typed := value.(type) {
	case *big.Int:
		return typed.String(), true
	case big.Int:
		return typed.String(), true
	case *big.Rat:
		return bigRatDecimalString(typed), true
	case big.Rat:
		return bigRatDecimalString(&typed), true
	}
	return "", false
}

// bigRatDecimalString renders a rational in plain decimal notation when the
// value is integral and falls back to trimmed fixed-point expansion for
// fractional values; BigDecimal-style trailing zeros are removed.
func bigRatDecimalString(value *big.Rat) string {
	if value.IsInt() {
		return new(big.Int).Quo(value.Num(), value.Denom()).String()
	}
	text := value.FloatString(12)
	for len(text) > 0 && text[len(text)-1] == '0' {
		text = text[:len(text)-1]
	}
	if len(text) > 0 && text[len(text)-1] == '.' {
		text = text[:len(text)-1]
	}
	return text
}

func isBoundUnboundCase(caseName string) bool {
	switch caseName {
	case "bound-unbound-minmax", "bound-unbound-lastfirst-window", "bound-unbound-sorted-minmaxby":
		return true
	}
	return false
}

func decodeInfraTableIntoTablePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    *string `json:"theString"`
			IntPrimitive int     `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		event := infraTableIntoTableBean{IntPrimitive: payload.IntPrimitive}
		if payload.TheString != nil {
			event.TheString = *payload.TheString
		}
		return event, nil
	case "SupportBean_S0":
		var payload struct {
			ID  int    `json:"id"`
			P00 string `json:"p00"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		return infraTableIntoTableS0{ID: payload.ID, P00: payload.P00}, nil
	case "SupportBeanNumeric":
		var payload struct {
			BigInt string `json:"bigint"`
			BigDec string `json:"bigdec"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		event := infraTableIntoTableNumeric{}
		if _, ok := event.BigInt.SetString(payload.BigInt, 10); !ok {
			return nil, fmt.Errorf("invalid bigint literal %q", payload.BigInt)
		}
		if _, ok := event.BigDec.SetString(payload.BigDec); !ok {
			return nil, fmt.Errorf("invalid bigdec literal %q", payload.BigDec)
		}
		return event, nil
	case "SupportEventWithManyArray":
		var payload struct {
			ID     string `json:"id"`
			Value  int    `json:"value"`
			IntOne []int  `json:"intOne"`
			IntTwo *[]int `json:"intTwo"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, err
		}
		event := infraTableIntoTableManyArray{ID: payload.ID, Value: payload.Value, IntOne: payload.IntOne}
		if payload.IntTwo != nil {
			event.IntTwo = *payload.IntTwo
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported infra table into-table event type %q", step.EventType)
	}
}

func infraTableIntoTableSnapshot(env *esper.Environment, fixture *infraTableIntoTableFixture, label, table string) error {
	plan, err := env.Build(esper.FromTable(env, table).Query(esper.StatementName("snapshot-" + label)))
	if err != nil {
		return err
	}
	fixture.snapshots[label] = plan
	return nil
}

func optionalAggColumn(name, aggName, description string) esper.TableColumn {
	return esper.OptionalTableColumnOf[any](name, esper.WithTableAgg(aggName, description, false))
}

func optionalBoundAggColumn(name, aggName, description string) esper.TableColumn {
	return esper.OptionalTableColumnOf[int](name, esper.WithTableAgg(aggName, description, true))
}

// deferredUnkeyedRowLabels lists snapshot labels of unkeyed single-row
// tables whose logical row Java defers to the first contribution.
func deferredUnkeyedRowLabels(caseName string) map[string]bool {
	switch caseName {
	case "unkeyed-simple-same-module", "unkeyed-simple-two-module":
		return map[string]bool{"tbl": true}
	case "no-keys", "with-keys":
		return map[string]bool{"create-state": true}
	}
	return nil
}

func isZeroDefault(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case int:
		return typed == 0
	case int64:
		return typed == 0
	case float64:
		return typed == 0
	case string:
		return typed == ""
	}
	return false
}

// materializedTableResults drops the synthetic all-absent row that plain
// table scans expose for empty unkeyed tables: Java's create-table iterator
// yields no rows until the first into-table contribution materializes one.
func materializedTableResults(results []esper.Result) []esper.Result {
	kept := make([]esper.Result, 0, len(results))
	for _, result := range results {
		event, ok := result.Event()
		if !ok {
			kept = append(kept, result)
			continue
		}
		present := false
		for _, field := range event.Schema().Fields() {
			if event.Get(field.Name).IsPresent() {
				present = true
				break
			}
		}
		if present {
			kept = append(kept, result)
		}
	}
	return kept
}

// buildInfraTableIntoTableCase assembles one case's schemas, table and
// statements in pinned deployment order and prepares its snapshot plans and
// build-error probes.
// buildInfraTableIntoTableCase assembles one case's schemas, table and
// statements in pinned deployment order and prepares its snapshot plans and
// build-error probes. Each case mirrors its pinned execution: fresh
// environment, fresh table, statements deployed in declaration order.
func buildInfraTableIntoTableCase(caseName string) (*infraTableIntoTableFixture, error) {
	env := esper.NewEnvironment()
	fixture := &infraTableIntoTableFixture{
		env: env,
		engine: esper.NewEngine(env,
			esper.WithStartTime(time.Unix(0, 0).UTC()),
			esper.WithRuntimeURI(infraTableIntoTableRuntimeID(caseName)),
		),
		snapshots: map[string]esper.Plan{},
		probes:    map[string]func(*esper.Environment) esper.Query{},
	}
	switch caseName {
	case "unkeyed-simple-same-module", "unkeyed-simple-two-module":
		twoModule := caseName == "unkeyed-simple-two-module"
		if _, err := esper.RegisterStruct[infraTableIntoTableBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		columns := []esper.TableColumn{esper.TableColumnOf[int64]("mycnt")}
		tableTarget := "MyTable"
		var snapshotPlan esper.Plan
		var err error
		if twoModule {
			module, moduleErr := env.RegisterModule("itto-two", esper.PublicModule())
			if moduleErr != nil {
				return nil, moduleErr
			}
			if _, err := module.RegisterTable("MyTable", columns); err != nil {
				return nil, err
			}
			tableTarget = module.QualifiedName("MyTable")
			createPlan, planErr := module.Build(module.Table("MyTable").Query(esper.StatementName("tbl-create")))
			if planErr != nil {
				return nil, planErr
			}
			fixture.plans = append(fixture.plans, createPlan)
			snapshotPlan, err = env.Build(esper.FromTableInModule(env, module.Name(), "MyTable").Query(
				esper.StatementName("snapshot-tbl")))
			if err != nil {
				return nil, err
			}
		} else {
			if _, err := esper.CreateTable(env, "MyTable", columns); err != nil {
				return nil, err
			}
			snapshotPlan, err = env.Build(esper.FromTable(env, "MyTable").Query(esper.StatementName("snapshot-tbl")))
			if err != nil {
				return nil, err
			}
		}
		intoPlan, err := env.Build(esper.From[infraTableIntoTableBean](env, "SupportBean").Aggregate(
			esper.Alias("mycnt", esper.CountAll()),
		).IntoTable(tableTarget))
		if err != nil {
			return nil, err
		}
		fixture.plans = append(fixture.plans, intoPlan)
		fixture.snapshots["tbl"] = snapshotPlan

	case "bound-unbound-minmax", "bound-unbound-lastfirst-window", "bound-unbound-sorted-minmaxby":
		theString := func(event *infraTableIntoTableBean) esper.Expression[string] {
			return esper.Field[infraTableIntoTableBean, string]("theString")
		}
		_ = theString
		intPrimitive := esper.Field[infraTableIntoTableBean, int]("intPrimitive")
		eventValue := func() esper.Expression[esper.Event] { return esper.EventValue[esper.Event]() }
		columns := []esper.TableColumn{}
		switch caseName {
		case "bound-unbound-minmax":
			columns = append(columns,
				optionalBoundAggColumn("maxb", "max", "max(int)"),
				esper.OptionalTableColumnOf[int]("maxu", esper.WithTableAgg("maxever", "maxever(int)", false)),
				optionalBoundAggColumn("minb", "min", "min(int)"),
				esper.OptionalTableColumnOf[int]("minu", esper.WithTableAgg("minever", "minever(int)", false)),
			)
		case "bound-unbound-lastfirst-window":
			columns = append(columns,
				optionalAggColumn("lasteveru", "lastever", "lastever(*)"),
				optionalAggColumn("firsteveru", "firstever", "firstever(*)"),
				optionalAggColumn("windowb", "window", "window(*)"),
			)
		case "bound-unbound-sorted-minmaxby":
			columns = append(columns,
				optionalAggColumn("maxbyeveru", "maxbyever", "maxbyever(intPrimitive)"),
				optionalAggColumn("minbyeveru", "minbyever", "minbyever(intPrimitive)"),
				optionalAggColumn("sortedb", "sorted", "sorted(intPrimitive)"),
			)
		}
		if _, err := esper.RegisterStruct[infraTableIntoTableBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraTableIntoTableS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		if _, err := esper.CreateTable(env, "varagg", columns); err != nil {
			return nil, err
		}
		boundStream := esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.LengthWindow(2))
		unboundStream := esper.From[infraTableIntoTableBean](env, "SupportBean")
		var boundPlan, unboundPlan esper.Plan
		var err error
		switch caseName {
		case "bound-unbound-minmax":
			boundPlan, err = env.Build(boundStream.Aggregate(
				esper.Alias("maxb", esper.Max[int](intPrimitive)),
				esper.Alias("minb", esper.Min[int](intPrimitive)),
			).IntoTable("varagg"))
			if err != nil {
				return nil, err
			}
			unboundPlan, err = env.Build(unboundStream.Aggregate(
				esper.Alias("maxu", esper.MaxEver[int](intPrimitive)),
				esper.Alias("minu", esper.MinEver[int](intPrimitive)),
			).IntoTable("varagg"))
			if err != nil {
				return nil, err
			}
			fixture.probes["invalid-unbound-max-into-bound"] = func(env *esper.Environment) esper.Query {
				stream := esper.From[infraTableIntoTableBean](env, "SupportBean")
				return stream.Aggregate(
					esper.Alias("maxb", esper.Max[int](intPrimitive)),
				).IntoTable("varagg")
			}
		case "bound-unbound-lastfirst-window":
			boundPlan, err = env.Build(boundStream.Aggregate(
				esper.Alias("windowb", esper.WindowEvents()),
			).IntoTable("varagg"))
			if err != nil {
				return nil, err
			}
			unboundPlan, err = env.Build(unboundStream.Aggregate(
				esper.Alias("lasteveru", esper.LastEver[esper.Event](eventValue())),
				esper.Alias("firsteveru", esper.FirstEver[esper.Event](eventValue())),
			).IntoTable("varagg"))
			if err != nil {
				return nil, err
			}
			fixture.probes["invalid-last-into-table"] = func(env *esper.Environment) esper.Query {
				return esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
					esper.Alias("lasteveru", esper.Last[esper.Event](eventValue())),
				).IntoTable("varagg")
			}
			fixture.probes["invalid-lastever-into-window"] = func(env *esper.Environment) esper.Query {
				return esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
					esper.Alias("windowb", esper.LastEver[esper.Event](eventValue())),
				).IntoTable("varagg")
			}
			fixture.probes["invalid-lastever-null"] = func(env *esper.Environment) esper.Query {
				return esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
					esper.Alias("lasteveru", esper.LastEver[any](esper.Literal[any](nil))),
				).IntoTable("varagg")
			}
		case "bound-unbound-sorted-minmaxby":
			boundPlan, err = env.Build(boundStream.Aggregate(
				esper.Alias("sortedb", esper.SortedEvents(esper.Ascending(intPrimitive))),
			).IntoTable("varagg"))
			if err != nil {
				return nil, err
			}
			// Java's pinned statements use the zero-argument maxbyever()/
			// minbyever() forms whose sort specification is inherited from
			// the table column declaration. The Go projections spell the
			// same declared key explicitly - observationally identical
			// values under the frozen declaration (registered
			// representation choice).
			unboundPlan, err = env.Build(unboundStream.Aggregate(
				esper.Alias("maxbyeveru", esper.MaxByEver[esper.Event, int](eventValue(), intPrimitive)),
				esper.Alias("minbyeveru", esper.MinByEver[esper.Event, int](eventValue(), intPrimitive)),
			).IntoTable("varagg"))
			if err != nil {
				return nil, err
			}
			fixture.probes["invalid-maxby-sortexpr"] = func(env *esper.Environment) esper.Query {
				return esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
					esper.Alias("maxbyeveru", esper.MaxBy[esper.Event, int](eventValue(), intPrimitive)),
				).IntoTable("varagg")
			}
			fixture.probes["invalid-maxbyever-into-sorted"] = func(env *esper.Environment) esper.Query {
				return esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.LengthWindow(2)).Aggregate(
					esper.Alias("sortedb", esper.MaxByEver[esper.Event, int](eventValue())),
				).IntoTable("varagg")
			}
		}
		fixture.plans = append(fixture.plans, boundPlan, unboundPlan)
		if err := infraTableIntoTableSnapshot(env, fixture, "varagg-state", "varagg"); err != nil {
			return nil, err
		}

	case "window-sorted-from-join":
		if _, err := esper.RegisterStruct[infraTableIntoTableS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraTableIntoTableBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			optionalAggColumn("thewin", "window", "window(*)"),
			optionalAggColumn("thesort", "sorted", "sorted(intPrimitive desc)"),
		}); err != nil {
			return nil, err
		}
		s0Stream := esper.From[infraTableIntoTableS0](env, "SupportBean_S0").Window(esper.LastEvent())
		sbStream := esper.From[infraTableIntoTableBean](env, "SupportBean").Window(esper.KeepAll())
		intoPlan, err := env.Build(esper.Join(s0Stream, sbStream).Aggregate(
			esper.Alias("thewin", esper.WindowValues[esper.Event](esper.JoinEventValue[esper.Event](1))),
			esper.Alias("thesort", esper.SortedEventsBy[esper.Event, int](
				esper.JoinEventValue[esper.Event](1), esper.JoinField[int](1, "intPrimitive"), true)),
		).IntoTable("MyTable"))
		if err != nil {
			return nil, err
		}
		fixture.plans = append(fixture.plans, intoPlan)
		if err := infraTableIntoTableSnapshot(env, fixture, "faf-table", "MyTable"); err != nil {
			return nil, err
		}

	case "no-keys", "with-keys":
		if _, err := esper.RegisterStruct[infraTableIntoTableBean](env, "SupportBean"); err != nil {
			return nil, err
		}
		if _, err := esper.RegisterStruct[infraTableIntoTableS0](env, "SupportBean_S0"); err != nil {
			return nil, err
		}
		columns := []esper.TableColumn{esper.TableColumnOf[int]("sumint")}
		tableTarget := "MyTable"
		if caseName == "with-keys" {
			columns = []esper.TableColumn{
				esper.PrimaryKeyColumn[string]("pkey"),
				esper.TableColumnOf[int]("sumint"),
			}
		}
		if _, err := esper.CreateTable(env, "MyTable", columns); err != nil {
			return nil, err
		}
		intoBuilder := func() (esper.Plan, error) {
			stream := esper.From[infraTableIntoTableBean](env, "SupportBean")
			if caseName == "with-keys" {
				theString := esper.Field[infraTableIntoTableBean, string]("theString")
				return env.Build(stream.GroupBy(theString).Select(
					esper.Alias("pkey", theString),
					esper.Alias("sumint", esper.Sum[int](esper.Field[infraTableIntoTableBean, int]("intPrimitive"))),
				).IntoTable(tableTarget))
			}
			return env.Build(stream.Aggregate(
				esper.Alias("sumint", esper.Sum[int](esper.Field[infraTableIntoTableBean, int]("intPrimitive"))),
			).IntoTable(tableTarget))
		}
		intoPlan, err := intoBuilder()
		if err != nil {
			return nil, err
		}
		consumerPlan, consumerErr := func() (esper.Plan, error) {
			s0 := esper.From[infraTableIntoTableS0](env, "SupportBean_S0")
			if caseName == "with-keys" {
				return env.Build(esper.Select(s0, esper.Alias("c0",
					esper.SubqueryValueWithOptions[int](
						esper.FromTable(env, "MyTable"),
						esper.Field[any, int]("sumint"),
						esper.SubqueryWhere(esper.Equal[string](
							esper.Field[any, string]("pkey"), esper.OuterField[string]("p00"))),
					))).Query(esper.StatementName("s0")))
			}
			return env.Build(esper.Select(s0, esper.Alias("c0",
				esper.SubqueryValue[int](
					esper.FromTable(env, "MyTable"),
					esper.Field[any, int]("sumint"),
				))).Query(esper.StatementName("s0")))
		}()
		if consumerErr != nil {
			return nil, consumerErr
		}
		fixture.plans = append(fixture.plans, intoPlan, consumerPlan)
		fixture.listenerNames = append(fixture.listenerNames, "s0")
		if err := infraTableIntoTableSnapshot(env, fixture, "create-state", "MyTable"); err != nil {
			return nil, err
		}

	case "big-number-avg-sum":
		if _, err := esper.RegisterStruct[infraTableIntoTableNumeric](env, "SupportBeanNumeric"); err != nil {
			return nil, err
		}
		if _, err := esper.CreateTable(env, "MyTable", []esper.TableColumn{
			esper.OptionalTableColumnOf[big.Rat]("c0"),
			esper.OptionalTableColumnOf[big.Rat]("c1"),
			esper.OptionalTableColumnOf[big.Int]("c2"),
			esper.OptionalTableColumnOf[big.Rat]("c3"),
		}); err != nil {
			return nil, err
		}
		bigint := esper.Field[infraTableIntoTableNumeric, big.Int]("bigint")
		bigdec := esper.Field[infraTableIntoTableNumeric, big.Rat]("bigdec")
		intoPlan, err := env.Build(esper.From[infraTableIntoTableNumeric](env, "SupportBeanNumeric").Window(esper.LastEvent()).Aggregate(
			esper.Alias("c0", esper.AvgExact[big.Int](bigint)),
			esper.Alias("c1", esper.AvgExact[big.Rat](bigdec)),
			esper.Alias("c2", esper.SumExact[big.Int](bigint)),
			esper.Alias("c3", esper.SumExact[big.Rat](bigdec)),
		).IntoTable("MyTable"))
		if err != nil {
			return nil, err
		}
		fixture.plans = append(fixture.plans, intoPlan)
		if err := infraTableIntoTableSnapshot(env, fixture, "tbl", "MyTable"); err != nil {
			return nil, err
		}

	case "multikey-warray-single", "multikey-warray-two":
		if _, err := esper.RegisterStruct[infraTableIntoTableManyArray](env, "SupportEventWithManyArray"); err != nil {
			return nil, err
		}
		twoKey := caseName == "multikey-warray-two"
		columns := []esper.TableColumn{}
		keys := []esper.Expr{}
		selections := []esper.Selection{}
		if twoKey {
			columns = append(columns,
				esper.PrimaryKeyColumn[[]int]("k1"),
				esper.PrimaryKeyColumn[[]int]("k2"),
				esper.TableColumnOf[int]("thesum"),
			)
			intOne := esper.Field[infraTableIntoTableManyArray, []int]("intOne")
			intTwo := esper.Field[infraTableIntoTableManyArray, []int]("intTwo")
			keys = append(keys, intOne, intTwo)
			selections = append(selections,
				esper.Alias("k1", intOne),
				esper.Alias("k2", intTwo),
			)
		} else {
			columns = append(columns,
				esper.PrimaryKeyColumn[[]int]("k"),
				esper.TableColumnOf[int]("thesum"),
			)
			intOne := esper.Field[infraTableIntoTableManyArray, []int]("intOne")
			keys = append(keys, intOne)
			selections = append(selections, esper.Alias("k", intOne))
		}
		if _, err := esper.CreateTable(env, "MyTable", columns); err != nil {
			return nil, err
		}
		selections = append(selections, esper.Alias("thesum",
			esper.Sum[int](esper.Field[infraTableIntoTableManyArray, int]("value"))))
		stream := esper.From[infraTableIntoTableManyArray](env, "SupportEventWithManyArray")
		intoPlan, err := env.Build(stream.GroupBy(keys...).Select(selections...).IntoTable("MyTable"))
		if err != nil {
			return nil, err
		}
		fixture.plans = append(fixture.plans, intoPlan)
		if err := infraTableIntoTableSnapshot(env, fixture, "tbl", "MyTable"); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported infra table into-table case %q", caseName)
	}
	return fixture, nil
}
