import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Direct Esper 9.0.0 oracle for the six RowRecogGreedyness/RowRecogOps
 * executions replayed as one differential chain:
 *
 * reluctant-zero-to-one (Greedyness ord 0, RowRecogReluctantZeroToOne):
 * pattern (A?? B?) over a single E1(v=1) event; the reluctant A?? prefers
 * skipping A so the event binds to B, yielding {a_string:null,
 * b_string:'E1'} on both the listener and the iterator.
 *
 * reluctant-zero-to-many (Greedyness ord 1, RowRecogReluctantZeroToMany):
 * pattern (A*? B? C) with tag-indexed measures A[0..2]; the reluctant A*?
 * binds the fewest A events, so matches complete on E4, E15, E17 and E18.
 * The Java execution asserts the listener only.
 *
 * reluctant-one-to-many (Greedyness ord 2, RowRecogReluctantOneToMany):
 * the same measures under (A+? B? C); matches complete on E4, E15 and E17
 * while E18 produces no listener callback because A+? requires at least
 * one A event.
 *
 * unlimited-partition (Ops ord 5, RowRecogUnlimitedPartition): pattern
 * (A B) partitioned by value stresses the partition state repository
 * beyond INITIAL_COLLECTION_MIN=100: 500 A(i),B(i) pairs each invoke the
 * listener, 500 A-only sends at value i+100000 do not, and the 500 B
 * sends at value i+100000 complete those partitions — 1000 listener
 * records across 1000 partitions.
 *
 * alter-within-concat (Ops ord 7, RowRecogAlterWithinConcat): all-matches
 * pattern ( (A | B) (C | D) ); E6 completes {a:'E5',c:'E6'} and E8
 * completes {b:'E7',c:'E8'}, with the iterator accumulating both rows.
 *
 * regex (Ops ord 9, RowRecogRegex): pure JVM String.matches assertions
 * with zero Esper runtime surface; the scenario pins them as an
 * "unrepresentable" record that this oracle re-verifies before emitting.
 *
 * Mirroring SupportEvalRunner, each deployed case compiles
 * "@name('s0') <case EPL>" once, sends every assertion event and
 * snapshots the iterator where Java asserts it. The pinned case EPL is
 * the contract text verbatim including its irregular whitespace.
 * SupportRecogBean is declared as a map event type (theString string,
 * value int) because the regression-lib jar is not on the oracle
 * classpath. The TraceWriter skips null/null listener callbacks.
 */
public final class RowRecogGreedynessOpsScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "rowrecog-greedyness-ops";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogGreedyness.java";
    private static final String JAVA_SOURCE2 =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogOps.java";

    private static final String DESCRIPTION =
            "RowRecogGreedyness ordinals 0-2 plus RowRecogOps ordinals 5/7/9 (six executions): "
                    + "reluctant-zero-to-one pins A?? B? preferring the B binding on a single event; "
                    + "reluctant-zero-to-many and reluctant-one-to-many replay A*? B? C and A+? B? C "
                    + "over the E1-E18 send sequence; unlimited-partition stresses the partition "
                    + "state repository beyond INITIAL_COLLECTION_MIN=100 with 500 A,B pairs, 500 "
                    + "unmatched A-only sends and 500 completing B sends across 1000 partitions; "
                    + "alter-within-concat replays all-matches (A|B)(C|D); regex pins the JVM-only "
                    + "String.matches assertions as an unrepresentable record (no Esper runtime "
                    + "surface). Each deployed case compiles s0 with the verbatim Java EPL, sends "
                    + "SupportRecogBean events and snapshots the statement iterator where Java "
                    + "asserts it.";

    private static final String[] CASES = {
            "reluctant-zero-to-one", "reluctant-zero-to-many", "reluctant-one-to-many",
            "unlimited-partition", "alter-within-concat", "regex"};
    private static final int[] ORDINALS = {0, 1, 2, 5, 7, 9};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-6db0d9ba9099e88cb87c",
            "java-runtime-20afb4135dd06ce5fb8c",
            "java-runtime-470384aabd98754fe4f4",
            "java-runtime-ca4a83fa8b88dd4d2b32",
            "java-runtime-2e6e0566fe07043c23c7",
            "java-runtime-c489386411c57214eb75"
    };
    private static final String[] EXECUTIONS = {
            "RowRecogReluctantZeroToOne",
            "RowRecogReluctantZeroToMany",
            "RowRecogReluctantOneToMany",
            "RowRecogUnlimitedPartition",
            "RowRecogAlterWithinConcat",
            "RowRecogRegex"
    };
    private static final String[] STATIC_IDS = {
            "java-26ecc8f683281497dc04",
            "java-03abe565af0f2440f2bd"
    };
    private static final String[] OBSERVATIONS = {
            "listener+iterator; the single E1 event binds to B because reluctant A?? prefers "
                    + "skipping A",
            "listener; four matches complete on E4, E15, E17 and E18 (reluctant A*? binds the "
                    + "fewest A events)",
            "listener; three matches complete on E4, E15 and E17; E18 produces no callback "
                    + "because A+? requires at least one A event",
            "listener; 500 A,B pairs each invoke the listener, 500 A-only sends at value "
                    + "i+100000 do not, and the 500 B sends complete those partitions — 1000 "
                    + "listener records across 1000 partitions",
            "listener+iterator; all-matches (A|B)(C|D) — E6 completes {a:E5,c:E6} and E8 "
                    + "completes {b:E7,c:E8}",
            "unrepresentable; pure JVM String.matches assertions with no Esper runtime surface "
                    + "— the pinned record carries the assertion matrix"
    };
    private static final String[] CASE_EPLS = {
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string   pattern (A?? B?)   define    A as A.value = 1,   B as B.value = 1)",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A[0].theString as a0, A[1].theString as a1, A[2].theString as a2, B.theString as b, C.theString as c  pattern (A*? B? C)   define    A as A.value = 1,   B as B.value in (1, 2),   C as C.value = 3)",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A[0].theString as a0, A[1].theString as a1, A[2].theString as a2, B.theString as b, C.theString as c  pattern (A+? B? C)   define    A as A.value = 1,   B as B.value in (1, 2),   C as C.value = 3)",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  partition by value  measures A.theString as a_string   pattern (A B)   define     A as (A.theString = 'A'),    B as (B.theString = 'B'))",
            "@name('s0') select * from SupportRecogBean#keepall match_recognize (  measures A.theString as a_string, B.theString as b_string, C.theString as c_string, D.theString as d_string   all matches pattern ( (A | B) (C | D) )   define     A as (A.value = 1),    B as (B.value = 2),    C as (C.value = 3),    D as (D.value = 4))",
            ""
    };

    private static final String UNREPRESENTABLE_STATEMENT = "string-matches";
    private static final String UNREPRESENTABLE_NOTE =
            "JVM-only String.matches assertions (no Esper runtime surface): \"aq\" and \"id\" "
                    + "match \"^aq|^id\"; \"ad\", \"aqd\" and \"aid\" match \"a(q|i)?d\" while "
                    + "\"aed\" does not; \"a\" does not match \"(a(b?)c)?\"";

    // Pinned op sequences per case (after the case marker).
    private static final String[][] CASE_OPS = {
            {"deploy", "send", "snapshot", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "undeploy-all"},
            {"deploy", "send", "send", "send", "send", "send", "send", "send", "send",
                    "send", "send", "send", "send", "undeploy-all"},
            unlimitedPartitionOps(),
            {"deploy", "send", "send", "send", "send", "send", "snapshot", "send",
                    "snapshot", "send", "send", "snapshot", "undeploy-all"},
            {"unrepresentable"}
    };

    // Pinned send payloads per case, in send order, encoded "theString:value".
    private static final String[][] CASE_SENDS = {
            {"E1:1"},
            {"E1:1", "E2:1", "E3:1", "E4:3", "E11:1", "E12:1", "E13:1", "E14:1",
                    "E15:3", "E16:1", "E17:3", "E18:3"},
            {"E1:1", "E2:1", "E3:1", "E4:3", "E11:1", "E12:1", "E13:1", "E14:1",
                    "E15:3", "E16:1", "E17:3", "E18:3"},
            unlimitedPartitionSends(),
            {"E1:3", "E2:1", "E3:2", "E4:5", "E5:1", "E6:3", "E7:2", "E8:3"},
            {}
    };

    private static final int EXPECTED_STEPS = 2054;
    private static final int EXPECTED_RECORDS = 1015;

    private RowRecogGreedynessOpsScenarioOracle() {
    }

    // Ops ord 5: deploy, then 500 A(i),B(i) pairs (1000 sends), 500 A-only
    // sends at value i+100000 and 500 B sends at value i+100000 — 2000
    // sends total — then undeploy-all.
    private static String[] unlimitedPartitionOps() {
        String[] ops = new String[2002];
        ops[0] = "deploy";
        Arrays.fill(ops, 1, 2001, "send");
        ops[2001] = "undeploy-all";
        return ops;
    }

    private static String[] unlimitedPartitionSends() {
        String[] sends = new String[2000];
        int index = 0;
        for (int i = 0; i < 500; i++) {
            sends[index++] = "A:" + i;
            sends[index++] = "B:" + i;
        }
        for (int i = 0; i < 500; i++) {
            sends[index++] = "A:" + (i + 100000);
        }
        for (int i = 0; i < 500; i++) {
            sends[index++] = "B:" + (i + 100000);
        }
        return sends;
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: RowRecogGreedynessOpsScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);

        JsonArray records = new JsonArray();
        JsonArray steps = scenario.get("steps").asArray();
        for (int index = 0; index < CASES.length; index++) {
            runCase(steps, CASES[index], RUNTIME_IDS[index], index, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    private static void runCase(JsonArray steps, String caseName, String runtimeId,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("value", Integer.class);
        configuration.getCommon().addEventType("SupportRecogBean", beanType);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = null;
            int deployCount = 0;
            boolean active = false;
            for (int index = 0; index < steps.size(); index++) {
                JsonObject step = steps.get(index).asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                if ("deploy".equals(operation)) {
                    String epl = step.getString("epl", "");
                    EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl,
                            new CompilerArguments(runtime.getRuntimePath()));
                    EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                            new DeploymentOptions().setDeploymentId(
                                    SCENARIO_ID + "-" + caseIndex + "-" + deployCount));
                    deployCount++;
                    writer = new TraceWriter(records, caseName, findStatement(deployment), runtime);
                    writer.statement.addListener(writer);
                } else if ("undeploy-all".equals(operation)) {
                    runtime.getDeploymentService().undeployAll();
                    writer = null;
                } else if ("send".equals(operation)) {
                    sendEvent(runtime, step);
                } else if ("snapshot".equals(operation)) {
                    if (writer == null) {
                        throw new IllegalStateException("snapshot without a deployed statement");
                    }
                    writer.snapshot();
                } else if ("unrepresentable".equals(operation)) {
                    unrepresentableStep(caseName, step, records);
                } else {
                    throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Emits the pinned "unrepresentable" record for the RowRecogRegex
     * execution after re-running the JVM-only String.matches assertion
     * matrix the record documents; the Go side only pins the note.
     */
    private static void unrepresentableStep(String caseName, JsonObject step,
                                            JsonArray records) {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        if (!UNREPRESENTABLE_STATEMENT.equals(label) || !UNREPRESENTABLE_NOTE.equals(note)) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " carries an unpinned note");
        }
        if (!"aq".matches("^aq|^id") || !"id".matches("^aq|^id")
                || !"ad".matches("a(q|i)?d") || !"aqd".matches("a(q|i)?d")
                || !"aid".matches("a(q|i)?d") || "aed".matches("a(q|i)?d")
                || "a".matches("(a(b?)c)?")) {
            throw new IllegalStateException(
                    "String.matches assertions drifted from the pinned note");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement candidate : deployment.getStatements()) {
            if ("s0".equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String eventType = step.getString("eventType", "");
        if (!"SupportRecogBean".equals(eventType)) {
            throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
        JsonObject payload = step.get("payload").asObject();
        Map<String, Object> event = new HashMap<>();
        event.put("theString", payload.getString("theString", null));
        event.put("value", payload.get("value").asInt());
        runtime.getEventService().sendEventMap(event, "SupportRecogBean");
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSource2", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))
                || !JAVA_SOURCE2.equals(string(scenario, "javaSource2"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly six cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case definition " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " metadata is not pinned");
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        validateSteps(steps);
    }

    /**
     * Pins the full step sequence: each case marker is followed by the
     * case's pinned ops — deploy s0 with the verbatim EPL, the assertion
     * sends with pinned payloads, iterator snapshots, undeploy-all, and
     * the regex case's single unrepresentable record. Unknown step fields
     * are rejected.
     */
    private static void validateSteps(JsonArray steps) {
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps, got " + steps.size());
        }
        int cursor = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(cursor), "case marker " + cursor);
            requireFields(marker, "op", "case");
            if (!"case".equals(string(marker, "op")) || !CASES[caseIndex].equals(string(marker, "case"))) {
                throw new IllegalArgumentException("case marker " + cursor + " is not pinned");
            }
            cursor++;
            int sends = 0;
            for (String operation : CASE_OPS[caseIndex]) {
                JsonObject step = object(steps.get(cursor), "step " + cursor);
                if (!operation.equals(string(step, "op"))
                        || !CASES[caseIndex].equals(string(step, "case"))) {
                    throw new IllegalArgumentException("step " + cursor + " is not pinned");
                }
                switch (operation) {
                    case "deploy":
                        requireFields(step, "op", "case", "statement", "epl");
                        if (!"s0".equals(string(step, "statement"))
                                || !CASE_EPLS[caseIndex].equals(string(step, "epl"))) {
                            throw new IllegalArgumentException("deploy step " + cursor + " is not pinned");
                        }
                        break;
                    case "send":
                        requireFields(step, "op", "case", "eventType", "payload");
                        if (!"SupportRecogBean".equals(string(step, "eventType"))) {
                            throw new IllegalArgumentException("send step " + cursor + " is not pinned");
                        }
                        JsonObject payload = object(step.get("payload"), "send payload " + cursor);
                        requireFields(payload, "theString", "value");
                        String expected = CASE_SENDS[caseIndex][sends++];
                        String actual = string(payload, "theString") + ":" + integer(payload, "value");
                        if (!expected.equals(actual)) {
                            throw new IllegalArgumentException("send payload " + cursor
                                    + " is not pinned: expected " + expected + " got " + actual);
                        }
                        break;
                    case "snapshot":
                        requireFields(step, "op", "case", "statement");
                        if (!"s0".equals(string(step, "statement"))) {
                            throw new IllegalArgumentException("snapshot step " + cursor + " is not pinned");
                        }
                        break;
                    case "undeploy-all":
                        requireFields(step, "op", "case");
                        break;
                    case "unrepresentable":
                        requireFields(step, "op", "case", "statement", "expectError");
                        if (!UNREPRESENTABLE_STATEMENT.equals(string(step, "statement"))
                                || !UNREPRESENTABLE_NOTE.equals(string(step, "expectError"))) {
                            throw new IllegalArgumentException(
                                    "unrepresentable step " + cursor + " is not pinned");
                        }
                        break;
                    default:
                        throw new IllegalArgumentException("unsupported operation at step " + cursor);
                }
                cursor++;
            }
            if (sends != CASE_SENDS[caseIndex].length) {
                throw new IllegalArgumentException("case " + caseIndex + " send count is not pinned");
            }
        }
        if (cursor != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
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

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null || !value.isString()) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longNumber(object, name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        return (int) value;
    }

    private static long longNumber(JsonObject object, String name) {
        if (!(object.get(name) instanceof JsonNumber)) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        String text = object.get(name).toString();
        if (!text.matches("-?(0|[1-9][0-9]*)")) {
            throw new IllegalArgumentException(name + " must be an integer JSON number");
        }
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(name + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        JsonArray actual = array(value, name);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(name + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (item == null || !item.isString() || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + index);
            }
        }
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            append("listener", ++sequence, newEvents, oldEvents);
        }

        private void snapshot() {
            List<EventBean> events = new ArrayList<>();
            Iterator<EventBean> iterator = statement.iterator();
            while (iterator.hasNext()) {
                events.add(iterator.next());
            }
            append("snapshot", 0, events.toArray(new EventBean[0]), null);
        }

        private void append(String operation, long sequence, EventBean[] newEvents, EventBean[] oldEvents) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement.getName())
                    .add("sequence", sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof BigDecimal) {
                return Json.value(((BigDecimal) value).toPlainString());
            }
            if (value instanceof BigInteger) {
                return Json.value(value.toString());
            }
            if (value instanceof EventBean[] events) {
                JsonArray array = new JsonArray();
                for (EventBean event : events) {
                    array.add(normalize(event));
                }
                return array;
            }
            if (value instanceof EventBean event) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                return new JsonObject().add("kind", "row").add("fields", fields);
            }
            if (value instanceof Object[] objects) {
                JsonArray array = new JsonArray();
                for (Object object : objects) {
                    array.add(normalize(object));
                }
                return array;
            }
            if (value instanceof Integer || value instanceof Long || value instanceof Short
                    || value instanceof Byte) {
                return Json.value(((Number) value).longValue());
            }
            if (value instanceof Number) {
                double number = ((Number) value).doubleValue();
                if (number == Math.rint(number) && !Double.isInfinite(number)) {
                    return Json.value((long) number);
                }
                return Json.value(number);
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            if (value instanceof Character character) {
                return Json.value(String.valueOf(character));
            }
            return Json.value(String.valueOf(value));
        }
    }
}
