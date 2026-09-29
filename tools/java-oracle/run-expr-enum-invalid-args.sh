#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-expr-enum-invalid-args.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java 9.0.0 oracle commit. Java 17
and Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
EOF
    exit 2
}

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

while [ "$#" -gt 0 ]; do
    case "$1" in
        --esper-root)
            [ "$#" -ge 2 ] || usage
            esper_root=$2
            shift 2
            ;;
        --scenario)
            [ "$#" -ge 2 ] || usage
            scenario=$2
            shift 2
            ;;
        --output)
            [ "$#" -ge 2 ] || usage
            output=$2
            shift 2
            ;;
        --skip-build)
            skip_build=1
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage
            ;;
    esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 1; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || { echo "cannot read HEAD" >&2; exit 1; }
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper root commit $actual_commit does not match pinned $expected_commit" >&2
    exit 1
fi

java_bin=${JAVA:-java}
javac_bin=${JAVAC:-javac}
mvn_bin=${MAVEN:-mvn}
if [ -n "${JAVA_HOME:-}" ]; then
    java_bin="$JAVA_HOME/bin/java"
    javac_bin="$JAVA_HOME/bin/javac"
fi
if [ -n "${MAVEN_HOME:-}" ]; then
    mvn_bin="$MAVEN_HOME/bin/mvn"
