package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	efabID         = "expr-filter-in-and-between"
	efabJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	efabSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterInAndBetween.java"
)

const efabDescription = "ExprFilterInAndBetween in/not-in/between filter semantics (pinned 9e1b9f1cc9117fea4bf33ab043762c045d73839c): dynamic in-sets over pattern tags (ord 0), the tryReuse deploy/send/undeployModuleContaining protocol over 8 in-groups (ord 5) and 4 not-in groups (ord 6), and deploy-order-sensitive multi-statement filters with like clauses (ords 7-8). Ord 4 (ExprFilterInInvalid) is a compile-failure-only execution gated on JVM filter index planning and carries no scenario case. Java milestones are ordering markers with no virtual time and are implicit in the step order; assertListenerInvoked/NotInvoked/assertEventNew map to listener record presence/absence in the trace (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterInAndBetween.java)."

var efabCaseObservations = []string{
	"listener; two sequential deployments of a dynamic in-set over pattern tags: a=SupportBeanNumeric -> every b=SupportBean(intPrimitive in (a.intOne, a.intTwo)) then a=SupportBean_S0 -> every b=SupportBean(theString in (a.p00, a.p01, a.p02)); the undeployAll between phases keeps the s0 listener sequence running (2 fires in phase A, 3 in phase B)",
	"listener; ExprFilterReuse tryReuse protocol over 8 groups: each statement deploys as its own module, one send fires every listener, then undeployModuleContaining removes s0..sn-1 one at a time with a send after each so only the surviving listeners fire; a final send fires none",
	"listener; ExprFilterReuseNot tryReuse protocol over 4 groups of not-in/not-between filters (including reversed-bound not between 0 and -3 and open-both-ends not in (3:4)); same deploy/send/undeploy cadence as in-reuse-undeploy",
	"listener; deploy order matters: the non-matching statement A (intPrimitive in (0,0,1) and theString like 'X%') deploys before matching B (intPrimitive in (0,1) and theString like 'A%'); one send delivers exactly one new event to B and nothing to A",
	"listener; s1 (intPrimitive in (0) and theString like 'X%') deploys before s2 (intPrimitive in (0,1) and theString like 'A%'); one send delivers exactly one new event to s2 and nothing to s1",
}

var efabCaseEPLs = []string{
	"@name('s0') select * from pattern [a=SupportBeanNumeric -> every b=SupportBean(intPrimitive in (a.intOne, a.intTwo))]\n@name('s0') select * from pattern [a=SupportBean_S0 -> every b=SupportBean(theString in (a.p00, a.p01, a.p02))]\n",
	"@name('s0')select * from SupportBean(intBoxed in [2:4]);\n@name('s1')select * from SupportBean(intBoxed in [2:4]);\n@name('s0')select * from SupportBean(intBoxed in (1, 2, 3));\n@name('s1')select * from SupportBean(intBoxed in (1, 2, 3));\n@name('s0')select * from SupportBean(intBoxed in (2:3]);\n@name('s1')select * from SupportBean(intBoxed in (1:3]);\n@name('s0')select * from SupportBean(intBoxed in (2, 3, 4));\n@name('s1')select * from SupportBean(intBoxed in (1, 3));\n@name('s0')select * from SupportBean(intBoxed in (2, 3, 4));\n@name('s1')select * from SupportBean(intBoxed in (1, 3));\n@name('s2')select * from SupportBean(intBoxed in (8, 3));\n@name('s0')select * from SupportBean(intBoxed in (3, 1, 3));\n@name('s1')select * from SupportBean(intBoxed in (3, 3));\n@name('s2')select * from SupportBean(intBoxed in (1, 3));\n@name('s0')select * from SupportBean(boolPrimitive=false, intBoxed in (1, 2, 3));\n@name('s1')select * from SupportBean(boolPrimitive=false, intBoxed in (3, 4));\n@name('s2')select * from SupportBean(boolPrimitive=false, intBoxed in (3));\n@name('s0')select * from SupportBean(intBoxed in (1, 2, 3), longPrimitive >= 0);\n@name('s1')select * from SupportBean(intBoxed in (3, 4), intPrimitive >= 0);\n@name('s2')select * from SupportBean(intBoxed in (3), bytePrimitive < 1);\n",
	"@name('s0')select * from SupportBean(intBoxed not in [1:2]);\n@name('s1')select * from SupportBean(intBoxed not in [1:2]);\n@name('s0')select * from SupportBean(intBoxed in (3, 1, 3));\n@name('s1')select * from SupportBean(intBoxed not in (2, 1));\n@name('s2')select * from SupportBean(intBoxed not between 0 and -3);\n@name('s0')select * from SupportBean(intBoxed not in (1, 4, 5));\n@name('s1')select * from SupportBean(intBoxed not in (1, 4, 5));\n@name('s2')select * from SupportBean(intBoxed not in (4, 5, 1));\n@name('s0')select * from SupportBean(intBoxed not in (3:4));\n@name('s1')select * from SupportBean(intBoxed not in [1:3));\n@name('s2')select * from SupportBean(intBoxed not in (1,1,1,33));\n",
	"@name('A') select * from SupportBean(intPrimitive in (0,0,1) and theString like 'X%')\n@name('B') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')\n",
	"@name('s1') select * from SupportBean(intPrimitive in (0) and theString like 'X%')\n@name('s2') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')\n",
}

