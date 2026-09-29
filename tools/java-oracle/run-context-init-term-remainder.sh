#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-context-init-term-remainder.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java 9.0.0 oracle commit. Java 17
and Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
The db-historical case requires the esper-mysql fixture listening on
127.0.0.1:3306 with mytesttable loaded.
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
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || { echo "cannot read HEAD" >&2; exit 1; }
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
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "context-init-term-remainder" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java" and
    (.description | test("ContextStartEndDBHistorical")) and
    (.description | test("ContextInitTermWithDistinctInvalid")) and
    (.description | test("ContextInitTermWNowInvalid")) and
    (.description | test("ContextHashInvalid")) and
    (.javaSourceFiles == ["regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermTemporalFixed.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithDistinct.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermWithNow.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextHashSegmented.java"]) and
    (.javaRuntimes == ["java-runtime-bc152186877c0a641b3d", "java-runtime-19cc6b63d1614c49dfbf", "java-runtime-6b3caa8f5e3490b05507", "java-runtime-25a58a30d6cbd02f00e7"]) and
    (.javaNames == ["ContextStartEndDBHistorical", "ContextInitTermWithDistinctInvalid", "ContextInitTermWNowInvalid", "ContextHashInvalid"]) and
    (.javaStaticIds == ["java-06954b45a1979f495425", "java-1db75f8dcee67079871d", "java-21fe1b2ee6da1a4f412c", "java-0564864de64ece6e7772"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["db-historical", "distinct-invalid", "now-invalid", "hash-invalid"]) and
    ([.cases[].ordinal] == [18, 0, 2, 8]) and
    ([.cases[].runtimeId] == ["java-runtime-bc152186877c0a641b3d", "java-runtime-19cc6b63d1614c49dfbf", "java-runtime-6b3caa8f5e3490b05507", "java-runtime-25a58a30d6cbd02f00e7"]) and
    ([.cases[].executionName] == ["ContextStartEndDBHistorical", "ContextInitTermWithDistinctInvalid", "ContextInitTermWNowInvalid", "ContextHashInvalid"]) and
    # case metadata epl fields pinned byte-exact: the db-historical case
    # lists the s0 join statement; each invalid case lists its first probe.
    ([.cases[].epl] == ["@name('"'"'s0'"'"') context NineToFive select * from SupportBean_S0 as s0, sql:MyDB ['"'"'select * from mytesttable where ${id} = mytesttable.mybigint'"'"'] as s1", "create context MyContext initiated by distinct(theString) SupportBean terminated after 15 seconds", "create context TimedImmediate initiated @now terminated after 10 seconds", "create context ACtx coalesce hash_code(intPrimitive) from SupportBean(dummy = 1) granularity 10"]) and
    # the case key set and the step key sets are pinned exactly: the Go
    # loader rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "op"] or . == ["at", "case", "op"] or . == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "epl", "expectError", "op", "statement"] or . == ["case", "compileWithoutPath", "epl", "expectError", "op", "statement"])) and
    # per-case step counts: db-historical = 1 case + 4 advance-time +
    # 2 deploys + 4 sends + 1 undeploy-all = 12; distinct-invalid =
    # 1 + 5 build-errors + 1 undeploy-all = 7; now-invalid = 1 + 3 + 1 = 5;
    # hash-invalid = 1 + 6 build-errors + 2 deploys + 1 undeploy-all = 10
    # (12 + 7 + 5 + 10 = 34 total).
    ([.steps[] | select(.case == "db-historical")] | length == 12) and
    ([.steps[] | select(.case == "distinct-invalid")] | length == 7) and
    ([.steps[] | select(.case == "now-invalid")] | length == 5) and
    ([.steps[] | select(.case == "hash-invalid")] | length == 10) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "advance-time")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 4) and
    ([.steps[] | select(.op == "send")] | length == 4) and
    ([.steps[] | select(.op == "build-error")] | length == 14) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    # compileWithoutPath marks exactly the twelve path-less probes; the two
    # hash probes that run against the accumulated module path omit it.
    ([.steps[] | select(.op == "build-error" and (.compileWithoutPath == true))] | length == 12) and
    ([.steps[] | select(.op == "build-error" and (has("compileWithoutPath") | not)) | .statement] == ["statement-stream-type", "partition-named-window"]) and
    # advance-time pins: the NineToFive deploy-time clock plus the three
    # window boundary advances of ContextStartEndDBHistorical.
    ([.steps[] | select(.op == "advance-time") | .at] == ["2002-05-01T08:00:00.000Z", "2002-05-01T09:00:00.000Z", "2002-05-01T17:00:00.000Z", "2002-05-02T09:00:00.000Z"]) and
    # byte-exact deploy EPLs: db-historical deploys the NineToFive context
    # and the s0 sql:MyDB join; hash-invalid deploys the ACtx context and
    # the MyWindow fixture.
    ([.steps[] | select(.op == "deploy" and .case == "db-historical") | .epl] == ["@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)", "@name('"'"'s0'"'"') context NineToFive select * from SupportBean_S0 as s0, sql:MyDB ['"'"'select * from mytesttable where ${id} = mytesttable.mybigint'"'"'] as s1"]) and
    ([.steps[] | select(.op == "deploy" and .case == "hash-invalid") | .statement] == ["ctx", "window"]) and
    ([.steps[] | select(.op == "deploy" and .case == "hash-invalid") | .epl] == ["@public create context ACtx coalesce hash_code(intPrimitive) from SupportBean granularity 10", "@public create window MyWindow#keepall as SupportBean"]) and
    # probe order and epl/expectError pins per case.
    ([.steps[] | select(.op == "build-error" and .case == "distinct-invalid") | .statement] == ["distinct-no-as", "distinct-pattern", "distinct-subselect", "distinct-empty", "start-distinct"]) and
    ([.steps[] | select(.op == "build-error" and .case == "distinct-invalid") | .expectError] == ["Distinct-expressions require that a stream name is assigned to the stream using '"'"'as'"'"'", "Distinct-expressions require a stream as the initiated-by condition", "Invalid context distinct-clause expression '"'"'subselect_0'"'"': Aggregation, sub-select, previous or prior functions are not supported in this context", "Distinct-expressions have not been provided", "Incorrect syntax near '"'"'distinct'"'"' (a reserved keyword)"]) and
    ([.steps[] | select(.op == "build-error" and .case == "now-invalid") | .statement] == ["now-alone-terminated", "now-and-nonoverlapping", "now-with-filter"]) and
    ([.steps[] | select(.op == "build-error" and .case == "now-invalid") | .expectError] == ["Incorrect syntax near '"'"'terminated'"'"' (a reserved keyword) expecting '"'"'and'"'"'", "Incorrect syntax near '"'"'and'"'"' (a reserved keyword)", "Invalid use of '"'"'now'"'"' with initiated-by stream"]) and
    ([.steps[] | select(.op == "build-error" and .case == "hash-invalid") | .statement] == ["hash-dummy-filter", "hash-bad-func", "hash-bare-prop", "hash-no-params", "statement-stream-type", "partition-named-window"]) and
    ([.steps[] | select(.op == "build-error" and .case == "hash-invalid") | .expectError] == ["Failed to validate filter expression '"'"'dummy=1'"'"': Property named '"'"'dummy'"'"' is not valid in any stream", "expected a hash function that is any of {consistent_hash_crc32, hash_code}", "expected a hash function that is any of {consistent_hash_crc32, hash_code}", "expected one or more parameters to the hash function", "requires that any of the event types that are listed in the segmented context also appear in any of the filter expressions of the statement, type '"'"'SupportBean_S0'"'"' is not one of the types listed", "Partition criteria may not include named windows"]) and
    # send payloads pinned value-for-value: every send is a SupportBean_S0
    # carrying id plus p00 (the bean constructor second field, null in
    # this execution; ids are 2, 2, 2, 3 in step order).
    ([.steps[] | select(.op == "send") | .eventType] | all(. == "SupportBean_S0")) and
    ([.steps[] | select(.op == "send") | .payload] | all(keys | sort == ["id", "p00"])) and
    ([.steps[] | select(.op == "send") | .payload] | all(.p00 == null)) and
    ([.steps[] | select(.op == "send") | .payload.id] == [2, 2, 2, 3]) and
    (.steps | type == "array" and length == 34)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid context-init-term-remainder replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    if ! "$mvn_bin" -q -f "$esper_root/pom.xml" -DskipTests -Dgpg.skip=true \
        -pl common,compiler,runtime -am install >/dev/null; then
        echo "Esper build failed" >&2
        exit 1
    fi
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-context-init-term-remainder.XXXXXX")
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

