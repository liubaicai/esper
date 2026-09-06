package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	infraNWTableSubqUncorrelID          = "infra-nwtable-subq-uncorrel"
	infraNWTableSubqUncorrelDescription = "InfraNWTableSubqUncorrel uncorrelated scalar subqueries (select a from MyInfra) over a keepall named window and over a table, covering named-window subquery index sharing through enable_window_subquery_indexshare create hints and disable_window_subquery_indexshare consumer hints, multi-row scalar null results, and on-delete row propagation captured from create, select, selectTwo, and delete listeners (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqUncorrel.java)."
	infraNWTableSubqUncorrelJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWTableSubqUncorrelSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqUncorrel.java"

	infraNWTableSubqUncorrelCreateNW   = "@name('create') @public create window MyInfra#keepall as select theString as a, longPrimitive as b, longBoxed as c from SupportBean"
	infraNWTableSubqUncorrelCreateNWES = "@Hint('enable_window_subquery_indexshare') " + infraNWTableSubqUncorrelCreateNW
	infraNWTableSubqUncorrelCreateTbl  = "@name('create') @public create table MyInfra(a string primary key, b long, c long)"
	infraNWTableSubqUncorrelInsert     = "insert into MyInfra select theString as a, longPrimitive as b, longBoxed as c from SupportBean"
	infraNWTableSubqUncorrelSelect     = "@name('select') select irstream (select a from MyInfra) as value, symbol from SupportMarketDataBean"
	infraNWTableSubqUncorrelSelectHint = "@Hint('disable_window_subquery_indexshare') " + infraNWTableSubqUncorrelSelect
	infraNWTableSubqUncorrelSelectTwo  = "@name('selectTwo') select irstream (select a from MyInfra) as value, symbol from SupportMarketDataBean"
	infraNWTableSubqUncorrelSelectTwoH = "@Hint('disable_window_subquery_indexshare') " + infraNWTableSubqUncorrelSelectTwo
	infraNWTableSubqUncorrelDelete     = "@name('delete') on SupportBean_A delete from MyInfra where id = a"
)

var (
	infraNWTableSubqUncorrelJavaSources = []string{
		infraNWTableSubqUncorrelSource,
	}
	infraNWTableSubqUncorrelJavaRuntimeIDs = []string{
		"java-runtime-e175cfaf55ce541ca088",
		"java-runtime-f03db66c82987b5cef54",
		"java-runtime-d521ebd5c45c89c8e842",
		"java-runtime-1a498d1407159a6d8c90",
	}
	infraNWTableSubqUncorrelJavaExecutions = []string{
		"InfraNWTableSubqUncorrelAssertion{namedWindow=true, enableIndexShareCreate=false, disableIndexShareConsumer=false}",
		"InfraNWTableSubqUncorrelAssertion{namedWindow=true, enableIndexShareCreate=true, disableIndexShareConsumer=false}",
		"InfraNWTableSubqUncorrelAssertion{namedWindow=true, enableIndexShareCreate=true, disableIndexShareConsumer=true}",
		"InfraNWTableSubqUncorrelAssertion{namedWindow=false, enableIndexShareCreate=false, disableIndexShareConsumer=false}",
	}
	infraNWTableSubqUncorrelJavaStaticIDs = []string{
		"java-5af40542812749ae30e4",
		"java-5af40542812749ae30e4",
		"java-5af40542812749ae30e4",
		"java-5af40542812749ae30e4",
	}
	infraNWTableSubqUncorrelCases = []string{
		"nw-no-share",
		"nw-index-share",
		"nw-disable-share",
		"table",
	}
	infraNWTableSubqUncorrelOrdinals = []int{0, 1, 0, 1}
)

type infraNWTableSubqUncorrelBean struct {
	TheString     string `esper:"theString"`
	LongPrimitive int64  `esper:"longPrimitive"`
	LongBoxed     *int64 `esper:"longBoxed"`
}

type infraNWTableSubqUncorrelMarket struct {
	Symbol string `esper:"symbol"`
}

type infraNWTableSubqUncorrelA struct {
	ID string `esper:"id"`
}

func loadInfraNWTableSubqUncorrelScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableSubqUncorrelID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableSubqUncorrelID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqUncorrelID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableSubqUncorrelID, err)
	}
	if err := requireInfraNWTableSubqUncorrelFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableSubqUncorrelID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableSubqUncorrelID ||
		metadata.Description != infraNWTableSubqUncorrelDescription ||
		metadata.JavaCommit != infraNWTableSubqUncorrelJavaCommit ||
		metadata.JavaSource != infraNWTableSubqUncorrelSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableSubqUncorrelID)
	}
	if err := validateInfraNWTableSubqUncorrelStringArray(root["javaRuntimes"], infraNWTableSubqUncorrelJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqUncorrelStringArray(root["javaNames"], infraNWTableSubqUncorrelJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqUncorrelStringArray(root["javaStaticIds"], infraNWTableSubqUncorrelJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableSubqUncorrelStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableSubqUncorrelCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly four cases", infraNWTableSubqUncorrelID)
	}
	selectEPL := func(caseIndex int) string {
		if caseIndex == 2 {
			return infraNWTableSubqUncorrelSelectHint
		}
		return infraNWTableSubqUncorrelSelect
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableSubqUncorrelFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case              string `json:"case"`
			Ordinal           int    `json:"ordinal"`
			RuntimeID         string `json:"runtimeId"`
			ExecutionName     string `json:"executionName"`
			Observation       string `json:"observation"`
			IteratorSnapshots int    `json:"iteratorSnapshots"`
			EPL               string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != infraNWTableSubqUncorrelCases[index] ||
			definition.Ordinal != infraNWTableSubqUncorrelOrdinals[index] ||
			definition.RuntimeID != infraNWTableSubqUncorrelJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableSubqUncorrelJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != selectEPL(index) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableSubqUncorrelID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableSubqUncorrelID)
	}
	steps := make([]compat.Step, len(rawSteps))
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
			if err := requireInfraNWTableSubqUncorrelFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableSubqUncorrelFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableSubqUncorrelFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableSubqUncorrelPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableSubqUncorrelFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraNWTableSubqUncorrelScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// infraNWTableSubqUncorrelCreateEPL pins the create statement per case.
func infraNWTableSubqUncorrelCreateEPL(caseName string) string {
	switch caseName {
	case "nw-no-share":
		return infraNWTableSubqUncorrelCreateNW
	case "nw-index-share", "nw-disable-share":
		return infraNWTableSubqUncorrelCreateNWES
	default:
		return infraNWTableSubqUncorrelCreateTbl
	}
}

func infraNWTableSubqUncorrelIsTable(caseName string) bool {
	return caseName == "table"
}

func validateInfraNWTableSubqUncorrelScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraNWTableSubqUncorrelID {
		return fmt.Errorf("%s scenario shape is not pinned", infraNWTableSubqUncorrelID)
	}
	// Per case: marker + 5 deploys (create/insert/select [mid-timeline selectTwo/delete]...) —
	// selectTwo deploys mid-timeline after the S1 insert, delete after the M2 send.
	type deployPin struct {
		statement string
		epl       string
	}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqUncorrelCases {
		steps := scenario.Steps[offset:]
		if len(steps) < 2 {
			return fmt.Errorf("%s case %q steps are truncated", infraNWTableSubqUncorrelID, caseName)
		}
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", infraNWTableSubqUncorrelID, caseName)
		}
		selectEPL := infraNWTableSubqUncorrelSelect
		selectTwoEPL := infraNWTableSubqUncorrelSelectTwo
		if caseIndex == 2 {
			selectEPL = infraNWTableSubqUncorrelSelectHint
			selectTwoEPL = infraNWTableSubqUncorrelSelectTwoH
		}
		createEPL := infraNWTableSubqUncorrelCreateEPL(caseName)
		deploys := map[string]string{
			"create":    createEPL,
			"insert":    infraNWTableSubqUncorrelInsert,
			"select":    selectEPL,
			"selectTwo": selectTwoEPL,
			"delete":    infraNWTableSubqUncorrelDelete,
		}
		deployCount := 0
		sendCounters := map[string]int{}
		for _, step := range steps[1:] {
			if step.Op == "case" {
				// The next case marker terminates this case span.
				break
			}
			switch step.Op {
			case "deploy":
				deployCount++
				epl, ok := deploys[step.Statement]
				if !ok || step.Epl != epl {
					return fmt.Errorf("%s case %q deploy %q is not pinned", infraNWTableSubqUncorrelID, caseName, step.Statement)
				}
			case "send":
				if _, err := decodeInfraNWTableSubqUncorrelPayload(step); err != nil {
					return fmt.Errorf("%s case %q: %w", infraNWTableSubqUncorrelID, caseName, err)
				}
				if err := validateInfraNWTableSubqUncorrelSendValues(caseName, step, sendCounters); err != nil {
					return fmt.Errorf("%s case %q: %w", infraNWTableSubqUncorrelID, caseName, err)
				}
			case "undeploy-all":
			default:
				return fmt.Errorf("%s case %q has unsupported op %q", infraNWTableSubqUncorrelID, caseName, step.Op)
			}
		}
		if deployCount != 5 {
			return fmt.Errorf("%s case %q must contain exactly five deploys, got %d", infraNWTableSubqUncorrelID, caseName, deployCount)
		}
		// advance past this case's steps
		consumed := 1
		for _, step := range steps[1:] {
			if step.Op == "case" {
				break
			}
			consumed++
		}
		offset += consumed
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableSubqUncorrelID)
	}
	return nil
}

func runInfraNWTableSubqUncorrelScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraNWTableSubqUncorrelScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableSubqUncorrelCases {
		// find this case's step span
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Case == caseName {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableSubqUncorrelCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableSubqUncorrelID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runInfraNWTableSubqUncorrelCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWTableSubqUncorrelBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqUncorrelMarket](env, "SupportMarketDataBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[infraNWTableSubqUncorrelA](env, "SupportBean_A"); err != nil {
		return compat.Trace{}, err
	}

	isTable := infraNWTableSubqUncorrelIsTable(caseName)
	// Infra must exist before the engine snapshots environment named windows.
	if isTable {
		if _, err := esper.CreateTable(env, "MyInfra", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("a"),
			esper.TableColumnOf[int64]("b"),
			esper.TableColumnOf[int64]("c"),
		}); err != nil {
			return compat.Trace{}, err
		}
	} else {
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return compat.Trace{}, fmt.Errorf("SupportBean schema is missing")
		}
		projection, err := esper.NewMapSchema("MyInfraSchema", []esper.FieldSpec{
			esper.FieldDef("a", reflect.TypeOf("")),
			esper.FieldDef("b", reflect.TypeOf(int64(0))),
			esper.FieldDef("c", reflect.TypeOf(int64(0))),
		})
		if err2 := env.RegisterSchema(projection); err2 != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyInfra", projection, esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
		_ = schema
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableSubqUncorrelJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := map[string]uint64{
		"create":    0,
		"select":    0,
		"selectTwo": 0,
		"delete":    0,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableSubqUncorrelID}
	record := func(statement string, batch esper.ResultBatch) {
		sequence[statement]++
		rec := compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: statement,
			Sequence:  sequence[statement],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		}
		if len(rec.Old) == 0 {
			rec.Old = nil
		}
		trace.Records = append(trace.Records, rec)
	}

	var deployments []*esper.Deployment
	var listened []*esper.Statement
	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plan esper.Plan
			var err error
			switch step.Statement {
			case "create":
				if isTable {
					// The table create is a catalog operation, already applied.
					var inner esper.RecordStream = esper.FromTable(env, "MyInfra")
					plan, err = env.Build(inner.Query(esper.StatementName("create")))
				} else {
					plan, err = env.Build(esper.FromNamedWindow(env, "MyInfra").
						Query(esper.StatementName("create"), esper.WithOldStream()))
				}
			case "insert":
				source := esper.From[infraNWTableSubqUncorrelBean](env, "SupportBean")
				assignments := []esper.TableAssignment{
					esper.SetColumn("a", esper.Field[infraNWTableSubqUncorrelBean, string]("theString")),
					esper.SetColumn("b", esper.Field[infraNWTableSubqUncorrelBean, int64]("longPrimitive")),
					esper.SetColumn("c", esper.Cast[*int64, int64](esper.Field[infraNWTableSubqUncorrelBean, *int64]("longBoxed"))),
				}
				if isTable {
					plan, err = env.Build(esper.OnEvent(source).InsertIntoTable("MyInfra", assignments...).Query(esper.StatementName("insert")))
				} else {
					plan, err = env.Build(esper.OnEvent(source).InsertIntoNamedWindow("MyInfra", assignments...).Query(esper.StatementName("insert")))
				}
			case "select":
				var inner esper.RecordStream
				if isTable {
					inner = esper.FromTable(env, "MyInfra")
				} else {
					inner = esper.FromNamedWindow(env, "MyInfra")
				}
				plan, err = env.Build(esper.Select(
					esper.From[infraNWTableSubqUncorrelMarket](env, "SupportMarketDataBean"),
					esper.Alias("value", esper.SubqueryValueWithOptions[string](inner,
						esper.Field[any, string]("a"),
						esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
					esper.Alias("symbol", esper.Field[infraNWTableSubqUncorrelMarket, string]("symbol")),
				).Query(esper.StatementName("select"), esper.WithOldStream()))
			case "selectTwo":
				var inner esper.RecordStream
				if isTable {
					inner = esper.FromTable(env, "MyInfra")
				} else {
					inner = esper.FromNamedWindow(env, "MyInfra")
				}
				plan, err = env.Build(esper.Select(
					esper.From[infraNWTableSubqUncorrelMarket](env, "SupportMarketDataBean"),
					esper.Alias("value", esper.SubqueryValueWithOptions[string](inner,
						esper.Field[any, string]("a"),
						esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))),
					esper.Alias("symbol", esper.Field[infraNWTableSubqUncorrelMarket, string]("symbol")),
				).Query(esper.StatementName("selectTwo"), esper.WithOldStream()))
			case "delete":
				source := esper.From[infraNWTableSubqUncorrelA](env, "SupportBean_A")
				if isTable {
					plan, err = env.Build(esper.OnEvent(source).DeleteFromTableWhere("MyInfra",
						esper.Equal[string](esper.TableField[string]("a"),
							esper.Field[infraNWTableSubqUncorrelA, string]("id"))).Query(esper.StatementName("delete")))
				} else {
					plan, err = env.Build(esper.OnEvent(source).DeleteFromNamedWindow("MyInfra",
						esper.Equal[string](esper.NamedWindowField[string]("a"),
							esper.Field[infraNWTableSubqUncorrelA, string]("id"))).Query(esper.StatementName("delete")))
				}
			default:
				return compat.Trace{}, fmt.Errorf("unexpected deploy statement %q", step.Statement)
			}
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			deployments = append(deployments, deployment)
			if step.Statement != "insert" {
				for _, statement := range deployment.Statements() {
					if statement.Name() == step.Statement {
						name := step.Statement
						listened = append(listened, statement)
						if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
							record(name, batch)
							return nil
						}); err != nil {
							return compat.Trace{}, err
						}
					}
				}
			}
		case "send":
			payload, err := decodeInfraNWTableSubqUncorrelPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
		}
	}
	for _, deployment := range deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return compat.Trace{}, err
		}
	}
	return trace, nil
}

func decodeInfraNWTableSubqUncorrelPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWTableSubqUncorrelFields(fields, "theString", "longPrimitive", "longBoxed"); err != nil {
			return nil, err
		}
		var bean infraNWTableSubqUncorrelBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWTableSubqUncorrelFields(fields, "symbol", "price", "volume", "feed"); err != nil {
			return nil, err
		}
		var market infraNWTableSubqUncorrelMarket
		if err := json.Unmarshal(step.Payload, &market); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return market, nil
	case "SupportBean_A":
		if err := requireInfraNWTableSubqUncorrelFields(fields, "id"); err != nil {
			return nil, err
		}
		var a infraNWTableSubqUncorrelA
		if err := json.Unmarshal(step.Payload, &a); err != nil {
			return nil, fmt.Errorf("decode SupportBean_A: %w", err)
		}
		return a, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", infraNWTableSubqUncorrelID, step.EventType)
	}
}

func requireInfraNWTableSubqUncorrelFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateInfraNWTableSubqUncorrelStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

// validateInfraNWTableSubqUncorrelSendValues pins every send payload so a
// drifted scenario cannot silently replay different market/bean values.
func validateInfraNWTableSubqUncorrelSendValues(caseName string, step compat.Step, counters map[string]int) error {
	kind := step.EventType
	defer func() { counters[kind]++ }()
	market := []string{"M1", "M1", "M2", "M3", "M4", "M5"}
	bean := []struct {
		theString            string
		longPrimitive, boxed int64
	}{{"S1", 1, 2}, {"S2", 10, 20}, {"S3", 100, 200}}
	switch step.EventType {
	case "SupportMarketDataBean":
		if counters[kind] >= len(market) {
			return fmt.Errorf("unexpected extra market send %d", counters[kind])
		}
		var payload struct {
			Symbol string
			Price  float64
			Volume *int64
			Feed   string
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		if payload.Symbol != market[counters[kind]] || payload.Price != 0 || payload.Volume != nil || payload.Feed != "" {
			return fmt.Errorf("market payload %d is not pinned", counters[kind])
		}
	case "SupportBean":
		if counters[kind] >= len(bean) {
			return fmt.Errorf("unexpected extra bean send %d", counters[kind])
		}
		var payload struct {
			TheString     string
			LongPrimitive int64
			LongBoxed     *int64
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		want := bean[counters[kind]]
		if payload.TheString != want.theString || payload.LongPrimitive != want.longPrimitive ||
			payload.LongBoxed == nil || *payload.LongBoxed != want.boxed {
			return fmt.Errorf("bean payload %d is not pinned", counters[kind])
		}
	case "SupportBean_A":
		want := map[int]string{0: "S1", 1: "S2"}
		var payload struct {
			ID string
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return err
		}
		if payload.ID != want[counters[kind]] {
			return fmt.Errorf("A payload %d is not pinned", counters[kind])
		}
	}
	return nil
}
