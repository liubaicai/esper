#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-bean-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-bean-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews bean slice: the bean-backed window with its representation matrix and on-trigger update, the contained-bean window fed by a stream-wildcard insert, the schema alias read through a fire-and-forget query, and the deep-supertype insert whose window reads the most-derived override (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-f0a1da1fe931e132c21f", "java-runtime-fe6adc5803da60bf18f5", "java-runtime-f195548d023dfbde1aed", "java-runtime-baa0bd4ca41b9b2c5f94"]) and
    (.javaNames == ["InfraBeanBacked", "InfraBeanContained", "InfraBeanSchemaBacked", "InfraDeepSupertypeInsert"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["bean-backed", "bean-contained", "bean-schema-backed", "deep-supertype-insert"]) and
    ([.cases[].ordinal] == [2, 35, 37, 38]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "keepall window declared as SupportBean and replayed once per event-representation annotation: the annotation is ignored, so every sub-run delivers bean events of the window type named MyWindowBB through the create and s0 listeners and an on-trigger update that delivers the updated row as new and the pre-update row as old",
        "keepall window declared as (bean SupportBean_S0) with one sub-run per representation: the objectarray and map/default annotations shape the window underlying and the nested bean.p00 property is read through the stream-wildcard insert, while the avro variant must fail to compile",
        "keepall window declared over a schema alias of the SupportBean class: the schema type ABC is distinct from the configured SupportBean type, so the second bean send feeds only the window and the fire-and-forget query reads one row of the window type MyWindowBSB while the select over ABC stays uninvoked",
        "keepall window declared from as select * from SupportOverrideBase and fed by an insert from SupportOverrideOneA: the same underlying object is re-typed into the window and the val property reads the most-derived override"
    ]) and
    # the case key set and the step key sets are pinned exactly: the Go loader
    # rejects any other field set.  This chain is the first to need the
    # representation-variant create slots (ord 2 runs four and ord 35 three
    # sub-runs), the schema/update/faf slots and the negative-compile text.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "createEplAvro", "createEplMap", "createEplObjectArray", "deleteEpl", "deploys", "description", "executionName", "fafEpl", "insertEpl", "iteratorSnapshots", "listened", "negativeCompileEpl", "onSetEpl", "ordinal", "runtimeId", "s0Epl", "s2Epl", "s3Epl", "schemaEpl", "updateEpl", "varEpl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "epl", "fields", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "fields", "op", "statement"] or . == ["case", "op"] or . == ["case", "mode", "op", "statement"] or . == ["case", "fields", "mode", "op", "statement"])) and
    # iteratorSnapshots counts the snapshot steps: only ord 38 reads an iterator
    # (INV:370).
    ([.cases[].iteratorSnapshots] == [0, 0, 0, 1]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "update"], ["create", "insert"], ["schema", "create", "insert", "s0"], ["create", "insert"]]) and
    ([.cases[].listened] == [["create", "s0", "update"], ["create"], ["s0"], []]) and
    # per-case step counts: bean-backed = 1 case + 16 deploys (four statements
    # per sub-run over four sub-runs) + 8 sends + 4 undeploy-all = 29;
    # bean-contained = 1 + 6 + 3 + 3 = 13; bean-schema-backed =
    # 1 + 4 + 2 sends + 1 faf + 1 undeploy-all = 9; deep-supertype-insert =
    # 1 + 2 + 1 send + 1 snapshot + 1 undeploy-all = 6 (29+13+9+6 = 57).
    ([.steps[] | select(.case == "bean-backed")] | length == 29) and
    ([.steps[] | select(.case == "bean-contained")] | length == 13) and
    ([.steps[] | select(.case == "bean-schema-backed")] | length == 9) and
    ([.steps[] | select(.case == "deep-supertype-insert")] | length == 6) and
    (.steps | type == "array" and length == 57) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 28) and
    ([.steps[] | select(.op == "deploy" and .case == "bean-backed")] | length == 16) and
    ([.steps[] | select(.op == "deploy" and .case == "bean-contained")] | length == 6) and
    ([.steps[] | select(.op == "send")] | length == 14) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 6) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportOverrideOneA")] | length == 1) and
    ([.steps[] | select(.op == "faf")] | length == 1) and
    ([.steps[] | select(.op == "snapshot")] | length == 1) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 9) and
    ([.steps[] | select(.op == "advance-time")] | length == 0) and
    # the single snapshot is ord 38'"'"'s ordered window state projecting val.
    ([.steps[] | select(.op == "snapshot") | [.case, .statement, .mode, (.fields | join(","))]] == [["deep-supertype-insert", "create", "ordered", "val"]]) and
    # the faf step carries the ord-37 query text and an empty projection (the
    # suite reads only the returned row'"'"'s event type).
    ([.steps[] | select(.op == "faf") | [.case, .statement, .epl, (.fields | length)]] == [["bean-schema-backed", "faf", "select * from MyWindowBSB", 0]]) and
    # byte-exact EPL pins: the four representated ord-2 create texts plus its
    # insert/s0/update, the three ord-35 create texts plus its insert and the
    # negative-compile text, the ord-37 schema/window/insert/select/query texts
    # and the ord-38 window/insert texts.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') @public create window MyWindowBB#keepall as SupportBean", "@name('"'"'create'"'"') @public create window MyWindowBC#keepall as (bean SupportBean_S0)", "@public create window MyWindowBSB#keepall as ABC", "@name('"'"'create'"'"') create window MyWindowDSI#keepall as select * from SupportOverrideBase"]) and
    ([.cases[].createEplObjectArray] == ["@EventRepresentation('"'"'objectarray'"'"') @name('"'"'create'"'"') @public create window MyWindowBB#keepall as SupportBean", "@EventRepresentation('"'"'objectarray'"'"') @name('"'"'create'"'"') @public create window MyWindowBC#keepall as (bean SupportBean_S0)", "", ""]) and
    ([.cases[].createEplMap] == ["@EventRepresentation('"'"'map'"'"') @name('"'"'create'"'"') @public create window MyWindowBB#keepall as SupportBean", "@EventRepresentation('"'"'map'"'"') @name('"'"'create'"'"') @public create window MyWindowBC#keepall as (bean SupportBean_S0)", "", ""]) and
    ([.cases[].createEplAvro] == ["@EventRepresentation('"'"'avro'"'"') @name('"'"'create'"'"') @public create window MyWindowBB#keepall as SupportBean", "", "", ""]) and
    ([.cases[].insertEpl] == ["@public insert into MyWindowBB select * from SupportBean", "insert into MyWindowBC select bean.* as bean from SupportBean_S0 as bean", "insert into MyWindowBSB select * from SupportBean", "insert into MyWindowDSI select * from SupportOverrideOneA"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select * from MyWindowBB", "", "@name('"'"'s0'"'"') select * from ABC", ""]) and
    ([.cases[].updateEpl] == ["@name('"'"'update'"'"') on SupportBean_A update MyWindowBB set theString='"'"'s'"'"'", "", "", ""]) and
    ([.cases[].schemaEpl] == ["", "", "@public create schema ABC as com.espertech.esper.common.internal.support.SupportBean", ""]) and
    ([.cases[].fafEpl] == ["", "", "select * from MyWindowBSB", ""]) and
    ([.cases[].negativeCompileEpl] == ["", "@EventRepresentation('"'"'avro'"'"') @name('"'"'create'"'"') create window MyWindowBC#keepall as (bean SupportBean_S0)", "", ""]) and
    ([.cases[].consumeEpl] == ["", "", "", ""]) and
    ([.cases[].deleteEpl] == ["", "", "", ""]) and
    ([.cases[].varEpl] == ["", "", "", ""]) and
    ([.cases[].onSetEpl] == ["", "", "", ""]) and
    ([.cases[].s2Epl] == ["", "", "", ""]) and
    ([.cases[].s3Epl] == ["", "", "", ""]) and
    # deploy-step EPLs equal the matching case slots, except the ord-2 create
    # steps which cycle objectarray, map, default and avro in that order.
    ([.steps[] | select(.op == "deploy" and .case == "bean-backed" and .statement == "create") | .epl] == [.cases[0].createEplObjectArray, .cases[0].createEplMap, .cases[0].createEpl, .cases[0].createEplAvro]) and
    ([.steps[] | select(.op == "deploy" and .case == "bean-contained" and .statement == "create") | .epl] == [.cases[1].createEplObjectArray, .cases[1].createEplMap, .cases[1].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[0].insertEpl, .cases[0].insertEpl, .cases[0].insertEpl, .cases[1].insertEpl, .cases[1].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[0].s0Epl, .cases[0].s0Epl, .cases[0].s0Epl, .cases[2].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "update") | .epl] == [.cases[0].updateEpl, .cases[0].updateEpl, .cases[0].updateEpl, .cases[0].updateEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "schema") | .epl] == [.cases[2].schemaEpl]) and
    # send payload shapes: the no-argument SupportBean sends carry an EMPTY
    # payload (every property keeps its Java default, theString included), the
    # trigger carries id, the contained fill carries id plus p00 and the
    # subtype send carries the override chain so the window can read the
    # most-derived value.
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean") | (.payload | keys)] | all(. == [])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A") | (.payload | keys)] | all(. == ["id"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | (.payload | keys | sort)] | all(. == ["id", "p00"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportOverrideOneA") | (.payload | keys | sort)] | all(. == ["val", "valOne", "valOneA"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportOverrideOneA") | [.payload.valOneA, .payload.valOne, .payload.val]] == [["1a", "1", "base"]]) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | [.payload.id, .payload.p00]] | all(. == [1, "E1"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A") | .payload.id] | all(. == "A1")) and
    # placement pins: every case opens with its module(s) in source order and
    # ends with its teardown; ord 2'"'"'s update module sits between the fill and
    # the trigger in every sub-run; ord 37'"'"'s query runs after the first send and
    # before the late consumer.
    ([.steps[] | select(.case == "bean-backed")] | .[0] == {"op": "case", "case": "bean-backed"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].op == "send" and .[5].statement == "update" and .[6].eventType == "SupportBean_A" and .[7].op == "undeploy-all" and .[28].op == "undeploy-all") and
    ([.steps[] | select(.case == "bean-contained")] | .[0] == {"op": "case", "case": "bean-contained"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].eventType == "SupportBean_S0" and .[4].op == "undeploy-all" and .[12].op == "undeploy-all") and
    ([.steps[] | select(.case == "bean-schema-backed")] | .[0] == {"op": "case", "case": "bean-schema-backed"} and .[1].statement == "schema" and .[2].statement == "create" and .[3].statement == "insert" and .[4].op == "send" and .[5].op == "faf" and .[6].op == "deploy" and .[6].statement == "s0" and .[7].op == "send" and .[8].op == "undeploy-all") and
    ([.steps[] | select(.case == "deep-supertype-insert")] | .[0] == {"op": "case", "case": "deep-supertype-insert"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].eventType == "SupportOverrideOneA" and .[4].op == "snapshot" and .[5].op == "undeploy-all")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-bean-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-bean-views.XXXXXX")
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
# The suite enables Avro in its harness configuration, so ord 35's compile
# rejection needs the Avro provider and its Jackson dependencies.  The
# regression-run module's classpath is the harness's own provider set; the
# pinned module target/classes above take precedence over any repo jars it
# also carries, and the oracle's SupportOverride* classes are its own.
"$mvn_bin" -q -f "$esper_root/regression-run/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/harness-cp.txt" -Dmdep.includeScope=runtime \
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
harness_cp=$(tr -d '\r\n' < "$work/harness-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp$cp_sep$harness_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNamedWindowBeanViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowBeanViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    # Ord 2 asserts only the delivered event type (assertEvent), so its rows
    # carry the type name and project no field; ord 35 and ord 38 assert one
    # value each and project exactly that field; ord 37 reads the
    # fire-and-forget row type only.
    def br: {"kind": "row", "type": "MyWindowBB", "fields": {}};
    def bc($v): {"kind": "row", "fields": {"bean.p00": $v}};
    def bsb: {"kind": "row", "type": "MyWindowBSB", "fields": {}};
    def dsi($v): {"kind": "row", "fields": {"val": $v}};
    def lis($c; $st; $q; $t; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new};
    def lis2($c; $st; $q; $t; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new, "old": $old};
    def fafr($c; $st; $t; $rows): {"case": $c, "operation": "faf", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    def snapr($c; $st; $t; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 2 (InfraBeanBacked, helper at 3591-3611 x four representations): each
    # cycle is the insert pair (create then s0), then the on-trigger update
    # wave, which delivers the updated copy as new AND the pre-update row as old
    # to the window'"'"'s own listener, then to the update statement'"'"'s listener and
    # then to the s0 consumer with new only.  Sequence counters continue across
    # the four cycles.  All rows are bean events of the window type MyWindowBB.
    def bb: [
        lis("bean-backed"; "create"; 1; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "s0"; 1; "1970-01-01T00:00:00Z"; [br]),
        lis2("bean-backed"; "create"; 2; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis2("bean-backed"; "update"; 1; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis("bean-backed"; "s0"; 2; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "create"; 3; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "s0"; 3; "1970-01-01T00:00:00Z"; [br]),
        lis2("bean-backed"; "create"; 4; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis2("bean-backed"; "update"; 2; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis("bean-backed"; "s0"; 4; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "create"; 5; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "s0"; 5; "1970-01-01T00:00:00Z"; [br]),
        lis2("bean-backed"; "create"; 6; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis2("bean-backed"; "update"; 3; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis("bean-backed"; "s0"; 6; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "create"; 7; "1970-01-01T00:00:00Z"; [br]),
        lis("bean-backed"; "s0"; 7; "1970-01-01T00:00:00Z"; [br]),
        lis2("bean-backed"; "create"; 8; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis2("bean-backed"; "update"; 4; "1970-01-01T00:00:00Z"; [br]; [br]),
        lis("bean-backed"; "s0"; 8; "1970-01-01T00:00:00Z"; [br])
    ];
    # Ord 35 (InfraBeanContained, helper at 3498-3509 x three representations):
    # one create callback per sub-run reading the nested POJO property; the
    # window type is representation-shaped, so no type tag is pinned.
    def bcv: [
        lis("bean-contained"; "create"; 1; "1970-01-01T00:00:00Z"; [bc("E1")]),
        lis("bean-contained"; "create"; 2; "1970-01-01T00:00:00Z"; [bc("E1")]),
        lis("bean-contained"; "create"; 3; "1970-01-01T00:00:00Z"; [bc("E1")])
    ];
    # Ord 37 (InfraBeanSchemaBacked, lines 347-360): the only record is the
    # fire-and-forget row of the window type MyWindowBSB; the select over the
    # schema type ABC is never invoked by the second bean send.
    def bsbv: [
        fafr("bean-schema-backed"; "faf"; "1970-01-01T00:00:00Z"; [bsb])
    ];
    # Ord 38 (InfraDeepSupertypeInsert, lines 366-371): the one window iterator
    # state reads the most-derived override of val.
    def dsiv: [
        snapr("deep-supertype-insert"; "create"; "1970-01-01T00:00:00Z"; [dsi("1a")])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-bean-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 25) and
    # 20 + 3 + 1 + 1 records; 24 listener, 1 faf and 1 snapshot.
    ((crec("bean-backed") | length) == 20) and
    ((crec("bean-contained") | length) == 3) and
    ((crec("bean-schema-backed") | length) == 1) and
    ((crec("deep-supertype-insert") | length) == 1) and
    ([.records[] | select(.operation == "listener")] | length == 23) and
    ([.records[] | select(.operation == "faf")] | length == 1) and
    ([.records[] | select(.operation == "snapshot")] | length == 1) and
    ([.records[].operation] | all(. == "listener" or . == "faf" or . == "snapshot")) and
    ([.records[].case] == (repv("bean-backed"; 20) + repv("bean-contained"; 3) + repv("bean-schema-backed"; 1) + repv("deep-supertype-insert"; 1))) and
    # none of the four executions moves the clock.
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # sequences: ord 2 restarts nothing across its four cycles (create and s0
    # reach 8, update 4), ord 35 counts per sub-run (1..3) and ord 38'"'"'s snapshot
    # carries 0.
    ([.records[] | select(.operation == "listener") | [.case, .statement, .sequence]] == ([["bean-backed", "create", 1], ["bean-backed", "s0", 1], ["bean-backed", "create", 2], ["bean-backed", "update", 1], ["bean-backed", "s0", 2], ["bean-backed", "create", 3], ["bean-backed", "s0", 3], ["bean-backed", "create", 4], ["bean-backed", "update", 2], ["bean-backed", "s0", 4], ["bean-backed", "create", 5], ["bean-backed", "s0", 5], ["bean-backed", "create", 6], ["bean-backed", "update", 3], ["bean-backed", "s0", 6], ["bean-backed", "create", 7], ["bean-backed", "s0", 7], ["bean-backed", "create", 8], ["bean-backed", "update", 4], ["bean-backed", "s0", 8]] + [["bean-contained", "create", 1], ["bean-contained", "create", 2], ["bean-contained", "create", 3]])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    ([.records[] | select(.operation == "faf") | [.statement, .sequence]] | all(. == ["faf", 0])) and
    # ord 2 is the only case whose records carry the new-plus-old shape (the
    # update waves) and the only one with a type tag.
    ([.records[] | select(has("old"))] | length == 8) and
    ([.records[] | select(has("old")) | [.statement, .sequence]] == [["create", 2], ["update", 1], ["create", 4], ["update", 2], ["create", 6], ["update", 3], ["create", 8], ["update", 4]]) and
    ([.records[] | select(.case == "bean-backed") | ((.new // []) + (.old // []))[] | [.kind, .type, (.fields | length)]] | all(. == ["row", "MyWindowBB", 0])) and
    ([.records[] | select(.case == "bean-schema-backed") | .new[] | [.kind, .type, (.fields | length)]] | all(. == ["row", "MyWindowBSB", 0])) and
    ([.records[] | select(.case == "bean-contained") | .new[] | .fields] | all(. == {"bean.p00": "E1"})) and
    ([.records[] | select(.case == "deep-supertype-insert") | .new[] | .fields] | all(. == {"val": "1a"})) and
    ([.records[] | select(.case == "bean-contained") | (.new[] | has("type"))] | all(. == false)) and
    ([.records[] | select(.case == "deep-supertype-insert") | (.new[] | has("type"))] | all(. == false)) and
    (crec("bean-backed") == bb) and
    (crec("bean-contained") == bcv) and
    (crec("bean-schema-backed") == bsbv) and
    (crec("deep-supertype-insert") == dsiv)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-bean-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
