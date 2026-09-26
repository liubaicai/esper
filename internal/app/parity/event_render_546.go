package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

// Draft-4.546 runner for the seven unreferenced EventRender* executions:
//
//   - EventRender$EventRenderPropertyCustomRenderer (SERDEREQUIRED): the
//     JSONRenderingOptions/XMLRenderingOptions setRenderer SPI has no Go
//     boundary, so both renders pin their whitespace-stripped Java output as
//     unrepresentable records; the oracle still runs the real assertions.
//   - EventRender$EventRenderObjectArray: object-array select-star over
//     MyObjectArrayType. The Go schema registers fields in Java RENDER order
//     (p0,p1,p3,p4,p2: Esper's renderer emits fragment/nested properties last)
//     and the send carries values in the same order, so the JSON render is
//     byte-exact. The XML render is unrepresentable: Go renders the Double
//     column as "3" (Java "3.0") and emits <p01></p01> empty elements for the
//     nested bean's null String fields where Java drops them entirely.
//   - EventRender$EventRenderPOJOMap (EXCLUDEWHENINSTRUMENTED): map-typed
//     bean property over SupportBeanRendererOne plus the undeclared-Map
//     SupportBeanRendererThree JSON repeat. JSON and default-element XML are
//     byte-exact; the defaultAsAttribute render is unrepresentable because
//     Java drops null map entries and self-closes the mapped element while Go
//     emits efg="" and an explicit </stringObjectMap> close tag.
//   - EventRenderJSON$EventRenderEmptyMap: props null vs {} vs {a:b} JSON
//     renders via select-star capture.
//   - EventRenderJSON$EventRenderEnquote / EventRenderXML$EventRenderEnquote:
//     pure OutputValueRenderer*String helpers with no engine surface; each
//     row pins input->output as an unrepresentable record.
//   - EventRenderXML$EventRenderSQLDate (ESPER-469): computed-column select
//     projects a DateOnly literal; the XML render and the property value are
//     pinned as value records.
//
// Rendered strings are pinned with the same full-whitespace strip the 545
// runner uses (render output contains only insignificant whitespace), while
// the oracle additionally asserts Java's removeNewline comparison against the
// verbatim expected strings from the regression sources.
const eventRender546JavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
const eventRender546ID = "event-render-546"
const eventRender546Source = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render"
const eventRender546Description = "Event render parity (7 executions): custom EventPropertyRenderer SPI pinned unrepresentable, object-array nested-last ordering with Double/long lexemes, POJO map JSON/XML/attribute renders, empty-map JSON three-way, JSON/XML enquote helper pins, sql-date XML render."

var eventRender546JavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRender.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRender.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRender.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRenderJSON.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRenderJSON.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRenderXML.java",
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/render/EventRenderXML.java",
}

var eventRender546JavaRuntimeIDs = []string{
	"java-runtime-26bb69572227aa67230c",
	"java-runtime-eacece93880bd4448b7d",
	"java-runtime-6af1376411de38cffefe",
	"java-runtime-96f1450787b11db671fd",
	"java-runtime-8cce94662734d4052c0e",
	"java-runtime-f47d81ef0d1a48b66476",
	"java-runtime-539c60ae3344b3001b44",
}

var eventRender546JavaExecutions = []string{
	"EventRenderPropertyCustomRenderer",
	"EventRenderObjectArray",
	"EventRenderPOJOMap",
	"EventRenderEmptyMap",
	"EventRenderEnquote",
	"EventRenderSQLDate",
	"EventRenderEnquote",
}

var eventRender546JavaStaticIDs = []string{
	"java-037e8856e0079705578b",
	"java-037e8856e0079705578b",
	"java-037e8856e0079705578b",
	"java-049ac5adb236d6173921",
	"java-049ac5adb236d6173921",
	"java-34444bf4f27c5d12a7a1",
	"java-34444bf4f27c5d12a7a1",
}

var eventRender546JavaFlags = []string{"SERDEREQUIRED", "EXCLUDEWHENINSTRUMENTED"}

