package parity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// infraNWTableCreateDDL563Bean mirrors SupportSpatialPoint: the event type the
// AdvancedSyntax execution deploys its MyWindow#keepall over (InfraNWTableCreateIndexAdvancedSyntax).
type infraNWTableCreateDDL563Bean struct {
	ID       string   `esper:"id"`
	Px       *float64 `esper:"px"`
	Py       *float64 `esper:"py"`
	Category *string  `esper:"category"`
}

const (
	infraNWTableCreateDDL563ID         = "infra-nwtable-create-ddl-563"
	infraNWTableCreateDDL563JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	// The scenario covers both suite files in this directory; the full
	// per-file list rides javaSourceFiles.
	infraNWTableCreateDDL563JavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable"
)

const infraNWTableCreateDDL563Description = "InfraNWTableCreate ordinals 0-1 plus InfraNWTableCreateIndexAdvancedSyntax ordinal 0: InfraCreateGenericColType {namedWindow=true/false} deploys one @public @buseventtype create-map-schema MyInputEvent module (8 parameterized columns from SupportGenericColUtil: java.util.List<String>, java.util.List<Optional<Integer>>, java.util.Map<String,Integer>, java.util.List<String>[], java.util.List<String[]>, java.util.List<String>[][], java.util.List<String[][]>, java.util.List<T>) including @name('infra') create-window MyInfra#keepall or create-table MyInfra over the same columns and 'on MyInputEvent merge MyInfra insert select <8 names>', asserts the infra statement's EPType descriptors, sends the SupportGenericColUtil sample map event, runs milestone(0) (a harness no-op carrying no step) and iterators the single merged row; InfraNWTableCreateIndexAdvancedSyntax runs four SODA eplToModel round-trips, deploys an @public SupportSpatialPoint(id,px,py,category) keepall MyWindow and replays five invalid create-index compile probes as unrepresentable records (the Go surface carries no statement-object model or EPL-text compiler; the typed create-index validation equivalents are exercised live where a Go form exists)."

// Verbatim transcriptions of InfraNWTableCreate.java lines 40-47 (ords
// 0-1, InfraCreateGenericColType.run): each execution concatenates the
// three statements with ';\n' separators and passes the module to a
// single env.compileDeploy. allNamesAndTypes() emits comma-joined
// java.util.-qualified type tokens; allNames() is comma-joined.
const (
	infraNWTableCreateDDL563NamesAndTypes = "listOfString java.util.List<String>,listOfOptionalInteger java.util.List<Optional<Integer>>,mapOfStringAndInteger java.util.Map<String, Integer>,listArrayOfString java.util.List<String>[],listOfStringArray java.util.List<String[]>,listArray2DimOfString java.util.List<String>[][],listOfStringArray2Dim java.util.List<String[][]>,listOfT java.util.List<Object>"
	infraNWTableCreateDDL563AllNames      = "listOfString,listOfOptionalInteger,mapOfStringAndInteger,listArrayOfString,listOfStringArray,listArray2DimOfString,listOfStringArray2Dim,listOfT"
	infraNWTableCreateDDL563ModuleWindow  = "@public @buseventtype create schema MyInputEvent(" + infraNWTableCreateDDL563NamesAndTypes + ");\n@name('infra')create window MyInfra#keepall as (" + infraNWTableCreateDDL563NamesAndTypes + ");\non MyInputEvent merge MyInfra insert select " + infraNWTableCreateDDL563AllNames + ";\n"
	infraNWTableCreateDDL563ModuleTable   = "@public @buseventtype create schema MyInputEvent(" + infraNWTableCreateDDL563NamesAndTypes + ");\n@name('infra')create table MyInfra as (" + infraNWTableCreateDDL563NamesAndTypes + ");\non MyInputEvent merge MyInfra insert select " + infraNWTableCreateDDL563AllNames + ";\n"
)

// Verbatim transcriptions of InfraNWTableCreateIndexAdvancedSyntax.java
// (ord 0): lines 22-25 the four SODA eplToModel round-trips, line 28 the
// window deploy, lines 30-43 the five tryInvalidCompile probes.
const (
	infraNWTableCreateDDL563CreateWindowAdv  = "@public create window MyWindow#keepall as SupportSpatialPoint"
	infraNWTableCreateDDL563SODAAdvancedArgs = `create index MyIndex on MyWindow((x,y) dummy_name("a",10101))`
	infraNWTableCreateDDL563SODASingleType   = "create index MyIndex on MyWindow(x dummy_name)"
	infraNWTableCreateDDL563SODAMultiColType = "create index MyIndex on MyWindow((x,y,z) dummy_name)"
	infraNWTableCreateDDL563SODAMixedCols    = `create index MyIndex on MyWindow(x dummy_name, (y,z) dummy_name_2("a"), p dummyname3)`
	infraNWTableCreateDDL563InvalidEmptyList = "create index MyIndex on MyWindow(())"
	infraNWTableCreateDDL563InvalidExprCol   = "create index MyIndex on MyWindow(intPrimitive+1)"
	infraNWTableCreateDDL563InvalidMulti     = "create index MyIndex on MyWindow((x, y))"
	infraNWTableCreateDDL563InvalidDotted    = "create index MyIndex on MyWindow(x.y)"
	infraNWTableCreateDDL563InvalidAdvType   = "create index MyIndex on MyWindow(id xxxx)"
)

