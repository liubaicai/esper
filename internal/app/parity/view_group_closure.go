package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// view_group_closure.go replays the three remaining ViewGroup executions
// (ords 7, 8 and 19) against the pinned Java oracle, closing the suite:
//
//   - invalid (ord 7, ViewGroupInvalid): five tryInvalidCompile probes pin
//     the Java message prefixes for groupwin-declaration rejections —
//     multiple groupwin declarations, a groupwin with no child view, a
//     groupwin that is not the first declaration, a merge view combined
//     with multiple data windows, and a null-typed group-window criteria
//     expression. All probes compile without the runtime path, mirroring
//     env.tryInvalidCompile's path-less compileWCheckedEx.
//   - length-win-weighted-avg (ord 8, ViewGroupLengthWinWeightAvg,
//     EXCLUDEWHENINSTRUMENTED+PERFORMANCE): deploys the
//     #groupwin(type)#length(10000000)#weighted_avg(measurement,confidence)
//     statement with a listener, sends a 100-event prime batch and a
//     10000-event measured batch of SupportSensorEvent beans, and undeploys.
//     The suite's only assertion is a wall-clock bound, so the trace stays
//     thin: one deployed record plus one sent marker per batch.
//   - escaped-property-text (ord 19, ViewGroupEscapedPropertyText,
//     SERDEREQUIRED): compiles and deploys the three-statement module —
//     create schema event as EventWithTags, insert into stream1, and a
//     groupwin(name, tags('a\.b')) length-10 select with a having count(1)
//     >= 5 predicate — then undeploys. The escaped mapped-property key
//     'a\.b' resolves to the map key "a.b". Deployed records only.
//
// Approved differences (observably identical to the Java EPL):
//   - `create schema`/`insert into` module statements map to env-level
//     registrations and per-statement deploys (RegisterStruct, RegisterMap,
//     InsertInto); Go has no module path, so the single compileDeploy module
//     is replayed as one deploy step whose Go side performs the three
//     registrations/deploys in statement order.
//   - Probe 4 (merge view over multiple data windows) is unrepresentable in
//     the typed Go API — there is no merge WindowSpec — so it records the
//     pinned prefix without claiming a Go rejection boundary, mirroring the
//     context_lifecycle.go unrepresentable precedent.
//   - Probe 5's `somefield null` column has no Go null type (map fields
//     default to any), so the nearest expressible boundary is a null literal
//     group-window key, which the shared core rejects with the same
//     "Group-window received a null-typed criteria expression" wording.
//   - The send-batch payloads pin the batch's first event; the
//     index-sequence mode documents that measurement/confidence advance with
//     the batch index exactly like the suite's (double) i constructor args.

const (
	viewGroupClosureID         = "view-group-closure"
	viewGroupClosureJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	viewGroupClosureSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewGroup.java"
)

const viewGroupClosureDescription = "ViewGroup closure surface (ords 7, 8, 19): invalid replays ViewGroupInvalid's five tryInvalidCompile probes pinning the groupwin-declaration rejection prefixes (multiple groupwin declarations, groupwin without a child view, groupwin not in first position, merge with multiple data windows — unrepresentable in Go, and a null-typed criteria expression); length-win-weighted-avg replays ViewGroupLengthWinWeightAvg's performance smoke, deploying the groupwin(type)/length(10000000)/weighted_avg(measurement,confidence) statement and sending 100 prime plus 10000 measured SupportSensorEvent beans with only deployed/sent markers (the wall-clock assert is not trace-visible); escaped-property-text replays ViewGroupEscapedPropertyText's compile+deploy+undeploy of the three-statement module whose select groups stream1 by name and the escaped mapped property tags('a\\.b') under having count(1) >= 5. compile-error records carry the pinned Java message prefixes; the unrepresentable record pins the merge-view prefix; deployed records mark each module statement; sent records pin the batch sizes (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewGroup.java)."

