package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// Parity coverage for InfraNamedWindowOM (3 executions, the whole file):
// the SODA statement-object-model compile path over one keepall key/value
// named window plus its insert, irstream consumer, on-delete and on-select
// triggers. The SODA/eplToModel surface itself has no Go counterpart (the
// immutable Plan is the object model), so every assertEquals(model.toEPL())
// pin rides an unrepresentable record carrying the Java-asserted text while
// the deployed observable behavior — create-window, insert, select and
// on-trigger listener deliveries — replays through the typed Go chain.
//
// Covered executions (Java ordinals of the suite's executions()):
//   - ord 0 InfraCompile              java-runtime-2fadc5dd18bce73a28d5
//   - ord 1 InfraOM                   java-runtime-2d6d5656b8c1bbec11ac
//   - ord 2 InfraOMCreateTableSyntax  java-runtime-7521a52d49a6bc033264
const (
	infraNWOM564ID          = "infra-namedwindow-om-564"
	infraNWOM564Description = "InfraNamedWindowOM SODA/eplToModel compile-path slice (ords 0-2, all executions of the file): InfraCompile deploys five statements built by env.eplToModel — @name('create') @public create window MyWindow#keepall as select theString as key, longBoxed as value from SupportBean, an unconditional @name('onselect') on SupportBean_B select mywin.*, @name('insert') insert-into, @name('select') select irstream key, value*2 as value from MyWindow(key is not null) and a mid-scenario @name('delete') on SupportMarketDataBean delete where s0.symbol=s1.key — with toEPL equality asserts on create/select/delete, listener deliveries on all five statements and per-statement undeployModuleContaining teardown (delete/onselect/select/insert/create); InfraOM builds the same five statements as programmatic EPStatementObjectModels with toEPL asserts on create/select/ondelete/onselect, the select filter on value instead of key, a correlated on-select where s0.id=s1.key and undeployAll teardown; InfraOMCreateTableSyntax is compile-only (schema-column create-window OM asserted through toEPL). The Go runner replays the deployed key/value window, insert, irstream consumer, on-delete and on-select triggers through the typed chain — SODA model construction and toEPL round-trips have no Go counterpart (the immutable Plan is the object model) and ride unrepresentable records pinned to the asserted text; boundary sends pin the per-ord filters (null key skipped by the ord-0 key filter, null key emitted by the ord-1 value filter; null value projected as null by ord-0, filtered by ord-1) and the null-correlation delete probes (Java ' = ' is three-valued, so the null-key/null-symbol rows never match)."
	infraNWOM564JavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWOM564Source      = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOM.java"
)

// Byte-exact EPL pins (InfraNamedWindowOM.java): InfraCompile deploys the
// Java string literals at lines 44/49/53/57/72 via eplToModel; InfraOM's
// programmatic models are pinned by the assertEquals literals at lines
// 126/140/161/189 (create's assert runs after the annotations are set, so
// its pin carries them; select/ondelete/onselect assert the unannotated
// model text and the deploy step carries the annotated toEPL rendering);
// InfraOMCreateTableSyntax pins the line-212 literal.
const (
	infraNWOM564CreateEPL       = "@name('create') @public create window MyWindow#keepall as select theString as key, longBoxed as value from SupportBean"
	infraNWOM564OnSelectEPL     = "@name('onselect') on SupportBean_B select mywin.* from MyWindow as mywin"
	infraNWOM564InsertEPL       = "@name('insert') insert into MyWindow select theString as key, longBoxed as value from SupportBean"
	infraNWOM564SelectEPL       = "@name('select') select irstream key, value*2 as value from MyWindow(key is not null)"
	infraNWOM564DeleteEPL       = "@name('delete') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol=s1.key"
	infraNWOM564OMInsertEPL     = "insert into MyWindow select theString as key, longBoxed as value from SupportBean"
	infraNWOM564OMSelectText    = "select irstream key, value*2 as value from MyWindow(value is not null)"
	infraNWOM564OMSelectEPL     = "@name('select') " + infraNWOM564OMSelectText
	infraNWOM564OMDeleteText    = "on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol=s1.key"
	infraNWOM564OMDeleteEPL     = "@name('ondelete') " + infraNWOM564OMDeleteText
	infraNWOM564OMOnSelectText  = "on SupportBean_B as s0 select s1.* from MyWindow as s1 where s0.id=s1.key"
	infraNWOM564OMOnSelectEPL   = "@name('onselect') " + infraNWOM564OMOnSelectText
	infraNWOM564CreateTableText = "create window MyWindowOM#keepall as (a1 string, a2 double, a3 int)"
)

