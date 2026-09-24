package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type eplInsertIntoTypedColumnsBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive int64  `esper:"longPrimitive"`
}

type eplInsertIntoTypedColumnsS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

type eplInsertIntoTypedColumnsS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

// eplInsertIntoTypedColumnsMinimalBean is the pojo-typed-column case's
// SupportBean: only theString is observable (the Java assertions read
// outputevent.getTheString()), so the registered schema carries just that
// property — an approved projection-to-asserted-fields difference.
type eplInsertIntoTypedColumnsMinimalBean struct {
	TheString string `esper:"theString"`
}

// eplInsertIntoTypedColumnsEmptyBean mirrors SupportBeanWithoutProps: a
// class-provided bean event type with zero properties.
type eplInsertIntoTypedColumnsEmptyBean struct{}

const eplInsertIntoTypedColumnsJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplInsertIntoTypedColumnsJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoEmptyPropType.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/insertinto/EPLInsertIntoEventTypedColumnFromProp.java",
}

var eplInsertIntoTypedColumnsJavaRuntimeIDs = []string{
	"java-runtime-0870abe075dc95308a27",
	"java-runtime-d585492dbeef1deee74f",
	"java-runtime-2fb237744f8a8b010414",
	"java-runtime-700d690d1c5c019ec414",
}

var eplInsertIntoTypedColumnsJavaExecutions = []string{
	"EPLInsertIntoNamedWindowModelAfter",
	"EPLInsertIntoCreateSchemaInsertInto",
	"EPLInsertIntoEventTypedColumnOnMerge",
	"EPLInsertIntoPOJOTypedColumnOnMerge",
}

// Case order is fixed; the three create-schema-* labels are the sub-rounds
// of the single EPLInsertIntoCreateSchemaInsertInto execution and share its
// runtime ID (the map sub-round's soda=true/false double run is a
// compile-path-only difference replayed once).
var eplInsertIntoTypedColumnsCases = []string{
	"named-window-model-after",
	"create-schema-map",
	"create-schema-objectarray",
	"create-schema-bean",
	"event-typed-column-on-merge",
	"pojo-typed-column-on-merge",
}

var eplInsertIntoTypedColumnsCaseRuntimeIDs = map[string]string{
	"named-window-model-after":    "java-runtime-0870abe075dc95308a27",
	"create-schema-map":           "java-runtime-d585492dbeef1deee74f",
	"create-schema-objectarray":   "java-runtime-d585492dbeef1deee74f",
	"create-schema-bean":          "java-runtime-d585492dbeef1deee74f",
	"event-typed-column-on-merge": "java-runtime-2fb237744f8a8b010414",
	"pojo-typed-column-on-merge":  "java-runtime-700d690d1c5c019ec414",
}

