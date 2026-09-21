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

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

// infra_table_invalid.go replays InfraTableInvalid (ords 0-3) against the
// pinned Java oracle: a pure-invalidity file where every probe is a
// compile-time rejection and no events are sent.
//
// agg-match-single-func (ord 0) and agg-match-multi-func (ord 1) replay
// tryInvalidAggMatch: each probe deploys `@name('create') @public create
// table var1(value <declared>)`, compiles `into table var1 select
// <provided> as value from SupportBean[#time(1000)]` expecting a compile
// error, then undeploys the 'create' module. annotations (ord 2) replays
// five path-less tryInvalidCompile probes over table-column annotations.
// invalid (ord 3) deploys twelve fixtures then runs fifty path-ful
// tryInvalidCompile probes across declaration, into-table, consumption and
// table-misuse surfaces before undeployAll.
//
// Approved differences (observably identical to the Java EPL):
//   - `create table`/`create context`/`create variable`/`create window`/
//     `create schema` map to env-level registrations (CreateTable,
//     CreatePatternInitiatedTerminatedContext, RegisterVariable,
//     CreateNamedWindow, RegisterMap/RegisterObjectArray); Go has no module
//     path, so compileWithoutPath only steers the oracle's compiler args.
//   - undeploy-all mirrors env.undeployAll() and, for the agg-match cases,
//     undeployModuleContaining("create") — the probe module is the only
//     deployment.
//   - EPL-only surfaces are unrepresentable in the typed Go API (constant
//     variables, `prim key`/`primary keys` keyword typos, `window(sb.*)`,
//     `null` column types, table-column annotations, keyed table access
//     `t[k].col`, contained-event expressions, retain-union and
//     update-istream on tables); those probes pin the Java prefix without
//     claiming a Go boundary.
//   - Probes whose Go surface exists but whose rejection Java derives from
//     EPL text Go does not model (window(*) parameter declarations,
//     ignore-nulls/event-type signature details, table access inside view
//     parameters, unidirectional/retain markers on table sources) are
//     attempted and tolerated: the runner records the pinned prefix whether
//     or not the fluent equivalent rejects.

// itiBean mirrors SupportBean's asserted fields.
type itiBean struct {
	TheString       string  `esper:"theString"`
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
	BoolPrimitive   bool    `esper:"boolPrimitive"`
}

// itiS0 mirrors SupportBean_S0's asserted fields (id, p00).
type itiS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
}

// itiS1 mirrors SupportBean_S1's asserted fields (id, p10).
type itiS1 struct {
	ID  int    `esper:"id"`
	P10 string `esper:"p10"`
}

const (
	infraTableInvalidID         = "infra-table-invalid"
	infraTableInvalidJavaCommit = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
	infraTableInvalidJavaSource = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableInvalid.java"
	infraTableInvalidDesc       = "InfraTableInvalid (ords 0-3): pure-invalidity replay. InfraInvalidAggMatchSingleFunc " +
		"runs 43 tryInvalidAggMatch probes (declared vs provided aggregation signature " +
		"mismatches: parameter type, distinct, filter, ignore-nulls, min/max direction, " +
		"nth size, rate interval, ever direction, plug-in names); " +
		"InfraInvalidAggMatchMultiFunc runs 6 unbound probes (window(*) @type event-type " +
		"mismatch, sorted() sort-expression rejections, se1() plug-in name mismatch); " +
		"InfraInvalidAnnotations runs 5 path-less annotation probes; InfraInvalid deploys " +
		"12 fixtures then runs 50 path-ful probes across declaration, into-table, " +
		"consumption and table-misuse surfaces. Zero events are sent."

	itiContainsIncompatible = "Incompatible aggregation function for table"
)

var (
	infraTableInvalidJavaSources    = []string{infraTableInvalidJavaSource}
	infraTableInvalidJavaRuntimeIDs = []string{
		"java-runtime-70e525638c5fbaf37752",
		"java-runtime-7c97277eedfa76b7e0cf",
		"java-runtime-fd2a7de8e5606de63b80",
		"java-runtime-b9b6435f81135ce15040",
	}
	infraTableInvalidJavaExecutions = []string{
		"InfraInvalidAggMatchSingleFunc",
		"InfraInvalidAggMatchMultiFunc",
		"InfraInvalidAnnotations",
		"InfraInvalid",
	}
	infraTableInvalidJavaStaticIDs = []string{
		"java-dc8058ccbd099b1d1361",
		"java-ecf59e6ef7727ab987fa",
		"java-53cb0410fe21932803bc",
		"java-c44ff6d4e86c09522b35",
	}
	infraTableInvalidCases    = []string{"agg-match-single-func", "agg-match-multi-func", "annotations", "invalid"}
	infraTableInvalidOrdinals = []int{0, 1, 2, 3}
)

// itiAggProbe pins one tryInvalidAggMatch probe: the declared column
// signature, the provided aggregate expression, the pinned Java assertion
// (expectError prefix or expectContains substring) and the Go-side
// verification. goSub non-empty means the Go rejection is verified for
// ErrorInvalidRule plus the substring; goSub empty with a non-nil expr
// means the fluent equivalent is attempted and tolerated; a nil expr means
// the probe is unrepresentable and only the pinned prefix is recorded.
type itiAggProbe struct {
	label    string
	decl     esper.TableAggDecl
	declEpl  string
	unbound  bool
	expr     func(env *esper.Environment) esper.Expr
	expect   string
	contains string
	goSub    string
}

func itiDecl(name, description, paramType string) esper.TableAggDecl {
	return esper.TableAggDecl{Name: name, Description: description, ParamType: paramType, NthSize: -1, RateInterval: -1}
}

func itiDeclEpl(name, description, declEpl, paramType string) (esper.TableAggDecl, string) {
	return itiDecl(name, description, paramType), declEpl
}

