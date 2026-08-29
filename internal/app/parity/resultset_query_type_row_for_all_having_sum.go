package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

func decodeResultSetQueryTypeRowForAllHavingSumInteger(raw json.RawMessage, name string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number == "" {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil || value < 0 || int64(int(value)) != value {
		return 0, fmt.Errorf("%s must be an integer JSON number", name)
	}
	return int(value), nil
}

func validateResultSetQueryTypeRowForAllHavingSumStringArray(raw json.RawMessage, expected []string, name string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return fmt.Errorf("%s must be a string array", name)
	}
	if len(values) != len(expected) {
		return fmt.Errorf("%s is not pinned", name)
	}
	for i := range expected {
		if values[i] != expected[i] {
			return fmt.Errorf("%s is not pinned", name)
		}
	}
	return nil
}

const (
	resultSetQueryTypeRowForAllHavingSumJavaCommit  = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	resultSetQueryTypeRowForAllHavingSumCase        = "sum-one-view"
	resultSetQueryTypeRowForAllHavingSumID          = "resultset-querytype-row-for-all-having-sum-one"
	resultSetQueryTypeRowForAllHavingSumDescription = "ResultSetQueryTypeRowForAllHaving ordinal 0: time-window sum with having threshold and listener expiry."
	resultSetQueryTypeRowForAllHavingSumEPL         = "@name('s0') select irstream sum(longBoxed) as mySum from SupportBean#time(10 seconds) having sum(longBoxed) > 10"
)

var (
	resultSetQueryTypeRowForAllHavingSumJavaSources    = []string{"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAllHaving.java"}
	resultSetQueryTypeRowForAllHavingSumJavaRuntimeIDs = []string{"java-runtime-65e4ba1499cb12f04d8a"}
	resultSetQueryTypeRowForAllHavingSumJavaExecutions = []string{"ResultSetQueryTypeRowForAllWHavingSumOneView"}
)

type resultSetQueryTypeRowForAllHavingSumBean struct {
	TheString string `esper:"theString"`
	LongBoxed *int64 `esper:"longBoxed"`
}

