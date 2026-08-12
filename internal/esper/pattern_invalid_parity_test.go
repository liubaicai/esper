package esper

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Parity coverage for regression-lib/suite/pattern/PatternInvalid.java
// (PatternInvalidExpr, PatternStatementException, PatternUseResult).
//
// Go-style differences that are unrepresentable in the typed fluent API
// (they are Go compile-time errors rather than Build rejections):
//   - EPL text-syntax errors (PatternInvalidExpr first case);
//   - timer:within used as an observer root and timer:interval used as a
//     guard (Within is a guard combinator, TimerInterval an observer root);
//   - empty parameter lists (TimerInterval/Within require a duration
//     argument) and non-duration guard parameters such as within("s");
//   - timer:at named parameters and the single-parameter form (CronSchedule
//     carries exactly the five positional fields plus opt-in sub-fields,
//     and the timezone is a string-typed option, so a numeric timezone
//     parameter is unrepresentable).

type patternInvalidNestedProps struct {
	Value string `esper:"value"`
}

type patternInvalidComplexProps struct {
	Nested patternInvalidNestedProps `esper:"nested"`
}

type patternInvalidWaitRow struct {
	WaitTime int64 `esper:"waitTime"`
}

func newPatternInvalidEnv(t *testing.T) *Environment {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[patternOpBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternUseResultN](env, "SupportBean_N"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternInvalidComplexProps](env, "SupportBeanComplexProps"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[patternInvalidWaitRow](env, "PatternInvalidWaitRow"); err != nil {
		t.Fatal(err)
	}
	return env
}

func assertPatternInvalidBuild(t *testing.T, env *Environment, name string, query Query, fragments ...string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		_, err := env.Build(query)
		if err == nil {
			t.Fatalf("invalid pattern %s was accepted", name)
		}
		if !errors.Is(err, ErrorInvalidRule) && !errors.Is(err, ErrorTypeMismatch) && !errors.Is(err, ErrorUnknownName) {
			t.Fatalf("invalid pattern %s error = %v, want InvalidRule/TypeMismatch/UnknownName", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(err.Error(), fragment) {
				t.Fatalf("invalid pattern %s error = %v, want fragment %q", name, err, fragment)
			}
		}
	})
}

