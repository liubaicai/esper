package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	esper "github.com/liubaicai/esper/internal/esper"
)

// infra_table_count_min_sketch.go replays InfraTableCountMinSketch (ords 0-3)
// against the pinned Java oracle: count-min-sketch table columns with topk
// capacity, a custom agent, and the invalid-probe surface.
//
// frequency-and-topk (ord 0) deploys the module
// `@public create table MyApproxFT(wordapprox countMinSketch({topk:3}))` plus
// `into table MyApproxFT select countMinSketchAdd(theString) as wordapprox
// from SupportBean` plus the `frequency`/`topk` read statements, then replays
// assertOutput: one SupportBean_S0 per expected frequency pair and one
// SupportBean_S1 per topk assertion, so the listeners record freq/topk after
// every send. The join and subquery modules deploy, fire one SupportBean_S2
// each, and undeployModuleContaining.
//
// doc-samples (ord 1) is compile-only: seven compileDeploy calls register the
// WordEvent/EstimateWordCountEvent schemas, the plain and fully-parameterized
// countMinSketch tables, the into-table feed, the frequency read, and a
// pattern-driven topk read. No events are sent.
//
// non-string-type (ord 2) declares `bytefreq
// countMinSketch({  epsOfTotalCount: 0.02,  confidence: 0.98,  topk: null,
// agent:'...MyBytesPassthruAgentForge'})`, feeds byte[] bodies from
// SupportByteArrEventStringId(id='A'), and reads
// countMinSketchFrequency(body) from id='B' events.
//
// invalid (ord 3) deploys `@public create table MyCMS(wordcms
// countMinSketch())` then runs fifteen tryInvalidCompile probes across the
// declaration, into-table, and consumption surfaces before undeployAll.
//
// Approved differences (observably identical to the Java EPL):
// - `create schema`/`create table` map to env-level registrations
//   (RegisterStruct/CreateTable); the deploy step emits the "deployed"
//   marker without an engine deployment, mirroring the reset runner.
// - `select T.c.m(x) from S` maps to OnEvent(S).SelectFromTableWhere(T, true,
//   ...), the established table-read form.
// - byte[] keys feed the sketch as strings via the pinned `bytes` Func1
//   conversion; payloads carry the body as base64 (Go's canonical []byte
//   JSON encoding).
// - The agent parameter is carried as TableAggDecl.Agent metadata; Go does
//   not load agent classes, so the agent probes are tolerated attempts.

// itcmsBean mirrors SupportBean for the count-min-sketch executions.
type itcmsBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// itcmsS0 mirrors SupportBean_S0 (frequency read driver).
type itcmsS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// itcmsS1 mirrors SupportBean_S1 (topk read driver).
type itcmsS1 struct {
	ID int `esper:"id"`
}

// itcmsS2 mirrors SupportBean_S2 (join/subquery driver).
type itcmsS2 struct {
	ID  int    `esper:"id"`
	P20 string `esper:"p20"`
}

// itcmsByteArr mirrors SupportByteArrEventStringId; Body decodes from the
// payload's base64 string.
type itcmsByteArr struct {
	ID   string `esper:"id"`
	Body []byte `esper:"body"`
}

// itcmsWordEvent mirrors the doc-sample create-schema WordEvent.
type itcmsWordEvent struct {
	Word string `esper:"word"`
}

// itcmsEstimateWordCountEvent mirrors the doc-sample create-schema
// EstimateWordCountEvent.
type itcmsEstimateWordCountEvent struct {
	Word string `esper:"word"`
}

const (
	infraTableCMSID         = "infra-table-count-min-sketch"
	infraTableCMSJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableCMSJavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableCountMinSketch.java"

	infraTableCMSDescription = "InfraTableCountMinSketch ords 0-3: count-min-sketch table columns — a topk-3 column fed by countMinSketchAdd with frequency/topk listeners plus join and subquery reads, the compile-only doc samples including a fully-parameterized declaration, byte[] keys via a custom agent, and fifteen invalid compile probes."

	itcmsAgentUTF16  = "com.espertech.esper.common.client.util.CountMinSketchAgentStringUTF16Forge"
	itcmsAgentBytes  = "com.espertech.esper.regressionlib.suite.infra.tbl.InfraTableCountMinSketch$MyBytesPassthruAgentForge"
	itcmsAgentString = "java.lang.String"

	itcmsEplModule = "@public create table MyApproxFT(wordapprox countMinSketch({topk:3}));\n" +
		"into table MyApproxFT select countMinSketchAdd(theString) as wordapprox from SupportBean;\n" +
		"@name('frequency') select MyApproxFT.wordapprox.countMinSketchFrequency(p00) as freq from SupportBean_S0;\n" +
		"@name('topk') select MyApproxFT.wordapprox.countMinSketchTopk() as topk from SupportBean_S1;\n"
	itcmsEplJoin = "@name('join') select wordapprox.countMinSketchFrequency(s2.p20) as c0 from MyApproxFT, SupportBean_S2 s2 unidirectional"
	itcmsEplSubq = "@name('subq') select (select wordapprox.countMinSketchFrequency(s2.p20) from MyApproxFT) as c0 from SupportBean_S2 s2"

	itcmsEplSchemaWord    = "@public create schema WordEvent (word string)"
	itcmsEplSchemaEstim   = "@public create schema EstimateWordCountEvent (word string)"
	itcmsEplTableWord     = "@public create table WordCountTable(wordcms countMinSketch())"
	itcmsEplTableWordTwo  = "@public create table WordCountTable2(wordcms countMinSketch({\n  epsOfTotalCount: 0.000002,\n  confidence: 0.999,\n  seed: 38576,\n  topk: 20,\n  agent: '" + itcmsAgentUTF16 + "'}))"
	itcmsEplIntoWord      = "into table WordCountTable select countMinSketchAdd(word) as wordcms from WordEvent"
	itcmsEplSelectFreq    = "select WordCountTable.wordcms.countMinSketchFrequency(word) from EstimateWordCountEvent"
	itcmsEplSelectTopk    = "select WordCountTable.wordcms.countMinSketchTopk() from pattern[every timer:interval(10 sec)]"
	itcmsEplTableNS       = "@public create table MyApproxNS(bytefreq countMinSketch({  epsOfTotalCount: 0.02,  confidence: 0.98,  topk: null,  agent: '" + itcmsAgentBytes + "'}))"
	itcmsEplIntoNS        = "into table MyApproxNS select countMinSketchAdd(body) as bytefreq from SupportByteArrEventStringId(id='A')"
	itcmsEplReadNS        = "@name('s0') select MyApproxNS.bytefreq.countMinSketchFrequency(body) as freq from SupportByteArrEventStringId(id='B')"
	itcmsEplInvalidDeploy = "@public create table MyCMS(wordcms countMinSketch())"
)

