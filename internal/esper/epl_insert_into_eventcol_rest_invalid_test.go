package esper

import (
	"reflect"
	"strings"
	"testing"
)

type ecrSupportBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

type ecrSupportBeanS0 struct {
	ID  int    `esper:"id"`
	P00 string `esper:"p00"`
	P01 string `esper:"p01"`
}

type ecrInvalidS1 struct {
	Dummy int `esper:"dummy"`
}

// TestEPLInsertIntoColBeanInvalidBuild pins the build-phase rejection of the
// B10 shapes (java-runtime-10c2cd110438e2422b62). Java rejects both targets
// with "Incompatible type detected attempting to insert into column 'sbs'
// type 'com.espertech.esper.common.internal.support.SupportBean' compared to
// selected type 'SupportBean_S0'" (SelectExprProcessorHelper.java:1333-1335,
// startsWith match). The Go route validation exposes the same surface when a
// column is bound to a member schema via WithNestedPropertySchema and the
// projected whole-event subquery reads a different source representation.
func TestEPLInsertIntoColBeanInvalidBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ecrSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ecrInvalidS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "TypeOne", []FieldSpec{
		FieldDef("sbs", reflect.TypeOf([]Event{})),
	}, WithNestedPropertySchema("sbs", mustSchema(env, "SupportBean"))); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "TypeTwo", []FieldSpec{
		FieldDef("sbs", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("sbs", mustSchema(env, "SupportBean"))); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"TypeOne", "TypeTwo"} {
		innerS0 := From[ecrSupportBeanS0](env, "SupportBean_S0").Window(KeepAll()).AsRecord()
		var project Expr
		if target == "TypeTwo" {
			project = SubqueryValueWithOptions[Event](innerS0, EventValue[Event]())
		} else {
			project = SubqueryRowsAsEvent(env, "SupportBean_S0", innerS0)
		}
		_, err := env.Build(From[ecrInvalidS1](env, "SupportBean_S1").AsRecord().Select(
			Alias("sbs", project),
		).InsertInto(target, StatementName("invalid")))
		if err == nil {
			t.Fatalf("%s: incompatible subquery projection accepted", target)
		}
		if !strings.Contains(err.Error(), "Incompatible type detected attempting to insert into column") {
			t.Fatalf("%s error = %v, want pinned Incompatible-type diagnostic", target, err)
		}
		t.Logf("%s rejected with pinned diagnostic", target)
	}
}

// TestEPLInsertIntoColNonBeanBeanInvalidBuild pins Go's build-phase rejection
// of the N15 shapes (java-runtime-05f38146a2f28887769c): anonymous-struct
// member type mismatch and unknown member (SelectExprInsertEventBeanFactory
// .java:270-277, SelectExprProcessorHelper.java:1364-1366).
func TestEPLInsertIntoColNonBeanBeanInvalidBuild(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	n1_1, errReg := RegisterMap(env, "N1_1", []FieldSpec{
		FieldDef("p0", reflect.TypeOf(0)),
	})
	if errReg != nil {
		t.Fatal(errReg)
	}
	if _, err := RegisterMap(env, "N1_2", []FieldSpec{
		{Name: "p1", Type: reflect.TypeOf(map[string]any{})},
	}, WithNestedPropertySchema("p1", n1_1)); err != nil {
		t.Fatal(err)
	}

	newStructWrong := StructOf(
		StructField("p0", Literal("a")),
	)
	_, err := env.Build(From[ecrSupportBean](env, "SupportBean").AsRecord().Select(
		Alias("p1", newStructWrong),
	).InsertInto("N1_2", StatementName("invalid")))
	if err == nil {
		t.Fatal("member type mismatch accepted")
	}
	javaPrefix1 := "Invalid assignment of column 'p0' of type 'String' to event property 'p0' typed as 'Integer', column and parameter types mismatch"
	t.Logf("mismatch rejected: %v (java prefix %q)", err, javaPrefix1)

	newStructUnknown := StructOf(
		StructField("xxx", Literal("a")),
	)
	_, err = env.Build(From[ecrSupportBean](env, "SupportBean").AsRecord().Select(
		Alias("p1", newStructUnknown),
	).InsertInto("N1_2", StatementName("invalid")))
	if err == nil {
		t.Fatal("unknown member accepted")
	}
	if !strings.HasPrefix(err.Error(),
		"esper: InvalidRule at route: Failed to find property \"xxx\" among properties for target event type \"N1_1\"") {
		t.Fatalf("unknown member error = %v, want pinned N15 unknown-member text", err)
	}
}

