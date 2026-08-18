package parity

import (
	"context"
	"encoding/json"
	"fmt"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

func subselectMultirowGroupSelections() []esper.Selection {
	return []esper.Selection{
		esper.Alias("c0", esper.Field[any, string]("theString")),
		esper.Alias("c1", esper.Sum[int](esper.Field[any, int]("intPrimitive"))),
	}
}

func subselectMultirowKeepAllWindow(env *esper.Environment, name string) error {
	schema, err := esper.StructSchema[subselectMultirowBean]("SupportBean")
	if err != nil {
		return err
	}
	_, err = esper.CreateNamedWindow(env, name, schema)
	return err
}
func subselectMultirowDeploy(ctx context.Context, env *esper.Environment, plans ...esper.Plan) (*esper.Engine, []*esper.Statement, error) {
	engine := esper.NewEngine(env)
	statements := make([]*esper.Statement, 0, len(plans))
	for _, plan := range plans {
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			_ = engine.Close(context.Background())
			return nil, nil, err
		}
		if len(deployment.Statements()) != 1 {
			_ = engine.Close(context.Background())
			return nil, nil, fmt.Errorf("expected one statement, got %d", len(deployment.Statements()))
		}
		statements = append(statements, deployment.Statements()[0])
	}
	return engine, statements, nil
}

func subselectMultirowResolver(statement *esper.Statement) compat.StatementResolver {
	return func(name string) (*esper.Statement, error) {
		if name != statement.Name() {
			return nil, fmt.Errorf("unknown subselect-multirow statement %q", name)
		}
		return statement, nil
	}
}

