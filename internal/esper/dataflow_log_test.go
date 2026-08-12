package esper

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDataflowLogSinkStructuredFormatsAndLayoutMatchEsper(t *testing.T) {
	tests := []struct {
		name   string
		format DataflowLogFormat
		check  func(*testing.T, string)
	}{
		{
			name:   "summary",
			format: DataflowLogSummary,
			check: func(t *testing.T, line string) {
				if !strings.Contains(line, "Trade[") || !strings.Contains(line, "A") {
					t.Fatalf("summary log line = %q", line)
				}
			},
		},
		{
			name:   "json",
			format: DataflowLogJSON,
			check: func(t *testing.T, line string) {
				var decoded map[string]any
				if err := json.Unmarshal([]byte(line), &decoded); err != nil {
					t.Fatalf("JSON log line %q: %v", line, err)
				}
				if decoded["Trade"] == nil {
					t.Fatalf("JSON log line = %#v", decoded)
				}
			},
		},
		{
			name:   "xml",
			format: DataflowLogXML,
			check: func(t *testing.T, line string) {
				if !strings.Contains(line, "<?xml") || !strings.Contains(line, "<Trade>") || !strings.Contains(line, "<symbol>A</symbol>") {
					t.Fatalf("XML log line = %q", line)
				}
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			env := NewEnvironment()
			if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
				t.Fatal(err)
			}
			var lines []string
			definition, err := DefineDataflow(env, "log-"+testCase.name).
				EventBusSource("source", "Trade").
				LogSinkWithOptions("sink", DataflowLogSinkOptions{
					Format:      testCase.format,
					Layout:      "%df|%p|%i|%t|%e",
					Title:       "title",
					LineFeed:    false,
					LineFeedSet: true,
					Writer: func(_ context.Context, line string) error {
						lines = append(lines, line)
						return nil
					},
				}).
				Build()
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(env)
			instance, err := engine.InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{InstanceID: "instance-1"})
			if err != nil {
				t.Fatal(err)
			}
			if err := instance.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer instance.Cancel(context.Background())
			if err := engine.Send(context.Background(), "Trade", runtimeTestTrade{Symbol: "A", Price: 12.5}); err != nil {
				t.Fatal(err)
			}
			if len(lines) != 1 {
				t.Fatalf("log lines = %#v", lines)
			}
			if got := lines[0]; !strings.HasPrefix(got, "log-"+testCase.name+"|0|instance-1|title|") || strings.Contains(got, "\n") {
				t.Fatalf("layout log line = %q", got)
			}
			testCase.check(t, strings.TrimPrefix(lines[0], "log-"+testCase.name+"|0|instance-1|title|"))
		})
	}
}

func TestDataflowLogSinkRendersProjectionRowsAndDefaultsLineFeed(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	var lines []string
	definition, err := DefineDataflow(env, "log-row").
		EventBusSource("source", "Trade").
		Select("select",
			Alias("symbol", Field[runtimeTestTrade, string]("symbol")),
			Alias("price", Field[runtimeTestTrade, float64]("price")),
		).
		LogSinkWithOptions("sink", DataflowLogSinkOptions{
			Format: DataflowLogJSON,
			Layout: "%e",
			Writer: func(_ context.Context, line string) error {
				lines = append(lines, line)
				return nil
			},
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	instance, err := engine.InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer instance.Cancel(context.Background())
	if err := engine.Send(context.Background(), "Trade", runtimeTestTrade{Symbol: "R", Price: 4}); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "\n") {
		t.Fatalf("row log lines = %#v", lines)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(lines[0], "\n")), &decoded); err != nil {
		t.Fatal(err)
	}
	row, ok := decoded["dataflow:select"].(map[string]any)
	if !ok || row["symbol"] != "R" || row["price"] != float64(4) {
		t.Fatalf("row log value = %#v", decoded)
	}
}

func TestDataflowLogSinkRejectsUnsupportedFormatAndOutputEdges(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[runtimeTestTrade](env, "Trade"); err != nil {
		t.Fatal(err)
	}
	writer := func(context.Context, string) error { return nil }
	if _, err := DefineDataflow(env, "log-invalid-format").
		LogSinkWithOptions("sink", DataflowLogSinkOptions{Format: "yaml", Writer: writer}).
		Build(); err == nil {
		t.Fatal("unsupported LogSink format was accepted")
	}
	if _, err := DefineDataflow(env, "log-linear-output").
		Emitter("source").
		LogSinkWithOptions("sink", DataflowLogSinkOptions{Writer: writer}).
		Emitter("after").
		Build(); err == nil {
		t.Fatal("LogSink with a linear output operator was accepted")
	}
	if _, err := DefineDataflow(env, "log-graph-output").
		Emitter("source").
		LogSinkWithOptions("sink", DataflowLogSinkOptions{Writer: writer}).
		Emitter("after").
		Connect("source", "sink").
		Connect("sink", "after").
		Build(); err == nil {
		t.Fatal("LogSink graph output edge was accepted")
	}
}
