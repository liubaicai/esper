package esper

import (
	"context"
	"reflect"
	"testing"
)

// The struct field table caches the ordered candidate walk, so these tests
// pin the resolution semantics that walk must keep: an unresolved candidate
// (for example a nil anonymous pointer) must fall through to later
// candidates, recursive embedding must terminate, tag precedence must not
// change, and case-insensitive matching must keep strings.EqualFold
// semantics.

type propertyAccessFallbackLeaf struct {
	Leaf string `esper:"probe"`
}

type propertyAccessFallbackHolder struct {
	*propertyAccessFallbackLeaf
	Probe string
}

type propertyAccessSelfRecursive struct {
	*propertyAccessSelfRecursive
	Value string `esper:"value"`
}

type propertyAccessTagPrecedence struct {
	Tagged    string `esper:"tagged" json:"jsonTagged"`
	JSONOnly  string `json:"jsonOnly"`
	Ignored   string `esper:"-"`
	PlainName string
}

type propertyAccessFold struct {
	Kelvin string `esper:"k"`
	LongS  string `esper:"s"`
	Mixed  string `esper:"mixedCase"`
}

func TestStructPropertyFallbackFollowsCandidateOrder(t *testing.T) {
	schema, err := StructSchema[propertyAccessFallbackHolder]("PropertyAccessFallback", WithPropertyResolution(PropertyCaseInsensitive))
	if err != nil {
		t.Fatal(err)
	}
	holder := propertyAccessFallbackHolder{Probe: "outer"}
	if got := schema.get(holder, "probe").Any(); got != "outer" {
		t.Fatalf("nil anonymous candidate must fall through to the later Go field, got %v", got)
	}
	holder.propertyAccessFallbackLeaf = &propertyAccessFallbackLeaf{Leaf: "inner"}
	if got := schema.get(holder, "probe").Any(); got != "inner" {
		t.Fatalf("earlier embedded candidate must win once it resolves, got %v", got)
	}
	if got := schema.get(&holder, "probe").Any(); got != "inner" {
		t.Fatalf("pointer-valued events must resolve identically, got %v", got)
	}
}

// TestStructPropertyResolutionTerminatesOnRecursiveEmbedding pins two
// pre-existing behaviors the cached walk must keep: schema registration still
// rejects recursive embedding, and the accessor still resolves such a type
// without looping for dynamic property access that bypasses registration.
func TestStructPropertyResolutionTerminatesOnRecursiveEmbedding(t *testing.T) {
	if _, err := StructSchema[propertyAccessSelfRecursive]("PropertyAccessSelfRecursive"); err == nil {
		t.Fatal("recursive embedded struct must stay rejected by schema registration")
	}
	value := &propertyAccessSelfRecursive{Value: "value"}
	value.propertyAccessSelfRecursive = value
	if got := structFieldValue(reflect.ValueOf(value), "value", PropertyCaseSensitive).Interface(); got != "value" {
		t.Fatalf("value = %v", got)
	}
	if structFieldValue(reflect.ValueOf(value), "missing", PropertyCaseSensitive).IsValid() {
		t.Fatal("unknown property must stay invalid")
	}
}

func TestStructPropertyTagPrecedenceUnchanged(t *testing.T) {
	schema, err := StructSchema[propertyAccessTagPrecedence]("PropertyAccessTags")
	if err != nil {
		t.Fatal(err)
	}
	value := propertyAccessTagPrecedence{Tagged: "t", JSONOnly: "j", Ignored: "i", PlainName: "p"}
	for name, expected := range map[string]any{
		"tagged": "t", "Tagged": "t", "jsonOnly": "j", "JSONOnly": "j", "PlainName": "p",
	} {
		if got := schema.get(value, name).Any(); got != expected {
			t.Fatalf("property %q = %v, want %v", name, got, expected)
		}
	}
	for _, name := range []string{"jsonTagged", "Ignored", "ignored"} {
		if !schema.get(value, name).IsMissing() {
			t.Fatalf("property %q must not be exposed by the esper tag rules", name)
		}
	}
}

func TestStructPropertyCaseInsensitiveFoldingMatchesEqualFold(t *testing.T) {
	schema, err := StructSchema[propertyAccessFold]("PropertyAccessFold", WithPropertyResolution(PropertyCaseInsensitive))
	if err != nil {
		t.Fatal(err)
	}
	value := propertyAccessFold{Kelvin: "kelvin", LongS: "longs", Mixed: "mixed"}
	for name, expected := range map[string]any{
		// U+212A KELVIN SIGN and U+017F LATIN SMALL LETTER LONG S only compare
		// equal under simple fold, not under strings.ToLower.
		"k": "kelvin", "K": "kelvin", "\u212A": "kelvin",
		"s": "longs", "S": "longs", "\u017F": "longs",
		"mixedCase": "mixed", "MIXEDCASE": "mixed",
	} {
		if got := schema.get(value, name).Any(); got != expected {
			t.Fatalf("property %q = %v, want %v", name, got, expected)
		}
	}
}

