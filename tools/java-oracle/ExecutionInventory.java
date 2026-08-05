import java.io.BufferedReader;
import java.io.IOException;
import java.io.PrintWriter;
import java.lang.reflect.Constructor;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.List;

/**
 * Enumerates Esper regression executions without running their test bodies.
 *
 * Input is tab-separated: id, outer class, outer class FQCN, source file.
 * The preferred source of executions is the outer class's static executions()
 * method. Direct RegressionExecution and RegressionExecutionPreConfigured
 * classes are recorded when no collection factory exists.
 */
public final class ExecutionInventory {
    private static final String EXECUTION_INTERFACE =
        "com.espertech.esper.regressionlib.framework.RegressionExecution";
    private static final String PRECONFIGURED_INTERFACE =
        "com.espertech.esper.regressionlib.framework.RegressionExecutionPreConfigured";

    private ExecutionInventory() {
    }

    public static void main(String[] args) throws IOException {
        if (args.length != 2) {
            System.err.println("usage: ExecutionInventory <input.tsv> <output.jsonl>");
            System.exit(2);
        }

        Path input = Path.of(args[0]);
        Path output = Path.of(args[1]);
        try (BufferedReader reader = Files.newBufferedReader(input, StandardCharsets.UTF_8);
             PrintWriter writer = new PrintWriter(Files.newBufferedWriter(output, StandardCharsets.UTF_8))) {
            String line;
            int lineNumber = 0;
            while ((line = reader.readLine()) != null) {
                lineNumber++;
                if (lineNumber == 1 && line.startsWith("\ufeff")) {
                    line = line.substring(1);
                }
                if (line.isBlank() || line.startsWith("#")) {
                    continue;
                }
                String[] fields = line.split("\\t", -1);
                if (fields.length != 4) {
                    writeError(writer, "", "", "", "", "", "input line " + lineNumber + " must have 4 tab-separated fields");
                    continue;
                }
                String id = fields[0];
                String outerClass = fields[1];
                String outerClassName = fields[2];
                String sourceFile = fields[3];
                try {
                    inspectOuter(writer, id, outerClass, outerClassName, sourceFile);
                } catch (Throwable error) {
                    writeError(writer, id, outerClass, outerClassName, sourceFile, "", describe(error));
                }
            }
        }
    }

    private static void inspectOuter(
        PrintWriter writer,
        String id,
        String outerClass,
        String outerClassName,
        String sourceFile) throws Exception {
        Class<?> type = Class.forName(outerClassName);
        Class<?> executionInterface = Class.forName(EXECUTION_INTERFACE, false, type.getClassLoader());
        Class<?> preconfiguredInterface = Class.forName(PRECONFIGURED_INTERFACE, false, type.getClassLoader());

        List<Method> factories = findExecutionsFactories(type);
        if (!factories.isEmpty()) {
            boolean emitted = false;
            for (Method factory : factories) {
                List<Object[]> calls = factoryArguments(factory);
                if (calls.isEmpty()) {
                    writeIgnored(writer, id, outerClass, outerClassName, sourceFile,
                        "executions() has unsupported parameters: " + factorySignature(factory));
                    continue;
                }
                for (Object[] arguments : calls) {
                    String variant = "collection:" + factorySignature(factory, arguments);
                    try {
                        factory.setAccessible(true);
                        Object value = factory.invoke(null, arguments);
                        if (!(value instanceof Iterable<?> iterable)) {
                            writeError(writer, id, outerClass, outerClassName, sourceFile, variant,
                                "executions() did not return an Iterable");
                            continue;
                        }
                        int ordinal = 0;
                        for (Object execution : iterable) {
                            writeExecution(writer, id, outerClass, outerClassName, sourceFile, variant, ordinal++, execution, executionInterface, preconfiguredInterface);
                            emitted = true;
                        }
                        if (ordinal == 0) {
                            writeIgnored(writer, id, outerClass, outerClassName, sourceFile, "executions() returned an empty collection: " + variant);
                        }
                    } catch (Throwable error) {
                        writeError(writer, id, outerClass, outerClassName, sourceFile, variant, describe(error));
                    }
                }
            }
            if (emitted) {
                return;
            }
            if (executionInterface.isAssignableFrom(type) && !type.isInterface() && !Modifier.isAbstract(type.getModifiers())) {
                Object execution = instantiate(type);
                writeExecution(writer, id, outerClass, outerClassName, sourceFile, "direct-fallback", 0, execution, executionInterface, preconfiguredInterface);
            }
            return;
        }

        if (executionInterface.isAssignableFrom(type)) {
            if (sourceFile.replace('\\', '/').contains("/support/")) {
                writeIgnored(writer, id, outerClass, outerClassName, sourceFile, "support helper is not a suite outer entry");
                return;
            }
            if (type.isInterface() || Modifier.isAbstract(type.getModifiers())) {
                writeIgnored(writer, id, outerClass, outerClassName, sourceFile, "abstract or interface execution class");
                return;
            }
            Object execution = instantiate(type);
            writeExecution(writer, id, outerClass, outerClassName, sourceFile, "direct", 0, execution, executionInterface, preconfiguredInterface);
            return;
        }

        if (preconfiguredInterface.isAssignableFrom(type)) {
            writeExecution(writer, id, outerClass, outerClassName, sourceFile, "direct-preconfigured", 0, null, executionInterface, preconfiguredInterface);
            return;
        }

        writeIgnored(writer, id, outerClass, outerClassName, sourceFile, "outer class is not a supported regression execution entry");
    }

