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

const (
	infraNWOnDeleteIndexesID         = "infra-namedwindow-on-delete-indexes"
	infraNWOnDeleteIndexesJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraNWOnDeleteIndexesSource     = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java"
)

const infraNWOnDeleteIndexesDescription = "InfraNamedWindowOnDelete implicit-index semantics: staggered cross-window on-delete (ord 1, DEFAULT event representation only) plus the coercion-key, coercion-range and coercion-key-range multi-prop index executions (ords 2-4). index-count records pin getIndexDescriptors().length (of=indexes) or getCountDataWindow() (of=rows, ord 1) via the create statement's deployment; snapshot records pin the create statement iterator in insertion order; listener records capture createOne/createTwo/delete deliveries (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java)."

var infraNWOnDeleteIndexesCaseObservations = []string{
	"listener; two keepall windows fed by filtered insert-intos, a cross-window on-delete trigger, per-statement undeploys; window row counts and insertion-order iterator snapshots recorded as index-count(of=rows) and snapshot records",
	"listener; seven on-delete hash-index variants over a five-prop keepall window pin implicit index counts 1,1,2,3,4,5,6 (coercion type and prop order decide reuse), then a late delete on the filled window, then phase B on-SELECT implicit indexes over WinOne",
	"listener; six on-delete range-index variants over SupportBeanTwo triggers pin implicit index counts 1,2,3,4,4,4 (between creates a range prop, <= and not between reuse-or-scan); null bounds and reversed ranges do not match",
	"listener; four on-delete mixed hash+range variants over SupportBeanTwo triggers pin implicit index counts 1,2,3,4 (mixed props live in one index, hash order and coercion type decide reuse)",
}

var infraNWOnDeleteIndexesCaseEPLs = []string{
	"@name('createOne') @public create window MyWindowSTAG#keepall as select theString as a1, intPrimitive as b1 from SupportBean\n @name('createTwo') @public create window MyWindowSTAGTwo#keepall as select theString as a2, intPrimitive as b2 from SupportBean\n@name('delete') on MyWindowSTAG delete from MyWindowSTAGTwo where a1 = a2\n@name('insert') insert into MyWindowSTAG select theString as a1, intPrimitive as b1 from SupportBean(intPrimitive > 0)\n@name('insertTwo') insert into MyWindowSTAGTwo select theString as a2, intPrimitive as b2 from SupportBean(intPrimitive < 0)",
	"@name('createOne') @public create window MyWindowCK#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean\n@name('d1') on SupportBean(theString='DB') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.doubleBoxed\n@name('d2') on SupportBean(theString='DP') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.doublePrimitive\n@name('d3') on SupportBean(theString='IB') as s0 delete from MyWindowCK where MyWindowCK.intPrimitive = s0.intBoxed\n@name('d4') on SupportBean(theString='IPDP') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive\n@name('d5') on SupportBean(theString='IPDP2') as s0 delete from MyWindowCK as win where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive\n@name('d6') on SupportBean(theString='IPDPIB') as s0 delete from MyWindowCK as win where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive and win.intBoxed = s0.intBoxed\n@name('d7') on SupportBean(theString='CAST') as s0 delete from MyWindowCK as win where win.intBoxed = s0.intPrimitive and win.doublePrimitive = s0.doubleBoxed and win.intPrimitive = s0.intBoxed\ninsert into MyWindowCK select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean(theString like 'E%')\n@name('d0') on SupportBean(theString='LAST') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive\n@name('createTwo') @public create window WinOne#keepall as SupportBean\non SupportBean_ST0 select * from WinOne where theString = key0\non SupportBean_ST0 select * from WinOne where theString = key0 and intPrimitive = p00",
	"@name('createOne') @public create window MyWindowCR#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean\ninsert into MyWindowCR select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean\n@name('d0') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.doublePrimitiveTwo and s2.doubleBoxedTwo\n@name('d1') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo\n@name('d2') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo and win.doublePrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo\n@name('d3') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.doublePrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo and win.intPrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo\n@name('d4') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive <= doublePrimitiveTwo\n@name('d5') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive not between s2.intPrimitiveTwo and s2.intBoxedTwo",
	"@name('createOne') @public create window MyWindowCKR#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean\ninsert into MyWindowCKR select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean\n@name('d0') on SupportBeanTwo delete from MyWindowCKR where theString = stringTwo and intPrimitive between doublePrimitiveTwo and doubleBoxedTwo\n@name('d1') on SupportBeanTwo delete from MyWindowCKR where theString = stringTwo and intPrimitive = intPrimitiveTwo and intBoxed between doublePrimitiveTwo and doubleBoxedTwo\n@name('d2') on SupportBeanTwo delete from MyWindowCKR where intBoxed between doubleBoxedTwo and doublePrimitiveTwo and intPrimitive = intPrimitiveTwo and theString = stringTwo \n@name('d3') on SupportBeanTwo delete from MyWindowCKR where intBoxed between intBoxedTwo and intBoxedTwo and intPrimitive = intPrimitiveTwo and theString = stringTwo ",
}

var (
	infraNWOnDeleteIndexesJavaRuntimeIDs = []string{
		"java-runtime-0dcb2b72f505c7931a45",
		"java-runtime-c4c336036fdd92d803f7",
		"java-runtime-a58ae70ca908579af50a",
		"java-runtime-2d6d018e91b5664f3734",
	}
	infraNWOnDeleteIndexesJavaExecutions = []string{
		"InfraStaggeredNamedWindow",
		"InfraCoercionKeyMultiPropIndexes",
		"InfraCoercionRangeMultiPropIndexes",
		"InfraCoercionKeyAndRangeMultiPropIndexes",
	}
	infraNWOnDeleteIndexesJavaStaticIDs = []string{
		"java-06bf0eb71230b3119293",
		"java-06bf0eb71230b3119293",
		"java-06bf0eb71230b3119293",
		"java-06bf0eb71230b3119293",
	}
	infraNWOnDeleteIndexesJavaFlags = []string{"STATICHOOK"}
	infraNWOnDeleteIndexesCases     = []string{
		"staggered",
		"coercion-key",
		"coercion-range",
		"coercion-key-range",
	}
	infraNWOnDeleteIndexesOrdinals = []int{1, 2, 3, 4}
	infraNWOnDeleteIndexesSources  = []string{infraNWOnDeleteIndexesSource}
)

// infraNWOnDeleteIndexesBean mirrors SupportBean for the coercion cases:
// theString/intPrimitive/intBoxed/doublePrimitive/doubleBoxed.
type infraNWOnDeleteIndexesBean struct {
	TheString       string   `esper:"theString"`
	IntPrimitive    int64    `esper:"intPrimitive"`
	IntBoxed        *int64   `esper:"intBoxed"`
	DoublePrimitive float64  `esper:"doublePrimitive"`
	DoubleBoxed     *float64 `esper:"doubleBoxed"`
}

