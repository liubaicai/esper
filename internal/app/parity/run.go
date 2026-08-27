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
	mode := flags.String("mode", "stage1", "runner mode: stage1, context-hash, context-hash-diff, expr-core-bitwise, expr-core-bitwise-diff, expr-core-logical, expr-core-logical-diff, expr-core-coalesce, expr-core-coalesce-diff, expr-core-relop, expr-core-relop-diff, expr-core-like-regexp, expr-core-like-regexp-diff, expr-core-in-between, expr-core-in-between-diff, expr-core-equals-is, expr-core-equals-is-diff, expr-core-case, expr-core-case-diff, expr-core-instanceof, expr-core-instanceof-diff, expr-core-type-name, expr-core-type-name-diff, expr-core-exists-cast, expr-core-exists-cast-diff, expr-core-current-timestamp, expr-core-current-timestamp-diff, expr-core-current-evaluation-context, expr-core-current-evaluation-context-diff, expr-dt-between, expr-dt-between-diff, filter-window-aggregate, filter-window-aggregate-diff, join-length-window, join-length-window-diff, pattern-timer, pattern-timer-diff, output-policy, output-policy-diff, output-after, output-after-diff, resultset-aggregate-first-ever-last-ever, resultset-aggregate-first-ever-last-ever-diff")
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
	if *mode == "expr-core-bitwise" || *mode == "expr-core-bitwise-diff" {
		trace, err := runExprCoreBitwiseScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-bitwise-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreBitwiseJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreBitwiseJavaSources),
				splitMetadata(*javaExecutions, exprCoreBitwiseJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-logical" || *mode == "expr-core-logical-diff" {
		trace, err := runExprCoreLogicalScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-logical-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreLogicalJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreLogicalJavaSources),
				splitMetadata(*javaExecutions, exprCoreLogicalJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-coalesce" || *mode == "expr-core-coalesce-diff" {
		trace, err := runExprCoreCoalesceScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-coalesce-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreCoalesceJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreCoalesceJavaSources),
				splitMetadata(*javaExecutions, exprCoreCoalesceJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-relop" || *mode == "expr-core-relop-diff" {
		trace, err := runExprCoreRelOpScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-relop-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreRelOpJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreRelOpJavaSources),
				splitMetadata(*javaExecutions, exprCoreRelOpJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-like-regexp" || *mode == "expr-core-like-regexp-diff" {
		trace, err := runExprCoreLikeRegexpScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-like-regexp-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreLikeRegexpJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreLikeRegexpJavaSources),
				splitMetadata(*javaExecutions, exprCoreLikeRegexpJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-in-between" || *mode == "expr-core-in-between-diff" {
		trace, err := runExprCoreInBetweenScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-in-between-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreInBetweenJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreInBetweenJavaSources),
				splitMetadata(*javaExecutions, exprCoreInBetweenJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-equals-is" || *mode == "expr-core-equals-is-diff" {
		trace, err := runExprCoreEqualsIsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-equals-is-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreEqualsIsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreEqualsIsJavaSources),
				splitMetadata(*javaExecutions, exprCoreEqualsIsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-case" || *mode == "expr-core-case-diff" {
		trace, err := runExprCoreCaseScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-case-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreCaseJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreCaseJavaSources),
				splitMetadata(*javaExecutions, exprCoreCaseJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-instanceof" || *mode == "expr-core-instanceof-diff" {
		trace, err := runExprCoreInstanceOfScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-instanceof-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreInstanceOfJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreInstanceOfJavaSources),
				splitMetadata(*javaExecutions, exprCoreInstanceOfJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-type-name" || *mode == "expr-core-type-name-diff" {
		trace, err := runExprCoreTypeNameScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-type-name-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreTypeNameJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreTypeNameJavaSources),
				splitMetadata(*javaExecutions, exprCoreTypeNameJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-exists-cast" || *mode == "expr-core-exists-cast-diff" {
		trace, err := runExprCoreExistsCastScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-exists-cast-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreExistsCastJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreExistsCastJavaSources),
				splitMetadata(*javaExecutions, exprCoreExistsCastJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-current-timestamp" || *mode == "expr-core-current-timestamp-diff" {
		trace, err := runExprCoreCurrentTimestampScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-current-timestamp-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreCurrentTimestampJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreCurrentTimestampJavaSources),
				splitMetadata(*javaExecutions, exprCoreCurrentTimestampJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-core-current-evaluation-context" || *mode == "expr-core-current-evaluation-context-diff" {
		trace, err := runExprCoreCurrentEvaluationContextScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-core-current-evaluation-context-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprCoreCurrentEvaluationContextJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprCoreCurrentEvaluationContextJavaSources),
				splitMetadata(*javaExecutions, exprCoreCurrentEvaluationContextJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-between" || *mode == "expr-dt-between-diff" {
		trace, err := runExprDTBetweenScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-between-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTBetweenJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTBetweenJavaSources),
				splitMetadata(*javaExecutions, exprDTBetweenJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
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
	if *mode == "epl-insert-into-transpose-pattern" || *mode == "epl-insert-into-transpose-pattern-diff" {
		trace, err := runTransposePatternScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-transpose-pattern-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, transposePatternJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, transposePatternJavaSources),
				splitMetadata(*javaExecutions, transposePatternJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-insert-into-populate-und-stream-select" || *mode == "epl-insert-into-populate-und-stream-select-diff" {
		trace, err := runEplInsertIntoPopulateUndStreamSelectScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-populate-und-stream-select-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoPopulateUndStreamSelectJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoPopulateUndStreamSelectJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoPopulateUndStreamSelectJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-insert-into-eventcol-rest" || *mode == "epl-insert-into-eventcol-rest-diff" {
		trace, err := runEplInsertIntoEventColRestScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-eventcol-rest-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoEventColRestJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoEventColRestJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoEventColRestJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-faf-scene-two" || *mode == "infra-faf-scene-two-diff" {
		trace, err := runInfraFAFSceneTwoScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-faf-scene-two-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraFAFSceneTwoJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraFAFSceneTwoJavaSources),
				splitMetadata(*javaExecutions, infraFAFSceneTwoJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subquery" || *mode == "infra-nwtable-subquery-diff" {
		trace, err := runInfraNWTableSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqueryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqueryJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqueryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subquery-delete-aggregate" || *mode == "infra-nwtable-subquery-delete-aggregate-diff" {
		trace, err := runInfraNWTableSubqueryDeleteAggregateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subquery-delete-aggregate-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqueryDeleteAggregateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqueryDeleteAggregateJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqueryDeleteAggregateJavaExecutions), scenario, trace)
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
	if *mode == "variable-deploy" || *mode == "variable-deploy-diff" {
		trace, err := runVariableDeployScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "variable-deploy-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, variableDeployJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, variableDeployJavaSources),
				splitMetadata(*javaExecutions, variableDeployJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-output" || *mode == "context-output-diff" {
		trace, err := runContextOutputScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-output-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextOutputJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextOutputJavaSources),
				splitMetadata(*javaExecutions, contextOutputJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "deployment-restart" || *mode == "deployment-restart-diff" {
		trace, err := runDeploymentRestartScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "deployment-restart-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, deploymentRestartJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, deploymentRestartJavaSources),
				splitMetadata(*javaExecutions, deploymentRestartJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "high-cardinality" || *mode == "high-cardinality-diff" {
		trace, err := runHighCardinalityScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "high-cardinality-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, highCardinalityJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, highCardinalityJavaSources),
				splitMetadata(*javaExecutions, highCardinalityJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "time-window" || *mode == "time-window-diff" {
		trace, err := runTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, timeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, timeWindowJavaSources),
				splitMetadata(*javaExecutions, timeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-connector" || *mode == "dataflow-connector-diff" {
		trace, err := runDataflowConnectorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-connector-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowConnectorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowConnectorJavaSources),
				splitMetadata(*javaExecutions, dataflowConnectorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "output-after" || *mode == "output-after-diff" {
		trace, err := runOutputAfterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "output-after-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, outputAfterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, outputAfterJavaSources),
				splitMetadata(*javaExecutions, outputAfterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup" || *mode == "rollup-diff" {
		trace, err := runRollupScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupJavaSources),
				splitMetadata(*javaExecutions, rollupJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-every-sorted" || *mode == "rollup-output-every-sorted-diff" {
		trace, err := runRollupOutputEverySortedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-every-sorted-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputEverySortedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputEverySortedJavaSources),
				splitMetadata(*javaExecutions, rollupOutputEverySortedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-last" || *mode == "rollup-output-last-diff" {
		trace, err := runRollupOutputLastScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-last-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputLastJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputLastJavaSources),
				splitMetadata(*javaExecutions, rollupOutputLastJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-last-sorted" || *mode == "rollup-output-last-sorted-diff" {
		trace, err := runRollupOutputLastSortedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-last-sorted-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputLastSortedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputLastSortedJavaSources),
				splitMetadata(*javaExecutions, rollupOutputLastSortedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-first" || *mode == "rollup-output-first-diff" {
		trace, err := runRollupOutputFirstScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-first-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputFirstJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputFirstJavaSources),
				splitMetadata(*javaExecutions, rollupOutputFirstJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-first-sorted" || *mode == "rollup-output-first-sorted-diff" {
		trace, err := runRollupOutputFirstSortedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-first-sorted-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputFirstSortedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputFirstSortedJavaSources),
				splitMetadata(*javaExecutions, rollupOutputFirstSortedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-snapshot-order-limit" || *mode == "rollup-output-snapshot-order-limit-diff" {
		trace, err := runRollupOutputSnapshotOrderLimitScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-snapshot-order-limit-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputSnapshotOrderLimitJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputSnapshotOrderLimitJavaSources),
				splitMetadata(*javaExecutions, rollupOutputSnapshotOrderLimitJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-snapshot" || *mode == "rollup-output-snapshot-diff" {
		trace, err := runRollupOutputSnapshotScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-snapshot-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputSnapshotJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputSnapshotJavaSources),
				splitMetadata(*javaExecutions, rollupOutputSnapshotJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-last-market" || *mode == "rollup-output-last-market-diff" {
		trace, err := runRollupOutputLastMarketScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-last-market-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputLastMarketJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputLastMarketJavaSources),
				splitMetadata(*javaExecutions, rollupOutputLastMarketJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-first-market" || *mode == "rollup-output-first-market-diff" {
		trace, err := runRollupOutputFirstMarketScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-first-market-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputFirstMarketJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputFirstMarketJavaSources),
				splitMetadata(*javaExecutions, rollupOutputFirstMarketJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-no-limit-market" || *mode == "rollup-output-no-limit-market-diff" {
		trace, err := runRollupOutputNoLimitMarketScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-no-limit-market-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputNoLimitMarketJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputNoLimitMarketJavaSources),
				splitMetadata(*javaExecutions, rollupOutputNoLimitMarketJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-default-market" || *mode == "rollup-output-default-market-diff" {
		trace, err := runRollupOutputDefaultMarketScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-default-market-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputDefaultMarketJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputDefaultMarketJavaSources),
				splitMetadata(*javaExecutions, rollupOutputDefaultMarketJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-all" || *mode == "rollup-output-all-diff" {
		trace, err := runRollupOutputAllScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-all-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputAllJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputAllJavaSources),
				splitMetadata(*javaExecutions, rollupOutputAllJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-all-sorted" || *mode == "rollup-output-all-sorted-diff" {
		trace, err := runRollupOutputAllSortedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-all-sorted-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputAllSortedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputAllSortedJavaSources),
				splitMetadata(*javaExecutions, rollupOutputAllSortedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-output-first-having" || *mode == "rollup-output-first-having-diff" {
		trace, err := runRollupOutputFirstHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-output-first-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupOutputFirstHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupOutputFirstHavingJavaSources),
				splitMetadata(*javaExecutions, rollupOutputFirstHavingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-first-ever-last-ever" || *mode == "resultset-aggregate-first-ever-last-ever-diff" {
		trace, err := runResultSetAggregateFirstEverLastEverScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-first-ever-last-ever-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFirstEverLastEverJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFirstEverLastEverJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFirstEverLastEverJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-default" || *mode == "resultset-aggregate-default-diff" {
		trace, err := runResultSetAggregateDefaultScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-default-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateDefaultJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateDefaultJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateDefaultJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-last" || *mode == "resultset-aggregate-last-diff" {
		trace, err := runResultSetAggregateLastScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-last-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateLastJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateLastJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateLastJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-no-output" || *mode == "resultset-aggregate-no-output-diff" {
		trace, err := runResultSetAggregateNoOutputScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-no-output-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateNoOutputJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateNoOutputJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateNoOutputJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "match-recognize" || *mode == "match-recognize-diff" {
		trace, err := runMatchRecognizeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "match-recognize-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, matchRecognizeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, matchRecognizeJavaSources),
				splitMetadata(*javaExecutions, matchRecognizeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "unidirectional-join" || *mode == "unidirectional-join-diff" {
		trace, err := runUnidirectionalJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "unidirectional-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, unidirectionalJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, unidirectionalJoinJavaSources),
				splitMetadata(*javaExecutions, unidirectionalJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "output-first-having" || *mode == "output-first-having-diff" {
		trace, err := runOutputFirstHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "output-first-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, outputFirstHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, outputFirstHavingJavaSources),
				splitMetadata(*javaExecutions, outputFirstHavingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-keyed-subquery" || *mode == "context-keyed-subquery-diff" {
		trace, err := runContextKeyedSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-keyed-subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeyedSubqueryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeyedSubqueryJavaSources),
				splitMetadata(*javaExecutions, contextKeyedSubqueryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-aggregation" || *mode == "rowrecog-aggregation-diff" {
		trace, err := runRowRecogAggregationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-aggregation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogAggregationJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogAggregationJavaSources),
				splitMetadata(*javaExecutions, rowRecogAggregationJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-grouped-time-window" || *mode == "resultset-grouped-time-window-diff" {
		trace, err := runResultSetGroupedTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-grouped-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetGroupedTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetGroupedTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetGroupedTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-time-window" || *mode == "resultset-aggregate-time-window-diff" {
		trace, err := runResultSetAggregateTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-last-time-window" || *mode == "resultset-aggregate-last-time-window-diff" {
		trace, err := runResultSetAggregateLastTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-last-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateLastTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateLastTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateLastTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-first-time-window" || *mode == "resultset-aggregate-first-time-window-diff" {
		trace, err := runResultSetAggregateFirstTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-first-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFirstTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFirstTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFirstTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-snapshot-time-window" || *mode == "resultset-aggregate-snapshot-time-window-diff" {
		trace, err := runResultSetAggregateSnapshotTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-snapshot-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateSnapshotTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateSnapshotTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateSnapshotTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-all-events" || *mode == "resultset-aggregate-all-events-diff" {
		trace, err := runResultSetAggregateAllEventsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-all-events-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateAllEventsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateAllEventsJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateAllEventsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-having-every-events" || *mode == "resultset-having-every-events-diff" {
		trace, err := runResultSetHavingEveryEventsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-having-every-events-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetHavingEveryEventsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetHavingEveryEventsJavaSources),
				splitMetadata(*javaExecutions, resultsetHavingEveryEventsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-max-time-window" || *mode == "resultset-aggregate-max-time-window-diff" {
		trace, err := runResultSetAggregateMaxTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-max-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateMaxTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateMaxTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateMaxTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-join" || *mode == "resultset-aggregate-join-diff" {
		trace, err := runResultSetAggregateJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateJoinJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-all-time-window" || *mode == "resultset-aggregate-all-time-window-diff" {
		trace, err := runResultSetAggregateAllTimeWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-all-time-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateAllTimeWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateAllTimeWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateAllTimeWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-all-having" || *mode == "resultset-aggregate-all-having-diff" {
		trace, err := runResultSetAggregateAllHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-all-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateAllHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateAllHavingJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateAllHavingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-join-events" || *mode == "resultset-aggregate-join-events-diff" {
		trace, err := runResultSetAggregateJoinEventsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-join-events-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateJoinEventsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateJoinEventsJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateJoinEventsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-join-sort-window" || *mode == "resultset-aggregate-join-sort-window-diff" {
		trace, err := runResultSetAggregateJoinSortWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-join-sort-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateJoinSortWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateJoinSortWindowJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateJoinSortWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-multikey" || *mode == "resultset-aggregate-multikey-diff" {
		trace, err := runResultSetAggregateMultikeyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-multikey-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateMultikeyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateMultikeyJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateMultikeyJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-group-output" || *mode == "resultset-aggregate-group-output-diff" {
		trace, err := runResultSetAggregateGroupOutputScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-group-output-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateGroupOutputJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateGroupOutputJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateGroupOutputJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-start-end-correlated" || *mode == "context-start-end-correlated-diff" {
		trace, err := runContextStartEndCorrelatedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-start-end-correlated-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextStartEndJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextStartEndJavaSources),
				splitMetadata(*javaExecutions, contextStartEndJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-duration" || *mode == "context-init-term-duration-diff" {
		trace, err := runContextInitTermDurationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-duration-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermDurationJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermDurationJavaSources),
				splitMetadata(*javaExecutions, contextInitTermDurationJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-inclusive-equals" || *mode == "context-init-term-inclusive-equals-diff" {
		trace, err := runContextInitTermInclusiveEqualsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-inclusive-equals-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermInclusiveJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermInclusiveJavaSources),
				splitMetadata(*javaExecutions, contextInitTermInclusiveJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-partition-selection" || *mode == "context-init-term-partition-selection-diff" {
		trace, err := runContextInitTermPartitionSelectionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-partition-selection-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermPartitionSelectionJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermPartitionSelectionJavaSources),
				splitMetadata(*javaExecutions, contextInitTermPartitionSelectionJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-prev-prior" || *mode == "context-init-term-prev-prior-diff" {
		trace, err := runContextInitTermPrevPriorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-prev-prior-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermPrevPriorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermPrevPriorJavaSources),
				splitMetadata(*javaExecutions, contextInitTermPrevPriorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-view" || *mode == "context-key-segmented-view-diff" {
		trace, err := runContextKeySegmentedViewScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-view-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedViewJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedViewJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedViewJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-pattern-filter" || *mode == "context-key-segmented-pattern-filter-diff" {
		trace, err := runContextKeySegmentedPatternFilterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-pattern-filter-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedPatternFilterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedPatternFilterJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedPatternFilterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-join-remove-stream" || *mode == "context-key-segmented-join-remove-stream-diff" {
		trace, err := runContextKeySegmentedJoinRemoveStreamScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-join-remove-stream-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedJoinRemoveStreamJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedJoinRemoveStreamJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedJoinRemoveStreamJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-join-where-clause-on-partition-key" || *mode == "context-key-segmented-join-where-clause-on-partition-key-diff" {
		trace, err := runContextKeySegmentedJoinWhereClauseOnPartitionKeyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-join-where-clause-on-partition-key-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedJoinWhereClauseOnPartitionKeyJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-pattern-scene-two" || *mode == "context-key-segmented-pattern-scene-two-diff" {
		trace, err := runContextKeySegmentedPatternSceneTwoScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-pattern-scene-two-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedPatternSceneTwoJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedPatternSceneTwoJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedPatternSceneTwoJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-join-multitype-multifield" || *mode == "context-key-segmented-join-multitype-multifield-diff" {
		trace, err := runContextKeySegmentedJoinMultitypeMultifieldScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-join-multitype-multifield-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedJoinMultitypeMultifieldJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedJoinMultitypeMultifieldJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedJoinMultitypeMultifieldJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-multi-statement-filter-count" || *mode == "context-key-segmented-multi-statement-filter-count-diff" {
		trace, err := runContextKeySegmentedMultiStatementFilterCountScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-multi-statement-filter-count-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedMultiStatementFilterCountJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedMultiStatementFilterCountJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedMultiStatementFilterCountJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-additional-filters" || *mode == "context-key-segmented-additional-filters-diff" {
		trace, err := runContextKeySegmentedAdditionalFiltersScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-additional-filters-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedAdditionalFiltersJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedAdditionalFiltersJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedAdditionalFiltersJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-join" || *mode == "context-key-segmented-join-diff" {
		trace, err := runContextKeySegmentedJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedJoinJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-selector" || *mode == "context-key-segmented-selector-diff" {
		trace, err := runContextKeySegmentedSelectorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-selector-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedSelectorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedSelectorJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedSelectorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-prior" || *mode == "context-key-segmented-prior-diff" {
		trace, err := runContextKeySegmentedPriorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-prior-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedPriorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedPriorJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedPriorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-pattern" || *mode == "context-key-segmented-pattern-diff" {
		trace, err := runContextKeySegmentedPatternScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-pattern-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedPatternJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedPatternJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedPatternJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-null-keys" || *mode == "context-key-segmented-null-keys-diff" {
		trace, err := runContextKeySegmentedNullKeysScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-null-keys-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedNullKeysJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedNullKeysJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedNullKeysJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-match-recognize" || *mode == "context-key-segmented-match-recognize-diff" {
		trace, err := runContextKeySegmentedMatchRecognizeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-match-recognize-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedMatchRecognizeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedMatchRecognizeJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedMatchRecognizeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-multikey-w-array-two-field" || *mode == "context-key-segmented-multikey-w-array-two-field-diff" {
		trace, err := runContextKeySegmentedMultikeyWArrayTwoFieldScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-multikey-w-array-two-field-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedMultikeyWArrayTwoFieldJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedMultikeyWArrayTwoFieldJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedMultikeyWArrayTwoFieldJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-multikey-w-array-of-primitive" || *mode == "context-key-segmented-multikey-w-array-of-primitive-diff" {
		trace, err := runContextKeySegmentedMultikeyWArrayOfPrimitiveScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-multikey-w-array-of-primitive-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedMultikeyWArrayOfPrimitiveJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedMultikeyWArrayOfPrimitiveJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedMultikeyWArrayOfPrimitiveJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-w-init-term-pattern-as-name" || *mode == "context-key-segmented-w-init-term-pattern-as-name-diff" {
		trace, err := runContextKeySegmentedWInitTermPatternAsNameScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-w-init-term-pattern-as-name-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedWInitTermPatternAsNameJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedWInitTermPatternAsNameJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedWInitTermPatternAsNameJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-term-event-select" || *mode == "context-key-segmented-term-event-select-diff" {
		trace, err := runContextKeySegmentedTermEventSelectScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-term-event-select-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedTermEventSelectJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedTermEventSelectJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedTermEventSelectJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-w-init-term-end-event" || *mode == "context-key-segmented-w-init-term-end-event-diff" {
		trace, err := runContextKeySegmentedWInitTermEndEventScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-w-init-term-end-event-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedWInitTermEndEventJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedWInitTermEndEventJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedWInitTermEndEventJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-term-by-filter" || *mode == "context-key-segmented-term-by-filter-diff" {
		trace, err := runContextKeySegmentedTermByFilterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-term-by-filter-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedTermByFilterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedTermByFilterJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedTermByFilterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-output-clause" || *mode == "context-init-term-output-clause-diff" {
		trace, err := runContextInitTermOutputClauseScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-output-clause-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermOutputJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermOutputJavaSources),
				splitMetadata(*javaExecutions, contextInitTermOutputJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-filter-operators" || *mode == "context-init-term-filter-operators-diff" {
		trace, err := runContextInitTermFilterOperatorsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-filter-operators-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermFilterOperatorsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermFilterOperatorsJavaSources),
				splitMetadata(*javaExecutions, contextInitTermFilterOperatorsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-filter-pattern-end" || *mode == "context-init-term-filter-pattern-end-diff" {
		trace, err := runContextInitTermFilterPatternEndScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-filter-pattern-end-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermFilterPatternJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermFilterPatternJavaSources),
				splitMetadata(*javaExecutions, contextInitTermFilterPatternJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-keyed-aggregation" || *mode == "context-init-term-keyed-aggregation-diff" {
		trace, err := runContextInitTermKeyedAggregationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-keyed-aggregation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermKeyedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermKeyedJavaSources),
				splitMetadata(*javaExecutions, contextInitTermKeyedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-overlap-duration" || *mode == "context-init-term-overlap-duration-diff" {
		trace, err := runContextInitTermOverlapDurationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-overlap-duration-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermOverlapJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermOverlapJavaSources),
				splitMetadata(*javaExecutions, contextInitTermOverlapJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-with-now" || *mode == "context-init-term-with-now-diff" {
		trace, err := runContextInitTermWithNowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-with-now-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermWithNowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermWithNowJavaSources),
				splitMetadata(*javaExecutions, contextInitTermWithNowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-correlated" || *mode == "context-init-term-correlated-diff" {
		trace, err := runContextInitTermCorrelatedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-correlated-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermCorrelatedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermCorrelatedJavaSources),
				splitMetadata(*javaExecutions, contextInitTermCorrelatedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-init-term-temporal-fixed" || *mode == "context-init-term-temporal-fixed-diff" {
		trace, err := runContextInitTermTemporalFixedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-temporal-fixed-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermJavaSources),
				splitMetadata(*javaExecutions, contextInitTermJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-in" || *mode == "subselect-in-diff" {
		trace, err := runSubselectInScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-in-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectInJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectInJavaSources),
				splitMetadata(*javaExecutions, subselectInJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-unfiltered" || *mode == "subselect-unfiltered-diff" {
		trace, err := runSubselectUnfilteredScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-unfiltered-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectUnfilteredJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectUnfilteredJavaSources),
				splitMetadata(*javaExecutions, subselectUnfilteredJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-multicolumn" || *mode == "subselect-multicolumn-diff" {
		trace, err := runSubselectMulticolumnScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-multicolumn-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectMulticolumnJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectMulticolumnJavaSources),
				splitMetadata(*javaExecutions, subselectMulticolumnJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-multirow" || *mode == "subselect-multirow-diff" {
		trace, err := runSubselectDirectMultirowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-multirow-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectDirectMultirowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectDirectMultirowJavaSources),
				splitMetadata(*javaExecutions, subselectDirectMultirowJavaExecutions), scenario, trace,
				normalizeSubselectDirectMultirowUnderlyingTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-aggregated-multirow" || *mode == "subselect-aggregated-multirow-diff" {
		trace, err := runSubselectMultirowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-aggregated-multirow-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectMultirowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectMultirowJavaSources),
				splitMetadata(*javaExecutions, subselectMultirowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "orderby-simple" || *mode == "orderby-simple-diff" {
		trace, err := runOrderBySimpleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "orderby-simple-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, orderBySimpleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, orderBySimpleJavaSources),
				splitMetadata(*javaExecutions, orderBySimpleJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-distinct" || *mode == "epl-other-distinct-diff" {
		trace, err := runEplOtherDistinctScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-distinct-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherDistinctJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherDistinctJavaSources),
				splitMetadata(*javaExecutions, eplOtherDistinctJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-join" || *mode == "infra-named-window-join-diff" {
		trace, err := runInfraNamedWindowJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-join-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNamedWindowJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNamedWindowJoinJavaSources),
				splitMetadata(*javaExecutions, infraNamedWindowJoinJavaExecutions), scenario, trace,
				normalizeInfraNamedWindowJoinTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-insert-into" || *mode == "infra-table-insert-into-diff" {
		trace, err := runInfraTableInsertIntoScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-insert-into-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableInsertIntoJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableInsertIntoJavaSources),
				splitMetadata(*javaExecutions, infraTableInsertIntoJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-into-table" || *mode == "infra-table-into-table-diff" {
		trace, err := runInfraTableIntoTableScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-into-table-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableIntoTableJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableIntoTableJavaSources),
				splitMetadata(*javaExecutions, infraTableIntoTableJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-row-per-group" || *mode == "resultset-orderby-row-per-group-diff" {
		trace, err := runResultsetOrderbyRowPerGroupScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-row-per-group-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbyRowPerGroupJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbyRowPerGroupJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbyRowPerGroupJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-faf-join" || *mode == "infra-nwtable-faf-join-diff" {
		trace, err := runInfraNwTableFafJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-faf-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNwTableFafJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNwTableFafJoinJavaSources),
				splitMetadata(*javaExecutions, infraNwTableFafJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-length-batch" || *mode == "view-length-batch-diff" {
		trace, err := runViewLengthBatchScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-length-batch-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewLengthBatchJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewLengthBatchJavaSources),
				splitMetadata(*javaExecutions, viewLengthBatchJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dt-round" || *mode == "dt-round-diff" {
		trace, err := runExprDTRoundScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dt-round-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTRoundJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTRoundJavaSources),
				splitMetadata(*javaExecutions, exprDTRoundJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-map-core" || *mode == "event-map-core-diff" {
		trace, err := runEventMapCoreScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-map-core-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eventMapCoreJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventMapCoreJavaSources),
				splitMetadata(*javaExecutions, eventMapCoreJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-query-type-aggregate-grouped" || *mode == "resultset-query-type-aggregate-grouped-diff" {
		trace, err := runResultSetQueryTypeAggregateGroupedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-query-type-aggregate-grouped-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeAggregateGroupedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeAggregateGroupedJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeAggregateGroupedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-row-for-all" || *mode == "resultset-orderby-row-for-all-diff" {
		trace, err := runResultSetOrderByRowForAllScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-row-for-all-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetOrderByRowForAllJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetOrderByRowForAllJavaSources),
				splitMetadata(*javaExecutions, resultSetOrderByRowForAllJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-w-time-batch" || *mode == "resultset-querytype-w-time-batch-diff" {
		trace, err := runResultSetQueryTypeWTimeBatchScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-w-time-batch-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeWTimeBatchJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeWTimeBatchJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeWTimeBatchJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-query-type-having" || *mode == "resultset-query-type-having-diff" {
		trace, err := runResultSetQueryTypeHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-query-type-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetQueryTypeHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeHavingJavaSources),
				splitMetadata(*javaExecutions, resultsetQueryTypeHavingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-wildcard-additional" || *mode == "epl-other-wildcard-additional-diff" {
		trace, err := runEplOtherWildcardAdditionalScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-wildcard-additional-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherWildcardAdditionalJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherWildcardAdditionalJavaSources),
				splitMetadata(*javaExecutions, eplOtherWildcardAdditionalJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-class-static-method" || *mode == "expr-class-static-method-diff" {
		trace, err := runEcsmScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-class-static-method-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, ecsmJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, ecsmJavaSources),
				splitMetadata(*javaExecutions, ecsmJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-bean-property-fragment" || *mode == "event-bean-property-fragment-diff" {
		trace, err := runEbprScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-bean-property-fragment-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, ebprJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, ebprJavaSources),
				splitMetadata(*javaExecutions, ebprJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-filter-optimizable-value-limited" || *mode == "expr-filter-optimizable-value-limited-diff" {
		trace, err := runEfovScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-filter-optimizable-value-limited-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, efovJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, efovJavaSources),
				splitMetadata(*javaExecutions, efovJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-filter-optimizable" || *mode == "expr-filter-optimizable-diff" {
		trace, err := runEfoScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-filter-optimizable-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, efoJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, efoJavaSources),
				splitMetadata(*javaExecutions, efoJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-unique" || *mode == "view-unique-diff" {
		trace, err := runViewUniqueScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-unique-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewUniqueJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewUniqueJavaSources),
				splitMetadata(*javaExecutions, viewUniqueJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "variables-onset" || *mode == "variables-onset-diff" {
		trace, err := runVariablesOnsetScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "variables-onset-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, variablesOnsetJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, variablesOnsetJavaSources),
				splitMetadata(*javaExecutions, variablesOnsetJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "variables-use" || *mode == "variables-use-diff" {
		trace, err := runVariablesUseScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "variables-use-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, variablesUseJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, variablesUseJavaSources),
				splitMetadata(*javaExecutions, variablesUseJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "stream-selector" || *mode == "stream-selector-diff" {
		trace, err := runStreamSelectorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "stream-selector-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, streamSelectorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, streamSelectorJavaSources),
				splitMetadata(*javaExecutions, streamSelectorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-dimensionality" || *mode == "rollup-dimensionality-diff" {
		trace, err := runRollupDimensionalityScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-dimensionality-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rollupDimensionalityJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupDimensionalityJavaSources),
				splitMetadata(*javaExecutions, rollupDimensionalityJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-filtered" || *mode == "subselect-filtered-diff" {
		trace, err := runSubselectFilteredScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-filtered-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectFilteredJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectFilteredJavaSources),
				splitMetadata(*javaExecutions, subselectFilteredJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-quantified" || *mode == "subselect-quantified-diff" {
		trace, err := runSubselectQuantifiedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-quantified-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectQuantifiedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectQuantifiedJavaSources),
				splitMetadata(*javaExecutions, subselectQuantifiedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-aggregated-single-value" || *mode == "subselect-aggregated-single-value-diff" {
		trace, err := runSubselectAggregatedSingleValueScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-aggregated-single-value-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectSingleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectSingleJavaSources),
				splitMetadata(*javaExecutions, subselectSingleJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "subselect-aggregated-in-exists-any-all" || *mode == "subselect-aggregated-in-exists-any-all-diff" {
		trace, err := runSubselectAggregatedInExistsAnyAllScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "subselect-aggregated-in-exists-any-all-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, subselectAggregatedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, subselectAggregatedJavaSources),
				splitMetadata(*javaExecutions, subselectAggregatedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-filtered" || *mode == "resultset-aggregate-filtered-diff" {
		trace, err := runResultSetAggregateFilteredScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-filtered-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFilteredJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFilteredJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFilteredJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-filtered-all" || *mode == "resultset-aggregate-filtered-all-diff" {
		trace, err := runResultSetAggregateFilteredAllScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-filtered-all-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFilteredAllJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFilteredAllJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFilteredAllJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-count-sum" || *mode == "resultset-aggregate-count-sum-diff" {
		trace, err := runResultSetAggregateCountSumScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-count-sum-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateCountSumJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateCountSumJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateCountSumJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-limit-snapshot" || *mode == "resultset-aggregate-limit-snapshot-diff" {
		trace, err := runResultSetAggregateLimitSnapshotScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-limit-snapshot-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateLimitSnapshotJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateLimitSnapshotJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateLimitSnapshotJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-row-per-group-simple" || *mode == "resultset-row-per-group-simple-diff" {
		trace, err := runResultSetRowPerGroupSimpleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-row-per-group-simple-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetRowPerGroupSimpleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetRowPerGroupSimpleJavaSources),
				splitMetadata(*javaExecutions, resultsetRowPerGroupSimpleJavaExecutions), scenario, trace)
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

// runDifferentialMode loads the Java trace, builds canonical differential evidence, and writes it. It returns 1 when the normalized traces differ.
func runDifferentialMode(stdout, stderr io.Writer, javaTracePath, evidencePath, javaCommit string, runtimeIDs, sourceFiles, executions []string, scenario compat.Scenario, goTrace compat.Trace) int {
	return runDifferentialModeWithNormalizer(stdout, stderr, javaTracePath, evidencePath, javaCommit,
		runtimeIDs, sourceFiles, executions, scenario, goTrace, nil)
}

func runDifferentialModeWithNormalizer(stdout, stderr io.Writer, javaTracePath, evidencePath, javaCommit string, runtimeIDs, sourceFiles, executions []string, scenario compat.Scenario, goTrace compat.Trace, normalize func(compat.Trace) compat.Trace) int {
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
	if normalize != nil {
		javaTrace = normalize(javaTrace)
		goTrace = normalize(goTrace)
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
