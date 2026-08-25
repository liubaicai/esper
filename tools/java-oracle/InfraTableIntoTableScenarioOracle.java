import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetQueryResult;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerContext;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.regressionlib.support.bean.SupportBeanNumeric;
import com.espertech.esper.regressionlib.support.bean.SupportEventWithManyArray;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for the InfraTableIntoTable into-table parity scenario.
 *
 * Covers nine executions across eleven cases. Unkeyed-simple-same-module
 * and unkeyed-simple-two-module replay InfraIntoTableUnkeyedSimpleSameModule
 * and InfraIntoTableUnkeyedSimpleTwoModule: a count(*) table MyTable filled
 * by an into-table from SupportBean and observed through create-table
 * statement snapshots across an empty/one/two-row cycle. Bound-unbound-minmax,
 * bound-unbound-lastfirst-window and bound-unbound-sorted-minmaxby each
 * replay one representative sub-phase of InfraBoundUnbound: a @public table
 * varagg declaring bound and unbound aggregation columns, an iterate
 * statement over SupportBean_S0#lastevent fed by one S0(0), then bound-window
 * and unbound-ever into-table statements; observations flatten the varagg
 * Map of the iterate row and rejected into-table compilations are recorded
 * as build-error records carrying the full EPCompileException message.
 * Window-sorted-from-join replays InfraIntoTableWindowSortedFromJoin through
 * fire-and-forget select over MyTable(thewin window(*), thesort sorted(int
 * Primitive desc)). No-keys and with-keys replay InfraTableIntoTableNoKeys
 * and InfraTableIntoTableWithKeys: sum tables driven through a subquery-over-
 * table s0 statement whose listener records c0 per probe alongside
 * Create-Table snapshots. Big-number-avg-sum replays
 * InfraTableBigNumberAggregation over SupportBeanNumeric#lastevent.
 * Multikey-warray-single and multikey-warray-two replay the primitive
 * int-array primary-key tables grouped by intOne or intOne,intTwo.
 *
 * Events are SupportBean payloads carrying theString/intPrimitive,
 * SupportBean_S0 payloads carrying id/p00, SupportBeanNumeric payloads
 * carrying bigint/bigdec decimal strings and SupportEventWithManyArray
 * payloads carrying id/value/intOne/intTwo.
 *
 * Observations follow the standard protocol: snapshot records placed at the
 * pinned assertIterator/assertPropsPerRowIteratorAnyOrder positions (rows
 * carry AnyOrder canonical-JSON ordering, field names sorted), automatic
 * listener records for the s0 statement with per-statement sequence numbers,
 * and build-error records carrying the full EPCompileException message so
 * both runtimes compare identical failure text. Time is frozen at epoch zero
 * because the internal timer is disabled and time is advanced to zero only.
 */
