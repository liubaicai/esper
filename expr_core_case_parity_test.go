package esper

import (
	"context"
	"testing"
)

// caseParityMarketData mirrors SupportMarketDataBean (symbol, volume).
type caseParityMarketData struct {
	Symbol string `esper:"symbol"`
	Volume int64  `esper:"volume"`
	Price  float64 `esper:"price"`
}

// caseParityBean mirrors SupportBean numeric fields for Syntax2 case tests.
type caseParityBean struct {
	IntPrimitive    int     `esper:"intPrimitive"`
	LongPrimitive   int64   `esper:"longPrimitive"`
	FloatPrimitive  float32 `esper:"floatPrimitive"`
	DoublePrimitive float64 `esper:"doublePrimitive"`
}

func caseEnv(t *testing.T) (*Environment, *Engine) {
	t.Helper()
	env := NewEnvironment()
	if _, err := RegisterStruct[caseParityMarketData](env, "SupportMarketDataBean"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterStruct[caseParityBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	return env, NewEngine(env)
}

func sendCaseMarket(t *testing.T, engine *Engine, symbol string, volume int64) {
	t.Helper()
	if err := engine.SendEvent(context.Background(), caseParityMarketData{Symbol: symbol, Volume: volume}); err != nil {
		t.Fatal(err)
	}
}

func subscribeInt64(dep *Deployment, field string, collect *[]int64) {
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			v, _ := As[int64](r.Get(field))
			*collect = append(*collect, v)
		}
		return nil
	})
}

func subscribeF64(dep *Deployment, field string, collect *[]float64) {
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			v, _ := As[float64](r.Get(field))
			*collect = append(*collect, v)
		}
		return nil
	})
}