var (
	infraNWTableCreateDDL563JavaSources = []string{
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreate.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndexAdvancedSyntax.java",
		"regression-lib/src/main/java/com/espertech/esper/regressionlib/support/events/SupportGenericColUtil.java",
	}
	infraNWTableCreateDDL563JavaRuntimeIDs = []string{
		"java-runtime-b7fab192ff4a0d2ff2ce",
		"java-runtime-48d80d017090dd3410b7",
		"java-runtime-bc1a897eca64b5da6df3",
	}
	infraNWTableCreateDDL563JavaExecutions = []string{
		"InfraCreateGenericColType{namedWindow=true}",
		"InfraCreateGenericColType{namedWindow=false}",
		"InfraNWTableCreateIndexAdvancedSyntax",
	}
	infraNWTableCreateDDL563JavaStaticIDs = []string{
		"java-621032f62ef7cf1bc193",
		"java-621032f62ef7cf1bc193",
		"java-b6b074549bedc2ab1767",
	}
	infraNWTableCreateDDL563JavaFlags = []string{"SERDEREQUIRED"}
	infraNWTableCreateDDL563Cases     = []string{
		"generic-col-window",
		"generic-col-table",
		"index-syntax",
	}
	infraNWTableCreateDDL563Ordinals = []int{0, 1, 0}
)

// infraNWTableCreateDDL563CaseEPLs pins the newline-joined EPL of every
// EPL-bearing step in the case, in step order — the value carried by the
// scenario cases[] metadata. The create legs carry the single module
// text Java passes to compileDeploy; index-syntax joins the four SODA
// round-trips, the window deploy and the five invalid probes in source
// order.
var infraNWTableCreateDDL563CaseEPLs = []string{
	infraNWTableCreateDDL563ModuleWindow,
	infraNWTableCreateDDL563ModuleTable,
	strings.Join([]string{
		infraNWTableCreateDDL563SODAAdvancedArgs,
		infraNWTableCreateDDL563SODASingleType,
		infraNWTableCreateDDL563SODAMultiColType,
		infraNWTableCreateDDL563SODAMixedCols,
		infraNWTableCreateDDL563CreateWindowAdv,
		infraNWTableCreateDDL563InvalidEmptyList,
		infraNWTableCreateDDL563InvalidExprCol,
		infraNWTableCreateDDL563InvalidMulti,
		infraNWTableCreateDDL563InvalidDotted,
		infraNWTableCreateDDL563InvalidAdvType,
	}, "\n"),
}

var infraNWTableCreateDDL563CaseObservations = []string{
	"deploy+types+send+snapshot+undeploy; a single module deploy carrying @public @buseventtype create-map-schema MyInputEvent (8 SupportGenericColUtil parameterized columns), @name('infra') keepall MyInfra named window and 'on MyInputEvent merge MyInfra insert select <8 names>': the types record pins the eight declared java.util.-qualified type tokens, the sample map send inserts one row and the iterator snapshot returns all eight columns (Optional/parameterized generic metadata has no Go schema surface; the Go schema pins element/container reflect types, so EPType generic-arg fidelity is documented in the record note)",
	"deploy+types+send+snapshot+undeploy; a single module deploy carrying the same @public @buseventtype schema plus an unkeyed MyInfra table over the 8 parameterized columns fed by the merge insert: the types record pins the declared type tokens, the sample map send inserts one row and the iterator snapshot returns all eight columns (same Optional/parameterized downgrade note as the window leg)",
	"unrepresentable+deploy+unrepresentable; four SODA eplToModel round-trips run before the @public SupportSpatialPoint(id,px,py,category) keepall MyWindow deploy (matching the Java source order - the round-trips are syntax-only), then five invalid create-index compile probes (empty expression list, expression column, multi-expression list, dotted column, unknown advanced type) run against the deployed path, their Java messages pinned; the Go runner exercises the typed create-index validation equivalents live where a form exists (empty columns, unknown-column, invalid kind) and records the EPL-text-only probes without a Go counterpart",
}

// infraNWTableCreateDDL563CaseSpec carries the per-case fixture constants:
// the Java execution family (create-DDL window/table leg vs the
// advanced-syntax case), whether the named-window or table variant runs,
// and the byte-exact module text Java passes to compileDeploy.
type infraNWTableCreateDDL563CaseSpec struct {
	indexSyntax bool
	namedWindow bool
	module      string
}

var infraNWTableCreateDDL563CaseSpecs = map[string]infraNWTableCreateDDL563CaseSpec{
	"generic-col-window": {
		namedWindow: true,
		module:      infraNWTableCreateDDL563ModuleWindow,
	},
	"generic-col-table": {
		module: infraNWTableCreateDDL563ModuleTable,
	},
	"index-syntax": {
		indexSyntax: true,
	},
}

