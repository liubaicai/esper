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
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableContext ordinals 0 and 1: the parameterized
 * InfraContext execution (namedWindow=true/false) replays a contexted
 * keepall named window and a contexted primary-key table under the
 * non-overlapping ContextOne init-term context. Two executions on two
 * runtimes.
 *
 * Both cases deploy `@public create context ContextOne start SupportBean_S0
 * end SupportBean_S1`, the @public contexted create-infra statement
 * (`create window MyInfra#keepall as (pkey0 string, pkey1 int, c0 long)` or
 * `create table MyInfra as (pkey0 string primary key, pkey1 int primary
 * key, c0 long)`) and the contexted insert-into feed over SupportBean.
 * SupportBean_S0(0) opens the partition, SupportBean(E1,10,100) and
 * SupportBean(E2,20,200) land in it, and the six `output snapshot when
 * terminated` selects s1..s6 deploy late into the already-active partition.
 * SupportBean_S1(0) then terminates the partition and every statement fires
 * exactly once: s1 emits both window/table rows, s2 the single count 2, s3
 * one row per infra row carrying the total count, s4 the per-pkey0 counts,
 * s5 the group-member pkey1 values, and s6 the rollup grouping sets with
 * null super-aggregate columns.
 *
 * Java milestone(0) is a harness no-op and carries no step. The listener
 * batches sort rows by their compact field rendering because the Java
 * execution asserts them with assertPropsPerRowLastNewAnyOrder (s2 uses
 * assertPropsNew, a single new row).
 */
