#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-time-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-time-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews time-slice: the projection time(10 sec) window with a separately deployed irstream consumer, the firsttime(10 sec) map window and the bean-backed time(10 sec) window over a SupportBean_A delete trigger, captured from listener callbacks and ordered window iterator snapshots under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-879d6ad8aee378657a02", "java-runtime-5f5a65dd5f72c8e24619", "java-runtime-a6bfdd3389c077127861"]) and
    (.javaNames == ["InfraTimeWindowSceneTwo", "InfraTimeFirstWindow", "InfraExtTimeWindowSceneThree"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["time-window-scene-two", "time-first-window", "ext-time-window-scene-three"]) and
    ([.cases[].ordinal] == [4, 5, 8]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "projection time(10 sec) window over theString/intBoxed with four module deployments; the window statement and the consume consumer both carry irstream, expiry at t=10000 and t=36000 delivers old-only rows and equal-time advances stay silent",
        "firsttime(10 sec) window over the key/value map schema anchored at deploy time (clock 1000): it closes silently at 11000, later sends are dropped while retained rows survive until deleted",
        "bean-backed win:time(10 sec) window with a SupportBean_A delete trigger and a wildcard irstream consumer projected to theString; expiry at the 13000 boundary releases the retained row"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  The advance-time steps are the new protocol
    # element of this slice and carry an absolute RFC3339 instant.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"] or . == ["at", "case", "op"])) and
    # iteratorSnapshots is the execution'"'"'s assertPropsPerRowIterator call-site
    # count: ten for ord 4, seven for ord 5 and six for ord 8 (the ord-8 count
    # is the corrected one - see the trace block below).
    ([.cases[].iteratorSnapshots] == [10, 7, 6]) and
    ([.cases[].deploys] == [["create", "insert", "consume", "delete"], ["create", "insert", "s0", "delete"], ["create", "insert", "s0", "delete"]]) and
    ([.cases[].listened] == [["create", "consume"], ["create", "s0"], ["s0"]]) and
    # per-case step counts: time-window-scene-two = 1 case + 4 deploys + 9
    # advance-time + 8 sends + 10 snapshots + 1 undeploy-all = 33;
    # time-first-window = 1 + 4 + 5 + 5 + 7 + 1 = 23;
    # ext-time-window-scene-three = 1 + 4 + 7 + 5 + 6 + 1 = 24 (33+23+24 = 80).
    ([.steps[] | select(.case == "time-window-scene-two")] | length == 33) and
    ([.steps[] | select(.case == "time-first-window")] | length == 23) and
    ([.steps[] | select(.case == "ext-time-window-scene-three")] | length == 24) and
    (.steps | type == "array" and length == 80) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 12) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 3) and
    ([.steps[] | select(.op == "send")] | length == 18) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 23) and
    ([.steps[] | select(.op == "advance-time")] | length == 21) and
    # ords 4 and 5 snapshot the window statement, ord 8 its s0 consumer, and
    # every snapshot of this slice is exact-order.
    ([.steps[] | select(.op == "snapshot" and (.case == "time-window-scene-two" or .case == "time-first-window")) | .statement] | all(. == "create")) and
    ([.steps[] | select(.op == "snapshot" and .case == "ext-time-window-scene-three") | .statement] | all(. == "s0")) and
    ([.steps[] | select(.op == "snapshot") | .mode] | all(. == "ordered")) and
    # byte-exact EPL pins, including the @public window creates of the
    # multi-module cases, the capital @Name('"'"'s0'"'"') of ord 8 and the irstream
    # consumer of ord 4.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') @public create window MyWindow#time(10 sec) as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowTFW#firsttime(10 sec) as MySimpleKeyValueMap", "@public create window ABCWin.win:time(10 sec) as SupportBean"]) and
    ([.cases[].insertEpl] == ["@name('"'"'insert'"'"') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean", "insert into MyWindowTFW select theString as key, longBoxed as value from SupportBean", "insert into ABCWin select * from SupportBean"]) and
    ([.cases[].s0Epl] == ["", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowTFW", "@Name('"'"'s0'"'"') select irstream * from ABCWin"]) and
    ([.cases[].consumeEpl] == ["@name('"'"'consume'"'"') select irstream key, value as value from MyWindow", "", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowTFW as s1 where s0.symbol = s1.key", "on SupportBean_A delete from ABCWin where theString = id"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume") | .epl] == [.cases[0].consumeEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[1].s0Epl, .cases[2].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[2].deleteEpl]) and
    # send payload shapes: theString plus exactly the one value field the case
    # window reads (intBoxed for the projection window, longBoxed for the map
    # window, theString alone for the bean-typed window); the on-delete market
    # trigger carries symbol and the SupportBean_A trigger carries id.
    ([.steps[] | select(.op == "send" and .case == "time-window-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "time-first-window" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "ext-time-window-scene-three" and .eventType == "SupportBean") | (.payload | keys)] | all(. == ["theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A") | (.payload | keys)] | all(. == ["id"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    # the pinned clock moves, one advance-time step per source advanceTime
    # call (ord 4 J:558/565/582/587/593/595/597/612/614, ord 5 J:654/668/674/
    # 681/698, ord 8 J:864/874/881/888/892/905/908).  The equal-time pairs are
    # ord 4 at 25000 and ord 8 at 3000; 35999 and 12999 are the silent moves
    # before the 36000 and 13000 expiry boundaries.
    ([.steps[] | select(.op == "advance-time" and .case == "time-window-scene-two") | .at] == ["1970-01-01T00:00:00Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:26Z", "1970-01-01T00:00:27Z", "1970-01-01T00:00:35.999Z", "1970-01-01T00:00:36Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-first-window") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:12Z", "1970-01-01T00:01:40Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "ext-time-window-scene-three") | .at] == ["1970-01-01T00:00:00Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:02Z", "1970-01-01T00:00:03Z", "1970-01-01T00:00:03Z", "1970-01-01T00:00:12.999Z", "1970-01-01T00:00:13Z"]) and
    # ord 5 moves the clock to 1000 BEFORE its four deploys (the source sends
    # the timer before compiling), which anchors the firsttime close at 11000;
    # ord 8 opens with its advanceTime(0) before the four module deploys and
    # ord 4 performs its advanceTime(0) after them.
    ([.steps[] | select(.case == "time-first-window")] | .[0] == {"op": "case", "case": "time-first-window"} and .[1].op == "advance-time" and .[1].at == "1970-01-01T00:00:01Z" and .[2].op == "deploy") and
    ([.steps[] | select(.case == "ext-time-window-scene-three")] | .[0] == {"op": "case", "case": "ext-time-window-scene-three"} and .[1].op == "advance-time" and .[1].at == "1970-01-01T00:00:00Z" and .[2].op == "deploy") and
    ([.steps[] | select(.case == "time-window-scene-two")] | .[0] == {"op": "case", "case": "time-window-scene-two"} and .[1].op == "deploy" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:00Z")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-time-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-time-views.XXXXXX")
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
    "$script_root/InfraNamedWindowTimeViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowTimeViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def rb($s): {"kind": "row", "fields": {"theString": $s}};
    def lis($c; $st; $q; $t; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new, "old": $old};
    def lisn($c; $st; $q; $t; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new};
    def liso($c; $st; $q; $t; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "old": $old};
    def snapr($c; $st; $t; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    # snap0 is the empty-iterator snapshot record: the new key is omitted, not
    # an empty array, exactly like the Go normalizer renders it.
    def snap0($c; $st; $t): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 4 (InfraTimeWindowSceneTwo, lines 534-633): the window statement is
    # deployed before the consume consumer, so every wave delivers create first
    # and then consume; both carry irstream, so the expiry at exactly 10000
    # (G1, ts 0) and at exactly 36000 (G4, ts 26000) delivers old rows only,
    # while 35999 stays silent after the silent 35000 reschedule.  The two
    # 25000 advances are the equal-time pair and the G2/G3/G5 deletes are
    # old-only waves.  G4 is sent at 26000 and G5 at 27000, which is why the
    # three snapshots after the G3 delete all read t=27000.
    def twst: [
        lisn("time-window-scene-two"; "create"; 1; "1970-01-01T00:00:00Z"; [r("G1"; 10)]),
        lisn("time-window-scene-two"; "consume"; 1; "1970-01-01T00:00:00Z"; [r("G1"; 10)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 10)]),
        lisn("time-window-scene-two"; "create"; 2; "1970-01-01T00:00:05Z"; [r("G2"; 20)]),
        lisn("time-window-scene-two"; "consume"; 2; "1970-01-01T00:00:05Z"; [r("G2"; 20)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 10), r("G2"; 20)]),
        liso("time-window-scene-two"; "create"; 3; "1970-01-01T00:00:05Z"; [r("G2"; 20)]),
        liso("time-window-scene-two"; "consume"; 3; "1970-01-01T00:00:05Z"; [r("G2"; 20)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 10)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 10)]),
        liso("time-window-scene-two"; "create"; 4; "1970-01-01T00:00:10Z"; [r("G1"; 10)]),
        liso("time-window-scene-two"; "consume"; 4; "1970-01-01T00:00:10Z"; [r("G1"; 10)]),
        lisn("time-window-scene-two"; "create"; 5; "1970-01-01T00:00:25Z"; [r("G3"; 30)]),
        lisn("time-window-scene-two"; "consume"; 5; "1970-01-01T00:00:25Z"; [r("G3"; 30)]),
        lisn("time-window-scene-two"; "create"; 6; "1970-01-01T00:00:26Z"; [r("G4"; 40)]),
        lisn("time-window-scene-two"; "consume"; 6; "1970-01-01T00:00:26Z"; [r("G4"; 40)]),
        lisn("time-window-scene-two"; "create"; 7; "1970-01-01T00:00:27Z"; [r("G5"; 50)]),
        lisn("time-window-scene-two"; "consume"; 7; "1970-01-01T00:00:27Z"; [r("G5"; 50)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:27Z"; [r("G3"; 30), r("G4"; 40), r("G5"; 50)]),
        liso("time-window-scene-two"; "create"; 8; "1970-01-01T00:00:27Z"; [r("G3"; 30)]),
        liso("time-window-scene-two"; "consume"; 8; "1970-01-01T00:00:27Z"; [r("G3"; 30)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:27Z"; [r("G4"; 40), r("G5"; 50)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:27Z"; [r("G4"; 40), r("G5"; 50)]),
        liso("time-window-scene-two"; "create"; 9; "1970-01-01T00:00:36Z"; [r("G4"; 40)]),
        liso("time-window-scene-two"; "consume"; 9; "1970-01-01T00:00:36Z"; [r("G4"; 40)]),
        snapr("time-window-scene-two"; "create"; "1970-01-01T00:00:36Z"; [r("G5"; 50)]),
        liso("time-window-scene-two"; "create"; 10; "1970-01-01T00:00:36Z"; [r("G5"; 50)]),
        liso("time-window-scene-two"; "consume"; 10; "1970-01-01T00:00:36Z"; [r("G5"; 50)]),
        snap0("time-window-scene-two"; "create"; "1970-01-01T00:00:36Z"),
        snap0("time-window-scene-two"; "create"; "1970-01-01T00:00:36Z")
    ];
    # Ord 5 (InfraTimeFirstWindow, lines 654-702): the deploy happens at clock
    # 1000, so the firsttime gate closes silently at 11000; E3 arrives at
    # exactly 10000 (admitted, the close has not fired yet) and E4 at 12000 is
    # dropped without any callback, while the retained rows never expire and
    # only the explicit market delete at 12000 removes E2.  The seven snapshots
    # are the single empty one at t=1000, three growth states and the three
    # 12000 states around the dropped E4 and the delete.
    def tfw: [
        snap0("time-first-window"; "create"; "1970-01-01T00:00:01Z"),
        lisn("time-first-window"; "create"; 1; "1970-01-01T00:00:01Z"; [r("E1"; 1)]),
        lisn("time-first-window"; "s0"; 1; "1970-01-01T00:00:01Z"; [r("E1"; 1)]),
        snapr("time-first-window"; "create"; "1970-01-01T00:00:01Z"; [r("E1"; 1)]),
        lisn("time-first-window"; "create"; 2; "1970-01-01T00:00:05Z"; [r("E2"; 2)]),
        lisn("time-first-window"; "s0"; 2; "1970-01-01T00:00:05Z"; [r("E2"; 2)]),
        snapr("time-first-window"; "create"; "1970-01-01T00:00:05Z"; [r("E1"; 1), r("E2"; 2)]),
        lisn("time-first-window"; "create"; 3; "1970-01-01T00:00:10Z"; [r("E3"; 3)]),
        lisn("time-first-window"; "s0"; 3; "1970-01-01T00:00:10Z"; [r("E3"; 3)]),
        snapr("time-first-window"; "create"; "1970-01-01T00:00:10Z"; [r("E1"; 1), r("E2"; 2), r("E3"; 3)]),
        snapr("time-first-window"; "create"; "1970-01-01T00:00:12Z"; [r("E1"; 1), r("E2"; 2), r("E3"; 3)]),
        snapr("time-first-window"; "create"; "1970-01-01T00:00:12Z"; [r("E1"; 1), r("E2"; 2), r("E3"; 3)]),
        liso("time-first-window"; "create"; 4; "1970-01-01T00:00:12Z"; [r("E2"; 2)]),
        liso("time-first-window"; "s0"; 4; "1970-01-01T00:00:12Z"; [r("E2"; 2)]),
        snapr("time-first-window"; "create"; "1970-01-01T00:00:12Z"; [r("E1"; 1), r("E3"; 3)])
    ];
    # Ord 8 (InfraExtTimeWindowSceneThree, lines 864-915): only s0 is listened.
    # E2 and E3 arrive at the same virtual time 3000 (the pinned equal-time
    # advance pair) and iterate in arrival order; the two SupportBean_A deletes
    # deliver old only, and E2 expires at exactly 13000 after the silent 12999
    # move.  Six snapshots (three of them empty) and six listener records make
    # this case 12 records, the corrected ord-8 count of this slice.
    def ext: [
        snap0("ext-time-window-scene-three"; "s0"; "1970-01-01T00:00:00Z"),
        lisn("ext-time-window-scene-three"; "s0"; 1; "1970-01-01T00:00:01Z"; [rb("E1")]),
        snapr("ext-time-window-scene-three"; "s0"; "1970-01-01T00:00:01Z"; [rb("E1")]),
        liso("ext-time-window-scene-three"; "s0"; 2; "1970-01-01T00:00:02Z"; [rb("E1")]),
        snap0("ext-time-window-scene-three"; "s0"; "1970-01-01T00:00:02Z"),
        lisn("ext-time-window-scene-three"; "s0"; 3; "1970-01-01T00:00:03Z"; [rb("E2")]),
        lisn("ext-time-window-scene-three"; "s0"; 4; "1970-01-01T00:00:03Z"; [rb("E3")]),
        snapr("ext-time-window-scene-three"; "s0"; "1970-01-01T00:00:03Z"; [rb("E2"), rb("E3")]),
        liso("ext-time-window-scene-three"; "s0"; 5; "1970-01-01T00:00:03Z"; [rb("E3")]),
        snapr("ext-time-window-scene-three"; "s0"; "1970-01-01T00:00:03Z"; [rb("E2")]),
        liso("ext-time-window-scene-three"; "s0"; 6; "1970-01-01T00:00:13Z"; [rb("E2")]),
        snap0("ext-time-window-scene-three"; "s0"; "1970-01-01T00:00:13Z")
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-time-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 57) and
    # 30 + 15 + 12 records, 34 listener and 23 snapshot.
    ((crec("time-window-scene-two") | length) == 30) and
    ((crec("time-first-window") | length) == 15) and
    ((crec("ext-time-window-scene-three") | length) == 12) and
    ([.records[] | select(.operation == "listener")] | length == 34) and
    ([.records[] | select(.operation == "snapshot")] | length == 23) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("time-window-scene-two"; 30) + repv("time-first-window"; 15) + repv("ext-time-window-scene-three"; 12))) and
    # per-case snapshot record counts: ten for ord 4, seven for ord 5 and six
    # for ord 8 (the corrected count: six assertPropsPerRowIterator call sites).
    ((crec("time-window-scene-two") | map(select(.operation == "snapshot")) | length) == 10) and
    ((crec("time-first-window") | map(select(.operation == "snapshot")) | length) == 7) and
    ((crec("ext-time-window-scene-three") | map(select(.operation == "snapshot")) | length) == 6) and
    # every record of this slice carries the virtual clock at delivery, never
    # the wall clock, and only the pinned instants occur.
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z" or . == "1970-01-01T00:00:01Z" or . == "1970-01-01T00:00:02Z" or . == "1970-01-01T00:00:03Z" or . == "1970-01-01T00:00:05Z" or . == "1970-01-01T00:00:10Z" or . == "1970-01-01T00:00:12Z" or . == "1970-01-01T00:00:13Z" or . == "1970-01-01T00:00:25Z" or . == "1970-01-01T00:00:26Z" or . == "1970-01-01T00:00:27Z" or . == "1970-01-01T00:00:36Z")) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # Ord 4 alternates create and consume across its ten waves; ord 5 pairs
    # create and s0 across four accepted sends; ord 8 listens to s0 only and
    # emits six records.  34 listener records.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "consume"]; 10) + repa(["create", "s0"]; 4) + repv("s0"; 6))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10] + [1, 1, 2, 2, 3, 3, 4, 4] + [1, 2, 3, 4, 5, 6])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    ([.records[] | select(.operation == "snapshot" and .case == "time-window-scene-two") | .statement] | all(. == "create")) and
    ([.records[] | select(.operation == "snapshot" and .case == "time-first-window") | .statement] | all(. == "create")) and
    ([.records[] | select(.operation == "snapshot" and .case == "ext-time-window-scene-three") | .statement] | all(. == "s0")) and
    # the six empty-iterator snapshots emit the record with the new key
    # omitted: two in ord 4, one in ord 5 and three in ord 8.
    ([.records[] | select(.operation == "snapshot" and (has("new") | not))] | length == 6) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | .case] == (repv("time-window-scene-two"; 2) + ["time-first-window"] + repv("ext-time-window-scene-three"; 3))) and
    ([.records[] | select(.operation == "snapshot" and has("new")) | (.new | length) > 0] | all) and
    # every listener record carries at least one stream and every row projects
    # exactly the case fields: key/value for ords 4 and 5, theString for ord 8.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | select(.case == "time-window-scene-two") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-first-window") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "ext-time-window-scene-three") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["theString"]] | all) and
    (crec("time-window-scene-two") == twst) and
    (crec("time-first-window") == tfw) and
    (crec("ext-time-window-scene-three") == ext)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-time-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