// infraNWTableCreateDDL563Columns mirrors SupportGenericColUtil columnSetup
// order: each entry pins the column name, the declared EPL type token
// (asserted by assertPropertyEPTypes on the Java side) and the Go
// reflect.Type that carries the same container/element shape. Java's
// Optional<...> element wraps Integer, so the Go mapping is *[]int32; the
// parameterized generic-arg metadata itself has no Go schema surface and
// rides the types-record note.
var infraNWTableCreateDDL563Columns = []struct {
	name    string
	eplType string
	goType  reflect.Type
}{
	{"listOfString", "java.util.List<String>", reflect.TypeOf([]string{})},
	{"listOfOptionalInteger", "java.util.List<Optional<Integer>>", reflect.TypeOf([]*int32{})},
	{"mapOfStringAndInteger", "java.util.Map<String, Integer>", reflect.TypeOf(map[string]int32{})},
	{"listArrayOfString", "java.util.List<String>[]", reflect.TypeOf([][]string{})},
	{"listOfStringArray", "java.util.List<String[]>", reflect.TypeOf([][]string{})},
	{"listArray2DimOfString", "java.util.List<String>[][]", reflect.TypeOf([][][]string{})},
	{"listOfStringArray2Dim", "java.util.List<String[][]>", reflect.TypeOf([][][]string{})},
	{"listOfT", "java.util.List<Object>", reflect.TypeOf([]any{})},
}

// infraNWTableCreateDDL563FieldSpecs builds the shared map-schema field
// list used by both the MyInputEvent registration and the inline MyInfra
// window schema.
func infraNWTableCreateDDL563FieldSpecs() []esper.FieldSpec {
	fields := make([]esper.FieldSpec, 0, len(infraNWTableCreateDDL563Columns))
	for _, column := range infraNWTableCreateDDL563Columns {
		fields = append(fields, esper.FieldDef(column.name, column.goType))
	}
	return fields
}

// infraNWTableCreateDDL563SnapshotFields is the sorted field list pinned on
// the iterator snapshots (unordered map events carry the 8 columns).
var infraNWTableCreateDDL563SnapshotFields = []string{
	"listArray2DimOfString",
	"listArrayOfString",
	"listOfOptionalInteger",
	"listOfString",
	"listOfStringArray",
	"listOfStringArray2Dim",
	"listOfT",
	"mapOfStringAndInteger",
}

// infraNWTableCreateDDL563Pin is one unrepresentable record's pinned probe:
// the byte-exact EPL text plus the recorded note (the SODA EPL-text pin or
// the Java compile-error message plus its typed-API mapping note).
type infraNWTableCreateDDL563Pin struct {
	epl  string
	note string
}

// infraNWTableCreateDDL563UnrepresentablePins pins the AdvancedSyntax probe
// texts and record notes. The SODA probes pin the asserted EPL text itself
// (Java asserts epl == model.toEPL()); the invalid probes pin the Java
// EPCompileExceptionItemMultiPart message text plus the nearest typed
// create-index equivalent exercised on the Go side.
var infraNWTableCreateDDL563UnrepresentablePins = map[string]infraNWTableCreateDDL563Pin{
	"soda-index-advanced-args": {
		epl: infraNWTableCreateDDL563SODAAdvancedArgs,
		note: `soda-eplToModel: create index MyIndex on MyWindow((x,y) dummy_name("a",10101)) - ` +
			"Java eplToModel parses the statement text into a statement object model and " +
			"assertEquals(epl, model.toEPL()) round-trips it; the Go surface has no EPL-text " +
			"parser or statement object model, so the create-index syntax round-trip is pinned " +
			"text with no Go counterpart",
	},
	"soda-index-single-named-type": {
		epl: infraNWTableCreateDDL563SODASingleType,
		note: "soda-eplToModel: create index MyIndex on MyWindow(x dummy_name) - same SODA " +
			"round-trip; single-expression advanced-type indexes have no typed Go " +
			"create-index form",
	},
	"soda-index-multi-col-named-type": {
		epl: infraNWTableCreateDDL563SODAMultiColType,
		note: "soda-eplToModel: create index MyIndex on MyWindow((x,y,z) dummy_name) - same " +
			"SODA round-trip; multi-expression advanced-type lists have no typed Go " +
			"create-index form",
	},
	"soda-index-mixed-columns": {
		epl: infraNWTableCreateDDL563SODAMixedCols,
		note: `soda-eplToModel: create index MyIndex on MyWindow(x dummy_name, (y,z) ` +
			`dummy_name_2("a"), p dummyname3) - same SODA round-trip; mixed ` +
			"column-list/named-type index forms have no Go equivalent",
	},
	"invalid-empty-expr-list": {
		epl: infraNWTableCreateDDL563InvalidEmptyList,
		note: "Invalid empty list of index expressions - Go equivalent: " +
			"NamedWindow.CreateIndex rejects the empty column list (InvalidRule)",
	},
	"invalid-expression": {
		epl: infraNWTableCreateDDL563InvalidExprCol,
		note: "Invalid index expression 'intPrimitive+1' - Go equivalent: " +
			"NamedWindow.CreateIndex rejects 'intPrimitive+1' as an unknown column name " +
			"(UnknownName); expression columns have no Go form",
	},
	"invalid-multi-expr-list": {
		epl: infraNWTableCreateDDL563InvalidMulti,
		note: "Invalid multiple index expressions - Go equivalent: " +
			"NamedWindow.CreateIndex on the typed column list (x,y) rejects 'x'/'y' as " +
			"unknown columns (UnknownName)",
	},
	"invalid-dotted-expr": {
		epl: infraNWTableCreateDDL563InvalidDotted,
		note: "Invalid index expression 'x.y' - Go equivalent: NamedWindow.CreateIndex " +
			"rejects 'x.y' as an unknown column name (UnknownName)",
	},
	"invalid-advanced-type": {
		epl: infraNWTableCreateDDL563InvalidAdvType,
		note: "Unrecognized advanced-type index 'xxxx' - Go equivalent: the typed " +
			"create-index API carries no named index-type surface; the closest typed " +
			"probe is the invalid-Kind rejection (InvalidRule)",
	},
}