# The MySQL Connector/J driver is test-scoped in the Esper modules and is not
# part of the runtime classpath above; resolve it from the local Maven
# repository using the version pinned by the Esper parent pom.
mysql_version=$(awk -F'[<>]' '/<mysql-connector-java.version>/{print $3; exit}' "$esper_root/pom.xml")
[ -n "$mysql_version" ] || { echo "cannot read mysql-connector-java.version from $esper_root/pom.xml" >&2; exit 1; }
mysql_jar=${MYSQL_CONNECTOR_JAR:-$HOME/.m2/repository/com/mysql/mysql-connector-j/$mysql_version/mysql-connector-j-$mysql_version.jar}
[ -f "$mysql_jar" ] || { echo "MySQL Connector/J was not found in the local Maven repository: $mysql_jar (build the Esper modules once, or point MYSQL_CONNECTOR_JAR at the driver jar)" >&2; exit 1; }

classes="$work/classes"
mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"
classpath="$classpath:$mysql_jar"
# The Avro and Jackson jars are optional/provided-scope dependencies that
# the runtime classpath omits; resolve them from the local Maven
# repository like run-context-lifecycle.sh.
avro_jar=$(find "$HOME/.m2/repository/org/apache/avro/avro" -name 'avro-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
jackson_databind=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-databind" -name 'jackson-databind-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
jackson_core=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-core" -name 'jackson-core-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
jackson_ann=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-annotations" -name 'jackson-annotations-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
for jar in "$avro_jar" "$jackson_databind" "$jackson_core" "$jackson_ann"; do
    [ -n "$jar" ] && classpath="$classpath:$jar"
done

# Validate the esper-mysql fixture early with the same connection path the
# oracle uses (the suite's EPLDatabaseMySQLDatabaseConnection probe: driver
# load, DriverManager connection over the SupportDatabaseService URL, one
# SELECT over mytesttable). The script never starts Docker itself; when the
# fixture is missing it fails with the operator-facing instructions below.
cat > "$work/DbProbe.java" <<'EOF'
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.ResultSet;
import java.sql.Statement;

public class DbProbe {
    public static void main(String[] args) throws Exception {
        try {
            Class.forName("com.mysql.cj.jdbc.Driver").getDeclaredConstructor().newInstance();
            Connection conn = DriverManager.getConnection(
                    "jdbc:mysql://localhost/test?user=root&password=password&useSSL=false");
            Statement stmt = conn.createStatement();
            ResultSet rs = stmt.executeQuery("select count(*) from mytesttable");
            rs.next();
            int rows = rs.getInt(1);
            rs.close();
            stmt.close();
            conn.close();
            if (rows != 10) {
                System.err.println("esper-mysql fixture has " + rows + " mytesttable rows, expected 10; reload common/etc/regression/create_testdb.sql with the '//' comment lines stripped");
                System.exit(3);
            }
            System.out.println("mysql-ok rows=" + rows);
        } catch (Throwable t) {
            System.err.println("esper-mysql fixture check failed: " + t);
            System.exit(3);
        }
    }
}
EOF
probe_classes="$work/probe-classes"
mkdir -p "$probe_classes"
"$javac_bin" -encoding UTF-8 -d "$probe_classes" "$work/DbProbe.java"
if ! "$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -cp "$probe_classes:$classpath" DbProbe; then
  echo "The oracle requires the esper-mysql MySQL 8.0 fixture listening on 127.0.0.1:3306." >&2
  echo "Start and load it per docs/integration/external-services.md (the script does not start Docker itself):" >&2
  echo "  docker run --name esper-mysql -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test -p 3306:3306 -d mysql:8.0" >&2
  echo "  sed '/^[[:space:]]*\\/\\//d' $esper_root/common/etc/regression/create_testdb.sql | docker exec -i esper-mysql mysql -uroot -ppassword --force test" >&2
  exit 3
fi

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ContextInitTermRemainderScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ContextInitTermRemainderScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def lis($q; $t; $v): {"case": "db-historical", "operation": "listener", "statement": "s0", "sequence": $q, "time": $t, "new": [{"kind": "row", "fields": {"s1.mychar": $v}}]};
    def cerr($c; $st; $v): {"case": $c, "operation": "compile-error", "statement": $st, "sequence": 0, "value": $v};
    def crec($c): [.records[] | select(.case == $c)];
    .version == "esper-parity/v1" and
    .id == "context-init-term-remainder" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 16) and
    ((crec("db-historical") | length) == 2) and
    ((crec("distinct-invalid") | length) == 5) and
    ((crec("now-invalid") | length) == 3) and
    ((crec("hash-invalid") | length) == 6) and
    # db-historical: the two in-window deliveries pin the asserted
    # s1.mychar field (env.assertPropsNew): "Y" for mybigint 2 on day one
    # and "X" for mybigint 3 on day two; the out-of-window sends emit
    # nothing.
    (crec("db-historical") == [
        lis(1; "2002-05-01T09:00:00Z"; "Y"),
        lis(2; "2002-05-02T09:00:00Z"; "X")
    ]) and
    # distinct-invalid: five compile-error records carrying the pinned
    # prefixes in probe order.
    (crec("distinct-invalid") == [
        cerr("distinct-invalid"; "distinct-no-as"; "Distinct-expressions require that a stream name is assigned to the stream using '"'"'as'"'"'"),
        cerr("distinct-invalid"; "distinct-pattern"; "Distinct-expressions require a stream as the initiated-by condition"),
        cerr("distinct-invalid"; "distinct-subselect"; "Invalid context distinct-clause expression '"'"'subselect_0'"'"': Aggregation, sub-select, previous or prior functions are not supported in this context"),
        cerr("distinct-invalid"; "distinct-empty"; "Distinct-expressions have not been provided"),
        cerr("distinct-invalid"; "start-distinct"; "Incorrect syntax near '"'"'distinct'"'"' (a reserved keyword)")
    ]) and
    # now-invalid: three compile-error records in probe order.
    (crec("now-invalid") == [
        cerr("now-invalid"; "now-alone-terminated"; "Incorrect syntax near '"'"'terminated'"'"' (a reserved keyword) expecting '"'"'and'"'"'"),
        cerr("now-invalid"; "now-and-nonoverlapping"; "Incorrect syntax near '"'"'and'"'"' (a reserved keyword)"),
        cerr("now-invalid"; "now-with-filter"; "Invalid use of '"'"'now'"'"' with initiated-by stream")
    ]) and
    # hash-invalid: six compile-error records in probe order (four
    # path-less, then the two with-path probes after the ctx/window
    # fixture deploys).
    (crec("hash-invalid") == [
        cerr("hash-invalid"; "hash-dummy-filter"; "Failed to validate filter expression '"'"'dummy=1'"'"': Property named '"'"'dummy'"'"' is not valid in any stream"),
        cerr("hash-invalid"; "hash-bad-func"; "expected a hash function that is any of {consistent_hash_crc32, hash_code}"),
        cerr("hash-invalid"; "hash-bare-prop"; "expected a hash function that is any of {consistent_hash_crc32, hash_code}"),
        cerr("hash-invalid"; "hash-no-params"; "expected one or more parameters to the hash function"),
        cerr("hash-invalid"; "statement-stream-type"; "requires that any of the event types that are listed in the segmented context also appear in any of the filter expressions of the statement, type '"'"'SupportBean_S0'"'"' is not one of the types listed"),
        cerr("hash-invalid"; "partition-named-window"; "Partition criteria may not include named windows")
    ])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid context-init-term-remainder trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
