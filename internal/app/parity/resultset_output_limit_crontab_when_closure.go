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

// resultset_output_limit_crontab_when_closure.go replays the three remaining
// ResultSetOutputLimitCrontabWhen executions (ords 9, 10 and 13) against the
// pinned Java oracle, closing the suite:
//
//   - when-then-soda (ord 9, ResultSetOutputWhenThenExpressionSODA): sets
//     myvar=0, advances to 2008-02-01T08:00, deploys the inert
//     `on SupportBean set myvar = intPrimitive` trigger and the s0
//     length(2) select whose output clause is
//     `when myvar=1 then set myvar=0, count_insert_var=count_insert`, then
//     undeploys. The suite's only assertion is the SODA model's toEPL text;
//     the Go API has no statement object model, so the asserted EPL pins as
//     an unrepresentable record. No events are sent and the listener never
//     fires, so the trace carries deployed markers plus the unrepresentable
//     record only.
//   - same-var-twice (ord 10, ResultSetOutputWhenThenSameVarTwice,
//     JIRA-386): deploys s1 and s2 as separate modules with the identical
//     `select * from SupportMarketDataBean output last when myvar=100`
//     clause, sends E1/E2, advances one second (neither listener fires),
//     sets myvar=100 and advances one more second so both listeners emit
//     the single last buffered row E2, then undeploys both modules.
//   - invalid (ord 13, ResultSetInvalid): eight path-less
//     tryInvalidCompile probes pin the output-clause rejection prefixes —
//     a non-boolean when expression, a then-set type mismatch, an aggregate
//     in then-set, a then-set literal without a variable name
//     (unrepresentable: SetOutputVariable requires a name), a stream
//     property inside an aggregate in when, an aggregate over count_insert
//     in when, prev(count_insert) in when, and a zero-second every
//     interval. Every probe compiles without the runtime path, mirroring
//     env.tryInvalidCompile's path-less compileWCheckedEx.
//
// Approved differences (observably identical to the Java EPL):
//   - The SODA model build and its toEPL assertion have no Go boundary, so
//     the unrepresentable step pins the asserted EPL text verbatim (the
//     context_lifecycle.go unrepresentable precedent); the s0 deploy
//     replays the byte-exact EPL the model compiles to.
//   - `then set 1` is unrepresentable in the typed Go API —
//     SetOutputVariable requires a variable name — so the step records the
//     pinned prefix without claiming a Go rejection boundary.
//   - Java declares count_insert_var as int; Go registers it int64 because
//     OutputCountInsert is int64 and the assignment widener rejects
//     narrowing. The variable is never read in the asserted surface.
//   - env.undeployModuleContaining("s1"/"s2") maps to per-statement
//     undeploy steps; env.undeployAll maps to undeploy-all.
//   - Plain send steps emit no trace record (the listener records carry
//     the observable output), matching the established runner convention;
//     deployed markers pin each module deploy.

const (
	resultsetOutputLimitCrontabWhenClosureID         = "resultset-output-limit-crontab-when-closure"
	resultsetOutputLimitCrontabWhenClosureJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultsetOutputLimitCrontabWhenClosureSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitCrontabWhen.java"
)

const resultsetOutputLimitCrontabWhenClosureDescription = "ResultSetOutputLimitCrontabWhen closure surface (ords 9, 10, 13): when-then-soda replays ResultSetOutputWhenThenExpressionSODA — runtimeSetVariable(myvar,0), advance to 2008-02-01T08:00, deploy the inert `on SupportBean set myvar = intPrimitive` trigger and the s0 `select symbol from SupportMarketDataBean#length(2) output when myvar=1 then set myvar=0, count_insert_var=count_insert` statement, then undeploy-all; the SODA toEPL assertion pins as an unrepresentable record carrying the asserted EPL (no Go object model); same-var-twice replays ResultSetOutputWhenThenSameVarTwice (JIRA-386) — s1 and s2 deploy the identical `select * from SupportMarketDataBean output last when myvar=100` module, E1/E2 sends plus a one-second advance fire neither listener, then myvar=100 and a second advance fire both with the single last buffered row E2 before both modules undeploy; invalid replays ResultSetInvalid's eight path-less tryInvalidCompile probes pinning the output-clause rejection prefixes (non-boolean when, then-set type mismatch, aggregate in then-set, then-set literal — unrepresentable in Go, aggregate over a stream property in when, aggregate over count_insert in when, prev in when, zero-second every interval). compile-error records carry the pinned Java message prefixes; unrepresentable records pin the asserted EPL and the then-set-literal prefix; deployed records mark each module statement; listener records carry the s1/s2 last-row emissions (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitCrontabWhen.java)."