public final class InfraNWTableContextScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-context";
    private static final String DESCRIPTION =
            "InfraNWTableContext ords 0 and 1: the non-overlapping ContextOne "
                    + "init-term context (start SupportBean_S0 end SupportBean_S1) "
                    + "binds a keepall named window (ord 0) or a two-primary-key "
                    + "table (ord 1) fed by a contexted insert-into over "
                    + "SupportBean; six output-snapshot-when-terminated selects "
                    + "(wildcard, count(*), ungrouped per-row count, group-by "
                    + "pkey0, group-by pkey0 with member pkey1, and "
                    + "rollup(pkey0,pkey1)) deploy late into the active "
                    + "partition and each fires once when SupportBean_S1(0) "
                    + "ends it (Java source regression-lib/src/main/java/com/"
                    + "espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableContext.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableContext.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-dfaacac6bb82c21d6d47",
            "java-runtime-913c09693fb262b75d75"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraContext{namedWindow=true}",
            "InfraContext{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-504b139a33ff22d20d65",
            "java-504b139a33ff22d20d65"
    };
    private static final String[] JAVA_FLAGS = {};
    private static final String[] CASES = {
            "named-window",
            "table"
    };
    private static final int[] ORDINALS = {0, 1};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; six late-deployed output-snapshot-when-terminated selects "
                    + "over the contexted keepall window each fire once when "
                    + "SupportBean_S1(0) terminates the partition: s1 "
                    + "{E1,10,100},{E2,20,200}; s2 {thecnt=2}; s3 {E1,2},{E2,2}; "
                    + "s4 {E1,1},{E2,1}; s5 {E1,10,1},{E2,20,1}; s6 adds rollup "
                    + "super-aggregate rows {E1,null,1},{E2,null,1},{null,null,2}",
            "listener; six late-deployed output-snapshot-when-terminated selects "
                    + "over the contexted two-primary-key table each fire once "
                    + "when SupportBean_S1(0) terminates the partition: s1 "
                    + "{E1,10,100},{E2,20,200}; s2 {thecnt=2}; s3 {E1,2},{E2,2}; "
                    + "s4 {E1,1},{E2,1}; s5 {E1,10,1},{E2,20,1}; s6 adds rollup "
                    + "super-aggregate rows {E1,null,1},{E2,null,1},{null,null,2}"
    };

    // Verbatim transcriptions of InfraNWTableContext lines 41, 43-45, 48 and
    // 55-60: each compileDeploy is a separate module sharing one
    // RegressionPath, so @public on the context and the infra is required
    // for cross-module visibility. `@name('sN')` is concatenated directly
    // onto `context` — no space.
    private static final String EPL_CTX =
            "@public create context ContextOne start SupportBean_S0 end SupportBean_S1";
    private static final String EPL_CREATE_NW =
            "@public context ContextOne create window MyInfra#keepall as "
                    + "(pkey0 string, pkey1 int, c0 long)";
    private static final String EPL_CREATE_TABLE =
            "@public context ContextOne create table MyInfra as "
                    + "(pkey0 string primary key, pkey1 int primary key, c0 long)";
    private static final String EPL_INSERT =
            "context ContextOne insert into MyInfra select theString as pkey0, "
                    + "intPrimitive as pkey1, longPrimitive as c0 from SupportBean";
    private static final String EPL_S1 =
            "@name('s1')context ContextOne select * from MyInfra "
                    + "output snapshot when terminated";
    private static final String EPL_S2 =
            "@name('s2')context ContextOne select count(*) as thecnt from MyInfra "
                    + "output snapshot when terminated";
    private static final String EPL_S3 =
            "@name('s3')context ContextOne select pkey0, count(*) as thecnt from MyInfra "
                    + "output snapshot when terminated";
    private static final String EPL_S4 =
            "@name('s4')context ContextOne select pkey0, count(*) as thecnt from MyInfra "
                    + "group by pkey0 output snapshot when terminated";
    private static final String EPL_S5 =
            "@name('s5')context ContextOne select pkey0, pkey1, count(*) as thecnt "
                    + "from MyInfra group by pkey0 output snapshot when terminated";
    private static final String EPL_S6 =
            "@name('s6')context ContextOne select pkey0, pkey1, count(*) as thecnt "
                    + "from MyInfra group by rollup (pkey0, pkey1) "
                    + "output snapshot when terminated";

    private static final Set<String> LISTENED_STATEMENTS =
            new HashSet<>(Arrays.asList("s1", "s2", "s3", "s4", "s5", "s6"));
    private static final int EXPECTED_STEPS = 30;
    private static final int EXPECTED_RECORDS = 12;

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields.  Deploy steps carry the byte-exact EPL text;
     * send payloads render as their compact JSON.
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("named-window", caseSteps("named-window", EPL_CREATE_NW));
        CASE_STEPS.put("table", caseSteps("table", EPL_CREATE_TABLE));
    }

    private static String[] caseSteps(String caseName, String createEpl) {
        return new String[]{
                "deploy|" + caseName + "|ctx||" + EPL_CTX + "|||||||",
                "deploy|" + caseName + "|create||" + createEpl + "|||||||",
                "deploy|" + caseName + "|insert||" + EPL_INSERT + "|||||||",
                "send|" + caseName + "||SupportBean_S0||{\"id\":0}||||||",
                "send|" + caseName + "||SupportBean||"
                        + "{\"theString\":\"E1\",\"intPrimitive\":10,\"longPrimitive\":100}||||||",
                "send|" + caseName + "||SupportBean||"
                        + "{\"theString\":\"E2\",\"intPrimitive\":20,\"longPrimitive\":200}||||||",
                "deploy|" + caseName + "|s1||" + EPL_S1 + "|||||||",
                "deploy|" + caseName + "|s2||" + EPL_S2 + "|||||||",
                "deploy|" + caseName + "|s3||" + EPL_S3 + "|||||||",
                "deploy|" + caseName + "|s4||" + EPL_S4 + "|||||||",
                "deploy|" + caseName + "|s5||" + EPL_S5 + "|||||||",
                "deploy|" + caseName + "|s6||" + EPL_S6 + "|||||||",
                "send|" + caseName + "||SupportBean_S1||{\"id\":0}||||||",
                "undeploy-all|" + caseName + "||||||||||",
        };
    }

    private InfraNWTableContextScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableContextScenarioOracle <scenario.json>");
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
     * Replays one case's steps on a fresh runtime (each Java execution gets
     * its own runtime). SupportBean, SupportBean_S0 and SupportBean_S1 are
     * preconfigured, the internal timer is disabled and the rethrowing
     * exception handler surfaces statement failures to the sender thread.
     * Every deploy compiles its EPL as one module against the runtime path
     * (the execution's shared RegressionPath) and attaches the listener to
     * each s1..s6 statement, mirroring register(env, path, num, epl).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_S1.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
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
                        EPDeployment deployment = compileDeploy(runtime, epl);
                        for (EPStatement statement : deployment.getStatements()) {
                            if (LISTENED_STATEMENTS.contains(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
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

    /** compileDeploy mirrors env.compileDeploy(epl, path): module compile
     * against the runtime path followed by a deployment. */
    private static EPDeployment compileDeploy(EPRuntime runtime, String epl)
            throws EPCompileException, EPDeployException {
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; the default istream selector means only a new
     * array renders and only when non-empty. Rows sort by their compact
     * field rendering because the Java execution asserts the batches with
     * assertPropsPerRowLastNewAnyOrder.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = sortedRows(rows(newEvents));
            JsonArray oldRows = sortedRows(rows(oldEvents));
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering with sorted property names for a stable field order. */
    private static JsonArray rows(EventBean[] events) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            String[] names = event.getEventType().getPropertyNames().clone();
            Arrays.sort(names);
            JsonObject fields = new JsonObject();
            for (String name : names) {
                fields.add(name, normalize(event.get(name)));
            }
            item.add("fields", fields);
            array.add(item);
        }
        return array;
    }

    /** Sorts rendered rows by their compact field JSON so the any-order
     * Java assertions pin one canonical order on both traces. */
    private static JsonArray sortedRows(JsonArray rows) {
        List<JsonValue> items = new ArrayList<>();
        for (JsonValue item : rows) {
            items.add(item);
        }
        items.sort((left, right) -> left.asObject().get("fields").toString()
                .compareTo(right.asObject().get("fields").toString()));
        JsonArray sorted = new JsonArray();
        for (JsonValue item : items) {
            sorted.add(item);
        }
        return sorted;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, and Object[]/int[] row underlyings as JSON
     * arrays.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Object[]) {
            JsonArray items = new JsonArray();
            for (Object element : (Object[]) value) {
                items.add(normalize(element));
            }
            return items;
        }
        if (value instanceof int[]) {
            JsonArray items = new JsonArray();
            for (int element : (int[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Sends one pinned event: SupportBean carries theString/intPrimitive
     * plus longPrimitive through its setter (mirroring
     * makeSendSupportBean), while SupportBean_S0 and SupportBean_S1 carry
     * only id (the single-argument constructors the Java execution uses).
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean event = new SupportBean(
                        string(payload, "theString"),
                        intField(payload.get("intPrimitive"), "intPrimitive"));
                event.setLongPrimitive(longField(payload.get("longPrimitive"), "longPrimitive"));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(
                        intField(payload.get("id"), "id"));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            case "SupportBean_S1": {
                SupportBean_S1 event = new SupportBean_S1(
                        intField(payload.get("id"), "id"));
                runtime.getEventService().sendEventBean(event, type);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
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
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
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

    /** The pinned cases[] epl: the newline-joined EPL of every deploy step
     * in the case, in step order. */
    private static String caseEpl(String caseName) {
        String create = "named-window".equals(caseName) ? EPL_CREATE_NW : EPL_CREATE_TABLE;
        return String.join("\n", EPL_CTX, create, EPL_INSERT,
                EPL_S1, EPL_S2, EPL_S3, EPL_S4, EPL_S5, EPL_S6);
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
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields with the payload compacted and ids/fields
     * rendered as JSON arrays.  Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields"));
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
                + "|" + string(step, "expectError") + "|" + cwp
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

    private static int intField(JsonValue value, String label) {
        long parsed = longInteger(value, label);
        if (parsed < Integer.MIN_VALUE || parsed > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(label + " is outside the Java int range");
        }
        return (int) parsed;
    }

    private static long longField(JsonValue value, String label) {
        return longInteger(value, label);
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
