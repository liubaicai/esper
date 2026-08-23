import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.avro.support.SupportAvroUtil;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;

import org.apache.avro.Schema;
import org.apache.avro.generic.GenericData;

import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for InfraNWTableFAF.Infra3StreamInnerJoin (pinned Esper 9.0.0
 * commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c, regression-lib
 * .../infra/nwtable/InfraNWTableFAF.java lines 301-395).
 *
 * Covers all twelve representation x namedWindow executions of
 * Infra3StreamInnerJoin. Each case deploys one module holding the three
 * @public @buseventtype schemas (Product, Category, ProductOwnerDetails)
 * plus either three create-window/insert pairs (namedWindow=true) or three
 * create-table/on-merge-when-not-matched-insert pairs (namedWindow=false),
 * transcribed verbatim from the pinned source; only line wrapping changed
 * and no statement-name additions were needed.
 * The module is deployed against the
 * runtime path, the five pinned events are sent with the pinned sendEvent
 * helper semantics per representation (object-array positional values, map,
 * Avro GenericData.Record over the deployed schema, JSON text), and the four
 * pinned fire-and-forget join queries q1..q4 are compiled path-aware and
 * executed via the runtime fire-and-forget service.
 *
 * Each faf step emits ONE record:
 * {case, operation:"faf", statement:"qN", new:[{kind:"row",
 * fields:{"WinProduct.productId": <string>}}]} with no old array and no
 * sequence/time fields; every query expects exactly
 * [{WinProduct.productId:"Product1"}] across all twelve cases, proving
 * representation invariance of FAF results.
 *
 * JSONCLASSPROVIDED wiring note: the pinned EPL prefixes each schema with
 * EventRepresentationChoice.JSONCLASSPROVIDED.getAnnotationTextWJsonProvided,
 * i.e. "@JsonSchema(className='<provided class>') @EventRepresentation('json')"
 * resolving local MyLocalJsonProvided* mirror classes. Those mirrors are wired
 * here as public nested classes at the bottom of this file (same shape as the
 * pinned ones), so the annotation text matches the pinned suite exactly; the
 * work-unit contract alternatively sanctioned plain json annotation text had
 * wiring been impractical. Observable FAF results are unaffected either way.
 */
public class InfraNWTableFAFJoinScenarioOracle {

    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "infra-nwtable-faf-join";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    private enum Rep {
        OBJECTARRAY, MAP, AVRO, JSON, JSONCLASSPROVIDED, DEFAULT
    }

    private static final class CaseDef {
        private final String name;
        private final Rep rep;
        private final boolean namedWindow;

        private CaseDef(String name, Rep rep, boolean namedWindow) {
            this.name = name;
            this.rep = rep;
            this.namedWindow = namedWindow;
        }
    }