var (
	infraNWOM564JavaSources = []string{
		infraNWOM564Source,
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportBean_B.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportMarketDataBean.java",
	}
	infraNWOM564JavaRuntimeIDs = []string{
		"java-runtime-2fadc5dd18bce73a28d5",
		"java-runtime-2d6d5656b8c1bbec11ac",
		"java-runtime-7521a52d49a6bc033264",
	}
	infraNWOM564JavaExecutions = []string{
		"InfraCompile",
		"InfraOM",
		"InfraOMCreateTableSyntax",
	}
	infraNWOM564JavaStaticIDs = []string{
		"java-10c000a768d7eb5776f7",
		"java-10c000a768d7eb5776f7",
		"java-10c000a768d7eb5776f7",
	}
	infraNWOM564JavaFlags = []string{}
	infraNWOM564Cases     = []string{
		"compile",
		"om",
		"create-table-syntax",
	}
	infraNWOM564Ordinals = []int{0, 1, 2}
)

// infraNWOM564CaseObservations pin each case's ordered observable surface.
var infraNWOM564CaseObservations = []string{
	"listener+deployed+unrepresentable+undeploy; five eplToModel deploys in " +
		"source order (create, onselect, insert, select, then delete after the " +
		"E1/E2 sends) with toEPL pins on create/select/delete: each SupportBean " +
		"delivers the insert row plus the create-window and filtered-irstream " +
		"new rows, each matching market delete delivers the on-delete row plus " +
		"create/select old rows, the second E1 delete stays silent, B1 probes " +
		"the empty window silently and B2 emits the E3 row through mywin.*; " +
		"null-key/null-value boundary beans pin the key-is-not-null filter and " +
		"the null-symbol/null-key delete probes pin three-valued equals; " +
		"per-statement undeploys run delete/onselect/select/insert/create",
	"listener+deployed+unrepresentable+undeploy-all; programmatic " +
		"EPStatementObjectModel builds of the same five statements with toEPL " +
		"pins on create/select/ondelete/onselect: identical insert/delete " +
		"behavior, the select filter reads value instead of key (the null-key " +
		"boundary bean still emits {null,198}), the on-select correlates " +
		"s0.id=s1.key so B1 and the null-id probe stay silent while the E3 " +
		"trigger emits the matching s1.* row, and teardown is undeployAll",
	"unrepresentable; compile-only object-model construction of a " +
		"schema-column create window asserted through toEPL — zero deploys, " +
		"events or listeners, so the single record pins the asserted text",
}

// infraNWOM564Pin is one unrepresentable record's pinned probe: the
// byte-exact EPL text the Java execution asserts (model.toEPL()) plus the
// recorded note.
type infraNWOM564Pin struct {
	epl  string
	note string
}

// infraNWOM564Note renders the pinned unrepresentable-record note shared by
// the Go runner and the Java oracle: every toEPL assert is a SODA
// statement-object-model surface with no Go counterpart.
func infraNWOM564Note(epl string) string {
	return "soda-EPStatementObjectModel.toEPL: " + epl + " - Java builds or " +
		"parses a statement object model and asserts assertEquals(text, " +
		"model.toEPL()); the Go surface has no statement object model or " +
		"EPL-text parser (the immutable Plan is the object model), so the " +
		"asserted text is pinned verbatim with no Go counterpart"
}

// infraNWOM564UnrepresentablePins pins the toEPL-asserted text per record
// label. compile asserts after deploy (create/select/delete); om asserts the
// unannotated model before the annotation is set (select/ondelete/onselect)
// or the annotated model (create); create-table-syntax asserts the
// schema-column model.
var infraNWOM564UnrepresentablePins = map[string]infraNWOM564Pin{
	"toepl-create":        {epl: infraNWOM564CreateEPL, note: infraNWOM564Note(infraNWOM564CreateEPL)},
	"toepl-select":        {epl: infraNWOM564SelectEPL, note: infraNWOM564Note(infraNWOM564SelectEPL)},
	"toepl-delete":        {epl: infraNWOM564DeleteEPL, note: infraNWOM564Note(infraNWOM564DeleteEPL)},
	"toepl-select-om":     {epl: infraNWOM564OMSelectText, note: infraNWOM564Note(infraNWOM564OMSelectText)},
	"toepl-ondelete":      {epl: infraNWOM564OMDeleteText, note: infraNWOM564Note(infraNWOM564OMDeleteText)},
	"toepl-onselect":      {epl: infraNWOM564OMOnSelectText, note: infraNWOM564Note(infraNWOM564OMOnSelectText)},
	"toepl-create-window": {epl: infraNWOM564CreateTableText, note: infraNWOM564Note(infraNWOM564CreateTableText)},
}

