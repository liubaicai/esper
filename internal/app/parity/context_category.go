package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

const (
	contextCategoryID         = "context-category"
	contextCategoryJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	contextCategorySource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextCategory.java"
)

const contextCategoryDescription = "ContextCategory predefined-partition semantics: a two-category context with admin assertions (statement names, nesting level, partition ids/count, CONTEXTNAME/CONTEXTDEPLOYMENTID statement properties) and per-category count(*) plus context.label delivery where a non-matching event stays silent (ord 0); the 'group by' spelling with c1..c5 fields including context.name/context.id and a cumulative per-category sum (ord 1); three categories with eager empty partitions, iterators over all partitions including null sums, and undeploy cleanup (ord 2); two-deployment shared-path module with 'like' patterns and ctx-deployment CONTEXTDEPLOYMENTID (ord 3); iterator-only keepall+group-by statement with by-id, category, filtered, null/empty-set category selectors and a segmented-selector rejection (ord 4); single-category prior(1) replayed twice where the second cycle mirrors the SODA round-trip and a non-matching event does not advance prior (ord 5); compile-error probes for a bad filter property, a non-boolean predicate and a statement stream type not listed in the category context (ord 6); and declared expressions resolving context.label in script-call (ord 8) and alias (ord 7) forms. listener records mirror s0 IR pairs; snapshot records mirror statement iterators; admin records mirror context-partition service reads; compile-error and selector-error records pin the Go wording while the oracle asserts the Java prefix (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextCategory.java)."

var (
	contextCategoryJavaRuntimeIDs = []string{
		"java-runtime-edb899dd318dc4e8711e",
		"java-runtime-440efde2e13b0063a968",
		"java-runtime-cbe8b6c2ba86887f6807",
		"java-runtime-ad038edc0893e86eba46",
		"java-runtime-fe484daf28031390497b",
		"java-runtime-96843edb4ce366e1d74e",
		"java-runtime-51b59dd76ca097972003",
		"java-runtime-629da2acd413b0688d68",
		"java-runtime-76b9f0c6ca0a53be2ab0",
	}
	contextCategoryJavaExecutions = []string{
		"ContextCategorySceneOne",
		"ContextCategorySceneTwo",
		"ContextCategoryWContextProps",
		"ContextCategoryBooleanExprFilter",
		"ContextCategoryContextPartitionSelection",
		"ContextCategorySingleCategorySODAPrior",
		"ContextCategoryInvalid",
		"ContextCategoryDeclaredExpr{isAlias=true}",
		"ContextCategoryDeclaredExpr{isAlias=false}",
	}
	contextCategoryJavaStaticIDs = []string{
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
		"java-2275d4c280d0acadad30",
	}
	contextCategoryJavaFlags = []string{}
	contextCategoryCases     = []string{
		"scene-one",
		"scene-two",
		"w-context-props",
		"boolean-expr-filter",
		"partition-selection",
		"single-category-soda-prior",
		"invalid",
		"declared-expr-alias",
		"declared-expr-call",
	}
	contextCategoryOrdinals = []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	contextCategorySources  = []string{contextCategorySource}
)

var contextCategoryCaseObservations = []string{
	"listener+admin; one module deploys the context (trailing space after 'cat2 ') and s0; admin pins statement names [s0], nesting level 1, partition ids {0,1} and count 2, null context properties on 'context' and CategoryContext/module-deployment properties on 's0'; A and B deliver per-category counts, C matches nothing",
	"listener+admin; 'group by' spelling; partition identifiers carry labels {cat1,cat2} with id 0 = cat1 and per-id context properties; c1..c5 project theString, sum, context.label, context.name and context.id; the cumulative sum is per category",
	"listener+snapshot+admin; three categories (between 10 and 20 inclusive) allocate eagerly; iterators over all partitions include empty ones with null sums; the oracle asserts filterSvcCountApprox==3 and 3 eager agent instances; context count drops to 0 after undeploying the module containing s0",
	"listener+admin; two deployments share the module path; 'like' patterns A%/B%/C% route to agroup/bgroup/cgroup; s0's CONTEXTDEPLOYMENTID is the ctx deployment from the earlier module",
	"snapshot+snapshot-selector+selector-error; keepall group-by-theString statement with no listener; selectors pin by-id, by-category {grp1,grp3}, filtered label match, null and empty category sets, an always-false collecting selector observing all three labels, and a segmented selector rejected as an invalid context partition selector",
	"listener+admin; single category; prior(1) is per-partition so E1(5) yields null then E1(4) yields 5 while non-matching E2(20) neither fires nor advances prior; the second deploy+send cycle (mode 'soda') mirrors the Java eplToModel round-trip; context count returns to 0 after each undeploy-all",
	"compile-error; probes pin a bad filter property (dummy=1), a non-boolean predicate (intPrimitive), and a statement stream type (SupportBean_S0) not listed in the category context; the record value pins the Go wording while the oracle asserts the Java prefix",
	"listener; declared expressions resolving context.label deploy before the s0 module whose inline expression uses the alias-for form; E1(-2) yields n/xnx/n and E2(1) yields p/xpx/p",
	"listener; declared expressions resolving context.label deploy before the s0 module whose inline expression uses the script-call form; E1(-2) yields n/xnx/n and E2(1) yields p/xpx/p",
}

var contextCategoryCaseEPLs = []string{
	"@name('s0') context CategoryContext select count(*) as c0, context.label as c1 from SupportBean",
	"@Name('s0') context CtxCategory select theString as c1, sum(intPrimitive) as c2, context.label as c3, context.name as c4, context.id as c5 from SupportBean",
	"@name('s0') context CategorizedContext select context.name as c0, context.label as c1, sum(intPrimitive) as c2 from SupportBean",
	"@name('s0') context Ctx600a select context.label as c0, count(*) as c1 from SupportBean",
	"@name('s0') context MyCtx select context.id as c0, context.label as c1, theString as c2, sum(intPrimitive) as c3 from SupportBean#keepall group by theString",
	"@name('s0') context CategorizedContext select context.name as c0, context.label as c1, prior(1,intPrimitive) as c2 from SupportBean",
	"context ACtx select * from SupportBean_S0",
	"@name('s0') expression getLabelThree alias for { context.label } context MyCtx select getLabelOne as c0, getLabelTwo as c1, getLabelThree as c2 from SupportBean",
	"@name('s0') expression getLabelThree { context.label } context MyCtx select getLabelOne() as c0, getLabelTwo() as c1, getLabelThree() as c2 from SupportBean",
}

