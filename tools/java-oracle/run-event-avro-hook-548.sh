#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-event-avro-hook-548.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
        --esper-root)   esper_root=$2; shift 2 ;;
        --scenario)     scenario=$2;   shift 2 ;;
        --output)       output=$2;     shift 2 ;;
        --skip-build)   skip_build=1;  shift ;;
        -h|--help)      usage ;;
        *)              echo "unknown argument: $1" >&2; usage ;;
    esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ]   || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ]     || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ]   || { echo "scenario was not found: $scenario" >&2; exit 1; }

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "could not resolve the Esper checkout commit" >&2; exit 1;
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is at $actual_commit, expected $expected_commit" >&2; exit 1
fi

javac_bin=${JAVAC:-javac}
java_bin=${JAVA:-java}
mvn_bin=${MAVEN:-mvn}
if [ -n "${JAVA_HOME:-}" ]; then
    javac_bin=$JAVA_HOME/bin/javac
    java_bin=$JAVA_HOME/bin/java
fi
if [ -n "${MAVEN_HOME:-}" ]; then
    mvn_bin=$MAVEN_HOME/bin/mvn
fi

command -v "$javac_bin" >/dev/null 2>&1 || { echo "javac was not found: $javac_bin" >&2; exit 1; }
command -v "$java_bin"  >/dev/null 2>&1 || { echo "java was not found: $java_bin" >&2; exit 1; }
command -v "$mvn_bin"   >/dev/null 2>&1 || { echo "mvn was not found: $mvn_bin" >&2; exit 1; }
command -v jq           >/dev/null 2>&1 || { echo "jq was not found" >&2; exit 1; }

java_version=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([^"]*\)".*/\1/p' | head -n 1)
case $java_version in
    17*) ;;
    *) echo "Java 17 is required for the Esper 9.0.0 oracle, found: $java_version" >&2; exit 1 ;;
esac

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-event-avro-hook-548.XXXXXX")
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
"$mvn_bin" -q -f "$esper_root/regression-run/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/regression-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) cp_sep=';' ;;
    *)                    cp_sep=':' ;;
esac
native_path() {
    case "$cp_sep" in
        ';') (cd "$1" >/dev/null 2>&1 && pwd -W) || printf '%s' "$1" ;;
        *)   printf '%s' "$1" ;;
    esac
}
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
regression_cp=$(tr -d '\r\n' < "$work/regression-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp$cp_sep$regression_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/EventAvroHook548ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EventAvroHook548ScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and .id == "event-avro-hook-548" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 14) and
    ([.records[] | select(.case == "property-coerce")] | length == 3) and
    ([.records[] | select(.case == "schema-from-class")] | length == 3) and
    ([.records[] | select(.case == "populate")] | length == 3) and
    ([.records[] | select(.case == "named-window-property-assignment")] | length == 5) and
    ([.records[] | select(.operation == "compile-error")] | length == 1) and
    ([.records[] | select(.operation == "deployed")] | length == 6) and
    ([.records[] | select(.operation == "listener")] | length == 3) and
    ([.records[] | select(.operation == "value")] | length == 3) and
    ([.records[] | select(.operation == "snapshot")] | length == 1) and
    ([.records[] | select(.operation == "compile-error")][0].value ==
        "Invalid assignment of column '"'"'isodate'"'"' of type '"'"'java.time.ZonedDateTime'"'"' to event property '"'"'isodate'"'"' typed as '"'"'java.lang.CharSequence'"'"', column and parameter types mismatch") and
    ([.records[] | select(.case == "property-coerce" and .operation == "listener")] |
        .[0].new[0].fields.isodate == "2026-09-27T01:02:03.123456789") and
    ([.records[] | select(.name == "avro-schema")][0].value ==
        "{\"type\":\"record\",\"name\":\"MyEventOut\",\"fields\":[{\"name\":\"isodate\",\"type\":\"string\"}]}") and
    ([.records[] | select(.case == "schema-from-class" and .operation == "listener")] |
        .[0].new[0].fields.isodate == "<isodate>") and
    ([.records[] | select(.case == "populate" and .name == "avroToJson")][0].value ==
        "{\"sb\":{\"theString\":\"E1\",\"intPrimitive\":10}}") and
    ([.records[] | select(.case == "named-window-property-assignment" and .operation == "snapshot")] |
        .[0].new[0].fields.sb.SupportBeanSchema.theString == "E1" and
        .[0].new[0].fields.sb.SupportBeanSchema.intPrimitive == 10) and
    ([.records[] | select(.case == "named-window-property-assignment" and .name == "avroToJson")][0].value ==
        "{\"sb\":{\"SupportBeanSchema\":{\"theString\":\"E1\",\"intPrimitive\":10}}}")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid event-avro-hook-548 trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