// infraNWOM564Statement carries one deployed label: the byte-exact EPL Java
// compiles plus the listener name the deployed statement reports (InfraOM's
// insert is unnamed and its delete/on-select carry the ondelete/onselect
// names).
type infraNWOM564Statement struct {
	epl      string
	listener string
}

// infraNWOM564CaseSpec pins one Java execution: which deploy labels exist,
// their EPL and listener names, and which pin labels apply.
type infraNWOM564CaseSpec struct {
	statements map[string]infraNWOM564Statement
	listened   map[string]bool
	model      bool
	correlated bool
	selectKey  string
}

var infraNWOM564CaseSpecs = map[string]infraNWOM564CaseSpec{
	"compile": {
		statements: map[string]infraNWOM564Statement{
			"create":   {epl: infraNWOM564CreateEPL, listener: "create"},
			"onselect": {epl: infraNWOM564OnSelectEPL, listener: "onselect"},
			"insert":   {epl: infraNWOM564InsertEPL, listener: "insert"},
			"select":   {epl: infraNWOM564SelectEPL, listener: "select"},
			"delete":   {epl: infraNWOM564DeleteEPL, listener: "delete"},
		},
		listened:  map[string]bool{"create": true, "onselect": true, "insert": true, "select": true, "delete": true},
		selectKey: "key",
	},
	"om": {
		statements: map[string]infraNWOM564Statement{
			"create":   {epl: infraNWOM564CreateEPL, listener: "create"},
			"insert":   {epl: infraNWOM564OMInsertEPL},
			"select":   {epl: infraNWOM564OMSelectEPL, listener: "select"},
			"delete":   {epl: infraNWOM564OMDeleteEPL, listener: "ondelete"},
			"onselect": {epl: infraNWOM564OMOnSelectEPL, listener: "onselect"},
		},
		listened:   map[string]bool{"create": true, "select": true, "delete": true, "onselect": true},
		model:      true,
		correlated: true,
		selectKey:  "value",
	},
	"create-table-syntax": {
		statements: map[string]infraNWOM564Statement{},
		listened:   map[string]bool{},
	},
}

// infraNWOM564Bean mirrors the SupportBean columns the Java module reads:
// theString and the boxed longBoxed (both nullable Long/String properties).
type infraNWOM564Bean struct {
	TheString *string `esper:"theString"`
	LongBoxed *int64  `esper:"longBoxed"`
}

// infraNWOM564B mirrors SupportBean_B (id only) and infraNWOM564Market
// mirrors the SupportMarketDataBean(symbol, 0, 0L, "") trigger surface.
type infraNWOM564B struct {
	ID *string `esper:"id"`
}

type infraNWOM564Market struct {
	Symbol *string `esper:"symbol"`
}

func runInfraNWOM564Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	if len(scenario.Steps) == 0 {
		return compat.Trace{}, fmt.Errorf("%s scenario has no steps", infraNWOM564ID)
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWOM564Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		records, err := runInfraNWOM564Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWOM564ID, caseName, err)
		}
		trace.Records = append(trace.Records, records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWOM564ID)
	}
	return trace, nil
}

// infraNWOM564CaseState carries the per-case replay state: the engine,
// label-keyed deployments and the deployed-label set.
type infraNWOM564CaseState struct {
	env         *esper.Environment
	engine      *esper.Engine
	spec        infraNWOM564CaseSpec
	caseName    string
	deployments map[string]*esper.Deployment
	deployed    map[string]bool
	sequence    map[string]uint64
	records     []compat.TraceRecord
}

