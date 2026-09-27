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
	flags.Usage = func() {
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-median-and-deviation and resultset-aggregate-median-and-deviation-diff, resultset-aggregate-minmax-no-data-window-subquery and resultset-aggregate-minmax-no-data-window-subquery-diff, resultset-aggregate-minmax-named-window-wever and resultset-aggregate-minmax-named-window-wever-diff, resultset-aggregate-minmax-groupby, resultset-aggregate-minmax-groupby-diff, resultset-aggregate-minmax-groupby-om-viewcompile, resultset-aggregate-minmax-groupby-om-viewcompile-diff, resultset-aggregate-minmax-groupby-join-select-having and resultset-aggregate-minmax-groupby-join-select-having-diff, resultset-querytype-row-for-all-select-avg-expr-std-group-by and resultset-querytype-row-for-all-select-avg-expr-std-group-by-diff, resultset-querytype-row-for-all-select-avg-std-group-by-uni and resultset-querytype-row-for-all-select-avg-std-group-by-uni-diff, resultset-querytype-row-for-all-static-method-double-nested and resultset-querytype-row-for-all-static-method-double-nested-diff, resultset-querytype-row-for-all-having-avg-group-window and resultset-querytype-row-for-all-having-avg-group-window-diff, resultset-querytype-row-for-all-having-sum and resultset-querytype-row-for-all-having-sum-diff, resultset-querytype-row-for-all-having-sum-join and resultset-querytype-row-for-all-having-sum-join-diff, resultset-querytype-rollup-having-iterator and resultset-querytype-rollup-having-iterator-diff, rollup-dimensionality and rollup-dimensionality-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-rollup-orderby-unidirectional and resultset-querytype-rollup-orderby-unidirectional-diff, rollup-grouping-funcs-dedicated and rollup-grouping-funcs-dedicated-diff, rollup-grouping-funcs-faf-dedicated and rollup-grouping-funcs-faf-dedicated-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-sorted-no-data-window and resultset-aggregate-sorted-no-data-window-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-window and resultset-aggregate-window-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-sorted-table-access and resultset-aggregate-sorted-table-access-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-sorted-minmax-by-no-alias and resultset-aggregate-sorted-minmax-by-no-alias-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-row-per-group-having and resultset-querytype-row-per-group-having-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-row-per-event and resultset-querytype-row-per-event-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-by and resultset-querytype-local-group-by-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-ungrouped and resultset-querytype-local-group-ungrouped-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-extended and resultset-querytype-local-group-extended-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-grouped and resultset-querytype-local-group-grouped-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-ungrouped-agg and resultset-querytype-local-group-ungrouped-agg-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-row-remove and resultset-querytype-local-group-row-remove-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-closure and resultset-querytype-local-group-closure-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-context-terminated and resultset-querytype-local-group-context-terminated-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-keys and resultset-querytype-local-group-keys-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-local-group-solution-pattern and resultset-querytype-local-group-solution-pattern-diff")
		fmt.Fprintln(stderr, "runner modes include orderby-rowperevent-agg and orderby-rowperevent-agg-diff")
		fmt.Fprintln(stderr, "runner modes include orderby-rowperevent-agg-join and orderby-rowperevent-agg-join-diff")
		fmt.Fprintln(stderr, "runner modes include orderby-rowperevent-iterator and orderby-rowperevent-iterator-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-querytype-aggregate-grouped-having and resultset-querytype-aggregate-grouped-having-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-orderby-aggregate-grouped and resultset-orderby-aggregate-grouped-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-orderby-simple-descending-om and resultset-orderby-simple-descending-om-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-orderby-simple-expressions-aliases and resultset-orderby-simple-expressions-aliases-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-orderby-simple-join-wildcard and resultset-orderby-simple-join-wildcard-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-orderby-simple-no-output-invalid and resultset-orderby-simple-no-output-invalid-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-output-limit-row-limit-context-grouped and resultset-output-limit-row-limit-context-grouped-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-output-limit-row-limit and resultset-output-limit-row-limit-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-output-limit-row-limit-negative-rowcount and resultset-output-limit-row-limit-negative-rowcount-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-output-limit-row-limit-invalid and resultset-output-limit-row-limit-invalid-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-output-limit-row-limit-variable and resultset-output-limit-row-limit-variable-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-filter-named-parameter and resultset-aggregate-filter-named-parameter-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-filtered-w-math-context and resultset-aggregate-filtered-w-math-context-diff")
		fmt.Fprintln(stderr, "runner modes include infra-named-window-insert-shape and infra-named-window-insert-shape-diff")
		fmt.Fprintln(stderr, "runner modes include view-time-win and view-time-win-diff")
		fmt.Fprintln(stderr, "runner modes include view-first-time and view-first-time-diff")
		fmt.Fprintln(stderr, "runner modes include view-length-win-property-detail and view-length-win-property-detail-diff")
		fmt.Fprintln(stderr, "runner modes include view-time-batch and view-time-batch-diff")
		fmt.Fprintln(stderr, "runner modes include view-parameterized-by-context and view-parameterized-by-context-diff")
		fmt.Fprintln(stderr, "runner modes include view-intersect and view-intersect-diff")
		fmt.Fprintln(stderr, "runner modes include infra-named-window-final-views and infra-named-window-final-views-diff")
		fmt.Fprintln(stderr, "runner modes include epl-other-from-clause-optional and epl-other-from-clause-optional-diff")
		fmt.Fprintln(stderr, "runner modes include epl-other-select-expr-stream-selector-remainder and epl-other-select-expr-stream-selector-remainder-diff")
		fmt.Fprintln(stderr, "runner modes include context-key-segmented-subselect-prev-prior and context-key-segmented-subselect-prev-prior-diff")
		fmt.Fprintln(stderr, "runner modes include context-key-segmented-infra-prioritized and context-key-segmented-infra-prioritized-diff")
		fmt.Fprintln(stderr, "runner modes include context-key-segmented-named-window and context-key-segmented-named-window-diff")
		fmt.Fprintln(stderr, "runner modes include context-key-segmented-named-window-subquery and context-key-segmented-named-window-subquery-diff")
		fmt.Fprintln(stderr, "runner modes include context-key-segmented-invalid and context-key-segmented-invalid-diff")
		fmt.Fprintln(stderr, "runner modes include context-key-segmented-allocation-time and context-key-segmented-allocation-time-diff")
		fmt.Fprintln(stderr, "runner modes include context-nested-initterm and context-nested-initterm-diff")
		fmt.Fprintln(stderr, "runner modes include context-lifecycle and context-lifecycle-diff")
		fmt.Fprintln(stderr, "runner modes include context-init-term-prioritized and context-init-term-prioritized-diff")
		fmt.Fprintln(stderr, "runner modes include context-selection-faf and context-selection-faf-diff")
		fmt.Fprintln(stderr, "runner modes include context-selection-faf-nested and context-selection-faf-nested-diff")
		fmt.Fprintln(stderr, "runner modes include view-group-closure and view-group-closure-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-output-limit-crontab-when-closure and resultset-output-limit-crontab-when-closure-diff")
		fmt.Fprintln(stderr, "runner modes include resultset-aggregate-invalid-closure and resultset-aggregate-invalid-closure-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-context and infra-nwtable-context-diff")
		fmt.Fprintln(stderr, "runner modes include epl-contained-event-example and epl-contained-event-example-diff")
		fmt.Fprintln(stderr, "runner modes include epl-other-pattern-event-properties and epl-other-pattern-event-properties-diff")
		fmt.Fprintln(stderr, "runner modes include event-map-properties and event-map-properties-diff")
		fmt.Fprintln(stderr, "runner modes include event-object-array-core and event-object-array-core-diff")
		fmt.Fprintln(stderr, "runner modes include event-infra-getter-dynamic and event-infra-getter-dynamic-diff")
		fmt.Fprintln(stderr, "runner modes include event-infra-getter-nested and event-infra-getter-nested-diff")
		fmt.Fprintln(stderr, "runner modes include event-infra-545 and event-infra-545-diff")
		fmt.Fprintln(stderr, "runner modes include event-render-546 and event-render-546-diff")
		fmt.Fprintln(stderr, "runner modes include event-map-nested-547 and event-map-nested-547-diff")
		fmt.Fprintln(stderr, "runner modes include event-avro-hook-548 and event-avro-hook-548-diff")
		fmt.Fprintln(stderr, "runner modes include event-infra-property-dynamic and event-infra-property-dynamic-diff")
		fmt.Fprintln(stderr, "runner modes include event-infra-property-non-dynamic and event-infra-property-non-dynamic-diff")
		fmt.Fprintln(stderr, "runner modes include epl-other-istream-rstream-keywords and epl-other-istream-rstream-keywords-diff")
		fmt.Fprintln(stderr, "runner modes include epl-other-create-expression and epl-other-create-expression-diff")
		fmt.Fprintln(stderr, "runner modes include epl-other-invalid and epl-other-invalid-diff")
		fmt.Fprintln(stderr, "runner modes include expr-define-value-parameter and expr-define-value-parameter-diff")
		fmt.Fprintln(stderr, "runner modes include expr-filter-opt-lkup-limited-remaining and expr-filter-opt-lkup-limited-remaining-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-interval-ops and expr-dt-interval-ops-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-resolution and expr-dt-resolution-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-data-sources and expr-dt-data-sources-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-with-minmax-549 and expr-dt-with-minmax-549-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-set-nested-550 and expr-dt-set-nested-550-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-format-551 and expr-dt-format-551-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-remainder-552 and expr-dt-remainder-552-diff")
		fmt.Fprintln(stderr, "runner modes include expr-dt-tail-553 and expr-dt-tail-553-diff")
		fmt.Fprintln(stderr, "runner modes include expr-enum-remainder-554 and expr-enum-remainder-554-diff")
		fmt.Fprintln(stderr, "runner modes include expr-define-locreport-555 and expr-define-locreport-555-diff")
		fmt.Fprintln(stderr, "runner modes include expr-script-threading-556 and expr-script-threading-556-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-comparative-557 and infra-nwtable-comparative-557-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-widening-558 and infra-nwtable-widening-558-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-index-faf-559 and infra-nwtable-index-faf-559-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-late-index-560 and infra-nwtable-late-index-560-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-index-ops-561 and infra-nwtable-index-ops-561-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-mrak-562 and infra-nwtable-mrak-562-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-create-ddl-563 and infra-nwtable-create-ddl-563-diff")
		fmt.Fprintln(stderr, "runner modes include infra-namedwindow-om-564 and infra-namedwindow-om-564-diff")
		fmt.Fprintln(stderr, "runner modes include expr-enum-select-from and expr-enum-select-from-diff")
		fmt.Fprintln(stderr, "runner modes include expr-class-type-use and expr-class-type-use-diff")
		fmt.Fprintln(stderr, "runner modes include expr-class-for-epl-objects and expr-class-for-epl-objects-diff")
		fmt.Fprintln(stderr, "runner modes include expr-class-class-dependency and expr-class-class-dependency-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-event-type and infra-nwtable-event-type-diff")
		fmt.Fprintln(stderr, "runner modes include infra-nwtable-on-select-aggregation and infra-nwtable-on-select-aggregation-diff")
		flags.PrintDefaults()
	}
	path := flags.String("scenario", "testdata/parity/stage1-length-window.json", "scenario JSON file")
	mode := flags.String("mode", "stage1", "runner mode")
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
	var scenario compat.Scenario
	if *mode == "resultset-querytype-aggregate-grouped-having" || *mode == "resultset-querytype-aggregate-grouped-having-diff" {
		scenario, err = loadResultsetQueryTypeAggregateGroupedHavingScenario(file)
	} else if *mode == "resultset-orderby-aggregate-grouped" || *mode == "resultset-orderby-aggregate-grouped-diff" {
		scenario, err = loadResultsetOrderbyAggregateGroupedScenario(file)
	} else if *mode == "resultset-orderby-join" || *mode == "resultset-orderby-join-diff" {
		scenario, err = loadResultsetOrderbyJoinScenario(file)
	} else if *mode == "resultset-orderby-multi-delivery" || *mode == "resultset-orderby-multi-delivery-diff" {
		scenario, err = loadResultsetOrderbyMultiDeliveryScenario(file)
	} else if *mode == "resultset-orderby-simple-descending-om" || *mode == "resultset-orderby-simple-descending-om-diff" {
		scenario, err = loadResultsetOrderbySimpleDescendingOMScenario(file)
	} else if *mode == "resultset-orderby-simple-expressions-aliases" || *mode == "resultset-orderby-simple-expressions-aliases-diff" {
		scenario, err = loadResultsetOrderbySimpleExpressionsAliasesScenario(file)
	} else if *mode == "resultset-orderby-simple-join-wildcard" || *mode == "resultset-orderby-simple-join-wildcard-diff" {
		scenario, err = loadResultsetOrderbySimpleJoinWildcardScenario(file)
	} else if *mode == "resultset-orderby-simple-no-output-invalid" || *mode == "resultset-orderby-simple-no-output-invalid-diff" {
		scenario, err = loadResultsetOrderbySimpleNoOutputInvalidScenario(file)
	} else if *mode == "resultset-orderby-self-join" || *mode == "resultset-orderby-self-join-diff" {
		scenario, err = loadResultsetOrderbySelfJoinScenario(file)
	} else if *mode == "output-after-events" || *mode == "output-after-events-diff" {
		scenario, err = loadOutputAfterEventsScenario(file)
	} else if *mode == "resultset-output-limit-insert-into" || *mode == "resultset-output-limit-insert-into-diff" {
		scenario, err = loadResultsetOutputLimitInsertIntoScenario(file)
	} else if *mode == "resultset-output-limit-microsecond-resolution" || *mode == "resultset-output-limit-microsecond-resolution-diff" {
		scenario, err = loadResultsetOutputLimitMicrosecondScenario(file)
	} else if *mode == "resultset-output-limit-parameterized-context" || *mode == "resultset-output-limit-parameterized-context-diff" {
		scenario, err = loadResultsetOutputLimitParameterizedContextScenario(file)
	} else if *mode == "resultset-output-limit-changeset-opt" || *mode == "resultset-output-limit-changeset-opt-diff" {
		scenario, err = loadResultsetOutputLimitChangesetScenario(file)
	} else if *mode == "resultset-outputlimit-simple-none" || *mode == "resultset-outputlimit-simple-none-diff" {
		scenario, err = loadResultSetOutputLimitSimpleNoneScenario(file)
	} else if *mode == "expr-enum-sumof-remainder" || *mode == "expr-enum-sumof-remainder-diff" {
		scenario, err = loadExprEnumSumOfRemainderScenario(file)
	} else if *mode == "expr-enum-select-from" || *mode == "expr-enum-select-from-diff" {
		scenario, err = loadExprEnumSelectFromScenario(file)
	} else if *mode == "expr-class-type-use" || *mode == "expr-class-type-use-diff" {
		scenario, err = loadExprClassTypeUseScenario(file)
	} else if *mode == "expr-class-for-epl-objects" || *mode == "expr-class-for-epl-objects-diff" {
		scenario, err = loadExprClassForEPLObjectsScenario(file)
	} else if *mode == "expr-class-class-dependency" || *mode == "expr-class-class-dependency-diff" {
		scenario, err = loadExprClassClassDependencyScenario(file)
	} else if *mode == "epl-variable-output-rate" || *mode == "epl-variable-output-rate-diff" {
		scenario, err = loadEplVariableOutputRateScenario(file)
	} else if *mode == "epl-subselect-within-filter-having" || *mode == "epl-subselect-within-filter-having-diff" {
		scenario, err = loadEplSubselectWithinFilterHavingScenario(file)
	} else if *mode == "epl-subselect-order-of-eval-index" || *mode == "epl-subselect-order-of-eval-index-diff" {
		scenario, err = loadEplSubselectOrderOfEvalIndexScenario(file)
	} else if *mode == "infra-nwtable-subq-uncorrel" || *mode == "infra-nwtable-subq-uncorrel-diff" {
		scenario, err = loadInfraNWTableSubqUncorrelScenario(file)
	} else if *mode == "infra-nwtable-subq-at-eventbean" || *mode == "infra-nwtable-subq-at-eventbean-diff" {
		scenario, err = loadInfraNWTableSubqAtEventBeanScenario(file)
	} else if *mode == "infra-nwtable-subq-correl-join" || *mode == "infra-nwtable-subq-correl-join-diff" {
		scenario, err = loadInfraNWTableSubqCorrelJoinScenario(file)
	} else if *mode == "infra-nwtable-subq-filtered-correl" || *mode == "infra-nwtable-subq-filtered-correl-diff" {
		scenario, err = loadInfraNWTableSubqFilteredCorrelScenario(file)
	} else if *mode == "epl-other-plan-in-keyword" || *mode == "epl-other-plan-in-keyword-diff" {
		scenario, err = loadEplOtherPlanInKeywordScenario(file)
	} else if *mode == "infra-nwtable-on-delete" || *mode == "infra-nwtable-on-delete-diff" {
		scenario, err = loadInfraNWTableOnDeleteScenario(file)
	} else if *mode == "infra-nwtable-on-select-aggregation" || *mode == "infra-nwtable-on-select-aggregation-diff" {
		scenario, err = loadInfraNWTableOnSelectAggScenario(file)
	} else if *mode == "infra-nwtable-on-update" || *mode == "infra-nwtable-on-update-diff" {
		scenario, err = loadInfraNWTableOnUpdateScenario(file)
	} else if *mode == "infra-nwtable-on-merge" || *mode == "infra-nwtable-on-merge-diff" {
		scenario, err = loadInfraNWTableOnMergeScenario(file)
	} else if *mode == "infra-nwtable-on-merge-nested" || *mode == "infra-nwtable-on-merge-nested-diff" {
		scenario, err = loadInfraNWTableOnMergeNestedScenario(file)
	} else if *mode == "infra-nwtable-on-merge-insertstream" || *mode == "infra-nwtable-on-merge-insertstream-diff" {
		scenario, err = loadInfraNWTableOnMergeInsertStreamScenario(file)
	} else if *mode == "infra-nwtable-on-merge-multiaction" || *mode == "infra-nwtable-on-merge-multiaction-diff" {
		scenario, err = loadInfraNWTableOnMergeMultiactionScenario(file)
	} else if *mode == "infra-nwtable-on-merge-pattern-nowhere" || *mode == "infra-nwtable-on-merge-pattern-nowhere-diff" {
		scenario, err = loadInfraNWTableOnMergePatternNoWhereScenario(file)
	} else if *mode == "infra-nwtable-on-merge-flow-itv" || *mode == "infra-nwtable-on-merge-flow-itv-diff" {
		scenario, err = loadInfraNWTableOnMergeFlowITVScenario(file)
	} else if *mode == "infra-nwtable-on-merge-invalid-insertonly" || *mode == "infra-nwtable-on-merge-invalid-insertonly-diff" {
		scenario, err = loadInfraNWTableOnMergeInvalidInsertOnlyScenario(file)
	} else if *mode == "infra-nwtable-on-merge-insertonly-deletethenupdate" || *mode == "infra-nwtable-on-merge-insertonly-deletethenupdate-diff" {
		scenario, err = loadInfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenario(file)
	} else if *mode == "infra-namedwindow-on-delete-silent" || *mode == "infra-namedwindow-on-delete-silent-diff" {
		scenario, err = loadInfraNWOnDeleteSilentScenario(file)
	} else if *mode == "infra-namedwindow-on-delete-indexes" || *mode == "infra-namedwindow-on-delete-indexes-diff" {
		scenario, err = loadInfraNWOnDeleteIndexesScenario(file)
	} else if *mode == "infra-namedwindow-processing-order" || *mode == "infra-namedwindow-processing-order-diff" {
		scenario, err = loadInfraNWProcessingOrderScenario(file)
	} else if *mode == "infra-namedwindow-consumer" || *mode == "infra-namedwindow-consumer-diff" {
		scenario, err = loadInfraNWConsumerScenario(file)
	} else if *mode == "epl-variables-create" || *mode == "epl-variables-create-diff" {
		scenario, err = loadEplVariablesCreateScenario(file)
	} else if *mode == "context-variables" || *mode == "context-variables-diff" {
		scenario, err = loadContextVariablesScenario(file)
	} else if *mode == "context-declared-expression" || *mode == "context-declared-expression-diff" {
		scenario, err = loadContextDeclaredExpressionScenario(file)
	} else if *mode == "context-admin-listen" || *mode == "context-admin-listen-diff" {
		scenario, err = loadContextAdminListenScenario(file)
	} else if *mode == "context-category" || *mode == "context-category-diff" {
		scenario, err = loadContextCategoryScenario(file)
	} else if *mode == "epl-other-from-clause-optional" || *mode == "epl-other-from-clause-optional-diff" {
		scenario, err = loadEplOtherFromClauseOptionalScenario(file)
	} else if *mode == "epl-other-select-expr-stream-selector-remainder" || *mode == "epl-other-select-expr-stream-selector-remainder-diff" {
		scenario, err = loadEplOtherSelectExprStreamSelectorRemainderScenario(file)
	} else if *mode == "context-key-segmented-subselect-prev-prior" || *mode == "context-key-segmented-subselect-prev-prior-diff" {
		scenario, err = loadContextKeySegmentedSubselectPrevPriorScenario(file)
	} else if *mode == "context-key-segmented-invalid" || *mode == "context-key-segmented-invalid-diff" {
		scenario, err = loadContextKeySegmentedInvalidScenario(file)
	} else if *mode == "expr-dt-interval-ops" || *mode == "expr-dt-interval-ops-diff" {
		scenario, err = loadExprDTIntervalOpsScenario(file)
	} else if *mode == "expr-dt-resolution" || *mode == "expr-dt-resolution-diff" {
		scenario, err = loadExprDTResolutionScenario(file)
	} else if *mode == "expr-dt-data-sources" || *mode == "expr-dt-data-sources-diff" {
		scenario, err = loadExprDTDataSourcesScenario(file)
	} else if *mode == "expr-dt-with-minmax-549" || *mode == "expr-dt-with-minmax-549-diff" {
		scenario, err = loadExprDTWithMinMax549Scenario(file)
	} else if *mode == "expr-dt-set-nested-550" || *mode == "expr-dt-set-nested-550-diff" {
		scenario, err = loadExprDTSetNested550Scenario(file)
	} else if *mode == "expr-dt-format-551" || *mode == "expr-dt-format-551-diff" {
		scenario, err = loadExprDTFormat551Scenario(file)
	} else if *mode == "expr-dt-remainder-552" || *mode == "expr-dt-remainder-552-diff" {
		scenario, err = loadExprDTRemainder552Scenario(file)
	} else if *mode == "expr-dt-tail-553" || *mode == "expr-dt-tail-553-diff" {
		scenario, err = loadExprDTTail553Scenario(file)
	} else if *mode == "expr-enum-remainder-554" || *mode == "expr-enum-remainder-554-diff" {
		scenario, err = loadExprEnumRemainder554Scenario(file)
	} else if *mode == "expr-define-locreport-555" || *mode == "expr-define-locreport-555-diff" {
		scenario, err = loadExprDefineLocReport555Scenario(file)
	} else if *mode == "expr-script-threading-556" || *mode == "expr-script-threading-556-diff" {
		scenario, err = loadExprScriptThreading556Scenario(file)
	} else if *mode == "context-key-segmented-infra-prioritized" || *mode == "context-key-segmented-infra-prioritized-diff" {
		scenario, err = loadContextKeySegmentedInfraPrioritizedScenario(file)
	} else if *mode == "context-key-segmented-named-window" || *mode == "context-key-segmented-named-window-diff" {
		scenario, err = loadContextKeySegmentedNamedWindowScenario(file)
	} else if *mode == "context-key-segmented-named-window-subquery" || *mode == "context-key-segmented-named-window-subquery-diff" {
		scenario, err = loadContextKeySegmentedNamedWindowSubqueryScenario(file)
	} else if *mode == "context-init-term-prioritized" || *mode == "context-init-term-prioritized-diff" {
		scenario, err = loadContextInitTermPrioritizedScenario(file)
	} else if *mode == "context-lifecycle" || *mode == "context-lifecycle-diff" {
		scenario, err = loadContextLifecycleScenario(file)
	} else if *mode == "context-selection-faf" || *mode == "context-selection-faf-diff" {
		scenario, err = loadContextSelectionFAFScenario(file)
	} else if *mode == "context-selection-faf-nested" || *mode == "context-selection-faf-nested-diff" {
		scenario, err = loadContextSelectionFAFNestedScenario(file)
	} else if *mode == "epl-variables-event-typed" || *mode == "epl-variables-event-typed-diff" {
		scenario, err = loadEplVariablesEventTypedScenario(file)
	} else if *mode == "epl-as-keyword-backtick" || *mode == "epl-as-keyword-backtick-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "epl-from-clause-method-variable" || *mode == "epl-from-clause-method-variable-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-types" || *mode == "dataflow-types-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-op-lifecycle" || *mode == "dataflow-op-lifecycle-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-create-start-stop-destroy" || *mode == "dataflow-create-start-stop-destroy-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-select-flows" || *mode == "dataflow-select-flows-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-select-state" || *mode == "dataflow-select-state-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-select-representation" || *mode == "dataflow-select-representation-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-epstatement-source" || *mode == "dataflow-epstatement-source-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-beacon-source" || *mode == "dataflow-beacon-source-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-eventbus-source" || *mode == "dataflow-eventbus-source-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-eventbus-sink" || *mode == "dataflow-eventbus-sink-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-captive-lifecycle" || *mode == "dataflow-captive-lifecycle-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-ports-feedback" || *mode == "dataflow-ports-feedback-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-exceptions" || *mode == "dataflow-exceptions-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-lifecycle-core" || *mode == "dataflow-lifecycle-core-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-lifecycle-cancel-join" || *mode == "dataflow-lifecycle-cancel-join-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-lifecycle-blocking" || *mode == "dataflow-lifecycle-blocking-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "epl-database-join" || *mode == "epl-database-join-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "epl-database-join-2" || *mode == "epl-database-join-2-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "event-json-sender-getter" || *mode == "event-json-sender-getter-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "event-json-adapter" || *mode == "event-json-adapter-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "epl-database-restart" || *mode == "epl-database-restart-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "epl-database-timebatch" || *mode == "epl-database-timebatch-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "dataflow-doc-samples" || *mode == "dataflow-doc-samples-diff" {
		scenario, err = compat.LoadScenario(file)
	} else if *mode == "infra-named-window-on-update" || *mode == "infra-named-window-on-update-diff" {
		scenario, err = loadInfraNamedWindowOnUpdateScenario(file)
	} else if *mode == "infra-named-window-on-update-misc" || *mode == "infra-named-window-on-update-misc-diff" {
		scenario, err = loadInfraNamedWindowOnUpdateMiscScenario(file)
	} else if *mode == "infra-named-window-insert-from" || *mode == "infra-named-window-insert-from-diff" {
		scenario, err = loadInfraNamedWindowInsertFromScenario(file)
	} else if *mode == "infra-named-window-keepall-delete" || *mode == "infra-named-window-keepall-delete-diff" {
		scenario, err = loadInfraNWKDScenario(file)
	} else if *mode == "infra-named-window-on-select" || *mode == "infra-named-window-on-select-diff" {
		scenario, err = loadInfraNWOSScenario(file)
	} else if *mode == "infra-named-window-subquery" || *mode == "infra-named-window-subquery-diff" {
		scenario, err = loadInfraNWSubqueryScenario(file)
	} else if *mode == "infra-nwtable-event-type" || *mode == "infra-nwtable-event-type-diff" {
		scenario, err = loadInfraNWTableEventTypeScenario(file)
	} else if *mode == "infra-named-window-retention-views" || *mode == "infra-named-window-retention-views-diff" {
		scenario, err = loadInfraNWRVScenario(file)
	} else if *mode == "infra-named-window-unique-views" || *mode == "infra-named-window-unique-views-diff" {
		scenario, err = loadInfraNWRUScenario(file)
	} else if *mode == "infra-named-window-length-views" || *mode == "infra-named-window-length-views-diff" {
		scenario, err = loadInfraNWRLScenario(file)
	} else if *mode == "infra-named-window-lengthbatch-sort-views" || *mode == "infra-named-window-lengthbatch-sort-views-diff" {
		scenario, err = loadInfraNWRBScenario(file)
	} else if *mode == "infra-named-window-time-views" || *mode == "infra-named-window-time-views-diff" {
		scenario, err = loadInfraNWRTScenario(file)
	} else if *mode == "infra-named-window-ext-time-views" || *mode == "infra-named-window-ext-time-views-diff" {
		scenario, err = loadInfraNWRXScenario(file)
	} else if *mode == "infra-named-window-time-order-accum-views" || *mode == "infra-named-window-time-order-accum-views-diff" {
		scenario, err = loadInfraNWRAScenario(file)
	} else if *mode == "infra-named-window-time-batch-views" || *mode == "infra-named-window-time-batch-views-diff" {
		scenario, err = loadInfraNWRTBScenario(file)
	} else if *mode == "infra-named-window-groupwin-views" || *mode == "infra-named-window-groupwin-views-diff" {
		scenario, err = loadInfraNWGWScenario(file)
	} else if *mode == "infra-named-window-consumer-views" || *mode == "infra-named-window-consumer-views-diff" {
		scenario, err = loadInfraNWCViewScenario(file)
	} else if *mode == "infra-named-window-bean-views" || *mode == "infra-named-window-bean-views-diff" {
		scenario, err = loadInfraNWBVScenario(file)
	} else if *mode == "infra-named-window-insert-shape" || *mode == "infra-named-window-insert-shape-diff" {
		scenario, err = loadInfraNamedWindowInsertShapeScenario(file)
	} else if *mode == "infra-named-window-final-views" || *mode == "infra-named-window-final-views-diff" {
		scenario, err = loadInfraNWFVScenario(file)
	} else if *mode == "resultset-aggregate-filter-named-parameter-linear-join" || *mode == "resultset-aggregate-filter-named-parameter-linear-join-diff" {
		scenario, err = loadResultsetAggregateFilterNamedParameterLinearJoinScenario(file)
	} else if *mode == "resultset-aggregate-filter-named-parameter-sorted-join" || *mode == "resultset-aggregate-filter-named-parameter-sorted-join-diff" {
		scenario, err = loadResultsetAggregateFilterNamedParameterSortedJoinScenario(file)
	} else if *mode == "infra-nwtable-subq-correl-coerce" || *mode == "infra-nwtable-subq-correl-coerce-diff" {
		scenario, err = loadInfraNWTableSubqCorrelCoerceScenario(file)
	} else if *mode == "epl-other-stream-expr" || *mode == "epl-other-stream-expr-diff" {
		scenario, err = loadEplOtherStreamExprScenario(file)
	} else if *mode == "epl-other-create-expression" || *mode == "epl-other-create-expression-diff" {
		scenario, err = loadEplOtherCreateExpressionScenario(file)
	} else if *mode == "epl-other-invalid" || *mode == "epl-other-invalid-diff" {
		scenario, err = loadEplOtherInvalidScenario(file)
	} else if *mode == "expr-define-value-parameter" || *mode == "expr-define-value-parameter-diff" {
		scenario, err = loadExprDefineValueParameterScenario(file)
	} else if *mode == "expr-filter-opt-lkup-limited-remaining" || *mode == "expr-filter-opt-lkup-limited-remaining-diff" {
		scenario, err = loadEfolrScenario(file)
	} else if *mode == "epl-other-select-expr-stream-selector" || *mode == "epl-other-select-expr-stream-selector-diff" {
		scenario, err = loadEplOtherSelectExprStreamSelectorScenario(file)
	} else if *mode == "infra-nwtable-start-stop" || *mode == "infra-nwtable-start-stop-diff" {
		scenario, err = loadInfraNWTableStartStopScenario(file)
	} else if *mode == "resultset-querytype-row-for-all-select-avg-expr-std-group-by" || *mode == "resultset-querytype-row-for-all-select-avg-expr-std-group-by-diff" {
		scenario, err = loadResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(file)
	} else if *mode == "resultset-querytype-row-for-all-select-avg-std-group-by-uni" || *mode == "resultset-querytype-row-for-all-select-avg-std-group-by-uni-diff" {
		scenario, err = loadResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(file)
	} else if *mode == "resultset-querytype-row-for-all-static-method-double-nested" || *mode == "resultset-querytype-row-for-all-static-method-double-nested-diff" {
		scenario, err = loadResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(file)
	} else if *mode == "resultset-querytype-row-for-all-having-avg-group-window" || *mode == "resultset-querytype-row-for-all-having-avg-group-window-diff" {
		scenario, err = loadResultSetQueryTypeRowForAllHavingAvgScenario(file)
	} else if *mode == "resultset-querytype-row-for-all-having-sum-join" || *mode == "resultset-querytype-row-for-all-having-sum-join-diff" {
		scenario, err = loadResultSetQueryTypeRowForAllHavingSumJoinScenario(file)
	} else if *mode == "resultset-querytype-row-for-all-having-sum" || *mode == "resultset-querytype-row-for-all-having-sum-diff" {
		scenario, err = loadResultSetQueryTypeRowForAllHavingSumScenario(file)
	} else if *mode == "resultset-aggregate-sorted-multi-criteria" || *mode == "resultset-aggregate-sorted-multi-criteria-diff" {
		scenario, err = loadResultSetAggregateSortedMultiCriteriaScenario(file)
	} else if *mode == "resultset-aggregate-median-and-deviation" || *mode == "resultset-aggregate-median-and-deviation-diff" {
		scenario, err = loadResultSetAggregateMedianAndDeviationScenario(file)
	} else if *mode == "resultset-aggregate-minmax-named-window-wever" || *mode == "resultset-aggregate-minmax-named-window-wever-diff" {
		scenario, err = loadResultSetAggregateMinMaxNamedWindowWEverScenario(file)
	} else if *mode == "resultset-aggregate-minmax-no-data-window-subquery" || *mode == "resultset-aggregate-minmax-no-data-window-subquery-diff" {
		scenario, err = loadResultSetAggregateMinMaxNoDataWindowSubqueryScenario(file)
	} else if *mode == "resultset-aggregate-sorted-no-data-window" || *mode == "resultset-aggregate-sorted-no-data-window-diff" {
		scenario, err = loadResultSetAggregateSortedNoDataWindowScenario(file)
	} else if *mode == "resultset-aggregate-window" || *mode == "resultset-aggregate-window-diff" {
		scenario, err = loadResultSetAggregateWindowScenario(file)
	} else if *mode == "resultset-aggregate-sorted-table-access" || *mode == "resultset-aggregate-sorted-table-access-diff" {
		scenario, err = loadResultSetAggregateSortedTableAccessScenario(file)
	} else if *mode == "resultset-aggregate-sorted-grouped" || *mode == "resultset-aggregate-sorted-grouped-diff" {
		scenario, err = loadResultSetAggregateSortedGroupedScenario(file)
	} else if *mode == "resultset-aggregate-sorted-first-last" || *mode == "resultset-aggregate-sorted-first-last-diff" {
		scenario, err = loadResultSetAggregateSortedFirstLastScenario(file)
	} else if *mode == "resultset-aggregate-sorted-minmax-by-no-alias" || *mode == "resultset-aggregate-sorted-minmax-by-no-alias-diff" {
		scenario, err = loadResultSetAggregateSortedMinMaxByNoAliasScenario(file)
	} else if *mode == "resultset-aggregate-filtered-w-math-context" || *mode == "resultset-aggregate-filtered-w-math-context-diff" {
		scenario, err = loadResultSetAggregateFilteredWMathContextScenario(file)
	} else if *mode == "resultset-aggregate-filter-named-parameter" || *mode == "resultset-aggregate-filter-named-parameter-diff" {
		scenario, err = loadResultSetAggregateFilterNamedParameterScenario(file)
	} else if *mode == "rollup-dimensionality" || *mode == "rollup-dimensionality-diff" {
		scenario, err = loadRollupDimensionalityScenario(file)
	} else if *mode == "rollup-grouping-funcs-dedicated" || *mode == "rollup-grouping-funcs-dedicated-diff" {
		scenario, err = loadRollupGroupingFuncsScenario(file)
	} else if *mode == "rollup-grouping-funcs-faf-dedicated" || *mode == "rollup-grouping-funcs-faf-dedicated-diff" {
		scenario, err = loadRollupGroupingFAFScenario(file)
	} else if *mode == "resultset-querytype-rollup-having-iterator" || *mode == "resultset-querytype-rollup-having-iterator-diff" {
		scenario, err = loadResultSetQueryTypeRollupHavingIteratorScenario(file)
	} else if *mode == "resultset-querytype-rollup-orderby-unidirectional" || *mode == "resultset-querytype-rollup-orderby-unidirectional-diff" {
		scenario, err = loadResultSetQueryTypeRollupOrderByUnidirectionalScenario(file)
	} else if *mode == "resultset-querytype-local-group-by" || *mode == "resultset-querytype-local-group-by-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupByScenario(file)
	} else if *mode == "resultset-querytype-local-group-ungrouped" || *mode == "resultset-querytype-local-group-ungrouped-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupUngroupedScenario(file)
	} else if *mode == "resultset-querytype-local-group-extended" || *mode == "resultset-querytype-local-group-extended-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupExtendedScenario(file)
	} else if *mode == "resultset-querytype-local-group-grouped" || *mode == "resultset-querytype-local-group-grouped-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupGroupedScenario(file)
	} else if *mode == "resultset-querytype-local-group-ungrouped-agg" || *mode == "resultset-querytype-local-group-ungrouped-agg-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupUngroupedAggScenario(file)
	} else if *mode == "resultset-querytype-local-group-row-remove" || *mode == "resultset-querytype-local-group-row-remove-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupRowRemoveScenario(file)
	} else if *mode == "resultset-querytype-local-group-closure" || *mode == "resultset-querytype-local-group-closure-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupClosureScenario(file)
	} else if *mode == "resultset-querytype-local-group-context-terminated" || *mode == "resultset-querytype-local-group-context-terminated-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupCtxTermScenario(file)
	} else if *mode == "resultset-querytype-local-group-keys" || *mode == "resultset-querytype-local-group-keys-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupKeysScenario(file)
	} else if *mode == "resultset-querytype-local-group-solution-pattern" || *mode == "resultset-querytype-local-group-solution-pattern-diff" {
		scenario, err = loadResultSetQueryTypeLocalGroupSolutionScenario(file)
	} else if *mode == "view-intersect" || *mode == "view-intersect-diff" {
		scenario, err = loadViewIntersectScenario(file)
	} else if *mode == "orderby-rowperevent-agg" || *mode == "orderby-rowperevent-agg-diff" {
		scenario, err = loadOrderByRowPerEventAggScenario(file)
	} else if *mode == "orderby-rowperevent-agg-join" || *mode == "orderby-rowperevent-agg-join-diff" {
		scenario, err = loadOrderByRowPerEventAggJoinScenario(file)
	} else if *mode == "orderby-rowperevent-iterator" || *mode == "orderby-rowperevent-iterator-diff" {
		scenario, err = loadOrderByRowPerEventIteratorScenario(file)
	} else if *mode == "resultset-output-limit-row-limit-context-grouped" || *mode == "resultset-output-limit-row-limit-context-grouped-diff" {
		scenario, err = loadResultsetOutputLimitRowLimitContextGroupedScenario(file)
	} else if *mode == "resultset-output-limit-row-limit" || *mode == "resultset-output-limit-row-limit-diff" {
		scenario, err = loadResultsetOutputLimitRowLimitScenario(file)
	} else if *mode == "resultset-output-limit-row-limit-negative-rowcount" || *mode == "resultset-output-limit-row-limit-negative-rowcount-diff" {
		scenario, err = loadResultsetOutputLimitRowLimitNegativeRowcountScenario(file)
	} else if *mode == "resultset-output-limit-row-limit-invalid" || *mode == "resultset-output-limit-row-limit-invalid-diff" {
		scenario, err = loadResultsetOutputLimitRowLimitInvalidScenario(file)
	} else if *mode == "resultset-output-limit-row-limit-variable" || *mode == "resultset-output-limit-row-limit-variable-diff" {
		scenario, err = loadResultsetOutputLimitRowLimitVariableScenario(file)
	} else if *mode == "expr-filter-in-and-between" || *mode == "expr-filter-in-and-between-diff" {
		scenario, err = loadEfabScenario(file)
	} else if *mode == "expr-filter-expressions" || *mode == "expr-filter-expressions-diff" {
		scenario, err = loadEfeScenario(file)
	} else if *mode == "infra-table-join" || *mode == "infra-table-join-diff" {
		scenario, err = loadInfraTableJoinScenario(file)
	} else if *mode == "infra-table-reset" || *mode == "infra-table-reset-diff" {
		scenario, err = loadInfraTableResetScenario(file)
	} else if *mode == "infra-table-select-enum-multikey" || *mode == "infra-table-select-enum-multikey-diff" {
		scenario, err = loadInfraTableSelectEnumMultikeyScenario(file)
	} else if *mode == "infra-table-update-and-index" || *mode == "infra-table-update-and-index-diff" {
		scenario, err = loadInfraTableUpdateIndexScenario(file)
	} else if *mode == "infra-table-faf-execute-query" || *mode == "infra-table-faf-execute-query-diff" {
		scenario, err = loadInfraTableFAFScenario(file)
	} else if *mode == "infra-table-subquery" || *mode == "infra-table-subquery-diff" {
		scenario, err = loadInfraTableSubqueryScenario(file)
	} else if *mode == "infra-table-context" || *mode == "infra-table-context-diff" {
		scenario, err = loadInfraTableContextScenario(file)
	} else if *mode == "infra-nwtable-context" || *mode == "infra-nwtable-context-diff" {
		scenario, err = loadInfraNWTableContextScenario(file)
	} else if *mode == "infra-nwtable-comparative-557" || *mode == "infra-nwtable-comparative-557-diff" {
		scenario, err = loadInfraNWTableComparative557Scenario(file)
	} else if *mode == "infra-nwtable-widening-558" || *mode == "infra-nwtable-widening-558-diff" {
		scenario, err = loadInfraNWTableWidening558Scenario(file)
	} else if *mode == "infra-nwtable-index-faf-559" || *mode == "infra-nwtable-index-faf-559-diff" {
		scenario, err = loadInfraNWTableIndexFAF559Scenario(file)
	} else if *mode == "infra-nwtable-late-index-560" || *mode == "infra-nwtable-late-index-560-diff" {
		scenario, err = loadInfraNWTableLateIndex560Scenario(file)
	} else if *mode == "infra-nwtable-index-ops-561" || *mode == "infra-nwtable-index-ops-561-diff" {
		scenario, err = loadInfraNWTableIndexOps561Scenario(file)
	} else if *mode == "infra-nwtable-mrak-562" || *mode == "infra-nwtable-mrak-562-diff" {
		scenario, err = loadInfraNWTableMRAK562Scenario(file)
	} else if *mode == "infra-nwtable-create-ddl-563" || *mode == "infra-nwtable-create-ddl-563-diff" {
		scenario, err = loadInfraNWTableCreateDDL563Scenario(file)
	} else if *mode == "infra-namedwindow-om-564" || *mode == "infra-namedwindow-om-564-diff" {
		scenario, err = loadInfraNWOM564Scenario(file)
	} else if *mode == "epl-contained-event-example" || *mode == "epl-contained-event-example-diff" {
		scenario, err = loadEPLContainedEventExampleScenario(file)
	} else if *mode == "epl-other-pattern-event-properties" || *mode == "epl-other-pattern-event-properties-diff" {
		scenario, err = loadEPLOtherPatternEventPropertiesScenario(file)
	} else if *mode == "event-map-properties" || *mode == "event-map-properties-diff" {
		scenario, err = loadEventMapPropertiesScenario(file)
	} else if *mode == "event-object-array-core" || *mode == "event-object-array-core-diff" {
		scenario, err = loadEventObjectArrayCoreScenario(file)
	} else if *mode == "event-infra-getter-dynamic" || *mode == "event-infra-getter-dynamic-diff" {
		scenario, err = loadEventInfraGetterDynamicScenario(file)
	} else if *mode == "event-infra-getter-nested" || *mode == "event-infra-getter-nested-diff" {
		scenario, err = loadEventInfraGetterNestedScenario(file)
	} else if *mode == "event-infra-545" || *mode == "event-infra-545-diff" {
		scenario, err = loadEventInfra545Scenario(file)
	} else if *mode == "event-render-546" || *mode == "event-render-546-diff" {
		scenario, err = loadEventRender546Scenario(file)
	} else if *mode == "event-map-nested-547" || *mode == "event-map-nested-547-diff" {
		scenario, err = loadEventMapNested547Scenario(file)
	} else if *mode == "event-avro-hook-548" || *mode == "event-avro-hook-548-diff" {
		scenario, err = loadEventAvroHook548Scenario(file)
	} else if *mode == "event-infra-property-dynamic" || *mode == "event-infra-property-dynamic-diff" {
		scenario, err = loadEventInfraPropertyDynamicScenario(file)
	} else if *mode == "event-infra-property-non-dynamic" || *mode == "event-infra-property-non-dynamic-diff" {
		scenario, err = loadEventInfraPropertyNonDynamicScenario(file)
	} else if *mode == "epl-other-istream-rstream-keywords" || *mode == "epl-other-istream-rstream-keywords-diff" {
		scenario, err = loadEplOtherIStreamRStreamKeywordsScenario(file)
	} else if *mode == "infra-table-invalid" || *mode == "infra-table-invalid-diff" {
		scenario, err = loadInfraTableInvalidScenario(file)
	} else if *mode == "infra-table-count-min-sketch" || *mode == "infra-table-count-min-sketch-diff" {
		scenario, err = loadInfraTableCMSScenario(file)
	} else if *mode == "resultset-output-limit-crontab-when-closure" || *mode == "resultset-output-limit-crontab-when-closure-diff" {
		scenario, err = loadResultSetOutputLimitCrontabWhenClosureScenario(file)
	} else if *mode == "resultset-aggregate-invalid-closure" || *mode == "resultset-aggregate-invalid-closure-diff" {
		scenario, err = loadResultSetAggregateInvalidClosureScenario(file)
	} else if *mode == "view-group-closure" || *mode == "view-group-closure-diff" {
		scenario, err = loadViewGroupClosureScenario(file)
	} else {
		scenario, err = compat.LoadScenario(file)
	}
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
	if *mode == "epl-variable-output-rate" || *mode == "epl-variable-output-rate-diff" {
		trace, err := runEplVariableOutputRateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-variable-output-rate-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplVariableOutputRateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplVariableOutputRateJavaSources),
				splitMetadata(*javaExecutions, eplVariableOutputRateJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-subselect-within-filter-having" || *mode == "epl-subselect-within-filter-having-diff" {
		trace, err := runEplSubselectWithinFilterHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-subselect-within-filter-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplSubselectWithinFilterHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplSubselectWithinFilterHavingJavaSources),
				splitMetadata(*javaExecutions, eplSubselectWithinFilterHavingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-subselect-order-of-eval-index" || *mode == "epl-subselect-order-of-eval-index-diff" {
		trace, err := runEplSubselectOrderOfEvalIndexScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-subselect-order-of-eval-index-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplSubselectOrderOfEvalIndexJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplSubselectOrderOfEvalIndexJavaSources),
				splitMetadata(*javaExecutions, eplSubselectOrderOfEvalIndexJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subq-uncorrel" || *mode == "infra-nwtable-subq-uncorrel-diff" {
		trace, err := runInfraNWTableSubqUncorrelScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subq-uncorrel-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqUncorrelJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqUncorrelJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqUncorrelJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-delete" || *mode == "infra-nwtable-on-delete-diff" {
		trace, err := runInfraNWTableOnDeleteScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-delete-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnDeleteJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnDeleteJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnDeleteJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnDeleteJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-select-aggregation" || *mode == "infra-nwtable-on-select-aggregation-diff" {
		trace, err := runInfraNWTableOnSelectAggScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-select-aggregation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnSelectAggJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnSelectAggJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnSelectAggJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnSelectAggJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-update" || *mode == "infra-nwtable-on-update-diff" {
		trace, err := runInfraNWTableOnUpdateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-update-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnUpdateJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnUpdateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnUpdateJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnUpdateJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge" || *mode == "infra-nwtable-on-merge-diff" {
		trace, err := runInfraNWTableOnMergeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-nested" || *mode == "infra-nwtable-on-merge-nested-diff" {
		trace, err := runInfraNWTableOnMergeNestedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-nested-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeNestedJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeNestedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeNestedJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeNestedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-insertstream" || *mode == "infra-nwtable-on-merge-insertstream-diff" {
		trace, err := runInfraNWTableOnMergeInsertStreamScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-insertstream-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeInsertStreamJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeInsertStreamJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeInsertStreamJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeInsertStreamJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-multiaction" || *mode == "infra-nwtable-on-merge-multiaction-diff" {
		trace, err := runInfraNWTableOnMergeMultiactionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-multiaction-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeMultiactionJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeMultiactionJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeMultiactionJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeMultiactionJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-pattern-nowhere" || *mode == "infra-nwtable-on-merge-pattern-nowhere-diff" {
		trace, err := runInfraNWTableOnMergePatternNoWhereScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-pattern-nowhere-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergePatternNoWhereJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergePatternNoWhereJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergePatternNoWhereJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergePatternNoWhereJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-flow-itv" || *mode == "infra-nwtable-on-merge-flow-itv-diff" {
		trace, err := runInfraNWTableOnMergeFlowITVScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-flow-itv-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeFlowITVJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeFlowITVJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeFlowITVJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeFlowITVJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-invalid-insertonly" || *mode == "infra-nwtable-on-merge-invalid-insertonly-diff" {
		trace, err := runInfraNWTableOnMergeInvalidInsertOnlyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-invalid-insertonly-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeInvalidInsertOnlyJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeInvalidInsertOnlyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeInvalidInsertOnlyJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeInvalidInsertOnlyJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-on-merge-insertonly-deletethenupdate" || *mode == "infra-nwtable-on-merge-insertonly-deletethenupdate-diff" {
		trace, err := runInfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-on-merge-insertonly-deletethenupdate-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableOnMergeIDTUJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableOnMergeIDTUJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableOnMergeIDTUJavaSources),
				splitMetadata(*javaExecutions, infraNWTableOnMergeIDTUJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-namedwindow-on-delete-silent" || *mode == "infra-namedwindow-on-delete-silent-diff" {
		trace, err := runInfraNWOnDeleteSilentScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-namedwindow-on-delete-silent-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWOnDeleteSilentJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWOnDeleteSilentJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWOnDeleteSilentJavaSources),
				splitMetadata(*javaExecutions, infraNWOnDeleteSilentJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-namedwindow-processing-order" || *mode == "infra-namedwindow-processing-order-diff" {
		trace, err := runInfraNWProcessingOrderScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-namedwindow-processing-order-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWProcessingOrderJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWProcessingOrderJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWProcessingOrderSources),
				splitMetadata(*javaExecutions, infraNWProcessingOrderJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-namedwindow-consumer" || *mode == "infra-namedwindow-consumer-diff" {
		trace, err := runInfraNWConsumerScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-namedwindow-consumer-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWConsumerJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWConsumerJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWConsumerSources),
				splitMetadata(*javaExecutions, infraNWConsumerJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-namedwindow-on-delete-indexes" || *mode == "infra-namedwindow-on-delete-indexes-diff" {
		trace, err := runInfraNWOnDeleteIndexesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-namedwindow-on-delete-indexes-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraNWOnDeleteIndexesJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWOnDeleteIndexesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWOnDeleteIndexesSources),
				splitMetadata(*javaExecutions, infraNWOnDeleteIndexesJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subq-at-eventbean" || *mode == "infra-nwtable-subq-at-eventbean-diff" {
		trace, err := runInfraNWTableSubqAtEventBeanScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subq-at-eventbean-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqAtEventBeanJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqAtEventBeanJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqAtEventBeanJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subq-correl-join" || *mode == "infra-nwtable-subq-correl-join-diff" {
		trace, err := runInfraNWTableSubqCorrelJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subq-correl-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqCorrelJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqCorrelJoinJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqCorrelJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-stream-expr" || *mode == "epl-other-stream-expr-diff" {
		trace, err := runEplOtherStreamExprScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-stream-expr-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherStreamExprJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherStreamExprJavaSources),
				splitMetadata(*javaExecutions, eplOtherStreamExprJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-create-expression" || *mode == "epl-other-create-expression-diff" {
		trace, err := runEplOtherCreateExpressionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-create-expression-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplOtherCreateExpressionJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherCreateExpressionJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherCreateExpressionJavaSources),
				splitMetadata(*javaExecutions, eplOtherCreateExpressionJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-invalid" || *mode == "epl-other-invalid-diff" {
		trace, err := runEplOtherInvalidScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-invalid-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplOtherInvalidJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherInvalidJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherInvalidSources),
				splitMetadata(*javaExecutions, eplOtherInvalidJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-define-value-parameter" || *mode == "expr-define-value-parameter-diff" {
		trace, err := runExprDefineValueParameterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-define-value-parameter-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDefineValueParameterJavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDefineValueParameterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDefineValueParameterJavaSources),
				splitMetadata(*javaExecutions, exprDefineValueParameterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-doc-samples" || *mode == "dataflow-doc-samples-diff" {
		trace, err := runDataflowDocSamplesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-doc-samples-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowDocSamplesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowDocSamplesJavaSources),
				splitMetadata(*javaExecutions, dataflowDocSamplesJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-lifecycle-blocking" || *mode == "dataflow-lifecycle-blocking-diff" {
		trace, err := runDataflowLifecycleBlockingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-lifecycle-blocking-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowLifecycleBlockingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowLifecycleBlockingJavaSources),
				splitMetadata(*javaExecutions, dataflowLifecycleBlockingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-database-join" || *mode == "epl-database-join-diff" {
		trace, err := runEplDatabaseJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-database-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplDatabaseJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplDatabaseJoinJavaSources),
				splitMetadata(*javaExecutions, eplDatabaseJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-database-join-2" || *mode == "epl-database-join-2-diff" {
		trace, err := runEplDatabaseJoin2Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-database-join-2-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplDatabaseJoin2JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplDatabaseJoin2JavaSources),
				splitMetadata(*javaExecutions, eplDatabaseJoin2JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-database-restart" || *mode == "epl-database-restart-diff" {
		trace, err := runEplDatabaseRestartScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-database-restart-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplDatabaseRestartJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplDatabaseRestartJavaSources),
				splitMetadata(*javaExecutions, eplDatabaseRestartJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-database-timebatch" || *mode == "epl-database-timebatch-diff" {
		trace, err := runEplDatabaseTimeBatchScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-database-timebatch-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplDatabaseTimeBatchJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplDatabaseTimeBatchJavaSources),
				splitMetadata(*javaExecutions, eplDatabaseTimeBatchJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-json-sender-getter" || *mode == "event-json-sender-getter-diff" {
		trace, err := runDataflowEventJsonSenderGetterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-json-sender-getter-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eventJsonSenderGetterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventJsonSenderGetterJavaSources),
				splitMetadata(*javaExecutions, eventJsonSenderGetterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-json-adapter" || *mode == "event-json-adapter-diff" {
		trace, err := runEventJsonAdapterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-json-adapter-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eventJsonAdapterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventJsonAdapterJavaSources),
				splitMetadata(*javaExecutions, eventJsonAdapterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-lifecycle-cancel-join" || *mode == "dataflow-lifecycle-cancel-join-diff" {
		trace, err := runDataflowLifecycleCancelJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-lifecycle-cancel-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowLifecycleCancelJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowLifecycleCancelJoinJavaSources),
				splitMetadata(*javaExecutions, dataflowLifecycleCancelJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-lifecycle-core" || *mode == "dataflow-lifecycle-core-diff" {
		trace, err := runDataflowLifecycleCoreScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-lifecycle-core-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowLifecycleCoreJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowLifecycleCoreJavaSources),
				splitMetadata(*javaExecutions, dataflowLifecycleCoreJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-exceptions" || *mode == "dataflow-exceptions-diff" {
		trace, err := runDataflowExceptionsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-exceptions-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowExceptionsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowExceptionsJavaSources),
				splitMetadata(*javaExecutions, dataflowExceptionsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-ports-feedback" || *mode == "dataflow-ports-feedback-diff" {
		trace, err := runDataflowPortsFeedbackScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-ports-feedback-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowPortsFeedbackJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowPortsFeedbackJavaSources),
				splitMetadata(*javaExecutions, dataflowPortsFeedbackJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-captive-lifecycle" || *mode == "dataflow-captive-lifecycle-diff" {
		trace, err := runDataflowCaptiveLifecycleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-captive-lifecycle-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowCaptiveLifecycleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowCaptiveLifecycleJavaSources),
				splitMetadata(*javaExecutions, dataflowCaptiveLifecycleJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-eventbus-sink" || *mode == "dataflow-eventbus-sink-diff" {
		trace, err := runDataflowEventbusSinkScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-eventbus-sink-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowEventbusSinkJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowEventbusSinkJavaSources),
				splitMetadata(*javaExecutions, dataflowEventbusSinkJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-eventbus-source" || *mode == "dataflow-eventbus-source-diff" {
		trace, err := runDataflowEventbusSourceScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-eventbus-source-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowEventbusSourceJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowEventbusSourceJavaSources),
				splitMetadata(*javaExecutions, dataflowEventbusSourceJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-beacon-source" || *mode == "dataflow-beacon-source-diff" {
		trace, err := runDataflowBeaconSourceScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-beacon-source-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowBeaconSourceJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowBeaconSourceJavaSources),
				splitMetadata(*javaExecutions, dataflowBeaconSourceJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-epstatement-source" || *mode == "dataflow-epstatement-source-diff" {
		trace, err := runDataflowEPStatementSourceScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-epstatement-source-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowEPStatementSourceJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowEPStatementSourceJavaSources),
				splitMetadata(*javaExecutions, dataflowEPStatementSourceJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-select-representation" || *mode == "dataflow-select-representation-diff" {
		trace, err := runDataflowSelectRepresentationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-select-representation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowSelectRepresentationJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowSelectRepresentationJavaSources),
				splitMetadata(*javaExecutions, dataflowSelectRepresentationJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-select-state" || *mode == "dataflow-select-state-diff" {
		trace, err := runDataflowSelectStateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-select-state-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowSelectStateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowSelectStateJavaSources),
				splitMetadata(*javaExecutions, dataflowSelectStateJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-select-flows" || *mode == "dataflow-select-flows-diff" {
		trace, err := runDataflowSelectFlowsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-select-flows-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowSelectFlowsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowSelectFlowsJavaSources),
				splitMetadata(*javaExecutions, dataflowSelectFlowsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-create-start-stop-destroy" || *mode == "dataflow-create-start-stop-destroy-diff" {
		trace, err := runDataflowCreateStartStopDestroyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-create-start-stop-destroy-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowCSSDJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowCSSDJavaSources),
				splitMetadata(*javaExecutions, dataflowCSSDJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-op-lifecycle" || *mode == "dataflow-op-lifecycle-diff" {
		trace, err := runDataflowOpLifecycleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-op-lifecycle-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowOpLifecycleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowOpLifecycleJavaSources),
				splitMetadata(*javaExecutions, dataflowOpLifecycleJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "dataflow-types" || *mode == "dataflow-types-diff" {
		trace, err := runDataflowTypesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "dataflow-types-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, dataflowTypesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, dataflowTypesJavaSources),
				splitMetadata(*javaExecutions, dataflowTypesJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-from-clause-method-variable" || *mode == "epl-from-clause-method-variable-diff" {
		trace, err := runEplFromClauseMethodVariableScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-from-clause-method-variable-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplFromClauseMethodVariableJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplFromClauseMethodVariableJavaSources),
				splitMetadata(*javaExecutions, eplFromClauseMethodVariableJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-as-keyword-backtick" || *mode == "epl-as-keyword-backtick-diff" {
		trace, err := runEplAsKeywordBacktickScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-as-keyword-backtick-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplAsKeywordBacktickJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplAsKeywordBacktickJavaSources),
				splitMetadata(*javaExecutions, eplAsKeywordBacktickJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "viewgroup-merge-view" || *mode == "viewgroup-merge-view-diff" {
		trace, err := runViewGroupMergeViewScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "viewgroup-merge-view-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewGroupMergeViewJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewGroupMergeViewJavaSources),
				splitMetadata(*javaExecutions, viewGroupMergeViewJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-group-closure" || *mode == "view-group-closure-diff" {
		trace, err := runViewGroupClosureScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-group-closure-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewGroupClosureJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewGroupClosureSources),
				splitMetadata(*javaExecutions, viewGroupClosureJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-select-expr-stream-selector" || *mode == "epl-other-select-expr-stream-selector-diff" {
		trace, err := runEplOtherSelectExprStreamSelectorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-select-expr-stream-selector-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherSelectExprStreamSelectorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherSelectExprStreamSelectorJavaSources),
				splitMetadata(*javaExecutions, eplOtherSelectExprStreamSelectorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subq-correl-coerce" || *mode == "infra-nwtable-subq-correl-coerce-diff" {
		trace, err := runInfraNWTableSubqCorrelCoerceScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subq-correl-coerce-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqCorrelCoerceJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqCorrelCoerceJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqCorrelCoerceJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-filter-named-parameter-sorted-join" || *mode == "resultset-aggregate-filter-named-parameter-sorted-join-diff" {
		trace, err := runResultsetAggregateFilterNamedParameterSortedJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-filter-named-parameter-sorted-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFilterNamedParameterSortedJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFilterNamedParameterSortedJoinJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFilterNamedParameterSortedJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-filter-named-parameter-linear-join" || *mode == "resultset-aggregate-filter-named-parameter-linear-join-diff" {
		trace, err := runResultsetAggregateFilterNamedParameterLinearJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-filter-named-parameter-linear-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFilterNamedParameterLinearJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFilterNamedParameterLinearJoinJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFilterNamedParameterLinearJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-time-batch-views" || *mode == "infra-named-window-time-batch-views-diff" {
		trace, err := runInfraNWRTBScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-time-batch-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRTBJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRTBJavaSources),
				splitMetadata(*javaExecutions, infraNWRTBJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-groupwin-views" || *mode == "infra-named-window-groupwin-views-diff" {
		trace, err := runInfraNWGWScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-groupwin-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWGWJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWGWJavaSources),
				splitMetadata(*javaExecutions, infraNWGWJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-consumer-views" || *mode == "infra-named-window-consumer-views-diff" {
		trace, err := runInfraNWCViewScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-consumer-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWCViewJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWCViewJavaSources),
				splitMetadata(*javaExecutions, infraNWCViewJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-bean-views" || *mode == "infra-named-window-bean-views-diff" {
		trace, err := runInfraNWBVScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-bean-views-diff" {
			return runDifferentialModeWithGoNormalizer(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWBVJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWBVJavaSources),
				splitMetadata(*javaExecutions, infraNWBVJavaExecutions), scenario, trace, normalizeInfraNWBVTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-time-order-accum-views" || *mode == "infra-named-window-time-order-accum-views-diff" {
		trace, err := runInfraNWRAScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-time-order-accum-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRAJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRAJavaSources),
				splitMetadata(*javaExecutions, infraNWRAJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-ext-time-views" || *mode == "infra-named-window-ext-time-views-diff" {
		trace, err := runInfraNWRXScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-ext-time-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRXJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRXJavaSources),
				splitMetadata(*javaExecutions, infraNWRXJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-time-views" || *mode == "infra-named-window-time-views-diff" {
		trace, err := runInfraNWRTScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-time-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRTJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRTJavaSources),
				splitMetadata(*javaExecutions, infraNWRTJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-lengthbatch-sort-views" || *mode == "infra-named-window-lengthbatch-sort-views-diff" {
		trace, err := runInfraNWRBScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-lengthbatch-sort-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRBJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRBJavaSources),
				splitMetadata(*javaExecutions, infraNWRBJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-length-views" || *mode == "infra-named-window-length-views-diff" {
		trace, err := runInfraNWRLScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-length-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRLJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRLJavaSources),
				splitMetadata(*javaExecutions, infraNWRLJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-unique-views" || *mode == "infra-named-window-unique-views-diff" {
		trace, err := runInfraNWRUScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-unique-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRUJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRUJavaSources),
				splitMetadata(*javaExecutions, infraNWRUJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-retention-views" || *mode == "infra-named-window-retention-views-diff" {
		trace, err := runInfraNWRVScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-retention-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWRVJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWRVJavaSources),
				splitMetadata(*javaExecutions, infraNWRVJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-keepall-delete" || *mode == "infra-named-window-keepall-delete-diff" {
		trace, err := runInfraNWKDScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-keepall-delete-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWKDJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWKDJavaSources),
				splitMetadata(*javaExecutions, infraNWKDJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-on-select" || *mode == "infra-named-window-on-select-diff" {
		trace, err := runInfraNWOSScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-on-select-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWOSJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWOSJavaSources),
				splitMetadata(*javaExecutions, infraNWOSJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-subquery" || *mode == "infra-named-window-subquery-diff" {
		trace, err := runInfraNWSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWSubqueryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWSubqueryJavaSources),
				splitMetadata(*javaExecutions, infraNWSubqueryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-event-type" || *mode == "infra-nwtable-event-type-diff" {
		trace, err := runInfraNWTableEventTypeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-event-type-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableEventTypeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableEventTypeJavaSources),
				splitMetadata(*javaExecutions, infraNWTableEventTypeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-insert-from" || *mode == "infra-named-window-insert-from-diff" {
		trace, err := runInfraNamedWindowInsertFromScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-insert-from-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNamedWindowInsertFromJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNamedWindowInsertFromJavaSources),
				splitMetadata(*javaExecutions, infraNamedWindowInsertFromJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-insert-shape" || *mode == "infra-named-window-insert-shape-diff" {
		trace, err := runInfraNamedWindowInsertShapeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-insert-shape-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNamedWindowInsertShapeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNamedWindowInsertShapeJavaSources),
				splitMetadata(*javaExecutions, infraNamedWindowInsertShapeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-final-views" || *mode == "infra-named-window-final-views-diff" {
		trace, err := runInfraNWFVScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-final-views-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWFVJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWFVJavaSources),
				splitMetadata(*javaExecutions, infraNWFVJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-on-update-misc" || *mode == "infra-named-window-on-update-misc-diff" {
		trace, err := runInfraNamedWindowOnUpdateMiscScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-on-update-misc-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNamedWindowOnUpdateMiscJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNamedWindowOnUpdateMiscJavaSources),
				splitMetadata(*javaExecutions, infraNamedWindowOnUpdateMiscJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-named-window-on-update" || *mode == "infra-named-window-on-update-diff" {
		trace, err := runInfraNamedWindowOnUpdateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-named-window-on-update-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNamedWindowOnUpdateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNamedWindowOnUpdateJavaSources),
				splitMetadata(*javaExecutions, infraNamedWindowOnUpdateJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-subq-filtered-correl" || *mode == "infra-nwtable-subq-filtered-correl-diff" {
		trace, err := runInfraNWTableSubqFilteredCorrelScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-subq-filtered-correl-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableSubqFilteredCorrelJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableSubqFilteredCorrelJavaSources),
				splitMetadata(*javaExecutions, infraNWTableSubqFilteredCorrelJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-start-stop" || *mode == "infra-nwtable-start-stop-diff" {
		trace, err := runInfraNWTableStartStopScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-start-stop-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableStartStopJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableStartStopJavaSources),
				splitMetadata(*javaExecutions, infraNWTableStartStopJavaExecutions), scenario, trace)
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
	if *mode == "expr-dt-interval-ops" || *mode == "expr-dt-interval-ops-diff" {
		trace, err := runExprDTIntervalOpsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-interval-ops-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTIntervalOpsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTIntervalOpsJavaSources),
				splitMetadata(*javaExecutions, exprDTIntervalOpsJavaExecutions), scenario, trace,
				func(trace compat.Trace) compat.Trace { return trace })
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-resolution" || *mode == "expr-dt-resolution-diff" {
		trace, err := runExprDTResolutionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-resolution-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTResolutionJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTResolutionJavaSources),
				splitMetadata(*javaExecutions, exprDTResolutionJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-data-sources" || *mode == "expr-dt-data-sources-diff" {
		trace, err := runExprDTDataSourcesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-data-sources-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTDataSourcesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTDataSourcesJavaSources),
				splitMetadata(*javaExecutions, exprDTDataSourcesJavaExecutions), scenario, trace)
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
	if *mode == "infra-nwtable-comparative-557" || *mode == "infra-nwtable-comparative-557-diff" {
		trace, err := runInfraNWTableComparative557Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-comparative-557-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableComparative557JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableComparative557JavaSources),
				splitMetadata(*javaExecutions, infraNWTableComparative557JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-widening-558" || *mode == "infra-nwtable-widening-558-diff" {
		trace, err := runInfraNWTableWidening558Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-widening-558-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableWidening558JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableWidening558JavaSources),
				splitMetadata(*javaExecutions, infraNWTableWidening558JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-index-faf-559" || *mode == "infra-nwtable-index-faf-559-diff" {
		trace, err := runInfraNWTableIndexFAF559Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-index-faf-559-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableIndexFAF559JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableIndexFAF559JavaSources),
				splitMetadata(*javaExecutions, infraNWTableIndexFAF559JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-late-index-560" || *mode == "infra-nwtable-late-index-560-diff" {
		trace, err := runInfraNWTableLateIndex560Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-late-index-560-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableLateIndex560JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableLateIndex560JavaSources),
				splitMetadata(*javaExecutions, infraNWTableLateIndex560JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-index-ops-561" || *mode == "infra-nwtable-index-ops-561-diff" {
		trace, err := runInfraNWTableIndexOps561Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-index-ops-561-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableIndexOps561JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableIndexOps561JavaSources),
				splitMetadata(*javaExecutions, infraNWTableIndexOps561JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-create-ddl-563" || *mode == "infra-nwtable-create-ddl-563-diff" {
		trace, err := runInfraNWTableCreateDDL563Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-create-ddl-563-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableCreateDDL563JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableCreateDDL563JavaSources),
				splitMetadata(*javaExecutions, infraNWTableCreateDDL563JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-namedwindow-om-564" || *mode == "infra-namedwindow-om-564-diff" {
		trace, err := runInfraNWOM564Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-namedwindow-om-564-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWOM564JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWOM564JavaSources),
				splitMetadata(*javaExecutions, infraNWOM564JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-mrak-562" || *mode == "infra-nwtable-mrak-562-diff" {
		trace, err := runInfraNWTableMRAK562Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-mrak-562-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableMRAK562JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableMRAK562JavaSources),
				splitMetadata(*javaExecutions, infraNWTableMRAK562JavaExecutions), scenario, trace)
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
	if *mode == "resultset-aggregate-firstlastwindow-indexed" || *mode == "resultset-aggregate-firstlastwindow-indexed-diff" {
		trace, err := runResultSetAggregateFirstLastWindowIndexedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-firstlastwindow-indexed-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateFirstLastWindowIndexedJavaCommit,
				resultsetAggregateFirstLastWindowIndexedJavaRuntimeIDs, resultsetAggregateFirstLastWindowIndexedJavaSources,
				resultsetAggregateFirstLastWindowIndexedJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-firstlastwindow-prev-nth" || *mode == "resultset-aggregate-firstlastwindow-prev-nth-diff" {
		trace, err := runResultSetAggregateFirstLastWindowPrevNthScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-firstlastwindow-prev-nth-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateFirstLastWindowPrevNthJavaCommit,
				resultsetAggregateFirstLastWindowPrevNthJavaRuntimeIDs, resultsetAggregateFirstLastWindowPrevNthJavaSources,
				resultsetAggregateFirstLastWindowPrevNthJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-nth" || *mode == "resultset-aggregate-nth-diff" {
		trace, err := runResultSetAggregateNthScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-nth-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateNthJavaCommit,
				resultsetAggregateNthJavaRuntimeIDs, resultsetAggregateNthJavaSources,
				resultsetAggregateNthJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-median-and-deviation" || *mode == "resultset-aggregate-median-and-deviation-diff" {
		trace, err := runResultSetAggregateMedianAndDeviationScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-median-and-deviation-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateMedianAndDeviationJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateMedianAndDeviationJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateMedianAndDeviationJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-minmax-no-data-window-subquery" || *mode == "resultset-aggregate-minmax-no-data-window-subquery-diff" {
		trace, err := runResultSetAggregateMinMaxNoDataWindowSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-minmax-no-data-window-subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateMinMaxNoDataWindowSubqueryJavaCommit, resultsetAggregateMinMaxNoDataWindowSubqueryJavaRuntimeIDs, resultsetAggregateMinMaxNoDataWindowSubqueryJavaSources, resultsetAggregateMinMaxNoDataWindowSubqueryJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-minmax-named-window-wever" || *mode == "resultset-aggregate-minmax-named-window-wever-diff" {
		trace, err := runResultSetAggregateMinMaxNamedWindowWEverScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-minmax-named-window-wever-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateMinMaxNamedWindowWEverJavaCommit,
				resultsetAggregateMinMaxNamedWindowWEverJavaRuntimeIDs, resultsetAggregateMinMaxNamedWindowWEverJavaSources,
				resultsetAggregateMinMaxNamedWindowWEverJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-minmax-groupby" || *mode == "resultset-aggregate-minmax-groupby-diff" {
		trace, err := runResultSetAggregateMinMaxGroupByScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-minmax-groupby-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateMinMaxGroupByJavaCommit,
				resultsetAggregateMinMaxGroupByJavaRuntimeIDs, resultsetAggregateMinMaxGroupByJavaSources,
				resultsetAggregateMinMaxGroupByJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-minmax-groupby-join-select-having" || *mode == "resultset-aggregate-minmax-groupby-join-select-having-diff" {
		trace, err := runResultSetAggregateMinMaxGroupByJoinSelectHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-minmax-groupby-join-select-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateMinMaxGroupByJoinSelectHavingJavaCommit,
				resultsetAggregateMinMaxGroupByJoinSelectHavingJavaRuntimeIDs, resultsetAggregateMinMaxGroupByJoinSelectHavingJavaSources,
				resultsetAggregateMinMaxGroupByJoinSelectHavingJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-minmax-groupby-om-viewcompile" || *mode == "resultset-aggregate-minmax-groupby-om-viewcompile-diff" {
		trace, err := runResultSetAggregateMinMaxGroupByOMViewCompileScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-minmax-groupby-om-viewcompile-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateMinMaxGroupByOMViewCompileJavaCommit,
				resultsetAggregateMinMaxGroupByOMViewCompileJavaRuntimeIDs, resultsetAggregateMinMaxGroupByOMViewCompileJavaSources,
				resultsetAggregateMinMaxGroupByOMViewCompileJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-table-access" || *mode == "resultset-aggregate-sorted-table-access-diff" {
		trace, err := runResultSetAggregateSortedTableAccessScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-table-access-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedTableAccessJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateSortedTableAccessJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{resultsetAggregateSortedTableAccessSource}),
				splitMetadata(*javaExecutions, resultsetAggregateSortedTableAccessJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-grouped" || *mode == "resultset-aggregate-sorted-grouped-diff" {
		trace, err := runResultSetAggregateSortedGroupedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-grouped-diff" {
			return runDifferentialModeWithGoNormalizer(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedGroupedJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateSortedGroupedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{resultsetAggregateSortedGroupedSource}),
				splitMetadata(*javaExecutions, resultsetAggregateSortedGroupedJavaExecutions), scenario, trace,
				normalizeResultSetAggregateSortedGroupedTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-first-last" || *mode == "resultset-aggregate-sorted-first-last-diff" {
		trace, err := runResultSetAggregateSortedFirstLastScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-first-last-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedFirstLastJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateSortedFirstLastJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{resultsetAggregateSortedFirstLastSource}),
				splitMetadata(*javaExecutions, resultsetAggregateSortedFirstLastJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-minmax-by-no-alias" || *mode == "resultset-aggregate-sorted-minmax-by-no-alias-diff" {
		trace, err := runResultSetAggregateSortedMinMaxByNoAliasScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-minmax-by-no-alias-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedMinMaxByNoAliasJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateSortedMinMaxByNoAliasJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateSortedMinMaxByNoAliasJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateSortedMinMaxByNoAliasJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-minmax-by" || *mode == "resultset-aggregate-sorted-minmax-by-diff" {
		trace, err := runResultSetAggregateSortedMinMaxByScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-minmax-by-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedMinMaxByJavaCommit,
				resultsetAggregateSortedMinMaxByJavaRuntimeIDs, resultsetAggregateSortedMinMaxByJavaSources,
				resultsetAggregateSortedMinMaxByJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-no-data-window" || *mode == "resultset-aggregate-sorted-no-data-window-diff" {
		trace, err := runResultSetAggregateSortedNoDataWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-no-data-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedNoDataWindowJavaCommit,
				resultsetAggregateSortedNoDataWindowJavaRuntimeIDs, resultsetAggregateSortedNoDataWindowJavaSources,
				resultsetAggregateSortedNoDataWindowJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-multi-criteria" || *mode == "resultset-aggregate-sorted-multi-criteria-diff" {
		trace, err := runResultSetAggregateSortedMultiCriteriaScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-multi-criteria-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedMultiCriteriaJavaCommit,
				resultsetAggregateSortedMultiCriteriaJavaRuntimeIDs, resultsetAggregateSortedMultiCriteriaJavaSources,
				resultsetAggregateSortedMultiCriteriaJavaExecutions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-sorted-multi-criteria-simple" || *mode == "resultset-aggregate-sorted-multi-criteria-simple-diff" {
		trace, err := runResultSetAggregateSortedMultiCriteriaSimpleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-sorted-multi-criteria-simple-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateSortedMultiCriteriaSimpleJavaCommit,
				resultsetAggregateSortedMultiCriteriaSimpleJavaRuntimeIDs, resultsetAggregateSortedMultiCriteriaSimpleJavaSources,
				resultsetAggregateSortedMultiCriteriaSimpleJavaExecutions, scenario, trace)
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
	if *mode == "resultset-aggregate-firstlastwindow-current" || *mode == "resultset-aggregate-firstlastwindow-current-diff" {
		trace, err := runResultSetAggregateFirstLastWindowCurrentScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-firstlastwindow-current-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFirstLastWindowCurrentJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFirstLastWindowCurrentJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFirstLastWindowCurrentJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-firstlastwindow-star" || *mode == "resultset-aggregate-firstlastwindow-star-diff" {
		trace, err := runResultSetAggregateFirstLastWindowStarScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-firstlastwindow-star-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFirstLastWindowStarJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateFirstLastWindowStarJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateFirstLastWindowStarJavaExecutions), scenario, trace)
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
	if *mode == "context-init-term-prioritized" || *mode == "context-init-term-prioritized-diff" {
		trace, err := runContextInitTermPrioritizedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-init-term-prioritized-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextInitTermPrioritizedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextInitTermPrioritizedJavaSources),
				splitMetadata(*javaExecutions, contextInitTermPrioritizedJavaExecutions), scenario, trace)
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
	if *mode == "context-key-segmented-subselect-prev-prior" || *mode == "context-key-segmented-subselect-prev-prior-diff" {
		trace, err := runContextKeySegmentedSubselectPrevPriorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-subselect-prev-prior-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedSubselectPrevPriorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedSubselectPrevPriorJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedSubselectPrevPriorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-infra-prioritized" || *mode == "context-key-segmented-infra-prioritized-diff" {
		trace, err := runContextKeySegmentedInfraPrioritizedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-infra-prioritized-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedInfraPrioritizedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedInfraPrioritizedJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedInfraPrioritizedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-named-window" || *mode == "context-key-segmented-named-window-diff" {
		trace, err := runContextKeySegmentedNamedWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-named-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedNamedWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedNamedWindowJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedNamedWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-named-window-subquery" || *mode == "context-key-segmented-named-window-subquery-diff" {
		trace, err := runContextKeySegmentedNamedWindowSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-named-window-subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedNamedWindowSubqueryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedNamedWindowSubqueryJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedNamedWindowSubqueryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-invalid" || *mode == "context-key-segmented-invalid-diff" {
		trace, err := runContextKeySegmentedInvalidScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-invalid-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedInvalidJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedInvalidJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedInvalidJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-key-segmented-allocation-time" || *mode == "context-key-segmented-allocation-time-diff" {
		trace, err := runContextKeySegmentedAllocationTimeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-key-segmented-allocation-time-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextKeySegmentedAllocationTimeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextKeySegmentedAllocationTimeJavaSources),
				splitMetadata(*javaExecutions, contextKeySegmentedAllocationTimeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-nested-initterm" || *mode == "context-nested-initterm-diff" {
		trace, err := runContextNestedInitTermScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-nested-initterm-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextNestedInitTermJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextNestedInitTermJavaSources),
				splitMetadata(*javaExecutions, contextNestedInitTermJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-lifecycle" || *mode == "context-lifecycle-diff" {
		trace, err := runContextLifecycleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-lifecycle-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextLifecycleJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextLifecycleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextLifecycleSources),
				splitMetadata(*javaExecutions, contextLifecycleJavaExecutions), scenario, trace)
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
	if *mode == "resultset-orderby-join" || *mode == "resultset-orderby-join-diff" {
		trace, err := runResultsetOrderbyJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbyJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbyJoinJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbyJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-multi-delivery" || *mode == "resultset-orderby-multi-delivery-diff" {
		trace, err := runResultsetOrderbyMultiDeliveryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-multi-delivery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbyMultiDeliveryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbyMultiDeliveryJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbyMultiDeliveryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-simple-descending-om" || *mode == "resultset-orderby-simple-descending-om-diff" {
		trace, err := runResultsetOrderbySimpleDescendingOMScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-simple-descending-om-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbySimpleDescendingOMJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbySimpleDescendingOMJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbySimpleDescendingOMJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-simple-expressions-aliases" || *mode == "resultset-orderby-simple-expressions-aliases-diff" {
		trace, err := runResultsetOrderbySimpleExpressionsAliasesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-simple-expressions-aliases-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbySimpleExpressionsAliasesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbySimpleExpressionsAliasesJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbySimpleExpressionsAliasesJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-simple-join-wildcard" || *mode == "resultset-orderby-simple-join-wildcard-diff" {
		trace, err := runResultsetOrderbySimpleJoinWildcardScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-simple-join-wildcard-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbySimpleJoinWildcardJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbySimpleJoinWildcardJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbySimpleJoinWildcardJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-simple-no-output-invalid" || *mode == "resultset-orderby-simple-no-output-invalid-diff" {
		trace, err := runResultsetOrderbySimpleNoOutputInvalidScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-simple-no-output-invalid-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbySimpleNoOutputInvalidJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbySimpleNoOutputInvalidJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbySimpleNoOutputInvalidJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-orderby-self-join" || *mode == "resultset-orderby-self-join-diff" {
		trace, err := runResultsetOrderbySelfJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-self-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbySelfJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbySelfJoinJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbySelfJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "output-after-events" || *mode == "output-after-events-diff" {
		trace, err := runOutputAfterEventsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "output-after-events-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, outputAfterEventsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, outputAfterEventsJavaSources),
				splitMetadata(*javaExecutions, outputAfterEventsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-insert-into" || *mode == "resultset-output-limit-insert-into-diff" {
		trace, err := runResultsetOutputLimitInsertIntoScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-insert-into-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitInsertIntoJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitInsertIntoJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitInsertIntoJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-microsecond-resolution" || *mode == "resultset-output-limit-microsecond-resolution-diff" {
		trace, err := runResultsetOutputLimitMicrosecondScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-microsecond-resolution-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitMicrosecondJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitMicrosecondJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitMicrosecondJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-parameterized-context" || *mode == "resultset-output-limit-parameterized-context-diff" {
		trace, err := runResultsetOutputLimitParameterizedContextScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-parameterized-context-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitParameterizedContextJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitParameterizedContextJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitParameterizedContextJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-changeset-opt" || *mode == "resultset-output-limit-changeset-opt-diff" {
		trace, err := runResultsetOutputLimitChangesetScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-changeset-opt-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitChangesetJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitChangesetJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitChangesetJavaExecutions), scenario, trace)
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
	if *mode == "epl-insert-into-typed-columns" || *mode == "epl-insert-into-typed-columns-diff" {
		trace, err := runEplInsertIntoTypedColumnsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-typed-columns-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoTypedColumnsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoTypedColumnsJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoTypedColumnsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-insert-into-populate-single-col-method-call" || *mode == "epl-insert-into-populate-single-col-method-call-diff" {
		trace, err := runEplInsertIntoSingleColMethodCallScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-populate-single-col-method-call-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoSingleColMethodCallJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoSingleColMethodCallJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoSingleColMethodCallJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-from-clause-optional" || *mode == "epl-other-from-clause-optional-diff" {
		trace, err := runEplOtherFromClauseOptionalScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-from-clause-optional-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherFromClauseOptionalJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherFromClauseOptionalJavaSources),
				splitMetadata(*javaExecutions, eplOtherFromClauseOptionalJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-select-expr-stream-selector-remainder" || *mode == "epl-other-select-expr-stream-selector-remainder-diff" {
		trace, err := runEplOtherSelectExprStreamSelectorRemainderScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-select-expr-stream-selector-remainder-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherSelectExprStreamSelectorRemainderJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherSelectExprStreamSelectorRemainderJavaSources),
				splitMetadata(*javaExecutions, eplOtherSelectExprStreamSelectorRemainderJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-insert-into-from-pattern" || *mode == "epl-insert-into-from-pattern-diff" {
		trace, err := runEplInsertIntoFromPatternScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-from-pattern-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoFromPatternJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoFromPatternJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoFromPatternJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-insert-into-wrapper" || *mode == "epl-insert-into-wrapper-diff" {
		trace, err := runEplInsertIntoWrapperScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-wrapper-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoWrapperJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoWrapperJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoWrapperJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-insert-into-istream-func" || *mode == "epl-insert-into-istream-func-diff" {
		trace, err := runEplInsertIntoIStreamFuncScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-insert-into-istream-func-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eplInsertIntoIStreamFuncJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplInsertIntoIStreamFuncJavaSources),
				splitMetadata(*javaExecutions, eplInsertIntoIStreamFuncJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-istream-rstream-keywords" || *mode == "epl-other-istream-rstream-keywords-diff" {
		trace, err := runEplOtherIStreamRStreamKeywordsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-istream-rstream-keywords-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, eplOtherIStreamRStreamKeywordsJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherIStreamRStreamKeywordsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherIStreamRStreamKeywordsSources),
				splitMetadata(*javaExecutions, eplOtherIStreamRStreamKeywordsJavaExecutions), scenario, trace, normalizeEplOtherIStreamRStreamKeywordsTrace)
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
	if *mode == "infra-table-join" || *mode == "infra-table-join-diff" {
		trace, err := runInfraTableJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableJoinJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableJoinJavaSources),
				splitMetadata(*javaExecutions, infraTableJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-reset" || *mode == "infra-table-reset-diff" {
		trace, err := runInfraTableResetScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-reset-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableResetJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableResetJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableResetJavaSources),
				splitMetadata(*javaExecutions, infraTableResetJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-select-enum-multikey" || *mode == "infra-table-select-enum-multikey-diff" {
		trace, err := runInfraTableSelectEnumMultikeyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-select-enum-multikey-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableSelectEnumMultikeyJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableSelectEnumMultikeyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableSelectEnumMultikeyJavaSources),
				splitMetadata(*javaExecutions, infraTableSelectEnumMultikeyJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-update-and-index" || *mode == "infra-table-update-and-index-diff" {
		trace, err := runInfraTableUpdateIndexScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-update-and-index-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableUpdateIndexJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableUpdateIndexJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableUpdateIndexJavaSources),
				splitMetadata(*javaExecutions, infraTableUpdateIndexJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-faf-execute-query" || *mode == "infra-table-faf-execute-query-diff" {
		trace, err := runInfraTableFAFScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-faf-execute-query-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableFAFJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableFAFJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableFAFJavaSources),
				splitMetadata(*javaExecutions, infraTableFAFJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-subquery" || *mode == "infra-table-subquery-diff" {
		trace, err := runInfraTableSubqueryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-subquery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableSubqueryJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableSubqueryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableSubqueryJavaSources),
				splitMetadata(*javaExecutions, infraTableSubqueryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-context" || *mode == "infra-table-context-diff" {
		trace, err := runInfraTableContextScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-context-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableContextJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableContextJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableContextJavaSources),
				splitMetadata(*javaExecutions, infraTableContextJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-nwtable-context" || *mode == "infra-nwtable-context-diff" {
		trace, err := runInfraNWTableContextScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-nwtable-context-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, infraNWTableContextJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraNWTableContextJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraNWTableContextJavaSources),
				splitMetadata(*javaExecutions, infraNWTableContextJavaExecutions), scenario, trace, normalizeInfraNWTableContextTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-contained-event-example" || *mode == "epl-contained-event-example-diff" {
		trace, err := runEPLContainedEventExampleScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-contained-event-example-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, eplContainedEventExampleJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplContainedEventExampleJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplContainedEventExampleJavaSources),
				splitMetadata(*javaExecutions, eplContainedEventExampleJavaExecutions), scenario, trace, normalizeEPLContainedEventExampleTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-pattern-event-properties" || *mode == "epl-other-pattern-event-properties-diff" {
		trace, err := runEPLOtherPatternEventPropertiesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-pattern-event-properties-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, eplOtherPatternEventPropertiesJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherPatternEventPropertiesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherPatternEventPropertiesJavaSources),
				splitMetadata(*javaExecutions, eplOtherPatternEventPropertiesJavaExecutions), scenario, trace, normalizeEPLOtherPatternEventPropertiesTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-invalid" || *mode == "infra-table-invalid-diff" {
		trace, err := runInfraTableInvalidScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-invalid-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableInvalidJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableInvalidJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableInvalidJavaSources),
				splitMetadata(*javaExecutions, infraTableInvalidJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "infra-table-count-min-sketch" || *mode == "infra-table-count-min-sketch-diff" {
		trace, err := runInfraTableCMSScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "infra-table-count-min-sketch-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, infraTableCMSJavaCommit,
				splitMetadata(*javaRuntimeIDs, infraTableCMSJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, infraTableCMSJavaSources),
				splitMetadata(*javaExecutions, infraTableCMSJavaExecutions), scenario, trace)
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
	if *mode == "resultset-orderby-aggregate-grouped" || *mode == "resultset-orderby-aggregate-grouped-diff" {
		trace, err := runResultsetOrderbyAggregateGroupedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-orderby-aggregate-grouped-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOrderbyAggregateGroupedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOrderbyAggregateGroupedJavaSources),
				splitMetadata(*javaExecutions, resultsetOrderbyAggregateGroupedJavaExecutions), scenario, trace)
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
	if *mode == "view-parameterized-by-context" || *mode == "view-parameterized-by-context-diff" {
		trace, err := runViewParameterizedByContextScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-parameterized-by-context-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewParameterizedByContextJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewParameterizedByContextJavaSources),
				splitMetadata(*javaExecutions, viewParameterizedByContextJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-intersect" || *mode == "view-intersect-diff" {
		trace, err := runViewIntersectScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-intersect-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewIntersectJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, viewIntersectJavaSources),
				splitMetadata(*javaExecutions, viewIntersectJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-time-batch" || *mode == "view-time-batch-diff" {
		trace, err := runViewTimeBatchScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-time-batch-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewTimeBatchJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewTimeBatchJavaSources),
				splitMetadata(*javaExecutions, viewTimeBatchJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-length-win-property-detail" || *mode == "view-length-win-property-detail-diff" {
		trace, err := runViewLengthWinPropertyDetailScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-length-win-property-detail-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewLengthWinPropertyDetailJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewLengthWinPropertyDetailJavaSources),
				splitMetadata(*javaExecutions, viewLengthWinPropertyDetailJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-first-time" || *mode == "view-first-time-diff" {
		trace, err := runViewFirstTimeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-first-time-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewFirstTimeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewFirstTimeJavaSources),
				splitMetadata(*javaExecutions, viewFirstTimeJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-time-win" || *mode == "view-time-win-diff" {
		trace, err := runViewTimeWinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-time-win-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewTimeWinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewTimeWinJavaSources),
				splitMetadata(*javaExecutions, viewTimeWinJavaExecutions), scenario, trace)
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
	if *mode == "event-map-properties" || *mode == "event-map-properties-diff" {
		trace, err := runEventMapPropertiesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-map-properties-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, eventMapPropertiesJavaCommit,
				splitMetadata(*javaRuntimeIDs, eventMapPropertiesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventMapPropertiesSources),
				splitMetadata(*javaExecutions, eventMapPropertiesJavaExecutions), scenario, trace, normalizeEventMapPropertiesTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-infra-getter-dynamic" || *mode == "event-infra-getter-dynamic-diff" {
		trace, err := runEventInfraGetterDynamicScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-infra-getter-dynamic-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventInfraGetterDynamicJavaCommit,
				splitMetadata(*javaRuntimeIDs, eventInfraGetterDynamicJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventInfraGetterDynamicJavaSources),
				splitMetadata(*javaExecutions, eventInfraGetterDynamicJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-infra-getter-nested" || *mode == "event-infra-getter-nested-diff" {
		trace, err := runEventInfraGetterNestedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-infra-getter-nested-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventInfraGetterNestedJavaCommit,
				splitMetadata(*javaRuntimeIDs, eventInfraGetterNestedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventInfraGetterNestedJavaSources),
				splitMetadata(*javaExecutions, eventInfraGetterNestedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-infra-545" || *mode == "event-infra-545-diff" {
		trace, err := runEventInfra545Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-infra-545-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventInfra545JavaCommit,
				splitMetadata(*javaRuntimeIDs, eventInfra545JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventInfra545JavaSources),
				splitMetadata(*javaExecutions, eventInfra545JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-render-546" || *mode == "event-render-546-diff" {
		trace, err := runEventRender546Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-render-546-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventRender546JavaCommit,
				splitMetadata(*javaRuntimeIDs, eventRender546JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventRender546JavaSources),
				splitMetadata(*javaExecutions, eventRender546JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-map-nested-547" || *mode == "event-map-nested-547-diff" {
		trace, err := runEventMapNested547Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-map-nested-547-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventMapNested547JavaCommit,
				splitMetadata(*javaRuntimeIDs, eventMapNested547JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventMapNested547JavaSources),
				splitMetadata(*javaExecutions, eventMapNested547JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-avro-hook-548" || *mode == "event-avro-hook-548-diff" {
		trace, err := runEventAvroHook548Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-avro-hook-548-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventAvroHook548JavaCommit,
				splitMetadata(*javaRuntimeIDs, eventAvroHook548JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventAvroHook548JavaSources),
				splitMetadata(*javaExecutions, eventAvroHook548JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-with-minmax-549" || *mode == "expr-dt-with-minmax-549-diff" {
		trace, err := runExprDTWithMinMax549Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-with-minmax-549-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDTWithMinMax549JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTWithMinMax549JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTWithMinMax549JavaSources),
				splitMetadata(*javaExecutions, exprDTWithMinMax549JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-set-nested-550" || *mode == "expr-dt-set-nested-550-diff" {
		trace, err := runExprDTSetNested550Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-set-nested-550-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDTSetNested550JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTSetNested550JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTSetNested550JavaSources),
				splitMetadata(*javaExecutions, exprDTSetNested550JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-format-551" || *mode == "expr-dt-format-551-diff" {
		trace, err := runExprDTFormat551Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-format-551-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDTFormat551JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTFormat551JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTFormat551JavaSources),
				splitMetadata(*javaExecutions, exprDTFormat551JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-remainder-552" || *mode == "expr-dt-remainder-552-diff" {
		trace, err := runExprDTRemainder552Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-remainder-552-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDTRemainder552JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTRemainder552JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTRemainder552JavaSources),
				splitMetadata(*javaExecutions, exprDTRemainder552JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-dt-tail-553" || *mode == "expr-dt-tail-553-diff" {
		trace, err := runExprDTTail553Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-dt-tail-553-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDTTail553JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDTTail553JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDTTail553JavaSources),
				splitMetadata(*javaExecutions, exprDTTail553JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-enum-remainder-554" || *mode == "expr-enum-remainder-554-diff" {
		trace, err := runExprEnumRemainder554Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-enum-remainder-554-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprEnumRemainder554JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprEnumRemainder554JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprEnumRemainder554JavaSources),
				splitMetadata(*javaExecutions, exprEnumRemainder554JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-define-locreport-555" || *mode == "expr-define-locreport-555-diff" {
		trace, err := runExprDefineLocReport555Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-define-locreport-555-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprDefineLocReport555JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprDefineLocReport555JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprDefineLocReport555JavaSources),
				splitMetadata(*javaExecutions, exprDefineLocReport555JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-script-threading-556" || *mode == "expr-script-threading-556-diff" {
		trace, err := runExprScriptThreading556Scenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-script-threading-556-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprScriptThreading556JavaCommit,
				splitMetadata(*javaRuntimeIDs, exprScriptThreading556JavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprScriptThreading556JavaSources),
				splitMetadata(*javaExecutions, exprScriptThreading556JavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-infra-property-dynamic" || *mode == "event-infra-property-dynamic-diff" {
		trace, err := runEventInfraPropertyDynamicScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-infra-property-dynamic-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventInfraPropertyDynamicJavaCommit,
				splitMetadata(*javaRuntimeIDs, eventInfraPropertyDynamicJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventInfraPropertyDynamicJavaSources),
				splitMetadata(*javaExecutions, eventInfraPropertyDynamicJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-infra-property-non-dynamic" || *mode == "event-infra-property-non-dynamic-diff" {
		trace, err := runEventInfraPropertyNonDynamicScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-infra-property-non-dynamic-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eventInfraPropertyNonDynamicJavaCommit,
				splitMetadata(*javaRuntimeIDs, eventInfraPropertyNonDynamicJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventInfraPropertyNonDynamicJavaSources),
				splitMetadata(*javaExecutions, eventInfraPropertyNonDynamicJavaExecutions), scenario, trace)
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
	if *mode == "resultset-querytype-row-for-all-static-method-double-nested" || *mode == "resultset-querytype-row-for-all-static-method-double-nested-diff" {
		trace, err := runResultSetQueryTypeRowForAllStaticMethodDoubleNestedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-static-method-double-nested-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllStaticMethodDoubleNestedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-for-all-select-avg-expr-std-group-by" || *mode == "resultset-querytype-row-for-all-select-avg-expr-std-group-by-diff" {
		trace, err := runResultSetQueryTypeRowForAllSelectAvgExprStdGroupByScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-select-avg-expr-std-group-by-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllSelectAvgExprStdGroupByJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-for-all-select-avg-std-group-by-uni" || *mode == "resultset-querytype-row-for-all-select-avg-std-group-by-uni-diff" {
		trace, err := runResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-select-avg-std-group-by-uni-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllSelectAvgStdGroupByUniJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-for-all-having-avg-group-window" || *mode == "resultset-querytype-row-for-all-having-avg-group-window-diff" {
		trace, err := runResultSetQueryTypeRowForAllHavingAvgScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-having-avg-group-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllHavingAvgJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllHavingAvgJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllHavingAvgJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllHavingAvgJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-for-all-having-sum-join" || *mode == "resultset-querytype-row-for-all-having-sum-join-diff" {
		trace, err := runResultSetQueryTypeRowForAllHavingSumJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-having-sum-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllHavingSumJoinJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllHavingSumJoinJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllHavingSumJoinJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllHavingSumJoinJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-for-all-having-sum" || *mode == "resultset-querytype-row-for-all-having-sum-diff" {
		trace, err := runResultSetQueryTypeRowForAllHavingSumScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-having-sum-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllHavingSumJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllHavingSumJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllHavingSumJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllHavingSumJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-window" || *mode == "resultset-aggregate-window-diff" {
		trace, err := runResultSetAggregateWindowScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-window-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateWindowJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateWindowJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{resultsetAggregateWindowSource}),
				splitMetadata(*javaExecutions, resultsetAggregateWindowJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-for-all" || *mode == "resultset-querytype-row-for-all-diff" {
		trace, err := runResultSetQueryTypeRowForAllScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-for-all-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRowForAllJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRowForAllJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRowForAllJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRowForAllJavaExecutions), scenario, trace)
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
	if *mode == "resultset-querytype-row-per-group-having" || *mode == "resultset-querytype-row-per-group-having-diff" {
		trace, err := runResultsetQueryTypeRowPerGroupHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-per-group-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetQueryTypeRowPerGroupHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetQueryTypeRowPerGroupHavingJavaSources),
				splitMetadata(*javaExecutions, resultsetQueryTypeRowPerGroupHavingJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-row-per-event" || *mode == "resultset-querytype-row-per-event-diff" {
		trace, err := runResultsetQueryTypeRowPerEventScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-row-per-event-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetQueryTypeRowPerEventJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetQueryTypeRowPerEventJavaSources),
				splitMetadata(*javaExecutions, resultsetQueryTypeRowPerEventJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-by" || *mode == "resultset-querytype-local-group-by-diff" {
		trace, err := runResultSetQueryTypeLocalGroupByScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-by-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupByJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupByJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupByJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-ungrouped" || *mode == "resultset-querytype-local-group-ungrouped-diff" {
		trace, err := runResultSetQueryTypeLocalGroupUngroupedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-ungrouped-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupUngroupedJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupUngroupedJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupUngroupedJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-extended" || *mode == "resultset-querytype-local-group-extended-diff" {
		trace, err := runResultSetQueryTypeLocalGroupExtendedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-extended-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupExtendedJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupExtendedJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupExtendedJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-grouped" || *mode == "resultset-querytype-local-group-grouped-diff" {
		trace, err := runResultSetQueryTypeLocalGroupGroupedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-grouped-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupGroupedJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupGroupedJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupGroupedJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-ungrouped-agg" || *mode == "resultset-querytype-local-group-ungrouped-agg-diff" {
		trace, err := runResultSetQueryTypeLocalGroupUngroupedAggScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-ungrouped-agg-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupUngroupedAggJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupUngroupedAggJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupUngroupedAggJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-row-remove" || *mode == "resultset-querytype-local-group-row-remove-diff" {
		trace, err := runResultSetQueryTypeLocalGroupRowRemoveScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-row-remove-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupRowRemoveJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupRowRemoveJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupRowRemoveJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-closure" || *mode == "resultset-querytype-local-group-closure-diff" {
		trace, err := runResultSetQueryTypeLocalGroupClosureScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-closure-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupClosureJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupClosureJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupClosureJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-context-terminated" || *mode == "resultset-querytype-local-group-context-terminated-diff" {
		trace, err := runResultSetQueryTypeLocalGroupCtxTermScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-context-terminated-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupCtxTermJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupCtxTermJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupCtxTermJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-keys" || *mode == "resultset-querytype-local-group-keys-diff" {
		trace, err := runResultSetQueryTypeLocalGroupKeysScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-keys-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupKeysJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupKeysJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupKeysJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-local-group-solution-pattern" || *mode == "resultset-querytype-local-group-solution-pattern-diff" {
		trace, err := runResultSetQueryTypeLocalGroupSolutionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-local-group-solution-pattern-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeLocalGroupSolutionJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeLocalGroupSolutionSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeLocalGroupSolutionJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "orderby-rowperevent-agg" || *mode == "orderby-rowperevent-agg-diff" {
		trace, err := runOrderByRowPerEventAggScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "orderby-rowperevent-agg-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, orderbyRowPerEventAggJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, orderbyRowPerEventAggSources),
				splitMetadata(*javaExecutions, orderbyRowPerEventAggJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "orderby-rowperevent-agg-join" || *mode == "orderby-rowperevent-agg-join-diff" {
		trace, err := runOrderByRowPerEventAggJoinScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "orderby-rowperevent-agg-join-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, orderbyRowPerEventAggJoinJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, orderbyRowPerEventAggJoinSources),
				splitMetadata(*javaExecutions, orderbyRowPerEventAggJoinJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "orderby-rowperevent-iterator" || *mode == "orderby-rowperevent-iterator-diff" {
		trace, err := runOrderByRowPerEventIteratorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "orderby-rowperevent-iterator-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, orderbyRowPerEventIteratorJavaRuntimeIDs()),
				splitMetadata(*javaSourceFiles, orderbyRowPerEventIteratorSources),
				splitMetadata(*javaExecutions, orderbyRowPerEventIteratorJavaExecutions()), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-limit-context-grouped" || *mode == "resultset-output-limit-row-limit-context-grouped-diff" {
		trace, err := runResultsetOutputLimitRowLimitContextGroupedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-limit-context-grouped-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowLimitContextGroupedJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowLimitContextGroupedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowLimitContextGroupedJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowLimitContextGroupedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-limit" || *mode == "resultset-output-limit-row-limit-diff" {
		trace, err := runResultsetOutputLimitRowLimitScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-limit-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowLimitJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowLimitJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowLimitJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowLimitJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-limit-negative-rowcount" || *mode == "resultset-output-limit-row-limit-negative-rowcount-diff" {
		trace, err := runResultsetOutputLimitRowLimitNegativeRowcountScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-limit-negative-rowcount-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowLimitJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowLimitNegativeRowcountJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowLimitNegativeRowcountJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowLimitNegativeRowcountJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-limit-invalid" || *mode == "resultset-output-limit-row-limit-invalid-diff" {
		trace, err := runResultsetOutputLimitRowLimitInvalidScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-limit-invalid-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowLimitInvalidJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowLimitInvalidJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowLimitInvalidJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowLimitInvalidJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-limit-variable" || *mode == "resultset-output-limit-row-limit-variable-diff" {
		trace, err := runResultsetOutputLimitRowLimitVariableScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-limit-variable-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowLimitVariableJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowLimitVariableJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowLimitVariableJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowLimitVariableJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-events" || *mode == "resultset-output-limit-row-per-group-events-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupEventsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-events-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupEventsJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupEventsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupEventsJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupEventsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-multikey" || *mode == "resultset-output-limit-row-per-group-multikey-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupMultikeyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-multikey-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupMultikeyJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupMultikeyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupMultikeyJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupMultikeyJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-last" || *mode == "resultset-output-limit-row-per-group-last-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupLastScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-last-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupLastJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupLastJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupLastJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupLastJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-having-first-snap" || *mode == "resultset-output-limit-row-per-group-having-first-snap-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupHavingFirstSnapScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-having-first-snap-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupHavingFirstSnapJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupHavingFirstSnapJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupHavingFirstSnapJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupHavingFirstSnapJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-all" || *mode == "resultset-output-limit-row-per-group-all-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupAllScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-all-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupAllJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupAllJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupAllJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupAllJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-none" || *mode == "resultset-output-limit-row-per-group-none-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupNoneScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-none-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupNoneDefaultJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupNoneJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupNoneDefaultJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupNoneJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-default" || *mode == "resultset-output-limit-row-per-group-default-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupDefaultScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-default-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupNoneDefaultJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupDefaultJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupNoneDefaultJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupDefaultJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-outputlimit-simple-none" || *mode == "resultset-outputlimit-simple-none-diff" {
		trace, err := runResultSetOutputLimitSimpleNoneScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-outputlimit-simple-none-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitSimpleNoneJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitSimpleNoneJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitSimpleNoneJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitSimpleNoneJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-enum-sumof-remainder" || *mode == "expr-enum-sumof-remainder-diff" {
		trace, err := runExprEnumSumOfRemainderScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-enum-sumof-remainder-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, exprEnumSumOfRemainderJavaCommit,
				splitMetadata(*javaRuntimeIDs, exprEnumSumOfRemainderJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprEnumSumOfRemainderJavaSources),
				splitMetadata(*javaExecutions, exprEnumSumOfRemainderJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-row-per-group-first" || *mode == "resultset-output-limit-row-per-group-first-diff" {
		trace, err := runResultSetOutputLimitRowPerGroupFirstScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-row-per-group-first-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitRowPerGroupFirstJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitRowPerGroupFirstJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitRowPerGroupFirstJavaSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitRowPerGroupFirstJavaExecutions), scenario, trace,
				normalizeResultSetOutputLimitRowPerGroupFirstTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-output-limit-crontab-when-closure" || *mode == "resultset-output-limit-crontab-when-closure-diff" {
		trace, err := runResultSetOutputLimitCrontabWhenClosureScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-output-limit-crontab-when-closure-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetOutputLimitCrontabWhenClosureJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetOutputLimitCrontabWhenClosureJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetOutputLimitCrontabWhenClosureSources),
				splitMetadata(*javaExecutions, resultsetOutputLimitCrontabWhenClosureJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-invalid-closure" || *mode == "resultset-aggregate-invalid-closure-diff" {
		trace, err := runResultSetAggregateInvalidClosureScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-invalid-closure-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateInvalidClosureJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateInvalidClosureJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateInvalidClosureSources),
				splitMetadata(*javaExecutions, resultsetAggregateInvalidClosureJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-select-expr" || *mode == "epl-other-select-expr-diff" {
		trace, err := runEplOtherSelectExprScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-select-expr-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplOtherSelectExprJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherSelectExprJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherSelectExprJavaSources),
				splitMetadata(*javaExecutions, eplOtherSelectExprJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-plan-in-keyword" || *mode == "epl-other-plan-in-keyword-diff" {
		trace, err := runEplOtherPlanInKeywordScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-plan-in-keyword-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplOtherPlanInKeywordJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherPlanInKeywordJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherPlanInKeywordJavaSources),
				splitMetadata(*javaExecutions, eplOtherPlanInKeywordJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-for-group-delivery" || *mode == "epl-other-for-group-delivery-diff" {
		trace, err := runEplOtherForGroupDeliveryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-for-group-delivery-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplOtherForGroupDeliveryJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherForGroupDeliveryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherForGroupDeliveryJavaSources),
				splitMetadata(*javaExecutions, eplOtherForGroupDeliveryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-other-pattern-queries" || *mode == "epl-other-pattern-queries-diff" {
		trace, err := runEplOtherPatternQueriesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-other-pattern-queries-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplOtherPatternQueriesJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplOtherPatternQueriesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplOtherPatternQueriesJavaSources),
				splitMetadata(*javaExecutions, eplOtherPatternQueriesJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-first-last-event" || *mode == "view-first-last-event-diff" {
		trace, err := runViewFirstLastEventScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-first-last-event-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, viewFirstLastEventJavaCommit,
				splitMetadata(*javaRuntimeIDs, viewFirstLastEventJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewFirstLastEventJavaSources),
				splitMetadata(*javaExecutions, viewFirstLastEventJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-method-remainder" || *mode == "resultset-aggregate-method-remainder-diff" {
		trace, err := runResultSetAggregateMethodRemainderScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-method-remainder-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateMethodRemainderJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateMethodRemainderJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateMethodRemainderJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateMethodRemainderJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-remainder" || *mode == "resultset-aggregate-remainder-diff" {
		trace, err := runResultSetAggregateRemainderScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-remainder-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetAggregateRemainderJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateRemainderJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetAggregateRemainderJavaSources),
				splitMetadata(*javaExecutions, resultsetAggregateRemainderJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-rollup-having-orderby" || *mode == "resultset-rollup-having-orderby-diff" {
		trace, err := runResultSetRollupHavingOrderByScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-rollup-having-orderby-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultsetRollupHavingOrderByJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetRollupHavingOrderByJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetRollupHavingOrderByJavaSources),
				splitMetadata(*javaExecutions, resultsetRollupHavingOrderByJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-aggregate-grouped-having" || *mode == "resultset-querytype-aggregate-grouped-having-diff" {
		trace, err := runResultsetQueryTypeAggregateGroupedHavingScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-aggregate-grouped-having-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetQueryTypeAggregateGroupedHavingJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultsetQueryTypeAggregateGroupedHavingJavaSources),
				splitMetadata(*javaExecutions, resultsetQueryTypeAggregateGroupedHavingJavaExecutions), scenario, trace)
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
	if *mode == "event-objectarray-nested" || *mode == "event-objectarray-nested-diff" {
		trace, err := runEventObjectArrayNestedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-objectarray-nested-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, eventObjectArrayNestedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventObjectArrayNestedJavaSources),
				splitMetadata(*javaExecutions, eventObjectArrayNestedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "event-object-array-core" || *mode == "event-object-array-core-diff" {
		trace, err := runEventObjectArrayCoreScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "event-object-array-core-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, eventObjectArrayCoreJavaCommit,
				splitMetadata(*javaRuntimeIDs, eventObjectArrayCoreJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eventObjectArrayCoreSources),
				splitMetadata(*javaExecutions, eventObjectArrayCoreJavaExecutions), scenario, trace, normalizeEventObjectArrayCoreTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-after" || *mode == "rowrecog-after-diff" {
		trace, err := runRowRecogAfterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-after-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogAfterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogAfterJavaSources),
				splitMetadata(*javaExecutions, rowRecogAfterJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-repetition" || *mode == "rowrecog-repetition-diff" {
		trace, err := runRowRecogRepetitionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-repetition-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogRepetitionJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogRepetitionJavaSources),
				splitMetadata(*javaExecutions, rowRecogRepetitionJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-greedyness-ops" || *mode == "rowrecog-greedyness-ops-diff" {
		trace, err := runRowRecogGreedynessOpsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-greedyness-ops-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogGreedynessOpsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogGreedynessOpsJavaSources),
				splitMetadata(*javaExecutions, rowRecogGreedynessOpsJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-prev" || *mode == "rowrecog-prev-diff" {
		trace, err := runRowRecogPrevScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-prev-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogPrevJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogPrevJavaSources),
				splitMetadata(*javaExecutions, rowRecogPrevJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-interval" || *mode == "rowrecog-interval-diff" {
		trace, err := runRowRecogIntervalScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-interval-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogIntervalJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogIntervalJavaSources),
				splitMetadata(*javaExecutions, rowRecogIntervalJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rowrecog-multikey-warray" || *mode == "rowrecog-multikey-warray-diff" {
		trace, err := runRowRecogMultikeyWArrayScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rowrecog-multikey-warray-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, rowRecogMultikeyWArrayJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rowRecogMultikeyWArrayJavaSources),
				splitMetadata(*javaExecutions, rowRecogMultikeyWArrayJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "view-systime-trio" || *mode == "view-systime-trio-diff" {
		trace, err := runViewSystimeTrioScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "view-systime-trio-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, viewSystimeTrioJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, viewSystimeTrioJavaSources),
				splitMetadata(*javaExecutions, viewSystimeTrioJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-start-end-trio" || *mode == "context-start-end-trio-diff" {
		trace, err := runContextStartEndTrioScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-start-end-trio-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, contextStartEndTrioJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextStartEndTrioJavaSources),
				splitMetadata(*javaExecutions, contextStartEndTrioJavaExecutions), scenario, trace)
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
	if *mode == "expr-class-type-use" || *mode == "expr-class-type-use-diff" {
		trace, err := runExprClassTypeUseScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-class-type-use-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, exprClassTypeUseJavaCommit,
				splitMetadata(*javaRuntimeIDs, exprClassTypeUseJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprClassTypeUseJavaSources),
				splitMetadata(*javaExecutions, exprClassTypeUseJavaExecutions), scenario, trace,
				normalizeExprClassTypeUseTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-class-for-epl-objects" || *mode == "expr-class-for-epl-objects-diff" {
		trace, err := runExprClassForEPLObjectsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-class-for-epl-objects-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, exprClassForEPLObjectsJavaCommit,
				splitMetadata(*javaRuntimeIDs, exprClassForEPLObjectsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprClassForEPLObjectsJavaSources),
				splitMetadata(*javaExecutions, exprClassForEPLObjectsJavaExecutions), scenario, trace,
				normalizeExprClassForEPLObjectsTrace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-class-class-dependency" || *mode == "expr-class-class-dependency-diff" {
		trace, err := runExprClassClassDependencyScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-class-class-dependency-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, exprClassClassDependencyJavaCommit,
				splitMetadata(*javaRuntimeIDs, exprClassClassDependencyJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprClassClassDependencyJavaSources),
				splitMetadata(*javaExecutions, exprClassClassDependencyJavaExecutions), scenario, trace,
				normalizeExprClassClassDependencyTrace)
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
	if *mode == "expr-filter-opt-lkup-limited-remaining" || *mode == "expr-filter-opt-lkup-limited-remaining-diff" {
		trace, err := runEfolrScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-filter-opt-lkup-limited-remaining-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, efolrJavaCommit,
				splitMetadata(*javaRuntimeIDs, efolrJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, efolrJavaSources),
				splitMetadata(*javaExecutions, efolrJavaExecutions), scenario, trace)
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
	if *mode == "expr-filter-in-and-between" || *mode == "expr-filter-in-and-between-diff" {
		trace, err := runEfabScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-filter-in-and-between-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, efabJavaCommit,
				splitMetadata(*javaRuntimeIDs, efabJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, efabSources),
				splitMetadata(*javaExecutions, efabJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "expr-filter-expressions" || *mode == "expr-filter-expressions-diff" {
		trace, err := runEfeScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-filter-expressions-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, efeJavaCommit,
				splitMetadata(*javaRuntimeIDs, efeJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, efeSources),
				splitMetadata(*javaExecutions, efeJavaExecutions), scenario, trace)
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
	if *mode == "epl-variables-create" || *mode == "epl-variables-create-diff" {
		trace, err := runEplVariablesCreateScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-variables-create-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplVariablesCreateJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplVariablesCreateJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplVariablesCreateSources),
				splitMetadata(*javaExecutions, eplVariablesCreateJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-variables" || *mode == "context-variables-diff" {
		trace, err := runContextVariablesScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-variables-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextVariablesJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextVariablesJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextVariablesSources),
				splitMetadata(*javaExecutions, contextVariablesJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-declared-expression" || *mode == "context-declared-expression-diff" {
		trace, err := runContextDeclaredExpressionScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-declared-expression-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextDeclaredExpressionJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextDeclaredExpressionJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextDeclaredExpressionSources),
				splitMetadata(*javaExecutions, contextDeclaredExpressionJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-admin-listen" || *mode == "context-admin-listen-diff" {
		trace, err := runContextAdminListenScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-admin-listen-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextAdminListenJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextAdminListenJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextAdminListenSources),
				splitMetadata(*javaExecutions, contextAdminListenJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-category" || *mode == "context-category-diff" {
		trace, err := runContextCategoryScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-category-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextCategoryJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextCategoryJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextCategorySources),
				splitMetadata(*javaExecutions, contextCategoryJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-selection-faf" || *mode == "context-selection-faf-diff" {
		trace, err := runContextSelectionFAFScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-selection-faf-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextSelectionFAFJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextSelectionFAFJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextSelectionFAFSources),
				splitMetadata(*javaExecutions, contextSelectionFAFJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "context-selection-faf-nested" || *mode == "context-selection-faf-nested-diff" {
		trace, err := runContextSelectionFAFNestedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "context-selection-faf-nested-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, contextSelectionFAFNestedJavaCommit,
				splitMetadata(*javaRuntimeIDs, contextSelectionFAFNestedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, contextSelectionFAFNestedSources),
				splitMetadata(*javaExecutions, contextSelectionFAFNestedJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "epl-variables-event-typed" || *mode == "epl-variables-event-typed-diff" {
		trace, err := runEplVariablesEventTypedScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "epl-variables-event-typed-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, eplVariablesEventTypedJavaCommit,
				splitMetadata(*javaRuntimeIDs, eplVariablesEventTypedJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, eplVariablesEventTypedSources),
				splitMetadata(*javaExecutions, eplVariablesEventTypedJavaExecutions), scenario, trace)
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
			runtimes := rollupDimensionalityJavaRuntimeIDs
			executions := rollupDimensionalityJavaExecutions
			if scenario.ID == rollupDimensionalityDedicatedID {
				runtimes = rollupDimensionalityDedicatedJavaRuntimeIDs
				executions = rollupDimensionalityDedicatedJavaExecutions
			}
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				runtimes, rollupDimensionalityJavaSources, executions, scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-grouping-funcs-faf-dedicated" || *mode == "rollup-grouping-funcs-faf-dedicated-diff" {
		trace, err := runRollupGroupingFAFScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-grouping-funcs-faf-dedicated-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, rollupGroupingFAFJavaCommit,
				splitMetadata(*javaRuntimeIDs, rollupGroupingFAFJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupGroupingFAFJavaSources),
				splitMetadata(*javaExecutions, rollupGroupingFAFJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-rollup-having-iterator" || *mode == "resultset-querytype-rollup-having-iterator-diff" {
		trace, err := runResultSetQueryTypeRollupHavingIteratorScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-rollup-having-iterator-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRollupHavingIteratorJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRollupHavingIteratorJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRollupHavingIteratorJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRollupHavingIteratorJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-querytype-rollup-orderby-unidirectional" || *mode == "resultset-querytype-rollup-orderby-unidirectional-diff" {
		trace, err := runResultSetQueryTypeRollupOrderByUnidirectionalScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-querytype-rollup-orderby-unidirectional-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, resultSetQueryTypeRollupOrderByUnidirectionalJavaCommit,
				splitMetadata(*javaRuntimeIDs, resultSetQueryTypeRollupOrderByUnidirectionalJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, resultSetQueryTypeRollupOrderByUnidirectionalJavaSources),
				splitMetadata(*javaExecutions, resultSetQueryTypeRollupOrderByUnidirectionalJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "rollup-grouping-funcs-dedicated" || *mode == "rollup-grouping-funcs-dedicated-diff" {
		trace, err := runRollupGroupingFuncsScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "rollup-grouping-funcs-dedicated-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, rollupGroupingFuncsJavaCommit,
				splitMetadata(*javaRuntimeIDs, rollupGroupingFuncsJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, rollupGroupingFuncsJavaSources),
				splitMetadata(*javaExecutions, rollupGroupingFuncsJavaExecutions), scenario, trace)
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
	if *mode == "resultset-aggregate-filtered-w-math-context" || *mode == "resultset-aggregate-filtered-w-math-context-diff" {
		trace, err := runResultSetAggregateFilteredWMathContextScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-filtered-w-math-context-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFilteredWMathContextJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{resultsetAggregateFilteredWMathContextSource}),
				splitMetadata(*javaExecutions, resultsetAggregateFilteredWMathContextJavaExecutions), scenario, trace)
		}
		if err := json.NewEncoder(stdout).Encode(trace); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if *mode == "resultset-aggregate-filter-named-parameter" || *mode == "resultset-aggregate-filter-named-parameter-diff" {
		trace, err := runResultSetAggregateFilterNamedParameterScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "resultset-aggregate-filter-named-parameter-diff" {
			return runDifferentialMode(stdout, stderr, *javaTracePath, *evidencePath, *javaCommit,
				splitMetadata(*javaRuntimeIDs, resultsetAggregateFilterNamedParameterJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, []string{resultsetAggregateFilterNamedParameterSource}),
				splitMetadata(*javaExecutions, resultsetAggregateFilterNamedParameterJavaExecutions), scenario, trace)
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
	if *mode == "expr-enum-select-from" || *mode == "expr-enum-select-from-diff" {
		trace, err := runExprEnumSelectFromScenario(context.Background(), scenario)
		if err != nil {
			return fail(stderr, err)
		}
		if *mode == "expr-enum-select-from-diff" {
			return runDifferentialModeWithNormalizer(stdout, stderr, *javaTracePath, *evidencePath, exprEnumSelectFromJavaCommit,
				splitMetadata(*javaRuntimeIDs, exprEnumSelectFromJavaRuntimeIDs),
				splitMetadata(*javaSourceFiles, exprEnumSelectFromJavaSources),
				splitMetadata(*javaExecutions, exprEnumSelectFromJavaExecutions), scenario, trace,
				normalizeExprEnumSelectFromTrace)
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
	return runDifferentialModeWithNormalizers(stdout, stderr, javaTracePath, evidencePath, javaCommit,
		runtimeIDs, sourceFiles, executions, scenario, goTrace, normalize, normalize)
}

func runDifferentialModeWithGoNormalizer(stdout, stderr io.Writer, javaTracePath, evidencePath, javaCommit string, runtimeIDs, sourceFiles, executions []string, scenario compat.Scenario, goTrace compat.Trace, normalize func(compat.Trace) compat.Trace) int {
	return runDifferentialModeWithNormalizers(stdout, stderr, javaTracePath, evidencePath, javaCommit,
		runtimeIDs, sourceFiles, executions, scenario, goTrace, nil, normalize)
}

func runDifferentialModeWithNormalizers(stdout, stderr io.Writer, javaTracePath, evidencePath, javaCommit string, runtimeIDs, sourceFiles, executions []string, scenario compat.Scenario, goTrace compat.Trace, normalizeJava, normalizeGo func(compat.Trace) compat.Trace) int {
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
	if normalizeJava != nil {
		javaTrace = normalizeJava(javaTrace)
	}
	if normalizeGo != nil {
		goTrace = normalizeGo(goTrace)
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