fi
command -v "$java_bin" >/dev/null 2>&1 || { echo "Java executable was not found: $java_bin" >&2; exit 1; }
command -v "$javac_bin" >/dev/null 2>&1 || { echo "javac executable was not found: $javac_bin" >&2; exit 1; }
command -v "$mvn_bin" >/dev/null 2>&1 || { echo "Maven executable was not found: $mvn_bin" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found; it is required to validate the scenario and oracle trace" >&2; exit 1; }

java_version=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "expr-enum-invalid-args" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMax.java" and
    (.description | test("ExprEnumInvalid")) and
    (.description | test("ExprEnumMinMaxByInvalid")) and
    (.description | test("ExprEnumOrderByInvalid")) and
    (.description | test("ExprEnumTakeInvalid")) and
    (.description | test("ExprEnumTakeWhileInvalid")) and
    (.javaSourceFiles == ["regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMax.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumMinMaxBy.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumOrderBy.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumTakeAndTakeLast.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumTakeWhileAndWhileLast.java"]) and
    (.javaRuntimes == ["java-runtime-67deab0a6d57e2bc57fa", "java-runtime-5556be14d552b0228acc", "java-runtime-f721caa1c77ad596eca4", "java-runtime-3c4f6374416fe5a2ee04", "java-runtime-50b5bc269985fbfd9ed2"]) and
    (.javaNames == ["ExprEnumInvalid", "ExprEnumMinMaxByInvalid", "ExprEnumOrderByInvalid", "ExprEnumTakeInvalid", "ExprEnumTakeWhileInvalid"]) and
    (.javaStaticIds == ["java-167415d6e7af87a7a111", "java-4083b006f50db9c59884", "java-0277b2cbf963ec07d2c0", "java-3f7b6e1e84fe78b4a816", "java-0fec7ea37236a9b16857"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["min-invalid", "minby-invalid", "orderby-invalid", "take-invalid", "takewhile-invalid"]) and
    ([.cases[].ordinal] == [4, 2, 4, 2, 2]) and
    ([.cases[].runtimeId] == ["java-runtime-67deab0a6d57e2bc57fa", "java-runtime-5556be14d552b0228acc", "java-runtime-f721caa1c77ad596eca4", "java-runtime-3c4f6374416fe5a2ee04", "java-runtime-50b5bc269985fbfd9ed2"]) and
    ([.cases[].executionName] == ["ExprEnumInvalid", "ExprEnumMinMaxByInvalid", "ExprEnumOrderByInvalid", "ExprEnumTakeInvalid", "ExprEnumTakeWhileInvalid"]) and
    # case metadata epl fields pinned byte-exact: each case lists its
    # first probe EPL.
    ([.cases[].epl] == ["select contained.min() from SupportBean_ST0_Container", "select contained.minBy(x => null) from SupportBean_ST0_Container", "select contained.orderBy() from SupportBean_ST0_Container", "select strvals.take(null) from SupportCollection", "select strvals.takeWhile(x => null) from SupportCollection"]) and
    # the case key set and the step key sets are pinned exactly: the Go
    # loader rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "op"] or . == ["case", "compileWithoutPath", "epl", "expectError", "op", "statement"])) and
    # per-case step counts: min-invalid = 1 case + 2 build-errors +
    # 1 undeploy-all = 4; minby-invalid = 1 + 1 + 1 = 3;
    # orderby-invalid = 1 + 2 + 1 = 4; take-invalid = 1 + 1 + 1 = 3;
    # takewhile-invalid = 1 + 1 + 1 = 3 (4 + 3 + 4 + 3 + 3 = 17 total).
    ([.steps[] | select(.case == "min-invalid")] | length == 4) and
    ([.steps[] | select(.case == "minby-invalid")] | length == 3) and
    ([.steps[] | select(.case == "orderby-invalid")] | length == 4) and
    ([.steps[] | select(.case == "take-invalid")] | length == 3) and
    ([.steps[] | select(.case == "takewhile-invalid")] | length == 3) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "build-error")] | length == 7) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 5) and
    # every probe uses the path-less env.tryInvalidCompile overload, so
    # all seven build-error steps carry compileWithoutPath=true.
    ([.steps[] | select(.op == "build-error" and (.compileWithoutPath == true))] | length == 7) and
    # probe order and epl/expectError pins per case.
    ([.steps[] | select(.op == "build-error" and .case == "min-invalid") | .statement] == ["min-event-input", "min-null-selector"]) and
    ([.steps[] | select(.op == "build-error" and .case == "min-invalid") | .epl] == ["select contained.min() from SupportBean_ST0_Container", "select contained.min(x => null) from SupportBean_ST0_Container"]) and
    ([.steps[] | select(.op == "build-error" and .case == "min-invalid") | .expectError] == ["Failed to validate select-clause expression '"'"'contained.min()'"'"': Invalid input for built-in enumeration method '"'"'min'"'"' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '"'"'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'"'"'", "Null-type is not allowed"]) and
    ([.steps[] | select(.op == "build-error" and .case == "minby-invalid") | .statement] == ["minby-null-selector"]) and
    ([.steps[] | select(.op == "build-error" and .case == "minby-invalid") | .epl] == ["select contained.minBy(x => null) from SupportBean_ST0_Container"]) and
    ([.steps[] | select(.op == "build-error" and .case == "minby-invalid") | .expectError] == ["Null-type is not allowed"]) and
    ([.steps[] | select(.op == "build-error" and .case == "orderby-invalid") | .statement] == ["orderby-event-input", "orderby-null-selector"]) and
    ([.steps[] | select(.op == "build-error" and .case == "orderby-invalid") | .epl] == ["select contained.orderBy() from SupportBean_ST0_Container", "select strvals.orderBy(v => null) from SupportCollection"]) and
    ([.steps[] | select(.op == "build-error" and .case == "orderby-invalid") | .expectError] == ["Failed to validate select-clause expression '"'"'contained.orderBy()'"'"': Invalid input for built-in enumeration method '"'"'orderBy'"'"' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '"'"'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'"'"'", "Null-type is not allowed"]) and
    ([.steps[] | select(.op == "build-error" and .case == "take-invalid") | .statement] == ["take-null-count"]) and
    ([.steps[] | select(.op == "build-error" and .case == "take-invalid") | .epl] == ["select strvals.take(null) from SupportCollection"]) and
    ([.steps[] | select(.op == "build-error" and .case == "take-invalid") | .expectError] == ["Failed to validate enumeration method '"'"'take'"'"', expected a non-null result for expression parameter 0 but received a null-typed expression"]) and
    ([.steps[] | select(.op == "build-error" and .case == "takewhile-invalid") | .statement] == ["takewhile-null-predicate"]) and
    ([.steps[] | select(.op == "build-error" and .case == "takewhile-invalid") | .epl] == ["select strvals.takeWhile(x => null) from SupportCollection"]) and
    ([.steps[] | select(.op == "build-error" and .case == "takewhile-invalid") | .expectError] == ["Failed to validate enumeration method '"'"'takeWhile'"'"', expected a non-null result for expression parameter 0 but received a null-typed expression"]) and
    (.steps | type == "array" and length == 17)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid expr-enum-invalid-args replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    if ! "$mvn_bin" -q -f "$esper_root/pom.xml" -DskipTests -Dgpg.skip=true \
        -pl common,compiler,runtime -am install >/dev/null; then
        echo "Esper build failed" >&2
        exit 1
    fi
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-expr-enum-invalid-args.XXXXXX")
cleanup() {
    rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"
# The Avro and Jackson jars are optional/provided-scope dependencies that
# the runtime classpath omits; resolve them from the local Maven
# repository like run-context-lifecycle.sh.
avro_jar=$(find "$HOME/.m2/repository/org/apache/avro/avro" -name 'avro-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
jackson_databind=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-databind" -name 'jackson-databind-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
jackson_core=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-core" -name 'jackson-core-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
jackson_ann=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-annotations" -name 'jackson-annotations-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
for jar in "$avro_jar" "$jackson_databind" "$jackson_core" "$jackson_ann"; do
    [ -n "$jar" ] && classpath="$classpath:$jar"
done

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ExprEnumInvalidArgsScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ExprEnumInvalidArgsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def cerr($c; $st; $v): {"case": $c, "operation": "compile-error", "statement": $st, "sequence": 0, "value": $v};
    def crec($c): [.records[] | select(.case == $c)];
    .version == "esper-parity/v1" and
    .id == "expr-enum-invalid-args" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 7) and
    ((crec("min-invalid") | length) == 2) and
    ((crec("minby-invalid") | length) == 1) and
    ((crec("orderby-invalid") | length) == 2) and
    ((crec("take-invalid") | length) == 1) and
    ((crec("takewhile-invalid") | length) == 1) and
    # min-invalid: two compile-error records carrying the pinned values
    # in probe order.
    (crec("min-invalid") == [
        cerr("min-invalid"; "min-event-input"; "Failed to validate select-clause expression '"'"'contained.min()'"'"': Invalid input for built-in enumeration method '"'"'min'"'"' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '"'"'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'"'"'"),
        cerr("min-invalid"; "min-null-selector"; "Null-type is not allowed")
    ]) and
    # minby-invalid: one compile-error record.
    (crec("minby-invalid") == [
        cerr("minby-invalid"; "minby-null-selector"; "Null-type is not allowed")
    ]) and
    # orderby-invalid: two compile-error records in probe order.
    (crec("orderby-invalid") == [
        cerr("orderby-invalid"; "orderby-event-input"; "Failed to validate select-clause expression '"'"'contained.orderBy()'"'"': Invalid input for built-in enumeration method '"'"'orderBy'"'"' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '"'"'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'"'"'"),
        cerr("orderby-invalid"; "orderby-null-selector"; "Null-type is not allowed")
    ]) and
    # take-invalid: one compile-error record.
    (crec("take-invalid") == [
        cerr("take-invalid"; "take-null-count"; "Failed to validate enumeration method '"'"'take'"'"', expected a non-null result for expression parameter 0 but received a null-typed expression")
    ]) and
    # takewhile-invalid: one compile-error record.
    (crec("takewhile-invalid") == [
        cerr("takewhile-invalid"; "takewhile-null-predicate"; "Failed to validate enumeration method '"'"'takeWhile'"'"', expected a non-null result for expression parameter 0 but received a null-typed expression")
    ])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid expr-enum-invalid-args trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