// infraNWOnDeleteIndexesBeanTwo mirrors SupportBeanTwo.
type infraNWOnDeleteIndexesBeanTwo struct {
	StringTwo          string   `esper:"stringTwo"`
	IntPrimitiveTwo    int64    `esper:"intPrimitiveTwo"`
	IntBoxedTwo        *int64   `esper:"intBoxedTwo"`
	DoublePrimitiveTwo float64  `esper:"doublePrimitiveTwo"`
	DoubleBoxedTwo     *float64 `esper:"doubleBoxedTwo"`
}

// infraNWOnDeleteIndexesST0 mirrors SupportBean_ST0.
type infraNWOnDeleteIndexesST0 struct {
	ID   string `esper:"id"`
	Key0 string `esper:"key0"`
	P00  int64  `esper:"p00"`
}

func decodeInfraNWOnDeleteIndexesPayload(step compat.Step) (any, error) {
	var fields map[string]json.RawMessage
	if err := strictObject(step.Payload, &fields); err != nil {
		return nil, fmt.Errorf("decode %s payload: %w", step.EventType, err)
	}
	decode := func(target any) error {
		for key, raw := range fields {
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decode %s.%s: %w", step.EventType, key, err)
			}
			switch typed := target.(type) {
			case *infraNWOnDeleteIndexesBean:
				switch key {
				case "theString":
					typed.TheString = stringField(value)
				case "intPrimitive":
					typed.IntPrimitive = int64Field(value)
				case "intBoxed":
					typed.IntBoxed = int64PtrField(value)
				case "doublePrimitive":
					typed.DoublePrimitive = float64Field(value)
				case "doubleBoxed":
					typed.DoubleBoxed = float64PtrField(value)
				default:
					return fmt.Errorf("unknown %s field %q", step.EventType, key)
				}
			case *infraNWOnDeleteIndexesBeanTwo:
				switch key {
				case "stringTwo":
					typed.StringTwo = stringField(value)
				case "intPrimitiveTwo":
					typed.IntPrimitiveTwo = int64Field(value)
				case "intBoxedTwo":
					typed.IntBoxedTwo = int64PtrField(value)
				case "doublePrimitiveTwo":
					typed.DoublePrimitiveTwo = float64Field(value)
				case "doubleBoxedTwo":
					typed.DoubleBoxedTwo = float64PtrField(value)
				default:
					return fmt.Errorf("unknown %s field %q", step.EventType, key)
				}
			case *infraNWOnDeleteIndexesST0:
				switch key {
				case "id":
					typed.ID = stringField(value)
				case "key0":
					typed.Key0 = stringField(value)
				case "p00":
					typed.P00 = int64Field(value)
				default:
					return fmt.Errorf("unknown %s field %q", step.EventType, key)
				}
			}
		}
		return nil
	}
	switch step.EventType {
	case "SupportBean":
		var bean infraNWOnDeleteIndexesBean
		if err := decode(&bean); err != nil {
			return nil, err
		}
		return bean, nil
	case "SupportBeanTwo":
		var bean infraNWOnDeleteIndexesBeanTwo
		if err := decode(&bean); err != nil {
			return nil, err
		}
		return bean, nil
	case "SupportBean_ST0":
		var bean infraNWOnDeleteIndexesST0
		if err := decode(&bean); err != nil {
			return nil, err
		}
		return bean, nil
	default:
		return nil, fmt.Errorf("unsupported event type %q", step.EventType)
	}
}