var (
	resultsetOutputLimitCrontabWhenClosureJavaRuntimeIDs = []string{
		"java-runtime-c8af5811fef1d7f582d6",
		"java-runtime-feee4a26544010fc7fed",
		"java-runtime-54ed0a4e11d7714b8289",
	}
	resultsetOutputLimitCrontabWhenClosureJavaExecutions = []string{
		"ResultSetOutputWhenThenExpressionSODA",
		"ResultSetOutputWhenThenSameVarTwice",
		"ResultSetInvalid",
	}
	resultsetOutputLimitCrontabWhenClosureJavaStaticIDs = []string{
		"java-1ebf6ec5d07196df180e",
		"java-40115e717917aa787855",
		"java-ecf17baa44d87226cc82",
	}
	resultsetOutputLimitCrontabWhenClosureJavaFlags = []string{}
	resultsetOutputLimitCrontabWhenClosureCases     = []string{
		"when-then-soda",
		"same-var-twice",
		"invalid",
	}
	resultsetOutputLimitCrontabWhenClosureOrdinals = []int{9, 10, 13}
	resultsetOutputLimitCrontabWhenClosureSources  = []string{resultsetOutputLimitCrontabWhenClosureSource}
)

var resultsetOutputLimitCrontabWhenClosureCaseRuntimeIDs = map[string]string{
	"when-then-soda": "java-runtime-c8af5811fef1d7f582d6",
	"same-var-twice": "java-runtime-feee4a26544010fc7fed",
	"invalid":        "java-runtime-54ed0a4e11d7714b8289",
}

// The byte-exact EPLs the deploys and probes pin. The deploy EPLs are the
// verbatim compileDeploy inputs; the probe EPLs are the verbatim
// tryInvalidCompile inputs.
const (
	resultsetOutputLimitCrontabWhenClosureOnSetEPL  = "on SupportBean set myvar = intPrimitive"
	resultsetOutputLimitCrontabWhenClosureSodaEPL   = "@name('s0') select symbol from SupportMarketDataBean#length(2) output when myvar=1 then set myvar=0, count_insert_var=count_insert"
	resultsetOutputLimitCrontabWhenClosureS1EPL     = "@name('s1') select * from SupportMarketDataBean output last when myvar=100"
	resultsetOutputLimitCrontabWhenClosureS2EPL     = "@name('s2') select * from SupportMarketDataBean output last when myvar=100"
	resultsetOutputLimitCrontabWhenClosureSodaTime  = "2008-02-01T08:00:00Z"
	resultsetOutputLimitCrontabWhenClosureEpochTime = "1970-01-01T00:00:00Z"
	resultsetOutputLimitCrontabWhenClosurePlus1s    = "1970-01-01T00:00:01Z"
	resultsetOutputLimitCrontabWhenClosurePlus2s    = "1970-01-01T00:00:02Z"
)

var resultsetOutputLimitCrontabWhenClosureProbeEPLs = map[string]string{
	"when-non-bool":               "select * from SupportMarketDataBean output when myvardummy",
	"then-set-type-mismatch":      "select * from SupportMarketDataBean output when true then set myvardummy = 'b'",
	"then-set-aggregate":          "select * from SupportMarketDataBean output when true then set myvardummy = sum(myvardummy)",
	"then-set-literal":            "select * from SupportMarketDataBean output when true then set 1",
	"when-aggregate-property":     "select * from SupportMarketDataBean output when sum(price) > 0",
	"when-aggregate-count-insert": "select * from SupportMarketDataBean output when sum(count_insert) > 0",
	"when-prev-count-insert":      "select * from SupportMarketDataBean output when prev(1, count_insert) = 0",
	"every-zero-seconds":          "select theString, count(*) from SupportBean#length(2) group by theString output all every 0 seconds",
}

