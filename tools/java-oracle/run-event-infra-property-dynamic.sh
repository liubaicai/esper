#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-event-infra-property-dynamic.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "Esper checkout commit $actual_commit does not match the pinned oracle $expected_commit" >&2
    exit 1
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

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-event-infra-property-dynamic.XXXXXX")
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
    MINGW*|MSYS*|CYGWIN*)
        cp_sep=';'
        native_path() { cygpath -w "$1"; }
        ;;
    *)
        cp_sep=':'
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
regression_cp=$(tr -d '\r\n' < "$work/regression-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp$cp_sep$regression_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/EventInfraPropertyDynamicScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EventInfraPropertyDynamicScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and .id == "event-infra-property-dynamic" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 879) and
    ([.records[] | select(.case == "dynamic-simple")] | length == 37) and
    ([.records[] | select(.case == "dynamic-nested")] | length == 460) and
    ([.records[] | select(.case == "nested-deep")] | length == 126) and
    ([.records[] | select(.case == "rooted-nonsimple")] | length == 110) and
    ([.records[] | select(.case == "rooted-simple")] | length == 68) and
    ([.records[] | select(.case == "nonsimple")] | length == 78) and
    ([.records[] | select(.case == "dynamic-simple" and .operation == "listener" and .statement == "s0")] |
        .[0].new[0].fields.myid == "e1" and .[0].new[0].fields.exists_myid == true) and
    ([.records[] | select(.case == "dynamic-simple" and .operation == "listener" and .statement == "s0")] |
        .[2].new[0].fields.exists_myid == false) and
    ([.records[] | select(.case == "dynamic-simple" and .operation == "listener" and .statement == "s0")] |
        .[6].new[0].fields.myid.state == "null" and .[6].new[0].fields.exists_myid == true) and
    ([.records[] | select(.operation == "types")] | length == 344) and
    ([.records[] | select(.operation == "getter")] | length == 106) and
    ([.records[] | select(.case == "dynamic-nested" and .operation == "getter" and .name == "item.id?")] |
        length == 106) and
    ([.records[] | select(.case == "dynamic-nested" and .operation == "getter" and .name == "item.id?" and .value.exists == false)] |
        length > 0) and
    ([.records[] | select(.case == "nested-deep" and .operation == "listener" and .statement == "s0")] |
        .[0].new[0].fields.n1 == "nestedValue" and .[0].new[0].fields.n6 == "nestedNestedValue") and
    ([.records[] | select(.case == "rooted-nonsimple" and .operation == "listener" and .statement == "s0")] |
        .[1].new[0].fields.exists_array == true and .[1].new[0].fields.exists_map == true) and
    ([.records[] | select(.case == "rooted-nonsimple" and .operation == "listener" and .statement == "s0")] |
        .[0].new[0].fields.exists_array == false and .[0].new[0].fields.exists_map == false) and
    ([.records[] | select(.case == "rooted-simple" and .operation == "listener" and .statement == "s0")] |
        .[0].new[0].fields.simple == "simple" and .[0].new[0].fields.nestedNested == "nestedNestedValue") and
    ([.records[] | select(.case == "nonsimple" and .operation == "listener" and .statement == "s0")] |
        .[0].new[0].fields.indexed1 == 1 and .[0].new[0].fields.mapped1 == "valueOne"
        and .[1].new[0].fields.exists_indexed1 == false)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid event-infra-property-dynamic trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
