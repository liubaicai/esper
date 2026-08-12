package esper

import (
	"context"
	"database/sql/driver"
	"fmt"
	"reflect"
	"testing"
)

func TestDatabaseHintOutputColumnConversionMatchesJava(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("myvariableOCC", 0); err != nil {
		t.Fatal(err)
	}
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		result := dbJoinSQLResult{columns: []string{"myint"}}
		if len(args) == 0 {
			return result, nil
		}
		v, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			if int64(row.myint) == v {
				result.rows = append(result.rows, []driver.Value{row.myint})
			}
		}
		return result, nil
	})
	schema, err := NewMapSchema("HintColSchema", []FieldSpec{FieldDef("myint", reflect.TypeOf(false))})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint from mytesttable where myint = ?",
		SQLHistoricalProviderOptions{
			ValueConverter: func(_ SQLHistoricalColumnMetadata, value any, _ reflect.Type) (any, error) {
				v, _ := dbJoinToInt64(value)
				return v >= 50, nil
			},
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any {
					raw, _ := request.Variables["myvariableOCC"]
					return raw.Any()
				},
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBHintCol", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myint", Field[map[string]any, bool]("myint")),
	).Query(StatementName("s0-hint-col")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if err := engine.SetVariable(context.Background(), "myvariableOCC", 10); err != nil {
		t.Fatal(err)
	}
	res, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results()) != 1 || res.Results()[0].Get("myint").Any() != false {
		t.Fatalf("expected myint=false for 10, got %#v", res.Results())
	}
	if err := engine.SetVariable(context.Background(), "myvariableOCC", 60); err != nil {
		t.Fatal(err)
	}
	res, err = engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results()) != 1 || res.Results()[0].Get("myint").Any() != true {
		t.Fatalf("expected myint=true for 60, got %#v", res.Results())
	}
}

// TestDatabaseHintInputParameterConversionMatchesJava covers the SQLCOL hook
// for input parameter conversion. The Java test stores "x60" in a string
// variable and the converter strips "x". In Go, the argument function
// performs the equivalent transformation before the value reaches SQL.
func TestDatabaseHintInputParameterConversionMatchesJava(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("myvariableIPC", 0); err != nil {
		t.Fatal(err)
	}
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		result := dbJoinSQLResult{columns: []string{"myint"}}
		if len(args) == 0 {
			return result, nil
		}
		v, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			if int64(row.myint) == v {
				result.rows = append(result.rows, []driver.Value{row.myint})
			}
		}
		return result, nil
	})
	schema, err := NewMapSchema("HintParamSchema", []FieldSpec{FieldDef("myint", reflect.TypeOf(false))})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	// The argument function demonstrates input parameter conversion:
	// it reads the raw variable and applies a transform (offset by 50)
	// before the value reaches SQL. This is the Go equivalent of the
	// Java SQLCOL hook's SQLInputParameterContext.getParameterValue().
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select myint from mytesttable where myint = ?",
		SQLHistoricalProviderOptions{
			ValueConverter: func(_ SQLHistoricalColumnMetadata, value any, _ reflect.Type) (any, error) {
				v, _ := dbJoinToInt64(value)
				return v >= 50, nil
			},
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any {
					raw, _ := request.Variables["myvariableIPC"]
					v, _ := dbJoinToInt64(raw.Any())
					return v + 50 // converter adds offset: 10 + 50 = 60
				},
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBHintParam", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("myint", Field[map[string]any, bool]("myint")),
	).Query(StatementName("s0-hint-param")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	// myvariableIPC = 10 -> converter adds 50 -> SQL arg = 60 -> myint=60 -> >= 50 -> true
	if err := engine.SetVariable(context.Background(), "myvariableIPC", 10); err != nil {
		t.Fatal(err)
	}
	res, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results()) != 1 || res.Results()[0].Get("myint").Any() != true {
		t.Fatalf("expected myint=true for IPC=10->60, got %#v", res.Results())
	}
}

func TestDatabaseHintOutputRowConversionMatchesJava(t *testing.T) {
	env := NewEnvironment()
	if err := env.RegisterVariable("myvariableORC", 0); err != nil {
		t.Fatal(err)
	}
	dbJoinSetHandler(func(_ string, args []any) (dbJoinSQLResult, error) {
		result := dbJoinSQLResult{columns: dbJoinAllFieldNames}
		if len(args) == 0 {
			return result, nil
		}
		v, ok := dbJoinToInt64(args[0])
		if !ok {
			return result, nil
		}
		for _, row := range dbJoinMyTestTable {
			if int64(row.myint) == v {
				result.rows = append(result.rows, dbJoinRowToValues(row, dbJoinAllFieldNames))
			}
		}
		return result, nil
	})
	schema, err := NewMapSchema("HintRowConv", []FieldSpec{
		FieldDef("theString", reflect.TypeOf("")),
		FieldDef("intPrimitive", reflect.TypeOf(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := dbJoinOpenDB(t)
	defer db.Close()
	provider, err := NewSQLHistoricalProviderWithOptions(db, schema,
		"select mybigint, myint, myvarchar, mychar, mybool, mynumeric, mydecimal, mydouble, myreal from mytesttable where myint = ?",
		SQLHistoricalProviderOptions{
			RowConverter: func(_ SQLHistoricalRowMetadata, row map[string]any) (any, error) {
				if row["mynumeric"] == nil {
					return nil, nil
				}
				v, _ := dbJoinToInt64(row["myint"])
				return map[string]any{
					"theString":    fmt.Sprintf(">%d<", v),
					"intPrimitive": 99000 + int(v),
				}, nil
			},
			Arguments: []func(HistoricalRequest) any{
				func(request HistoricalRequest) any {
					raw, _ := request.Variables["myvariableORC"]
					return raw.Any()
				},
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	source := FromHistorical[map[string]any](env, "MyDBHintRow", schema, provider)
	plan, err := env.Build(Select(source,
		Alias("theString", Field[map[string]any, string]("theString")),
		Alias("intPrimitive", Field[map[string]any, int]("intPrimitive")),
	).Query(StatementName("s0-hint-row")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)

	// ORC=10: myint=10, mynumeric=5000 (non-null) -> RowConverter -> ">10<", 99010
	if err := engine.SetVariable(context.Background(), "myvariableORC", 10); err != nil {
		t.Fatal(err)
	}
	res, err := engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results()) != 1 {
		t.Fatalf("expected 1 result for ORC=10, got %d", len(res.Results()))
	}
	if res.Results()[0].Get("theString").Any() != ">10<" || res.Results()[0].Get("intPrimitive").Any() != 99010 {
		t.Fatalf("expected >10</99010, got %#v", res.Results()[0])
	}

	// ORC=60: myint=60, mynumeric=200 -> ">60<", 99060
	if err := engine.SetVariable(context.Background(), "myvariableORC", 60); err != nil {
		t.Fatal(err)
	}
	res, err = engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results()) != 1 {
		t.Fatalf("expected 1 result for ORC=60, got %d", len(res.Results()))
	}
	if res.Results()[0].Get("theString").Any() != ">60<" || res.Results()[0].Get("intPrimitive").Any() != 99060 {
		t.Fatalf("expected >60</99060, got %#v", res.Results()[0])
	}

	// ORC=90: myint=90, mynumeric=nil -> RowConverter returns nil -> no results
	if err := engine.SetVariable(context.Background(), "myvariableORC", 90); err != nil {
		t.Fatal(err)
	}
	res, err = engine.ExecuteFireAndForget(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results()) != 0 {
		t.Fatalf("expected 0 results for ORC=90 (null mynumeric filtered), got %d", len(res.Results()))
	}
}