// runEplInsertIntoTypedColumnsScenario replays the four typed-column
// insert-into executions: empty-property schemas populated exclusively by
// `select null`, and event/POJO-typed table columns populated through
// on-merge inserts observed via a routed output stream.
func runEplInsertIntoTypedColumnsScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplInsertIntoTypedColumnsCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEplInsertIntoTypedColumnsCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("epl insert-into typed-columns case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("epl insert-into typed-columns scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runEplInsertIntoTypedColumnsCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	plans, snapshotWindows, err := buildEplInsertIntoTypedColumnsCase(env, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(eplInsertIntoTypedColumnsCaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()
	if err := engine.AdvanceTime(ctx, time.Unix(0, 0).UTC()); err != nil {
		return compat.Trace{}, err
	}

	// Snapshot sources read the named window's or table's iterator, matching
	// the Java statement-iterator assertions.
	snapshotSource := make(map[string]func() (esper.QueryResult, error), len(snapshotWindows))
	for label, window := range snapshotWindows {
		name := window
		snapshotSource[label] = func() (esper.QueryResult, error) {
			plan, err := env.Build(esper.FromNamedWindow(env, name).Query())
			if err != nil {
				return esper.QueryResult{}, err
			}
			return engine.ExecuteFireAndForget(ctx, plan)
		}
	}

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	sequences := make(map[string]uint64)
	valueEmitted := make(map[string]bool)
	// Callback records buffer per step and flush in the frozen contract
	// order listener, subscriber, value; Esper dispatches subscribers
	// before listeners, so the natural callback order cannot be used.
	var pending []compat.TraceRecord
	flushPending := func() {
		order := map[string]int{"listener": 0, "subscriber": 1, "value": 2}
		sort.SliceStable(pending, func(i, j int) bool {
			return order[pending[i].Operation] < order[pending[j].Operation]
		})
		trace.Records = append(trace.Records, pending...)
		pending = pending[:0]
	}
	// Only the create-schema sub-rounds pin the delivered row's event-type
	// name through the listener; the named-window case uses a value step.
	emitValueOnFirstDelivery := strings.HasPrefix(caseName, "create-schema-")
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return trace, err
		}
		for _, statement := range deployment.Statements() {
			if statement.Name() != "s0" {
				continue
			}
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				sequences["listener:"+stmt.Name()]++
				pending = append(pending, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  sequences["listener:"+stmt.Name()],
					Time:      engine.Now().UTC().Format(time.RFC3339Nano),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				})
				// The Java assertions also pin the delivered event's type
				// name; emit it once per statement as a value record.
				if emitValueOnFirstDelivery && !valueEmitted["listener:"+stmt.Name()] {
					for _, result := range batch.New {
						if event, ok := result.Event(); ok {
							valueEmitted["listener:"+stmt.Name()] = true
							pending = append(pending, compat.TraceRecord{
								Case:      caseName,
								Operation: "value",
								Statement: stmt.Name(),
								Value:     event.TypeName(),
							})
							break
						}
					}
				}
				return nil
			}); err != nil {
				return trace, err
			}
			if caseName == "create-schema-objectarray" {
				if err := stmt.SetSubscriber(func(_ context.Context, update esper.SubscriberUpdate) error {
					if len(update.NewRows()) == 0 && len(update.OldRows()) == 0 {
						return nil
					}
					sequences["subscriber:"+stmt.Name()]++
					newRows := make([]compat.ResultRecord, 0, len(update.NewRows()))
					for _, row := range update.NewRows() {
						newRows = append(newRows, compat.NormalizeResults([]esper.Result{row.Result()})...)
					}
					oldRows := make([]compat.ResultRecord, 0, len(update.OldRows()))
					for _, row := range update.OldRows() {
						oldRows = append(oldRows, compat.NormalizeResults([]esper.Result{row.Result()})...)
					}
					pending = append(pending, compat.TraceRecord{
						Case:      caseName,
						Operation: "subscriber",
						Statement: stmt.Name(),
						Sequence:  sequences["subscriber:"+stmt.Name()],
						Time:      engine.Now().UTC().Format(time.RFC3339Nano),
						New:       newRows,
						Old:       oldRows,
					})
					return nil
				}); err != nil {
					return trace, err
				}
			}
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
			payload, err := decodeEplInsertIntoTypedColumnsPayload(step, caseName)
			if err != nil {
				return trace, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return trace, err
			}
		case "advance-time":
			at, err := time.Parse(time.RFC3339Nano, step.At)
			if err != nil {
				return trace, err
			}
			if err := engine.AdvanceTime(ctx, at); err != nil {
				return trace, err
			}
		case "snapshot":
			source, ok := snapshotSource[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown epl insert-into typed-columns snapshot statement %q", step.Statement)
			}
			result, err := source()
			if err != nil {
				return trace, err
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Time:      engine.Now().UTC().Format(time.RFC3339Nano),
				New:       compat.NormalizeResults(result.Batch.New),
			})
		case "value":
			// Pins the Java getEventType().getName() assertion: the event
			// type name of the first row of a fresh snapshot.
			source, ok := snapshotSource[step.Statement]
			if !ok {
				return trace, fmt.Errorf("unknown epl insert-into typed-columns value statement %q", step.Statement)
			}
			result, err := source()
			if err != nil {
				return trace, err
			}
			var typeName string
			found := false
			for _, row := range result.Batch.New {
				if event, ok := row.Event(); ok {
					typeName = event.TypeName()
					found = true
					break
				}
				if event, ok := row.RowEvent(); ok {
					typeName = event.TypeName()
					found = true
					break
				}
			}
			if !found {
				return trace, fmt.Errorf("no rows for value op on statement %q", step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "value",
				Statement: step.Statement,
				Value:     typeName,
			})
		case "faf-insert":
			// compileExecuteFAFNoResult("insert into <W> select null"): one
			// empty row lands in the zero-column window.
			plan, err := env.Build(esper.FromNamedWindow(env, step.Statement).OnDemand().InsertRows(esper.InsertValues()))
			if err != nil {
				return trace, err
			}
			if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
				return trace, err
			}
		case "faf-delete":
			plan, err := env.Build(esper.FromNamedWindow(env, step.Statement).OnDemand().DeleteAll())
			if err != nil {
				return trace, err
			}
			if _, err := engine.ExecuteFireAndForget(ctx, plan); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported epl insert-into typed-columns step op %q", step.Op)
		}
		flushPending()
	}
	flushPending()
	return trace, nil
}

