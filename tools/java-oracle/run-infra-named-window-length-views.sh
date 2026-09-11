#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-length-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-length-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews length-slice: the MySimpleKeyValueMap length and firstlength windows, the projection length window and the bean-backed length window over a SupportBean_A delete trigger, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-d931bfbbbaa3ea632c2b", "java-runtime-793f6ec7608e25556803", "java-runtime-b53dbf8f541e79537f32", "java-runtime-c656dd0eeae6618a4162"]) and
    (.javaNames == ["InfraLengthWindow", "InfraLengthWindowSceneTwo", "InfraLengthFirstWindow", "InfraLengthWindowSceneThree"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["length-window", "length-window-scene-two", "length-first-window", "length-window-scene-three"]) and
    ([.cases[].ordinal] == [11, 12, 13, 25]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "length(3) window over the key/value map schema: FIFO eviction delivers one invocation carrying the new row and the evicted row, with ordered snapshots around every delete and eviction",
        "legacy win:length(3) projection window over theString/intBoxed with three module deployments and only the window statement listened across ten delete and eviction waves",
        "firstlength(2) window over the key/value map schema: inserts into the full window are dropped without any callback, deletes free a slot, and the freed slot is filled at the tail by the next arrival",
        "bean-backed length(2) window with a SupportBean_A delete trigger and a wildcard irstream consumer projected to theString; the first two snapshots are empty"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  consumeEpl is empty for every case of this
    # slice; the ord-12 scene-two case has no consumer statement at all, so its
    # s0Epl is empty and its deploys carry no s0.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"])) and
    ([.cases[].iteratorSnapshots] == [4, 13, 4, 4]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "delete"], ["create", "insert", "s0", "delete"], ["create", "insert", "delete", "s0"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create"], ["create", "s0"], ["s0"]]) and
    # per-case step counts: length-window = 1 case + 4 deploys + 7 sends + 4
    # snapshots + 1 undeploy-all = 17; length-window-scene-two = 1 + 3 + 11 +
    # 13 + 1 = 29; length-first-window = 1 + 4 + 6 + 4 + 1 = 16;
    # length-window-scene-three = 1 + 4 + 7 + 4 + 1 = 17 (17+29+16+17 = 79).
    ([.steps[] | select(.case == "length-window")] | length == 17) and
    ([.steps[] | select(.case == "length-window-scene-two")] | length == 29) and
    ([.steps[] | select(.case == "length-first-window")] | length == 16) and
    ([.steps[] | select(.case == "length-window-scene-three")] | length == 17) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 15) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 24) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 5) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 25) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create")] | length == 21) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s0")] | length == 4) and
    # every snapshot of this slice is exact-order; the any-mode protocol branch
    # of the shared oracle never fires here.
    ([.steps[] | select(.op == "snapshot") | .mode] | all(. == "ordered")) and
    ([.steps[] | select(.op == "snapshot") | .statement] == ([range(21) | "create"] + [range(4) | "s0"])) and
    # byte-exact EPL pins, including the legacy MyWindow.win:length(3) spelling
    # with @public, the unnamed bean-backed ord-25 window and the capital
    # @Name('"'"'s0'"'"') consumer.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowLW#length(3) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.win:length(3) as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowLFW#firstlength(2) as MySimpleKeyValueMap", "@public create window ABCWin#length(2) as SupportBean"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowLW select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean", "insert into MyWindowLFW select theString as key, longBoxed as value from SupportBean", "insert into ABCWin select * from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select irstream key, value as value from MyWindowLW", "", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowLFW", "@Name('"'"'s0'"'"') select irstream * from ABCWin"]) and
    ([.cases[].consumeEpl] == ["", "", "", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindowLW where symbol = key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindowLFW where symbol = key", "on SupportBean_A delete from ABCWin where theString = id"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[2].s0Epl, .cases[3].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[2].deleteEpl, .cases[3].deleteEpl]) and
    # send payload shapes: theString plus exactly the one value field the case
    # window projects (ord 25 leaves intBoxed/longBoxed null and sends
    # theString only), the SupportBean_A trigger carries id, and the on-delete
    # market trigger carries symbol.
    ([.steps[] | select(.op == "send" and .case == "length-window" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "length-window-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "length-first-window" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "length-window-scene-three" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A") | (.payload | keys)] | all(. == ["id"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    (.steps | type == "array" and length == 79)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-length-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-length-views.XXXXXX")
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
    "$script_root/InfraNamedWindowLengthViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowLengthViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def t($s): {"kind": "row", "fields": {"theString": $s}};
    def snapr($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    # snap0 is the empty-iterator snapshot record: the new key is omitted, not
    # an empty array, exactly like the Go normalizer renders it.
    def snap0($c; $st): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z"};
    def lis($c; $st; $q; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new, "old": $old};
    def lisn($c; $st; $q; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    def liso($c; $st; $q; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "old": $old};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 11 (InfraLengthWindow, lines 1099-1141): the create window statement
    # is deployed before s0, so every wave delivers create first and then s0;
    # the E5 and E6 inserts evict one row each in the SAME invocation that
    # carries the new row (LengthWindowView.update appends then removeFirst()s
    # and posts both streams to the child in one update), so those waves are
    # assertPropsIRPair records rather than two callbacks.
    def lw: [
        lisn("length-window"; "create"; 1; [r("E1"; 1)]),
        lisn("length-window"; "s0"; 1; [r("E1"; 1)]),
        lisn("length-window"; "create"; 2; [r("E2"; 2)]),
        lisn("length-window"; "s0"; 2; [r("E2"; 2)]),
        lisn("length-window"; "create"; 3; [r("E3"; 3)]),
        lisn("length-window"; "s0"; 3; [r("E3"; 3)]),
        snapr("length-window"; "create"; [r("E1"; 1), r("E2"; 2), r("E3"; 3)]),
        liso("length-window"; "create"; 4; [r("E2"; 2)]),
        liso("length-window"; "s0"; 4; [r("E2"; 2)]),
        snapr("length-window"; "create"; [r("E1"; 1), r("E3"; 3)]),
        lisn("length-window"; "create"; 5; [r("E4"; 4)]),
        lisn("length-window"; "s0"; 5; [r("E4"; 4)]),
        snapr("length-window"; "create"; [r("E1"; 1), r("E3"; 3), r("E4"; 4)]),
        lis("length-window"; "create"; 6; [r("E5"; 5)]; [r("E1"; 1)]),
        lis("length-window"; "s0"; 6; [r("E5"; 5)]; [r("E1"; 1)]),
        snapr("length-window"; "create"; [r("E3"; 3), r("E4"; 4), r("E5"; 5)]),
        lis("length-window"; "create"; 7; [r("E6"; 6)]; [r("E3"; 3)]),
        lis("length-window"; "s0"; 7; [r("E6"; 6)]; [r("E3"; 3)])
    ];
    # Ord 12 (InfraLengthWindowSceneTwo, lines 1151-1255): three modules, the
    # create listener only (there is no consumer statement), eleven deliveries
    # across ten waves - the IR pairs are the G6 and G8 evictions - and the
    # thirteen ordered snapshots the source asserts, including the four pairs
    # re-asserted across a milestone.
    def ls: [
        lisn("length-window-scene-two"; "create"; 1; [r("G1"; 10)]),
        snapr("length-window-scene-two"; "create"; [r("G1"; 10)]),
        lisn("length-window-scene-two"; "create"; 2; [r("G2"; 20)]),
        snapr("length-window-scene-two"; "create"; [r("G1"; 10), r("G2"; 20)]),
        liso("length-window-scene-two"; "create"; 3; [r("G2"; 20)]),
        snapr("length-window-scene-two"; "create"; [r("G1"; 10)]),
        snapr("length-window-scene-two"; "create"; [r("G1"; 10)]),
        lisn("length-window-scene-two"; "create"; 4; [r("G3"; 30)]),
        snapr("length-window-scene-two"; "create"; [r("G1"; 10), r("G3"; 30)]),
        liso("length-window-scene-two"; "create"; 5; [r("G1"; 10)]),
        snapr("length-window-scene-two"; "create"; [r("G3"; 30)]),
        snapr("length-window-scene-two"; "create"; [r("G3"; 30)]),
        lisn("length-window-scene-two"; "create"; 6; [r("G4"; 40)]),
        snapr("length-window-scene-two"; "create"; [r("G3"; 30), r("G4"; 40)]),
        lisn("length-window-scene-two"; "create"; 7; [r("G5"; 50)]),
        snapr("length-window-scene-two"; "create"; [r("G3"; 30), r("G4"; 40), r("G5"; 50)]),
        lis("length-window-scene-two"; "create"; 8; [r("G6"; 60)]; [r("G3"; 30)]),
        snapr("length-window-scene-two"; "create"; [r("G4"; 40), r("G5"; 50), r("G6"; 60)]),
        liso("length-window-scene-two"; "create"; 9; [r("G6"; 60)]),
        snapr("length-window-scene-two"; "create"; [r("G4"; 40), r("G5"; 50)]),
        snapr("length-window-scene-two"; "create"; [r("G4"; 40), r("G5"; 50)]),
        lisn("length-window-scene-two"; "create"; 10; [r("G7"; 70)]),
        snapr("length-window-scene-two"; "create"; [r("G4"; 40), r("G5"; 50), r("G7"; 70)]),
        lis("length-window-scene-two"; "create"; 11; [r("G8"; 80)]; [r("G4"; 40)])
    ];
    # Ord 13 (InfraLengthFirstWindow, lines 1263-1297): the E3 and E5 inserts
    # arrive at the full firstlength(2) window and produce NOTHING anywhere
    # (FirstLengthWindowView updates its child only when something changed), the
    # E2 delete frees a slot that E4 fills at the tail, and no eviction ever
    # happens.
    def lf: [
        lisn("length-first-window"; "create"; 1; [r("E1"; 1)]),
        lisn("length-first-window"; "s0"; 1; [r("E1"; 1)]),
        lisn("length-first-window"; "create"; 2; [r("E2"; 2)]),
        lisn("length-first-window"; "s0"; 2; [r("E2"; 2)]),
        snapr("length-first-window"; "create"; [r("E1"; 1), r("E2"; 2)]),
        liso("length-first-window"; "create"; 3; [r("E2"; 2)]),
        liso("length-first-window"; "s0"; 3; [r("E2"; 2)]),
        snapr("length-first-window"; "create"; [r("E1"; 1)]),
        lisn("length-first-window"; "create"; 4; [r("E4"; 4)]),
        lisn("length-first-window"; "s0"; 4; [r("E4"; 4)]),
        snapr("length-first-window"; "create"; [r("E1"; 1), r("E4"; 4)]),
        snapr("length-first-window"; "create"; [r("E1"; 1), r("E4"; 4)])
    ];
    # Ord 25 (InfraLengthWindowSceneThree, lines 2463-2502): the s0 consumer is
    # the only listened statement, the project reads theString only, and the
    # first two snapshots are EMPTY iterators - their records carry no new key
    # (the suite asserts null then new Object[0][]).  The E5 insert evicts E2
    # in one IR-pair invocation.
    def l3: [
        snap0("length-window-scene-three"; "s0"),
        lisn("length-window-scene-three"; "s0"; 1; [t("E1")]),
        liso("length-window-scene-three"; "s0"; 2; [t("E1")]),
        snap0("length-window-scene-three"; "s0"),
        lisn("length-window-scene-three"; "s0"; 3; [t("E2")]),
        lisn("length-window-scene-three"; "s0"; 4; [t("E3")]),
        snapr("length-window-scene-three"; "s0"; [t("E2"), t("E3")]),
        liso("length-window-scene-three"; "s0"; 5; [t("E3")]),
        snapr("length-window-scene-three"; "s0"; [t("E2")]),
        lisn("length-window-scene-three"; "s0"; 6; [t("E4")]),
        lis("length-window-scene-three"; "s0"; 7; [t("E5")]; [t("E2")])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-length-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 65) and
    # 18 + 24 + 12 + 11 records, 40 listener and 25 snapshot.
    ((crec("length-window") | length) == 18) and
    ((crec("length-window-scene-two") | length) == 24) and
    ((crec("length-first-window") | length) == 12) and
    ((crec("length-window-scene-three") | length) == 11) and
    ([.records[] | select(.operation == "listener")] | length == 40) and
    ([.records[] | select(.operation == "snapshot")] | length == 25) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("length-window"; 18) + repv("length-window-scene-two"; 24) + repv("length-first-window"; 12) + repv("length-window-scene-three"; 11))) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # The length-window and firstlength blocks are create, s0 per wave (see the
    # lw() and lf() notes above); scene-two listens to create only and
    # scene-three to s0 only.  40 listener records.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 7) + repv("create"; 11) + repa(["create", "s0"]; 4) + repv("s0"; 7))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7] + [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11] + [1, 1, 2, 2, 3, 3, 4, 4] + [1, 2, 3, 4, 5, 6, 7])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == repv(0; 25)) and
    ([.records[] | select(.operation == "snapshot") | .statement] == (repv("create"; 21) + repv("s0"; 4))) and
    # the two empty ord-25 snapshots emit the record with the new key omitted.
    ([.records[] | select(.case == "length-window-scene-three" and .operation == "snapshot" and (has("new") | not))] | length == 2) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | .case] == ["length-window-scene-three", "length-window-scene-three"]) and
    ([.records[] | select(.operation == "snapshot" and has("new")) | (.new | length) > 0] | all) and
    # every listener record carries at least one stream and every row projects
    # exactly the case fields: theString only for the bean window, key/value
    # for the three map-schema and projection windows.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | select(.case == "length-window-scene-three") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["theString"]] | all) and
    ([.records[] | select(.case != "length-window-scene-three") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    (crec("length-window") == lw) and
    (crec("length-window-scene-two") == ls) and
    (crec("length-first-window") == lf) and
    (crec("length-window-scene-three") == l3)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-length-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