// runSubselectMultirowIndexShareCase covers
// EPLSubselectMultirowGroupedNamedWindowSubqueryIndexShared: the window is
// created and pre-populated with E1 rows before the s0 deploy, then the
// uncorrelated variant (group-rows take(1)) and the correlated variant
// (where theString = s0.p00).
func runSubselectMultirowIndexShareCase(ctx context.Context, env *esper.Environment, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	if err := subselectMultirowKeepAllWindow(env, "MyWindow"); err != nil {
		return compat.Trace{}, err
	}
	insertPlan, err := env.Build(esper.OnEvent(esper.From[subselectMultirowBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		esper.SetColumn("theString", esper.Field[subselectMultirowBean, string]("theString")),
		esper.SetColumn("intPrimitive", esper.Field[subselectMultirowBean, int]("intPrimitive")),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	options := []esper.SubqueryGroupOption{}
	if caseName == "indexshare-correlated" {
		options = append(options, esper.SubqueryGroupWhere(esper.Equal[string](
			esper.Field[any, string]("theString"),
			esper.OuterField[string]("p00"),
		)))
	}
	query := esper.Select(esper.From[subselectMultirowS0](env, "SupportBean_S0"),
		esper.Alias("e1", esper.EnumTake[map[string]any](
			esper.SubqueryGroupRows(esper.FromNamedWindow(env, "MyWindow"),
				esper.Field[any, string]("theString"), subselectMultirowGroupSelections(), options...), 10)),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statements, err := subselectMultirowDeploy(ctx, env, insertPlan, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statements[1], scenario, decodeSubselectMultirowPayload,
		subselectMultirowResolver(statements[1]))
}

// runSubselectMultirowNamedWindowDeleteCase covers the named-window with
// on-delete variant of EPLSubselectMulticolumnGroupedUncorrelatedUnfiltered:
// S1 delete events remove MyWindow rows where S1.id = window intPrimitive.
func runSubselectMultirowNamedWindowDeleteCase(ctx context.Context, env *esper.Environment, scenario compat.Scenario) (compat.Trace, error) {
	if err := subselectMultirowKeepAllWindow(env, "MyWindow"); err != nil {
		return compat.Trace{}, err
	}
	insertPlan, err := env.Build(esper.OnEvent(esper.From[subselectMultirowBean](env, "SupportBean")).InsertIntoNamedWindow(
		"MyWindow",
		esper.SetColumn("theString", esper.Field[subselectMultirowBean, string]("theString")),
		esper.SetColumn("intPrimitive", esper.Field[subselectMultirowBean, int]("intPrimitive")),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	deletePlan, err := env.Build(esper.OnEvent(esper.From[subselectMultirowS1](env, "SupportBean_S1")).DeleteFromNamedWindow(
		"MyWindow",
		esper.Equal[int](esper.NamedWindowField[int]("intPrimitive"), esper.Field[subselectMultirowS1, int]("id")),
	).Query(esper.StatementName("delete")))
	if err != nil {
		return compat.Trace{}, err
	}
	query := esper.Select(esper.From[subselectMultirowS0](env, "SupportBean_S0"),
		esper.Alias("subq", esper.SubqueryGroupRow(esper.FromNamedWindow(env, "MyWindow"),
			esper.Field[any, string]("theString"), subselectMultirowGroupSelections())),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statements, err := subselectMultirowDeploy(ctx, env, insertPlan, deletePlan, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statements[2], scenario, decodeSubselectMultirowPayload,
		subselectMultirowResolver(statements[2]))
}

// runSubselectMultirowIteratorCase covers
// EPLSubselectMultirowGroupedUncorrelatedIteratorAndExpressionDef: e1 uses
// the un-enumerated grouped row form and e2 the enumerated collection form,
// each over SupportBean#keepall with the outer stream at #lastevent; the
// snapshot op asserts the statement iterator matches the listener row.
func runSubselectMultirowIteratorCase(ctx context.Context, env *esper.Environment, scenario compat.Scenario) (compat.Trace, error) {
	inner := func() esper.RecordStream {
		return esper.From[subselectMultirowBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
	}
	query := esper.Select(esper.From[subselectMultirowS0](env, "SupportBean_S0").Window(esper.LastEvent()),
		esper.Alias("e1", esper.SubqueryGroupRow(inner(), esper.Field[any, string]("theString"), subselectMultirowGroupSelections())),
		esper.Alias("e2", esper.EnumTake[map[string]any](esper.SubqueryGroupRows(inner(),
			esper.Field[any, string]("theString"), subselectMultirowGroupSelections()), 10)),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statements, err := subselectMultirowDeploy(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statements[0], scenario, decodeSubselectMultirowPayload,
		subselectMultirowResolver(statements[0]))
}

func runSubselectMultirowContextCase(ctx context.Context, env *esper.Environment, scenario compat.Scenario) (compat.Trace, error) {
	if _, err := esper.CreateKeyContextByStreams(env, "MyCtx",
		esper.KeyContextStream{Type: "SupportBean", Keys: []esper.Expr{esper.Field[subselectMultirowBean, string]("theString")}},
		esper.KeyContextStream{Type: "SupportBean_S0", Keys: []esper.Expr{esper.Field[subselectMultirowS0, string]("p00")}},
	); err != nil {
		return compat.Trace{}, err
	}
	// The inner subquery shares the statement context: a query-scoped select
	// over SupportBean (no keepall window, mirroring the Java
	// SupportBean#keepall context-partitioned stream) keeps the groups
	// partition-local.
	inner := esper.Select(esper.From[subselectMultirowBean](env, "SupportBean").Window(esper.KeepAll()))
	query := esper.Select(esper.From[subselectMultirowS0](env, "SupportBean_S0"),
		esper.Alias("subq", esper.SubqueryGroupRow(inner,
			esper.Field[any, string]("theString"), subselectMultirowGroupSelections())),
	).Query(esper.StatementName("s0"), esper.WithContext("MyCtx"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statements, err := subselectMultirowDeploy(ctx, env, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statements[0], scenario, decodeSubselectMultirowPayload,
		subselectMultirowResolver(statements[0]))
}

// runSubselectMultirowIndexShareArrayCase covers
// EPLSubselectMultirowGroupedIndexSharedMultikeyWArray: the window over
// SupportEventWithManyArray is created and pre-populated before the s0
// deploy; the group key is the int[] intOne field and the projection sums
// value.
func runSubselectMultirowIndexShareArrayCase(ctx context.Context, env *esper.Environment, scenario compat.Scenario) (compat.Trace, error) {
	schema, err := esper.StructSchema[subselectMultirowManyArray]("SupportEventWithManyArray")
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.CreateNamedWindow(env, "MyWindow", schema); err != nil {
		return compat.Trace{}, err
	}
	insertPlan, err := env.Build(esper.OnEvent(esper.From[subselectMultirowManyArray](env, "SupportEventWithManyArray")).InsertIntoNamedWindow(
		"MyWindow",
		esper.SetColumn("id", esper.Field[subselectMultirowManyArray, string]("id")),
		esper.SetColumn("intOne", esper.Field[subselectMultirowManyArray, []int]("intOne")),
		esper.SetColumn("value", esper.Field[subselectMultirowManyArray, int]("value")),
	).Query(esper.StatementName("insert")))
	if err != nil {
		return compat.Trace{}, err
	}
	query := esper.Select(esper.From[subselectMultirowS0](env, "SupportBean_S0"),
		esper.Alias("e1", esper.EnumTake[map[string]any](
			esper.SubqueryGroupRows(esper.FromNamedWindow(env, "MyWindow"),
				esper.Field[any, []int]("intOne"),
				[]esper.Selection{
					esper.Alias("c0", esper.Field[any, []int]("intOne")),
					esper.Alias("c1", esper.Sum[int](esper.Field[any, int]("value"))),
				}), 10)),
	).Query(esper.StatementName("s0"))
	plan, err := env.Build(query)
	if err != nil {
		return compat.Trace{}, err
	}
	engine, statements, err := subselectMultirowDeploy(ctx, env, insertPlan, plan)
	if err != nil {
		return compat.Trace{}, err
	}
	defer func() { _ = engine.Close(context.Background()) }()
	return compat.ReplayWithStatements(ctx, engine, statements[1], scenario, decodeSubselectMultirowPayload,
		subselectMultirowResolver(statements[1]))
}

func decodeSubselectMultirowPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean_S0":
		var value subselectMultirowS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportBean_S0: %w", err)
		}
		return value, nil
	case "SupportBean_S1":
		var value subselectMultirowS1
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportBean_S1: %w", err)
		}
		return value, nil
	case "SupportBean":
		var value subselectMultirowBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportBean: %w", err)
		}
		return value, nil
	case "SupportEventWithIntArray":
		var value subselectMultirowIntArray
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportEventWithIntArray: %w", err)
		}
		return value, nil
	case "SupportEventWithManyArray":
		var value subselectMultirowManyArray
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("subselect-multirow SupportEventWithManyArray: %w", err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("subselect-multirow: unsupported event type %q", step.EventType)
	}
}
