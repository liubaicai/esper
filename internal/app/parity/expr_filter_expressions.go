package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	efeID         = "expr-filter-expressions"
	efeJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	efeSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterExpressions.java"
)

const efeDescription = "ExprFilterExpressions filter-expression sites (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): ExprFilterOverInClause (ord 7) deploys pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)] s0 then pattern [every event1=SupportTradeEvent(userId in ('100','101'))] s1 — 'every' is cumulative so both statements fire on the second send; ExprFilterStaticFunc (ord 17) runs the runIsInvokedWTestdata matrix over 8 statements mixing SupportStaticMethodLib.isStringEquals, theString || 'x' concat, comma-vs-and conjunctions and reversed equals, sending 'a','b','c' with invoked flags [F,T,F] for s0-s6 and [F,F,F] for the unsatisfiable s7; ExprFilterInstanceMethodWWildcard (ord 27) runs three deploy/send/undeployAll cycles asserting instance-method filters: s0.myInstanceMethodAlwaysTrue() [T,T,T], s0.myInstanceMethodEventBean(s0,'x',1) [F,T,F] and the '*' wildcard argument [F,T,F]. Java milestones are ordering markers with no virtual time; assertEqualsNew/assertListenerInvokedFlag map to listener records on fire and listener-not-invoked count records otherwise (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterExpressions.java)."

var efeCaseObservations = []string{
	"listener; s0 pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)] fires on Trade(1,'100',1001); after s1 pattern [every event1=SupportTradeEvent(userId in ('100','101'))] deploys, Trade(2,'100',1001) fires both s0 and s1 — 'every' keeps s0 cumulative",
	"listener+invoked-flags; runIsInvokedWTestdata over 8 statements s0-s7 (isStringEquals UDF, theString || 'x' concat, comma-vs-and, reversed equals, unsatisfiable conjunction): sends 'a','b','c' yield [F,T,F] for s0-s6 and [F,F,F] for s7",
	"listener+invoked-flags; three deploy/send/undeployAll cycles: s0.myInstanceMethodAlwaysTrue() fires [T,T,T], s0.myInstanceMethodEventBean(s0,'x',1) fires [F,T,F] only for x=1, and the '*' wildcard argument fires identically [F,T,F]",
}

// efeStaticFuncLib is the SupportStaticMethodLib FQN the pinned EPLs carry.
const efeStaticFuncLib = "com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib"

// efeStaticFuncBodies pins the eight ExprFilterStaticFunc EPL bodies in
// assertion order (the harness wraps each as "@name('s<i>') " + body). The
// s2 comma has no following space and s3/s4/s5 keep theirs, exactly as the
// Java concatenations produce them.
var efeStaticFuncBodies = []string{
	"select * from SupportBean(" + efeStaticFuncLib + ".isStringEquals('b', theString))",
	"select * from SupportBean(" + efeStaticFuncLib + ".isStringEquals('bx', theString || 'x'))",
	"select * from SupportBean('b'=theString," + efeStaticFuncLib + ".isStringEquals('bx', theString || 'x'))",
	"select * from SupportBean('b'=theString, theString='b', theString != 'a')",
	"select * from SupportBean(theString != 'a', theString != 'c')",
	"select * from SupportBean(theString = 'b', theString != 'c')",
	"select * from SupportBean(theString != 'a' and theString != 'c')",
	"select * from SupportBean(theString = 'a' and theString = 'c' and " + efeStaticFuncLib + ".isStringEquals('bx', theString || 'x'))",
}

// efeInstanceMethodBodies pins the three tryFilterInstanceMethod EPL bodies;
// the harness wraps each as "@name('s0') " + body. The '*' wildcard argument
// passes the same stream EventBean as the s0 alias.
var efeInstanceMethodBodies = []string{
	"select * from SupportInstanceMethodBean(s0.myInstanceMethodAlwaysTrue()) as s0",
	"select * from SupportInstanceMethodBean(s0.myInstanceMethodEventBean(s0, 'x', 1)) as s0",
	"select * from SupportInstanceMethodBean(s0.myInstanceMethodEventBean(*, 'x', 1)) as s0",
}