// infraNWTableCreateDDL563IndexProbes maps each invalid-compile record to
// the typed Go create-index probe that exercises the nearest validation
// boundary: the empty-expression-list probe maps to a nil column list,
// every expression-shaped column maps to the unknown-column rejection on
// the typed column list, and the unknown advanced-type name maps to an
// unrecognized IndexKind. Probes are verified against MyWindow before the
// record is emitted; zero catalog indexes must remain afterwards.
var infraNWTableCreateDDL563IndexProbes = map[string]struct {
	name    string
	columns []string
	kind    esper.IndexKind
	want    esper.ErrorCode
}{
	"invalid-empty-expr-list": {name: "MyIndex", columns: nil, kind: esper.IndexHash, want: esper.ErrorInvalidRule},
	"invalid-expression":      {name: "MyIndex", columns: []string{"intPrimitive+1"}, kind: esper.IndexHash, want: esper.ErrorUnknownName},
	"invalid-multi-expr-list": {name: "MyIndex", columns: []string{"x", "y"}, kind: esper.IndexHash, want: esper.ErrorUnknownName},
	"invalid-dotted-expr":     {name: "MyIndex", columns: []string{"x.y"}, kind: esper.IndexHash, want: esper.ErrorUnknownName},
	"invalid-advanced-type":   {name: "MyIndex", columns: []string{"id"}, kind: esper.IndexKind(99), want: esper.ErrorInvalidRule},
}

// infraNWTableCreateDDL563CaseState carries the per-case replay state: the
// environment/engine pair, the label→deployments bookkeeping used by
// undeploy-all, the deployed-label set for marker checks and the deployed
// infra statement the iterator snapshot reads.
type infraNWTableCreateDDL563CaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	spec           infraNWTableCreateDDL563CaseSpec
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	statements     map[string]*esper.Statement
	caseName       string
}

// runInfraNWTableCreateDDL563Scenario replays the three create-DDL
// executions: each case runs on a fresh environment/engine pair (one
// runtime per Java execution) and every step dispatches to the matching
// runtime action. The oracle emits the epoch time for every timed record,
// so the runner pins the same value.
func runInfraNWTableCreateDDL563Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraNWTableCreateDDL563Cases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraNWTableCreateDDL563Case(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraNWTableCreateDDL563ID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraNWTableCreateDDL563ID)
	}
	return trace, nil
}

func runInfraNWTableCreateDDL563Case(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	spec, ok := infraNWTableCreateDDL563CaseSpecs[caseName]
	if !ok {
		return compat.Trace{}, fmt.Errorf("%s: unknown case %q", infraNWTableCreateDDL563ID, caseName)
	}
	env := esper.NewEnvironment()
	if spec.indexSyntax {
		if _, err := esper.RegisterStruct[infraNWTableCreateDDL563Bean](env, "SupportSpatialPoint"); err != nil {
			return compat.Trace{}, err
		}
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraNWTableCreateDDL563JavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	state := &infraNWTableCreateDDL563CaseState{
		env:            env,
		engine:         engine,
		spec:           spec,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		statements:     make(map[string]*esper.Statement),
		caseName:       caseName,
	}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWTableCreateDDL563ID}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return compat.Trace{}, err
		}
		switch step.Op {
		case "case":
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "deployed":
			if !state.deployedLabels[step.Statement] {
				return compat.Trace{}, fmt.Errorf("%s: deployed marker for unknown statement %q",
					infraNWTableCreateDDL563ID, step.Statement)
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "deployed",
				Statement: step.Statement,
				Sequence:  1,
				Time:      "1970-01-01T00:00:00Z",
			})
		case "types":
			if err := state.types(step, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "send":
			event, err := decodeInfraNWTableCreateDDL563Payload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, err
			}
		case "snapshot":
			if err := state.snapshot(ctx, step, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "unrepresentable":
			if err := state.unrepresentable(step, &trace); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraNWTableCreateDDL563ID, step.Op)
		}
	}
	return trace, nil
}

// deploy maps each scenario label to the equivalent Go replay: the
// create-case "module" step mirrors the single env.compileDeploy(module)
// — env-level MyInputEvent registration plus one DeployPlans deployment
// carrying the window/table, the named "infra" statement and the merge
// feed (Java's module-internal MyInfra is not @public, so the merge must
// share the deployment) — and the index-syntax "window" step registers
// the @public keepall MyWindow. The byte-exact module text Java passes
// to compileDeploy is pinned by the loader's step keys.
func (s *infraNWTableCreateDDL563CaseState) deploy(ctx context.Context, step compat.Step) error {
	if s.spec.indexSyntax {
		if step.Statement != "window" || step.Epl != infraNWTableCreateDDL563CreateWindowAdv {
			return s.deployDrift(step)
		}
		schema, ok := s.env.Schema("SupportSpatialPoint")
		if !ok {
			return fmt.Errorf("%s: SupportSpatialPoint schema is missing", infraNWTableCreateDDL563ID)
		}
		if _, err := esper.CreateNamedWindow(s.env, "MyWindow", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return err
		}
		s.deployedLabels[step.Statement] = true
		return nil
	}
	if step.Statement != "module" || step.Epl != s.spec.module {
		return s.deployDrift(step)
	}
	return s.deployModule(ctx, step.Statement)
}

