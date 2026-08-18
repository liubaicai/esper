import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.internal.support.SupportBean_S1;
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
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;

/**
 * Java oracle for EPLSubselectMulticolumn multi-column subselect scenarios.
 *
 * Covers 3 behavioral executions (4 scenario cases; columns-uncorrelated-om is
 * the fresh-statement OM re-deploy round of ColumnsUncorrelated).
 * EPLSubselectInvalid is excluded (compile-time-only, approved difference).
 *
 * Multi-column subselect results are nested row maps (s1totals/subrow); they
 * are rendered as nested JSON objects whose keys are sorted. window() results
 * (subrow.v3/subrow.v4) are asserted any-order upstream, so their arrays are
 * expanded element-wise, sorted on their rendered canonical form, and event
 * elements are rendered as protocol row objects.
 */
public class EPLSubselectMulticolumnScenarioOracle {

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: EPLSubselectMulticolumnScenarioOracle <scenario.json>");
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
        config.getCommon().addEventType(SupportBean_S0.class);
        config.getCommon().addEventType(SupportBean_S1.class);
        config.getRuntime().getThreading().setInternalTimerEnabled(false);
        EPRuntime runtime = EPRuntimeProvider.getRuntime("EPLSubselectMulticolumnScenarioOracle-" + caseName, config);
        runtime.getEventService().advanceTime(0);
        try {
            String[] epls = buildEPL(caseName);
            List<EPStatement> s0Statements = new ArrayList<>();
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epls[0], new CompilerArguments(config));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    s0Statements.add(candidate);
                }
            }

            int[] seq = new int[] {0};
            for (EPStatement stmt : s0Statements) {
                stmt.addListener((newData, oldData, statement, rt) -> {
                    if (newData != null && newData.length > 0) {
                        seq[0]++;
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "listener");
                        record.add("statement", statement.getName());
                        record.add("sequence", seq[0]);
                        record.add("time", java.time.Instant.ofEpochMilli(rt.getEventService().getCurrentTime()).toString());
                        JsonArray newArr = new JsonArray();
                        for (EventBean event : newData) {
                            JsonObject newItem = new JsonObject();
                            newItem.add("kind", "row");
                            JsonObject fields = new JsonObject();
                            for (String prop : new TreeSet<>(Arrays.asList(event.getEventType().getPropertyNames()))) {
                                fields.add(prop, normalize(event.get(prop)));
                            }
                            newItem.add("fields", fields);
                            newArr.add(newItem);
                        }
                        record.add("new", newArr);
                        records.add(record);
                    }
                });
            }

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
                }
            }

        } finally {
            runtime.destroy();
        }
    }

    private static void sendEvent(EPRuntime runtime, JsonObject step) {
        String type = step.getString("eventType", "");
        JsonObject payload = step.get("payload").asObject();
        switch (type) {
            case "SupportBean": {
                SupportBean event = new SupportBean();
                event.setTheString(payload.getString("theString", ""));
                event.setIntPrimitive(payload.getInt("intPrimitive", 0));
                runtime.getEventService().sendEventBean(event, "SupportBean");
                break;
            }
            case "SupportBean_S0": {
                SupportBean_S0 event = new SupportBean_S0(payload.getInt("id", 0));
                String p00 = payload.getString("p00", null);
                if (p00 != null) {
                    event.setP00(p00);
                }
                runtime.getEventService().sendEventBean(event, "SupportBean_S0");
                break;
            }
            case "SupportBean_S1": {
                SupportBean_S1 event = new SupportBean_S1(payload.getInt("id", 0));
                String p10 = payload.getString("p10", null);
                if (p10 != null) {
                    event.setP10(p10);
                }
                runtime.getEventService().sendEventBean(event, "SupportBean_S1");
                break;
            }
            default:
                throw new IllegalStateException("unknown type: " + type);
        }
    }

    private static String[] buildEPL(String caseName) {
        return switch (caseName) {
            case "multicolumn-agg" -> new String[]{
                "@name('s0') select id, "
                    + "(select count(*) as v1, sum(id) as v2 from SupportBean_S1#length(3)) as s1totals "
                    + "from SupportBean_S0 s0"
            };
            case "columns-uncorrelated", "columns-uncorrelated-om" -> new String[]{
                "@name('s0') select "
                    + "(select theString as v1, intPrimitive as v2 from SupportBean#lastevent) as subrow "
                    + "from SupportBean_S0 as s0"
            };
            case "correlated-aggregation" -> new String[]{
                "@name('s0') select p00, "
                    + "(select sum(intPrimitive) as v1, sum(intPrimitive + 1) as v2, "
                    + "window(intPrimitive) as v3, window(sb.*) as v4 "
                    + "from SupportBean#keepall sb where theString = s0.p00) as subrow "
                    + "from SupportBean_S0 s0"
            };
            default -> throw new IllegalStateException("unknown case: " + caseName);
        };
    }

    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof SupportBean) {
            // window(sb.*) elements arrive as raw SupportBean POJOs (probed at
            // the pinned commit); render them as protocol row objects over the
            // scenario-relevant fields, matching the Go trace normalizer.
            return supportBeanRow((SupportBean) value);
        }
        if (value instanceof EventBean) {
            // Event-valued columns render as the protocol row shape
            // {"kind":"row","fields":{...}}, matching the Go compat trace
            // normalizer's Event rendering.
            return eventRow((EventBean) value);
        }
        if (value instanceof Map) {
            // Nested subquery row maps (s1totals/subrow) render as a bare
            // sorted-fields object, matching the Go normalizer's nested-map
            // branch (no "kind" wrapper).
            return fieldsObject((Map<?, ?>) value);
        }
        if (value instanceof Object[]) {
            // window() results are asserted any-order upstream: render each
            // element first, then sort on the rendered canonical form so the
            // Java and Go traces agree on element order.
            Object[] elements = (Object[]) value;
            JsonValue[] rendered = new JsonValue[elements.length];
            for (int i = 0; i < elements.length; i++) {
                rendered[i] = normalize(elements[i]);
            }
            Arrays.sort(rendered, EPLSubselectMulticolumnScenarioOracle::compareRendered);
            JsonArray arr = new JsonArray();
            for (JsonValue item : rendered) {
                arr.add(item);
            }
            return arr;
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

    private static JsonObject fieldsObject(Map<?, ?> values) {
        JsonObject fields = new JsonObject();
        for (Object key : new TreeSet<>(values.keySet())) {
            fields.add(String.valueOf(key), normalize(values.get(key)));
        }
        return fields;
    }

    private static JsonValue supportBeanRow(SupportBean bean) {
        Map<String, Object> props = new HashMap<>();
        props.put("theString", bean.getTheString());
        props.put("intPrimitive", bean.getIntPrimitive());
        JsonObject row = new JsonObject();
        row.add("kind", "row");
        row.add("fields", fieldsObject(props));
        return row;
    }

    private static JsonValue eventRow(EventBean event) {
        Object underlying = event.getUnderlying();
        if (underlying instanceof SupportBean) {
            return supportBeanRow((SupportBean) underlying);
        }
        if (underlying instanceof Map) {
            JsonObject row = new JsonObject();
            row.add("kind", "row");
            row.add("fields", fieldsObject((Map<?, ?>) underlying));
            return row;
        }
        throw new IllegalStateException("unsupported event column underlying type: "
            + (underlying == null ? "null" : underlying.getClass().getName()));
    }

    private static int compareRendered(JsonValue left, JsonValue right) {
        if (left.isNumber() && right.isNumber()) {
            return Double.compare(left.asDouble(), right.asDouble());
        }
        return left.toString().compareTo(right.toString());
    }
}
