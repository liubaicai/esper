package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/liubaicai/esper/internal/compat"
	"github.com/liubaicai/esper/internal/esper"
)

const eplOtherSelectExprID = "epl-other-select-expr"
const eplOtherSelectExprJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"

var eplOtherSelectExprJavaSources = []string{
	"regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExpr.java",
}

var eplOtherSelectExprJavaRuntimeIDs = []string{
	"java-runtime-7222e4dfd73a239bf53c",
	"java-runtime-a1605a2ba0d017fa91f1",
	"java-runtime-bc47c8b86afd9b19f5c6",
	"java-runtime-446476d182df93abd788",
	"java-runtime-d01d8e953908acc44955",
	"java-runtime-5371694959860be64ff6",
}

var eplOtherSelectExprJavaExecutions = []string{
	"EPLOtherPrecedenceNoColumnName",
	"EPLOtherGraphSelect",
	"EPLOtherKeywordsAllowed",
	"EPLOtherEscapeString",
	"EPLOtherGetEventType",
	"EPLOtherWindowStats",
}

type eplOtherSelectExprSupportBean struct {
	TheString      string   `esper:"theString"`
	IntPrimitive   int      `esper:"intPrimitive"`
	BoolBoxed      *bool    `esper:"boolBoxed"`
	FloatPrimitive float32  `esper:"floatPrimitive"`
	FloatBoxed     *float32 `esper:"floatBoxed"`
}

type eplOtherSelectExprNestedNested struct {
	NestedNestedValue string `esper:"nestedNestedValue"`
}

type eplOtherSelectExprNested struct {
	NestedValue  string                         `esper:"nestedValue"`
	NestedNested eplOtherSelectExprNestedNested `esper:"nestedNested"`
}

type eplOtherSelectExprComplexProps struct {
	Nested eplOtherSelectExprNested `esper:"nested"`
}

type eplOtherSelectExprKeywords struct {
	Count          int `esper:"count"`
	Escape         int `esper:"escape"`
	Every          int `esper:"every"`
	Sum            int `esper:"sum"`
	Avg            int `esper:"avg"`
	Max            int `esper:"max"`
	Min            int `esper:"min"`
	Coalesce       int `esper:"coalesce"`
	Median         int `esper:"median"`
	Stddev         int `esper:"stddev"`
	Avedev         int `esper:"avedev"`
	Events         int `esper:"events"`
	First          int `esper:"first"`
	Last           int `esper:"last"`
	Unidirectional int `esper:"unidirectional"`
	Pattern        int `esper:"pattern"`
	SQL            int `esper:"sql"`
	Metadatasql    int `esper:"metadatasql"`
	Prev           int `esper:"prev"`
	Prior          int `esper:"prior"`
	Weekday        int `esper:"weekday"`
	Lastweekday    int `esper:"lastweekday"`
	Cast           int `esper:"cast"`
	Snapshot       int `esper:"snapshot"`
	Variable       int `esper:"variable"`
	Window         int `esper:"window"`
	Left           int `esper:"left"`
	Right          int `esper:"right"`
	Full           int `esper:"full"`
	Outer          int `esper:"outer"`
	Join           int `esper:"join"`
}

func runEplOtherSelectExprScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := ctx.Err(); err != nil {
		return compat.Trace{}, err
	}
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[eplOtherSelectExprSupportBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherSelectExprComplexProps](env, "SupportBeanComplexProps"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[eplOtherSelectExprKeywords](env, "SupportBeanKeywords"); err != nil {
		return compat.Trace{}, err
	}
	nestedNestedSchema, err := esper.NewMapSchema("NestedNested", []esper.FieldSpec{
		{Name: "nestedNestedValue", Type: reflect.TypeOf("")},
	})
	if err != nil {
		return compat.Trace{}, err
	}
	nestedSchema, err := esper.NewMapSchema("Nested", []esper.FieldSpec{
		{Name: "nestedValue", Type: reflect.TypeOf("")},
		{Name: "nestedNested", Type: reflect.TypeOf(map[string]any{})},
	}, esper.WithNestedPropertySchema("nestedNested", nestedNestedSchema))
	if err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterMap(env, "MyStream", []esper.FieldSpec{
		{Name: "nested", Type: reflect.TypeOf(map[string]any{})},
	}, esper.WithNestedPropertySchema("nested", nestedSchema)); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env)
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	var currentCase string
	var deployment *esper.Deployment
	var deployments []*esper.Deployment
	var listener *eplOtherSelectExprListener
	sequence := uint64(0)

	for _, step := range scenario.Steps {
		switch step.Op {
		case "case":
			currentCase = step.Case
			sequence = 0
		case "deploy":
			query, err := eplOtherSelectExprQuery(env, currentCase, step.Epl, step.Statement)
			if err != nil {
				return trace, err
			}
			plan, err := env.Build(query)
			if err != nil {
				return trace, err
			}
			deployment, err = engine.Deploy(ctx, plan)
			if err != nil {
				return trace, err
			}
			deployments = append(deployments, deployment)
			if step.Statement == "s0" {
				listener = &eplOtherSelectExprListener{caseName: currentCase, trace: &trace, sequence: &sequence, engine: engine}
				deployment.Statements()[0].Subscribe(listener.deliver)
			}
		case "deployed":
			sequence++
			trace.Records = append(trace.Records, compat.TraceRecord{Case: currentCase, Operation: "deployed",
				Statement: step.Statement, Sequence: sequence, Time: "1970-01-01T00:00:00Z"})
		case "types":
			sequence++
			record := compat.TraceRecord{Case: currentCase, Operation: "types",
				Statement: step.Statement, Sequence: sequence, Time: "1970-01-01T00:00:00Z"}
			if deployment != nil {
				if schema, ok := deployment.Statements()[0].Plan().ResultSchema(); ok {
					var entries []map[string]any
					for _, name := range schema.PropertyNames() {
						typ, _ := schema.PropertyType(name)
						entries = append(entries, map[string]any{"name": name, "type": eplOtherSelectExprTypeToken(typ)})
					}
					record.Value = entries
				}
			}
			trace.Records = append(trace.Records, record)
		case "undeploy-all":
			for _, d := range deployments {
				if err := d.Undeploy(ctx); err != nil {
					return trace, err
				}
			}
			deployments = nil
			deployment = nil
		case "send":
			if err := eplOtherSelectExprSend(ctx, engine, step); err != nil {
				return trace, err
			}
		}
	}
	return trace, nil
}

type eplOtherSelectExprListener struct {
	caseName string
	trace    *compat.Trace
	sequence *uint64
	engine   *esper.Engine
}

func (l *eplOtherSelectExprListener) deliver(ctx context.Context, batch esper.ResultBatch) error {
	*l.sequence++
	record := compat.TraceRecord{Case: l.caseName, Operation: "listener", Statement: "s0",
		Sequence: *l.sequence, Time: l.engine.Now().UTC().Format(time.RFC3339)}
	record.New = compat.NormalizeResults(batch.New)
	record.Old = compat.NormalizeResults(batch.Old)
	l.trace.Records = append(l.trace.Records, record)
	return nil
}

func eplOtherSelectExprTypeToken(typ reflect.Type) string {
	if typ == nil {
		return "Object"
	}
	switch typ.Kind() {
	case reflect.String:
		return "String"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return "Integer"
	case reflect.Int64:
		return "Long"
	case reflect.Float32:
		return "Float"
	case reflect.Float64:
		return "Double"
	case reflect.Bool:
		return "Boolean"
	case reflect.Pointer:
		return eplOtherSelectExprTypeToken(typ.Elem())
	}
	return "Object"
}

