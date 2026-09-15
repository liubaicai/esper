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
	orderbyRowPerEventAggID          = "orderby-rowperevent-agg"
	orderbyRowPerEventAggJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	orderbyRowPerEventAggSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerEvent.java"
	orderbyRowPerEventAggDescription = "ResultSetOrderByRowPerEvent ordinals 3 and 5: ungrouped row-per-event aggregates ordered by an order-by key that mixes the event's volume with sum(price), and the nested max(sum(price)) historical-prefix aggregate, both delivered once per six events over a length(10) window."

	orderbyRowPerEventAggOrderFunctionRuntimeID = "java-runtime-e5b38082f9b30ce8a13b"
	orderbyRowPerEventAggMaxSumRuntimeID        = "java-runtime-1be3cb0efcdb94f20d5f"
	orderbyRowPerEventAggOrderFunctionStaticID  = "java-f583dbb6e793f0fae373"
	orderbyRowPerEventAggMaxSumStaticID         = "java-55bd91f3bd2eecde7acf"
	orderbyRowPerEventAggOrderFunctionCase      = "order-function"
	orderbyRowPerEventAggMaxSumCase             = "max-sum"
	orderbyRowPerEventAggMarketType             = "SupportMarketDataBean"

	// The pinned EPL texts are the Java source statements verbatim, including
	// the single spaces the Java line continuations leave between clauses. The
	// unaliased aggregate columns keep Java's engine-generated names.
	orderbyRowPerEventAggOrderFunctionEPL = "@name('s0') select symbol, sum(price) from SupportMarketDataBean#length(10) output every 6 events order by volume*sum(price), symbol"
	orderbyRowPerEventAggMaxSumEPL        = "@name('s0') select symbol, max(sum(price)) from SupportMarketDataBean#length(10) output every 6 events order by symbol"
)

// orderbyRowPerEventAggCaseSpec pins one replayed execution. `output every 6
// events` fires exactly one listener callback per case, on the sixth event.
type orderbyRowPerEventAggCaseSpec struct {
	name      string
	ordinal   int
	runtimeID string
	execution string
	epl       string
	records   int
}

var orderbyRowPerEventAggCaseSpecs = []orderbyRowPerEventAggCaseSpec{
	{
		name:      orderbyRowPerEventAggOrderFunctionCase,
		ordinal:   3,
		runtimeID: orderbyRowPerEventAggOrderFunctionRuntimeID,
		execution: "ResultSetRowPerEventOrderFunction",
		epl:       orderbyRowPerEventAggOrderFunctionEPL,
		records:   1,
	},
	{
		name:      orderbyRowPerEventAggMaxSumCase,
		ordinal:   5,
		runtimeID: orderbyRowPerEventAggMaxSumRuntimeID,
		execution: "ResultSetRowPerEventMaxSum",
		epl:       orderbyRowPerEventAggMaxSumEPL,
		records:   1,
	},
}

var (
	orderbyRowPerEventAggRuntimes   = []string{orderbyRowPerEventAggOrderFunctionRuntimeID, orderbyRowPerEventAggMaxSumRuntimeID}
	orderbyRowPerEventAggExecutions = []string{"ResultSetRowPerEventOrderFunction", "ResultSetRowPerEventMaxSum"}
	orderbyRowPerEventAggSources    = []string{orderbyRowPerEventAggSource}
	orderbyRowPerEventAggStaticIDs  = []string{orderbyRowPerEventAggOrderFunctionStaticID, orderbyRowPerEventAggMaxSumStaticID}
)

func orderbyRowPerEventAggJavaRuntimeIDs() []string {
	return append([]string(nil), orderbyRowPerEventAggRuntimes...)
}

func orderbyRowPerEventAggJavaExecutions() []string {
	return append([]string(nil), orderbyRowPerEventAggExecutions...)
}

// orderbyRowPerEventAggMarket mirrors the SupportMarketDataBean events this
// execution sends: `new SupportMarketDataBean(symbol, price, 0L, null)` with a
// NON-NULL zero volume, which is load-bearing for ordinal 3's order key
// `volume*sum(price)` (0.0 for every row, so `symbol` decides).
type orderbyRowPerEventAggMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
}

// orderbyRowPerEventAggStepSpec pins every scenario step in order; the loader
// rejects any drift before the runtime is touched.
type orderbyRowPerEventAggStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   string
}