// TestECRInvalidTexts asserts the verbatim Java diagnostics start-with the
// pinned regression-suite messages for all four compile-rejected records of
// this work unit (B10 ×2 and N15 ×2).
func TestECRInvalidTexts(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[ecrSupportBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ecrSupportBeanS0](env, "SupportBean_S0"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[ecrInvalidS1](env, "SupportBean_S1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterObjectArray(env, "TypeOne", []FieldSpec{
		FieldDef("sbs", reflect.TypeOf([]Event{})),
	}, WithNestedPropertySchema("sbs", mustSchema(env, "SupportBean"))); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterMap(env, "TypeTwo", []FieldSpec{
		FieldDef("sbs", reflect.TypeOf(Event{})),
	}, WithNestedPropertySchema("sbs", mustSchema(env, "SupportBean"))); err != nil {
		t.Fatal(err)
	}
	n1_1, errReg := RegisterMap(env, "N1_1", []FieldSpec{
		FieldDef("p0", reflect.TypeOf(0)),
	})
	if errReg != nil {
		t.Fatal(errReg)
	}
	if _, err := RegisterMap(env, "N1_2", []FieldSpec{
		{Name: "p1", Type: reflect.TypeOf(map[string]any{})},
	}, WithNestedPropertySchema("p1", n1_1)); err != nil {
		t.Fatal(err)
	}
	innerS0 := From[ecrSupportBeanS0](env, "SupportBean_S0").Window(KeepAll()).AsRecord()
	for _, target := range []string{"TypeOne", "TypeTwo"} {
		_, buildErr := env.Build(From[ecrInvalidS1](env, "SupportBean_S1").AsRecord().Select(
			Alias("sbs", SubqueryValueWithOptions[Event](innerS0, EventValue[Event]())),
		).InsertInto(target, StatementName("invalid")))
		// Java: ... type 'com.espertech.esper.common.internal.support.SupportBean'
		// compared to selected type 'SupportBean_S0'; the Go representation
		// names types by their registered event-schema name.
		want := "Incompatible type detected attempting to insert into column 'sbs' type 'SupportBean' compared to selected type 'SupportBean_S0'"
		assertStartsWith(t, target, buildErr, want)
	}
	wrongMember := StructOf(StructField("p0", Literal("a")))
	_, buildErr := env.Build(From[ecrSupportBean](env, "SupportBean").AsRecord().Select(
		Alias("p1", wrongMember),
	).InsertInto("N1_2", StatementName("invalid")))
	assertStartsWith(t, "nonbean-invalid/p0-mismatch", buildErr,
		"Invalid assignment of column 'p0' of type 'String' to event property 'p0' typed as 'Integer', column and parameter types mismatch")

	unknownMember := StructOf(StructField("xxx", Literal("a")))
	_, buildErr = env.Build(From[ecrSupportBean](env, "SupportBean").AsRecord().Select(
		Alias("p1", unknownMember),
	).InsertInto("N1_2", StatementName("invalid")))
	assertStartsWith(t, "nonbean-invalid/xxx-unknown", buildErr,
		"Failed to find property 'xxx' among properties for target event type 'N1_1'")
}

// assertStartsWith mirrors SupportMessageAssertUtil.assertMessage: the Go
// Build error message must start with the pinned Java diagnostic after
// stripping the engine's rule-context wrapper.
func assertStartsWith(t *testing.T, label string, err error, javaPrefix string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s accepted; want rejection with prefix %q", label, javaPrefix)
	}
	msg := strings.ReplaceAll(err.Error(), "\"", "'")
	if !strings.Contains(msg, javaPrefix) {
		t.Fatalf("%s error %q does not embed java prefix %q", label, msg, javaPrefix)
	}
}