var (
	viewGroupClosureJavaRuntimeIDs = []string{
		"java-runtime-a72aa6eebc4ce8d21115",
		"java-runtime-e718af611543b6d40436",
		"java-runtime-d9b2e762c318369875e5",
	}
	viewGroupClosureJavaExecutions = []string{
		"ViewGroupInvalid",
		"ViewGroupLengthWinWeightAvg",
		"ViewGroupEscapedPropertyText",
	}
	viewGroupClosureJavaStaticIDs = []string{
		"java-cd39f97bf5d0490c295a",
		"java-586becfef68e2842ef18",
		"java-3426d2a7294c0f93c788",
	}
	viewGroupClosureJavaFlags = []string{"EXCLUDEWHENINSTRUMENTED", "PERFORMANCE", "SERDEREQUIRED"}
	viewGroupClosureCases     = []string{
		"invalid",
		"length-win-weighted-avg",
		"escaped-property-text",
	}
	viewGroupClosureOrdinals = []int{7, 8, 19}
	viewGroupClosureSources  = []string{viewGroupClosureSource}
)

var viewGroupClosureCaseRuntimeIDs = map[string]string{
	"invalid":                 "java-runtime-a72aa6eebc4ce8d21115",
	"length-win-weighted-avg": "java-runtime-e718af611543b6d40436",
	"escaped-property-text":   "java-runtime-d9b2e762c318369875e5",
}

// The byte-exact EPLs the probes and deploys pin. Probe EPLs are the
// verbatim tryInvalidCompile inputs; the escaped-property-text deploy pins
// the whole three-statement module text including the blank lines and the
// trailing newline the suite's string concatenation produces.
var viewGroupClosureProbeEPLs = map[string]string{
	"multiple-groupwin":   "select * from SupportBean#groupwin(theString)#length(1)#groupwin(theString)#uni(intPrimitive)",
	"groupwin-no-child":   "select avg(price), symbol from SupportMarketDataBean#length(100)#groupwin(symbol)",
	"groupwin-not-first":  "select * from SupportBean#keepall#groupwin(theString)#length(2)",
	"groupwin-merge":      "select * from SupportBean#groupwin(theString)#length(2)#merge(theString)#keepall",
	"groupwin-null-field": "create schema MyEvent(somefield null);\nselect * from MyEvent#groupwin(somefield)#length(2)",
}

const viewGroupClosureLengthWinEPL = "@name('s0') select * from SupportSensorEvent#groupwin(type)#length(10000000)#weighted_avg(measurement, confidence)"

const viewGroupClosureModuleEPL = "create schema event as com.espertech.esper.regressionlib.suite.view.ViewGroup$EventWithTags;\n" +
	"\n" +
	"insert into stream1\n" +
	"select name, tags from event;\n" +
	"\n" +
	"select name, tags('a\\.b') from stream1.std:groupwin(name, tags('a\\.b')).win:length(10)\n" +
	"having count(1) >= 5;\n"

// The pinned Java message prefixes (SupportMessageAssertUtil.assertMessage
// startsWith semantics). All five share the data-window declaration prefix.
var viewGroupClosureProbeErrors = map[string]string{
	"multiple-groupwin":   "Failed to validate data window declaration: Multiple groupwin-declarations are not supported",
	"groupwin-no-child":   "Failed to validate data window declaration: Invalid use of the 'groupwin' view, the view requires one or more child views to group, or consider using the group-by clause",
	"groupwin-not-first":  "Failed to validate data window declaration: The 'groupwin' declaration must occur in the first position",
	"groupwin-merge":      "Failed to validate data window declaration: The 'merge' declaration cannot be used in conjunction with multiple data windows",
	"groupwin-null-field": "Failed to validate data window declaration: Group-window received a null-typed criteria expression",
}

var viewGroupClosureCaseObservations = []string{
	"compile-error+unrepresentable; five path-less tryInvalidCompile probes pin the groupwin-declaration rejections: multiple groupwin declarations, groupwin without a child view, groupwin not in first position, the merge view over multiple data windows (pinned-only; no Go merge view), and the null-typed criteria expression",
	"deployed+sent; the groupwin(type)/length(10000000)/weighted_avg statement deploys with a listener, then 100 prime and 10000 measured SupportSensorEvent sends replay as index-sequenced batches; the suite's sub-second wall-clock assert is not trace-visible so only the deployed marker and per-batch sent counts record",
	"deployed; the three-statement module (create schema event as EventWithTags, insert into stream1 select name, tags, select name, tags('a\\.b') from stream1 groupwin(name, tags('a\\.b')) length(10) having count(1) >= 5) compiles and deploys, then undeploy-all retires it",
}