var (
	efabJavaRuntimeIDs = []string{
		"java-runtime-0d06d25e978384a0b008",
		"java-runtime-d16fe1fd7693643a5001",
		"java-runtime-9245458815076f0baee3",
		"java-runtime-a3336b696b1ae9b2c831",
		"java-runtime-ed506bfbf3734b4fff03",
	}
	efabJavaExecutions = []string{
		"ExprFilterInDynamic",
		"ExprFilterReuse",
		"ExprFilterReuseNot",
		"ExprFilterInMultipleNonMatchingFirst",
		"ExprFilterInMultipleWithBool",
	}
	efabJavaStaticIDs = []string{
		"java-17cece2bf9c2df0b27f1",
		"java-17cece2bf9c2df0b27f1",
		"java-17cece2bf9c2df0b27f1",
		"java-17cece2bf9c2df0b27f1",
		"java-17cece2bf9c2df0b27f1",
	}
	efabJavaFlags = []string{"OBSERVEROPS"}
	efabCases     = []string{
		"in-dynamic-pattern",
		"in-reuse-undeploy",
		"not-in-reuse-undeploy",
		"in-multiple-nonmatching-first",
		"in-multiple-with-bool",
	}
	efabOrdinals = []int{0, 5, 6, 7, 8}
	efabSources  = []string{efabSource}
)

