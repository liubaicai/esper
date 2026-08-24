package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

var ecsmJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/clazz/ExprClassStaticMethod.java",
}

type ecsmBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
}

var ecsmJavaRuntimeIDs = []string{
	"java-runtime-23ebc0a94390e9ac66cd",
	"java-runtime-7f2346b554acf5f94745",
	"java-runtime-bf25fdd8aadf8784565f",
	"java-runtime-00b7147ca51c1d12efb7",
	"java-runtime-43b48f547e0b645a0225",
	"java-runtime-d778a673081da61e849c",
	"java-runtime-19af491ff80c737aa9b2",
	"java-runtime-5a87cab48e592ec8cb3c",
	"java-runtime-0c4767c9a92380fcf0e4",
	"java-runtime-a6dac1b76794aa1447ac",
	"java-runtime-10cf1e47a1cfad251b08",
}

var ecsmJavaExecutions = []string{
	"ExprClassStaticMethodLocal{soda=false}",
	"ExprClassStaticMethodLocal{soda=true}",
	"ExprClassStaticMethodCreate{soda=false}",
	"ExprClassStaticMethodCreate{soda=true}",
	"ExprClassStaticMethodLocalFAFQuery",
	"ExprClassStaticMethodCreateFAFQuery",
	"ExprClassStaticMethodLocalAndCreateClassTogether",
	"ExprClassDocSamples",
	"ExprClassInvalidCompile",
	"ExprClassStaticMethodCreateClassWithPackageName",
	"ExprClassStaticMethodLocalWithPackageName",
}

func runEcsmScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	for _, cn := range []string{
		"local", "create", "local-faf", "create-faf", "local-and-create",
		"doc-samples", "invalid-compile-valid", "package-create", "package-local",
	} {
		if !scenarioHasCase(scenario, cn) {
			continue
		}
		ct, err := runEcsmCase(ctx, scenario, cn)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("case %q: %w", cn, err)
		}
		trace.Records = append(trace.Records, ct.Records...)
	}
	return trace, nil
}

func validateEcsmDeploymentSteps(scenario compat.Scenario, caseName string) error {
	expected := []string(nil)
	switch caseName {
	case "local", "package-create", "package-local":
		expected = []string{"s0"}
	case "create":
		expected = []string{"create-class", "s0"}
	case "local-faf", "create-faf":
		expected = []string{"window"}
	case "local-and-create":
		expected = []string{"classes", "s0"}
	case "doc-samples", "invalid-compile-valid":
	default:
		return fmt.Errorf("unsupported expr-class-static-method case %q", caseName)
	}

	deployIndex := 0
	actionStarted := false
	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "deploy":
			if actionStarted {
				return fmt.Errorf("expr-class-static-method case %q deploy %q follows an action", caseName, step.Statement)
			}
			if deployIndex >= len(expected) {
				return fmt.Errorf("expr-class-static-method case %q has unexpected deploy %q", caseName, step.Statement)
			}
			if step.Statement != expected[deployIndex] {
				return fmt.Errorf("expr-class-static-method case %q deploy step %d = %q, want %q", caseName, deployIndex, step.Statement, expected[deployIndex])
			}
			deployIndex++
		case "send", "faf":
			actionStarted = true
		}
	}
	if deployIndex != len(expected) {
		return fmt.Errorf("expr-class-static-method case %q has %d deploy steps, want %d", caseName, deployIndex, len(expected))
	}
	return nil
}

func runEcsmCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	caseScenario, err := scenarioForCase(scenario, caseName)
	if err != nil {
		return compat.Trace{}, err
	}
	if err := validateEcsmDeploymentSteps(caseScenario, caseName); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[ecsmBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if caseName == "local-faf" || caseName == "create-faf" {
		windowSchema, err := esper.StructSchema[ecsmWindowBean]("MyWindowSchema")
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := esper.CreateNamedWindow(env, "MyWindow", windowSchema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return compat.Trace{}, err
		}
	}

	engine := esper.NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := &compat.Trace{Version: caseScenario.Version, ID: caseScenario.ID}
	seqByStmt := make(map[string]uint64)
	deploy := func(query esper.Query) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		for _, st := range deployment.Statements() {
			stmt := st
			if _, subErr := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				seqByStmt[stmt.Name()]++
				record := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Time:      batch.Time.UTC().Format(time.RFC3339),
					Sequence:  seqByStmt[stmt.Name()],
				}
				record.New = compat.NormalizeResults(batch.New)
				record.Old = compat.NormalizeResults(batch.Old)
				if record.New == nil {
					record.New = []compat.ResultRecord{}
				}
				if record.Old == nil {
					record.Old = []compat.ResultRecord{}
				}
				trace.Records = append(trace.Records, record)
				return nil
			}); subErr != nil {
				return subErr
			}
		}
		return nil
	}

	ts := func() esper.Expression[string] { return esper.Field[ecsmBean, string]("theString") }
	ip := func() esper.Expression[int32] { return esper.Field[ecsmBean, int32]("intPrimitive") }

	// Statement to deploy per case, plus FAF plans for the faf cases.
	type fafDef struct {
		plan func() (esper.Plan, error)
		seq  uint64
	}
	var fafPlans map[string]*fafDef

	switch caseName {
	case "local":
		// inlined_class MyClass.doIt: '|'+parameter+'|' — Go closure.
		doIt := esper.Func1[string, string]("MyClass.doIt",
			func(s string) string { return "|" + s + "|" }, ts())
		if err := deploy(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", doIt),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "create":
		// @public create inlined_class — shared named expression registry.
		if err := esper.DefineExpression[string](env, "MyClass.doIt", esper.Func1[string, string]("doIt",
			func(s string) string { return "|" + s + "|" },
			esper.ExpressionParam[string]("parameter"))); err != nil {
			return compat.Trace{}, err
		}
		if err := deploy(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", esper.ExpressionRef[string](env, "MyClass.doIt", ts())),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "local-faf":
		// Named window MyWindow#keepall(theString) fed by merge; FAF with
		// embedded local class doIt: '>'+parameter+'<'.

		insertPlan, err := env.Build(esper.OnEvent(
			esper.From[ecsmBean](env, "SupportBean")).InsertIntoNamedWindow(
			"MyWindow", esper.SetColumn("theString", ts()),
		).Query(esper.StatementName("merge")))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := engine.Deploy(ctx, insertPlan); err != nil {
			return compat.Trace{}, err
		}
		fafPlans = map[string]*fafDef{
			"s0": {plan: func() (esper.Plan, error) {
				nwField := esper.Field[any, string]("theString")
				doIt := esper.Func1[string, string]("MyClass.doIt",
					func(s string) string { return ">" + s + "<" }, nwField)
				return env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
					esper.Alias("c0", doIt),
				).Query(esper.StatementName("s0")))
			}},
		}

	case "create-faf":

		insertPlan, err := env.Build(esper.OnEvent(
			esper.From[ecsmBean](env, "SupportBean")).InsertIntoNamedWindow(
			"MyWindow", esper.SetColumn("theString", ts()),
		).Query(esper.StatementName("merge")))
		if err != nil {
			return compat.Trace{}, err
		}
		if _, err := engine.Deploy(ctx, insertPlan); err != nil {
			return compat.Trace{}, err
		}
		if err := esper.DefineExpression[string](env, "MyClass.doIt", esper.Func1[string, string]("doIt",
			func(s string) string { return "abc" },
			esper.ExpressionParam[string]("parameter"))); err != nil {
			return compat.Trace{}, err
		}
		fafPlans = map[string]*fafDef{
			"s0": {plan: func() (esper.Plan, error) {
				nwField := esper.Field[any, string]("theString")
				return env.Build(esper.FromNamedWindow(env, "MyWindow").Select(
					esper.Alias("c0", esper.ExpressionRef[string](env, "MyClass.doIt", nwField)),
				).Query(esper.StatementName("s0")))
			}},
		}

	case "local-and-create":
		if err := esper.DefineExpression[string](env, "MyUtil.returnBubba", esper.Func0[string]("returnBubba",
			func() string { return "bubba" })); err != nil {
			return compat.Trace{}, err
		}
		if err := esper.DefineExpression[string](env, "MyClass.doIt", esper.Concat(
			esper.Literal("|"),
			esper.ExpressionRef[string](env, "MyUtil.returnBubba"),
			esper.Literal("|"),
		)); err != nil {
			return compat.Trace{}, err
		}
		if err := deploy(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", esper.ExpressionRef[string](env, "MyClass.doIt")),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "doc-samples":
		// Build the recursive fib projection and the midPrice expression plan.
		var fib func(int) float64
		fib = func(n int) float64 {
			if n <= 1 {
				return float64(n)
			}
			return fib(n-1) + fib(n-2)
		}
		fibExpr := esper.Func1[int32, float64]("MyUtility.fib",
			func(n int32) float64 { return fib(int(n)) }, ip())
		if _, err := env.Build(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", fibExpr),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}
		if err := esper.DefineExpression[float64](env, "MyUtility.midPrice", esper.Func2[float64, float64, float64]("midPrice",
			func(buy, sell float64) float64 { return (buy + sell) / 2 },
			esper.ExpressionParam[float64]("buy"),
			esper.ExpressionParam[float64]("sell"))); err != nil {
			return compat.Trace{}, err
		}
		if _, err := env.Build(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", esper.ExpressionRef[float64](env, "MyUtility.midPrice",
				esper.Field[ecsmBean, float64]("doublePrimitive"),
				esper.Field[ecsmBean, float64]("doubleBoxed"))),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "invalid-compile-valid":
		// Case-1: empty class text compiles; Janino-only invalid diagnostics are
		// a separate approved difference.
		if _, err := env.Build(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "package-create":
		if err := esper.DefineExpression[string](env, "mypackage.MyUtil.doIt", esper.Func2[string, int32, string]("doIt",
			func(s string, n int32) string { return s + fmt.Sprintf("%d", n) },
			esper.ExpressionParam[string]("theString"),
			esper.ExpressionParam[int32]("intPrimitive"))); err != nil {
			return compat.Trace{}, err
		}
		if err := deploy(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", esper.ExpressionRef[string](env, "mypackage.MyUtil.doIt", ts(), ip())),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	case "package-local":
		doIt := esper.Func0[string]("mypackage.MyUtil.doIt",
			func() string { return "test" })
		if err := deploy(esper.Select(
			esper.From[ecsmBean](env, "SupportBean"),
			esper.Alias("c0", doIt),
		).Query(esper.StatementName("s0"))); err != nil {
			return compat.Trace{}, err
		}

	default:
		return compat.Trace{}, fmt.Errorf("unsupported expr-class-static-method case %q", caseName)
	}

	for _, step := range caseScenario.Steps {
		switch step.Op {
		case "case":
			continue
		case "send":
			var payload map[string]any
			if err := json.Unmarshal(step.Payload, &payload); err != nil {
				return *trace, err
			}
			v := ecsmBean{
				TheString:    jsonString(payload["theString"]),
				IntPrimitive: jsonInt32(payload["intPrimitive"]),
			}
			if err := engine.SendEvent(ctx, v); err != nil {
				return *trace, err
			}
		case "deploy":
			// Statements deploy eagerly in the case setup.
			continue
		case "faf":
			def, ok := fafPlans[step.Statement]
			if !ok {
				return *trace, fmt.Errorf("unknown faf query %q", step.Statement)
			}
			plan, err := def.plan()
			if err != nil {
				return *trace, err
			}
			result, err := engine.ExecuteFireAndForget(ctx, plan)
			if err != nil {
				return *trace, err
			}
			def.seq++
			record := compat.TraceRecord{
				Case:      caseName,
				Operation: "faf",
				Statement: step.Statement,
				Sequence:  def.seq,
			}
			record.New = compat.NormalizeResults(result.Batch.New)
			record.Old = compat.NormalizeResults(result.Batch.Old)
			if record.New == nil {
				record.New = []compat.ResultRecord{}
			}
			if record.Old == nil {
				record.Old = []compat.ResultRecord{}
			}
			trace.Records = append(trace.Records, record)
		default:
			return *trace, fmt.Errorf("unsupported expr-class-static-method op %q (case %q)", step.Op, caseName)
		}
	}
	return *trace, nil
}

type ecsmWindowBean struct {
	TheString string `esper:"theString"`
}