var viewGroupClosureCaseEPLs = []string{
	"select * from SupportBean#groupwin(theString)#length(1)#groupwin(theString)#uni(intPrimitive)",
	viewGroupClosureLengthWinEPL,
	viewGroupClosureModuleEPL,
}

// viewGroupClosureCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count
// keys. build-error steps carry the byte-exact probe EPL and the pinned
// expectError prefix; every probe is path-less (compileWithoutPath=1)
// mirroring env.tryInvalidCompile(epl, ...). The unrepresentable step pins
// the merge-view prefix in expectError. send-batch steps pin the first
// event's payload, the index-sequence mode and the batch count; the deploy
// steps carry the byte-exact EPL text the Java oracle compiles.
var viewGroupClosureCaseSteps = map[string][]string{
	"invalid": {
		"build-error|multiple-groupwin|||" + viewGroupClosureProbeEPLs["multiple-groupwin"] + "||" + viewGroupClosureProbeErrors["multiple-groupwin"] + "|1|||",
		"build-error|groupwin-no-child|||" + viewGroupClosureProbeEPLs["groupwin-no-child"] + "||" + viewGroupClosureProbeErrors["groupwin-no-child"] + "|1|||",
		"build-error|groupwin-not-first|||" + viewGroupClosureProbeEPLs["groupwin-not-first"] + "||" + viewGroupClosureProbeErrors["groupwin-not-first"] + "|1|||",
		"unrepresentable|groupwin-merge|||" + viewGroupClosureProbeEPLs["groupwin-merge"] + "||" + viewGroupClosureProbeErrors["groupwin-merge"] + "|1|||",
		"build-error|groupwin-null-field|||" + viewGroupClosureProbeEPLs["groupwin-null-field"] + "||" + viewGroupClosureProbeErrors["groupwin-null-field"] + "|1|||",
	},
	"length-win-weighted-avg": {
		"deploy|s0|||" + viewGroupClosureLengthWinEPL + "||||||",
		"deployed|s0|||||||||",
		"send-batch||prime|SupportSensorEvent||{\"id\":0,\"type\":\"A\",\"device\":\"1\",\"measurement\":0,\"confidence\":0}|||index-sequence||100",
		"send-batch||measure|SupportSensorEvent||{\"id\":0,\"type\":\"A1\",\"device\":\"1\",\"measurement\":0,\"confidence\":0}|||index-sequence||10000",
		"undeploy-all||||||||||",
	},
	"escaped-property-text": {
		"deploy|module|||" + viewGroupClosureModuleEPL + "||||||",
		"deployed|schema|||||||||",
		"deployed|insert|||||||||",
		"deployed|select|||||||||",
		"undeploy-all||||||||||",
	},
}

// viewGroupClosureBean mirrors SupportBean's asserted fields.
type viewGroupClosureBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// viewGroupClosureMarket mirrors SupportMarketDataBean's asserted fields.
type viewGroupClosureMarket struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
	Volume int64   `esper:"volume"`
	Feed   string  `esper:"feed"`
}

// viewGroupClosureSensor mirrors SupportSensorEvent's asserted fields.
type viewGroupClosureSensor struct {
	ID          int     `esper:"id"`
	Type        string  `esper:"type"`
	Device      string  `esper:"device"`
	Measurement float64 `esper:"measurement"`
	Confidence  float64 `esper:"confidence"`
}

// viewGroupClosureTagged mirrors ViewGroup.EventWithTags: a name plus a
// string map whose "a.b" key the escaped mapped property resolves.
type viewGroupClosureTagged struct {
	Name string            `esper:"name"`
	Tags map[string]string `esper:"tags"`
}

// viewGroupClosureCaseState carries the per-case replay state: the
// environment/engine pair, the deployed-step label bookkeeping, and the
// deployments the undeploy steps retire.
type viewGroupClosureCaseState struct {
	caseName       string
	env            *esper.Environment
	engine         *esper.Engine
	trace          *compat.Trace
	deployedLabels map[string]bool
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	sequences      map[string]uint64
}

func runViewGroupClosureScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateViewGroupClosureScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeViewGroupClosure(ctx, scenario, &trace)
}