func (s *infraNWTableCreateDDL563CaseState) deployDrift(step compat.Step) error {
	return fmt.Errorf("%s: deploy %q does not pin the expected EPL %q",
		infraNWTableCreateDDL563ID, step.Statement, step.Epl)
}

// deployModule mirrors the single env.compileDeploy(module) call of
// InfraCreateGenericColType.run: `@public @buseventtype create schema
// MyInputEvent(...)` registers the bus-visible map schema env-level;
// `@name('infra')create window MyInfra#keepall as (...)` or
// `@name('infra')create table MyInfra as (...)` creates the catalog
// object (the inline Java column list becomes an inline map schema);
// `on MyInputEvent merge MyInfra insert select <8 names>` deploys an
// insert-only merge whose select maps to CopyMatchingFields. Java's
// assertStatement("infra") reads the create statement's event type and
// env.iterator("infra") reads its rows: Go carries no iterator on the
// env-level catalog create, so the named "infra" statement rides
// CreateNamedWindowQuery (window leg) or a passive FromTable query
// (table leg). Both statements deploy together via DeployPlans,
// mirroring the single Java deployment.
func (s *infraNWTableCreateDDL563CaseState) deployModule(ctx context.Context, label string) error {
	if _, err := esper.RegisterMap(s.env, "MyInputEvent",
		infraNWTableCreateDDL563FieldSpecs(), esper.BusEventType()); err != nil {
		return err
	}
	var infraPlan, mergePlan esper.Plan
	if s.spec.namedWindow {
		schema, schemaErr := esper.NewMapSchema("MyInfra", infraNWTableCreateDDL563FieldSpecs())
		if schemaErr != nil {
			return schemaErr
		}
		if _, err := esper.CreateNamedWindow(s.env, "MyInfra", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return err
		}
		var err error
		infraPlan, err = s.env.Build(esper.FromNamedWindow(s.env, "MyInfra").
			CreateNamedWindowQuery(esper.StatementName("infra")))
		if err != nil {
			return err
		}
		mergePlan, err = s.env.Build(esper.OnRecord(esper.FromAny(s.env, "MyInputEvent")).
			MergeInsertIntoNamedWindow("MyInfra", nil, esper.CopyMatchingFields()).Query())
		if err != nil {
			return err
		}
	} else {
		columns := make([]esper.TableColumn, 0, len(infraNWTableCreateDDL563Columns))
		for _, column := range infraNWTableCreateDDL563Columns {
			columns = append(columns, esper.TableColumn{Name: column.name, Type: column.goType})
		}
		if _, err := esper.CreateTable(s.env, "MyInfra", columns); err != nil {
			return err
		}
		var err error
		infraPlan, err = s.env.Build(esper.FromTable(s.env, "MyInfra").
			Query(esper.StatementName("infra")))
		if err != nil {
			return err
		}
		mergePlan, err = s.env.Build(esper.OnRecord(esper.FromAny(s.env, "MyInputEvent")).
			MergeInsertIntoTable("MyInfra", nil, esper.CopyMatchingFields()).Query())
		if err != nil {
			return err
		}
	}
	deployment, err := s.engine.DeployPlans(ctx, []esper.Plan{infraPlan, mergePlan})
	if err != nil {
		return err
	}
	s.deployments[label] = append(s.deployments[label], deployment)
	s.deployedLabels[label] = true
	for _, statement := range deployment.Statements() {
		s.statements[statement.Name()] = statement
	}
	return nil
}

// types mirrors the assertStatement infra-type assert: Java's
// SupportGenericColUtil.assertPropertyEPTypes checks every descriptor's
// EPType against NAMESANDTYPES plus the indexed/mapped flags. The Go
// schema carries no parameterized/Optional EPType metadata (the generic
// args are lost at registration), so the record pins the Java-asserted
// EPL type token per column while the runner asserts the mapped
// container/element reflect.Type, the non-optional flag and the
// derived indexed/mapped flags — documented downgrade, not a silent one.
func (s *infraNWTableCreateDDL563CaseState) types(step compat.Step, trace *compat.Trace) error {
	if step.Statement != "infra" {
		return fmt.Errorf("%s: types step %q is not pinned", infraNWTableCreateDDL563ID, step.Statement)
	}
	var schema esper.Schema
	if s.spec.namedWindow {
		window, ok := s.env.NamedWindow("MyInfra")
		if !ok {
			return fmt.Errorf("%s: named window MyInfra is missing", infraNWTableCreateDDL563ID)
		}
		schema = window.Schema()
	} else {
		table, ok := s.env.Table("MyInfra")
		if !ok {
			return fmt.Errorf("%s: table MyInfra is missing", infraNWTableCreateDDL563ID)
		}
		schema = table.Schema()
	}
	var entries []map[string]any
	for _, column := range infraNWTableCreateDDL563Columns {
		got, ok := schema.PropertyType(column.name)
		if !ok {
			return fmt.Errorf("%s: infra schema is missing property %q",
				infraNWTableCreateDDL563ID, column.name)
		}
		if got != column.goType {
			return fmt.Errorf("%s: property %q mapped to %v, want %v",
				infraNWTableCreateDDL563ID, column.name, got, column.goType)
		}
		if spec, ok := schema.Field(column.name); ok && spec.Optional {
			return fmt.Errorf("%s: property %q must not be optional (Java pins the column value type)",
				infraNWTableCreateDDL563ID, column.name)
		}
		indexed := got.Kind() == reflect.Slice || got.Kind() == reflect.Array
		mapped := got.Kind() == reflect.Map
		entries = append(entries, map[string]any{
			"name":    column.name,
			"type":    column.eplType,
			"indexed": indexed,
			"mapped":  mapped,
		})
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "types",
		Statement: step.Statement,
		Sequence:  0,
		Time:      "1970-01-01T00:00:00Z",
		Value:     entries,
	})
	return nil
}

