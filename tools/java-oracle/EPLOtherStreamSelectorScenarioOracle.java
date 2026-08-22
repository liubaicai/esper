import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Java oracle for EPLOtherSelectExprStreamSelector stream wildcard scenarios.
 *
 * Covers the NoJoinWildcardWithAlias, JoinWildcardWithAlias, AloneJoinAlias,
 * and AloneJoinNoAlias runtimes by replaying deterministic event sequences and
 * recording the observable output column names and event identity.
 *
 * SupportMarketDataBean is registered as a Map event type to avoid compiling
 * the full regression-lib module; the observable contract (property names,
 * event identity, column presence) is identical.
 */
public class EPLOtherStreamSelectorScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLOtherStreamSelectorScenarioOracle <scenario.json>");
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
        root.add("javaCommit", "9e1b9f1cc9117fea4bf33ab043762c045d73839c");
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
        Map<String, Object> marketType = new HashMap<>();
        marketType.put("symbol", String.class);
        marketType.put("volume", Long.class);
        marketType.put("price", Double.class);
        config.getCommon().addEventType("SupportMarketDataBean", marketType);
        if (caseName.equals("config-selector-istream")) {
            config.getCompiler().getStreamSelection().setDefaultStreamSelector(
                com.espertech.esper.common.client.soda.StreamSelector.RSTREAM_ISTREAM_BOTH);
        } else if (caseName.equals("config-selector-rstream")) {
            config.getCompiler().getStreamSelection().setDefaultStreamSelector(
                com.espertech.esper.common.client.soda.StreamSelector.RSTREAM_ONLY);
        }
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLOtherStreamSelectorScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String epl = buildEPL(caseName);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());

            EPStatement stmt = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    stmt = candidate;
                    break;
                }
            }
            if (stmt == null) {
                throw new IllegalStateException("statement s0 not found");
            }

            java.util.Set<String> projected = selectorProjectedFields(caseName);
            int[] seq = new int[] {0};
            stmt.addListener((newData, oldData, statement, rt) -> {
                boolean hasNew = newData != null && newData.length > 0;
                boolean hasOld = oldData != null && oldData.length > 0;
                if (!hasNew && !hasOld) {
                    return;
                }
                seq[0]++;
                JsonObject record = new JsonObject();
                record.add("case", caseName);
                record.add("operation", "listener");
                record.add("statement", statement.getName());
                record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                record.add("sequence", seq[0]);
                JsonArray newArr = new JsonArray();
                for (EventBean event : hasNew ? newData : new EventBean[0]) {
                    newArr.add(selectorRow(caseName, event, projected));
                }
                record.add("new", newArr);
                if (hasOld) {
                    JsonArray oldArr = new JsonArray();
                    for (EventBean event : oldData) {
                        oldArr.add(selectorRow(caseName, event, projected));
                    }
                    record.add("old", oldArr);
                }
                records.add(record);
            });

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
                    String type = step.getString("eventType", "");
                    JsonObject payload = step.get("payload").asObject();
                    switch (type) {
                        case "SupportBean": {
                            SupportBean bean = new SupportBean();
                            bean.setTheString(payload.getString("theString", ""));
                            bean.setIntPrimitive(payload.getInt("intPrimitive", 0));
                            runtime.getEventService().sendEventBean(bean, type);
                            break;
                        }
                        case "SupportMarketDataBean": {
                            Map<String, Object> marketEvent = new HashMap<>();
                            marketEvent.put("symbol", payload.getString("symbol", ""));
                            marketEvent.put("volume", (long) payload.getDouble("volume", 0.0));
                            marketEvent.put("price", payload.getDouble("price", 0.0));
                            runtime.getEventService().sendEventMap(marketEvent, type);
                            break;
                        }
                        default:
                            throw new IllegalStateException("unknown type: " + type);
                    }
                }
            }

            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static String summarizeUnderlying(EventBean event) {
        Object underlying = event.getUnderlying();
        if (underlying instanceof SupportBean) {
            SupportBean b = (SupportBean) underlying;
            return "SupportBean(theString=" + b.getTheString() + ",intPrimitive=" + b.getIntPrimitive() + ")";
        }
        if (underlying instanceof Map) {
            @SuppressWarnings("unchecked")
            Map<String, Object> m = (Map<String, Object>) underlying;
            return "SupportMarketDataBean(symbol=" + m.get("symbol") + ",volume=" + m.get("volume") + ")";
        }
        return String.valueOf(underlying);
    }

    /** Assertion-surface field projection per execution; null keeps all columns. */
    private static java.util.Set<String> selectorProjectedFields(String caseName) {
        switch (caseName) {
            case "nojoin-wildcard-noalias":
            case "config-selector-istream":
            case "config-selector-rstream":
                return new java.util.HashSet<>(java.util.Arrays.asList("theString", "intPrimitive"));
            case "join-wildcard-noalias":
                return new java.util.HashSet<>(java.util.Arrays.asList("s0", "s1", "symbol", "volume"));
            case "nojoin-wildcard-alias":
                return new java.util.HashSet<>(java.util.Arrays.asList("theString", "intPrimitive", "s0"));
            case "join-wildcard-alias":
                return new java.util.HashSet<>(java.util.Arrays.asList("s0", "s1", "s0stream", "s1stream"));
            case "nojoin-noalias-withproperties":
                return new java.util.HashSet<>(java.util.Arrays.asList("a", "theString", "intPrimitive", "b"));
            case "join-noalias-withproperties":
                return new java.util.HashSet<>(java.util.Arrays.asList("intPrimitive", "sym", "symbol"));
            case "alone-nojoin-noalias":
                return new java.util.HashSet<>(java.util.Arrays.asList("theString"));
            case "alone-nojoin-alias":
                return new java.util.HashSet<>(java.util.Arrays.asList("s0"));
            case "alone-join-alias":
                return new java.util.HashSet<>(java.util.Arrays.asList("s1"));
            case "alone-join-alias-reverse":
                return new java.util.HashSet<>(java.util.Arrays.asList("szero"));
            case "alone-join-noalias":
                return new java.util.HashSet<>(java.util.Arrays.asList("symbol"));
            case "alone-join-noalias-reverse":
                return new java.util.HashSet<>(java.util.Arrays.asList("theString"));
            default:
                return null;
        }
    }

    /** Builds one projected output row for the stream-selector family. */
    private static JsonObject selectorRow(String caseName, EventBean event, java.util.Set<String> projected) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject fields = new JsonObject();
        for (String prop : event.getEventType().getPropertyNames()) {
            if (projected != null && !projected.contains(prop)) {
                continue;
            }
            Object value = event.get(prop);
            fields.add(prop, selectorValue(value));
        }
        item.add("fields", fields);
        return item;
    }

    /** Deterministic rendering shared with the Go runner for event/map columns. */
    private static String selectorValue(Object value) {
        if (value == null) {
            return null;
        }
        if (value instanceof EventBean inner) {
            return selectorValue(inner.getUnderlying());
        }
        if (value instanceof com.espertech.esper.common.internal.support.SupportBean bean) {
            return "SupportBean[theString=" + bean.getTheString()
                + ",intPrimitive=" + bean.getIntPrimitive() + "]";
        }
        if (value instanceof Map<?, ?> map) {
            StringBuilder sb = new StringBuilder("{");
            List<String> keys = new ArrayList<>();
            for (Object key : map.keySet()) {
                keys.add(String.valueOf(key));
            }
            java.util.Collections.sort(keys);
            for (int i = 0; i < keys.size(); i++) {
                if (i > 0) {
                    sb.append(";");
                }
                sb.append(keys.get(i)).append("=").append(map.get(keys.get(i)));
            }
            return sb.append("}").toString();
        }
        return String.valueOf(value);
    }

    private static String buildEPL(String caseName) {
        return switch (caseName) {
            case "nojoin-wildcard-alias" ->
                "@name('s0') select *, win.* as s0 from SupportBean#length(3) as win";
            case "join-wildcard-alias" ->
                "@name('s0') select *, s1.* as s1stream, s0.* as s0stream from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "alone-join-alias" ->
                "@name('s0') select s1.* as s1 from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "alone-join-noalias" ->
                "@name('s0') select s1.* from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "nojoin-wildcard-noalias" ->
                "@name('s0') select *, win.* from SupportBean#length(3) as win";
            case "join-wildcard-noalias" ->
                "@name('s0') select *, s1.* from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "nojoin-noalias-withproperties" ->
                "@name('s0') select intPrimitive as a, string.*, intPrimitive as b from SupportBean#length(3) as string";
            case "join-noalias-withproperties" ->
                "@name('s0') select intPrimitive, s1.*, symbol as sym from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "alone-nojoin-noalias" ->
                "@name('s0') select theString.* from SupportBean#length(3) as theString";
            case "alone-nojoin-alias" ->
                "@name('s0') select theString.* as s0 from SupportBean#length(3) as theString";
            case "alone-join-alias-reverse" ->
                "@name('s0') select s0.* as szero from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "alone-join-noalias-reverse" ->
                "@name('s0') select s0.* from SupportBean#length(3) as s0, " +
                "SupportMarketDataBean#keepall as s1";
            case "config-selector-istream", "config-selector-rstream" ->
                "@name('s0') select * from SupportBean#length(3)";
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }
}