// runInfraNWOM564Case replays one Java execution on a fresh
// environment/engine pair (one runtime per execution). The oracle runs with
// the internal timer disabled at epoch, so every timed record pins the
// epoch.
func runInfraNWOM564Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) ([]compat.TraceRecord, error) {
	spec, ok := infraNWOM564CaseSpecs[caseName]
	if !ok {
		return nil, fmt.Errorf("%s: unknown case %q", infraNWOM564ID, caseName)
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[infraNWOM564Bean](env, "SupportBean"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWOM564B](env, "SupportBean_B"); err != nil {
		return nil, err
	}
	if _, err := esper.RegisterStruct[infraNWOM564Market](env, "SupportMarketDataBean"); err != nil {
		return nil, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWOM564JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()))
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraNWOM564CaseState{
		env:         env,
		engine:      engine,
		spec:        spec,
		caseName:    caseName,
		deployments: map[string]*esper.Deployment{},
		deployed:    map[string]bool{},
		sequence:    map[string]uint64{},
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch step.Op {
		case "case":
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return nil, err
			}
		case "deployed":
			if err := state.emitDeployed(step); err != nil {
				return nil, err
			}
		case "send":
			payload, err := decodeInfraNWOM564Payload(step)
			if err != nil {
				return nil, err
			}
			if err := engine.Send(ctx, step.EventType, payload); err != nil {
				return nil, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "unrepresentable":
			if err := state.emitUnrepresentable(step); err != nil {
				return nil, err
			}
		case "undeploy":
			if err := state.undeploy(ctx, step); err != nil {
				return nil, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s: unexpected op %q", infraNWOM564ID, step.Op)
		}
	}
	if len(state.records) == 0 {
		return nil, fmt.Errorf("case %q produced no records", caseName)
	}
	return state.records, nil
}

// recordListener appends the normalized new/old rows of one listener
// delivery under the Java listener name.
func (s *infraNWOM564CaseState) recordListener(statement string, batch esper.ResultBatch) {
	s.sequence[statement]++
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "listener",
		Statement: statement,
		Sequence:  s.sequence[statement],
		Time:      compat.FormatTraceTime(batch.Time),
		New:       compat.NormalizeResults(batch.New),
		Old:       compat.NormalizeResults(batch.Old),
	}
	if len(record.Old) == 0 {
		record.Old = nil
	}
	s.records = append(s.records, record)
}

