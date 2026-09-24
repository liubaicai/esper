#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-event-type.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "failed to resolve Esper commit in $esper_root" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is at $actual_commit, expected $expected_commit" >&2
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
    echo "Java 17 is required; resolved java reports version $java_version" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-event-type" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableEventType.java" and
    (.description | test("InfraNWTableEventType")) and
    (.javaRuntimes == ["java-runtime-d3c3a24f11288969df7e", "java-runtime-8dc1233d90336dcdb929", "java-runtime-14412293edec92196bd9"]) and
    (.javaNames == ["InfraNWTableEventTypeInvalid", "InfraNWTableEventTypeDefineFields", "InfraNWTableEventTypeInsertIntoProtected"]) and
    (.javaStaticIds == ["java-08b4a7da65c76bf5abdc", "java-50c0d9ea894da18ddb54", "java-c007ca393634b73ec8b5"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["invalid", "define-fields-window", "define-fields-table", "insert-into-protected"]) and
    ([.cases[].ordinal] == [0, 1, 1, 2]) and
    ([.cases[].runtimeId] == ["java-runtime-d3c3a24f11288969df7e", "java-runtime-8dc1233d90336dcdb929", "java-runtime-8dc1233d90336dcdb929", "java-runtime-14412293edec92196bd9"]) and
    ([.cases[].executionName] == ["InfraNWTableEventTypeInvalid", "InfraNWTableEventTypeDefineFields", "InfraNWTableEventTypeDefineFields", "InfraNWTableEventTypeInsertIntoProtected"]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "epl", "expectError", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "op", "statement"] or . == ["case", "op"])) and
    # per-case step counts: invalid = 1 case + 2 build-error = 3;
    # define-fields-window/-table = 1 + 1 deploy + 1 deployed + 1 types +
    # 1 undeploy-all = 5 each; insert-into-protected = 1 + 3 deploys +
    # 3 deployed + 2 sends + 1 snapshot + 1 undeploy-all = 11
    # (3 + 5 + 5 + 11 = 24 total).
    ([.steps[] | select(.case == "invalid")] | length == 3) and
    ([.steps[] | select(.case == "define-fields-window")] | length == 5) and
    ([.steps[] | select(.case == "define-fields-table")] | length == 5) and
    ([.steps[] | select(.case == "insert-into-protected")] | length == 11) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 5) and
    ([.steps[] | select(.op == "deployed")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "event")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "window")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 1) and
    ([.steps[] | select(.op == "build-error")] | length == 2) and
    ([.steps[] | select(.op == "types")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 1) and
    ([.steps[] | select(.op == "send")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "Fubar")] | length == 2) and
    # byte-exact EPL pins, including the @name/@public/@protected/@private
    # annotations and the doubled @public literal the Java source writes.
    ([.steps[] | select(.op == "build-error" and .case == "invalid") | .epl] == ["create schema SchemaOne as (p0 string);\ncreate window SchemaOne#keepall as SchemaOne;\n", "create schema SchemaTwo as (p0 string);\ncreate table SchemaTwo(c0 int);\n"]) and
    ([.steps[] | select(.op == "build-error" and .case == "invalid") | .expectError] == ["Error starting statement: An event type or schema by name '"'"'SchemaOne'"'"' already exists", "An event type by name '"'"'SchemaTwo'"'"' has already been declared"]) and
    ([.steps[] | select(.op == "deploy" and .case == "define-fields-window") | .epl] == ["@name('"'"'s0'"'"') @public create window MyInfra#keepall as (c0 int[], c1 int[primitive])"]) and
    ([.steps[] | select(.op == "deploy" and .case == "define-fields-table") | .epl] == ["@name('"'"'s0'"'"') @public create table MyInfra (c0 int[], c1 int[primitive])"]) and
    ([.steps[] | select(.op == "deploy" and .case == "insert-into-protected") | .epl] == ["@name('"'"'event'"'"') @public @buseventtype @public create map schema Fubar as (foo string, bar double)", "@name('"'"'window'"'"') @protected create window Snafu#keepall as Fubar", "@name('"'"'insert'"'"') @private insert into Snafu select * from Fubar"]) and
    # send payload shape: foo + bar for the Fubar map events.
    ([.steps[] | select(.op == "send" and .eventType == "Fubar") | (.payload | keys | sort)] | all(. == ["bar", "foo"])) and
    ([.steps[] | select(.op == "send") | .payload.foo] == ["a", "b"]) and
    (.steps | type == "array" and length == 24)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-event-type replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-event-type.XXXXXX")
cleanup() {
    rm -rf "$work"
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
    CYGWIN*|MINGW*|MSYS*)
        cp_sep=';'
        native_path() { cygpath -w "$1"; }
        ;;
    *)
        cp_sep=':'
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableEventTypeScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableEventTypeScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def dep($c; $st; $q): {"case": $c, "operation": "deployed", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z"};
    def cerr($c; $st; $v): {"case": $c, "operation": "compile-error", "statement": $st, "sequence": 0, "value": $v};
    def typ($c; $st): {"case": $c, "operation": "types", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "value": [{"name": "c0", "type": "Integer[]"}, {"name": "c1", "type": "int[]"}]};
    def snap($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def fubar($f; $b): {"kind": "row", "fields": {"bar": $b, "foo": $f}};
    def crec($c): [.records[] | select(.case == $c)];
    # invalid: the two compile-error records carrying the pinned Java
    # message prefixes for the window and table name collisions.
    def invalid: [
        cerr("invalid"; "window-name-collision"; "Error starting statement: An event type or schema by name '"'"'SchemaOne'"'"' already exists"),
        cerr("invalid"; "table-name-collision"; "An event type by name '"'"'SchemaTwo'"'"' has already been declared")
    ];
    # define-fields: the s0 deployed marker and the positional types
    # record (c0=Integer[] boxed, c1=int[] primitive) for each variant.
    def window: [
        dep("define-fields-window"; "s0"; 1),
        typ("define-fields-window"; "s0")
    ];
    def table: [
        dep("define-fields-table"; "s0"; 1),
        typ("define-fields-table"; "s0")
    ];
    # insert-into-protected: three deployed markers and the ordered
    # keepall-window snapshot rows {foo:a,bar:1},{foo:b,bar:2}.
    def protected: [
        dep("insert-into-protected"; "event"; 1),
        dep("insert-into-protected"; "window"; 1),
        dep("insert-into-protected"; "insert"; 1),
        snap("insert-into-protected"; "window"; [fubar("a"; 1), fubar("b"; 2)])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-event-type" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 10) and
    ((crec("invalid") | length) == 2) and
    ((crec("define-fields-window") | length) == 2) and
    ((crec("define-fields-table") | length) == 2) and
    ((crec("insert-into-protected") | length) == 4) and
    ([.records[].case] == (["invalid", "invalid"]
        + ["define-fields-window", "define-fields-window"]
        + ["define-fields-table", "define-fields-table"]
        + ["insert-into-protected", "insert-into-protected", "insert-into-protected", "insert-into-protected"])) and
    ([.records[] | select(.operation == "snapshot") | has("new")] | all) and
    (crec("invalid") == invalid) and
    (crec("define-fields-window") == window) and
    (crec("define-fields-table") == table) and
    (crec("insert-into-protected") == protected)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-event-type trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
