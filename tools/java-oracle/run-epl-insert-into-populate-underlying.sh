#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-insert-into-populate-underlying.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided. The
Avro case additionally requires the esper common-avro module classes, the
Avro 1.11.3 jar and Jackson jars on the classpath; the script derives them
from the local Maven repository when present.
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
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -f "$scenario" ] || { echo "scenario file not found: $scenario" >&2; exit 2; }

# Verify Java 17
java_version=$(java -version 2>&1 | head -1)
case "$java_version" in
    *17*) ;;
    *) echo "Java 17 required, got: $java_version" >&2; exit 2 ;;
esac

# Verify commit
actual_commit=$(cd "$esper_root" && git rev-parse HEAD)
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "WARNING: Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2
fi

# Resolve scenario/output to absolute paths; the oracle runs with the Esper
# checkout as its working directory so relative repo paths would not resolve.
scenario=$(CDPATH= cd -- "$(dirname -- "$scenario")" && pwd)/$(basename -- "$scenario")
output_dir=$(CDPATH= cd -- "$(dirname -- "$output")" && pwd)
output="$output_dir/$(basename -- "$output")"

oracle_java="$script_root/EPLInsertIntoPopulateUnderlyingScenarioOracle.java"
oracle_class="$script_root/EPLInsertIntoPopulateUnderlyingScenarioOracle.class"

# Classpath: base esper modules plus Avro/Jackson dependencies for the avro case.
classpath_file="/tmp/esper-cp-avro.txt"
if [ ! -f "$classpath_file" ]; then
    base_cp="/tmp/esper-cp.txt"
    if [ ! -f "$base_cp" ]; then
        echo "Building Esper modules..." >&2
        (cd "$esper_root" && mvn -pl common,compiler,runtime,common-avro -am -Dgpg.skip=true -DskipTests package -q)
        # Re-derive: modules print classpath via dependency:build-classpath is
        # expensive; fall back to the module target classes plus local Maven jars.
    fi
    avro_jar=$(find "$HOME/.m2/repository/org/apache/avro/avro" -name 'avro-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
    jackson_databind=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-databind" -name 'jackson-databind-*.jar' ! -name '*sources*' 2>/dev/null | sort -V | tail -1)
    jackson_core=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-core" -name 'jackson-core-*.jar' ! -name '*sources*' 2>/dev/null | sort -V | tail -1)
    jackson_ann=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-annotations" -name 'jackson-annotations-*.jar' ! -name '*sources*' 2>/dev/null | sort -V | tail -1)
    {
        cat "$base_cp"
        printf '%s\n' "$esper_root/common-avro/target/classes"
        [ -n "$avro_jar" ] && printf '%s\n' "$avro_jar"
        [ -n "$jackson_databind" ] && printf '%s\n' "$jackson_databind"
        [ -n "$jackson_core" ] && printf '%s\n' "$jackson_core"
        [ -n "$jackson_ann" ] && printf '%s\n' "$jackson_ann"
    } | tr '\n' ':' | sed 's/:$//' > "$classpath_file"
fi

if [ "$skip_build" -eq 0 ]; then
    # Compile oracle
    echo "Compiling oracle..." >&2
    (cd "$esper_root" && javac -cp "$(cat "$classpath_file")" -d /tmp/oracle-classes "$oracle_java")
fi

# Run oracle
echo "Running oracle..." >&2
(cd "$esper_root" && java -cp "$(cat "$classpath_file"):/tmp/oracle-classes" \
    EPLInsertIntoPopulateUnderlyingScenarioOracle "$scenario" > "$output")

echo "Trace written to: $output" >&2