func orderbyRowPerEventAggSend(symbol string, price float64) string {
	return fmt.Sprintf(`{"symbol":"%s","price":%v,"volume":0}`, symbol, price)
}

var orderbyRowPerEventAggStepSpecs = func() []orderbyRowPerEventAggStepSpec {
	rounds := []struct {
		caseName string
		sends    [][2]any
	}{
		{orderbyRowPerEventAggOrderFunctionCase, [][2]any{{"IBM", 2.0}, {"KGB", 1.0}, {"CMU", 3.0}, {"IBM", 6.0}, {"CAT", 6.0}, {"CAT", 5.0}}},
		{orderbyRowPerEventAggMaxSumCase, [][2]any{{"IBM", 3.0}, {"IBM", 4.0}, {"CMU", 1.0}, {"CMU", 2.0}, {"CAT", 5.0}, {"CAT", 6.0}}},
	}
	specs := make([]orderbyRowPerEventAggStepSpec, 0, 14)
	for _, round := range rounds {
		specs = append(specs, orderbyRowPerEventAggStepSpec{op: "case", caseName: round.caseName})
		for _, send := range round.sends {
			symbol, _ := send[0].(string)
			price, _ := send[1].(float64)
			specs = append(specs, orderbyRowPerEventAggStepSpec{
				op: "send", caseName: round.caseName, eventType: orderbyRowPerEventAggMarketType,
				payload: orderbyRowPerEventAggSend(symbol, price),
			})
		}
	}
	return specs
}()

func loadOrderByRowPerEventAggScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", orderbyRowPerEventAggID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", orderbyRowPerEventAggID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", orderbyRowPerEventAggID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", orderbyRowPerEventAggID, err)
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
	if version != compat.ScenarioVersion || id != orderbyRowPerEventAggID ||
		description != orderbyRowPerEventAggDescription ||
		javaCommit != orderbyRowPerEventAggJavaCommit ||
		javaSource != orderbyRowPerEventAggSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", orderbyRowPerEventAggID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], orderbyRowPerEventAggRuntimes, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], orderbyRowPerEventAggExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], orderbyRowPerEventAggStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(orderbyRowPerEventAggCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", orderbyRowPerEventAggID, len(orderbyRowPerEventAggCaseSpecs))
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
		spec := orderbyRowPerEventAggCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || caseMeta.Ordinal != spec.ordinal || caseMeta.RuntimeID != spec.runtimeID ||
			caseMeta.ExecutionName != spec.execution || caseMeta.Observation != "listener" ||
			caseMeta.IteratorSnapshots != 0 || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", orderbyRowPerEventAggID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(orderbyRowPerEventAggStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", orderbyRowPerEventAggID, len(orderbyRowPerEventAggStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := orderbyRowPerEventAggStepSpecs[index]
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
	if err := validateOrderByRowPerEventAggScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateOrderByRowPerEventAggScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != orderbyRowPerEventAggID ||
		len(scenario.Steps) != len(orderbyRowPerEventAggStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", orderbyRowPerEventAggID)
	}
	for index, spec := range orderbyRowPerEventAggStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", orderbyRowPerEventAggID, index, spec.op)
		}
		if spec.op == "case" {
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", orderbyRowPerEventAggID, index, spec.caseName)
			}
			continue
		}
		if step.EventType != spec.eventType || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", orderbyRowPerEventAggID, index, spec.eventType)
		}
		compact, err := resultSetQueryTypeLocalGroupKeysCompactJSON(step.Payload)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", orderbyRowPerEventAggID, index, err)
		}
		if compact != spec.payload {
			return fmt.Errorf("%s scenario step %d %s payload is not pinned", orderbyRowPerEventAggID, index, spec.eventType)
		}
	}
	return nil
}

func runOrderByRowPerEventAggScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateOrderByRowPerEventAggScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: orderbyRowPerEventAggID}
	for _, spec := range orderbyRowPerEventAggCaseSpecs {
		caseScenario, err := scenarioForCase(scenario, spec.name)
		if err != nil {
			return compat.Trace{}, err
		}
		records, err := runOrderByRowPerEventAggCase(ctx, spec, caseScenario)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", orderbyRowPerEventAggID, spec.name, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if err := validateOrderByRowPerEventAggTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runOrderByRowPerEventAggCase(ctx context.Context, spec orderbyRowPerEventAggCaseSpec, caseScenario compat.Scenario) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[orderbyRowPerEventAggMarket](env, orderbyRowPerEventAggMarketType); err != nil {
		return nil, err
	}
	symbol := esper.Field[orderbyRowPerEventAggMarket, string]("symbol")
	volume := esper.Field[orderbyRowPerEventAggMarket, int64]("volume")
	price := esper.Field[orderbyRowPerEventAggMarket, float64]("price")
	stream := esper.From[orderbyRowPerEventAggMarket](env, orderbyRowPerEventAggMarketType).
		Window(esper.LengthWindow(10))
	options := []esper.QueryOption{
		esper.StatementName("s0"),
		esper.WithOutput(esper.OutputEvery(6)),
	}
	var query esper.Query
	switch spec.name {
	case orderbyRowPerEventAggOrderFunctionCase:
		// Java's order key `volume*sum(price)` mixes the event's own volume with
		// the row-per-event running sum; volume is 0L for every event here, so
		// the key collapses to 0.0 and `symbol` decides. The projection keeps
		// Java's unaliased engine-generated column name.
		query = stream.Aggregate(
			esper.Alias("symbol", symbol),
			esper.Alias("sum(price)", esper.Sum[float64](price)),
		).Query(append(options, esper.OrderBy(
			esper.Ascending(esper.Multiply[float64](esper.Cast[int64, float64](volume), esper.Sum[float64](price))),
			esper.Ascending(symbol),
		))...)
	case orderbyRowPerEventAggMaxSumCase:
		// Java's nested max(sum(price)) evaluates the inner sum once per
		// historical group prefix and keeps the extreme, so each delivered row
		// carries the running maximum as of its own event.
		query = stream.Aggregate(
			esper.Alias("symbol", symbol),
			esper.Alias("max(sum(price))", esper.Max[float64](esper.Sum[float64](price))),
		).Query(append(options, esper.OrderBy(esper.Ascending(symbol)))...)
	default:
		return nil, fmt.Errorf("unknown %s case %q", orderbyRowPerEventAggID, spec.name)
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
			if err := requireResultSetQueryTypeLocalGroupByFields(fields, "symbol", "price", "volume"); err != nil {
				return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventAggMarketType, err)
			}
			var event orderbyRowPerEventAggMarket
			if err := json.Unmarshal(step.Payload, &event); err != nil {
				return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventAggMarketType, err)
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", orderbyRowPerEventAggID, step.Op)
		}
	}
	// Java delivers exactly one non-empty callback per case (the sixth event)
	// and never an empty one, so a callback-count drift is a parity failure.
	if emptyCallbacks != 0 {
		return nil, fmt.Errorf("%s case %q delivered %d empty listener callbacks", orderbyRowPerEventAggID, spec.name, emptyCallbacks)
	}
	if invocations != spec.records || len(records) != spec.records {
		return nil, fmt.Errorf("%s case %q delivered %d listener callbacks (%d records), want %d",
			orderbyRowPerEventAggID, spec.name, invocations, len(records), spec.records)
	}
	return records, nil
}

func validateOrderByRowPerEventAggTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != orderbyRowPerEventAggID {
		return fmt.Errorf("%s trace identity is not pinned", orderbyRowPerEventAggID)
	}
	if len(trace.Records) != len(orderbyRowPerEventAggCaseSpecs) {
		return fmt.Errorf("%s trace must contain exactly %d records", orderbyRowPerEventAggID, len(orderbyRowPerEventAggCaseSpecs))
	}
	for index, spec := range orderbyRowPerEventAggCaseSpecs {
		record := trace.Records[index]
		if record.Case != spec.name || record.Operation != "listener" || record.Statement != "s0" ||
			record.Sequence != 1 || record.Time != "1970-01-01T00:00:00Z" {
			return fmt.Errorf("%s trace record %d is not pinned", orderbyRowPerEventAggID, index)
		}
		if len(record.Old) != 0 || len(record.New) != 6 {
			return fmt.Errorf("%s trace record %d must carry six new rows and no old rows", orderbyRowPerEventAggID, index)
		}
	}
	return nil
}