func loadResultSetQueryTypeRowForAllHavingSumScenario(r io.Reader) (compat.Scenario, error) {
	if r == nil {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum scenario reader is required")
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return compat.Scenario{}, err
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode resultset-querytype-row-for-all-having-sum scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, err
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("scenario contains unexpected or missing fields")
	}
	for _, k := range required {
		if _, ok := root[k]; !ok {
			return compat.Scenario{}, fmt.Errorf("scenario is missing field %q", k)
		}
	}
	var version, id, description, commit, source string
	for k, p := range map[string]*string{"version": &version, "id": &id, "description": &description, "javaCommit": &commit, "javaSource": &source} {
		if err := json.Unmarshal(root[k], p); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario %s must be a string", k)
		}
	}
	if version != compat.ScenarioVersion || id != resultSetQueryTypeRowForAllHavingSumID || description != resultSetQueryTypeRowForAllHavingSumDescription || commit != resultSetQueryTypeRowForAllHavingSumJavaCommit || source != resultSetQueryTypeRowForAllHavingSumJavaSources[0] {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum metadata is not pinned")
	}
	var runtimes, names, staticIDs, flags []string
	for k, expected := range map[string][]string{
		"javaRuntimes":  resultSetQueryTypeRowForAllHavingSumJavaRuntimeIDs,
		"javaNames":     resultSetQueryTypeRowForAllHavingSumJavaExecutions,
		"javaStaticIds": {"java-52f40599a0c7dc213c2b"},
		"javaFlags":     {},
	} {
		if err := validateResultSetQueryTypeRowForAllHavingSumStringArray(root[k], expected, k); err != nil {
			return compat.Scenario{}, err
		}
	}
	if err := json.Unmarshal(root["javaRuntimes"], &runtimes); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaNames"], &names); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaStaticIds"], &staticIDs); err != nil {
		return compat.Scenario{}, err
	}
	if err := json.Unmarshal(root["javaFlags"], &flags); err != nil {
		return compat.Scenario{}, err
	}
	if !equalStrings(runtimes, resultSetQueryTypeRowForAllHavingSumJavaRuntimeIDs) || !equalStrings(names, resultSetQueryTypeRowForAllHavingSumJavaExecutions) || !equalStrings(staticIDs, []string{"java-52f40599a0c7dc213c2b"}) || len(flags) != 0 {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum Java references are not pinned")
	}
	var cases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &cases); err != nil || len(cases) != 1 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly one case")
	}
	var caseObject map[string]json.RawMessage
	if err := strictObject(cases[0], &caseObject); err != nil {
		return compat.Scenario{}, fmt.Errorf("scenario case: %w", err)
	}
	if len(caseObject) != 7 {
		return compat.Scenario{}, fmt.Errorf("scenario case contains unexpected or missing fields")
	}
	for _, k := range []string{"case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl"} {
		if _, ok := caseObject[k]; !ok {
			return compat.Scenario{}, fmt.Errorf("scenario case is missing field %q", k)
		}
	}
	var cm struct {
		Case              string `json:"case"`
		Ordinal           int    `json:"ordinal"`
		RuntimeID         string `json:"runtimeId"`
		ExecutionName     string `json:"executionName"`
		Observation       string `json:"observation"`
		IteratorSnapshots int    `json:"iteratorSnapshots"`
		EPL               string `json:"epl"`
	}
	ordinal, ordinalErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(caseObject["ordinal"], "ordinal")
	iteratorSnapshots, iteratorErr := decodeResultSetQueryTypeRowForAllHavingSumInteger(caseObject["iteratorSnapshots"], "iteratorSnapshots")
	if ordinalErr != nil || iteratorErr != nil || ordinal != 0 || iteratorSnapshots != 0 || json.Unmarshal(cases[0], &cm) != nil || cm.Case != resultSetQueryTypeRowForAllHavingSumCase || cm.RuntimeID != resultSetQueryTypeRowForAllHavingSumJavaRuntimeIDs[0] || cm.ExecutionName != resultSetQueryTypeRowForAllHavingSumJavaExecutions[0] || cm.Observation != "listener" || cm.EPL != resultSetQueryTypeRowForAllHavingSumEPL {
		return compat.Scenario{}, fmt.Errorf("resultset-querytype-row-for-all-having-sum case metadata is not pinned")
	}
	var steps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &steps); err != nil || len(steps) != 8 {
		return compat.Scenario{}, fmt.Errorf("scenario must contain exactly eight steps")
	}
	decoded := make([]compat.Step, len(steps))
	for i, p := range steps {
		var obj map[string]json.RawMessage
		if err := strictObject(p, &obj); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", i, err)
		}
		expectedFields := 2
		if i > 0 && i%2 == 0 {
			expectedFields = 3
		}
		if len(obj) != expectedFields {
			return compat.Scenario{}, fmt.Errorf("scenario step %d fields are not pinned", i)
		}
		if err := json.Unmarshal(p, &decoded[i]); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", i, err)
		}
	}
	s := compat.Scenario{Version: version, ID: id, Steps: decoded}
	if err := validateResultSetQueryTypeRowForAllHavingSumScenario(s); err != nil {
		return compat.Scenario{}, err
	}
	return s, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func rejectDuplicateJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return walkJSON(d)
}
func walkJSON(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == json.Delim('{') {
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			key := k.(string)
			if seen[key] {
				return fmt.Errorf("JSON object contains duplicate field %q", key)
			}
			seen[key] = true
			if e = walkJSON(d); e != nil {
				return e
			}
		}
		_, err = d.Token()
		return err
	}
	if t == json.Delim('[') {
		for d.More() {
			if err := walkJSON(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}
func validateResultSetQueryTypeRowForAllHavingSumScenario(s compat.Scenario) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.ID != resultSetQueryTypeRowForAllHavingSumID || len(s.Steps) != 8 || s.Steps[0].Op != "case" || s.Steps[0].Case != resultSetQueryTypeRowForAllHavingSumCase {
		return fmt.Errorf("resultset-querytype-row-for-all-having-sum scenario steps are not pinned")
	}
	times := []int64{0, 5, 8, 10}
	for i, idx := range []int{1, 3, 5, 7} {
		st := s.Steps[idx]
		if st.Op != "advance-time" {
			return fmt.Errorf("scenario step %d must advance time", idx)
		}
		at, e := time.Parse(time.RFC3339Nano, st.At)
		if e != nil || at.Unix() != times[i] || !at.Equal(time.Unix(times[i], 0).UTC()) {
			return fmt.Errorf("scenario advance-time step %d is not pinned", idx)
		}
	}
	want := []int64{10, 15, -5}
	for i, v := range want {
		idx := []int{2, 4, 6}[i]
		st := s.Steps[idx]
		if st.Op != "send" || st.EventType != "SupportBean" {
			return fmt.Errorf("scenario step %d must send SupportBean", idx)
		}
		b, e := decodeResultSetQueryTypeRowForAllHavingSumPayloadTyped(st)
		if e != nil {
			return fmt.Errorf("step %d: %w", idx, e)
		}
		if b.LongBoxed == nil || *b.LongBoxed != v || b.TheString != "KEY" {
			return fmt.Errorf("step %d has unexpected SupportBean payload", idx)
		}
	}
	return nil
}
func runResultSetQueryTypeRowForAllHavingSumScenario(ctx context.Context, s compat.Scenario) (compat.Trace, error) {
	if err := validateResultSetQueryTypeRowForAllHavingSumScenario(s); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, e := esper.RegisterStruct[resultSetQueryTypeRowForAllHavingSumBean](env, "SupportBean"); e != nil {
		return compat.Trace{}, e
	}
	f := esper.Field[resultSetQueryTypeRowForAllHavingSumBean, *int64]("longBoxed")
	sum := esper.Sum[int64](esper.Cast[*int64, int64](f))
	q := esper.From[resultSetQueryTypeRowForAllHavingSumBean](env, "SupportBean").Window(esper.TimeWindow(10*time.Second)).Aggregate(esper.Alias("mySum", sum)).Having(esper.Greater[int64](sum, esper.Literal(int64(10)))).Query(esper.StatementName("s0"), esper.WithOldStream())
	plan, e := env.Build(q)
	if e != nil {
		return compat.Trace{}, e
	}
	engine, st, e := deployParityStatementWithRuntime(ctx, env, plan, resultSetQueryTypeRowForAllHavingSumJavaRuntimeIDs[0])
	if e != nil {
		return compat.Trace{}, e
	}
	defer engine.Close(context.Background())
	tr, e := compat.ReplayWithStatements(ctx, engine, st, s, decodeResultSetQueryTypeRowForAllHavingSumPayload, func(n string) (*esper.Statement, error) {
		if n != st.Name() {
			return nil, fmt.Errorf("unknown statement %q", n)
		}
		return st, nil
	})
	return tr, e
}

func deployParityStatementWithRuntime(ctx context.Context, env *esper.Environment, plan esper.Plan, uri string) (*esper.Engine, *esper.Statement, error) {
	engine := esper.NewEngine(env, esper.WithStartTime(time.Unix(0, 0).UTC()), esper.WithRuntimeURI(uri))
	d, e := engine.Deploy(ctx, plan)
	if e != nil {
		return nil, nil, e
	}
	ss := d.Statements()
	if len(ss) != 1 {
		return nil, nil, fmt.Errorf("expected one statement, got %d", len(ss))
	}
	return engine, ss[0], nil
}
func decodeResultSetQueryTypeRowForAllHavingSumPayload(st compat.Step) (any, error) {
	return decodeResultSetQueryTypeRowForAllHavingSumPayloadTyped(st)
}
func decodeResultSetQueryTypeRowForAllHavingSumPayloadTyped(st compat.Step) (resultSetQueryTypeRowForAllHavingSumBean, error) {
	var m map[string]json.RawMessage
	if err := strictObject(st.Payload, &m); err != nil {
		return resultSetQueryTypeRowForAllHavingSumBean{}, err
	}
	if len(m) != 2 {
		return resultSetQueryTypeRowForAllHavingSumBean{}, fmt.Errorf("SupportBean payload must contain exactly theString and longBoxed")
	}
	var b resultSetQueryTypeRowForAllHavingSumBean
	if err := json.Unmarshal(m["theString"], &b.TheString); err != nil {
		return b, fmt.Errorf("theString must be a string")
	}
	if string(bytes.TrimSpace(m["longBoxed"])) == "null" {
		return b, fmt.Errorf("longBoxed cannot be null")
	}
	if err := json.Unmarshal(m["longBoxed"], &b.LongBoxed); err != nil {
		return b, fmt.Errorf("longBoxed must be an integer")
	}
	return b, nil
}