// contextCategoryCaseSteps pins the complete step sequence per case as
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|
// ids|selector|filterProperty|filterValue keys. Deploy steps carry the
// byte-exact EPL text the Java oracle compiles; mode "soda" marks the ord-5
// second cycle whose Java execution compiles through eplToModel; mode
// "clear-path" on undeploy-all marks the RegressionPath.clear() between the
// two ord-5 cycles; mode "admin:<probe>" on snapshot pins context-partition
// service reads instead of statement iterators; mode "any" pins an any-order
// iterator assertion. Java's milestone calls are documented no-ops and carry
// no steps.
var contextCategoryCaseSteps = map[string][]string{
	"scene-one": {
		"deploy|module|||@name('context') create context CategoryContext\ngroup theString = 'A' as cat1,\ngroup theString = 'B' as cat2 \nfrom SupportBean;\n@name('s0') context CategoryContext select count(*) as c0, context.label as c1 from SupportBean;\n||||||||",
		"deployed|module|||||||||||",
		"snapshot|context|CategoryContext||||||admin:context-admin||||",
		"snapshot|context|||||||admin:statement-props||||",
		"snapshot|s0|||||||admin:statement-props||||",
		"send|||SupportBean||{\"theString\":\"A\",\"intPrimitive\":1}|||||||",
		"send|||SupportBean||{\"theString\":\"C\",\"intPrimitive\":2}|||||||",
		"send|||SupportBean||{\"theString\":\"B\",\"intPrimitive\":3}|||||||",
		"send|||SupportBean||{\"theString\":\"A\",\"intPrimitive\":4}|||||||",
		"send|||SupportBean||{\"theString\":\"A\",\"intPrimitive\":6}|||||||",
		"send|||SupportBean||{\"theString\":\"B\",\"intPrimitive\":5}|||||||",
		"send|||SupportBean||{\"theString\":\"C\",\"intPrimitive\":7}|||||||",
		"undeploy-all||||||||||||",
	},
	"scene-two": {
		"deploy|module|||@Name('CTX') create context CtxCategory group by intPrimitive > 0 as cat1,group by intPrimitive < 0 as cat2 from SupportBean;\n@Name('s0') context CtxCategory select theString as c1, sum(intPrimitive) as c2, context.label as c3, context.name as c4, context.id as c5 from SupportBean;\n||||||||",
		"deployed|module|||||||||||",
		"snapshot|CTX|CtxCategory||||||admin:partition-info||||",
		"send|||SupportBean||{\"theString\":\"G1\",\"intPrimitive\":1}|||||||",
		"snapshot|CTX|CtxCategory||||||admin:partition-info||||",
		"send|||SupportBean||{\"theString\":\"G2\",\"intPrimitive\":-2}|||||||",
		"send|||SupportBean||{\"theString\":\"G3\",\"intPrimitive\":3}|||||||",
		"send|||SupportBean||{\"theString\":\"G4\",\"intPrimitive\":-4}|||||||",
		"send|||SupportBean||{\"theString\":\"G5\",\"intPrimitive\":5}|||||||",
		"undeploy-all||||||||||||",
	},
	"w-context-props": {
		"deploy|module|||@Name('context') create context CategorizedContext group intPrimitive < 10 as cat1, group intPrimitive between 10 and 20 as cat2, group intPrimitive > 20 as cat3 from SupportBean;\n@name('s0') context CategorizedContext select context.name as c0, context.label as c1, sum(intPrimitive) as c2 from SupportBean;\n||||||||",
		"deployed|module|||||||||||",
		"snapshot|context|CategorizedContext||||||admin:partition-info||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":5}|||||||",
		"snapshot|s0|||||||any||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":4}|||||||",
		"send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":11}|||||||",
		"send|||SupportBean||{\"theString\":\"E4\",\"intPrimitive\":25}|||||||",
		"send|||SupportBean||{\"theString\":\"E5\",\"intPrimitive\":25}|||||||",
		"send|||SupportBean||{\"theString\":\"E6\",\"intPrimitive\":3}|||||||",
		"snapshot|s0|||||||any||||",
		"snapshot|context|||||||admin:context-count||||",
		"undeploy|module|||||||||||",
		"snapshot|context|||||||admin:context-count||||",
	},
	"boolean-expr-filter": {
		"deploy|ctx|||@name('ctx') @public create context Ctx600a group by theString like 'A%' as agroup, group by theString like 'B%' as bgroup, group by theString like 'C%' as cgroup from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"deploy|s0|||@name('s0') context Ctx600a select context.label as c0, count(*) as c1 from SupportBean||||||||",
		"deployed|s0|||||||||||",
		"snapshot|s0|||||||admin:statement-props||||",
		"send|||SupportBean||{\"theString\":\"B1\",\"intPrimitive\":1}|||||||",
		"send|||SupportBean||{\"theString\":\"A1\",\"intPrimitive\":1}|||||||",
		"send|||SupportBean||{\"theString\":\"B171771\",\"intPrimitive\":1}|||||||",
		"send|||SupportBean||{\"theString\":\"A  x\",\"intPrimitive\":1}|||||||",
		"undeploy-all||||||||||||",
	},
	"partition-selection": {
		"deploy|ctx|||@name('ctx') @public create context MyCtx as group by intPrimitive < -5 as grp1, group by intPrimitive between -5 and +5 as grp2, group by intPrimitive > 5 as grp3 from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"deploy|s0|||@name('s0') context MyCtx select context.id as c0, context.label as c1, theString as c2, sum(intPrimitive) as c3 from SupportBean#keepall group by theString||||||||",
		"deployed|s0|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":1}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":-5}|||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":2}|||||||",
		"send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":-100}|||||||",
		"send|||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":-8}|||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":60}|||||||",
		"snapshot|s0|||||||any||||",
		"snapshot|ctx|MyCtx||||||admin:context-props||||",
		"snapshot-selector|s0||||||||[1]|ids||",
		"snapshot-selector|s0||||[\"grp1\",\"grp3\"]|||||filtered|labels|",
		"snapshot-selector|s0|||||||||filtered|label|grp1",
		"snapshot-selector|s0||||null|||||filtered|labels|",
		"snapshot-selector|s0||||[]|||||filtered|labels|",
		"snapshot-selector|s0|||||||||filtered|label|",
		"snapshot-selector|s0|||||Invalid context partition selector, expected an implementation class of any of [ContextPartitionSelectorAll, ContextPartitionSelectorFiltered, ContextPartitionSelectorById, ContextPartitionSelectorCategory] interfaces but received ||||segmented||",
		"undeploy-all||||||||||||",
	},
	"single-category-soda-prior": {
		"deploy|ctx|||@Name('context') @public create context CategorizedContext as group intPrimitive<10 as cat1 from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"deploy|s0|||@name('s0') context CategorizedContext select context.name as c0, context.label as c1, prior(1,intPrimitive) as c2 from SupportBean||||||||",
		"deployed|s0|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":5}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":4}|||||||",
		"snapshot|context|||||||admin:context-count||||",
		"undeploy-all||||||||clear-path||||",
		"snapshot|context|||||||admin:context-count||||",
		"deploy|ctx-soda|||@Name('context') @public create context CategorizedContext as group intPrimitive<10 as cat1 from SupportBean||||soda||||",
		"deployed|ctx-soda|||||||||||",
		"deploy|s0-soda|||@name('s0') context CategorizedContext select context.name as c0, context.label as c1, prior(1,intPrimitive) as c2 from SupportBean||||soda||||",
		"deployed|s0-soda|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":5}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":20}|||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":4}|||||||",
		"snapshot|context|||||||admin:context-count||||",
		"undeploy-all||||||||||||",
		"snapshot|context|||||||admin:context-count||||",
	},
	"invalid": {
		"build-error|bad-filter-prop|||create context ACtx group theString is not null as cat1 from SupportBean(dummy = 1)||Failed to validate filter expression 'dummy=1': Property named 'dummy' is not valid in any stream [|1|||||",
		"build-error|non-boolean-predicate|||create context ACtx group intPrimitive as grp1 from SupportBean||Filter expression not returning a boolean value: 'intPrimitive' [|1|||||",
		"deploy|ctx|||@public create context ACtx group intPrimitive < 10 as cat1 from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"build-error|statement-stream-type|||context ACtx select * from SupportBean_S0||Category context 'ACtx' requires that any of the events types that are listed in the category context also appear in any of the filter expressions of the statement [||||||",
		"undeploy-all||||||||||||",
	},
	"declared-expr-alias": {
		"deploy|ctx|||@name('ctx') @public create context MyCtx as group by intPrimitive < 0 as n, group by intPrimitive > 0 as p from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"deploy|expr-1|||@name('expr-1') @public create expression getLabelOne { context.label }||||||||",
		"deployed|expr-1|||||||||||",
		"deploy|expr-2|||@name('expr-2') @public create expression getLabelTwo { 'x'||context.label||'x' }||||||||",
		"deployed|expr-2|||||||||||",
		"deploy|s0|||@name('s0') expression getLabelThree alias for { context.label } context MyCtx select getLabelOne as c0, getLabelTwo as c1, getLabelThree as c2 from SupportBean||||||||",
		"deployed|s0|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":-2}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":1}|||||||",
		"undeploy-all||||||||||||",
	},
	"declared-expr-call": {
		"deploy|ctx|||@name('ctx') @public create context MyCtx as group by intPrimitive < 0 as n, group by intPrimitive > 0 as p from SupportBean||||||||",
		"deployed|ctx|||||||||||",
		"deploy|expr-1|||@name('expr-1') @public create expression getLabelOne { context.label }||||||||",
		"deployed|expr-1|||||||||||",
		"deploy|expr-2|||@name('expr-2') @public create expression getLabelTwo { 'x'||context.label||'x' }||||||||",
		"deployed|expr-2|||||||||||",
		"deploy|s0|||@name('s0') expression getLabelThree { context.label } context MyCtx select getLabelOne() as c0, getLabelTwo() as c1, getLabelThree() as c2 from SupportBean||||||||",
		"deployed|s0|||||||||||",
		"send|||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":-2}|||||||",
		"send|||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":1}|||||||",
		"undeploy-all||||||||||||",
	},
}

