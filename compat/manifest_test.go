package compat

import "testing"

func TestStaticManifestScannerProducesStableCandidate(t *testing.T) {
	source := `package example;
public class ViewCase {
  public static class Execution implements RegressionExecution {
    public String name() { return "length"; }
    public EnumSet<RegressionFlag> flags() { return EnumSet.of(RegressionFlag.PERFORMANCE); }
  }
}`
	cases := ScanRegressionSource("suite/view/ViewCase.java", source)
	if len(cases) != 1 {
		t.Fatalf("cases = %#v", cases)
	}
	if cases[0].ExecutionClass != "example.ViewCase$Execution" || cases[0].ExecutionName != "length" {
		t.Fatalf("case = %#v", cases[0])
	}
	if len(cases[0].Flags) != 1 || cases[0].Flags[0] != "PERFORMANCE" {
		t.Fatalf("flags = %#v", cases[0].Flags)
	}
	if cases[0].Discovery != "static-candidate" || len(cases[0].ID) != len("java-")+20 {
		t.Fatalf("candidate metadata = %#v", cases[0])
	}
}

func TestSourceTestKindAndModuleAreStable(t *testing.T) {
	if kind, ok := sourceTestKind("runtime/src/test/java/example/RuntimeTest.java"); !ok || kind != "core-unit" {
		t.Fatalf("core source kind = %q, %v", kind, ok)
	}
	if kind, ok := sourceTestKind("esperio/esperio-kafka/src/test/java/example/KafkaTest.java"); !ok || kind != "esperio-unit" {
		t.Fatalf("EsperIO source kind = %q, %v", kind, ok)
	}
	if kind, ok := sourceTestKind("regression-run/src/test/java/example/RegressionSuite.java"); !ok || kind != "regression-entry" {
		t.Fatalf("regression source kind = %q, %v", kind, ok)
	}
	if kind, ok := sourceTestKind("examples/stockticker/src/test/java/example/StockTickerTest.java"); !ok || kind != "example-test" {
		t.Fatalf("example test kind = %q, %v", kind, ok)
	}
	if kind, ok := sourceTestKind("examples/stockticker/src/main/java/example/StockTicker.java"); !ok || kind != "example-source" {
		t.Fatalf("example source kind = %q, %v", kind, ok)
	}
	if module := moduleFromPath("runtime/src/test/java/example/RuntimeTest.java"); module != "runtime" {
		t.Fatalf("module = %q", module)
	}
	if module := moduleFromPath("esperio/esperio-kafka/src/test/java/example/KafkaTest.java"); module != "esperio-kafka" {
		t.Fatalf("EsperIO module = %q", module)
	}
}