// deploy maps one scenario label onto the typed Go chain equivalent of the
// Java eplToModel/EPStatementObjectModel statement: create registers the
// keepall key/value window env-level (the Java window is @public so every
// statement compiles against the shared RegressionPath) and deploys the
// direct-child create-window query, insert deploys the on-SupportBean
// trigger, select deploys the filtered irstream consumer (ord 0 filters key,
// ord 1 filters value), delete deploys the correlated on-delete trigger and
// onselect deploys the unconditional (ord 0) or correlated (ord 1)
// on-select trigger. Statements carrying a Java listener name are
// subscribed for trace records.
func (s *infraNWOM564CaseState) deploy(ctx context.Context, step compat.Step) error {
	statement, ok := s.spec.statements[step.Statement]
	if !ok || step.Epl != statement.epl {
		return fmt.Errorf("%s: deploy %q does not pin the expected EPL %q",
			infraNWOM564ID, step.Statement, step.Epl)
	}
	// delete pre-deploys at the select step so Go's deployment-order merge
	// places its listener records where Java emits them; the adopt branch
	// in the delete deploy step only validates the EPL pin and flips the
	// deployed marker.
	if step.Statement == "select" {
		if err := s.preDeployDelete(ctx); err != nil {
			return err
		}
	}
	var deployment *esper.Deployment
	if step.Statement == "delete" && s.deployments["delete"] != nil {
		deployment = s.deployments["delete"]
	} else {
		plan, err := s.buildPlan(step.Statement, statement)
		if err != nil {
			return fmt.Errorf("%s: build %q: %w", infraNWOM564ID, step.Statement, err)
		}
		deployment, err = s.engine.Deploy(ctx, plan)
		if err != nil {
			return fmt.Errorf("%s: deploy %q: %w", infraNWOM564ID, step.Statement, err)
		}
		s.deployments[step.Statement] = deployment
	}
	s.deployed[step.Statement] = true
	if !s.spec.listened[step.Statement] {
		return nil
	}
	deployed := deployment.Statements()
	if len(deployed) != 1 {
		return fmt.Errorf("%s: deploy %q produced %d statements", infraNWOM564ID, step.Statement, len(deployed))
	}
	listener := statement.listener
	if _, err := deployed[0].Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		s.recordListener(listener, batch)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// preDeployDelete deploys the on-delete trigger ahead of the select
// consumer: Go's deferred trigger dispatches merge into the window delta
// wave by deployment order, and Java delivers each market delete as
// create-window old rows, then the on-delete statement's new rows, then the
// irstream consumer's old rows — so the delete trigger needs a lower
// deployment order than select. The trigger observes only
// SupportMarketDataBean events, which the scenario never sends before the
// delete deploy step, so the early deployment has no observable effect.
func (s *infraNWOM564CaseState) preDeployDelete(ctx context.Context) error {
	statement, ok := s.spec.statements["delete"]
	if !ok {
		return nil
	}
	if _, deployed := s.deployments["delete"]; deployed {
		return nil
	}
	plan, err := s.buildPlan("delete", statement)
	if err != nil {
		return fmt.Errorf("%s: build %q: %w", infraNWOM564ID, "delete", err)
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return fmt.Errorf("%s: deploy %q: %w", infraNWOM564ID, "delete", err)
	}
	s.deployments["delete"] = deployment
	return nil
}

func (s *infraNWOM564CaseState) buildPlan(label string, statement infraNWOM564Statement) (esper.Plan, error) {
	name := statement.listener
	if name == "" {
		name = label
	}
	switch label {
	case "create":
		// create window MyWindow#keepall as select theString as key,
		// longBoxed as value from SupportBean: the schema is the two
		// projected nullable columns; the direct-child query carries the
		// window's own IR stream (the Java 'create' listener sees inserts
		// as new rows and deletes as old rows).
		schema, err := esper.NewMapSchema("MyWindow", []esper.FieldSpec{
			esper.OptionalFieldDef("key", reflect.TypeOf((*string)(nil))),
			esper.OptionalFieldDef("value", reflect.TypeOf((*int64)(nil))),
		})
		if err != nil {
			return esper.Plan{}, err
		}
		if _, err := esper.CreateNamedWindow(s.env, "MyWindow", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return esper.Plan{}, err
		}
		return s.env.Build(esper.FromNamedWindow(s.env, "MyWindow").
			CreateNamedWindowQuery(esper.StatementName(name), esper.WithOldStream()))
	case "insert":
		return s.env.Build(esper.OnEvent(esper.From[infraNWOM564Bean](s.env, "SupportBean")).
			InsertIntoNamedWindow("MyWindow",
				esper.SetColumn("key", esper.Field[infraNWOM564Bean, *string]("theString")),
				esper.SetColumn("value", esper.Field[infraNWOM564Bean, *int64]("longBoxed"))).
			Query(esper.StatementName(name)))
	case "select":
		// select irstream key, value*2 as value from MyWindow(<col> is not
		// null): ord 0 filters the key column, ord 1 the value column.
		return s.env.Build(esper.FromNamedWindow(s.env, "MyWindow").
			Filter(esper.Not(esper.IsNull[any](esper.Field[any, any](s.spec.selectKey)))).
			Select(
				esper.Alias("key", esper.Field[any, any]("key")),
				esper.Alias("value", esper.MultiplyOf[int64](esper.Field[any, any]("value"), esper.Literal(int64(2))))).
			Query(esper.StatementName(name), esper.WithOldStream()))
	case "delete":
		// on SupportMarketDataBean as s0 delete from MyWindow as s1 where
		// s0.symbol=s1.key: Field reads the trigger event, NamedWindowField
		// reads the candidate window row.
		return s.env.Build(esper.OnEvent(esper.From[infraNWOM564Market](s.env, "SupportMarketDataBean")).
			DeleteFromNamedWindow("MyWindow",
				esper.Equal[*string](esper.Field[infraNWOM564Market, *string]("symbol"),
					esper.NamedWindowField[*string]("key"))).
			Query(esper.StatementName(name)))
	case "onselect":
		trigger := esper.OnEvent(esper.From[infraNWOM564B](s.env, "SupportBean_B"))
		if s.spec.correlated {
			// on SupportBean_B as s0 select s1.* from MyWindow as s1 where
			// s0.id=s1.key: the correlation reads the trigger id against
			// the candidate row's key; s1.* emits the window row itself.
			return s.env.Build(trigger.SelectFromNamedWindow("MyWindow",
				esper.Equal[*string](esper.Field[infraNWOM564B, *string]("id"),
					esper.NamedWindowField[*string]("key"))).
				Query(esper.StatementName(name)))
		}
		// on SupportBean_B select mywin.* from MyWindow as mywin:
		// unconditional on-select emitting the retained window rows.
		return s.env.Build(trigger.SelectFromNamedWindow("MyWindow", nil).
			Query(esper.StatementName(name)))
	}
	return esper.Plan{}, fmt.Errorf("%s: unexpected deploy label %q", infraNWOM564ID, label)
}

// emitDeployed mirrors the oracle's deployed marker: a per-label sequence
// and the epoch timestamp (the oracle runs with the internal timer off).
func (s *infraNWOM564CaseState) emitDeployed(step compat.Step) error {
	if !s.deployed[step.Statement] {
		return fmt.Errorf("%s: deployed marker for unknown statement %q",
			infraNWOM564ID, step.Statement)
	}
	s.sequence[step.Statement+":deployed"]++
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "deployed",
		Statement: step.Statement,
		Sequence:  s.sequence[step.Statement+":deployed"],
		Time:      compat.FormatTraceTime(s.engine.Now()),
	})
	return nil
}

