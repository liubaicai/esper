import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
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
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
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
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableOnMerge ordinals 4-7: nested-event merge
 * assignment and insert-stream merge.
 *
 * InfraUpdateNestedEvent (ords 4-5) runs two sub-scenarios per execution —
 * metaType "map" then "objectarray" — each a full compileDeploy(path) ->
 * send -> milestoneInc -> FAF assert -> undeployAll cycle; the scenario
 * splits them into four cases (nested-nw-map, nested-nw-oa,
 * nested-table-map, nested-table-oa) sharing the execution's runtime ID.
 * The six-statement module declares the Composite/AInfraType/MyEvent
 * schemas, the AInfra lastevent window or unkeyed table, the SupportBean
 * insert feed and the matched merge that assigns the Composite-valued
 * cflat/carr columns wholesale.  The only observable is the fire-and-forget
 * select reading cflat.c0, carr[0].c0 and carr[1].c0.
 *
 * InfraOnMergeInsertStream (ords 6-7) deploys one seven-statement module
 * (schema, Create window/table, the unnamed merge, s1-s4 consumers) and
 * attaches listeners to s1-s4 only.  Both SupportBean_ST0 sends take the
 * not-matched branch, whose five ordered insert actions route the wildcard
 * row to StreamOne, id+key0 projections to StreamTwo/StreamThree, the
 * key0="K2"-filtered row to StreamFour, and the target row into WinOMIS.
 * The 'Create' iterator reads the target in order after the second send.
 *
 * Replays the six cases on one runtime with undeployAll between cases,
 * mirroring the regression-suite harness: SupportBean from esper-common
 * plus a local mirror of the regression SupportBean_ST0 (regression-lib is
 * not on the oracle classpath), internal timer disabled, and the rethrowing
 * exception handler so statement failures surface to the sender thread.
 * Deployed markers bind module statements positionally in EPL order;
 * milestone calls are HA checkpoint no-ops and emit nothing.
 */
public final class InfraNWTableOnMergeNestedScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-nested";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 4-7: nested-event merge assignment of map and "
                    + "objectarray Composite payloads over a lastevent named window and an "
                    + "unkeyed table asserted by fire-and-forget select, and insert-stream "
                    + "merge whose single not-matched branch runs five ordered insert actions "
                    + "into three side streams, a where-filtered side stream, and the merge "
                    + "target (Java source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-065003de88aca37795b8",
            "java-runtime-065003de88aca37795b8",
            "java-runtime-2e8d691b5e2c927038d7",
            "java-runtime-2e8d691b5e2c927038d7",
            "java-runtime-2c687a68317caea3c148",
            "java-runtime-081455731ccafbf6847c"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraUpdateNestedEvent{namedWindow=true}",
            "InfraUpdateNestedEvent{namedWindow=true}",
            "InfraUpdateNestedEvent{namedWindow=false}",
            "InfraUpdateNestedEvent{namedWindow=false}",
            "InfraOnMergeInsertStream{namedWindow=true}",
            "InfraOnMergeInsertStream{namedWindow=false}"
    };
    private static final String[] STATIC_IDS = {
            "java-712b26dbe20bbda50f37",
            "java-712b26dbe20bbda50f37",
            "java-712b26dbe20bbda50f37",
            "java-712b26dbe20bbda50f37",
            "java-9bcebf3cac321ec7aee2",
            "java-9bcebf3cac321ec7aee2"
    };
    private static final String[] CASES = {
            "nested-nw-map", "nested-nw-oa", "nested-table-map", "nested-table-oa",
            "insertstream-nw", "insertstream-table"
    };
    private static final int[] ORDINALS = {4, 4, 5, 5, 6, 7};
    private static final String[] CASE_OBSERVATIONS = {
            "faf; map sub-run of the nested-event merge: a six-statement module declares "
                    + "Composite/AInfraType/MyEvent map schemas and a lastevent window, the "
                    + "SupportBean insert seeds k=E1 with null composites, the MyEvent merge "
                    + "assigns cf/ca wholesale, and the FAF select reads cflat.c0=1, "
                    + "carr[0].c0=1, carr[1].c0=2",
            "faf; objectarray sub-run of the nested-event merge over the lastevent window: "
                    + "same module shape with objectarray schemas, positional MyEvent payload "
                    + "[[1],[[1],[2]]], identical FAF assertion",
            "faf; map sub-run of the nested-event merge over the unkeyed table (no primary "
                    + "key): same module shape, single-row table updated by the matched merge, "
                    + "identical FAF assertion",
            "faf; objectarray sub-run of the nested-event merge over the unkeyed table: same "
                    + "module shape with objectarray schemas, positional MyEvent payload, "
                    + "identical FAF assertion",
            "listener+iterator; not-matched merge over the keepall window runs five ordered "
                    + "insert actions: StreamOne gets the wildcard row, StreamTwo/Three get "
                    + "id+key0, StreamFour filters key0=K2, and the target insert lands last; "
                    + "s1-s3 fire on both sends, s4 only on K2, Create iterates {K1,1},{K2,2}",
            "listener+iterator; same five-action not-matched merge over the primary-key "
                    + "table: s1-s3 fire on both sends, s4 only on K2, Create iterates "
                    + "{K1,1},{K2,2}"
    };

    // Verbatim transcriptions of InfraNWTableOnMerge lines 1336-1343 (nested
    // module, metaType substituted) and 1048-1062 (insert-stream module,
    // including the trailing ";\n" after the merge statement and after s4).
    private static String nestedModuleEpl(String metaType, boolean namedWindow) {
        return "@public create " + metaType + " schema Composite as (c0 int);\n"
                + "@buseventtype @public create " + metaType
                + " schema AInfraType as (k string, cflat Composite, carr Composite[]);\n"
                + (namedWindow
                ? "@public create window AInfra#lastevent as AInfraType;\n"
                : "@public create table AInfra (k string, cflat Composite, carr Composite[]);\n")
                + "insert into AInfra select theString as k, null as cflat, null as carr "
                + "from SupportBean;\n"
                + "@public @buseventtype create " + metaType
                + " schema MyEvent as (cf Composite, ca Composite[]);\n"
                + "on MyEvent e merge AInfra when matched then update set cflat = e.cf, "
                + "carr = e.ca";
    }

    private static String insertStreamModuleEpl(boolean namedWindow) {
        return "create schema WinOMISSchema as (v1 string, v2 int);\n"
                + (namedWindow
                ? "@name('Create') create window WinOMIS#keepall as WinOMISSchema;\n"
                : "@name('Create') create table WinOMIS as (v1 string primary key, v2 int);\n")
                + "on SupportBean_ST0 as st0 merge WinOMIS as win where win.v1=st0.key0 "
                + "when not matched "
                + "then insert into StreamOne select * "
                + "then insert into StreamTwo select st0.id as id, st0.key0 as key0 "
                + "then insert into StreamThree(id, key0) select st0.id, st0.key0 "
                + "then insert into StreamFour select id, key0 where key0=\"K2\" "
                + "then insert into WinOMIS select key0 as v1, p00 as v2;\n"
                + "@name('s1') select * from StreamOne;\n"
                + "@name('s2') select * from StreamTwo;\n"
                + "@name('s3') select * from StreamThree;\n"
                + "@name('s4') select * from StreamFour;\n";
    }

    private static final String[] CASE_EPLS = {
            nestedModuleEpl("map", true),
            nestedModuleEpl("objectarray", true),
            nestedModuleEpl("map", false),
            nestedModuleEpl("objectarray", false),
            insertStreamModuleEpl(true),
            insertStreamModuleEpl(false)
    };
    private static final String EPL_FAF =
            "select cflat.c0 as cf0, carr[0].c0 as ca0, carr[1].c0 as ca1 from AInfra";
    private static final String[] FAF_FIELDS = {"cf0", "ca0", "ca1"};
    private static final String[] CREATE_FIELDS = {"v1", "v2"};

    // Module statement labels in EPL order; unnamed statements bind by
    // position like the silent-delete oracle's insert label.
    private static final String[] NESTED_LABELS = {
            "schema-composite", "schema-ainfra", "infra", "insert", "schema-myevent", "merge"
    };
    private static final String[] INSERTSTREAM_LABELS = {
            "schema", "create", "merge", "s1", "s2", "s3", "s4"
    };
    private static final Set<String> INSERTSTREAM_LISTENED =
            new HashSet<>(Arrays.asList("s1", "s2", "s3", "s4"));

    private static final int EXPECTED_STEPS = 74;
    private static final int EXPECTED_RECORDS = 58;

    private InfraNWTableOnMergeNestedScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergeNestedScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        Configuration configuration = new Configuration();
        configuration.getCommon().addEventType(SupportBean.class);
        configuration.getCommon().addEventType(SupportBean_ST0.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            for (String caseName : CASES) {
                runCase(caseName, configuration, runtime, allSteps, records);
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
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
     * Replays one case's steps on the shared runtime; sequences restart per
     * case.  The single module deploy binds its statements positionally to
     * the pinned labels so deployed markers and the 'Create' snapshot can
     * resolve them; listeners attach to s1-s4 for the insert-stream cases,
     * matching env.compileDeploy(epl).addListener("s1")...addListener("s4").
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        String[] labels = isInsertStream(caseName) ? INSERTSTREAM_LABELS : NESTED_LABELS;
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
                    CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                    EPCompiled compiled = EPCompilerProvider.getCompiler()
                            .compile(epl, compilerArgs);
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    EPStatement[] deployed = deployment.getStatements();
                    if (deployed.length != labels.length) {
                        throw new IllegalStateException("module deployment of " + caseName
                                + " has " + deployed.length + " statements, want "
                                + labels.length);
                    }
                    for (int index = 0; index < deployed.length; index++) {
                        EPStatement statement = deployed[index];
                        if (INSERTSTREAM_LISTENED.contains(statement.getName())) {
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                        statements.put(labels[index], statement);
                    }
                    break;
                }
                case "deployed": {
                    String label = string(step, "statement");
                    if (!statements.containsKey(label)) {
                        throw new IllegalStateException(
                                "deployed marker for unknown statement " + label);
                    }
                    int sequence = sequences.merge(label + ":deployed", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "deployed");
                    record.add("statement", label);
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    records.add(record);
                    break;
                }
                case "send":
                    sendEvent(runtime, caseName, string(step, "eventType"),
                            step.get("payload"));
                    break;
                case "snapshot": {
                    String label = string(step, "statement");
                    EPStatement statement = statements.get(label);
                    if (statement == null) {
                        throw new IllegalStateException("snapshot statement " + label
                                + " was not deployed in case " + caseName);
                    }
                    String[] fields = stringArray(step.get("fields"), "fields");
                    JsonArray rows = new JsonArray();
                    for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
                        rows.add(projectedRow(iterator.next(), fields));
                    }
                    if ("any".equals(string(step, "mode"))) {
                        sortRowsCanonical(rows);
                    }
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "snapshot");
                    record.add("statement", statement.getName());
                    record.add("sequence", 0);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    if (rows.size() > 0) {
                        record.add("new", rows);
                    }
                    records.add(record);
                    break;
                }
                case "faf": {
                    // Fire-and-forget select against the module's public
                    // AInfra infra, mirroring compileExecuteFAF(epl, path):
                    // compiled as a query with the runtime path and executed
                    // on demand.
                    CompilerArguments fafArgs = new CompilerArguments(configuration);
                    fafArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled query = EPCompilerProvider.getCompiler()
                            .compileQuery(string(step, "epl"), fafArgs);
                    EPFireAndForgetQueryResult result = runtime.getFireAndForgetService()
                            .executeQuery(query);
                    String[] fields = stringArray(step.get("fields"), "fields");
                    JsonArray rows = new JsonArray();
                    for (EventBean row : result.getArray()) {
                        rows.add(projectedRow(row, fields));
                    }
                    int sequence = sequences.merge("faf", 1, Integer::sum);
                    JsonObject record = new JsonObject();
                    record.add("case", caseName);
                    record.add("operation", "faf");
                    record.add("statement", string(step, "statement"));
                    record.add("sequence", sequence);
                    record.add("time", Instant.ofEpochMilli(
                            runtime.getEventService().getCurrentTime()).toString());
                    if (rows.size() > 0) {
                        record.add("new", rows);
                    }
                    records.add(record);
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statements.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + operation);
            }
        }
        runtime.getDeploymentService().undeployAll();
        statements.clear();
    }

    private static boolean isInsertStream(String caseName) {
        return caseName.equals("insertstream-nw") || caseName.equals("insertstream-table");
    }

    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * This mirrors assertPropsNew/assertListenerNotInvoked on s1-s4: the
     * side-stream consumers receive each routed merge-insert row as new
     * data.
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
            JsonArray newRows = rows(newEvents);
            JsonArray oldRows = rows(oldEvents);
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

    /** Row projected to exactly the fields the step's assertions read,
     * with property names sorted alphabetically so the canonical ordering
     * matches the Go runner's fields-JSON sort. */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        String[] names = fields.clone();
        Arrays.sort(names);
        JsonObject values = new JsonObject();
        for (String field : names) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    /**
     * Canonical row ordering for "any"-mode snapshots, mirroring the Go
     * runner's sortRowsCanonical freeze of Java's assertEqualsAnyOrder: rows
     * sort by their fields JSON (keys already sorted alphabetically).
     */
    private static void sortRowsCanonical(JsonArray rows) {
        List<JsonValue> items = new ArrayList<>();
        for (JsonValue row : rows) {
            items.add(row);
        }
        items.sort(Comparator.comparing(row -> row.asObject().get("fields").toString()));
        while (rows.size() > 0) {
            rows.remove(0);
        }
        for (JsonValue item : items) {
            rows.add(item);
        }
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, and null as the tagged
     * {"state":"null"} object.
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
        return Json.value(String.valueOf(value));
    }

    private static void sendEvent(EPRuntime runtime, String caseName, String type,
                                  JsonValue payload) {
        switch (type) {
            case "SupportBean": {
                JsonObject object = object(payload, "SupportBean payload");
                SupportBean bean = new SupportBean();
                bean.setTheString(string(object, "theString"));
                bean.setIntPrimitive((int) longInteger(object.get("intPrimitive"),
                        "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                return;
            }
            case "SupportBean_ST0": {
                JsonObject object = object(payload, "SupportBean_ST0 payload");
                runtime.getEventService().sendEventBean(
                        new SupportBean_ST0(string(object, "id"), string(object, "key0"),
                                (int) longInteger(object.get("p00"), "p00")), type);
                return;
            }
            case "MyEvent": {
                if (caseName.endsWith("-oa")) {
                    // Positional objectarray payload, mirroring
                    // makeNestedOAEvent: Object[]{cf, Object[]{ca0, ca1}}.
                    runtime.getEventService().sendEventObjectArray(
                            toObjectArray(array(payload, "MyEvent payload")), type);
                } else {
                    // Map payload, mirroring makeNestedMapEvent: cf is a
                    // Composite map and ca a Composite[] (Map[]) pair.
                    JsonObject object = object(payload, "MyEvent payload");
                    Map<String, Object> event = new HashMap<>();
                    event.put("cf", toMap(object(object.get("cf"), "cf")));
                    JsonArray ca = array(object.get("ca"), "ca");
                    Map<String, Object>[] elements = new Map[ca.size()];
                    for (int index = 0; index < ca.size(); index++) {
                        elements[index] = toMap(object(ca.get(index), "ca element"));
                    }
                    event.put("ca", elements);
                    runtime.getEventService().sendEventMap(event, type);
                }
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    private static Map<String, Object> toMap(JsonObject object) {
        Map<String, Object> map = new HashMap<>();
        for (Member member : object) {
            map.put(member.getName(), toJavaValue(member.getValue()));
        }
        return map;
    }

    private static Object[] toObjectArray(JsonArray array) {
        Object[] values = new Object[array.size()];
        for (int index = 0; index < array.size(); index++) {
            values[index] = toJavaValue(array.get(index));
        }
        return values;
    }

    private static Object toJavaValue(JsonValue value) {
        if (value == null || value.isNull()) {
            return null;
        }
        if (value.isObject()) {
            return toMap(value.asObject());
        }
        if (value.isArray()) {
            return toObjectArray(value.asArray());
        }
        if (value instanceof JsonNumber) {
            long number = longInteger(value, "payload number");
            if (number >= Integer.MIN_VALUE && number <= Integer.MAX_VALUE) {
                return (int) number;
            }
            return number;
        }
        if (value.isBoolean()) {
            return value.asBoolean();
        }
        return value.asString();
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

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
        offset = validateNestedCase(steps, offset, "nested-nw-map", CASE_EPLS[0], false);
        offset = validateNestedCase(steps, offset, "nested-nw-oa", CASE_EPLS[1], true);
        offset = validateNestedCase(steps, offset, "nested-table-map", CASE_EPLS[2], false);
        offset = validateNestedCase(steps, offset, "nested-table-oa", CASE_EPLS[3], true);
        offset = validateInsertStreamCase(steps, offset, "insertstream-nw", CASE_EPLS[4]);
        offset = validateInsertStreamCase(steps, offset, "insertstream-table", CASE_EPLS[5]);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact step sequence of runUpdateNestedEvent (lines 1334-1362): one
     * six-statement module deploy, the SupportBean seed send, the MyEvent
     * send (map object or objectarray positional payload), the FAF select
     * assertion, and undeployAll; milestoneInc is an HA checkpoint no-op
     * that emits no steps.
     */
    private static int validateNestedCase(JsonArray steps, int offset, String caseName,
                                          String moduleEpl, boolean objectArray) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateModuleDeploy(steps.get(offset++), caseName, moduleEpl);
        for (String label : NESTED_LABELS) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateBeanSend(steps.get(offset++), caseName);
        validateMyEventSend(steps.get(offset++), caseName, objectArray);
        validateFaf(steps.get(offset++), caseName);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact step sequence of InfraOnMergeInsertStream (lines 1047-1078):
     * one seven-statement module deploy, the two not-matched
     * SupportBean_ST0 sends, the ordered 'Create' iterator read, and
     * undeployAll; milestone(0) is an HA checkpoint no-op that emits no
     * steps.
     */
    private static int validateInsertStreamCase(JsonArray steps, int offset, String caseName,
                                                String moduleEpl) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateModuleDeploy(steps.get(offset++), caseName, moduleEpl);
        for (String label : INSERTSTREAM_LABELS) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateST0Send(steps.get(offset++), caseName, "ID1", "K1", 1);
        validateST0Send(steps.get(offset++), caseName, "ID1", "K2", 2);
        validateSnapshot(steps.get(offset++), caseName, "create", "ordered", CREATE_FIELDS);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    private static void validateModuleDeploy(JsonValue value, String caseName,
                                             String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"module".equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("module deploy step is not pinned for "
                    + caseName);
        }
    }

    private static void validateDeployed(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "deployed step");
        requireFields(step, "op", "case", "statement");
        if (!"deployed".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("deployed step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode, String[] expectedFields) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), expectedFields,
                "snapshot fields for " + caseName);
    }

    private static void validateFaf(JsonValue value, String caseName) {
        JsonObject step = object(value, "faf step");
        requireFields(step, "op", "case", "statement", "epl", "fields");
        if (!"faf".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"faf".equals(string(step, "statement"))
                || !EPL_FAF.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("faf step is not pinned for " + caseName);
        }
        validateStringArray(step.get("fields"), FAF_FIELDS, "faf fields for " + caseName);
    }

    private static void validateBeanSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        if (!"E1".equals(string(payload, "theString"))
                || longInteger(payload.get("intPrimitive"), "intPrimitive") != 1) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateMyEventSend(JsonValue value, String caseName,
                                            boolean objectArray) {
        JsonObject step = object(value, "MyEvent step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("MyEvent step is not pinned for " + caseName);
        }
        JsonValue payload = step.get("payload");
        if (objectArray) {
            // Positional [[1],[[1],[2]]]: cf={c0:1}, ca={{c0:1},{c0:2}}.
            JsonArray items = array(payload, "MyEvent payload");
            if (items.size() != 2) {
                throw new IllegalArgumentException("MyEvent payload is not pinned for " + caseName);
            }
            validateCompositeArray(items.get(0), 1, caseName);
            JsonArray ca = array(items.get(1), "MyEvent ca");
            if (ca.size() != 2) {
                throw new IllegalArgumentException("MyEvent ca is not pinned for " + caseName);
            }
            validateCompositeArray(ca.get(0), 1, caseName);
            validateCompositeArray(ca.get(1), 2, caseName);
            return;
        }
        JsonObject object = object(payload, "MyEvent payload");
        requireFields(object, "cf", "ca");
        validateCompositeMap(object.get("cf"), 1, caseName);
        JsonArray ca = array(object.get("ca"), "MyEvent ca");
        if (ca.size() != 2) {
            throw new IllegalArgumentException("MyEvent ca is not pinned for " + caseName);
        }
        validateCompositeMap(ca.get(0), 1, caseName);
        validateCompositeMap(ca.get(1), 2, caseName);
    }

    private static void validateCompositeMap(JsonValue value, long c0, String caseName) {
        JsonObject composite = object(value, "Composite payload");
        requireFields(composite, "c0");
        if (longInteger(composite.get("c0"), "c0") != c0) {
            throw new IllegalArgumentException("Composite payload is not pinned for " + caseName);
        }
    }

    private static void validateCompositeArray(JsonValue value, long c0, String caseName) {
        JsonArray composite = array(value, "Composite payload");
        if (composite.size() != 1
                || longInteger(composite.get(0), "c0") != c0) {
            throw new IllegalArgumentException("Composite payload is not pinned for " + caseName);
        }
    }

    private static void validateST0Send(JsonValue value, String caseName, String expectedId,
                                        String expectedKey0, long expectedP00) {
        JsonObject step = object(value, "SupportBean_ST0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_ST0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException(
                    "SupportBean_ST0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_ST0 payload");
        requireFields(payload, "id", "key0", "p00");
        if (!expectedId.equals(string(payload, "id"))
                || !expectedKey0.equals(string(payload, "key0"))
                || longInteger(payload.get("p00"), "p00") != expectedP00) {
            throw new IllegalArgumentException(
                    "SupportBean_ST0 payload is not pinned for " + caseName);
        }
    }

    private static void validateUndeployAll(JsonValue value, String caseName) {
        JsonObject step = object(value, "undeploy-all step");
        requireFields(step, "op", "case");
        if (!"undeploy-all".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))) {
            throw new IllegalArgumentException("undeploy-all step is not pinned for " + caseName);
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

    private static String[] stringArray(JsonValue value, String label) {
        JsonArray items = array(value, label);
        String[] names = new String[items.size()];
        for (int index = 0; index < items.size(); index++) {
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException(label + " must be a string array");
            }
            names[index] = item.asString();
        }
        return names;
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

    /**
     * Local mirror of the regression SupportBean_ST0 (regression-lib is not
     * on the oracle classpath): id/key0/p00 plus the nullable p01Long and
     * pcommon properties so the StreamOne wildcard row carries the full
     * five-property shape.
     */
    public static class SupportBean_ST0 {
        private final String id;
        private final String key0;
        private final int p00;
        private final Long p01Long;
        private final String pcommon;

        public SupportBean_ST0(String id, String key0, int p00) {
            this.id = id;
            this.key0 = key0;
            this.p00 = p00;
            this.p01Long = null;
            this.pcommon = null;
        }

        public String getId() {
            return id;
        }

        public String getKey0() {
            return key0;
        }

        public int getP00() {
            return p00;
        }

        public Long getP01Long() {
            return p01Long;
        }

        public String getPcommon() {
            return pcommon;
        }
    }
}