var eventRender546Cases = []string{
	"custom-renderer",
	"object-array",
	"pojo-map",
	"empty-map",
	"enquote-json",
	"sqldate",
	"enquote-xml",
}

var eventRender546CaseOrdinals = []int{0, 1, 2, 2, 3, 2, 3}

// Byte-exact EPLs pinned from the Java regression sources. The enquote cases
// have no EPL; the pinned text documents the internal helper under test.
var eventRender546CaseEPLs = map[string]string{
	"custom-renderer": "@name('s0') select * from MyRendererEvent",
	"object-array":    "@name('s0') select * from MyObjectArrayType",
	"pojo-map":        "@name('s0') select * from SupportBeanRendererOne\n@name('s0') select * from SupportBeanRendererThree",
	"empty-map":       "@name('s0') select * from EmptyMapEvent",
	"enquote-json":    "OutputValueRendererJSONString.enquote(input, buf)",
	"sqldate":         "@name('s0') select java.sql.Date.valueOf(\"2010-01-31\") as mySqlDate from SupportBean",
	"enquote-xml":     "OutputValueRendererXMLString.xmlEncode(input, buf, true)",
}

// Whitespace-stripped pins of the Java-expected render output, recorded by
// unrepresentable steps where the Go surface cannot produce the same bytes.
const er546CustomRendererJSONPin = `{"MyEvent":{"id":"id1","someProperties":["index#0=1;index#1=x","index#0=2;index#1=y"],"mappedProperty":{"key":"value"}}}`
const er546CustomRendererXMLPin = `<?xmlversion="1.0"encoding="UTF-8"?><MyEvent><id>id1</id><someProperties>index#0=1;index#1=x</someProperties><someProperties>index#0=2;index#1=y</someProperties><mappedProperty><key>value</key></mappedProperty></MyEvent>`
const er546ObjectArrayXMLPin = `<?xmlversion="1.0"encoding="UTF-8"?><MyEvent><p0>abc</p0><p1>1</p1><p3>2</p3><p4>3.0</p4><p2><id>1</id><p00>p00</p00></p2></MyEvent>`
const er546POJOMapXMLAttrPin = `<?xmlversion="1.0"encoding="UTF-8"?><MyEvent><stringObjectMapabc="def"def="123"/></MyEvent>`

// Java EventRenderJSON.EventRenderEnquote input -> expected enquote output.
var er546JSONEnquoteRows = [][2]string{
	{"\t", "\"\\t\""},
	{"\n", "\"\\n\""},
	{"\r", "\"\\r\""},
	{"\x00", "\"\\u0000\""},
}

// Java EventRenderXML.EventRenderEnquote input -> expected xmlEncode output.
var er546XMLEncodeRows = [][2]string{
	{"\"", "&quot;"},
	{"'", "&apos;"},
	{"&", "&amp;"},
	{"<", "&lt;"},
	{">", "&gt;"},
	{"\x00", "\\u0000"},
}

var er546Whitespace = regexp.MustCompile(`(\s|\n|\t)`)

// er546CaseState accumulates one case's records; sequences follow the parity
// convention (per-statement, per-operation counters starting at one).
type er546CaseState struct {
	caseName string
	trace    *compat.Trace
	seq      map[string]uint64
	now      string
}

func newER546CaseState(caseName string, trace *compat.Trace) *er546CaseState {
	return &er546CaseState{
		caseName: caseName,
		trace:    trace,
		seq:      map[string]uint64{},
		now:      compat.FormatTraceTime(time.Unix(0, 0).UTC()),
	}
}

func (s *er546CaseState) emit(operation, statement string, newRows []compat.ResultRecord, name string, value any) {
	key := statement + ":" + operation
	s.seq[key]++
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: operation,
		Statement: statement,
		Sequence:  s.seq[key],
		Time:      s.now,
		New:       newRows,
		Name:      name,
		Value:     value,
	})
}

