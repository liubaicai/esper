package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// epl_contained_event_example.go replays EPLContainedEventExample ords 0-5
// against the pinned Java oracle: the media-order XML contained-event
// example. Six executions on six runtimes.
//
// contained-example (ord 0, EPLContainedExample) deploys twelve statements
// over the MediaOrder XML DOM type and sends mediaOrderOne.xml once: s1
// selects orderId plus the indexed items.item[0].itemId path, s2 expands
// [books.book], s3 filters the parent before expansion, s4 counts over
// #unique(bookId), s5 expands [books.book][review], s6 is a never-fired
// Cancel->book pattern, s7-s10 exercise contained select projections
// (parent fields, wildcard fragments, grandparent references), s11_0
// inserts fragment-typed columns into ReviewStream and s11 reads them
// back, and s12 filters the book fragment before expanding review.
//
// solution-pattern (ord 1, EPLContainedSolutionPattern) expands the
// SupportResponseEvent.subEvents bean array inside a contained select that
// carries the parent category, windows the fragments over #time(1 min) and
// groups by category+subEventType; two sends yield cumulative averages.
//
// join-self-join / join-self-left-outer / join-self-full-outer (ords 2-4)
// self-join the book and item contained streams on productId=bookId in
// three phases separated by undeploy-all: the row select, a cumulative
// count(*) join and a unidirectional count(*) join.
//
// financial (ord 5, EPLContainedSolutionPatternFinancial) deploys one
// module: Symbol and bus-event ForeignSymbols/LocalSymbols map schemas,
// the two-key Mapping table with secondary indexes, the SymbolsPair
// lastevent join insert, the split-all on-trigger that emits begin marker,
// contained foreign/local company rows, output marker and end marker, the
// start/end-event SymbolsPairContext, the contexted Result table, the two
// contexted merges with scalar Mapping subselects, and the ordered 'out'
// select. Four fire-and-forget positional-parameter inserts load Mapping;
// ForeignSymbols/LocalSymbols sends then produce the four ordered rows.
//
// Approved differences (observably identical to the Java EPL):
//   - The XML DOM type maps to RegisterXML with nil-typed fields; the XSD
//     schemaResource has no Go counterpart because Go resolves XML fields
//     dynamically through the parsed document.
//   - `create schema`/`create table`/`create context` module statements map
//     to env-level registrations (RegisterMap, CreateTable,
//     CreatePatternInitiatedTerminatedContext); Go has no module path.
//   - `select *` wildcard fragment projections expand to the pinned
//     explicit Alias selections the Java assertions read; contained select
//     projections ([select ... from books.book]) collapse to the equivalent
//     ancestor-field reads on the doubly-unnested stream.
//   - `insert into X select null` marker events map to zero-selection
//     SplitInto branches over empty registered schemas.
//   - The merge `where result.K = fsr.symbol` predicates map to merge key
//     expressions; the second primary-key column resolves through the
//     scalar Mapping subselect so each merge still carries two keys.
//   - Java milestone(0) is a harness no-op and carries no step.
//   - Listener batches record in canonical sorted-field order on both
//     traces (the infra-nwtable-context precedent).

const eplContainedEventExampleID = "epl-contained-event-example"
const eplContainedEventExampleJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

const eplContainedEventExampleDescription = "EPLContainedEventExample ords 0-5: the media-order XML contained-event example. Ord 0 expands MediaOrder[books.book] and [books.book][review] contained fragments (s1 indexed item path, s2 wildcard book, s3 filtered parent, s4 unique-count, s5 review fields, s6 unobserved pattern, s7-s10 contained select projections, s11 ReviewStream round-trip, s12 filtered book). Ord 1 expands the SupportResponseEvent.subEvents bean array under a 1-minute time window grouped by parent category. Ords 2-4 self-join the book and item contained streams on productId=bookId (inner, left-outer, full-outer) plus cumulative and unidirectional count phases. Ord 5 is the financial solution pattern: a SymbolsPair lastevent join feeds a split-all on-trigger that unnests foreign/local companies, merges foreign and local Symbol rows into the contexted Result table via Mapping subselects, and fires the ordered SymbolsPairOutputEvent select between begin/end markers (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/contained/EPLContainedEventExample.java)."

const eplContainedEventExampleSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/contained/EPLContainedEventExample.java"

