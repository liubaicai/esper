package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for the ExprDefineValueParameter slice (ords 7, 9 and 11 —
// the file's three remaining unreferenced executions).
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 7 ExprDefineValueParameterEVEVE
//     java-runtime-d1b67308d74aec6b3092
//   - ord 9 ExprDefineValueParameterCache (STATICHOOK)
//     java-runtime-65eb95bacf881252faa4
//   - ord 11 ExprDefineValueParameterSubquery
//     java-runtime-45b280bc6b857cf5fa3a
//
// EVEVE registers the five-parameter declared expression cc env-level
// (DefineExpression with three Event parameters read through NestedField and
// two string parameters) and replays the three-statement module over three
// filtered SupportBean_S0#lastevent join sources; the JoinEventValue
// arguments pin the alias rebinding across s0/s1/s2. Cache maps the
// module-local `create variable ExprDefineLocalService myService` onto a
// runner-local service registered through env.RegisterVariable and invokes
// Calc through Method over VariableRef inside the doit expression; the
// iterator-count steps pin the Java getCalculations().size() assertions.
// Subquery binds the statement-local cc through Query.WithExpression and
// feeds it two SubqueryValue arguments over SupportBean_S0#lastevent.
const exprDefineValueParameterID = "expr-define-value-parameter"
const exprDefineValueParameterJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const exprDefineValueParameterSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/define/ExprDefineValueParameter.java"
const exprDefineValueParameterDescription = "ExprDefineValueParameter slice (ords 7/9/11): eveve deploys @public create expression cc { (a,v1,b,v2,c) -> a.p00 || v1 || b.p00 || v2 || c.p00} on the shared path, then one module deploys s0/s1/s2 over three filtered SupportBean_S0#lastevent streams — s0 asserts c0 String and the third send fires all three joins emitting 'BxCyA'/'BxAyC'/'CxByA'; cache deploys create variable ExprDefineLocalService myService plus create expression doit {v -> myService.calc(v)} feeding select doit(theString) as c0 — each SupportBean('E10',-1) emits c0=10 and the service's calculations list grows 1 then 2 (STATICHOOK is metadata-only); subquery binds statement-local expression cc {(v1,v2) -> v1||v2} over two scalar subqueries on SupportBean_S0#lastevent — one SupportBean_S1(0) send with no S0 events emits c0=null. Deployed markers pin the module fan-out, listener records carry the new-data rows, the types record pins the asserted c0 String type, and iterator-count records pin the service invocation counts (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/define/ExprDefineValueParameter.java)."

// Byte-exact EPL pins (ExprDefineValueParameter.java lines 53-54, 185-194,
// 218-220). The eveve module and the cache module keep their multi-statement
// text exactly as the Java source concatenates it.
const (
	exprDefineValueParameterExprCC = "@public create expression cc { (a,v1,b,v2,c) -> a.p00 || v1 || b.p00 || v2 || c.p00}"
	exprDefineValueParameterModule = "@name('s0') select cc(e2, 'x', e3, 'y', e1) as c0 from \n" +
		"SupportBean_S0(id=1)#lastevent as e1, SupportBean_S0(id=2)#lastevent as e2, SupportBean_S0(id=3)#lastevent as e3;\n" +
		"@name('s1') select cc(e2, 'x', e3, 'y', e1) as c0 from \n" +
		"SupportBean_S0(id=1)#lastevent as e3, SupportBean_S0(id=2)#lastevent as e2, SupportBean_S0(id=3)#lastevent as e1;\n" +
		"@name('s2') select cc(e1, 'x', e2, 'y', e3) as c0 from \n" +
		"SupportBean_S0(id=1)#lastevent as e3, SupportBean_S0(id=2)#lastevent as e2, SupportBean_S0(id=3)#lastevent as e1;\n"
	exprDefineValueParameterCacheModule = "create variable ExprDefineLocalService myService = new ExprDefineLocalService();\n" +
		"create expression doit {v -> myService.calc(v)};\n" +
		"@name('s0') select doit(theString) as c0 from SupportBean;\n"
	exprDefineValueParameterSubquery = "@name('s0') expression cc { (v1, v2) -> v1 || v2} " +
		"select cc((select p00 from SupportBean_S0#lastevent), (select p01 from SupportBean_S0#lastevent)) as c0 from SupportBean_S1"
)

var (
	exprDefineValueParameterJavaSources = []string{
		exprDefineValueParameterSource,
	}
	exprDefineValueParameterJavaRuntimeIDs = []string{
		"java-runtime-d1b67308d74aec6b3092",
		"java-runtime-65eb95bacf881252faa4",
		"java-runtime-45b280bc6b857cf5fa3a",
	}
	exprDefineValueParameterJavaExecutions = []string{
		"ExprDefineValueParameterEVEVE",
		"ExprDefineValueParameterCache",
		"ExprDefineValueParameterSubquery",
	}
	exprDefineValueParameterJavaStaticIDs = []string{
		"java-2f93b37cf7c6dc83e2dd",
		"java-c325db71b15aab20ac8e",
		"java-a816df5a959d4e4aa7a2",
	}
	exprDefineValueParameterJavaFlags = []string{"STATICHOOK"}
	exprDefineValueParameterCases     = []string{
		"eveve",
		"cache",
		"subquery",
	}
	exprDefineValueParameterOrdinals = []int{7, 9, 11}
)

// exprDefineValueParameterCaseSpec pins one case: identity and the case-level
// observation/EPL the scenario repeats.
type exprDefineValueParameterCaseSpec struct {
	name        string
	ordinal     int
	runtimeID   string
	execution   string
	observation string
	epl         string
}

var exprDefineValueParameterCaseSpecs = []exprDefineValueParameterCaseSpec{
	{
		name:      "eveve",
		ordinal:   7,
		runtimeID: "java-runtime-d1b67308d74aec6b3092",
		execution: "ExprDefineValueParameterEVEVE",
		observation: "deployed+types+listener; @public create expression cc" +
			" { (a,v1,b,v2,c) -> a.p00 || v1 || b.p00 || v2 || c.p00} deploys on the" +
			" shared path, then one module deploys s0/s1/s2 over three filtered" +
			" SupportBean_S0#lastevent streams; s0 asserts c0 String; S0(1,'A') and" +
			" S0(3,'C') stay silent while S0(2,'B') completes the join and fires" +
			" s0='BxCyA', s1='BxAyC', s2='CxByA'",
		epl: exprDefineValueParameterExprCC + ";\n" + exprDefineValueParameterModule,
	},
	{
		name:      "cache",
		ordinal:   9,
		runtimeID: "java-runtime-65eb95bacf881252faa4",
		execution: "ExprDefineValueParameterCache",
		observation: "deployed+listener+iterator-count; create variable" +
			" ExprDefineLocalService myService plus create expression doit" +
			" {v -> myService.calc(v)} feed select doit(theString) as c0: each" +
			" SupportBean('E10',-1) emits c0=10 and the service's calculations list" +
			" grows 1 then 2 (STATICHOOK: the services registry is metadata-only)",
		epl: exprDefineValueParameterCacheModule,
	},
	{
		name:      "subquery",
		ordinal:   11,
		runtimeID: "java-runtime-45b280bc6b857cf5fa3a",
		execution: "ExprDefineValueParameterSubquery",
		observation: "deployed+listener; statement-local expression cc" +
			" { (v1, v2) -> v1 || v2} receives two scalar subqueries over" +
			" SupportBean_S0#lastevent projecting p00 and p01; one" +
			" SupportBean_S1(0) send with no S0 events emits c0=null (null||null)",
		epl: exprDefineValueParameterSubquery + ";\n",
	},
}

// exprDefineValueParameterBean mirrors the SupportBean surface the cache
// select reads: theString feeds doit(theString) and intPrimitive rides along
// as the pinned -1.
type exprDefineValueParameterBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int32  `esper:"intPrimitive"`
}

