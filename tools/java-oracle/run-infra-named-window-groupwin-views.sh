#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-groupwin-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-groupwin-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews groupwin-slice: the per-group #groupwin(value)#length(2) map window whose iterator walks group-creation order and whose delete removes without expiring, the per-group #groupwin(value)#time_batch(10 sec) map window whose boundary flush concatenates the group batches as [E1,E4,E2,E3], and the #groupwin(theString, intPrimitive)#length(9) projection windows whose late grouped count(*)/avg/count consumers preload the populated window, the second one having-filtered on a runtime variable (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-083a289ee5f82dd87ad3", "java-runtime-accaf82c3c846493832b", "java-runtime-3326973240f20b92dced", "java-runtime-0ec49d4098c9796540b0"]) and
    (.javaNames == ["InfraLengthWindowPerGroup", "InfraTimeBatchPerGroup", "InfraSelectGroupedViewLateStart", "InfraSelectGroupedViewLateStartVariableIterate"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["length-per-group", "time-batch-per-group", "select-grouped-view-late-start", "select-grouped-view-late-start-variable-iterate"]) and
    ([.cases[].ordinal] == [26, 27, 44, 55]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "per-group #groupwin(value)#length(2) window over the key/value map schema: each group keeps its newest two rows, an over-full group expels its oldest row as old data in the same delta as the triggering insert, a delete removes without expiring, and the iterator walks group-creation order",
        "per-group #groupwin(value)#time_batch(10 sec) window over the key/value map schema: arrivals buffer silently, each group anchors its boundary at its first arrival, and the 11000 flush concatenates the completed group batches as [E1,E4,E2,E3] for both the window stream and the plain consumer",
        "grouped #groupwin(theString, intPrimitive)#length(9) projection window whose count(*) consumer is deployed after the window is populated: the late preload builds the per-group counts, the window iterator keeps the twelve rows of the ten groups, and the consumer iterates one sorted row per group",
        "grouped #groupwin(theString, intPrimitive)#length(9) projection window whose variable-filtered avg/count consumer is deployed after the window is populated: the late preload builds the group state including both rows of the non-uniform (c1,1) group, and the having clause reads the runtime variable at iterate time so the on-set trigger switches the visible theString group"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  varEpl/onSetEpl pin the ord-55 variable and
    # on-set texts and stay empty on the other three cases; the advance-time
    # steps carry the absolute RFC3339 instant the suite passes to sendTimer.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "onSetEpl", "ordinal", "runtimeId", "s0Epl", "varEpl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "fields", "mode", "op", "statement"] or . == ["case", "op", "statement"] or . == ["case", "op"] or . == ["at", "case", "op"])) and
    # iteratorSnapshots is the count of snapshot steps: two for ord 26
    # (INV:2527/2533), none for ord 27, two for ord 44 (INV:3015-3018 and
    # INV:3023-3039) and three for ord 55 (INV:3077-3080, INV:3091-3100,
    # INV:3105-3114).
    ([.cases[].iteratorSnapshots] == [2, 0, 2, 3]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "s0"], ["create", "insert", "s0"], ["create", "insert", "var", "on-set", "s0"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create", "s0"], [], []]) and
    # per-case step counts: length-per-group = 1 case + 4 deploys + 10 sends +
    # 2 snapshots + 1 undeploy-all = 18; time-batch-per-group =
    # 1 + 3 deploys + 3 advance-time + 4 sends + 1 undeploy-all = 12;
    # select-grouped-view-late-start = 1 + 3 + 12 + 2 + 2 undeploys = 20;
    # select-grouped-view-late-start-variable-iterate =
    # 1 + 5 + 12 + 3 + 1 = 22 (18+12+20+22 = 72).
    ([.steps[] | select(.case == "length-per-group")] | length == 18) and
    ([.steps[] | select(.case == "time-batch-per-group")] | length == 12) and
    ([.steps[] | select(.case == "select-grouped-view-late-start")] | length == 20) and
    ([.steps[] | select(.case == "select-grouped-view-late-start-variable-iterate")] | length == 22) and
    (.steps | type == "array" and length == 72) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "undeploy")] | length == 2) and
    ([.steps[] | select(.op == "deploy")] | length == 15) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "var")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "on-set")] | length == 1) and
    ([.steps[] | select(.op == "send")] | length == 38) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 34) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportVariableSetEvent")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 7) and
    ([.steps[] | select(.op == "advance-time")] | length == 3) and
    # snapshot statements and modes in source order: the ord-26 window twice
    # ordered, the ord-44 window count-only (empty fields, mode any) followed by
    # its ordered consumer, the ord-55 window count-only followed by two ordered
    # consumer states.
    ([.steps[] | select(.op == "snapshot") | .statement] == ["create", "create", "create", "s0", "create", "s0", "s0"]) and
    ([.steps[] | select(.op == "snapshot") | .mode] == ["ordered", "ordered", "any", "ordered", "any", "ordered", "ordered"]) and
    ([.steps[] | select(.op == "snapshot" and .mode == "any") | .fields] | all(. == [])) and
    ([.steps[] | select(.op == "snapshot" and .case == "length-per-group") | .fields] | all(. == ["key", "value"])) and
    ([.steps[] | select(.op == "snapshot" and .case == "select-grouped-view-late-start" and .statement == "s0") | .fields] | all(. == ["theString", "intPrimitive", "count(*)"])) and
    ([.steps[] | select(.op == "snapshot" and .case == "select-grouped-view-late-start-variable-iterate" and .statement == "s0") | .fields] | all(. == ["theString", "intPrimitive", "avgLong", "cntBool"])) and
    # byte-exact EPL pins: the groupwin spellings, the two @public window
    # creates of the late-start cases (ords 26/27 carry none), the ord-44 named
    # insert, the unnamed ord-55 insert and the ord-55 variable/on-set texts.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowWPG#groupwin(value)#length(2) as MySimpleKeyValueMap", "@name('"'"'create'"'"') create window MyWindowTBPG#groupwin(value)#time_batch(10 sec) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindowSGVS#groupwin(theString, intPrimitive)#length(9) as select theString, intPrimitive from SupportBean", "@name('"'"'create'"'"') @public create window MyWindowSGVLS#groupwin(theString, intPrimitive)#length(9) as select theString, intPrimitive, longPrimitive, boolPrimitive from SupportBean"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowWPG select theString as key, longBoxed as value from SupportBean", "insert into MyWindowTBPG select theString as key, longBoxed as value from SupportBean", "@name('"'"'insert'"'"') insert into MyWindowSGVS select theString, intPrimitive from SupportBean", "insert into MyWindowSGVLS select theString, intPrimitive, longPrimitive, boolPrimitive from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select irstream key, value as value from MyWindowWPG", "@name('"'"'s0'"'"') select key, value as value from MyWindowTBPG", "@name('"'"'s0'"'"') select theString, intPrimitive, count(*) from MyWindowSGVS group by theString, intPrimitive order by theString, intPrimitive", "@name('"'"'s0'"'"') select theString, intPrimitive, avg(longPrimitive) as avgLong, count(boolPrimitive) as cntBool from MyWindowSGVLS group by theString, intPrimitive having theString = var_1_1_1 order by theString, intPrimitive"]) and
    ([.cases[].consumeEpl] == ["", "", "", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindowWPG where symbol = key", "", "", ""]) and
    ([.cases[].varEpl] == ["", "", "", "@public create variable string var_1_1_1"]) and
    ([.cases[].onSetEpl] == ["", "", "", "on SupportVariableSetEvent(variableName='"'"'var_1_1_1'"'"') set var_1_1_1 = value"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[1].s0Epl, .cases[2].s0Epl, .cases[3].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "var") | .epl] == [.cases[3].varEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "on-set") | .epl] == [.cases[3].onSetEpl]) and
    # send payload shapes: theString plus longBoxed for the two per-group map
    # windows, theString plus intPrimitive for ord 44 (the SupportBean(String,int)
    # constructor), theString plus intPrimitive/longPrimitive/boolPrimitive for
    # ord 55, the market trigger carries symbol and the variable trigger carries
    # variableName plus value.
    ([.steps[] | select(.op == "send" and (.case == "length-per-group" or .case == "time-batch-per-group") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "select-grouped-view-late-start" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "select-grouped-view-late-start-variable-iterate" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["boolPrimitive", "intPrimitive", "longPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportVariableSetEvent") | (.payload | keys | sort)] | all(. == ["value", "variableName"])) and
    # the pinned clock moves: only ord 27 advances, from the leading
    # sendTimer(0) pin through the 1000 arrival instant to the 11000 flush.
    ([.steps[] | select(.op == "advance-time" and .case == "time-batch-per-group") | .at] == ["1970-01-01T00:00:00Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:11Z"]) and
    ([.steps[] | select(.op == "advance-time") | .case] | all(. == "time-batch-per-group")) and
    # placement pins: the ord-26 four-statement module and the ord-27
    # three-statement module open their cases; ord 44 opens with create+insert,
    # snapshots the window, deploys its late consumer and ends with the two
    # module undeploys; ord 55 opens with its four modules, fills the window,
    # snapshots it, deploys the late consumer, sets the variable twice and ends
    # with undeploy-all.
    ([.steps[] | select(.case == "length-per-group")] | .[0] == {"op": "case", "case": "length-per-group"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].statement == "delete" and .[5].op == "send" and .[17].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-batch-per-group")] | .[0] == {"op": "case", "case": "time-batch-per-group"} and .[1].op == "advance-time" and .[1].at == "1970-01-01T00:00:00Z" and .[2].statement == "create" and .[3].statement == "insert" and .[4].statement == "s0" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:01Z" and .[11].op == "undeploy-all") and
    ([.steps[] | select(.case == "select-grouped-view-late-start")] | .[0] == {"op": "case", "case": "select-grouped-view-late-start"} and .[1].statement == "create" and .[2].statement == "insert" and .[15].op == "snapshot" and .[15].statement == "create" and .[16].op == "deploy" and .[16].statement == "s0" and .[17].op == "snapshot" and .[17].statement == "s0" and .[18].op == "undeploy" and .[18].statement == "s0" and .[19].op == "undeploy" and .[19].statement == "create") and
    ([.steps[] | select(.case == "select-grouped-view-late-start-variable-iterate")] | .[0] == {"op": "case", "case": "select-grouped-view-late-start-variable-iterate"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "var" and .[4].statement == "on-set" and .[15].op == "snapshot" and .[15].statement == "create" and .[16].op == "deploy" and .[16].statement == "s0" and .[17].op == "send" and .[17].eventType == "SupportVariableSetEvent" and .[18].op == "snapshot" and .[20].op == "snapshot" and .[21].op == "undeploy-all")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-groupwin-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-groupwin-views.XXXXXX")
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
    "$script_root/InfraNamedWindowGroupwinViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowGroupwinViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    # e is the count-only row: the ord-44/55 window snapshots project no field.
    def e: {"kind": "row", "fields": {}};
    def lis($c; $st; $q; $t; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new, "old": $old};
    def lisn($c; $st; $q; $t; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new};
    def liso($c; $st; $q; $t; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "old": $old};
    def snapr($c; $st; $t; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    def snapc($c; $st; $t; $n): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": [range($n) | e]};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 26 (InfraLengthWindowPerGroup, lines 2506-2577): the fourth arrival of
    # group 1L (E5) expels E1 in the same callback that carries E5, and the same
    # happens for group 2L at E8 expelling E3; the two deletes are old-only and
    # the two window snapshots list the groups in creation order (g1 before g2)
    # with insertion order inside each group.
    def lpg: [
        lisn("length-per-group"; "create"; 1; "1970-01-01T00:00:00Z"; [r("E1"; 1)]),
        lisn("length-per-group"; "s0"; 1; "1970-01-01T00:00:00Z"; [r("E1"; 1)]),
        lisn("length-per-group"; "create"; 2; "1970-01-01T00:00:00Z"; [r("E2"; 1)]),
        lisn("length-per-group"; "s0"; 2; "1970-01-01T00:00:00Z"; [r("E2"; 1)]),
        lisn("length-per-group"; "create"; 3; "1970-01-01T00:00:00Z"; [r("E3"; 2)]),
        lisn("length-per-group"; "s0"; 3; "1970-01-01T00:00:00Z"; [r("E3"; 2)]),
        snapr("length-per-group"; "create"; "1970-01-01T00:00:00Z"; [r("E1"; 1), r("E2"; 1), r("E3"; 2)]),
        liso("length-per-group"; "create"; 4; "1970-01-01T00:00:00Z"; [r("E2"; 1)]),
        liso("length-per-group"; "s0"; 4; "1970-01-01T00:00:00Z"; [r("E2"; 1)]),
        snapr("length-per-group"; "create"; "1970-01-01T00:00:00Z"; [r("E1"; 1), r("E3"; 2)]),
        lisn("length-per-group"; "create"; 5; "1970-01-01T00:00:00Z"; [r("E4"; 1)]),
        lisn("length-per-group"; "s0"; 5; "1970-01-01T00:00:00Z"; [r("E4"; 1)]),
        lis("length-per-group"; "create"; 6; "1970-01-01T00:00:00Z"; [r("E5"; 1)]; [r("E1"; 1)]),
        lis("length-per-group"; "s0"; 6; "1970-01-01T00:00:00Z"; [r("E5"; 1)]; [r("E1"; 1)]),
        lisn("length-per-group"; "create"; 7; "1970-01-01T00:00:00Z"; [r("E6"; 2)]),
        lisn("length-per-group"; "s0"; 7; "1970-01-01T00:00:00Z"; [r("E6"; 2)]),
        liso("length-per-group"; "create"; 8; "1970-01-01T00:00:00Z"; [r("E6"; 2)]),
        liso("length-per-group"; "s0"; 8; "1970-01-01T00:00:00Z"; [r("E6"; 2)]),
        lisn("length-per-group"; "create"; 9; "1970-01-01T00:00:00Z"; [r("E7"; 2)]),
        lisn("length-per-group"; "s0"; 9; "1970-01-01T00:00:00Z"; [r("E7"; 2)]),
        lis("length-per-group"; "create"; 10; "1970-01-01T00:00:00Z"; [r("E8"; 2)]; [r("E3"; 2)]),
        lis("length-per-group"; "s0"; 10; "1970-01-01T00:00:00Z"; [r("E8"; 2)]; [r("E3"; 2)])
    ];
    # Ord 27 (InfraTimeBatchPerGroup, lines 2585-2614): both per-group batches
    # arm at the 1000 arrivals and flush at 11000 in ONE callback per listener,
    # carrying group 10L (E1,E4) before group 20L (E2,E3); the flush has no old
    # data because each group'"'"'s previous batch is empty.
    def tbp: [
        lisn("time-batch-per-group"; "create"; 1; "1970-01-01T00:00:11Z"; [r("E1"; 10), r("E4"; 10), r("E2"; 20), r("E3"; 20)]),
        lisn("time-batch-per-group"; "s0"; 1; "1970-01-01T00:00:11Z"; [r("E1"; 10), r("E4"; 10), r("E2"; 20), r("E3"; 20)])
    ];
    # Ord 44 (InfraSelectGroupedViewLateStart, lines 3001-3042): no listeners.
    # The window holds all twelve events and the late-started consumer preloads
    # them, so its iterator reports the ten groups with the doubled counts of
    # (c0,1) and (c1,2) in order-by order.
    def sgvs: [
        snapc("select-grouped-view-late-start"; "create"; "1970-01-01T00:00:00Z"; 12),
        snapr("select-grouped-view-late-start"; "s0"; "1970-01-01T00:00:00Z"; [
            {"kind": "row", "fields": {"theString": "c0", "intPrimitive": 0, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c0", "intPrimitive": 1, "count(*)": 2}},
            {"kind": "row", "fields": {"theString": "c0", "intPrimitive": 2, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c1", "intPrimitive": 0, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c1", "intPrimitive": 1, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c1", "intPrimitive": 2, "count(*)": 2}},
            {"kind": "row", "fields": {"theString": "c2", "intPrimitive": 0, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c2", "intPrimitive": 1, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c2", "intPrimitive": 2, "count(*)": 1}},
            {"kind": "row", "fields": {"theString": "c3", "intPrimitive": 3, "count(*)": 1}}
        ])
    ];
    # Ord 55 (InfraSelectGroupedViewLateStartVariableIterate, lines 3051-3116):
    # no listeners.  The window keeps all ten events; the late-started consumer
    # preloads them and its having clause reads the variable at iterate time, so
    # the same consumer reports the c0 groups first and the c1 groups after the
    # on-set trigger, with avg 5.5 and count 2 for the non-uniform (c1,1) group.
    def sgvls: [
        snapc("select-grouped-view-late-start-variable-iterate"; "create"; "1970-01-01T00:00:00Z"; 10),
        snapr("select-grouped-view-late-start-variable-iterate"; "s0"; "1970-01-01T00:00:00Z"; [
            {"kind": "row", "fields": {"theString": "c0", "intPrimitive": 0, "avgLong": 0.0, "cntBool": 1}},
            {"kind": "row", "fields": {"theString": "c0", "intPrimitive": 1, "avgLong": 1.0, "cntBool": 1}},
            {"kind": "row", "fields": {"theString": "c0", "intPrimitive": 2, "avgLong": 2.0, "cntBool": 1}}
        ]),
        snapr("select-grouped-view-late-start-variable-iterate"; "s0"; "1970-01-01T00:00:00Z"; [
            {"kind": "row", "fields": {"theString": "c1", "intPrimitive": 0, "avgLong": 0.0, "cntBool": 1}},
            {"kind": "row", "fields": {"theString": "c1", "intPrimitive": 1, "avgLong": 5.5, "cntBool": 2}},
            {"kind": "row", "fields": {"theString": "c1", "intPrimitive": 2, "avgLong": 2.0, "cntBool": 1}}
        ])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-groupwin-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 29) and
    # 22 + 2 + 2 + 3 records; 22 listener and 7 snapshot records.
    ((crec("length-per-group") | length) == 22) and
    ((crec("time-batch-per-group") | length) == 2) and
    ((crec("select-grouped-view-late-start") | length) == 2) and
    ((crec("select-grouped-view-late-start-variable-iterate") | length) == 3) and
    ([.records[] | select(.operation == "listener")] | length == 22) and
    ([.records[] | select(.operation == "snapshot")] | length == 7) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("length-per-group"; 22) + repv("time-batch-per-group"; 2) + repv("select-grouped-view-late-start"; 2) + repv("select-grouped-view-late-start-variable-iterate"; 3))) and
    # per-case listener/snapshot split: 20+2, 2+0, 0+2 and 0+3.
    ((crec("length-per-group") | map(select(.operation == "listener")) | length) == 20) and
    ((crec("length-per-group") | map(select(.operation == "snapshot")) | length) == 2) and
    ((crec("time-batch-per-group") | map(select(.operation == "listener")) | length) == 2) and
    ((crec("time-batch-per-group") | map(select(.operation == "snapshot")) | length) == 0) and
    ((crec("select-grouped-view-late-start") | map(select(.operation == "listener")) | length) == 0) and
    ((crec("select-grouped-view-late-start") | map(select(.operation == "snapshot")) | length) == 2) and
    ((crec("select-grouped-view-late-start-variable-iterate") | map(select(.operation == "listener")) | length) == 0) and
    ((crec("select-grouped-view-late-start-variable-iterate") | map(select(.operation == "snapshot")) | length) == 3) and
    # virtual instants: only the ord-27 flush moves the clock; every other
    # record carries the pinned start instant.
    ([.records[] | select(.case == "length-per-group") | .time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.case == "time-batch-per-group") | .time] | all(. == "1970-01-01T00:00:11Z")) and
    ([.records[] | select(.case == "select-grouped-view-late-start") | .time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.case == "select-grouped-view-late-start-variable-iterate") | .time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statements and per-statement 1-based sequences: ord 26 alternates
    # create/s0 across ten waves, ord 27 fires both listeners once; snapshots
    # carry sequence 0.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 10) + repa(["create", "s0"]; 1))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10] + [1, 1])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    # the flush and expiry shapes: exactly four records carry both streams (the
    # two per-group expiries of ord 26 seen by both listeners), exactly four are
    # old-only (the two deletes of ord 26 seen by both listeners) and the ord-27
    # flush is the only four-row new array.
    ([.records[] | select(has("new") and has("old"))] | length == 4) and
    ([.records[] | select(has("new") and has("old")) | [.case, .statement, .sequence]] == [["length-per-group", "create", 6], ["length-per-group", "s0", 6], ["length-per-group", "create", 10], ["length-per-group", "s0", 10]]) and
    ([.records[] | select(has("old") and (has("new") | not)) | [.case, .statement, .sequence, (.old | length)]] == [["length-per-group", "create", 4, 1], ["length-per-group", "s0", 4, 1], ["length-per-group", "create", 8, 1], ["length-per-group", "s0", 8, 1]]) and
    ([.records[] | select(.operation == "listener" and (.new | length) == 4) | [.case, .statement, .sequence]] == [["time-batch-per-group", "create", 1], ["time-batch-per-group", "s0", 1]]) and
    ([.records[] | select(.operation == "listener" and (.new | length) > 1) | (.new | length)] | all(. == 4)) and
    # the count-only snapshots emit empty field maps: twelve rows for ord 44'"'"'s
    # window and ten for ord 55'"'"'s, and no other snapshot projects an empty map.
    ([.records[] | select(.operation == "snapshot" and (.new | length) > 0 and (.new[0].fields | length) == 0) | [.case, .statement, (.new | length)]] == [["select-grouped-view-late-start", "create", 12], ["select-grouped-view-late-start-variable-iterate", "create", 10]]) and
    ([.records[] | select(.operation == "snapshot" and .case == "length-per-group") | .new | map(.fields | length) | unique] | all(. == [2])) and
    # every listener record carries at least one stream and every row projects
    # exactly the case fields.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | select(.case == "length-per-group") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-batch-per-group") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "select-grouped-view-late-start" and .operation == "snapshot" and .statement == "s0") | .new[] | (.fields | keys) == ["count(*)", "intPrimitive", "theString"]] | all) and
    ([.records[] | select(.case == "select-grouped-view-late-start-variable-iterate" and .operation == "snapshot" and .statement == "s0") | .new[] | (.fields | keys) == ["avgLong", "cntBool", "intPrimitive", "theString"]] | all) and
    (crec("length-per-group") == lpg) and
    (crec("time-batch-per-group") == tbp) and
    (crec("select-grouped-view-late-start") == sgvs) and
    (crec("select-grouped-view-late-start-variable-iterate") == sgvls)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-groupwin-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
