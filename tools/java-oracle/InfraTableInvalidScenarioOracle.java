import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.compiler.ConfigurationCompilerPlugInAggregationMultiFunction;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.suite.infra.tbl.InfraTableInvalid;
import com.espertech.esper.regressionlib.support.extend.aggfunc.SupportCountBackAggregationFunctionForge;
import com.espertech.esper.regressionlib.support.extend.aggmultifunc.SupportAggMFMultiRTForge;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraTableInvalid (ords 0-3).  Four cases replay on four
 * fresh runtimes; every probe is a compile-time rejection and no events are
 * sent:
 *
 * agg-match-single-func (ordinal 0, InfraInvalidAggMatchSingleFunc): 43
 * tryInvalidAggMatch probes.  Each probe deploys
 * `@name('create') @public create table var1(value <declared>)` with the
 * runtime path, compiles `into table var1 select <provided> as value from
 * SupportBean[#time(1000)]` expecting a compile error, then undeploys the
 * 'create' module.  12 probes pin startsWith prefixes; 31 null-message
 * probes assert contains("Incompatible aggregation function for table").
 *
 * agg-match-multi-func (ordinal 1, InfraInvalidAggMatchMultiFunc): 6
 * tryInvalidAggMatch probes, all unbound (#time(1000)): window(*) @type
 * event-type mismatch, sorted() sort-expression rejections and the se1()
 * plug-in name mismatch.
 *
 * annotations (ordinal 2, InfraInvalidAnnotations): 5 tryInvalidCompile
 * probes (path-less) over table-column annotations: unknown annotation,
 * missing value, duplicate annotation, non-string value, unknown event
 * type.
 *
 * invalid (ordinal 3, InfraInvalid): 12 fixture deploys (11 with the
 * runtime path; the objectarray schema deploys without it, mirroring
 * env.compileDeploy(epl)) followed by 50 tryInvalidCompile(path, ...)
 * probes across declaration, into-table, consumption and table-misuse
 * surfaces, then undeployAll.
 *
 * Assertion encoding: expectError pins a startsWith prefix, expectContains
 * pins a contains substring, and both empty mirrors assertMessage("skip").
 */
public final class InfraTableInvalidScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-invalid";
    private static final String DESCRIPTION =
            "InfraTableInvalid (ords 0-3): pure-invalidity replay. InfraInvalidAggMatchSingleFunc "
                    + "runs 43 tryInvalidAggMatch probes (declared vs provided aggregation signature "
                    + "mismatches: parameter type, distinct, filter, ignore-nulls, min/max direction, "
                    + "nth size, rate interval, ever direction, plug-in names); "
                    + "InfraInvalidAggMatchMultiFunc runs 6 unbound probes (window(*) @type event-type "
                    + "mismatch, sorted() sort-expression rejections, se1() plug-in name mismatch); "
                    + "InfraInvalidAnnotations runs 5 path-less annotation probes; InfraInvalid deploys "
                    + "12 fixtures then runs 50 path-ful probes across declaration, into-table, "
                    + "consumption and table-misuse surfaces. Zero events are sent.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableInvalid.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-70e525638c5fbaf37752",
            "java-runtime-7c97277eedfa76b7e0cf",
            "java-runtime-fd2a7de8e5606de63b80",
            "java-runtime-b9b6435f81135ce15040"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraInvalidAggMatchSingleFunc",
            "InfraInvalidAggMatchMultiFunc",
            "InfraInvalidAnnotations",
            "InfraInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-dc8058ccbd099b1d1361",
            "java-ecf59e6ef7727ab987fa",
            "java-53cb0410fe21932803bc",
            "java-c44ff6d4e86c09522b35"
    };
    private static final String[] JAVA_FLAGS = {"INVALIDITY"};
    private static final String[] CASES = {
            "agg-match-single-func",
            "agg-match-multi-func",
            "annotations",
            "invalid"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] CASE_OBSERVATIONS = {
            "compile-error; 43 tryInvalidAggMatch probes each deploy "
                    + "@name('create') @public create table var1(value <declared>) with the runtime "
                    + "path, compile the into-table probe against the path and undeploy the 'create' "
                    + "module; 12 probes pin startsWith prefixes and 31 null-message probes pin "
                    + "contains 'Incompatible aggregation function for table'",
            "compile-error; 6 tryInvalidAggMatch probes, all unbound (#time(1000)): window(*) @type "
                    + "event-type mismatch, sorted() sort-expression rejections and the se1() plug-in "
                    + "name mismatch",
            "compile-error; 5 path-less tryInvalidCompile probes over table-column annotations: "
                    + "unknown annotation, missing value, duplicate annotation, non-string value and "
                    + "unknown event type",
            "compile-error; 12 fixture deploys (the objectarray schema deploys without the runtime "
                    + "path, mirroring env.compileDeploy(epl)) then 50 path-ful tryInvalidCompile "
                    + "probes across declaration, into-table, consumption and table-misuse surfaces, "
                    + "then undeployAll"
    };

    private static final String CONTAINS_INCOMPATIBLE =
            "Incompatible aggregation function for table";

    /** One tryInvalidAggMatch probe: declared signature, unbound flag, provided expression. */
    private static final class AggProbe {
        final String label;
        final String declared;
        final boolean unbound;
        final String provided;
        final String expectError;
        final String expectContains;

        AggProbe(String label, String declared, boolean unbound, String provided,
                 String expectError, String expectContains) {
            this.label = label;
            this.declared = declared;
            this.unbound = unbound;
            this.provided = provided;
            this.expectError = expectError;
            this.expectContains = expectContains;
        }

        String createEpl() {
            return "@name('create') @public create table var1(value " + declared + ")";
        }

        String probeEpl() {
            return "into table var1 select " + provided + " as value from SupportBean"
                    + (unbound ? "#time(1000)" : "");
        }
    }

    // Verbatim transcriptions of InfraInvalidAggMatchSingleFunc's 43
    // tryInvalidAggMatch calls (ord 0) in source order.
    private static final AggProbe[] SINGLE_FUNC_PROBES = {
            new AggProbe("sum-param-type", "sum(double)", false, "sum(intPrimitive)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double)' and received 'sum(intPrimitive)': The required parameter type is Double and provided is Integer [", ""),
            new AggProbe("sum-name-mismatch", "sum(double)", false, "count(*)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double)' and received 'count(*)': The table declares 'sum(double)' and provided is 'count(*)'", ""),
            new AggProbe("sum-filter-provided", "sum(double)", false, "sum(doublePrimitive, theString='a')",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double)' and received 'sum(doublePrimitive,theString=\"a\")': The aggregation declares no filter expression and provided is a filter expression [", ""),
            new AggProbe("sum-filter-declared", "sum(double, boolean)", false, "sum(doublePrimitive)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'sum(double,boolean)' and received 'sum(doublePrimitive)': The aggregation declares a filter expression and provided is no filter expression [", ""),
            new AggProbe("count-name-mismatch", "count(*)", false, "sum(intPrimitive)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'count(*)' and received 'sum(intPrimitive)': The table declares 'count(*)' and provided is 'sum(intPrimitive)'", ""),
            new AggProbe("count-distinct-provided", "count(*)", false, "count(distinct intPrimitive)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'count(*)' and received 'count(distinct intPrimitive)': The aggregation declares no distinct and provided is a distinct [", ""),
            new AggProbe("count-distinct-multikey", "count(*)", false, "count(distinct intPrimitive, boolPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("count-distinct-param-type", "count(distinct int)", false, "count(distinct doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("count-ignore-nulls", "count(int)", false, "count(*)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'count(int)' and received 'count(*)': The aggregation declares ignore nulls and provided is no ignore nulls [", ""),
            new AggProbe("avg-name-mismatch", "avg(int)", false, "sum(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("avg-param-type", "avg(int)", false, "avg(longPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("avg-filter-provided", "avg(int)", false, "avg(intPrimitive, boolPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("avg-distinct-provided", "avg(int)", false, "avg(distinct intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("max-direction", "max(int)", false, "min(intPrimitive)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'max(int)' and received 'min(intPrimitive)': The aggregation declares max and provided is min [", ""),
            new AggProbe("min-name-mismatch", "min(int)", false, "avg(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("min-param-type", "min(int)", false, "min(doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("min-filter-provided", "min(int)", false, "fmin(intPrimitive, theString='a')",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'min(int)' and received 'min(intPrimitive,theString=\"a\")': The aggregation declares no filter expression and provided is a filter expression [", ""),
            new AggProbe("stddev-name-mismatch", "stddev(int)", false, "avg(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("stddev-param-type", "stddev(int)", false, "stddev(doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("stddev-filter-provided", "stddev(int)", false, "stddev(intPrimitive, true)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("avedev-name-mismatch", "avedev(int)", false, "avg(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("avedev-param-type", "avedev(int)", false, "avedev(doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("avedev-filter-provided", "avedev(int)", false, "avedev(intPrimitive, true)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("median-name-mismatch", "median(int)", false, "avg(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("median-param-type", "median(int)", false, "median(doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("median-filter-provided", "median(int)", false, "median(intPrimitive, true)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("firstever-direction", "firstever(int)", false, "lastever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("firstever-param-type", "firstever(int)", false, "firstever(doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("firstever-filter-declared", "firstever(int, boolean)", false, "firstever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("lastever-direction", "lastever(int)", false, "firstever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("lastever-param-type", "lastever(int)", false, "lastever(doublePrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("lastever-filter-declared", "lastever(int, boolean)", false, "lastever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("countever-direction", "lastever(int)", true, "countever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("countever-filter-declared", "lastever(int, boolean)", true, "countever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("countever-filter-provided", "lastever(int)", true, "countever(intPrimitive, true)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("countever-ignore-nulls", "countever(*)", true, "countever(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("nth-name-mismatch", "nth(int, 10)", false, "avg(20)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("nth-size", "nth(int, 10)", false, "nth(intPrimitive, 11)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'nth(int,10)' and received 'nth(intPrimitive,11)': The size is 10 and provided is 11 [", ""),
            new AggProbe("nth-param-type", "nth(int, 10)", false, "nth(doublePrimitive, 10)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("rate-name-mismatch", "rate(20)", false, "avg(20)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("rate-interval", "rate(20)", false, "rate(11)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'rate(20)' and received 'rate(11)': The interval-time is 20000 and provided is 11000 [", ""),
            new AggProbe("leaving-name-mismatch", "leaving()", false, "avg(intPrimitive)", "", CONTAINS_INCOMPATIBLE),
            new AggProbe("plugin-single-name", "myaggsingle()", false, "leaving()",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'myaggsingle(*)' and received 'leaving(*)': The table declares 'myaggsingle(*)' and provided is 'leaving(*)'", ""),
    };

    // Verbatim transcriptions of InfraInvalidAggMatchMultiFunc's 6
    // tryInvalidAggMatch calls (ord 1); all are unbound (#time(1000)).
    private static final AggProbe[] MULTI_FUNC_PROBES = {
            new AggProbe("window-vs-agg-method", "window(*) @type(SupportBean)", true, "avg(intPrimitive)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'window(*)' and received 'avg(intPrimitive)': The table declares 'window(*)' and provided is 'avg(intPrimitive)'", ""),
            new AggProbe("window-vs-sorted", "window(*) @type(SupportBean)", true, "sorted(intPrimitive)",
                    "Failed to validate select-clause expression 'sorted(intPrimitive)': When specifying into-table a sort expression cannot be provided [", ""),
            new AggProbe("window-event-type", "window(*) @type(SupportBean_S0)", true, "window(*)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'window(*)' and received 'window(*)': The required event type is 'SupportBean_S0' and provided is 'SupportBean' [", ""),
            new AggProbe("sorted-vs-window", "sorted(intPrimitive) @type(SupportBean)", true, "window(*)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'sorted(intPrimitive)' and received 'window(*)': The table declares 'sorted(intPrimitive)' and provided is 'window(*)'", ""),
            new AggProbe("sorted-sort-expr", "sorted(id) @type(SupportBean_S0)", true, "sorted(intPrimitive)",
                    "Failed to validate select-clause expression 'sorted(intPrimitive)': When specifying into-table a sort expression cannot be provided [", ""),
            new AggProbe("plugin-multi-vs-window", "se1() @type(SupportBean)", true, "window(*)",
                    "Incompatible aggregation function for table 'var1' column 'value', expecting 'se1(*)' and received 'window(*)': The table declares 'se1(*)' and provided is 'window(*)'", ""),
    };

    // Verbatim transcriptions of InfraInvalidAnnotations' 5
    // tryInvalidCompile calls (ord 2); all compile without the path.
    private static final String[][] ANNOTATION_PROBES = {
            {"annotation-unknown", "create table v1 (abc window(*) @unknown)",
                    "For column 'abc' unrecognized annotation 'unknown' ["},
            {"annotation-missing-value", "create table v1 (abc window(*) @type)",
                    "For column 'abc' no value provided for annotation 'type', expected a value ["},
            {"annotation-duplicate", "create table v1 (abc window(*) @type(SupportBean) @type(SupportBean))",
                    "For column 'abc' multiple annotations provided named 'type' ["},
            {"annotation-non-string", "create table v1 (abc window(*) @type(1))",
                    "For column 'abc' string value expected for annotation 'type' ["},
            {"annotation-unknown-type", "create table v1 (abc window(*) @type(xx))",
                    "For column 'abc' failed to find event type 'xx' ["},
    };

    // Verbatim transcriptions of InfraInvalid's fixture deploys (ord 3).
    // All but the objectarray schema compile with the runtime path;
    // env.compileDeploy(epl) for MyEvent compiles without it.
    private static final String[][] INVALID_DEPLOYS = {
            {"table-grouped-string", "@public create table aggvar_grouped_string (key string primary key, total count(*))", ""},
            {"table-twogrouped", "@public create table aggvar_twogrouped (keyone string primary key, keytwo string primary key, total count(*))", ""},
            {"table-grouped-int", "@public create table aggvar_grouped_int (key int primary key, total count(*))", ""},
            {"table-ungrouped", "@public create table aggvar_ungrouped as (total count(*))", ""},
            {"table-ungrouped-window", "@public create table aggvar_ungrouped_window as (win window(*) @type(SupportBean))", ""},
            {"context", "@public create context MyContext initiated by SupportBean_S0 terminated by SupportBean_S1", ""},
            {"table-context", "@public context MyContext create table aggvarctx (total count(*))", ""},
            {"context-other", "@public create context MyOtherContext initiated by SupportBean_S0 terminated by SupportBean_S1", ""},
            {"variable", "@public create variable int myvariable", ""},
            {"named-window", "@public create window MyNamedWindow#keepall as select * from SupportBean", ""},
            {"schema", "@public create schema SomeSchema(p0 string)", ""},
            {"schema-objectarray", "create objectarray schema MyEvent(abc int[])", "1"},
    };

    // Verbatim transcriptions of InfraInvalid's 50 tryInvalidCompile(path,
    // ...) probes (ord 3) in source order.  The two "skip" assertions carry
    // an empty expectError.
    private static final String[][] INVALID_PROBES = {
            {"declare-constant-variable", "create constant variable aggvar_ungrouped (total count(*))",
                    "Incorrect syntax near '(' expecting an identifier but found an opening parenthesis '(' at line 1 column 42 ["},
            {"declare-invalid-type", "create table aggvar_notright as (total sum(abc))",
                    "Failed to resolve type 'abc': Could not load class by name 'abc', please check imports ["},
            {"declare-non-aggregation", "create table aggvar_wrongtoo as (total singlerow(1))",
                    "Expression 'singlerow(1)' is not an aggregation ["},
            {"declare-window-param", "create table aggvar_invalid as (mywindow window(intPrimitive) @type(SupportBean))",
                    "Failed to validate table-column expression 'window(intPrimitive)': For tables columns, the window aggregation function requires the 'window(*)' declaration ["},
            {"declare-last-star", "create table aggvar_invalid as (mywindow last(*)@type(SupportBean))", ""},
            {"declare-window-stream-star", "create table aggvar_invalid as (mywindow window(sb.*)@type(SupportBean)", ""},
            {"declare-maxby", "create table aggvar_invalid as (mymax maxBy(intPrimitive) @type(SupportBean))",
                    "Failed to validate table-column expression 'maxby(intPrimitive)': For tables columns, the aggregation function requires the 'sorted(*)' declaration ["},
            {"declare-duplicate-column", "create table aggvar_invalid as (mycount count(*),mycount count(*))",
                    "Column 'mycount' is listed more than once [create table aggvar_invalid as (mycount count(*),mycount count(*))]"},
            {"declare-variable-collision", "create table myvariable as (mycount count(*))",
                    "A variable by name 'myvariable' has already been declared ["},
            {"declare-table-collision", "create table aggvar_ungrouped as (total count(*))",
                    "A table by name 'aggvar_ungrouped' has already been declared ["},
            {"declare-pk-expression", "create table abc as (total count(*) primary key)",
                    "Column 'total' may not be tagged as primary key, an expression cannot become a primary key column ["},
            {"declare-pk-event-type", "create table abc as (arr SupportBean primary key)",
                    "Column 'arr' may not be tagged as primary key, received unexpected event type 'SupportBean' ["},
            {"declare-pk-prim", "create table abc as (mystr string prim key)",
                    "Invalid keyword 'prim' encountered, expected 'primary key' ["},
            {"declare-pk-keys", "create table abc as (mystr string primary keys)",
                    "Invalid keyword 'keys' encountered, expected 'primary key' ["},
            {"declare-schema-collision", "create table SomeSchema as (mystr string)",
                    "An event type by name 'SomeSchema' has already been declared"},
            {"into-table-not-found", "into table xxx select count(*) as total from SupportBean group by intPrimitive",
                    "Invalid into-table clause: Failed to find table by name 'xxx' ["},
            {"into-groupby-type", "into table aggvar_grouped_string select count(*) as total from SupportBean group by intPrimitive",
                    "Incompatible type returned by a group-by expression for use with table 'aggvar_grouped_string', the group-by expression 'intPrimitive' returns 'Integer' but the table expects 'String' ["},
            {"into-groupby-count-over", "into table aggvar_grouped_string select count(*) as total from SupportBean group by theString, intPrimitive",
                    "Incompatible number of group-by expressions for use with table 'aggvar_grouped_string', the table expects 1 group-by expressions and provided are 2 group-by expressions ["},
            {"into-groupby-count-ungrouped", "into table aggvar_ungrouped select count(*) as total from SupportBean group by theString",
                    "Incompatible number of group-by expressions for use with table 'aggvar_ungrouped', the table expects no group-by expressions and provided are 1 group-by expressions ["},
            {"into-groupby-count-missing", "into table aggvar_grouped_string select count(*) as total from SupportBean",
                    "Incompatible number of group-by expressions for use with table 'aggvar_grouped_string', the table expects 1 group-by expressions and provided are no group-by expressions ["},
            {"into-context-missing", "into table aggvarctx select count(*) as total from SupportBean",
                    "Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [into table aggvarctx select count(*) as total from SupportBean]"},
            {"into-context-other", "context MyOtherContext into table aggvarctx select count(*) as total from SupportBean",
                    "Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [context MyOtherContext into table aggvarctx select count(*) as total from SupportBean]"},
            {"into-write-only", "into table aggvar_ungrouped select count(*) as total, aggvar_ungrouped from SupportBean",
                    "Invalid use of table 'aggvar_ungrouped', aggregate-into requires write-only, the expression 'aggvar_ungrouped' is not allowed [into table aggvar_ungrouped select count(*) as total, aggvar_ungrouped from SupportBean]"},
            {"into-unidirectional", "into table aggvar_ungrouped select count(*) as total from SupportBean unidirectional, SupportBean_S0#keepall",
                    "Into-table does not allow unidirectional joins ["},
            {"into-requires-aggregation", "into table aggvar_ungrouped select * from SupportBean",
                    "Into-table requires at least one aggregation function ["},
            {"access-key-count", "select aggvar_ungrouped['a'].total from SupportBean",
                    "Failed to validate select-clause expression 'aggvar_ungrouped[\"a\"].total': Incompatible number of key expressions for use with table 'aggvar_ungrouped', the table expects no key expressions and provided are 1 key expressions [select aggvar_ungrouped['a'].total from SupportBean]"},
            {"access-no-key", "select aggvar_grouped_string.total from SupportBean",
                    "Failed to validate select-clause expression 'aggvar_grouped_string.total': Failed to resolve property 'aggvar_grouped_string.total' to a stream or nested property in a stream"},
            {"access-key-type", "select aggvar_grouped_string[5].total from SupportBean",
                    "Failed to validate select-clause expression 'aggvar_grouped_string[5].total': Incompatible type returned by a key expression for use with table 'aggvar_grouped_string', the key expression '5' returns 'Integer' but the table expects 'String' [select aggvar_grouped_string[5].total from SupportBean]"},
            {"access-unknown-function", "select aggvar_grouped_string.something() from SupportBean",
                    "Invalid use of table 'aggvar_grouped_string', unrecognized use of function 'something', expected 'keys()'"},
            {"access-unknown-name", "select dummy[intPrimitive] from SupportBean",
                    "Failed to validate select-clause expression 'dummy[intPrimitive]': Failed to resolve 'dummy' to a property, single-row function, aggregation function, script, stream or class name"},
            {"access-unknown-column", "select aggvarctx.dummy from SupportBean",
                    "Failed to validate select-clause expression 'aggvarctx.dummy': A column 'dummy' could not be found for table 'aggvarctx' [select aggvarctx.dummy from SupportBean]"},
            {"access-window-column-method", "select aggvarctx_ungrouped_window.win.dummy(123) from SupportBean",
                    "Failed to validate select-clause expression 'aggvarctx_ungrouped_window.win.dumm...(41 chars)': Failed to resolve 'aggvarctx_ungrouped_window.win.dummy' to a property, single-row function, aggregation function, script, stream or class name [select aggvarctx_ungrouped_window.win.dummy(123) from SupportBean]"},
            {"access-context-other", "context MyOtherContext select aggvarctx.total from SupportBean",
                    "Failed to validate select-clause expression 'aggvarctx.total': Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [context MyOtherContext select aggvarctx.total from SupportBean]"},
            {"access-context-other-2", "context MyOtherContext select aggvarctx.total from SupportBean",
                    "Failed to validate select-clause expression 'aggvarctx.total': Table by name 'aggvarctx' has been declared for context 'MyContext' and can only be used within the same context [context MyOtherContext select aggvarctx.total from SupportBean]"},
            {"access-unknown-column-nested", "select aggvar_grouped_int[0].a.b from SupportBean",
                    "Failed to validate select-clause expression 'aggvar_grouped_int[0].a.b': A column 'a' could not be found for table 'aggvar_grouped_int'"},
            {"view-param-table", "select * from SupportBean#time(aggvar_ungrouped.total sec)",
                    "Failed to validate data window declaration: Error in view 'time', Invalid parameter expression 0 for Time view: Failed to validate view parameter expression 'aggvar_ungrouped.total seconds': Invalid use of table access expression, expression 'aggvar_ungrouped' is not allowed here"},
            {"view-on-table", "select * from aggvar_grouped_string#time(30)",
                    "Views are not supported with tables"},
            {"view-on-table-subquery", "select (select * from aggvar_ungrouped#keepall) from SupportBean",
                    "Views are not supported with tables ["},
            {"contained-table", "select * from aggvar_grouped_string[books]",
                    "Contained-event expressions are not supported with tables"},
            {"join-method-on-column", "select aggvar_grouped_int[1].total.countMinSketchFrequency(theString) from SupportBean",
                    "Failed to validate select-clause expression 'aggvar_grouped_int[1].total.countMi...(62 chars)': Failed to resolve method 'countMinSketchFrequency': Could not find enumeration method, date-time method, instance method or property named 'countMinSketchFrequency' in class 'Long' with matching parameter number and expected parameter type(s) 'String' "},
            {"join-unidirectional-method", "select total.countMinSketchFrequency(theString) from aggvar_grouped_int, SupportBean unidirectional",
                    "Failed to validate select-clause expression 'total.countMinSketchFrequency(theString)': Failed to resolve method 'countMinSketchFrequency': Could not find"},
            {"table-unidirectional", "select * from aggvar_grouped_int unidirectional, SupportBean",
                    "Tables cannot be marked as unidirectional ["},
            {"table-retain", "select * from aggvar_grouped_int retain-union",
                    "Tables cannot be marked with retain ["},
            {"table-on-action", "on aggvar_ungrouped select * from aggvar_ungrouped",
                    "Tables cannot be used in an on-action statement triggering stream ["},
            {"table-match-recognize", "select * from aggvar_ungrouped match_recognize ( measures a.theString as a pattern (A) define A as true)",
                    "Tables cannot be used with match-recognize ["},
            {"table-update-istream", "update istream aggvar_grouped_string set key = 'a'",
                    "Tables cannot be used in an update-istream statement ["},
            {"table-context-declaration", "create context InvalidCtx as start aggvar_ungrouped end after 5 seconds",
                    "Tables cannot be used in a context declaration ["},
            {"table-pattern-atom", "select * from pattern[aggvar_ungrouped]",
                    "Tables cannot be used in pattern filter atoms ["},
            {"schema-table-collision", "create schema aggvar_ungrouped as SupportBean",
                    "A table by name 'aggvar_ungrouped' already exists ["},
            {"declare-null-pk", "create table MyTable(somefield null primary key, id string)",
                    "Incorrect syntax near 'null' (a reserved keyword)"},
    };

    private static final String[] CASE_EPLS = {
            SINGLE_FUNC_PROBES[0].createEpl(),
            MULTI_FUNC_PROBES[0].createEpl(),
            ANNOTATION_PROBES[0][1],
            INVALID_DEPLOYS[0][1],
    };

    private static final int EXPECTED_STEPS = 219;
    private static final int EXPECTED_RECORDS = 104;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|expectContains|
     * compileWithoutPath|mode|selector|ids|fields.  Deploy steps carry the
     * byte-exact EPL text; build-error steps carry the pinned expectError
     * prefix or expectContains substring and the compileWithoutPath marker
     * for the path-less probes.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("agg-match-single-func", aggMatchSteps("agg-match-single-func", SINGLE_FUNC_PROBES));
        CASE_STEPS.put("agg-match-multi-func", aggMatchSteps("agg-match-multi-func", MULTI_FUNC_PROBES));
        List<String> annotations = new ArrayList<>();
        for (String[] probe : ANNOTATION_PROBES) {
            annotations.add("build-error|annotations|" + probe[0] + "||" + probe[1]
                    + "||" + probe[2] + "||1||||");
        }
        CASE_STEPS.put("annotations", annotations.toArray(new String[0]));
        List<String> invalid = new ArrayList<>();
        for (String[] deploy : INVALID_DEPLOYS) {
            invalid.add("deploy|invalid|" + deploy[0] + "||" + deploy[1] + "||||" + deploy[2] + "||||");
        }
        for (String[] probe : INVALID_PROBES) {
            invalid.add("build-error|invalid|" + probe[0] + "||" + probe[1] + "||" + probe[2] + "||||||");
        }
        invalid.add("undeploy-all|invalid|||||||||||");
        CASE_STEPS.put("invalid", invalid.toArray(new String[0]));
    }

    /**
     * Renders the deploy/build-error/undeploy-all step keys for one
     * tryInvalidAggMatch probe sequence: the 'create' table deploys with
     * the runtime path, the into-table probe compiles against the path,
     * and undeploy-all mirrors env.undeployModuleContaining("create") (the
     * probe module is the only deployment).
     */
    private static String[] aggMatchSteps(String caseName, AggProbe[] probes) {
        List<String> steps = new ArrayList<>();
        for (AggProbe probe : probes) {
            steps.add("deploy|" + caseName + "|" + probe.label + "||" + probe.createEpl() + "||||||||");
            steps.add("build-error|" + caseName + "|" + probe.label + "||" + probe.probeEpl()
                    + "||" + probe.expectError + "|" + probe.expectContains + "|||||");
            steps.add("undeploy-all|" + caseName + "|||||||||||");
        }
        return steps.toArray(new String[0]);
    }

    private InfraTableInvalidScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableInvalidScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays the case's steps on a fresh runtime.  Deploy steps compile
     * with the runtime path (env.compileDeploy(epl, path)) unless marked
     * compileWithoutPath (env.compileDeploy(epl)); build-error steps
     * compile with or without the path per their compileWithoutPath
     * marker, mirroring env.tryInvalidCompile's two forms; undeploy-all
     * mirrors env.undeployAll() and, for the agg-match cases,
     * undeployModuleContaining("create") (the probe module is the only
     * deployment).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getCompiler().addPlugInSingleRowFunction("singlerow",
                InfraTableInvalid.class.getName(), "mySingleRowFunction");
        configuration.getCompiler().addPlugInAggregationFunctionForge("myaggsingle",
                SupportCountBackAggregationFunctionForge.class.getName());
        configuration.getCompiler().addPlugInAggregationMultiFunction(
                new ConfigurationCompilerPlugInAggregationMultiFunction("se1".split(","),
                        SupportAggMFMultiRTForge.class.getName()));
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        try {
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy": {
                        String epl = string(step, "epl");
                        EPCompiled compiled;
                        if (step.getBoolean("compileWithoutPath", false)) {
                            // env.compileDeploy(epl): compile without the
                            // runtime path, then deploy.
                            compiled = EPCompilerProvider.getCompiler()
                                    .compile(epl, new CompilerArguments(configuration));
                        } else {
                            compiled = compileModule(epl, configuration, runtime);
                        }
                        runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        break;
                    }
                    case "build-error":
                        buildErrorStep(runtime, configuration, caseName, step, records);
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /**
     * Module compile with the runtime path, mirroring
     * RegressionEnvironmentBase.compileDeploy(epl, path): the compiler sees
     * the configuration plus the already-deployed module path so the
     * fixture deploys and the path-ful probes resolve earlier module
     * objects.
     */
    private static EPCompiled compileModule(String epl, Configuration configuration,
                                            EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * after verifying the caught message: expectError pins a startsWith
     * prefix, expectContains pins a contains substring (the null-message
     * tryInvalidAggMatch arm), and both empty mirrors assertMessage("skip").
     * Probes marked compileWithoutPath compile without the runtime path,
     * mirroring env.tryInvalidCompile's path-less compileWCheckedEx; the
     * rest compile with the path.
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String expectedContains = string(step, "expectContains");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!step.getBoolean("compileWithoutPath", false)) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if (caught == null || caught.equals("<no-error>")) {
            throw new IllegalStateException("build-error probe " + label
                    + " unexpectedly succeeded");
        }
        if (!expected.isEmpty() && !caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        if (!expectedContains.isEmpty() && !caught.contains(expectedContains)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected contains [" + expectedContains + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        } else if (!expectedContains.isEmpty()) {
            record.add("value", expectedContains);
        }
        records.add(record);
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|expectContains|
     * compileWithoutPath|mode|selector|ids|fields with the payload compacted
     * and ids/fields rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "expectContains", "compileWithoutPath", "mode", "selector",
                "ids", "fields"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + string(step, "expectContains") + "|" + cwp
                + "|" + string(step, "mode") + "|" + string(step, "selector") + "|" + idsText
                + "|" + fieldsText;
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
