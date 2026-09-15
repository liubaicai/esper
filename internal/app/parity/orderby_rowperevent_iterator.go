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
	orderbyRowPerEventIteratorID          = "orderby-rowperevent-iterator"
	orderbyRowPerEventIteratorJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	orderbyRowPerEventIteratorSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/orderby/ResultSetOrderByRowPerEvent.java"
	orderbyRowPerEventIteratorDescription = "ResultSetOrderByRowPerEvent ordinal 0: the join aggregate read through the statement iterator twice inside one runtime, each row carrying the current window sum, ordered by symbol."

	orderbyRowPerEventIteratorRuntimeID  = "java-runtime-7bef2fa8f755e74b24ac"
	orderbyRowPerEventIteratorStaticID   = "java-d88b4c5e379242ff177b"
	orderbyRowPerEventIteratorCase       = "iterator-join"
	orderbyRowPerEventIteratorExecution  = "ResultSetIteratorAggregateRowPerEvent"
	orderbyRowPerEventIteratorMarketType = "SupportMarketDataBean"
	orderbyRowPerEventIteratorSBSType    = "SupportBeanString"
	orderbyRowPerEventIteratorSnapshots  = 2
	orderbyRowPerEventIteratorFirstRows  = 4
	orderbyRowPerEventIteratorSecondRows = 5

	// The pinned EPL text is the Java source statement verbatim, including the
	// single spaces the Java line continuations leave between clauses. There is
	// no output policy: the execution reads the statement iterator.
	orderbyRowPerEventIteratorEPL = "@name('s0') select symbol, sum(price) as sumPrice from SupportMarketDataBean#length(10) as one, SupportBeanString#length(100) as two where one.symbol = two.theString order by symbol"
)

// orderbyRowPerEventIteratorCaseSpec pins the single replayed execution: two
// iterator reads, 4 rows then 5 rows.
type orderbyRowPerEventIteratorCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	epl         string
	snapshots   int
	snapshotLen []int
}

var orderbyRowPerEventIteratorCaseSpecs = []orderbyRowPerEventIteratorCaseSpec{
	{
		name:        orderbyRowPerEventIteratorCase,
		ordinal:     0,
		runtimeID:   orderbyRowPerEventIteratorRuntimeID,
		execution:   orderbyRowPerEventIteratorExecution,
		epl:         orderbyRowPerEventIteratorEPL,
		snapshots:   orderbyRowPerEventIteratorSnapshots,
		snapshotLen: []int{orderbyRowPerEventIteratorFirstRows, orderbyRowPerEventIteratorSecondRows},
	},
}

var (
	orderbyRowPerEventIteratorRuntimes   = []string{orderbyRowPerEventIteratorRuntimeID}
	orderbyRowPerEventIteratorExecutions = []string{orderbyRowPerEventIteratorExecution}
	orderbyRowPerEventIteratorSources    = []string{orderbyRowPerEventIteratorSource}
	orderbyRowPerEventIteratorStaticIDs  = []string{orderbyRowPerEventIteratorStaticID}
)

func orderbyRowPerEventIteratorJavaRuntimeIDs() []string {
	return append([]string(nil), orderbyRowPerEventIteratorRuntimes...)
}

func orderbyRowPerEventIteratorJavaExecutions() []string {
	return append([]string(nil), orderbyRowPerEventIteratorExecutions...)
}

// orderbyRowPerEventIteratorStepSpec pins every scenario step in order; the
// loader rejects any drift before the runtime is touched. The snapshot steps
// carry no mode: Java's assertPropsPerRowIterator is index-by-index EXACT
// order, so the rows are compared as-is.
type orderbyRowPerEventIteratorStepSpec struct {
	op        string
	caseName  string
	eventType string
	payload   string
}

func orderbyRowPerEventIteratorSendMDB(symbol string, price float64) string {
	return fmt.Sprintf(`{"symbol":"%s","price":%v,"volume":0}`, symbol, price)
}

func orderbyRowPerEventIteratorSendSBS(v string) string {
	return fmt.Sprintf(`{"theString":"%s"}`, v)
}

var orderbyRowPerEventIteratorStepSpecs = func() []orderbyRowPerEventIteratorStepSpec {
	specs := make([]orderbyRowPerEventIteratorStepSpec, 0, 11)
	specs = append(specs, orderbyRowPerEventIteratorStepSpec{op: "case", caseName: orderbyRowPerEventIteratorCase})
	for _, v := range []string{"CAT", "IBM", "KGB"} {
		specs = append(specs, orderbyRowPerEventIteratorStepSpec{
			op: "send", caseName: orderbyRowPerEventIteratorCase, eventType: orderbyRowPerEventIteratorSBSType,
			payload: orderbyRowPerEventIteratorSendSBS(v),
		})
	}
	for _, send := range [][2]any{{"CAT", 50.0}, {"IBM", 49.0}, {"CAT", 15.0}, {"IBM", 100.0}} {
		symbol, _ := send[0].(string)
		price, _ := send[1].(float64)
		specs = append(specs, orderbyRowPerEventIteratorStepSpec{
			op: "send", caseName: orderbyRowPerEventIteratorCase, eventType: orderbyRowPerEventIteratorMarketType,
			payload: orderbyRowPerEventIteratorSendMDB(symbol, price),
		})
	}
	specs = append(specs, orderbyRowPerEventIteratorStepSpec{op: "snapshot", caseName: orderbyRowPerEventIteratorCase})
	specs = append(specs, orderbyRowPerEventIteratorStepSpec{
		op: "send", caseName: orderbyRowPerEventIteratorCase, eventType: orderbyRowPerEventIteratorMarketType,
		payload: orderbyRowPerEventIteratorSendMDB("KGB", 75.0),
	})
	specs = append(specs, orderbyRowPerEventIteratorStepSpec{op: "snapshot", caseName: orderbyRowPerEventIteratorCase})
	return specs
}()

func loadOrderByRowPerEventIteratorScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", orderbyRowPerEventIteratorID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", orderbyRowPerEventIteratorID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", orderbyRowPerEventIteratorID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", orderbyRowPerEventIteratorID, err)
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
	if version != compat.ScenarioVersion || id != orderbyRowPerEventIteratorID ||
		description != orderbyRowPerEventIteratorDescription ||
		javaCommit != orderbyRowPerEventIteratorJavaCommit ||
		javaSource != orderbyRowPerEventIteratorSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", orderbyRowPerEventIteratorID)
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaRuntimes"], orderbyRowPerEventIteratorRuntimes, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaNames"], orderbyRowPerEventIteratorExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaStaticIds"], orderbyRowPerEventIteratorStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultSetQueryTypeLocalGroupByStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(orderbyRowPerEventIteratorCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d case", orderbyRowPerEventIteratorID, len(orderbyRowPerEventIteratorCaseSpecs))
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
		spec := orderbyRowPerEventIteratorCaseSpecs[index]
		if err := json.Unmarshal(rawCase, &caseMeta); err != nil ||
			caseMeta.Case != spec.name || caseMeta.Ordinal != spec.ordinal || caseMeta.RuntimeID != spec.runtimeID ||
			caseMeta.ExecutionName != spec.execution || caseMeta.Observation != "iterator" ||
			caseMeta.IteratorSnapshots != spec.snapshots || caseMeta.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", orderbyRowPerEventIteratorID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil || len(rawSteps) != len(orderbyRowPerEventIteratorStepSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d steps", orderbyRowPerEventIteratorID, len(orderbyRowPerEventIteratorStepSpecs))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		spec := orderbyRowPerEventIteratorStepSpecs[index]
		var expected []string
		switch spec.op {
		case "case":
			expected = []string{"op", "case"}
		case "snapshot":
			expected = []string{"op", "statement"}
		default:
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
	if err := validateOrderByRowPerEventIteratorScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateOrderByRowPerEventIteratorScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != orderbyRowPerEventIteratorID ||
		len(scenario.Steps) != len(orderbyRowPerEventIteratorStepSpecs) {
		return fmt.Errorf("%s scenario steps are not pinned", orderbyRowPerEventIteratorID)
	}
	for index, spec := range orderbyRowPerEventIteratorStepSpecs {
		step := scenario.Steps[index]
		if step.Op != spec.op {
			return fmt.Errorf("%s scenario step %d must be %q", orderbyRowPerEventIteratorID, index, spec.op)
		}
		switch spec.op {
		case "case":
			if step.Case != spec.caseName {
				return fmt.Errorf("%s scenario step %d must open case %q", orderbyRowPerEventIteratorID, index, spec.caseName)
			}
			continue
		case "snapshot":
			if step.Statement != "s0" || step.Case != "" || step.Mode != "" {
				return fmt.Errorf("%s scenario step %d must be a strict s0 snapshot (no mode)", orderbyRowPerEventIteratorID, index)
			}
			continue
		}
		if step.EventType != spec.eventType || step.Case != "" {
			return fmt.Errorf("%s scenario step %d must be an unlabelled %s send", orderbyRowPerEventIteratorID, index, spec.eventType)
		}
		compact, err := resultSetQueryTypeLocalGroupKeysCompactJSON(step.Payload)
		if err != nil {
			return fmt.Errorf("%s scenario step %d: %w", orderbyRowPerEventIteratorID, index, err)
		}
		if compact != spec.payload {
			return fmt.Errorf("%s scenario step %d %s payload is not pinned", orderbyRowPerEventIteratorID, index, spec.eventType)
		}
	}
	return nil
}

func runOrderByRowPerEventIteratorScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateOrderByRowPerEventIteratorScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: orderbyRowPerEventIteratorID}
	spec := orderbyRowPerEventIteratorCaseSpecs[0]
	caseScenario, err := scenarioForCase(scenario, spec.name)
	if err != nil {
		return compat.Trace{}, err
	}
	records, err := runOrderByRowPerEventIteratorCase(ctx, spec, caseScenario)
	if err != nil {
		return compat.Trace{}, fmt.Errorf("%s case %q: %w", orderbyRowPerEventIteratorID, spec.name, err)
	}
	trace.Records = append(trace.Records, records...)
	if err := validateOrderByRowPerEventIteratorTrace(trace); err != nil {
		return compat.Trace{}, err
	}
	return trace, nil
}