    /** Inventory order = pinned registration order (rep loop, nw then table). */
    private static final CaseDef[] CASES = {
        new CaseDef("objectarray-namedwindow", Rep.OBJECTARRAY, true),
        new CaseDef("objectarray-table", Rep.OBJECTARRAY, false),
        new CaseDef("map-namedwindow", Rep.MAP, true),
        new CaseDef("map-table", Rep.MAP, false),
        new CaseDef("avro-namedwindow", Rep.AVRO, true),
        new CaseDef("avro-table", Rep.AVRO, false),
        new CaseDef("json-namedwindow", Rep.JSON, true),
        new CaseDef("json-table", Rep.JSON, false),
        new CaseDef("jsonclassprovided-namedwindow", Rep.JSONCLASSPROVIDED, true),
        new CaseDef("jsonclassprovided-table", Rep.JSONCLASSPROVIDED, false),
        new CaseDef("default-namedwindow", Rep.DEFAULT, true),
        new CaseDef("default-table", Rep.DEFAULT, false),
    };

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: InfraNWTableFAFJoinScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unexpected scenario id " + scenario.getString("id", ""));
        }
        JsonArray allSteps = scenario.get("steps").asArray();
        validateSteps(allSteps);

        List<JsonObject> records = new ArrayList<>();
        for (CaseDef caseDef : CASES) {
            runCase(allSteps, caseDef, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", scenario.getString("id", ""));
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        JsonArray recordsArr = new JsonArray();
        for (JsonObject record : records) {
            recordsArr.add(record);
        }
        root.add("records", recordsArr);
        System.out.println(root.toString());
    }

    private static void validateSteps(JsonArray allSteps) {
        int markers = 0;
        for (int i = 0; i < allSteps.size(); i++) {
            JsonObject step = allSteps.get(i).asObject();
            if (!"case".equals(step.getString("op", ""))) {
                continue;
            }
            markers++;
            String caseName = step.getString("case", "");
            if (!isCase(caseName)) {
                throw new IllegalArgumentException("unsupported case " + caseName);
            }
        }
        if (markers != CASES.length) {
            throw new IllegalArgumentException("expected exactly " + CASES.length + " cases");
        }
    }

    private static boolean isCase(String caseName) {
        for (CaseDef candidate : CASES) {
            if (candidate.name.equals(caseName)) {
                return true;
            }
        }
        return false;
    }

    private static int findCase(JsonArray steps, String caseName) {
        for (int i = 0; i < steps.size(); i++) {
            JsonObject step = steps.get(i).asObject();
            if ("case".equals(step.getString("op", "")) && caseName.equals(step.getString("case", ""))) {
                return i;
            }
        }
        throw new IllegalArgumentException("case not found " + caseName);
    }

    private static void runCase(JsonArray allSteps, CaseDef caseDef, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        if (caseDef.rep == Rep.AVRO) {
            config.getCommon().getEventMeta().getAvroSettings().setEnableAvro(true);
        }
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(SCENARIO_ID + "-" + caseDef.name, config);
        runtime.getEventService().advanceTime(0);
        List<EPCompiled> pathCompileds = new ArrayList<>();
        try {
            // Configuration carries representation/avro enablement; the
            // compileds list carries deployed infra visibility (RegressionPath
            // style) without re-adding generated JSON classes.
            CompilerArguments compilerArgs = new CompilerArguments(config);
            compilerArgs.getPath().addAll(pathCompileds);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(buildModule(caseDef), compilerArgs);
            pathCompileds.add(compiled);
            runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            int start = findCase(allSteps, caseDef.name);
            int sends = 0;
            List<String> fafStatements = new ArrayList<>();
            for (int i = start + 1; i < allSteps.size(); i++) {
                JsonObject step = allSteps.get(i).asObject();
                String op = step.getString("op", "");
                if ("case".equals(op)) {
                    break;
                }
                if ("send".equals(op)) {
                    sendEvent(caseDef.rep, runtime, step.getString("eventType", ""), step.get("payload").asObject());
                    sends++;
                } else if ("faf".equals(op)) {
                    String statement = step.getString("statement", "");
                    executeFaf(runtime, pathCompileds, config, caseDef.name, statement, records);
                    fafStatements.add(statement);
                } else {
                    throw new IllegalArgumentException("unsupported op " + op + " in case " + caseDef.name);
                }
            }
            if (sends != 5 || !fafStatements.equals(List.of("q1", "q2", "q3", "q4"))) {
                throw new IllegalArgumentException(
                    "case " + caseDef.name + " must contain five sends followed by faf q1..q4");
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * Verbatim transcription of the pinned module construction
     * (InfraNWTableFAF.java lines 311-331); only line wrapping was added.
     */
    private static String buildModule(CaseDef caseDef) {
        String eplEvents =
            annotationTextWJsonProvided(caseDef.rep, "MyLocalJsonProvidedProduct") +
                " @public @buseventtype create schema Product (productId string, categoryId string);" +
                annotationTextWJsonProvided(caseDef.rep, "MyLocalJsonProvidedCategory") +
                " @public @buseventtype create schema Category (categoryId string, owner string);" +
                annotationTextWJsonProvided(caseDef.rep, "MyLocalJsonProvidedProductOwnerDetails") +
                " @public @buseventtype create schema ProductOwnerDetails (productId string, owner string);";
        if (caseDef.namedWindow) {
            return eplEvents +
                "@public create window WinProduct#keepall as select * from Product;" +
                "@public create window WinCategory#keepall as select * from Category;" +
                "@public create window WinProductOwnerDetails#keepall as select * from ProductOwnerDetails;" +
                "insert into WinProduct select * from Product;" +
                "insert into WinCategory select * from Category;" +
                "insert into WinProductOwnerDetails select * from ProductOwnerDetails;";
        }
        return eplEvents +
            "@public create table WinProduct (productId string primary key, categoryId string primary key);" +
            "@public create table WinCategory (categoryId string primary key, owner string primary key);" +
            "@public create table WinProductOwnerDetails (productId string primary key, owner string);" +
            "on Product t1 merge WinProduct t2 where t1.productId = t2.productId and t1.categoryId = t2.categoryId when not matched then insert select productId, categoryId;" +
            "on Category t1 merge WinCategory t2 where t1.categoryId = t2.categoryId when not matched then insert select categoryId, owner;" +
            "on ProductOwnerDetails t1 merge WinProductOwnerDetails t2 where t1.productId = t2.productId when not matched then insert select productId, owner;";
    }

    /**
     * Mirrors EventRepresentationChoice.getAnnotationTextWJsonProvided(Class):
     * only JSONCLASSPROVIDED differs from the plain annotation text, prefixing
     * "@JsonSchema(className='...')" referencing a json-provided mirror class
     * (see class comment).
     */
    private static String annotationTextWJsonProvided(Rep rep, String providedClassSimpleName) {
        switch (rep) {
            case OBJECTARRAY:
                return "@EventRepresentation('objectarray')";
            case MAP:
                return "@EventRepresentation('map')";
            case AVRO:
                return "@EventRepresentation('avro')";
            case JSON:
                return "@EventRepresentation('json')";
            case JSONCLASSPROVIDED:
                return "@JsonSchema(className='" + InfraNWTableFAFJoinScenarioOracle.class.getName() +
                    "$" + providedClassSimpleName + "') @EventRepresentation('json')";
            case DEFAULT:
                return "";
            default:
                throw new IllegalStateException("unhandled representation " + rep);
        }
    }

    /**
     * Mirrors the pinned sendEvent helper semantics per representation; field
     * values are read in event-type declaration order because object-array
     * sends are positional.
     */

    /** Null-safe minimaljson string read: absent and JSON-null both return null. */
    private static String jsonString(JsonObject payload, String field) {
        JsonValue value = payload.get(field);
        if (value == null || value.isNull()) {
            return null;
        }
        return value.asString();
    }

    private static void sendEvent(Rep rep, EPRuntime runtime, String eventType, JsonObject payload) {
        List<String> fields = typeFieldsInDeclarationOrder(eventType);
        if (rep == Rep.OBJECTARRAY) {
            List<Object> eventObjectArray = new ArrayList<>();
            for (String field : fields) {
                eventObjectArray.add(jsonString(payload, field));
            }
            runtime.getEventService().sendEventObjectArray(eventObjectArray.toArray(), eventType);
        } else if (rep == Rep.MAP || rep == Rep.DEFAULT) {
            Map<String, Object> eventMap = new HashMap<>();
            for (String field : fields) {
                eventMap.put(field, jsonString(payload, field));
            }
            runtime.getEventService().sendEventMap(eventMap, eventType);
        } else if (rep == Rep.AVRO) {
            Schema schema = SupportAvroUtil.getAvroSchema(
                runtime.getEventTypeService().getEventTypePreconfigured(eventType));
            GenericData.Record record = new GenericData.Record(schema);
            for (String field : fields) {
                record.put(field, jsonString(payload, field));
            }
            runtime.getEventService().sendEventAvro(record, eventType);
        } else {
            JsonObject event = new JsonObject();
            for (String field : fields) {
                event.add(field, jsonString(payload, field));
            }
            runtime.getEventService().sendEventJson(event.toString(), eventType);
        }
    }

    private static List<String> typeFieldsInDeclarationOrder(String eventType) {
        switch (eventType) {
            case "Product":
                return List.of("productId", "categoryId");
            case "Category":
                return List.of("categoryId", "owner");
            case "ProductOwnerDetails":
                return List.of("productId", "owner");
            default:
                throw new IllegalArgumentException("unsupported event type " + eventType);
        }
    }

    /**
     * The four pinned fire-and-forget queries (InfraNWTableFAF.java lines
     * 344-374), verbatim including spacing; q4 is the q1 join tree with the
     * ownership predicate moved to HAVING (no group-by).
     */
    private static String fafQuery(String statementName) {
        switch (statementName) {
            case "q1":
                return "" +
                    "select WinProduct.productId " +
                    " from WinProduct" +
                    " inner join WinCategory on WinProduct.categoryId=WinCategory.categoryId" +
                    " inner join WinProductOwnerDetails on WinProduct.productId=WinProductOwnerDetails.productId";
            case "q2":
                return "" +
                    "select WinProduct.productId " +
                    " from WinProduct" +
                    " inner join WinCategory on WinProduct.categoryId=WinCategory.categoryId" +
                    " inner join WinProductOwnerDetails on WinProduct.productId=WinProductOwnerDetails.productId" +
                    " where WinCategory.owner=WinProductOwnerDetails.owner";
            case "q3":
                return "" +
                    "select WinProduct.productId " +
                    " from WinProduct, WinCategory, WinProductOwnerDetails" +
                    " where WinCategory.owner=WinProductOwnerDetails.owner" +
                    " and WinProduct.categoryId=WinCategory.categoryId" +
                    " and WinProduct.productId=WinProductOwnerDetails.productId";
            case "q4":
                return "" +
                    "select WinProduct.productId " +
                    " from WinProduct" +
                    " inner join WinCategory on WinProduct.categoryId=WinCategory.categoryId" +
                    " inner join WinProductOwnerDetails on WinProduct.productId=WinProductOwnerDetails.productId" +
                    " having WinCategory.owner=WinProductOwnerDetails.owner";
            default:
                throw new IllegalArgumentException("unsupported faf statement " + statementName);
        }
    }

private static void executeFaf(EPRuntime runtime, List<EPCompiled> pathCompileds, Configuration config, String caseName,
                               String statementName, List<JsonObject> records) {
        try {
            CompilerArguments arguments = new CompilerArguments(config);
            arguments.getPath().addAll(pathCompileds);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery(fafQuery(statementName), arguments);
            EventBean[] rows = runtime.getFireAndForgetService().executeQuery(compiled).getArray();

            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "faf");
            record.add("statement", statementName);
            JsonArray newArr = new JsonArray();
            for (EventBean row : rows) {
                JsonObject newItem = new JsonObject();
                newItem.add("kind", "row");
                JsonObject fields = new JsonObject();
                for (String prop : new TreeSet<>(java.util.Arrays.asList(row.getEventType().getPropertyNames()))) {
                    fields.add(prop, normalize(row.get(prop)));
                }
                newItem.add("fields", fields);
                newArr.add(newItem);
            }
            record.add("new", newArr);
            records.add(record);
        } catch (EPCompileException ex) {
            throw new IllegalStateException("FAF compile failed for " + caseName + "/" + statementName, ex);
        } catch (RuntimeException ex) {
            throw new IllegalStateException("FAF query failed for " + caseName + "/" + statementName, ex);
        }
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short || value instanceof Byte) {
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

    /** Local mirrors of the pinned MyLocalJsonProvided* regression classes. */
    public static class MyLocalJsonProvidedProduct implements Serializable {
        public String productId;
        public String categoryId;
    }

    /** Local mirrors of the pinned MyLocalJsonProvided* regression classes. */
    public static class MyLocalJsonProvidedCategory implements Serializable {
        public String categoryId;
        public String owner;
    }

    /** Local mirrors of the pinned MyLocalJsonProvided* regression classes. */
    public static class MyLocalJsonProvidedProductOwnerDetails implements Serializable {
        public String productId;
        public String owner;
    }
}