var efeCaseEPLs = []string{
	"@name('s0') select * from pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)]\n" +
		"@name('s1') select * from pattern [every event1=SupportTradeEvent(userId in ('100','101'))]\n",
	"@name('s0') " + efeStaticFuncBodies[0] + "\n" +
		"@name('s1') " + efeStaticFuncBodies[1] + "\n" +
		"@name('s2') " + efeStaticFuncBodies[2] + "\n" +
		"@name('s3') " + efeStaticFuncBodies[3] + "\n" +
		"@name('s4') " + efeStaticFuncBodies[4] + "\n" +
		"@name('s5') " + efeStaticFuncBodies[5] + "\n" +
		"@name('s6') " + efeStaticFuncBodies[6] + "\n" +
		"@name('s7') " + efeStaticFuncBodies[7] + "\n",
	"@name('s0') " + efeInstanceMethodBodies[0] + "\n" +
		"@name('s0') " + efeInstanceMethodBodies[1] + "\n" +
		"@name('s0') " + efeInstanceMethodBodies[2] + "\n",
}

var (
	efeJavaRuntimeIDs = []string{
		"java-runtime-e9c9627ad604f3404620", // ExprFilterOverInClause
		"java-runtime-15341d9e0dc15c4b2fc3", // ExprFilterStaticFunc
		"java-runtime-0a80365acd3e6ab25238", // ExprFilterInstanceMethodWWildcard
	}
	efeJavaExecutions = []string{
		"ExprFilterOverInClause",
		"ExprFilterStaticFunc",
		"ExprFilterInstanceMethodWWildcard",
	}
	efeJavaStaticIDs = []string{
		"java-9d2f2743881d949bbddc",
		"java-314bbbdca28d0445fdd0",
		"java-576e4853b035fd6064f5",
	}
	efeJavaFlags = []string{}
	efeCases     = []string{
		"over-in-clause",
		"static-func",
		"instance-method-wildcard",
	}
	efeOrdinals = []int{7, 17, 27}
	efeSources  = []string{efeSource}
)

// efeTradeBean mirrors SupportTradeEvent(id, userId, ccypair, direction,
// amount): the 3-argument Java constructor leaves ccypair/direction null, so
// they are pointer fields rendering the {state:null} marker.
type efeTradeBean struct {
	ID        int32   `esper:"id"`
	UserID    *string `esper:"userId"`
	CcyPair   *string `esper:"ccypair"`
	Direction *string `esper:"direction"`
	Amount    int32   `esper:"amount"`
}

// efeInstanceBean mirrors SupportInstanceMethodBean(x): the Java bean exposes
// getX plus the two filter methods; myInstanceMethodEventBean reads the
// property through the passed EventBean, so the Go method takes the event
// argument like the Java signature (the '*' wildcard and the s0 alias both
// pass the same stream event).
type efeInstanceBean struct {
	X int `esper:"x"`
}

// MyInstanceMethodAlwaysTrue mirrors SupportInstanceMethodBean.
// myInstanceMethodAlwaysTrue(): constant true.
func (b efeInstanceBean) MyInstanceMethodAlwaysTrue() bool { return true }

// MyInstanceMethodEventBean mirrors SupportInstanceMethodBean.
// myInstanceMethodEventBean(EventBean, String, int): event.get(propertyName)
// .equals(expected).
func (b efeInstanceBean) MyInstanceMethodEventBean(event esper.Event, propertyName string, expected int) bool {
	value := event.Get(propertyName)
	if !value.IsPresent() || value.IsNull() {
		return false
	}
	actual, ok := value.Any().(int)
	return ok && actual == expected
}