    private static List<Method> findExecutionsFactories(Class<?> type) {
        List<Method> methods = new ArrayList<>();
        for (Method method : type.getDeclaredMethods()) {
            if (method.getName().equals("executions") && Modifier.isStatic(method.getModifiers())) {
                methods.add(method);
            }
        }
        methods.sort(Comparator.comparing(ExecutionInventory::factorySignature));
        return methods;
    }

    private static List<Object[]> factoryArguments(Method method) {
        Class<?>[] parameterTypes = method.getParameterTypes();
        if (parameterTypes.length == 0) {
            List<Object[]> arguments = new ArrayList<>();
            arguments.add(new Object[0]);
            return arguments;
        }
        for (Class<?> parameterType : parameterTypes) {
            if (parameterType != boolean.class && parameterType != Boolean.class) {
                return List.of();
            }
        }
        int combinations = 1 << parameterTypes.length;
        List<Object[]> arguments = new ArrayList<>(combinations);
        for (int mask = 0; mask < combinations; mask++) {
            Object[] values = new Object[parameterTypes.length];
            for (int i = 0; i < parameterTypes.length; i++) {
                values[i] = (mask & (1 << i)) != 0;
            }
            arguments.add(values);
        }
        return arguments;
    }

    private static String factorySignature(Method method) {
        return factorySignature(method, new Object[method.getParameterCount()]);
    }

    private static String factorySignature(Method method, Object[] arguments) {
        StringBuilder signature = new StringBuilder("executions(");
        for (int i = 0; i < arguments.length; i++) {
            if (i > 0) {
                signature.append(',');
            }
            signature.append(arguments[i] == null ? "?" : arguments[i]);
        }
        return signature.append(')').toString();
    }

    private static void writeExecution(
        PrintWriter writer,
        String id,
        String outerClass,
        String outerClassName,
        String sourceFile,
        String variant,
        int ordinal,
        Object execution,
        Class<?> executionInterface,
        Class<?> preconfiguredInterface) throws Exception {
        String executionClass = execution == null ? outerClassName : execution.getClass().getName();
        String name = null;
        List<String> flags = List.of();
        if (execution != null && executionInterface.isAssignableFrom(execution.getClass())) {
            Object value = executionInterface.getMethod("name").invoke(execution);
            name = value == null ? null : value.toString();
            flags = enumNames(executionInterface.getMethod("flags").invoke(execution));
        } else if (execution != null && !preconfiguredInterface.isAssignableFrom(execution.getClass())) {
            throw new IllegalArgumentException("executions() returned an unsupported object " + execution.getClass().getName());
        }

        StringBuilder json = baseRecord(id, outerClass, outerClassName, sourceFile, "ok");
        field(json, "runtimeId", runtimeId(id, outerClassName, variant, ordinal));
        field(json, "variant", variant);
        fieldNumber(json, "ordinal", ordinal);
        field(json, "executionClass", executionClass);
        field(json, "name", name);
        json.append(",\"flags\":[");
        for (int i = 0; i < flags.size(); i++) {
            if (i > 0) {
                json.append(',');
            }
            json.append('"').append(escape(flags.get(i))).append('"');
        }
        json.append("]}");
        writer.println(json);
    }