// exprDefineValueParameterS0 mirrors SupportBean_S0: the id filter picks the
// join source and p00/p01 feed the expression arguments and subqueries.
type exprDefineValueParameterS0 struct {
	ID  int32  `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

// exprDefineValueParameterS1 mirrors SupportBean_S1: only the id is pinned.
type exprDefineValueParameterS1 struct {
	ID int32 `esper:"id"`
}

// exprDefineValueParameterLocalService mirrors ExprDefineLocalService: calc
// records each invocation and returns the digits after the leading 'E'
// (Integer.parseInt(value.substring(1))). The runner registers one instance
// per cache replay so the iterator-count steps observe the same sizes the
// Java execution asserts on services.get(0).getCalculations().
type exprDefineValueParameterLocalService struct {
	calculations int
}

func (s *exprDefineValueParameterLocalService) Calc(value string) int32 {
	s.calculations++
	parsed, err := strconv.Atoi(value[1:])
	if err != nil {
		// Java propagates NumberFormatException through the statement
		// exception handler; a panic unwinds to the same missing result.
		panic(err)
	}
	return int32(parsed)
}

func exprDefineValueParameterCaseSpecFor(name string) (exprDefineValueParameterCaseSpec, bool) {
	for _, spec := range exprDefineValueParameterCaseSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return exprDefineValueParameterCaseSpec{}, false
}

// runExprDefineValueParameterScenario replays the three cases, one fresh
// environment and engine per case (each Java execution gets its own runtime
// and ends with undeployAll).
func runExprDefineValueParameterScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", exprDefineValueParameterID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, spec := range exprDefineValueParameterCaseSpecs {
		caseTrace, err := runExprDefineValueParameterCase(ctx, scenario, spec)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", exprDefineValueParameterID, spec.name, err)
		}
		trace.Records = append(trace.Records, caseTrace...)
	}
	return trace, nil
}

// exprDefineValueParameterCaseState carries one case's replay state: the live
// deployments keyed by scenario label (the module labels hold one deployment
// per contained statement), the built plans for the types probe, the service
// instance for the cache case, the listener sequence counters and the
// emitted records.
type exprDefineValueParameterCaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	caseName    string
	spec        exprDefineValueParameterCaseSpec
	deployments map[string][]*esper.Deployment
	deployOrder []string
	deployed    map[string]bool
	plans       map[string]esper.Plan
	sequences   map[string]uint64
	service     *exprDefineValueParameterLocalService
	pending     []compat.TraceRecord
	records     []compat.TraceRecord
}

func runExprDefineValueParameterCase(ctx context.Context, scenario compat.Scenario, spec exprDefineValueParameterCaseSpec) ([]compat.TraceRecord, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[exprDefineValueParameterBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[exprDefineValueParameterS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[exprDefineValueParameterS1](env, "SupportBean_S1"); err != nil {
		return nil, err
	}
	state := &exprDefineValueParameterCaseState{
		env:         env,
		caseName:    spec.name,
		spec:        spec,
		deployments: map[string][]*esper.Deployment{},
		deployed:    map[string]bool{},
		plans:       map[string]esper.Plan{},
		sequences:   map[string]uint64{},
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(spec.runtimeID),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = state.engine.Close(context.Background()) }()
	defer func() { _ = state.undeployAll(context.Background()) }()

	inCase := false
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			inCase = step.Case == spec.name
			continue
		}
		if !inCase {
			continue
		}
		var err error
		switch step.Op {
		case "deploy":
			err = state.deploy(ctx, step)
		case "deployed":
			err = state.deployedMarker(step)
		case "send":
			err = state.send(ctx, step)
		case "types":
			err = state.types(step)
		case "iterator-count":
			err = state.iteratorCount(step)
		case "undeploy-all":
			err = state.undeployAll(ctx)
		default:
			err = fmt.Errorf("unsupported step op %q", step.Op)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(state.records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", spec.name)
	}
	return state.records, nil
}

// deploy maps one scenario deploy step onto the typed chain. Java's
// create-expression statement registers the expression env-level and deploys
// a placeholder select so the deployed marker pins the module fan-out; the
// module labels deploy each contained statement as its own Go plan (the Go
// engine deploys one query per plan) while the marker stays one record per
// Java deploy call.
func (s *exprDefineValueParameterCaseState) deploy(ctx context.Context, step compat.Step) error {
	plans, err := s.buildPlans(step.Statement)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", exprDefineValueParameterID, step.Statement, err)
	}
	for _, plan := range plans {
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: deploy %q: %w", exprDefineValueParameterID, step.Statement, err)
		}
		s.deployments[step.Statement] = append(s.deployments[step.Statement], deployment)
		for _, statement := range deployment.Statements() {
			if !exprDefineValueParameterListened(s.caseName, statement.Name()) {
				continue
			}
			name := statement.Name()
			if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				s.record(name, batch)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	s.deployOrder = append(s.deployOrder, step.Statement)
	s.deployed[step.Statement] = true
	return nil
}

func exprDefineValueParameterListened(caseName, statement string) bool {
	switch caseName {
	case "eveve":
		return statement == "s0" || statement == "s1" || statement == "s2"
	case "cache", "subquery":
		return statement == "s0"
	default:
		return false
	}
}

// record buffers one listener record for the in-flight send. Java's join
// dispatch fires the last-deployed statement first (the eveve send fires
// s2, s1, s0), so send flushes the buffer in reverse attach order.
func (s *exprDefineValueParameterCaseState) record(statement string, batch esper.ResultBatch) {
	s.sequences[statement]++
	rec := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "listener",
		Statement: statement,
		Sequence:  s.sequences[statement],
		Time:      compat.FormatTraceTime(batch.Time),
		New:       compat.NormalizeResults(batch.New),
		Old:       compat.NormalizeResults(batch.Old),
	}
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.pending = append(s.pending, rec)
}

func (s *exprDefineValueParameterCaseState) deployedMarker(step compat.Step) error {
	if !s.deployed[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", exprDefineValueParameterID, step.Statement)
	}
	s.sequences[step.Statement+":deployed"]++
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequences[step.Statement+":deployed"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

func (s *exprDefineValueParameterCaseState) send(ctx context.Context, step compat.Step) error {
	payload, err := exprDefineValueParameterDecodePayload(step)
	if err != nil {
		return err
	}
	if err := s.engine.Send(ctx, step.EventType, payload); err != nil {
		return err
	}
	// Java's join dispatch fires the last-deployed statement first, so the
	// buffered listener records emit in reverse attach order.
	for index := len(s.pending) - 1; index >= 0; index-- {
		s.records = append(s.records, s.pending[index])
	}
	s.pending = nil
	return nil
}

// undeployAll removes deployments in reverse deploy order, draining every
// statement a module label deployed, mirroring undeployAll.
func (s *exprDefineValueParameterCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		for _, deployment := range s.deployments[label] {
			if err := deployment.Undeploy(ctx); err != nil {
				return fmt.Errorf("%s: undeploy-all %q: %w", exprDefineValueParameterID, label, err)
			}
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// types pins the asserted select-clause property surface: EVEVE's
// assertTypeExpected(env, String.class) checks c0 on s0. The Go result
// schema is verified against the equivalent shape before the pinned Java
// names are recorded (convention:
// context_key_segmented_subselect_prev_prior.go types).
func (s *exprDefineValueParameterCaseState) types(step compat.Step) error {
	if s.caseName != "eveve" || step.Statement != "s0" {
		return fmt.Errorf("%s: unknown types statement %q in case %q", exprDefineValueParameterID, step.Statement, s.caseName)
	}
	plan, ok := s.plans[step.Statement]
	if !ok {
		return fmt.Errorf("%s: types statement %q was not deployed", exprDefineValueParameterID, step.Statement)
	}
	schema, ok := plan.ResultSchema()
	if !ok {
		return fmt.Errorf("%s: statement %q has no result schema", exprDefineValueParameterID, step.Statement)
	}
	c0, exists := schema.Field("c0")
	if !exists || c0.Type != reflect.TypeOf("") {
		return fmt.Errorf("%s: s0 c0 drift: %v", exprDefineValueParameterID, c0.Type)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Sequence:  0,
		Time:      compat.FormatTraceTime(s.engine.Now()),
		Value: map[string]any{
			"properties": map[string]any{"c0": "String"},
		},
	})
	return nil
}

// iteratorCount emits one count record for the pinned
// service.getCalculations().size() assertions: the step's declared count is
// the Java-asserted value and the record carries the observed invocation
// count of the runner-local service (convention: context_lifecycle.go
// countRecord). The statement field pins s0, the select whose doit
// evaluations drive the counter.
func (s *exprDefineValueParameterCaseState) iteratorCount(step compat.Step) error {
	if s.caseName != "cache" || s.service == nil || step.Statement != "s0" {
		return fmt.Errorf("%s: iterator-count step outside the cache case", exprDefineValueParameterID)
	}
	count := int64(s.service.calculations)
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "iterator-count",
		Statement: "s0",
		Count:     &count,
	})
	return nil
}

// buildPlans maps one scenario deploy label onto the typed chain. The module
// labels return one plan per contained statement in module order.
func (s *exprDefineValueParameterCaseState) buildPlans(label string) ([]esper.Plan, error) {
	env := s.env
	beanSource := esper.From[exprDefineValueParameterBean](env, "SupportBean")
	s0Source := esper.From[exprDefineValueParameterS0](env, "SupportBean_S0")
	placeholder := func() (esper.Plan, error) {
		// Java's create-expression statement occupies the statement slot
		// without a select; the placeholder deploys a silent select * so
		// the deployed marker pins the module fan-out.
		return env.Build(esper.Select(beanSource).Query(esper.StatementName(label)))
	}
	switch s.caseName {
	case "eveve":
		switch label {
		case "cc":
			// @public create expression cc { (a,v1,b,v2,c) -> a.p00 || v1 ||
			// b.p00 || v2 || c.p00}
			if err := esper.DefineExpression[string](env, "cc", esper.Concat(
				esper.NestedField[string](esper.ExpressionParam[esper.Event]("a"), "p00"),
				esper.ExpressionParam[string]("v1"),
				esper.NestedField[string](esper.ExpressionParam[esper.Event]("b"), "p00"),
				esper.ExpressionParam[string]("v2"),
				esper.NestedField[string](esper.ExpressionParam[esper.Event]("c"), "p00"),
			)); err != nil {
				return nil, err
			}
			plan, err := placeholder()
			if err != nil {
				return nil, err
			}
			return []esper.Plan{plan}, nil
		case "module":
			// One Java module deploys s0/s1/s2 over the same three filtered
			// #lastevent sources (source 0 = id=1, source 1 = id=2, source 2
			// = id=3); only the alias-to-source binding changes per
			// statement, so the cc event arguments pin different positions.
			id := esper.Field[exprDefineValueParameterS0, int32]("id")
			sources := []esper.JoinInput{
				esper.JoinSource(s0Source.Filter(esper.Equal[int32](id, esper.Literal(int32(1)))).Window(esper.LastEvent())),
				esper.JoinSource(s0Source.Filter(esper.Equal[int32](id, esper.Literal(int32(2)))).Window(esper.LastEvent())),
				esper.JoinSource(s0Source.Filter(esper.Equal[int32](id, esper.Literal(int32(3)))).Window(esper.LastEvent())),
			}
			plans := make([]esper.Plan, 0, 3)
			for _, stmt := range []struct {
				name    string
				a, b, c int
			}{
				// s0: cc(e2,'x',e3,'y',e1) with e1=src0, e2=src1, e3=src2
				{"s0", 1, 2, 0},
				// s1: cc(e2,'x',e3,'y',e1) with e3=src0, e2=src1, e1=src2
				{"s1", 1, 0, 2},
				// s2: cc(e1,'x',e2,'y',e3) with e3=src0, e2=src1, e1=src2
				{"s2", 2, 1, 0},
			} {
				plan, err := env.Build(esper.JoinMany(sources...).
					Select(esper.SelectFrom(0, "c0", esper.ExpressionRef[string](env, "cc",
						esper.JoinEventValue[esper.Event](stmt.a),
						esper.Literal("x"),
						esper.JoinEventValue[esper.Event](stmt.b),
						esper.Literal("y"),
						esper.JoinEventValue[esper.Event](stmt.c),
					))).Query(esper.StatementName(stmt.name)))
				if err != nil {
					return nil, err
				}
				s.plans[stmt.name] = plan
				plans = append(plans, plan)
			}
			return plans, nil
		}
	case "cache":
		if label == "module" {
			// create variable ExprDefineLocalService myService = new
			// ExprDefineLocalService() — the runner-local service instance
			// plays the variable role so the service-count steps observe
			// the same invocation list the Java execution asserts.
			s.service = &exprDefineValueParameterLocalService{}
			if err := env.RegisterVariable("myService", s.service); err != nil {
				return nil, err
			}
			// create expression doit {v -> myService.calc(v)}
			if err := esper.DefineExpression[int32](env, "doit", esper.Method[int32](
				esper.VariableRef[*exprDefineValueParameterLocalService]("myService"),
				"Calc",
				esper.ExpressionParam[string]("v"),
			)); err != nil {
				return nil, err
			}
			// @name('s0') select doit(theString) as c0 from SupportBean
			plan, err := env.Build(esper.Select(beanSource,
				esper.Alias("c0", esper.ExpressionRef[int32](env, "doit",
					esper.Field[exprDefineValueParameterBean, string]("theString"))),
			).Query(esper.StatementName("s0")))
			if err != nil {
				return nil, err
			}
			return []esper.Plan{plan}, nil
		}
	case "subquery":
		if label == "s0" {
			// @name('s0') expression cc { (v1, v2) -> v1 || v2} select
			// cc((select p00 from SupportBean_S0#lastevent), (select p01
			// from SupportBean_S0#lastevent)) as c0 from SupportBean_S1 —
			// the statement-local declaration binds through WithExpression.
			plan, err := env.Build(esper.Select(
				esper.From[exprDefineValueParameterS1](env, "SupportBean_S1"),
				esper.Alias("c0", esper.ExpressionRef[string](env, "cc",
					esper.SubqueryValue[string](
						esper.FromAny(env, "SupportBean_S0").Window(esper.LastEvent()),
						esper.Field[any, string]("p00")),
					esper.SubqueryValue[string](
						esper.FromAny(env, "SupportBean_S0").Window(esper.LastEvent()),
						esper.Field[any, string]("p01")),
				)),
			).Query(esper.StatementName("s0")).
				WithExpression("cc", esper.Concat(
					esper.ExpressionParam[string]("v1"),
					esper.ExpressionParam[string]("v2"),
				)))
			if err != nil {
				return nil, err
			}
			return []esper.Plan{plan}, nil
		}
	}
	return nil, fmt.Errorf("%s: case %q has no deploy fixture for %q",
		exprDefineValueParameterID, s.caseName, label)
}

// exprDefineValueParameterDecodePayload converts a send payload into the
// typed host event. SupportBean carries theString/intPrimitive; SupportBean_S0
// requires id and defaults p00/p01 to the Java null-equivalent empty string;
// SupportBean_S1 requires id.
func exprDefineValueParameterDecodePayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var payload struct {
			TheString    *string `json:"theString"`
			IntPrimitive *int32  `json:"intPrimitive"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		bean := exprDefineValueParameterBean{}
		if payload.TheString != nil {
			bean.TheString = *payload.TheString
		}
		if payload.IntPrimitive != nil {
			bean.IntPrimitive = *payload.IntPrimitive
		}
		return bean, nil
	case "SupportBean_S0":
		var payload struct {
			ID  *int32  `json:"id"`
			P00 *string `json:"p00"`
			P01 *string `json:"p01"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		if payload.ID == nil {
			return nil, fmt.Errorf("SupportBean_S0 payload requires id")
		}
		bean := exprDefineValueParameterS0{ID: *payload.ID}
		if payload.P00 != nil {
			bean.P00 = *payload.P00
		}
		if payload.P01 != nil {
			bean.P01 = *payload.P01
		}
		return bean, nil
	case "SupportBean_S1":
		var payload struct {
			ID *int32 `json:"id"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		if payload.ID == nil {
			return nil, fmt.Errorf("SupportBean_S1 payload requires id")
		}
		return exprDefineValueParameterS1{ID: *payload.ID}, nil
	default:
		return nil, fmt.Errorf("unsupported %s event type %q", exprDefineValueParameterID, step.EventType)
	}
}

