#!/usr/bin/env sh
set -eu

usage() {
  cat >&2 <<'EOF'
Usage: $0 --esper-root <path> --scenario <json> --output <trace.json>
EOF
  exit 1
}

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

while [ "$#" -gt 0 ]; do
  case "$1" in
    --esper-root) esper_root="$2"; shift 2;;
    --scenario) scenario="$2"; shift 2;;
    --output) output="$2"; shift 2;;
    --skip-build) skip_build=1; shift;;
    *) usage;;
  esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || { echo "cannot read HEAD" >&2; exit 1; }
if [ "$actual_commit" != "$expected_commit" ]; then
  echo "Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2
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
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found; it is required to validate the oracle trace" >&2; exit 1; }

java_version=$($java_bin -version 2>&1 | awk -F'[".]' '/version/ {print $2; exit}')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if [ "$skip_build" -eq 0 ]; then
  "$mvn_bin" -q -f "$esper_root/common/pom.xml" install -DskipTests -Dgpg.skip=true
  "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" install -DskipTests -Dgpg.skip=true
  "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" install -DskipTests -Dgpg.skip=true
  "$mvn_bin" -q -f "$esper_root/regression-lib/pom.xml" install -DskipTests -Dgpg.skip=true
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-database-restart.XXXXXX")
cleanup() { rm -rf "$work"; }
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
    "$script_root/EPLDatabaseRestartScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLDatabaseRestartScenarioOracle "$scenario" > "$output"

if ! jq -e '.version == "esper-parity/v1" and .id == "epl-database-restart" and (.records | type == "array") and (.records | length == 100) and
    ([.records[] | select(.case == "restart-statement" and .operation == "listener" and .statement == "s0" and (.new | length == 1) and .new[0].kind == "row" and .new[0].fields.mychar == "Z")] | length) == 100 and
    ([.records[].sequence] == [range(1; 101)])' "$output" >/dev/null 2>&1; then
  echo "Oracle trace validation failed" >&2
  exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
