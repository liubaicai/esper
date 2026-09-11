#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-lengthbatch-sort-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-lengthbatch-sort-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews length-batch/sort-slice: the MySimpleKeyValueMap length_batch and sort windows, their projection variants and the sort-window delete trigger, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-81f4d92e62dbafdd7f2f", "java-runtime-ce4a95ec7941743a7bfe", "java-runtime-7edfd8382e4eee6892dc", "java-runtime-5fb3e9a4d20bc3964f5f"]) and
    (.javaNames == ["InfraLengthBatch", "InfraLengthBatchSceneTwo", "InfraSortWindow", "InfraSortWindowSceneTwo"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["length-batch", "length-batch-scene-two", "sort-window", "sort-window-scene-two"]) and
    ([.cases[].ordinal] == [19, 20, 21, 22]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "length_batch(3) window over the key/value map schema: the batch releases only at the size boundary, carrying the buffered rows together with the rows replaced by the batch, and deletes remove rows without a pending flush",
        "legacy win:length_batch(3) projection window over theString/intBoxed with three module deployments and only the window statement listened across the full three-flush timeline",
        "sort(3, value asc) window over the key/value map schema: retention keeps the three lowest values with recency tie ordering, expelling the maximum on overflow and reflecting deletes in the ordered snapshots",
        "legacy ext:sort(3, value) projection window over theString/intBoxed with three module deployments, ordered retention and the delete trigger"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  consumeEpl is empty for every case of this
    # slice; the two scene-two cases have no consumer statement at all, so
    # their s0Epl is empty and their deploys carry no s0.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"])) and
    ([.cases[].iteratorSnapshots] == [5, 23, 9, 6]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "delete"], ["create", "insert", "s0", "delete"], ["create", "insert", "delete"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create"], ["create", "s0"], ["create"]]) and
    # per-case step counts: length-batch = 1 case + 4 deploys + 18 sends + 5
    # snapshots + 1 undeploy-all = 29; length-batch-scene-two = 1 + 3 + 19 +
    # 23 + 1 = 47; sort-window = 1 + 4 + 12 + 9 + 1 = 27;
    # sort-window-scene-two = 1 + 3 + 7 + 6 + 1 = 18 (29+47+27+18 = 121).
    ([.steps[] | select(.case == "length-batch")] | length == 29) and
    ([.steps[] | select(.case == "length-batch-scene-two")] | length == 47) and
    ([.steps[] | select(.case == "sort-window")] | length == 27) and
    ([.steps[] | select(.case == "sort-window-scene-two")] | length == 18) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 14) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 43) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 13) and
    ([.steps[] | select(.op == "snapshot")] | length == 43) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create")] | length == 43) and
    # every snapshot of this slice is exact-order; the any-mode protocol branch
    # of the shared oracle never fires here.
    ([.steps[] | select(.op == "snapshot") | .mode] | all(. == "ordered")) and
    # byte-exact EPL pins, including the legacy MyWindow.win:length_batch(3)
    # and MyWindow.ext:sort(3, value) spellings with @public, the "value asc"
    # criterion and the istream-only s0 consumers of ords 19/21.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowLB#length_batch(3) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.win:length_batch(3) as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowSW#sort(3, value asc) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.ext:sort(3, value) as select theString as key, intBoxed as value from SupportBean"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowLB select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean", "insert into MyWindowSW select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select key, value as value from MyWindowLB", "", "@name('"'"'s0'"'"') select key, value as value from MyWindowSW", ""]) and
    ([.cases[].consumeEpl] == ["", "", "", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowLB as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowSW as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[2].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[2].deleteEpl, .cases[3].deleteEpl]) and
    # send payload shapes: theString plus exactly the one value field the case
    # window projects (longBoxed for the map-schema windows, intBoxed for the
    # projection windows), and the on-delete market trigger carries symbol.
    ([.steps[] | select(.op == "send" and (.case == "length-batch" or .case == "sort-window") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and (.case == "length-batch-scene-two" or .case == "sort-window-scene-two") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    (.steps | type == "array" and length == 121)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-lengthbatch-sort-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-lengthbatch-sort-views.XXXXXX")
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
    "$script_root/InfraNamedWindowLengthBatchSortScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowLengthBatchSortScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def snapr($c; $rows): {"case": $c, "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    # snap0 is the empty-iterator snapshot record: the new key is omitted, not
    # an empty array, exactly like the Go normalizer renders it.
    def snap0($c): {"case": $c, "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z"};
    def lis($c; $st; $q; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new, "old": $old};
    def lisn($c; $st; $q; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    def liso($c; $st; $q; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "old": $old};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 19 (InfraLengthBatch, lines 1826-1901): the create window statement
    # is deployed before s0, so every flush wave delivers create first and then
    # the istream-only s0; the flush carries the whole completed batch as new
    # data in ONE invocation and the prior batch as old data in the SAME
    # invocation (the first flush has no old stream), while deletes of
    # unflushed batch rows produce no callback anywhere and merely shrink the
    # iterator.  E2 is deleted before the first flush, and the two E10 sends
    # are both removed by the one symbol=E10 delete.
    def lb: [
        snapr("length-batch"; [r("E1"; 1), r("E2"; 2)]),
        snapr("length-batch"; [r("E1"; 1)]),
        snapr("length-batch"; [r("E1"; 1), r("E3"; 3)]),
        lisn("length-batch"; "create"; 1; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        lisn("length-batch"; "s0"; 1; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        snap0("length-batch"),
        snap0("length-batch"),
        lis("length-batch"; "create"; 2; [r("E7"; 7), r("E8"; 8), r("E9"; 9)]; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        lisn("length-batch"; "s0"; 2; [r("E7"; 7), r("E8"; 8), r("E9"; 9)]),
        lis("length-batch"; "create"; 3; [r("E21"; 21), r("E22"; 22), r("E23"; 23)]; [r("E7"; 7), r("E8"; 8), r("E9"; 9)]),
        lisn("length-batch"; "s0"; 3; [r("E21"; 21), r("E22"; 22), r("E23"; 23)])
    ];
    # Ord 20 (InfraLengthBatchSceneTwo, lines 1911-2079): three modules, the
    # create listener only (there is no consumer statement), the three flushes
    # at G6/G10/G14 (the first without an old stream), the five silent deletes
    # and the twenty-three ordered snapshots the source asserts, seven of them
    # observing the emptied current batch.  The pairs of identical snapshots
    # re-assert the same window across a milestone.
    def lbst: [
        snapr("length-batch-scene-two"; [r("G1"; 10), r("G2"; 20)]),
        snapr("length-batch-scene-two"; [r("G1"; 10)]),
        snapr("length-batch-scene-two"; [r("G1"; 10)]),
        snap0("length-batch-scene-two"),
        snap0("length-batch-scene-two"),
        snapr("length-batch-scene-two"; [r("G3"; 30)]),
        snapr("length-batch-scene-two"; [r("G3"; 30), r("G4"; 40)]),
        snapr("length-batch-scene-two"; [r("G3"; 30), r("G4"; 40)]),
        snapr("length-batch-scene-two"; [r("G3"; 30)]),
        snapr("length-batch-scene-two"; [r("G3"; 30), r("G5"; 50)]),
        lisn("length-batch-scene-two"; "create"; 1; [r("G3"; 30), r("G5"; 50), r("G6"; 60)]),
        snap0("length-batch-scene-two"),
        snap0("length-batch-scene-two"),
        snapr("length-batch-scene-two"; [r("G7"; 70)]),
        snapr("length-batch-scene-two"; [r("G7"; 70)]),
        snapr("length-batch-scene-two"; [r("G7"; 70), r("G8"; 80)]),
        snapr("length-batch-scene-two"; [r("G8"; 80)]),
        snapr("length-batch-scene-two"; [r("G8"; 80)]),
        snapr("length-batch-scene-two"; [r("G8"; 80), r("G9"; 90)]),
        lis("length-batch-scene-two"; "create"; 2; [r("G8"; 80), r("G9"; 90), r("G10"; 100)]; [r("G3"; 30), r("G5"; 50), r("G6"; 60)]),
        snap0("length-batch-scene-two"),
        snap0("length-batch-scene-two"),
        snapr("length-batch-scene-two"; [r("G11"; 110)]),
        snapr("length-batch-scene-two"; [r("G11"; 110), r("G13"; 130)]),
        lis("length-batch-scene-two"; "create"; 3; [r("G11"; 110), r("G13"; 130), r("G14"; 140)]; [r("G8"; 80), r("G9"; 90), r("G10"; 100)]),
        snap0("length-batch-scene-two")
    ];
    # Ord 21 (InfraSortWindow, lines 2100-2155): the create listener of the
    # window statement and the istream-only s0 consumer fire per insert wave (create
    # first), the delete waves deliver old only to create and reach s0 not at
    # all, and the E5/E7/E9 inserts over capacity deliver one invocation
    # carrying the new row and the expelled maximum.  Ties keep the newest
    # first, so E7(16) precedes E6(16) and E9(1) precedes E8(1).
    def sw: [
        lisn("sort-window"; "create"; 1; [r("E1"; 10)]),
        lisn("sort-window"; "s0"; 1; [r("E1"; 10)]),
        lisn("sort-window"; "create"; 2; [r("E2"; 20)]),
        lisn("sort-window"; "s0"; 2; [r("E2"; 20)]),
        lisn("sort-window"; "create"; 3; [r("E3"; 15)]),
        lisn("sort-window"; "s0"; 3; [r("E3"; 15)]),
        snapr("sort-window"; [r("E1"; 10), r("E3"; 15), r("E2"; 20)]),
        liso("sort-window"; "create"; 4; [r("E2"; 20)]),
        snapr("sort-window"; [r("E1"; 10), r("E3"; 15)]),
        lisn("sort-window"; "create"; 5; [r("E4"; 18)]),
        lisn("sort-window"; "s0"; 4; [r("E4"; 18)]),
        snapr("sort-window"; [r("E1"; 10), r("E3"; 15), r("E4"; 18)]),
        lis("sort-window"; "create"; 6; [r("E5"; 17)]; [r("E4"; 18)]),
        lisn("sort-window"; "s0"; 5; [r("E5"; 17)]),
        snapr("sort-window"; [r("E1"; 10), r("E3"; 15), r("E5"; 17)]),
        liso("sort-window"; "create"; 7; [r("E1"; 10)]),
        snapr("sort-window"; [r("E3"; 15), r("E5"; 17)]),
        lisn("sort-window"; "create"; 8; [r("E6"; 16)]),
        lisn("sort-window"; "s0"; 6; [r("E6"; 16)]),
        snapr("sort-window"; [r("E3"; 15), r("E6"; 16), r("E5"; 17)]),
        lis("sort-window"; "create"; 9; [r("E7"; 16)]; [r("E5"; 17)]),
        lisn("sort-window"; "s0"; 7; [r("E7"; 16)]),
        snapr("sort-window"; [r("E3"; 15), r("E7"; 16), r("E6"; 16)]),
        liso("sort-window"; "create"; 10; [r("E7"; 16)]),
        snapr("sort-window"; [r("E3"; 15), r("E6"; 16)]),
        lisn("sort-window"; "create"; 11; [r("E8"; 1)]),
        lisn("sort-window"; "s0"; 8; [r("E8"; 1)]),
        snapr("sort-window"; [r("E8"; 1), r("E3"; 15), r("E6"; 16)]),
        lis("sort-window"; "create"; 12; [r("E9"; 1)]; [r("E6"; 16)]),
        lisn("sort-window"; "s0"; 9; [r("E9"; 1)])
    ];
    # Ord 22 (InfraSortWindowSceneTwo, lines 2165-2224): three modules, the
    # create listener only, the delete wave delivering old only, and the final
    # G6 insert into the full window expelling ITSELF - one invocation carrying
    # the same row as new and old (assertPropsIRPair at line 2221).
    def swst: [
        lisn("sort-window-scene-two"; "create"; 1; [r("G1"; 10)]),
        snapr("sort-window-scene-two"; [r("G1"; 10)]),
        lisn("sort-window-scene-two"; "create"; 2; [r("G2"; 9)]),
        snapr("sort-window-scene-two"; [r("G2"; 9), r("G1"; 10)]),
        liso("sort-window-scene-two"; "create"; 3; [r("G2"; 9)]),
        snapr("sort-window-scene-two"; [r("G1"; 10)]),
        lisn("sort-window-scene-two"; "create"; 4; [r("G3"; 3)]),
        snapr("sort-window-scene-two"; [r("G3"; 3), r("G1"; 10)]),
        lisn("sort-window-scene-two"; "create"; 5; [r("G4"; 4)]),
        snapr("sort-window-scene-two"; [r("G3"; 3), r("G4"; 4), r("G1"; 10)]),
        lis("sort-window-scene-two"; "create"; 6; [r("G5"; 5)]; [r("G1"; 10)]),
        snapr("sort-window-scene-two"; [r("G3"; 3), r("G4"; 4), r("G5"; 5)]),
        lis("sort-window-scene-two"; "create"; 7; [r("G6"; 6)]; [r("G6"; 6)])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-lengthbatch-sort-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 80) and
    # 11 + 26 + 30 + 13 records, 37 listener and 43 snapshot.
    ((crec("length-batch") | length) == 11) and
    ((crec("length-batch-scene-two") | length) == 26) and
    ((crec("sort-window") | length) == 30) and
    ((crec("sort-window-scene-two") | length) == 13) and
    ([.records[] | select(.operation == "listener")] | length == 37) and
    ([.records[] | select(.operation == "snapshot")] | length == 43) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("length-batch"; 11) + repv("length-batch-scene-two"; 26) + repv("sort-window"; 30) + repv("sort-window-scene-two"; 13))) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # The length-batch case alternates create, s0 per flush wave; scene-two
    # listens to create only; the sort-window case interleaves the old-only
    # create delete waves between the insert pairs and the sort-window-scene-two
    # case listens to create only.  37 listener records.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 3) + repv("create"; 3) + repa(["create", "s0"]; 3) + ["create"] + repa(["create", "s0"]; 2) + ["create"] + repa(["create", "s0"]; 2) + ["create"] + repa(["create", "s0"]; 2) + repv("create"; 7))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3] + [1, 2, 3] + [1, 1, 2, 2, 3, 3, 4, 5, 4, 6, 5, 7, 8, 6, 9, 7, 10, 11, 8, 12, 9] + [1, 2, 3, 4, 5, 6, 7])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    ([.records[] | select(.operation == "snapshot") | .statement] | all(. == "create")) and
    # the nine empty-iterator snapshots emit the record with the new key
    # omitted: two in the length-batch case, seven in scene-two.
    ([.records[] | select(.operation == "snapshot" and (has("new") | not))] | length == 9) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | .case] == (repv("length-batch"; 2) + repv("length-batch-scene-two"; 7))) and
    ([.records[] | select(.operation == "snapshot" and has("new")) | (.new | length) > 0] | all) and
    # every listener record carries at least one stream and every row projects
    # exactly the case fields: key/value for all four windows.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    (crec("length-batch") == lb) and
    (crec("length-batch-scene-two") == lbst) and
    (crec("sort-window") == sw) and
    (crec("sort-window-scene-two") == swst)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-lengthbatch-sort-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