// efabBean mirrors the full SupportBean schema. Nullable columns are pointer
// fields so Java's bean defaults (theString=null, boxed=null) render as the
// {state:null} marker; charPrimitive is a string carrying Java's '\u0000'
// default, matching the Java oracle's String.valueOf(char) rendering.
type efabBean struct {
	TheString       *string  `esper:"theString"`
	BoolPrimitive   bool     `esper:"boolPrimitive"`
	IntPrimitive    int32    `esper:"intPrimitive"`
	LongPrimitive   int64    `esper:"longPrimitive"`
	CharPrimitive   string   `esper:"charPrimitive"`
	ShortPrimitive  int16    `esper:"shortPrimitive"`
	BytePrimitive   int8     `esper:"bytePrimitive"`
	FloatPrimitive  float32  `esper:"floatPrimitive"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	BoolBoxed       *bool    `esper:"boolBoxed"`
	IntBoxed        *int32   `esper:"intBoxed"`
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

// efabNumericBean mirrors SupportBeanNumeric(intOne, intTwo): the constructor
// leaves bigint/bigdec/bigdecTwo null and the primitive doubles/floats at 0.
type efabNumericBean struct {
	IntOne    *int64   `esper:"intOne"`
	IntTwo    *int64   `esper:"intTwo"`
	Bigint    *big.Int `esper:"bigint"`
	Bigdec    *big.Rat `esper:"bigdec"`
	BigdecTwo *big.Rat `esper:"bigdecTwo"`
	DoubleOne float64  `esper:"doubleOne"`
	DoubleTwo float64  `esper:"doubleTwo"`
	FloatOne  float32  `esper:"floatOne"`
	FloatTwo  float32  `esper:"floatTwo"`
}

// efabCaseSteps pins the complete step sequence per case. Deploy steps carry
// the byte-exact EPL the Java harness compiles (the reuse groups concatenate
// "@name('sN')" with the filter body, so there is no space after the
// annotation); send steps carry the compacted payload; undeploy steps carry
// the statement whose module Java's undeployModuleContaining removes.
var efabCaseSteps = map[string][]string{
	"in-dynamic-pattern": {
		"deploy:s0:@name('s0') select * from pattern [a=SupportBeanNumeric -> every b=SupportBean(intPrimitive in (a.intOne, a.intTwo))]",
		"deployed:s0",
		"send:SupportBeanNumeric:{\"intOne\":10,\"intTwo\":20}",
		"send:SupportBean:{\"intPrimitive\":10}",
		"send:SupportBean:{\"intPrimitive\":11}",
		"send:SupportBean:{\"intPrimitive\":20}",
		"undeploy-all:",
		"deploy:s0:@name('s0') select * from pattern [a=SupportBean_S0 -> every b=SupportBean(theString in (a.p00, a.p01, a.p02))]",
		"deployed:s0",
		"send:SupportBean_S0:{\"id\":1,\"p00\":\"a\",\"p01\":\"b\",\"p02\":\"c\",\"p03\":\"d\"}",
		"send:SupportBean:{\"theString\":\"a\"}",
		"send:SupportBean:{\"theString\":\"x\"}",
		"send:SupportBean:{\"theString\":\"b\"}",
		"send:SupportBean:{\"theString\":\"c\"}",
		"send:SupportBean:{\"theString\":\"d\"}",
		"undeploy-all:",
	},
	"in-reuse-undeploy": {
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in [2:4])",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in [2:4])",
		"deployed:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (1, 2, 3))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in (1, 2, 3))",
		"deployed:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		// Java evaluates range filters in TreeMap (min,max) index order, so
		// (1:3] fires before (2:3] regardless of deploy order; Go dispatches in
		// deploy order, hence s1 deploys first to keep traces aligned.
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in (1:3])",
		"deployed:s1",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (2:3])",
		"deployed:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (2, 3, 4))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in (1, 3))",
		"deployed:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (2, 3, 4))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in (1, 3))",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(intBoxed in (8, 3))",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (3, 1, 3))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in (3, 3))",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(intBoxed in (1, 3))",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(boolPrimitive=false, intBoxed in (1, 2, 3))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(boolPrimitive=false, intBoxed in (3, 4))",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(boolPrimitive=false, intBoxed in (3))",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (1, 2, 3), longPrimitive >= 0)",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed in (3, 4), intPrimitive >= 0)",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(intBoxed in (3), bytePrimitive < 1)",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
	},
	"not-in-reuse-undeploy": {
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed not in [1:2])",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed not in [1:2])",
		"deployed:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed in (3, 1, 3))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed not in (2, 1))",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(intBoxed not between 0 and -3)",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed not in (1, 4, 5))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed not in (1, 4, 5))",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(intBoxed not in (4, 5, 1))",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
		"deploy:s0:@name('s0')select * from SupportBean(intBoxed not in (3:4))",
		"deployed:s0",
		"deploy:s1:@name('s1')select * from SupportBean(intBoxed not in [1:3))",
		"deployed:s1",
		"deploy:s2:@name('s2')select * from SupportBean(intBoxed not in (1,1,1,33))",
		"deployed:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s0",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s1",
		"send:SupportBean:{\"intBoxed\":3}",
		"undeploy:s2",
		"send:SupportBean:{\"intBoxed\":3}",
		"send:SupportBean:{\"intBoxed\":3}",
	},
	"in-multiple-nonmatching-first": {
		"deploy:A:@name('A') select * from SupportBean(intPrimitive in (0,0,1) and theString like 'X%')",
		"deployed:A",
		"deploy:B:@name('B') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')",
		"deployed:B",
		"send:SupportBean:{\"theString\":\"A\",\"intPrimitive\":0}",
		"undeploy-all:",
	},
	"in-multiple-with-bool": {
		"deploy:s1:@name('s1') select * from SupportBean(intPrimitive in (0) and theString like 'X%')",
		"deployed:s1",
		"deploy:s2:@name('s2') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')",
		"deployed:s2",
		"send:SupportBean:{\"theString\":\"A\",\"intPrimitive\":1}",
		"undeploy-all:",
	},
}

func executeEfab(ctx context.Context, steps []compat.Step) (compat.Trace, error) {
	caseName := ""
	caseIndex := -1
	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: efabID}
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
	startCase := func() error {
		env = esper.NewEnvironment()
		if _, err := esper.RegisterStruct[efabBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[efabNumericBean](env, "SupportBeanNumeric"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[efovS0](env, "SupportBean_S0"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithRuntimeURI(efabJavaRuntimeIDs[caseIndex]),
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
			sequence = map[string]uint64{}
			deployments = map[string]*esper.Deployment{}
			if err := startCase(); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			plan, err := efabBuild(env, step.Statement, step.Epl)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			for _, statement := range deployment.Statements() {
				name := statement.Name()
				if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
					record(name, batch)
					return nil
				}); err != nil {
					return compat.Trace{}, err
				}
			}
		case "deployed":
			sequence[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(engine.Now()),
			})
		case "send":
			fields, err := decodeEfabPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := efabSend(ctx, engine, step.EventType, fields); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "undeploy":
			// undeployModuleContaining: every statement deploys as its own
			// module, so the statement key selects the whole deployment.
			deployment, ok := deployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("undeploy %q/%q: no deployment", caseName, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, fmt.Errorf("undeploy %q/%q: %w", caseName, step.Statement, err)
			}
			delete(deployments, step.Statement)
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
		default:
			return compat.Trace{}, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

// efabBuild maps one pinned deploy EPL to its fluent plan. The @name prefix
// is stripped (the reuse groups concatenate it without a separating space)
// and the remaining body selects the filter construction; the statement name
// comes from the step so the listener record keys match Java's @name.
func efabBuild(env *esper.Environment, statement, epl string) (esper.Plan, error) {
	body := epl
	if strings.HasPrefix(body, "@name('") {
		if end := strings.Index(body, "')"); end >= 0 {
			body = strings.TrimPrefix(body[end+2:], " ")
		}
	}
	bean := func() esper.Stream[efabBean] { return esper.From[efabBean](env, "SupportBean") }
	ib := func() esper.Expression[*int32] { return esper.Field[efabBean, *int32]("intBoxed") }
	ip := func() esper.Expression[int32] { return esper.Field[efabBean, int32]("intPrimitive") }
	ts := func() esper.Expression[*string] { return esper.Field[efabBean, *string]("theString") }
	bp := func() esper.Expression[bool] { return esper.Field[efabBean, bool]("boolPrimitive") }
	lp := func() esper.Expression[int64] { return esper.Field[efabBean, int64]("longPrimitive") }
	byp := func() esper.Expression[int8] { return esper.Field[efabBean, int8]("bytePrimitive") }
	lit := func(v int64) esper.Expression[int64] { return esper.Literal(v) }

	var filter esper.Expression[bool]
	switch body {
	case "select * from SupportBean(intBoxed in [2:4])":
		filter = esper.BetweenRangeOf(ib(), lit(2), lit(4), true, true)
	case "select * from SupportBean(intBoxed in (1, 2, 3))":
		filter = esper.InOf(ib(), lit(1), lit(2), lit(3))
	case "select * from SupportBean(intBoxed in (2:3])":
		filter = esper.BetweenRangeOf(ib(), lit(2), lit(3), false, true)
	case "select * from SupportBean(intBoxed in (1:3])":
		filter = esper.BetweenRangeOf(ib(), lit(1), lit(3), false, true)
	case "select * from SupportBean(intBoxed in (2, 3, 4))":
		filter = esper.InOf(ib(), lit(2), lit(3), lit(4))
	case "select * from SupportBean(intBoxed in (1, 3))":
		filter = esper.InOf(ib(), lit(1), lit(3))
	case "select * from SupportBean(intBoxed in (8, 3))":
		filter = esper.InOf(ib(), lit(8), lit(3))
	case "select * from SupportBean(intBoxed in (3, 1, 3))":
		filter = esper.InOf(ib(), lit(3), lit(1), lit(3))
	case "select * from SupportBean(intBoxed in (3, 3))":
		filter = esper.InOf(ib(), lit(3), lit(3))
	case "select * from SupportBean(boolPrimitive=false, intBoxed in (1, 2, 3))":
		filter = esper.And(
			esper.EqualOf(bp(), esper.Literal(false)),
			esper.InOf(ib(), lit(1), lit(2), lit(3)))
	case "select * from SupportBean(boolPrimitive=false, intBoxed in (3, 4))":
		filter = esper.And(
			esper.EqualOf(bp(), esper.Literal(false)),
			esper.InOf(ib(), lit(3), lit(4)))
	case "select * from SupportBean(boolPrimitive=false, intBoxed in (3))":
		filter = esper.And(
			esper.EqualOf(bp(), esper.Literal(false)),
			esper.InOf(ib(), lit(3)))
	case "select * from SupportBean(intBoxed in (1, 2, 3), longPrimitive >= 0)":
		filter = esper.And(
			esper.InOf(ib(), lit(1), lit(2), lit(3)),
			esper.GreaterOrEqualOf(lp(), lit(0)))
	case "select * from SupportBean(intBoxed in (3, 4), intPrimitive >= 0)":
		filter = esper.And(
			esper.InOf(ib(), lit(3), lit(4)),
			esper.GreaterOrEqualOf(ip(), lit(0)))
	case "select * from SupportBean(intBoxed in (3), bytePrimitive < 1)":
		filter = esper.And(
			esper.InOf(ib(), lit(3)),
			esper.LessOf(byp(), esper.Literal(int8(1))))
	case "select * from SupportBean(intBoxed not in [1:2])":
		filter = esper.NotBetweenRangeOf(ib(), lit(1), lit(2), true, true)
	case "select * from SupportBean(intBoxed not in (2, 1))":
		filter = esper.NotInOf(ib(), lit(2), lit(1))
	case "select * from SupportBean(intBoxed not between 0 and -3)":
		// Java normalizes reversed between bounds to [-3, 0]; the Go
		// between expression does the same normalization.
		filter = esper.NotBetweenOf(ib(), lit(0), lit(-3))
	case "select * from SupportBean(intBoxed not in (1, 4, 5))":
		filter = esper.NotInOf(ib(), lit(1), lit(4), lit(5))
	case "select * from SupportBean(intBoxed not in (4, 5, 1))":
		filter = esper.NotInOf(ib(), lit(4), lit(5), lit(1))
	case "select * from SupportBean(intBoxed not in (3:4))":
		filter = esper.NotBetweenRangeOf(ib(), lit(3), lit(4), false, false)
	case "select * from SupportBean(intBoxed not in [1:3))":
		filter = esper.NotBetweenRangeOf(ib(), lit(1), lit(3), true, false)
	case "select * from SupportBean(intBoxed not in (1,1,1,33))":
		filter = esper.NotInOf(ib(), lit(1), lit(1), lit(1), lit(33))
	case "select * from SupportBean(intPrimitive in (0,0,1) and theString like 'X%')":
		filter = esper.And(
			esper.InOf(ip(), lit(0), lit(0), lit(1)),
			esper.LikeOf(ts(), esper.Literal("X%")))
	case "select * from SupportBean(intPrimitive in (0,1) and theString like 'A%')":
		filter = esper.And(
			esper.InOf(ip(), lit(0), lit(1)),
			esper.LikeOf(ts(), esper.Literal("A%")))
	case "select * from SupportBean(intPrimitive in (0) and theString like 'X%')":
		filter = esper.And(
			esper.InOf(ip(), lit(0)),
			esper.LikeOf(ts(), esper.Literal("X%")))
	case "select * from pattern [a=SupportBeanNumeric -> every b=SupportBean(intPrimitive in (a.intOne, a.intTwo))]":
		pattern := esper.PatternFrom(
			esper.From[efabNumericBean](env, "SupportBeanNumeric"), "a", esper.Literal(true)).
			Then(esper.PatternFrom(bean(), "b", esper.InOf(
				ip(),
				esper.TagField[*int64]("a", "intOne"),
				esper.TagField[*int64]("a", "intTwo"))).Every())
		return env.Build(pattern.Select(
			esper.Alias("a", efoTagMap(esper.PatternEvent("a"))),
			esper.Alias("b", efoTagMap(esper.PatternEvent("b"))),
		).Query(esper.StatementName(statement)))
	case "select * from pattern [a=SupportBean_S0 -> every b=SupportBean(theString in (a.p00, a.p01, a.p02))]":
		pattern := esper.PatternFrom(
			esper.From[efovS0](env, "SupportBean_S0"), "a", esper.Literal(true)).
			Then(esper.PatternFrom(bean(), "b", esper.InOf(
				ts(),
				esper.TagField[string]("a", "p00"),
				esper.TagField[string]("a", "p01"),
				esper.TagField[string]("a", "p02"))).Every())
		return env.Build(pattern.Select(
			esper.Alias("a", efoTagMap(esper.PatternEvent("a"))),
			esper.Alias("b", efoTagMap(esper.PatternEvent("b"))),
		).Query(esper.StatementName(statement)))
	default:
		return esper.Plan{}, fmt.Errorf("%s statement %q has no plan for EPL %q", efabID, statement, epl)
	}
	return env.Build(esper.Select(bean().Filter(filter)).Query(esper.StatementName(statement)))
}

func efabSend(ctx context.Context, engine *esper.Engine, eventType string, fields map[string]json.RawMessage) error {
	switch eventType {
	case "SupportBean":
		// Java's `new SupportBean()` leaves charPrimitive at '\u0000' and
		// every boxed/string column null; only the payload keys are set.
		bean := efabBean{CharPrimitive: "\u0000"}
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
			case "intPrimitive":
				bean.IntPrimitive = int32(int64Field(value))
			case "intBoxed":
				if value != nil {
					v := int32(int64Field(value))
					bean.IntBoxed = &v
				}
			default:
				return fmt.Errorf("unknown %s field %q", eventType, key)
			}
		}
		return engine.Send(ctx, eventType, bean)
	case "SupportBeanNumeric":
		var bean efabNumericBean
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "intOne":
				if value != nil {
					v := int64Field(value)
					bean.IntOne = &v
				}
			case "intTwo":
				if value != nil {
					v := int64Field(value)
					bean.IntTwo = &v
				}
			default:
				return fmt.Errorf("unknown %s field %q", eventType, key)
			}
		}
		return engine.Send(ctx, eventType, bean)
	case "SupportBean_S0":
		var bean efovS0
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", eventType, key, err)
			}
			switch key {
			case "id":
				bean.ID = int(int64Field(value))
			case "p00":
				bean.P00 = stringField(value)
			case "p01":
				bean.P01 = stringField(value)
			case "p02":
				bean.P02 = stringField(value)
			case "p03":
				bean.P03 = stringField(value)
			default:
				return fmt.Errorf("unknown %s field %q", eventType, key)
			}
		}
		return engine.Send(ctx, eventType, bean)
	}
	return fmt.Errorf("unknown event type %q", eventType)
}

func decodeEfabPayload(step compat.Step) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	return fields, nil
}

func runEfabScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	return executeEfab(ctx, scenario.Steps)
}

func loadEfabScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", efabID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", efabID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", efabID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", efabID, err)
	}
	if err := requireEfabFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", efabID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != efabID ||
		metadata.Description != efabDescription ||
		metadata.JavaCommit != efabJavaCommit ||
		metadata.JavaSource != efabSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", efabID)
	}
	if err := validateEfabStringArray(root["javaRuntimes"], efabJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEfabStringArray(root["javaNames"], efabJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEfabStringArray(root["javaStaticIds"], efabJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEfabStringArray(root["javaFlags"], efabJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(efabCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", efabID, len(efabCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEfabFields(object,
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
		if definition.Case != efabCases[index] ||
			definition.Ordinal != efabOrdinals[index] ||
			definition.RuntimeID != efabJavaRuntimeIDs[index] ||
			definition.ExecutionName != efabJavaExecutions[index] ||
			definition.Observation != efabCaseObservations[index] ||
			definition.EPL != efabCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", efabID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", efabID, err)
	}
	offset := 0
	for _, caseName := range efabCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", efabID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", efabID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", efabID, offset, caseName)
		}
		offset++
		want, ok := efabCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", efabID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", efabID, caseName)
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
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", efabID, offset, err)
			}
			if step.Case != caseName {
				return compat.Scenario{}, fmt.Errorf("%s step %d case = %q, want %q", efabID, offset, step.Case, caseName)
			}
			key := step.Op + ":" + step.Statement + step.EventType
			if step.Op == "send" {
				var compacted bytes.Buffer
				if err := json.Compact(&compacted, step.Payload); err != nil {
					return compat.Scenario{}, fmt.Errorf("%s step %d payload: %w", efabID, offset, err)
				}
				key += ":" + compacted.String()
			}
			if step.Op == "deploy" {
				key += ":" + step.Epl
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", efabID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", efabID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", efabID, err)
	}
	return scenario, nil
}

func requireEfabFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s fields = %d, want %d", efabID, len(object), len(names))
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s missing field %q", efabID, name)
		}
	}
	return nil
}

func validateEfabStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("%s %s: %w", efabID, name, err)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s %s has %d entries, want %d", efabID, name, len(values), len(expected))
	}
	for index, value := range values {
		if value != expected[index] {
			return fmt.Errorf("%s %s[%d] = %q, want %q", efabID, name, index, value, expected[index])
		}
	}
	return nil
}
