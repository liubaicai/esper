package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	orderbyRowPerEventAggJoinID          = "orderby-rowperevent-agg-join"
	orderbyRowPerEventAggJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	orderbyRowPerEventAggJoinSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerEvent.java"
	orderbyRowPerEventAggJoinDescription = "ResultSetOrderByRowPerEvent ordinals 2, 9 and 10: ungrouped row-per-event aggregates over a SupportMarketDataBean#length(10) x SupportBeanString#length(100) join, delivered once per six result rows - the order-function variant, the nested max(sum(price)) variant and the having variant with the plain running sum."

	orderbyRowPerEventAggJoinOrderFunctionRuntimeID = "java-runtime-3864ed6701fd9371d2d6"
	orderbyRowPerEventAggJoinMaxRuntimeID           = "java-runtime-eb2d5eb23ce35ef9d935"
	orderbyRowPerEventAggJoinHavingRuntimeID        = "java-runtime-73a76f426926d18792f2"
	orderbyRowPerEventAggJoinOrderFunctionStaticID  = "java-bb2fc1b5fb51cb943021"
	orderbyRowPerEventAggJoinMaxStaticID            = "java-7c35d3dcefb677a20097"
	orderbyRowPerEventAggJoinHavingStaticID         = "java-9c135512047209da35dd"
	orderbyRowPerEventAggJoinOrderFunctionCase      = "join-order-function"
	orderbyRowPerEventAggJoinMaxCase                = "join-max"
	orderbyRowPerEventAggJoinHavingCase             = "join-having"
	orderbyRowPerEventAggJoinMDBType                = "SupportMarketDataBean"
	orderbyRowPerEventAggJoinSBSType                = "SupportBeanString"

	// The pinned EPL texts are the Java source statements verbatim, including
	// the single spaces the Java line continuations leave between clauses. The
	// unaliased aggregate columns keep Java's engine-generated names.
	orderbyRowPerEventAggJoinOrderFunctionEPL = "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by volume*sum(price), symbol"
	orderbyRowPerEventAggJoinMaxEPL           = "@name('s0') select symbol, max(sum(price)) from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString output every 6 events order by symbol"
	orderbyRowPerEventAggJoinHavingEPL        = "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString having sum(price) > 0 output every 6 events order by symbol"
)

// orderbyRowPerEventAggJoinCaseSpec pins one replayed execution. `output every
// 6 events` counts RESULT rows (join pairs), so the single delivery arrives at
// the SBS send that creates the sixth pair.
type orderbyRowPerEventAggJoinCaseSpec struct {
	name      string
	ordinal   int
	runtimeID string
	execution string
	epl       string
	records   int
}

var orderbyRowPerEventAggJoinCaseSpecs = []orderbyRowPerEventAggJoinCaseSpec{
	{
		name:      orderbyRowPerEventAggJoinOrderFunctionCase,
		ordinal:   2,
		runtimeID: orderbyRowPerEventAggJoinOrderFunctionRuntimeID,
		execution: "ResultSetRowPerEventJoinOrderFunction",
		epl:       orderbyRowPerEventAggJoinOrderFunctionEPL,
		records:   1,
	},
	{
		name:      orderbyRowPerEventAggJoinMaxCase,
		ordinal:   9,
		runtimeID: orderbyRowPerEventAggJoinMaxRuntimeID,
		execution: "ResultSetRowPerEventJoinMax",
		epl:       orderbyRowPerEventAggJoinMaxEPL,
		records:   1,
	},
	{
		name:      orderbyRowPerEventAggJoinHavingCase,
		ordinal:   10,
		runtimeID: orderbyRowPerEventAggJoinHavingRuntimeID,
		execution: "ResultSetAggHaving",
		epl:       orderbyRowPerEventAggJoinHavingEPL,
		records:   1,
	},
}

var (
	orderbyRowPerEventAggJoinRuntimes   = []string{orderbyRowPerEventAggJoinOrderFunctionRuntimeID, orderbyRowPerEventAggJoinMaxRuntimeID, orderbyRowPerEventAggJoinHavingRuntimeID}
	orderbyRowPerEventAggJoinExecutions = []string{"ResultSetRowPerEventJoinOrderFunction", "ResultSetRowPerEventJoinMax", "ResultSetAggHaving"}
	orderbyRowPerEventAggJoinSources    = []string{orderbyRowPerEventAggJoinSource}
	orderbyRowPerEventAggJoinStaticIDs  = []string{orderbyRowPerEventAggJoinOrderFunctionStaticID, orderbyRowPerEventAggJoinMaxStaticID, orderbyRowPerEventAggJoinHavingStaticID}
)