var infraTableCMSJavaSources = []string{infraTableCMSJavaSource}

// Case order fixes the runtime-ID index mapping below.
var infraTableCMSCases = []string{
	"frequency-and-topk",
	"doc-samples",
	"non-string-type",
	"invalid",
}

var infraTableCMSJavaRuntimeIDs = []string{
	"java-runtime-f09401ff1e3349b3497b",
	"java-runtime-87f8d0057f91e1dfa7ec",
	"java-runtime-4b4c531bbad3d4e1491d",
	"java-runtime-217779fa3c860cc1ea18",
}

var infraTableCMSJavaExecutions = []string{
	"InfraFrequencyAndTopk",
	"InfraDocSamples",
	"InfraNonStringType",
	"InfraInvalid",
}

var infraTableCMSJavaStaticIDs = []string{
	"java-834a25e0038e5f5a8950",
	"java-d926cce1a816fb780ed8",
	"java-304d2b5b2913d2f02828",
	"java-91bcaf7a054d58304543",
}

var infraTableCMSJavaFlags = []string{}

var infraTableCMSCaseObservations = []string{
	"topk-3 count-min-sketch column fed by countMinSketchAdd(theString); frequency/topk listeners record after every send, the join and subquery modules each fire once, and undeployAll closes the case",
	"compile-only doc samples: two create-schema registrations, a default and a fully-parameterized countMinSketch table, the into-table feed, the frequency read, and a pattern-driven topk read; no events are sent",
	"byte[] keys via MyBytesPassthruAgentForge: the feed filters id='A', the read filters id='B', and countMinSketchFrequency(body) reports 0 then 1",
	"deploys the MyCMS fixture then runs fifteen tryInvalidCompile probes across declaration, into-table, and consumption surfaces before undeployAll",
}

var infraTableCMSCaseEPLs = []string{
	itcmsEplModule + itcmsEplJoin + "\n" + itcmsEplSubq,
	itcmsEplSchemaWord + "\n" + itcmsEplSchemaEstim + "\n" + itcmsEplTableWord + "\n" + itcmsEplTableWordTwo + "\n" + itcmsEplIntoWord + "\n" + itcmsEplSelectFreq + "\n" + itcmsEplSelectTopk,
	itcmsEplTableNS + "\n" + itcmsEplIntoNS + "\n" + itcmsEplReadNS,
	itcmsEplInvalidDeploy,
}

// itcmsDeployEPLs pins the EPL behind every deploy step by case and
// statement label, mirroring the Java compileDeploy calls.
var itcmsDeployEPLs = map[string]map[string]string{
	"frequency-and-topk": {
		"module": itcmsEplModule,
		"join":   itcmsEplJoin,
		"subq":   itcmsEplSubq,
	},
	"doc-samples": {
		"schema-word-event":     itcmsEplSchemaWord,
		"schema-estimate-event": itcmsEplSchemaEstim,
		"table-word-count":      itcmsEplTableWord,
		"table-word-count2":     itcmsEplTableWordTwo,
		"into-word-count":       itcmsEplIntoWord,
		"select-frequency":      itcmsEplSelectFreq,
		"select-topk":           itcmsEplSelectTopk,
	},
	"non-string-type": {
		"table": itcmsEplTableNS,
		"into":  itcmsEplIntoNS,
		"s0":    itcmsEplReadNS,
	},
	"invalid": {
		"table": itcmsEplInvalidDeploy,
	},
}

