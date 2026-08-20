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
import com.espertech.esper.regressionlib.support.bean.SupportBeanComplexProps;
import com.espertech.esper.regressionlib.support.bean.SupportBeanDynRoot;
import com.espertech.esper.regressionlib.support.bean.SupportBean_A;
import com.espertech.esper.regressionlib.support.bean.SupportMarkerInterface;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;
import com.espertech.esper.runtime.internal.kernel.service.EPRuntimeSPI;

import java.io.FileReader;
import java.time.Instant;
import java.util.Arrays;

/** Direct Esper 9.0.0 oracle for replayable ExprCoreExists executions. */
public final class ExprCoreExistsCastScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String SCENARIO_ID = "expr-core-exists-cast";
    private static final String[] CASES = {
            "exists-simple", "exists-inner", "exists-om", "exists-compile",
            "cast-simple", "cast-simple-more-types", "cast-as-parse", "cast-double-null-om",
            "cast-string-and-null", "cast-boolean", "cast-w-static-type",
            "cast-bigdecimal-bigint"
    };
    private static final String[] EVENT_TYPES = {
            "SupportBean", "SupportMarkerInterface", "SupportMarkerInterface", "SupportMarkerInterface",
            "SupportBean", "SupportBean", "SupportBean", "SupportBeanDynRoot",
            "SupportBeanDynRoot", "SupportBean", "StaticTypeMapEvent", "MyEvent"
    };
    private static final int[] SEND_COUNTS = {1, 5, 3, 3, 2, 1, 1, 6, 6, 3, 1, 8};

    private ExprCoreExistsCastScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        String scenarioFile = args.length > 0 ? args[0] : "testdata/parity/expr-core-exists-cast.json";
        JsonObject scenario = Json.parse(new FileReader(scenarioFile)).asObject();
        if (!VERSION.equals(scenario.getString("version", ""))) {
            throw new IllegalArgumentException("unsupported scenario version");
        }
        if (!SCENARIO_ID.equals(scenario.getString("id", ""))) {
            throw new IllegalArgumentException("unsupported scenario id " + scenario.getString("id", ""));
        }
        JsonValue stepsValue = scenario.get("steps");
        if (stepsValue == null || !stepsValue.isArray()) {
            throw new IllegalArgumentException("scenario steps are required");
        }
        JsonArray steps = stepsValue.asArray();
        validateScenario(steps);

        JsonObject trace = new JsonObject()
                .add("version", VERSION)
                .add("id", SCENARIO_ID)
                .add("records", new JsonArray());
        JsonArray records = trace.get("records").asArray();
        for (String caseName : CASES) {
            runCase(steps, caseName, records);
        }
        System.out.println(trace);
    }

    private static void validateScenario(JsonArray steps) {
        int offset = 0;
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            if (offset >= steps.size()) {
                throw new IllegalArgumentException("scenario is missing case " + CASES[caseIndex]);
            }
            JsonObject marker = steps.get(offset++).asObject();
            if (!"case".equals(marker.getString("op", "")) ||
                    !CASES[caseIndex].equals(marker.getString("case", ""))) {
                throw new IllegalArgumentException("scenario case order mismatch at " + caseIndex);
            }
            requireStepFields(marker, "case", "op", "case");
            for (int sendIndex = 0; sendIndex < SEND_COUNTS[caseIndex]; sendIndex++) {
                if (offset >= steps.size()) {
                    throw new IllegalArgumentException("scenario is missing send for " + CASES[caseIndex]);
                }
                JsonObject step = steps.get(offset++).asObject();
                if (!"send".equals(step.getString("op", "")) ||
                        !EVENT_TYPES[caseIndex].equals(step.getString("eventType", ""))) {
                    throw new IllegalArgumentException("scenario send shape mismatch for " + CASES[caseIndex]);
                }
                requireStepFields(step, "send", "op", "eventType", "payload");
                JsonValue payloadValue = step.get("payload");
                if (payloadValue == null || !payloadValue.isObject()) {
                    throw new IllegalArgumentException("scenario payload is required for " + CASES[caseIndex]);
                }
                validatePayload(payloadValue.asObject(), caseIndex, sendIndex);
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario has trailing steps");
        }
    }

    private static void requireStepFields(JsonObject step, String kind, String... allowed) {
        if (step.names().size() != allowed.length) {
            throw new IllegalArgumentException("scenario " + kind + " has unsupported metadata");
        }
        for (String name : allowed) {
            if (!step.names().contains(name)) {
                throw new IllegalArgumentException("scenario " + kind + " is missing field " + name);
            }
        }
    }


    private static void validatePayload(JsonObject payload, int caseIndex, int sendIndex) {
        if (caseIndex == 0) {
            requireFieldCount(payload, 4, CASES[caseIndex]);
            requireString(payload, "theString", "abc");
            requireNumber(payload, "intPrimitive", 100);
            requireNumber(payload, "intBoxed", 3);
            requireDouble(payload, "floatBoxed", 9.5);
            return;
        }
		if (caseIndex == 4) {
			requireFieldCount(payload, sendIndex == 0 ? 4 : 4, CASES[caseIndex]);
			if (sendIndex == 0) {
				requireString(payload, "theString", "abc");
				requireNumber(payload, "intPrimitive", 100);
				requireNumber(payload, "intBoxed", 3);
				requireDouble(payload, "floatBoxed", 9.5);
			} else {
				requireNull(payload, "theString");
				requireNumber(payload, "intPrimitive", 100);
				requireNull(payload, "intBoxed");
				requireNull(payload, "floatBoxed");
			}
			return;
		}
		if (caseIndex == 5) {
			requireFieldCount(payload, 3, CASES[caseIndex]);
			requireString(payload, "theString", "true");
			requireNumber(payload, "intPrimitive", 1);
			requireNumber(payload, "doublePrimitive", 1);
			return;
		}
		if (caseIndex == 6) {
			requireFieldCount(payload, 2, CASES[caseIndex]);
			requireString(payload, "theString", "12");
			requireNumber(payload, "intPrimitive", 1);
			return;
		}
		if (caseIndex == 7 || caseIndex == 8) {
			String[] itemTypes = {"int", "byte", "double", "int64", "null", "string"};
			requireFieldCount(payload, sendIndex == 4 ? 1 : 2, CASES[caseIndex]);
			requireString(payload, "itemType", itemTypes[sendIndex]);
			if (sendIndex == 0) {
				requireNumber(payload, "itemValue", 100);
			} else if (sendIndex == 1) {
				requireNumber(payload, "itemValue", 2);
			} else if (sendIndex == 2) {
				requireDouble(payload, "itemValue", 77.7777);
			} else if (sendIndex == 3) {
				requireNumber(payload, "itemValue", 6);
			} else if (sendIndex == 5) {
				requireString(payload, "itemValue", "abc");
			}
			return;
		}
		if (caseIndex == 9) {
			requireFieldCount(payload, 4, CASES[caseIndex]);
			if (sendIndex == 0) {
				requireString(payload, "theString", "abc");
				requireNumber(payload, "intPrimitive", 100);
				requireBoolean(payload, "boolPrimitive", true);
				requireBoolean(payload, "boolBoxed", true);
			} else if (sendIndex == 1) {
				requireNull(payload, "theString");
				requireNumber(payload, "intPrimitive", 100);
				requireBoolean(payload, "boolPrimitive", false);
				requireBoolean(payload, "boolBoxed", false);
			} else {
				requireNull(payload, "theString");
				requireNumber(payload, "intPrimitive", 100);
				requireBoolean(payload, "boolPrimitive", true);
				requireNull(payload, "boolBoxed");
			}
			return;
		}
		if (caseIndex == 10) {
			requireFieldCount(payload, 8, CASES[caseIndex]);
			requireString(payload, "anInt", "100");
			requireString(payload, "anDouble", "1.4E-1");
			requireString(payload, "anLong", "-10");
			requireString(payload, "anFloat", "1.001");
			requireString(payload, "anByte", "0x0A");
			requireString(payload, "anShort", "223");
			requireNumber(payload, "intPrimitive", 10);
			requireNumber(payload, "intBoxed", 11);
			return;
		}
		if (caseIndex == 11) {
			String kind = payload.getString("kind", "");
			switch (kind) {
				case "int":
				case "long":
				case "double":
					requireStepFields(payload, "send", "kind", "value");
					if (!payload.get("value").isNumber()) {
						throw new IllegalArgumentException("payload " + kind + " kind must carry a number");
					}
					break;
				case "decimal":
				case "bigint":
					requireStepFields(payload, "send", "kind", "text");
					break;
				case "pow2-decimal":
				case "pow2-bigint":
					requireStepFields(payload, "send", "kind", "exponent");
					if (!payload.get("exponent").isNumber()) {
						throw new IllegalArgumentException("payload pow2 kind must carry a numeric exponent");
					}
					break;
				case "null":
					requireStepFields(payload, "send", "kind");
					break;
				default:
					throw new IllegalArgumentException("unsupported value kind " + kind);
			}
			return;
		}

		String[] shapes = caseIndex == 1
				? new String[]{"null", "complex", "complex", "nested-support-bean", "support-bean-a"}
				: new String[]{"support-bean", "null", "string"};
        requireFieldCount(payload, 1, CASES[caseIndex]);
        requireString(payload, "shape", shapes[sendIndex]);
    }


	private static void requireBoolean(JsonObject payload, String name, boolean expected) {
		JsonValue value = payload.get(name);
		if (value == null || !value.isBoolean() || value.asBoolean() != expected) {
			throw new IllegalArgumentException("payload boolean mismatch for " + name);
		}
	}
    private static void requireFieldCount(JsonObject payload, int expected, String caseName) {
        if (payload.names().size() != expected) {
            throw new IllegalArgumentException("payload field count mismatch for " + caseName);
        }
    }

    private static void requireNumber(JsonObject payload, String name, long expected) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull() || !value.isNumber() || value.asLong() != expected) {
            throw new IllegalArgumentException("payload number mismatch for " + name);
        }
    }

    private static void requireDouble(JsonObject payload, String name, double expected) {
        JsonValue value = payload.get(name);
        if (value == null || value.isNull() || !value.isNumber() || Double.compare(value.asDouble(), expected) != 0) {
            throw new IllegalArgumentException("payload decimal mismatch for " + name);
        }
    }

    private static void requireString(JsonObject payload, String name, String expected) {
        JsonValue value = payload.get(name);
        if (value == null || !value.isString() || !expected.equals(value.asString())) {
            throw new IllegalArgumentException("payload string mismatch for " + name);
        }
    }

	private static void requireNull(JsonObject payload, String name) {
		JsonValue value = payload.get(name);
		if (value == null || !value.isNull()) {
			throw new IllegalArgumentException("payload null mismatch for " + name);
		}
	}

    private static void runCase(JsonArray allSteps, String caseName, JsonArray records) throws Exception {
        Configuration configuration = new Configuration();
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getCommon().addEventType("SupportBean", SupportBean.class);
        configuration.getCommon().addEventType("SupportMarkerInterface", SupportMarkerInterface.class);
        configuration.getCommon().addEventType("SupportBeanDynRoot", SupportBeanDynRoot.class);
		configuration.getCommon().addEventType("StaticTypeMapEvent", staticTypeMapEventMap());
        String runtimeName = "parity-expr-core-exists-cast-" + caseName;
		configuration.getCommon().addEventType("MyEvent", myEventMap());
        EPRuntime runtime = EPRuntimeProvider.getRuntime(runtimeName, configuration);
        try {
            ((EPRuntimeSPI) runtime).initialize(0L);
            EPCompiled compiled = EPCompilerProvider.getCompiler().compile(
                    "@name('s0') " + eplFor(caseName),
                    new CompilerArguments(runtime.getRuntimePath()));
            EPDeployment deployment = runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
            EPStatement statement = null;
            for (EPStatement candidate : deployment.getStatements()) {
                if ("s0".equals(candidate.getName())) {
                    statement = candidate;
                    break;
                }
            }
            if (statement == null) {
                throw new IllegalStateException("statement s0 was not deployed");
            }
            statement.addListener(new TraceWriter(records, caseName, statement, runtime));
            replayCase(allSteps, caseName, runtime);
            runtime.getDeploymentService().undeployAll();
        } finally {
            runtime.destroy();
        }
    }

    private static java.util.Map<String, Object> staticTypeMapEventMap() {
        java.util.Map<String, Object> map = new java.util.HashMap<>();
        map.put("anInt", String.class);
        map.put("anDouble", String.class);
        map.put("anLong", String.class);
        map.put("anFloat", String.class);
        map.put("anByte", String.class);
        map.put("anShort", String.class);
        map.put("intPrimitive", int.class);
        map.put("intBoxed", Integer.class);
        return map;
    }

    private static java.util.Map<String, Object> myEventMap() {
        java.util.Map<String, Object> map = new java.util.HashMap<>();
        map.put("value", Object.class);
        return map;
    }

    private static String eplFor(String caseName) {
        if ("exists-simple".equals(caseName)) {
            return "select exists(theString) as c0, exists(intBoxed?) as c1, " +
                    "exists(dummy?) as c2, exists(intPrimitive?) as c3, " +
                    "exists(intPrimitive) as c4 from SupportBean";
        }
        if ("exists-inner".equals(caseName)) {
            return "select exists(item?.id) as t0, " +
                    "exists(item?.id?) as t1, " +
                    "exists(item?.item.intBoxed) as t2, " +
                    "exists(item?.indexed[0]?) as t3, " +
                    "exists(item?.mapped('keyOne')?) as t4, " +
                    "exists(item?.nested?) as t5, " +
                    "exists(item?.nested.nestedValue?) as t6, " +
                    "exists(item?.nested.nestedNested?) as t7, " +
                    "exists(item?.nested.nestedNested.nestedNestedValue?) as t8, " +
                    "exists(item?.nested.nestedNested.nestedNestedValue.dummy?) as t9, " +
                    "exists(item?.nested.nestedNested.dummy?) as t10 " +
                    "from SupportMarkerInterface";
        }
        if ("exists-om".equals(caseName) || "exists-compile".equals(caseName)) {
            return "select exists(item?.intBoxed) as t0 from SupportMarkerInterface";
        }
		if ("cast-simple".equals(caseName)) {
			return "select cast(theString as string) as c0, cast(intBoxed, int) as c1, " +
					"cast(floatBoxed, java.lang.Float) as c2, cast(theString, java.lang.String) as c3, " +
					"cast(intPrimitive, java.lang.Integer) as c4, cast(intPrimitive, long) as c5, " +
					"cast(intPrimitive, java.lang.Number) as c6, cast(floatBoxed, long) as c7 from SupportBean";
		}
		if ("cast-simple-more-types".equals(caseName)) {
			return "select cast(intPrimitive, float) as c0, cast(intPrimitive, short) as c1, " +
					"cast(intPrimitive, byte) as c2, cast(theString, char) as c3, " +
					"cast(theString, boolean) as c4, cast(intPrimitive, BigInteger) as c5, " +
					"cast(intPrimitive, BigDecimal) as c6, cast(doublePrimitive, BigDecimal) as c7, " +
					"cast(theString, char) as c8 from SupportBean";
		}
		if ("cast-as-parse".equals(caseName)) {
			return "select cast(theString, int) as t0 from SupportBean";
		}
		if ("cast-double-null-om".equals(caseName) || "cast-string-and-null".equals(caseName)) {
			String target = "cast-double-null-om".equals(caseName) ? "double" : "java.lang.String";
			return "select cast(item?," + target + ") as t0 from SupportBeanDynRoot";
		}
		if ("cast-boolean".equals(caseName)) {
			return "select cast(boolPrimitive as java.lang.Boolean) as t0, " +
					"cast(boolBoxed | boolPrimitive, boolean) as t1, " +
					"cast(boolBoxed, string) as t2 from SupportBean";
		}
		if ("cast-w-static-type".equals(caseName)) {
			return "select cast(anInt, int) as intVal, cast(anDouble, double) as doubleVal, " +
					"cast(anLong, long) as longVal, cast(anFloat, float) as floatVal, " +
					"cast(anByte, byte) as byteVal, cast(anShort, short) as shortVal, " +
					"cast(intPrimitive, int) as intOne, cast(intBoxed, int) as intTwo, " +
					"cast(intPrimitive, java.lang.Long) as longOne, cast(intBoxed, long) as longTwo " +
					"from StaticTypeMapEvent";
		}
		if ("cast-bigdecimal-bigint".equals(caseName)) {
			return "select cast(value, BigDecimal) as c0, cast(value, BigInteger) as c1 from MyEvent";
		}
        throw new IllegalArgumentException("unsupported case " + caseName);
    }
    private static void replayCase(JsonArray allSteps, String caseName, EPRuntime runtime) {
        boolean active = false;
        for (JsonValue value : allSteps) {
            JsonObject step = value.asObject();
            String op = step.getString("op", "");
            if ("case".equals(op)) {
                active = caseName.equals(step.getString("case", ""));
                continue;
            }
            if (!active || !"send".equals(op)) {
                continue;
            }
            JsonObject payload = step.get("payload").asObject();
            if ("SupportBean".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(toSupportBean(payload), "SupportBean");
            } else if ("SupportMarkerInterface".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(toDynamicRoot(payload), "SupportMarkerInterface");
            } else if ("SupportBeanDynRoot".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventBean(toCastDynamicRoot(payload), "SupportBeanDynRoot");
            } else if ("StaticTypeMapEvent".equals(step.getString("eventType", ""))) {
                runtime.getEventService().sendEventMap(toStaticTypeMapEvent(payload), "StaticTypeMapEvent");
            } else if ("MyEvent".equals(step.getString("eventType", ""))) {
                java.util.Map<String, Object> map = new java.util.HashMap<>();
                map.put("value", toMyEventValue(payload));
                runtime.getEventService().sendEventMap(map, "MyEvent");
            } else {
                throw new IllegalArgumentException("unsupported event type");
            }
        }
    }

    private static SupportBean toSupportBean(JsonObject payload) {
		JsonValue theString = payload.get("theString");
		SupportBean bean = new SupportBean(theString == null || theString.isNull() ? null : theString.asString(), payload.getInt("intPrimitive", 0));
		JsonValue intBoxed = payload.get("intBoxed");
		bean.setIntBoxed(intBoxed == null || intBoxed.isNull() ? null : payload.getInt("intBoxed", 0));
		JsonValue floatBoxed = payload.get("floatBoxed");
		bean.setFloatBoxed(floatBoxed == null || floatBoxed.isNull() ? null : (float) payload.getDouble("floatBoxed", 0.0));
		JsonValue doublePrimitive = payload.get("doublePrimitive");
		if (doublePrimitive != null && !doublePrimitive.isNull()) {
			bean.setDoublePrimitive(payload.getDouble("doublePrimitive", 0.0));
		}
		JsonValue boolPrimitive = payload.get("boolPrimitive");
		if (boolPrimitive != null && boolPrimitive.isBoolean()) {
			bean.setBoolPrimitive(boolPrimitive.asBoolean());
		}
		JsonValue boolBoxed = payload.get("boolBoxed");
		if (boolBoxed != null && !boolBoxed.isNull() && boolBoxed.isBoolean()) {
			bean.setBoolBoxed(boolBoxed.asBoolean());
		}
        return bean;
    }

    private static java.util.Map<String, Object> toStaticTypeMapEvent(JsonObject payload) {
        java.util.Map<String, Object> map = new java.util.HashMap<>();
        JsonValue anInt = payload.get("anInt");
        map.put("anInt", anInt == null || anInt.isNull() ? null : anInt.asString());
        JsonValue anDouble = payload.get("anDouble");
        map.put("anDouble", anDouble == null || anDouble.isNull() ? null : anDouble.asString());
        JsonValue anLong = payload.get("anLong");
        map.put("anLong", anLong == null || anLong.isNull() ? null : anLong.asString());
        JsonValue anFloat = payload.get("anFloat");
        map.put("anFloat", anFloat == null || anFloat.isNull() ? null : anFloat.asString());
        JsonValue anByte = payload.get("anByte");
        map.put("anByte", anByte == null || anByte.isNull() ? null : anByte.asString());
        JsonValue anShort = payload.get("anShort");
        map.put("anShort", anShort == null || anShort.isNull() ? null : anShort.asString());
        map.put("intPrimitive", payload.getInt("intPrimitive", 0));
        JsonValue intBoxed = payload.get("intBoxed");
        map.put("intBoxed", intBoxed == null || intBoxed.isNull() ? null : payload.getInt("intBoxed", 0));
        return map;
    }
    private static Object toMyEventValue(JsonObject payload) {
        String kind = payload.getString("kind", "");
        switch (kind) {
            case "int":
                return payload.getInt("value", 0);
            case "long":
                return payload.getLong("value", 0L);
            case "double":
                return payload.getDouble("value", 0.0);
            case "decimal":
                return new java.math.BigDecimal(payload.getString("text", "0"));
            case "bigint":
                return new java.math.BigInteger(payload.getString("text", "0"));
            case "pow2-decimal":
                java.math.BigInteger base = java.math.BigInteger.valueOf(2);
                java.math.BigDecimal pow = new java.math.BigDecimal(base.pow(payload.getInt("exponent", 0)));
                return pow.add(new java.math.BigDecimal("0.1"));
            case "pow2-bigint":
                return java.math.BigInteger.valueOf(2).pow(payload.getInt("exponent", 0));
            case "null":
                return null;
            default:
                throw new IllegalArgumentException("unsupported value kind " + kind);
        }
    }
	private static SupportBeanDynRoot toCastDynamicRoot(JsonObject payload) {
		String itemType = payload.getString("itemType", "");
		JsonValue itemValue = payload.get("itemValue");
		switch (itemType) {
			case "int":
				return new SupportBeanDynRoot(payload.getInt("itemValue", 0));
			case "byte":
				return new SupportBeanDynRoot((byte) payload.getInt("itemValue", 0));
			case "double":
				return new SupportBeanDynRoot(payload.getDouble("itemValue", 0.0));
			case "int64":
				return new SupportBeanDynRoot((long) payload.getLong("itemValue", 0L));
			case "null":
				return new SupportBeanDynRoot(null);
			case "string":
				return new SupportBeanDynRoot(itemValue == null ? "" : itemValue.asString());
			default:
				throw new IllegalArgumentException("unsupported cast item type " + itemType);
		}
	}

    private static SupportBeanDynRoot toDynamicRoot(JsonObject payload) {
        String shape = payload.getString("shape", "");
        switch (shape) {
            case "null":
                return new SupportBeanDynRoot(null);
            case "complex":
                return new SupportBeanDynRoot(SupportBeanComplexProps.makeDefaultBean());
            case "nested-support-bean":
                return new SupportBeanDynRoot(new SupportBeanDynRoot(new SupportBean()));
            case "support-bean-a":
                return new SupportBeanDynRoot(new SupportBean_A("10"));
            case "support-bean":
                return new SupportBeanDynRoot(new SupportBean());
            case "string":
                return new SupportBeanDynRoot("abc");
            default:
                throw new IllegalArgumentException("unsupported shape " + shape);
        }
    }

    private static final class TraceWriter implements UpdateListener {
        private final JsonArray records;
        private final String caseName;
        private final EPStatement statement;
        private final EPRuntime runtime;
        private long sequence;

        private TraceWriter(JsonArray records, String caseName, EPStatement statement, EPRuntime runtime) {
            this.records = records;
            this.caseName = caseName;
            this.statement = statement;
            this.runtime = runtime;
        }

        @Override
        public void update(EventBean[] newEvents, EventBean[] oldEvents, EPStatement ignored, EPRuntime ignoredRuntime) {
            JsonObject record = new JsonObject()
                    .add("case", caseName)
                    .add("operation", "listener")
                    .add("statement", statement.getName())
                    .add("sequence", ++sequence)
                    .add("time", Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newArray = results(newEvents);
            if (newArray.size() > 0) {
                record.add("new", newArray);
            }
            if (oldEvents != null && oldEvents.length > 0) {
                record.add("old", results(oldEvents));
            }
            records.add(record);
        }

        private JsonArray results(EventBean[] events) {
            JsonArray output = new JsonArray();
            if (events == null) {
                return output;
            }
            for (EventBean event : events) {
                JsonObject fields = new JsonObject();
                String[] names = event.getEventType().getPropertyNames().clone();
                Arrays.sort(names);
                for (String name : names) {
                    fields.add(name, normalize(event.get(name)));
                }
                output.add(new JsonObject().add("kind", "row").add("fields", fields));
            }
            return output;
        }

        private JsonValue normalize(Object value) {
            if (value == null) {
                return new JsonObject().add("state", "null");
            }
            if (value instanceof Boolean) {
                return Json.value((Boolean) value);
            }
            return Json.value(String.valueOf(value));
        }
    }
}
