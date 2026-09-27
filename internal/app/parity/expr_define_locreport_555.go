package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// expr_define_locreport_555.go replays ExprDefineLambdaLocReport
// (java-1333311a1ef60ea35019, java-runtime-867dc2875052889e9df2, ord 0
// direct) as the Draft 4.555 single-execution differential chain. The Java
// execution deploys one EPL module declaring three chained expressions —
// lostLuggage, passengers and nearestOwner — then sends one
// LocationReportFactory.makeLarge() bean (21 items in fixed order) and
// asserts val1 = the separated luggage [L00000, L00007, L00008] and
// val2 = the per-luggage nearest passenger map {L00000->P00008,
// L00007->P00001, L00008->P00001}.
//
// Approved differences (observably identical to the Java EPL):
//   - The Java oracle deploys "@name('s0') <module EPL>"; the Go runner
//     names the fluent query s0. The pinned module EPL is the deployed
//     text verbatim, including the two-space indentation inside each
//     expression body and the empty concatenated string literals.
//   - The declared expressions map to env-level DefineExpression
//     registrations with ExpressionParam/ExpressionRef parameters. Two
//     helper definitions (ownerFar, nearestPassenger) carry the
//     same-typed outer enum element as an explicit parameter: Esper's
//     nested lambdas keep the outer l/value visible inside anyof/minBy,
//     while Go's single enum-element slot shadows same-typed outer
//     elements, so the helpers take (lr, element) and the call sites pass
//     EnumElement[exprDefineLocReport555Item]() — the same correlation.
//   - LRUtil.distance maps to a Func4 over float64 coordinates; Java's
//     Math.sqrt(Math.pow(dx, 2) + Math.pow(dy, 2)) over int inputs is
//     bit-identical to sqrt(dx*dx + dy*dy) in float64.
//   - assetIdPassenger is a *string so Java's null passenger link
//     renders as JSON null inside item rows, matching the oracle's
//     bare-null item rendering.
//   - The send payload carries the full 21-item makeLarge() fixture; the
//     runner requires the decoded payload to equal the pinned fixture
//     before sending, so the load-bearing item order cannot drift.

const exprDefineLocReport555ID = "expr-define-locreport-555"
const exprDefineLocReport555JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const exprDefineLocReport555Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/define/ExprDefineLambdaLocReport.java"
const exprDefineLocReport555Description = "ExprDefineLambdaLocReport (ord 0): the single-module deploy declares lostLuggage/passengers/nearestOwner — lostLuggage(lr) filters L-typed items whose owning passenger sits farther than LRUtil.distance > 20, and nearestOwner(lr) maps each lost item's assetId to the minBy-distance passenger — then one LocationReportFactory.makeLarge() send emits val1=[L00000,L00007,L00008] and val2={L00000->P00008, L00007->P00001, L00008->P00001}. The types record pins the asserted val1 Collection / val2 Map surface (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/define/ExprDefineLambdaLocReport.java)."

// exprDefineLocReport555EPL is the byte-exact transcription of the
// ExprDefineLambdaLocReport.java lines 30-44 string concatenation,
// including the double spaces inside each expression body.
const exprDefineLocReport555EPL = "@name('s0') " +
	"expression lostLuggage {" +
	"  lr => lr.items.where(l => l.type='L' and " +
	"    lr.items.anyof(p => p.type='P' and p.assetId=l.assetIdPassenger and LRUtil.distance(l.location.x, l.location.y, p.location.x, p.location.y) > 20))" +
	"}" +
	"expression passengers {" +
	"  lr => lr.items.where(l => l.type='P')" +
	"}" +
	"" +
	"expression nearestOwner {" +
	"  lr => lostLuggage(lr).toMap(key => key.assetId, " +
	"     value => passengers(lr).minBy(p => LRUtil.distance(value.location.x, value.location.y, p.location.x, p.location.y)))" +
	"}" +
	"" +
	"select lostLuggage(lr) as val1, nearestOwner(lr) as val2 from LocationReport lr"

