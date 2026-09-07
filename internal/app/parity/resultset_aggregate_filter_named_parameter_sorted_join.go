package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	resultsetAggregateFilterNamedParameterSortedJoinID          = "resultset-aggregate-filter-named-parameter-sorted-join"
	resultsetAggregateFilterNamedParameterSortedJoinDescription = "ResultSetAggregateFilterNamedParameter sorted-aggregate join executions: maxby/minby/maxbyever/minbyever with theString value projections and sorted event-array columns under named filter parameters over a last-event join with length(4) or keepall SupportBean windows, plus two-key sorted(intPrimitive, doublePrimitive) multicriteria ordering with null rendered for empty filtered sets (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java)."
	resultsetAggregateFilterNamedParameterSortedJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateFilterNamedParameterSortedJoinSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"

	// EPL pins are the exact Java-source concatenations (no spaces between
	// the comma-separated select expressions).
	resultsetAggregateFilterNamedParameterSortedJoinBoundEPL   = "@name('s0') select maxby(intPrimitive, filter:theString like 'A%').theString as aMaxby,minby(intPrimitive, filter:theString like 'A%').theString as aMinby,sorted(intPrimitive, filter:theString like 'A%') as aSorted,maxby(intPrimitive, filter:theString like 'B%').theString as bMaxby,minby(intPrimitive, filter:theString like 'B%').theString as bMinby,sorted(intPrimitive, filter:theString like 'B%') as bSorted from SupportBean_S1#lastevent, SupportBean#length(4)"
	resultsetAggregateFilterNamedParameterSortedJoinUnboundEPL = "@name('s0') select maxby(intPrimitive, filter:theString like 'A%').theString as aMaxby,maxbyever(intPrimitive, filter:theString like 'A%').theString as aMaxbyever,minby(intPrimitive, filter:theString like 'A%').theString as aMinby,minbyever(intPrimitive, filter:theString like 'A%').theString as aMinbyever from SupportBean_S1#lastevent, SupportBean#keepall"
	resultsetAggregateFilterNamedParameterSortedJoinMultiEPL   = "@name('s0') select sorted(intPrimitive, doublePrimitive, filter:theString like 'A%') as aSorted,sorted(intPrimitive, doublePrimitive, filter:theString like 'B%') as bSorted from SupportBean#keepall"
)

var (
	resultsetAggregateFilterNamedParameterSortedJoinJavaSources = []string{
		resultsetAggregateFilterNamedParameterSortedJoinSource,
	}
	resultsetAggregateFilterNamedParameterSortedJoinJavaRuntimeIDs = []string{
		"java-runtime-6b6e0d2290261cb8cd93",
		"java-runtime-236d99a9d77ed3932510",
		"java-runtime-398a780be4d755650285",
	}
	resultsetAggregateFilterNamedParameterSortedJoinJavaExecutions = []string{
		"ResultSetAggregateAccessAggSortedBound{join=true}",
		"ResultSetAggregateAccessAggSortedUnbound{join=true}",
		"ResultSetAggregateAccessAggSortedMulticriteria",
	}
	resultsetAggregateFilterNamedParameterSortedJoinJavaStaticIDs = []string{
		"java-ea6830fd215ba36a098b",
		"java-d726134be3446919675e",
		"java-a26ba8e7bb8f4e6c9f2d",
	}
	resultsetAggregateFilterNamedParameterSortedJoinCases = []string{
		"sorted-join-bound",
		"sorted-join-unbound",
		"sorted-multicriteria",
	}
	resultsetAggregateFilterNamedParameterSortedJoinOrdinals = []int{15, 17, 18}
)

type resultsetAggregateFilterNamedParameterSortedJoinBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

