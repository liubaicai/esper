import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.common.internal.support.SupportBean;
import com.espertech.esper.common.internal.support.SupportBean_S0;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonVariantStream;
import com.espertech.esper.common.client.module.Module;
import com.espertech.esper.common.client.module.ModuleItem;
import com.espertech.esper.common.client.soda.AnnotationPart;
import com.espertech.esper.common.client.soda.CreateWindowClause;
import com.espertech.esper.common.client.soda.EPStatementObjectModel;
import com.espertech.esper.common.client.soda.Expression;
import com.espertech.esper.common.client.soda.Expressions;
import com.espertech.esper.common.client.soda.SelectClause;
import com.espertech.esper.common.client.soda.View;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportBean_B;
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
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for InfraNamedWindowInsertFrom insert-from-window executions:
 * type-by-window creation after an existing named window with shared insert
 * delivery, seeded keep-all and filtered and unique-index create-window-insert
 * with deploy-time row copying that never reaches create-statement listeners,
 * filtered routing inserts into each window, the staggered map-representation
 * insert-where create chain (ord 2 narrowed to rep=MAP; the object-model
 * toEPL round-trip is verified in-process and pinned as unrepresentable),
 * the invalid create-window-insert compile probes, the variant-stream window
 * fed by insert-into routing, and lenient partial-column inserts over map
 * and object-array schemas.
 *
 * Replays the seven pinned executions (ordinals 0-6 of the suite's
 * executions()) on one runtime with undeployAll between cases, mirroring the
 * regression-suite harness: SupportBean, SupportBean_S0, SupportBean_A and
 * SupportBean_B plus the MyMapAB map type and the VarStream variant from the
 * suite configuration, internal timer disabled, and the rethrowing exception
 * handler so statement failures surface to the sender thread.  The suite
 * compiles each execution as ONE module while this scenario deploys steps as
 * separate modules, so window-referencing creates carry @public (see
 * EPL_CREATE_WINDOW_TWO for the one whitespace-only transcription this
 * forces).  Listeners attach when the deployment containing the statement
 * completes, exactly like the source's compileDeploy(...).addListener(...)
 * chaining: the seeded create-window-insert statements copy rows during
 * their deployment BEFORE the listener attaches, so seed rows never reach
 * the create-statement listeners (the source pins this with
 * assertListenerNotInvoked), while the ord-1 and ord-2 "window" statements
 * attach their listeners before the seed sends and therefore record them.
 * Listener rows project to intPrimitive and theString (sorted; the ord-0
 * selectOne row carries only theString since it selects that one property)
 * or to a and b for the ord-2 map rows, the ord-1 seed snapshots project to
 * theString (the Java iterator asserts are theString-only), the ord-2
 * snapshots project to a,b and a, the ord-4 variant snapshot projects the
 * dynamic id? property, and the lenient snapshots project to c0 and c1.
 * Ordered snapshots emit the engine iterator order the Java exact-order
 * iterator asserts pin; the single mode-any snapshot (windowFour, asserted
 * any-order in Java) emits the canonical marshaled-fields row order.
 */
public final class InfraNamedWindowInsertFromScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "infra-named-window-insert-from";
    private static final String DESCRIPTION =
            "InfraNamedWindowInsertFrom insert-from-window semantics: type-by-window "
                    + "creation after an existing named window with shared insert delivery, seeded "
                    + "keep-all and filtered and unique-index create-window-insert with deploy-time "
                    + "row copying that never reaches create-statement listeners, filtered routing "
                    + "inserts into each window, the staggered map-representation insert-where "
                    + "create chain (ord 2 narrowed to rep=MAP; the object-model toEPL round-trip "
                    + "is unrepresentable), the invalid create-window-insert compile probes, the "
                    + "variant-stream window fed by insert-into routing, and lenient partial-column "
                    + "inserts over map and object-array schemas, captured from window listeners "
                    + "and ordered or canonical mode-any snapshots projected to the Java-asserted "
                    + "fields (Java source "
                    + "regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/infra/namedwindow/InfraNamedWindowInsertFrom.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/"
                    + "InfraNamedWindowInsertFrom.java";
    private static final String[] RUNTIME_IDS = {
            "java-runtime-b3f6cb7b36c5211c8822",
            "java-runtime-e601b3cc7f827d578185",
            "java-runtime-f0f0e5e651a8a513377e",
            "java-runtime-75dcb72bef59bc4cc772",
            "java-runtime-b6e5b14130feae458c23",
            "java-runtime-46011542d6e9d34a87f5",
            "java-runtime-8138dd777290d00417d1"
    };
    private static final String[] EXECUTION_NAMES = {
            "InfraCreateNamedAfterNamed",
            "InfraInsertWhereTypeAndFilter",
            "InfraInsertWhereOMStaggered",
            "InfraInvalid",
            "InfraVariantStream",
            "InfraNamedWindowInsertLenientPropCount{rep=MAP}",
            "InfraNamedWindowInsertLenientPropCount{rep=OBJECTARRAY}"
    };
    private static final String[] STATIC_IDS = {
            "java-b0917219c462cba9d770",
            "java-cff4a3193da2b063a351",
            "java-03b8de29f9f76bb86748",
            "java-10fd9729c661bf429fe4",
            "java-1e55dc9c7e5d907881c5",
            "java-282d7b64878866ea4709",
            "java-282d7b64878866ea4709"
    };

    private static final String CASE_CREATE_AFTER_NAMED = "create-after-named";
    private static final String CASE_INSERT_WHERE_TYPE_FILTER = "insert-where-type-filter";
    private static final String CASE_OM_STAGGERED = "insert-where-om-staggered";
    private static final String CASE_INVALID = "infra-invalid";
    private static final String CASE_VARIANT_STREAM = "variant-stream";
    private static final String CASE_LENIENT_MAP = "lenient-map";
    private static final String CASE_LENIENT_OBJECTARRAY = "lenient-objectarray";



    // Transcriptions of InfraNamedWindowInsertFrom lines 89-92, 109-110,
    // 132, 143, 153, 162-165, and 62-65, without the statement-terminating
    // ";\n".  The suite compiles each execution as one module, while this
    // scenario deploys steps as separate modules, so window-referencing
    // creates add @public for cross-module visibility (same convention as the
    // sibling infra-named-window oracles); visibility has no observable
    // effect.  The ord-1 and lenient sources already carry @public on the
    // referencing statements, so those deploys are verbatim source text.  The
    // case pins keep the Java source text of the execution's first statement.
    private static final String EPL_CREATE_WINDOW_ONE =
            "@name('windowOne') @public create window MyWindow#keepall as SupportBean";
    /**
     * Forced whitespace-only transcription: the source writes
     * "@name('windowTwo')create window MyWindowTwo#keepall as MyWindow"
     * (InfraCreateNamedAfterNamed line 90) with no space after the
     * annotation; inserting the @public deployment-convention annotation
     * requires a separating space.  The normalized text is pinned identically
     * in the scenario and this oracle, and whitespace between annotations has
     * no observable effect.
     */
    private static final String EPL_CREATE_WINDOW_TWO =
            "@name('windowTwo') @public create window MyWindowTwo#keepall as MyWindow";
    private static final String EPL_INSERT =
            "insert into MyWindow select * from SupportBean";
    private static final String EPL_SELECT_ONE =
            "@name('selectOne') select theString from MyWindow";
    private static final String EPL_CREATE_IWT =
            "@name('window') @public create window MyWindowIWT#keepall as SupportBean";
    private static final String EPL_INSERT_IWT =
            "insert into MyWindowIWT select * from SupportBean(intPrimitive > 0)";
    private static final String EPL_CREATE_IWT_TWO =
            "@name('windowTwo') @public create window MyWindowTwo#keepall as MyWindowIWT insert";
    private static final String EPL_CREATE_IWT_THREE =
            "@name('windowThree') @public create window MyWindowThree#keepall"
                    + " as MyWindowIWT insert where theString like 'A%'";
    private static final String EPL_CREATE_IWT_FOUR =
            "@name('windowFour') @public create window MyWindowFour#unique(intPrimitive)"
                    + " as MyWindowIWT insert";
    private static final String EPL_INSERT_IWT_A =
            "insert into MyWindowIWT select * from SupportBean(theString like 'A%')";
    private static final String EPL_INSERT_IWT_B =
            "insert into MyWindowTwo select * from SupportBean(theString like 'B%')";
    private static final String EPL_INSERT_IWT_C =
            "insert into MyWindowThree select * from SupportBean(theString like 'C%')";
    private static final String EPL_INSERT_IWT_D =
            "insert into MyWindowFour select * from SupportBean(theString like 'D%')";
    private static final String EPL_SCHEMA_MAP =
            "@public create MAP schema MyTwoColEvent(c0 string, c1 int)";
    private static final String EPL_SCHEMA_OBJECTARRAY =
            "@public create OBJECTARRAY schema MyTwoColEvent(c0 string, c1 int)";
    private static final String EPL_WINDOW_TWO_COL =
            "@public @name('window') create window MyWindow#keepall as MyTwoColEvent";
    private static final String EPL_INSERT_ONE =
            "insert into MyWindow select theString as c0 from SupportBean";
    private static final String EPL_INSERT_TWO =
            "insert into MyWindow select id as c1 from SupportBean_S0";
    private static final String EPL_CREATE_IWOM =
            "@EventRepresentation('map') @name('window') @public create window MyWindowIWOM#keepall"
                    + " as select a, b from MyMapAB";
    private static final String EPL_INSERT_IWOM =
            "@public insert into MyWindowIWOM select a, b from MyMapAB";
    private static final String EPL_CREATE_IWOM_TWO =
            "@name('windowTwo') @public create window MyWindowIWOMTwo#keepall"
                    + " as select * from MyWindowIWOM insert where b=10";
    private static final String EPL_CREATE_IWOM_THREE =
            "@EventRepresentation('map') @name('windowThree') create window MyWindowIWOMThree#keepall"
                    + " as select a from MyWindowIWOMTwo insert where a = 'E2'";
    private static final String EPL_CREATE_INV =
            "@public create window MyWindowINV#keepall as SupportBean";
    private static final String EPL_CREATE_VS =
            "@public create window MyWindowVS#keepall as select * from VarStream";
    private static final String EPL_CREATE_VS_TWO =
            "@name('window') @public create window MyWindowVSTwo#keepall as MyWindowVS";
    private static final String EPL_INSERT_VS_A =
            "insert into VarStream select * from SupportBean_A";
    private static final String EPL_INSERT_VS_B =
            "insert into VarStream select * from SupportBean_B";
    private static final String EPL_INSERT_VS_TWO =
            "insert into MyWindowVSTwo select * from VarStream";

    // The byte-exact EPL of the five InfraInvalid tryInvalidCompile probes
    // (InfraNamedWindowInsertFrom lines 303-312) and the pinned Java message
    // prefixes the compile-error records carry.
    private static final String PROBE_INSERT =
            "create window testWindow3#keepall as SupportBean insert";
    private static final String PROBE_INSERT_WHERE =
            "create window testWindow3#keepall as select * from SupportBean"
                    + " insert where (intPrimitive = 10)";
    private static final String PROBE_SUBSELECT =
            "create window MyWindowTwo#keepall as MyWindowINV insert where"
                    + " (select intPrimitive from SupportBean#lastevent)";
    private static final String PROBE_AGGREGATION =
            "create window MyWindowTwo#keepall as MyWindowINV insert where sum(intPrimitive) > 2";
    private static final String PROBE_PREV =
            "create window MyWindowTwo#keepall as MyWindowINV insert where prev(1, intPrimitive) = 1";
    private static final String PREFIX_MISSING_WINDOW =
            "A named window by name 'SupportBean' could not be located,"
                    + " the insert-keyword requires an existing named window";
    private static final String PREFIX_SUBSELECT =
            "Create window where-clause may not have a subselect";
    private static final String PREFIX_AGGREGATION =
            "Create window where-clause may not have an aggregation function";
    private static final String PREFIX_PREV =
            "Create window where-clause may not have a function that requires"
                    + " view resources (prior, prev)";

    // OM_NOTE pins the unrepresentable record for the object-model toEPL
    // round-trip asserts at InfraInsertWhereOMStaggered lines 252-263; the
    // oracle verifies the round-trip in-process before emitting the record.
    private static final String OM_NOTE =
            "object-model round-trip: the programmatic create-window insert-where model and"
                    + " eplToModel both render '@public create window MyWindowIWOMTwo#keepall"
                    + " as select * from MyWindowIWOM insert where b=10'; no Go"
                    + " statement-object-model surface";

    // Case pins: the execution's first statement exactly as the Java source
    // writes it, without the @public deployment-convention annotation (the
    // ord-1 and lenient sources already carry @public, so their pins equal
    // their deploys; ord 2 pins the rep=MAP annotation text the narrowed
    // replay deploys).
    private static final String[] SOURCE_EPLS = {
            "@name('windowOne') create window MyWindow#keepall as SupportBean",
            "@name('window') @public create window MyWindowIWT#keepall as SupportBean",
            EPL_CREATE_IWOM,
            EPL_CREATE_INV,
            EPL_CREATE_VS,
            "@public @name('window') create window MyWindow#keepall as MyTwoColEvent",
            "@public @name('window') create window MyWindow#keepall as MyTwoColEvent"
    };

    // Per-case observation strings pinned in the scenario cases[] metadata.
    private static final String[] OBSERVATIONS = {
            "listener",
            "listener",
            "listener+snapshot+unrepresentable; Java loops all EventRepresentationChoice values"
                    + " while the replay covers rep=MAP only (the rep-matrix narrowing): the"
                    + " keepall window over MyMapAB{a,b} receives three map sends through the"
                    + " shared insert, then the b=10-filtered create-window-insert seeds"
                    + " MyWindowIWOMTwo ({E2,10},{E3,10}) and the chained a='E2' create seeds"
                    + " MyWindowIWOMThree ({E2}); the object-model toEPL round-trip assertion is"
                    + " unrepresentable on the Go surface and pins a marker",
            "compile-error; after deploying MyWindowINV five probes pin the Java message prefixes:"
                    + " the two insert-keyword probes against the SupportBean event type compile"
                    + " without the runtime path, while the subselect, aggregation and prev()"
                    + " insert-where probes compile with the path and are unrepresentable on the"
                    + " Go surface (no create-window-where boundary)",
            "snapshot; the variant-schema window pair is fed by insert-into routing from"
                    + " SupportBean_A/SupportBean_B through VarStream (the Java"
                    + " create-window-as-select over VarStream is a continuous feed with identical"
                    + " observable rows); A1 then B1 land in MyWindowVSTwo in insertion order and"
                    + " the iterator projects the dynamic id? property",
            "listener",
            "listener"
    };

    /**
     * Listener discipline: the create-after-named case listens only to
     * windowOne and selectOne (mirroring addListener("selectOne")
     * .addListener("windowOne") at InfraNamedWindowInsertFrom line 93), the
     * insert-where-type-filter case only to window, windowTwo, windowThree,
     * and windowFour (lines 111, 133, 144, and 154), and the
     * insert-where-om-staggered case to window and windowTwo (lines 239 and
     * 265).  The invalid, variant and lenient executions never attach
     * listeners.  Listeners attach when their deployment completes: the
     * seeded create-window-insert deploys copy rows during deployment,
     * before their listeners attach, so those deliveries never reach the
     * listener (the source pins this with assertListenerNotInvoked at lines
     * 135, 146, and 156).
     */
    private static final Map<String, Set<String>> LISTENED_STATEMENTS;

    static {
        Map<String, Set<String>> listened = new HashMap<>();
        listened.put(CASE_CREATE_AFTER_NAMED,
                new HashSet<>(Arrays.asList("selectOne", "windowOne")));
        listened.put(CASE_INSERT_WHERE_TYPE_FILTER,
                new HashSet<>(Arrays.asList("window", "windowTwo", "windowThree", "windowFour")));
        listened.put(CASE_OM_STAGGERED,
                new HashSet<>(Arrays.asList("window", "windowTwo")));
        listened.put(CASE_INVALID, Collections.emptySet());
        listened.put(CASE_VARIANT_STREAM, Collections.emptySet());
        listened.put(CASE_LENIENT_MAP, Collections.emptySet());
        listened.put(CASE_LENIENT_OBJECTARRAY, Collections.emptySet());
        LISTENED_STATEMENTS = Collections.unmodifiableMap(listened);
    }
    /**
     * Row projections: listener rows of the bean cases render intPrimitive
     * and theString sorted (the rows that decide the filters and the unique
     * index; the Java asserts read theString), except the ord-0 selectOne
     * row, whose statement selects only theString so the projection is
     * theString alone; ord-2 listener rows render a and b.  The ord-1 seed
     * snapshots project to theString (the Java iterator asserts are
     * theString-only, lines 134, 145, and 155), the ord-2 snapshots project
     * to a,b and a (lines 266 and 270), the ord-4 variant snapshot projects
     * the dynamic id? property (line 291), and the lenient snapshots project
     * to c0 and c1 (lines 70 and 75).
     */
    private static final String[] FIELDS_BEAN = {"intPrimitive", "theString"};
    private static final String[] FIELDS_STRING = {"theString"};
    private static final String[] FIELDS_TWO_COL = {"c0", "c1"};
    private static final String[] FIELDS_AB = {"a", "b"};
    private static final String[] FIELDS_A = {"a"};
    private static final String[] FIELDS_VARIANT_ID = {"id?"};

    private static final int EXPECTED_RECORDS = 30;
    private static final int EXPECTED_STEPS = 80;

    private InfraNamedWindowInsertFromScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: InfraNamedWindowInsertFromScenarioOracle <scenario.json>");
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
        configuration.getCommon().addEventType(SupportBean_S0.class);
        configuration.getCommon().addEventType(SupportBean_A.class);
        configuration.getCommon().addEventType(SupportBean_B.class);
        Map<String, Object> mapAB = new HashMap<>();
        mapAB.put("a", String.class);
        mapAB.put("b", int.class);
        configuration.getCommon().addEventType("MyMapAB", mapAB);
        ConfigurationCommonVariantStream variant = new ConfigurationCommonVariantStream();
        variant.addEventTypeName("SupportBean_A");
        variant.addEventTypeName("SupportBean_B");
        configuration.getCommon().addVariantStream("VarStream", variant);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-oracle", configuration);
        runtime.getEventService().advanceTime(0);

        JsonArray records = new JsonArray();
        try {
            runCase(CASE_CREATE_AFTER_NAMED, configuration, runtime, allSteps, records);
            runCase(CASE_INSERT_WHERE_TYPE_FILTER, configuration, runtime, allSteps, records);
            runCase(CASE_OM_STAGGERED, configuration, runtime, allSteps, records);
            runCase(CASE_INVALID, configuration, runtime, allSteps, records);
            runCase(CASE_VARIANT_STREAM, configuration, runtime, allSteps, records);
            runCase(CASE_LENIENT_MAP, configuration, runtime, allSteps, records);
            runCase(CASE_LENIENT_OBJECTARRAY, configuration, runtime, allSteps, records);
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
     * Replays one case's steps on the shared runtime; sequences restart per case.
     * Compiles with CompilerArguments(configuration) plus the runtime path (the
     * sibling-oracle convention): the configuration makes the base event types
     * resolvable to the compiler, and the runtime path carries the prior
     * deployments' public types so cross-module named-window and schema
     * references (create-window-as, insert-from, two-column windows) resolve.
     * No plugin function is needed for these executions.
     */
    private static void runCase(String caseName, Configuration configuration, EPRuntime runtime,
                                JsonArray allSteps, JsonArray records) throws Exception {
        Map<String, Integer> sequences = new HashMap<>();
        Map<String, EPStatement> statementsByName = new HashMap<>();
        boolean inCase = false;
        for (JsonValue stepValue : allSteps) {
            JsonObject step = stepValue.asObject();
            if ("case".equals(string(step, "op"))) {
                inCase = caseName.equals(string(step, "case"));
                continue;
            }
            if (!inCase) {
                continue;
            }
            switch (string(step, "op")) {
                case "deploy": {
                    String label = string(step, "statement");
                    CompilerArguments compilerArgs = new CompilerArguments(configuration);
                    compilerArgs.getPath().add(runtime.getRuntimePath());
                    EPCompiled compiled;
                    if (CASE_OM_STAGGERED.equals(caseName) && "windowTwo".equals(label)) {
                        // InfraInsertWhereOMStaggered deploys the windowTwo
                        // create from the object model (lines 262-265); the
                        // round-trip asserts are verified by the
                        // unrepresentable step.
                        EPStatementObjectModel modelTwo = EPCompilerProvider.getCompiler()
                                .eplToModel(string(step, "epl"), configuration);
                        modelTwo.setAnnotations(Arrays.asList(
                                AnnotationPart.nameAnnotation("windowTwo"),
                                new AnnotationPart("public")));
                        Module module = new Module();
                        module.getItems().add(new ModuleItem(modelTwo));
                        module.setModuleText(modelTwo.toEPL());
                        compiled = EPCompilerProvider.getCompiler().compile(module, compilerArgs);
                    } else {
                        compiled = EPCompilerProvider.getCompiler()
                                .compile(string(step, "epl"), compilerArgs);
                    }
                    EPDeployment deployment = runtime.getDeploymentService()
                            .deploy(compiled, new DeploymentOptions());
                    for (EPStatement statement : deployment.getStatements()) {
                        statementsByName.put(statement.getName(), statement);
                        statementsByName.put(label, statement);
                        if (LISTENED_STATEMENTS.getOrDefault(caseName, Collections.emptySet())
                                .contains(statement.getName())) {
                            statement.addListener(
                                    listener(caseName, sequences, records, runtime));
                        }
                    }
                    break;
                }
                case "build-error":
                    buildErrorStep(configuration, runtime, caseName, step, records);
                    break;
                case "unrepresentable":
                    unrepresentableStep(configuration, caseName, step, records);
                    break;
                case "send":
                    sendEvent(runtime, string(step, "eventType"),
                            object(step.get("payload"), "payload"));
                    break;
                case "snapshot": {
                    EPStatement statement = statementsByName.get(string(step, "statement"));
                    if (statement == null) {
                        throw new IllegalStateException("snapshot targets unknown statement in case "
                                + caseName);
                    }
                    JsonValue modeValue = step.get("mode");
                    boolean canonical = modeValue != null && "any".equals(string(step, "mode"));
                    records.add(snapshot(runtime, statement, caseName, canonical));
                    break;
                }
                case "undeploy-all":
                    runtime.getDeploymentService().undeployAll();
                    statementsByName.clear();
                    break;
                default:
                    throw new IllegalStateException("unsupported step op " + string(step, "op"));
            }
        }
        runtime.getDeploymentService().undeployAll();
        statementsByName.clear();
    }

    /**
     * Listener emitting one record per invocation with a per-statement sequence
     * counter; new and old arrays render only when non-empty.  Window insert
     * deliveries are new-only; the projection follows the listened statement
     * (selectOne renders theString alone, the create-window statements render
     * intPrimitive and theString).
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
            JsonArray newRows = rows(newEvents, listenerFields(caseName, statement.getName()));
            JsonArray oldRows = rows(oldEvents, listenerFields(caseName, statement.getName()));
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /**
     * Snapshot of the statement iterator: one record with sequence 0.  Ordered
     * snapshots emit the engine iterator order that the Java exact-order
     * iterator asserts pin (keepall iteration is insertion order); the single
     * mode-any snapshot (windowFour, asserted any-order at InfraNamedWindowInsertFrom
     * line 155) emits the canonical row order — the ascending marshaled-fields
     * string with sorted keys, identical to the Go runner and to
     * compat.CanonicalTrace's mode-any normalization — so the checked-in trace
     * is stable against engine iteration order on both sides.
     */
    private static JsonObject snapshot(EPRuntime runtime, EPStatement statement, String caseName,
                                       boolean canonical) {
        String[] fields = snapshotFields(caseName, statement.getName());
        List<JsonObject> projected = new ArrayList<>();
        for (Iterator<EventBean> iterator = statement.iterator(); iterator.hasNext(); ) {
            projected.add(projectedRow(iterator.next(), fields));
        }
        if (canonical) {
            projected.sort(Comparator.comparing(row -> row.get("fields").asObject().toString()));
        }
        JsonArray rows = new JsonArray();
        for (JsonObject row : projected) {
            rows.add(row);
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "snapshot");
        record.add("statement", statement.getName());
        record.add("sequence", 0);
        record.add("time",
                Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
        record.add("new", rows);
        return record;
    }

    /**
     * Compiles an expected-invalid statement, mirroring env.tryInvalidCompile:
     * the two insert-keyword probes against the SupportBean event type compile
     * without the runtime path (the source's path-less tryInvalidCompile
     * overload at lines 303-306), the three insert-where probes compile with
     * the path so MyWindowINV resolves.  The compile must fail and the message
     * must start with the pinned prefix; the record value carries the asserted
     * prefix.
     */
    private static void buildErrorStep(Configuration configuration, EPRuntime runtime,
                                       String caseName, JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String expected = string(step, "expectError");
        boolean withoutPath = "missing-window-insert".equals(label)
                || "missing-window-insert-where".equals(label);
        String caught;
        try {
            CompilerArguments compilerArgs = new CompilerArguments(configuration);
            if (!withoutPath) {
                compilerArgs.getPath().add(runtime.getRuntimePath());
            }
            EPCompilerProvider.getCompiler().compile(string(step, "epl"), compilerArgs);
            caught = "<no-error>";
        } catch (Exception ex) {
            caught = ex.getMessage();
        }
        if ("<no-error>".equals(caught)) {
            throw new IllegalStateException("build-error probe " + label + " unexpectedly compiled");
        }
        if (!caught.startsWith(expected)) {
            throw new IllegalStateException("compile-error message drift for " + label
                    + ": expected prefix [" + expected + "] got [" + caught + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "compile-error");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", expected);
        records.add(record);
    }

    /**
     * Emits the pinned "unrepresentable" record for the ord-2 object-model
     * round-trip after verifying the surface in-process: the programmatic
     * create-window insert-where model renders the pinned EPL and eplToModel
     * round-trips it (InfraInsertWhereOMStaggered lines 252-263).
     */
    private static void unrepresentableStep(Configuration configuration, String caseName,
                                            JsonObject step, JsonArray records) {
        String label = string(step, "statement");
        String note = string(step, "expectError");
        if (!"om-roundtrip".equals(label) || !OM_NOTE.equals(note)) {
            throw new IllegalStateException("unrepresentable step " + label
                    + " is not pinned");
        }
        EPStatementObjectModel model = new EPStatementObjectModel();
        model.setAnnotations(new ArrayList<>(2));
        model.getAnnotations().add(new AnnotationPart("public"));
        Expression where = Expressions.eq("b", 10);
        model.setCreateWindow(CreateWindowClause.create("MyWindowIWOMTwo", View.create("keepall"))
                .insert(true).insertWhereClause(where).setAsEventTypeName("MyWindowIWOM"));
        model.setSelectClause(SelectClause.createWildcard());
        String text = "@public create window MyWindowIWOMTwo#keepall"
                + " as select * from MyWindowIWOM insert where b=10";
        if (!text.equals(model.toEPL().trim())) {
            throw new IllegalStateException("object-model toEPL drift: got [" + model.toEPL() + "]");
        }
        EPStatementObjectModel roundTripped;
        try {
            roundTripped = EPCompilerProvider.getCompiler().eplToModel(" " + text, configuration);
        } catch (Exception ex) {
            throw new IllegalStateException("eplToModel failed for the pinned create", ex);
        }
        if (!text.equals(roundTripped.toEPL().trim())) {
            throw new IllegalStateException("eplToModel round-trip drift: got ["
                    + roundTripped.toEPL() + "]");
        }
        JsonObject record = new JsonObject();
        record.add("case", caseName);
        record.add("operation", "unrepresentable");
        record.add("statement", label);
        record.add("sequence", 0);
        record.add("value", note);
        records.add(record);
    }

    private static String[] listenerFields(String caseName, String statementName) {
        if (CASE_CREATE_AFTER_NAMED.equals(caseName)) {
            return "selectOne".equals(statementName) ? FIELDS_STRING : FIELDS_BEAN;
        }
        if (CASE_INSERT_WHERE_TYPE_FILTER.equals(caseName)) {
            return FIELDS_BEAN;
        }
        if (CASE_OM_STAGGERED.equals(caseName)) {
            return FIELDS_AB;
        }
        throw new IllegalStateException("case " + caseName + " attaches no listeners");
    }

    private static String[] snapshotFields(String caseName, String statementName) {
        if (CASE_INSERT_WHERE_TYPE_FILTER.equals(caseName)) {
            return FIELDS_STRING;
        }
        if (CASE_OM_STAGGERED.equals(caseName)) {
            return "windowThree".equals(statementName) ? FIELDS_A : FIELDS_AB;
        }
        if (CASE_VARIANT_STREAM.equals(caseName)) {
            return FIELDS_VARIANT_ID;
        }
        if (CASE_LENIENT_MAP.equals(caseName) || CASE_LENIENT_OBJECTARRAY.equals(caseName)) {
            return FIELDS_TWO_COL;
        }
        throw new IllegalStateException("case " + caseName + " takes no snapshots");
    }

    /**
     * Row rendering projected to exactly the fields the Java assertions read,
     * in sorted order; a full window row carries many properties the Java
     * tests never read, so both engines render the projected fields only and
     * the pinned contract is the Java test's projection.
     */
    private static JsonObject projectedRow(EventBean event, String[] fields) {
        JsonObject item = new JsonObject();
        item.add("kind", "row");
        JsonObject values = new JsonObject();
        for (String field : fields) {
            values.add(field, normalize(event.get(field)));
        }
        item.add("fields", values);
        return item;
    }

    /** Projected rows for listener delivery, in delivery order. */
    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            array.add(projectedRow(event, fields));
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

    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "SupportBean": {
                SupportBean bean = new SupportBean();
                JsonValue theString = payload.get("theString");
                bean.setTheString(theString == null || theString.isNull()
                        ? null : string(payload, "theString"));
                bean.setIntPrimitive((int) longInteger(payload.get("intPrimitive"), "intPrimitive"));
                runtime.getEventService().sendEventBean(bean, type);
                break;
            }
            case "SupportBean_S0": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_S0((int) longInteger(payload.get("id"), "id")), type);
                break;
            }
            case "MyMapAB": {
                Map<String, Object> row = new HashMap<>();
                row.put("a", string(payload, "a"));
                row.put("b", (int) longInteger(payload.get("b"), "b"));
                runtime.getEventService().sendEventMap(row, type);
                break;
            }
            case "SupportBean_A": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_A(string(payload, "id")), type);
                break;
            }
            case "SupportBean_B": {
                runtime.getEventService().sendEventBean(
                        new SupportBean_B(string(payload, "id")), type);
                break;
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
        validateStringArray(scenario.get("javaFlags"), new String[0], "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != RUNTIME_IDS.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + RUNTIME_IDS.length + " cases");
        }
        String[] expectedCases = {
                CASE_CREATE_AFTER_NAMED, CASE_INSERT_WHERE_TYPE_FILTER, CASE_OM_STAGGERED,
                CASE_INVALID, CASE_VARIANT_STREAM,
                CASE_LENIENT_MAP, CASE_LENIENT_OBJECTARRAY};
        int[] expectedOrdinals = {0, 1, 2, 3, 4, 5, 6};
        int[] expectedSnapshots = {0, 3, 2, 0, 1, 1, 1};
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "iteratorSnapshots", "epl");
            if (!expectedCases[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != expectedOrdinals[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !OBSERVATIONS[index].equals(string(definition, "observation"))
                    || integer(definition, "iteratorSnapshots") != expectedSnapshots[index]
                    || !SOURCE_EPLS[index].equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        offset = validateCreateAfterNamedCase(steps, offset);
        offset = validateInsertWhereTypeFilterCase(steps, offset);
        offset = validateOMStaggeredCase(steps, offset);
        offset = validateInvalidCase(steps, offset);
        offset = validateVariantStreamCase(steps, offset);
        offset = validateLenientCase(steps, offset, CASE_LENIENT_MAP);
        offset = validateLenientCase(steps, offset, CASE_LENIENT_OBJECTARRAY);
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /**
     * Exact create-after-named step sequence mirroring InfraCreateNamedAfterNamed
     * lines 89-100: a keepall window created after nothing, a second window
     * typed by the first, the shared insert, and a select over the window; one
     * SupportBean delivers to both the windowOne and selectOne listeners.
     */
    private static int validateCreateAfterNamedCase(JsonArray steps, int offset) {
        String caseName = CASE_CREATE_AFTER_NAMED;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "windowOne", EPL_CREATE_WINDOW_ONE);
        validateDeploy(steps.get(offset++), caseName, "windowTwo", EPL_CREATE_WINDOW_TWO);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT);
        validateDeploy(steps.get(offset++), caseName, "selectOne", EPL_SELECT_ONE);
        validateBeanSend(steps.get(offset++), caseName, "E1", 1);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact insert-where-type-filter step sequence mirroring
     * InfraInsertWhereTypeAndFilter lines 109-224: five filtered seed sends,
     * the keep-all, filtered, and unique-index seeded creates (whose listeners
     * see nothing at deploy time), and four filtered routing inserts each
     * delivering exactly one row to its target window's listener.
     */
    private static int validateInsertWhereTypeFilterCase(JsonArray steps, int offset) {
        String caseName = CASE_INSERT_WHERE_TYPE_FILTER;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_CREATE_IWT);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_IWT);
        validateBeanSend(steps.get(offset++), caseName, "A1", 1);
        validateBeanSend(steps.get(offset++), caseName, "B2", 1);
        validateBeanSend(steps.get(offset++), caseName, "C3", 1);
        validateBeanSend(steps.get(offset++), caseName, "A4", 4);
        validateBeanSend(steps.get(offset++), caseName, "C5", 4);
        validateDeploy(steps.get(offset++), caseName, "windowTwo", EPL_CREATE_IWT_TWO);
        validateSnapshot(steps.get(offset++), caseName, "windowTwo", "ordered");
        validateDeploy(steps.get(offset++), caseName, "windowThree", EPL_CREATE_IWT_THREE);
        validateSnapshot(steps.get(offset++), caseName, "windowThree", "ordered");
        validateDeploy(steps.get(offset++), caseName, "windowFour", EPL_CREATE_IWT_FOUR);
        validateSnapshot(steps.get(offset++), caseName, "windowFour", "any");
        validateDeploy(steps.get(offset++), caseName, "insertA", EPL_INSERT_IWT_A);
        validateDeploy(steps.get(offset++), caseName, "insertB", EPL_INSERT_IWT_B);
        validateDeploy(steps.get(offset++), caseName, "insertC", EPL_INSERT_IWT_C);
        validateDeploy(steps.get(offset++), caseName, "insertD", EPL_INSERT_IWT_D);
        // InfraInsertWhereTypeAndFilter line 171 sends ("B9", -9); the B-prefixed
        // routing row lands in MyWindowTwo only.
        validateBeanSend(steps.get(offset++), caseName, "B9", -9);
        validateBeanSend(steps.get(offset++), caseName, "A8", -8);
        validateBeanSend(steps.get(offset++), caseName, "C7", -7);
        validateBeanSend(steps.get(offset++), caseName, "D6", -6);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact insert-where-om-staggered step sequence mirroring
     * InfraInsertWhereOMStaggered lines 235-272 narrowed to rep=MAP: the
     * keepall window over MyMapAB, the shared insert, three map sends, the
     * pinned unrepresentable marker for the object-model round-trip asserts,
     * the b=10-filtered create-window-insert seeding MyWindowIWOMTwo, and the
     * chained a='E2' create seeding MyWindowIWOMThree.
     */
    private static int validateOMStaggeredCase(JsonArray steps, int offset) {
        String caseName = CASE_OM_STAGGERED;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_CREATE_IWOM);
        validateDeploy(steps.get(offset++), caseName, "insert", EPL_INSERT_IWOM);
        validateMapABSend(steps.get(offset++), caseName, "E1", 2);
        validateMapABSend(steps.get(offset++), caseName, "E2", 10);
        validateMapABSend(steps.get(offset++), caseName, "E3", 10);
        validateUnrepresentable(steps.get(offset++), caseName, "om-roundtrip", OM_NOTE);
        validateDeploy(steps.get(offset++), caseName, "windowTwo", EPL_CREATE_IWOM_TWO);
        validateSnapshot(steps.get(offset++), caseName, "windowTwo", "ordered");
        validateDeploy(steps.get(offset++), caseName, "windowThree", EPL_CREATE_IWOM_THREE);
        validateSnapshot(steps.get(offset++), caseName, "windowThree", "ordered");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact infra-invalid step sequence mirroring InfraInvalid lines
     * 300-314: the MyWindowINV fixture deploy then the five tryInvalidCompile
     * probes with their pinned message prefixes.
     */
    private static int validateInvalidCase(JsonArray steps, int offset) {
        String caseName = CASE_INVALID;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_CREATE_INV);
        validateBuildError(steps.get(offset++), caseName, "missing-window-insert",
                PROBE_INSERT, PREFIX_MISSING_WINDOW);
        validateBuildError(steps.get(offset++), caseName, "missing-window-insert-where",
                PROBE_INSERT_WHERE, PREFIX_MISSING_WINDOW);
        validateBuildError(steps.get(offset++), caseName, "insert-where-subselect",
                PROBE_SUBSELECT, PREFIX_SUBSELECT);
        validateBuildError(steps.get(offset++), caseName, "insert-where-aggregation",
                PROBE_AGGREGATION, PREFIX_AGGREGATION);
        validateBuildError(steps.get(offset++), caseName, "insert-where-prev",
                PROBE_PREV, PREFIX_PREV);
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact variant-stream step sequence mirroring InfraVariantStream lines
     * 278-293: the variant-schema window pair, the three insert-into routing
     * statements, the SupportBean_A/SupportBean_B sends, and the ordered
     * iterator snapshot projecting the dynamic id? property.
     */
    private static int validateVariantStreamCase(JsonArray steps, int offset) {
        String caseName = CASE_VARIANT_STREAM;
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "windowVS", EPL_CREATE_VS);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_CREATE_VS_TWO);
        validateDeploy(steps.get(offset++), caseName, "insertA", EPL_INSERT_VS_A);
        validateDeploy(steps.get(offset++), caseName, "insertB", EPL_INSERT_VS_B);
        validateDeploy(steps.get(offset++), caseName, "insertVS", EPL_INSERT_VS_TWO);
        validateVariantSend(steps.get(offset++), caseName, "SupportBean_A", "A1");
        validateVariantSend(steps.get(offset++), caseName, "SupportBean_B", "B1");
        validateSnapshot(steps.get(offset++), caseName, "window", "ordered");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    /**
     * Exact lenient step sequence mirroring InfraNamedWindowInsertLenientPropCount
     * lines 61-77: a two-column schema and keepall window, two partial-column
     * inserts, and one send per insert, each observed through an ordered
     * iterator snapshot showing the unassigned column as null.
     */
    private static int validateLenientCase(JsonArray steps, int offset, String caseName) {
        validateCaseMarker(steps.get(offset++), caseName);
        validateDeploy(steps.get(offset++), caseName, "schema",
                CASE_LENIENT_MAP.equals(caseName) ? EPL_SCHEMA_MAP : EPL_SCHEMA_OBJECTARRAY);
        validateDeploy(steps.get(offset++), caseName, "window", EPL_WINDOW_TWO_COL);
        validateDeploy(steps.get(offset++), caseName, "insertOne", EPL_INSERT_ONE);
        validateDeploy(steps.get(offset++), caseName, "insertTwo", EPL_INSERT_TWO);
        validateBeanSend(steps.get(offset++), caseName, "E1", 0);
        validateSnapshot(steps.get(offset++), caseName, "window", "ordered");
        validateS0Send(steps.get(offset++), caseName, 10);
        validateSnapshot(steps.get(offset++), caseName, "window", "ordered");
        validateUndeployAll(steps.get(offset++), caseName);
        return offset;
    }

    private static void validateMapABSend(JsonValue value, String caseName, String expectedA,
                                          long expectedB) {
        JsonObject step = object(value, "MyMapAB step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"MyMapAB".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("MyMapAB step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "MyMapAB payload");
        requireFields(payload, "a", "b");
        if (!expectedA.equals(string(payload, "a"))
                || longInteger(payload.get("b"), "b") != expectedB) {
            throw new IllegalArgumentException("MyMapAB payload is not pinned for " + caseName);
        }
    }

    private static void validateVariantSend(JsonValue value, String caseName, String expectedType,
                                            String expectedId) {
        JsonObject step = object(value, "variant send step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedType.equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("variant send step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "variant payload");
        requireFields(payload, "id");
        if (!expectedId.equals(string(payload, "id"))) {
            throw new IllegalArgumentException("variant payload is not pinned for " + caseName);
        }
    }

    private static void validateBuildError(JsonValue value, String caseName, String expectedLabel,
                                           String expectedEpl, String expectedError) {
        JsonObject step = object(value, "build-error step");
        requireFields(step, "op", "case", "statement", "epl", "expectError");
        if (!"build-error".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedLabel.equals(string(step, "statement"))
                || !expectedEpl.equals(string(step, "epl"))
                || !expectedError.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("build-error step is not pinned for " + caseName
                    + "/" + expectedLabel);
        }
    }

    private static void validateUnrepresentable(JsonValue value, String caseName,
                                                String expectedLabel, String expectedNote) {
        JsonObject step = object(value, "unrepresentable step");
        requireFields(step, "op", "case", "statement", "expectError");
        if (!"unrepresentable".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedLabel.equals(string(step, "statement"))
                || !expectedNote.equals(string(step, "expectError"))) {
            throw new IllegalArgumentException("unrepresentable step is not pinned for " + caseName
                    + "/" + expectedLabel);
        }
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

    private static void validateBeanSend(JsonValue value, String caseName, String expectedString,
                                         long expectedIntPrimitive) {
        JsonObject step = object(value, "SupportBean step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean payload");
        requireFields(payload, "theString", "intPrimitive");
        JsonValue theString = payload.get("theString");
        boolean stringMatches = expectedString == null
                ? theString.isNull()
                : theString instanceof JsonString && expectedString.equals(theString.asString());
        if (!stringMatches
                || longInteger(payload.get("intPrimitive"), "intPrimitive")
                != expectedIntPrimitive) {
            throw new IllegalArgumentException("SupportBean payload is not pinned for " + caseName);
        }
    }

    private static void validateS0Send(JsonValue value, String caseName, long expectedId) {
        JsonObject step = object(value, "SupportBean_S0 step");
        requireFields(step, "op", "case", "eventType", "payload");
        if (!"send".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !"SupportBean_S0".equals(string(step, "eventType"))) {
            throw new IllegalArgumentException("SupportBean_S0 step is not pinned for " + caseName);
        }
        JsonObject payload = object(step.get("payload"), "SupportBean_S0 payload");
        requireFields(payload, "id");
        if (longInteger(payload.get("id"), "id") != expectedId) {
            throw new IllegalArgumentException("SupportBean_S0 payload is not pinned for "
                    + caseName);
        }
    }

    private static void validateSnapshot(JsonValue value, String caseName, String expectedStatement,
                                         String expectedMode) {
        JsonObject step = object(value, "snapshot step");
        requireFields(step, "op", "case", "statement", "mode");
        if (!"snapshot".equals(string(step, "op"))
                || !caseName.equals(string(step, "case"))
                || !expectedStatement.equals(string(step, "statement"))
                || !expectedMode.equals(string(step, "mode"))) {
            throw new IllegalArgumentException("snapshot step is not pinned for " + caseName + "/"
                    + expectedStatement);
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