// ExprCoreCaseSyntax1WithElse (ordinal 3). Java:
//   select case when symbol='DELL' then volume*3 else volume end as p1
//   from SupportMarketDataBean#length(3)
// When symbol != DELL, return volume (else). When DELL, return volume*3.
func TestExprCoreCaseSyntax1WithElseParity(t *testing.T) {
	env, engine := caseEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[caseParityMarketData](env, "SupportMarketDataBean").Window(LengthWindow(3))
	expr := CaseWhen[int64](
		Equal[string](Field[caseParityMarketData, string]("symbol"), Literal("DELL")),
		Multiply[int64](Field[caseParityMarketData, int64]("volume"), Literal(int64(3))),
	).Else(Field[caseParityMarketData, int64]("volume"))
	plan, err := env.Build(Select(input, Alias("p1", expr)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []int64
	subscribeInt64(dep, "p1", &results)
	sendCaseMarket(t, engine, "CSCO", 4000)
	if len(results) != 1 || results[0] != 4000 {
		t.Fatalf("Syntax1WithElse CSCO: p1 = %v, want [4000]", results)
	}
	sendCaseMarket(t, engine, "DELL", 20)
	if len(results) != 2 || results[1] != 60 {
		t.Fatalf("Syntax1WithElse DELL: p1 = %v, want [4000, 60]", results)
	}
}

// ExprCoreCaseSyntax1Branches3 (ordinal 6). Java:
//   case when (symbol='GE') then volume
//     when (symbol='DELL') then volume / 2.0
//     when (symbol='MSFT') then volume / 3.0
//   end
// No ELSE clause; unmatched symbol yields null.
func TestExprCoreCaseSyntax1Branches3Parity(t *testing.T) {
	env, engine := caseEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[caseParityMarketData](env, "SupportMarketDataBean")
	expr := CaseWhen[float64](
		Equal[string](Field[caseParityMarketData, string]("symbol"), Literal("GE")),
		Cast[int64, float64](Field[caseParityMarketData, int64]("volume")),
	).When(
		Equal[string](Field[caseParityMarketData, string]("symbol"), Literal("DELL")),
		DivideFloat(Field[caseParityMarketData, int64]("volume"), Literal(2.0)),
	).When(
		Equal[string](Field[caseParityMarketData, string]("symbol"), Literal("MSFT")),
		DivideFloat(Field[caseParityMarketData, int64]("volume"), Literal(3.0)),
	).Build()
	plan, err := env.Build(Select(input, Alias("c0", expr)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []*float64
	_, _ = dep.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, r := range batch.New {
			raw := r.Get("c0").Any()
			if raw == nil {
				results = append(results, nil)
				continue
			}
			v, _ := As[float64](r.Get("c0"))
			results = append(results, &v)
		}
		return nil
	})
	// symbol=GE → volume=100
	sendCaseMarket(t, engine, "GE", 100)
	if len(results) != 1 || results[0] == nil || *results[0] != 100 {
		t.Fatalf("Branches3 GE: c0 = %v, want 100", results)
	}
	// symbol=DELL → volume/2.0 = 50
	sendCaseMarket(t, engine, "DELL", 100)
	if len(results) != 2 || results[1] == nil || *results[1] != 50 {
		t.Fatalf("Branches3 DELL: c0 = %v, want 50", results)
	}
	// symbol=UNKNOWN → null (no else)
	sendCaseMarket(t, engine, "JOE", 100)
	if len(results) != 3 || results[2] != nil {
		t.Fatalf("Branches3 JOE: c0 = %v, want nil", results)
	}
}

// ExprCoreCaseSyntax2 (ordinal 7). Java:
//   case intPrimitive
//     when longPrimitive then (intPrimitive + longPrimitive)
//     when doublePrimitive then intPrimitive * doublePrimitive
//     when floatPrimitive then floatPrimitive / doublePrimitive
//     else (intPrimitive + longPrimitive + floatPrimitive + doublePrimitive) end
// The result type is Double due to mixed-numeric branches.
func TestExprCoreCaseSyntax2Parity(t *testing.T) {
	env, engine := caseEnv(t)
	defer func() { _ = engine.Close(context.Background()) }()
	input := From[caseParityBean](env, "SupportBean")
	expr := CaseValue[float64](
		Field[caseParityBean, int]("intPrimitive"),
		Field[caseParityBean, int64]("longPrimitive"),
		AddOf[float64](Field[caseParityBean, int]("intPrimitive"), Field[caseParityBean, int64]("longPrimitive")),
	).When(
		Field[caseParityBean, float64]("doublePrimitive"),
		MultiplyOf[float64](Field[caseParityBean, int]("intPrimitive"), Field[caseParityBean, float64]("doublePrimitive")),
	).When(
		Field[caseParityBean, float32]("floatPrimitive"),
		DivideFloat(Field[caseParityBean, float32]("floatPrimitive"), Field[caseParityBean, float64]("doublePrimitive")),
	).Else(
		AddOf[float64](
			AddOf[float64](Field[caseParityBean, int]("intPrimitive"), Field[caseParityBean, int64]("longPrimitive")),
			AddOf[float64](Field[caseParityBean, float32]("floatPrimitive"), Field[caseParityBean, float64]("doublePrimitive")),
		),
	)
	plan, err := env.Build(Select(input, Alias("c0", expr)).Query(StatementName("s0")))
	if err != nil {
		t.Fatal(err)
	}
	dep, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dep.Undeploy(context.Background()) }()
	var results []float64
	subscribeF64(dep, "c0", &results)
	// int=2, long=2, float=1, double=1 → int==long → 2+2=4
	if err := engine.SendEvent(context.Background(), caseParityBean{IntPrimitive: 2, LongPrimitive: 2, FloatPrimitive: 1, DoublePrimitive: 1}); err != nil {
		t.Fatal(err)
	}
	// int=5, long=1, float=1, double=5 → int==double → 5*5=25
	if err := engine.SendEvent(context.Background(), caseParityBean{IntPrimitive: 5, LongPrimitive: 1, FloatPrimitive: 1, DoublePrimitive: 5}); err != nil {
		t.Fatal(err)
	}
	// int=12, long=1, float=12, double=4 → int==float → 12/4=3
	if err := engine.SendEvent(context.Background(), caseParityBean{IntPrimitive: 12, LongPrimitive: 1, FloatPrimitive: 12, DoublePrimitive: 4}); err != nil {
		t.Fatal(err)
	}
	// int=1, long=2, float=3, double=4 → no match → else=1+2+3+4=10
	if err := engine.SendEvent(context.Background(), caseParityBean{IntPrimitive: 1, LongPrimitive: 2, FloatPrimitive: 3, DoublePrimitive: 4}); err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("Syntax2: result count = %d, want 4", len(results))
	}
	expected := []float64{4, 25, 3, 10}
	for i, want := range expected {
		if results[i] != want {
			t.Fatalf("Syntax2[%d]: c0 = %v, want %v (all: %v)", i, results[i], want, results)
		}
	}
}
