import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowCancellationException;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstance;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowInstantiationOptions;
import com.espertech.esper.common.client.dataflow.core.EPDataFlowState;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportCaptureOp;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProvider;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportGraphOpProviderByOpName;
import com.espertech.esper.common.internal.epl.dataflow.util.DefaultSupportSourceOp;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.regressionlib.framework.RegressionPath;
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
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Direct Esper 9.0.0 oracle for the dataflow blocking run()/cancel lifecycle
 * differential chain (work unit 4.376, dataflow-lifecycle-blocking): the
 * three blocking executions of EPLDataflowAPIRunStartCancelJoin (ordinals
 * 4, 10 and 11). The seven non-blocking executions (ordinals 0, 1, 2, 5, 7,
 * 9 and 12) are covered by the dataflow-lifecycle-cancel-join oracle and
 * ordinals 3 and 6 by the dataflow-lifecycle-core oracle.
 *
 * blocking-cancel (ordinal 4) deploys the byte-exact SomeType schema preamble
 * plus the two-clause source/capture graph whose capture clause is
 * concatenated with NO separator, instantiates through
 * DefaultSupportGraphOpProviderByOpName a source instructed
 * [latch, Object[]{1}] and a plain (unlatched) capture. Java's blocking
 * run() cannot serve as the replay driver (it parks the driver thread), so
 * the deterministic replay drives the started instance: instance.start(); a
 * bounded in-process poll for the RUNNING state (10 s deadline, 10 ms steps —
 * the replay of the suite's 300 ms cancelling-thread sleep, a wait device;
 * start() sets RUNNING synchronously so the poll returns immediately); then
 * instance.cancel(), which synchronously shuts down and interrupts the
 * latched source thread and moves the instance to CANCELLED (asserted
 * in-process; the suite never releases the latch — the cancel interrupt is
 * the only unblock). The suite's assertion run() throws
 * EPDataFlowCancellationException with message "Data flow 'MyDataFlowOne'
 * execution was cancelled" is anchored in-process against the Java model
 * with a second instance instantiated from the same deployment and driven
 * through the byte-exact blocking path: a side thread bounded-polls its
 * RUNNING state (10 s deadline, 10 ms steps) and cancels, the main thread's
 * run() throws EPDataFlowCancellationException, the exact message is asserted
 * with equals and the second instance is asserted CANCELLED with an empty
 * capture (cancel wins before the row instruction is reached). The exception
 * text is NEVER recorded — only the error-class token is.
 *
 * fast-complete-blocking (ordinal 10) deploys the byte-exact BeaconSource
 * graph (iterations:1 with the capture clause concatenated with NO separator,
 * no schema preamble) with a capture latched at 1 through
 * DefaultSupportGraphOpProvider. The dataflow name 'MyDataFlowOne' and the
 * INSTANTIATED entry state are asserted in-process; the suite's sleep(1000)
 * proving the negative not-done property is replayed as an instant in-process
 * assertion (no sleep — nothing has started the flow, so the capture cannot
 * be done) recorded as the lifecycle token not-done-before-run=true; the
 * blocking run() completes the single-iteration flow; the capture read holds
 * exactly 1 row; then tryAssertionAfterExec replays in-process: join() no-op,
 * run()/start() IllegalStateException with the byte-exact already-completed
 * message (asserted in-process), cancel() and join() silent no-ops.
 *
 * run-blocking (ordinal 11) deploys the byte-exact SomeType preamble plus the
 * one-line s-stream graph (same one-clause shape as blocking-cancel with the
 * stream named 's'), instantiates through DefaultSupportGraphOpProvider in
 * the suite's (future, source) order a source instructed
 * [latch, Object[]{1}] and a capture latched at 1. The dataflow name and the
 * INSTANTIATED entry state are asserted in-process. The suite's unlatching
 * thread (spin on the RUNNING state with sleep(0) plus a 100 ms sleep) is
 * replayed as a bounded poll for RUNNING (10 s deadline, 10 ms steps) that
 * then releases the latch — the release is unconditional so the blocking
 * run() always returns and a missed observation fails the oracle loudly. The
 * main thread parks inside run(); the RUNNING state record is therefore
 * sourced from that deterministic pre-release observation (volatile flag),
 * because by the time run() returns the state has moved to COMPLETE. After
 * run(): state COMPLETE; the first capture batch holds exactly 1 row; and the
 * source consumed the latch await, the row submit and the final marker
 * (getCurrentCount()==2, in-process).
 *
 * Record protocol (dataflow state/count conventions): state records are
 * {case, operation:"state", statement:"flow", sequence, time,
 * name:"instance.state", value:UPPERCASE EPDataFlowState name}, count
 * records are {case, operation:"count", statement:"flow", sequence, time,
 * name, count} and lifecycle records are {case, operation:"lifecycle",
 * statement:"flow", sequence, time, name, value}. Capture reads are recorded
 * through the count operation only (capture-empty for the cancelled case,
 * capture-rows for the completing cases — the observable is the row count
 * being zero/one, not the capture contents). Error texts are NEVER recorded:
 * the cancellation message and the already-completed IllegalStateException
 * text are asserted in-process with equals against the Java engine, and only
 * the error-class token "cancellation-exception" and the lifecycle token
 * "not-done-before-run" are recorded. Sequence is case-local and restarts at
 * 1; the time is the fixed epoch 1970-01-01T00:00:00Z. Each case runs on its
 * own fresh runtime (internal timer off, epoch initialization) that is
 * undeployAll'd after the case body and destroyed in finally; the dataflow
 * deployment id is pinned 'flow' but never recorded, while the SomeType-schema
 * deployments use engine-generated ids. Session configuration mirrors
 * TestSuiteEPLDataflow.configure restricted to these executions: only the
 * com.espertech.esper.common.internal.epl.dataflow.util.* package import
 * that resolves DefaultSupportSourceOp / DefaultSupportCaptureOp in EPL text
 * is required (BeaconSource resolves through the built-in
 * ConfigurationCommon.DATAFLOWOPERATOR_IMPORT and the SomeType type is
 * declared by the deployed byte-exact EPL preamble over the RegressionPath).
 */
public final class DataflowLifecycleBlockingScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-lifecycle-blocking";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIRunStartCancelJoin.java";
    private static final String CASE_BLOCKING_CANCEL = "blocking-cancel";
    private static final String CASE_FAST_COMPLETE_BLOCKING = "fast-complete-blocking";
    private static final String CASE_RUN_BLOCKING = "run-blocking";
    private static final String[] CASES = {
            CASE_BLOCKING_CANCEL, CASE_FAST_COMPLETE_BLOCKING, CASE_RUN_BLOCKING};
    private static final int[] ORDINALS = {4, 10, 11};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-91a5f5421ce63806e2c4",
            "java-runtime-42082e1062bebbe49a0a",
            "java-runtime-bcc34451f5417d84e6fa"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowBlockingCancel",
            "EPLDataflowFastCompleteBlocking",
            "EPLDataflowRunBlocking"};
    private static final int[] RECORD_COUNTS = {4, 3, 3};
    private static final int TOTAL_RECORDS = 10;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME_ONE = "MyDataFlowOne";
    private static final String EPOCH = "1970-01-01T00:00:00Z";
    private static final long BOUNDED_WAIT_MS = 10000L;

    // Byte-exact graphs from EPLDataflowAPIRunStartCancelJoin: the
    // blocking-cancel graph concatenates the DefaultSupportCaptureOp clause
    // with NO separator after the source clause's trailing '{}' (lines
    // 198-200); the fast-complete-blocking graph concatenates the capture
    // clause with NO separator after the BeaconSource clause's trailing '}'
    // (lines 527-529); the run-blocking graph has the same one-clause shape
    // with the stream named 's' (lines 568-570).
    private static final String SOME_TYPE_EPL = "@public create schema SomeType ()";
    private static final String OUTSTREAM_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "DefaultSupportSourceOp -> outstream<SomeType> {}" +
            "DefaultSupportCaptureOp(outstream) {}";
    private static final String BEACON_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "BeaconSource -> BeaconStream {iterations : 1}" +
            "DefaultSupportCaptureOp(BeaconStream) {}";
    private static final String S_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "DefaultSupportSourceOp -> s<SomeType> {}" +
            "DefaultSupportCaptureOp(s) {}";

    // Byte-exact Java message texts asserted in-process, never recorded.
    private static final String MSG_CANCELLATION =
            "Data flow 'MyDataFlowOne' execution was cancelled";
    private static final String MSG_AFTER_COMPLETE =
            "Data flow 'MyDataFlowOne' instance has already completed, please use instantiate to run the data flow again";

    private DataflowLifecycleBlockingScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowLifecycleBlockingScenarioOracle <scenario.json>");
            System.exit(2);
        }
        JsonObject scenario = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8)).asObject();
        validateScenario(scenario);
        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, records);
        }
        if (records.size() != TOTAL_RECORDS) {
            throw new IllegalStateException("expected " + TOTAL_RECORDS + " records, got " + records.size());
        }
        JsonObject trace = new JsonObject().add("version", VERSION).add("id", SCENARIO_ID)
                .add("javaCommit", PINNED_COMMIT).add("java", System.getProperty("java.version")).add("records", records);
        System.out.println(trace);
    }

    private static void validateScenario(JsonObject scenario) {
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id: " + scenario.getString("id", ""));
        }
        if (!PINNED_COMMIT.equals(scenario.getString("javaCommit", ""))) {
            throw new IllegalArgumentException("scenario javaCommit is not pinned");
        }
        if (!JAVA_SOURCE.equals(scenario.getString("javaSource", JAVA_SOURCE))) {
            throw new IllegalArgumentException("scenario javaSource is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");

        JsonValue caseValue = scenario.get("cases");
        if (caseValue == null || !caseValue.isArray() || caseValue.asArray().size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain the three selected cases");
        }
        JsonArray caseDefinitions = caseValue.asArray();
        for (int i = 0; i < CASES.length; i++) {
            JsonObject definition = object(caseDefinitions.get(i), "case definition " + i);
            if (!CASES[i].equals(definition.getString("case", ""))
                    || definition.getInt("ordinal", -1) != ORDINALS[i]
                    || !RUNTIME_IDS[i].equals(definition.getString("runtimeId", ""))
                    || !EXECUTION_NAMES[i].equals(definition.getString("executionName", ""))) {
                throw new IllegalArgumentException("case metadata mismatch at index " + i);
            }
        }

        JsonValue stepValue = scenario.get("steps");
        if (stepValue == null || !stepValue.isArray() || stepValue.asArray().size() != CASES.length * 2) {
            throw new IllegalArgumentException("scenario must contain exactly " + (CASES.length * 2) + " steps");
        }
        JsonArray steps = stepValue.asArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            JsonObject marker = object(steps.get(caseIndex * 2), "case marker " + caseIndex);
            if (!"case".equals(marker.getString("op", "")) || !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("cases must appear once in source order");
            }
            JsonObject advance = object(steps.get(caseIndex * 2 + 1), "advance-time " + caseIndex);
            if (!"advance-time".equals(advance.getString("op", ""))
                    || !EPOCH.equals(advance.getString("at", ""))) {
                throw new IllegalArgumentException("each case must pin the epoch advance-time step");
            }
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String name) {
        if (value == null || !value.isArray() || value.asArray().size() != expected.length) {
            throw new IllegalArgumentException(name + " must contain exactly " + expected.length + " values");
        }
        JsonArray values = value.asArray();
        for (int i = 0; i < expected.length; i++) {
            if (!expected[i].equals(values.get(i).asString())) {
                throw new IllegalArgumentException(name + " mismatch at index " + i);
            }
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be an object");
        }
        return value.asObject();
    }

    private static void runCase(int caseIndex, JsonArray records) throws Exception {
        String caseName = CASES[caseIndex];

        // Session configuration per TestSuiteEPLDataflow.configure restricted
        // to these executions: only the dataflow-util package import that
        // resolves DefaultSupportSourceOp / DefaultSupportCaptureOp in EPL
        // text (all three graphs name at least the capture operator).
        // BeaconSource resolves through the built-in
        // ConfigurationCommon.DATAFLOWOPERATOR_IMPORT; the SomeType type is
        // declared by the deployed byte-exact EPL preamble over the
        // RegressionPath. Internal timer off, epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-lifecycle-blocking-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            switch (caseIndex) {
                case 0:
                    runBlockingCancel(configuration, runtime, writer);
                    break;
                case 1:
                    runFastCompleteBlocking(configuration, runtime, writer);
                    break;
                default:
                    runRunBlocking(configuration, runtime, writer);
                    break;
            }
            if (writer.count() != RECORD_COUNTS[caseIndex]) {
                throw new IllegalStateException("case " + caseName + " produced " + writer.count()
                        + " records, expected " + RECORD_COUNTS[caseIndex]);
            }
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowBlockingCancel (ordinal 4):
     * cancel ends the blocking execution — the started-instance replay is
     * CANCELLED with an empty capture, and the suite's run()-throws
     * EPDataFlowCancellationException assertion is anchored in-process
     * against the Java model with a second instance driven through the
     * blocking path.
     */
    private static void runBlockingCancel(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 196-200: the byte-exact SomeType preamble and the
        // two-clause graph (capture clause concatenated with NO separator)
        // over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, OUTSTREAM_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 203-210: named operators through
        // DefaultSupportGraphOpProviderByOpName; the source is instructed
        // [latch, Object[]{1}] and the capture is a plain (unlatched) capture.
        CountDownLatch latchOne = new CountDownLatch(1);
        Map<String, Object> ops = new HashMap<String, Object>();
        ops.put("DefaultSupportSourceOp", new DefaultSupportSourceOp(new Object[]{latchOne, new Object[]{1}}));
        DefaultSupportCaptureOp<Object> output = new DefaultSupportCaptureOp<Object>();
        ops.put("DefaultSupportCaptureOp", output);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(ops));
        final EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // Deterministic replay of suite lines 212-229: Java's blocking run()
        // cannot serve as the replay driver, so the started instance carries
        // it — start() (start sets RUNNING synchronously), the bounded poll
        // for RUNNING (the replay of the suite's 300 ms cancelling sleep, a
        // wait device), then cancel() which synchronously interrupts the
        // latched await. The suite never releases the latch.
        dfOne.start();
        awaitState(dfOne, EPDataFlowState.RUNNING, "blocking-cancel");
        assertState(dfOne, EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());
        dfOne.cancel();
        assertState(dfOne, EPDataFlowState.CANCELLED);

        // suite lines 224-229 anchored in-process against the Java model:
        // a second instance from the same deployment, driven through the
        // byte-exact blocking path — a side thread bounded-polls its RUNNING
        // state and cancels, and the main thread's run() throws
        // EPDataFlowCancellationException with the byte-exact message
        // (asserted with equals, never recorded).
        CountDownLatch latchTwo = new CountDownLatch(1);
        Map<String, Object> opsTwo = new HashMap<String, Object>();
        opsTwo.put("DefaultSupportSourceOp", new DefaultSupportSourceOp(new Object[]{latchTwo, new Object[]{1}}));
        DefaultSupportCaptureOp<Object> outputTwo = new DefaultSupportCaptureOp<Object>();
        opsTwo.put("DefaultSupportCaptureOp", outputTwo);
        EPDataFlowInstantiationOptions optionsTwo = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(opsTwo));
        final EPDataFlowInstance dfTwo = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, optionsTwo);

        Thread cancellingThread = new Thread(new Runnable() {
            public void run() {
                long start = System.currentTimeMillis();
                while (dfTwo.getState() != EPDataFlowState.RUNNING
                        && System.currentTimeMillis() - start <= BOUNDED_WAIT_MS) {
                    try {
                        Thread.sleep(10);
                    } catch (InterruptedException e) {
                        break;
                    }
                }
                dfTwo.cancel();
            }
        }, "EPLDataflowBlockingCancel-run-cancelling");
        cancellingThread.start();
        try {
            dfTwo.run();
            throw new IllegalStateException("expected EPDataFlowCancellationException from run");
        } catch (EPDataFlowCancellationException ex) {
            if (!MSG_CANCELLATION.equals(ex.getMessage())) {
                throw new IllegalStateException("run cancellation message was " + ex.getMessage()
                        + ", expected " + MSG_CANCELLATION);
            }
        }
        cancellingThread.join();
        assertState(dfTwo, EPDataFlowState.CANCELLED);
        if (outputTwo.getAndReset().size() != 0) {
            throw new IllegalStateException("second instance capture was not empty");
        }

        // record 2: the error-class token for the cancellation.
        writer.add("lifecycle", "flow", "run.error-class", Json.value("cancellation-exception"));

        // suite line 230: the cancelled state of the replay instance.
        assertState(dfOne, EPDataFlowState.CANCELLED);
        writer.addState(dfOne.getState());

        // suite line 231: the capture read is empty.
        int captured = output.getAndReset().size();
        if (captured != 0) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 0");
        }
        writer.addCount("capture-empty", captured);

        // suite line 232: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowFastCompleteBlocking
     * (ordinal 10): the BeaconSource iterations:1 flow completes inside the
     * blocking run and the post-execution join/run/start/cancel state machine
     * holds.
     */
    private static void runFastCompleteBlocking(Configuration configuration, EPRuntime runtime,
                                                TraceWriter writer) throws Exception {
        // suite lines 527-529: byte-exact BeaconSource graph — the capture
        // clause is concatenated with NO separator after the source clause's
        // trailing '}' and no schema preamble is needed.
        deploy(runtime, compile(configuration, runtime, BEACON_FLOW_EPL, null), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 532-536: the capture is latched at 1 row; the
        // INSTANTIATED entry state and the dataflow name are in-process
        // asserts.
        DefaultSupportCaptureOp<Object> future = new DefaultSupportCaptureOp<Object>(1);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(future));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);
        if (!FLOW_NAME_ONE.equals(dfOne.getDataFlowName())) {
            throw new IllegalStateException("dataflow name was " + dfOne.getDataFlowName()
                    + ", expected " + FLOW_NAME_ONE);
        }
        assertState(dfOne, EPDataFlowState.INSTANTIATED);

        // suite lines 538-540: the not-done property. The suite sleeps 1000
        // ms to prove the negative; nothing has started the flow, so the
        // property is deterministic and the replay asserts it instantly (no
        // sleep), recorded as the lifecycle token.
        if (future.isDone()) {
            throw new IllegalStateException("capture reported done before the flow ran");
        }
        writer.add("lifecycle", "flow", "not-done-before-run", Json.value(true));

        // suite lines 542-544: the blocking run completes the flow.
        dfOne.run();
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite lines 545-549: the capture read holds exactly 1 row.
        Object[] currentBatch;
        try {
            currentBatch = future.get();
        } catch (Throwable t) {
            throw new RuntimeException(t);
        }
        if (currentBatch.length != 1) {
            throw new IllegalStateException("capture held " + currentBatch.length + " rows, expected 1");
        }
        writer.addCount("capture-rows", currentBatch.length);

        // suite line 552 (and 654-683): tryAssertionAfterExec — cancel and
        // join ignored, run/start rejected with the byte-exact message
        // (asserted in-process), cancel and join silent no-ops.
        tryAssertionAfterExec(dfOne);

        // suite line 554: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowRunBlocking (ordinal 11):
     * the blocking run parks until the deterministic pre-release RUNNING
     * observation, then completes with one captured row and both source
     * instructions consumed.
     */
    private static void runRunBlocking(Configuration configuration, EPRuntime runtime,
                                       TraceWriter writer) throws Exception {
        // suite lines 566-570: the byte-exact SomeType preamble and the
        // one-line s-stream graph (capture clause concatenated with NO
        // separator) over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, S_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 573-577: the source is instructed [latch, Object[]{1}]
        // and the capture is latched at 1 row, provided in the suite's
        // (future, source) order.
        final CountDownLatch latch = new CountDownLatch(1);
        DefaultSupportSourceOp source = new DefaultSupportSourceOp(new Object[]{latch, new Object[]{1}});
        DefaultSupportCaptureOp<Object> future = new DefaultSupportCaptureOp<Object>(1);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(future, source));
        final EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite lines 578-579: the dataflow name and the INSTANTIATED entry
        // state (in-process asserts, not recorded).
        if (!FLOW_NAME_ONE.equals(dfOne.getDataFlowName())) {
            throw new IllegalStateException("dataflow name was " + dfOne.getDataFlowName()
                    + ", expected " + FLOW_NAME_ONE);
        }
        assertState(dfOne, EPDataFlowState.INSTANTIATED);

        // suite lines 581-597: the unlatching thread. The suite spins on the
        // RUNNING state (sleep(0)) and sleeps 100 ms before the countDown —
        // replayed as a bounded poll for RUNNING (10 s deadline, 10 ms steps)
        // that then releases the latch. The release is unconditional so the
        // blocking run() always returns; a missed observation fails the
        // oracle loudly below.
        final AtomicBoolean observedRunning = new AtomicBoolean(false);
        Thread unlatchingThread = new Thread(new Runnable() {
            public void run() {
                long start = System.currentTimeMillis();
                while (dfOne.getState() != EPDataFlowState.RUNNING
                        && System.currentTimeMillis() - start <= BOUNDED_WAIT_MS) {
                    try {
                        Thread.sleep(10);
                    } catch (InterruptedException e) {
                        break;
                    }
                }
                if (dfOne.getState() == EPDataFlowState.RUNNING) {
                    observedRunning.set(true);
                }
                latch.countDown();
            }
        }, "EPLDataflowRunBlocking-unlatching");
        unlatchingThread.start();
        dfOne.run();

        // suite line 598: the blocking run completes the flow.
        assertState(dfOne, EPDataFlowState.COMPLETE);
        if (!observedRunning.get()) {
            throw new IllegalStateException("the RUNNING state was not observed within "
                    + BOUNDED_WAIT_MS + " ms before the latch release");
        }
        // the RUNNING record is sourced from the deterministic pre-release
        // observation: the main thread was parked inside run() while the
        // state held.
        writer.addState(EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());

        // suite line 599: the first capture batch holds exactly 1 row.
        List<List<Object>> batches = future.getAndReset();
        if (batches.isEmpty() || batches.get(0).size() != 1) {
            throw new IllegalStateException("first capture batch was "
                    + (batches.isEmpty() ? "absent" : Integer.toString(batches.get(0).size()))
                    + ", expected 1 row");
        }
        writer.addCount("capture-rows", batches.get(0).size());

        // suite line 600: the source consumed the latch await, the row submit
        // and the final marker (currentCount 2).
        if (source.getCurrentCount() != 2) {
            throw new IllegalStateException("source currentCount was " + source.getCurrentCount()
                    + ", expected 2");
        }

        // suite lines 601-605: the unlatching thread terminates.
        unlatchingThread.join();

        // suite line 606: undeployAll happens at the case boundary.
    }

    /**
     * Mirrors EPLDataflowAPIRunStartCancelJoin.tryAssertionAfterExec: join is
     * ignored after COMPLETE, run/start throw the byte-exact
     * IllegalStateException, and a trailing cancel plus join are silent
     * no-ops.
     */
    private static void tryAssertionAfterExec(EPDataFlowInstance df) throws Exception {
        // cancel and join ignored
        df.join();

        // can't start or run again
        try {
            df.run();
            throw new IllegalStateException("expected IllegalStateException from run");
        } catch (IllegalStateException ex) {
            if (!MSG_AFTER_COMPLETE.equals(ex.getMessage())) {
                throw new IllegalStateException("run message was " + ex.getMessage()
                        + ", expected " + MSG_AFTER_COMPLETE);
            }
        }

        try {
            df.start();
            throw new IllegalStateException("expected IllegalStateException from start");
        } catch (IllegalStateException ex) {
            if (!MSG_AFTER_COMPLETE.equals(ex.getMessage())) {
                throw new IllegalStateException("start message was " + ex.getMessage()
                        + ", expected " + MSG_AFTER_COMPLETE);
            }
        }

        df.cancel();
        df.join();
    }

    private static void awaitState(EPDataFlowInstance instance, EPDataFlowState expected, String what) {
        long start = System.currentTimeMillis();
        while (instance.getState() != expected) {
            if (System.currentTimeMillis() - start > BOUNDED_WAIT_MS) {
                throw new IllegalStateException("timed out waiting for the " + expected
                        + " state (" + what + ")");
            }
            try {
                Thread.sleep(10);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new IllegalStateException("interrupted waiting for the " + expected + " state");
            }
        }
    }

    private static void assertState(EPDataFlowInstance instance, EPDataFlowState expected) {
        EPDataFlowState actual = instance.getState();
        if (actual != expected) {
            throw new IllegalStateException("instance state was " + actual + ", expected " + expected);
        }
    }

    /**
     * Compiles the EPL: with a RegressionPath the compileds collected on the
     * path are exported (RegressionEnvironmentBase.getArgsWithExportToPath
     * analog), without one the runtime path is exported.
     */
    private static EPCompiled compile(Configuration configuration, EPRuntime runtime, String epl, RegressionPath path)
            throws Exception {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        if (path != null) {
            compilerArgs.getPath().getCompileds().addAll(path.getCompileds());
        } else {
            compilerArgs.getPath().add(runtime.getRuntimePath());
        }
        return EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
    }

    private static EPDeployment deploy(EPRuntime runtime, EPCompiled compiled, String deploymentId) throws Exception {
        return runtime.getDeploymentService().deploy(compiled,
                new DeploymentOptions().setDeploymentId(deploymentId));
    }

    /**
     * RegressionEnvironmentBase.deploymentId analog: the deployment id of the
     * deployment containing the named statement.
     */
    private static String deploymentId(EPRuntime runtime, String statementName) throws Exception {
        for (String deployment : runtime.getDeploymentService().getDeployments()) {
            EPDeployment info = runtime.getDeploymentService().getDeployment(deployment);
            for (EPStatement stmt : info.getStatements()) {
                if (stmt.getName().equals(statementName)) {
                    return stmt.getDeploymentId();
                }
            }
        }
        throw new IllegalStateException("statement not found: " + statementName);
    }

    /**
     * Emits {case, operation:"state", statement:"flow", sequence, time,
     * name:"instance.state", value} records for the instance-state reads,
     * {case, operation:"count", statement:"flow", sequence, time, name,
     * count} records for the capture reads and {case, operation:"lifecycle",
     * statement:"flow", sequence, time, name, value} records for the
     * lifecycle observables; the sequence is case-local.
     */
    private static final class TraceWriter {
        private final JsonArray records;
        private final String caseName;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName) {
            this.records = records;
            this.caseName = caseName;
        }

        private void addCount(String name, int count) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "count")
                    .add("statement", "flow")
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", name);
            record.add("count", count);
            records.add(record);
        }

        private void add(String operation, String statement, String name, JsonValue value) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", operation)
                    .add("statement", statement)
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", name);
            record.add("value", value);
            records.add(record);
        }

        private void addState(EPDataFlowState state) {
            add("state", "flow", "instance.state", Json.value(state.name()));
        }

        private long count() {
            return sequence;
        }
    }
}