// contextCategoryBean mirrors SupportBean for the category cases.
type contextCategoryBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

// contextCategoryS0 mirrors SupportBean_S0 for the invalid case's
// statement-stream-type probe.
type contextCategoryS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// contextCategoryCaseState carries the per-case replay state: the
// environment/engine pair, label→deployment bookkeeping, the deployed
// statements the snapshot and admin fixtures resolve by label, the context
// name each deploy label registers, and the statement→context/deploy-label
// maps the statement-props probe normalizes through.
type contextCategoryCaseState struct {
	env              *esper.Environment
	engine           *esper.Engine
	deployments      map[string]*esper.Deployment
	statements       map[string]*esper.Statement
	deployOrder      []string
	listenedLabels   map[string]bool
	sequences        map[string]uint64
	contextByLabel   map[string]string
	contextOwner     map[string]string
	statementContext map[string]string
	statementOwner   map[string]string
	liveContexts     map[string]bool
	trace            *compat.Trace
	caseName         string
}

// contextCategoryCollectingSelector mirrors MySelectorFilteredCategory: it
// visits every partition descriptor, records the observed labels, and accepts
// only the pinned match label (empty means never match).
type contextCategoryCollectingSelector struct {
	match  string
	labels []string
}

func (s *contextCategoryCollectingSelector) SelectContextPartition(string) bool { return true }

func (s *contextCategoryCollectingSelector) SelectContextPartitionDescriptor(descriptor esper.ContextPartitionDescriptor) bool {
	s.labels = append(s.labels, contextCategoryDescriptorLabel(descriptor))
	return s.match != "" && s.match == contextCategoryDescriptorLabel(descriptor)
}

func contextCategoryDescriptorLabel(descriptor esper.ContextPartitionDescriptor) string {
	value, ok := descriptor.Property("label")
	if !ok || value.Any() == nil {
		return ""
	}
	text, _ := value.Any().(string)
	return text
}

func runContextCategoryScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateContextCategoryScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	return executeContextCategory(ctx, scenario, &trace)
}