func (s *er546CaseState) emitDeployed(statement string) {
	s.emit("deployed", statement, nil, "", nil)
}

func (s *er546CaseState) emitRow(statement string, fields map[string]any) {
	s.emit("listener", statement, []compat.ResultRecord{{Kind: "row", Fields: fields}}, "", nil)
}

func (s *er546CaseState) emitValue(statement, name string, value any) {
	s.emit("value", statement, nil, name, value)
}

func (s *er546CaseState) emitUnrepresentable(statement, name, note string) {
	s.emit("unrepresentable", statement, nil, name, note)
}

func (s *er546CaseState) resetSequences() {
	// undeployAll mirrors Java's per-module listener counters restarting.
	s.seq = map[string]uint64{}
}

// er546Bean types mirror the regression beans at the Go boundary.
type er546RendererEvent struct {
	ID             string         `esper:"id" json:"id"`
	SomeProperties [][]any        `esper:"someProperties" json:"someProperties"`
	MappedProperty map[string]any `esper:"mappedProperty" json:"mappedProperty"`
}

type er546BeanS0 struct {
	ID  int     `esper:"id" json:"id"`
	P00 string  `esper:"p00" json:"p00"`
	P01 *string `esper:"p01" json:"p01"`
	P02 *string `esper:"p02" json:"p02"`
	P03 *string `esper:"p03" json:"p03"`
}

type er546POJOMapEvent struct {
	StringObjectMap map[string]any `esper:"stringObjectMap" json:"stringObjectMap"`
}

type er546EmptyMapEvent struct {
	Props map[string]string `esper:"props" json:"props"`
}

type er546SupportBean struct {
	TheString string `esper:"theString" json:"theString"`
}

// er546Env carries one case's engine, deployments and captured output.
type er546Env struct {
	env         *esper.Environment
	engine      *esper.Engine
	deployments map[string]*esper.Deployment
	lastEvent   *esper.Event
	lastRow     *esper.Row
}

func newER546Env() *er546Env {
	env := esper.NewEnvironment()
	return &er546Env{
		env:         env,
		engine:      esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC())),
		deployments: map[string]*esper.Deployment{},
	}
}

// er546TypeName resolves the send/deploy event-type name for a case.
func er546TypeName(caseName, mode string) string {
	switch caseName {
	case "custom-renderer":
		return "MyRendererEvent"
	case "object-array":
		return "MyObjectArrayType"
	case "pojo-map":
		if mode == "three" {
			return "SupportBeanRendererThree"
		}
		return "SupportBeanRendererOne"
	case "empty-map":
		return "EmptyMapEvent"
	case "sqldate":
		return "SupportBean"
	}
	return ""
}

// er546Register declares the Go-side event types a case needs; it is
// idempotent per env and only called once per case run.
func er546Register(env *esper.Environment, caseName string) error {
	switch caseName {
	case "custom-renderer":
		_, err := esper.RegisterStruct[er546RendererEvent](env, "MyRendererEvent")
		return err
	case "object-array":
		nested, err := esper.RegisterStruct[er546BeanS0](env, "SupportBean_S0")
		if err != nil {
			return err
		}
		// Fields registered in Java render order (simple p0,p1,p3,p4 then
		// fragment p2) so the emitted document order is byte-identical; the
		// object-array send below supplies values in this same order.
		_, err = esper.RegisterObjectArray(env, "MyObjectArrayType", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
			esper.FieldDef("p1", reflect.TypeOf(0)),
			esper.FieldDef("p3", reflect.TypeOf(int64(0))),
			esper.FieldDef("p4", reflect.TypeOf(float64(0))),
			esper.OptionalFieldDef("p2", reflect.TypeOf((*any)(nil)).Elem()),
		}, esper.WithNestedPropertySchema("p2", nested))
		return err
	case "pojo-map":
		if _, err := esper.RegisterStruct[er546POJOMapEvent](env, "SupportBeanRendererOne"); err != nil {
			return err
		}
		_, err := esper.RegisterStruct[er546POJOMapEvent](env, "SupportBeanRendererThree")
		return err
	case "empty-map":
		_, err := esper.RegisterStruct[er546EmptyMapEvent](env, "EmptyMapEvent")
		return err
	case "sqldate":
		_, err := esper.RegisterStruct[er546SupportBean](env, "SupportBean")
		return err
	}
	return nil
}

