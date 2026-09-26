import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeAvro;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithLocalDateTime;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithZonedDateTime;
import com.espertech.esper.regressionlib.suite.event.avro.EventAvroHook;
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
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

import static org.apache.avro.SchemaBuilder.record;
import static org.apache.avro.SchemaBuilder.unionOf;

/**
 * Direct Esper 9.0.0 oracle for the four EventAvroHook executions, replayed
 * as one differential chain under the TestSuiteEventAvroWConfig session
 * configuration: enableAvro, the four bean event types, the
 * MyTypeRepresentationMapper/MyObjectValueTypeWidenerFactory hook classes
 * and the three preconfigured Avro types MyEventPopulate/MyEvent/
 * MyEventWSchema.
 *
 * property-coerce (ord 0, EventAvroHookSimpleWriteablePropertyCoerce): the
 * zdt probe compiles the pinned EPL in-process and asserts the
 * tryInvalidCompile text (assertMessage is a startsWith check) before the
 * "compile-error" record carries the pinned prefix; the valid ldt
 * insert-into deploys, sends the scenario-pinned LocalDateTime and the
 * listener asserts isodate equals ISO_DATE_TIME.format(ldt) — the pinned
 * send text round-trips byte-identically.
 *
 * schema-from-class (ord 1, EventAvroHookSchemaFromClass): the avro
 * representation insert-into deploys, the deployment's MyEventOut schema
 * text is asserted byte-exact and recorded as value/avro-schema, then a
 * SupportBean send drives the wall-clock makeLocalDateTime() UDF; the
 * listener asserts isodate length > 10 and records the fixed "<isodate>"
 * marker because now() is never a replayable value.
 *
 * populate (ord 2, EventAvroHookPopulate, STATICHOOK): a SupportBean_S0
 * send drives makeSupportBean() through the static-field widener schema;
 * the listener captures the event and value/avroToJson asserts the
 * byte-exact flat encoding.
 *
 * named-window-property-assignment (ord 3, STATICHOOK): a keepall window
 * over the union-field MyEventWSchema, an empty record seeds sb=null, the
 * SupportBean trigger assigns sb through the widener, and the iterator row
 * asserts the double-nested union encoding
 * {"sb":{"SupportBeanSchema":{...}}} — the union branch wrapper is the
 * pinned observable and is never flattened. Snapshot and listener rows
 * render in the same avroToJson shape: union record fields keep the
 * branch-name wrapper, required record fields flatten, and null fields
 * render {"state":"null"}.
 */
