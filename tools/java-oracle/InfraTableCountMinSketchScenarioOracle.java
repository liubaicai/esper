import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
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
import com.espertech.esper.common.client.util.CountMinSketchTopK;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.epl.approx.countminsketch.CountMinSketchAggState;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.common.internal.support.SupportBean_S2;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportByteArrEventStringId;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraTableCountMinSketch (ords 0-3).  Four cases replay on
 * four fresh runtimes:
 *
 * frequency-and-topk (ordinal 0, InfraFrequencyAndTopk): one compileDeploy of
 * the four-statement module (create table MyApproxFT with a topk-3
 * countMinSketch column, the into-table countMinSketchAdd feed, and the
 * named frequency/topk read statements) followed by the assertOutput send
 * sequence — one SupportBean_S0 per expected frequency pair and one
 * SupportBean_S1 per topk assertion — then the join and subquery modules
 * each deploy, fire one SupportBean_S2, and undeployModuleContaining.
 *
 * doc-samples (ordinal 1, InfraDocSamples): seven compileDeploy calls (two
 * create-schema, two create-table, the into-table feed, the frequency read,
 * and the pattern-driven topk read); no events are sent.
 *
 * non-string-type (ordinal 2, InfraNonStringType): the MyApproxNS table with
 * the MyBytesPassthruAgentForge agent, the id='A' into-table feed, and the
 * id='B' s0 read; three SupportByteArrEventStringId sends record freq 0 then
 * 1.
 *
 * invalid (ordinal 3, InfraInvalid): the MyCMS fixture deploy then fifteen
 * tryInvalidCompile probes across declaration, into-table, and consumption
 * surfaces, then undeployAll.
 *
 * Assertion encoding: expectError pins a startsWith prefix verified against
 * the caught compile exception before the compile-error record is emitted.
 * Listener records render the topk column as a JSON array of
 * {value,frequency} objects in emission order (the Java suite asserts
 * membership, not order; the deterministic order is pinned by the trace).
 */
public final class InfraTableCountMinSketchScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-table-count-min-sketch";
    private static final String DESCRIPTION =
            "InfraTableCountMinSketch ords 0-3: count-min-sketch table columns — a topk-3 "
                    + "column fed by countMinSketchAdd with frequency/topk listeners plus join and "
                    + "subquery reads, the compile-only doc samples including a fully-parameterized "
                    + "declaration, byte[] keys via a custom agent, and fifteen invalid compile probes.";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/"
                    + "InfraTableCountMinSketch.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-f09401ff1e3349b3497b",
            "java-runtime-87f8d0057f91e1dfa7ec",
            "java-runtime-4b4c531bbad3d4e1491d",
            "java-runtime-217779fa3c860cc1ea18"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraFrequencyAndTopk",
            "InfraDocSamples",
            "InfraNonStringType",
            "InfraInvalid"
    };
    private static final String[] STATIC_IDS = {
            "java-834a25e0038e5f5a8950",
            "java-d926cce1a816fb780ed8",
            "java-304d2b5b2913d2f02828",
            "java-91bcaf7a054d58304543"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "frequency-and-topk",
            "doc-samples",
            "non-string-type",
            "invalid"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] CASE_OBSERVATIONS = {
            "topk-3 count-min-sketch column fed by countMinSketchAdd(theString); frequency/topk "
                    + "listeners record after every send, the join and subquery modules each fire "
                    + "once, and undeployAll closes the case",
            "compile-only doc samples: two create-schema registrations, a default and a "
                    + "fully-parameterized countMinSketch table, the into-table feed, the frequency "
                    + "read, and a pattern-driven topk read; no events are sent",
            "byte[] keys via MyBytesPassthruAgentForge: the feed filters id='A', the read filters "
                    + "id='B', and countMinSketchFrequency(body) reports 0 then 1",
            "deploys the MyCMS fixture then runs fifteen tryInvalidCompile probes across "
                    + "declaration, into-table, and consumption surfaces before undeployAll"
    };

    private static final String AGENT_UTF16 =
            "com.espertech.esper.common.client.util.CountMinSketchAgentStringUTF16Forge";
    private static final String AGENT_BYTES =
            "com.espertech.esper.regressionlib.suite.infra.tbl.InfraTableCountMinSketch$MyBytesPassthruAgentForge";

    private static final String EPL_MODULE =
            "@public create table MyApproxFT(wordapprox countMinSketch({topk:3}));\n"
                    + "into table MyApproxFT select countMinSketchAdd(theString) as wordapprox from SupportBean;\n"
                    + "@name('frequency') select MyApproxFT.wordapprox.countMinSketchFrequency(p00) as freq from SupportBean_S0;\n"
                    + "@name('topk') select MyApproxFT.wordapprox.countMinSketchTopk() as topk from SupportBean_S1;\n";
    private static final String EPL_JOIN =
            "@name('join') select wordapprox.countMinSketchFrequency(s2.p20) as c0 from MyApproxFT, SupportBean_S2 s2 unidirectional";
    private static final String EPL_SUBQ =
            "@name('subq') select (select wordapprox.countMinSketchFrequency(s2.p20) from MyApproxFT) as c0 from SupportBean_S2 s2";

    private static final String EPL_SCHEMA_WORD = "@public create schema WordEvent (word string)";
    private static final String EPL_SCHEMA_ESTIM =
            "@public create schema EstimateWordCountEvent (word string)";
    private static final String EPL_TABLE_WORD =
            "@public create table WordCountTable(wordcms countMinSketch())";
    private static final String EPL_TABLE_WORD_TWO =
            "@public create table WordCountTable2(wordcms countMinSketch({\n"
                    + "  epsOfTotalCount: 0.000002,\n"
                    + "  confidence: 0.999,\n"
                    + "  seed: 38576,\n"
                    + "  topk: 20,\n"
                    + "  agent: '" + AGENT_UTF16 + "'}))";
    private static final String EPL_INTO_WORD =
            "into table WordCountTable select countMinSketchAdd(word) as wordcms from WordEvent";
    private static final String EPL_SELECT_FREQ =
            "select WordCountTable.wordcms.countMinSketchFrequency(word) from EstimateWordCountEvent";
    private static final String EPL_SELECT_TOPK =
            "select WordCountTable.wordcms.countMinSketchTopk() from pattern[every timer:interval(10 sec)]";
    private static final String EPL_TABLE_NS =
            "@public create table MyApproxNS(bytefreq countMinSketch({  epsOfTotalCount: 0.02,"
                    + "  confidence: 0.98,  topk: null,  agent: '" + AGENT_BYTES + "'}))";
    private static final String EPL_INTO_NS =
            "into table MyApproxNS select countMinSketchAdd(body) as bytefreq from SupportByteArrEventStringId(id='A')";
    private static final String EPL_READ_NS =
            "@name('s0') select MyApproxNS.bytefreq.countMinSketchFrequency(body) as freq from SupportByteArrEventStringId(id='B')";
    private static final String EPL_INVALID_DEPLOY =
            "@public create table MyCMS(wordcms countMinSketch())";

    private static final String[] CASE_EPLS = {
            EPL_MODULE + EPL_JOIN + "\n" + EPL_SUBQ,
            EPL_SCHEMA_WORD + "\n" + EPL_SCHEMA_ESTIM + "\n" + EPL_TABLE_WORD + "\n"
                    + EPL_TABLE_WORD_TWO + "\n" + EPL_INTO_WORD + "\n" + EPL_SELECT_FREQ + "\n"
                    + EPL_SELECT_TOPK,
            EPL_TABLE_NS + "\n" + EPL_INTO_NS + "\n" + EPL_READ_NS,
            EPL_INVALID_DEPLOY,
    };

    // Verbatim transcriptions of InfraInvalid's fifteen tryInvalidCompile
    // probes (ord 3) in source order, including the oracle's typos ('expects
    // an Double', the double space after 'countMinSketch'', 'requires a no
    // parameter expressions', the trailing space in 'could not be resolved ',
    // and the 43-char expression truncation).
    private static final String[][] INVALID_PROBES = {
            {"cms-in-select", "select countMinSketch() from SupportBean",
                    "Failed to validate select-clause expression 'countMinSketch()': Count-min-sketch aggregation function 'countMinSketch' can only be used in create-table statements ["},
            {"cms-invalid-param", "create table MyTable(cms countMinSketch(5))",
                    "Failed to validate table-column expression 'countMinSketch(5)': Count-min-sketch aggregation function 'countMinSketch'  expects either no parameter or a single json parameter object ["},
            {"cms-invalid-json-param", "create table MyTable(cms countMinSketch({xxx:3}))",
                    "Failed to validate table-column expression 'countMinSketch({xxx=3})': Unrecognized parameter 'xxx' ["},
            {"cms-invalid-eps", "create table MyTable(cms countMinSketch({epsOfTotalCount:'a'}))",
                    "Failed to validate table-column expression 'countMinSketch({epsOfTotalCount=a})': Property 'epsOfTotalCount' expects an Double but receives a value of type String ["},
            {"cms-agent-not-found", "create table MyTable(cms countMinSketch({agent:'a'}))",
                    "Failed to validate table-column expression 'countMinSketch({agent=a})': Failed to instantiate agent provider: Could not load class by name 'a', please check imports ["},
            {"cms-agent-wrong-type", "create table MyTable(cms countMinSketch({agent:'java.lang.String'}))",
                    "Failed to validate table-column expression 'countMinSketch({agent=java.lang.String})': Failed to instantiate agent provider: Class 'java.lang.String' does not implement interface 'com.espertech.esper.common.client.util.CountMinSketchAgentForge' ["},
            {"add-in-select", "select countMinSketchAdd(theString) from SupportBean",
                    "Failed to validate select-clause expression 'countMinSketchAdd(theString)': Count-min-sketch aggregation function 'countMinSketchAdd' can only be used with into-table"},
            {"add-no-param", "into table MyCMS select countMinSketchAdd() as wordcms from SupportBean",
                    "Failed to validate select-clause expression 'countMinSketchAdd()': Count-min-sketch aggregation function 'countMinSketchAdd' requires a single parameter expression"},
            {"add-byte-param", "into table MyCMS select countMinSketchAdd(body) as wordcms from SupportByteArrEventStringId",
                    "Incompatible aggregation function for table 'MyCMS' column 'wordcms', expecting 'countMinSketch()' and received 'countMinSketchAdd(body)': Mismatching parameter return type, expected any of [class java.lang.String] but received byte[] ["},
            {"add-distinct", "into table MyCMS select countMinSketchAdd(distinct 'abc') as wordcms from SupportByteArrEventStringId",
                    "Failed to validate select-clause expression 'countMinSketchAdd(distinct \"abc\")': Count-min-sketch aggregation function 'countMinSketchAdd' is not supported with distinct ["},
            {"add-null", "into table MyCMS select countMinSketchAdd(null) as wordcms from SupportByteArrEventStringId",
                    "Failed to validate select-clause expression 'countMinSketchAdd(null)': Invalid null-type parameter"},
            {"frequency-into-table", "into table MyCMS select countMinSketchFrequency(theString) as wordcms from SupportBean",
                    "Failed to validate select-clause expression 'countMinSketchFrequency(theString)': Unknown single-row function, aggregation function or mapped or indexed property named 'countMinSketchFrequency' could not be resolved "},
            {"frequency-select", "select countMinSketchFrequency() from SupportBean",
                    "Failed to validate select-clause expression 'countMinSketchFrequency()': Unknown single-row function, expression declaration, script or aggregation function named 'countMinSketchFrequency' could not be resolved"},
            {"topk-select", "select countMinSketchTopk() from SupportBean",
                    "Failed to validate select-clause expression 'countMinSketchTopk()': Unknown single-row function, expression declaration, script or aggregation function named 'countMinSketchTopk' could not be resolved"},
            {"topk-param", "select MyCMS.wordcms.countMinSketchTopk(theString) from SupportBean",
                    "Failed to validate select-clause expression 'MyCMS.wordcms.countMinSketchTopk(th...(43 chars)': Count-min-sketch aggregation function 'countMinSketchTopk' requires a no parameter expressions ["},
    };

    private static final int EXPECTED_STEPS = 88;
    private static final int EXPECTED_RECORDS = 57;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|expectContains|
     * compileWithoutPath|mode|selector|ids|fields.  Deploy steps carry the
     * byte-exact EPL text; send steps carry the compacted payload;
     * build-error steps carry the pinned expectError prefix.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        List<String> frequencyAndTopk = new ArrayList<>();
        frequencyAndTopk.add("deploy|frequency-and-topk|module||" + EPL_MODULE + "||||||||");
        frequencyAndTopk.add("deployed|frequency-and-topk|module||||||||||");
        // assertOutput("E1=1", "E1=1") after SupportBean("E1")
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean||{\"theString\":\"E1\",\"intPrimitive\":0}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // milestone(0); assertOutput("E1=1", "E1=1")
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // send E2; milestone(1); assertOutput("E1=1,E2=1", "E1=1,E2=1")
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":0}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // send E2; assertOutput("E1=1,E2=2", "E1=1,E2=2")
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean||{\"theString\":\"E2\",\"intPrimitive\":0}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // send E3; assertOutput("E1=1,E2=2,E3=1", "E1=1,E2=2,E3=1")
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean||{\"theString\":\"E3\",\"intPrimitive\":0}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E3\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // milestone(2); send E4; assertOutput("E1=1,E2=2,E3=1,E4=1",
        // "E1=1,E2=2,E3=1") — strict-> admission keeps E4 out of topk
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean||{\"theString\":\"E4\",\"intPrimitive\":0}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E3\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E4\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // send E4; assertOutput("E1=1,E2=2,E3=1,E4=2", "E1=1,E2=2,E4=2") —
        // E3 evicted
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean||{\"theString\":\"E4\",\"intPrimitive\":0}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E1\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E2\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E3\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S0||{\"id\":0,\"p00\":\"E4\"}|||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S1||{\"id\":0}|||||||");
        // join module: deploy, milestone(3), one S2, undeployModuleContaining
        frequencyAndTopk.add("deploy|frequency-and-topk|join||" + EPL_JOIN + "||||||||");
        frequencyAndTopk.add("deployed|frequency-and-topk|join||||||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S2||{\"id\":0,\"p20\":\"E3\"}|||||||");
        frequencyAndTopk.add("undeploy|frequency-and-topk|join||||||||||");
        // subquery module: deploy, milestone(4), one S2, undeploy
        frequencyAndTopk.add("deploy|frequency-and-topk|subq||" + EPL_SUBQ + "||||||||");
        frequencyAndTopk.add("deployed|frequency-and-topk|subq||||||||||");
        frequencyAndTopk.add("send|frequency-and-topk||SupportBean_S2||{\"id\":0,\"p20\":\"E3\"}|||||||");
        frequencyAndTopk.add("undeploy|frequency-and-topk|subq||||||||||");
        frequencyAndTopk.add("undeploy-all|frequency-and-topk|||||||||||");
        CASE_STEPS.put("frequency-and-topk", frequencyAndTopk.toArray(new String[0]));

        List<String> docSamples = new ArrayList<>();
        docSamples.add("deploy|doc-samples|schema-word-event||" + EPL_SCHEMA_WORD + "||||||||");
        docSamples.add("deployed|doc-samples|schema-word-event||||||||||");
        docSamples.add("deploy|doc-samples|schema-estimate-event||" + EPL_SCHEMA_ESTIM + "||||||||");
        docSamples.add("deployed|doc-samples|schema-estimate-event||||||||||");
        docSamples.add("deploy|doc-samples|table-word-count||" + EPL_TABLE_WORD + "||||||||");
        docSamples.add("deployed|doc-samples|table-word-count||||||||||");
        docSamples.add("deploy|doc-samples|table-word-count2||" + EPL_TABLE_WORD_TWO + "||||||||");
        docSamples.add("deployed|doc-samples|table-word-count2||||||||||");
        docSamples.add("deploy|doc-samples|into-word-count||" + EPL_INTO_WORD + "||||||||");
        docSamples.add("deployed|doc-samples|into-word-count||||||||||");
        docSamples.add("deploy|doc-samples|select-frequency||" + EPL_SELECT_FREQ + "||||||||");
        docSamples.add("deployed|doc-samples|select-frequency||||||||||");
        docSamples.add("deploy|doc-samples|select-topk||" + EPL_SELECT_TOPK + "||||||||");
        docSamples.add("deployed|doc-samples|select-topk||||||||||");
        docSamples.add("undeploy-all|doc-samples|||||||||||");
        CASE_STEPS.put("doc-samples", docSamples.toArray(new String[0]));

        List<String> nonString = new ArrayList<>();
        nonString.add("deploy|non-string-type|table||" + EPL_TABLE_NS + "||||||||");
        nonString.add("deployed|non-string-type|table||||||||||");
        nonString.add("deploy|non-string-type|into||" + EPL_INTO_NS + "||||||||");
        nonString.add("deployed|non-string-type|into||||||||||");
        nonString.add("deploy|non-string-type|s0||" + EPL_READ_NS + "||||||||");
        nonString.add("deployed|non-string-type|s0||||||||||");
        nonString.add("send|non-string-type||SupportByteArrEventStringId||{\"id\":\"A\",\"body\":\"AQID\"}|||||||");
        nonString.add("send|non-string-type||SupportByteArrEventStringId||{\"id\":\"B\",\"body\":\"AAID\"}|||||||");
        nonString.add("send|non-string-type||SupportByteArrEventStringId||{\"id\":\"B\",\"body\":\"AQID\"}|||||||");
        nonString.add("undeploy-all|non-string-type|||||||||||");
        CASE_STEPS.put("non-string-type", nonString.toArray(new String[0]));

        List<String> invalid = new ArrayList<>();
        invalid.add("deploy|invalid|table||" + EPL_INVALID_DEPLOY + "||||||||");
        invalid.add("deployed|invalid|table||||||||||");
        for (String[] probe : INVALID_PROBES) {
            invalid.add("build-error|invalid|" + probe[0] + "||" + probe[1]
                    + "||" + probe[2] + "||||||");
        }
        invalid.add("undeploy-all|invalid|||||||||||");
        CASE_STEPS.put("invalid", invalid.toArray(new String[0]));
    }

    private InfraTableCountMinSketchScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraTableCountMinSketchScenarioOracle <scenario.json>");
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
     * with the runtime path (env.compileDeploy(epl, path)); build-error
     * steps compile with the path, mirroring env.tryInvalidCompile(path,
     * ...); undeploy mirrors env.undeployModuleContaining(label) and
     * undeploy-all mirrors env.undeployAll().
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = config();
        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-" + ID + "-" + RUNTIME_IDS[caseIndex], configuration);
        runtime.getEventService().advanceTime(0);
        Map<String, String> deploymentIds = new HashMap<>();
        Map<String, Integer> sequences = new HashMap<>();
        Set<EPStatement> listened = Collections.newSetFromMap(new IdentityHashMap<>());
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
                    case "deploy":
                        deployStep(runtime, configuration, caseName, step, records,
                                deploymentIds, sequences, listened);
                        break;
                    case "deployed": {
                        String label = string(step, "statement");
                        records.add(marker(caseName, "deployed", label, sequences));
                        break;
                    }
                    case "send":
                        sendStep(runtime, step);
                        break;
                    case "build-error":
                        buildErrorStep(runtime, configuration, caseName, step, records);
                        break;
                    case "undeploy": {
                        String label = string(step, "statement");
                        String deploymentId = deploymentIds.remove(label);
                        if (deploymentId == null) {
                            throw new IllegalStateException("no deployment for undeploy " + label);
                        }
                        runtime.getDeploymentService().undeploy(deploymentId);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        deploymentIds.clear();
                        listened.clear();
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
     * the configuration plus the already-deployed module path so the join
     * and subquery modules resolve MyApproxFT.
     */
    private static EPCompiled compileModule(String epl, Configuration configuration,
                                            EPRuntime runtime) throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().add(runtime.getRuntimePath());
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    /**
     * Deploys the step's EPL and attaches listeners to the statements the
     * Java suite subscribes: frequency/topk on the ord-0 module, join and
     * subq on their own modules, and s0 on the ord-2 read.
     */
    private static void deployStep(EPRuntime runtime, Configuration configuration,
                                   String caseName, JsonObject step, JsonArray records,
                                   Map<String, String> deploymentIds,
                                   Map<String, Integer> sequences,
                                   Set<EPStatement> listened) throws Exception {
        String label = string(step, "statement");
        String epl = string(step, "epl");
        EPCompiled compiled = compileModule(epl, configuration, runtime);
        EPDeployment deployment = runtime.getDeploymentService()
                .deploy(compiled, new DeploymentOptions());
        deploymentIds.put(label, deployment.getDeploymentId());
        for (EPStatement stmt : deployment.getStatements()) {
            if (listenedStatement(caseName, stmt.getName())) {
                attachListener(caseName, stmt, records, sequences, listened);
            }
        }
    }

    private static boolean listenedStatement(String caseName, String name) {
        switch (caseName) {
            case "frequency-and-topk":
                return "frequency".equals(name) || "topk".equals(name)
                        || "join".equals(name) || "subq".equals(name);
            case "non-string-type":
                return "s0".equals(name);
            default:
                return false;
        }
    }

    private static JsonObject marker(String caseName, String operation, String label,
                                     Map<String, Integer> sequences) {
        String key = label + ":" + operation;
        int seq = sequences.getOrDefault(key, 0) + 1;
        sequences.put(key, seq);
        JsonObject rec = new JsonObject();
        rec.add("case", caseName);
        rec.add("operation", operation);
        rec.add("statement", label);
        rec.add("sequence", seq);
        rec.add("time", "0");
        return rec;
    }

    private static void attachListener(String caseName, EPStatement stmt, JsonArray records,
                                       Map<String, Integer> sequences,
                                       Set<EPStatement> listened) {
        if (listened.contains(stmt)) {
            return;
        }
        listened.add(stmt);
        stmt.addListener(new UpdateListener() {
            public void update(EventBean[] newEvents, EventBean[] oldEvents,
                               EPStatement statement, EPRuntime runtime) {
                String key = statement.getName() + ":listener";
                int seq = sequences.getOrDefault(key, 0) + 1;
                sequences.put(key, seq);
                JsonObject rec = new JsonObject();
                rec.add("case", caseName);
                rec.add("operation", "listener");
                rec.add("statement", statement.getName());
                rec.add("sequence", seq);
                rec.add("time", "0");
                if (newEvents != null && newEvents.length > 0) {
                    JsonArray newArr = new JsonArray();
                    for (EventBean event : newEvents) {
                        newArr.add(normalizeEvent(event));
                    }
                    rec.add("new", newArr);
                }
                if (oldEvents != null && oldEvents.length > 0) {
                    JsonArray oldArr = new JsonArray();
                    for (EventBean event : oldEvents) {
                        oldArr.add(normalizeEvent(event));
                    }
                    rec.add("old", oldArr);
                }
                records.add(rec);
            }
        });
    }

    private static JsonObject normalizeEvent(EventBean event) {
        JsonObject row = new JsonObject();
        row.add("kind", "row");
        JsonObject fields = new JsonObject();
        String[] names = event.getEventType().getPropertyNames();
        Arrays.sort(names);
        for (String name : names) {
            fields.add(name, normalizeValue(event.get(name)));
        }
        row.add("fields", fields);
        return row;
    }

    private static JsonValue normalizeValue(Object value) {
        if (value instanceof EventBean) {
            value = ((EventBean) value).getUnderlying();
        }
        if (value == null) {
            return Json.NULL;
        }
        if (value instanceof CountMinSketchTopK[]) {
            // The topk column renders as a JSON array of {value,frequency}
            // objects in emission order; the Go runner emits the same shape
            // for []CountMinSketchTopKItem.
            JsonArray array = new JsonArray();
            for (CountMinSketchTopK item : (CountMinSketchTopK[]) value) {
                JsonObject obj = new JsonObject();
                obj.add("value", normalizeValue(item.getValue()));
                obj.add("frequency", item.getFrequency());
                array.add(obj);
            }
            return array;
        }
        if (value instanceof CountMinSketchTopK) {
            CountMinSketchTopK item = (CountMinSketchTopK) value;
            JsonObject obj = new JsonObject();
            obj.add("value", normalizeValue(item.getValue()));
            obj.add("frequency", item.getFrequency());
            return obj;
        }
        if (value instanceof CountMinSketchAggState) {
            // The cell's object identity is not observable: the pinned suite
            // reads it only through countMinSketchFrequency, so render a
            // stable type marker.
            JsonObject obj = new JsonObject();
            obj.add("__type", "CountMinSketchAggState");
            return obj;
        }
        if (value instanceof SupportBean) {
            SupportBean bean = (SupportBean) value;
            JsonObject obj = new JsonObject();
            obj.add("theString", bean.getTheString());
            obj.add("intPrimitive", bean.getIntPrimitive());
            return obj;
        }
        if (value instanceof SupportBean_S0) {
            SupportBean_S0 bean = (SupportBean_S0) value;
            JsonObject obj = new JsonObject();
            obj.add("id", bean.getId());
            obj.add("p00", bean.getP00());
            obj.add("p01", bean.getP01());
            return obj;
        }
        if (value instanceof SupportBean_S1) {
            SupportBean_S1 bean = (SupportBean_S1) value;
            JsonObject obj = new JsonObject();
            obj.add("id", bean.getId());
            obj.add("p10", bean.getP10());
            return obj;
        }
        if (value instanceof SupportBean_S2) {
            SupportBean_S2 bean = (SupportBean_S2) value;
            JsonObject obj = new JsonObject();
            obj.add("id", bean.getId());
            obj.add("p20", bean.getP20());
            obj.add("p21", bean.getP21());
            return obj;
        }
        if (value instanceof SupportByteArrEventStringId) {
            SupportByteArrEventStringId bean = (SupportByteArrEventStringId) value;
            JsonObject obj = new JsonObject();
            obj.add("id", bean.getId());
            obj.add("body", Base64.getEncoder().encodeToString(bean.getBody()));
            return obj;
        }
        if (value instanceof EventBean[]) {
            JsonArray array = new JsonArray();
            for (EventBean event : (EventBean[]) value) {
                array.add(normalizeEvent(event));
            }
            return array;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalizeValue(item));
            }
            return array;
        }
        if (value instanceof Map) {
            JsonObject obj = new JsonObject();
            Map<?, ?> map = (Map<?, ?>) value;
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            Collections.sort(keys);
            for (String key : keys) {
                obj.add(key, normalizeValue(map.get(key)));
            }
            return obj;
        }
        if (value instanceof Double || value instanceof Float) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Compiles an expected-invalid probe and emits {"operation":"compile-error"}
     * after verifying the caught message starts with the pinned expectError
     * prefix (SupportMessageAssertUtil.assertMessage semantics).  All fifteen
     * probes compile with the runtime path, mirroring
     * env.tryInvalidCompile(path, ...).
     */
    private static void buildErrorStep(EPRuntime runtime, Configuration configuration,
                                       String caseName, JsonObject step, JsonArray records)
            throws Exception {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        String epl = string(step, "epl");
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            compilerArgs.getPath().add(runtime.getRuntimePath());
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
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        if (!expected.isEmpty()) {
            record.add("value", expected);
        }
        records.add(record);
    }

    private static void sendStep(EPRuntime runtime, JsonObject step) {
        String type = string(step, "eventType");
        JsonObject payload = object(step.get("payload"), "payload");
        Object event;
        switch (type) {
            case "SupportBean":
                event = new SupportBean(
                        string(payload, "theString"),
                        integer(payload, "intPrimitive"));
                break;
            case "SupportBean_S0":
                event = new SupportBean_S0(
                        integer(payload, "id"),
                        string(payload, "p00"));
                break;
            case "SupportBean_S1":
                event = new SupportBean_S1(integer(payload, "id"));
                break;
            case "SupportBean_S2":
                event = new SupportBean_S2(
                        integer(payload, "id"),
                        string(payload, "p20"));
                break;
            case "SupportByteArrEventStringId":
                event = new SupportByteArrEventStringId(
                        string(payload, "id"),
                        Base64.getDecoder().decode(string(payload, "body")));
                break;
            default:
                throw new IllegalStateException("unsupported event type " + type);
        }
        runtime.getEventService().sendEventBean(event, type);
    }

    private static Configuration config() {
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getCommon().addEventType(SupportBean_S2.class);
        configuration.getCommon().addEventType(SupportByteArrEventStringId.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        return configuration;
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
