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
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import org.apache.avro.Schema;
import org.apache.avro.generic.GenericData;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNWTableOnMerge ordinals 8-19: the
 * InfraInsertOtherStream event-representation matrix.  Each of the six
 * EventRepresentationChoice values (OBJECTARRAY, MAP, AVRO, JSON,
 * JSONCLASSPROVIDED, DEFAULT) runs twice — namedWindow=true first, then
 * namedWindow=false — giving twelve executions on twelve runtimes.
 *
 * Every execution deploys one six-statement module: the annotated MyEvent
 * schema, the MyInfraIOS unique-key named window or composite-primary-key
 * table, the MyEvent insert-into feeder, the dead InputEvent schema, the
 * on-merge that routes event_name plus the matched target value (0d when
 * not matched) into OtherStreamOne, and the s0 consumer.  The only
 * observable is the s0 listener: for named windows the on-merge trigger
 * fires before the insert-into route, so the first event is not matched
 * (status=0d) and each later same-key event matches the previous value
 * (10d then 11d); for tables insert-into routes the row before the
 * trigger evaluates, so the first event is matched (status=10d).  The
 * merge never mutates the target.
 *
 * Send payloads follow makeSendNameValueEvent: a positional object array
 * for OBJECTARRAY, a name/value map for MAP and DEFAULT, a
 * GenericData.Record over the preconfigured runtime Avro schema for AVRO,
 * and a JSON object for JSON and JSONCLASSPROVIDED.  The JSONCLASSPROVIDED
 * module references the regression-lib MyLocalJsonProvided* classes in its
 * @JsonSchema annotations, so the run script puts regression-lib classes
 * on the classpath.
 */
