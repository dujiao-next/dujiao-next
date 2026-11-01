#!/usr/bin/env python3
"""Reject a green Go run that silently skipped the required real DB scenarios."""

import json
from pathlib import Path
import sys


PACKAGE = "github.com/dujiao-next/internal/persistence/integrationtest/payment"
REQUIRED = frozenset({
    "TestPostgresPaymentSettlementReplayAndFreeze",
    "TestPostgresPaymentAtomicRollbackAndGateLock",
    "TestPostgresPaymentRecoversLostRedisQueueAndDeliversOnce",
})


def check(lines):
    completed = set()
    package_passed = False
    for line in lines:
        event = json.loads(line)
        if not isinstance(event, dict):
            raise ValueError("invalid Go test event")
        action = event.get("Action")
        if action == "fail":
            raise ValueError("Go report contains a failed test or package")
        if event.get("Package") != PACKAGE:
            continue
        name = event.get("Test", "")
        if action == "skip" and name.split("/", 1)[0] in REQUIRED:
            raise ValueError("required PostgreSQL/Redis scenario was skipped")
        if action == "pass":
            if name in REQUIRED:
                completed.add(name)
            elif not name:
                package_passed = True
    if completed != REQUIRED or not package_passed:
        raise ValueError("required PostgreSQL/Redis results missing or incomplete")


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: check_payment_test_report.py GO_TEST_JSONL")
    try:
        with Path(sys.argv[1]).open(encoding="utf-8") as report:
            check(report)
    except (OSError, ValueError) as error:
        raise SystemExit(str(error)) from error
    print("Required PostgreSQL/Redis scenarios passed without skips.")


if __name__ == "__main__":
    main()
