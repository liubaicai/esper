package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

var exprDTRoundJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTRound.java",
}

var exprDTRoundJavaRuntimeIDs = []string{
	"java-runtime-9838679b35a6a3507332",
	"java-runtime-8a02976a0c5c4eb03030",
	"java-runtime-95d240ad18c89abe6e99",
	"java-runtime-be79bf345b96e2ccc206",
}

var exprDTRoundJavaExecutions = []string{
	"ExprDTRoundInput",
	"ExprDTRoundCeil",
	"ExprDTRoundFloor",
	"ExprDTRoundHalf",
}

var exprDTRoundCases = []string{"input", "ceil", "floor", "half"}

func runExprDTRoundScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDTRoundCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDTRoundCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("scenario has no supported cases")
	}
	return trace, nil
}

func runExprDTRoundCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	seq := uint64(0)

	timeT := reflect.TypeOf(time.Time{})
	int64T := reflect.TypeOf(int64(0))
	if _, err := esper.RegisterMap(env, "SupportDateTime", []esper.FieldSpec{
		{Name: "longdate", Type: int64T},
		{Name: "utildate", Type: timeT},
		{Name: "caldate", Type: timeT},
		{Name: "localdate", Type: timeT},
		{Name: "zoneddate", Type: timeT},
	}); err != nil {
		return trace, fmt.Errorf("register SupportDateTime: %w", err)
	}

	// deployNamed builds and deploys one named statement; the half case
	// redeploys mid-case exactly like Java's undeployAll + recompile.
	var currentDeployment *esper.Deployment
	deployNamed := func(name string, build func() (esper.Query, error)) error {
		query, err := build()
		if err != nil {
			return err
		}
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		currentDeployment = deployment
		for _, statement := range deployment.Statements() {
			captured := statement
			if _, err := captured.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				newRows := compat.NormalizeResults(batch.New)
				exprDTRoundRenderMillis(newRows)
				if len(newRows) == 0 {
					return nil
				}
				seq++
				trace.Records = append(trace.Records, compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: captured.Name(),
					Sequence:  seq,
					Time:      batch.Time.UTC().Format(time.RFC3339),
					New:       newRows,
				})
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}
	undeployCurrent := func() error {
		if currentDeployment == nil {
			return nil
		}
		if err := currentDeployment.Undeploy(ctx); err != nil {
			return err
		}
		currentDeployment = nil
		return nil
	}

	longdate := esper.Field[map[string]any, int64]("longdate")
	utildate := esper.Field[map[string]any, time.Time]("utildate")
	caldate := esper.Field[map[string]any, time.Time]("caldate")
	localdate := esper.Field[map[string]any, time.Time]("localdate")
	zoneddate := esper.Field[map[string]any, time.Time]("zoneddate")

	roundSeven := func(build func(esper.Expression[time.Time], string) esper.Expression[time.Time]) esper.Query {
		units := []string{"msec", "sec", "minutes", "hour", "day", "month", "year"}
		selections := make([]esper.Selection, 0, len(units))
		for index, unit := range units {
			selections = append(selections, esper.Alias(fmt.Sprintf("val%d", index), build(utildate, unit)))
		}
		return esper.FromAny(env, "SupportDateTime").Select(selections...).Query(esper.StatementName("s0"))
	}
	buildInitial := func() (esper.Query, error) {
		switch caseName {
		case "input":
			return esper.FromAny(env, "SupportDateTime").Select(
				esper.Alias("val0", esper.DateTimeRoundCeiling[int64](longdate, "hour")),
				esper.Alias("val1", esper.DateTimeRoundCeiling[time.Time](utildate, "hour")),
				esper.Alias("val2", esper.DateTimeRoundCeiling[time.Time](caldate, "hour")),
				esper.Alias("val3", esper.DateTimeRoundCeiling[time.Time](localdate, "hour")),
				esper.Alias("val4", esper.DateTimeRoundCeiling[time.Time](zoneddate, "hour")),
			).Query(esper.StatementName("s0")), nil
		case "ceil":
			return roundSeven(esper.DateTimeRoundCeiling[time.Time]), nil
		case "floor":
			return roundSeven(esper.DateTimeRoundFloor[time.Time]), nil
		case "half":
			return roundSeven(esper.DateTimeRoundHalf[time.Time]), nil
		default:
			return esper.Query{}, fmt.Errorf("unsupported case %q", caseName)
		}
	}
	if err := deployNamed("s0", buildInitial); err != nil {
		return trace, err
	}

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "deploy":
			// round-half-min mirrors Java's undeployAll + recompile with the
			// same statement name; the per-case sequence counter survives.
			if step.Statement != "round-half-min" {
				return trace, fmt.Errorf("unsupported deploy %q", step.Statement)
			}
			if err := undeployCurrent(); err != nil {
				return trace, err
			}
			build := func() (esper.Query, error) {
				return esper.FromAny(env, "SupportDateTime").Select(
					esper.Alias("val0", esper.DateTimeRoundHalf[time.Time](utildate, "min")),
				).Query(esper.StatementName("s0")), nil
			}
			if err := deployNamed("s0", build); err != nil {
				return trace, err
			}
		case "send":
			var payload struct {
				Date string `json:"date"`
			}
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return trace, fmt.Errorf("decode SupportDateTime: %w", err)
			}
			parsed, err := time.Parse(time.RFC3339Nano, payload.Date)
			if err != nil {
				return trace, fmt.Errorf("parse date %q: %w", payload.Date, err)
			}
			// SupportDateTime.make derives every representation from one
			// instant; the calendar rep forces MILLISECOND to zero.
			event := map[string]any{
				"longdate":  parsed.UnixMilli(),
				"utildate":  parsed,
				"caldate":   parsed.Add(-time.Duration(parsed.Nanosecond())),
				"localdate": parsed,
				"zoneddate": parsed,
			}
			if err := engine.SendRecord(ctx, "SupportDateTime", event); err != nil {
				return trace, err
			}
		default:
			return trace, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

// exprDTRoundRenderMillis renders rounded time.Time cells as epoch millis,
// the oracle's instant-token convention for every SupportDateTime
// representation (Date/Calendar follow the ExprCoreExistsCast precedent;
// LocalDateTime/ZonedDateTime collapse the same way).
func exprDTRoundRenderMillis(rows []compat.ResultRecord) {
	for _, row := range rows {
		for name, value := range row.Fields {
			if located, ok := value.(time.Time); ok {
				row.Fields[name] = located.UnixMilli()
			}
		}
	}
}