// The pinned Java message prefixes (SupportMessageAssertUtil.assertMessage
// startsWith semantics). The soda-to-epl entry is not an error prefix: it
// pins the asserted SODA toEPL text the unrepresentable record carries.
var resultsetOutputLimitCrontabWhenClosureProbeErrors = map[string]string{
	"when-non-bool":               "The when-trigger expression in the OUTPUT WHEN clause must return a boolean-type value [select * from SupportMarketDataBean output when myvardummy]",
	"then-set-type-mismatch":      "Failed to validate the output rate limiting clause: Failed to validate assignment expression 'myvardummy=\"b\"': Variable 'myvardummy' of declared type Integer cannot be assigned a value of type String [select * from SupportMarketDataBean output when true then set myvardummy = 'b']",
	"then-set-aggregate":          "Aggregation functions may not be used within update-set [select * from SupportMarketDataBean output when true then set myvardummy = sum(myvardummy)]",
	"then-set-literal":            "Failed to validate the output rate limiting clause: Failed to validate assignment expression '1': Assignment expression must receive a single variable value",
	"when-aggregate-property":     "Failed to validate output limit expression '(sum(price))>0': Property named 'price' is not valid in any stream [select * from SupportMarketDataBean output when sum(price) > 0]",
	"when-aggregate-count-insert": "An aggregate function may not appear in a OUTPUT LIMIT clause [select * from SupportMarketDataBean output when sum(count_insert) > 0]",
	"when-prev-count-insert":      "Failed to validate output limit expression 'prev(1,count_insert)=0': Previous function cannot be used in this context [select * from SupportMarketDataBean output when prev(1, count_insert) = 0]",
	"every-zero-seconds":          "Invalid time period expression returns a zero or negative time interval [select theString, count(*) from SupportBean#length(2) group by theString output all every 0 seconds]",
	"soda-to-epl":                 resultsetOutputLimitCrontabWhenClosureSodaEPL,
}

var resultsetOutputLimitCrontabWhenClosureCaseObservations = []string{
	"deployed+unrepresentable; myvar=0 and the 2008-02-01T08:00 advance precede the inert on-set deploy and the s0 length(2) output-when-then deploy; the SODA model's toEPL assertion pins the byte-exact EPL as an unrepresentable record (no Go object model); no events are sent so the listener never fires",
	"deployed+listener; s1 and s2 deploy the identical `output last when myvar=100` module, E1/E2 sends and a one-second advance fire neither listener, then myvar=100 and a second advance fire both listeners with the single last buffered row E2 before both modules undeploy",
	"compile-error+unrepresentable; eight path-less tryInvalidCompile probes pin the output-clause rejections: non-boolean when, then-set type mismatch, aggregate in then-set, then-set literal without a variable name (pinned-only; SetOutputVariable requires a name), aggregate over a stream property in when, aggregate over count_insert in when, prev in when, and a zero-second every interval",
}

var resultsetOutputLimitCrontabWhenClosureCaseEPLs = []string{
	resultsetOutputLimitCrontabWhenClosureSodaEPL,
	resultsetOutputLimitCrontabWhenClosureS1EPL,
	resultsetOutputLimitCrontabWhenClosureProbeEPLs["when-non-bool"],
}

// resultsetOutputLimitCrontabWhenClosureCaseSteps pins the complete step
// sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
// keys. deploy steps carry the byte-exact EPL text the Java oracle compiles;
// build-error steps carry the byte-exact probe EPL and the pinned
// expectError prefix; every probe is path-less (compileWithoutPath=1)
// mirroring env.tryInvalidCompile(epl, ...). The soda-to-epl
// unrepresentable step pins the asserted toEPL text in epl and expectError;
// the then-set-literal unrepresentable step pins its probe EPL and prefix.
var resultsetOutputLimitCrontabWhenClosureCaseSteps = map[string][]string{
	"when-then-soda": {
		"set-variable||myvar|||0|||",
		"advance-time||||||||" + resultsetOutputLimitCrontabWhenClosureSodaTime,
		"deploy|on-set|||" + resultsetOutputLimitCrontabWhenClosureOnSetEPL + "||||",
		"deployed|on-set|||||||",
		"deploy|s0|||" + resultsetOutputLimitCrontabWhenClosureSodaEPL + "||||",
		"deployed|s0|||||||",
		"unrepresentable|soda-to-epl|||" + resultsetOutputLimitCrontabWhenClosureSodaEPL + "||" + resultsetOutputLimitCrontabWhenClosureSodaEPL + "||",
		"set-variable||myvar|||0|||",
		"undeploy-all||||||||",
	},
	"same-var-twice": {
		"advance-time||||||||" + resultsetOutputLimitCrontabWhenClosureEpochTime,
		"deploy|s1|||" + resultsetOutputLimitCrontabWhenClosureS1EPL + "||||",
		"deployed|s1|||||||",
		"deploy|s2|||" + resultsetOutputLimitCrontabWhenClosureS2EPL + "||||",
		"deployed|s2|||||||",
		"send|||SupportMarketDataBean||{\"symbol\":\"ABC\",\"id\":\"E1\",\"price\":100}|||",
		"send|||SupportMarketDataBean||{\"symbol\":\"ABC\",\"id\":\"E2\",\"price\":100}|||",
		"advance-time||||||||" + resultsetOutputLimitCrontabWhenClosurePlus1s,
		"set-variable||myvar|||100|||",
		"advance-time||||||||" + resultsetOutputLimitCrontabWhenClosurePlus2s,
		"undeploy|s1|||||||",
		"undeploy|s2|||||||",
	},
	"invalid": {
		"build-error|when-non-bool|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["when-non-bool"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["when-non-bool"] + "|1|",
		"build-error|then-set-type-mismatch|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["then-set-type-mismatch"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["then-set-type-mismatch"] + "|1|",
		"build-error|then-set-aggregate|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["then-set-aggregate"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["then-set-aggregate"] + "|1|",
		"unrepresentable|then-set-literal|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["then-set-literal"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["then-set-literal"] + "|1|",
		"build-error|when-aggregate-property|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["when-aggregate-property"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["when-aggregate-property"] + "|1|",
		"build-error|when-aggregate-count-insert|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["when-aggregate-count-insert"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["when-aggregate-count-insert"] + "|1|",
		"build-error|when-prev-count-insert|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["when-prev-count-insert"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["when-prev-count-insert"] + "|1|",
		"build-error|every-zero-seconds|||" + resultsetOutputLimitCrontabWhenClosureProbeEPLs["every-zero-seconds"] + "||" + resultsetOutputLimitCrontabWhenClosureProbeErrors["every-zero-seconds"] + "|1|",
	},
}