const exprDefineLocReport555Observation = "types+listener; lostLuggage(lr) " +
	"filters L items whose assetIdPassenger passenger is farther than " +
	"LRUtil.distance > 20, nearestOwner(lr) maps each lost assetId to the " +
	"minBy-distance passenger: the single makeLarge() send emits " +
	"val1=[L00000,L00007,L00008] and " +
	"val2={L00000->P00008, L00007->P00001, L00008->P00001}"

var exprDefineLocReport555JavaSources = []string{exprDefineLocReport555Source}
var exprDefineLocReport555JavaRuntimeIDs = []string{"java-runtime-867dc2875052889e9df2"}
var exprDefineLocReport555JavaExecutions = []string{"ExprDefineLambdaLocReport"}
var exprDefineLocReport555JavaStaticIDs = []string{"java-1333311a1ef60ea35019"}
var exprDefineLocReport555Cases = []string{"locreport"}
var exprDefineLocReport555CaseRuntimeIDs = map[string]string{
	"locreport": "java-runtime-867dc2875052889e9df2",
}

// exprDefineLocReport555Location mirrors lrreport.Location: the int x/y
// coordinates LRUtil.distance consumes.
type exprDefineLocReport555Location struct {
	X int `json:"x" esper:"x"`
	Y int `json:"y" esper:"y"`
}

// exprDefineLocReport555Item mirrors lrreport.Item: assetIdPassenger is a
// *string so the Java null owner link renders as JSON null in item rows.
type exprDefineLocReport555Item struct {
	AssetId          string                         `json:"assetId" esper:"assetId"`
	Location         exprDefineLocReport555Location `json:"location" esper:"location"`
	Type             string                         `json:"type" esper:"type"`
	AssetIdPassenger *string                        `json:"assetIdPassenger" esper:"assetIdPassenger"`
}

// exprDefineLocReport555Report mirrors lrreport.LocationReport: the items
// collection every declared expression enumerates.
type exprDefineLocReport555Report struct {
	Items []exprDefineLocReport555Item `json:"items" esper:"items"`
}

func exprDefineLocReport555Str(value string) *string { return &value }