// deploy builds and deploys the statement for one deploy step and attaches
// the recording listener (delivered marker + last event/row capture).
func (e *er546Env) deploy(ctx context.Context, caseName, statement, mode string, onResult func()) error {
	var plan esper.Plan
	var err error
	if caseName == "sqldate" {
		plan, err = e.env.Build(esper.FromAny(e.env, "SupportBean").
			Select(esper.Alias("mySqlDate", esper.Literal(esper.DateOnly("2010-01-31")))).
			Query(esper.StatementName(statement)))
	} else {
		plan, err = e.env.Build(esper.FromAny(e.env, er546TypeName(caseName, mode)).
			Query(esper.StatementName(statement)))
	}
	if err != nil {
		return err
	}
	deployment, err := e.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	e.deployments[statement] = deployment
	stmt := deployment.Statements()[0]
	_, err = stmt.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				e.lastEvent = &event
				e.lastRow = nil
			}
			if row, ok := result.Row(); ok {
				e.lastRow = &row
				e.lastEvent = nil
			}
			onResult()
		}
		return nil
	})
	return err
}

// send delivers the pinned underlying for a send step.
func (e *er546Env) send(ctx context.Context, caseName, mode string, payload json.RawMessage) error {
	var body struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return fmt.Errorf("decode send payload: %w", err)
	}
	switch caseName {
	case "custom-renderer":
		return e.engine.Send(ctx, "MyRendererEvent", er546RendererEvent{
			ID:             "id1",
			SomeProperties: [][]any{{1, "x"}, {2, "y"}},
			MappedProperty: map[string]any{"key": "value"},
		})
	case "object-array":
		nested, ok := e.env.Schema("SupportBean_S0")
		if !ok {
			return fmt.Errorf("object-array nested schema missing")
		}
		fragment, err := esper.NewEvent(nested, er546BeanS0{ID: 1, P00: "p00"}, time.Unix(0, 0).UTC())
		if err != nil {
			return err
		}
		// Values ride in Java render order (p0,p1,p3,p4,p2); the nested bean
		// is a materialized Event so the renderer walks it as an object.
		return e.engine.SendObjectArray(ctx, "MyObjectArrayType",
			[]any{"abc", 1, int64(2), 3.0, fragment})
	case "pojo-map":
		return e.engine.Send(ctx, er546TypeName(caseName, mode), er546POJOMapEvent{
			// Java also sends `otherMap.put(null, 1234)`; Go map keys cannot be
			// null and both renderers skip null-keyed entries, so the entry is
			// omitted — unobservable in the rendered output.
			StringObjectMap: map[string]any{"abc": "def", "def": 123, "efg": nil},
		})
	case "empty-map":
		var props map[string]string
		switch body.Payload {
		case "null":
			props = nil
		case "empty":
			props = map[string]string{}
		case "map":
			props = map[string]string{"a": "b"}
		default:
			return fmt.Errorf("unknown empty-map payload %q", body.Payload)
		}
		return e.engine.Send(ctx, "EmptyMapEvent", er546EmptyMapEvent{Props: props})
	case "sqldate":
		return e.engine.Send(ctx, "SupportBean", er546SupportBean{})
	}
	return fmt.Errorf("no send handling for case %q", caseName)
}