type propertyAccessWideEvent struct {
	F000 string `esper:"f000"`
	F001 string `esper:"f001"`
	F002 string `esper:"f002"`
	F003 string `esper:"f003"`
	F004 string `esper:"f004"`
	F005 string `esper:"f005"`
	F006 string `esper:"f006"`
	F007 string `esper:"f007"`
	F008 string `esper:"f008"`
	F009 string `esper:"f009"`
	F010 string `esper:"f010"`
	F011 string `esper:"f011"`
	F012 string `esper:"f012"`
	F013 string `esper:"f013"`
	F014 string `esper:"f014"`
	F015 string `esper:"f015"`
	F016 string `esper:"f016"`
	F017 string `esper:"f017"`
	F018 string `esper:"f018"`
	F019 string `esper:"f019"`
	F020 string `esper:"f020"`
	F021 string `esper:"f021"`
	F022 string `esper:"f022"`
	F023 string `esper:"f023"`
	F024 string `esper:"f024"`
	F025 string `esper:"f025"`
	F026 string `esper:"f026"`
	F027 string `esper:"f027"`
	F028 string `esper:"f028"`
	F029 string `esper:"f029"`
	F030 string `esper:"f030"`
	F031 string `esper:"f031"`
	F032 string `esper:"f032"`
	F033 string `esper:"f033"`
	F034 string `esper:"f034"`
	F035 string `esper:"f035"`
	F036 string `esper:"f036"`
	F037 string `esper:"f037"`
	F038 string `esper:"f038"`
	F039 string `esper:"f039"`
	F040 string `esper:"f040"`
	F041 string `esper:"f041"`
	F042 string `esper:"f042"`
	F043 string `esper:"f043"`
	F044 string `esper:"f044"`
	F045 string `esper:"f045"`
	F046 string `esper:"f046"`
	F047 string `esper:"f047"`
	F048 string `esper:"f048"`
	F049 string `esper:"f049"`
	F050 string `esper:"f050"`
	F051 string `esper:"f051"`
	F052 string `esper:"f052"`
	F053 string `esper:"f053"`
	F054 string `esper:"f054"`
	F055 string `esper:"f055"`
	F056 string `esper:"f056"`
	F057 string `esper:"f057"`
	F058 string `esper:"f058"`
	F059 string `esper:"f059"`
	F060 string `esper:"f060"`
	F061 string `esper:"f061"`
	F062 string `esper:"f062"`
	F063 string `esper:"f063"`
}

// allocationPerSend measures the allocation count of one SendEvent through a
// single-condition filter that probes the given property.
func allocationPerSend(t *testing.T, field string) float64 {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[propertyAccessWideEvent](env, "PropertyAccessWideEvent"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[propertyAccessWideEvent](env, "PropertyAccessWideEvent").
		Filter(Equal[string](
			Field[propertyAccessWideEvent, string](field),
			Literal("absent"),
		)).
		Query(StatementName("property-access-wide")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	ctx := context.Background()
	if _, err := engine.Deploy(ctx, plan); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = engine.Close(ctx) }()
	event := propertyAccessWideEvent{}
	return testing.AllocsPerRun(50, func() {
		if err := engine.SendEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	})
}

// TestPropertyAccessAllocationIsPositionIndependent guards the cached
// accessor table: reading a property declared last must not cost more
// allocations than reading one declared first. The per-lookup field scan
// allocated work proportional to the declaring position (148 versus 22
// allocations for this shape) and would fail this comparison.
func TestPropertyAccessAllocationIsPositionIndependent(t *testing.T) {
	first := allocationPerSend(t, "f000")
	last := allocationPerSend(t, "f063")
	if last > first {
		t.Fatalf("last-field property read allocates %.0f per send versus %.0f for the first field", last, first)
	}
}

func BenchmarkPropertyAccessByFieldPosition(b *testing.B) {
	for _, field := range []string{"f000", "f031", "f063"} {
		b.Run(field, func(b *testing.B) {
			env := NewEnvironment()
			if _, err := RegisterStruct[propertyAccessWideEvent](env, "PropertyAccessWideEvent"); err != nil {
				b.Fatal(err)
			}
			plan, err := env.Build(From[propertyAccessWideEvent](env, "PropertyAccessWideEvent").
				Filter(Equal[string](
					Field[propertyAccessWideEvent, string](field),
					Literal("absent"),
				)).
				Query(StatementName("property-access-wide")))
			if err != nil {
				b.Fatal(err)
			}
			engine := NewEngine(env)
			ctx := context.Background()
			if _, err := engine.Deploy(ctx, plan); err != nil {
				b.Fatal(err)
			}
			defer func() { _ = engine.Close(ctx) }()
			event := propertyAccessWideEvent{}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := engine.SendEvent(ctx, event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