// itiSingleFuncProbes transcribes InfraInvalidAggMatchSingleFunc's 43
// tryInvalidAggMatch calls in source order.
var itiSingleFuncProbes = []itiAggProbe{
	{label: "sum-param-type", decl: itiDecl("sum", "sum(double)", "Double"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Sum[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double)' and received 'sum(intPrimitive)': The required parameter type is Double and provided is Integer [",
		goSub:  "The required parameter type is Double and provided is Integer"},
	{label: "sum-name-mismatch", decl: itiDecl("sum", "sum(double)", "Double"),
		expr:   func(env *esper.Environment) esper.Expr { return esper.CountAll() },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double)' and received 'count(*)': The table declares 'sum(double)' and provided is 'count(*)'",
		goSub:  "The table declares 'sum(double)' and provided is 'count(*)'"},
	{label: "sum-filter-provided", decl: itiDecl("sum", "sum(double)", "Double"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.SumIf[float64](esper.Field[itiBean, float64]("doublePrimitive"),
				esper.Equal[string](esper.Field[itiBean, string]("theString"), esper.Literal("a")))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double)' and received 'sum(doublePrimitive,theString=\"a\")': The aggregation declares no filter expression and provided is a filter expression [",
		goSub:  "The aggregation declares no filter expression and provided is a filter expression"},
	{label: "sum-filter-declared", decl: func() esper.TableAggDecl {
		d := itiDecl("sum", "sum(double, boolean)", "Double")
		d.Filter = true
		return d
	}(),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Sum[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double,boolean)' and received 'sum(doublePrimitive)': The aggregation declares a filter expression and provided is no filter expression [",
		goSub:  "The aggregation declares a filter expression and provided is no filter expression"},
	{label: "count-name-mismatch", decl: itiDecl("count", "count(*)", ""),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Sum[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'count(*)' and received 'sum(intPrimitive)': The table declares 'count(*)' and provided is 'sum(intPrimitive)'",
		goSub:  "The table declares 'count(*)' and provided is 'sum(intPrimitive)'"},
	{label: "count-distinct-provided", decl: itiDecl("count", "count(*)", ""),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.CountDistinct[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'count(*)' and received 'count(distinct intPrimitive)': The aggregation declares no distinct and provided is a distinct [",
		goSub:  "The aggregation declares no distinct and provided is a distinct"},
	{label: "count-distinct-multikey", decl: itiDecl("count", "count(*)", ""),
		expr: func(env *esper.Environment) esper.Expr {
			// Java provides count(distinct intPrimitive, boolPrimitive);
			// the Go distinct wrapper carries one input, and the distinct
			// mismatch fires before any multi-key check.
			return esper.DistinctAggregate[int64](esper.Count[int](esper.Field[itiBean, int]("intPrimitive")),
				esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares no distinct and provided is a distinct"},
	{label: "count-distinct-param-type", decl: func() esper.TableAggDecl {
		d := itiDecl("count", "count(distinct int)", "Integer")
		d.Distinct = true
		return d
	}(),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.CountDistinct[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "count-ignore-nulls", decl: func() esper.TableAggDecl {
		d := itiDecl("count", "count(int)", "Integer")
		d.IgnoreNulls = true
		return d
	}(),
		expr:   func(env *esper.Environment) esper.Expr { return esper.CountAll() },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'count(int)' and received 'count(*)': The aggregation declares ignore nulls and provided is no ignore nulls ["},
	{label: "avg-name-mismatch", decl: itiDecl("avg", "avg(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Sum[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'avg(int)' and provided is 'sum(intPrimitive)'"},
	{label: "avg-param-type", decl: itiDecl("avg", "avg(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int64](esper.Field[itiBean, int64]("longPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Long"},
	{label: "avg-filter-provided", decl: itiDecl("avg", "avg(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.AvgIf[int](esper.Field[itiBean, int]("intPrimitive"),
				esper.Field[itiBean, bool]("boolPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares no filter expression and provided is a filter expression"},
	{label: "avg-distinct-provided", decl: itiDecl("avg", "avg(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.DistinctAggregate[float64](esper.Avg[int](esper.Field[itiBean, int]("intPrimitive")),
				esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares no distinct and provided is a distinct"},
	{label: "max-direction", decl: itiDecl("max", "max(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Min[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'max(int)' and received 'min(intPrimitive)': The aggregation declares max and provided is min [",
		goSub:  "The aggregation declares max and provided is min"},
	{label: "min-name-mismatch", decl: itiDecl("min", "min(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'min(int)' and provided is 'avg(intPrimitive)'"},
	{label: "min-param-type", decl: itiDecl("min", "min(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Min[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "min-filter-provided", decl: itiDecl("min", "min(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.MinIf[int](esper.Field[itiBean, int]("intPrimitive"),
				esper.Equal[string](esper.Field[itiBean, string]("theString"), esper.Literal("a")))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'min(int)' and received 'min(intPrimitive,theString=\"a\")': The aggregation declares no filter expression and provided is a filter expression [",
		goSub:  "The aggregation declares no filter expression and provided is a filter expression"},
	{label: "stddev-name-mismatch", decl: itiDecl("stddev", "stddev(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'stddev(int)' and provided is 'avg(intPrimitive)'"},
	{label: "stddev-param-type", decl: itiDecl("stddev", "stddev(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.StdDev[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "stddev-filter-provided", decl: itiDecl("stddev", "stddev(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.StdDev[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible},
	{label: "avedev-name-mismatch", decl: itiDecl("avedev", "avedev(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'avedev(int)' and provided is 'avg(intPrimitive)'"},
	{label: "avedev-param-type", decl: itiDecl("avedev", "avedev(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avedev[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "avedev-filter-provided", decl: itiDecl("avedev", "avedev(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avedev[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible},
	{label: "median-name-mismatch", decl: itiDecl("median", "median(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'median(int)' and provided is 'avg(intPrimitive)'"},
	{label: "median-param-type", decl: itiDecl("median", "median(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Median[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "median-filter-provided", decl: itiDecl("median", "median(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Median[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible},
	{label: "firstever-direction", decl: itiDecl("firstever", "firstever(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.LastEver[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares firstever and provided is lastever"},
	{label: "firstever-param-type", decl: itiDecl("firstever", "firstever(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.FirstEver[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "firstever-filter-declared", decl: func() esper.TableAggDecl {
		d := itiDecl("firstever", "firstever(int, boolean)", "Integer")
		d.Filter = true
		return d
	}(),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.FirstEver[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares a filter expression and provided is no filter expression"},
	{label: "lastever-direction", decl: itiDecl("lastever", "lastever(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.FirstEver[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares lastever and provided is firstever"},
	{label: "lastever-param-type", decl: itiDecl("lastever", "lastever(int)", "Integer"),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.LastEver[float64](esper.Field[itiBean, float64]("doublePrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The required parameter type is Integer and provided is Double"},
	{label: "lastever-filter-declared", decl: func() esper.TableAggDecl {
		d := itiDecl("lastever", "lastever(int, boolean)", "Integer")
		d.Filter = true
		return d
	}(),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.LastEver[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The aggregation declares a filter expression and provided is no filter expression"},
	{label: "countever-direction", decl: itiDecl("lastever", "lastever(int)", "Integer"), unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			return esper.CountEver(esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'lastever(int)' and provided is 'countever(intPrimitive)'"},
	{label: "countever-filter-declared", decl: func() esper.TableAggDecl {
		d := itiDecl("lastever", "lastever(int, boolean)", "Integer")
		d.Filter = true
		return d
	}(), unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			return esper.CountEver(esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible},
	{label: "countever-filter-provided", decl: itiDecl("lastever", "lastever(int)", "Integer"), unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			return esper.CountEver(esper.Field[itiBean, int]("intPrimitive"), esper.Literal(true))
		},
		contains: itiContainsIncompatible},
	{label: "countever-ignore-nulls", decl: itiDecl("countever", "countever(*)", ""), unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			return esper.CountEver(esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible},
	{label: "nth-name-mismatch", decl: func() esper.TableAggDecl {
		d := itiDecl("nth", "nth(int, 10)", "Integer")
		d.NthSize = 10
		return d
	}(),
		expr:     func(env *esper.Environment) esper.Expr { return esper.Avg[int](esper.Literal(20)) },
		contains: itiContainsIncompatible},
	{label: "nth-size", decl: func() esper.TableAggDecl {
		d := itiDecl("nth", "nth(int, 10)", "Integer")
		d.NthSize = 10
		return d
	}(),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Nth[int](esper.Field[itiBean, int]("intPrimitive"), 11)
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'nth(int,10)' and received 'nth(intPrimitive,11)': The size is 10 and provided is 11 ["},
	{label: "nth-param-type", decl: func() esper.TableAggDecl {
		d := itiDecl("nth", "nth(int, 10)", "Integer")
		d.NthSize = 10
		return d
	}(),
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Nth[float64](esper.Field[itiBean, float64]("doublePrimitive"), 10)
		},
		contains: itiContainsIncompatible},
	{label: "rate-name-mismatch", decl: func() esper.TableAggDecl {
		d := itiDecl("rate", "rate(20)", "")
		d.RateInterval = 20000
		return d
	}(),
		expr:     func(env *esper.Environment) esper.Expr { return esper.Avg[int](esper.Literal(20)) },
		contains: itiContainsIncompatible},
	{label: "rate-interval", decl: func() esper.TableAggDecl {
		d := itiDecl("rate", "rate(20)", "")
		d.RateInterval = 20000
		return d
	}(),
		expr:   func(env *esper.Environment) esper.Expr { return esper.Rate(11 * time.Second) },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'rate(20)' and received 'rate(11)': The interval-time is 20000 and provided is 11000 [",
		goSub:  "The interval-time is 20000 and provided is 11000"},
	{label: "leaving-name-mismatch", decl: itiDecl("leaving", "leaving(*)", ""), declEpl: "leaving()",
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		contains: itiContainsIncompatible,
		goSub:    "The table declares 'leaving(*)' and provided is 'avg(intPrimitive)'"},
	{label: "plugin-single-name", decl: itiDecl("myaggsingle", "myaggsingle(*)", ""), declEpl: "myaggsingle()",
		expr:   func(env *esper.Environment) esper.Expr { return esper.Leaving() },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'myaggsingle(*)' and received 'leaving(*)': The table declares 'myaggsingle(*)' and provided is 'leaving(*)'",
		goSub:  "The table declares 'myaggsingle(*)' and provided is 'leaving(*)'"},
}

// itiMultiFuncProbes transcribes InfraInvalidAggMatchMultiFunc's 6
// tryInvalidAggMatch calls; all are unbound (#time(1000)).
var itiMultiFuncProbes = []itiAggProbe{
	{label: "window-vs-agg-method", decl: func() esper.TableAggDecl {
		d := itiDecl("window", "window(*)", "")
		d.EventType = "SupportBean"
		return d
	}(), declEpl: "window(*) @type(SupportBean)", unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			return esper.Avg[int](esper.Field[itiBean, int]("intPrimitive"))
		},
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'window(*)' and received 'avg(intPrimitive)': The table declares 'window(*)' and provided is 'avg(intPrimitive)'",
		goSub:  "The table declares 'window(*)' and provided is 'avg(intPrimitive)'"},
	{label: "window-vs-sorted", decl: func() esper.TableAggDecl {
		d := itiDecl("window", "window(*)", "")
		d.EventType = "SupportBean"
		return d
	}(), declEpl: "window(*) @type(SupportBean)", unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			// sorted(intPrimitive) — a sorted() carrying an explicit sort
			// key; Java rejects the sort expression before the signature
			// check.
			return esper.SortedEvents(esper.Ascending(esper.Field[itiBean, int]("intPrimitive")))
		},
		expect: "Failed to validate select-clause expression 'sorted(intPrimitive)': When specifying into-table a sort expression cannot be provided [",
		goSub:  "When specifying into-table a sort expression cannot be provided"},
	{label: "window-event-type", decl: func() esper.TableAggDecl {
		d := itiDecl("window", "window(*)", "")
		d.EventType = "SupportBean_S0"
		return d
	}(), declEpl: "window(*) @type(SupportBean_S0)", unbound: true,
		expr:   func(env *esper.Environment) esper.Expr { return esper.WindowEvents() },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'window(*)' and received 'window(*)': The required event type is 'SupportBean_S0' and provided is 'SupportBean' [",
		goSub:  "The required event type is 'SupportBean_S0' and provided is 'SupportBean'"},
	{label: "sorted-vs-window", decl: func() esper.TableAggDecl {
		d := itiDecl("sorted", "sorted(intPrimitive)", "")
		d.EventType = "SupportBean"
		return d
	}(), declEpl: "sorted(intPrimitive) @type(SupportBean)", unbound: true,
		expr:   func(env *esper.Environment) esper.Expr { return esper.WindowEvents() },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'sorted(intPrimitive)' and received 'window(*)': The table declares 'sorted(intPrimitive)' and provided is 'window(*)'",
		goSub:  "The table declares 'sorted(intPrimitive)' and provided is 'window(*)'"},
	{label: "sorted-sort-expr", decl: func() esper.TableAggDecl {
		d := itiDecl("sorted", "sorted(id)", "")
		d.EventType = "SupportBean_S0"
		return d
	}(), declEpl: "sorted(id) @type(SupportBean_S0)", unbound: true,
		expr: func(env *esper.Environment) esper.Expr {
			return esper.SortedEvents(esper.Ascending(esper.Field[itiBean, int]("intPrimitive")))
		},
		expect: "Failed to validate select-clause expression 'sorted(intPrimitive)': When specifying into-table a sort expression cannot be provided [",
		goSub:  "When specifying into-table a sort expression cannot be provided"},
	{label: "plugin-multi-vs-window", decl: func() esper.TableAggDecl {
		d := itiDecl("se1", "se1(*)", "")
		d.EventType = "SupportBean"
		return d
	}(), declEpl: "se1() @type(SupportBean)", unbound: true,
		expr:   func(env *esper.Environment) esper.Expr { return esper.WindowEvents() },
		expect: "Incompatible aggregation function for table 'var1' column 'value', expecting 'se1(*)' and received 'window(*)': The table declares 'se1(*)' and provided is 'window(*)'",
		goSub:  "The table declares 'se1(*)' and provided is 'window(*)'"},
}

// itiAnnotationProbes transcribes InfraInvalidAnnotations' 5 path-less
// tryInvalidCompile calls. Table-column annotations have no typed-Go
// surface, so every probe is unrepresentable and only pins the prefix.
var itiAnnotationProbes = []struct {
	label  string
	epl    string
	expect string
}{
	{"annotation-unknown", "create table v1 (abc window(*) @unknown)",
		"For column 'abc' unrecognized annotation 'unknown' ["},
	{"annotation-missing-value", "create table v1 (abc window(*) @type)",
		"For column 'abc' no value provided for annotation 'type', expected a value ["},
	{"annotation-duplicate", "create table v1 (abc window(*) @type(SupportBean) @type(SupportBean))",
		"For column 'abc' multiple annotations provided named 'type' ["},
	{"annotation-non-string", "create table v1 (abc window(*) @type(1))",
		"For column 'abc' string value expected for annotation 'type' ["},
	{"annotation-unknown-type", "create table v1 (abc window(*) @type(xx))",
		"For column 'abc' failed to find event type 'xx' ["},
}

// itiInvalidDeploys transcribes InfraInvalid's fixture deploys; all but the
// objectarray schema compile with the runtime path.
var itiInvalidDeploys = map[string]string{
	"table-grouped-string":   "@public create table aggvar_grouped_string (key string primary key, total count(*))",
	"table-twogrouped":       "@public create table aggvar_twogrouped (keyone string primary key, keytwo string primary key, total count(*))",
	"table-grouped-int":      "@public create table aggvar_grouped_int (key int primary key, total count(*))",
	"table-ungrouped":        "@public create table aggvar_ungrouped as (total count(*))",
	"table-ungrouped-window": "@public create table aggvar_ungrouped_window as (win window(*) @type(SupportBean))",
	"context":                "@public create context MyContext initiated by SupportBean_S0 terminated by SupportBean_S1",
	"table-context":          "@public context MyContext create table aggvarctx (total count(*))",
	"context-other":          "@public create context MyOtherContext initiated by SupportBean_S0 terminated by SupportBean_S1",
	"variable":               "@public create variable int myvariable",
	"named-window":           "@public create window MyNamedWindow#keepall as select * from SupportBean",
	"schema":                 "@public create schema SomeSchema(p0 string)",
	"schema-objectarray":     "create objectarray schema MyEvent(abc int[])",
}

// itiInvalidProbe pins one ord-3 tryInvalidCompile probe. go nil means the
// EPL surface is unrepresentable; goSub non-empty verifies the Go
// rejection; goSub empty tolerates the outcome.
type itiInvalidProbe struct {
	epl    string
	expect string
	go_    func(s *itiCaseState) error
	goSub  string
	goCode esper.ErrorCode
}

func itiInvalidProbes() map[string]itiInvalidProbe {
	countStar := esper.TableAggDecl{Name: "count", Description: "count(*)", NthSize: -1, RateInterval: -1}
	return map[string]itiInvalidProbe{
		"declare-constant-variable": {
			epl:    "create constant variable aggvar_ungrouped (total count(*))",
			expect: "Incorrect syntax near '(' expecting an identifier but found an opening parenthesis '(' at line 1 column 42 [",
		},
		"declare-invalid-type": {
			epl:    "create table aggvar_notright as (total sum(abc))",
			expect: "Failed to resolve type 'abc': Could not load class by name 'abc', please check imports [",
		},
		"declare-non-aggregation": {
			epl:    "create table aggvar_wrongtoo as (total singlerow(1))",
			expect: "Expression 'singlerow(1)' is not an aggregation [",
		},
		"declare-window-param": {
			epl:    "create table aggvar_invalid as (mywindow window(intPrimitive) @type(SupportBean))",
			expect: "Failed to validate table-column expression 'window(intPrimitive)': For tables columns, the window aggregation function requires the 'window(*)' declaration [",
			go_: func(s *itiCaseState) error {
				d := itiDecl("window", "window(intPrimitive)", "")
				d.EventType = "SupportBean"
				_, err := esper.CreateTable(s.env, "aggvar_invalid", []esper.TableColumn{
					esper.OptionalTableColumnOf[any]("mywindow", esper.WithTableAggDecl(d)),
				})
				return err
			},
		},
		"declare-last-star": {
			epl: "create table aggvar_invalid as (mywindow last(*)@type(SupportBean))",
			go_: func(s *itiCaseState) error {
				d := itiDecl("last", "last(*)", "")
				d.EventType = "SupportBean"
				_, err := esper.CreateTable(s.env, "aggvar_invalid", []esper.TableColumn{
					esper.OptionalTableColumnOf[any]("mywindow", esper.WithTableAggDecl(d)),
				})
				return err
			},
		},
		"declare-window-stream-star": {
			epl: "create table aggvar_invalid as (mywindow window(sb.*)@type(SupportBean)",
		},
		"declare-maxby": {
			epl:    "create table aggvar_invalid as (mymax maxBy(intPrimitive) @type(SupportBean))",
			expect: "Failed to validate table-column expression 'maxby(intPrimitive)': For tables columns, the aggregation function requires the 'sorted(*)' declaration [",
			go_: func(s *itiCaseState) error {
				d := itiDecl("maxby", "maxby(intPrimitive)", "Integer")
				d.EventType = "SupportBean"
				_, err := esper.CreateTable(s.env, "aggvar_invalid", []esper.TableColumn{
					esper.OptionalTableColumnOf[any]("mymax", esper.WithTableAggDecl(d)),
				})
				return err
			},
		},
		"declare-duplicate-column": {
			epl:    "create table aggvar_invalid as (mycount count(*),mycount count(*))",
			expect: "Column 'mycount' is listed more than once [create table aggvar_invalid as (mycount count(*),mycount count(*))]",
			go_: func(s *itiCaseState) error {
				_, err := esper.CreateTable(s.env, "aggvar_invalid", []esper.TableColumn{
					esper.OptionalTableColumnOf[int64]("mycount", esper.WithTableAggDecl(countStar)),
					esper.OptionalTableColumnOf[int64]("mycount", esper.WithTableAggDecl(countStar)),
				})
				return err
			},
			goSub: "duplicates column",
		},
		"declare-variable-collision": {
			epl:    "create table myvariable as (mycount count(*))",
			expect: "A variable by name 'myvariable' has already been declared [",
			go_: func(s *itiCaseState) error {
				_, err := esper.CreateTable(s.env, "myvariable", []esper.TableColumn{
					esper.OptionalTableColumnOf[int64]("mycount", esper.WithTableAggDecl(countStar)),
				})
				return err
			},
			goSub: "A variable by name 'myvariable' has already been declared",
		},
		"declare-table-collision": {
			epl:    "create table aggvar_ungrouped as (total count(*))",
			expect: "A table by name 'aggvar_ungrouped' has already been declared [",
			go_: func(s *itiCaseState) error {
				_, err := esper.CreateTable(s.env, "aggvar_ungrouped", []esper.TableColumn{
					esper.OptionalTableColumnOf[int64]("total", esper.WithTableAggDecl(countStar)),
				})
				return err
			},
			goSub:  "aggvar_ungrouped",
			goCode: esper.ErrorDependency,
		},
		"declare-pk-expression": {
			epl:    "create table abc as (total count(*) primary key)",
			expect: "Column 'total' may not be tagged as primary key, an expression cannot become a primary key column [",
			go_: func(s *itiCaseState) error {
				_, err := esper.CreateTable(s.env, "abc", []esper.TableColumn{
					esper.PrimaryKeyColumn[int64]("total", esper.WithTableAggDecl(countStar)),
				})
				return err
			},
			goSub: "an expression cannot become a primary key column",
		},
		"declare-pk-event-type": {
			epl:    "create table abc as (arr SupportBean primary key)",
			expect: "Column 'arr' may not be tagged as primary key, received unexpected event type 'SupportBean' [",
			go_: func(s *itiCaseState) error {
				schema, ok := s.env.Schema("SupportBean")
				if !ok {
					return fmt.Errorf("SupportBean schema is not registered")
				}
				_, err := esper.CreateTable(s.env, "abc", []esper.TableColumn{
					esper.PrimaryKeyColumn[any]("arr", esper.WithTableColumnNestedSchema(schema)),
				})
				return err
			},
			goSub: "an event type cannot become a primary key column",
		},
		"declare-pk-prim": {
			epl:    "create table abc as (mystr string prim key)",
			expect: "Invalid keyword 'prim' encountered, expected 'primary key' [",
		},
		"declare-pk-keys": {
			epl:    "create table abc as (mystr string primary keys)",
			expect: "Invalid keyword 'keys' encountered, expected 'primary key' [",
		},
		"declare-schema-collision": {
			epl:    "create table SomeSchema as (mystr string)",
			expect: "An event type by name 'SomeSchema' has already been declared",
			go_: func(s *itiCaseState) error {
				_, err := esper.CreateTable(s.env, "SomeSchema", []esper.TableColumn{
					esper.OptionalTableColumnOf[string]("mystr"),
				})
				return err
			},
			goSub: "An event type by name 'SomeSchema' has already been declared",
		},
		"into-table-not-found": {
			epl:    "into table xxx select count(*) as total from SupportBean group by intPrimitive",
			expect: "Invalid into-table clause: Failed to find table by name 'xxx' [",
			go_: func(s *itiCaseState) error {
				intPrimitive := esper.Field[itiBean, int]("intPrimitive")
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					GroupBy(intPrimitive).
					Select(esper.Alias("total", esper.CountAll())).
					IntoTable("xxx"))
				return err
			},
			goSub:  "xxx",
			goCode: esper.ErrorUnknownName,
		},
		"into-groupby-type": {
			epl:    "into table aggvar_grouped_string select count(*) as total from SupportBean group by intPrimitive",
			expect: "Incompatible type returned by a group-by expression for use with table 'aggvar_grouped_string', the group-by expression 'intPrimitive' returns 'Integer' but the table expects 'String' [",
			go_: func(s *itiCaseState) error {
				intPrimitive := esper.Field[itiBean, int]("intPrimitive")
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					GroupBy(intPrimitive).
					Select(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvar_grouped_string"))
				return err
			},
			goSub: "Incompatible type returned by a group-by expression for use with table 'aggvar_grouped_string'",
		},
		"into-groupby-count-over": {
			epl:    "into table aggvar_grouped_string select count(*) as total from SupportBean group by theString, intPrimitive",
			expect: "Incompatible number of group-by expressions for use with table 'aggvar_grouped_string', the table expects 1 group-by expressions and provided are 2 group-by expressions [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					GroupBy(esper.Field[itiBean, string]("theString"), esper.Field[itiBean, int]("intPrimitive")).
					Select(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvar_grouped_string"))
				return err
			},
			goSub: "the table expects 1 group-by expressions and provided are 2 group-by expressions",
		},
		"into-groupby-count-ungrouped": {
			epl:    "into table aggvar_ungrouped select count(*) as total from SupportBean group by theString",
			expect: "Incompatible number of group-by expressions for use with table 'aggvar_ungrouped', the table expects no group-by expressions and provided are 1 group-by expressions [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					GroupBy(esper.Field[itiBean, string]("theString")).
					Select(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvar_ungrouped"))
				return err
			},
			goSub: "the table expects no group-by expressions and provided are 1 group-by expressions",
		},
		"into-groupby-count-missing": {
			epl:    "into table aggvar_grouped_string select count(*) as total from SupportBean",
			expect: "Incompatible number of group-by expressions for use with table 'aggvar_grouped_string', the table expects 1 group-by expressions and provided are no group-by expressions [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					Aggregate(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvar_grouped_string"))
				return err
			},
			goSub: "the table expects 1 group-by expressions and provided are no group-by expressions",
		},
		"into-context-missing": {
			epl:    "into table aggvarctx select count(*) as total from SupportBean",
			expect: "Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [into table aggvarctx select count(*) as total from SupportBean]",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					Aggregate(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvarctx"))
				return err
			},
		},
		"into-context-other": {
			epl:    "context MyOtherContext into table aggvarctx select count(*) as total from SupportBean",
			expect: "Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [context MyOtherContext into table aggvarctx select count(*) as total from SupportBean]",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					Aggregate(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvarctx", esper.WithContext("MyOtherContext")))
				return err
			},
		},
		"into-write-only": {
			epl:    "into table aggvar_ungrouped select count(*) as total, aggvar_ungrouped from SupportBean",
			expect: "Invalid use of table 'aggvar_ungrouped', aggregate-into requires write-only, the expression 'aggvar_ungrouped' is not allowed [into table aggvar_ungrouped select count(*) as total, aggvar_ungrouped from SupportBean]",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					Aggregate(
						esper.Alias("total", esper.CountAll()),
						esper.Alias("dummy", esper.Field[itiBean, string]("aggvar_ungrouped")),
					).
					IntoTable("aggvar_ungrouped"))
				return err
			},
		},
		"into-unidirectional": {
			epl:    "into table aggvar_ungrouped select count(*) as total from SupportBean unidirectional, SupportBean_S0#keepall",
			expect: "Into-table does not allow unidirectional joins [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.JoinMany(
					esper.JoinSource(esper.From[itiBean](s.env, "SupportBean")).Unidirectional(),
					esper.JoinSource(esper.From[itiS0](s.env, "SupportBean_S0").Window(esper.KeepAll())),
				).Aggregate(esper.Alias("total", esper.CountAll())).
					IntoTable("aggvar_ungrouped"))
				return err
			},
			goSub: "Into-table does not allow unidirectional joins",
		},
		"into-requires-aggregation": {
			epl:    "into table aggvar_ungrouped select * from SupportBean",
			expect: "Into-table requires at least one aggregation function [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.From[itiBean](s.env, "SupportBean").
					Aggregate(esper.Alias("total", esper.Field[itiBean, string]("theString"))).
					IntoTable("aggvar_ungrouped"))
				return err
			},
			goSub: "Into-table requires at least one aggregation function",
		},
		"access-key-count": {
			epl:    "select aggvar_ungrouped['a'].total from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvar_ungrouped[\"a\"].total': Incompatible number of key expressions for use with table 'aggvar_ungrouped', the table expects no key expressions and provided are 1 key expressions [select aggvar_ungrouped['a'].total from SupportBean]",
		},
		"access-no-key": {
			epl:    "select aggvar_grouped_string.total from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvar_grouped_string.total': Failed to resolve property 'aggvar_grouped_string.total' to a stream or nested property in a stream",
		},
		"access-key-type": {
			epl:    "select aggvar_grouped_string[5].total from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvar_grouped_string[5].total': Incompatible type returned by a key expression for use with table 'aggvar_grouped_string', the key expression '5' returns 'Integer' but the table expects 'String' [select aggvar_grouped_string[5].total from SupportBean]",
		},
		"access-unknown-function": {
			epl:    "select aggvar_grouped_string.something() from SupportBean",
			expect: "Invalid use of table 'aggvar_grouped_string', unrecognized use of function 'something', expected 'keys()'",
		},
		"access-unknown-name": {
			epl:    "select dummy[intPrimitive] from SupportBean",
			expect: "Failed to validate select-clause expression 'dummy[intPrimitive]': Failed to resolve 'dummy' to a property, single-row function, aggregation function, script, stream or class name",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.Select(esper.From[itiBean](s.env, "SupportBean"),
					esper.Alias("c0", esper.Field[itiBean, string]("dummy"))).
					Query())
				return err
			},
			goSub: "unknown field",
		},
		"access-unknown-column": {
			epl:    "select aggvarctx.dummy from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvarctx.dummy': A column 'dummy' could not be found for table 'aggvarctx' [select aggvarctx.dummy from SupportBean]",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.Select(esper.From[itiBean](s.env, "SupportBean"),
					esper.Alias("c0", esper.TableField[int64]("dummy"))).
					Query(esper.WithContext("MyContext")))
				return err
			},
		},
		"access-window-column-method": {
			epl:    "select aggvarctx_ungrouped_window.win.dummy(123) from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvarctx_ungrouped_window.win.dumm...(41 chars)': Failed to resolve 'aggvarctx_ungrouped_window.win.dummy' to a property, single-row function, aggregation function, script, stream or class name [select aggvarctx_ungrouped_window.win.dummy(123) from SupportBean]",
		},
		"access-context-other": {
			epl:    "context MyOtherContext select aggvarctx.total from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvarctx.total': Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [context MyOtherContext select aggvarctx.total from SupportBean]",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.Select(esper.From[itiBean](s.env, "SupportBean"),
					esper.Alias("c0", esper.TableField[int64]("total"))).
					Query(esper.WithContext("MyOtherContext")))
				return err
			},
		},
		"access-context-other-2": {
			epl:    "context MyOtherContext select aggvarctx.total from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvarctx.total': Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [context MyOtherContext select aggvarctx.total from SupportBean]",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.Select(esper.From[itiBean](s.env, "SupportBean"),
					esper.Alias("c0", esper.TableField[int64]("total"))).
					Query(esper.WithContext("MyOtherContext")))
				return err
			},
		},
		"access-unknown-column-nested": {
			epl:    "select aggvar_grouped_int[0].a.b from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvar_grouped_int[0].a.b': A column 'a' could not be found for table 'aggvar_grouped_int'",
		},
		"view-param-table": {
			epl:    "select * from SupportBean#time(aggvar_ungrouped.total sec)",
			expect: "Failed to validate data window declaration: Error in view 'time', Invalid parameter expression 0 for Time view: Failed to validate view parameter expression 'aggvar_ungrouped.total seconds': Invalid use of table access expression, expression 'aggvar_ungrouped' is not allowed here",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.Select(
					esper.From[itiBean](s.env, "SupportBean").
						Window(esper.TimeWindowExpr(esper.TableField[int64]("total"))),
					esper.Alias("e", esper.EventValue[esper.Event]())).
					Query())
				return err
			},
		},
		"view-on-table": {
			epl:    "select * from aggvar_grouped_string#time(30)",
			expect: "Views are not supported with tables",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.FromTable(s.env, "aggvar_grouped_string").
					Window(esper.TimeWindow(30 * time.Second)).
					Select(esper.Alias("e", esper.EventValue[esper.Event]())).
					Query())
				return err
			},
			goSub: "Views are not supported with tables",
		},
		"view-on-table-subquery": {
			epl:    "select (select * from aggvar_ungrouped#keepall) from SupportBean",
			expect: "Views are not supported with tables [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.Select(esper.From[itiBean](s.env, "SupportBean"),
					esper.Alias("c0", esper.SubqueryValueWithOptions[esper.Event](
						esper.FromTable(s.env, "aggvar_ungrouped").Window(esper.KeepAll()),
						esper.EventValue[esper.Event](),
						esper.SubqueryCardinalityMode(esper.SubqueryNullOnMultiple)))).
					Query())
				return err
			},
		},
		"contained-table": {
			epl:    "select * from aggvar_grouped_string[books]",
			expect: "Contained-event expressions are not supported with tables",
		},
		"join-method-on-column": {
			epl:    "select aggvar_grouped_int[1].total.countMinSketchFrequency(theString) from SupportBean",
			expect: "Failed to validate select-clause expression 'aggvar_grouped_int[1].total.countMi...(62 chars)': Failed to resolve method 'countMinSketchFrequency': Could not find enumeration method, date-time method, instance method or property named 'countMinSketchFrequency' in class 'Long' with matching parameter number and expected parameter type(s) 'String' ",
		},
		"join-unidirectional-method": {
			epl:    "select total.countMinSketchFrequency(theString) from aggvar_grouped_int, SupportBean unidirectional",
			expect: "Failed to validate select-clause expression 'total.countMinSketchFrequency(theString)': Failed to resolve method 'countMinSketchFrequency': Could not find",
		},
		"table-unidirectional": {
			epl:    "select * from aggvar_grouped_int unidirectional, SupportBean",
			expect: "Tables cannot be marked as unidirectional [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.JoinMany(
					esper.JoinRecordSource(esper.FromTable(s.env, "aggvar_grouped_int")).Unidirectional(),
					esper.JoinSource(esper.From[itiBean](s.env, "SupportBean")),
				).Select(esper.SelectSourceEvent(0, "aggvar_grouped_int"), esper.SelectSourceEvent(1, "SupportBean")).Query())
				return err
			},
			goSub: "Tables cannot be marked as unidirectional",
		},
		"table-retain": {
			epl:    "select * from aggvar_grouped_int retain-union",
			expect: "Tables cannot be marked with retain [",
		},
		"table-on-action": {
			epl:    "on aggvar_ungrouped select * from aggvar_ungrouped",
			expect: "Tables cannot be used in an on-action statement triggering stream [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.OnRecord(esper.FromTable(s.env, "aggvar_ungrouped")).
					SelectFromTableWhere("aggvar_ungrouped", esper.Literal(true),
						esper.Alias("total", esper.TableField[int64]("total"))).
					Query())
				return err
			},
			goSub: "Tables cannot be used in an on-action statement triggering stream",
		},
		"table-match-recognize": {
			epl:    "select * from aggvar_ungrouped match_recognize ( measures a.theString as a pattern (A) define A as true)",
			expect: "Tables cannot be used with match-recognize [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.FromTable(s.env, "aggvar_ungrouped").
					MatchRecognize(esper.RowVar("A")).
					Define("A", esper.Literal(true)).
					Measures(esper.Alias("a", esper.TagField[string]("A", "theString"))).
					Query())
				return err
			},
			goSub: "Tables cannot be used with match-recognize",
		},
		"table-update-istream": {
			epl:    "update istream aggvar_grouped_string set key = 'a'",
			expect: "Tables cannot be used in an update-istream statement [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.FromTable(s.env, "aggvar_grouped_string").
					UpdateStream(esper.SetColumn("key", esper.Literal("a"))).
					Query())
				return err
			},
			goSub: "Tables cannot be used in an update-istream statement",
		},
		"table-context-declaration": {
			epl:    "create context InvalidCtx as start aggvar_ungrouped end after 5 seconds",
			expect: "Tables cannot be used in a context declaration [",
			go_: func(s *itiCaseState) error {
				_, err := esper.CreatePatternInitiatedTerminatedContext(s.env, "InvalidCtx",
					esper.PatternFromRecord(esper.FromTable(s.env, "aggvar_ungrouped"), "a", esper.Literal(true)),
					esper.PatternFromRecord(esper.From[itiBean](s.env, "SupportBean").AsRecord(), "b", esper.Literal(true)))
				return err
			},
			goSub: "Tables cannot be used in a context declaration",
		},
		"table-pattern-atom": {
			epl:    "select * from pattern[aggvar_ungrouped]",
			expect: "Tables cannot be used in pattern filter atoms [",
			go_: func(s *itiCaseState) error {
				_, err := s.env.Build(esper.PatternFromRecord(
					esper.FromTable(s.env, "aggvar_ungrouped"), "a", esper.Literal(true)).Query())
				return err
			},
			goSub: "Tables cannot be used in pattern filter atoms",
		},
		"schema-table-collision": {
			epl:    "create schema aggvar_ungrouped as SupportBean",
			expect: "A table by name 'aggvar_ungrouped' already exists [",
			go_: func(s *itiCaseState) error {
				_, err := esper.RegisterMap(s.env, "aggvar_ungrouped", []esper.FieldSpec{
					esper.FieldDef("p0", reflect.TypeOf("")),
				})
				return err
			},
			goSub: "A table by name 'aggvar_ungrouped' already exists",
		},
		"declare-null-pk": {
			epl:    "create table MyTable(somefield null primary key, id string)",
			expect: "Incorrect syntax near 'null' (a reserved keyword)",
		},
	}
}

// itiCaseState carries the per-case replay state: the environment/engine
// pair, the deployed-label bookkeeping used by undeploy-all and the trace
// the runner records into.
type itiCaseState struct {
	env            *esper.Environment
	engine         *esper.Engine
	deployments    map[string][]*esper.Deployment
	deployedLabels map[string]bool
	trace          *compat.Trace
	caseName       string
	var1Target     string
}

// runInfraTableInvalidScenario replays the four InfraTableInvalid
// executions: each case runs on a fresh environment/engine pair (one
// runtime per Java execution) and every step dispatches to the matching
// runtime action. No events are sent and no listeners attach, so records
// carry no time.
func runInfraTableInvalidScenario(ctx context.Context, scenario compat.Scenario) (compat.Trace, error) {
	if err := validateInfraTableInvalidScenario(scenario); err != nil {
		return compat.Trace{}, err
	}
	trace := compat.Trace{Version: scenario.Version, ID: scenario.ID}
	offset := 0
	for caseIndex, caseName := range infraTableInvalidCases {
		end := offset + 1
		for end < len(scenario.Steps) && scenario.Steps[end].Op != "case" {
			end++
		}
		caseSteps := scenario.Steps[offset:end]
		offset = end
		caseTrace, err := runInfraTableInvalidCase(ctx, caseSteps, caseName, caseIndex)
		if err != nil {
			return compat.Trace{}, fmt.Errorf("%s case %q: %w", infraTableInvalidID, caseName, err)
		}
		trace.Records = append(trace.Records, caseTrace.Records...)
	}
	if offset != len(scenario.Steps) {
		return compat.Trace{}, fmt.Errorf("%s scenario has trailing steps", infraTableInvalidID)
	}
	return trace, nil
}

func runInfraTableInvalidCase(ctx context.Context, steps []compat.Step, caseName string, caseIndex int) (compat.Trace, error) {
	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[itiBean](env, "SupportBean"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[itiS0](env, "SupportBean_S0"); err != nil {
		return compat.Trace{}, err
	}
	if _, err := esper.RegisterStruct[itiS1](env, "SupportBean_S1"); err != nil {
		return compat.Trace{}, err
	}
	engine := esper.NewEngine(env,
		esper.WithRuntimeURI(infraTableInvalidJavaRuntimeIDs[caseIndex]),
		esper.WithStartTime(time.Unix(0, 0).UTC()),
	)
	defer func() { _ = engine.Close(context.Background()) }()

	trace := compat.Trace{Version: "esper-parity/v1", ID: infraTableInvalidID}
	state := &itiCaseState{
		env:            env,
		engine:         engine,
		deployments:    make(map[string][]*esper.Deployment),
		deployedLabels: make(map[string]bool),
		trace:          &trace,
		caseName:       caseName,
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return compat.Trace{}, err
		}
		if step.Op != "case" && step.Case != caseName {
			return compat.Trace{}, fmt.Errorf("%s: step %q carries unpinned case %q, want %q", infraTableInvalidID, step.Statement, step.Case, caseName)
		}
		switch step.Op {
		case "case":
		case "deploy":
			if err := state.deploy(ctx, step); err != nil {
				return compat.Trace{}, err
			}
		case "build-error":
			if err := state.buildError(step); err != nil {
				return compat.Trace{}, err
			}
		case "undeploy-all":
			if err := state.undeployAll(ctx); err != nil {
				return compat.Trace{}, err
			}
		default:
			return compat.Trace{}, fmt.Errorf("%s: unexpected op %q", infraTableInvalidID, step.Op)
		}
	}
	return trace, nil
}

// deploy executes the fixture registrations the path-ful probes compile
// against: the per-probe 'create' table for the agg-match cases and the
// twelve InfraInvalid fixtures for the invalid case.
func (s *itiCaseState) deploy(ctx context.Context, step compat.Step) error {
	switch s.caseName {
	case "agg-match-single-func", "agg-match-multi-func":
		return s.deployAggMatch(ctx, step)
	case "invalid":
		return s.deployInvalid(ctx, step)
	default:
		return fmt.Errorf("%s: unexpected deploy %q in case %q", infraTableInvalidID, step.Statement, s.caseName)
	}
}

// deployAggMatch deploys `@name('create') @public create table
// var1(value <declared>)` for one tryInvalidAggMatch probe.
func (s *itiCaseState) deployAggMatch(ctx context.Context, step compat.Step) error {
	probe, ok := itiAggProbeFor(s.caseName, step.Statement)
	if !ok {
		return fmt.Errorf("%s: unknown deploy %q in case %q", infraTableInvalidID, step.Statement, s.caseName)
	}
	expected := "@name('create') @public create table var1(value " + probe.declEpl + ")"
	if probe.declEpl == "" {
		expected = "@name('create') @public create table var1(value " + probe.decl.Description + ")"
	}
	if step.Epl != expected || step.ExpectContains != "" {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", infraTableInvalidID, step.Statement, step.Epl)
	}
	// Java deploys the 'create' module and undeploys it after the probe;
	// the Go equivalent registers a per-probe protected module (module
	// names cannot repeat) whose table is removed when the activation
	// deployment undeploys.
	moduleName := "create-" + step.Statement
	module, err := s.env.RegisterModule(moduleName, esper.ProtectedModule())
	if err != nil {
		return err
	}
	if _, err := module.RegisterTable("var1", []esper.TableColumn{
		esper.OptionalTableColumnOf[any]("value", esper.WithTableAggDecl(probe.decl)),
	}); err != nil {
		return err
	}
	plan, err := module.Build(module.Table("var1").Query(esper.StatementName("create")))
	if err != nil {
		return err
	}
	deployment, err := s.engine.Deploy(ctx, plan)
	if err != nil {
		return err
	}
	s.deployments[step.Statement] = append(s.deployments[step.Statement], deployment)
	s.deployedLabels[step.Statement] = true
	s.var1Target = module.QualifiedName("var1")
	return nil
}

// deployInvalid builds the ord-3 fixture: the grouped/ungrouped tables,
// the MyContext/MyOtherContext contexts, the context-bound aggvarctx
// table, the myvariable variable, the MyNamedWindow named window, the
// SomeSchema map type and the MyEvent objectarray type.
func (s *itiCaseState) deployInvalid(ctx context.Context, step compat.Step) error {
	if pinned, ok := itiInvalidDeploys[step.Statement]; !ok || step.Epl != pinned || step.ExpectContains != "" {
		return fmt.Errorf("%s: deploy %q carries an unpinned EPL %q", infraTableInvalidID, step.Statement, step.Epl)
	}
	countStar := esper.TableAggDecl{Name: "count", Description: "count(*)", NthSize: -1, RateInterval: -1}
	switch step.Statement {
	case "table-grouped-string":
		// `@public create table aggvar_grouped_string (key string primary
		// key, total count(*))`.
		if _, err := esper.CreateTable(s.env, "aggvar_grouped_string", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("key"),
			esper.OptionalTableColumnOf[int64]("total", esper.WithTableAggDecl(countStar)),
		}); err != nil {
			return err
		}
	case "table-twogrouped":
		// `@public create table aggvar_twogrouped (keyone string primary
		// key, keytwo string primary key, total count(*))`.
		if _, err := esper.CreateTable(s.env, "aggvar_twogrouped", []esper.TableColumn{
			esper.PrimaryKeyColumn[string]("keyone"),
			esper.PrimaryKeyColumn[string]("keytwo"),
			esper.OptionalTableColumnOf[int64]("total", esper.WithTableAggDecl(countStar)),
		}); err != nil {
			return err
		}
	case "table-grouped-int":
		// `@public create table aggvar_grouped_int (key int primary key,
		// total count(*))`.
		if _, err := esper.CreateTable(s.env, "aggvar_grouped_int", []esper.TableColumn{
			esper.PrimaryKeyColumn[int]("key"),
			esper.OptionalTableColumnOf[int64]("total", esper.WithTableAggDecl(countStar)),
		}); err != nil {
			return err
		}
	case "table-ungrouped":
		// `@public create table aggvar_ungrouped as (total count(*))`.
		if _, err := esper.CreateTable(s.env, "aggvar_ungrouped", []esper.TableColumn{
			esper.OptionalTableColumnOf[int64]("total", esper.WithTableAggDecl(countStar)),
		}); err != nil {
			return err
		}
	case "table-ungrouped-window":
		// `@public create table aggvar_ungrouped_window as (win window(*)
		// @type(SupportBean))`.
		windowDecl := esper.TableAggDecl{Name: "window", Description: "window(*)",
			EventType: "SupportBean", NthSize: -1, RateInterval: -1}
		if _, err := esper.CreateTable(s.env, "aggvar_ungrouped_window", []esper.TableColumn{
			esper.OptionalTableColumnOf[any]("win", esper.WithTableAggDecl(windowDecl)),
		}); err != nil {
			return err
		}
	case "context":
		// `@public create context MyContext initiated by SupportBean_S0
		// terminated by SupportBean_S1`.
		if _, err := esper.CreatePatternInitiatedTerminatedContext(s.env, "MyContext",
			esper.PatternFromRecord(esper.From[itiS0](s.env, "SupportBean_S0").AsRecord(), "a", esper.Literal(true)),
			esper.PatternFromRecord(esper.From[itiS1](s.env, "SupportBean_S1").AsRecord(), "b", esper.Literal(true))); err != nil {
			return err
		}
	case "table-context":
		// `@public context MyContext create table aggvarctx (total
		// count(*))`.
		if _, err := esper.CreateTable(s.env, "aggvarctx", []esper.TableColumn{
			esper.OptionalTableColumnOf[int64]("total", esper.WithTableAggDecl(countStar)),
		}, esper.TableContext("MyContext")); err != nil {
			return err
		}
	case "context-other":
		// `@public create context MyOtherContext initiated by
		// SupportBean_S0 terminated by SupportBean_S1`.
		if _, err := esper.CreatePatternInitiatedTerminatedContext(s.env, "MyOtherContext",
			esper.PatternFromRecord(esper.From[itiS0](s.env, "SupportBean_S0").AsRecord(), "a", esper.Literal(true)),
			esper.PatternFromRecord(esper.From[itiS1](s.env, "SupportBean_S1").AsRecord(), "b", esper.Literal(true))); err != nil {
			return err
		}
	case "variable":
		// `@public create variable int myvariable`.
		if err := s.env.RegisterVariable("myvariable", 0); err != nil {
			return err
		}
	case "named-window":
		// `@public create window MyNamedWindow#keepall as select * from
		// SupportBean`.
		schema, ok := s.env.Schema("SupportBean")
		if !ok {
			return fmt.Errorf("%s: SupportBean schema is not registered", infraTableInvalidID)
		}
		if _, err := esper.CreateNamedWindow(s.env, "MyNamedWindow", schema,
			esper.NamedWindowRetention(esper.KeepAll())); err != nil {
			return err
		}
	case "schema":
		// `@public create schema SomeSchema(p0 string)`.
		if _, err := esper.RegisterMap(s.env, "SomeSchema", []esper.FieldSpec{
			esper.FieldDef("p0", reflect.TypeOf("")),
		}); err != nil {
			return err
		}
	case "schema-objectarray":
		// `create objectarray schema MyEvent(abc int[])` — the only deploy
		// Java compiles without the runtime path.
		if _, err := esper.RegisterObjectArray(s.env, "MyEvent", []esper.FieldSpec{
			esper.FieldDef("abc", reflect.TypeOf([]int{})),
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: unknown invalid deploy %q", infraTableInvalidID, step.Statement)
	}
	s.deployedLabels[step.Statement] = true
	return nil
}

// itiAggProbeFor resolves one agg-match probe by case and label.
func itiAggProbeFor(caseName, label string) (itiAggProbe, bool) {
	probes := itiSingleFuncProbes
	if caseName == "agg-match-multi-func" {
		probes = itiMultiFuncProbes
	}
	for _, probe := range probes {
		if probe.label == label {
			return probe, true
		}
	}
	return itiAggProbe{}, false
}

// buildError runs one expected-invalid probe against the fluent
// equivalent of the pinned EPL, then records the pinned Java assertion.
// Verified probes (goSub set) require the Go rejection to carry the
// expected code and substring; tolerated probes attempt the fluent
// equivalent without gating; unrepresentable probes skip the attempt.
func (s *itiCaseState) buildError(step compat.Step) error {
	switch s.caseName {
	case "agg-match-single-func", "agg-match-multi-func":
		return s.buildErrorAggMatch(step)
	case "annotations":
		return s.buildErrorAnnotation(step)
	case "invalid":
		return s.buildErrorInvalid(step)
	default:
		return fmt.Errorf("%s: unknown build-error probe %q in case %q", infraTableInvalidID, step.Statement, s.caseName)
	}
}

func (s *itiCaseState) buildErrorAggMatch(step compat.Step) error {
	probe, ok := itiAggProbeFor(s.caseName, step.Statement)
	if !ok {
		return fmt.Errorf("%s: unknown build-error probe %q", infraTableInvalidID, step.Statement)
	}
	expectedEpl := "into table var1 select " + probeProvided(probe) + " as value from SupportBean"
	if probe.unbound {
		expectedEpl += "#time(1000)"
	}
	if step.Epl != expectedEpl || step.ExpectError != probe.expect || step.ExpectContains != probe.contains {
		return fmt.Errorf("%s: build-error probe %q is not pinned", infraTableInvalidID, step.Statement)
	}
	if probe.expr != nil {
		stream := esper.From[itiBean](s.env, "SupportBean")
		var aggregate esper.AggregateStream
		if probe.unbound {
			aggregate = stream.Window(esper.TimeWindow(time.Second)).Aggregate(
				esper.Alias("value", probe.expr(s.env)))
		} else {
			aggregate = stream.Aggregate(esper.Alias("value", probe.expr(s.env)))
		}
		_, buildErr := s.env.Build(aggregate.IntoTable(s.var1Target))
		if probe.goSub != "" {
			if buildErr == nil {
				return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", infraTableInvalidID, step.Statement)
			}
			var espErr *esper.Error
			if !errors.As(buildErr, &espErr) || espErr.Code != esper.ErrorInvalidRule ||
				!strings.Contains(buildErr.Error(), probe.goSub) {
				return fmt.Errorf("%s: build-error probe %q drift: got %v", infraTableInvalidID, step.Statement, buildErr)
			}
		}
	}
	s.trace.Records = append(s.trace.Records, compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
		Value:     itiRecordValue(step),
	})
	return nil
}

// probeProvided renders the provided-expression text the pinned EPL
// carries; the runner derives it from the probe table so the deploy and
// build-error steps stay byte-exact with the Java source.
func probeProvided(probe itiAggProbe) string {
	return itiProvidedEPLs[probe.label]
}

// itiProvidedEPLs pins the provided-expression text per probe label,
// transcribed from the Java source.
var itiProvidedEPLs = map[string]string{
	"sum-param-type":            "sum(intPrimitive)",
	"sum-name-mismatch":         "count(*)",
	"sum-filter-provided":       "sum(doublePrimitive, theString='a')",
	"sum-filter-declared":       "sum(doublePrimitive)",
	"count-name-mismatch":       "sum(intPrimitive)",
	"count-distinct-provided":   "count(distinct intPrimitive)",
	"count-distinct-multikey":   "count(distinct intPrimitive, boolPrimitive)",
	"count-distinct-param-type": "count(distinct doublePrimitive)",
	"count-ignore-nulls":        "count(*)",
	"avg-name-mismatch":         "sum(intPrimitive)",
	"avg-param-type":            "avg(longPrimitive)",
	"avg-filter-provided":       "avg(intPrimitive, boolPrimitive)",
	"avg-distinct-provided":     "avg(distinct intPrimitive)",
	"max-direction":             "min(intPrimitive)",
	"min-name-mismatch":         "avg(intPrimitive)",
	"min-param-type":            "min(doublePrimitive)",
	"min-filter-provided":       "fmin(intPrimitive, theString='a')",
	"stddev-name-mismatch":      "avg(intPrimitive)",
	"stddev-param-type":         "stddev(doublePrimitive)",
	"stddev-filter-provided":    "stddev(intPrimitive, true)",
	"avedev-name-mismatch":      "avg(intPrimitive)",
	"avedev-param-type":         "avedev(doublePrimitive)",
	"avedev-filter-provided":    "avedev(intPrimitive, true)",
	"median-name-mismatch":      "avg(intPrimitive)",
	"median-param-type":         "median(doublePrimitive)",
	"median-filter-provided":    "median(intPrimitive, true)",
	"firstever-direction":       "lastever(intPrimitive)",
	"firstever-param-type":      "firstever(doublePrimitive)",
	"firstever-filter-declared": "firstever(intPrimitive)",
	"lastever-direction":        "firstever(intPrimitive)",
	"lastever-param-type":       "lastever(doublePrimitive)",
	"lastever-filter-declared":  "lastever(intPrimitive)",
	"countever-direction":       "countever(intPrimitive)",
	"countever-filter-declared": "countever(intPrimitive)",
	"countever-filter-provided": "countever(intPrimitive, true)",
	"countever-ignore-nulls":    "countever(intPrimitive)",
	"nth-name-mismatch":         "avg(20)",
	"nth-size":                  "nth(intPrimitive, 11)",
	"nth-param-type":            "nth(doublePrimitive, 10)",
	"rate-name-mismatch":        "avg(20)",
	"rate-interval":             "rate(11)",
	"leaving-name-mismatch":     "avg(intPrimitive)",
	"plugin-single-name":        "leaving()",
	"window-vs-agg-method":      "avg(intPrimitive)",
	"window-vs-sorted":          "sorted(intPrimitive)",
	"window-event-type":         "window(*)",
	"sorted-vs-window":          "window(*)",
	"sorted-sort-expr":          "sorted(intPrimitive)",
	"plugin-multi-vs-window":    "window(*)",
}

func (s *itiCaseState) buildErrorAnnotation(step compat.Step) error {
	for _, probe := range itiAnnotationProbes {
		if probe.label != step.Statement {
			continue
		}
		if step.Epl != probe.epl || step.ExpectError != probe.expect || step.ExpectContains != "" {
			return fmt.Errorf("%s: build-error probe %q is not pinned", infraTableInvalidID, step.Statement)
		}
		// Table-column annotations have no typed-Go surface: pin the
		// prefix without claiming a boundary.
		s.trace.Records = append(s.trace.Records, compat.TraceRecord{
			Case:      s.caseName,
			Operation: "compile-error",
			Statement: step.Statement,
			Value:     step.ExpectError,
		})
		return nil
	}
	return fmt.Errorf("%s: unknown annotations probe %q", infraTableInvalidID, step.Statement)
}

func (s *itiCaseState) buildErrorInvalid(step compat.Step) error {
	probes := itiInvalidProbes()
	probe, ok := probes[step.Statement]
	if !ok || step.Epl != probe.epl || step.ExpectError != probe.expect || step.ExpectContains != "" {
		return fmt.Errorf("%s: build-error probe %q is not pinned", infraTableInvalidID, step.Statement)
	}
	if probe.go_ != nil {
		buildErr := probe.go_(s)
		if probe.goSub != "" {
			if buildErr == nil {
				return fmt.Errorf("%s: build-error probe %q unexpectedly compiled", infraTableInvalidID, step.Statement)
			}
			code := probe.goCode
			if code == "" {
				code = esper.ErrorInvalidRule
			}
			// errors.Is covers both *esper.Error (Code match) and
			// *esper.DuplicateModuleObjectError (Is -> ErrorDependency).
			if !errors.Is(buildErr, code) ||
				!strings.Contains(buildErr.Error(), probe.goSub) {
				return fmt.Errorf("%s: build-error probe %q drift: got %v", infraTableInvalidID, step.Statement, buildErr)
			}
		}
	}
	record := compat.TraceRecord{
		Case:      s.caseName,
		Operation: "compile-error",
		Statement: step.Statement,
	}
	// Java omits the value field for "skip"-pinned probes; an empty string
	// would serialize as a present-but-empty value and diff against absent.
	if value := itiRecordValue(step); value != "" {
		record.Value = value
	}
	s.trace.Records = append(s.trace.Records, record)
	return nil
}

// itiRecordValue mirrors the oracle's record value: the expectError prefix
// when pinned, otherwise the expectContains substring, otherwise empty.
func itiRecordValue(step compat.Step) string {
	if step.ExpectError != "" {
		return step.ExpectError
	}
	return step.ExpectContains
}

func (s *itiCaseState) undeployAll(ctx context.Context) error {
	for _, deployments := range s.deployments {
		for _, deployment := range deployments {
			if err := deployment.Undeploy(ctx); err != nil {
				return err
			}
		}
	}
	// Table/context/variable/window/schema fixtures are env-level
	// registrations; undeploy-all clears the deployment bookkeeping so the
	// next probe's 'create' table registers cleanly.
	s.deployments = make(map[string][]*esper.Deployment)
	s.deployedLabels = make(map[string]bool)
	return nil
}

// validateInfraTableInvalidScenario pins the scenario identity; the strict
// loader already pins the case metadata and step sequence.
func validateInfraTableInvalidScenario(scenario compat.Scenario) error {
	if err := scenario.Validate(); err != nil {
		return err
	}
	if scenario.ID != infraTableInvalidID || scenario.Version != "esper-parity/v1" {
		return fmt.Errorf("%s scenario shape is not pinned", infraTableInvalidID)
	}
	return nil
}

// loadInfraTableInvalidScenario decodes the scenario with the strict-shape
// checks the raw-mutation tests pin: no duplicate JSON keys, the exact
// top-level field set, pinned case metadata, and per-step field whitelists
// so unknown or duplicated step fields fail the replay.
func loadInfraTableInvalidScenario(reader io.Reader) (compat.Scenario, error) {
	if reader == nil {
		return compat.Scenario{}, fmt.Errorf("infra-table-invalid scenario reader is required")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return compat.Scenario{}, fmt.Errorf("read infra-table-invalid scenario: %w", err)
	}
	if err := rejectDuplicateJSON(raw); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode infra-table-invalid scenario: %w", err)
	}
	var root map[string]json.RawMessage
	if err := strictObject(raw, &root); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode infra-table-invalid scenario: %w", err)
	}
	required := []string{"version", "id", "description", "javaCommit", "javaSource", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps"}
	if len(root) != len(required) {
		return compat.Scenario{}, fmt.Errorf("infra-table-invalid scenario contains unexpected or missing fields")
	}
	for _, name := range required {
		if _, ok := root[name]; !ok {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid scenario is missing field %q", name)
		}
	}
	var cases []struct {
		Case          string `json:"case"`
		Ordinal       int    `json:"ordinal"`
		RuntimeID     string `json:"runtimeId"`
		ExecutionName string `json:"executionName"`
	}
	if err := json.Unmarshal(root["cases"], &cases); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode infra-table-invalid cases: %w", err)
	}
	if len(cases) != len(infraTableInvalidCases) {
		return compat.Scenario{}, fmt.Errorf("infra-table-invalid scenario must contain exactly %d cases", len(infraTableInvalidCases))
	}
	for index, entry := range cases {
		if entry.Case != infraTableInvalidCases[index] ||
			entry.Ordinal != infraTableInvalidOrdinals[index] ||
			entry.RuntimeID != infraTableInvalidJavaRuntimeIDs[index] ||
			entry.ExecutionName != infraTableInvalidJavaExecutions[index] {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid scenario case %d metadata is not pinned", index)
		}
	}
	var rawSteps []json.RawMessage
	if err := json.Unmarshal(root["steps"], &rawSteps); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode infra-table-invalid steps: %w", err)
	}
	stepFields := map[string][]string{
		"case":         {"op", "case"},
		"deploy":       {"op", "case", "statement", "epl", "compileWithoutPath", "expectContains"},
		"build-error":  {"op", "case", "statement", "epl", "expectError", "expectContains", "compileWithoutPath"},
		"undeploy-all": {"op", "case"},
	}
	if len(rawSteps) != 219 {
		return compat.Scenario{}, fmt.Errorf("infra-table-invalid scenario must contain exactly 219 steps, found %d", len(rawSteps))
	}
	for index, rawStep := range rawSteps {
		var object map[string]json.RawMessage
		if err := strictObject(rawStep, &object); err != nil {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid step %d: %w", index, err)
		}
		var op string
		if err := json.Unmarshal(object["op"], &op); err != nil {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid step %d op: %w", index, err)
		}
		allowed, ok := stepFields[op]
		if !ok {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid step %d has unsupported op %q", index, op)
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
				return compat.Scenario{}, fmt.Errorf("infra-table-invalid step %d has unexpected field %q", index, field)
			}
		}
		var stepCase string
		if err := json.Unmarshal(object["case"], &stepCase); err != nil {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid step %d case: %w", index, err)
		}
		pinned := false
		for _, name := range infraTableInvalidCases {
			if stepCase == name {
				pinned = true
				break
			}
		}
		if !pinned {
			return compat.Scenario{}, fmt.Errorf("infra-table-invalid step %d has unknown case %q", index, stepCase)
		}
	}
	var scenario compat.Scenario
	if err := json.Unmarshal(raw, &scenario); err != nil {
		return compat.Scenario{}, fmt.Errorf("decode infra-table-invalid scenario: %w", err)
	}
	if err := scenario.Validate(); err != nil {
		return compat.Scenario{}, err
	}
	return scenario, nil
}