public final class InfraNWTableOnMergeInsertStreamScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-on-merge-insertstream";
    private static final String DESCRIPTION =
            "InfraNWTableOnMerge ordinals 8-19: InfraInsertOtherStream event-representation "
                    + "matrix — MyEvent merges into a #unique(name) named window or a "
                    + "composite-primary-key table over OBJECTARRAY, MAP, AVRO, JSON, "
                    + "JSONCLASSPROVIDED and DEFAULT payloads; each trigger routes event_name "
                    + "plus the matched target value (0d when not matched) into OtherStreamOne "
                    + "observed by the s0 listener, so the named window's first event is not "
                    + "matched (0d, then previous-value matches 10d/11d) while the table's "
                    + "first event is matched (10d) (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/"
                    + "infra/nwtable/InfraNWTableOnMerge.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/"
                    + "InfraNWTableOnMerge.java";

    private static final String[] REPRS = {
            "objectarray", "map", "avro", "json", "jsonclassprovided", "default"
    };
    private static final String[] RUNTIME_IDS = {
            "java-runtime-c91b3df37512cfac454f",
            "java-runtime-b54b14998ff59e65f2fc",
            "java-runtime-945ba0790a1cfa2b1033",
            "java-runtime-fef4b3a46f9b7da65a72",
            "java-runtime-d8cb24fc9e29421236e2",
            "java-runtime-e22957dc02c3ac4b8bb7",
            "java-runtime-284497e8a9729bb76ac2",
            "java-runtime-f6718d2a96a3e3668239",
            "java-runtime-51773f11823a58156447",
            "java-runtime-b891f1f54423ccb8a628",
            "java-runtime-dd95faed8e31c95bec9f",
            "java-runtime-6e68e972f23cef8dde85"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=OBJECTARRAY}",
            "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=OBJECTARRAY}",
            "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=MAP}",
            "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=MAP}",
            "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=AVRO}",
            "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=AVRO}",
            "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=JSON}",
            "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=JSON}",
            "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=JSONCLASSPROVIDED}",
            "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=JSONCLASSPROVIDED}",
            "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=DEFAULT}",
            "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=DEFAULT}"
    };
    private static final String[] STATIC_IDS = {
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f",
            "java-8304a4459a4ea865bf1f"
    };
    private static final String[] CASES = {
            "insertstream-nw-objectarray", "insertstream-table-objectarray",
            "insertstream-nw-map", "insertstream-table-map",
            "insertstream-nw-avro", "insertstream-table-avro",
            "insertstream-nw-json", "insertstream-table-json",
            "insertstream-nw-jsonclassprovided", "insertstream-table-jsonclassprovided",
            "insertstream-nw-default", "insertstream-table-default"
    };
    private static final int[] ORDINALS = {8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; objectarray positional MyEvent payload merges into the #unique(name) "
                    + "named window: the on-merge trigger fires before the insert-into route, "
                    + "so the first event is not matched (status=0d) and each later same-key "
                    + "event matches the previous value (10d then 11d); merge never mutates "
                    + "the target",
            "listener; objectarray positional MyEvent payload merges into the "
                    + "composite-primary-key table: insert-into routes the row before the "
                    + "on-merge trigger evaluates, so the first event is matched (status=10d); "
                    + "merge never mutates the target",
            "listener; map MyEvent payload merges into the #unique(name) named window: the "
                    + "on-merge trigger fires before the insert-into route, so the first "
                    + "event is not matched (status=0d) and each later same-key event matches "
                    + "the previous value (10d then 11d); merge never mutates the target",
            "listener; map MyEvent payload merges into the composite-primary-key table: "
                    + "insert-into routes the row before the on-merge trigger evaluates, so "
                    + "the first event is matched (status=10d); merge never mutates the "
                    + "target",
            "listener; Avro GenericData.Record MyEvent payload over the preconfigured schema "
                    + "merges into the #unique(name) named window: the on-merge trigger fires "
                    + "before the insert-into route, so the first event is not matched "
                    + "(status=0d) and each later same-key event matches the previous value "
                    + "(10d then 11d); merge never mutates the target",
            "listener; Avro GenericData.Record MyEvent payload over the preconfigured schema "
                    + "merges into the composite-primary-key table: insert-into routes the "
                    + "row before the on-merge trigger evaluates, so the first event is "
                    + "matched (status=10d); merge never mutates the target",
            "listener; JSON object MyEvent payload merges into the #unique(name) named "
                    + "window: the on-merge trigger fires before the insert-into route, so "
                    + "the first event is not matched (status=0d) and each later same-key "
                    + "event matches the previous value (10d then 11d); merge never mutates "
                    + "the target",
            "listener; JSON object MyEvent payload merges into the composite-primary-key "
                    + "table: insert-into routes the row before the on-merge trigger "
                    + "evaluates, so the first event is matched (status=10d); merge never "
                    + "mutates the target",
            "listener; JSON object MyEvent payload over the @JsonSchema provided class merges "
                    + "into the #unique(name) named window: the on-merge trigger fires before "
                    + "the insert-into route, so the first event is not matched (status=0d) "
                    + "and each later same-key event matches the previous value (10d then "
                    + "11d); merge never mutates the target",
            "listener; JSON object MyEvent payload over the @JsonSchema provided class merges "
                    + "into the composite-primary-key table: insert-into routes the row "
                    + "before the on-merge trigger evaluates, so the first event is matched "
                    + "(status=10d); merge never mutates the target",
            "listener; default-representation map MyEvent payload merges into the "
                    + "#unique(name) named window: the on-merge trigger fires before the "
                    + "insert-into route, so the first event is not matched (status=0d) and "
                    + "each later same-key event matches the previous value (10d then 11d); "
                    + "merge never mutates the target",
            "listener; default-representation map MyEvent payload merges into the "
                    + "composite-primary-key table: insert-into routes the row before the "
                    + "on-merge trigger evaluates, so the first event is matched "
                    + "(status=10d); merge never mutates the target"
    };

    private static final String JSON_PROVIDED_BASE =
            "com.espertech.esper.regressionlib.suite.infra.nwtable."
                    + "InfraNWTableOnMerge$MyLocalJsonProvided";

    /**
     * Per-statement annotation text mirroring
     * EventRepresentationChoice.getAnnotationTextWJsonProvided: the Java
     * source concatenates annotationText + " " before each statement, so
     * DEFAULT's empty annotation leaves a leading space.
     */
    private static String ann(String repr, String provided) {
        switch (repr) {
            case "objectarray":
                return "@EventRepresentation('objectarray') ";
            case "map":
                return "@EventRepresentation('map') ";
            case "avro":
                return "@EventRepresentation('avro') ";
            case "json":
                return "@EventRepresentation('json') ";
            case "jsonclassprovided":
                return "@JsonSchema(className='" + JSON_PROVIDED_BASE + provided
                        + "') @EventRepresentation('json') ";
            default:
                return " ";
        }
    }

    /**
     * Verbatim transcription of InfraInsertOtherStream.run (lines
     * 1254-1268): the annotated MyEvent schema, the annotated window or
     * unannotated composite-primary-key table, the insert-into feeder, the
     * dead InputEvent schema, the blank line, the multi-line on-merge with
     * matched/not-matched OtherStreamOne inserts, and the s0 consumer.
     */
    private static String moduleEpl(String repr, boolean namedWindow) {
        return ann(repr, "MyEvent")
                + "@public @buseventtype @public create schema MyEvent as (name string, value double);\n"
                + (namedWindow
                ? ann(repr, "MyEvent")
                + "@public create window MyInfraIOS#unique(name) as MyEvent;\n"
                : "@public create table MyInfraIOS (name string primary key, value double primary key);\n")
                + "insert into MyInfraIOS select * from MyEvent;\n"
                + ann(repr, "InputEvent")
                + "create schema InputEvent as (col1 string, col2 double);\n"
                + "\n"
                + "on MyEvent as eme\n"
                + "  merge MyInfraIOS as MyInfraIOS where MyInfraIOS.name = eme.name\n"
                + "   when matched then\n"
                + "      insert into OtherStreamOne select eme.name as event_name, MyInfraIOS.value as status\n"
                + "   when not matched then\n"
                + "      insert into OtherStreamOne select eme.name as event_name, 0d as status;\n"
                + "@name('s0') select * from OtherStreamOne;\n";
    }

    private static final String[] CASE_EPLS = {
            moduleEpl("objectarray", true), moduleEpl("objectarray", false),
            moduleEpl("map", true), moduleEpl("map", false),
            moduleEpl("avro", true), moduleEpl("avro", false),
            moduleEpl("json", true), moduleEpl("json", false),
            moduleEpl("jsonclassprovided", true), moduleEpl("jsonclassprovided", false),
            moduleEpl("default", true), moduleEpl("default", false)
    };

    // Module statement labels in EPL order; the single compileDeploy binds
    // deployment.getStatements() positionally to these labels.
    private static final String[] LABELS = {
            "schema-myevent", "infra", "insert", "schema-input", "merge", "s0"
    };

    private static final int EXPECTED_STEPS = 132;
    private static final int EXPECTED_RECORDS = 96;

    private InfraNWTableOnMergeInsertStreamScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableOnMergeInsertStreamScenarioOracle <scenario.json>");
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
        for (String caseName : CASES) {
            runCase(caseName, allSteps, records);
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
     * its own runtime).  The single module deploy binds its statements
     * positionally to the pinned labels and attaches the listener to s0,
     * mirroring env.compileDeploy(epl, path).addListener("s0"); internal
     * timer is disabled and the rethrowing exception handler surfaces
     * statement failures to the sender thread.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        CompilerArguments compilerArgs = new CompilerArguments(runtime.getRuntimePath());
        compilerArgs.getConfiguration().getCommon().getEventMeta().getAvroSettings()
                .setEnableAvro(true);
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
                        EPCompiled compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService()
                                .deploy(compiled, new DeploymentOptions());
                        EPStatement[] deployed = deployment.getStatements();
                        if (deployed.length != LABELS.length) {
                            throw new IllegalStateException("module deployment of " + caseName
                                    + " has " + deployed.length + " statements, want "
                                    + LABELS.length);
                        }
                        for (int index = 0; index < deployed.length; index++) {
                            EPStatement statement = deployed[index];
                            if ("s0".equals(statement.getName())) {
                                statement.addListener(
                                        listener(caseName, sequences, records, runtime));
                            }
                            statements.put(LABELS[index], statement);
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
                        sendEvent(runtime, string(step, "eventType"), step.get("payload"));
                        break;
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        compilerArgs = new CompilerArguments(runtime.getRuntimePath());
                        compilerArgs.getConfiguration().getCommon().getEventMeta()
                                .getAvroSettings().setEnableAvro(true);
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
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; new and old arrays render only when non-empty.
     * This mirrors assertPropsNew on s0: the OtherStreamOne consumer
     * receives each routed merge-insert row as new data.
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

    /**
     * Sends one MyEvent trigger in the case's event representation,
     * mirroring makeSendNameValueEvent: positional object array for
     * OBJECTARRAY, name/value map for MAP and DEFAULT, GenericData.Record
     * over the preconfigured runtime Avro schema for AVRO, and a JSON
     * object for JSON and JSONCLASSPROVIDED.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonValue payload) {
        if (!"MyEvent".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        JsonObject object = object(payload, "MyEvent payload");
        String repr = string(object, "repr");
        JsonObject fields = object(object.get("fields"), "MyEvent fields");
        String name = string(fields, "name");
        double value = doubleValue(fields.get("value"), "value");
        switch (repr) {
            case "objectarray":
                runtime.getEventService().sendEventObjectArray(new Object[]{name, value}, type);
                return;
            case "map":
            case "default": {
                Map<String, Object> event = new HashMap<>();
                event.put("name", name);
                event.put("value", value);
                runtime.getEventService().sendEventMap(event, type);
                return;
            }
            case "avro": {
                // Mirrors env.runtimeAvroSchemaPreconfigured(typeName): the
                // runtime event-type service hands back the schema the
                // deployed @EventRepresentation('avro') create-schema
                // preconfigured.
                Schema schema = SupportAvroUtil.getAvroSchema(
                        runtime.getEventTypeService().getEventTypePreconfigured(type));
                GenericData.Record record = new GenericData.Record(schema);
                record.put("name", name);
                record.put("value", value);
                runtime.getEventService().sendEventAvro(record, type);
                return;
            }
            case "json":
            case "jsonclassprovided":
                runtime.getEventService().sendEventJson(
                        new JsonObject().add("name", name).add("value", value).toString(),
                        type);
                return;
            default:
                throw new IllegalArgumentException("unknown representation: " + repr);
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
        for (int index = 0; index < CASES.length; index++) {
            offset = validateCase(steps, offset, CASES[index], REPRS[index / 2], index % 2 == 0);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact step sequence of InfraInsertOtherStream.run (lines 1254-1282):
     * one six-statement module deploy, the first MyEvent send, two more
     * same-key sends for named windows only, and undeployAll.
     */
    private static int validateCase(JsonArray steps, int offset, String caseName, String repr,
                                    boolean namedWindow) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateModuleDeploy(steps.get(offset++), caseName, moduleEpl(repr, namedWindow));
        for (String label : LABELS) {
            validateDeployed(steps.get(offset++), caseName, label);
        }
        validateSend(steps.get(offset++), caseName, repr, "name1", 10);
        if (namedWindow) {
            validateSend(steps.get(offset++), caseName, repr, "name1", 11);
            validateSend(steps.get(offset++), caseName, repr, "name1", 12);
        }
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

    private static void validateSend(JsonValue value, String caseName, String repr,
                                     String expectedName, double expectedValue) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "MyEvent payload");
        requireFields(payload, "repr", "fields");
        if (!repr.equals(string(payload, "repr"))) {
            throw new IllegalArgumentException("send repr is not pinned for " + caseName);
        }
        JsonObject fields = object(payload.get("fields"), "MyEvent fields");
        requireFields(fields, "name", "value");
        if (!expectedName.equals(string(fields, "name"))
                || doubleValue(fields.get("value"), "value") != expectedValue) {
            throw new IllegalArgumentException("MyEvent payload is not pinned for " + caseName);
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

    private static double doubleValue(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON number");
        }
        try {
            return Double.parseDouble(value.toString());
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is not a JSON number", ex);
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
