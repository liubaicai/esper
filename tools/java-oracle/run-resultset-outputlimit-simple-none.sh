#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-outputlimit-simple-none.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided. The
scenario must be resultset-outputlimit-simple-none.
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
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
    exit 1
fi

if [ -n "${JAVA_HOME:-}" ]; then
    javac_bin="$JAVA_HOME/bin/javac"
    java_bin="$JAVA_HOME/bin/java"
else
    javac_bin=$(command -v javac)
    java_bin=$(command -v java)
fi
if [ -n "${MAVEN_HOME:-}" ]; then
    mvn_bin="$MAVEN_HOME/bin/mvn"
else
    mvn_bin=$(command -v mvn)
fi
java_version=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([0-9]*\).*/\1/p' | head -1)
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-outputlimit-simple-none" and
    (.steps | type == "array") and
    (.steps | length) == 320 and
    ([.steps[] | select(.op == "case")] | length) == 4 and
    ([.steps[] | select(.op == "advance-time")] | length) == 204 and
    ([.steps[] | select(.op == "deploy")] | length) == 8 and
    ([.steps[] | select(.op == "undeploy-all")] | length) == 8 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length) == 72 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length) == 24 and
    ([.steps[] | select(.op != "case" and .op != "advance-time" and .op != "deploy" and .op != "undeploy-all" and .op != "send")] | length) == 0
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-outputlimit-simple-none replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 \
        -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-outputlimit-simple-none.XXXXXX")
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
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ResultSetOutputLimitSimpleNoneScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetOutputLimitSimpleNoneScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-outputlimit-simple-none" and
    (.records | type == "array") and
    (.records | length) == 62 and
    ([.records[] | select(.case == "none-no-having-no-join" and .operation == "listener")] | length) == 21 and
    ([.records[] | select(.case == "none-no-having-join" and .operation == "listener")] | length) == 21 and
    ([.records[] | select(.case == "none-having-no-join" and .operation == "listener")] | length) == 10 and
    ([.records[] | select(.case == "none-having-join" and .operation == "listener")] | length) == 10 and
    ([.records[] | select(.statement == "s0")] | length) == 62
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
