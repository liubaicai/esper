package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// Parity coverage for PatternSuperAndInterfaces: pattern filters over
// interface and superclass event types with inherited and overridden
// properties. Java class hierarchies map to parented Map schemas
// (WithSchemaParent); getter overrides map to the effective (most-derived)
// property value stored per concrete record, since the shadowed base storage
// is unobservable through any Java getter in the suite. Each concrete event
// type is unique per set-five event, so the fired tag's TypeName identifies
// the matched Java event exactly.

func TestPatternSuperAndInterfacesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	str := reflect.TypeOf("")
	register := func(name string, fields []string, parents ...Schema) Schema {
		t.Helper()
		specs := make([]FieldSpec, 0, len(fields))
		for _, field := range fields {
			specs = append(specs, FieldDef(field, str))
		}
		opts := make([]SchemaOption, 0, len(parents))
		for _, parent := range parents {
			opts = append(opts, WithSchemaParent(parent))
		}
		schema, err := RegisterMap(env, name, specs, opts...)
		if err != nil {
			t.Fatal(err)
		}
		return schema
	}

	// Interface and abstract-class schemas.
	baseAB := register("ISupportBaseAB", []string{"baseAB"})
	interfaceA := register("ISupportA", []string{"a"}, baseAB)
	interfaceB := register("ISupportB", []string{"b"}, baseAB)
	interfaceC := register("ISupportC", []string{"c"})
	baseDBase := register("ISupportBaseDBase", []string{"baseDBase"})
	baseD := register("ISupportBaseD", []string{"baseD"}, baseDBase)
	interfaceD := register("ISupportD", []string{"d"}, baseD)
	superG := register("ISupportAImplSuperG", []string{"g"}, interfaceA)
	overrideBase := register("SupportOverrideBase", []string{"val"})
	overrideOne := register("SupportOverrideOne", nil, overrideBase)
	register("SupportOverrideOneA", nil, overrideOne)
	register("SupportOverrideOneB", nil, overrideOne)

	// Concrete event types.
	register("ISupportCImpl", nil, interfaceC)
	register("ISupportABCImpl", nil, interfaceA, interfaceB, interfaceC)
	register("ISupportAImpl", nil, interfaceA)
	register("ISupportBImpl", nil, interfaceB)
	register("ISupportDImpl", nil, interfaceD)
	register("ISupportBCImpl", nil, interfaceB, interfaceC)
	register("ISupportBaseABImpl", nil, baseAB)
	register("ISupportAImplSuperGImpl", nil, superG)
	register("ISupportAImplSuperGImplPlus", nil, superG, interfaceB, interfaceC)

	engine := NewEngine(env)
	defer func() { _ = engine.Close(context.Background()) }()

	fieldEq := func(name, want string) Expression[bool] {
		return Equal[string](Field[any, string](name), Literal(want))
	}
	and := func(first Expression[bool], rest ...Expression[bool]) Expression[bool] {
		result := first
		for _, pred := range rest {
			result = And(result, pred)
		}
		return result
	}

	cases := []struct {
		name     string
		typeName string
		pred     Expression[bool]
		every    bool
		want     []string
	}{
		{"c=ISupportC", "ISupportC", nil, false, []string{"ISupportCImpl"}},
		{"baseab=ISupportBaseAB", "ISupportBaseAB", nil, false, []string{"ISupportABCImpl"}},
		{"every a=ISupportA", "ISupportA", nil, true, []string{"ISupportABCImpl", "ISupportAImpl", "ISupportAImplSuperGImplPlus", "ISupportAImplSuperGImpl"}},
		{"every a=ISupportB", "ISupportB", nil, true, []string{"ISupportABCImpl", "ISupportBImpl", "ISupportBCImpl", "ISupportAImplSuperGImplPlus"}},
		{"every a=ISupportB(b=B1)", "ISupportB", fieldEq("b", "B1"), true, []string{"ISupportABCImpl", "ISupportBImpl"}},
		{"every a=ISupportA(a=A3)", "ISupportA", fieldEq("a", "A3"), true, []string{"ISupportAImplSuperGImplPlus"}},
		{"every a=ISupportC(c=C2)", "ISupportC", fieldEq("c", "C2"), true, []string{"ISupportBCImpl", "ISupportAImplSuperGImplPlus"}},
		{"every a=ISupportC(c=C1)", "ISupportC", fieldEq("c", "C1"), true, []string{"ISupportCImpl", "ISupportABCImpl"}},
		{"every a=ISupportD(d=D1)", "ISupportD", fieldEq("d", "D1"), true, []string{"ISupportDImpl"}},
		{"every a=ISupportBaseD(baseD=BaseD)", "ISupportBaseD", fieldEq("baseD", "BaseD"), true, []string{"ISupportDImpl"}},
		{"every a=ISupportBaseDBase(baseDBase=BaseDBase)", "ISupportBaseDBase", fieldEq("baseDBase", "BaseDBase"), true, []string{"ISupportDImpl"}},
		{"every a=ISupportD(d,baseD,baseDBase)", "ISupportD", and(fieldEq("d", "D1"), fieldEq("baseD", "BaseD"), fieldEq("baseDBase", "BaseDBase")), true, []string{"ISupportDImpl"}},
		{"every a=ISupportBaseD(baseD,baseDBase)", "ISupportBaseD", and(fieldEq("baseD", "BaseD"), fieldEq("baseDBase", "BaseDBase")), true, []string{"ISupportDImpl"}},
		{"every a=SuperG", "ISupportAImplSuperG", nil, true, []string{"ISupportAImplSuperGImplPlus", "ISupportAImplSuperGImpl"}},
		{"every a=SuperG(g=G1)", "ISupportAImplSuperG", fieldEq("g", "G1"), true, []string{"ISupportAImplSuperGImplPlus"}},
		{"every a=SuperG(baseAB=BaseAB5)", "ISupportAImplSuperG", fieldEq("baseAB", "BaseAB5"), true, []string{"ISupportAImplSuperGImpl"}},
		{"every a=SuperG(baseAB,g,a)", "ISupportAImplSuperG", and(fieldEq("baseAB", "BaseAB4"), fieldEq("g", "G1"), fieldEq("a", "A3")), true, []string{"ISupportAImplSuperGImplPlus"}},
		{"every a=SuperGImplPlus(all)", "ISupportAImplSuperGImplPlus", and(fieldEq("baseAB", "BaseAB4"), fieldEq("g", "G1"), fieldEq("a", "A3"), fieldEq("b", "B4"), fieldEq("c", "C2")), true, []string{"ISupportAImplSuperGImplPlus"}},
		{"every a=OverrideBase", "SupportOverrideBase", nil, true, []string{"SupportOverrideOneA", "SupportOverrideOneB", "SupportOverrideOne", "SupportOverrideBase"}},
		{"every a=OverrideOne", "SupportOverrideOne", nil, true, []string{"SupportOverrideOneA", "SupportOverrideOneB", "SupportOverrideOne"}},
		{"every a=OverrideOneA", "SupportOverrideOneA", nil, true, []string{"SupportOverrideOneA"}},
		{"every a=OverrideOneB", "SupportOverrideOneB", nil, true, []string{"SupportOverrideOneB"}},
		{"every a=OverrideBase(val=OB1)", "SupportOverrideBase", fieldEq("val", "OB1"), true, []string{"SupportOverrideOneB"}},
		{"every a=OverrideBase(val=O3)", "SupportOverrideBase", fieldEq("val", "O3"), true, []string{"SupportOverrideOne"}},
		{"every a=OverrideBase(val=OBase)", "SupportOverrideBase", fieldEq("val", "OBase"), true, []string{"SupportOverrideBase"}},
		{"every a=OverrideBase(val=O2)", "SupportOverrideBase", fieldEq("val", "O2"), true, nil},
		{"every a=OverrideOne(val=OA1)", "SupportOverrideOne", fieldEq("val", "OA1"), true, []string{"SupportOverrideOneA"}},
		{"every a=OverrideOne(val=O3)", "SupportOverrideOne", fieldEq("val", "O3"), true, []string{"SupportOverrideOne"}},
	}

	fires := make(map[string][]string)
	for _, testCase := range cases {
		pred := testCase.pred
		if pred == nil {
			pred = Literal(true)
		}
		pattern := PatternFromRecord(FromAny(env, testCase.typeName), "a", pred)
		if testCase.every {
			pattern = pattern.Every()
		}
		plan, err := env.Build(pattern.Select(Alias("events", TagEvents("a"))).Query(StatementName(testCase.name)))
		if err != nil {
			t.Fatalf("%s: build: %v", testCase.name, err)
		}
		deployment, err := engine.Deploy(context.Background(), plan)
		if err != nil {
			t.Fatalf("%s: deploy: %v", testCase.name, err)
		}
		name := testCase.name
		if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
			for _, result := range batch.New {
				row, ok := result.Row()
				if !ok {
					return fmt.Errorf("no row")
				}
				events, _ := row.Get("events").Any().([]Event)
				if len(events) == 0 {
					return fmt.Errorf("empty tag events")
				}
				fires[name] = append(fires[name], events[0].TypeName())
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deployment.Undeploy(context.Background()) }()
	}

	// EventCollectionFactory.getSetFiveInterfaces, in order.
	events := []struct {
		typeName string
		record   map[string]any
	}{
		{"ISupportCImpl", map[string]any{"c": "C1"}},
		{"ISupportABCImpl", map[string]any{"a": "A1", "b": "B1", "baseAB": "BaseB", "c": "C1"}},
		{"ISupportAImpl", map[string]any{"a": "A1", "baseAB": "BaseAB"}},
		{"ISupportBImpl", map[string]any{"b": "B1", "baseAB": "BaseAB"}},
		{"ISupportDImpl", map[string]any{"d": "D1", "baseD": "BaseD", "baseDBase": "BaseDBase"}},
		{"ISupportBCImpl", map[string]any{"b": "B2", "baseAB": "BaseAB2", "c": "C2"}},
		{"ISupportBaseABImpl", map[string]any{"baseAB": "BaseAB3"}},
		{"SupportOverrideOneA", map[string]any{"val": "OA1"}},
		{"SupportOverrideOneB", map[string]any{"val": "OB1"}},
		{"SupportOverrideOne", map[string]any{"val": "O3"}},
		{"SupportOverrideBase", map[string]any{"val": "OBase"}},
		{"ISupportAImplSuperGImplPlus", map[string]any{"g": "G1", "a": "A3", "baseAB": "BaseAB4", "b": "B4", "c": "C2"}},
		{"ISupportAImplSuperGImpl", map[string]any{"g": "G2", "a": "A14", "baseAB": "BaseAB5"}},
	}
	for _, event := range events {
		if err := engine.SendRecord(context.Background(), event.typeName, event.record); err != nil {
			t.Fatalf("send %s: %v", event.typeName, err)
		}
	}

	for _, testCase := range cases {
		got := append([]string(nil), fires[testCase.name]...)
		sort.Strings(got)
		want := append([]string(nil), testCase.want...)
		sort.Strings(want)
		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
			t.Errorf("%s: fires = %v, want %v", testCase.name, got, want)
		}
	}
}
