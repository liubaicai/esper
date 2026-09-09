package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	eplSubselectOrderOfEvalIndexID          = "epl-subselect-order-of-eval-index"
	eplSubselectOrderOfEvalIndexDescription = "EPLSubselectOrderOfEval correlated subquery window ordering plus subquery-first order-of-evaluation silence, with EPLSubselectIndex subquery index choices over an overdefined where clause and unique/firstunique/time/groupwin correlated subquery indexes (second oracle source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectIndex.java; third oracle source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectOrderOfEvalNoPreeval.java)."
	eplSubselectOrderOfEvalIndexJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	eplSubselectOrderOfEvalIndexSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectOrderOfEval.java"
)

var (
	eplSubselectOrderOfEvalIndexJavaSources = []string{
		eplSubselectOrderOfEvalIndexSource,
	}
	eplSubselectOrderOfEvalIndexJavaRuntimeIDs = []string{
		"java-runtime-3dbb926e23a4521d64d5",
		"java-runtime-9a68943733f98a1ea1dd",
		"java-runtime-f220d864166a5650e6f7",
		"java-runtime-0fbd10b1080bb8e7afa3",
		"java-runtime-ac2d59129befe9a3b088",
	}
	eplSubselectOrderOfEvalIndexJavaExecutions = []string{
		"EPLSubselectCorrelatedSubqueryOrder",
		"EPLSubselectOrderOfEvaluationSubselectFirst",
		"EPLSubselectIndexChoicesOverdefinedWhere",
		"EPLSubselectUniqueIndexCorrelated",
		"EPLSubselectOrderOfEvalNoPreeval",
	}
	eplSubselectOrderOfEvalIndexJavaStaticIDs = []string{
		"java-2ddf3ed58c64fe3674f7",
		"java-eaa8e4f13676cc852f92",
		"java-29ec8e7d3c6e96c8d5aa",
		"java-77258a6e645d2449b31c",
		"java-04d47cea26b120f5f805",
	}
	eplSubselectOrderOfEvalIndexCases = []string{
		"correlated-subquery-order",
		"order-of-eval-subselect-first",
		"index-choices-overdefined-where",
		"unique-index-correlated",
		"order-of-eval-no-preeval",
	}
	eplSubselectOrderOfEvalIndexOrdinals = []int{0, 1, 0, 1, 0}
)

type eplSubselectOrderOfEvalIndexTrade struct {
	Time       int64   `esper:"time"`
	SecurityID int     `esper:"securityID"`
	Price      float64 `esper:"price"`
	Volume     int64   `esper:"volume"`
}

type eplSubselectOrderOfEvalIndexSimpleOne struct {
	S1 string  `esper:"s1"`
	I1 int     `esper:"i1"`
	D1 float64 `esper:"d1"`
	L1 int64   `esper:"l1"`
}

type eplSubselectOrderOfEvalIndexSimpleTwo struct {
	S2 string  `esper:"s2"`
	I2 int     `esper:"i2"`
	D2 float64 `esper:"d2"`
	L2 int64   `esper:"l2"`
}

type eplSubselectOrderOfEvalIndexBean struct {
	TheString     string `esper:"theString"`
	IntPrimitive  int    `esper:"intPrimitive"`
	LongPrimitive *int64 `esper:"longPrimitive"`
}

