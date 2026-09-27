import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.EventPropertyDescriptor;
import com.espertech.esper.common.client.EventType;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.type.EPType;
import com.espertech.esper.common.internal.util.SerializableObjectCopier;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.support.bean.SupportSpatialPoint;
import com.espertech.esper.regressionlib.support.events.SupportGenericColUtil;
import com.espertech.esper.regressionlib.support.events.SupportGenericColUtil.PairOfNameAndType;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;

/**
 * Java oracle for the create-DDL bundle: InfraNWTableCreate ordinals 0-1
 * (InfraCreateGenericColType {namedWindow=true/false}) plus
 * InfraNWTableCreateIndexAdvancedSyntax ordinal 0. Three executions on
 * three runtimes.
 *
 * generic-col-window/generic-col-table (InfraNWTableCreate.java lines
 * 40-58): compiles and deploys ONE module per execution —
 * `@public @buseventtype create schema MyInputEvent(<namesAndTypes>);`
 * plus `@name('infra')create window MyInfra#keepall as (<namesAndTypes>);`
 * or `@name('infra')create table MyInfra as (<namesAndTypes>);` and
 * `on MyInputEvent merge MyInfra insert select <names>;` — then asserts
 * the infra statement's EPType descriptors
 * (SupportGenericColUtil.assertPropertyEPTypes), sends the sample map
 * event (sendEventMap — the schema is bus-visible), runs milestone(0)
 * (harness no-op carrying no step) and iterators the single merged row
 * before undeployAll.
 *
 * index-syntax (InfraNWTableCreateIndexAdvancedSyntax.java lines 21-44):
 * runs the four SODA assertCompileSODA eplToModel round-trips first (they
 * are syntax-only and precede the window deploy in the source), then
 * deploys `@public create window MyWindow#keepall as SupportSpatialPoint`
 * and runs the five tryInvalidCompile probes against the accumulated
 * path. All nine probes ride unrepresentable records — the Go surface
 * carries no statement object model or EPL-text compiler, so SODA
 * records pin the asserted EPL text and invalid records pin the Java
 * compile-error message plus the typed create-index equivalent the Go
 * runner exercises.
 */
