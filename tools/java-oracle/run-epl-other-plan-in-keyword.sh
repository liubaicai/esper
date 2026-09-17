#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-other-plan-in-keyword.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided. The
scenario must be epl-other-plan-in-keyword.
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
    echo "could not read Esper commit from $esper_root" >&2; exit 1; }
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper commit mismatch: expected $expected_commit, found $actual_commit" >&2
    exit 1
fi

if [ -n "${JAVA_HOME:-}" ]; then
    java_bin="$JAVA_HOME/bin/java"
    javac_bin="$JAVA_HOME/bin/javac"
else
    java_bin=java
    javac_bin=javac
fi
if [ -n "${MAVEN_HOME:-}" ]; then
    mvn_bin="$MAVEN_HOME/bin/mvn"
else
    mvn_bin=mvn
fi
java_version=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([0-9]*\).*/\1/p' | head -1)
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    (.steps | length) == 261 and
    .id == "epl-other-plan-in-keyword" and
    ([.steps[] | select(.op == "case")] | length) == 13 and
    ([.steps[] | select(.op == "deploy")] | length) == 44 and
    ([.steps[] | select(.op == "undeploy-all")] | length) == 30 and
    ([.steps[] | select(.op == "deployed")] | length) == 20 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length) == 76 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S1")] | length) == 78 and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "undeploy-all" and .op != "deployed" and .op != "send")] | length) == 0
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid epl-other-plan-in-keyword replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 \
        -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-other-plan-in-keyword.XXXXXX")
cleanup() {
    find "$work" -type f -delete 2>/dev/null || true
    find "$work" -depth -type d -empty -delete 2>/dev/null || true
}
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

# The pinned EPLs carry INDEX_CALLBACK_HOOK, which names the regression-lib
# class com.espertech.esper.regressionlib.support.util.SupportQueryPlanIndexHook.
# regression-lib is not on the oracle classpath, so the runner generates a
# compile-time no-op mirror implementing QueryPlanIndexHook; the suite's
# plan-shape assertions are harness-internal and emit no records.
stub_dir="$work/stub/com/espertech/esper/regressionlib/support/util"
mkdir -p "$stub_dir"
cat > "$stub_dir/SupportQueryPlanIndexHook.java" <<'EOF'
package com.espertech.esper.regressionlib.support.util;

import com.espertech.esper.common.internal.epl.join.queryplan.QueryPlanForge;
import com.espertech.esper.common.internal.epl.join.support.QueryPlanIndexDescFAF;
import com.espertech.esper.common.internal.epl.join.support.QueryPlanIndexDescHistorical;
import com.espertech.esper.common.internal.epl.join.support.QueryPlanIndexDescOnExpr;
import com.espertech.esper.common.internal.epl.join.support.QueryPlanIndexDescSubquery;
import com.espertech.esper.common.internal.epl.join.support.QueryPlanIndexHook;

/**
 * Compile-time no-op mirror of the regression-lib SupportQueryPlanIndexHook
 * named by INDEX_CALLBACK_HOOK in the pinned EPLOtherPlanInKeywordQuery EPLs.
 * The oracle replays observable behavior only; the suite's query-plan shape
 * assertions are harness-internal and emit no trace records.
 */
public class SupportQueryPlanIndexHook implements QueryPlanIndexHook {
    public void subquery(QueryPlanIndexDescSubquery subquery) {
    }

    public void infraOnExpr(QueryPlanIndexDescOnExpr onexpr) {
    }

    public void fireAndForget(QueryPlanIndexDescFAF queryPlanIndexDescFAF) {
    }

    public void join(QueryPlanForge join) {
    }

    public void historical(QueryPlanIndexDescHistorical historical) {
    }
}
EOF

classes="$work/classes"
mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$stub_dir/SupportQueryPlanIndexHook.java" \
    "$script_root/EPLOtherPlanInKeywordScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLOtherPlanInKeywordScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "epl-other-plan-in-keyword" and
    (.records | type == "array") and
    (.records | length) == 114 and
    ([.records[] | select(.case == "not-in" and .operation == "deployed")] | length) == 1 and
    ([.records[] | select(.case == "multi-idx-join" and .operation == "listener")] | length) == 11 and
    ([.records[] | select(.case == "multi-idx-window" and .operation == "listener")] | length) == 11 and
    ([.records[] | select(.case == "multi-idx-table" and .operation == "listener")] | length) == 11 and
    ([.records[] | select(.case == "multi-idx-subquery" and .operation == "listener")] | length) == 15 and
    ([.records[] | select(.case == "multi-idx-subquery" and .operation == "deployed")] | length) == 1 and
    ([.records[] | select(.case == "single-idx-join" and .operation == "listener")] | length) == 9 and
    ([.records[] | select(.case == "single-idx-window" and .operation == "listener")] | length) == 9 and
    ([.records[] | select(.case == "single-idx-table" and .operation == "listener")] | length) == 9 and
    ([.records[] | select(.case == "single-idx-subquery" and .operation == "listener")] | length) == 15 and
    ([.records[] | select(.case == "single-idx-subquery" and .operation == "deployed")] | length) == 1 and
    ([.records[] | select(.case == "single-idx-constants" and .operation == "listener")] | length) == 2 and
    ([.records[] | select(.case == "multi-idx-constants" and .operation == "listener")] | length) == 2 and
    ([.records[] | select(.case == "plan-3stream" and .operation == "deployed")] | length) == 4 and
    ([.records[] | select(.case == "plan-2stream" and .operation == "deployed")] | length) == 13 and
    ([.records[] | select(.operation != "listener" and .operation != "deployed")] | length) == 0
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