public final class EventAvroHook548ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "event-avro-hook-548";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/avro/EventAvroHook.java";

    private static final String DESCRIPTION =
            "EventAvroHook hook slice (all 4 executions, ord 2-3 STATICHOOK): property-coerce pins the invalid zdt->isodate insert-into compile probe and the valid ldt insert-into asserting the captured ISO_DATE_TIME string; schema-from-class pins the MyEventOut avro schema JSON byte-exact plus a shape-only now() listener row; populate inserts a SupportBeanSchema record into MyEventPopulate(sb) pinning the flat avroToJson; named-window-property-assignment updates a keepall window's union sb through a SupportBean trigger pinning the double-nested SupportBeanSchema union encoding. The TypeRepresentationMapper/ObjectValueTypeWidenerFactory codegen-hook mechanism is intentionally-different (explicit Func1 UDFs + predeclared schemas).";

    private static final String[] CASES = {
            "property-coerce", "schema-from-class", "populate", "named-window-property-assignment"};
    private static final int[] ORDINALS = {0, 1, 2, 3};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-34ec4dc8abe4f2443930",
            "java-runtime-c80f13d8b1a53d4ff458",
            "java-runtime-20190c427227521c092b",
            "java-runtime-85c9c44d4baae883febe"
    };
    private static final String[] EXECUTIONS = {
            "EventAvroHookSimpleWriteablePropertyCoerce",
            "EventAvroHookSchemaFromClass",
            "EventAvroHookPopulate",
            "EventAvroHookNamedWindowPropertyAssignment"
    };
    private static final String[] STATIC_IDS = {
            "java-281ee8b5adc379b62b2c",
            "java-281ee8b5adc379b62b2c",
            "java-281ee8b5adc379b62b2c",
            "java-281ee8b5adc379b62b2c"
    };
    private static final String[] JAVA_FLAGS = {"STATICHOOK"};
    private static final String[] OBSERVATIONS = {
            "compile-error+deployed+listener",
            "deployed+value+listener",
            "deployed+listener+value",
            "deployed+snapshot+value"
    };
    private static final String[][] CASE_FLAGS = {
            {}, {}, {"STATICHOOK"}, {"STATICHOOK"}
    };

    // Verbatim EPLs pinned from the Java regression source, including the
    // missing space after @EventRepresentation('avro').
    private static final String EPL_COERCE_INVALID =
            "insert into MyEvent(isodate) select zdt from SupportEventWithZonedDateTime";
    private static final String EPL_COERCE =
            "@name('s0') insert into MyEvent(isodate) select ldt from SupportEventWithLocalDateTime";
    private static final String EPL_SCHEMA =
            "@name('s0') @public @EventRepresentation('avro')insert into MyEventOut select com.espertech.esper.regressionlib.suite.event.avro.EventAvroHook.makeLocalDateTime() as isodate from SupportBean as e1";
    private static final String EPL_POPULATE =
            "@name('s0') insert into MyEventPopulate(sb) select com.espertech.esper.regressionlib.suite.event.avro.EventAvroHook.makeSupportBean() from SupportBean_S0 as e1";
    private static final String EPL_WINDOW_CREATE =
            "@Name('NamedWindow') @public create window MyWindow#keepall as MyEventWSchema";
    private static final String EPL_WINDOW_INSERT =
            "insert into MyWindow select * from MyEventWSchema";
    private static final String EPL_WINDOW_UPDATE =
            "on SupportBean thebean update MyWindow set sb = thebean";

    private static final String[][] DEPLOY_EPLS = {
            {EPL_COERCE},
            {EPL_SCHEMA},
            {EPL_POPULATE},
            {EPL_WINDOW_CREATE, EPL_WINDOW_INSERT, EPL_WINDOW_UPDATE}
    };
    private static final String[][] DEPLOY_STATEMENTS = {
            {"s0"},
            {"s0"},
            {"s0"},
            {"NamedWindow", "insert-window", "update-window"}
    };
    private static final String[] CASE_EPLS = {
            EPL_COERCE,
            EPL_SCHEMA,
            EPL_POPULATE,
            EPL_WINDOW_CREATE + "\n" + EPL_WINDOW_INSERT + "\n" + EPL_WINDOW_UPDATE
    };

    private static final String PROBE_MESSAGE =
            "Invalid assignment of column 'isodate' of type 'java.time.ZonedDateTime' to event property 'isodate' typed as 'java.lang.CharSequence', column and parameter types mismatch";
    private static final String MY_EVENT_OUT_SCHEMA_JSON =
            "{\"type\":\"record\",\"name\":\"MyEventOut\",\"fields\":[{\"name\":\"isodate\",\"type\":\"string\"}]}";
    private static final String POPULATE_JSON =
            "{\"sb\":{\"theString\":\"E1\",\"intPrimitive\":10}}";
    private static final String WINDOW_JSON =
            "{\"sb\":{\"SupportBeanSchema\":{\"theString\":\"E1\",\"intPrimitive\":10}}}";
    private static final String ISODATE_MARKER = "<isodate>";

    private static final String NOW = Instant.ofEpochMilli(0).toString();
    private static final int EXPECTED_STEPS = 30;
    private static final int EXPECTED_RECORDS = 14;

    private EventAvroHook548ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EventAvroHook548ScenarioOracle <scenario.json>");
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
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(steps, CASES[caseIndex], caseIndex, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS
                    + " records, got " + records.size());
        }

        System.out.println(new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", JAVA_COMMIT).add("java", System.getProperty("java.version"))
                .add("records", records));
    }

    /**
     * One runtime per case, mirroring the regression runner's clean session
     * per execution; steps without a matching case marker are skipped.
     */
    private static void runCase(JsonArray steps, String caseName,
                                int caseIndex, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        registerEventTypes(configuration);

        String runtimeURI = SCENARIO_ID + "-" + caseName;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        runtime.getEventService().advanceTime(0);
        try {
            State state = new State(records, caseName);
            // Statements keyed by the scenario deploy-step label: the window
            // insert/update EPLs are unnamed (Esper assigns a generated
            // name), so the label is the only stable pin across replays.
            Map<String, EPStatement> statements = new HashMap<>();
            List<EPCompiled> deployedModules = new ArrayList<>();
            boolean active = false;
            int deployIndex = 0;
            for (JsonValue stepValue : steps) {
                JsonObject step = stepValue.asObject();
                String operation = step.getString("op", "");
                if ("case".equals(operation)) {
                    active = caseName.equals(step.getString("case", ""));
                    continue;
                }
                if (!active) {
                    continue;
                }
                switch (operation) {
                    case "deploy" -> {
                        if (deployIndex >= DEPLOY_EPLS[caseIndex].length) {
                            throw new IllegalStateException("unexpected deploy index "
                                    + deployIndex + " for case " + caseName);
                        }
                        String epl = DEPLOY_EPLS[caseIndex][deployIndex];
                        String label = DEPLOY_STATEMENTS[caseIndex][deployIndex];
                        if (!epl.equals(step.getString("epl", ""))
                                || !label.equals(step.getString("statement", ""))) {
                            throw new IllegalArgumentException("deploy step is not pinned for case "
                                    + caseName + " deploy " + deployIndex);
                        }
                        deployIndex++;
                        // RegressionPath: earlier compiled modules join the
                        // path so the @public MyWindow resolves.
                        CompilerArguments compilerArgs = new CompilerArguments(configuration);
                        for (EPCompiled deployed : deployedModules) {
                            compilerArgs.getPath().add(deployed);
                        }
                        EPCompiled compiled =
                                EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                        EPDeployment deployment = runtime.getDeploymentService().deploy(compiled);
                        deployedModules.add(compiled);
                        if (deployment.getStatements().length != 1) {
                            throw new IllegalStateException("deploy " + label + " produced "
                                    + deployment.getStatements().length + " statements");
                        }
                        EPStatement statement = deployment.getStatements()[0];
                        statements.put(label, statement);
                        if ("s0".equals(statement.getName())) {
                            statement.addListener(state.writer);
                        }
                    }
                    case "deployed" -> {
                        String label = step.getString("statement", "");
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException("deployed step references "
                                    + label + " before deploy");
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "deployed");
                        record.add("statement", label);
                        record.add("sequence",
                                state.sequences.merge(label + ":deployed", 1, Integer::sum));
                        record.add("time", NOW);
                        records.add(record);
                    }
                    case "build-error" -> buildError(configuration, caseName, step, records);
                    case "send" -> sendEvent(runtime, state, step);
                    case "value" -> emitValue(runtime, state, statements, step);
                    case "snapshot" -> emitSnapshot(state, statements, step);
                    case "undeploy-all" -> {
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deployedModules.clear();
                    }
                    default -> throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            runtime.getDeploymentService().undeployAll();
            runtime.destroy();
        }
    }

    /**
     * Registers the WConfig session types: enableAvro, the four bean event
     * types, the two hook classes, and the three preconfigured Avro types
     * (MyEventPopulate {sb:SupportBeanSchema}, MyEvent {isodate:string},
     * MyEventWSchema {sb:union(null,SupportBeanSchema)}).
     */
    private static void registerEventTypes(Configuration configuration) {
        configuration.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        for (Class<?> clazz : new Class[]{SupportBean.class, SupportBean_S0.class,
                SupportEventWithLocalDateTime.class, SupportEventWithZonedDateTime.class}) {
            configuration.getCommon().addEventType(clazz);
        }
        configuration.getCommon().getEventMeta().getAvroSettings()
                .setTypeRepresentationMapperClass(EventAvroHook.MyTypeRepresentationMapper.class.getName());
        configuration.getCommon().getEventMeta().getAvroSettings()
                .setObjectValueTypeWidenerFactoryClass(EventAvroHook.MyObjectValueTypeWidenerFactory.class.getName());

        EventAvroHook.MySupportBeanWidener.supportBeanSchema =
                record("SupportBeanSchema").fields()
                        .requiredString("theString").requiredInt("intPrimitive").endRecord();
        Schema schemaMyEventPopulate = record("MyEventSchema").fields()
                .name("sb").type(EventAvroHook.MySupportBeanWidener.supportBeanSchema).noDefault()
                .endRecord();
        configuration.getCommon().addEventTypeAvro("MyEventPopulate",
                new ConfigurationCommonEventTypeAvro(schemaMyEventPopulate));

        Schema schemaMyEventSchema = record("MyEventSchema").fields()
                .requiredString("isodate").endRecord();
        configuration.getCommon().addEventTypeAvro("MyEvent",
                new ConfigurationCommonEventTypeAvro(schemaMyEventSchema));

        Schema schemaMyEventWSchema = record("MyEventSchema").fields()
                .name("sb").type(unionOf().nullType()
                        .and().type(EventAvroHook.MySupportBeanWidener.supportBeanSchema).endUnion())
                .noDefault().endRecord();
        configuration.getCommon().addEventTypeAvro("MyEventWSchema",
                new ConfigurationCommonEventTypeAvro(schemaMyEventWSchema));
    }

    /**
     * Mirrors env.tryInvalidCompile: the compile must throw and the message
     * must start with the pinned assertMessage text; the record carries
     * the pinned prefix (no time, mirroring the compile-error convention).
     */
    private static void buildError(Configuration configuration, String caseName,
                                   JsonObject step, JsonArray records) {
        String epl = step.getString("epl", "");
        String expected = step.getString("expectError", "");
        if (!EPL_COERCE_INVALID.equals(epl) || !PROBE_MESSAGE.equals(expected)
                || !"property-coerce".equals(caseName)) {
            throw new IllegalStateException("build-error step is not pinned");
        }
        String caught;
        try {
            EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(configuration));
            throw new IllegalStateException("build-error probe unexpectedly compiled");
        } catch (IllegalStateException propagate) {
            throw propagate;
        } catch (Exception ex) {
            caught = ex.getMessage() == null ? ex.getClass().getName() : ex.getMessage();
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("build-error message drift: expected prefix ["
                    + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", step.getString("statement", ""));
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    private static void sendEvent(EPRuntime runtime, State state, JsonObject step) {
        String eventType = step.getString("eventType", "");
        JsonObject payload = step.get("payload") != null && step.get("payload").isObject()
                ? step.get("payload").asObject() : new JsonObject();
        switch (eventType) {
            case "SupportEventWithLocalDateTime" -> {
                // The scenario pins LocalDateTime.now() captured at scenario
                // build; ISO_DATE_TIME.format round-trips it byte-exactly.
                String text = payload.getString("ldt", "");
                state.expectedIsodate = text;
                runtime.getEventService().sendEventBean(
                        new SupportEventWithLocalDateTime(LocalDateTime.parse(text)), eventType);
            }
            case "SupportBean" -> runtime.getEventService().sendEventBean(new SupportBean(
                    payload.getString("theString", ""), payload.getInt("intPrimitive", 0)), eventType);
            case "SupportBean_S0" -> runtime.getEventService().sendEventBean(
                    new SupportBean_S0(payload.getInt("id", 0)), eventType);
            case "MyEventWSchema" -> {
                if (payload.size() != 0) {
                    throw new IllegalStateException(
                            "MyEventWSchema payload must be the empty record");
                }
                Schema schema = SupportAvroUtil.getAvroSchema(
                        runtime.getEventTypeService().getEventTypePreconfigured("MyEventWSchema"));
                runtime.getEventService().sendEventAvro(new GenericData.Record(schema), eventType);
            }
            default -> throw new IllegalArgumentException("unsupported event type: " + eventType);
        }
    }

    /**
     * Runs the pinned in-process assertion for one value step and emits the
     * record: avro-schema asserts the deployment's MyEventOut schema text
     * byte-exact; avroToJson asserts the last captured populate event or
     * the window row byte-exact via SupportAvroUtil.avroToJson.
     */
    private static void emitValue(EPRuntime runtime, State state,
                                  Map<String, EPStatement> statements, JsonObject step) {
        String statement = step.getString("statement", "");
        String name = step.getString("name", "");
        String value;
        if ("avro-schema".equals(name)) {
            if (!"schema-from-class".equals(state.caseName)) {
                throw new IllegalStateException("avro-schema step outside schema-from-class");
            }
            EPStatement s0 = statements.get("s0");
            if (s0 == null) {
                throw new IllegalStateException("s0 statement not deployed");
            }
            Schema schema = SupportAvroUtil.getAvroSchema(
                    runtime.getEventTypeService().getEventType(s0.getDeploymentId(), "MyEventOut"));
            value = schema.toString();
            if (!MY_EVENT_OUT_SCHEMA_JSON.equals(value)) {
                throw new IllegalStateException("MyEventOut schema drift: " + value);
            }
        } else if ("avroToJson".equals(name)) {
            EventBean event;
            if ("populate".equals(state.caseName)) {
                event = state.lastEvent;
                if (event == null) {
                    throw new IllegalStateException("avroToJson step has no captured event");
                }
            } else if ("named-window-property-assignment".equals(state.caseName)) {
                event = singleWindowRow(statements);
            } else {
                throw new IllegalStateException("avroToJson step outside pinned cases");
            }
            value = SupportAvroUtil.avroToJson(event);
            String pinned = "populate".equals(state.caseName) ? POPULATE_JSON : WINDOW_JSON;
            if (!pinned.equals(value)) {
                throw new IllegalStateException("avroToJson drift: expected ["
                        + pinned + "] got [" + value + "]");
            }
        } else {
            throw new IllegalStateException("unsupported value step " + name);
        }
        JsonObject record = new JsonObject();
        record.add("case", state.caseName);
        record.add("operation", "value");
        record.add("statement", statement);
        record.add("sequence", state.sequences.merge(statement + ":value", 1, Integer::sum));
        record.add("time", NOW);
        record.add("name", name);
        record.add("value", value);
        state.records.add(record);
    }

    private static EventBean singleWindowRow(Map<String, EPStatement> statements) {
        EPStatement window = statements.get("NamedWindow");
        if (window == null) {
            throw new IllegalStateException("NamedWindow statement not deployed");
        }
        Iterator<EventBean> iterator = window.iterator();
        if (!iterator.hasNext()) {
            throw new IllegalStateException("window holds no row");
        }
        EventBean event = iterator.next();
        if (iterator.hasNext()) {
            throw new IllegalStateException("window holds more than one row");
        }
        return event;
    }

    /**
     * Mirrors env.iterator("NamedWindow"): the window row renders in the
     * avroToJson shape (union branch kept, null as {"state":"null"}).
     */
    private static void emitSnapshot(State state, Map<String, EPStatement> statements,
                                     JsonObject step) {
        String statement = step.getString("statement", "");
        if (!"named-window-property-assignment".equals(state.caseName)) {
            throw new IllegalStateException("snapshot step outside named-window case");
        }
        EPStatement window = statements.get("NamedWindow");
        if (window == null) {
            throw new IllegalStateException("NamedWindow statement not deployed");
        }
        JsonArray rows = new JsonArray();
        for (Iterator<EventBean> iterator = window.iterator(); iterator.hasNext(); ) {
            rows.add(avroEventRow(iterator.next()));
        }
        JsonObject record = new JsonObject();
        record.add("case", state.caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement);
        record.add("sequence", 0);
        record.add("time", NOW);
        record.add("new", rows);
        state.records.add(record);
    }

    /**
     * Renders an Avro event in the avroToJson shape: declared fields in
     * order, a union record field wrapped under the branch record name,
     * required record fields flattened, null as {"state":"null"}.
     */
    private static JsonObject avroEventRow(EventBean event) {
        Schema schema = SupportAvroUtil.getAvroSchema(event);
        GenericData.Record record = (GenericData.Record) event.getUnderlying();
        JsonObject fields = new JsonObject();
        for (Schema.Field field : schema.getFields()) {
            fields.add(field.name(), avroFieldValue(field.schema(), record.get(field.name())));
        }
        return new JsonObject().add("kind", "row").add("fields", fields);
    }

    private static JsonValue avroFieldValue(Schema fieldSchema, Object value) {
        if (value == null) {
            return new JsonObject().add("state", "null");
        }
        if (value instanceof GenericData.Record record) {
            if (fieldSchema.getType() == Schema.Type.UNION) {
                for (Schema branch : fieldSchema.getTypes()) {
                    if (branch.getType() == Schema.Type.RECORD
                            && branch.getName().equals(record.getSchema().getName())) {
                        return new JsonObject().add(branch.getName(), avroRecordJson(record));
                    }
                }
                throw new IllegalStateException("union branch not resolved for " + fieldSchema);
            }
            return avroRecordJson(record);
        }
        return avroScalarJson(value);
    }

    private static JsonObject avroRecordJson(GenericData.Record record) {
        JsonObject fields = new JsonObject();
        for (Schema.Field field : record.getSchema().getFields()) {
            fields.add(field.name(), avroFieldValue(field.schema(), record.get(field.name())));
        }
        return fields;
    }

    private static JsonValue avroScalarJson(Object value) {
        if (value instanceof CharSequence) {
            return Json.value(value.toString());
        }
        if (value instanceof Double || value instanceof Float) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Boolean flag) {
            return Json.value(flag);
        }
        return Json.value(String.valueOf(value));
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !SCENARIO_ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTIONS, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < CASES.length; index++) {
            JsonObject definition = object(cases.get(index), "case " + index);
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl", "flags");
            if (!CASES[index].equals(string(definition, "case"))
                    || ORDINALS[index] != integer(definition, "ordinal")
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTIONS[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !CASE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case " + index + " identity is not pinned");
            }
            validateStringArray(definition.get("flags"), CASE_FLAGS[index],
                    "case " + index + " flags");
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + EXPECTED_STEPS + " steps");
        }
        // Per-step pinning happens at execution: each step's op/statement/
        // epl/payload is asserted against the pinned table as it runs.
    }

    private static final class State {
        final JsonArray records;
        final String caseName;
        final Map<String, Integer> sequences = new HashMap<>();
        final TraceWriter writer;
        String expectedIsodate;
        EventBean lastEvent;

        State(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
            this.writer = new TraceWriter(this);
        }
    }

    /**
     * Automatic s0 listener mirroring env.addListener("s0"): every update
     * emits one listener record whose sequence increments per case. Rows
     * render in the avroToJson shape; ord-0 asserts isodate equals the
     * pinned send text, ord-1 asserts the wall-clock length then records
     * the fixed marker, ord-2 captures the event for avroToJson.
     */
    private static final class TraceWriter implements UpdateListener {
        private final State state;
        private long sequence;

        private TraceWriter(State state) {
            this.state = state;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement statement,
                           EPRuntime ignoredRuntime) {
            if (newEvents == null && oldEvents == null) {
                return;
            }
            JsonObject record = new JsonObject()
                    .add("case", state.caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", NOW);
            JsonArray newRows = rows(newEvents);
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", rows(oldEvents));
            }
            state.records.add(record);
        }

        private JsonArray rows(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                output.add(row(event));
            }
            return output;
        }

        private JsonObject row(EventBean event) {
            state.lastEvent = event;
            JsonObject fields = new JsonObject();
            for (String prop : event.getEventType().getPropertyNames()) {
                fields.add(prop, listenerValue(prop, event.get(prop)));
            }
            return new JsonObject().add("kind", "row").add("fields", fields);
        }

        private JsonValue listenerValue(String prop, Object value) {
            if ("isodate".equals(prop) && value instanceof String text) {
                if ("property-coerce".equals(state.caseName)) {
                    if (!text.equals(state.expectedIsodate)) {
                        throw new IllegalStateException("isodate " + text
                                + " != " + state.expectedIsodate);
                    }
                    return Json.value(text);
                }
                if ("schema-from-class".equals(state.caseName)) {
                    if (text.length() <= 10) {
                        throw new IllegalStateException("isodate length <= 10: " + text);
                    }
                    return Json.value(ISODATE_MARKER);
                }
            }
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof GenericData.Record record) {
                return avroRecordJson(record);
            }
            return avroScalarJson(value);
        }
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            JsonObject object = value.asObject();
            java.util.Set<String> names = new HashSet<>();
            for (Member member : object) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON key: " + member.getName());
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
}