// buildEplInsertIntoTypedColumnsCase registers the case's schemas, windows
// and tables and returns the deployment plans in pinned order plus the
// snapshot-label -> window/table-name map.
func buildEplInsertIntoTypedColumnsCase(env *esper.Environment, caseName string) ([]esper.Plan, map[string]string, error) {
	switch caseName {
	case "named-window-model-after":
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsBean](env, "SupportBean"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsS0](env, "SupportBean_S0"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsS1](env, "SupportBean_S1"); err != nil {
			return nil, nil, err
		}
		// The schema registers under the window's own name so the iterated
		// row's event-type name is EmptyPropWin, matching the Java window
		// type assertion.
		windowSchema, err := esper.RegisterMap(env, "EmptyPropWinType", nil)
		if err != nil {
			return nil, nil, err
		}
		if _, err := esper.CreateNamedWindow(env, "EmptyPropWin", windowSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.From[eplInsertIntoTypedColumnsBean](env, "SupportBean").
			InsertInto("EmptyPropWin"))
		if err != nil {
			return nil, nil, err
		}
		mergePlan, err := env.Build(esper.OnEvent(esper.From[eplInsertIntoTypedColumnsS0](env, "SupportBean_S0")).
			MergeIntoNamedWindowWhen("EmptyPropWin", nil,
				esper.WhenNotMatchedAny(esper.CopyMatchingFields()),
			).Query(esper.StatementName("on-merge")))
		if err != nil {
			return nil, nil, err
		}
		onInsertPlan, err := env.Build(esper.OnEvent(esper.From[eplInsertIntoTypedColumnsS1](env, "SupportBean_S1")).
			InsertIntoNamedWindow("EmptyPropWin", esper.CopyMatchingFields()).
			Query(esper.StatementName("on-insert")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan, mergePlan, onInsertPlan}, map[string]string{"window": "EmptyPropWin"}, nil
	case "create-schema-map":
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsBean](env, "SupportBean"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterMap(env, "EmptyMapSchema", nil); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.From[eplInsertIntoTypedColumnsBean](env, "SupportBean").
			InsertInto("EmptyMapSchema"))
		if err != nil {
			return nil, nil, err
		}
		s0Plan, err := env.Build(esper.FromAny(env, "EmptyMapSchema").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil, nil
	case "create-schema-objectarray":
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsBean](env, "SupportBean"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterObjectArray(env, "EmptyOASchema", nil); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.From[eplInsertIntoTypedColumnsBean](env, "SupportBean").
			InsertInto("EmptyOASchema"))
		if err != nil {
			return nil, nil, err
		}
		s0Plan, err := env.Build(esper.FromAny(env, "EmptyOASchema").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil, nil
	case "create-schema-bean":
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsBean](env, "SupportBean"); err != nil {
			return nil, nil, err
		}
		if _, err := esper.RegisterStruct[eplInsertIntoTypedColumnsEmptyBean](env, "MyBeanWithoutProps"); err != nil {
			return nil, nil, err
		}
		insertPlan, err := env.Build(esper.From[eplInsertIntoTypedColumnsBean](env, "SupportBean").
			InsertInto("MyBeanWithoutProps"))
		if err != nil {
			return nil, nil, err
		}
		s0Plan, err := env.Build(esper.FromAny(env, "MyBeanWithoutProps").Query(esper.StatementName("s0")))
		if err != nil {
			return nil, nil, err
		}
		return []esper.Plan{insertPlan, s0Plan}, nil, nil
	case "event-typed-column-on-merge":
		return buildEplInsertIntoTypedColumnsEventCase(env)
	case "pojo-typed-column-on-merge":
		return buildEplInsertIntoTypedColumnsPojoCase(env)
	default:
		return nil, nil, fmt.Errorf("unsupported epl insert-into typed-columns case %q", caseName)
	}
}

