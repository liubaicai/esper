#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-<scenario>.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the Java oracle for the EPLInsertIntoTransposePattern executions and
writes a normalized trace. Requires Java 17 and the fixed Esper commit.
EOF
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
    *openjdk*17*|*"17"*|*"Java(TM) SE Runtime Environment 17"*)
        ;;
    *)
        echo "ERROR: expected Java 17, got: $java_version" >&2
        exit 2
        ;;
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

oracle_java="$script_root/EPLInsertIntoTransposePatternScenarioOracle.java"
oracle_class="$script_root/EPLInsertIntoTransposePatternScenarioOracle.class"

# Classpath: base esper modules (no Avro case in this slice).
classpath_file="/tmp/esper-cp.txt"
if [ ! -f "$classpath_file" ]; then
    echo "Building Esper modules..." >&2
    (cd "$esper_root" && mvn -pl common,compiler,runtime -am -Dgpg.skip=true -DskipTests package -q)
    {
        find "$esper_root/common/target/classes" "$esper_root/compiler/target/classes" "$esper_root/runtime/target/classes" -maxdepth 0 -type d
        find "$HOME/.m2/repository" -name '*.jar' \
            ! -name '*sources*' ! -name '*javadoc*' \
            \( -path '*log4j*' -o -path '*slf4j*' -o -path '*antlr*' -o -path '*commons-*' -o -path '*jackson-*' -o -path '*avro*' \) 2>/dev/null
    } | tr '\n' ':' | sed 's/:$//' > "$classpath_file"
fi

if [ "$skip_build" -eq 0 ]; then
    echo "Compiling oracle..." >&2
    (cd "$esper_root" && javac -cp "$(cat "$classpath_file")" -d /tmp/oracle-classes "$oracle_java")
fi

# Run oracle
echo "Running oracle..." >&2
(cd "$esper_root" && java -cp "$(cat "$classpath_file"):/tmp/oracle-classes" \
    EPLInsertIntoTransposePatternScenarioOracle "$scenario" > "$output")

echo "Trace written to: $output" >&2