// eplSubselectOrderOfEvalIndexFullBean mirrors the real SupportBean surface
// for the no-preeval case: the Java execution projects select * over the
// bean class, so the Go trace renders the full 20-property row. Boxed,
// decimal, and enum columns are pointers so a plain send decodes to Java's
// nulls, and charPrimitive mirrors the Java char default via the decode.
type eplSubselectOrderOfEvalIndexFullBean struct {
	TheString       string   `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int      `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int     `esper:"intBoxed"`
	LongBoxed       *int64   `esper:"longBoxed"`
	CharBoxed       *string  `esper:"charBoxed"`
	ShortBoxed      *int16   `esper:"shortBoxed"`
	ByteBoxed       *int8    `esper:"byteBoxed"`
	FloatBoxed      *float32 `esper:"floatBoxed"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
	BigDecimal      *big.Rat `esper:"bigDecimal"`
	BigInteger      *big.Int `esper:"bigInteger"`
	EnumValue       *string  `esper:"enumValue"`
}

type eplSubselectOrderOfEvalIndexS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

func loadEplSubselectOrderOfEvalIndexScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplSubselectOrderOfEvalIndexID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplSubselectOrderOfEvalIndexID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplSubselectOrderOfEvalIndexID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplSubselectOrderOfEvalIndexID, err)
	}
	if err := requireEplSubselectOrderOfEvalIndexFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplSubselectOrderOfEvalIndexID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplSubselectOrderOfEvalIndexID ||
		metadata.Description != eplSubselectOrderOfEvalIndexDescription ||
		metadata.JavaCommit != eplSubselectOrderOfEvalIndexJavaCommit ||
		metadata.JavaSource != eplSubselectOrderOfEvalIndexSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplSubselectOrderOfEvalIndexID)
	}
	if err := validateEplSubselectOrderOfEvalIndexStringArray(root["javaRuntimes"], eplSubselectOrderOfEvalIndexJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectOrderOfEvalIndexStringArray(root["javaNames"], eplSubselectOrderOfEvalIndexJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectOrderOfEvalIndexStringArray(root["javaStaticIds"], eplSubselectOrderOfEvalIndexJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEplSubselectOrderOfEvalIndexStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplSubselectOrderOfEvalIndexCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly five cases", eplSubselectOrderOfEvalIndexID)
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEplSubselectOrderOfEvalIndexFields(object,
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
		if definition.Case != eplSubselectOrderOfEvalIndexCases[index] ||
			definition.Ordinal != eplSubselectOrderOfEvalIndexOrdinals[index] ||
			definition.RuntimeID != eplSubselectOrderOfEvalIndexJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplSubselectOrderOfEvalIndexJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplSubselectOrderOfEvalIndexID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", eplSubselectOrderOfEvalIndexID)
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
			if err := requireEplSubselectOrderOfEvalIndexFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireEplSubselectOrderOfEvalIndexFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireEplSubselectOrderOfEvalIndexFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeEplSubselectOrderOfEvalIndexPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireEplSubselectOrderOfEvalIndexFields(object, "op", "case"); err != nil {
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
	if err := validateEplSubselectOrderOfEvalIndexScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateEplSubselectOrderOfEvalIndexScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != eplSubselectOrderOfEvalIndexID {
		return fmt.Errorf("%s scenario shape is not pinned", eplSubselectOrderOfEvalIndexID)
	}
	offset := 0
	expectations := []struct {
		caseName string
		cycles   int
		sends    []int
	}{
		{caseName: eplSubselectOrderOfEvalIndexCases[0], cycles: 1, sends: []int{2}},
		{caseName: eplSubselectOrderOfEvalIndexCases[1], cycles: 2, sends: []int{1, 1}},
		{caseName: eplSubselectOrderOfEvalIndexCases[2], cycles: 19, sends: eplSubselectOrderOfEvalIndexCycleSendCounts()},
		{caseName: eplSubselectOrderOfEvalIndexCases[3], cycles: 4, sends: []int{6, 6, 6, 4}},
		{caseName: eplSubselectOrderOfEvalIndexCases[4], cycles: 2, sends: []int{1, 1}},
	}
	for _, expectation := range expectations {
		steps := scenario.Steps[offset:]
		total := 1 + expectation.cycles*(2) + sumEplSubselectOrderOfEvalIndex(expectation.sends)
		if len(steps) < total {
			return fmt.Errorf("%s case %q steps are truncated", eplSubselectOrderOfEvalIndexID, expectation.caseName)
		}
		if steps[0].Op != "case" || steps[0].Case != expectation.caseName {
			return fmt.Errorf("%s case %q must start with its case marker", eplSubselectOrderOfEvalIndexID, expectation.caseName)
		}
		cursor := 1
		for cycle := 0; cycle < expectation.cycles; cycle++ {
			step := steps[cursor]
			cursor++
			if step.Op != "deploy" || step.Case != expectation.caseName || step.Statement != "s0" {
				return fmt.Errorf("%s case %q cycle %d must deploy s0", eplSubselectOrderOfEvalIndexID, expectation.caseName, cycle)
			}
			for send := 0; send < expectation.sends[cycle]; send++ {
				sendStep := steps[cursor]
				cursor++
				if sendStep.Op != "send" || sendStep.Case != expectation.caseName {
					return fmt.Errorf("%s case %q cycle %d must contain sends", eplSubselectOrderOfEvalIndexID, expectation.caseName, cycle)
				}
				if _, err := decodeEplSubselectOrderOfEvalIndexPayload(sendStep); err != nil {
					return fmt.Errorf("%s case %q cycle %d: %w", eplSubselectOrderOfEvalIndexID, expectation.caseName, cycle, err)
				}
			}
			undeploy := steps[cursor]
			cursor++
			if undeploy.Op != "undeploy-all" || undeploy.Case != expectation.caseName {
				return fmt.Errorf("%s case %q cycle %d must end with undeploy-all", eplSubselectOrderOfEvalIndexID, expectation.caseName, cycle)
			}
		}
		offset += total
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", eplSubselectOrderOfEvalIndexID)
	}
	return nil
}

func eplSubselectOrderOfEvalIndexCycleSendCounts() []int {
	counts := make([]int, 19)
	for index := range counts {
		counts[index] = 4
		if index >= 15 {
			counts[index] = 7
		}
	}
	return counts
}

func sumEplSubselectOrderOfEvalIndex(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func runEplSubselectOrderOfEvalIndexScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateEplSubselectOrderOfEvalIndexScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	expectations := []struct {
		caseName string
		cycles   int
		sends    []int
	}{
		{caseName: eplSubselectOrderOfEvalIndexCases[0], cycles: 1, sends: []int{2}},
		{caseName: eplSubselectOrderOfEvalIndexCases[1], cycles: 2, sends: []int{1, 1}},
		{caseName: eplSubselectOrderOfEvalIndexCases[2], cycles: 19, sends: eplSubselectOrderOfEvalIndexCycleSendCounts()},
		{caseName: eplSubselectOrderOfEvalIndexCases[3], cycles: 4, sends: []int{6, 6, 6, 4}},
		{caseName: eplSubselectOrderOfEvalIndexCases[4], cycles: 2, sends: []int{1, 1}},
	}
	for caseIndex, expectation := range expectations {
		total := 1 + expectation.cycles*2 + sumEplSubselectOrderOfEvalIndex(expectation.sends)
		caseSteps := scenario.Steps[offset : offset+total]
		offset += total
		caseTrace, err := runEplSubselectOrderOfEvalIndexCase(ctx, caseSteps, expectation, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplSubselectOrderOfEvalIndexID, expectation.caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runEplSubselectOrderOfEvalIndexCase(ctx context.Context, steps []compat.Step, expectation struct {
	caseName string
	cycles   int
	sends    []int
}, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if caseIndex == 4 {
		// The no-preeval statements project select * over the bean class;
		// register the full SupportBean surface so the row shape matches.
		if _, err := esper.RegisterStruct[eplSubselectOrderOfEvalIndexFullBean](env, "SupportBean"); err != nil {
			return compat.Trace{}, err
		}
	} else if _, err := esper.RegisterStruct[eplSubselectOrderOfEvalIndexBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectOrderOfEvalIndexS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectOrderOfEvalIndexTrade](env, "SupportTradeEventTwo"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectOrderOfEvalIndexSimpleOne](env, "SupportSimpleBeanOne"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplSubselectOrderOfEvalIndexSimpleTwo](env, "SupportSimpleBeanTwo"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplSubselectOrderOfEvalIndexJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	sequence := uint64(0)
	trace := compat.Trace{Version: "esper-parity/v1", ID: eplSubselectOrderOfEvalIndexID}
	record := func(batch esper.ResultBatch) {
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      expectation.caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}
	buildS0 := func(cycle int) (esper.Query, error) {
		return eplSubselectOrderOfEvalIndexQuery(env, expectation.caseName, cycle)
	}

	deployments := make([]*esper.Deployment, 0, 2)
	cursor := 1
	for cycle := 0; cycle < expectation.cycles; cycle++ {
		if step := steps[cursor]; step.Op != "deploy" || step.Statement != "s0" {
			return compat.Trace{}, fmt.Errorf("cycle %d must deploy s0", cycle)
		}
		cursor++
		query, err := buildS0(cycle)
		if err != nil {
			return compat.Trace{}, err
		}
		plan, err := env.Build(query)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("cycle %d build: %w", cycle, err)
		}
		// The correlated-subquery-order module also carries the unheard
		// seed statement (select * from SupportTradeEventTwo#lastevent).
		if expectation.caseName == eplSubselectOrderOfEvalIndexCases[0] {
			seedPlan, err := env.Build(esper.From[eplSubselectOrderOfEvalIndexTrade](env, "SupportTradeEventTwo").
				Window(esper.LastEvent()).Query(esper.StatementName("seed")))
			if err != nil {
				return compat.Trace{}, err
			}
			seedDeployment, err := engine.Deploy(ctx, seedPlan)
			if err != nil {
				return compat.Trace{}, err
			}
			deployments = append(deployments, seedDeployment)
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("cycle %d deploy: %w", cycle, err)
		}
		statements := deployment.Statements()
		if len(statements) != 1 || statements[0].Name() != "s0" {
			return compat.Trace{}, fmt.Errorf("cycle %d deployed unexpected statements", cycle)
		}
		if _, err := statements[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			record(batch)
			return nil
		}); err != nil {
			return compat.Trace{}, err
		}
		deployments = append(deployments, deployment)
		for send := 0; send < expectation.sends[cycle]; send++ {
			step := steps[cursor]
			cursor++
			var payload any
			if caseIndex == 4 {
				payload, err = decodeEplSubselectOrderOfEvalIndexNoPreevalPayload(step)
			} else {
				payload, err = decodeEplSubselectOrderOfEvalIndexPayload(step)
			}
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		}
		if step := steps[cursor]; step.Op != "undeploy-all" {
			return compat.Trace{}, fmt.Errorf("cycle %d must end with undeploy-all", cycle)
		}
		cursor++
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, err
			}
		}
		deployments = deployments[:0]
	}
	return trace, nil
}

// eplSubselectOrderOfEvalIndexQuery builds the s0 statement for one case
// cycle. Cycle semantics: case 0 uses window aggregates over a correlated
// time-window subselect; case 1 exercises preeval-on not-in filtering; case
// 2 encodes the 19 index-choice where-shape matrix (the unique-field lists
// and hints select plan-side backing only and are not representable in the
// typed trace contract); case 3 covers unique/firstunique/time/groupwin
// correlated scalar subqueries.
func eplSubselectOrderOfEvalIndexQuery(env *esper.Environment, caseName string, cycle int) (esper.Query, error) {
	switch caseName {
	case eplSubselectOrderOfEvalIndexCases[0]:
		securityID := esper.Field[eplSubselectOrderOfEvalIndexTrade, int]("securityID")
		inner := esper.From[eplSubselectOrderOfEvalIndexTrade](env, "SupportTradeEventTwo").
			Window(esper.TimeWindow(20 * time.Minute)).AsRecord()
		shortItems := esper.SubqueryValueWithOptions[[]esper.Event](inner,
			esper.WindowEvents(),
			esper.SubqueryWhere(esper.Equal[int](
				esper.Field[any, int]("securityID"),
				esper.OuterField[int]("securityID"))))
		return esper.From[eplSubselectOrderOfEvalIndexTrade](env, "SupportTradeEventTwo").
			Filter(esper.Equal[int](securityID, esper.Literal(1000))).
			Window(esper.TimeWindow(20*time.Minute)).
			GroupBy(securityID).
			Select(
				esper.Alias("longItems", esper.WindowEvents()),
				esper.Alias("shortItems", shortItems),
			).Query(esper.StatementName("s0")), nil
	case eplSubselectOrderOfEvalIndexCases[1]:
		intPrimitive := esper.Field[eplSubselectOrderOfEvalIndexBean, int]("intPrimitive")
		if cycle == 0 {
			inner := esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
				Window(esper.Unique(intPrimitive)).AsRecord()
			return esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
				Filter(esper.Less[int](intPrimitive, esper.Literal(10))).
				Filter(esper.Not(esper.SubqueryIn[int](intPrimitive, inner, intPrimitive))).
				Query(esper.StatementName("s0")), nil
		}
		inner := esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
			Filter(esper.Less[int](intPrimitive, esper.Literal(10))).
			Window(esper.Unique(intPrimitive)).AsRecord()
		return esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
			Filter(esper.Not(esper.SubqueryIn[int](intPrimitive, inner, intPrimitive))).
			Query(esper.StatementName("s0")), nil
	case eplSubselectOrderOfEvalIndexCases[4]:
		// EPLSubselectOrderOfEvalNoPreeval: the same two statements as the
		// preeval-on case, built over the full SupportBean surface with
		// WithSelfSubselectPreeval(false) so the not-in filter observes the
		// pre-arrival subselect window and the first E1/5 emits (the deferred
		// acceptance ingests it afterwards).
		intPrimitive := esper.Field[eplSubselectOrderOfEvalIndexFullBean, int]("intPrimitive")
		if cycle == 0 {
			inner := esper.From[eplSubselectOrderOfEvalIndexFullBean](env, "SupportBean").
				Window(esper.Unique(intPrimitive)).AsRecord()
			return esper.From[eplSubselectOrderOfEvalIndexFullBean](env, "SupportBean").
				Filter(esper.Less[int](intPrimitive, esper.Literal(10))).
				Filter(esper.Not(esper.SubqueryIn[int](intPrimitive, inner, intPrimitive))).
				Query(esper.StatementName("s0"), esper.WithSelfSubselectPreeval(false)), nil
		}
		inner := esper.From[eplSubselectOrderOfEvalIndexFullBean](env, "SupportBean").
			Filter(esper.Less[int](intPrimitive, esper.Literal(10))).
			Window(esper.Unique(intPrimitive)).AsRecord()
		return esper.From[eplSubselectOrderOfEvalIndexFullBean](env, "SupportBean").
			Filter(esper.Not(esper.SubqueryIn[int](intPrimitive, inner, intPrimitive))).
			Query(esper.StatementName("s0"), esper.WithSelfSubselectPreeval(false)), nil
	case eplSubselectOrderOfEvalIndexCases[2]:
		if cycle < 0 || cycle >= 19 {
			return esper.Query{}, fmt.Errorf("index-choice cycle %d out of range", cycle)
		}
		// Each cycle pins Esper's #unique(<fields>) data window: the window
		// decides which rows survive for the subquery, not just the plan.
		uniqueKeys := [][]esper.Expr{
			[]esper.Expr{esper.Field[any, string]("s2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, int]("i2"), esper.Field[any, float64]("d2"), esper.Field[any, int64]("l2")},
			[]esper.Expr{esper.Field[any, int]("i2"), esper.Field[any, float64]("d2"), esper.Field[any, int64]("l2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int64]("l2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, float64]("d2"), esper.Field[any, int64]("l2"), esper.Field[any, int]("i2")},
			[]esper.Expr{esper.Field[any, int64]("l2")},
			[]esper.Expr{esper.Field[any, int64]("l2")},
			[]esper.Expr{esper.Field[any, int64]("l2")},
			[]esper.Expr{esper.Field[any, string]("s2")},
			[]esper.Expr{esper.Field[any, string]("s2")},
			[]esper.Expr{esper.Field[any, string]("s2")},
			[]esper.Expr{esper.Field[any, string]("s2")},
		}[cycle]
		inner := esper.From[eplSubselectOrderOfEvalIndexSimpleTwo](env, "SupportSimpleBeanTwo").
			Window(esper.UniqueBy(uniqueKeys...)).AsRecord()
		innerI2 := esper.Field[any, int]("i2")
		innerD2 := esper.Field[any, float64]("d2")
		innerL2 := esper.Field[any, int64]("l2")
		innerS2 := esper.Field[any, string]("s2")
		outerI1 := esper.OuterField[int]("i1")
		outerD1 := esper.OuterField[float64]("d1")
		outerL1 := esper.OuterField[int64]("l1")
		var predicate esper.Expression[bool]
		switch cycle {
		case 0:
			predicate = nil
		case 1:
			predicate = esper.And(esper.Equal[int](innerI2, outerI1), esper.Equal[float64](innerD2, outerD1))
		case 2:
			predicate = esper.And(esper.Equal[float64](innerD2, outerD1), esper.Equal[int](innerI2, outerI1))
		case 3:
			predicate = esper.And(
				esper.And(esper.Equal[int64](innerL2, outerL1), esper.Equal[float64](innerD2, outerD1)),
				esper.Equal[int](innerI2, outerI1))
		case 4:
			predicate = esper.And(esper.Equal[int64](innerL2, outerL1), esper.Equal[int](innerI2, outerI1))
		case 5:
			predicate = esper.Equal[float64](innerD2, outerD1)
		case 6:
			predicate = esper.And(
				esper.And(esper.Equal[int](innerI2, outerI1), esper.Equal[float64](innerD2, outerD1)),
				esper.Between[int64](innerL2, esper.Literal(int64(1)), esper.Literal(int64(1000))))
		case 7:
			predicate = esper.And(esper.Equal[float64](innerD2, outerD1),
				esper.Between[int64](innerL2, esper.Literal(int64(1)), esper.Literal(int64(1000))))
		case 8:
			predicate = esper.And(esper.Equal[int64](innerL2, outerL1), esper.Equal[float64](innerD2, outerD1))
		case 9:
			predicate = esper.And(
				esper.And(esper.Equal[int64](innerL2, outerL1), esper.Equal[int](innerI2, outerI1)),
				esper.Equal[float64](innerD2, outerD1))
		case 10:
			predicate = esper.And(
				esper.And(esper.Equal[int64](innerL2, outerL1), esper.Equal[int](innerI2, outerI1)),
				esper.Equal[float64](innerD2, outerD1))
		case 11:
			predicate = esper.And(
				esper.And(
					esper.And(esper.Equal[int64](innerL2, outerL1), esper.Equal[int](innerI2, outerI1)),
					esper.Equal[float64](innerD2, outerD1)),
				esper.Between[string](innerS2, esper.Literal("E3"), esper.Literal("E4")))
		case 12, 13:
			predicate = esper.Equal[int64](innerL2, outerL1)
		case 14:
			predicate = esper.And(esper.Equal[int64](innerL2, outerL1),
				esper.Between[int](outerI1, esper.Literal(1), esper.Literal(20)))
		case 15:
			predicate = esper.Greater[int](outerI1, innerI2)
		case 16:
			predicate = esper.GreaterOrEqual[int](outerI1, innerI2)
		case 17:
			predicate = esper.Less[int](outerI1, innerI2)
		default:
			predicate = esper.LessOrEqual[int](outerI1, innerI2)
		}
		outer := esper.From[eplSubselectOrderOfEvalIndexSimpleOne](env, "SupportSimpleBeanOne")
		// Java scalar subqueries return null when more than one row matches.
		var projection esper.Expression[string]
		if predicate != nil {
			projection = esper.SubqueryValueWithOptions[string](inner, innerS2,
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple),
				esper.SubqueryWhere(predicate))
		} else {
			projection = esper.SubqueryValueWithOptions[string](inner, innerS2,
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple))
		}
		return esper.Select(outer,
			esper.Alias("c0", esper.Field[eplSubselectOrderOfEvalIndexSimpleOne, string]("s1")),
			esper.Alias("c1", projection),
		).Query(esper.StatementName("s0")), nil
	case eplSubselectOrderOfEvalIndexCases[3]:
		innerKey := esper.Field[any, string]("theString")
		outerP00 := esper.OuterField[string]("p00")
		keyPredicate := esper.Equal[string](innerKey, outerP00)
		var inner esper.RecordStream
		projection := esper.Field[any, int]("intPrimitive")
		switch cycle {
		case 0:
			inner = esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
				Window(esper.Unique(esper.Field[eplSubselectOrderOfEvalIndexBean, string]("theString"))).AsRecord()
		case 1:
			inner = esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
				Window(esper.FirstUnique(esper.Field[eplSubselectOrderOfEvalIndexBean, string]("theString"))).AsRecord()
		case 2:
			inner = esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
				Window(esper.TimeWindow(time.Second)).
				Window(esper.Unique(esper.Field[eplSubselectOrderOfEvalIndexBean, string]("theString"))).AsRecord()
		default:
			inner = esper.From[eplSubselectOrderOfEvalIndexBean](env, "SupportBean").
				Window(esper.GroupWindow(
					esper.Field[eplSubselectOrderOfEvalIndexBean, string]("theString"),
					esper.Unique(esper.Field[eplSubselectOrderOfEvalIndexBean, int]("intPrimitive")))).AsRecord()
			projection = esper.Field[any, int64]("longPrimitive")
			keyPredicate = esper.And(keyPredicate, esper.Equal[int](
				esper.Field[any, int]("intPrimitive"),
				esper.OuterField[int]("id")))
		}
		return esper.Select(esper.From[eplSubselectOrderOfEvalIndexS0](env, "SupportBean_S0"),
			esper.Alias("c0", esper.Field[eplSubselectOrderOfEvalIndexS0, int]("id")),
			esper.Alias("c1", esper.SubqueryValueWithOptions[int](inner, projection,
				esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple),
				esper.SubqueryWhere(keyPredicate))),
		).Query(esper.StatementName("s0")), nil
	}
	return esper.Query{}, fmt.Errorf("unknown %s case %q", eplSubselectOrderOfEvalIndexID, caseName)
}

func decodeEplSubselectOrderOfEvalIndexPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if len(fields) == 2 {
			if err := requireEplSubselectOrderOfEvalIndexFields(fields, "theString", "intPrimitive"); err != nil {
				return nil, err
			}
		} else {
			if err := requireEplSubselectOrderOfEvalIndexFields(fields, "theString", "intPrimitive", "longPrimitive"); err != nil {
				return nil, err
			}
		}
		var bean eplSubselectOrderOfEvalIndexBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		if err := requireEplSubselectOrderOfEvalIndexFields(fields, "id", "p00"); err != nil {
			return nil, err
		}
		var s0 eplSubselectOrderOfEvalIndexS0
		if err := json.Unmarshal(step.Payload, &s0); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return s0, nil
	case "SupportTradeEventTwo":
		if err := requireEplSubselectOrderOfEvalIndexFields(fields, "time", "securityID", "price", "volume"); err != nil {
			return nil, err
		}
		var trade eplSubselectOrderOfEvalIndexTrade
		if err := json.Unmarshal(step.Payload, &trade); err != nil {
			return nil, fmt.Errorf("decode SupportTradeEventTwo: %w", err)
		}
		return trade, nil
	case "SupportSimpleBeanOne":
		if len(fields) == 2 {
			if err := requireEplSubselectOrderOfEvalIndexFields(fields, "s1", "i1"); err != nil {
				return nil, err
			}
		} else {
			if err := requireEplSubselectOrderOfEvalIndexFields(fields, "s1", "i1", "d1", "l1"); err != nil {
				return nil, err
			}
		}
		var one eplSubselectOrderOfEvalIndexSimpleOne
		if err := json.Unmarshal(step.Payload, &one); err != nil {
			return nil, fmt.Errorf("decode SupportSimpleBeanOne: %w", err)
		}
		return one, nil
	case "SupportSimpleBeanTwo":
		if len(fields) == 2 {
			if err := requireEplSubselectOrderOfEvalIndexFields(fields, "s2", "i2"); err != nil {
				return nil, err
			}
		} else {
			if err := requireEplSubselectOrderOfEvalIndexFields(fields, "s2", "i2", "d2", "l2"); err != nil {
				return nil, err
			}
		}
		var two eplSubselectOrderOfEvalIndexSimpleTwo
		if err := json.Unmarshal(step.Payload, &two); err != nil {
			return nil, fmt.Errorf("decode SupportSimpleBeanTwo: %w", err)
		}
		return two, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", eplSubselectOrderOfEvalIndexID, step.EventType)
	}
}
func requireEplSubselectOrderOfEvalIndexFields(object map[string]json.RawMessage, names ...string) error {
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

func validateEplSubselectOrderOfEvalIndexStringArray(raw json.RawMessage, expected []string, name string) error {
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

// decodeEplSubselectOrderOfEvalIndexNoPreevalPayload decodes sends for the
// no-preeval case. charPrimitive mirrors Java's char default: an absent
// payload field decodes to the NUL character the Java bean carries.
func decodeEplSubselectOrderOfEvalIndexNoPreevalPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value eplSubselectOrderOfEvalIndexFullBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		if value.CharPrimitive == "" {
			value.CharPrimitive = "\x00"
		}
		return value, nil
	default:
		return decodeEplSubselectOrderOfEvalIndexPayload(step)
	}
}