// infraTableCMSCaseSteps pins the per-case step sequence as
// `op|statement|eventType` keys, mirroring the Java oracle's step gate.
var infraTableCMSCaseSteps = map[string][]string{
	"frequency-and-topk": {
		"deploy|module|",
		"deployed|module|",
		"send||SupportBean|{\"theString\":\"E1\",\"intPrimitive\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"send||SupportBean|{\"theString\":\"E2\",\"intPrimitive\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E2\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"send||SupportBean|{\"theString\":\"E2\",\"intPrimitive\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E2\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"send||SupportBean|{\"theString\":\"E3\",\"intPrimitive\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E2\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E3\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"send||SupportBean|{\"theString\":\"E4\",\"intPrimitive\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E2\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E3\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E4\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"send||SupportBean|{\"theString\":\"E4\",\"intPrimitive\":0}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E1\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E2\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E3\"}",
		"send||SupportBean_S0|{\"id\":0,\"p00\":\"E4\"}",
		"send||SupportBean_S1|{\"id\":0}",
		"deploy|join|",
		"deployed|join|",
		"send||SupportBean_S2|{\"id\":0,\"p20\":\"E3\"}",
		"undeploy|join|",
		"deploy|subq|",
		"deployed|subq|",
		"send||SupportBean_S2|{\"id\":0,\"p20\":\"E3\"}",
		"undeploy|subq|",
		"undeploy-all||",
	},
	"doc-samples": {
		"deploy|schema-word-event|",
		"deployed|schema-word-event|",
		"deploy|schema-estimate-event|",
		"deployed|schema-estimate-event|",
		"deploy|table-word-count|",
		"deployed|table-word-count|",
		"deploy|table-word-count2|",
		"deployed|table-word-count2|",
		"deploy|into-word-count|",
		"deployed|into-word-count|",
		"deploy|select-frequency|",
		"deployed|select-frequency|",
		"deploy|select-topk|",
		"deployed|select-topk|",
		"undeploy-all||",
	},
	"non-string-type": {
		"deploy|table|",
		"deployed|table|",
		"deploy|into|",
		"deployed|into|",
		"deploy|s0|",
		"deployed|s0|",
		"send||SupportByteArrEventStringId|{\"id\":\"A\",\"body\":\"AQID\"}",
		"send||SupportByteArrEventStringId|{\"id\":\"B\",\"body\":\"AAID\"}",
		"send||SupportByteArrEventStringId|{\"id\":\"B\",\"body\":\"AQID\"}",
		"undeploy-all||",
	},
	"invalid": {
		"deploy|table|",
		"deployed|table|",
		"build-error|cms-in-select|",
		"build-error|cms-invalid-param|",
		"build-error|cms-invalid-json-param|",
		"build-error|cms-invalid-eps|",
		"build-error|cms-agent-not-found|",
		"build-error|cms-agent-wrong-type|",
		"build-error|add-in-select|",
		"build-error|add-no-param|",
		"build-error|add-byte-param|",
		"build-error|add-distinct|",
		"build-error|add-null|",
		"build-error|frequency-into-table|",
		"build-error|frequency-select|",
		"build-error|topk-select|",
		"build-error|topk-param|",
		"undeploy-all||",
	},
}

// itcmsProbe pins one tryInvalidCompile probe: the EPL text, the startsWith
// prefix Java asserts, and the Go fluent equivalent. A nil go_ means the
// probe is unrepresentable on the fluent surface and only the pinned prefix
// is recorded; a non-nil go_ is attempted and its outcome tolerated when the
// probe is marked tolerated.
type itcmsProbe struct {
	label     string
	epl       string
	expect    string
	go_       func(s *itcmsCaseState) error
	tolerated bool
	goCode    esper.ErrorCode
	goSub     string
}

// itcmsInvalidProbes transcribes InfraInvalid's fifteen tryInvalidCompile
// probes verbatim, including the oracle's typos ('expects an Double', the
// double space after 'countMinSketch”, 'requires a no parameter
// expressions', the trailing space in 'could not be resolved ', and the
// 43-char expression truncation).
var itcmsInvalidProbes = []itcmsProbe{
	{
		label:  "cms-in-select",
		epl:    "select countMinSketch() from SupportBean",
		expect: "Failed to validate select-clause expression 'countMinSketch()': Count-min-sketch aggregation function 'countMinSketch' can only be used in create-table statements [",
	},
	{
		label:  "cms-invalid-param",
		epl:    "create table MyTable(cms countMinSketch(5))",
		expect: "Failed to validate table-column expression 'countMinSketch(5)': Count-min-sketch aggregation function 'countMinSketch'  expects either no parameter or a single json parameter object [",
	},
	{
		label:  "cms-invalid-json-param",
		epl:    "create table MyTable(cms countMinSketch({xxx:3}))",
		expect: "Failed to validate table-column expression 'countMinSketch({xxx=3})': Unrecognized parameter 'xxx' [",
	},
	{
		label:  "cms-invalid-eps",
		epl:    "create table MyTable(cms countMinSketch({epsOfTotalCount:'a'}))",
		expect: "Failed to validate table-column expression 'countMinSketch({epsOfTotalCount=a})': Property 'epsOfTotalCount' expects an Double but receives a value of type String [",
	},
	{
		label:  "cms-agent-not-found",
		epl:    "create table MyTable(cms countMinSketch({agent:'a'}))",
		expect: "Failed to validate table-column expression 'countMinSketch({agent=a})': Failed to instantiate agent provider: Could not load class by name 'a', please check imports [",
		// Go carries the agent as declared metadata and does not load
		// agent classes; the attempt is tolerated.
		go_: func(s *itcmsCaseState) error {
			_, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{itcmsCMSColumnAgent("cms", "a")})
			return err
		},
		tolerated: true,
	},
	{
		label:  "cms-agent-wrong-type",
		epl:    "create table MyTable(cms countMinSketch({agent:'java.lang.String'}))",
		expect: "Failed to validate table-column expression 'countMinSketch({agent=java.lang.String})': Failed to instantiate agent provider: Class 'java.lang.String' does not implement interface 'com.espertech.esper.common.client.util.CountMinSketchAgentForge' [",
		go_: func(s *itcmsCaseState) error {
			_, err := esper.CreateTable(s.env, "MyTable", []esper.TableColumn{itcmsCMSColumnAgent("cms", itcmsAgentString)})
			return err
		},
		tolerated: true,
	},
	{
		label:  "add-in-select",
		epl:    "select countMinSketchAdd(theString) from SupportBean",
		expect: "Failed to validate select-clause expression 'countMinSketchAdd(theString)': Count-min-sketch aggregation function 'countMinSketchAdd' can only be used with into-table",
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.From[itcmsBean](s.env, "SupportBean").Aggregate(
				esper.Alias("countMinSketchAdd(theString)", esper.CountMinSketchAdd[string](esper.Field[itcmsBean, string]("theString"))),
			).Query())
			return err
		},
		goSub: "can only be used with into-table",
	},
	{
		label:  "add-no-param",
		epl:    "into table MyCMS select countMinSketchAdd() as wordcms from SupportBean",
		expect: "Failed to validate select-clause expression 'countMinSketchAdd()': Count-min-sketch aggregation function 'countMinSketchAdd' requires a single parameter expression",
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.From[itcmsBean](s.env, "SupportBean").Aggregate(
				esper.Alias("wordcms", esper.CountMinSketchAdd[string](nil)),
			).IntoTable("MyCMS"))
			return err
		},
		goSub: "requires a single parameter expression",
	},
	{
		label:  "add-byte-param",
		epl:    "into table MyCMS select countMinSketchAdd(body) as wordcms from SupportByteArrEventStringId",
		expect: "Incompatible aggregation function for table 'MyCMS' column 'wordcms', expecting 'countMinSketch()' and received 'countMinSketchAdd(body)': Mismatching parameter return type, expected any of [class java.lang.String] but received byte[] [",
		// Unrepresentable: the Go column is typed CountMinSketchValue[string]
		// and a []byte feed does not type-check.
	},
	{
		label:  "add-distinct",
		epl:    "into table MyCMS select countMinSketchAdd(distinct 'abc') as wordcms from SupportByteArrEventStringId",
		expect: "Failed to validate select-clause expression 'countMinSketchAdd(distinct \"abc\")': Count-min-sketch aggregation function 'countMinSketchAdd' is not supported with distinct [",
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.From[itcmsByteArr](s.env, "SupportByteArrEventStringId").Aggregate(
				esper.Alias("wordcms", esper.DistinctAggregate[esper.CountMinSketchValue[string]](esper.CountMinSketchAdd[string](esper.Literal("abc")), nil)),
			).IntoTable("MyCMS"))
			return err
		},
		goSub: "not supported with distinct",
	},
	{
		label:  "add-null",
		epl:    "into table MyCMS select countMinSketchAdd(null) as wordcms from SupportByteArrEventStringId",
		expect: "Failed to validate select-clause expression 'countMinSketchAdd(null)': Invalid null-type parameter",
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.From[itcmsByteArr](s.env, "SupportByteArrEventStringId").Aggregate(
				esper.Alias("wordcms", esper.CountMinSketchAdd[string](esper.NullLiteral[string]())),
			).IntoTable("MyCMS"))
			return err
		},
		goSub: "null-type parameter",
	},
	{
		label:  "frequency-into-table",
		epl:    "into table MyCMS select countMinSketchFrequency(theString) as wordcms from SupportBean",
		expect: "Failed to validate select-clause expression 'countMinSketchFrequency(theString)': Unknown single-row function, aggregation function or mapped or indexed property named 'countMinSketchFrequency' could not be resolved ",
		// The Go surface exists but EPL name resolution is what fails; the
		// fluent equivalent compiles, so the attempt is tolerated.
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.From[itcmsBean](s.env, "SupportBean").Aggregate(
				esper.Alias("wordcms", esper.CountMinSketchFrequency[string](nil, esper.Field[itcmsBean, string]("theString"))),
			).IntoTable("MyCMS"))
			return err
		},
		tolerated: true,
	},
	{
		label:  "frequency-select",
		epl:    "select countMinSketchFrequency() from SupportBean",
		expect: "Failed to validate select-clause expression 'countMinSketchFrequency()': Unknown single-row function, expression declaration, script or aggregation function named 'countMinSketchFrequency' could not be resolved",
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.Select(esper.From[itcmsBean](s.env, "SupportBean"),
				esper.Alias("countMinSketchFrequency()", esper.CountMinSketchFrequency[string](nil, nil)),
			).Query())
			return err
		},
		tolerated: true,
	},
	{
		label:  "topk-select",
		epl:    "select countMinSketchTopk() from SupportBean",
		expect: "Failed to validate select-clause expression 'countMinSketchTopk()': Unknown single-row function, expression declaration, script or aggregation function named 'countMinSketchTopk' could not be resolved",
		go_: func(s *itcmsCaseState) error {
			_, err := s.env.Build(esper.Select(esper.From[itcmsBean](s.env, "SupportBean"),
				esper.Alias("countMinSketchTopk()", esper.CountMinSketchTopK[string](nil)),
			).Query())
			return err
		},
		tolerated: true,
	},
	{
		label:  "topk-param",
		epl:    "select MyCMS.wordcms.countMinSketchTopk(theString) from SupportBean",
		expect: "Failed to validate select-clause expression 'MyCMS.wordcms.countMinSketchTopk(th...(43 chars)': Count-min-sketch aggregation function 'countMinSketchTopk' requires a no parameter expressions [",
		// Unrepresentable: the Go countMinSketchTopk equivalent takes no key
		// parameter.
	},
}