    private static String runtimeId(String id, String outerClassName, String variant, int ordinal) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] bytes = digest.digest((id + "\0" + outerClassName + "\0" + variant + "\0" + ordinal).getBytes(StandardCharsets.UTF_8));
            StringBuilder hex = new StringBuilder(20);
            for (int i = 0; i < 10; i++) {
                hex.append(String.format("%02x", bytes[i]));
            }
            return "java-runtime-" + hex;
        } catch (NoSuchAlgorithmException error) {
            throw new IllegalStateException("SHA-256 is unavailable", error);
        }
    }

    private static Object instantiate(Class<?> type) throws Exception {
        Constructor<?>[] constructors = type.getDeclaredConstructors();
        Arrays.sort(constructors, Comparator.comparingInt(Constructor::getParameterCount));
        Throwable last = null;
        for (Constructor<?> constructor : constructors) {
            try {
                constructor.setAccessible(true);
                Class<?>[] parameterTypes = constructor.getParameterTypes();
                Object[] arguments = new Object[parameterTypes.length];
                for (int i = 0; i < parameterTypes.length; i++) {
                    arguments[i] = defaultValue(parameterTypes[i]);
                }
                return constructor.newInstance(arguments);
            } catch (InvocationTargetException error) {
                last = error.getCause() == null ? error : error.getCause();
            } catch (Throwable error) {
                last = error;
            }
        }
        if (last instanceof Exception exception) {
            throw exception;
        }
        if (last instanceof Error error) {
            throw error;
        }
        throw new IllegalStateException("no usable constructor");
    }

    private static Object defaultValue(Class<?> type) {
        if (!type.isPrimitive()) {
            return null;
        }
        if (type == boolean.class) {
            return false;
        }
        if (type == byte.class) {
            return (byte) 0;
        }
        if (type == short.class) {
            return (short) 0;
        }
        if (type == int.class) {
            return 0;
        }
        if (type == long.class) {
            return 0L;
        }
        if (type == float.class) {
            return 0.0f;
        }
        if (type == double.class) {
            return 0.0d;
        }
        if (type == char.class) {
            return '\0';
        }
        throw new IllegalArgumentException("unsupported primitive constructor parameter " + type.getName());
    }

    private static List<String> enumNames(Object value) {
        List<String> names = new ArrayList<>();
        if (value instanceof Iterable<?> iterable) {
            for (Object item : iterable) {
                if (item != null) {
                    names.add(item.toString());
                }
            }
        } else if (value != null && value.getClass().isArray()) {
            int length = java.lang.reflect.Array.getLength(value);
            for (int i = 0; i < length; i++) {
                Object item = java.lang.reflect.Array.get(value, i);
                if (item != null) {
                    names.add(item.toString());
                }
            }
        } else if (value != null) {
            names.add(value.toString());
        }
        names.sort(String::compareTo);
        return names;
    }

    private static void writeIgnored(
        PrintWriter writer,
        String id,
        String outerClass,
        String outerClassName,
        String sourceFile,
        String reason) {
        StringBuilder json = baseRecord(id, outerClass, outerClassName, sourceFile, "ignored");
        field(json, "reason", reason);
        json.append('}');
        writer.println(json);
    }

    private static void writeError(
        PrintWriter writer,
        String id,
        String outerClass,
        String outerClassName,
        String sourceFile,
        String variant,
        String error) {
        StringBuilder json = baseRecord(id, outerClass, outerClassName, sourceFile, "error");
        if (!variant.isEmpty()) {
            field(json, "variant", variant);
        }
        field(json, "error", error);
        json.append('}');
        writer.println(json);
    }

    private static StringBuilder baseRecord(
        String id,
        String outerClass,
        String outerClassName,
        String sourceFile,
        String status) {
        StringBuilder json = new StringBuilder(256);
        json.append('{');
        field(json, "id", id);
        field(json, "outerClass", outerClass);
        field(json, "outerClassName", outerClassName);
        field(json, "sourceFile", sourceFile);
        field(json, "status", status);
        return json;
    }

    private static void field(StringBuilder json, String name, String value) {
        if (json.length() > 1) {
            json.append(',');
        }
        json.append('"').append(escape(name)).append("\":");
        if (value == null) {
            json.append("null");
        } else {
            json.append('"').append(escape(value)).append('"');
        }
    }

    private static void fieldNumber(StringBuilder json, String name, int value) {
        if (json.length() > 1) {
            json.append(',');
        }
        json.append('"').append(escape(name)).append("\":").append(value);
    }

    private static String describe(Throwable error) {
        Throwable current = error;
        while ((current instanceof ExceptionInInitializerError || current instanceof InvocationTargetException) && current.getCause() != null) {
            current = current.getCause();
        }
        String message = current.getMessage();
        if (message == null || message.isBlank()) {
            return current.getClass().getName();
        }
        return current.getClass().getName() + ": " + message;
    }

    private static String escape(String value) {
        StringBuilder escaped = new StringBuilder(value.length() + 16);
        for (int i = 0; i < value.length(); i++) {
            char character = value.charAt(i);
            switch (character) {
                case '"' -> escaped.append("\\\"");
                case '\\' -> escaped.append("\\\\");
                case '\b' -> escaped.append("\\b");
                case '\f' -> escaped.append("\\f");
                case '\n' -> escaped.append("\\n");
                case '\r' -> escaped.append("\\r");
                case '\t' -> escaped.append("\\t");
                default -> {
                    if (character < 0x20) {
                        escaped.append(String.format("\\u%04x", (int) character));
                    } else {
                        escaped.append(character);
                    }
                }
            }
        }
        return escaped.toString();
    }
}