public final class InfraNWTableCreateDDL563ScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-nwtable-create-ddl-563";
    private static final String DESCRIPTION =
            "InfraNWTableCreate ordinals 0-1 plus "
                    + "InfraNWTableCreateIndexAdvancedSyntax ordinal 0: "
                    + "InfraCreateGenericColType {namedWindow=true/false} deploys "
                    + "one @public @buseventtype create-map-schema MyInputEvent "
                    + "module (8 parameterized columns from SupportGenericColUtil: "
                    + "java.util.List<String>, java.util.List<Optional<Integer>>, "
                    + "java.util.Map<String,Integer>, java.util.List<String>[], "
                    + "java.util.List<String[]>, java.util.List<String>[][], "
                    + "java.util.List<String[][]>, java.util.List<T>) including "
                    + "@name('infra') create-window MyInfra#keepall or "
                    + "create-table MyInfra over the same columns and 'on "
                    + "MyInputEvent merge MyInfra insert select <8 names>', "
                    + "asserts the infra statement's EPType descriptors, sends "
                    + "the SupportGenericColUtil sample map event, runs "
                    + "milestone(0) (a harness no-op carrying no step) and "
                    + "iterators the single merged row; "
                    + "InfraNWTableCreateIndexAdvancedSyntax runs four SODA "
                    + "eplToModel round-trips, deploys an @public "
                    + "SupportSpatialPoint(id,px,py,category) keepall MyWindow "
                    + "and replays five invalid create-index compile probes as "
                    + "unrepresentable records (the Go surface carries no "
                    + "statement-object model or EPL-text compiler; the typed "
                    + "create-index validation equivalents are exercised live "
                    + "where a Go form exists).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    // The scenario covers both suite files in this directory; the per-file
    // list rides javaSourceFiles.
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable";
    private static final String[] JAVA_SOURCE_FILES = {
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreate.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndexAdvancedSyntax.java",
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/events/SupportGenericColUtil.java"
    };

    private static final String[] RUNTIME_IDS = {
            "java-runtime-b7fab192ff4a0d2ff2ce",
            "java-runtime-48d80d017090dd3410b7",
            "java-runtime-bc1a897eca64b5da6df3"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraCreateGenericColType{namedWindow=true}",
            "InfraCreateGenericColType{namedWindow=false}",
            "InfraNWTableCreateIndexAdvancedSyntax"
    };
    private static final String[] STATIC_IDS = {
            "java-621032f62ef7cf1bc193",
            "java-621032f62ef7cf1bc193",
            "java-b6b074549bedc2ab1767"
    };
    private static final String[] JAVA_FLAGS = {"SERDEREQUIRED"};
    private static final String[] CASES = {
            "generic-col-window",
            "generic-col-table",
            "index-syntax"
    };
    private static final int[] ORDINALS = {0, 1, 0};
    private static final String[] CASE_OBSERVATIONS = {
            "deploy+types+send+snapshot+undeploy; a single module deploy "
                    + "carrying @public @buseventtype create-map-schema "
                    + "MyInputEvent (8 SupportGenericColUtil parameterized "
                    + "columns), @name('infra') keepall MyInfra named window "
                    + "and 'on MyInputEvent merge MyInfra insert select <8 "
                    + "names>': the types record pins the eight declared "
                    + "java.util.-qualified type tokens, the sample map send "
                    + "inserts one row and the iterator snapshot returns all "
                    + "eight columns (Optional/parameterized generic metadata "
                    + "has no Go schema surface; the Go schema pins "
                    + "element/container reflect types, so EPType generic-arg "
                    + "fidelity is documented in the record note)",
            "deploy+types+send+snapshot+undeploy; a single module deploy "
                    + "carrying the same @public @buseventtype schema plus an "
                    + "unkeyed MyInfra table over the 8 parameterized columns "
                    + "fed by the merge insert: the types record pins the "
                    + "declared type tokens, the sample map send inserts one "
                    + "row and the iterator snapshot returns all eight columns "
                    + "(same Optional/parameterized downgrade note as the "
                    + "window leg)",
            "unrepresentable+deploy+unrepresentable; four SODA eplToModel "
                    + "round-trips run before the @public SupportSpatialPoint"
                    + "(id,px,py,category) keepall MyWindow deploy (matching "
                    + "the Java source order - the round-trips are "
                    + "syntax-only), then five invalid create-index compile "
                    + "probes (empty expression list, expression column, "
                    + "multi-expression list, dotted column, unknown advanced "
                    + "type) run against the deployed path, their Java "
                    + "messages pinned; the Go runner exercises the typed "
                    + "create-index validation equivalents live where a form "
                    + "exists (empty columns, unknown-column, invalid kind) "
                    + "and records the EPL-text-only probes without a Go "
                    + "counterpart"
    };

    // Verbatim module text of InfraNWTableCreate.java lines 40-47 (ords
    // 0-1, InfraCreateGenericColType.run): the Java execution concatenates
    // the three statements with ';\n' separators and passes the module to
    // a single env.compileDeploy. allNamesAndTypes() emits comma-joined
    // java.util.-qualified type tokens; allNames() is comma-joined.
    private static final String NAMES_AND_TYPES =
            "listOfString java.util.List<String>,"
                    + "listOfOptionalInteger java.util.List<Optional<Integer>>,"
                    + "mapOfStringAndInteger java.util.Map<String, Integer>,"
                    + "listArrayOfString java.util.List<String>[],"
                    + "listOfStringArray java.util.List<String[]>,"
                    + "listArray2DimOfString java.util.List<String>[][],"
                    + "listOfStringArray2Dim java.util.List<String[][]>,"
                    + "listOfT java.util.List<Object>";
    private static final String ALL_NAMES =
            "listOfString,listOfOptionalInteger,mapOfStringAndInteger,"
                    + "listArrayOfString,listOfStringArray,"
                    + "listArray2DimOfString,listOfStringArray2Dim,listOfT";
    private static final String EPL_MODULE_WINDOW =
            "@public @buseventtype create schema MyInputEvent(" + NAMES_AND_TYPES
                    + ");\n@name('infra')create window MyInfra#keepall as ("
                    + NAMES_AND_TYPES + ");\non MyInputEvent merge MyInfra "
                    + "insert select " + ALL_NAMES + ";\n";
    private static final String EPL_MODULE_TABLE =
            "@public @buseventtype create schema MyInputEvent(" + NAMES_AND_TYPES
                    + ");\n@name('infra')create table MyInfra as ("
                    + NAMES_AND_TYPES + ");\non MyInputEvent merge MyInfra "
                    + "insert select " + ALL_NAMES + ";\n";

    // Verbatim literals of InfraNWTableCreateIndexAdvancedSyntax.java:
    // lines 22-25 (four assertCompileSODA eplToModel round-trips, run
    // before the window deploy), line 28 (the @public window deploy) and
    // lines 30-43 (five tryInvalidCompile probes).
    private static final String EPL_CREATE_WINDOW_ADV =
            "@public create window MyWindow#keepall as SupportSpatialPoint";
    private static final String[] SODA_LABELS = {
            "soda-index-advanced-args",
            "soda-index-single-named-type",
            "soda-index-multi-col-named-type",
            "soda-index-mixed-columns"
    };
    private static final String[] SODA_EPLS = {
            "create index MyIndex on MyWindow((x,y) dummy_name(\"a\",10101))",
            "create index MyIndex on MyWindow(x dummy_name)",
            "create index MyIndex on MyWindow((x,y,z) dummy_name)",
            "create index MyIndex on MyWindow(x dummy_name, (y,z) "
                    + "dummy_name_2(\"a\"), p dummyname3)"
    };
    private static final String[] INVALID_LABELS = {
            "invalid-empty-expr-list",
            "invalid-expression",
            "invalid-multi-expr-list",
            "invalid-dotted-expr",
            "invalid-advanced-type"
    };
    private static final String[] INVALID_EPLS = {
            "create index MyIndex on MyWindow(())",
            "create index MyIndex on MyWindow(intPrimitive+1)",
            "create index MyIndex on MyWindow((x, y))",
            "create index MyIndex on MyWindow(x.y)",
            "create index MyIndex on MyWindow(id xxxx)"
    };
    // The Java-only compile-error messages asserted by tryInvalidCompile;
    // each scenario note appends the Go mapping note after ' - '.
    private static final String[] INVALID_JAVA_MESSAGES = {
            "Invalid empty list of index expressions",
            "Invalid index expression 'intPrimitive+1'",
            "Invalid multiple index expressions",
            "Invalid index expression 'x.y'",
            "Unrecognized advanced-type index 'xxxx'"
    };

    private static final String[] SNAPSHOT_FIELDS = {
            "listArray2DimOfString",
            "listArrayOfString",
            "listOfOptionalInteger",
            "listOfString",
            "listOfStringArray",
            "listOfStringArray2Dim",
            "listOfT",
            "mapOfStringAndInteger"
    };

    private static final int EXPECTED_STEPS = 27;
    private static final int EXPECTED_RECORDS = 16;

    private InfraNWTableCreateDDL563ScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNWTableCreateDDL563ScenarioOracle <scenario.json>");
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
     * its own runtime). The AdvancedSyntax case preconfigures
     * SupportSpatialPoint; the internal timer is disabled and the
     * rethrowing exception handler surfaces statement failures to the
     * sender thread. Deployed modules accumulate on the path exactly like
     * env.compileDeploy(epl, path) so later compiles resolve the @public
     * infra. The types step runs
     * SupportGenericColUtil.assertPropertyEPTypes against the deployed
     * infra statement's event type and emits the pinned declared type
     * token plus the observed indexed/mapped flags per column in declared
     * order; the snapshot step iterators the infra statement (window and
     * table create statements are both iterable); unrepresentable steps
     * run the SODA eplToModel round-trip or the tryInvalidCompile probe
     * and emit the pinned note.
     */
    private static void runCase(String caseName, JsonArray allSteps, JsonArray records)
            throws Exception {
        Configuration configuration = new Configuration();
        boolean indexSyntax = "index-syntax".equals(caseName);
        if (indexSyntax) {
            configuration.getCommon().addEventType(SupportSpatialPoint.class);
        }
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                com.espertech.esper.common.client.util.UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statements = new HashMap<>();
        Map<String, EPDeployment> deployments = new HashMap<>();
        // RegressionPath equivalent: every deployed module joins the path so
        // later compiles (the invalid probes) resolve the @public window.
        List<EPCompiled> path = new ArrayList<>();
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
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        EPDeployment deployment = compileDeploy(runtime, configuration, epl, path);
                        deployments.put(label, deployment);
                        for (EPStatement statement : deployment.getStatements()) {
                            statements.put(statement.getName(), statement);
                        }
                        break;
                    }
                    case "deployed": {
                        String label = string(step, "statement");
                        if (!deployments.containsKey(label) && !statements.containsKey(label)) {
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
                    case "types": {
                        // assertStatement("infra", ...assertPropertyEPTypes):
                        // assert the EPType descriptors of the infra
                        // statement's event type, then emit the pinned
                        // declared type token plus the observed
                        // indexed/mapped flags per column in declared order.
                        String label = string(step, "statement");
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "types targets unknown statement " + label);
                        }
                        EventType eventType = statement.getEventType();
                        SupportGenericColUtil.assertPropertyEPTypes(eventType);
                        Map<String, EventPropertyDescriptor> byName = new HashMap<>();
                        for (EventPropertyDescriptor descriptor
                                : eventType.getPropertyDescriptors()) {
                            byName.put(descriptor.getPropertyName(), descriptor);
                        }
                        JsonArray entries = new JsonArray();
                        for (PairOfNameAndType pair : SupportGenericColUtil.NAMESANDTYPES) {
                            EventPropertyDescriptor descriptor = byName.get(pair.getName());
                            if (descriptor == null) {
                                throw new IllegalStateException(
                                        "missing property descriptor " + pair.getName());
                            }
                            EPType eptype = descriptor.getPropertyEPType();
                            if (eptype == null) {
                                throw new IllegalStateException(
                                        "null EPType for " + pair.getName());
                            }
                            JsonObject entry = new JsonObject();
                            entry.add("name", pair.getName());
                            entry.add("type", pair.getType());
                            entry.add("indexed", descriptor.isIndexed());
                            entry.add("mapped", descriptor.isMapped());
                            entries.add(entry);
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "types");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("value", entries);
                        records.add(record);
                        break;
                    }
                    case "send":
                        sendEventMap(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "snapshot": {
                        // env.iterator("infra"): iterate the deployed infra
                        // statement (the create-window/create-table
                        // statement's event type carries the columns) and
                        // assert the single merged row like
                        // SupportGenericColUtil.compare does.
                        String label = string(step, "statement");
                        EPStatement statement = statements.get(label);
                        if (statement == null) {
                            throw new IllegalStateException(
                                    "snapshot targets unknown statement " + label);
                        }
                        String[] fields = stringArray(step.get("fields"), "fields");
                        if (!"unordered".equals(string(step, "mode"))) {
                            throw new IllegalStateException(
                                    "snapshot mode is not unordered for " + label);
                        }
                        Iterator<EventBean> iterator = statement.iterator();
                        if (!iterator.hasNext()) {
                            throw new IllegalStateException("iterator has no rows");
                        }
                        EventBean event = iterator.next();
                        if (iterator.hasNext()) {
                            throw new IllegalStateException("iterator has more than one row");
                        }
                        SupportGenericColUtil.compare(event);
                        JsonArray rows = new JsonArray();
                        rows.add(projectedRow(event, fields));
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "snapshot");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("time", Instant.ofEpochMilli(
                                runtime.getEventService().getCurrentTime()).toString());
                        record.add("new", rows);
                        records.add(record);
                        break;
                    }
                    case "unrepresentable": {
                        String label = string(step, "statement");
                        String epl = string(step, "epl");
                        String note = string(step, "expectError");
                        int sodaIndex = indexOf(SODA_LABELS, label);
                        if (sodaIndex >= 0) {
                            if (!SODA_EPLS[sodaIndex].equals(epl)) {
                                throw new IllegalStateException("unrepresentable step "
                                        + label + " does not pin the probe EPL");
                            }
                            // env.eplToModel: parse the pinned text into a
                            // statement object model, serialization-copy it
                            // and assert toEPL() reproduces the text.
                            EPStatementObjectModel model = SerializableObjectCopier
                                    .copyMayFail(EPCompilerProvider.getCompiler()
                                            .eplToModel(epl, configuration));
                            if (model == null || !epl.equals(model.toEPL())) {
                                throw new IllegalStateException(
                                        "eplToModel round-trip failed for " + label);
                            }
                            if (!note.equals(sodaNote(epl))) {
                                throw new IllegalStateException(
                                        "unrepresentable note is not pinned for " + label);
                            }
                        } else {
                            int invalidIndex = indexOf(INVALID_LABELS, label);
                            if (invalidIndex < 0) {
                                throw new IllegalStateException(
                                        "unknown unrepresentable label " + label);
                            }
                            if (!INVALID_EPLS[invalidIndex].equals(epl)) {
                                throw new IllegalStateException("unrepresentable step "
                                        + label + " does not pin the probe EPL");
                            }
                            // tryInvalidCompile(path, epl, message): compile
                            // the pinned probe against the path and assert
                            // the failure message the Java execution pins.
                            String message = INVALID_JAVA_MESSAGES[invalidIndex];
                            tryInvalidCompile(configuration, path, epl, message);
                            if (!note.startsWith(message + " - ")) {
                                throw new IllegalStateException(
                                        "unrepresentable note does not carry the pinned "
                                                + "message for " + label);
                            }
                        }
                        JsonObject record = new JsonObject();
                        record.add("case", caseName);
                        record.add("operation", "unrepresentable");
                        record.add("statement", label);
                        record.add("sequence", 0);
                        record.add("value", note);
                        records.add(record);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        statements.clear();
                        deployments.clear();
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

    private static int indexOf(String[] labels, String label) {
        for (int index = 0; index < labels.length; index++) {
            if (labels[index].equals(label)) {
                return index;
            }
        }
        return -1;
    }

    /** The pinned note a SODA record carries: the asserted EPL text plus
     * the fixed suffix documenting the round-trip and its Go mapping. */
    private static String sodaNote(String epl) {
        switch (epl) {
            case "create index MyIndex on MyWindow((x,y) dummy_name(\"a\",10101))":
                return "soda-eplToModel: " + epl
                        + " - Java eplToModel parses the statement text into a "
                        + "statement object model and assertEquals(epl, "
                        + "model.toEPL()) round-trips it; the Go surface has no "
                        + "EPL-text parser or statement object model, so the "
                        + "create-index syntax round-trip is pinned text with no "
                        + "Go counterpart";
            case "create index MyIndex on MyWindow(x dummy_name)":
                return "soda-eplToModel: " + epl
                        + " - same SODA round-trip; single-expression "
                        + "advanced-type indexes have no typed Go create-index form";
            case "create index MyIndex on MyWindow((x,y,z) dummy_name)":
                return "soda-eplToModel: " + epl
                        + " - same SODA round-trip; multi-expression "
                        + "advanced-type lists have no typed Go create-index form";
            default:
                return "soda-eplToModel: " + epl
                        + " - same SODA round-trip; mixed column-list/named-type "
                        + "index forms have no Go equivalent";
        }
    }

    /** compileDeploy mirrors env.compileDeploy(epl, path): module compile
     * with the configuration and accumulated path followed by a
     * deployment; the compiled module joins the path. */
    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration,
                                              String epl, List<EPCompiled> path)
            throws EPCompileException, EPDeployException {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().getCompileds().addAll(path);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        path.add(compiled);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }

    /** tryInvalidCompile mirrors the regression-suite helper: compile the
     * probe with the accumulated path and assert the failure message
     * (SupportMessageAssertUtil.assertMessage: startsWith when the
     * expected text exceeds 10 chars, equals otherwise). */
    private static void tryInvalidCompile(Configuration configuration, List<EPCompiled> path,
                                          String epl, String expected) {
        String message;
        try {
            CompilerArguments arguments = new CompilerArguments(configuration);
            arguments.getPath().getCompileds().addAll(path);
            EPCompilerProvider.getCompiler().compile(epl, arguments);
            throw new IllegalStateException("probe unexpectedly compiled: " + epl);
        } catch (EPCompileException ex) {
            message = ex.getMessage();
        }
        boolean matches = expected.length() > 10
                ? message.startsWith(expected)
                : message.equals(expected);
        if (!matches) {
            throw new IllegalStateException("compile message mismatch: expected <"
                    + expected + "> got <" + message + ">");
        }
    }

    /** Row projected to exactly the fields the step's assertions read. */
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
     * Normalization mirroring the compat diffing canonicalization: null as
     * the tagged {"state":"null"} object, Optional unwrapped, maps as JSON
     * objects, collections and arrays as JSON arrays, integral numbers as
     * JSON integers, other numbers as doubles, booleans and strings
     * passthrough, everything else rendered as a string.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Optional) {
            Optional<?> optional = (Optional<?>) value;
            return optional.isPresent() ? normalize(optional.get()) : normalize(null);
        }
        if (value instanceof Map) {
            JsonObject object = new JsonObject();
            for (Map.Entry<?, ?> entry : ((Map<?, ?>) value).entrySet()) {
                object.add(String.valueOf(entry.getKey()), normalize(entry.getValue()));
            }
            return object;
        }
        if (value instanceof Collection) {
            JsonArray array = new JsonArray();
            for (Object item : (Collection<?>) value) {
                array.add(normalize(item));
            }
            return array;
        }
        if (value instanceof Object[]) {
            JsonArray array = new JsonArray();
            for (Object item : (Object[]) value) {
                array.add(normalize(item));
            }
            return array;
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
     * Sends the SupportGenericColUtil.getSampleEvent map: the scenario
     * payload pins every column value; the oracle validates the pinned
     * shapes and rebuilds the Java-typed sample (Optional elements, list
     * arrays, map) exactly like the regression send.
     */
    private static void sendEventMap(EPRuntime runtime, String type, JsonObject payload) {
        if (!"MyInputEvent".equals(type)) {
            throw new IllegalArgumentException("unknown event type: " + type);
        }
        requireFields(payload, "listOfString", "listOfOptionalInteger",
                "mapOfStringAndInteger", "listArrayOfString", "listOfStringArray",
                "listArray2DimOfString", "listOfStringArray2Dim", "listOfT");
        requireJsonEquals(payload.get("listOfString"), "[\"a\"]", "listOfString");
        requireJsonEquals(payload.get("listOfOptionalInteger"), "[10]",
                "listOfOptionalInteger");
        requireJsonEquals(payload.get("mapOfStringAndInteger"), "{\"k\":20}",
                "mapOfStringAndInteger");
        requireJsonEquals(payload.get("listArrayOfString"), "[[\"b\"]]",
                "listArrayOfString");
        requireJsonEquals(payload.get("listOfStringArray"), "[[\"c\"]]",
                "listOfStringArray");
        requireJsonEquals(payload.get("listArray2DimOfString"), "[[[\"b\"]]]",
                "listArray2DimOfString");
        requireJsonEquals(payload.get("listOfStringArray2Dim"), "[[[\"c\"]]]",
                "listOfStringArray2Dim");
        requireJsonEquals(payload.get("listOfT"), "[\"x\"]", "listOfT");

        Map<String, Object> event = new HashMap<>();
        event.put("listOfString", Arrays.asList("a"));
        event.put("listOfOptionalInteger", Arrays.asList(Optional.of(10)));
        event.put("mapOfStringAndInteger", singletonMapOf("k", 20));
        event.put("listArrayOfString", new List[]{Arrays.asList("b")});
        List<String[]> listOfStringArray = new ArrayList<>();
        listOfStringArray.add(new String[]{"c"});
        event.put("listOfStringArray", listOfStringArray);
        event.put("listArray2DimOfString", new List[][]{new List[]{Arrays.asList("b")}});
        List<String[][]> listOfStringArray2Dim = new ArrayList<>();
        listOfStringArray2Dim.add(new String[][]{new String[]{"c"}});
        event.put("listOfStringArray2Dim", listOfStringArray2Dim);
        event.put("listOfT", new ArrayList<>(Arrays.asList("x")));
        runtime.getEventService().sendEventMap(event, type);
    }

    private static Map<String, Object> singletonMapOf(String key, Object value) {
        Map<String, Object> map = new HashMap<>();
        map.put(key, value);
        return map;
    }

    /** Order-independent JSON equality against a pinned literal. */
    private static void requireJsonEquals(JsonValue value, String expected, String label) {
        JsonValue parsed = Json.parse(expected);
        if (!jsonEquals(parsed, value)) {
            throw new IllegalArgumentException(label + " payload is not pinned: " + value);
        }
    }

    private static boolean jsonEquals(JsonValue left, JsonValue right) {
        if (left == null || right == null) {
            return left == right;
        }
        if (left.isObject() && right.isObject()) {
            JsonObject a = left.asObject();
            JsonObject b = right.asObject();
            if (a.size() != b.size()) {
                return false;
            }
            for (Member member : a) {
                if (!jsonEquals(member.getValue(), b.get(member.getName()))) {
                    return false;
                }
            }
            return true;
        }
        if (left.isArray() && right.isArray()) {
            JsonArray a = left.asArray();
            JsonArray b = right.asArray();
            if (a.size() != b.size()) {
                return false;
            }
            for (int index = 0; index < a.size(); index++) {
                if (!jsonEquals(a.get(index), b.get(index))) {
                    return false;
                }
            }
            return true;
        }
        return left.toString().equals(right.toString());
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaSourceFiles", "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags",
                "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaSourceFiles"), JAVA_SOURCE_FILES, "javaSourceFiles");
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
            offset = validateCaseSteps(steps, offset, caseName);
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every EPL-bearing
     * step in the case, in step order (the module text for the create
     * legs; the four SODA texts, the window deploy and the five invalid
     * probes for index-syntax). */
    private static String caseEpl(String caseName) {
        switch (caseName) {
            case "generic-col-window":
                return EPL_MODULE_WINDOW;
            case "generic-col-table":
                return EPL_MODULE_TABLE;
            default:
                StringBuilder builder = new StringBuilder();
                String separator = "";
                for (String epl : SODA_EPLS) {
                    builder.append(separator).append(epl);
                    separator = "\n";
                }
                builder.append('\n').append(EPL_CREATE_WINDOW_ADV);
                for (String epl : INVALID_EPLS) {
                    builder.append('\n').append(epl);
                }
                return builder.toString();
        }
    }

    /**
     * Exact step sequence per case. generic-col-* mirrors
     * InfraCreateGenericColType.run (InfraNWTableCreate.java lines 40-58):
     * the single module deploy, the infra EPType assert, the sample map
     * send, the iterator snapshot and undeployAll (milestone(0) carries no
     * step). index-syntax mirrors
     * InfraNWTableCreateIndexAdvancedSyntax.run (lines 21-44) in source
     * order: the four SODA round-trips first, then the @public window
     * deploy, then the five invalid probes (all unrepresentable records),
     * then undeployAll.
     */
    private static int validateCaseSteps(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        if ("index-syntax".equals(caseName)) {
            for (int index = 0; index < SODA_LABELS.length; index++) {
                validateUnrepresentable(steps.get(offset++), caseName, SODA_LABELS[index],
                        SODA_EPLS[index], -1);
            }
            validateDeploy(steps.get(offset++), caseName, "window", EPL_CREATE_WINDOW_ADV);
            validateDeployed(steps.get(offset++), caseName, "window");
            for (int index = 0; index < INVALID_LABELS.length; index++) {
                validateUnrepresentable(steps.get(offset++), caseName, INVALID_LABELS[index],
                        INVALID_EPLS[index], index);
            }
            validateUndeployAll(steps.get(offset++), caseName);
            return offset;
        }
        String module = caseName.endsWith("-window") ? EPL_MODULE_WINDOW : EPL_MODULE_TABLE;
        validateDeploy(steps.get(offset++), caseName, "module", module);
        validateDeployed(steps.get(offset++), caseName, "module");
        validateTypes(steps.get(offset++), caseName, "infra");
        validateSend(steps.get(offset++), caseName);
        validateSnapshot(steps.get(offset++), caseName, "infra");
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

    private static void validateDeploy(JsonValue value, String caseName, String expectedStatement,
                                       String expectedEpl) {
        JsonObject step = object(value, "deploy step");
        requireFields(step, "op", "case", "statement", "epl");
        if (!"deploy".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))) {
            throw new IllegalArgumentException("deploy step is not pinned for " + caseName + "/"
                    + expectedStatement);
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

    /** Pins the types step: op, case and the "infra" statement label. */
    private static void validateTypes(JsonValue value, String caseName,
                                      String expectedStatement) {
        JsonObject step = object(value, "types step");
        requireFields(step, "op", "case", "statement");
        if (!"types".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))) {
            throw new IllegalArgumentException("types step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
    }

    /** Pins the env.iterator("infra") snapshot: unordered mode with the 8
     * sorted column names (map events carry no order). */
    private static void validateSnapshot(JsonValue value, String caseName,
                                         String expectedStatement) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode", "fields");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !"unordered".equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
        }
        validateStringArray(step.get("fields"), SNAPSHOT_FIELDS,
                "snapshot fields for " + caseName);
    }

    /** Pins the sendEventMap(sample) step. */
    private static void validateSend(JsonValue value, String caseName) {
        JsonObject step = object(value, "send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyInputEvent".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("send step is not pinned for " + caseName
                    + "/MyInputEvent");
        }
        JsonObject payload = object(step.get("payload"), "MyInputEvent payload");
        requireFields(payload, "listOfString", "listOfOptionalInteger",
                "mapOfStringAndInteger", "listArrayOfString", "listOfStringArray",
                "listArray2DimOfString", "listOfStringArray2Dim", "listOfT");
        requireJsonEquals(payload.get("listOfString"), "[\"a\"]", "listOfString");
        requireJsonEquals(payload.get("listOfOptionalInteger"), "[10]",
                "listOfOptionalInteger");
        requireJsonEquals(payload.get("mapOfStringAndInteger"), "{\"k\":20}",
                "mapOfStringAndInteger");
        requireJsonEquals(payload.get("listArrayOfString"), "[[\"b\"]]",
                "listArrayOfString");
        requireJsonEquals(payload.get("listOfStringArray"), "[[\"c\"]]",
                "listOfStringArray");
        requireJsonEquals(payload.get("listArray2DimOfString"), "[[[\"b\"]]]",
                "listArray2DimOfString");
        requireJsonEquals(payload.get("listOfStringArray2Dim"), "[[[\"c\"]]]",
                "listOfStringArray2Dim");
        requireJsonEquals(payload.get("listOfT"), "[\"x\"]", "listOfT");
    }

    /** Pins one unrepresentable step: op, case, label, the probe EPL and
     * the record note (SODA text pin or Java message + Go mapping note). */
    private static void validateUnrepresentable(JsonValue value, String caseName,
                                                String expectedStatement, String expectedEpl,
                                                int invalidIndex) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        String note = string(step, "expectError");
        boolean noteOk;
        if (invalidIndex < 0) {
            noteOk = note.equals(sodaNote(expectedEpl));
        } else {
            noteOk = note.startsWith(INVALID_JAVA_MESSAGES[invalidIndex] + " - ");
        }
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !noteOk) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for "
                    + caseName + "/" + expectedStatement);
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
    public static class HarnessRethrowExceptionHandlerFactory
            implements com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory {
        @Override
        public com.espertech.esper.common.client.hook.exception.ExceptionHandler getHandler(
                com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