// snapshot mirrors the env.iterator("infra") assert: it snapshots the
// deployed infra statement (window: CreateNamedWindowQuery; table: passive
// FromTable query) and pins the single merged row — a defensive size check
// guards the Java assertEventProp expectations even though the -diff path
// never compares Go records.
func (s *infraNWTableCreateDDL563CaseState) snapshot(ctx context.Context, step compat.Step,
	trace *compat.Trace) error {
	if step.Statement != "infra" {
		return fmt.Errorf("%s: snapshot step %q is not pinned", infraNWTableCreateDDL563ID, step.Statement)
	}
	statement, ok := s.statements["infra"]
	if !ok {
		return fmt.Errorf("%s: snapshot statement %q was not deployed", infraNWTableCreateDDL563ID, step.Statement)
	}
	result, err := statement.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("%s: snapshot %q: %w", infraNWTableCreateDDL563ID, step.Statement, err)
	}
	rows := compat.NormalizeResults(result.Batch.New)
	if len(rows) != 1 {
		return fmt.Errorf("%s: iterator returned %d rows, want the single merged row",
			infraNWTableCreateDDL563ID, len(rows))
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "snapshot",
		Statement: step.Statement,
		Sequence:  0,
		Time:      "1970-01-01T00:00:00Z",
		New:       rows,
	})
	return nil
}

// unrepresentable emits the pinned plan-only records of the AdvancedSyntax
// case: the four SODA eplToModel round-trips have no Go statement object
// model, so the asserted EPL text is recorded verbatim. The five invalid
// create-index probes additionally exercise the nearest typed validation
// boundary against MyWindow (empty name/columns → InvalidRule,
// non-column-name → UnknownName) and assert zero catalog indexes remain
// before the record is emitted.
func (s *infraNWTableCreateDDL563CaseState) unrepresentable(step compat.Step,
	trace *compat.Trace) error {
	pin, ok := infraNWTableCreateDDL563UnrepresentablePins[step.Statement]
	if !ok {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWTableCreateDDL563ID, step.Statement)
	}
	if step.Epl != pin.epl || step.ExpectError != pin.note {
		return fmt.Errorf("%s: unrepresentable step %q is not pinned",
			infraNWTableCreateDDL563ID, step.Statement)
	}
	if probe, found := infraNWTableCreateDDL563IndexProbes[step.Statement]; found {
		window, ok := s.engine.NamedWindow("MyWindow")
		if !ok {
			return fmt.Errorf("%s: named window MyWindow is missing", infraNWTableCreateDDL563ID)
		}
		err := window.CreateIndex(probe.name, probe.columns, probe.kind, false)
		if err == nil {
			return fmt.Errorf("%s: create-index probe %q unexpectedly succeeded",
				infraNWTableCreateDDL563ID, step.Statement)
		}
		if !errors.Is(err, probe.want) {
			return fmt.Errorf("%s: create-index probe %q rejected with %v, want code %s",
				infraNWTableCreateDDL563ID, step.Statement, err, probe.want)
		}
		if len(window.Definition().Indexes()) != 0 {
			return fmt.Errorf("%s: create-index probe %q left catalog indexes behind",
				infraNWTableCreateDDL563ID, step.Statement)
		}
	}
	trace.Records = append(trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "unrepresentable",
		Statement: step.Statement,
		Value:     step.ExpectError,
	})
	return nil
}

func (s *infraNWTableCreateDDL563CaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	s.deployments = make(map[string][]*esper.Deployment)
	s.deployedLabels = make(map[string]bool)
	s.statements = make(map[string]*esper.Statement)
	return nil
}

