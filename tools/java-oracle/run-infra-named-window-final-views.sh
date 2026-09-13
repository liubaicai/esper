#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-final-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
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
[ "$java_version" = "17" ] || {
    echo "Java 17 is required for the Java 9.0.0 oracle; found ${java_version:-unknown}" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-named-window-final-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews final slice: the pattern consumer over the named-window insert stream with or-quit silence, and the time-to-live window with merge insert and on-delete triggers observed through any-order iterator snapshots under absolute virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-476271957d6ffdb3a878", "java-runtime-3c2f3a2696127c04b2a6"]) and
    (.javaNames == ["InfraPattern", "InfraNamedWindowTimeToLiveDelete"]) and
    (.javaStaticIds == ["java-030c8e6d456d680e8745"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 2) and
    ([.cases[].case] == ["pattern", "ttl-delete"]) and
    ([.cases[].ordinal] == [52, 57]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "pattern consumer over the named-window insert stream: every S1 match re-arms while the single S2 match quits the whole or-expression",
        "whole-bean timetolive window with merge insert and p00 deletes observed through any-order iterator snapshots over absolute virtual time"
    ]) and
    # the case key set and the step key sets are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "createEpl", "deleteEpl", "deploys", "description", "epl", "executionName", "insertEpl", "iteratorSnapshots", "listened", "mergeEpl", "observation", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["at", "case", "op"] or . == ["case", "op"])) and
    # observation pins: the pattern case is listener-only, the TTL case is
    # iterator-only with five snapshots.
    ([.cases[] | [.case, .observation, .iteratorSnapshots]] == [["pattern", "listener", 0], ["ttl-delete", "iterator", 5]]) and
    ([.cases[].deploys] == [["create", "s0", "insert"], ["win", "merge", "delete"]]) and
    ([.cases[].listened] == [["s0"], []]) and
    # per-case step counts: pattern = 1 case + 3 deploys + 5 sends +
    # 1 undeploy-all = 10; ttl-delete = 1 case + 4 advance-time + 3 deploys +
    # 6 sends + 5 snapshots + 1 undeploy-all = 20 (10+20 = 30).
    ([.steps[] | select(.case == "pattern")] | length == 10) and
    ([.steps[] | select(.case == "ttl-delete")] | length == 20) and
    (.steps | type == "array" and length == 30) and
    ([.steps[] | select(.op == "case")] | length == 2) and
    ([.steps[] | select(.op == "deploy")] | length == 6) and
    ([.steps[] | select(.op == "send")] | length == 11) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 9) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 5) and
    ([.steps[] | select(.op == "advance-time")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 2) and
    # every snapshot targets the win statement in any mode.
    ([.steps[] | select(.op == "snapshot") | [.statement, .mode]] | all(. == ["win", "any"])) and
    # byte-exact EPL pins.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowPAT#keepall as MySimpleKeyValueMap", "@name('"'"'win'"'"') create window MyWindow#timetolive(current_timestamp() + longPrimitive) as SupportBean"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowPAT select theString as key, longBoxed as value from SupportBean", ""]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select a.key as key, a.value as value from pattern [every a=MyWindowPAT(key='"'"'S1'"'"') or a=MyWindowPAT(key='"'"'S2'"'"')]", ""]) and
    ([.cases[].mergeEpl] == ["", "on SupportBean merge MyWindow insert select *"]) and
    ([.cases[].deleteEpl] == ["", "on SupportBean_S0 delete from MyWindow where theString = p00"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "win") | .epl] == [.cases[1].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "merge") | .epl] == [.cases[1].mergeEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[1].deleteEpl]) and
    # send payload shapes: longBoxed for the pattern case, longPrimitive for
    # the TTL inserts, p00 for the deletes.
    ([.steps[] | select(.op == "send" and .case == "pattern") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "ttl-delete" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | (.payload | keys)] | all(. == ["p00"])) and
    ([.steps[] | select(.op == "send" and .case == "pattern") | [.payload.theString, .payload.longBoxed]] == [["E1", 1], ["S1", 2], ["S1", 3], ["S2", 4], ["S1", 1]]) and
    ([.steps[] | select(.op == "send" and .case == "ttl-delete" and .eventType == "SupportBean") | [.payload.theString, .payload.longPrimitive]] == [["E1", 2000], ["E2", 3000], ["E3", 1000], ["E4", 2000]]) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | .payload.p00] == ["E2", "E1"]) and
    # the pinned clock moves of the TTL case.
    ([.steps[] | select(.op == "advance-time") | .at] == ["1970-01-01T00:00:00Z", "1970-01-01T00:00:00.500Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:02Z"]) and
    # placement pins: the pattern case opens with its three-statement module
    # (s0 before insert, matching the Java source order); the TTL case opens
    # with the epoch pin before its module; every case ends with undeploy-all.
    ([.steps[] | select(.case == "pattern")] | .[0] == {"op": "case", "case": "pattern"} and .[1].statement == "create" and .[2].statement == "s0" and .[3].statement == "insert" and .[9].op == "undeploy-all") and
    ([.steps[] | select(.case == "ttl-delete")] | .[0] == {"op": "case", "case": "ttl-delete"} and .[1].op == "advance-time" and .[1].at == "1970-01-01T00:00:00Z" and .[2].statement == "win" and .[3].statement == "merge" and .[4].statement == "delete" and .[19].op == "undeploy-all")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-final-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-final-views.XXXXXX")