// efeCaseSteps pins the complete step sequence per case. Deploy steps carry
// the byte-exact EPL the Java harness compiles; send steps carry the
// compacted payload including the per-statement expected invoked flags the
// Java assertions pin; undeploy-all steps close each deploy group.
var efeCaseSteps = map[string][]string{
	"over-in-clause": {
		"deploy:s0:@name('s0') select * from pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)]",
		"deployed:s0",
		"send:SupportTradeEvent:{\"id\":1,\"userId\":\"100\",\"amount\":1001,\"expected\":{\"s0\":true}}",
		"deploy:s1:@name('s1') select * from pattern [every event1=SupportTradeEvent(userId in ('100','101'))]",
		"deployed:s1",
		"send:SupportTradeEvent:{\"id\":2,\"userId\":\"100\",\"amount\":1001,\"expected\":{\"s0\":true,\"s1\":true}}",
		"undeploy-all:",
	},
	"static-func": {
		"deploy:s0:@name('s0') select * from SupportBean(com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.isStringEquals('b', theString))",
		"deployed:s0",
		"deploy:s1:@name('s1') select * from SupportBean(com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.isStringEquals('bx', theString || 'x'))",
		"deployed:s1",
		"deploy:s2:@name('s2') select * from SupportBean('b'=theString,com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.isStringEquals('bx', theString || 'x'))",
		"deployed:s2",
		"deploy:s3:@name('s3') select * from SupportBean('b'=theString, theString='b', theString != 'a')",
		"deployed:s3",
		"deploy:s4:@name('s4') select * from SupportBean(theString != 'a', theString != 'c')",
		"deployed:s4",
		"deploy:s5:@name('s5') select * from SupportBean(theString = 'b', theString != 'c')",
		"deployed:s5",
		"deploy:s6:@name('s6') select * from SupportBean(theString != 'a' and theString != 'c')",
		"deployed:s6",
		"deploy:s7:@name('s7') select * from SupportBean(theString = 'a' and theString = 'c' and com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.isStringEquals('bx', theString || 'x'))",
		"deployed:s7",
		"send:SupportBean:{\"theString\":\"a\",\"expected\":{\"s0\":false,\"s1\":false,\"s2\":false,\"s3\":false,\"s4\":false,\"s5\":false,\"s6\":false,\"s7\":false}}",
		"send:SupportBean:{\"theString\":\"b\",\"expected\":{\"s0\":true,\"s1\":true,\"s2\":true,\"s3\":true,\"s4\":true,\"s5\":true,\"s6\":true,\"s7\":false}}",
		"send:SupportBean:{\"theString\":\"c\",\"expected\":{\"s0\":false,\"s1\":false,\"s2\":false,\"s3\":false,\"s4\":false,\"s5\":false,\"s6\":false,\"s7\":false}}",
		"undeploy-all:",
	},
	"instance-method-wildcard": {
		"deploy:s0:@name('s0') select * from SupportInstanceMethodBean(s0.myInstanceMethodAlwaysTrue()) as s0",
		"deployed:s0",
		"send:SupportInstanceMethodBean:{\"x\":0,\"expected\":{\"s0\":true}}",
		"send:SupportInstanceMethodBean:{\"x\":1,\"expected\":{\"s0\":true}}",
		"send:SupportInstanceMethodBean:{\"x\":2,\"expected\":{\"s0\":true}}",
		"undeploy-all:",
		"deploy:s0:@name('s0') select * from SupportInstanceMethodBean(s0.myInstanceMethodEventBean(s0, 'x', 1)) as s0",
		"deployed:s0",
		"send:SupportInstanceMethodBean:{\"x\":0,\"expected\":{\"s0\":false}}",
		"send:SupportInstanceMethodBean:{\"x\":1,\"expected\":{\"s0\":true}}",
		"send:SupportInstanceMethodBean:{\"x\":2,\"expected\":{\"s0\":false}}",
		"undeploy-all:",
		"deploy:s0:@name('s0') select * from SupportInstanceMethodBean(s0.myInstanceMethodEventBean(*, 'x', 1)) as s0",
		"deployed:s0",
		"send:SupportInstanceMethodBean:{\"x\":0,\"expected\":{\"s0\":false}}",
		"send:SupportInstanceMethodBean:{\"x\":1,\"expected\":{\"s0\":true}}",
		"send:SupportInstanceMethodBean:{\"x\":2,\"expected\":{\"s0\":false}}",
		"undeploy-all:",
	},
}