type resultsetAggregateFilterNamedParameterSortedJoinS1 struct {
	ID              int     `esper:"id"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

func loadResultsetAggregateFilterNamedParameterSortedJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateFilterNamedParameterSortedJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateFilterNamedParameterSortedJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilterNamedParameterSortedJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilterNamedParameterSortedJoinID, err)
	}
	if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetAggregateFilterNamedParameterSortedJoinID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetAggregateFilterNamedParameterSortedJoinID ||
		metadata.Description != resultsetAggregateFilterNamedParameterSortedJoinDescription ||
		metadata.JavaCommit != resultsetAggregateFilterNamedParameterSortedJoinJavaCommit ||
		metadata.JavaSource != resultsetAggregateFilterNamedParameterSortedJoinSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateFilterNamedParameterSortedJoinID)
	}
	if err := validateResultsetAggregateFilterNamedParameterSortedJoinStringArray(root["javaRuntimes"], resultsetAggregateFilterNamedParameterSortedJoinJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetAggregateFilterNamedParameterSortedJoinStringArray(root["javaNames"], resultsetAggregateFilterNamedParameterSortedJoinJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetAggregateFilterNamedParameterSortedJoinStringArray(root["javaStaticIds"], resultsetAggregateFilterNamedParameterSortedJoinJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetAggregateFilterNamedParameterSortedJoinStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetAggregateFilterNamedParameterSortedJoinCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetAggregateFilterNamedParameterSortedJoinID)
	}
	observedEPL := map[string]string{
		"sorted-join-bound":    resultsetAggregateFilterNamedParameterSortedJoinBoundEPL,
		"sorted-join-unbound":  resultsetAggregateFilterNamedParameterSortedJoinUnboundEPL,
		"sorted-multicriteria": resultsetAggregateFilterNamedParameterSortedJoinMultiEPL,
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(object,
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
		if definition.Case != resultsetAggregateFilterNamedParameterSortedJoinCases[index] ||
			definition.Ordinal != resultsetAggregateFilterNamedParameterSortedJoinOrdinals[index] ||
			definition.RuntimeID != resultsetAggregateFilterNamedParameterSortedJoinJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetAggregateFilterNamedParameterSortedJoinJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != observedEPL[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateFilterNamedParameterSortedJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", resultsetAggregateFilterNamedParameterSortedJoinID)
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
			if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetAggregateFilterNamedParameterSortedJoinPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(object, "op", "case"); err != nil {
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
	if err := validateResultsetAggregateFilterNamedParameterSortedJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetAggregateFilterNamedParameterSortedJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateFilterNamedParameterSortedJoinID {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetAggregateFilterNamedParameterSortedJoinID)
	}
	deployEPLs := map[string]string{
		"sorted-join-bound":    resultsetAggregateFilterNamedParameterSortedJoinBoundEPL,
		"sorted-join-unbound":  resultsetAggregateFilterNamedParameterSortedJoinUnboundEPL,
		"sorted-multicriteria": resultsetAggregateFilterNamedParameterSortedJoinMultiEPL,
	}
	// Send tables: per case, S1 kickoff first (join cases only), then the
	// Java send order. The 2-arg Java sendEvent defaults doublePrimitive to
	// -1 in the join cases; ord 18 pins explicit doubles.
	caseSends := map[string][]compat.Step{
		"sorted-join-bound": {
			{EventType: "SupportBean_S1", Payload: json.RawMessage(`{"id": 0, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B1", "intPrimitive": 1, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A10", "intPrimitive": 10, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B2", "intPrimitive": 2, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A5", "intPrimitive": 5, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A15", "intPrimitive": 15, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X3", "intPrimitive": 3, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X4", "intPrimitive": 4, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X5", "intPrimitive": 5, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X6", "intPrimitive": 6, "doublePrimitive": -1}`)},
		},
		"sorted-join-unbound": {
			{EventType: "SupportBean_S1", Payload: json.RawMessage(`{"id": 0, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B1", "intPrimitive": 1, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A10", "intPrimitive": 10, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A5", "intPrimitive": 5, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A15", "intPrimitive": 15, "doublePrimitive": -1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B1000", "intPrimitive": 1000, "doublePrimitive": -1}`)},
		},
		"sorted-multicriteria": {
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B1", "intPrimitive": 1, "doublePrimitive": 10}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A1", "intPrimitive": 100, "doublePrimitive": 2}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B2", "intPrimitive": 1, "doublePrimitive": 4}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A2", "intPrimitive": 100, "doublePrimitive": 3}`)},
		},
	}
	offset := 0
	for _, caseName := range resultsetAggregateFilterNamedParameterSortedJoinCases {
		sends := caseSends[caseName]
		total := 1 + 1 + len(sends) + 1
		steps := scenario.Steps[offset : offset+total]
		offset += total
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", resultsetAggregateFilterNamedParameterSortedJoinID, caseName)
		}
		if steps[1].Op != "deploy" || steps[1].Statement != "s0" || steps[1].Epl != deployEPLs[caseName] {
			return fmt.Errorf("%s case %q step 1 must deploy the pinned s0 statement", resultsetAggregateFilterNamedParameterSortedJoinID, caseName)
		}
		for index, want := range sends {
			step := steps[index+2]
			if step.Op != "send" || step.Case != caseName || step.EventType != want.EventType {
				return fmt.Errorf("%s case %q step %d must send %q", resultsetAggregateFilterNamedParameterSortedJoinID, caseName, index+2, want.EventType)
			}
			payload, err := decodeResultsetAggregateFilterNamedParameterSortedJoinPayload(step)
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", resultsetAggregateFilterNamedParameterSortedJoinID, caseName, index+2, err)
			}
			var wantPayload struct {
				TheString       string  `json:"theString"`
				IntPrimitive    int     `json:"intPrimitive"`
				DoublePrimitive float64 `json:"doublePrimitive"`
				ID              int     `json:"id"`
			}
			if err := json.Unmarshal(want.Payload, &wantPayload); err != nil {
				return err
			}
			switch typed := payload.(type) {
			case resultsetAggregateFilterNamedParameterSortedJoinBean:
				if typed.TheString != wantPayload.TheString || typed.IntPrimitive != wantPayload.IntPrimitive || typed.DoublePrimitive != wantPayload.DoublePrimitive {
					return fmt.Errorf("%s case %q step %d SupportBean payload is not pinned", resultsetAggregateFilterNamedParameterSortedJoinID, caseName, index+2)
				}
			case resultsetAggregateFilterNamedParameterSortedJoinS1:
				if typed.ID != wantPayload.ID {
					return fmt.Errorf("%s case %q step %d SupportBean_S1 payload is not pinned", resultsetAggregateFilterNamedParameterSortedJoinID, caseName, index+2)
				}
			}
		}
		last := steps[total-1]
		if last.Op != "undeploy-all" || last.Case != caseName {
			return fmt.Errorf("%s case %q must end with undeploy-all", resultsetAggregateFilterNamedParameterSortedJoinID, caseName)
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetAggregateFilterNamedParameterSortedJoinID)
	}
	return nil
}

func runResultsetAggregateFilterNamedParameterSortedJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetAggregateFilterNamedParameterSortedJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	spans := map[string]int{
		"sorted-join-bound":    13,
		"sorted-join-unbound":  9,
		"sorted-multicriteria": 7,
	}
	for caseIndex, caseName := range resultsetAggregateFilterNamedParameterSortedJoinCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseName]]
		offset += spans[caseName]
		caseTrace, err := runResultsetAggregateFilterNamedParameterSortedJoinCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateFilterNamedParameterSortedJoinID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultsetAggregateFilterNamedParameterSortedJoinCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFilterNamedParameterSortedJoinBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateFilterNamedParameterSortedJoinS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetAggregateFilterNamedParameterSortedJoinJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: resultsetAggregateFilterNamedParameterSortedJoinID}
	sequence := uint64(0)
	record := func(batch esper.ResultBatch) {
		sequence++
		trace.Records = append(trace.Records, compat.TraceRecord{
			Case:      caseName,
			Operation: "listener",
			Statement: "s0",
			Sequence:  sequence,
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		})
	}

	for _, step := range steps {
		switch step.Op {
		case "case":
		case "deploy":
			var plan esper.Plan
			var err error
			beanString := esper.JoinField[string](1, "theString")
			beanInt := esper.JoinField[int](1, "intPrimitive")
			aFilter := esper.Like(beanString, esper.Literal("A%"))
			bFilter := esper.Like(beanString, esper.Literal("B%"))
			switch caseName {
			case "sorted-join-bound":
				// Join aggregate group rows are join tuples: the sorted
				// event-array column reads the bean member through
				// JoinEventValue(1) so the rows render the bean schema, and
				// the filtered maxby/minby columns read theString via
				// JoinField with the intPrimitive sort key.
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterSortedJoinS1](env, "SupportBean_S1").Window(esper.LastEvent())),
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterSortedJoinBean](env, "SupportBean").Window(esper.LengthWindow(4))),
				).Aggregate(
					esper.Alias("aMaxby", esper.FilterAggregate[string](esper.MaxBy[string, int](beanString, beanInt), aFilter)),
					esper.Alias("aMinby", esper.FilterAggregate[string](esper.MinBy[string, int](beanString, beanInt), aFilter)),
					esper.Alias("aSorted", esper.FilterAggregate[[]esper.Event](esper.SortedEventsBy[esper.Event, int](esper.JoinEventValue[esper.Event](1), beanInt, false), aFilter)),
					esper.Alias("bMaxby", esper.FilterAggregate[string](esper.MaxBy[string, int](beanString, beanInt), bFilter)),
					esper.Alias("bMinby", esper.FilterAggregate[string](esper.MinBy[string, int](beanString, beanInt), bFilter)),
					esper.Alias("bSorted", esper.FilterAggregate[[]esper.Event](esper.SortedEventsBy[esper.Event, int](esper.JoinEventValue[esper.Event](1), beanInt, false), bFilter)),
				).Query(esper.StatementName("s0")))
			case "sorted-join-unbound":
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterSortedJoinS1](env, "SupportBean_S1").Window(esper.LastEvent())),
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterSortedJoinBean](env, "SupportBean").Window(esper.KeepAll())),
				).Aggregate(
					esper.Alias("aMaxby", esper.FilterAggregate[string](esper.MaxBy[string, int](beanString, beanInt), aFilter)),
					esper.Alias("aMaxbyever", esper.FilterAggregate[string](esper.MaxByEver[string, int](beanString, beanInt), aFilter)),
					esper.Alias("aMinby", esper.FilterAggregate[string](esper.MinBy[string, int](beanString, beanInt), aFilter)),
					esper.Alias("aMinbyever", esper.FilterAggregate[string](esper.MinByEver[string, int](beanString, beanInt), aFilter)),
				).Query(esper.StatementName("s0")))
			default:
				// The multicriteria execution has no join: the filter
				// predicates bind to the plain stream field, not JoinField.
				plainString := esper.Field[resultsetAggregateFilterNamedParameterSortedJoinBean, string]("theString")
				plainInt := esper.Field[resultsetAggregateFilterNamedParameterSortedJoinBean, int]("intPrimitive")
				plainDouble := esper.Field[resultsetAggregateFilterNamedParameterSortedJoinBean, float64]("doublePrimitive")
				streamAFilter := esper.Like(plainString, esper.Literal("A%"))
				streamBFilter := esper.Like(plainString, esper.Literal("B%"))
				plan, err = env.Build(esper.From[resultsetAggregateFilterNamedParameterSortedJoinBean](env, "SupportBean").
					Window(esper.KeepAll()).AsRecord().Aggregate(
					esper.Alias("aSorted", esper.FilterAggregate[[]esper.Event](esper.SortedEvents(
						esper.Ascending(plainInt), esper.Ascending(plainDouble)), streamAFilter)),
					esper.Alias("bSorted", esper.FilterAggregate[[]esper.Event](esper.SortedEvents(
						esper.Ascending(plainInt), esper.Ascending(plainDouble)), streamBFilter)),
				).Query(esper.StatementName("s0")))
			}
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q: %w", step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q: %w", step.Statement, err)
			}
			for _, statement := range deployment.Statements() {
				if statement.Name() == "s0" {
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
				}
			}
		case "send":
			payload, err := decodeResultsetAggregateFilterNamedParameterSortedJoinPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
		}
	}
	return trace, nil
}

func decodeResultsetAggregateFilterNamedParameterSortedJoinPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(fields, "theString", "intPrimitive", "doublePrimitive"); err != nil {
			return nil, err
		}
		var bean resultsetAggregateFilterNamedParameterSortedJoinBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S1":
		if err := requireResultsetAggregateFilterNamedParameterSortedJoinFields(fields, "id", "doublePrimitive"); err != nil {
			return nil, err
		}
		var s1 resultsetAggregateFilterNamedParameterSortedJoinS1
		if err := json.Unmarshal(step.Payload, &s1); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return s1, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateFilterNamedParameterSortedJoinID, step.EventType)
	}
}

func requireResultsetAggregateFilterNamedParameterSortedJoinFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetAggregateFilterNamedParameterSortedJoinStringArray(raw json.RawMessage, expected []string, name string) error {
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
