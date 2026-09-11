#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-time-order-accum-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-time-order-accum-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews time-order/accum-slice: the time_order(value, 10 sec) sliding window and its ext:time_order projection variant with pass-through rows, plus the time_accum map and projection windows whose multi-row expiry bursts release in insertion order under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-320f1b03eddb24520308", "java-runtime-ed5bfc247d088a603224", "java-runtime-d74b2c9c614b26973637", "java-runtime-8b4e2e7d81bb990c7d72"]) and
    (.javaNames == ["InfraTimeOrderWindow", "InfraTimeOrderSceneTwo", "InfraTimeAccum", "InfraTimeAccumSceneTwo"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["time-order-window", "time-order-scene-two", "time-accum", "time-accum-scene-two"]) and
    ([.cases[].ordinal] == [9, 10, 14, 15]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "time_order(value, 10 sec) window over the key/value map schema: rows are retained while value >= clock - 9999 and released old-only by timer waves during advance-time, with deletes releasing immediately and ascending-value iterators",
        "legacy ext:time_order(value, 10) projection window over theString/longBoxed with three module deployments and only the window statement listened; a stale arriving row passes through in one callback carrying both streams",
        "time_accum(10 sec) window over the key/value map schema: arrivals deliver immediately and re-arm the flush to their arrival plus ten seconds, whose bursts release all retained rows as old data in insertion order, with newest-row deletes re-arming and last-row deletes cancelling the timer",
        "legacy win:time_accum(10 sec) projection window over theString/intBoxed with four module deployments and only the window statement listened across the delete, re-arm and two-row burst timeline"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  The advance-time steps carry the absolute
    # RFC3339 instant the suite passes to advanceTime/sendTimer, and the
    # expiry waves of this slice fire inside them with no send.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"] or . == ["at", "case", "op"])) and
    # iteratorSnapshots is the execution'"'"'s assertPropsPerRowIterator call-site
    # count: four for ord 9 (J:943/949/955/964), ten for ord 10
    # (J:999/1007/1019/1027/1034/1041/1049/1057/1064/1071), eight for ord 14
    # (J:1331/1337/1354/1375/1381/1397/1403/1409) and fifteen for ord 15
    # (J:1451/1458/1461/1468/1471/1476/1484/1492/1499/1506/1520/1528/1536/1544/1551).
    ([.cases[].iteratorSnapshots] == [4, 10, 8, 15]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "delete"], ["create", "insert", "s0", "delete"], ["create", "insert", "consume", "delete"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create"], ["create", "s0"], ["create"]]) and
    # per-case step counts: time-order-window = 1 case + 4 deploys + 7
    # advance-time + 4 sends + 4 snapshots + 1 undeploy-all = 21;
    # time-order-scene-two = 1 + 3 + 14 + 8 + 10 + 1 = 37;
    # time-accum = 1 + 4 + 14 + 12 + 8 + 1 = 40;
    # time-accum-scene-two = 1 + 4 + 13 + 13 + 15 + 1 = 47 (21+37+40+47 = 145).
    ([.steps[] | select(.case == "time-order-window")] | length == 21) and
    ([.steps[] | select(.case == "time-order-scene-two")] | length == 37) and
    ([.steps[] | select(.case == "time-accum")] | length == 40) and
    ([.steps[] | select(.case == "time-accum-scene-two")] | length == 47) and
    (.steps | type == "array" and length == 145) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 15) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 4) and
    ([.steps[] | select(.op == "send")] | length == 37) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 25) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 12) and
    ([.steps[] | select(.op == "snapshot")] | length == 37) and
    ([.steps[] | select(.op == "advance-time")] | length == 48) and
    # all four cases snapshot the window statement (never the s0 consumer or
    # the excluded consume statement) and every snapshot is exact-order.
    ([.steps[] | select(.op == "snapshot") | .statement] | all(. == "create")) and
    ([.steps[] | select(.op == "snapshot") | .mode] | all(. == "ordered")) and
    # byte-exact EPL pins: the @public window creates of the multi-module cases
    # and of ord 9 (the ord-14 map window deliberately carries no @public), the
    # ext:time_order spelling of ord 10, the win:time_accum spelling of ord 15,
    # its named insert/consume statements and the "as s0" stream alias of the
    # ord-14 delete trigger.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') @public create window MyWindowTOW#time_order(value, 10 sec) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.ext:time_order(value, 10) as select theString as key, longBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowTA#time_accum(10 sec) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.win:time_accum(10 sec) as select theString as key, intBoxed as value from SupportBean"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowTOW select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, longBoxed from SupportBean", "insert into MyWindowTA select theString as key, longBoxed as value from SupportBean", "@name('"'"'insert'"'"') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select irstream key, value as value from MyWindowTOW", "", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowTA", ""]) and
    ([.cases[].consumeEpl] == ["", "", "", "@name('"'"'consume'"'"') select irstream key, value as value from MyWindow"]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindowTOW where symbol = key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowTA as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[2].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume") | .epl] == [.cases[3].consumeEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[2].deleteEpl, .cases[3].deleteEpl]) and
    # send payload shapes: theString plus exactly the one value field each case
    # window reads (longBoxed for ords 9/10/14, intBoxed for ord 15); the
    # on-delete market trigger carries symbol.
    ([.steps[] | select(.op == "send" and (.case == "time-order-window" or .case == "time-order-scene-two" or .case == "time-accum") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "time-accum-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    # the pinned clock moves, one advance-time step per source advanceTime
    # call: ord 9 J:929/934/939/946/957/961/966, ord 10 J:992/1000/1008/1020/
    # 1028/1035/1042/1044/1050/1052/1058/1065/1072/1074, ord 14 J:1312/1317/
    # 1322/1327/1340/1344/1361/1366/1371/1383/1387/1399/1405/1411 and ord 15
    # J:1443/1450/1466/1469/1477/1485/1507/1513/1521/1529/1537/1557/1559.  The
    # equal-time pairs are ord 10 at 20000 and 32000 and ord 15 around 29999
    # (the G4 arrival and both deletes share that instant); the .999 moves
    # precede the 13000/28000/32000/35000/11000/25000/41000/53000 boundaries.
    ([.steps[] | select(.op == "advance-time" and .case == "time-order-window") | .at] == ["1970-01-01T00:00:05Z", "1970-01-01T00:00:06Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:12.999Z", "1970-01-01T00:00:13Z", "1970-01-01T00:01:40Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-order-scene-two") | .at] == ["1970-01-01T00:00:20Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:22Z", "1970-01-01T00:00:23Z", "1970-01-01T00:00:27.999Z", "1970-01-01T00:00:28Z", "1970-01-01T00:00:31.999Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:34.999Z", "1970-01-01T00:00:35Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-accum") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:24.999Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:30Z", "1970-01-01T00:00:31Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:40.999Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:50Z", "1970-01-01T00:00:55Z", "1970-01-01T00:01:40Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-accum-scene-two") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10.999Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:40Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:42Z", "1970-01-01T00:00:43Z", "1970-01-01T00:00:44Z", "1970-01-01T00:00:52.999Z", "1970-01-01T00:00:53Z"]) and
    # placement pins: ord 9 opens with its four-statement module and only then
    # moves the clock; ords 10 and 15 deploy all their single-statement modules
    # before the first advance (no leading clock pin is needed because the
    # first sendTimer of every case is an absolute set); ord 14 moves to 1000
    # before its module deploy; every case ends with undeploy-all.
    ([.steps[] | select(.case == "time-order-window")] | .[0] == {"op": "case", "case": "time-order-window"} and .[1].op == "deploy" and .[2].op == "deploy" and .[3].op == "deploy" and .[4].op == "deploy" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:05Z" and .[20].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-order-scene-two")] | .[0] == {"op": "case", "case": "time-order-scene-two"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "delete" and .[4].op == "advance-time" and .[4].at == "1970-01-01T00:00:20Z" and .[36].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-accum")] | .[0] == {"op": "case", "case": "time-accum"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].statement == "delete" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:01Z" and .[39].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-accum-scene-two")] | .[0] == {"op": "case", "case": "time-accum-scene-two"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "consume" and .[4].statement == "delete" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:01Z" and .[16].op == "snapshot" and .[17].op == "snapshot" and .[46].op == "undeploy-all")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-time-order-accum-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-time-order-accum-views.XXXXXX")
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
    "$script_root/InfraNamedWindowTimeOrderAccumScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowTimeOrderAccumScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def rk($k): {"kind": "row", "fields": {"key": $k}};
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
    # Ord 9 (InfraTimeOrderWindow, lines 923-970): the window statement is
    # deployed first, so every wave delivers create before s0.  Rows are held
    # while value >= clock - 9999: E3 (1000) expires at exactly 11000, the E2
    # delete at 11000 releases old-only rows, the 12999 move stays silent after
    # the silent 12000 reschedule and E1 (3000) expires at exactly 13000.
    # Iterators list the retained rows in ascending value order and the four
    # snapshots are the J:943/949/955/964 states (the last one empty).
    def tow: [
        lisn("time-order-window"; "create"; 1; "1970-01-01T00:00:05Z"; [r("E1"; 3000)]),
        lisn("time-order-window"; "s0"; 1; "1970-01-01T00:00:05Z"; [r("E1"; 3000)]),
        lisn("time-order-window"; "create"; 2; "1970-01-01T00:00:06Z"; [r("E2"; 2000)]),
        lisn("time-order-window"; "s0"; 2; "1970-01-01T00:00:06Z"; [r("E2"; 2000)]),
        lisn("time-order-window"; "create"; 3; "1970-01-01T00:00:10Z"; [r("E3"; 1000)]),
        lisn("time-order-window"; "s0"; 3; "1970-01-01T00:00:10Z"; [r("E3"; 1000)]),
        snapr("time-order-window"; "create"; "1970-01-01T00:00:10Z"; [r("E3"; 1000), r("E2"; 2000), r("E1"; 3000)]),
        liso("time-order-window"; "create"; 4; "1970-01-01T00:00:11Z"; [r("E3"; 1000)]),
        liso("time-order-window"; "s0"; 4; "1970-01-01T00:00:11Z"; [r("E3"; 1000)]),
        snapr("time-order-window"; "create"; "1970-01-01T00:00:11Z"; [r("E2"; 2000), r("E1"; 3000)]),
        liso("time-order-window"; "create"; 5; "1970-01-01T00:00:11Z"; [r("E2"; 2000)]),
        liso("time-order-window"; "s0"; 5; "1970-01-01T00:00:11Z"; [r("E2"; 2000)]),
        snapr("time-order-window"; "create"; "1970-01-01T00:00:11Z"; [r("E1"; 3000)]),
        liso("time-order-window"; "create"; 6; "1970-01-01T00:00:13Z"; [r("E1"; 3000)]),
        liso("time-order-window"; "s0"; 6; "1970-01-01T00:00:13Z"; [r("E1"; 3000)]),
        snap0("time-order-window"; "create"; "1970-01-01T00:00:13Z")
    ];
    # Ord 10 (InfraTimeOrderSceneTwo, lines 980-1078): only create is listened
    # and rows project key alone.  G1 (23000) and G2 (19000) arrive at 20000;
    # the stale G3 (10000) is below the tail (21000 - 10000 + 1 = 11001) and
    # passes through in ONE callback carrying it as new AND old (J:1008-1014);
    # the G2 and G1 deletes release old-only rows; G4 (18000) at 22000 re-arms
    # the timer to 28000 and G5 (22000) at 23000 leaves it there; the 27999 and
    # 31999 moves stay silent before the 28000 and 32000 releases; G6 (25000)
    # arrives at 32000 and the 34999 move hides the silent 33000 reschedule
    # before the 35000 release.  Ten snapshots, all in ascending value order.
    def tw2: [
        lisn("time-order-scene-two"; "create"; 1; "1970-01-01T00:00:20Z"; [rk("G1")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:20Z"; [rk("G1")]),
        lisn("time-order-scene-two"; "create"; 2; "1970-01-01T00:00:20Z"; [rk("G2")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:20Z"; [rk("G2"), rk("G1")]),
        lis("time-order-scene-two"; "create"; 3; "1970-01-01T00:00:21Z"; [rk("G3")]; [rk("G3")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:21Z"; [rk("G2"), rk("G1")]),
        liso("time-order-scene-two"; "create"; 4; "1970-01-01T00:00:21Z"; [rk("G2")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:21Z"; [rk("G1")]),
        lisn("time-order-scene-two"; "create"; 5; "1970-01-01T00:00:22Z"; [rk("G4")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:22Z"; [rk("G4"), rk("G1")]),
        lisn("time-order-scene-two"; "create"; 6; "1970-01-01T00:00:23Z"; [rk("G5")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:23Z"; [rk("G4"), rk("G5"), rk("G1")]),
        liso("time-order-scene-two"; "create"; 7; "1970-01-01T00:00:28Z"; [rk("G4")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:28Z"; [rk("G5"), rk("G1")]),
        liso("time-order-scene-two"; "create"; 8; "1970-01-01T00:00:32Z"; [rk("G5")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:32Z"; [rk("G1")]),
        lisn("time-order-scene-two"; "create"; 9; "1970-01-01T00:00:32Z"; [rk("G6")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:32Z"; [rk("G1"), rk("G6")]),
        liso("time-order-scene-two"; "create"; 10; "1970-01-01T00:00:32Z"; [rk("G1")]),
        snapr("time-order-scene-two"; "create"; "1970-01-01T00:00:32Z"; [rk("G6")]),
        liso("time-order-scene-two"; "create"; 11; "1970-01-01T00:00:35Z"; [rk("G6")])
    ];
    # Ord 14 (InfraTimeAccum, lines 1306-1415): create and s0 listen, every
    # arrival is delivered immediately and re-arms the flush to arrival plus
    # ten seconds.  The E2 delete at 15000 is old-only and leaves the 25000
    # timer alone; the 25000 flush releases the retained E1/E3/E4 in ONE
    # callback as three old rows in insertion order (J:1344-1353), the E4
    # delete right after it matches nothing, the E7 delete at 38000 re-arms the
    # 48000 flush to 41000 whose burst releases E5/E6 as two old rows
    # (J:1387-1396), and the E8 delete at 55000 cancels the timer.  Eight
    # snapshots (J:1331/1337/1354/1375/1381/1397/1403/1409), three of them
    # empty and all in insertion order.
    def ta: [
        lisn("time-accum"; "create"; 1; "1970-01-01T00:00:01Z"; [r("E1"; 1)]),
        lisn("time-accum"; "s0"; 1; "1970-01-01T00:00:01Z"; [r("E1"; 1)]),
        lisn("time-accum"; "create"; 2; "1970-01-01T00:00:05Z"; [r("E2"; 2)]),
        lisn("time-accum"; "s0"; 2; "1970-01-01T00:00:05Z"; [r("E2"; 2)]),
        lisn("time-accum"; "create"; 3; "1970-01-01T00:00:10Z"; [r("E3"; 3)]),
        lisn("time-accum"; "s0"; 3; "1970-01-01T00:00:10Z"; [r("E3"; 3)]),
        lisn("time-accum"; "create"; 4; "1970-01-01T00:00:15Z"; [r("E4"; 4)]),
        lisn("time-accum"; "s0"; 4; "1970-01-01T00:00:15Z"; [r("E4"; 4)]),
        snapr("time-accum"; "create"; "1970-01-01T00:00:15Z"; [r("E1"; 1), r("E2"; 2), r("E3"; 3), r("E4"; 4)]),
        liso("time-accum"; "create"; 5; "1970-01-01T00:00:15Z"; [r("E2"; 2)]),
        liso("time-accum"; "s0"; 5; "1970-01-01T00:00:15Z"; [r("E2"; 2)]),
        snapr("time-accum"; "create"; "1970-01-01T00:00:15Z"; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        liso("time-accum"; "create"; 6; "1970-01-01T00:00:25Z"; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        liso("time-accum"; "s0"; 6; "1970-01-01T00:00:25Z"; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        snap0("time-accum"; "create"; "1970-01-01T00:00:25Z"),
        lisn("time-accum"; "create"; 7; "1970-01-01T00:00:30Z"; [r("E5"; 5)]),
        lisn("time-accum"; "s0"; 7; "1970-01-01T00:00:30Z"; [r("E5"; 5)]),
        lisn("time-accum"; "create"; 8; "1970-01-01T00:00:31Z"; [r("E6"; 6)]),
        lisn("time-accum"; "s0"; 8; "1970-01-01T00:00:31Z"; [r("E6"; 6)]),
        lisn("time-accum"; "create"; 9; "1970-01-01T00:00:38Z"; [r("E7"; 7)]),
        lisn("time-accum"; "s0"; 9; "1970-01-01T00:00:38Z"; [r("E7"; 7)]),
        snapr("time-accum"; "create"; "1970-01-01T00:00:38Z"; [r("E5"; 5), r("E6"; 6), r("E7"; 7)]),
        liso("time-accum"; "create"; 10; "1970-01-01T00:00:38Z"; [r("E7"; 7)]),
        liso("time-accum"; "s0"; 10; "1970-01-01T00:00:38Z"; [r("E7"; 7)]),
        snapr("time-accum"; "create"; "1970-01-01T00:00:38Z"; [r("E5"; 5), r("E6"; 6)]),
        liso("time-accum"; "create"; 11; "1970-01-01T00:00:41Z"; [r("E5"; 5), r("E6"; 6)]),
        liso("time-accum"; "s0"; 11; "1970-01-01T00:00:41Z"; [r("E5"; 5), r("E6"; 6)]),
        snap0("time-accum"; "create"; "1970-01-01T00:00:41Z"),
        lisn("time-accum"; "create"; 12; "1970-01-01T00:00:50Z"; [r("E8"; 8)]),
        lisn("time-accum"; "s0"; 12; "1970-01-01T00:00:50Z"; [r("E8"; 8)]),
        snapr("time-accum"; "create"; "1970-01-01T00:00:50Z"; [r("E8"; 8)]),
        liso("time-accum"; "create"; 13; "1970-01-01T00:00:55Z"; [r("E8"; 8)]),
        liso("time-accum"; "s0"; 13; "1970-01-01T00:00:55Z"; [r("E8"; 8)]),
        snap0("time-accum"; "create"; "1970-01-01T00:00:55Z")
    ];
    # Ord 15 (InfraTimeAccumSceneTwo, lines 1425-1572): only create is listened.
    # The G2 delete at 5000 is a newest-row delete, so the flush moves from 15000
    # to 11000; the 10999 move stays silent and the 11000 burst releases the
    # single retained G1, followed by the two consecutive empty iterator states
    # (J:1471/1476).  G4 (29999) re-arms the flush to 39999, the G3 delete
    # leaves it and the G4 delete cancels it, so the 40000 move is silent.  The
    # G6 delete at 44000 leaves the 54000 timer alone while the G8 delete
    # re-arms it to 53000, whose burst releases G5 and G7 as two old rows in
    # insertion order (J:1559-1566).  Fifteen snapshots in engine order; the
    # snapshot of J:1484 is taken between the G3 send and the 29999 advance
    # exactly as the source orders it, so it carries t=20000.
    def tas: [
        lisn("time-accum-scene-two"; "create"; 1; "1970-01-01T00:00:01Z"; [r("G1"; 1)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1)]),
        lisn("time-accum-scene-two"; "create"; 2; "1970-01-01T00:00:05Z"; [r("G2"; 2)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1), r("G2"; 2)]),
        liso("time-accum-scene-two"; "create"; 3; "1970-01-01T00:00:05Z"; [r("G2"; 2)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:10.999Z"; [r("G1"; 1)]),
        liso("time-accum-scene-two"; "create"; 4; "1970-01-01T00:00:11Z"; [r("G1"; 1)]),
        snap0("time-accum-scene-two"; "create"; "1970-01-01T00:00:11Z"),
        snap0("time-accum-scene-two"; "create"; "1970-01-01T00:00:11Z"),
        lisn("time-accum-scene-two"; "create"; 5; "1970-01-01T00:00:20Z"; [r("G3"; 3)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:20Z"; [r("G3"; 3)]),
        lisn("time-accum-scene-two"; "create"; 6; "1970-01-01T00:00:29.999Z"; [r("G4"; 4)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:29.999Z"; [r("G3"; 3), r("G4"; 4)]),
        liso("time-accum-scene-two"; "create"; 7; "1970-01-01T00:00:29.999Z"; [r("G3"; 3)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:29.999Z"; [r("G4"; 4)]),
        liso("time-accum-scene-two"; "create"; 8; "1970-01-01T00:00:29.999Z"; [r("G4"; 4)]),
        snap0("time-accum-scene-two"; "create"; "1970-01-01T00:00:29.999Z"),
        lisn("time-accum-scene-two"; "create"; 9; "1970-01-01T00:00:41Z"; [r("G5"; 5)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:41Z"; [r("G5"; 5)]),
        lisn("time-accum-scene-two"; "create"; 10; "1970-01-01T00:00:42Z"; [r("G6"; 6)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:42Z"; [r("G5"; 5), r("G6"; 6)]),
        lisn("time-accum-scene-two"; "create"; 11; "1970-01-01T00:00:43Z"; [r("G7"; 7)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:43Z"; [r("G5"; 5), r("G6"; 6), r("G7"; 7)]),
        lisn("time-accum-scene-two"; "create"; 12; "1970-01-01T00:00:44Z"; [r("G8"; 8)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:44Z"; [r("G5"; 5), r("G6"; 6), r("G7"; 7), r("G8"; 8)]),
        liso("time-accum-scene-two"; "create"; 13; "1970-01-01T00:00:44Z"; [r("G6"; 6)]),
        snapr("time-accum-scene-two"; "create"; "1970-01-01T00:00:44Z"; [r("G5"; 5), r("G7"; 7), r("G8"; 8)]),
        liso("time-accum-scene-two"; "create"; 14; "1970-01-01T00:00:44Z"; [r("G8"; 8)]),
        liso("time-accum-scene-two"; "create"; 15; "1970-01-01T00:00:53Z"; [r("G5"; 5), r("G7"; 7)])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-time-order-accum-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 101) and
    # 16 + 21 + 34 + 30 records, 64 listener and 37 snapshot.
    ((crec("time-order-window") | length) == 16) and
    ((crec("time-order-scene-two") | length) == 21) and
    ((crec("time-accum") | length) == 34) and
    ((crec("time-accum-scene-two") | length) == 30) and
    ([.records[] | select(.operation == "listener")] | length == 64) and
    ([.records[] | select(.operation == "snapshot")] | length == 37) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("time-order-window"; 16) + repv("time-order-scene-two"; 21) + repv("time-accum"; 34) + repv("time-accum-scene-two"; 30))) and
    # per-case listener/snapshot split: 12+4, 11+10, 26+8 and 15+15.
    ((crec("time-order-window") | map(select(.operation == "listener")) | length) == 12) and
    ((crec("time-order-window") | map(select(.operation == "snapshot")) | length) == 4) and
    ((crec("time-order-scene-two") | map(select(.operation == "listener")) | length) == 11) and
    ((crec("time-order-scene-two") | map(select(.operation == "snapshot")) | length) == 10) and
    ((crec("time-accum") | map(select(.operation == "listener")) | length) == 26) and
    ((crec("time-accum") | map(select(.operation == "snapshot")) | length) == 8) and
    ((crec("time-accum-scene-two") | map(select(.operation == "listener")) | length) == 15) and
    ((crec("time-accum-scene-two") | map(select(.operation == "snapshot")) | length) == 15) and
    # per-case virtual times: every advance-time boundary of the case plus its
    # send instants, never the wall clock.
    ([.records[] | select(.case == "time-order-window") | .time] == ["1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:06Z", "1970-01-01T00:00:06Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:13Z", "1970-01-01T00:00:13Z", "1970-01-01T00:00:13Z"]) and
    ([.records[] | select(.case == "time-order-scene-two") | .time] == ["1970-01-01T00:00:20Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:22Z", "1970-01-01T00:00:22Z", "1970-01-01T00:00:23Z", "1970-01-01T00:00:23Z", "1970-01-01T00:00:28Z", "1970-01-01T00:00:28Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:32Z", "1970-01-01T00:00:35Z"]) and
    ([.records[] | select(.case == "time-accum") | .time] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:30Z", "1970-01-01T00:00:30Z", "1970-01-01T00:00:31Z", "1970-01-01T00:00:31Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:38Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:50Z", "1970-01-01T00:00:50Z", "1970-01-01T00:00:50Z", "1970-01-01T00:00:55Z", "1970-01-01T00:00:55Z", "1970-01-01T00:00:55Z"]) and
    ([.records[] | select(.case == "time-accum-scene-two") | .time] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10.999Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:20Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:29.999Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:41Z", "1970-01-01T00:00:42Z", "1970-01-01T00:00:42Z", "1970-01-01T00:00:43Z", "1970-01-01T00:00:43Z", "1970-01-01T00:00:44Z", "1970-01-01T00:00:44Z", "1970-01-01T00:00:44Z", "1970-01-01T00:00:44Z", "1970-01-01T00:00:44Z", "1970-01-01T00:00:53Z"]) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # Ord 9 alternates create and s0 across six waves, ord 10 listens to create
    # alone, ord 14 alternates create and s0 across thirteen waves and ord 15
    # listens to create alone.  64 listener records.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 6) + repv("create"; 11) + repa(["create", "s0"]; 13) + repv("create"; 15))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6] + [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11] + [1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13] + [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    ([.records[] | select(.operation == "snapshot") | .statement] | all(. == "create")) and
    # the seven empty-iterator snapshots emit the record with the new key
    # omitted: one in ord 9, three in ord 14 and three in ord 15.
    ([.records[] | select(.operation == "snapshot" and (has("new") | not))] | length == 7) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | .case] == (["time-order-window"] + repv("time-accum"; 3) + repv("time-accum-scene-two"; 3))) and
    ([.records[] | select(.operation == "snapshot" and has("new")) | (.new | length) > 0] | all) and
    # every listener record carries at least one stream and every row projects
    # exactly the case fields: key/value for ords 9, 14 and 15, key for ord 10.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | select(.case == "time-order-window") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-order-scene-two") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key"]] | all) and
    ([.records[] | select(.case == "time-accum") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-accum-scene-two") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    # the multi-row bursts are single callbacks: ord 14 releases three rows at
    # 25000 and two at 41000 next to the ord-15 two-row 53000 burst, and no
    # other record carries more than one old row except the ord-10 pass-through
    # whose single callback carries the same row as new and old.
    ([.records[] | select(.operation == "listener" and (.old | length) > 1) | [.case, .statement, .time, (.old | length)]] == [["time-accum", "create", "1970-01-01T00:00:25Z", 3], ["time-accum", "s0", "1970-01-01T00:00:25Z", 3], ["time-accum", "create", "1970-01-01T00:00:41Z", 2], ["time-accum", "s0", "1970-01-01T00:00:41Z", 2], ["time-accum-scene-two", "create", "1970-01-01T00:00:53Z", 2]]) and
    # the ord-10 pass-through is the only record carrying both streams of the
    # same row.
    ([.records[] | select(has("new") and has("old")) | [.case, .statement, .sequence] == ["time-order-scene-two", "create", 3]] | all) and
    ([.records[] | select(has("new") and has("old"))] | length == 1) and
    (crec("time-order-window") == tow) and
    (crec("time-order-scene-two") == tw2) and
    (crec("time-accum") == ta) and
    (crec("time-accum-scene-two") == tas)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-time-order-accum-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