// exprDefineValueParameterCaseSteps pins the complete step sequence per
// case: case marker, deploy/deployed pairs in Java compileDeploy order, the
// types probe, sends, the iterator-count probes and undeploy-all.
var exprDefineValueParameterCaseSteps = map[string][]string{
	"eveve": {
		"deploy:cc:" + exprDefineValueParameterExprCC,
		"deployed:cc",
		"deploy:module:" + exprDefineValueParameterModule,
		"deployed:module",
		"types:s0",
		`send:SupportBean_S0:{"id":1,"p00":"A"}`,
		`send:SupportBean_S0:{"id":3,"p00":"C"}`,
		`send:SupportBean_S0:{"id":2,"p00":"B"}`,
		"undeploy-all",
	},
	"cache": {
		"deploy:module:" + exprDefineValueParameterCacheModule,
		"deployed:module",
		`send:SupportBean:{"intPrimitive":-1,"theString":"E10"}`,
		"iterator-count:s0:1",
		`send:SupportBean:{"intPrimitive":-1,"theString":"E10"}`,
		"iterator-count:s0:2",
		"undeploy-all",
	},
	"subquery": {
		"deploy:s0:" + exprDefineValueParameterSubquery,
		"deployed:s0",
		`send:SupportBean_S1:{"id":0}`,
		"undeploy-all",
	},
}