// render executes a value step against the captured event or row.
func (e *er546Env) render(caseName, name string) (string, error) {
	switch name {
	case "json":
		if e.lastEvent == nil {
			return "", fmt.Errorf("case %q has no captured event for json", caseName)
		}
		rendered, err := esper.RenderJSON(*e.lastEvent, esper.WithJSONTitle(er546Title(caseName)))
		if err != nil {
			return "", err
		}
		return er546Whitespace.ReplaceAllString(rendered, ""), nil
	case "xml":
		if caseName == "sqldate" {
			return e.renderSQLDateXML()
		}
		if e.lastEvent == nil {
			return "", fmt.Errorf("case %q has no captured event for xml", caseName)
		}
		rendered, err := esper.RenderXML(*e.lastEvent, esper.WithXMLTitle(er546Title(caseName)))
		if err != nil {
			return "", err
		}
		return er546Whitespace.ReplaceAllString(rendered, ""), nil
	case "mySqlDate":
		if e.lastRow == nil {
			return "", fmt.Errorf("case %q has no captured row for mySqlDate", caseName)
		}
		return fmt.Sprint(e.lastRow.Get("mySqlDate").Any()), nil
	}
	return "", fmt.Errorf("unknown value name %q for case %q", name, caseName)
}

// renderSQLDateXML re-materializes the projected row as an event so RenderXML
// applies to the {mySqlDate} projection the same way Java's iterator event
// carries the computed column.
func (e *er546Env) renderSQLDateXML() (string, error) {
	if e.lastRow == nil {
		return "", fmt.Errorf("sqldate has no captured row")
	}
	event, err := esper.NewEvent(e.lastRow.Schema(), e.lastRow.AsMap(), time.Unix(0, 0).UTC())
	if err != nil {
		return "", err
	}
	rendered, err := esper.RenderXML(event, esper.WithXMLTitle("testsqldate"))
	if err != nil {
		return "", err
	}
	return er546Whitespace.ReplaceAllString(rendered, ""), nil
}

func er546Title(caseName string) string {
	switch caseName {
	case "empty-map":
		return "outer"
	case "sqldate":
		return "testsqldate"
	}
	return "MyEvent"
}

func runEventRender546Scenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := scenario.Validate(); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: compat.ScenarioVersion, ID: scenario.ID}
	type caseBlock struct {
		name  string
		steps []compat.Step
	}
	var blocks []caseBlock
	for _, step := range scenario.Steps {
		if step.Op == "case" {
			blocks = append(blocks, caseBlock{name: step.Case})
			continue
		}
		if len(blocks) == 0 {
			return trace, fmt.Errorf("%s: step %q precedes the first case marker", eventRender546ID, step.Op)
		}
		blocks[len(blocks)-1].steps = append(blocks[len(blocks)-1].steps, step)
	}
	for _, block := range blocks {
		if err := runER546Case(ctx, block.name, block.steps, &trace); err != nil {
			return trace, fmt.Errorf("%s case %q: %w", eventRender546ID, block.name, err)
		}
	}
	return trace, nil
}

