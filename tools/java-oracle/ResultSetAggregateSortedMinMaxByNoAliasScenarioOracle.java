import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;

/**
 * Direct Esper 9.0.0 oracle for ResultSetAggregateSortedMinMaxBy ordinal 3
 * (ResultSetAggregateNoAlias). The execution compiles and deploys one
 * statement whose five unaliased projections auto-name their output columns;
 * the differential replay records only the deployment acknowledgment and the
 * ordered property names/types, never any events.
 */
public final class ResultSetAggregateSortedMinMaxByNoAliasScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "resultset-aggregate-sorted-minmax-by-no-alias";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java";
    private static final String RUNTIME_ID = "java-runtime-bb6969a66cad8464ae18";
    private static final String STATIC_ID = "java-553516b9d01c12a13172";
    private static final String EXECUTION = "ResultSetAggregateNoAlias";
    private static final String CASE = "no-alias";
    private static final String EPL = "@name('s0') select maxby(intPrimitive).theString, minby(intPrimitive),"
            + "maxbyever(intPrimitive).theString, minbyever(intPrimitive),"
            + "sorted(intPrimitive asc, theString desc) from SupportBean#time(10)";

    private ResultSetAggregateSortedMinMaxByNoAliasScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: ResultSetAggregateSortedMinMaxByNoAliasScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be an object");
        }
        validateScenario(parsed.asObject());

        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        Map<String, Object> beanType = new HashMap<>();
        beanType.put("theString", String.class);
        beanType.put("intPrimitive", Integer.class);
        configuration.getCommon().addEventType("SupportBean", beanType);

        String runtimeURI = "parity-" + ID;
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeURI, configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(EPL,
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled,
                    new DeploymentOptions().setDeploymentId(runtimeURI));
            EPStatement statement = findStatement(deployment);

            JsonArray records = new JsonArray();
            records.add(new JsonObject()
                    .add("case", CASE)
                    .add("operation", "deployed")
                    .add("statement", statement.getName())
                    .add("sequence", 0));

            EventPropertyDescriptor[] descriptors = statement.getEventType().getPropertyDescriptors();
            String[] names = {
                    "maxby(intPrimitive).theString",
                    "minby(intPrimitive)",
                    "maxbyever(intPrimitive).theString",
                    "minbyever(intPrimitive)",
                    "sorted(intPrimitive,theString desc)",
            };
            // Only the String-typed columns are differential. Java renders
            // whole-event minby/minbyever outputs and sorted() rows as
            // java.util.Map (map-underlying render) while the Go typed API
            // keeps the SupportBean event identity; those three columns are
            // registered as representation differences in the manifest.
            String[] tokens = {"String", "Map", "String", "Map", "Map[]"};
            boolean[] differential = {true, false, true, false, false};
            if (descriptors.length != names.length) {
                throw new IllegalStateException("expected five property descriptors, got " + descriptors.length);
            }
            JsonArray value = new JsonArray();
            for (int index = 0; index < descriptors.length; index++) {
                if (!names[index].equals(descriptors[index].getPropertyName())) {
                    throw new IllegalStateException("descriptor " + index + " = "
                            + descriptors[index].getPropertyName() + ", want " + names[index]);
                }
                String actual = simpleTypeName(descriptors[index].getPropertyType());
                if (!tokens[index].equals(actual)) {
                    throw new IllegalStateException("descriptor " + names[index] + " type "
                            + actual + ", want " + tokens[index]);
                }
                if (differential[index]) {
                    value.add(new JsonObject().add("name", names[index]).add("type", actual));
                }
            }
            records.add(new JsonObject()
                    .add("case", CASE)
                    .add("operation", "types")
                    .add("statement", statement.getName())
                    .add("sequence", 0)
                    .add("value", value));

            if (records.size() != 2) {
                throw new IllegalStateException("expected two trace records, got " + records.size());
            }
            System.out.println(new JsonObject()
                    .add("version", VERSION)
                    .add("id", ID)
                    .add("javaCommit", JAVA_COMMIT)
                    .add("javaSource", JAVA_SOURCE)
                    .add("javaRuntimes", new JsonArray().add(RUNTIME_ID))
                    .add("javaNames", new JsonArray().add(EXECUTION))
                    .add("javaStaticIds", new JsonArray().add(STATIC_ID))
                    .add("javaFlags", new JsonArray())
                    .add("records", records));
        } finally {
            runtime.destroy();
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(requireString(scenario, "version"))
                || !ID.equals(requireString(scenario, "id"))
                || !JAVA_COMMIT.equals(requireString(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(requireString(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        JsonArray runtimes = scenario.get("javaRuntimes").asArray();
        if (runtimes.size() != 1 || !RUNTIME_ID.equals(runtimes.get(0).asString())) {
            throw new IllegalArgumentException("javaRuntimes is not pinned");
        }
        JsonArray names = scenario.get("javaNames").asArray();
        if (names.size() != 1 || !EXECUTION.equals(names.get(0).asString())) {
            throw new IllegalArgumentException("javaNames is not pinned");
        }
        JsonArray staticIds = scenario.get("javaStaticIds").asArray();
        if (staticIds.size() != 1 || !STATIC_ID.equals(staticIds.get(0).asString())) {
            throw new IllegalArgumentException("javaStaticIds is not pinned");
        }
        if (!scenario.get("javaFlags").asArray().isEmpty()) {
            throw new IllegalArgumentException("javaFlags must be empty");
        }

        JsonArray cases = scenario.get("cases").asArray();
        if (cases.size() != 1) {
            throw new IllegalArgumentException("scenario must contain exactly one case");
        }
        JsonObject entry = cases.get(0).asObject();
        requireFields(entry, "case", "ordinal", "runtimeId", "executionName", "observation",
                "iteratorSnapshots", "epl");
        if (!CASE.equals(requireString(entry, "case"))
                || entry.getInt("ordinal", -1) != 3
                || !RUNTIME_ID.equals(requireString(entry, "runtimeId"))
                || !EXECUTION.equals(requireString(entry, "executionName"))
                || !"statement-metadata".equals(requireString(entry, "observation"))
                || entry.getInt("iteratorSnapshots", -1) != 0
                || !EPL.equals(requireString(entry, "epl"))) {
            throw new IllegalArgumentException("scenario case metadata is not pinned");
        }

        JsonArray steps = scenario.get("steps").asArray();
        if (steps.size() != 3) {
            throw new IllegalArgumentException("scenario must contain exactly three steps");
        }
        JsonObject marker = steps.get(0).asObject();
        requireFields(marker, "op", "case");
        if (!"case".equals(requireString(marker, "op")) || !CASE.equals(requireString(marker, "case"))) {
            throw new IllegalArgumentException("scenario must start with the no-alias case marker");
        }
        JsonObject deployed = steps.get(1).asObject();
        requireFields(deployed, "op", "statement");
        if (!"deployed".equals(requireString(deployed, "op"))
                || !"s0".equals(requireString(deployed, "statement"))) {
            throw new IllegalArgumentException("scenario must acknowledge s0 deployment");
        }
        JsonObject types = steps.get(2).asObject();
        requireFields(types, "op", "statement");
        if (!"types".equals(requireString(types, "op"))
                || !"s0".equals(requireString(types, "statement"))) {
            throw new IllegalArgumentException("scenario must read s0 types");
        }
    }

    private static String simpleTypeName(Class<?> type) {
        if (type == String.class) {
            return "String";
        }
        if (type == Integer.class || type == int.class) {
            return "Integer";
        }
        if (type == Long.class || type == long.class) {
            return "Long";
        }
        if (type == java.util.Map.class) {
            return "Map";
        }
        if (type.isArray()) {
            return simpleTypeName(type.getComponentType()) + "[]";
        }
        String simple = type.getSimpleName();
        if (simple.isEmpty()) {
            throw new IllegalStateException("anonymous property type " + type);
        }
        return simple;
    }

    private static EPStatement findStatement(EPDeployment deployment) {
        for (EPStatement statement : deployment.getStatements()) {
            if ("s0".equals(statement.getName())) {
                return statement;
            }
        }
        throw new IllegalStateException("statement s0 was not deployed");
    }

    private static void requireFields(JsonObject object, String... names) {
        for (String name : object.names()) {
            boolean known = false;
            for (String candidate : names) {
                if (candidate.equals(name)) {
                    known = true;
                    break;
                }
            }
            if (!known) {
                throw new IllegalArgumentException("unexpected field " + name);
            }
        }
        for (String name : names) {
            if (object.get(name) == null) {
                throw new IllegalArgumentException("missing field " + name);
            }
        }
    }

    private static String requireString(JsonObject object, String name) {
        String value = object.getString(name, null);
        if (value == null) {
            throw new IllegalArgumentException("field " + name + " must be a string");
        }
        return value;
    }
}
