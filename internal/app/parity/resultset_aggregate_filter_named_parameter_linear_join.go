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
	resultsetAggregateFilterNamedParameterLinearJoinID          = "resultset-aggregate-filter-named-parameter-linear-join"
	resultsetAggregateFilterNamedParameterLinearJoinDescription = "ResultSetAggregateFilterNamedParameter join and mixed-filter gap executions: filtered access aggregates (first, last, window, firstever, lastever, countever) over joined last-event and length-window or keepall streams with JoinField-bound filter predicates, plus mixed-filter and filter-leading window(sb) event-array projections rendered as event rows (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java)."
	resultsetAggregateFilterNamedParameterLinearJoinJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetAggregateFilterNamedParameterLinearJoinSource      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"

	// EPL pins are the exact Java-source concatenations (no spaces between
	// the comma-separated select expressions).
	resultsetAggregateFilterNamedParameterLinearJoinBoundEPL   = "@name('s0') select first(intPrimitive, filter:theString like 'A%') as aFirst,last(intPrimitive, filter:theString like 'A%') as aLast,window(intPrimitive, filter:theString like 'A%') as aWindow,first(intPrimitive, filter:theString like 'B%') as bFirst,last(intPrimitive, filter:theString like 'B%') as bLast,window(intPrimitive, filter:theString like 'B%') as bWindow from SupportBean_S1#lastevent, SupportBean#length(5)"
	resultsetAggregateFilterNamedParameterLinearJoinUnboundEPL = "@name('s0') select first(intPrimitive, filter:theString like 'A%') as aFirst,firstever(intPrimitive, filter:theString like 'A%') as aFirstever,last(intPrimitive, filter:theString like 'A%') as aLast,lastever(intPrimitive, filter:theString like 'A%') as aLastever,countever(intPrimitive, filter:theString like 'A%') as aCountever from SupportBean_S1#lastevent, SupportBean#keepall"
	resultsetAggregateFilterNamedParameterLinearJoinMixedEPL   = "@name('s0') select window(sb, filter:theString like 'A%') as c0,window(sb) as c1,window(filter:theString like 'B%', sb) as c2 from SupportBean#keepall as sb"
)

var (
	resultsetAggregateFilterNamedParameterLinearJoinJavaSources = []string{
		resultsetAggregateFilterNamedParameterLinearJoinSource,
	}
	resultsetAggregateFilterNamedParameterLinearJoinJavaRuntimeIDs = []string{
		"java-runtime-d652fb38c70d8bcc778f",
		"java-runtime-4add988d1015cdae8bb2",
		"java-runtime-97101f0ae69f6fad8c29",
	}
	resultsetAggregateFilterNamedParameterLinearJoinJavaExecutions = []string{
		"ResultSetAggregateAccessAggLinearBound{join=true}",
		"ResultSetAggregateAccessAggLinearUnbound{join=true}",
		"ResultSetAggregateAccessAggLinearBoundMixedFilter",
	}
	resultsetAggregateFilterNamedParameterLinearJoinJavaStaticIDs = []string{
		"java-85b8ba64c1be948a5676",
		"java-97884ae57325e3acd3f2",
		"java-9d0ccec4c0bb19e6a14c",
	}
	resultsetAggregateFilterNamedParameterLinearJoinCases = []string{
		"linear-join-bound",
		"linear-join-unbound",
		"mixed-filter-window",
	}
	resultsetAggregateFilterNamedParameterLinearJoinOrdinals = []int{9, 11, 13}
)

type resultsetAggregateFilterNamedParameterLinearJoinBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type resultsetAggregateFilterNamedParameterLinearJoinS1 struct {
	ID int `esper:"id"`
}