func eplOtherSelectExprQuery(env *esper.Environment, caseName, epl, statement string) (esper.Query, error) {
	trueVal := true
	supportBean := esper.From[eplOtherSelectExprSupportBean](env, "SupportBean")
	complexProps := esper.From[eplOtherSelectExprComplexProps](env, "SupportBeanComplexProps")
	keywords := esper.From[eplOtherSelectExprKeywords](env, "SupportBeanKeywords")
	theString := esper.Field[eplOtherSelectExprSupportBean, string]("theString")
	intPrimitive := esper.Field[eplOtherSelectExprSupportBean, int]("intPrimitive")
	boolBoxed := esper.Field[eplOtherSelectExprSupportBean, *bool]("boolBoxed")
	floatPrimitive := esper.Field[eplOtherSelectExprSupportBean, float32]("floatPrimitive")
	floatBoxed := esper.Field[eplOtherSelectExprSupportBean, *float32]("floatBoxed")

	switch {
	case caseName == "precedence-no-column-name" && epl == "@name('s0') select 3*2+1 from SupportBean":
		return esper.Select(supportBean, esper.Alias("3*2+1", esper.Add[int](esper.Multiply[int](esper.Literal(3), esper.Literal(2)), esper.Literal(1)))).Query(esper.StatementName("s0")), nil
	case caseName == "precedence-no-column-name" && epl == "@name('s0') select (3*2)+1 from SupportBean":
		return esper.Select(supportBean, esper.Alias("3*2+1", esper.Add[int](esper.Multiply[int](esper.Literal(3), esper.Literal(2)), esper.Literal(1)))).Query(esper.StatementName("s0")), nil
	case caseName == "precedence-no-column-name" && epl == "@name('s0') select 3*(2+1) from SupportBean":
		return esper.Select(supportBean, esper.Alias("3*(2+1)", esper.Multiply[int](esper.Literal(3), esper.Add[int](esper.Literal(2), esper.Literal(1))))).Query(esper.StatementName("s0")), nil
	case caseName == "graph-select" && epl == "@public insert into MyStream select nested from SupportBeanComplexProps":
		return esper.Select(complexProps, esper.Alias("nested", esper.Field[eplOtherSelectExprComplexProps, eplOtherSelectExprNested]("nested"))).InsertInto("MyStream"), nil
	case caseName == "graph-select" && epl == "@name('s0') select nested.nestedValue, nested.nestedNested.nestedNestedValue from MyStream":
		myStream := esper.FromAny(env, "MyStream")
		nested := esper.Field[any, any]("nested")
		return myStream.Select(
			esper.Alias("nested.nestedValue", esper.Property[string](nested, "nestedValue")),
			esper.Alias("nested.nestedNested.nestedNestedValue", esper.Property[string](esper.Property[any](nested, "nestedNested"), "nestedNestedValue")),
		).Query(esper.StatementName("s0")), nil
	case caseName == "keywords-allowed" && epl == "@name('s0') select count,escape,every,sum,avg,max,min,coalesce,median,stddev,avedev,events,first,last,unidirectional,pattern,sql,metadatasql,prev,prior,weekday,lastweekday,cast,snapshot,variable,window,left,right,full,outer,join from SupportBeanKeywords":
		fields := []string{"count", "escape", "every", "sum", "avg", "max", "min", "coalesce", "median", "stddev", "avedev", "events", "first", "last", "unidirectional", "pattern", "sql", "metadatasql", "prev", "prior", "weekday", "lastweekday", "cast", "snapshot", "variable", "window", "left", "right", "full", "outer", "join"}
		selections := make([]esper.Selection, 0, len(fields))
		for _, f := range fields {
			selections = append(selections, esper.Alias(f, esper.Field[eplOtherSelectExprKeywords, int](f)))
		}
		return esper.Select(keywords, selections...).Query(esper.StatementName("s0")), nil
	case caseName == "keywords-allowed" && epl == "@name('s0') select escape as stddev, count(*) as count, last from SupportBeanKeywords":
		return keywords.Aggregate(
			esper.Alias("stddev", esper.Field[eplOtherSelectExprKeywords, int]("escape")),
			esper.Alias("count", esper.CountAll()),
			esper.Alias("last", esper.Field[eplOtherSelectExprKeywords, int]("last")),
		).Query(esper.StatementName("s0")), nil
	case caseName == "escape-string" && (epl == "@name('s0') select * from SupportBean(theString=\"A'B\")" || epl == "@name('s0') select * from SupportBean(theString='A\\'B')" || epl == "@name('s0') select * from SupportBean(theString='A\\u0027B')"):
		return supportBean.Filter(esper.Equal[string](theString, esper.Literal("A'B"))).Query(esper.StatementName("s0")), nil
	case caseName == "escape-string" && (epl == "@name('s0') select * from SupportBean(theString='A\"B')" || epl == "@name('s0') select * from SupportBean(theString='A\\\"B')" || epl == "@name('s0') select * from SupportBean(theString='A\\u0022B')"):
		return supportBean.Filter(esper.Equal[string](theString, esper.Literal("A\"B"))).Query(esper.StatementName("s0")), nil
	case caseName == "escape-string" && epl == "@Name('A\\'B') @Description(\"A\\\"B\") select * from SupportBean":
		return supportBean.Query(esper.StatementName("A'B"), esper.StatementDescription("A\"B")), nil
	case caseName == "escape-string" && epl == "@name('s0') select 'volume' as field1, \"sleep\" as field2, \"\\u0041\" as unicodeA from SupportBean":
		return esper.Select(supportBean,
			esper.Alias("field1", esper.Literal("volume")),
			esper.Alias("field2", esper.Literal("sleep")),
			esper.Alias("unicodeA", esper.Literal("A")),
		).Query(esper.StatementName("s0")), nil
	case caseName == "escape-string" && (epl == "@name('s0') select * from SupportBean(theString='John\\'s')" || epl == "@name('s0') select * from SupportBean(theString='John\\u0027s')"):
		return supportBean.Filter(esper.Equal[string](theString, esper.Literal("John's"))).Query(esper.StatementName("s0")), nil
	case caseName == "escape-string" && (epl == "@name('s0') select * from SupportBean(theString like \"Quote \\\"Hello\\\"\")" || epl == "@name('s0') select * from SupportBean(theString like \"Quote \\u0022Hello\\u0022\")"):
		return supportBean.Filter(esper.Equal[string](theString, esper.Literal("Quote \"Hello\""))).Query(esper.StatementName("s0")), nil
	case caseName == "get-event-type":
		return esper.Select(supportBean.Window(esper.LengthWindow(3)).Filter(esper.Equal[*bool](boolBoxed, esper.Literal[*bool](&trueVal))),
			esper.Alias("theString", theString),
			esper.Alias("aBool", boolBoxed),
			esper.Alias("3*intPrimitive", esper.Multiply[int](esper.Literal(3), intPrimitive)),
			esper.Alias("result", esper.Add[float32](floatBoxed, floatPrimitive)),
		).Query(esper.StatementName("s0")), nil
	case caseName == "window-stats":
		return esper.Select(supportBean.Window(esper.LengthWindow(3)).Filter(esper.Equal[*bool](boolBoxed, esper.Literal[*bool](&trueVal))),
			esper.Alias("theString", theString),
			esper.Alias("aBool", boolBoxed),
			esper.Alias("3*intPrimitive", esper.Multiply[int](esper.Literal(3), intPrimitive)),
			esper.Alias("result", esper.Add[float32](floatBoxed, floatPrimitive)),
		).Query(esper.StatementName("s0")), nil
	}
	return esper.Query{}, fmt.Errorf("%s: no fluent query for case %q epl %q", eplOtherSelectExprID, caseName, epl)
}