// loadExprDefineValueParameterScenario decodes the scenario with the strict
// contract shared by the differential runners: no duplicate or unknown JSON
// fields, pinned metadata, pinned per-case runtime/execution/EPL, and a
// per-op step field whitelist.
func loadExprDefineValueParameterScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", exprDefineValueParameterID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", exprDefineValueParameterID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDefineValueParameterID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", exprDefineValueParameterID, err)
	}
	if err := requireExprDefineValueParameterFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	var metadata struct {
		Version      string   `json:"version"`
		ID           string   `json:"id"`
		Description  string   `json:"description"`
		JavaCommit   string   `json:"javaCommit"`
		JavaSource   string   `json:"javaSource"`
		JavaRuntimes []string `json:"javaRuntimes"`
		JavaNames    []string `json:"javaNames"`
		JavaStaticID []string `json:"javaStaticIds"`
		JavaFlags    []string `json:"javaFlags"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", exprDefineValueParameterID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != exprDefineValueParameterID ||
		metadata.Description != exprDefineValueParameterDescription ||
		metadata.JavaCommit != exprDefineValueParameterJavaCommit || metadata.JavaSource != exprDefineValueParameterSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", exprDefineValueParameterID)
	}
	if err := exprDefineValueParameterRequireEqual(metadata.JavaFlags, exprDefineValueParameterJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}
	if err := exprDefineValueParameterRequireEqual(metadata.JavaRuntimes, exprDefineValueParameterJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := exprDefineValueParameterRequireEqual(metadata.JavaNames, exprDefineValueParameterJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := exprDefineValueParameterRequireEqual(metadata.JavaStaticID, exprDefineValueParameterJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", exprDefineValueParameterID, err)
	}
	if len(rawCases) != len(exprDefineValueParameterCaseSpecs) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", exprDefineValueParameterID, len(rawCases), len(exprDefineValueParameterCaseSpecs))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireExprDefineValueParameterFields(object,
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
		spec := exprDefineValueParameterCaseSpecs[index]
		if definition.Case != spec.name || definition.Ordinal != spec.ordinal ||
			definition.RuntimeID != spec.runtimeID || definition.ExecutionName != spec.execution {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity = %+v, want case %q ordinal %d runtime %s execution %s",
				exprDefineValueParameterID, index, definition, spec.name, spec.ordinal, spec.runtimeID, spec.execution)
		}
		if definition.Observation != spec.observation {
			return compat.Scenario{}, fmt.Errorf("scenario case %q observation does not match the pinned slice description", spec.name)
		}
		if definition.EPL != spec.epl {
			return compat.Scenario{}, fmt.Errorf("scenario case %q EPL does not match the Java source", spec.name)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", exprDefineValueParameterID, err)
	}
	if len(rawSteps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", exprDefineValueParameterID)
	}
	steps := make([]compat.Step, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireExprDefineValueParameterFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireExprDefineValueParameterFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				EPL       string `json:"epl"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if !exprDefineValueParameterDeployPinned(step.Case, step.Statement, step.EPL) {
				return compat.Scenario{}, fmt.Errorf("scenario step %d deploy %q/%q EPL is not pinned", index, step.Case, step.Statement)
			}
		case "deployed":
			if err := requireExprDefineValueParameterFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireExprDefineValueParameterFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := exprDefineValueParameterDecodePayload(compat.Step{EventType: step.EventType, Payload: step.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "types":
			if err := requireExprDefineValueParameterFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "eveve" || step.Statement != "s0" {
				return compat.Scenario{}, fmt.Errorf("scenario step %d types is not pinned", index)
			}
		case "iterator-count":
			if err := requireExprDefineValueParameterFields(object, "op", "case", "statement", "count"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var step struct {
				Case      string `json:"case"`
				Statement string `json:"statement"`
				Count     *int64 `json:"count"`
			}
			if err := json.Unmarshal(rawStep, &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if step.Case != "cache" || step.Statement != "s0" || step.Count == nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d iterator-count is not pinned", index)
			}
		case "undeploy-all":
			if err := requireExprDefineValueParameterFields(object, "op", "case"); err != nil {
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
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateExprDefineValueParameterRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// exprDefineValueParameterDeployPinned reports whether the deploy step's EPL
// belongs to the pinned set for the case/statement pair; the ordered
// case-steps pin fixes each position.
func exprDefineValueParameterDeployPinned(caseName, statement, epl string) bool {
	pinned, ok := map[string]map[string]string{
		"eveve": {
			"cc":     exprDefineValueParameterExprCC,
			"module": exprDefineValueParameterModule,
		},
		"cache": {
			"module": exprDefineValueParameterCacheModule,
		},
		"subquery": {
			"s0": exprDefineValueParameterSubquery,
		},
	}[caseName]
	if !ok {
		return false
	}
	want, ok := pinned[statement]
	return ok && epl == want
}

func exprDefineValueParameterRequireEqual(got, want []string, label string) error {
	if len(got) != len(want) {
		return fmt.Errorf("%s = %v, want %v", label, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	return nil
}

func requireExprDefineValueParameterFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return fmt.Errorf("unexpected field %q", name)
		}
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("missing field %q", name)
		}
	}
	return nil
}

// validateExprDefineValueParameterRawSteps pins the complete step sequence
// per case against the raw JSON objects.
func validateExprDefineValueParameterRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range exprDefineValueParameterCases {
		want, ok := exprDefineValueParameterCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", exprDefineValueParameterID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", exprDefineValueParameterID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", exprDefineValueParameterID, offset, caseName)
		}
		var marker struct {
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil || marker.Case != caseName {
			return fmt.Errorf("%s step %d must open case %q", exprDefineValueParameterID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := exprDefineValueParameterStepKey(rawSteps[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", exprDefineValueParameterID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q", exprDefineValueParameterID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", exprDefineValueParameterID)
	}
	return nil
}

// exprDefineValueParameterStepKey renders a raw step object into its pinned
// string form.
func exprDefineValueParameterStepKey(raw json.RawMessage, operation string) (string, error) {
	var step struct {
		Statement string          `json:"statement"`
		EPL       string          `json:"epl"`
		EventType string          `json:"eventType"`
		Payload   json.RawMessage `json:"payload"`
		Count     *int64          `json:"count"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", fmt.Errorf("decode step: %w", err)
	}
	switch operation {
	case "deploy":
		return "deploy:" + step.Statement + ":" + step.EPL, nil
	case "deployed":
		return "deployed:" + step.Statement, nil
	case "send":
		var payload map[string]any
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		return "send:" + step.EventType + ":" + string(canonical), nil
	case "types":
		return "types:" + step.Statement, nil
	case "iterator-count":
		if step.Count == nil {
			return "", fmt.Errorf("iterator-count step requires count")
		}
		return "iterator-count:" + step.Statement + ":" + strconv.FormatInt(*step.Count, 10), nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}