// TestPatternInvalidExprMatchesEsper mirrors PatternInvalid.PatternInvalidExpr:
// a negative left branch of followed-by compiles and deploys, and a
// subselect inside a timer observer parameter is rejected at Build.
func TestPatternInvalidExprMatchesEsper(t *testing.T) {
	env := newPatternInvalidEnv(t)
	sb := From[patternOpBean](env, "SupportBean")
	theString := Field[patternOpBean, string]("theString")

	// Java: select * from pattern[(not a=SupportBean) -> SupportBean(theString=a.theString)]
	// compiles and deploys; the negative first child of a followed-by is legal.
	plan, err := env.Build(PatternFrom(sb, "a", Literal(true)).Not().Then(
		PatternFrom(sb, "b", Equal[string](theString, TagField[string]("a", "theString"))),
	).Select(Alias("c", Literal(1))).Query(StatementName("pattern-invalid-not-first")))
	if err != nil {
		t.Fatalf("not-first followed-by pattern was rejected: %v", err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatalf("not-first followed-by pattern failed to deploy: %v", err)
	}
	if err := deployment.Undeploy(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Java: pattern[timer:interval((select waitTime from WaitWindow))] is
	// rejected because subselects are not allowed within pattern observer
	// parameters. The fluent form wraps the subquery in a duration cast so
	// the expression is type-correct and only the observer placement is
	// invalid, matching the Java boundary exactly.
	waitSchema, ok := env.Schema("PatternInvalidWaitRow")
	if !ok {
		t.Fatal("wait row schema is missing")
	}
	if _, err := CreateNamedWindow(env, "PatternInvalidWaitWindow", waitSchema, NamedWindowRetention(KeepAll())); err != nil {
		t.Fatal(err)
	}
	interval := Cast[int64, time.Duration](SubqueryValue[int64](
		FromNamedWindow(env, "PatternInvalidWaitWindow"), Field[any, int64]("waitTime")))
	assertPatternInvalidBuild(t, env, "subselect-observer-parameter",
		TimerIntervalExpr(sb, interval).Select(Alias("c", Literal(1))).Query(),
		"subselects not allowed within pattern observer parameters")

	// The same restriction applies to cron observer fields: a subselect
	// inside a timer:at parameter expression is rejected at Build.
	cron := NewCronSchedule(
		CronValuesExpr(Cast[int64, int](SubqueryValue[int64](FromNamedWindow(env, "PatternInvalidWaitWindow"), Field[any, int64]("waitTime")))),
		CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard())
	assertPatternInvalidBuild(t, env, "subselect-cron-observer-parameter",
		TimerCron(sb, cron).Select(Alias("c", Literal(1))).Query(),
		"subselects not allowed within pattern observer parameters")

	// Filter expressions keep accepting subqueries (Java walks observer
	// parameters only), so a pattern filter with a subselect still builds.
	if _, err := env.Build(PatternFrom(sb, "a", Equal[int](
		Field[patternOpBean, int]("intPrimitive"),
		Cast[int64, int](SubqueryValue[int64](FromNamedWindow(env, "PatternInvalidWaitWindow"), Field[any, int64]("waitTime"))),
	)).Select(Alias("c", Literal(1))).Query(StatementName("pattern-invalid-filter-subquery-ok"))); err != nil {
		t.Fatalf("pattern filter subquery was rejected: %v", err)
	}
}

// TestPatternInvalidStatementExceptionMatchesEsper mirrors
// PatternInvalid.PatternStatementException: every invalidity boundary that
// is representable in the fluent API fails at Build with the corresponding
// validation message.
func TestPatternInvalidStatementExceptionMatchesEsper(t *testing.T) {
	env := newPatternInvalidEnv(t)
	sb := From[patternOpBean](env, "SupportBean")
	sn := From[patternUseResultN](env, "SupportBean_N")
	sc := From[patternInvalidComplexProps](env, "SupportBeanComplexProps")
	theString := Field[patternOpBean, string]("theString")
	doubleF := Field[patternUseResultN, float64]("doublePrimitive")

	// timer:at(2,3,4,4,4): a single day-of-month value combined with a
	// restricted day-of-week field is rejected by the cron validator,
	// matching Java ScheduleSpecUtil.
	assertPatternInvalidBuild(t, env, "cron-dom-dow-combination",
		TimerCron(sb, NewCronSchedule(CronValues(2), CronValues(3), CronValues(4), CronValues(4), CronValues(4))).
			Select(Alias("c", Literal(1))).Query(),
		"cron single day-of-month value cannot be combined with a weekday field")

	// The multi-value day-of-month form stays valid, matching the Java
	// resolved-set rule (only single-value sets conflict).
	if _, err := env.Build(TimerCron(sb, NewCronSchedule(CronValues(2), CronValues(3), CronValues(4, 5), CronValues(4), CronValues(4))).
		Select(Alias("c", Literal(1))).Query(StatementName("pattern-invalid-cron-multi-dom-ok"))); err != nil {
		t.Fatalf("multi-value day-of-month with weekday was rejected: %v", err)
	}

	// Java passes a numeric timezone parameter (timer:at(*,*,*,*,*,0,-1));
	// the fluent timezone option is string-typed, so the observable boundary
	// is the rejection of an unknown timezone name.
	assertPatternInvalidBuild(t, env, "cron-unknown-timezone",
		TimerCron(sb, NewCronScheduleWithSeconds(CronValues(0), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard(), CronWildcard()).InTimeZone("not-a-zone")).
			Select(Alias("c", Literal(1))).Query(),
		"timezone")

	// dummypkg.dummy(): unresolvable event type.
	assertPatternInvalidBuild(t, env, "unknown-event-type",
		FromAny(env, "dummypkg.dummy").Query(),
		"no registered schema")

	// SupportBean_N(dummy=1): unknown simple property.
	assertPatternInvalidBuild(t, env, "unknown-property",
		From[patternUseResultN](env, "SupportBean_N").Filter(
			Equal[int](Field[patternUseResultN, int]("dummy"), Literal(1))).Query(),
		"unknown field")

	// SupportBean_N(dummy.nested=1): unresolvable nested property path.
	assertPatternInvalidBuild(t, env, "unknown-nested-property",
		From[patternUseResultN](env, "SupportBean_N").Filter(
			Equal[int](Field[patternUseResultN, int]("dummy.nested"), Literal(1))).Query(),
		"unknown field")

	// SupportBean_N(intPrimitive='s'): wrong-type constant.
	assertPatternInvalidBuild(t, env, "property-wrong-type-constant",
		PatternFrom(sn, "a", EqualOf(Field[patternUseResultN, int]("intPrimitive"), Literal("s"))).
			Select(Alias("c", Literal(1))).Query(),
		"equality operands are not compatible: int and string")

	// SupportBeanComplexProps(nested=1): scalar assigned to an object
	// property.
	assertPatternInvalidBuild(t, env, "property-object-wrong-type",
		PatternFrom(sc, "a", EqualOf(Field[patternInvalidComplexProps, patternInvalidNestedProps]("nested"), Literal(1))).
			Select(Alias("c", Literal(1))).Query(),
		"equality operands are not compatible")

	// SupportBean_N(doublePrimitive=x.abc): no tag matches prior use.
	assertPatternInvalidBuild(t, env, "unknown-tag-property",
		PatternFrom(sn, "a", EqualOf(doubleF, TagField[float64]("x", "abc"))).
			Select(Alias("c", Literal(1))).Query(),
		"unknown tag")

	// SupportBean(theString in [1:2]): a numeric range over a string field
	// has no implicit conversion.
	assertPatternInvalidBuild(t, env, "range-on-string-field",
		PatternFrom(sb, "a", BetweenOf(theString, Literal(1), Literal(2))).
			Select(Alias("c", Literal(1))).Query(),
		"between-of operands are not compatible: string and int")

	// SupportBean(doubleBoxed in ['a':2]): a string range bound over a
	// numeric field.
	assertPatternInvalidBuild(t, env, "range-string-bound",
		PatternFrom(sn, "a", BetweenOf(Field[patternUseResultN, float64]("doubleBoxed"), Literal("a"), Literal(2))).
			Select(Alias("c", Literal(1))).Query(),
		"between-of operands are not compatible")

	// x=SupportBean -> SupportBean(doublePrimitive=x.boolBoxed): use-result
	// correlation with a wrong-typed property (mapped to SupportBean_N,
	// which carries the same double/boolean field pair).
	assertPatternInvalidBuild(t, env, "use-result-wrong-type",
		PatternFrom(sb, "x", Literal(true)).Then(
			PatternFrom(sn, "b", EqualOf(doubleF, TagField[bool]("x", "boolBoxed"))),
		).Select(Alias("c", Literal(1))).Query(),
		"equality operands are not compatible")

	// Java's remaining cases are unrepresentable in the fluent API and are
	// documented at the top of this file: timer:within as observer,
	// timer:interval as guard, empty observer/guard parameter lists,
	// timer:within("s"), timer:at(9l) arity, and named observer parameters.
	_ = theString
}

// TestPatternInvalidUseResultMatchesEsper mirrors
// PatternInvalid.PatternUseResult: the valid/invalid tag-correlation matrix
// over followed-by filters, including range bounds.
func TestPatternInvalidUseResultMatchesEsper(t *testing.T) {
	env := newPatternInvalidEnv(t)
	sn := From[patternUseResultN](env, "SupportBean_N")
	doubleF := Field[patternUseResultN, float64]("doublePrimitive")
	intBoxedF := Field[patternUseResultN, int]("intBoxed")

	assertValid := func(name string, pattern PatternStream) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if _, err := env.Build(pattern.Select(Alias("c", Literal(1))).Query(StatementName("pattern-invalid-useresult-" + name))); err != nil {
				t.Fatalf("valid correlation %s was rejected: %v", name, err)
			}
		})
	}
	na := func() PatternStream { return PatternFrom(sn, "na", Literal(true)) }

	assertValid("doublePrimitive-eq-tag", na().Then(PatternFrom(sn, "nb",
		Equal[float64](doubleF, TagField[float64]("na", "doublePrimitive")))))
	assertValid("doublePrimitive-and-intBoxed-tags", na().Then(PatternFrom(sn, "nb",
		And(
			Equal[float64](doubleF, TagField[float64]("na", "doublePrimitive")),
			Equal[int](intBoxedF, TagField[int]("na", "intBoxed")),
		))))
	// doublePrimitive in (na.doublePrimitive:na.doubleBoxed): open range.
	assertValid("open-range-tags", na().Then(PatternFrom(sn, "nb",
		And(
			GreaterOf(doubleF, TagField[float64]("na", "doublePrimitive")),
			LessOf(doubleF, TagField[float64]("na", "doubleBoxed")),
		))))
	// doublePrimitive in [na.doublePrimitive:na.doubleBoxed]: closed range.
	assertValid("closed-range-tags", na().Then(PatternFrom(sn, "nb",
		BetweenOf(doubleF, TagField[float64]("na", "doublePrimitive"), TagField[float64]("na", "doubleBoxed")))))
	// doublePrimitive in [na.intBoxed:na.intPrimitive]: numeric range bounds
	// of a different numeric type stay valid.
	assertValid("closed-range-int-tags", na().Then(PatternFrom(sn, "nb",
		BetweenOf(doubleF, TagField[int]("na", "intBoxed"), TagField[int]("na", "intPrimitive")))))

	// xx=... -> nb(doublePrimitive = na.doublePrimitive): na is not bound.
	assertPatternInvalidBuild(t, env, "unbound-na",
		PatternFrom(sn, "xx", Literal(true)).Then(PatternFrom(sn, "nb",
			Equal[float64](doubleF, TagField[float64]("na", "doublePrimitive")))).Select(Alias("c", Literal(1))).Query(),
		"unknown tag")
	// na -> nb(doublePrimitive = xx.doublePrimitive): xx is not bound.
	assertPatternInvalidBuild(t, env, "unbound-xx",
		na().Then(PatternFrom(sn, "nb",
			Equal[float64](doubleF, TagField[float64]("xx", "doublePrimitive")))).Select(Alias("c", Literal(1))).Query(),
		"unknown tag")
	// na -> nb(doublePrimitive = na.xx): unknown property on a bound tag.
	assertPatternInvalidBuild(t, env, "unknown-tag-field",
		na().Then(PatternFrom(sn, "nb",
			Equal[float64](doubleF, TagField[float64]("na", "xx")))).Select(Alias("c", Literal(1))).Query(),
		"not valid on schema")
	// xx=... -> nb(xx = na.doublePrimitive): unknown filter property (na is
	// also unbound; either validation may fire first).
	assertPatternInvalidBuild(t, env, "unknown-filter-property",
		PatternFrom(sn, "xx", Literal(true)).Then(PatternFrom(sn, "nb",
			EqualOf(Field[patternUseResultN, int]("xx"), TagField[float64]("na", "doublePrimitive")))).Select(Alias("c", Literal(1))).Query())
	// na -> nb(xx = na.xx): unknown filter property and unknown tag property.
	assertPatternInvalidBuild(t, env, "unknown-filter-and-tag-property",
		na().Then(PatternFrom(sn, "nb",
			EqualOf(Field[patternUseResultN, int]("xx"), TagField[float64]("na", "xx")))).Select(Alias("c", Literal(1))).Query())
	// na -> nb(doublePrimitive in [na.intBoxed:na.xx]): unknown upper-bound
	// tag property.
	assertPatternInvalidBuild(t, env, "range-unknown-upper-tag",
		na().Then(PatternFrom(sn, "nb",
			BetweenOf(doubleF, TagField[int]("na", "intBoxed"), TagField[int]("na", "xx")))).Select(Alias("c", Literal(1))).Query(),
		"not valid on schema")
	// na -> nb(doublePrimitive in [na.intBoxed:na.boolBoxed]): a boolean
	// range bound is not ordered.
	assertPatternInvalidBuild(t, env, "range-bool-upper-bound",
		na().Then(PatternFrom(sn, "nb",
			BetweenOf(doubleF, TagField[int]("na", "intBoxed"), TagField[bool]("na", "boolBoxed")))).Select(Alias("c", Literal(1))).Query(),
		"must be ordered")
	// na -> nb(doublePrimitive in [na.xx:na.intPrimitive]): unknown
	// lower-bound tag property.
	assertPatternInvalidBuild(t, env, "range-unknown-lower-tag",
		na().Then(PatternFrom(sn, "nb",
			BetweenOf(doubleF, TagField[int]("na", "xx"), TagField[int]("na", "intPrimitive")))).Select(Alias("c", Literal(1))).Query(),
		"not valid on schema")
	// na -> nb(doublePrimitive in [na.boolBoxed:na.intPrimitive]): a boolean
	// lower range bound is not ordered.
	assertPatternInvalidBuild(t, env, "range-bool-lower-bound",
		na().Then(PatternFrom(sn, "nb",
			BetweenOf(doubleF, TagField[bool]("na", "boolBoxed"), TagField[int]("na", "intPrimitive")))).Select(Alias("c", Literal(1))).Query(),
		"must be ordered")
}