// itcmsCaseState carries the per-case replay state: the environment/engine
// pair, the deployed-label bookkeeping used by undeploy and undeploy-all, and
// the trace the runner records into.
type itcmsCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	trace          *compat.Trace
	caseName       string
	sequences      map[string]uint64
}

// runInfraTableCMSScenario replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func runInfraTableCMSScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraTableCMSScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableCMSCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		if err := runInfraTableCMSCase(ctx, caseIndex, caseName, caseSteps, &trace); err != nil {
			return compat.Trace{}, err
		}
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s: %d trailing steps after the last case", infraTableCMSID, len(scenario.Steps)-offset)
	}
	return trace, nil
}

func runInfraTableCMSCase(ctx context.Context, caseIndex int, caseName string, steps []compat.Step, trace *compat.Trace) error {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[itcmsBean](env, "SupportBean"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[itcmsS0](env, "SupportBean_S0"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[itcmsS1](env, "SupportBean_S1"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[itcmsS2](env, "SupportBean_S2"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[itcmsByteArr](env, "SupportByteArrEventStringId"); err != nil {
		return err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableCMSJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer engine.Close(ctx)

	state := &itcmsCaseState{
		env:            env,
		engine:         engine,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		trace:          trace,
		caseName:       caseName,
		sequences:      make(map[string]uint64),
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if step.Op != "case" && step.Case != caseName {
			return fmt.Errorf("%s: step %q carries case %q, want %q", infraTableCMSID, step.Op, step.Case, caseName)
		}
		switch step.Op {
		case "case":
			// case metadata is validated up front.
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return err
			}
		case "deployed":
			if !state.deployedLabels[step.Statement] {
				return fmt.Errorf("%s: deployed marker %q has no matching deploy step", infraTableCMSID, step.Statement)
			}
			state.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      state.caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequences[step.Statement+":deployed"],
				Time:      "0",
			})
		case "send":
			if err := state.send(ctx, step); err != nil {
				return err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return err
			}
		case "undeploy":
			if err := state.undeploy(ctx, step.Statement); err != nil {
				return err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: unsupported op %q", infraTableCMSID, step.Op)
		}
	}
	return nil
}

// deploy pins the step EPL and dispatches to the case-specific deployer.
func (s *itcmsCaseState) deploy(ctx context.Context, step compat.Step) error {
	pinned, ok := itcmsDeployEPLs[s.caseName][step.Statement]
	if !ok {
		return fmt.Errorf("%s: case %q has no pinned deploy %q", infraTableCMSID, s.caseName, step.Statement)
	}
	if step.Epl != pinned {
		return fmt.Errorf("%s: deploy %q EPL = %q, want %q", infraTableCMSID, step.Statement, step.Epl, pinned)
	}
	var err error
	switch s.caseName {
	case "frequency-and-topk":
		err = s.deployFrequencyAndTopk(ctx, step.Statement)
	case "doc-samples":
		err = s.deployDocSamples(ctx, step.Statement)
	case "non-string-type":
		err = s.deployNonStringType(ctx, step.Statement)
	case "invalid":
		err = s.deployInvalidFixture(step.Statement)
	default:
		err = fmt.Errorf("%s: unsupported deploy case %q", infraTableCMSID, s.caseName)
	}
	if err != nil {
		return err
	}
	s.deployedLabels[step.Statement] = true
	return nil
}

// deployPlan deploys one or more plans under a single label and subscribes
// the listener-recording statements.
func (s *itcmsCaseState) deployPlan(ctx context.Context, label string, plans ...esper.Plan) error {
	for _, plan := range plans {
		deployment, err := s.engine.Deploy(ctx, plan)
		if err != nil {
			return err
		}
		s.deployments[label] = append(s.deployments[label], deployment)
		for _, statement := range deployment.Statements() {
			if !itcmsListenedStatement(s.caseName, statement.Name()) {
				continue
			}
			stmt := statement
			if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
				if len(batch.New) == 0 && len(batch.Old) == 0 {
					return nil
				}
				s.sequences[stmt.Name()+":listener"]++
				record := compat.TraceRecord{
					Case:      s.caseName,
					Operation: "listener",
					Statement: stmt.Name(),
					Sequence:  s.sequences[stmt.Name()+":listener"],
					Time:      "0",
				}
				if rows := itcmsNormalizeResults(batch.New); len(rows) > 0 {
					record.New = rows
				}
				if rows := itcmsNormalizeResults(batch.Old); len(rows) > 0 {
					record.Old = rows
				}
				s.trace.Records = append(s.trace.Records, record)
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// itcmsListenedStatement reports whether the Java oracle attaches a listener
// to the named statement for this case.
func itcmsListenedStatement(caseName, name string) bool {
	switch caseName {
	case "frequency-and-topk":
		return name == "frequency" || name == "topk" || name == "join" || name == "subq"
	case "non-string-type":
		return name == "s0"
	}
	return false
}

// deployFrequencyAndTopk replays the ord-0 module deploy plus the join and
// subquery module deploys.
func (s *itcmsCaseState) deployFrequencyAndTopk(ctx context.Context, label string) error {
	switch label {
	case "module":
		// `@public create table MyApproxFT(wordapprox countMinSketch({topk:3}))`
		if _, err := esper.CreateTable(s.env, "MyApproxFT", []esper.TableColumn{itcmsCMSColumnTopK("wordapprox", 3)}); err != nil {
			return err
		}
		// `into table MyApproxFT select countMinSketchAdd(theString) as
		// wordapprox from SupportBean`
		intoPlan, err := s.env.Build(esper.From[itcmsBean](s.env, "SupportBean").Aggregate(
			esper.Alias("wordapprox", esper.CountMinSketchAdd[string](esper.Field[itcmsBean, string]("theString"))),
		).IntoTable("MyApproxFT"))
		if err != nil {
			return err
		}
		// `@name('frequency') select MyApproxFT.wordapprox
		// .countMinSketchFrequency(p00) as freq from SupportBean_S0`
		frequencyPlan, err := s.env.Build(esper.OnEvent(esper.From[itcmsS0](s.env, "SupportBean_S0")).
			SelectFromTableWhere("MyApproxFT", esper.Literal(true),
				esper.Alias("freq", esper.CountMinSketchFrequency[string](
					esper.TableField[esper.CountMinSketchValue[string]]("wordapprox"),
					esper.Field[itcmsS0, string]("p00")))).
			Query(esper.StatementName("frequency")))
		if err != nil {
			return err
		}
		// `@name('topk') select MyApproxFT.wordapprox.countMinSketchTopk() as
		// topk from SupportBean_S1`
		topkPlan, err := s.env.Build(esper.OnEvent(esper.From[itcmsS1](s.env, "SupportBean_S1")).
			SelectFromTableWhere("MyApproxFT", esper.Literal(true),
				esper.Alias("topk", esper.CountMinSketchTopK[string](
					esper.TableField[esper.CountMinSketchValue[string]]("wordapprox")))).
			Query(esper.StatementName("topk")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, "module", intoPlan, frequencyPlan, topkPlan)
	case "join":
		// `@name('join') select wordapprox.countMinSketchFrequency(s2.p20) as
		// c0 from MyApproxFT, SupportBean_S2 s2 unidirectional`
		plan, err := s.env.Build(esper.JoinMany(
			esper.JoinRecordSource(esper.FromTable(s.env, "MyApproxFT")),
			esper.JoinSource(esper.From[itcmsS2](s.env, "SupportBean_S2")).Unidirectional(),
		).Select(
			esper.SelectFrom(0, "c0", esper.CountMinSketchFrequency[string](
				esper.JoinField[esper.CountMinSketchValue[string]](0, "wordapprox"),
				esper.JoinField[string](1, "p20"))),
		).Query(esper.StatementName("join")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, "join", plan)
	case "subq":
		// `@name('subq') select (select wordapprox
		// .countMinSketchFrequency(s2.p20) from MyApproxFT) as c0 from
		// SupportBean_S2 s2`
		plan, err := s.env.Build(esper.Select(esper.From[itcmsS2](s.env, "SupportBean_S2"),
			esper.Alias("c0", esper.SubqueryValueWithOptions[int64](
				esper.FromTable(s.env, "MyApproxFT"),
				esper.CountMinSketchFrequency[string](
					esper.Field[any, esper.CountMinSketchValue[string]]("wordapprox"),
					esper.OuterField[string]("p20")))),
		).Query(esper.StatementName("subq")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, "subq", plan)
	}
	return fmt.Errorf("%s: unsupported frequency-and-topk deploy %q", infraTableCMSID, label)
}

// deployDocSamples replays the seven compileDeploy calls of ord 1. The
// create-schema/create-table calls map to env-level registrations; the
// into-table, frequency-read, and pattern-topk statements deploy as plans.
func (s *itcmsCaseState) deployDocSamples(ctx context.Context, label string) error {
	switch label {
	case "schema-word-event":
		_, err := esper.RegisterStruct[itcmsWordEvent](s.env, "WordEvent")
		return err
	case "schema-estimate-event":
		_, err := esper.RegisterStruct[itcmsEstimateWordCountEvent](s.env, "EstimateWordCountEvent")
		return err
	case "table-word-count":
		_, err := esper.CreateTable(s.env, "WordCountTable", []esper.TableColumn{itcmsCMSColumn("wordcms")})
		return err
	case "table-word-count2":
		_, err := esper.CreateTable(s.env, "WordCountTable2", []esper.TableColumn{itcmsCMSColumnAgent("wordcms", itcmsAgentUTF16)})
		return err
	case "into-word-count":
		plan, err := s.env.Build(esper.From[itcmsWordEvent](s.env, "WordEvent").Aggregate(
			esper.Alias("wordcms", esper.CountMinSketchAdd[string](esper.Field[itcmsWordEvent, string]("word"))),
		).IntoTable("WordCountTable"))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "select-frequency":
		plan, err := s.env.Build(esper.OnEvent(esper.From[itcmsEstimateWordCountEvent](s.env, "EstimateWordCountEvent")).
			SelectFromTableWhere("WordCountTable", esper.Literal(true),
				esper.Alias("countMinSketchFrequency(word)", esper.CountMinSketchFrequency[string](
					esper.TableField[esper.CountMinSketchValue[string]]("wordcms"),
					esper.Field[itcmsEstimateWordCountEvent, string]("word")))).
			Query())
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "select-topk":
		plan, err := s.env.Build(esper.OnPattern(esper.TimerInterval(esper.From[itcmsWordEvent](s.env, "WordEvent"), 10*time.Second).Every()).
			SelectFromTableWhere("WordCountTable", esper.Literal(true),
				esper.Alias("countMinSketchTopk()", esper.CountMinSketchTopK[string](
					esper.TableField[esper.CountMinSketchValue[string]]("wordcms")))).
			Query())
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	}
	return fmt.Errorf("%s: unsupported doc-samples deploy %q", infraTableCMSID, label)
}

// deployNonStringType replays the ord-2 table, into-table, and read deploys.
func (s *itcmsCaseState) deployNonStringType(ctx context.Context, label string) error {
	switch label {
	case "table":
		_, err := esper.CreateTable(s.env, "MyApproxNS", []esper.TableColumn{itcmsCMSColumnAgent("bytefreq", itcmsAgentBytes)})
		return err
	case "into":
		// `into table MyApproxNS select countMinSketchAdd(body) as bytefreq
		// from SupportByteArrEventStringId(id='A')`; the byte[] body feeds
		// the sketch through the pinned `bytes` conversion.
		plan, err := s.env.Build(esper.From[itcmsByteArr](s.env, "SupportByteArrEventStringId").
			Filter(esper.Equal[string](esper.Field[itcmsByteArr, string]("id"), esper.Literal("A"))).
			Aggregate(
				esper.Alias("bytefreq", esper.CountMinSketchAdd[string](
					esper.Func1("bytes", func(b []byte) string { return string(b) }, esper.Field[itcmsByteArr, []byte]("body")))),
			).IntoTable("MyApproxNS"))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	case "s0":
		// `@name('s0') select MyApproxNS.bytefreq.countMinSketchFrequency(body)
		// as freq from SupportByteArrEventStringId(id='B')`
		plan, err := s.env.Build(esper.OnEvent(esper.From[itcmsByteArr](s.env, "SupportByteArrEventStringId").
			Filter(esper.Equal[string](esper.Field[itcmsByteArr, string]("id"), esper.Literal("B")))).
			SelectFromTableWhere("MyApproxNS", esper.Literal(true),
				esper.Alias("freq", esper.CountMinSketchFrequency[string](
					esper.TableField[esper.CountMinSketchValue[string]]("bytefreq"),
					esper.Func1("bytes", func(b []byte) string { return string(b) }, esper.Field[itcmsByteArr, []byte]("body"))))).
			Query(esper.StatementName("s0")))
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan)
	}
	return fmt.Errorf("%s: unsupported non-string-type deploy %q", infraTableCMSID, label)
}

// deployInvalidFixture replays the ord-3 `@public create table MyCMS(wordcms
// countMinSketch())` deploy as an env-level registration.
func (s *itcmsCaseState) deployInvalidFixture(label string) error {
	if label != "table" {
		return fmt.Errorf("%s: unsupported invalid deploy %q", infraTableCMSID, label)
	}
	_, err := esper.CreateTable(s.env, "MyCMS", []esper.TableColumn{itcmsCMSColumn("wordcms")})
	return err
}

// buildError replays one ord-3 tryInvalidCompile probe: the Go fluent
// equivalent is attempted when representable, and the pinned Java prefix is
// recorded either way.
func (s *itcmsCaseState) buildError(step compat.Step) error {
	probe, ok := itcmsInvalidProbeByLabel(step.Statement)
	if !ok {
		return fmt.Errorf("%s: unknown probe %q", infraTableCMSID, step.Statement)
	}
	if step.Epl != probe.epl || step.ExpectError != probe.expect {
		return fmt.Errorf("%s: build-error probe %q is not pinned", infraTableCMSID, step.Statement)
	}
	if probe.go_ != nil {
		buildErr := probe.go_(s)
		if !probe.tolerated {
			if buildErr == nil {
				return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", infraTableCMSID, step.Statement)
			}
			code := probe.goCode
			if code == "" {
				code = esper.ErrorInvalidRule
			}
			if !errors.Is(buildErr, code) ||
				(probe.goSub != "" && !strings.Contains(buildErr.Error(), probe.goSub)) {
				return fmt.Errorf("%s: build-error probe %q drift: got %v", infraTableCMSID, step.Statement, buildErr)
			}
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func itcmsInvalidProbeByLabel(label string) (itcmsProbe, bool) {
	for _, probe := range itcmsInvalidProbes {
		if probe.label == label {
			return probe, true
		}
	}
	return itcmsProbe{}, false
}

// send decodes the payload and delivers the event to the engine.
func (s *itcmsCaseState) send(ctx context.Context, step compat.Step) error {
	event, err := decodeInfraTableCMSPayload(step.EventType, step.Payload)
	if err != nil {
		return err
	}
	return s.engine.Send(ctx, step.EventType, event)
}

func decodeInfraTableCMSPayload(eventType string, raw json.RawMessage) (any, error) {
	switch eventType {
	case "SupportBean":
		var bean itcmsBean
		if err := json.Unmarshal(raw, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_S0":
		var bean itcmsS0
		if err := json.Unmarshal(raw, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S0: %w", err)
		}
		return bean, nil
	case "SupportBean_S1":
		var bean itcmsS1
		if err := json.Unmarshal(raw, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S1: %w", err)
		}
		return bean, nil
	case "SupportBean_S2":
		var bean itcmsS2
		if err := json.Unmarshal(raw, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean_S2: %w", err)
		}
		return bean, nil
	case "SupportByteArrEventStringId":
		var bean itcmsByteArr
		if err := json.Unmarshal(raw, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportByteArrEventStringId: %w", err)
		}
		return bean, nil
	}
	return nil, fmt.Errorf("%s: unsupported eventType %q", infraTableCMSID, eventType)
}

// undeploy mirrors undeployModuleContaining: every deployment registered
// under the label undeploys and the label clears.
func (s *itcmsCaseState) undeploy(ctx context.Context, label string) error {
	for _, deployment := range s.deployments[label] {
		if err := deployment.Undeploy(ctx); err != nil {
			return err
		}
	}
	delete(s.deployments, label)
	delete(s.deployedLabels, label)
	return nil
}

func (s *itcmsCaseState) undeployAll(ctx context.Context) error {
	for label, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
		delete(s.deployments, label)
	}
	s.deployedLabels = make(map[string]bool)
	return nil
}

// itcmsNormalizeResults renders listener batches the way the pinned oracle
// serializes them: null/missing fields become JSON null and topk arrays
// become {value,frequency} objects in emission order.
func itcmsNormalizeResults(results []esper.Result) []compat.ResultRecord {
	if len(results) == 0 {
		return nil
	}
	normalized := make([]compat.ResultRecord, 0, len(results))
	for _, result := range results {
		if event, ok := result.Event(); ok {
			record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range event.Schema().Fields() {
				record.Fields[field.Name] = itcmsNormalizeValue(event.Get(field.Name))
			}
			normalized = append(normalized, record)
			continue
		}
		if row, ok := result.Row(); ok {
			record := compat.ResultRecord{Kind: "row", Fields: make(map[string]any)}
			for _, field := range row.Schema().Fields() {
				record.Fields[field.Name] = itcmsNormalizeValue(row.Get(field.Name))
			}
			normalized = append(normalized, record)
		}
	}
	return normalized
}

func itcmsNormalizeValue(value esper.Value) any {
	if value.IsMissing() || value.IsNull() {
		return nil
	}
	return itcmsNormalizeUnderlying(value.Any())
}

func itcmsNormalizeUnderlying(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case esper.Event:
		return itcmsNormalizeEvent(typed)
	case []esper.CountMinSketchTopKItem[string]:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, map[string]any{
				"value":     itcmsNormalizeUnderlying(item.Value),
				"frequency": item.Frequency,
			})
		}
		return items
	case []esper.Event:
		items := make([]any, 0, len(typed))
		for _, event := range typed {
			items = append(items, itcmsNormalizeEvent(event))
		}
		return items
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, itcmsNormalizeUnderlying(item))
		}
		return items
	case map[string]any:
		fields := make(map[string]any, len(typed))
		for name, field := range typed {
			fields[name] = itcmsNormalizeUnderlying(field)
		}
		return fields
	default:
		return typed
	}
}

// itcmsNormalizeEvent renders a fragment event like the oracle's
// normalizeValue: a table-row event (map underlying) becomes the positional
// Object[] array, while a bean event becomes the plain object.
func itcmsNormalizeEvent(event esper.Event) any {
	underlying := event.Underlying()
	if _, isMap := underlying.(map[string]any); isMap {
		items := make([]any, 0, len(event.Schema().Fields()))
		for _, field := range event.Schema().Fields() {
			items = append(items, itcmsNormalizeValue(event.Get(field.Name)))
		}
		return items
	}
	fields := make(map[string]any)
	for _, field := range event.Schema().Fields() {
		fields[field.Name] = itcmsNormalizeValue(event.Get(field.Name))
	}
	return fields
}

// itcmsCMSDecl renders the `countMinSketch()` table-declaration signature.
func itcmsCMSDecl() esper.TableAggDecl {
	return esper.TableAggDecl{
		Name:         "countMinSketch",
		Description:  "countMinSketch()",
		NthSize:      -1,
		RateInterval: -1,
	}
}

// itcmsCMSDeclTopK renders `countMinSketch({topk:N})`.
func itcmsCMSDeclTopK(topK int) esper.TableAggDecl {
	decl := itcmsCMSDecl()
	decl.TopK = topK
	decl.Description = fmt.Sprintf("countMinSketch({topk=%d})", topK)
	return decl
}

// itcmsCMSDeclAgent renders `countMinSketch({agent='...'})`; the agent string
// is carried as declared metadata because Go does not load agent classes.
func itcmsCMSDeclAgent(agent string) esper.TableAggDecl {
	decl := itcmsCMSDecl()
	decl.Agent = agent
	decl.Description = fmt.Sprintf("countMinSketch({agent='%s'})", agent)
	return decl
}

// itcmsCMSColumn renders the `countMinSketch()` table column.
func itcmsCMSColumn(name string) esper.TableColumn {
	return esper.TableColumnOf[esper.CountMinSketchValue[string]](name,
		esper.WithTableAggDecl(itcmsCMSDecl()))
}

// itcmsCMSColumnTopK renders the `countMinSketch({topk:N})` table column.
func itcmsCMSColumnTopK(name string, topK int) esper.TableColumn {
	return esper.TableColumnOf[esper.CountMinSketchValue[string]](name,
		esper.WithTableAggDecl(itcmsCMSDeclTopK(topK)))
}

// itcmsCMSColumnAgent renders the `countMinSketch({agent='...'})` table
// column.
func itcmsCMSColumnAgent(name, agent string) esper.TableColumn {
	return esper.TableColumnOf[esper.CountMinSketchValue[string]](name,
		esper.WithTableAggDecl(itcmsCMSDeclAgent(agent)))
}

// validateInfraTableCMSScenario pins the scenario identity; the strict
// loader already pins the case metadata and step sequence.
func validateInfraTableCMSScenario(scenario compat.Scenario) error {
	if scenario.Version != compat.ScenarioVersion || scenario.ID != infraTableCMSID {
		return fmt.Errorf("%s: scenario identity = %q/%q", infraTableCMSID, scenario.Version, scenario.ID)
	}
	return scenario.Validate()
}

// loadInfraTableCMSScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned case metadata, and per-step field whitelists
// so unknown or duplicated step fields fail the replay.
func loadInfraTableCMSScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraTableCMSID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraTableCMSID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableCMSID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableCMSID, err)
	}
	if err := requireInfraTableCMSFields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
		"javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectInfraTableCMSExtraFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraTableCMSID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraTableCMSID ||
		metadata.Description != infraTableCMSDescription ||
		metadata.JavaCommit != infraTableCMSJavaCommit ||
		metadata.JavaSource != infraTableCMSJavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraTableCMSID)
	}
	if err := validateInfraTableCMSStringArray(root["javaRuntimes"], infraTableCMSJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableCMSStringArray(root["javaNames"], infraTableCMSJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableCMSStringArray(root["javaStaticIds"], infraTableCMSJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraTableCMSStringArray(root["javaFlags"], infraTableCMSJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraTableCMSCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraTableCMSID, len(infraTableCMSCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraTableCMSFields(object,
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
		if definition.Case != infraTableCMSCases[index] ||
			definition.Ordinal != index ||
			definition.RuntimeID != infraTableCMSJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraTableCMSJavaExecutions[index] ||
			definition.Observation != infraTableCMSCaseObservations[index] ||
			definition.EPL != infraTableCMSCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraTableCMSID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", infraTableCMSID, err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl"},
		"deployed":     {"op", "case", "statement"},
		"send":         {"op", "case", "eventType", "payload"},
		"build-error":  {"op", "case", "statement", "epl", "expectError"},
		"undeploy":     {"op", "case", "statement"},
		"undeploy-all": {"op", "case"},
	}
	offset := 0
	for _, caseName := range infraTableCMSCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", infraTableCMSID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableCMSID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", infraTableCMSID, offset, caseName)
		}
		offset++
		want, ok := infraTableCMSCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", infraTableCMSID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", infraTableCMSID, caseName)
		}
		for _, pinned := range want {
			rawStep := rawSteps[offset]
			var object map[string]json.RawMessage
			if err := strictObject(rawStep, &object); err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableCMSID, offset, err)
			}
			var op string
			if err := json.Unmarshal(object["op"], &op); err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d op: %w", infraTableCMSID, offset, err)
			}
			allowed, ok := stepFields[op]
			if !ok {
				return compat.Scenario{}, fmt.Errorf("%s step %d has unsupported op %q", infraTableCMSID, offset, op)
			}
			for field := range object {
				found := false
				for _, name := range allowed {
					if field == name {
						found = true
						break
					}
				}
				if !found {
					return compat.Scenario{}, fmt.Errorf("%s step %d has unexpected field %q", infraTableCMSID, offset, field)
				}
			}
			var stepCase string
			if err := json.Unmarshal(object["case"], &stepCase); err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d case: %w", infraTableCMSID, offset, err)
			}
			if stepCase != caseName {
				return compat.Scenario{}, fmt.Errorf("%s step %d carries case %q, want %q", infraTableCMSID, offset, stepCase, caseName)
			}
			key, err := infraTableCMSStepKey(rawStep)
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", infraTableCMSID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", infraTableCMSID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", infraTableCMSID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraTableCMSID, err)
	}
	if len(scenario.Steps) == 0 {
		return compat.Scenario{}, fmt.Errorf("%s scenario has no steps", infraTableCMSID)
	}
	return scenario, nil
}