public class InfraTableIntoTableScenarioOracle {
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: InfraTableIntoTableScenarioOracle <scenario.json>");
            System.exit(2);
        }
        String scenarioText = Files.readString(Path.of(args[0]), StandardCharsets.UTF_8);
        JsonObject scenario = Json.parse(scenarioText).asObject();
        JsonArray allSteps = scenario.get("steps").asArray();
        List<JsonObject> records = new ArrayList<>();

        for (JsonValue caseVal : scenario.get("cases").asArray()) {
            String caseName = caseVal.asObject().getString("case", "");
            runCase(allSteps, caseName, records);
        }

        JsonObject root = new JsonObject();
        root.add("version", "esper-parity/v1");
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

    private static void runCase(JsonArray allSteps, String caseName, List<JsonObject> records) throws Exception {
        Configuration config = new Configuration();
        config.getCommon().addEventType(SupportBean.class);
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBeanNumeric.class);
        config.getCommon().addEventType(SupportEventWithManyArray.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        // The pinned runner rethrows statement exceptions on the sending
        // thread; mirror that so unexpected failures surface loudly instead
        // of being absorbed by the default handler.
        config.getRuntime().getExceptionHandling().addClass(RethrowExceptionHandlerFactory.class);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("InfraTableIntoTableScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            // Each InfraBoundUnbound sub-phase runs against its own fresh
            // RegressionPath after env.undeployAll(); mirror that isolation
            // by undeploying everything and dropping accumulated path
            // compileds between phases.
            List<List<SetupAction>> phases = phases(caseName);
            for (int phaseIndex = 0; phaseIndex < phases.size(); phaseIndex++) {
                if (phaseIndex > 0) {
                    runtime.getDeploymentService().undeployAll();
                }
                CaseState state = new CaseState(caseName, config, runtime, records);
                deployPhase(state, phases.get(phaseIndex));

                boolean inCase = false;
                for (JsonValue stepVal : allSteps) {
                    JsonObject step = stepVal.asObject();
                    String op = step.getString("op", "");
                    if ("case".equals(op)) {
                        inCase = caseName.equals(step.getString("case", ""));
                        continue;
                    }
                    if (!inCase) {
                        continue;
                    }
                    if ("send".equals(op)) {
                        sendEvent(runtime, step);
                    } else if ("snapshot".equals(op)) {
                        snapshot(state, step);
                    } else if ("build-error".equals(op)) {
                        buildError(state, step);
                    } else {
                        throw new IllegalStateException("unsupported op " + op);
                    }
                }
            }
        } finally {
            runtime.destroy();
        }
    }

    private static void deployPhase(CaseState state, List<SetupAction> actions) throws Exception {
        for (SetupAction action : actions) {
            if (action instanceof DeployEpl) {
                String epl = ((DeployEpl) action).epl;
                CompilerArguments compilerArgs = new CompilerArguments(state.config);
                compilerArgs.getPath().getCompileds().addAll(state.pathCompileds);
                EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
                state.pathCompileds.add(compiled);
                EPDeployment deployment = state.runtime.getDeploymentService()
                    .deploy(compiled, new DeploymentOptions());
                for (EPStatement added : deployment.getStatements()) {
                    state.statements.add(added);
                    if ("s0".equals(added.getName())) {
                        added.addListener(new ListenerRecorder(state.caseName, added, state.runtime, state.records));
                    }
                }
            } else if (action instanceof SendS0) {
                state.runtime.getEventService()
                    .sendEventBean(new SupportBean_S0(((SendS0) action).id), "SupportBean_S0");
            } else {
                throw new IllegalStateException("unsupported setup action " + action.getClass());
            }
        }
    }

    private static List<List<SetupAction>> phases(String caseName) {
        return switch (caseName) {
            case "unkeyed-simple-same-module" -> List.of(List.of(
                new DeployEpl("@name('tbl') create table MyTable(mycnt count(*));\n" +
                    "into table MyTable select count(*) as mycnt from SupportBean;\n")));
            case "unkeyed-simple-two-module" -> List.of(List.of(
                new DeployEpl("@name('tbl') @public create table MyTable(mycnt count(*))"),
                new DeployEpl("into table MyTable select count(*) as mycnt from SupportBean;\n")));
            case "bound-unbound-minmax" -> List.of(minMaxPhase());
            case "bound-unbound-lastfirst-window" -> List.of(lastFirstWindowPhase());
            case "bound-unbound-sorted-minmaxby" -> List.of(sortedMinMaxByPhase());
            case "window-sorted-from-join" -> List.of(List.of(
                new DeployEpl("@public create table MyTable(" +
                    "thewin window(*) @type('SupportBean')," +
                    "thesort sorted(intPrimitive desc) @type('SupportBean')" +
                    ")"),
                new DeployEpl("into table MyTable " +
                    "select window(sb.*) as thewin, sorted(sb.*) as thesort " +
                    "from SupportBean_S0#lastevent, SupportBean#keepall as sb")));
            case "no-keys" -> List.of(List.of(
                new DeployEpl("@Name('Create-Table')@public  create table MyTable(sumint sum(int))"),
                new DeployEpl("@Name('Into-Table') into table MyTable select sum(intPrimitive) as sumint from SupportBean"),
                new DeployEpl("@Name('s0') select (select sumint from MyTable) as c0 from SupportBean_S0 as s0")));
            case "with-keys" -> List.of(List.of(
                new DeployEpl("@Name('Create-Table') @public create table MyTable(pkey string primary key, sumint sum(int))"),
                new DeployEpl("@Name('Into-Table') into table MyTable select sum(intPrimitive) as sumint from SupportBean group by theString"),
                new DeployEpl("@Name('s0') select (select sumint from MyTable where pkey = s0.p00) as c0 from SupportBean_S0 as s0")));
            case "big-number-avg-sum" -> List.of(List.of(
                new DeployEpl("@name('tbl') create table MyTable as (c0 avg(BigInteger), c1 avg(BigDecimal), c2 sum(BigInteger), c3 sum(BigDecimal));\n" +
                    "into table MyTable select avg(bigint) as c0, avg(bigdec) as c1, sum(bigint) as c2, sum(bigdec) as c3  from SupportBeanNumeric#lastevent;\n")));
            case "multikey-warray-single" -> List.of(List.of(
                new DeployEpl("@name('tbl') create table MyTable(k int[primitive] primary key, thesum sum(int));\n" +
                    "into table MyTable select intOne, sum(value) as thesum from SupportEventWithManyArray group by intOne;\n")));
            case "multikey-warray-two" -> List.of(List.of(
                new DeployEpl("@name('tbl') create table MyTable(k1 int[primitive] primary key, k2 int[primitive] primary key, thesum sum(int));\n" +
                    "into table MyTable select intOne, intTwo, sum(value) as thesum from SupportEventWithManyArray group by intOne, intTwo;\n")));
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    /**
     * Mirrors tryAssertionMinMax: declare, iterate, S0(0), bound into, then
     * unbound into; steps observe the iterate row and reject the unbound max
     * compilation.
     */
    private static List<SetupAction> minMaxPhase() {
        return List.of(
            new DeployEpl("@public create table varagg (" +
                "maxb max(int), maxu maxever(int), minb min(int), minu minever(int))"),
            new DeployEpl("@name('iterate') select varagg from SupportBean_S0#lastevent"),
            new SendS0(0),
            new DeployEpl("into table varagg select " +
                "max(intPrimitive) as maxb, min(intPrimitive) as minb " +
                "from SupportBean#length(2)"),
            new DeployEpl("into table varagg select " +
                "maxever(intPrimitive) as maxu, minever(intPrimitive) as minu " +
                "from SupportBean"));
    }

    /**
     * Mirrors tryAssertionLastFirstWindow: lastever/firstever/window columns,
     * bound window into plus unbound ever into, three rejected compilations.
     */
    private static List<SetupAction> lastFirstWindowPhase() {
        return List.of(
            new DeployEpl("@public create table varagg (" +
                "lasteveru lastever(*) @type('SupportBean'), " +
                "firsteveru firstever(*) @type('SupportBean'), " +
                "windowb window(*) @type('SupportBean'))"),
            new DeployEpl("@name('iterate') select varagg from SupportBean_S0#lastevent"),
            new SendS0(0),
            new DeployEpl("into table varagg select window(*) as windowb from SupportBean#length(2)"),
            new DeployEpl("into table varagg select lastever(*) as lasteveru, firstever(*) as firsteveru from SupportBean"));
    }

    /**
     * Mirrors tryAssertionSortedMinMaxBy: maxbyever/minbyever/sorted columns,
     * bound sorted into plus unbound ever into, two rejected compilations.
     */
    private static List<SetupAction> sortedMinMaxByPhase() {
        return List.of(
            new DeployEpl("@public create table varagg (" +
                "maxbyeveru maxbyever(intPrimitive) @type('SupportBean'), " +
                "minbyeveru minbyever(intPrimitive) @type('SupportBean'), " +
                "sortedb sorted(intPrimitive) @type('SupportBean'))"),
            new DeployEpl("@name('iterate') select varagg from SupportBean_S0#lastevent"),
            new SendS0(0),
            new DeployEpl("into table varagg select sorted() as sortedb from SupportBean#length(2)"),
            new DeployEpl("into table varagg select maxbyever() as maxbyeveru, minbyever() as minbyeveru from SupportBean"));
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean" -> {
                SupportBean event = new SupportBean();
                JsonValue theStringVal = payload.get("theString");
                event.setTheString(theStringVal instanceof JsonString ? ((JsonString) theStringVal).asString() : null);
                JsonValue intPrimitiveVal = payload.get("intPrimitive");
                if (intPrimitiveVal instanceof JsonNumber) {
                    event.setIntPrimitive(((JsonNumber) intPrimitiveVal).asInt());
                }
                runtime.getEventService().sendEventBean(event, "SupportBean");
            }
            case "SupportBean_S0" -> {
                SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0), payload.getString("p00", null));
                runtime.getEventService().sendEventBean(event, "SupportBean_S0");
            }
            case "SupportBeanNumeric" -> {
                BigInteger bigint = null;
                BigDecimal bigdec = null;
                JsonValue bigintVal = payload.get("bigint");
                if (bigintVal instanceof JsonString) {
                    bigint = new BigInteger(((JsonString) bigintVal).asString());
                }
                JsonValue bigdecVal = payload.get("bigdec");
                if (bigdecVal instanceof JsonString) {
                    bigdec = new BigDecimal(((JsonString) bigdecVal).asString());
                }
                runtime.getEventService().sendEventBean(new SupportBeanNumeric(bigint, bigdec), "SupportBeanNumeric");
            }
            case "SupportEventWithManyArray" -> {
                SupportEventWithManyArray event = new SupportEventWithManyArray(payload.getString("id", null));
                JsonValue valueVal = payload.get("value");
                if (valueVal instanceof JsonNumber) {
                    event.withValue(((JsonNumber) valueVal).asInt());
                }
                event.withIntOne(intArray(payload.get("intOne")));
                JsonValue intTwoVal = payload.get("intTwo");
                event.withIntTwo(intTwoVal == null ? null : intArray(intTwoVal));
                runtime.getEventService().sendEventBean(event, "SupportEventWithManyArray");
            }
            default -> throw new IllegalStateException("unknown eventType: " + type);
        }
    }

    private static int[] intArray(JsonValue value) {
        if (!(value instanceof JsonArray)) {
            throw new IllegalStateException("expected array payload member, found " + value);
        }
        JsonArray array = (JsonArray) value;
        int[] result = new int[array.size()];
        for (int i = 0; i < array.size(); i++) {
            result[i] = array.get(i).asInt();
        }
        return result;
    }

    /**
     * Snapshot at the pinned assertion positions. tbl/create-state read the
     * create-table statement iterator; varagg-state flattens the varagg Map
     * of the iterate row; faf-table executes fire-and-forget select over the
     * table. Rows carry AnyOrder canonical-JSON ordering.
     */
    private static void snapshot(CaseState state, JsonObject step) throws Exception {
        String label = step.getString("statement", "");
        List<JsonObject> rows = new ArrayList<>();
        switch (label) {
            case "tbl" -> collectIteratorRows(rows, findStatement(state, "tbl"));
            case "create-state" -> collectIteratorRows(rows, findStatement(state, "Create-Table"));
            case "varagg-state" -> {
                EPStatement target = findStatement(state, "iterate");
                Iterator<EventBean> it = target.iterator();
                if (!it.hasNext()) {
                    throw new IllegalStateException("iterate produced no row in case " + state.caseName);
                }
                Object varagg = it.next().get("varagg");
                if (!(varagg instanceof Map)) {
                    throw new IllegalStateException("varagg is not a Map: " + varagg);
                }
                rows.add(flattenMap((Map<?, ?>) varagg));
            }
            case "faf-table" -> {
                CompilerArguments compilerArgs = new CompilerArguments(state.config);
                compilerArgs.getPath().getCompileds().addAll(state.pathCompileds);
                EPCompiled compiled = EPCompilerProvider.getCompiler().compileQuery("select * from MyTable", compilerArgs);
                EPFireAndForgetQueryResult result = state.runtime.getFireAndForgetService().executeQuery(compiled);
                for (EventBean event : result.getArray()) {
                    rows.add(renderRow(event));
                }
            }
            default -> throw new IllegalStateException("unknown snapshot label: " + label);
        }
        rows.sort(java.util.Comparator.comparing(row -> canonical(row)));

        JsonObject record = new JsonObject();
        record.add("case", state.caseName);
        record.add("operation", "snapshot");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(state.runtime.getEventService().getCurrentTime()).toString());
        JsonArray rowsArr = new JsonArray();
        for (JsonObject row : rows) {
            rowsArr.add(row);
        }
        record.add("new", rowsArr);
        state.records.add(record);
    }

    /**
     * Compiles the pinned EPL expected to fail and records the full
     * EPCompileException message, or the "&lt;no-error&gt;" drift marker.
     */
    private static void buildError(CaseState state, JsonObject step) throws Exception {
        String epl = step.getString("epl", "");
        CompilerArguments compilerArgs = new CompilerArguments(state.config);
        compilerArgs.getPath().getCompileds().addAll(state.pathCompileds);
        String message;
        try {
            EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
            message = "<no-error>";
        } catch (EPCompileException ex) {
            message = ex.getMessage();
        }
        JsonObject record = new JsonObject();
        record.add("case", state.caseName);
        record.add("operation", "build-error");
        record.add("statement", step.getString("statement", ""));
        record.add("sequence", 0);
        record.add("time", Instant.ofEpochMilli(state.runtime.getEventService().getCurrentTime()).toString());
        record.add("value", message);
        state.records.add(record);
    }

    private static EPStatement findStatement(CaseState state, String name) {
        for (EPStatement candidate : state.statements) {
            if (name.equals(candidate.getName())) {
                return candidate;
            }
        }
        throw new IllegalStateException("no statement named " + name + " in case " + state.caseName);
    }

    private static void collectIteratorRows(List<JsonObject> rows, EPStatement target) {
        Iterator<EventBean> it = target.iterator();
        while (it.hasNext()) {
            rows.add(renderRow(it.next()));
        }
    }

    /** Flattens a Map (the varagg table state) into a single row object. */
    private static JsonObject flattenMap(Map<?, ?> map) {
        TreeSet<String> keys = new TreeSet<>();
        for (Object key : map.keySet()) {
            keys.add(String.valueOf(key));
        }
        JsonObject fields = new JsonObject();
        for (String key : keys) {
            fields.add(key, normalize(map.get(key)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    private static JsonObject renderRow(EventBean event) {
        JsonObject fields = new JsonObject();
        for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
            fields.add(prop, normalize(event.get(prop)));
        }
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        item.add("fields", fields);
        return item;
    }

    /**
     * Canonical JSON text used for AnyOrder row ordering: fields were built
     * with sorted names and minimal-json preserves insertion order, so the
     * serialized form is the normalized key-sorted rendering.
     */
    private static String canonical(JsonObject row) {
        return row.get("fields").toString();
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
        if (value instanceof BigInteger) {
            // Arbitrary-precision values render as exact decimal strings so
            // both runtimes compare byte-identical text without float drift.
            return Json.value(value.toString());
        }
        if (value instanceof BigDecimal) {
            return Json.value(((BigDecimal) value).toPlainString());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof SupportBean bean) {
            JsonObject fields = new JsonObject();
            fields.add("intPrimitive", Json.value(bean.getIntPrimitive()));
            fields.add("theString", Json.value(bean.getTheString()));
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            item.add("fields", fields);
            return item;
        }
        if (value instanceof EventBean) {
            EventBean inner = (EventBean) value;
            JsonObject fields = new JsonObject();
            for (String prop : new TreeSet<>(Arrays.asList(inner.getEventType().getPropertyNames()))) {
                fields.add(prop, normalize(inner.get(prop)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof Map<?, ?>) {
            Map<?, ?> mapValue = (Map<?, ?>) value;
            TreeSet<String> keys = new TreeSet<>();
            for (Object key : mapValue.keySet()) {
                keys.add(String.valueOf(key));
            }
            JsonObject fields = new JsonObject();
            for (String key : keys) {
                fields.add(key, normalize(mapValue.get(key)));
            }
            JsonObject rowObj = new JsonObject();
            rowObj.add("kind", "row");
            rowObj.add("fields", fields);
            return rowObj;
        }
        if (value instanceof Collection<?>) {
            JsonArray items = new JsonArray();
            for (Object element : (Collection<?>) value) {
                items.add(normalize(element));
            }
            return items;
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
        if (value instanceof long[]) {
            JsonArray items = new JsonArray();
            for (long element : (long[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /** Per-deployment state shared by the ops of one case phase. */
    private static final class CaseState {
        private final String caseName;
        private final Configuration config;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private final List<EPStatement> statements = new ArrayList<>();
        private final List<EPCompiled> pathCompileds = new ArrayList<>();

        private CaseState(String caseName, Configuration config, EPRuntime runtime, List<JsonObject> records) {
            this.caseName = caseName;
            this.config = config;
            this.runtime = runtime;
            this.records = records;
        }
    }

    private abstract static class SetupAction {
    }

    private static final class DeployEpl extends SetupAction {
        private final String epl;

        private DeployEpl(String epl) {
            this.epl = epl;
        }
    }

    private static final class SendS0 extends SetupAction {
        private final int id;

        private SendS0(int id) {
            this.id = id;
        }
    }

    /**
     * Automatic s0 listener mirroring env.addListener("s0"): every update
     * emits one listener record whose sequence increments per statement.
     */
    private static final class ListenerRecorder implements UpdateListener {
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private final List<JsonObject> records;
        private long sequence;

        private ListenerRecorder(String caseName, EPStatement statement, EPRuntime runtime, List<JsonObject> records) {
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
            this.records = records;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", ++sequence);
            record.add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = new JsonArray();
            if (newEvents != null) {
                for (EventBean event : newEvents) {
                    newArray.add(renderRow(event));
                }
            }
            record.add("new", newArray);
            records.add(record);
        }
    }

    /**
     * Mirrors the pinned runner's SupportExceptionHandlerFactoryRethrow:
     * statement exceptions rethrow on the sending thread instead of being
     * absorbed by the default handler.
     */
    public static class RethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return new ExceptionHandler() {
                @Override
                public void handle(ExceptionHandlerContext context) {
                    throw new RuntimeException("Unexpected exception in statement '" + context.getStatementName() +
                        "': " + context.getThrowable().getMessage(), context.getThrowable());
                }
            };
        }
    }
}