// Verbatim transcriptions of EPLContainedEventExample lines 156-194 (ord 0),
// 371 (ord 1), 225/244/255 (ord 2), 278/294/305 (ord 3), 328/344/355 (ord 4)
// and 64-95 (ord 5 module).
const (
	eceS1    = "@name('s1') select orderId, items.item[0].itemId from MediaOrder"
	eceS2    = "@name('s2') select * from MediaOrder[books.book]"
	eceS3    = "@name('s3') select * from MediaOrder(orderId='PO200901')[books.book]"
	eceS4    = "@name('s4') select count(*) from MediaOrder[books.book]#unique(bookId)"
	eceS5    = "@name('s5') select * from MediaOrder[books.book][review]"
	eceS6    = "@name('s6') select * from pattern [c=Cancel -> o=MediaOrder(orderId = c.orderId)[books.book]]"
	eceS7    = "@name('s7') select * from MediaOrder[select orderId, bookId from books.book][select * from review]"
	eceS8    = "@name('s8') select * from MediaOrder[select * from books.book][select reviewId, comment from review]"
	eceS9    = "@name('s9') select * from MediaOrder[books.book as book][select book.*, reviewId, comment from review]"
	eceS10   = "@name('s10') select * from MediaOrder[books.book as book][select mediaOrder.*, bookId, reviewId from review] as mediaOrder"
	eceS11_0 = "@name('s11_0') @public insert into ReviewStream select * from MediaOrder[books.book as book]\n    [select mediaOrder.* as mediaOrder, book.* as book, review.* as review from review as review] as mediaOrder"
	eceS11   = "@name('s11') select mediaOrder.orderId, book.bookId, review.reviewId from ReviewStream"
	eceS12   = "@name('s12') select * from MediaOrder[books.book where author = 'Orson Scott Card'][review]"

	eceOrd1S0 = "@name('s0') select category, subEventType, avg(responseTimeMillis) as avgTime from SupportResponseEvent[select category, * from subEvents]#time(1 min) group by category, subEventType order by category, subEventType"

	eceRowsInner = "@name('s0') select book.bookId,item.itemId from MediaOrder[books.book] as book, MediaOrder[items.item] as item where productId = bookId order by bookId, item.itemId asc"
	eceCntInner  = "@name('s0') select count(*) from MediaOrder[books.book] as book, MediaOrder[items.item] as item where productId = bookId order by bookId asc"
	eceUniInner  = "@name('s0') select count(*) from MediaOrder[books.book] as book unidirectional, MediaOrder[items.item] as item where productId = bookId order by bookId asc"
	eceRowsLeft  = "@name('s0') select book.bookId,item.itemId from MediaOrder[books.book] as book left outer join MediaOrder[items.item] as item on productId = bookId order by bookId, item.itemId asc"
	eceCntLeft   = "@name('s0') select count(*) from MediaOrder[books.book] as book left outer join MediaOrder[items.item] as item on productId = bookId"
	eceUniLeft   = "@name('s0') select count(*) from MediaOrder[books.book] as book unidirectional left outer join MediaOrder[items.item] as item on productId = bookId"
	eceRowsFull  = "@name('s0') select orderId, book.bookId,item.itemId from MediaOrder[books.book] as book full outer join MediaOrder[select orderId, * from items.item] as item on productId = bookId order by bookId, item.itemId asc"
	eceCntFull   = "@name('s0') select count(*) from MediaOrder[books.book] as book full outer join MediaOrder[items.item] as item on productId = bookId"
	eceUniFull   = "@name('s0') select count(*) from MediaOrder[books.book] as book unidirectional full outer join MediaOrder[items.item] as item on productId = bookId"

	eceModule = "create schema Symbol(symbol string, value double);\n" +
		"@public @buseventtype create schema ForeignSymbols(companies Symbol[]);\n" +
		"@public @buseventtype create schema LocalSymbols(companies Symbol[]);\n" +
		"\n" +
		"@public create table Mapping(foreignSymbol string primary key, localSymbol string primary key);\n" +
		"create index MappingIndexForeignSymbol on Mapping(foreignSymbol);\n" +
		"create index MappingIndexLocalSymbol on Mapping(localSymbol);\n" +
		"\n" +
		"insert into SymbolsPair select * from ForeignSymbols#lastevent as foreign, LocalSymbols#lastevent as local;\n" +
		"on SymbolsPair\n" +
		"  insert into SymbolsPairBeginEvent select null\n" +
		"  insert into ForeignSymbolRow select * from [foreign.companies]\n" +
		"  insert into LocalSymbolRow select * from [local.companies]\n" +
		"  insert into SymbolsPairOutputEvent select null" +
		"  insert into SymbolsPairEndEvent select null" +
		"  output all;\n" +
		"\n" +
		"create context SymbolsPairContext start SymbolsPairBeginEvent end SymbolsPairEndEvent;\n" +
		"context SymbolsPairContext create table Result(foreignSymbol string primary key, localSymbol string primary key, value double);\n" +
		"\n" +
		"context SymbolsPairContext on ForeignSymbolRow as fsr merge Result as result where result.foreignSymbol = fsr.symbol\n" +
		"  when not matched then insert select fsr.symbol as foreignSymbol,\n" +
		"    (select localSymbol from Mapping as mapping where mapping.foreignSymbol = fsr.symbol) as localSymbol, fsr.value as value\n" +
		"  when matched and fsr.value > result.value then update set value = fsr.value;\n" +
		"\n" +
		"context SymbolsPairContext on LocalSymbolRow as lsr merge Result as result where result.localSymbol = lsr.symbol\n" +
		"  when not matched then insert select (select foreignSymbol from Mapping as mapping where mapping.localSymbol = lsr.symbol) as foreignSymbol," +
		"    lsr.symbol as localSymbol, lsr.value as value\n" +
		"  when matched and lsr.value > result.value then update set value = lsr.value;\n" +
		"\n" +
		"@name('out') context SymbolsPairContext on SymbolsPairOutputEvent select foreignSymbol, localSymbol, value from Result order by foreignSymbol asc;\n"
	eceFAFMapping = "insert into Mapping select ?::string as foreignSymbol, ?::string as localSymbol"
)

var eplContainedEventExampleJavaSources = []string{
	eplContainedEventExampleSource,
}

var eplContainedEventExampleJavaRuntimeIDs = []string{
	"java-runtime-36bfa282c286614e1bc0", // EPLContainedExample
	"java-runtime-3bcf56da38fd1e2f669a", // EPLContainedSolutionPattern
	"java-runtime-06cbcd9c14891d14a995", // EPLContainedJoinSelfJoin
	"java-runtime-bfcd3e5954bd02726c90", // EPLContainedJoinSelfLeftOuterJoin
	"java-runtime-e9cfcbfcc5556a9ea9ab", // EPLContainedJoinSelfFullOuterJoin
	"java-runtime-23292c0fd335038c05d4", // EPLContainedSolutionPatternFinancial
}

var eplContainedEventExampleJavaExecutions = []string{
	"EPLContainedExample",
	"EPLContainedSolutionPattern",
	"EPLContainedJoinSelfJoin",
	"EPLContainedJoinSelfLeftOuterJoin",
	"EPLContainedJoinSelfFullOuterJoin",
	"EPLContainedSolutionPatternFinancial",
}

// One static-manifest id per execution, aligned with javaRuntimes order.
var eplContainedEventExampleJavaStaticIDs = []string{
	"java-d763dc53250784007213",
	"java-3370a9fc6c79a91c99db",
	"java-a340361a64068355d364",
	"java-e0629eece325f19d5eab",
	"java-b66b814fdafb047c88ca",
	"java-364de601500af6cb9ce9",
}

var eplContainedEventExampleJavaFlags = []string{"FIREANDFORGET"}

var eplContainedEventExampleCases = []string{
	"contained-example",
	"solution-pattern",
	"join-self-join",
	"join-self-left-outer",
	"join-self-full-outer",
	"financial",
}

var eplContainedEventExampleOrdinals = []int{0, 1, 2, 3, 4, 5}

var eplContainedEventExampleCaseObservations = []string{
	"listener; twelve deploys over the MediaOrder XML DOM type: s1 orderId plus indexed items.item[0].itemId, s2 wildcard book fragments, s3 filtered parent expansion, s4 count over #unique(bookId), s5 review fragments, s6 unobserved Cancel->book pattern, s7-s10 contained select projections, s11 ReviewStream round-trip, s12 filtered book expansion; one mediaOrderOne.xml send fires every statement except s6",
	"listener; one deploy expands the SupportResponseEvent.subEvents bean array under #time(1 min) grouped by category+subEventType; two bean sends yield cumulative averages {svcOne,typeA,1000},{svcOne,typeB,800} then {svcOne,typeA,750},{svcOne,typeB,600}",
	"listener; three phases separated by undeploy-all: the book/item inner join row select, a cumulative count(*) join and a unidirectional count(*) join; sends docOne, docOne, docTwo, then docTwo, docOne, then docTwo, docOne",
	"listener; three phases separated by undeploy-all: the book/item left-outer join row select, a cumulative count(*) join and a unidirectional count(*) join; sends docTwo, docOne, then docTwo, docOne, then docTwo, docOne",
	"listener; three phases separated by undeploy-all: the book/item full-outer join row select (item side carries the contained select orderId), a cumulative count(*) join and a unidirectional count(*) join; sends docTwo, docOne, then docTwo, docOne, then docTwo, docOne",
	"listener; one module deploy registers map schemas, the Mapping table with secondary indexes, the SymbolsPair lastevent insert, the begin/foreign/local/output/end split-all with contained branches, the start/end-event context, the contexted Result table merges with Mapping subselects and the ordered out select; four fire-and-forget Mapping loads seed the pairs; ForeignSymbols/LocalSymbols sends produce four ordered rows",
}