func runOrderByRowPerEventIteratorCase(ctx context.Context, spec orderbyRowPerEventIteratorCaseSpec, caseScenario compat.Scenario) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[orderbyRowPerEventAggMarket](env, orderbyRowPerEventIteratorMarketType); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[resultsetQueryTypeRowPerEventBeanString](env, orderbyRowPerEventIteratorSBSType); err != nil {
		return nil, err
	}
	// No output policy: the execution reads the statement iterator, and the
	// order-by symbol makes the iterator order deterministic (which Java pins
	// index-by-index through assertPropsPerRowIterator).
	query := esper.Join(
		esper.From[orderbyRowPerEventAggMarket](env, orderbyRowPerEventIteratorMarketType).Window(esper.LengthWindow(10)),
		esper.From[resultsetQueryTypeRowPerEventBeanString](env, orderbyRowPerEventIteratorSBSType).Window(esper.LengthWindow(100)),
		esper.OnEqual(esper.JoinField[string](0, "symbol"), esper.JoinField[string](1, "theString")),
	).Aggregate(
		esper.Alias("symbol", esper.JoinField[string](0, "symbol")),
		esper.Alias("sumPrice", esper.Sum[float64](esper.JoinField[float64](0, "price"))),
	).Query(esper.OrderBy(esper.Ascending(esper.JoinField[string](0, "symbol"))), esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return nil, err
	}
	engine, statement, err := deployParityStatementWithRuntime(ctx, env, plan, spec.runtimeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = engine.Close(context.Background()) }()

	records := make([]compat.TraceRecord, 0, spec.snapshots)
	snapshot := 0
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
			case orderbyRowPerEventIteratorMarketType:
				if err := requireResultSetQueryTypeLocalGroupByFields(fields, "symbol", "price", "volume"); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventIteratorMarketType, err)
				}
				var event orderbyRowPerEventAggMarket
				if err := json.Unmarshal(step.Payload, &event); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventIteratorMarketType, err)
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return nil, err
				}
			case orderbyRowPerEventIteratorSBSType:
				if err := requireResultSetQueryTypeLocalGroupByFields(fields, "theString"); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventIteratorSBSType, err)
				}
				var event resultsetQueryTypeRowPerEventBeanString
				if err := json.Unmarshal(step.Payload, &event); err != nil {
					return nil, fmt.Errorf("%s payload: %w", orderbyRowPerEventIteratorSBSType, err)
				}
				if err := engine.Send(ctx, step.EventType, event); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unknown %s event type %q", orderbyRowPerEventIteratorID, step.EventType)
			}
		case "snapshot":
			if step.Statement != statement.Name() {
				return nil, fmt.Errorf("snapshot metadata is not pinned")
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return nil, err
			}
			rows := resultSetQueryTypeLocalGroupUngroupedRows(result.Batch.New)
			if rows == nil {
				rows = []compat.ResultRecord{}
			}
			snapshot++
			records = append(records, compat.TraceRecord{
				Case:      spec.name,
				Operation: "snapshot",
				Statement: statement.Name(),
				Sequence:  uint64(snapshot),
				Time:      compat.FormatTraceTime(engine.Now()),
				New:       rows,
			})
		default:
			return nil, fmt.Errorf("unsupported %s step op %q", orderbyRowPerEventIteratorID, step.Op)
		}
	}
	if len(records) != spec.snapshots {
		return nil, fmt.Errorf("%s case %q produced %d snapshots, want %d",
			orderbyRowPerEventIteratorID, spec.name, len(records), spec.snapshots)
	}
	return records, nil
}

func validateOrderByRowPerEventIteratorTrace(trace compat.Trace) error {
	if trace.Version != compat.ScenarioVersion || trace.ID != orderbyRowPerEventIteratorID {
		return fmt.Errorf("%s trace identity is not pinned", orderbyRowPerEventIteratorID)
	}
	spec := orderbyRowPerEventIteratorCaseSpecs[0]
	if len(trace.Records) != spec.snapshots {
		return fmt.Errorf("%s trace must contain exactly %d records", orderbyRowPerEventIteratorID, spec.snapshots)
	}
	for index, record := range trace.Records {
		if record.Case != spec.name || record.Operation != "snapshot" || record.Statement != "s0" ||
			record.Sequence != uint64(index+1) || record.Time != "1970-01-01T00:00:00Z" {
			return fmt.Errorf("%s trace record %d is not pinned", orderbyRowPerEventIteratorID, index)
		}
		if len(record.Old) != 0 || len(record.New) != spec.snapshotLen[index] {
			return fmt.Errorf("%s trace record %d must carry %d rows and no old rows",
				orderbyRowPerEventIteratorID, index, spec.snapshotLen[index])
		}
	}
	return nil
}