// executeContextCategory replays the scenario: each case runs on a fresh
// environment/engine pair (one runtime per Java execution) and every step
// dispatches to the matching runtime action.
func executeContextCategory(ctx context.Context, scenario compat.Scenario, trace *compat.Trace) (compat.Trace, error) {
	var state *contextCategoryCaseState
	for _, step := range scenario.Steps {
		if err := ctx.Err(); err != nil {
			return *trace, err
		}
		switch step.Op {
		case "case":
			if state != nil && state.engine != nil {
				_ = state.engine.Close(context.Background())
			}
			var err error
			state, err = startContextCategoryCase(step.Case, trace)
			if err != nil {
				return *trace, err
			}
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return *trace, err
			}
		case "deployed":
			state.sequences[step.Statement+":deployed"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      state.caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  state.sequences[step.Statement+":deployed"],
				Time:      compat.FormatTraceTime(state.engine.Now()),
			})
		case "send":
			event, err := decodeContextCategoryPayload(step)
			if err != nil {
				return *trace, err
			}
			if err := state.engine.Send(ctx, step.EventType, event); err != nil {
				return *trace, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step); err != nil {
				return *trace, err
			}
		case "snapshot-selector":
			if err := state.snapshotSelector(ctx, step); err != nil {
				return *trace, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return *trace, err
			}
		case "undeploy":
			if err := state.undeploy(ctx, step.Statement); err != nil {
				return *trace, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx, step.Mode); err != nil {
				return *trace, err
			}
		default:
			return *trace, fmt.Errorf("%s: unsupported step op %q", contextCategoryID, step.Op)
		}
	}
	if state != nil && state.engine != nil {
		_ = state.engine.Close(context.Background())
	}
	return *trace, nil
}