// eplContainedEventExampleCaseEPLs pins the newline-joined EPL of every
// deploy step in the case, in step order — the value carried by the
// scenario cases[] metadata.
var eplContainedEventExampleCaseEPLs = []string{
	strings.Join([]string{eceS1, eceS2, eceS3, eceS4, eceS5, eceS6, eceS7,
		eceS8, eceS9, eceS10, eceS11_0, eceS11, eceS12}, "\n"),
	eceOrd1S0,
	strings.Join([]string{eceRowsInner, eceCntInner, eceUniInner}, "\n"),
	strings.Join([]string{eceRowsLeft, eceCntLeft, eceUniLeft}, "\n"),
	strings.Join([]string{eceRowsFull, eceCntFull, eceUniFull}, "\n"),
	eceModule,
}

// eplContainedEventExampleCaseEPLSets pins the byte-exact EPL texts each
// case may carry on deploy/faf steps (join cases reuse statement name s0
// across phases, so membership is keyed on the EPL text itself).
var eplContainedEventExampleCaseEPLSets = map[string]map[string]bool{
	"contained-example": eceEPLSet(eceS1, eceS2, eceS3, eceS4, eceS5, eceS6,
		eceS7, eceS8, eceS9, eceS10, eceS11_0, eceS11, eceS12),
	"solution-pattern":     eceEPLSet(eceOrd1S0),
	"join-self-join":       eceEPLSet(eceRowsInner, eceCntInner, eceUniInner),
	"join-self-left-outer": eceEPLSet(eceRowsLeft, eceCntLeft, eceUniLeft),
	"join-self-full-outer": eceEPLSet(eceRowsFull, eceCntFull, eceUniFull),
	"financial":            eceEPLSet(eceModule, eceFAFMapping),
}

func eceEPLSet(epls ...string) map[string]bool {
	set := make(map[string]bool, len(epls))
	for _, epl := range epls {
		set[epl] = true
	}
	return set
}

// eplContainedEventExampleListenerNames pins the deployed statement name the
// listener attaches to per deploy EPL (empty = no listener, s11_0).
var eplContainedEventExampleListenerNames = map[string]string{
	eceS1: "s1", eceS2: "s2", eceS3: "s3", eceS4: "s4", eceS5: "s5",
	eceS6: "s6", eceS7: "s7", eceS8: "s8", eceS9: "s9", eceS10: "s10",
	eceS11_0: "", eceS11: "s11", eceS12: "s12",
	eceOrd1S0:    "s0",
	eceRowsInner: "s0", eceCntInner: "s0", eceUniInner: "s0",
	eceRowsLeft: "s0", eceCntLeft: "s0", eceUniLeft: "s0",
	eceRowsFull: "s0", eceCntFull: "s0", eceUniFull: "s0",
	eceModule: "out",
}