func stringField(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func int64Field(value any) int64 {
	switch n := value.(type) {
	case float64:
		return int64(n)
	case json.Number:
		v, _ := n.Int64()
		return v
	}
	return 0
}

func int64PtrField(value any) *int64 {
	if value == nil {
		return nil
	}
	v := int64Field(value)
	return &v
}

func float64Field(value any) float64 {
	switch n := value.(type) {
	case float64:
		return n
	case json.Number:
		v, _ := n.Float64()
		return v
	}
	return 0
}

func float64PtrField(value any) *float64 {
	if value == nil {
		return nil
	}
	v := float64Field(value)
	return &v
}

// infraNWOnDeleteIndexesBuild builds the plan for one deploy step. The
// create statements also register their named window on the environment
// before the plan is built (Java's create-window statement does both).
func infraNWOnDeleteIndexesBuild(env *esper.Environment, caseName, statement, epl string) (esper.Plan, error) {
	bean := esper.From[infraNWOnDeleteIndexesBean](env, "SupportBean")
	beanTwo := esper.From[infraNWOnDeleteIndexesBeanTwo](env, "SupportBeanTwo")
	st0 := esper.From[infraNWOnDeleteIndexesST0](env, "SupportBean_ST0")
	beanField := func(name string) esper.Expression[string] {
		return esper.Field[infraNWOnDeleteIndexesBean, string](name)
	}
	beanInt := func(name string) esper.Expression[int64] {
		return esper.Field[infraNWOnDeleteIndexesBean, int64](name)
	}
	beanIntBoxed := func(name string) esper.Expression[*int64] {
		return esper.Field[infraNWOnDeleteIndexesBean, *int64](name)
	}
	beanDouble := func(name string) esper.Expression[float64] {
		return esper.Field[infraNWOnDeleteIndexesBean, float64](name)
	}
	beanDoubleBoxed := func(name string) esper.Expression[*float64] {
		return esper.Field[infraNWOnDeleteIndexesBean, *float64](name)
	}
	twoField := func(name string) esper.Expression[string] {
		return esper.Field[infraNWOnDeleteIndexesBeanTwo, string](name)
	}
	twoInt := func(name string) esper.Expression[int64] {
		return esper.Field[infraNWOnDeleteIndexesBeanTwo, int64](name)
	}
	twoIntBoxed := func(name string) esper.Expression[*int64] {
		return esper.Field[infraNWOnDeleteIndexesBeanTwo, *int64](name)
	}
	twoDouble := func(name string) esper.Expression[float64] {
		return esper.Field[infraNWOnDeleteIndexesBeanTwo, float64](name)
	}
	twoDoubleBoxed := func(name string) esper.Expression[*float64] {
		return esper.Field[infraNWOnDeleteIndexesBeanTwo, *float64](name)
	}
	nwStr := func(name string) esper.Expression[string] {
		return esper.NamedWindowField[string](name)
	}
	nwInt := func(name string) esper.Expression[int64] {
		return esper.NamedWindowField[int64](name)
	}
	nwIntBoxed := func(name string) esper.Expression[*int64] {
		return esper.NamedWindowField[*int64](name)
	}
	nwDouble := func(name string) esper.Expression[float64] {
		return esper.NamedWindowField[float64](name)
	}
	beanFilter := func(value string) esper.Expression[bool] {
		return esper.Equal[string](beanField("theString"), esper.Literal(value))
	}
	createWindow := func(name string, schema esper.Schema) error {
		_, err := esper.CreateNamedWindow(env, name, schema, esper.NamedWindowRetention(esper.KeepAll()))
		return err
	}
	mapSchema := func(name string, fields ...esper.FieldSpec) (esper.Schema, error) {
		schema, err := esper.NewMapSchema(name, fields)
		if err != nil {
			return esper.Schema{}, err
		}
		if err := env.RegisterSchema(schema); err != nil {
			return esper.Schema{}, err
		}
		return schema, nil
	}
	beanSchema := func() (esper.Schema, error) {
		schema, ok := env.Schema("SupportBean")
		if !ok {
			return esper.Schema{}, fmt.Errorf("SupportBean schema not registered")
		}
		return schema, nil
	}
	// create builds the direct-child select that stands in for the Java
	// create-window statement's own listener (see the silent-delete unit).
	create := func(window, stmtName string) (esper.Plan, error) {
		return env.Build(esper.FromNamedWindow(env, window).
			CreateNamedWindowQuery(esper.StatementName(stmtName), esper.WithOldStream()))
	}
	deleteOnBean := func(window, stmtName, filter string, pred esper.Expression[bool]) (esper.Plan, error) {
		return env.Build(esper.OnEvent(bean.Filter(beanFilter(filter))).
			DeleteFromNamedWindow(window, pred).Query(esper.StatementName(stmtName)))
	}
	deleteOnTwo := func(window, stmtName string, pred esper.Expression[bool]) (esper.Plan, error) {
		return env.Build(esper.OnEvent(beanTwo).
			DeleteFromNamedWindow(window, pred).Query(esper.StatementName(stmtName)))
	}
	insertFiltered := func(window, stmtName string, pred esper.Expression[bool], assignments ...esper.TableAssignment) (esper.Plan, error) {
		stream := bean
		if pred != nil {
			stream = stream.Filter(pred)
		}
		return env.Build(esper.OnEvent(stream).
			InsertIntoNamedWindow(window, assignments...).Query(esper.StatementName(stmtName)))
	}

	switch caseName {
	case "staggered":
		switch statement {
		case "createOne":
			schema, err := mapSchema("MyWindowSTAGSchema",
				esper.FieldDef("a1", reflect.TypeOf("")),
				esper.FieldDef("b1", reflect.TypeOf(int64(0))))
			if err != nil {
				return esper.Plan{}, err
			}
			if err := createWindow("MyWindowSTAG", schema); err != nil {
				return esper.Plan{}, err
			}
			return create("MyWindowSTAG", "createOne")
		case "createTwo":
			schema, err := mapSchema("MyWindowSTAGTwoSchema",
				esper.FieldDef("a2", reflect.TypeOf("")),
				esper.FieldDef("b2", reflect.TypeOf(int64(0))))
			if err != nil {
				return esper.Plan{}, err
			}
			if err := createWindow("MyWindowSTAGTwo", schema); err != nil {
				return esper.Plan{}, err
			}
			return create("MyWindowSTAGTwo", "createTwo")
		case "delete":
			return env.Build(esper.OnRecord(esper.FromNamedWindow(env, "MyWindowSTAG")).
				DeleteFromNamedWindow("MyWindowSTAGTwo",
					esper.EqualOf(esper.Field[any, string]("a1"), nwStr("a2"))).
				Query(esper.StatementName("delete")))
		case "insert":
			return insertFiltered("MyWindowSTAG", "insert",
				esper.Greater[int64](beanInt("intPrimitive"), esper.Literal(int64(0))),
				esper.SetColumn("a1", beanField("theString")),
				esper.SetColumn("b1", beanInt("intPrimitive")))
		case "insertTwo":
			return insertFiltered("MyWindowSTAGTwo", "insertTwo",
				esper.Less[int64](beanInt("intPrimitive"), esper.Literal(int64(0))),
				esper.SetColumn("a2", beanField("theString")),
				esper.SetColumn("b2", beanInt("intPrimitive")))
		}
	case "coercion-key":
		switch statement {
		case "createOne":
			schema, err := mapSchema("MyWindowCKSchema",
				esper.FieldDef("theString", reflect.TypeOf("")),
				esper.FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
				esper.FieldDef("intBoxed", reflect.TypeOf((*int64)(nil))),
				esper.FieldDef("doublePrimitive", reflect.TypeOf(float64(0))),
				esper.FieldDef("doubleBoxed", reflect.TypeOf((*float64)(nil))))
			if err != nil {
				return esper.Plan{}, err
			}
			if err := createWindow("MyWindowCK", schema); err != nil {
				return esper.Plan{}, err
			}
			return create("MyWindowCK", "createOne")
		case "d1":
			return deleteOnBean("MyWindowCK", "d1", "DB",
				esper.EqualOf(nwInt("intPrimitive"), beanDoubleBoxed("doubleBoxed")))
		case "d2":
			return deleteOnBean("MyWindowCK", "d2", "DP",
				esper.EqualOf(nwInt("intPrimitive"), beanDouble("doublePrimitive")))
		case "d3":
			return deleteOnBean("MyWindowCK", "d3", "IB",
				esper.EqualOf(nwInt("intPrimitive"), beanIntBoxed("intBoxed")))
		case "d4":
			return deleteOnBean("MyWindowCK", "d4", "IPDP", esper.And(
				esper.EqualOf(nwInt("intPrimitive"), beanInt("intPrimitive")),
				esper.EqualOf(nwDouble("doublePrimitive"), beanDouble("doublePrimitive"))))
		case "d5":
			return deleteOnBean("MyWindowCK", "d5", "IPDP2", esper.And(
				esper.EqualOf(nwDouble("doublePrimitive"), beanDouble("doublePrimitive")),
				esper.EqualOf(nwInt("intPrimitive"), beanInt("intPrimitive"))))
		case "d6":
			return deleteOnBean("MyWindowCK", "d6", "IPDPIB", esper.And(
				esper.EqualOf(nwDouble("doublePrimitive"), beanDouble("doublePrimitive")),
				esper.And(
					esper.EqualOf(nwInt("intPrimitive"), beanInt("intPrimitive")),
					esper.EqualOf(nwIntBoxed("intBoxed"), beanIntBoxed("intBoxed")))))
		case "d7":
			return deleteOnBean("MyWindowCK", "d7", "CAST", esper.And(
				esper.EqualOf(nwIntBoxed("intBoxed"), beanInt("intPrimitive")),
				esper.And(
					esper.EqualOf(nwDouble("doublePrimitive"), beanDoubleBoxed("doubleBoxed")),
					esper.EqualOf(nwInt("intPrimitive"), beanIntBoxed("intBoxed")))))
		case "d0":
			return deleteOnBean("MyWindowCK", "d0", "LAST", esper.And(
				esper.EqualOf(nwInt("intPrimitive"), beanInt("intPrimitive")),
				esper.EqualOf(nwDouble("doublePrimitive"), beanDouble("doublePrimitive"))))
		case "insert":
			return insertFiltered("MyWindowCK", "insert",
				esper.Like(beanField("theString"), esper.Literal("E%")),
				esper.CopyMatchingFields())
		case "createTwo":
			schema, err := beanSchema()
			if err != nil {
				return esper.Plan{}, err
			}
			if err := createWindow("WinOne", schema); err != nil {
				return esper.Plan{}, err
			}
			return create("WinOne", "createTwo")
		case "select1":
			return env.Build(esper.OnEvent(st0).
				SelectFromNamedWindow("WinOne",
					esper.EqualOf(nwStr("theString"), esper.Field[infraNWOnDeleteIndexesST0, string]("key0"))).
				Query(esper.StatementName("select1")))
		case "select2":
			return env.Build(esper.OnEvent(st0).
				SelectFromNamedWindow("WinOne", esper.And(
					esper.EqualOf(nwStr("theString"), esper.Field[infraNWOnDeleteIndexesST0, string]("key0")),
					esper.EqualOf(nwInt("intPrimitive"), esper.Field[infraNWOnDeleteIndexesST0, int64]("p00")))).
				Query(esper.StatementName("select2")))
		}
	case "coercion-range":
		switch statement {
		case "createOne":
			schema, err := mapSchema("MyWindowCRSchema",
				esper.FieldDef("theString", reflect.TypeOf("")),
				esper.FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
				esper.FieldDef("intBoxed", reflect.TypeOf((*int64)(nil))),
				esper.FieldDef("doublePrimitive", reflect.TypeOf(float64(0))),
				esper.FieldDef("doubleBoxed", reflect.TypeOf((*float64)(nil))))
			if err != nil {
				return esper.Plan{}, err
			}
			if err := createWindow("MyWindowCR", schema); err != nil {
				return esper.Plan{}, err
			}
			return create("MyWindowCR", "createOne")
		case "insert":
			return insertFiltered("MyWindowCR", "insert", nil, esper.CopyMatchingFields())
		case "d0":
			return deleteOnTwo("MyWindowCR", "d0",
				esper.BetweenOf(nwInt("intPrimitive"), twoDouble("doublePrimitiveTwo"), twoDoubleBoxed("doubleBoxedTwo")))
		case "d1":
			return deleteOnTwo("MyWindowCR", "d1",
				esper.BetweenOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo"), twoIntBoxed("intBoxedTwo")))
		case "d2":
			return deleteOnTwo("MyWindowCR", "d2", esper.And(
				esper.BetweenOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo"), twoIntBoxed("intBoxedTwo")),
				esper.BetweenOf(nwDouble("doublePrimitive"), twoInt("intPrimitiveTwo"), twoIntBoxed("intBoxedTwo"))))
		case "d3":
			return deleteOnTwo("MyWindowCR", "d3", esper.And(
				esper.BetweenOf(nwDouble("doublePrimitive"), twoInt("intPrimitiveTwo"), twoInt("intPrimitiveTwo")),
				esper.BetweenOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo"), twoInt("intPrimitiveTwo"))))
		case "d4":
			return deleteOnTwo("MyWindowCR", "d4",
				esper.LessOrEqualOf(nwInt("intPrimitive"), twoDouble("doublePrimitiveTwo")))
		case "d5":
			return deleteOnTwo("MyWindowCR", "d5",
				esper.NotBetweenOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo"), twoIntBoxed("intBoxedTwo")))
		}
	case "coercion-key-range":
		switch statement {
		case "createOne":
			schema, err := mapSchema("MyWindowCKRSchema",
				esper.FieldDef("theString", reflect.TypeOf("")),
				esper.FieldDef("intPrimitive", reflect.TypeOf(int64(0))),
				esper.FieldDef("intBoxed", reflect.TypeOf((*int64)(nil))),
				esper.FieldDef("doublePrimitive", reflect.TypeOf(float64(0))),
				esper.FieldDef("doubleBoxed", reflect.TypeOf((*float64)(nil))))
			if err != nil {
				return esper.Plan{}, err
			}
			if err := createWindow("MyWindowCKR", schema); err != nil {
				return esper.Plan{}, err
			}
			return create("MyWindowCKR", "createOne")
		case "insert":
			return insertFiltered("MyWindowCKR", "insert", nil, esper.CopyMatchingFields())
		case "d0":
			return deleteOnTwo("MyWindowCKR", "d0", esper.And(
				esper.EqualOf(nwStr("theString"), twoField("stringTwo")),
				esper.BetweenOf(nwInt("intPrimitive"), twoDouble("doublePrimitiveTwo"), twoDoubleBoxed("doubleBoxedTwo"))))
		case "d1":
			return deleteOnTwo("MyWindowCKR", "d1", esper.And(
				esper.EqualOf(nwStr("theString"), twoField("stringTwo")),
				esper.And(
					esper.EqualOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo")),
					esper.BetweenOf(nwIntBoxed("intBoxed"), twoDouble("doublePrimitiveTwo"), twoDoubleBoxed("doubleBoxedTwo")))))
		case "d2":
			return deleteOnTwo("MyWindowCKR", "d2", esper.And(
				esper.BetweenOf(nwIntBoxed("intBoxed"), twoDoubleBoxed("doubleBoxedTwo"), twoDouble("doublePrimitiveTwo")),
				esper.And(
					esper.EqualOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo")),
					esper.EqualOf(nwStr("theString"), twoField("stringTwo")))))
		case "d3":
			return deleteOnTwo("MyWindowCKR", "d3", esper.And(
				esper.BetweenOf(nwIntBoxed("intBoxed"), twoIntBoxed("intBoxedTwo"), twoIntBoxed("intBoxedTwo")),
				esper.And(
					esper.EqualOf(nwInt("intPrimitive"), twoInt("intPrimitiveTwo")),
					esper.EqualOf(nwStr("theString"), twoField("stringTwo")))))
		}
	}
	return esper.Plan{}, fmt.Errorf("%s case %q statement %q has no plan", infraNWOnDeleteIndexesID, caseName, statement)
}

// infraNWOnDeleteIndexesListened mirrors the Java oracle's LISTENED set:
// the create statements plus the staggered delete.
func infraNWOnDeleteIndexesListened(statement string) bool {
	switch statement {
	case "createOne", "createTwo", "delete":
		return true
	}
	return false
}

func executeInfraNWOnDeleteIndexes(ctx context.Context, steps []compat.Step) (compat.Trace, error) {
	caseName := ""
	caseIndex := -1
	var env *esper.Environment
	var engine *esper.Engine
	deployments := map[string]*esper.Deployment{}
	statements := map[string]*esper.Statement{}
	sequence := map[string]uint64{}
	trace := compat.Trace{Version: "esper-parity/v1", ID: infraNWOnDeleteIndexesID}
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
		if _, err := esper.RegisterStruct[infraNWOnDeleteIndexesBean](env, "SupportBean"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNWOnDeleteIndexesBeanTwo](env, "SupportBeanTwo"); err != nil {
			return err
		}
		if _, err := esper.RegisterStruct[infraNWOnDeleteIndexesST0](env, "SupportBean_ST0"); err != nil {
			return err
		}
		engine = esper.NewEngine(env,
			esper.WithRuntimeURI(infraNWOnDeleteIndexesJavaRuntimeIDs[caseIndex]),
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
			statements = map[string]*esper.Statement{}
			if err := startCase(); err != nil {
				return compat.Trace{}, err
			}
		case "deploy":
			plan, err := infraNWOnDeleteIndexesBuild(env, caseName, step.Statement, step.Epl)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("build %q/%q: %w", caseName, step.Statement, err)
			}
			deployment, err := engine.Deploy(ctx, plan)
			if err != nil {
				return compat.Trace{}, fmt.Errorf("deploy %q/%q: %w", caseName, step.Statement, err)
			}
			deployments[step.Statement] = deployment
			for _, statement := range deployment.Statements() {
				statements[step.Statement] = statement
				if infraNWOnDeleteIndexesListened(statement.Name()) {
					name := statement.Name()
					if _, err := statement.Subscribe(func(_ context.Context, batch esper.ResultBatch) error {
						record(name, batch)
						return nil
					}); err != nil {
						return compat.Trace{}, err
					}
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
			event, err := decodeInfraNWOnDeleteIndexesPayload(step)
			if err != nil {
				return compat.Trace{}, err
			}
			if err := engine.Send(ctx, step.EventType, event); err != nil {
				return compat.Trace{}, fmt.Errorf("send %s: %w", step.EventType, err)
			}
		case "snapshot":
			statement, ok := statements[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("snapshot targets unknown statement %q", step.Statement)
			}
			result, err := statement.Snapshot(ctx)
			if err != nil {
				return compat.Trace{}, err
			}
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "snapshot",
				Statement: step.Statement,
				Sequence:  0,
				Time:      compat.FormatTraceTime(engine.Now()),
				New:       compat.NormalizeResults(result.Batch.New),
			})
		case "index-count":
			window, ok := engine.NamedWindow(step.Statement)
			if !ok {
				return compat.Trace{}, fmt.Errorf("index-count targets unknown window %q", step.Statement)
			}
			var count int64
			switch step.Of {
			case "rows":
				rows, err := window.Snapshot(ctx)
				if err != nil {
					return compat.Trace{}, err
				}
				count = int64(len(rows))
			case "indexes":
				count = int64(window.IndexCount())
			default:
				return compat.Trace{}, fmt.Errorf("index-count has unknown kind %q", step.Of)
			}
			if step.Count != nil && *step.Count != count {
				return compat.Trace{}, fmt.Errorf("index-count mismatch for %s (%s): expected %d, got %d",
					step.Statement, step.Of, *step.Count, count)
			}
			sequence[step.Statement+":index-count"]++
			trace.Records = append(trace.Records, compat.TraceRecord{
				Case:      caseName,
				Operation: "index-count",
				Statement: step.Statement,
				Sequence:  sequence[step.Statement+":index-count"],
				Time:      compat.FormatTraceTime(engine.Now()),
				Count:     &count,
			})
		case "undeploy":
			deployment, ok := deployments[step.Statement]
			if !ok {
				return compat.Trace{}, fmt.Errorf("undeploy targets unknown statement %q", step.Statement)
			}
			if err := deployment.Undeploy(ctx); err != nil {
				return compat.Trace{}, err
			}
			delete(deployments, step.Statement)
			delete(statements, step.Statement)
		case "undeploy-all":
			for name, deployment := range deployments {
				if err := deployment.Undeploy(ctx); err != nil {
					return compat.Trace{}, fmt.Errorf("undeploy-all %q: %w", name, err)
				}
			}
			deployments = map[string]*esper.Deployment{}
			statements = map[string]*esper.Statement{}
		default:
			return compat.Trace{}, fmt.Errorf("unsupported op %q", step.Op)
		}
	}
	return trace, nil
}

func runInfraNWOnDeleteIndexesScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	return executeInfraNWOnDeleteIndexes(ctx, scenario.Steps)
}

func loadInfraNWOnDeleteIndexesScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario reader is required", infraNWOnDeleteIndexesID)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read %s scenario: %w", infraNWOnDeleteIndexesID, err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOnDeleteIndexesID, err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode %s scenario: %w", infraNWOnDeleteIndexesID, err)
	}
	if err := requireInfraNWOnDeleteIndexesFields(root,
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
		return compat.Scenario{}, fmt.Errorf("decode %s metadata: %w", infraNWOnDeleteIndexesID, err)
	}
	if metadata.Version != compat.ScenarioVersion || metadata.ID != infraNWOnDeleteIndexesID ||
		metadata.Description != infraNWOnDeleteIndexesDescription ||
		metadata.JavaCommit != infraNWOnDeleteIndexesJavaCommit ||
		metadata.JavaSource != infraNWOnDeleteIndexesSource {
		return compat.Scenario{}, fmt.Errorf("%s scenario metadata is not pinned", infraNWOnDeleteIndexesID)
	}
	if err := validateInfraNWOnDeleteIndexesStringArray(root["javaRuntimes"], infraNWOnDeleteIndexesJavaRuntimeIDs, "javaRuntimes"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOnDeleteIndexesStringArray(root["javaNames"], infraNWOnDeleteIndexesJavaExecutions, "javaNames"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOnDeleteIndexesStringArray(root["javaStaticIds"], infraNWOnDeleteIndexesJavaStaticIDs, "javaStaticIds"); err != nil {
		return compat.Scenario{}, err
	}
	if err := validateInfraNWOnDeleteIndexesStringArray(root["javaFlags"], infraNWOnDeleteIndexesJavaFlags, "javaFlags"); err != nil {
		return compat.Scenario{}, err
	}

	var rawCases []json.RawMessage
	if err := json.Unmarshal(root["cases"], &rawCases); err != nil || len(rawCases) != len(infraNWOnDeleteIndexesCases) {
		return compat.Scenario{}, fmt.Errorf("%s scenario must contain exactly %d cases", infraNWOnDeleteIndexesID, len(infraNWOnDeleteIndexesCases))
	}
	for index, rawCase := range rawCases {
		var object map[string]json.RawMessage
		if err := strictObject(rawCase, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario case %d: %w", index, err)
		}
		if err := requireInfraNWOnDeleteIndexesFields(object,
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
		if definition.Case != infraNWOnDeleteIndexesCases[index] ||
			definition.Ordinal != infraNWOnDeleteIndexesOrdinals[index] ||
			definition.RuntimeID != infraNWOnDeleteIndexesJavaRuntimeIDs[index] ||
			definition.ExecutionName != infraNWOnDeleteIndexesJavaExecutions[index] ||
			definition.Observation != infraNWOnDeleteIndexesCaseObservations[index] ||
			definition.EPL != infraNWOnDeleteIndexesCaseEPLs[index] {
			return compat.Scenario{}, fmt.Errorf("%s scenario case %d metadata is not pinned", infraNWOnDeleteIndexesID, index)
		}
	}

	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("%s scenario steps must be an array", infraNWOnDeleteIndexesID)
	}
	steps := make([]compat.Step, len(rawSteps))
	operations := make([]string, len(rawSteps))
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
		}
		var operation string
		if err := json.Unmarshal(object["op"], &operation); err != nil {
			return compat.Scenario{}, fmt.Errorf("scenario step %d op must be a string", index)
		}
		operations[index] = operation
		switch operation {
		case "case":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deploy":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case", "statement", "epl"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "deployed":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "send":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case", "eventType", "payload"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			var payload struct {
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(rawStep, &payload); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
			if _, err := decodeInfraNWOnDeleteIndexesPayload(compat.Step{EventType: payload.EventType, Payload: payload.Payload}); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "snapshot":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "index-count":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case", "statement", "create", "of", "count"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case", "statement"); err != nil {
				return compat.Scenario{}, fmt.Errorf("scenario step %d: %w", index, err)
			}
		case "undeploy-all":
			if err := requireInfraNWOnDeleteIndexesFields(object, "op", "case"); err != nil {
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
	if err := validateInfraNWOnDeleteIndexesRawSteps(rawSteps, operations); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}

func requireInfraNWOnDeleteIndexesFields(object map[string]json.RawMessage, names ...string) error {
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

func validateInfraNWOnDeleteIndexesStringArray(raw json.RawMessage, expected []string, name string) error {
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

func validateInfraNWOnDeleteIndexesRawSteps(rawSteps []json.RawMessage, operations []string) error {
	offset := 0
	for _, caseName := range infraNWOnDeleteIndexesCases {
		want, ok := infraNWOnDeleteIndexesCaseSteps[caseName]
		if !ok {
			return fmt.Errorf("%s case %q has no pinned step sequence", infraNWOnDeleteIndexesID, caseName)
		}
		if offset+1+len(want) > len(rawSteps) {
			return fmt.Errorf("%s case %q is truncated", infraNWOnDeleteIndexesID, caseName)
		}
		if operations[offset] != "case" {
			return fmt.Errorf("%s case %q does not start with a case marker", infraNWOnDeleteIndexesID, caseName)
		}
		offset++
		for _, pinned := range want {
			var step struct {
				Op        string          `json:"op"`
				Case      string          `json:"case"`
				Statement string          `json:"statement"`
				EPL       string          `json:"epl"`
				EventType string          `json:"eventType"`
				Payload   json.RawMessage `json:"payload"`
				Create    string          `json:"create"`
				Of        string          `json:"of"`
				Count     *int64          `json:"count"`
			}
			if err := json.Unmarshal(rawSteps[offset], &step); err != nil {
				return fmt.Errorf("%s step %d: %w", infraNWOnDeleteIndexesID, offset, err)
			}
			if step.Case != caseName {
				return fmt.Errorf("%s step %d is not pinned for case %q", infraNWOnDeleteIndexesID, offset, caseName)
			}
			var key string
			switch step.Op {
			case "deploy":
				key = "deploy:" + step.Statement + ":" + step.EPL
			case "deployed":
				key = "deployed:" + step.Statement
			case "send":
				var payload map[string]any
				if err := json.Unmarshal(step.Payload, &payload); err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOnDeleteIndexesID, offset, err)
				}
				canonical, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("%s step %d send payload: %w", infraNWOnDeleteIndexesID, offset, err)
				}
				key = "send:" + step.EventType + ":" + string(canonical)
			case "index-count":
				if step.Count == nil {
					return fmt.Errorf("%s step %d index-count has no count", infraNWOnDeleteIndexesID, offset)
				}
				key = fmt.Sprintf("index-count:%s:%s:%s:%d", step.Statement, step.Create, step.Of, *step.Count)
			case "snapshot":
				key = "snapshot:" + step.Statement
			case "undeploy":
				key = "undeploy:" + step.Statement
			case "undeploy-all":
				key = "undeploy-all"
			default:
				return fmt.Errorf("%s step %d has unsupported op %q", infraNWOnDeleteIndexesID, offset, step.Op)
			}
			if key != pinned {
				return fmt.Errorf("%s step %d is not pinned: got %q want %q", infraNWOnDeleteIndexesID, offset, key, pinned)
			}
			offset++
		}
	}
	if offset != len(rawSteps) {
		return fmt.Errorf("%s scenario steps contain an unexpected suffix", infraNWOnDeleteIndexesID)
	}
	return nil
}

// infraNWOnDeleteIndexesCaseSteps pins the complete step sequence per
// case: markers, deploys with byte-exact EPL, deployed markers, sends
// with canonical payloads, index-count and snapshot reads, and
// per-statement undeploys.
var infraNWOnDeleteIndexesCaseSteps = map[string][]string{
	"staggered": {
		"deploy:createOne:@name('createOne') @public create window MyWindowSTAG#keepall as select theString as a1, intPrimitive as b1 from SupportBean",
		"deployed:createOne",
		"index-count:MyWindowSTAG:createOne:rows:0",
		"deploy:createTwo: @name('createTwo') @public create window MyWindowSTAGTwo#keepall as select theString as a2, intPrimitive as b2 from SupportBean",
		"deployed:createTwo",
		"index-count:MyWindowSTAGTwo:createTwo:rows:0",
		"deploy:delete:@name('delete') on MyWindowSTAG delete from MyWindowSTAGTwo where a1 = a2",
		"deployed:delete",
		"deploy:insert:@name('insert') insert into MyWindowSTAG select theString as a1, intPrimitive as b1 from SupportBean(intPrimitive > 0)",
		"deployed:insert",
		"deploy:insertTwo:@name('insertTwo') insert into MyWindowSTAGTwo select theString as a2, intPrimitive as b2 from SupportBean(intPrimitive < 0)",
		"deployed:insertTwo",
		"send:SupportBean:{\"intPrimitive\":-10,\"theString\":\"E1\"}",
		"snapshot:createTwo",
		"index-count:MyWindowSTAGTwo:createTwo:rows:1",
		"send:SupportBean:{\"intPrimitive\":5,\"theString\":\"E2\"}",
		"snapshot:createOne",
		"index-count:MyWindowSTAG:createOne:rows:1",
		"send:SupportBean:{\"intPrimitive\":-1,\"theString\":\"E3\"}",
		"snapshot:createTwo",
		"index-count:MyWindowSTAGTwo:createTwo:rows:2",
		"send:SupportBean:{\"intPrimitive\":1,\"theString\":\"E3\"}",
		"snapshot:createOne",
		"snapshot:createTwo",
		"index-count:MyWindowSTAG:createOne:rows:2",
		"index-count:MyWindowSTAGTwo:createTwo:rows:1",
		"undeploy:delete",
		"undeploy:insert",
		"undeploy:insertTwo",
		"undeploy:createOne",
		"undeploy:createTwo",
	},
	"coercion-key": {
		"deploy:createOne:@name('createOne') @public create window MyWindowCK#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean",
		"deployed:createOne",
		"deploy:d1:@name('d1') on SupportBean(theString='DB') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.doubleBoxed",
		"deployed:d1",
		"index-count:MyWindowCK:createOne:indexes:1",
		"deploy:d2:@name('d2') on SupportBean(theString='DP') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.doublePrimitive",
		"deployed:d2",
		"index-count:MyWindowCK:createOne:indexes:1",
		"deploy:d3:@name('d3') on SupportBean(theString='IB') as s0 delete from MyWindowCK where MyWindowCK.intPrimitive = s0.intBoxed",
		"deployed:d3",
		"index-count:MyWindowCK:createOne:indexes:2",
		"deploy:d4:@name('d4') on SupportBean(theString='IPDP') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive",
		"deployed:d4",
		"index-count:MyWindowCK:createOne:indexes:3",
		"deploy:d5:@name('d5') on SupportBean(theString='IPDP2') as s0 delete from MyWindowCK as win where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive",
		"deployed:d5",
		"index-count:MyWindowCK:createOne:indexes:4",
		"deploy:d6:@name('d6') on SupportBean(theString='IPDPIB') as s0 delete from MyWindowCK as win where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive and win.intBoxed = s0.intBoxed",
		"deployed:d6",
		"index-count:MyWindowCK:createOne:indexes:5",
		"deploy:d7:@name('d7') on SupportBean(theString='CAST') as s0 delete from MyWindowCK as win where win.intBoxed = s0.intPrimitive and win.doublePrimitive = s0.doubleBoxed and win.intPrimitive = s0.intBoxed",
		"deployed:d7",
		"index-count:MyWindowCK:createOne:indexes:6",
		"deploy:insert:insert into MyWindowCK select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean(theString like 'E%')",
		"deployed:insert",
		"send:SupportBean:{\"doubleBoxed\":1000,\"doublePrimitive\":100,\"intBoxed\":10,\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"doubleBoxed\":2000,\"doublePrimitive\":200,\"intBoxed\":20,\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"doubleBoxed\":3000,\"doublePrimitive\":300,\"intBoxed\":30,\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean:{\"doubleBoxed\":4000,\"doublePrimitive\":400,\"intBoxed\":40,\"intPrimitive\":4,\"theString\":\"E4\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":0,\"intBoxed\":0,\"intPrimitive\":0,\"theString\":\"DB\"}",
		"send:SupportBean:{\"doubleBoxed\":3,\"doublePrimitive\":0,\"intBoxed\":0,\"intPrimitive\":0,\"theString\":\"DB\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":5,\"intBoxed\":0,\"intPrimitive\":0,\"theString\":\"DP\"}",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":4,\"intBoxed\":0,\"intPrimitive\":0,\"theString\":\"DP\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":0,\"intBoxed\":-1,\"intPrimitive\":0,\"theString\":\"IB\"}",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":0,\"intBoxed\":1,\"intPrimitive\":0,\"theString\":\"IB\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":5000,\"doublePrimitive\":500,\"intBoxed\":50,\"intPrimitive\":5,\"theString\":\"E5\"}",
		"send:SupportBean:{\"doubleBoxed\":6000,\"doublePrimitive\":600,\"intBoxed\":60,\"intPrimitive\":6,\"theString\":\"E6\"}",
		"send:SupportBean:{\"doubleBoxed\":7000,\"doublePrimitive\":700,\"intBoxed\":70,\"intPrimitive\":7,\"theString\":\"E7\"}",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":500,\"intBoxed\":0,\"intPrimitive\":5,\"theString\":\"IPDP\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":600,\"intBoxed\":0,\"intPrimitive\":6,\"theString\":\"IPDP2\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":0,\"intBoxed\":70,\"intPrimitive\":7,\"theString\":\"IPDPIB\"}",
		"send:SupportBean:{\"doubleBoxed\":null,\"doublePrimitive\":700,\"intBoxed\":70,\"intPrimitive\":7,\"theString\":\"IPDPIB\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":8000,\"doublePrimitive\":800,\"intBoxed\":80,\"intPrimitive\":8,\"theString\":\"E8\"}",
		"snapshot:createOne",
		"send:SupportBean:{\"doubleBoxed\":800,\"doublePrimitive\":0,\"intBoxed\":8,\"intPrimitive\":80,\"theString\":\"CAST\"}",
		"snapshot:createOne",
		"undeploy:d1",
		"undeploy:d2",
		"undeploy:d3",
		"undeploy:d4",
		"undeploy:d5",
		"undeploy:d6",
		"undeploy:d7",
		"deploy:d0:@name('d0') on SupportBean(theString='LAST') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive",
		"deployed:d0",
		"send:SupportBean:{\"doubleBoxed\":2000,\"doublePrimitive\":200,\"intBoxed\":20,\"intPrimitive\":2,\"theString\":\"LAST\"}",
		"snapshot:createOne",
		"undeploy:d0",
		"index-count:MyWindowCK:createOne:indexes:0",
		"undeploy-all",
		"deploy:createTwo:@name('createTwo') @public create window WinOne#keepall as SupportBean",
		"deployed:createTwo",
		"deploy:select1:on SupportBean_ST0 select * from WinOne where theString = key0",
		"deployed:select1",
		"index-count:WinOne:createTwo:indexes:1",
		"deploy:select2:on SupportBean_ST0 select * from WinOne where theString = key0 and intPrimitive = p00",
		"deployed:select2",
		"index-count:WinOne:createTwo:indexes:2",
		"undeploy-all",
	},
	"coercion-range": {
		"deploy:createOne:@name('createOne') @public create window MyWindowCR#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean",
		"deployed:createOne",
		"deploy:insert:insert into MyWindowCR select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean",
		"deployed:insert",
		"send:SupportBean:{\"doubleBoxed\":1000,\"doublePrimitive\":100,\"intBoxed\":10,\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"doubleBoxed\":2000,\"doublePrimitive\":200,\"intBoxed\":20,\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"doubleBoxed\":30,\"doublePrimitive\":3,\"intBoxed\":30,\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean:{\"doubleBoxed\":40,\"doublePrimitive\":4,\"intBoxed\":40,\"intPrimitive\":4,\"theString\":\"E4\"}",
		"send:SupportBean:{\"doubleBoxed\":5000,\"doublePrimitive\":500,\"intBoxed\":50,\"intPrimitive\":5,\"theString\":\"E5\"}",
		"send:SupportBean:{\"doubleBoxed\":6000,\"doublePrimitive\":600,\"intBoxed\":60,\"intPrimitive\":6,\"theString\":\"E6\"}",
		"deploy:d0:@name('d0') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.doublePrimitiveTwo and s2.doubleBoxedTwo",
		"deployed:d0",
		"index-count:MyWindowCR:createOne:indexes:1",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":null,\"doublePrimitiveTwo\":0,\"intBoxedTwo\":0,\"intPrimitiveTwo\":0,\"stringTwo\":\"T\"}",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":1,\"doublePrimitiveTwo\":-1,\"intBoxedTwo\":0,\"intPrimitiveTwo\":0,\"stringTwo\":\"T\"}",
		"deploy:d1:@name('d1') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo",
		"deployed:d1",
		"index-count:MyWindowCR:createOne:indexes:2",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":0,\"doublePrimitiveTwo\":0,\"intBoxedTwo\":2,\"intPrimitiveTwo\":-2,\"stringTwo\":\"T\"}",
		"deploy:d2:@name('d2') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo and win.doublePrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo",
		"deployed:d2",
		"index-count:MyWindowCR:createOne:indexes:3",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":3,\"doublePrimitiveTwo\":-3,\"intBoxedTwo\":3,\"intPrimitiveTwo\":-3,\"stringTwo\":\"T\"}",
		"deploy:d3:@name('d3') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.doublePrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo and win.intPrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo",
		"deployed:d3",
		"index-count:MyWindowCR:createOne:indexes:4",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":4,\"doublePrimitiveTwo\":-4,\"intBoxedTwo\":4,\"intPrimitiveTwo\":-4,\"stringTwo\":\"T\"}",
		"deploy:d4:@name('d4') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive <= doublePrimitiveTwo",
		"deployed:d4",
		"index-count:MyWindowCR:createOne:indexes:4",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":1,\"doublePrimitiveTwo\":5,\"intBoxedTwo\":0,\"intPrimitiveTwo\":0,\"stringTwo\":\"T\"}",
		"deploy:d5:@name('d5') on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive not between s2.intPrimitiveTwo and s2.intBoxedTwo",
		"deployed:d5",
		"index-count:MyWindowCR:createOne:indexes:4",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":0,\"doublePrimitiveTwo\":0,\"intBoxedTwo\":200,\"intPrimitiveTwo\":100,\"stringTwo\":\"T\"}",
		"undeploy:d0",
		"undeploy:d1",
		"undeploy:d2",
		"undeploy:d3",
		"undeploy:d4",
		"undeploy:d5",
		"index-count:MyWindowCR:createOne:indexes:0",
		"undeploy-all",
	},
	"coercion-key-range": {
		"deploy:createOne:@name('createOne') @public create window MyWindowCKR#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean",
		"deployed:createOne",
		"deploy:insert:insert into MyWindowCKR select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean",
		"deployed:insert",
		"send:SupportBean:{\"doubleBoxed\":1000,\"doublePrimitive\":100,\"intBoxed\":10,\"intPrimitive\":1,\"theString\":\"E1\"}",
		"send:SupportBean:{\"doubleBoxed\":2000,\"doublePrimitive\":200,\"intBoxed\":20,\"intPrimitive\":2,\"theString\":\"E2\"}",
		"send:SupportBean:{\"doubleBoxed\":3000,\"doublePrimitive\":300,\"intBoxed\":30,\"intPrimitive\":3,\"theString\":\"E3\"}",
		"send:SupportBean:{\"doubleBoxed\":4000,\"doublePrimitive\":400,\"intBoxed\":40,\"intPrimitive\":4,\"theString\":\"E4\"}",
		"deploy:d0:@name('d0') on SupportBeanTwo delete from MyWindowCKR where theString = stringTwo and intPrimitive between doublePrimitiveTwo and doubleBoxedTwo",
		"deployed:d0",
		"index-count:MyWindowCKR:createOne:indexes:1",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":200,\"doublePrimitiveTwo\":1,\"intBoxedTwo\":0,\"intPrimitiveTwo\":0,\"stringTwo\":\"T\"}",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":200,\"doublePrimitiveTwo\":1,\"intBoxedTwo\":0,\"intPrimitiveTwo\":0,\"stringTwo\":\"E1\"}",
		"deploy:d1:@name('d1') on SupportBeanTwo delete from MyWindowCKR where theString = stringTwo and intPrimitive = intPrimitiveTwo and intBoxed between doublePrimitiveTwo and doubleBoxedTwo",
		"deployed:d1",
		"index-count:MyWindowCKR:createOne:indexes:2",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":21,\"doublePrimitiveTwo\":19,\"intBoxedTwo\":0,\"intPrimitiveTwo\":2,\"stringTwo\":\"E2\"}",
		"deploy:d2:@name('d2') on SupportBeanTwo delete from MyWindowCKR where intBoxed between doubleBoxedTwo and doublePrimitiveTwo and intPrimitive = intPrimitiveTwo and theString = stringTwo ",
		"deployed:d2",
		"index-count:MyWindowCKR:createOne:indexes:3",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":34,\"doublePrimitiveTwo\":29,\"intBoxedTwo\":0,\"intPrimitiveTwo\":3,\"stringTwo\":\"E3\"}",
		"deploy:d3:@name('d3') on SupportBeanTwo delete from MyWindowCKR where intBoxed between intBoxedTwo and intBoxedTwo and intPrimitive = intPrimitiveTwo and theString = stringTwo ",
		"deployed:d3",
		"index-count:MyWindowCKR:createOne:indexes:4",
		"send:SupportBeanTwo:{\"doubleBoxedTwo\":null,\"doublePrimitiveTwo\":0,\"intBoxedTwo\":40,\"intPrimitiveTwo\":4,\"stringTwo\":\"E4\"}",
		"undeploy:d0",
		"undeploy:d1",
		"undeploy:d2",
		"undeploy:d3",
		"index-count:MyWindowCKR:createOne:indexes:0",
		"undeploy-all",
	},
}