func orderbyRowPerEventAggJoinJavaRuntimeIDs() []string {
	return append([]string(nil), orderbyRowPerEventAggJoinRuntimes...)
}

func orderbyRowPerEventAggJoinJavaExecutions() []string {
	return append([]string(nil), orderbyRowPerEventAggJoinExecutions...)
}

// orderbyRowPerEventAggJoinStepSpec pins every scenario step in order; the
// loader rejects any drift before the runtime is touched.
type orderbyRowPerEventAggJoinStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   string
}

func orderbyRowPerEventAggJoinSendMDB(symbol string, price float64) string {
	return fmt.Sprintf(`{"symbol":"%s","price":%v,"volume":0}`, symbol, price)
}

func orderbyRowPerEventAggJoinSendSBS(v string) string {
	return fmt.Sprintf(`{"theString":"%s"}`, v)
}

var orderbyRowPerEventAggJoinStepSpecs = func() []orderbyRowPerEventAggJoinStepSpec {
	mdbRounds := []struct {
		caseName string
		sends    [][2]any
	}{
		{orderbyRowPerEventAggJoinOrderFunctionCase, [][2]any{{"IBM", 2.0}, {"KGB", 1.0}, {"CMU", 3.0}, {"IBM", 6.0}, {"CAT", 6.0}, {"CAT", 5.0}}},
		{orderbyRowPerEventAggJoinMaxCase, [][2]any{{"IBM", 3.0}, {"IBM", 4.0}, {"CMU", 1.0}, {"CMU", 2.0}, {"CAT", 5.0}, {"CAT", 6.0}}},
		{orderbyRowPerEventAggJoinHavingCase, [][2]any{{"IBM", 3.0}, {"IBM", 4.0}, {"CMU", 1.0}, {"CMU", 2.0}, {"CAT", 5.0}, {"CAT", 6.0}}},
	}
	sbsRounds := map[string][]string{
		orderbyRowPerEventAggJoinOrderFunctionCase: {"CAT", "IBM", "CMU", "KGB", "DOG"},
		orderbyRowPerEventAggJoinMaxCase:           {"CAT", "IBM", "CMU"},
		orderbyRowPerEventAggJoinHavingCase:        {"CAT", "IBM", "CMU"},
	}
	specs := make([]orderbyRowPerEventAggJoinStepSpec, 0, 32)
	for _, round := range mdbRounds {
		specs = append(specs, orderbyRowPerEventAggJoinStepSpec{op: "case", caseName: round.caseName})
		for _, send := range round.sends {
			symbol, _ := send[0].(string)
			price, _ := send[1].(float64)
			specs = append(specs, orderbyRowPerEventAggJoinStepSpec{
				op: "send", caseName: round.caseName, eventType: orderbyRowPerEventAggJoinMDBType,
				payload: orderbyRowPerEventAggJoinSendMDB(symbol, price),
			})
		}
		for _, v := range sbsRounds[round.caseName] {
			specs = append(specs, orderbyRowPerEventAggJoinStepSpec{
				op: "send", caseName: round.caseName, eventType: orderbyRowPerEventAggJoinSBSType,
				payload: orderbyRowPerEventAggJoinSendSBS(v),
			})
		}
	}
	return specs
}()

func loadOrderByRowPerEventAggJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", orderbyRowPerEventAggJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", orderbyRowPerEventAggJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", orderbyRowPerEventAggJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", orderbyRowPerEventAggJoinID, err)
	}
	if err := requireResultSetQueryTypeLocalGroupByFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}

	version, err := decodeResultSetQueryTypeLocalGroupByString(root["version"], "version")
	if err != nil {
		return compat.Scenario{}, err
	}
	id, err := decodeResultSetQueryTypeLocalGroupByString(root["id"], "id")
	if err != nil {
		return compat.Scenario{}, err
	}
	description, err := decodeResultSetQueryTypeLocalGroupByString(root["description"], "description")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaCommit, err := decodeResultSetQueryTypeLocalGroupByString(root["javaCommit"], "javaCommit")
	if err != nil {
		return compat.Scenario{}, err
	}
	javaSource, err := decodeResultSetQueryTypeLocalGroupByString(root["javaSource"], "javaSource")
	if err != nil {
		return compat.Scenario{}, err
	}
	if version != compat.ScenarioVersion || id != orderbyRowPerEventAggJoinID ||
		description != orderbyRowPerEventAggJoinDescription ||
		javaCommit != orderbyRowPerEventAggJoinJavaCommit ||
		javaSource != orderbyRowPerEventAggJoinSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", orderbyRowPerEventAggJoinID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], orderbyRowPerEventAggJoinRuntimes, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], orderbyRowPerEventAggJoinExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], orderbyRowPerEventAggJoinStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(orderbyRowPerEventAggJoinCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", orderbyRowPerEventAggJoinID, len(orderbyRowPerEventAggJoinCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var caseObject map[string]json.RawMessage
		if err := strictObject(rawCase, &caseObject); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(caseObject,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var caseMeta struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		spec := orderbyRowPerEventAggJoinCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || caseMeta.Ordinal != spec.ordinal || caseMeta.RuntimeID != spec.runtimeID ||
			caseMeta.ExecutionName != spec.execution || caseMeta.Observation != "listener" ||
			caseMeta.IteratorSnapshots != 0 || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", orderbyRowPerEventAggJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(orderbyRowPerEventAggJoinStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", orderbyRowPerEventAggJoinID, len(orderbyRowPerEventAggJoinStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := orderbyRowPerEventAggJoinStepSpecs[index]
		var expected []string
		if spec.op == "case" {
			expected = []string{"op", "case"}
		} else {
			expected = []string{"op", "eventType", "payload"}
		}
		if err := requireResultSetQueryTypeLocalGroupByFields(object, expected...); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: version, ID: id, Steps: steps}
	if err := validateOrderByRowPerEventAggJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateOrderByRowPerEventAggJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != orderbyRowPerEventAggJoinID ||
		len(scenario.Steps) != len(orderbyRowPerEventAggJoinStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", orderbyRowPerEventAggJoinID)
	}
	for index, spec := range orderbyRowPerEventAggJoinStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", orderbyRowPerEventAggJoinID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", orderbyRowPerEventAggJoinID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.eventType || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", orderbyRowPerEventAggJoinID, index, spec.eventType)
		}
		compact, err := resultSetQueryTypeLocalGroupKeysCompactJSON(step.Payload)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", orderbyRowPerEventAggJoinID, index, err)
		}
		if compact != spec.payload {
			return fmt.Errorf("%s scenario step %d %s payload is not pinned", orderbyRowPerEventAggJoinID, index, spec.eventType)
		}
	}
	return nil
}

func runOrderByRowPerEventAggJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateOrderByRowPerEventAggJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: orderbyRowPerEventAggJoinID}
	for _, spec := range orderbyRowPerEventAggJoinCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runOrderByRowPerEventAggJoinCase(ctx, spec, caseScenario)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", orderbyRowPerEventAggJoinID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateOrderByRowPerEventAggJoinTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runOrderByRowPerEventAggJoinCase(ctx context.Context, spec orderbyRowPerEventAggJoinCaseSpec, caseScenario compat.Scenario) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[orderbyRowPerEventAggMarket](env, orderbyRowPerEventAggJoinMDBType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerEventBeanString](env, orderbyRowPerEventAggJoinSBSType); err != nil {
		return nil, err
	}
	symbol := esper.JoinField[string](0, "symbol")
	volume := esper.JoinField[int64](0, "volume")
	price := esper.JoinField[float64](0, "price")
	stream := esper.Join(
		esper.From[orderbyRowPerEventAggMarket](env, orderbyRowPerEventAggJoinMDBType).Window(esper.LengthWindow(10)),
		esper.From[resultsetQueryTypeRowPerEventBeanString](env, orderbyRowPerEventAggJoinSBSType).Window(esper.LengthWindow(100)),
		esper.OnEqual(esper.JoinField[string](0, "symbol"), esper.JoinField[string](1, "theString")),
	)
	options := []esper.QueryOption{
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputEvery(6)),
	}
	var query esper.Query
	switch spec.name {
	case orderbyRowPerEventAggJoinOrderFunctionCase:
		// Java's order key `volume*sum(price)` mixes the pair's own volume with
		// the join-output running sum; volume is 0L for every event here, so the
		// key collapses to 0.0 and `symbol` decides. The projection keeps Java's
		// unaliased engine-generated column name.
		query = stream.Aggregate(
			esper.Alias("symbol", symbol),
			esper.Alias("sum(price)", esper.Sum[float64](price)),
		).Query(append(options, esper.OrderBy(
			esper.Ascending(esper.Multiply[float64](esper.Cast[int64, float64](volume), esper.Sum[float64](price))),
			esper.Ascending(symbol),
		))...)
	case orderbyRowPerEventAggJoinMaxCase:
		// The nested max(sum(price)) evaluates the inner sum once per
		// historical join-output prefix and keeps the extreme, so each delivered
		// row carries the running maximum as of its own pair's creation.
		query = stream.Aggregate(
			esper.Alias("symbol", symbol),
			esper.Alias("max(sum(price))", esper.Max[float64](esper.Sum[float64](price))),
		).Query(append(options, esper.OrderBy(esper.Ascending(symbol)))...)
	case orderbyRowPerEventAggJoinHavingCase:
		// The having gates the rows on the plain running sum (always true for
		// these sends), and the projection keeps Java's unaliased column name.
		query = stream.Aggregate(
			esper.Alias("symbol", symbol),
			esper.Alias("sum(price)", esper.Sum[float64](price)),
		).Having(esper.Greater[float64](esper.Sum[float64](price), esper.Literal[float64](0))).
			Query(append(options, esper.OrderBy(esper.Ascending(symbol)))...)
	default:
		return nil, fmt.Errorf("unknown %s case %q", orderbyRowPerEventAggJoinID, spec.name)
	}
	plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, spec.runtimeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	records := make([]compat.TraceRecord, 0, spec.records)
	invocations := 0
	emptyCallbacks := 0
	if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		invocations++
		newRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.New)
		oldRows := resultSetQueryTypeLocalGroupUngroupedRows(batch.Old)
		if len(newRows) == 0 && len(oldRows) == 0 {
			emptyCallbacks++
			return nil
		}
		records = append(records, compat.TraceRecord{
			Case:      spec.name,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  1,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       newRows,
			Old:       oldRows,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var fields map[string]json.RawMessage
			if err := strictObject(step.Payload, &fields); err != nil {
				return nil, err
			}
			switch step.EventType {
			case orderbyRowPerEventAggJoinMDBType:
				if err := requireResultSetQueryTypeLocalGroupByFields(fields, "symbol", "price", "volume"); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventAggJoinMDBType, err)
				}
				var event orderbyRowPerEventAggMarket
				if err := json.Unmarshal(step.Payload, &event); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventAggJoinMDBType, err)
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return nil, err
				}
			case orderbyRowPerEventAggJoinSBSType:
				if err := requireResultSetQueryTypeLocalGroupByFields(fields, "theString"); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventAggJoinSBSType, err)
				}
				var event resultsetQueryTypeRowPerEventBeanString
				if err := json.Unmarshal(step.Payload, &event); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventAggJoinSBSType, err)
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unknown %s event type %q", orderbyRowPerEventAggJoinID, step.EventType)
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", orderbyRowPerEventAggJoinID, step.Op)
		}
	}
	// Java delivers exactly one non-empty callback per case (the SBS send that
	// creates the sixth result row) and never an empty one.
	if emptyCallbacks != 0 {
		return nil, fmt.Errorf("%s case %q delivered %d empty listener callbacks", orderbyRowPerEventAggJoinID, spec.name, emptyCallbacks)
	}
	if invocations != spec.records || len(records) != spec.records {
		return nil, fmt.Errorf("%s case %q delivered %d listener callbacks (%d records), want %d",
			orderbyRowPerEventAggJoinID, spec.name, invocations, len(records), spec.records)
	}
	return records, nil
}

func validateOrderByRowPerEventAggJoinTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != orderbyRowPerEventAggJoinID {
		return fmt.Errorf("%s trace identity is not pinned", orderbyRowPerEventAggJoinID)
	}
	if len(trace.Records) != len(orderbyRowPerEventAggJoinCaseSpecs) {
		return fmt.Errorf("%s trace must contain exactly %d records", orderbyRowPerEventAggJoinID, len(orderbyRowPerEventAggJoinCaseSpecs))
	}
	for index, spec := range orderbyRowPerEventAggJoinCaseSpecs {
		record := trace.Records[index]
		if record.Case != spec.name || record.Operation != "listener" || record.Statement != "s0" ||
			record.Sequence != 1 || record.Time != "1970-01-01T00:00:00Z" {
			return fmt.Errorf("%s trace record %d is not pinned", orderbyRowPerEventAggJoinID, index)
		}
		if len(record.Old) != 0 || len(record.New) != 6 {
			return fmt.Errorf("%s trace record %d must carry six new rows and no old rows", orderbyRowPerEventAggJoinID, index)
		}
	}
	return nil
}