// makeExprDefineLocReport555Large mirrors LocationReportFactory.makeLarge()
// byte-for-byte: the item order is load-bearing for val1's
// [L00000, L00007, L00008] ordering.
func makeExprDefineLocReport555Large() exprDefineLocReport555Report {
	items := []exprDefineLocReport555Item{
		{AssetId: "P00002", Location: exprDefineLocReport555Location{X: 40, Y: 40}, Type: "P"},
		{AssetId: "L00001", Location: exprDefineLocReport555Location{X: 42, Y: 41}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00002")},
		{AssetId: "L00002", Location: exprDefineLocReport555Location{X: 43, Y: 43}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00002")},
		{AssetId: "P00001", Location: exprDefineLocReport555Location{X: 10, Y: 10}, Type: "P"},
		{AssetId: "L00000", Location: exprDefineLocReport555Location{X: 99, Y: 97}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00001")},
		{AssetId: "P00004", Location: exprDefineLocReport555Location{X: 20, Y: 20}, Type: "P"},
		{AssetId: "P00002", Location: exprDefineLocReport555Location{X: 40, Y: 40}, Type: "P"},
		{AssetId: "L00003", Location: exprDefineLocReport555Location{X: 29, Y: 26}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00004")},
		{AssetId: "E00011", Location: exprDefineLocReport555Location{X: 90, Y: 95}, Type: "P"},
		{AssetId: "A00010", Location: exprDefineLocReport555Location{X: 104, Y: 101}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("E00011")},
		{AssetId: "A00011", Location: exprDefineLocReport555Location{X: 96, Y: 100}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("E00011")},
		{AssetId: "E00010", Location: exprDefineLocReport555Location{X: 90, Y: 95}, Type: "P"},
		{AssetId: "L00009", Location: exprDefineLocReport555Location{X: 102, Y: 101}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("E00010")},
		{AssetId: "P00005", Location: exprDefineLocReport555Location{X: 30, Y: 30}, Type: "P"},
		{AssetId: "L00004", Location: exprDefineLocReport555Location{X: 26, Y: 27}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00005")},
		{AssetId: "L00005", Location: exprDefineLocReport555Location{X: 30, Y: 28}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00005")},
		{AssetId: "P00007", Location: exprDefineLocReport555Location{X: 90, Y: 95}, Type: "P"},
		{AssetId: "L00006", Location: exprDefineLocReport555Location{X: 96, Y: 100}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00007")},
		{AssetId: "P00008", Location: exprDefineLocReport555Location{X: 100, Y: 100}, Type: "P"},
		{AssetId: "L00007", Location: exprDefineLocReport555Location{X: 10, Y: 12}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00008")},
		{AssetId: "L00008", Location: exprDefineLocReport555Location{X: 10, Y: 12}, Type: "L", AssetIdPassenger: exprDefineLocReport555Str("P00008")},
	}
	return exprDefineLocReport555Report{Items: items}
}

// exprDefineLocReport555Distance mirrors LRUtil.distance: Java's
// Math.sqrt(Math.pow(dx, 2) + Math.pow(dy, 2)) over int coordinates is
// bit-identical to the float64 product form for every int input.
func exprDefineLocReport555Distance(x1, y1, x2, y2 float64) float64 {
	dx, dy := x1-x2, y1-y2
	return math.Sqrt(dx*dx + dy*dy)
}

type exprDefineLocReport555CaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequences   map[string]uint64
	statements  map[string]*esper.Statement
	deployments map[string]string
	plans       map[string]esper.Plan
}

func runExprDefineLocReport555Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range exprDefineLocReport555Cases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runExprDefineLocReport555Case(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("expr-define-locreport-555 case %q: %w", caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("expr-define-locreport-555 scenario %q has no supported cases", scenario.ID)
	}
	return trace, nil
}

func runExprDefineLocReport555Case(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprDefineLocReport555Report](env, "LocationReport"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithStartTime(time.Unix(0, 0).UTC()),
		esper.WithRuntimeURI(exprDefineLocReport555CaseRuntimeIDs[caseName]),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &exprDefineLocReport555CaseState{
		caseName:    caseName,
		env:         env,
		engine:      engine,
		trace:       &compat.Trace{Version: scenario.Version, ID: scenario.ID},
		sequences:   make(map[string]uint64),
		statements:  make(map[string]*esper.Statement),
		deployments: make(map[string]string),
		plans:       make(map[string]esper.Plan),
	}
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *state.trace, err
		}
		switch step.Op {
		case "case":
			continue
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *state.trace, err
			}
		case "types":
			if err := state.types(step); err != nil {
				return *state.trace, err
			}
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("expr-define-locreport-555: unsupported step op %q", step.Op)
		}
	}
	return *state.trace, nil
}

// deploy maps the single-module deploy step onto the typed chain. The Java
// module's three declared expressions plus the two same-type-shadowing
// helpers register as env-level DefineExpression definitions; the select
// maps to a fluent query named s0 with lostLuggage(lr)/nearestOwner(lr)
// called on the stream event via ExpressionRef.
func (s *exprDefineLocReport555CaseState) deploy(ctx context.Context, step compat.Step) error {
	if step.Statement != "s0" {
		return fmt.Errorf("expr-define-locreport-555: unknown deploy %q in case %q", step.Statement, s.caseName)
	}
	if step.Epl != exprDefineLocReport555EPL {
		return fmt.Errorf("expr-define-locreport-555: deploy %q carries an unpinned EPL %q", step.Statement, step.Epl)
	}
	env := s.env
	lr := esper.ExpressionParam[exprDefineLocReport555Report]("lr")
	items := esper.Property[[]exprDefineLocReport555Item](lr, "items")
	itemElement := func() esper.Expression[exprDefineLocReport555Item] {
		return esper.EnumElement[exprDefineLocReport555Item]()
	}
	elementLocation := func(name string) esper.Expression[float64] {
		return esper.Property[float64](itemElement(), "location."+name)
	}
	itemTypeIs := func(want string) esper.Expression[bool] {
		return esper.Equal[string](esper.EnumField[exprDefineLocReport555Item, string]("type"), esper.Literal(want))
	}

	// ownerFar { (lr, l) => lr.items.anyof(p => p.type='P' and
	//   p.assetId=l.assetIdPassenger and LRUtil.distance(l.location, p.location) > 20) }
	// — the anyof inner lambda shadows the same-typed outer l, so the outer
	// element arrives as an explicit declared-expression parameter.
	l := esper.ExpressionParam[exprDefineLocReport555Item]("l")
	ownerFar := esper.EnumAnyOf[exprDefineLocReport555Item](items,
		esper.And(
			esper.And(
				itemTypeIs("P"),
				esper.Equal[string](
					esper.EnumField[exprDefineLocReport555Item, string]("assetId"),
					esper.Property[string](l, "assetIdPassenger")),
			),
			esper.Greater[float64](
				esper.Func4[float64, float64, float64, float64, float64]("LRUtil.distance", exprDefineLocReport555Distance,
					esper.Property[float64](l, "location.x"), esper.Property[float64](l, "location.y"),
					elementLocation("x"), elementLocation("y")),
				esper.Literal(float64(20))),
		))
	if err := env.DefineExpression("ownerFar", ownerFar); err != nil {
		return fmt.Errorf("expr-define-locreport-555: define ownerFar: %w", err)
	}

	// lostLuggage { lr => lr.items.where(l => l.type='L' and ownerFar(lr, l)) }
	lostLuggage := esper.EnumWhere[exprDefineLocReport555Item](items,
		esper.And(
			itemTypeIs("L"),
			esper.ExpressionRef[bool](env, "ownerFar", lr, itemElement())))
	if err := env.DefineExpression("lostLuggage", lostLuggage); err != nil {
		return fmt.Errorf("expr-define-locreport-555: define lostLuggage: %w", err)
	}

	// passengers { lr => lr.items.where(l => l.type='P') }
	passengers := esper.EnumWhere[exprDefineLocReport555Item](items, itemTypeIs("P"))
	if err := env.DefineExpression("passengers", passengers); err != nil {
		return fmt.Errorf("expr-define-locreport-555: define passengers: %w", err)
	}

	// nearestPassenger { (lr, v) => passengers(lr).minBy(p =>
	//   LRUtil.distance(v.location, p.location)) } — same shadowing bridge as
	// ownerFar for the toMap value lambda's outer element.
	v := esper.ExpressionParam[exprDefineLocReport555Item]("v")
	nearestPassenger := esper.EnumMinBy[exprDefineLocReport555Item, float64](
		esper.ExpressionRef[[]exprDefineLocReport555Item](env, "passengers", lr),
		esper.Func4[float64, float64, float64, float64, float64]("LRUtil.distance", exprDefineLocReport555Distance,
			esper.Property[float64](v, "location.x"), esper.Property[float64](v, "location.y"),
			elementLocation("x"), elementLocation("y")))
	if err := env.DefineExpression("nearestPassenger", nearestPassenger); err != nil {
		return fmt.Errorf("expr-define-locreport-555: define nearestPassenger: %w", err)
	}

	// nearestOwner { lr => lostLuggage(lr).toMap(key => key.assetId,
	//   value => nearestPassenger(lr, value)) }
	nearestOwner := esper.EnumToMap[exprDefineLocReport555Item, string, exprDefineLocReport555Item](
		esper.ExpressionRef[[]exprDefineLocReport555Item](env, "lostLuggage", lr),
		esper.EnumField[exprDefineLocReport555Item, string]("assetId"),
		esper.ExpressionRef[exprDefineLocReport555Item](env, "nearestPassenger", lr, itemElement()))
	if err := env.DefineExpression("nearestOwner", nearestOwner); err != nil {
		return fmt.Errorf("expr-define-locreport-555: define nearestOwner: %w", err)
	}

	plan, err := env.Build(esper.Select(
		esper.From[exprDefineLocReport555Report](env, "LocationReport"),
		esper.Alias("val1", esper.ExpressionRef[[]exprDefineLocReport555Item](env, "lostLuggage", esper.EventValue[exprDefineLocReport555Report]())),
		esper.Alias("val2", esper.ExpressionRef[map[string]exprDefineLocReport555Item](env, "nearestOwner", esper.EventValue[exprDefineLocReport555Report]())),
	).Query(esper.StatementName("s0")))
	if err != nil {
		return fmt.Errorf("expr-define-locreport-555: deploy %q: %w", step.Statement, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("expr-define-locreport-555: deploy %q: %w", step.Statement, err)
	}
	for _, statement := range deployment.Statements() {
		s.statements[step.Statement] = statement
		s.deployments[step.Statement] = statement.DeploymentID()
		s.plans[step.Statement] = plan
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			s.sequences["listener:"+stmt.Name()]++
			s.trace.Records = append(s.trace.Records, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: stmt.Name(),
				Sequence:  s.sequences["listener:"+stmt.Name()],
				Time:      compat.FormatTraceTime(s.engine.Now()),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// types pins the asserted select-clause property surface: the Java
// execution's listener asserts val1 Collection<Item> and val2 Map. The Go
// result schema is verified against the equivalent shape before the pinned
// Java type names are recorded (convention:
// context_key_segmented_subselect_prev_prior.go types).
func (s *exprDefineLocReport555CaseState) types(step compat.Step) error {
	if step.Statement != "s0" {
		return fmt.Errorf("expr-define-locreport-555: unknown types statement %q", step.Statement)
	}
	plan, ok := s.plans[step.Statement]
	if !ok {
		return fmt.Errorf("expr-define-locreport-555: types statement %q was not deployed", step.Statement)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		return fmt.Errorf("expr-define-locreport-555: statement %q has no result schema", step.Statement)
	}
	val1, exists := schema.Field("val1")
	if !exists || val1.Type != reflect.TypeOf([]exprDefineLocReport555Item{}) {
		return fmt.Errorf("expr-define-locreport-555: s0 val1 drift: %v", val1.Type)
	}
	val2, exists := schema.Field("val2")
	if !exists || val2.Type != reflect.TypeOf(map[string]exprDefineLocReport555Item{}) {
		return fmt.Errorf("expr-define-locreport-555: s0 val2 drift: %v", val2.Type)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value: map[string]any{
			"properties": map[string]any{"val1": "Collection", "val2": "Map"},
		},
	})
	return nil
}

// send decodes the pinned makeLarge() payload and requires it to equal the
// fixture exactly — the item order is load-bearing for val1's ordering.
func (s *exprDefineLocReport555CaseState) send(ctx context.Context, step compat.Step) error {
	if step.EventType != "LocationReport" {
		return fmt.Errorf("expr-define-locreport-555: unknown event type %q", step.EventType)
	}
	report, err := exprDefineLocReport555DecodePayload(step)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(report, makeExprDefineLocReport555Large()) {
		return fmt.Errorf("expr-define-locreport-555: LocationReport payload drifted from makeLarge()")
	}
	return s.engine.Send(ctx, step.EventType, report)
}

func (s *exprDefineLocReport555CaseState) undeployAll(ctx context.Context) error {
	for name, deploymentID := range s.deployments {
		if err := s.engine.Undeploy(ctx, deploymentID); err != nil {
			return fmt.Errorf("expr-define-locreport-555: undeploy-all %q: %w", name, err)
		}
	}
	s.statements = make(map[string]*esper.Statement)
	s.deployments = make(map[string]string)
	s.plans = make(map[string]esper.Plan)
	return nil
}

// exprDefineLocReport555DecodePayload converts the send payload into the
// typed LocationReport fixture. Every item requires the full Item property
// set; assetIdPassenger stays a *string so the Java null link survives.
func exprDefineLocReport555DecodePayload(step compat.Step) (exprDefineLocReport555Report, error) {
	var payload struct {
		Items []exprDefineLocReport555Item `json:"items"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return exprDefineLocReport555Report{}, fmt.Errorf("expr-define-locreport-555: decode LocationReport payload: %w", err)
	}
	return exprDefineLocReport555Report{Items: payload.Items}, nil
}

// loadExprDefineLocReport555Scenario decodes the checked-in scenario with
// the strict contract shared by the differential runners: no duplicate or
// unknown JSON fields, pinned metadata, pinned case identity and EPL, a
// pinned five-step schedule, and a per-op step field whitelist.
func loadExprDefineLocReport555Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read expr-define-locreport-555 scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode expr-define-locreport-555 scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode expr-define-locreport-555 scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario is missing field %q", name)
		}
	}
	for name, want := range map[string]string{
		"version":     compat.ScenarioVersion,
		"id":          exprDefineLocReport555ID,
		"description": exprDefineLocReport555Description,
		"javaCommit":  exprDefineLocReport555JavaCommit,
		"javaSource":  exprDefineLocReport555Source,
	} {
		var got string
		if err := json.Unmarshal(root[name], &got); err != nil || got != want {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario field %q is not pinned", name)
		}
	}
	for name, want := range map[string][]string{
		"javaRuntimes":  exprDefineLocReport555JavaRuntimeIDs,
		"javaNames":     exprDefineLocReport555JavaExecutions,
		"javaStaticIds": exprDefineLocReport555JavaStaticIDs,
		"javaFlags":     {},
	} {
		var got []string
		if err := json.Unmarshal(root[name], &got); err != nil || !equalStrings(got, want) {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario field %q is not pinned", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
		Observation   string `json:"observation"`
		Epl           string `json:"epl"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode expr-define-locreport-555 cases: %w", err)
	}
	if len(cases) != len(exprDefineLocReport555Cases) {
		return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario must contain exactly %d cases", len(exprDefineLocReport555Cases))
	}
	for index, entry := range cases {
		if entry.Case != exprDefineLocReport555Cases[index] ||
			entry.Ordinal != 0 ||
			entry.RuntimeID != exprDefineLocReport555CaseRuntimeIDs[entry.Case] ||
			entry.ExecutionName != exprDefineLocReport555JavaExecutions[index] ||
			entry.Observation != exprDefineLocReport555Observation ||
			entry.Epl != exprDefineLocReport555EPL {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode expr-define-locreport-555 steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"types":        {"op", "case", "statement"},
		"send":         {"op", "case", "eventType", "payload"},
		"undeploy-all": {"op", "case"},
	}
	expectedOps := []string{"case", "deploy", "types", "send", "undeploy-all"}
	if len(rawSteps) != len(expectedOps) {
		return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 scenario must contain exactly %d steps, found %d", len(expectedOps), len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d op: %w", index, err)
		}
		if op != expectedOps[index] {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d has unpinned op %q", index, op)
		}
		allowed := stepFields[op]
		for field := range object {
			found := false
			for _, name := range allowed {
				if field == name {
					found = true
					break
				}
			}
			if !found {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d case: %w", index, err)
		}
		if stepCase != exprDefineLocReport555Cases[0] {
			return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d has unknown case %q", index, stepCase)
		}
		switch op {
		case "deploy":
			var epl string
			if err := json.Unmarshal(object["epl"], &epl); err != nil {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d epl: %w", index, err)
			}
			if epl != exprDefineLocReport555EPL {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d carries an unpinned EPL", index)
			}
		case "types":
			var statement string
			if err := json.Unmarshal(object["statement"], &statement); err != nil {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d statement: %w", index, err)
			}
			if statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d has unpinned types statement %q", index, statement)
			}
		case "send":
			var eventType string
			if err := json.Unmarshal(object["eventType"], &eventType); err != nil {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d eventType: %w", index, err)
			}
			if eventType != "LocationReport" {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d has unknown event type %q", index, eventType)
			}
			var payload map[string]json.RawMessage
			if err := strictObject(object["payload"], &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d payload: %w", index, err)
			}
			if len(payload) != 1 {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d payload has unexpected fields", index)
			}
			if _, ok := payload["items"]; !ok {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d payload is missing items", index)
			}
			var items []map[string]json.RawMessage
			if err := json.Unmarshal(payload["items"], &items); err != nil {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d items: %w", index, err)
			}
			if len(items) != len(makeExprDefineLocReport555Large().Items) {
				return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d payload item count is not pinned", index)
			}
			itemFields := map[string]bool{"assetId": true, "location": true, "type": true, "assetIdPassenger": true}
			for itemIndex, item := range items {
				if len(item) != len(itemFields) {
					return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d item %d has unexpected fields", index, itemIndex)
				}
				for field := range item {
					if !itemFields[field] {
						return compat.Scenario{}, fmt.Errorf("expr-define-locreport-555 step %d item %d has unexpected field %q", index, itemIndex, field)
					}
				}
			}
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode expr-define-locreport-555 scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