// eplContainedEventExampleCaseSteps pins the complete step sequence per case
// as op|statement|eventType|epl|listen|fields keys so the loader asserts the
// scenario file matches the contract. Java milestone(0) carries no step.
var eplContainedEventExampleCaseSteps = map[string][]string{
	"contained-example": {
		eceDeployKey("s1", eceS1, "s1", "items.item[0].itemId,orderId"),
		eceDeployKey("s2", eceS2, "s2", "bookId"),
		eceDeployKey("s3", eceS3, "s3", "bookId"),
		eceDeployKey("s4", eceS4, "s4", "count(*)"),
		eceDeployKey("s5", eceS5, "s5", "reviewId"),
		eceDeployKey("s6", eceS6, "s6", "c.orderId,o.bookId"),
		eceDeployKey("s7", eceS7, "s7", "bookId,orderId,reviewId"),
		eceDeployKey("s8", eceS8, "s8", "bookId,reviewId"),
		eceDeployKey("s9", eceS9, "s9", "bookId,reviewId"),
		eceDeployKey("s10", eceS10, "s10", "bookId,reviewId"),
		eceDeployKey("s11_0", eceS11_0, "", ""),
		eceDeployKey("s11", eceS11, "s11", "book.bookId,mediaOrder.orderId,review.reviewId"),
		eceDeployKey("s12", eceS12, "s12", "reviewId"),
		"send||MediaOrder|||",
		"undeploy-all|||||",
	},
	"solution-pattern": {
		eceDeployKey("s0", eceOrd1S0, "s0", "avgTime,category,subEventType"),
		"send||SupportResponseEvent|||",
		"send||SupportResponseEvent|||",
		"undeploy-all|||||",
	},
	"join-self-join": {
		eceDeployKey("s0", eceRowsInner, "s0", "book.bookId,item.itemId"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
		eceDeployKey("s0", eceCntInner, "s0", "count(*)"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
		eceDeployKey("s0", eceUniInner, "s0", "count(*)"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
	},
	"join-self-left-outer": {
		eceDeployKey("s0", eceRowsLeft, "s0", "book.bookId,item.itemId"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
		eceDeployKey("s0", eceCntLeft, "s0", "count(*)"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
		eceDeployKey("s0", eceUniLeft, "s0", "count(*)"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
	},
	"join-self-full-outer": {
		eceDeployKey("s0", eceRowsFull, "s0", "book.bookId,item.itemId,orderId"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
		eceDeployKey("s0", eceCntFull, "s0", "count(*)"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
		eceDeployKey("s0", eceUniFull, "s0", "count(*)"),
		"send||MediaOrder|||",
		"send||MediaOrder|||",
		"undeploy-all|||||",
	},
	"financial": {
		eceDeployKey("module", eceModule, "out", "foreignSymbol,localSymbol,value"),
		"faf|load-mapping||" + eceFAFMapping + "||",
		"faf|load-mapping||" + eceFAFMapping + "||",
		"faf|load-mapping||" + eceFAFMapping + "||",
		"faf|load-mapping||" + eceFAFMapping + "||",
		"send||ForeignSymbols|||",
		"send||LocalSymbols|||",
		"undeploy-all|||||",
	},
}

func eceDeployKey(statement, epl, listen, fields string) string {
	return "deploy|" + statement + "||" + epl + "|" + listen + "|" + fields
}

// eceResponseSubEvent mirrors SupportResponseSubEvent's asserted fields
// (responseTimeMillis, subEventType).
type eceResponseSubEvent struct {
	ResponseTimeMillis int64  `esper:"responseTimeMillis" json:"responseTimeMillis"`
	SubEventType       string `esper:"subEventType" json:"subEventType"`
}

// eceResponseEvent mirrors SupportResponseEvent's asserted fields (category,
// subEvents).
type eceResponseEvent struct {
	Category  string                `esper:"category" json:"category"`
	SubEvents []eceResponseSubEvent `esper:"subEvents" json:"subEvents"`
}

// eplContainedEventExampleCaseState carries the per-case replay state: the
// environment/engine pair, deployment bookkeeping for undeploy-all, the
// prepared FAF handle for the financial mapping load, and the
// trace/sequence counters the listener records draw from.
type eplContainedEventExampleCaseState struct {
	caseName    string
	env         *esper.Environment
	engine      *esper.Engine
	trace       *compat.Trace
	sequences   map[string]uint64
	deployments map[string]*esper.Deployment
	deployOrder []string
	preparedFAF *esper.PreparedQuery
}

func runEPLContainedEventExampleScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	found := false
	for _, caseName := range eplContainedEventExampleCases {
		if !scenarioHasCase(scenario, caseName) {
			continue
		}
		found = true
		caseScenario, err := scenarioForCase(scenario, caseName)
		if err != nil {
			return compat.Trace{}, err
		}
		caseTrace, err := runEPLContainedEventExampleCase(ctx, caseScenario, caseName)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", eplContainedEventExampleID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if !found {
		return compat.Trace{}, fmt.Errorf("%s scenario %q has no supported cases", eplContainedEventExampleID, scenario.ID)
	}
	return trace, nil
}

func runEPLContainedEventExampleCase(ctx context.Context, scenario compat.Scenario, caseName string) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if err := registerEPLContainedEventExampleSchemas(env); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(eplContainedEventExampleJavaRuntimeIDs[eplContainedEventExampleOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	state := &eplContainedEventExampleCaseState{
		caseName:    caseName,
		env:         env,
		engine:      engine,
		trace:       &trace,
		sequences:   make(map[string]uint64),
		deployments: make(map[string]*esper.Deployment),
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
		case "send":
			if err := state.send(ctx, step); err != nil {
				return *state.trace, err
			}
		case "faf":
			if err := state.faf(ctx, step); err != nil {
				return *state.trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return *state.trace, err
			}
		default:
			return *state.trace, fmt.Errorf("%s: unsupported step op %q", eplContainedEventExampleID, step.Op)
		}
	}
	return *state.trace, nil
}

func eplContainedEventExampleOrdinal(caseName string) int {
	for index, name := range eplContainedEventExampleCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// registerEPLContainedEventExampleSchemas mirrors the suite configuration:
// MediaOrder and Cancel share the XML DOM registration (nil-typed fields
// resolved dynamically through the parsed document), Book/Item/Review are
// the contained fragment map schemas, SupportResponseEvent/
// SupportResponseSubEvent are bean structs, and the financial case's map
// schemas, tables and context register up front because Go has no module
// path (the Java module's create statements are env-level registrations).
func registerEPLContainedEventExampleSchemas(env *esper.Environment) error {
	xmlFields := []esper.FieldSpec{
		esper.FieldDef("orderId", nil),
		esper.FieldDef("items", nil),
		esper.FieldDef("books", nil),
	}
	if _, err := esper.RegisterXML(env, "MediaOrder", xmlFields); err != nil {
		return err
	}
	if _, err := esper.RegisterXML(env, "Cancel", xmlFields); err != nil {
		return err
	}
	fragment := func(name string, fields ...string) error {
		specs := make([]esper.FieldSpec, 0, len(fields))
		for _, field := range fields {
			specs = append(specs, esper.FieldDef(field, nil))
		}
		_, err := esper.RegisterMap(env, name, specs)
		return err
	}
	if err := fragment("Book", "bookId", "author", "review"); err != nil {
		return err
	}
	if err := fragment("Item", "itemId", "productId", "amount", "price"); err != nil {
		return err
	}
	if err := fragment("Review", "reviewId", "comment"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[eceResponseEvent](env, "SupportResponseEvent"); err != nil {
		return err
	}
	if _, err := esper.RegisterStruct[eceResponseSubEvent](env, "SupportResponseSubEvent"); err != nil {
		return err
	}
	// ReviewStream carries fragment-typed columns (mediaOrder/book/review)
	// produced by the s11_0 contained-select insert.
	if err := fragment("ReviewStream", "mediaOrder", "book", "review"); err != nil {
		return err
	}
	// Financial module registrations.
	if err := fragment("Symbol", "symbol", "value"); err != nil {
		return err
	}
	if err := fragment("ForeignSymbols", "companies"); err != nil {
		return err
	}
	if err := fragment("LocalSymbols", "companies"); err != nil {
		return err
	}
	if err := fragment("ForeignSymbolRow", "symbol", "value"); err != nil {
		return err
	}
	if err := fragment("LocalSymbolRow", "symbol", "value"); err != nil {
		return err
	}
	if err := fragment("SymbolsPair", "foreign", "local"); err != nil {
		return err
	}
	if err := fragment("SymbolsPairOutputEvent", "foreignSymbol", "localSymbol", "value"); err != nil {
		return err
	}
	if err := fragment("SymbolsPairBeginEvent"); err != nil {
		return err
	}
	if err := fragment("SymbolsPairEndEvent"); err != nil {
		return err
	}
	if _, err := esper.CreateTable(env, "Mapping", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("foreignSymbol"),
		esper.PrimaryKeyColumn[string]("localSymbol"),
	}, esper.SecondaryIndex("MappingIndexForeignSymbol", "foreignSymbol"),
		esper.SecondaryIndex("MappingIndexLocalSymbol", "localSymbol")); err != nil {
		return err
	}
	if _, err := esper.CreatePatternInitiatedTerminatedContext(env, "SymbolsPairContext",
		esper.PatternFrom(esper.From[map[string]any](env, "SymbolsPairBeginEvent"), "s", esper.Literal(true)),
		esper.PatternFrom(esper.From[map[string]any](env, "SymbolsPairEndEvent"), "e", esper.Literal(true))); err != nil {
		return err
	}
	_, err := esper.CreateTable(env, "Result", []esper.TableColumn{
		esper.PrimaryKeyColumn[string]("foreignSymbol"),
		esper.PrimaryKeyColumn[string]("localSymbol"),
		esper.TableColumnOf[float64]("value"),
	}, esper.TableContext("SymbolsPairContext"))
	return err
}

// deploy maps each scenario step to the equivalent Go plan and attaches the
// listener that emits the normalized per-delivery records, mirroring the
// Java compileDeploy(epl, path).addListener(name). The pinned EPL text
// selects the plan (join cases reuse statement name s0 across phases).
func (s *eplContainedEventExampleCaseState) deploy(ctx context.Context, step compat.Step) error {
	if !eplContainedEventExampleCaseEPLSets[s.caseName][step.Epl] {
		return fmt.Errorf("%s: deploy %q in case %q carries an unpinned EPL %q",
			eplContainedEventExampleID, step.Statement, s.caseName, step.Epl)
	}
	if err := s.build(ctx, step); err != nil {
		return fmt.Errorf("%s: deploy %q in case %q: %w", eplContainedEventExampleID, step.Statement, s.caseName, err)
	}
	return nil
}

// eceMediaOrderStreams returns the contained-expansion helpers over the
// MediaOrder XML source.
func eceBooks(source esper.Stream[esper.Event]) esper.Stream[esper.Event] {
	return esper.UnnestAs[esper.Event, map[string]any](source,
		esper.Property[[]map[string]any](esper.EventValue[esper.Event](), "books.book"), "Book")
}

func eceItems(source esper.Stream[esper.Event]) esper.Stream[esper.Event] {
	return esper.UnnestAs[esper.Event, map[string]any](source,
		esper.Property[[]map[string]any](esper.EventValue[esper.Event](), "items.item"), "Item")
}

func eceReviews(source esper.Stream[esper.Event]) esper.Stream[esper.Event] {
	return esper.UnnestAs[esper.Event, map[string]any](source,
		esper.Property[[]map[string]any](esper.EventValue[esper.Event](), "review"), "Review")
}

// build constructs and deploys the Go plan(s) for one deploy step. The
// financial module deploys its intermediate statements (pair insert, split,
// merges) inside build and returns through the shared 'out' deploy.
func (s *eplContainedEventExampleCaseState) build(ctx context.Context, step compat.Step) error {
	env := s.env
	mediaOrder := esper.From[esper.Event](env, "MediaOrder")
	listen := eplContainedEventExampleListenerNames[step.Epl]
	deploy := func(label string, query esper.Query, listenName string) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan, listenName)
	}
	switch step.Epl {
	case eceS1:
		return deploy("s1", mediaOrder.AsRecord().Select(
			esper.Alias("orderId", esper.Field[esper.Event, string]("orderId")),
			esper.Alias("items.item[0].itemId", esper.Property[string](
				esper.ArrayAt[map[string]any](
					esper.Property[[]map[string]any](esper.EventValue[esper.Event](), "items.item"),
					esper.Literal(0)),
				"itemId")),
		).Query(esper.StatementName("s1")), listen)
	case eceS2:
		return deploy("s2", eceBooks(mediaOrder).AsRecord().Select(
			esper.Alias("bookId", esper.Field[esper.Event, string]("bookId")),
		).Query(esper.StatementName("s2")), listen)
	case eceS3:
		return deploy("s3", eceBooks(mediaOrder.Filter(
			esper.Equal[string](esper.Field[esper.Event, string]("orderId"), esper.Literal("PO200901")))).AsRecord().Select(
			esper.Alias("bookId", esper.Field[esper.Event, string]("bookId")),
		).Query(esper.StatementName("s3")), listen)
	case eceS4:
		return deploy("s4", eceBooks(mediaOrder).
			Window(esper.Unique(esper.Field[esper.Event, string]("bookId"))).
			Aggregate(esper.Alias("count(*)", esper.CountAll())).
			Query(esper.StatementName("s4")), listen)
	case eceS5:
		return deploy("s5", eceReviews(eceBooks(mediaOrder)).AsRecord().Select(
			esper.Alias("reviewId", esper.Field[esper.Event, string]("reviewId")),
		).Query(esper.StatementName("s5")), listen)
	case eceS6:
		return deploy("s6", esper.PatternFrom(esper.From[esper.Event](env, "Cancel"), "c", esper.Literal(true)).
			Then(esper.PatternFrom(eceBooks(mediaOrder), "o",
				esper.Equal[string](esper.ContainedParentField[string]("orderId"),
					esper.TagField[string]("c", "orderId")))).
			Select(
				esper.Alias("c.orderId", esper.TagField[string]("c", "orderId")),
				esper.Alias("o.bookId", esper.TagField[string]("o", "bookId")),
			).
			Query(esper.StatementName("s6")), listen)
	case eceS7:
		return deploy("s7", eceReviews(eceBooks(mediaOrder)).AsRecord().Select(
			esper.Alias("orderId", esper.ContainedAncestorField[string](2, "orderId")),
			esper.Alias("bookId", esper.ContainedParentField[string]("bookId")),
			esper.Alias("reviewId", esper.Field[esper.Event, string]("reviewId")),
		).Query(esper.StatementName("s7")), listen)
	case eceS8, eceS9, eceS10:
		// The three contained-select variants assert the same {bookId,
		// reviewId} pair; the Go plan reads bookId through the review
		// fragment's parent chain.
		name := map[string]string{eceS8: "s8", eceS9: "s9", eceS10: "s10"}[step.Epl]
		return deploy(name, eceReviews(eceBooks(mediaOrder)).AsRecord().Select(
			esper.Alias("bookId", esper.ContainedParentField[string]("bookId")),
			esper.Alias("reviewId", esper.Field[esper.Event, string]("reviewId")),
		).Query(esper.StatementName(name)), listen)
	case eceS11_0:
		return deploy("s11_0", eceReviews(eceBooks(mediaOrder)).AsRecord().Select(
			esper.Alias("mediaOrder", esper.ContainedAncestorEvent(2)),
			esper.Alias("book", esper.ContainedParentEvent()),
			esper.Alias("review", esper.EventValue[esper.Event]()),
		).InsertInto("ReviewStream", esper.StatementName("s11_0")), listen)
	case eceS11:
		return deploy("s11", esper.FromAny(env, "ReviewStream").Select(
			esper.Alias("mediaOrder.orderId", esper.Property[string](
				esper.Field[any, esper.Event]("mediaOrder"), "orderId")),
			esper.Alias("book.bookId", esper.Property[string](
				esper.Field[any, esper.Event]("book"), "bookId")),
			esper.Alias("review.reviewId", esper.Property[string](
				esper.Field[any, esper.Event]("review"), "reviewId")),
		).Query(esper.StatementName("s11")), listen)
	case eceS12:
		filtered := eceBooks(mediaOrder).Filter(
			esper.Equal[string](esper.Field[esper.Event, string]("author"), esper.Literal("Orson Scott Card")))
		return deploy("s12", eceReviews(filtered).AsRecord().Select(
			esper.Alias("reviewId", esper.Field[esper.Event, string]("reviewId")),
		).Query(esper.StatementName("s12")), listen)
	case eceOrd1S0:
		subEvents := esper.Unnest[eceResponseEvent, eceResponseSubEvent](
			esper.From[eceResponseEvent](env, "SupportResponseEvent"),
			esper.Property[[]eceResponseSubEvent](esper.EventValue[eceResponseEvent](), "subEvents"))
		return deploy("s0", subEvents.Window(esper.TimeWindow(time.Minute)).
			GroupBy(esper.ContainedParentField[string]("category"),
				esper.Field[eceResponseSubEvent, string]("subEventType")).
			Select(
				esper.Alias("category", esper.ContainedParentField[string]("category")),
				esper.Alias("subEventType", esper.Field[eceResponseSubEvent, string]("subEventType")),
				esper.Alias("avgTime", esper.Avg[int64](esper.Field[eceResponseSubEvent, int64]("responseTimeMillis"))),
			).Query(esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(esper.ResultField[any]("category")),
				esper.Ascending(esper.ResultField[any]("subEventType")))), listen)
	case eceRowsInner, eceCntInner, eceUniInner,
		eceRowsLeft, eceCntLeft, eceUniLeft,
		eceRowsFull, eceCntFull, eceUniFull:
		return s.buildJoin(ctx, step)
	case eceModule:
		return s.buildFinancial(ctx)
	}
	return fmt.Errorf("%s: unknown deploy EPL in case %q", eplContainedEventExampleID, s.caseName)
}

// buildJoin constructs the ord 2-4 book/item contained self-join plans.
// The rows EPL is the ordered row select, the count EPL the cumulative
// count(*) join and the uni EPL the unidirectional count(*) join.
func (s *eplContainedEventExampleCaseState) buildJoin(ctx context.Context, step compat.Step) error {
	env := s.env
	bookStream := eceBooks(esper.From[esper.Event](env, "MediaOrder"))
	itemStream := eceItems(esper.From[esper.Event](env, "MediaOrder"))
	bookSource := esper.JoinSource(bookStream)
	unidirectional := step.Epl == eceUniInner || step.Epl == eceUniLeft || step.Epl == eceUniFull
	if unidirectional {
		bookSource = bookSource.Unidirectional()
	}
	on := esper.OnSourcesEqual(0, esper.Field[esper.Event, string]("bookId"),
		1, esper.Field[esper.Event, string]("productId"))
	chain := esper.JoinChain(bookSource)
	var join esper.ChainedJoinStream
	switch step.Epl {
	case eceRowsInner, eceCntInner, eceUniInner:
		join = chain.InnerJoin(esper.JoinSource(itemStream), on)
	case eceRowsLeft, eceCntLeft, eceUniLeft:
		join = chain.LeftOuterJoin(esper.JoinSource(itemStream), on)
	default:
		join = chain.FullOuterJoin(esper.JoinSource(itemStream), on)
	}
	var query esper.Query
	if step.Epl == eceRowsInner || step.Epl == eceRowsLeft || step.Epl == eceRowsFull {
		selections := []esper.JoinSelection{
			esper.SelectFrom(0, "book.bookId", esper.Field[esper.Event, string]("bookId")),
			esper.SelectFrom(1, "item.itemId", esper.Field[esper.Event, string]("itemId")),
		}
		if step.Epl == eceRowsFull {
			// orderId is the item side's contained-select projection of the
			// parent MediaOrder field: present for matched and right-only
			// rows, null for left-only rows.
			selections = append(selections,
				esper.SelectFrom(1, "orderId", esper.ContainedParentField[string]("orderId")))
		}
		query = join.Select(selections...).Query(esper.StatementName("s0"),
			esper.OrderBy(esper.Ascending(esper.ResultField[any]("book.bookId")),
				esper.Ascending(esper.ResultField[any]("item.itemId"))))
	} else {
		query = join.Aggregate(esper.Alias("count(*)", esper.CountAll())).
			Query(esper.StatementName("s0"))
	}
	plan, err := env.Build(query)
	if err != nil {
		return err
	}
	return s.deployPlan(ctx, "s0", plan, "s0")
}

// buildFinancial deploys the ord 5 module's runtime pieces in Java module
// order: the SymbolsPair lastevent join insert, the split-all on-trigger
// with contained foreign/local branches and markers, the two contexted
// merges and the ordered 'out' select. Schemas, tables and the context are
// env-level registrations done in registerEPLContainedEventExampleSchemas.
func (s *eplContainedEventExampleCaseState) buildFinancial(ctx context.Context) error {
	env := s.env
	deploy := func(label string, query esper.Query, listenName string) error {
		plan, err := env.Build(query)
		if err != nil {
			return err
		}
		return s.deployPlan(ctx, label, plan, listenName)
	}
	// insert into SymbolsPair select * from ForeignSymbols#lastevent as
	// foreign, LocalSymbols#lastevent as local
	if err := deploy("symbols-pair", esper.JoinMany(
		esper.JoinSource(esper.From[map[string]any](env, "ForeignSymbols")).Window(esper.LastEvent()),
		esper.JoinSource(esper.From[map[string]any](env, "LocalSymbols")).Window(esper.LastEvent())).
		Select(
			esper.SelectSourceEvent(0, "foreign"),
			esper.SelectSourceEvent(1, "local"),
		).InsertInto("SymbolsPair", esper.StatementName("symbols-pair")), ""); err != nil {
		return err
	}
	// on SymbolsPair insert into Begin/ForeignSymbolRow/LocalSymbolRow/
	// Output/End output all — the contained branches unnest the join
	// event's foreign.companies / local.companies Symbol arrays.
	pair := esper.From[map[string]any](env, "SymbolsPair")
	foreignCompanies := esper.UnnestAs[map[string]any, map[string]any](pair,
		esper.Property[[]map[string]any](
			esper.Property[esper.Event](esper.EventValue[any](), "foreign"), "companies"), "Symbol")
	localCompanies := esper.UnnestAs[map[string]any, map[string]any](pair,
		esper.Property[[]map[string]any](
			esper.Property[esper.Event](esper.EventValue[any](), "local"), "companies"), "Symbol")
	if err := deploy("split", esper.OnEvent(pair).SplitAll(
		esper.SplitInto("SymbolsPairBeginEvent"),
		esper.SplitFrom(foreignCompanies).Into("ForeignSymbolRow"),
		esper.SplitFrom(localCompanies).Into("LocalSymbolRow"),
		esper.SplitInto("SymbolsPairOutputEvent"),
		esper.SplitInto("SymbolsPairEndEvent"),
	).Query(esper.StatementName("split")), ""); err != nil {
		return err
	}
	// context SymbolsPairContext on ForeignSymbolRow merge Result: the
	// where predicate maps to key expressions — foreignSymbol from the
	// trigger row, localSymbol through the scalar Mapping subselect.
	mappingLocalSymbol := esper.SubqueryValue[string](esper.FromTable(env, "Mapping"),
		esper.Field[any, string]("localSymbol"),
		esper.Equal[string](esper.Field[any, string]("foreignSymbol"), esper.OuterField[string]("symbol")))
	if err := deploy("merge-foreign", esper.OnEvent(esper.From[map[string]any](env, "ForeignSymbolRow")).
		MergeIntoTableWhen("Result",
			[]esper.Expr{
				esper.Field[any, string]("symbol"),
				mappingLocalSymbol,
			},
			esper.WhenNotMatchedAny(
				esper.SetColumn("foreignSymbol", esper.Field[any, string]("symbol")),
				esper.SetColumn("localSymbol", mappingLocalSymbol),
				esper.SetColumn("value", esper.Field[any, float64]("value")),
			),
			esper.WhenMatched(
				esper.Greater[float64](esper.Field[any, float64]("value"), esper.TableField[float64]("value")),
				esper.SetColumn("value", esper.Field[any, float64]("value")),
			),
		).Query(esper.StatementName("merge-foreign"), esper.WithContext("SymbolsPairContext")), ""); err != nil {
		return err
	}
	mappingForeignSymbol := esper.SubqueryValue[string](esper.FromTable(env, "Mapping"),
		esper.Field[any, string]("foreignSymbol"),
		esper.Equal[string](esper.Field[any, string]("localSymbol"), esper.OuterField[string]("symbol")))
	if err := deploy("merge-local", esper.OnEvent(esper.From[map[string]any](env, "LocalSymbolRow")).
		MergeIntoTableWhen("Result",
			[]esper.Expr{
				mappingForeignSymbol,
				esper.Field[any, string]("symbol"),
			},
			esper.WhenNotMatchedAny(
				esper.SetColumn("foreignSymbol", mappingForeignSymbol),
				esper.SetColumn("localSymbol", esper.Field[any, string]("symbol")),
				esper.SetColumn("value", esper.Field[any, float64]("value")),
			),
			esper.WhenMatched(
				esper.Greater[float64](esper.Field[any, float64]("value"), esper.TableField[float64]("value")),
				esper.SetColumn("value", esper.Field[any, float64]("value")),
			),
		).Query(esper.StatementName("merge-local"), esper.WithContext("SymbolsPairContext")), ""); err != nil {
		return err
	}
	// @name('out') context SymbolsPairContext on SymbolsPairOutputEvent
	// select foreignSymbol, localSymbol, value from Result order by
	// foreignSymbol asc — the traced statement.
	return deploy("module", esper.OnEvent(esper.From[map[string]any](env, "SymbolsPairOutputEvent")).
		SelectFromTableWhere("Result", nil,
			esper.Alias("foreignSymbol", esper.TableField[string]("foreignSymbol")),
			esper.Alias("localSymbol", esper.TableField[string]("localSymbol")),
			esper.Alias("value", esper.TableField[float64]("value")),
		).Query(esper.StatementName("out"), esper.WithContext("SymbolsPairContext"),
		esper.OrderBy(esper.Ascending(esper.ResultField[any]("foreignSymbol")))), "out")
}

// deployPlan deploys one plan and, when listen is non-empty, subscribes the
// listener that emits the normalized per-delivery records on the statement
// carrying that name.
func (s *eplContainedEventExampleCaseState) deployPlan(ctx context.Context, label string,
	plan esper.Plan, listen string) error {
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		if listen == "" || statement.Name() != listen {
			continue
		}
		stmt := statement
		if _, err := stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
			s.record(stmt.Name(), batch)
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// record emits one listener record per invocation with a per-statement
// sequence counter, mirroring the Java oracle's UpdateListener. Rows sort
// by their compact field rendering because the Java execution asserts the
// batches with assertPropsPerRowLastNew / assertPropsPerRowNewOnly.
func (s *eplContainedEventExampleCaseState) record(statement string, batch esper.ResultBatch) {
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
	sortRowsCanonical(rec.New)
	sortRowsCanonical(rec.Old)
	if len(rec.New) == 0 {
		rec.New = nil
	}
	if len(rec.Old) == 0 {
		rec.Old = nil
	}
	s.trace.Records = append(s.trace.Records, rec)
}

// undeployAll mirrors the Java execution's undeployAll: deployments retire
// in reverse deployment order; the env-scoped schema/table/context
// registrations retire with the engine.
func (s *eplContainedEventExampleCaseState) undeployAll(ctx context.Context) error {
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		deployment, ok := s.deployments[label]
		if !ok {
			continue
		}
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", eplContainedEventExampleID, label, err)
		}
	}
	s.deployments = make(map[string]*esper.Deployment)
	s.deployOrder = nil
	return nil
}

// send dispatches one pinned event: MediaOrder sends the inline XML through
// SendXML, SupportResponseEvent decodes the bean payload, and
// ForeignSymbols/LocalSymbols send map records.
func (s *eplContainedEventExampleCaseState) send(ctx context.Context, step compat.Step) error {
	switch step.EventType {
	case "MediaOrder":
		var payload struct {
			XML string `json:"xml"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode MediaOrder payload: %w", err)
		}
		return s.engine.SendXML(ctx, "MediaOrder", []byte(payload.XML))
	case "SupportResponseEvent":
		var event eceResponseEvent
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return fmt.Errorf("decode SupportResponseEvent payload: %w", err)
		}
		return s.engine.Send(ctx, "SupportResponseEvent", event)
	case "ForeignSymbols", "LocalSymbols":
		var payload struct {
			Companies []map[string]any `json:"companies"`
		}
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("decode %s payload: %w", step.EventType, err)
		}
		return s.engine.SendRecord(ctx, step.EventType, map[string]any{"companies": payload.Companies})
	default:
		return fmt.Errorf("%s: unsupported event type %q", eplContainedEventExampleID, step.EventType)
	}
}

// faf executes one prepared fire-and-forget Mapping load with positional
// parameters, mirroring the Java prepareQueryWithParameters + setObject +
// executeQuery flow.
func (s *eplContainedEventExampleCaseState) faf(ctx context.Context, step compat.Step) error {
	if step.Epl != eceFAFMapping {
		return fmt.Errorf("%s: faf %q in case %q carries an unpinned EPL %q",
			eplContainedEventExampleID, step.Statement, s.caseName, step.Epl)
	}
	var payload struct {
		ForeignSymbol string `json:"foreignSymbol"`
		LocalSymbol   string `json:"localSymbol"`
	}
	if err := json.Unmarshal(step.Payload, &payload); err != nil {
		return fmt.Errorf("decode faf payload: %w", err)
	}
	if s.preparedFAF == nil {
		plan, err := s.env.Build(esper.FromTable(s.env, "Mapping").OnDemand().InsertRows(
			esper.InsertValues(
				esper.PositionalParameter[string](1),
				esper.PositionalParameter[string](2))))
		if err != nil {
			return fmt.Errorf("%s: build faf insert: %w", eplContainedEventExampleID, err)
		}
		prepared, err := s.engine.PrepareFireAndForget(plan)
		if err != nil {
			return fmt.Errorf("%s: prepare faf insert: %w", eplContainedEventExampleID, err)
		}
		s.preparedFAF = prepared
	}
	if _, err := s.preparedFAF.ExecuteWithPositionalParameters(ctx, payload.ForeignSymbol, payload.LocalSymbol); err != nil {
		return fmt.Errorf("%s: execute faf insert: %w", eplContainedEventExampleID, err)
	}
	return nil
}

// loadEPLContainedEventExampleScenario decodes the scenario with the
// strict-shape checks the raw-mutation tests pin: no duplicate JSON keys,
// the exact top-level field set, pinned case metadata, and per-step field
// whitelists so unknown or duplicated step fields fail the replay.
func loadEPLContainedEventExampleScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", eplContainedEventExampleID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", eplContainedEventExampleID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplContainedEventExampleID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplContainedEventExampleID, err)
	}
	if err := requireEPLContainedEventExampleFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eplContainedEventExampleID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eplContainedEventExampleID ||
		metadata.Description != eplContainedEventExampleDescription ||
		metadata.JavaCommit != eplContainedEventExampleJavaCommit ||
		metadata.JavaSource != eplContainedEventExampleSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", eplContainedEventExampleID)
	}
	if err := validateEPLContainedEventExampleStringArray(root["javaRuntimes"], eplContainedEventExampleJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEPLContainedEventExampleStringArray(root["javaNames"], eplContainedEventExampleJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEPLContainedEventExampleStringArray(root["javaStaticIds"], eplContainedEventExampleJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateEPLContainedEventExampleStringArray(root["javaFlags"], eplContainedEventExampleJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(eplContainedEventExampleCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", eplContainedEventExampleID, len(eplContainedEventExampleCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEPLContainedEventExampleFields(object,
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
		if definition.Case != eplContainedEventExampleCases[index] ||
			definition.Ordinal != eplContainedEventExampleOrdinals[index] ||
			definition.RuntimeID != eplContainedEventExampleJavaRuntimeIDs[index] ||
			definition.ExecutionName != eplContainedEventExampleJavaExecutions[index] ||
			definition.Observation != eplContainedEventExampleCaseObservations[index] ||
			definition.EPL != eplContainedEventExampleCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", eplContainedEventExampleID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", eplContainedEventExampleID, err)
	}
	offset := 0
	for _, caseName := range eplContainedEventExampleCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", eplContainedEventExampleID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplContainedEventExampleID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", eplContainedEventExampleID, offset, caseName)
		}
		if _, err := eplContainedEventExampleStepKey(rawSteps[offset]); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplContainedEventExampleID, offset, err)
		}
		offset++
		want, ok := eplContainedEventExampleCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", eplContainedEventExampleID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", eplContainedEventExampleID, caseName)
		}
		for _, pinned := range want {
			key, err := eplContainedEventExampleStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", eplContainedEventExampleID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", eplContainedEventExampleID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", eplContainedEventExampleID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", eplContainedEventExampleID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// eplContainedEventExampleStepKey renders one raw step as its pinned key:
// op|statement|eventType|epl|listen|fields. Unknown fields on the step
// object are rejected; send payloads are restricted to the registered event
// type's asserted fields.
func eplContainedEventExampleStepKey(raw json.RawMessage) (string, error) {
	var object map[string]json.RawMessage
	if err := strictObject(raw, &object); err != nil {
		return "", err
	}
	var step struct {
		Op        string          `json:"op"`
		Case      string          `json:"case"`
		Statement string          `json:"statement"`
		EventType string          `json:"eventType"`
		Epl       string          `json:"epl"`
		Listen    string          `json:"listen"`
		Fields    []string        `json:"fields"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl", "fields", "listen"},
		"send":         {"op", "case", "eventType", "payload"},
		"faf":          {"op", "case", "statement", "epl", "payload"},
		"undeploy-all": {"op", "case"},
	}
	fields, ok := allowed[step.Op]
	if !ok {
		return "", fmt.Errorf("step has unsupported op %q", step.Op)
	}
	for field := range object {
		found := false
		for _, name := range fields {
			if field == name {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	if step.Op == "send" {
		payloadFields := map[string][]string{
			"MediaOrder":           {"xml"},
			"SupportResponseEvent": {"category", "subEvents"},
			"ForeignSymbols":       {"companies"},
			"LocalSymbols":         {"companies"},
		}
		allowedPayload, ok := payloadFields[step.EventType]
		if !ok {
			return "", fmt.Errorf("step has unknown event type %q", step.EventType)
		}
		var payload map[string]json.RawMessage
		if err := strictObject(step.Payload, &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		for field := range payload {
			found := false
			for _, name := range allowedPayload {
				if field == name {
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("send payload has unexpected field %q", field)
			}
		}
	}
	return step.Op + "|" + step.Statement + "|" + step.EventType + "|" + step.Epl +
		"|" + step.Listen + "|" + strings.Join(step.Fields, ","), nil
}

func requireEPLContainedEventExampleFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", eplContainedEventExampleID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", eplContainedEventExampleID, name)
		}
	}
	return nil
}

func validateEPLContainedEventExampleStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}

// normalizeEPLContainedEventExampleTrace sorts each case's listener records
// by statement name. Java's listener dispatch order within a single send is
// not deployment order (contained-example fires s3 before s1/s2); the
// canonical order is alphabetical by statement name, matching the evidence's
// stored order.
func normalizeEPLContainedEventExampleTrace(trace compat.Trace) compat.Trace {
	for start := 0; start < len(trace.Records); {
		end := start
		for end < len(trace.Records) && trace.Records[end].Case == trace.Records[start].Case {
			end++
		}
		run := trace.Records[start:end]
		sort.SliceStable(run, func(left, right int) bool {
			return run[left].Statement < run[right].Statement
		})
		start = end
	}
	return trace
}