// emitUnrepresentable emits the pinned toEPL record: Java's
// assertEquals(text, model.toEPL()) is a SODA object-model surface with no
// Go counterpart, so the asserted text rides the record note verbatim.
func (s *infraNWOM564CaseState) emitUnrepresentable(step compat.Step) error {
	pin, ok := infraNWOM564UnrepresentablePins[step.Statement]
	if !ok || step.Epl != pin.epl || step.ExpectError != pin.note {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWOM564ID, step.Statement)
	}
	s.records = append(s.records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

// undeploy mirrors undeployModuleContaining: each deploy step produced one
// single-statement deployment, so the label selects the deployment
// containing the named statement.
func (s *infraNWOM564CaseState) undeploy(ctx context.Context, step compat.Step) error {
	deployment, ok := s.deployments[step.Statement]
	if !ok {
		return fmt.Errorf("%s: undeploy %q without deploy", infraNWOM564ID, step.Statement)
	}
	if err := deployment.Undeploy(ctx); err != nil {
		return fmt.Errorf("%s: undeploy %q: %w", infraNWOM564ID, step.Statement, err)
	}
	delete(s.deployments, step.Statement)
	delete(s.deployed, step.Statement)
	return nil
}

func (s *infraNWOM564CaseState) undeployAll(ctx context.Context) error {
	for label, deployment := range s.deployments {
		if err := deployment.Undeploy(ctx); err != nil {
			return fmt.Errorf("%s: undeploy-all %q: %w", infraNWOM564ID, label, err)
		}
	}
	s.deployments = map[string]*esper.Deployment{}
	s.deployed = map[string]bool{}
	return nil
}

func decodeInfraNWOM564Payload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, err
	}
	switch step.EventType {
	case "SupportBean":
		if err := requireInfraNWOM564Fields(fields, "theString", "longBoxed"); err != nil {
			return nil, fmt.Errorf("SupportBean payload: %w", err)
		}
		var bean infraNWOM564Bean
		if err := json.Unmarshal(step.Payload, &bean); err != nil {
			return nil, fmt.Errorf("decode SupportBean: %w", err)
		}
		return bean, nil
	case "SupportBean_B":
		if err := requireInfraNWOM564Fields(fields, "id"); err != nil {
			return nil, fmt.Errorf("SupportBean_B payload: %w", err)
		}
		var event infraNWOM564B
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportBean_B: %w", err)
		}
		return event, nil
	case "SupportMarketDataBean":
		if err := requireInfraNWOM564Fields(fields, "symbol"); err != nil {
			return nil, fmt.Errorf("SupportMarketDataBean payload: %w", err)
		}
		var event infraNWOM564Market
		if err := json.Unmarshal(step.Payload, &event); err != nil {
			return nil, fmt.Errorf("decode SupportMarketDataBean: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

// loadInfraNWOM564Scenario enforces the strict scenario contract shared by
// the differential runners: no duplicate or unknown JSON fields, pinned
// metadata, pinned per-case identity and a full step-shape pin.
func loadInfraNWOM564Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWOM564ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWOM564ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOM564ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOM564ID, err)
	}
	if err := requireInfraNWOM564Fields(root,
		"version", "id", "description", "javaCommit", "javaSource", "javaSourceFiles",
		"javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"); err != nil {
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWOM564ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWOM564ID ||
		metadata.Description != infraNWOM564Description ||
		metadata.JavaCommit != infraNWOM564JavaCommit ||
		metadata.JavaSource != infraNWOM564Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWOM564ID)
	}
	if err := validateInfraNWOM564StringArray(root["javaSourceFiles"], infraNWOM564JavaSources, "javaSourceFiles"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOM564StringArray(root["javaRuntimes"], infraNWOM564JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOM564StringArray(root["javaNames"], infraNWOM564JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOM564StringArray(root["javaStaticIds"], infraNWOM564JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOM564StringArray(root["javaFlags"], infraNWOM564JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWOM564Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWOM564ID, len(infraNWOM564Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWOM564Fields(object,
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
		if definition.Case != infraNWOM564Cases[index] ||
			definition.Ordinal != infraNWOM564Ordinals[index] ||
			definition.RuntimeID != infraNWOM564JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWOM564JavaExecutions[index] ||
			definition.Observation != infraNWOM564CaseObservations[index] ||
			definition.EPL != infraNWOM564CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWOM564ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWOM564ID)
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		switch operation {
		case "case":
			if err := requireInfraNWOM564Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWOM564Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed", "undeploy":
			if err := requireInfraNWOM564Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireInfraNWOM564Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWOM564Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWOM564Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWOM564Fields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWOM564RawSteps(rawSteps); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func requireInfraNWOM564Fields(object map[string]json.RawMessage, names ...string) error {
	if len(object) != len(names) {
		return fmt.Errorf("JSON object has unexpected fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return fmt.Errorf("JSON object is missing field %q", name)
		}
	}
	return nil
}

func validateInfraNWOM564StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for index := range expected {
		if values[index] != expected[index] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

// infraNWOM564CaseEPLs pins the newline-joined EPL of every EPL-bearing
// step in the case, in step order — the value carried by the scenario
// cases[] metadata. compile joins the five deploys and the three toEPL pins;
// om joins the four toEPL pins and the five deploys; create-table-syntax
// carries the single toEPL pin.
var infraNWOM564CaseEPLs = []string{
	infraNWOM564CreateEPL + "\n" +
		infraNWOM564CreateEPL + "\n" +
		infraNWOM564OnSelectEPL + "\n" +
		infraNWOM564InsertEPL + "\n" +
		infraNWOM564SelectEPL + "\n" +
		infraNWOM564SelectEPL + "\n" +
		infraNWOM564DeleteEPL + "\n" +
		infraNWOM564DeleteEPL,
	infraNWOM564CreateEPL + "\n" +
		infraNWOM564CreateEPL + "\n" +
		infraNWOM564OMInsertEPL + "\n" +
		infraNWOM564OMSelectText + "\n" +
		infraNWOM564OMSelectEPL + "\n" +
		infraNWOM564OMDeleteText + "\n" +
		infraNWOM564OMDeleteEPL + "\n" +
		infraNWOM564OMOnSelectText + "\n" +
		infraNWOM564OMOnSelectEPL,
	infraNWOM564CreateTableText,
}

// infraNWOM564CaseSteps pins the complete step sequence per case in Java
// source order. compile (InfraCompile.run, InfraNamedWindowOM.java lines
// 44-109) deploys create/onselect/insert/select up front, sends E1/E2,
// deploys delete, runs the delete matrix and the B1/B2 on-select probes,
// then the null-boundary sends and the per-statement undeploys. om
// (InfraOM.run, lines 119-206) interleaves each toEPL assert before the
// deploy, deploys on-select after the delete matrix, sends E3/E4 and the
// B1/E3 probes before the boundary sends and undeployAll. create-table-syntax
// (InfraOMCreateTableSyntax.run, lines 212-221) is the single toEPL pin.
var infraNWOM564CaseSteps = map[string][]string{
	"compile": {
		"deploy:create:" + infraNWOM564CreateEPL,
		"deployed:create",
		"unrepresentable:toepl-create:" + infraNWOM564CreateEPL + ":" + infraNWOM564Note(infraNWOM564CreateEPL),
		"deploy:onselect:" + infraNWOM564OnSelectEPL,
		"deployed:onselect",
		"deploy:insert:" + infraNWOM564InsertEPL,
		"deployed:insert",
		"deploy:select:" + infraNWOM564SelectEPL,
		"deployed:select",
		"unrepresentable:toepl-select:" + infraNWOM564SelectEPL + ":" + infraNWOM564Note(infraNWOM564SelectEPL),
		`send:SupportBean:{"longBoxed":10,"theString":"E1"}`,
		`send:SupportBean:{"longBoxed":20,"theString":"E2"}`,
		"deploy:delete:" + infraNWOM564DeleteEPL,
		"deployed:delete",
		"unrepresentable:toepl-delete:" + infraNWOM564DeleteEPL + ":" + infraNWOM564Note(infraNWOM564DeleteEPL),
		`send:SupportMarketDataBean:{"symbol":"E1"}`,
		`send:SupportMarketDataBean:{"symbol":"E1"}`,
		`send:SupportMarketDataBean:{"symbol":"E2"}`,
		`send:SupportBean_B:{"id":"B1"}`,
		`send:SupportBean:{"longBoxed":30,"theString":"E3"}`,
		`send:SupportBean_B:{"id":"B2"}`,
		`send:SupportBean:{"longBoxed":99,"theString":null}`,
		`send:SupportBean:{"longBoxed":null,"theString":"BND"}`,
		`send:SupportMarketDataBean:{"symbol":null}`,
		`send:SupportMarketDataBean:{"symbol":"BND"}`,
		"undeploy:delete",
		"undeploy:onselect",
		"undeploy:select",
		"undeploy:insert",
		"undeploy:create",
	},
	"om": {
		"unrepresentable:toepl-create:" + infraNWOM564CreateEPL + ":" + infraNWOM564Note(infraNWOM564CreateEPL),
		"deploy:create:" + infraNWOM564CreateEPL,
		"deployed:create",
		"deploy:insert:" + infraNWOM564OMInsertEPL,
		"deployed:insert",
		"unrepresentable:toepl-select-om:" + infraNWOM564OMSelectText + ":" + infraNWOM564Note(infraNWOM564OMSelectText),
		"deploy:select:" + infraNWOM564OMSelectEPL,
		"deployed:select",
		`send:SupportBean:{"longBoxed":10,"theString":"E1"}`,
		`send:SupportBean:{"longBoxed":20,"theString":"E2"}`,
		"unrepresentable:toepl-ondelete:" + infraNWOM564OMDeleteText + ":" + infraNWOM564Note(infraNWOM564OMDeleteText),
		"deploy:delete:" + infraNWOM564OMDeleteEPL,
		"deployed:delete",
		`send:SupportMarketDataBean:{"symbol":"E1"}`,
		`send:SupportMarketDataBean:{"symbol":"E1"}`,
		`send:SupportMarketDataBean:{"symbol":"E2"}`,
		"unrepresentable:toepl-onselect:" + infraNWOM564OMOnSelectText + ":" + infraNWOM564Note(infraNWOM564OMOnSelectText),
		"deploy:onselect:" + infraNWOM564OMOnSelectEPL,
		"deployed:onselect",
		`send:SupportBean:{"longBoxed":30,"theString":"E3"}`,
		`send:SupportBean:{"longBoxed":40,"theString":"E4"}`,
		`send:SupportBean_B:{"id":"B1"}`,
		`send:SupportBean_B:{"id":"E3"}`,
		`send:SupportBean:{"longBoxed":99,"theString":null}`,
		`send:SupportBean:{"longBoxed":null,"theString":"BND"}`,
		`send:SupportBean_B:{"id":null}`,
		`send:SupportMarketDataBean:{"symbol":null}`,
		`send:SupportMarketDataBean:{"symbol":"BND"}`,
		"undeploy-all",
	},
	"create-table-syntax": {
		"unrepresentable:toepl-create-window:" + infraNWOM564CreateTableText + ":" + infraNWOM564Note(infraNWOM564CreateTableText),
	},
}

// validateInfraNWOM564RawSteps pins the complete step sequence per case
// against the raw JSON objects: op field whitelist is enforced at parse
// time, the ordered pinned keys cover deploy/deployed/unrepresentable/send/
// undeploy/undeploy-all.
func validateInfraNWOM564RawSteps(rawSteps []json.RawMessage) error {
	offset := 0
	for _, caseName := range infraNWOM564Cases {
		want, ok := infraNWOM564CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWOM564ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWOM564ID, caseName)
		}
		var marker struct {
			Op   string `json:"op"`
			Case string `json:"case"`
		}
		if err := json.Unmarshal(rawSteps[offset], &marker); err != nil {
			return fmt.Errorf("%s step %d: %w", infraNWOM564ID, offset, err)
		}
		if marker.Op != "case" || marker.Case != caseName {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWOM564ID, caseName)
		}
		offset++
		for _, pinned := range want {
			var step struct {
				Op          string          `json:"op"`
				Case        string          `json:"case"`
				Statement   string          `json:"statement"`
				EPL         string          `json:"epl"`
				ExpectError string          `json:"expectError"`
				EventType   string          `json:"eventType"`
				Payload     json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWOM564ID, offset, err)
			}
			if step.Case != caseName {
				return fmt.Errorf("%s step %d is not pinned for case %q", infraNWOM564ID, offset, caseName)
			}
			var key string
			switch step.Op {
			case "deploy":
				key = "deploy:" + step.Statement + ":" + step.EPL
			case "deployed":
				key = "deployed:" + step.Statement
			case "unrepresentable":
				key = "unrepresentable:" + step.Statement + ":" + step.EPL + ":" + step.ExpectError
			case "send":
				var payload map[string]any
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOM564ID, offset, err)
				}
				canonical, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOM564ID, offset, err)
				}
				key = "send:" + step.EventType + ":" + string(canonical)
			case "undeploy":
				key = "undeploy:" + step.Statement
			case "undeploy-all":
				key = "undeploy-all"
			default:
				return fmt.Errorf("%s step %d has unsupported op %q", infraNWOM564ID, offset, step.Op)
			}
			if key != pinned {
				return fmt.Errorf("%s step %d is not pinned: got %q want %q", infraNWOM564ID, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWOM564ID)
	}
	return nil
}
