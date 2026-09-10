import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.configuration.Configuration;
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
import com.espertech.esper.regressionlib.suite.epl.dataflow.EPLDataflowAPIRunStartCancelJoin;
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

/**
 * Direct Esper 9.0.0 oracle for the dataflow non-blocking start/join/cancel
 * differential chain (work unit 4.375, dataflow-lifecycle-cancel-join): the
 * seven non-blocking executions of EPLDataflowAPIRunStartCancelJoin (ordinals
 * 0, 1, 2, 5, 7, 9 and 12) driven through DefaultSupportSourceOp positional
 * instructions and latched DefaultSupportCaptureOp reads. The blocking run()
 * executions are not part of this unit (ordinals 3 and 6 are covered by the
 * dataflow-lifecycle-core oracle).
 *
 * nonblocking-join-cancel (ordinal 0) deploys the byte-exact SomeType schema
 * preamble plus the two-clause source/capture graph whose capture clause is
 * concatenated with NO separator, instantiates a source instructed [latch]
 * (the suite never releases this latch) and a plain capture through
 * DefaultSupportGraphOpProvider, start()s (strengthened RUNNING state — the
 * suite asserts no state here), and cancels from a side thread. The suite's
 * 300 ms cancel sleep is a wait device replayed as a bounded cancel: the side
 * thread polls for the RUNNING state (10 s deadline, 10 ms steps) and then
 * cancels — cancel interrupts the latched await, join() returns and the state
 * is CANCELLED (asserted in-process) while the capture read is empty.
 *
 * nonblocking-join-exception (ordinal 1) deploys the same byte-exact graph,
 * instantiates a source instructed [latch, MyRuntimeException("TestException")]
 * and a plain capture, start()s and releases the latch from a side thread
 * (bounded replay of the suite's 300 ms unlatch sleep as above). After the
 * await the source throws the wrapped "Support-graph-source generated
 * exception: TestException" RuntimeException. Java's start-mode source
 * runnable logs and swallows that throwable when no exception handler is
 * registered, so the Java oracle cannot observe the message and join()
 * returns normally with the state COMPLETE; the Go engine surfaces the source
 * error from Join and asserts the wrapped message in-process — that semantic
 * difference is documented in the scenario and the message text is NEVER
 * recorded. The recorded observables are the COMPLETE state and the empty
 * capture read.
 *
 * nonblocking-exception (ordinal 2) deploys the same byte-exact graph with a
 * source instructed [MyRuntimeException("TestException")]; start() lets the
 * source throw immediately and the suite's sleep(200) is replayed as a bounded
 * sleep-poll for the COMPLETE state (10 s deadline). The capture read is
 * empty.
 *
 * nonblocking-cancel (ordinal 5) deploys the same byte-exact graph shape with
 * DefaultSupportGraphOpProviderByOpName, a source instructed
 * [latch, Object[]{1}] and a plain capture. start(); strengthened RUNNING;
 * cancel(); strengthened CANCELLED (asserted before the countDown — cancel is
 * synchronous and already interrupted the latched await); countDown; the
 * suite's trailing sleep(100) is dropped (cancel is synchronous) and the
 * capture read is empty.
 *
 * nonblocking-join-multiple-runnable (ordinal 7) deploys the byte-exact
 * two-named-source graph ('{ name: 'SourceOne' }' and '{ name: 'SourceTwo' }'
 * clauses concatenated with NO separators) and instantiates through
 * DefaultSupportGraphOpProviderByOpName with SourceOne/SourceTwo each
 * instructed [latch, Object[]{1}] and one capture latched at 2 fed by both.
 * start(); strengthened RUNNING (the suite's sleep(50) is dropped — start()
 * sets RUNNING synchronously); SourceOne released; strengthened still-RUNNING
 * (the suite's sleep(200) is dropped — the property is deterministic because
 * SourceTwo stays latched); SourceTwo released; join() completes; the capture
 * holds 2 batches (2 rows total).
 *
 * nonblocking-join-single-runnable (ordinal 9) deploys the byte-exact
 * single-source graph with DefaultSupportGraphOpProvider, a source instructed
 * [latch, Object[]{1}] and a capture latched at 1. The name 'MyDataFlowOne'
 * and the INSTANTIATED entry state are asserted in-process; start(); the
 * suite's sleep(100) is dropped and RUNNING is strengthened; release; join()
 * completes; the first capture batch holds exactly 1 row and the source
 * consumed both pre-marker instructions (getCurrentCount()==2, in-process);
 * the trailing cancel() is a silent Java no-op after COMPLETE
 * (EPDataFlowInstanceImpl.cancel returns for COMPLETE) while the Go Cancel
 * errors there — frozen disposition, no record.
 *
 * fast-complete-nonblocking (ordinal 12) deploys the byte-exact BeaconSource
 * graph (iterations:1 with the capture clause concatenated with NO separator,
 * no schema preamble) with a capture latched at 1 through
 * DefaultSupportGraphOpProvider. The INSTANTIATED entry state and the not-done
 * capture are asserted in-process; start(); the suite's 1 s busy-wait for the
 * COMPLETE state is replayed byte-exact; the capture read holds exactly 1 row
 * (the pre-marker batch is still the current batch — the final marker is not
 * bound to this capture channel); then tryAssertionAfterExec replays
 * in-process: join() no-op, run()/start() IllegalStateException with the
 * byte-exact already-completed message (asserted in-process), cancel() and
 * join() silent no-ops. The Go side asserts the same transitions with its
 * Cancel-on-Complete erroring — frozen, no record.
 *
 * Record protocol (dataflow state/count conventions): state records are
 * {case, operation:"state", statement:"flow", sequence, time,
 * name:"instance.state", value:UPPERCASE EPDataFlowState name} and count
 * records are {case, operation:"count", statement:"flow", sequence, time,
 * name, count}. Capture reads are recorded through the count operation only
 * (capture-empty for the cancelled/exception cases, capture-rows for the
 * completing cases — the observable is the row count being zero/one/two, not
 * the capture contents). Error texts are NEVER recorded: the wrapped Java
 * source-throw message and the IllegalStateException texts are asserted
 * in-process, and the Go-side error texts stay unrecorded on both sides
 * (state-only recording). Sequence is case-local and restarts at 1; the time
 * is the fixed epoch 1970-01-01T00:00:00Z. Each case runs on its own fresh
 * runtime (internal timer off, epoch initialization) that is undeployAll'd
 * after the case body and destroyed in finally; the dataflow deployment id is
 * pinned 'flow' but never recorded, while the SomeType-schema deployments use
 * engine-generated ids. Session configuration mirrors
 * TestSuiteEPLDataflow.configure restricted to these executions: only the
 * com.espertech.esper.common.internal.epl.dataflow.util.* package import that
 * resolves DefaultSupportSourceOp / DefaultSupportCaptureOp in EPL text is
 * required (BeaconSource resolves through the built-in
 * ConfigurationCommon.DATAFLOWOPERATOR_IMPORT and the SomeType type is
 * declared by the deployed byte-exact EPL preamble over the RegressionPath).
 */
public final class DataflowLifecycleCancelJoinScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "dataflow-lifecycle-cancel-join";
    private static final String PINNED_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/dataflow/EPLDataflowAPIRunStartCancelJoin.java";
    private static final String CASE_NONBLOCKING_JOIN_CANCEL = "nonblocking-join-cancel";
    private static final String CASE_NONBLOCKING_JOIN_EXCEPTION = "nonblocking-join-exception";
    private static final String CASE_NONBLOCKING_EXCEPTION = "nonblocking-exception";
    private static final String CASE_NONBLOCKING_CANCEL = "nonblocking-cancel";
    private static final String CASE_NONBLOCKING_JOIN_MULTIPLE = "nonblocking-join-multiple-runnable";
    private static final String CASE_NONBLOCKING_JOIN_SINGLE = "nonblocking-join-single-runnable";
    private static final String CASE_FAST_COMPLETE_NONBLOCKING = "fast-complete-nonblocking";
    private static final String[] CASES = {
            CASE_NONBLOCKING_JOIN_CANCEL, CASE_NONBLOCKING_JOIN_EXCEPTION,
            CASE_NONBLOCKING_EXCEPTION, CASE_NONBLOCKING_CANCEL,
            CASE_NONBLOCKING_JOIN_MULTIPLE, CASE_NONBLOCKING_JOIN_SINGLE,
            CASE_FAST_COMPLETE_NONBLOCKING};
    private static final int[] ORDINALS = {0, 1, 2, 5, 7, 9, 12};
    private static final String[] RUNTIME_IDS = {
            "java-runtime-893c8283ee90019f37fa",
            "java-runtime-e1bdb19779e28fa49bc2",
            "java-runtime-69dbb4e0d879421b52f7",
            "java-runtime-f053fbdc01fad03ba5a6",
            "java-runtime-827b0af4ea14c59da2bc",
            "java-runtime-a9a21d69ccb78f9fd152",
            "java-runtime-de4fc2179a2cb72a97e6"};
    private static final String[] EXECUTION_NAMES = {
            "EPLDataflowNonBlockingJoinCancel",
            "EPLDataflowNonBlockingJoinException",
            "EPLDataflowNonBlockingException",
            "EPLDataflowNonBlockingCancel",
            "EPLDataflowNonBlockingJoinMultipleRunnable",
            "EPLDataflowNonBlockingJoinSingleRunnable",
            "EPLDataflowFastCompleteNonBlocking"};
    private static final int[] RECORD_COUNTS = {2, 2, 2, 3, 4, 3, 2};
    private static final int TOTAL_RECORDS = 18;
    private static final String FLOW_DEPLOYMENT_ID = "flow";
    private static final String FLOW_NAME_ONE = "MyDataFlowOne";
    private static final String EPOCH = "1970-01-01T00:00:00Z";
    private static final long BOUNDED_WAIT_MS = 10000L;

    // Byte-exact graphs from EPLDataflowAPIRunStartCancelJoin: the source and
    // multiple-source graphs concatenate the DefaultSupportCaptureOp clause
    // with NO separator after the preceding clause's trailing '{}' (lines
    // 54-56, 243-245, 330-333); the fast-complete graph concatenates the
    // capture clause with NO separator after the BeaconSource clause's
    // trailing '}' (lines 618-620). The '{ name: 'SourceOne' }' parameter
    // braces carry exact interior spaces (line 331).
    private static final String SOME_TYPE_EPL = "@public create schema SomeType ()";
    private static final String SOURCE_CAPTURE_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "DefaultSupportSourceOp -> outstream<SomeType> {}" +
            "DefaultSupportCaptureOp(outstream) {}";
    private static final String MULTIPLE_SOURCE_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "DefaultSupportSourceOp -> outstream<SomeType> { name: 'SourceOne' }" +
            "DefaultSupportSourceOp -> outstream<SomeType> { name: 'SourceTwo' }" +
            "DefaultSupportCaptureOp(outstream) {}";
    private static final String BEACON_FLOW_EPL =
            "@name('flow') create dataflow " + FLOW_NAME_ONE + " " +
            "BeaconSource -> BeaconStream {iterations : 1}" +
            "DefaultSupportCaptureOp(BeaconStream) {}";

    // Byte-exact Java message texts asserted in-process, never recorded.
    private static final String MSG_AFTER_COMPLETE =
            "Data flow 'MyDataFlowOne' instance has already completed, please use instantiate to run the data flow again";
    private static final String MSG_SOURCE_GENERATED = "Support-graph-source generated exception: TestException";

    private DataflowLifecycleCancelJoinScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            System.err.println("usage: DataflowLifecycleCancelJoinScenarioOracle <scenario.json>");
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
            throw new IllegalArgumentException("scenario must contain the seven selected cases");
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
        // text. BeaconSource resolves through the built-in
        // ConfigurationCommon.DATAFLOWOPERATOR_IMPORT; the SomeType type is
        // declared by the deployed byte-exact EPL preamble over the
        // RegressionPath. Internal timer off, epoch initialization.
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addImport(DefaultSupportSourceOp.class.getPackage().getName() + ".*");

        EPRuntime runtime = EPRuntimeProvider.getRuntime(
                "parity-dataflow-lifecycle-cancel-join-" + RUNTIME_IDS[caseIndex], configuration);
        ((EPRuntimeSPI) runtime).initialize(0L);
        try {
            TraceWriter writer = new TraceWriter(records, caseName);
            switch (caseIndex) {
                case 0:
                    runNonBlockingJoinCancel(configuration, runtime, writer);
                    break;
                case 1:
                    runNonBlockingJoinException(configuration, runtime, writer);
                    break;
                case 2:
                    runNonBlockingException(configuration, runtime, writer);
                    break;
                case 3:
                    runNonBlockingCancel(configuration, runtime, writer);
                    break;
                case 4:
                    runNonBlockingJoinMultipleRunnable(configuration, runtime, writer);
                    break;
                case 5:
                    runNonBlockingJoinSingleRunnable(configuration, runtime, writer);
                    break;
                default:
                    runFastCompleteNonBlocking(configuration, runtime, writer);
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
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowNonBlockingJoinCancel
     * (ordinal 0): cancel from a side thread ends the latched source and
     * join() returns with the instance CANCELLED and an empty capture.
     */
    private static void runNonBlockingJoinCancel(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 52-56: the byte-exact SomeType preamble and the
        // two-clause graph (capture clause concatenated with NO separator)
        // over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, SOURCE_CAPTURE_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 58-62: the source is instructed [latch] — the suite
        // never releases this latch; the cancel interruption is the only
        // unblock. The capture is a plain (unlatched) capture.
        final CountDownLatch latchOne = new CountDownLatch(1);
        DefaultSupportSourceOp src = new DefaultSupportSourceOp(new Object[]{latchOne});
        DefaultSupportCaptureOp<Object> output = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(src, output));
        final EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite line 64: start launches the source thread; RUNNING is a
        // strengthened assert (the suite asserts no state here).
        dfOne.start();
        assertState(dfOne, EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());

        // suite lines 66-81: the cancelling thread (the suite sleeps 300 ms —
        // a wait device replayed as a bounded cancel: poll for RUNNING within
        // the deadline, then cancel) interrupts the latched await and join()
        // returns.
        Thread cancellingThread = new Thread(new Runnable() {
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
                dfOne.cancel();
            }
        }, "EPLDataflowNonBlockingJoinCancel-cancelling");
        cancellingThread.start();
        dfOne.join();

        // suite line 83: the cancelled state (strengthened assert, not
        // recorded in this case).
        assertState(dfOne, EPDataFlowState.CANCELLED);

        // suite line 84: the capture read is empty.
        int captured = output.getAndReset().size();
        if (captured != 0) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 0");
        }
        writer.addCount("capture-empty", captured);

        // suite line 86: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowNonBlockingJoinException
     * (ordinal 1): the start-mode source runnable logs and swallows the
     * wrapped source throwable, so join() returns normally with the instance
     * COMPLETE and an empty capture.
     */
    private static void runNonBlockingJoinException(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 96-100: the byte-exact SomeType preamble and the
        // two-clause graph over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, SOURCE_CAPTURE_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 102-106: the source throws the wrapped
        // MyRuntimeException after the latch. The wrapped message text is
        // never surfaced by the Java start-mode (no handler registered) and
        // is never recorded; the Go side asserts it in-process against the
        // frozen constant MSG_SOURCE_GENERATED.
        final CountDownLatch latchOne = new CountDownLatch(1);
        DefaultSupportSourceOp src = new DefaultSupportSourceOp(
                new Object[]{latchOne, new EPLDataflowAPIRunStartCancelJoin.MyRuntimeException("TestException")});
        DefaultSupportCaptureOp<Object> output = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(src, output));
        final EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite line 108: start.
        dfOne.start();

        // suite lines 110-125: the unlatching thread (the suite sleeps 300 ms
        // — replayed as a bounded release: poll for RUNNING within the
        // deadline, then countDown).
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
                latchOne.countDown();
            }
        }, "EPLDataflowNonBlockingJoinException-unlatching");
        unlatchingThread.start();
        dfOne.join();

        // suite line 127: the start-mode swallow completes the instance.
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite line 128: the capture read is empty.
        int captured = output.getAndReset().size();
        if (captured != 0) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 0");
        }
        writer.addCount("capture-empty", captured);

        // suite line 129: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowNonBlockingException
     * (ordinal 2): the immediate source throw completes the instance while
     * the capture stays empty.
     */
    private static void runNonBlockingException(Configuration configuration, EPRuntime runtime, TraceWriter writer)
            throws Exception {
        // suite lines 138-142: the byte-exact SomeType preamble and the
        // two-clause graph over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, SOURCE_CAPTURE_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 145-148: the source throws the wrapped
        // MyRuntimeException immediately (instruction 0).
        DefaultSupportSourceOp src = new DefaultSupportSourceOp(
                new Object[]{new EPLDataflowAPIRunStartCancelJoin.MyRuntimeException("TestException")});
        DefaultSupportCaptureOp<Object> output = new DefaultSupportCaptureOp<Object>();
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(src, output));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite lines 150-152: the suite sleeps 200 ms before asserting
        // COMPLETE — replayed as a bounded sleep-poll with the same outcome.
        dfOne.start();
        awaitState(dfOne, EPDataFlowState.COMPLETE, "nonblocking-exception");

        // suite line 152: the completed state.
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite line 153: the capture read is empty.
        int captured = output.getAndReset().size();
        if (captured != 0) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 0");
        }
        writer.addCount("capture-empty", captured);

        // suite line 154: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowNonBlockingCancel (ordinal
     * 5): cancel of the RUNNING instance is synchronous — the state is
     * CANCELLED before the source latch is released and the capture stays
     * empty.
     */
    private static void runNonBlockingCancel(Configuration configuration, EPRuntime runtime,
                                             TraceWriter writer) throws Exception {
        // suite lines 242-246: the byte-exact SomeType preamble and the
        // two-clause graph over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, SOURCE_CAPTURE_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 250-256: named operators through
        // DefaultSupportGraphOpProviderByOpName; the source is instructed
        // [latch, Object[]{1}] — the Object[] is submitted as one row when
        // reached (it is not reached: the cancel interrupt wins).
        CountDownLatch latchOne = new CountDownLatch(1);
        Map<String, Object> ops = new HashMap<String, Object>();
        ops.put("DefaultSupportSourceOp", new DefaultSupportSourceOp(new Object[]{latchOne, new Object[]{1}}));
        DefaultSupportCaptureOp<Object> output = new DefaultSupportCaptureOp<Object>();
        ops.put("DefaultSupportCaptureOp", output);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(ops));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite lines 259-260: start and the RUNNING state.
        dfOne.start();
        assertState(dfOne, EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());

        // suite lines 262-266: cancel is synchronous, so CANCELLED is
        // asserted (and recorded) before the latch release; the suite's
        // trailing sleep(100) is dropped.
        dfOne.cancel();
        assertState(dfOne, EPDataFlowState.CANCELLED);
        writer.addState(dfOne.getState());
        latchOne.countDown();

        // suite line 267: the capture read is empty.
        int captured = output.getAndReset().size();
        if (captured != 0) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 0");
        }
        writer.addCount("capture-empty", captured);

        // suite line 268: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin
     * EPLDataflowNonBlockingJoinMultipleRunnable (ordinal 7): releasing one
     * of two latched sources keeps the instance RUNNING; releasing the second
     * completes it with two captured batches.
     */
    private static void runNonBlockingJoinMultipleRunnable(Configuration configuration, EPRuntime runtime,
                                                            TraceWriter writer) throws Exception {
        // suite lines 327-333: the byte-exact SomeType preamble and the
        // two-named-source graph (exact interior spaces in the parameter
        // braces, clauses concatenated with NO separators) over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, MULTIPLE_SOURCE_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 335-344: SourceOne and SourceTwo each instructed
        // [latch, Object[]{1}]; the capture is latched at 2 rows (a wait
        // device — the read records the full batch state).
        CountDownLatch latchOne = new CountDownLatch(1);
        CountDownLatch latchTwo = new CountDownLatch(1);
        Map<String, Object> ops = new HashMap<String, Object>();
        ops.put("SourceOne", new DefaultSupportSourceOp(new Object[]{latchOne, new Object[]{1}}));
        ops.put("SourceTwo", new DefaultSupportSourceOp(new Object[]{latchTwo, new Object[]{1}}));
        DefaultSupportCaptureOp<Object> future = new DefaultSupportCaptureOp<Object>(2);
        ops.put("DefaultSupportCaptureOp", future);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProviderByOpName(ops));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite lines 347-349: start and the RUNNING state; the suite's
        // sleep(50) is dropped — start() sets RUNNING synchronously.
        dfOne.start();
        assertState(dfOne, EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());

        // suite lines 351-353: SourceOne released; the instance is STILL
        // RUNNING — a deterministic property because SourceTwo stays latched
        // (the suite's sleep(200) is dropped).
        latchOne.countDown();
        assertState(dfOne, EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());

        // suite lines 355-361: SourceTwo released; join() completes.
        latchTwo.countDown();
        dfOne.join();
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite line 362: the capture read holds 2 batches (2 rows total).
        int captured = future.getAndReset().size();
        if (captured != 2) {
            throw new IllegalStateException("capture held " + captured + " batches, expected 2");
        }
        writer.addCount("capture-rows", captured);

        // suite line 363: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin
     * EPLDataflowNonBlockingJoinSingleRunnable (ordinal 9): the single-source
     * start/join round trip delivers one row and the trailing cancel after
     * COMPLETE is a silent Java no-op (Go errors — frozen disposition, no
     * record).
     */
    private static void runNonBlockingJoinSingleRunnable(Configuration configuration, EPRuntime runtime,
                                                          TraceWriter writer) throws Exception {
        // suite lines 423-427: the byte-exact SomeType preamble and the
        // two-clause graph over the path.
        RegressionPath path = new RegressionPath();
        EPCompiled schemaCompiled = compile(configuration, runtime, SOME_TYPE_EPL, path);
        runtime.getDeploymentService().deploy(schemaCompiled);
        path.add(schemaCompiled);
        EPCompiled flowCompiled = compile(configuration, runtime, SOURCE_CAPTURE_FLOW_EPL, path);
        deploy(runtime, flowCompiled, FLOW_DEPLOYMENT_ID);
        path.add(flowCompiled);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 430-435: the source is instructed [latch, Object[]{1}];
        // the capture is latched at 1 row.
        CountDownLatch latch = new CountDownLatch(1);
        DefaultSupportSourceOp source = new DefaultSupportSourceOp(new Object[]{latch, new Object[]{1}});
        DefaultSupportCaptureOp<Object> future = new DefaultSupportCaptureOp<Object>(1);
        EPDataFlowInstantiationOptions options = new EPDataFlowInstantiationOptions()
                .operatorProvider(new DefaultSupportGraphOpProvider(source, future));
        EPDataFlowInstance dfOne = runtime.getDataFlowService().instantiate(
                flowDeploymentId, FLOW_NAME_ONE, options);

        // suite lines 436-437: the dataflow name and the INSTANTIATED entry
        // state (in-process asserts, not recorded).
        if (!FLOW_NAME_ONE.equals(dfOne.getDataFlowName())) {
            throw new IllegalStateException("dataflow name was " + dfOne.getDataFlowName()
                    + ", expected " + FLOW_NAME_ONE);
        }
        assertState(dfOne, EPDataFlowState.INSTANTIATED);

        // suite lines 439-441: start and the RUNNING state; the suite's
        // sleep(100) is dropped — start() sets RUNNING synchronously.
        dfOne.start();
        assertState(dfOne, EPDataFlowState.RUNNING);
        writer.addState(dfOne.getState());

        // suite lines 443-449: release; join() completes.
        latch.countDown();
        dfOne.join();
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite line 450: the first capture batch holds exactly 1 row.
        List<List<Object>> batches = future.getAndReset();
        if (batches.isEmpty() || batches.get(0).size() != 1) {
            throw new IllegalStateException("first capture batch was "
                    + (batches.isEmpty() ? "absent" : Integer.toString(batches.get(0).size()))
                    + ", expected 1 row");
        }
        writer.addCount("capture-rows", batches.get(0).size());

        // suite line 451: the source consumed the latch await and the row
        // submit (currentCount 2 after the final marker call).
        if (source.getCurrentCount() != 2) {
            throw new IllegalStateException("source currentCount was " + source.getCurrentCount()
                    + ", expected 2");
        }

        // suite lines 453-454: cancel after COMPLETE is a silent Java no-op
        // (EPDataFlowInstanceImpl.cancel returns for COMPLETE) and the state
        // stays COMPLETE; the Go Cancel errors there — frozen disposition,
        // no record.
        dfOne.cancel();
        assertState(dfOne, EPDataFlowState.COMPLETE);

        // suite line 455: undeployAll happens at the case boundary.
    }

    /**
     * EPLDataflowAPIRunStartCancelJoin EPLDataflowFastCompleteNonBlocking
     * (ordinal 12): the BeaconSource iterations:1 flow completes immediately
     * after start and the post-execution join/run/start/cancel state machine
     * holds.
     */
    private static void runFastCompleteNonBlocking(Configuration configuration, EPRuntime runtime,
                                                    TraceWriter writer) throws Exception {
        // suite lines 618-620: byte-exact BeaconSource graph — the capture
        // clause is concatenated with NO separator after the source clause's
        // trailing '}' and no schema preamble is needed.
        deploy(runtime, compile(configuration, runtime, BEACON_FLOW_EPL, null), FLOW_DEPLOYMENT_ID);
        String flowDeploymentId = deploymentId(runtime, "flow");
        if (!FLOW_DEPLOYMENT_ID.equals(flowDeploymentId)) {
            throw new IllegalStateException("deployment id for statement 'flow' was " + flowDeploymentId
                    + ", expected " + FLOW_DEPLOYMENT_ID);
        }

        // suite lines 623-628: the capture is latched at 1 row; the
        // INSTANTIATED entry state, the dataflow name and the not-done
        // capture are in-process asserts.
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
        if (future.isDone()) {
            throw new IllegalStateException("capture reported done before the flow started");
        }

        // suite lines 631-637: non-blocking start and the byte-exact 1 s
        // busy-wait for COMPLETE.
        dfOne.start();
        long start = System.currentTimeMillis();
        while (dfOne.getState() != EPDataFlowState.COMPLETE) {
            if (System.currentTimeMillis() - start > 1000) {
                throw new IllegalStateException("timed out waiting for the COMPLETE state");
            }
        }

        // strengthened record point: the completed state.
        assertState(dfOne, EPDataFlowState.COMPLETE);
        writer.addState(dfOne.getState());

        // suite lines 638-642: the capture read holds exactly 1 row (the
        // pre-marker batch is still the current batch — the final marker is
        // not bound to this capture channel).
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

        // suite lines 645 and 654-683: tryAssertionAfterExec — cancel and
        // join ignored, run/start rejected with the byte-exact message
        // (asserted in-process), cancel and join silent no-ops. The Go side
        // asserts the same transitions with its Cancel-on-Complete erroring
        // (frozen, no record).
        tryAssertionAfterExec(dfOne);

        // suite line 646: undeployAll happens at the case boundary.
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
     * name:"instance.state", value} records for the instance-state reads and
     * {case, operation:"count", statement:"flow", sequence, time, name,
     * count} records for the capture reads; the sequence is case-local.
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

        private void addState(EPDataFlowState state) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "state")
                    .add("statement", "flow")
                    .add("sequence", ++sequence)
                    .add("time", EPOCH);
            record.add("name", "instance.state");
            record.add("value", state.name());
            records.add(record);
        }

        private long count() {
            return sequence;
        }
    }
}