func eplOtherSelectExprSend(ctx context.Context, engine *esper.Engine, step compat.Step) error {
	var payload map[string]any
	if len(step.Payload) > 0 {
		if err := json.Unmarshal(step.Payload, &payload); err != nil {
			return fmt.Errorf("%s send payload: %w", eplOtherSelectExprID, err)
		}
	}
	switch step.EventType {
	case "SupportBean":
		event := eplOtherSelectExprSupportBean{}
		if v, ok := payload["theString"].(string); ok {
			event.TheString = v
		}
		if v, ok := payload["intPrimitive"].(float64); ok {
			event.IntPrimitive = int(v)
		}
		if v, ok := payload["boolBoxed"].(bool); ok {
			event.BoolBoxed = &v
		}
		if v, ok := payload["floatPrimitive"].(float64); ok {
			event.FloatPrimitive = float32(v)
		}
		if v, ok := payload["floatBoxed"].(float64); ok {
			f := float32(v)
			event.FloatBoxed = &f
		}
		return engine.SendEvent(ctx, event)
	case "SupportBeanComplexProps":
		event := eplOtherSelectExprComplexProps{}
		if v, ok := payload["nestedValue"].(string); ok {
			event.Nested.NestedValue = v
		}
		if v, ok := payload["nestedNestedValue"].(string); ok {
			event.Nested.NestedNested.NestedNestedValue = v
		}
		return engine.SendEvent(ctx, event)
	case "SupportBeanKeywords":
		return engine.SendEvent(ctx, eplOtherSelectExprKeywords{
			Count: 1, Escape: 1, Every: 1, Sum: 1, Avg: 1, Max: 1, Min: 1,
			Coalesce: 1, Median: 1, Stddev: 1, Avedev: 1, Events: 1, First: 1,
			Last: 1, Unidirectional: 1, Pattern: 1, SQL: 1, Metadatasql: 1,
			Prev: 1, Prior: 1, Weekday: 1, Lastweekday: 1, Cast: 1, Snapshot: 1,
			Variable: 1, Window: 1, Left: 1, Right: 1, Full: 1, Outer: 1, Join: 1,
		})
	}
	return fmt.Errorf("%s: unsupported event type %q", eplOtherSelectExprID, step.EventType)
}
