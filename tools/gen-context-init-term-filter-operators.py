#!/usr/bin/env python3
"""Generate the context-init-term-filter-operators parity scenario JSON.

Mirrors ContextInitTermFilterAllOperators (25-operator correlated filter
matrix), ContextInitTermFilterBooleanOperator (arithmetic correlated
filter) and ContextInitTermPatternInitiatedStraightSelect (OR-pattern
start with unbound-tag projections) from the pinned Esper 9.0.0 commit.

The case names are the contract between the Go runner and the Java
oracle; both map each name to the same filter expression. Run:
  python3 tools/gen-context-init-term-filter-operators.py \
      > testdata/parity/context-init-term-filter-operators.json
"""
import json
import sys

VERSION = "esper-parity/v1"
ID = "context-init-term-filter-operators"

# op -> [(boxed value or None, expected match), ...]  (intBoxed rows)
INT_MATRIX = [
    ("op-eq-intboxed-lhs", [(10, True), (9, False), (None, False)]),
    ("op-eq-intboxed-rhs", [(10, True), (9, False), (None, False)]),
    ("op-gt-intboxed", [(11, False), (10, False), (9, True), (8, True)]),
    ("op-ge-intboxed", [(11, False), (10, True), (9, True), (8, True)]),
    ("op-lt-intboxed", [(11, True), (10, False), (9, False), (8, False)]),
    ("op-le-intboxed", [(11, True), (10, True), (9, False), (8, False)]),
    ("op-lt-intboxed-rev", [(11, False), (10, False), (9, True), (8, True)]),
    ("op-le-intboxed-rev", [(11, False), (10, True), (9, True), (8, True)]),
    ("op-gt-intboxed-rev", [(11, True), (10, False), (9, False), (8, False)]),
    ("op-ge-intboxed-rev", [(11, True), (10, True), (9, False), (8, False)]),
    ("op-in-intboxed", [(11, False), (10, True), (9, False), (8, False)]),
    ("op-between-intboxed", [(11, False), (10, True), (9, False), (8, False)]),
    ("op-ne-intboxed-lhs", [(10, False), (9, True), (None, False)]),
    ("op-ne-intboxed-rhs", [(10, False), (9, True), (None, False)]),
    ("op-notin-intboxed", [(11, True), (10, False), (9, True), (8, True)]),
    ("op-notbetween-intboxed", [(11, True), (10, False), (9, True), (8, True)]),
    ("op-is-intboxed-lhs", [(10, True), (9, False), (None, False)]),
    ("op-is-intboxed-rhs", [(10, True), (9, False), (None, False)]),
    ("op-isnot-intboxed-lhs", [(10, False), (9, True), (None, True)]),
    ("op-isnot-intboxed-rhs", [(10, False), (9, True), (None, True)]),
]
# op -> rows with shortBoxed values
SHORT_MATRIX = [
    ("op-eq-short-lhs", [(10, True), (9, False), (None, False)]),
    ("op-eq-short-rhs", [(10, True), (9, False), (None, False)]),
    ("op-gt-short", [(11, False), (10, False), (9, True), (8, True)]),
    ("op-lt-short-rev", [(11, False), (10, False), (9, True), (8, True)]),
    ("op-in-short", [(11, False), (10, True), (9, False), (8, False)]),
]

# Case groups: execution -> case names. The generator emits the cases in
# this order so both oracles see identical event streams.
CASE_ORDER = (
    [name for name, _ in INT_MATRIX]
    + [name for name, _ in SHORT_MATRIX]
    + ["op-boolean-arithmetic", "op-pattern-straight-select"]
)


def bean_payload(int_value, short_value, the_string="", int_primitive=0):
    return {
        "theString": the_string,
        "intPrimitive": int_primitive,
        "intBoxed": int_value,
        "shortBoxed": short_value,
        "longBoxed": None,
    }


def bean(the_string, int_primitive):
    return bean_payload(None, None, the_string, int_primitive)


def build():
    steps = []
    for name, rows in INT_MATRIX + SHORT_MATRIX:
        steps.append({"op": "case", "case": name})
        steps.append({"op": "send", "eventType": "SupportBean_S0",
                      "payload": {"id": 10, "p00": "S01", "p01": None}})
        short = name.endswith("-short") or "-short-" in name
        for value, _ in rows:
            if short:
                steps.append({"op": "send", "eventType": "SupportBean",
                              "payload": bean_payload(None, value)})
            else:
                steps.append({"op": "send", "eventType": "SupportBean",
                              "payload": bean_payload(value, None)})
    # ContextInitTermFilterBooleanOperator: S0(3,"S01") starts a partition;
    # E2(2) matches 2+3=5; S0(3,"S02") adds a second; E3(2) matches both;
    # S0(4,"S03") adds a third; E4(2) matches only the first two; E5(1)
    # matches only the third. E1(2) precedes any initiation.
    steps.append({"op": "case", "case": "op-boolean-arithmetic"})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E1", 2)})
    steps.append({"op": "send", "eventType": "SupportBean_S0",
                  "payload": {"id": 3, "p00": "S01", "p01": None}})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E2", 2)})
    steps.append({"op": "send", "eventType": "SupportBean_S0",
                  "payload": {"id": 3, "p00": "S02", "p01": None}})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E3", 2)})
    steps.append({"op": "send", "eventType": "SupportBean_S0",
                  "payload": {"id": 4, "p00": "S03", "p01": None}})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E4", 2)})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E5", 1)})
    # ContextInitTermPatternInitiatedStraightSelect: S1(2) starts partition
    # with tag b; E1(0) projects c1=context.a.id (unbound -> null),
    # c2=context.b.id=2; S0(3) starts partition with tag a; E2(0) projects
    # both partitions.
    steps.append({"op": "case", "case": "op-pattern-straight-select"})
    steps.append({"op": "send", "eventType": "SupportBean_S1",
                  "payload": {"id": 2, "p10": None, "p11": None}})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E1", 0)})
    steps.append({"op": "send", "eventType": "SupportBean_S0",
                  "payload": {"id": 3, "p00": None, "p01": None}})
    steps.append({"op": "send", "eventType": "SupportBean",
                  "payload": bean("E2", 0)})
    return {"version": VERSION, "id": ID, "steps": steps}


def main():
    if len(sys.argv) != 1:
        raise SystemExit(__doc__)
    json.dump(build(), sys.stdout, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