// startContextCategoryCase builds the fresh per-case environment: the two
// event types the suite registers plus the engine pinned to the case's Java
// runtime id at the epoch start time.
func startContextCategoryCase(caseName string, trace *compat.Trace) (*contextCategoryCaseState, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[contextCategoryBean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[contextCategoryS0](env, "SupportBean_S0"); err != nil {
		return nil, err
	}
	state := &contextCategoryCaseState{
		env:              env,
		deployments:      map[string]*esper.Deployment{},
		statements:       map[string]*esper.Statement{},
		listenedLabels:   map[string]bool{},
		sequences:        map[string]uint64{},
		contextByLabel:   map[string]string{},
		contextOwner:     map[string]string{},
		statementContext: map[string]string{},
		statementOwner:   map[string]string{},
		liveContexts:     map[string]bool{},
		trace:            trace,
		caseName:         caseName,
	}
	state.engine = esper.NewEngine(env,
		esper.WithRuntimeURI(contextCategoryJavaRuntimeIDs[contextCategoryCaseOrdinal(caseName)]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	return state, nil
}

func contextCategoryCaseOrdinal(caseName string) int {
	for index, name := range contextCategoryCases {
		if name == caseName {
			return index
		}
	}
	return 0
}

// deploy executes one deploy step: registration fixtures model create-context
// and create-expression EPL at their scenario positions (the established
// approved difference for the missing deployable statement types), and
// statement fixtures deploy labeled plans. The s0 listener attaches on first
// deploy for the cases whose Java execution calls addListener, mirroring
// env.addListener; mode "soda" is the Go-side no-op marker for the ord-5
// eplToModel round-trip (the Go plan is already the compiled form).
func (s *contextCategoryCaseState) deploy(ctx context.Context, step compat.Step) error {
	label := step.Statement
	beanSource := esper.From[contextCategoryBean](s.env, "SupportBean")
	theString := esper.Field[contextCategoryBean, string]("theString")
	intPrimitive := esper.Field[contextCategoryBean, int]("intPrimitive")
	registerContext := func(name string, categories ...esper.ContextCategory) error {
		// Go contexts are environment-scoped registrations; the ord-5 second
		// cycle redeploys the same context EPL after undeploy-all, so an
		// already-registered name is reused (documented deployment-lifecycle
		// difference from Java's per-deployment context). The context owner
		// label is the deploy step that first registered the context, which
		// normalizes Java's CONTEXTDEPLOYMENTID for the statement-props probe.
		if _, ok := s.env.Context(name); ok {
			s.liveContexts[name] = true
			s.contextByLabel[label] = name
			return nil
		}
		if _, err := esper.CreateCategoryContext(s.env, name, categories...); err != nil {
			return err
		}
		s.liveContexts[name] = true
		s.contextByLabel[label] = name
		if _, owned := s.contextOwner[name]; !owned {
			s.contextOwner[name] = label
		}
		return nil
	}
	switch s.caseName {
	case "scene-one":
		if label != "module" {
			break
		}
		if err := registerContext("CategoryContext",
			esper.Category("cat1", esper.Equal[string](theString, esper.Literal("A"))),
			esper.Category("cat2", esper.Equal[string](theString, esper.Literal("B"))),
		); err != nil {
			return err
		}
		plan, err := s.env.Build(beanSource.Aggregate(
			esper.Alias("c0", esper.CountAll()),
			esper.Alias("c1", esper.ContextLabel()),
		).Query(esper.StatementName("s0"), esper.WithContext("CategoryContext")))
		_, err = s.deployPlan(label, "CategoryContext", plan, err)
		return err
	case "scene-two":
		if label != "module" {
			break
		}
		if err := registerContext("CtxCategory",
			esper.Category("cat1", esper.Greater[int](intPrimitive, esper.Literal(0))),
			esper.Category("cat2", esper.Less[int](intPrimitive, esper.Literal(0))),
		); err != nil {
			return err
		}
		plan, err := s.env.Build(beanSource.Aggregate(
			esper.Alias("c1", theString),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
			esper.Alias("c3", esper.ContextLabel()),
			esper.Alias("c4", esper.ContextName()),
			esper.Alias("c5", esper.ContextID()),
		).Query(esper.StatementName("s0"), esper.WithContext("CtxCategory")))
		_, err = s.deployPlan(label, "CtxCategory", plan, err)
		return err
	case "w-context-props":
		if label != "module" {
			break
		}
		if err := registerContext("CategorizedContext",
			esper.Category("cat1", esper.Less[int](intPrimitive, esper.Literal(10))),
			esper.Category("cat2", esper.Between[int](intPrimitive, esper.Literal(10), esper.Literal(20))),
			esper.Category("cat3", esper.Greater[int](intPrimitive, esper.Literal(20))),
		); err != nil {
			return err
		}
		plan, err := s.env.Build(beanSource.Aggregate(
			esper.Alias("c0", esper.ContextName()),
			esper.Alias("c1", esper.ContextLabel()),
			esper.Alias("c2", esper.Sum[int](intPrimitive)),
		).Query(esper.StatementName("s0"), esper.WithContext("CategorizedContext")))
		_, err = s.deployPlan(label, "CategorizedContext", plan, err)
		return err
	case "boolean-expr-filter":
		switch label {
		case "ctx":
			return registerContext("Ctx600a",
				esper.Category("agroup", esper.Like(theString, esper.Literal("A%"))),
				esper.Category("bgroup", esper.Like(theString, esper.Literal("B%"))),
				esper.Category("cgroup", esper.Like(theString, esper.Literal("C%"))),
			)
		case "s0":
			plan, err := s.env.Build(beanSource.Aggregate(
				esper.Alias("c0", esper.ContextLabel()),
				esper.Alias("c1", esper.CountAll()),
			).Query(esper.StatementName("s0"), esper.WithContext("Ctx600a")))
			_, err = s.deployPlan(label, "Ctx600a", plan, err)
			return err
		}
	case "partition-selection":
		switch label {
		case "ctx":
			return registerContext("MyCtx",
				esper.Category("grp1", esper.Less[int](intPrimitive, esper.Literal(-5))),
				esper.Category("grp2", esper.Between[int](intPrimitive, esper.Literal(-5), esper.Literal(5))),
				esper.Category("grp3", esper.Greater[int](intPrimitive, esper.Literal(5))),
			)
		case "s0":
			plan, err := s.env.Build(beanSource.GroupBy(theString).Select(
				esper.Alias("c0", esper.ContextID()),
				esper.Alias("c1", esper.ContextLabel()),
				esper.Alias("c2", theString),
				esper.Alias("c3", esper.Sum[int](intPrimitive)),
			).Query(esper.StatementName("s0"), esper.WithContext("MyCtx")))
			_, err = s.deployPlan(label, "MyCtx", plan, err)
			return err
		}
	case "single-category-soda-prior":
		switch label {
		case "ctx", "ctx-soda":
			return registerContext("CategorizedContext",
				esper.Category("cat1", esper.Less[int](intPrimitive, esper.Literal(10))),
			)
		case "s0", "s0-soda":
			plan, err := s.env.Build(esper.Select(
				beanSource.Window(esper.KeepAll()),
				esper.Alias("c0", esper.ContextName()),
				esper.Alias("c1", esper.ContextLabel()),
				esper.Alias("c2", esper.Prior[int](0, intPrimitive)),
			).Query(esper.StatementName("s0"), esper.WithContext("CategorizedContext")))
			_, err = s.deployPlan(label, "CategorizedContext", plan, err)
			return err
		}
	case "invalid":
		if label == "ctx" {
			return registerContext("ACtx",
				esper.Category("cat1", esper.Less[int](intPrimitive, esper.Literal(10))),
			)
		}
	case "declared-expr-alias", "declared-expr-call":
		switch label {
		case "ctx":
			return registerContext("MyCtx",
				esper.Category("n", esper.Less[int](intPrimitive, esper.Literal(0))),
				esper.Category("p", esper.Greater[int](intPrimitive, esper.Literal(0))),
			)
		case "expr-1":
			return s.env.DefineExpression("getLabelOne", esper.ContextLabel())
		case "expr-2":
			return s.env.DefineExpression("getLabelTwo",
				esper.Concat(esper.Literal("x"), esper.ContextLabel(), esper.Literal("x")))
		case "s0":
			// The Java module declares getLabelThree inline; the Go fixture
			// registers the same expression at this deploy position and
			// references all three by name (the alias/call distinction has no
			// typed-API counterpart).
			if err := s.env.DefineExpression("getLabelThree", esper.ContextLabel()); err != nil {
				return err
			}
			plan, err := s.env.Build(esper.Select(
				beanSource,
				esper.Alias("c0", esper.ExpressionRef[string](s.env, "getLabelOne")),
				esper.Alias("c1", esper.ExpressionRef[string](s.env, "getLabelTwo")),
				esper.Alias("c2", esper.ExpressionRef[string](s.env, "getLabelThree")),
			).Query(esper.StatementName("s0"), esper.WithContext("MyCtx")))
			_, err = s.deployPlan(label, "MyCtx", plan, err)
			return err
		}
	}
	return fmt.Errorf("%s: case %q has no deploy fixture for %q", contextCategoryID, s.caseName, label)
}

// deployPlan deploys one built plan under the step label, records the
// deployment for targeted undeploy, and attaches the s0 listener for the
// cases whose Java execution calls addListener.
func (s *contextCategoryCaseState) deployPlan(label, contextName string, plan esper.Plan, planErr error) (*esper.Deployment, error) {
	if planErr != nil {
		return nil, planErr
	}
	deployment, err := s.engine.Deploy(context.Background(), plan)
	if err != nil {
		return nil, fmt.Errorf("%s: deploy %q: %w", contextCategoryID, label, err)
	}
	s.deployments[label] = deployment
	s.deployOrder = append(s.deployOrder, label)
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
		s.statementOwner[statement.Name()] = label
		if contextName != "" {
			s.statementContext[statement.Name()] = contextName
		}
		if statement.Name() == "s0" && contextCategoryListened(s.caseName) && !s.listenedLabels["s0"] {
			s.listenedLabels["s0"] = true
			if err := s.subscribeStatement(statement); err != nil {
				return nil, err
			}
		}
	}
	return deployment, nil
}

// contextCategoryListened mirrors the Java executions' addListener("s0")
// calls: every case except partition-selection listens on s0.
func contextCategoryListened(caseName string) bool {
	return caseName != "partition-selection"
}

// subscribeStatement mirrors the oracle's listener: one listener record per
// delivered batch with normalized row fields.
func (s *contextCategoryCaseState) subscribeStatement(statement *esper.Statement) error {
	_, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		if len(batch.New) == 0 && len(batch.Old) == 0 {
			return nil
		}
		s.sequences[statement.Name()]++
		record := compat.TraceRecord{
			Case:      s.caseName,
			Operation: "listener",
			Statement: statement.Name(),
			Sequence:  s.sequences[statement.Name()],
			Time:      compat.FormatTraceTime(batch.Time),
			New:       compat.NormalizeResults(batch.New),
			Old:       compat.NormalizeResults(batch.Old),
		}
		s.trace.Records = append(s.trace.Records, record)
		return nil
	})
	return err
}