func loadResultsetAggregateFilterNamedParameterLinearJoinScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetAggregateFilterNamedParameterLinearJoinID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetAggregateFilterNamedParameterLinearJoinID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilterNamedParameterLinearJoinID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetAggregateFilterNamedParameterLinearJoinID, err)
	}
	if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetAggregateFilterNamedParameterLinearJoinID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetAggregateFilterNamedParameterLinearJoinID ||
		metadata.Description != resultsetAggregateFilterNamedParameterLinearJoinDescription ||
		metadata.JavaCommit != resultsetAggregateFilterNamedParameterLinearJoinJavaCommit ||
		metadata.JavaSource != resultsetAggregateFilterNamedParameterLinearJoinSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetAggregateFilterNamedParameterLinearJoinID)
	}
	if err := validateResultsetAggregateFilterNamedParameterLinearJoinStringArray(root["javaRuntimes"], resultsetAggregateFilterNamedParameterLinearJoinJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetAggregateFilterNamedParameterLinearJoinStringArray(root["javaNames"], resultsetAggregateFilterNamedParameterLinearJoinJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetAggregateFilterNamedParameterLinearJoinStringArray(root["javaStaticIds"], resultsetAggregateFilterNamedParameterLinearJoinJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateResultsetAggregateFilterNamedParameterLinearJoinStringArray(root["javaFlags"], []string{}, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetAggregateFilterNamedParameterLinearJoinCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly three cases", resultsetAggregateFilterNamedParameterLinearJoinID)
	}
	observedEPL := map[string]string{
		"linear-join-bound":   resultsetAggregateFilterNamedParameterLinearJoinBoundEPL,
		"linear-join-unbound": resultsetAggregateFilterNamedParameterLinearJoinUnboundEPL,
		"mixed-filter-window": resultsetAggregateFilterNamedParameterLinearJoinMixedEPL,
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(object,
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
		if definition.Case != resultsetAggregateFilterNamedParameterLinearJoinCases[index] ||
			definition.Ordinal != resultsetAggregateFilterNamedParameterLinearJoinOrdinals[index] ||
			definition.RuntimeID != resultsetAggregateFilterNamedParameterLinearJoinJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetAggregateFilterNamedParameterLinearJoinJavaExecutions[index] ||
			definition.Observation != "listener" || definition.IteratorSnapshots != 0 ||
			definition.EPL != observedEPL[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetAggregateFilterNamedParameterLinearJoinID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", resultsetAggregateFilterNamedParameterLinearJoinID)
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
			if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload compat.Step
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeResultsetAggregateFilterNamedParameterLinearJoinPayload(payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(object, "op", "case"); err != nil {
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
	if err := validateResultsetAggregateFilterNamedParameterLinearJoinScenario(scenario); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func validateResultsetAggregateFilterNamedParameterLinearJoinScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != resultsetAggregateFilterNamedParameterLinearJoinID {
		return fmt.Errorf("%s scenario shape is not pinned", resultsetAggregateFilterNamedParameterLinearJoinID)
	}
	deployEPLs := map[string]string{
		"linear-join-bound":   resultsetAggregateFilterNamedParameterLinearJoinBoundEPL,
		"linear-join-unbound": resultsetAggregateFilterNamedParameterLinearJoinUnboundEPL,
		"mixed-filter-window": resultsetAggregateFilterNamedParameterLinearJoinMixedEPL,
	}
	// Per case: marker, one s0 deploy, sends in Java order, undeploy-all.
	// The kickoff SupportBean_S1(0) joins an empty SupportBean window and
	// produces no output.
	caseSends := map[string][]compat.Step{
		"linear-join-bound": {
			{EventType: "SupportBean_S1", Payload: json.RawMessage(`{"id": 0}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X1", "intPrimitive": 1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B2", "intPrimitive": 2}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B3", "intPrimitive": 3}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A4", "intPrimitive": 4}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B5", "intPrimitive": 5}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A6", "intPrimitive": 6}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X2", "intPrimitive": 7}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X3", "intPrimitive": 8}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X4", "intPrimitive": 9}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X5", "intPrimitive": 10}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X6", "intPrimitive": 11}`)},
		},
		"linear-join-unbound": {
			{EventType: "SupportBean_S1", Payload: json.RawMessage(`{"id": 0}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X0", "intPrimitive": 0}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A1", "intPrimitive": 1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X2", "intPrimitive": 2}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A3", "intPrimitive": 3}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X4", "intPrimitive": 4}`)},
		},
		"mixed-filter-window": {
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "X1", "intPrimitive": 1}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "A2", "intPrimitive": 2}`)},
			{EventType: "SupportBean", Payload: json.RawMessage(`{"theString": "B3", "intPrimitive": 3}`)},
		},
	}
	offset := 0
	for _, caseName := range resultsetAggregateFilterNamedParameterLinearJoinCases {
		sends := caseSends[caseName]
		total := 1 + 1 + len(sends) + 1
		steps := scenario.Steps[offset : offset+total]
		offset += total
		if steps[0].Op != "case" || steps[0].Case != caseName {
			return fmt.Errorf("%s case %q must start with its case marker", resultsetAggregateFilterNamedParameterLinearJoinID, caseName)
		}
		if steps[1].Op != "deploy" || steps[1].Statement != "s0" || steps[1].Epl != deployEPLs[caseName] {
			return fmt.Errorf("%s case %q step 1 must deploy the pinned s0 statement", resultsetAggregateFilterNamedParameterLinearJoinID, caseName)
		}
		for index, want := range sends {
			step := steps[index+2]
			if step.Op != "send" || step.Case != caseName || step.EventType != want.EventType {
				return fmt.Errorf("%s case %q step %d must send %q", resultsetAggregateFilterNamedParameterLinearJoinID, caseName, index+2, want.EventType)
			}
			payload, err := decodeResultsetAggregateFilterNamedParameterLinearJoinPayload(step)
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", resultsetAggregateFilterNamedParameterLinearJoinID, caseName, index+2, err)
			}
			var wantPayload struct {
				TheString    string `json:"theString"`
				IntPrimitive int    `json:"intPrimitive"`
				ID           int    `json:"id"`
			}
			if err := json.Unmarshal(want.Payload, &wantPayload); err != nil {
				return err
			}
			switch typed := payload.(type) {
			case resultsetAggregateFilterNamedParameterLinearJoinBean:
				if typed.TheString != wantPayload.TheString || typed.IntPrimitive != wantPayload.IntPrimitive {
					return fmt.Errorf("%s case %q step %d SupportBean payload is not pinned", resultsetAggregateFilterNamedParameterLinearJoinID, caseName, index+2)
				}
			case resultsetAggregateFilterNamedParameterLinearJoinS1:
				if typed.ID != wantPayload.ID {
					return fmt.Errorf("%s case %q step %d SupportBean_S1 payload is not pinned", resultsetAggregateFilterNamedParameterLinearJoinID, caseName, index+2)
				}
			}
		}
		last := steps[total-1]
		if last.Op != "undeploy-all" || last.Case != caseName {
			return fmt.Errorf("%s case %q must end with undeploy-all", resultsetAggregateFilterNamedParameterLinearJoinID, caseName)
		}
	}
	if offset != len(scenario.Steps) {
		return fmt.Errorf("%s scenario has trailing steps", resultsetAggregateFilterNamedParameterLinearJoinID)
	}
	return nil
}

func runResultsetAggregateFilterNamedParameterLinearJoinScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultsetAggregateFilterNamedParameterLinearJoinScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	spans := map[string]int{
		"linear-join-bound":   15,
		"linear-join-unbound": 9,
		"mixed-filter-window": 6,
	}
	for caseIndex, caseName := range resultsetAggregateFilterNamedParameterLinearJoinCases {
		caseSteps := scenario.Steps[offset : offset+spans[caseName]]
		offset += spans[caseName]
		caseTrace, err := runResultsetAggregateFilterNamedParameterLinearJoinCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", resultsetAggregateFilterNamedParameterLinearJoinID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	return trace, nil
}

func runResultsetAggregateFilterNamedParameterLinearJoinCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[resultsetAggregateFilterNamedParameterLinearJoinBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[resultsetAggregateFilterNamedParameterLinearJoinS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}

	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetAggregateFilterNamedParameterLinearJoinJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: resultsetAggregateFilterNamedParameterLinearJoinID}
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
			beanInt := esper.JoinField[int](1, "intPrimitive")
			beanString := esper.JoinField[string](1, "theString")
			aFilter := esper.Like(beanString, esper.Literal("A%"))
			bFilter := esper.Like(beanString, esper.Literal("B%"))
			switch caseName {
			case "linear-join-bound":
				// In a join aggregate the group rows are join tuples, so the
				// filtered-aggregate predicates and inputs bind through
				// JoinField (FilterAggregate evaluates the predicate with
				// each group event as ctx.Event, resolving the joinTuple
				// member).
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterLinearJoinS1](env, "SupportBean_S1").Window(esper.LastEvent())),
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterLinearJoinBean](env, "SupportBean").Window(esper.LengthWindow(5))),
				).Aggregate(
					esper.Alias("aFirst", esper.FilterAggregate[int](esper.First[int](beanInt), aFilter)),
					esper.Alias("aLast", esper.FilterAggregate[int](esper.Last[int](beanInt), aFilter)),
					esper.Alias("aWindow", esper.FilterAggregate[[]int](esper.WindowValues[int](beanInt), aFilter)),
					esper.Alias("bFirst", esper.FilterAggregate[int](esper.First[int](beanInt), bFilter)),
					esper.Alias("bLast", esper.FilterAggregate[int](esper.Last[int](beanInt), bFilter)),
					esper.Alias("bWindow", esper.FilterAggregate[[]int](esper.WindowValues[int](beanInt), bFilter)),
				).Query(esper.StatementName("s0")))
			case "linear-join-unbound":
				plan, err = env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterLinearJoinS1](env, "SupportBean_S1").Window(esper.LastEvent())),
					esper.JoinSource(esper.From[resultsetAggregateFilterNamedParameterLinearJoinBean](env, "SupportBean").Window(esper.KeepAll())),
				).Aggregate(
					esper.Alias("aFirst", esper.FilterAggregate[int](esper.First[int](beanInt), aFilter)),
					esper.Alias("aFirstever", esper.FilterAggregate[int](esper.FirstEver[int](beanInt), aFilter)),
					esper.Alias("aLast", esper.FilterAggregate[int](esper.Last[int](beanInt), aFilter)),
					esper.Alias("aLastever", esper.FilterAggregate[int](esper.LastEver[int](beanInt), aFilter)),
					esper.Alias("aCountever", esper.FilterAggregate[int64](esper.CountEver(beanInt), aFilter)),
				).Query(esper.StatementName("s0")))
			default:
				_ = aFilter
				_ = bFilter
				stream := esper.From[resultsetAggregateFilterNamedParameterLinearJoinBean](env, "SupportBean").Window(esper.KeepAll()).AsRecord()
				sbField := esper.Field[resultsetAggregateFilterNamedParameterLinearJoinBean, string]("theString")
				plan, err = env.Build(stream.Aggregate(
					esper.Alias("c0", esper.FilterAggregate[[]esper.Event](esper.WindowEvents(), esper.Like(sbField, esper.Literal("A%")))),
					esper.Alias("c1", esper.WindowEvents()),
					esper.Alias("c2", esper.FilterAggregate[[]esper.Event](esper.WindowEvents(), esper.Like(sbField, esper.Literal("B%")))),
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
			payload, err := decodeResultsetAggregateFilterNamedParameterLinearJoinPayload(step)
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

func decodeResultsetAggregateFilterNamedParameterLinearJoinPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(fields, "theString", "intPrimitive"); err != nil {
			return nil, err
		}
		var bean resultsetAggregateFilterNamedParameterLinearJoinBean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S1":
		if err := requireResultsetAggregateFilterNamedParameterLinearJoinFields(fields, "id"); err != nil {
			return nil, err
		}
		var s1 resultsetAggregateFilterNamedParameterLinearJoinS1
		if err := json.Unmarshal(step.Payload, &s1); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return s1, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", resultsetAggregateFilterNamedParameterLinearJoinID, step.EventType)
	}
}

func requireResultsetAggregateFilterNamedParameterLinearJoinFields(object map[string]json.RawMessage, names ...string) error {
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

func validateResultsetAggregateFilterNamedParameterLinearJoinStringArray(raw json.RawMessage, expected []string, name string) error {
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
