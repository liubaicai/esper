#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-expr-filter-optimizable-or-rewrite.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
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

oracle_java="$script_root/ExprFilterOptimizableOrRewriteScenarioOracle.java"
oracle_class="$script_root/ExprFilterOptimizableOrRewriteScenarioOracle.class"
classpath_file="/tmp/esper-cp.txt"

if [ "$skip_build" -eq 0 ]; then
    # Build Esper modules if needed
    if [ ! -f "$classpath_file" ]; then
        echo "Building Esper modules..." >&2
        cd "$esper_root"
        mvn -pl common,compiler,runtime -am -Dgpg.skip=true -DskipTests package -q
    fi

    # Compile oracle
    echo "Compiling oracle..." >&2
    javac -cp "$(cat "$classpath_file")" -d "$script_root" "$oracle_java"
fi

# Run oracle
echo "Running oracle..." >&2
java -cp "$(cat "$classpath_file"):$script_root" \
    ExprFilterOptimizableOrRewriteScenarioOracle "$scenario" > "$output"

echo "Trace written to: $output" >&2
