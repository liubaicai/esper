#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-expr-core-exists-cast.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
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
        --esper-root) [ "$#" -ge 2 ] || usage; esper_root=$2; shift 2 ;;
        --scenario) [ "$#" -ge 2 ] || usage; scenario=$2; shift 2 ;;
        --output) [ "$#" -ge 2 ] || usage; output=$2; shift 2 ;;
        --skip-build) skip_build=1; shift ;;
        -h|--help) usage ;;
        *) echo "unknown argument: $1" >&2; usage ;;
    esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

if [ -n "$(git -C "$esper_root" status --porcelain --untracked-files=all 2>/dev/null)" ]; then
    echo "Esper checkout has uncommitted or untracked changes: $esper_root" >&2
    exit 1
fi

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
[ "$actual_commit" = "$expected_commit" ] || {
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
    exit 1
}

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
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found" >&2; exit 1; }

java_version=$($java_bin -version 2>&1 | awk -F'[".]' '/version/ {print $2; exit}')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "expr-core-exists-cast" and
    (.steps | type == "array") and
    ([.steps[] | select(.op == "case") | .case] == [
        "exists-simple", "exists-inner", "exists-om", "exists-compile",
        "cast-simple", "cast-simple-more-types", "cast-as-parse", "cast-double-null-om",
        "cast-interface", "cast-string-and-null", "cast-boolean", "cast-w-static-type",
        "cast-bigdecimal-bigint", "cast-warray", "cast-warray-soda"
    ]) and
    ([.steps[] | select(.op == "send")] | length) == 49 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length) == 8 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarkerInterface")] | length) == 11 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBeanDynRoot")] | length) == 17 and
    ([.steps[] | select(.op == "send" and .eventType == "StaticTypeMapEvent")] | length) == 1 and
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent")] | length) == 8 and
    ([.steps[] | select(.op == "send" and .eventType == "MyEventWArray")] | length) == 4
    ' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid expr-core-exists-cast replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-expr-core-exists-cast.XXXXXX")
cleanup() {
    find "$work" -type f -delete 2>/dev/null || true
    rmdir "$work" 2>/dev/null || true
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

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ExprCoreExistsCastScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ExprCoreExistsCastScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "expr-core-exists-cast" and
    (.records | type == "array") and
    ((.records | length) == 49) and
    ([.records[] | select(.operation == "listener" and .statement == "s0" and
        (.new | type == "array" and length == 1) and
        ((.old // []) | length == 0) and
        .new[0].kind == "row")] | length) == 49 and
    ([.records[] | select(.case == "exists-simple")] | length) == 1 and
    ([.records[] | select(.case == "exists-inner")] | length) == 5 and
    ([.records[] | select(.case == "exists-om")] | length) == 3 and
    ([.records[] | select(.case == "exists-compile")] | length) == 3 and
    ([.records[] | select(.case == "cast-simple")] | length) == 2 and
    ([.records[] | select(.case == "cast-simple-more-types")] | length) == 1 and
    ([.records[] | select(.case == "cast-as-parse")] | length) == 1 and
    ([.records[] | select(.case == "cast-double-null-om")] | length) == 6 and
    ([.records[] | select(.case == "cast-interface")] | length) == 5 and
    ([.records[] | select(.case == "cast-string-and-null")] | length) == 6 and
    ([.records[] | select(.case == "cast-boolean")] | length) == 3 and
    ([.records[] | select(.case == "cast-w-static-type")] | length) == 1 and
    ([.records[] | select(.case == "cast-bigdecimal-bigint")] | length) == 8 and
    ([.records[] | select(.case == "cast-warray")] | length) == 2 and
    ([.records[] | select(.case == "cast-warray-soda")] | length) == 2 and
    ([.records[] | select(.case == "exists-simple") | (.new[0].fields | keys)] | unique) == [["c0", "c1", "c2", "c3", "c4"]] and
    ([.records[] | select(.case == "exists-inner") | (.new[0].fields | keys)] | unique) == [["t0", "t1", "t10", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9"]] and
    ([.records[] | select(.case == "exists-om" or .case == "exists-compile") | (.new[0].fields | keys)] | unique) == [["t0"]] and
    ([.records[] | select(.case == "cast-simple") | (.new[0].fields | keys)] | unique) == [["c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7"]] and
    ([.records[] | select(.case == "cast-simple-more-types") | (.new[0].fields | keys)] | unique) == [["c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"]] and
    ([.records[] | select(.case == "cast-as-parse" or .case == "cast-double-null-om" or .case == "cast-string-and-null") | (.new[0].fields | keys)] | unique) == [["t0"]] and
    ([.records[] | select(.case == "cast-boolean") | (.new[0].fields | keys)] | unique) == [["t0", "t1", "t2"]] and
    ([.records[] | select(.case == "cast-interface") | (.new[0].fields | keys)] | unique) == [["t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7"]] and
    ([.records[] | select(.case == "cast-w-static-type") | (.new[0].fields | keys)] | unique) == [["byteVal", "doubleVal", "floatVal", "intOne", "intTwo", "intVal", "longOne", "longTwo", "longVal", "shortVal"]] and
    ([.records[] | select(.case == "cast-bigdecimal-bigint") | (.new[0].fields | keys)] | unique) == [["c0", "c1"]] and
    ([.records[] | select(.case == "cast-warray" or .case == "cast-warray-soda") | (.new[0].fields | keys)] | unique) == [["c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"]]
    ' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