func runER546Case(ctx context.Context, caseName string, steps []compat.Step, trace *compat.Trace) error {
	state := newER546CaseState(caseName, trace)
	var ee *er546Env
	mode := ""
	if caseName != "enquote-json" && caseName != "enquote-xml" {
		ee = newER546Env()
		if err := er546Register(ee.env, caseName); err != nil {
			return err
		}
	}
	for _, step := range steps {
		switch step.Op {
		case "deploy":
			if ee == nil {
				return fmt.Errorf("deploy before engine in %q", caseName)
			}
			if step.Mode != "" {
				mode = step.Mode
			}
			if err := ee.deploy(ctx, caseName, step.Statement, mode, func() {
				state.emitRow(step.Statement, map[string]any{"delivered": true})
			}); err != nil {
				return err
			}
		case "deployed":
			state.emitDeployed(step.Statement)
		case "send":
			if ee == nil {
				return fmt.Errorf("send before deploy in %q", caseName)
			}
			if err := ee.send(ctx, caseName, mode, step.Payload); err != nil {
				return err
			}
		case "value":
			if ee == nil {
				return fmt.Errorf("value step before engine in %q", caseName)
			}
			rendered, err := ee.render(caseName, step.Name)
			if err != nil {
				return err
			}
			state.emitValue(step.Statement, step.Name, rendered)
		case "unrepresentable":
			state.emitUnrepresentable(step.Statement, step.Name, step.ExpectError)
		case "undeploy":
			if ee == nil {
				return fmt.Errorf("undeploy before engine in %q", caseName)
			}
			deployment, ok := ee.deployments[step.Statement]
			if !ok {
				return fmt.Errorf("%s: no deployment for statement %q", caseName, step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
			delete(ee.deployments, step.Statement)
		case "undeploy-all":
			state.resetSequences()
			if ee != nil {
				for _, deployment := range ee.deployments {
					if err := deployment.Undeploy(ctx); err != nil {
						return err
					}
				}
				if err := ee.engine.Close(ctx); err != nil {
					return err
				}
				ee = nil
			}
		default:
			return fmt.Errorf("unsupported op %q in %q", step.Op, caseName)
		}
	}
	if ee != nil {
		return ee.engine.Close(ctx)
	}
	return nil
}

// ---------- pinned step keys and scenario loader ----------

func er546DeployKey(caseName, statement, epl, mode string) string {
	return strings.Join([]string{"deploy", caseName, statement, "", mode, "", epl, "", ""}, "|")
}
func er546DeployedKey(caseName, statement string) string {
	return strings.Join([]string{"deployed", caseName, statement, "", "", "", "", "", ""}, "|")
}
func er546SendKey(caseName, eventType, mode, payload string) string {
	return strings.Join([]string{"send", caseName, "", eventType, mode, "", "", "", payload}, "|")
}
func er546ValueKey(caseName, statement, name string) string {
	return strings.Join([]string{"value", caseName, statement, "", "", name, "", "", ""}, "|")
}
func er546UnrepresentableKey(caseName, statement, name, note string) string {
	return strings.Join([]string{"unrepresentable", caseName, statement, "", "", name, "", note, ""}, "|")
}
func er546UndeployKey(caseName, statement string) string {
	return strings.Join([]string{"undeploy", caseName, statement, "", "", "", "", "", ""}, "|")
}

func er546SendPayload(marker string) string {
	raw, _ := json.Marshal(map[string]string{"payload": marker})
	return string(raw)
}

func er546EnquoteNote(helper, input, output string) string {
	return fmt.Sprintf("%s(%q) -> %q", helper, input, output)
}

// eventRender546PinnedSteps rebuilds the pinned step-key sequence; the loader
// compares every decoded step against it.
func eventRender546PinnedSteps() []string {
	var keys []string
	caseKey := func(name string) { keys = append(keys, "case|"+name+"|||||||") }
	addDeploy := func(caseName, statement, epl, mode string) {
		keys = append(keys, er546DeployKey(caseName, statement, epl, mode), er546DeployedKey(caseName, statement))
	}
	// custom-renderer
	caseKey("custom-renderer")
	addDeploy("custom-renderer", "s0", eventRender546CaseEPLs["custom-renderer"], "")
	keys = append(keys, er546SendKey("custom-renderer", "MyRendererEvent", "", er546SendPayload("custom-renderer")))
	keys = append(keys,
		er546UnrepresentableKey("custom-renderer", "s0", "json", er546CustomRendererJSONPin),
		er546UnrepresentableKey("custom-renderer", "s0", "xml", er546CustomRendererXMLPin),
		"undeploy-all|custom-renderer|||||||",
	)
	// object-array
	caseKey("object-array")
	addDeploy("object-array", "s0", eventRender546CaseEPLs["object-array"], "")
	keys = append(keys, er546SendKey("object-array", "MyObjectArrayType", "", er546SendPayload("object-array")))
	keys = append(keys,
		er546ValueKey("object-array", "s0", "json"),
		er546UnrepresentableKey("object-array", "s0", "xml", er546ObjectArrayXMLPin),
		"undeploy-all|object-array|||||||",
	)
	// pojo-map
	caseKey("pojo-map")
	addDeploy("pojo-map", "s0", "@name('s0') select * from SupportBeanRendererOne", "one")
	keys = append(keys, er546SendKey("pojo-map", "SupportBeanRendererOne", "one", er546SendPayload("one")))
	keys = append(keys,
		er546ValueKey("pojo-map", "s0", "json"),
		er546ValueKey("pojo-map", "s0", "xml"),
		er546UnrepresentableKey("pojo-map", "s0", "xml-attr", er546POJOMapXMLAttrPin),
		er546UndeployKey("pojo-map", "s0"),
	)
	addDeploy("pojo-map", "s0", "@name('s0') select * from SupportBeanRendererThree", "three")
	keys = append(keys,
		er546SendKey("pojo-map", "SupportBeanRendererThree", "three", er546SendPayload("three")),
		er546ValueKey("pojo-map", "s0", "json"),
		"undeploy-all|pojo-map|||||||",
	)
	// empty-map
	caseKey("empty-map")
	addDeploy("empty-map", "s0", eventRender546CaseEPLs["empty-map"], "")
	for _, marker := range []string{"null", "empty", "map"} {
		keys = append(keys,
			er546SendKey("empty-map", "EmptyMapEvent", "", er546SendPayload(marker)),
			er546ValueKey("empty-map", "s0", "json"))
	}
	keys = append(keys, "undeploy-all|empty-map|||||||")
	// enquote-json
	caseKey("enquote-json")
	for i, row := range er546JSONEnquoteRows {
		keys = append(keys, er546UnrepresentableKey("enquote-json", "enquote",
			fmt.Sprintf("row%d", i), er546EnquoteNote("enquote", row[0], row[1])))
	}
	// sqldate
	caseKey("sqldate")
	addDeploy("sqldate", "s0", eventRender546CaseEPLs["sqldate"], "")
	keys = append(keys,
		er546SendKey("sqldate", "SupportBean", "", er546SendPayload("sqldate")),
		er546ValueKey("sqldate", "s0", "mySqlDate"),
		er546ValueKey("sqldate", "s0", "xml"),
		"undeploy-all|sqldate|||||||",
	)
	// enquote-xml
	caseKey("enquote-xml")
	for i, row := range er546XMLEncodeRows {
		keys = append(keys, er546UnrepresentableKey("enquote-xml", "xmlEncode",
			fmt.Sprintf("row%d", i), er546EnquoteNote("xmlEncode", row[0], row[1])))
	}
	return keys
}

func er546StepKey(step compat.Step) string {
	payload := ""
	if len(step.Payload) > 0 {
		var decoded any
		if err := json.Unmarshal(step.Payload, &decoded); err == nil {
			if compacted, err := json.Marshal(decoded); err == nil {
				payload = string(compacted)
			}
		}
		if payload == "" {
			payload = string(step.Payload)
		}
	}
	return strings.Join([]string{step.Op, step.Case, step.Statement, step.EventType, step.Mode, step.Name, step.Epl, step.ExpectError, payload}, "|")
}

func er546ValidateStep(index int, object map[string]json.RawMessage, step compat.Step) error {
	require := func(names ...string) error {
		if err := requireEventInfraGetterNestedFields(object, names...); err != nil {
			return fmt.Errorf("scenario step %d: %w", index, err)
		}
		return nil
	}
	switch step.Op {
	case "case":
		return require("op", "case")
	case "deploy":
		if step.Mode != "" {
			return require("op", "case", "statement", "epl", "mode")
		}
		return require("op", "case", "statement", "epl")
	case "deployed":
		return require("op", "case", "statement")
	case "send":
		if step.Mode != "" {
			return require("op", "case", "eventType", "mode", "payload")
		}
		return require("op", "case", "eventType", "payload")
	case "value":
		return require("op", "case", "statement", "name")
	case "unrepresentable":
		if step.Name != "" {
			return require("op", "case", "statement", "name", "expectError")
		}
		return require("op", "case", "statement", "expectError")
	case "undeploy":
		return require("op", "case", "statement")
	case "undeploy-all":
		return require("op", "case")
	}
	return fmt.Errorf("scenario step %d has unsupported op %q", index, step.Op)
}

func loadEventRender546Scenario(reader io.Reader) (compat.Scenario, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, err
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario must be a JSON object: %w", eventRender546ID, err)
	}
	if err := requireEventInfraGetterNestedFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", eventRender546ID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != eventRender546ID ||
		metadata.Description != eventRender546Description ||
		metadata.JavaCommit != eventRender546JavaCommit || metadata.JavaSource != eventRender546Source {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata does not match the pinned values", eventRender546ID)
	}
	if !reflect.DeepEqual(metadata.JavaFlags, eventRender546JavaFlags) {
		return compat.Scenario{}, fmt.Errorf("%s javaFlags = %#v", eventRender546ID, metadata.JavaFlags)
	}
	if !reflect.DeepEqual(metadata.JavaRuntimes, eventRender546JavaRuntimeIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaRuntimes = %#v", eventRender546ID, metadata.JavaRuntimes)
	}
	if !reflect.DeepEqual(metadata.JavaNames, eventRender546JavaExecutions) {
		return compat.Scenario{}, fmt.Errorf("%s javaNames = %#v", eventRender546ID, metadata.JavaNames)
	}
	if !reflect.DeepEqual(metadata.JavaStaticID, eventRender546JavaStaticIDs) {
		return compat.Scenario{}, fmt.Errorf("%s javaStaticIds = %#v", eventRender546ID, metadata.JavaStaticID)
	}
	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario cases must be an array: %w", eventRender546ID, err)
	}
	if len(rawCases) != len(eventRender546Cases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d cases, want %d", eventRender546ID, len(rawCases), len(eventRender546Cases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireEventInfraGetterNestedFields(object,
			"case", "ordinal", "runtimeId", "executionName", "observation", "epl", "flags"); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		var definition struct {
			Case          string   `json:"case"`
			Ordinal       int      `json:"ordinal"`
			RuntimeID     string   `json:"runtimeId"`
			ExecutionName string   `json:"executionName"`
			Observation   string   `json:"observation"`
			EPL           string   `json:"epl"`
			Flags         []string `json:"flags"`
		}
		if err := json.Unmarshal(rawCase, &definition); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if definition.Case != eventRender546Cases[index] || definition.Ordinal != eventRender546CaseOrdinals[index] ||
			definition.RuntimeID != eventRender546JavaRuntimeIDs[index] ||
			definition.ExecutionName != eventRender546JavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d identity mismatch", eventRender546ID, index)
		}
		if definition.EPL != eventRender546CaseEPLs[definition.Case] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q EPL is not pinned", eventRender546ID, definition.Case)
		}
		var wantFlags []string
		switch definition.Case {
		case "custom-renderer":
			wantFlags = []string{"SERDEREQUIRED"}
		case "pojo-map":
			wantFlags = []string{"EXCLUDEWHENINSTRUMENTED"}
		default:
			wantFlags = []string{}
		}
		if !reflect.DeepEqual(definition.Flags, wantFlags) {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %q flags = %#v, want %#v", eventRender546ID, definition.Case, definition.Flags, wantFlags)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array: %w", eventRender546ID, err)
	}
	pinned := eventRender546PinnedSteps()
	if len(rawSteps) != len(pinned) {
		return compat.Scenario{}, fmt.Errorf("%s scenario has %d steps, want %d", eventRender546ID, len(rawSteps), len(pinned))
	}
	steps := make([]compat.Step, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var step compat.Step
		if err := json.Unmarshal(rawStep, &step); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		if err := er546ValidateStep(index, object, step); err != nil {
			return compat.Scenario{}, err
		}
		if key := er546StepKey(step); key != pinned[index] {
			return compat.Scenario{}, fmt.Errorf("scenario step %d = %q, want pinned %q", index, key, pinned[index])
		}
		steps[index] = step
	}
	return compat.Scenario{Version: metadata.Version, ID: metadata.ID, Steps: steps}, nil
}