// crontabWhenClosureBean mirrors SupportBean's asserted fields.
type crontabWhenClosureBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// crontabWhenClosureMarket mirrors SupportMarketDataBean's five fields;
// Volume and Feed stay nullable like the Java Long/String properties.
type crontabWhenClosureMarket struct {
	Symbol string  `esper:"symbol"`
	ID     string  `esper:"id"`
	Price  float64 `esper:"price"`
	Volume *int64  `esper:"volume"`
	Feed   *string `esper:"feed"`
}

// crontabWhenClosureCaseState carries the per-case replay state: the
// environment/engine pair, the deployed-step label bookkeeping, the
// deployments the undeploy steps retire, and the per-statement listener
// sequence counters.
type crontabWhenClosureCaseState struct {
	caseName       string
	env            *esper.Environment
	engine         *esper.Engine
	trace          *compat.Trace
	deployedLabels map[string]bool
	deployments    map[string]*esper.Deployment
	deployOrder    []string
	sequences      map[string]uint64
}

func runResultSetOutputLimitCrontabWhenClosureScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetOutputLimitCrontabWhenClosureScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeResultSetOutputLimitCrontabWhenClosure(ctx, scenario, &trace)
}

// executeResultSetOutputLimitCrontabWhenClosure replays the scenario: each
// case runs on a fresh environment/engine pair (one runtime per Java
// execution) and every step dispatches to the matching runtime action.
func executeResultSetOutputLimitCrontabWhenClosure(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *crontabWhenClosureCaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		if step.Op != "case" && state == nil {
			return *trace, fmt.Errorf("%s: step %q arrives before any case marker", resultsetOutputLimitCrontabWhenClosureID, step.Op)
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startCrontabWhenClosureCase(step.Case, trace)
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
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *trace, err
			}
		case "set-variable":
			if err := state.setVariable(ctx, step); err != nil {
				return *trace, err
			}
		case "advance-time":
			if err := state.advanceTime(ctx, step); err != nil {
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
		case "undeploy":
			if err := state.undeploy(ctx, step); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", resultsetOutputLimitCrontabWhenClosureID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startCrontabWhenClosureCase builds the fresh per-case environment: the
// SupportBean/SupportMarketDataBean event types and the myvar, myvardummy
// and count_insert_var variables the suite configuration declares (myvar and
// myvardummy are int; count_insert_var is int64 because OutputCountInsert is
// int64 and narrowing is rejected), plus the engine pinned to the case's
// Java runtime id at the epoch start time.
func startCrontabWhenClosureCase(caseName string, trace *compat.Trace) (*crontabWhenClosureCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[crontabWhenClosureBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[crontabWhenClosureMarket](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	if err := env.RegisterVariable("myvar", 0); err != nil {
		return nil, err
	}
	if err := env.RegisterVariable("myvardummy", 0); err != nil {
		return nil, err
	}
	if err := env.RegisterVariable("count_insert_var", int64(0)); err != nil {
		return nil, err
	}
	state := &crontabWhenClosureCaseState{
		caseName:       caseName,
		env:            env,
		deployedLabels: map[string]bool{},
		deployments:    map[string]*esper.Deployment{},
		sequences:      map[string]uint64{},
		trace:          trace,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(resultsetOutputLimitCrontabWhenClosureCaseRuntimeIDs[caseName]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

// deploy executes one deploy step. The when-then-soda on-set deploy builds
// the `on SupportBean set myvar = intPrimitive` trigger; the s0 deploy
// builds the length(2) select with the when/then output clause the SODA
// model compiles to. The same-var-twice deploys build the identical
// output-last-when select for s1 and s2 and attach the listener the Java
// execution registers on each statement.
func (s *crontabWhenClosureCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	market := esper.From[crontabWhenClosureMarket](s.env, "SupportMarketDataBean")
	var plan esper.Plan
	var err error
	switch label {
	case "on-set":
		if s.caseName != "when-then-soda" || step.Epl != resultsetOutputLimitCrontabWhenClosureOnSetEPL {
			return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", resultsetOutputLimitCrontabWhenClosureID, s.caseName, label, step.Epl)
		}
		plan, err = s.env.Build(esper.OnEvent(esper.From[crontabWhenClosureBean](s.env, "SupportBean")).
			SetVariable("myvar", esper.Field[crontabWhenClosureBean, int]("intPrimitive")).
			Query())
	case "s0":
		if s.caseName != "when-then-soda" || step.Epl != resultsetOutputLimitCrontabWhenClosureSodaEPL {
			return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", resultsetOutputLimitCrontabWhenClosureID, s.caseName, label, step.Epl)
		}
		plan, err = s.env.Build(esper.Select(market.Window(esper.LengthWindow(2)),
			esper.Alias("symbol", esper.Field[crontabWhenClosureMarket, string]("symbol"))).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(
					esper.Equal[int](esper.VariableRef[int]("myvar"), esper.Literal(1)),
					esper.SetOutputVariable("myvar", esper.Literal(0)),
					esper.SetOutputVariable("count_insert_var", esper.OutputCountInsert())))))
	case "s1", "s2":
		pinned := resultsetOutputLimitCrontabWhenClosureS1EPL
		if label == "s2" {
			pinned = resultsetOutputLimitCrontabWhenClosureS2EPL
		}
		if s.caseName != "same-var-twice" || step.Epl != pinned {
			return fmt.Errorf("%s: case %q deploy %q carries an unpinned EPL %q", resultsetOutputLimitCrontabWhenClosureID, s.caseName, label, step.Epl)
		}
		plan, err = s.env.Build(esper.Select(market).
			Query(esper.StatementName(label),
				esper.WithOutput(esper.OutputWhenWith(esper.OutputLast(),
					esper.Equal[int](esper.VariableRef[int]("myvar"), esper.Literal(100))))))
	default:
		return fmt.Errorf("%s: case %q has no deploy fixture for %q", resultsetOutputLimitCrontabWhenClosureID, s.caseName, label)
	}
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", resultsetOutputLimitCrontabWhenClosureID, label, err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", resultsetOutputLimitCrontabWhenClosureID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	s.deployedLabels[label] = true
	// Mirrors addListener("s0"/"s1"/"s2"): the listened statements record
	// each delivered batch; the inert on-set trigger gets no listener.
	for _, statement := range deployment.Statements() {
		if statement.Name() != label || (label != "s0" && label != "s1" && label != "s2") {
			continue
		}
		statementName := statement.Name()
		if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			if len(batch.New) == 0 && len(batch.Old) == 0 {
				return nil
			}
			s.sequences[statementName+":listener"]++
			s.trace.Records = append(s.trace.Records, compat.TraceRecord{
				Case:      s.caseName,
				Operation: "listener",
				Statement: statementName,
				Sequence:  s.sequences[statementName+":listener"],
				Time:      compat.FormatTraceTime(s.engine.Now()),
				New:       compat.NormalizeResults(batch.New),
				Old:       compat.NormalizeResults(batch.Old),
			})
			return nil
		}); err != nil {
			return fmt.Errorf("%s: deploy %q listener: %w", resultsetOutputLimitCrontabWhenClosureID, label, err)
		}
	}
	return nil
}

// deployed emits the deployed marker for a statement label the preceding
// deploy step registered, mirroring the oracle's per-statement marker.
func (s *crontabWhenClosureCaseState) deployed(step compat.Step) error {
	if !s.deployedLabels[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement)
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

// send replays one send step: a SupportMarketDataBean carrier whose payload
// pins symbol, id and price (volume and feed stay null like the Java
// three-argument constructor). Sends emit no trace record; the listener
// records carry the observable output.
func (s *crontabWhenClosureCaseState) send(ctx context.Context, step compat.Step) error {
	if step.EventType != "SupportMarketDataBean" {
		return fmt.Errorf("%s: send has unpinned event type %q", resultsetOutputLimitCrontabWhenClosureID, step.EventType)
	}
	var payload crontabWhenClosureMarket
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("%s: send %s payload: %w", resultsetOutputLimitCrontabWhenClosureID, step.EventType, err)
	}
	if err := s.engine.Send(ctx, step.EventType, payload); err != nil {
		return fmt.Errorf("%s: send %s: %w", resultsetOutputLimitCrontabWhenClosureID, step.EventType, err)
	}
	return nil
}

// setVariable replays runtimeSetVariable(null, name, value); the pinned
// variables are int-typed so the payload decodes as a Go int.
func (s *crontabWhenClosureCaseState) setVariable(ctx context.Context, step compat.Step) error {
	if step.Name != "myvar" {
		return fmt.Errorf("%s: set-variable has unpinned name %q", resultsetOutputLimitCrontabWhenClosureID, step.Name)
	}
	var value int
	if err := json.Unmarshal(step.Payload, &value); err != nil {
		return fmt.Errorf("%s: set-variable %s payload: %w", resultsetOutputLimitCrontabWhenClosureID, step.Name, err)
	}
	if err := s.engine.SetVariable(ctx, step.Name, value); err != nil {
		return fmt.Errorf("%s: set-variable %s: %w", resultsetOutputLimitCrontabWhenClosureID, step.Name, err)
	}
	return nil
}

// advanceTime replays sendTimer/sendTimeEvent: the step's RFC3339 instant
// is the absolute engine time (sendTimer(ms) is epoch+ms; sendTimeEvent
// pins 2008-02-01T08:00 under the oracle's UTC zone).
func (s *crontabWhenClosureCaseState) advanceTime(ctx context.Context, step compat.Step) error {
	at, err := time.Parse(time.RFC3339Nano, step.At)
	if err != nil {
		return fmt.Errorf("%s: advance-time: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	if err := s.engine.AdvanceTime(ctx, at); err != nil {
		return fmt.Errorf("%s: advance-time %s: %w", resultsetOutputLimitCrontabWhenClosureID, step.At, err)
	}
	return nil
}

// buildError runs one expected-invalid probe against the fluent equivalent
// of the pinned EPL. Each probe verifies Go rejects the nearest expressible
// boundary before recording the pinned Java message prefix.
func (s *crontabWhenClosureCaseState) buildError(step compat.Step) error {
	if pinned, ok := resultsetOutputLimitCrontabWhenClosureProbeEPLs[step.Statement]; !ok || step.Epl != pinned {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned EPL %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement, step.Epl)
	}
	if step.ExpectError != resultsetOutputLimitCrontabWhenClosureProbeErrors[step.Statement] {
		return fmt.Errorf("%s: build-error probe %q carries an unpinned prefix %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement, step.ExpectError)
	}
	market := esper.From[crontabWhenClosureMarket](s.env, "SupportMarketDataBean")
	var buildErr error
	switch step.Statement {
	case "when-non-bool":
		// `output when myvardummy` — the int-typed variable is not a
		// boolean when-trigger.
		_, buildErr = s.env.Build(esper.Select(market).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(esper.VariableRef[int]("myvardummy")))))
	case "then-set-type-mismatch":
		// `then set myvardummy = 'b'` — a string literal cannot assign the
		// int-typed variable.
		_, buildErr = s.env.Build(esper.Select(market).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(esper.Literal(true),
					esper.SetOutputVariable("myvardummy", esper.Literal("b"))))))
	case "then-set-aggregate":
		// `then set myvardummy = sum(myvardummy)` — aggregates are rejected
		// inside update-set assignments.
		_, buildErr = s.env.Build(esper.Select(market).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(esper.Literal(true),
					esper.SetOutputVariable("myvardummy", esper.Sum[int](esper.VariableRef[int]("myvardummy")))))))
	case "when-aggregate-property":
		// `output when sum(price) > 0` — the when condition may reference
		// variables only, so the stream property inside the aggregate is
		// rejected first.
		_, buildErr = s.env.Build(esper.Select(market).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(
					esper.Greater[float64](esper.Sum[float64](esper.Field[crontabWhenClosureMarket, float64]("price")), esper.Literal(0.0))))))
	case "when-aggregate-count-insert":
		// `output when sum(count_insert) > 0` — an aggregate may not appear
		// in the output-limit clause.
		_, buildErr = s.env.Build(esper.Select(market).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(
					esper.Greater[int64](esper.Sum[int64](esper.OutputCountInsert()), esper.Literal(int64(0)))))))
	case "when-prev-count-insert":
		// `output when prev(1, count_insert) = 0` — previous functions are
		// rejected in the output-limit clause.
		_, buildErr = s.env.Build(esper.Select(market).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputWhen(
					esper.Equal[int64](esper.Prev[int64](1, esper.OutputCountInsert()), esper.Literal(int64(0)))))))
	case "every-zero-seconds":
		// `output all every 0 seconds` — the time-based interval must be
		// positive.
		theString := esper.Field[crontabWhenClosureBean, string]("theString")
		_, buildErr = s.env.Build(esper.From[crontabWhenClosureBean](s.env, "SupportBean").
			Window(esper.LengthWindow(2)).
			GroupBy(theString).
			Select(
				esper.Alias("theString", theString),
				esper.Alias("count(*)", esper.CountAll())).
			Query(esper.StatementName("s0"),
				esper.WithOutput(esper.OutputAllEveryTime(0))))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", resultsetOutputLimitCrontabWhenClosureID, step.Statement)
	}
	// Verify the Go rejection carries the expected wording before recording
	// the pinned Java prefix.
	expected := map[string]string{
		"when-non-bool":               "output when condition must be boolean",
		"then-set-type-mismatch":      `output variable "myvardummy" has type int, assignment expression has type string`,
		"then-set-aggregate":          "Aggregation functions may not be used within update-set",
		"when-aggregate-property":     "output when condition can reference variables only",
		"when-aggregate-count-insert": "An aggregate function may not appear in a OUTPUT LIMIT clause",
		"when-prev-count-insert":      "Previous function cannot be used in this context",
		"every-zero-seconds":          "time-based output interval must be positive",
	}
	if want, ok := expected[step.Statement]; ok && !strings.Contains(buildErr.Error(), want) {
		return fmt.Errorf("%s: build-error probe %q drift: got %v", resultsetOutputLimitCrontabWhenClosureID, step.Statement, buildErr)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// unrepresentable emits the pinned record for the two surfaces the typed Go
// API cannot express: the SODA model's toEPL assertion (no statement object
// model; the pinned value is the asserted EPL text) and the `then set 1`
// probe (SetOutputVariable requires a variable name; the pinned value is the
// Java message prefix). Both record the pinned value without claiming a Go
// rejection boundary, mirroring the context_lifecycle.go precedent.
func (s *crontabWhenClosureCaseState) unrepresentable(step compat.Step) error {
	var pinnedEPL, pinnedValue string
	switch step.Statement {
	case "soda-to-epl":
		if s.caseName != "when-then-soda" {
			return fmt.Errorf("%s: unrepresentable step %q is not pinned for case %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement, s.caseName)
		}
		pinnedEPL = resultsetOutputLimitCrontabWhenClosureSodaEPL
		pinnedValue = resultsetOutputLimitCrontabWhenClosureSodaEPL
	case "then-set-literal":
		if s.caseName != "invalid" {
			return fmt.Errorf("%s: unrepresentable step %q is not pinned for case %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement, s.caseName)
		}
		pinnedEPL = resultsetOutputLimitCrontabWhenClosureProbeEPLs["then-set-literal"]
		pinnedValue = resultsetOutputLimitCrontabWhenClosureProbeErrors["then-set-literal"]
	default:
		return fmt.Errorf("%s: unknown unrepresentable step %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement)
	}
	if step.Epl != pinnedEPL {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned EPL %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement, step.Epl)
	}
	if step.ExpectError != pinnedValue {
		return fmt.Errorf("%s: unrepresentable step %q carries an unpinned value %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement, step.ExpectError)
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeploy retires one labeled deployment, mirroring
// env.undeployModuleContaining.
func (s *crontabWhenClosureCaseState) undeploy(ctx context.Context, step compat.Step) error {
	deployment, ok := s.deployments[step.Statement]
	if !ok {
		return fmt.Errorf("%s: undeploys unknown statement %q", resultsetOutputLimitCrontabWhenClosureID, step.Statement)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", resultsetOutputLimitCrontabWhenClosureID, step.Statement, err)
	}
	delete(s.deployments, step.Statement)
	for index, label := range s.deployOrder {
		if label == step.Statement {
			s.deployOrder = append(s.deployOrder[:index], s.deployOrder[index+1:]...)
			break
		}
	}
	return nil
}

// undeployAll undeploys deployments in reverse deploy order, mirroring
// env.undeployAll().
func (s *crontabWhenClosureCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", resultsetOutputLimitCrontabWhenClosureID, label, err)
		}
		delete(s.deployments, label)
	}
	s.deployOrder = nil
	return nil
}

// loadResultSetOutputLimitCrontabWhenClosureScenario decodes the scenario
// with the strict-shape checks the raw-mutation tests pin: no duplicate JSON
// keys, the exact top-level field set, pinned metadata and case metadata,
// and the pinned per-case step keys so unknown or mutated steps fail the
// replay.
func loadResultSetOutputLimitCrontabWhenClosureScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", resultsetOutputLimitCrontabWhenClosureID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	if err := requireCrontabWhenClosureFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != resultsetOutputLimitCrontabWhenClosureID ||
		metadata.Description != resultsetOutputLimitCrontabWhenClosureDescription ||
		metadata.JavaCommit != resultsetOutputLimitCrontabWhenClosureJavaCommit ||
		metadata.JavaSource != resultsetOutputLimitCrontabWhenClosureSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", resultsetOutputLimitCrontabWhenClosureID)
	}
	if err := validateCrontabWhenClosureStringArray(root["javaRuntimes"], resultsetOutputLimitCrontabWhenClosureJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateCrontabWhenClosureStringArray(root["javaNames"], resultsetOutputLimitCrontabWhenClosureJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateCrontabWhenClosureStringArray(root["javaStaticIds"], resultsetOutputLimitCrontabWhenClosureJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateCrontabWhenClosureStringArray(root["javaFlags"], resultsetOutputLimitCrontabWhenClosureJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(resultsetOutputLimitCrontabWhenClosureCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", resultsetOutputLimitCrontabWhenClosureID, len(resultsetOutputLimitCrontabWhenClosureCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireCrontabWhenClosureFields(object,
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
		if definition.Case != resultsetOutputLimitCrontabWhenClosureCases[index] ||
			definition.Ordinal != resultsetOutputLimitCrontabWhenClosureOrdinals[index] ||
			definition.RuntimeID != resultsetOutputLimitCrontabWhenClosureJavaRuntimeIDs[index] ||
			definition.ExecutionName != resultsetOutputLimitCrontabWhenClosureJavaExecutions[index] ||
			definition.Observation != resultsetOutputLimitCrontabWhenClosureCaseObservations[index] ||
			definition.EPL != resultsetOutputLimitCrontabWhenClosureCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", resultsetOutputLimitCrontabWhenClosureID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	offset := 0
	for _, caseName := range resultsetOutputLimitCrontabWhenClosureCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", resultsetOutputLimitCrontabWhenClosureID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", resultsetOutputLimitCrontabWhenClosureID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", resultsetOutputLimitCrontabWhenClosureID, offset, caseName)
		}
		offset++
		want, ok := resultsetOutputLimitCrontabWhenClosureCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", resultsetOutputLimitCrontabWhenClosureID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", resultsetOutputLimitCrontabWhenClosureID, caseName)
		}
		for _, pinned := range want {
			key, err := crontabWhenClosureStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", resultsetOutputLimitCrontabWhenClosureID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", resultsetOutputLimitCrontabWhenClosureID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", resultsetOutputLimitCrontabWhenClosureID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", resultsetOutputLimitCrontabWhenClosureID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", resultsetOutputLimitCrontabWhenClosureID)
	}
	return scenario, nil
}

// crontabWhenClosureStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|at
// with the payload compacted. Unknown fields on the step object are
// rejected.
func crontabWhenClosureStepKey(raw json.RawMessage) (string, error) {
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
		At                 string          `json:"at"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "at": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payload := ""
	if len(step.Payload) != 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("payload: %w", err)
		}
		payload = compacted.String()
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.At, nil
}

// validateResultSetOutputLimitCrontabWhenClosureScenario re-checks a decoded
// scenario (used when the runner receives a scenario decoded by the generic
// loader path). The generic compat.Step op whitelist accepts the
// unrepresentable op this scenario pins, so validation is the id and
// step-count invariants.
func validateResultSetOutputLimitCrontabWhenClosureScenario(scenario compat.Scenario) error {
	if scenario.ID != resultsetOutputLimitCrontabWhenClosureID {
		return fmt.Errorf("%s scenario id %q is not pinned", resultsetOutputLimitCrontabWhenClosureID, scenario.ID)
	}
	if len(scenario.Steps) == 0 {
		return fmt.Errorf("%s scenario has no steps", resultsetOutputLimitCrontabWhenClosureID)
	}
	return nil
}

func requireCrontabWhenClosureFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", resultsetOutputLimitCrontabWhenClosureID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", resultsetOutputLimitCrontabWhenClosureID, name)
		}
	}
	return nil
}

func validateCrontabWhenClosureStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