func requireInfraTableCMSFields(object map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s scenario is missing field %q", infraTableCMSID, name)
		}
	}
	return nil
}

// rejectInfraTableCMSExtraFields rejects top-level fields outside the pinned
// scenario schema so a mutated scenario cannot smuggle ignored keys.
func rejectInfraTableCMSExtraFields(object map[string]json.RawMessage, names ...string) error {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	for name := range object {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("%s scenario has unexpected field %q", infraTableCMSID, name)
		}
	}
	return nil
}

func validateInfraTableCMSStringArray(raw json.RawMessage, want []string, field string) error {
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		return fmt.Errorf("%s scenario field %q: %w", infraTableCMSID, field, err)
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s scenario field %q length = %d, want %d", infraTableCMSID, field, len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("%s scenario field %q[%d] = %q, want %q", infraTableCMSID, field, index, got[index], want[index])
		}
	}
	return nil
}

// infraTableCMSStepKey renders the pinned step key `op|statement|eventType`
// plus the compacted payload for send steps, matching the Java oracle's
// stepKey payload pinning and the sibling-runner convention.
func infraTableCMSStepKey(raw json.RawMessage) (string, error) {
	var step struct {
		Op        string          `json:"op"`
		Statement string          `json:"statement"`
		EventType string          `json:"eventType"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	key := step.Op + "|" + step.Statement + "|" + step.EventType
	if len(step.Payload) > 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", err
		}
		key += "|" + compacted.String()
	}
	return key, nil
}