func buildEplInsertIntoTypedColumnsEventCase(env *esper.Environment) ([]esper.Plan, map[string]string, error) {
	carEvent, err := esper.RegisterMap(env, "CarEvent", []esper.FieldSpec{
		esper.FieldDef("carId", reflect.TypeOf("")),
		esper.FieldDef("tracked", reflect.TypeOf(false)),
	})
	if err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterMap(env, "CarOutputStream", []esper.FieldSpec{
		esper.FieldDef("status", reflect.TypeOf("")),
		esper.FieldDef("outputevent", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("outputevent", carEvent)); err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterMap(env, "CarTimeoutStream", []esper.FieldSpec{
		esper.FieldDef("carId", reflect.TypeOf("")),
		esper.FieldDef("tracked", reflect.TypeOf(false)),
	}); err != nil {
		return nil, nil, err
	}
	if _, err := esper.CreateTable(env, "StatusTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("carId"),
		esper.TableColumnOf[esper.Event]("lastevent", esper.WithTableColumnNestedSchema(carEvent)),
	}); err != nil {
		return nil, nil, err
	}
	mergePlan, err := env.Build(esper.OnRecord(esper.FromAny(env, "CarEvent").
		Filter(esper.Equal[bool](esper.Field[any, bool]("tracked"), esper.Literal(true)))).
		MergeIntoTableWhen("StatusTable",
			[]esper.Expr{esper.Field[any, string]("carId")},
			esper.WhenMatchedAny(
				esper.SetColumn("lastevent", esper.EventValue[esper.Event]()),
			),
			esper.WhenNotMatchedActions(
				esper.ThenInsertIntoTarget(
					esper.SetColumn("carId", esper.Field[any, string]("carId")),
					esper.SetColumn("lastevent", esper.EventValue[esper.Event]()),
				),
				esper.ThenInsertInto("CarOutputStream",
					esper.Alias("status", esper.Literal("online")),
					esper.Alias("outputevent", esper.EventValue[esper.Event]()),
				),
			),
		).Query(esper.StatementName("car-merge")))
	if err != nil {
		return nil, nil, err
	}
	// every e=CarEvent(tracked=true) -> (timer:interval(1 minutes) and not
	// CarEvent(carId = e.carId, tracked=true)); select e.* is projected
	// per-property (no wildcard-tag helper — approved difference). All
	// pattern legs share one source node so the combined pattern has a
	// single input; the untagged not-leg carries a dummy tag that no
	// statement references.
	carStream := esper.From[map[string]any](env, "CarEvent")
	timeoutPlan, err := env.Build(
		esper.PatternFrom(carStream, "e",
			esper.Equal[bool](esper.Field[map[string]any, bool]("tracked"), esper.Literal(true))).
			Every().
			Then(
				esper.TimerInterval(carStream, time.Minute).
					And(esper.PatternFrom(carStream, "n",
						esper.And(
							esper.Equal[string](esper.Field[map[string]any, string]("carId"), esper.TagField[string]("e", "carId")),
							esper.Equal[bool](esper.Field[map[string]any, bool]("tracked"), esper.Literal(true)),
						)).Not()),
			).
			Select(
				esper.Alias("carId", esper.TagField[string]("e", "carId")),
				esper.Alias("tracked", esper.TagField[bool]("e", "tracked")),
			).InsertInto("CarTimeoutStream"))
	if err != nil {
		return nil, nil, err
	}
	timeoutMergePlan, err := env.Build(esper.OnRecord(esper.FromAny(env, "CarTimeoutStream")).
		MergeIntoTableWhen("StatusTable",
			[]esper.Expr{esper.Field[any, string]("carId")},
			// Go's ThenDelete terminates the matched action chain for table
			// targets, so the side-stream insert runs first; observably
			// identical because lastevent reads the pre-delete row.
			esper.WhenMatchedActions(
				esper.ThenInsertInto("CarOutputStream",
					esper.Alias("status", esper.Literal("offline")),
					esper.Alias("outputevent", esper.TableField[esper.Event]("lastevent")),
				),
				esper.ThenDelete(esper.Literal(true)),
			),
		).Query(esper.StatementName("timeout-merge")))
	if err != nil {
		return nil, nil, err
	}
	s0Plan, err := env.Build(esper.FromAny(env, "CarOutputStream").Query(esper.StatementName("s0")))
	if err != nil {
		return nil, nil, err
	}
	return []esper.Plan{mergePlan, timeoutPlan, timeoutMergePlan, s0Plan}, nil, nil
}

func buildEplInsertIntoTypedColumnsPojoCase(env *esper.Environment) ([]esper.Plan, map[string]string, error) {
	beanSchema, err := esper.RegisterStruct[eplInsertIntoTypedColumnsMinimalBean](env, "SupportBean")
	if err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterMap(env, "CarOutputStream", []esper.FieldSpec{
		esper.FieldDef("status", reflect.TypeOf("")),
		esper.FieldDef("outputevent", reflect.TypeOf(esper.Event{})),
	}, esper.WithNestedPropertySchema("outputevent", beanSchema)); err != nil {
		return nil, nil, err
	}
	if _, err := esper.RegisterMap(env, "CarTimeoutStream", []esper.FieldSpec{
		esper.FieldDef("theString", reflect.TypeOf("")),
	}); err != nil {
		return nil, nil, err
	}
	if _, err := esper.CreateTable(env, "StatusTable", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("theString"),
		esper.TableColumnOf[esper.Event]("lastevent", esper.WithTableColumnNestedSchema(beanSchema)),
	}); err != nil {
		return nil, nil, err
	}
	mergePlan, err := env.Build(esper.OnEvent(esper.From[eplInsertIntoTypedColumnsMinimalBean](env, "SupportBean")).
		MergeIntoTableWhen("StatusTable",
			[]esper.Expr{esper.Field[eplInsertIntoTypedColumnsMinimalBean, string]("theString")},
			esper.WhenMatchedAny(
				esper.SetColumn("lastevent", esper.EventValue[esper.Event]()),
			),
			esper.WhenNotMatchedActions(
				esper.ThenInsertIntoTarget(
					esper.SetColumn("theString", esper.Field[eplInsertIntoTypedColumnsMinimalBean, string]("theString")),
					esper.SetColumn("lastevent", esper.EventValue[esper.Event]()),
				),
				esper.ThenInsertInto("CarOutputStream",
					esper.Alias("status", esper.Literal("online")),
					esper.Alias("outputevent", esper.EventValue[esper.Event]()),
				),
			),
		).Query(esper.StatementName("car-merge")))
	if err != nil {
		return nil, nil, err
	}
	beanStream := esper.From[eplInsertIntoTypedColumnsMinimalBean](env, "SupportBean")
	timeoutPlan, err := env.Build(
		esper.PatternFrom(beanStream, "e", esper.Literal(true)).
			Every().
			Then(
				esper.TimerInterval(beanStream, time.Minute).
					And(esper.PatternFrom(beanStream, "n",
						esper.Equal[string](esper.Field[eplInsertIntoTypedColumnsMinimalBean, string]("theString"), esper.TagField[string]("e", "theString"))).Not()),
			).
			Select(
				esper.Alias("theString", esper.TagField[string]("e", "theString")),
			).InsertInto("CarTimeoutStream"))
	if err != nil {
		return nil, nil, err
	}
	timeoutMergePlan, err := env.Build(esper.OnRecord(esper.FromAny(env, "CarTimeoutStream")).
		MergeIntoTableWhen("StatusTable",
			[]esper.Expr{esper.Field[any, string]("theString")},
			esper.WhenMatchedActions(
				esper.ThenInsertInto("CarOutputStream",
					esper.Alias("status", esper.Literal("offline")),
					esper.Alias("outputevent", esper.TableField[esper.Event]("lastevent")),
				),
				esper.ThenDelete(esper.Literal(true)),
			),
		).Query(esper.StatementName("timeout-merge")))
	if err != nil {
		return nil, nil, err
	}
	s0Plan, err := env.Build(esper.FromAny(env, "CarOutputStream").Query(esper.StatementName("s0")))
	if err != nil {
		return nil, nil, err
	}
	return []esper.Plan{mergePlan, timeoutPlan, timeoutMergePlan, s0Plan}, nil, nil
}

func decodeEplInsertIntoTypedColumnsPayload(step compat.Step, caseName string) (any, error) {
	switch step.EventType {
	case "SupportBean":
		if caseName == "pojo-typed-column-on-merge" {
			// The pojo case registers the asserted-field SupportBean schema;
			// decode into the matching minimal bean.
			var value eplInsertIntoTypedColumnsMinimalBean
			if err := json.Unmarshal(step.Payload, &value); err != nil {
				return nil, fmt.Errorf("decode SupportBean: %w", err)
			}
			return value, nil
		}
		var value eplInsertIntoTypedColumnsBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return value, nil
	case "SupportBean_S0":
		var value eplInsertIntoTypedColumnsS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value eplInsertIntoTypedColumnsS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return value, nil
	case "CarEvent":
		var value map[string]any
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode CarEvent: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported epl insert-into typed-columns event type %q", step.EventType)
	}
}