// executeEfe replays the pinned step stream. Listener callbacks buffer
// deliveries per statement; each send drains them in deploy order and emits
// one record per deployed statement — a listener record carrying the row on
// fire, or a listener-not-invoked count record — mirroring the Java oracle's
// assertEqualsNew/assertListenerInvokedFlag checks. A delivery that
// contradicts the pinned expected flag is a replay error. All records share
// one case-local sequence so the record order is the step order.
func executeEfe(ctx context.Context, steps []compat.Step) (compat.Trace, error) {
	caseName := ""
	caseIndex := -1
	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	var deployedOrder []string
	pending := map[string][]esper.ResultBatch{}
	var sequence uint64
	trace := compat.Trace{Version: "esper-parity/v1", ID: efeID}

	emit := func(record compat.TraceRecord) {
		sequence++
		record.Sequence = sequence
		trace.Records = append(trace.Records, record)
	}
	// drain emits one record per deployed statement in deploy order: a
	// listener record when the send fired the statement (the buffered
	// batches), or a listener-not-invoked count record. The pinned expected
	// flags must cover exactly the deployed statements.
	drain := func(expected map[string]bool) error {
		if len(expected) != len(deployedOrder) {
			return fmt.Errorf("%s case %q send expected %d statements, %d deployed", efeID, caseName, len(expected), len(deployedOrder))
		}
		for _, name := range deployedOrder {
			want, ok := expected[name]
			if !ok {
				return fmt.Errorf("%s case %q send has no expected flag for %q", efeID, caseName, name)
			}
			batches := pending[name]
			delete(pending, name)
			if (len(batches) > 0) != want {
				return fmt.Errorf("%s case %q statement %q fired=%t, want %t", efeID, caseName, name, len(batches) > 0, want)
			}
			for _, batch := range batches {
				rec := compat.TraceRecord{
					Case:      caseName,
					Operation: "listener",
					Statement: name,
					Time:      compat.FormatTraceTime(batch.Time),
					New:       compat.NormalizeResults(batch.New),
					Old:       compat.NormalizeResults(batch.Old),
				}
				if len(rec.Old) == 0 {
					rec.Old = nil
				}
				emit(rec)
			}
			if len(batches) == 0 {
				zero := int64(0)
				emit(compat.TraceRecord{
					Case:      caseName,
					Operation: "count",
					Statement: name,
					Time:      compat.FormatTraceTime(engine.Now()),
					Name:      "listener-not-invoked",
					Count:     &zero,
				})
			}
		}
		if len(pending) != 0 {
			return fmt.Errorf("%s case %q send delivered to undeployed statements", efeID, caseName)
		}
		return nil
	}
	startCase := func() error {
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[efabBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[efeTradeBean](env, "SupportTradeEvent"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[efeInstanceBean](env, "SupportInstanceMethodBean"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithRuntimeURI(efeJavaRuntimeIDs[caseIndex]),
			esper.WithStartTime(time.Unix(0, 0).UTC()))
		return nil
	}
	defer func() {
		if engine != nil {
			_ = engine.Close(context.Background())
		}
	}()

	for _, step := range steps {
		switch step.Op {
		case "case":
			caseName = step.Case
			caseIndex++
			sequence = 0
			deployments = map[string]*esper.Deployment{}
			deployedOrder = nil
			pending = map[string][]esper.ResultBatch{}
			if err := startCase(); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			plan, err := efeBuild(env, step.Statement, step.Epl)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			deployedOrder = append(deployedOrder, step.Statement)
			for _, statement := range deployment.Statements() {
				name := statement.Name()
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					if len(batch.New) == 0 && len(batch.Old) == 0 {
						return nil
					}
					pending[name] = append(pending[name], batch)
					return nil
				}); err != nil {
					return compat.Trace{}, err
				}
			}
		case "deployed":
			if _, ok := deployments[step.Statement]; !ok {
				return compat.Trace{}, fmt.Errorf("deployed %q/%q: no deployment", caseName, step.Statement)
			}
			emit(compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			fields, expected, err := decodeEfePayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := efeSend(ctx, engine, step.EventType, fields); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
			if err := drain(expected); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
			deployedOrder = nil
			pending = map[string][]esper.ResultBatch{}
		default:
			return compat.Trace{}, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

// efeBuild maps one pinned deploy EPL to its fluent plan. The @name prefix is
// stripped and the remaining body selects the filter construction; the
// statement name comes from the step so the listener record keys match
// Java's @name.
func efeBuild(env *esper.Environment, statement, epl string) (esper.Plan, error) {
	body := epl
	if strings.HasPrefix(body, "@name('") {
		if end := strings.Index(body, "')"); end >= 0 {
			body = strings.TrimPrefix(body[end+2:], " ")
		}
	}
	bean := func() esper.Stream[efabBean] { return esper.From[efabBean](env, "SupportBean") }
	trade := func() esper.Stream[efeTradeBean] { return esper.From[efeTradeBean](env, "SupportTradeEvent") }
	instance := func() esper.Stream[efeInstanceBean] {
		return esper.From[efeInstanceBean](env, "SupportInstanceMethodBean")
	}
	ts := func() esper.Expression[*string] { return esper.Field[efabBean, *string]("theString") }
	uid := func() esper.Expression[*string] { return esper.Field[efeTradeBean, *string]("userId") }
	amt := func() esper.Expression[int32] { return esper.Field[efeTradeBean, int32]("amount") }
	// isStringEquals mirrors SupportStaticMethodLib.isStringEquals:
	// value.equals(compareTo) — the first argument is the constant, the
	// second the compared value.
	isStringEquals := func(prefix, value string) bool { return prefix == value }

	var filter esper.Expression[bool]
	switch body {
	case "select * from pattern[every event1=SupportTradeEvent(userId in ('100','101'),amount>=1000)]":
		pattern := esper.PatternFrom(trade(), "event1", esper.And(
			esper.InOf(uid(), esper.Literal("100"), esper.Literal("101")),
			esper.GreaterOrEqualOf(amt(), esper.Literal(int64(1000))),
		)).Every()
		return env.Build(pattern.Select(
			esper.Alias("event1", efoTagMap(esper.PatternEvent("event1"))),
		).Query(esper.StatementName(statement)))
	case "select * from pattern [every event1=SupportTradeEvent(userId in ('100','101'))]":
		pattern := esper.PatternFrom(trade(), "event1",
			esper.InOf(uid(), esper.Literal("100"), esper.Literal("101"))).Every()
		return env.Build(pattern.Select(
			esper.Alias("event1", efoTagMap(esper.PatternEvent("event1"))),
		).Query(esper.StatementName(statement)))
	case efeStaticFuncBodies[0]:
		filter = esper.Func2[string, *string, bool]("isStringEquals",
			func(prefix string, value *string) bool { return value != nil && prefix == *value },
			esper.Literal("b"), ts())
	case efeStaticFuncBodies[1]:
		filter = esper.Func2[string, string, bool]("isStringEquals", isStringEquals,
			esper.Literal("bx"), esper.ConcatOf(ts(), esper.Literal("x")))
	case efeStaticFuncBodies[2]:
		filter = esper.And(
			esper.EqualOf(esper.Literal("b"), ts()),
			esper.Func2[string, string, bool]("isStringEquals", isStringEquals,
				esper.Literal("bx"), esper.ConcatOf(ts(), esper.Literal("x"))))
	case efeStaticFuncBodies[3]:
		filter = esper.And(
			esper.And(
				esper.EqualOf(esper.Literal("b"), ts()),
				esper.EqualOf(ts(), esper.Literal("b"))),
			esper.NotEqualOf(ts(), esper.Literal("a")))
	case efeStaticFuncBodies[4]:
		filter = esper.And(
			esper.NotEqualOf(ts(), esper.Literal("a")),
			esper.NotEqualOf(ts(), esper.Literal("c")))
	case efeStaticFuncBodies[5]:
		filter = esper.And(
			esper.EqualOf(ts(), esper.Literal("b")),
			esper.NotEqualOf(ts(), esper.Literal("c")))
	case efeStaticFuncBodies[6]:
		filter = esper.And(
			esper.NotEqualOf(ts(), esper.Literal("a")),
			esper.NotEqualOf(ts(), esper.Literal("c")))
	case efeStaticFuncBodies[7]:
		filter = esper.And(
			esper.And(
				esper.EqualOf(ts(), esper.Literal("a")),
				esper.EqualOf(ts(), esper.Literal("c"))),
			esper.Func2[string, string, bool]("isStringEquals", isStringEquals,
				esper.Literal("bx"), esper.ConcatOf(ts(), esper.Literal("x"))))
	case efeInstanceMethodBodies[0]:
		filter = esper.Method[bool](
			esper.EventValue[efeInstanceBean](), "MyInstanceMethodAlwaysTrue")
	case efeInstanceMethodBodies[1], efeInstanceMethodBodies[2]:
		// The s0 alias and the '*' wildcard both pass the stream's EventBean;
		// the stream-function precedent (epl_other_stream_expr.go) collapses
		// both Java argument forms onto EventValue[Event].
		filter = esper.Method[bool](
			esper.EventValue[efeInstanceBean](), "MyInstanceMethodEventBean",
			esper.EventValue[esper.Event](), esper.Literal("x"), esper.Literal(1))
	default:
		return esper.Plan{}, fmt.Errorf("%s statement %q has no plan for EPL %q", efeID, statement, epl)
	}
	if strings.HasPrefix(body, "select * from SupportInstanceMethodBean") {
		return env.Build(esper.Select(instance().Filter(filter)).Query(esper.StatementName(statement)))
	}
	return env.Build(esper.Select(bean().Filter(filter)).Query(esper.StatementName(statement)))
}

// efeSend delivers one pinned event. SupportBean mirrors sendBeanString
// (new SupportBean(theString, -1)): intPrimitive is -1 and every other
// column keeps the Java bean default (charPrimitive ' ', boxed/string
// columns null). SupportTradeEvent mirrors the 3-argument constructor
// (ccypair/direction null). SupportInstanceMethodBean carries x.
func efeSend(ctx context.Context, engine *esper.Engine, eventType string, fields map[string]json.RawMessage) error {
	switch eventType {
	case "SupportBean":
		// sendBeanString uses new SupportBean(theString, -1): intPrimitive=-1,
		// charPrimitive at the Java bean default '\u0000', boxed columns null.
		bean := efabBean{CharPrimitive: "\u0000", IntPrimitive: -1}
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "theString":
				if s, ok := value.(string); ok {
					bean.TheString = &s
				}
			default:
				return fmt.Errorf("unknown %s field %q", eventType, key)
			}
		}
		return engine.Send(ctx, eventType, bean)
	case "SupportTradeEvent":
		var bean efeTradeBean
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "id":
				bean.ID = int32(int64Field(value))
			case "userId":
				if s, ok := value.(string); ok {
					bean.UserID = &s
				}
			case "ccypair":
				if s, ok := value.(string); ok {
					bean.CcyPair = &s
				}
			case "direction":
				if s, ok := value.(string); ok {
					bean.Direction = &s
				}
			case "amount":
				bean.Amount = int32(int64Field(value))
			default:
				return fmt.Errorf("unknown %s field %q", eventType, key)
			}
		}
		return engine.Send(ctx, eventType, bean)
	case "SupportInstanceMethodBean":
		var bean efeInstanceBean
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "x":
				bean.X = int(int64Field(value))
			default:
				return fmt.Errorf("unknown %s field %q", eventType, key)
			}
		}
		return engine.Send(ctx, eventType, bean)
	}
	return fmt.Errorf("unknown event type %q", eventType)
}

