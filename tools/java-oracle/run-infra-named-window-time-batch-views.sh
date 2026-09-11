#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-time-batch-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-time-batch-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews time-batch-slice: the time_batch(10 sec) map and projection windows with buffered arrivals and boundary flushes, the late aggregate consumer whose preload is skipped, and the time_length_batch(10 sec, 3|4) windows with their size-or-time dual trigger under virtual time (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-1ad42a8ed8025c4a0730", "java-runtime-1eb11f5cf275069ef6b5", "java-runtime-bb926d092e7110db203f", "java-runtime-22bf6b3644a24862df7d", "java-runtime-dd924e1b7e500df135f8"]) and
    (.javaNames == ["InfraTimeBatch", "InfraTimeBatchSceneTwo", "InfraTimeBatchLateConsumer", "InfraTimeLengthBatch", "InfraTimeLengthBatchSceneTwo"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["time-batch", "time-batch-scene-two", "time-batch-late-consumer", "time-length-batch", "time-length-batch-scene-two"]) and
    ([.cases[].ordinal] == [16, 17, 18, 23, 24]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "time_batch(10 sec) window over the key/value map schema: arrivals buffer silently, deletes remove pending rows without a callback, and boundary flushes deliver the buffered rows as one new-data batch followed by an old-only flush of the previous batch",
        "legacy win:time_batch(10) projection window over theString/intBoxed with three module deployments and only the window statement listened; the anchor stays at the first arrival even across a silent empty flush",
        "time_batch(10 sec) window over the key/value map schema with a late-deployed aggregate consumer whose batch preload is skipped, so the first flush reports the sum of the whole batch including rows that arrived before the consumer existed",
        "time_length_batch(10 sec, 3) window over the key/value map schema: the size trigger flushes immediately on the third arrival and re-arms the boundary, after which deletes stay silent and the time trigger flushes the remaining row with the prior batch as old data",
        "legacy win:time_length_batch(10 sec, 4) projection window over theString/intBoxed with three module deployments and only the window statement listened across the silent-delete, size and time trigger timeline"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  The advance-time steps carry the absolute
    # RFC3339 instant the suite passes to advanceTime/sendTimer; the batch
    # flushes of this slice fire inside them with no send.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"] or . == ["at", "case", "op"])) and
    # iteratorSnapshots is the execution'"'"'s assertPropsPerRowIterator call-site
    # count: five for ord 16 (J:1607/1611/1628/1643/1646), eight for ord 17
    # (J:1682/1689/1696/1702/1723/1740/1760/1772), one for ord 18 (J:1817), six
    # for ord 23 (J:2256/2262/2267/2280/2285/2290) and eleven for ord 24
    # (J:2340/2347/2354/2360/2373/2380/2387/2406/2419/2429/2439).
    ([.cases[].iteratorSnapshots] == [5, 8, 1, 6, 11]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "delete"], ["create", "insert", "s0"], ["create", "insert", "s0", "delete"], ["create", "insert", "delete"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create"], ["s0"], ["create", "s0"], ["create"]]) and
    # per-case step counts: time-batch = 1 case + 4 deploys + 7 advance-time + 6
    # sends + 5 snapshots + 1 undeploy-all = 24; time-batch-scene-two =
    # 1 + 3 + 8 + 14 + 8 + 1 = 35; time-batch-late-consumer =
    # 1 + 3 + 5 + 3 + 1 + 1 = 14; time-length-batch = 1 + 4 + 4 + 8 + 6 + 1 =
    # 24; time-length-batch-scene-two = 1 + 3 + 11 + 14 + 11 + 1 = 41
    # (24+35+14+24+41 = 138).
    ([.steps[] | select(.case == "time-batch")] | length == 24) and
    ([.steps[] | select(.case == "time-batch-scene-two")] | length == 35) and
    ([.steps[] | select(.case == "time-batch-late-consumer")] | length == 14) and
    ([.steps[] | select(.case == "time-length-batch")] | length == 24) and
    ([.steps[] | select(.case == "time-length-batch-scene-two")] | length == 41) and
    (.steps | type == "array" and length == 138) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 5) and
    ([.steps[] | select(.op == "deploy")] | length == 17) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 4) and
    ([.steps[] | select(.op == "send")] | length == 45) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 31) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 14) and
    ([.steps[] | select(.op == "snapshot")] | length == 31) and
    ([.steps[] | select(.op == "advance-time")] | length == 35) and
    # every snapshot targets the window statement (never the s0 consumer and
    # never the excluded delete trigger) and every snapshot is exact-order.
    ([.steps[] | select(.op == "snapshot") | .statement] | all(. == "create")) and
    ([.steps[] | select(.op == "snapshot") | .mode] | all(. == "ordered")) and
    # byte-exact EPL pins: the @public window creates of the multi-module cases
    # (ords 16 and 23 deliberately carry no @public), the legacy
    # win:time_batch(10) / win:time_length_batch(10 sec, 4) spellings of
    # ords 17/24, the ord-18 late consumer'"'"'s ungrouped sum over the map
    # window and the empty s0/consume slots of the projection cases.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowTB#time_batch(10 sec) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.win:time_batch(10) as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') @public create window MyWindowTBLC#time_batch(10 sec) as MySimpleKeyValueMap", "@name('"'"'create'"'"') create window MyWindowTLB#time_length_batch(10 sec, 3) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.win:time_length_batch(10 sec, 4) as select theString as key, intBoxed as value from SupportBean"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowTB select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean", "insert into MyWindowTBLC select theString as key, longBoxed as value from SupportBean", "insert into MyWindowTLB select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select key, value as value from MyWindowTB", "", "@name('"'"'s0'"'"') select sum(value) as value from MyWindowTBLC", "@name('"'"'s0'"'"') select key, value as value from MyWindowTLB", ""]) and
    ([.cases[].consumeEpl] == ["", "", "", "", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowTB as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowTLB as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl, .cases[4].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl, .cases[4].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[2].s0Epl, .cases[3].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[3].deleteEpl, .cases[4].deleteEpl]) and
    # send payload shapes: theString plus exactly the one value field each case
    # window reads (longBoxed for ords 16/18/23, intBoxed for ords 17/24); the
    # on-delete market trigger carries symbol.
    ([.steps[] | select(.op == "send" and (.case == "time-batch" or .case == "time-batch-late-consumer" or .case == "time-length-batch") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and (.case == "time-batch-scene-two" or .case == "time-length-batch-scene-two") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    # the pinned clock moves, one advance-time step per source advanceTime /
    # sendTimer call: ord 16 J:1599/1602/1605/1614/1618/1630/1648, ord 17
    # J:1674/1681/1703/1709/1724/1731/1745/1761, ord 18 J:1795/1801/1804/1811/
    # 1815, ord 23 J:2251/2282/2292/2296 and ord 24 J:2332/2339/2361/2367/2374/
    # 2388/2395/2397/2411/2427/2430.  The repeated instants are ord 18'"'"'s
    # sendTimer(0) pair (the second call is a no-op that still replays) and the
    # .999 moves that precede the 11000/21000/25000/35000 boundaries.
    ([.steps[] | select(.op == "advance-time" and .case == "time-batch") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:10.999Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:31Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-batch-scene-two") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:18Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:22Z", "1970-01-01T00:00:31Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-batch-late-consumer") | .at] == ["1970-01-01T00:00:00Z", "1970-01-01T00:00:00Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:08Z", "1970-01-01T00:00:10Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-length-batch") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:10.999Z", "1970-01-01T00:00:11Z"]) and
    ([.steps[] | select(.op == "advance-time" and .case == "time-length-batch-scene-two") | .at] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:16Z", "1970-01-01T00:00:18Z", "1970-01-01T00:00:24.999Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:28Z", "1970-01-01T00:00:34.999Z", "1970-01-01T00:00:35Z"]) and
    # placement pins: ords 16 and 23 open with their four-statement module,
    # ords 17 and 24 with three single-statement modules and ord 18 with the
    # sendTimer(0) pin before its two-statement module; ord 18'"'"'s consumer
    # deploy sits at index 8, after the 5000 move and its send and before the
    # 8000 move, which is the source position of J:1808-1809; every case ends
    # with undeploy-all.
    ([.steps[] | select(.case == "time-batch")] | .[0] == {"op": "case", "case": "time-batch"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].statement == "delete" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:01Z" and .[23].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-batch-scene-two")] | .[0] == {"op": "case", "case": "time-batch-scene-two"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "delete" and .[4].op == "advance-time" and .[4].at == "1970-01-01T00:00:01Z" and .[34].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-batch-late-consumer")] | .[0] == {"op": "case", "case": "time-batch-late-consumer"} and .[1].op == "advance-time" and .[1].at == "1970-01-01T00:00:00Z" and .[2].statement == "create" and .[3].statement == "insert" and .[4].op == "advance-time" and .[4].at == "1970-01-01T00:00:00Z" and .[5].op == "send" and .[6].op == "advance-time" and .[6].at == "1970-01-01T00:00:05Z" and .[7].op == "send" and .[8].op == "deploy" and .[8].statement == "s0" and .[9].op == "advance-time" and .[9].at == "1970-01-01T00:00:08Z" and .[12].op == "snapshot" and .[13].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-length-batch")] | .[0] == {"op": "case", "case": "time-length-batch"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].statement == "delete" and .[5].op == "advance-time" and .[5].at == "1970-01-01T00:00:01Z" and .[23].op == "undeploy-all") and
    ([.steps[] | select(.case == "time-length-batch-scene-two")] | .[0] == {"op": "case", "case": "time-length-batch-scene-two"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "delete" and .[4].op == "advance-time" and .[4].at == "1970-01-01T00:00:01Z" and .[40].op == "undeploy-all")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-time-batch-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-time-batch-views.XXXXXX")
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
    "$script_root/InfraNamedWindowTimeBatchViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowTimeBatchViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def v($n): {"kind": "row", "fields": {"value": $n}};
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
    # Ord 16 (InfraTimeBatch, lines 1593-1652): arrivals buffer silently and the
    # window iterator shows the pending batch only, so S1 lists E1/E2/E3 and S2
    # drops the deleted E2.  The 11000 boundary fires ONE flush: the create
    # listener gets the two buffered rows as new data (no old data, first
    # flush) and the istream s0 consumer sees the same rows; the iterator is
    # empty right after.  The 21000 flush has an empty pending batch, so the
    # create listener gets old-only rows (the 11000 batch) and s0 is NOT
    # invoked, and the 31000 flush finds both batches empty and emits nothing.
    def tb: [
        snapr("time-batch"; "create"; "1970-01-01T00:00:10Z"; [r("E1"; 1), r("E2"; 2), r("E3"; 3)]),
        snapr("time-batch"; "create"; "1970-01-01T00:00:10Z"; [r("E1"; 1), r("E3"; 3)]),
        lisn("time-batch"; "create"; 1; "1970-01-01T00:00:11Z"; [r("E1"; 1), r("E3"; 3)]),
        lisn("time-batch"; "s0"; 1; "1970-01-01T00:00:11Z"; [r("E1"; 1), r("E3"; 3)]),
        snap0("time-batch"; "create"; "1970-01-01T00:00:11Z"),
        liso("time-batch"; "create"; 2; "1970-01-01T00:00:21Z"; [r("E1"; 1), r("E3"; 3)]),
        snapr("time-batch"; "create"; "1970-01-01T00:00:21Z"; [r("E4"; 4)]),
        snap0("time-batch"; "create"; "1970-01-01T00:00:21Z")
    ];
    # Ord 17 (InfraTimeBatchSceneTwo, lines 1662-1775): only create is listened
    # and rows project key/value with Integer values.  The G2/G1 deletes empty
    # the pending batch silently, so the 11000 flush emits nothing and does not
    # re-arm: the 15000 batch flushes at 21000 because the anchor is still the
    # first arrival at 1000.  G5 is deleted at 15000 and G8 is the only
    # survivor of the 22000 batch, whose 31000 flush returns the 21000 batch as
    # three old rows in insertion order.
    def tb2: [
        snapr("time-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1)]),
        snapr("time-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1), r("G2"; 2)]),
        snapr("time-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1)]),
        snap0("time-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"),
        snapr("time-batch-scene-two"; "create"; "1970-01-01T00:00:15Z"; [r("G3"; 3), r("G4"; 4)]),
        lisn("time-batch-scene-two"; "create"; 1; "1970-01-01T00:00:21Z"; [r("G3"; 3), r("G4"; 4), r("G6"; 6)]),
        snap0("time-batch-scene-two"; "create"; "1970-01-01T00:00:21Z"),
        snapr("time-batch-scene-two"; "create"; "1970-01-01T00:00:22Z"; [r("G8"; 8)]),
        lis("time-batch-scene-two"; "create"; 2; "1970-01-01T00:00:31Z"; [r("G8"; 8)]; [r("G3"; 3), r("G4"; 4), r("G6"; 6)]),
        snap0("time-batch-scene-two"; "create"; "1970-01-01T00:00:31Z")
    ];
    # Ord 18 (InfraTimeBatchLateConsumer, lines 1795-1819): the aggregate
    # consumer is deployed at clock 5000, after E1 and E2 are already buffered,
    # and its batch preload is skipped, so the 10000 flush of the whole batch
    # (1+2+3) reports 6L in ONE new row with no old row; the create statement
    # has no listener and its iterator is empty after the flush.
    def tblc: [
        lisn("time-batch-late-consumer"; "s0"; 1; "1970-01-01T00:00:10Z"; [v(6)]),
        snap0("time-batch-late-consumer"; "create"; "1970-01-01T00:00:10Z")
    ];
    # Ord 23 (InfraTimeLengthBatch, lines 2245-2310): the E4 arrival reaches the
    # size trigger of 3 at t=1000 and flushes immediately (new = E1/E3/E4 in
    # insertion order, no old data), re-arming the boundary to 11000; the E5
    # delete at 5000 and the silent 10999 move precede the time-triggered flush
    # of E6 with the size-flushed batch as three old rows, which the istream s0
    # consumer drops while still delivering the new row.
    def tlb: [
        snapr("time-length-batch"; "create"; "1970-01-01T00:00:01Z"; [r("E1"; 1), r("E2"; 2)]),
        snapr("time-length-batch"; "create"; "1970-01-01T00:00:01Z"; [r("E1"; 1)]),
        snapr("time-length-batch"; "create"; "1970-01-01T00:00:01Z"; [r("E1"; 1), r("E3"; 3)]),
        lisn("time-length-batch"; "create"; 1; "1970-01-01T00:00:01Z"; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        lisn("time-length-batch"; "s0"; 1; "1970-01-01T00:00:01Z"; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        snap0("time-length-batch"; "create"; "1970-01-01T00:00:01Z"),
        snapr("time-length-batch"; "create"; "1970-01-01T00:00:05Z"; [r("E5"; 5), r("E6"; 6)]),
        snapr("time-length-batch"; "create"; "1970-01-01T00:00:05Z"; [r("E6"; 6)]),
        lis("time-length-batch"; "create"; 2; "1970-01-01T00:00:11Z"; [r("E6"; 6)]; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        lisn("time-length-batch"; "s0"; 2; "1970-01-01T00:00:11Z"; [r("E6"; 6)])
    ];
    # Ord 24 (InfraTimeLengthBatchSceneTwo, lines 2320-2442): only create is
    # listened.  G2/G1 delete leaves an empty batch so the 11000 move is silent;
    # the 15000 batch survives the G5 delete and flushes at 25000 (time trigger,
    # no old data because it is the first callback of the case); the 28000 batch
    # loses G7 and G9 and its G8 survivor flushes at 35000 together with the
    # 25000 batch as three old rows.  The snapshots of S10 is asserted at 34999.
    def tlb2: [
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1)]),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1), r("G2"; 2)]),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"; [r("G1"; 1)]),
        snap0("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:05Z"),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:15Z"; [r("G3"; 3), r("G4"; 4)]),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:16Z"; [r("G3"; 3), r("G4"; 4), r("G5"; 5)]),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:16Z"; [r("G3"; 3), r("G4"; 4)]),
        lisn("time-length-batch-scene-two"; "create"; 1; "1970-01-01T00:00:25Z"; [r("G3"; 3), r("G4"; 4), r("G6"; 6)]),
        snap0("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:25Z"),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:28Z"; [r("G7"; 7), r("G8"; 8), r("G9"; 9)]),
        snapr("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:34.999Z"; [r("G8"; 8)]),
        lis("time-length-batch-scene-two"; "create"; 2; "1970-01-01T00:00:35Z"; [r("G8"; 8)]; [r("G3"; 3), r("G4"; 4), r("G6"; 6)]),
        snap0("time-length-batch-scene-two"; "create"; "1970-01-01T00:00:35Z")
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-time-batch-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 43) and
    # 8 + 10 + 2 + 10 + 13 records, 12 listener and 31 snapshot.
    ((crec("time-batch") | length) == 8) and
    ((crec("time-batch-scene-two") | length) == 10) and
    ((crec("time-batch-late-consumer") | length) == 2) and
    ((crec("time-length-batch") | length) == 10) and
    ((crec("time-length-batch-scene-two") | length) == 13) and
    ([.records[] | select(.operation == "listener")] | length == 12) and
    ([.records[] | select(.operation == "snapshot")] | length == 31) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("time-batch"; 8) + repv("time-batch-scene-two"; 10) + repv("time-batch-late-consumer"; 2) + repv("time-length-batch"; 10) + repv("time-length-batch-scene-two"; 13))) and
    # per-case listener/snapshot split: 3+5, 2+8, 1+1, 4+6 and 2+11.
    ((crec("time-batch") | map(select(.operation == "listener")) | length) == 3) and
    ((crec("time-batch") | map(select(.operation == "snapshot")) | length) == 5) and
    ((crec("time-batch-scene-two") | map(select(.operation == "listener")) | length) == 2) and
    ((crec("time-batch-scene-two") | map(select(.operation == "snapshot")) | length) == 8) and
    ((crec("time-batch-late-consumer") | map(select(.operation == "listener")) | length) == 1) and
    ((crec("time-batch-late-consumer") | map(select(.operation == "snapshot")) | length) == 1) and
    ((crec("time-length-batch") | map(select(.operation == "listener")) | length) == 4) and
    ((crec("time-length-batch") | map(select(.operation == "snapshot")) | length) == 6) and
    ((crec("time-length-batch-scene-two") | map(select(.operation == "listener")) | length) == 2) and
    ((crec("time-length-batch-scene-two") | map(select(.operation == "snapshot")) | length) == 11) and
    # per-case virtual times: every record lands on an advance-time boundary or
    # a send instant, never the wall clock.
    ([.records[] | select(.case == "time-batch") | .time] == ["1970-01-01T00:00:10Z", "1970-01-01T00:00:10Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z"]) and
    ([.records[] | select(.case == "time-batch-scene-two") | .time] == ["1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:21Z", "1970-01-01T00:00:22Z", "1970-01-01T00:00:31Z", "1970-01-01T00:00:31Z"]) and
    ([.records[] | select(.case == "time-batch-late-consumer") | .time] == ["1970-01-01T00:00:10Z", "1970-01-01T00:00:10Z"]) and
    ([.records[] | select(.case == "time-length-batch") | .time] == ["1970-01-01T00:00:01Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:01Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:11Z", "1970-01-01T00:00:11Z"]) and
    ([.records[] | select(.case == "time-length-batch-scene-two") | .time] == ["1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:15Z", "1970-01-01T00:00:16Z", "1970-01-01T00:00:16Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:25Z", "1970-01-01T00:00:28Z", "1970-01-01T00:00:34.999Z", "1970-01-01T00:00:35Z", "1970-01-01T00:00:35Z"]) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # Ord 16 alternates create/s0 for the single callback wave that reaches s0,
    # ords 17 and 24 listen to create alone, ord 18 to s0 alone and ord 23
    # alternates create/s0 across the size and time flushes.  12 listener
    # records and 31 snapshot records.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 1) + ["create"] + repv("create"; 2) + ["s0"] + repa(["create", "s0"]; 2) + repv("create"; 2))) and
    ([.records[] | select(.operation == "listener") | .sequence] == [1, 1, 2, 1, 2, 1, 1, 1, 2, 2, 1, 2]) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    ([.records[] | select(.operation == "snapshot") | .statement] | all(. == "create")) and
    # the ten empty-iterator snapshots emit the record with the new key
    # omitted: two in ord 16 (J:1628/1646), three in ord 17 (J:1702/1740/1772),
    # one in ord 18 (J:1817), one in ord 23 (J:2280) and three in ord 24
    # (J:2360/2406/2439).
    ([.records[] | select(.operation == "snapshot" and (has("new") | not))] | length == 10) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | .case] == (repv("time-batch"; 2) + repv("time-batch-scene-two"; 3) + repv("time-batch-late-consumer"; 1) + repv("time-length-batch"; 1) + repv("time-length-batch-scene-two"; 3))) and
    ([.records[] | select(.operation == "snapshot" and has("new")) | (.new | length) > 0] | all) and
    # every listener record carries at least one stream and every row projects
    # exactly the case fields: key/value for ords 16, 17, 23 and 24, value for
    # ord 18'"'"'s aggregate row.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | select(.case == "time-batch") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-batch-scene-two") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-batch-late-consumer") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["value"]] | all) and
    ([.records[] | select(.case == "time-length-batch") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "time-length-batch-scene-two") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    # the flush shapes: exactly three records carry both streams (the second
    # callback of ords 17/23/24), exactly four old arrays carry more than one
    # row (ord 16'"'"'s two-row old-only flush and the three-row old arrays of
    # ords 17/23/24), and the ord-18 sum row is the only single-column row.
    ([.records[] | select(has("new") and has("old"))] | length == 3) and
    ([.records[] | select(has("new") and has("old")) | [.case, .statement, .sequence]] == [["time-batch-scene-two", "create", 2], ["time-length-batch", "create", 2], ["time-length-batch-scene-two", "create", 2]]) and
    ([.records[] | select(.operation == "listener" and (.old | length) > 1) | [.case, .statement, .time, (.old | length)]] == [["time-batch", "create", "1970-01-01T00:00:21Z", 2], ["time-batch-scene-two", "create", "1970-01-01T00:00:31Z", 3], ["time-length-batch", "create", "1970-01-01T00:00:11Z", 3], ["time-length-batch-scene-two", "create", "1970-01-01T00:00:35Z", 3]]) and
    ([.records[] | select(.operation == "listener" and (.new | length) > 3)] | length == 0) and
    ([.records[] | select(.operation == "listener" and .case == "time-batch-late-consumer") | .new[0].fields] == [{"value": 6}]) and
    (crec("time-batch") == tb) and
    (crec("time-batch-scene-two") == tb2) and
    (crec("time-batch-late-consumer") == tblc) and
    (crec("time-length-batch") == tlb) and
    (crec("time-length-batch-scene-two") == tlb2)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-time-batch-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