// decodeInfraNWTableCreateDDL563Payload mirrors SupportGenericColUtil
// .sampleEvent: the payload carries the eight columns as typed nested
// values; the runner decodes each field into the registered Go container
// shape (Optional<Integer> elements ride *int32, mirroring the Java
// Optional-boxed element).
func decodeInfraNWTableCreateDDL563Payload(step compat.Step) (any, error) {
	if step.EventType != "MyInputEvent" {
		return nil, fmt.Errorf("unsupported infra nwtable create-ddl event type %q", step.EventType)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(step.Payload, &raw); err != nil {
		return nil, fmt.Errorf("decode MyInputEvent: %w", err)
	}
	if len(raw) != len(infraNWTableCreateDDL563Columns) {
		return nil, fmt.Errorf("decode MyInputEvent: want %d fields, got %d",
			len(infraNWTableCreateDDL563Columns), len(raw))
	}
	event := make(map[string]any, len(raw))
	for _, column := range infraNWTableCreateDDL563Columns {
		field, ok := raw[column.name]
		if !ok {
			return nil, fmt.Errorf("decode MyInputEvent: missing field %q", column.name)
		}
		value, err := decodeInfraNWTableCreateDDL563Field(column.name, field)
		if err != nil {
			return nil, err
		}
		event[column.name] = value
	}
	return event, nil
}

func decodeInfraNWTableCreateDDL563Field(name string, raw json.RawMessage) (any, error) {
	switch name {
	case "listOfString":
		var value []string
		return value, json.Unmarshal(raw, &value)
	case "listOfOptionalInteger":
		var value []*int32
		return value, json.Unmarshal(raw, &value)
	case "mapOfStringAndInteger":
		var value map[string]int32
		return value, json.Unmarshal(raw, &value)
	case "listArrayOfString", "listOfStringArray":
		var value [][]string
		return value, json.Unmarshal(raw, &value)
	case "listArray2DimOfString", "listOfStringArray2Dim":
		var value [][][]string
		return value, json.Unmarshal(raw, &value)
	case "listOfT":
		var value []any
		return value, json.Unmarshal(raw, &value)
	}
	return nil, fmt.Errorf("decode MyInputEvent: unknown field %q", name)
}

// loadInfraNWTableCreateDDL563Scenario enforces the strict scenario
// contract shared by the differential runners: no duplicate or unknown
// JSON fields, pinned metadata, pinned per-case runtime/execution/EPL, and
// a per-op step field whitelist followed by a full step-shape pin.
func loadInfraNWTableCreateDDL563Scenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWTableCreateDDL563ID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWTableCreateDDL563ID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableCreateDDL563ID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWTableCreateDDL563ID, err)
	}
	if err := requireInfraNWTableCreateDDL563Fields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWTableCreateDDL563ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWTableCreateDDL563ID ||
		metadata.Description != infraNWTableCreateDDL563Description ||
		metadata.JavaCommit != infraNWTableCreateDDL563JavaCommit ||
		metadata.JavaSource != infraNWTableCreateDDL563JavaSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWTableCreateDDL563ID)
	}
	if err := validateInfraNWTableCreateDDL563StringArray(root["javaSourceFiles"], infraNWTableCreateDDL563JavaSources, "javaSourceFiles"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableCreateDDL563StringArray(root["javaRuntimes"], infraNWTableCreateDDL563JavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableCreateDDL563StringArray(root["javaNames"], infraNWTableCreateDDL563JavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableCreateDDL563StringArray(root["javaStaticIds"], infraNWTableCreateDDL563JavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWTableCreateDDL563StringArray(root["javaFlags"], infraNWTableCreateDDL563JavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWTableCreateDDL563Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWTableCreateDDL563ID, len(infraNWTableCreateDDL563Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWTableCreateDDL563Fields(object,
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
		if definition.Case != infraNWTableCreateDDL563Cases[index] ||
			definition.Ordinal != infraNWTableCreateDDL563Ordinals[index] ||
			definition.RuntimeID != infraNWTableCreateDDL563JavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWTableCreateDDL563JavaExecutions[index] ||
			definition.Observation != infraNWTableCreateDDL563CaseObservations[index] ||
			definition.EPL != infraNWTableCreateDDL563CaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWTableCreateDDL563ID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWTableCreateDDL563ID)
	}
	steps := make([]compat.Step, len(rawSteps))
	objects := make([]map[string]json.RawMessage, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		objects[index] = object
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "types":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWTableCreateDDL563Payload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case", "statement", "mode", "fields"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "unrepresentable":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case", "statement", "epl", "expectError"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWTableCreateDDL563Fields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		default:
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unsupported op %q", index, operation)
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d case: %w", index, err)
		}
		found := false
		for _, name := range infraNWTableCreateDDL563Cases {
			if stepCase == name {
				found = true
				break
			}
		}
		if !found {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has unknown case %q", index, stepCase)
		}
		if err := json.Unmarshal(rawStep, &steps[index]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d has invalid types: %w", index, err)
		}
	}
	scenario := compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}
	if err := validateInfraNWTableCreateDDL563RawSteps(rawSteps, objects, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

// validateInfraNWTableCreateDDL563RawSteps pins the complete step sequence
// per case against the raw JSON objects: deploy steps with byte-exact EPL,
// deployed markers, the infra types assert, the MyInputEvent sample send,
// the iterator snapshot, the SODA/invalid unrepresentable records and the
// undeploy-all terminators. Java milestone(0) carries no step.
func validateInfraNWTableCreateDDL563RawSteps(rawSteps []json.RawMessage, objects []map[string]json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWTableCreateDDL563Cases {
		want, ok := infraNWTableCreateDDL563CaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWTableCreateDDL563ID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWTableCreateDDL563ID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableCreateDDL563ID, offset, caseName)
		}
		var head string
		if err := json.Unmarshal(objects[offset]["case"], &head); err != nil || head != caseName {
			return fmt.Errorf("%s step %d must open case %q", infraNWTableCreateDDL563ID, offset, caseName)
		}
		offset++
		for index, pinned := range want {
			actual, err := infraNWTableCreateDDL563StepKey(objects[offset+index], operations[offset+index])
			if err != nil {
				return fmt.Errorf("%s case %q step %d: %w", infraNWTableCreateDDL563ID, caseName, index+1, err)
			}
			if actual != pinned {
				return fmt.Errorf("%s case %q step %d is not pinned: got %q want %q",
					infraNWTableCreateDDL563ID, caseName, index+1, actual, pinned)
			}
		}
		offset += len(want)
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario has trailing steps", infraNWTableCreateDDL563ID)
	}
	return nil
}

// infraNWTableCreateDDL563StepKey renders a raw step object into its
// pinned string form. Fields are read from the raw JSON because
// compat.Step does not carry the fields array.
func infraNWTableCreateDDL563StepKey(object map[string]json.RawMessage, operation string) (string, error) {
	stringField := func(name string) (string, error) {
		var value string
		if err := json.Unmarshal(object[name], &value); err != nil {
			return "", fmt.Errorf("step field %q must be a string", name)
		}
		return value, nil
	}
	switch operation {
	case "deploy":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		return "deploy:" + statement + ":" + epl, nil
	case "deployed":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		return operation + ":" + statement, nil
	case "types":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		return "types:" + statement, nil
	case "send":
		eventType, err := stringField("eventType")
		if err != nil {
			return "", err
		}
		var payload map[string]any
		if err := json.Unmarshal(object["payload"], &payload); err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		canonical, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("send payload: %w", err)
		}
		return "send:" + eventType + ":" + string(canonical), nil
	case "snapshot":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		mode, err := stringField("mode")
		if err != nil {
			return "", err
		}
		var fields []string
		if err := json.Unmarshal(object["fields"], &fields); err != nil {
			return "", fmt.Errorf("step fields must be a string array")
		}
		return "snapshot:" + statement + ":" + mode + ":" + joinStrings(fields, ","), nil
	case "unrepresentable":
		statement, err := stringField("statement")
		if err != nil {
			return "", err
		}
		epl, err := stringField("epl")
		if err != nil {
			return "", err
		}
		note, err := stringField("expectError")
		if err != nil {
			return "", err
		}
		return "unrepresentable:" + statement + ":" + epl + ":" + note, nil
	case "undeploy-all":
		return "undeploy-all", nil
	}
	return "", fmt.Errorf("unsupported op %q", operation)
}

// createDDL563CaseSteps renders the pinned step sequence of
// InfraCreateGenericColType.run (InfraNWTableCreate.java lines 40-58):
// the single module deploy, the infra EPType assert, the sample map
// send, the iterator snapshot and undeployAll. Java milestone(0)
// carries no step.
func createDDL563CaseSteps(spec infraNWTableCreateDDL563CaseSpec) []string {
	return []string{
		"deploy:module:" + spec.module,
		"deployed:module",
		"types:infra",
		`send:MyInputEvent:{"listArray2DimOfString":[[["b"]]],"listArrayOfString":[["b"]],"listOfOptionalInteger":[10],"listOfString":["a"],"listOfStringArray":[["c"]],"listOfStringArray2Dim":[[["c"]]],"listOfT":["x"],"mapOfStringAndInteger":{"k":20}}`,
		"snapshot:infra:unordered:" + joinStrings(infraNWTableCreateDDL563SnapshotFields, ","),
		"undeploy-all",
	}
}

// indexSyntax563CaseSteps renders the pinned step sequence of
// InfraNWTableCreateIndexAdvancedSyntax.run (lines 21-44) in source
// order: the four SODA eplToModel round-trips first (syntax-only, run
// before the window deploy), then the @public window deploy, then the
// five tryInvalidCompile probes, each carried as an unrepresentable
// record.
func indexSyntax563CaseSteps() []string {
	steps := make([]string, 0, 12)
	for _, label := range []string{
		"soda-index-advanced-args",
		"soda-index-single-named-type",
		"soda-index-multi-col-named-type",
		"soda-index-mixed-columns",
	} {
		pin := infraNWTableCreateDDL563UnrepresentablePins[label]
		steps = append(steps, "unrepresentable:"+label+":"+pin.epl+":"+pin.note)
	}
	steps = append(steps,
		"deploy:window:"+infraNWTableCreateDDL563CreateWindowAdv,
		"deployed:window")
	for _, label := range []string{
		"invalid-empty-expr-list",
		"invalid-expression",
		"invalid-multi-expr-list",
		"invalid-dotted-expr",
		"invalid-advanced-type",
	} {
		pin := infraNWTableCreateDDL563UnrepresentablePins[label]
		steps = append(steps, "unrepresentable:"+label+":"+pin.epl+":"+pin.note)
	}
	return append(steps, "undeploy-all")
}

// infraNWTableCreateDDL563CaseSteps pins the exact op sequence per case.
var infraNWTableCreateDDL563CaseSteps = map[string][]string{
	"generic-col-window": createDDL563CaseSteps(infraNWTableCreateDDL563CaseSpecs["generic-col-window"]),
	"generic-col-table":  createDDL563CaseSteps(infraNWTableCreateDDL563CaseSpecs["generic-col-table"]),
	"index-syntax":       indexSyntax563CaseSteps(),
}

func requireInfraNWTableCreateDDL563Fields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWTableCreateDDL563StringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if !reflect.DeepEqual(values, expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	return nil
}