// decodeEfePayload splits a send payload into the event fields and the
// per-statement expected invoked flags the Java assertions pin.
func decodeEfePayload(step compat.Step) (map[string]json.RawMessage, map[string]bool, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, nil, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	rawExpected, ok := fields["expected"]
	if !ok {
		return nil, nil, fmt.Errorf("decode %s payload: missing expected flags", step.EventType)
	}
	delete(fields, "expected")
	var expected map[string]bool
	if err := json.Unmarshal(rawExpected, &expected); err != nil || expected == nil {
		return nil, nil, fmt.Errorf("decode %s expected flags: %w", step.EventType, err)
	}
	return fields, expected, nil
}

func runEfeScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	return executeEfe(ctx, scenario.Steps)
}

func loadEfeScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", efeID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", efeID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", efeID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", efeID, err)
	}
	if err := requireEfeFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", efeID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != efeID ||
		metadata.Description != efeDescription ||
		metadata.JavaCommit != efeJavaCommit ||
		metadata.JavaSource != efeSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", efeID)
	}
	if err := validateEfeStringArray(root["javaRuntimes"], efeJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEfeStringArray(root["javaNames"], efeJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEfeStringArray(root["javaStaticIds"], efeJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEfeStringArray(root["javaFlags"], efeJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(efeCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", efeID, len(efeCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEfeFields(object,
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
		if definition.Case != efeCases[index] ||
			definition.Ordinal != efeOrdinals[index] ||
			definition.RuntimeID != efeJavaRuntimeIDs[index] ||
			definition.ExecutionName != efeJavaExecutions[index] ||
			definition.Observation != efeCaseObservations[index] ||
			definition.EPL != efeCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", efeID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", efeID, err)
	}
	offset := 0
	for _, caseName := range efeCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", efeID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", efeID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", efeID, offset, caseName)
		}
		offset++
		want, ok := efeCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", efeID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", efeID, caseName)
		}
		for _, pinned := range want {
			var step struct {
				Op        string          `json:"op"`
				Case      string          `json:"case"`
				Statement string          `json:"statement"`
				EventType string          `json:"eventType"`
				Epl       string          `json:"epl"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", efeID, offset, err)
			}
			if step.Case != caseName {
				return compat.Scenario{}, fmt.Errorf("%s step %d case = %q, want %q", efeID, offset, step.Case, caseName)
			}
			key := step.Op + ":" + step.Statement + step.EventType
			if step.Op == "send" {
				var compacted bytes.Buffer
				if err := json.Compact(&compacted, step.Payload); err != nil {
					return compat.Scenario{}, fmt.Errorf("%s step %d payload: %w", efeID, offset, err)
				}
				key += ":" + compacted.String()
			}
			if step.Op == "deploy" {
				key += ":" + step.Epl
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", efeID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", efeID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", efeID, err)
	}
	return scenario, nil
}

func requireEfeFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s fields = %d, want %d", efeID, len(object), len(names))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s missing field %q", efeID, name)
		}
	}
	return nil
}

func validateEfeStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("%s %s: %w", efeID, name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s %s has %d entries, want %d", efeID, name, len(values), len(expected))
	}
	for index, value := range values {
		if value != expected[index] {
			return fmt.Errorf("%s %s[%d] = %q, want %q", efeID, name, index, value, expected[index])
		}
	}
	return nil
}
