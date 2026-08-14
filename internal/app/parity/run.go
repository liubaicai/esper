// Package parity implements the parity scenario runner command.
package parity

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/internal/compat"
)

type trade struct {
	Symbol string  `esper:"symbol"`
	Price  float64 `esper:"price"`
}

// Run executes the parity command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("parity", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("scenario", "testdata/parity/stage1-length-window.json", "scenario JSON file")
	mode := flags.String("mode", "stage1", "runner mode: stage1, context-hash, context-hash-diff, filter-window-aggregate, filter-window-aggregate-diff, join-length-window, join-length-window-diff, output-policy, output-policy-diff, pattern-timer, pattern-timer-diff, subquery, subquery-diff, named-window-mutation, named-window-mutation-diff, table-mutation or table-mutation-diff")
	javaTracePath := flags.String("java-trace", "", "Java trace JSON for context-hash-diff")
	evidencePath := flags.String("evidence", "", "write differential evidence JSON to this path")
	javaCommit := flags.String("java-commit", contextHashJavaCommit, "Java oracle commit for differential evidence")
	javaRuntimeIDs := flags.String("java-runtime-ids", "", "comma-separated Java runtime IDs")
	javaSourceFiles := flags.String("java-source-files", "", "comma-separated Java source files")
	javaExecutions := flags.String("java-executions", "", "comma-separated Java execution names")
	if err := flags.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	file, err := os.Open(*path)
	if err != nil {
		return fail(stderr, err)
	}
	defer file.Close()
	scenario, err := compat.LoadScenario(file)
	if err != nil {
		return fail(stderr, err)
	}
	if *mode == "context-hash" || *mode == "context-hash-diff" {
		trace, err := runContextHashScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-hash-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextHashJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{contextHashJavaSource}),
				splitMetadata(*javaExecutions, contextHashJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "filter-window-aggregate" || *mode == "filter-window-aggregate-diff" {
		trace, err := runFilterWindowAggregateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "filter-window-aggregate-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, filterWindowAggregateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, filterWindowAggregateJavaSources),
				splitMetadata(*javaExecutions, filterWindowAggregateJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "join-length-window" || *mode == "join-length-window-diff" {
		trace, err := runJoinLengthWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "join-length-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, joinLengthWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, joinLengthWindowJavaSources),
				splitMetadata(*javaExecutions, joinLengthWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "output-policy" || *mode == "output-policy-diff" {
		trace, err := runOutputPolicyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "output-policy-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, outputPolicyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, outputPolicyJavaSources),
				splitMetadata(*javaExecutions, outputPolicyJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "pattern-timer" || *mode == "pattern-timer-diff" {
		trace, err := runPatternTimerScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "pattern-timer-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, patternTimerJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, patternTimerJavaSources),
				splitMetadata(*javaExecutions, patternTimerJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subquery" || *mode == "subquery-diff" {
		trace, err := runSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subqueryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subqueryJavaSources),
				splitMetadata(*javaExecutions, subqueryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "named-window-mutation" || *mode == "named-window-mutation-diff" {
		trace, err := runNamedWindowMutationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "named-window-mutation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, namedWindowMutationJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, namedWindowMutationJavaSources),
				splitMetadata(*javaExecutions, namedWindowMutationJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "table-mutation" || *mode == "table-mutation-diff" {
		trace, err := runTableMutationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "table-mutation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, tableMutationJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, tableMutationJavaSources),
				splitMetadata(*javaExecutions, tableMutationJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode != "stage1" {
		return fail(stderr, fmt.Errorf("unsupported parity mode %q", *mode))
	}

	env := esper.NewEnvironment()
	if _, err := esper.RegisterStruct[trade](env, "Trade"); err != nil {
		return fail(stderr, err)
	}
	stream := esper.From[trade](env, "Trade").
		Filter(esper.Greater[float64](esper.Field[trade, float64]("price"), esper.Literal(10.0))).
		Window(esper.LengthWindow(2))
	plan, err := env.Build(stream.Query(esper.StatementName("parity-stage1"), esper.WithOldStream()))
	if err != nil {
		return fail(stderr, err)
	}
	engine := env.NewEngine()
	defer func() { _ = engine.Close(context.Background()) }()
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		return fail(stderr, err)
	}

	trace, err := compat.Replay(context.Background(), engine, deployment.Statements()[0], scenario, func(step compat.Step) (any, error) {
		var value trade
		if err := json.Unmarshal(step.Payload, &value); err != nil {
			return nil, fmt.Errorf("decode Trade: %w", err)
		}
		return value, nil
	})
	if err != nil {
		return fail(stderr, err)
	}
	if err := json.NewEncoder(stdout).Encode(trace); err != nil {
		return fail(stderr, err)
	}
	return 0
}

// runDifferentialMode loads the Java trace, builds canonical differential
// evidence and writes it. It returns 1 when the normalized traces differ.
func runDifferentialMode(stdout, stderr io.Writer, javaTracePath, evidencePath, javaCommit string, runtimeIDs, sourceFiles, executions []string, scenario compat.Scenario, goTrace compat.Trace) int {
	if javaTracePath == "" {
		return fail(stderr, fmt.Errorf("Java trace path is required for differential mode"))
	}
	file, err := os.Open(javaTracePath)
	if err != nil {
		return fail(stderr, err)
	}
	javaTrace, loadErr := compat.LoadTrace(file)
	closeErr := file.Close()
	if loadErr != nil {
		return fail(stderr, loadErr)
	}
	if closeErr != nil {
		return fail(stderr, closeErr)
	}
	evidence, err := compat.NewDifferentialEvidence(javaCommit, runtimeIDs, sourceFiles, executions, scenario, javaTrace, goTrace)
	if err != nil {
		return fail(stderr, err)
	}
	if err := writeJSON(stdout, evidencePath, evidence); err != nil {
		return fail(stderr, err)
	}
	if len(evidence.Differences) != 0 {
		return 1
	}
	return 0
}

func writeJSON(stdout io.Writer, path string, value any) error {
	if path == "" {
		return json.NewEncoder(stdout).Encode(value)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	return 1
}