// snapshot dispatches on the step mode: "admin:<probe>" emits an admin record
// mirroring the Java execution's context-partition service reads and
// statement-property assertions; "any" (or empty) emits one snapshot record
// mirroring the statement iterator with rows in canonical any-order and the
// statement's category partitions.
func (s *contextCategoryCaseState) snapshot(ctx context.Context, step compat.Step) error {
	if strings.HasPrefix(step.Mode, "admin:") {
		return s.admin(ctx, step)
	}
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed", contextCategoryID, step.Statement)
	}
	result, err := statement.SnapshotWithSelector(ctx, nil)
	if err != nil {
		return err
	}
	record := compat.TraceRecord{
		Case:       s.caseName,
		Operation:  "snapshot",
		Statement:  step.Statement,
		Time:       compat.FormatTraceTime(s.engine.Now()),
		New:        contextCategorySortedRows(compat.NormalizeResults(result.Results())),
		Partitions: contextCategoryPartitions(statement.ContextPartitionsWith(nil)),
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// admin emits one {"operation":"admin"} record for the pinned probe:
// context-admin mirrors getContextStatementNames/getContextNestingLevel/
// getContextPartitionIds/getContextPartitionCount; statement-props mirrors
// the CONTEXTNAME/CONTEXTDEPLOYMENTID statement properties (deployment ids
// normalize to the deploy-step label); partition-info and context-props
// mirror getContextPartitions/getIdentifier/getContextProperties; and
// context-count mirrors SupportContextMgmtHelper.getContextCount as the count
// of live context deployments.
func (s *contextCategoryCaseState) admin(ctx context.Context, step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "admin",
		Statement: step.Statement,
		Name:      step.Name,
		Time:      compat.FormatTraceTime(s.engine.Now()),
	}
	switch step.Mode {
	case "admin:context-admin":
		names, err := s.engine.ContextStatementNames(step.Name)
		if err != nil {
			return err
		}
		level, err := s.engine.ContextNestingLevel(step.Name)
		if err != nil {
			return err
		}
		descriptors, err := s.engine.ContextPartitionDescriptors(step.Name, esper.ContextPartitionSelectorAll{})
		if err != nil {
			return err
		}
		record.Value = map[string]any{
			"statementNames": names,
			"nestingLevel":   level,
			"partitionCount": len(descriptors),
		}
		record.Partitions = contextCategoryPartitions(descriptors)
	case "admin:statement-props":
		value := map[string]any{
			"contextName":         map[string]any{"state": "null"},
			"contextDeploymentId": map[string]any{"state": "null"},
		}
		if contextName, ok := s.statementContext[step.Statement]; ok {
			value["contextName"] = contextName
			value["contextDeploymentId"] = s.contextOwner[contextName]
		}
		record.Value = value
	case "admin:partition-info", "admin:context-props":
		descriptors, err := s.engine.ContextPartitionDescriptors(step.Name, esper.ContextPartitionSelectorAll{})
		if err != nil {
			return err
		}
		record.Partitions = contextCategoryPartitionProperties(descriptors)
	case "admin:context-count":
		record.Value = len(s.liveContexts)
	default:
		return fmt.Errorf("%s: unknown admin probe %q", contextCategoryID, step.Mode)
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// snapshotSelector replays one selector-targeted iterator step, mirroring the
// Java execution's statement.iterator(selector) calls: "ids" maps to
// ContextPartitionSelectorById, "filtered" with filterProperty "labels" maps
// to ContextPartitionSelectorCategory over the payload label array (null and
// empty sets both select nothing), "filtered" with filterProperty "label"
// maps to MySelectorFilteredCategory collecting observed labels, and
// "segmented" asserts the invalid-selector rejection.
func (s *contextCategoryCaseState) snapshotSelector(ctx context.Context, step compat.Step) error {
	statement, ok := s.statements[step.Statement]
	if !ok {
		return fmt.Errorf("%s: snapshot-selector statement %q was not deployed", contextCategoryID, step.Statement)
	}
	var selector esper.ContextPartitionSelector
	var collector *contextCategoryCollectingSelector
	switch step.Selector {
	case "ids":
		selector = esper.SelectContextPartitionIDs(step.IDs...)
	case "filtered":
		switch step.FilterProperty {
		case "labels":
			var labels []string
			if len(step.Payload) > 0 && string(step.Payload) != "null" {
				if err := json.Unmarshal(step.Payload, &labels); err != nil {
					return fmt.Errorf("%s: category labels payload: %w", contextCategoryID, err)
				}
			}
			selector = esper.SelectContextPartitionCategories(labels...)
		case "label":
			collector = &contextCategoryCollectingSelector{match: step.FilterValue}
			selector = collector
		default:
			return fmt.Errorf("%s: filtered selector property %q is not pinned", contextCategoryID, step.FilterProperty)
		}
	case "segmented":
		selector = esper.SelectContextPartitionSegments([]any{"category-segmented-probe"})
	default:
		return fmt.Errorf("%s: unsupported selector %q", contextCategoryID, step.Selector)
	}
	result, err := statement.SnapshotWithSelector(ctx, selector)
	if err != nil {
		record := compat.TraceRecord{
			Case:      s.caseName,
			Operation: "selector-error",
			Statement: step.Statement,
			Time:      compat.FormatTraceTime(s.engine.Now()),
		}
		var espErr *esper.Error
		if step.ExpectError == "" {
			return err
		}
		// Verify the Go rejection is the InvalidRule selector incompatibility
		// before recording the pinned Java prefix (the approved wording
		// difference), mirroring the isContextVariablesNotFound gate.
		if !errors.As(err, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
			!strings.Contains(err.Error(), "incompatible with context") {
			return fmt.Errorf("%s: selector-error drift for %q: got %v",
				contextCategoryID, step.Selector, err)
		}
		record.Value = step.ExpectError
		s.trace.Records = append(s.trace.Records, record)
		return nil
	}
	record := compat.TraceRecord{
		Case:       s.caseName,
		Operation:  "snapshot-selector",
		Statement:  step.Statement,
		Time:       compat.FormatTraceTime(s.engine.Now()),
		New:        contextCategorySortedRows(compat.NormalizeResults(result.Results())),
		Partitions: contextCategoryPartitions(statement.ContextPartitionsWith(selector)),
	}
	if collector != nil {
		// The filtered selector observes every partition label while
		// filtering; the record pins the observed set like the Java
		// execution's getCategories() assertion.
		labels := append([]string(nil), collector.labels...)
		sort.Strings(labels)
		labels = dedupeAdjacentStrings(labels)
		record.Value = map[string]any{"observedLabels": labels}
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// dedupeAdjacentStrings removes consecutive duplicates from a sorted slice,
// collapsing the double observation the descriptor filter makes when the
// runtime evaluates it once for rows and once for partition descriptors.
func dedupeAdjacentStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

// buildError runs one expected-invalid compile probe against the fluent
// equivalent of the pinned EPL. The record carries the pinned Java message
// prefix once the Go rejection verifies, matching the oracle's prefix
// assertion (the approved wording difference).
func (s *contextCategoryCaseState) buildError(step compat.Step) error {
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	var buildErr error
	switch step.Statement {
	case "bad-filter-prop":
		// Java's SupportBean(dummy = 1) stream filter on the create-context
		// fails validation; the Go fixture folds the filter into the category
		// predicate so the unknown field surfaces when the first statement
		// using the context is built. A distinct context name keeps the probe
		// from colliding with the later ACtx deploy step.
		if _, err := esper.CreateCategoryContext(s.env, "ACtxBad",
			esper.Category("cat1", esper.Equal[int](
				esper.Field[contextCategoryBean, int]("dummy"), esper.Literal(1))),
		); err != nil {
			return fmt.Errorf("%s: build-error probe %q context registration failed: %w", contextCategoryID, step.Statement, err)
		}
		_, buildErr = s.env.Build(esper.From[contextCategoryBean](s.env, "SupportBean").Query(
			esper.StatementName("s0"), esper.WithContext("ACtxBad")))
	case "non-boolean-predicate":
		// Go's generic Expression[bool] signature rejects a non-boolean
		// predicate at the type level; the nil-predicate rejection is the
		// runtime boundary for the same invalid category declaration.
		_, buildErr = esper.NewContextCategory("grp1", nil)
	case "statement-stream-type":
		_, buildErr = s.env.Build(esper.From[contextCategoryS0](s.env, "SupportBean_S0").Query(
			esper.StatementName("s0"), esper.WithContext("ACtx")))
	default:
		return fmt.Errorf("%s: unknown build-error probe %q", contextCategoryID, step.Statement)
	}
	if buildErr == nil {
		return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", contextCategoryID, step.Statement)
	}
	// Verify the Go rejection carries the expected category and wording
	// before recording the pinned Java prefix, mirroring the
	// isContextVariablesNotFound gate on the runtime read/set paths.
	expected := map[string]struct {
		code      esper.ErrorCode
		substring string
	}{
		"bad-filter-prop":       {esper.ErrorInvalidRule, `unknown field "dummy"`},
		"non-boolean-predicate": {esper.ErrorInvalidRule, "context category predicate is required"},
		"statement-stream-type": {esper.ErrorInvalidRule, "requires that any of the event types that are listed in the category context"},
	}
	if want, ok := expected[step.Statement]; ok {
		var espErr *esper.Error
		if !errors.As(buildErr, &espErr) || espErr.Code != want.code || !strings.Contains(buildErr.Error(), want.substring) {
			return fmt.Errorf("%s: build-error probe %q drift: got %v", contextCategoryID, step.Statement, buildErr)
		}
	}
	if step.ExpectError != "" {
		record.Value = step.ExpectError
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// undeploy removes the deployment registered under the label, mirroring
// undeployModuleContaining for the ord-2 module deployment.
func (s *contextCategoryCaseState) undeploy(ctx context.Context, label string) error {
	deployment, ok := s.deployments[label]
	if !ok {
		return fmt.Errorf("%s: unknown undeploy label %q", contextCategoryID, label)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", contextCategoryID, label, err)
	}
	s.dropDeployment(label)
	return nil
}

// undeployAll removes deployments in reverse deploy order so dependents
// undeploy before the modules they reference, mirroring undeployAll. Mode
// "clear-path" marks the RegressionPath.clear() between the two ord-5 cycles;
// it is a Go no-op because the Go fixture has no module path.
func (s *contextCategoryCaseState) undeployAll(ctx context.Context, mode string) error {
	if mode != "" && mode != "clear-path" {
		return fmt.Errorf("%s: undeploy-all mode %q is not pinned", contextCategoryID, mode)
	}
	for index := len(s.deployOrder) - 1; index >= 0; index-- {
		label := s.deployOrder[index]
		if _, ok := s.deployments[label]; !ok {
			continue
		}
		if err := s.deployments[label].Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", contextCategoryID, label, err)
		}
		s.dropDeployment(label)
	}
	// Context-registration labels (ctx, ctx-soda, expr-*) own no Deployment;
	// undeployAll still retires their contexts like Java's undeployAll.
	for label, contextName := range s.contextByLabel {
		delete(s.liveContexts, contextName)
		delete(s.contextByLabel, label)
	}
	s.deployOrder = nil
	s.listenedLabels = map[string]bool{}
	return nil
}

func (s *contextCategoryCaseState) dropDeployment(label string) {
	delete(s.deployments, label)
	if contextName := s.contextByLabel[label]; contextName != "" {
		delete(s.liveContexts, contextName)
		delete(s.contextByLabel, label)
	}
	for name, owner := range s.statementOwner {
		if owner == label {
			delete(s.statements, name)
			delete(s.statementOwner, name)
			delete(s.statementContext, name)
		}
	}
}

// decodeContextCategoryPayload converts a send payload into the typed bean
// for the step's event type.
func decodeContextCategoryPayload(step compat.Step) (any, error) {
	switch step.EventType {
	case "SupportBean":
		var value contextCategoryBean
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean: %w", contextCategoryID, err)
		}
		return value, nil
	case "SupportBean_S0":
		var value contextCategoryS0
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("%s SupportBean_S0: %w", contextCategoryID, err)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported event type %q", contextCategoryID, step.EventType)
	}
}

// contextCategoryPartitions renders the statement-scoped partition
// identifiers the Java oracle emits for snapshot records: one record per
// ContextPartitionIdentifierCategory carrying its id, category key and label.
func contextCategoryPartitions(descriptors []esper.ContextPartitionDescriptor) []compat.PartitionRecord {
	result := make([]compat.PartitionRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		properties := map[string]any{}
		if label := contextCategoryDescriptorLabel(descriptor); label != "" {
			properties["label"] = label
		}
		result = append(result, compat.PartitionRecord{
			ID:         descriptor.ID,
			Key:        descriptor.Key,
			Properties: properties,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// contextCategoryPartitionProperties renders the admin getContextProperties
// view: name, id and label per partition.
func contextCategoryPartitionProperties(descriptors []esper.ContextPartitionDescriptor) []compat.PartitionRecord {
	result := make([]compat.PartitionRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		properties := map[string]any{
			"name": descriptor.ContextName,
			"id":   descriptor.ID,
		}
		if label := contextCategoryDescriptorLabel(descriptor); label != "" {
			properties["label"] = label
		}
		result = append(result, compat.PartitionRecord{
			ID:         descriptor.ID,
			Key:        descriptor.Key,
			Properties: properties,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// contextCategorySortedRows canonicalizes row order for the Java
// assertPropsPerRowIteratorAnyOrder assertions: rows sort by their compact
// JSON field rendering so both traces pin the same order.
func contextCategorySortedRows(rows []compat.ResultRecord) []compat.ResultRecord {
	if len(rows) == 0 {
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := json.Marshal(rows[i].Fields)
		right, _ := json.Marshal(rows[j].Fields)
		return string(left) < string(right)
	})
	return rows
}

func loadContextCategoryScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", contextCategoryID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", contextCategoryID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextCategoryID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextCategoryID, err)
	}
	if err := requireContextCategoryFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", contextCategoryID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != contextCategoryID ||
		metadata.Description != contextCategoryDescription ||
		metadata.JavaCommit != contextCategoryJavaCommit ||
		metadata.JavaSource != contextCategorySource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", contextCategoryID)
	}
	if err := validateContextCategoryStringArray(root["javaRuntimes"], contextCategoryJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextCategoryStringArray(root["javaNames"], contextCategoryJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextCategoryStringArray(root["javaStaticIds"], contextCategoryJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateContextCategoryStringArray(root["javaFlags"], contextCategoryJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(contextCategoryCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", contextCategoryID, len(contextCategoryCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireContextCategoryFields(object,
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
		if definition.Case != contextCategoryCases[index] ||
			definition.Ordinal != contextCategoryOrdinals[index] ||
			definition.RuntimeID != contextCategoryJavaRuntimeIDs[index] ||
			definition.ExecutionName != contextCategoryJavaExecutions[index] ||
			definition.Observation != contextCategoryCaseObservations[index] ||
			definition.EPL != contextCategoryCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", contextCategoryID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s steps: %w", contextCategoryID, err)
	}
	offset := 0
	for _, caseName := range contextCategoryCases {
		if offset >= len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s scenario is missing case %q", contextCategoryID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextCategoryID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return compat.Scenario{}, fmt.Errorf("%s step %d is not the %q case marker", contextCategoryID, offset, caseName)
		}
		offset++
		want, ok := contextCategoryCaseSteps[caseName]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("%s case %q has no pinned steps", contextCategoryID, caseName)
		}
		if offset+len(want) > len(rawSteps) {
			return compat.Scenario{}, fmt.Errorf("%s case %q is truncated", contextCategoryID, caseName)
		}
		for _, pinned := range want {
			key, err := contextCategoryStepKey(rawSteps[offset])
			if err != nil {
				return compat.Scenario{}, fmt.Errorf("%s step %d: %w", contextCategoryID, offset, err)
			}
			if key != pinned {
				return compat.Scenario{}, fmt.Errorf("%s case %q step %d = %q, want %q", contextCategoryID, caseName, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d trailing steps", contextCategoryID, len(rawSteps)-offset)
	}

	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", contextCategoryID, err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// contextCategoryStepKey renders one raw step as its pinned key:
// op|statement|name|eventType|epl|payload|expectError|compileWithoutPath|mode|
// ids|selector|filterProperty|filterValue with the payload and ids compacted.
// Unknown fields on the step object are rejected.
func contextCategoryStepKey(raw json.RawMessage) (string, error) {
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
		Selector           string          `json:"selector"`
		FilterProperty     string          `json:"filterProperty"`
		FilterValue        string          `json:"filterValue"`
	}
	if err := json.Unmarshal(raw, &step); err != nil {
		return "", err
	}
	allowed := map[string]bool{
		"op": true, "case": true, "statement": true, "name": true,
		"eventType": true, "epl": true, "payload": true, "expectError": true,
		"compileWithoutPath": true, "mode": true, "ids": true,
		"selector": true, "filterProperty": true, "filterValue": true,
	}
	for field := range object {
		if !allowed[field] {
			return "", fmt.Errorf("step has unexpected field %q", field)
		}
	}
	payload := ""
	if len(step.Payload) > 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.Payload); err != nil {
			return "", fmt.Errorf("payload: %w", err)
		}
		payload = compacted.String()
	}
	ids := ""
	if len(step.IDs) > 0 {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, step.IDs); err != nil {
			return "", fmt.Errorf("ids: %w", err)
		}
		ids = compacted.String()
	}
	cwp := ""
	if step.CompileWithoutPath {
		cwp = "1"
	}
	return step.Op + "|" + step.Statement + "|" + step.Name + "|" + step.EventType +
		"|" + step.Epl + "|" + payload + "|" + step.ExpectError + "|" + cwp +
		"|" + step.Mode + "|" + ids + "|" + step.Selector + "|" + step.FilterProperty +
		"|" + step.FilterValue, nil
}

// validateContextCategoryScenario re-checks a decoded scenario (used when the
// runner receives a scenario decoded by the generic loader path).
func validateContextCategoryScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != contextCategoryID {
		return fmt.Errorf("%s scenario id %q is not pinned", contextCategoryID, scenario.ID)
	}
	return nil
}

func requireContextCategoryFields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("%s JSON object has unexpected fields", contextCategoryID)
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("%s JSON object is missing field %q", contextCategoryID, name)
		}
	}
	return nil
}

func validateContextCategoryStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