cleanup() {
    status=$?
    rm -rf "$work"
    exit $status
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
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    MINGW* | MSYS* | CYGWIN*)
        cp_sep=";"
        native_path() { cygpath -m "$1"; }
        ;;
    *)
        cp_sep=":"
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNamedWindowFinalViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowFinalViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def t($s): {"kind": "row", "fields": {"theString": $s}};
    def lisn($c; $st; $q; $t; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new};
    def snapr($c; $st; $t; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    # snap0 is the empty-iterator snapshot record: the new key is omitted, not
    # an empty array, exactly like the Go normalizer renders it.
    def snap0($c; $st; $t): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t};
    def crec($c): [.records[] | select(.case == $c)];
    # Ord 52 (InfraPattern, lines 3372-3396): three ordered new rows on s0;
    # the first and the last send stay silent.
    def pat: [
        lisn("pattern"; "s0"; 1; "1970-01-01T00:00:00Z"; [r("S1"; 2)]),
        lisn("pattern"; "s0"; 2; "1970-01-01T00:00:00Z"; [r("S1"; 3)]),
        lisn("pattern"; "s0"; 3; "1970-01-01T00:00:00Z"; [r("S2"; 4)])
    ];
    # Ord 57 (InfraNamedWindowTimeToLiveDelete, lines 109-156): five any-order
    # iterator snapshots; the fourth expires E3 and the fifth expires E4.
    def ttl: [
        snapr("ttl-delete"; "win"; "1970-01-01T00:00:00Z"; [t("E1"), t("E2"), t("E3"), t("E4")]),
        snapr("ttl-delete"; "win"; "1970-01-01T00:00:00.500Z"; [t("E1"), t("E3"), t("E4")]),
        snapr("ttl-delete"; "win"; "1970-01-01T00:00:00.500Z"; [t("E3"), t("E4")]),
        snapr("ttl-delete"; "win"; "1970-01-01T00:00:01Z"; [t("E4")]),
        snap0("ttl-delete"; "win"; "1970-01-01T00:00:02Z")
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-final-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 8) and
    ((crec("pattern") | length) == 3) and
    ((crec("ttl-delete") | length) == 5) and
    ([.records[] | select(.operation == "listener")] | length == 3) and
    ([.records[] | select(.operation == "snapshot")] | length == 5) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == ["pattern", "pattern", "pattern", "ttl-delete", "ttl-delete", "ttl-delete", "ttl-delete", "ttl-delete"]) and
    # snapshots carry the any-order multiset (sorted here); listener rows are
    # delivery order.
    ([.records[] | select(.operation == "listener") | [.case, .statement, .sequence]] == [["pattern", "s0", 1], ["pattern", "s0", 2], ["pattern", "s0", 3]]) and
    ([.records[] | select(.operation == "snapshot") | [.case, .statement, .sequence]] | all(. == ["ttl-delete", "win", 0])) and
    ([.records[] | select(.operation == "snapshot") | .time] == ["1970-01-01T00:00:00Z", "1970-01-01T00:00:00.500Z", "1970-01-01T00:00:00.500Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:02Z"]) and
    ([.records[] | select(.operation == "listener") | .time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | .new[]? | has("type")] | all(. == false)) and
    (crec("pattern") == pat) and
    ((crec("ttl-delete")[0].new | sort_by(.fields.theString) | map(.fields.theString)) == ["E1", "E2", "E3", "E4"]) and
    ((crec("ttl-delete")[1].new | sort_by(.fields.theString) | map(.fields.theString)) == ["E1", "E3", "E4"]) and
    ((crec("ttl-delete")[2].new | sort_by(.fields.theString) | map(.fields.theString)) == ["E3", "E4"]) and
    (crec("ttl-delete")[3] == snapr("ttl-delete"; "win"; "1970-01-01T00:00:01Z"; [t("E4")])) and
    (crec("ttl-delete")[4] == snap0("ttl-delete"; "win"; "1970-01-01T00:00:02Z"))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-final-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