// executeViewGroupClosure replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeViewGroupClosure(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *viewGroupClosureCaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		if step.Op != "case" && state == nil {
			return *trace, fmt.Errorf("%s: step %q arrives before any case marker", viewGroupClosureID, step.Op)
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startViewGroupClosureCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "deployed":
			if err := state.deployed(step); err != nil {
				return *trace, err
			}
		case "send-batch":
			if err := state.sendBatch(ctx, step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", viewGroupClosureID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startViewGroupClosureCase builds the fresh per-case environment: the
// SupportBean/SupportMarketDataBean/SupportSensorEvent event types the
// probes and the weighted-avg case reference, and the engine pinned to the
// case's Java runtime id at the epoch start time.
func startViewGroupClosureCase(caseName string, trace *compat.Trace) (*viewGroupClosureCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[viewGroupClosureBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[viewGroupClosureMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[viewGroupClosureSensor](env, "SupportSensorEvent"); err != nil {
		return nil, err
	}
	state := &viewGroupClosureCaseState{
		caseName:       caseName,
		env:            env,
		deployedLabels: map[string]bool{},
		deployments:    map[string]*esper.Deployment{},
		sequences:      map[string]uint64{},
		trace:          trace,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(viewGroupClosureCaseRuntimeIDs[caseName]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

// deploy executes one deploy step. The length-win-weighted-avg deploy builds
// the groupwin(type)/length/weighted-avg plan and attaches the listener the
// Java execution registers (a no-op subscriber: the suite never asserts
// deliveries, so the trace stays thin). The escaped-property-text deploy
// replays the single compileDeploy module as the env-level schema
// registrations plus the insert-into and select statement deploys in module
// order.
func (s *viewGroupClosureCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	switch s.caseName {
	case "length-win-weighted-avg":
		if label != "s0" || step.Epl != viewGroupClosureLengthWinEPL {
			return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", viewGroupClosureID, s.caseName, label, step.Epl)
		}
		sensor := esper.From[viewGroupClosureSensor](s.env, "SupportSensorEvent")
		measurement := esper.Field[viewGroupClosureSensor, float64]("measurement")
		confidence := esper.Field[viewGroupClosureSensor, float64]("confidence")
		plan, err := s.env.Build(sensor.Window(
			esper.GroupWindow(esper.Field[viewGroupClosureSensor, string]("type"), esper.LengthWindow(10000000))).
			Aggregate(esper.Alias("average", esper.WeightedAvg[float64, float64](measurement, confidence))).
			Query(esper.StatementName("s0")))
		if err != nil {
			return fmt.Errorf("%s: deploy %q: %w", viewGroupClosureID, label, err)
		}
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: deploy %q: %w", viewGroupClosureID, label, err)
		}
		s.deployments[label] = deployment
		s.deployOrder = append(s.deployOrder, label)
		s.deployedLabels[label] = true
		// Mirrors addListener("s0"): the suite attaches a listener but never
		// asserts a delivery, so the subscriber records nothing.
		for _, statement := range deployment.Statements() {
			if _, err := statement.Subscribe(func(context.Context, esper.ResultBatch) error { return nil }); err != nil {
				return fmt.Errorf("%s: deploy %q listener: %w", viewGroupClosureID, label, err)
			}
		}
		return nil
	case "escaped-property-text":
		if label != "module" || step.Epl != viewGroupClosureModuleEPL {
			return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", viewGroupClosureID, s.caseName, label, step.Epl)
		}
		return s.deployEscapedModule(ctx)
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", viewGroupClosureID, s.caseName, label)
}

// deployEscapedModule replays the ord-19 module: `create schema event as
// EventWithTags` maps to the env-level struct registration, `insert into
// stream1 select name, tags from event` registers the stream1 map schema and
// deploys the insert-into, and the select deploys the groupwin(name,
// tags('a\.b')) length-10 statement with its having count(1) >= 5 predicate.
func (s *viewGroupClosureCaseState) deployEscapedModule(ctx context.Context) error {
	if _, err := esper.RegisterStruct[viewGroupClosureTagged](s.env, "event"); err != nil {
		return fmt.Errorf("%s: register event schema: %w", viewGroupClosureID, err)
	}
	if _, err := esper.RegisterMap(s.env, "stream1", []esper.FieldSpec{
		esper.FieldDef("name", reflect.TypeOf("")),
		esper.FieldDef("tags", reflect.TypeOf(map[string]string{})),
	}); err != nil {
		return fmt.Errorf("%s: register stream1 schema: %w", viewGroupClosureID, err)
	}
	event := esper.From[viewGroupClosureTagged](s.env, "event")
	name := esper.Field[viewGroupClosureTagged, string]("name")
	tags := esper.Field[viewGroupClosureTagged, map[string]string]("tags")
	insertPlan, err := s.env.Build(esper.Select(event,
		esper.Alias("name", name),
		esper.Alias("tags", tags),
	).InsertInto("stream1"))
	if err != nil {
		return fmt.Errorf("%s: deploy module insert: %w", viewGroupClosureID, err)
	}
	insertDeployment, err := s.engine.Deploy(ctx, insertPlan)
	if err != nil {
		return fmt.Errorf("%s: deploy module insert: %w", viewGroupClosureID, err)
	}
	s.deployments["insert"] = insertDeployment
	s.deployOrder = append(s.deployOrder, "insert")
	// `select name, tags('a\.b') from stream1.std:groupwin(name,
	// tags('a\.b')).win:length(10) having count(1) >= 5` — the escaped
	// mapped-property key resolves to the "a.b" map key.
	stream1 := esper.FromAny(s.env, "stream1")
	streamName := esper.Field[map[string]any, string]("name")
	streamTags := esper.Field[map[string]any, map[string]string]("tags")
	tagAB := esper.MapAt[string](streamTags, esper.Literal("a.b"))
	selectPlan, err := s.env.Build(stream1.Window(esper.GroupWindowKeys(
		[]esper.Expr{streamName, tagAB},
		esper.LengthWindow(10))).
		Aggregate(
			esper.Alias("name", streamName),
			esper.Alias("tags('a\\.b')", tagAB)).
		Having(esper.GreaterOrEqual[int64](esper.Count[int64](esper.Literal(int64(1))), esper.Literal(int64(5)))).
		Query())
	if err != nil {
		return fmt.Errorf("%s: deploy module select: %w", viewGroupClosureID, err)
	}
	selectDeployment, err := s.engine.Deploy(ctx, selectPlan)
	if err != nil {
		return fmt.Errorf("%s: deploy module select: %w", viewGroupClosureID, err)
	}
	s.deployments["select"] = selectDeployment
	s.deployOrder = append(s.deployOrder, "select")
	s.deployedLabels["schema"] = true
	s.deployedLabels["insert"] = true
	s.deployedLabels["select"] = true
	return nil
}

// deployed emits the deployed marker for a statement label the preceding
// deploy step registered, mirroring the oracle's per-statement marker.
func (s *viewGroupClosureCaseState) deployed(step compat.Step) error {
	if !s.deployedLabels[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", viewGroupClosureID, step.Statement)
	}
	s.sequences[step.Statement+":deployed"]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequences[step.Statement+":deployed"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

// sendBatch replays one send-batch step: count SupportSensorEvent beans whose
// measurement/confidence advance with the batch index (the index-sequence
// mode mirrors the suite's (double) i constructor arguments), then emits the
// thin trace's sent marker carrying the batch name and count.
func (s *viewGroupClosureCaseState) sendBatch(ctx context.Context, step compat.Step) error {
	if step.Mode != "index-sequence" {
		return fmt.Errorf("%s: send-batch %q has unpinned mode %q", viewGroupClosureID, step.Name, step.Mode)
	}
	if step.Count == nil || *step.Count <= 0 {
		return fmt.Errorf("%s: send-batch %q has no positive count", viewGroupClosureID, step.Name)
	}
	var payload struct {
		ID     int    `json:"id"`
		Type   string `json:"type"`
		Device string `json:"device"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: send-batch %s payload: %w", viewGroupClosureID, step.EventType, err)
	}
	for i := int64(0); i < *step.Count; i++ {
		event := viewGroupClosureSensor{
			ID:          payload.ID,
			Type:        payload.Type,
			Device:      payload.Device,
			Measurement: float64(i),
			Confidence:  float64(i),
		}
		if err := s.engine.Send(ctx, step.EventType, event); err != nil {
			return fmt.Errorf("%s: send-batch %s #%d: %w", viewGroupClosureID, step.EventType, i, err)
		}
	}
	count := *step.Count
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "sent",
		Name:      step.Name,
		Count:     &count,
	})
	return nil
}

// buildError runs one expected-invalid probe against the fluent equivalent
// of the pinned EPL. Each probe verifies Go rejects the nearest expressible
// boundary before recording the pinned Java message prefix.
func (s *viewGroupClosureCaseState) buildError(step compat.Step) error {
	if pinned, ok := viewGroupClosureProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", viewGroupClosureID, step.Statement, step.Epl)
	}
	if step.ExpectError != viewGroupClosureProbeErrors[step.Statement] {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned prefix %q", viewGroupClosureID, step.Statement, step.ExpectError)
	}
	theString := esper.Field[viewGroupClosureBean, string]("theString")
	var buildErr error
	switch step.Statement {
	case "multiple-groupwin":
		// `#groupwin(theString)#length(1)#groupwin(theString)#uni(...)` —
		// the nearest expressible form nests a second group window inside
		// the first, which the spec validation rejects as multiple
		// group-window declarations.
		_, buildErr = s.env.Build(esper.From[viewGroupClosureBean](s.env, "SupportBean").Window(
			esper.GroupWindow(theString,
				esper.GroupWindow(theString, esper.LengthWindow(1)))).
			Query(esper.StatementName("s0")))
	case "groupwin-no-child":
		// `SupportMarketDataBean#length(100)#groupwin(symbol)` — the
		// groupwin declaration follows the length window and has no child
		// view to group.
		symbol := esper.Field[viewGroupClosureMarket, string]("symbol")
		_, buildErr = s.env.Build(esper.From[viewGroupClosureMarket](s.env, "SupportMarketDataBean").
			Window(esper.LengthWindow(100)).
			Window(esper.GroupWindow(symbol, nil)).
			Query(esper.StatementName("s0")))
	case "groupwin-not-first":
		// `SupportBean#keepall#groupwin(theString)#length(2)` — the
		// groupwin declaration is not the first data-window declaration.
		_, buildErr = s.env.Build(esper.From[viewGroupClosureBean](s.env, "SupportBean").
			Window(esper.KeepAll()).
			Window(esper.GroupWindow(theString, esper.LengthWindow(2))).
			Query(esper.StatementName("s0")))
	case "groupwin-null-field":
		// `create schema MyEvent(somefield null); select * from
		// MyEvent#groupwin(somefield)#length(2)` — Go schemas have no null
		// type (the field registers as any), so the nearest expressible
		// boundary is a null literal group-window key.
		if _, err := esper.RegisterMap(s.env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("somefield", nil),
		}); err != nil {
			return fmt.Errorf("%s: build-error probe %q schema registration failed: %w", viewGroupClosureID, step.Statement, err)
		}
		_, buildErr = s.env.Build(esper.FromAny(s.env, "MyEvent").Window(
			esper.GroupWindow(esper.NullLiteral[any](), esper.LengthWindow(2))).
			Query(esper.StatementName("s0")))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", viewGroupClosureID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", viewGroupClosureID, step.Statement)
	}
	// Verify the Go rejection carries the expected wording before recording
	// the pinned Java prefix.
	expected := map[string]string{
		"multiple-groupwin":   "group-window declarations are not supported",
		"groupwin-no-child":   "requires one or more child views",
		"groupwin-not-first":  "must occur in the first position",
		"groupwin-null-field": "Group-window received a null-typed criteria expression",
	}
	if want, ok := expected[step.Statement]; ok && !strings.Contains(buildErr.Error(), want) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", viewGroupClosureID, step.Statement, buildErr)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// unrepresentable emits the pinned record for the merge-view probe: Go has
// no merge WindowSpec, so the step records the pinned Java prefix without
// claiming a Go rejection boundary (the context_lifecycle.go precedent).
func (s *viewGroupClosureCaseState) unrepresentable(step compat.Step) error {
	if pinned, ok := viewGroupClosureProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned EPL %q", viewGroupClosureID, step.Statement, step.Epl)
	}
	if step.ExpectError != viewGroupClosureProbeErrors[step.Statement] {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned prefix %q", viewGroupClosureID, step.Statement, step.ExpectError)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeployAll undeploys deployments in reverse deploy order, mirroring
// env.undeployAll().
func (s *viewGroupClosureCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", viewGroupClosureID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// loadViewGroupClosureScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned metadata and case metadata, and the pinned
// per-case step keys so unknown or mutated steps fail the replay.
func loadViewGroupClosureScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", viewGroupClosureID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", viewGroupClosureID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewGroupClosureID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewGroupClosureID, err)
	}
	if err := requireViewGroupClosureFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", viewGroupClosureID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != viewGroupClosureID ||
		metadata.Description != viewGroupClosureDescription ||
		metadata.JavaCommit != viewGroupClosureJavaCommit ||
		metadata.JavaSource != viewGroupClosureSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", viewGroupClosureID)
	}
	if err := validateViewGroupClosureStringArray(root["javaRuntimes"], viewGroupClosureJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateViewGroupClosureStringArray(root["javaNames"], viewGroupClosureJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateViewGroupClosureStringArray(root["javaStaticIds"], viewGroupClosureJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateViewGroupClosureStringArray(root["javaFlags"], viewGroupClosureJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(viewGroupClosureCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", viewGroupClosureID, len(viewGroupClosureCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireViewGroupClosureFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string `json:"case"`
			Ordinal       int    `json:"ordinal"`
			RuntimeID     string `json:"runtimeId"`
			ExecutionName string `json:"executionName"`
			Observation   string `json:"observation"`
			EPL           string `json:"epl"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != viewGroupClosureCases[index] ||
			definition.Ordinal != viewGroupClosureOrdinals[index] ||
			definition.RuntimeID != viewGroupClosureJavaRuntimeIDs[index] ||
			definition.ExecutionName != viewGroupClosureJavaExecutions[index] ||
			definition.Observation != viewGroupClosureCaseObservations[index] ||
			definition.EPL != viewGroupClosureCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", viewGroupClosureID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", viewGroupClosureID, err)
	}
	offset := 0
	for _, caseName := range viewGroupClosureCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", viewGroupClosureID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", viewGroupClosureID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", viewGroupClosureID, offset, caseName)
		}
		offset++
		want, ok := viewGroupClosureCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", viewGroupClosureID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", viewGroupClosureID, caseName)
		}
		for _, pinned := range want {
			key, err := viewGroupClosureStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", viewGroupClosureID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", viewGroupClosureID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", viewGroupClosureID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", viewGroupClosureID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", viewGroupClosureID)
	}
	return scenario, nil
}

// viewGroupClosureStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|ids|count
// with the payload, ids and count compacted. Unknown fields on the step
// object are rejected.
func viewGroupClosureStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op                 string          `json:"op"`
		Case               string          `json:"case"`
		Statement          string          `json:"statement"`
		Name               string          `json:"name"`
		EventType          string          `json:"eventType"`
		Epl                string          `json:"epl"`
		Payload            json.RawMessage `json:"payload"`
		ExpectError        string          `json:"expectError"`
		CompileWithoutPath bool            `json:"compileWithoutPath"`
		Mode               string          `json:"mode"`
		IDs                json.RawMessage `json:"ids"`
		Count              json.RawMessage `json:"count"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "mode": true, "ids": true, "count": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	compact := func(value json.RawMessage) (string, error) {
		if len(value) == 0 {
			return "", nil
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, value); err != nil {
			return "", err
		}
		return compacted.String(), nil
	}
	payload, err := compact(step.Payload)
	if err != nil {
		return "", fmt.Errorf("payload: %w", err)
	}
	ids, err := compact(step.IDs)
	if err != nil {
		return "", fmt.Errorf("ids: %w", err)
	}
	count, err := compact(step.Count)
	if err != nil {
		return "", fmt.Errorf("count: %w", err)
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.Mode + "|" + ids + "|" + count, nil
}

// validateViewGroupClosureScenario re-checks a decoded scenario (used when
// the runner receives a scenario decoded by the generic loader path). The
// generic compat.Step op whitelist accepts the unrepresentable op this
// scenario pins, so validation is the id and step-count invariants.
func validateViewGroupClosureScenario(scenario compat.Scenario) error {
	if scenario.ID != viewGroupClosureID {
		return fmt.Errorf("%s scenario id %q is not pinned", viewGroupClosureID, scenario.ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", viewGroupClosureID)
	}
	return nil
}

func requireViewGroupClosureFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", viewGroupClosureID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", viewGroupClosureID, name)
		}
	}
	return nil
}

func validateViewGroupClosureStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
